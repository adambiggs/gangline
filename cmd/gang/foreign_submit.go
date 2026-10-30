package main

import (
	"errors"
	"fmt"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// foreignSubmit rejects spawned-agent hooks before matching the conversation's
// session: a child can report its parent's session ID with its own transcript.
// A submit without a transcript cannot replace the conversation either.
func foreignSubmit(a core.Agent, event harness.HookEvent) string {
	session := event.Payload["session_id"]
	if agent := event.Payload["agent_id"]; agent != "" {
		return fmt.Sprintf("native session %s submitted as spawned agent %s", session, agent)
	}
	if event.Kind != string(harness.TurnStarted) || session == "" || session == a.Native.SessionID {
		return ""
	}
	if event.Absent["transcript_path"] {
		return fmt.Sprintf("native session %s submitted without a transcript", session)
	}
	return ""
}

// recordForeignSubmit retains a foreign submit's raw payload with the team and
// logs it, leaving the agent's registration and witness untouched.
func (run *runtime) recordForeignSubmit(a core.Agent, event harness.HookEvent, payload []byte, reason string) error {
	for _, value := range []string{event.Payload["session_id"], event.Payload["agent_id"]} {
		if len(value) > 4096 {
			return fmt.Errorf("hook routing metadata exceeds maximum size")
		}
	}
	path, retentionErr := run.team.WriteForeignSubmit(store.ForeignSubmit{
		At: run.cmd.now(), HitchID: a.ID, Name: a.Name, Registered: a.Native.SessionID,
		SessionID: event.Payload["session_id"], Reason: reason, Payload: string(payload),
	})
	text := reason + ", so it is not the agent's conversation"
	if retentionErr == nil {
		text += "; raw payload retained at " + path
	}
	return errors.Join(retentionErr, run.record(a, core.Event{Type: "native_hook", NativeEvent: event.NativeEvent, Status: "foreign-session", Reason: text}))
}
