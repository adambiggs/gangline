package tmux

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// reapTmux returns once the server has reaped every child that had exited.
func reapTmux(t *testing.T, binary, socket string) {
	t.Helper()
	// Bounded so a server that stops answering cannot hold the test forever.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if output, err := Reaped(ctx, func(ctx context.Context, arguments ...string) (string, error) {
		output, err := exec.CommandContext(ctx, binary, append([]string{"-S", socket}, arguments...)...).CombinedOutput()
		return string(output), err
	}); err != nil {
		t.Fatalf("reap exited children: %v\n%s", err, output)
	}
}

// A server that missed the SIGCHLD its run-shell child sent leaves the child
// unreaped until another child exits. The wrapper holds the reap's run-shell
// until a background run-shell starts, the way that server holds it until a
// later child exits, and then lets both reach the server.
func TestReleaseExitReapsAfterAMissedChildSignal(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	kicks := filepath.Join(root, "kicks")
	if err := syscall.Mkfifo(kicks, 0o600); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(root, "tmux")
	// Opening the FIFO read-write never blocks, so a kick with no reap
	// waiting is dropped instead of holding its client.
	script := "#!/bin/sh\ncase \"$3 $4\" in\n" +
		"'run-shell true') read -r _ <'" + kicks + "' || exit 97;;\n" +
		"'run-shell -b') printf 'kick\\n' 1<>'" + kicks + "';;\n" +
		"esac\nexec '" + binary + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	backend, err := New(Config{Binary: wrapper, Socket: socket, Session: "missed"})
	if err != nil {
		t.Fatal(err)
	}
	pane, err := backend.CreateSession(context.Background(), exitingSpec(t, binary, socket, root, "release", "3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=missed") })
	releaseAndAwaitExit(t, binary, socket, root, "release")
	// Bounded so a reap that waits for a child signal that never comes fails
	// the test instead of holding it.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	assertExited(t, backend.ReleaseExit(ctx, pane.ID), "3")
}
