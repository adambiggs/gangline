package store

import (
	"fmt"
	"os"
	"path/filepath"
)

const LayoutVersion = "v1"

type Paths struct {
	Root string
}

type TeamPaths struct {
	Directory string
	Events    string
	Snapshot  string
	Lock      string
}

func DefaultPaths() (Paths, error) {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve state home: %w", err)
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return Paths{Root: filepath.Join(stateHome, "gangline")}, nil
}

func (paths Paths) Team(team string) (TeamPaths, error) {
	if paths.Root == "" {
		return TeamPaths{}, fmt.Errorf("state root is empty")
	}
	if team == "" || filepath.Base(team) != team || team == "." || team == ".." {
		return TeamPaths{}, fmt.Errorf("team name %q is not a path segment", team)
	}
	directory := filepath.Join(paths.Root, LayoutVersion, team)
	return TeamPaths{
		Directory: directory,
		Events:    filepath.Join(directory, "events.jsonl"),
		Snapshot:  filepath.Join(directory, "snapshot.json"),
		Lock:      filepath.Join(directory, "team.lock"),
	}, nil
}
