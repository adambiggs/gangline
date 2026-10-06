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

func TestCurfewDeadlineNotifiesWithoutDropping(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	paths, _ := f.run.team.Agent(a.ID)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	deadline := f.cmd.now().Add(time.Hour)
	if err := paths.Publish(core.Envelope{ID: "pending", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "lead"}, Message: core.Message{Text: "later"}, CreatedAt: f.cmd.now(), NotBefore: deadline.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.execute([]string{"curfew", "1h"}); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.execute([]string{"tick"}); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 {
		t.Fatal("curfew notice before deadline")
	}
	f.cmd.clock = func() time.Time { return deadline }
	for range 2 {
		if err := f.cmd.execute([]string{"tick"}); err != nil {
			t.Fatal(err)
		}
	}
	current, err := paths.Read()
	if err != nil || current.Status != core.Active {
		t.Fatalf("curfew changed registration: %+v, %v", current, err)
	}
	want := "Team unit curfew " + deadline.Format(time.RFC3339) + " has passed. Observed " + deadline.Format(time.RFC3339) + "."
	if f.input.submits != 1 || !strings.Contains(f.input.pasted, want) {
		t.Fatalf("curfew notice submits=%d text=%q, want %q", f.input.submits, f.input.pasted, want)
	}
	pending, err := paths.ListNew()
	if err != nil || len(pending) != 1 || pending[0].ID != "pending" {
		t.Fatalf("curfew changed pending work: %+v, %v", pending, err)
	}
	// Settling ordinary input removes the previous receipt. A new runtime
	// still knows this deadline was already announced.
	if err := paths.Publish(core.Envelope{ID: "ordinary", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "ordinary"}, CreatedAt: deadline}); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.execute([]string{"tick"}); err != nil {
		t.Fatal(err)
	}
	restarted, err := f.cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 2 {
		t.Fatalf("curfew repeated after ordinary delivery and restart: %d submits", f.input.submits)
	}
	log, err := os.Open(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	if err := store.ReadLog(log, func(e core.Event) error {
		if e.Type == "drop_started" || e.Type == "delivery_failed" {
			t.Fatalf("curfew performed teardown: %+v", e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPassedCurfewAllowsStartupClaim(t *testing.T) {
	for _, args := range [][]string{{"up", "-c", "codex"}, {"up", "captain", "-c", "codex"}, {"hitch", "worker", "-c", "codex"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			f, _ := stoppedClaimFixture(t, "old")
			if err := f.run.team.WriteTeam(core.Team{Curfew: f.cmd.now()}); err != nil {
				t.Fatal(err)
			}
			err := f.cmd.execute(args)
			if err == nil || !strings.Contains(err.Error(), "fixture-reached-new-session") {
				t.Fatalf("startup did not reach pane creation: %v", err)
			}
		})
	}
}

func TestStartupCrossingCurfewAllowsClaim(t *testing.T) {
	f, _ := stoppedClaimFixture(t, "old")
	now := f.cmd.now()
	deadline := now.Add(time.Hour)
	f.cmd.clock = func() time.Time { return now }
	if err := f.run.team.WriteTeam(core.Team{Curfew: deadline}); err != nil {
		t.Fatal(err)
	}
	f.cmd.getwd = func() (string, error) {
		now = deadline
		return f.env["GANG_STATE_ROOT"], nil
	}
	err := f.cmd.execute([]string{"up", "-c", "codex"})
	if err == nil || !strings.Contains(err.Error(), "fixture-reached-new-session") {
		t.Fatalf("expired startup did not reach pane creation: %v", err)
	}
}

func TestStoppedTeamIgnoresPassedCurfew(t *testing.T) {
	f := newStateFixture(t)
	if err := f.run.team.WriteTeam(core.Team{Curfew: f.cmd.now()}); err != nil {
		t.Fatal(err)
	}
	err := f.run.stoppedTeamError()
	requireRefused(t, err, "no team")
	if strings.Contains(err.Error(), "curfew") {
		t.Fatalf("stopped team attributed to curfew: %v", err)
	}
}
