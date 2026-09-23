package main

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestRetryMenuReportsBlockerAndPreservesSend(t *testing.T) {
	for _, liveOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "queue", true: "live-only"}[liveOnly], func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			// Reconstructed from the reported strings, not a captured full native screen.
			f.input.screen = screenWithText("Giving this request a little extra thought", "› 1. Retry with a faster model", "  2. Keep waiting")
			f.cmd.stdin = strings.NewReader("retain this message")
			args := []string{"worker", "--from", "operator"}
			if liveOnly {
				args = append(args, "--live-only")
			}
			err := f.cmd.send(args)
			if liveOnly {
				if err == nil || !strings.Contains(err.Error(), "Giving this request a little extra thought") {
					t.Fatalf("missing precise refusal: %v", err)
				}
			} else if err != nil || !strings.Contains(f.errOut.String(), "Giving this request a little extra thought") {
				t.Fatalf("missing blocker: err=%v stderr=%s", err, f.errOut)
			}
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Activity != core.Blocked || !strings.Contains(got.Evidence, "Retry with a faster model") || f.input.pasted != "" || f.input.submits != 0 {
				t.Fatalf("blocked state=%+v pasted=%q submits=%d", got, f.input.pasted, f.input.submits)
			}
			queued, err := p.ListNew()
			if err != nil {
				t.Fatal(err)
			}
			if liveOnly {
				if len(queued) != 0 {
					t.Fatal("live-only published a message")
				}
				return
			}
			if len(queued) != 1 || queued[0].Message.Text != "retain this message" {
				t.Fatalf("queue=%+v", queued)
			}
			f.input.screen = screenWithText("READY", "› ")
			f.input.submit = func(prompt string) error {
				return p.WriteWitness(store.Witness{ID: "accepted", At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
			}
			outcome, err := f.run.drain(a.ID, queued[0].ID)
			if err != nil || outcome != "delivered" || f.input.submits != 1 {
				t.Fatalf("resolved outcome=%s err=%v submits=%d", outcome, err, f.input.submits)
			}
		})
	}
}
