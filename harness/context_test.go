package harness

import "testing"

func TestActiveContextBandUsesMostSpecificModelSelector(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if band := ActiveContextBand(collar, "claude-opus-5", ContextReading{Percent: 0.41}); band == nil || band.Name != "red" {
		t.Fatalf("opus band = %+v, want red", band)
	}
	if band := ActiveContextBand(collar, "claude-haiku-5", ContextReading{Percent: 0.41}); band != nil {
		t.Fatalf("haiku band = %+v, want none", band)
	}
	if band := ActiveContextBand(collar, "", ContextReading{Percent: 0.99}); band != nil {
		t.Fatalf("unknown-model band = %+v, want none", band)
	}
}
