package cli

import (
	"time"

	"github.com/saitota/asq/internal/session"
)

type listOutput struct {
	Sessions []listSession `json:"sessions"`
}

type listSession struct {
	ID        string        `json:"id"`
	Source    string        `json:"source"`
	Name      string        `json:"name"`
	CWD       string        `json:"cwd"`
	GitBranch string        `json:"git_branch,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Messages  []listMessage `json:"messages"`
	Errors    []string      `json:"errors,omitempty"`
}

type listMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func newListOutput(sessions []session.Session) listOutput {
	out := listOutput{Sessions: make([]listSession, 0, len(sessions))}
	for _, sess := range sessions {
		out.Sessions = append(out.Sessions, newListSession(sess))
	}
	return out
}

func newListSession(sess session.Session) listSession {
	return listSession{
		ID:        sess.ID,
		Source:    string(sess.Source),
		Name:      sess.Name,
		CWD:       sess.CWD,
		GitBranch: sess.GitBranch,
		Timestamp: sess.Timestamp,
		Messages:  newListMessages(sess.Messages),
		Errors:    sess.Errors,
	}
}

func newListMessages(messages []session.Message) []listMessage {
	if messages == nil {
		return nil
	}
	out := make([]listMessage, len(messages))
	for i, msg := range messages {
		out[i] = listMessage{Role: string(msg.Role), Content: msg.Content, Timestamp: msg.Timestamp}
	}
	return out
}
