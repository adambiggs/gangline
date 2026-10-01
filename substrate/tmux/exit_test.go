package tmux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/adambiggs/gangline/substrate"
)

// exitingSpec runs a process that holds the write end of root's exit pipe,
// waits for release, writes to stderr, and exits with status.
func exitingSpec(t *testing.T, binary, socket, root, release, status string) substrate.SpawnSpec {
	t.Helper()
	if err := syscall.Mkfifo(filepath.Join(root, "exit-pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `exec 3>"$5"; "$1" -S "$2" wait-for "$3"; printf 'boot failure\n' >&2; exit "$4"`
	return substrate.SpawnSpec{
		Name: "native", Directory: root, Command: "sh",
		Args:       []string{"-c", script, "sh", binary, socket, release, status, filepath.Join(root, "exit-pipe")},
		KeepExited: true,
	}
}

// releaseAndAwaitExit observes the exit as EOF on the exit pipe, which does not
// depend on when tmux reaps the process. tmux can leave an exited pane child
// unreaped, and then pane-died never fires; run-shell's own child makes the
// server collect every exited child before the command returns.
func releaseAndAwaitExit(t *testing.T, binary, socket, root, release string) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		// Opening blocks until the process holds the write end.
		pipe, err := os.Open(filepath.Join(root, "exit-pipe"))
		if err != nil {
			done <- err
			return
		}
		defer pipe.Close()
		if output, err := exec.Command(binary, "-S", socket, "wait-for", "-S", release).CombinedOutput(); err != nil {
			done <- fmt.Errorf("release: %v: %s", err, output)
			return
		}
		_, err = io.Copy(io.Discard, pipe)
		done <- err
	}()
	// Bounded so a process that never exits cannot hold the test forever.
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("await process exit: %v", err)
		}
	case <-time.After(time.Minute):
		t.Fatal("process did not exit")
	}
	runTmux(t, binary, socket, "run-shell", "true")
}

func assertExited(t *testing.T, err error, status string) {
	t.Helper()
	var exited *substrate.ExitedError
	if !errors.As(err, &exited) {
		t.Fatalf("error = %v, want native exit", err)
	}
	if exited.Status != status || exited.Output != "boot failure" {
		t.Fatalf("exit = %+v, want status %s and the native output", *exited, status)
	}
}

func TestCreateSessionKeepsExitedPaneReadable(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	backend, err := New(Config{Binary: binary, Socket: socket, Session: "exits"})
	if err != nil {
		t.Fatal(err)
	}
	pane, err := backend.CreateSession(context.Background(), exitingSpec(t, binary, socket, root, "release", "3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=exits") })
	releaseAndAwaitExit(t, binary, socket, root, "release")
	_, err = backend.Capture(context.Background(), pane.ID)
	assertExited(t, err, "3")
	_, err = backend.Identity(context.Background(), pane.ID)
	assertExited(t, err, "3")
	assertExited(t, backend.ReleaseExit(context.Background(), pane.ID), "3")
}

func TestSpawnKeepsOnlyItsOwnExitedPane(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	backend, err := New(Config{Binary: binary, Socket: socket, Session: "spawns"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := backend.CreateSession(context.Background(), substrate.SpawnSpec{Name: "first", Directory: root, Command: "sh"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=spawns") })
	runTmux(t, binary, socket, "set-hook", "-g", "after-new-window", "set-option -g @existing ran")
	pane, err := backend.Spawn(context.Background(), exitingSpec(t, binary, socket, root, "release", "4"))
	if err != nil {
		t.Fatal(err)
	}
	if ran := runTmux(t, binary, socket, "show-options", "-gv", "@existing"); strings.TrimSpace(ran) != "ran" {
		t.Fatalf("existing after-new-window hook did not run: %q", ran)
	}
	if hooks := runTmux(t, binary, socket, "show-hooks", "-g"); strings.Count(hooks, "after-new-window") != 1 {
		t.Fatalf("spawn changed the global hooks: %q", hooks)
	}
	if hooks := runTmux(t, binary, socket, "show-hooks", "-t", "=spawns:"); strings.Contains(hooks, "after-new-window") {
		t.Fatalf("spawn left a session hook: %q", hooks)
	}
	if held := runTmux(t, binary, socket, "show-options", "-p", "-v", "-t", string(first.ID), "remain-on-exit"); strings.TrimSpace(held) == "on" {
		t.Fatal("spawn held the session's existing pane")
	}
	releaseAndAwaitExit(t, binary, socket, root, "release")
	_, err = backend.Capture(context.Background(), pane.ID)
	assertExited(t, err, "4")
}

func TestReleaseExitLetsLivePaneClose(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	backend, err := New(Config{Binary: binary, Socket: socket, Session: "releases"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.CreateSession(context.Background(), substrate.SpawnSpec{Name: "first", Directory: root, Command: "sh"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=releases") })
	pane, err := backend.Spawn(context.Background(), exitingSpec(t, binary, socket, root, "release", "5"))
	if err != nil {
		t.Fatal(err)
	}
	if held := runTmux(t, binary, socket, "show-options", "-p", "-v", "-t", string(pane.ID), "remain-on-exit"); strings.TrimSpace(held) != "on" {
		t.Fatalf("spawned pane remain-on-exit = %q, want on", held)
	}
	if err := backend.ReleaseExit(context.Background(), pane.ID); err != nil {
		t.Fatal(err)
	}
	releaseAndAwaitExit(t, binary, socket, root, "release")
	if panes := runTmux(t, binary, socket, "list-panes", "-s", "-t", "=releases:", "-F", "#{pane_id}"); strings.Contains(panes, string(pane.ID)+"\n") {
		t.Fatalf("released pane %s stayed after exit: %q", pane.ID, panes)
	}
}

func TestFailedSpawnClearsItsHook(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	backend, err := New(Config{Binary: binary, Socket: socket, Session: "fails"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.CreateSession(context.Background(), substrate.SpawnSpec{Name: "first", Directory: root, Command: "sh"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = runTmuxResult(binary, socket, "wait-for", "-S", "hold")
		_, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=fails")
	})
	// A hook that waits stops the spawn's list before it removes its own hook.
	runTmux(t, binary, socket, "set-hook", "-g", "after-new-window", "wait-for -S entered ; wait-for hold")
	ctx, cancel := context.WithCancel(context.Background())
	spawned := make(chan error, 1)
	go func() {
		_, err := backend.Spawn(ctx, substrate.SpawnSpec{Name: "native", Directory: root, Command: "sh", KeepExited: true})
		spawned <- err
	}()
	entered, stop := context.WithTimeout(context.Background(), time.Minute)
	defer stop()
	if output, err := exec.CommandContext(entered, binary, "-S", socket, "wait-for", "entered").CombinedOutput(); err != nil {
		t.Fatalf("await waiting hook: %v: %s", err, output)
	}
	cancel()
	if err := <-spawned; err == nil {
		t.Fatal("spawn succeeded after its context ended")
	}
	if hooks := runTmux(t, binary, socket, "show-hooks", "-g"); strings.Contains(hooks, keepExitedHook) {
		t.Fatalf("failed spawn left its hook set: %q", hooks)
	}
}

func TestExitedOutputIsBoundedInBytes(t *testing.T) {
	long := strings.Repeat("é", exitedOutputBytes)
	// The odd byte puts the cut inside a character.
	output := exited("1", "first\n"+long+"x\n").Output
	if len(output) > exitedOutputBytes || !utf8.ValidString(output) || !strings.HasSuffix(output, "éx") {
		t.Fatalf("output is %d bytes, valid UTF-8 %t", len(output), utf8.ValidString(output))
	}
}
