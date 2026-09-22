package harness

import (
	"context"
	"github.com/adambiggs/gangline/substrate"
	"testing"
)

func TestIdleRequiresAnEmptyComposerWithoutNativeWork(t *testing.T) {
	collar := Collar{}
	collar.Primitives.Composer = Invocation{Name: "codex-composer"}
	collar.Primitives.Wedge = Invocation{Name: "stable-busy-screen", Params: map[string]string{"busy": "Working"}}
	for _, test := range []struct {
		name, status, composer string
		idle                   bool
	}{
		{"ready", "Ready", "› ", true},
		{"busy empty composer", "Working", "› ", false},
		{"occupied composer", "Ready", "› draft", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			screen := testScreen(testCells(test.status, false), testCells(test.composer, false))
			idle, err := Idle(collar, screen)
			if err != nil || idle != test.idle {
				t.Fatalf("idle = %v, error = %v", idle, err)
			}
			if test.idle {
				if err := AwaitIdle(context.Background(), func(context.Context, substrate.PaneID) (substrate.Screen, error) { return screen, nil }, "%1", collar); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	if _, err := Idle(collar, testScreen(testCells("unrecognized surface", false))); err == nil {
		t.Fatal("unrecognized surface accepted")
	}
}
