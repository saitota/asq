// Package tui implements the in-process interactive picker that replaced the
// external fzf integration: an inline fuzzy filter over pre-rendered items
// with a live preview pane, reading keys from the controlling terminal and
// drawing on it so stdout stays clean for command output.
package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/saitota/asq/internal/ansitext"
)

// Item is one selectable row. Display may contain ANSI colors; Haystack is
// the lowercased text queries match against.
type Item struct {
	Display  string
	Haystack string
}

// ErrCancelled is returned when the user aborts the picker.
var ErrCancelled = errors.New("cancelled")

const (
	pickerHeightPercent = 80
	minPickerHeight     = 8
	previewWidthPercent = 40
	promptPrefix        = "> "
	pointerPrefix       = "▌ "
	blankPrefix         = "  "
)

// Pick runs the picker over items with query pre-seeded and returns the index
// of the chosen item. preview renders the right pane for the highlighted item.
func Pick(items []Item, preview func(index int) string, query string) (int, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return 0, errors.New("interactive picker requires a terminal; pass a session id or use `asq list --fzf`")
	}
	defer tty.Close()

	model := newPickerModel(items, preview, query)
	program := tea.NewProgram(model, tea.WithInput(tty), tea.WithOutput(tty))
	final, err := program.Run()
	if err != nil {
		return 0, err
	}
	result, ok := final.(pickerModel)
	if !ok {
		return 0, fmt.Errorf("unexpected picker model %T", final)
	}
	if result.cancelled || result.choice < 0 {
		return 0, ErrCancelled
	}
	return result.choice, nil
}

type pickerModel struct {
	items    []Item
	preview  func(index int) string
	previews map[int]string

	query         string
	filtered      []int
	cursor        int
	listOffset    int
	previewOffset int

	width  int
	height int

	choice    int
	cancelled bool
}

func newPickerModel(items []Item, preview func(index int) string, query string) pickerModel {
	m := pickerModel{
		items:    items,
		preview:  preview,
		previews: map[int]string{},
		query:    query,
		choice:   -1,
	}
	m.refilter()
	return m
}

func (m pickerModel) Init() tea.Cmd {
	return nil
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.PasteMsg:
		m.setQuery(m.query + sanitizePaste(msg.Content))
		return m, nil
	}
	return m, nil
}

// sanitizePaste keeps pasted text usable as a one-line query: the filter
// tokenizes on whitespace, so newlines and tabs become spaces; other control
// characters would corrupt the prompt line and are dropped.
func sanitizePaste(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

func (m pickerModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, tea.Quit
	case "enter":
		if len(m.filtered) > 0 {
			m.choice = m.filtered[m.cursor]
			return m, tea.Quit
		}
		return m, nil
	case "up", "ctrl+p", "ctrl+k":
		m.moveCursor(-1)
		return m, nil
	case "down", "ctrl+n", "ctrl+j":
		m.moveCursor(1)
		return m, nil
	case "pgup":
		m.previewOffset -= m.paneHeight() / 2
		if m.previewOffset < 0 {
			m.previewOffset = 0
		}
		return m, nil
	case "pgdown":
		m.previewOffset += m.paneHeight() / 2
		return m, nil
	case "backspace":
		runes := []rune(m.query)
		if len(runes) > 0 {
			m.setQuery(string(runes[:len(runes)-1]))
		}
		return m, nil
	case "ctrl+u":
		m.setQuery("")
		return m, nil
	case "ctrl+w":
		m.setQuery(strings.TrimRight(m.query, " "))
		if i := strings.LastIndex(m.query, " "); i >= 0 {
			m.setQuery(m.query[:i+1])
		} else {
			m.setQuery("")
		}
		return m, nil
	}
	if msg.Text != "" {
		m.setQuery(m.query + msg.Text)
	}
	return m, nil
}

func (m *pickerModel) setQuery(query string) {
	if m.query == query {
		return
	}
	m.query = query
	m.refilter()
}

// refilter recomputes the matches: every whitespace-separated token must be a
// case-insensitive substring of the item's haystack.
func (m *pickerModel) refilter() {
	tokens := strings.Fields(strings.ToLower(m.query))
	m.filtered = m.filtered[:0]
	for i, item := range m.items {
		if matchesAll(item.Haystack, tokens) {
			m.filtered = append(m.filtered, i)
		}
	}
	m.cursor = 0
	m.listOffset = 0
	m.previewOffset = 0
}

func matchesAll(haystack string, tokens []string) bool {
	for _, token := range tokens {
		if !strings.Contains(haystack, token) {
			return false
		}
	}
	return true
}

func (m *pickerModel) moveCursor(delta int) {
	if len(m.filtered) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > len(m.filtered)-1 {
		m.cursor = len(m.filtered) - 1
	}
	m.previewOffset = 0
	pane := m.paneHeight()
	if pane <= 0 {
		return
	}
	if m.cursor < m.listOffset {
		m.listOffset = m.cursor
	}
	if m.cursor >= m.listOffset+pane {
		m.listOffset = m.cursor - pane + 1
	}
}

// paneHeight is the number of rows available to the list and preview panes:
// the picker height minus the prompt and counter lines.
func (m pickerModel) paneHeight() int {
	pane := m.pickerHeight() - 2
	if pane < 0 {
		return 0
	}
	return pane
}

func (m pickerModel) pickerHeight() int {
	height := m.height * pickerHeightPercent / 100
	if height < minPickerHeight {
		height = minPickerHeight
	}
	if m.height > 0 && height > m.height {
		height = m.height
	}
	return height
}

func (m pickerModel) View() tea.View {
	if m.width <= 0 || m.height <= 0 {
		return tea.NewView("")
	}
	listWidth := m.width * (100 - previewWidthPercent) / 100
	previewWidth := m.width - listWidth - 3
	pane := m.paneHeight()

	lines := make([]string, 0, m.pickerHeight())
	lines = append(lines, truncateDisplay(promptPrefix+m.query, m.width))
	lines = append(lines, truncateDisplay(fmt.Sprintf("  %d/%d", len(m.filtered), len(m.items)), m.width))

	listLines := m.listLines(listWidth, pane)
	previewLines := m.previewLines(previewWidth, pane)
	for i := 0; i < pane; i++ {
		left := padDisplay(listLines[i], listWidth)
		lines = append(lines, left+" │ "+previewLines[i])
	}

	view := tea.NewView(strings.Join(lines, "\n"))
	view.Cursor = tea.NewCursor(lipgloss.Width(promptPrefix+m.query), 0)
	return view
}

func (m pickerModel) listLines(width, pane int) []string {
	lines := make([]string, pane)
	for row := 0; row < pane; row++ {
		idx := m.listOffset + row
		if idx >= len(m.filtered) {
			lines[row] = ""
			continue
		}
		prefix := blankPrefix
		if idx == m.cursor {
			prefix = pointerPrefix
		}
		lines[row] = prefix + truncateDisplay(m.items[m.filtered[idx]].Display, width-lipgloss.Width(prefix))
	}
	return lines
}

func (m pickerModel) previewLines(width, pane int) []string {
	lines := make([]string, pane)
	if len(m.filtered) == 0 || width <= 0 {
		return lines
	}
	selected := m.filtered[m.cursor]
	text, ok := m.previews[selected]
	if !ok {
		text = m.preview(selected)
		m.previews[selected] = text
	}
	wrapped := wrapDisplay(text, width)
	for row := 0; row < pane; row++ {
		line := ""
		if m.previewOffset+row < len(wrapped) {
			line = wrapped[m.previewOffset+row]
		}
		lines[row] = line
	}
	return lines
}

func truncateDisplay(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansitext.CutDisplay(value, width)
}

func padDisplay(value string, width int) string {
	return ansitext.Pad(value, width)
}

func wrapDisplay(text string, width int) []string {
	return ansitext.Wrap(text, width)
}
