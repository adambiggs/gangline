package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
)

// callerInTeam returns the agent of the selected team whose hitch identity
// this process inherited, or nil when it carries none there: the operator at
// a keyboard, or an agent of another team acting on a team it selected.
// Every shell an agent starts inherits its pane's identity, team, and state
// root, so a command run there acts as that agent.
func (run *runtime) callerInTeam() (*core.Agent, error) {
	id := core.HitchID(run.cmd.environment("GANG_AGENT_ID"))
	if id == "" {
		return nil, nil
	}
	p, err := run.team.Agent(id)
	if err != nil {
		return nil, err
	}
	a, err := p.Read()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// holdsToken reports whether this process holds the pane token registered for
// a, which only a process started in a's pane inherits.
func (run *runtime) holdsToken(a core.Agent) bool {
	token := run.cmd.environment("GANG_AGENT_NONCE")
	return token != "" && a.Registration.TokenHash != "" && tokenHash(token) == a.Registration.TokenHash
}

// isLead reports whether a holds the lead's authority: the lead role, given by
// the operator rather than by an agent that hitched it. A name confers nothing,
// since any agent may rename itself.
func isLead(a core.Agent) bool { return a.Role == "lead" && a.HitchedBy == "" }

// leadCaller reports whether caller is the selected team's lead, proven by its
// pane token. A nil caller is not an agent of the team.
func (run *runtime) leadCaller(caller *core.Agent) bool {
	return caller != nil && isLead(*caller) && run.holdsToken(*caller)
}

// refuseTeamWide refuses an operation that ends or schedules the end of every
// agent in the team when a non-lead agent of that team runs it.
func (run *runtime) refuseTeamWide(operation string) error {
	caller, err := run.callerInTeam()
	if err != nil || caller == nil || run.leadCaller(caller) {
		return err
	}
	return refuseError("%s refused: this process carries the hitch identity of %s, an agent of team %q that is not its lead; this protects every agent in the team from a teammate's command. Ask the lead to run it, or act on a test team by setting GANG_SESSION and GANG_STATE_ROOT to that team", operation, caller.Name, run.settings.Session)
}

// refuseDrop refuses a drop of target by an agent of the team other than its
// lead or the agent that hitched target. A target with no recorded hitcher is
// the lead's to drop.
func (run *runtime) refuseDrop(target core.Agent) error {
	caller, err := run.callerInTeam()
	if err != nil || caller == nil || run.leadCaller(caller) {
		return err
	}
	if target.HitchedBy != "" && target.HitchedBy == caller.ID && run.holdsToken(*caller) {
		return nil
	}
	hitcher := "records no hitcher, so only the lead may drop it"
	if target.HitchedBy != "" {
		hitcher = "was hitched by another agent, so only that agent or the lead may drop it"
		if p, err := run.team.Agent(target.HitchedBy); err == nil {
			if a, err := p.Read(); err == nil && isLead(a) {
				hitcher = "was hitched by " + string(a.Name) + ", the lead, so only the lead may drop it"
			} else if err == nil {
				hitcher = "was hitched by " + string(a.Name) + ", so only " + string(a.Name) + " or the lead may drop it"
			}
		}
	}
	return refuseError("drop refused: this process carries the hitch identity of %s in team %q, and %s %s; this protects an agent from a teammate that did not start it. Ask that agent or the lead to drop %s", caller.Name, run.settings.Session, target.Name, hitcher, target.Name)
}

// notifyHitcher tells the agent that hitched a that a has failed, since
// nothing else reaches it: the failure is usually observed by a tick, and the
// failed agent can no longer report. A boot failure the hitch command returns
// to its caller is not repeated. A notice that cannot be sent is logged and
// warned about rather than failing the operation that observed the failure.
func (run *runtime) notifyHitcher(a core.Agent, reason string) error {
	if a.HitchedBy == "" || a.HitchedBy == a.ID || a.ID == run.hitching {
		return nil
	}
	id := core.EnvelopeID("failed-" + a.ID)
	err := run.sendHitcherNotice(a, id, reason)
	if err == nil {
		return nil
	}
	if err := run.record(a, core.Event{Type: "notice_failed", ID: string(id), Reason: err.Error()}); err != nil {
		return err
	}
	if run.cmd.stderr != nil {
		if _, err := fmt.Fprintf(run.cmd.stderr, "warning: could not tell the agent that hitched %s about its failure: %v\n", a.Name, err); err != nil {
			return err
		}
	}
	return nil
}

func (run *runtime) sendHitcherNotice(a core.Agent, id core.EnvelopeID, reason string) error {
	p, err := run.team.Agent(a.HitchedBy)
	if err != nil {
		return err
	}
	hitcher, err := p.Read()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if hitcher.Status != core.Active {
		return nil
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	text := fmt.Sprintf("%s, which you hitched, failed: %s. gang status %s shows its record.", a.Name, reason, a.Name)
	if err := run.publishOnceTo(p, hitcher, core.Envelope{ID: id, Token: token, Recipient: hitcher.ID, To: hitcher.Name, From: core.Sender{Kind: core.SenderGangline, Name: "hitch"}, Message: core.Message{Text: text}, CreatedAt: run.cmd.now()}); err != nil {
		return err
	}
	run.wake = append(run.wake, hitcher.ID)
	return nil
}

// downRecord names who ended a team and from where. Down deletes the team's
// directory and its event log, and its own drops can end the process running
// it, so the record is written to the state root before any agent is dropped.
type downRecord struct {
	At            time.Time `json:"at"`
	Team          string    `json:"team"`
	Agents        int       `json:"agents"`
	Caller        string    `json:"caller,omitempty"`
	HitchIdentity string    `json:"hitch_identity,omitempty"`
	PID           int       `json:"pid"`
	ParentPID     int       `json:"parent_pid"`
	ParentCommand string    `json:"parent_command,omitempty"`
	Directory     string    `json:"directory,omitempty"`
	TmuxPane      string    `json:"tmux_pane,omitempty"`
}

func (run *runtime) recordDown(agents int) error {
	r := downRecord{At: run.cmd.now(), Team: run.settings.Session, Agents: agents, HitchIdentity: run.cmd.environment("GANG_AGENT_ID"), PID: os.Getpid(), ParentPID: os.Getppid(), TmuxPane: run.cmd.environment("TMUX_PANE")}
	if caller, err := run.callerInTeam(); err == nil && caller != nil {
		r.Caller = string(caller.Name)
	}
	// Linux names the parent's command line; elsewhere the record keeps its pid.
	if argv, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", r.ParentPID)); err == nil {
		r.ParentCommand = strings.TrimSpace(strings.ReplaceAll(string(argv), "\x00", " "))
	}
	if dir, err := run.cmd.getwd(); err == nil {
		r.Directory = dir
	}
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(run.settings.StateRoot, "downs.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("record down: %w", err)
	}
	_, err = f.Write(append(line, '\n'))
	return errors.Join(err, f.Sync(), f.Close())
}
