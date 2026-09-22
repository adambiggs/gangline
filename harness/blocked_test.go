package harness

import (
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestEmbeddedCollarsDetectRuntimeApprovalSurfaces(t *testing.T) {
	tests := []struct {
		collar string
		lines  []string
	}{
		{collar: "codex", lines: []string{"Hooks need review", "› 1. Review hooks", "2. Trust all and continue", "Press enter to confirm or esc to go back"}},
		{collar: "claude-code", lines: []string{"Bash command", "Do you want to proceed?", "❯ 1. Yes", "  2. No"}},
		{collar: "codex", lines: []string{"Would you like to run the following command?", "› 1. Yes, proceed", "  2. No"}},
	}
	for _, test := range tests {
		t.Run(test.collar, func(t *testing.T) {
			collar, err := EmbeddedCollar(test.collar)
			if err != nil {
				t.Fatal(err)
			}
			rows := make([][]substrate.Cell, len(test.lines))
			for index, line := range test.lines {
				rows[index] = testCells(line, false)
			}
			blocked, found, err := DetectBlocked(collar.Primitives.Blocked, testScreen(rows...))
			if err != nil || !found || blocked.Evidence == "" {
				t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
			}
		})
	}
}

func TestBlockedPrimitiveRequiresPromptAndChoice(t *testing.T) {
	invocation := Invocation{Name: "screen-blocked", Params: map[string]string{"prompt": "approve", "choice": "yes"}}
	_, found, err := DetectBlocked(invocation, testScreen(testCells("approve this command", false)))
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("prompt without a choice was detected as blocked")
	}
}
