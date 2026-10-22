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
			if len(e.Token) != 16 || !strings.Contains(f.input.pasted, "#"+e.Token+"]") || strings.Contains(f.input.pasted, string(e.ID)) {
				t.Fatalf("wire token differs from stored token: id=%s token=%q wire=%q", e.ID, e.Token, f.input.pasted)
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

func TestSendUsesPositionalBodyInsteadOfStdin(t *testing.T) {
	f, _, p := queueSendFixture(t)
	f.cmd.stdin = strings.NewReader("wrong stdin body")
	f.input.submit = func(wire string) error {
		f.input.screen = nativeQueueScreen(wire)
		return nil
	}
	if err := f.cmd.send([]string{"worker", "--from", "operator", "two\nlines"}); err != nil {
		t.Fatal(err)
	}
	id := core.EnvelopeID(strings.Fields(f.out.String())[0])
	e, err := p.ReadEnvelope("cur", id)
	if err != nil || e.Message.Text != "two\nlines" {
		t.Fatalf("receipt = %+v, %v", e, err)
	}
}

func TestSendRejectsEmptyPositionalBodyWithoutReadingStdin(t *testing.T) {
	f, _, _ := queueSendFixture(t)
	f.cmd.stdin = strings.NewReader("decoy stdin")
	if err := f.cmd.send([]string{"worker", "--from", "operator", ""}); err == nil || !strings.Contains(err.Error(), "message body is empty") {
		t.Fatalf("empty positional body error = %v", err)
	}
}

func TestSendDoesNotAcceptAmbiguousQueueEvidence(t *testing.T) {
	for _, kind := range []string{"wrong ID", "wrong sender", "missing header", "not dim", "occupied composer", "clipped ID"} {
		t.Run(kind, func(t *testing.T) {
			f, _, _ := queueSendFixture(t)
			f.input.submit = func(wire string) error {
				switch kind {
				case "wrong ID":
					wire = strings.Replace(wire, "#", "#different", 1)
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

func TestSendWaitsPastEarlierSubmitHook(t *testing.T) {
	for _, test := range []struct{ collar, receipt string }{
		{"codex", "accepted"},
		{"codex", "delivered"},
		{"codex", "missing"},
		{"codex", "foreign session"},
		{"claude", "delivered"},
		{"claude", "missing"},
		{"claude", "foreign session"},
	} {
		t.Run(test.collar+"/"+test.receipt, func(t *testing.T) {
			receipt := test.receipt
			fixture := queueSendFixture
			if test.collar == "claude" {
				fixture = claudeRecipient
			}
			f, a, p := fixture(t)
			f.cmd.stdin = strings.NewReader("receipt ordering")
			f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "s" })
			session := "s"
			if receipt == "foreign session" {
				session = "foreign"
			}
			// An earlier queued input reaches its submit hook before this
			// message's queue preview paints or its own submit hook arrives.
			f.input.submit = func(string) error {
				return p.WriteWitness(store.Witness{ID: "earlier-hook", At: f.cmd.now(), SessionID: session, Prompt: "earlier queued input"})
			}
			waits := 0
			f.cmd.newWatch = func(string) (changeWait, error) {
				return waitFixture{func(context.Context) error {
					waits++
					if waits > 1 || receipt == "missing" || receipt == "foreign session" {
						return errors.New("test boundary: no matching receipt")
					}
					if receipt == "accepted" {
						f.input.screen = nativeQueueScreen(f.input.pasted)
						return nil
					}
					return p.WriteWitness(store.Witness{ID: "matching-hook", At: f.cmd.now(), SessionID: "s", Prompt: f.input.pasted})
				}}, nil
			}
			err := f.cmd.send([]string{"worker", "--from", "operator"})
			want := receipt
			if receipt == "missing" || receipt == "foreign session" {
				want = "unverified"
			}
			fields := strings.Fields(f.out.String())
			if len(fields) != 2 || fields[1] != want || (err != nil) != (want == "unverified") {
				t.Fatalf("output=%q err=%v; want %s", f.out, err, want)
			}
			if f.input.submits != 1 || receipt != "foreign session" && waits != 1 || receipt == "foreign session" && waits != 0 {
				t.Fatalf("submits=%d waits=%d", f.input.submits, waits)
			}
			directory := "cur"
			if want == "unverified" {
				directory = "failed"
			}
			e, readErr := p.ReadEnvelope(directory, core.EnvelopeID(fields[0]))
			if readErr != nil || e.Outcome != want || want != "unverified" && e.Witnessed != "" {
				t.Fatalf("receipt=%+v err=%v", e, readErr)
			}
		})
	}
}

func TestEarlierSubmitHooksRespectOperationDeadline(t *testing.T) {
	for _, budget := range []time.Duration{time.Millisecond, time.Second, time.Hour} {
		for _, test := range []struct {
			collar   string
			canceled bool
		}{
			{"codex", false},
			{"codex", true},
			{"claude", false},
			{"claude", true},
		} {
			t.Run(fmt.Sprintf("%s/%s/canceled=%t", test.collar, budget, test.canceled), func(t *testing.T) {
				canceled := test.canceled
				fixture := queueSendFixture
				if test.collar == "claude" {
					fixture = claudeRecipient
				}
				f, a, p := fixture(t)
				f.cmd.stdin = strings.NewReader("receipt ordering")
				f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "s" })
				start, now := f.cmd.now(), f.cmd.now()
				f.cmd.clock = func() time.Time { return now }
				var cancelOperation context.CancelFunc
				f.cmd.newTimeout = func(ctx context.Context, requested time.Duration) (context.Context, context.CancelFunc) {
					if requested != operationTimeout {
						t.Fatalf("delivery budget=%s", requested)
					}
					ctx, cancelOperation = context.WithCancel(ctx)
					return fakeDeadline{ctx, start.Add(budget)}, cancelOperation
				}
				f.input.submit = func(string) error { return nil }
				watches := 0
				f.cmd.newWatch = func(string) (changeWait, error) {
					watches++
					if watches > 2 {
						return nil, errors.New("processed another hook after the operation ended")
					}
					// One hook arrives halfway through the budget; the next
					// reaches its deadline or cancels with half still unused.
					now = start.Add(time.Duration(watches) * budget / 2)
					if canceled {
						now = start.Add(budget / 2)
						if watches == 2 {
							cancelOperation()
						}
					}
					if err := p.WriteWitness(store.Witness{ID: fmt.Sprintf("earlier-%d", watches), At: now, SessionID: "s", Prompt: "earlier queued input"}); err != nil {
						t.Fatal(err)
					}
					return waitFixture{func(context.Context) error { return errors.New("unexpected wait with a fresh hook") }}, nil
				}
				err := f.cmd.send([]string{"worker", "--from", "operator"})
				var ce commandError
				if !errors.As(err, &ce) || ce.status != exitUnknown || watches != 2 || f.input.submits != 1 {
					t.Fatalf("result=%v watches=%d submits=%d", err, watches, f.input.submits)
				}
				fields := strings.Fields(f.out.String())
				if len(fields) != 2 || fields[1] != "unverified" {
					t.Fatalf("output=%q", f.out)
				}
				e, err := p.ReadEnvelope("failed", core.EnvelopeID(fields[0]))
				want := context.DeadlineExceeded.Error()
				if canceled {
					want = context.Canceled.Error()
				}
				if err != nil || e.Outcome != "unverified" || e.Reason != want {
					t.Fatalf("receipt=%+v err=%v; want %s", e, err, want)
				}
			})
		}
	}
}

func TestResumeSubmitMismatchDoesNotRetry(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		t.Run(collar, func(t *testing.T) {
			fixture := queueSendFixture
			if collar == "claude" {
				fixture = claudeRecipient
			}
			f, a, p := fixture(t)
			a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "s" })
			// This recognized resume form has no explicit purpose.
			e := core.Envelope{ID: "resume-c", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "compact"}, Message: core.Message{Text: "resume"}, CreatedAt: f.cmd.now()}
			if !isResumeEnvelope(e) {
				t.Fatal("fixture is not a resume envelope")
			}
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
			start, now := f.cmd.now(), f.cmd.now()
			f.cmd.clock = func() time.Time { return now }
			f.cmd.newTimeout = func(ctx context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
				return fakeDeadline{ctx, start.Add(time.Second)}, func() {}
			}
			f.input.submit = func(string) error {
				now = start.Add(time.Second)
				return p.WriteWitness(store.Witness{ID: "other", At: start, SessionID: "s", Prompt: "another input"})
			}
			f.run.cmd = f.cmd
			c, err := loadCollar(collar, f.run.settings)
			if err != nil {
				t.Fatal(err)
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			outcome, _, err := f.run.deliver(l, &a, e, f.input, c, "")
			if err != nil || outcome != "unverified" || f.input.submits != 1 {
				t.Fatalf("outcome=%s err=%v submits=%d", outcome, err, f.input.submits)
			}
			got, err := p.ReadEnvelope("failed", e.ID)
			if err != nil || got.Reason != "submit witness does not match the message text and one-time token" {
				t.Fatalf("resume mismatch: %+v err=%v", got, err)
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

func TestContextBandRequiresExactHookDespiteQueuePreview(t *testing.T) {
	for _, exact := range []bool{false, true} {
		t.Run(map[bool]string{false: "preview only", true: "exact hook"}[exact], func(t *testing.T) {
			f, a, p := queueSendFixture(t)
			e := core.Envelope{ID: "context-2", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "context-band"}, Message: core.Message{Text: "crossed"}, CreatedAt: f.cmd.now()}
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
			f.input.submit = func(wire string) error {
				f.input.screen = nativeQueueScreen(wire)
				if exact {
					return p.WriteWitness(store.Witness{ID: "new-hook", At: f.cmd.now(), SessionID: "s", Prompt: wire})
				}
				return nil
			}
			f.cmd.newWatch = func(string) (changeWait, error) {
				t.Fatal("context notice must not wait for a submit hook while holding the agent lock")
				return nil, errors.New("unexpected witness wait")
			}
			f.run.cmd = f.cmd
			if _, err := f.run.drain(a.ID, e.ID); err != nil {
				t.Fatal(err)
			}
			dir, want := "failed", "unverified"
			if exact {
				dir, want = "cur", "delivered"
			}
			got, readErr := p.ReadEnvelope(dir, e.ID)
			if readErr != nil || got.Outcome != want || f.input.submits != 1 {
				t.Fatalf("receipt: %+v %v submits=%d", got, readErr, f.input.submits)
			}
			if !exact {
				if err := p.WriteWitness(store.Witness{ID: "later-hook", At: f.cmd.now(), SessionID: "s", Prompt: f.input.pasted}); err != nil {
					t.Fatal(err)
				}
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
					t.Fatal(err)
				}
				got, readErr = p.ReadEnvelope("cur", e.ID)
				if readErr != nil || got.Outcome != "delivered" || f.input.submits != 1 {
					t.Fatalf("late receipt: %+v %v submits=%d", got, readErr, f.input.submits)
				}
			}
		})
	}
}
