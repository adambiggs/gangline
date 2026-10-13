package harness

import (
	"strings"
	"time"
)

// UsageWindowKind accepts only native labels or explicit native durations that
// identify the provider window. An unnamed primary/secondary bucket is unknown.
func UsageWindowKind(label string, minutes int) string {
	switch minutes {
	case 300:
		return "five_hour"
	case 10080:
		return "weekly"
	case 0:
	default:
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "five_hour", "current session", "5h limit", "5-hour", "5-hour limit":
		return "five_hour"
	case "seven_day", "current week", "weekly", "weekly limit", "7d limit":
		return "weekly"
	default:
		return ""
	}
}

// Pace compares a window's use with the share of the window that has elapsed.
// Delta is used minus elapsed, in percentage points: positive is ahead of the
// line that would exhaust the window exactly at its reset. Exhausts is when
// the window's average rate so far reaches 100%, and is zero when that comes
// at or after the reset.
type Pace struct {
	ElapsedPercent float64
	Delta          float64
	Exhausts       time.Time
}

// UsagePace needs the window's duration: native minutes, or the duration its
// label names. A window of unknown length has no pace.
func UsagePace(used float64, reset time.Time, label string, minutes int, now time.Time) (Pace, bool) {
	if minutes <= 0 {
		switch UsageWindowKind(label, 0) {
		case "five_hour":
			minutes = 300
		case "weekly":
			minutes = 10080
		default:
			return Pace{}, false
		}
	}
	duration := time.Duration(minutes) * time.Minute
	start := reset.Add(-duration)
	elapsed := min(max(now.Sub(start), 0), duration)
	p := Pace{ElapsedPercent: float64(elapsed) / float64(duration) * 100}
	p.Delta = used - p.ElapsedPercent
	switch {
	case used >= 100:
		p.Exhausts = now
	case used > 0 && elapsed > 0:
		// Compare in floating point first: a tiny use projects past any
		// representable duration.
		if span := float64(elapsed) * 100 / used; span < float64(duration) {
			p.Exhausts = start.Add(time.Duration(span))
		}
	}
	return p, true
}
