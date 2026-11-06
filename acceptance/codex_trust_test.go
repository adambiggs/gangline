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
	if _, kept := config["tui"]; kept {
		t.Fatalf("private settings from an earlier run were kept: %s", content)
	}
}

func TestSeedCodexConfigDropsProviderTheRegularConfigNoLongerSelects(t *testing.T) {
	root := t.TempDir()
	regularHome, privateHome := filepath.Join(root, "regular"), filepath.Join(root, "private")
	for _, dir := range []string{regularHome, privateHome} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	cwd := filepath.Join(root, "checkout")
	project := "[projects." + strconv.Quote(cwd) + "]\ntrust_level = 'trusted'\n"
	routed := "model_provider = 'proxy'\n[model_providers.proxy]\nbase_url = 'https://proxy.invalid/v1'\n" + project
	if err := os.WriteFile(filepath.Join(regularHome, "config.toml"), []byte(routed), 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedCodexConfig(regularHome, privateHome, cwd); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(regularHome, "config.toml"), []byte(project), 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedCodexConfig(regularHome, privateHome, cwd); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(privateHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "proxy") {
		t.Fatalf("private config kept a provider route the regular config dropped:\n%s", content)
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

func TestSeedCodexConfigKeepsPrivateHookTrust(t *testing.T) {
	root := t.TempDir()
	regularHome, privateHome := filepath.Join(root, "regular"), filepath.Join(root, "private")
	for _, dir := range []string{regularHome, privateHome} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	cwd := filepath.Join(root, "checkout")
	key := "/<session-flags>/config.toml:stop:0:0"
	regular := "[projects." + strconv.Quote(cwd) + "]\ntrust_level = 'trusted'\n[hooks.state." + strconv.Quote("/<session-flags>/config.toml:session_start:0:0") + "]\ntrusted_hash = 'sha256:live-team'\n"
	if err := os.WriteFile(filepath.Join(regularHome, "config.toml"), []byte(regular), 0600); err != nil {
		t.Fatal(err)
	}
	private := "[hooks.state." + strconv.Quote(key) + "]\ntrusted_hash = 'sha256:acceptance'\n"
	if err := os.WriteFile(filepath.Join(privateHome, "config.toml"), []byte(private), 0600); err != nil {
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
	var config struct {
		Hooks struct {
			State map[string]struct {
				TrustedHash string `toml:"trusted_hash"`
			} `toml:"state"`
		} `toml:"hooks"`
	}
	if err := toml.Unmarshal(content, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Hooks.State) != 1 || config.Hooks.State[key].TrustedHash != "sha256:acceptance" {
		t.Fatalf("private hook trust = %+v, want only %s = sha256:acceptance:\n%s", config.Hooks.State, key, content)
	}
}

func TestResetCodexHomeKeepsOnlyConfigAndAuth(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "home")
	for _, dir := range []string{profile, filepath.Join(profile, "sessions", "2026")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"config.toml", "history.jsonl", "state.sqlite", filepath.Join("sessions", "2026", "rollout.jsonl")} {
		if err := os.WriteFile(filepath.Join(profile, name), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "auth.json"), filepath.Join(profile, "auth.json")); err != nil {
		t.Fatal(err)
	}
	if err := resetCodexHome(profile); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(profile)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !reflect.DeepEqual(names, []string{"auth.json", "config.toml"}) {
		t.Fatalf("reset left %v, want auth.json and config.toml", names)
	}
}

func TestLiveCodexDirectoryAdmitsOneRun(t *testing.T) {
	cache := t.TempDir()
	first, release, err := liveCodexDirectory(cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := liveCodexDirectory(cache); err == nil || !strings.Contains(err.Error(), "another live Codex acceptance run") {
		t.Fatalf("second run while locked: %v", err)
	}
	release()
	second, releaseAgain, err := liveCodexDirectory(cache)
	if err != nil {
		t.Fatalf("run after release: %v", err)
	}
	releaseAgain()
	if first != second || first != filepath.Join(cache, "gangline-acceptance", "codex") {
		t.Fatalf("directories %q and %q, want one fixed path", first, second)
	}
}

func TestCodexHookReviewStepNamesOperatorAction(t *testing.T) {
	for _, text := range []string{"Hooks need review\n› 1. Review hooks", "blocked: native hooks need review before they can run", "1 hook needs review"} {
		if !codexHooksAwaitReview(text) {
			t.Errorf("review not recognised in %q", text)
		}
	}
	if codexHooksAwaitReview("Trust this folder?") {
		t.Error("folder trust read as hook review")
	}
	step := codexHookReviewStep("/tmp/root", "/tmp/root/s.sock", "gangline-provider-codex", "%3", "/cache/home")
	for _, want := range []string{
		`tmux -S /tmp/root/s.sock attach-session -t gangline-provider-codex \; select-window -t %3 \; select-pane -t %3`,
		`"Trust all and continue"`,
		"tmux -S /tmp/root/s.sock kill-server; rm -r /tmp/root;",
		"/cache/home/config.toml only, not in ~/.codex",
	} {
		if !strings.Contains(step, want) {
			t.Errorf("step lacks %q:\n%s", want, step)
		}
	}
	if !strings.Contains(codexHookReviewStep("/r", "/r/s.sock", "s", "", "/h"), "select the worker window") {
		t.Error("step without a pane does not say which window to select")
	}
}

func TestRetainedCodexSessionBlocksUntilItsServerEnds(t *testing.T) {
	directory := t.TempDir()
	if err := refuseRetainedCodexSession(directory, func(string) bool { t.Fatal("probed without a record"); return false }); err != nil {
		t.Fatalf("no record: %v", err)
	}
	record := retainedCodexSession(directory)
	if err := os.WriteFile(record, []byte("/tmp/root/tmux.sock\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var probed string
	err := refuseRetainedCodexSession(directory, func(socket string) bool { probed = socket; return true })
	if err == nil || !strings.Contains(err.Error(), "/tmp/root/tmux.sock") || probed != "/tmp/root/tmux.sock" {
		t.Fatalf("live retained server: probed %q, err %v", probed, err)
	}
	if _, err := os.Stat(record); err != nil {
		t.Fatalf("record removed while its server lives: %v", err)
	}
	if err := refuseRetainedCodexSession(directory, func(string) bool { return false }); err != nil {
		t.Fatalf("ended retained server: %v", err)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatalf("record of an ended server kept: %v", err)
	}
}
