package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

const refusalMessage = "This content was flagged for possible cybersecurity risk."

// codexTurnErrorFixture is a Codex agent whose submit hook witnessed turn
// witnessed and whose session log ends turn failed with an error of class.
// Codex fires no Stop hook for the failed turn, so the log is its only
// boundary, and the pane reads idle.
func codexTurnErrorFixture(t *testing.T, witnessed, failed string, class any, message string) (*stateFixture, core.Agent) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	at := f.cmd.now()
	if err := p.WriteWitness(store.Witness{ID: "w1", At: at, SessionID: "s", TurnID: witnessed}); err != nil {
		t.Fatal(err)
	}
	stamp := at.UTC().Format("2006-01-02T15:04:05.000Z")
	path := filepath.Join(f.env["GANG_STATE_ROOT"], "rollout.jsonl")
	errorClass, err := json.Marshal(class)
	if err != nil {
		t.Fatal(err)
	}
	appendLines(t, path,
		`{"type":"session_meta","payload":{"id":"s"}}`,
		fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"task_started","turn_id":%q}}`, stamp, failed),
		fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"task_complete","turn_id":%q,"last_agent_message":null,"error":{"message":%q,"codex_error_info":%s}}}`, stamp, failed, message, errorClass),
	)
	a = f.setAgent(t, a, func(a *core.Agent) {
		a.Native.SessionID, a.Native.TurnID, a.Native.Transcript, a.Native.SubmittedAt = "s", witnessed, path, at
	})
	// The pane has read idle past the submit's paint window.
	f.cmd.clock = func() time.Time { return at.Add(time.Minute) }
	f.run.cmd = f.cmd
	return f, a
}

// A turn Codex refused reads blocked with the provider's reason in roster,
// status --why and the log, never idle, whether the roster reads the pane or
// the submit hook is still fresh.
func TestCodexRefusedTurnReadsBlocked(t *testing.T) {
	for _, freshHook := range []bool{false, true} {
		t.Run(fmt.Sprintf("fresh hook %v", freshHook), func(t *testing.T) {
			f, a := codexTurnErrorFixture(t, "t1", "t1", "cyber_policy", refusalMessage)
			if freshHook {
				a = f.setAgent(t, a, func(a *core.Agent) {
					a.Native.HookAt, a.Native.HookKind = f.cmd.now(), "turn-started"
				})
			}
			advice := "; the agent takes the next message: rebrief it, and replace it only if the rebrief is refused too"
			want := "native turn refused: cyber_policy: " + refusalMessage + advice
			if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
				t.Fatal(err)
			}
			if out := f.out.String(); !strings.Contains(out, "\tblocked\n") || !strings.Contains(out, want) {
				t.Fatalf("status --why: %q", out)
			}
			if got := f.agent(t, a.ID); got.Native.FailedTurn != "t1" || got.Native.FinishedAt.IsZero() {
				t.Fatalf("refused turn left open: failed %q finished %s", got.Native.FailedTurn, got.Native.FinishedAt)
			}
			f.out.Reset()
			if err := f.cmd.roster(nil); err != nil {
				t.Fatal(err)
			}
			if row := strings.TrimSpace(f.out.String()); !strings.Contains(row, " blocked ") || !strings.Contains(row, "native turn refused: cyber_policy: ") || !strings.HasSuffix(row, "brief it, and replace it only if the rebrief is refused too") {
				t.Fatalf("roster row: %q", row)
			}
			f.out.Reset()
			if err := f.cmd.log(nil); err != nil {
				t.Fatal(err)
			}
			if out := f.out.String(); !strings.Contains(out, want) {
				t.Fatalf("log lacks the refusal: %s", out)
			}
		})
	}
}

// Any other Codex turn error reads unknown with its reason, as a Claude
// StopFailure does, and a failure of a turn the submit hook no longer names
// describes nothing current.
func TestCodexTurnErrorAttribution(t *testing.T) {
	for _, tc := range []struct {
		name, witnessed, class string
		want                   core.Activity
		evidence               string
	}{
		{"capacity", "t1", "server_overloaded", core.Unknown, "native turn failed: server_overloaded: Selected model is at capacity."},
		{"older turn", "t2", "cyber_policy", core.Idle, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := refusalMessage
			if tc.class == "server_overloaded" {
				message = "Selected model is at capacity."
			}
			f, a := codexTurnErrorFixture(t, tc.witnessed, "t1", tc.class, message)
			if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
				t.Fatal(err)
			}
			got := f.agent(t, a.ID)
			if got.Activity != tc.want || got.Evidence != tc.evidence {
				t.Fatalf("activity = %s %q, want %s %q", got.Activity, got.Evidence, tc.want, tc.evidence)
			}
		})
	}
}

// The next prompt the submit hook witnesses ends the refusal's reading.
func TestCodexRefusalClearsOnNextSubmit(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "cyber_policy", refusalMessage)
	if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, a.ID); got.Activity != core.Blocked {
		t.Fatalf("refusal premise: %s %q", got.Activity, got.Evidence)
	}
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WriteWitness(store.Witness{ID: "w2", At: f.cmd.now(), SessionID: "s", TurnID: "t2"}); err != nil {
		t.Fatal(err)
	}
	later := f.cmd.now().Add(time.Minute)
	f.cmd.clock = func() time.Time { return later }
	f.run.cmd = f.cmd
	if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, a.ID); got.Native.TurnFailure != "" || got.Activity == core.Blocked {
		t.Fatalf("refusal outlived the next submit: %s %q", got.Activity, got.Evidence)
	}
}

// One agent's unreadable session log leaves the roster readable for the team.
func TestRosterSurvivesUnreadableSessionLog(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "cyber_policy", refusalMessage)
	f.setAgent(t, f.add(t, "b", "other", "codex"), func(b *core.Agent) {
		b.Native.SessionID, b.Native.Transcript = "s2", filepath.Join(f.env["GANG_STATE_ROOT"], "absent.jsonl")
	})
	if err := f.cmd.roster(nil); err != nil {
		t.Fatalf("roster failed for the team: %v", err)
	}
	out := f.out.String()
	if !strings.Contains(out, "worker ") || !strings.Contains(out, "other ") {
		t.Fatalf("roster rows missing: %q", out)
	}
	if got := f.agent(t, a.ID); got.Activity != core.Blocked {
		t.Fatalf("readable agent lost its refusal: %s %q", got.Activity, got.Evidence)
	}
}

func TestCodexProviderErrorVisibleWithoutFailureHook(t *testing.T) {
	for _, status := range []int{429, 503} {
		for _, read := range []string{"status", "roster", "tick"} {
			for _, probed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/probed=%v", status, read, probed), func(t *testing.T) {
					message := "exceeded retry limit, last status: 429 Too Many Requests"
					if status == 503 {
						message = "exceeded retry limit, last status: 503 Service Unavailable"
					}
					class := map[string]any{"response_too_many_failed_attempts": map[string]int{"http_status_code": status}}
					f, a := codexTurnErrorFixture(t, "t1", "t1", class, message)
					f.setAgent(t, a, func(a *core.Agent) {
						a.Native.HookAt, a.Native.HookKind = f.cmd.now().Add(-time.Second), "turn-started"
						if probed {
							a.StatusProbeAt = f.cmd.now()
							a.Activity, a.Evidence = core.Idle, ""
						}
					})
					var err error
					switch read {
					case "status":
						err = f.cmd.status([]string{"worker", "--why"})
					case "roster":
						err = f.cmd.roster(nil)
					case "tick":
						err = f.run.tickAgent(a.ID, hookNotice{}, false)
					}
					if err != nil {
						t.Fatal(err)
					}
					got := f.agent(t, a.ID)
					if got.Activity != core.Unknown || got.Evidence != "native turn failed: "+message {
						t.Fatalf("provider error invisible: activity=%s evidence=%q", got.Activity, got.Evidence)
					}
					if read != "tick" && !strings.Contains(f.out.String(), message) {
						t.Fatalf("output omits provider error: %q", f.out.String())
					}
				})
			}
		}
	}
}

func TestStartupRecoveryDoesNotReplayProviderFailedTurn(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", map[string]any{"response_too_many_failed_attempts": map[string]int{"http_status_code": 429}}, "exceeded retry limit, last status: 429 Too Many Requests")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: "original", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Purpose: "assignment", Message: core.Message{Text: "Do the assignment."}, Startup: startupSections("worker", startupProse{Contract: []byte("Report completion.")}), CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&a, e, "delivered", "native submit witness"); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
		t.Fatal(err)
	}
	f.out.Reset()
	if err := f.cmd.hitch([]string{"worker", "--recover"}); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 || !strings.Contains(f.out.String(), "delivered") {
		t.Fatalf("recovery replayed delivered assignment: submits=%d output=%q", f.input.submits, f.out.String())
	}
	if got := f.agent(t, a.ID); got.Native.TurnFailure == "" {
		t.Fatal("delivery recovery erased the execution failure")
	}
}
