// Package progress renders agentsview sync progress and degraded-mode
// warnings on stderr, updating a terminal in place and staying line-oriented
// on pipes.
package progress

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/x/term"
)

const minAnnouncedSessions = 10
const tickInterval = 200 * time.Millisecond

// listWaitNotice is how long `session list` may stay silent before asq says
// it is waiting; a var so tests can shorten it.
var listWaitNotice = time.Second

type Reporter struct {
	w        io.Writer
	terminal bool

	listStop     chan struct{}
	listFinished chan struct{}

	fetchAnnounced bool
	fetchDone      atomic.Int64
	fetchStop      chan struct{}
	fetchFinished  chan struct{}
}

// NewReporter renders to w; a nil w discards every event.
func NewReporter(w io.Writer) *Reporter {
	terminal := false
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		terminal = term.IsTerminal(f.Fd())
	}
	return newReporter(w, terminal)
}

func newReporter(w io.Writer, terminal bool) *Reporter {
	return &Reporter{w: w, terminal: terminal}
}

func (r *Reporter) printf(format string, args ...any) {
	if r.w == nil {
		return
	}
	_, _ = fmt.Fprintf(r.w, format, args...)
}

// ListStarted explains a stalled listing — the first `session list`
// autostarts the agentsview daemon, which may resync its whole store for
// tens of seconds before answering — and, on a terminal, keeps an elapsed
// counter updated in place.
func (r *Reporter) ListStarted() {
	if r.w == nil {
		return
	}
	stop := make(chan struct{})
	finished := make(chan struct{})
	r.listStop = stop
	r.listFinished = finished
	go func() {
		defer close(finished)
		select {
		case <-stop:
			return
		case <-time.After(listWaitNotice):
		}
		started := time.Now()
		r.printf("asq: waiting for agentsview (it may be starting up and syncing its session store)...\n")
		if !r.terminal {
			<-stop
			return
		}
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				r.printf("\r\x1b[Kasq: agentsview responded after %ds\n", int(time.Since(started).Seconds()))
				return
			case <-ticker.C:
				r.printf("\r\x1b[Kasq: waiting for agentsview... %ds", int(time.Since(started).Seconds()))
			}
		}
	}()
}

func (r *Reporter) ListFinished() {
	stop := r.listStop
	if stop == nil {
		return
	}
	finished := r.listFinished
	r.listStop = nil
	r.listFinished = nil
	close(stop)
	<-finished
}

// FetchStarted announces a large fetch — a full sync otherwise looks frozen
// for tens of seconds — and, on a terminal, keeps a counter updated in
// place.
func (r *Reporter) FetchStarted(total int) {
	r.fetchDone.Store(0)
	if r.w == nil || total < minAnnouncedSessions {
		r.fetchAnnounced = false
		return
	}
	r.fetchAnnounced = true
	r.printf("asq: syncing %d conversations from agentsview...\n", total)
	if !r.terminal {
		return
	}
	stop := make(chan struct{})
	finished := make(chan struct{})
	r.fetchStop = stop
	r.fetchFinished = finished
	go func() {
		defer close(finished)
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				r.printf("\r\x1b[Kasq: syncing conversations %d/%d", r.fetchDone.Load(), total)
			}
		}
	}()
}

// FetchProgress records the latest completed count for FetchStarted's
// ticker; it may be called concurrently from several worker goroutines.
func (r *Reporter) FetchProgress(done, total int) {
	for {
		current := r.fetchDone.Load()
		if int64(done) <= current || r.fetchDone.CompareAndSwap(current, int64(done)) {
			return
		}
	}
}

func (r *Reporter) FetchFinished(done int) {
	if !r.fetchAnnounced {
		return
	}
	r.fetchAnnounced = false
	if stop := r.fetchStop; stop != nil {
		finished := r.fetchFinished
		r.fetchStop = nil
		r.fetchFinished = nil
		close(stop)
		<-finished
	}
	if r.terminal {
		r.printf("\r\x1b[Kasq: synced %d conversations\n", done)
	}
}

func (r *Reporter) Warn(message string) {
	r.printf("asq: %s\n", message)
}
