package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

// A native permission request reads blocked on a screen the collar cannot
// read until any later hook arrives or the screen reads idle. A screen that
// reads idle or busy shows the request was dismissed or answered, so it wins
// over the witness; only an idle one ends it, since a busy screen can precede
// the prompt's render.
func TestPermissionRequestHoldsBlockedUntilALaterHook(t *testing.T) {
	unreadable := screenWithText("unrecognized screen")
	idle := screenWithText("────────", "❯ ", "────────")
	busy := screenWithText("✻ Working… (3s · esc to interrupt)", "────────", "❯ ", "────────")
	for _, tc := range []struct {
		name     string
		hooks    []string
		before   []substrate.Screen
		screen   substrate.Screen
		activity core.Activity
		rule     string
	}{
		{name: "unreadable screen after a request", hooks: []string{"PermissionRequest"}, screen: unreadable, activity: core.Blocked, rule: "permission-request"},
		{name: "unreadable screen without a request", screen: unreadable, activity: core.Unknown, rule: "screen"},
		{name: "a later hook ends the request", hooks: []string{"PermissionRequest", "PostToolUse"}, screen: unreadable, activity: core.Unknown, rule: "screen"},
		{name: "idle screen after a request", hooks: []string{"PermissionRequest"}, screen: idle, activity: core.Idle, rule: "screen"},
		{name: "busy screen after a request", hooks: []string{"PermissionRequest"}, screen: busy, activity: core.Busy, rule: "screen"},
		{name: "an idle screen ends the request", hooks: []string{"PermissionRequest"}, before: []substrate.Screen{idle}, screen: unreadable, activity: core.Unknown, rule: "screen"},
		{name: "a busy screen keeps the request", hooks: []string{"PermissionRequest"}, before: []substrate.Screen{busy}, screen: unreadable, activity: core.Blocked, rule: "permission-request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "claude")
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.input.command = "claude"
			saveAgent(t, f, a.ID, func(a *core.Agent) { a.Activity, a.Evidence = core.Unknown, "seeded" })
			c, err := loadCollar("claude", f.run.settings)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := harness.Idle(c, unreadable); err == nil {
				t.Fatal("unreadable screen premise: the collar reads it")
			}
			for _, name := range tc.hooks {
				payload, err := json.Marshal(map[string]string{"hook_event_name": name, "session_id": "s"})
				if err != nil {
					t.Fatal(err)
				}
				f.cmd.stdin = strings.NewReader(string(payload))
				if err := f.cmd.handleHook(nil); err != nil {
					t.Fatal(err)
				}
			}
			for _, screen := range append(tc.before, tc.screen) {
				f.input.screen = screen
				if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
					t.Fatal(err)
				}
			}
			got := f.agent(t, a.ID)
			if got.Activity != tc.activity {
				t.Fatalf("activity = %s (%s), want %s", got.Activity, got.Evidence, tc.activity)
			}
			var basis *core.ActivityBasis
			data, err := os.ReadFile(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.ReadLog(strings.NewReader(string(data)), func(e core.Event) error {
				if e.Type == "activity_observed" {
					basis = e.Basis
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if basis == nil || basis.Rule != tc.rule {
				t.Fatalf("basis = %+v, want rule %s", basis, tc.rule)
			}
		})
	}
}

// Display state that cannot be kept fails the hook only after the hook has
// recorded its boundary.
func TestHookRecordsItsBoundaryWhenDisplayStateFails(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.detach = func(string, hookNotice) error { return nil }
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(p.Permission, "blocker"), 0o700); err != nil {
		t.Fatal(err)
	}
	f.cmd.stdin = strings.NewReader(`{"hook_event_name":"Stop","session_id":"s"}`)
	if err := f.cmd.handleHook(nil); err == nil {
		t.Fatal("hook succeeded with a permission witness it could not remove")
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"status":"turn-finished"`) {
		t.Fatalf("log lacks the turn-finished hook:\n%s", data)
	}
}
