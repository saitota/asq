package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saitota/asq/internal/agentsviewtest"
	"github.com/saitota/asq/internal/source"
	"github.com/saitota/asq/internal/tui"
)

func useAgentsFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agents.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(source.EnvAgentsFile, path)
	return path
}

func runAsq(args ...string) (stdout, stderr string, err error) {
	var out, errOut bytes.Buffer
	err = App{Stdout: &out, Stderr: &errOut, Stdin: strings.NewReader("")}.Run(args)
	return out.String(), errOut.String(), err
}

func conversation(text string) []map[string]any {
	return []map[string]any{
		agentsviewtest.Message("user", text, "2026-01-02T00:00:00Z"),
		agentsviewtest.Message("assistant", "reply to "+text, "2026-01-02T00:00:01Z"),
	}
}

// agentsWarnings picks the agents-file warnings out of stderr, which also
// carries the "waiting for agentsview" notice when the stub is slow.
func agentsWarnings(stderr string) []string {
	var lines []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(line, "asq: ignoring user-defined agents") {
			lines = append(lines, line)
		}
	}
	return lines
}

func requireOneWarning(t *testing.T, stderr, path string) {
	t.Helper()
	if lines := agentsWarnings(stderr); len(lines) != 1 || !strings.Contains(lines[0], path) {
		t.Fatalf("stderr = %q, want one warning naming %s", stderr, path)
	}
}

func requireNoWarning(t *testing.T, stderr string) {
	t.Helper()
	if lines := agentsWarnings(stderr); len(lines) != 0 {
		t.Fatalf("stderr = %q, want no warning", stderr)
	}
}

func TestUserDefinedAgentResumesAndGetsItsColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	projectDir := makeDir(t, t.TempDir(), "proj")
	useAgentsFile(t, `{"agents":[{"source":"acme","command":["acme","session","--id={id}"],"color":"#112233"}]}`)
	installAgentsviewStub(t,
		[]map[string]any{stubSession("acme:sid", "acme", "", projectDir, "hello acme")},
		map[string][]map[string]any{"acme:sid": conversation("hello acme")})
	cache := tempCache(t)

	stdout, stderr, err := runAsq("resume", "--cache", cache, "--dry-run", "sid")
	if err != nil || !strings.Contains(stdout, `argv: ["acme","session","--id=sid"]`) {
		t.Fatalf("resume = %q err=%v", stdout, err)
	}
	requireNoWarning(t, stderr)

	stdout, _, err = runAsq("list", "--cache", cache, "--fzf", "--all")
	if err != nil {
		t.Fatalf("list err = %v", err)
	}
	if field := strings.Split(stdout, "\t")[fzfSourceField]; !strings.Contains(field, "\x1b[38;2;17;34;51m") {
		t.Fatalf("source field = %q", field)
	}

	var line, preview string
	stop := errors.New("stop")
	app := App{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	app.Pick = func(items []tui.Item, previewOf func(int) string, query string) (int, error) {
		line, preview = items[0].Display, previewOf(0)
		return 0, stop
	}
	if err := app.Run([]string{"pick", "--cache", cache}); !errors.Is(err, stop) {
		t.Fatalf("pick err = %v", err)
	}
	if !strings.Contains(line, "\x1b[38;2;17;34;51m") {
		t.Fatalf("picker line = %q", line)
	}
	if !strings.Contains(preview, "\x1b[48;2;17;34;51;") {
		t.Fatalf("preview = %q", preview)
	}
}

func TestPickRunsUserDefinedCommandInSessionCWD(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	useAgentsFile(t, `{"agents":[{"source":"acme","command":["acme","session","--id={id}"]}]}`)
	installAgentsviewStub(t,
		[]map[string]any{stubSession("acme:sid", "acme", "", projectDir, "hello acme")},
		map[string][]map[string]any{"acme:sid": stubMessages("hello acme")})
	outFile := filepath.Join(t.TempDir(), "resume.out")
	binDir := t.TempDir()
	writeStubScript(t, filepath.Join(binDir, "acme"), fmt.Sprintf("#!/bin/sh\n{ pwd; printf '%%s\\n' \"$@\"; } > %q\n", outFile))
	t.Setenv("PATH", binDir)

	app := App{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}
	app.Pick = func([]tui.Item, func(int) string, string) (int, error) { return 0, nil }
	if err := app.Run([]string{"pick", "--cache", tempCache(t)}); err != nil {
		t.Fatalf("pick err = %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("acme did not run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 || lines[1] != "session" || lines[2] != "--id=sid" {
		t.Fatalf("acme recorded %q", string(data))
	}
	wantDir, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if lines[0] != wantDir && lines[0] != projectDir {
		t.Fatalf("acme ran in %q, want %q", lines[0], projectDir)
	}
}

func TestUserDefinedAgentOverridesBuiltinCommandAndKeepsItsColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	projectDir := makeDir(t, t.TempDir(), "proj")
	useAgentsFile(t, `{"agents":[{"source":"claude","command":["my-claude","--continue","{id}"]}]}`)
	installAgentsviewStub(t,
		[]map[string]any{stubSession("sid", "claude", "", projectDir, "hello claude")},
		map[string][]map[string]any{"sid": stubMessages("hello claude")})
	cache := tempCache(t)

	stdout, stderr, err := runAsq("resume", "--cache", cache, "--dry-run", "sid")
	if err != nil || !strings.Contains(stdout, `argv: ["my-claude","--continue","sid"]`) {
		t.Fatalf("resume = %q err=%v", stdout, err)
	}
	requireNoWarning(t, stderr)
	stdout, _, err = runAsq("list", "--cache", cache, "--fzf", "--all")
	if err != nil {
		t.Fatalf("list err = %v", err)
	}
	claudeColor := "\x1b[" + source.Builtin().ANSIColor(source.Claude) + "m"
	if field := strings.Split(stdout, "\t")[fzfSourceField]; !strings.Contains(field, claudeColor) {
		t.Fatalf("source field = %q, want %q", field, claudeColor)
	}
}

func TestColorSourceInLabelUsesTheLoadedRegistry(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	useAgentsFile(t, `{"agents":[
		{"source":"acme","command":["acme","{id}"],"color":"#112233"},
		{"source":"claude","command":["my-claude","{id}"],"color":"#445566"}
	]}`)
	reg, err := source.Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	label := "  acme acme-0001 main"
	want := "  \x1b[38;2;17;34;51macme\x1b[0m acme-0001 main"
	if got := colorSourceInLabel(reg, label, "acme"); got != want {
		t.Fatalf("user-defined agent: got %q, want %q", got, want)
	}
	wantClaude := "  \x1b[38;2;68;85;102mclaude\x1b[0m sid"
	if got := colorSourceInLabel(reg, "  claude sid", source.Claude); got != wantClaude {
		t.Fatalf("overridden color: got %q, want %q", got, wantClaude)
	}
	if got := colorSourceInLabel(source.Builtin(), label, "acme"); got == want {
		t.Fatalf("built-in registry colored the user-defined agent: %q", got)
	}
}

func TestOpenInterpreterResumesWithTheBuiltinTable(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("open-interpreter:sid", "open-interpreter", "", projectDir, "hello interpreter")},
		map[string][]map[string]any{"open-interpreter:sid": stubMessages("hello interpreter")})
	cache := tempCache(t)

	stdout, stderr, err := runAsq("resume", "--cache", cache, "--dry-run", "sid")
	if err != nil || !strings.Contains(stdout, `argv: ["interpreter","resume","sid"]`) {
		t.Fatalf("resume = %q err=%v", stdout, err)
	}
	requireNoWarning(t, stderr)
	stdout, _, err = runAsq("list", "--cache", cache, "--fzf", "--all")
	if err != nil {
		t.Fatalf("list err = %v", err)
	}
	if field := strings.Split(stdout, "\t")[fzfSourceField]; !strings.Contains(field, "\x1b[38;2;244;114;182m") {
		t.Fatalf("source field = %q", field)
	}
}

func TestUnusableAgentsFileFallsBackToBuiltinsWithOneWarning(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{
			stubSession("sid", "claude", "", projectDir, "hello claude"),
			stubSession("acme:sid-acme", "acme", "", projectDir, "hello acme"),
		},
		map[string][]map[string]any{
			"sid":           stubMessages("hello claude"),
			"acme:sid-acme": stubMessages("hello acme"),
		})
	cache := tempCache(t)
	cases := []struct{ name, content string }{
		{"broken JSON", `{"agents":[`},
		{"empty command", `{"agents":[{"source":"acme","command":[]}]}`},
		{"command without {id}", `{"agents":[{"source":"acme","command":["acme","resume"]}]}`},
		{"bad color", `{"agents":[{"source":"acme","command":["acme","{id}"],"color":"red"}]}`},
		{"one bad entry among good ones", `{"agents":[{"source":"acme","command":["acme","{id}"]},{"source":"claude","command":["my-claude"]}]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := useAgentsFile(t, c.content)
			stdout, stderr, err := runAsq("resume", "--cache", cache, "--dry-run", "sid")
			if err != nil || !strings.Contains(stdout, `argv: ["claude","--resume","sid"]`) {
				t.Fatalf("resume = %q err=%v", stdout, err)
			}
			requireOneWarning(t, stderr, path)
			_, stderr, err = runAsq("resume", "--cache", cache, "--dry-run", "sid-acme")
			if err == nil || !strings.Contains(err.Error(), "resume is not supported for acme sessions") {
				t.Fatalf("err = %v", err)
			}
			requireOneWarning(t, stderr, path)
		})
	}

	t.Run("unreadable file", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(source.EnvAgentsFile, dir)
		stdout, stderr, err := runAsq("resume", "--cache", cache, "--dry-run", "sid")
		if err != nil || !strings.Contains(stdout, `argv: ["claude","--resume","sid"]`) {
			t.Fatalf("resume = %q err=%v", stdout, err)
		}
		requireOneWarning(t, stderr, dir)
	})
}

func TestMissingAgentsFileIsSilent(t *testing.T) {
	projectDir := makeDir(t, t.TempDir(), "proj")
	installAgentsviewStub(t,
		[]map[string]any{stubSession("sid", "claude", "", projectDir, "hello claude")},
		map[string][]map[string]any{"sid": stubMessages("hello claude")})
	t.Setenv(source.EnvAgentsFile, filepath.Join(t.TempDir(), "agents.json"))
	stdout, stderr, err := runAsq("resume", "--cache", tempCache(t), "--dry-run", "sid")
	if err != nil || !strings.Contains(stdout, `argv: ["claude","--resume","sid"]`) {
		t.Fatalf("resume = %q err=%v", stdout, err)
	}
	requireNoWarning(t, stderr)
}

func TestVersionAndHelpDoNotReadAgentsFile(t *testing.T) {
	useAgentsFile(t, `{`)
	for _, command := range []string{"version", "help"} {
		stdout, stderr, err := runAsq(command)
		if err != nil || stdout == "" || stderr != "" {
			t.Fatalf("%s = %q stderr=%q err=%v", command, stdout, stderr, err)
		}
	}
}

func TestListSourceHelpIncludesUserDefinedAgents(t *testing.T) {
	useAgentsFile(t, `{"agents":[{"source":"acme","command":["acme","{id}"]}]}`)
	_, stderr, err := runAsq("list", "-h")
	if err != nil || !strings.Contains(stderr, "|open-interpreter|acme\n") {
		t.Fatalf("list -h = %q err=%v", stderr, err)
	}
}
