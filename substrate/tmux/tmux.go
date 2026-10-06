// Package tmux drives panes through tmux.
package tmux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	Stdout  io.Writer
	Stderr  io.Writer
}

// Backend is a tmux implementation of substrate.Substrate.
type Backend struct {
	config  Config
	birthMu sync.Mutex
	birth   map[substrate.PaneID]PaneIdentity
}

type PaneInfo struct {
	Pane  substrate.Pane
	Title string
	// Registration is the pane as RegisterPane names it. A pane id is unique
	// only within one server lifetime, so a record that names this pane id
	// holds this pane only when its registration is equal.
	Registration PaneIdentity
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
	arguments, command, err := backend.launchArguments("new-session", spec)
	if err != nil {
		return substrate.Pane{}, err
	}
	arguments = append(append(arguments, "-s", backend.config.Session), command...)
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
	arguments, command, err := backend.launchArguments("new-window", spec)
	if err != nil {
		return substrate.Pane{}, err
	}
	arguments = append(append(arguments, "-t", "="+backend.config.Session+":"), command...)
	return backend.launch(ctx, "spawn pane", arguments)
}

// Split creates a sibling only while the target still has its registered
// identity. The condition and allocation run in one tmux command list.
func (backend *Backend) Split(ctx context.Context, target PaneIdentity, spec substrate.SpawnSpec, vertical bool) (substrate.Pane, error) {
	panes, err := backend.Panes(ctx)
	if err != nil {
		return substrate.Pane{}, err
	}
	member := false
	for _, pane := range panes {
		member = member || pane.Registration == target
	}
	if !member {
		return substrate.Pane{}, fmt.Errorf("split target is not in the selected team")
	}
	present, err := backend.CheckPane(ctx, target)
	if err != nil {
		return substrate.Pane{}, err
	}
	if !present {
		return substrate.Pane{}, fmt.Errorf("split target is absent")
	}
	arguments, command, err := backend.launchArguments("split-window", spec)
	if err != nil {
		return substrate.Pane{}, err
	}
	orientation := "-h"
	if vertical {
		orientation = "-v"
	}
	arguments = append(append(arguments, orientation, "-t", target.Pane), command...)
	return backend.launch(ctx, "split pane", []string{"if-shell", "-F", "-t", target.Pane, registeredCondition(target), tmuxCommand(arguments[0], arguments[1:]...), tmuxCommand("display-message", "-p", "split target was replaced")})
}

// ReleaseExit restores the default close-on-exit behaviour for a pane spawned
// with KeepExited. A pane whose process already exited is left open and
// reported as an ExitedError carrying its final output.
func (backend *Backend) ReleaseExit(ctx context.Context, pane substrate.PaneID) error {
	if err := validPaneID(pane); err != nil {
		return err
	}
	// One command list: the process cannot exit between the check and the
	// release. It reaps first, so an exit already made reports its status.
	output, err := backend.reaped(ctx,
		"display-message", "-p", "-t", string(pane), "#{pane_dead},#{pane_dead_status}", ";",
		"capture-pane", "-p", "-J", "-S", "-", "-t", string(pane), ";",
		"set-option", "-p", "-u", "-t", string(pane), "remain-on-exit")
	if err != nil {
		return tmuxError("release exited pane", err, output)
	}
	return releasedExit(output)
}

// releasedExit reads the pane state line and history that a release printed.
func releasedExit(output string) error {
	state, history, _ := strings.Cut(output, "\n")
	dead, status, ok := strings.Cut(state, ",")
	if !ok || dead != "0" && dead != "1" {
		return fmt.Errorf("release exited pane: tmux returned %q", state)
	}
	if dead == "0" {
		return nil
	}
	return exited(status, history)
}

// Some tmux versions mark a pane dead when its terminal closes, which can come
// before the server reaps the process and collects its status, so the status
// is read once the server has reaped every exited child.
func (backend *Backend) exitedPane(ctx context.Context, pane substrate.PaneID, status string) error {
	if status == "" {
		output, err := backend.reaped(ctx, "display-message", "-p", "-t", string(pane), "#{pane_dead_status}")
		if err != nil {
			return tmuxError("read exit status", err, output)
		}
		status = strings.TrimSpace(output)
	}
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

// launchArguments returns the arguments that create a pane for spec and,
// separately, the command the pane runs, which ends the tmux command.
func (backend *Backend) launchArguments(command string, spec substrate.SpawnSpec) ([]string, []string, error) {
	if err := validPaneTitle(spec.Name); err != nil {
		return nil, nil, err
	}
	if spec.Directory == "" {
		return nil, nil, fmt.Errorf("spawn directory is required")
	}
	if spec.Command == "" {
		return nil, nil, fmt.Errorf("spawn command is required")
	}
	arguments := []string{
		command, "-d", "-P", "-F", "#{pane_id}",
		"-c", spec.Directory,
	}
	if command != "split-window" {
		arguments = append(arguments, "-n", escapeFormat(spec.Name))
	}
	names := make([]string, 0, len(spec.Env))
	for name := range spec.Env {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := spec.Env[name]
		if name == "" || strings.ContainsAny(name, "=\x00") || strings.ContainsRune(value, '\x00') {
			return nil, nil, fmt.Errorf("invalid spawn environment %q", name)
		}
		arguments = append(arguments, "-e", name+"="+value)
	}
	native := shellCommand(spec.Command, spec.Args)
	if spec.KeepExited {
		// The pane holds itself before the native command starts. A tmux
		// hook or a command after this one resolves its pane when it runs,
		// so a user hook that splits the window or opens another could
		// move the hold to its own pane.
		// The pane starts in spec.Directory, not gang's directory.
		binary, err := exec.LookPath(backend.config.Binary)
		if err == nil {
			binary, err = physicalPath(binary)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("find %s: %w", backend.config.Binary, err)
		}
		hold := shellWords([]string{binary})
		if backend.config.Socket != "" {
			socket, err := physicalPath(backend.config.Socket)
			if err != nil {
				return nil, nil, err
			}
			// A unix socket address is short, so the hold connects from
			// the socket's directory by its name, in a subshell that
			// leaves the native command's directory alone.
			hold = "cd -- " + shellWords([]string{filepath.Dir(socket)}) + " && " + hold + " " + shellWords([]string{"-S", filepath.Base(socket)})
		}
		// tmux runs a command of several words without the user's
		// default-shell, which need not parse the hold. The native command
		// then runs through that shell, which tmux names in SHELL, as tmux
		// runs a command of one word.
		// A pane that fails to hold itself closes with whatever the hold
		// printed, so that goes to the log with the hold's exit status. Only
		// a failed hold writes the log, and a log the pane cannot write
		// changes nothing else. The pane resolves the path from its own
		// directory, so the path is absolute.
		log := os.DevNull
		if spec.HoldLog != "" {
			if log, err = filepath.Abs(spec.HoldLog); err != nil {
				return nil, nil, err
			}
		}
		return arguments, []string{"/bin/sh", "-c", "out=$( (" + hold + ` set-option -p -t "$TMUX_PANE" remain-on-exit on) 2>&1 ) || { status=$?; printf '%s\nexit status %s\n' "$out" "$status" >"$2"; exit "$status"; }; exec "$SHELL" -c "$1"`, "sh", native, log}, nil
	}
	return arguments, []string{native}, nil
}

// physicalPath makes path absolute the way the kernel resolves it, where a
// ".." after a symbolic link leaves the link's target rather than the
// directory that holds the link.
func physicalPath(path string) (string, error) {
	dir, name := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dir) {
		wd, err := os.Getwd()
		if err == nil {
			wd, err = filepath.EvalSymlinks(wd)
		}
		if err != nil {
			return "", err
		}
		dir = filepath.Join(wd, dir)
	}
	return filepath.Join(dir, name), nil
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
	var output string
	if arguments[0] == "if-shell" {
		// Quoting a native launch inside the registered condition can exceed
		// tmux's IPC message limit. Stream the command list without that limit.
		var commands []string
		start := 0
		for i, argument := range arguments {
			if argument == ";" {
				commands = append(commands, tmuxCommand(arguments[start], arguments[start+1:i]...))
				start = i + 1
			}
		}
		commands = append(commands, tmuxCommand(arguments[start], arguments[start+1:]...))
		output, err = backend.runWithInput(ctx, strings.NewReader(strings.Join(commands, " ; ")+"\n"), "source-file", "-")
	} else {
		output, err = backend.run(ctx, arguments...)
	}
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

func (backend *Backend) Panes(ctx context.Context) ([]PaneInfo, error) {
	output, err := backend.run(ctx, "list-panes", "-s", "-t", "="+backend.config.Session+":", "-F", "#{pane_id}\t#{"+generationOption+"}\t#{session_id}\t#{@gangline_title}")
	if err != nil {
		return nil, tmuxError("list panes", err, output)
	}
	var panes []PaneInfo
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			return nil, fmt.Errorf("list panes: malformed record %q", line)
		}
		pane := substrate.PaneID(fields[0])
		if err := validPaneID(pane); err != nil {
			return nil, err
		}
		panes = append(panes, PaneInfo{
			Pane:         substrate.Pane{ID: pane},
			Title:        fields[3],
			Registration: PaneIdentity{Generation: fields[1], Session: fields[2], Pane: fields[0]},
		})
	}
	return panes, nil
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

// DescribePane locates a pane for an operator who has no registered agent
// name for it. Pane indexes are positions within a window, not pane handles.
func (backend *Backend) DescribePane(ctx context.Context, pane substrate.PaneID) (string, error) {
	if err := validPaneID(pane); err != nil {
		return "", err
	}
	output, err := backend.run(ctx, "display-message", "-p", "-t", string(pane), "#{window_name}\t#{pane_index}")
	if err != nil {
		return "", tmuxError("locate pane", err, output)
	}
	fields := strings.Split(strings.TrimSuffix(output, "\n"), "\t")
	if len(fields) != 2 {
		return "", fmt.Errorf("locate pane: malformed position")
	}
	position, err := nonNegative(fields[1])
	if err != nil {
		return "", fmt.Errorf("locate pane: invalid position")
	}
	return fmt.Sprintf("window %q at pane position %d", fields[0], position), nil
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
	output, err := backend.run(ctx, "kill-pane", "-t", string(pane))
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
	arguments := []string{"attach-session", "-t", backend.config.Session, ";", "select-window", "-t", string(pane), ";", "select-pane", "-t", string(pane)}
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
	// A client outside tmux whose locale does not name UTF-8 prints tabs and
	// non-ASCII bytes as underscores, and gang parses what it prints. -u
	// keeps the output intact whatever locale gang's caller has.
	arguments = append([]string{"-u"}, arguments...)
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

func validPaneTitle(name string) error {
	if name == "" {
		return fmt.Errorf("pane title is required")
	}
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return fmt.Errorf("pane title contains a control character")
	}
	return nil
}

func escapeFormat(value string) string {
	return strings.ReplaceAll(value, "#", "##")
}

func shellCommand(command string, arguments []string) string {
	return "exec " + shellWords(append([]string{command}, arguments...))
}

func shellWords(words []string) string {
	quoted := make([]string, len(words))
	for index, word := range words {
		quoted[index] = "'" + strings.ReplaceAll(word, "'", "'\\''") + "'"
	}
	return strings.Join(quoted, " ")
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
