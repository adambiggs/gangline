package tmux

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextWidgetIsSessionScopedAndRestoresInheritance(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux required")
	}
	socket := filepath.Join(t.TempDir(), "widget.sock")
	const session = "gangline-widget-test"
	runTmux(t, binary, socket, "-f", "/dev/null", "new-session", "-d", "-s", session)
	t.Cleanup(func() { runTmux(t, binary, socket, "kill-session", "-t", "="+session) })
	listed := strings.TrimSpace(runTmux(t, binary, socket, "list-sessions", "-F", "#{session_name}"))
	if listed != session {
		t.Fatalf("socket contains other sessions: %s", listed)
	}
	runTmux(t, binary, socket, "set-option", "-g", "status-right", "operator-global")
	backend, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.ContextWidget(context.Background(), "h1", "worker context 25%"); err != nil {
		t.Fatal(err)
	}
	if global := strings.TrimSpace(runTmux(t, binary, socket, "show-options", "-gv", "status-right")); global != "operator-global" {
		t.Fatalf("global changed: %s", global)
	}
	if err := backend.PublishContext(context.Background(), "h1", "worker context 30%"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(runTmux(t, binary, socket, "show-options", "-v", "-t", session, "@gangline_context_value")); got != "worker context 30%" {
		t.Fatalf("widget = %s", got)
	}
	if err := backend.ContextWidget(context.Background(), "", ""); err != nil {
		t.Fatal(err)
	}
	options := runTmux(t, binary, socket, "show-options", "-t", session)
	if hasSessionOption(options, "status-right") || strings.Contains(options, "@gangline_context_") {
		t.Fatalf("widget left local options: %s", options)
	}
}
