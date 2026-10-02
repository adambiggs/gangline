//go:build linux

package tmux

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The witness is the process at the other end of the server's socket. A
// server that names a process other than that peer is not witnessed.
func TestServerIdentityRefusesAProcessThatIsNotTheSocketPeer(t *testing.T) {
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "peer.sock")
	// This process listens on the socket, so it is the peer a dial finds.
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	binary := filepath.Join(root, "tmux")
	id := PaneIdentity{Generation: strings.Repeat("a", 64), Session: "$0", Pane: "%1"}
	b, err := New(Config{Binary: binary, Session: "test"})
	if err != nil {
		t.Fatal(err)
	}
	fixture := func(pid int) {
		t.Helper()
		script := fmt.Sprintf("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\nprintf '%s\\t%s\\t%d\\n'\n", id.Generation, socket, pid)
		if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	fixture(os.Getpid())
	server, err := b.ServerIdentity(context.Background(), id)
	if err != nil || server.PID != os.Getpid() {
		t.Fatalf("witness of a server that is the socket's peer = %+v, %v", server, err)
	}
	// The parent is a live, readable process in this namespace: only the
	// socket says it is not the server.
	fixture(os.Getppid())
	server, err = b.ServerIdentity(context.Background(), id)
	if err != nil || server != (Identity{}) {
		t.Fatalf("witness of a server whose socket peer is another process = %+v, %v", server, err)
	}
}
