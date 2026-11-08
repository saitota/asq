package agentsview

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"asq/internal/agentsviewtest"
	"asq/internal/session"
)

type recordingReporter struct {
	mu     sync.Mutex
	events []string
}

func (r *recordingReporter) record(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recordingReporter) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.events))
	copy(out, r.events)
	return out
}

func (r *recordingReporter) ListStarted()  { r.record("list-started") }
func (r *recordingReporter) ListFinished() { r.record("list-finished") }
func (r *recordingReporter) FetchStarted(total int) {
	r.record(fmt.Sprintf("fetch-started %d", total))
}
func (r *recordingReporter) FetchProgress(done, total int) {
	r.record(fmt.Sprintf("fetch-progress %d/%d", done, total))
}
func (r *recordingReporter) FetchFinished(done int) {
	r.record(fmt.Sprintf("fetch-finished %d", done))
}
func (r *recordingReporter) Warn(message string) {
	r.record(fmt.Sprintf("warn: %s", message))
}

func codexEntry(overrides ...map[string]any) map[string]any {
	entry := agentsviewtest.Session("codex:0000-1111", "codex", "tfupdate-series", "/tmp/proj", "PRつくって",
		map[string]any{"ended_at": "2026-09-01T03:06:13.618Z", "message_count": 2})
	for _, override := range overrides {
		for key, value := range override {
			entry[key] = value
		}
	}
	return entry
}

func codexMessage() map[string]any {
	return agentsviewtest.Message("user", "PRつくって", "2026-08-31T09:42:43.808Z")
}

func TestSessionsMapsListedSessions(t *testing.T) {
	stub := agentsviewtest.Install(t)
	stub.SetList(t,
		codexEntry(),
		agentsviewtest.Session("8a36c6da-48e4", "claude", "", "/tmp/other", "hello",
			map[string]any{"source_session_id": "8a36c6da-48e4"}),
		agentsviewtest.Session("cowork:aaaa", "cowork", "", "/tmp/cowork", "non-resumable agent"),
	)
	stub.SetMessages(t, "codex:0000-1111",
		codexMessage(),
		agentsviewtest.Message("assistant", "作りました", "2026-08-31T09:43:00Z"),
	)
	stub.SetMessages(t, "8a36c6da-48e4", agentsviewtest.Message("user", "hello", "2026-09-01T03:08:47.543Z"))
	stub.SetMessages(t, "cowork:aaaa", agentsviewtest.Message("user", "non-resumable agent", "2026-09-01T00:00:00Z"))
	got, err := Client{Bin: stub.Bin}.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("sessions = %+v", got)
	}
	byID := map[string]session.Session{}
	for _, sess := range got {
		byID[sess.ID] = sess
	}
	codex, ok := byID["0000-1111"]
	if !ok {
		t.Fatalf("codex id prefix not stripped: %+v", got)
	}
	if codex.Source != session.Source("codex") || codex.Name != "tfupdate-series" || codex.CWD != "/tmp/proj" || codex.GitBranch != "main" {
		t.Fatalf("codex session = %+v", codex)
	}
	if codex.Timestamp.IsZero() || len(codex.Messages) != 2 || codex.Messages[1].Content != "作りました" {
		t.Fatalf("codex session = %+v", codex)
	}
	if _, ok := byID["8a36c6da-48e4"]; !ok {
		t.Fatalf("claude session missing: %+v", got)
	}
	cowork, ok := byID["aaaa"]
	if !ok || cowork.Source != session.Source("cowork") {
		t.Fatalf("non-resumable agent session missing: %+v", got)
	}
}

func TestSessionsCachesMessagesBySignature(t *testing.T) {
	stub := agentsviewtest.Install(t)
	cache := filepath.Join(t.TempDir(), "agentsview-messages.json")
	stub.SetList(t, codexEntry())
	stub.SetMessages(t, "codex:0000-1111", codexMessage())
	client := Client{Bin: stub.Bin, CachePath: cache}
	if _, err := client.Sessions(); err != nil {
		t.Fatal(err)
	}
	firstRun := stub.MessageFetches(t)
	if firstRun == 0 {
		t.Fatal("first run fetched nothing")
	}
	got, err := client.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if fetches := stub.MessageFetches(t); fetches != firstRun {
		t.Fatalf("second run fetched again: %d -> %d", firstRun, fetches)
	}
	if len(got) != 1 || got[0].Messages[0].Content != "PRつくって" {
		t.Fatalf("cached session = %+v", got)
	}
}

func TestSessionsRefetchesWhenSignatureChanges(t *testing.T) {
	stub := agentsviewtest.Install(t)
	cache := filepath.Join(t.TempDir(), "agentsview-messages.json")
	stub.SetList(t, codexEntry())
	stub.SetMessages(t, "codex:0000-1111", codexMessage())
	client := Client{Bin: stub.Bin, CachePath: cache}
	if _, err := client.Sessions(); err != nil {
		t.Fatal(err)
	}
	firstRun := stub.MessageFetches(t)
	stub.SetList(t, codexEntry(map[string]any{"ended_at": "2026-09-01T05:00:00Z", "message_count": 3}))
	stub.SetMessages(t, "codex:0000-1111",
		codexMessage(),
		agentsviewtest.Message("assistant", "更新しました", "2026-09-01T05:00:00Z"),
	)
	got, err := client.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if fetches := stub.MessageFetches(t); fetches <= firstRun {
		t.Fatalf("signature change did not refetch: %d -> %d", firstRun, fetches)
	}
	if len(got) != 1 || len(got[0].Messages) != 2 {
		t.Fatalf("refetched session = %+v", got)
	}
}

func TestSessionsFallBackToFirstMessageWhenFetchFails(t *testing.T) {
	stub := agentsviewtest.Install(t)
	cache := filepath.Join(t.TempDir(), "agentsview-messages.json")
	stub.SetList(t, codexEntry())
	got, err := Client{Bin: stub.Bin, CachePath: cache}.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Messages[0].Content != "PRつくって" || len(got[0].Errors) != 1 {
		t.Fatalf("fallback session = %+v", got)
	}
	if entries := loadCache(cache); len(entries.Entries) != 0 {
		t.Fatalf("failed fetch was cached: %+v", entries.Entries)
	}
}

func TestSessionsPrunesCacheEntriesForRemovedSessions(t *testing.T) {
	stub := agentsviewtest.Install(t)
	cache := filepath.Join(t.TempDir(), "agentsview-messages.json")
	stub.SetList(t, codexEntry())
	stub.SetMessages(t, "codex:0000-1111", codexMessage())
	client := Client{Bin: stub.Bin, CachePath: cache}
	if _, err := client.Sessions(); err != nil {
		t.Fatal(err)
	}
	stub.SetList(t)
	if _, err := client.Sessions(); err != nil {
		t.Fatal(err)
	}
	if entries := loadCache(cache); len(entries.Entries) != 0 {
		t.Fatalf("stale cache entries remain: %+v", entries.Entries)
	}
}

func TestSessionsReportsMissingBinary(t *testing.T) {
	_, err := Client{Bin: filepath.Join(t.TempDir(), "missing")}.Sessions()
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestRawIDPrefersSourceSessionIDThenPrefix(t *testing.T) {
	cases := []struct {
		item listedSession
		want string
	}{
		{listedSession{ID: "codex:abc", Agent: "codex"}, "abc"},
		{listedSession{ID: "codex:abc", Agent: "codex", SourceSessionID: "xyz"}, "xyz"},
		{listedSession{ID: "8a36", Agent: "claude", SourceSessionID: "8a36"}, "8a36"},
		{listedSession{ID: "opaque", Agent: "claude"}, "opaque"},
	}
	for _, c := range cases {
		if got := c.item.rawID(); got != c.want {
			t.Fatalf("rawID(%+v) = %q, want %q", c.item, got, c.want)
		}
	}
}

func TestSessionsPagesThroughLongConversations(t *testing.T) {
	stub := agentsviewtest.Install(t)
	stub.SetList(t, codexEntry(map[string]any{"message_count": 401}))
	page := make([]map[string]any, 0, messageFetchPageLimit)
	roles := []string{"user", "assistant"}
	for i := range messageFetchPageLimit {
		page = append(page, agentsviewtest.Message(roles[i%2], fmt.Sprintf("msg %d", i), "2026-08-31T09:42:43Z"))
	}
	stub.SetMessagesPage(t, "codex:0000-1111", 0, messageFetchPageLimit-1, page...)
	stub.SetMessagesPage(t, "codex:0000-1111", messageFetchPageLimit, messageFetchPageLimit,
		agentsviewtest.Message("user", "tail", "2026-09-01T00:00:00Z"))
	stub.SetMessagesPage(t, "codex:0000-1111", messageFetchPageLimit+1, messageFetchPageLimit)
	got, err := Client{Bin: stub.Bin}.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Messages) != messageFetchPageLimit+1 {
		t.Fatalf("messages = %d", len(got[0].Messages))
	}
	if got[0].Messages[messageFetchPageLimit].Content != "tail" {
		t.Fatalf("last message = %+v", got[0].Messages[messageFetchPageLimit])
	}
}

func TestSessionsDoesNotCacheEmptyFetch(t *testing.T) {
	stub := agentsviewtest.Install(t)
	cache := filepath.Join(t.TempDir(), "agentsview-messages.json")
	stub.SetList(t, codexEntry())
	stub.SetMessages(t, "codex:0000-1111")
	got, err := Client{Bin: stub.Bin, CachePath: cache}.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Messages[0].Content != "PRつくって" || len(got[0].Errors) != 0 {
		t.Fatalf("fallback session = %+v", got)
	}
	if entries := loadCache(cache); len(entries.Entries) != 0 {
		t.Fatalf("empty fetch was cached: %+v", entries.Entries)
	}
}

func TestSessionsIgnoreCacheRefetches(t *testing.T) {
	stub := agentsviewtest.Install(t)
	cache := filepath.Join(t.TempDir(), "agentsview-messages.json")
	stub.SetList(t, codexEntry())
	stub.SetMessages(t, "codex:0000-1111", codexMessage())
	if _, err := (Client{Bin: stub.Bin, CachePath: cache}).Sessions(); err != nil {
		t.Fatal(err)
	}
	firstRun := stub.MessageFetches(t)
	if _, err := (Client{Bin: stub.Bin, CachePath: cache, IgnoreCache: true}).Sessions(); err != nil {
		t.Fatal(err)
	}
	if fetches := stub.MessageFetches(t); fetches != firstRun*2 {
		t.Fatalf("IgnoreCache fetches = %d, want %d", fetches, firstRun*2)
	}
}

func TestSessionsServesCacheWhenAgentsviewUnavailable(t *testing.T) {
	stub := agentsviewtest.Install(t)
	cache := filepath.Join(t.TempDir(), "agentsview-messages.json")
	stub.SetList(t, codexEntry())
	stub.SetMessages(t, "codex:0000-1111", codexMessage())
	if _, err := (Client{Bin: stub.Bin, CachePath: cache}).Sessions(); err != nil {
		t.Fatal(err)
	}
	stub.Break(t)
	rec := &recordingReporter{}
	got, err := Client{Bin: stub.Bin, CachePath: cache, Reporter: rec}.Sessions()
	if err != nil {
		t.Fatalf("degraded mode err = %v", err)
	}
	if len(got) != 1 || got[0].ID != "0000-1111" || got[0].CWD != "/tmp/proj" {
		t.Fatalf("degraded sessions = %+v", got)
	}
	warnings := 0
	for _, event := range rec.snapshot() {
		if strings.HasPrefix(event, "warn: ") && strings.Contains(event, "agentsview unavailable") {
			warnings++
		}
	}
	if warnings != 1 {
		t.Fatalf("warn events = %d, events = %v", warnings, rec.snapshot())
	}
}

func TestSessionsDoesNotCacheIncompletePaging(t *testing.T) {
	stub := agentsviewtest.Install(t)
	cache := filepath.Join(t.TempDir(), "agentsview-messages.json")
	stub.SetList(t, codexEntry(map[string]any{"message_count": 800}))
	page := make([]map[string]any, 0, messageFetchPageLimit)
	roles := []string{"user", "assistant"}
	for i := range messageFetchPageLimit {
		page = append(page, agentsviewtest.Message(roles[i%2], fmt.Sprintf("msg %d", i), "2026-08-31T09:42:43Z"))
	}
	// last_ordinal never advances: paging cannot trust the body as complete.
	stub.SetMessagesPage(t, "codex:0000-1111", 0, 0, page...)
	stub.SetMessagesPage(t, "codex:0000-1111", 1, 0, page...)
	got, err := Client{Bin: stub.Bin, CachePath: cache}.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Messages) == 0 {
		t.Fatalf("sessions = %+v", got)
	}
	if len(got[0].Errors) != 1 || !strings.Contains(got[0].Errors[0], "truncated") {
		t.Fatalf("truncation not surfaced: %+v", got[0].Errors)
	}
	if entries := loadCache(cache); len(entries.Entries) != 0 {
		t.Fatalf("incomplete fetch was cached: %d entries", len(entries.Entries))
	}
}

func TestSessionsRefreshesMetadataOnCacheHit(t *testing.T) {
	stub := agentsviewtest.Install(t)
	cache := filepath.Join(t.TempDir(), "agentsview-messages.json")
	stub.SetList(t, codexEntry())
	stub.SetMessages(t, "codex:0000-1111", codexMessage())
	client := Client{Bin: stub.Bin, CachePath: cache}
	if _, err := client.Sessions(); err != nil {
		t.Fatal(err)
	}
	firstRun := stub.MessageFetches(t)
	stub.SetList(t, codexEntry(map[string]any{"display_name": "renamed-thread"}))
	got, err := client.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "renamed-thread" || got[0].Messages[0].Content != "PRつくって" {
		t.Fatalf("renamed session = %+v", got)
	}
	if fetches := stub.MessageFetches(t); fetches != firstRun {
		t.Fatalf("rename triggered refetch: %d -> %d", firstRun, fetches)
	}
}

func TestSessionsKeepsListedSessionWhenFetchFailsWithEmptyExcerpt(t *testing.T) {
	stub := agentsviewtest.Install(t)
	stub.SetList(t, codexEntry(map[string]any{"first_message": ""}))
	got, err := Client{Bin: stub.Bin}.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Errors) != 1 || len(got[0].Messages) != 0 {
		t.Fatalf("sessions = %+v", got)
	}
}

func TestSessionsReportsProgressEvents(t *testing.T) {
	stub := agentsviewtest.Install(t)
	const total = 10
	entries := make([]map[string]any, 0, total)
	for i := range total {
		id := fmt.Sprintf("codex:sess-%d", i)
		entries = append(entries, agentsviewtest.Session(id, "codex", "", "/tmp/proj", "hello"))
		stub.SetMessages(t, id, agentsviewtest.Message("user", "hello", "2026-01-02T00:00:00Z"))
	}
	stub.SetList(t, entries...)
	rec := &recordingReporter{}
	if _, err := (Client{Bin: stub.Bin, Reporter: rec}).Sessions(); err != nil {
		t.Fatal(err)
	}
	events := rec.snapshot()
	if len(events) < 4 || events[0] != "list-started" || events[1] != "list-finished" || events[2] != fmt.Sprintf("fetch-started %d", total) {
		t.Fatalf("events = %v", events)
	}
	if last := events[len(events)-1]; last != fmt.Sprintf("fetch-finished %d", total) {
		t.Fatalf("last event = %q, events = %v", last, events)
	}
	progressCount := 0
	highestDone := 0
	for _, event := range events[3 : len(events)-1] {
		var done, evTotal int
		if _, err := fmt.Sscanf(event, "fetch-progress %d/%d", &done, &evTotal); err != nil {
			t.Fatalf("unexpected event %q: %v", event, err)
		}
		if evTotal != total {
			t.Fatalf("event total = %d, want %d: %q", evTotal, total, event)
		}
		if done > highestDone {
			highestDone = done
		}
		progressCount++
	}
	if progressCount != total {
		t.Fatalf("fetch-progress count = %d, want %d: %v", progressCount, total, events)
	}
	if highestDone != total {
		t.Fatalf("highest done = %d, want %d", highestDone, total)
	}
}
