package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestUpStartupTaskForResumedLead(t *testing.T) {
	for _, test := range []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "resume", args: []string{"--resume", "retained-session"}, want: "This is a resumed session. Re-read your brief and durable state, then continue your work or report that you are waiting for an assignment."},
		{name: "named resume", args: []string{"captain", "--resume", "retained-session"}, want: "This is a resumed session. Re-read your brief and durable state, then continue your work or report that you are waiting for an assignment."},
		{name: "supplied task", args: []string{"--resume", "retained-session", "--task", "Finish the pending change."}, want: "Finish the pending change."},
		{name: "short task", args: []string{"--resume", "retained-session", "-t", "Finish the pending change."}, want: "Finish the pending change."},
		{name: "empty task", args: []string{"--resume", "retained-session", "--task", ""}, want: "No assignment was supplied."},
		{name: "empty short task", args: []string{"--resume", "retained-session", "-t="}, want: "No assignment was supplied."},
		{name: "stdin task", args: []string{"--resume", "retained-session", "--stdin"}, stdin: "Read the saved report.", want: "Read the saved report."},
		{name: "fresh", want: "No assignment was supplied."},
	} {
		t.Run(test.name, func(t *testing.T) {
			name := "lead"
			if test.name == "named resume" {
				name = "captain"
			}
			f, _ := stoppedClaimFixture(t, name)
			t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "codex"))
			f.cmd.stdin = strings.NewReader(test.stdin)
			err := f.cmd.up(append(test.args, "-c", "codex"))
			if err == nil || !strings.Contains(err.Error(), "fixture-reached-new-session") {
				t.Fatalf("up = %v", err)
			}
			a, err := f.run.resolve(name)
			if err != nil {
				t.Fatal(err)
			}
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			envelopes, err := p.ListNew()
			if err != nil || len(envelopes) != 1 {
				t.Fatalf("startup envelopes = %+v, %v", envelopes, err)
			}
			e := envelopes[0]
			want := test.want
			purpose := "startup"
			if test.want != "No assignment was supplied." {
				want = "Assignment:\n\n" + want
				purpose = "assignment"
			}
			if e.Message.Text != want || e.Purpose != purpose || a.Role != "lead" {
				t.Fatalf("startup = %q (%s), role %q; want %q (%s), lead", e.Message.Text, e.Purpose, a.Role, want, purpose)
			}
		})
	}
}
