package session

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FilterOptions struct {
	Source   Source
	CWD      string
	CWDExact bool
	Limit    int
}

// Filter returns the sessions matching opts, sorted newest first.
func Filter(sessions []Session, opts FilterOptions) []Session {
	out := make([]Session, 0, len(sessions))
	var matcher CWDMatcher
	if opts.CWD != "" && opts.CWDExact {
		matcher = NewCWDMatcher(opts.CWD)
	}
	for _, sess := range sessions {
		if opts.Source != "" && sess.Source != opts.Source {
			continue
		}
		if opts.CWD != "" {
			if opts.CWDExact {
				if !matcher.Matches(sess) {
					continue
				}
			} else if !strings.Contains(sess.CWD, opts.CWD) {
				continue
			}
		}
		out = append(out, sess)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	if opts.Limit > 0 && len(out) > opts.Limit {
		return out[:opts.Limit]
	}
	return out
}

type CWDMatcher struct {
	path string
	info os.FileInfo
	ok   bool
}

// NewCWDMatcher stats path once so Matches can compare many sessions cheaply.
func NewCWDMatcher(path string) CWDMatcher {
	info, err := os.Stat(path)
	return CWDMatcher{path: path, info: info, ok: err == nil}
}

func (m CWDMatcher) Matches(sess Session) bool {
	if !sess.HasCWD() || m.path == "" || m.path == "." {
		return false
	}
	if filepath.Clean(sess.CWD) == filepath.Clean(m.path) {
		return true
	}
	if !m.ok {
		return false
	}
	info, err := os.Stat(sess.CWD)
	return err == nil && os.SameFile(info, m.info)
}

// FindByID matches the full id first, then a unique prefix; ambiguous
// matches report not found.
func FindByID(sessions []Session, id string) (Session, bool) {
	var found Session
	for _, sess := range sessions {
		if sess.ID == id {
			if found.ID != "" {
				return Session{}, false
			}
			found = sess
		}
	}
	if found.ID != "" {
		return found, true
	}
	for _, sess := range sessions {
		if strings.HasPrefix(sess.ID, id) {
			if found.ID != "" {
				return Session{}, false
			}
			found = sess
		}
	}
	return found, found.ID != ""
}
