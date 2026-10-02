package main

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

// Each logged activity reading names what it was derived from, so the log
// alone explains it. The basis and reason name screen states, never screen
// text, so the log carries no draft, prompt, or reply from the pane.
func TestActivityObservationRecordsItsBasis(t *testing.T) {
	permission := fixtureScreenWithText(t, "claude-code-2.1.287-permission-bash.txt")
	idle := screenWithText("────────", "❯ ", "────────")
	busy := screenWithText("✻ Working… (59m · esc to interrupt)", "────────", "❯ ", "────────")
	for _, tc := range []struct {
		name       string
		screen     substrate.Screen
		captureErr error
		setup      func(*core.Agent, time.Time)
		activity   core.Activity
		basis      core.ActivityBasis
	}{
		{name: "idle screen after an unverified compaction", screen: idle,
			setup: func(a *core.Agent, now time.Time) {
				a.Compaction = &core.Compaction{ID: "c", Status: "unverified", StartedAt: now.Add(-time.Minute), Deadline: now.Add(-time.Second)}
			},
			activity: core.Idle, basis: core.ActivityBasis{Screen: "idle", Rule: "screen", Compaction: "unverified"}},
		{name: "compacting screen", screen: screenWithText("✻ Compacting conversation… (12s · ↓ 1.0k tokens)", "────────", "❯ ", "────────"),
			activity: core.Compacting, basis: core.ActivityBasis{Screen: "compacting", Rule: "screen"}},
		{name: "submitted compaction behind an idle screen", screen: idle,
			setup: func(a *core.Agent, now time.Time) {
				a.Compaction = &core.Compaction{ID: "c", Status: "submitted", StartedAt: now, Deadline: now.Add(time.Hour)}
			},
			activity: core.Compacting, basis: core.ActivityBasis{Screen: "idle", Rule: "compaction-record", Compaction: "submitted"}},
		{name: "turn failure behind an idle screen", screen: idle,
			setup:    func(a *core.Agent, _ time.Time) { a.Native.TurnFailure = "invalid_request" },
			activity: core.Unknown, basis: core.ActivityBasis{Screen: "idle", Rule: "turn-failure"}},
		{name: "open turn behind an idle screen", screen: idle,
			setup:    func(a *core.Agent, now time.Time) { a.Native.SubmittedAt = now.Add(-time.Second) },
			activity: core.Busy, basis: core.ActivityBasis{Screen: "idle", Rule: "open-turn"}},
		{name: "busy screen unchanged past the wedge budget", screen: busy,
			setup: func(a *core.Agent, now time.Time) {
				a.ScreenFingerprint, a.ScreenSince = harness.ScreenFingerprint(busy), now.Add(-time.Hour)
			},
			activity: core.Wedged, basis: core.ActivityBasis{Screen: "busy", Rule: "wedge"}},
		{name: "native permission prompt", screen: permission,
			activity: core.Blocked, basis: core.ActivityBasis{Screen: "blocked", Rule: "screen"}},
		{name: "unsubmitted composer", screen: screenWithText("────────", "❯ secretdraftword", "────────"),
			activity: core.Blocked, basis: core.ActivityBasis{Screen: "unsubmitted", Rule: "screen"}},
		{name: "failed probe", captureErr: errors.New("capture failed"),
			activity: core.Unknown, basis: core.ActivityBasis{Screen: "unread", Rule: "probe-failure"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "claude")
			a.Activity, a.Evidence = core.Unknown, "seeded"
			if tc.setup != nil {
				tc.setup(&a, f.cmd.now())
			}
			p, _ := f.run.team.Agent(a.ID)
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			l.Close()
			f.input.screen, f.input.captureErr = tc.screen, tc.captureErr
			if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil && tc.captureErr == nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			if line, ok := logQuotesScreen(string(data), tc.screen); ok {
				t.Fatalf("log quotes screen line %d", line)
			}
			var observed []core.Event
			if err := store.ReadLog(strings.NewReader(string(data)), func(e core.Event) error {
				if e.Type == "activity_observed" {
					observed = append(observed, e)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if len(observed) != 1 {
				t.Fatalf("activity observations: %+v", observed)
			}
			e := observed[0]
			if e.Activity != tc.activity || e.Basis == nil || *e.Basis != tc.basis {
				t.Fatalf("observation %s basis %+v, want %s basis %+v", e.Activity, e.Basis, tc.activity, tc.basis)
			}
			want := ""
			if tc.captureErr == nil {
				want = harness.ScreenFingerprint(tc.screen)
			}
			if e.Fingerprint != want {
				t.Fatalf("observation fingerprint %q, want %q", e.Fingerprint, want)
			}
		})
	}
}

// A delivery that finds the recipient blocked logs the basis of that reading.
func TestBlockedDeliveryRecordsItsBasis(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	f.input.screen = screenWithText("Giving this request a little extra thought", "› 1. Retry with a faster model", "  2. Keep waiting")
	f.cmd.stdin = strings.NewReader("retain this message")
	if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var observed []core.Event
	if err := store.ReadLog(file, func(e core.Event) error {
		if e.Type == "activity_observed" {
			observed = append(observed, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if line, ok := logQuotesScreen(logText(t, f), f.input.screen); ok {
		t.Fatalf("log quotes screen line %d", line)
	}
	want := core.ActivityBasis{Screen: "blocked", Rule: "screen"}
	if len(observed) != 1 || observed[0].Basis == nil || *observed[0].Basis != want || observed[0].Fingerprint != harness.ScreenFingerprint(f.input.screen) {
		t.Fatalf("blocked delivery observations: %+v", observed)
	}
}

// A startup blocked by a native prompt logs that it is blocked without
// quoting the prompt.
func TestBlockedStartupLogsNoScreenText(t *testing.T) {
	f := newStateFixture(t)
	f.input.screen = screenWithText("Giving this request a little extra thought", "› 1. Retry with a faster model", "  2. Keep waiting")
	a := f.setAgent(t, f.add(t, "a", "worker", "codex"), func(a *core.Agent) {
		a.Status, a.Activity, a.BootDeadline = core.Booting, core.Unknown, time.Time{}
	})
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, a.ID); got.Activity != core.Blocked || got.Evidence == "" {
		t.Fatalf("startup reading %s %q, want blocked", got.Activity, got.Evidence)
	}
	if n := eventsOfType(t, f, "hitch_blocked"); n != 1 {
		t.Fatalf("hitch_blocked events = %d, want 1", n)
	}
	if line, ok := logQuotesScreen(logText(t, f), f.input.screen); ok {
		t.Fatalf("log quotes screen line %d", line)
	}
}

// logQuotesScreen reports the first screen line whose words reach the log
// verbatim: any run of three consecutive words, or every word of a shorter
// line. Failures name the line, never its text, since screen text can carry
// a draft or a native prompt.
func logQuotesScreen(log string, screen substrate.Screen) (int, bool) {
	for index, row := range screen.Rows {
		var text strings.Builder
		for _, cell := range row {
			text.WriteString(cell.Text)
		}
		var words []string
		for _, word := range strings.Fields(text.String()) {
			if strings.IndexFunc(word, unicode.IsLetter) >= 0 {
				words = append(words, word)
			}
		}
		width := min(3, len(words))
		for start := 0; width > 0 && start+width <= len(words); start++ {
			if run := strings.Join(words[start:start+width], " "); len(run) >= 4 && strings.Contains(log, run) {
				return index, true
			}
		}
	}
	return 0, false
}

func fixtureScreenWithText(t *testing.T, name string) substrate.Screen {
	t.Helper()
	data, err := os.ReadFile("../../test/fixtures/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return screenWithText(strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")...)
}

func logText(t *testing.T, f *stateFixture) string {
	t.Helper()
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
