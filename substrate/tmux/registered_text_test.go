package tmux

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestRegisteredMultilineTextPreservesWhitespace(t *testing.T) {
	want := "\x1b[200~contract\n [/gang:contract#0123456789abcdef-contract]\n\n  indented\n\ttabbed\n'quote' \"; display-message -p injected; #\" $HOME ${HOME} #{pane_id} `uname` \\\n  after backslash\n~ ~/dir ~root é猫\r\x01\x7f trailing  \n\x1b[201~\n"
	testRegisteredText(t, want)
}

func TestRegisteredLongTextPreservesExactBytes(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
	}{
		{"failed-brief-size", 4300},
		{"well-above-brief-size", 65536},
		{"maximum-envelope-size", 1 << 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want string
			if tc.size == 4300 {
				// Newlines expand fourfold in tmux's command argument.
				want = strings.Repeat("\n", tc.size)
				if len(tmuxCommand("send-keys", "-t", "%1", "-l", "--", want)) <= 16*1024 {
					t.Fatal("fixture does not exceed tmux's IPC command limit")
				}
			} else if tc.size == 65536 {
				line := "  spaced 'quote' \"double\" $HOME #{pane_id} \\ é猫\t\r\x1b[200~\x01\x7f\n"
				want = strings.Repeat(line, tc.size/len(line)+1)[:tc.size]
				want = want[:strings.LastIndexByte(want, '\n')+1]
			} else {
				line := strings.Repeat("a", 1000) + "\n"
				want = strings.Repeat(line, tc.size/len(line))
			}
			testRegisteredText(t, want)
		})
	}
}

func TestRegisteredTextInCopyMode(t *testing.T) {
	testRegisteredTextMode(t, "\x1b[200~[gang:gangline:startup#b480bb271b092c51 startup] No assignment was supplied. [/gang:gangline:startup#b480bb271b092c51]\x1b[201~\n", true)
}

func testRegisteredText(t *testing.T, want string) {
	testRegisteredTextMode(t, want, false)
}

func testRegisteredTextMode(t *testing.T, want string, copyMode bool) {
	t.Helper()
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	const session = "registered-text-test"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session)
	t.Cleanup(func() { runTmux(t, binary, socket, "kill-session", "-t", "="+session) })
	if listed := strings.TrimSpace(runTmux(t, binary, socket, "list-sessions", "-F", "#{session_name}")); listed != session {
		t.Fatalf("private server sessions = %q", listed)
	}
	b, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "typed")
	ready := filepath.Join(root, "ready")
	if result, err := exec.Command("mkfifo", ready).CombinedOutput(); err != nil {
		t.Fatalf("create ready pipe: %v\n%s", err, result)
	}
	script := `stty raw -echo; printf x > "$4"; while IFS= read -r text; do [ "$text" = END ] && break; printf '%s\n' "$text"; done > "$1"; "$2" -S "$3" wait-for -S typed; exec cat`
	pane, err := b.Spawn(context.Background(), substrate.SpawnSpec{Name: "text", Directory: root, Command: "sh", Args: []string{"-c", script, "sh", output, binary, socket, ready}})
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.RegisterPane(context.Background(), pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := os.Open(ready)
	if err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	_, readErr := pipe.Read(one[:])
	closeErr := pipe.Close()
	if readErr != nil || closeErr != nil || one[0] != 'x' {
		t.Fatalf("shell ready pipe: byte=%q read=%v close=%v", one, readErr, closeErr)
	}
	command, err := b.ForegroundCommand(context.Background(), pane.ID)
	if err != nil || command != "sh" && command != "bash" && command != "dash" {
		t.Fatalf("shell foreground command = %q: %v", command, err)
	}
	runTmux(t, binary, socket, "set-buffer", "keep")
	if copyMode {
		runTmux(t, binary, socket, "copy-mode", "-t", id.Pane)
	}
	if err := b.SendRegisteredKeys(context.Background(), id, command, substrate.Keys{Text: want + "END", Names: []string{"C-j"}}); err != nil {
		t.Fatal(err)
	}
	runTmux(t, binary, socket, "wait-for", "typed")
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("typed %d bytes; want %d exact bytes", len(got), len(want))
	}
	if got := runTmux(t, binary, socket, "save-buffer", "-"); got != "keep" {
		t.Fatalf("default tmux buffer changed: %q", got)
	}
}
