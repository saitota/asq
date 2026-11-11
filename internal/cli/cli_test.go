package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/saitota/asq/internal/agentsviewtest"
	"github.com/saitota/asq/internal/ansitext"
	"github.com/saitota/asq/internal/session"
	"github.com/saitota/asq/internal/source"
	"github.com/saitota/asq/internal/tui"
)

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

// runTests points the agents file at a path nothing writes to, so no test can
// pick up the developer's own agents.json.
func runTests(m *testing.M) int {
	dir, err := os.MkdirTemp("", "asq-cli-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.Setenv(source.EnvAgentsFile, filepath.Join(dir, "agents.json")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return m.Run()
}

func stubSession(id, agent, name, cwd, firstMessage string) map[string]any {
	return agentsviewtest.Session(id, agent, name, cwd, firstMessage)
}

func stubMessages(texts ...string) []map[string]any {
	messages := make([]map[string]any, len(texts))
	for i, text := range texts {
		messages[i] = agentsviewtest.Message("user", text, "2026-01-02T00:00:00Z")
	}
	return messages
}

func installAgentsviewStub(t *testing.T, listEntries []map[string]any, messagesByID map[string][]map[string]any) {
	t.Helper()
	stub := agentsviewtest.Install(t)
	stub.SetList(t, listEntries...)
	for id, messages := range messagesByID {
		stub.SetMessages(t, id, messages...)
	}
}

func tempCache(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "agentsview-messages.json")
}

func makeDir(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working dir: %v", err)
		}
	})
}

func writeStubScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestCLIReindexListResumeDryRun(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("sid", "claude", "", projectDir, "hello cli")},
		map[string][]map[string]any{"sid": stubMessages("hello cli")})
	cache := tempCache(t)
	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}
	if err := app.Run([]string{"reindex", "--cache", cache}); err != nil {
		t.Fatalf("reindex err = %v stderr=%s", err, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "sessions=1" {
		t.Fatalf("reindex output = %q", got)
	}
	stdout.Reset()
	if err := app.Run([]string{"list", "--cache", cache, "--fzf", "--all"}); err != nil {
		t.Fatalf("list err = %v", err)
	}
	if !strings.Contains(stdout.String(), "sid") || !strings.Contains(stdout.String(), "hello cli") {
		t.Fatalf("list output = %s", stdout.String())
	}
	stdout.Reset()
	if err := app.Run([]string{"resume", "--cache", cache, "--dry-run", "sid"}); err != nil {
		t.Fatalf("resume err = %v", err)
	}
	if !strings.Contains(stdout.String(), `argv: ["claude","--resume","sid"]`) {
		t.Fatalf("resume output = %s", stdout.String())
	}
}

func TestResumeStripsAgentPrefixFromID(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("codex:agview-1", "codex", "agview session", projectDir, "hello agentsview")},
		map[string][]map[string]any{"codex:agview-1": stubMessages("hello agentsview")})
	cache := tempCache(t)
	var stdout bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	if err := app.Run([]string{"resume", "--cache", cache, "--dry-run", "agview-1"}); err != nil {
		t.Fatalf("resume err = %v", err)
	}
	if !strings.Contains(stdout.String(), `argv: ["codex","resume","agview-1"]`) {
		t.Fatalf("resume output = %s", stdout.String())
	}
}

func TestResumeDryRunValidatesCWD(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("sid", "claude", "", missing, "hello cli")},
		map[string][]map[string]any{"sid": stubMessages("hello cli")})
	app := App{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	err := app.Run([]string{"resume", "--cache", tempCache(t), "--dry-run", "sid"})
	if err == nil || !strings.Contains(err.Error(), "cannot access session cwd") {
		t.Fatalf("err = %v", err)
	}
}

func TestResumeAcceptsUniqueIDPrefix(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("abcdef1234567890", "claude", "", projectDir, "hello cli")},
		map[string][]map[string]any{"abcdef1234567890": stubMessages("hello cli")})
	var stdout bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	if err := app.Run([]string{"resume", "--cache", tempCache(t), "--dry-run", "abcdef123456"}); err != nil {
		t.Fatalf("resume err = %v", err)
	}
	if !strings.Contains(stdout.String(), `argv: ["claude","--resume","abcdef1234567890"]`) {
		t.Fatalf("resume output = %s", stdout.String())
	}
}

func TestResumeRejectsInvalidSessionID(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("codex:bad id", "codex", "", projectDir, "hello cli")},
		map[string][]map[string]any{"codex:bad id": stubMessages("hello cli")})
	app := App{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	err := app.Run([]string{"resume", "--cache", tempCache(t), "--dry-run", "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid cached session id") {
		t.Fatalf("err = %v", err)
	}
}

func TestVersionCommand(t *testing.T) {
	var stdout bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	if err := app.Run([]string{"version"}); err != nil {
		t.Fatalf("version err = %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "dev" {
		t.Fatalf("version output = %q", got)
	}
}

func TestListDefaultsToCurrentDirectory(t *testing.T) {
	base := t.TempDir()
	currentDir := makeDir(t, base, "current")
	otherDir := makeDir(t, base, "other")
	withWorkingDir(t, currentDir)
	installAgentsviewStub(t,
		[]map[string]any{
			stubSession("current-sid", "claude", "", currentDir, "hello current"),
			stubSession("other-sid", "claude", "", otherDir, "hello other"),
		},
		map[string][]map[string]any{
			"current-sid": stubMessages("hello current"),
			"other-sid":   stubMessages("hello other"),
		})
	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}
	if err := app.Run([]string{"list", "--cache", tempCache(t), "--fzf"}); err != nil {
		t.Fatalf("list err = %v stderr=%s", err, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "hello current") {
		t.Fatalf("list output missing current session: %s", output)
	}
	if strings.Contains(output, "hello other") {
		t.Fatalf("list output included other directory: %s", output)
	}
}

func TestListAllShowsEveryDirectory(t *testing.T) {
	base := t.TempDir()
	currentDir := makeDir(t, base, "current")
	otherDir := makeDir(t, base, "other")
	withWorkingDir(t, currentDir)
	installAgentsviewStub(t,
		[]map[string]any{
			stubSession("current-sid", "claude", "", currentDir, "hello current"),
			stubSession("other-sid", "claude", "", otherDir, "hello other"),
		},
		map[string][]map[string]any{
			"current-sid": stubMessages("hello current"),
			"other-sid":   stubMessages("hello other"),
		})
	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}
	if err := app.Run([]string{"list", "--cache", tempCache(t), "--fzf", "--all"}); err != nil {
		t.Fatalf("list err = %v stderr=%s", err, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "hello current") || !strings.Contains(output, "hello other") {
		t.Fatalf("list output = %s", output)
	}
}

// list --json is documented machine-readable output, so a key change must be deliberate.
func TestListJSONOutputShape(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{
			stubSession("sid", "claude", "session name", projectDir, "hello json"),
			stubSession("sid-failed", "claude", "", projectDir, "hello failed"),
		},
		map[string][]map[string]any{"sid": stubMessages("hello json")})
	cache := tempCache(t)
	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}
	if err := app.Run([]string{"list", "--cache", cache, "--json", "--all"}); err != nil {
		t.Fatalf("list err = %v stderr=%s", err, stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if keys := slices.Sorted(maps.Keys(decoded)); !reflect.DeepEqual(keys, []string{"sessions"}) {
		t.Fatalf("top-level keys = %v", keys)
	}
	sessions, ok := decoded["sessions"].([]any)
	if !ok || len(sessions) != 2 {
		t.Fatalf("sessions = %+v", decoded["sessions"])
	}
	byID := map[string]map[string]any{}
	for _, item := range sessions {
		sess, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("session = %+v", item)
		}
		id, _ := sess["id"].(string)
		byID[id] = sess
	}
	sess, ok := byID["sid"]
	if !ok {
		t.Fatalf("sessions = %+v", decoded["sessions"])
	}
	wantSessionKeys := []string{"cwd", "git_branch", "id", "messages", "name", "source", "timestamp"}
	if keys := slices.Sorted(maps.Keys(sess)); !reflect.DeepEqual(keys, wantSessionKeys) {
		t.Fatalf("session keys = %v, want %v", keys, wantSessionKeys)
	}
	messages, ok := sess["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %+v", sess["messages"])
	}
	msg, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("message = %+v", messages[0])
	}
	wantMessageKeys := []string{"content", "role", "timestamp"}
	if keys := slices.Sorted(maps.Keys(msg)); !reflect.DeepEqual(keys, wantMessageKeys) {
		t.Fatalf("message keys = %v, want %v", keys, wantMessageKeys)
	}
	failed, ok := byID["sid-failed"]
	if !ok {
		t.Fatalf("sessions = %+v", decoded["sessions"])
	}
	wantFailedKeys := []string{"cwd", "errors", "git_branch", "id", "messages", "name", "source", "timestamp"}
	if keys := slices.Sorted(maps.Keys(failed)); !reflect.DeepEqual(keys, wantFailedKeys) {
		t.Fatalf("failed session keys = %v, want %v", keys, wantFailedKeys)
	}
	if errs, ok := failed["errors"].([]any); !ok || len(errs) == 0 {
		t.Fatalf("failed session errors = %+v", failed["errors"])
	}
}

func TestListAllRejectsCWD(t *testing.T) {
	app := App{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	err := app.Run([]string{"list", "--all", "--cwd", "proj"})
	if err == nil || !strings.Contains(err.Error(), "cannot use --all with --cwd") {
		t.Fatalf("err = %v", err)
	}
}

func TestSessionTableStaysWithinConfiguredWidth(t *testing.T) {
	t.Setenv("COLUMNS", "90")
	var stdout bytes.Buffer
	sessions := []session.Session{{
		ID:        "019e61bd-fc3c-very-long-session-id",
		Source:    source.Codex,
		Name:      "Width check",
		CWD:       "/Users/example/work/github.com/example-org/example-repository-with-a-long-name",
		GitBranch: "NOTICKET_asq_with_long_branch_name",
		Timestamp: time.Date(2026, 5, 26, 5, 11, 0, 0, time.UTC),
		Messages: []session.Message{{
			Role:    session.RoleUser,
			Content: "これは横幅を壊しやすい日本語を含むとても長いメッセージです",
		}},
	}}
	if err := writeSessionTable(&stdout, source.Builtin(), sessions); err != nil {
		t.Fatalf("writeSessionTable err = %v", err)
	}
	for _, line := range strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n") {
		if width := lipgloss.Width(line); width > 90 {
			t.Fatalf("line width = %d want <= 90: %s", width, line)
		}
	}
}

func TestSessionTableLayoutExpandsWithColumns(t *testing.T) {
	var stdout bytes.Buffer
	t.Setenv("COLUMNS", "160")
	wide := sessionTableLayout(&stdout)
	t.Setenv("COLUMNS", "90")
	narrow := sessionTableLayout(&stdout)
	if wide.TableWidth != 160 || narrow.TableWidth != 90 {
		t.Fatalf("widths = wide:%d narrow:%d", wide.TableWidth, narrow.TableWidth)
	}
	if wide.PathWidth <= narrow.PathWidth || wide.SnippetWidth <= narrow.SnippetWidth {
		t.Fatalf("layout did not expand: wide=%+v narrow=%+v", wide, narrow)
	}
}

func TestFZFLineColorsOnlySource(t *testing.T) {
	t.Setenv("NO_COLOR", "")
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
		}, {
			Role:    session.RoleAssistant,
			Content: "deep hidden match",
		}},
	}
	line := newSelectionLine(source.Builtin(), sess, true).encode()
	fields := strings.Split(line, "\t")
	if len(fields) != fzfFieldCount {
		t.Fatalf("fields = %d line=%q", len(fields), line)
	}
	if fields[fzfSourceField] == string(source.Codex) || !strings.Contains(fields[fzfSourceField], "\x1b[") {
		t.Fatalf("source field is not colored: %q", fields[fzfSourceField])
	}
	if fields[fzfNameField] != "Session name" {
		t.Fatalf("name field = %q", fields[fzfNameField])
	}
	if !strings.Contains(fields[fzfSearchTextField], "deep hidden match") {
		t.Fatalf("search field = %q", fields[fzfSearchTextField])
	}
	for _, field := range []int{fzfIDField, fzfSearchTextField} {
		if !strings.HasPrefix(fields[field], ansiConcealStart) || !strings.HasSuffix(fields[field], ansiConcealEnd) {
			t.Fatalf("field %d is not concealed: %q", field, fields[field])
		}
	}
	for i, field := range fields[1:] {
		switch i + 1 {
		case fzfIDField, fzfSearchTextField:
			continue
		}
		if strings.Contains(field, "\x1b[") {
			t.Fatalf("field %d unexpectedly colored: %q", i+1, field)
		}
	}
}

func TestFZFLineStripsANSIFromHiddenSearchText(t *testing.T) {
	sess := session.Session{
		ID:     "sid",
		Source: source.Codex,
		Messages: []session.Message{{
			Role:    session.RoleUser,
			Content: "before \x1b[0m exposed",
		}},
	}
	fields := strings.Split(newSelectionLine(source.Builtin(), sess, true).encode(), "\t")
	searchText := strings.TrimPrefix(fields[fzfSearchTextField], ansiConcealStart)
	searchText = strings.TrimSuffix(searchText, ansiConcealEnd)
	if ansitext.Strip(searchText) != searchText {
		t.Fatalf("search field contains ANSI: %q", fields[fzfSearchTextField])
	}
	if !strings.Contains(searchText, "before  exposed") {
		t.Fatalf("search field = %q", searchText)
	}
}

func TestPreviewTextShowsConversationContext(t *testing.T) {
	sess := session.Session{
		ID:        "sid",
		Source:    source.Claude,
		Name:      "Session name",
		CWD:       "/tmp/proj",
		GitBranch: "main",
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Messages: []session.Message{{
			Role:      session.RoleUser,
			Content:   "hello preview",
			Timestamp: time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		}, {
			Role:      session.RoleAssistant,
			Content:   "context response",
			Timestamp: time.Date(2026, 1, 1, 0, 2, 0, 0, time.UTC),
		}},
	}
	got := previewText(sess)
	for _, want := range []string{"claude sid", "name: Session name", "cwd: /tmp/proj", "[1] user", "hello preview", "[2] assistant", "context response"} {
		if !strings.Contains(got, want) {
			t.Fatalf("preview = %q, want %q", got, want)
		}
	}
}

func TestPreviewTextShowsWarnings(t *testing.T) {
	sess := session.Session{
		ID:       "sid",
		Source:   source.Claude,
		CWD:      "/tmp/proj",
		Errors:   []string{"messages unavailable"},
		Messages: []session.Message{{Role: session.RoleUser, Content: "hello"}},
	}
	if got := previewText(sess); !strings.Contains(got, "warning: messages unavailable") {
		t.Fatalf("preview = %q", got)
	}
}

// previewBar and previewMetaLine are producers whose output tui.wrapDisplay
// (via ansitext.Wrap) consumes: wrapDisplay reopens every SGR sequence still
// active at a wrap point, so as long as these emit at most one open style
// per value, a wrap can never leave more than one sequence to reopen. This
// pins that single-style contract so a future change that layers styles
// (e.g. bold + color) here fails loudly instead of surfacing as a wrap-time
// color bleed in tui.
func TestPreviewStyleHelpersEmitAtMostOneOpenStyle(t *testing.T) {
	cases := []string{
		previewBar(previewMetaBg, "role 12:00"),
		previewMetaLine("cwd: /tmp/proj"),
	}
	for _, value := range cases {
		opens := strings.Count(value, "\x1b[") - strings.Count(value, "\x1b[0m")
		if opens > 1 {
			t.Fatalf("value emits %d open SGR sequences, want at most 1: %q", opens, value)
		}
		if !strings.HasSuffix(value, "\x1b[0m") {
			t.Fatalf("value does not close its style with a reset: %q", value)
		}
	}
}

func TestSelectionKeyStripsANSI(t *testing.T) {
	selection := "\x1b[38;2;96;165;250mcodex\x1b[0m\t2026-01-01T00:00:00Z\tSession name\t/tmp/proj\tmain\thello\tuser hello\tsid"
	if got := selectionKeyFromSelection(selection); got != "sid" {
		t.Fatalf("selectionKeyFromSelection = %q", got)
	}
}

func TestSelectionKeyStripsConcealedID(t *testing.T) {
	if got := selectionKeyFromSelection(conceal("sid")); got != "sid" {
		t.Fatalf("selectionKeyFromSelection = %q", got)
	}
}

func TestSelectionKeyFromArgsHandlesXargsSplitTSV(t *testing.T) {
	args := strings.Fields("claude\t2026-01-01T00:00:00Z\tsession-name\t/tmp/my project\tmain\thello world\tuser hello world\tsid")
	if got := selectionKeyFromArgs(args); got != "sid" {
		t.Fatalf("selectionKeyFromArgs = %q", got)
	}
}

func TestCWDCommandPrintsSessionCWD(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("cwd-sid", "claude", "", projectDir, "hello cwd")},
		map[string][]map[string]any{"cwd-sid": stubMessages("hello cwd")})
	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}
	if err := app.Run([]string{"cwd", "--cache", tempCache(t), "cwd-sid"}); err != nil {
		t.Fatalf("cwd err = %v stderr=%s", err, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != projectDir {
		t.Fatalf("cwd output = %q", got)
	}
}

func TestPickResumesFromSessionCWD(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("sid-pick", "claude", "", projectDir, "hello pick")},
		map[string][]map[string]any{"sid-pick": stubMessages("hello pick")})
	cache := tempCache(t)

	outFile := filepath.Join(t.TempDir(), "resume.out")
	binDir := t.TempDir()
	writeStubScript(t, filepath.Join(binDir, "claude"), fmt.Sprintf("#!/bin/sh\n{ pwd; printf '%%s\\n' \"$@\"; } > %q\n", outFile))
	t.Setenv("PATH", binDir)

	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}
	app.Pick = func(items []tui.Item, preview func(int) string, query string) (int, error) {
		if len(items) != 1 {
			t.Fatalf("picker got %d items", len(items))
		}
		return 0, nil
	}

	if err := app.Run([]string{"pick", "--cache", cache}); err != nil {
		t.Fatalf("pick err = %v stderr=%s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("pick stdout = %q, want empty", stdout.String())
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("resume stub did not run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("resume stub recorded %q", string(data))
	}
	wantDir, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if lines[0] != wantDir && lines[0] != projectDir {
		t.Fatalf("resume ran in %q, want %q", lines[0], projectDir)
	}
	if lines[1] != "--resume" || lines[2] != "sid-pick" {
		t.Fatalf("resume args = %q", lines[1:])
	}
}

func TestDefaultCommandPicksAndResumes(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("sid-default", "claude", "", projectDir, "hello default")},
		map[string][]map[string]any{"sid-default": stubMessages("hello default")})
	cache := tempCache(t)

	outFile := filepath.Join(t.TempDir(), "resume.out")
	binDir := t.TempDir()
	writeStubScript(t, filepath.Join(binDir, "claude"), fmt.Sprintf("#!/bin/sh\n{ pwd; printf '%%s\\n' \"$@\"; } > %q\n", outFile))
	t.Setenv("PATH", binDir)

	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}
	gotQuery := ""
	app.Pick = func(items []tui.Item, preview func(int) string, query string) (int, error) {
		gotQuery = query
		return 0, nil
	}

	if err := app.Run([]string{"--cache", cache, "hello default"}); err != nil {
		t.Fatalf("default run err = %v stderr=%s", err, stderr.String())
	}
	if gotQuery != "hello default" {
		t.Fatalf("picker query = %q, want %q", gotQuery, "hello default")
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("resume stub did not run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 || lines[1] != "--resume" || lines[2] != "sid-default" {
		t.Fatalf("resume stub recorded %q", string(data))
	}
}

func TestResumeReportsMissingAgentCLI(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("sid-nocli", "claude", "", projectDir, "hello cli")},
		map[string][]map[string]any{"sid-nocli": stubMessages("hello cli")})
	app := App{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	t.Setenv("PATH", t.TempDir())
	err := app.Run([]string{"resume", "--cache", tempCache(t), "sid-nocli"})
	want := "claude not found; install it to resume claude sessions"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestPickerLineUsesColorMarkerAndBudgets(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	sess := session.Session{
		Source:    source.Claude,
		Name:      "a very long session name that keeps going and going",
		CWD:       "/Users/you/work/github.com/example/some-repository-name",
		GitBranch: "main",
		Messages: []session.Message{{
			Role:    session.RoleUser,
			Content: "hello picker",
		}},
	}
	line := pickerLine(source.Builtin(), sess)
	if strings.Contains(line, "claude") {
		t.Fatalf("picker line should encode the source as color, got %q", line)
	}
	if !strings.Contains(line, "●") || !strings.Contains(line, source.Builtin().ANSIColor(source.Claude)) {
		t.Fatalf("picker line missing colored marker: %q", line)
	}
	if !strings.Contains(line, "...") {
		t.Fatalf("picker line should truncate long fields: %q", line)
	}
	if !strings.Contains(line, "pository-name") {
		t.Fatalf("path tail should survive middle truncation: %q", line)
	}
	if !strings.Contains(line, "hello picker") {
		t.Fatalf("picker line missing snippet: %q", line)
	}
}

func TestPickerLineFallsBackToSourceTextWithoutColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	sess := session.Session{Source: source.Codex, CWD: "/tmp"}
	line := pickerLine(source.Builtin(), sess)
	if !strings.Contains(line, "codex") || strings.Contains(line, "●") {
		t.Fatalf("expected text source without colors, got %q", line)
	}
}

func TestPickerPreviewTextDrawsMetaBars(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	sess := session.Session{
		ID:     "sid",
		Source: source.Claude,
		Name:   "Named",
		CWD:    "/tmp",
		Messages: []session.Message{
			{Role: session.RoleUser, Content: "question text"},
			{Role: session.RoleAssistant, Content: "answer text"},
		},
	}
	got := pickerPreviewText(source.Builtin(), sess)
	claudeRGB := strings.TrimPrefix(source.Builtin().ANSIColor(source.Claude), "38;2;")
	for _, want := range []string{
		"\x1b[38;2;" + previewMetaBg + "mclaude sid\x1b[0m",
		"\x1b[38;2;" + previewMetaBg + "mname: Named\x1b[0m",
		"\x1b[48;2;" + previewMetaBg + ";38;2;" + previewMetaFg + "m[1] user",
		"\x1b[48;2;" + claudeRGB + ";38;2;" + previewMetaFg + "m[2] assistant",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("preview missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "\nquestion text\n") || !strings.Contains(got, "\nanswer text\n") {
		t.Fatalf("message bodies must stay plain:\n%s", got)
	}
}

func TestPickerPreviewTextPlainWithoutColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	sess := session.Session{
		ID:       "sid",
		Source:   source.Claude,
		CWD:      "/tmp",
		Messages: []session.Message{{Role: session.RoleUser, Content: "hello"}},
	}
	if got := pickerPreviewText(source.Builtin(), sess); strings.Contains(got, "\x1b[") {
		t.Fatalf("expected plain preview with NO_COLOR:\n%q", got)
	}
}

func TestCWDAcceptsColonBearingIDOfNonResumableAgent(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("openclaw:agent:xyz", "openclaw", "", projectDir, "hello openclaw")},
		map[string][]map[string]any{"openclaw:agent:xyz": stubMessages("hello openclaw")})
	var stdout bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	if err := app.Run([]string{"cwd", "--cache", tempCache(t), "agent:xyz"}); err != nil {
		t.Fatalf("cwd err = %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != projectDir {
		t.Fatalf("cwd output = %q", got)
	}
}

func TestPickerPreviewTextAndPreviewTextShareLayout(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	totalMessages := previewMessageLimit + 5
	messages := make([]session.Message, 0, totalMessages)
	for i := 0; i < totalMessages; i++ {
		role := session.RoleUser
		if i%2 == 1 {
			role = session.RoleAssistant
		}
		content := fmt.Sprintf("message %d", i)
		if i == 5 {
			content = "   "
		}
		messages = append(messages, session.Message{
			Role:      role,
			Content:   content,
			Timestamp: time.Date(2026, 1, 1, 0, i, 0, 0, time.UTC),
		})
	}
	sess := session.Session{
		ID:        "sid",
		Source:    source.Claude,
		Name:      "Session name",
		CWD:       "/tmp/proj",
		GitBranch: "main",
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Errors:    []string{"warning one", "warning two"},
		Messages:  messages,
	}
	colored := ansitext.Strip(pickerPreviewText(source.Builtin(), sess))
	plain := sanitizePreview(previewText(sess))
	if colored != plain {
		t.Fatalf("colored preview stripped = %q, want %q", colored, plain)
	}
}

func TestResumeReportsUnsupportedAgentBeforeIDValidation(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("openclaw:agent:xyz", "openclaw", "", projectDir, "hello openclaw")},
		map[string][]map[string]any{"openclaw:agent:xyz": stubMessages("hello openclaw")})
	app := App{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	err := app.Run([]string{"resume", "--cache", tempCache(t), "--dry-run", "agent:xyz"})
	if err == nil || !strings.Contains(err.Error(), "resume is not supported for openclaw sessions") {
		t.Fatalf("err = %v", err)
	}
}

func TestResumeReportsInvalidIDBeforeCWD(t *testing.T) {
	installAgentsviewStub(t,
		[]map[string]any{stubSession("claude:agent:xyz", "claude", "", filepath.Join(t.TempDir(), "missing"), "hello")},
		map[string][]map[string]any{"claude:agent:xyz": stubMessages("hello")})
	app := App{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	err := app.Run([]string{"resume", "--cache", tempCache(t), "--dry-run", "agent:xyz"})
	if err == nil || !strings.Contains(err.Error(), "invalid cached session id") {
		t.Fatalf("err = %v", err)
	}
}
