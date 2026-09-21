package main

import (
	"testing"
	"time"
)

func TestParseScheduleSupportsDurationAndNextLocalClock(t *testing.T) {
	location := time.FixedZone("test", -7*60*60)
	now := time.Date(2026, 9, 21, 17, 30, 0, 0, location)

	got, err := parseSchedule("2h15m", now)
	if err != nil {
		t.Fatal(err)
	}
	if want := now.Add(2*time.Hour + 15*time.Minute); !got.Equal(want) {
		t.Fatalf("duration schedule = %s, want %s", got, want)
	}

	got, err = parseSchedule("08:00", now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 9, 22, 8, 0, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("clock schedule = %s, want %s", got, want)
	}
}

func TestParseScheduleRejectsPartialAndZeroDurations(t *testing.T) {
	for _, value := range []string{"", "1h30", "0s", "25:00", "-1h"} {
		if _, err := parseSchedule(value, time.Now()); err == nil {
			t.Fatalf("schedule %q passed", value)
		}
	}
}
