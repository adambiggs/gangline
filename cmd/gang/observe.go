package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

func (cmd command) context(arguments []string) error {
	name, state, run, err := cmd.observationTarget(arguments, "context")
	if err != nil {
		return err
	}
	hitch, _ := activeByName(state, name)
	backend, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	screen, err := backend.Capture(context.Background(), substrate.PaneID(hitch.Pane))
	if err != nil {
		return err
	}
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return err
	}
	reading, err := harness.ReadContext(collar.Primitives.Context, screen)
	if err != nil {
		return commandError{status: exitUnknown, text: err.Error()}
	}
	model := ""
	if collar.Models.Selected != nil {
		model, _ = harness.ReadSelectedModel(*collar.Models.Selected, screen)
	}
	band := harness.ActiveContextBand(collar, model, reading)
	bandName := "none"
	if band != nil {
		bandName = band.Name
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s\t%d/%d\t%.0f%%\t%s\n", name, reading.Used, reading.Limit, reading.Percent*100, bandName)
	return err
}

func (cmd command) limits(arguments []string) error {
	if len(arguments) == 1 && arguments[0] == "--history" {
		return commandError{status: exitUnknown, text: "no retained provider-limit history"}
	}
	name, state, run, err := cmd.observationTarget(arguments, "limits")
	if err != nil {
		return err
	}
	hitch, _ := activeByName(state, name)
	backend, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	screen, err := backend.Capture(context.Background(), substrate.PaneID(hitch.Pane))
	if err != nil {
		return err
	}
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return err
	}
	readings, err := harness.ReadProviderLimits(collar.Primitives.ProviderLimits, screen, time.Now())
	if err != nil {
		return commandError{status: exitUnknown, text: err.Error()}
	}
	for _, reading := range readings {
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%d%%\t%s\n", reading.Label, reading.UsedPercent, reading.ResetAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return nil
}

func (cmd command) observationTarget(arguments []string, commandName string) (string, core.State, *runtime, error) {
	if len(arguments) > 1 {
		return "", core.State{}, nil, usageError("%s: expected at most one agent", commandName)
	}
	run, err := cmd.runtime()
	if err != nil {
		return "", core.State{}, nil, err
	}
	state, err := run.load()
	if err != nil {
		return "", core.State{}, nil, err
	}
	name := ""
	if len(arguments) == 1 {
		name = arguments[0]
	}
	if name == "" {
		pane := cmd.environment("TMUX_PANE")
		for _, hitch := range state.Hitches {
			if hitch.Pane == pane {
				name = string(hitch.Name)
				break
			}
		}
	}
	if _, ok := activeByName(state, name); !ok {
		return "", core.State{}, nil, refuseError("agent %q is not active", name)
	}
	return name, state, run, nil
}

type wedgeObservationFile struct {
	Screen substrate.Screen `json:"screen"`
}

func (cmd command) observeWedges(run *runtime, state core.State) error {
	backend, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		return err
	}
	entries, err := cmd.teamLog(run)
	if err != nil {
		return err
	}
	for _, hitch := range state.Hitches {
		if hitch.Status != core.HitchActive || hitch.Activity != core.ActivityBusy {
			continue
		}
		screen, err := backend.Capture(context.Background(), substrate.PaneID(hitch.Pane))
		if err != nil {
			continue
		}
		file := filepath.Join(paths.Directory, "observe-"+string(hitch.ID)+".json")
		var prior wedgeObservationFile
		data, readErr := os.ReadFile(file)
		hasPrior := readErr == nil && json.Unmarshal(data, &prior) == nil
		encoded, _ := json.Marshal(wedgeObservationFile{Screen: screen})
		if err := os.WriteFile(file, append(encoded, '\n'), 0o600); err != nil {
			return err
		}
		if !hasPrior {
			continue
		}
		collar, err := loadCollar(hitch.Collar, run.settings)
		if err != nil {
			return err
		}
		wedge, err := harness.DetectWedge(collar.Primitives.Wedge, harness.WedgeObservation{
			Previous: prior.Screen, Current: screen, BusySince: busySince(entries, hitch.ID), ObservedAt: time.Now(), TurnActive: true,
		})
		if err != nil {
			return err
		}
		if wedge.Detected {
			state, err = run.drive(core.WedgeDetected{At: time.Now(), HitchID: hitch.ID, Evidence: wedge.Evidence})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (cmd command) teamLog(run *runtime) ([]store.LogEntry, error) {
	locked, err := run.paths().Lock(run.settings.Session)
	if err != nil {
		return nil, err
	}
	defer locked.Close()
	return locked.Log()
}

func busySince(entries []store.LogEntry, id core.HitchID) time.Time {
	var since time.Time
	for _, entry := range entries {
		if event, ok := entry.Event.(core.TurnStarted); ok && event.HitchID == id {
			since = event.At
		}
		if event, ok := entry.Event.(core.TurnBoundaryReached); ok && event.HitchID == id {
			since = time.Time{}
		}
	}
	return since
}

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
	report := harness.CheckReport{Collar: collar.Name, HarnessVersion: installedHarnessVersion(collar)}
	results := make(map[string]harness.ProbeResult)
	for _, name := range harness.RequiredProbes() {
		results[name] = harness.ProbeResult{Name: name, Detail: "unknown: probe did not run"}
	}
	root := settings.StateRoot
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	id, err := randomID("check")
	if err != nil {
		return err
	}
	checkDir := filepath.Join(root, "native-project-"+id)
	if err := os.Mkdir(checkDir, 0o775); err != nil {
		return err
	}
	defer os.RemoveAll(checkDir)
	if output, err := exec.Command("git", "-C", checkDir, "init", "--quiet").CombinedOutput(); err != nil {
		return fmt.Errorf("initialize disposable collar-check project: %w: %s", err, strings.TrimSpace(string(output)))
	}
	home, err := cmd.userHomeDir()
	if err != nil {
		return err
	}
	socketRoot := filepath.Join(home, ".local", "state")
	if err := os.MkdirAll(socketRoot, 0o700); err != nil {
		return err
	}
	shortID := id
	if len(shortID) > 18 {
		shortID = shortID[:18]
	}
	trustSocket := filepath.Join(socketRoot, "gl-"+shortID+"-t.sock")
	activeSocket := filepath.Join(socketRoot, "gl-"+shortID+"-a.sock")
	defer os.Remove(trustSocket)
	defer os.Remove(activeSocket)

	// A fresh directory exercises the native trust surface. Gangline observes it
	// and never answers it.
	trustBackend, err := tmux.New(tmux.Config{Binary: valueOr(cmd.environment("GANGLINE_TMUX"), "tmux"), Socket: trustSocket, Session: "gang-check-trust-" + id})
	if err != nil {
		return err
	}
	trustCollar := collar
	trustCollar.Hooks = nil
	baseLaunch, err := harness.RenderLaunch(trustCollar, harness.LaunchOptions{})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	trustPane, launchErr := trustBackend.CreateSession(ctx, baseLaunch.SpawnSpec("probe", checkDir))
	if launchErr == nil {
		results[harness.ProbeLaunch] = harness.ProbeResult{Name: harness.ProbeLaunch, Passed: true, Detail: "native process launched in a private tmux server"}
		startup, _, inspectErr := awaitStartup(ctx, trustBackend, trustPane.ID, collar)
		if inspectErr != nil {
			results[harness.ProbeTrustPrompt] = harness.ProbeResult{Name: harness.ProbeTrustPrompt, Detail: inspectErr.Error()}
		} else if startup.State == harness.StartupTrustRequired {
			results[harness.ProbeTrustPrompt] = harness.ProbeResult{Name: harness.ProbeTrustPrompt, Passed: true, Detail: startup.Prompt}
		} else {
			results[harness.ProbeTrustPrompt] = harness.ProbeResult{Name: harness.ProbeTrustPrompt, Passed: true, Detail: "persisted native trust admitted the disposable project"}
		}
	} else {
		results[harness.ProbeLaunch] = harness.ProbeResult{Name: harness.ProbeLaunch, Detail: launchErr.Error()}
	}
	_ = trustBackend.KillSession(context.Background())

	// Use the already-trusted repository for the active hook/composer probes.
	fifo := filepath.Join(checkDir, "hook.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		return err
	}
	hook := []string{"sh", "-c", "cat > \"$1\"", "gang-collar-check", fifo}
	launch, err := harness.RenderLaunch(collar, harness.LaunchOptions{HookCommand: hook, Probe: true})
	if err != nil {
		return err
	}
	launch = applyLaunchPolicy(launch, collar.Name, settings)
	activeBackend, err := tmux.New(tmux.Config{Binary: valueOr(cmd.environment("GANGLINE_TMUX"), "tmux"), Socket: activeSocket, Session: "gang-check-active-" + id})
	if err != nil {
		return err
	}
	defer activeBackend.KillSession(context.Background())
	workdir, err := activeProbeDirectory(cmd.getwd)
	if err != nil {
		return err
	}
	pane, err := activeBackend.CreateSession(ctx, launch.SpawnSpec("probe", workdir))
	if err == nil {
		if startup, screen, inspectErr := awaitStartup(ctx, activeBackend, pane.ID, collar); inspectErr != nil {
			results[harness.ProbeComposer] = harness.ProbeResult{Name: harness.ProbeComposer, Detail: inspectErr.Error()}
		} else {
			if startup.State == harness.StartupTrustRequired {
				results[harness.ProbeTrustPrompt] = harness.ProbeResult{Name: harness.ProbeTrustPrompt, Passed: true, Detail: startup.Prompt}
			}
			if startup.State == harness.StartupReady {
				composer, composerErr := harness.ReadComposer(collar.Primitives.Composer, screen)
				if composerErr != nil {
					results[harness.ProbeComposer] = harness.ProbeResult{Name: harness.ProbeComposer, Detail: composerErr.Error()}
				} else if composer.Text != "" {
					results[harness.ProbeComposer] = harness.ProbeResult{Name: harness.ProbeComposer, Detail: "native composer was not empty"}
				} else {
					results[harness.ProbeComposer] = harness.ProbeResult{Name: harness.ProbeComposer, Passed: true, Detail: "native empty composer detected"}
					action, _ := harness.Submit(collar.Primitives.Submit, "Reply with exactly READY.")
					settle, _ := harness.SubmitSettle(collar.Primitives.Submit)
					payload, submitErr := awaitNativeHook(ctx, fifo, func() error {
						if err := activeBackend.SendKeys(ctx, pane.ID, substrate.Keys{Text: action.Text}); err != nil {
							return err
						}
						if err := awaitComposerText(ctx, activeBackend, pane.ID, collar, action.Text, settle); err != nil {
							return err
						}
						return activeBackend.SendKeys(ctx, pane.ID, substrate.Keys{Names: action.Keys, Submit: action.Submit})
					})
					if submitErr != nil {
						results[harness.ProbeSubmit] = harness.ProbeResult{Name: harness.ProbeSubmit, Detail: submitErr.Error()}
					} else {
						results[harness.ProbeSubmit] = harness.ProbeResult{Name: harness.ProbeSubmit, Passed: true, Detail: "native submit produced a hook witness"}
						boundary, _, decodeErr := harness.DetectTurnBoundary(collar, payload)
						if decodeErr != nil {
							results[harness.ProbeHook] = harness.ProbeResult{Name: harness.ProbeHook, Detail: decodeErr.Error()}
						} else {
							results[harness.ProbeHook] = harness.ProbeResult{Name: harness.ProbeHook, Passed: true, Detail: "native hook payload decoded"}
							if boundary == harness.TurnStarted {
								results[harness.ProbeTurnBoundary] = harness.ProbeResult{Name: harness.ProbeTurnBoundary, Passed: true, Detail: "native turn-start boundary decoded"}
							} else {
								results[harness.ProbeTurnBoundary] = harness.ProbeResult{Name: harness.ProbeTurnBoundary, Detail: fmt.Sprintf("first native hook decoded as %q", boundary)}
							}
						}
					}
				}
			}
		}
	}
	for _, name := range harness.RequiredProbes() {
		report.Results = append(report.Results, results[name])
	}
	for _, result := range report.Results {
		mark := "FAIL"
		if result.Passed {
			mark = "PASS"
		}
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", mark, result.Name, result.Detail); err != nil {
			return err
		}
	}
	if !report.Passed() {
		fmt.Fprintln(cmd.stdout)
		fmt.Fprint(cmd.stdout, report.IssueBody())
		fmt.Fprintln(cmd.stdout, report.IssueCommand("adambiggs/gangline"))
		return commandError{status: exitNative, text: "collar check incomplete; native prompts are never auto-answered"}
	}
	return nil
}

func awaitComposerText(ctx context.Context, backend interface {
	Capture(context.Context, substrate.PaneID) (substrate.Screen, error)
}, pane substrate.PaneID, collar harness.Collar, want string, settle time.Duration) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var stableSince time.Time
	for {
		screen, err := backend.Capture(ctx, pane)
		if err == nil {
			composer, readErr := harness.ReadComposer(collar.Primitives.Composer, screen)
			if readErr == nil && composer.Text == want {
				if stableSince.IsZero() {
					stableSince = time.Now()
				}
				if time.Since(stableSince) >= settle {
					return nil
				}
			} else {
				stableSince = time.Time{}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("native composer did not show submitted text: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func activeProbeDirectory(getwd func() (string, error)) (string, error) {
	workdir, err := getwd()
	if err != nil {
		return "", err
	}
	command := exec.Command("git", "-C", workdir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	output, err := command.Output()
	if err != nil {
		return workdir, nil
	}
	common := strings.TrimSpace(string(output))
	if filepath.Base(common) != ".git" {
		return workdir, nil
	}
	return filepath.Dir(common), nil
}

func awaitStartup(ctx context.Context, backend interface {
	Capture(context.Context, substrate.PaneID) (substrate.Screen, error)
}, pane substrate.PaneID, collar harness.Collar) (harness.Startup, substrate.Screen, error) {
	const readySettle = 400 * time.Millisecond
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	lastErr := errors.New("no screen captured")
	var readySince time.Time
	for {
		screen, err := backend.Capture(ctx, pane)
		if err == nil {
			startup, inspectErr := harness.InspectStartup(collar, screen)
			if inspectErr == nil && startup.State == harness.StartupTrustRequired {
				return startup, screen, nil
			}
			if inspectErr == nil && startup.State == harness.StartupReady {
				if readySince.IsZero() {
					readySince = time.Now()
				}
				if time.Since(readySince) >= readySettle {
					return startup, screen, nil
				}
			} else {
				readySince = time.Time{}
			}
			if inspectErr != nil {
				lastErr = inspectErr
			} else {
				lastErr = errors.New(startup.Prompt)
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return harness.Startup{}, substrate.Screen{}, ctx.Err()
		case <-deadline.C:
			return harness.Startup{}, substrate.Screen{}, fmt.Errorf("native startup was not observable within 5s: %w", lastErr)
		case <-ticker.C:
		}
	}
}

func installedHarnessVersion(collar harness.Collar) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, collar.Launch.Command, "--version").Output()
	if err != nil {
		return "unknown"
	}
	line := strings.TrimSpace(string(output))
	if before, _, ok := strings.Cut(line, "\n"); ok {
		line = before
	}
	if line == "" {
		return "unknown"
	}
	return line
}

func awaitNativeHook(ctx context.Context, fifo string, trigger func() error) ([]byte, error) {
	reader := exec.CommandContext(ctx, "cat", fifo)
	result := make(chan struct {
		data []byte
		err  error
	}, 1)
	go func() {
		data, err := reader.Output()
		result <- struct {
			data []byte
			err  error
		}{data, err}
	}()
	if err := trigger(); err != nil {
		return nil, err
	}
	select {
	case got := <-result:
		return got.data, got.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
