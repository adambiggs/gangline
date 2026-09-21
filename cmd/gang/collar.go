package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

func (cmd command) collar(arguments []string) error {
	if len(arguments) != 2 || arguments[0] != "check" {
		return usageError("collar: expected 'check NAME'")
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	collar, err := loadCollar(arguments[1], settings)
	if err != nil {
		return err
	}
	id, directory, sockets, err := cmd.prepareCollarCheck(settings.StateRoot)
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	defer os.Remove(sockets[0])
	defer os.Remove(sockets[1])

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	results := unknownProbeResults()
	trustBackend, err := tmux.New(cmd.tmuxConfig(sockets[0], "gang-check-trust-"+id))
	if err != nil {
		return err
	}
	trustResults, err := probeTrust(ctx, trustBackend, collar, directory)
	if err != nil {
		return err
	}
	for name, result := range trustResults {
		results[name] = result
	}

	activeBackend, err := tmux.New(cmd.tmuxConfig(sockets[1], "gang-check-active-"+id))
	if err != nil {
		return err
	}
	workdir, err := activeProbeDirectory(cmd.getwd)
	if err != nil {
		return err
	}
	activeResults, err := cmd.probeActive(ctx, activeBackend, collar, settings, directory, workdir)
	if err != nil {
		return err
	}
	for name, result := range activeResults {
		results[name] = result
	}
	report := harness.CheckReport{
		Collar:         collar.Name,
		HarnessVersion: installedHarnessVersion(collar),
	}
	for _, name := range harness.RequiredProbes() {
		report.Results = append(report.Results, results[name])
	}
	return cmd.printCheckReport(report)
}

func (cmd command) prepareCollarCheck(root string) (string, string, [2]string, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", "", [2]string{}, err
	}
	id, err := randomID("check")
	if err != nil {
		return "", "", [2]string{}, err
	}
	directory := filepath.Join(root, "native-project-"+id)
	if err := os.Mkdir(directory, 0o775); err != nil {
		return "", "", [2]string{}, err
	}
	if output, err := exec.Command("git", "-C", directory, "init", "--quiet").CombinedOutput(); err != nil {
		return "", "", [2]string{}, fmt.Errorf("initialize disposable collar-check project: %w: %s", err, strings.TrimSpace(string(output)))
	}
	home, err := cmd.userHomeDir()
	if err != nil {
		return "", "", [2]string{}, err
	}
	socketRoot := filepath.Join(home, ".local", "state")
	if err := os.MkdirAll(socketRoot, 0o700); err != nil {
		return "", "", [2]string{}, err
	}
	shortID := id
	if len(shortID) > 18 {
		shortID = shortID[:18]
	}
	sockets := [2]string{
		filepath.Join(socketRoot, "gl-"+shortID+"-t.sock"),
		filepath.Join(socketRoot, "gl-"+shortID+"-a.sock"),
	}
	return id, directory, sockets, nil
}

func probeTrust(ctx context.Context, backend *tmux.Backend, collar harness.Collar, directory string) (map[string]harness.ProbeResult, error) {
	results := make(map[string]harness.ProbeResult)
	defer backend.KillSession(context.Background())
	trustCollar := collar
	trustCollar.Hooks = nil
	launch, err := harness.RenderLaunch(trustCollar, harness.LaunchOptions{})
	if err != nil {
		return nil, err
	}
	pane, err := backend.CreateSession(ctx, launch.SpawnSpec("probe", directory))
	if err != nil {
		results[harness.ProbeLaunch] = harness.ProbeResult{Name: harness.ProbeLaunch, Detail: err.Error()}
		return results, nil
	}
	results[harness.ProbeLaunch] = harness.ProbeResult{Name: harness.ProbeLaunch, Passed: true, Detail: "native process launched in a private tmux server"}
	startup, _, err := awaitStartup(ctx, backend, pane.ID, collar)
	if err != nil {
		results[harness.ProbeTrustPrompt] = harness.ProbeResult{Name: harness.ProbeTrustPrompt, Detail: err.Error()}
		return results, nil
	}
	detail := "persisted native trust admitted the disposable project"
	if startup.State == harness.StartupTrustRequired {
		detail = startup.Prompt
	}
	results[harness.ProbeTrustPrompt] = harness.ProbeResult{Name: harness.ProbeTrustPrompt, Passed: true, Detail: detail}
	return results, nil
}

func (cmd command) probeActive(ctx context.Context, backend *tmux.Backend, collar harness.Collar, settings settings, directory, workdir string) (map[string]harness.ProbeResult, error) {
	results := make(map[string]harness.ProbeResult)
	defer backend.KillSession(context.Background())
	fifo := filepath.Join(directory, "hook.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		return nil, err
	}
	hook := []string{"sh", "-c", "cat > \"$1\"", "gang-collar-check", fifo}
	launch, err := harness.RenderLaunch(collar, harness.LaunchOptions{HookCommand: hook, Probe: true})
	if err != nil {
		return nil, err
	}
	launch = applyLaunchPolicy(launch, collar.Name, settings)
	pane, err := backend.CreateSession(ctx, launch.SpawnSpec("probe", workdir))
	if err != nil {
		return results, nil
	}
	startup, screen, err := awaitStartup(ctx, backend, pane.ID, collar)
	if err != nil {
		results[harness.ProbeComposer] = harness.ProbeResult{Name: harness.ProbeComposer, Detail: err.Error()}
		return results, nil
	}
	if startup.State == harness.StartupTrustRequired {
		results[harness.ProbeTrustPrompt] = harness.ProbeResult{Name: harness.ProbeTrustPrompt, Passed: true, Detail: startup.Prompt}
	}
	if startup.State != harness.StartupReady {
		return results, nil
	}
	composer, err := harness.ReadComposer(collar.Primitives.Composer, screen)
	if err != nil {
		results[harness.ProbeComposer] = harness.ProbeResult{Name: harness.ProbeComposer, Detail: err.Error()}
		return results, nil
	}
	if composer.Text != "" {
		results[harness.ProbeComposer] = harness.ProbeResult{Name: harness.ProbeComposer, Detail: "native composer was not empty"}
		return results, nil
	}
	results[harness.ProbeComposer] = harness.ProbeResult{Name: harness.ProbeComposer, Passed: true, Detail: "native empty composer detected"}
	action, _ := harness.Submit(collar.Primitives.Submit, "Reply with exactly READY.")
	settle, _ := harness.SubmitSettle(collar.Primitives.Submit)
	input, _ := harness.SubmitInput(collar.Primitives.Submit, action.Text)
	payload, err := awaitNativeHook(ctx, fifo, func() error {
		if err := sendHarnessKeys(ctx, backend, pane.ID, collar, input); err != nil {
			return err
		}
		if err := awaitComposerText(ctx, backend, pane.ID, collar, action.Text, settle); err != nil {
			return err
		}
		return sendHarnessKeys(ctx, backend, pane.ID, collar, substrate.Keys{Names: action.Keys, Submit: action.Submit})
	})
	if err != nil {
		results[harness.ProbeSubmit] = harness.ProbeResult{Name: harness.ProbeSubmit, Detail: err.Error()}
		return results, nil
	}
	results[harness.ProbeSubmit] = harness.ProbeResult{Name: harness.ProbeSubmit, Passed: true, Detail: "native submit produced a hook witness"}
	boundary, _, err := harness.DetectTurnBoundary(collar, payload)
	if err != nil {
		results[harness.ProbeHook] = harness.ProbeResult{Name: harness.ProbeHook, Detail: err.Error()}
		return results, nil
	}
	results[harness.ProbeHook] = harness.ProbeResult{Name: harness.ProbeHook, Passed: true, Detail: "native hook payload decoded"}
	if boundary == harness.TurnStarted {
		results[harness.ProbeTurnBoundary] = harness.ProbeResult{Name: harness.ProbeTurnBoundary, Passed: true, Detail: "native turn-start boundary decoded"}
	} else {
		results[harness.ProbeTurnBoundary] = harness.ProbeResult{Name: harness.ProbeTurnBoundary, Detail: fmt.Sprintf("first native hook decoded as %q", boundary)}
	}
	return results, nil
}

func unknownProbeResults() map[string]harness.ProbeResult {
	results := make(map[string]harness.ProbeResult)
	for _, name := range harness.RequiredProbes() {
		results[name] = harness.ProbeResult{Name: name, Detail: "unknown: probe did not run"}
	}
	return results
}

func (cmd command) printCheckReport(report harness.CheckReport) error {
	for _, result := range report.Results {
		mark := "FAIL"
		if result.Passed {
			mark = "PASS"
		}
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", mark, result.Name, result.Detail); err != nil {
			return err
		}
	}
	if report.Passed() {
		return nil
	}
	fmt.Fprintln(cmd.stdout)
	fmt.Fprint(cmd.stdout, report.IssueBody())
	fmt.Fprintln(cmd.stdout, report.IssueCommand("adambiggs/gangline"))
	return commandError{status: exitNative, text: "collar check incomplete; native prompts are never auto-answered"}
}
