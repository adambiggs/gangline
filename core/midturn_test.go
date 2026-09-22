package core

import (
	"encoding/json"
	"testing"
)

func midturnSend(t *testing.T, id EnvelopeID, capable bool) SendRequested {
	t.Helper()
	event := SendRequested{At: testNow, Deadline: testDeadline, Envelope: Envelope{ID: id, From: Sender{Kind: SenderSelfDeclared, Name: "operator"}, To: "worker", Message: Message{Text: "steer"}, CreatedAt: testNow}}
	// JSON lets this behavior regression run against the pre-capability API too.
	data, err := json.Marshal(map[string]bool{"mid_turn": capable})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	return event
}

func TestBusyNativeSendIsAcceptedBeforeTurnBoundary(t *testing.T) {
	for _, capable := range []bool{false, true} {
		state := twoActiveHitches(t)
		state, _ = Step(state, TurnStarted{At: testNow, HitchID: "worker-id"})
		state, effects := Step(state, midturnSend(t, "steer", capable))
		if !capable {
			if len(effects) != 0 || state.Deliveries["steer"].Status != DeliveryQueued {
				t.Fatal("unsupported collar did not spool")
			}
			continue
		}
		if len(effects) != 1 || state.Deliveries["steer"].Status != DeliveryDelivering || state.Hitches["worker-id"].Activity != ActivityBusy {
			t.Fatalf("mid-turn send waited for Stop or replaced turn state: effects=%v delivery=%+v hitch=%+v", effects, state.Deliveries["steer"], state.Hitches["worker-id"])
		}
		state, _ = Step(state, DeliverySucceeded{At: testNow, EnvelopeID: "steer"})
		if state.Deliveries["steer"].Status != DeliveryDelivered || state.Hitches["worker-id"].Activity != ActivityBusy {
			t.Fatal("native acceptance did not preserve busy turn")
		}
	}
}

func TestMidTurnDeliveryPreservesConcurrentTurnFacts(t *testing.T) {
	for _, terminal := range []string{"deferred", "failed", "succeeded", "unverified"} {
		for _, boundary := range []bool{false, true} {
			state := twoActiveHitches(t)
			state, _ = Step(state, TurnStarted{At: testNow, HitchID: "worker-id"})
			state, _ = Step(state, midturnSend(t, "first", true))
			// A second send cannot type over the first owner's composer or witness.
			state, effects := Step(state, midturnSend(t, "second", true))
			if len(effects) != 0 {
				t.Fatal("overlapping input effects")
			}
			if boundary {
				state, effects = Step(state, TurnBoundaryReached{At: testNow, HitchID: "worker-id"})
				if len(effects) != 0 || state.Hitches["worker-id"].Activity != ActivityIdle {
					t.Fatal("boundary lost or dispatched over in-flight input")
				}
			}
			var event Event
			switch terminal {
			case "deferred":
				event = DeliveryDeferred{At: testNow, EnvelopeID: "first", Reason: "occupied"}
			case "failed":
				event = DeliveryFailedEvent{At: testNow, EnvelopeID: "first", Reason: "refused"}
			case "succeeded":
				event = DeliverySucceeded{At: testNow, EnvelopeID: "first"}
			case "unverified":
				event = DeliveryUnverifiedEvent{At: testNow, EnvelopeID: "first", Evidence: "witness mismatch"}
			}
			state, effects = Step(state, event)
			want := ActivityBusy
			if boundary {
				want = ActivityIdle
			}
			if terminal == "unverified" {
				want = ActivityWedged
			}
			if (terminal == "failed" || terminal == "succeeded") && boundary {
				want = ActivityDelivering
			}
			if state.Hitches["worker-id"].Activity != want {
				t.Fatalf("terminal=%s boundary=%v: activity=%s want=%s", terminal, boundary, state.Hitches["worker-id"].Activity, want)
			}
			if terminal == "unverified" && len(effects) != 0 {
				t.Fatal("unverified input automatically retried")
			}
			if terminal == "succeeded" && (len(effects) != 1 || state.Deliveries["second"].Status != DeliveryDelivering) {
				t.Fatal("second native send did not drain")
			}
		}
	}
}

func TestMidTurnCapabilityRoundTrips(t *testing.T) {
	event := midturnSend(t, "steer", true)
	bytes, err := EncodeEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEvent(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.(SendRequested); !got.MidTurn {
		t.Fatal("capability lost on replay")
	}
}
