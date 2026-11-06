package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// hookNotice contains bounded routing and measurement evidence, never a prompt or transcript.
// The detached tick validates it against the agent's current native session.
type hookNotice struct {
	Readings    []core.Reading `json:"readings,omitempty"`
	Kind        string         `json:"kind"`
	NativeEvent string         `json:"native_event"`
	At          time.Time      `json:"at"`
	SessionID   string         `json:"session_id,omitempty"`
	TurnID      string         `json:"turn_id,omitempty"`
	Transcript  string         `json:"transcript,omitempty"`
	Failure     string         `json:"failure,omitempty"`
}

func (cmd command) hook(args []string) error {
	if fifo := cmd.environment(collarCheckFIFOEnvironment); fifo != "" {
		if err := cmd.collarCheckHook(args, fifo); err != nil && cmd.stderr != nil {
			_, _ = fmt.Fprintf(cmd.stderr, "gang hook: %s\n", cmd.operatorText(err.Error()))
		}
		return nil
	}
	err := cmd.handleHook(args)
	if err != nil {
		if cmd.stderr != nil {
			_, _ = fmt.Fprintf(cmd.stderr, "gang hook: %s\n", cmd.operatorText(err.Error()))
		}
		if run, setupErr := cmd.runtime(); setupErr == nil {
			_ = run.team.Append(core.Event{Type: "hook_failed", At: cmd.now(), HitchID: core.HitchID(cmd.environment("GANGLINE_HITCH_ID")), Pane: cmd.environment("TMUX_PANE"), Reason: err.Error()})
		}
	}
	return nil
}
func (cmd command) handleHook(args []string) (result error) {
	if err := noArguments(args, "hook"); err != nil {
		return err
	}
	payload, err := io.ReadAll(io.LimitReader(cmd.stdin, maximumHookBytes+1))
	if err != nil {
		return err
	}
	if len(payload) > maximumHookBytes {
		if err := json.NewEncoder(cmd.stdout).Encode(map[string]string{"decision": "block", "reason": "hook payload exceeds maximum size"}); err != nil {
			return err
		}
		return fmt.Errorf("hook payload exceeds maximum size")
	}
	var submitted struct {
		Event  string `json:"hook_event_name"`
		Prompt string `json:"prompt"`
	}
	blockedWritten := false
	resumeAdmitted := false
	clearAdmitted := false
	resumeCandidate := json.Unmarshal(payload, &submitted) == nil && submitted.Event == "UserPromptSubmit" && resumePromptPattern.MatchString(submitted.Prompt)
	clearCandidate := submitted.Event == "UserPromptSubmit" && clearPromptPattern.MatchString(submitted.Prompt)
	if resumeCandidate || clearCandidate {
		defer func() {
			if result != nil && !blockedWritten && !resumeAdmitted && !clearAdmitted {
				reason := "compaction continuation could not be verified"
				if clearCandidate {
					reason = "clear continuation could not be verified"
				}
				blockErr := json.NewEncoder(cmd.stdout).Encode(map[string]string{"decision": "block", "reason": reason})
				result = errors.Join(result, blockErr)
			}
		}()
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	id := core.HitchID(cmd.environment("GANGLINE_HITCH_ID"))
	p, err := run.team.Agent(id)
	if err != nil {
		return err
	}
	a, err := p.Read()
	if err != nil {
		return err
	}
	observedSession := a.Native.SessionID
	token := cmd.environment("GANG_AGENT_NONCE")
	if token == "" || tokenHash(token) != a.Registration.TokenHash {
		return refuseError("hitch token does not match the registered pane")
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	_, event, err := harness.DetectTurnBoundary(c, payload)
	if err != nil {
		return err
	}
	if event.Kind == "session-started" {
		if err := p.WriteSessionStart(store.SessionStart{At: cmd.now(), SessionID: event.Payload["session_id"], Transcript: event.Payload["transcript_path"], Source: event.Payload["source"], Registration: a.Registration.Generation}); err != nil {
			return err
		}
		return run.record(a, core.Event{Type: "native_hook", NativeEvent: event.NativeEvent, Status: event.Kind})
	}
	if event.Kind == "turn-started" && (clearPending(a) || clearPromptPattern.MatchString(event.Payload["prompt"])) {
		if admissionErr := run.admitClearPrompt(p, a, c, event.Payload["prompt"], event.Payload["session_id"]); admissionErr != nil {
			blockErr := json.NewEncoder(cmd.stdout).Encode(map[string]string{"decision": "block", "reason": admissionErr.Error()})
			blockedWritten = blockErr == nil
			return errors.Join(blockErr, run.record(a, core.Event{Type: "native_hook", NativeEvent: event.NativeEvent, Status: "clear-blocked", Reason: admissionErr.Error()}))
		}
		clearAdmitted = true
	}
	if clearedSession(a) && !clearAdmitted && event.Payload["session_id"] != "" && event.Payload["session_id"] != a.Native.SessionID {
		same, moveErr := followNativeSession(c, &a, event.Payload["session_id"], event.Payload["transcript_path"])
		if moveErr != nil || !same {
			return run.record(a, core.Event{Type: "native_hook", NativeEvent: event.NativeEvent, Status: "retired-session", Reason: "hook does not belong to the current native conversation"})
		}
	}
	if reason := foreignSubmit(a, event); reason != "" {
		if event.Kind == "turn-started" {
			return run.recordForeignSubmit(a, event, payload, reason)
		}
		return run.record(a, core.Event{Type: "native_hook", NativeEvent: event.NativeEvent, Status: "foreign-session", Reason: reason})
	}
	if (resumeCandidate || clearCandidate) && event.Kind != "turn-started" {
		return fmt.Errorf("continuation did not map to a submit boundary")
	}
	// Display state never gates the hook's own handling: a failure to keep it
	// is reported once the hook has done its work.
	displayErr := p.WriteDisplayHook(store.StatusHook{Kind: event.Kind, At: cmd.now(), SessionID: event.Payload["session_id"]}, observedSession)
	defer func() { result = errors.Join(result, displayErr) }()
	if event.Kind == "turn-started" {
		for _, value := range []string{event.Payload["session_id"], event.Payload["turn_id"], event.Payload["transcript_path"], event.NativeEvent} {
			if len(value) > 4096 {
				return fmt.Errorf("hook routing metadata exceeds maximum size")
			}
		}
		reason := ""
		if resumeCandidate {
			reason, err = run.admitCompactionResume(id, c, event.Payload["prompt"], event.Payload["session_id"])
			if err != nil {
				return err
			}
			resumeAdmitted = reason == ""
		}
		if reason != "" {
			if err := json.NewEncoder(cmd.stdout).Encode(map[string]string{"decision": "block", "reason": reason}); err != nil {
				return err
			}
			blockedWritten = true
			if err := run.record(a, core.Event{Type: "native_hook", NativeEvent: event.NativeEvent, Status: "resume-blocked", Reason: reason}); err != nil {
				return err
			}
			// Input held behind a blocked note need not wait for the watchdog.
			return run.detachTick(id)
		}
	}
	receipt, err := randomID("hook")
	if err != nil {
		return err
	}
	failure := ""
	if event.Kind == "turn-failed" {
		failure = strings.TrimSpace(event.Payload["error"])
		if detail := strings.TrimSpace(event.Payload["error_details"]); detail != "" {
			if failure != "" {
				failure += ": "
			}
			failure += detail
		}
		if failure == "" {
			failure = "native turn failed"
		}
		failure = boundedFailureReason(failure)
	}
	if err := run.record(a, core.Event{Type: "native_hook", ID: receipt, NativeEvent: event.NativeEvent, Status: event.Kind, Reason: failure}); err != nil {
		return err
	}
	switch event.Kind {
	case "turn-started":
		w := store.Witness{ID: receipt, At: cmd.now(), Prompt: event.Payload["prompt"], SessionID: event.Payload["session_id"], TurnID: event.Payload["turn_id"], Transcript: event.Payload["transcript_path"]}
		if err := p.WriteNativeWitness(w, observedSession); err != nil {
			return err
		}
		// Always reconcile: a concurrent failure tick may have read the old
		// witness before this write and save its result after our state read.
		n := hookNotice{Kind: "turn-started", NativeEvent: event.NativeEvent, At: cmd.now(), SessionID: event.Payload["session_id"], TurnID: event.Payload["turn_id"], Transcript: event.Payload["transcript_path"]}
		if err := p.WriteStatusHook(statusHook(n)); err != nil {
			return err
		}
		if cmd.detach != nil {
			return cmd.detach(string(id), n)
		}
		return cmd.detachTick(string(id), n, run.settings)
	case "turn-finished", "turn-failed", "compaction-finished", "activity", "permission-requested", "compaction-started":
		n := hookNotice{Kind: event.Kind, NativeEvent: event.NativeEvent, At: cmd.now(), SessionID: event.Payload["session_id"], TurnID: event.Payload["turn_id"], Transcript: event.Payload["transcript_path"], Failure: failure}
		for _, value := range []string{n.SessionID, n.TurnID, n.Transcript, n.NativeEvent} {
			if len(value) > 4096 {
				return fmt.Errorf("hook routing metadata exceeds maximum size")
			}
		}
		if n.Kind == "compaction-finished" {
			if err := run.confirmCompactionHook(id, n); err != nil {
				return err
			}
		}
		if err := p.WriteStatusHook(statusHook(n)); err != nil {
			return err
		}
		if cmd.detach != nil {
			return cmd.detach(string(id), n)
		}
		return cmd.detachTick(string(id), n, run.settings)
	}
	return nil
}

func boundedFailureReason(reason string) string {
	const limit = 4096
	if len(reason) <= limit {
		return reason
	}
	const suffix = " [truncated]"
	end := limit - len(suffix)
	for end > 0 && !utf8.RuneStart(reason[end]) {
		end--
	}
	return reason[:end] + suffix
}
func (cmd command) detachTick(id string, n hookNotice, s settings) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	child := exec.Command(exe, "tick", "--agent", id)
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	child.Env = append(os.Environ(), "GANGLINE_BOUNDARY="+string(data), "GANG_SESSION="+s.Session, "GANG_STATE_ROOT="+s.StateRoot, "GANG_CONFIG_DIR="+s.ConfigDir)
	if s.Socket != "" {
		child.Env = append(child.Env, "GANG_TMUX_SOCKET="+s.Socket)
	}
	if s.CollarDir != "" {
		child.Env = append(child.Env, "GANG_COLLARS="+s.CollarDir)
	}
	// Nil standard streams connect to the null device, not the harness pipes;
	// the tick records its own failures in the team log.
	if err := child.Start(); err != nil {
		return err
	}
	return child.Process.Release()
}
func (cmd command) tick(args []string) (result error) {
	id, source, generation := "", "", ""
	flags := boundFlagSet("tick", map[string]any{
		"agent": &id, "source": &source, "watchdog": &generation,
	})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("tick: %v", err)
	}
	if len(positionals) != 0 {
		return usageError("tick: unexpected argument %q", positionals[0])
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	if id != "" {
		resolved, err := run.tickTarget(id)
		if err != nil {
			return err
		}
		id = string(resolved)
	}
	var notice hookNotice
	if id != "" && cmd.environment("GANGLINE_BOUNDARY") != "" {
		if err := json.Unmarshal([]byte(cmd.environment("GANGLINE_BOUNDARY")), &notice); err != nil {
			return run.tickFailed("hook", core.HitchID(id), fmt.Errorf("read hook notice: %w", err))
		}
	}
	if source == "" {
		source = "command"
		if notice.Kind != "" {
			source = "hook"
		}
	} else if source != "watchdog" || generation == "" || id != "" {
		return usageError("tick: explicit source requires --source watchdog --watchdog UNIT")
	}
	if generation != "" && source != "watchdog" {
		return usageError("tick: --watchdog requires --source watchdog")
	}
	proceed, err := run.updateWatchdog(generation, false, id == "")
	if err != nil {
		schedulerErr := err
		defer func() { result = errors.Join(result, schedulerErr) }()
	}
	if err == nil && !proceed {
		return nil
	}
	if err := run.team.Append(core.Event{Type: "tick", At: cmd.now(), Source: source}); err != nil {
		return run.tickFailed(source, core.HitchID(id), err)
	}
	if id != "" {
		// Frequent display hooks can skip occupied agents: their screen or
		// permission witness remains available to the independent watchdog.
		// Native boundaries still wait so their turn outcomes are reconciled.
		wait := notice.Kind != "activity" && notice.Kind != "permission-requested" && notice.Kind != "compaction-started"
		return errors.Join(run.tickFailed(source, core.HitchID(id), run.tickAgent(core.HitchID(id), notice, wait)), run.tickFailed(source, "", run.flushUsageWork()))
	}
	agents, err := run.team.ListAgents()
	if err != nil {
		return run.tickFailed(source, "", err)
	}
	run.idleSweep, err = run.startIdleSweep(agents)
	if err != nil {
		return run.tickFailed(source, "", err)
	}
	sweepErr := eachAgent(agents, func(a core.Agent) error {
		// Deferred notices belong to this worker's lock lifetime. Parallel
		// workers must not drain or mutate one another's wake queues.
		agentRun := *run
		agentRun.wake = nil
		return agentRun.tickFailed(source, a.ID, agentRun.tickAgent(a.ID, hookNotice{}, false))
	})
	usageErr := run.tickFailed(source, "", run.flushUsageWork())
	if sweepErr != nil || usageErr != nil {
		return errors.Join(sweepErr, usageErr)
	}
	if err := run.reserveIdleNotice(run.idleSweep); err != nil {
		return run.tickFailed(source, "", err)
	}
	return run.tickFailed(source, "", run.flushIdleNotice())
}

// tickTarget resolves --agent. Hooks pass a hitch ID and operators pass a
// registered name. A hook may name an agent dropped since it fired, and
// tickAgent ignores that agent, so only an operator tick refuses an unknown
// value.
func (run *runtime) tickTarget(value string) (core.HitchID, error) {
	id := core.HitchID(value)
	if p, err := run.team.Agent(id); err == nil {
		if _, err := p.Read(); err == nil {
			return id, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if resolved, err := run.team.ResolveName(value); err == nil {
		return resolved, nil
	}
	if run.cmd.environment("GANGLINE_BOUNDARY") != "" {
		return id, nil
	}
	return "", refuseError("tick: %q is neither a registered agent nor a hitch ID", value)
}

// tickFailed records a tick error in the team log. A hook-triggered tick runs
// detached with no standard streams, so this record is its only diagnostic.
func (run *runtime) tickFailed(source string, id core.HitchID, err error) error {
	if err == nil {
		return nil
	}
	if pending, ok := err.(*pendingInputError); ok {
		return commandError{status: exitUnknown, text: pending.Error()}
	}
	e := core.Event{Type: "tick_failed", At: run.cmd.now(), Source: source, HitchID: id, Reason: boundedFailureReason(err.Error())}
	if id != "" {
		if p, pathErr := run.team.Agent(id); pathErr == nil {
			if a, readErr := p.Read(); readErr == nil {
				e.Name = a.Name
			}
		}
	}
	return errors.Join(err, run.team.Append(e))
}
func (cmd command) log(args []string) error {
	filter, files, err := parseLogFilter(args, true)
	if err != nil {
		return err
	}
	path := ""
	if len(files) > 0 && cmd.team != "" {
		return usageError("log: --team does not apply to LOG.jsonl")
	}
	if len(files) > 0 {
		path = files[0]
	} else {
		run, err := cmd.runtime()
		if err != nil {
			return err
		}
		path = run.team.Log
	}
	if filter.Metadata || filter.Summary {
		return cmd.logMetadata(path, filter)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeFilteredLog(cmd.stdout, cmd.stderr, f, filter, cmd.operatorText)
}
func (cmd command) wait(args []string) error {
	o, err := parseWait(args)
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
	p, err := run.team.Agent(a.ID)
	if err != nil {
		return err
	}
	ctx, cancel := cmd.timeout(o.Timeout)
	defer cancel()
	for {
		watch, err := cmd.watch(p.State)
		if err != nil {
			return err
		}
		a, err = p.Read()
		if err != nil {
			_ = watch.Close()
			return err
		}
		l, current, lockErr := run.acquire(a.ID, false)
		if lockErr == nil {
			a = current
			if err := run.release(l); err != nil {
				_ = watch.Close()
				return err
			}
		} else if !errors.Is(lockErr, store.ErrLocked) {
			_ = watch.Close()
			return lockErr
		} else {
			a, _ = core.Step(a, core.Event{Type: "deadline_checked", At: cmd.now(), HitchID: a.ID})
		}
		if a.Status == core.Active && a.Activity == core.Idle {
			_ = watch.Close()
			return nil
		}
		if a.Status == core.Failed || a.Status == core.Dropping {
			_ = watch.Close()
			return refuseError("agent %q is %s", a.Name, a.Status)
		}
		if o.Timeout == 0 {
			_ = watch.Close()
			return refuseError("agent %q is not idle", a.Name)
		}
		err = watch.Wait(ctx)
		closeErr := watch.Close()
		if errors.Is(err, context.DeadlineExceeded) {
			return refuseError("agent %q did not become idle before the wait deadline", a.Name)
		}
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
}
