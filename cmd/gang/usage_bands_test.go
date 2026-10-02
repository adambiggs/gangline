package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func usageSnapshot(t *testing.T, run *runtime) usageState {
	t.Helper()
	var state usageState
	if err := run.withUsageState(func(current *usageState) error {
		state = *current
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestUsageBandDefaultIsMeasurementOnly(t *testing.T) {
	reset := time.Date(2026, 10, 3, 16, 58, 14, 0, time.UTC).Unix()
	got := renderUsageBand(harness.UsageBand{Name: "yellow", At: 0.75}, "codex", "weekly", 84, reset)
	want := "codex weekly usage: 84% used, resets 2026-10-03T16:58:14Z"
	if got != want {
		t.Fatalf("default notice = %q, want %q", got, want)
	}
	custom := renderUsageBand(harness.UsageBand{Name: "red", Message: "{{collar}} {{band}}: operator note"}, "claude", "five_hour", 95, reset)
	if custom != "claude red: operator note" {
		t.Fatalf("operator message = %q", custom)
	}
	withNote := renderUsageBand(harness.UsageBand{Name: "red", Note: "{{band}}: operator guidance"}, "codex", "weekly", 95, reset)
	if withNote != "codex weekly usage: 95% used, resets 2026-10-03T16:58:14Z red: operator guidance" {
		t.Fatalf("operator note = %q", withNote)
	}
}

func TestCapResetUsesEarliestCappedWindow(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	reset, ok := cappedNativeReset([]core.LimitWindow{
		{Label: "weekly", WindowMinutes: 10080, UsedPercent: 100, ResetAt: now.Add(7 * 24 * time.Hour).Unix()},
		{Label: "five_hour", WindowMinutes: 300, UsedPercent: 100, ResetAt: now.Add(5 * time.Hour).Unix()},
	}, now)
	if !ok || !reset.Equal(now.Add(5*time.Hour)) {
		t.Fatalf("capped reset = %s, %t", reset, ok)
	}
}

func TestNativeCapParksAndResumesWithoutManualSnooze(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			f.input.command = collar
			if collar == "claude" {
				f.input.screen = screenWithText("────────────────────────────────────────────────────────────────────────────────", "❯", "────────────────────────────────────────────────────────────────────────────────")
			}
			a := f.add(t, "caller-id", "worker", collar)
			at := f.cmd.now()
			reset := at.Add(5 * time.Hour)
			a.Native.SubmittedAt = at.Add(-time.Second)
			a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", WindowMinutes: 300, UsedPercent: 100, ResetAt: reset.Unix()}}}
			failure := hookNotice{Kind: "turn-failed", TurnID: "native-turn", At: at, Failure: "You have hit your usage limit"}
			if collar == "claude" {
				a.Native.FailedTurn = "native-turn"
			} else {
				failure = hookNotice{}
				a.Native.LastErrorAt, a.Native.LastError = at, "You have hit your usage limit"
			}
			if err := f.run.observeAutoCap(a, failure); err != nil {
				t.Fatal(err)
			}
			parked := usageSnapshot(t, f.run).Snoozes[string(a.ID)]
			if !parked.Auto || !parked.At.Equal(reset) {
				t.Fatalf("cap did not park until reset: %+v", parked)
			}
			if err := f.run.observeAutoCap(a, failure); err != nil {
				t.Fatal(err)
			}
			if got := usageSnapshot(t, f.run).Snoozes[string(a.ID)].ID; got != parked.ID {
				t.Fatalf("duplicate cap created another wake: %s", got)
			}
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			if err := f.run.flushUsageWork(); err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 0 {
				t.Fatal("wake submitted before reset")
			}
			f.run.cmd.clock = func() time.Time { return reset }
			if err := f.run.flushUsageWork(); err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 1 || !strings.Contains(f.input.pasted, "Resume the interrupted work") {
				t.Fatalf("reset did not resume parked agent: submits=%d text=%q", f.input.submits, f.input.pasted)
			}
		})
	}
}

func TestCapWithoutResetWaitsForNativeReading(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "claude")
	at := f.cmd.now()
	a.Native.FailedTurn = "native-turn"
	failure := hookNotice{Kind: "turn-failed", TurnID: "native-turn", At: at, Failure: "You have hit your usage limit"}
	if err := f.run.observeAutoCap(a, failure); err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 || !usageSnapshot(t, f.run).Snoozes[string(a.ID)].At.IsZero() {
		t.Fatal("cap without reset submitted a wake")
	}
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 100, ResetAt: at.Add(5 * time.Hour).Unix()}}}
	if err := f.run.observeAutoCap(a, hookNotice{}); err != nil {
		t.Fatal(err)
	}
	if usageSnapshot(t, f.run).Snoozes[string(a.ID)].At.IsZero() {
		t.Fatal("native reading did not resolve parked wake")
	}
}

func TestGenericCapFailureWaitsForFreshCappedReading(t *testing.T) {
	f := newStateFixture(t)
	f.input.command = "claude"
	a := f.add(t, "caller-id", "worker", "claude")
	at := f.cmd.now()
	a.Native.SubmittedAt = at.Add(-time.Second)
	a.Native.FailedTurn = "native-turn"
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 40, ResetAt: at.Add(5 * time.Hour).Unix()}}}
	failure := hookNotice{Kind: "turn-failed", TurnID: "native-turn", At: at, Failure: "rate_limit"}
	if err := f.run.observeAutoCap(a, failure); err != nil {
		t.Fatal(err)
	}
	state := usageSnapshot(t, f.run)
	if state.Snoozes[string(a.ID)].ID != "" || state.AutoCandidates[string(a.ID)].At.IsZero() {
		t.Fatalf("generic throttle was parked without cap evidence: %+v", state)
	}
	f.env["GANG_AGENT_ID"] = string(a.ID)
	f.env["TMUX_PANE"] = a.Pane
	if err := f.cmd.snooze([]string{"--status"}); err != nil || !strings.Contains(f.out.String(), "provider cap unconfirmed") {
		t.Fatalf("unconfirmed cap status: %q, %v", f.out.String(), err)
	}
	fresh := at.Add(time.Second)
	a.Native.Limits.At = &fresh
	a.Native.Limits.Limits[0].UsedPercent = 100
	f.run.cmd.clock = func() time.Time { return fresh }
	if err := f.run.observeAutoCap(a, hookNotice{}); err != nil {
		t.Fatal(err)
	}
	state = usageSnapshot(t, f.run)
	if !state.Snoozes[string(a.ID)].Auto || len(state.AutoCandidates) != 0 {
		t.Fatalf("fresh cap reading did not confirm park: %+v", state)
	}
}

func TestPostResetLowUsageWakesPendingCapImmediately(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "claude")
	at := f.cmd.now()
	a.Native.FailedTurn = "native-turn"
	failure := hookNotice{Kind: "turn-failed", TurnID: "native-turn", At: at, Failure: "You have hit your usage limit"}
	if err := f.run.observeAutoCap(a, failure); err != nil {
		t.Fatal(err)
	}
	later := at.Add(5 * time.Hour)
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &later, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 1, ResetAt: later.Add(5 * time.Hour).Unix()}}}
	f.run.cmd.clock = func() time.Time { return later }
	if err := f.run.observeAutoCap(a, hookNotice{}); err != nil {
		t.Fatal(err)
	}
	if got := usageSnapshot(t, f.run).Snoozes[string(a.ID)].At; !got.Equal(later) {
		t.Fatalf("post-reset wake due %s, want %s", got, later)
	}
}

func TestClaudeCapHookSchedulesAutomaticWake(t *testing.T) {
	f := newStateFixture(t)
	f.input.command = "claude"
	f.input.screen = screenWithText("────────────────────────────────────────────────────────────────────────────────", "❯", "────────────────────────────────────────────────────────────────────────────────")
	a := f.add(t, "caller-id", "worker", "claude")
	at := f.cmd.now()
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 100, ResetAt: at.Add(5 * time.Hour).Unix()}}}
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WriteWitness(store.Witness{ID: "native-submit", At: at.Add(-time.Second), SessionID: "s", TurnID: "native-turn"}); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	failure := hookNotice{Kind: "turn-failed", SessionID: "s", TurnID: "native-turn", At: at, Failure: "You have hit your usage limit"}
	if err := f.run.tickAgent(a.ID, failure, true); err != nil {
		t.Fatal(err)
	}
	parked := usageSnapshot(t, f.run).Snoozes[string(a.ID)]
	if !parked.Auto || !parked.At.Equal(at.Add(5*time.Hour)) {
		t.Fatalf("hook failed to park agent: %+v", parked)
	}
}

func TestCodexAutomaticWakeRearmsAfterAnotherCap(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "codex")
	at := f.cmd.now()
	a.Native.TurnID = "wake-turn"
	a.Native.LastErrorAt, a.Native.LastError = at, "Usage limit reached"
	reset := at.Add(5 * time.Hour)
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 100, ResetAt: reset.Unix()}}}
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[string(a.ID)] = usageSnooze{ID: "old-wake", CallerID: a.ID, CallerName: a.Name, RecipientID: a.ID, TurnID: "wake-turn", SubmittedAt: at.Add(-time.Second), Auto: true}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeSnoozeTurn(a, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	next := usageSnapshot(t, f.run).Snoozes[string(a.ID)]
	if !next.Auto || next.Rearms != 1 || !next.At.Equal(reset) {
		t.Fatalf("automatic wake was not rearmed: %+v", next)
	}
}

func TestUsageBandsAreAccountWideAndLeadOnly(t *testing.T) {
	f := newStateFixture(t)
	lead := f.add(t, "lead-id", "ser5", "codex")
	p, _ := f.run.team.Agent(lead.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	lead.Role = "lead"
	if err := l.Save(lead); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	first := f.add(t, "first", "first", "codex")
	second := f.add(t, "second", "second", "codex")
	c, err := loadCollar("codex", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	reset := f.cmd.now().Add(5 * time.Hour).Unix()
	observe := func(a core.Agent, percent float64, at time.Time, status string) {
		t.Helper()
		f.run.cmd.clock = func() time.Time { return at }
		a.Native.Limits = core.Reading{Kind: "provider-limits", Status: status, At: &at, Limits: []core.LimitWindow{{Label: "codex/primary", WindowMinutes: 300, UsedPercent: percent, ResetAt: reset}}}
		if err := f.run.observeUsageBands(a, c); err != nil {
			t.Fatal(err)
		}
	}
	at := f.cmd.now()
	observe(first, 80, at, "observed")
	observe(second, 95, at, "observed")                  // the same native timestamp can carry a newer account percentage
	observe(first, 99, at.Add(-time.Second), "observed") // an older agent reading cannot roll the state back
	observe(second, 100, at.Add(2*time.Second), "unknown")
	state := usageSnapshot(t, f.run)
	if len(state.Notices) != 2 || len(state.Windows["codex/five_hour"].Fired) != 2 {
		t.Fatalf("account crossing state: %+v", state)
	}
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 2 || !strings.Contains(f.input.pasted, "[gang:usage-band#") {
		t.Fatalf("lead received %d notices; last = %q", f.input.submits, f.input.pasted)
	}
	if len(usageSnapshot(t, f.run).Notices) != 0 {
		t.Fatal("delivered notices remain pending")
	}
	observe(second, 100, at.Add(3*time.Second), "observed")
	if len(usageSnapshot(t, f.run).Notices) != 0 {
		t.Fatal("same reset repeated a band")
	}
	reset = f.cmd.now().Add(10 * time.Hour).Unix()
	observe(first, 95, at.Add(2*time.Second), "observed")
	if len(usageSnapshot(t, f.run).Notices) != 0 {
		t.Fatal("older observation with a later reset repeated bands")
	}
	observe(second, 80, at.Add(4*time.Second), "observed")
	if len(usageSnapshot(t, f.run).Notices) != 1 {
		t.Fatal("new native reset did not create a new crossing")
	}
}

func TestUsageBandWaitsForLead(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "worker-id", "worker", "claude")
	c, err := loadCollar("claude", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	at := f.cmd.now()
	worker.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "seven_day", UsedPercent: 95, ResetAt: at.Add(7 * 24 * time.Hour).Unix()}}}
	if err := f.run.observeUsageBands(worker, c); err != nil {
		t.Fatal(err)
	}
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 || len(usageSnapshot(t, f.run).Notices) != 2 {
		t.Fatal("missing lead consumed or misrouted notices")
	}
}

func TestUsageNoticeReceiptSurvivesLaterDeliveryUntilAcknowledged(t *testing.T) {
	f := newStateFixture(t)
	lead := f.add(t, "lead-id", "lead", "codex")
	worker := f.add(t, "worker-id", "worker", "codex")
	c, err := loadCollar("codex", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	at := f.cmd.now()
	worker.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "primary", WindowMinutes: 300, UsedPercent: 80, ResetAt: at.Add(5 * time.Hour).Unix()}}}
	if err := f.run.observeUsageBands(worker, c); err != nil {
		t.Fatal(err)
	}
	n := usageSnapshot(t, f.run).Notices[0]
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Notices[0].RecipientID, state.Notices[0].RecipientName = lead.ID, lead.Name
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: n.ID, Token: n.Token, Recipient: lead.ID, To: lead.Name, From: core.Sender{Kind: core.SenderGangline, Name: "usage-band"}, Message: core.Message{Text: n.Text}, CreatedAt: n.CreatedAt}
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	if delivered, err := f.run.publishUsageEnvelope(e); err != nil || !delivered {
		t.Fatalf("first notice delivery: %v, %t", err, delivered)
	}
	if len(usageSnapshot(t, f.run).Notices) != 0 {
		t.Fatal("exact submit did not durably acknowledge the notice")
	}
	p, err := f.run.team.Agent(lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	other := core.Envelope{ID: "other-delivery", Token: "0123456789abcdef", Recipient: lead.ID, To: lead.Name, From: core.Sender{Kind: core.SenderGangline, Name: "other"}, Message: core.Message{Text: "another message"}, CreatedAt: at}
	if err := p.Publish(other); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	agent, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&agent, other, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReadEnvelope("cur", n.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("later delivery did not evict old inbox receipt: %v", err)
	}
	before := f.input.submits
	if delivered, err := f.run.publishUsageEnvelope(e); err != nil || delivered {
		t.Fatalf("stale flusher republished acknowledged notice: %t, %v", delivered, err)
	}
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != before || len(usageSnapshot(t, f.run).Notices) != 0 {
		t.Fatalf("notice was duplicated or not acknowledged: submits=%d", f.input.submits)
	}
}

func TestUnconfirmedUsageInputIsNotResubmittedAfterReceiptEviction(t *testing.T) {
	for _, tc := range []struct{ kind, outcome, dir string }{
		{"notice", "accepted", "cur"},
		{"notice", "unverified", "failed"},
		{"wake", "accepted", "cur"},
		{"wake", "unverified", "failed"},
	} {
		t.Run(tc.kind+"-"+tc.outcome, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "lead-id", "lead", "codex")
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			at := f.cmd.now()
			from := "usage-band"
			id := core.EnvelopeID("pending-notice")
			if tc.kind == "wake" {
				from, id = "snooze", "pending-wake"
			}
			e := core.Envelope{ID: id, Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: core.AgentName(from)}, Message: core.Message{Text: "resume"}, CreatedAt: at}
			if err := f.run.withUsageState(func(state *usageState) error {
				if tc.kind == "notice" {
					state.Notices = append(state.Notices, usageNotice{ID: id, Token: e.Token, Collar: "codex", Window: "five_hour", Band: "yellow", Text: e.Message.Text, CreatedAt: at, RecipientID: a.ID, RecipientName: a.Name})
				} else {
					state.Snoozes[string(a.ID)] = usageSnooze{ID: id, Token: e.Token, CallerID: a.ID, CallerName: a.Name, At: at, Note: e.Message.Text, RecipientID: a.ID, RecipientName: a.Name}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, envelope := range []core.Envelope{e, {ID: "other-input", Token: "abcdef0123456789", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "other"}, Message: core.Message{Text: "other"}, CreatedAt: at}} {
				if err := p.Publish(envelope); err != nil {
					t.Fatal(err)
				}
				l, err := p.TryLock()
				if err != nil {
					t.Fatal(err)
				}
				if err := f.run.apply(l, &a, core.Event{Type: "input_started", ID: string(envelope.ID), Status: "envelope"}); err != nil {
					t.Fatal(err)
				}
				if err := f.run.finishInput(l, &a, envelope, tc.outcome, "native input not yet confirmed"); err != nil {
					t.Fatal(err)
				}
				if err := l.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := p.ReadEnvelope(tc.dir, id); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("prior receipt was not evicted: %v", err)
			}
			if err := f.run.flushUsageWork(); err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 0 {
				t.Fatalf("unconfirmed %s input was resubmitted", tc.kind)
			}
			state := usageSnapshot(t, f.run)
			if tc.kind == "notice" {
				if len(state.Notices) != 1 || state.Notices[0].Submission != tc.outcome {
					t.Fatalf("notice uncertainty lost: %+v", state)
				}
				wire, err := envelopeText(e)
				if err != nil {
					t.Fatal(err)
				}
				if err := p.WriteWitness(store.Witness{ID: "late-notice", At: at.Add(time.Second), TurnID: "notice-turn", Prompt: wire}); err != nil {
					t.Fatal(err)
				}
				l, _, err := f.run.acquire(a.ID, false)
				if err != nil {
					t.Fatal(err)
				}
				if err := l.Close(); err != nil {
					t.Fatal(err)
				}
				if notices := usageSnapshot(t, f.run).Notices; len(notices) != 0 {
					t.Fatalf("late witness did not clear submitted notice: %+v", notices)
				}
			} else {
				if state.Snoozes[string(a.ID)].Submission != tc.outcome {
					t.Fatalf("wake uncertainty lost: %+v", state)
				}
				wire, err := envelopeText(e)
				if err != nil {
					t.Fatal(err)
				}
				if err := p.WriteWitness(store.Witness{ID: "late-wake", At: at.Add(time.Second), TurnID: "wake-turn", Prompt: wire, Joined: true}); err != nil {
					t.Fatal(err)
				}
				l, a, err := f.run.acquire(a.ID, false)
				if err != nil {
					t.Fatal(err)
				}
				if err := l.Close(); err != nil {
					t.Fatal(err)
				}
				state = usageSnapshot(t, f.run)
				if state.Snoozes[string(a.ID)].ID != "" || state.Recent[string(a.ID)].TurnID != "wake-turn" || !state.Recent[string(a.ID)].Joined {
					t.Fatalf("late witness did not link wake turn: %+v", state)
				}
				if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-finished", TurnID: "wake-turn"}, false); err != nil {
					t.Fatal(err)
				}
				if len(usageSnapshot(t, f.run).Recent) != 0 {
					t.Fatal("matching successful turn left queued wake pending")
				}
			}
		})
	}
}

func TestConcurrentUsageBandSamplesCreateOneAccountNotice(t *testing.T) {
	f := newStateFixture(t)
	c, err := loadCollar("codex", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	at := f.cmd.now()
	start := make(chan struct{})
	errors := make(chan error, 2)
	var group sync.WaitGroup
	for _, id := range []core.HitchID{"a", "b"} {
		group.Add(1)
		go func(id core.HitchID) {
			defer group.Done()
			<-start
			a := core.Agent{ID: id, Collar: "codex", Status: core.Active, Native: core.NativeState{Limits: core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "codex/primary", WindowMinutes: 300, UsedPercent: 80, ResetAt: at.Add(5 * time.Hour).Unix()}}}}}
			errors <- f.run.observeUsageBands(a, c)
		}(id)
	}
	close(start)
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := len(usageSnapshot(t, f.run).Notices); got != 1 {
		t.Fatalf("concurrent samples created %d notices, want one", got)
	}
}

func TestSnoozeDurableDueAndCallerFallback(t *testing.T) {
	f := newStateFixture(t)
	caller := f.add(t, "caller-id", "worker", "codex")
	f.env["GANG_AGENT_ID"] = string(caller.ID)
	f.env["TMUX_PANE"] = caller.Pane
	if err := f.cmd.snooze([]string{"--at", "2h", "--note", "Continue the saved task"}); err != nil {
		t.Fatal(err)
	}
	s := usageSnapshot(t, f.run).Snoozes[string(caller.ID)]
	if s.ID == "" || !s.At.Equal(f.cmd.now().Add(2*time.Hour)) {
		t.Fatalf("snooze was not retained: %+v", s)
	}
	restarted, err := f.cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	f.run = restarted
	if got := usageSnapshot(t, f.run).Snoozes[string(caller.ID)].ID; got != s.ID {
		t.Fatalf("restarted runtime lost wake %q: got %q", s.ID, got)
	}
	if err := f.run.flushUsageWork(); err != nil || f.input.submits != 0 {
		t.Fatalf("wake delivered early: %v, %d submits", err, f.input.submits)
	}
	// A restarted team supersedes the old hitch claim. The retained wake goes to
	// the newly active lead, with the missed deadline stated in its message.
	if err := f.run.team.RemoveName(caller.Name, caller.ID); err != nil {
		t.Fatal(err)
	}
	lead := f.add(t, "new-lead", "lead", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	now := f.cmd.now().Add(3 * time.Hour)
	f.run.cmd.clock = func() time.Time { return now }
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 || !strings.Contains(f.input.pasted, "overdue by 1h0m0s") || !strings.Contains(f.input.pasted, "worker is no longer active") || !strings.Contains(f.input.pasted, "Continue the saved task") {
		t.Fatalf("fallback wake: submits=%d text=%q", f.input.submits, f.input.pasted)
	}
	if state := usageSnapshot(t, f.run); len(state.Snoozes) != 0 || len(state.Recent) != 1 {
		t.Fatal("submitted wake was not retained pending turn success")
	}
}

func TestUnconfirmedWakeRoutesToLeadAfterRecipientDisappears(t *testing.T) {
	f := newStateFixture(t)
	caller := f.add(t, "caller-id", "worker", "codex")
	at := f.cmd.now().Add(-time.Hour)
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[string(caller.ID)] = usageSnooze{ID: "submitted-wake", Token: "0123456789abcdef", CallerID: caller.ID, CallerName: caller.Name, RecipientID: caller.ID, RecipientName: caller.Name, TurnID: "old-turn", At: at, Note: "Resume saved work"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.team.RemoveName(caller.Name, caller.ID); err != nil {
		t.Fatal(err)
	}
	lead := f.add(t, "new-lead", "lead", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 || !strings.Contains(f.input.pasted, "overdue") || !strings.Contains(f.input.pasted, "worker is no longer active") {
		t.Fatalf("unconfirmed wake was not routed to lead: %q", f.input.pasted)
	}
	if state := usageSnapshot(t, f.run); len(state.Snoozes) != 0 || state.Recent[string(caller.ID)].RecipientID != lead.ID {
		t.Fatalf("recovered wake lost pending completion: %+v", state)
	}
}

func TestSnoozeDefaultsToNativeResetAndCanBeCleared(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "codex")
	f.env["GANG_AGENT_ID"] = string(a.ID)
	f.env["TMUX_PANE"] = a.Pane
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	at := f.cmd.now()
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{
		{Label: "codex/primary", WindowMinutes: 300, UsedPercent: 85, ResetAt: at.Add(5 * time.Hour).Unix()},
		{Label: "codex/secondary", WindowMinutes: 10080, UsedPercent: 90, ResetAt: at.Add(7 * 24 * time.Hour).Unix()},
	}}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	if err := f.cmd.snooze(nil); err != nil {
		t.Fatal(err)
	}
	s := usageSnapshot(t, f.run).Snoozes[string(a.ID)]
	if !s.At.Equal(at.Add(7 * 24 * time.Hour)) {
		t.Fatalf("default reset = %s", s.At)
	}
	if err := f.cmd.snooze([]string{"--clear"}); err != nil {
		t.Fatal(err)
	}
	if len(usageSnapshot(t, f.run).Snoozes) != 0 {
		t.Fatal("clear retained the wake")
	}
}

func TestSnoozeRejectsUnknownNativeReset(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "codex")
	f.env["GANG_AGENT_ID"] = string(a.ID)
	if err := f.cmd.snooze(nil); err == nil || !strings.Contains(err.Error(), "unavailable or stale") {
		t.Fatalf("unknown limit accepted: %v", err)
	}
}

func TestSnoozeRejectsNoteThatWouldOverflowOverdueFallback(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "codex")
	f.env["GANG_AGENT_ID"] = string(a.ID)
	f.env["TMUX_PANE"] = a.Pane
	note := strings.Repeat("x", maximumMessageBytes-180)
	if err := f.cmd.snooze([]string{"--at", "1h", "--note", note}); err == nil || !strings.Contains(err.Error(), "message exceeds") {
		t.Fatalf("oversize fallback accepted: %v", err)
	}
	if len(usageSnapshot(t, f.run).Snoozes) != 0 {
		t.Fatal("invalid wake was persisted")
	}
}

func TestSnoozeWaitsForNativeInputAndRetriesQueuedWake(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "codex")
	f.env["GANG_AGENT_ID"] = string(a.ID)
	f.env["TMUX_PANE"] = a.Pane
	if err := f.cmd.snooze([]string{"--at", "1m"}); err != nil {
		t.Fatal(err)
	}
	f.input.screen = screenWithText("Would you like to run this command?", "› 1. Yes, proceed", "  2. No")
	due := f.cmd.now().Add(time.Minute)
	f.run.cmd.clock = func() time.Time { return due }
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 || len(usageSnapshot(t, f.run).Snoozes) != 1 {
		t.Fatal("blocked input consumed a queued wake")
	}
	f.input.screen = screenWithText("READY", "› ")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if state := usageSnapshot(t, f.run); f.input.submits != 1 || len(state.Snoozes) != 0 || len(state.Recent) != 1 {
		t.Fatalf("retry did not confirm the wake: submits=%d", f.input.submits)
	}
}

func TestAttributableUsageCapFailureRearmsOneWakeAtNativeReset(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "claude")
	at := f.cmd.now()
	reset := at.Add(5 * time.Hour)
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 100, ResetAt: reset.Unix()}}}
	old := usageSnooze{ID: "old-wake", Token: "0123456789abcdef", CallerID: a.ID, CallerName: a.Name, RecipientID: a.ID, RecipientName: a.Name, At: at, Note: "Resume from saved state"}
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Snoozes[string(a.ID)] = old
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: old.ID, Token: old.Token, Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "snooze"}, Message: core.Message{Text: old.Note}, CreatedAt: at}
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	witness := store.Witness{ID: "wake-start", At: at.Add(-time.Second), TurnID: "wake-turn"}
	if err := p.WriteWitness(witness); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Native.TurnID = "wake-turn"
	if err := f.run.apply(l, &a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.finishInput(l, &a, e, "delivered", "", witness); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if state := usageSnapshot(t, f.run); len(state.Snoozes) != 0 || state.Recent[string(a.ID)].TurnID != "wake-turn" {
		t.Fatalf("confirmed Claude wake did not retain turn identity: %+v", state)
	}
	a.Native.FailedTurn = "wake-turn"
	failure := hookNotice{Kind: "turn-failed", TurnID: "wake-turn", Failure: "You have hit your usage limit"}
	if err := f.run.observeSnoozeTurn(a, failure, false); err != nil {
		t.Fatal(err)
	}
	state := usageSnapshot(t, f.run)
	rearmed := state.Snoozes[string(a.ID)]
	if rearmed.ID == "" || rearmed.ID == old.ID || !rearmed.At.Equal(reset) || rearmed.Note != old.Note || len(state.Recent) != 0 {
		t.Fatalf("native reset rearm: %+v", state)
	}
	if err := f.run.observeSnoozeTurn(a, failure, false); err != nil {
		t.Fatal(err)
	}
	if got := usageSnapshot(t, f.run).Snoozes[string(a.ID)].ID; got != rearmed.ID {
		t.Fatalf("duplicate failure rearmed another wake: %q", got)
	}
	if err := f.run.withUsageState(func(state *usageState) error {
		next := state.Snoozes[string(a.ID)]
		next.RecipientID, next.RecipientName = a.ID, a.Name
		state.Snoozes[string(a.ID)] = next
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.run.cmd.clock = func() time.Time { return reset }
	second := core.Envelope{ID: rearmed.ID, Token: rearmed.Token, Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "snooze"}, Message: core.Message{Text: rearmed.Note}, CreatedAt: reset}
	if err := p.Publish(second); err != nil {
		t.Fatal(err)
	}
	witness = store.Witness{ID: "second-start", At: reset.Add(-time.Second), TurnID: "second-turn"}
	if err := p.WriteWitness(witness); err != nil {
		t.Fatal(err)
	}
	l, err = p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Native.TurnID = "second-turn"
	if err := f.run.apply(l, &a, core.Event{Type: "input_started", ID: string(second.ID), Status: "envelope"}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.finishInput(l, &a, second, "delivered", "", witness); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	a.Native.FailedTurn = "second-turn"
	if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-failed", TurnID: "second-turn", Failure: "Usage limit reached again"}, false); err != nil {
		t.Fatal(err)
	}
	if state := usageSnapshot(t, f.run); len(state.Snoozes) != 0 || len(state.Recent) != 1 || !state.Recent[string(a.ID)].CapRejected {
		t.Fatalf("second rejection lost unresolved wake or exceeded retry budget: %+v", state)
	}
	log, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(log, []byte(`"type":"snooze_rearmed"`)) != 1 {
		t.Fatalf("rearm was not logged once: %s", log)
	}
}

func TestNonCapFailureDoesNotRearmWake(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "claude")
	a.Native.FailedTurn = "wake-turn"
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[string(a.ID)] = usageSnooze{ID: "old-wake", CallerID: a.ID, RecipientID: a.ID, TurnID: "wake-turn"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-failed", TurnID: "wake-turn", Failure: "Login expired"}, false); err != nil {
		t.Fatal(err)
	}
	state := usageSnapshot(t, f.run)
	if len(state.Recent) != 1 || !state.Recent[string(a.ID)].TurnFailed || len(state.Snoozes) != 0 {
		t.Fatalf("non-cap failure lost unresolved wake or rearmed: %+v", state)
	}
}

func TestGenericRateLimitWaitsForNativeCapEvidence(t *testing.T) {
	f := newStateFixture(t)
	f.input.command = "claude"
	a := f.add(t, "caller-id", "worker", "claude")
	a.Native.FailedTurn = "wake-turn"
	at := f.cmd.now()
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 40, ResetAt: at.Add(5 * time.Hour).Unix()}}}
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[string(a.ID)] = usageSnooze{ID: "wake", CallerID: a.ID, CallerName: a.Name, RecipientID: a.ID, TurnID: "wake-turn", SubmittedAt: at.Add(-time.Second)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-failed", TurnID: "wake-turn", Failure: "rate_limit"}, false); err != nil {
		t.Fatal(err)
	}
	state := usageSnapshot(t, f.run)
	if len(state.Snoozes) != 0 || !state.Recent[string(a.ID)].CapCandidate || state.Recent[string(a.ID)].CapRejected {
		t.Fatalf("generic throttle was treated as an attributable cap: %+v", state)
	}
	f.env["GANG_AGENT_ID"] = string(a.ID)
	f.env["TMUX_PANE"] = a.Pane
	if err := f.cmd.snooze([]string{"--status"}); err != nil || !strings.Contains(f.out.String(), "rate limit unconfirmed") {
		t.Fatalf("uncertain throttle status: %q, %v", f.out.String(), err)
	}
	a.Native.Limits.Limits[0].UsedPercent = 100
	if err := f.run.observeSnoozeTurn(a, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if next := usageSnapshot(t, f.run).Snoozes[string(a.ID)]; next.ID == "" || next.Rearms != 1 {
		t.Fatalf("fresh capped reading did not rearm candidate: %+v", next)
	}
}

func TestOldRateLimitCandidateCannotUseLaterCapReading(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "claude")
	a.Native.FailedTurn = "wake-turn"
	at := f.cmd.now()
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 40, ResetAt: at.Add(5 * time.Hour).Unix()}}}
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[string(a.ID)] = usageSnooze{ID: "wake", CallerID: a.ID, RecipientID: a.ID, TurnID: "wake-turn", SubmittedAt: at.Add(-time.Second)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-failed", TurnID: "wake-turn", Failure: "rate_limit"}, false); err != nil {
		t.Fatal(err)
	}
	later := at.Add(3 * 24 * time.Hour)
	f.run.cmd.clock = func() time.Time { return later }
	a.Native.Limits.At = &later
	a.Native.Limits.Limits[0].UsedPercent = 100
	a.Native.Limits.Limits[0].ResetAt = later.Add(5 * time.Hour).Unix()
	if err := f.run.observeSnoozeTurn(a, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	state := usageSnapshot(t, f.run)
	if len(state.Snoozes) != 0 || !state.Recent[string(a.ID)].TurnFailed || state.Recent[string(a.ID)].CapCandidate {
		t.Fatalf("later unrelated cap rearmed old rate-limit failure: %+v", state)
	}
}

func TestLeadCanInspectAndClearUncertainUsageIntents(t *testing.T) {
	f := newStateFixture(t)
	lead := f.add(t, "lead-id", "lead", "codex")
	f.env["GANG_AGENT_ID"] = string(lead.ID)
	f.env["TMUX_PANE"] = lead.Pane
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Notices = []usageNotice{{ID: "notice", RecipientID: lead.ID, Submission: "unverified", Collar: "codex", Window: "weekly", Band: "red"}}
		state.Snoozes["gone-caller"] = usageSnooze{ID: "fallback", CallerID: "gone-caller", CallerName: "worker", RecipientID: lead.ID, Submission: "accepted"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.snooze([]string{"--status"}); err != nil {
		t.Fatal(err)
	}
	if output := f.out.String(); !strings.Contains(output, "notice\t") || !strings.Contains(output, "fallback\t") {
		t.Fatalf("lead cannot inspect uncertain intents: %q", output)
	}
	for _, id := range []string{"notice", "fallback"} {
		if err := f.cmd.snooze([]string{"--clear", id}); err != nil {
			t.Fatal(err)
		}
	}
	state := usageSnapshot(t, f.run)
	if len(state.Notices) != 0 || len(state.Snoozes) != 0 {
		t.Fatalf("lead could not clear uncertain intents: %+v", state)
	}
}

func TestFailedRecipientReroutesUncertainNativeInput(t *testing.T) {
	f := newStateFixture(t)
	caller := f.add(t, "old-worker", "worker", "codex")
	p, err := f.run.team.Agent(caller.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.apply(l, &caller, core.Event{Type: "hitch_failed", Reason: "registered pane is absent from tmux"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	lead := f.add(t, "new-lead", "lead", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Notices = []usageNotice{{ID: "notice", Token: "0123456789abcdef", RecipientID: caller.ID, RecipientName: caller.Name, Submission: "unverified", Text: "Cap warning", ResetAt: f.cmd.now().Add(time.Hour).Unix()}}
		state.Recent[string(caller.ID)] = usageSnooze{ID: "recent", Token: "abcdef0123456789", CallerID: caller.ID, CallerName: caller.Name, RecipientID: caller.ID, RecipientName: caller.Name, At: f.cmd.now().Add(-time.Hour), TurnID: "old-turn", Note: "Resume saved work", CapCandidate: true}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	state := usageSnapshot(t, f.run)
	if f.input.submits != 2 || len(state.Notices) != 0 || state.Recent[string(caller.ID)].RecipientID != lead.ID || state.Recent[string(caller.ID)].CapCandidate {
		t.Fatalf("failed recipient stranded uncertain input instead of rerouting to lead: submits=%d state=%+v", f.input.submits, state)
	}
}

func TestDeliveredWakeKeepsItsOwnWitness(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "codex")
	at := f.cmd.now()
	witnessAt := at.Add(-time.Second)
	s := usageSnooze{ID: "wake", Token: "0123456789abcdef", CallerID: a.ID, RecipientID: a.ID, At: at}
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Snoozes[string(a.ID)] = s
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: s.ID, Token: s.Token, Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "snooze"}}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteWitness(store.Witness{ID: "other", At: at, TurnID: "other-turn"}); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Native.TurnID = "wake-turn"
	if err := f.run.apply(l, &a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.finishInput(l, &a, e, "delivered", "", store.Witness{At: witnessAt, Joined: true}); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if got := usageSnapshot(t, f.run).Recent[string(a.ID)]; !got.SubmittedAt.Equal(witnessAt) || !got.Joined {
		t.Fatalf("delivered wake lost its own witness (submitted %s, joined): %+v", witnessAt, got)
	}
	a.Native.LastErrorAt = witnessAt.Add(500 * time.Millisecond)
	if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-finished", TurnID: "wake-turn"}, false); err != nil {
		t.Fatal(err)
	}
	if len(usageSnapshot(t, f.run).Recent) != 1 {
		t.Fatal("native error after the original submit was treated as turn success")
	}
}

func TestSnoozeRequiresMatchingSuccessfulNativeTurn(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "caller-id", "worker", collar)
			if err := f.run.withUsageState(func(state *usageState) error {
				state.Recent[string(a.ID)] = usageSnooze{ID: "wake", CallerID: a.ID, RecipientID: a.ID, TurnID: "wake-turn"}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-finished", TurnID: "other-turn"}, false); err != nil {
				t.Fatal(err)
			}
			if len(usageSnapshot(t, f.run).Recent) != 1 {
				t.Fatal("another native turn completed the wake")
			}
			if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-finished", TurnID: "wake-turn"}, false); err != nil {
				t.Fatal(err)
			}
			if len(usageSnapshot(t, f.run).Recent) != 0 {
				t.Fatal("matching successful native turn left wake pending")
			}
		})
	}
}

func TestCodexCapacityScreenDoesNotCompleteWake(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "codex")
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[string(a.ID)] = usageSnooze{ID: "wake", CallerID: a.ID, RecipientID: a.ID, TurnID: "wake-turn"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-finished", TurnID: "wake-turn"}, true); err != nil {
		t.Fatal(err)
	}
	if s := usageSnapshot(t, f.run).Recent[string(a.ID)]; !s.TurnFailed {
		t.Fatal("terminal capacity error completed a wake")
	}
}

func TestCodexNativeErrorDoesNotCompleteWake(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "codex")
	at := f.cmd.now()
	a.Native.LastErrorAt = at
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[string(a.ID)] = usageSnooze{ID: "wake", CallerID: a.ID, RecipientID: a.ID, TurnID: "wake-turn", SubmittedAt: at.Add(-time.Second)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-finished", TurnID: "wake-turn"}, false); err != nil {
		t.Fatal(err)
	}
	if s := usageSnapshot(t, f.run).Recent[string(a.ID)]; !s.TurnFailed {
		t.Fatal("native error after wake submit was treated as success")
	}
}

func TestCapRejectedWakeWaitsForKnownNativeReset(t *testing.T) {
	f := newStateFixture(t)
	f.input.command = "claude"
	a := f.add(t, "caller-id", "worker", "claude")
	a.Native.FailedTurn = "wake-turn"
	at := f.cmd.now()
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[string(a.ID)] = usageSnooze{ID: "old-wake", CallerID: a.ID, CallerName: a.Name, RecipientID: a.ID, TurnID: "wake-turn", SubmittedAt: at.Add(-time.Second), Note: "Resume saved work"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeSnoozeTurn(a, hookNotice{Kind: "turn-failed", TurnID: "wake-turn", Failure: "Usage limit reached"}, false); err != nil {
		t.Fatal(err)
	}
	state := usageSnapshot(t, f.run)
	if len(state.Snoozes) != 0 || !state.Recent[string(a.ID)].CapRejected {
		t.Fatalf("missing reset discarded rejected wake: %+v", state)
	}
	f.env["GANG_AGENT_ID"] = string(a.ID)
	f.env["TMUX_PANE"] = a.Pane
	if err := f.cmd.snooze([]string{"--status"}); err != nil || !strings.Contains(f.out.String(), "waiting for native reset") {
		t.Fatalf("pending reset status: %q, %v", f.out.String(), err)
	}
	stale := at.Add(-2 * time.Second)
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &stale, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 100, ResetAt: at.Add(5 * time.Hour).Unix()}}}
	if err := f.run.observeSnoozeTurn(a, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if got := usageSnapshot(t, f.run).Snoozes[string(a.ID)].ID; got != "" {
		t.Fatal("pre-submission native limit reading rearmed rejected wake")
	}
	a.Native.Limits.At = &at
	if err := f.run.observeSnoozeTurn(a, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if got := usageSnapshot(t, f.run).Snoozes[string(a.ID)].ID; got == "" {
		t.Fatal("new native reset did not rearm rejected wake")
	}
}

func TestUsageWindowKindNeedsNativeIdentification(t *testing.T) {
	for _, tc := range []struct {
		label   string
		minutes int
		want    string
	}{
		{"five_hour", 0, "five_hour"},
		{"seven_day", 0, "weekly"},
		{"codex/primary", 300, "five_hour"},
		{"codex/secondary", 10080, "weekly"},
		{"codex/primary", 0, ""},
		{"spend_limit", 0, ""},
		{"seven_day", 90, ""},
	} {
		if got := harness.UsageWindowKind(tc.label, tc.minutes); got != tc.want {
			t.Errorf("kind(%q, %d) = %q, want %q", tc.label, tc.minutes, got, tc.want)
		}
	}
}

func TestFailedNoticeRecipientDoesNotBlockOtherWakes(t *testing.T) {
	f := newStateFixture(t)
	lead := f.add(t, "lead-id", "lead", "claude")
	worker := f.add(t, "caller-id", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(worker.ID)
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Notices = []usageNotice{{ID: "usage-band-notice", Token: "0123456789abcdef", Text: "usage warning", CreatedAt: f.cmd.now(), ResetAt: f.cmd.now().Add(time.Hour).Unix(), RecipientID: lead.ID, RecipientName: lead.Name}}
		state.Snoozes[string(worker.ID)] = usageSnooze{ID: "snooze-wake", Token: "fedcba9876543210", CallerID: worker.ID, CallerName: worker.Name, At: f.cmd.now().Add(-time.Hour), Note: "Resume work"}
		state.Snoozes[string(lead.ID)] = usageSnooze{ID: "snooze-lead", Token: "00112233445566ff", CallerID: lead.ID, CallerName: lead.Name, At: f.cmd.now().Add(-time.Hour), Note: "Lead work"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// The screen is a Codex composer, which the lead's Claude collar cannot parse.
	err := f.run.flushUsageWork()
	state := usageSnapshot(t, f.run)
	if f.input.submits != 1 || !strings.Contains(f.input.pasted, "Resume work") || len(state.Snoozes) != 1 || state.Snoozes[string(lead.ID)].Submission != "" || len(state.Recent) != 1 {
		t.Fatalf("worker wake was not delivered past the lead failure: submits=%d text=%q err=%v", f.input.submits, f.input.pasted, err)
	}
	// The lead's wake waits behind its failed notice rather than overtaking it.
	if err == nil || !strings.Contains(err.Error(), "usage notice for lead: ") || strings.Count(err.Error(), "for lead") != 1 {
		t.Fatalf("lead input failure was not reported once with its recipient: %v", err)
	}
	if len(state.Notices) != 1 || state.Notices[0].Submission != "" {
		t.Fatalf("failed lead notice was not kept pending: %+v", state.Notices)
	}
}

// A warning describes one provider window. Once that window resets, the
// warning is false: no recipient may receive it, whenever one appears.
func TestUsageNoticeExpiresWithItsWindow(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "caller-id", "worker", "codex")
	c, err := loadCollar("codex", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	at := f.cmd.now()
	reset := at.Add(time.Hour)
	worker.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "primary", WindowMinutes: 300, UsedPercent: 80, ResetAt: reset.Unix()}}}
	if err := f.run.observeUsageBands(worker, c); err != nil {
		t.Fatal(err)
	}
	if len(usageSnapshot(t, f.run).Notices) != 1 {
		t.Fatal("band crossing recorded no notice")
	}
	f.cmd.clock = func() time.Time { return reset }
	f.run.cmd.clock = f.cmd.clock
	lead := f.add(t, "lead-id", "lead", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 {
		t.Fatalf("notice for a reset window was delivered: %q", f.input.pasted)
	}
	if n := usageSnapshot(t, f.run).Notices; len(n) != 0 {
		t.Fatalf("notice for a reset window was kept: %+v", n)
	}
}

// A warning already queued behind a busy recipient is withdrawn, not typed,
// once its window resets.
func TestQueuedUsageNoticeIsCancelledAtReset(t *testing.T) {
	f := newStateFixture(t)
	lead := f.add(t, "lead-id", "lead", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	c, err := loadCollar("codex", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	at := f.cmd.now()
	reset := at.Add(time.Hour)
	lead.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "primary", WindowMinutes: 300, UsedPercent: 80, ResetAt: reset.Unix()}}}
	if err := f.run.observeUsageBands(lead, c); err != nil {
		t.Fatal(err)
	}
	ready := f.input.screen
	f.input.screen = screenWithText("READY", "› draft")
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	p, err := f.run.team.Agent(lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 || len(queued) != 1 {
		t.Fatalf("notice did not wait behind the draft: submits=%d queued=%d", f.input.submits, len(queued))
	}
	// Withdrawal must leave an existing retained receipt in place.
	retained := core.Envelope{ID: "retained", Token: "fedcba9876543210", Recipient: lead.ID, To: lead.Name, From: core.Sender{Kind: core.SenderGangline, Name: "other"}, Message: core.Message{Text: "unconfirmed"}, CreatedAt: at}
	if err := p.Publish(retained); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	agent, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&agent, retained, "unverified", "native input not yet confirmed"); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	f.cmd.clock = func() time.Time { return reset }
	f.run.cmd.clock = f.cmd.clock
	f.input.screen = ready
	if _, err := f.run.drain(lead.ID, ""); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 {
		t.Fatalf("queued notice for a reset window was delivered: %q", f.input.pasted)
	}
	if queued, err := p.ListNew(); err != nil || len(queued) != 0 {
		t.Fatalf("queued notice was not withdrawn: %+v %v", queued, err)
	}
	if agent, err := p.Read(); err != nil || agent.LastFailed != retained.ID {
		t.Fatalf("withdrawal replaced the retained receipt: %q %v", agent.LastFailed, err)
	}
	if _, err := p.ReadEnvelope("failed", retained.ID); err != nil {
		t.Fatalf("withdrawal removed the retained receipt: %v", err)
	}
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if n := usageSnapshot(t, f.run).Notices; len(n) != 0 || f.input.submits != 0 {
		t.Fatalf("withdrawn notice was kept or republished: %+v submits=%d", n, f.input.submits)
	}
}

func TestQueuedWakeCanBeClearedOrReplaced(t *testing.T) {
	queued := func(t *testing.T) (*stateFixture, core.Agent, core.Agent, usageSnooze) {
		t.Helper()
		f := newStateFixture(t)
		caller := f.add(t, "caller-id", "worker", "codex")
		f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(caller.ID), caller.Pane
		if err := f.cmd.snooze([]string{"--at", "1h"}); err != nil {
			t.Fatal(err)
		}
		due := f.cmd.now().Add(2 * time.Hour)
		f.cmd.clock = func() time.Time { return due }
		f.run.cmd = f.cmd
		lead := f.add(t, "lead-id", "lead", "codex")
		f.input.screen = screenWithText("› an unsent operator draft")
		if err := f.run.flushUsageWork(); err != nil {
			t.Fatal(err)
		}
		s := usageSnapshot(t, f.run).Snoozes[string(caller.ID)]
		if s.RecipientID != caller.ID || s.Submission != "" || f.input.submits != 0 || len(inboxNew(t, f, caller)) != 1 {
			t.Fatalf("wake is not queued behind the draft: %+v", s)
		}
		return f, caller, lead, s
	}
	// Once the draft is gone, a withdrawn wake must not reach native input.
	assertWithdrawn := func(t *testing.T, f *stateFixture, recipient core.Agent, id core.EnvelopeID) {
		t.Helper()
		if q := inboxNew(t, f, recipient); len(q) != 0 {
			t.Fatalf("withdrawn wake still queued: %+v", q)
		}
		f.input.screen = screenWithText("READY", "› ")
		f.env["GANGLINE_HITCH_ID"] = string(recipient.ID)
		if err := f.run.flushUsageWork(); err != nil {
			t.Fatal(err)
		}
		if _, err := f.run.drain(recipient.ID, ""); err != nil {
			t.Fatal(err)
		}
		if f.input.submits != 0 {
			t.Fatalf("withdrawn wake was submitted: %q", f.input.pasted)
		}
	}
	t.Run("own clear", func(t *testing.T) {
		f, caller, _, s := queued(t)
		f.out.Reset()
		if err := f.cmd.snooze([]string{"--status"}); err != nil {
			t.Fatal(err)
		}
		if got := f.out.String(); !strings.Contains(got, string(s.ID)+"\tqueued for worker") {
			t.Fatalf("status does not show the queued wake: %q", got)
		}
		if err := f.cmd.snooze([]string{"--clear"}); err != nil {
			t.Fatal(err)
		}
		if state := usageSnapshot(t, f.run); len(state.Snoozes)+len(state.Recent) != 0 {
			t.Fatalf("clear retained the wake: %+v", state)
		}
		assertWithdrawn(t, f, caller, s.ID)
	})
	t.Run("busy recipient", func(t *testing.T) {
		f, caller, _, s := queued(t)
		p, _ := f.run.team.Agent(caller.ID)
		l, err := p.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		for _, args := range [][]string{{"--clear"}, {"--at", "3h"}} {
			if err := f.cmd.snooze(args); err == nil || !strings.Contains(err.Error(), "busy") {
				t.Fatalf("%v withdrew a wake under a held recipient lock: %v", args, err)
			}
		}
		if got := usageSnapshot(t, f.run).Snoozes[string(caller.ID)]; got.ID != s.ID || len(inboxNew(t, f, caller)) != 1 {
			t.Fatalf("refused withdrawal changed the wake: %+v", got)
		}
	})
	t.Run("recipient reconcile error", func(t *testing.T) {
		f, caller, _, s := queued(t)
		p, _ := f.run.team.Agent(caller.ID)
		l, err := p.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		a, _ := p.Read()
		a.LastFailed = "missing-receipt"
		if err := l.Save(a); err != nil {
			t.Fatal(err)
		}
		l.Close()
		if err := p.WriteWitness(store.Witness{ID: "w", At: f.cmd.now(), Prompt: "x"}); err != nil {
			t.Fatal(err)
		}
		if err := f.cmd.snooze([]string{"--clear"}); err == nil {
			t.Fatal("clear succeeded although the recipient could not be locked")
		}
		if got := usageSnapshot(t, f.run).Snoozes[string(caller.ID)]; got.ID != s.ID || len(inboxNew(t, f, caller)) != 1 {
			t.Fatalf("failed withdrawal changed the wake: %+v", got)
		}
	})
	t.Run("unverified startup receipt", func(t *testing.T) {
		f := newStateFixture(t)
		caller := f.add(t, "caller-id", "worker", "codex")
		f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(caller.ID), caller.Pane
		p, _ := f.run.team.Agent(caller.ID)
		caller.LastFailed = "original"
		if err := p.Publish(core.Envelope{ID: caller.LastFailed, Recipient: caller.ID, To: caller.Name, From: core.Sender{Kind: core.SenderGangline, Name: "hitch"}, Purpose: "assignment", Message: core.Message{Text: "original"}, CreatedAt: f.cmd.now()}); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(p.Inbox, "new", "original.json"), filepath.Join(p.Inbox, "failed", "original.json")); err != nil {
			t.Fatal(err)
		}
		l, err := p.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Save(caller); err != nil {
			t.Fatal(err)
		}
		l.Close()
		if err := f.cmd.snooze([]string{"--at", "1h"}); err != nil {
			t.Fatal(err)
		}
		due := f.cmd.now().Add(2 * time.Hour)
		f.cmd.clock = func() time.Time { return due }
		f.run.cmd = f.cmd
		f.input.screen = screenWithText("READY", "› ")
		if err := f.run.flushUsageWork(); err != nil {
			t.Fatal(err)
		}
		if len(inboxNew(t, f, caller)) != 1 || f.input.submits != 0 {
			t.Fatalf("wake is not queued behind the unverified startup receipt: submits=%d", f.input.submits)
		}
		if err := f.cmd.snooze([]string{"--clear"}); err != nil {
			t.Fatal(err)
		}
		if a, err := p.Read(); err != nil || a.LastFailed != "original" {
			t.Fatalf("withdrawal displaced the startup receipt: %+v %v", a, err)
		}
		delete(f.env, "GANG_AGENT_ID")
		delete(f.env, "TMUX_PANE")
		f.cmd.stdin = strings.NewReader("replacement assignment without contract")
		if err := f.cmd.send([]string{"worker", "--from", "operator"}); err == nil || !strings.Contains(err.Error(), "--recover") || f.input.submits != 0 {
			t.Fatalf("plain send after withdrawal err=%v submits=%d", err, f.input.submits)
		}
	})
	t.Run("replace", func(t *testing.T) {
		f, caller, _, s := queued(t)
		if err := f.cmd.snooze([]string{"--at", "3h"}); err != nil {
			t.Fatal(err)
		}
		next := usageSnapshot(t, f.run).Snoozes[string(caller.ID)]
		if next.ID == s.ID || next.RecipientID != "" || !next.At.Equal(f.cmd.now().Add(3*time.Hour)) {
			t.Fatalf("wake was not replaced: %+v", next)
		}
		assertWithdrawn(t, f, caller, s.ID)
	})
	t.Run("lead clear by ID", func(t *testing.T) {
		f := newStateFixture(t)
		caller := f.add(t, "caller-id", "worker", "codex")
		f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(caller.ID), caller.Pane
		if err := f.cmd.snooze([]string{"--at", "1h"}); err != nil {
			t.Fatal(err)
		}
		if err := f.run.team.RemoveName(caller.Name, caller.ID); err != nil {
			t.Fatal(err)
		}
		lead := f.add(t, "lead-id", "lead", "codex")
		f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(lead.ID), lead.Pane
		due := f.cmd.now().Add(2 * time.Hour)
		f.cmd.clock = func() time.Time { return due }
		f.run.cmd = f.cmd
		f.input.screen = screenWithText("› an unsent operator draft")
		if err := f.run.flushUsageWork(); err != nil {
			t.Fatal(err)
		}
		s := usageSnapshot(t, f.run).Snoozes[string(caller.ID)]
		if s.RecipientID != lead.ID || s.Submission != "" || len(inboxNew(t, f, lead)) != 1 {
			t.Fatalf("fallback wake is not queued for the lead: %+v", s)
		}
		f.out.Reset()
		if err := f.cmd.snooze([]string{"--status"}); err != nil {
			t.Fatal(err)
		}
		if got := f.out.String(); !strings.Contains(got, string(s.ID)+"\twake for worker; queued for lead") {
			t.Fatalf("lead status does not show the queued wake: %q", got)
		}
		if err := f.cmd.snooze([]string{"--clear", string(s.ID)}); err != nil {
			t.Fatal(err)
		}
		if state := usageSnapshot(t, f.run); len(state.Snoozes)+len(state.Recent) != 0 {
			t.Fatalf("clear retained the wake: %+v", state)
		}
		assertWithdrawn(t, f, lead, s.ID)
	})
}

func inboxNew(t *testing.T, f *stateFixture, a core.Agent) []core.Envelope {
	t.Helper()
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	q, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	return q
}
