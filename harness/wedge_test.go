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
		Previous: ScreenFingerprint(screen), Current: screen, BusySince: since,
		ObservedAt: since.Add(5 * time.Minute), TurnActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !wedge.Detected || wedge.Evidence != "busy screen unchanged for at least 5m" {
		t.Fatalf("wedge = %+v", wedge)
	}
}

func TestDetectWedgeRejectsChangedScreen(t *testing.T) {
	since := time.Unix(100, 0)
	wedge, err := DetectWedge(Invocation{
		Name:   "stable-busy-screen",
		Params: map[string]string{"busy": "Retrying", "after": "1m"},
	}, WedgeObservation{
		Previous:  ScreenFingerprint(testScreen(testCells("Retrying 1", false))),
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

// A busy title on a screen without a busy marker does not wedge: the harness
// may hold one glyph through quiet work.
func TestDetectWedgeIgnoresAFrozenBusyTitle(t *testing.T) {
	since := time.Unix(100, 0)
	wedge := Invocation{Name: "stable-busy-screen", Params: map[string]string{"busy": "esc to interrupt", "after": "5m", "title_busy": "^◐ "}}
	screen := testScreen(testCells("a long tool call with no marker", false))
	screen.Title = "◐ Fix the flaky test"
	got, err := DetectWedge(wedge, WedgeObservation{
		Previous: activityFingerprint(wedge, screen), Current: screen, BusySince: since,
		ObservedAt: since.Add(10 * time.Minute), TurnActive: true,
	})
	if err != nil || got.Detected {
		t.Fatalf("frozen busy title wedged: %+v, error = %v", got, err)
	}
}
