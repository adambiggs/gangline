package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

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
}

func (cmd command) hook(args []string) error {
	err := cmd.handleHook(args)
	if err != nil {
		if cmd.stderr != nil {
			_, _ = fmt.Fprintf(cmd.stderr, "gang hook: %v\n", err)
		}
		if run, setupErr := cmd.runtime(); setupErr == nil {
			_ = run.team.Append(core.Event{Type: "hook_failed", At: cmd.now(), HitchID: core.HitchID(cmd.environment("GANGLINE_HITCH_ID")), Pane: cmd.environment("TMUX_PANE"), Reason: err.Error()})
		}
	}
	return nil
}
func (cmd command) handleHook(args []string) error {
	if err := noArguments(args, "hook"); err != nil {
		return err
	}
	payload, err := io.ReadAll(io.LimitReader(cmd.stdin, maximumHookBytes+1))
	if err != nil {
		return err
	}
	if len(payload) > maximumHookBytes {
		return fmt.Errorf("hook payload exceeds maximum size")
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
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	_, event, err := harness.DetectTurnBoundary(c, payload)
	if err != nil {
		return err
	}
	receipt, err := randomID("hook")
	if err != nil {
		return err
	}
	if err := run.record(a, core.Event{Type: "native_hook", ID: receipt, NativeEvent: event.NativeEvent, Status: event.Kind}); err != nil {
		return err
	}
	switch event.Kind {
	case "turn-started":
		if err := p.WriteWitness(store.Witness{ID: receipt, At: cmd.now(), Prompt: event.Payload["prompt"], SessionID: event.Payload["session_id"], TurnID: event.Payload["turn_id"], Transcript: event.Payload["transcript_path"]}); err != nil {
			return err
		}
		if a.LastFailed == "" && a.LastAccepted == "" {
			return nil
		}
		// The native queue may submit after the sending command has exited.
		// A detached tick reconciles its receipt without holding up the hook.
		if cmd.detach != nil {
			return cmd.detach(string(id), hookNotice{Kind: "turn-started"})
		}
		return cmd.detachTick(string(id), hookNotice{Kind: "turn-started"}, run.settings)
	case "turn-finished", "turn-failed", "compaction-finished":
		n := hookNotice{Kind: event.Kind, NativeEvent: event.NativeEvent, At: cmd.now(), SessionID: event.Payload["session_id"], TurnID: event.Payload["turn_id"], Transcript: event.Payload["transcript_path"]}
		for _, value := range []string{n.SessionID, n.TurnID, n.Transcript, n.NativeEvent} {
			if len(value) > 4096 {
				return fmt.Errorf("hook routing metadata exceeds maximum size")
			}
		}
		if cmd.detach != nil {
			return cmd.detach(string(id), n)
		}
		return cmd.detachTick(string(id), n, run.settings)
	}
	return nil
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
	// Nil standard streams connect to the null device, not the harness pipes.
	if err := child.Start(); err != nil {
		return err
	}
	return child.Process.Release()
}
func (cmd command) tick(args []string) (result error) {
	id, source, generation := "", "", ""
	flags := quietFlagSet("tick")
	flags.StringVar(&id, "agent", "", "hitch ID")
	flags.StringVar(&source, "source", "", "tick source")
	flags.StringVar(&generation, "watchdog", "", "watchdog generation")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return usageError("tick: expected optional --agent ID or --source watchdog --watchdog UNIT")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	var notice hookNotice
	if id != "" && cmd.environment("GANGLINE_BOUNDARY") != "" {
		if err := json.Unmarshal([]byte(cmd.environment("GANGLINE_BOUNDARY")), &notice); err != nil {
			return err
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
		return err
	}
	if id != "" {
		return run.tickAgent(core.HitchID(id), notice, true)
	}
	agents, err := run.team.ListAgents()
	if err != nil {
		return err
	}
	return eachAgent(agents, func(a core.Agent) error { return run.tickAgent(a.ID, hookNotice{}, false) })
}
func (cmd command) log(args []string) error {
	filter, files, err := parseLogFilter(args, true)
	if err != nil {
		return err
	}
	path := ""
	if len(files) > 0 {
		path = files[0]
	} else {
		run, err := cmd.runtime()
		if err != nil {
			return err
		}
		path = run.team.Log
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeFilteredLog(cmd.stdout, f, filter)
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
