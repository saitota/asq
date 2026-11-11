package cli

import (
	"os"
	"strings"

	"github.com/saitota/asq/internal/session"
	"github.com/saitota/asq/internal/source"
)

const displayTimeFormat = "2006-01-02T15:04:05Z07:00"
const tableTimeFormat = "01-02 15:04"

func colorEnabled() bool {
	return os.Getenv("NO_COLOR") == ""
}

func colorSource(reg source.Registry, src session.Source, value string) string {
	if !colorEnabled() {
		return value
	}
	return "\x1b[" + reg.ANSIColor(src) + "m" + value + "\x1b[0m"
}

func colorSourceInLabel(reg source.Registry, label string, src session.Source) string {
	sourceText := string(src)
	index := strings.Index(label, sourceText)
	if index < 0 {
		return label
	}
	return label[:index] + colorSource(reg, src, sourceText) + label[index+len(sourceText):]
}

func tildePath(path string) string {
	if path == "" {
		return "."
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == home {
		return "~"
	}
	prefix := home + string(os.PathSeparator)
	if strings.HasPrefix(path, prefix) {
		return "~" + string(os.PathSeparator) + strings.TrimPrefix(path, prefix)
	}
	return path
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func sessionTime(sess session.Session) string {
	return sess.Timestamp.Format(displayTimeFormat)
}

func tableTime(sess session.Session) string {
	return sess.Timestamp.Format(tableTimeFormat)
}
