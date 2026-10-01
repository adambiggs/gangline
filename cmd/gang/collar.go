package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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

func (cmd command) collar(arguments []string) (result error) {
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
	var cleanup []error
	var keepSocket [2]bool
	defer func() {
		cleanup = append(cleanup, removeCollarCheckFiles(directory, sockets, keepSocket))
		result = withCleanupError(result, errors.Join(cleanup...))
	}()
	// A session that could not be stopped keeps its socket, so the leaked
	// server stays reachable at the path the error names.
	stop := func(index int, backend *tmux.Backend, session string) {
		if err := stopProbeSession(backend); err != nil {
			keepSocket[index] = true
			cleanup = append(cleanup, fmt.Errorf("stop tmux session %s on %s: %w", session, sockets[index], err))
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	results := unknownProbeResults()
	trustSession := "gang-check-trust-" + id
	trustBackend, err := tmux.New(cmd.tmuxConfig(sockets[0], trustSession))
	if err != nil {
		return err
	}
	trustResults, err := probeTrust(ctx, trustBackend, collar, directory)
	stop(0, trustBackend, trustSession)
	if err != nil {
		return err
	}
	for name, result := range trustResults {
		results[name] = result
	}

	activeSession := "gang-check-active-" + id
	activeBackend, err := tmux.New(cmd.tmuxConfig(sockets[1], activeSession))
	if err != nil {
		return err
	}
	workdir, err := activeProbeDirectory(cmd.getwd)
	if err != nil {
		return err
	}
	activeResults, err := cmd.probeActive(ctx, activeBackend, collar, settings, directory, workdir)
	stop(1, activeBackend, activeSession)
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

// stopProbeSession ends a probe's private tmux session. A kill that fails
// because no session exists, including one that ended on its own, is not a
// failure.
func stopProbeSession(backend *tmux.Backend) error {
	err := backend.KillSession(context.Background())
	if err == nil {
		return nil
	}
	if exists, existsErr := backend.SessionExists(context.Background()); existsErr == nil && !exists {
		return nil
	}
	return err
}

func removeCollarCheckFiles(directory string, sockets [2]string, keep [2]bool) error {
	errs := []error{os.RemoveAll(directory)}
	for index, socket := range sockets {
		if keep[index] {
			continue
		}
		if err := os.Remove(socket); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// withCleanupError reports a cleanup failure without hiding the check's own
// outcome or exit status.
func withCleanupError(result, cleanup error) error {
	if cleanup == nil {
		return result
	}
	text := "collar check cleanup failed: " + strings.ReplaceAll(cleanup.Error(), "\n", "; ")
	var ce commandError
	switch {
	case result == nil:
		return commandError{status: exitError, text: text}
	case errors.As(result, &ce):
		return commandError{status: ce.status, text: ce.text + "; " + text}
	default:
		return fmt.Errorf("%w; %s", result, text)
	}
}

func (cmd command) prepareCollarCheck(root string) (_ string, _ string, _ [2]string, err error) {
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
	ready := false
	defer func() {
		if !ready {
			err = errors.Join(err, os.RemoveAll(directory))
		}
	}()
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
	ready = true
	return id, directory, sockets, nil
}

func probeTrust(ctx context.Context, backend *tmux.Backend, collar harness.Collar, directory string) (map[string]harness.ProbeResult, error) {
	results := make(map[string]harness.ProbeResult)
	trustCollar := collar
	trustCollar.Hooks = nil
	launch, err := harness.RenderLaunch(trustCollar, harness.LaunchOptions{})
	if err != nil {
		return nil, err
	}
	pane, err := backend.CreateSession(ctx, launch.SpawnSpec("probe", directory))
	if err != nil {
		results[harness.ProbeLaunch] = unknownProbe(harness.ProbeLaunch, "tmux did not start the probe: "+err.Error())
		return results, nil
	}
	results[harness.ProbeLaunch] = passedProbe(harness.ProbeLaunch, "native process launched in a private tmux server")
	startup, _, err := harness.AwaitStartup(ctx, backend.Capture, pane.ID, collar)
	if err != nil {
		results[harness.ProbeTrustPrompt] = probeErrorResult(ctx, harness.ProbeTrustPrompt, err, backend.SessionExists)
		return results, nil
	}
	detail := "persisted native trust admitted the disposable project"
	if startup.State == harness.StartupTrustRequired {
		detail = startup.Prompt
	}
	results[harness.ProbeTrustPrompt] = passedProbe(harness.ProbeTrustPrompt, detail)
	return results, nil
}

func (cmd command) probeActive(ctx context.Context, backend *tmux.Backend, collar harness.Collar, settings settings, directory, workdir string) (map[string]harness.ProbeResult, error) {
	results := make(map[string]harness.ProbeResult)
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
		results[harness.ProbeComposer] = unknownProbe(harness.ProbeComposer, "tmux did not start the hooked probe: "+err.Error())
		return results, nil
	}
	startup, screen, err := harness.AwaitStartup(ctx, backend.Capture, pane.ID, collar)
	if err != nil {
		results[harness.ProbeComposer] = probeErrorResult(ctx, harness.ProbeComposer, err, backend.SessionExists)
		return results, nil
	}
	if startup.State == harness.StartupTrustRequired {
		results[harness.ProbeTrustPrompt] = passedProbe(harness.ProbeTrustPrompt, startup.Prompt)
		results[harness.ProbeComposer] = unknownProbe(harness.ProbeComposer, fmt.Sprintf("native trust prompt in %s awaits the operator: %s", workdir, startup.Prompt))
		return results, nil
	}
	if startup.State != harness.StartupReady {
		results[harness.ProbeComposer] = unknownProbe(harness.ProbeComposer, fmt.Sprintf("native startup is %s: %s", startup.State, startup.Prompt))
		return results, nil
	}
	composer, err := harness.ReadComposer(collar.Primitives.Composer, screen)
	if err != nil {
		results[harness.ProbeComposer] = failedProbe(harness.ProbeComposer, err.Error())
		return results, nil
	}
	if composer.Text != "" {
		results[harness.ProbeComposer] = failedProbe(harness.ProbeComposer, "native composer was not empty")
		return results, nil
	}
	results[harness.ProbeComposer] = passedProbe(harness.ProbeComposer, "native empty composer detected")
	action, _ := harness.Submit(collar.Primitives.Submit, "Reply with exactly READY.")
	settle, _ := harness.SubmitSettle(collar.Primitives.Submit)
	input, _ := harness.SubmitInput(collar.Primitives.Submit, action.Text)
	payload, err := awaitNativeHook(ctx, fifo, func() error {
		if err := sendHarnessKeys(ctx, backend, pane.ID, collar, input); err != nil {
			return err
		}
		if err := harness.AwaitComposerText(ctx, backend.Capture, pane.ID, collar, action.Text, settle); err != nil {
			return err
		}
		return sendHarnessKeys(ctx, backend, pane.ID, collar, substrate.Keys{Names: action.Keys, Submit: action.Submit})
	})
	if err != nil {
		results[harness.ProbeSubmit] = probeErrorResult(ctx, harness.ProbeSubmit, err, backend.SessionExists)
		return results, nil
	}
	results[harness.ProbeSubmit] = passedProbe(harness.ProbeSubmit, "native submit produced a hook witness")
	boundary, _, err := harness.DetectTurnBoundary(collar, payload)
	if err != nil {
		results[harness.ProbeHook] = failedProbe(harness.ProbeHook, err.Error())
		return results, nil
	}
	results[harness.ProbeHook] = passedProbe(harness.ProbeHook, "native hook payload decoded")
	if boundary == harness.TurnStarted {
		results[harness.ProbeTurnBoundary] = passedProbe(harness.ProbeTurnBoundary, "native turn-start boundary decoded")
	} else {
		results[harness.ProbeTurnBoundary] = failedProbe(harness.ProbeTurnBoundary, fmt.Sprintf("first native hook decoded as %q", boundary))
	}
	return results, nil
}

func unknownProbeResults() map[string]harness.ProbeResult {
	results := make(map[string]harness.ProbeResult)
	for _, name := range harness.RequiredProbes() {
		results[name] = unknownProbe(name, "probe did not run")
	}
	return results
}

func unknownProbe(name, detail string) harness.ProbeResult {
	return harness.ProbeResult{Name: name, Outcome: harness.ProbeUnknown, Detail: detail}
}

func failedProbe(name, detail string) harness.ProbeResult {
	return harness.ProbeResult{Name: name, Outcome: harness.ProbeFailed, Detail: detail}
}

func passedProbe(name, detail string) harness.ProbeResult {
	return harness.ProbeResult{Name: name, Outcome: harness.ProbePassed, Detail: detail}
}

// probeErrorResult classifies an error from a probe that had started. An
// error from the probe's own tmux or helper commands observed nothing about
// the harness unless the probe session is gone: the native process ends that
// session when it exits. Any other error, including a deadline the harness
// did not meet, is an observed failure.
func probeErrorResult(ctx context.Context, name string, err error, sessionExists func(context.Context) (bool, error)) harness.ProbeResult {
	var exitErr *exec.ExitError
	var execErr *exec.Error
	if ctx.Err() != nil || (!errors.As(err, &exitErr) && !errors.As(err, &execErr)) {
		return failedProbe(name, err.Error())
	}
	if exists, existsErr := sessionExists(context.Background()); existsErr == nil && !exists {
		return failedProbe(name, "native process exited: "+err.Error())
	}
	return unknownProbe(name, err.Error())
}

func (cmd command) printCheckReport(report harness.CheckReport) error {
	for _, result := range report.Results {
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", result.Outcome, result.Name, result.Detail); err != nil {
			return err
		}
	}
	if report.Passed() {
		return nil
	}
	failed, unknown := report.Probes(harness.ProbeFailed), report.Probes(harness.ProbeUnknown)
	if len(failed) == 0 {
		return commandError{status: exitUnknown, text: "collar check incomplete; unknown: " + strings.Join(unknown, ", ")}
	}
	fmt.Fprintln(cmd.stdout)
	fmt.Fprint(cmd.stdout, report.IssueBody())
	fmt.Fprintln(cmd.stdout, report.IssueCommand("adambiggs/gangline"))
	text := "collar check failed: " + strings.Join(failed, ", ")
	if len(unknown) > 0 {
		text += "; unknown: " + strings.Join(unknown, ", ")
	}
	return commandError{status: exitNative, text: text}
}
