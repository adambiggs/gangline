package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestBackendDrivesPrivateTmuxServer(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := t.TempDir()
	socket := filepath.Join(root, "tmux.sock")
	const session = "substrate-test"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session)
	t.Cleanup(func() {
		runTmux(t, binary, socket, "kill-session", "-t", session)
	})
	if listed := strings.TrimSpace(runTmux(t, binary, socket, "list-sessions", "-F", "#{session_name}")); listed != session {
		t.Fatalf("private server sessions = %q, want %q", listed, session)
	}
	runTmux(t, binary, socket, "set-option", "-g", "base-index", "5")
	runTmux(t, binary, socket, "set-option", "-g", "pane-base-index", "8")

	backend, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ready, received, done := "backend-ready", "backend-received", "backend-done"
	script := `"$1" -S "$2" wait-for -S "$3"; IFS= read -r line; printf 'received: %s\n' "$line"; "$1" -S "$2" wait-for -S "$4"; "$1" -S "$2" wait-for "$5"`
	pane, err := backend.Spawn(context.Background(), substrate.SpawnSpec{
		Name:      "worker#S",
		Directory: root,
		Command:   "sh",
		Args:      []string{"-c", script, "sh", binary, socket, ready, received, done},
		Env:       map[string]string{"BACKEND_TEST": "one"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(pane.ID), "%") {
		t.Fatalf("pane id = %q, want tmux pane id", pane.ID)
	}
	if name := strings.TrimSpace(runTmux(t, binary, socket, "display-message", "-p", "-t", string(pane.ID), "#{window_name}")); name != "worker#S" {
		t.Fatalf("window name = %q, want literal format marker", name)
	}
	if found, err := backend.PaneNamed(context.Background(), "worker#S"); err != nil || found.ID != pane.ID {
		t.Fatalf("named pane = %#v, %v; want %q", found, err, pane.ID)
	}
	if err := backend.Rename(context.Background(), pane.ID, "renamed#S"); err != nil {
		t.Fatal(err)
	}
	if found, err := backend.PaneNamed(context.Background(), "renamed#S"); err != nil || found.ID != pane.ID {
		t.Fatalf("renamed pane = %#v, %v; want %q", found, err, pane.ID)
	}
	runTmux(t, binary, socket, "wait-for", ready)
	if err := backend.SendKeys(context.Background(), pane.ID, substrate.Keys{Text: "hello from backend", Submit: true}); err != nil {
		t.Fatal(err)
	}
	runTmux(t, binary, socket, "wait-for", received)
	screen, err := backend.Capture(context.Background(), pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(screenText(screen), "received: hello from backend") {
		t.Fatalf("captured screen = %#v", screen)
	}
	if screen.Cursor.Row < 0 || screen.Cursor.Column < 0 {
		t.Fatalf("cursor = %#v", screen.Cursor)
	}
	if err := backend.Kill(context.Background(), pane.ID); err != nil {
		t.Fatal(err)
	}
}

func TestBackendCreatesAndKillsSession(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := t.TempDir()
	backend, err := New(Config{Binary: binary, Socket: filepath.Join(root, "tmux.sock"), Session: "created"})
	if err != nil {
		t.Fatal(err)
	}
	pane, err := backend.CreateSession(context.Background(), substrate.SpawnSpec{Name: "lead", Directory: root, Command: "sh"})
	if err != nil {
		t.Fatal(err)
	}
	if exists, err := backend.SessionExists(context.Background()); err != nil || !exists {
		t.Fatalf("session exists = %t, %v", exists, err)
	}
	if err := backend.Kill(context.Background(), pane.ID); err != nil {
		t.Fatal(err)
	}
	if exists, err := backend.SessionExists(context.Background()); err != nil || exists {
		t.Fatalf("session exists after last pane kill = %t, %v", exists, err)
	}
}

func TestNewRequiresSession(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("empty session passed")
	}
}

func TestProcessTableSelectsOnlyPaneForegroundGroup(t *testing.T) {
	records, err := parseProcessTable(`
100 1 100 200 sh
200 100 200 200 codex
201 200 200 200 helper
300 200 300 200 background worker
`)
	if err != nil {
		t.Fatal(err)
	}
	root := records[100]
	var commands []string
	for pid, record := range records {
		if record.GroupID == root.foregroundGroup && descendsFrom(pid, 100, records) {
			commands = append(commands, record.Command)
		}
	}
	sort.Strings(commands)
	if strings.Join(commands, ",") != "codex,helper" {
		t.Fatalf("foreground commands = %q", commands)
	}
}

func runTmux(t *testing.T, binary, socket string, arguments ...string) string {
	t.Helper()
	command := exec.Command(binary, append([]string{"-S", socket}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
}

func screenText(screen substrate.Screen) string {
	lines := make([]string, len(screen.Rows))
	for index, row := range screen.Rows {
		cells := make([]string, len(row))
		for cellIndex, cell := range row {
			cells[cellIndex] = cell.Text
		}
		lines[index] = strings.Join(cells, "")
	}
	return fmt.Sprint(strings.Join(lines, "\n"))
}
