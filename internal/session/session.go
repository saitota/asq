package session

import (
	"strconv"
	"strings"
	"time"
)

type Source string

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role      Role
	Content   string
	Timestamp time.Time
}

// PRLink is a pull request agentsview linked to the session.
type PRLink struct {
	URL        string
	Host       string
	Repository string
	Number     int
}

type Session struct {
	ID        string
	Source    Source
	Name      string
	CWD       string
	GitBranch string
	Timestamp time.Time
	PRLinks   []PRLink
	Messages  []Message
	Errors    []string
}

func Normalize(s Session) Session {
	out := s
	out.Messages = JoinConsecutiveMessages(s.Messages)
	out.Messages = DropEmptyMessages(out.Messages)
	if out.CWD == "" {
		out.CWD = "."
	}
	if out.Timestamp.IsZero() {
		for _, msg := range out.Messages {
			if msg.Timestamp.After(out.Timestamp) {
				out.Timestamp = msg.Timestamp
			}
		}
	}
	return out
}

// HasCWD reports whether the session's cwd was decoded. Normalize stores "."
// as the sentinel for unknown, so both "" and "." count as missing.
func (s Session) HasCWD() bool {
	return s.CWD != "" && s.CWD != "."
}

func JoinConsecutiveMessages(messages []Message) []Message {
	out := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if len(out) > 0 && out[len(out)-1].Role == msg.Role {
			out[len(out)-1].Content += "\n\n" + msg.Content
			out[len(out)-1].Timestamp = msg.Timestamp
			continue
		}
		out = append(out, msg)
	}
	return out
}

func DropEmptyMessages(messages []Message) []Message {
	out := messages[:0]
	for _, msg := range messages {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		out = append(out, msg)
	}
	return out
}

func (s Session) Snippet(limit int) string {
	for _, msg := range s.Messages {
		text := strings.Join(strings.Fields(msg.Content), " ")
		if text == "" {
			continue
		}
		runes := []rune(text)
		if len(runes) > limit {
			return string(runes[:limit]) + "..."
		}
		return text
	}
	return ""
}

func (s Session) SearchText(limit int) string {
	var out strings.Builder
	remaining := limit
	write := func(value string) bool {
		if limit <= 0 {
			out.WriteString(value)
			return false
		}
		for _, r := range value {
			if remaining <= 0 {
				return true
			}
			out.WriteRune(r)
			remaining--
		}
		return remaining <= 0
	}
	for _, msg := range s.Messages {
		text := strings.Join(strings.Fields(msg.Content), " ")
		if text == "" {
			continue
		}
		if out.Len() > 0 {
			if write(" ") {
				return out.String()
			}
		}
		if write(string(msg.Role)) {
			return out.String()
		}
		if write(" ") {
			return out.String()
		}
		if write(text) {
			return out.String()
		}
	}
	return out.String()
}

// PRSearchText lists every spelling a linked PR can be searched by:
// owner/repo#N, #N, pull/N and the URL.
func (s Session) PRSearchText() string {
	parts := make([]string, 0, len(s.PRLinks)*3)
	for _, link := range s.PRLinks {
		if link.Number > 0 {
			ref := "#" + strconv.Itoa(link.Number)
			if link.Repository != "" {
				ref = link.Repository + ref
			}
			parts = append(parts, ref, "pull/"+strconv.Itoa(link.Number))
		}
		if link.URL != "" {
			parts = append(parts, link.URL)
		}
	}
	return strings.Join(parts, " ")
}
