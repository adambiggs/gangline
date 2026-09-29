package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

const defaultSnoozeNote = "Re-read your assignment and durable state, then continue only if work remains."

func snoozeWakeText(s usageSnooze, now time.Time) string {
	when := s.At.UTC().Format(time.RFC3339)
	message := fmt.Sprintf("Your scheduled wake was due at %s.", when)
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
	case s.CapCandidate:
		return "rate limit unconfirmed by native usage; inspect or clear"
	case s.CapRejected && s.Rearms > 0:
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

func (cmd command) snooze(args []string) error {
	var at, note string
	var clear, status bool
	flags := boundFlagSet("snooze", map[string]any{"at": &at, "note": &note, "clear": &clear, "status": &status})
	positionals, err := parseOptions(flags, args)
	if err != nil || len(positionals) > 1 || len(positionals) == 1 && !clear {
		return usageError("snooze: expected --at TIME, --note TEXT, --clear [ID], or --status")
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
	if a == nil || a.Status != core.Active {
		return refuseError("snooze is available only to a registered active agent")
	}
	key := string(a.ID)
	isLead := a.Role == "lead" || a.Name == "lead"
	if status {
		var rows []string
		if err := run.withUsageState(func(state *usageState) error {
			if current := state.Snoozes[key]; current.ID != "" {
				if current.Submission != "" {
					rows = append(rows, fmt.Sprintf("%s\t%s", current.ID, snoozeStatusText(current, true)))
				} else {
					rows = append(rows, fmt.Sprintf("%s\t%s", current.ID, current.At.UTC().Format(time.RFC3339)))
				}
			} else if current := state.Recent[key]; current.ID != "" {
				rows = append(rows, fmt.Sprintf("%s\t%s", current.ID, snoozeStatusText(current, false)))
			}
			if isLead {
				for _, n := range state.Notices {
					if n.RecipientID == a.ID && n.Submission != "" {
						rows = append(rows, fmt.Sprintf("%s\t%s %s %s notice; native %s; inspect or clear by ID", n.ID, n.Collar, n.Window, n.Band, n.Submission))
					}
				}
				for caller, s := range state.Snoozes {
					if caller != key && s.RecipientID == a.ID && s.Submission != "" {
						rows = append(rows, fmt.Sprintf("%s\twake for %s; %s", s.ID, s.CallerName, snoozeStatusText(s, true)))
					}
				}
				for caller, s := range state.Recent {
					if caller != key && s.RecipientID == a.ID {
						rows = append(rows, fmt.Sprintf("%s\twake for %s; %s", s.ID, s.CallerName, snoozeStatusText(s, false)))
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
			if err := run.withUsageState(func(state *usageState) error {
				for i, n := range state.Notices {
					if n.ID == id && n.RecipientID == a.ID && n.Submission != "" {
						state.Notices = append(state.Notices[:i], state.Notices[i+1:]...)
						found = true
						return nil
					}
				}
				for caller, s := range state.Snoozes {
					if s.ID == id && s.RecipientID == a.ID && s.Submission != "" {
						delete(state.Snoozes, caller)
						found = true
						return nil
					}
				}
				for caller, s := range state.Recent {
					if s.ID == id && s.RecipientID == a.ID {
						delete(state.Recent, caller)
						found = true
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
			_, err = fmt.Fprintf(cmd.stdout, "%s\tcleared\n", id)
			return err
		}
		if err := run.withUsageState(func(state *usageState) error {
			if current := state.Snoozes[key]; current.RecipientID != "" && current.Submission == "" {
				return refuseError("wake is already due or submitted; inspect its delivery before clearing")
			}
			delete(state.Snoozes, key)
			delete(state.Recent, key)
			return nil
		}); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.stdout, "wake cleared")
		return err
	}
	now := cmd.now()
	var due time.Time
	if at != "" {
		due, err = parseSchedule(at, now)
		if err != nil {
			return usageError("snooze: --at: %v", err)
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
	if err := run.withUsageState(func(state *usageState) error {
		if current := state.Snoozes[key]; current.RecipientID != "" {
			return refuseError("previous wake is already due or submitted; inspect its delivery before replacing it")
		}
		if recent := state.Recent[key]; recent.ID != "" && !recent.CapRejected && !recent.TurnFailed {
			return refuseError("previous wake awaits a successful native turn; inspect it with --status or clear it")
		}
		delete(state.Recent, key)
		state.Snoozes[key] = s
		return nil
	}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s\t%s\n", s.ID, due.UTC().Format(time.RFC3339))
	return err
}
