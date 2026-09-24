package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"syscall"
	"time"

	"github.com/adambiggs/gangline/core"
)

const watchdogTimeout = time.Minute

var watchdogEnvironmentKeys = []string{"GANG_SESSION", "GANG_STATE_ROOT", "GANG_CONFIG_DIR", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_TMUX", "GANG_CAPACITY_TIMEOUT", "PATH"}
var watchdogUnsetEnvironment = []string{"TMUX", "TMUX_PANE", "TMUX_TMPDIR", "GANGLINE_BOUNDARY", "GANGLINE_HITCH_ID", "GANG_COLLAR", "GANG_LAUNCH_ARGS"}

type watchdogScheduler interface {
	Arm(string, string, map[string]string) error
	Disarm(string) error
}

type systemdWatchdog struct{}

func (systemdWatchdog) Arm(unit, executable string, environment map[string]string) error {
	args := []string{"--user", "--collect", "--unit=" + unit, "--on-active=" + watchdogTimeout.String(), "--timer-property=AccuracySec=1s", "--timer-property=RemainAfterElapse=no", "--property=KillMode=process", "--working-directory=/"}
	unset := append([]string(nil), watchdogUnsetEnvironment...)
	for _, key := range watchdogEnvironmentKeys {
		if value := environment[key]; value != "" {
			args = append(args, "--setenv="+key+"="+value)
		} else {
			unset = append(unset, key)
		}
	}
	args = append(args, "--property=UnsetEnvironment="+strings.Join(unset, " "))
	args = append(args, executable, "tick", "--source", "watchdog", "--watchdog", unit)
	_, err := watchdogCommand("systemd-run", args...)
	return err
}
func (systemdWatchdog) Disarm(unit string) error {
	_, stopErr := watchdogCommand("systemctl", "--user", "stop", unit+".timer")
	if stopErr == nil {
		return nil
	}
	// An elapsed timer can unload at any point. Only a successful absent-state
	// observation can turn a failed stop into success.
	out, err := watchdogCommand("systemctl", "--user", "show", unit+".timer", "--property=LoadState", "--value")
	if err == nil && strings.TrimSpace(string(out)) == "not-found" {
		return nil
	}
	return errors.Join(stopErr, err)
}
func watchdogCommand(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("watchdog %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
func (cmd command) watchdogScheduler(directory string) watchdogScheduler {
	if cmd.newScheduler != nil {
		return cmd.newScheduler()
	}
	if goruntime.GOOS == "darwin" {
		return launchdWatchdog{directory: directory, domain: fmt.Sprintf("user/%d", os.Getuid()), command: watchdogCommand}
	}
	if goruntime.GOOS != "linux" {
		return nil
	}
	for _, binary := range []string{"systemd-run", "systemctl"} {
		if _, err := exec.LookPath(binary); err != nil {
			return nil
		}
	}
	// A session bus alone does not imply a running systemd user manager.
	if _, err := os.Stat(filepath.Join("/run/user", fmt.Sprint(os.Getuid()), "systemd", "private")); err != nil {
		return nil
	}
	return systemdWatchdog{}
}

// updateWatchdog holds only the scheduler transaction, never agent work. A tick
// that loses this nonblocking lock leaves replacement to the current owner.
func (run *runtime) updateWatchdog(generation string, cleanup, reset bool) (proceed bool, result error) {
	defer func() {
		if result != nil {
			result = errors.Join(result, run.team.Append(core.Event{Type: "watchdog_failed", At: run.cmd.now(), Reason: result.Error()}))
		}
	}()
	path := filepath.Join(run.team.Directory, "watchdog")
	// Scoped ticks leave an existing team deadline alone, without contending
	// with its expiry. Only a full sweep is allowed to postpone idle peers.
	if !cleanup && !reset {
		if _, err := os.Stat(path); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	lock, err := os.OpenFile(filepath.Join(run.team.Directory, "watchdog.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) && !cleanup {
			return generation == "", nil
		}
		return false, fmt.Errorf("watchdog scheduler busy: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	held, err := lock.Stat()
	if err != nil {
		return false, err
	}
	current, err := os.Stat(filepath.Join(run.team.Directory, "watchdog.lock"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !os.SameFile(held, current) {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	old := string(data)
	if generation != "" && generation != old {
		return false, nil
	}
	agents, err := run.team.ListAgents()
	if err != nil {
		return false, err
	}
	scheduler := run.cmd.watchdogScheduler(run.team.Directory)
	if scheduler == nil {
		if len(agents) == 0 || cleanup {
			return false, nil
		}
		marker := filepath.Join(run.team.Directory, "watchdog-unavailable")
		if _, err := os.Stat(marker); errors.Is(err, os.ErrNotExist) {
			if err := run.team.Append(core.Event{Type: "watchdog_unavailable", At: run.cmd.now(), Reason: "no user watchdog scheduler available; ticks continue without a timer"}); err != nil {
				return false, err
			}
			if err := os.WriteFile(marker, nil, 0600); err != nil {
				return false, err
			}
		} else if err != nil {
			return false, err
		}
		return true, nil
	}
	if cleanup && len(agents) != 0 {
		return true, nil
	}
	if old != "" && !reset && !cleanup && len(agents) != 0 {
		return true, nil
	}
	if old != "" {
		if err := scheduler.Disarm(old); err != nil {
			return false, err
		}
		if err := os.Remove(path); err != nil {
			return false, err
		}
	}
	if len(agents) == 0 || cleanup {
		return false, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return false, err
	}
	root, err := filepath.Abs(run.settings.StateRoot)
	if err != nil {
		return false, err
	}
	token, err := randomID("watchdog")
	if err != nil {
		return false, err
	}
	unit := fmt.Sprintf("gangline-%x-%s", sha256.Sum256([]byte(filepath.Join(root, "teams", run.settings.Session))), token)
	// Intent survives a failed/uncertain launch so drop can still clean it up.
	if err := os.WriteFile(path, []byte(unit), 0600); err != nil {
		return false, err
	}
	socket := run.settings.Socket
	if socket == "" {
		socket, _, _ = strings.Cut(run.cmd.environment("TMUX"), ",")
	}
	if socket == "" {
		socket = filepath.Join(valueOr(run.cmd.environment("TMUX_TMPDIR"), "/tmp"), fmt.Sprintf("tmux-%d", os.Getuid()), "default")
	}
	socket, err = filepath.Abs(socket)
	if err != nil {
		return false, err
	}
	environment := map[string]string{"GANG_SESSION": run.settings.Session, "GANG_STATE_ROOT": root, "GANG_CONFIG_DIR": run.settings.ConfigDir, "GANG_TMUX_SOCKET": socket, "GANG_COLLARS": run.settings.CollarDir, "GANG_CAPACITY_TIMEOUT": run.settings.CapacityTimeout.String(), "GANG_TMUX": run.cmd.environment("GANG_TMUX"), "PATH": os.Getenv("PATH")}
	if err := scheduler.Arm(unit, executable, environment); err != nil {
		return false, err
	}
	return true, nil
}

func (run *runtime) disarmEmptyWatchdog() error {
	agents, err := run.team.ListAgents()
	if err != nil || len(agents) != 0 {
		return err
	}
	_, err = run.updateWatchdog("", true, false)
	return err
}
