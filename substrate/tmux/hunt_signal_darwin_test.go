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
)

// Hunt-only probe: signal a process SIGTERM then SIGKILL through its audit
// token, as reapOwnedProcesses does, and count each error the kernel returns.
func TestHuntDarwinTokenSignalRace(t *testing.T) {
	if os.Getenv("GANG_HUNT") == "" {
		t.Skip("hunt only")
	}
	for _, orphan := range []bool{true, false} {
		counts := map[string]int{}
		for i := 0; i < 400; i++ {
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
