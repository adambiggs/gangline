package harness

import (
	"github.com/adambiggs/gangline/substrate"
	"testing"
)

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
		{"permission", []string{"Would you like to run", "Yes, proceed", "› do the work", failure, "", "› "}, false, false},
		{"busy", []string{"› do the work", failure, "esc to interrupt", "› "}, false, false},
		{"typed composer", []string{"› do the work", failure, "", "› draft"}, false, false},
		{"clipped originating prompt", []string{failure, "", "› "}, false, true},
		{"collapsed originating prompt", []string{"› [Pasted Content 3 chars]", failure, "", "› "}, false, true},
		{"unrelated provider error", []string{"› do the work", "■ Quota exceeded.", "", "› "}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rows [][]substrate.Cell
			for _, line := range tc.lines {
				rows = append(rows, testCells(line, false))
			}
			got, found, err := DetectCapacity(collar, testScreen(rows...))
			if found != tc.found || (err != nil) != tc.unknown {
				t.Fatalf("found=%v capacity=%+v error=%v", found, got, err)
			}
		})
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
