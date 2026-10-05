package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestCurfewCommandPersistsAndShowsDeadline(t *testing.T) {
	f := newStateFixture(t)
	if err := f.cmd.execute([]string{"curfew"}); err != nil {
		t.Fatal(err)
	}
	if got := f.out.String(); got != "clear\n" {
		t.Fatalf("initial curfew = %q", got)
	}
	f.out.Reset()
	if err := f.cmd.execute([]string{"curfew", "2h"}); err != nil {
		t.Fatal(err)
	}
	deadline := f.cmd.now().Add(2 * time.Hour)
	team, err := f.run.team.ReadTeam()
	if err != nil {
		t.Fatal(err)
	}
	if !team.Curfew.Equal(deadline) {
		t.Fatalf("stored curfew = %s, want %s", team.Curfew, deadline)
	}
	if err := f.cmd.execute([]string{"curfew"}); err != nil {
		t.Fatal(err)
	}
	if got := f.out.String(); got != deadline.Format(time.RFC3339)+"\n" {
		t.Fatalf("visible curfew = %q", got)
	}
	visible := strings.TrimSpace(f.out.String())
	if err := f.cmd.execute([]string{"curfew", visible}); err != nil {
		t.Fatalf("displayed deadline did not round-trip: %v", err)
	}
	team, err = f.run.team.ReadTeam()
	if err != nil {
		t.Fatal(err)
	}
	if got := team.Curfew.Format(time.RFC3339); got != visible {
		t.Fatalf("round-tripped curfew = %q, want %q", got, visible)
	}
	deadline, err = time.Parse(time.RFC3339, visible)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.execute([]string{"curfew", "invalid"}); err == nil {
		t.Fatal("invalid curfew accepted")
	}
	team, err = f.run.team.ReadTeam()
	if err != nil {
		t.Fatal(err)
	}
	if !team.Curfew.Equal(deadline) {
		t.Fatalf("invalid command changed curfew to %s", team.Curfew)
	}
	f.out.Reset()
	if err := f.cmd.execute([]string{"curfew", "clear"}); err != nil {
		t.Fatal(err)
	}
	team, err = f.run.team.ReadTeam()
	if err != nil {
		t.Fatal(err)
	}
	if !team.Curfew.IsZero() {
		t.Fatalf("clear left curfew %s", team.Curfew)
	}
	if err := f.cmd.execute([]string{"curfew"}); err != nil {
		t.Fatal(err)
	}
	if got := f.out.String(); got != "clear\n" {
		t.Fatalf("cleared curfew = %q", got)
	}
}

func TestCurfewCommandDeadlineDropsAgent(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	paths, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Publish(core.Envelope{ID: "pending", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "lead"}, Message: core.Message{Text: "later"}, CreatedAt: f.cmd.now(), NotBefore: f.cmd.now().Add(10 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.execute([]string{"curfew", "1h"}); err != nil {
		t.Fatal(err)
	}
	end := f.cmd.now().Add(2 * time.Hour)
	f.cmd.clock = func() time.Time { return end }
	if err := f.cmd.execute([]string{"tick"}); err != nil {
		t.Fatal(err)
	}
	agents, err := f.run.team.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 0 {
		t.Fatalf("curfew left agents %+v", agents)
	}
	log, err := os.Open(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	failed := false
	if err := store.ReadLog(log, func(event core.Event) error {
		if event.Type == "delivery_failed" && event.ID == "pending" && event.Reason == "recipient was dropped" {
			failed = true
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !failed {
		t.Fatal("curfew did not cancel the pending message")
	}
}

func TestPassedCurfewRefusesStartup(t *testing.T) {
	for _, args := range [][]string{{"up"}, {"up", "captain"}, {"hitch", "worker"}, {"up", "--recover"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			f := newStateFixture(t)
			deadline := f.cmd.now()
			if err := f.run.team.WriteTeam(core.Team{Curfew: deadline}); err != nil {
				t.Fatal(err)
			}
			err := f.cmd.execute(args)
			requireRefused(t, err, "curfew")
			for _, text := range []string{"unit", deadline.Format(time.RFC3339), "has passed", "gang curfew clear"} {
				if !strings.Contains(err.Error(), text) {
					t.Fatalf("refusal %q omits %q", err, text)
				}
			}
			agents, err := f.run.team.ListAgents()
			if err != nil || len(agents) != 0 {
				t.Fatalf("refused startup changed registrations: %+v %v", agents, err)
			}
			if err := f.cmd.execute([]string{"curfew", "clear"}); err != nil {
				t.Fatal(err)
			}
			team, err := f.run.team.ReadTeam()
			if err != nil || !team.Curfew.IsZero() {
				t.Fatalf("clear left curfew %+v: %v", team, err)
			}
		})
	}
}

func TestStartupRechecksCurfewBeforeClaim(t *testing.T) {
	f := newStateFixture(t)
	fakeCodexOnPath(t)
	now := f.cmd.now()
	f.cmd.clock = func() time.Time { return now }
	deadline := now.Add(time.Hour)
	if err := f.run.team.WriteTeam(core.Team{Curfew: deadline}); err != nil {
		t.Fatal(err)
	}
	f.cmd.getwd = func() (string, error) {
		now = deadline
		return f.env["GANG_STATE_ROOT"], nil
	}
	requireRefused(t, f.cmd.execute([]string{"up", "-c", "codex"}), "gang curfew clear")
	agents, err := f.run.team.ListAgents()
	if err != nil || len(agents) != 0 {
		t.Fatalf("expired startup claimed an agent: %+v %v", agents, err)
	}
}

func TestStoppedTeamExplainsPassedCurfew(t *testing.T) {
	f := newStateFixture(t)
	if err := f.run.team.WriteTeam(core.Team{Curfew: f.cmd.now()}); err != nil {
		t.Fatal(err)
	}
	requireRefused(t, f.run.stoppedTeamError(), "gang curfew clear")
}

func TestCurfewDropExplainsReason(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	deadline := f.cmd.now()
	if err := f.run.team.WriteTeam(core.Team{Curfew: deadline}); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.execute([]string{"tick"}); err != nil {
		t.Fatal(err)
	}
	want := "team curfew " + deadline.Format(time.RFC3339) + " has passed"
	if !strings.Contains(f.out.String(), "worker dropped: "+want) {
		t.Fatalf("drop output omits cause: %q", f.out.String())
	}
	log, err := os.Open(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	found := false
	if err := store.ReadLog(log, func(event core.Event) error {
		if event.Type == "drop_started" && event.HitchID == a.ID && event.Reason == want {
			found = true
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("drop audit omits curfew reason")
	}
}
