package store

import (
	"fmt"
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

func (paths Paths) Team(team string) (TeamPaths, error) {
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
