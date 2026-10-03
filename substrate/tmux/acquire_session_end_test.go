package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

// A session that ends while the pane's process tree is read leaves the
// recorded root to acquire, as when the pane was already gone.
func TestAcquireTreeTakesRecordedRootWhenSessionEnds(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	for _, mode := range serverModes() {
		for _, read := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s/read-%d", mode.name, read), func(t *testing.T) {
				socket, session := sessionEndCase(t, binary, mode.keep)
				real, err := New(Config{Binary: binary, Socket: socket, Session: session})
				if err != nil {
					t.Fatal(err)
				}
				ctx := context.Background()
				root := t.TempDir()
				if err := syscall.Mkfifo(filepath.Join(root, "exit-pipe"), 0o600); err != nil {
					t.Fatal(err)
				}
				// The process opens its pipe only once it ignores the hangup
				// that ending the session sends.
				pane, err := real.Spawn(ctx, substrate.SpawnSpec{Name: "registered", Directory: root, Command: "sh", Args: []string{"-c", `trap '' HUP; exec 3>"$1"; exec sleep 600`, "sh", filepath.Join(root, "exit-pipe")}})
				if err != nil {
					t.Fatal(err)
				}
				awaitStart(t, binary, socket, root, pane.ID)
				if _, err := real.RegisterPane(ctx, pane.ID); err != nil {
					t.Fatal(err)
				}
				expected, err := real.Identity(ctx, pane.ID)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = syscall.Kill(expected.PID, syscall.SIGKILL) })
				wrapper, arm, ended := sessionEndingTmux(t, binary, socket, session, `"#{pane_pid} "`, read, false)
				b, err := New(Config{Binary: wrapper, Socket: socket, Session: session})
				if err != nil {
					t.Fatal(err)
				}
				arm()
				owned, err := b.AcquireTree(ctx, pane.ID, expected)
				if !ended() {
					t.Fatalf("the session did not end before pane process read %d", read)
				}
				if err != nil {
					t.Fatalf("AcquireTree: %v", err)
				}
				defer owned.Close()
				identities := owned.Identities()
				if len(identities) != 1 || identities[0].PID != expected.PID {
					t.Fatalf("acquired %+v, want the recorded root %d", identities, expected.PID)
				}
				if err := owned.Stop(ctx); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// A process read that fails while the pane is still registered, or whose
// registration cannot then be read, acquires nothing and returns the failure.
func TestAcquireTreeKeepsProcessReadFailureForLivePane(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	for _, tc := range []struct {
		name        string
		failRecheck bool
	}{{"pane-registered", false}, {"recheck-fails", true}} {
		t.Run(tc.name, func(t *testing.T) {
			socket, session := sessionEndCase(t, binary, false)
			real, err := New(Config{Binary: binary, Socket: socket, Session: session})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			pane, err := real.Spawn(ctx, substrate.SpawnSpec{Name: "registered", Directory: t.TempDir(), Command: "cat"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := real.RegisterPane(ctx, pane.ID); err != nil {
				t.Fatal(err)
			}
			expected, err := real.Identity(ctx, pane.ID)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			wrapper := filepath.Join(dir, "tmux")
			recheck := ""
			if tc.failRecheck {
				recheck = `case "$*" in *" list-panes "*) [ -e "$dir/failed" ] && { echo 'recheck refused' >&2; exit 1; };; esac`
			}
			script := fmt.Sprintf(`#!/bin/sh
dir='%s'
%s
case "$*" in *"#{pane_pid} "*) : >"$dir/failed"; echo 'process read refused' >&2; exit 1;; esac
exec '%s' "$@"
`, dir, recheck, binary)
			if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			b, err := New(Config{Binary: wrapper, Socket: socket, Session: session})
			if err != nil {
				t.Fatal(err)
			}
			owned, err := b.AcquireTree(ctx, pane.ID, expected)
			if err == nil || !strings.Contains(err.Error(), "process read refused") {
				t.Fatalf("AcquireTree = %+v, %v; want the process read failure", owned, err)
			}
			if owned != nil {
				t.Fatalf("acquired %+v despite the failure", owned.Identities())
			}
		})
	}
}
