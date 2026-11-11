package source

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/saitota/asq/internal/session"
)

const EnvAgentsFile = "ASQ_AGENTS_FILE"

const idPlaceholder = "{id}"

type agentsFile struct {
	Agents []agentEntry `json:"agents"`
}

type agentEntry struct {
	Source  session.Source `json:"source"`
	Command []string       `json:"command"`
	Color   *string        `json:"color"`
}

func Load() (Registry, error) {
	path, ok := agentsFilePath()
	if !ok {
		return Builtin(), nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Builtin(), nil
	}
	if err != nil {
		return Registry{}, err
	}
	user, err := parseAgents(data)
	if err != nil {
		return Registry{}, fmt.Errorf("%s: %w", path, err)
	}
	return Registry{user: user}, nil
}

func agentsFilePath() (string, bool) {
	if path := os.Getenv(EnvAgentsFile); path != "" {
		return path, true
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(dir, "asq", "agents.json"), true
}

func parseAgents(data []byte) ([]Descriptor, error) {
	var file agentsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	descriptors := make([]Descriptor, 0, len(file.Agents))
	for i, entry := range file.Agents {
		d, err := entry.descriptor()
		if err != nil {
			return nil, fmt.Errorf("agents[%d]: %w", i, err)
		}
		if _, dup := find(descriptors, d.Source); dup {
			return nil, fmt.Errorf("agents[%d]: duplicate source %s", i, d.Source)
		}
		descriptors = append(descriptors, d)
	}
	return descriptors, nil
}

func (e agentEntry) descriptor() (Descriptor, error) {
	switch {
	case e.Source == "":
		return Descriptor{}, errors.New("source is required")
	case len(e.Command) == 0:
		return Descriptor{}, errors.New("command is required")
	case e.Command[0] == "":
		return Descriptor{}, errors.New("command[0] is empty")
	case !slices.ContainsFunc(e.Command, func(arg string) bool { return strings.Contains(arg, idPlaceholder) }):
		return Descriptor{}, fmt.Errorf("command must contain %s", idPlaceholder)
	}
	color := Builtin().ANSIColor(e.Source)
	if e.Color != nil {
		var err error
		if color, err = parseColor(*e.Color); err != nil {
			return Descriptor{}, err
		}
	}
	// ValidID stays nil so the id passes the same guard as for built-ins.
	return Descriptor{
		Source: e.Source,
		ResumeCommand: func(id string) (string, []string) {
			argv := make([]string, len(e.Command))
			for i, arg := range e.Command {
				argv[i] = strings.ReplaceAll(arg, idPlaceholder, id)
			}
			return argv[0], argv[1:]
		},
		ANSIColor: color,
	}, nil
}

func parseColor(value string) (string, error) {
	digits, ok := strings.CutPrefix(value, "#")
	rgb, err := hex.DecodeString(digits)
	if !ok || err != nil || len(rgb) != 3 {
		return "", fmt.Errorf("color %q must be #RRGGBB", value)
	}
	return fmt.Sprintf("38;2;%d;%d;%d", rgb[0], rgb[1], rgb[2]), nil
}
