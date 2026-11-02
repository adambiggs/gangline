//go:build acceptance

package acceptance

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestSeedCodexConfigCopiesOnlyAcceptedDirectory(t *testing.T) {
	root := t.TempDir()
	regularHome := filepath.Join(root, "regular")
	privateHome := filepath.Join(root, "private")
	for _, dir := range []string{regularHome, privateHome} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	cwd := filepath.Join(root, "checkout")
	regular := "[projects." + strconv.Quote(cwd) + "]\ntrust_level = 'trusted'\n[projects.'/other']\ntrust_level = 'trusted'\n"
	if err := os.WriteFile(filepath.Join(regularHome, "config.toml"), []byte(regular), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(privateHome, "config.toml"), []byte("[tui]\nscreen_reader_detection_done = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := seedCodexConfig(regularHome, privateHome, cwd); err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile(filepath.Join(privateHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := toml.Unmarshal(content, &config); err != nil {
		t.Fatal(err)
	}
	if got := codexTrustLevel(config, cwd); got != "trusted" {
		t.Fatalf("test directory trust = %q, want trusted", got)
	}
	if got := codexTrustLevel(config, "/other"); got != "" {
		t.Fatalf("copied unrelated trust = %q", got)
	}
	tui, ok := config["tui"].(map[string]any)
	if !ok || tui["screen_reader_detection_done"] != true {
		t.Fatalf("existing private config was lost: %s", content)
	}
}

func TestSeedCodexConfigRequiresPriorDecision(t *testing.T) {
	root := t.TempDir()
	regularHome := filepath.Join(root, "regular")
	privateHome := filepath.Join(root, "private")
	for _, dir := range []string{regularHome, privateHome} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(regularHome, "config.toml"), []byte("[projects.'/other']\ntrust_level = 'trusted'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err := seedCodexConfig(regularHome, privateHome, filepath.Join(root, "checkout"))
	if err == nil || !strings.Contains(err.Error(), "trust it interactively") {
		t.Fatalf("missing trust decision error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(privateHome, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("private config changed without prior trust: %v", err)
	}
}

func TestSeedCodexConfigPreservesSelectedProviderConnection(t *testing.T) {
	root := t.TempDir()
	regularHome, privateHome := filepath.Join(root, "regular"), filepath.Join(root, "private")
	for _, dir := range []string{regularHome, privateHome} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	cwd := filepath.Join(root, "checkout")
	connection := map[string]any{
		"name": "Synthetic gateway", "base_url": "https://gateway.invalid/v1",
		"wire_api": "responses", "requires_openai_auth": true,
		"env_key": "SYNTHETIC_API_KEY", "env_http_headers": map[string]any{"X-Route": "SYNTHETIC_ROUTE"},
		"http_headers": map[string]any{"X-Test": "synthetic"}, "query_params": map[string]any{"version": "test"},
	}
	regular := map[string]any{
		"model_provider": "gateway", "model": "unrelated-model",
		"model_providers":        map[string]any{"gateway": connection, "unrelated": map[string]any{"base_url": "https://other.invalid"}},
		"projects":               map[string]any{cwd: map[string]any{"trust_level": "trusted"}},
		"hooks":                  map[string]any{"sentinel": "do-not-copy"},
		"mcp_servers":            map[string]any{"sentinel": map[string]any{"command": "do-not-run"}},
		"developer_instructions": "do-not-copy", "approval_policy": "never", "sandbox_mode": "danger-full-access",
	}
	data, err := toml.Marshal(regular)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(regularHome, "config.toml"), data, 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := seedCodexConfig(regularHome, privateHome, cwd); err != nil {
			t.Fatal(err)
		}
	}
	result, err := os.ReadFile(filepath.Join(privateHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := toml.Unmarshal(result, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"model_provider":  "gateway",
		"model_providers": map[string]any{"gateway": connection},
		"projects":        map[string]any{cwd: map[string]any{"trust_level": "trusted"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("isolated configuration differs: got %v, want %v", got, want)
	}
	info, err := os.Stat(filepath.Join(privateHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
	unchanged, err := os.ReadFile(filepath.Join(regularHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != string(data) {
		t.Fatal("regular config changed")
	}
}
