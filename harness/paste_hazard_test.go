package harness

import (
	"strings"
	"testing"
)

func TestPasteHazardRefusesPlaceholderShapesForClaude(t *testing.T) {
	claude := Invocation{Name: "claude-pasted-content"}
	for _, text := range []string{
		"[Pasted text #3]",
		"quote [Pasted text #12 +40 lines] in a reply",
		"line one\n[Pasted text #101 +2 lines]\nline three",
	} {
		reason, err := PasteHazard(claude, text)
		if err != nil || reason == "" {
			t.Errorf("%q: reason=%q err=%v", text, reason, err)
		}
		if strings.Contains(reason, "Pasted text #") {
			t.Errorf("reason quotes the token it refuses: %q", reason)
		}
	}
	for _, text := range []string{
		"Pasted text #3",
		"[Pasted text #]",
		"[Pasted text 3]",
		"[Pasted text #3",
	} {
		if reason, err := PasteHazard(claude, text); err != nil || reason != "" {
			t.Errorf("%q: reason=%q err=%v", text, reason, err)
		}
	}
}

func TestPasteHazardLeavesExactPromptCollarsAlone(t *testing.T) {
	if reason, err := PasteHazard(Invocation{Name: "exact-prompt"}, "[Pasted text #3]"); err != nil || reason != "" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if _, err := PasteHazard(Invocation{Name: "unknown"}, "text"); err == nil {
		t.Fatal("unknown submit witness accepted")
	}
}
