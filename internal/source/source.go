// Package source is the single registry of how sources are wired into
// resume and display. Any agent from agentsview lists fine; only agents
// with a Descriptor can be resumed. No other package should switch on
// session.Source.
package source

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/saitota/asq/internal/ansitext"
	"github.com/saitota/asq/internal/session"
)

const (
	Claude          session.Source = "claude"
	Codex           session.Source = "codex"
	Cursor          session.Source = "cursor"
	Copilot         session.Source = "copilot"
	Gemini          session.Source = "gemini"
	OpenCode        session.Source = "opencode"
	Amp             session.Source = "amp"
	Kiro            session.Source = "kiro"
	Pi              session.Source = "pi"
	TraeX           session.Source = "traex"
	AugureCode      session.Source = "augure-code"
	OpenInterpreter session.Source = "open-interpreter"
)

type Descriptor struct {
	Source        session.Source
	ValidID       func(id string) bool // guards the id before it reaches exec argv; nil means defaultValidID
	ResumeCommand func(id string) (string, []string)
	Color         ansitext.RGB
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
		Color: ansitext.RGB{245, 158, 11},
	},
	{
		Source: Codex,
		ResumeCommand: func(id string) (string, []string) {
			return "codex", []string{"resume", id}
		},
		Color: ansitext.RGB{96, 165, 250},
	},
	{
		Source: Cursor,
		ResumeCommand: func(id string) (string, []string) {
			return "cursor-agent", []string{"--resume=" + id}
		},
		Color: ansitext.RGB{156, 163, 175},
	},
	{
		Source: Copilot,
		ResumeCommand: func(id string) (string, []string) {
			return "copilot", []string{"--resume=" + id}
		},
		Color: ansitext.RGB{167, 139, 250},
	},
	// The agents below and their commands follow resumeAgents in agentsview's
	// internal/server/resume.go, the table behind its own resume menu.
	{
		Source: Gemini,
		ResumeCommand: func(id string) (string, []string) {
			return "gemini", []string{"--resume", id}
		},
		Color: extraColors[Gemini],
	},
	{
		Source: OpenCode,
		ResumeCommand: func(id string) (string, []string) {
			return "opencode", []string{"--session", id}
		},
		Color: extraColors[OpenCode],
	},
	{
		Source: Amp,
		ResumeCommand: func(id string) (string, []string) {
			return "amp", []string{"--resume", id}
		},
		Color: extraColors[Amp],
	},
	{
		Source: Kiro,
		ResumeCommand: func(id string) (string, []string) {
			return "kiro-cli", []string{"chat", "--resume-id", id}
		},
		Color: extraColors[Kiro],
	},
	{
		Source: Pi,
		ResumeCommand: func(id string) (string, []string) {
			return "pi", []string{"--session", id}
		},
		Color: extraColors[Pi],
	},
	{
		Source: TraeX,
		ResumeCommand: func(id string) (string, []string) {
			return "traex", []string{"resume", id}
		},
		Color: extraColors[TraeX],
	},
	{
		Source: AugureCode,
		ResumeCommand: func(id string) (string, []string) {
			return "augure", []string{"resume", id}
		},
		Color: extraColors[AugureCode],
	},
	{
		Source: OpenInterpreter,
		ResumeCommand: func(id string) (string, []string) {
			return "interpreter", []string{"resume", id}
		},
		Color: extraColors[OpenInterpreter],
	},
}

var DefaultColor = ansitext.RGB{209, 213, 219}

// extraColors are agentsview's own dark-theme agent accents, from
// KNOWN_AGENTS in frontend/src/lib/utils/agents.ts with hex values resolved
// via frontend/src/app.css and kit-ui/src/lib/brand.css. Agents without an
// agentsview color fall back to DefaultColor gray. Claude, Codex, Cursor
// and Copilot keep asq's established colors instead.
var extraColors = map[session.Source]ansitext.RGB{
	"visualstudio-copilot": {96, 165, 250}, // blue
	"codebuddy":            {96, 165, 250},
	"zed":                  {74, 222, 128}, // green
	"codebuff":             {251, 191, 36}, // amber
	"tau":                  {251, 191, 36},
	"devin":                {248, 113, 113}, // red
	"zencoder":             {248, 113, 113},
	"codex-cloud":          {248, 113, 113},
	"opencode":             {167, 139, 250}, // purple
	"kilo":                 {167, 139, 250},
	"kilo-legacy":          {167, 139, 250},
	"cowork":               {56, 189, 248}, // sky
	"iflow":                {56, 189, 248},
	"freebuff":             {56, 189, 248},
	"fx":                   {56, 189, 248},
	"muse":                 {56, 189, 248},
	"traex":                {243, 78, 63}, // coral
	"amp":                  {243, 78, 63},
	"crush":                {243, 78, 63},
	"gemini":               {251, 113, 133}, // rose
	"gemini-apps":          {251, 113, 133},
	"roocode":              {251, 113, 133},
	"openhands":            {45, 212, 191}, // teal
	"vscode-copilot":       {45, 212, 191},
	"omnigent":             {45, 212, 191},
	"evener":               {45, 212, 191},
	"cursor-ide":           {176, 176, 176}, // black
	"pi":                   {129, 140, 248}, // indigo
	"prime-agent":          {129, 140, 248},
	"posit-assistant":      {129, 140, 248},
	"opencodereview":       {129, 140, 248},
	"stepcode":             {129, 140, 248},
	"qwen":                 {34, 211, 238}, // cyan
	"qwenpaw":              {34, 211, 238},
	"deepseek-tui":         {34, 211, 238},
	"deepseek-harness":     {34, 211, 238},
	"cortex":               {34, 211, 238},
	"qoder":                {34, 211, 238},
	"poolside":             {34, 211, 238},
	"openclaw":             {240, 160, 80}, // orange
	"qclaw":                {240, 160, 80},
	"piebald":              {240, 160, 80},
	"vibe":                 {240, 160, 80},
	"kimi":                 {244, 114, 182}, // pink
	"kimi-work":            {244, 114, 182},
	"open-interpreter":     {244, 114, 182},
	"claude-ai":            {167, 139, 250}, // violet
	"workbuddy":            {167, 139, 250},
	"antigravity":          {167, 139, 250},
	"antigravity-cli":      {167, 139, 250},
	"augure-desktop":       {167, 139, 250},
	"cline":                {167, 139, 250},
	"chatgpt":              {163, 230, 53}, // lime
	"kiro":                 {163, 230, 53},
	"kiro-ide":             {163, 230, 53},
	"augure-code":          {163, 230, 53},
	"junie":                {163, 230, 53},
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

func Color(s session.Source) ansitext.RGB {
	if d, ok := Get(s); ok {
		return d.Color
	}
	if color, ok := extraColors[s]; ok {
		return color
	}
	return DefaultColor
}

// List is "|"-joined for use in flag usage strings.
func List() string {
	names := make([]string, len(All))
	for i, d := range All {
		names[i] = string(d.Source)
	}
	return strings.Join(names, "|")
}
