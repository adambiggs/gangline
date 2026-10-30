//go:build acceptance

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

type processVisibilityFixture struct {
	identityFixture
	visibilityErr error
}

func (b processVisibilityFixture) ProcessVisibility(context.Context, substrate.PaneID) (bool, error) {
	return b.visible, b.visibilityErr
}

func TestProcessAvailabilityUsesCurrentObservation(t *testing.T) {
	failure := errors.New("process observation failed")
	for _, tc := range []struct {
		name                            string
		marker, visible, absent, noPane bool
		checkErr, visibilityErr         error
		available                       bool
		wantErr                         error
	}{
		{name: "visible with stale marker", marker: true, visible: true, available: true},
		{name: "visible without marker", visible: true, available: true},
		{name: "hidden with marker", marker: true},
		{name: "hidden without marker"},
		{name: "absent pane", absent: true, visible: true},
		{name: "replaced pane", checkErr: tmux.ErrPaneReplaced, visible: true},
		{name: "no pane", noPane: true, visible: true},
		{name: "exited process", visibilityErr: &substrate.ExitedError{}},
		{name: "registration error", checkErr: failure, wantErr: failure},
		{name: "observation error", visibilityErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			if tc.noPane {
				a.Pane = ""
			}
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(p.Directory, "process-unavailable")
			if tc.marker {
				if err := os.WriteFile(marker, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			f.cmd.paneBackend = processVisibilityFixture{identityFixture{f.input, !tc.absent, tc.visible, tc.checkErr, nil}, tc.visibilityErr}
			run, err := f.cmd.runtime()
			if err != nil {
				t.Fatal(err)
			}
			row, err := run.agentRow(a)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v, want %v", err, tc.wantErr)
			}
			if err == nil && row.ProcessAvailable != tc.available {
				t.Fatalf("process available=%v, want %v", row.ProcessAvailable, tc.available)
			}
			_, err = os.Stat(marker)
			if (err == nil) != tc.marker {
				t.Fatalf("observation changed marker: %v", err)
			}
		})
	}
}

func TestRosterAndStatusShowCurrentProcessAvailability(t *testing.T) {
	for _, visible := range []bool{false, true} {
		for _, args := range [][]string{{"roster"}, {"roster", "--json"}, {"status", "worker", "--json"}} {
			t.Run(strings.Join(args, " ")+"/visible="+map[bool]string{false: "false", true: "true"}[visible], func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", "codex")
				p, err := f.run.team.Agent(a.ID)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(p.Directory, "process-unavailable"), nil, 0600); err != nil {
					t.Fatal(err)
				}
				f.cmd.paneBackend = identityFixture{f.input, true, visible, nil, nil}
				if err := f.cmd.execute(args); err != nil {
					t.Fatal(err)
				}
				if len(args) == 1 {
					if got := strings.Contains(f.out.String(), "[process-unavailable]"); got == visible {
						t.Fatalf("roster=%q", f.out)
					}
				} else {
					var row agentJSON
					if args[0] == "roster" {
						var roster rosterJSON
						decodeOutput(t, f, &roster)
						if len(roster.Agents) != 1 {
							t.Fatalf("roster=%+v", roster)
						}
						row = roster.Agents[0]
					} else {
						decodeOutput(t, f, &row)
					}
					if row.ProcessAvailable != visible {
						t.Fatalf("row=%+v", row)
					}
				}
			})
		}
	}
}
