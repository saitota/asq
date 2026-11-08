package buildinfo

import (
	"runtime/debug"
	"strings"
)

// Build commit truncation is display-only and unrelated to session ID prefixes.
const shortCommitLength = 12

var (
	Version   = "dev"
	BuildDate = ""
	Commit    = "unknown"
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if trimmed := strings.TrimSpace(setting.Value); trimmed != "" {
				Commit = trimmed
			}
		case "vcs.modified":
			if strings.TrimSpace(setting.Value) == "true" && Commit != "unknown" {
				Commit += "-dirty"
			}
		}
	}
}

func ShortCommit() string {
	trimmed := strings.TrimSpace(Commit)
	if trimmed == "" || trimmed == "unknown" {
		return "unknown"
	}
	if len(trimmed) <= shortCommitLength {
		return trimmed
	}
	return trimmed[:shortCommitLength]
}

func DisplayVersion() string {
	if strings.TrimSpace(BuildDate) == "" {
		return Version
	}
	return Version + " (built " + BuildDate + ")"
}
