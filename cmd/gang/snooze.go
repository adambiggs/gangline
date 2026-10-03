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

const defaultSnoozeNote = "Re-read your assignment and durable state, then continue only if work remains."

func snoozeWakeText(s usageSnooze, now time.Time) string {
	when := s.At.UTC().Format(time.RFC3339)
	message := fmt.Sprintf("Your scheduled wake was due at %s.", when)
	if s.Auto {
		message = fmt.Sprintf("Provider cap reset was due at %s. Resume the interrupted work.", when)
	}
	if now.After(s.At) {
		message += fmt.Sprintf(" It is overdue by %s.", now.Sub(s.At).Round(time.Second))
	}
	if s.RecipientID != s.CallerID {
		message += fmt.Sprintf(" Agent %s is no longer active; the lead is receiving its wake.", s.CallerName)
	}
	return message + " " + s.Note
}

func snoozeReset(limits []core.LimitWindow, now time.Time) (time.Time, error) {
	var selected *core.LimitWindow
	for i := range limits {
		w := &limits[i]
		if harness.UsageWindowKind(w.Label, w.WindowMinutes) == "" || w.ResetAt <= now.Unix() || w.UsedPercent < 0 || w.UsedPercent > 100 {
			continue
		}
		if selected == nil || w.UsedPercent > selected.UsedPercent || w.UsedPercent == selected.UsedPercent && w.ResetAt > selected.ResetAt {
			selected = w
		}
	}
	if selected == nil {
		return time.Time{}, fmt.Errorf("native provider limits have no future five-hour or weekly reset; use --at for an explicit wake")
	}
	return time.Unix(selected.ResetAt, 0), nil
}

func snoozeStatusText(s usageSnooze, submitted bool) string {
	if submitted {
		if s.Submission == "accepted" {
			return "accepted in native queue; awaiting turn success"
		}
		return "native submission unverified; inspect recipient"
	}
	switch {
	case s.Auto && s.At.IsZero():
		return "provider cap confirmed; awaiting native reset time"
	case s.CapCandidate:
		return "rate limit unconfirmed by native usage; inspect or clear"
	case s.CapRejected && s.Rearms > 0 && !s.Auto:
		return "usage cap rejected the replacement; manual action needed"
	case s.CapRejected:
		return "waiting for native reset after cap rejection"
	case s.TurnFailed:
		return "native turn failed; manual action needed"
	case s.TurnID == "":
		return "native turn identity unavailable; manual action needed"
	default:
		return "awaiting successful native turn"
	}
}

func snoozeQueuedText(s usageSnooze) string {
	return fmt.Sprintf("queued for %s, not yet submitted; due %s", s.RecipientName, s.At.UTC().Format(time.RFC3339))
}

// teammateWakeRow describes another agent's wake to the lead. The lead can
// clear only a wake routed to it.
func teammateWakeRow(s usageSnooze, recent bool, lead core.HitchID) string {
	due := "; due " + s.At.UTC().Format(time.RFC3339)
	var state string
	switch {
	case recent && s.CapCandidate && s.RecipientID != lead:
		state = "rate limit unconfirmed by native usage" + due
	case recent:
		state = snoozeStatusText(s, false) + due
	case s.Submission != "":
		state = snoozeStatusText(s, true) + due
	case s.RecipientID != "":
		state = snoozeQueuedText(s)
		if s.RecipientID == lead {
			state += "; --clear ID withdraws it"
		}
	case s.At.IsZero():
		state = snoozeStatusText(s, false)
	default:
		state = "scheduled" + due
	}
	row := fmt.Sprintf("%s\twake for %s; %s", s.ID, s.CallerName, state)
	if note := strings.Join(strings.Fields(s.Note), " "); note != "" {
		row += "; note: " + note
	}
	return row
}

func queuedWakeRecipient(s usageSnooze) core.HitchID {
	if s.Submission != "" {
		return ""
	}
	return s.RecipientID
}

// withQueuedWake changes usage state while holding the agent lock of the
// recipient that recipient names. A queued wake is published only after its
// intent is re-read under that lock, so withdraw can remove the inbox envelope
// and the intent together without racing a publish or a drain. Withdrawal
// leaves the agent's retained failure receipt, which may be an unverified
// startup contract, in place.
func (run *runtime) withQueuedWake(recipient func(*usageState) core.HitchID, change func(state *usageState, withdraw func(usageSnooze) error) error) (result error) {
	var id core.HitchID
	if err := run.withUsageState(func(state *usageState) error {
		id = recipient(state)
		return nil
	}); err != nil {
		return err
	}
	var l *store.LockedAgent
	var a core.Agent
	if id != "" {
		p, err := run.team.Agent(id)
		if err != nil {
			return err
		}
		if _, err := os.Stat(p.State); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		} else if err == nil {
			l, a, err = run.acquireBounded(id)
			if errors.Is(err, store.ErrLocked) {
				return refuseError("wake recipient is busy with an input operation; retry")
			}
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, run.release(l)) }()
		}
	}
	var cancelled []core.EnvelopeID
	const reason = "wake withdrawn before native submission"
	if err := run.withUsageState(func(state *usageState) error {
		if recipient(state) != id {
			return refuseError("wake delivery changed while it was being withdrawn; retry")
		}
		return change(state, func(s usageSnooze) error {
			if l == nil || queuedWakeRecipient(s) != a.ID {
				return nil
			}
			if err := l.Withdraw(s.ID); errors.Is(err, os.ErrNotExist) {
				return nil
			} else if err != nil {
				return err
			}
			cancelled = append(cancelled, s.ID)
			return nil
		})
	}); err != nil {
		return err
	}
	for _, e := range cancelled {
		if err := run.record(a, core.Event{Type: "send_cancelled", ID: string(e), Reason: reason}); err != nil {
			return err
		}
	}
	return nil
}

func (cmd command) snooze(args []string) error {
	var at, note string
	var clear, status bool
	flags := boundFlagSet("snooze", map[string]any{"at": &at, "note": &note, "clear": &clear, "status": &status})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("snooze: %v", err)
	}
	if len(positionals) > 1 || len(positionals) == 1 && !clear {
		return usageError("snooze: unexpected argument %q; only --clear takes an ID", positionals[len(positionals)-1])
	}
	if (clear || status) && (at != "" || note != "") || clear && status {
		return usageError("snooze: --clear and --status take no other options")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	a, err := run.observedAgent()
	if err != nil {
		return err
	}
	if a == nil {
		return refuseError("snooze schedules a wake for the calling agent, and the operator is not an agent; ask the agent to snooze with gang send NAME")
	}
	if a.Status != core.Active {
		return refuseError("hitch identity %s is %s, not active; run gang snooze once its hitch completes", a.Name, a.Status)
	}
	key := string(a.ID)
	isLead := a.Role == "lead" || a.Name == "lead"
	if status {
		var rows []string
		if err := run.withUsageState(func(state *usageState) error {
			if current := state.Snoozes[key]; current.ID != "" {
				if current.Submission != "" {
					rows = append(rows, fmt.Sprintf("%s\t%s", current.ID, snoozeStatusText(current, true)))
				} else if current.RecipientID != "" {
					rows = append(rows, fmt.Sprintf("%s\t%s; --clear withdraws it, --at replaces it", current.ID, snoozeQueuedText(current)))
				} else if current.At.IsZero() {
					rows = append(rows, fmt.Sprintf("%s\t%s", current.ID, snoozeStatusText(current, false)))
				} else {
					rows = append(rows, fmt.Sprintf("%s\t%s", current.ID, current.At.UTC().Format(time.RFC3339)))
				}
			} else if current := state.Recent[key]; current.ID != "" {
				rows = append(rows, fmt.Sprintf("%s\t%s", current.ID, snoozeStatusText(current, false)))
			} else if candidate := state.AutoCandidates[key]; !candidate.At.IsZero() {
				rows = append(rows, "provider cap unconfirmed; awaiting fresh native usage")
			}
			if isLead {
				for _, n := range state.Notices {
					if n.RecipientID == a.ID && n.Submission != "" {
						rows = append(rows, fmt.Sprintf("%s\t%s %s %s notice; native %s; inspect or clear by ID", n.ID, n.Collar, n.Window, n.Band, n.Submission))
					}
				}
				for caller, s := range state.Snoozes {
					if caller != key {
						rows = append(rows, teammateWakeRow(s, false, a.ID))
					}
				}
				for caller, s := range state.Recent {
					if caller != key {
						rows = append(rows, teammateWakeRow(s, true, a.ID))
					}
				}
			}
			return nil
		}); err != nil {
			return err
		}
		if len(rows) == 0 {
			if isLead {
				_, err = fmt.Fprintln(cmd.stdout, "no wake scheduled or uncertain usage notice")
			} else {
				_, err = fmt.Fprintln(cmd.stdout, "no wake scheduled")
			}
			return err
		}
		sort.Strings(rows)
		_, err = fmt.Fprintln(cmd.stdout, strings.Join(rows, "\n"))
		return err
	}
	if clear {
		if len(positionals) == 1 {
			if !isLead {
				return refuseError("only the lead can clear another usage intent by ID")
			}
			id := core.EnvelopeID(positionals[0])
			found := false
			var events []core.Event
			now := cmd.now()
			if err := run.withQueuedWake(func(state *usageState) core.HitchID {
				for _, s := range state.Snoozes {
					if s.ID == id && s.RecipientID == a.ID {
						return queuedWakeRecipient(s)
					}
				}
				return ""
			}, func(state *usageState, withdraw func(usageSnooze) error) error {
				for i, n := range state.Notices {
					if n.ID == id && n.RecipientID == a.ID && n.Submission != "" {
						state.Notices = append(state.Notices[:i], state.Notices[i+1:]...)
						found = true
						return nil
					}
				}
				for caller, s := range state.Snoozes {
					if s.ID == id && s.RecipientID == a.ID {
						if err := withdraw(s); err != nil {
							return err
						}
						delete(state.Snoozes, caller)
						found = true
						events = append(events, wakeEvent("snooze_cleared", s, now, "cleared by lead "+string(a.Name)))
						return nil
					}
				}
				for caller, s := range state.Recent {
					if s.ID == id && s.RecipientID == a.ID {
						delete(state.Recent, caller)
						found = true
						events = append(events, wakeEvent("snooze_cleared", s, now, "cleared by lead "+string(a.Name)))
						return nil
					}
				}
				return nil
			}); err != nil {
				return err
			}
			if !found {
				return refuseError("usage intent %s is not awaiting review for this lead", id)
			}
			if err := run.appendEvents(events); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.stdout, "%s\tcleared\n", id)
			return err
		}
		var events []core.Event
		now := cmd.now()
		if err := run.withQueuedWake(func(state *usageState) core.HitchID {
			return queuedWakeRecipient(state.Snoozes[key])
		}, func(state *usageState, withdraw func(usageSnooze) error) error {
			if err := withdraw(state.Snoozes[key]); err != nil {
				return err
			}
			for _, s := range []usageSnooze{state.Snoozes[key], state.Recent[key]} {
				if s.ID != "" {
					events = append(events, wakeEvent("snooze_cleared", s, now, "cleared by its agent"))
				}
			}
			delete(state.Snoozes, key)
			delete(state.Recent, key)
			delete(state.AutoCandidates, key)
			return nil
		}); err != nil {
			return err
		}
		if err := run.appendEvents(events); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.stdout, "wake cleared")
		return err
	}
	now := cmd.now()
	var due time.Time
	reason := "native usage reset"
	if at != "" {
		reason = "explicit time"
		due, err = parseSchedule(at, now)
		if err != nil {
			return usageError("snooze: invalid --at %q (%v)", at, err)
		}
	} else {
		updated, err := run.latest(*a)
		if err != nil {
			return err
		}
		r := updated.Native.Limits
		if r.Status != "observed" || r.At == nil || r.At.After(now) || now.Sub(*r.At) > 5*time.Minute {
			return refuseError("native provider limits are unavailable or stale; use gang limits or --at")
		}
		due, err = snoozeReset(r.Limits, now)
		if err != nil {
			return refuseError("%v", err)
		}
	}
	if !due.After(now) {
		return usageError("snooze: wake time must be in the future")
	}
	if strings.TrimSpace(note) == "" {
		note = defaultSnoozeNote
	}
	// An overdue wake goes to the lead when the caller is gone.
	if reason, err := harness.AnyPasteHazard(note); err != nil {
		return err
	} else if reason != "" {
		return refuseError("%s", reason)
	}
	id, err := randomID("snooze")
	if err != nil {
		return err
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	s := usageSnooze{ID: core.EnvelopeID(id), Token: token, CallerID: a.ID, CallerName: a.Name, At: due, Note: note}
	// Delivery can be overdue and routed to the lead. Validate that longest
	// rendered form before accepting a note that could never be delivered.
	s.RecipientID = "lead-fallback"
	maximumOverdue := due.Add(time.Duration(1<<63 - 1))
	if _, err := envelopeText(core.Envelope{ID: s.ID, Token: s.Token, From: core.Sender{Kind: core.SenderGangline, Name: "snooze"}, Message: core.Message{Text: snoozeWakeText(s, maximumOverdue)}}); err != nil {
		return usageError("snooze: %v", err)
	}
	s.RecipientID = ""
	var events []core.Event
	if err := run.withQueuedWake(func(state *usageState) core.HitchID {
		return queuedWakeRecipient(state.Snoozes[key])
	}, func(state *usageState, withdraw func(usageSnooze) error) error {
		current := state.Snoozes[key]
		if current.RecipientID != "" && current.Submission != "" {
			return refuseError("previous wake is already submitted; inspect it with --status or clear it")
		}
		if recent := state.Recent[key]; recent.ID != "" && !recent.CapRejected && !recent.TurnFailed {
			return refuseError("previous wake has an unresolved native outcome; inspect it with --status or clear it")
		}
		if err := withdraw(current); err != nil {
			return err
		}
		for _, old := range []usageSnooze{current, state.Recent[key]} {
			if old.ID != "" {
				events = append(events, wakeEvent("snooze_cleared", old, now, "replaced by "+string(s.ID)))
			}
		}
		events = append(events, wakeEvent("snooze_scheduled", s, now, reason))
		delete(state.Recent, key)
		state.Snoozes[key] = s
		return nil
	}); err != nil {
		return err
	}
	if err := run.appendEvents(events); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\n", s.ID, due.UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return run.ensureWatchdog()
}
