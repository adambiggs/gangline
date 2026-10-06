package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate/tmux"
	"github.com/pelletier/go-toml/v2"
)

type settings struct {
	CapacityTimeout        time.Duration
	Session                string
	StateRoot              string
	Collar                 string
	Socket                 string
	ConfigDir              string
	CollarDir              string
	LaunchArgs             map[string][]string
	LaunchArgsJSON         string
	CodexPermissionProfile string
	Origins                map[string]string
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
	configDirEnv := cmd.environment("GANG_CONFIG_DIR")
	configDir := configDirEnv
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
	stateRootEnv := cmd.environment("GANG_STATE_ROOT")
	if stateRootEnv != "" {
		stateRoot = stateRootEnv
	}
	socketEnv := cmd.environment("GANG_TMUX_SOCKET")

	result := settings{
		Session:    "gangline",
		StateRoot:  stateRoot,
		Collar:     "claude",
		LaunchArgs: make(map[string][]string),
		Socket:     socketEnv,
		ConfigDir:  configDir,
		Origins:    make(map[string]string),
	}
	for name, value := range map[string]string{
		"GANG_CONFIG_DIR":  configDirEnv,
		"GANG_STATE_ROOT":  stateRootEnv,
		"GANG_TMUX_SOCKET": socketEnv,
	} {
		if value != "" {
			result.Origins[name] = "env"
		}
	}
	capacityTimeout := "5m"
	values := map[string]*string{
		"GANG_CAPACITY_TIMEOUT":         &capacityTimeout,
		"GANG_SESSION":                  &result.Session,
		"GANG_COLLAR":                   &result.Collar,
		"GANG_COLLARS":                  &result.CollarDir,
		"GANG_LAUNCH_ARGS":              &result.LaunchArgsJSON,
		"GANG_CODEX_PERMISSION_PROFILE": &result.CodexPermissionProfile,
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
			result.Origins[name] = "env"
		}
	}
	if cmd.team != "" {
		// A hitched pane inherits its team; the flag cannot move it elsewhere.
		if cmd.environment("GANG_AGENT_ID") != "" && cmd.team != result.Session {
			return settings{}, refuseError("in a hitched pane --team must name the selected team %q, not %q; set GANG_SESSION to act on another team", result.Session, cmd.team)
		}
		result.Session = cmd.team
		result.Origins["GANG_SESSION"] = "flag"
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
	result.CapacityTimeout, err = time.ParseDuration(capacityTimeout)
	if err != nil || result.CapacityTimeout <= 0 {
		return settings{}, fmt.Errorf("GANG_CAPACITY_TIMEOUT must be a positive duration")
	}
	if result.CollarDir != "" && !filepath.IsAbs(result.CollarDir) {
		return settings{}, fmt.Errorf("GANG_COLLARS must be an absolute path")
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
	if result.CodexPermissionProfile != "" && !codexProfileName(result.CodexPermissionProfile) {
		return settings{}, fmt.Errorf("GANG_CODEX_PERMISSION_PROFILE must contain only letters, digits, hyphens, or underscores")
	}
	return result, nil
}

var configurationKeys = map[string]bool{
	"GANG_CAPACITY_TIMEOUT":         true,
	"GANG_COLLAR":                   true,
	"GANG_SESSION":                  true,
	"GANG_COLLARS":                  true,
	"GANG_LAUNCH_ARGS":              true,
	"GANG_CODEX_PERMISSION_PROFILE": true,
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

func applyLaunchPolicy(command harness.Command, collar string, settings settings) harness.Command {
	result := command
	arguments := settings.LaunchArgs[collar]
	result.Args = append(append([]string(nil), command.Args...), arguments...)
	return result
}

func repositoryGitdirs(dir string) []string {
	gitdir := linkedWorktreeGitdir(dir)
	if gitdir == "" {
		return nil
	}
	// The verified backlink and layout bind these paths to this worktree's
	// common directory. Grant storage, not shared config or executable hooks.
	common := filepath.Dir(filepath.Dir(gitdir))
	paths := []string{gitdir}
	for _, name := range []string{"objects", "refs", "logs"} {
		path := filepath.Join(common, name)
		// Do not follow a storage symlink into an unrelated host directory or
		// create missing metadata merely to extend the sandbox's write roots.
		if info, err := os.Lstat(path); err == nil && info.IsDir() {
			paths = append(paths, path)
		}
	}
	return paths
}

func codexRepositoryLaunch(args []string, profile, dir string) ([]string, error) {
	return codexLaunch(args, profile, repositoryGitdirs(dir)...)
}

func linkedWorktreeGitdir(dir string) string {
	for {
		file := filepath.Join(dir, ".git")
		info, err := os.Lstat(file)
		if os.IsNotExist(err) {
			parent := filepath.Dir(dir)
			if parent == dir {
				return ""
			}
			dir = parent
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return "" // A primary checkout keeps .git as a directory.
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return ""
		}
		value, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
		if !ok || strings.ContainsAny(value, "\r\n") {
			return ""
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(dir, value)
		}
		return linkedGitdirFromPointer(value, file)
	}
}

func linkedGitdirFromPointer(path, file string) string {
	gitdir, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(gitdir, "gitdir"))
	if err != nil {
		return ""
	}
	backlink := strings.TrimSpace(string(data))
	if backlink == "" || strings.ContainsAny(backlink, "\r\n") {
		return ""
	}
	if !filepath.IsAbs(backlink) {
		backlink = filepath.Join(gitdir, backlink)
	}
	backlink, err = filepath.EvalSymlinks(backlink)
	if err != nil {
		return ""
	}
	file, err = filepath.EvalSymlinks(file)
	if err != nil {
		return ""
	}
	if backlink != file {
		return ""
	}
	data, err = os.ReadFile(filepath.Join(gitdir, "commondir"))
	if err != nil {
		return ""
	}
	commonPath := strings.TrimSpace(string(data))
	if commonPath == "" || strings.ContainsAny(commonPath, "\r\n") {
		return ""
	}
	if !filepath.IsAbs(commonPath) {
		commonPath = filepath.Join(gitdir, commonPath)
	}
	common, err := filepath.EvalSymlinks(commonPath)
	if err != nil {
		return ""
	}
	name, err := filepath.Rel(filepath.Join(common, "worktrees"), gitdir)
	if err != nil || name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) {
		return ""
	}
	return gitdir
}

func codexProfileName(profile string) bool {
	return profile != "" && strings.IndexFunc(profile, func(r rune) bool {
		return r != '-' && r != '_' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
	}) < 0
}

// codexLaunch selects the configured permission profile and makes the
// linked worktree's private Git directory and common storage writable. Codex
// marks the working directory's resolved gitdir read-only with an exact-path rule that only an exact grant overrides,
// so without one a profile's pattern grant over the repository's worktrees
// still refuses index and fetch writes. Codex refuses to start when given a
// writable directory under its read-only sandbox, so arguments that select
// that sandbox get no grant.
func codexLaunch(args []string, profile string, gitdirs ...string) ([]string, error) {
	result, err := codexProfileLaunch(args, profile)
	if err != nil || codexSandbox(result) == "read-only" {
		return result, err
	}
	result = append([]string(nil), result...)
	for _, gitdir := range gitdirs {
		if gitdir != "" {
			result = append(result, "--add-dir", gitdir)
		}
	}
	return result, nil
}

// codexSandbox returns the sandbox mode args select, or "" when they select
// none. Codex lets a sandbox flag win over a sandbox_mode config override.
func codexSandbox(args []string) string {
	flag, config := "", ""
	for index, arg := range args {
		switch {
		case arg == "-s" || arg == "--sandbox":
			if index+1 < len(args) {
				flag = args[index+1]
			}
		case strings.HasPrefix(arg, "--sandbox="):
			flag = strings.TrimPrefix(arg, "--sandbox=")
		case strings.HasPrefix(arg, "-s"):
			flag = strings.TrimPrefix(strings.TrimPrefix(arg, "-s"), "=")
		case arg == "-c" || arg == "--config" || strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "-c"):
			setting := arg
			if arg == "-c" || arg == "--config" {
				if index+1 == len(args) {
					continue
				}
				setting = args[index+1]
			} else {
				setting = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(setting, "--config="), "-c"), "=")
			}
			key, value, _ := strings.Cut(setting, "=")
			if strings.TrimSpace(key) == "sandbox_mode" {
				config = codexConfigString(value)
			}
		}
	}
	if flag != "" {
		return flag
	}
	return config
}

// codexConfigString reads a -c value the way Codex does: as a TOML value, or
// as a literal string when it does not parse.
func codexConfigString(value string) string {
	var parsed struct {
		Value any `toml:"value"`
	}
	if toml.Unmarshal([]byte("value = "+value), &parsed) == nil {
		text, _ := parsed.Value.(string)
		return text
	}
	return strings.Trim(strings.TrimSpace(value), `"'`)
}

func codexProfileLaunch(args []string, profile string) ([]string, error) {
	if profile == "" {
		return args, nil
	}
	if !codexProfileName(profile) {
		return nil, fmt.Errorf("invalid Codex permission profile name %q", profile)
	}
	for index, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-s") && arg != "-", strings.HasPrefix(arg, "-p") && arg != "-", arg == "--sandbox", strings.HasPrefix(arg, "--sandbox="), arg == "--profile", strings.HasPrefix(arg, "--profile="), arg == "--dangerously-bypass-approvals-and-sandbox", arg == "--yolo":
			return nil, fmt.Errorf("GANG_CODEX_PERMISSION_PROFILE conflicts with Codex argument %q", arg)
		case arg == "-c" || arg == "--config":
			if index+1 == len(args) {
				return nil, fmt.Errorf("Codex argument %q has no value", arg)
			}
			key, _, _ := strings.Cut(args[index+1], "=")
			key = strings.TrimSpace(key)
			if key == "default_permissions" || key == "sandbox_mode" || key == "profile" {
				return nil, fmt.Errorf("GANG_CODEX_PERMISSION_PROFILE conflicts with Codex setting %q", key)
			}
		case strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "-c"):
			value := strings.TrimPrefix(arg, "--config=")
			value = strings.TrimPrefix(value, "-c")
			value = strings.TrimPrefix(value, "=")
			key, _, _ := strings.Cut(value, "=")
			key = strings.TrimSpace(key)
			if key == "default_permissions" || key == "sandbox_mode" || key == "profile" {
				return nil, fmt.Errorf("GANG_CODEX_PERMISSION_PROFILE conflicts with Codex setting %q", key)
			}
		}
	}
	quoted, _ := json.Marshal(profile)
	return append(append([]string(nil), args...), "-c", "default_permissions="+string(quoted)), nil
}

func (cmd command) tmux(settings settings) (*tmux.Backend, error) {
	config := cmd.tmuxConfig(settings.Socket, settings.Session)
	config.Stdin, _ = cmd.stdin.(*os.File)
	config.Stdout, _ = outputFile(cmd.stdout)
	config.Stderr, _ = outputFile(cmd.stderr)
	return tmux.New(config)
}

func (cmd command) tmuxConfig(socket, session string) tmux.Config {
	stdin, _ := cmd.stdin.(*os.File)
	return tmux.Config{
		Binary:  valueOr(cmd.environment("GANG_TMUX"), "tmux"),
		Socket:  socket,
		Session: session,
		Stdin:   stdin,
		Stdout:  childOutput(cmd.stdout),
		Stderr:  cmd.stderr,
	}
}
