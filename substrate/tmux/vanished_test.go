package tmux

import (
	"context"
	"errors"
	"os"
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
func childProcess(t *testing.T) (processRecord, func()) {
	t.Helper()
	child := exec.Command("cat")
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	reaped := false
	reap := func() {
		reaped = true
		if err := stdin.Close(); err != nil {
			t.Fatal(err)
		}
		if err := child.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if !reaped {
			_ = stdin.Close()
			_ = child.Wait()
		}
	})
	r, err := readCurrentProcess(child.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	return r, reap
}

func TestAcquireRecordedSkipsProcessThatVanishesBeforePinning(t *testing.T) {
	r, reap := childProcess(t)
	boot, err := bootIdentity()
	if err != nil {
		t.Fatal(err)
	}
	namespace, err := nativeProcessNamespace()
	if err != nil {
		t.Fatal(err)
	}
	id := Identity{PID: r.PID, Started: r.started, Version: r.version, UniqueID: r.uniqueID, BootID: boot, Namespace: namespace}
	// The observation is taken while the process lives; it is then reaped,
	// so the reads that pin it find it gone.
	owned, err := acquireRecorded([]Identity{id}, boot, func(pid int) (processObservation, error) {
		observation, err := observeProcess(pid)
		if err == nil {
			reap()
		}
		return observation, err
	})
	if err != nil || len(owned.Identities()) != 0 {
		t.Fatalf("vanished process stopped recorded teardown: owned=%v err=%v", owned, err)
	}
	_, err = acquireRecorded([]Identity{id}, boot, func(pid int) (processObservation, error) {
		return processObservation{record: r, read: func() (processRecord, error) { return processRecord{}, os.ErrPermission }, close: func() error { return nil }}, nil
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("pin error was hidden: %v", err)
	}
}

func TestOpenProcessHandleReportsReapedProcessAsGone(t *testing.T) {
	r, reap := childProcess(t)
	reap()
	handle, err := openProcessHandle(r)
	if err == nil {
		_ = handle.close()
	}
	if !processGone(err) {
		t.Fatalf("reaped process handle: err=%v, want a gone process", err)
	}
}
