package main

import (
	"bytes"
	"encoding/json"
	"github.com/adambiggs/gangline/core"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHookStatusSurvivesSkippedRefreshWithoutCapture(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		for _, tc := range []struct{ event, title string }{
			{"UserPromptSubmit", "-worker-"}, {"PostToolUse", "-worker-"},
			{"Stop", "~worker~"}, {"PermissionRequest", "!worker!"}, {"PreCompact", "-worker-"}, {"PostCompact", "~worker~"},
		} {
			t.Run(collar+"/"+tc.event, func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", collar)
				f.env["GANGLINE_HITCH_ID"] = string(a.ID)
				f.cmd.detach = func(string, hookNotice) error { return nil }
				payload, _ := json.Marshal(map[string]string{"hook_event_name": tc.event, "session_id": "s", "turn_id": "t", "prompt": "work"})
				hook := f.cmd
				hook.stdin = bytes.NewReader(payload)
				if err := hook.handleHook(nil); err != nil {
					t.Fatal(err)
				}
				// No detached handler runs. The ordinary sweep must recover the record.
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
					t.Fatal(err)
				}
				if got := f.agent(t, a.ID); paneTitle(got) != tc.title {
					t.Errorf("title=%q want %q, native=%+v", paneTitle(got), tc.title, got.Native)
				}
				if f.input.captures != 0 {
					t.Errorf("fresh hook caused %d captures", f.input.captures)
				}
			})
		}
	}
}

func TestHookStatusExpiredEvidenceCaptures(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.detach = func(string, hookNotice) error { return nil }
	hook := f.cmd
	hook.stdin = bytes.NewBufferString(`{"hook_event_name":"PostToolUse","session_id":"s"}`)
	if err := hook.handleHook(nil); err != nil {
		t.Fatal(err)
	}
	now := f.cmd.now().Add(31 * time.Second)
	f.cmd.clock = func() time.Time { return now }
	f.run.cmd.clock = f.cmd.clock
	f.input.screen = screenWithText("› ")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.captures != 1 {
		t.Fatalf("captures=%d", f.input.captures)
	}
	if got := f.agent(t, a.ID); got.Activity != core.Idle {
		t.Fatalf("activity=%s", got.Activity)
	}
}

func TestHookStatusFallbackCadenceAndLog(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.detach = func(string, hookNotice) error { return nil }
	hook := f.cmd
	hook.stdin = bytes.NewBufferString(`{"hook_event_name":"PostToolUse","session_id":"s"}`)
	if err := hook.handleHook(nil); err != nil {
		t.Fatal(err)
	}
	now := f.cmd.now()
	f.run.cmd.clock = func() time.Time { return now }
	f.input.screen = screenWithText("› ")
	for _, step := range []struct {
		advance  time.Duration
		captures int
		title    string
	}{
		{statusHookFreshness - time.Nanosecond, 0, "-worker-"},
		{time.Nanosecond, 1, "~worker~"},
		{statusHookFreshness - time.Nanosecond, 1, "~worker~"},
		{time.Nanosecond, 2, "~worker~"},
	} {
		now = now.Add(step.advance)
		if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
			t.Fatal(err)
		}
		if f.input.captures != step.captures {
			t.Fatalf("at %s: captures=%d want %d", now, f.input.captures, step.captures)
		}
		if got := paneTitle(f.agent(t, a.ID)); got != step.title {
			t.Fatalf("title=%s want %s", got, step.title)
		}
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "status pane capture: hook evidence expired") != 2 {
		t.Fatalf("fallback reasons missing: %s", data)
	}
}

func TestHookStatusLateFinishDoesNotCloseNewTurn(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "new", "prompt": "work"})
	late := call(map[string]string{"hook_event_name": "Stop", "session_id": "s", "prompt_id": "old"})
	for i := 0; i < 2; i++ {
		if err := f.run.tickAgent(a.ID, late, true); err != nil {
			t.Fatal(err)
		}
		got := f.agent(t, a.ID)
		if paneTitle(got) != "-worker-" || !got.Native.SubmittedAt.After(got.Native.FinishedAt) {
			t.Fatalf("old finish closed new turn: %+v", got.Native)
		}
	}
	if f.input.captures != 0 {
		t.Fatalf("captures=%d", f.input.captures)
	}
}

func TestHookStatusFailureNeedsNoPane(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "t", "prompt": "work"})
	call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "t", "error": "Login expired"})
	for i := 0; i < 2; i++ {
		if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
			t.Fatal(err)
		}
		if got := f.agent(t, a.ID); paneTitle(got) != "!worker!" || !strings.Contains(got.Evidence, "Login expired") {
			t.Fatalf("failure=%+v", got)
		}
	}
	if f.input.captures != 0 {
		t.Fatalf("captures=%d", f.input.captures)
	}
}

func TestHookStatusDoesNotSubmitOverDraft(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.cmd.detach = func(string, hookNotice) error { return nil }
			hook := f.cmd
			hook.stdin = bytes.NewBufferString(`{"hook_event_name":"Stop","session_id":"s"}`)
			if err := hook.handleHook(nil); err != nil {
				t.Fatal(err)
			}
			f.input.command = collar
			f.input.screen = screenWithText("› operator draft")
			if collar == "claude" {
				f.input.screen = screenWithText("────────", "❯ operator draft", "────────")
			}
			p, _ := f.run.team.Agent(a.ID)
			if err := p.Publish(core.Envelope{ID: "msg", Token: "token", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "fixture"}, Message: core.Message{Text: "queued work"}, CreatedAt: f.cmd.now()}); err != nil {
				t.Fatal(err)
			}
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 0 || f.input.pasted != "" {
				t.Fatalf("draft overwritten: %+v", f.input)
			}
			if got := paneTitle(f.agent(t, a.ID)); got != "~worker~" {
				t.Fatalf("title=%s", got)
			}
		})
	}
}

func TestHookStatusReadersUseRetainedEvidence(t *testing.T) {
	for _, read := range []string{"status", "roster"} {
		t.Run(read, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.cmd.detach = func(string, hookNotice) error { return nil }
			hook := f.cmd
			hook.stdin = bytes.NewBufferString(`{"hook_event_name":"PostToolUse","session_id":"s"}`)
			if err := hook.handleHook(nil); err != nil {
				t.Fatal(err)
			}
			var err error
			if read == "status" {
				err = f.cmd.status([]string{"worker"})
			} else {
				err = f.cmd.roster(nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			if f.input.captures != 0 {
				t.Fatalf("captures=%d", f.input.captures)
			}
			if got := paneTitle(f.agent(t, a.ID)); got != "-worker-" {
				t.Fatalf("title=%s", got)
			}
		})
	}
}
