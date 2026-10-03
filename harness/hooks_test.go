package harness

import (
	"strings"
	"testing"
)

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

func TestDecodeHookCountsDeclaredArrays(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		body  string
		count int
		known bool
	}{
		{`{"hook_event_name":"Stop","background_tasks":[{"type":"shell","status":"running"},{"type":"workflow","status":"running"}]}`, 2, true},
		{`{"hook_event_name":"Stop","background_tasks":[]}`, 0, true},
		{`{"hook_event_name":"Stop"}`, 0, false},
		{`{"hook_event_name":"Stop","background_tasks":"two"}`, 0, false},
	} {
		event, err := DecodeHook(collar, []byte(c.body))
		if err != nil {
			t.Fatal(err)
		}
		count, known := event.Counts["background_tasks"]
		if count != c.count || known != c.known {
			t.Fatalf("%s: count = %d, known = %v", c.body, count, known)
		}
	}
}

func TestDecodeHookRefusesUnknownNativeEvent(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
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

// Claude Code 2.1.287 delivered briefs that quoted its own wrapper tag with a
// backslash after each '<' that opened one, in a long wrapped paste and in a
// short unwrapped one; the rest of the text was unchanged.
func TestSubmittedPromptMatchesClaudeEscapedWrapperTag(t *testing.T) {
	sent := "[gang:lead#message-1] wrapped in `<pasted_content>` and </pasted_content> [/gang:lead#message-1]"
	body := "[gang:lead#message-1] wrapped in `<\\pasted_content>` and <\\/pasted_content> [/gang:lead#message-1]"
	witness := "\n\n<pasted_content id=\"0c3d\">\n" + body + "\n</pasted_content id=\"0c3d\">\n"
	primitive := Invocation{Name: "claude-pasted-content"}
	if matched, err := SubmittedPromptMatches(primitive, sent, witness); err != nil || !matched {
		t.Fatalf("escaped tag: matched = %v, err = %v", matched, err)
	}
	if matched, err := SubmittedPromptMatches(primitive, sent, body); err != nil || !matched {
		t.Fatalf("escaped tag without wrapper: matched = %v, err = %v", matched, err)
	}
	// A lookalike opener is replaced by the same ASCII escape.
	lookalike := "[gang:lead#message-1] quoting ‹pasted_content› and ＜/pasted_content＞ [/gang:lead#message-1]"
	escaped := "[gang:lead#message-1] quoting <\\pasted_content› and <\\/pasted_content＞ [/gang:lead#message-1]"
	if matched, err := SubmittedPromptMatches(primitive, lookalike, escaped); err != nil || !matched {
		t.Fatalf("escaped lookalike opener: matched = %v, err = %v", matched, err)
	}
	// Only an opener may become the escape; an ASCII letter or bracket may not.
	for _, other := range []string{"(", "a"} {
		changed := strings.Replace(sent, "`<pasted_content>`", "`"+other+"pasted_content>`", 1)
		if matched, err := SubmittedPromptMatches(primitive, changed, body); err != nil || matched {
			t.Fatalf("escape stood for %q: matched = %v, err = %v", other, matched, err)
		}
	}
	if matched, err := SubmittedPromptStartsWith(primitive, sent, witness+"later input"); err != nil || !matched {
		t.Fatalf("escaped tag with later input: matched = %v, err = %v", matched, err)
	}
	for _, altered := range []string{
		strings.Replace(body, "<\\pasted", "\\<pasted", 1),
		strings.Replace(body, "and <", "and <\\\\", 1),
		strings.Replace(body, "<\\pasted_content>", "<\\\\pasted_content>", 1),
		strings.Replace(body, " [/gang", "\\ [/gang", 1),
	} {
		for _, witness := range []string{"\n\n<pasted_content id=\"0c3d\">\n" + altered + "\n</pasted_content id=\"0c3d\">\n", altered} {
			if matched, err := SubmittedPromptMatches(primitive, sent, witness); err != nil || matched {
				t.Fatalf("altered text matched = %v, err = %v: %q", matched, err, witness)
			}
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
