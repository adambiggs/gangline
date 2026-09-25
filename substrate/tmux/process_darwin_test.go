//go:build darwin

package tmux

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"github.com/adambiggs/gangline/substrate"
)

func TestDarwinArgvHelper(t *testing.T) {
	if os.Getenv("GANGLINE_ARGV_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "ready")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestDarwinForegroundCommandKeepsSymlinkName(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "gangline-native-harness-entry")
	if err := os.Symlink(binary, link); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(link, "-test.run=^TestDarwinArgvHelper$")
	child.Env = append(os.Environ(), "GANGLINE_ARGV_HELPER=1")
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(ready) != "ready" {
		t.Fatalf("helper readiness = %q, %v", ready, err)
	}
	got, err := readDarwinArgv0(child.Process.Pid)
	if err != nil || got != link {
		t.Fatalf("native argv[0] = %q, %v; want symlink %q", got, err, link)
	}
	if command := foregroundCommand(substrate.Process{PID: child.Process.Pid, Command: "truncated"}); command != link {
		t.Fatalf("foreground command = %q; want symlink %q", command, link)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestDarwinNativeProcessTable(t *testing.T) {
	var info darwinBSDInfo
	if unsafe.Sizeof(info) != 136 || unsafe.Offsetof(info.GroupID) != 100 || unsafe.Offsetof(info.ForegroundGroup) != 112 || unsafe.Offsetof(info.StartSeconds) != 120 {
		t.Fatalf("proc_bsdinfo ABI layout changed: size=%d pgid=%d tpgid=%d start=%d", unsafe.Sizeof(info), unsafe.Offsetof(info.GroupID), unsafe.Offsetof(info.ForegroundGroup), unsafe.Offsetof(info.StartSeconds))
	}
	record, err := readDarwinBSDProcess(os.Getpid())
	if err != nil || record.PID != os.Getpid() || record.ParentPID != os.Getppid() || record.GroupID <= 0 || record.started == "" {
		t.Fatalf("native BSD record = %+v, %v", record, err)
	}
	records, err := readProcessTable(context.Background(), os.Getpid())
	if err != nil || records[os.Getpid()].PID != os.Getpid() {
		t.Fatalf("native process table = %+v, %v", records[os.Getpid()], err)
	}
}

func TestDarwinNativeProcessIdentity(t *testing.T) {
	var info darwinProcessInfo
	if unsafe.Sizeof(info) != 56 || unsafe.Offsetof(info.UniqueID) != 16 || unsafe.Offsetof(info.Version) != 32 {
		t.Fatalf("proc_info ABI layout changed: size=%d uniqueid=%d version=%d", unsafe.Sizeof(info), unsafe.Offsetof(info.UniqueID), unsafe.Offsetof(info.Version))
	}
	record, _, err := readDarwinProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if record.PID != os.Getpid() || record.parentUniqueID == 0 || record.started == "" {
		t.Fatalf("native identity = %+v", record)
	}
	observation, err := observeProcess(record.PID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = observation.close() })
	identity, err := pinObservedProcess(observation.record, observation.read, openProcessHandle)
	if err != nil {
		t.Fatal(err)
	}
	if err := closeOwnedProcesses([]processIdentity{identity}); err != nil {
		t.Fatal(err)
	}
}
