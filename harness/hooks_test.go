package harness

import "testing"

func TestDecodeHookMapsNativeBoundaryAndPayload(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	event, err := DecodeHook(collar, []byte(`{
  "hook_event_name":"Stop",
  "session_id":"session-1",
  "turn_id":"turn-2",
  "transcript_path":"/state/rollout.jsonl"
}`))
	if err != nil {
		t.Fatal(err)
	}
	if event.Kind != "turn-finished" || event.Payload["turn_id"] != "turn-2" {
		t.Fatalf("event = %+v", event)
	}
}

func TestDecodeHookRefusesUnknownNativeEvent(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeHook(collar, []byte(`{"hook_event_name":"Unknown"}`)); err == nil {
		t.Fatal("unknown hook event was accepted")
	}
}

func TestDetectTurnBoundaryIgnoresActivityHook(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	boundary, event, err := DetectTurnBoundary(collar, []byte(`{"hook_event_name":"PostToolUse"}`))
	if err != nil {
		t.Fatal(err)
	}
	if boundary != "" || event.Kind != "activity" {
		t.Fatalf("boundary = %q, event = %+v", boundary, event)
	}
}
