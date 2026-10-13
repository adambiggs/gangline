package harness

import (
	"testing"
	"time"
)

func TestUsagePaceArithmetic(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour
	for _, tc := range []struct {
		name     string
		used     float64
		reset    time.Time
		label    string
		minutes  int
		elapsed  float64
		delta    float64
		exhausts time.Time
	}{
		// Half the week gone, 75% used: the average rate reaches 100% at two
		// thirds of the week.
		{"ahead", 75, now.Add(week / 2), "seven_day", 0, 50, 25, now.Add(-week / 2).Add(week * 2 / 3)},
		{"behind", 25, now.Add(week / 2), "weekly", 10080, 50, -25, time.Time{}},
		{"on the line exhausts at reset, not before", 50, now.Add(week / 2), "seven_day", 0, 50, 0, time.Time{}},
		{"native minutes override the label", 30, now.Add(time.Hour), "codex/primary", 300, 80, -50, time.Time{}},
		{"window just opened", 10, now.Add(5 * time.Hour), "five_hour", 0, 0, 10, time.Time{}},
		{"capped", 100, now.Add(time.Hour), "five_hour", 0, 80, 20, now},
		{"reset already passed clamps elapsed", 40, now.Add(-time.Minute), "five_hour", 0, 100, -60, time.Time{}},
		{"reset beyond the window clamps elapsed", 5, now.Add(10 * time.Hour), "five_hour", 0, 0, 5, time.Time{}},
		// A tiny use projects past any representable duration.
		{"tiny use never overflows", 1e-12, now.Add(week / 2), "seven_day", 0, 50, -50, time.Time{}},
		{"unused never exhausts", 0, now.Add(week / 2), "seven_day", 0, 50, -50, time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := UsagePace(tc.used, tc.reset, tc.label, tc.minutes, now)
			if !ok {
				t.Fatal("pace unknown")
			}
			if !near(p.ElapsedPercent, tc.elapsed) || !near(p.Delta, tc.delta) {
				t.Fatalf("elapsed %v delta %v, want %v and %v", p.ElapsedPercent, p.Delta, tc.elapsed, tc.delta)
			}
			if p.Exhausts.Sub(tc.exhausts).Abs() > time.Second || p.Exhausts.IsZero() != tc.exhausts.IsZero() {
				t.Fatalf("exhausts %v, want %v", p.Exhausts, tc.exhausts)
			}
		})
	}
}

func TestUsagePaceNeedsAWindowLength(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	for _, label := range []string{"spend_limit", "codex/primary", "seven_day/fable"} {
		if p, ok := UsagePace(50, now.Add(time.Hour), label, 0, now); ok {
			t.Fatalf("%s has pace %+v without a known duration", label, p)
		}
	}
}

func near(a, b float64) bool { return a-b < 1e-6 && b-a < 1e-6 }
