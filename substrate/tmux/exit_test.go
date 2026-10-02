package tmux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/adambiggs/gangline/substrate"
)

// exitingSpec runs a process that holds the write end of root's exit pipe,
// waits for release, writes to stderr, and exits with status.
func exitingSpec(t *testing.T, binary, socket, root, release, status string) substrate.SpawnSpec {
	t.Helper()
	if err := syscall.Mkfifo(filepath.Join(root, "exit-pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `exec 3>"$5"; "$1" -S "$2" wait-for "$3"; printf 'boot failure\n' >&2; exit "$4"`
	return substrate.SpawnSpec{
		Name: "native", Directory: root, Command: "sh",
		Args:       []string{"-c", script, "sh", binary, socket, release, status, filepath.Join(root, "exit-pipe")},
		KeepExited: true,
	}
}

// awaitStart returns once the process holds the write end of root's exit
// pipe, which it opens only after its pane holds itself.
func awaitStart(t *testing.T, root string) *os.File {
	t.Helper()
	opened := make(chan *os.File, 1)
	failed := make(chan error, 1)
	go func() {
		// Opening blocks until the process holds the write end.
		pipe, err := os.Open(filepath.Join(root, "exit-pipe"))
		if err != nil {
			failed <- err
			return
		}
		opened <- pipe
	}()
	// Bounded so a process that never starts cannot hold the test forever.
	select {
	case pipe := <-opened:
		t.Cleanup(func() { pipe.Close() })
		return pipe
	case err := <-failed:
		t.Fatalf("await process start: %v", err)
	case <-time.After(time.Minute):
		t.Fatal("process did not start")
	}
	return nil
}

// releaseAndAwaitExit observes the exit as EOF on the exit pipe, which does not
// depend on when tmux reaps the process. tmux can leave an exited pane child
// unreaped, and then pane-died never fires; run-shell's own child makes the
// server collect every exited child before the command returns.
func releaseAndAwaitExit(t *testing.T, binary, socket, root, release string) {
	t.Helper()
	awaitExit(t, binary, socket, awaitStart(t, root), release)
}

func awaitExit(t *testing.T, binary, socket string, pipe *os.File, release string) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		if output, err := exec.Command(binary, "-S", socket, "wait-for", "-S", release).CombinedOutput(); err != nil {
			done <- fmt.Errorf("release: %v: %s", err, output)
			return
		}
		_, err := io.Copy(io.Discard, pipe)
		done <- err
	}()
	// Bounded so a process that never exits cannot hold the test forever.
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("await process exit: %v", err)
		}
	case <-time.After(time.Minute):
		t.Fatal("process did not exit")
	}
	runTmux(t, binary, socket, "run-shell", "true")
}

func assertExited(t *testing.T, err error, status string) {
	t.Helper()
	var exited *substrate.ExitedError
	if !errors.As(err, &exited) {
		t.Fatalf("error = %v, want native exit", err)
	}
	if exited.Status != status || exited.Output != "boot failure" {
		t.Fatalf("exit = %+v, want status %s and the native output", *exited, status)
	}
}

func TestCreateSessionKeepsExitedPaneReadable(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	backend, err := New(Config{Binary: binary, Socket: socket, Session: "exits"})
	if err != nil {
		t.Fatal(err)
	}
	pane, err := backend.CreateSession(context.Background(), exitingSpec(t, binary, socket, root, "release", "3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=exits") })
	releaseAndAwaitExit(t, binary, socket, root, "release")
	_, err = backend.Capture(context.Background(), pane.ID)
	assertExited(t, err, "3")
	_, err = backend.Identity(context.Background(), pane.ID)
	assertExited(t, err, "3")
	assertExited(t, backend.ReleaseExit(context.Background(), pane.ID), "3")
}

func TestSpawnKeepsOnlyItsOwnExitedPane(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	backend, err := New(Config{Binary: binary, Socket: socket, Session: "spawns"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := backend.CreateSession(context.Background(), substrate.SpawnSpec{Name: "first", Directory: root, Command: "sh"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=spawns") })
	runTmux(t, binary, socket, "set-hook", "-g", "after-new-window", "set-option -g @existing ran")
	pane, err := backend.Spawn(context.Background(), exitingSpec(t, binary, socket, root, "release", "4"))
	if err != nil {
		t.Fatal(err)
	}
	if ran := runTmux(t, binary, socket, "show-options", "-gv", "@existing"); strings.TrimSpace(ran) != "ran" {
		t.Fatalf("existing after-new-window hook did not run: %q", ran)
	}
	if hooks := runTmux(t, binary, socket, "show-hooks", "-g"); strings.Count(hooks, "after-new-window") != 1 {
		t.Fatalf("spawn changed the global hooks: %q", hooks)
	}
	if hooks := runTmux(t, binary, socket, "show-hooks", "-t", "=spawns:"); strings.Contains(hooks, "after-new-window") {
		t.Fatalf("spawn left a session hook: %q", hooks)
	}
	started := awaitStart(t, root)
	if held := runTmux(t, binary, socket, "show-options", "-p", "-v", "-t", string(first.ID), "remain-on-exit"); strings.TrimSpace(held) == "on" {
		t.Fatal("spawn held the session's existing pane")
	}
	awaitExit(t, binary, socket, started, "release")
	_, err = backend.Capture(context.Background(), pane.ID)
	assertExited(t, err, "4")
}

func TestReleaseExitLetsLivePaneClose(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	backend, err := New(Config{Binary: binary, Socket: socket, Session: "releases"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.CreateSession(context.Background(), substrate.SpawnSpec{Name: "first", Directory: root, Command: "sh"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=releases") })
	pane, err := backend.Spawn(context.Background(), exitingSpec(t, binary, socket, root, "release", "5"))
	if err != nil {
		t.Fatal(err)
	}
	started := awaitStart(t, root)
	if held := runTmux(t, binary, socket, "show-options", "-p", "-v", "-t", string(pane.ID), "remain-on-exit"); strings.TrimSpace(held) != "on" {
		t.Fatalf("spawned pane remain-on-exit = %q, want on", held)
	}
	if err := backend.ReleaseExit(context.Background(), pane.ID); err != nil {
		t.Fatal(err)
	}
	awaitExit(t, binary, socket, started, "release")
	if panes := runTmux(t, binary, socket, "list-panes", "-s", "-t", "=releases:", "-F", "#{pane_id}"); strings.Contains(panes, string(pane.ID)+"\n") {
		t.Fatalf("released pane %s stayed after exit: %q", pane.ID, panes)
	}
}

// User hooks that split the new window and open another run before any
// command after the launch, and take focus from the launched pane. The hold
// stays on the launched pane alone.
func TestLaunchHoldsItsPaneUnderUserHooks(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	for _, tc := range []struct {
		name   string
		launch func(*Backend, context.Context, substrate.SpawnSpec) (substrate.Pane, error)
		// existing creates the session before the launch under test.
		existing bool
	}{
		{name: "create", launch: (*Backend).CreateSession},
		{name: "spawn", launch: (*Backend).Spawn, existing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := privateTmuxRoot(t)
			socket := filepath.Join(root, "tmux.sock")
			runTmux(t, binary, socket, "new-session", "-d", "-s", "user", "sh")
			t.Cleanup(func() {
				_, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=user")
				_, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=hooked")
			})
			backend, err := New(Config{Binary: binary, Socket: socket, Session: "hooked"})
			if err != nil {
				t.Fatal(err)
			}
			if tc.existing {
				if _, err := backend.CreateSession(context.Background(), substrate.SpawnSpec{Name: "first", Directory: root, Command: "sh"}); err != nil {
					t.Fatal(err)
				}
			}
			for _, event := range []string{"after-new-session", "after-new-window"} {
				runTmux(t, binary, socket, "set-hook", "-g", event, "split-window sh ; new-window sh")
			}
			pane, err := tc.launch(backend, context.Background(), exitingSpec(t, binary, socket, root, "release", "5"))
			if err != nil {
				t.Fatal(err)
			}
			started := awaitStart(t, root)
			window := strings.TrimSpace(runTmux(t, binary, socket, "display-message", "-p", "-t", string(pane.ID), "#{window_id}"))
			if panes := strings.Fields(runTmux(t, binary, socket, "list-panes", "-t", window, "-F", "#{pane_id}")); len(panes) != 2 {
				t.Fatalf("user hook did not split the launched window: %q", panes)
			}
			if active := runTmux(t, binary, socket, "display-message", "-p", "-t", window, "#{pane_id}"); strings.TrimSpace(active) == string(pane.ID) {
				t.Fatal("user split did not take focus")
			}
			for _, id := range strings.Fields(runTmux(t, binary, socket, "list-panes", "-a", "-F", "#{pane_id}")) {
				held := strings.TrimSpace(runTmux(t, binary, socket, "show-options", "-p", "-v", "-t", id, "remain-on-exit"))
				if (held == "on") != (id == string(pane.ID)) {
					t.Fatalf("pane %s remain-on-exit = %q; launched pane is %s", id, held, pane.ID)
				}
			}
			awaitExit(t, binary, socket, started, "release")
			_, err = backend.Capture(context.Background(), pane.ID)
			assertExited(t, err, "5")
		})
	}
}

// The pane holds itself on the server gang named, though it starts in
// another directory than gang's, the socket's absolute path may exceed the
// length a unix socket address holds, and a ".." may follow a symbolic link.
func TestLaunchHoldsItsPaneOnRelativePaths(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	for _, tc := range []struct {
		name string
		long bool
		// linked runs gang from a link to a directory below the socket's and
		// names the binary and socket through "..".
		linked bool
	}{
		{name: "short"},
		{name: "long", long: true},
		{name: "linked", linked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := privateTmuxRoot(t)
			dir, link := root, root
			if tc.long {
				dir = filepath.Join(root, strings.Repeat("d", 120))
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				// The test reaches the server through a short link to its directory.
				link = filepath.Join(root, "s")
				if err := os.Symlink(dir, link); err != nil {
					t.Fatal(err)
				}
				if path := filepath.Join(dir, "tmux.sock"); len(path) < 108 {
					t.Fatalf("socket path %q fits a unix socket address", path)
				}
			}
			socket := filepath.Join(link, "tmux.sock")
			if err := os.Mkdir(filepath.Join(dir, "bin"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(binary, filepath.Join(dir, "bin", "tmux")); err != nil {
				t.Fatal(err)
			}
			config := Config{Binary: "bin/tmux", Socket: "tmux.sock", Session: "relative"}
			cwd := dir
			if tc.linked {
				below := filepath.Join(dir, "below")
				if err := os.Mkdir(below, 0o700); err != nil {
					t.Fatal(err)
				}
				// Without the link, ".." from the link would name its own directory.
				cwd = filepath.Join(root, "elsewhere", "linked")
				if err := os.Mkdir(filepath.Dir(cwd), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(below, cwd); err != nil {
					t.Fatal(err)
				}
				config.Binary, config.Socket = "../bin/tmux", "../tmux.sock"
			}
			t.Chdir(cwd)
			backend, err := New(config)
			if err != nil {
				t.Fatal(err)
			}
			spec := exitingSpec(t, binary, socket, dir, "release", "6")
			spec.Directory = filepath.Join(dir, "work")
			if err := os.Mkdir(spec.Directory, 0o700); err != nil {
				t.Fatal(err)
			}
			work, err := filepath.EvalSymlinks(spec.Directory)
			if err != nil {
				t.Fatal(err)
			}
			// The native command runs in its own directory, not the socket's.
			opened, rest, _ := strings.Cut(spec.Args[1], "; ")
			spec.Args[1] = opened + `; [ "$(pwd -P)" = '` + work + `' ] || exit 9; ` + rest
			pane, err := backend.CreateSession(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=relative") })
			releaseAndAwaitExit(t, binary, socket, dir, "release")
			_, err = backend.Capture(context.Background(), pane.ID)
			assertExited(t, err, "6")
		})
	}
}

// The pane holds itself whatever shell the user's server runs commands with.
func TestLaunchHoldsItsPaneUnderAnyDefaultShell(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	runTmux(t, binary, socket, "new-session", "-d", "-s", "user", "sh")
	t.Cleanup(func() {
		_, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=user")
		_, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=shelled")
	})
	// A shell that parses no command, standing in for one without POSIX syntax.
	shell := filepath.Join(root, "shell")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nexit 97\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runTmux(t, binary, socket, "set-option", "-g", "default-shell", shell)
	backend, err := New(Config{Binary: binary, Socket: socket, Session: "shelled"})
	if err != nil {
		t.Fatal(err)
	}
	pane, err := backend.CreateSession(context.Background(), exitingSpec(t, binary, socket, root, "release", "7"))
	if err != nil {
		t.Fatal(err)
	}
	releaseAndAwaitExit(t, binary, socket, root, "release")
	_, err = backend.Capture(context.Background(), pane.ID)
	assertExited(t, err, "7")
}

// A pane that cannot hold itself never starts the native command, whose exit
// would otherwise close the pane with its output.
func TestLaunchWithoutHoldDoesNotStartNative(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	refusing := filepath.Join(root, "tmux")
	// Refuses only the hold the pane sets on itself.
	script := "#!/bin/sh\n[ \"$3 $7 $8\" = \"set-option remain-on-exit on\" ] && exit 1\nexec '" + binary + "' \"$@\"\n"
	if err := os.WriteFile(refusing, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	// The user session keeps the server alive after the launched one closes.
	runTmux(t, binary, socket, "new-session", "-d", "-s", "user", "sh")
	t.Cleanup(func() {
		_, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=user")
		_, _ = runTmuxResult(binary, socket, "kill-session", "-t", "=unheld")
	})
	runTmux(t, binary, socket, "set-hook", "-g", "session-closed", "wait-for -S closed")
	backend, err := New(Config{Binary: refusing, Socket: socket, Session: "unheld"})
	if err != nil {
		t.Fatal(err)
	}
	started := filepath.Join(root, "started")
	if _, err := backend.CreateSession(context.Background(), substrate.SpawnSpec{
		Name: "native", Directory: root, Command: "sh",
		Args:       []string{"-c", `touch "$1"; exec sleep 600`, "sh", started},
		KeepExited: true,
	}); err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() {
		output, err := exec.Command(binary, "-S", socket, "wait-for", "closed").CombinedOutput()
		if err != nil {
			err = fmt.Errorf("%v: %s", err, output)
		}
		closed <- err
	}()
	// Bounded so a native that started and holds its pane cannot hold the
	// test forever.
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("await session close: %v", err)
		}
	case <-time.After(time.Minute):
		t.Fatal("launched session stayed open")
	}
	if _, err := os.Stat(started); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("native command started without the hold: %v", err)
	}
}

func TestExitedOutputIsBoundedInBytes(t *testing.T) {
	long := strings.Repeat("é", exitedOutputBytes)
	// The odd byte puts the cut inside a character.
	output := exited("1", "first\n"+long+"x\n").Output
	if len(output) > exitedOutputBytes || !utf8.ValidString(output) || !strings.HasSuffix(output, "éx") {
		t.Fatalf("output is %d bytes, valid UTF-8 %t", len(output), utf8.ValidString(output))
	}
}
