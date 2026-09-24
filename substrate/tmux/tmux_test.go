package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

const detachedHelperEnvironment = "GANGLINE_TMUX_DETACHED_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(detachedHelperEnvironment) == "1" {
		os.Exit(runDetachedHelper())
	}
	os.Exit(m.Run())
}

func runDetachedHelper() int {
	if _, err := syscall.Setsid(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	signal.Ignore(syscall.SIGHUP, syscall.SIGTERM)
	pidFile := os.Getenv("GANGLINE_TMUX_DETACHED_PID_FILE")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	command := exec.Command(os.Getenv("GANGLINE_TMUX_BINARY"), "-S", os.Getenv("GANGLINE_TMUX_SOCKET"), "wait-for", "-S", os.Getenv("GANGLINE_TMUX_READY"))
	if output, err := command.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "%v: %s", err, output)
		return 1
	}
	blocked := make(chan os.Signal, 1)
	signal.Notify(blocked, syscall.SIGUSR1)
	<-blocked
	return 0
}

// Keep the socket path below the Unix socket limit even when the OS uses a
// long temporary root. t.TempDir adds the full test name to that path.
func privateTmuxRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp(os.TempDir(), "tmux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	return root
}

func TestBackendDrivesPrivateTmuxServer(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
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
	root := privateTmuxRoot(t)
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

func TestBackendKillStopsDetachedDescendant(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	pidFile := filepath.Join(root, "detached.pid")
	const session = "reap-detached-test"
	backend, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ready := "detached-ready"
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := `"$1" & exec "$2" -S "$3" wait-for detached-hold`
	pane, err := backend.CreateSession(context.Background(), substrate.SpawnSpec{
		Name: "detached", Directory: root, Command: "sh",
		Args: []string{"-c", script, "sh", executable, binary, socket},
		Env: map[string]string{
			detachedHelperEnvironment:         "1",
			"GANGLINE_TMUX_DETACHED_PID_FILE": pidFile,
			"GANGLINE_TMUX_BINARY":            binary,
			"GANGLINE_TMUX_SOCKET":            socket,
			"GANGLINE_TMUX_READY":             ready,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if output, err := runTmuxResult(binary, socket, "wait-for", ready); err != nil {
		t.Fatalf("wait for detached descendant: %v\n%s", err, output)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		t.Fatalf("detached pid = %q, %v", data, err)
	}
	observation, err := observeProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	defer observation.close()
	identity, err := pinObservedProcess(observation.record, observation.read, openProcessHandle)
	if err != nil {
		t.Fatal(err)
	}
	defer identity.handle.close()
	t.Cleanup(func() { _ = identity.handle.signal(syscall.SIGKILL) })

	if err := backend.Kill(context.Background(), pane.ID); err != nil {
		t.Fatal(err)
	}
	if err := identity.handle.wait(context.Background()); err != nil {
		t.Fatalf("detached descendant %d did not exit: %v", pid, err)
	}
}

func TestNewRequiresSession(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("empty session passed")
	}
}

func TestProcessTableSelectsOnlyPaneForegroundGroup(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	const session = "foreground-test"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session)
	t.Cleanup(func() { runTmux(t, binary, socket, "kill-session", "-t", session) })
	if listed := strings.TrimSpace(runTmux(t, binary, socket, "list-sessions", "-F", "#{session_name}")); listed != session {
		t.Fatalf("private server sessions = %q, want %q", listed, session)
	}
	pane := strings.TrimSpace(runTmux(t, binary, socket, "list-panes", "-t", session, "-F", "#{pane_id}"))
	pid, err := strconv.Atoi(strings.TrimSpace(runTmux(t, binary, socket, "display-message", "-p", "-t", pane, "#{pane_pid}")))
	if err != nil {
		t.Fatal(err)
	}
	ps := filepath.Join(root, "ps")
	observations := fmt.Sprintf(`#!/bin/sh
cat <<'EOF'
%d 1 100 200 Sun Sep 21 08:00:00 2026 sh
200 %d 200 200 Sun Sep 21 08:00:01 2026 harness
201 200 200 200 Sun Sep 21 08:00:02 2026 helper
300 200 300 200 Sun Sep 21 08:00:03 2026 background worker
EOF
`, pid, pid)
	if err := os.WriteFile(ps, []byte(observations), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+":"+os.Getenv("PATH"))
	backend, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	processes, err := backend.ForegroundProcesses(context.Background(), substrate.PaneID(pane))
	if err != nil {
		t.Fatal(err)
	}
	var commands []string
	for _, process := range processes {
		commands = append(commands, process.Command)
	}
	sort.Strings(commands)
	if strings.Join(commands, ",") != "harness,helper" {
		t.Fatalf("foreground commands = %q", commands)
	}
}

func runTmux(t *testing.T, binary, socket string, arguments ...string) string {
	t.Helper()
	output, err := runTmuxResult(binary, socket, arguments...)
	if err != nil {
		t.Fatalf("tmux %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
}

func runTmuxResult(binary, socket string, arguments ...string) ([]byte, error) {
	command := exec.Command(binary, append([]string{"-S", socket}, arguments...)...)
	return command.CombinedOutput()
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
