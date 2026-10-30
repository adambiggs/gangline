//go:build acceptance && darwin

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

// The peer transport check reads the root sshd above this user's sshd. PID 1
// is launchd, owned by root, so an unprivileged caller reading it crosses the
// same user boundary.
func TestProcessOwnerReadsRootProcessUnprivileged(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root proves nothing about an unprivileged read")
	}
	owner, err := ProcessOwner(1)
	if err != nil {
		t.Fatal(err)
	}
	if owner.PID != 1 || owner.ParentPID != 0 || owner.UID != 0 || owner.RealUID != 0 || owner.Command != "launchd" {
		t.Fatalf("owner = %+v", owner)
	}
}
