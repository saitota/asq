package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/term"
	"github.com/saitota/asq/internal/ansitext"
	"github.com/saitota/asq/internal/session"
)

const defaultTableWidth = 120
const minTableWidth = 84
const tableUpdatedWidth = 11

// Five padded columns plus table borders; keep content width math independent from lipgloss internals.
const tableFrameWidth = 16
const minTableContentWidth = minSessionColumnWidth + minNameColumnWidth + minPathColumnWidth + minSnippetColumnWidth
const minSessionColumnWidth = 16
const minNameColumnWidth = 10
const minPathColumnWidth = 18
const minSnippetColumnWidth = 12
const sessionColumnPercent = 25
const nameColumnPercent = 20
const pathColumnPercent = 35
const tableSnippetPreTruncateMultiplier = 2

// Session IDs are shortened for display only; lookups still resolve against full IDs.
const shortSessionIDLength = 12

func writeSessionTable(w io.Writer, sessions []session.Session) error {
	if len(sessions) == 0 {
		_, err := fmt.Fprintln(w, "No sessions found")
		return err
	}
	layout := sessionTableLayout(w)
	rows := make([][]string, 0, len(sessions))
	current, _ := os.Getwd()
	matcher := session.NewCWDMatcher(current)
	color := tableColorEnabled(w)
	for _, sess := range sessions {
		label := ansitext.TruncateEnd(sessionLabel(sess, matcher), layout.SessionWidth)
		if color {
			label = colorSourceInLabel(label, sess.Source)
		}
		rows = append(rows, []string{
			label,
			ansitext.TruncateEnd(sess.Name, layout.NameWidth),
			ansitext.TruncateMiddle(tildePath(sess.CWD), layout.PathWidth),
			tableTime(sess),
			ansitext.TruncateEnd(sess.Snippet(layout.SnippetWidth*tableSnippetPreTruncateMultiplier), layout.SnippetWidth),
		})
	}
	t := table.New().
		Border(lipgloss.NormalBorder()).
		Width(layout.TableWidth).
		Wrap(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			return lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
		}).
		Headers("SESSION", "NAME", "PATH", "UPDATED", "SNIPPET").
		Rows(rows...)
	_, err := fmt.Fprintln(w, t.Render())
	return err
}

type sessionTable struct {
	TableWidth   int
	SessionWidth int
	NameWidth    int
	PathWidth    int
	SnippetWidth int
}

func sessionTableLayout(w io.Writer) sessionTable {
	width := terminalWidth(w)
	if width <= 0 {
		width = defaultTableWidth
	}
	if width < minTableWidth {
		width = minTableWidth
	}
	contentWidth := width - tableFrameWidth - tableUpdatedWidth
	if contentWidth < minTableContentWidth {
		contentWidth = minTableContentWidth
	}
	sessionWidth := contentWidth * sessionColumnPercent / 100
	nameWidth := contentWidth * nameColumnPercent / 100
	pathWidth := contentWidth * pathColumnPercent / 100
	snippetWidth := contentWidth - sessionWidth - nameWidth - pathWidth
	sessionWidth = maxInt(sessionWidth, minSessionColumnWidth)
	nameWidth = maxInt(nameWidth, minNameColumnWidth)
	pathWidth = maxInt(pathWidth, minPathColumnWidth)
	snippetWidth = maxInt(snippetWidth, minSnippetColumnWidth)
	for sessionWidth+nameWidth+pathWidth+snippetWidth > contentWidth {
		switch {
		case pathWidth > minPathColumnWidth:
			pathWidth--
		case sessionWidth > minSessionColumnWidth:
			sessionWidth--
		case nameWidth > minNameColumnWidth:
			nameWidth--
		default:
			snippetWidth--
		}
	}
	return sessionTable{
		TableWidth:   width,
		SessionWidth: sessionWidth,
		NameWidth:    nameWidth,
		PathWidth:    pathWidth,
		SnippetWidth: snippetWidth,
	}
}

func terminalWidth(w io.Writer) int {
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		width, _, err := term.GetSize(f.Fd())
		if err == nil && width > 0 {
			return width
		}
	}
	columns, err := strconv.Atoi(os.Getenv("COLUMNS"))
	if err == nil && columns > 0 {
		return columns
	}
	return 0
}

func tableColorEnabled(w io.Writer) bool {
	if !colorEnabled() {
		return false
	}
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		return term.IsTerminal(f.Fd())
	}
	return false
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func sessionLabel(sess session.Session, current session.CWDMatcher) string {
	marker := "  "
	if current.Matches(sess) {
		marker = "● "
	}
	label := marker + string(sess.Source)
	if short := shortID(sess.ID); short != "" {
		label += " " + short
	}
	if sess.GitBranch != "" {
		label += " " + sess.GitBranch
	}
	return label
}

func shortID(id string) string {
	runes := []rune(id)
	if len(runes) <= shortSessionIDLength {
		return id
	}
	return string(runes[:shortSessionIDLength])
}
