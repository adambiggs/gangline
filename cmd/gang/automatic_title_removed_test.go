//go:build acceptance

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func TestIdleNeverRenamesNativeSession(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		for _, freshHook := range []bool{false, true} {
			for _, legacy := range []string{"absent", "pending", "attempted"} {
				t.Run(fmt.Sprintf("%s/hooks=%t/title=%s", collar, freshHook, legacy), func(t *testing.T) {
					f := titleFixture(t, collar)
					f.setAgent(t, f.agent(t, "a"), func(a *core.Agent) {
						a.Native.SubmittedAt = f.cmd.now().Add(-time.Second)
						a.Native.FinishedAt = f.cmd.now()
						if freshHook {
							a.Native.HookAt, a.Native.HookKind = f.cmd.now(), "turn-finished"
						}
						if legacy != "absent" {
							a.NativeTitle = &core.NativeTitle{Text: "worker@team: A long title retained from an older installation", Attempted: legacy == "attempted"}
						}
					})
					p, _ := f.run.team.Agent("a")
					var titles int
					paint := f.input.onKeys
					f.input.onKeys = func(k substrate.Keys) error {
						if strings.Contains(k.Text, "/rename ") {
							titles++
						}
						return paint(k)
					}
					// Idle observation alone must not type anything, including an old pending title.
					for range 2 {
						run, err := f.cmd.runtime()
						if err != nil {
							t.Fatal(err)
						}
						if err := run.tickAgent("a", hookNotice{}, false); err != nil {
							t.Fatal(err)
						}
					}
					if f.input.pasted != "" || f.input.submits != 0 || titles != 0 {
						t.Fatalf("idle injected native input: pasted=%q submits=%d titles=%d", f.input.pasted, f.input.submits, titles)
					}
					e := core.Envelope{ID: "followup", Token: "0123456789abcdef", Recipient: "a", To: "worker", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "continue the work"}, CreatedAt: f.cmd.now()}
					if err := p.Publish(e); err != nil {
						t.Fatal(err)
					}
					clear := f.input.submit
					f.input.submit = func(text string) error {
						if err := clear(text); err != nil {
							return err
						}
						return p.WriteWitness(store.Witness{ID: "followup-submit", At: f.cmd.now(), Prompt: text, SessionID: "s"})
					}
					if err := f.run.tickAgent("a", hookNotice{}, false); err != nil {
						t.Fatal(err)
					}
					receipt, err := p.ReadEnvelope("cur", e.ID)
					if err != nil || receipt.Outcome != "delivered" || f.input.submits != 1 || titles != 0 {
						t.Fatalf("followup=%+v error=%v submits=%d titles=%d", receipt, err, f.input.submits, titles)
					}
					log, err := os.ReadFile(f.run.team.Log)
					if err != nil || strings.Contains(string(log), "automatic title:") {
						t.Fatalf("automatic title event: %s error=%v", log, err)
					}
				})
			}
		}
	}
}

func TestHitchDoesNotScheduleNativeTitle(t *testing.T) {
	f, _ := stoppedClaimFixture(t, "retired")
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "codex"))
	err := f.cmd.hitch([]string{"worker", "-c", "codex", "-t", "A long first task line that would previously become a generated native title\nFurther details"})
	if err == nil || !strings.Contains(err.Error(), "fixture-reached-new-session") {
		t.Fatalf("hitch = %v", err)
	}
	a, err := f.run.resolve("worker")
	if err != nil || a.NativeTitle != nil {
		t.Fatalf("hitch scheduled title: %+v error=%v", a.NativeTitle, err)
	}
}
