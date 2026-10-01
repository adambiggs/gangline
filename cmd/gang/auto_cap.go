package main

import (
	"time"

	"github.com/adambiggs/gangline/core"
)

// A cap refusal is authoritative; a recent native limits reading supplies its
// reset. If the reset is unavailable, retain the park until a reading arrives.
func recentNativeReset(r core.Reading, now time.Time) (time.Time, bool) {
	if r.Status != "observed" || r.At == nil || r.At.After(now) || now.Sub(*r.At) > 5*time.Minute {
		return time.Time{}, false
	}
	if reset, capped := cappedNativeReset(r.Limits, now); capped {
		return reset, true
	}
	reset, err := snoozeReset(r.Limits, now)
	return reset, err == nil
}

func (run *runtime) observeAutoCap(a core.Agent, notice hookNotice) error {
	now := run.cmd.now()
	key := string(a.ID)
	var failureAt time.Time
	var reason string
	if notice.Kind == "turn-failed" && notice.TurnID != "" && notice.TurnID == a.Native.FailedTurn {
		failureAt, reason = notice.At, notice.Failure
		if failureAt.IsZero() {
			failureAt = now
		}
	} else if a.Collar == "codex" && !a.Native.LastErrorAt.IsZero() &&
		!a.Native.SubmittedAt.IsZero() && !a.Native.LastErrorAt.Before(a.Native.SubmittedAt) {
		failureAt, reason = a.Native.LastErrorAt, a.Native.LastError
	}
	explicit, generic := usageCapFailure(reason)
	reset, capped := observedCappedReset(a.Native.Limits, a.Native.SubmittedAt, now)
	return run.withUsageState(func(state *usageState) error {
		if (explicit || generic) && failureAt.After(state.AutoCaps[key]) && !failureAt.After(now) {
			state.AutoCaps[key] = failureAt
			delete(state.AutoCandidates, key)
			if generic && !capped {
				state.AutoCandidates[key] = autoCapCandidate{At: failureAt, SubmittedAt: a.Native.SubmittedAt}
			} else {
				if !capped {
					reset, _ = recentNativeReset(a.Native.Limits, now)
				}
				if err := scheduleAutomaticWake(state, a, reset); err != nil {
					return err
				}
			}
		}
		if candidate, exists := state.AutoCandidates[key]; exists {
			if now.Sub(candidate.At) > 5*time.Minute {
				delete(state.AutoCandidates, key)
			} else if a.Native.Limits.At != nil && !a.Native.Limits.At.Before(candidate.At) {
				if candidateReset, confirmed := observedCappedReset(a.Native.Limits, candidate.SubmittedAt, now); confirmed {
					delete(state.AutoCandidates, key)
					if err := scheduleAutomaticWake(state, a, candidateReset); err != nil {
						return err
					}
				}
			}
		}
		s := state.Snoozes[key]
		if s.ID != "" && s.Auto && s.At.IsZero() && a.Native.Limits.Status == "observed" &&
			a.Native.Limits.At != nil && !a.Native.Limits.At.After(now) &&
			now.Sub(*a.Native.Limits.At) <= 5*time.Minute {
			if candidateReset, stillCapped := cappedNativeReset(a.Native.Limits.Limits, now); stillCapped {
				s.At = candidateReset
			} else if !a.Native.Limits.At.Before(state.AutoCaps[key]) {
				// A fresh low-usage reading proves the prior reset has passed.
				s.At = now
			}
			state.Snoozes[key] = s
		}
		return nil
	})
}

func scheduleAutomaticWake(state *usageState, a core.Agent, reset time.Time) error {
	key := string(a.ID)
	if state.Snoozes[key].ID != "" || state.Recent[key].ID != "" {
		return nil
	}
	id, err := randomID("snooze")
	if err != nil {
		return err
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	state.Snoozes[key] = usageSnooze{
		ID: core.EnvelopeID(id), Token: token, CallerID: a.ID, CallerName: a.Name,
		At: reset, Note: defaultSnoozeNote, Auto: true,
	}
	return nil
}
