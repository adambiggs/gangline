package tmux

import (
	"errors"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestParsePaneProcessPreservesEmptyFields(t *testing.T) {
	for _, tc := range []struct {
		name, record string
		pid          int
		dead         bool
		status       string
		invalid      bool
	}{
		{name: "live", record: "123 0 \n", pid: 123},
		{name: "exited with PID", record: "123 1 3\n", dead: true, status: "3"},
		{name: "exited without PID", record: " 1 3\n", dead: true, status: "3"},
		{name: "exited without PID or status", record: " 1 \n", dead: true},
		{name: "live without PID", record: " 0 \n", invalid: true},
		{name: "live with omitted status", record: "123 0\n", pid: 123},
		{name: "missing fields", record: "123\n", invalid: true},
		{name: "extra fields", record: "123 0  4\n", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pid, err := parsePaneProcess(tc.record)
			var nativeExit *substrate.ExitedError
			if tc.dead {
				if !errors.As(err, &nativeExit) || nativeExit.Status != tc.status {
					t.Fatalf("exit=%v, want status %q", err, tc.status)
				}
			} else if (err != nil) != tc.invalid {
				t.Fatalf("error=%v, want invalid=%v", err, tc.invalid)
			}
			if pid != tc.pid {
				t.Fatalf("pid=%d, want %d", pid, tc.pid)
			}
		})
	}
}
