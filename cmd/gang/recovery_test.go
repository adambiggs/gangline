package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestKilledInputOwnerNeverRetypes(t *testing.T) {
	if root := os.Getenv("GANGLINE_KILLED_OWNER_ROOT"); root != "" {
		team, err := (store.Paths{Root: root}).Team("unit")
		if err != nil {
			t.Fatal(err)
		}
		p, _ := team.Agent("a")
		l, err := p.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		a, err := p.Read()
		if err != nil {
			t.Fatal(err)
		}
		a.Input = &core.InputIntent{ID: "crashed", Kind: "envelope", At: time.Now()}
		if err := l.Save(a); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(os.Stdout, "intent saved")
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	e := core.Envelope{ID: "crashed", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "once"}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(exe, "-test.run=^TestKilledInputOwnerNeverRetypes$")
	child.Dir = f.env["GANG_STATE_ROOT"]
	child.Env = append(os.Environ(), "GANGLINE_KILLED_OWNER_ROOT="+child.Dir)
	child.Stderr = os.Stderr
	out, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	ready, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || ready != "intent saved\n" {
		t.Fatalf("owner barrier: %q %v", ready, err)
	}
	state, err := p.Read()
	if err != nil || state.Input == nil {
		t.Fatalf("child did not save intent: %+v %v", state, err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err == nil {
		t.Fatal("owner was not killed")
	}
	for range 2 {
		if _, err := f.run.drain(a.ID, ""); err != nil {
			t.Fatal(err)
		}
	}
	got, err := p.ReadEnvelope("failed", e.ID)
	if err != nil {
		t.Fatal(err)
	}
	state, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != "unverified" || state.Input != nil || f.input.submits != 0 {
		t.Fatalf("recovery: %+v %+v submits=%d", got, state.Input, f.input.submits)
	}
}

type fakeDeadline struct {
	context.Context
	at time.Time
}

func (c fakeDeadline) Deadline() (time.Time, bool) { return c.at, true }

type waitFixture struct{ wait func(context.Context) error }

func (w waitFixture) Wait(ctx context.Context) error { return w.wait(ctx) }
func (w waitFixture) Close() error                   { return nil }

func TestWitnessTimeoutUsesTheDeadlineAndNeverRetypes(t *testing.T) {
	for _, budget := range []time.Duration{time.Millisecond, time.Second, time.Hour} {
		t.Run(budget.String(), func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			start := f.cmd.now()
			now := start
			calls := 0
			cmd := f.cmd
			cmd.clock = func() time.Time { return now }
			cmd.newTimeout = func(ctx context.Context, requested time.Duration) (context.Context, context.CancelFunc) {
				if requested != operationTimeout {
					t.Fatalf("delivery budget=%s", requested)
				}
				return fakeDeadline{ctx, start.Add(budget)}, func() {}
			}
			cmd.newWatch = func(string) (changeWait, error) {
				return waitFixture{func(ctx context.Context) error {
					deadline, ok := ctx.Deadline()
					if !ok {
						t.Fatal("witness wait has no deadline")
					}
					calls++
					if calls == 1 {
						now = deadline.Add(-time.Nanosecond)
						return nil
					}
					now = now.Add(time.Nanosecond)
					if !now.Equal(deadline) {
						t.Fatalf("deadline margin=%s", now.Sub(deadline))
					}
					return context.DeadlineExceeded
				}}, nil
			}
			f.input.submit = func(string) error { return nil }
			cmd.stdin = strings.NewReader("only once")
			err := cmd.send([]string{"worker", "--from", "operator"})
			var ce commandError
			if !errors.As(err, &ce) || ce.status != exitUnknown {
				t.Fatalf("timeout result: %v", err)
			}
			a, err = p.Read()
			if err != nil {
				t.Fatal(err)
			}
			got, err := p.ReadEnvelope("failed", a.LastFailed)
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome != "unverified" || !strings.Contains(got.Reason, "deadline exceeded") || calls != 2 {
				t.Fatalf("timeout: %+v calls=%d", got, calls)
			}
			if _, err := f.run.drain(a.ID, ""); err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 1 {
				t.Fatalf("submits=%d", f.input.submits)
			}
			// All scales stop exactly at the fake deadline, with a 1ns pre-expiry margin.
			if now.Sub(start) != budget {
				t.Fatalf("elapsed=%s budget=%s", now.Sub(start), budget)
			}
		})
	}
}

func TestCompactionPublicationRecoveryCancelsUnsubmittedContinuation(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Compaction = &core.Compaction{ID: "c", Resume: core.Message{Text: "continue"}, StartedAt: f.cmd.now(), Deadline: f.cmd.now().Add(time.Minute), Status: "completed", CompletedAt: f.cmd.now().Add(time.Second)}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	// Stop at the publication boundary, before its state acknowledgement.
	e := core.Envelope{ID: "resume-c", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "compact"}, Message: a.Compaction.Resume, CreatedAt: f.cmd.now().Add(-time.Second)}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_HITCH_ID"] = "a"
	cmd := f.cmd
	cmd.stdin = strings.NewReader("next")
	if err := cmd.send([]string{"worker", "--from", "operator"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Inbox, "cur", "resume-c.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("test did not retire prior receipt: %v", err)
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	resume, err := p.ReadEnvelope("failed", "resume-c")
	if err != nil || got.Compaction.Status != "failed" || resume.Outcome != "cancelled" || f.input.submits != 1 {
		t.Fatalf("recovery sent continuation late: %+v, %v; status=%s submits=%d", resume, err, got.Compaction.Status, f.input.submits)
	}
}

func TestCompactionPublicationAcknowledgedBeforeInputIsCancelled(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	a.Compaction = &core.Compaction{ID: "c", Resume: core.Message{Text: "continue"}, ResumeToken: "aaaaaaaaaaaaaaaa", StartedAt: f.cmd.now(), Deadline: f.cmd.now().Add(time.Minute), CompletedAt: f.cmd.now().Add(time.Second), Status: "completed", Continuation: true}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: "resume-c", Token: a.Compaction.ResumeToken, Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "compact"}, Message: a.Compaction.Resume, Purpose: "resume", CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	l.Close()
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	resume, err := p.ReadEnvelope("failed", e.ID)
	if err != nil || got.Compaction.Status != "failed" || resume.Outcome != "cancelled" || f.input.submits != 0 {
		t.Fatalf("recovery submitted an unqueued resume: %+v, %v; status=%s submits=%d", resume, err, got.Compaction.Status, f.input.submits)
	}
}

func TestSchedulesBecomeDueOnTick(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = "a"
	cmd := f.cmd
	cmd.stdin = strings.NewReader("later")
	if err := cmd.send([]string{"worker", "--from", "operator", "--at", "1m"}); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 {
		t.Fatal("scheduled message submitted early")
	}
	now := f.cmd.now().Add(time.Minute)
	f.run.cmd.clock = func() time.Time { return now }
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 {
		t.Fatalf("due message not delivered: %d", f.input.submits)
	}
}

func TestWaitChecksExpiredBootDeadline(t *testing.T) {
	f := newStateFixture(t)
	f.input.screen = screenWithText("still launching")
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Status = core.Booting
	a.BootDeadline = f.cmd.now().Add(-time.Second)
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	err = f.cmd.wait([]string{"worker", "--timeout", "0s"})
	var ce commandError
	if !errors.As(err, &ce) || ce.status != exitRefused {
		t.Fatalf("wait result: %v", err)
	}
	a, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != core.Failed {
		t.Fatalf("expired boot retained: %+v", a)
	}
}

func TestAutomaticCompactionInvalidatesStaleContext(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	old := now.Add(-time.Second)
	next := now.Add(time.Second)
	percent := 50.0
	n := core.NativeState{Context: core.Reading{Kind: "context", At: &old, Status: "observed", Percent: &percent}}
	acceptReadings(&n, []core.Reading{{Kind: "compaction-checkpoint", At: &now, Source: "session-log"}, {Kind: "context", At: &old, Status: "observed", Percent: &percent}})
	if n.Context.Status != "unknown" || !n.CompactedAt.Equal(now) {
		t.Fatalf("stale context survived: %+v", n)
	}
	acceptReadings(&n, []core.Reading{{Kind: "context", At: &next, Status: "observed", Percent: &percent}})
	if n.Context.Status != "observed" {
		t.Fatalf("new native context rejected: %+v", n)
	}
	acceptReadings(&n, []core.Reading{{Kind: "compaction-finished", At: &now, Source: "native-hook"}})
	if n.Context.Status != "observed" || !n.Context.At.Equal(next) {
		t.Fatalf("delayed hook discarded newer context: %+v", n)
	}
}

func TestCapacityRecoveryIgnoresFutureSchedules(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	f.env["GANGLINE_HITCH_ID"] = "a"
	f.input.screen = screenWithText("› solve the task", "■ Selected model is at capacity. Please try a different model.", "", "› ")
	e := core.Envelope{ID: "future", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "later"}, CreatedAt: f.cmd.now(), NotBefore: f.cmd.now().Add(time.Hour)}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	f.run.cmd.clock = func() time.Time { return a.Capacity.NextAt }
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 {
		t.Fatalf("future schedule suppressed recovery: %d submits", f.input.submits)
	}
	if !strings.Contains(f.input.pasted, "[gang:gangline:capacity-recovery#") {
		t.Fatalf("capacity recovery sender: %q", f.input.pasted)
	}
	if !regexp.MustCompile(`\[gang:gangline:capacity-recovery#[0-9a-f]{16}\]`).MatchString(f.input.pasted) || strings.Contains(f.input.pasted, "capacity-"+a.Capacity.Fingerprint) {
		t.Fatalf("capacity recovery leaked store ID: %q", f.input.pasted)
	}
	pending, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID != "future" {
		t.Fatalf("scheduled message changed: %+v", pending)
	}
}

func TestRenameReleaseHandsOffConcurrentSend(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	e := core.Envelope{ID: "arrival", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "hello"}, CreatedAt: f.cmd.now()}
	cmd := f.cmd
	cmd.afterUnlock = func() {
		if err := p.Publish(e); err != nil {
			t.Fatal(err)
		}
	}
	called := 0
	cmd.detach = func(id string, n hookNotice) error {
		called++
		if id != "a" {
			t.Fatalf("wrong target %s", id)
		}
		return nil
	}
	if err := cmd.rename([]string{"worker", "renamed"}); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("arrival stranded: detached ticks=%d", called)
	}
}

func TestStatusMarksExpiredDeadlineFromTheObservedWindowTitle(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Status = core.Booting
	a.BootDeadline = f.cmd.now().Add(-time.Second)
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	fake := f.env["GANG_TMUX"]
	script := `#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
case "$1" in
 list-panes) printf '%%1\t?worker?\n'; printf 'listed\n' >> "$(dirname "$0")/listed";;
 capture-pane) printf 'still launching\n';;
 display-message) printf '0,0,0\n';;
 rename-window) printf '%s\n' "$*" > "$(dirname "$0")/marked";;
 *) exit 91;;
esac
`
	if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := f.cmd
	cmd.inputBackend = nil
	if err := cmd.status([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
	marked, err := os.ReadFile(filepath.Join(filepath.Dir(fake), "marked"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(marked), "!worker!") {
		t.Fatalf("expired window title: %s", marked)
	}
	listed, err := os.ReadFile(filepath.Join(filepath.Dir(fake), "listed"))
	if err != nil {
		t.Fatal(err)
	}
	if string(listed) != "listed\n" {
		t.Fatalf("expected one listing, got %q", listed)
	}
}
