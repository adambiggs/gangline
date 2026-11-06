package harness

import "testing"

func TestCapacityRequiresFinalNativeErrorAndIdleComposer(t *testing.T) {
	// Screen shape: the Codex TUI after a capacity failure, a retry, and a
	// second capacity failure.
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	const failure = "■ Selected model is at capacity. Please try a different model."
	for _, tc := range []struct {
		name           string
		lines          []string
		found, unknown bool
	}{
		{"historical retry text", []string{"› earlier task", "Reconnecting... 1/5", "• Done.", "› do the work", failure, "› "}, true, false},
		{"retry word in prompt", []string{"› check retrying behavior", failure, "› "}, true, false},
		{"native terminal error", []string{"› do the work", "", failure, "", "› "}, true, false},
		{"wrapped native terminal error", []string{"› do the work", "", "■ Selected model is at capacity. Please try a", "  different model.", "", "› "}, true, false},
		{"historical error before later answer", []string{"› do the work", failure, "", "› continue", "", "• Done.", "", "› "}, false, false},
		{"historical error before new prompt", []string{"› do the work", failure, "", "› continue", "", "› "}, false, false},
		{"native retry", []string{"› do the work", failure, "Reconnecting... 1/5", "", "› "}, false, false},
		{"native retry footer", []string{"› do the work", failure, "", "› ", "Reconnecting... 1/5"}, false, false},
		{"hook review", []string{"Hooks need review", "› 1. Review hooks"}, false, false},
		{"permission", []string{"› do the work", failure, "", "Would you like to run the following command?", "› 1. Yes, proceed"}, false, false},
		{"historical permission text", []string{"Would you like to run", "Yes, proceed", "› do the work", failure, "", "› "}, true, false},
		{"busy", []string{"› do the work", failure, "esc to interrupt", "› "}, false, false},
		{"typed composer", []string{"› do the work", failure, "", "› draft"}, false, false},
		{"clipped originating prompt", []string{failure, "", "› "}, false, true},
		{"collapsed originating prompt", []string{"› [Pasted Content 3 chars]", failure, "", "› "}, false, true},
		{"unrelated provider error", []string{"› do the work", "■ Quota exceeded.", "", "› "}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, found, err := DetectCapacity(collar, codexComposerScreen(tc.lines))
			if found != tc.found || (err != nil) != tc.unknown {
				t.Fatalf("found=%v capacity=%+v error=%v", found, got, err)
			}
		})
	}
}

func TestCapacityScreenFallbackRequiresCurrentEnvelope(t *testing.T) {
	c, _ := EmbeddedCollar("codex")
	const prompt = "[gang:lead#0123456789abcdef] Do the work. [/gang:lead#0123456789abcdef]"
	const failure = "■ Selected model is at capacity. Please try a different model."
	screen := codexComposerScreen([]string{"› " + prompt, "• Ran a tool", "", failure, "", "› "})
	turn := CapacityTurn{SessionID: "s", TurnID: "t1", Prompt: prompt}
	got, found, err := DetectCapacityForTurn(c, screen, turn)
	if err != nil || !found || got.Fingerprint != turn.Fingerprint() {
		t.Fatalf("current envelope not recovered: %+v found=%v err=%v", got, found, err)
	}
	turn.Failure = "server_overloaded: try later"
	native, found, err := DetectCapacityForTurn(c, screen, turn)
	if err != nil || !found || native.Fingerprint != got.Fingerprint {
		t.Fatalf("later native evidence changed receipt identity: %+v found=%v err=%v", native, found, err)
	}
	turn.Failure, turn.Prompt = "", "[gang:lead#fedcba9876543210] New work. [/gang:lead#fedcba9876543210]"
	if _, found, err := DetectCapacityForTurn(c, screen, turn); found || err == nil {
		t.Fatalf("old history matched new envelope: found=%v err=%v", found, err)
	}
}

func TestCapacityDistinguishesConsecutiveNonceBearingFailures(t *testing.T) {
	collar, _ := EmbeddedCollar("codex")
	read := func(nonce string) Capacity {
		got, found, err := DetectCapacity(collar, testScreen(testCells("› [gang:capacity-recovery#"+nonce+"] Continue [/gang:capacity-recovery#"+nonce+"]", false), testCells("■ Selected model is at capacity. Please try a different model.", false), testCells("› ", false)))
		if err != nil || !found {
			t.Fatalf("found=%v error=%v", found, err)
		}
		return got
	}
	if read("one").Fingerprint == read("two").Fingerprint {
		t.Fatal("fresh native failure reused historical error identity")
	}
	if read("one").Fingerprint != read("one").Fingerprint {
		t.Fatal("repeated observation changed failure identity")
	}
}

func TestCapacityNativeTurnIdentityAndInputEligibility(t *testing.T) {
	c, _ := EmbeddedCollar("codex")
	const reason = "server_overloaded: Selected model is at capacity. Please try a different model."
	const failure = "■ Selected model is at capacity. Please try a different model."
	for _, tc := range []struct {
		name, turn, reason string
		lines              []string
		want               bool
	}{
		{"clipped history", "t1", reason, []string{"• Ran a tool", "", failure, "", "› "}, true},
		{"collapsed prompt", "t1", reason, []string{"› [Pasted Content 100 lines]", failure, "", "› "}, true},
		{"no error painted yet", "t1", reason, []string{"› "}, true},
		{"later unfinished turn", "t2", "", []string{"› old work", failure, "", "› "}, false},
		{"different failure", "t2", "cyber_policy: refused", []string{"› old work", failure, "", "› "}, false},
		{"busy", "t1", reason, []string{failure, "• Working (esc to interrupt)", "› "}, false},
		{"retrying", "t1", reason, []string{failure, "› ", "Reconnecting... 1/5"}, false},
		{"draft", "t1", reason, []string{failure, "› unsent input"}, false},
		{"permission", "t1", reason, []string{failure, "Would you like to run the following command?", "› 1. Yes, proceed"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, found, err := DetectCapacityForTurn(c, codexComposerScreen(tc.lines), CapacityTurn{SessionID: "s", TurnID: tc.turn, Failure: tc.reason})
			if err != nil && tc.name != "later unfinished turn" || found != tc.want {
				t.Fatalf("found=%v err=%v", found, err)
			}
		})
	}
	screen := codexComposerScreen([]string{"› "})
	read := func(session, turn, reason string) Capacity {
		t.Helper()
		got, found, err := DetectCapacityForTurn(c, screen, CapacityTurn{SessionID: session, TurnID: turn, Failure: reason})
		if err != nil || !found {
			t.Fatalf("found=%v err=%v", found, err)
		}
		return got
	}
	one := read("s", "t1", reason)
	if one.Fingerprint != read("s", "t1", "server_overloaded: Try later.").Fingerprint {
		t.Fatal("wording changed the failed turn's receipt identity")
	}
	if one.Fingerprint == read("s", "t2", reason).Fingerprint || one.Fingerprint == read("other", "t1", reason).Fingerprint {
		t.Fatal("distinct native failures share a receipt identity")
	}
}

func TestCapacityReadsUnnamedGanglineEnvelope(t *testing.T) {
	collar, _ := EmbeddedCollar("codex")
	nonce := "0123456789abcdef"
	prompt := "[gang#" + nonce + "] Continue [/gang#" + nonce + "]"
	got, found, err := DetectCapacity(collar, testScreen(testCells("› "+prompt, false), testCells("■ Selected model is at capacity. Please try a different model.", false), testCells("› ", false)))
	if err != nil || !found || got.Fingerprint == "" {
		t.Fatalf("found=%v error=%v", found, err)
	}
	// An unnamed opener whose closing tag is off screen is clipped.
	if _, found, err := DetectCapacity(collar, testScreen(testCells("› [gang#"+nonce+"] Continue", false), testCells("■ Selected model is at capacity. Please try a different model.", false), testCells("› ", false))); found || err == nil {
		t.Fatalf("clipped unnamed envelope: found=%v err=%v", found, err)
	}
}
