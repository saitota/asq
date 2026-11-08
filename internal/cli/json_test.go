package cli

import (
	"encoding/json"
	"testing"
	"time"

	"asq/internal/session"
	"asq/internal/source"
)

func TestListOutputEncoding(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name     string
		sessions []session.Session
		want     string
	}{
		{"no sessions", nil, `{"sessions":[]}`},
		{
			"nil messages",
			[]session.Session{{ID: "a", Source: source.Claude, CWD: ".", Timestamp: ts}},
			`{"sessions":[{"id":"a","source":"claude","name":"","cwd":".","timestamp":"2026-01-02T03:04:05Z","messages":null}]}`,
		},
		{
			"empty messages",
			[]session.Session{{ID: "a", Source: source.Claude, CWD: ".", Timestamp: ts, Messages: []session.Message{}}},
			`{"sessions":[{"id":"a","source":"claude","name":"","cwd":".","timestamp":"2026-01-02T03:04:05Z","messages":[]}]}`,
		},
		{
			"full",
			[]session.Session{{
				ID: "a", Source: source.Codex, Name: "n", CWD: "/p", GitBranch: "main", Timestamp: ts,
				Messages: []session.Message{{Role: session.RoleUser, Content: "hi", Timestamp: ts}},
				Errors:   []string{"boom"},
			}},
			`{"sessions":[{"id":"a","source":"codex","name":"n","cwd":"/p","git_branch":"main","timestamp":"2026-01-02T03:04:05Z","messages":[{"role":"user","content":"hi","timestamp":"2026-01-02T03:04:05Z"}],"errors":["boom"]}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(newListOutput(tt.sessions))
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.want {
				t.Fatalf("got  %s\nwant %s", data, tt.want)
			}
		})
	}
}
