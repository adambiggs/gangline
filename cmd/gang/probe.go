package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
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

// collarCheckFIFOEnvironment names the FIFO a collar check reads its
// probe's hook payloads from.
const collarCheckFIFOEnvironment = "GANGLINE_COLLAR_CHECK_FIFO"

// collarCheckHook hands a hook payload from a collar check's probe to the
// check. It touches no team state, so a hitch ID the probe inherited from the
// caller cannot route the probe's events to the caller's agent.
func (cmd command) collarCheckHook(args []string, fifo string) error {
	if err := noArguments(args, "hook"); err != nil {
		return err
	}
	payload, err := io.ReadAll(io.LimitReader(cmd.stdin, maximumHookBytes+1))
	if err != nil {
		return err
	}
	if len(payload) > maximumHookBytes {
		return fmt.Errorf("hook payload exceeds maximum size")
	}
	info, err := os.Lstat(fifo)
	if err != nil {
		return err
	}
	if info.Mode().Type() != fs.ModeNamedPipe {
		return fmt.Errorf("%s %s is not a FIFO", collarCheckFIFOEnvironment, fifo)
	}
	file, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = file.Write(payload)
	return errors.Join(err, file.Close())
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
