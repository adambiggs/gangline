package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"
	"unicode/utf8"

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
	a, err := run.observedAgent()
	if err != nil || a == nil {
		return core.Sender{}, err
	}
	if a.Status == core.Active {
		return core.Sender{Kind: core.SenderAgent, Name: a.Name, HitchID: a.ID}, nil
	}
	return core.Sender{}, nil
}

func (run *runtime) observedAgent() (*core.Agent, error) {
	if id := core.HitchID(run.cmd.environment("GANG_AGENT_ID")); id != "" {
		p, err := run.team.Agent(id)
		if err != nil {
			return nil, refuseError("hitch identity is not registered")
		}
		a, err := p.Read()
		if err != nil {
			return nil, refuseError("hitch identity is not registered: %v", err)
		}
		if a.Pane == "" {
			return nil, refuseError("hitch identity %s has no registered pane; re-hitch this agent", a.Name)
		}
		if a.Status != core.Active && a.Status != core.Booting {
			return nil, refuseError("hitch identity %s is %s, not active; re-hitch this agent", a.Name, a.Status)
		}
		if pane := run.cmd.environment("TMUX_PANE"); pane != "" && pane != a.Pane {
			return nil, refuseError("hitch identity does not match the current pane")
		}
		if err := run.verifyCaller(a); err != nil {
			return nil, err
		}
		return &a, nil
	}
	if pane := run.cmd.environment("TMUX_PANE"); pane != "" {
		agents, err := run.team.ListAgents()
		if err != nil {
			return nil, err
		}
		for _, a := range agents {
			if a.Pane == pane && (a.Status == core.Active || a.Status == core.Booting) {
				return nil, refuseError("registered pane requires its inherited hitch identity; re-hitch this agent")
			}
		}
	}
	return nil, nil
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
		return inactiveRecipient(a)
	}
	p, err := run.team.Agent(a.ID)
	if err != nil {
		return err
	}
	_, failedStartup, err := retainedStartup(p, "failed", a.LastFailed)
	if err != nil {
		return err
	}
	if failedStartup && !o.Clear {
		return refuseError("startup contract input is unverified; inspect the recipient and run gang hitch %s --recover before sending another message", a.Name)
	}
	now := cmd.now()
	var e core.Envelope
	if !o.Clear {
		var due time.Time
		if o.At != "" {
			due, err = parseSchedule(o.At, now)
			if err != nil {
				return usageError("send: invalid --at %q (%v)", o.At, err)
			}
		}
		bodyReader := cmd.stdin
		if o.Body != nil {
			bodyReader = strings.NewReader(*o.Body)
		}
		body, err := readBody(bodyReader)
		if err != nil {
			return err
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
	if o.Supersede || o.Clear || o.LiveOnly {
		l, a, err = run.acquire(a.ID, false)
		if errors.Is(err, store.ErrLocked) {
			return refuseError("recipient is busy with an input operation; retry")
		}
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, run.release(l)) }()
	}
	if o.Supersede || o.Clear {
		removed, err := l.ClearScheduled(sender)
		if err != nil {
			return err
		}
		for _, e := range removed {
			if err := run.record(a, core.Event{Type: "send_cancelled", ID: string(e.ID), Reason: "sender cleared scheduled delivery"}); err != nil {
				return err
			}
		}
		if o.Clear {
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
	if err := run.checkRecipient(a); err != nil {
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
	if err := refusePasteHazard(c, e.Message.Text); err != nil {
		return err
	}
	if err := requireHarnessForeground(context.Background(), b, substrate.PaneID(a.Pane), c); err != nil {
		return err
	}
	if err := p.Publish(e); err != nil {
		return err
	}
	outcome := "queued"
	step := "record queue event"
	postPublishErr := run.record(a, core.Event{Type: "send_queued", Envelope: &e})
	if postPublishErr == nil {
		step = "drain"
		if l != nil {
			outcome, postPublishErr = run.drainFrom(l, a, e.ID)
		} else {
			outcome, postPublishErr = run.drain(a.ID, e.ID)
		}
	}
	// A hook may promote this receipt after drain releases the input lock.
	// Read this ID's retained receipt, not another message's latest result.
	if receipt, readErr := p.ReadEnvelope("cur", e.ID); readErr == nil {
		outcome = receipt.Outcome
	} else if !errors.Is(readErr, os.ErrNotExist) {
		if postPublishErr == nil {
			step = "read receipt"
		}
		postPublishErr = errors.Join(postPublishErr, fmt.Errorf("read receipt: %w", readErr))
	}
	if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\n", e.ID, outcome); err != nil {
		return err
	}
	if outcome == "accepted" {
		if _, err := fmt.Fprintln(cmd.stderr, "native queue accepted the message; do not resend"); err != nil {
			return err
		}
	}
	if postPublishErr != nil {
		if _, err := fmt.Fprintf(cmd.stderr, "warning: message %s was retained with %s status; %s failed: %v; do not resend\n", e.ID, outcome, step, postPublishErr); err != nil {
			return err
		}
	}
	return deliveryResult(outcome)
}
func (cmd command) queue(args []string) error {
	machine := false
	flags := boundFlagSet("queue", map[string]any{"json": &machine})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("queue: %v", err)
	}
	if len(positionals) > 1 {
		return usageError("queue: expected at most one agent")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	var agents []core.Agent
	if len(positionals) == 1 {
		a, err := run.resolve(positionals[0])
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
	rows := []queueRow{}
	for _, a := range agents {
		pending, err := run.pendingRows(a)
		if err != nil {
			return err
		}
		rows = append(rows, pending...)
	}
	if machine {
		return writeJSON(cmd.stdout, queueJSON{Messages: rows})
	}
	if len(rows) == 0 {
		return nil
	}
	w := tabwriter.NewWriter(cmd.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTO\tFROM\tKIND\tSTATE\tDETAIL\tTEXT")
	for _, row := range rows {
		detail := valueOr(row.Reason, "-")
		if row.DueAt != nil {
			detail = "due " + row.DueAt.Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row.ID, row.To, senderLabel(row.From), row.Kind, row.State, detail, row.Excerpt)
	}
	return w.Flush()
}

// Queue states. A ready message's recipient takes input at its next drain;
// unknown means the recipient could not be observed.
const (
	queueReady     = "ready"
	queueScheduled = "scheduled"
	queueBlocked   = "blocked"
	queueUnknown   = "unknown"
)

// pendingRows classifies a's pending messages with the rules drainLocked
// applies, observing the recipient without delivering or withdrawing anything.
// Releasing the lock starts a tick for due input, as other state readers do.
func (run *runtime) pendingRows(a core.Agent) (rows []queueRow, result error) {
	l, current, err := run.acquire(a.ID, false)
	locked := errors.Is(err, store.ErrLocked)
	if err != nil && !locked {
		return nil, err
	}
	var pending []core.Envelope
	if locked {
		p, err := run.team.Agent(a.ID)
		if err != nil {
			return nil, err
		}
		if pending, err = p.ListNew(); err != nil {
			return nil, err
		}
	} else {
		defer func() { result = errors.Join(result, run.release(l)) }()
		if err := run.checkDeadlines(l, &current); err != nil {
			return nil, err
		}
		if pending, err = l.Paths.ListNew(); err != nil {
			return nil, err
		}
	}
	now := run.cmd.now()
	rows = make([]queueRow, 0, len(pending))
	var head, resume *core.Envelope
	var verdict *queueRow
	for i, e := range pending {
		row := queueRow{ID: e.ID, To: a.Name, HitchID: a.ID, From: e.From, Kind: valueOr(e.Purpose, "message"), CreatedAt: e.CreatedAt, Excerpt: messageExcerpt(e.Message.Text)}
		expired := !e.NotAfter.IsZero() && !e.NotAfter.After(now)
		switch {
		case !expired && e.NotBefore.After(now):
			due := e.NotBefore
			row.State, row.DueAt = queueScheduled, &due
		case locked:
			row.State, row.Reason = queueUnknown, "agent state is locked by another gang operation"
		case current.Status != core.Active:
			row.State, row.Reason = queueBlocked, "recipient is "+string(current.Status)
		case expired:
			row.State, row.Reason = queueBlocked, "expired; the next delivery withdraws it"
		case resume != nil:
			row.State, row.Reason = queueBlocked, "behind compaction resume "+string(resume.ID)
		case isResumeEnvelope(e):
			row.State, row.Reason = queueBlocked, "waits for its compaction to start"
			resume = &pending[i]
		default:
			if verdict == nil {
				v, err := run.queueVerdict(l, &current)
				if err != nil {
					return nil, err
				}
				verdict = &v
			}
			row.State, row.Reason = verdict.State, verdict.Reason
			if head != nil && row.State == queueReady {
				row.Reason = "after " + string(head.ID)
			}
		}
		if head == nil && !expired && row.DueAt == nil {
			head = &pending[i]
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// queueVerdict says whether the next due message would be delivered now.
func (run *runtime) queueVerdict(l *store.LockedAgent, a *core.Agent) (queueRow, error) {
	_, failedStartup, err := retainedStartup(l.Paths, "failed", a.LastFailed)
	if err != nil {
		return queueRow{}, err
	}
	if failedStartup {
		return queueRow{State: queueBlocked, Reason: fmt.Sprintf("startup input is unverified; run gang hitch %s --recover", a.Name)}, nil
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return queueRow{State: queueUnknown, Reason: err.Error()}, nil
	}
	b, err := run.input()
	if err != nil {
		return queueRow{State: queueUnknown, Reason: err.Error()}, nil
	}
	v, err := run.inputState(l, a, b, c)
	var refusal commandError
	switch {
	case errors.As(err, &refusal) && refusal.status == exitRefused:
		return queueRow{State: queueBlocked, Reason: err.Error()}, nil
	case err != nil:
		return queueRow{State: queueUnknown, Reason: err.Error()}, nil
	case v.Free:
		return queueRow{State: queueReady}, nil
	}
	return queueRow{State: queueBlocked, Reason: v.Reason}, nil
}

const excerptRunes = 60

// messageExcerpt flattens text to one line and bounds it for display.
func messageExcerpt(text string) string {
	text = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if !unicode.IsPrint(r) {
			return ' '
		}
		return r
	}, text)), " ")
	if runes := []rune(text); len(runes) > excerptRunes {
		return string(runes[:excerptRunes]) + "…"
	}
	return text
}
func (cmd command) interrupt(args []string) (result error) {
	reason := ""
	flags := boundFlagSet("interrupt", map[string]any{"m": &reason, "message": &reason})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("interrupt: %v", err)
	}
	if len(positionals) > 1 {
		return usageError("interrupt: unexpected argument %q", positionals[1])
	}
	name := ""
	if len(positionals) == 1 {
		name = positionals[0]
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
		return inactiveRecipient(a)
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	if err := refusePasteHazard(c, reason); err != nil {
		return err
	}
	b = run.registeredInput(a, b)
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
	b = run.registeredInput(a, b)
	if o.Recover {
		return run.recoverCompaction(l, &a, b, c)
	}
	if a.Status != core.Active {
		return inactiveRecipient(a)
	}
	_, failedStartup, err := retainedStartup(l.Paths, "failed", a.LastFailed)
	if err != nil {
		return err
	}
	if failedStartup {
		return refuseError("startup contract input is unverified; inspect the recipient and run gang hitch %s --recover before compacting", a.Name)
	}
	requester, err := run.observedSender()
	if err != nil {
		if o.Resume != "" {
			return err
		}
		requester = core.Sender{}
	}
	resumeFrom := core.Sender{Kind: core.SenderGangline, Name: "compact"}
	if o.Resume != "" {
		resumeFrom = requester
		if resumeFrom.Kind == "" {
			resumeFrom = core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"}
		}
	}
	if err := refusePasteHazard(c, o.Resume); err != nil {
		return err
	}
	resume := o.Resume
	if resume == "" {
		resume = fmt.Sprintf("Your context was compacted. Re-read your brief and durable state, then resume your work or report it complete. Messages queued for you are recorded in gang log --agent %s --type send_queued.", a.Name)
	}
	id, err := randomID("compact")
	if err != nil {
		return err
	}
	if _, err := envelopeText(core.Envelope{Token: "0123456789abcdef", From: resumeFrom, Message: core.Message{Text: resume}}); err != nil {
		return err
	}
	now := cmd.now()
	compact := core.Compaction{ID: id, Resume: core.Message{Text: resume}, ResumeFrom: resumeFrom, Requester: requester, StartedAt: now, Deadline: now.Add(operationTimeout), Status: "queued"}
	if err := run.apply(l, &a, core.Event{Type: "compaction_requested", Compaction: &compact}); err != nil {
		return err
	}
	run.reported = id
	if err := run.startCompaction(l, &a); err != nil {
		return err
	}
	if a.Compaction.Status == "queued" {
		_, err := fmt.Fprintf(cmd.stdout, "%s\tqueued; waiting for native idle; resume enters native queue when compaction starts\n", id)
		return err
	}
	if a.Compaction.Status == "failed" {
		return commandError{status: exitNative, text: a.Compaction.Reason}
	}
	if a.Compaction.Status != "completed" {
		return commandError{status: exitUnknown, text: "compaction submitted; native completion unconfirmed; resume queued ahead of later input"}
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
	// The resume note would take the receipt slot that holds an unverified
	// startup, so a queued compaction waits until startup recovery clears it.
	if _, failedStartup, err := retainedStartup(l.Paths, "failed", a.LastFailed); err != nil || failedStartup {
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
	b = run.registeredInput(*a, b)
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
	// Pasted input cannot submit itself, so a failure before the compact
	// Enter leaves a compaction that never ran; after it, the compaction may
	// have started.
	entered, compactText := false, ""
	defer func() {
		if result == nil || a.Input == nil {
			return
		}
		if !entered {
			result = run.abortCompactInput(l, a, b, c, compactText, result)
			return
		}
		result = errors.Join(result, run.apply(l, a, core.Event{Type: "compaction_unverified", ID: a.Compaction.ID, Reason: result.Error()}))
	}()
	action, err := harness.RenderAction(c.Actions.Compact, map[string]string{"instructions": a.Compaction.Resume.Text})
	if err != nil {
		return err
	}
	compactText = action.Text
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
		return errors.New("native compaction input blocked: " + blocker.Evidence)
	}
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil {
		return err
	}
	// A harness may show pasted input as something other than the command it
	// was, such as a collapsed paste placeholder. Submitting that would send
	// the command as an ordinary prompt.
	// An empty composer means the command left it without gang's submit key,
	// so Enter here would prove nothing about whether a compaction started.
	if composer.Text == "" {
		return errors.New("the compact command left the composer before its submit key")
	}
	if !harness.SameComposerText(composer.Text, action.Text) {
		// The reason reaches the agent as input, so it must not quote the
		// composer: Claude Code expands a quoted paste placeholder in input
		// back into the paste it names.
		return run.abandonCompactDraft(ctx, l, a, b, c, composer.Text, compactionNotRun, fmt.Sprintf("compaction not submitted: the native composer did not read back as the %d-character compact command, as when the harness collapses a long or multi-line paste", utf8.RuneCountInString(action.Text)))
	}
	if busy, err := harness.Busy(c, screen); err != nil {
		return err
	} else if busy {
		return run.abandonCompactDraft(ctx, l, a, b, c, composer.Text, compactionNotRun, "compaction not submitted: native task became active before compaction submit")
	}
	entered = true
	if err := sendHarnessKeys(ctx, b, pane, c, substrate.Keys{Names: action.Keys, Submit: action.Submit}); err != nil {
		return err
	}
	if err := run.apply(l, a, core.Event{Type: "compaction_submitted", ID: a.Compaction.ID}); err != nil {
		return err
	}
	if err := run.queueCompactionResume(l, a, b, c, action.Text); errors.Is(err, errCompactionNotStarted) {
		return run.failCompaction(l, a, compactionMayHaveRun, err.Error()+"; resume withheld")
	} else if err != nil {
		return run.failCompaction(l, a, compactionMayHaveRun, "resume submission failed; continuation withheld: "+err.Error())
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
