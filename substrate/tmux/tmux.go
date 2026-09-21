// Package tmux drives panes through tmux.
package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"unicode"

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
	config Config
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
	return backend.launch(ctx, "create session", arguments)
}

func (backend *Backend) SessionExists(ctx context.Context) (bool, error) {
	output, err := backend.run(ctx, "has-session", "-t", backend.config.Session)
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
	arguments = append(arguments[:len(arguments)-1], "-t", backend.config.Session, arguments[len(arguments)-1])
	return backend.launch(ctx, "spawn pane", arguments)
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
	output, err := backend.run(ctx, arguments...)
	if err != nil {
		return substrate.Pane{}, tmuxError(action, err, output)
	}
	identifier := strings.TrimSpace(output)
	if identifier == "" || strings.ContainsAny(identifier, "\r\n") {
		return substrate.Pane{}, fmt.Errorf("%s: tmux returned invalid pane id %q", action, output)
	}
	return substrate.Pane{ID: substrate.PaneID(identifier)}, nil
}

func (backend *Backend) Windows(ctx context.Context) ([]Window, error) {
	output, err := backend.run(ctx, "list-windows", "-t", backend.config.Session, "-F", "#{window_id}")
	if err != nil {
		return nil, tmuxError("list windows", err, output)
	}
	identifiers := lines(output)
	windows := make([]Window, 0, len(identifiers))
	for _, identifier := range identifiers {
		panes, err := backend.run(ctx, "list-panes", "-t", identifier, "-F", "#{pane_id}")
		if err != nil {
			return nil, tmuxError("list panes", err, panes)
		}
		for _, paneID := range lines(panes) {
			pane := substrate.PaneID(paneID)
			if err := validPaneID(pane); err != nil {
				return nil, fmt.Errorf("list panes: %w", err)
			}
			name, err := backend.windowName(ctx, pane)
			if err != nil {
				return nil, err
			}
			windows = append(windows, Window{Pane: substrate.Pane{ID: pane}, Name: name})
		}
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
	cursor, err := backend.cursor(ctx, pane)
	if err != nil {
		return substrate.Screen{}, err
	}
	screen, err := parseScreen(raw, cursor)
	if err != nil {
		return substrate.Screen{}, fmt.Errorf("parse captured pane %q: %w", pane, err)
	}
	return screen, nil
}

func (backend *Backend) Kill(ctx context.Context, pane substrate.PaneID) error {
	if err := validPaneID(pane); err != nil {
		return err
	}
	output, err := backend.run(ctx, "kill-window", "-t", string(pane))
	if err != nil {
		return tmuxError("kill pane", err, output)
	}
	return nil
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

func (backend *Backend) cursor(ctx context.Context, pane substrate.PaneID) (substrate.Cursor, error) {
	output, err := backend.run(ctx, "display-message", "-p", "-t", string(pane), "#{cursor_x},#{cursor_y},#{cursor_flag}")
	if err != nil {
		return substrate.Cursor{}, tmuxError("read cursor", err, output)
	}
	parts := strings.Split(strings.TrimSpace(output), ",")
	if len(parts) != 3 {
		return substrate.Cursor{}, fmt.Errorf("read cursor: tmux returned %q", output)
	}
	column, err := nonNegative(parts[0])
	if err != nil {
		return substrate.Cursor{}, fmt.Errorf("read cursor column: %w", err)
	}
	row, err := nonNegative(parts[1])
	if err != nil {
		return substrate.Cursor{}, fmt.Errorf("read cursor row: %w", err)
	}
	visible, err := parseFlag(parts[2])
	if err != nil {
		return substrate.Cursor{}, fmt.Errorf("read cursor visibility: %w", err)
	}
	return substrate.Cursor{Row: row, Column: column, Visible: visible}, nil
}

func (backend *Backend) run(ctx context.Context, arguments ...string) (string, error) {
	if backend.config.Socket != "" {
		arguments = append([]string{"-S", backend.config.Socket}, arguments...)
	}
	command := exec.CommandContext(ctx, backend.config.Binary, arguments...)
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

func (backend *Backend) windowName(ctx context.Context, pane substrate.PaneID) (string, error) {
	output, err := backend.run(ctx, "display-message", "-p", "-t", string(pane), "#{window_name}")
	if err != nil {
		return "", tmuxError("read window name", err, output)
	}
	return strings.TrimSuffix(output, "\n"), nil
}

func lines(output string) []string {
	output = strings.TrimSuffix(output, "\n")
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}

var _ substrate.Substrate = (*Backend)(nil)
