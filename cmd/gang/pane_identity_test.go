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

type replacedNativeFixture struct{ *inputFixture }

func (b replacedNativeFixture) Identity(context.Context, substrate.PaneID) (tmux.Identity, error) {
	return tmux.Identity{PID: 7, Started: "replacement", BootID: "fixture", Namespace: "fixture"}, nil
}

func TestLegacySenderRejectsReusedPane(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["TMUX_PANE"] = a.Pane
	f.cmd.paneBackend = replacedNativeFixture{f.input}
	run, err := f.cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.observedAgent(); err == nil || !strings.Contains(err.Error(), "native identity") {
		t.Fatalf("reused pane accepted: %v", err)
	}
}

func TestLegacySandboxOperationsKeepRegistrationAndInbox(t *testing.T) {
	for _, op := range []string{"send", "drop"} {
		t.Run(op, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			f.cmd.paneBackend = identityFixture{inputFixture: f.input, present: true, visible: false}
			var err error
			if op == "send" {
				err = f.cmd.send([]string{"worker", "--from", "operator", "do not queue"})
			} else {
				err = f.cmd.drop([]string{"worker"})
			}
			if err == nil {
				t.Fatal("unprovable legacy operation succeeded")
			}
			p, _ := f.run.team.Agent(a.ID)
			if _, err := p.Read(); err != nil {
				t.Fatalf("registration lost: %v", err)
			}
			pending, err := p.ListNew()
			if err != nil || len(pending) != 0 || f.input.submits != 0 {
				t.Fatalf("pending=%+v submits=%d err=%v", pending, f.input.submits, err)
			}
		})
	}
}

func TestAdoptedPaneInputWorksButSandboxCallerIsRefused(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	a.Registration = core.PaneRegistration{Generation: strings.Repeat("a", 64), Session: "$1"}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.cmd.paneBackend = identityFixture{inputFixture: f.input, present: true, visible: false}
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	if err := f.cmd.send([]string{"worker", "--from", "operator", "registered target"}); err != nil {
		t.Fatal(err)
	}
	f.env["TMUX_PANE"] = a.Pane
	run, err := f.cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.observedAgent(); err == nil {
		t.Fatal("adoption provided sandbox caller authority without inherited capability")
	}
}
