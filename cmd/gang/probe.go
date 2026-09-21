package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adambiggs/gangline/harness"
)

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
