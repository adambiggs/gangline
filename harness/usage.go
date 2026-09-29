package harness

import "strings"

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
