//go:build linux

package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

func foregroundCommand(process substrate.Process) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(process.PID) + "/cmdline")
	if err == nil {
		if command, _, ok := strings.Cut(string(data), "\x00"); ok && command != "" {
			return command
		}
	}
	return process.Command
}

func readProcessTable(ctx context.Context, _ int) (map[int]processRecord, error) {
	command := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=,pgid=,tpgid=,lstart=,comm=")
	command.Env = append(os.Environ(), "LC_ALL=C")
	data, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("read process tree: %w", err)
	}
	return parseProcessTable(string(data))
}
