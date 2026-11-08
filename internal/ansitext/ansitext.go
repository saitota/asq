// Package ansitext provides width-aware operations (strip, width, truncate,
// wrap, pad) over text that may contain ANSI SGR escape sequences. It is the
// single place that understands the escape grammar, so every caller measures
// and cuts text the same way regardless of how many styles are layered on.
package ansitext

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// resetSequence is the SGR reset; Wrap and TruncateEnd/TruncateMiddle append
// it whenever a cut leaves a style open, so no style can bleed past the cut.
const resetSequence = "\x1b[0m"

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// Strip removes ANSI escape sequences from value.
func Strip(value string) string {
	return ansiEscapePattern.ReplaceAllString(value, "")
}

// Width returns the display width of value, ignoring ANSI escape sequences.
func Width(value string) int {
	return lipgloss.Width(value)
}

// SequenceLen reports the byte length of the CSI escape sequence at the start
// of s, or 0 when s does not start with one.
func SequenceLen(s string) int {
	if len(s) < 2 || s[0] != 0x1b || s[1] != '[' {
		return 0
	}
	i := 2
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == ';' || s[i] == '?') {
		i++
	}
	for i < len(s) && s[i] >= 0x20 && s[i] <= 0x2f {
		i++
	}
	if i < len(s) && s[i] >= 0x40 && s[i] <= 0x7e {
		return i + 1
	}
	return 0
}

// TruncateEnd cuts value to the given display width, appending "...". ANSI
// escape sequences are copied whole and contribute no width, so styled
// content is never split mid-sequence; a reset is appended before "..." when
// styled text was cut.
func TruncateEnd(value string, limit int) string {
	if Width(value) <= limit {
		return value
	}
	suffix := "..."
	target := limit - len(suffix)
	if target <= 0 {
		if limit < 0 {
			limit = 0
		}
		return suffix[:min(limit, len(suffix))]
	}
	out, _ := cutDisplay(value, target)
	return out + suffix
}

// CutDisplay copies value up to the given display width without adding a
// truncation marker, preserving whole ANSI escape sequences and appending a
// reset when a style was left open by the cut. Used where the caller wraps
// or lays out the remainder itself instead of ending the string outright.
func CutDisplay(value string, width int) string {
	out, _ := cutDisplay(value, width)
	return out
}

// TruncateMiddle cuts the center out of value, keeping a prefix and suffix
// that together fit limit, joined by "...".
func TruncateMiddle(value string, limit int) string {
	if Width(value) <= limit {
		return value
	}
	suffix := "..."
	target := limit - len(suffix)
	if target <= 0 {
		if limit < 0 {
			limit = 0
		}
		return suffix[:min(limit, len(suffix))]
	}
	leftWidth := target / 2
	rightWidth := target - leftWidth
	left, _ := cutDisplay(value, leftWidth)
	right := takeRight(value, rightWidth)
	return left + suffix + right
}

// Pad right-pads value with spaces until it reaches width, measuring by
// display width so ANSI escape sequences do not count toward padding.
func Pad(value string, width int) string {
	gap := width - Width(value)
	if gap <= 0 {
		return value
	}
	return value + strings.Repeat(" ", gap)
}

// cutDisplay copies value up to the given display width, preserving whole
// ANSI escape sequences, and reports whether any styling was seen. A reset is
// appended when a style was left open by the cut.
func cutDisplay(value string, width int) (string, bool) {
	var out strings.Builder
	current := 0
	sawANSI := false
	for i := 0; i < len(value); {
		if value[i] == 0x1b {
			if seq := SequenceLen(value[i:]); seq > 0 {
				out.WriteString(value[i : i+seq])
				sawANSI = true
				i += seq
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		w := Width(string(r))
		if current+w > width {
			break
		}
		out.WriteRune(r)
		current += w
		i += size
	}
	if sawANSI {
		out.WriteString(resetSequence)
	}
	return out.String(), sawANSI
}

// takeRight returns the display-width-limited suffix of value, measured from
// the end. Unlike cutDisplay it does not track ANSI sequences because callers
// use it only on plain (already-stripped) text.
func takeRight(value string, limit int) string {
	runes := []rune(value)
	var parts []rune
	width := 0
	for i := len(runes) - 1; i >= 0; i-- {
		runeWidth := Width(string(runes[i]))
		if width+runeWidth > limit {
			break
		}
		parts = append(parts, runes[i])
		width += runeWidth
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return string(parts)
}

// Wrap hard-wraps text at the given display width, like fzf's
// --preview-window wrap. ANSI escape sequences are copied whole with zero
// width. All SGR styles active at a wrap point are closed at the line end and
// reopened on the continuation, so any number of layered styles (bold+color,
// etc.) survive a wrap without bleeding into neighboring content.
func Wrap(text string, width int) []string {
	var lines []string
	for _, raw := range strings.Split(text, "\n") {
		lines = append(lines, wrapLine(raw, width)...)
	}
	return lines
}

func wrapLine(raw string, width int) []string {
	if raw == "" {
		return []string{""}
	}
	var lines []string
	var current strings.Builder
	currentWidth := 0
	active := newActiveStyles()
	for i := 0; i < len(raw); {
		if raw[i] == 0x1b {
			if seq := SequenceLen(raw[i:]); seq > 0 {
				style := raw[i : i+seq]
				current.WriteString(style)
				if strings.HasSuffix(style, "m") {
					active.apply(style)
				}
				i += seq
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(raw[i:])
		w := Width(string(r))
		if currentWidth+w > width && currentWidth > 0 {
			line := current.String()
			if !active.empty() {
				line += resetSequence
			}
			lines = append(lines, line)
			current.Reset()
			currentWidth = 0
			current.WriteString(active.reopen())
		}
		current.WriteRune(r)
		currentWidth += w
		i += size
	}
	line := current.String()
	if !active.empty() && !strings.HasSuffix(line, resetSequence) {
		line += resetSequence
	}
	lines = append(lines, line)
	return lines
}

// activeStyles tracks the SGR sequences currently open, in application
// order, so a wrap point can close and reopen every layered style (e.g.
// bold + a color) rather than assuming only one is ever active.
type activeStyles struct {
	sequences []string
}

func newActiveStyles() *activeStyles {
	return &activeStyles{}
}

func (a *activeStyles) apply(style string) {
	if style == resetSequence || style == "\x1b[m" {
		a.sequences = a.sequences[:0]
		return
	}
	a.sequences = append(a.sequences, style)
}

func (a *activeStyles) empty() bool {
	return len(a.sequences) == 0
}

func (a *activeStyles) reopen() string {
	return strings.Join(a.sequences, "")
}
