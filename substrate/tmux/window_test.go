package tmux

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

// windowSession starts a private session with a first window named "first" at
// index 0. Session options override whatever the user's tmux config sets.
func windowSession(t *testing.T, session string, renumber bool) (*Backend, func(...string) string) {
	t.Helper()
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	backend, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.CreateSession(context.Background(), substrate.SpawnSpec{Name: "first", Directory: root, Command: "sh"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runTmuxResult(binary, socket, "kill-session", "-t", "="+session) })
	tmux := func(arguments ...string) string { return runTmux(t, binary, socket, arguments...) }
	value := "off"
	if renumber {
		value = "on"
	}
	tmux("set-option", "-t", session, "renumber-windows", value)
	tmux("set-option", "-t", session, "base-index", "0")
	if strings.TrimSpace(tmux("list-windows", "-t", "="+session+":", "-F", "#{window_index}")) != "0" {
		tmux("move-window", "-s", "="+session+":", "-t", "="+session+":0")
	}
	return backend, tmux
}

func spawnWindow(t *testing.T, backend *Backend, name string, at *Window) PaneIdentity {
	t.Helper()
	pane, err := backend.SpawnWindow(context.Background(), substrate.SpawnSpec{Name: name, Directory: t.TempDir(), Command: "sh"}, at)
	if err != nil {
		t.Fatal(err)
	}
	id, err := backend.RegisterPane(context.Background(), pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func windowList(tmux func(...string) string, session string) string {
	return strings.TrimSpace(tmux("list-windows", "-t", "="+session+":", "-F", "#{window_index}=#{window_name}"))
}

func TestSpawnWindowRefusesAnOccupiedIndex(t *testing.T) {
	backend, tmux := windowSession(t, "window-index", false)
	spawnWindow(t, backend, "seven", &Window{Index: 7})
	before := windowList(tmux, "window-index")
	if before != "0=first\n7=seven" {
		t.Fatalf("windows = %q", before)
	}
	_, err := backend.SpawnWindow(context.Background(), substrate.SpawnSpec{Name: "other", Directory: t.TempDir(), Command: "sh"}, &Window{Index: 7})
	if err == nil || !strings.Contains(err.Error(), "index 7 in use") {
		t.Fatalf("spawn at an occupied index: %v", err)
	}
	if after := windowList(tmux, "window-index"); after != before {
		t.Fatalf("refused spawn changed the windows: %q", after)
	}
	windows, err := backend.Windows(context.Background())
	if err != nil || len(windows) != 2 || windows[0].Index != 0 || windows[0].Name != "first" || windows[1].Index != 7 || windows[1].Name != "seven" || windows[0].ID == windows[1].ID {
		t.Fatalf("windows = %+v, %v", windows, err)
	}
	if _, err := backend.SpawnWindow(context.Background(), substrate.SpawnSpec{Name: "x", Directory: t.TempDir(), Command: "sh"}, &Window{Index: -1}); err == nil {
		t.Fatal("negative window index accepted")
	}
}
