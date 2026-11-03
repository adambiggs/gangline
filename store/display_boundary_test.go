package store

import (
	"errors"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestDisplayBoundaryUsesNativeIdentityAndOrder(t *testing.T) {
	at := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	bound := core.Agent{Native: core.NativeState{SessionID: "s", TurnID: "current", SubmittedAt: at}}
	witness := Witness{SessionID: "s", TurnID: "current", At: at}
	for _, tc := range []struct {
		name    string
		n       StatusHook
		a       core.Agent
		w       Witness
		records []StatusHook
		want    bool
	}{
		{name: "current zero", n: StatusHook{Kind: "turn-finished", SessionID: "s", TurnID: "current"}, a: bound, w: witness, want: true},
		{name: "late old zero", n: StatusHook{Kind: "turn-finished", SessionID: "s", TurnID: "old"}, a: bound, w: witness},
		{name: "late old failure", n: StatusHook{Kind: "turn-failed", SessionID: "s", TurnID: "old"}, a: bound, w: witness},
		{name: "foreign session", n: StatusHook{Kind: "turn-finished", SessionID: "foreign", TurnID: "current"}, a: bound, w: witness},
		{name: "missing identity cannot erase bound work", n: StatusHook{Kind: "turn-finished", SessionID: "s"}, a: bound, w: witness},
		{name: "legacy unbound display", n: StatusHook{Kind: "turn-finished", SessionID: "s"}, want: true},
		{name: "new witness before reconciliation", n: StatusHook{Kind: "turn-finished", SessionID: "s", TurnID: "old"}, a: core.Agent{Native: core.NativeState{SessionID: "s", TurnID: "old", SubmittedAt: at.Add(-time.Second)}}, w: witness},
		{name: "queued turn after witness", n: StatusHook{Kind: "turn-finished", SessionID: "s", TurnID: "queued"}, a: bound, w: witness, want: true},
		{name: "unorderable queued turn retains prior behavior", n: StatusHook{Kind: "turn-finished", SessionID: "s", TurnID: "unseen"}, a: bound, w: witness, want: true},
		{name: "queued terminal supersedes witnessed turn", n: StatusHook{Kind: "turn-finished", SessionID: "s", TurnID: "current"}, a: bound, w: witness, records: []StatusHook{{Kind: "turn-finished", SessionID: "s", TurnID: "queued", Sequence: 2}}},
		{name: "stale retained hook cannot poison current turn", n: StatusHook{Kind: "turn-finished", SessionID: "s", TurnID: "current"}, a: bound, w: witness, records: []StatusHook{{Kind: "turn-failed", SessionID: "s", TurnID: "unseen", Sequence: 2}}, want: true},
		{name: "queued failure blocks older finish", n: StatusHook{Kind: "turn-finished", SessionID: "s", TurnID: "current"}, a: core.Agent{Native: core.NativeState{SessionID: "s", TurnID: "current", FailedTurn: "queued", SubmittedAt: at}}, w: witness},
		{name: "new submit supersedes old failure", n: StatusHook{Kind: "turn-finished", SessionID: "s", TurnID: "current"}, a: core.Agent{Native: core.NativeState{SessionID: "s", TurnID: "old", FailedTurn: "old", SubmittedAt: at.Add(-time.Second)}}, w: witness, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			order := map[string]int{"old": 1, "current": 2, "queued": 3}
			later := func(earlier, later string) (bool, error) {
				return order[earlier] > 0 && order[later] > order[earlier], nil
			}
			got, err := displayBoundaryAllowed(tc.n, tc.a, tc.w, tc.records, later)
			if err != nil || got != tc.want {
				t.Fatalf("current=%v err=%v, want %v", got, err, tc.want)
			}
		})
	}
}

func TestDisplayBoundaryPreservesOrderingReadFailure(t *testing.T) {
	want := errors.New("unreadable native order")
	got, err := displayBoundaryAllowed(StatusHook{SessionID: "s", TurnID: "queued"}, core.Agent{Native: core.NativeState{SessionID: "s", TurnID: "current"}}, Witness{}, nil, func(string, string) (bool, error) { return false, want })
	if !got || !errors.Is(err, want) {
		t.Fatalf("current=%v err=%v", got, err)
	}
}
