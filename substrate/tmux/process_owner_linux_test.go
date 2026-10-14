//go:build linux

package tmux

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestProcessOwnerReadsThisProcess(t *testing.T) {
	comm, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := ProcessOwner(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	want := Owner{PID: os.Getpid(), ParentPID: os.Getppid(), UID: uint32(os.Geteuid()), RealUID: uint32(os.Getuid()), Command: string(bytes.TrimSuffix(comm, []byte("\n")))}
	if owner != want {
		t.Fatalf("owner = %+v, want %+v", owner, want)
	}
}

// procStatTail is the fields after the command name up to the start time.
const procStatTail = " S 100 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 123401"

func TestProcOwnerKeepsTheRawCommandName(t *testing.T) {
	name := "sshd) (x \xff\xfe y"
	owner, err := parseProcOwner(200, []byte("200 ("+name+")"+procStatTail), []byte("Name:\tsshd\nUmask:\t0022\nUid:\t1000\t1001\t1002\t1003\nGid:\t5\t5\t5\t5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if owner != (Owner{PID: 200, ParentPID: 100, UID: 1001, RealUID: 1000, Command: name}) {
		t.Fatalf("owner = %+v", owner)
	}
}

func TestProcOwnerRefusesMalformedRecords(t *testing.T) {
	stat := []byte("200 (sshd)" + procStatTail)
	uid := "Uid:\t0\t0\t0\t0\n"
	for name, tc := range map[string]struct {
		pid          int
		stat, status string
		want         string
	}{
		"other PID":        {201, string(stat), uid, "names PID 200"},
		"no Uid line":      {200, string(stat), "Name:\tsshd\nGid:\t0\t0\t0\t0\n", "no Uid line"},
		"two Uid lines":    {200, string(stat), uid + "Uid:\t1000\t1000\t1000\t1000\n", "more than one Uid line"},
		"short Uid line":   {200, string(stat), "Uid:\t0\t0\t0\n", "3 fields"},
		"bad real ID":      {200, string(stat), "Uid:\tx\t0\t0\t0\n", "real user ID"},
		"bad effective ID": {200, string(stat), "Uid:\t0\t-1\t0\t0\n", "effective user ID"},
		"no command":       {200, "200 sshd" + procStatTail, uid, "malformed"},
	} {
		if _, err := parseProcOwner(tc.pid, []byte(tc.stat), []byte(tc.status)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want %q", name, err, tc.want)
		}
	}
}
