package tmux

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

// vanishingPane starts a held pane whose process exits on release. The
// returned function exits that process and waits until tmux has reaped it, so
// a process read after it finds the process gone.
func vanishingPane(t *testing.T) (*Backend, substrate.PaneID, func()) {
	t.Helper()
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	backend, err := New(Config{Binary: binary, Socket: socket, Session: "vanishes"})
	if err != nil {
		t.Fatal(err)
	}
	pane, err := backend.CreateSession(context.Background(), exitingSpec(t, binary, socket, root, "release", "3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=vanishes") })
	pipe := awaitStart(t, root)
	return backend, pane.ID, func() { awaitExit(t, binary, socket, pipe, "release") }
}

func TestIdentityReportsVanishedProcessAsExit(t *testing.T) {
	backend, pane, exit := vanishingPane(t)
	_, err := backend.identity(context.Background(), pane, func(pid int) (processObservation, error) {
		exit()
		return observeProcess(pid)
	})
	assertExited(t, err, "3")
}

func TestProcessVisibilityReportsVanishedProcessAsExit(t *testing.T) {
	backend, pane, exit := vanishingPane(t)
	_, err := backend.processVisibility(context.Background(), pane, func(pid int) (processRecord, error) {
		exit()
		return readCurrentProcess(pid)
	})
	assertExited(t, err, "3")
}

// childProcess starts a process this test owns. The returned function ends
// and reaps it, so a process read after it finds the process gone.
