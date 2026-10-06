package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestCompactionResumeMatchesEnvelopeWithKeyboardSuffix(t *testing.T) {
	for _, altered := range []bool{false, true} {
		t.Run(map[bool]string{false: "trailing-byte", true: "altered-envelope"}[altered], func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "claude")
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			a.Native.SessionID = "s"
			a.Compaction = &core.Compaction{
				ID:          "c",
				Resume:      core.Message{Text: "Continue from the saved state."},
				ResumeToken: "aaaaaaaaaaaaaaaa",
				ResumeFrom:  core.Sender{Kind: core.SenderAgent, Name: "worker", HitchID: a.ID},
				StartedAt:   f.cmd.now().Add(-time.Minute), CompletedAt: f.cmd.now(),
				Status: "completed", Continuation: true,
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			e := core.Envelope{ID: "resume-c", Token: a.Compaction.ResumeToken, From: a.Compaction.ResumeFrom, Message: a.Compaction.Resume, Purpose: "resume", CreatedAt: f.cmd.now()}
			wire, err := envelopeText(e)
			if err != nil {
				t.Fatal(err)
			}
			// The queued receipt is unverified until the actual submit hook arrives.
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
			l, err = p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			a.Input = &core.InputIntent{ID: string(e.ID), Kind: "envelope", At: f.cmd.now()}
			if err := f.run.finishInput(l, &a, e, "unverified", "awaiting submit hook"); err != nil {
				t.Fatal(err)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			prompt := wire + "s"
			if altered {
				prompt = strings.Replace(prompt, "Continue from", "Changed from", 1)
			}
			payload, err := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": prompt, "session_id": "s"})
			if err != nil {
				t.Fatal(err)
			}
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.cmd.stdin = bytes.NewReader(payload)
			if err := f.cmd.handleHook(nil); err != nil {
				t.Fatal(err)
			}
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if !altered {
				if !got.Compaction.ResumeAdmitted || strings.Contains(f.out.String(), `"decision":"block"`) {
					t.Fatalf("envelope with keyboard suffix rejected: %+v %s", got.Compaction, f.out.String())
				}

				l, got, err = f.run.acquire(a.ID, false)
				if err != nil {
					t.Fatal(err)
				}
				if err := l.Close(); err != nil {
					t.Fatal(err)
				}
				receipt, err := p.ReadEnvelope("cur", e.ID)
				if err != nil || receipt.Outcome != "delivered" || receipt.Reason != `session keyboard input outside gang envelope: "s"` {
					t.Fatalf("delivery with suffix: %+v, %v", receipt, err)
				}
				log, err := os.Open(f.run.team.Log)
				if err != nil {
					t.Fatal(err)
				}
				defer log.Close()
				observed, delivered := false, false
				if err := store.ReadLog(log, func(event core.Event) error {
					if event.Reason == `session keyboard input outside gang envelope: "s"` {
						observed = observed || event.Status == "resume-admitted"
						delivered = delivered || event.Type == "delivery_succeeded"
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if !observed || !delivered {
					t.Fatalf("outside bytes missing from log: admission=%t delivery=%t", observed, delivered)
				}
			} else {
				if got.Compaction.ResumeAdmitted || got.Compaction.Status != "failed" || got.Compaction.Reason != "resume note blocked: stale or altered compaction continuation" || !strings.Contains(f.out.String(), `"decision":"block"`) {
					t.Fatalf("altered note result: %+v %s", got.Compaction, f.out.String())
				}
			}
		})
	}
}
