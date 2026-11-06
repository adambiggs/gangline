package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

type nativeQueueResult int

const (
	// nativeQueueUnused means the message is typed as usual.
	nativeQueueUnused nativeQueueResult = iota
	// nativeQueueHeld means the harness's own queue holds the message.
	nativeQueueHeld
	// nativeQueueRefused means the queue did not take the message, so typing
	// it cannot deliver it twice.
	nativeQueueRefused
	// nativeQueueUncertain means the queue may hold the message, so typing
	// it could deliver it twice.
	nativeQueueUncertain
)

// queueNative hands wire to the collar's deliver primitive. Only the exit
// status decides: a native queue may print warnings on success.
func (run *runtime) queueNative(ctx context.Context, a core.Agent, c harness.Collar, wire string) (nativeQueueResult, string) {
	call, ok, err := harness.NativeDeliveryFor(c, a.Native.SessionID, a.Native.Transcript, wire, os.Environ())
	if err != nil {
		return nativeQueueRefused, "native queue unavailable: " + err.Error()
	}
	// An expired operation leaves the message to the typing path, which
	// reports the deadline itself.
	if !ok || ctx.Err() != nil {
		return nativeQueueUnused, ""
	}
	queue := run.cmd.nativeQueue
	if queue == nil {
		queue = runNativeQueue
	}
	output, err := queue(ctx, call.Argv, call.Env)
	if err == nil {
		return nativeQueueHeld, ""
	}
	detail := err.Error()
	if text := lastLine(output); text != "" {
		detail += ": " + text
	}
	var exit *exec.ExitError
	switch {
	case errors.Is(err, errNativeQueueNotStarted):
		return nativeQueueRefused, detail
	case errors.As(err, &exit) && exit.ExitCode() > 0 && ctx.Err() == nil:
		return nativeQueueRefused, "native queue refused the message: " + detail
	default:
		return nativeQueueUncertain, "native queue did not finish and may hold the message: " + detail
	}
}

// errNativeQueueNotStarted marks a queue command that never ran, such as a
// missing command or one whose message exceeds the platform's argument limit.
// Its text begins the receipt reason.
var errNativeQueueNotStarted = errors.New("native queue did not start")

// nativeQueueOutputBytes bounds the output kept for the receipt's last line.
const nativeQueueOutputBytes = 4096

func runNativeQueue(ctx context.Context, argv, env []string) ([]byte, error) {
	output := &tailBuffer{limit: nativeQueueOutputBytes}
	child := exec.CommandContext(ctx, argv[0], argv[1:]...)
	child.Env = env
	child.Stdout, child.Stderr = output, output
	// Bounds how long a descendant left holding the output can keep Wait,
	// and so the agent lock, after the command exits or is killed.
	child.WaitDelay = time.Second
	if err := child.Start(); err != nil {
		return nil, fmt.Errorf("%w: %w", errNativeQueueNotStarted, err)
	}
	err := child.Wait()
	// A zero exit queued the message, even when Wait also reports a
	// descendant's open output (ErrWaitDelay) or a deadline that fired after
	// the exit.
	if child.ProcessState != nil && child.ProcessState.Success() {
		err = nil
	}
	return output.data, err
}
