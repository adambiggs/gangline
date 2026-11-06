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

// A harness that paints no busy marker on screen still freezes its title
// spinner when it wedges.
func TestDetectWedgeAcceptsAFrozenBusyTitle(t *testing.T) {
	since := time.Unix(100, 0)
	wedge := Invocation{Name: "stable-busy-screen", Params: map[string]string{"busy": "esc to interrupt", "after": "5m", "title_busy": "^◐ "}}
	screen := testScreen(testCells("a reply streaming with no marker", false))
	screen.Title = "◐ Fix the flaky test"
	got, err := DetectWedge(wedge, WedgeObservation{
		Previous: activityFingerprint(wedge, screen), Current: screen, BusySince: since,
		ObservedAt: since.Add(5 * time.Minute), TurnActive: true,
	})
	if err != nil || !got.Detected {
		t.Fatalf("wedge = %+v, error = %v", got, err)
	}
	screen.Title = "✳ Fix the flaky test"
	got, err = DetectWedge(wedge, WedgeObservation{
		Previous: activityFingerprint(wedge, screen), Current: screen, BusySince: since,
		ObservedAt: since.Add(5 * time.Minute), TurnActive: true,
	})
	if err != nil || got.Detected {
		t.Fatalf("idle title wedged: %+v, error = %v", got, err)
	}
}
