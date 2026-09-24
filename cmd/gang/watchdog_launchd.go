package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type launchdWatchdog struct {
	directory string
	domain    string
	command   func(string, ...string) ([]byte, error)
}

func (s launchdWatchdog) Arm(unit, executable string, environment map[string]string) error {
	// The short-lived shell leaves tick running when launchd retires the job.
	// Otherwise bootout during self-rearm would terminate the tick itself.
	args := []string{"/usr/bin/env"}
	unset := append([]string(nil), watchdogUnsetEnvironment...)
	for _, key := range watchdogEnvironmentKeys {
		if environment[key] == "" {
			unset = append(unset, key)
		}
	}
	for _, key := range unset {
		args = append(args, "-u", key)
	}
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if environment[key] != "" {
			args = append(args, key+"="+environment[key])
		}
	}
	// env must see a fixed command before the executable path: a path with '='
	// would otherwise be parsed as another environment assignment.
	args = append(args, "/bin/sh", "-c", `"$@" &`, "gangline-watchdog", executable, "tick", "--source", "watchdog", "--watchdog", unit)
	var plist strings.Builder
	plist.WriteString(xml.Header + `<plist version="1.0"><dict><key>Label</key>`)
	value, err := xml.Marshal(unit)
	if err != nil {
		return err
	}
	plist.Write(value)
	plist.WriteString(`<key>ProgramArguments</key><array>`)
	for _, arg := range args {
		value, err := xml.Marshal(arg)
		if err != nil {
			return err
		}
		plist.Write(value)
	}
	fmt.Fprintf(&plist, `</array><key>WorkingDirectory</key><string>/</string><key>StartInterval</key><integer>%d</integer><key>LaunchOnlyOnce</key><true/><key>AbandonProcessGroup</key><true/></dict></plist>`, int(watchdogTimeout/time.Second))
	// Outside ~/Library/LaunchAgents: a reboot must not resurrect a stale team.
	path := filepath.Join(s.directory, unit+".plist")
	if err := os.WriteFile(path, []byte(plist.String()), 0600); err != nil {
		return err
	}
	// Keep the plist on uncertain bootstrap failure. The generation intent lets
	// Disarm retry removal before deleting the file or the team directory.
	_, err = s.command("/bin/launchctl", "bootstrap", s.domain, path)
	return err
}

func (s launchdWatchdog) Disarm(unit string) error {
	target := s.domain + "/" + unit
	if _, err := s.command("/bin/launchctl", "bootout", target); err != nil {
		// LaunchOnlyOnce may already have retired the job. Other failures,
		// including a missing user domain, do not prove service absence.
		out, probeErr := s.command("/bin/launchctl", "print", target)
		if probeErr == nil || !strings.HasPrefix(strings.TrimSpace(string(out)), "Could not find service \""+unit+"\" in domain ") {
			return errors.Join(err, probeErr)
		}
	}
	err := os.Remove(filepath.Join(s.directory, unit+".plist"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
