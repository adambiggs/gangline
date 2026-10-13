package main

import (
	"errors"
	"fmt"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// foreignSubmit says why a submit hook cannot come from the agent's own
// conversation, or is empty when it may. A native harness can run other
// sessions in the same process, and their hooks reach the hitch too: a spawned
// agent names itself, and a session the harness never persists has no
// transcript. Neither can replace the conversation, so neither is a session
// change. A different session with a transcript is left to the continuation
// guard, and the first submit registers whichever session sent it.
func foreignSubmit(a core.Agent, event harness.HookEvent) string {
	session := event.Payload["session_id"]
	if event.Kind != string(harness.TurnStarted) || session == "" || a.Native.SessionID == "" || session == a.Native.SessionID {
		return ""
	}
	if agent := event.Payload["agent_id"]; agent != "" {
		return fmt.Sprintf("native session %s submitted as spawned agent %s", session, agent)
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
