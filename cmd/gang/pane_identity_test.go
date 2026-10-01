package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

type identityFixture struct {
	*inputFixture
	present, visible      bool
	checkErr, ancestryErr error
}

func (b identityFixture) CheckPane(context.Context, tmux.PaneIdentity) (bool, error) {
	return b.present, b.checkErr
}
func (b identityFixture) ProcessVisibility(context.Context, substrate.PaneID) (bool, error) {
	return b.visible, nil
}
func (b identityFixture) VerifyCaller(context.Context, substrate.PaneID) error { return b.ancestryErr }

func TestPaneTokenIdentityInPrivateNamespace(t *testing.T) {
	for _, tc := range []struct {
		name, token, pane     string
		present, visible      bool
		checkErr, ancestryErr error
		want                  string
	}{
		{name: "sandbox", token: "secret", pane: "%1", present: true},
		{name: "missing pane", token: "secret", present: true, want: "current pane"},
		{name: "missing token", pane: "%1", present: true, want: "token"},
		{name: "visible missing token", pane: "%1", present: true, visible: true, want: "token"},
		{name: "wrong token", token: "other", pane: "%1", present: true, want: "token"},
		{name: "wrong pane", token: "secret", pane: "%2", present: true, want: "current pane"},
		{name: "pane absent", token: "secret", pane: "%1", want: "absent"},
		{name: "server replaced", token: "secret", pane: "%1", checkErr: errors.New("server generation differs"), want: "generation"},
		{name: "visible ancestor", token: "secret", pane: "%1", present: true, visible: true},
		{name: "visible outsider", token: "secret", pane: "%1", present: true, visible: true, ancestryErr: errors.New("caller is outside pane ancestry"), want: "ancestry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			a.Registration = core.PaneRegistration{Generation: "generation", Session: "$1", TokenHash: tokenHash("secret")}
			p, _ := f.run.team.Agent(a.ID)
			l, err := p.LockAgent()
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			l.Close()
			f.env["GANG_AGENT_ID"] = string(a.ID)
			f.env["GANG_AGENT_NONCE"] = tc.token
			f.env["TMUX_PANE"] = tc.pane
			f.cmd.paneBackend = identityFixture{f.input, tc.present, tc.visible, tc.checkErr, tc.ancestryErr}
			run, err := f.cmd.runtime()
			if err != nil {
				t.Fatal(err)
			}
			got, err := run.observedAgent()
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("got %v, want %s", err, tc.want)
				}
				return
			}
			if err != nil || got == nil || got.ID != a.ID {
				t.Fatalf("identity=%+v err=%v", got, err)
			}
			limited, err := run.processLimited(a)
			if err != nil || limited == tc.visible {
				t.Fatalf("limited=%v err=%v", limited, err)
			}
		})
	}
}

func TestForeignForegroundSendDoesNotPublish(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.input.tmuxCommand = "sh"
	err := f.cmd.send([]string{"worker", "--from", "operator", "must not queue"})
	if err == nil || !strings.Contains(err.Error(), "foreground") {
		t.Fatalf("send error=%v", err)
	}
	p, _ := f.run.team.Agent(a.ID)
	pending, err := p.ListNew()
	if err != nil || len(pending) != 0 || f.input.submits != 0 {
		t.Fatalf("pending=%+v submits=%d err=%v", pending, f.input.submits, err)
	}
}

func TestIncompletePaneRegistrationIsNeverBoundOnDemand(t *testing.T) {
	for _, field := range []string{"all", "generation", "session", "token", "pane"} {
		t.Run(field, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			switch field {
			case "all":
				a.Registration = core.PaneRegistration{}
			case "generation":
				a.Registration.Generation = ""
			case "session":
				a.Registration.Session = ""
			case "token":
				a.Registration.TokenHash = ""
			case "pane":
				a.Pane = ""
			}
			f.input.submit = nil
			for name, check := range map[string]func() error{
				"recipient": func() error { return f.run.checkRecipient(a) },
				"caller":    func() error { return f.run.verifyCaller(a) },
				"input": func() error {
					return f.run.registeredInput(a, f.input).SendKeys(context.Background(), substrate.PaneID(a.Pane), substrate.Keys{Text: "must not send", Submit: true})
				},
			} {
				if err := check(); err == nil || !strings.Contains(err.Error(), "incomplete pane registration") {
					t.Errorf("%s accepted incomplete %s: %v", name, field, err)
				}
			}
			if f.input.registrations != 0 || f.input.submits != 0 || f.input.pasted != "" {
				t.Fatalf("unregistered input mutated pane: registrations=%d submits=%d text=%q", f.input.registrations, f.input.submits, f.input.pasted)
			}
		})
	}
}

func TestSandboxedOwnerAndLeadDeliverInBothDirections(t *testing.T) {
	f := newStateFixture(t)
	lead := f.add(t, "lead-id", "lead", "codex")
	owner := f.add(t, "owner-id", "owner", "codex")
	owner.Pane = "%2"
	p, err := f.run.team.Agent(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(owner); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	f.cmd.paneBackend = identityFixture{inputFixture: f.input, present: true, visible: false}
	f.input.processErr = errors.New("host process tree unavailable in private namespace")
	for _, direction := range []struct{ sender, recipient core.Agent }{{owner, lead}, {lead, owner}} {
		f.env["GANG_AGENT_ID"] = string(direction.sender.ID)
		f.env["TMUX_PANE"] = direction.sender.Pane
		f.env["GANGLINE_HITCH_ID"] = string(direction.recipient.ID)
		f.out.Reset()
		if err := f.cmd.send([]string{string(direction.recipient.Name), "registered delivery"}); err != nil {
			t.Fatalf("%s to %s: %v", direction.sender.Name, direction.recipient.Name, err)
		}
		p, err := f.run.team.Agent(direction.recipient.ID)
		if err != nil {
			t.Fatal(err)
		}
		a, err := p.Read()
		if err != nil {
			t.Fatal(err)
		}
		e, err := p.ReadEnvelope("cur", a.LastDelivered)
		if err != nil {
			t.Fatal(err)
		}
		if e.Outcome != "delivered" || e.From.Kind != core.SenderAgent || e.From.HitchID != direction.sender.ID || e.From.Name != direction.sender.Name || !strings.Contains(f.out.String(), "delivered") {
			t.Fatalf("wrong delivery or attribution: envelope=%+v output=%s", e, f.out.String())
		}
	}
	if f.input.submits != 2 || f.input.registrations != 0 {
		t.Fatalf("submits=%d registrations=%d", f.input.submits, f.input.registrations)
	}
}

func TestRegisteredPaneRequiresInheritedHitchID(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["TMUX_PANE"] = a.Pane
	if _, err := f.run.observedAgent(); err == nil || !strings.Contains(err.Error(), "inherited hitch identity") {
		t.Fatalf("pane inferred a caller without hitch ID: %v", err)
	}
	if err := f.cmd.send([]string{"worker", "--from", "operator", "must not send"}); err == nil || !strings.Contains(err.Error(), "inherited hitch identity") {
		t.Fatalf("missing hitch ID bypassed sender attribution: %v", err)
	}
	if f.input.submits != 0 {
		t.Fatalf("submitted without hitch ID: %d", f.input.submits)
	}
}
