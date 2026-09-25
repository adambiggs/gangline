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

func TestSubmittedPromptMatchesClaudePastedContent(t *testing.T) {
	sent := "[gang:lead#message-1] first\nsecond [/gang:lead#message-1]"
	for _, witness := range []string{
		"\n\n<pasted_content id=\"c623\">\n" + sent + "\n</pasted_content id=\"c623\">\n",
		"<pasted_content id=\"c623\">\n" + sent + "\n</pasted_content id=\"c623\">",
	} {
		matched, err := SubmittedPromptMatches(Invocation{Name: "claude-pasted-content"}, sent, witness)
		if err != nil || !matched {
			t.Fatalf("matched = %v, err = %v for %q", matched, err, witness)
		}
	}
}

func TestSubmittedPromptMatchRejectsChangedContentOrWrapper(t *testing.T) {
	sent := "[gang:lead#message-1] first\nsecond [/gang:lead#message-1]"
	for _, witness := range []string{
		"\n\n<pasted_content id=\"c623\">\n[gang:lead#message-1] first [/gang:lead#message-1]\n</pasted_content id=\"c623\">\n",
		"\n\n<pasted_content id=\"c623\">\n" + sent + "\n</pasted_content id=\"other\">\n",
		"anything",
	} {
		matched, err := SubmittedPromptMatches(Invocation{Name: "claude-pasted-content"}, sent, witness)
		if err != nil {
			t.Fatal(err)
		}
		if matched {
			t.Fatalf("changed witness matched: %q", witness)
		}
	}
}

func TestSubmittedPromptStartsWithPreservesContinuationBoundary(t *testing.T) {
	sent := "[gang:compact#token resume] state [/gang:compact#token]"
	for _, tc := range []struct {
		name, witness string
		want          bool
	}{
		{"codex merged", sent + "\nlater input", true},
		{"codex reordered", "later input\n" + sent, false},
		{"codex altered", sent + " altered", false},
		{"claude merged", "<pasted_content id=\"p\">\n" + sent + "\n</pasted_content id=\"p\">\nlater input", true},
		{"claude altered", "<pasted_content id=\"p\">\n" + sent + " altered\n</pasted_content id=\"p\">\nlater input", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			primitive := Invocation{Name: "exact-prompt"}
			if len(tc.name) >= 6 && tc.name[:6] == "claude" {
				primitive.Name = "claude-pasted-content"
			}
			got, err := SubmittedPromptStartsWith(primitive, sent, tc.witness)
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}
