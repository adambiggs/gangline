package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

const symlinkedHarnessEnvironment = "GANGLINE_SYMLINKED_HARNESS_FIFO"

// A native Claude Code install launches through a symlink named after the
// harness that points at a file named after its version. tmux on macOS names
// the pane's process after that file, and input must still reach the harness.
func TestInputReachesHarnessLaunchedThroughSymlink(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root, err := os.MkdirTemp("", "gang-symlinked-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launch := filepath.Join(root, "claude")
	if err := os.Symlink(executable, launch); err != nil {
		t.Fatal(err)
	}
	ready, received := filepath.Join(root, "ready"), filepath.Join(root, "received")
	for _, fifo := range []string{ready, received} {
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	socket, session := root+"/tmux.sock", "symlinked"
	if out, err := exec.Command(binary, "-S", socket, "-f", "/dev/null", "new-session", "-d", "-s", session, "cat").CombinedOutput(); err != nil {
		t.Fatalf("new-session: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command(binary, "-S", socket, "kill-session", "-t", "="+session).Run() })
	b, err := tmux.New(tmux.Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pane, err := b.Spawn(ctx, substrate.SpawnSpec{
		Name: "harness", Directory: root, Command: launch,
		Args: []string{"-test.run=^TestSymlinkedHarnessHelper$"},
		Env:  map[string]string{symlinkedHarnessEnvironment: root},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The helper opens the FIFO only once it runs as the pane's harness.
	if line, err := os.ReadFile(ready); err != nil || string(line) != "ready\n" {
		t.Fatalf("helper readiness = %q, %v", line, err)
	}

	claude := harness.Collar{Launch: harness.Launch{Command: "claude"}}
	if err := requireHarnessForeground(ctx, b, pane.ID, claude); err != nil {
		t.Fatalf("symlinked harness refused: %v", err)
	}
	other := harness.Collar{Launch: harness.Launch{Command: "codex"}}
	if err := requireHarnessForeground(ctx, b, pane.ID, other); err == nil || !strings.Contains(err.Error(), "refuse input") {
		t.Fatalf("another harness's input was not refused: %v", err)
	}

	registration, err := b.RegisterPane(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	input := paneInput{harnessInput: b, registry: b, identity: registration, launch: "claude"}
	if err := sendHarnessKeys(ctx, input, pane.ID, claude, substrate.Keys{Text: "typed", Submit: true}); err != nil {
		t.Fatalf("registered input to symlinked harness: %v", err)
	}
	if line, err := os.ReadFile(received); err != nil || string(line) != "typed\n" {
		t.Fatalf("harness received %q, %v", line, err)
	}
}

// TestSymlinkedHarnessHelper is the harness that
// TestInputReachesHarnessLaunchedThroughSymlink runs in its pane.
func TestSymlinkedHarnessHelper(t *testing.T) {
	root := os.Getenv(symlinkedHarnessEnvironment)
	if root == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(root, "ready"), []byte("ready\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "received"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
}
