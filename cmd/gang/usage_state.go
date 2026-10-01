package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

type usageState struct {
	Windows        map[string]usageWindowState `json:"windows,omitempty"`
	Notices        []usageNotice               `json:"notices,omitempty"`
	Snoozes        map[string]usageSnooze      `json:"snoozes,omitempty"`
	Recent         map[string]usageSnooze      `json:"recent_wakes,omitempty"`
	AutoCaps       map[string]time.Time        `json:"auto_caps,omitempty"`
	AutoCandidates map[string]autoCapCandidate `json:"auto_candidates,omitempty"`
}

type usageWindowState struct {
	ResetAt    int64     `json:"reset_at"`
	ObservedAt time.Time `json:"observed_at"`
	Percent    float64   `json:"percent"`
	Fired      []string  `json:"fired,omitempty"`
}

type usageNotice struct {
	ID            core.EnvelopeID `json:"id"`
	Token         string          `json:"token"`
	Collar        string          `json:"collar"`
	Window        string          `json:"window"`
	Band          string          `json:"band"`
	Text          string          `json:"text"`
	CreatedAt     time.Time       `json:"created_at"`
	RecipientID   core.HitchID    `json:"recipient_id,omitempty"`
	RecipientName core.AgentName  `json:"recipient_name,omitempty"`
	Submission    string          `json:"submission,omitempty"`
}

type usageSnooze struct {
	ID            core.EnvelopeID `json:"id"`
	Token         string          `json:"token"`
	CallerID      core.HitchID    `json:"caller_id"`
	CallerName    core.AgentName  `json:"caller_name"`
	At            time.Time       `json:"at"`
	Note          string          `json:"note"`
	RecipientID   core.HitchID    `json:"recipient_id,omitempty"`
	RecipientName core.AgentName  `json:"recipient_name,omitempty"`
	Submission    string          `json:"submission,omitempty"`
	InputText     string          `json:"input_text,omitempty"`
	TurnID        string          `json:"turn_id,omitempty"`
	SubmittedAt   time.Time       `json:"submitted_at,omitzero"`
	CapRejected   bool            `json:"cap_rejected,omitempty"`
	CapCandidate  bool            `json:"cap_candidate,omitempty"`
	CapFailedAt   time.Time       `json:"cap_failed_at,omitzero"`
	TurnFailed    bool            `json:"turn_failed,omitempty"`
	Rearms        int             `json:"rearms,omitempty"`
	Auto          bool            `json:"auto,omitempty"`
}

type autoCapCandidate struct {
	At          time.Time `json:"at"`
	SubmittedAt time.Time `json:"submitted_at"`
}

func (run *runtime) withUsageState(change func(*usageState) error) (result error) {
	l, err := run.team.LockUsage()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, l.Close()) }()
	var state usageState
	if err := l.Read(&state); err != nil {
		return err
	}
	if state.Windows == nil {
		state.Windows = make(map[string]usageWindowState)
	}
	if state.Snoozes == nil {
		state.Snoozes = make(map[string]usageSnooze)
	}
	if state.Recent == nil {
		state.Recent = make(map[string]usageSnooze)
	}
	if state.AutoCaps == nil {
		state.AutoCaps = make(map[string]time.Time)
	}
	if state.AutoCandidates == nil {
		state.AutoCandidates = make(map[string]autoCapCandidate)
	}
	for key, legacy := range state.Windows {
		kind, ok := strings.CutPrefix(key, "claude-code/")
		if !ok {
			continue
		}
		canonicalKey := "claude/" + kind
		if current, exists := state.Windows[canonicalKey]; exists {
			if current.ResetAt == legacy.ResetAt {
				fired := make(map[string]bool, len(current.Fired))
				for _, band := range current.Fired {
					fired[band] = true
				}
				for _, band := range legacy.Fired {
					if !fired[band] {
						current.Fired = append(current.Fired, band)
						fired[band] = true
					}
				}
				if legacy.ObservedAt.After(current.ObservedAt) {
					current.ObservedAt, current.Percent = legacy.ObservedAt, legacy.Percent
				}
				state.Windows[canonicalKey] = current
			} else if legacy.ObservedAt.After(current.ObservedAt) {
				state.Windows[canonicalKey] = legacy
			}
		} else {
			state.Windows[canonicalKey] = legacy
		}
		delete(state.Windows, key)
	}
	for i := range state.Notices {
		if state.Notices[i].Collar == "claude-code" {
			state.Notices[i].Collar = "claude"
		}
	}
	if err := change(&state); err != nil {
		return err
	}
	return l.Save(state)
}

func (run *runtime) acknowledgeUsageDelivery(l *store.LockedAgent, a core.Agent, e core.Envelope, outcome string, witnessedAt ...time.Time) error {
	if e.From.Kind != core.SenderGangline ||
		e.From.Name != "snooze" && e.From.Name != "usage-band" {
		return nil
	}
	var submittedAt time.Time
	if outcome == "delivered" && e.From.Name == "snooze" {
		if len(witnessedAt) > 0 {
			submittedAt = witnessedAt[0]
		} else {
			w, err := l.Paths.ReadWitness()
			if err == nil && w.TurnID == a.Native.TurnID {
				submittedAt = w.At
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return run.withUsageState(func(state *usageState) error {
		if e.From.Name == "usage-band" {
			for i, n := range state.Notices {
				if n.ID == e.ID && n.RecipientID == a.ID {
					if outcome == "delivered" {
						state.Notices = append(state.Notices[:i], state.Notices[i+1:]...)
					} else if outcome == "accepted" || outcome == "unverified" {
						state.Notices[i].Submission = outcome
					}
					break
				}
			}
			return nil
		}
		for key, s := range state.Snoozes {
			if s.ID == e.ID && s.RecipientID == a.ID {
				if outcome == "accepted" || outcome == "unverified" {
					s.Submission = outcome
					s.InputText = e.Message.Text
					state.Snoozes[key] = s
					break
				}
				if outcome != "delivered" {
					break
				}
				s.TurnID = a.Native.TurnID
				s.SubmittedAt = submittedAt
				s.Submission = ""
				s.InputText = ""
				state.Recent[key] = s
				delete(state.Snoozes, key)
				break
			}
		}
		return nil
	})
}

// An accepted or uncertain submission can outlive its one-slot inbox receipt.
// Retain its exact input so a later native submit witness still identifies the
// turn whose successful completion can clear the wake.
func (run *runtime) reconcileUsageSubmission(l *store.LockedAgent, a *core.Agent) (result error) {
	w, err := l.Paths.ReadWitness()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if a.Native.SessionID != "" && w.SessionID != "" && w.SessionID != a.Native.SessionID {
		return nil
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	usage, err := run.team.LockUsage()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, usage.Close()) }()
	var state usageState
	if err := usage.Read(&state); err != nil {
		return err
	}
	changed := false
	for i := len(state.Notices) - 1; i >= 0; i-- {
		n := state.Notices[i]
		if n.RecipientID != a.ID || n.Submission == "" || w.At.Before(n.CreatedAt) {
			continue
		}
		e := core.Envelope{Token: n.Token, From: core.Sender{Kind: core.SenderGangline, Name: "usage-band"}, Message: core.Message{Text: n.Text}}
		wire, err := envelopeText(e)
		if err != nil {
			return err
		}
		matched, err := harness.SubmittedPromptStartsWith(c.Primitives.SubmitWitness, wire, w.Prompt)
		if err != nil {
			return err
		}
		if matched {
			state.Notices = append(state.Notices[:i], state.Notices[i+1:]...)
			changed = true
		}
	}
	for key, s := range state.Snoozes {
		if s.RecipientID != a.ID || s.Submission == "" || s.InputText == "" || w.At.Before(s.At) {
			continue
		}
		e := core.Envelope{Token: s.Token, From: core.Sender{Kind: core.SenderGangline, Name: "snooze"}, Message: core.Message{Text: s.InputText}}
		wire, err := envelopeText(e)
		if err != nil {
			return err
		}
		matched, err := harness.SubmittedPromptStartsWith(c.Primitives.SubmitWitness, wire, w.Prompt)
		if err != nil {
			return err
		}
		if !matched {
			continue
		}
		s.Submission, s.InputText = "", ""
		s.TurnID, s.SubmittedAt = w.TurnID, w.At
		if state.Recent == nil {
			state.Recent = make(map[string]usageSnooze)
		}
		state.Recent[key] = s
		delete(state.Snoozes, key)
		changed = true
	}
	if changed {
		return usage.Save(state)
	}
	return nil
}

func usageCapFailure(reason string) (explicit, generic bool) {
	reason = strings.ToLower(reason)
	for _, phrase := range []string{"usage limit", "usage_limit", "five-hour limit", "5-hour limit", "weekly limit", "hit your limit", "reached your limit"} {
		if strings.Contains(reason, phrase) {
			return true, false
		}
	}
	return false, strings.Contains(reason, "rate limit") || strings.Contains(reason, "rate_limit") || strings.Contains(reason, "quota")
}

func observedCappedReset(r core.Reading, submittedAt, now time.Time) (time.Time, bool) {
	if r.Status != "observed" || r.At == nil || submittedAt.IsZero() || !r.At.After(submittedAt) ||
		r.At.After(now) || now.Sub(*r.At) > 5*time.Minute {
		return time.Time{}, false
	}
	return cappedNativeReset(r.Limits, now)
}

func cappedNativeReset(limits []core.LimitWindow, now time.Time) (time.Time, bool) {
	var reset time.Time
	for _, window := range limits {
		if harness.UsageWindowKind(window.Label, window.WindowMinutes) != "" &&
			window.UsedPercent >= 99 && window.ResetAt > now.Unix() {
			candidate := time.Unix(window.ResetAt, 0)
			if reset.IsZero() || candidate.Before(reset) {
				reset = candidate
			}
		}
	}
	return reset, !reset.IsZero()
}

// A wake completes only on a matching successful native turn. An attributable
// usage-cap failure can re-arm it once at the next observed provider reset.
func (run *runtime) observeSnoozeTurn(a core.Agent, notice hookNotice, providerBlocked bool) error {
	now := run.cmd.now()
	var events []core.Event
	if err := run.withUsageState(func(state *usageState) error {
		for key, s := range state.Recent {
			if s.RecipientID != a.ID || s.TurnID == "" {
				continue
			}
			reset, observedCap := observedCappedReset(a.Native.Limits, s.SubmittedAt, now)
			if notice.TurnID == s.TurnID && notice.Kind == "turn-finished" {
				nativeError := !s.SubmittedAt.IsZero() && !a.Native.LastErrorAt.Before(s.SubmittedAt)
				if !providerBlocked && !nativeError && a.Native.TurnFailure == "" {
					delete(state.Recent, key)
				} else {
					s.TurnFailed = true
					state.Recent[key] = s
				}
				continue
			}
			confirmed := false
			failure := notice.Failure
			failedTurn := notice.TurnID == s.TurnID && notice.Kind == "turn-failed" && a.Native.FailedTurn == s.TurnID
			if s.Auto && a.Collar == "codex" && a.Native.TurnID == s.TurnID &&
				!s.SubmittedAt.IsZero() && !a.Native.LastErrorAt.Before(s.SubmittedAt) {
				failedTurn, failure = true, a.Native.LastError
			}
			if failedTurn {
				explicit, generic := usageCapFailure(failure)
				if !explicit && !generic {
					s.TurnFailed = true
					state.Recent[key] = s
					continue
				}
				if generic && !observedCap {
					s.CapCandidate = true
					s.CapFailedAt = now
					state.Recent[key] = s
					continue
				}
				confirmed = true
			}
			if s.CapCandidate && (s.CapFailedAt.IsZero() || now.Sub(s.CapFailedAt) > 5*time.Minute) {
				s.CapCandidate = false
				s.TurnFailed = true
				state.Recent[key] = s
				continue
			}
			if s.CapCandidate && observedCap && a.Native.Limits.At != nil && !a.Native.Limits.At.Before(s.CapFailedAt) {
				s.CapCandidate = false
				confirmed = true
			}
			if confirmed && !s.CapRejected {
				s.CapRejected = true
				state.Recent[key] = s
				reason := failure
				if reason == "" {
					reason = "native rate limit confirmed by fresh capped-window reading"
				}
				if s.Rearms > 0 && !s.Auto {
					reason += "; no further wake scheduled"
				}
				events = append(events, core.Event{Type: "snooze_cap_rejected", At: now, HitchID: a.ID, Name: a.Name, ID: string(s.ID), Reason: reason})
			}
			if s.CapRejected && s.Rearms > 0 && !s.Auto {
				continue
			}
			if s.Auto && s.CapRejected && !observedCap {
				reset, observedCap = recentNativeReset(a.Native.Limits, now)
			}
			if !s.CapRejected || !observedCap {
				continue
			}
			if current := state.Snoozes[key]; current.ID != "" {
				delete(state.Recent, key)
				continue
			}
			id, err := randomID("snooze")
			if err != nil {
				return err
			}
			token, err := randomEnvelopeToken()
			if err != nil {
				return err
			}
			next := s
			next.ID, next.Token, next.At = core.EnvelopeID(id), token, reset
			next.RecipientID, next.RecipientName, next.TurnID, next.CapRejected, next.TurnFailed = "", "", "", false, false
			next.CapCandidate = false
			next.CapFailedAt = time.Time{}
			next.SubmittedAt = time.Time{}
			next.Submission = ""
			next.Rearms++
			state.Snoozes[key] = next
			delete(state.Recent, key)
			events = append(events, core.Event{Type: "snooze_rearmed", At: now, HitchID: a.ID, Name: a.Name, ID: id, Deadline: reset, Reason: "attributable native usage-cap rejection"})
		}
		return nil
	}); err != nil {
		return err
	}
	for _, event := range events {
		if err := run.team.Append(event); err != nil {
			return err
		}
	}
	return nil
}

func renderUsageBand(band harness.UsageBand, collar, window string, used float64, reset int64) string {
	message := band.Message
	if message == "" {
		message = "{{collar}} {{window}} usage: {{used_percent}}% used, resets {{reset_at}}"
	}
	if band.Note != "" {
		message += " " + band.Note
	}
	return strings.NewReplacer(
		"{{band}}", band.Name,
		"{{collar}}", collar,
		"{{window}}", strings.ReplaceAll(window, "_", "-"),
		"{{threshold_percent}}", fmt.Sprintf("%.0f", band.At*100),
		"{{used_percent}}", fmt.Sprintf("%.0f", used),
		"{{reset_at}}", time.Unix(reset, 0).UTC().Format(time.RFC3339),
		"{{snooze_command}}", "gang snooze --note 'Resume from your saved state'",
	).Replace(message)
}

// Every provider window has one account-wide crossing state per collar. A
// stale reading from another agent cannot roll it back or repeat a notice.
func (run *runtime) observeUsageBands(a core.Agent, c harness.Collar) error {
	r := a.Native.Limits
	if a.Status != core.Active || r.Status != "observed" || r.At == nil || r.At.After(run.cmd.now()) {
		return nil
	}
	now := run.cmd.now()
	windows := make(map[string]core.LimitWindow)
	for _, w := range r.Limits {
		kind := harness.UsageWindowKind(w.Label, w.WindowMinutes)
		if kind == "" || w.ResetAt <= now.Unix() || w.UsedPercent < 0 || w.UsedPercent > 100 {
			continue
		}
		if selected, exists := windows[kind]; !exists || w.UsedPercent > selected.UsedPercent || w.UsedPercent == selected.UsedPercent && w.ResetAt > selected.ResetAt {
			windows[kind] = w
		}
	}
	return run.withUsageState(func(state *usageState) error {
		for kind, w := range windows {
			key := c.Name + "/" + kind
			previous := state.Windows[key]
			if previous.ResetAt > w.ResetAt || r.At.Before(previous.ObservedAt) ||
				previous.ResetAt != w.ResetAt && !r.At.After(previous.ObservedAt) ||
				previous.ResetAt == w.ResetAt && r.At.Equal(previous.ObservedAt) && w.UsedPercent <= previous.Percent {
				continue
			}
			if previous.ResetAt != w.ResetAt {
				previous = usageWindowState{ResetAt: w.ResetAt}
			}
			fired := make(map[string]bool, len(previous.Fired))
			for _, name := range previous.Fired {
				fired[name] = true
			}
			for _, band := range c.UsageBands[kind] {
				if w.UsedPercent < band.At*100 || fired[band.Name] {
					continue
				}
				id, err := randomID("usage-band")
				if err != nil {
					return err
				}
				token, err := randomEnvelopeToken()
				if err != nil {
					return err
				}
				state.Notices = append(state.Notices, usageNotice{ID: core.EnvelopeID(id), Token: token, Collar: c.Name, Window: kind, Band: band.Name, Text: renderUsageBand(band, c.Name, kind, w.UsedPercent, w.ResetAt), CreatedAt: now})
				previous.Fired = append(previous.Fired, band.Name)
				fired[band.Name] = true
			}
			previous.ObservedAt, previous.Percent = *r.At, w.UsedPercent
			state.Windows[key] = previous
		}
		return nil
	})
}

func currentLead(agents []core.Agent) (core.Agent, bool) {
	var leads []core.Agent
	for _, a := range agents {
		if a.Status == core.Active && a.Role == "lead" {
			leads = append(leads, a)
		}
	}
	if len(leads) == 0 {
		for _, a := range agents {
			if a.Status == core.Active && a.Name == "lead" {
				leads = append(leads, a)
			}
		}
	}
	if len(leads) != 1 {
		return core.Agent{}, false
	}
	return leads[0], true
}

func (run *runtime) publishUsageEnvelope(e core.Envelope) (bool, error) {
	l, a, err := run.acquire(e.Recipient, false)
	if errors.Is(err, store.ErrLocked) || errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer l.Close()
	if a.Status != core.Active {
		return false, nil
	}
	ready := false
	if err := run.withUsageState(func(state *usageState) error {
		if e.From.Name == "usage-band" {
			for _, n := range state.Notices {
				if n.ID == e.ID && n.RecipientID == a.ID && n.Submission == "" {
					ready = true
					break
				}
			}
		} else if e.From.Name == "snooze" {
			for _, s := range state.Snoozes {
				if s.ID == e.ID && s.RecipientID == a.ID && s.Submission == "" {
					ready = true
					break
				}
			}
		}
		return nil
	}); err != nil {
		return false, err
	}
	if !ready {
		return false, nil
	}
	if err := run.publishOnce(l, &a, e); err != nil {
		return false, err
	}
	if _, err := run.drainFrom(l, a, e.ID); err != nil {
		return false, err
	}
	receipt, err := l.Paths.ReadEnvelope("cur", e.ID)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return receipt.Outcome == "delivered", nil
}

// Publication runs after agent locks have been released. Exact submit proof
// clears the durable intent before ordinary inbox receipt cleanup can run.
func (run *runtime) flushUsageWork() error {
	agents, err := run.team.ListAgents()
	if err != nil {
		return err
	}
	lead, haveLead := currentLead(agents)
	active := make(map[core.HitchID]core.Agent, len(agents))
	for _, a := range agents {
		if a.Status == core.Active {
			active[a.ID] = a
		}
	}
	var notices []usageNotice
	var wakes []usageSnooze
	now := run.cmd.now()
	if err := run.withUsageState(func(state *usageState) error {
		for key, pending := range state.Recent {
			if _, exists := active[pending.RecipientID]; exists {
				continue
			}
			if state.Snoozes[key].ID == "" {
				id, err := randomID("snooze")
				if err != nil {
					return err
				}
				token, err := randomEnvelopeToken()
				if err != nil {
					return err
				}
				pending.ID, pending.Token = core.EnvelopeID(id), token
				pending.RecipientID, pending.RecipientName, pending.TurnID = "", "", ""
				pending.CapRejected, pending.CapCandidate, pending.TurnFailed = false, false, false
				pending.CapFailedAt = time.Time{}
				pending.Submission = ""
				pending.SubmittedAt = time.Time{}
				state.Snoozes[key] = pending
			}
			delete(state.Recent, key)
		}
		for i := range state.Notices {
			n := &state.Notices[i]
			if _, exists := active[n.RecipientID]; !exists {
				if n.RecipientID != "" && n.Submission != "" {
					id, err := randomID("usage-band")
					if err != nil {
						return err
					}
					token, err := randomEnvelopeToken()
					if err != nil {
						return err
					}
					n.ID, n.Token = core.EnvelopeID(id), token
				}
				n.RecipientID, n.RecipientName = "", ""
				n.Submission = ""
			}
			if n.RecipientID == "" && haveLead {
				n.RecipientID, n.RecipientName = lead.ID, lead.Name
			}
			if n.RecipientID != "" && n.Submission == "" {
				notices = append(notices, *n)
			}
		}
		keys := make([]string, 0, len(state.Snoozes))
		for key := range state.Snoozes {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			s := state.Snoozes[key]
			if s.At.IsZero() || s.At.After(now) {
				continue
			}
			if _, exists := active[s.RecipientID]; !exists {
				if s.RecipientID != "" && s.Submission != "" {
					id, err := randomID("snooze")
					if err != nil {
						return err
					}
					token, err := randomEnvelopeToken()
					if err != nil {
						return err
					}
					s.ID, s.Token = core.EnvelopeID(id), token
				}
				s.RecipientID, s.RecipientName = "", ""
				s.Submission = ""
			}
			if s.RecipientID == "" {
				if caller, exists := active[s.CallerID]; exists {
					s.RecipientID, s.RecipientName = caller.ID, caller.Name
				} else if haveLead {
					s.RecipientID, s.RecipientName = lead.ID, lead.Name
				}
			}
			state.Snoozes[key] = s
			if s.RecipientID != "" && s.Submission == "" {
				wakes = append(wakes, s)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, n := range notices {
		e := core.Envelope{ID: n.ID, Token: n.Token, Recipient: n.RecipientID, To: n.RecipientName, From: core.Sender{Kind: core.SenderGangline, Name: "usage-band"}, Message: core.Message{Text: n.Text}, CreatedAt: n.CreatedAt}
		if _, err := run.publishUsageEnvelope(e); err != nil {
			return err
		}
	}
	for _, s := range wakes {
		text := snoozeWakeText(s, now)
		e := core.Envelope{ID: s.ID, Token: s.Token, Recipient: s.RecipientID, To: s.RecipientName, From: core.Sender{Kind: core.SenderGangline, Name: "snooze"}, Message: core.Message{Text: text}, CreatedAt: now}
		if _, err := run.publishUsageEnvelope(e); err != nil {
			return err
		}
	}
	return nil
}
