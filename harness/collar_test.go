package harness

import (
	"strings"
	"testing"
)

func TestUsageBandOverlayReplacesOnlySelectedWindow(t *testing.T) {
	collar, err := LoadCustomCollar("codex", "codex.cue", []byte(`collar: {
		usage_bands: {
			five_hour: [{name: "early", ahead: 0.8, message: "{{collar}} {{reset_at}}"}]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := collar.UsageBands["five_hour"]; len(got) != 1 || got[0].Name != "early" {
		t.Fatalf("five-hour overlay = %+v", got)
	}
	if got := collar.UsageBands["weekly"]; len(got) == 0 || got[0].Name != "yellow" {
		t.Fatalf("weekly defaults were lost: %+v", got)
	}
}

// A band measures pace, not absolute use; an overlay that still sets the
// absolute "at" threshold must fail to load rather than change meaning.
func TestUsageBandRejectsAbsoluteThreshold(t *testing.T) {
	_, err := LoadCustomCollar("codex", "codex.cue", []byte(`collar: {
		usage_bands: {weekly: [{name: "yellow", at: 0.75}]}
	}`))
	if err == nil || !strings.Contains(err.Error(), "at: field not allowed") {
		t.Fatalf("absolute usage-band threshold = %v", err)
	}
}

func TestUsageBandMessageTokensCarryPace(t *testing.T) {
	if _, err := LoadCustomCollar("codex", "codex.cue", []byte(`collar: {
		usage_bands: {weekly: [{name: "yellow", ahead: 0.1, message: "{{used_percent}} {{elapsed_percent}} {{pace_points}} {{exhaustion_at}} {{threshold_percent}}"}]}
	}`)); err != nil {
		t.Fatal(err)
	}
}

func TestUsageBandRejectsInvalidMessageToken(t *testing.T) {
	_, err := LoadCustomCollar("codex", "codex.cue", []byte(`collar: {
		usage_bands: {
			weekly: [{name: "warn", ahead: 0.8, message: "{{secret}}"}]
		}
	}`))
	if err == nil {
		t.Fatal("unknown usage-band message token passed validation")
	}
}

func TestUsageBandRejectsInvalidNoteToken(t *testing.T) {
	_, err := LoadCustomCollar("codex", "codex.cue", []byte(`collar: {
		usage_bands: {weekly: [{name: "yellow", ahead: 0.75, note: "{{unknown}}"}]}
	}`))
	if err == nil {
		t.Fatal("unknown usage-band note token passed validation")
	}
}

func TestHitchGuardOverlayLoads(t *testing.T) {
	collar, err := LoadCustomCollar("codex", "codex.cue", []byte(`collar: {
		hitch_guard: {models: ["costly"], weekly: 0.9, refuse: true}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	g := collar.HitchGuard
	if g == nil || len(g.Models) != 1 || g.Models[0] != "costly" || g.Weekly == nil || *g.Weekly != 0.9 || g.FiveHour != nil || !g.Refuse {
		t.Fatalf("hitch guard = %+v", g)
	}
}

func TestHitchGuardRejectsMissingThreshold(t *testing.T) {
	_, err := LoadCustomCollar("codex", "codex.cue", []byte(`collar: {
		hitch_guard: {models: ["costly"]}
	}`))
	if err == nil {
		t.Fatal("hitch guard without a threshold passed validation")
	}
}

func TestEmbeddedCollarsValidate(t *testing.T) {
	want := []string{"claude", "codex"}
	names, err := EmbeddedCollarNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != len(want) {
		t.Fatalf("embedded collar names = %q, want %q", names, want)
	}
	for index := range want {
		if names[index] != want[index] {
			t.Fatalf("embedded collar names = %q, want %q", names, want)
		}
		collar, err := EmbeddedCollar(names[index])
		if err != nil {
			t.Fatalf("load %s: %v", names[index], err)
		}
		if collar.Name != names[index] {
			t.Fatalf("collar name = %q, want %q", collar.Name, names[index])
		}
		if collar.HitchGuard != nil {
			t.Fatalf("%s ships a hitch guard; guards are operator opt-in", collar.Name)
		}
	}
}

func TestLoadCollarRejectsUnknownField(t *testing.T) {
	data := []byte(`package collars

collar: {
	name: "example"
	launch: {command: "example"}
	primitives: {
		startup: [{name: "trust-prompt"}]
		composer: {name: "composer-read"}
		submit: {name: "submit"}
		turn_boundary: {name: "hook-boundary"}
		blocked: {name: "screen-blocked", params: {prompt: "BLOCKED", choice: "ALLOW"}}
		context: {name: "screen-context"}
		provider_limits: {name: "screen-limits"}
		wedge: {name: "stable-busy-screen"}
	}
	actions: {interrupt: {keys: ["Escape"]}, compact: {text: "/compact", submit: true}, compact_recover: []}
	models: {catalog: {name: "models"}, option: {args: ["--model", "{{value}}"]}}
	context_bands: {}
	unknown: true
}`)
	if _, err := LoadCollar("example.cue", data); err == nil {
		t.Fatal("unknown collar field passed validation")
	}
}

func TestCollarRejectsUnknownQueuedTurnSource(t *testing.T) {
	_, err := LoadCustomCollar("claude", "claude.cue", []byte(`collar: primitives: turn_boundary: params: queued_turns: "codex"`))
	if err == nil {
		t.Fatal("unknown queued_turns source passed validation")
	}
}

func TestCollarRejectsUnknownSessionMoveSource(t *testing.T) {
	_, err := LoadCustomCollar("claude", "claude.cue", []byte(`collar: primitives: turn_boundary: params: session_moves: "codex"`))
	if err == nil {
		t.Fatal("unknown session_moves source passed validation")
	}
}

func TestTitleActionOverlay(t *testing.T) {
	c, err := LoadCustomCollar("codex", "custom.cue", []byte(`collar: actions: title: {text: "/name {{title}}", refusal: "^failed to set thread name:"}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Actions.Title.Text != "/name {{title}}" || !c.Actions.Title.Submit {
		t.Fatalf("title = %+v", c.Actions.Title)
	}
	for _, overlay := range []string{
		`collar: actions: title: {refusal: "("}`,
		`collar: actions: title: {submit: "yes"}`,
		`collar: actions: title: {unknown: true}`,
	} {
		if _, err := LoadCustomCollar("codex", "custom.cue", []byte(overlay)); err == nil {
			t.Fatalf("invalid title accepted: %s", overlay)
		}
	}
}
