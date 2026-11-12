package source

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/saitota/asq/internal/session"
)

func useAgentsFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agents.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvAgentsFile, path)
	return path
}

func TestLoadWithoutFileKeepsBuiltins(t *testing.T) {
	t.Setenv(EnvAgentsFile, filepath.Join(t.TempDir(), "missing.json"))
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	if !reflect.DeepEqual(reg, Builtin()) {
		t.Fatalf("Load = %+v, want the built-ins", reg)
	}
}

func TestLoadAddsAgentsAndOverridesBuiltins(t *testing.T) {
	useAgentsFile(t, `{"agents":[
		{"source":"acme","command":["acme","run","--session={id}"],"color":"#112233"},
		{"source":"claude","command":["my-claude","--continue","{id}"]},
		{"source":"open-interpreter","command":["oi","{id}"],"color":"#F472B6"}
	]}`)
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	cases := []struct {
		source session.Source
		cmd    string
		args   []string
		color  string
	}{
		{"acme", "acme", []string{"run", "--session=sid"}, "38;2;17;34;51"},
		{Claude, "my-claude", []string{"--continue", "sid"}, Builtin().ANSIColor(Claude)},
		{OpenInterpreter, "oi", []string{"sid"}, "38;2;244;114;182"},
		{Codex, "codex", []string{"resume", "sid"}, Builtin().ANSIColor(Codex)},
	}
	for _, c := range cases {
		cmd, args, err := reg.Resume(c.source, "sid")
		if err != nil || cmd != c.cmd || !reflect.DeepEqual(args, c.args) {
			t.Fatalf("%s: got %q %v %v, want %q %v", c.source, cmd, args, err, c.cmd, c.args)
		}
		if got := reg.ANSIColor(c.source); got != c.color {
			t.Fatalf("%s: color = %q, want %q", c.source, got, c.color)
		}
	}
	if got, want := reg.List(), Builtin().List()+"|acme"; got != want {
		t.Fatalf("List = %q, want %q", got, want)
	}
}

func TestLoadDoesNotChangeBuiltins(t *testing.T) {
	useAgentsFile(t, `{"agents":[
		{"source":"acme","command":["acme","{id}"]},
		{"source":"claude","command":["my-claude","{id}"],"color":"#112233"}
	]}`)
	if _, err := Load(); err != nil {
		t.Fatalf("Load err = %v", err)
	}
	reg := Builtin()
	if _, ok := reg.Get("acme"); ok {
		t.Fatal("user-defined agent leaked into the built-ins")
	}
	cmd, args, err := reg.Resume(Claude, "sid")
	if err != nil || cmd != "claude" || !reflect.DeepEqual(args, []string{"--resume", "sid"}) {
		t.Fatalf("got %q %v %v", cmd, args, err)
	}
	if got := reg.ANSIColor(Claude); got != "38;2;245;158;11" {
		t.Fatalf("claude color = %q", got)
	}
}

func TestLoadColorOfUserDefinedAgentWithoutColor(t *testing.T) {
	useAgentsFile(t, `{"agents":[
		{"source":"cowork","command":["cowork","{id}"]},
		{"source":"aider","command":["aider","{id}"]}
	]}`)
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	if got, want := reg.ANSIColor("cowork"), extraColors["cowork"]; got != want || got == DefaultANSIColor {
		t.Fatalf("cowork color = %q, want %q", got, want)
	}
	if got := reg.ANSIColor("aider"); got != DefaultANSIColor {
		t.Fatalf("aider color = %q, want the default", got)
	}
}

func TestLoadSubstitutesIDInEveryElement(t *testing.T) {
	useAgentsFile(t, `{"agents":[{"source":"acme","command":["acme","--id={id}","{id}","<{id}|{id}>"]}]}`)
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	cmd, args, err := reg.Resume("acme", "s.1")
	want := []string{"--id=s.1", "s.1", "<s.1|s.1>"}
	if err != nil || cmd != "acme" || !reflect.DeepEqual(args, want) {
		t.Fatalf("got %q %v %v, want %v", cmd, args, err, want)
	}
}

func TestLoadKeepsDefaultIDValidation(t *testing.T) {
	useAgentsFile(t, `{"agents":[{"source":"acme","command":["acme","{id}"]}]}`)
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	for _, id := range []string{"", "a b", "a:b", "--flag;", "a/b", "id\n"} {
		_, _, err := reg.Resume("acme", id)
		if err == nil || err.Error() != "invalid cached session id: "+id {
			t.Fatalf("%q: got %v", id, err)
		}
	}
}

func TestLoadAcceptsAnEmptyList(t *testing.T) {
	for _, content := range []string{`{}`, `{"agents":[]}`, `{"agents":null}`} {
		useAgentsFile(t, content)
		reg, err := Load()
		if err != nil || reg.List() != Builtin().List() {
			t.Fatalf("%s: got %q %v", content, reg.List(), err)
		}
	}
}

func TestLoadRejectsUnusableFiles(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"broken JSON", `{"agents":[`, "unexpected end of JSON input"},
		{"empty file", ``, "unexpected end of JSON input"},
		{"trailing data", `{} {}`, "after top-level value"},
		{"wrong shape", `{"agents":{}}`, "cannot unmarshal object"},
		{"unknown top-level key", `{"agent":[{"source":"acme","command":["acme","{id}"]}]}`, `unknown field "agent"`},
		{"unknown key in an entry", `{"agents":[{"source":"acme","command":["acme","{id}"],"colour":"#ff0000"}]}`, `unknown field "colour"`},
		{"missing source", `{"agents":[{"command":["acme","{id}"]}]}`, "agents[0]: source is required"},
		{"missing command", `{"agents":[{"source":"acme"}]}`, "agents[0]: command is required"},
		{"empty command", `{"agents":[{"source":"acme","command":[]}]}`, "agents[0]: command is required"},
		{"empty program", `{"agents":[{"source":"acme","command":["","{id}"]}]}`, "agents[0]: command[0] is empty"},
		{"no placeholder", `{"agents":[{"source":"acme","command":["acme","resume"]}]}`, "agents[0]: command must contain {id}"},
		{"named color", `{"agents":[{"source":"acme","command":["acme","{id}"],"color":"red"}]}`, `color "red" must be #RRGGBB`},
		{"short color", `{"agents":[{"source":"acme","command":["acme","{id}"],"color":"#fff"}]}`, `color "#fff" must be #RRGGBB`},
		{"color without hash", `{"agents":[{"source":"acme","command":["acme","{id}"],"color":"f472b6"}]}`, `color "f472b6" must be #RRGGBB`},
		{"color with alpha", `{"agents":[{"source":"acme","command":["acme","{id}"],"color":"#f472b6ff"}]}`, `must be #RRGGBB`},
		{"non-hex color", `{"agents":[{"source":"acme","command":["acme","{id}"],"color":"#gggggg"}]}`, `must be #RRGGBB`},
		{"empty color", `{"agents":[{"source":"acme","command":["acme","{id}"],"color":""}]}`, `color "" must be #RRGGBB`},
		{"duplicate source", `{"agents":[{"source":"acme","command":["acme","{id}"]},{"source":"acme","command":["acme2","{id}"]}]}`, `agents[1]: duplicate source "acme"`},
		{"duplicate source with a newline", `{"agents":[{"source":"a\nb","command":["a","{id}"]},{"source":"a\nb","command":["b","{id}"]}]}`, `agents[1]: duplicate source "a\nb"`},
		{"one bad entry among good ones", `{"agents":[{"source":"acme","command":["acme","{id}"]},{"source":"beta","command":["beta"]}]}`, "agents[1]: command must contain {id}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := useAgentsFile(t, c.content)
			reg, err := Load()
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), path) {
				t.Fatalf("err = %v, want one naming %s and containing %q", err, path, c.want)
			}
			if strings.Contains(err.Error(), "\n") {
				t.Fatalf("err spans lines: %q", err)
			}
			if !reflect.DeepEqual(reg, Registry{}) {
				t.Fatalf("Load returned %+v with an error", reg)
			}
		})
	}
}

func TestLoadRejectsUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvAgentsFile, dir)
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), dir) {
		t.Fatalf("err = %v, want one naming %s", err, dir)
	}
}

func TestParseColor(t *testing.T) {
	for value, want := range map[string]string{
		"#f472b6": "38;2;244;114;182",
		"#F472B6": "38;2;244;114;182",
		"#000000": "38;2;0;0;0",
		"#ffffff": "38;2;255;255;255",
	} {
		if got, err := parseColor(value); err != nil || got != want {
			t.Fatalf("%s: got %q %v, want %q", value, got, err, want)
		}
	}
}

func TestAgentsFilePath(t *testing.T) {
	t.Setenv(EnvAgentsFile, "/tmp/custom/agents.json")
	if path, ok := agentsFilePath(); !ok || path != "/tmp/custom/agents.json" {
		t.Fatalf("got %q %v", path, ok)
	}

	t.Setenv(EnvAgentsFile, "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "asq", "agents.json")
	if path, ok := agentsFilePath(); !ok || path != want {
		t.Fatalf("got %q %v, want %q", path, ok, want)
	}

	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if path, ok := agentsFilePath(); ok {
		t.Fatalf("got %q without a config directory", path)
	}
	if reg, err := Load(); err != nil || !reflect.DeepEqual(reg, Builtin()) {
		t.Fatalf("Load without a config directory = %+v %v", reg, err)
	}
}

func TestLoadReadsTheDefaultLocation(t *testing.T) {
	t.Setenv(EnvAgentsFile, "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "asq"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "asq", "agents.json")
	if err := os.WriteFile(file, []byte(`{"agents":[{"source":"acme","command":["acme","{id}"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load err = %v", err)
	}
	if _, _, err := reg.Resume("acme", "sid"); err != nil {
		t.Fatalf("acme from %s: %v", file, err)
	}
}
