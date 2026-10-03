package tmux

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

// A harness launched through a symlink, as a native Claude Code install is
// (claude -> versions/<version>), is recognisable from the process table under
// both the name tmux reports and the name it was invoked as.
func TestForegroundProcessesNameSymlinkedHarness(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	const session = "symlink-foreground-test"
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "helper", "background-worker"} {
		if err := os.Symlink(executable, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	ready := filepath.Join(root, "ready")
	if err := syscall.Mkfifo(ready, 0600); err != nil {
		t.Fatal(err)
	}
	runTmux(t, binary, socket, "-f", "/dev/null", "new-session", "-d", "-s", session,
		filepath.Join(root, "claude"), "--foreground-fixture", ready)
	t.Cleanup(func() { runTmux(t, binary, socket, "kill-session", "-t", session) })
	if _, err := os.ReadFile(ready); err != nil {
		t.Fatal(err)
	}
	pane := strings.TrimSpace(runTmux(t, binary, socket, "list-panes", "-t", session, "-F", "#{pane_id}"))
	backend, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	command, err := backend.ForegroundCommand(context.Background(), substrate.PaneID(pane))
	if err != nil {
		t.Fatal(err)
	}
	processes, err := backend.ForegroundProcesses(context.Background(), substrate.PaneID(pane))
	if err != nil {
		t.Fatal(err)
	}
	// tmux names the foreground group's leader: by its invoked name on Linux,
	// by its executable file on macOS. Either way the process table carries
	// that name beside the invoked one.
	for _, process := range processes {
		if process.PID == process.GroupID {
			if process.Name != command || filepath.Base(process.Command) != "claude" {
				t.Fatalf("foreground leader %+v; tmux reports %q", process, command)
			}
			return
		}
	}
	t.Fatalf("no foreground group leader in %+v", processes)
}

// A caller that cannot see tmux's processes would read unrelated processes
// under its PIDs, so the process table is refused to it.
func TestForegroundProcessesRefusesInvisibleProcesses(t *testing.T) {
	backend, pane, _ := vanishingPane(t)
	_, err := backend.foregroundProcesses(context.Background(), pane, func(int) (processRecord, error) {
		return processRecord{}, os.ErrPermission
	})
	if err == nil || !strings.Contains(err.Error(), "not visible") {
		t.Fatalf("foreground processes of an invisible pane: %v", err)
	}
}
