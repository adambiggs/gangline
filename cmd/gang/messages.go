package main

import (
	"fmt"
	"time"

	"github.com/adambiggs/gangline/core"
)

func (cmd command) send(arguments []string) error {
	options, err := parseSend(arguments)
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
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
	if options.LiveOnly {
		hitch, ok := activeByName(state, options.Name)
		if !ok || hitch.Activity != core.ActivityIdle {
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
	state, err = run.drive(core.SendRequested{
		At: now, Deadline: now.Add(deliveryTimeout), NotBefore: notBefore,
		Envelope: core.Envelope{ID: core.EnvelopeID(id), From: sender, To: core.AgentName(options.Name), Message: core.Message{Text: body}, CreatedAt: now},
	})
	if err != nil {
		return err
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
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
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
