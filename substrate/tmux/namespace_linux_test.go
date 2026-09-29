//go:build linux

package tmux

import (
	"os"
	"testing"
)

func TestPeerProcessVisibility(t *testing.T) {
	for _, test := range []struct {
		name                 string
		peer                 int
		same, readable, want bool
	}{
		{"same namespace", 42, true, true, true},
		{"ancestor invisible", 0, true, true, false},
		{"translated pid", 7, true, true, false},
		{"pid collision", 42, false, true, false},
		{"unreadable process", 42, true, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			reads := 0
			got, err := peerProcessVisible(42, test.peer, func(int) bool { return test.same }, func(int) (processRecord, error) {
				reads++
				if !test.readable {
					return processRecord{}, os.ErrPermission
				}
				return processRecord{}, nil
			})
			if got != test.want || err != nil {
				t.Fatalf("visibility = %v, %v; want %v", got, err, test.want)
			}
			if (test.peer != 42 || !test.same) && reads != 0 {
				t.Fatal("observed process before validating its namespace")
			}
		})
	}
	if !samePIDNamespace(os.Getpid()) {
		t.Fatal("current process namespace differs from itself")
	}
}
