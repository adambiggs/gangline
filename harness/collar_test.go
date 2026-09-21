package harness

import "testing"

func TestEmbeddedCollarsValidate(t *testing.T) {
	want := []string{"claude-code", "codex"}
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
		context: {name: "screen-context"}
		wedge: {name: "stable-busy-screen"}
	}
	models: {catalog: {name: "models"}, option: {flag: "--model"}}
	context_bands: []
	unknown: true
}`)
	if _, err := LoadCollar("example.cue", data); err == nil {
		t.Fatal("unknown collar field passed validation")
	}
}
