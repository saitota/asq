// Package agentsview is asq's session source: it reads sessions from the
// agentsview CLI (https://github.com/kenn-io/agentsview) and caches them on
// disk keyed by a per-session change signature, so only sessions that
// changed since the last run are refetched.
package agentsview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saitota/asq/internal/session"
)

// EnvBin overrides the agentsview binary name, mainly for tests.
const EnvBin = "ASQ_AGENTSVIEW_BIN"

const listPageLimit = 500
const messageFetchPageLimit = 400
const maxMessageFetchPages = 50
const maxFetchWorkers = 8

// Generous because the first `session list` may autostart the daemon and
// sync the whole store.
const commandTimeout = 120 * time.Second

// Reporter receives progress and degraded-mode events from Sessions; nil
// discards them. ListStarted/ListFinished and FetchStarted/FetchFinished are
// called in pairs from the calling goroutine; only FetchProgress is called
// concurrently, from worker goroutines.
type Reporter interface {
	ListStarted()
	ListFinished()
	FetchStarted(total int)
	FetchProgress(done, total int)
	FetchFinished(done int)
	Warn(message string)
}

type Client struct {
	Bin string // agentsview binary; empty means $ASQ_AGENTSVIEW_BIN, then PATH
	// CachePath is the session cache location; empty disables caching.
	CachePath string
	// IgnoreCache refetches everything; `asq reindex --force`.
	IgnoreCache bool
	Reporter    Reporter
}

type noopReporter struct{}

func (noopReporter) ListStarted()           {}
func (noopReporter) ListFinished()          {}
func (noopReporter) FetchStarted(int)       {}
func (noopReporter) FetchProgress(int, int) {}
func (noopReporter) FetchFinished(int)      {}
func (noopReporter) Warn(string)            {}

func (b Client) reporter() Reporter {
	if b.Reporter == nil {
		return noopReporter{}
	}
	return b.Reporter
}

func DefaultCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "asq", "agentsview-messages.json"), nil
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "asq", "agentsview-messages.json"), nil
	}
	return filepath.Join(home, ".cache", "asq", "agentsview-messages.json"), nil
}

func (b Client) Sessions() ([]session.Session, error) {
	bin := b.Bin
	if bin == "" {
		bin = os.Getenv(EnvBin)
	}
	if bin == "" {
		bin = "agentsview"
	}
	cached := cacheFile{Entries: map[string]cacheEntry{}}
	if !b.IgnoreCache {
		cached = loadCache(b.CachePath)
	}
	reporter := b.reporter()
	reporter.ListStarted()
	listed, err := listAll(bin)
	reporter.ListFinished()
	if err != nil {
		if len(cached.Entries) == 0 {
			return nil, err
		}
		reporter.Warn(fmt.Sprintf("agentsview unavailable, serving %d cached sessions (%v)", len(cached.Entries), err))
		out := make([]session.Session, 0, len(cached.Entries))
		for _, entry := range cached.Entries {
			out = append(out, entry.Session.toSession())
		}
		return out, nil
	}
	entries, sessions := b.build(bin, listed, cached)
	if b.CachePath != "" && !cached.equal(entries) {
		// A failed write only costs a refetch next run.
		_ = saveCache(b.CachePath, entries)
	}
	return sessions, nil
}

// build assembles sessions and the next cache content. Metadata always
// comes from the fresh listing so renames propagate without invalidating
// cached messages.
func (b Client) build(bin string, listed []listedSession, cached cacheFile) (map[string]cacheEntry, []session.Session) {
	entries := make(map[string]cacheEntry, len(listed))
	misses := make([]listedSession, 0, len(listed))
	for _, item := range listed {
		if entry, ok := cached.Entries[item.ID]; !ok || entry.Signature != item.signature() {
			misses = append(misses, item)
		}
	}
	fetched := b.fetchAll(bin, misses)
	var out []session.Session
	for _, item := range listed {
		var messages []session.Message
		var errs []string
		cacheable := false
		if entry, ok := cached.Entries[item.ID]; ok && entry.Signature == item.signature() {
			messages = entry.Session.toMessages()
			cacheable = true
		} else if result := fetched[item.ID]; result.err == nil && len(result.messages) > 0 {
			messages = result.messages
			cacheable = result.complete
			if !result.complete {
				errs = append(errs, "conversation truncated: paging stopped before the end")
			}
		} else {
			// Failed and suspiciously empty fetches degrade to the listing's
			// first-message excerpt and are not cached, so the next run
			// retries instead of freezing the bad result.
			messages = []session.Message{{Role: session.RoleUser, Content: item.FirstMessage, Timestamp: item.time()}}
			if result.err != nil {
				errs = append(errs, result.err.Error())
			}
		}
		sess := session.Normalize(session.Session{
			ID:        item.rawID(),
			Source:    session.Source(item.Agent),
			Name:      item.DisplayName,
			CWD:       item.CWD,
			GitBranch: item.GitBranch,
			Timestamp: item.time(),
			Messages:  messages,
			Errors:    errs,
		})
		// A listed session whose fetch failed stays visible with its warning.
		if len(sess.Messages) == 0 && len(sess.Errors) == 0 {
			continue
		}
		if cacheable {
			entries[item.ID] = cacheEntry{Signature: item.signature(), Session: newCachedSession(sess)}
		}
		out = append(out, sess)
	}
	return entries, out
}

type fetchResult struct {
	messages []session.Message
	complete bool
	err      error
}

func (b Client) fetchAll(bin string, misses []listedSession) map[string]fetchResult {
	results := make([]fetchResult, len(misses))
	var done atomic.Int64
	reporter := b.reporter()
	total := len(misses)
	reporter.FetchStarted(total)
	workers := runtime.GOMAXPROCS(0)
	if workers > maxFetchWorkers {
		workers = maxFetchWorkers
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				messages, complete, err := fetchMessages(bin, misses[i].ID)
				results[i] = fetchResult{messages: messages, complete: complete, err: err}
				reporter.FetchProgress(int(done.Add(1)), total)
			}
		}()
	}
	for i := range misses {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	reporter.FetchFinished(int(done.Load()))
	out := make(map[string]fetchResult, len(misses))
	for i, item := range misses {
		out[item.ID] = results[i]
	}
	return out
}

type listResponse struct {
	Sessions   []listedSession `json:"sessions"`
	NextCursor string          `json:"next_cursor"`
}

type listedSession struct {
	ID                 string `json:"id"`
	Agent              string `json:"agent"`
	DisplayName        string `json:"display_name"`
	FirstMessage       string `json:"first_message"`
	StartedAt          string `json:"started_at"`
	EndedAt            string `json:"ended_at"`
	MessageCount       int    `json:"message_count"`
	CWD                string `json:"cwd"`
	GitBranch          string `json:"git_branch"`
	SourceSessionID    string `json:"source_session_id"`
	TranscriptRevision string `json:"transcript_revision"`
}

// rawID recovers the id the agent's own CLI resumes with. source_session_id
// carries it when agentsview provides one; stripping the "<agent>:" prefix
// agentsview namespaces other ids with is the observed-behavior fallback.
func (item listedSession) rawID() string {
	if item.SourceSessionID != "" {
		return item.SourceSessionID
	}
	if raw, ok := strings.CutPrefix(item.ID, item.Agent+":"); ok {
		return raw
	}
	return item.ID
}

func (item listedSession) time() time.Time {
	value := item.EndedAt
	if value == "" {
		value = item.StartedAt
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

// signature captures everything that invalidates cached messages.
func (item listedSession) signature() string {
	return item.EndedAt + "|" + strconv.Itoa(item.MessageCount) + "|" + item.TranscriptRevision
}

func listAll(bin string) ([]listedSession, error) {
	var out []listedSession
	cursor := ""
	for {
		args := []string{
			"session", "list", "--json",
			"--include-one-shot", "--include-automated",
			"--limit", strconv.Itoa(listPageLimit),
		}
		if cursor != "" {
			args = append(args, "--cursor", cursor)
		}
		var page listResponse
		if err := runJSON(bin, &page, args...); err != nil {
			return nil, err
		}
		out = append(out, page.Sessions...)
		if page.NextCursor == "" || page.NextCursor == cursor || len(page.Sessions) == 0 {
			return out, nil
		}
		cursor = page.NextCursor
	}
}

type messagesResponse struct {
	Messages    []listedMessage `json:"messages"`
	LastOrdinal int             `json:"last_ordinal"`
}

type listedMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// fetchMessages pages through the whole conversation by ordinal. Ordinals
// are sparse under the --role filter, so a short page proves nothing; an
// observed empty page is the only trusted end-of-conversation signal.
// complete is false when paging stopped early (non-advancing last_ordinal
// or page cap), so callers can avoid caching a truncated body.
func fetchMessages(bin, id string) ([]session.Message, bool, error) {
	var messages []session.Message
	from := 0
	for range maxMessageFetchPages {
		var resp messagesResponse
		err := runJSON(bin, &resp,
			"session", "messages", id, "--json",
			"--role", "user,assistant",
			"--from", strconv.Itoa(from),
			"--limit", strconv.Itoa(messageFetchPageLimit),
		)
		if err != nil {
			return nil, false, err
		}
		if len(resp.Messages) == 0 {
			return messages, true, nil
		}
		for _, msg := range resp.Messages {
			role := session.Role(msg.Role)
			if role != session.RoleUser && role != session.RoleAssistant {
				continue
			}
			timestamp, _ := time.Parse(time.RFC3339Nano, msg.Timestamp)
			messages = append(messages, session.Message{Role: role, Content: msg.Content, Timestamp: timestamp.UTC()})
		}
		if resp.LastOrdinal < from {
			return messages, false, nil
		}
		from = resp.LastOrdinal + 1
	}
	return messages, false, nil
}

func runJSON(bin string, out any, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found; asq needs the agentsview CLI (https://github.com/kenn-io/agentsview)", bin)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%s %s: timed out after %s", bin, strings.Join(args, " "), commandTimeout)
		}
		detail := firstLine(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("%s %s: %s", bin, strings.Join(args, " "), detail)
	}
	if err := unmarshalJSON(stdout.Bytes(), out); err != nil {
		return fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), err)
	}
	return nil
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return value[:index]
	}
	return value
}
