package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func inputEvents(t *testing.T, f *stateFixture) []core.Event {
	t.Helper()
	file, err := os.Open(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var events []core.Event
	s := bufio.NewScanner(file)
	s.Buffer(nil, 1<<20)
	for s.Scan() {
		e, err := core.DecodeEvent(s.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(e.Type, "input_") {
			events = append(events, e)
		}
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func pendingFixture(t *testing.T, collar string) (*stateFixture, core.Agent, store.AgentPaths) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", collar)
	p, _ := f.run.team.Agent(a.ID)
	f.input.command = collar
	f.input.screen = screenWithText("screen temporarily has no input surface")
	e := core.Envelope{ID: "pending", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "continue"}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	return f, a, p
}

func TestPendingInputObservationOutage(t *testing.T) {
	// Codex fails during DetectCapacity, Claude during drain's inputState.
	for _, collar := range []string{"codex", "claude"} {
		t.Run(collar, func(t *testing.T) {
			f, a, p := pendingFixture(t, collar)
			now := f.cmd.now()
			start := now
			f.cmd.clock = func() time.Time { return now }
			f.run.cmd = f.cmd
			tick := func() {
				t.Helper()
				err := f.cmd.tick([]string{"--agent", "worker"})
				var ce commandError
				if !errors.As(err, &ce) || ce.status != exitUnknown || !strings.Contains(err.Error(), "input") || !strings.Contains(err.Error(), "queued") {
					t.Fatalf("pending diagnostic: %v", err)
				}
				if q := inboxNew(t, f, a); len(q) != 1 || q[0].ID != "pending" {
					t.Fatalf("queued input changed: %+v", q)
				}
				if f.input.pasted != "" || f.input.submits != 0 || len(f.input.keys) != 0 {
					t.Fatal("observation sent input")
				}
				if failures := tickFailures(t, f); len(failures) != 0 {
					t.Fatalf("pending reported as failure: %+v", failures)
				}
			}
			tick()
			tick()
			got, err := p.Read()
			if err != nil || got.InputOutage == nil || !got.InputOutage.Since.Equal(start) || got.InputOutage.Escalated {
				t.Fatalf("first marker: %+v %v", got.InputOutage, err)
			}
			now = start.Add(operationTimeout - time.Nanosecond)
			tick()
			if events := inputEvents(t, f); len(events) != 1 || events[0].Type != "input_pending" {
				t.Fatalf("duplicate/premature warning: %+v", events)
			}
			now = start.Add(operationTimeout)
			tick()
			tick()
			got, err = p.Read()
			if err != nil || got.InputOutage == nil || !got.InputOutage.Escalated {
				t.Fatalf("persistent marker: %+v %v", got.InputOutage, err)
			}
			events := inputEvents(t, f)
			if len(events) != 2 || events[1].Type != "input_observation_outage" {
				t.Fatalf("outage events: %+v", events)
			}
			// The escalation boundary has a measured fake-clock margin of 1ns.
			f.out.Reset()
			if err := f.cmd.status([]string{"worker", "--json"}); err != nil {
				t.Fatal(err)
			}
			var row agentJSON
			if err := json.Unmarshal([]byte(f.out.String()), &row); err != nil || row.InputOutage == nil || !row.InputOutage.Escalated {
				t.Fatalf("status marker: %s %v", f.out.String(), err)
			}
			if collar == "codex" {
				f.input.screen = screenWithText("› operator draft")
			} else {
				f.input.screen = screenWithText("────────", "❯ operator draft", "────────")
			}
			if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
				t.Fatal(err)
			}
			got, err = p.Read()
			if err != nil || got.InputOutage != nil {
				t.Fatalf("outage not cleared: %+v %v", got.InputOutage, err)
			}
			events = inputEvents(t, f)
			if len(events) != 3 || events[2].Type != "input_observation_recovered" {
				t.Fatalf("recovery events: %+v", events)
			}
			if f.input.pasted != "" || f.input.submits != 0 || len(inboxNew(t, f, a)) != 1 {
				t.Fatal("recovery altered queued input")
			}
		})
	}
}

func TestPendingInputForeignForegroundStaysLoud(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		t.Run(collar, func(t *testing.T) {
			f, _, p := pendingFixture(t, collar)
			f.input.command = "zsh"
			err := f.cmd.tick([]string{"--agent", "worker"})
			if err == nil || !strings.Contains(err.Error(), "foreground") {
				t.Fatalf("foreign foreground: %v", err)
			}
			failures := tickFailures(t, f)
			if len(failures) != 1 || !strings.Contains(failures[0].Reason, "foreground") {
				t.Fatalf("foreign foreground hidden: %+v", failures)
			}
			got, _ := p.Read()
			if got.InputOutage != nil || len(inputEvents(t, f)) != 0 {
				t.Fatal("foreign foreground marked pending")
			}
		})
	}
}

func TestPendingInputDoesNotClassifyUnsafeErrors(t *testing.T) {
	f, a, p := pendingFixture(t, "codex")
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, err := loadCollar(a.Collar, f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, original := range []error{harness.ErrForeignComposer, harness.ErrComposerOccupied, errors.Join(harness.ErrNoComposer, errors.New("I/O failure"))} {
		if got := f.run.pendingInput(l, &a, f.input, c, original); got != original {
			t.Fatalf("unsafe error reclassified: %v", got)
		}
	}
	a.Input = &core.InputIntent{ID: "pending"}
	if err := f.run.pendingInput(l, &a, f.input, c, harness.ErrNoComposer); err != harness.ErrNoComposer {
		t.Fatalf("post-input error reclassified: %v", err)
	}
	mixed := errors.Join(&pendingInputError{reason: "pending"}, errors.New("I/O failure"))
	if err := f.run.tickFailed("command", a.ID, mixed); err == nil || len(tickFailures(t, f)) != 1 {
		t.Fatal("mixed failure hidden")
	}
}

func TestPendingInputUnlockFailureStaysLoud(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		t.Run(collar, func(t *testing.T) {
			f, a, p := pendingFixture(t, collar)
			sentinel := errors.New("detach I/O failure")
			f.run.cmd.detach = func(string, hookNotice) error { return sentinel }
			f.run.wake = []core.HitchID{"other"}
			err := f.run.tickAgent(a.ID, hookNotice{}, false)
			if !errors.Is(err, sentinel) {
				t.Fatalf("unlock failure lost: %v", err)
			}
			err = f.run.tickFailed("command", a.ID, err)
			if err == nil || len(tickFailures(t, f)) != 1 {
				t.Fatal("unlock failure softened")
			}
			got, _ := p.Read()
			if got.InputOutage == nil || len(inboxNew(t, f, a)) != 1 {
				t.Fatal("queued input or marker lost")
			}
		})
	}
}
