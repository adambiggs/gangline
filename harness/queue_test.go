package harness

import "testing"

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
