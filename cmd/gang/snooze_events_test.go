package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// wakeEvents returns the wake lifecycle events that gang log --agent shows for
// name, in log order.
func wakeEvents(t *testing.T, f *stateFixture, name string) []core.Event {
	t.Helper()
	file, err := os.Open(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var filtered bytes.Buffer
	if err := writeFilteredLog(&filtered, file, logFilter{Agent: name}); err != nil {
		t.Fatal(err)
	}
	var events []core.Event
	if err := store.ReadLog(&filtered, func(e core.Event) error {
		if strings.HasPrefix(e.Type, "snooze_") {
			events = append(events, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return events
}

func wakeEventSummary(events []core.Event) string {
	var lines []string
	for _, e := range events {
		lines = append(lines, e.Type+" "+e.ID)
	}
	return strings.Join(lines, "\n")
}

func setClock(f *stateFixture, at time.Time) {
	f.cmd.clock = func() time.Time { return at }
	f.run.cmd = f.cmd
}

func TestLeadStatusShowsEveryTeammateWake(t *testing.T) {
	f := newStateFixture(t)
	lead := f.add(t, "lead-id", "lead", "codex")
	worker := f.add(t, "worker-id", "worker", "codex")
	reviewer := f.add(t, "reviewer-id", "reviewer", "codex")
	builder := f.add(t, "builder-id", "builder", "codex")
	now := f.cmd.now()
	f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(worker.ID), worker.Pane
	if err := f.cmd.snooze([]string{"--at", "2h", "--note", "Resume  from\n plan.md"}); err != nil {
		t.Fatal(err)
	}
	scheduled := usageSnapshot(t, f.run).Snoozes[string(worker.ID)]
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[string(reviewer.ID)] = usageSnooze{ID: "pending-wake", CallerID: reviewer.ID, CallerName: reviewer.Name, RecipientID: reviewer.ID, RecipientName: reviewer.Name, Submission: "accepted", TurnID: "wake-turn", At: now.Add(-time.Minute), Note: "Check the queue"}
		state.Snoozes[string(builder.ID)] = usageSnooze{ID: "queued-wake", CallerID: builder.ID, CallerName: builder.Name, RecipientID: builder.ID, RecipientName: builder.Name, At: now.Add(-time.Minute)}
		state.Snoozes["gone-id"] = usageSnooze{ID: "routed-wake", CallerID: "gone-id", CallerName: "gone", RecipientID: lead.ID, RecipientName: lead.Name, At: now.Add(-time.Minute)}
		state.Snoozes["tester-id"] = usageSnooze{ID: "sent-wake", CallerID: "tester-id", CallerName: "tester", RecipientID: "tester-id", RecipientName: "tester", Submission: "accepted", At: now.Add(-time.Minute)}
		state.Snoozes["capped-id"] = usageSnooze{ID: "auto-wake", CallerID: "capped-id", CallerName: "capped", Auto: true}
		state.Recent["limited-id"] = usageSnooze{ID: "limited-wake", CallerID: "limited-id", CallerName: "limited", RecipientID: "limited-id", RecipientName: "limited", TurnID: "t", CapCandidate: true, At: now.Add(-time.Minute)}
		state.Recent["left-id"] = usageSnooze{ID: "left-wake", CallerID: "left-id", CallerName: "left", RecipientID: lead.ID, RecipientName: lead.Name, TurnID: "t", CapCandidate: true, At: now.Add(-time.Minute)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(lead.ID), lead.Pane
	if err := f.cmd.snooze([]string{"--at", "3h"}); err != nil {
		t.Fatal(err)
	}
	own := usageSnapshot(t, f.run).Snoozes[string(lead.ID)].ID
	f.out.Reset()
	if err := f.cmd.snooze([]string{"--status"}); err != nil {
		t.Fatal(err)
	}
	got := f.out.String()
	for _, want := range []string{
		string(scheduled.ID) + "\twake for worker; scheduled; due 2026-09-22T12:00:00Z; note: Resume from plan.md\n",
		"pending-wake\twake for reviewer; awaiting successful native turn; due 2026-09-22T09:59:00Z; note: Check the queue\n",
		"queued-wake\twake for builder; queued for builder, not yet submitted; due 2026-09-22T09:59:00Z\n",
		"routed-wake\twake for gone; queued for lead, not yet submitted; due 2026-09-22T09:59:00Z; --clear ID withdraws it\n",
		"sent-wake\twake for tester; accepted in native queue; awaiting turn success; due 2026-09-22T09:59:00Z\n",
		"auto-wake\twake for capped; provider cap confirmed; awaiting native reset time\n",
		"limited-wake\twake for limited; rate limit unconfirmed by native usage; due 2026-09-22T09:59:00Z\n",
		"left-wake\twake for left; rate limit unconfirmed by native usage; inspect or clear; due 2026-09-22T09:59:00Z\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("lead status lacks %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, string(own)); n != 1 {
		t.Fatalf("lead status lists its own wake %d times:\n%s", n, got)
	}
	if n := strings.Count(got, "withdraws it"); n != 1 {
		t.Fatalf("lead status offers to clear %d teammate wakes, want only the one routed to it:\n%s", n, got)
	}
	if n := strings.Count(got, "inspect or clear"); n != 1 {
		t.Fatalf("lead status offers to clear %d cap-candidate wakes, want only the one routed to it:\n%s", n, got)
	}
}

func TestWakeLifecycleIsLogged(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "worker-id", "worker", "codex")
	key := string(worker.ID)
	start := f.cmd.now()
	f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = key, worker.Pane
	snooze := func(args ...string) core.EnvelopeID {
		t.Helper()
		if err := f.cmd.snooze(args); err != nil {
			t.Fatal(err)
		}
		return usageSnapshot(t, f.run).Snoozes[key].ID
	}
	a := snooze("--at", "1h")
	b := snooze("--at", "2h")
	snooze("--clear")
	c := snooze("--at", "1m")
	setClock(f, start.Add(time.Minute))
	f.env["GANGLINE_HITCH_ID"] = key
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if err := f.run.withUsageState(func(state *usageState) error {
		s := state.Recent[key]
		if s.ID != c {
			t.Fatalf("wake %s was not submitted: %+v", c, state)
		}
		s.TurnID = "wake-turn"
		state.Recent[key] = s
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeSnoozeTurn(worker, hookNotice{Kind: "turn-finished", TurnID: "wake-turn"}, false); err != nil {
		t.Fatal(err)
	}
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[key] = usageSnooze{ID: "failed-wake", CallerID: worker.ID, CallerName: worker.Name, RecipientID: worker.ID, RecipientName: worker.Name, TurnID: "failed-turn", At: start}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	worker.Native.FailedTurn = "failed-turn"
	for range 2 {
		if err := f.run.observeSnoozeTurn(worker, hookNotice{Kind: "turn-failed", TurnID: "failed-turn", Failure: "Login expired"}, false); err != nil {
			t.Fatal(err)
		}
	}
	e := snooze("--at", "1h")
	events := wakeEvents(t, f, "worker")
	want := strings.Join([]string{
		"snooze_scheduled " + string(a),
		"snooze_cleared " + string(a),
		"snooze_scheduled " + string(b),
		"snooze_cleared " + string(b),
		"snooze_scheduled " + string(c),
		"snooze_completed " + string(c),
		"snooze_failed failed-wake",
		"snooze_cleared failed-wake",
		"snooze_scheduled " + string(e),
	}, "\n")
	if got := wakeEventSummary(events); got != want {
		t.Fatalf("wake lifecycle log:\n%s\nwant:\n%s", got, want)
	}
	for _, event := range events {
		if event.HitchID != worker.ID || event.Name != worker.Name {
			t.Fatalf("wake event is not attributed to its caller: %+v", event)
		}
	}
	if !events[0].Deadline.Equal(start.Add(time.Hour)) || !events[4].Deadline.Equal(start.Add(time.Minute)) {
		t.Fatalf("scheduled events lack fire times: %+v %+v", events[0], events[4])
	}
	if !strings.Contains(events[1].Reason, string(b)) || !strings.Contains(events[7].Reason, string(e)) {
		t.Fatalf("replacement does not name its successor: %q %q", events[1].Reason, events[7].Reason)
	}
	if !strings.Contains(events[6].Reason, "Login expired") {
		t.Fatalf("failure reason = %q", events[6].Reason)
	}
}

func TestWakeRoutedToLeadIsLoggedUnderItsCaller(t *testing.T) {
	f := newStateFixture(t)
	caller := f.add(t, "caller-id", "worker", "codex")
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[string(caller.ID)] = usageSnooze{ID: "submitted-wake", Token: "0123456789abcdef", CallerID: caller.ID, CallerName: caller.Name, RecipientID: caller.ID, RecipientName: caller.Name, TurnID: "old-turn", At: f.cmd.now().Add(-time.Hour), Note: "Resume saved work"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.team.RemoveName(caller.Name, caller.ID); err != nil {
		t.Fatal(err)
	}
	lead := f.add(t, "lead-id", "lead", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	next := usageSnapshot(t, f.run).Recent[string(caller.ID)].ID
	f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(lead.ID), lead.Pane
	if err := f.cmd.snooze([]string{"--clear", string(next)}); err != nil {
		t.Fatal(err)
	}
	events := wakeEvents(t, f, "worker")
	if got, want := wakeEventSummary(events), "snooze_rearmed "+string(next)+"\nsnooze_cleared "+string(next); got != want {
		t.Fatalf("routed wake log:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(events[0].Reason, "submitted-wake") || !strings.Contains(events[1].Reason, "lead") {
		t.Fatalf("routed wake reasons: %q %q", events[0].Reason, events[1].Reason)
	}
}

func TestAutomaticWakeIsLoggedWhenItsResetBecomesKnown(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "caller-id", "worker", "claude")
	at := f.cmd.now()
	a.Native.FailedTurn = "native-turn"
	if err := f.run.observeAutoCap(a, hookNotice{Kind: "turn-failed", TurnID: "native-turn", At: at, Failure: "You have hit your usage limit"}); err != nil {
		t.Fatal(err)
	}
	// A reading older than the cap leaves the due time unknown and logs nothing.
	stale := at.Add(-time.Minute)
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &stale}
	for range 2 {
		if err := f.run.observeAutoCap(a, hookNotice{}); err != nil {
			t.Fatal(err)
		}
	}
	reset := at.Add(5 * time.Hour)
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 100, ResetAt: reset.Unix()}}}
	for range 2 {
		if err := f.run.observeAutoCap(a, hookNotice{}); err != nil {
			t.Fatal(err)
		}
	}
	id := string(usageSnapshot(t, f.run).Snoozes[string(a.ID)].ID)
	events := wakeEvents(t, f, "worker")
	if got, want := wakeEventSummary(events), "snooze_scheduled "+id+"\nsnooze_scheduled "+id; got != want {
		t.Fatalf("automatic wake log:\n%s\nwant:\n%s", got, want)
	}
	if !events[0].Deadline.IsZero() || !events[1].Deadline.Equal(reset) {
		t.Fatalf("automatic wake deadlines: %s then %s", events[0].Deadline, events[1].Deadline)
	}
}

func TestSupersededAndReroutedWakesAreLogged(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "worker-id", "worker", "codex")
	key := string(worker.ID)
	now := f.cmd.now()
	f.env["GANGLINE_HITCH_ID"] = key
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[key] = usageSnooze{ID: "orphaned", CallerID: worker.ID, CallerName: worker.Name, RecipientID: "gone-id", RecipientName: "gone", TurnID: "t", At: now.Add(-time.Hour)}
		state.Snoozes[key] = usageSnooze{ID: "submitted", Token: "0123456789abcdef", CallerID: worker.ID, CallerName: worker.Name, RecipientID: "gone-id", RecipientName: "gone", Submission: "accepted", At: now.Add(-time.Minute), Note: "Resume"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	next := usageSnapshot(t, f.run).Recent[key]
	if next.ID == "" || next.ID == "submitted" {
		t.Fatalf("rerouted wake was not submitted: %+v", next)
	}
	// A cap-rejected wake is dropped when a newer wake already exists.
	submitted := now.Add(-time.Minute)
	worker.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &now, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 100, ResetAt: now.Add(5 * time.Hour).Unix()}}}
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[key] = usageSnooze{ID: "rejected", CallerID: worker.ID, CallerName: worker.Name, RecipientID: worker.ID, RecipientName: worker.Name, TurnID: "t", CapRejected: true, SubmittedAt: submitted, At: submitted}
		state.Snoozes[key] = usageSnooze{ID: "newer", CallerID: worker.ID, CallerName: worker.Name, At: now.Add(time.Hour)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeSnoozeTurn(worker, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	events := wakeEvents(t, f, "worker")
	want := "snooze_cleared orphaned\nsnooze_rearmed " + string(next.ID) + "\nsnooze_cleared rejected"
	if got := wakeEventSummary(events); got != want {
		t.Fatalf("superseded wake log:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(events[0].Reason, "submitted") || !strings.Contains(events[1].Reason, "gone") || !strings.Contains(events[2].Reason, "newer") {
		t.Fatalf("superseded wake reasons: %q %q %q", events[0].Reason, events[1].Reason, events[2].Reason)
	}
}

func TestFailedWakeIsLoggedOnce(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "worker-id", "worker", "codex")
	key := string(worker.ID)
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[key] = usageSnooze{ID: "blocked-wake", CallerID: worker.ID, CallerName: worker.Name, RecipientID: worker.ID, RecipientName: worker.Name, TurnID: "wake-turn", At: f.cmd.now()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := f.run.observeSnoozeTurn(worker, hookNotice{Kind: "turn-finished", TurnID: "wake-turn"}, true); err != nil {
			t.Fatal(err)
		}
	}
	// A later unconfirmed rate limit on the same turn does not fail it again.
	worker.Native.FailedTurn = "wake-turn"
	if err := f.run.observeSnoozeTurn(worker, hookNotice{Kind: "turn-failed", TurnID: "wake-turn", Failure: "rate limit reached"}, false); err != nil {
		t.Fatal(err)
	}
	if !usageSnapshot(t, f.run).Recent[key].CapCandidate {
		t.Fatal("rate limit did not mark the wake a cap candidate")
	}
	setClock(f, f.cmd.now().Add(6*time.Minute))
	if err := f.run.observeSnoozeTurn(worker, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	events := wakeEvents(t, f, "worker")
	if got, want := wakeEventSummary(events), "snooze_failed blocked-wake"; got != want {
		t.Fatalf("blocked wake log:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(events[0].Reason, "provider blocked") {
		t.Fatalf("failure reason = %q", events[0].Reason)
	}
}
