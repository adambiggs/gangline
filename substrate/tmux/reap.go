package tmux

import (
	"context"
	"sync"
	"time"
)

// reapKickInterval paces the background run-shell commands a reap starts
// while it waits. Too short, and a healthy reap that is merely slow starts
// short-lived jobs it did not need. Too long, and a missed SIGCHLD holds the
// reap for up to the interval.
const reapKickInterval = 100 * time.Millisecond

// Reaped runs a tmux command list once the server has reaped every child that
// had exited. run runs one tmux client invocation with the given arguments.
//
// run-shell returns only once the server has reaped its own child, and the
// server reaps every exited child whenever it handles a SIGCHLD. A server can
// lose that signal: tmux built with libutempter calls it when a pane closes,
// and libutempter sets SIGCHLD to its default action while its helper runs,
// which discards a SIGCHLD that arrives meanwhile. A child that exits then
// stays unreaped until another child exits, and nothing else may exit. So
// while the command waits, background run-shell commands keep giving the
// server another child to exit.
func Reaped(ctx context.Context, run func(context.Context, ...string) (string, error), after ...string) (string, error) {
	arguments := []string{"run-shell", "true"}
	if len(after) > 0 {
		arguments = append(append(arguments, ";"), after...)
	}
	kicks, stop := context.WithCancel(ctx)
	var kicking sync.WaitGroup
	kicking.Add(1)
	go func() {
		defer kicking.Done()
		ticker := time.NewTicker(reapKickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-kicks.Done():
				return
			case <-ticker.C:
				// A kick only gives the server a child to exit. The
				// waiting command reports what the server answers.
				_, _ = run(kicks, "run-shell", "-b", "true")
			}
		}
	}()
	output, err := run(ctx, arguments...)
	stop()
	kicking.Wait()
	return output, err
}

func (backend *Backend) reaped(ctx context.Context, after ...string) (string, error) {
	return Reaped(ctx, backend.run, after...)
}
