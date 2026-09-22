package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
)

func (cmd command) send(arguments []string) error {
	options, err := parseSend(arguments)
	if err != nil {
		return err
	}
	run, state, err := cmd.loaded()
	if err != nil {
		return err
	}
	if options.At == "clear" {
		_, err = run.drive(core.TimedDeliveriesCleared{At: time.Now(), Recipient: core.AgentName(options.Name)})
		return err
	}
	body, err := readBody(cmd.stdin)
	if err != nil {
		return err
	}
	sender, err := cmd.sender(state, options.From)
	if err != nil {
		return err
	}
	if options.Supersede {
		if _, err := run.drive(core.TimedDeliveriesCleared{At: time.Now(), Recipient: core.AgentName(options.Name)}); err != nil {
			return err
		}
	}
	hitch, found := activeByName(state, options.Name)
	if !found {
		return refuseError("agent %q is not active", options.Name)
	}
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return err
	}
	if options.LiveOnly {
		hitch, ok := activeByName(state, options.Name)
		if !ok || (hitch.Activity != core.ActivityIdle && !(hitch.Activity == core.ActivityBusy && collar.Primitives.MidTurn)) {
			return refuseError("agent %q is not immediately deliverable", options.Name)
		}
	}
	now := time.Now()
	var notBefore time.Time
	if options.At != "" {
		notBefore, err = parseSchedule(options.At, now)
		if err != nil {
			return usageError("send: --at: %v", err)
		}
	}
	id, err := randomID("msg")
	if err != nil {
		return err
	}
	deadline := now.Add(run.deliveryBudget())
	if !notBefore.IsZero() {
		deadline = notBefore.Add(run.deliveryBudget())
	}
	state, err = run.drive(core.SendRequested{
		At: now, Deadline: deadline, NotBefore: notBefore, MidTurn: collar.Primitives.MidTurn,
		Envelope: core.Envelope{ID: core.EnvelopeID(id), From: sender, To: core.AgentName(options.Name), Message: core.Message{Text: body}, CreatedAt: now},
	})
	if err != nil {
		return err
	}
	if candidate := state.Deliveries[core.EnvelopeID(id)]; candidate.Status == core.DeliveryQueued && candidate.MidTurn && candidate.NotBefore.IsZero() {
		state, err = run.awaitDelivery(state, core.EnvelopeID(id))
		if err != nil {
			return err
		}
	}
	delivery, ok := state.Deliveries[core.EnvelopeID(id)]
	if !ok {
		return refuseError("send was rejected; inspect 'gang log'")
	}
	if delivery.Status == core.DeliveryUnverified {
		return commandError{status: exitUnknown, text: "delivery may have landed but could not be verified: " + delivery.Reason}
	}
	if delivery.Status == core.DeliveryFailed {
		return refuseError("delivery failed: %s", delivery.Reason)
	}
	if delivery.Status == core.DeliveryQueued {
		_, err = fmt.Fprintf(cmd.stdout, "%s\tqueued\n", id)
		return err
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s\t%s\n", id, delivery.Status)
	return err
}

func (cmd command) sender(state core.State, declared string) (core.Sender, error) {
	if pane := cmd.environment("TMUX_PANE"); pane != "" {
		for _, hitch := range state.Hitches {
			if hitch.Pane == pane && hitch.Status == core.HitchActive {
				if declared != "" && declared != string(hitch.Name) {
					return core.Sender{}, refuseError("--from cannot override observed pane identity %q", hitch.Name)
				}
				return core.Sender{Kind: core.SenderAgent, Name: hitch.Name, HitchID: hitch.ID}, nil
			}
		}
	}
	if declared == "" {
		return core.Sender{}, refuseError("sender is outside the team; provide --from NAME")
	}
	return core.Sender{Kind: core.SenderSelfDeclared, Name: core.AgentName(declared)}, nil
}

func (cmd command) queue(arguments []string) error {
	if len(arguments) > 1 {
		return usageError("queue: expected at most one agent")
	}
	_, state, err := cmd.loaded()
	if err != nil {
		return err
	}
	name := ""
	if len(arguments) == 1 {
		name = arguments[0]
	}
	for _, id := range state.DeliveryOrder {
		delivery := state.Deliveries[id]
		if delivery.Status != core.DeliveryQueued || name != "" && string(delivery.Envelope.To) != name {
			continue
		}
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", id, delivery.Envelope.To, delivery.Envelope.From.Name); err != nil {
			return err
		}
	}
	return nil
}

func (cmd command) interrupt(arguments []string) error {
	name := ""
	if len(arguments) != 0 && !strings.HasPrefix(arguments[0], "-") {
		name, arguments = arguments[0], arguments[1:]
	}
	reason := ""
	flags := quietFlagSet("interrupt")
	flags.StringVar(&reason, "m", "", "reason")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return usageError("interrupt: invalid arguments")
	}
	run, state, err := cmd.loaded()
	if err != nil {
		return err
	}
	if name == "" {
		name = nameAtPane(state, cmd.environment("TMUX_PANE"))
		if name == "" {
			name = string(state.Team.Name)
		}
	}
	hitch, ok := activeByName(state, name)
	if !ok {
		return refuseError("agent %q is not active", name)
	}
	if hitch.Activity != core.ActivityBusy && hitch.Activity != core.ActivityWedged {
		return refuseError("agent %q is not in an interruptible turn", name)
	}
	now := time.Now()
	_, err = run.drive(core.InterruptRequested{At: now, HitchID: hitch.ID, Reason: reason, Deadline: now.Add(operationTimeout)})
	return err
}

func (cmd command) compact(arguments []string) error {
	options, err := parseCompact(arguments)
	if err != nil {
		return err
	}
	run, state, err := cmd.loaded()
	if err != nil {
		return err
	}
	name := options.Name
	if name == "" {
		name = nameAtPane(state, cmd.environment("TMUX_PANE"))
	}
	hitch, ok := activeByName(state, name)
	if !ok {
		return refuseError("agent %q is not active", name)
	}
	if options.Recover {
		collar, err := loadCollar(hitch.Collar, run.settings)
		if err != nil {
			return err
		}
		backend, err := cmd.tmux(run.settings)
		if err != nil {
			return err
		}
		for _, action := range collar.Actions.CompactRecover {
			if err := sendHarnessKeys(context.Background(), backend, substrate.PaneID(hitch.Pane), collar, action.Input()); err != nil {
				return err
			}
		}
		if hitch.PendingCompactID != "" {
			_, err = run.drive(core.CompactionFailedEvent{At: time.Now(), CompactionID: hitch.PendingCompactID, Reason: "operator requested native compaction recovery"})
		}
		return err
	}
	resume := options.Resume
	if resume == "" {
		resume = "Your context was just compacted. Re-read your brief and durable state, then resume your lane or report it complete."
	}
	id, err := randomID("compact")
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = run.drive(core.CompactionRequested{At: now, Compaction: core.Compaction{ID: core.CompactionID(id), HitchID: hitch.ID, Resume: core.Message{Text: resume}, Deadline: now.Add(operationTimeout)}})
	return err
}
