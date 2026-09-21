package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		switch event := entry.Event.(type) {
		case core.TurnStarted:
			if event.HitchID == id {
				since = event.At
			}
		case core.TurnBoundaryReached:
			if event.HitchID == id {
				since = time.Time{}
			}
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
	report := harness.CheckReport{Collar: collar.Name, HarnessVersion: "unknown"}
	results := make(map[string]harness.ProbeResult)
	for _, name := range harness.RequiredProbes() {
		results[name] = harness.ProbeResult{Name: name, Detail: "unknown: probe did not run"}
	}
	root := filepath.Join(settings.StateRoot, "collar-check")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	id, err := randomID("check")
	if err != nil {
		return err
	}
	checkDir := filepath.Join(root, id)
	if err := os.Mkdir(checkDir, 0o700); err != nil {
		return err
	}
	defer os.RemoveAll(checkDir)
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	_ = executable

	// A fresh directory exercises the native trust surface. Gangline observes it
	// and never answers it.
	trustBackend, err := tmux.New(tmux.Config{Binary: valueOr(cmd.environment("GANGLINE_TMUX"), "tmux"), Socket: filepath.Join(checkDir, "trust.sock"), Session: "gang-check-trust-" + id})
	if err != nil {
		return err
	}
	baseLaunch, err := harness.RenderLaunch(collar, harness.LaunchOptions{HookCommand: []string{"true"}})
	if err != nil {
		return err
	}
	baseLaunch = applyLaunchPolicy(baseLaunch, collar.Name, settings)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	trustPane, launchErr := trustBackend.CreateSession(ctx, baseLaunch.SpawnSpec("probe", checkDir))
	if launchErr == nil {
		results[harness.ProbeLaunch] = harness.ProbeResult{Name: harness.ProbeLaunch, Passed: true, Detail: "native process launched in a private tmux server"}
		if screen, captureErr := trustBackend.Capture(ctx, trustPane.ID); captureErr == nil {
			if startup, inspectErr := harness.InspectStartup(collar, screen); inspectErr == nil && startup.State == harness.StartupTrustRequired {
				results[harness.ProbeTrustPrompt] = harness.ProbeResult{Name: harness.ProbeTrustPrompt, Passed: true, Detail: startup.Prompt}
			}
		}
	}
	_ = trustBackend.KillSession(context.Background())

	// Use the already-trusted repository for the active hook/composer probes.
	fifo := filepath.Join(checkDir, "hook.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		return err
	}
	hook := []string{"sh", "-c", "cat > \"$1\"", "gang-collar-check", fifo}
	launch, err := harness.RenderLaunch(collar, harness.LaunchOptions{HookCommand: hook})
	if err != nil {
		return err
	}
	launch = applyLaunchPolicy(launch, collar.Name, settings)
	activeBackend, err := tmux.New(tmux.Config{Binary: valueOr(cmd.environment("GANGLINE_TMUX"), "tmux"), Socket: filepath.Join(checkDir, "active.sock"), Session: "gang-check-active-" + id})
	if err != nil {
		return err
	}
	defer activeBackend.KillSession(context.Background())
	workdir, err := cmd.getwd()
	if err != nil {
		return err
	}
	pane, err := activeBackend.CreateSession(ctx, launch.SpawnSpec("probe", workdir))
	if err == nil {
		if screen, captureErr := activeBackend.Capture(ctx, pane.ID); captureErr == nil {
			startup, inspectErr := harness.InspectStartup(collar, screen)
			if inspectErr == nil && startup.State == harness.StartupReady {
				composer, composerErr := harness.ReadComposer(collar.Primitives.Composer, screen)
				if composerErr == nil && composer.Text == "" {
					results[harness.ProbeComposer] = harness.ProbeResult{Name: harness.ProbeComposer, Passed: true, Detail: "native empty composer detected"}
					action, _ := harness.Submit(collar.Primitives.Submit, "Reply with exactly READY.")
					payload, submitErr := awaitNativeHook(ctx, fifo, func() error { return activeBackend.SendKeys(ctx, pane.ID, action.Input()) })
					if submitErr == nil {
						results[harness.ProbeSubmit] = harness.ProbeResult{Name: harness.ProbeSubmit, Passed: true, Detail: "native submit produced a hook witness"}
						boundary, _, decodeErr := harness.DetectTurnBoundary(collar, payload)
						if decodeErr == nil {
							results[harness.ProbeHook] = harness.ProbeResult{Name: harness.ProbeHook, Passed: true, Detail: "native hook payload decoded"}
							if boundary == harness.TurnStarted {
								results[harness.ProbeTurnBoundary] = harness.ProbeResult{Name: harness.ProbeTurnBoundary, Passed: true, Detail: "native turn-start boundary decoded"}
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
		return commandError{status: exitNative, text: "collar check incomplete; native prompts are never auto-answered"}
	}
	return nil
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
