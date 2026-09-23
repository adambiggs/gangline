package harness

import "testing"

func TestActiveContextBandUsesMostSpecificModelSelector(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if band := ActiveContextBand(collar, "claude-opus-5", ContextReading{Percent: 0.41}); band == nil || band.Name != "late" {
		t.Fatalf("opus band = %+v, want late", band)
	}
	if band := ActiveContextBand(collar, "claude-haiku-5", ContextReading{Percent: 0.41}); band != nil {
		t.Fatalf("haiku band = %+v, want none", band)
	}
	if band := ActiveContextBand(collar, "", ContextReading{Percent: 0.99}); band != nil {
		t.Fatalf("unknown-model band = %+v, want none", band)
	}
}

func TestCrossedContextBandsIncludesOnsetAndSkippedThresholds(t *testing.T) {
	collar := Collar{ContextBands: map[string][]ContextBand{"*": {{Name: "onset", At: 0}, {Name: "checkpoint", At: 0.5}}}}
	crossed := CrossedContextBands(collar, "model", -1, 0.5)
	if len(crossed) != 2 || crossed[0].Name != "onset" || crossed[1].Name != "checkpoint" {
		t.Fatalf("crossed=%+v", crossed)
	}
	if got := CrossedContextBands(collar, "model", 0.5, 0.6); len(got) != 0 {
		t.Fatalf("same bands repeated: %+v", got)
	}
	if got := CrossedContextBands(collar, "", -1, 1); len(got) != 0 {
		t.Fatalf("unknown model crossed: %+v", got)
	}
}
