//go:build acceptance && linux

package tmux

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPaneCommandUsesKernelShortName(t *testing.T) {
	// The test binary, not a system tool: a multicall coreutils refuses to
	// run under a name other than its own.
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launch := filepath.Join(t.TempDir(), "custom-agent-command")
	if err := os.Symlink(executable, launch); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(launch, "--foreground-fixture")
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
	// The fixture byte establishes that the child has started.
	ready := make([]byte, 1)
	if _, err := io.ReadFull(output, ready); err != nil {
		t.Fatal(err)
	}
	got, err := readCurrentProcess(cmd.Process.Pid)
	if err != nil || got.Command != "custom-agent-co" {
		t.Fatalf("kernel command = %q, %v", got.Command, err)
	}
}
