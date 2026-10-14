package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/harness"
)

// The hooked probe runs in the caller's repository, where native hook review
// is the only thing between a planted hook and the operator's account. It
// launches the hooks a hitch installs, so trust the operator gave them for
// hitches covers the probe, and it launches nothing that bypasses review.
func TestCollarCheckProbeInstallsTheHitchHooksWithoutBypassingReview(t *testing.T) {
	names, err := harness.EmbeddedCollarNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			collar, err := harness.EmbeddedCollar(name)
			if err != nil {
				t.Fatal(err)
			}
			f := fakeCollarCheckTmux(t, `new-session) printf '%s\n' "$@" > "$socket.spawn"; prev= prev2= generation=
	for argument; do [ "$prev2" = -soq ] && generation=$argument; prev2=$prev; prev=$argument; done
	printf '%%1\n%s\t$1\t%%1\n' "$generation";;
has-session) exit 1;;
`)
			native := filepath.Join(t.TempDir(), collar.Launch.Command)
			if err := os.WriteFile(native, []byte("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\nexit 91\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", filepath.Dir(native)+string(os.PathListSeparator)+os.Getenv("PATH"))
			f.env["GANGLINE_HITCH_ID"] = "caller-agent"
			f.input.screen = screenWithText("Trust this folder?", "› 1. Trust and continue")
			_ = f.cmd.collar([]string{"check", name})
			spawns, _ := filepath.Glob(filepath.Join(f.env["GANG_STATE_ROOT"], ".local", "state", "gl-*-a.sock.spawn"))
			if len(spawns) != 1 {
				t.Fatalf("hooked probe spawns = %v, want one\n%s", spawns, f.out.String())
			}
			data, err := os.ReadFile(spawns[0])
			if err != nil {
				t.Fatal(err)
			}
			argv := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			launched := []string(nil)
			for _, arg := range argv {
				if strings.HasPrefix(arg, "exec ") {
					// tmux runs the native command as one shell-quoted word.
					words, err := exec.Command("sh", "-c", `eval "set -- $1"; printf '%s\n' "$@"`, "sh", strings.TrimPrefix(arg, "exec ")).Output()
					if err != nil {
						t.Fatal(err)
					}
					launched = strings.Split(strings.TrimSuffix(string(words), "\n"), "\n")
				}
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			options := agentHooks(exe)
			options.Probe = true
			hitch, err := harness.RenderLaunch(collar, options)
			if err != nil {
				t.Fatal(err)
			}
			if want := append([]string{hitch.Name}, hitch.Args...); !reflect.DeepEqual(launched, want) {
				t.Errorf("probe launch does not install the hitch hooks:\n got %q\nwant %q", launched, want)
			}
			for _, arg := range launched {
				if strings.Contains(arg, "dangerously") {
					t.Errorf("probe launch passes %q", arg)
				}
			}
			fifo := ""
			for i, arg := range argv {
				if i > 0 && argv[i-1] == "-e" && strings.HasPrefix(arg, "GANGLINE_COLLAR_CHECK_FIFO=") {
					fifo = strings.TrimPrefix(arg, "GANGLINE_COLLAR_CHECK_FIFO=")
				}
			}
			if fifo == "" || !containsRun(argv, []string{"-e", "GANGLINE_HITCH_ID="}) {
				t.Errorf("probe environment does not route hooks to the check and away from the caller's hitch:\n%q", argv)
			}
		})
	}
}

// A hitched agent handles its own hooks even where the hitcher's environment
// names a collar check's FIFO.
func TestHitchedAgentHooksIgnoreACollarCheckFIFO(t *testing.T) {
	f := newStateFixture(t)
	fakeCodexOnPath(t)
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "codex"))
	spawn := filepath.Join(t.TempDir(), "spawn")
	program := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" + fakeTmuxUTF8 +
		"[ \"$1\" = -S ] && shift 2\n" +
		"case \"$1\" in has-session) exit 1;; new-session) printf '%s\\n' \"$@\" > " + spawn + "; echo fixture-reached-new-session >&2; exit 91;; *) exit 92;; esac\n"
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
	f.env[collarCheckFIFOEnvironment] = filepath.Join(t.TempDir(), "stray.fifo")
	if err := f.cmd.hitch([]string{"worker", "-c", "codex"}); err == nil || !strings.Contains(err.Error(), "fixture-reached-new-session") {
		t.Fatalf("hitch = %v", err)
	}
	data, err := os.ReadFile(spawn)
	if err != nil {
		t.Fatal(err)
	}
	if argv := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"); !containsRun(argv, []string{"-e", collarCheckFIFOEnvironment + "="}) {
		t.Errorf("hitch spawn does not clear %s:\n%q", collarCheckFIFOEnvironment, argv)
	}
}

func containsRun(haystack, run []string) bool {
	for i := 0; i+len(run) <= len(haystack); i++ {
		if reflect.DeepEqual(haystack[i:i+len(run)], run) {
			return true
		}
	}
	return false
}

// A hook launched by a collar check hands its payload to the check and
// nothing else, even where a hitch ID is inherited from the caller.
func TestHookHandsACollarCheckPayloadToTheCheck(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	before := collarCheckHookState(t, f)
	fifo := filepath.Join(t.TempDir(), "hook.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	// The check's open read end lets the hook's write proceed, and a read
	// with no writer left returns what was written, or nothing.
	reader, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	payload := []byte(`{"hook_event_name":"UserPromptSubmit","prompt":"Reply with exactly READY.","session_id":"s"}`)
	f.env["GANGLINE_HITCH_ID"] = "a"
	f.env["GANGLINE_COLLAR_CHECK_FIFO"] = fifo
	f.cmd.stdin = bytes.NewReader(payload)
	if err := f.cmd.hook(nil); err != nil {
		t.Fatal(err)
	}
	if got, err := io.ReadAll(reader); err != nil || !bytes.Equal(got, payload) {
		t.Errorf("check received %q, want %q", got, payload)
	}
	if after := collarCheckHookState(t, f); after != before {
		t.Errorf("collar check hook changed team state:\nbefore %s\nafter  %s", before, after)
	}

	regular := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(regular, []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_COLLAR_CHECK_FIFO"] = regular
	f.cmd.stdin = bytes.NewReader(payload)
	if err := f.cmd.hook(nil); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(regular); string(data) != "kept" {
		t.Errorf("collar check hook wrote a regular file: %q", data)
	}
	if !strings.Contains(f.errOut.String(), "not a FIFO") {
		t.Errorf("refusal not reported: %q", f.errOut.String())
	}
	if after := collarCheckHookState(t, f); after != before {
		t.Errorf("refused collar check hook changed team state:\nbefore %s\nafter  %s", before, after)
	}
}

func collarCheckHookState(t *testing.T, f *stateFixture) string {
	t.Helper()
	log, _ := os.ReadFile(filepath.Join(f.run.team.Directory, "log.jsonl"))
	p, err := f.run.team.Agent("a")
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	return string(log) + "\n" + string(a.Activity) + " " + a.ChangedAt.String() + " " + a.Native.SessionID
}
