//go:build darwin

package tmux

import (
	"os"
	"testing"
)

func TestProcessOwnerReadsThisProcess(t *testing.T) {
	owner, err := ProcessOwner(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if owner.PID != os.Getpid() || owner.ParentPID != os.Getppid() || owner.UID != uint32(os.Geteuid()) || owner.RealUID != uint32(os.Getuid()) || owner.Command == "" {
		t.Fatalf("owner = %+v", owner)
	}
}
