package main

import (
	"errors"
	"github.com/adambiggs/gangline/core"
	"os"
	"strings"
	"testing"
)

func TestReplacementLeadCanDropEarlierLeadsChildren(t *testing.T) {
	for _, name := range []string{"lead", "coordinator"} {
		t.Run(name, func(t *testing.T) {
			f := newStateFixture(t)
			old := f.addHitched(t, "old-lead", "lead", "lead", "")
			child := f.addHitched(t, "child", "worker", "worker", old.ID)
			if err := f.cmd.drop([]string{"lead"}); err != nil {
				t.Fatal(err)
			}
			current := f.addHitched(t, "replacement-lead", name, "lead", "")
			f.env["GANG_AGENT_ID"] = string(current.ID)
			if err := f.cmd.drop([]string{"worker"}); err != nil {
				t.Fatalf("replacement lead could not drop earlier lead's child: %v", err)
			}
			p, err := f.run.team.Agent(child.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Read(); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("child registration remains: %v", err)
			}
		})
	}
}

func TestDropRefusalReportsCallerAuthority(t *testing.T) {
	for _, tc := range []struct{ role, hitcher, nonce, want string }{
		{"worker", "", "fixture-nonce", `role="worker", hitched_by="", pane_token_match=true`},
		{"lead", "old-owner", "fixture-nonce", `role="lead", hitched_by="old-owner", pane_token_match=true`},
		{"lead", "", "wrong-token", `role="lead", hitched_by="", pane_token_match=false`},
	} {
		t.Run(tc.want, func(t *testing.T) {
			f := newStateFixture(t)
			caller := f.addHitched(t, "caller", "lead", tc.role, core.HitchID(tc.hitcher))
			f.addHitched(t, "child", "worker", "worker", "absent-old-lead")
			f.env["GANG_AGENT_ID"], f.env["GANG_AGENT_NONCE"] = string(caller.ID), tc.nonce
			err := f.cmd.drop([]string{"worker"})
			requireRefused(t, err, "drop refused")
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("missing caller authority: %v", err)
			}
		})
	}
}
