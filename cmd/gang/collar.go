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
	trustBackend, err := tmux.New(tmux.Config{Binary: valueOr(cmd.environment("GANG_TMUX"), "tmux"), Socket: trustSocket, Session: "gang-check-trust-" + id})
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
	activeBackend, err := tmux.New(tmux.Config{Binary: valueOr(cmd.environment("GANG_TMUX"), "tmux"), Socket: activeSocket, Session: "gang-check-active-" + id})
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
					input, _ := harness.SubmitInput(collar.Primitives.Submit, action.Text)
					payload, submitErr := awaitNativeHook(ctx, fifo, func() error {
						if err := sendHarnessKeys(ctx, activeBackend, pane.ID, collar, input); err != nil {
							return err
						}
						if err := awaitComposerText(ctx, activeBackend, pane.ID, collar, action.Text, settle); err != nil {
							return err
						}
						return sendHarnessKeys(ctx, activeBackend, pane.ID, collar, substrate.Keys{Names: action.Keys, Submit: action.Submit})
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
