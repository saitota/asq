package cli

import (
	"strings"

	"github.com/saitota/asq/internal/ansitext"
)

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
