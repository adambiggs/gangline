package main

import (
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestSnoozeLeadAuthorityRequiresRole(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		hitchedBy  core.HitchID
		allowed    bool
	}{
		{"lead", "", "", false},
		{"lead", "worker", "", false},
		{"lead", "lead", "worker-id", false},
		{"child-coordinator", "lead", "worker-id", false},
		{"coordinator", "lead", "", true},
	} {
		t.Run(tc.name+"/"+tc.role, func(t *testing.T) {
			f := newStateFixture(t)
			caller := f.add(t, "caller", tc.name, "codex")
			saveAgent(t, f, caller.ID, func(a *core.Agent) {
				a.Role, a.HitchedBy = tc.role, tc.hitchedBy
			})
			f.env["GANGLINE_HITCH_ID"] = string(caller.ID)
			f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(caller.ID), caller.Pane
			if err := f.run.withUsageState(func(state *usageState) error {
				state.Notices = []usageNotice{{ID: "uncertain-notice", RecipientID: caller.ID, Submission: "unverified"}}
				state.Snoozes["gone"] = usageSnooze{ID: "queued-wake", CallerID: "gone", CallerName: "gone", RecipientID: caller.ID, At: f.cmd.now()}
				state.Recent["left"] = usageSnooze{ID: "recent-wake", CallerID: "left", CallerName: "left", RecipientID: caller.ID, At: f.cmd.now()}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := f.cmd.snooze([]string{"--status"}); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"uncertain-notice", "queued-wake", "recent-wake"} {
				if got := strings.Contains(f.out.String(), id); got != tc.allowed {
					t.Errorf("status contains %s = %v, want %v; output %q", id, got, tc.allowed, f.out.String())
				}
				if err := f.cmd.snooze([]string{"--clear", id}); (err == nil) != tc.allowed {
					t.Errorf("clear %s error = %v; allowed = %v", id, err, tc.allowed)
				}
			}
			state := usageSnapshot(t, f.run)
			if cleared := len(state.Notices) == 0 && len(state.Snoozes) == 0 && len(state.Recent) == 0; cleared != tc.allowed {
				t.Fatalf("usage intents cleared = %v, want %v; state %+v", cleared, tc.allowed, state)
			}
			if !tc.allowed && (len(state.Notices) != 1 || state.Snoozes["gone"].ID != "queued-wake" || state.Recent["left"].ID != "recent-wake") {
				t.Fatalf("refused clear changed usage intents: %+v", state)
			}
		})
	}
}

func TestUsageRoutingRequiresLeadRole(t *testing.T) {
	for _, tc := range []struct {
		name                string
		haveLead, childLead bool
	}{
		{"name-only", false, false},
		{"renamed-lead", true, false},
		{"child-only", false, true},
		{"operator-lead-with-child-lead", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			impostor := f.add(t, "name-only", "lead", "codex")
			recipient := impostor
			var want core.HitchID
			if tc.childLead {
				recipient = f.add(t, "child-lead", "child-coordinator", "codex")
				saveAgent(t, f, recipient.ID, func(a *core.Agent) {
					a.Role, a.HitchedBy = "lead", "worker-id"
				})
			}
			if tc.haveLead {
				recipient = f.add(t, "role-lead", "coordinator", "codex")
				saveAgent(t, f, recipient.ID, func(a *core.Agent) { a.Role = "lead" })
				want = recipient.ID
			}
			// Keep publication pending so the durable recipient can be inspected.
			p, err := f.run.team.Agent(recipient.ID)
			if err != nil {
				t.Fatal(err)
			}
			lock, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Close()
			at := f.cmd.now()
			if err := f.run.withUsageState(func(state *usageState) error {
				state.Notices = []usageNotice{{ID: "usage-notice", Token: "notice-token", ResetAt: at.Add(time.Hour).Unix()}}
				state.Snoozes["gone"] = usageSnooze{ID: "orphan-wake", Token: "wake-token", CallerID: "gone", CallerName: "gone", At: at.Add(-time.Minute)}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := f.run.flushUsageWork(); err != nil {
				t.Fatal(err)
			}
			state := usageSnapshot(t, f.run)
			if len(state.Notices) != 1 || state.Notices[0].RecipientID != want {
				t.Errorf("notice recipient = %+v, want %q", state.Notices, want)
			}
			if got := state.Snoozes["gone"].RecipientID; got != want {
				t.Errorf("orphan wake recipient = %q, want %q", got, want)
			}
			if f.input.submits != 0 {
				t.Fatalf("pending work submitted %d times", f.input.submits)
			}
		})
	}
}
