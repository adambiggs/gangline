package main

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

// compactStartFixture drives a Claude compaction whose pane shows afterPaste
// once the compact command is pasted and afterEnter once it is submitted. The
// window for start evidence has already run out, so only the first capture
// after the submit key can show a compaction starting.
func compactStartFixture(t *testing.T, afterPaste, afterEnter func(harness.Collar) substrate.Screen) (*stateFixture, *core.Agent) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.env["GANG_AGENT_ID"] = string(a.ID)
	f.env["TMUX_PANE"] = a.Pane
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.command = "claude"
	c, err := loadCollar("claude", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	empty := screenWithText("────────", "❯ ", "────────")
	f.input.screen = empty
	f.input.onKeys = func(k substrate.Keys) error {
		switch {
		case strings.Contains(k.Text, "/compact "):
			f.input.screen = afterPaste(c)
		case k.Submit && strings.HasPrefix(f.input.pasted, "/compact "):
			f.input.screen = afterEnter(c)
		case slices.Contains(k.Names, "C-u"):
			f.input.screen = empty
		}
		return nil
	}
	f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
		return nil
	}
	f.cmd.newTimeout = func(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
		if d == compactStartWindow {
			spent, cancel := context.WithCancel(ctx)
			cancel()
			return spent, cancel
		}
		return context.WithTimeout(ctx, d)
	}
	// The submit hook's proof is not under test, and run in-process it would
	// wait on the lock this command holds.
	f.input.submit = nil
	f.run.cmd = f.cmd
	return f, &a
}

func compactDraftScreen(harness.Collar) substrate.Screen {
	return screenWithText("────────", "❯ /compact Resume from the state file.", "────────")
}

func idleEmptyScreen(harness.Collar) substrate.Screen {
	return screenWithText("────────", "❯ ", "────────")
}

// The compact command can leave the composer without gang's submit key, as
// when someone else clears it or presses Enter. Gang then sends no Enter, and
// what it tells the agent follows what the pane shows.
func TestCompactCommandGoneBeforeSubmitKey(t *testing.T) {
	for _, tc := range []struct {
		name       string
		after      func(*testing.T) func(harness.Collar) substrate.Screen
		wantNotice string
	}{
		{name: "idle", after: func(*testing.T) func(harness.Collar) substrate.Screen { return idleEmptyScreen }, wantNotice: "was not compacted"},
		{name: "compacting", after: func(t *testing.T) func(harness.Collar) substrate.Screen {
			return func(c harness.Collar) substrate.Screen { return claudeCompactingScreen(t, c) }
		}, wantNotice: "may have run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, a := compactStartFixture(t, tc.after(t), idleEmptyScreen)
			_ = f.cmd.compact([]string{"--resume", "Resume from the state file."})
			p, _ := f.run.team.Agent(a.ID)
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 0 {
				t.Fatalf("Enter reached the pane %d times", f.input.submits)
			}
			if got.Compaction.Status != "failed" || !strings.Contains(got.Compaction.Reason, "left the composer before") {
				t.Fatalf("compaction: %+v", got.Compaction)
			}
			if slices.Contains(f.input.keys, "C-u") {
				t.Fatal("clear keys sent to an empty composer")
			}
			if notice := failureNotice(t, p, "new", got.Compaction.ID); !strings.Contains(notice.Message.Text, tc.wantNotice) {
				t.Fatalf("notice: %q", notice.Message.Text)
			}
		})
	}
}

// An empty composer after the submit key is not evidence that a compaction
// started: the command can be consumed as nothing, or held behind a streaming
// turn and run later. The resume note is pasted only once the pane shows the
// compaction running; without that the compaction may have run.
func TestCompactionResumeNeedsStartEvidence(t *testing.T) {
	t.Run("no compaction shown", func(t *testing.T) {
		f, a := compactStartFixture(t, compactDraftScreen, idleEmptyScreen)
		_ = f.cmd.compact([]string{"--resume", "Resume from the state file."})
		p, _ := f.run.team.Agent(a.ID)
		got, err := p.Read()
		if err != nil {
			t.Fatal(err)
		}
		if f.input.submits != 1 || !strings.HasPrefix(f.input.pasted, "/compact ") {
			t.Fatalf("resume reached the pane: submits=%d pasted=%q", f.input.submits, f.input.pasted)
		}
		if got.Compaction.Status != "failed" || !strings.Contains(got.Compaction.Reason, "no compaction showed") || got.Input != nil {
			t.Fatalf("compaction: %+v input=%+v", got.Compaction, got.Input)
		}
		if notice := failureNotice(t, p, "new", got.Compaction.ID); !strings.Contains(notice.Message.Text, "may have run") {
			t.Fatalf("notice: %q", notice.Message.Text)
		}
	})
	t.Run("command stays in the composer", func(t *testing.T) {
		f, a := compactStartFixture(t, compactDraftScreen, compactDraftScreen)
		_ = f.cmd.compact([]string{"--resume", "Resume from the state file."})
		p, _ := f.run.team.Agent(a.ID)
		got, err := p.Read()
		if err != nil {
			t.Fatal(err)
		}
		if f.input.submits != 1 || got.Compaction.Status != "failed" || !strings.Contains(got.Compaction.Reason, "remained in the composer") {
			t.Fatalf("submits=%d compaction: %+v", f.input.submits, got.Compaction)
		}
	})
	t.Run("compaction shown", func(t *testing.T) {
		f, a := compactStartFixture(t, compactDraftScreen, func(c harness.Collar) substrate.Screen { return claudeCompactingScreen(t, c) })
		_ = f.cmd.compact([]string{"--resume", "Resume from the state file."})
		p, _ := f.run.team.Agent(a.ID)
		got, err := p.Read()
		if err != nil {
			t.Fatal(err)
		}
		if f.input.submits != 2 || got.Compaction.Status == "failed" {
			t.Fatalf("submits=%d compaction: %+v", f.input.submits, got.Compaction)
		}
	})
}

// echoCompactInput makes the fake pane show pasted compact input in its
// composer, and on submit show the corresponding compaction spinner before
// the resume. A key hook the test installed earlier still runs after it.
func echoCompactInput(f *stateFixture) {
	next := f.input.onKeys
	f.input.onKeys = func(k substrate.Keys) error {
		paste := strings.TrimSuffix(strings.TrimPrefix(k.Text, "\x1b[200~"), "\x1b[201~")
		switch {
		case strings.HasPrefix(paste, "/compact"):
			f.input.screen = withComposerText(f.input.screen, paste)
		case k.Submit && strings.HasPrefix(f.input.pasted, "/compact"):
			f.input.screen = withComposerText(f.input.screen, "")
			if f.input.command == "claude" {
				f.input.screen.Rows = append(screenWithText("✻ Compacting conversation… (0s)").Rows, f.input.screen.Rows...)
			} else {
				f.input.screen.Rows = append(screenWithText("◦ Compacting context (0s • esc to interrupt)").Rows, f.input.screen.Rows...)
			}
		}
		if next != nil {
			return next(k)
		}
		return nil
	}
}

// withComposerText replaces the text after the lowest composer prompt.
func withComposerText(screen substrate.Screen, text string) substrate.Screen {
	rows := slices.Clone(screen.Rows)
	for i := len(rows) - 1; i >= 0; i-- {
		var line strings.Builder
		for _, cell := range rows[i] {
			line.WriteString(cell.Text)
		}
		for _, prompt := range []string{"❯ ", "› "} {
			if strings.HasPrefix(line.String(), prompt) {
				rows[i] = screenWithText(prompt + text).Rows[0]
				return substrate.Screen{Rows: rows, Cursor: screen.Cursor}
			}
		}
	}
	return screen
}
