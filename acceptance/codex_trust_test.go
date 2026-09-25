package acceptance

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestSeedCodexTrustCopiesOnlyAcceptedDirectory(t *testing.T) {
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
		if err := seedCodexTrust(regularHome, privateHome, cwd); err != nil {
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

func TestSeedCodexTrustRequiresPriorDecision(t *testing.T) {
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
	err := seedCodexTrust(regularHome, privateHome, filepath.Join(root, "checkout"))
	if err == nil || !strings.Contains(err.Error(), "trust it interactively") {
		t.Fatalf("missing trust decision error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(privateHome, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("private config changed without prior trust: %v", err)
	}
}
