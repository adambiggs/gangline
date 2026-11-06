//go:build acceptance

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
	"syscall"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/pelletier/go-toml/v2"
)

const liveProviderTimeout = 60 * time.Second

func TestLiveClaudeCode(t *testing.T) { testLiveProvider(t, "claude", "claude", "sonnet") }
func TestLiveCodex(t *testing.T)      { testLiveProvider(t, "codex", "codex", "gpt-5.6-luna") }

func testLiveProvider(t *testing.T, collar, cli, cheapest string) {
	t.Helper()
	if os.Getenv("GANGLINE_LIVE_PROVIDERS") != "1" {
		t.Skip("live provider lane runs from test/acceptance.sh with visible output")
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
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// Select an owned Gangline state root before creating any fixtures. Keep
	// every asset inside that root, never the caller's live default state.
	// A short temporary path also keeps the private tmux socket addressable.
	root, err := os.MkdirTemp("", "gang-provider-")
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
	scratch := filepath.Join(root, "provider-acceptance")
	if err := os.Mkdir(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(scratch, "gang-"+collar)
	profile, kept := "", ""
	if cli == "codex" {
		// Codex keys hook trust to the rendered hook command, which names the
		// gang binary, and stores it in CODEX_HOME. A fixed binary path and a
		// kept private home let one operator review persist across runs
		// without touching the trust ~/.codex holds for the live team.
		cache, err := os.UserCacheDir()
		if err != nil {
			t.Fatalf("locate user cache for live Codex acceptance: %v", err)
		}
		stable, release, err := liveCodexDirectory(cache)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(release)
		kept = stable
		binary = filepath.Join(stable, "gang")
		profile = filepath.Join(stable, "home")
		if err := resetCodexHome(profile); err != nil {
			t.Fatal(err)
		}
		originalAuth := filepath.Join(home, ".codex", "auth.json")
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
	}
	ctx, cancel := context.WithTimeout(context.Background(), liveProviderTimeout)
	defer cancel()
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
		if err := seedCodexConfig(filepath.Join(home, ".codex"), profile, canonical); err != nil {
			t.Fatalf("prepare private Codex configuration for %s: %v", canonical, err)
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
	session := "gangline-provider-" + strings.ReplaceAll(collar, "-", "")
	socket := filepath.Join(root, "tmux.sock")
	collarDirectory := filepath.Join(repo, "harness", "collars")
	if override := os.Getenv("GANGLINE_ACCEPTANCE_COLLARS"); override != "" {
		collarDirectory = override
	}
	environment := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_TMUX", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN", "GANG_LAUNCH_ARGS", "CODEX_HOME"),
		"GANG_CONFIG_DIR="+filepath.Join(root, "config"),
		"GANG_SESSION="+session,
		"GANG_STATE_ROOT="+root,
		"GANG_TMUX_SOCKET="+socket,
		"GANG_COLLARS="+collarDirectory,
		"GANG_COLLAR="+collar)
	if cli == "codex" {
		environment = append(environment, "CODEX_HOME="+profile)
	}
	runner := tmuxRunner{binary: "tmux", socket: socket, env: environment}
	if kept != "" {
		// Record the server before it starts. Codex in it uses the kept
		// binary and home until the server ends, even if this process dies
		// before its cleanups run. Successful teardown removes the record.
		if err := recordRetainedCodexSession(kept, socket, codexTeardown(binary, root, socket, session, collarDirectory)); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := runner.run("new-session", "-d", "-s", session, "-n", "control"); err != nil {
		t.Fatalf("create private session: %v: %s", err, out)
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
				return
			}
		}
		if kept != "" {
			if err := os.Remove(retainedCodexSession(kept)); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Errorf("remove retained live Codex session record: %v", err)
			}
		}
	})
	if out, err := runner.run("list-sessions", "-F", "#{session_name}"); err != nil || strings.TrimSpace(out) != session {
		t.Fatalf("private socket has sessions %q: %v", out, err)
	}
	if collar == "claude" {
		options, err := runner.run("show-options", "-s")
		if err != nil {
			t.Fatalf("read private server options: %v: %s", err, options)
		}
		for _, setting := range liveClaudeInputSettings(options) {
			if out, err := runner.run(setting...); err != nil {
				t.Fatalf("private input setting %v: %v: %s", setting, err, out)
			}
			t.Logf("private input setting applied: %s", strings.Join(setting, " "))
		}
	}
	models, diagnostic, err := runGang(ctx, "", "models", "-c", collar, "--all")
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
	evidence := " <transcript_evidence>\n" + strings.Repeat("Synthetic delivery evidence. ", 32) + "\n\n  Indented é猫.\nAnother line.\nLast line.\nEnd.\n</transcript_evidence>"
	if collar == "claude" {
		assignment += evidence
	}
	hitch, hitchDiagnostic, err := runGang(ctx, "", "hitch", "worker", "-c", collar, "-d", canonical, "-m", cheapest, "-e", "low", "-t", assignment)
	if err != nil {
		if strings.Contains(hitchDiagnostic, "startup is queued") {
			if out, diagnostic, tickErr := runGang(ctx, "", "tick"); tickErr != nil {
				t.Fatalf("drain queued startup: %v: %s %s", tickErr, out, diagnostic)
			}
		} else {
			status, _, _ := runGang(ctx, "", "status", "worker", "--why")
			screen, _, _ := runGang(ctx, "", "capture", "worker", "-n", "30")
			if cli == "codex" && codexHooksAwaitReview(status+screen+hitchDiagnostic) {
				keepForOperator = true
				t.Fatalf("%s\ngang hitch: %v: %s %s\nstatus: %s\nscreen: %s", codexHookReviewStep(codexTeardown(binary, root, socket, session, collarDirectory), socket, session, workerPane(root, session), profile), err, hitch, hitchDiagnostic, status, screen)
			}
			if strings.Contains(strings.ToLower(status+screen+hitchDiagnostic), "trust") || strings.Contains(strings.ToLower(status+screen+hitchDiagnostic), "permission") {
				keepForOperator = true
				t.Fatalf("native prompt needs operator in private pane (socket %s, session %s): hitch=%s status=%s screen=%s", socket, session, hitchDiagnostic, status, screen)
			}
			t.Fatalf("gang hitch: %v: %s %s\nstatus: %s\nscreen: %s", err, hitch, hitchDiagnostic, status, screen)
		}
	}
	team, err := (store.Paths{Root: root}).Team(session)
	if err != nil {
		t.Fatal(err)
	}
	agents, err := team.ListAgents()
	if err != nil || len(agents) != 1 {
		t.Fatalf("private agents: %+v: %v", agents, err)
	}
	pane := agents[0].Pane
	if hitch != "" && strings.TrimSpace(hitch) != "worker" {
		t.Fatalf("unexpected hitch result: stdout=%q stderr=%q pane=%q", hitch, hitchDiagnostic, pane)
	}
	paths, err := team.Agent(agents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	startupWitness, err := awaitProviderWitness(ctx, paths, assignment)
	if err != nil || !strings.Contains(startupWitness.Prompt, "[gang#") || !strings.Contains(startupWitness.Prompt, "[gang ctx=") {
		diagnosticCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		status, _, _ := runGang(diagnosticCtx, "", "status", "worker", "--why")
		screen, _, _ := runGang(diagnosticCtx, "", "capture", "worker", "-n", "30")
		pending, _, _ := runGang(diagnosticCtx, "", "queue", "worker")
		if cli == "codex" && codexHooksAwaitReview(status+screen) {
			keepForOperator = true
			t.Fatalf("%s\nstartup witness: %v: %+v\nstatus: %s\nqueue: %s\nscreen: %s", codexHookReviewStep(codexTeardown(binary, root, socket, session, collarDirectory), socket, session, string(pane), profile), err, startupWitness, status, pending, screen)
		}
		t.Fatalf("startup hook did not witness pasted assignment and envelope: %v: %+v; socket=%s status=%s queue=%s screen=%s", err, startupWitness, socket, status, pending, screen)
	}
	second := "Reply with exactly SECOND. Do not use tools."
	if collar == "claude" {
		second += evidence
	}
	sent, sendDiagnostic, err := runGang(ctx, second, "send", "worker", "--from", "acceptance")
	if err != nil {
		t.Fatalf("mid-turn send: %v: %s %s", err, sent, sendDiagnostic)
	}
	sendFields := strings.Fields(sent)
	if len(sendFields) < 2 || (sendFields[1] != "delivered" && sendFields[1] != "accepted" && sendFields[1] != "queued") {
		t.Fatalf("unexpected send receipt: stdout=%q stderr=%q", sent, sendDiagnostic)
	}
	events, err := awaitProviderEvents(ctx, team.Log, sendFields[0], collar)
	if err != nil {
		screen, _, _ := runGang(ctx, "", "capture", "worker", "-n", "30")
		t.Fatalf("provider events: %v; initial receipt=%s pane=%s screen=%s timeline=%s", err, sendFields[1], pane, screen, providerTimeline(team.Log))
	}
	if collar == "claude" {
		authenticatedStatusline := false
		for _, event := range events {
			if event.Type != "observation" {
				continue
			}
			for _, reading := range event.Readings {
				if reading.Source == "status-line" {
					authenticatedStatusline = true
				}
			}
		}
		if !authenticatedStatusline {
			t.Fatal("native status-line child did not record an authenticated observation")
		}
		t.Log("native status-line child recorded an authenticated observation")
	}
	lastWitness, err := awaitProviderWitness(ctx, paths, second)
	if err != nil || !strings.Contains(lastWitness.Prompt, "[gang:") {
		t.Fatalf("second hook did not witness envelope and body: %v: %+v", err, lastWitness)
	}
	t.Logf("%s PASS startup and second hook, Stop, envelope, paste, mid-turn delivery initial receipt=%s pane=%s", collar, sendFields[1], pane)
}

func liveClaudeInputSettings(options string) [][]string {
	settings := [][]string{
		{"set-option", "-s", "extended-keys", "on"},
	}
	for _, line := range strings.Split(options, "\n") {
		if strings.HasPrefix(line, "extended-keys-format ") {
			settings = append(settings, []string{"set-option", "-s", "extended-keys-format", "xterm"})
			break
		}
	}
	return append(settings, []string{"set-option", "-g", "default-terminal", "tmux-256color"})
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
		if collar == "claude" {
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

// liveCodexDirectory returns the per-user directory that keeps the live Codex
// check's gang binary and private CODEX_HOME at fixed paths, locked for this
// run. Clearing the user cache costs only a new operator review.
func liveCodexDirectory(cache string) (string, func(), error) {
	directory := filepath.Join(cache, "gangline-acceptance", "codex")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", nil, err
	}
	lock, err := os.OpenFile(filepath.Join(directory, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return "", nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		return "", nil, fmt.Errorf("another live Codex acceptance run holds %s: %w", lock.Name(), err)
	}
	if err := refuseRetainedCodexSession(directory, tmuxServerAlive); err != nil {
		_ = lock.Close()
		return "", nil, err
	}
	return directory, func() { _ = lock.Close() }, nil
}

// retainedCodexSession names the file that records the socket of a live Codex
// run's private tmux server. Codex in that server uses the kept binary and home
// for as long as the server lives, including after its test process ends.
func retainedCodexSession(directory string) string {
	return filepath.Join(directory, "retained-socket")
}

// codexTeardown is the operator's command to end a kept run: the run's own
// gang down unloads its watchdog job and stops its agents before the server
// and root are removed.
func codexTeardown(binary, root, socket, session, collars string) string {
	return fmt.Sprintf("env -u TMUX -u TMUX_PANE GANG_STATE_ROOT=%s GANG_CONFIG_DIR=%s GANG_SESSION=%s GANG_TMUX_SOCKET=%s GANG_COLLARS=%s %s down --yes && { tmux -S %s kill-server; rm -r %s; }",
		shellQuote(root), shellQuote(filepath.Join(root, "config")), shellQuote(session), shellQuote(socket), shellQuote(collars), shellQuote(binary), shellQuote(socket), shellQuote(root))
}

func recordRetainedCodexSession(directory, socket, teardown string) error {
	file, err := os.CreateTemp(directory, "retained-socket-*")
	if err != nil {
		return fmt.Errorf("record live Codex session: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(socket + "\n" + teardown + "\n"); err != nil {
		_ = file.Close()
		return fmt.Errorf("record live Codex session: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("record live Codex session: %w", err)
	}
	if err := os.Rename(file.Name(), retainedCodexSession(directory)); err != nil {
		return fmt.Errorf("record live Codex session: %w", err)
	}
	return nil
}

// refuseRetainedCodexSession stops a new run while an earlier run's tmux
// server may still be alive, so the new build and reset do not change files
// under it. A record whose server has ended is removed.
func refuseRetainedCodexSession(directory string, alive func(socket string) (bool, error)) error {
	record := retainedCodexSession(directory)
	content, err := os.ReadFile(record)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("read retained live Codex session: %w", err)
	}
	socket, teardown, _ := strings.Cut(strings.TrimSpace(string(content)), "\n")
	socket, teardown = strings.TrimSpace(socket), strings.TrimSpace(teardown)
	if teardown == "" {
		teardown = "tmux -S " + shellQuote(socket) + " kill-server"
	}
	end := fmt.Sprintf("Do any review its pane still needs, then end it with: %s; and rerun.", teardown)
	if socket == "" {
		return fmt.Errorf("live Codex session record %s is empty; confirm no earlier run's tmux server uses %s, then remove the record", record, directory)
	}
	running, err := alive(socket)
	if err != nil {
		return fmt.Errorf("cannot tell whether an earlier live Codex run's tmux server at %s still uses %s: %v. %s", socket, directory, err, end)
	}
	if running {
		return fmt.Errorf("an earlier live Codex run's tmux server at %s is still alive and uses %s. %s", socket, directory, end)
	}
	if err := os.Remove(record); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove retained live Codex session record: %w", err)
	}
	return nil
}

// tmuxServerAlive reports whether a tmux server answers on socket. A missing
// socket or a refused connection means the server has ended; any other failure
// is unknown.
func tmuxServerAlive(socket string) (bool, error) {
	if _, err := os.Stat(socket); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", "-S", socket, "list-sessions").CombinedOutput()
	if err == nil {
		return true, nil
	}
	if ctx.Err() != nil {
		return false, fmt.Errorf("tmux did not answer within 5s")
	}
	text := strings.ToLower(string(out))
	if strings.Contains(text, "no server running") || strings.Contains(text, "connection refused") {
		return false, nil
	}
	return false, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
}

// resetCodexHome empties the kept private CODEX_HOME of everything a previous
// run left except config.toml, which holds the operator's hook trust, and the
// auth.json link.
func resetCodexHome(profile string) error {
	if err := os.MkdirAll(profile, 0700); err != nil {
		return err
	}
	entries, err := os.ReadDir(profile)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "config.toml" || entry.Name() == "auth.json" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(profile, entry.Name())); err != nil {
			return fmt.Errorf("reset private Codex home: %w", err)
		}
	}
	return nil
}

func codexHooksAwaitReview(text string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, "hooks need review") || strings.Contains(text, "hook needs review")
}

// workerPane names the private worker's pane when the team recorded one.
func workerPane(root, session string) string {
	team, err := (store.Paths{Root: root}).Team(session)
	if err != nil {
		return ""
	}
	agents, err := team.ListAgents()
	if err != nil || len(agents) != 1 {
		return ""
	}
	return string(agents[0].Pane)
}

func codexHookReviewStep(teardown, socket, session, pane, profile string) string {
	attach := fmt.Sprintf("tmux -S %s attach-session -t %s", shellQuote(socket), shellQuote(session))
	if pane != "" {
		attach += fmt.Sprintf(" \\; select-window -t %s \\; select-pane -t %s", shellQuote(pane), shellQuote(pane))
	} else {
		attach += " (then select the worker window)"
	}
	return fmt.Sprintf("Codex is waiting for the operator to review Gangline's hooks. One-time step: run %s; optionally choose \"Review hooks\" to read them; choose \"Trust all and continue\"; type /quit; detach from tmux; run %s; rerun acceptance. The trust is stored in %s only, not in ~/.codex. Repeat when the collar's Codex hooks change.", attach, teardown, filepath.Join(profile, "config.toml"))
}

func seedCodexConfig(regularHome, privateHome, cwd string) error {
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

	// Rebuild the private config from the regular one on every run so it
	// matches a fresh hitch. Only the operator's hook trust carries over.
	configPath := filepath.Join(privateHome, "config.toml")
	isolated := make(map[string]any)
	info, err := os.Lstat(configPath)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("private Codex config %s is not a regular file", configPath)
		}
		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("read private Codex config: %w", err)
		}
		var previous struct {
			Hooks struct {
				State map[string]any `toml:"state"`
			} `toml:"hooks"`
		}
		if err := toml.Unmarshal(content, &previous); err != nil {
			return fmt.Errorf("parse private Codex config: %w", err)
		}
		if len(previous.Hooks.State) > 0 {
			isolated["hooks"] = map[string]any{"state": previous.Hooks.State}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect private Codex config: %w", err)
	}
	// Authentication belongs to the selected connection. Copying auth.json
	// without its provider route can send the same login to a different API.
	if provider, ok := regular["model_provider"].(string); ok && provider != "" {
		isolated["model_provider"] = provider
		providers, _ := regular["model_providers"].(map[string]any)
		if configured, exists := providers[provider]; exists {
			connection, ok := configured.(map[string]any)
			if !ok {
				return fmt.Errorf("selected Codex provider configuration is not a table")
			}
			selected := make(map[string]any)
			for _, key := range []string{
				"name", "base_url", "wire_api", "env_key", "env_key_instructions",
				"requires_openai_auth", "experimental_bearer_token", "auth",
				"http_headers", "env_http_headers", "query_params",
			} {
				if value, exists := connection[key]; exists {
					selected[key] = value
				}
			}
			isolated["model_providers"] = map[string]any{provider: selected}
		}
	}
	isolated["projects"] = map[string]any{cwd: map[string]any{"trust_level": "trusted"}}
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
