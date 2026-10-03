package main

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// All waits advance this clock directly. The last attempt is at the budget,
// so exhaustion has zero fake-clock overshoot.
func lockClock(t *testing.T, f *stateFixture, during func(time.Duration)) *time.Duration {
	t.Helper()
	elapsed := new(time.Duration)
	start := f.cmd.now()
	f.cmd.lockClock = func() time.Time { return start.Add(*elapsed) }
	f.cmd.lockWait = func(d time.Duration) {
		if d <= 0 || d > 100*time.Millisecond {
			t.Fatalf("invalid backoff %s", d)
		}
		*elapsed += d
		if f.input.pasted != "" || f.input.submits != 0 || len(f.input.keys) != 0 {
			t.Fatal("input effect before lock acquisition")
		}
		during(*elapsed)
	}
	f.run.cmd = f.cmd
	return elapsed
}

func TestBoundedLockCommands(t *testing.T) {
	for _, operation := range []string{"compact", "clear", "supersede", "wake"} {
		for _, expires := range []bool{false, true} {
			t.Run(operation+map[bool]string{false: "/released", true: "/exhausted"}[expires], func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", "codex")
				p, _ := f.run.team.Agent(a.ID)
				l, err := p.TryLock()
				if err != nil {
					t.Fatal(err)
				}
				defer l.Close()
				// An occupied composer keeps compact and supersede safely queued.
				f.input.screen = screenWithText("› operator draft")
				sender := core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}
				e := core.Envelope{ID: "scheduled", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: sender, Message: core.Message{Text: "old"}, CreatedAt: f.cmd.now(), NotBefore: f.cmd.now().Add(time.Hour)}
				if err := p.Publish(e); err != nil {
					t.Fatal(err)
				}
				if operation == "wake" {
					f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(a.ID), a.Pane
					if err := f.run.withUsageState(func(s *usageState) error {
						s.Snoozes[string(a.ID)] = usageSnooze{ID: e.ID, Token: e.Token, CallerID: a.ID, CallerName: a.Name, RecipientID: a.ID, RecipientName: a.Name, At: e.NotBefore}
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				}
				elapsed := lockClock(t, f, func(d time.Duration) {
					if !expires && d >= 70*time.Millisecond {
						if err := l.Close(); err != nil {
							t.Fatal(err)
						}
					}
				})
				switch operation {
				case "compact":
					err = f.cmd.compact([]string{"worker"})
				case "clear":
					err = f.cmd.send([]string{"worker", "--from", "operator", "--clear"})
				case "supersede":
					err = f.cmd.send([]string{"worker", "--from", "operator", "--supersede", "--at", "2h", "new"})
				case "wake":
					err = f.cmd.snooze([]string{"--clear"})
				}
				if expires {
					var ce commandError
					if !errors.As(err, &ce) || ce.status != exitRefused || !strings.Contains(err.Error(), "busy") {
						t.Fatalf("exhaustion: %v", err)
					}
					if *elapsed != agentLockBudget {
						t.Fatalf("elapsed=%s budget=%s", *elapsed, agentLockBudget)
					}
					got, readErr := p.Read()
					if readErr != nil || got.Compaction != nil {
						t.Fatalf("state changed: %+v %v", got, readErr)
					}
					if q := inboxNew(t, f, a); len(q) != 1 || q[0].ID != e.ID {
						t.Fatalf("queue changed: %+v", q)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if *elapsed != 70*time.Millisecond {
						t.Fatalf("elapsed=%s want 70ms", *elapsed)
					}
					if operation == "compact" {
						got, _ := p.Read()
						if got.Compaction == nil || got.Compaction.Status != "queued" {
							t.Fatalf("compact not queued: %+v", got)
						}
					} else {
						for _, q := range inboxNew(t, f, a) {
							if q.ID == e.ID {
								t.Fatal("scheduled input retained")
							}
						}
						if operation == "wake" && len(usageSnapshot(t, f.run).Snoozes) != 0 {
							t.Fatal("wake retained")
						}
					}
				}
				if f.input.pasted != "" || f.input.submits != 0 || len(f.input.keys) != 0 {
					t.Fatal("lock retry caused input")
				}
			})
		}
	}
}

func TestBoundedLockDropAndNameReuse(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "drop", true: "name reuse"}[reuse], func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "original", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			elapsed := lockClock(t, f, func(time.Duration) {
				if err := f.run.team.RemoveName(a.Name, a.ID); err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(p.Directory); err != nil {
					t.Fatal(err)
				}
				if err := l.Close(); err != nil {
					t.Fatal(err)
				}
				if reuse {
					f.add(t, "replacement", "worker", "codex")
				}
			})
			err = f.cmd.compact([]string{"worker"})
			if !errors.Is(err, os.ErrNotExist) || *elapsed != 10*time.Millisecond {
				t.Fatalf("drop result: %v elapsed=%s", err, *elapsed)
			}
			if reuse {
				next, _ := f.run.team.Agent("replacement")
				got, err := next.Read()
				if err != nil || got.Compaction != nil {
					t.Fatalf("replacement touched: %+v %v", got, err)
				}
			}
			if f.input.pasted != "" || len(f.input.keys) != 0 {
				t.Fatal("input after drop")
			}
		})
	}
}

func TestBoundedLockRereadsPreconditions(t *testing.T) {
	for _, operation := range []string{"compact", "clear", "supersede"} {
		t.Run(operation, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			elapsed := lockClock(t, f, func(time.Duration) {
				a.Status = core.Dropping
				if err := l.Save(a); err != nil {
					t.Fatal(err)
				}
				if err := l.Close(); err != nil {
					t.Fatal(err)
				}
			})
			if operation == "compact" {
				err = f.cmd.compact([]string{"worker"})
			} else {
				args := []string{"worker", "--from", "operator", "--clear"}
				if operation == "supersede" {
					args = []string{"worker", "--from", "operator", "--supersede", "new"}
				}
				err = f.cmd.send(args)
			}
			if err == nil || !strings.Contains(err.Error(), "dropping") || *elapsed != 10*time.Millisecond {
				t.Fatalf("stale precondition: %v elapsed=%s", err, *elapsed)
			}
			if f.input.pasted != "" || len(f.input.keys) != 0 {
				t.Fatal("input despite dropping")
			}
		})
	}
}

func TestLiveOnlyLockDoesNotWait(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	lockClock(t, f, func(time.Duration) { t.Fatal("live-only waited") })
	if err := f.cmd.send([]string{"worker", "--from", "operator", "--live-only", "hello"}); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("live-only: %v", err)
	}
}

func TestBoundedLockReconciliationErrorIsNotRetried(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a.LastFailed = "missing"
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteWitness(store.Witness{ID: "w", At: f.cmd.now(), Prompt: "x"}); err != nil {
		t.Fatal(err)
	}
	elapsed := lockClock(t, f, func(time.Duration) {
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
	})
	if _, _, err := f.run.acquireBounded(a.ID); err == nil {
		t.Fatal("reconciliation succeeded with missing receipt")
	}
	if *elapsed != 10*time.Millisecond {
		t.Fatalf("reconciliation retried: %s", *elapsed)
	}
}
