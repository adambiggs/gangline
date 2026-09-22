package main

import (
	"errors"
	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecoveryNeverReplaysLiveOrAbandonedNativeInput(t *testing.T) {
	root := t.TempDir()
	run := &runtime{cmd: command{getenv: func(string) string { return "" }}, settings: settings{Session: "owner-test", StateRoot: root, Socket: filepath.Join(root, "absent.sock")}}
	now := time.Now()
	events := []core.Event{
		core.AdoptRequested{At: now, Pane: "%1", Hitch: core.Hitch{ID: "worker", Name: "worker", Collar: "codex", Directory: root}},
		core.TurnStarted{At: now, HitchID: "worker"},
		core.SendRequested{At: now, Deadline: now.Add(time.Minute), MidTurn: true, Envelope: core.Envelope{ID: "message", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, To: "worker", Message: core.Message{Text: "steer"}, CreatedAt: now}},
	}
	held := make(map[core.HitchID]*os.File)
	for _, event := range events {
		if _, _, _, err := run.appendEvent(event, held); err != nil {
			t.Fatal(err)
		}
	}
	effect := core.DeliverEnvelope{Envelope: events[2].(core.SendRequested).Envelope, Pane: "%1", Deadline: now.Add(time.Minute)}
	owner := held["worker"]
	if owner == nil {
		t.Fatal("published input intent has no owner")
	}
	if other, err := run.paths().LockInput(run.settings.Session, "worker"); !errors.Is(err, store.ErrLocked) {
		if other != nil {
			other.Close()
		}
		t.Fatalf("input intent visible without exclusive ownership: %v", err)
	}
	for i := 0; i < 2; i++ {
		state, err := run.recoverDelivery(effect)
		if err != nil || state.Deliveries["message"].Status != core.DeliveryDelivering {
			t.Fatalf("live owner disturbed: %v %v", state.Deliveries["message"], err)
		}
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	state, err := run.recoverDelivery(effect)
	if err != nil || state.Deliveries["message"].Status != core.DeliveryUnverified {
		t.Fatalf("abandoned owner retried: %v %v", state.Deliveries["message"], err)
	}
	if len(core.PendingEffects(state)) != 0 {
		t.Fatal("unknown input remains executable")
	}
}

func TestContendedInputIntentIsPublishedAsDeferred(t *testing.T) {
	root := t.TempDir()
	run := &runtime{settings: settings{Session: "owner-test", StateRoot: root}}
	now := time.Now()
	held := make(map[core.HitchID]*os.File)
	for _, event := range []core.Event{
		core.AdoptRequested{At: now, Pane: "%1", Hitch: core.Hitch{ID: "worker", Name: "worker", Collar: "codex", Directory: root}},
		core.TurnStarted{At: now, HitchID: "worker"},
	} {
		if _, _, _, err := run.appendEvent(event, held); err != nil {
			t.Fatal(err)
		}
	}
	owner, err := run.paths().LockInput(run.settings.Session, "worker")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	request := core.SendRequested{At: now, Deadline: now.Add(time.Minute), MidTurn: true, Envelope: core.Envelope{ID: "message", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, To: "worker", Message: core.Message{Text: "steer"}, CreatedAt: now}}
	_, state, effects, err := run.appendEvent(request, held)
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 0 || len(held) != 0 || state.Deliveries["message"].Status != core.DeliveryQueued || state.Hitches["worker"].Activity != core.ActivityBusy {
		t.Fatalf("contended input was not safely deferred: %#v, %v", state.Deliveries["message"], effects)
	}
	stored, err := run.load()
	if err != nil || stored.Deliveries["message"].Status != core.DeliveryQueued {
		t.Fatalf("observer saw in-flight input without owner: %v %v", stored.Deliveries["message"], err)
	}
}
