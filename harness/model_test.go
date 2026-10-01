package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCodexModelCatalogLeavesUnlistedModelsUnknown(t *testing.T) {
	catalog, err := ParseModelCatalog(Invocation{Name: "codex-debug-models"}, []byte(`{
  "models":[{"slug":"gpt-one","supported_reasoning_levels":[{"effort":"low"},{"effort":"high"}]}]
}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := ValidateEffort(catalog, "missing", "high"); got != ModelUnknown {
		t.Fatalf("unlisted model effort = %q", got)
	}
	if got := ValidateEffort(catalog, "gpt-one", "xhigh"); got != ModelUnrecognized {
		t.Fatalf("unlisted effort = %q", got)
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
	if got := ValidateEffort(catalog, "opus", "high"); got != ModelRecognized {
		t.Fatalf("opus = %q", got)
	}
	if got := ValidateEffort(catalog, "claude-opus-5", "high"); got != ModelUnknown {
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

func TestDiscoverModelsReportsNativeDiagnostic(t *testing.T) {
	for _, test := range []struct{ script, status string }{
		{"exit 9", "exit status 9"},
		{"printf 'not json\\n'", "decode codex model catalog"},
	} {
		binary := filepath.Join(t.TempDir(), "fake-catalog")
		if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf 'catalog credential failure\\n' >&2\n"+test.script+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
		c, err := EmbeddedCollar("codex")
		if err != nil {
			t.Fatal(err)
		}
		c.Models.Catalog.Params["command"] = binary
		_, err = DiscoverModels(context.Background(), c)
		if err == nil || !strings.Contains(err.Error(), "catalog credential failure") || !strings.Contains(err.Error(), test.status) {
			t.Fatalf("%s: DiscoverModels = %v", test.script, err)
		}
	}
}

func TestValidateEffortLeavesModelWithoutListedEffortsUnknown(t *testing.T) {
	catalog := ModelCatalog{Models: []Model{{ID: "bare"}}}
	if got := ValidateEffort(catalog, "bare", "high"); got != ModelUnknown {
		t.Fatalf("ValidateEffort = %s", got)
	}
}
