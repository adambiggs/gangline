package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTranscriptCursorRejectsWrongSessionAndTruncation(t *testing.T) {
	input := `{"type":"session_meta","payload":{"id":"s1"}}
{"timestamp":"2026-09-22T10:00:00Z","type":"event_msg","payload":{"type":"context_compacted"}}
{"type":"event_msg"`
	primitive := Invocation{Name: "codex-session-log"}
	if _, err := ReadTranscript(primitive, strings.NewReader(input), "other", 0, time.Time{}); err == nil {
		t.Fatal("cross-session transcript accepted")
	}
	first, err := ReadTranscript(primitive, strings.NewReader(input), "s1", 0, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Readings) != 1 || first.Readings[0].Kind != "compaction-finished" {
		t.Fatalf("readings = %#v", first.Readings)
	}
	second, err := ReadTranscript(primitive, strings.NewReader(input), "s1", first.Offset, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Readings) != 0 || second.Offset != first.Offset {
		t.Fatalf("partial or previously consumed record repeated: %#v", second)
	}
	if _, err := ReadTranscript(primitive, strings.NewReader(input[:first.Offset-2]), "s1", first.Offset, time.Time{}); err == nil {
		t.Fatal("truncated transcript accepted")
	}
}

func TestTranscriptMalformedRecognizedShapeFails(t *testing.T) {
	prefix := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"s\"}}\n"
	for _, payload := range []string{`{"type":"token_count","info":{}}`, `{"type":"token_count","info":{"last_token_usage":{"total_tokens":-1},"model_context_window":200000}}`, `{"type":"error"}`} {
		line := `{"timestamp":"2026-09-22T10:00:00Z","type":"event_msg","payload":` + payload + "}\n"
		if _, err := ReadTranscript(Invocation{Name: "codex-session-log"}, strings.NewReader(prefix+line), "s", 0, time.Time{}); err == nil {
			t.Errorf("accepted %s", payload)
		}
	}
}

func TestStatuslineUnknownIsNotZero(t *testing.T) {
	_, readings, err := ReadStatusline([]byte(`{"session_id":"s","context_window":{"context_window_size":200000,"current_usage":null}}`))
	if err != nil {
		t.Fatal(err)
	}
	if readings[0].Status != "unknown" || readings[0].Used != nil {
		t.Fatalf("missing usage became zero: %#v", readings[0])
	}
}

func TestInstallStatuslinePreservesCustomSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	data := []byte(`{"permissions":{"defaultMode":"default"},"statusLine":{"type":"command","command":"my-footer"}}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := InstallStatusline(path, "/opt/Gang Line/gang")
	if err != nil || changed {
		t.Fatalf("custom setting: changed=%t error=%v", changed, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(data) {
		t.Fatalf("custom settings changed: %s %v", got, err)
	}
	empty := filepath.Join(t.TempDir(), "settings.json")
	changed, err = InstallStatusline(empty, "/opt/Gang Line/gang")
	if err != nil || !changed {
		t.Fatalf("absent setting: changed=%t error=%v", changed, err)
	}
	got, err = os.ReadFile(empty)
	if err != nil || !strings.Contains(string(got), "statusline") {
		t.Fatalf("missing installed command: %s %v", got, err)
	}
}

func TestClaudeLaunchInstallsManagedStatusline(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	command, err := RenderLaunch(collar, LaunchOptions{HookCommand: []string{"/installed/gang", "hook"}})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(command.Args, " ")
	if !strings.Contains(joined, `"statusLine":{"command"`) && !strings.Contains(joined, `"statusLine":{"type"`) {
		t.Fatalf("missing statusLine settings: %s", joined)
	}
	if !strings.Contains(joined, "/installed/gang statusline") || !strings.Contains(joined, "StopFailure") {
		t.Fatalf("missing installed callbacks: %s", joined)
	}
}

func TestTerminalFailureIsANativeTurnBoundary(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	boundary, event, err := DetectTurnBoundary(collar, []byte(`{"hook_event_name":"StopFailure","prompt_id":"native-turn","error":"rate_limit","error_details":"try later"}`))
	if err != nil {
		t.Fatal(err)
	}
	if boundary != TurnFailed || event.Kind != "turn-failed" || event.Payload["error"] != "rate_limit" || event.Payload["turn_id"] != "native-turn" {
		t.Fatalf("failure = %q %#v", boundary, event)
	}
}

func TestStatuslineUsesLatestMatchingNativeMeasurement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	data := `{"type":"assistant","sessionId":"s","timestamp":"2026-09-22T10:00:00Z","message":{"model":"claude-test","usage":{"input_tokens":100,"cache_creation_input_tokens":200,"cache_read_input_tokens":300}}}
{"type":"assistant","sessionId":"s","timestamp":"2026-09-22T10:01:00Z","message":{"model":"claude-test","usage":{"input_tokens":20,"cache_creation_input_tokens":20,"cache_read_input_tokens":20}}}
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	payload := []byte(fmt.Sprintf(`{"session_id":"s","transcript_path":%q}`, path))
	old, current := int64(600), int64(60)
	at, err := StatuslineMeasurementTime(payload, Reading{Used: &old, Model: "claude-test"})
	if err != nil {
		t.Fatal(err)
	}
	if at != nil {
		t.Fatalf("old statusline matched latest assistant: %v", at)
	}
	at, err = StatuslineMeasurementTime(payload, Reading{Used: &current, Model: "claude-test"})
	if err != nil {
		t.Fatal(err)
	}
	if at == nil || at.Format(time.RFC3339) != "2026-09-22T10:01:00Z" {
		t.Fatalf("measurement = %v", at)
	}
}

func TestTerminalFailureHookReleasesNativeDispatcher(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	launch, err := RenderLaunch(collar, LaunchOptions{HookCommand: []string{"gang", "hook"}})
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Async bool `json:"async"`
			}
		} `json:"hooks"`
	}
	found := false
	for i, arg := range launch.Args {
		if arg == "--settings" && i+1 < len(launch.Args) {
			if err := json.Unmarshal([]byte(launch.Args[i+1]), &settings); err != nil {
				t.Fatal(err)
			}
			found = true
			break
		}
	}
	hooks := settings.Hooks["StopFailure"]
	if !found || len(hooks) != 1 || len(hooks[0].Hooks) != 1 || !hooks[0].Hooks[0].Async {
		t.Fatal("terminal failure owns queued delivery while blocking the native dispatcher")
	}
}
