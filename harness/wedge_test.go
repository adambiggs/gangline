package harness

import (
	"testing"
	"time"
)

func TestDetectWedgeRequiresStableBusyActiveTurn(t *testing.T) {
	since := time.Unix(100, 0)
	screen := testScreen(testCells("esc to interrupt", false))
	wedge, err := DetectWedge(Invocation{
		Name:   "stable-busy-screen",
		Params: map[string]string{"busy": "esc to interrupt", "after": "5m"},
	}, WedgeObservation{
		Previous: screen, Current: screen, BusySince: since,
		ObservedAt: since.Add(5 * time.Minute), TurnActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !wedge.Detected || wedge.Evidence != "esc to interrupt" {
		t.Fatalf("wedge = %+v", wedge)
	}
}

func TestDetectWedgeRejectsChangedScreen(t *testing.T) {
	since := time.Unix(100, 0)
	wedge, err := DetectWedge(Invocation{
		Name:   "stable-busy-screen",
		Params: map[string]string{"busy": "Retrying", "after": "1m"},
	}, WedgeObservation{
		Previous:  testScreen(testCells("Retrying 1", false)),
		Current:   testScreen(testCells("Retrying 2", false)),
		BusySince: since, ObservedAt: since.Add(time.Minute), TurnActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if wedge.Detected {
		t.Fatal("changing screen was declared wedged")
	}
}
