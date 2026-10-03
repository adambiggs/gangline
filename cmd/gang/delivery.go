package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

type harnessInput interface {
	Capture(context.Context, substrate.PaneID) (substrate.Screen, error)
	ForegroundCommand(context.Context, substrate.PaneID) (string, error)
	ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error)
	SendKeys(context.Context, substrate.PaneID, substrate.Keys) error
}

func requireHarnessForeground(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar) error {
	_, err := harnessForeground(ctx, b, pane, filepath.Base(c.Launch.Command))
	return err
}

// harnessForeground returns the name tmux reports for the pane's foreground
// process while that process is the harness launched as launch, and refuses
// otherwise. tmux on macOS names a process by its executable file, so a
// harness launched through a symlink (claude -> versions/<version>) is reported
// under the link target's name. The process table then identifies it: the
// leader of the foreground process group, which is the process tmux names,
// carries that name and was invoked as launch.
func harnessForeground(ctx context.Context, b harnessInput, pane substrate.PaneID, launch string) (string, error) {
	command, err := b.ForegroundCommand(ctx, pane)
	if err != nil {
		return "", err
	}
	if filepath.Base(command) == launch {
		return launch, nil
	}
	processes, err := b.ForegroundProcesses(ctx, pane)
	for _, p := range processes {
		if p.PID == p.GroupID && p.Name == command && filepath.Base(p.Command) == launch {
			return command, nil
		}
	}
	refusal := fmt.Errorf("refuse input: harness %q is not in the pane foreground; tmux reports %q", launch, command)
	if err != nil {
		return "", errors.Join(refusal, err)
	}
	return "", refusal
}
func sendHarnessKeys(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar, keys substrate.Keys) error {
	if err := requireHarnessForeground(ctx, b, pane, c); err != nil {
		return err
	}
	return b.SendKeys(ctx, pane, keys)
}

// refusePasteHazard refuses text that the recipient's harness could rewrite
// before submitting it.
func refusePasteHazard(c harness.Collar, text string) error {
	reason, err := harness.PasteHazard(c.Primitives.SubmitWitness, text)
	if err != nil {
		return err
	}
	if reason != "" {
		return refuseError("%s", reason)
	}
	return nil
}

func startupPasteSafe(input substrate.Keys, wire string) bool {
	return !input.Submit && len(input.Names) == 0 && input.Text == "\x1b[200~"+wire+"\x1b[201~"
}

func isContextBandNotice(e core.Envelope) bool {
	return e.From.Kind == core.SenderGangline && e.From.Name == "context-band"
}

// isResumeEnvelope reports a compaction resume, which is delivered only when
// its compaction starts and holds every later message behind it.
func isResumeEnvelope(e core.Envelope) bool {
	return e.Purpose == "resume" || e.From.Name == "compact" && strings.HasPrefix(string(e.ID), "resume-")
}

func senderLabel(s core.Sender) string {
	if s.Kind == core.SenderSelfDeclared {
		return "self-declared:" + string(s.Name)
	}
	return string(s.Name)
}

func envelopeText(e core.Envelope) (string, error) {
	sender := senderLabel(e.From)
	if e.Startup != nil {
		return startupEnvelopeText(e.Startup, sender, e.Token, e.Purpose, e.Message.Text)
	}
	return renderEnvelope(sender, e.Token, e.Purpose, e.Message.Text)
}

func startupEnvelopeText(sections *core.StartupSections, sender, token, purpose, message string) (string, error) {
	if sections.Contract == "" || message == "" || purpose != "assignment" && purpose != "startup" {
		return "", fmt.Errorf("invalid startup sections")
	}
	var parts []string
	for _, section := range []struct{ name, body string }{
		{"contract", sections.Contract},
		{"doctrine", sections.Doctrine},
		{"role", sections.Role},
	} {
		if section.body == "" {
			continue
		}
		part, err := renderEnvelope(section.name, token+"-"+section.name, "startup", section.body)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	if purpose == "startup" {
		sender = "startup"
	}
	part, err := renderEnvelope(sender, token, purpose, message)
	if err != nil {
		return "", err
	}
	parts = append(parts, part)
	wire := strings.Join(parts, "\n\n")
	encoded, err := json.Marshal(wire)
	if err != nil {
		return "", err
	}
	if len(encoded) > maximumMessageBytes {
		return "", refuseError("message exceeds the %d-byte encoded envelope budget; put details in a state file and send its path", maximumMessageBytes)
	}
	return wire, nil
}

// inputVerdict says whether the recipient's composer can take input now and,
// when it cannot, why. Blocker carries a native blocker's evidence only.
type inputVerdict struct {
	Free            bool
	Blocker, Reason string
	// Owner is the label on the free composer's rule, which input typed into
	// it must still carry once the composer has grown.
	Owner string
}

// continuationQueued reports a resume note that the harness holds in its own
// queue after a completed compaction and has not yet submitted. The note's
// submit hook takes the agent lock, so input typed ahead of it would hold that
// lock while waiting for its own hook, which the harness runs only after the
// note's. Without an admission the hold ends after one operation's timeout.
// A harness whose native queue witnesses accepted input needs no such hook.
func continuationQueued(c core.Compaction, now time.Time) bool {
	return c.Status == "completed" && c.Continuation && !c.ResumeAdmitted && now.Before(c.CompletedAt.Add(operationTimeout))
}

// inputState observes the composer even during a running turn. A permission
// prompt or foreign foreground process never qualifies as a free composer.
func (run *runtime) inputState(l *store.LockedAgent, a *core.Agent, b harnessInput, c harness.Collar) (inputVerdict, error) {
	if err := run.checkRecipient(*a); err != nil {
		return inputVerdict{}, err
	}
	switch {
	case a.Status != core.Active:
		return inputVerdict{Reason: "recipient is " + string(a.Status)}, nil
	case a.Activity == core.Interrupting:
		return inputVerdict{Reason: "recipient is being interrupted"}, nil
	case a.Compaction != nil && a.Compaction.Status == "submitted":
		return inputVerdict{Reason: "compaction request awaits the harness"}, nil
	case c.Primitives.QueueWitness == nil && a.Compaction != nil && continuationQueued(*a.Compaction, run.cmd.now()):
		return inputVerdict{Reason: "compaction continuation waits in the harness's queue ahead of this input"}, nil
	}
	screen, err := b.Capture(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		return inputVerdict{}, err
	}
	blocker, blocked, err := harness.InputBlocked(c, screen)
	if err != nil {
		return inputVerdict{}, err
	}
	if blocked {
		if a.Activity != core.Blocked || a.Evidence != blocker.Evidence {
			basis := activityBasis(*a, "blocked", "screen")
			if err := run.apply(l, a, core.Event{Type: "activity_observed", Activity: core.Blocked, Reason: blocker.Evidence, Fingerprint: harness.ScreenFingerprint(screen), Basis: &basis}); err != nil {
				return inputVerdict{}, err
			}
		}
		return inputVerdict{Blocker: blocker.Evidence, Reason: blocker.Evidence}, nil
	}
	// A prompt typed during a compaction waits in the harness's own queue,
	// where no receipt witnesses it until the compaction ends.
	if compacting, err := harness.CompactionActive(c, screen); err != nil {
		return inputVerdict{}, err
	} else if compacting {
		return inputVerdict{Reason: "recipient is compacting; its harness defers queued input"}, nil
	}
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil {
		return inputVerdict{}, err
	}
	if composer.Text != "" {
		return inputVerdict{Reason: "composer holds unsent input"}, nil
	}
	if !c.Primitives.MidTurn {
		idle, err := harness.Idle(c, screen)
		if err != nil {
			return inputVerdict{}, err
		}
		if !idle {
			return inputVerdict{Reason: "recipient is mid-turn and its harness takes input only when idle"}, nil
		}
	}
	if err := requireHarnessForeground(context.Background(), b, substrate.PaneID(a.Pane), c); err != nil {
		return inputVerdict{}, err
	}
	return inputVerdict{Free: true, Owner: composer.Owner}, nil
}

func (run *runtime) available(l *store.LockedAgent, a *core.Agent, b harnessInput, c harness.Collar) (bool, string, error) {
	v, err := run.inputState(l, a, b, c)
	return v.Free, v.Blocker, err
}

// deliver types e into a composer that inputState found free and owned by
// owner.
func (run *runtime) deliver(l *store.LockedAgent, a *core.Agent, e core.Envelope, b harnessInput, c harness.Collar, owner string) (outcome string, withdrawn bool, err error) {
	if err := run.checkRecipient(*a); err != nil {
		return "", false, err
	}
	b = run.registeredInput(*a, b)
	wire, err := envelopeText(e)
	if err != nil {
		return "", false, err
	}
	input, err := harness.SubmitInput(c.Primitives.Submit, wire)
	if err != nil {
		return "", false, err
	}
	var queue func(context.Context) (bool, error)
	// Startup needs its exact contract witness. Context notes likewise keep
	// exact hook proof rather than native queue acceptance.
	if c.Primitives.QueueWitness != nil && c.Primitives.MidTurn && e.Purpose == "" && !isContextBandNotice(e) {
		opener := wire[:strings.Index(wire, "]")+1]
		queue = func(ctx context.Context) (bool, error) {
			if err := requireHarnessForeground(ctx, b, substrate.PaneID(a.Pane), c); err != nil {
				return false, err
			}
			screen, err := b.Capture(ctx, substrate.PaneID(a.Pane))
			if err != nil {
				return false, err
			}
			return harness.NativeQueueAccepted(c, screen, opener)
		}
		// A fresh one-time token must not already appear before this submission.
		seen, err := queue(context.Background())
		if err != nil {
			return "", false, err
		}
		if seen {
			return "", false, fmt.Errorf("message already appears in native queue before input; inspect the recipient; do not resend")
		}
	}
	old, err := l.Paths.ReadWitness()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	if err := run.apply(l, a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
		return "", false, err
	}
	if reason, err := harness.PasteHazard(c.Primitives.SubmitWitness, wire); err != nil || reason != "" {
		if err != nil {
			reason = err.Error()
		}
		if err := run.finishInput(l, a, e, "failed", reason); err != nil {
			return "failed", false, err
		}
		return "failed", false, run.mark(*a)
	}
	if isStartupEnvelope(e) && startupPasteSafe(input, wire) {
		e.PasteOnly = &core.StartupPaste{WitnessID: old.ID}
		if err := l.Paths.Publish(e); err != nil {
			return "", false, err
		}
	}
	outcome, reason := "delivered", ""
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	pane := substrate.PaneID(a.Pane)
	if err = sendHarnessKeys(ctx, b, pane, c, input); err == nil {
		settle, settleErr := harness.SubmitSettle(c.Primitives.Submit)
		if settleErr != nil {
			err = settleErr
		} else if run.cmd.settleInput != nil {
			err = run.cmd.settleInput(ctx, b, pane, c, settle)
		} else {
			err = harness.AwaitOwnedComposerSettle(ctx, b.Capture, pane, c, owner, settle)
		}
	}
	if err == nil {
		screen, captureErr := b.Capture(ctx, pane)
		err = captureErr
		if err == nil {
			var blocked bool
			var blocker harness.Blocked
			blocker, blocked, err = harness.InputBlocked(c, screen)
			if blocked && err == nil {
				err = fmt.Errorf("native prompt needs attention after paste: %s", blocker.Evidence)
			}
		}
	}
	if err == nil {
		err = startupSubmitPossible(l.Paths, &e)
	}
	submitted := false
	if err == nil {
		submitted = true
		err = sendHarnessKeys(ctx, b, pane, c, substrate.Keys{Submit: true})
	}
	var witness store.Witness
	var accepted, held bool
	if err == nil {
		if isContextBandNotice(e) {
			// Context notes use exact hook proof. Leave a queued note for a
			// later hook instead of holding the agent lock waiting for its turn.
			witness, err = l.Paths.ReadWitness()
			if errors.Is(err, os.ErrNotExist) || err == nil && witness.ID == old.ID {
				err = fmt.Errorf("context-band submit hook is not yet available")
			}
		} else {
			witness, accepted, err = run.cmd.awaitReceipt(ctx, l.Paths, old.ID, queue)
		}
	}
	if err == nil && !accepted {
		var matched bool
		matched, err = harness.SubmittedPromptMatches(c.Primitives.SubmitWitness, wire, witness.Prompt)
		if err == nil && !matched {
			if queue != nil && (a.Native.SessionID == "" || witness.SessionID == a.Native.SessionID) {
				accepted, err = queue(ctx)
			}
			if err == nil && !accepted {
				err = fmt.Errorf("submit witness does not match the message text and one-time token")
			}
		}
	}
	if err != nil {
		outcome, reason = "unverified", err.Error()
		// Startup recovery resubmits a startup paste the composer still holds.
		// A usage wake reads a failed receipt as never submitted yet cannot
		// republish under the same ID, so Gangline's own input stays
		// unverified. A composer emptied by something else may hold input
		// that is not the paste.
		if !submitted && withdrawable(e) && !errors.Is(err, harness.ErrComposerEmptied) {
			outcome, reason, held = run.withdrawPaste(*a, b, c, reason)
			withdrawn = outcome == "failed"
		}
	} else if accepted {
		outcome, reason = "accepted", "native queue shows sender and one-time token; do not resend"
	} else {
		if a.Native.SessionID != "" && a.Native.SessionID != witness.SessionID {
			outcome, reason = "unverified", "submit witness belongs to another native session"
		} else {
			a.Native.SessionID = witness.SessionID
			a.Native.TurnID = witness.TurnID
			a.Native.Transcript = witness.Transcript
		}
	}
	if err := run.finishInput(l, a, e, outcome, reason, witness); err != nil {
		return outcome, withdrawn, err
	}
	if outcome == "unverified" {
		if err := run.reconcileDelivery(l, a); err != nil {
			return outcome, withdrawn, err
		}
		if a.LastDelivered == e.ID {
			outcome = "delivered"
		}
	}
	switch {
	case withdrawn:
		// A sender that is not an agent gets no notice of its own, and a
		// composer that keeps refusing pastes withdraws one message per drain.
		text := fmt.Sprintf("A message from %s to %s, which you hitched, was withdrawn from its composer and not delivered: %s.", e.From.Name, a.Name, reason)
		if err := run.notifyHitcher(*a, core.EnvelopeID("withdrawn-"+e.ID), text); err != nil {
			return outcome, withdrawn, err
		}
	case held && outcome == "unverified":
		// Later messages wait behind the paste, and neither the recipient nor
		// the sender can clear it.
		text := fmt.Sprintf("A message to %s, which you hitched, is unverified: %s. Messages to %s wait while its composer holds input; gang capture --composer %s shows it.", a.Name, reason, a.Name, a.Name)
		if err := run.notifyHitcher(*a, core.EnvelopeID("held-input-"+e.ID), text); err != nil {
			return outcome, withdrawn, err
		}
	}
	if err := run.mark(*a); err != nil {
		return outcome, withdrawn, err
	}
	return outcome, withdrawn, nil
}

// withdrawable reports whether a paste of e abandoned before its submit key
// is withdrawn: a message from an agent or the operator, not Gangline's own
// startup, resume, or notice input.
func withdrawable(e core.Envelope) bool {
	return e.From.Kind != core.SenderGangline && !isStartupEnvelope(e) && !isResumeEnvelope(e)
}

// withdrawPaste clears a message paste abandoned before its submit key, so the
// recipient is not left holding it in its composer, and returns the delivery's
// outcome. A bracketed paste cannot submit itself, so once the composer reads
// empty the message never reached the harness and the delivery fails. The
// composer was empty before the paste, so what it holds now is the paste. It
// works under a fresh deadline because the cause may be the operation's own.
// Behind a native prompt the clear keys would answer the prompt, and on an
// unreadable composer they could not be confirmed, so the paste stays, the
// delivery remains unverified, and held reports that the composer may still
// hold it.
func (run *runtime) withdrawPaste(a core.Agent, b harnessInput, c harness.Collar, reason string) (outcome, why string, held bool) {
	const remains = "; the pasted input may remain in the composer"
	if !harness.BracketedPaste(c.Primitives.Submit) {
		return "unverified", reason, false
	}
	ctx, cancel := run.cmd.timeout(compactAbortTimeout)
	defer cancel()
	pane := substrate.PaneID(a.Pane)
	screen, err := b.Capture(ctx, pane)
	if err != nil {
		return "unverified", reason + remains + "; composer unread: " + err.Error(), true
	}
	if _, blocked, err := harness.InputBlocked(c, screen); err != nil {
		return "unverified", reason + remains + "; composer unread: " + err.Error(), true
	} else if blocked {
		return "unverified", reason + remains + " behind a native prompt", true
	}
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil {
		return "unverified", reason + remains + "; composer unread: " + err.Error(), true
	}
	if composer.Text == "" {
		return "unverified", reason, false
	}
	if c.Actions.CompactClear == nil {
		return "unverified", reason + "; the pasted input remains in the composer and the collar declares no clear keys", true
	}
	if err := run.clearComposerDraft(ctx, b, pane, c, composer.Text); err != nil {
		return "unverified", reason + remains + "; " + err.Error(), true
	}
	return "failed", reason + "; the pasted input was withdrawn from the composer", false
}

// drainLocked returns the queue it could not deliver. The owner uses those
// identities to distinguish existing blocked work from arrivals during unlock.
func (run *runtime) drainLocked(l *store.LockedAgent, a *core.Agent, target core.EnvelopeID) (string, []core.Envelope, error) {
	result := "queued"
	if err := run.checkDeadlines(l, a); err != nil {
		return result, nil, err
	}
	if err := run.continueCompaction(l, a); err != nil {
		return result, nil, err
	}
	if a.Status != core.Active {
		pending, err := l.Paths.ListNew()
		return result, pending, err
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return result, nil, err
	}
	b, err := run.input()
	if err != nil {
		return result, nil, err
	}
	for {
		pending, err := l.Paths.ListNew()
		if err != nil {
			return result, nil, err
		}
		var next *core.Envelope
		for i := 0; i < len(pending); i++ {
			if e := pending[i]; !e.NotAfter.IsZero() && !e.NotAfter.After(run.cmd.now()) {
				const reason = "expired before delivery"
				if err := l.Withdraw(e.ID); err != nil {
					return result, pending, err
				}
				if err := run.record(*a, core.Event{Type: "send_cancelled", ID: string(e.ID), Reason: reason}); err != nil {
					return result, pending, err
				}
				continue
			}
			if !pending[i].NotBefore.After(run.cmd.now()) {
				next = &pending[i]
				break
			}
		}
		if next == nil {
			return result, pending, nil
		}
		if isResumeEnvelope(*next) {
			return result, pending, nil
		}
		_, failedStartup, err := retainedStartup(l.Paths, "failed", a.LastFailed)
		if err != nil {
			return result, pending, err
		}
		if failedStartup {
			return result, pending, nil
		}
		v, err := run.inputState(l, a, b, c)
		if err != nil {
			return result, pending, err
		}
		if !v.Free {
			if v.Blocker != "" && target != "" && run.cmd.stderr != nil {
				if _, err := fmt.Fprintf(run.cmd.stderr, "%s input blocked: %s; message remains queued\n", a.Name, v.Blocker); err != nil {
					return result, pending, err
				}
			}
			return result, pending, nil
		}
		outcome, withdrawn, err := run.deliver(l, a, *next, b, c, v.Owner)
		if next.ID == target {
			result = outcome
		}
		if err != nil {
			return result, pending, err
		}
		if next.ID != target && (outcome == "failed" || outcome == "unverified") {
			if err := run.notifySender(*a, *next, outcome); err != nil {
				return result, pending, err
			}
		}
		if withdrawn {
			// What failed this paste may fail the next one too, so a later
			// drain retries the rest of the queue rather than withdrawing each
			// message in turn.
			pending, err := l.Paths.ListNew()
			return result, pending, err
		}
	}
}

// notifySender tells the agent that sent e that it was not delivered, or that
// a message it was told was unverified has since been delivered. A sending
// command reports its own message's outcome, so only an outcome reached by a
// later command needs the notice. Gangline sends the notice
// itself, and only agents receive one, so a notice never produces another.
// The notice rides on a drain, drop, or recovery that has already settled e,
// so a notice that cannot be sent is logged and warned about rather than
// failing that operation.
func (run *runtime) notifySender(a core.Agent, e core.Envelope, outcome string) error {
	if e.From.Kind != core.SenderAgent || e.From.HitchID == a.ID {
		return nil
	}
	err := run.sendNotice(a, e, outcome)
	if err == nil {
		return nil
	}
	if err := run.record(a, core.Event{Type: "notice_failed", ID: string(e.ID), Reason: err.Error()}); err != nil {
		return err
	}
	if run.cmd.stderr != nil {
		if _, err := fmt.Fprintf(run.cmd.stderr, "warning: could not tell %s about message %s: %v\n", e.From.Name, e.ID, err); err != nil {
			return err
		}
	}
	return nil
}
func (run *runtime) sendNotice(a core.Agent, e core.Envelope, outcome string) error {
	p, err := run.team.Agent(e.From.HitchID)
	if err != nil {
		return err
	}
	sender, err := p.Read()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if sender.Status != core.Active {
		return nil
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	text := fmt.Sprintf("Message %s to %s ", e.ID, a.Name)
	id := core.EnvelopeID("outcome-" + e.ID)
	switch outcome {
	case "delivered":
		// A notice is published once per ID, so the unverified notice already
		// holds "outcome-" for this message.
		id = core.EnvelopeID("delivered-" + e.ID)
		text += "is delivered: Gangline confirmed delivery after recording it unverified; do not send it again."
	case "unverified":
		text += fmt.Sprintf("is unverified: Gangline could not confirm delivery and may still confirm it later. Inspect %s before sending again; gang log has the reason.", a.Name)
	case "dropped":
		text += fmt.Sprintf("was not delivered: %s was dropped.", a.Name)
	default:
		text += "failed and was not delivered; gang log has the reason."
	}
	if err := run.publishOnceTo(p, sender, core.Envelope{ID: id, Token: token, Recipient: sender.ID, To: sender.Name, From: core.Sender{Kind: core.SenderGangline, Name: "delivery"}, Message: core.Message{Text: text}, CreatedAt: run.cmd.now()}); err != nil {
		return err
	}
	run.wake = append(run.wake, sender.ID)
	return nil
}
func (run *runtime) drain(id core.HitchID, target core.EnvelopeID) (string, error) {
	l, a, err := run.acquire(id, false)
	if errors.Is(err, store.ErrLocked) {
		return "queued", nil
	}
	if err != nil {
		return "queued", err
	}
	return run.drainFrom(l, a, target)
}
func (run *runtime) drainFrom(l *store.LockedAgent, a core.Agent, target core.EnvelopeID) (string, error) {
	result := "queued"
	for {
		got, waiting, err := run.drainLocked(l, &a, target)
		if got != "queued" {
			result = got
		}
		closeErr := run.unlock(l)
		if err != nil {
			return result, err
		}
		if closeErr != nil {
			return result, closeErr
		}
		if run.cmd.afterUnlock != nil {
			run.cmd.afterUnlock()
		}
		pending, err := l.Paths.ListNew()
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		if err != nil {
			return result, err
		}
		seen := make(map[core.EnvelopeID]bool, len(waiting))
		for _, e := range waiting {
			seen[e.ID] = true
		}
		arrived := false
		for _, e := range pending {
			if !seen[e.ID] && !e.NotBefore.After(run.cmd.now()) {
				arrived = true
				break
			}
		}
		if !arrived {
			return result, nil
		}
		l, a, err = run.acquire(a.ID, false)
		if errors.Is(err, store.ErrLocked) || errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		if err != nil {
			return result, err
		}
	}
}
func deliveryResult(outcome string) error {
	if outcome == "unverified" {
		return commandError{status: exitUnknown, text: "message input is unverified; inspect the recipient before sending again"}
	}
	return nil
}
