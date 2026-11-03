package store

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adambiggs/gangline/core"
)

// RetainLog links the audit outside removable runtime state. The caller holds
// the team lock. A retry uses the same identity; reusing a removed team name
// gets a new identity. No retention period or automatic deletion is implied.
func (p TeamPaths) RetainLog() (string, error) {
	directory, id, err := p.retainedLogDirectory()
	if err != nil {
		return "", err
	}
	// Create an empty audit for a team that has not emitted an event yet.
	f, err := os.OpenFile(p.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	path := filepath.Join(directory, id+".jsonl")
	return path, retainLogFile(p.Log, path)
}

// RetainHoldLog preserves the existing failed pane-hold diagnostic before its
// registration is removed or another launch can replace it. No file is made
// for a successful hold. Each failed launch keeps its own diagnostic.
func (p TeamPaths) RetainHoldLog(hitch core.HitchID) error {
	a, err := p.Agent(hitch)
	if err != nil {
		return err
	}
	source := filepath.Join(a.Directory, "hold")
	if _, err := os.Lstat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	directory, id, err := p.retainedLogDirectory()
	if err != nil {
		return err
	}
	path := filepath.Join(directory, id+"-hold-"+string(hitch)+"-"+rand.Text()+".log")
	if err := retainLogFile(source, path); err != nil {
		return err
	}
	return os.Remove(source)
}

func (p TeamPaths) retainedLogDirectory() (string, string, error) {
	marker := filepath.Join(p.Directory, "log-id.json")
	var id string
	if err := readJSON(marker, &id); errors.Is(err, os.ErrNotExist) {
		id = rand.Text()
		tmp, err := jsonTemp(p.Directory, id)
		if err != nil {
			return "", "", err
		}
		defer os.Remove(tmp)
		if err := os.Link(tmp, marker); errors.Is(err, os.ErrExist) {
			if err := readJSON(marker, &id); err != nil {
				return "", "", err
			}
		} else if err != nil {
			return "", "", err
		}
	} else if err != nil {
		return "", "", err
	}
	if err := segment(id); err != nil {
		return "", "", fmt.Errorf("invalid retained log identity: %w", err)
	}
	root := filepath.Dir(filepath.Dir(p.Directory))
	directory := filepath.Join(root, "logs", filepath.Base(p.Directory))
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", "", err
	}
	return directory, id, nil
}

func retainLogFile(from, to string) error {
	source, err := os.Lstat(from)
	if err != nil {
		return err
	}
	if !source.Mode().IsRegular() {
		return fmt.Errorf("log is not a regular file")
	}
	if err := os.Link(from, to); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	retained, err := os.Lstat(to)
	if err != nil {
		return err
	}
	if !os.SameFile(source, retained) {
		return fmt.Errorf("retained log path belongs to a different audit")
	}
	return nil
}
