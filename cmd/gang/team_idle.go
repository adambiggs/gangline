package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

type idleSweep struct {
	mu       sync.Mutex
	start    time.Time
	agents   []core.Agent
	outside  []store.OutsideRegistration
	work     map[core.HitchID]uint64
	observed map[core.HitchID]bool
}

func (s *idleSweep) observedAgent(id core.HitchID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observed[id] = true
}

func (run *runtime) startIdleSweep(agents []core.Agent) (*idleSweep, error) {
	outside, err := run.team.ListOutside()
	if err != nil {
		return nil, err
	}
	l, err := run.team.LockIdle()
	if err != nil {
		return nil, err
	}
	defer l.Close()
	s, err := l.Read()
	if err != nil {
		return nil, err
	}
	return &idleSweep{start: run.cmd.now(), agents: agents, outside: outside, work: maps.Clone(s.Work), observed: make(map[core.HitchID]bool)}, nil
}

func idleMember(a core.Agent, work uint64) store.IdleMember {
	return store.IdleMember{ID: a.ID, Name: a.Name, Generation: a.Registration.Generation, Session: a.Native.SessionID, Work: work}
}
func sameIdleIdentity(a, b store.IdleMember) bool {
	a.Work, b.Work = 0, 0
	return a == b
}

// advanceIdleEpisode is independent of native turn IDs and activity flicker.
// Changes of membership establish a baseline, never completion by subtraction.
func advanceIdleEpisode(s *store.IdleState, lead store.IdleMember, members []store.IdleMember) {
	same := s.Initialized && sameIdleIdentity(s.Lead, lead) && len(s.Members) == len(members)
	if same {
		for i := range members {
			if !sameIdleIdentity(s.Members[i], members[i]) {
				same = false
				break
			}
		}
	}
	if !same {
		s.Armed = false
	} else {
		for i := range members {
			if members[i].Work > s.Members[i].Work {
				s.Armed = true
			}
		}
	}
	s.Initialized = true
	s.Lead = lead
	s.Members = members
}

func idleObservation(a core.Agent, now time.Time) bool {
	if a.Status != core.Active || a.Activity != core.Idle || a.Pane == "" || a.Input != nil || a.InputOutage != nil || a.Rehitch != nil || a.Registration.Held || a.Native.TurnFailure != "" || a.Capacity.Fingerprint != "" || !a.InterruptDeadline.IsZero() || clearPending(a) {
		return false
	}
	if a.Compaction != nil && a.Compaction.Status != "completed" {
		return false
	}
	if a.Evidence == heldInputEvidence {
		return false
	}
	at := a.Native.HookAt
	if a.StatusProbeAt.After(at) {
		at = a.StatusProbeAt
	}
	return !at.IsZero() && !at.After(now) && now.Sub(at) < statusHookFreshness
}

// reserveIdleNotice only reads existing observations. No pane capture or native
// delivery occurs while membership and agent snapshot locks are held.
func (run *runtime) reserveIdleNotice(sweep *idleSweep) error {
	teamLock, err := (store.Paths{Root: run.settings.StateRoot}).LockTeam(run.settings.Session)
	if errors.Is(err, store.ErrLocked) {
		return nil
	}
	if err != nil {
		return err
	}
	defer teamLock.Close()
	agents, err := run.team.ListAgents()
	if err != nil {
		return err
	}
	if len(agents) != len(sweep.agents) {
		return nil
	}
	outside, err := run.team.ListOutside()
	if err != nil {
		return err
	}
	if len(outside) != len(sweep.outside) {
		return nil
	}
	for i, o := range outside {
		if o.ID != sweep.outside[i].ID || o.Name != sweep.outside[i].Name {
			return nil
		}
	}
	// ListAgents omits incomplete name/state bindings for roster recovery.
	// Such a registered worker is unknown here, rather than absent.
	known := make(map[string]core.HitchID, len(agents)+len(outside))
	for _, a := range agents {
		known[string(a.Name)] = a.ID
	}
	for _, o := range outside {
		known[string(o.Name)] = o.ID
	}
	claims, err := os.ReadDir(run.team.Names)
	if err != nil {
		return err
	}
	for _, claim := range claims {
		id, err := run.team.ResolveName(claim.Name())
		if err != nil {
			return err
		}
		if !store.IsSenderID(id) && known[claim.Name()] != id {
			return nil
		}
	}
	var locks []*store.LockedAgent
	defer func() {
		for _, l := range locks {
			_ = l.Close()
		}
	}()
	for i, a := range agents {
		if !sameIdleIdentity(idleMember(a, 0), idleMember(sweep.agents[i], 0)) || !sweep.observed[a.ID] {
			return nil
		}
		p, err := run.team.Agent(a.ID)
		if err != nil {
			return err
		}
		l, err := p.TryLock()
		if errors.Is(err, store.ErrLocked) || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		locks = append(locks, l)
		fresh, err := p.Read()
		if err != nil {
			return err
		}
		if !sameIdleIdentity(idleMember(a, 0), idleMember(fresh, 0)) {
			return nil
		}
		agents[i] = fresh
	}
	lead, ok := currentLead(agents)
	if !ok {
		return nil
	}
	// Wake withdrawal holds usage before resolving publication, so snapshots
	// use that same order. Read only; notification does not rewrite schedules.
	usageLock, err := run.team.LockUsage()
	if err != nil {
		return err
	}
	defer usageLock.Close()
	var usage usageState
	if err := usageLock.Read(&usage); err != nil {
		return err
	}
	l, err := run.team.LockIdle()
	if err != nil {
		return err
	}
	defer l.Close()
	s, err := l.Read()
	if err != nil {
		return err
	}
	members := make([]store.IdleMember, 0, len(agents))
	for _, a := range agents {
		if a.ID == lead.ID {
			continue
		}
		if s.Work[a.ID] != sweep.work[a.ID] {
			return nil
		}
		members = append(members, idleMember(a, s.Work[a.ID]))
	}
	for _, o := range outside {
		members = append(members, store.IdleMember{ID: o.ID, Name: o.Name, Generation: "mailbox"})
	}
	advanceIdleEpisode(&s, idleMember(lead, 0), members)
	// Bound retained work to current registrations, after resolving publication.
	active := make(map[core.HitchID]bool, len(agents))
	for _, a := range agents {
		active[a.ID] = true
	}
	for id := range s.Work {
		if !active[id] {
			delete(s.Work, id)
		}
	}
	ready := len(members) > 0 && len(outside) == 0 && s.Armed && idleNoticeSettled(s.Notice)
	now := run.cmd.now()
	if now.Before(sweep.start) {
		ready = false
	}
	future := 0
	var next time.Time
	for i, a := range agents {
		if a.ID == lead.ID {
			continue
		}
		if !idleObservation(a, now) {
			ready = false
		}
		p := locks[i].Paths
		if _, pending, err := p.ReadPermissionWitness(); err != nil {
			return err
		} else if pending {
			ready = false
		}
		hooks, err := p.ReadStatusHooks()
		if err != nil {
			return err
		}
		for _, h := range hooks {
			if h.Sequence > a.Native.HookSequence {
				ready = false
			}
		}
		if a.LastAccepted != "" {
			ready = false
		}
		if a.LastFailed != "" {
			e, err := p.ReadEnvelope("failed", a.LastFailed)
			if err != nil {
				return err
			}
			if e.Outcome == "unverified" {
				ready = false
			}
		}
		pending, err := p.ListNew()
		if err != nil {
			return err
		}
		for _, e := range pending {
			if e.Outcome != "" || !e.NotBefore.After(now) {
				ready = false
				continue
			}
			future++
			if next.IsZero() || e.NotBefore.Before(next) {
				next = e.NotBefore
			}
		}
	}
	// Snoozes not yet in inboxes still carry due or uncertain worker work.
	for _, wake := range usage.Snoozes {
		if wake.CallerID == lead.ID || !active[wake.CallerID] {
			continue
		}
		if wake.Submission != "" || wake.At.IsZero() || !wake.At.After(now) {
			ready = false
			continue
		}
		future++
		if next.IsZero() || wake.At.Before(next) {
			next = wake.At
		}
	}
	for _, wake := range usage.Recent {
		if wake.CallerID != lead.ID && active[wake.CallerID] {
			ready = false
		}
	}

	for id, candidate := range usage.AutoCandidates {
		if core.HitchID(id) != lead.ID && active[core.HitchID(id)] && !candidate.At.IsZero() {
			ready = false
		}
	}
	windowMembers := make([]store.IdleWindowMember, 0, len(members))
	for _, a := range agents {
		if a.ID != lead.ID {
			windowMembers = append(windowMembers, store.IdleWindowMember{Member: idleMember(a, s.Work[a.ID]), HookSequence: a.Native.HookSequence, ChangedAt: a.ChangedAt, ScreenSince: a.ScreenSince})
		}
	}
	window, mature := advanceIdleWindow(l.ReadWindow(), store.IdleWindow{Since: now, Last: now, Minimum: run.settings.TeamIdleMinDuration, Lead: idleMember(lead, 0), Members: windowMembers}, ready)
	if mature {
		id, err := randomID("team-idle")
		if err != nil {
			return err
		}
		token, err := randomEnvelopeToken()
		if err != nil {
			return err
		}
		s.Notice = &store.IdleNotice{Generation: lead.Registration.Generation, Session: lead.Native.SessionID, Envelope: core.Envelope{ID: core.EnvelopeID(id), Token: token, Recipient: lead.ID, To: lead.Name, From: core.Sender{Kind: core.SenderGangline, Name: "team-idle"}, CreatedAt: now, Message: core.Message{Text: idleNoticeText(run.settings.Session, members, window.Since, now, future, next)}}}
		s.Armed = false
		window = nil
	}
	// Disarm durably before clearing the cache. An interrupted cleanup cannot
	// reserve another notice, even if the previous window was already mature.
	if err := l.Save(s); err != nil {
		return err
	}
	return l.SaveWindow(window)
}

// advanceIdleWindow measures repeated observations, not native completion.
// Unevaluated sweeps never call it and cannot extend the observation window.
func advanceIdleWindow(previous *store.IdleWindow, current store.IdleWindow, ready bool) (*store.IdleWindow, bool) {
	if !ready {
		return nil, false
	}
	same := previous != nil && !previous.Since.IsZero() && !previous.Last.Before(previous.Since) && !current.Last.Before(previous.Last) && current.Last.Sub(previous.Last) < statusHookFreshness && previous.Minimum == current.Minimum && previous.Lead == current.Lead && len(previous.Members) == len(current.Members)
	if same {
		for i, a := range current.Members {
			b := previous.Members[i]
			if a.Member != b.Member || a.HookSequence != b.HookSequence || !a.ChangedAt.Equal(b.ChangedAt) || !a.ScreenSince.Equal(b.ScreenSince) {
				same = false
				break
			}
		}
	}
	if !same {
		return &current, false
	}
	current.Since = previous.Since
	return &current, current.Minimum > 0 && current.Last.Sub(current.Since) >= current.Minimum
}

func idleNoticeSettled(n *store.IdleNotice) bool {
	return n == nil || n.Outcome == "delivered" || n.Outcome == "failed" || n.Outcome == "recipient-gone" || n.Outcome == "recipient-changed"
}

func idleNoticeText(team string, members []store.IdleMember, start, end time.Time, future int, next time.Time) string {
	names := make([]string, 0, len(members))
	for _, m := range members {
		names = append(names, fmt.Sprintf("%s (%s)", m.Name, m.ID))
	}
	context := "none observed"
	if future > 0 {
		context = fmt.Sprintf("%d pending; next due %s", future, next.UTC().Format(time.RFC3339Nano))
	}
	return fmt.Sprintf("Gangline observed all %d registered workers in %s as idle across repeated observations %s–%s: %s. This is not proof of continuous native idleness: quiet gaps can look idle, and native child/background work may still be running. It does not mean assignments are complete or authorize resuming held work. Future scheduled work: %s.", len(members), team, start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano), strings.Join(names, ", "), context)
}

func (run *runtime) readIdleNotice() (*store.IdleNotice, error) {
	l, err := run.team.LockIdle()
	if err != nil {
		return nil, err
	}
	defer l.Close()
	s, err := l.Read()
	if err != nil {
		return nil, err
	}
	return s.Notice, nil
}

func (run *runtime) settleIdleNotice(e core.Envelope, outcome string) error {
	if e.From.Kind != core.SenderGangline || e.From.Name != "team-idle" {
		return nil
	}
	l, err := run.team.LockIdle()
	if err != nil {
		return err
	}
	defer l.Close()
	s, err := l.Read()
	if err != nil {
		return err
	}
	if s.Notice == nil || s.Notice.Envelope.ID != e.ID || s.Notice.Envelope.Recipient != e.Recipient || s.Notice.Envelope.Token != e.Token {
		return nil
	}
	s.Notice.Outcome = outcome
	return l.Save(s)
}

func (run *runtime) flushIdleNotice() error {
	n, err := run.readIdleNotice()
	if err != nil || n == nil || idleNoticeSettled(n) {
		return err
	}
	l, a, err := run.acquire(n.Envelope.Recipient, false)
	if errors.Is(err, store.ErrLocked) {
		return nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return run.settleIdleNotice(n.Envelope, "recipient-gone")
	}
	if err != nil {
		return err
	}
	defer run.unlock(l)
	// A concurrent evaluator may have settled or replaced the intent.
	current, err := run.readIdleNotice()
	if err != nil {
		return err
	}
	if current == nil || current.Envelope.ID != n.Envelope.ID {
		return nil
	}
	if a.Status != core.Active || !isLead(a) || a.Registration.Generation != n.Generation || a.Native.SessionID != n.Session {
		return run.settleIdleNotice(n.Envelope, "recipient-changed")
	}
	if current.Outcome != "" {
		return nil
	}
	if err := run.publishOnce(l, &a, n.Envelope); err != nil {
		return err
	}
	_, err = run.drainFrom(l, a, n.Envelope.ID)
	return err
}

// A notice already queued before a rehitch must not reach the replacement.
// This check also closes the crash window between settlement and receipt filing.
func (run *runtime) retireIdleEnvelope(l *store.LockedAgent, a core.Agent, e core.Envelope) (bool, error) {
	if e.From.Kind != core.SenderGangline || e.From.Name != "team-idle" {
		return false, nil
	}
	n, err := run.readIdleNotice()
	if err != nil {
		return false, err
	}
	match := n != nil && n.Envelope.ID == e.ID && n.Envelope.Token == e.Token && n.Envelope.Recipient == a.ID
	if match && n.Outcome == "" && isLead(a) && n.Generation == a.Registration.Generation && n.Session == a.Native.SessionID {
		return false, nil
	}
	if a.Input != nil {
		return false, fmt.Errorf("idle notice has unreconciled input intent")
	}
	if match && n.Outcome == "" {
		if err := run.settleIdleNotice(e, "recipient-changed"); err != nil {
			return false, err
		}
	}
	if err := l.Withdraw(e.ID); err != nil {
		return false, err
	}
	return true, run.record(a, core.Event{Type: "send_cancelled", ID: string(e.ID), Reason: "idle notice no longer belongs to this lead observation intent"})
}
