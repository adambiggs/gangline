//go:build darwin

package tmux

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// proc_bsdshortinfo through PROC_PIDT_SHORTBSDINFO, which XNU answers for a
// zombie too. Status 5 is SZOMB; flag 4 is PROC_FLAG_INEXIT.
type huntShortInfo struct {
	PID, PPID, PGID, Status uint32
	Comm                    [16]byte
	Flags                   uint32
	IDs                     [6]uint32
	Rfu                     uint32
}

func huntState(pid int) string {
	var info huntShortInfo
	count, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, procInfoPIDInfo, uintptr(pid), 13, 0, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if errno != 0 {
		return "short:" + errno.Error()
	}
	return "short:n=" + strconv.Itoa(int(count)) + ",stat=" + strconv.Itoa(int(info.Status)) + ",inexit=" + strconv.FormatBool(info.Flags&4 != 0)
}

func huntErr(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}

// Hunt-only probe: signal a process SIGTERM then SIGKILL through its audit
// token, as reapOwnedProcesses does, and count each error the kernel returns.
func TestHuntDarwinTokenSignalRace(t *testing.T) {
	if os.Getenv("GANG_HUNT") == "" {
		t.Skip("hunt only")
	}
	for _, orphan := range []bool{true, false} {
		counts := map[string]int{}
		for i := 0; i < 1500; i++ {
			var pid int
			var cmd *exec.Cmd
			if orphan {
				out, err := exec.Command("sh", "-c", "trap '' HUP; sleep 600 >/dev/null 2>&1 </dev/null & echo $!").Output()
				if err != nil {
					t.Fatal(err)
				}
				pid, _ = strconv.Atoi(strings.TrimSpace(string(out)))
			} else {
				cmd = exec.Command("sleep", "600")
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				pid = cmd.Process.Pid
			}
			record, _, err := readDarwinProcess(pid)
			if err != nil {
				t.Fatalf("read %d: %v", pid, err)
			}
			handle, err := openProcessHandle(record)
			if err != nil {
				t.Fatalf("open %d: %v", pid, err)
			}
			for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL} {
				err := handle.signal(sig)
				key := sig.String() + ":ok"
				if err != nil {
					var errno syscall.Errno
					if errors.As(err, &errno) {
						key = sig.String() + ":" + errno.Error()
					} else {
						key = sig.String() + ":" + err.Error()
					}
				}
				counts[key]++
				if err != nil && strings.Contains(key, "not permitted") {
					state := huntState(pid)
					current, _, readErr := readDarwinProcess(pid)
					identity := huntErr(readErr)
					if readErr == nil {
						identity = "same=" + strconv.FormatBool(current.started == handle.(*darwinProcessHandle).started)
					}
					retry := huntErr(handle.signal(sig))
					later := huntState(pid)
					t.Logf("EPERM orphan=%v i=%d sig=%v state=%s read=%s retry=%s state-after-retry=%s", orphan, i, sig, state, identity, retry, later)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := handle.wait(ctx); err != nil {
				t.Errorf("wait %d: %v", pid, err)
			}
			cancel()
			_ = handle.close()
			_ = syscall.Kill(pid, syscall.SIGKILL)
			if cmd != nil {
				_ = cmd.Wait()
			}
		}
		t.Logf("orphan=%v counts=%v", orphan, counts)
		for key, n := range counts {
			if strings.Contains(key, "not permitted") {
				t.Errorf("orphan=%v: %s %d times", orphan, key, n)
			}
		}
	}
}
