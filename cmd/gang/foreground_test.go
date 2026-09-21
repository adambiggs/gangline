package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
)

type foregroundBackend struct {
	processes []substrate.Process
	screen    substrate.Screen
	sends     int
}

func (backend *foregroundBackend) Capture(context.Context, substrate.PaneID) (substrate.Screen, error) {
	if len(backend.screen.Rows) != 0 {
		return backend.screen, nil
	}
	return substrate.Screen{Rows: [][]substrate.Cell{{{Text: "›"}, {Text: " "}}}}, nil
}

func TestDeliveryRecordsBlockedSurfaceWithoutInput(t *testing.T) {
	now := time.Now()
	hitch := core.Hitch{
		ID: "h-1", Name: "worker", Collar: "codex", Directory: "/work",
		Status: core.HitchActive, Activity: core.ActivityDelivering, Pane: "%1",
	}
	envelope := core.Envelope{
		ID: "e-1", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"},
		To: hitch.Name, Message: core.Message{Text: "hello"}, CreatedAt: now,
	}
	state := core.NewState(core.Team{ID: "team", Name: "team"})
	state.Hitches[hitch.ID] = hitch
	backend := &foregroundBackend{
		processes: []substrate.Process{{PID: 42, Command: "codex"}},
		screen:    screenWithText("Would you like to run the following command?", "› 1. Yes, proceed"),
	}
	run := &runtime{settings: settings{}}
	event, err := run.deliver(state, backend, core.DeliverEnvelope{Envelope: envelope, Pane: hitch.Pane, Deadline: now.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	blocked, ok := event.(core.BlockedDetected)
	if !ok || blocked.HitchID != hitch.ID || blocked.Evidence == "" {
		t.Fatalf("event = %#v, want blocked detection", event)
	}
	if backend.sends != 0 {
		t.Fatalf("send count = %d, want zero", backend.sends)
	}
}

func screenWithText(lines ...string) substrate.Screen {
	rows := make([][]substrate.Cell, len(lines))
	for row, line := range lines {
		for _, char := range line {
			rows[row] = append(rows[row], substrate.Cell{Text: string(char)})
		}
	}
	return substrate.Screen{Rows: rows}
}

func (backend *foregroundBackend) ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error) {
	return backend.processes, nil
}

func (backend *foregroundBackend) SendKeys(context.Context, substrate.PaneID, substrate.Keys) error {
	backend.sends++
	return nil
}

func TestDeliveryRefusesReplacementForegroundProcessBeforeInput(t *testing.T) {
	now := time.Now()
	hitch := core.Hitch{
		ID: "h-1", Name: "worker", Collar: "codex", Directory: "/work",
		Status: core.HitchActive, Activity: core.ActivityDelivering, Pane: "%1",
	}
	envelope := core.Envelope{
		ID: "e-1", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"},
		To: hitch.Name, Message: core.Message{Text: "hello"}, CreatedAt: now,
	}
	state := core.NewState(core.Team{ID: "team", Name: "team"})
	state.Hitches[hitch.ID] = hitch
	backend := &foregroundBackend{processes: []substrate.Process{{PID: 42, Command: "sh"}}}
	run := &runtime{settings: settings{}}

	event, err := run.deliver(state, backend, core.DeliverEnvelope{
		Envelope: envelope, Pane: hitch.Pane, Deadline: now.Add(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	deferred, ok := event.(core.DeliveryDeferred)
	if !ok || !strings.Contains(deferred.Reason, "pane foreground") {
		t.Fatalf("event = %#v, want recorded foreground refusal", event)
	}
	if backend.sends != 0 {
		t.Fatalf("send count = %d, want zero", backend.sends)
	}
}
