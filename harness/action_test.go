package harness

import "testing"

func TestRenderCompactAction(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	action, err := RenderAction(collar.Actions.Compact, map[string]string{"instructions": "keep the decision"})
	if err != nil {
		t.Fatal(err)
	}
	if action.Text != "/compact keep the decision" || !action.Submit {
		t.Fatalf("action = %+v", action)
	}
}

func TestSubmitSettle(t *testing.T) {
	settle, err := SubmitSettle(Invocation{Name: "enter-submit", Params: map[string]string{"settle": "400ms"}})
	if err != nil {
		t.Fatal(err)
	}
	if settle.String() != "400ms" {
		t.Fatalf("settle = %s", settle)
	}
	if _, err := SubmitSettle(Invocation{Name: "enter-submit", Params: map[string]string{"settle": "later"}}); err == nil {
		t.Fatal("invalid settle duration was accepted")
	}
}

func TestSubmitInputUsesBracketedPaste(t *testing.T) {
	input, err := SubmitInput(Invocation{Name: "enter-submit", Params: map[string]string{"paste": "bracketed"}}, "hello\nworld")
	if err != nil {
		t.Fatal(err)
	}
	if input.Text != "\x1b[200~hello\nworld\x1b[201~" {
		t.Fatalf("text = %q", input.Text)
	}
}

func TestRenderActionPreservesBracesInUserValue(t *testing.T) {
	action, err := RenderAction(Action{Text: "/compact {{instructions}}"}, map[string]string{
		"instructions": "keep {{literal}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if action.Text != "/compact keep {{literal}}" {
		t.Fatalf("text = %q", action.Text)
	}
}

func TestSubmitBuildsAtomicTextAndEnter(t *testing.T) {
	action, err := Submit(Invocation{Name: "enter-submit"}, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if action.Text != "hello" || !action.Submit {
		t.Fatalf("action = %+v", action)
	}
}

func TestActionConvertsToSubstrateKeys(t *testing.T) {
	keys := (Action{Keys: []string{"Escape", "Enter"}}).Input()
	if len(keys.Names) != 2 || keys.Names[0] != "Escape" || keys.Names[1] != "Enter" {
		t.Fatalf("keys = %+v", keys)
	}
}

func TestClaudeCompactionActiveMatchesOnlyTheLiveSpinner(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	if active, err := CompactionActive(collar, fixtureScreen(t, "claude-code-2.1.287-compacting.txt")); err != nil || !active {
		t.Fatalf("compacting screen: active=%v err=%v", active, err)
	}
	// Claude Code writes a duration past a minute in minutes and seconds.
	for _, line := range []string{"✻ Compacting conversation… (0s)", "✻ Compacting conversation… (1m 5s · ↓ 4.1k tokens)"} {
		if active, err := CompactionActive(collar, testScreen(testCells(line, false))); err != nil || !active {
			t.Fatalf("%q: active=%v err=%v", line, active, err)
		}
	}
	for _, line := range []string{
		"  ✽ Compacting conversation… (4s)",
		"⏺ ✽ Compacting conversation… (4s)",
		"✽ Compacting conversation… was quoted here",
		"✻ Conversation compacted (ctrl+o for history)",
	} {
		screen := testScreen(testCells(line, false), testCells("❯ ", false))
		if active, err := CompactionActive(collar, screen); err != nil || active {
			t.Fatalf("%q: active=%v err=%v", line, active, err)
		}
	}
	codex, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	if active, err := CompactionActive(codex, fixtureScreen(t, "claude-code-2.1.287-compacting.txt")); err != nil || active {
		t.Fatalf("other compaction spinner: active=%v err=%v", active, err)
	}
	collar.Actions.Compact.Active = "("
	if err := validateCollar(collar); err == nil {
		t.Fatal("invalid compact active pattern accepted")
	}
}

func TestCodexCompactionActiveRequiresTheCompactionSpinner(t *testing.T) {
	c, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	if active, err := CompactionActive(c, fixtureScreen(t, "codex-compacting.txt")); err != nil || !active {
		t.Fatalf("captured compaction: active=%v err=%v", active, err)
	}
	if active, err := CompactionActive(c, testScreen(testCells("◦ Compacting context (1m 5s • esc to interrupt)", false))); err != nil || !active {
		t.Fatalf("longer compaction: active=%v err=%v", active, err)
	}
	for _, line := range []string{
		"Working (12s • esc to interrupt)",
		"• Context compacted · 23s",
		"› ◦ Compacting context (13s • esc to interrupt)",
		"  ◦ Compacting context was quoted here",
	} {
		if active, err := CompactionActive(c, testScreen(testCells(line, false))); err != nil || active {
			t.Fatalf("ordinary screen %q: active=%v err=%v", line, active, err)
		}
	}
}
