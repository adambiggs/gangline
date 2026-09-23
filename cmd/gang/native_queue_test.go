package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func nativeQueueScreen(wire string) substrate.Screen {
	opener := wire[:strings.Index(wire, "]")+1]
	screen := screenWithText("• Working (esc to interrupt)", "", "• Messages to be submitted after next tool call", "  (press esc to interrupt and send immediately)", "  ↳ "+opener+" message preview", "", "› ")
	for i := range screen.Rows[4] {
		screen.Rows[4][i].Attributes.Dim = true
	}
	return screen
}

func queueSendFixture(t *testing.T) (*stateFixture, core.Agent, store.AgentPaths) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	f.input.screen = screenWithText("• Working (esc to interrupt)", "› ")
	f.cmd.stdin = strings.NewReader("native queue acceptance")
	f.cmd.newWatch = func(string) (changeWait, error) {
		return waitFixture{func(context.Context) error { return errors.New("test boundary: no submit hook yet") }}, nil
	}
	return f, a, p
}

func TestSendReportsNativeQueueAcceptance(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		t.Run(map[bool]string{false: "immediate", true: "paint event"}[delayed], func(t *testing.T) {
			f, a, p := queueSendFixture(t)
			f.input.submit = func(wire string) error {
				if !delayed {
					f.input.screen = nativeQueueScreen(wire)
				}
				return nil
			}
			waits := 0
			if delayed {
				f.cmd.newWatch = func(string) (changeWait, error) {
					return waitFixture{func(context.Context) error {
						waits++
						if waits > 1 {
							return errors.New("queue acceptance missed")
						}
						f.input.screen = nativeQueueScreen(f.input.pasted)
						return nil
					}}, nil
				}
			}
			if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(f.out.String(), "accepted") || !strings.Contains(f.errOut.String(), "do not resend") {
				t.Fatalf("output: %q", f.out)
			}
			id := core.EnvelopeID(strings.Fields(f.out.String())[0])
			e, err := p.ReadEnvelope("cur", id)
			if err != nil || e.Outcome != "accepted" || !strings.Contains(e.Reason, "native queue") {
				t.Fatalf("receipt: %+v %v", e, err)
			}
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Input != nil || got.LastDelivered == id || got.LastFailed == id {
				t.Fatalf("acceptance confused with delivery: %+v", got)
			}
			f.run.cmd = f.cmd
			if _, err := f.run.drain(a.ID, ""); err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 1 {
				t.Fatalf("repeated accepted input: %d", f.input.submits)
			}
			// A later exact hook promotes the retained acceptance without typing again.
			if err := p.WriteWitness(store.Witness{ID: "later", At: f.cmd.now(), SessionID: "s", Prompt: f.input.pasted}); err != nil {
				t.Fatal(err)
			}
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			e, err = p.ReadEnvelope("cur", id)
			if err != nil || e.Outcome != "delivered" || f.input.submits != 1 {
				t.Fatalf("late exact proof: %+v %v submits=%d", e, err, f.input.submits)
			}
		})
	}
}

func TestSendDoesNotAcceptAmbiguousQueueEvidence(t *testing.T) {
	for _, kind := range []string{"wrong ID", "wrong sender", "missing header", "not dim", "occupied composer", "clipped ID"} {
		t.Run(kind, func(t *testing.T) {
			f, _, _ := queueSendFixture(t)
			f.input.submit = func(wire string) error {
				switch kind {
				case "wrong ID":
					wire = strings.Replace(wire, "#msg-", "#different-", 1)
				case "wrong sender":
					wire = strings.Replace(wire, "operator#", "other#", 1)
				case "clipped ID":
					wire = "[gang:self-declared:operator#msg-…]"
				}
				f.input.screen = nativeQueueScreen(wire)
				switch kind {
				case "missing header":
					f.input.screen.Rows[2] = nil
				case "not dim":
					for i := range f.input.screen.Rows[4] {
						f.input.screen.Rows[4][i].Attributes.Dim = false
					}
				case "occupied composer":
					f.input.screen.Rows[6] = screenWithText("› unfinished draft").Rows[0]
				}
				return nil
			}
			var ce commandError
			err := f.cmd.send([]string{"worker", "--from", "operator"})
			if !errors.As(err, &ce) || ce.status != exitUnknown || strings.Contains(f.out.String(), "accepted") {
				t.Fatalf("ambiguous acceptance: %q %v", f.out, err)
			}
		})
	}
}

func TestNativeQueueReceiptRespectsExactHookProof(t *testing.T) {
	for _, exact := range []bool{false, true} {
		t.Run(map[bool]string{false: "other prompt", true: "exact prompt"}[exact], func(t *testing.T) {
			f, _, p := queueSendFixture(t)
			f.input.submit = func(wire string) error {
				f.input.screen = nativeQueueScreen(wire)
				prompt := "an earlier prompt"
				if exact {
					prompt = wire
				}
				return p.WriteWitness(store.Witness{ID: "new-hook", At: f.cmd.now(), SessionID: "s", Prompt: prompt})
			}
			if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil {
				t.Fatal(err)
			}
			want := "accepted"
			if exact {
				want = "delivered"
			}
			if !strings.Contains(f.out.String(), want) {
				t.Fatalf("receipt: %q, want %s", f.out, want)
			}
		})
	}
}

func TestNativeQueueReceiptRecoveryNeverRetypes(t *testing.T) {
	f, a, p := queueSendFixture(t)
	e := core.Envelope{ID: "interrupted", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "once"}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Input = &core.InputIntent{ID: string(e.ID), Kind: "envelope", At: f.cmd.now()}
	if err := l.Settle(&a, e, "accepted", "native queue receipt"); err != nil {
		t.Fatal(err)
	}
	l.Close()
	if _, err := f.run.drain(a.ID, ""); err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadEnvelope("cur", e.ID)
	if err != nil || got.Outcome != "accepted" || f.input.submits != 0 {
		t.Fatalf("interrupted acceptance: %+v %v submits=%d", got, err, f.input.submits)
	}
}

func TestSendRecognizesWrappedNativeQueueOpener(t *testing.T) {
	f, _, _ := queueSendFixture(t)
	f.input.submit = func(wire string) error {
		f.input.screen = nativeQueueScreen(wire)
		opener := wire[:strings.Index(wire, "]")+1]
		first := "  ↳ " + opener[:30]
		second := "    " + opener[30:] + " preview"
		rows := screenWithText(first, second).Rows
		for r := range rows {
			for c := range rows[r] {
				rows[r][c].Attributes.Dim = true
			}
		}
		f.input.screen.Rows = append(f.input.screen.Rows[:4], append(rows, f.input.screen.Rows[5:]...)...)
		return nil
	}
	if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "accepted") {
		t.Fatalf("wrapped receipt: %q", f.out)
	}
}
