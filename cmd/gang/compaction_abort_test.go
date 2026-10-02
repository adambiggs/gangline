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
