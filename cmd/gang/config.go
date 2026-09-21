package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type settings struct {
	Session   string
	StateRoot string
	Collar    string
	Socket    string
	ConfigDir string
}

func (cmd command) settings() (settings, error) {
	home, err := cmd.userHomeDir()
	if err != nil {
		return settings{}, fmt.Errorf("locate home directory: %w", err)
	}
	configRoot := cmd.getenv("XDG_CONFIG_HOME")
	if configRoot == "" {
		configRoot = filepath.Join(home, ".config")
	}
	configDir := cmd.getenv("GANG_CONFIG_DIR")
	if configDir == "" {
		configDir = filepath.Join(configRoot, "gangline")
	}
	stateRoot := cmd.getenv("XDG_STATE_HOME")
	if stateRoot == "" {
		stateRoot = filepath.Join(home, ".local", "state")
	}
	stateRoot = filepath.Join(stateRoot, "gangline")
	if configured := cmd.getenv("GANG_STATE_ROOT"); configured != "" {
		stateRoot = configured
	}

	result := settings{
		Session:   valueOr(cmd.getenv("GANG_SESSION"), "gangline"),
		StateRoot: stateRoot,
		Collar:    valueOr(cmd.getenv("GANG_COLLAR"), "claude-code"),
		Socket:    cmd.getenv("GANG_TMUX_SOCKET"),
		ConfigDir: configDir,
	}
	for label, value := range map[string]string{
		"GANG_SESSION":    result.Session,
		"GANG_STATE_ROOT": result.StateRoot,
		"GANG_CONFIG_DIR": result.ConfigDir,
	} {
		if strings.TrimSpace(value) == "" {
			return settings{}, fmt.Errorf("%s must not be blank", label)
		}
	}
	return result, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
