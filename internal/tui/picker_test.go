package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
)

func lipglossWidthForTest(s string) int {
	return lipgloss.Width(s)
}

func testItems() []Item {
	return []Item{
		{Display: "claude 07-06 fix-retry ~/work/api", Haystack: "claude fix-retry /users/you/work/api the retry test fails"},
		{Display: "codex 07-05 auth ~/work/webapp", Haystack: "codex auth /users/you/work/webapp refactor the auth middleware"},
		{Display: "copilot 07-04 docs ~/work/docs", Haystack: "copilot docs /users/you/work/docs draft a quickstart"},
	}
}

func testModel(query string) pickerModel {
	m := newPickerModel(testItems(), func(i int) string { return fmt.Sprintf("preview %d", i) }, query)
	m.width = 80
	m.height = 24
	return m
}

func press(t *testing.T, m pickerModel, keys ...string) pickerModel {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyPressMsg
		if strings.HasPrefix(k, "text:") {
			text := strings.TrimPrefix(k, "text:")
			msg = tea.KeyPressMsg{Text: text, Code: []rune(text)[0]}
		} else {
			switch k {
			case "enter":
				msg = tea.KeyPressMsg{Code: tea.KeyEnter}
			case "esc":
				msg = tea.KeyPressMsg{Code: tea.KeyEscape}
			case "backspace":
				msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
			case "up":
				msg = tea.KeyPressMsg{Code: tea.KeyUp}
			case "down":
				msg = tea.KeyPressMsg{Code: tea.KeyDown}
			case "ctrl+c":
				msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
			case "ctrl+u":
				msg = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
			case "ctrl+w":
				msg = tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl}
			default:
				t.Fatalf("unknown key %q", k)
			}
		}
		next, _ := m.Update(msg)
		m = next.(pickerModel)
	}
	return m
}

func TestFilterMatchesEveryTokenAcrossHaystack(t *testing.T) {
	m := testModel("")
	if len(m.filtered) != 3 {
		t.Fatalf("filtered = %d, want 3", len(m.filtered))
	}
	m = press(t, m, "text:a", "text:u", "text:t", "text:h")
	if len(m.filtered) != 1 || m.filtered[0] != 1 {
		t.Fatalf("filtered = %v, want [1]", m.filtered)
	}
	// Tokens are AND-ed and match hidden text too.
	m = press(t, m, "text: ", "text:r", "text:e", "text:f")
	if len(m.filtered) != 1 || m.filtered[0] != 1 {
		t.Fatalf("filtered = %v, want [1]", m.filtered)
	}
	m = press(t, m, "text:z")
	if len(m.filtered) != 0 {
		t.Fatalf("filtered = %v, want empty", m.filtered)
	}
}

func TestFilterIsCaseInsensitive(t *testing.T) {
	m := testModel("QuickStart")
	if len(m.filtered) != 1 || m.filtered[0] != 2 {
		t.Fatalf("filtered = %v, want [2]", m.filtered)
	}
}

func TestEnterSelectsHighlightedItem(t *testing.T) {
	m := testModel("")
	m = press(t, m, "down", "enter")
	if m.cancelled || m.choice != 1 {
		t.Fatalf("choice = %d cancelled = %v", m.choice, m.cancelled)
	}
}

func TestEnterWithNoMatchesDoesNotSelect(t *testing.T) {
	m := testModel("nomatchxyz")
	m = press(t, m, "enter")
	if m.choice != -1 {
		t.Fatalf("choice = %d, want -1", m.choice)
	}
}

func TestEscapeAndCtrlCCancel(t *testing.T) {
	for _, key := range []string{"esc", "ctrl+c"} {
		m := press(t, testModel(""), key)
		if !m.cancelled {
			t.Fatalf("%s did not cancel", key)
		}
	}
}

func TestQueryEditingKeys(t *testing.T) {
	m := testModel("auth extra")
	m = press(t, m, "ctrl+w")
	if m.query != "auth " {
		t.Fatalf("ctrl+w query = %q, want %q", m.query, "auth ")
	}
	m = press(t, m, "backspace")
	if m.query != "auth" {
		t.Fatalf("backspace query = %q", m.query)
	}
	m = press(t, m, "ctrl+u")
	if m.query != "" || len(m.filtered) != 3 {
		t.Fatalf("ctrl+u query = %q filtered = %v", m.query, m.filtered)
	}
}

func paste(t *testing.T, m pickerModel, content string) pickerModel {
	t.Helper()
	next, _ := m.Update(tea.PasteMsg{Content: content})
	return next.(pickerModel)
}

func TestPasteAppendsToQueryAndRefilters(t *testing.T) {
	m := testModel("au")
	m = paste(t, m, "th")
	if m.query != "auth" {
		t.Fatalf("query = %q, want %q", m.query, "auth")
	}
	if len(m.filtered) != 1 || m.filtered[0] != 1 {
		t.Fatalf("filtered = %v, want [1]", m.filtered)
	}
}

func TestPasteConvertsNewlinesAndTabsToSpaces(t *testing.T) {
	m := testModel("")
	m = paste(t, m, "auth\nwebapp\tcodex")
	if m.query != "auth webapp codex" {
		t.Fatalf("query = %q, want %q", m.query, "auth webapp codex")
	}
	if len(m.filtered) != 1 || m.filtered[0] != 1 {
		t.Fatalf("filtered = %v, want [1]", m.filtered)
	}
}

func TestPasteDropsControlCharacters(t *testing.T) {
	m := paste(t, testModel(""), "fo\x00o\r\nb\x1bar\x7f")
	if m.query != "foo  bar" {
		t.Fatalf("query = %q, want %q", m.query, "foo  bar")
	}
}

func TestCursorClampsAndResetsOnQueryChange(t *testing.T) {
	m := testModel("")
	m = press(t, m, "up")
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.cursor)
	}
	m = press(t, m, "down", "down", "down", "down")
	if m.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", m.cursor)
	}
	m = press(t, m, "text:d")
	if m.cursor != 0 {
		t.Fatalf("cursor after query change = %d, want 0", m.cursor)
	}
}

func TestViewShowsCounterPointerAndPreview(t *testing.T) {
	m := testModel("auth")
	view := m.View().Content
	if !strings.Contains(view, "> auth") {
		t.Fatalf("view missing prompt: %q", view)
	}
	if !strings.Contains(view, "1/3") {
		t.Fatalf("view missing counter: %q", view)
	}
	if !strings.Contains(view, pointerPrefix) {
		t.Fatalf("view missing pointer: %q", view)
	}
	if !strings.Contains(view, "preview 1") {
		t.Fatalf("view missing preview: %q", view)
	}
	for i, line := range strings.Split(view, "\n") {
		if got := lipglossWidthForTest(line); got > m.width {
			t.Fatalf("line %d width = %d > %d: %q", i, got, m.width, line)
		}
	}
}

func TestViewBeforeWindowSizeIsEmpty(t *testing.T) {
	m := newPickerModel(testItems(), func(int) string { return "" }, "")
	if got := m.View().Content; got != "" {
		t.Fatalf("view = %q, want empty", got)
	}
}

func TestWrapDisplayWrapsWideRunes(t *testing.T) {
	lines := wrapDisplay("日本語のテキストです", 6)
	for _, line := range lines {
		if lipglossWidthForTest(line) > 6 {
			t.Fatalf("wrapped line too wide: %q", line)
		}
	}
	if len(lines) < 3 {
		t.Fatalf("expected multiple wrapped lines, got %v", lines)
	}
}

func TestViewSurvivesTinyTerminals(t *testing.T) {
	for height := 0; height <= 4; height++ {
		m := testModel("")
		m.width = 5
		m.height = height
		_ = m.View().Content
	}
}

func TestTruncateDisplaySkipsANSISequences(t *testing.T) {
	colored := "\x1b[38;2;245;158;11m●\x1b[0m  07-06 18:13  fix-flaky"
	got := truncateDisplay(colored, 10)
	if lipglossWidthForTest(got) != 10 {
		t.Fatalf("visible width = %d, want 10: %q", lipglossWidthForTest(got), got)
	}
	if !strings.Contains(got, "\x1b[38;2;245;158;11m●\x1b[0m") {
		t.Fatalf("color sequence was split or dropped: %q", got)
	}
	if !strings.HasSuffix(got, "\x1b[0m") {
		t.Fatalf("truncated styled text must end with a reset: %q", got)
	}
	if !strings.Contains(got, "07-06") {
		t.Fatalf("escape bytes were counted as width: %q", got)
	}
}

func TestTruncateDisplayNeverCutsMidSequence(t *testing.T) {
	value := "abc\x1b[1mBold\x1b[22mrest of the line"
	for width := 1; width <= 12; width++ {
		got := truncateDisplay(value, width)
		if strings.Contains(got, "\x1b") && !strings.HasSuffix(got, "m") && !strings.HasSuffix(got, "\x1b[0m") {
			t.Fatalf("width %d left a dangling escape: %q", width, got)
		}
		stripped := got
		for _, seq := range []string{"\x1b[1m", "\x1b[22m", "\x1b[0m"} {
			stripped = strings.ReplaceAll(stripped, seq, "")
		}
		if strings.Contains(stripped, "\x1b") {
			t.Fatalf("width %d split an escape sequence: %q", width, got)
		}
	}
}

func TestWrapDisplayCarriesStyleAcrossWrappedLines(t *testing.T) {
	text := "\x1b[38;2;245;158;11massistant\x1b[0m plain tail here"
	lines := wrapDisplay(text, 6)
	for i, line := range lines {
		if lipglossWidthForTest(line) > 6 {
			t.Fatalf("line %d too wide: %q", i, line)
		}
		open := strings.Count(line, "\x1b[38;2;245;158;11m")
		if open > 0 && !strings.Contains(line, "\x1b[0m") {
			t.Fatalf("line %d leaves color open: %q", i, line)
		}
	}
	if !strings.HasPrefix(lines[1], "\x1b[38;2;245;158;11m") {
		t.Fatalf("continuation line should reopen the color: %q", lines[1])
	}
}
