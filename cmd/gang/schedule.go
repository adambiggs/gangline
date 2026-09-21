package main

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var durationPartPattern = regexp.MustCompile(`([0-9]+)([hms])`)
var clockPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

func parseSchedule(value string, now time.Time) (time.Time, error) {
	if match := clockPattern.FindStringSubmatch(value); match != nil {
		hour, _ := strconv.Atoi(match[1])
		minute, _ := strconv.Atoi(match[2])
		result := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if !result.After(now) {
			result = result.AddDate(0, 0, 1)
		}
		return result, nil
	}
	duration, err := parseDuration(value)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(duration), nil
}

func parseDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("duration is empty")
	}
	remaining := value
	var duration time.Duration
	for remaining != "" {
		match := durationPartPattern.FindStringSubmatchIndex(remaining)
		if match == nil || match[0] != 0 {
			return 0, fmt.Errorf("invalid duration %q; use values such as 2h30m or 45s", value)
		}
		amount, err := strconv.ParseUint(remaining[match[2]:match[3]], 10, 63)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", value)
		}
		var unit time.Duration
		switch remaining[match[4]:match[5]] {
		case "h":
			unit = time.Hour
		case "m":
			unit = time.Minute
		case "s":
			unit = time.Second
		}
		if amount > uint64((time.Duration(1<<63-1)-duration)/unit) {
			return 0, fmt.Errorf("duration %q is too large", value)
		}
		duration += time.Duration(amount) * unit
		remaining = remaining[match[1]:]
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return duration, nil
}
