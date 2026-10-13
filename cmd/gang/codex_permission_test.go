package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

// Both approval modes emit PermissionRequest with permission_mode=default.
// The pane distinguishes a running approved tool from an unanswered menu.
func TestCodexPermissionRequestRequiresVisibleMenu(t *testing.T) {
	for _, read := range []string{"status", "roster", "tick"} {
		for _, tc := range []struct {
			name   string
			screen substrate.Screen
			want   core.Activity
		}{
			{"approved tool running", screenWithText("• Running sleep 90; printf done", "", "• Working (23s • esc to interrupt)", "", "", "› ", "GPT-6-Astra high · Workspace · Approve for me"), core.Busy},
			{"visible permission menu", screenWithText("• Running sleep 90; printf done", "", "", "Would you like to run the following command?", "", "$ sleep 90; printf done", "", "› 1. Yes, proceed (y)", "2. Yes, and don't ask again (p)", "3. No, and tell Codex what to do differently (esc)", "Press enter to confirm or esc to cancel"), core.Blocked},
			{"request with unreadable pane", screenWithText("unrecognized screen"), core.Unknown},
			{"dismissed menu", screenWithText("› "), core.Idle},
		} {
			t.Run(read+"/"+tc.name, func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", "codex")
				f.env["GANGLINE_HITCH_ID"] = string(a.ID)
				hook := f.cmd
				hook.stdin = bytes.NewBufferString(`{"hook_event_name":"PermissionRequest","session_id":"s","turn_id":"t","permission_mode":"default","tool_name":"Bash","tool_input":{"command":"sleep 90; printf done"}}`)
				if err := hook.handleHook(nil); err != nil {
					t.Fatal(err)
				}
				f.input.screen = tc.screen
				now := f.cmd.now()
				f.cmd.clock = func() time.Time { return now }
				f.run.cmd.clock = f.cmd.clock
				// Check fresh hook handling and the retained request after expiry.
				for _, advance := range []time.Duration{0, statusHookFreshness + time.Second} {
					now = now.Add(advance)
					var err error
					switch read {
					case "status":
						err = f.cmd.status([]string{"worker", "--why"})
					case "roster":
						err = f.cmd.roster([]string{"--json"})
					case "tick":
						err = f.run.tickAgent(a.ID, hookNotice{}, false)
					}
					if read == "tick" && tc.want == core.Unknown {
						if !errors.Is(err, harness.ErrNoComposer) {
							t.Fatalf("unreadable pane error=%v, want no composer", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
					got := f.agent(t, a.ID)
					if got.Activity != tc.want {
						t.Fatalf("activity=%s (%s), want %s", got.Activity, got.Evidence, tc.want)
					}
					if strings.Contains(got.Evidence, "permission request awaits") {
						t.Fatalf("request alone reported an unanswered menu: %s", got.Evidence)
					}
				}
				if f.input.captures != 2 {
					t.Fatalf("captures=%d, want a current pane for each permission reading", f.input.captures)
				}
			})
		}
	}
}
