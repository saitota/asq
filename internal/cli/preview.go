package cli

import (
	"fmt"
	"strings"

	"github.com/saitota/asq/internal/session"
	"github.com/saitota/asq/internal/source"
)

const previewMessageLimit = 80
const previewMessageCharLimit = 2000

// Preview metadata is set apart from plain log content two ways: header
// fields render as gray text, and message role/time lines render as
// background bars — gray for user turns, the source color for assistant
// turns.
//
// previewBar and previewMetaLine each emit exactly one SGR sequence per
// value, immediately closed with a reset — never layered styles (e.g. bold
// plus color). tui's ansitext.Wrap does support reopening multiple layered
// styles across a wrap point, but this package intentionally stays within
// the single-style-per-line subset so the two packages' contract stays
// simple and easy to verify by inspection.
const previewMetaBg = "156;163;175"
const previewMetaFg = "17;24;39"

func previewBar(bgRGB, value string) string {
	return "\x1b[48;2;" + bgRGB + ";38;2;" + previewMetaFg + "m" + value + "\x1b[0m"
}

func previewMetaLine(value string) string {
	return "\x1b[38;2;" + previewMetaBg + "m" + value + "\x1b[0m"
}

func sourceRGB(src session.Source) string {
	return strings.TrimPrefix(source.ANSIColor(src), "38;2;")
}

type previewStyle struct {
	meta func(text string) string
	bar  func(rgb, text string) string
	body func(text string) string
}

var plainPreviewStyle = previewStyle{
	meta: func(text string) string { return text },
	bar:  func(_, text string) string { return text },
	body: func(text string) string { return text },
}

var coloredPreviewStyle = previewStyle{
	meta: func(text string) string { return previewMetaLine(sanitizeDisplay(text)) },
	bar:  previewBar,
	body: sanitizePreview,
}

func renderPreview(sess session.Session, style previewStyle) string {
	var out strings.Builder
	_, _ = fmt.Fprintf(&out, "%s\n", style.meta(fmt.Sprintf("%s %s", sess.Source, sess.ID)))
	if sess.Name != "" {
		_, _ = fmt.Fprintf(&out, "%s\n", style.meta("name: "+sess.Name))
	}
	_, _ = fmt.Fprintf(&out, "%s\n", style.meta("cwd: "+sess.CWD))
	if sess.GitBranch != "" {
		_, _ = fmt.Fprintf(&out, "%s\n", style.meta("branch: "+sess.GitBranch))
	}
	for _, warning := range sess.Errors {
		_, _ = fmt.Fprintf(&out, "%s\n", style.meta("warning: "+warning))
	}
	_, _ = fmt.Fprintf(&out, "%s\n\n", style.meta("updated: "+tableTime(sess)))
	srcRGB := sourceRGB(sess.Source)
	for i, msg := range sess.Messages {
		if i >= previewMessageLimit {
			_, _ = fmt.Fprintf(&out, "%s\n", style.meta(fmt.Sprintf("... %d more messages", len(sess.Messages)-i)))
			break
		}
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}
		barRGB := srcRGB
		if msg.Role == session.RoleUser {
			barRGB = previewMetaBg
		}
		header := fmt.Sprintf("[%d] %s %s", i+1, msg.Role, tableTime(session.Session{Timestamp: msg.Timestamp}))
		_, _ = fmt.Fprintf(&out, "%s\n", style.bar(barRGB, header))
		_, _ = fmt.Fprintln(&out, style.body(truncateRunes(text, previewMessageCharLimit)))
		_, _ = fmt.Fprintln(&out)
	}
	return out.String()
}

func pickerPreviewText(sess session.Session) string {
	if !colorEnabled() {
		return sanitizePreview(previewText(sess))
	}
	return renderPreview(sess, coloredPreviewStyle)
}

func previewText(sess session.Session) string {
	return renderPreview(sess, plainPreviewStyle)
}
