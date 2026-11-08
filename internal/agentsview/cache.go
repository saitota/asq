package agentsview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"asq/internal/session"
)

// The cache is disposable: a missing, corrupt, or outdated file only costs
// a refetch. Writes go through a temp file and rename, so concurrent runs
// can at worst lose each other's updates, never corrupt the file.

// Bump when the cachedSession or cachedMessage JSON tags change.
const cacheVersion = 2
const cacheDirMode = 0o700
const cacheFileMode = 0o600

type cacheFile struct {
	Version int                   `json:"version"`
	Entries map[string]cacheEntry `json:"entries"`
}

type cacheEntry struct {
	Signature string        `json:"signature"`
	Session   cachedSession `json:"session"`
}

type cachedSession struct {
	ID        string          `json:"id"`
	Source    string          `json:"source"`
	Name      string          `json:"name"`
	CWD       string          `json:"cwd"`
	GitBranch string          `json:"git_branch,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Messages  []cachedMessage `json:"messages"`
}

type cachedMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func newCachedSession(sess session.Session) cachedSession {
	return cachedSession{
		ID:        sess.ID,
		Source:    string(sess.Source),
		Name:      sess.Name,
		CWD:       sess.CWD,
		GitBranch: sess.GitBranch,
		Timestamp: sess.Timestamp,
		Messages:  newCachedMessages(sess.Messages),
	}
}

func newCachedMessages(messages []session.Message) []cachedMessage {
	if messages == nil {
		return nil
	}
	out := make([]cachedMessage, len(messages))
	for i, msg := range messages {
		out[i] = cachedMessage{Role: string(msg.Role), Content: msg.Content, Timestamp: msg.Timestamp}
	}
	return out
}

func (c cachedSession) toSession() session.Session {
	return session.Session{
		ID:        c.ID,
		Source:    session.Source(c.Source),
		Name:      c.Name,
		CWD:       c.CWD,
		GitBranch: c.GitBranch,
		Timestamp: c.Timestamp,
		Messages:  c.toMessages(),
	}
}

func (c cachedSession) toMessages() []session.Message {
	if c.Messages == nil {
		return nil
	}
	out := make([]session.Message, len(c.Messages))
	for i, msg := range c.Messages {
		out[i] = session.Message{Role: session.Role(msg.Role), Content: msg.Content, Timestamp: msg.Timestamp}
	}
	return out
}

func loadCache(path string) cacheFile {
	empty := cacheFile{Version: cacheVersion, Entries: map[string]cacheEntry{}}
	if path == "" {
		return empty
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return empty
	}
	var out cacheFile
	if err := json.Unmarshal(data, &out); err != nil || out.Version != cacheVersion || out.Entries == nil {
		return empty
	}
	return out
}

// equal compares by signature; bodies never change under an unchanged one.
func (c cacheFile) equal(entries map[string]cacheEntry) bool {
	if len(c.Entries) != len(entries) {
		return false
	}
	for id, entry := range entries {
		if existing, ok := c.Entries[id]; !ok || existing.Signature != entry.Signature {
			return false
		}
	}
	return true
}

func saveCache(path string, entries map[string]cacheEntry) error {
	data, err := json.Marshal(cacheFile{Version: cacheVersion, Entries: entries})
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, cacheDirMode); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".agentsview-messages-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()
	if err := tmp.Chmod(cacheFileMode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func unmarshalJSON(data []byte, out any) error {
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
