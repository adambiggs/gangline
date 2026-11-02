package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestListOutputBoundsRowsAndReportsOmissions(t *testing.T) {
	rows := strings.Split(strings.Repeat("entry\n", 65), "\n")[:65]
	for _, tc := range []struct {
		args  []string
		count int
	}{
		{nil, 50}, {[]string{"--limit", "3"}, 3}, {[]string{"--all"}, 65},
		{[]string{"--limit", "100"}, 65},
	} {
		page, _, err := parseListOptions("teams", tc.args, nil)
		if err != nil {
			t.Fatal(err)
		}
		var out, warn bytes.Buffer
		if err := page.writeLines(&out, &warn, rows); err != nil {
			t.Fatal(err)
		}
		if strings.Count(out.String(), "\n") != tc.count {
			t.Fatalf("%v: %d rows", tc.args, strings.Count(out.String(), "\n"))
		}
		if strings.Contains(warn.String(), "--all") != (tc.count < len(rows)) {
			t.Fatalf("%v: warning %q", tc.args, warn.String())
		}
		if err := page.writeLines(listFailedOutput{}, io.Discard, rows); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("output error = %v", err)
		}
	}
}

func TestListOutputRejectsInvalidLimitsBeforeReadingState(t *testing.T) {
	for _, args := range [][]string{{"--limit", "0"}, {"--limit", "-1"}, {"--limit", "bad"}, {"--limit", "3", "--all"}} {
		if _, _, err := parseListOptions("teams", args, nil); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	cmd := command{}
	for _, args := range [][]string{{"--all"}, {"--at", "1h", "--limit", "3"}, {"--clear", "--all"}} {
		if err := cmd.snooze(args); err == nil || !strings.Contains(err.Error(), "require --status") {
			t.Errorf("snooze %v: %v", args, err)
		}
	}
}

type listFailedOutput struct{}

func (listFailedOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
