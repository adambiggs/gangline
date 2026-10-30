//go:build acceptance

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

// windowID is the id of the session's window named name.
func windowID(t *testing.T, backend *Backend, name string) string {
	t.Helper()
	windows, err := backend.Windows(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, window := range windows {
		if window.Name == name {
			return window.ID
		}
	}
	t.Fatalf("no window %q in %+v", name, windows)
	return ""
}

func restoreWindow(t *testing.T, backend *Backend, name string, position WindowPosition) bool {
	t.Helper()
	pane, restored, err := backend.RestoreWindow(context.Background(), substrate.SpawnSpec{Name: name, Directory: t.TempDir(), Command: "sh"}, position)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.RegisterPane(context.Background(), pane.ID); err != nil {
		t.Fatal(err)
	}
	return restored
}

func readPosition(t *testing.T, backend *Backend, id PaneIdentity) WindowPosition {
	t.Helper()
	position, ok, err := backend.WindowPosition(context.Background(), id)
	if err != nil || !ok {
		t.Fatalf("position of %s: %v, %v", id.Pane, ok, err)
	}
	return position
}

// With renumber-windows on, closing a window renumbers every window, filling
// any lower gap too. The next window's id survives that, and an insert before
// it restores the order.
func TestRestoreWindowAfterRenumber(t *testing.T) {
	backend, tmux := windowSession(t, "restore-renumber", false)
	agent := spawnWindow(t, backend, "agent", &Window{Index: 2})
	spawnWindow(t, backend, "later", &Window{Index: 3})
	tail := spawnWindow(t, backend, "tail", &Window{Index: 4})
	tmux("set-option", "-t", "restore-renumber", "renumber-windows", "on")
	position := readPosition(t, backend, agent)
	if want := (WindowPosition{Index: 2, Renumber: true, Next: windowID(t, backend, "later")}); position != want {
		t.Fatalf("position = %+v, want %+v", position, want)
	}
	if err := backend.RemoveRegisteredPane(context.Background(), agent); err != nil {
		t.Fatal(err)
	}
	if got := windowList(tmux, "restore-renumber"); got != "0=first\n1=later\n2=tail" {
		t.Fatalf("renumbered windows = %q", got)
	}
	if !restoreWindow(t, backend, "agent", position) {
		t.Fatal("place before the next window reported lost")
	}
	if got := windowList(tmux, "restore-renumber"); got != "0=first\n1=agent\n2=later\n3=tail" {
		t.Fatalf("restored windows = %q", got)
	}
	// The last window has no next window; renumbering left no gap, so default
	// placement is its place.
	last := readPosition(t, backend, tail)
	if last.Next != "" {
		t.Fatalf("last window position = %+v", last)
	}
	if err := backend.RemoveRegisteredPane(context.Background(), tail); err != nil {
		t.Fatal(err)
	}
	if !restoreWindow(t, backend, "tail", last) {
		t.Fatal("last window's place reported lost")
	}
	if got := windowList(tmux, "restore-renumber"); got != "0=first\n1=agent\n2=later\n3=tail" {
		t.Fatalf("restored last window = %q", got)
	}
}

// A next window that closed during the restart leaves no place to restore;
// the window goes where tmux puts it and moves no other.
func TestRestoreWindowWhoseNextWindowClosed(t *testing.T) {
	backend, tmux := windowSession(t, "restore-next-closed", true)
	agent := spawnWindow(t, backend, "agent", nil)
	spawnWindow(t, backend, "later", nil)
	spawnWindow(t, backend, "tail", nil)
	position := readPosition(t, backend, agent)
	if err := backend.RemoveRegisteredPane(context.Background(), agent); err != nil {
		t.Fatal(err)
	}
	tmux("kill-window", "-t", "=restore-next-closed:"+position.Next)
	if restoreWindow(t, backend, "agent", position) {
		t.Fatal("restore reported a place whose next window closed")
	}
	if got := windowList(tmux, "restore-next-closed"); got != "0=first\n1=tail\n2=agent" {
		t.Fatalf("windows = %q", got)
	}
}

// Without renumbering, a closed window leaves its index free, and the restore
// skips a lower gap that default placement would fill. An index another
// window took during the restart stays that window's.
func TestRestoreWindowAtItsIndex(t *testing.T) {
	backend, tmux := windowSession(t, "restore-index", false)
	agent := spawnWindow(t, backend, "agent", &Window{Index: 2})
	position := readPosition(t, backend, agent)
	if position != (WindowPosition{Index: 2}) {
		t.Fatalf("position = %+v", position)
	}
	if err := backend.RemoveRegisteredPane(context.Background(), agent); err != nil {
		t.Fatal(err)
	}
	if !restoreWindow(t, backend, "agent", position) {
		t.Fatal("free index reported lost")
	}
	if got := windowList(tmux, "restore-index"); got != "0=first\n2=agent" {
		t.Fatalf("windows = %q", got)
	}
	tmux("kill-window", "-t", "=restore-index:2")
	tmux("new-window", "-d", "-t", "=restore-index:2", "-n", "operator", "sh")
	if restoreWindow(t, backend, "agent", position) {
		t.Fatal("restore reported an index another window holds")
	}
	if got := windowList(tmux, "restore-index"); got != "0=first\n1=agent\n2=operator" {
		t.Fatalf("windows = %q", got)
	}
}

// A window linked into another session has an index there too; the position
// is the one in the registered session.
func TestWindowPositionInItsRegisteredSession(t *testing.T) {
	backend, tmux := windowSession(t, "position-linked", false)
	agent := spawnWindow(t, backend, "agent", &Window{Index: 4})
	tmux("new-session", "-d", "-s", "position-linked-view", "sh")
	t.Cleanup(func() { tmux("kill-session", "-t", "=position-linked-view") })
	tmux("link-window", "-s", agent.Pane, "-t", "=position-linked-view:7")
	tmux("select-window", "-t", "=position-linked-view:7")
	if position := readPosition(t, backend, agent); position != (WindowPosition{Index: 4}) {
		t.Fatalf("position = %+v", position)
	}
}

func TestWindowPositionOfSharedAndClosedPanes(t *testing.T) {
	backend, _ := windowSession(t, "window-shared", false)
	agent := spawnWindow(t, backend, "agent", nil)
	pane, err := backend.Split(context.Background(), agent, substrate.SpawnSpec{Name: "split", Directory: t.TempDir(), Command: "sh"}, false)
	if err != nil {
		t.Fatal(err)
	}
	split, err := backend.RegisterPane(context.Background(), pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []PaneIdentity{agent, split} {
		position, ok, err := backend.WindowPosition(context.Background(), id)
		if err != nil || !ok || position != (WindowPosition{Index: 1, Shared: true}) {
			t.Fatalf("position of %s = %+v, %v, %v", id.Pane, position, ok, err)
		}
	}
	if err := backend.RemoveRegisteredPane(context.Background(), split); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := backend.WindowPosition(context.Background(), split); ok || err != nil {
		t.Fatalf("closed pane position: %v, %v", ok, err)
	}
	replaced := agent
	replaced.Generation = strings.Repeat("0", 64)
	if _, ok, err := backend.WindowPosition(context.Background(), replaced); ok || err != nil {
		t.Fatalf("replaced pane position: %v, %v", ok, err)
	}
}
