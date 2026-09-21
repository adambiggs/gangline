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

func New(config Config) (*Backend, error) {
	if config.Session == "" {
		return nil, fmt.Errorf("tmux session is required")
	}
	if config.Binary == "" {
		config.Binary = "tmux"
	}
	return &Backend{config: config}, nil
}

func (backend *Backend) Spawn(ctx context.Context, spec substrate.SpawnSpec) (substrate.Pane, error) {
	if err := validWindowName(spec.Name); err != nil {
		return substrate.Pane{}, err
	}
	if spec.Directory == "" {
		return substrate.Pane{}, fmt.Errorf("spawn directory is required")
	}
	if spec.Command == "" {
		return substrate.Pane{}, fmt.Errorf("spawn command is required")
	}

	arguments := []string{
		"new-window", "-d", "-P", "-F", "#{pane_id}",
		"-t", backend.config.Session,
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
			return substrate.Pane{}, fmt.Errorf("invalid spawn environment %q", name)
		}
		arguments = append(arguments, "-e", name+"="+value)
	}
	arguments = append(arguments, shellCommand(spec.Command, spec.Args))

	output, err := backend.run(ctx, arguments...)
	if err != nil {
		return substrate.Pane{}, tmuxError("spawn pane", err, output)
	}
	identifier := strings.TrimSpace(output)
	if identifier == "" || strings.ContainsAny(identifier, "\r\n") {
		return substrate.Pane{}, fmt.Errorf("spawn pane: tmux returned invalid pane id %q", output)
	}
	return substrate.Pane{ID: substrate.PaneID(identifier)}, nil
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

func (backend *Backend) ComposerState(ctx context.Context, pane substrate.PaneID) (substrate.ComposerState, error) {
	if err := validPaneID(pane); err != nil {
		return substrate.ComposerState{}, err
	}
	output, err := backend.run(ctx, "display-message", "-p", "-t", string(pane), "#{pane_in_mode}")
	if err != nil {
		return substrate.ComposerState{}, tmuxError("read pane mode", err, output)
	}
	inMode, err := parseFlag(strings.TrimSpace(output))
	if err != nil {
		return substrate.ComposerState{}, fmt.Errorf("read pane mode: %w", err)
	}
	return substrate.ComposerState{Ready: !inMode}, nil
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

var _ substrate.Substrate = (*Backend)(nil)
