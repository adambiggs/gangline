package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestCompactionQueuesResumeBeforeCompletion(t *testing.T) {
	for _, collar := range []string{"codex", "claude-code"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			p, _ := f.run.team.Agent(a.ID)
			a.Native.SessionID = "s"
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			l.Close()
			f.env["TMUX_PANE"] = a.Pane
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			if collar == "claude-code" {
				f.input.command = "claude"
				f.input.screen = screenWithText("────────", "❯ ", "────────")
			}
			f.input.submit = func(prompt string) error {
				if strings.HasPrefix(prompt, "/compact") {
					return nil
				}
				got, err := p.Read()
				if err != nil {
					return err
				}
				if got.Compaction.Status != "submitted" || got.Input == nil || got.Input.ID != "resume-"+got.Compaction.ID {
					t.Fatalf("resume was not submitted during compaction: %+v", got)
				}
				if collar == "codex" {
					f.input.screen = nativeQueueScreen(prompt)
				}
				return nil
			}
			var ce commandError
			if err := f.cmd.compact([]string{"worker", "--resume", "continue the work"}); !errors.As(err, &ce) || ce.status != exitUnknown {
				t.Fatalf("submission: %v", err)
			}
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Compaction.Status != "submitted" || !got.Compaction.Continuation || f.input.submits != 2 {
				t.Fatalf("early continuation: %+v submits=%d", got.Compaction, f.input.submits)
			}
			dir, outcome := "cur", "accepted"
			if collar == "claude-code" {
				dir, outcome = "failed", "unverified"
			}
			e, err := p.ReadEnvelope(dir, core.EnvelopeID("resume-"+got.Compaction.ID))
			if err != nil || e.Outcome != outcome || e.Purpose != "resume" || len(e.Token) != 16 {
				t.Fatalf("early receipt: %+v, %v", e, err)
			}
			if e.From != (core.Sender{Kind: core.SenderAgent, Name: a.Name, HitchID: a.ID}) {
				t.Fatalf("resume sender: %+v", e.From)
			}
			wire, err := envelopeText(e)
			if err != nil || f.input.pasted != wire {
				t.Fatalf("native queue text: %q, %v", f.input.pasted, err)
			}
			prompt := wire
			if collar == "claude-code" {
				prompt = "<pasted_content id=\"probe\">\n" + wire + "\n</pasted_content id=\"probe\">"
			} else {
				prompt += "\nLATER_OPERATOR_INPUT"
			}
			payload, err := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": prompt, "session_id": "s"})
			if err != nil {
				t.Fatal(err)
			}
			f.out.Reset()
			hook := f.cmd
			hook.stdin = bytes.NewReader(payload)
			if err := hook.handleHook(nil); err != nil || !strings.Contains(f.out.String(), `"decision":"block"`) {
				t.Fatalf("unconfirmed compaction hook: %v, %q", err, f.out.String())
			}
			if err := f.run.confirmCompactionHook(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: f.cmd.now().Add(time.Second)}); err != nil {
				t.Fatal(err)
			}
			got, err = p.Read()
			if err != nil {
				t.Fatal(err)
			}
			f.out.Reset()
			hook.stdin = bytes.NewReader(payload)
			if err := hook.handleHook(nil); err != nil || strings.Contains(f.out.String(), `"decision":"block"`) {
				t.Fatalf("confirmed compaction hook: %v, %q", err, f.out.String())
			}
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			e, err = p.ReadEnvelope("cur", e.ID)
			if err != nil || e.Outcome != "delivered" || f.input.submits != 2 {
				t.Fatalf("resume delivery: %+v, %v; submits=%d", e, err, f.input.submits)
			}
			f.out.Reset()
			hook.stdin = bytes.NewReader(payload)
			if err := hook.handleHook(nil); err != nil || !strings.Contains(f.out.String(), `"decision":"block"`) {
				t.Fatalf("duplicate resume was admitted: %v, %q", err, f.out.String())
			}
			if err := f.run.confirmCompactionHook(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: f.cmd.now().Add(time.Second)}); err != nil || f.input.submits != 2 {
				t.Fatalf("repeated completion: %v; submits=%d", err, f.input.submits)
			}
		})
	}
}

func TestCompactionResumeHookBlocksWhenStateCannotBeRead(t *testing.T) {
	f := newStateFixture(t)
	f.env["GANGLINE_HITCH_ID"] = "missing"
	f.cmd.stdin = strings.NewReader(`{"hook_event_name":"UserPromptSubmit","prompt":"[gang:self-declared:compact#token resume] state [/gang:self-declared:compact#token]"}`)
	if err := f.cmd.hook(nil); err != nil || !strings.Contains(f.out.String(), `"decision":"block"`) {
		t.Fatalf("unverifiable resume was not blocked: %v, %q", err, f.out.String())
	}
}

func TestOversizedCompactionResumeHookFailsClosed(t *testing.T) {
	f := newStateFixture(t)
	f.cmd.stdin = strings.NewReader(`{"hook_event_name":"UserPromptSubmit","prompt":"[gang:self-declared:compact#token resume] ` + strings.Repeat("x", maximumHookBytes) + `"}`)
	if err := f.cmd.hook(nil); err != nil || !strings.Contains(f.out.String(), `"decision":"block"`) {
		t.Fatalf("oversized resume was not blocked: %v, %q", err, f.out.String())
	}
}

func TestCompactionResumeAdmissionIsAtomic(t *testing.T) {
	f, a, p := compactionFixture(t)
	a.Compaction = &core.Compaction{
		ID: "c", Resume: core.Message{Text: "continue"}, ResumeToken: "aaaaaaaaaaaaaaaa", Continuation: true,
		ResumeFrom: core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"},
		StartedAt:  f.cmd.now().Add(-time.Second), CompletedAt: f.cmd.now(), Status: "completed",
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	c, err := loadCollar("codex", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := envelopeText(core.Envelope{ID: "resume-c", Token: "aaaaaaaaaaaaaaaa", From: a.Compaction.ResumeFrom, Message: a.Compaction.Resume, Purpose: "resume"})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan struct {
		reason string
		err    error
	}, 2)
	for range 2 {
		go func() {
			<-start
			reason, err := f.run.admitCompactionResume(a.ID, c, wire, "s")
			results <- struct {
				reason string
				err    error
			}{reason, err}
		}()
	}
	close(start)
	admitted := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.reason == "" {
			admitted++
		} else if result.reason != "compaction continuation already admitted" {
			t.Fatalf("unexpected block: %q", result.reason)
		}
	}
	if admitted != 1 {
		t.Fatalf("admitted %d continuations; want one", admitted)
	}
}

func TestCompactionResumeHookFailurePreservesAdmission(t *testing.T) {
	f, a, p := compactionFixture(t)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	a.Compaction = &core.Compaction{
		ID: "c", Resume: core.Message{Text: "continue"}, ResumeToken: "aaaaaaaaaaaaaaaa", Continuation: true,
		ResumeFrom: core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"},
		StartedAt:  f.cmd.now().Add(-time.Second), CompletedAt: f.cmd.now(), Status: "completed",
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	wire, err := envelopeText(core.Envelope{ID: "resume-c", Token: a.Compaction.ResumeToken, From: a.Compaction.ResumeFrom, Message: a.Compaction.Resume, Purpose: "resume"})
	if err != nil {
		t.Fatal(err)
	}
	hook := func(transcript string) string {
		t.Helper()
		payload, err := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": wire, "session_id": "s", "transcript_path": transcript})
		if err != nil {
			t.Fatal(err)
		}
		f.out.Reset()
		cmd := f.cmd
		cmd.stdin = bytes.NewReader(payload)
		if err := cmd.hook(nil); err != nil {
			t.Fatal(err)
		}
		return f.out.String()
	}
	if out := hook(strings.Repeat("x", 4097)); !strings.Contains(out, `"decision":"block"`) {
		t.Fatalf("invalid metadata was admitted: %q", out)
	}
	state, err := p.Read()
	if err != nil || state.Compaction.ResumeAdmitted {
		t.Fatalf("blocked hook consumed resume: %+v, %v", state.Compaction, err)
	}
	cmd := f.cmd
	cmd.detach = func(string, hookNotice) error { return errors.New("tick could not start") }
	f.cmd = cmd
	if out := hook("valid"); strings.Contains(out, `"decision":"block"`) {
		t.Fatalf("admitted hook was blocked after auxiliary failure: %q", out)
	}
	state, err = p.Read()
	if err != nil || !state.Compaction.ResumeAdmitted {
		t.Fatalf("admitted hook lost resume: %+v, %v", state.Compaction, err)
	}
	if out := hook("valid"); !strings.Contains(out, `"decision":"block"`) {
		t.Fatalf("duplicate hook was admitted: %q", out)
	}
}

func TestCompactionOccupiedComposerFailsWithoutLateResume(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	a.Native.SessionID = "s"
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.input.submit = func(prompt string) error {
		if strings.HasPrefix(prompt, "/compact") {
			f.input.screen = screenWithText("› unfinished draft")
			return nil
		}
		return p.WriteWitness(store.Witness{ID: "resume-witness", At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
	}
	if err := f.cmd.compact([]string{"worker"}); err == nil || !strings.Contains(err.Error(), "composer occupied") {
		t.Fatalf("occupied composer result: %v", err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	e, err := p.ReadEnvelope("failed", core.EnvelopeID("resume-"+got.Compaction.ID))
	if err != nil || got.Compaction.Status != "failed" || f.input.submits != 1 || e.Outcome != "cancelled" {
		t.Fatalf("failed continuation: %+v, %v; status=%s submits=%d", e, err, got.Compaction.Status, f.input.submits)
	}
	f.input.screen = screenWithText("› ")
	if err := f.run.confirmCompactionHook(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: f.cmd.now().Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 {
		t.Fatalf("continuation ran after ordering failed: %d", f.input.submits)
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
	if f.input.submits != 2 || a.Compaction == nil || a.Compaction.Status != "submitted" || a.Compaction.Resume.Text != "state is in FILE" {
		t.Fatalf("compaction after the turn: submits=%d compaction=%+v", f.input.submits, a.Compaction)
	}
}
