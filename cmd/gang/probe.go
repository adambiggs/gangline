package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

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
	var lastErr error
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
