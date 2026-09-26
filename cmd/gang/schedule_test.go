package main

import (
	"testing"
	"time"
)

func TestScheduleAcceptsExactDeadlineAndGoDurations(t *testing.T) {
	now := time.Date(2026, time.September, 26, 12, 0, 0, 0, time.FixedZone("local", -7*60*60))
	deadline := "2026-09-27T09:30:15+02:00"
	got, err := parseSchedule(deadline, now)
	if err != nil || got.Format(time.RFC3339) != deadline {
		t.Fatalf("deadline = %s, %v", got, err)
	}
	for _, value := range []string{"1h30m", "1.5h", "500ms"} {
		got, err := parseSchedule(value, now)
		duration, parseErr := time.ParseDuration(value)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if err != nil || !got.Equal(now.Add(duration)) {
			t.Fatalf("schedule %q = %s, %v", value, got, err)
		}
	}
	got, err = parseSchedule("12:00", now)
	if err != nil || !got.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("local clock = %s, %v", got, err)
	}
}

func TestScheduleRejectsNonPositiveAndInvalidDurations(t *testing.T) {
	now := time.Date(2026, time.September, 26, 12, 0, 0, 0, time.UTC)
	for _, value := range []string{"", "0", "-1s", "1w", "25:99"} {
		if _, err := parseSchedule(value, now); err == nil {
			t.Errorf("schedule %q passed", value)
		}
	}
}
