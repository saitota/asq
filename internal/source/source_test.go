package source

import (
	"slices"
	"testing"

	"github.com/saitota/asq/internal/session"
)

func TestAllDescriptorsAreComplete(t *testing.T) {
	for _, d := range All {
		if d.Source == "" {
			t.Fatalf("descriptor missing Source: %+v", d)
		}
		if d.ResumeCommand == nil {
			t.Fatalf("%s: missing ResumeCommand", d.Source)
		}
		if d.ANSIColor == "" {
			t.Fatalf("%s: missing ANSIColor", d.Source)
		}
		cmd, _ := d.ResumeCommand("id")
		if cmd == "" {
			t.Fatalf("%s: ResumeCommand returned empty command", d.Source)
		}
	}
}

func TestGetUnknownSource(t *testing.T) {
	if _, ok := Get("bogus"); ok {
		t.Fatal("expected unknown source to miss")
	}
	if got := ANSIColor("bogus"); got != DefaultANSIColor {
		t.Fatalf("expected default color, got %q", got)
	}
	if cmd, args, err := Resume("bogus", "id"); err == nil || cmd != "" || args != nil {
		t.Fatalf("expected unsupported error, got %q %v %v", cmd, args, err)
	}
}

func TestNonResumableAgentsStillGetAccentColors(t *testing.T) {
	for _, agent := range []string{"cowork", "qwen", "kimi"} {
		if got := ANSIColor(session.Source(agent)); got == DefaultANSIColor || got == "" {
			t.Fatalf("%s: expected an accent color, got %q", agent, got)
		}
		if _, _, err := Resume(session.Source(agent), "id"); err == nil {
			t.Fatalf("%s: expected resume to be unsupported", agent)
		}
	}
	// aider has no agentsview accent; it takes the gray default.
	if got := ANSIColor("aider"); got != DefaultANSIColor {
		t.Fatalf("aider: expected default color, got %q", got)
	}
}

func TestResumeChecksUnsupportedBeforeID(t *testing.T) {
	_, _, err := Resume("cowork", "bad id")
	if err == nil || err.Error() != "resume is not supported for cowork sessions" {
		t.Fatalf("got %v", err)
	}
}

func TestResumeValidatesID(t *testing.T) {
	for _, d := range All {
		for _, id := range []string{"", "a b", "a:b", "--flag;", "a/b", "id\n"} {
			_, _, err := Resume(d.Source, id)
			if err == nil || err.Error() != "invalid cached session id: "+id {
				t.Fatalf("%s %q: got %v", d.Source, id, err)
			}
		}
		cmd, args, err := Resume(d.Source, "Ab0._-")
		if err != nil || cmd == "" || len(args) == 0 {
			t.Fatalf("%s: got %q %v %v", d.Source, cmd, args, err)
		}
	}
}

func TestResumeUsesDescriptorValidID(t *testing.T) {
	orig := All
	t.Cleanup(func() { All = orig })
	All = []Descriptor{{
		Source:        "custom",
		ValidID:       func(id string) bool { return id == "a:b" },
		ResumeCommand: func(id string) (string, []string) { return "custom", []string{id} },
		ANSIColor:     "1",
	}}
	if _, args, err := Resume("custom", "a:b"); err != nil || len(args) != 1 || args[0] != "a:b" {
		t.Fatalf("got %v %v", args, err)
	}
	_, _, err := Resume("custom", "abc")
	if err == nil || err.Error() != "invalid cached session id: abc" {
		t.Fatalf("got %v", err)
	}
}

func TestTableFollowsAgentsview(t *testing.T) {
	cmd, args, err := Resume(OpenInterpreter, "sid")
	if err != nil || cmd != "interpreter" || !slices.Equal(args, []string{"resume", "sid"}) {
		t.Fatalf("open-interpreter: got %q %v %v", cmd, args, err)
	}
	colors := map[session.Source]string{
		"augure-code":      "38;2;163;230;53",
		"augure-desktop":   "38;2;167;139;250",
		"cline":            "38;2;167;139;250",
		"codebuddy":        "38;2;96;165;250",
		"codex-cloud":      "38;2;248;113;113",
		"crush":            "38;2;243;78;63",
		"evener":           "38;2;45;212;191",
		"fx":               "38;2;56;189;248",
		"junie":            "38;2;163;230;53",
		"muse":             "38;2;56;189;248",
		"open-interpreter": "38;2;244;114;182",
		"opencodereview":   "38;2;129;140;248",
		"stepcode":         "38;2;129;140;248",
		"tau":              "38;2;251;191;36",
	}
	for agent, want := range colors {
		if got := ANSIColor(agent); got != want {
			t.Fatalf("%s: color = %q, want %q", agent, got, want)
		}
	}
}
