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
	want := "\x1b[200~contract\n [/gang:gangline:contract#0123456789abcdef-contract]\n\n  indented\n\ttabbed\n'quote' \"; display-message -p injected; #\" $HOME ${HOME} #{pane_id} `uname` \\\n  after backslash\n~ ~/dir ~root é猫\r\x01\x7f trailing  \n\x1b[201~\n"
	if err := b.SendRegisteredKeys(context.Background(), id, command, substrate.Keys{Text: want + "END", Names: []string{"C-j"}}); err != nil {
		t.Fatal(err)
	}
	runTmux(t, binary, socket, "wait-for", "typed")
	got, err := os.ReadFile(output)
	if err != nil || string(got) != want {
		t.Fatalf("typed = %q, %v; want %q", got, err, want)
	}
}
