package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestLogPrintsRecentMatchesByDefaultAndAllOnlyWhenRequested(t *testing.T) {
	var input strings.Builder
	for i := 0; i < 65; i++ {
		fmt.Fprintln(&input, strings.TrimSuffix(filterTestEvent, "}")+fmt.Sprintf(`,"reason":"event-%d"}`, i))
	}
	// Unselected records at the end must not consume the limit.
	for i := 0; i < 10; i++ {
		fmt.Fprintln(&input, strings.ReplaceAll(filterTestEvent, `"worker"`, `"other"`))
	}
	for _, tc := range []struct {
		name         string
		args         []string
		first, count int
	}{
		{"default", nil, 15, 50},
		{"one", []string{"--limit", "1"}, 64, 1},
		{"larger", []string{"--limit", "100"}, 0, 65},
		{"all", []string{"--all"}, 0, 65},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filter, _, err := parseLogFilter(append([]string{"--agent", "worker"}, tc.args...), true)
			if err != nil {
				t.Fatal(err)
			}
			var out, warn bytes.Buffer
			if err := writeFilteredLog(&out, &warn, strings.NewReader(input.String()), filter, func(s string) string { return s }); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(out.String()), "\n")
			if len(lines) != tc.count {
				t.Fatalf("printed %d events, want %d", len(lines), tc.count)
			}
			for i, line := range lines {
				var event struct{ Reason string }
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					t.Fatal(err)
				}
				if want := fmt.Sprintf("event-%d", tc.first+i); event.Reason != want {
					t.Fatalf("event %d: %q, want %q", i, event.Reason, want)
				}
			}
			if truncated := tc.count < 65; strings.Contains(warn.String(), "--all") != truncated {
				t.Fatalf("truncation notice = %q", warn.String())
			}
		})
	}
}

func TestLogRejectsInvalidLimits(t *testing.T) {
	for _, args := range [][]string{{"--limit", "0"}, {"--limit", "-1"}, {"--limit", "lots"}, {"--limit", "1", "--all"}} {
		if _, _, err := parseLogFilter(args, true); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
