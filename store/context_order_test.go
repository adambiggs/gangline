package store

import (
	"github.com/adambiggs/gangline/core"
	"testing"
	"time"
)

func TestContextSequenceOrdersEqualTimestampCrossings(t *testing.T) {
	team := testTeam(t)
	a := testAgent("a", "worker")
	l, err := team.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	at := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	for _, id := range []core.EnvelopeID{"context-10", "context-9"} {
		e := core.Envelope{ID: id, Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "context-band"}, Message: core.Message{Text: "crossed"}, CreatedAt: at}
		if err := l.Paths.Publish(e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := l.Paths.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "context-9" || got[1].ID != "context-10" {
		t.Fatalf("crossing order: %+v", got)
	}
}
