package harness

import (
	"testing"
	"time"
)

func TestReadClaudeProviderLimits(t *testing.T) {
	now := time.Date(2026, time.September, 21, 10, 0, 0, 0, time.UTC)
	limits, err := ReadProviderLimits(
		Invocation{Name: "claude-screen-limits"},
		testScreen(testRow("Current session: 23% used · resets Sep 21, 5pm (UTC)", false)),
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if limits[0].UsedPercent != 23 || limits[0].ResetAt.Hour() != 17 {
		t.Fatalf("limit = %+v", limits[0])
	}
}

func TestReadCodexProviderLimitsConvertsRemaining(t *testing.T) {
	now := time.Date(2026, time.September, 21, 10, 0, 0, 0, time.UTC)
	limits, err := ReadProviderLimits(
		Invocation{Name: "codex-screen-limits"},
		testScreen(testRow("5h limit: 70% left · resets 3:00 PM", false)),
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if limits[0].UsedPercent != 30 {
		t.Fatalf("used = %d, want 30", limits[0].UsedPercent)
	}
}
