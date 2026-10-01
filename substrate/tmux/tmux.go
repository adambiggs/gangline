// Package tmux drives panes through tmux.
package tmux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/adambiggs/gangline/substrate"
)

// Config identifies the tmux server and session a Backend drives. Socket is
// optional; when set, every invocation uses that private tmux socket.
type Config struct {
	Binary  string
	Socket  string
	Session string
	Stdin   *os.File
	Stdout  *os.File
	Stderr  *os.File
}

// Backend is a tmux implementation of substrate.Substrate.
type Backend struct {
	config  Config
	birthMu sync.Mutex
	birth   map[substrate.PaneID]PaneIdentity
}

type Window struct {
	Pane substrate.Pane
	Name string
}

func New(config Config) (*Backend, error) {
	if config.Session == "" {
		return nil, fmt.Errorf("tmux session is required")
	}
	if config.Binary == "" {
		config.Binary = "tmux"
	}
	return &Backend{config: config}, nil
}

func (backend *Backend) CreateSession(ctx context.Context, spec substrate.SpawnSpec) (substrate.Pane, error) {
	arguments, err := backend.launchArguments("new-session", spec)
	if err != nil {
		return substrate.Pane{}, err
	}
	arguments = append(arguments[:len(arguments)-1], "-s", backend.config.Session, arguments[len(arguments)-1])
	if spec.KeepExited {
		// This assumes the new session's only pane is the one the target
		// resolves to; a user after-new-session hook that splits the window
		// would make it the active pane instead.
		arguments = append(arguments, ";", "set-option", "-p", "-t", "="+backend.config.Session+":", "remain-on-exit", "on")
	}
	return backend.launch(ctx, "create session", arguments)
}

func (backend *Backend) SessionExists(ctx context.Context) (bool, error) {
	output, err := backend.run(ctx, "has-session", "-t", "="+backend.config.Session)
	if err == nil {
		return true, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, tmuxError("check session", err, output)
}

func (backend *Backend) Spawn(ctx context.Context, spec substrate.SpawnSpec) (substrate.Pane, error) {
	arguments, err := backend.launchArguments("new-window", spec)
	if err != nil {
		return substrate.Pane{}, err
	}
	arguments = append(arguments[:len(arguments)-1], "-t", "="+backend.config.Session+":", arguments[len(arguments)-1])
	if !spec.KeepExited {
		return backend.launch(ctx, "spawn pane", arguments)
	}
	// A detached window is not current, so a command after new-window would
	// reach another pane. The hook runs with the new pane as its target. It
	// takes its own global index so hooks already set still run, and it only
	// exists inside this command list.
	arguments = append([]string{"set-hook", "-g", keepExitedHook, "set-option -p remain-on-exit on", ";"}, arguments...)
	arguments = append(arguments, ";", "set-hook", "-gu", keepExitedHook)
	pane, err := backend.launch(ctx, "spawn pane", arguments)
	if err != nil {
		// Best effort: a list that stopped early may have left the hook set.
		// The launch may have failed on ctx's deadline, so the cleanup gets
		// its own.
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), keepExitedCleanup)
		defer cancel()
		_, _ = backend.run(cleanup, "set-hook", "-gu", keepExitedHook)
	}
	return pane, err
}

// keepExitedHook is a global hook slot reserved for one spawn's command list.
const keepExitedHook = "after-new-window[7193]"

// keepExitedCleanup bounds the removal of a hook a failed spawn left set.
const keepExitedCleanup = 5 * time.Second

// ReleaseExit restores the default close-on-exit behaviour for a pane spawned
// with KeepExited. A pane whose process already exited is left open and
// reported as an ExitedError carrying its final output.
func (backend *Backend) ReleaseExit(ctx context.Context, pane substrate.PaneID) error {
	if err := validPaneID(pane); err != nil {
		return err
	}
	// One command list: the process cannot exit between the check and the release.
	output, err := backend.run(ctx,
		"display-message", "-p", "-t", string(pane), "#{pane_dead},#{pane_dead_status}", ";",
		"capture-pane", "-p", "-J", "-S", "-", "-t", string(pane), ";",
		"set-option", "-p", "-u", "-t", string(pane), "remain-on-exit")
	if err != nil {
		return tmuxError("release exited pane", err, output)
	}
	state, history, _ := strings.Cut(output, "\n")
	dead, status, ok := strings.Cut(state, ",")
	if !ok {
		return fmt.Errorf("release exited pane: tmux returned %q", state)
	}
	if dead != "1" {
		return nil
	}
	return exited(status, history)
}

func (backend *Backend) exitedPane(ctx context.Context, pane substrate.PaneID, status string) error {
	history, err := backend.run(ctx, "capture-pane", "-p", "-J", "-S", "-", "-t", string(pane))
	if err != nil {
		return tmuxError("capture exited pane", err, history)
	}
	return exited(status, history)
}

// exitedOutputLines and exitedOutputBytes bound how much of an exited pane's
// history an error carries; one joined line can be long.
const (
	exitedOutputLines = 10
	exitedOutputBytes = 2048
)

func exited(status, history string) *substrate.ExitedError {
	var kept []string
	for _, line := range lines(history) {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		// tmux writes this notice into the pane when its process exits.
		if line == "" || strings.HasPrefix(line, "Pane is dead") {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) > exitedOutputLines {
		kept = kept[len(kept)-exitedOutputLines:]
	}
	output := strings.Join(kept, "\n")
	if len(output) > exitedOutputBytes {
		output = output[len(output)-exitedOutputBytes:]
		for output != "" && !utf8.RuneStart(output[0]) {
			output = output[1:]
		}
	}
	return &substrate.ExitedError{Status: status, Output: output}
}

func (backend *Backend) launchArguments(command string, spec substrate.SpawnSpec) ([]string, error) {
	if err := validWindowName(spec.Name); err != nil {
		return nil, err
	}
	if spec.Directory == "" {
		return nil, fmt.Errorf("spawn directory is required")
	}
	if spec.Command == "" {
		return nil, fmt.Errorf("spawn command is required")
	}
	arguments := []string{
		command, "-d", "-P", "-F", "#{pane_id}",
		"-n", escapeFormat(spec.Name),
		"-c", spec.Directory,
	}
	names := make([]string, 0, len(spec.Env))
	for name := range spec.Env {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := spec.Env[name]
		if name == "" || strings.ContainsAny(name, "=\x00") || strings.ContainsRune(value, '\x00') {
			return nil, fmt.Errorf("invalid spawn environment %q", name)
		}
		arguments = append(arguments, "-e", name+"="+value)
	}
	arguments = append(arguments, shellCommand(spec.Command, spec.Args))
	return arguments, nil
}

func (backend *Backend) launch(ctx context.Context, action string, arguments []string) (substrate.Pane, error) {
	generation, err := randomGeneration()
	if err != nil {
		return substrate.Pane{}, err
	}
	creatingSession := arguments[0] == "new-session"
	arguments = append(arguments, ";", "set-option", "-soq", generationOption, generation)
	if creatingSession {
		// new-session -e populates session environment. The initial pane already
		// inherited its capability; later ordinary panes must not inherit it.
		for _, key := range []string{"GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN", "GANGLINE_HITCH_ID"} {
			arguments = append(arguments, ";", "set-environment", "-r", "-t", "="+backend.config.Session, key)
		}
	}
	arguments = append(arguments, ";", "list-panes", "-s", "-t", "="+backend.config.Session+":", "-F", "#{"+generationOption+"}\t#{session_id}\t#{pane_id}")
	output, err := backend.run(ctx, arguments...)
	if err != nil {
		return substrate.Pane{}, tmuxError(action, err, output)
	}
	identifier, records, found := strings.Cut(output, "\n")
	if !found || !numericTmuxID(identifier, '%') {
		return substrate.Pane{}, fmt.Errorf("%s: tmux returned invalid pane id %q", action, output)
	}
	pane := substrate.PaneID(identifier)
	for _, line := range strings.Split(strings.TrimSuffix(records, "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 || fields[2] != identifier {
			continue
		}
		id := PaneIdentity{Generation: fields[0], Session: fields[1], Pane: fields[2]}
		if err := validPaneIdentity(id); err != nil {
			return substrate.Pane{}, fmt.Errorf("%s: %w", action, err)
		}
		backend.birthMu.Lock()
		if backend.birth == nil {
			backend.birth = make(map[substrate.PaneID]PaneIdentity)
		}
		backend.birth[pane] = id
		backend.birthMu.Unlock()
		return substrate.Pane{ID: pane}, nil
	}
	return substrate.Pane{}, fmt.Errorf("%s: tmux did not return the created pane's identity", action)
}

func (backend *Backend) Windows(ctx context.Context) ([]Window, error) {
	output, err := backend.run(ctx, "list-panes", "-s", "-t", "="+backend.config.Session+":", "-F", "#{pane_id}\t#{window_name}")
	if err != nil {
		return nil, tmuxError("list panes", err, output)
	}
	var windows []Window
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if line == "" {
			continue
		}
		id, name, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("list panes: malformed record %q", line)
		}
		pane := substrate.PaneID(id)
		if err := validPaneID(pane); err != nil {
			return nil, err
		}
		windows = append(windows, Window{Pane: substrate.Pane{ID: pane}, Name: name})
	}
	return windows, nil
}

func (backend *Backend) PaneNamed(ctx context.Context, name string) (substrate.Pane, error) {
	windows, err := backend.Windows(ctx)
	if err != nil {
		return substrate.Pane{}, err
	}
	var match substrate.Pane
	for _, window := range windows {
		if window.Name != name {
			continue
		}
		if match.ID != "" {
			return substrate.Pane{}, fmt.Errorf("window name %q is ambiguous", name)
		}
		match = window.Pane
	}
	if match.ID == "" {
		return substrate.Pane{}, fmt.Errorf("window name %q was not found", name)
	}
	return match, nil
}

// ForegroundCommand asks the tmux server, which can inspect its pane even when
// the calling process cannot read the host process table.
func (backend *Backend) ForegroundCommand(ctx context.Context, pane substrate.PaneID) (string, error) {
	if err := validPaneID(pane); err != nil {
		return "", err
	}
	output, err := backend.run(ctx, "display-message", "-p", "-t", string(pane), "#{pane_current_command}")
	if err != nil {
		return "", tmuxError("read pane foreground command", err, output)
	}
	command := strings.TrimSpace(output)
	if command == "" || strings.ContainsAny(command, "\r\n") {
		return "", fmt.Errorf("read pane foreground command: tmux returned %q", output)
	}
	return command, nil
}

func (backend *Backend) Rename(ctx context.Context, pane substrate.PaneID, name string) error {
	if err := validPaneID(pane); err != nil {
		return err
	}
	if err := validWindowName(name); err != nil {
		return err
	}
	output, err := backend.run(ctx, "rename-window", "-t", string(pane), "--", escapeFormat(name))
	if err != nil {
		return tmuxError("rename pane", err, output)
	}
	return nil
}

func (backend *Backend) SendKeys(ctx context.Context, pane substrate.PaneID, keys substrate.Keys) error {
	if err := validPaneID(pane); err != nil {
		return err
	}
	if keys.Text != "" {
		output, err := backend.run(ctx, "send-keys", "-t", string(pane), "-l", keys.Text)
		if err != nil {
			return tmuxError("send text", err, output)
		}
	}
	keyNames := append([]string(nil), keys.Names...)
	if keys.Submit {
		keyNames = append(keyNames, "Enter")
	}
	if len(keyNames) == 0 {
		return nil
	}
	arguments := append([]string{"send-keys", "-t", string(pane)}, keyNames...)
	output, err := backend.run(ctx, arguments...)
	if err != nil {
		return tmuxError("send keys", err, output)
	}
	return nil
}

func (backend *Backend) Capture(ctx context.Context, pane substrate.PaneID) (substrate.Screen, error) {
	if err := validPaneID(pane); err != nil {
		return substrate.Screen{}, err
	}
	raw, err := backend.run(ctx, "capture-pane", "-e", "-p", "-t", string(pane))
	if err != nil {
		return substrate.Screen{}, tmuxError("capture pane", err, raw)
	}
	cursor, dead, err := backend.cursor(ctx, pane)
	if err != nil {
		return substrate.Screen{}, err
	}
	if dead != nil {
		return substrate.Screen{}, backend.exitedPane(ctx, pane, *dead)
	}
	screen, err := parseScreen(raw, cursor)
	if err != nil {
		return substrate.Screen{}, fmt.Errorf("parse captured pane %q: %w", pane, err)
	}
	return screen, nil
}

func (backend *Backend) Kill(ctx context.Context, pane substrate.PaneID) (result error) {
	if err := validPaneID(pane); err != nil {
		return err
	}
	owned, err := backend.ownedProcesses(ctx, pane)
	if err != nil {
		return fmt.Errorf("record pane descendants: %w", err)
	}
	defer func() { result = errors.Join(result, closeOwnedProcesses(owned)) }()
	output, err := backend.run(ctx, "kill-window", "-t", string(pane))
	if err != nil {
		return tmuxError("kill pane", err, output)
	}
	return reapOwnedProcesses(ctx, owned)
}

func (backend *Backend) KillSession(ctx context.Context) error {
	output, err := backend.run(ctx, "kill-session", "-t", backend.config.Session)
	if err != nil {
		return tmuxError("kill session", err, output)
	}
	return nil
}

func (backend *Backend) Attach(ctx context.Context, pane substrate.PaneID) error {
	if err := validPaneID(pane); err != nil {
		return err
	}
	arguments := []string{"attach-session", "-t", backend.config.Session, ";", "select-window", "-t", string(pane)}
	if backend.config.Socket != "" {
		arguments = append([]string{"-S", backend.config.Socket}, arguments...)
	}
	command := exec.CommandContext(ctx, backend.config.Binary, arguments...)
	command.Stdin = backend.config.Stdin
	command.Stdout = backend.config.Stdout
	command.Stderr = backend.config.Stderr
	if command.Stdin == nil {
		command.Stdin = os.Stdin
	}
	if command.Stdout == nil {
		command.Stdout = os.Stdout
	}
	if command.Stderr == nil {
		command.Stderr = os.Stderr
	}
	err := command.Run()
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	return nil
}

// cursor also reports whether the pane's process exited; a non-nil status
// means it did, and holds the exit status, empty until tmux collects one.
func (backend *Backend) cursor(ctx context.Context, pane substrate.PaneID) (substrate.Cursor, *string, error) {
	output, err := backend.run(ctx, "display-message", "-p", "-t", string(pane), "#{cursor_x},#{cursor_y},#{cursor_flag},#{pane_dead},#{pane_dead_status}")
	if err != nil {
		return substrate.Cursor{}, nil, tmuxError("read cursor", err, output)
	}
	parts := strings.Split(strings.TrimSpace(output), ",")
	if len(parts) != 5 {
		return substrate.Cursor{}, nil, fmt.Errorf("read cursor: tmux returned %q", output)
	}
	if parts[3] == "1" {
		return substrate.Cursor{}, &parts[4], nil
	}
	column, err := nonNegative(parts[0])
	if err != nil {
		return substrate.Cursor{}, nil, fmt.Errorf("read cursor column: %w", err)
	}
	row, err := nonNegative(parts[1])
	if err != nil {
		return substrate.Cursor{}, nil, fmt.Errorf("read cursor row: %w", err)
	}
	visible, err := parseFlag(parts[2])
	if err != nil {
		return substrate.Cursor{}, nil, fmt.Errorf("read cursor visibility: %w", err)
	}
	return substrate.Cursor{Row: row, Column: column, Visible: visible}, nil, nil
}

func (backend *Backend) run(ctx context.Context, arguments ...string) (string, error) {
	return backend.runWithInput(ctx, nil, arguments...)
}

func (backend *Backend) runWithInput(ctx context.Context, input io.Reader, arguments ...string) (string, error) {
	if backend.config.Socket != "" {
		arguments = append([]string{"-S", backend.config.Socket}, arguments...)
	}
	command := exec.CommandContext(ctx, backend.config.Binary, arguments...)
	command.Stdin = input
	output, err := command.CombinedOutput()
	return string(output), err
}

func validPaneID(pane substrate.PaneID) error {
	if !strings.HasPrefix(string(pane), "%") || len(pane) == 1 || strings.ContainsAny(string(pane), "\x00\r\n") {
		return fmt.Errorf("invalid pane id %q", pane)
	}
	return nil
}

func validWindowName(name string) error {
	if name == "" {
		return fmt.Errorf("window name is required")
	}
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return fmt.Errorf("window name contains a control character")
	}
	return nil
}

func escapeFormat(value string) string {
	return strings.ReplaceAll(value, "#", "##")
}

func shellCommand(command string, arguments []string) string {
	words := append([]string{command}, arguments...)
	for index, word := range words {
		words[index] = "'" + strings.ReplaceAll(word, "'", "'\\''") + "'"
	}
	return "exec " + strings.Join(words, " ")
}

func nonNegative(value string) (int, error) {
	result, err := strconv.Atoi(value)
	if err != nil || result < 0 {
		return 0, fmt.Errorf("expected non-negative whole number, got %q", value)
	}
	return result, nil
}

func parseFlag(value string) (bool, error) {
	switch value {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("expected 0 or 1, got %q", value)
	}
}

func tmuxError(action string, err error, output string) error {
	output = strings.TrimSpace(output)
	if output == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, output)
}

func lines(output string) []string {
	output = strings.TrimSuffix(output, "\n")
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}

var _ substrate.Substrate = (*Backend)(nil)
