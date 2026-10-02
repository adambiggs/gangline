package tmux

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

// A client whose server exits while it is connected learns nothing of the
// panes. The read is made once more, and the answer it gets then stands.
func TestPaneReadIsRepeatedOnceWhenItsServerIsLost(t *testing.T) {
	root := t.TempDir()
	binary, calls := filepath.Join(root, "tmux"), filepath.Join(root, "calls")
	id := PaneIdentity{Generation: strings.Repeat("a", 64), Session: "$0", Pane: "%1"}
	b, err := New(Config{Binary: binary, Session: "test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	fixture := func(second string) {
		t.Helper()
		script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\necho \"$1\" >>'" + calls + "'\n" +
			"if [ \"$(wc -l <'" + calls + "')\" -eq 1 ]; then echo 'server exited unexpectedly' >&2; exit 1; fi\n" + second + "\n"
		if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	reads := func() string {
		t.Helper()
		data, err := os.ReadFile(calls)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(calls); err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	fixture("echo 'no server running on /tmp/absent.sock' >&2; exit 1")
	if present, err := b.CheckPane(ctx, id); present || err != nil {
		t.Fatalf("pane of a server lost mid-read = %v, %v, want absent", present, err)
	}
	if got := reads(); got != "list-panes\nlist-panes\n" {
		t.Fatalf("pane check ran %q", got)
	}
	if err := b.RemoveRegisteredPane(ctx, id); err != nil {
		t.Fatalf("removal of a pane whose server exited: %v", err)
	}
	if got := reads(); got != "list-panes\nlist-panes\n" {
		t.Fatalf("pane removal ran %q", got)
	}
	if closed, err := b.PaneClosed(ctx, id, Identity{}); closed || err != nil {
		t.Fatalf("unwitnessed server lost mid-read reads closed = %v, %v", closed, err)
	}
	if got := reads(); got != "list-panes\nlist-panes\n" {
		t.Fatalf("closed check ran %q", got)
	}
	// The repeated read reaches the server's successor, whose answer stands.
	fixture("[ \"$1\" = list-panes ] && printf '" + id.Generation + "\\t$0\\t%%1\\ttest\\n'")
	if present, err := b.CheckPane(ctx, id); !present || err != nil {
		t.Fatalf("pane found by the repeated read = %v, %v, want present", present, err)
	}
	if got := reads(); got != "list-panes\nlist-panes\n" {
		t.Fatalf("pane check ran %q", got)
	}
	// A second loss is reported: the read is repeated once only.
	fixture("echo 'server exited unexpectedly' >&2; exit 1")
	if _, err := b.CheckPane(ctx, id); err == nil || !strings.Contains(err.Error(), "server exited unexpectedly") {
		t.Fatalf("server lost on both reads: %v", err)
	}
	if got := reads(); got != "list-panes\nlist-panes\n" {
		t.Fatalf("pane check ran %q", got)
	}
}

// A pane is closed once the process of the server that registered it is gone,
// whatever answers on the socket. A server process that still runs, or one
// this caller cannot identify, leaves it unknown.
func TestPaneClosedWhenItsServerProcessExited(t *testing.T) {
	root := t.TempDir()
	id := PaneIdentity{Generation: strings.Repeat("a", 64), Session: "$0", Pane: "%1"}
	ctx := context.Background()
	backends := map[string]*Backend{}
	for name, script := range map[string]string{
		"absent server":    "echo 'no server running on /tmp/absent.sock' >&2; exit 1",
		"empty server":     "echo 'no current target' >&2; exit 1",
		"successor server": "printf '" + strings.Repeat("b", 64) + "\\t%%1\\n'",
	} {
		binary := filepath.Join(root, strings.ReplaceAll(name, " ", "-"))
		if err := os.WriteFile(binary, []byte("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n"+script+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		b, err := New(Config{Binary: binary, Session: "test"})
		if err != nil {
			t.Fatal(err)
		}
		backends[name] = b
	}
	witness := func(pid int) Identity {
		t.Helper()
		r, err := readCurrentProcess(pid)
		if err != nil {
			t.Fatal(err)
		}
		boot, err := bootIdentity()
		if err != nil {
			t.Fatal(err)
		}
		namespace, err := nativeProcessNamespace()
		if err != nil {
			t.Fatal(err)
		}
		return Identity{PID: r.PID, Started: r.started, Version: r.version, UniqueID: r.uniqueID, BootID: boot, Namespace: namespace}
	}
	child := exec.Command("cat")
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	server := witness(child.Process.Pid)
	closed := func(name string, server Identity) bool {
		t.Helper()
		got, err := backends[name].PaneClosed(ctx, id, server)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	for name := range backends {
		if closed(name, server) {
			t.Fatalf("%s: pane reads closed while its server process runs", name)
		}
	}
	// Wait reaps the child, so its process is gone when it returns.
	_ = stdin.Close()
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	for name := range backends {
		if !closed(name, server) {
			t.Fatalf("%s: pane of an exited server process does not read closed", name)
		}
		if closed(name, Identity{}) {
			t.Fatalf("%s: pane reads closed without a server witness", name)
		}
		elsewhere := server
		elsewhere.Namespace = "elsewhere"
		if closed(name, elsewhere) {
			t.Fatalf("%s: pane reads closed on a witness from another process namespace", name)
		}
		earlier := server
		earlier.BootID = "earlier"
		if closed(name, earlier) {
			t.Fatalf("%s: pane reads closed on a witness from another boot", name)
		}
		// A PID that now names another process is the server's no longer.
		reused := witness(os.Getpid())
		reused.Started, reused.UniqueID = "reused", 0
		if !closed(name, reused) {
			t.Fatalf("%s: pane of a server whose PID was reused does not read closed", name)
		}
	}
}

// A server that has ended its last session has no pane, and answers a read of
// every pane with no target. Its panes read absent, and not closed while its
// process runs.
func TestPaneOfAServerWithNoSessionIsAbsent(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	const session = "empty-test"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session)
	// The server outlives its last session until the test lets it exit.
	runTmux(t, binary, socket, "set-option", "-s", "exit-empty", "off")
	t.Cleanup(func() {
		_, _ = runTmuxResult(binary, socket, "kill-session", "-t", "="+session)
		if out, err := runTmuxResult(binary, socket, "set-option", "-s", "exit-empty", "on"); err != nil {
			t.Errorf("let the server exit: %v\n%s", err, out)
		}
	})
	b, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pane, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "registered", Directory: root, Command: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.RegisterPane(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	server, err := b.ServerIdentity(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if present, err := b.CheckPane(ctx, id); !present || err != nil {
		t.Fatalf("registered pane = %v, %v, want present", present, err)
	}
	runTmux(t, binary, socket, "kill-session", "-t", "="+session)
	if out, err := runTmuxResult(binary, socket, "list-panes", "-a"); err == nil || !strings.HasPrefix(string(out), "no current target") {
		t.Fatalf("read of a server with no session = %q, %v", out, err)
	}
	if present, err := b.CheckPane(ctx, id); present || err != nil {
		t.Fatalf("pane of a server with no session = %v, %v, want absent", present, err)
	}
	if err := b.RemoveRegisteredPane(ctx, id); err != nil {
		t.Fatalf("removal of a pane whose server has no session: %v", err)
	}
	if closed, err := b.PaneClosed(ctx, id, server); closed || err != nil {
		t.Fatalf("pane reads closed while its server runs with no session: %v, %v", closed, err)
	}
}

// The server witness names the tmux server process itself, and is refused for
// a pane that server did not register.
func TestServerIdentityNamesTheRegisteringServer(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	const session = "server-test"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session)
	t.Cleanup(func() { runTmux(t, binary, socket, "kill-session", "-t", "="+session) })
	b, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pane, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "registered", Directory: root, Command: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.RegisterPane(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	server, err := b.ServerIdentity(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if server.PID == 0 {
		t.Skip("the tmux server's process is not readable here")
	}
	if want := strings.TrimSpace(runTmux(t, binary, socket, "display-message", "-p", "#{pid}")); strconv.Itoa(server.PID) != want || !CanReadIdentity(server) {
		t.Fatalf("server witness = %+v, want pid %s in this namespace", server, want)
	}
	// The live server keeps every reading of the pane open.
	elsewhere, _ := New(Config{Binary: binary, Socket: filepath.Join(root, "absent.sock"), Session: session})
	if closed, err := elsewhere.PaneClosed(ctx, id, server); closed || err != nil {
		t.Fatalf("pane reads closed on an unreachable socket while its server runs: %v, %v", closed, err)
	}
	other := id
	other.Generation = strings.Repeat("a", 64)
	if closed, err := b.PaneClosed(ctx, other, server); closed || err != nil {
		t.Fatalf("pane of another generation reads closed while the witnessed server runs: %v, %v", closed, err)
	}
	if _, err := b.ServerIdentity(ctx, other); !errors.Is(err, ErrPaneReplaced) {
		t.Fatalf("server witnessed for a pane it did not register: %v", err)
	}
}

// The witness is the record of a process read between two answers of one
// server. A server that names another process the second time is refused.
func TestServerIdentityRefusesAServerThatChangedMidRead(t *testing.T) {
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "peer.sock")
	// This process listens on the socket, so it is the peer a dial finds.
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	binary, calls := filepath.Join(root, "tmux"), filepath.Join(root, "calls")
	id := PaneIdentity{Generation: strings.Repeat("a", 64), Session: "$0", Pane: "%1"}
	b, err := New(Config{Binary: binary, Session: "test"})
	if err != nil {
		t.Fatal(err)
	}
	fixture := func(second int) {
		t.Helper()
		script := fmt.Sprintf("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\necho \"$1\" >>'%s'\npid=%d\n[ \"$(wc -l <'%s')\" -eq 1 ] || pid=%d\nprintf '%s\\t%s\\t%%s\\n' \"$pid\"\n", calls, os.Getpid(), calls, second, id.Generation, socket)
		if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(calls); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	fixture(os.Getpid())
	server, err := b.ServerIdentity(context.Background(), id)
	if err != nil || server.PID != os.Getpid() {
		t.Fatalf("witness of a steady server = %+v, %v", server, err)
	}
	fixture(os.Getppid())
	if server, err := b.ServerIdentity(context.Background(), id); !errors.Is(err, ErrPaneReplaced) {
		t.Fatalf("witness of a server that changed mid-read = %+v, %v", server, err)
	}
}
