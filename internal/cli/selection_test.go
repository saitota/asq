package cli

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/saitota/asq/internal/session"
	"github.com/saitota/asq/internal/source"
)

func TestSelectionLineFieldsMatchProtocolCount(t *testing.T) {
	if got := reflect.TypeFor[selectionLine]().NumField(); got != fzfFieldCount {
		t.Fatalf("selectionLine has %d fields, protocol declares %d", got, fzfFieldCount)
	}
}

func TestSelectionLineEncodeDecodeRoundTrip(t *testing.T) {
	sess := session.Session{
		ID:        "sid",
		Source:    source.Codex,
		Name:      "Session name",
		CWD:       "/tmp/proj",
		GitBranch: "main",
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Messages: []session.Message{{
			Role:    session.RoleUser,
			Content: "hello",
		}},
	}
	encoded := newSelectionLine(sess, true).encode()
	if got := selectionKeyFromSelection(encoded); got != sess.ID {
		t.Fatalf("selectionKeyFromSelection(encode()) = %q, want %q", got, sess.ID)
	}
	if got := len(strings.Split(encoded, "\t")); got != fzfFieldCount {
		t.Fatalf("encoded field count = %d, want %d", got, fzfFieldCount)
	}
}
