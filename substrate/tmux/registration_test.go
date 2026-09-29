package tmux

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestPaneRegistrationPrivateServer(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	const session = "registration-test"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session)
	t.Cleanup(func() { runTmux(t, binary, socket, "kill-session", "-t", "="+session) })
	if listed := strings.TrimSpace(runTmux(t, binary, socket, "list-sessions", "-F", "#{session_name}")); listed != session {
		t.Fatalf("private server sessions = %q", listed)
	}
	b, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	outputFile := filepath.Join(root, "typed")
	script := `IFS= read -r text; printf '%s' "$text" > "$1"; "$2" -S "$3" wait-for -S typed; exec cat`
	pane, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "registered", Directory: root, Command: "sh", Args: []string{"-c", script, "sh", outputFile, binary, socket}})
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.RegisterPane(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := b.RegisterPane(ctx, pane.ID)
	if err != nil || again != id {
		t.Fatalf("repeat registration = %+v, %v; want %+v", again, err, id)
	}
	if visible, err := b.ProcessVisibility(ctx, pane.ID); err != nil || !visible {
		t.Fatalf("local visibility = %v, %v", visible, err)
	}
	if err := b.VerifyCaller(ctx, pane.ID); err == nil {
		t.Fatal("outside caller accepted")
	}
	for _, change := range []func(*PaneIdentity){
		func(id *PaneIdentity) { id.Generation = strings.Repeat("a", 64) },
		func(id *PaneIdentity) { id.Session = "$999999" },
	} {
		stale := id
		change(&stale)
		if ok, err := b.CheckPane(ctx, stale); !errors.Is(err, ErrPaneReplaced) || ok {
			t.Fatalf("stale identity accepted: %+v, %v", stale, err)
		}
		if err := b.RemoveRegisteredPane(ctx, stale); err == nil {
			t.Fatal("stale removal accepted")
		}
		if err := b.SendRegisteredKeys(ctx, stale, "sh", substrate.Keys{Text: "wrong"}); err == nil {
			t.Fatal("stale send accepted")
		}
	}
	want := `literal 'quote' "double" $HOME; #{pane_id} \\ trailing`
	if err := b.SendRegisteredKeys(ctx, id, "sh", substrate.Keys{Text: want, Submit: true}); err != nil {
		t.Fatal(err)
	}
	runTmux(t, binary, socket, "wait-for", "typed")
	got, err := os.ReadFile(outputFile)
	if err != nil || string(got) != want {
		t.Fatalf("typed = %q, %v; want %q", got, err, want)
	}
	prefix, _ := New(Config{Binary: binary, Socket: socket, Session: "registration"})
	if _, err := prefix.Spawn(ctx, substrate.SpawnSpec{Name: "wrong-session", Directory: root, Command: "cat"}); err == nil {
		t.Fatal("spawn accepted session name prefix")
	}
	windows, err := b.Windows(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, window := range windows {
		if window.Name == "wrong-session" {
			t.Fatal("failed prefix spawn created a pane in the longer session")
		}
	}
	if exists, err := prefix.SessionExists(ctx); exists || err != nil {
		t.Fatalf("session prefix exists = %v, %v", exists, err)
	}
	if _, err := prefix.RegisterPane(ctx, pane.ID); err == nil {
		t.Fatal("session name prefix accepted")
	}
	if _, err := prefix.CheckPane(ctx, id); !errors.Is(err, ErrPaneReplaced) {
		t.Fatalf("missing configured session not classified as replacement: %v", err)
	}
	if err := b.SendRegisteredKeys(ctx, id, "sh},1}", substrate.Keys{Text: "wrong"}); err == nil {
		t.Fatal("tmux format syntax accepted in foreground command")
	}
	// Change the generation only after the ordinary read-side identity check.
	// The final tmux guard must reject both typing and removal.
	wrapper := filepath.Join(root, "replace-before-mutation")
	fixture := "#!/bin/sh\nif [ \"$3\" = if-shell ]; then\n" +
		strings.TrimPrefix(shellCommand(binary, []string{"-S", socket, "set-option", "-s", generationOption, strings.Repeat("b", 64)}), "exec ") +
		"\nfi\nexec " + "'" + strings.ReplaceAll(binary, "'", "'\\''") + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(fixture), 0o700); err != nil {
		t.Fatal(err)
	}
	replacement, _ := New(Config{Binary: wrapper, Socket: socket, Session: session})
	for _, mutate := range []func() error{
		func() error { return replacement.RemoveRegisteredPane(ctx, id) },
		func() error { return replacement.SendRegisteredKeys(ctx, id, "sh", substrate.Keys{Text: "wrong"}) },
	} {
		if err := mutate(); err == nil || !strings.Contains(err.Error(), "identity changed") {
			t.Fatalf("generation replacement guard: %v", err)
		}
		runTmux(t, binary, socket, "set-option", "-s", generationOption, id.Generation)
		if ok, err := b.CheckPane(ctx, id); !ok || err != nil {
			t.Fatalf("guarded pane changed: %v, %v", ok, err)
		}
	}
	foregroundWrapper := filepath.Join(root, "replace-foreground")
	changedShell := shellCommand("sh", []string{"-c", strings.TrimPrefix(shellCommand(binary, []string{"-S", socket, "wait-for", "-S", "foreground-changed"}), "exec ") + "; read line"})
	changeForeground := strings.TrimPrefix(shellCommand(binary, []string{"-S", socket, "respawn-pane", "-k", "-t", id.Pane, changedShell}), "exec ")
	changedBarrier := strings.TrimPrefix(shellCommand(binary, []string{"-S", socket, "wait-for", "foreground-changed"}), "exec ")
	foregroundScript := "#!/bin/sh\nif [ \"$3\" = if-shell ]; then\n" + changeForeground + "\n" + changedBarrier + "\nfi\nexec '" + strings.ReplaceAll(binary, "'", "'\\''") + "' \"$@\"\n"
	if err := os.WriteFile(foregroundWrapper, []byte(foregroundScript), 0o700); err != nil {
		t.Fatal(err)
	}
	if command, err := b.ForegroundCommand(ctx, pane.ID); err != nil || command != "cat" {
		t.Fatalf("precheck foreground = %q, %v", command, err)
	}
	changedForeground, _ := New(Config{Binary: foregroundWrapper, Socket: socket, Session: session})
	if err := changedForeground.SendRegisteredKeys(ctx, id, "cat", substrate.Keys{Text: "wrong", Submit: true}); err == nil {
		t.Fatal("foreground replacement between precheck and typing was accepted")
	}
	native, err := b.Identity(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The wrapper respawns only after the read-side checks have finished.
	if err := changedForeground.RemoveRegisteredNativePane(ctx, id, native); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("native replacement between precheck and removal: %v", err)
	}
	if ok, err := b.CheckPane(ctx, id); !ok || err != nil {
		t.Fatalf("replacement removed: %v, %v", ok, err)
	}
	if err := b.RemoveRegisteredPane(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ok, err := b.CheckPane(ctx, id); ok || err != nil {
		t.Fatalf("removed pane = %v, %v", ok, err)
	}
	if err := b.RemoveRegisteredPane(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := b.SendRegisteredKeys(ctx, id, "sh", substrate.Keys{Submit: true}); err == nil {
		t.Fatal("send to absent pane accepted")
	}
	if visible, err := b.ProcessVisibility(ctx, pane.ID); visible || err != nil {
		t.Fatalf("absent pane visibility = %v, %v", visible, err)
	}
	absent, _ := New(Config{Binary: binary, Socket: filepath.Join(root, "absent.sock"), Session: session})
	if visible, err := absent.ProcessVisibility(ctx, pane.ID); visible || err != nil {
		t.Fatalf("absent server visibility = %v, %v", visible, err)
	}
	cleanupPane, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "cleanup", Directory: root, Command: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	birth, ok := b.bornPane(cleanupPane.ID)
	if !ok {
		t.Fatal("spawn did not save its birth identity")
	}
	runTmux(t, binary, socket, "set-option", "-s", generationOption, strings.Repeat("c", 64))
	if _, err := b.RegisterPane(ctx, cleanupPane.ID); err == nil {
		t.Fatal("registration adopted replacement generation")
	}
	if err := b.KillUnregisteredPane(ctx, cleanupPane.ID); err == nil {
		t.Fatal("cleanup removed replacement generation")
	}
	runTmux(t, binary, socket, "set-option", "-s", generationOption, birth.Generation)
	if err := prefix.KillUnregisteredPane(ctx, cleanupPane.ID); err == nil {
		t.Fatal("cleanup accepted missing birth witness")
	}
	if err := b.KillUnregisteredPane(ctx, cleanupPane.ID); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := b.registeredPane(ctx, string(cleanupPane.ID)); exists || err != nil {
		t.Fatalf("failed registration cleanup left pane: %v, %v", exists, err)
	}
}

func TestVisibilityPreservesUnexpectedErrors(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "broken-tmux")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\necho unexpected-refusal >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	b, _ := New(Config{Binary: binary, Session: "test"})
	if visible, err := b.ProcessVisibility(context.Background(), "%1"); visible || err == nil || !strings.Contains(err.Error(), "unexpected-refusal") {
		t.Fatalf("unexpected visibility error = %v, %v", visible, err)
	}
}

func TestCallerAncestry(t *testing.T) {
	read := func(pid int) (processRecord, error) {
		return processRecord{Process: substrate.Process{PID: pid, ParentPID: map[int]int{3: 2, 2: 1, 1: 0}[pid]}}, nil
	}
	if err := verifyCallerAncestry(3, 2, read); err != nil {
		t.Fatal(err)
	}
	if err := verifyCallerAncestry(3, 4, read); err == nil {
		t.Fatal("outside caller accepted")
	}
}

func TestAbsentServerClassification(t *testing.T) {
	for _, output := range []string{"no server running on /tmp/test.sock\n", "error connecting to /tmp/test.sock (No such file or directory)\n", "error connecting to /tmp/test.sock (Connection refused)\n"} {
		if !absentTmuxServer(output) {
			t.Fatalf("absence not recognized: %q", output)
		}
	}
	if absentTmuxServer("error connecting to /tmp/test.sock (Permission denied)\n") {
		t.Fatal("permission failure treated as absence")
	}
}

func TestCreatedSessionDoesNotRetainHitchCapabilities(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("GANG_AGENT_ID", "outer-identity")
	t.Setenv("GANG_AGENT_NONCE", "outer-nonce")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	const session = "capability-session"
	b, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	script := `printf '%s\n' "${GANG_AGENT_ID-unset}" "${GANG_AGENT_NONCE-unset}" "${GANGLINE_HITCH_ID-unset}" > "$1"; "$2" -S "$3" wait-for -S "$4"; exec cat`
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	_, err = b.CreateSession(context.Background(), substrate.SpawnSpec{Name: "first", Directory: root, Command: "sh", Args: []string{"-c", script, "sh", first, binary, socket, "first-ready"}, Env: map[string]string{"GANG_AGENT_ID": "first-id", "GANG_AGENT_NONCE": "first-nonce", "GANGLINE_HITCH_ID": "first-id"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runTmux(t, binary, socket, "kill-session", "-t", "="+session) })
	runTmux(t, binary, socket, "wait-for", "first-ready")
	data, err := os.ReadFile(first)
	if err != nil || string(data) != "first-id\nfirst-nonce\nfirst-id\n" {
		t.Fatalf("initial capability=%q err=%v", data, err)
	}
	for _, key := range []string{"GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANGLINE_HITCH_ID"} {
		if got := strings.TrimSpace(runTmux(t, binary, socket, "show-environment", "-t", "="+session, key)); got != "-"+key {
			t.Fatalf("session retained %s: %q", key, got)
		}
	}
	_, err = b.Spawn(context.Background(), substrate.SpawnSpec{Name: "ordinary", Directory: root, Command: "sh", Args: []string{"-c", script, "sh", second, binary, socket, "second-ready"}})
	if err != nil {
		t.Fatal(err)
	}
	runTmux(t, binary, socket, "wait-for", "second-ready")
	data, err = os.ReadFile(second)
	if err != nil || string(data) != "unset\nunset\nunset\n" {
		t.Fatalf("ordinary pane inherited capability=%q err=%v", data, err)
	}
}
