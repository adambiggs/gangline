//go:build acceptance

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func stoppedRehitchFixture(t *testing.T) (*stateFixture, core.Agent) {
	t.Helper()
	f, a := stoppedClaimFixture(t, "worker")
	program, err := os.ReadFile(f.env["GANG_TMUX"])
	if err != nil {
		t.Fatal(err)
	}
	program = bytes.Replace(program, []byte("has-session) exit 1;;"), []byte("has-session) exit 1;; list-panes) echo 'no server running on fixture' >&2; exit 1;;"), 1)
	if err := os.WriteFile(f.env["GANG_TMUX"], program, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", t.TempDir())
	a.Directory = f.env["GANG_STATE_ROOT"]
	a.Process.Namespace = "old-namespace"
	a.ConfiguredModel, a.ConfiguredEffort = "saved-model", ""
	a.Role, a.HitchedBy = "lead", "parent-hitch"
	a.Native.Offset, a.Native.HookSequence = 123, 4
	a.Native.BoundSession = a.Native.SessionID
	a.NativeTitle = &core.NativeTitle{Text: "saved title", Attempted: true}
	a.ContextBandThresholds = &core.ContextBandThresholds{Early: 50, Late: 75}
	a.ContextBands = core.ContextBandState{Sequence: 3, Model: "saved-model", Percent: 60}
	a.Capacity = core.Capacity{Fingerprint: "limit", Attempts: 2, Submitted: true}
	a.Compaction = &core.Compaction{ID: "settled-compaction", Status: "completed", ResumeAdmitted: true, Continuation: true}
	a.Clear = &core.ClearRequest{Status: "completed", PreviousSession: "earlier-session"}
	a.LastAccepted, a.LastDelivered, a.LastFailed = "accepted", "delivered", "failed"
	a.BackgroundNoticed, a.CurfewNoticeDeadline = f.cmd.now(), f.cmd.now().Add(time.Hour)
	saveStoppedClaim(t, f, a)
	return f, a
}

func nextStoppedAgent(a core.Agent) core.Agent {
	return core.Agent{ID: a.ID, Name: a.Name, Collar: a.Collar, Role: a.Role, HitchedBy: a.HitchedBy,
		Directory: a.Directory, CreatedAt: a.CreatedAt, Status: core.Starting, Activity: core.Unknown,
		ContextBandThresholds: a.ContextBandThresholds, LastAccepted: a.LastAccepted,
		LastDelivered: a.LastDelivered, LastFailed: a.LastFailed,
		ConfiguredModel: a.ConfiguredModel, ConfiguredEffort: a.ConfiguredEffort}
}

func TestStoppedRehitchPreservesDurableStateAndMail(t *testing.T) {
	f, a := stoppedRehitchFixture(t)
	p, _ := f.run.team.Agent(a.ID)
	e := core.Envelope{ID: "queued-before-move", Recipient: a.ID, To: a.Name, Message: core.Message{Text: "continue work"}}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	mailPath, _ := p.EnvelopePath("new", e.ID)
	before, _ := os.ReadFile(mailPath)
	for _, path := range []string{p.Witness, p.CompactionWitness, p.Background, p.Permission} {
		if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := f.cmd.tmux(f.run.settings)
	next := nextStoppedAgent(a)
	l, err := f.run.prepareStoppedRehitch(context.Background(), b, a, &next)
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	want := nextStoppedAgent(a)
	want.Registration, want.Process = a.Registration, a.Process
	want.Registration.Held = false
	want.Native = a.Native
	want.Native.HookAt = f.cmd.now()
	want.NativeTitle, want.ContextBands = a.NativeTitle, a.ContextBands
	want.Compaction, want.Clear, want.Capacity = a.Compaction, a.Clear, a.Capacity
	want.BackgroundNoticed, want.CurfewNoticeDeadline = a.BackgroundNoticed, a.CurfewNoticeDeadline
	if !reflect.DeepEqual(next, want) {
		t.Fatalf("prepared state differs:\ngot  %+v\nwant %+v", next, want)
	}
	got, err := p.Read()
	// The old witness retains launch metadata until the new pane is registered.
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("saved state differs: %+v, %v", got, err)
	}
	after, err := os.ReadFile(mailPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("queued mail changed: %s, %v", after, err)
	}
	id, err := f.run.team.ResolveName(string(a.Name))
	if err != nil || id != a.ID {
		t.Fatalf("name now points to %s: %v", id, err)
	}
	for _, path := range []string{p.Witness, p.CompactionWitness, p.Background, p.Permission} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("old process witness remains: %s: %v", path, err)
		}
	}
}

// The first recovered agent already owns a new generation's %1. The old peer
// also recorded %1; recovery must create a new window without touching that pane.
func TestStoppedRehitchAfterLeadRecreatedTeam(t *testing.T) {
	f, a := stoppedRehitchFixture(t)
	lead := f.add(t, "retained-lead", "lead", "codex")
	lead.Registration.Generation = strings.Repeat("b", 64)
	saveStoppedClaim(t, f, lead)
	program := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" + fakeTmuxUTF8 +
		"if [ \"$1\" = -S ]; then shift 2; fi\n" +
		"case \"$1 $2\" in\n" +
		"'has-session -t') exit 0;;\n" +
		"'list-panes -a') printf '" + lead.Registration.Generation + "\\t$1\\t%%1\\tunit\\n';;\n" +
		"list-panes*) printf '%%1\\t" + lead.Registration.Generation + "\\t$1\\tlead\\n';;\n" +
		"new-window*) printf '%s\\n' \"$@\" > '" + filepath.Join(a.Directory, "launch-args") + "'; echo fixture-reached-new-window >&2; exit 91;;\n" +
		"*) echo unexpected-tmux-command >&2; exit 92;; esac\n"
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(program), 0700); err != nil {
		t.Fatal(err)
	}
	p, _ := f.run.team.Agent(a.ID)
	mail := core.Envelope{ID: "old-mail", Recipient: a.ID, To: a.Name, Message: core.Message{Text: "queued"}}
	if err := p.Publish(mail); err != nil {
		t.Fatal(err)
	}
	leadPath, _ := f.run.team.Agent(lead.ID)
	leadBefore, _ := os.ReadFile(leadPath.State)
	err := f.cmd.rehitch([]string{"worker", "--stopped", "--note", "continue from saved work"})
	if err == nil || !strings.Contains(err.Error(), "fixture-reached-new-window") {
		t.Fatalf("recovery did not reach new window: %v", err)
	}
	got, err := p.Read()
	if err != nil || got.ID != a.ID || got.Native.SessionID != a.Native.SessionID || got.Native.Offset != a.Native.Offset || got.Status != core.Failed || got.Pane != "" {
		t.Fatalf("failed launch lost retained record: %+v: %v", got, err)
	}
	if got, err := p.ReadEnvelope("new", mail.ID); err != nil || !reflect.DeepEqual(got, mail) {
		t.Fatalf("mail changed: %+v: %v", got, err)
	}
	leadAfter, _ := os.ReadFile(leadPath.State)
	if !bytes.Equal(leadBefore, leadAfter) {
		t.Fatal("recovered lead changed")
	}
	launch, err := os.ReadFile(filepath.Join(a.Directory, "launch-args"))
	if err != nil || !strings.Contains(string(launch), "retained-session") || !strings.Contains(string(launch), "saved-model") {
		t.Fatalf("saved native identity/model absent from launch: %s: %v", launch, err)
	}
	pending, err := p.ListNew()
	if err != nil || len(pending) != 1 {
		t.Fatalf("old mail = %+v: %v", pending, err)
	}

	// Failed pre-spawn launch is retryable without dropping the retained ID.
	err = f.cmd.rehitch([]string{"worker", "--stopped", "--note", "retry saved work"})
	if err == nil || !strings.Contains(err.Error(), "fixture-reached-new-window") {
		t.Fatalf("retry failed before launch: %v", err)
	}
}

func TestStoppedRehitchRefusesUnresolvedStateWithoutMutation(t *testing.T) {
	cases := map[string]func(*core.Agent){
		"unknown session":   func(a *core.Agent) { a.Native.SessionID = "" },
		"dropping":          func(a *core.Agent) { a.Status = core.Dropping },
		"teardown":          func(a *core.Agent) { a.Teardown = []core.ProcessIdentity{a.Process} },
		"rename":            func(a *core.Agent) { a.RenameTo = "renaming" },
		"input":             func(a *core.Agent) { a.Input = &core.InputIntent{ID: "unverified", Kind: "envelope"} },
		"cleanup":           func(a *core.Agent) { a.Cleanup = &core.ResultRef{ID: "pending"} },
		"clear":             func(a *core.Agent) { a.Clear.Status = "submitted" },
		"compaction":        func(a *core.Agent) { a.Compaction.Status, a.Compaction.ResumeAdmitted = "submitted", false },
		"rehitch":           func(a *core.Agent) { a.Rehitch = &core.Message{Text: "queued continuation"} },
		"partial process":   func(a *core.Agent) { a.Process.PID = 0 },
		"missing namespace": func(a *core.Agent) { a.Process.Namespace = "" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f, a := stoppedRehitchFixture(t)
			change(&a)
			saveStoppedClaim(t, f, a)
			p, _ := f.run.team.Agent(a.ID)
			before, _ := os.ReadFile(p.State)
			err := f.cmd.rehitch([]string{"worker", "--stopped", "--note", "resume"})
			if err == nil || errorStatus(err) != exitRefused {
				t.Fatalf("refusal = %v", err)
			}
			after, _ := os.ReadFile(p.State)
			if !bytes.Equal(before, after) {
				t.Fatal("refusal changed record")
			}
		})
	}
}

func TestStoppedRehitchRefusesRegisteredPaneAndLiveProcess(t *testing.T) {
	t.Run("registered pane", func(t *testing.T) {
		f, a := stoppedRehitchFixture(t)
		program := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" + fakeTmuxUTF8 +
			"if [ \"$1\" = -S ]; then shift 2; fi\n" +
			"case \"$1\" in list-panes) printf '" + a.Registration.Generation + "\\t$1\\t%%1\\tunit\\n';; *) exit 92;; esac\n"
		if err := os.WriteFile(f.env["GANG_TMUX"], []byte(program), 0700); err != nil {
			t.Fatal(err)
		}
		b, _ := f.cmd.tmux(f.run.settings)
		next := nextStoppedAgent(a)
		l, err := f.run.prepareStoppedRehitch(context.Background(), b, a, &next)
		if l != nil {
			_ = l.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "still has its registered pane") {
			t.Fatalf("live pane refusal = %v", err)
		}
	})
	t.Run("live process", func(t *testing.T) {
		if goruntime.GOOS != "linux" {
			t.Skip("Linux process witness fixture")
		}
		f, a := stoppedRehitchFixture(t)
		stat, err := os.ReadFile("/proc/" + strconv.Itoa(os.Getpid()) + "/stat")
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(string(stat)[strings.LastIndexByte(string(stat), ')')+2:])
		boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
		if err != nil {
			t.Fatal(err)
		}
		namespace, err := os.Readlink("/proc/self/ns/pid")
		if err != nil {
			t.Fatal(err)
		}
		a.Process = core.ProcessIdentity{PID: os.Getpid(), BootID: strings.TrimSpace(string(boot)), Namespace: namespace, Started: fields[19]}
		saveStoppedClaim(t, f, a)
		b, _ := f.cmd.tmux(f.run.settings)
		next := nextStoppedAgent(a)
		l, err := f.run.prepareStoppedRehitch(context.Background(), b, a, &next)
		if l != nil {
			_ = l.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "still has its recorded process") {
			t.Fatalf("live process refusal = %v", err)
		}
	})
}

func TestStoppedRehitchRequiresNameNoteAndNoUpgrade(t *testing.T) {
	for _, args := range [][]string{{"--stopped", "--note", "resume"}, {"worker", "--stopped"}, {"worker", "--stopped", "--note", " "}, {"worker", "--stopped", "--note", "resume", "--upgrade"}} {
		f := newStateFixture(t)
		if err := f.cmd.rehitch(args); errorStatus(err) != exitUsage {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestStoppedRehitchMetadataFallbackAndNeverSpawned(t *testing.T) {
	for _, variant := range []string{"missing", "corrupt", "wrong binding", "never spawned"} {
		t.Run(variant, func(t *testing.T) {
			f, a := stoppedRehitchFixture(t)
			p, _ := f.run.team.Agent(a.ID)
			metadata := filepath.Join(p.Directory, "model-metadata.json")
			switch variant {
			case "missing":
				if err := os.Remove(metadata); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(metadata, []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			case "wrong binding":
				data, err := os.ReadFile(metadata)
				if err != nil {
					t.Fatal(err)
				}
				data = bytes.ReplaceAll(data, []byte(string(a.ID)), []byte("other-hitch"))
				if err := os.WriteFile(metadata, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "never spawned":
				a.Pane, a.Registration, a.Process = "", core.PaneRegistration{}, core.ProcessIdentity{}
				a.Status = core.Failed
				saveStoppedClaim(t, f, a)
			}
			err := f.cmd.rehitch([]string{"worker", "--stopped", "--note", "resume saved work"})
			if err == nil || !strings.Contains(err.Error(), "fixture-reached-new-session") {
				t.Fatalf("recovery did not reach launch: %v", err)
			}
			got, err := p.Read()
			if err != nil || got.ConfiguredModel != "" || got.ConfiguredEffort != "" || got.Native.SessionID != a.Native.SessionID || got.ID != a.ID {
				t.Fatalf("metadata fallback = %+v: %v", got, err)
			}
			log, err := os.ReadFile(f.run.team.Log)
			if err != nil || !strings.Contains(string(log), "configured model/effort unknown") {
				t.Fatalf("unknown fallback absent: %s: %v", log, err)
			}
		})
	}
}

func TestStoppedRehitchForeignNamespaceUsesStopAttestation(t *testing.T) {
	if goruntime.GOOS != "linux" {
		t.Skip("Linux namespace fixture")
	}
	f, a := stoppedRehitchFixture(t)
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		t.Fatal(err)
	}
	a.Process.BootID = strings.TrimSpace(string(boot))
	// The numeric PID exists here; the foreign witness must never address it.
	a.Process.PID = os.Getpid()
	saveStoppedClaim(t, f, a)
	err = f.cmd.rehitch([]string{"worker", "--stopped", "--note", "resume from copied home"})
	if err == nil || !strings.Contains(err.Error(), "fixture-reached-new-session") {
		t.Fatalf("foreign namespace recovery = %v", err)
	}
}

func TestStoppedRehitchRechecksChangedRecordAndLock(t *testing.T) {
	for _, variant := range []string{"changed", "locked"} {
		t.Run(variant, func(t *testing.T) {
			f, previous := stoppedRehitchFixture(t)
			p, _ := f.run.team.Agent(previous.ID)
			if variant == "changed" {
				changed := previous
				changed.Native.SessionID = "another-session"
				saveStoppedClaim(t, f, changed)
			} else {
				l, err := p.TryLock()
				if err != nil {
					t.Fatal(err)
				}
				defer l.Close()
			}
			before, _ := os.ReadFile(p.State)
			b, _ := f.cmd.tmux(f.run.settings)
			next := nextStoppedAgent(previous)
			l, err := f.run.prepareStoppedRehitch(context.Background(), b, previous, &next)
			if l != nil {
				_ = l.Close()
			}
			if err == nil {
				t.Fatal("recovered changed/locked record")
			}
			after, _ := os.ReadFile(p.State)
			if !bytes.Equal(before, after) {
				t.Fatal("refusal changed record")
			}
		})
	}
}

func TestStoppedRehitchRetriesAfterRegistrationFailure(t *testing.T) {
	f, a := stoppedRehitchFixture(t)
	p, _ := f.run.team.Agent(a.ID)
	mail := core.Envelope{ID: "queued", Recipient: a.ID, To: a.Name, Message: core.Message{Text: "retained work"}}
	if err := p.Publish(mail); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(a.Directory, "fake-generation")
	launches := filepath.Join(a.Directory, "fake-launches")
	program := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" + fakeTmuxUTF8 +
		"if [ \"$1\" = -S ]; then shift 2; fi\n" +
		"case \"$1\" in\n" +
		"has-session) [ -e '" + marker + "' ];;\n" +
		"new-session) prev= prev2= generation=; for argument; do [ \"$prev2\" = -soq ] && generation=$argument; prev2=$prev; prev=$argument; done; " +
		"printf '%s' \"$generation\" > '" + marker + "'; printf 'x' >> '" + launches + "'; printf '%%9\\n%s\\t$1\\t%%9\\n' \"$generation\";;\n" +
		"list-panes) if [ -e '" + marker + "' ]; then printf '%s\\t$1\\t%%9\\tunit\\n' \"$(cat '" + marker + "')\"; else echo 'no server running on fixture' >&2; exit 1; fi;;\n" +
		"display-message) echo fixture-server-identity-read-failed >&2; exit 91;;\n" +
		"if-shell) rm '" + marker + "';;\n" +
		"*) echo unexpected-tmux-command >&2; exit 92;; esac\n"
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(program), 0700); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		err := f.cmd.rehitch([]string{"worker", "--stopped", "--note", "continue saved work"})
		if err == nil || !strings.Contains(err.Error(), "fixture-server-identity-read-failed") {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		got, err := p.Read()
		if err != nil || got.Pane != "" || got.Process.PID != 0 || got.Registration.Generation == a.Registration.Generation || got.ID != a.ID || got.Native.SessionID != a.Native.SessionID || got.ConfiguredModel != a.ConfiguredModel {
			t.Fatalf("attempt %d retained state: %+v: %v", attempt, got, err)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("pane cleanup not witnessed: %v", err)
		}
		pending, err := p.ListNew()
		if err != nil || len(pending) != 1 || !reflect.DeepEqual(pending[0], mail) {
			t.Fatalf("attempt %d queued mail: %+v: %v", attempt, pending, err)
		}
	}
	got, err := os.ReadFile(launches)
	if err != nil || string(got) != "xx" {
		t.Fatalf("launch count: %q: %v", got, err)
	}
}

func TestStoppedRehitchProcessInvisibleAcrossTwoRestorations(t *testing.T) {
	f, a := stoppedRehitchFixture(t)
	a.Process = core.ProcessIdentity{}
	saveStoppedClaim(t, f, a)
	p, _ := f.run.team.Agent(a.ID)
	b, _ := f.cmd.tmux(f.run.settings)
	for attempt := 0; attempt < 2; attempt++ {
		previous, err := p.Read()
		if err != nil {
			t.Fatal(err)
		}
		next := nextStoppedAgent(previous)
		l, err := f.run.prepareStoppedRehitch(context.Background(), b, previous, &next)
		if err != nil {
			t.Fatalf("restoration %d: %v", attempt, err)
		}
		// Model the fresh registration of a harness whose PID is not visible.
		next.Pane, next.Status = "%7", core.Active
		next.Registration.Generation = strings.Repeat(strconv.Itoa(attempt+1), 64)
		next.Registration.TokenHash = tokenHash("fresh-nonce")
		if err := l.Save(next); err != nil {
			t.Fatal(err)
		}
		_ = l.Close()
	}
	log, err := os.ReadFile(f.run.team.Log)
	if err != nil || strings.Count(string(log), "native process witness unavailable, relying on stop attestation") != 2 {
		t.Fatalf("unknown process evidence: %s: %v", log, err)
	}
}
