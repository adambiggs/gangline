//go:build linux

package tmux

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestPaneCommandRetainsLongInvokedName(t *testing.T) {
	cat, err := exec.LookPath("cat")
	if err != nil {
		t.Fatal(err)
	}
	launch := filepath.Join(t.TempDir(), "custom-agent-command")
	if err := os.Symlink(cat, launch); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(launch)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		input.Close()
		if err := cmd.Wait(); err != nil {
			t.Error(err)
		}
	})
	// Exec has completed at Start, but the kernel can still expose an empty
	// cmdline before the child first runs. An echo proves native readiness.
	if _, err := input.Write([]byte("ready\n")); err != nil {
		t.Fatal(err)
	}
	ready := make([]byte, len("ready\n"))
	if _, err := io.ReadFull(output, ready); err != nil {
		t.Fatal(err)
	}
	got := foregroundCommand(substrate.Process{PID: cmd.Process.Pid, Command: "custom-agent-co"})
	if got != launch {
		t.Fatalf("invoked command = %q, want %q", got, launch)
	}
}
