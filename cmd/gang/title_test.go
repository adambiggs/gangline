package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func titleFixture(t *testing.T, collar string) *stateFixture {
	t.Helper()
	f := newStateFixture(t)
	dir := t.TempDir()
	f.env["GANG_COLLARS"] = dir
	if err := os.WriteFile(filepath.Join(dir, collar+".cue"), []byte(`collar: primitives: submit: params: settle: "0s"`), 0600); err != nil {
		t.Fatal(err)
	}
	f.add(t, "a", "worker", collar)
	prompt := "› "
	if collar == "claude" {
		prompt = "❯ "
	}
	f.input.command = collar
	paint := func(text string) substrate.Screen {
		if collar == "claude" {
			return screenWithText("────────", prompt+text, "────────")
		}
		return screenWithText("READY", prompt+text)
	}
	f.input.screen = paint("")
	f.input.onKeys = func(k substrate.Keys) error {
		if k.Text != "" {
			text := strings.TrimSuffix(strings.TrimPrefix(k.Text, "\x1b[200~"), "\x1b[201~")
			f.input.screen = paint(text)
		}
		return nil
	}
	f.input.submit = func(string) error { f.input.screen = paint(""); return nil }
	return f
}

func TestTitleSubmitsNativeCommand(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		t.Run(collar, func(t *testing.T) {
			f := titleFixture(t, collar)
			err := f.cmd.execute([]string{"title", "worker", "Résumé {{literal}}", "--team", "unit"})
			var ce commandError
			if !errors.As(err, &ce) || ce.status != exitUnknown || !strings.Contains(err.Error(), "persistence unconfirmed") {
				t.Fatalf("result = %v", err)
			}
			if f.input.submits != 1 || f.input.pasted != "/rename Résumé {{literal}}" {
				t.Fatalf("submits=%d text=%q", f.input.submits, f.input.pasted)
			}
		})
	}
}

func TestTitleSelfTarget(t *testing.T) {
	f := titleFixture(t, "codex")
	f.env["GANGLINE_HITCH_ID"] = "a"
	f.env["GANG_AGENT_ID"] = "a"
	f.env["TMUX_PANE"] = "%1"
	f.env["GANG_AGENT_NONCE"] = "fixture-nonce"
	err := f.cmd.title([]string{"Own title"})
	if err == nil || !strings.Contains(err.Error(), "command submitted") || f.input.submits != 1 {
		t.Fatalf("self title: %v submits=%d", err, f.input.submits)
	}
}

func TestTitleRejectsInvalidText(t *testing.T) {
	for _, args := range [][]string{nil, {"a", "b", "c"}, {""}, {"  "}, {"line\n/compact"}, {"tab\tname"}, {"bad\x1b"}, {"bad\r"}, {"bad\u202e"}, {"bad\xff"}, {"--unknown"}, {"bad name", "title"}} {
		if _, _, err := parseTitle(args); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
	name, text, err := parseTitle([]string{"--", "-title"})
	if err != nil || name != "" || text != "-title" {
		t.Fatalf("option terminator: %q %q %v", name, text, err)
	}
}

func TestTitleRefusesOccupiedNativeInput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		lines  []string
		reason string
	}{
		{"busy", []string{"Working (esc to interrupt)", "› "}, "native idle"},
		{"draft", []string{"READY", "› operator input"}, "unsent input"},
		{"blocked", []string{"Would you like to run this command?", "› 1. Yes, proceed"}, "native input choice"},
		{"unknown", []string{"unrecognized screen"}, "composer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := titleFixture(t, "codex")
			f.input.screen = screenWithText(tc.lines...)
			err := f.cmd.title([]string{"worker", "title"})
			if err == nil || !strings.Contains(err.Error(), tc.reason) || f.input.pasted != "" || f.input.submits != 0 {
				t.Fatalf("result=%v pasted=%q submits=%d", err, f.input.pasted, f.input.submits)
			}
		})
	}
}

func TestTitleRefusesOpenNativeTurn(t *testing.T) {
	f := titleFixture(t, "codex")
	p, _ := f.run.team.Agent("a")
	l, _ := p.TryLock()
	a, _ := p.Read()
	a.Native.SubmittedAt = f.cmd.now()
	a.Native.SessionID = "s"
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	err := f.cmd.title([]string{"worker", "title"})
	if err == nil || !strings.Contains(err.Error(), "native idle") || f.input.pasted != "" {
		t.Fatalf("result=%v pasted=%q", err, f.input.pasted)
	}
}

func TestTitleRefusesInactiveOrStaleInput(t *testing.T) {
	for _, kind := range []string{"inactive", "input", "compaction", "foreign"} {
		t.Run(kind, func(t *testing.T) {
			f := titleFixture(t, "codex")
			p, _ := f.run.team.Agent("a")
			l, _ := p.TryLock()
			a, _ := p.Read()
			switch kind {
			case "inactive":
				a.Status = core.Failed
			case "input":
				a.Input = &core.InputIntent{ID: "other", Kind: "envelope", At: f.cmd.now()}
			case "compaction":
				a.Compaction = &core.Compaction{ID: "compact", Status: "submitted", StartedAt: f.cmd.now(), Deadline: f.cmd.now().Add(time.Hour)}
			case "foreign":
				f.input.command = "sh"
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			l.Close()
			if err := f.cmd.title([]string{"worker", "title"}); err == nil || f.input.pasted != "" {
				t.Fatalf("result=%v pasted=%q", err, f.input.pasted)
			}
		})
	}
}

func TestTitleRechecksPastedCommand(t *testing.T) {
	for _, kind := range []string{"busy", "blocked", "edited", "collapsed", "tail"} {
		t.Run(kind, func(t *testing.T) {
			f := titleFixture(t, "codex")
			f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
				switch kind {
				case "busy":
					f.input.screen = screenWithText("Working (esc to interrupt)", "› /rename title")
				case "blocked":
					f.input.screen = screenWithText("Would you like to run this command?", "› 1. Yes, proceed")
				case "edited":
					f.input.screen = screenWithText("READY", "› operator edit")
				case "tail":
					f.input.screen = screenWithText("READY", "› /rename title", "  /compact")
				case "collapsed":
					f.input.screen = screenWithText("READY", "› [Pasted Content 5000 chars]")
				}
				return nil
			}
			if err := f.cmd.title([]string{"worker", "title"}); err == nil || f.input.submits != 0 {
				t.Fatalf("result=%v submits=%d", err, f.input.submits)
			}
		})
	}
}

func TestTitleNativeRefusal(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		t.Run(map[bool]string{false: "historical", true: "fresh"}[fresh], func(t *testing.T) {
			f := titleFixture(t, "codex")
			dir := t.TempDir()
			f.env["GANG_COLLARS"] = dir
			if err := os.WriteFile(filepath.Join(dir, "codex.cue"), []byte(`collar: {actions: title: refusal: "(?m)^failed to set thread name:.*$", primitives: submit: params: settle: "0s"}`), 0600); err != nil {
				t.Fatal(err)
			}
			f.input.screen = screenWithText("failed to set thread name: old", "› ")
			f.input.submit = func(string) error {
				lines := []string{"failed to set thread name: old"}
				if fresh {
					lines = append(lines, "failed to set thread name: test refusal")
				}
				f.input.screen = screenWithText(append(lines, "› ")...)
				return nil
			}
			err := f.cmd.title([]string{"worker", "title"})
			var ce commandError
			if !errors.As(err, &ce) {
				t.Fatal(err)
			}
			want := exitUnknown
			if fresh {
				want = exitNative
			}
			if ce.status != want {
				t.Fatalf("result = %v", err)
			}
		})
	}
}

// titleResultCapture closes the operation's context at the empty-composer event.
// Waiting for pasted text after that event must fail without a clock wait.
type titleResultCapture struct {
	harnessInput
	input  *inputFixture
	cancel context.CancelFunc
}

func (b titleResultCapture) Capture(_ context.Context, pane substrate.PaneID) (substrate.Screen, error) {
	screen, err := b.harnessInput.Capture(context.Background(), pane)
	if b.input.submits > 0 {
		b.cancel()
	}
	return screen, err
}

func TestTitleWaitsForEmptyComposerAfterSubmit(t *testing.T) {
	f := titleFixture(t, "codex")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.cmd.newTimeout = func(context.Context, time.Duration) (context.Context, context.CancelFunc) { return ctx, cancel }
	f.cmd.inputBackend = titleResultCapture{harnessInput: f.input, input: f.input, cancel: cancel}
	f.cmd.settleInput = nil
	err := f.cmd.title([]string{"worker", "title"})
	if err == nil || err.Error() != "title command submitted; native title persistence unconfirmed" || f.input.submits != 1 {
		t.Fatalf("result=%v submits=%d", err, f.input.submits)
	}
}

func TestTitleCommandPreservesOptionTerminator(t *testing.T) {
	f := titleFixture(t, "codex")
	err := f.cmd.execute([]string{"title", "worker", "--", "--dash-title"})
	if err == nil || err.Error() != "title command submitted; native title persistence unconfirmed" || f.input.submits != 1 || f.input.pasted != "/rename --dash-title" {
		t.Fatalf("result=%v submits=%d text=%q", err, f.input.submits, f.input.pasted)
	}
}
