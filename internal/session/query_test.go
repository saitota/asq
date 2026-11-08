package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilterCWDExactRejectsUnknownCWD(t *testing.T) {
	current := t.TempDir()
	sessions := []Session{
		{ID: "known", CWD: current},
		{ID: "unknown", CWD: "."},
	}
	got := Filter(sessions, FilterOptions{CWD: current, CWDExact: true})
	if len(got) != 1 || got[0].ID != "known" {
		t.Fatalf("sessions = %+v", got)
	}
}

func TestCWDMatcherMatchesSymlinkedDirectory(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	matcher := NewCWDMatcher(target)
	if !matcher.Matches(Session{CWD: link}) {
		t.Fatalf("expected symlinked cwd to match")
	}
	if matcher.Matches(Session{CWD: "."}) {
		t.Fatalf(`expected "." cwd not to match`)
	}
	if matcher.Matches(Session{CWD: ""}) {
		t.Fatalf("expected empty cwd not to match")
	}
	other := t.TempDir()
	if matcher.Matches(Session{CWD: other}) {
		t.Fatalf("expected a different directory not to match")
	}
}

func TestCWDMatcherMatchesCleanedPathEvenWhenReferenceMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	matcher := NewCWDMatcher(missing)
	messy := missing + string(filepath.Separator) + "."
	if !matcher.Matches(Session{CWD: messy}) {
		t.Fatalf("expected an identically-cleaned path to match even though the reference does not exist")
	}
	existing := t.TempDir()
	if matcher.Matches(Session{CWD: existing}) {
		t.Fatalf("expected a different existing directory not to match a missing reference")
	}
}

func TestFindByIDMatchesUniquePrefix(t *testing.T) {
	sessions := []Session{
		{ID: "019e62ca-113f-74d2-9ad6-1410d123082f"},
		{ID: "019e680d-26a1-7b90-bb70-796893ddf2c4"},
	}
	got, ok := FindByID(sessions, "019e62ca-113")
	if !ok || got.ID != "019e62ca-113f-74d2-9ad6-1410d123082f" {
		t.Fatalf("FindByID prefix = %+v ok=%v", got, ok)
	}
}

func TestFindByIDRejectsAmbiguousPrefix(t *testing.T) {
	sessions := []Session{
		{ID: "abcdef123456-one"},
		{ID: "abcdef123456-two"},
	}
	if got, ok := FindByID(sessions, "abcdef123456"); ok {
		t.Fatalf("FindByID ambiguous prefix = %+v", got)
	}
}

func TestFindByIDPrefersExactMatchOverPrefix(t *testing.T) {
	sessions := []Session{
		{ID: "session"},
		{ID: "session-longer"},
	}
	got, ok := FindByID(sessions, "session")
	if !ok || got.ID != "session" {
		t.Fatalf("FindByID exact = %+v ok=%v", got, ok)
	}
}
