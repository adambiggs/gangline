package harness

import (
	"github.com/adambiggs/gangline/substrate"
	"testing"
)

func TestCodexQueueAcceptsWrappedHeader(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	opener := "[gang:self-declared:acceptance#f88fbd8cf1bfb451]"
	screen := testScreen(
		testCells("• Messages to be submitted after next tool call (press esc to interrupt and send", false),
		testCells("  immediately)", false),
		testCells("  ↳ "+opener+" Reply with exactly FOURTH.", true),
		testCells("    Do not use tools.", true),
		testCells("", false),
		append(testCells("› ", false), testCells("Ask Codex to do anything", true)...),
	)
	accepted, err := NativeQueueAccepted(collar, screen, opener)
	if err != nil || !accepted {
		t.Fatalf("wrapped native queue = %t, %v; want accepted", accepted, err)
	}
}

func TestNativeInputPendingReadsCurrentQueue(t *testing.T) {
	c, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		lines   []string
		pending bool
		unknown bool
	}{
		{"empty", []string{"READY", "› "}, false, false},
		{"manual", []string{"• Queued follow-up inputs", "  ↳ manual input", "", "› "}, true, false},
		{"history", []string{"• Queued follow-up inputs", "old conversation output", "› "}, false, false},
		{"no composer", []string{"unreadable"}, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var rows [][]substrate.Cell
			for _, line := range tt.lines {
				rows = append(rows, testCells(line, false))
			}
			got, err := NativeInputPending(c, testScreen(rows...), "", "")
			if got != tt.pending || (err != nil) != tt.unknown {
				t.Fatalf("pending=%t err=%v", got, err)
			}
		})
	}
}
