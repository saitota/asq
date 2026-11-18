package cli

import (
	"strings"
	"testing"

	"github.com/saitota/asq/internal/session"
)

func TestPickerHaystackIncludesTextBeyondFZFLimit(t *testing.T) {
	sess := session.Session{Messages: []session.Message{
		{Role: session.RoleUser, Content: strings.Repeat("x", fzfSearchTextLimit+10)},
		{Role: session.RoleAssistant, Content: "late-pull-marker"},
	}}
	if !strings.Contains(pickerHaystack(sess), "late-pull-marker") {
		t.Fatal("picker haystack misses text after the first 32K characters")
	}
}
