//go:build acceptance

package main

import (
	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestTeamIdleShortGapDoesNotNotify(t *testing.T) {
	f := newIdleFixture(t)
	f.reserve(t)
	f.work(t, "work", time.Time{})
	if n := f.reserve(t); n != nil {
		t.Fatal("single idle sweep notified during a short quiet gap")
	}
}

func TestTeamIdleWindowResetsOnInterveningEvidence(t *testing.T) {
	for _, mode := range []string{"busy-idle", "hook", "screen", "unknown", "gap", "backward", "minimum", "corrupt", "missing", "new-work"} {
		t.Run(mode, func(t *testing.T) {
			f := newIdleFixture(t)
			f.reserve(t)
			f.work(t, "work", time.Time{})
			f.reserve(t)
			for range 20 {
				f.advance(t, 5*time.Second)
				f.reserve(t)
			}
			p, _ := f.run.team.Agent(f.worker.ID)
			switch mode {
			case "busy-idle":
				l, err := p.LockAgent()
				if err != nil {
					t.Fatal(err)
				}
				a, err := p.Read()
				if err != nil {
					t.Fatal(err)
				}
				for _, activity := range []core.Activity{core.Busy, core.Idle} {
					if err := f.run.apply(l, &a, core.Event{Type: "activity_observed", Activity: activity, Reason: "fixture transition"}); err != nil {
						t.Fatal(err)
					}
				}
				_ = l.Close()
			case "hook":
				a, _ := p.Read()
				f.setAgent(t, a, func(a *core.Agent) { a.Native.HookSequence++ })
			case "screen":
				a, _ := p.Read()
				f.setAgent(t, a, func(a *core.Agent) { a.ScreenSince = f.cmd.now() })
			case "unknown":
				a, _ := p.Read()
				a = f.setAgent(t, a, func(a *core.Agent) { a.Activity = core.Unknown })
				f.reserve(t)
				f.setAgent(t, a, func(a *core.Agent) { a.Activity = core.Idle })
			case "gap":
				f.advance(t, 30*time.Second)
			case "backward":
				f.advance(t, -time.Second)
			case "minimum":
				f.run.settings.TeamIdleMinDuration = 90 * time.Second
			case "corrupt":
				if err := os.WriteFile(filepath.Join(f.run.team.Directory, "idle-window.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(filepath.Join(f.run.team.Directory, "idle-window.json")); err != nil {
					t.Fatal(err)
				}
			case "new-work":
				f.work(t, "next", time.Time{})
			}
			if n := f.reserve(t); n != nil {
				t.Fatal("changed evidence did not restart delay")
			}
			for elapsed := time.Duration(0); elapsed < f.run.settings.TeamIdleMinDuration-5*time.Second; elapsed += 5 * time.Second {
				f.advance(t, 5*time.Second)
				if n := f.reserve(t); n != nil {
					t.Fatal("notice before fresh minimum")
				}
			}
			f.advance(t, 5*time.Second)
			if n := f.reserve(t); n == nil {
				t.Fatal("fresh window never matured")
			}
		})
	}
}

func TestTeamIdleWindowSurvivesRestartAndLeadNoise(t *testing.T) {
	f := newIdleFixture(t)
	f.reserve(t)
	f.work(t, "work", time.Time{})
	f.reserve(t)
	for i := 1; i <= 24; i++ {
		f.advance(t, 5*time.Second)
		p, _ := f.run.team.Agent(f.lead.ID)
		a, err := p.Read()
		if err != nil {
			t.Fatal(err)
		}
		f.setAgent(t, a, func(a *core.Agent) {
			a.ChangedAt = f.cmd.now()
			a.ScreenSince = f.cmd.now()
			a.Native.HookSequence++
			a.Activity = core.Busy
		})
		if i == 12 {
			f.run, err = f.cmd.runtime()
			if err != nil {
				t.Fatal(err)
			}
		}
		n := f.reserve(t)
		if i < 24 && n != nil {
			t.Fatal("early notice")
		}
		if i == 24 && n == nil {
			t.Fatal("lead churn or restart stopped maturity")
		}
	}
}

func TestTeamIdleDisarmBeforeWindowCleanup(t *testing.T) {
	f := newIdleFixture(t)
	f.reserve(t)
	f.work(t, "work", time.Time{})
	n := f.mature(t)
	if n == nil {
		t.Fatal("no notice")
	}
	// Restore the stale cache as if the process exited after saving the notice
	// but before removing the cache. Durable disarm must win after restart.
	l, err := f.run.team.LockIdle()
	if err != nil {
		t.Fatal(err)
	}
	s, err := l.Read()
	if err != nil {
		t.Fatal(err)
	}
	p, _ := f.run.team.Agent(f.worker.ID)
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	w := &store.IdleWindow{Since: f.cmd.now().Add(-2 * time.Minute), Last: f.cmd.now(), Minimum: 2 * time.Minute, Lead: s.Lead, Members: []store.IdleWindowMember{{Member: s.Members[0], HookSequence: a.Native.HookSequence, ChangedAt: a.ChangedAt, ScreenSince: a.ScreenSince}}}
	if err := l.SaveWindow(w); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	if err := f.run.settleIdleNotice(n.Envelope, "delivered"); err != nil {
		t.Fatal(err)
	}
	f.run, err = f.cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	if got := f.reserve(t); got == nil || got.Envelope.ID != n.Envelope.ID {
		t.Fatal("stale cache repeated notice")
	}
	l, err = f.run.team.LockIdle()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if l.ReadWindow() != nil {
		t.Fatal("stale cache not removed")
	}
}

func TestTeamIdleMinimumSettingAndWatchdog(t *testing.T) {
	f, scheduler := watchdogFixture(t)
	f.cmd.lookupEnv = func(k string) (string, bool) { v, ok := f.env[k]; return v, ok }
	cfg, err := f.cmd.settings()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TeamIdleMinDuration != 2*time.Minute {
		t.Fatalf("default=%s", cfg.TeamIdleMinDuration)
	}
	if err := os.MkdirAll(cfg.ConfigDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ConfigDir, "config"), []byte("GANG_TEAM_IDLE_MIN_DURATION=3m\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = f.cmd.settings()
	if err != nil || cfg.TeamIdleMinDuration != 3*time.Minute || cfg.Origins["GANG_TEAM_IDLE_MIN_DURATION"] != "config" {
		t.Fatalf("config %+v %v", cfg, err)
	}
	f.env["GANG_TEAM_IDLE_MIN_DURATION"] = "45s"
	cfg, err = f.cmd.settings()
	if err != nil || cfg.TeamIdleMinDuration != 45*time.Second || cfg.Origins["GANG_TEAM_IDLE_MIN_DURATION"] != "env" {
		t.Fatalf("env %+v %v", cfg, err)
	}
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if got := scheduler.environment["GANG_TEAM_IDLE_MIN_DURATION"]; got != "45s" {
		t.Fatalf("watchdog=%s", got)
	}
	if !slices.Contains(watchdogEnvironmentKeys, "GANG_TEAM_IDLE_MIN_DURATION") {
		t.Fatal("scheduler drops setting")
	}
	for _, value := range []string{"", "0s", "-1m", "soon", "9999999999999999999h"} {
		f.env["GANG_TEAM_IDLE_MIN_DURATION"] = value
		if _, err := f.cmd.settings(); err == nil || !strings.Contains(err.Error(), "GANG_TEAM_IDLE_MIN_DURATION") {
			t.Fatalf("%q: %v", value, err)
		}
	}
}

func TestTeamIdleUnevaluatedSweepsDoNotExtendWindow(t *testing.T) {
	for _, mode := range []string{"incomplete", "contention"} {
		t.Run(mode, func(t *testing.T) {
			f := newIdleFixture(t)
			f.reserve(t)
			f.work(t, "work", time.Time{})
			f.reserve(t)
			for range 20 {
				f.advance(t, 5*time.Second)
				f.reserve(t)
			}
			for range 6 {
				f.advance(t, 5*time.Second)
				s := f.sweep(t)
				var closeLock func() error
				if mode == "incomplete" {
					delete(s.observed, f.worker.ID)
				} else {
					p, _ := f.run.team.Agent(f.worker.ID)
					l, err := p.LockAgent()
					if err != nil {
						t.Fatal(err)
					}
					closeLock = l.Close
				}
				err := f.run.reserveIdleNotice(s)
				if closeLock != nil {
					_ = closeLock()
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if n := f.reserve(t); n != nil {
				t.Fatal("unevaluated sweeps counted toward delay")
			}
			if n := f.mature(t); n == nil {
				t.Fatal("new continuous window did not mature")
			}
		})
	}
}
