package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type settings struct {
	Session         string
	StateRoot       string
	Collar          string
	Socket          string
	ConfigDir       string
	CollarDir       string
	Notify          string
	Scope           string
	ContextLights   string
	ContextBands    string
	CacheBands      string
	CacheCompaction string
	AutoResume      string
	LaunchArgs      map[string][]string
	LaunchArgsJSON  string
	Origins         map[string]string
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
	configDir := cmd.environment("GANG_CONFIG_DIR")
	if configDir == "" {
		configDir = filepath.Join(configRoot, "gangline")
	}
	if !filepath.IsAbs(configDir) {
		return settings{}, fmt.Errorf("GANG_CONFIG_DIR must be an absolute path")
	}
	configured, err := readConfiguration(filepath.Join(configDir, "config"))
	if err != nil {
		return settings{}, err
	}
	stateRoot := cmd.getenv("XDG_STATE_HOME")
	if stateRoot == "" {
		stateRoot = filepath.Join(home, ".local", "state")
	}
	stateRoot = filepath.Join(stateRoot, "gangline")
	if explicit := cmd.environment("GANG_STATE_ROOT"); explicit != "" {
		stateRoot = explicit
	}

	result := settings{
		Session:         "gangline",
		StateRoot:       stateRoot,
		Collar:          "claude-code",
		Notify:          "lead",
		Scope:           "off",
		ContextLights:   "collar",
		CacheCompaction: "claude-code=3600:300 codex=1800:180",
		AutoResume:      "off",
		LaunchArgs:      make(map[string][]string),
		Socket:          cmd.environment("GANG_TMUX_SOCKET"),
		ConfigDir:       configDir,
		Origins:         make(map[string]string),
	}
	values := map[string]*string{
		"GANG_SESSION":          &result.Session,
		"GANG_COLLAR":           &result.Collar,
		"GANG_COLLARS":          &result.CollarDir,
		"GANG_NOTIFY":           &result.Notify,
		"GANG_SCOPE":            &result.Scope,
		"GANG_CONTEXT_LIGHTS":   &result.ContextLights,
		"GANG_CONTEXT_BANDS":    &result.ContextBands,
		"GANG_CACHE_BANDS":      &result.CacheBands,
		"GANG_CACHE_COMPACTION": &result.CacheCompaction,
		"GANG_AUTO_RESUME":      &result.AutoResume,
		"GANG_LAUNCH_ARGS":      &result.LaunchArgsJSON,
	}
	for name, destination := range values {
		if value, ok := configured[name]; ok {
			*destination = value
			result.Origins[name] = "config"
		}
		if value, ok := cmd.environmentValue(name); ok {
			if value == "" {
				return settings{}, fmt.Errorf("%s must not be blank", name)
			}
			*destination = value
			result.Origins[name] = "environment"
		}
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
	if result.CollarDir != "" && !filepath.IsAbs(result.CollarDir) {
		return settings{}, fmt.Errorf("GANG_COLLARS must be an absolute path")
	}
	if result.Scope != "on" && result.Scope != "off" {
		return settings{}, fmt.Errorf("GANG_SCOPE must be on or off, got %q", result.Scope)
	}
	if result.LaunchArgsJSON != "" {
		if err := json.Unmarshal([]byte(result.LaunchArgsJSON), &result.LaunchArgs); err != nil {
			return settings{}, fmt.Errorf("GANG_LAUNCH_ARGS must be a JSON object of collar names to argument arrays: %w", err)
		}
		for collar, arguments := range result.LaunchArgs {
			if !collarNamePattern.MatchString(collar) {
				return settings{}, fmt.Errorf("GANG_LAUNCH_ARGS has invalid collar name %q", collar)
			}
			for _, argument := range arguments {
				if argument == "" || strings.ContainsRune(argument, 0) {
					return settings{}, fmt.Errorf("GANG_LAUNCH_ARGS[%q] contains an empty or NUL argument", collar)
				}
			}
		}
	}
	return result, nil
}

var configurationKeys = map[string]bool{
	"GANG_COLLAR":           true,
	"GANG_SESSION":          true,
	"GANG_COLLARS":          true,
	"GANG_NOTIFY":           true,
	"GANG_CONTEXT_LIGHTS":   true,
	"GANG_CONTEXT_BANDS":    true,
	"GANG_CACHE_BANDS":      true,
	"GANG_CACHE_COMPACTION": true,
	"GANG_AUTO_RESUME":      true,
	"GANG_SCOPE":            true,
	"GANG_LAUNCH_ARGS":      true,
}

func readConfiguration(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" || !configurationKeys[name] {
			return nil, fmt.Errorf("configuration line %d has unknown key %q", lineNumber, name)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("configuration line %d leaves %s blank", lineNumber, name)
		}
		if _, exists := values[name]; exists {
			return nil, fmt.Errorf("configuration line %d repeats %s", lineNumber, name)
		}
		values[name] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	return values, nil
}

func (cmd command) environment(name string) string {
	value, _ := cmd.environmentValue(name)
	return value
}

func (cmd command) environmentValue(name string) (string, bool) {
	if cmd.lookupEnv != nil {
		return cmd.lookupEnv(name)
	}
	if cmd.getenv == nil {
		return "", false
	}
	value := cmd.getenv(name)
	return value, value != ""
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
