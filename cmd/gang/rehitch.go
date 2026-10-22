package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func (cmd command) rehitch(args []string) (result error) {
	var resume string
	var upgrade bool
	flags := boundFlagSet("rehitch", map[string]any{"resume": &resume, "upgrade": &upgrade})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("rehitch: %v", err)
	}
	if len(positionals) != 0 || strings.TrimSpace(resume) == "" {
		return usageError("rehitch: expected --resume TEXT")
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
		return refuseError("rehitch: run this command from your own hitched harness")
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
		return refuseError("rehitch: harness must be active; inspect gang status %s --why", a.Name)
	}
	if a.Rehitch != nil {
		return refuseError("rehitch: a restart is already queued for %s", a.Name)
	}
	if a.Native.SessionID == "" {
		return refuseError("rehitch: native resume identity is unknown; inspect gang status %s --why", a.Name)
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	if len(c.Launch.ResumeArgs) == 0 {
		return refuseError("rehitch: collar %q declares no native resume arguments", c.Name)
	}
	if _, err := harness.ValidateResume(c, a.Native.SessionID); err != nil {
		return refuseError("rehitch: %s", err)
	}
	if _, err := exec.LookPath(c.Launch.Command); err != nil {
		return refuseError("rehitch: find harness: %s", err)
	}
	if err := refusePasteHazard(c, resume); err != nil {
		return err
	}
	if upgrade {
		if len(c.Launch.UpgradeArgs) == 0 {
			return refuseError("rehitch: collar %q declares no harness update command; update it yourself, then run gang rehitch without --upgrade", c.Name)
		}
		update := exec.Command(c.Launch.Command, c.Launch.UpgradeArgs...)
		update.Dir = a.Directory
		update.Stdout, update.Stderr = childOutput(cmd.stdout), childOutput(cmd.stderr)
		update.Env = update.Environ()
		for key, value := range c.Launch.Env {
			update.Env = append(update.Env, key+"="+value)
		}
		if err := update.Run(); err != nil {
			return fmt.Errorf("rehitch: harness update failed; restart was not queued: %w", err)
		}
	}
	a.Rehitch = &core.Message{Text: resume}
	if err := l.Save(a); err != nil {
		return err
	}
	if err := run.record(a, core.Event{Type: "rehitch_requested"}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s restart queued; finish this turn to resume with the new harness\n", a.Name)
	return err
}

func rehitchReady(a core.Agent) bool {
	return a.Rehitch != nil && a.Status == core.Active && a.Activity == core.Idle &&
		a.Input == nil && !a.Native.FinishedAt.IsZero() && !a.Native.SubmittedAt.After(a.Native.FinishedAt) &&
		(a.Compaction == nil || a.Compaction.Status == "failed" || a.Compaction.ResumeAdmitted)
}

// A terminal hook or the existing watchdog owns the restart after the caller's
// turn finishes, outside the native process tree that teardown stops.
func (run *runtime) rehitchPending(id core.HitchID) error {
	p, err := run.team.Agent(id)
	if err != nil {
		return err
	}
	a, err := p.Read()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !rehitchReady(a) {
		return err
	}
	args := []string{string(a.Name), "-c", a.Collar, "-d", a.Directory, "--resume", a.Native.SessionID, "-t", a.Rehitch.Text}
	if a.Role != "" {
		args = append(args, "-r", a.Role)
	}
	err = run.cmd.hitchReplacing(args, false, &a)
	if errors.Is(err, errRehitchDeferred) {
		return nil
	}
	if err == nil {
		return nil
	}
	return run.rehitchError(a, err)
}

func (run *runtime) rehitchError(previous core.Agent, err error) error {
	id := previous.ID
	// Preflight can fail while the original harness still runs. Consume the
	// request once and tell its owner; a watchdog must not repeat teardown.
	l, current, lockErr := run.acquire(id, true)
	if lockErr != nil {
		return errors.Join(err, lockErr)
	}
	defer run.release(l)
	current.Rehitch = nil
	if saveErr := l.Save(current); saveErr != nil {
		return errors.Join(err, saveErr)
	}
	kind := "rehitch_failed"
	message := fmt.Sprintf("%s could not restart its harness: %s. Inspect gang status %s --why and the team log.", current.Name, err, current.Name)
	if current.Pane != "" && current.Status != core.Failed && current.Registration.Generation != previous.Registration.Generation {
		kind = "rehitch_attention"
		message = fmt.Sprintf("%s restarted its harness but needs attention: %s. Inspect gang status %s --why and the team log.", current.Name, err, current.Name)
	}
	failureID, idErr := randomID("rehitch-failed")
	if idErr != nil {
		return errors.Join(err, idErr)
	}
	token, tokenErr := randomEnvelopeToken()
	if tokenErr != nil {
		return errors.Join(err, tokenErr)
	}
	envelope := core.Envelope{ID: core.EnvelopeID(failureID), Token: token, Recipient: id, To: current.Name, From: core.Sender{Kind: core.SenderGangline, Name: "rehitch"}, Message: core.Message{Text: message}, CreatedAt: run.cmd.now()}
	return errors.Join(err, run.record(current, core.Event{Type: kind, Reason: err.Error()}),
		l.Paths.Publish(envelope), run.notifyHitcher(current, envelope.ID, message))
}

var errRehitchDeferred = errors.New("rehitch deferred until native idle")

func (run *runtime) prepareRehitch(previous core.Agent, next *core.Agent) (*store.LockedAgent, error) {
	p, err := run.team.Agent(previous.ID)
	if err != nil {
		return nil, err
	}
	l, err := p.TryLock()
	if errors.Is(err, store.ErrLocked) {
		return nil, errRehitchDeferred
	}
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*store.LockedAgent, error) { return nil, errors.Join(err, run.unlock(l)) }
	a, err := p.Read()
	if err != nil {
		return fail(err)
	}
	if !rehitchReady(a) || a.Rehitch.Text != previous.Rehitch.Text || a.Pane != previous.Pane || a.Name != previous.Name || a.Native.SessionID != previous.Native.SessionID {
		return fail(errRehitchDeferred)
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return fail(err)
	}
	input, err := run.input()
	if err != nil {
		return fail(err)
	}
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	screen, err := run.captureRegistered(ctx, input, a)
	if err != nil {
		return fail(err)
	}
	if _, blocked, err := harness.InputBlocked(c, screen); err != nil {
		return fail(err)
	} else if blocked {
		return fail(errRehitchDeferred)
	}
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil {
		return fail(err)
	}
	idle, err := harness.Idle(c, screen)
	if err != nil {
		return fail(err)
	}
	if !idle || composer.Text != "" {
		return fail(errRehitchDeferred)
	}
	// Persist consumption before stopping native work. Failed launch leaves a
	// retained failed registration, rather than another scheduled restart.
	a.Rehitch = nil
	if err := l.Save(a); err != nil {
		return fail(err)
	}
	if stopping, err := run.endNative(l, &a); err != nil {
		if stopping {
			err = errors.Join(err, run.apply(l, &a, core.Event{Type: "hitch_failed", Reason: err.Error()}))
		}
		return fail(err)
	}
	next.Native = a.Native
	next.Native.HookAt, next.Native.HookKind = run.cmd.now(), ""
	// Preserve telemetry's cursor and acknowledged hook sequence across the
	// same native conversation, while requiring a new startup submit witness.
	for _, path := range []string{p.Witness, p.CompactionWitness, p.Background, p.Permission} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fail(err)
		}
	}
	if err := l.Save(*next); err != nil {
		return fail(err)
	}
	if err := run.record(*next, core.Event{Type: "rehitch_started"}); err != nil {
		return fail(err)
	}
	return l, nil
}
