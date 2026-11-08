package progress

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// firstWriteSignal closes ch on its first Write, letting a test wait
// deterministically for a reporter goroutine's first output instead of
// racing a sleep against it.
type firstWriteSignal struct {
	strings.Builder
	once sync.Once
	ch   chan struct{}
}

func newFirstWriteSignal() *firstWriteSignal {
	return &firstWriteSignal{ch: make(chan struct{})}
}

func (w *firstWriteSignal) Write(p []byte) (int, error) {
	n, err := w.Builder.Write(p)
	w.once.Do(func() { close(w.ch) })
	return n, err
}

func waitForWrite(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the reporter's first write")
	}
}

func TestFetchStartedAnnouncesLargeFetches(t *testing.T) {
	var out strings.Builder
	NewReporter(&out).FetchStarted(10)
	if !strings.Contains(out.String(), "syncing 10 conversations") {
		t.Fatalf("output = %q", out.String())
	}
	if strings.Contains(out.String(), "\r") {
		t.Fatalf("non-terminal output got control characters: %q", out.String())
	}
}

func TestFetchStartedStaysQuietForSmallFetches(t *testing.T) {
	var out strings.Builder
	NewReporter(&out).FetchStarted(9)
	if out.Len() != 0 {
		t.Fatalf("output = %q", out.String())
	}
}

func TestWarnPrefixesMessage(t *testing.T) {
	var out strings.Builder
	NewReporter(&out).Warn("x")
	if got := out.String(); got != "asq: x\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestListStartedExplainsSlowListing(t *testing.T) {
	out := newFirstWriteSignal()
	r := NewReporter(out)
	previous := listWaitNotice
	listWaitNotice = 20 * time.Millisecond
	t.Cleanup(func() { listWaitNotice = previous })
	r.ListStarted()
	waitForWrite(t, out.ch)
	r.ListFinished()
	if !strings.Contains(out.String(), "waiting for agentsview") {
		t.Fatalf("output = %q", out.String())
	}
	if strings.Contains(out.String(), "\r") {
		t.Fatalf("non-terminal output got control characters: %q", out.String())
	}
}

func TestListStartedStaysQuietWhenFinishedImmediately(t *testing.T) {
	var out strings.Builder
	r := NewReporter(&out)
	r.ListStarted()
	r.ListFinished()
	if out.Len() != 0 {
		t.Fatalf("output = %q", out.String())
	}
}

func TestNilReporterToleratesEveryMethod(t *testing.T) {
	r := NewReporter(nil)
	r.ListStarted()
	r.ListFinished()
	r.FetchStarted(100)
	r.FetchProgress(1, 100)
	r.FetchFinished(100)
	r.Warn("x")
}

func TestFinishWithoutStartDoesNotPanic(t *testing.T) {
	var out strings.Builder
	r := NewReporter(&out)
	r.FetchFinished(0)
	r.ListFinished()
}

func TestFetchProgressHandlesConcurrentCalls(t *testing.T) {
	var out strings.Builder
	r := NewReporter(&out)
	const total = 50
	r.FetchStarted(total)
	var wg sync.WaitGroup
	for i := 1; i <= total; i++ {
		wg.Add(1)
		go func(done int) {
			defer wg.Done()
			r.FetchProgress(done, total)
		}(i)
	}
	wg.Wait()
	r.FetchFinished(total)
}

func TestFetchFinishedPrintsSyncedCountOnTerminal(t *testing.T) {
	var out strings.Builder
	r := newReporter(&out, true)
	r.FetchStarted(10)
	r.FetchFinished(10)
	if !strings.Contains(out.String(), "\r\x1b[Kasq: synced 10 conversations\n") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestListFinishedPrintsRespondedOnTerminal(t *testing.T) {
	out := newFirstWriteSignal()
	r := newReporter(out, true)
	previous := listWaitNotice
	listWaitNotice = 20 * time.Millisecond
	t.Cleanup(func() { listWaitNotice = previous })
	r.ListStarted()
	waitForWrite(t, out.ch)
	r.ListFinished()
	if !strings.Contains(out.String(), "\r\x1b[Kasq: agentsview responded after") {
		t.Fatalf("output = %q", out.String())
	}
}
