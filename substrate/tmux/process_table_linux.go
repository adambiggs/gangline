//go:build linux

package tmux

import (
	"context"
	"os"
	"strconv"
)

func readProcessTable(ctx context.Context, root int, complete bool) (map[int]processRecord, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	pids := make([]int, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return collectProcessRecords(ctx, root, pids, complete, readCurrentProcess)
}
