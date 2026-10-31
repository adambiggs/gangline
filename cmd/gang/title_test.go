//go:build acceptance

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
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

func TestNativeSessionTitle(t *testing.T) {
	for _, tc := range []struct{ task, want string }{
		{"", "worker@unit"},
		{"Fix the parser\nDetails to omit", "worker@unit: Fix the parser"},
		{"  Résumé\t{{literal}}  \nDetails", "worker@unit: Résumé {{literal}}"},
		{strings.Repeat("界", 61), "worker@unit: " + strings.Repeat("界", 59) + "…"},
	} {
		if got := nativeSessionTitle("worker", "unit", tc.task); got != tc.want {
			t.Errorf("title(%q)=%q want %q", tc.task, got, tc.want)
		}
	}
}

func autoTitleFixture(t *testing.T, collar string, freshHook bool) *stateFixture {
	t.Helper()
	f := titleFixture(t, collar)
	p, _ := f.run.team.Agent("a")
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := p.Read()
	a.NativeTitle = &core.NativeTitle{Text: nativeSessionTitle(a.Name, "unit", "Fix the parser\nDetails")}
	a.Native.SubmittedAt = f.cmd.now().Add(-time.Second)
	a.Native.FinishedAt = f.cmd.now()
	if freshHook {
		a.Native.HookAt, a.Native.HookKind = f.cmd.now(), "turn-finished"
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	return f
}

func TestAutoTitleAtFirstIdleTick(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		for _, freshHook := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/hooks=%t", collar, freshHook), func(t *testing.T) {
				f := autoTitleFixture(t, collar, freshHook)
				for range 2 {
					// A second runtime reads the saved marker, as a later tick does.
					run, err := f.cmd.runtime()
					if err != nil {
						t.Fatal(err)
					}
					if err := run.tickAgent("a", hookNotice{}, false); err != nil {
						t.Fatal(err)
					}
				}
				if got := f.agent(t, "a"); f.input.submits != 1 || f.input.pasted != "/rename worker@unit: Fix the parser" || !got.NativeTitle.Attempted {
					t.Fatalf("submits=%d pasted=%q title=%+v", f.input.submits, f.input.pasted, got.NativeTitle)
				}
				log, err := os.ReadFile(f.run.team.Log)
				if err != nil || !strings.Contains(string(log), "automatic title: title command submitted; native title persistence unconfirmed") {
					t.Fatalf("unconfirmed title not logged: %s, %v", log, err)
				}
			})
		}
	}
}

func TestAutoTitleDefersUntilInputIsFree(t *testing.T) {
	for _, kind := range []string{"startup", "open turn", "busy", "draft", "tail", "permission", "compaction", "input"} {
		t.Run(kind, func(t *testing.T) {
			f := autoTitleFixture(t, "codex", true)
			ready := f.input.screen
			p, _ := f.run.team.Agent("a")
			l, _ := p.TryLock()
			a, _ := p.Read()
			switch kind {
			case "startup":
				a.Native.SubmittedAt, a.Native.FinishedAt = time.Time{}, time.Time{}
			case "open turn":
				a.Native.SubmittedAt = a.Native.FinishedAt.Add(time.Second)
			case "busy":
				f.input.screen = screenWithText("Working (esc to interrupt)", "› ")
			case "draft":
				f.input.screen = screenWithText("READY", "› operator input")
			case "tail":
				f.input.screen = screenWithText("READY", "› ", "  operator input")
			case "permission":
				f.input.screen = screenWithText("Would you like to run this command?", "› 1. Yes, proceed")
			case "compaction":
				a.Compaction = &core.Compaction{Status: "queued"}
			case "input":
				a.Input = &core.InputIntent{ID: "other", Kind: "envelope", At: f.cmd.now()}
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			c, err := loadCollar(a.Collar, f.run.settings)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.run.autoTitle(l, &a, c); err != nil || f.input.pasted != "" || a.NativeTitle.Attempted {
				t.Fatalf("deferred title: %v, pasted=%q title=%+v", err, f.input.pasted, a.NativeTitle)
			}
			f.input.screen = ready
			a.Activity, a.Input, a.Compaction = core.Idle, nil, nil
			a.Native.SubmittedAt, a.Native.FinishedAt = f.cmd.now().Add(-time.Second), f.cmd.now()
			if err := f.run.autoTitle(l, &a, c); err != nil || f.input.submits != 1 || !a.NativeTitle.Attempted {
				t.Fatalf("ready title: %v, submits=%d title=%+v", err, f.input.submits, a.NativeTitle)
			}
			l.Close()
		})
	}
}

func TestAutoTitleFailureIsLoggedWithoutReplay(t *testing.T) {
	for _, kind := range []string{"paste", "submit", "refusal"} {
		t.Run(kind, func(t *testing.T) {
			f := autoTitleFixture(t, "codex", true)
			switch kind {
			case "paste":
				f.input.onKeys = func(substrate.Keys) error { return errors.New("paste failed") }
			case "submit":
				f.input.submit = func(string) error { return errors.New("submit failed") }
			case "refusal":
				if err := os.WriteFile(filepath.Join(f.env["GANG_COLLARS"], "codex.cue"), []byte(`collar: {actions: title: refusal: "(?m)^failed to set thread name:.*$", primitives: submit: params: settle: "0s"}`), 0600); err != nil {
					t.Fatal(err)
				}
				f.input.submit = func(string) error {
					f.input.screen = screenWithText("failed to set thread name: native refusal", "› ")
					return nil
				}
			}
			for range 2 {
				if err := f.run.tickAgent("a", hookNotice{}, false); err != nil {
					t.Fatalf("title failure escaped tick: %v", err)
				}
			}
			log, err := os.ReadFile(f.run.team.Log)
			if err != nil || strings.Count(string(log), "automatic title:") != 1 || !f.agent(t, "a").NativeTitle.Attempted {
				t.Fatalf("failure not retained once: %s, %v", log, err)
			}
		})
	}
}

func TestAutoTitleWithdrawsUnsubmittedCommandAndDrainsQueue(t *testing.T) {
	for _, failure := range []string{"withheld", "unconsumed"} {
		t.Run(failure, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := autoTitleFixture(t, "codex", true)
				f.cmd.settleInput = nil
				p, _ := f.run.team.Agent("a")
				envelope := core.Envelope{ID: "followup", Token: "0123456789abcdef", Recipient: "a", To: "worker", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "continue the work"}, CreatedAt: f.cmd.now()}
				if err := p.Publish(envelope); err != nil {
					t.Fatal(err)
				}
				var titles, clears int
				f.input.onKeys = func(keys substrate.Keys) error {
					if keys.Text != "" {
						text := strings.TrimSuffix(strings.TrimPrefix(keys.Text, "\x1b[200~"), "\x1b[201~")
						f.input.screen = screenWithText("READY", "› "+text)
						if strings.HasPrefix(text, "/rename ") {
							titles++
							if failure == "withheld" {
								f.input.screen = screenWithText("Working (esc to interrupt)", "› "+text)
							}
						}
					}
					if strings.Join(keys.Names, " ") == "C-u" {
						clears++
						f.input.screen = screenWithText("READY", "› ")
					}
					return nil
				}
				f.input.submit = func(text string) error {
					if strings.HasPrefix(text, "/rename ") {
						return nil
					}
					f.input.screen = screenWithText("READY", "› ")
					return p.WriteWitness(store.Witness{ID: "followup-submit", At: f.cmd.now(), Prompt: text, SessionID: "s"})
				}
				f.run.cmd = f.cmd
				started := time.Now()
				for range 2 {
					if err := f.run.tickAgent("a", hookNotice{}, false); err != nil {
						t.Fatal(err)
					}
				}
				collar, err := loadCollar("codex", f.run.settings)
				if err != nil {
					t.Fatal(err)
				}
				composer, err := harness.ReadComposer(collar.Primitives.Composer, f.input.screen)
				if err != nil || composer.Text != "" || composer.TailOccupied || clears != 1 {
					t.Fatalf("stranded automatic title: composer=%+v clears=%d err=%v", composer, clears, err)
				}
				if titles != 1 || !f.agent(t, "a").NativeTitle.Attempted {
					t.Fatalf("automatic title replayed: pastes=%d title=%+v", titles, f.agent(t, "a").NativeTitle)
				}
				receipt, err := p.ReadEnvelope("cur", envelope.ID)
				if err != nil || receipt.Outcome != "delivered" {
					t.Fatalf("queue remained blocked: receipt=%+v err=%v", receipt, err)
				}
				log, err := os.ReadFile(f.run.team.Log)
				if err != nil || strings.Count(string(log), "automatic title:") != 1 || !strings.Contains(string(log), "composer clear of staged title command") {
					t.Fatalf("title outcome missing: %s, %v", log, err)
				}
				elapsed := time.Since(started)
				if failure == "unconsumed" {
					recoveryElapsed := elapsed - operationTimeout
					if recoveryElapsed < 0 || recoveryElapsed >= compactAbortTimeout {
						t.Fatalf("unconsumed submit fake elapsed=%s recovery=%s cleanup budget=%s", elapsed, recoveryElapsed, compactAbortTimeout)
					}
					t.Logf("fake elapsed=%s; recovery and queue delivery=%s; cleanup budget margin=%s", elapsed, recoveryElapsed, compactAbortTimeout-recoveryElapsed)
				}
			})
		})
	}
}

func TestAutoTitleCleanupPreservesChangedComposer(t *testing.T) {
	for _, changed := range []string{"edited", "whitespace", "newline", "tail", "blocked", "clear unconsumed"} {
		t.Run(changed, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := autoTitleFixture(t, "codex", true)
				f.input.submit = func(text string) error {
					switch changed {
					case "edited":
						f.input.screen = screenWithText("READY", "› operator edit")
					case "whitespace":
						f.input.screen = screenWithText("READY", "› "+strings.ReplaceAll(text, " ", "  "))
					case "newline":
						f.input.screen = screenWithText("READY", "› "+text[:20], "  "+text[20:])
						f.input.screen.Cursor = substrate.Cursor{Row: 2, Column: 5, Visible: true}
					case "tail":
						f.input.screen = screenWithText("READY", "› "+text, "  operator edit")
					case "blocked":
						f.input.screen = screenWithText("Would you like to run this command?", "› 1. Yes, proceed")
					}
					return errors.New("submit failed")
				}
				f.run.cmd = f.cmd
				for range 2 {
					if err := f.run.tickAgent("a", hookNotice{}, false); err != nil {
						t.Fatal(err)
					}
				}
				wantKeys := ""
				if changed == "clear unconsumed" {
					wantKeys = "C-u"
				}
				if got := strings.Join(f.input.keys, " "); got != wantKeys || f.input.submits != 1 {
					t.Fatalf("input overwritten or title replayed: keys=%q submits=%d", got, f.input.submits)
				}
				log, err := os.ReadFile(f.run.team.Log)
				if err != nil || !strings.Contains(string(log), "title cleanup unconfirmed") || strings.Count(string(log), "automatic title:") != 1 {
					t.Fatalf("cleanup uncertainty not logged: %s, %v", log, err)
				}
			})
		})
	}
}

func TestManualTitleSupersedesAutomaticTitle(t *testing.T) {
	f := autoTitleFixture(t, "codex", true)
	if err := f.cmd.title([]string{"worker", "Operator title"}); err == nil || !strings.Contains(err.Error(), "persistence unconfirmed") {
		t.Fatal(err)
	}
	if err := f.run.tickAgent("a", hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, "a"); f.input.submits != 1 || got.NativeTitle.Text != "Operator title" || !got.NativeTitle.Attempted {
		t.Fatalf("manual title overwritten: submits=%d title=%+v", f.input.submits, got.NativeTitle)
	}
}
