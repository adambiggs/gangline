package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func (run *runtime) sender(declared string) (core.Sender, error) {
	sender, err := run.observedSender()
	if err != nil {
		return core.Sender{}, err
	}
	if sender.Kind != "" {
		if declared != "" {
			return core.Sender{}, refuseError("--from is not allowed inside a registered agent pane")
		}
		return sender, nil
	}
	if declared == "" {
		return core.Sender{}, refuseError("sender is outside the team; provide --from NAME")
	}
	return core.Sender{Kind: core.SenderSelfDeclared, Name: core.AgentName(declared)}, nil
}

func (run *runtime) observedSender() (core.Sender, error) {
	if pane := run.cmd.environment("TMUX_PANE"); pane != "" {
		agents, err := run.team.ListAgents()
		if err != nil {
			return core.Sender{}, err
		}
		for _, a := range agents {
			if a.Pane == pane && a.Status == core.Active {
				return core.Sender{Kind: core.SenderAgent, Name: a.Name, HitchID: a.ID}, nil
			}
		}
	}
	return core.Sender{}, nil
}
func (cmd command) send(args []string) (result error) {
	o, err := parseSend(args)
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	a, err := run.resolve(o.Name)
	if err != nil {
		return err
	}
	sender, err := run.sender(o.From)
	if err != nil {
		return err
	}
	if sender.HitchID == a.ID {
		return refuseError("sender and recipient are the same hitch")
	}
	if a.Status != core.Active && a.Status != core.Booting {
		return refuseError("recipient is not active")
	}
	p, err := run.team.Agent(a.ID)
	if err != nil {
		return err
	}
	_, failedStartup, err := retainedStartup(p, "failed", a.LastFailed)
	if err != nil {
		return err
	}
	if failedStartup && o.At != "clear" {
		return refuseError("startup contract input is unverified; inspect the recipient and run gang hitch %s --recover before sending another message", a.Name)
	}
	now := cmd.now()
	var e core.Envelope
	if o.At != "clear" {
		bodyReader := cmd.stdin
		if o.Body != nil {
			bodyReader = strings.NewReader(*o.Body)
		}
		body, err := readBody(bodyReader)
		if err != nil {
			return err
		}
		var due time.Time
		if o.At != "" {
			due, err = parseSchedule(o.At, now)
			if err != nil {
				return usageError("send: --at: %v", err)
			}
		}
		id, err := randomID("msg")
		if err != nil {
			return err
		}
		token, err := randomEnvelopeToken()
		if err != nil {
			return err
		}
		e = core.Envelope{ID: core.EnvelopeID(id), Token: token, Recipient: a.ID, To: a.Name, From: sender, Message: core.Message{Text: body}, CreatedAt: now, NotBefore: due}
		if _, err := envelopeText(e); err != nil {
			return err
		}
	}
	var l *store.LockedAgent
	if o.Supersede || o.At == "clear" || o.LiveOnly {
		l, a, err = run.acquire(a.ID, false)
		if errors.Is(err, store.ErrLocked) {
			return refuseError("recipient is busy with an input operation")
		}
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, run.release(l)) }()
	}
	if o.Supersede || o.At == "clear" {
		removed, err := l.ClearScheduled(sender)
		if err != nil {
			return err
		}
		for _, e := range removed {
			if err := run.record(a, core.Event{Type: "send_cancelled", ID: string(e.ID), Reason: "sender cleared scheduled delivery"}); err != nil {
				return err
			}
		}
		if o.At == "clear" {
			return nil
		}
	}
	if o.LiveOnly {
		if err := run.checkDeadlines(l, &a); err != nil {
			return err
		}
		c, err := loadCollar(a.Collar, run.settings)
		if err != nil {
			return err
		}
		b, err := run.input()
		if err != nil {
			return err
		}
		free, reason, err := run.available(l, &a, b, c)
		if err != nil {
			return err
		}
		if !free {
			if reason != "" {
				return refuseError("recipient cannot accept input now: %s", reason)
			}
			return refuseError("recipient cannot accept input now")
		}
		queued, err := p.ListNew()
		if err != nil {
			return err
		}
		for _, pending := range queued {
			if !pending.NotBefore.After(now) {
				return refuseError("recipient has earlier due messages")
			}
		}
	}
	if err := p.Publish(e); err != nil {
		return err
	}
	if err := run.record(a, core.Event{Type: "send_queued", Envelope: &e}); err != nil {
		return err
	}
	outcome := "queued"
	if l != nil {
		outcome, err = run.drainFrom(l, a, e.ID)
	} else {
		outcome, err = run.drain(a.ID, e.ID)
	}
	if err != nil {
		return err
	}
	// A hook may promote this receipt after drain releases the input lock.
	// Read this ID's retained receipt, not another message's latest result.
	if receipt, readErr := p.ReadEnvelope("cur", e.ID); readErr == nil {
		outcome = receipt.Outcome
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	if outcome == "accepted" {
		if _, err := fmt.Fprintln(cmd.stderr, "native queue accepted the message; do not resend"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\n", e.ID, outcome); err != nil {
		return err
	}
	return deliveryResult(outcome)
}
func (cmd command) queue(args []string) error {
	if len(args) > 1 {
		return usageError("queue: expected at most one agent")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	var agents []core.Agent
	if len(args) == 1 {
		a, err := run.resolve(args[0])
		if err != nil {
			return err
		}
		agents = []core.Agent{a}
	} else {
		agents, err = run.team.ListAgents()
		if err != nil {
			return err
		}
	}
	for _, a := range agents {
		p, err := run.team.Agent(a.ID)
		if err != nil {
			return err
		}
		messages, err := p.ListNew()
		if err != nil {
			return err
		}
		for _, e := range messages {
			if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", e.ID, a.Name, e.From.Name); err != nil {
				return err
			}
		}
	}
	return nil
}
func (cmd command) interrupt(args []string) (result error) {
	name := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	reason := ""
	flags := boundFlagSet("interrupt", map[string]any{"m": &reason})
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return usageError("interrupt: invalid arguments")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	a, err := run.resolve(name)
	if err != nil {
		return err
	}
	l, a, err := run.acquire(a.ID, false)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, run.release(l)) }()
	if a.Status != core.Active {
		return refuseError("recipient is not active")
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	if err := run.apply(l, &a, core.Event{Type: "interrupt_requested", Deadline: cmd.now().Add(operationTimeout)}); err != nil {
		return err
	}
	id, err := randomID("interrupt")
	if err != nil {
		return err
	}
	if err := run.apply(l, &a, core.Event{Type: "input_started", ID: id, Status: "interrupt"}); err != nil {
		return err
	}
	if err := sendHarnessKeys(context.Background(), b, substrate.PaneID(a.Pane), c, c.Actions.Interrupt.Input()); err != nil {
		return err
	}
	a.Input = nil
	if err := l.Save(a); err != nil {
		return err
	}
	if reason != "" {
		token, err := randomEnvelopeToken()
		if err != nil {
			return err
		}
		e := core.Envelope{ID: core.EnvelopeID(id), Token: token, Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "interrupt"}, Message: core.Message{Text: reason}, CreatedAt: cmd.now()}
		if err := l.Paths.Publish(e); err != nil {
			return err
		}
		if err := run.record(a, core.Event{Type: "send_queued", Envelope: &e}); err != nil {
			return err
		}
	}
	return run.mark(a)
}
func (cmd command) compact(args []string) (result error) {
	o, err := parseCompact(args)
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	a, err := run.resolve(o.Name)
	if err != nil {
		return err
	}
	l, a, err := run.acquire(a.ID, false)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, run.release(l)) }()
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	if o.Recover {
		if a.Compaction == nil {
			return refuseError("no compaction to recover")
		}
		if err := run.apply(l, &a, core.Event{Type: "input_started", ID: a.Compaction.ID, Status: "compaction"}); err != nil {
			return err
		}
		for _, action := range c.Actions.CompactRecover {
			if err := sendHarnessKeys(context.Background(), b, substrate.PaneID(a.Pane), c, action.Input()); err != nil {
				return err
			}
		}
		return run.apply(l, &a, core.Event{Type: "compaction_unverified", ID: a.Compaction.ID, Reason: "operator requested recovery; inspect the harness before retrying"})
	}
	if a.Status != core.Active {
		return refuseError("recipient is not active")
	}
	resumeFrom := core.Sender{Kind: core.SenderGangline, Name: "compact"}
	if o.Resume != "" {
		resumeFrom, err = run.observedSender()
		if err != nil {
			return err
		}
		if resumeFrom.Kind == "" {
			resumeFrom = core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"}
		}
	}
	resume := o.Resume
	if resume == "" {
		resume = "Your context was compacted. Re-read your brief and durable state, then resume your work or report it complete."
	}
	id, err := randomID("compact")
	if err != nil {
		return err
	}
	if _, err := envelopeText(core.Envelope{Token: "0123456789abcdef", From: resumeFrom, Message: core.Message{Text: resume}}); err != nil {
		return err
	}
	now := cmd.now()
	compact := core.Compaction{ID: id, Resume: core.Message{Text: resume}, ResumeFrom: resumeFrom, StartedAt: now, Deadline: now.Add(operationTimeout), Status: "queued"}
	if err := run.apply(l, &a, core.Event{Type: "compaction_requested", Compaction: &compact}); err != nil {
		return err
	}
	if err := run.startCompaction(l, &a); err != nil {
		return err
	}
	if a.Compaction.Status == "queued" {
		_, err := fmt.Fprintf(cmd.stdout, "%s\tqueued; waiting for native idle; resume follows confirmed completion\n", id)
		return err
	}
	if a.Compaction.Status != "completed" {
		return commandError{status: exitUnknown, text: "compaction submitted; native completion unconfirmed; resume follows confirmed completion"}
	}
	outcome, err := run.drainFrom(l, a, core.EnvelopeID("resume-"+id))
	if err != nil {
		return err
	}
	if err := deliveryResult(outcome); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s\tcompleted; resume %s\n", id, outcome)
	return err
}
func (run *runtime) startCompaction(l *store.LockedAgent, a *core.Agent) (result error) {
	if a.Compaction == nil || a.Compaction.Status != "queued" || a.Status != core.Active {
		return nil
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	pane := substrate.PaneID(a.Pane)
	screen, err := b.Capture(ctx, pane)
	if err != nil {
		return errors.Join(err, run.observeProbeFailure(l, a, err))
	}
	if err := run.observeActivity(l, a, c, screen); err != nil {
		return err
	}
	if a.Activity != core.Idle {
		return nil
	}
	free, _, err := run.available(l, a, b, c)
	if err != nil || !free {
		return err
	}
	refusals, err := harness.ActionRefusals(c.Actions.Compact, screen)
	if err != nil {
		return err
	}
	a.Compaction.RefusalBefore = len(refusals)
	a.Compaction.StartedAt = run.cmd.now()
	a.Compaction.Deadline = a.Compaction.StartedAt.Add(operationTimeout)
	if err := run.apply(l, a, core.Event{Type: "input_started", ID: a.Compaction.ID, Status: "compaction"}); err != nil {
		return err
	}
	defer func() {
		if result != nil && a.Input != nil {
			result = errors.Join(result, run.apply(l, a, core.Event{Type: "compaction_unverified", ID: a.Compaction.ID, Reason: result.Error()}))
		}
	}()
	action, err := harness.RenderAction(c.Actions.Compact, map[string]string{"instructions": a.Compaction.Resume.Text})
	if err != nil {
		return err
	}
	input, err := harness.SubmitInput(c.Primitives.Submit, action.Text)
	if err != nil {
		return err
	}
	if err := sendHarnessKeys(ctx, b, pane, c, input); err != nil {
		return err
	}
	settle, err := harness.SubmitSettle(c.Primitives.Submit)
	if err != nil {
		return err
	}
	if run.cmd.settleInput != nil {
		err = run.cmd.settleInput(ctx, b, pane, c, settle)
	} else {
		err = harness.AwaitComposerSettle(ctx, b.Capture, pane, c, settle)
	}
	if err != nil {
		return err
	}
	screen, err = b.Capture(ctx, pane)
	if err != nil {
		return err
	}
	if blocker, blocked, err := harness.InputBlocked(c, screen); err != nil {
		return err
	} else if blocked {
		return commandError{status: exitNative, text: "native compaction input blocked: " + blocker.Evidence + "; compaction not submitted"}
	}
	if _, err := harness.ReadComposer(c.Primitives.Composer, screen); err != nil {
		return err
	}
	if busy, err := harness.Busy(c, screen); err != nil {
		return err
	} else if busy {
		return commandError{status: exitNative, text: "native task became active before compaction submit; inspect the composer; compaction not submitted"}
	}
	if err := sendHarnessKeys(ctx, b, pane, c, substrate.Keys{Names: action.Keys, Submit: action.Submit}); err != nil {
		return err
	}
	if err := run.apply(l, a, core.Event{Type: "compaction_submitted", ID: a.Compaction.ID}); err != nil {
		return err
	}
	if err := run.refreshNative(l, a, c); err != nil {
		return err
	}
	screen, err = b.Capture(ctx, pane)
	if err != nil {
		return err
	}
	if err := run.observeCompaction(l, a, c, screen); err != nil {
		return err
	}
	if err := run.continueCompaction(l, a); err != nil {
		return err
	}
	return run.mark(*a)
}
