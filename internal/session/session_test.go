package session

import (
	"strings"
	"testing"
)

func TestSearchTextIncludesAllMessages(t *testing.T) {
	sess := Session{Messages: []Message{
		{Role: RoleUser, Content: "first message"},
		{Role: RoleAssistant, Content: "deep hidden answer"},
	}}
	got := sess.SearchText(1000)
	for _, want := range []string{"user first message", "assistant deep hidden answer"} {
		if !strings.Contains(got, want) {
			t.Fatalf("SearchText = %q, want %q", got, want)
		}
	}
}

func TestSearchTextRespectsLimit(t *testing.T) {
	sess := Session{Messages: []Message{
		{Role: RoleUser, Content: "abcdefghij"},
	}}
	if got := sess.SearchText(8); got != "user abc" {
		t.Fatalf("SearchText limit = %q", got)
	}
}
