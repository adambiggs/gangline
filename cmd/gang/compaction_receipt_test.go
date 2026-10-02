package main

import (
	"errors"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func holdStartup(t *testing.T, f *stateFixture, a *core.Agent, p store.AgentPaths, purpose string) {
	t.Helper()
	retained := core.Envelope{ID: "startup-1", Token: "00112233445566ff", From: core.Sender{Kind: core.SenderGangline, Name: core.AgentName(purpose)}, Recipient: a.ID, To: a.Name, Message: core.Message{Text: "contract"}, Purpose: purpose, CreatedAt: f.cmd.now()}
	if err := p.Publish(retained); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Settle(a, retained, "unverified", "native input not yet confirmed"); err != nil {
		t.Fatal(err)
	}
}

func assertStartupHeld(t *testing.T, p store.AgentPaths) core.Agent {
	t.Helper()
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if _, held, err := retainedStartup(p, "failed", got.LastFailed); err != nil || !held {
		t.Fatalf("compaction released the startup hold: LastFailed=%q, %v", got.LastFailed, err)
	}
	return got
}

func TestCompactionRefusedWhileStartupHeld(t *testing.T) {
	for _, purpose := range []string{"startup", "assignment"} {
		t.Run(purpose, func(t *testing.T) {
			f, a, p := compactionFixture(t)
			holdStartup(t, f, &a, p, purpose)
			f.input.screen = screenWithText("READY", "› ")
			err := f.cmd.compact([]string{"worker"})
			var ce commandError
			if !errors.As(err, &ce) || ce.status != exitRefused {
				t.Fatalf("compaction not refused: %v", err)
			}
			got := assertStartupHeld(t, p)
			if f.input.submits != 0 || got.Compaction != nil {
				t.Fatalf("refused compaction changed state: %+v submits=%d", got.Compaction, f.input.submits)
			}
		})
	}
}

func TestQueuedCompactionWaitsForStartupHold(t *testing.T) {
	f, a, p := compactionFixture(t)
	f.input.screen = screenWithText("Working (esc to interrupt)", "› ")
	if err := f.cmd.compact([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	holdStartup(t, f, &a, p, "assignment")
	f.input.screen = screenWithText("READY", "› ")
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "turn-finished", SessionID: "s", At: f.cmd.now()}, false); err != nil {
		t.Fatal(err)
	}
	got := assertStartupHeld(t, p)
	if f.input.submits != 0 || got.Compaction == nil || got.Compaction.Status != "queued" {
		t.Fatalf("held compaction started: %+v submits=%d", got.Compaction, f.input.submits)
	}
}

func TestCompactRefusesALockedAgentWithItsRetry(t *testing.T) {
	f, _, p := compactionFixture(t)
	f.input.screen = screenWithText("READY", "› ")
	held, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	for _, c := range []struct {
		args  []string
		retry string
	}{
		{[]string{"worker"}, "gang compact worker"},
		{[]string{"worker", "--resume", "continue"}, "gang compact worker --resume with the same note"},
		{[]string{"worker", "--recover"}, "gang compact worker --recover"},
	} {
		err = f.cmd.compact(c.args)
		var ce commandError
		if !errors.As(err, &ce) || ce.status != exitRefused || err.Error() != "worker is busy with another gang operation; retry "+c.retry {
			t.Fatalf("compact %v on a locked agent: %v", c.args, err)
		}
	}
}
