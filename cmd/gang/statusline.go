package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func (cmd command) statusline(args []string) (result error) {
	if len(args) == 1 && args[0] == "--install" {
		home, err := cmd.userHomeDir()
		if err != nil {
			return err
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		changed, err := harness.InstallStatusline(filepath.Join(home, ".claude", "settings.json"), exe)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.stdout, "status-line settings updated: %t\n", changed)
		return err
	}
	if err := noArguments(args, "statusline"); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(cmd.stdin, maximumHookBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maximumHookBytes {
		return fmt.Errorf("status-line payload exceeds maximum size")
	}
	session, readings, err := harness.ReadStatusline(data)
	if err != nil {
		return err
	}
	if readings[0].Status == "observed" {
		at, err := harness.StatuslineMeasurementTime(data, readings[0])
		if err != nil {
			return err
		}
		readings[0].At = at
	}
	r := coreReading(readings[0])
	if id := core.HitchID(cmd.environment("GANGLINE_HITCH_ID")); id != "" {
		run, err := cmd.runtime()
		if err != nil {
			return err
		}
		p, err := run.team.Agent(id)
		if err != nil {
			return err
		}
		a, err := p.Read()
		if err != nil {
			return err
		}
		if a.Native.SessionID != "" && a.Native.SessionID != session {
			return fmt.Errorf("status-line session does not match registered native session")
		}
		converted := make([]core.Reading, len(readings))
		for i, value := range readings {
			converted[i] = coreReading(value)
		}
		if err := run.record(a, core.Event{Type: "observation", Readings: converted}); err != nil {
			return err
		}
		l, current, err := run.acquire(id, false)
		if err != nil && !errors.Is(err, store.ErrLocked) {
			return err
		}
		if l != nil {
			defer func() { result = errors.Join(result, run.release(l)) }()
			a = current
			a.Native.SessionID = session
			acceptReadings(&a.Native, converted)
			if err := l.Save(a); err != nil {
				return err
			}
			if err := run.publishContext(a); err != nil {
				return err
			}
			if err := run.release(l); err != nil {
				return err
			}
		} else {
			acceptReadings(&a.Native, converted)
		}
		r = a.Native.Context
	}
	if r.Status != "observed" || r.Used == nil || r.Limit == nil || r.Percent == nil {
		_, err = fmt.Fprintln(cmd.stdout, "context unknown: "+r.Reason)
	} else {
		_, err = fmt.Fprintf(cmd.stdout, "context %d/%d (%.0f%%)\n", *r.Used, *r.Limit, *r.Percent)
	}
	return err
}
