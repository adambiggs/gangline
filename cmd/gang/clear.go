package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

var clearPromptPattern = regexp.MustCompile(`(?m)^\[gang[:#][^]\n]+ clear\]`)

func clearPending(a core.Agent) bool { return a.Clear != nil && a.Clear.Status != "completed" }

func (cmd command) clear(args []string) (result error) {
	var note string
	flags := boundFlagSet("clear", map[string]any{"note": &note})
	positional, err := parseOptions(flags, args)
	if err != nil || len(positional) != 0 || strings.TrimSpace(note) == "" {
		return usageError("clear: expected --note TEXT")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	caller, err := run.callerInTeam()
	if err != nil {
		return err
	}
	if caller == nil {
		return refuseError("clear: run this command from your own hitched harness")
	}
	l, a, err := run.acquireBounded(caller.ID)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, run.unlock(l)) }()
	if err := run.verifyCaller(a); err != nil {
		return err
	}
	if a.Status != core.Active {
		return refuseError("clear: harness must be active")
	}
	if clearPending(a) || a.Rehitch != nil || a.Compaction != nil && a.Compaction.Status != "failed" && !a.Compaction.ResumeAdmitted {
		return refuseError("clear: another context transition is pending; inspect gang status --why")
	}
	if a.Native.SessionID == "" {
		return refuseError("clear: current native session is unknown")
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	if c.Actions.Clear == nil || c.Hooks == nil || c.Hooks.Events["sessionstart"].Event != "session-started" {
		return refuseError("clear: collar %q does not support native clear confirmation", c.Name)
	}
	start, err := l.Paths.ReadSessionStart()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err != nil || start.SessionID != a.Native.SessionID || start.Registration != a.Registration.Generation {
		return refuseError("clear: this running session has not confirmed its SessionStart hook; native clear was not submitted")
	}
	if reason, err := clearReceiptBlocker(l.Paths, a); err != nil {
		return err
	} else if reason != "" {
		return refuseError("clear: %s", reason)
	}
	prose, err := cmd.startupProse(a.Role)
	if err != nil {
		return err
	}
	id, err := randomID("clear")
	if err != nil {
		return err
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	e := core.Envelope{ID: core.EnvelopeID(id), Token: token, Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderAgent, Name: a.Name, HitchID: a.ID}, Purpose: "clear", Message: core.Message{Text: note}, Startup: startupSections(string(a.Name), prose), CreatedAt: cmd.now()}
	wire, err := envelopeText(e)
	if err != nil {
		return err
	}
	if err := refusePasteHazard(c, wire); err != nil {
		return err
	}
	a.Clear = &core.ClearRequest{Envelope: e, PreviousSession: a.Native.SessionID, Status: "queued", Reason: "waiting for the current turn to finish"}
	if err := l.Save(a); err != nil {
		return err
	}
	if err := l.Paths.PublishNew(e); err != nil {
		return err
	}
	if err := run.record(a, core.Event{Type: "clear_requested", ID: id, Reason: "retiring native session " + a.Native.SessionID}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s clear queued; finish this turn; current role and continuation will precede retained mail\n", a.Name)
	return err
}

func clearReceiptBlocker(p store.AgentPaths, a core.Agent) (string, error) {
	if a.Input != nil {
		return "native input is unresolved", nil
	}
	if a.LastAccepted != "" {
		return "native accepted receipt " + string(a.LastAccepted) + " is unconfirmed", nil
	}
	if a.LastFailed != "" {
		e, err := p.ReadEnvelope("failed", a.LastFailed)
		if err != nil {
			return "", err
		}
		if e.Outcome == "unverified" {
			return "native receipt " + string(e.ID) + " is unverified", nil
		}
	}
	return "", nil
}

func (run *runtime) clearState(l *store.LockedAgent, a *core.Agent, status, reason string) error {
	if a.Clear.Status == status && a.Clear.Reason == reason {
		return nil
	}
	a.Clear.Status, a.Clear.Reason = status, reason
	if err := l.Save(*a); err != nil {
		return err
	}
	kind := "clear_waiting"
	if status == "unverified" {
		kind = "clear_unverified"
	}
	if status == "completed" {
		kind = "clear_completed"
	}
	return run.record(*a, core.Event{Type: kind, ID: string(a.Clear.Envelope.ID), Status: status, Reason: reason})
}

// advanceClear runs under the existing input lock, after the caller's turn.
// Submitted native input is never repeated, including after a crashed owner.
func (run *runtime) advanceClear(l *store.LockedAgent, a *core.Agent) error {
	if !clearPending(*a) {
		return nil
	}
	if err := run.reconcileClear(l, a); err != nil {
		return err
	}
	if a.Clear.Status != "queued" {
		return nil
	}
	if reason, err := clearReceiptBlocker(l.Paths, *a); err != nil {
		return err
	} else if reason != "" {
		return run.clearState(l, a, "queued", reason)
	}
	if a.Status != core.Active || a.Activity != core.Idle || a.Native.FinishedAt.IsZero() || a.Native.SubmittedAt.After(a.Native.FinishedAt) {
		return run.clearState(l, a, "queued", "waiting for confirmed native turn completion")
	}
	if a.Rehitch != nil || a.Compaction != nil && a.Compaction.Status != "failed" && !a.Compaction.ResumeAdmitted {
		return run.clearState(l, a, "queued", "another context transition is pending")
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	if c.Actions.Clear == nil {
		return run.clearState(l, a, "queued", "collar no longer declares native clear")
	}
	n, err := l.Paths.ReadSessionStart()
	if err != nil {
		return err
	}
	if n.SessionID != a.Native.SessionID || n.Registration != a.Registration.Generation {
		return run.clearState(l, a, "queued", "current native SessionStart support is unconfirmed")
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	b = run.registeredInput(*a, b)
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	pane := substrate.PaneID(a.Pane)
	screen, err := run.captureRegistered(ctx, b, *a)
	if err != nil {
		return err
	}
	queued, err := harness.NativeInputPending(c, screen, a.Native.Transcript, a.Native.TurnID)
	if err != nil {
		return run.clearState(l, a, "queued", "native queue unread: "+err.Error())
	}
	if queued {
		return run.clearState(l, a, "queued", "native input remains queued")
	}
	idle, err := harness.Idle(c, screen)
	if err != nil {
		return err
	}
	free, _, err := run.available(l, a, b, c)
	if err != nil {
		return err
	}
	if !idle || !free {
		return run.clearState(l, a, "queued", "native composer or prompt is not clear and idle")
	}
	action, err := harness.RenderAction(*c.Actions.Clear, nil)
	if err != nil {
		return err
	}
	input, err := harness.SubmitInput(c.Primitives.Submit, action.Text)
	if err != nil {
		return err
	}
	if err := l.Paths.PublishNew(a.Clear.Envelope); err != nil && !errors.Is(err, store.ErrEnvelopeQueued) {
		return err
	}
	a.Clear.StartedAt = run.cmd.now()
	if err := run.clearState(l, a, "submitted", "native clear input reserved; do not repeat an uncertain command"); err != nil {
		return err
	}
	if err := run.record(*a, core.Event{Type: "clear_submitted", ID: string(a.Clear.Envelope.ID), Reason: "retiring native session " + a.Clear.PreviousSession}); err != nil {
		return err
	}
	fail := func(err error) error { return errors.Join(err, run.clearState(l, a, "unverified", err.Error())) }
	if err := sendHarnessKeys(ctx, b, pane, c, input); err != nil {
		return fail(err)
	}
	settle, err := harness.SubmitSettle(c.Primitives.Submit)
	if err != nil {
		return fail(err)
	}
	if run.cmd.settleInput != nil {
		err = run.cmd.settleInput(ctx, b, pane, c, settle)
	} else {
		err = harness.AwaitComposerSettle(ctx, b.Capture, pane, c, settle)
	}
	if err != nil {
		return fail(err)
	}
	screen, err = b.Capture(ctx, pane)
	if err != nil {
		return fail(err)
	}
	if err := refuseNativePrompt(ctx, b, pane, c); err != nil {
		return fail(err)
	}
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil {
		return fail(err)
	}
	if !harness.SameComposerText(composer.Text, action.Text) {
		return fail(fmt.Errorf("native clear command did not read back exactly; submission withheld"))
	}
	if err := sendHarnessKeys(ctx, b, pane, c, substrate.Keys{Names: action.Keys, Submit: action.Submit}); err != nil {
		return fail(err)
	}
	if err := awaitComposerChange(ctx, b, pane, c, action.Text, compactClearSettle); err != nil {
		return fail(err)
	}
	screen, err = b.Capture(ctx, pane)
	if err != nil {
		return fail(err)
	}
	composer, err = harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil {
		return fail(err)
	}
	if composer.Text != "" {
		return fail(fmt.Errorf("native clear outcome unconfirmed; composer still holds input"))
	}
	if err := refuseNativePrompt(ctx, b, pane, c); err != nil {
		return fail(err)
	}
	// Codex emits SessionStart only when this guarded first prompt submits.
	return run.clearState(l, a, "continuing", "awaiting exact continuation in a new native session")
}

func (run *runtime) admitClearPrompt(p store.AgentPaths, a core.Agent, c harness.Collar, prompt, session string) error {
	if a.Clear == nil || a.Clear.Status != "continuing" {
		return fmt.Errorf("clear continuation is not awaiting admission")
	}
	wire, err := envelopeText(a.Clear.Envelope)
	if err != nil {
		return err
	}
	matched, err := harness.SubmittedPromptMatches(c.Primitives.SubmitWitness, wire, prompt)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("clear continuation differs from the saved role, note or one-time token")
	}
	return p.AdmitClear(a.Clear.Envelope.ID, session, a.Registration.Generation)
}

// The native source=clear witness establishes identity before old transcript
// readers run. Admission and delivery remain separate: only the exact prompt's
// receipt releases retained mail.
func (run *runtime) reconcileClear(l *store.LockedAgent, a *core.Agent) error {
	if !clearPending(*a) || a.Clear.StartedAt.IsZero() {
		return nil
	}
	n, err := l.Paths.ReadSessionStart()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if n.Source != "clear" || n.ClearID != a.Clear.Envelope.ID || n.Registration != a.Registration.Generation || n.SessionID == a.Clear.PreviousSession || n.SessionID == "" || n.At.Before(a.Clear.StartedAt) {
		return nil
	}
	if a.Native.SessionID == a.Clear.PreviousSession {
		sequence, bound := a.Native.HookSequence, a.Native.BoundSession
		a.Native = core.NativeState{SessionID: n.SessionID, Transcript: n.Transcript, BoundSession: bound, HookSequence: sequence, HookAt: n.At}
		a.Compaction, a.Capacity = nil, core.Capacity{}
		a.ContextBands = core.ContextBandState{Sequence: a.ContextBands.Sequence, DiscardQueued: true}
		a.NativeTitle = nil
		a.Activity, a.ScreenFingerprint, a.ScreenSince = core.Idle, "", time.Time{}
		a.Evidence, a.StatusProbeAt = "", time.Time{}
		if err := l.Save(*a); err != nil {
			return err
		}
	}
	if a.ContextBands.DiscardQueued {
		if err := run.publishContextNotes(l, a); err != nil {
			return err
		}
	}
	receipt, receiptErr := l.Paths.ReadEnvelope("cur", a.Clear.Envelope.ID)
	if receiptErr != nil && !errors.Is(receiptErr, os.ErrNotExist) {
		return receiptErr
	}
	if receiptErr == nil && receipt.Outcome == "delivered" {
		return run.clearState(l, a, "completed", "new native session "+a.Native.SessionID+"; exact continuation receipt retained")
	}
	if a.Clear.Status == "submitted" {
		return run.clearState(l, a, "continuing", "native clear confirmed; continuation pending")
	}
	return nil
}

// A cleared conversation cannot be restored by its late observations. Native
// compaction moves still use the collar's existing continuity proof.
func clearedSession(a core.Agent) bool {
	return a.Clear != nil && a.Native.SessionID != ""
}
