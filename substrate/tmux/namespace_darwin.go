//go:build darwin

package tmux

import "context"

func nativeProcessNamespace() (string, error) { return "darwin", nil }

func readCallerProcess(pid int) (processRecord, error) {
	return callerProcessWithParent(pid, readCurrentProcess, readDarwinBSDProcess)
}

func serverProcessVisible(_ context.Context, _ string, pid int) (bool, error) {
	_, err := readCurrentProcess(pid)
	return processReadVisibility(err)
}
