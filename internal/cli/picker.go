package cli

import (
	"errors"
	"strings"

	"github.com/saitota/asq/internal/ansitext"
	"github.com/saitota/asq/internal/session"
	"github.com/saitota/asq/internal/tui"
)

// Field budgets keep every picker column visible in the list pane: the path
// is middle-truncated so the distinctive tail survives, and the snippet fills
// whatever remains.
const pickerNameWidth = 24
const pickerPathWidth = 28
const pickerBranchWidth = 20

const pickerSnippetLimit = 160
const pickerSearchTextLimit = 32 * 1024

func (a App) pickSession(sessions []session.Session, query string) (session.Session, error) {
	sessions = session.Filter(sessions, session.FilterOptions{})
	if len(sessions) == 0 {
		return session.Session{}, errors.New("no sessions found")
	}
	items := make([]tui.Item, len(sessions))
	for i, sess := range sessions {
		items[i] = tui.Item{Display: pickerLine(sess), Haystack: pickerHaystack(sess)}
	}
	pick := a.Pick
	if pick == nil {
		pick = tui.Pick
	}
	choice, err := pick(items, func(index int) string { return pickerPreviewText(sessions[index]) }, query)
	if err != nil {
		return session.Session{}, err
	}
	return sessions[choice], nil
}

func pickerLine(sess session.Session) string {
	fields := []string{pickerSourceMarker(sess.Source), tableTime(sess)}
	if name := sanitizeDisplay(sess.Name); name != "" {
		fields = append(fields, ansitext.TruncateEnd(name, pickerNameWidth))
	}
	fields = append(fields, ansitext.TruncateMiddle(sanitizeDisplay(tildePath(sess.CWD)), pickerPathWidth))
	if branch := sanitizeDisplay(sess.GitBranch); branch != "" {
		fields = append(fields, ansitext.TruncateEnd(branch, pickerBranchWidth))
	}
	if snippet := sanitizeDisplay(sess.Snippet(pickerSnippetLimit)); snippet != "" {
		fields = append(fields, snippet)
	}
	return strings.Join(fields, "  ")
}

// sanitizeDisplay removes ANSI sequences and control characters that session
// content could otherwise inject into the picker UI.
func sanitizeDisplay(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, ansitext.Strip(value))
}

func sanitizePreview(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = sanitizeDisplay(line)
	}
	return strings.Join(lines, "\n")
}

// pickerSourceMarker identifies the source by color alone to save row width;
// without colors it falls back to the source name.
func pickerSourceMarker(source session.Source) string {
	if !colorEnabled() {
		return string(source)
	}
	return colorSource(source, "●")
}

// pickerHaystack is the lowercased text queries match against: every visible
// field plus the session id and full conversation text.
func pickerHaystack(sess session.Session) string {
	parts := []string{
		string(sess.Source),
		sessionTime(sess),
		sess.Name,
		sess.CWD,
		sess.GitBranch,
		sess.ID,
		ansitext.Strip(sess.SearchText(pickerSearchTextLimit)),
	}
	return strings.ToLower(strings.Join(parts, " "))
}
