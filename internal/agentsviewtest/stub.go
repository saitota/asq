// Package agentsviewtest provides the single stub agentsview binary shared
// by tests, so the CLI contract asq assumes is faked in exactly one place.
package agentsviewtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type Stub struct {
	Bin string
	dir string
	log string
}

// Install writes the stub binary and points ASQ_AGENTSVIEW_BIN at it. The
// script resolves --from by flag name and uses absolute tool paths because
// some tests replace PATH.
func Install(t *testing.T) *Stub {
	t.Helper()
	dir := t.TempDir()
	s := &Stub{Bin: filepath.Join(dir, "agentsview"), dir: dir, log: filepath.Join(dir, "calls.log")}
	script := `#!/bin/sh
echo "$@" >> "` + s.log + `"
case "$1:$2" in
session:list)
  /bin/cat "` + dir + `/list.json" ;;
session:messages)
  from=""
  prev=""
  for arg in "$@"; do
    if [ "$prev" = "--from" ]; then from="$arg"; fi
    prev="$arg"
  done
  if [ -f "` + dir + `/messages-$3-from$from.json" ]; then /bin/cat "` + dir + `/messages-$3-from$from.json"
  else echo "session not found" >&2; exit 1; fi ;;
*) echo "unexpected command" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(s.Bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASQ_AGENTSVIEW_BIN", s.Bin)
	return s
}

func (s *Stub) SetList(t *testing.T, sessions ...map[string]any) {
	t.Helper()
	s.writeJSON(t, "list.json", map[string]any{"sessions": sessions, "next_cursor": ""})
}

// SetMessages makes the whole conversation one page followed by the empty
// page that marks its end.
func (s *Stub) SetMessages(t *testing.T, id string, messages ...map[string]any) {
	t.Helper()
	last := len(messages) - 1
	s.SetMessagesPage(t, id, 0, last, messages...)
	s.SetMessagesPage(t, id, last+1, last)
}

func (s *Stub) SetMessagesPage(t *testing.T, id string, from, lastOrdinal int, messages ...map[string]any) {
	t.Helper()
	if messages == nil {
		messages = []map[string]any{}
	}
	s.writeJSON(t, fmt.Sprintf("messages-%s-from%d.json", id, from), map[string]any{"messages": messages, "last_ordinal": lastOrdinal})
}

// ClearMessages removes every canned messages page so fetches fail.
func (s *Stub) ClearMessages(t *testing.T) {
	t.Helper()
	pages, err := filepath.Glob(filepath.Join(s.dir, "messages-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range pages {
		if err := os.Remove(page); err != nil {
			t.Fatal(err)
		}
	}
}

// Break replaces the stub with a failing script.
func (s *Stub) Break(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(s.Bin, []byte("#!/bin/sh\necho daemon exploded >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// MessageFetches counts `session messages` invocations.
func (s *Stub) MessageFetches(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile(s.log)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "session messages") {
			count++
		}
	}
	return count
}

func (s *Stub) writeJSON(t *testing.T, name string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Message is a canned user/assistant message.
func Message(role, content, timestamp string) map[string]any {
	return map[string]any{"role": role, "content": content, "timestamp": timestamp}
}

// Session is a canned `session list` entry.
func Session(id, agent, name, cwd, firstMessage string, overrides ...map[string]any) map[string]any {
	entry := map[string]any{
		"id":            id,
		"agent":         agent,
		"display_name":  name,
		"first_message": firstMessage,
		"ended_at":      "2026-01-02T00:00:00Z",
		"message_count": 1,
		"cwd":           cwd,
		"git_branch":    "main",
	}
	for _, override := range overrides {
		for key, value := range override {
			entry[key] = value
		}
	}
	return entry
}
