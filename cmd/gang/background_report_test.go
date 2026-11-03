//go:build acceptance

package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// A delayed or unorderable positive report is evidence that a hook reported
// tasks, not proof those tasks are still pending or when the agent went idle.
func TestBackgroundReportDoesNotAssertCurrentPendingWork(t *testing.T) {
	for _, prompt := range []string{"old", "unorderable", "current"} {
		t.Run(prompt, func(t *testing.T) {
			f := newBackgroundIdleFixture(t, "claude")
			f.input.command = "claude"
			f.input.screen = screenWithText("────────", "❯ ", "────────")
			at := f.cmd.now()
			f.cmd.clock = func() time.Time { return at }
			f.run.settings.BackgroundIdleNotice = time.Minute
			p, err := f.run.team.Agent(f.worker.ID)
			if err != nil {
				t.Fatal(err)
			}
			call := func(event, turn string, tasks []string) {
				t.Helper()
				at = at.Add(time.Second)
				data, err := json.Marshal(map[string]any{"hook_event_name": event, "session_id": "s", "prompt_id": turn, "prompt": "work", "transcript_path": f.transcript, "background_tasks": tasks})
				if err != nil {
					t.Fatal(err)
				}
				f.cmd.stdin = strings.NewReader(string(data))
				if err := f.cmd.handleHook(nil); err != nil {
					t.Fatal(err)
				}
			}
			call("UserPromptSubmit", "old", nil)
			call("Stop", "old", nil)
			call("UserPromptSubmit", "current", nil)
			call("Stop", "current", []string{})
			if _, present, err := p.ReadBackground(); err != nil || present {
				t.Fatalf("current empty report: present=%v err=%v", present, err)
			}
			if err := p.WritePermissionWitness(store.PermissionWitness{At: at, SessionID: "s"}); err != nil {
				t.Fatal(err)
			}
			call("Stop", prompt, []string{"reported task"})
			if _, present, err := p.ReadPermissionWitness(); err != nil || present {
				t.Fatalf("ordinary terminal permission effect withheld: present=%v err=%v", present, err)
			}
			f.woken = nil // Count notification wakes separately from hook reconciliation.
			f.tickAt(t, at.Add(f.run.settings.BackgroundIdleNotice))
			notices := f.notices(t)
			if len(notices) != 1 || len(f.woken) != 1 || len(f.events(t)) != 1 {
				t.Fatalf("report warning lost: notices=%+v wakes=%v events=%+v", notices, f.woken, f.events(t))
			}
			notice := notices[0]
			t.Logf("automatic warning: %s", notice.Message.Text)
			if strings.Contains(notice.Message.Text, "has been idle since") || strings.Contains(notice.Message.Text, "background task pending") {
				t.Errorf("report presented as current idle/pending truth: %s", notice.Message.Text)
			}
			for _, want := range []string{"is idle", "background report received at " + at.UTC().Format(time.RFC3339), "listed 1 background task", "current task state is unknown"} {
				if !strings.Contains(notice.Message.Text, want) {
					t.Errorf("notice missing %q: %s", want, notice.Message.Text)
				}
			}
			reason := f.events(t)[0].Reason
			for _, want := range []string{"background report received at " + at.UTC().Format(time.RFC3339), "listed 1 background task", "current task state is unknown"} {
				if !strings.Contains(reason, want) {
					t.Errorf("audit reason missing %q: %s", want, reason)
				}
			}
			f.out.Reset()
			f.cmd.clock = func() time.Time { return at.Add(2 * time.Minute) }
			if err := f.cmd.execute([]string{"status", "worker", "--why"}); err != nil {
				t.Fatal(err)
			}
			t.Logf("status --why: %s", f.out.String())
			if strings.Contains(f.out.String(), "idle since") || strings.Contains(f.out.String(), "background task pending") {
				t.Errorf("status presents unverified report as current work: %s", f.out.String())
			}
			for _, want := range []string{"background report received at " + at.UTC().Format(time.RFC3339), "(2m ago): 1 background task", "current task state is unknown"} {
				if !strings.Contains(f.out.String(), want) {
					t.Errorf("status missing %q: %s", want, f.out.String())
				}
			}
			// Delivery/receipt changes and repeated observations do not repeat the warning.
			f.settle(t, notice)
			f.settle(t, core.Envelope{ID: "unrelated-lead-receipt", Recipient: f.lead.ID, To: f.lead.Name, From: core.Sender{Kind: core.SenderGangline, Name: "test"}, Message: core.Message{Text: "other"}, CreatedAt: at})
			f.tickAt(t, at.Add(10*f.run.settings.BackgroundIdleNotice))
			if len(f.woken) != 1 || len(f.events(t)) != 1 || len(f.notices(t)) != 0 {
				t.Fatalf("warning repeated after lead receipt: wakes=%v events=%+v notices=%+v", f.woken, f.events(t), f.notices(t))
			}
		})
	}
}
