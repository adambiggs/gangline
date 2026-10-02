package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/adambiggs/gangline/core"
)

// A table reason keeps the head and the end of the evidence within its limit;
// the whole evidence stays in the JSON row.
func TestRosterReasonFitsItsRow(t *testing.T) {
	exit := "native process exited with status 1: " + strings.Repeat("starting\n", 80) + "error: unknown model"
	short := "boot deadline elapsed"
	f := newStateFixture(t)
	f.setAgent(t, f.add(t, "a", "long", "codex"), func(a *core.Agent) {
		a.Status, a.Activity, a.Evidence = core.Failed, core.Unknown, exit
	})
	f.setAgent(t, f.add(t, "b", "short", "codex"), func(a *core.Agent) {
		a.Status, a.Activity, a.Evidence = core.Failed, core.Unknown, short
	})
	rows := rosterRows(t, f)
	_, reason, _ := strings.Cut(rows["long"], "codex"+rosterReasonGap)
	if utf8.RuneCountInString(reason) != rosterReasonWidth || !strings.HasPrefix(reason, "native process exited with status 1: sta…") || !strings.HasSuffix(reason, "starting error: unknown model") {
		t.Fatalf("long reason = %q (%d characters), want the exit's status and last output in %d", reason, utf8.RuneCountInString(reason), rosterReasonWidth)
	}
	if !strings.HasSuffix(rows["short"], "codex"+rosterReasonGap+short) {
		t.Fatalf("short row = %q, want its whole reason", rows["short"])
	}
	f.out.Reset()
	if err := f.cmd.execute([]string{"status", "long", "--json"}); err != nil {
		t.Fatal(err)
	}
	var status agentJSON
	decodeOutput(t, f, &status)
	if status.Evidence != exit {
		t.Fatalf("JSON evidence holds %d bytes of %d", len(status.Evidence), len(exit))
	}
	row := strings.Repeat("r", 50)
	for _, tc := range []struct{ width, want int }{
		{160, 160 - 50 - len(rosterReasonGap)},
		{100, 100 - 50 - len(rosterReasonGap)},
		{50 + len(rosterReasonGap) + rosterReasonHead + 1, rosterReasonHead + 1},
		{80, rosterReasonHead + 1},
		{0, rosterReasonHead + 1},
	} {
		if got := rosterReasonRoom(tc.width, row); got != tc.want {
			t.Errorf("rosterReasonRoom(%d) = %d, want %d", tc.width, got, tc.want)
		}
	}
	if got, want := rosterReasonRoom(100, strings.Repeat("✓", 50)), 100-50-len(rosterReasonGap); got != want {
		t.Errorf("rosterReasonRoom counts %d for a row of 50 characters, want %d", got, want)
	}
	for _, tc := range []struct {
		evidence string
		limit    int
		want     string
	}{
		{"exactly ten", 11, rosterReasonGap + "exactly ten"},
		{"exactly ten!", 11, rosterReasonGap + "exactly te…"},
		{"wide ✓✓✓✓ cut", 8, rosterReasonGap + "wide ✓✓…"},
		{strings.Repeat("h", rosterReasonHead) + "dropped" + "✓end", rosterReasonHead + 5, rosterReasonGap + strings.Repeat("h", rosterReasonHead) + "…✓end"},
		{strings.Repeat("h", rosterReasonHead) + "dropped", rosterReasonHead + 1, rosterReasonGap + strings.Repeat("h", rosterReasonHead) + "…"},
	} {
		if got := rosterReason(core.Agent{Status: core.Failed, Evidence: tc.evidence}, tc.limit); got != tc.want {
			t.Errorf("rosterReason(%q, %d) = %q, want %q", tc.evidence, tc.limit, got, tc.want)
		}
	}
}
