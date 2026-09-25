package main

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestQueueCommandListsPendingMessages(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "a", "worker", "codex")
	other := f.add(t, "b", "other", "codex")
	for _, item := range []struct {
		agent core.Agent
		id    core.EnvelopeID
		from  core.AgentName
	}{
		{worker, "first", "lead"},
		{other, "second", "operator"},
	} {
		paths, err := f.run.team.Agent(item.agent.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := paths.Publish(core.Envelope{ID: item.id, Recipient: item.agent.ID, To: item.agent.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: item.from}, Message: core.Message{Text: "pending"}, CreatedAt: f.cmd.now()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.cmd.execute([]string{"queue", "worker"}); err != nil {
		t.Fatal(err)
	}
	if got := f.out.String(); got != "first\tworker\tlead\n" {
		t.Fatalf("targeted queue = %q", got)
	}
	f.out.Reset()
	if err := f.cmd.execute([]string{"queue"}); err != nil {
		t.Fatal(err)
	}
	if got := f.out.String(); got != "second\tother\toperator\nfirst\tworker\tlead\n" {
		t.Fatalf("team queue = %q", got)
	}
	if err := f.cmd.execute([]string{"queue", "worker", "other"}); err == nil || !strings.Contains(err.Error(), "expected at most one agent") {
		t.Fatalf("extra queue argument: %v", err)
	}
}
