package tmux

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestPaneStatusFormat(t *testing.T) {
	const title = "#{?@gangline_title,#{@gangline_title},#{window_name}}"
	for _, tc := range []struct{ input, want string }{
		{"#[bold] #I:#W / #{window_name}", "#[bold] #I:" + title + " / " + title},
		{title + " / #W", title + " / " + title},
		{"##W ##{window_name} #W", "##W ##{window_name} " + title},
		{"#" + title + " #W", "#" + title + " " + title},
	} {
		if got := paneStatusFormat(tc.input); got != tc.want {
			t.Errorf("paneStatusFormat(%q) = %q, want %q", tc.input, got, tc.want)
		} else if twice := paneStatusFormat(got); twice != got {
			t.Errorf("second conversion of %q = %q", got, twice)
		}
	}
}

func TestWindowTabsFollowActivePaneStatus(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	const session = "status-tabs"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session)
	t.Cleanup(func() { runTmux(t, binary, socket, "kill-session", "-t", "="+session) })
	runTmux(t, binary, socket, "set-option", "-g", "window-status-format", "#[fg=blue] #I:#{window_name}")
	runTmux(t, binary, socket, "set-option", "-g", "window-status-current-format", "#[bold] #I:#W")
	b, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "worker", Directory: root, Command: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.RegisterPane(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	window := strings.TrimSpace(runTmux(t, binary, socket, "display-message", "-p", "-t", id.Pane, "#{window_id}"))
	display := func(key string) string {
		return strings.TrimSpace(runTmux(t, binary, socket, "display-message", "-p", "-t", window, "#{E:"+key+"}"))
	}
	for _, label := range []string{"worker", "-worker-", "~worker~", "?worker?"} {
		t.Run(label, func(t *testing.T) {
			runTmux(t, binary, socket, "rename-window", "-t", window, "--", label)
			if err := b.TitleRegisteredPane(ctx, id, "~worker~"); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"window-status-format", "window-status-current-format"} {
				if got := display(key); !strings.HasSuffix(got, ":~worker~") {
					t.Fatalf("window %q %s renders %q, want current idle symbol", label, key, got)
				}
			}
			if got := strings.TrimSpace(runTmux(t, binary, socket, "display-message", "-p", "-t", window, "#{window_name}")); got != label {
				t.Fatalf("placement label changed: %q, want %q", got, label)
			}
		})
	}
	sibling, err := b.Split(ctx, id, substrate.SpawnSpec{Name: "peer", Directory: root, Command: "cat"}, false)
	if err != nil {
		t.Fatal(err)
	}
	peer, err := b.RegisterPane(ctx, sibling.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.TitleRegisteredPane(ctx, peer, "-peer-"); err != nil {
		t.Fatal(err)
	}
	for _, view := range []struct{ pane, title string }{{id.Pane, "~worker~"}, {peer.Pane, "-peer-"}} {
		runTmux(t, binary, socket, "select-pane", "-t", view.pane)
		if got := display("window-status-format"); !strings.HasSuffix(got, ":"+view.title) {
			t.Fatalf("selected %s renders %q", view.pane, got)
		}
	}
	runTmux(t, binary, socket, "select-pane", "-t", peer.Pane, "-T", "native-title")
	if got := display("window-status-current-format"); !strings.HasSuffix(got, ":-peer-") {
		t.Fatalf("native terminal title changed status: %q", got)
	}
	if got := strings.TrimSpace(runTmux(t, binary, socket, "show-options", "-g", "-v", "window-status-format")); got != "#[fg=blue] #I:#{window_name}" {
		t.Fatalf("global operator format changed: %q", got)
	}
	runTmux(t, binary, socket, "set-option", "-p", "-u", "-t", peer.Pane, "@gangline_title")
	if got := display("window-status-format"); !strings.HasSuffix(got, ":?worker?") {
		t.Fatalf("unregistered active pane lacks window-label fallback: %q", got)
	}
}
