package agentsview

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"asq/internal/session"
)

const goldenCacheJSON = `{"version":2,"entries":{"codex:0000-1111":{"signature":"sig-1","session":{"id":"0000-1111","source":"codex","name":"tfupdate-series","cwd":"/tmp/proj","git_branch":"main","timestamp":"2026-09-01T03:06:13Z","messages":[{"role":"user","content":"PRつくって","timestamp":"2026-08-31T09:42:43Z"},{"role":"assistant","content":"作りました","timestamp":"2026-08-31T09:43:00Z"}]}}}}`

func goldenCacheFile() cacheFile {
	return cacheFile{
		Version: cacheVersion,
		Entries: map[string]cacheEntry{
			"codex:0000-1111": {
				Signature: "sig-1",
				Session: cachedSession{
					ID:        "0000-1111",
					Source:    "codex",
					Name:      "tfupdate-series",
					CWD:       "/tmp/proj",
					GitBranch: "main",
					Timestamp: time.Date(2026, 9, 1, 3, 6, 13, 0, time.UTC),
					Messages: []cachedMessage{
						{Role: "user", Content: "PRつくって", Timestamp: time.Date(2026, 8, 31, 9, 42, 43, 0, time.UTC)},
						{Role: "assistant", Content: "作りました", Timestamp: time.Date(2026, 8, 31, 9, 43, 0, 0, time.UTC)},
					},
				},
			},
		},
	}
}

func TestCacheWireFormatGolden(t *testing.T) {
	data, err := json.Marshal(goldenCacheFile())
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != goldenCacheJSON {
		t.Fatalf("cache JSON =\n%s\nwant\n%s", got, goldenCacheJSON)
	}
	var out cacheFile
	if err := json.Unmarshal([]byte(goldenCacheJSON), &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, goldenCacheFile()) {
		t.Fatalf("round-tripped cacheFile = %+v, want %+v", out, goldenCacheFile())
	}
}

func TestCachedSessionRoundTrip(t *testing.T) {
	cases := []session.Session{
		{
			ID:        "0000-1111",
			Source:    session.Source("codex"),
			Name:      "tfupdate-series",
			CWD:       "/tmp/proj",
			GitBranch: "main",
			Timestamp: time.Date(2026, 9, 1, 3, 6, 13, 0, time.UTC),
			Messages: []session.Message{
				{Role: session.RoleUser, Content: "PRつくって", Timestamp: time.Date(2026, 8, 31, 9, 42, 43, 0, time.UTC)},
				{Role: session.RoleAssistant, Content: "作りました", Timestamp: time.Date(2026, 8, 31, 9, 43, 0, 0, time.UTC)},
			},
		},
		{
			ID:        "8a36c6da-48e4",
			Source:    session.Source("claude"),
			Name:      "",
			CWD:       "/tmp/other",
			GitBranch: "",
			Timestamp: time.Date(2026, 9, 1, 3, 8, 47, 0, time.UTC),
			Messages: []session.Message{
				{Role: session.RoleUser, Content: "hello", Timestamp: time.Date(2026, 9, 1, 3, 8, 47, 0, time.UTC)},
			},
		},
	}
	for _, want := range cases {
		got := newCachedSession(want).toSession()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("round trip = %+v, want %+v", got, want)
		}
	}
}

func TestLoadCacheReadsGoldenFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte(goldenCacheJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadCache(path)
	entry, ok := got.Entries["codex:0000-1111"]
	if !ok {
		t.Fatalf("entries = %+v", got.Entries)
	}
	want := []session.Message{
		{Role: session.RoleUser, Content: "PRつくって", Timestamp: time.Date(2026, 8, 31, 9, 42, 43, 0, time.UTC)},
		{Role: session.RoleAssistant, Content: "作りました", Timestamp: time.Date(2026, 8, 31, 9, 43, 0, 0, time.UTC)},
	}
	if got := entry.Session.toMessages(); !reflect.DeepEqual(got, want) {
		t.Fatalf("messages = %+v, want %+v", got, want)
	}
}
