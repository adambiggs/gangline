package main

import (
	"os"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestInactiveRefusalsNameARoute(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	a = f.setAgent(t, a, func(a *core.Agent) { a.Status = core.Failed })
	f.cmd.stdin = strings.NewReader("hello")
	want := "recipient worker is failed, not active; gang status worker shows why"
	for _, test := range []struct {
		name string
		call func() error
	}{
		{"send", func() error { return f.cmd.send([]string{"worker", "--from", "operator"}) }},
		{"interrupt", func() error { return f.cmd.interrupt([]string{"worker"}) }},
		{"compact", func() error { return f.cmd.compact([]string{"worker"}) }},
		{"startup recovery", func() error { return f.run.recoverStartup("worker") }},
		{"compact --recover", func() error {
			f.setAgent(t, a, func(a *core.Agent) {
				a.Compaction = &core.Compaction{ID: "c", Status: "submitted", Continuation: true}
			})
			return f.cmd.compact([]string{"worker", "--recover"})
		}},
	} {
		if err := test.call(); err == nil || err.Error() != want {
			t.Errorf("%s to a failed agent = %v; want %q", test.name, err, want)
		}
	}
	f.add(t, "b", "caller", "codex")
	f.setAgent(t, f.agent(t, "b"), func(a *core.Agent) { a.Status = core.Dropping })
	f.setAgent(t, a, func(a *core.Agent) { a.Status = core.Active })
	f.env["GANG_AGENT_ID"] = "b"
	f.cmd.stdin = strings.NewReader("hello")
	if err := f.cmd.send([]string{"worker"}); err == nil || err.Error() != "hitch identity caller is dropping, not active; re-hitch this agent" {
		t.Errorf("send from a dropping identity = %v", err)
	}
	f.setAgent(t, f.agent(t, "b"), func(a *core.Agent) { a.Status, a.Pane = core.Active, "" })
	f.cmd.stdin = strings.NewReader("hello")
	if err := f.cmd.send([]string{"worker"}); err == nil || err.Error() != "hitch identity caller has no registered pane; re-hitch this agent" {
		t.Errorf("send from an identity without a pane = %v", err)
	}
	delete(f.env, "GANG_AGENT_ID")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	held, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	f.cmd.stdin = strings.NewReader("hello")
	if err := f.cmd.send([]string{"worker", "--from", "operator", "--supersede"}); err == nil || err.Error() != "recipient is busy with an input operation; retry" {
		t.Errorf("send to a locked recipient = %v", err)
	}
}

func TestIdentityRefusalsNameARoute(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	f.add(t, "b", "caller", "codex")
	send := func() error {
		f.cmd.stdin = strings.NewReader("hello")
		return f.cmd.send([]string{"worker"})
	}
	for _, test := range []struct{ id, want string }{
		{"../x", `hitch identity "../x" is not registered in team unit; re-hitch this agent, or unset GANG_AGENT_ID to act as the operator`},
		{"c", `hitch identity "c" is not registered in team unit; re-hitch this agent, or unset GANG_AGENT_ID to act as the operator`},
	} {
		f.env["GANG_AGENT_ID"] = test.id
		if err := send(); err == nil || err.Error() != test.want {
			t.Errorf("send as %q = %v; want %q", test.id, err, test.want)
		}
	}
	f.env["GANG_AGENT_ID"] = "b"
	f.env["TMUX_PANE"] = "%99"
	if err := send(); err == nil || err.Error() != "hitch identity caller is registered to pane %1, not this pane %99; run gang from its pane, or unset GANG_AGENT_ID to act as the operator" {
		t.Errorf("send from another pane = %v", err)
	}
	p, err := f.run.team.Agent("b")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.State, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := send(); err == nil || !strings.HasPrefix(err.Error(), "hitch identity b cannot be read: decode ") {
		t.Errorf("send from an undecodable identity = %v", err)
	}
}
