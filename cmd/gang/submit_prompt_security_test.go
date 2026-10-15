package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func TestSubmitRechecksPromptAfterComposerSettle(t *testing.T) {
	for _, path := range []string{"delivery", "compaction resume"} {
		for _, prompt := range []struct {
			name, collar string
			screen       substrate.Screen
		}{
			{"trust", "claude", screenWithText("Quick safety check: Is this a project you created or one you trust?", "❯ 1. Yes, I trust this folder", "  2. No, exit")},
			{"permission", "claude", screenWithText("Do you want to proceed?", "❯ 1. Yes", "  2. No")},
			{"approval", "codex", screenWithText("Would you like to run the following command?", "  1. Yes, proceed", "  2. No")},
		} {
			t.Run(path+"/"+prompt.name, func(t *testing.T) {
				f := newStateFixture(t)
				f.cmd.newTimeout = func(ctx context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
					return context.WithCancel(ctx)
				}
				a := f.add(t, "a", "worker", prompt.collar)
				f.input.command = prompt.collar
				f.input.screen = screenWithText("READY", "› ")
				if prompt.collar == "claude" {
					f.input.screen = screenWithText("────────", "❯ ", "────────")
				}
				if path == "compaction resume" {
					a.Compaction = &core.Compaction{
						ID: "c", Status: "submitted", StartedAt: f.cmd.now(),
						Resume:     core.Message{Text: "Continue from the saved state."},
						ResumeFrom: core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"},
					}
					active := "◦ Compacting context (0s • esc to interrupt)"
					if prompt.collar == "claude" {
						active = "✻ Compacting conversation… (0s)"
					}
					f.input.screen.Rows = append(screenWithText(active).Rows, f.input.screen.Rows...)
				}
				settles := 0
				f.input.onKeys = func(k substrate.Keys) error {
					if k.Text != "" {
						paste := strings.TrimSuffix(strings.TrimPrefix(k.Text, "\x1b[200~"), "\x1b[201~")
						f.input.screen = withComposerText(f.input.screen, paste)
					}
					return nil
				}
				f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
					settles++
					if f.input.pasted == "" {
						t.Fatal("settle ran before the paste")
					}
					// The native prompt replaces the composer just as its settle succeeds.
					f.input.screen = prompt.screen
					return nil
				}
				f.run.cmd = f.cmd
				p, err := f.run.team.Agent(a.ID)
				if err != nil {
					t.Fatal(err)
				}
				f.input.submit = func(wire string) error {
					return p.WriteWitness(store.Witness{ID: "submitted", At: f.cmd.now(), SessionID: "s", Prompt: wire})
				}
				l, err := p.TryLock()
				if err != nil {
					t.Fatal(err)
				}
				defer l.Close()
				if err := l.Save(a); err != nil {
					t.Fatal(err)
				}
				c, err := loadCollar(a.Collar, f.run.settings)
				if err != nil {
					t.Fatal(err)
				}
				id := core.EnvelopeID("resume-c")
				var submitErr error
				var outcome string
				var withdrawn bool
				if path == "delivery" {
					id = "message"
					e := core.Envelope{ID: id, Token: "aaaaaaaaaaaaaaaa", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "hello"}, CreatedAt: f.cmd.now()}
					if err := p.Publish(e); err != nil {
						t.Fatal(err)
					}
					outcome, withdrawn, submitErr = f.run.deliver(l, &a, e, f.input, c, "")
				} else {
					submitErr = f.run.queueCompactionResume(l, &a, f.input, c, "/compact")
				}
				if settles != 1 || f.input.submits != 0 || len(f.input.keys) != 0 {
					t.Fatalf("settles=%d Enter=%d other keys=%v", settles, f.input.submits, f.input.keys)
				}
				if path == "delivery" && (submitErr != nil || outcome != "unverified" || withdrawn) {
					t.Fatalf("delivery outcome=%s withdrawn=%t err=%v", outcome, withdrawn, submitErr)
				}
				if path == "compaction resume" && (submitErr == nil || !strings.Contains(submitErr.Error(), "native prompt needs attention after paste")) {
					t.Fatalf("continuation did not report the prompt: %v", submitErr)
				}
				receipt, err := p.ReadEnvelope("failed", id)
				if err != nil || receipt.Outcome != "unverified" || !strings.Contains(receipt.Reason, "native prompt needs attention after paste") {
					t.Fatalf("retained receipt=%+v err=%v", receipt, err)
				}
				got, err := p.Read()
				if err != nil || got.Input != nil || got.LastFailed != id || got.LastDelivered == id {
					t.Fatalf("input state=%+v err=%v", got, err)
				}
				if path == "compaction resume" && (got.Compaction.Resume != a.Compaction.Resume || got.Compaction.ResumeAdmitted || got.Compaction.Status != "submitted") {
					t.Fatalf("continuation state=%+v", got.Compaction)
				}
			})
		}
	}
}
