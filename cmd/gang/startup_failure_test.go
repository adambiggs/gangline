//go:build acceptance

package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func deliveredStartup(t *testing.T, f *stateFixture, a core.Agent) (core.Agent, string) {
	t.Helper()
	e := core.Envelope{ID: "startup-original", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "startup"}, Purpose: "startup", Message: core.Message{Text: "Do the assignment."}, Startup: startupSections(string(a.Name), startupProse{Contract: []byte("Report completion.")}), CreatedAt: a.CreatedAt}
	p, _ := f.run.team.Agent(a.ID)
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
	wire, err := envelopeText(e)
	if err != nil {
		t.Fatal(err)
	}
	return a, wire
}

func TestStartupFailureNotifiesHitcher(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, fresh := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s fresh=%v", provider, fresh), func(t *testing.T) {
				var f *stateFixture
				var a core.Agent
				if provider == "codex" {
					f, a = codexTurnErrorFixture(t, "t1", "t1", "model_not_found", "unsupported model")
				} else {
					f = newStateFixture(t)
					a = f.add(t, "a", "worker", "claude")
				}
				lead := f.addHitched(t, "l", "lead", "lead", "")
				a = f.setAgent(t, a, func(a *core.Agent) { a.HitchedBy = lead.ID })
				a, wire := deliveredStartup(t, f, a)
				p, _ := f.run.team.Agent(a.ID)
				if err := p.WriteWitness(store.Witness{ID: "startup-witness", At: a.CreatedAt, SessionID: "s", TurnID: "t1", Prompt: wire, Transcript: a.Native.Transcript}); err != nil {
					t.Fatal(err)
				}
				f.setAgent(t, a, func(a *core.Agent) {
					a.Native.SessionID, a.Native.TurnID = "s", "t1"
					if provider == "claude" {
						a.Native.TurnFailure, a.Native.FailedTurn = "model_not_found: unsupported model", "t1"
					}
					if fresh {
						a.Native.HookKind, a.Native.HookAt = "activity", f.run.cmd.now()
					}
				})
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
					t.Fatal(err)
				}
				if got := f.agent(t, a.ID); got.Activity != core.Blocked || !strings.Contains(got.Evidence, "native startup turn failed: model_not_found") {
					t.Fatalf("startup failure=%+v", got)
				}
				lp, _ := f.run.team.Agent(lead.ID)
				notices, err := lp.ListNew()
				if err != nil || len(notices) != 1 || notices[0].From.Kind != core.SenderGangline || notices[0].From.Name != "hitch" || !strings.Contains(notices[0].Message.Text, "unsupported model") {
					t.Fatalf("notices=%+v err=%v", notices, err)
				}
				l, err := lp.TryLock()
				if err != nil {
					t.Fatal(err)
				}
				if err := l.Withdraw(notices[0].ID); err != nil {
					t.Fatal(err)
				}
				if err := l.Close(); err != nil {
					t.Fatal(err)
				}
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
					t.Fatal(err)
				}
				if notices, err := lp.ListNew(); err != nil || len(notices) != 0 {
					t.Fatalf("repeat notices=%+v err=%v", notices, err)
				}
				f.out.Reset()
				if err := f.cmd.status([]string{"worker"}); err != nil {
					t.Fatal(err)
				}
				if out := f.out.String(); !strings.Contains(out, "active\tblocked") || !strings.Contains(out, "model_not_found") {
					t.Fatalf("plain status=%q", out)
				}
			})
		}
	}
}

func TestLaterFailedTurnDoesNotFailStartup(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "model_not_found", "unsupported model")
	lead := f.addHitched(t, "l", "lead", "lead", "")
	a = f.setAgent(t, a, func(a *core.Agent) { a.HitchedBy = lead.ID })
	deliveredStartup(t, f, a)
	// The receipt is still startup, but this witness belongs to a later prompt.
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	lp, _ := f.run.team.Agent(lead.ID)
	if notices, err := lp.ListNew(); err != nil || len(notices) != 0 {
		t.Fatalf("notices=%+v err=%v", notices, err)
	}
	if got := f.agent(t, a.ID); got.Activity != core.Unknown {
		t.Fatalf("later failure=%s", got.Activity)
	}
}

func TestStartupCapacityFailureStillRecovers(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "server_overloaded", capacityError)
	lead := f.addHitched(t, "l", "lead", "lead", "")
	a = f.setAgent(t, a, func(a *core.Agent) { a.HitchedBy = lead.ID })
	a, wire := deliveredStartup(t, f, a)
	p, _ := f.run.team.Agent(a.ID)
	if err := p.WriteWitness(store.Witness{ID: "startup-witness", At: a.CreatedAt, SessionID: "s", TurnID: "t1", Prompt: wire, Transcript: a.Native.Transcript}); err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.screen = screenWithText("• Ran a tool", "", "■ "+capacityError, "", "› ")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got := f.agent(t, a.ID)
	if got.Activity != core.Blocked || got.Capacity.Fingerprint == "" {
		t.Fatalf("startup capacity: %s %+v", got.Activity, got.Capacity)
	}
	lp, _ := f.run.team.Agent(lead.ID)
	if notices, err := lp.ListNew(); err != nil || len(notices) != 1 {
		t.Fatalf("notices=%+v err=%v", notices, err)
	}
	f.run.cmd.clock = func() time.Time { return got.Capacity.NextAt }
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 {
		t.Fatalf("capacity continuation submits=%d", f.input.submits)
	}
}
