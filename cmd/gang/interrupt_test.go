package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

// A hitch started with SIGINT ignored, as a non-interactive shell starts a
// background command, keeps ignoring it: only the SIGTERM interrupts.
func TestInterruptibleKeepsIgnoredSignalIgnored(t *testing.T) {
	signal.Ignore(os.Interrupt)
	t.Cleanup(func() { signal.Reset(os.Interrupt) })
	ctx, stop := interruptible(context.Background())
	defer stop()
	for _, s := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		if err := syscall.Kill(os.Getpid(), s); err != nil {
			t.Fatal(err)
		}
	}
	<-ctx.Done()
	var interrupt interruptError
	if !errors.As(context.Cause(ctx), &interrupt) || interrupt.signal != syscall.SIGTERM {
		t.Fatalf("cause = %v, want the SIGTERM interrupt", context.Cause(ctx))
	}
	stop()
	if !signal.Ignored(os.Interrupt) {
		t.Fatal("SIGINT no longer ignored after stop")
	}
}
