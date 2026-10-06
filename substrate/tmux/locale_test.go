package tmux

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

// A tmux client outside tmux whose locale does not name UTF-8 rewrites tabs
// and non-ASCII bytes in what it prints as underscores. A scheduler runs gang
// with no locale at all, so every record gang parses must still arrive intact.
func TestRecordsSurviveCallerWithoutUTF8Locale(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	// tmux reads an empty TMUX as being inside tmux, so the caller's TMUX is
	// removed, not emptied.
	for _, name := range []string{"TMUX", "TMUX_PANE", "LC_ALL", "LC_CTYPE", "LANG"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	const session = "locale-test"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session)
	t.Cleanup(func() { runTmux(t, binary, socket, "kill-session", "-t", "="+session) })
	spawner, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	const name = "agent-é"
	pane, err := spawner.Spawn(ctx, substrate.SpawnSpec{Name: name, Directory: root, Command: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	// A second backend has no record of the spawn, as a separate gang
	// process has none, so registration reads the pane listing.
	b, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.RegisterPane(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := b.CheckPane(ctx, id); err != nil || !ok {
		t.Fatalf("check pane = %v, %v", ok, err)
	}
	if closed, err := b.PaneClosed(ctx, id, Identity{}); err != nil || closed {
		t.Fatalf("pane closed = %v, %v", closed, err)
	}
	if _, err := b.ServerIdentity(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := b.TitleRegisteredPane(ctx, id, name); err != nil {
		t.Fatal(err)
	}
	panes, err := b.Panes(ctx)
	if err != nil || len(panes) != 2 || panes[1].Title != name || panes[1].Pane.ID != pane.ID {
		t.Fatalf("pane title %q = %+v, %v", name, panes, err)
	}
}
