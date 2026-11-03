//go:build acceptance

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func clearFixture(t *testing.T) (*stateFixture, core.Agent, store.AgentPaths) {
	t.Helper()
	f, a, p := rehitchFixture(t)
	a.Rehitch = nil
	a.Role = "lead"
	saveClearAgent(t, p, a)
	if err := p.WriteSessionStart(store.SessionStart{At: f.cmd.now(), SessionID: a.Native.SessionID, Transcript: f.transcript, Source: "startup", Registration: a.Registration.Generation}); err != nil {
		t.Fatal(err)
	}
	return f, a, p
}
func saveClearAgent(t *testing.T, p store.AgentPaths, a core.Agent) {
	t.Helper()
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}
func clearHook(t *testing.T, f *stateFixture, event, session, source, prompt string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"hook_event_name": event, "session_id": session, "source": source, "prompt": prompt, "transcript_path": f.transcript, "turn_id": "turn-" + session})
	cmd := f.cmd
	cmd.stdin = bytes.NewReader(payload)
	if err := cmd.handleHook(nil); err != nil {
		t.Fatal(err)
	}
}
func clearMail(f *stateFixture, a core.Agent, id string, at time.Time) core.Envelope {
	return core.Envelope{ID: core.EnvelopeID(id), Token: "1234567890abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "test"}, Message: core.Message{Text: id}, CreatedAt: at}
}

func TestClearRequestChecksSelfAndInstalledSupport(t *testing.T) {
	for _, variant := range []string{"valid", "outside", "wrong token", "old launch", "foreign registration", "unknown session", "accepted", "unverified", "compaction", "rehitch"} {
		t.Run(variant, func(t *testing.T) {
			f, a, p := clearFixture(t)
			switch variant {
			case "outside":
				delete(f.env, "GANG_AGENT_ID")
				delete(f.env, "GANGLINE_HITCH_ID")
			case "wrong token":
				f.env["GANG_AGENT_NONCE"] = "other"
			case "old launch":
				if err := os.Remove(p.Directory + "/session-start"); err != nil {
					t.Fatal(err)
				}
			case "foreign registration":
				a.Registration.Generation = "other"
			case "unknown session":
				a.Native.SessionID = ""
			case "accepted", "unverified":
				e := clearMail(f, a, "unresolved", f.cmd.now())
				if err := p.Publish(e); err != nil {
					t.Fatal(err)
				}
				l, err := p.TryLock()
				if err != nil {
					t.Fatal(err)
				}
				if err := l.Settle(&a, e, variant, "awaiting proof"); err != nil {
					t.Fatal(err)
				}
				l.Close()
			case "compaction":
				a.Compaction = &core.Compaction{Status: "queued"}
			case "rehitch":
				a.Rehitch = &core.Message{Text: "pending"}
			}
			saveClearAgent(t, p, a)
			err := f.cmd.clear([]string{"--note", "Change sample at revision abc; next: verify current state."})
			got, readErr := p.Read()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if variant == "valid" {
				if err != nil || got.Clear == nil || got.Clear.Status != "queued" {
					t.Fatalf("request=%+v err=%v", got.Clear, err)
				}
			} else if err == nil || got.Clear != nil {
				t.Fatalf("unsafe request accepted: %+v %v", got.Clear, err)
			}
			if f.input.submits != 0 || got.Pane != a.Pane || got.Process != a.Process {
				t.Fatal("request changed native process/input")
			}
		})
	}
}

func TestClearHoldsMailWhileOldTurnRuns(t *testing.T) {
	f, a, p := clearFixture(t)
	a.Activity = core.Busy
	saveClearAgent(t, p, a)
	if err := f.cmd.clear([]string{"--note", "next phase"}); err != nil {
		t.Fatal(err)
	}
	a, _ = p.Read()
	for _, id := range []string{"before", "arrival"} {
		if err := p.Publish(clearMail(f, a, id, f.cmd.now())); err != nil {
			t.Fatal(err)
		}
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, pending, err := f.run.drainLocked(l, &a, "")
	if err != nil || len(pending) != 3 || f.input.submits != 0 || a.Clear.Status != "queued" {
		t.Fatalf("barrier pending=%d submitted=%d clear=%+v err=%v", len(pending), f.input.submits, a.Clear, err)
	}
}

func TestClearNativeAndContinuationOrder(t *testing.T) {
	for _, lazy := range []bool{false, true} {
		t.Run(map[bool]string{false: "immediate", true: "lazy"}[lazy], func(t *testing.T) {
			f, a, p := clearFixture(t)
			a.HitchedBy = "parent"
			saveClearAgent(t, p, a)
			child := f.add(t, "child", "child", "codex")
			child.HitchedBy = a.ID
			cp, _ := f.run.team.Agent(child.ID)
			saveClearAgent(t, cp, child)
			if err := p.Publish(clearMail(f, a, "retained", f.cmd.now().Add(-time.Minute))); err != nil {
				t.Fatal(err)
			}
			future := clearMail(f, a, "future", f.cmd.now())
			future.NotBefore = f.cmd.now().Add(time.Hour)
			if err := p.Publish(future); err != nil {
				t.Fatal(err)
			}
			expired := clearMail(f, a, "expired", f.cmd.now())
			expired.NotAfter = f.cmd.now().Add(-time.Second)
			if err := p.Publish(expired); err != nil {
				t.Fatal(err)
			}
			if err := f.cmd.clear([]string{"--note", "Change sample revision abc; next phase only."}); err != nil {
				t.Fatal(err)
			}
			a, _ = p.Read()
			before := a
			var submitted []string
			f.input.onKeys = func(k substrate.Keys) error {
				if k.Text != "" {
					text := strings.TrimSuffix(strings.TrimPrefix(k.Text, "\x1b[200~"), "\x1b[201~")
					f.input.screen = screenWithText("READY", "› "+text)
				}
				return nil
			}
			f.input.submit = func(prompt string) error {
				submitted = append(submitted, prompt)
				f.input.screen = screenWithText("READY", "› ")
				if prompt == "/clear" {
					if err := p.Publish(clearMail(f, a, "arrived-during-clear", f.cmd.now().Add(time.Nanosecond))); err != nil {
						return err
					}
					if !lazy {
						clearHook(t, f, "SessionStart", "new-session", "clear", "")
					}
					return nil
				}
				if len(submitted) == 2 && lazy {
					clearHook(t, f, "SessionStart", "new-session", "clear", "")
				}
				clearHook(t, f, "UserPromptSubmit", "new-session", "", prompt)
				return nil
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			_, pending, err := f.run.drainLocked(l, &a, "")
			l.Close()
			if err != nil {
				t.Fatal(err)
			}
			if len(submitted) != 4 || submitted[0] != "/clear" || !strings.Contains(submitted[1], "Change sample revision") || !strings.Contains(submitted[2], "retained") || !strings.Contains(submitted[3], "arrived-during-clear") {
				t.Fatalf("submission ordering=%q", submitted)
			}
			if strings.Index(submitted[1], "Role brief") < 0 || strings.Index(submitted[1], "Role brief") > strings.Index(submitted[1], "Change sample revision") {
				t.Fatal("role did not precede note")
			}
			if a.Clear.Status != "completed" || a.Native.SessionID != "new-session" || a.ID != before.ID || a.Pane != before.Pane || a.Process != before.Process || a.Registration != before.Registration || a.HitchedBy != before.HitchedBy {
				t.Fatalf("continuity=%+v", a)
			}
			if len(pending) != 1 || pending[0].ID != "future" {
				t.Fatalf("remaining=%+v", pending)
			}
			gotChild, _ := cp.Read()
			if gotChild.HitchedBy != a.ID {
				t.Fatal("child lineage lost")
			}
			f.out.Reset()
			clearHook(t, f, "UserPromptSubmit", "new-session", "", submitted[1])
			if !strings.Contains(f.out.String(), `"decision":"block"`) {
				t.Fatal("duplicate continuation admitted")
			}
			w, _ := p.ReadWitness()
			if err := p.WriteWitness(store.Witness{ID: "late-old", SessionID: before.Native.SessionID, At: f.cmd.now().Add(time.Hour)}); err == nil {
				t.Fatal("late old writer accepted")
			}
			kept, _ := p.ReadWitness()
			if kept.ID != w.ID {
				t.Fatal("old writer replaced current proof")
			}
		})
	}
}

func TestClearDefersUnsafeComposerAndQueue(t *testing.T) {
	for _, variant := range []string{"draft", "queue", "permission", "open turn"} {
		t.Run(variant, func(t *testing.T) {
			f, a, p := clearFixture(t)
			if err := f.cmd.clear([]string{"--note", "next phase"}); err != nil {
				t.Fatal(err)
			}
			a, _ = p.Read()
			switch variant {
			case "draft":
				f.input.screen = screenWithText("READY", "› /rename unfinished")
			case "queue":
				f.input.screen = screenWithText("• Queued follow-up inputs", "  ↳ manual queued message", "", "› ")
			case "permission":
				f.input.screen = screenWithText("Do you want to allow this command?", "1. Yes, proceed", "2. No")
			case "open turn":
				a.Native.FinishedAt = time.Time{}
			}
			saveClearAgent(t, p, a)
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			if err := f.run.advanceClear(l, &a); err != nil {
				t.Fatal(err)
			}
			if a.Clear.Status != "queued" || f.input.submits != 0 {
				t.Fatalf("unsafe clear: %+v", a.Clear)
			}
		})
	}
}

func TestClearAdmissionRequiresExactNoteAndFreshProof(t *testing.T) {
	for _, variant := range []string{"missing", "old", "wrong source", "altered", "duplicate", "valid"} {
		t.Run(variant, func(t *testing.T) {
			f, a, p := clearFixture(t)
			if err := f.cmd.clear([]string{"--note", "current change revision abc"}); err != nil {
				t.Fatal(err)
			}
			a, _ = p.Read()
			a.Clear.Status = "continuing"
			a.Clear.StartedAt = f.cmd.now()
			saveClearAgent(t, p, a)
			n := store.SessionStart{At: f.cmd.now(), SessionID: "new", Transcript: f.transcript, Source: "clear", Registration: a.Registration.Generation}
			if variant == "old" {
				n.SessionID = a.Native.SessionID
			}
			if variant == "wrong source" {
				n.Source = "resume"
			}
			if variant != "missing" && variant != "old" {
				if err := p.WriteSessionStart(n); err != nil {
					t.Fatal(err)
				}
			}
			c, _ := harness.EmbeddedCollar("codex")
			wire, _ := envelopeText(a.Clear.Envelope)
			if variant == "altered" {
				wire += " injected"
			}
			if variant == "duplicate" {
				if err := p.AdmitClear(a.Clear.Envelope.ID, "new", a.Registration.Generation); err != nil {
					t.Fatal(err)
				}
			}
			// Hold the delivery lock while the hook-side guard runs: no lock inversion.
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			err = f.run.admitClearPrompt(p, a, c, wire, n.SessionID)
			if (err == nil) != (variant == "valid") {
				t.Fatalf("admission %s: %v", variant, err)
			}
		})
	}
}

func TestClearDoesNotRepeatSubmittedCommand(t *testing.T) {
	f, a, p := clearFixture(t)
	if err := f.cmd.clear([]string{"--note", "next phase"}); err != nil {
		t.Fatal(err)
	}
	a, _ = p.Read()
	a.Clear.Status = "submitted"
	a.Clear.StartedAt = f.cmd.now()
	saveClearAgent(t, p, a)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for range 2 {
		if err := f.run.advanceClear(l, &a); err != nil {
			t.Fatal(err)
		}
	}
	if f.input.submits != 0 || a.Clear.Status != "submitted" {
		t.Fatal("uncertain native clear was repeated")
	}
}

func TestClearRejectsRetiredNativeObservations(t *testing.T) {
	f, a, p := clearFixture(t)
	if err := f.cmd.clear([]string{"--note", "next phase"}); err != nil {
		t.Fatal(err)
	}
	a, _ = p.Read()
	a.Clear.Status = "continuing"
	a.Clear.StartedAt = f.cmd.now()
	saveClearAgent(t, p, a)
	if err := p.WriteSessionStart(store.SessionStart{At: f.cmd.now(), SessionID: "new", Transcript: f.transcript, Source: "clear", Registration: a.Registration.Generation}); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := f.run.reconcileClear(l, &a); err != nil {
		t.Fatal(err)
	}
	// A retained old witness predates the new side-file writer protections.
	raw, _ := json.Marshal(store.Witness{ID: "old-witness", At: f.cmd.now().Add(time.Hour), SessionID: "s", Transcript: "old-transcript", TurnID: "old-turn"})
	if err := os.WriteFile(p.Witness, raw, 0600); err != nil {
		t.Fatal(err)
	}
	c, _ := harness.EmbeddedCollar("codex")
	if err := f.run.reconcileNativeBoundary(l, &a, c, hookNotice{Kind: "turn-finished", SessionID: "s", Transcript: "old-transcript", TurnID: "old-turn", At: f.cmd.now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if a.Native.SessionID != "new" || a.Native.Transcript != f.transcript || a.Native.TurnID != "" || a.Status != core.Active {
		t.Fatalf("old observation restored: %+v", a.Native)
	}
}

func TestClearContinuationUncertaintyHoldsLaterMail(t *testing.T) {
	f, a, p := clearFixture(t)
	if err := f.cmd.clear([]string{"--note", "next phase"}); err != nil {
		t.Fatal(err)
	}
	a, _ = p.Read()
	a.Clear.Status = "continuing"
	a.Clear.StartedAt = f.cmd.now()
	e := a.Clear.Envelope
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Settle(&a, e, "unverified", "hook did not confirm"); err != nil {
		t.Fatal(err)
	}
	if err := p.Publish(clearMail(f, a, "later", f.cmd.now())); err != nil {
		t.Fatal(err)
	}
	_, pending, err := f.run.drainLocked(l, &a, "")
	if err != nil || len(pending) != 1 || f.input.submits != 0 {
		t.Fatalf("uncertain startup bypassed: %v %+v", err, pending)
	}
}

func TestClearHookFailsClosedBeforeAdmission(t *testing.T) {
	for _, variant := range []string{"state unreadable", "wrong token", "collar unreadable", "proof unreadable"} {
		t.Run(variant, func(t *testing.T) {
			f, a, p := clearFixture(t)
			if err := f.cmd.clear([]string{"--note", "next phase"}); err != nil {
				t.Fatal(err)
			}
			a, _ = p.Read()
			a.Clear.Status, a.Clear.StartedAt = "continuing", f.cmd.now()
			if variant == "collar unreadable" {
				a.Collar = "missing"
			}
			saveClearAgent(t, p, a)
			wire, err := envelopeText(a.Clear.Envelope)
			if err != nil {
				t.Fatal(err)
			}
			switch variant {
			case "state unreadable":
				if err := os.WriteFile(p.State, []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "wrong token":
				f.env["GANG_AGENT_NONCE"] = "wrong"
			case "proof unreadable":
				if err := os.WriteFile(p.Directory+"/session-start", []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			f.out.Reset()
			payload, _ := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "new", "prompt": wire})
			cmd := f.cmd
			cmd.stdin = bytes.NewReader(payload)
			if err := cmd.hook(nil); err != nil {
				t.Fatal(err)
			}
			if strings.Count(f.out.String(), `"decision":"block"`) != 1 {
				t.Fatalf("unguarded prompt: %s", f.out.String())
			}
			if _, err := p.ReadClearAdmission(); err == nil {
				t.Fatal("failed check admitted continuation")
			}
		})
	}
}

func TestClearDeliveredReceiptSurvivesCrash(t *testing.T) {
	f, a, p := clearFixture(t)
	if err := f.cmd.clear([]string{"--note", "next phase"}); err != nil {
		t.Fatal(err)
	}
	a, _ = p.Read()
	a.Clear.Status = "continuing"
	a.Clear.StartedAt = f.cmd.now()
	saveClearAgent(t, p, a)
	if err := p.WriteSessionStart(store.SessionStart{At: f.cmd.now(), SessionID: "new", Transcript: f.transcript, Source: "clear", Registration: a.Registration.Generation}); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := f.run.reconcileClear(l, &a); err != nil {
		t.Fatal(err)
	}
	e := a.Clear.Envelope
	if err := l.Settle(&a, e, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := p.Publish(clearMail(f, a, "later", f.cmd.now())); err != nil {
		t.Fatal(err)
	}
	if err := f.run.reconcileClear(l, &a); err != nil {
		t.Fatal(err)
	}
	if clearPending(a) {
		t.Fatalf("durable delivered receipt not reconciled: clear=%s lastDelivered=%s", a.Clear.Status, a.LastDelivered)
	}
}
func TestClearSecondRequestRejectsOlderWitness(t *testing.T) {
	f, a, p := clearFixture(t)
	a.Clear = &core.ClearRequest{Envelope: core.Envelope{ID: "first-clear"}, PreviousSession: a.Native.SessionID, Status: "completed", StartedAt: f.cmd.now()}
	a.Native.SessionID = "middle"
	saveClearAgent(t, p, a)
	if err := p.WriteSessionStart(store.SessionStart{At: f.cmd.now(), SessionID: "middle", Transcript: f.transcript, Source: "clear", Registration: a.Registration.Generation}); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.clear([]string{"--note", "second phase"}); err != nil {
		t.Fatal(err)
	}
	a, _ = p.Read()
	if err := p.WriteWitness(store.Witness{ID: "late-old", SessionID: "s", Transcript: f.transcript, At: f.cmd.now().Add(time.Second)}); err != nil {
		return
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, _ := harness.EmbeddedCollar("codex")
	if err := f.run.reconcileNativeBoundary(l, &a, c, hookNotice{}); err != nil {
		t.Fatal(err)
	}
	if a.Status != core.Active {
		t.Fatalf("second queued clear lost retired witness guard: status=%s reason=%s", a.Status, a.Evidence)
	}
}

func TestClearRecoversOnlyUnadmittedContinuationDraft(t *testing.T) {
	for _, admitted := range []bool{false, true} {
		t.Run(map[bool]string{false: "unsubmitted", true: "already admitted"}[admitted], func(t *testing.T) {
			f, a, p := clearFixture(t)
			if err := f.cmd.clear([]string{"--note", "next phase"}); err != nil {
				t.Fatal(err)
			}
			a, _ = p.Read()
			a.Clear.Status, a.Clear.StartedAt = "continuing", f.cmd.now()
			saveClearAgent(t, p, a)
			clearHook(t, f, "SessionStart", "new", "clear", "")
			if admitted {
				if err := p.AdmitClear(a.Clear.Envelope.ID, "new", a.Registration.Generation); err != nil {
					t.Fatal(err)
				}
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			e := a.Clear.Envelope
			a.Input = &core.InputIntent{ID: string(e.ID), Kind: "envelope", At: f.cmd.now()}
			if err := f.run.finishInput(l, &a, e, "unverified", "submit not established"); err != nil {
				t.Fatal(err)
			}
			l.Close()
			wire, err := envelopeText(e)
			if err != nil {
				t.Fatal(err)
			}
			f.input.pasted = wire
			f.input.screen = screenWithText("READY", "› "+wire)
			f.input.submit = func(prompt string) error { clearHook(t, f, "UserPromptSubmit", "new", "", prompt); return nil }
			err = f.cmd.hitch([]string{"worker", "--recover"})
			got, readErr := p.Read()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if admitted {
				if err == nil || f.input.submits != 0 {
					t.Fatalf("admitted draft replayed: %v submits=%d", err, f.input.submits)
				}
			} else if err != nil || got.Clear.Status != "completed" || got.Native.SessionID != "new" {
				t.Fatalf("unsubmitted draft not recovered: %v %+v", err, got)
			}
		})
	}
}

func TestClearDiscardsRetiredContextNotes(t *testing.T) {
	f, a, p := clearFixture(t)
	e := clearMail(f, a, "old-context-note", f.cmd.now())
	e.From = core.Sender{Kind: core.SenderGangline, Name: "context-band"}
	e.Message.Text = "Old context full; compact now"
	a.ContextBands.Pending = []core.ContextBandNote{{Band: "red", Envelope: e}}
	saveClearAgent(t, p, a)
	if err := f.cmd.clear([]string{"--note", "next phase"}); err != nil {
		t.Fatal(err)
	}
	a, _ = p.Read()
	a.Clear.Status = "continuing"
	a.Clear.StartedAt = f.cmd.now()
	saveClearAgent(t, p, a)
	if err := p.WriteSessionStart(store.SessionStart{At: f.cmd.now(), SessionID: "new", Transcript: f.transcript, Source: "clear", Registration: a.Registration.Generation}); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := f.run.reconcileClear(l, &a); err != nil {
		t.Fatal(err)
	}
	if err := f.run.publishContextNotes(l, &a); err != nil {
		t.Fatal(err)
	}
	pending, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range pending {
		if isContextBandNotice(got) {
			t.Fatalf("retired context advice retained after clear: %s", got.Message.Text)
		}
	}
}

func TestClearDrainDiscardsOldContextBeforeMail(t *testing.T) {
	for _, lazy := range []bool{false, true} {
		t.Run(map[bool]string{false: "immediate", true: "lazy"}[lazy], func(t *testing.T) {
			f, a, p := clearFixture(t)
			oldContext := clearMail(f, a, "old-context", f.cmd.now().Add(-2*time.Minute))
			oldContext.From = core.Sender{Kind: core.SenderGangline, Name: "context-band"}
			oldContext.Message.Text = "STALE_CONTEXT compact now"
			if err := p.Publish(oldContext); err != nil {
				t.Fatal(err)
			}
			a.HitchedBy = "parent"
			saveClearAgent(t, p, a)
			child := f.add(t, "child", "child", "codex")
			child.HitchedBy = a.ID
			cp, _ := f.run.team.Agent(child.ID)
			saveClearAgent(t, cp, child)
			if err := p.Publish(clearMail(f, a, "retained", f.cmd.now().Add(-time.Minute))); err != nil {
				t.Fatal(err)
			}
			future := clearMail(f, a, "future", f.cmd.now())
			future.NotBefore = f.cmd.now().Add(time.Hour)
			if err := p.Publish(future); err != nil {
				t.Fatal(err)
			}
			expired := clearMail(f, a, "expired", f.cmd.now())
			expired.NotAfter = f.cmd.now().Add(-time.Second)
			if err := p.Publish(expired); err != nil {
				t.Fatal(err)
			}
			if err := f.cmd.clear([]string{"--note", "Change sample revision abc; next phase only."}); err != nil {
				t.Fatal(err)
			}
			a, _ = p.Read()
			before := a
			var submitted []string
			f.input.onKeys = func(k substrate.Keys) error {
				if k.Text != "" {
					text := strings.TrimSuffix(strings.TrimPrefix(k.Text, "\x1b[200~"), "\x1b[201~")
					f.input.screen = screenWithText("READY", "› "+text)
				}
				return nil
			}
			f.input.submit = func(prompt string) error {
				submitted = append(submitted, prompt)
				f.input.screen = screenWithText("READY", "› ")
				if prompt == "/clear" {
					if err := p.Publish(clearMail(f, a, "arrived-during-clear", f.cmd.now().Add(time.Nanosecond))); err != nil {
						return err
					}
					if !lazy {
						clearHook(t, f, "SessionStart", "new-session", "clear", "")
					}
					return nil
				}
				if len(submitted) == 2 && lazy {
					clearHook(t, f, "SessionStart", "new-session", "clear", "")
				}
				clearHook(t, f, "UserPromptSubmit", "new-session", "", prompt)
				return nil
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			_, pending, err := f.run.drainLocked(l, &a, "")
			l.Close()
			if err != nil {
				t.Fatal(err)
			}
			for _, prompt := range submitted {
				if strings.Contains(prompt, "STALE_CONTEXT") {
					t.Fatalf("old context delivered in clear drain: %s", prompt)
				}
			}
			if len(submitted) != 4 || submitted[0] != "/clear" || !strings.Contains(submitted[1], "Change sample revision") || !strings.Contains(submitted[2], "retained") || !strings.Contains(submitted[3], "arrived-during-clear") {
				t.Fatalf("submission ordering=%q", submitted)
			}
			if strings.Index(submitted[1], "Role brief") < 0 || strings.Index(submitted[1], "Role brief") > strings.Index(submitted[1], "Change sample revision") {
				t.Fatal("role did not precede note")
			}
			if a.Clear.Status != "completed" || a.Native.SessionID != "new-session" || a.ID != before.ID || a.Pane != before.Pane || a.Process != before.Process || a.Registration != before.Registration || a.HitchedBy != before.HitchedBy {
				t.Fatalf("continuity=%+v", a)
			}
			if len(pending) != 1 || pending[0].ID != "future" {
				t.Fatalf("remaining=%+v", pending)
			}
			gotChild, _ := cp.Read()
			if gotChild.HitchedBy != a.ID {
				t.Fatal("child lineage lost")
			}
			f.out.Reset()
			clearHook(t, f, "UserPromptSubmit", "new-session", "", submitted[1])
			if !strings.Contains(f.out.String(), `"decision":"block"`) {
				t.Fatal("duplicate continuation admitted")
			}
			w, _ := p.ReadWitness()
			if err := p.WriteWitness(store.Witness{ID: "late-old", SessionID: before.Native.SessionID, At: f.cmd.now().Add(time.Hour)}); err == nil {
				t.Fatal("late old writer accepted")
			}
			kept, _ := p.ReadWitness()
			if kept.ID != w.ID {
				t.Fatal("old writer replaced current proof")
			}
		})
	}
}
