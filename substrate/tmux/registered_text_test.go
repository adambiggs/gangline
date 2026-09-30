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
	script := `stty raw -echo; "$2" -S "$3" wait-for -S ready; while IFS= read -r text; do [ "$text" = END ] && break; printf '%s\n' "$text"; done > "$1"; "$2" -S "$3" wait-for -S typed; exec cat`
	pane, err := b.Spawn(context.Background(), substrate.SpawnSpec{Name: "text", Directory: root, Command: "sh", Args: []string{"-c", script, "sh", output, binary, socket}})
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.RegisterPane(context.Background(), pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	runTmux(t, binary, socket, "wait-for", "ready")
	want := "\x1b[200~contract\n [/gang:gangline:contract#0123456789abcdef-contract]\n\n  indented\n\ttabbed\n'quote' \"; display-message -p injected; #\" $HOME ${HOME} #{pane_id} `uname` \\\n  after backslash\n~ ~/dir ~root é猫\r\x01\x7f trailing  \n\x1b[201~\n"
	if err := b.SendRegisteredKeys(context.Background(), id, "sh", substrate.Keys{Text: want + "END", Names: []string{"C-j"}}); err != nil {
		t.Fatal(err)
	}
	runTmux(t, binary, socket, "wait-for", "typed")
	got, err := os.ReadFile(output)
	if err != nil || string(got) != want {
		t.Fatalf("typed = %q, %v; want %q", got, err, want)
	}
}
