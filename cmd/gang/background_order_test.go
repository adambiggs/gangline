//go:build acceptance

package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestLateBackgroundStopPreservesCurrentRosterCount(t *testing.T) {
	for _, tc := range []struct {
		name  string
		event map[string]any
	}{
		{"old zero", map[string]any{"hook_event_name": "Stop", "session_id": "s", "prompt_id": "old", "background_tasks": []map[string]string{}}},
		{"old positive", map[string]any{"hook_event_name": "Stop", "session_id": "s", "prompt_id": "old", "background_tasks": []map[string]string{{"id": "old1"}, {"id": "old2"}}}},
		{"old failure", map[string]any{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "old", "error": "old failure"}},
		{"missing prompt identity", map[string]any{"hook_event_name": "Stop", "session_id": "s", "background_tasks": []map[string]string{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "claude")
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.input.command = "claude"
			f.input.screen = screenWithText("────────", "❯ ", "────────")
			f.cmd.detach = func(string, hookNotice) error { return nil }
			ordered := `{"type":"user","promptId":"old","timestamp":"2026-09-22T09:00:00Z"}` + "\n" + `{"type":"user","promptId":"new","timestamp":"2026-09-22T09:00:01Z"}` + "\n"
			if err := os.WriteFile(f.transcript, []byte(ordered), 0600); err != nil {
				t.Fatal(err)
			}
			at := f.cmd.now()
			f.cmd.clock = func() time.Time { return at }
			call := func(fields map[string]any) {
				t.Helper()
				at = at.Add(time.Second)
				fields["transcript_path"] = f.transcript
				data, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				f.cmd.stdin = strings.NewReader(string(data))
				if err := f.cmd.handleHook(nil); err != nil {
					t.Fatal(err)
				}
			}
			read := func() agentJSON {
				t.Helper()
				f.out.Reset()
				if err := f.cmd.execute([]string{"status", "worker", "--json"}); err != nil {
					t.Fatal(err)
				}
				var got agentJSON
				decodeOutput(t, f, &got)
				return got
			}
			call(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "new", "prompt": "work", "transcript_path": f.transcript})
			call(map[string]any{"hook_event_name": "Stop", "session_id": "s", "prompt_id": "new", "background_tasks": []map[string]string{{"id": "shell-new", "type": "shell"}}})
			if got := read(); got.Activity != core.Idle || got.BackgroundTasks != 1 {
				t.Fatalf("current stop: %+v", got)
			}
			call(tc.event)
			if got := read(); got.Activity != core.Idle || got.BackgroundTasks != 1 {
				t.Fatalf("late old zero erased current background qualifier: %+v", got)
			}
			call(map[string]any{"hook_event_name": "Stop", "session_id": "s", "prompt_id": "new", "background_tasks": []map[string]string{}})
			if got := read(); got.BackgroundTasks != 0 {
				t.Fatalf("current zero did not clear the qualifier: %+v", got)
			}
		})
	}
}

func TestBackgroundCurrentCallbackWithUnavailableOrder(t *testing.T) {
	for _, mode := range []string{"lag", "out-of-window", "malformed", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "claude")
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.input.command = "claude"
			f.input.screen = screenWithText("────────", "❯ ", "────────")
			f.cmd.detach = func(string, hookNotice) error { return nil }
			at := f.cmd.now()
			f.cmd.clock = func() time.Time { return at }
			call := func(event, turn string, tasks []string) error {
				t.Helper()
				at = at.Add(time.Second)
				data, err := json.Marshal(map[string]any{"hook_event_name": event, "session_id": "s", "prompt_id": turn, "prompt": "work", "transcript_path": f.transcript, "background_tasks": tasks})
				if err != nil {
					t.Fatal(err)
				}
				f.cmd.stdin = strings.NewReader(string(data))
				return f.cmd.handleHook(nil)
			}
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := call("UserPromptSubmit", "submitted", nil); err != nil {
				t.Fatal(err)
			}
			if err := call("Stop", "submitted", []string{"old background"}); err != nil {
				t.Fatal(err)
			}
			if err := p.WritePermissionWitness(store.PermissionWitness{At: at, SessionID: "s"}); err != nil {
				t.Fatal(err)
			}
			first := `{"type":"user","promptId":"submitted","timestamp":"2026-09-22T09:00:00Z"}` + "\n"
			next := `{"type":"user","promptId":"queued","timestamp":"2026-09-22T09:00:01Z"}` + "\n"
			contents := first
			switch mode {
			case "out-of-window":
				contents += strings.Repeat("{}\n", (4<<20)/3+2) + next
			case "malformed":
				contents += "{bad json}\n" + next
			case "unreadable":
				if err := os.Remove(f.transcript); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "unreadable" {
				if err := os.WriteFile(f.transcript, []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
			}
			wantErr := mode == "malformed" || mode == "unreadable"
			for _, tasks := range [][]string{{"new one", "new two"}, {}} {
				err := call("Stop", "queued", tasks)
				if (err != nil) != wantErr {
					t.Fatalf("hook error=%v, want error=%v", err, wantErr)
				}
				background, _, err := p.ReadBackground()
				if err != nil || background.Tasks != len(tasks) {
					t.Fatalf("current display was frozen: %+v %v", background, err)
				}
				if _, present, err := p.ReadPermissionWitness(); err != nil || present {
					t.Fatalf("permission retained: present=%v err=%v", present, err)
				}
			}
			if err := call("UserPromptSubmit", "fresh", nil); err != nil {
				t.Fatal(err)
			}
			if err := call("Stop", "fresh", []string{"current"}); err != nil {
				t.Fatal(err)
			}
			background, _, err := p.ReadBackground()
			if err != nil || background.Tasks != 1 {
				t.Fatalf("next current terminal: %+v %v", background, err)
			}
		})
	}
}

func TestBackgroundSnapshotsOrderQueuedTurns(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.detach = func(string, hookNotice) error { return nil }
	data := `{"type":"user","promptId":"submitted","timestamp":"2026-11-03T00:00:00Z"}` + "\n" + `{"type":"user","promptId":"queued","timestamp":"2026-11-03T00:00:01Z"}` + "\n"
	if err := os.WriteFile(f.transcript, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	call := func(event, turn string, tasks []string) {
		t.Helper()
		data, err := json.Marshal(map[string]any{"hook_event_name": event, "session_id": "s", "prompt_id": turn, "transcript_path": f.transcript, "prompt": "work", "background_tasks": tasks})
		if err != nil {
			t.Fatal(err)
		}
		f.cmd.stdin = strings.NewReader(string(data))
		if err := f.cmd.handleHook(nil); err != nil {
			t.Fatal(err)
		}
	}
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	check := func(want int) {
		t.Helper()
		got, _, err := p.ReadBackground()
		if err != nil || got.Tasks != want {
			t.Fatalf("background=%+v err=%v, want %d", got, err, want)
		}
	}
	call("UserPromptSubmit", "submitted", nil)
	call("Stop", "queued", []string{"current background"})
	check(1)
	call("Stop", "submitted", []string{})
	check(1)
	call("StopFailure", "submitted", nil)
	check(1)
	call("Stop", "queued", []string{})
	check(0)
	call("Stop", "queued", []string{"current background"})
	check(1)
	call("StopFailure", "queued", nil)
	check(0)
}
