package harness

import "testing"

func TestUsageBandOverlayReplacesOnlySelectedWindow(t *testing.T) {
	collar, err := LoadCustomCollar("codex", "codex.cue", []byte(`collar: {
		usage_bands: {
			five_hour: [{name: "early", at: 0.8, message: "{{collar}} {{reset_at}}"}]
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

func TestUsageBandRejectsInvalidMessageToken(t *testing.T) {
	_, err := LoadCustomCollar("codex", "codex.cue", []byte(`collar: {
		usage_bands: {
			weekly: [{name: "warn", at: 0.8, message: "{{secret}}"}]
		}
	}`))
	if err == nil {
		t.Fatal("unknown usage-band message token passed validation")
	}
}

func TestUsageBandRejectsInvalidNoteToken(t *testing.T) {
	_, err := LoadCustomCollar("codex", "codex.cue", []byte(`collar: {
		usage_bands: {weekly: [{name: "yellow", at: 0.75, note: "{{unknown}}"}]}
	}`))
	if err == nil {
		t.Fatal("unknown usage-band note token passed validation")
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
