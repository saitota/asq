// Package source is the single registry of how sources are wired into
// resume and display. Any agent from agentsview lists fine; only agents
// with a Descriptor can be resumed. No other package should switch on
// session.Source.
package source

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/saitota/asq/internal/session"
)

const (
	Claude     session.Source = "claude"
	Codex      session.Source = "codex"
	Cursor     session.Source = "cursor"
	Copilot    session.Source = "copilot"
	Gemini     session.Source = "gemini"
	OpenCode   session.Source = "opencode"
	Amp        session.Source = "amp"
	Kiro       session.Source = "kiro"
	Pi         session.Source = "pi"
	TraeX      session.Source = "traex"
	AugureCode session.Source = "augure-code"
)

type Descriptor struct {
	Source        session.Source
	ValidID       func(id string) bool // guards the id before it reaches exec argv; nil means defaultValidID
	ResumeCommand func(id string) (string, []string)
	ANSIColor     string
}

var defaultIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func defaultValidID(id string) bool {
	return defaultIDPattern.MatchString(id)
}

var All = []Descriptor{
	{
		Source: Claude,
		ResumeCommand: func(id string) (string, []string) {
			return "claude", []string{"--resume", id}
		},
		ANSIColor: "38;2;245;158;11",
	},
	{
		Source: Codex,
		ResumeCommand: func(id string) (string, []string) {
			return "codex", []string{"resume", id}
		},
		ANSIColor: "38;2;96;165;250",
	},
	{
		Source: Cursor,
		ResumeCommand: func(id string) (string, []string) {
			return "cursor-agent", []string{"--resume=" + id}
		},
		ANSIColor: "38;2;156;163;175",
	},
	{
		Source: Copilot,
		ResumeCommand: func(id string) (string, []string) {
			return "copilot", []string{"--resume=" + id}
		},
		ANSIColor: "38;2;167;139;250",
	},
	// The agents below and their commands follow resumeAgents in agentsview's
	// internal/server/resume.go, the table behind its own resume menu.
	{
		Source: Gemini,
		ResumeCommand: func(id string) (string, []string) {
			return "gemini", []string{"--resume", id}
		},
		ANSIColor: extraColors[Gemini],
	},
	{
		Source: OpenCode,
		ResumeCommand: func(id string) (string, []string) {
			return "opencode", []string{"--session", id}
		},
		ANSIColor: extraColors[OpenCode],
	},
	{
		Source: Amp,
		ResumeCommand: func(id string) (string, []string) {
			return "amp", []string{"--resume", id}
		},
		ANSIColor: extraColors[Amp],
	},
	{
		Source: Kiro,
		ResumeCommand: func(id string) (string, []string) {
			return "kiro-cli", []string{"chat", "--resume-id", id}
		},
		ANSIColor: extraColors[Kiro],
	},
	{
		Source: Pi,
		ResumeCommand: func(id string) (string, []string) {
			return "pi", []string{"--session", id}
		},
		ANSIColor: extraColors[Pi],
	},
	{
		Source: TraeX,
		ResumeCommand: func(id string) (string, []string) {
			return "traex", []string{"resume", id}
		},
		ANSIColor: extraColors[TraeX],
	},
	{
		Source: AugureCode,
		ResumeCommand: func(id string) (string, []string) {
			return "augure", []string{"resume", id}
		},
		ANSIColor: DefaultANSIColor,
	},
}

const DefaultANSIColor = "38;2;209;213;219"

// extraColors are agentsview's own dark-theme agent accents, from
// KNOWN_AGENTS in frontend/src/lib/utils/agents.ts with hex values resolved
// via frontend/src/app.css and kit-ui/src/lib/brand.css. Agents without an
// agentsview color fall back to DefaultANSIColor gray. Claude, Codex, Cursor
// and Copilot keep asq's established colors instead.
var extraColors = map[session.Source]string{
	"visualstudio-copilot": "38;2;96;165;250",  // blue
	"zed":                  "38;2;74;222;128",  // green
	"codebuff":             "38;2;251;191;36",  // amber
	"devin":                "38;2;248;113;113", // red
	"zencoder":             "38;2;248;113;113",
	"opencode":             "38;2;167;139;250", // purple
	"kilo":                 "38;2;167;139;250",
	"kilo-legacy":          "38;2;167;139;250",
	"cowork":               "38;2;56;189;248", // sky
	"iflow":                "38;2;56;189;248",
	"freebuff":             "38;2;56;189;248",
	"traex":                "38;2;243;78;63", // coral
	"amp":                  "38;2;243;78;63",
	"gemini":               "38;2;251;113;133", // rose
	"gemini-apps":          "38;2;251;113;133",
	"roocode":              "38;2;251;113;133",
	"openhands":            "38;2;45;212;191", // teal
	"vscode-copilot":       "38;2;45;212;191",
	"omnigent":             "38;2;45;212;191",
	"cursor-ide":           "38;2;176;176;176", // black
	"pi":                   "38;2;129;140;248", // indigo
	"prime-agent":          "38;2;129;140;248",
	"posit-assistant":      "38;2;129;140;248",
	"qwen":                 "38;2;34;211;238", // cyan
	"qwenpaw":              "38;2;34;211;238",
	"deepseek-tui":         "38;2;34;211;238",
	"deepseek-harness":     "38;2;34;211;238",
	"cortex":               "38;2;34;211;238",
	"qoder":                "38;2;34;211;238",
	"poolside":             "38;2;34;211;238",
	"openclaw":             "38;2;240;160;80", // orange
	"qclaw":                "38;2;240;160;80",
	"piebald":              "38;2;240;160;80",
	"vibe":                 "38;2;240;160;80",
	"kimi":                 "38;2;244;114;182", // pink
	"kimi-work":            "38;2;244;114;182",
	"claude-ai":            "38;2;167;139;250", // violet
	"workbuddy":            "38;2;167;139;250",
	"antigravity":          "38;2;167;139;250",
	"antigravity-cli":      "38;2;167;139;250",
	"chatgpt":              "38;2;163;230;53", // lime
	"kiro":                 "38;2;163;230;53",
	"kiro-ide":             "38;2;163;230;53",
}

func Get(s session.Source) (Descriptor, bool) {
	for _, d := range All {
		if d.Source == s {
			return d, true
		}
	}
	return Descriptor{}, false
}

func Resume(s session.Source, id string) (string, []string, error) {
	d, ok := Get(s)
	if !ok {
		return "", nil, fmt.Errorf("resume is not supported for %s sessions", s)
	}
	valid := d.ValidID
	if valid == nil {
		valid = defaultValidID
	}
	if !valid(id) {
		return "", nil, fmt.Errorf("invalid cached session id: %s", id)
	}
	cmd, args := d.ResumeCommand(id)
	return cmd, args, nil
}

func ANSIColor(s session.Source) string {
	if d, ok := Get(s); ok {
		return d.ANSIColor
	}
	if color, ok := extraColors[s]; ok {
		return color
	}
	return DefaultANSIColor
}

// List is "|"-joined for use in flag usage strings.
func List() string {
	names := make([]string, len(All))
	for i, d := range All {
		names[i] = string(d.Source)
	}
	return strings.Join(names, "|")
}
