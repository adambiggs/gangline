package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// holdFor publishes a message from sender to recipient as a queued send
// leaves it, and records the agents later woken.
func holdFor(t *testing.T, f *stateFixture, recipient core.Agent, sender core.Sender, text string) (core.Envelope, *[]string) {
	t.Helper()
	p, err := f.run.team.Agent(recipient.ID)
	if err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: "msg-held", Token: "0123456789abcdef", Recipient: recipient.ID, To: recipient.Name, From: sender, Message: core.Message{Text: text}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	woken := []string{}
	f.cmd.detach = func(id string, _ hookNotice) error { woken = append(woken, id); return nil }
	f.run.cmd = f.cmd
	return e, &woken
}

func agentSender(a core.Agent) core.Sender {
	return core.Sender{Kind: core.SenderAgent, Name: a.Name, HitchID: a.ID}
}

func outcomeNotice(t *testing.T, f *stateFixture, sender core.Agent, id core.EnvelopeID) core.Envelope {
	t.Helper()
	p, _ := f.run.team.Agent(sender.ID)
	e, err := p.ReadEnvelope("new", "outcome-"+id)
	if err != nil {
		t.Fatalf("outcome notice for %s: %v", id, err)
	}
	if e.From != (core.Sender{Kind: core.SenderGangline, Name: "delivery"}) || e.Recipient != sender.ID || !strings.Contains(e.Message.Text, "Message "+string(id)+" to worker ") {
		t.Fatalf("outcome notice: %+v", e)
	}
	return e
}

// deliveredNotice reads the notice that tells sender a message it was told
// was unverified has since been delivered.
func deliveredNotice(t *testing.T, f *stateFixture, sender core.Agent, id core.EnvelopeID) core.Envelope {
	t.Helper()
	p, _ := f.run.team.Agent(sender.ID)
	e, err := p.ReadEnvelope("new", "delivered-"+id)
	if err != nil {
		t.Fatalf("delivered notice for %s: %v", id, err)
	}
	if e.From != (core.Sender{Kind: core.SenderGangline, Name: "delivery"}) || e.Recipient != sender.ID || !strings.Contains(e.Message.Text, "Message "+string(id)+" to worker is delivered") || !strings.Contains(e.Message.Text, "do not send it again") {
		t.Fatalf("delivered notice: %+v", e)
	}
	return e
}

func requireNoDeliveredNotice(t *testing.T, f *stateFixture, sender core.Agent, id core.EnvelopeID) {
	t.Helper()
	p, _ := f.run.team.Agent(sender.ID)
	if e, err := p.ReadEnvelope("new", "delivered-"+id); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("delivered notice for %s: %+v %v", id, e, err)
	}
}

func requireNoNotices(t *testing.T, f *stateFixture) {
	t.Helper()
	agents, err := f.run.team.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		p, _ := f.run.team.Agent(a.ID)
		pending, err := p.ListNew()
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range pending {
			if strings.HasPrefix(string(e.ID), "outcome-") || strings.HasPrefix(string(e.ID), "delivered-") {
				t.Fatalf("%s got outcome notice %+v", a.Name, e)
			}
		}
	}
}

// A message held for a busy or locked agent is delivered by a later command,
// after the sending command reported it queued, so the sender learns from its
// own queue when that delivery does not succeed.
func TestHeldMessageOutcomeReachesSender(t *testing.T) {
	for _, test := range []struct {
		name, outcome, text, want string
		recipient                 func(*testing.T) (*stateFixture, core.Agent, store.AgentPaths)
	}{
		{"unverified", "unverified", "check the build", "is unverified", queueSendFixture},
		{"failed", "failed", placeholderQuote, "failed and was not delivered", claudeRecipient},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, a, _ := test.recipient(t)
			lead := f.add(t, "b", "lead", "claude")
			e, woken := holdFor(t, f, a, agentSender(lead), test.text)
			outcome, err := f.run.drain(a.ID, "")
			if err != nil || outcome != "queued" {
				t.Fatalf("tick drain: %s %v", outcome, err)
			}
			p, _ := f.run.team.Agent(a.ID)
			if settled, err := p.ReadEnvelope("failed", e.ID); err != nil || settled.Outcome != test.outcome {
				t.Fatalf("held message: %+v %v", settled, err)
			}
			if notice := outcomeNotice(t, f, lead, e.ID); !strings.Contains(notice.Message.Text, test.want) || strings.Contains(notice.Message.Text, "placeholder") {
				t.Fatalf("notice: %q", notice.Message.Text)
			}
			if len(*woken) != 1 || (*woken)[0] != string(lead.ID) {
				t.Fatalf("woken: %v", *woken)
			}
		})
	}
}

// Gangline's own messages, which include these notices, and senders Gangline
// did not observe have no queue to tell, so a notice never produces another.
func TestHeldMessageOutcomeSkipsUnobservedSenders(t *testing.T) {
	for _, from := range []core.Sender{
		{Kind: core.SenderGangline, Name: "delivery"},
		{Kind: core.SenderSelfDeclared, Name: "lead"},
		{Kind: core.SenderAgent, Name: "worker", HitchID: "a"},
		{Kind: core.SenderAgent, Name: "gone", HitchID: "c"},
	} {
		t.Run(string(from.Kind)+"/"+string(from.Name), func(t *testing.T) {
			f, a, _ := queueSendFixture(t)
			f.add(t, "b", "lead", "claude")
			_, woken := holdFor(t, f, a, from, "check the build")
			if outcome, err := f.run.drain(a.ID, ""); err != nil || outcome != "queued" {
				t.Fatalf("tick drain: %s %v", outcome, err)
			}
			requireNoNotices(t, f)
			if len(*woken) != 0 {
				t.Fatalf("woken: %v", *woken)
			}
		})
	}
}

func TestHeldMessageOutcomeSkipsDroppingSender(t *testing.T) {
	f, a, _ := queueSendFixture(t)
	lead := f.add(t, "b", "lead", "claude")
	f.setAgent(t, lead, func(a *core.Agent) { a.Status = core.Dropping })
	_, woken := holdFor(t, f, a, agentSender(lead), "check the build")
	if outcome, err := f.run.drain(a.ID, ""); err != nil || outcome != "queued" {
		t.Fatalf("tick drain: %s %v", outcome, err)
	}
	requireNoNotices(t, f)
	if len(*woken) != 0 {
		t.Fatalf("woken: %v", *woken)
	}
}

func TestDeliveredHeldMessageSendsNoNotice(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	lead := f.add(t, "b", "lead", "claude")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	e, _ := holdFor(t, f, a, agentSender(lead), "check the build")
	if outcome, err := f.run.drain(a.ID, ""); err != nil || outcome != "queued" {
		t.Fatalf("tick drain: %s %v", outcome, err)
	}
	p, _ := f.run.team.Agent(a.ID)
	if settled, err := p.ReadEnvelope("cur", e.ID); err != nil || settled.Outcome != "delivered" {
		t.Fatalf("held message: %+v %v", settled, err)
	}
	requireNoNotices(t, f)
}

// The sending command reports its own message's outcome by exit status.
func TestDirectSendReportsItsOwnOutcomeWithoutNotice(t *testing.T) {
	f, _, _ := queueSendFixture(t)
	lead := f.add(t, "b", "lead", "codex")
	f.env["GANG_AGENT_ID"] = string(lead.ID)
	f.env["TMUX_PANE"] = lead.Pane
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	err := f.cmd.send([]string{"worker", "check the build"})
	var unknown commandError
	if !errors.As(err, &unknown) || unknown.status != exitUnknown {
		t.Fatalf("send: %v", err)
	}
	requireNoNotices(t, f)
}

// An input owner that exits mid-delivery never reports the outcome, so the
// command that recovers its input tells the sender.
func TestRecoveredHeldMessageReachesSender(t *testing.T) {
	f, a, _ := queueSendFixture(t)
	lead := f.add(t, "b", "lead", "claude")
	e, woken := holdFor(t, f, a, agentSender(lead), "check the build")
	f.setAgent(t, a, func(a *core.Agent) { a.Input = &core.InputIntent{ID: string(e.ID), Kind: "envelope"} })
	f.input.screen = screenWithText("• Working (esc to interrupt)", "› ")
	if _, err := f.run.drain(a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if notice := outcomeNotice(t, f, lead, e.ID); !strings.Contains(notice.Message.Text, "is unverified") {
		t.Fatalf("notice: %q", notice.Message.Text)
	}
	if len(*woken) != 1 || (*woken)[0] != string(lead.ID) {
		t.Fatalf("woken: %v", *woken)
	}
}

func TestDroppedRecipientHeldMessageReachesSender(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	lead := f.add(t, "b", "lead", "claude")
	e, woken := holdFor(t, f, a, agentSender(lead), "check the build")
	if err := f.cmd.drop([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
	if notice := outcomeNotice(t, f, lead, e.ID); !strings.Contains(notice.Message.Text, "worker was dropped") {
		t.Fatalf("notice: %q", notice.Message.Text)
	}
	if len(*woken) != 1 || (*woken)[0] != string(lead.ID) {
		t.Fatalf("woken: %v", *woken)
	}
}

// The notice rides on an operation that has already settled the message, so a
// sender that cannot take the notice is logged and warned about while the
// drain, recovery, or drop finishes.
func TestUnsendableNoticeDoesNotStopTheOperation(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*testing.T, *stateFixture, core.Agent, core.Envelope) error
		want string
	}{
		{"drain", func(t *testing.T, f *stateFixture, a core.Agent, _ core.Envelope) error {
			_, err := f.run.drain(a.ID, "")
			return err
		}, "delivery_unverified"},
		{"recover", func(t *testing.T, f *stateFixture, a core.Agent, e core.Envelope) error {
			f.setAgent(t, a, func(a *core.Agent) { a.Input = &core.InputIntent{ID: string(e.ID), Kind: "envelope"} })
			f.input.screen = screenWithText("• Working (esc to interrupt)", "› ")
			_, err := f.run.drain(a.ID, "")
			return err
		}, "input_finished"},
		{"drop", func(t *testing.T, f *stateFixture, _ core.Agent, _ core.Envelope) error {
			return f.cmd.drop([]string{"worker"})
		}, "drop_finished"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, a, _ := queueSendFixture(t)
			lead := f.add(t, "b", "lead", "claude")
			e, woken := holdFor(t, f, a, agentSender(lead), "check the build")
			p, _ := f.run.team.Agent(lead.ID)
			path, _ := p.EnvelopePath("new", "outcome-"+e.ID)
			inbox := filepath.Dir(path)
			if err := os.Chmod(inbox, 0500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(inbox, 0700) })
			if err := test.run(t, f, a, e); err != nil {
				t.Fatalf("%s: %v", test.name, err)
			}
			log, err := os.Open(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close()
			var failed, finished bool
			if err := store.ReadLog(log, func(event core.Event) error {
				if event.Type == "notice_failed" && event.HitchID == a.ID && event.ID == string(e.ID) && strings.Contains(event.Reason, "permission denied") {
					failed = true
				}
				finished = finished || event.Type == test.want && event.HitchID == a.ID
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if !failed || !finished {
				t.Fatalf("notice_failed %v, %s %v", failed, test.want, finished)
			}
			if !strings.Contains(f.errOut.String(), "warning: could not tell lead about message "+string(e.ID)) {
				t.Fatalf("stderr: %q", f.errOut.String())
			}
			if len(*woken) != 0 {
				t.Fatalf("woken: %v", *woken)
			}
		})
	}
}
