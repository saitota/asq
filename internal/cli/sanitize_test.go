package cli

import (
	"bytes"
	"strings"
	"testing"
)

const evilText = "evil\x1b[2Jname\x07\ttab"

func evilApp(t *testing.T) (App, *bytes.Buffer, string) {
	t.Helper()
	projectDir := makeDir(t, t.TempDir(), "proj")
	entry := stubSession("sid-evil", "claude", evilText, projectDir, "hello")
	entry["git_branch"] = evilText
	installAgentsviewStub(t, []map[string]any{entry}, map[string][]map[string]any{"sid-evil": stubMessages("hello")})
	var stdout bytes.Buffer
	return App{Stdout: &stdout, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("")}, &stdout, tempCache(t)
}

func TestListTableAndVerboseStripSessionControlChars(t *testing.T) {
	for _, flag := range []string{"", "--verbose"} {
		app, stdout, cache := evilApp(t)
		args := []string{"list", "--cache", cache, "--all"}
		if flag != "" {
			args = append(args, flag)
		}
		if err := app.Run(args); err != nil {
			t.Fatal(err)
		}
		out := stdout.String()
		if strings.ContainsAny(out, "\x1b\x07") {
			t.Fatalf("%q output contains ESC or BEL: %q", flag, out)
		}
		if flag == "--verbose" && strings.Count(strings.TrimSpace(out), "\t") != 5 {
			t.Fatalf("verbose columns shifted: %q", out)
		}
	}
}

func TestListFzfKeepsFieldCountWithControlChars(t *testing.T) {
	app, stdout, cache := evilApp(t)
	if err := app.Run([]string{"list", "--cache", cache, "--all", "--fzf"}); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimRight(stdout.String(), "\n")
	if got := len(strings.Split(line, "\t")); got != fzfFieldCount {
		t.Fatalf("fields = %d, want %d: %q", got, fzfFieldCount, line)
	}
	if got := selectionKeyFromSelection(line); got != "sid-evil" {
		t.Fatalf("id = %q", got)
	}
}

func TestListJSONKeepsRawName(t *testing.T) {
	app, stdout, cache := evilApp(t)
	if err := app.Run([]string{"list", "--cache", cache, "--all", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `evil\u001b[2Jname\u0007\ttab`) {
		t.Fatalf("json name altered: %s", stdout.String())
	}
}
