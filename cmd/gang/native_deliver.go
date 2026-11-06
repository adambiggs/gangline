package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"

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
	if !ok {
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
	case errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist):
		return nativeQueueRefused, "native queue did not start: " + detail
	case errors.As(err, &exit) && exit.ExitCode() > 0 && ctx.Err() == nil:
		return nativeQueueRefused, "native queue refused the message: " + detail
	default:
		return nativeQueueUncertain, "native queue did not finish and may hold the message: " + detail
	}
}

func runNativeQueue(ctx context.Context, argv, env []string) ([]byte, error) {
	child := exec.CommandContext(ctx, argv[0], argv[1:]...)
	child.Env = env
	return child.CombinedOutput()
}
