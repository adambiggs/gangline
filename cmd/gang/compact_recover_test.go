package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

type recoverScreens struct {
	idle, busy, blocked, draft, restored, unknown substrate.Screen
}

func compactInterruptScreens(t *testing.T, f *stateFixture, collar string) recoverScreens {
	t.Helper()
	s := recoverScreens{
		idle:     screenWithText("READY", "› "),
		busy:     screenWithText("• Compacting context (12s • esc to interrupt)", "› "),
		blocked:  screenWithText("Would you like to run the following command?", "› 1. Yes, proceed (y)", "  2. No, and tell Codex what to do differently (esc)"),
		draft:    screenWithText("• Compacting context (12s • esc to interrupt)", "› half-typed operator note"),
		restored: screenWithText("READY", "› continue the work"),
		unknown:  screenWithText("loading"),
	}
	if collar == "claude" {
		f.input.command = "claude"
		s = recoverScreens{
			idle:     screenWithText("────────", "❯ ", "────────"),
			busy:     screenWithText("✻ Compacting conversation… (esc to interrupt)", "────────", "❯ ", "────────"),
			blocked:  screenWithText("Bash command", "Do you want to proceed?", "❯ 1. Yes", "  2. No, and tell Claude what to do differently (esc)"),
			draft:    screenWithText("✻ Compacting conversation… (esc to interrupt)", "────────", "❯ half-typed operator note", "────────"),
			restored: screenWithText("────────", "❯ continue the work", "────────"),
			unknown:  screenWithText("loading"),
		}
	}
	// The fixture screens must classify as the surfaces they stand for, or
	// every assertion below could pass against a world the collar never sees.
	c, err := loadCollar(collar, f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	if idle, err := harness.Idle(c, s.idle); err != nil || !idle {
		t.Fatalf("idle screen: idle=%v err=%v", idle, err)
	}
	if busy, err := harness.Busy(c, s.busy); err != nil || !busy {
		t.Fatalf("busy screen: busy=%v err=%v", busy, err)
	}
	if composer, err := harness.ReadComposer(c.Primitives.Composer, s.busy); err != nil || composer.Text != "" {
		t.Fatalf("busy screen composer: %+v err=%v", composer, err)
	}
	if _, blocked, err := harness.InputBlocked(c, s.blocked); err != nil || !blocked {
		t.Fatalf("blocked screen: blocked=%v err=%v", blocked, err)
	}
	if composer, err := harness.ReadComposer(c.Primitives.Composer, s.draft); err != nil || composer.Text == "" {
		t.Fatalf("draft screen composer: %+v err=%v", composer, err)
	}
	if composer, err := harness.ReadComposer(c.Primitives.Composer, s.restored); err != nil || composer.Text == "" {
		t.Fatalf("restored screen composer: %+v err=%v", composer, err)
	}
	if busy, err := harness.Busy(c, s.restored); err != nil || busy {
		t.Fatalf("restored screen: busy=%v err=%v", busy, err)
	}
	if _, err := harness.ReadComposer(c.Primitives.Composer, s.unknown); err == nil {
		t.Fatal("unknown screen has a readable composer")
	}
	return s
}

func saveRecoverCompaction(t *testing.T, f *stateFixture, a core.Agent, status string, setup ...func(*core.Agent)) core.Agent {
	t.Helper()
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Compaction = &core.Compaction{ID: "c", Resume: core.Message{Text: "continue"}, ResumeFrom: core.Sender{Kind: core.SenderGangline, Name: "compact"}, StartedAt: f.cmd.now().Add(-time.Hour), Deadline: f.cmd.now().Add(time.Minute), Status: status, Continuation: true}
	if status == "failed" {
		a.Compaction.Reason = "native compaction refused"
	}
	for _, edit := range setup {
		edit(&a)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestCompactInterruptRefusesBeforeAnyKey(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		for _, tc := range []struct {
			name, status, surface, want string
			setup                       func(*core.Agent)
			foreground                  string
			settled                     bool
		}{
			{name: "failed-record-on-approval", status: "failed", surface: "blocked", want: "already failed"},
			{name: "completed-record", status: "completed", surface: "busy", want: "already completed"},
			{name: "queued-record", status: "queued", surface: "busy", want: "not submitted"},
			{name: "submitted-on-approval", status: "submitted", surface: "blocked", want: "native input blocked"},
			{name: "unverified-on-approval", status: "unverified", surface: "blocked", want: "native input blocked"},
			{name: "submitted-on-draft", status: "submitted", surface: "draft", want: "unsubmitted input"},
			{name: "submitted-on-unknown", status: "submitted", surface: "unknown", want: "unrecognized"},
			{name: "unverified-on-idle", status: "unverified", surface: "idle", want: "idle"},
			// An admitted continuation means the busy pane is resumed work.
			{name: "admitted-continuation-over-busy-turn", status: "unverified", surface: "busy", want: "already admitted", setup: func(a *core.Agent) { a.Compaction.ResumeAdmitted = true }},
			// Acquiring the agent settles a witnessed completion first.
			{name: "completion-witnessed", status: "unverified", surface: "busy", want: "already completed", settled: true, setup: func(a *core.Agent) { a.Compaction.CompletedAt = a.Compaction.StartedAt.Add(time.Minute) }},
			{name: "never-queued-continuation", status: "unverified", surface: "busy", want: "never queued", setup: func(a *core.Agent) { a.Compaction.Continuation = false }},
			{name: "already-recovered", status: "unverified", surface: "busy", want: "already recovered", setup: func(a *core.Agent) { a.Compaction.Recovered = true }},
			{name: "incomplete-registration", status: "submitted", surface: "busy", want: "incomplete pane registration", setup: func(a *core.Agent) { a.Registration.TokenHash = "" }},
			{name: "inactive-agent", status: "submitted", surface: "busy", want: "not active", setup: func(a *core.Agent) { a.Status = core.Dropping }},
			{name: "harness-not-foreground", status: "submitted", surface: "busy", want: "not in the pane foreground", foreground: "bash"},
		} {
			t.Run(collar+"/"+tc.name, func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", collar)
				screens := compactInterruptScreens(t, f, collar)
				f.input.screen = map[string]substrate.Screen{"idle": screens.idle, "busy": screens.busy, "blocked": screens.blocked, "draft": screens.draft, "unknown": screens.unknown}[tc.surface]
				var setup []func(*core.Agent)
				if tc.setup != nil {
					setup = append(setup, tc.setup)
				}
				before := saveRecoverCompaction(t, f, a, tc.status, setup...)
				f.input.tmuxCommand = tc.foreground
				err := f.cmd.compact([]string{"worker", "--interrupt"})
				var ce commandError
				if !errors.As(err, &ce) || ce.status != exitRefused || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("recover: %v, want refusal containing %q", err, tc.want)
				}
				if f.input.captures > 1 {
					t.Fatalf("captures=%d before refusal, want at most the one classifying capture", f.input.captures)
				}
				if len(f.input.keys) != 0 || f.input.submits != 0 || f.input.pasted != "" {
					t.Fatalf("keys sent before refusal: keys=%v submits=%d pasted=%q", f.input.keys, f.input.submits, f.input.pasted)
				}
				p, _ := f.run.team.Agent(a.ID)
				after, err := p.Read()
				if err != nil {
					t.Fatal(err)
				}
				if tc.settled {
					if after.Compaction.Status != "completed" || after.Input != nil {
						t.Fatalf("witnessed completion not settled: %+v", after.Compaction)
					}
					return
				}
				if !reflect.DeepEqual(after.Compaction, before.Compaction) || after.Input != nil || after.Activity != before.Activity || after.Evidence != before.Evidence {
					t.Fatalf("refusal mutated state:\nbefore %+v %+v\nafter  %+v %+v", before.Compaction, before.Activity, after.Compaction, after.Activity)
				}
			})
		}
	}
}

func TestCompactInterruptSendsOnlyEscapeAndReclassifies(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		for _, status := range []string{"submitted", "unverified"} {
			t.Run(collar+"/"+status, func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", collar)
				screens := compactInterruptScreens(t, f, collar)
				f.input.screen = screens.busy
				saveRecoverCompaction(t, f, a, status)
				p, _ := f.run.team.Agent(a.ID)
				f.input.onKeys = func(k substrate.Keys) error {
					got, err := p.Read()
					if err != nil {
						return err
					}
					if got.Input == nil || got.Input.ID != "c" {
						t.Fatalf("key %v sent without a durable intent: %+v", k.Names, got)
					}
					f.input.screen = screens.idle
					return nil
				}
				if err := f.cmd.compact([]string{"worker", "--interrupt"}); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(f.input.keys, []string{"Escape"}) || f.input.submits != 0 || f.input.captures != 2 {
					t.Fatalf("keys=%v submits=%d captures=%d, want Escape only between two captures", f.input.keys, f.input.submits, f.input.captures)
				}
				got, err := p.Read()
				if err != nil {
					t.Fatal(err)
				}
				if got.Compaction.Status != "unverified" || !strings.Contains(got.Compaction.Reason, "Escape") || got.Input != nil || got.Activity != core.Idle {
					t.Fatalf("after recovery: compaction=%+v input=%+v activity=%s", got.Compaction, got.Input, got.Activity)
				}
			})
		}
	}
}

func TestCompactInterruptReportsSurfaceAfterEscape(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		for _, tc := range []struct {
			name, surface, want string
			activity            core.Activity
		}{
			{"still-busy", "busy", "still active", core.Busy},
			{"approval", "blocked", "native input blocked", core.Blocked},
			{"restored-input", "restored", "unsubmitted input", core.Blocked},
			{"unrecognized", "unknown", "unrecognized", core.Unknown},
		} {
			t.Run(collar+"/"+tc.name, func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", collar)
				screens := compactInterruptScreens(t, f, collar)
				f.input.screen = screens.busy
				saveRecoverCompaction(t, f, a, "submitted")
				f.input.onKeys = func(substrate.Keys) error {
					f.input.screen = map[string]substrate.Screen{"busy": screens.busy, "blocked": screens.blocked, "restored": screens.restored, "unknown": screens.unknown}[tc.surface]
					return nil
				}
				// An expired settle window makes the first capture after Escape final.
				f.cmd.newTimeout = func(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
					if d == compactRecoverSettle {
						expired, cancel := context.WithCancel(ctx)
						cancel()
						return expired, cancel
					}
					return context.WithTimeout(ctx, d)
				}
				err := f.cmd.compact([]string{"worker", "--interrupt"})
				var ce commandError
				if !errors.As(err, &ce) || ce.status != exitNative || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "Escape") {
					t.Fatalf("recover: %v, want native error containing %q", err, tc.want)
				}
				if !reflect.DeepEqual(f.input.keys, []string{"Escape"}) || f.input.submits != 0 {
					t.Fatalf("keys=%v submits=%d, want Escape only", f.input.keys, f.input.submits)
				}
				p, _ := f.run.team.Agent(a.ID)
				got, err := p.Read()
				if err != nil {
					t.Fatal(err)
				}
				wantActivity := tc.activity
				// A visible compaction used to count only as busy; its dedicated
				// active pattern now preserves the compaction activity.
				if collar == "codex" && tc.surface == "busy" {
					wantActivity = core.Compacting
				}
				if got.Compaction.Status != "unverified" || !strings.Contains(got.Compaction.Reason, "Escape") || got.Input != nil || got.Activity != wantActivity {
					t.Fatalf("after recovery: compaction=%+v input=%+v activity=%s", got.Compaction, got.Input, got.Activity)
				}
			})
		}
	}
}

func TestCompactInterruptKeyFailureLeavesReceipt(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			screens := compactInterruptScreens(t, f, collar)
			f.input.screen = screens.busy
			saveRecoverCompaction(t, f, a, "submitted")
			f.input.onKeys = func(substrate.Keys) error { return errors.New("pane write failed") }
			err := f.cmd.compact([]string{"worker", "--interrupt"})
			var ce commandError
			if !errors.As(err, &ce) || ce.status != exitUnknown || !strings.Contains(err.Error(), "pane write failed") {
				t.Fatalf("recover: %v", err)
			}
			p, _ := f.run.team.Agent(a.ID)
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Compaction.Status != "unverified" || !strings.Contains(got.Compaction.Reason, "sent Escape; outcome unknown: pane write failed") || got.Input != nil || got.Activity != core.Unknown {
				t.Fatalf("after failed key: compaction=%+v input=%+v", got.Compaction, got.Input)
			}
		})
	}
}

func TestCompactInterruptStopsWhenThePaneLeavesBusy(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			screens := compactInterruptScreens(t, f, collar)
			f.input.screen = screens.busy
			saveRecoverCompaction(t, f, a, "submitted")
			f.input.onKeys = func(substrate.Keys) error {
				f.input.screen = screens.blocked
				return nil
			}
			c, err := loadCollar(collar, f.run.settings)
			if err != nil {
				t.Fatal(err)
			}
			// A collar overlay may declare several recovery keys; none may
			// land on the approval the first one exposed.
			c.Actions.CompactRecover = append(c.Actions.CompactRecover, c.Actions.CompactRecover...)
			l, got, err := f.run.acquire(a.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			b, err := f.run.input()
			if err != nil {
				t.Fatal(err)
			}
			err = f.run.interruptCompaction(l, &got, f.run.registeredInput(got, b), c)
			if releaseErr := f.run.release(l); releaseErr != nil {
				t.Fatal(releaseErr)
			}
			var ce commandError
			if !errors.As(err, &ce) || ce.status != exitNative || !strings.Contains(err.Error(), "native input blocked") {
				t.Fatalf("recover: %v", err)
			}
			if !reflect.DeepEqual(f.input.keys, []string{"Escape"}) {
				t.Fatalf("keys=%v, want the first recovery key only", f.input.keys)
			}
		})
	}
}

func TestCompactInterruptRunsOncePerCompaction(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		for _, crashed := range []bool{false, true} {
			t.Run(collar+"/"+map[bool]string{false: "recovered", true: "recovery-owner-exited"}[crashed], func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", collar)
				screens := compactInterruptScreens(t, f, collar)
				f.input.screen = screens.busy
				saveRecoverCompaction(t, f, a, "unverified", func(a *core.Agent) {
					if crashed {
						a.Input = &core.InputIntent{ID: "c", Kind: "compaction-recovery", At: a.Compaction.StartedAt}
					}
				})
				if !crashed {
					f.input.onKeys = func(substrate.Keys) error {
						f.input.screen = screens.idle
						return nil
					}
					if err := f.cmd.compact([]string{"worker", "--interrupt"}); err != nil {
						t.Fatal(err)
					}
					// The agent resumes ordinary work under the same record.
					f.input.screen, f.input.keys = screens.busy, nil
				}
				err := f.cmd.compact([]string{"worker", "--interrupt"})
				var ce commandError
				if !errors.As(err, &ce) || ce.status != exitRefused || !strings.Contains(err.Error(), "already recovered") {
					t.Fatalf("second recovery: %v", err)
				}
				if len(f.input.keys) != 0 {
					t.Fatalf("second recovery sent %v", f.input.keys)
				}
			})
		}
	}
}

func TestCompactInterruptUnknownAfterKey(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		for _, tc := range []struct {
			name, want string
			status     int
			after      func(*stateFixture)
		}{
			{"capture-fails", "outcome unknown: capture failed", exitUnknown, func(f *stateFixture) { f.input.captureErr = errors.New("capture failed") }},
			{"harness-leaves-foreground", "stopped before the next key", exitNative, func(f *stateFixture) { f.input.tmuxCommand = "bash" }},
		} {
			t.Run(collar+"/"+tc.name, func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", collar)
				screens := compactInterruptScreens(t, f, collar)
				f.input.screen = screens.busy
				saveRecoverCompaction(t, f, a, "submitted")
				f.input.onKeys = func(substrate.Keys) error {
					tc.after(f)
					return nil
				}
				f.run.cmd.newTimeout = func(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
					if d == compactRecoverSettle {
						expired, cancel := context.WithCancel(ctx)
						cancel()
						return expired, cancel
					}
					return context.WithTimeout(ctx, d)
				}
				c, err := loadCollar(collar, f.run.settings)
				if err != nil {
					t.Fatal(err)
				}
				c.Actions.CompactRecover = append(c.Actions.CompactRecover, c.Actions.CompactRecover...)
				l, got, err := f.run.acquire(a.ID, false)
				if err != nil {
					t.Fatal(err)
				}
				b, err := f.run.input()
				if err != nil {
					t.Fatal(err)
				}
				err = f.run.interruptCompaction(l, &got, f.run.registeredInput(got, b), c)
				if releaseErr := f.run.release(l); releaseErr != nil {
					t.Fatal(releaseErr)
				}
				var ce commandError
				if !errors.As(err, &ce) || ce.status != tc.status || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("recover: %v, want status %d containing %q", err, tc.status, tc.want)
				}
				if !reflect.DeepEqual(f.input.keys, []string{"Escape"}) {
					t.Fatalf("keys=%v, want the first recovery key only", f.input.keys)
				}
				p, _ := f.run.team.Agent(a.ID)
				saved, err := p.Read()
				if err != nil {
					t.Fatal(err)
				}
				if saved.Compaction.Status != "unverified" || !saved.Compaction.Recovered || saved.Input != nil || strings.Count(saved.Compaction.Reason, "recorded unverified") != 0 {
					t.Fatalf("after recovery: %+v input=%+v", saved.Compaction, saved.Input)
				}
			})
		}
	}
}
