package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	var frame bytes.Buffer
	if err := json.Compact(&frame, payload); err != nil {
		return fmt.Errorf("encode probe hook frame: %w", err)
	}
	frame.WriteByte('\n')
	// Payloads can exceed PIPE_BUF. Serialize complete frames so simultaneous
	// native callbacks cannot interleave their bytes.
	lock, err := os.OpenFile(fifo+".lock", os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	file, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	// Open without waiting for a vanished reader, then let the write block
	// while the live reader drains a frame larger than the pipe capacity.
	if err := syscall.SetNonblock(int(file.Fd()), false); err != nil {
		return errors.Join(err, file.Close())
	}
	_, err = file.Write(frame.Bytes())
	return errors.Join(err, file.Close())
}

// nativeHookProbe owns one persistent reader, including during native startup.
// Keeping the FIFO open across writers makes each newline a frame boundary;
// ending the check closes the reader and joins it even when submission fails.
type nativeHookProbe struct {
	keeper *os.File
	input  io.ReadCloser
	reader *exec.Cmd
	cancel context.CancelFunc
	result chan nativeHookResult
	done   chan struct{}
}

type nativeHookResult struct {
	payload []byte
	err     error
}

func startNativeHookProbe(fifo string, collar harness.Collar, prompt string) (*nativeHookProbe, error) {
	// The keeper prevents EOF between callback writers. Use cat's output
	// pipe for cancellable reads on Darwin and Linux.
	keeper, err := os.OpenFile(fifo, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	reader := exec.CommandContext(ctx, "cat", fifo)
	input, err := reader.StdoutPipe()
	if err != nil {
		cancel()
		return nil, errors.Join(err, keeper.Close())
	}
	if err := reader.Start(); err != nil {
		cancel()
		return nil, errors.Join(err, input.Close(), keeper.Close())
	}
	probe := &nativeHookProbe{keeper: keeper, input: input, reader: reader, cancel: cancel, result: make(chan nativeHookResult, 1), done: make(chan struct{})}
	go func() {
		defer close(probe.done)
		payload, err := readProbeTurn(input, collar, prompt)
		probe.result <- nativeHookResult{payload, err}
	}()
	return probe, nil
}

func (probe *nativeHookProbe) Close() error {
	probe.cancel()
	err := errors.Join(probe.input.Close(), probe.keeper.Close())
	<-probe.done
	// This reader is deliberately stopped when the probe ends. Wait reaps the
	// owned child; its cancellation exit is not a native harness failure.
	_ = probe.reader.Wait()
	return err
}

func (probe *nativeHookProbe) await(ctx context.Context, trigger func() error) ([]byte, error) {
	if err := trigger(); err != nil {
		return nil, err
	}
	select {
	case got := <-probe.result:
		return got.payload, got.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func readProbeTurn(input io.Reader, collar harness.Collar, prompt string) ([]byte, error) {
	scanner := bufio.NewScanner(input)
	// The hook budget is the existing native payload bound, plus its newline
	// and one byte to distinguish an over-budget frame.
	scanner.Buffer(make([]byte, 4096), maximumHookBytes+2)
	for scanner.Scan() {
		frame := append([]byte{}, scanner.Bytes()...)
		if len(frame) > maximumHookBytes {
			return frame, fmt.Errorf("probe hook frame exceeds maximum size")
		}
		boundary, event, err := harness.DetectTurnBoundary(collar, frame)
		if err != nil {
			return frame, err
		}
		if boundary == harness.TurnFailed {
			return frame, fmt.Errorf("native probe turn failed: %s", event.Payload["error"])
		}
		if boundary != harness.TurnStarted {
			continue
		}
		matched, err := harness.SubmittedPromptMatches(collar.Primitives.SubmitWitness, prompt, event.Payload["prompt"])
		if err != nil {
			return frame, err
		}
		if matched {
			return frame, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read probe hook stream: %w", err)
	}
	return nil, fmt.Errorf("native hook stream ended without the submitted prompt's turn-start: %w", io.EOF)
}
