package harness

import "testing"

func TestParseCodexModelCatalogIsComplete(t *testing.T) {
	catalog, err := ParseModelCatalog(Invocation{Name: "codex-debug-models"}, []byte(`{
  "models":[{"slug":"gpt-one","supported_reasoning_levels":[{"effort":"low"},{"effort":"high"}]}]
}`))
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.Complete || ValidateModel(catalog, "missing") != ModelUnrecognized {
		t.Fatalf("catalog = %+v", catalog)
	}
}

func TestParseClaudeModelCatalogLeavesFullNamesUnknown(t *testing.T) {
	help := `
  --effort <level>  Effort level (low, medium, high)
  --environment <id> Environment
  --model <model> Model alias ('fable', 'opus', or 'sonnet') or full name
  --name <name> Name
`
	catalog, err := ParseModelCatalog(Invocation{Name: "claude-help-models"}, []byte(help))
	if err != nil {
		t.Fatal(err)
	}
	if got := ValidateModel(catalog, "opus"); got != ModelRecognized {
		t.Fatalf("opus = %q", got)
	}
	if got := ValidateModel(catalog, "claude-opus-5"); got != ModelUnknown {
		t.Fatalf("full model = %q", got)
	}
}

func TestReadClaudeSelectedModelFromHeader(t *testing.T) {
	model, err := ReadSelectedModel(
		Invocation{Name: "claude-screen-model"},
		testScreen(testCells("Claude Code v2", false), testCells("Sonnet 4.5 · /work", false)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if model != "sonnet" {
		t.Fatalf("model = %q", model)
	}
}
