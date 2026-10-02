package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

// Pasted compact input does not submit itself, so any failure before the
// compact Enter leaves a compaction that never ran: it fails, tells the agent,
// and leaves no draft in the composer.
func TestCompactionFailureBeforeEnterIsNotRun(t *testing.T) {
	empty := screenWithText("────────", "❯ ", "────────")
	draft := screenWithText("────────", "❯ /compact Resume from the state file.", "────────")
	trust := screenWithText("Quick safety check: Is this a project you created or one you trust?", "❯ 1. Yes, I trust this folder", "  2. No, exit")
	for _, tc := range []struct {
		name       string
		settle     error
		after      substrate.Screen
		wantReason string
		wantClear  bool
	}{
		{name: "composer emptied", settle: harness.ErrComposerEmptied, after: empty, wantReason: "emptied before submission"},
		{name: "composer never settled", settle: fmt.Errorf("native composer did not settle before submission: %w", context.DeadlineExceeded), after: draft, wantReason: "composer cleared", wantClear: true},
		{name: "input blocked", after: trust, wantReason: "remain in the composer behind the prompt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "claude")
			p, _ := f.run.team.Agent(a.ID)
			f.env["GANG_AGENT_ID"] = string(a.ID)
			f.env["TMUX_PANE"] = a.Pane
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.input.command = "claude"
			f.input.screen = empty
			f.input.onKeys = func(k substrate.Keys) error {
				if strings.Contains(k.Text, "/compact ") {
					f.input.screen = tc.after
				}
				if slices.Contains(k.Names, "C-u") {
					f.input.screen = empty
				}
				return nil
			}
			// The paste's settle fails; the clear's own wait succeeds.
			settle := tc.settle
			f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
				err := settle
				settle = nil
				return err
			}
			f.run.cmd = f.cmd
			err := f.cmd.compact([]string{"--resume", "Resume from the state file."})
			if ce := (commandError{}); !errors.As(err, &ce) || ce.status != exitNative || !strings.Contains(err.Error(), tc.wantReason) {
				t.Fatalf("compact result: %v", err)
			}
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Compaction.Status != "failed" || got.Input != nil || got.Compaction.Continuation {
				t.Fatalf("compaction state: %+v input=%+v", got.Compaction, got.Input)
			}
			if cleared := slices.Contains(f.input.keys, "C-u"); cleared != tc.wantClear {
				t.Fatalf("clear keys sent=%v, want %v", cleared, tc.wantClear)
			}
			if f.input.submits != 0 {
				t.Fatalf("Enter reached the pane %d times", f.input.submits)
			}
			if notice := failureNotice(t, p, "new", got.Compaction.ID); !strings.Contains(notice.Message.Text, "was not compacted") {
				t.Fatalf("notice: %q", notice.Message.Text)
			}
		})
	}
}

// A compaction queued for another agent fails in a later tick, after the
// requesting command has exited, so the requester learns of it from its queue.
func TestQueuedCompactionFailureReachesRequester(t *testing.T) {
	f := newStateFixture(t)
	tester := f.add(t, "a", "tester", "claude")
	lead := f.add(t, "b", "lead", "claude")
	f.env["GANG_AGENT_ID"] = string(lead.ID)
	f.env["TMUX_PANE"] = lead.Pane
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	f.input.command = "claude"
	f.input.screen = screenWithText("✻ Working… (esc to interrupt)", "────────", "❯ ", "────────")
	var ticked []string
	f.cmd.detach = func(id string, _ hookNotice) error { ticked = append(ticked, id); return nil }
	f.run.cmd = f.cmd
	if err := f.cmd.compact([]string{"tester", "--resume", "Resume from the state file."}); err != nil {
		t.Fatal(err)
	}
	f.input.screen = screenWithText("────────", "❯ ", "────────")
	f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
		return harness.ErrComposerEmptied
	}
	f.run.cmd = f.cmd
	if err := f.run.tickAgent(tester.ID, hookNotice{Kind: "turn-finished", SessionID: "s", At: f.cmd.now()}, false); err != nil {
		t.Fatal(err)
	}
	tp, _ := f.run.team.Agent(tester.ID)
	got, err := tp.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Compaction.Status != "failed" {
		t.Fatalf("compaction: %+v", got.Compaction)
	}
	lp, _ := f.run.team.Agent(lead.ID)
	notice := failureNotice(t, lp, "new", got.Compaction.ID)
	if notice.Recipient != lead.ID || !strings.Contains(notice.Message.Text, "tester") || !strings.Contains(notice.Message.Text, "emptied before submission") {
		t.Fatalf("requester notice: %+v", notice)
	}
	if !slices.Contains(ticked, string(lead.ID)) {
		t.Fatalf("no tick started for the requester: %v", ticked)
	}
}

// A failure the requesting command reports itself is not repeated in the
// requester's queue.
func TestImmediateCompactionFailureNotRequeuedToRequester(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "tester", "claude")
	lead := f.add(t, "b", "lead", "claude")
	f.env["GANG_AGENT_ID"] = string(lead.ID)
	f.env["TMUX_PANE"] = lead.Pane
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	f.input.command = "claude"
	f.input.screen = screenWithText("────────", "❯ ", "────────")
	f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
		return harness.ErrComposerEmptied
	}
	f.run.cmd = f.cmd
	if err := f.cmd.compact([]string{"tester", "--resume", "Resume from the state file."}); !strings.Contains(fmt.Sprint(err), "emptied before submission") {
		t.Fatalf("compact result: %v", err)
	}
	lp, _ := f.run.team.Agent(lead.ID)
	if queued, err := lp.ListNew(); err != nil || len(queued) != 0 {
		t.Fatalf("requester queue: %+v, %v", queued, err)
	}
}

// The requester is recorded only when the caller's identity is observable; a
// compaction without a resume note does not need it.
func TestCompactWithoutResumeToleratesUnobservedCaller(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "tester", "claude")
	lead := f.add(t, "b", "lead", "claude")
	f.env["TMUX_PANE"] = lead.Pane
	f.input.command = "claude"
	f.input.screen = screenWithText("✻ Working… (esc to interrupt)", "────────", "❯ ", "────────")
	f.run.cmd = f.cmd
	if _, err := f.run.observedSender(); err == nil {
		t.Fatal("fixture caller is observable")
	}
	if err := f.cmd.compact([]string{"tester"}); err != nil {
		t.Fatalf("compact without resume: %v", err)
	}
	if err := f.cmd.compact([]string{"tester", "--resume", "Resume from the state file."}); !strings.Contains(fmt.Sprint(err), "inherited hitch identity") {
		t.Fatalf("resume note from an unobserved caller: %v", err)
	}
}
