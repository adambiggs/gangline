package main

import (
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLaunchdWatchdogPlist(t *testing.T) {
	dir := t.TempDir()
	unit := "gangline-test"
	path := filepath.Join(dir, unit+".plist")
	var calls [][]string
	s := launchdWatchdog{directory: dir, domain: "user/501", command: func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return nil, nil
	}}
	env := map[string]string{"GANG_SESSION": "team & <friends>", "GANG_STATE_ROOT": "/state with spaces", "PATH": "/usr/bin:/bin", "GANG_TMUX": ""}
	exe := filepath.Join(dir, "gang = ' \" & $()")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" \"$GANG_SESSION\" \"${TMUX-unset}\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.Arm(unit, exe, env); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plist struct {
		XMLName xml.Name `xml:"plist"`
		Dict    struct {
			Items []struct {
				XMLName xml.Name
				Text    string   `xml:",chardata"`
				Strings []string `xml:"string"`
			} `xml:",any"`
		} `xml:"dict"`
	}
	if err := xml.Unmarshal(data, &plist); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	var args []string
	items := plist.Dict.Items
	for i := 0; i < len(items); i += 2 {
		if i+1 == len(items) || items[i].XMLName.Local != "key" {
			t.Fatalf("invalid plist dictionary: %s", data)
		}
		key, value := items[i].Text, items[i+1]
		values[key] = value.XMLName.Local + ":" + value.Text
		if key == "ProgramArguments" {
			args = value.Strings
		}
	}
	for key, want := range map[string]string{"Label": "string:" + unit, "WorkingDirectory": "string:/", "StartInterval": "integer:60", "LaunchOnlyOnce": "true:", "AbandonProcessGroup": "true:"} {
		if values[key] != want {
			t.Errorf("%s: got %q, want %q", key, values[key], want)
		}
	}
	if _, ok := values["RunAtLoad"]; ok {
		t.Fatal("watchdog must wait for its interval")
	}
	if args[0] != "/usr/bin/env" || !reflect.DeepEqual(args[len(args)-10:len(args)-6], []string{"/bin/sh", "-c", `"$@" &`, "gangline-watchdog"}) {
		t.Fatalf("launcher: %q", args)
	}
	if !reflect.DeepEqual(args[len(args)-6:], []string{exe, "tick", "--source", "watchdog", "--watchdog", unit}) {
		t.Fatalf("tick arguments: %q", args)
	}
	unset := map[string]bool{}
	gotEnv := map[string]string{}
	for i := 1; i < len(args)-10; i++ {
		if args[i] == "-u" {
			i++
			unset[args[i]] = true
		} else {
			key, value, _ := strings.Cut(args[i], "=")
			gotEnv[key] = value
		}
	}
	for _, key := range []string{"TMUX", "TMUX_PANE", "TMUX_TMPDIR", "GANGLINE_BOUNDARY", "GANGLINE_HITCH_ID", "GANG_COLLAR", "GANG_LAUNCH_ARGS", "GANG_TMUX", "GANG_COLLARS", "GANG_CONFIG_DIR", "GANG_TMUX_SOCKET", "GANG_CAPACITY_TIMEOUT"} {
		if !unset[key] {
			t.Errorf("inherited routing variable %s survives", key)
		}
	}
	delete(env, "GANG_TMUX")
	if !reflect.DeepEqual(gotEnv, env) {
		t.Fatalf("environment: got %q, want %q", gotEnv, env)
	}
	// Output waits on the inherited pipe closing, not a sleep or a deadline.
	// Exercise the real launcher with shell metacharacters and '=' in the path.
	launcher := exec.Command(args[0], args[1:]...)
	launcher.Env = append(os.Environ(), "TMUX=wrong-socket")
	out, err := launcher.CombinedOutput()
	wantOutput := "tick\n--source\nwatchdog\n--watchdog\n" + unit + "\n" + env["GANG_SESSION"] + "\nunset\n"
	if err != nil || string(out) != wantOutput {
		t.Fatalf("launcher: output=%q, error=%v", out, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("plist permissions: %v, %v", info, err)
	}
	if err := s.Disarm(unit); err != nil {
		t.Fatal(err)
	}
	wantCalls := [][]string{{"/bin/launchctl", "bootstrap", "user/501", path}, {"/bin/launchctl", "bootout", "user/501/" + unit}}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("commands: %q", calls)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("plist remains: %v", err)
	}
}

func TestLaunchdWatchdogDisarmFailures(t *testing.T) {
	failure := errors.New("launchctl failed")
	for _, tc := range []struct {
		name, output string
		probeError   error
		removed      bool
	}{
		{"retired", "Could not find service \"gangline-test\" in domain for user: 501\n", failure, true},
		{"still loaded", "user/501/gangline-test = {}", nil, false},
		{"unavailable domain", "Could not find domain for", failure, false},
		{"permission denied", "Operation not permitted", failure, false},
		{"different service", "Could not find service \"other\" in domain for user: 501", failure, false},
		{"empty error", "", failure, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "gangline-test.plist")
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
			s := launchdWatchdog{directory: dir, domain: "user/501", command: func(name string, args ...string) ([]byte, error) {
				if name != "/bin/launchctl" || len(args) != 2 || args[1] != "user/501/gangline-test" {
					t.Fatalf("unexpected command: %s %q", name, args)
				}
				if args[0] == "bootout" {
					return nil, failure
				}
				if args[0] != "print" {
					t.Fatalf("unexpected probe: %q", args)
				}
				return []byte(tc.output), tc.probeError
			}}
			err := s.Disarm("gangline-test")
			if tc.removed && err != nil || !tc.removed && !errors.Is(err, failure) {
				t.Fatalf("disarm error: %v", err)
			}
			_, statErr := os.Stat(path)
			if errors.Is(statErr, os.ErrNotExist) != tc.removed {
				t.Fatalf("plist cleanup: %v", statErr)
			}
		})
	}
}

func TestLaunchdWatchdogFailedBootstrapRemainsCleanable(t *testing.T) {
	failure := errors.New("bootstrap outcome unknown")
	s := launchdWatchdog{directory: t.TempDir(), domain: "user/501", command: func(_ string, args ...string) ([]byte, error) {
		if args[0] == "bootstrap" {
			return nil, failure
		}
		return nil, nil
	}}
	if err := s.Arm("gangline-test", "/gang", nil); !errors.Is(err, failure) {
		t.Fatalf("arm: %v", err)
	}
	path := filepath.Join(s.directory, "gangline-test.plist")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lost cleanup artifact: %v", err)
	}
	if err := s.Disarm("gangline-test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cleanup: %v", err)
	}
	if err := s.Disarm("gangline-test"); err != nil {
		t.Fatalf("already removed plist: %v", err)
	}
}

func TestLaunchdWatchdogReplacementRearmAndLastDrop(t *testing.T) {
	f, _ := watchdogFixture(t)
	loaded := ""
	failure := errors.New("service absent")
	s := launchdWatchdog{directory: f.run.team.Directory, domain: "user/501", command: func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "bootstrap":
			if loaded != "" {
				t.Fatal("replacement left old job loaded")
			}
			loaded = strings.TrimSuffix(filepath.Base(args[2]), ".plist")
			return nil, nil
		case "bootout":
			if loaded == "" {
				return nil, failure
			}
			if args[1] != "user/501/"+loaded {
				t.Fatalf("removed wrong job: %q", args)
			}
			loaded = ""
			return nil, nil
		case "print":
			return []byte("Could not find service \"" + strings.TrimPrefix(args[1], "user/501/") + "\" in domain for user: 501"), failure
		default:
			t.Fatalf("unexpected launchctl command: %q", args)
			return nil, failure
		}
	}}
	f.cmd.newScheduler = func() watchdogScheduler { return s }
	f.run.cmd = f.cmd
	for i := 0; i < 2; i++ {
		if err := f.cmd.tick(nil); err != nil {
			t.Fatal(err)
		}
	}
	generation := loaded
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", generation}); err != nil {
		t.Fatal(err)
	}
	if loaded == "" || loaded == generation {
		t.Fatal("loaded generation did not rearm")
	}
	generation = loaded
	loaded = "" // Model launchd retiring the launcher after expiry.
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", generation}); err != nil {
		t.Fatal(err)
	}
	if loaded == "" || loaded == generation {
		t.Fatal("expired generation did not rearm")
	}
	paths, err := filepath.Glob(filepath.Join(s.directory, "*.plist"))
	if err != nil || len(paths) != 1 || paths[0] != filepath.Join(s.directory, loaded+".plist") {
		t.Fatalf("replacement plists: %v, %v", paths, err)
	}
	if err := f.cmd.drop([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
	paths, err = filepath.Glob(filepath.Join(s.directory, "*.plist"))
	if err != nil || len(paths) != 0 || loaded != "" {
		t.Fatalf("last-drop cleanup: loaded=%q, plists=%v, error=%v", loaded, paths, err)
	}
}
