package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

// An idle agent whose last turn left native background tasks pending shows
// them as a qualifier on idle; the next turn end without pending tasks
// clears it.
func TestRosterQualifiesIdleWithPendingBackgroundTasks(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.command = "claude"
	idle := screenWithText("────────", "❯ ", "────────")
	busy := screenWithText("✻ Working… (3s · esc to interrupt)", "────────", "❯ ", "────────")
	f.input.screen = idle
	f.cmd.detach = func(string, hookNotice) error { return nil }
	hook := func(fields map[string]any) {
		t.Helper()
		payload, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		f.cmd.stdin = strings.NewReader(string(payload))
		if err := f.cmd.handleHook(nil); err != nil {
			t.Fatal(err)
		}
	}
	stop := func(tasks []map[string]string) {
		t.Helper()
		hook(map[string]any{"hook_event_name": "Stop", "session_id": "s", "background_tasks": tasks})
	}
	setActivity := func(activity core.Activity) {
		t.Helper()
		saveAgent(t, f, a.ID, func(a *core.Agent) { a.Activity = activity })
		if f.input.screen = idle; activity == core.Busy {
			f.input.screen = busy
		}
	}
	read := func(args ...string) string {
		t.Helper()
		f.out.Reset()
		if err := f.cmd.execute(args); err != nil {
			t.Fatal(err)
		}
		return f.out.String()
	}

	stop([]map[string]string{
		{"id": "b1", "type": "shell", "status": "running", "description": "monitor"},
		{"id": "w1", "type": "workflow", "status": "running", "description": "workflow"},
	})
	f.out.Reset()
	if err := f.cmd.execute([]string{"status", "worker", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got agentJSON
	decodeOutput(t, f, &got)
	if got.Activity != core.Idle || got.BackgroundTasks != 2 {
		t.Fatalf("status = %+v", got)
	}
	if out := read("roster"); !strings.Contains(out, "idle (2 bg)") {
		t.Fatalf("roster = %q", out)
	}
	if out := read("status", "worker"); !strings.Contains(out, "\tidle (2 bg)\n") {
		t.Fatalf("status = %q", out)
	}

	setActivity(core.Busy)
	if out := read("status", "worker"); !strings.Contains(out, "\tbusy\n") {
		t.Fatalf("status while busy = %q", out)
	}
	setActivity(core.Idle)
	if out := read("status", "worker"); !strings.Contains(out, "\tidle (2 bg)\n") {
		t.Fatalf("status idle again = %q", out)
	}

	hook(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt": "next"})
	setActivity(core.Idle)
	if out := read("status", "worker"); !strings.Contains(out, "\tidle\n") {
		t.Fatalf("status after a turn started = %q", out)
	}

	stop([]map[string]string{{"id": "b1", "type": "shell", "status": "running", "description": "monitor"}})
	stop([]map[string]string{})
	if out := read("roster"); strings.Contains(out, " bg)") || !strings.Contains(out, "idle") {
		t.Fatalf("roster after an empty stop = %q", out)
	}
}
