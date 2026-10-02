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
	"github.com/adambiggs/gangline/substrate"
)

const watchdogTimeout = time.Minute

var watchdogEnvironmentKeys = []string{"GANG_SESSION", "GANG_STATE_ROOT", "GANG_CONFIG_DIR", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_TMUX", "GANG_CAPACITY_TIMEOUT", "PATH"}
var watchdogUnsetEnvironment = []string{"TMUX", "TMUX_PANE", "TMUX_TMPDIR", "GANGLINE_BOUNDARY", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN", "GANG_COLLAR", "GANG_LAUNCH_ARGS"}

type watchdogScheduler interface {
	Arm(string, string, map[string]string) error
	Disarm(string) error
}

type systemdWatchdog struct{}

func (systemdWatchdog) Available() error {
	_, err := watchdogCommand("systemctl", "--user", "show-environment")
	return err
}

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

// updateWatchdog holds only the scheduler transaction, never agent work. A
// caller that arms never waits for the scheduler lock: one that loses it leaves
// replacement to the current owner, unless it is the recorded timer's own
// elapsed tick, which marks an outage. Cleanup waits for the lock, so a tick
// still running when the last agent leaves cannot fail the drop or down that
// disarms the timer.
func (run *runtime) updateWatchdog(generation string, cleanup, reset bool) (proceed bool, result error) {
	path := filepath.Join(run.team.Directory, "watchdog")
	marker := filepath.Join(run.team.Directory, "watchdog-unavailable")
	// replacing is set once this call starts replacing the recorded timer.
	replacing := false
	defer func() {
		if result != nil {
			// Only an elapsed timer's own tick re-arms it, and a replacement that
			// fails leaves an intent no timer backs, so either failure leaves the
			// team without a timer until a caller that can arm replaces it.
			if generation != "" || replacing {
				result = errors.Join(result, run.noteWatchdogOutage(marker, "watchdog scheduler failed: "+result.Error()))
			}
			result = errors.Join(result, run.team.Append(core.Event{Type: "watchdog_failed", At: run.cmd.now(), Reason: result.Error()}))
		}
	}()
	// A timer that elapsed during an outage leaves its intent behind, so an
	// outage marker means the recorded timer may no longer exist.
	outage, err := watchdogOutage(marker)
	if err != nil {
		return false, err
	}
	// Scoped ticks leave an existing team deadline alone, without contending
	// with its expiry. Only a full sweep is allowed to postpone idle peers.
	if !cleanup && !reset && !outage {
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
			if generation == "" {
				return true, nil
			}
			// The holder may be a tick that cannot re-arm this elapsed timer.
			intent, err := os.ReadFile(path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return false, err
			}
			if string(intent) != generation {
				return false, nil
			}
			return false, run.noteWatchdogOutage(marker, "watchdog timer elapsed while another tick held the scheduler lock")
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return false, fmt.Errorf("watchdog scheduler busy: %w", err)
		}
		// Cleanup must disarm. Every cleanup holds the team lock, and no
		// holder of the scheduler lock waits for a team or agent lock, so the
		// wait ends when the holder's scheduler transaction does.
		if run.cmd.schedulerLockWait != nil {
			run.cmd.schedulerLockWait()
		}
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX)
		for errors.Is(err, syscall.EINTR) {
			err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX)
		}
		if err != nil {
			return false, fmt.Errorf("watchdog scheduler busy: %w", err)
		}
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
	if outage, err = watchdogOutage(marker); err != nil {
		return false, err
	}
	agents, err := run.team.ListAgents()
	if err != nil {
		return false, err
	}
	scheduler := run.cmd.watchdogScheduler(run.team.Directory)
	unavailableReason := "no user watchdog scheduler available; ticks continue without a timer"
	if scheduler != nil {
		b, err := run.registry()
		if err != nil {
			return false, err
		}
		backend, err := run.cmd.tmux(run.settings)
		if err != nil {
			return false, err
		}
		windows, err := backend.Windows(context.Background())
		if err != nil {
			exists, checkErr := backend.SessionExists(context.Background())
			if checkErr != nil {
				return false, checkErr
			}
			if exists {
				return false, err
			}
		}
		present := map[string]bool{}
		for _, w := range windows {
			present[string(w.Pane.ID)] = true
		}
		for _, a := range agents {
			if a.Pane == "" || !present[a.Pane] {
				continue
			}
			visible, err := b.ProcessVisibility(context.Background(), substrate.PaneID(a.Pane))
			// A pane held from boot after its native exit has no process to
			// read; another pane answers for the host.
			if exited := (*substrate.ExitedError)(nil); errors.As(err, &exited) {
				continue
			}
			if err != nil {
				return false, err
			}
			if !visible {
				scheduler = nil
				unavailableReason = "host process visibility unavailable; process watchdog skipped; ticks continue without a timer"
				if err := run.noteProcessUnavailable(a); err != nil {
					return false, err
				}
				// This caller cannot replace a recorded timer, which still fires and
				// rearms from its own tick. That tick is this one when the generation
				// matches, so its timer has already elapsed.
				if old != "" && generation == "" && !cleanup {
					return true, nil
				}
			}
			break
		}
	}
	if probe, ok := scheduler.(interface{ Available() error }); ok {
		if err := probe.Available(); err != nil {
			scheduler = nil
			unavailableReason = "user watchdog scheduler unavailable; ticks continue without a timer: " + err.Error()
		}
	}
	if scheduler == nil {
		if len(agents) == 0 && old == "" {
			return false, nil
		}
		if err := run.noteWatchdogOutage(marker, unavailableReason); err != nil {
			return false, err
		}
		if old != "" && cleanup {
			if _, err := fmt.Fprintln(run.cmd.stderr, "warning: watchdog unavailable; existing timer cancellation deferred"); err != nil {
				return false, err
			}
		}
		return len(agents) != 0 && !cleanup, nil
	}
	if cleanup && len(agents) != 0 {
		return true, nil
	}
	if old != "" && !reset && !cleanup && !outage && len(agents) != 0 {
		return true, nil
	}
	replacing = true
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
	if err := os.Remove(marker); err == nil {
		if err := run.team.Append(core.Event{Type: "watchdog_available", At: run.cmd.now()}); err != nil {
			return false, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, nil
}

// noteWatchdogOutage logs an outage once and leaves the marker that makes the
// next caller able to arm replace the recorded timer.
func (run *runtime) noteWatchdogOutage(marker, reason string) error {
	file, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return run.team.Append(core.Event{Type: "watchdog_unavailable", At: run.cmd.now(), Reason: reason})
}

func watchdogOutage(marker string) (bool, error) {
	_, err := os.Stat(marker)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// ensureWatchdog arms a timer when the team records none, for a command that queues work only a
// later tick can deliver, and says so when no timer can be armed.
func (run *runtime) ensureWatchdog() error {
	_, err := run.updateWatchdog("", false, false)
	if outage, statErr := watchdogOutage(filepath.Join(run.team.Directory, "watchdog-unavailable")); statErr != nil || !outage {
		return errors.Join(err, statErr)
	}
	_, warnErr := fmt.Fprintln(run.cmd.stderr, "warning: watchdog unavailable; wakes and queued work wait for the next hook or gang tick (see gang log)")
	return errors.Join(err, warnErr)
}

func (run *runtime) disarmEmptyWatchdog() error {
	agents, err := run.team.ListAgents()
	if err != nil || len(agents) != 0 {
		return err
	}
	_, err = run.updateWatchdog("", true, false)
	return err
}
