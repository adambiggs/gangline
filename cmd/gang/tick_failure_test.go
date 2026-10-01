package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func tickFailures(t *testing.T, f *stateFixture) []core.Event {
	t.Helper()
	data, err := os.Open(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()
	var failures []core.Event
	lines := bufio.NewScanner(data)
	lines.Buffer(nil, 1<<20)
	for lines.Scan() {
		e, err := core.DecodeEvent(lines.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if e.Type == "tick_failed" {
			failures = append(failures, e)
		}
	}
	if err := lines.Err(); err != nil {
		t.Fatal(err)
	}
	return failures
}

// A hook-triggered tick runs detached with its standard streams on the null
// device, so the team log is the only place its error can survive.
func TestDetachedTickFailureIsRecordedWithRecipient(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "missing-collar")
	notice, err := json.Marshal(hookNotice{Kind: "turn-finished", NativeEvent: "Stop", At: f.cmd.now()})
	if err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_BOUNDARY"] = string(notice)
	tickErr := f.cmd.tick([]string{"--agent", "a"})
	if tickErr == nil {
		t.Fatal("tick succeeded with an unavailable collar")
	}
	failures := tickFailures(t, f)
	if len(failures) != 1 {
		t.Fatalf("tick_failed records = %d, want 1: %+v", len(failures), failures)
	}
	got := failures[0]
	if got.HitchID != "a" || got.Name != "worker" || got.Source != "hook" {
		t.Fatalf("failure identity = %q/%q/%q, want a/worker/hook", got.HitchID, got.Name, got.Source)
	}
	if !strings.Contains(got.Reason, `"missing-collar"`) || !strings.Contains(tickErr.Error(), got.Reason) {
		t.Fatalf("failure reason %q does not carry tick error %q", got.Reason, tickErr)
	}
}

func TestTeamTickRecordsEachFailingRecipient(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	f.add(t, "b", "other", "missing-collar")
	if err := f.cmd.tick(nil); err == nil {
		t.Fatal("tick succeeded with an unavailable collar")
	}
	failures := tickFailures(t, f)
	if len(failures) != 1 {
		t.Fatalf("tick_failed records = %d, want 1: %+v", len(failures), failures)
	}
	if got := failures[0]; got.HitchID != "b" || got.Name != "other" || got.Source != "command" || !strings.Contains(got.Reason, `"missing-collar"`) {
		t.Fatalf("failure = %+v, want recipient b/other from a command tick", got)
	}
}

func TestUnreadableHookNoticeIsRecorded(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_BOUNDARY"] = "not json"
	if err := f.cmd.tick([]string{"--agent", "a"}); err == nil {
		t.Fatal("tick accepted an unreadable hook notice")
	}
	failures := tickFailures(t, f)
	if len(failures) != 1 || failures[0].HitchID != "a" || failures[0].Name != "worker" || failures[0].Source != "hook" || !strings.Contains(failures[0].Reason, "read hook notice") {
		t.Fatalf("failures = %+v, want one hook record for a/worker", failures)
	}
}
