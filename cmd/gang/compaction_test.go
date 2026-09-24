package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestCompactionQueuesContinuationAfterCompletion(t *testing.T) {
	for _, collar := range []string{"codex", "claude-code"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			p, _ := f.run.team.Agent(a.ID)
			if collar == "claude-code" {
				f.input.command = "claude"
				f.input.screen = screenWithText("────────", "❯ ", "────────")
			}
			f.cmd.newWatch = func(path string) (changeWait, error) {
				if path != p.Witness {
					t.Fatalf("unexpected watch: %s", path)
				}
				return waitFixture{func(context.Context) error {
					t.Fatal("submission witness was already published")
					return nil
				}}, nil
			}
			f.run.cmd = f.cmd
			f.input.submit = func(prompt string) error {
				if strings.HasPrefix(prompt, "/compact") {
					return nil
				}
				a, err := p.Read()
				if err != nil {
					return err
				}
				if a.Compaction.Status != "completed" || a.Input == nil || a.Input.ID != "resume-"+a.Compaction.ID || a.Native.Transcript != "" {
					t.Fatalf("continuation intent: %+v", a)
				}
				return p.WriteWitness(store.Witness{ID: "resume-witness", At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
			}
			var ce commandError
			if err := f.cmd.compact([]string{"worker", "--resume", "continue the work"}); !errors.As(err, &ce) || ce.status != exitUnknown {
				t.Fatalf("submission: %v", err)
			}
			if f.input.submits != 1 {
				t.Fatalf("resume preceded completion: submits=%d", f.input.submits)
			}
			if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: f.cmd.now().Add(time.Second)}, false); err != nil {
				t.Fatal(err)
			}
			a, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			e, err := p.ReadEnvelope("cur", core.EnvelopeID("resume-"+a.Compaction.ID))
			if err != nil || e.Outcome != "delivered" || a.Input != nil || f.input.submits != 2 {
				t.Fatalf("continuation: %+v input=%+v submits=%d err=%v", e, a.Input, f.input.submits, err)
			}
			for _, at := range []time.Time{f.cmd.now().Add(-time.Second), f.cmd.now().Add(time.Second), f.cmd.now().Add(time.Second)} {
				if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: at}, false); err != nil {
					t.Fatal(err)
				}
			}
			if f.input.submits != 2 {
				t.Fatalf("completion hooks repeated input: %d", f.input.submits)
			}
		})
	}
}

func TestCompactionContinuationUsesNormalDelivery(t *testing.T) {
	for _, outcome := range []string{"queued", "unverified"} {
		t.Run(outcome, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			f.input.submit = func(prompt string) error {
				if strings.HasPrefix(prompt, "/compact") {
					if outcome == "queued" {
						f.input.screen = screenWithText("› unfinished draft")
					}
					return nil
				}
				if outcome == "unverified" {
					prompt = "different message"
				}
				return p.WriteWitness(store.Witness{ID: "resume-witness", At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
			}
			err := f.cmd.compact([]string{"worker"})
			var ce commandError
			if !errors.As(err, &ce) || ce.status != exitUnknown {
				t.Fatalf("submission: %v", err)
			}
			// The resume below is delivered, so the report must not call it withheld.
			if !strings.HasSuffix(ce.text, "resume follows confirmed completion") {
				t.Fatalf("submission report: %q", ce.text)
			}
			if f.input.submits != 1 {
				t.Fatal("resume preceded completion")
			}
			if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: f.cmd.now().Add(time.Second)}, false); err != nil {
				t.Fatal(err)
			}

			a, err = p.Read()
			if err != nil {
				t.Fatal(err)
			}
			id := core.EnvelopeID("resume-" + a.Compaction.ID)
			if outcome == "queued" {
				e, err := p.ReadEnvelope("new", id)
				if err != nil || f.input.submits != 1 || e.From.Kind != core.SenderGangline {
					t.Fatalf("occupied composer: submits=%d from=%+v err=%v", f.input.submits, e.From, err)
				}
			} else {
				e, err := p.ReadEnvelope("failed", id)
				if err != nil || e.Outcome != "unverified" || a.Input != nil {
					t.Fatalf("failed continuation: %+v input=%+v err=%v", e, a.Input, err)
				}
			}
			f.input.screen = screenWithText("› ")
			for range 2 {
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
					t.Fatal(err)
				}
			}
			if f.input.submits != 2 {
				t.Fatalf("continuation submissions=%d", f.input.submits)
			}
		})
	}
}

func TestCompactionSubmissionRecovery(t *testing.T) {
	for _, submitted := range []bool{false, true} {
		t.Run(map[bool]string{false: "interrupted-input", true: "submitted"}[submitted], func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			f.env["GANGLINE_HITCH_ID"] = "a"
			p, _ := f.run.team.Agent(a.ID)
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			a.Compaction = &core.Compaction{ID: "c", Resume: core.Message{Text: "continue"}, StartedAt: f.cmd.now().Add(-time.Hour), Deadline: f.cmd.now().Add(-time.Minute), Status: "submitted"}
			if !submitted {
				a.Compaction.Status = "queued"
				a.Input = &core.InputIntent{ID: "c", Kind: "compaction", At: a.Compaction.StartedAt}
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
					t.Fatal(err)
				}
			}
			a, err = p.Read()
			if err != nil {
				t.Fatal(err)
			}
			wantSubmits, wantStatus := 0, "unverified"
			if f.input.submits != wantSubmits || a.Compaction.Status != wantStatus || a.Input != nil {
				t.Fatalf("recovery: %+v submits=%d", a, f.input.submits)
			}
		})
	}
}

func TestCompletedCompactionResumesBeforeQueuedMessages(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	var prompts []string
	f.input.submit = func(prompt string) error {
		if strings.HasPrefix(prompt, "/compact") {
			return nil
		}
		prompts = append(prompts, prompt)
		return p.WriteWitness(store.Witness{ID: fmt.Sprintf("witness-%d", len(prompts)), At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
	}
	var ce commandError
	if err := f.cmd.compact([]string{"worker", "--resume", "state is in FILE"}); !errors.As(err, &ce) || ce.status != exitUnknown {
		t.Fatalf("submission: %v", err)
	}
	// A teammate's message arrives while compaction is running.
	e := core.Envelope{ID: "teammate", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "sent during compaction"}, CreatedAt: f.cmd.now().Add(-time.Second)}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: f.cmd.now().Add(time.Second)}, false); err != nil {
		t.Fatal(err)
	}
	if len(prompts) == 0 || !strings.Contains(prompts[0], "state is in FILE") {
		t.Fatalf("first delivery after compaction was not the resume note: %q", prompts)
	}
}

// An agent compacts itself from its own window during a running turn.
func TestAgentCompactsItselfAfterItsTurn(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	f.env["TMUX_PANE"] = a.Pane
	f.input.screen = screenWithText("• Working (esc to interrupt)", "", "› ")
	if err := f.cmd.compact([]string{"--resume", "state is in FILE"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "queued") || f.input.submits != 0 {
		t.Fatalf("compaction during a turn: out=%q submits=%d", f.out.String(), f.input.submits)
	}
	f.input.screen = screenWithText("› ")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 || a.Compaction == nil || a.Compaction.Status != "submitted" || a.Compaction.Resume.Text != "state is in FILE" {
		t.Fatalf("compaction after the turn: submits=%d compaction=%+v", f.input.submits, a.Compaction)
	}
}
