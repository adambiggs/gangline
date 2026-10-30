package main

import (
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestForeignHookWithParentSession(t *testing.T) {
	collar, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	for _, native := range []string{"UserPromptSubmit", "PostToolUse", "PermissionRequest", "PreCompact", "PostCompact"} {
		t.Run(native, func(t *testing.T) {
			for _, registered := range []string{"", "parent"} {
				agent := core.Agent{Native: core.NativeState{SessionID: registered}}
				event, err := harness.DecodeHook(collar, []byte(`{"hook_event_name":"`+native+`","session_id":"parent","agent_id":"child","transcript_path":"/sessions/child.jsonl","turn_id":"child-turn"}`))
				if err != nil {
					t.Fatal(err)
				}
				if reason := foreignSubmit(agent, event); reason == "" {
					t.Errorf("child hook accepted with registered session %q", registered)
				}
				delete(event.Payload, "agent_id")
				if reason := foreignSubmit(agent, event); reason != "" {
					t.Errorf("conversation hook rejected: %s", reason)
				}
			}
		})
	}
}

func TestForeignHookAcceptsAbsentChildIdentity(t *testing.T) {
	collar, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	for _, native := range []string{"UserPromptSubmit", "PostToolUse", "PermissionRequest", "PreCompact", "PostCompact"} {
		for _, field := range []string{"", `,"agent_id":null`, `,"agent_id":""`} {
			event, err := harness.DecodeHook(collar, []byte(`{"hook_event_name":"`+native+`","session_id":"parent","transcript_path":"parent.jsonl"`+field+`}`))
			if err != nil {
				t.Fatal(err)
			}
			if reason := foreignSubmit(core.Agent{Native: core.NativeState{SessionID: "parent"}}, event); reason != "" {
				t.Errorf("parent %s hook with %q identity rejected: %s", native, field, reason)
			}
		}
	}
}
