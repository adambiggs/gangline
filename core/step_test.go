package core

import (
	"reflect"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
var testDeadline = testNow.Add(time.Minute)

func TestStepHitchBootLifecycle(t *testing.T) {
	hitch := Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: "/work"}
	tests := []struct {
		name         string
		event        Event
		wantStatus   HitchStatus
		wantActivity HitchActivity
		wantEffect   Effect
	}{
		{
			name:       "hitch intent spawns",
			event:      HitchRequested{At: testNow, Hitch: hitch, BootDeadline: testDeadline},
			wantStatus: HitchStarting, wantActivity: ActivityUnknown,
			wantEffect: SpawnHitch{Hitch: Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: "/work", Status: HitchStarting, Activity: ActivityUnknown, BootDeadline: testDeadline}},
		},
		{
			name:       "spawn outcome begins boot observation",
			event:      HitchSpawned{At: testNow, HitchID: "h-1", Pane: "%1"},
			wantStatus: HitchBooting, wantActivity: ActivityUnknown,
			wantEffect: AwaitBoot{HitchID: "h-1", Pane: "%1", Deadline: testDeadline},
		},
		{
			name:       "readiness activates the hitch",
			event:      HitchReady{At: testNow, HitchID: "h-1"},
			wantStatus: HitchActive, wantActivity: ActivityIdle,
		},
	}

	state := NewState(Team{ID: "team-1", Name: "example"})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var effects []Effect
			state, effects = Step(state, test.event)
			hitch := state.Hitches["h-1"]
			if hitch.Status != test.wantStatus || hitch.Activity != test.wantActivity {
				t.Fatalf("hitch = %#v", hitch)
			}
			assertEffect(t, effects, test.wantEffect)
		})
	}
}

func TestStepDeliveryWaitsForTurnBoundary(t *testing.T) {
	state := twoActiveHitches(t)
	state, _ = Step(state, TurnStarted{At: testNow, HitchID: "worker-id"})
	envelope := Envelope{
		ID: "e-1", From: Sender{Kind: SenderAgent, Name: "lead", HitchID: "lead-id"},
		To: "worker", Message: Message{Text: "build it"}, CreatedAt: testNow,
	}

	state, effects := Step(state, SendRequested{At: testNow, Envelope: envelope, Deadline: testDeadline})
	assertEffect(t, effects, nil)
	if state.Deliveries["e-1"].Status != DeliveryQueued {
		t.Fatalf("delivery = %#v", state.Deliveries["e-1"])
	}

	state, effects = Step(state, TurnBoundaryReached{At: testNow, HitchID: "worker-id"})
	assertEffect(t, effects, DeliverEnvelope{Envelope: envelope, Pane: "%2", Deadline: testDeadline})
	if state.Hitches["worker-id"].Activity != ActivityDelivering {
		t.Fatalf("activity = %q", state.Hitches["worker-id"].Activity)
	}

	state, effects = Step(state, DeliveryDeferred{At: testNow, EnvelopeID: "e-1", Reason: "composer occupied"})
	assertEffect(t, effects, nil)
	if state.Deliveries["e-1"].Status != DeliveryQueued || state.Hitches["worker-id"].Activity != ActivityIdle {
		t.Fatalf("deferred state = %#v %#v", state.Deliveries["e-1"], state.Hitches["worker-id"])
	}

	state, effects = Step(state, DeliveryRetryRequested{At: testNow, EnvelopeID: "e-1"})
	assertEffect(t, effects, DeliverEnvelope{Envelope: envelope, Pane: "%2", Deadline: testDeadline})
	state, effects = Step(state, DeliverySucceeded{At: testNow, EnvelopeID: "e-1"})
	assertEffect(t, effects, nil)
	if state.Deliveries["e-1"].Status != DeliveryDelivered || state.Hitches["worker-id"].Activity != ActivityBusy {
		t.Fatalf("delivered state = %#v %#v", state.Deliveries["e-1"], state.Hitches["worker-id"])
	}
}

func TestStepBlockedHoldsDeliveryUntilClearBoundary(t *testing.T) {
	state := busyHitch(t)
	state, effects := Step(state, BlockedDetected{At: testNow, HitchID: "worker-id", Evidence: "approval required"})
	assertEffect(t, effects, nil)
	if hitch := state.Hitches["worker-id"]; hitch.Activity != ActivityBlocked || hitch.BlockedFrom != ActivityBusy {
		t.Fatalf("blocked hitch = %#v", hitch)
	}
	envelope := Envelope{
		ID: "e-blocked", From: Sender{Kind: SenderAgent, Name: "lead", HitchID: "lead-id"},
		To: "worker", Message: Message{Text: "hold this"}, CreatedAt: testNow,
	}
	state, effects = Step(state, SendRequested{At: testNow, Envelope: envelope, Deadline: testDeadline})
	assertEffect(t, effects, nil)
	if state.Deliveries[envelope.ID].Status != DeliveryQueued {
		t.Fatalf("blocked delivery = %#v", state.Deliveries[envelope.ID])
	}
	state, effects = Step(state, BlockedCleared{At: testNow, HitchID: "worker-id"})
	assertEffect(t, effects, nil)
	if state.Hitches["worker-id"].Activity != ActivityBusy {
		t.Fatalf("cleared activity = %q", state.Hitches["worker-id"].Activity)
	}
	assertEffect(t, effects, nil)
	state, effects = Step(state, TurnBoundaryReached{At: testNow, HitchID: "worker-id"})
	assertEffect(t, effects, DeliverEnvelope{Envelope: envelope, Pane: "%2", Deadline: testDeadline})
}

func TestStepBlockedDeliveryRetriesWhenDialogClears(t *testing.T) {
	state := deliveringHitch(t)
	envelope := state.Deliveries["e-1"].Envelope
	state, effects := Step(state, BlockedDetected{At: testNow, HitchID: "worker-id", Evidence: "another surface owns input"})
	assertEffect(t, effects, nil)
	if hitch := state.Hitches["worker-id"]; hitch.Activity != ActivityBlocked || hitch.BlockedFrom != ActivityIdle {
		t.Fatalf("blocked delivery hitch = %#v", hitch)
	}
	state, effects = Step(state, BlockedCleared{At: testNow, HitchID: "worker-id"})
	assertEffect(t, effects, DeliverEnvelope{Envelope: envelope, Pane: "%2", Deadline: testDeadline})
}

func TestStepCompactionRunsBeforeQueuedDelivery(t *testing.T) {
	state := twoActiveHitches(t)
	state, _ = Step(state, TurnStarted{At: testNow, HitchID: "worker-id"})
	envelope := Envelope{ID: "e-1", From: Sender{Kind: SenderSelfDeclared, Name: "operator"}, To: "worker", Message: Message{Text: "later"}, CreatedAt: testNow}
	state, _ = Step(state, SendRequested{At: testNow, Envelope: envelope, Deadline: testDeadline})
	compact := Compaction{ID: "c-1", HitchID: "worker-id", Resume: Message{Text: "continue"}, Deadline: testDeadline}
	state, effects := Step(state, CompactionRequested{At: testNow, Compaction: compact})
	assertEffect(t, effects, nil)

	state, effects = Step(state, TurnBoundaryReached{At: testNow, HitchID: "worker-id"})
	wantCompact := compact
	wantCompact.Status = CompactionRunning
	assertEffect(t, effects, CompactHitch{Compaction: wantCompact, Pane: "%2"})

	state, _ = Step(state, CompactionCompleted{At: testNow, CompactionID: "c-1"})
	if state.Compactions["c-1"].Status != CompactionSucceeded || state.Hitches["worker-id"].Activity != ActivityBusy {
		t.Fatalf("compaction state = %#v %#v", state.Compactions["c-1"], state.Hitches["worker-id"])
	}
	state, effects = Step(state, TurnBoundaryReached{At: testNow, HitchID: "worker-id"})
	assertEffect(t, effects, DeliverEnvelope{Envelope: envelope, Pane: "%2", Deadline: testDeadline})
}

func TestStepAdoptRenameAndInterrupt(t *testing.T) {
	state := NewState(Team{ID: "team-1", Name: "example"})
	adopted := Hitch{ID: "worker-id", Name: "worker", Collar: "codex", Directory: "/work"}
	state, effects := Step(state, AdoptRequested{At: testNow, Hitch: adopted, Pane: "%2"})
	assertEffect(t, effects, nil)
	if hitch := state.Hitches["worker-id"]; hitch.Status != HitchActive || hitch.Activity != ActivityIdle {
		t.Fatalf("adopted hitch = %#v", hitch)
	}
	state = apply(t, state, RenameRequested{At: testNow, HitchID: "worker-id", Name: "builder"})
	if state.Hitches["worker-id"].Name != "builder" {
		t.Fatal("rename did not change the registered name")
	}
	state = apply(t, state, TurnStarted{At: testNow, HitchID: "worker-id"})
	state, effects = Step(state, InterruptRequested{At: testNow, HitchID: "worker-id", Reason: "stop", Deadline: testDeadline})
	assertEffect(t, effects, InterruptHitch{HitchID: "worker-id", Pane: "%2", Reason: "stop", Deadline: testDeadline})
	state = apply(t, state, InterruptSucceeded{At: testNow, HitchID: "worker-id"})
	if state.Hitches["worker-id"].Activity != ActivityBusy {
		t.Fatalf("activity = %q", state.Hitches["worker-id"].Activity)
	}
}

func TestStepTimedDeliveryAndCurfew(t *testing.T) {
	state := twoActiveHitches(t)
	notBefore := testNow.Add(30 * time.Second)
	envelope := Envelope{ID: "e-timed", From: Sender{Kind: SenderSelfDeclared, Name: "operator"}, To: "worker", Message: Message{Text: "later"}, CreatedAt: testNow}
	state, effects := Step(state, SendRequested{At: testNow, Envelope: envelope, Deadline: testDeadline, NotBefore: notBefore})
	assertEffect(t, effects, nil)
	if got := DueTimedDeliveries(state, notBefore.Add(-time.Second)); len(got) != 0 {
		t.Fatalf("early due deliveries = %#v", got)
	}
	if got := DueTimedDeliveries(state, notBefore); !reflect.DeepEqual(got, []EnvelopeID{"e-timed"}) {
		t.Fatalf("due deliveries = %#v", got)
	}
	state, effects = Step(state, TimedDeliveryReleased{At: notBefore, EnvelopeID: "e-timed"})
	assertEffect(t, effects, DeliverEnvelope{Envelope: envelope, Pane: "%2", Deadline: testDeadline})

	state, effects = Step(state, CurfewSet{At: testNow, Deadline: testDeadline})
	assertEffect(t, effects, nil)
	if state.Team.Curfew != testDeadline {
		t.Fatalf("curfew = %s", state.Team.Curfew)
	}
	state, effects = Step(state, CurfewCleared{At: testNow})
	assertEffect(t, effects, nil)
	if !state.Team.Curfew.IsZero() {
		t.Fatalf("curfew was not cleared: %s", state.Team.Curfew)
	}
}

func TestStepClearsScheduledDeliveries(t *testing.T) {
	state := twoActiveHitches(t)
	for id, text := range map[EnvelopeID]string{"e-1": "one", "e-2": "two"} {
		envelope := Envelope{ID: id, From: Sender{Kind: SenderSelfDeclared, Name: "operator"}, To: "worker", Message: Message{Text: text}, CreatedAt: testNow}
		state = apply(t, state, SendRequested{At: testNow, Envelope: envelope, Deadline: testDeadline, NotBefore: testNow.Add(30 * time.Second)})
	}
	state = apply(t, state, TimedDeliveriesCleared{At: testNow, Recipient: "worker"})
	for _, id := range []EnvelopeID{"e-1", "e-2"} {
		if state.Deliveries[id].Status != DeliveryCancelled {
			t.Fatalf("delivery %s = %#v", id, state.Deliveries[id])
		}
	}
}

func TestStepTimeouts(t *testing.T) {
	tests := []struct {
		name  string
		state func(*testing.T) State
		event OperationTimedOut
		check func(*testing.T, State)
	}{
		{
			name: "boot fails hitch", state: bootingHitch,
			event: OperationTimedOut{At: testDeadline, Operation: TimeoutBoot, ID: "worker-id", Deadline: testDeadline, Evidence: "startup deadline passed"},
			check: func(t *testing.T, state State) {
				if state.Hitches["worker-id"].Status != HitchFailed {
					t.Fatalf("hitch = %#v", state.Hitches["worker-id"])
				}
			},
		},
		{
			name: "turn wedges hitch", state: busyHitch,
			event: OperationTimedOut{At: testDeadline, Operation: TimeoutTurn, ID: "worker-id", Deadline: testDeadline, Evidence: "turn stale"},
			check: func(t *testing.T, state State) {
				if state.Hitches["worker-id"].Activity != ActivityWedged {
					t.Fatalf("hitch = %#v", state.Hitches["worker-id"])
				}
			},
		},
		{
			name: "delivery becomes unverified", state: deliveringHitch,
			event: OperationTimedOut{At: testDeadline, Operation: TimeoutDelivery, ID: "e-1", Deadline: testDeadline, Evidence: "verification deadline passed"},
			check: func(t *testing.T, state State) {
				if state.Deliveries["e-1"].Status != DeliveryUnverified || state.Hitches["worker-id"].Activity != ActivityWedged {
					t.Fatalf("state = %#v", state)
				}
			},
		},
		{
			name: "queued delivery fails without wedging", state: func(t *testing.T) State {
				state := deliveringHitch(t)
				state, _ = Step(state, DeliveryDeferred{At: testNow, EnvelopeID: "e-1", Reason: "another surface owns input"})
				return state
			},
			event: OperationTimedOut{At: testDeadline, Operation: TimeoutDelivery, ID: "e-1", Deadline: testDeadline, Evidence: "delivery deadline elapsed before input"},
			check: func(t *testing.T, state State) {
				if state.Deliveries["e-1"].Status != DeliveryFailed || state.Hitches["worker-id"].Activity != ActivityIdle {
					t.Fatalf("state = %#v", state)
				}
			},
		},
		{
			name: "compaction becomes unverified", state: compactingHitch,
			event: OperationTimedOut{At: testDeadline, Operation: TimeoutCompaction, ID: "c-1", Deadline: testDeadline, Evidence: "compaction deadline passed"},
			check: func(t *testing.T, state State) {
				if state.Compactions["c-1"].Status != CompactionUnverified || state.Hitches["worker-id"].Activity != ActivityWedged {
					t.Fatalf("state = %#v", state)
				}
			},
		},
		{
			name: "interrupt wedges hitch", state: interruptingHitch,
			event: OperationTimedOut{At: testDeadline, Operation: TimeoutInterrupt, ID: "worker-id", Deadline: testDeadline, Evidence: "interrupt deadline passed"},
			check: func(t *testing.T, state State) {
				if state.Hitches["worker-id"].Activity != ActivityWedged {
					t.Fatalf("state = %#v", state)
				}
			},
		},
		{
			name: "drop restores wedged hitch", state: droppingHitch,
			event: OperationTimedOut{At: testDeadline, Operation: TimeoutDrop, ID: "worker-id", Deadline: testDeadline, Evidence: "kill deadline passed"},
			check: func(t *testing.T, state State) {
				hitch := state.Hitches["worker-id"]
				if hitch.Status != HitchActive || hitch.Activity != ActivityWedged {
					t.Fatalf("hitch = %#v", hitch)
				}
			},
		},
		{
			name: "wait timeout records without changing hitch", state: busyHitch,
			event: OperationTimedOut{At: testDeadline, Operation: TimeoutWait, ID: "worker-id", Deadline: testDeadline, Evidence: "idle boundary deadline passed"},
			check: func(t *testing.T, state State) {
				if state.Hitches["worker-id"].Activity != ActivityBusy {
					t.Fatalf("state = %#v", state)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, effects := Step(test.state(t), test.event)
			assertEffect(t, effects, nil)
			test.check(t, state)
		})
	}
}

func TestStepWedgeClearAndDrop(t *testing.T) {
	state := busyHitch(t)
	state, _ = Step(state, WedgeDetected{At: testNow, HitchID: "worker-id", Evidence: "screen unchanged"})
	if state.Hitches["worker-id"].Activity != ActivityWedged {
		t.Fatal("wedge was not recorded")
	}
	state, _ = Step(state, WedgeCleared{At: testNow, HitchID: "worker-id"})
	if state.Hitches["worker-id"].Activity != ActivityIdle {
		t.Fatal("wedge was not cleared")
	}
	state, effects := Step(state, DropRequested{At: testNow, HitchID: "worker-id", Deadline: testDeadline})
	assertEffect(t, effects, KillHitch{HitchID: "worker-id", Pane: "%2", Deadline: testDeadline})
	state, _ = Step(state, DropSucceeded{At: testNow, HitchID: "worker-id"})
	if state.Hitches["worker-id"].Status != HitchDropped || state.Hitches["worker-id"].Pane != "" {
		t.Fatalf("hitch = %#v", state.Hitches["worker-id"])
	}
}

func TestFailedHitchKeepsNameUntilDropped(t *testing.T) {
	state := bootingHitch(t)
	state = apply(t, state, OperationTimedOut{
		At: testDeadline, Operation: TimeoutBoot, ID: "worker-id", Deadline: testDeadline, Evidence: "startup deadline passed",
	})

	replacement := Hitch{ID: "replacement-id", Name: "worker", Collar: "codex", Directory: "/work"}
	next, effects := Step(state, HitchRequested{At: testNow, Hitch: replacement, BootDeadline: testDeadline})
	if _, exists := next.Hitches[replacement.ID]; exists {
		t.Fatal("failed hitch did not retain its name")
	}
	if len(effects) != 1 {
		t.Fatalf("replacement effects = %#v, want rejection", effects)
	}
	if record, ok := effects[0].(RecordEvent); !ok {
		t.Fatalf("replacement effect = %T, want RecordEvent", effects[0])
	} else if rejected, ok := record.Event.(TransitionRejected); !ok || rejected.Reason != "hitch name already exists" {
		t.Fatalf("replacement rejection = %#v", record.Event)
	}

	state, effects = Step(state, DropRequested{At: testNow, HitchID: "worker-id", Deadline: testDeadline})
	assertEffect(t, effects, KillHitch{HitchID: "worker-id", Pane: "%2", Deadline: testDeadline})
	if hitch := state.Hitches["worker-id"]; hitch.Status != HitchDropping || hitch.PreviousStatus != HitchFailed {
		t.Fatalf("failed drop intent = %#v", hitch)
	}
	state = apply(t, state, DropSucceeded{At: testNow, HitchID: "worker-id"})
	if state.Hitches["worker-id"].Status != HitchDropped {
		t.Fatalf("dropped hitch = %#v", state.Hitches["worker-id"])
	}

	state, effects = Step(state, HitchRequested{At: testNow, Hitch: replacement, BootDeadline: testDeadline})
	assertEffect(t, effects, SpawnHitch{Hitch: state.Hitches[replacement.ID]})
}

func TestFailedDropFailureRestoresFailedState(t *testing.T) {
	state := bootingHitch(t)
	state = apply(t, state, OperationTimedOut{
		At: testDeadline, Operation: TimeoutBoot, ID: "worker-id", Deadline: testDeadline, Evidence: "startup deadline passed",
	})
	state = apply(t, state, DropRequested{At: testNow, HitchID: "worker-id", Deadline: testDeadline})
	state = apply(t, state, DropFailed{At: testNow, HitchID: "worker-id", Reason: "kill refused"})
	if hitch := state.Hitches["worker-id"]; hitch.Status != HitchFailed || hitch.Activity != ActivityUnknown {
		t.Fatalf("restored failed hitch = %#v", hitch)
	}
}

func TestDropFailureRestoresInFlightDelivery(t *testing.T) {
	state := deliveringHitch(t)
	state, effects := Step(state, DropRequested{At: testNow, HitchID: "worker-id", Deadline: testDeadline})
	assertEffect(t, effects, KillHitch{HitchID: "worker-id", Pane: "%2", Deadline: testDeadline})
	if state.Deliveries["e-1"].Status != DeliveryDelivering {
		t.Fatalf("drop intent changed delivery = %#v", state.Deliveries["e-1"])
	}
	state = apply(t, state, DropFailed{At: testNow, HitchID: "worker-id", Reason: "kill refused"})
	if state.Hitches["worker-id"].Activity != ActivityDelivering || state.Deliveries["e-1"].Status != DeliveryDelivering {
		t.Fatalf("restored state = %#v", state)
	}
}

func TestPaneVanishedFailsQueuedWorkAndPreservesUnknownDelivery(t *testing.T) {
	state := deliveringHitch(t)
	queued := Envelope{
		ID: "e-2", From: Sender{Kind: SenderAgent, Name: "lead", HitchID: "lead-id"},
		To: "worker", Message: Message{Text: "next"}, CreatedAt: testNow,
	}
	state = apply(t, state, SendRequested{At: testNow, Envelope: queued, Deadline: testDeadline})
	state = apply(t, state, PaneVanished{At: testNow, HitchID: "worker-id", Evidence: "pane %2 is absent"})

	hitch := state.Hitches["worker-id"]
	if hitch.Status != HitchFailed || hitch.Activity != ActivityWedged || hitch.WedgeEvidence == "" {
		t.Fatalf("hitch = %#v", hitch)
	}
	if delivery := state.Deliveries["e-1"]; delivery.Status != DeliveryUnverified || delivery.Reason == "" {
		t.Fatalf("in-flight delivery = %#v", delivery)
	}
	if delivery := state.Deliveries["e-2"]; delivery.Status != DeliveryFailed || delivery.Reason == "" {
		t.Fatalf("queued delivery = %#v", delivery)
	}
}

func TestStepRejectsInvalidTransitionWithoutMutatingInput(t *testing.T) {
	state := NewState(Team{ID: "team-1", Name: "example"})
	next, effects := Step(state, DropRequested{At: testNow, HitchID: "missing", Deadline: testDeadline})
	if len(state.Hitches) != 0 || len(next.Hitches) != 0 {
		t.Fatalf("invalid transition mutated state: before=%#v after=%#v", state, next)
	}
	if len(effects) != 1 {
		t.Fatalf("effects = %#v, want one rejection", effects)
	}
	record, ok := effects[0].(RecordEvent)
	if !ok {
		t.Fatalf("effect = %T, want RecordEvent", effects[0])
	}
	rejection, ok := record.Event.(TransitionRejected)
	if !ok || rejection.Event != "drop_requested" || rejection.Reason == "" {
		t.Fatalf("rejection = %#v", record.Event)
	}
}

func TestPendingEffectsReconstructsIntent(t *testing.T) {
	states := []struct {
		name     string
		state    func(*testing.T) State
		wantType any
	}{
		{name: "boot", state: bootingHitch, wantType: AwaitBoot{}},
		{name: "delivery", state: deliveringHitch, wantType: DeliverEnvelope{}},
		{name: "compaction", state: compactingHitch, wantType: CompactHitch{}},
		{name: "interrupt", state: interruptingHitch, wantType: InterruptHitch{}},
		{name: "drop", state: droppingHitch, wantType: KillHitch{}},
	}
	for _, test := range states {
		t.Run(test.name, func(t *testing.T) {
			effects := PendingEffects(test.state(t))
			if len(effects) != 1 || reflect.TypeOf(effects[0]) != reflect.TypeOf(test.wantType) {
				t.Fatalf("effects = %#v", effects)
			}
		})
	}
}

func twoActiveHitches(t *testing.T) State {
	t.Helper()
	state := NewState(Team{ID: "team-1", Name: "example"})
	for _, hitch := range []Hitch{
		{ID: "lead-id", Name: "lead", Collar: "codex", Directory: "/work"},
		{ID: "worker-id", Name: "worker", Collar: "codex", Directory: "/work"},
	} {
		state = apply(t, state, HitchRequested{At: testNow, Hitch: hitch, BootDeadline: testDeadline})
		state = apply(t, state, HitchSpawned{At: testNow, HitchID: hitch.ID, Pane: map[HitchID]string{"lead-id": "%1", "worker-id": "%2"}[hitch.ID]})
		state = apply(t, state, HitchReady{At: testNow, HitchID: hitch.ID})
	}
	return state
}

func bootingHitch(t *testing.T) State {
	state := NewState(Team{ID: "team-1", Name: "example"})
	state = apply(t, state, HitchRequested{At: testNow, Hitch: Hitch{ID: "worker-id", Name: "worker", Collar: "codex", Directory: "/work"}, BootDeadline: testDeadline})
	return apply(t, state, HitchSpawned{At: testNow, HitchID: "worker-id", Pane: "%2"})
}

func busyHitch(t *testing.T) State {
	state := twoActiveHitches(t)
	return apply(t, state, TurnStarted{At: testNow, HitchID: "worker-id"})
}

func deliveringHitch(t *testing.T) State {
	state := twoActiveHitches(t)
	envelope := Envelope{ID: "e-1", From: Sender{Kind: SenderAgent, Name: "lead", HitchID: "lead-id"}, To: "worker", Message: Message{Text: "hello"}, CreatedAt: testNow}
	return apply(t, state, SendRequested{At: testNow, Envelope: envelope, Deadline: testDeadline})
}

func compactingHitch(t *testing.T) State {
	state := twoActiveHitches(t)
	compact := Compaction{ID: "c-1", HitchID: "worker-id", Resume: Message{Text: "continue"}, Deadline: testDeadline}
	return apply(t, state, CompactionRequested{At: testNow, Compaction: compact})
}

func droppingHitch(t *testing.T) State {
	state := twoActiveHitches(t)
	return apply(t, state, DropRequested{At: testNow, HitchID: "worker-id", Deadline: testDeadline})
}

func interruptingHitch(t *testing.T) State {
	state := busyHitch(t)
	return apply(t, state, InterruptRequested{At: testNow, HitchID: "worker-id", Reason: "stop", Deadline: testDeadline})
}

func apply(t *testing.T, state State, event Event) State {
	t.Helper()
	next, effects := Step(state, event)
	for _, effect := range effects {
		if _, rejected := effect.(RecordEvent); rejected {
			t.Fatalf("%s rejected: %#v", EventName(event), effect)
		}
	}
	return next
}

func assertEffect(t *testing.T, effects []Effect, want Effect) {
	t.Helper()
	if want == nil {
		if len(effects) != 0 {
			t.Fatalf("effects = %#v, want none", effects)
		}
		return
	}
	if len(effects) != 1 || !reflect.DeepEqual(effects[0], want) {
		t.Fatalf("effects = %#v, want %#v", effects, want)
	}
}
