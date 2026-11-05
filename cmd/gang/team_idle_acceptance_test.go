//go:build acceptance

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

type idleFixture struct {
	*stateFixture
	lead, worker core.Agent
}

func newIdleFixture(t *testing.T) *idleFixture {
	f := &idleFixture{stateFixture: newStateFixture(t)}
	// Existing continuity fixtures use a shorter explicit duration; the default
	// and full thirty-minute boundary are checked separately.
	f.env["GANG_TEAM_IDLE_MIN_DURATION"] = "2m"
	f.run.settings.TeamIdleMinDuration = 2 * time.Minute
	f.lead = f.addHitched(t, "lead-id", "coordinator", "lead", "")
	f.worker = f.addHitched(t, "worker-id", "worker", "", f.lead.ID)
	f.worker = f.setAgent(t, f.worker, func(a *core.Agent) { a.StatusProbeAt = f.cmd.now() })
	return f
}
func (f *idleFixture) sweep(t *testing.T) *idleSweep {
	t.Helper()
	agents, err := f.run.team.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.run.startIdleSweep(agents)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		s.observed[a.ID] = true
	}
	return s
}
func (f *idleFixture) reserve(t *testing.T) *store.IdleNotice {
	t.Helper()
	if err := f.run.reserveIdleNotice(f.sweep(t)); err != nil {
		t.Fatal(err)
	}
	n, err := f.run.readIdleNotice()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// advance keeps probe freshness separate from the activity evidence used by
// the timer. Five-second steps model the normal watchdog cadence without sleep.
func (f *idleFixture) advance(t *testing.T, delta time.Duration) {
	t.Helper()
	now := f.cmd.now().Add(delta)
	f.cmd.clock = func() time.Time { return now }
	f.run.cmd = f.cmd
	agents, err := f.run.team.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		f.setAgent(t, a, func(a *core.Agent) { a.StatusProbeAt = now })
	}
}
func (f *idleFixture) mature(t *testing.T) *store.IdleNotice {
	t.Helper()
	f.reserve(t)
	var n *store.IdleNotice
	for elapsed := time.Duration(0); elapsed < f.run.settings.TeamIdleMinDuration; {
		step := min(5*time.Second, f.run.settings.TeamIdleMinDuration-elapsed)
		f.advance(t, step)
		n = f.reserve(t)
		elapsed += step
	}
	return n
}
func (f *idleFixture) publication(t *testing.T, id string, due time.Time) core.Envelope {
	t.Helper()
	p, _ := f.run.team.Agent(f.worker.ID)
	e := core.Envelope{ID: core.EnvelopeID(id), Recipient: f.worker.ID, To: f.worker.Name, CreatedAt: f.cmd.now(), NotBefore: due, From: core.Sender{Kind: core.SenderAgent, HitchID: f.lead.ID}}
	if err := p.PublishNew(e); err != nil {
		t.Fatal(err)
	}
	if due.IsZero() {
		l, err := p.LockAgent()
		if err != nil {
			t.Fatal(err)
		}
		a, err := p.Read()
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Settle(&a, e, "delivered", ""); err != nil {
			t.Fatal(err)
		}
		_ = l.Close()
	}
	return e
}
func (f *idleFixture) work(t *testing.T, id string, due time.Time) core.Envelope {
	t.Helper()
	e := f.publication(t, id, due)
	if due.IsZero() {
		f.advance(t, time.Nanosecond)
		f.activity(t, core.Busy)
		f.activity(t, core.Idle)
	}
	return e
}
func TestTeamIdleLastWorkerNoRepeatRestartPruningAndRearm(t *testing.T) {
	f := newIdleFixture(t)
	if n := f.reserve(t); n != nil {
		t.Fatal("initial notice")
	}
	f.work(t, "work-one", time.Time{})
	f.worker = f.setAgent(t, f.worker, func(a *core.Agent) { a.Activity = core.Busy })
	if n := f.reserve(t); n != nil {
		t.Fatal("busy worker notified")
	}
	f.worker = f.setAgent(t, f.worker, func(a *core.Agent) { a.Activity = core.Idle })
	n := f.mature(t)
	if n == nil {
		t.Fatal("last idle worker did not notify")
	}
	if n.Envelope.Recipient != f.lead.ID {
		t.Fatal("lead was guessed from name")
	}
	// Use the production settlement seam before receipt pruning.
	p, _ := f.run.team.Agent(f.lead.ID)
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Publish(n.Envelope); err != nil {
		t.Fatal(err)
	}
	if err := f.run.apply(l, &a, core.Event{Type: "input_started", ID: string(n.Envelope.ID), Status: "envelope"}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.finishInput(l, &a, n.Envelope, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	next := core.Envelope{ID: "lead-new", Recipient: a.ID}
	if err := p.PublishNew(next); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&a, next, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	if _, err := p.ReadEnvelope("cur", n.Envelope.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("notice receipt not pruned: %v", err)
	}
	f.run, err = f.cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	for _, turn := range []string{"new", "old", "new"} {
		f.worker = f.agent(t, f.worker.ID)
		f.worker = f.setAgent(t, f.worker, func(a *core.Agent) { a.Native.TurnID = turn; a.Activity = core.Busy })
		f.reserve(t)
		f.worker = f.setAgent(t, f.worker, func(a *core.Agent) { a.Activity = core.Idle })
		if got := f.reserve(t); got == nil || got.Envelope.ID != n.Envelope.ID {
			t.Fatal("callback/flicker rearmed")
		}
	}
	f.work(t, "work-two", time.Time{})
	if got := f.mature(t); got == nil || got.Envelope.ID == n.Envelope.ID {
		t.Fatal("new completed input between sweeps did not rearm")
	}
}
func TestTeamIdleSnapshotRacesAndBlockers(t *testing.T) {
	for _, mode := range []string{"publication", "lock", "incomplete", "due", "accepted", "unverified", "permission", "pending-hook", "stale", "future", "unknown", "blocked", "clear", "outside"} {
		t.Run(mode, func(t *testing.T) {
			f := newIdleFixture(t)
			f.reserve(t)
			f.work(t, "work", time.Time{})
			f.reserve(t)
			for range 23 {
				f.advance(t, 5*time.Second)
				f.reserve(t)
			}
			f.advance(t, 5*time.Second)
			s := f.sweep(t)
			p, _ := f.run.team.Agent(f.worker.ID)
			switch mode {
			case "publication":
				f.work(t, "racing", time.Time{})
			case "lock":
				l, err := p.LockAgent()
				if err != nil {
					t.Fatal(err)
				}
				defer l.Close()
			case "incomplete":
				delete(s.observed, f.worker.ID)
			case "due":
				e := core.Envelope{ID: "due", Recipient: f.worker.ID}
				if err := p.Publish(e); err != nil {
					t.Fatal(err)
				}
			case "accepted", "unverified":
				e := core.Envelope{ID: "uncertain", Recipient: f.worker.ID}
				if err := p.Publish(e); err != nil {
					t.Fatal(err)
				}
				l, err := p.LockAgent()
				if err != nil {
					t.Fatal(err)
				}
				a, _ := p.Read()
				if err := l.Settle(&a, e, mode, ""); err != nil {
					t.Fatal(err)
				}
				_ = l.Close()
			case "permission":
				if err := p.WritePermissionWitness(store.PermissionWitness{At: f.cmd.now()}); err != nil {
					t.Fatal(err)
				}
			case "pending-hook":
				if err := p.WriteStatusHook(store.StatusHook{At: f.cmd.now(), Kind: "turn-started"}); err != nil {
					t.Fatal(err)
				}
			case "stale":
				f.setAgent(t, f.worker, func(a *core.Agent) { a.StatusProbeAt = f.cmd.now().Add(-time.Minute) })
			case "future":
				f.setAgent(t, f.worker, func(a *core.Agent) { a.StatusProbeAt = f.cmd.now().Add(time.Second) })
			case "unknown":
				f.setAgent(t, f.worker, func(a *core.Agent) { a.Activity = core.Unknown })
			case "blocked":
				f.setAgent(t, f.worker, func(a *core.Agent) { a.Activity = core.Blocked })
			case "clear":
				f.setAgent(t, f.worker, func(a *core.Agent) { a.Clear = &core.ClearRequest{Status: "pending"} })
			case "outside":
				_, err := (store.Paths{Root: f.run.settings.StateRoot}).RegisterOutside(f.run.settings.Session, "mailbox", filepath.Join(t.TempDir(), "synthetic-key"), core.CredentialOrigin{At: f.cmd.now(), Command: "outside register", Actor: core.CredentialActor{Kind: "operator_shell"}})
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := f.run.reserveIdleNotice(s); err != nil {
				t.Fatal(err)
			}
			if n, err := f.run.readIdleNotice(); err != nil || n != nil {
				t.Fatalf("blocker %s notice=%+v error=%v", mode, n, err)
			}
		})
	}
}
func TestTeamIdleFutureMixedMembershipAndRecipientReplacement(t *testing.T) {
	f := newIdleFixture(t)
	other := f.addHitched(t, "nested-id", "nested", "", f.worker.ID)
	f.setAgent(t, other, func(a *core.Agent) { a.Collar = "claude"; a.StatusProbeAt = f.cmd.now() })
	f.reserve(t)
	f.work(t, "observed-work", time.Time{})
	e := f.work(t, "future", f.cmd.now().Add(time.Hour))
	n := f.mature(t)
	if n == nil {
		t.Fatal("future work vetoed")
	}
	p, _ := f.run.team.Agent(f.worker.ID)
	got, err := p.ReadEnvelope("new", e.ID)
	if err != nil || !got.NotBefore.Equal(e.NotBefore) {
		t.Fatal("future work advanced")
	}
	// Replacing the recipient never redirects an already reserved intent.
	f.setAgent(t, f.lead, func(a *core.Agent) { a.Registration.Generation = "replacement" })
	if err := f.run.flushIdleNotice(); err != nil {
		t.Fatal(err)
	}
	gotNotice, err := f.run.readIdleNotice()
	if err != nil || gotNotice.Outcome != "recipient-changed" {
		t.Fatalf("replacement: %+v %v", gotNotice, err)
	}
	leadPaths, _ := f.run.team.Agent(f.lead.ID)
	queued, err := leadPaths.ListNew()
	if err != nil || len(queued) != 0 {
		t.Fatalf("redirected notice: %+v %v", queued, err)
	}
}

func TestTeamIdlePublicationRecoveryUsesNormalDeliveryOnce(t *testing.T) {
	f := newIdleFixture(t)
	f.reserve(t)
	f.work(t, "work", time.Time{})
	n := f.mature(t)
	if n == nil {
		t.Fatal("missing intent")
	}
	f.env["GANGLINE_HITCH_ID"] = string(f.lead.ID)
	p, _ := f.run.team.Agent(f.lead.ID)
	// A process exited immediately after publication; the next flush must
	// reconcile the identical envelope, then use normal safe delivery.
	if err := p.Publish(n.Envelope); err != nil {
		t.Fatal(err)
	}
	if err := f.run.flushIdleNotice(); err != nil {
		t.Fatal(err)
	}
	got, err := f.run.readIdleNotice()
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != "delivered" {
		t.Fatalf("normal delivery outcome=%q", got.Outcome)
	}
	if err := f.run.flushIdleNotice(); err != nil {
		t.Fatal(err)
	}
	pending, err := p.ListNew()
	if err != nil || len(pending) != 0 {
		t.Fatalf("repeat queued: %+v %v", pending, err)
	}
}

func TestTeamIdleExplicitWakePublicationOnly(t *testing.T) {
	for _, mode := range []string{"explicit", "auto", "replacement"} {
		t.Run(mode, func(t *testing.T) {
			f := newIdleFixture(t)
			f.env["GANGLINE_HITCH_ID"] = string(f.worker.ID)
			f.reserve(t)
			wake := usageSnooze{ID: "wake", Token: "0123456789abcdef", CallerID: f.worker.ID, CallerName: f.worker.Name, RecipientID: f.worker.ID, RecipientName: f.worker.Name, At: f.cmd.now().Add(-time.Second), Note: "continue"}
			if mode == "auto" {
				wake.Auto = true
			}
			if mode == "replacement" {
				wake.Rearms = 1
			}
			if err := f.run.withUsageState(func(s *usageState) error { s.Snoozes[string(f.worker.ID)] = wake; return nil }); err != nil {
				t.Fatal(err)
			}
			e := core.Envelope{ID: wake.ID, Token: wake.Token, Recipient: wake.RecipientID, To: wake.RecipientName, From: core.Sender{Kind: core.SenderGangline, Name: "snooze"}, Message: core.Message{Text: snoozeWakeText(wake, f.cmd.now())}, CreatedAt: f.cmd.now()}
			if _, err := f.run.publishUsageEnvelope(e); err != nil {
				t.Fatal(err)
			}
			l, err := f.run.team.LockIdle()
			if err != nil {
				t.Fatal(err)
			}
			s, err := l.Read()
			_ = l.Close()
			if err != nil {
				t.Fatal(err)
			}
			want := uint64(0)
			if mode == "explicit" {
				want = 1
			}
			if s.Work[f.worker.ID] != want {
				t.Fatalf("work=%d want %d", s.Work[f.worker.ID], want)
			}
		})
	}
}

// Full ticks probe workers concurrently. Serialize the shared fake backend's
// capture counter without changing production concurrency.
type idleSweepInput struct {
	*inputFixture
	mu sync.Mutex
}

func (b *idleSweepInput) Capture(ctx context.Context, pane substrate.PaneID) (substrate.Screen, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.inputFixture.Capture(ctx, pane)
}

func TestTeamIdleFullSweepConcurrentWorkers(t *testing.T) {
	f := newIdleFixture(t)
	backend := &idleSweepInput{inputFixture: f.input}
	f.cmd.paneBackend, f.cmd.inputBackend = backend, backend
	f.run.cmd = f.cmd
	f.env["GANGLINE_HITCH_ID"] = string(f.lead.ID)
	for _, a := range []core.Agent{f.lead, f.worker, f.addHitched(t, "other", "other", "", f.worker.ID)} {
		f.setAgent(t, a, func(a *core.Agent) {
			a.Native.HookAt = f.cmd.now()
			a.Native.HookKind = "turn-finished"
			a.Native.FinishedAt = f.cmd.now()
		})
	}
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if n, err := f.run.readIdleNotice(); err != nil || n != nil {
		t.Fatalf("baseline %+v %v", n, err)
	}
	f.work(t, "full-sweep-work", time.Time{})
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if n, err := f.run.readIdleNotice(); err != nil || n != nil {
		t.Fatalf("single full sweep %+v %v", n, err)
	}
	// Allow the initial hook observation to age into the pane fallback, then
	// prove a quiet worker can mature across 5-second ticks without field churn.
	var baseline core.Agent
	for step := 1; step <= 36; step++ {
		now := f.cmd.now().Add(5 * time.Second)
		f.cmd.clock = func() time.Time { return now }
		f.run.cmd = f.cmd
		if err := f.cmd.tick(nil); err != nil {
			t.Fatal(err)
		}
		p, _ := f.run.team.Agent(f.worker.ID)
		a, err := p.Read()
		if err != nil {
			t.Fatal(err)
		}
		if step == 12 {
			baseline = a
		}
		if step > 12 && (!a.ChangedAt.Equal(baseline.ChangedAt) || !a.ScreenSince.Equal(baseline.ScreenSince)) {
			t.Fatalf("quiet worker evidence churn on tick %d: before=%+v after=%+v", step, baseline, a)
		}
	}
	if n, err := f.run.readIdleNotice(); err != nil || n == nil || n.Outcome != "delivered" {
		t.Fatalf("full sweep %+v %v", n, err)
	}
	t.Log("36 real full ticks at fake 5-second spacing; quiet-worker evidence stable for final 2m")
}

func TestTeamIdleUncertainOldRecipientDoesNotBlockNewWork(t *testing.T) {
	for _, outcome := range []string{"accepted", "unverified"} {
		t.Run(outcome, func(t *testing.T) {
			f := newIdleFixture(t)
			f.reserve(t)
			f.work(t, "one", time.Time{})
			n := f.mature(t)
			if err := f.run.settleIdleNotice(n.Envelope, outcome); err != nil {
				t.Fatal(err)
			}
			f.lead = f.setAgent(t, f.lead, func(a *core.Agent) { a.Registration.Generation = "new" })
			if err := f.run.flushIdleNotice(); err != nil {
				t.Fatal(err)
			}
			f.reserve(t) // new membership baseline
			f.work(t, "two", time.Time{})
			if next := f.mature(t); next == nil || next.Envelope.ID == n.Envelope.ID {
				t.Fatal("old uncertainty blocked new episode")
			}
		})
	}
}

func TestTeamIdleQueuedNoticeCannotReachReplacementViaDrain(t *testing.T) {
	f := newIdleFixture(t)
	f.reserve(t)
	f.work(t, "work", time.Time{})
	n := f.mature(t)
	p, _ := f.run.team.Agent(f.lead.ID)
	if err := p.Publish(n.Envelope); err != nil {
		t.Fatal(err)
	}
	f.lead = f.setAgent(t, f.lead, func(a *core.Agent) { a.Registration.Generation = "replacement" })
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.run.drainLocked(l, &f.lead, ""); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	if f.input.submits != 0 {
		t.Fatal("notice delivered to replacement")
	}
	pending, err := p.ListNew()
	if err != nil || len(pending) != 0 {
		t.Fatalf("old notice queued %+v %v", pending, err)
	}
	n, err = f.run.readIdleNotice()
	if err != nil || n.Outcome != "recipient-changed" {
		t.Fatalf("intent %+v %v", n, err)
	}
}

func TestTeamIdleCrashDuringInputNeverReplays(t *testing.T) {
	f := newIdleFixture(t)
	f.reserve(t)
	f.work(t, "work", time.Time{})
	n := f.mature(t)
	p, _ := f.run.team.Agent(f.lead.ID)
	if err := p.Publish(n.Envelope); err != nil {
		t.Fatal(err)
	}
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.apply(l, &a, core.Event{Type: "input_started", ID: string(n.Envelope.ID), Status: "envelope"}); err != nil {
		t.Fatal(err)
	}
	_ = l.Close() // no native outcome was saved
	if err := f.run.flushIdleNotice(); err != nil {
		t.Fatal(err)
	}
	n, err = f.run.readIdleNotice()
	if err != nil || n.Outcome != "unverified" {
		t.Fatalf("intent %+v %v", n, err)
	}
	if f.input.submits != 0 {
		t.Fatal("uncertain native submission replayed")
	}
}

func TestTeamIdleMissingRegistrationAndDropDoNotComplete(t *testing.T) {
	f := newIdleFixture(t)
	f.reserve(t)
	f.work(t, "work", time.Time{})
	if err := f.run.team.ClaimName("missing", "missing-id"); err != nil {
		t.Fatal(err)
	}
	if n := f.reserve(t); n != nil {
		t.Fatal("missing registered worker was excluded")
	}
	if err := f.run.team.RemoveName("missing", "missing-id"); err != nil {
		t.Fatal(err)
	}
	other := f.addHitched(t, "second", "second", "", f.worker.ID)
	f.setAgent(t, other, func(a *core.Agent) { a.StatusProbeAt = f.cmd.now(); a.Activity = core.Busy })
	f.reserve(t) // membership baseline, including busy worker
	f.work(t, "new-work", time.Time{})
	if n := f.reserve(t); n != nil {
		t.Fatal("busy worker excluded")
	}
	if err := f.run.team.RemoveName(other.Name, other.ID); err != nil {
		t.Fatal(err)
	}
	if n := f.reserve(t); n != nil {
		t.Fatal("drop manufactured completion")
	}
}

func TestTeamIdleConcurrentReservations(t *testing.T) {
	f := newIdleFixture(t)
	f.reserve(t)
	f.work(t, "work", time.Time{})
	f.reserve(t)
	for elapsed := time.Duration(0); elapsed < f.run.settings.TeamIdleMinDuration-5*time.Second; elapsed += 5 * time.Second {
		f.advance(t, 5*time.Second)
		f.reserve(t)
	}
	f.advance(t, 5*time.Second)
	first, second := f.sweep(t), f.sweep(t)
	results := make(chan error, 2)
	go func() { results <- f.run.reserveIdleNotice(first) }()
	go func() { results <- f.run.reserveIdleNotice(second) }()
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	n, err := f.run.readIdleNotice()
	if err != nil || n == nil {
		t.Fatalf("missing shared intent %+v %v", n, err)
	}
	if next := f.reserve(t); next.Envelope.ID != n.Envelope.ID {
		t.Fatal("concurrent reservations duplicated notice")
	}
}
