package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestStatuslineObservationIdentity(t *testing.T) {
	at := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	later := at.Add(time.Second)
	a := core.Agent{ID: "a", Registration: core.PaneRegistration{Generation: "one"}}
	base := []core.Reading{{Kind: "provider-limits", At: &at, Status: "unknown", Reason: "absent"}, {Kind: "context", At: &at, Status: "unknown", Reason: "absent"}}
	key := func(a core.Agent, session string, readings []core.Reading) string {
		t.Helper()
		k, err := statuslineObservationKey(a, session, readings)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	want := key(a, "s", base)
	copy := append([]core.Reading(nil), base...)
	copy[0].At = &later
	if key(a, "s", copy) != want {
		t.Fatal("adapter sample time defeated suppression")
	}
	if !reflect.DeepEqual(base[0].At, &at) {
		t.Fatal("normalization mutated original reading")
	}
	copy[1].At = &later
	if key(a, "s", copy) == want {
		t.Fatal("source time change suppressed")
	}
	copy = append([]core.Reading(nil), base...)
	copy[0].Reason = "unavailable"
	if key(a, "s", copy) == want {
		t.Fatal("unknown reason change suppressed")
	}
	if key(a, "new-session", base) == want {
		t.Fatal("new session reused marker")
	}
	a.Registration.Generation = "two"
	if key(a, "s", base) == want {
		t.Fatal("new registration reused marker")
	}
	a.Registration.Generation, a.ID = "one", "b"
	if key(a, "s", base) == want {
		t.Fatal("new hitch reused marker")
	}
}
