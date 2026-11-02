package harness

import (
	"reflect"
	"strings"
	"testing"
)

// claudeHelpWithoutHaiku mirrors the installed Claude Code --help output
// whose --model example lists fable, opus and sonnet only, while the native
// CLI also accepts the haiku alias.
const claudeHelpWithoutHaiku = `
Options:
  --effort <level>                      Effort level for the current session
                                        (low, medium, high, xhigh, max)
  --environment <environment_id>        Create a new cloud session that runs on
                                        the given self-hosted environment
                                        (ccpool_...).
  --model <model>                       Model for the current session. Provide
                                        an alias for the latest model (e.g.
                                        'fable', 'opus', or 'sonnet') or a
                                        model's full name.
  -n, --name <name>                     Set a display name for this session
`

func TestClaudeCatalogListsDeclaredAliasTheHelpOmits(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := ParseModelCatalog(collar.Models.Catalog, []byte(claudeHelpWithoutHaiku))
	if err != nil {
		t.Fatal(err)
	}
	if got := catalogModelIDs(catalog); !reflect.DeepEqual(got, []string{"fable", "haiku", "opus", "sonnet"}) {
		t.Fatalf("catalog ids = %v", got)
	}
	if efforts := ModelEfforts(catalog, "haiku"); len(efforts) != 0 {
		t.Fatalf("haiku efforts = %v, want none", efforts)
	}
	if got := ValidateEffort(catalog, "haiku", "high"); got != ModelUnknown {
		t.Fatalf("haiku high = %q", got)
	}
	if got := ValidateEffort(catalog, "haiku", "ultra"); got != ModelUnknown {
		t.Fatalf("haiku ultra = %q", got)
	}
	for _, id := range []string{"fable", "opus", "sonnet"} {
		if efforts := ModelEfforts(catalog, id); !reflect.DeepEqual(efforts, []string{"low", "medium", "high", "xhigh", "max"}) {
			t.Fatalf("%s efforts = %v", id, efforts)
		}
	}
}

func TestClaudeCatalogWithoutDeclaredAliasesKeepsHelpList(t *testing.T) {
	catalog, err := ParseModelCatalog(Invocation{Name: "claude-help-models", Params: map[string]string{"command": "claude", "args": "--help"}}, []byte(claudeHelpWithoutHaiku))
	if err != nil {
		t.Fatal(err)
	}
	if got := catalogModelIDs(catalog); !reflect.DeepEqual(got, []string{"fable", "opus", "sonnet"}) {
		t.Fatalf("catalog ids = %v", got)
	}
	if got := ValidateEffort(catalog, "haiku", "high"); got != ModelUnknown {
		t.Fatalf("unlisted haiku = %q", got)
	}
	if got := ValidateEffort(catalog, "claude-haiku-5-5", "high"); got != ModelUnknown {
		t.Fatalf("full model = %q", got)
	}
}

func TestClaudeCatalogDeduplicatesDeclaredAliasListedByHelp(t *testing.T) {
	help := strings.Replace(claudeHelpWithoutHaiku, "'fable', 'opus', or 'sonnet'", "'fable', 'haiku', 'opus', or 'sonnet'", 1)
	catalog, err := ParseModelCatalog(Invocation{Name: "claude-help-models", Params: map[string]string{"command": "claude", "args": "--help", "extra_aliases": "haiku"}}, []byte(help))
	if err != nil {
		t.Fatal(err)
	}
	if got := catalogModelIDs(catalog); !reflect.DeepEqual(got, []string{"fable", "haiku", "opus", "sonnet"}) {
		t.Fatalf("catalog ids = %v", got)
	}
	if !reflect.DeepEqual(ModelEfforts(catalog, "haiku"), ModelEfforts(catalog, "sonnet")) {
		t.Fatalf("help-listed haiku efforts %v differ from sonnet efforts %v", ModelEfforts(catalog, "haiku"), ModelEfforts(catalog, "sonnet"))
	}
}

func TestClaudeCatalogRejectsExtraAliasThatIsNotAnAlias(t *testing.T) {
	for _, extra := range []string{"Haiku", "claude-haiku-5-5", "haiku_beta"} {
		invocation := Invocation{Name: "claude-help-models", Params: map[string]string{"command": "claude", "args": "--help", "extra_aliases": extra}}
		if _, err := ParseModelCatalog(invocation, []byte(claudeHelpWithoutHaiku)); err == nil {
			t.Fatalf("extra alias %q accepted", extra)
		}
	}
}

func catalogModelIDs(catalog ModelCatalog) []string {
	ids := make([]string, 0, len(catalog.Models))
	for _, model := range catalog.Models {
		ids = append(ids, model.ID)
	}
	return ids
}
