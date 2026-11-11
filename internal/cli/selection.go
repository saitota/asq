package cli

// This file is the single definition of the fzf-style selection protocol:
// `asq list --fzf` emits these TSV lines and the positional arguments of
// resume/cwd decode them. Everything that depends on the field layout must
// derive from the constants below so a layout change stays in this file.

import (
	"strings"

	"github.com/saitota/asq/internal/ansitext"
	"github.com/saitota/asq/internal/session"
)

// The session ID is deliberately the last field so that when a whole line is
// word-split by the shell (xargs), the ID is always the final argument.
const (
	fzfSourceField = iota
	fzfTimeField
	fzfNameField
	fzfCWDField
	fzfGitBranchField
	fzfSnippetField
	fzfSearchTextField
	fzfIDField
	fzfFieldCount
)

const fzfSnippetLimit = 160
const fzfSearchTextLimit = 32 * 1024

const ansiConcealStart = "\x1b[8m"
const ansiConcealEnd = "\x1b[0m"

// selectionLine holds one session's fields in protocol order. ID and
// SearchText are stored plain and concealed at encode time.
type selectionLine struct {
	Source     string
	Time       string
	Name       string
	CWD        string
	GitBranch  string
	Snippet    string
	ID         string
	SearchText string
}

func newSelectionLine(sess session.Session, color bool) selectionLine {
	source := string(sess.Source)
	if color && colorEnabled() {
		source = colorSource(sess.Source, source)
	}
	return selectionLine{
		Source:     source,
		Time:       sessionTime(sess),
		Name:       sess.Name,
		CWD:        sess.CWD,
		GitBranch:  sess.GitBranch,
		Snippet:    sess.Snippet(fzfSnippetLimit),
		ID:         sess.ID,
		SearchText: ansitext.Strip(sess.SearchText(fzfSearchTextLimit)),
	}
}

func (l selectionLine) encode() string {
	fields := make([]string, fzfFieldCount)
	fields[fzfSourceField] = l.Source
	fields[fzfTimeField] = l.Time
	fields[fzfNameField] = l.Name
	fields[fzfCWDField] = l.CWD
	fields[fzfGitBranchField] = l.GitBranch
	fields[fzfSnippetField] = l.Snippet
	fields[fzfIDField] = conceal(l.ID)
	fields[fzfSearchTextField] = conceal(l.SearchText)
	return strings.Join(fields, "\t")
}

// selectionKeyFromSelection accepts a bare id or a whole selection line.
func selectionKeyFromSelection(selection string) string {
	selection = ansitext.Strip(selection)
	fields := strings.Split(strings.TrimSpace(selection), "\t")
	if len(fields) > 1 {
		return strings.TrimSpace(fields[len(fields)-1])
	}
	return strings.TrimSpace(selection)
}

// selectionKeyFromArgs takes a session id, or a selection line word-split
// by the shell (`asq list --fzf | fzf | xargs asq resume`), where the ID is
// the last argument.
func selectionKeyFromArgs(args []string) string {
	if len(args) == 1 {
		return selectionKeyFromSelection(args[0])
	}
	return ansitext.Strip(args[len(args)-1])
}

func conceal(value string) string {
	if value == "" {
		return ""
	}
	return ansiConcealStart + value + ansiConcealEnd
}
