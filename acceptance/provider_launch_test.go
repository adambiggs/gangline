package acceptance

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/pelletier/go-toml/v2"
)

const liveProviderTimeout = 60 * time.Second

func TestLiveClaudeCode(t *testing.T) { testLiveProvider(t, "claude-code", "claude", "sonnet") }
func TestLiveCodex(t *testing.T)      { testLiveProvider(t, "codex", "codex", "gpt-6-luna") }

func testLiveProvider(t *testing.T, collar, cli, cheapest string) {
	t.Helper()
	if os.Getenv("GANGLINE_LIVE_PROVIDERS") != "1" {
		t.Skip("live provider lane runs from test/go.sh with visible output")
	}
	if _, err := exec.LookPath(cli); err != nil {
		t.Skipf("%s is absent from PATH", cli)
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatal("tmux is required for live provider acceptance")
	}
	authCtx, cancelAuth := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelAuth()
	var auth *exec.Cmd
	if cli == "claude" {
		auth = exec.CommandContext(authCtx, cli, "auth", "status")
	} else {
		auth = exec.CommandContext(authCtx, cli, "login", "status")
	}
	output, err := auth.CombinedOutput()
	if authCtx.Err() != nil {
		t.Fatalf("%s authentication probe exceeded 10s", cli)
	}
	if err != nil {
		t.Skipf("%s is not authenticated: %s", cli, strings.TrimSpace(string(output)))
	}
	if cli == "claude" {
		var status struct {
			LoggedIn bool `json:"loggedIn"`
		}
		if err := json.Unmarshal(output, &status); err != nil {
			t.Fatalf("parse claude auth status: %v", err)
		}
		if !status.LoggedIn {
			t.Skip("claude auth status reports loggedIn=false")
		}
	}

	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	stateHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(stateHome, ".local", "state", "gangline-provider-acceptance")
	if err := os.MkdirAll(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	if cli == "codex" {
		profile := filepath.Join(scratch, "codex-home")
		if err := os.MkdirAll(profile, 0700); err != nil {
			t.Fatal(err)
		}
		originalAuth := filepath.Join(stateHome, ".codex", "auth.json")
		isolatedAuth := filepath.Join(profile, "auth.json")
		if _, err := os.Lstat(isolatedAuth); errors.Is(err, os.ErrNotExist) {
			if _, err := os.Stat(originalAuth); err == nil {
				if err := os.Symlink(originalAuth, isolatedAuth); err != nil {
					t.Fatal(err)
				}
			}
		} else if err != nil {
			t.Fatal(err)
		}
		isolatedCtx, cancelIsolated := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelIsolated()
		isolated := exec.CommandContext(isolatedCtx, cli, "login", "status")
		isolated.Env = append(withoutEnvironment(os.Environ(), "CODEX_HOME"), "CODEX_HOME="+profile)
		if out, err := isolated.CombinedOutput(); isolatedCtx.Err() != nil {
			t.Fatalf("isolated Codex authentication probe exceeded 10s: %v: %s", err, out)
		} else if err != nil {
			t.Skipf("codex authentication unavailable in isolated CODEX_HOME: %s", strings.TrimSpace(string(out)))
		}
	}
	root, err := os.MkdirTemp(scratch, collar+"-")
	if err != nil {
		t.Fatal(err)
	}
	keepForOperator := false
	t.Cleanup(func() {
		if !keepForOperator {
			if err := os.RemoveAll(root); err != nil {
				t.Errorf("remove private test state: %v", err)
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), liveProviderTimeout)
	defer cancel()
	binary := filepath.Join(scratch, "gang-"+collar)
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/gang")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gang: %v\n%s", err, out)
	}
	canonical := repo
	common := exec.CommandContext(ctx, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	common.Dir = repo
	if out, err := common.Output(); err == nil {
		canonical = filepath.Dir(strings.TrimSpace(string(out)))
	}
	if cli == "codex" {
		if err := seedCodexTrust(filepath.Join(stateHome, ".codex"), filepath.Join(scratch, "codex-home"), canonical); err != nil {
			t.Fatalf("prepare private Codex trust for %s: %v", canonical, err)
		}
	}
	session := "gangline-provider-" + strings.ReplaceAll(collar, "-", "")
	socket := filepath.Join(root, "tmux.sock")
	collarDirectory := filepath.Join(repo, "harness", "collars")
	if override := os.Getenv("GANGLINE_ACCEPTANCE_COLLARS"); override != "" {
		collarDirectory = override
	}
	environment := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_LAUNCH_ARGS", "CODEX_HOME"),
		"GANG_CONFIG_DIR="+filepath.Join(root, "config"),
		"GANG_SESSION="+session,
		"GANG_STATE_ROOT="+filepath.Join(root, "state"),
		"GANG_TMUX_SOCKET="+socket,
		"GANG_COLLARS="+collarDirectory,
		"GANG_COLLAR="+collar)
	if cli == "codex" {
		environment = append(environment, "CODEX_HOME="+filepath.Join(scratch, "codex-home"))
	}
	runner := tmuxRunner{binary: "tmux", socket: socket, env: environment}
	if out, err := runner.run("new-session", "-d", "-s", session, "-n", "control"); err != nil {
		t.Fatalf("create private session: %v: %s", err, out)
	}
	if out, err := runner.run("list-sessions", "-F", "#{session_name}"); err != nil || strings.TrimSpace(out) != session {
		t.Fatalf("private socket has sessions %q: %v", out, err)
	}
	runGang := func(c context.Context, input string, args ...string) (string, string, error) {
		command := exec.CommandContext(c, binary, args...)
		command.Dir = repo
		command.Env = environment
		command.Stdin = strings.NewReader(input)
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		return stdout.String(), stderr.String(), err
	}
	t.Cleanup(func() {
		if keepForOperator {
			return
		}
		cleanCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if out, diagnostic, err := runGang(cleanCtx, "", "roster"); err != nil {
			t.Errorf("private roster before cleanup: %v: %s %s", err, out, diagnostic)
		}
		if out, diagnostic, err := runGang(cleanCtx, "", "down", "--yes"); err != nil {
			keepForOperator = true
			t.Errorf("remove private team: %v: %s %s; socket retained at %s", err, out, diagnostic, socket)
			return
		}
		if _, err := runner.run("has-session", "-t", session); err == nil {
			if out, err := runner.run("kill-session", "-t", session); err != nil {
				keepForOperator = true
				t.Errorf("remove private control session: %v: %s", err, out)
			}
		}
	})
	models, diagnostic, err := runGang(ctx, "", "models", "-c", collar)
	if err != nil {
		t.Fatalf("gang models -c %s: %v: %s %s", collar, err, models, diagnostic)
	}
	modelFound := false
	for _, line := range strings.Split(models, "\n") {
		if strings.HasPrefix(line, cheapest+"\t") && strings.Contains(line, "low") {
			modelFound = true
		}
	}
	if !modelFound {
		t.Fatalf("cheapest expected model %s with low effort absent from gang models -c %s:\n%s", cheapest, collar, models)
	}
	t.Logf("%s model=%s effort=low private_socket=%s", collar, cheapest, socket)
	assignment := "Reply with 200 short numbered lines, each containing FIRST. Do not use tools."
	hitch, hitchDiagnostic, err := runGang(ctx, "", "hitch", "worker", "-c", collar, "-d", canonical, "-m", cheapest, "-e", "low", "-t", assignment)
	if err != nil {
		if strings.Contains(hitchDiagnostic, "startup is queued") {
			if out, diagnostic, tickErr := runGang(ctx, "", "tick"); tickErr != nil {
				t.Fatalf("drain queued startup: %v: %s %s", tickErr, out, diagnostic)
			}
		} else {
			status, _, _ := runGang(ctx, "", "status", "worker", "--why")
			screen, _, _ := runGang(ctx, "", "capture", "worker", "30")
			if strings.Contains(strings.ToLower(status+screen+hitchDiagnostic), "trust") || strings.Contains(strings.ToLower(status+screen+hitchDiagnostic), "permission") {
				keepForOperator = true
				t.Fatalf("native prompt needs operator in private pane (socket %s, session %s): hitch=%s status=%s screen=%s", socket, session, hitchDiagnostic, status, screen)
			}
			t.Fatalf("gang hitch: %v: %s %s\nstatus: %s\nscreen: %s", err, hitch, hitchDiagnostic, status, screen)
		}
	}
	team, err := (store.Paths{Root: filepath.Join(root, "state")}).Team(session)
	if err != nil {
		t.Fatal(err)
	}
	agents, err := team.ListAgents()
	if err != nil || len(agents) != 1 {
		t.Fatalf("private agents: %+v: %v", agents, err)
	}
	pane := agents[0].Pane
	if hitch != "" && strings.TrimSpace(hitch) != "worker\t"+pane {
		t.Fatalf("unexpected hitch result: stdout=%q stderr=%q pane=%q", hitch, hitchDiagnostic, pane)
	}
	paths, err := team.Agent(agents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	startupWitness, err := awaitProviderWitness(ctx, paths, assignment)
	if err != nil || !strings.Contains(startupWitness.Prompt, "[gang:") {
		diagnosticCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		status, _, _ := runGang(diagnosticCtx, "", "status", "worker", "--why")
		screen, _, _ := runGang(diagnosticCtx, "", "capture", "worker", "30")
		pending, _, _ := runGang(diagnosticCtx, "", "queue", "worker")
		if strings.Contains(strings.ToLower(status+screen), "hooks need review") {
			keepForOperator = true
		}
		t.Fatalf("startup hook did not witness pasted assignment and envelope: %v: %+v; socket=%s status=%s queue=%s screen=%s", err, startupWitness, socket, status, pending, screen)
	}
	second := "Reply with exactly SECOND. Do not use tools."
	sent, sendDiagnostic, err := runGang(ctx, second, "send", "worker", "--from", "acceptance")
	if err != nil {
		t.Fatalf("mid-turn send: %v: %s %s", err, sent, sendDiagnostic)
	}
	sendFields := strings.Fields(sent)
	if len(sendFields) < 2 || (sendFields[1] != "delivered" && sendFields[1] != "accepted" && sendFields[1] != "queued") {
		t.Fatalf("unexpected send receipt: stdout=%q stderr=%q", sent, sendDiagnostic)
	}
	if _, err := awaitProviderEvents(ctx, team.Log, sendFields[0], collar); err != nil {
		screen, _, _ := runGang(ctx, "", "capture", "worker", "30")
		t.Fatalf("provider events: %v; initial receipt=%s pane=%s screen=%s timeline=%s", err, sendFields[1], pane, screen, providerTimeline(team.Log))
	}
	lastWitness, err := awaitProviderWitness(ctx, paths, second)
	if err != nil || !strings.Contains(lastWitness.Prompt, "[gang:") {
		t.Fatalf("second hook did not witness envelope and body: %v: %+v", err, lastWitness)
	}
	t.Logf("%s PASS startup and second hook, Stop, envelope, paste, mid-turn delivery initial receipt=%s pane=%s", collar, sendFields[1], pane)
}

func providerTimeline(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return err.Error()
	}
	defer file.Close()
	var items []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event core.Event
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if event.Type == "native_hook" || event.Type == "send_queued" || event.Type == "delivery_succeeded" || event.Type == "input_finished" {
			items = append(items, fmt.Sprintf("%s %s %s %s", event.At.Format("15:04:05.000"), event.Type, event.NativeEvent, event.Status))
		}
	}
	if err := scanner.Err(); err != nil {
		return err.Error()
	}
	return strings.Join(items, "; ")
}

func awaitProviderWitness(ctx context.Context, paths store.AgentPaths, expected string) (store.Witness, error) {
	for {
		watch, err := store.Watch(paths.Witness)
		if err != nil {
			return store.Witness{}, err
		}
		witness, err := paths.ReadWitness()
		if err == nil && strings.Contains(witness.Prompt, expected) {
			_ = watch.Close()
			return witness, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = watch.Close()
			return store.Witness{}, err
		}
		if err := watch.Wait(ctx); err != nil {
			_ = watch.Close()
			return store.Witness{}, err
		}
		if err := watch.Close(); err != nil {
			return store.Witness{}, err
		}
	}
}

func awaitProviderEvents(ctx context.Context, path, secondID, collar string) ([]core.Event, error) {
	for {
		watch, err := store.Watch(path)
		if err != nil {
			return nil, err
		}
		file, err := os.Open(path)
		if err != nil {
			_ = watch.Close()
			return nil, err
		}
		var events []core.Event
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var event core.Event
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				_ = file.Close()
				_ = watch.Close()
				return nil, err
			}
			events = append(events, event)
		}
		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			_ = watch.Close()
			return nil, scanErr
		}
		firstStart, secondStart, firstStop, lastStop, secondQueued, secondAccepted, secondDelivered, starts, stops := -1, -1, -1, -1, -1, -1, -1, 0, 0
		for i, event := range events {
			if event.Type == "native_hook" && event.NativeEvent == "UserPromptSubmit" && event.Status == "turn-started" {
				starts++
				if firstStart == -1 {
					firstStart = i
				} else if secondStart == -1 {
					secondStart = i
				}
			}
			if event.Type == "native_hook" && event.NativeEvent == "Stop" && event.Status == "turn-finished" {
				stops++
				if firstStop == -1 {
					firstStop = i
				}
				lastStop = i
			}
			if event.Type == "send_queued" && event.Envelope != nil && string(event.Envelope.ID) == secondID {
				secondQueued = i
			}
			if event.Type == "delivery_succeeded" && event.ID == secondID {
				secondDelivered = i
			}
			if event.Type == "input_finished" && event.ID == secondID && event.Status == "accepted" {
				secondAccepted = i
			}
		}
		requiredStops := 1
		if collar == "claude-code" {
			requiredStops = 2
		}
		if starts >= 2 && stops >= requiredStops && secondDelivered >= 0 && (collar != "codex" || secondAccepted >= 0) {
			_ = watch.Close()
			if firstStart < 0 || secondQueued <= firstStart || secondStart <= secondQueued || lastStop <= secondStart || firstStop <= secondQueued {
				return nil, fmt.Errorf("second send was not queued during first native turn: start=%d queued=%d second_start=%d first_stop=%d last_stop=%d", firstStart, secondQueued, secondStart, firstStop, lastStop)
			}
			if collar == "codex" && (secondAccepted <= secondQueued || secondAccepted >= secondDelivered) {
				return nil, fmt.Errorf("Codex native queue was not accepted before delivery: queued=%d accepted=%d delivered=%d", secondQueued, secondAccepted, secondDelivered)
			}
			return events, nil
		}
		if err := watch.Wait(ctx); err != nil {
			_ = watch.Close()
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("hard %s deadline: starts=%d stops=%d second accepted=%t delivered=%t", liveProviderTimeout, starts, stops, secondAccepted >= 0, secondDelivered >= 0)
			}
			return nil, err
		}
		if err := watch.Close(); err != nil {
			return nil, err
		}
	}
}

func seedCodexTrust(regularHome, privateHome, cwd string) error {
	regularConfig, err := os.ReadFile(filepath.Join(regularHome, "config.toml"))
	if err != nil {
		return fmt.Errorf("read regular Codex config: %w", err)
	}
	var regular map[string]any
	if err := toml.Unmarshal(regularConfig, &regular); err != nil {
		return fmt.Errorf("parse regular Codex config: %w", err)
	}
	if codexTrustLevel(regular, cwd) != "trusted" {
		return fmt.Errorf("regular Codex config does not trust %s; trust it interactively before running live acceptance", cwd)
	}

	configPath := filepath.Join(privateHome, "config.toml")
	var isolated map[string]any
	info, err := os.Lstat(configPath)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("private Codex config %s is not a regular file", configPath)
		}
		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("read private Codex config: %w", err)
		}
		if err := toml.Unmarshal(content, &isolated); err != nil {
			return fmt.Errorf("parse private Codex config: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect private Codex config: %w", err)
	}
	if isolated == nil {
		isolated = make(map[string]any)
	}
	projects, ok := isolated["projects"].(map[string]any)
	if isolated["projects"] != nil && !ok {
		return fmt.Errorf("private Codex projects config has unexpected type %T", isolated["projects"])
	}
	if !ok {
		projects = make(map[string]any)
		isolated["projects"] = projects
	}
	project, ok := projects[cwd].(map[string]any)
	if projects[cwd] != nil && !ok {
		return fmt.Errorf("private Codex project %s has unexpected type %T", cwd, projects[cwd])
	}
	if !ok {
		project = make(map[string]any)
		projects[cwd] = project
	}
	project["trust_level"] = "trusted"
	content, err := toml.Marshal(isolated)
	if err != nil {
		return fmt.Errorf("encode private Codex config: %w", err)
	}
	file, err := os.CreateTemp(privateHome, "config-*.toml")
	if err != nil {
		return fmt.Errorf("create private Codex config: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write private Codex config: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close private Codex config: %w", err)
	}
	if err := os.Rename(file.Name(), configPath); err != nil {
		return fmt.Errorf("replace private Codex config: %w", err)
	}
	return nil
}

func codexTrustLevel(config map[string]any, cwd string) string {
	projects, ok := config["projects"].(map[string]any)
	if !ok {
		return ""
	}
	project, ok := projects[cwd].(map[string]any)
	if !ok {
		return ""
	}
	level, _ := project["trust_level"].(string)
	return level
}
