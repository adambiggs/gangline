package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// backgroundIdleFixture is a worker hitched by a lead, with the lead's
// background-idle notices and wakes and the worker's background_idle events
// readable.
type backgroundIdleFixture struct {
	*stateFixture
	lead, worker core.Agent
	woken        []string
}

func newBackgroundIdleFixture(t *testing.T, collar string) *backgroundIdleFixture {
	t.Helper()
	f := &backgroundIdleFixture{stateFixture: newStateFixture(t)}
	f.lead = f.addHitched(t, "l", "lead", "lead", "")
	f.worker = f.setAgent(t, f.add(t, "a", "worker", collar), func(a *core.Agent) { a.HitchedBy = f.lead.ID })
	f.env["GANGLINE_HITCH_ID"] = string(f.worker.ID)
	f.cmd.detach = func(id string, _ hookNotice) error { f.woken = append(f.woken, id); return nil }
	f.run.cmd = f.cmd
	return f
}

func (f *backgroundIdleFixture) notices(t *testing.T) []core.Envelope {
	t.Helper()
	lp, err := f.run.team.Agent(f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(lp.Inbox, "new"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var found []core.Envelope
	for _, entry := range entries {
		e, err := lp.ReadEnvelope("new", core.EnvelopeID(strings.TrimSuffix(entry.Name(), ".json")))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(string(e.ID), "background-idle-a-") {
			found = append(found, e)
		}
	}
	return found
}

func (f *backgroundIdleFixture) events(t *testing.T) []core.Event {
	t.Helper()
	file, err := os.Open(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var found []core.Event
	if err := store.ReadLog(file, func(e core.Event) error {
		if e.Type == "background_idle" {
			found = append(found, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return found
}

// settle delivers e to the lead, as its inbox records a delivery.
func (f *backgroundIdleFixture) settle(t *testing.T, e core.Envelope) {
	t.Helper()
	lp, err := f.run.team.Agent(f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if published, err := envelopePublished(lp, e.ID); err != nil {
		t.Fatal(err)
	} else if !published {
		if err := lp.Publish(e); err != nil {
			t.Fatal(err)
		}
	}
	l, err := lp.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	lead, err := lp.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&lead, e, "delivered", ""); err != nil {
		t.Fatal(err)
	}
}

func (f *backgroundIdleFixture) tickAt(t *testing.T, at time.Time) {
	t.Helper()
	f.run.cmd.clock = func() time.Time { return at }
	if err := f.run.tickAgent(f.worker.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
}

// An agent that stays idle for the threshold after a turn ended with native
// background tasks pending is reported to its hitcher once per such turn,
// with an event in the log, since it reads as idle throughout and nothing
// else reaches the hitcher.
func TestBackgroundIdleNotifiesHitcherOncePerTurn(t *testing.T) {
	f := newBackgroundIdleFixture(t, "claude")
	f.input.command = "claude"
	f.input.screen = screenWithText("────────", "❯ ", "────────")
	p, err := f.run.team.Agent(f.worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	threshold := 20 * time.Minute
	f.run.settings.BackgroundIdleNotice = threshold
	start := f.cmd.now()
	if err := p.WriteBackground(store.Background{At: start, Tasks: 1}); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		at   time.Duration
		want int
	}{
		{0, 0},
		{threshold - time.Millisecond, 0},
		{threshold, 1},
		{3 * threshold, 1},
	} {
		f.tickAt(t, start.Add(step.at))
		if got := f.notices(t); len(got) != step.want || len(f.woken) != step.want || len(f.events(t)) != step.want {
			t.Fatalf("at %s: notices = %+v, woken = %v, events = %+v", step.at, got, f.woken, f.events(t))
		}
	}
	notice := f.notices(t)[0]
	if notice.From.Kind != core.SenderGangline || notice.Recipient != f.lead.ID || !strings.Contains(notice.Message.Text, "worker, which you hitched, has been idle since 2026-09-22T10:00:00Z with 1 background task pending") {
		t.Fatalf("notice = %+v", notice)
	}
	if event := f.events(t)[0]; event.HitchID != f.worker.ID || event.ID != string(notice.ID) || event.Reason != "idle since 2026-09-22T10:00:00Z with 1 background task pending" {
		t.Fatalf("event = %+v", event)
	}

	// The hitcher receives the notice and then another message. Its inbox
	// keeps only the latest delivered receipt, and the stretch is still
	// announced once.
	f.settle(t, notice)
	f.settle(t, core.Envelope{ID: "later", Recipient: f.lead.ID, To: f.lead.Name, From: core.Sender{Kind: core.SenderGangline, Name: "test"}, Message: core.Message{Text: "later"}, CreatedAt: start})
	f.tickAt(t, start.Add(3*threshold))
	if len(f.woken) != 1 || len(f.events(t)) != 1 || len(f.notices(t)) != 0 {
		t.Fatalf("after the hitcher's later delivery: woken = %v, events = %+v, notices = %+v", f.woken, f.events(t), f.notices(t))
	}

	// A later turn that ends with tasks pending is a new idle stretch.
	later := start.Add(4 * threshold)
	if err := p.WriteBackground(store.Background{At: later, Tasks: 2}); err != nil {
		t.Fatal(err)
	}
	f.tickAt(t, later.Add(threshold-time.Millisecond))
	if len(f.woken) != 1 || len(f.events(t)) != 1 {
		t.Fatalf("before the second threshold: woken = %v, events = %+v", f.woken, f.events(t))
	}
	f.tickAt(t, later.Add(threshold))
	if got := f.notices(t); len(got) != 1 || len(f.woken) != 2 || len(f.events(t)) != 2 || !strings.Contains(got[0].Message.Text, "with 2 background tasks pending") {
		t.Fatalf("after the second threshold: notices = %+v, woken = %v, events = %+v", got, f.woken, f.events(t))
	}
}

// A busy agent, an idle one with no tasks pending, and one with no hitcher
// produce no notice and no event however long they stay that way.
func TestBackgroundIdleSkipsBusyUnpendingAndUnhitched(t *testing.T) {
	for _, tc := range []struct {
		name   string
		screen []string
		tasks  int
		orphan bool
	}{
		{"busy", []string{"✻ Working… (3s · esc to interrupt)", "────────", "❯ ", "────────"}, 1, false},
		{"no tasks", []string{"────────", "❯ ", "────────"}, 0, false},
		{"no hitcher", []string{"────────", "❯ ", "────────"}, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newBackgroundIdleFixture(t, "claude")
			f.input.command = "claude"
			f.input.screen = screenWithText(tc.screen...)
			if tc.orphan {
				f.worker = f.setAgent(t, f.worker, func(a *core.Agent) { a.HitchedBy = "" })
			}
			p, err := f.run.team.Agent(f.worker.ID)
			if err != nil {
				t.Fatal(err)
			}
			start := f.cmd.now()
			if tc.tasks > 0 {
				if err := p.WriteBackground(store.Background{At: start, Tasks: tc.tasks}); err != nil {
					t.Fatal(err)
				}
			}
			f.tickAt(t, start.Add(10*f.run.settings.BackgroundIdleNotice))
			if got := f.notices(t); len(got) != 0 || len(f.events(t)) != 0 || f.errOut.String() != "" {
				t.Fatalf("notices = %+v, events = %+v, stderr = %q, worker = %+v", got, f.events(t), f.errOut.String(), f.agent(t, f.worker.ID))
			}
		})
	}
}

// A tick that judges status from fresh hook evidence, without a pane capture,
// also sends the notice, so a short threshold is honored between captures.
func TestBackgroundIdleNoticeFromFreshHooks(t *testing.T) {
	f := newBackgroundIdleFixture(t, "claude")
	threshold := statusHookFreshness / 2
	f.run.settings.BackgroundIdleNotice = threshold
	start := f.cmd.now()
	payload, err := json.Marshal(map[string]any{"hook_event_name": "Stop", "session_id": "s", "background_tasks": []map[string]string{{"id": "b1", "type": "shell", "status": "running"}}})
	if err != nil {
		t.Fatal(err)
	}
	hook := f.cmd
	hook.stdin = bytes.NewReader(payload)
	if err := hook.handleHook(nil); err != nil {
		t.Fatal(err)
	}
	f.tickAt(t, start.Add(threshold-time.Millisecond))
	if got := f.notices(t); len(got) != 0 {
		t.Fatalf("before the threshold: notices = %+v", got)
	}
	f.tickAt(t, start.Add(threshold))
	if got := f.notices(t); len(got) != 1 || len(f.events(t)) != 1 {
		t.Fatalf("notices = %+v, events = %+v", got, f.events(t))
	}
	if f.input.captures != 0 {
		t.Fatalf("fresh hooks caused %d captures", f.input.captures)
	}
}

// Roster and status show how long an agent has been idle with tasks pending,
// and the JSON forms carry when that began.
func TestBackgroundIdleDurationInRosterAndStatus(t *testing.T) {
	f := newBackgroundIdleFixture(t, "claude")
	f.input.command = "claude"
	f.input.screen = screenWithText("────────", "❯ ", "────────")
	p, err := f.run.team.Agent(f.worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	start := f.cmd.now()
	if err := p.WriteBackground(store.Background{At: start, Tasks: 1}); err != nil {
		t.Fatal(err)
	}
	now := start.Add(7*time.Hour + 9*time.Minute + 48*time.Second)
	f.cmd.clock = func() time.Time { return now }
	read := func(args ...string) string {
		t.Helper()
		f.out.Reset()
		if err := f.cmd.execute(args); err != nil {
			t.Fatal(err)
		}
		return f.out.String()
	}
	if out := read("roster"); !strings.Contains(out, "worker           active     idle (1 bg 7h09m)  claude") {
		t.Fatalf("roster = %q", out)
	}
	if out := read("status", "worker", "--why"); !strings.Contains(out, "\tidle (1 bg 7h09m)\n") || !strings.Contains(out, "idle since 2026-09-22T10:00:00Z (7h09m) with 1 background task pending\n") {
		t.Fatalf("status --why = %q", out)
	}
	f.out.Reset()
	if err := f.cmd.execute([]string{"status", "worker", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got agentJSON
	decodeOutput(t, f.stateFixture, &got)
	if got.BackgroundTasks != 1 || got.BackgroundSince == nil || !got.BackgroundSince.Equal(start) {
		t.Fatalf("status --json = %+v", got)
	}
}

func TestIdleDuration(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{0, "<1m"},
		{time.Minute - time.Nanosecond, "<1m"},
		{time.Minute, "1m"},
		{59*time.Minute + 59*time.Second, "59m"},
		{time.Hour, "1h00m"},
		{25*time.Hour + 5*time.Minute, "25h05m"},
	} {
		if got := idleDuration(tc.d); got != tc.want {
			t.Errorf("idleDuration(%s) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestBackgroundIdleNoticeSetting(t *testing.T) {
	f := newStateFixture(t)
	settings, err := f.cmd.settings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.BackgroundIdleNotice != 30*time.Minute {
		t.Fatalf("default = %s", settings.BackgroundIdleNotice)
	}
	f.env["GANG_BACKGROUND_IDLE_NOTICE"] = "10m"
	if settings, err = f.cmd.settings(); err != nil || settings.BackgroundIdleNotice != 10*time.Minute || settings.Origins["GANG_BACKGROUND_IDLE_NOTICE"] != "env" {
		t.Fatalf("settings = %+v, %v", settings, err)
	}
	for _, value := range []string{"0s", "-1m", "soon"} {
		f.env["GANG_BACKGROUND_IDLE_NOTICE"] = value
		if _, err := f.cmd.settings(); err == nil || !strings.Contains(err.Error(), "GANG_BACKGROUND_IDLE_NOTICE must be a positive duration") {
			t.Fatalf("%q: err = %v", value, err)
		}
	}
}

// A watchdog tick runs outside the caller's environment, so the threshold the
// caller selected is carried to it, and every carried value is one the
// schedulers forward.
func TestBackgroundIdleNoticeReachesWatchdogTicks(t *testing.T) {
	f, s := watchdogFixture(t)
	f.env["GANG_BACKGROUND_IDLE_NOTICE"] = "7m"
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if got := s.environment["GANG_BACKGROUND_IDLE_NOTICE"]; got != "7m0s" {
		t.Fatalf("watchdog environment carries %q", got)
	}
	for key := range s.environment {
		if !slices.Contains(watchdogEnvironmentKeys, key) {
			t.Errorf("schedulers do not forward %s", key)
		}
	}
}
