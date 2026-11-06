package main

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

// crossedLogWindow bounds how much of the team log one send reads for the
// recipient's earlier messages to the sender.
const crossedLogWindow = 4 << 20

// unseenFrom lists, oldest first and comma-separated, the tokens of messages
// agent from sent to agent to that were not yet in to's native transcript at
// now. A message is seen when its envelope opener appears in input the
// harness gave to's model, as a new turn or mid-turn. It returns "unknown"
// when that cannot be read; the send itself does not depend on it.
func (run *runtime) unseenFrom(from core.HitchID, to core.Sender, now time.Time) string {
	if to.Kind != core.SenderAgent || to.HitchID == "" || to.HitchID == from {
		return ""
	}
	unseen, err := run.unseenMail(from, to.HitchID, now)
	if err != nil {
		return "unknown"
	}
	tokens := make([]string, 0, len(unseen))
	for _, e := range unseen {
		tokens = append(tokens, e.Token)
	}
	return strings.Join(tokens, ",")
}

// unseenMail reads from's due messages to to from to's queue and from the
// recent team log, drops those that were cancelled or failed, and returns
// the ones to's transcript does not show. The transcript is that of to's
// native session, following a move the harness's records prove from to's
// latest submit witness, as delivery does. A message older than the
// transcript records Gangline reads is judged only while it is still in
// Gangline's queue, since its delivery may lie in an earlier session or
// before the window. The reads take no lock, so a message settling at that
// moment can be misjudged.
func (run *runtime) unseenMail(from, to core.HitchID, now time.Time) ([]core.Envelope, error) {
	p, err := run.team.Agent(to)
	if err != nil {
		return nil, err
	}
	b, err := p.Read()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sent := func(e core.Envelope) bool {
		return e.From.Kind == core.SenderAgent && e.From.HitchID == from && e.Recipient == to &&
			e.Token != "" && !e.NotBefore.After(now)
	}
	queued, err := p.ListNew()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// inQueue holds each candidate's ID, true while it is in to's queue.
	inQueue := map[core.EnvelopeID]bool{}
	var candidates []core.Envelope
	for _, e := range queued {
		if sent(e) {
			inQueue[e.ID] = true
			candidates = append(candidates, e)
		}
	}
	// Events about to's mail are recorded under to, so only those lines
	// are decoded.
	recipient := []byte(`"hitch_id":"` + string(to) + `"`)
	ended := map[string]bool{}
	err = run.team.ReadRecent(crossedLogWindow, func(line []byte) bool {
		return bytes.Contains(line, recipient) && (bytes.Contains(line, []byte(`"send_queued"`)) ||
			bytes.Contains(line, []byte(`"delivery_failed"`)) || bytes.Contains(line, []byte(`"send_cancelled"`)))
	}, func(event core.Event) error {
		switch {
		case event.Type == "send_queued" && event.Envelope != nil:
			if e := *event.Envelope; sent(e) {
				if _, have := inQueue[e.ID]; !have {
					inQueue[e.ID] = false
					candidates = append(candidates, e)
				}
			}
		case event.Type == "delivery_failed" || event.Type == "send_cancelled":
			ended[event.ID] = true
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	candidates = slices.DeleteFunc(candidates, func(e core.Envelope) bool { return ended[string(e.ID)] })
	if len(candidates) == 0 {
		return nil, nil
	}
	c, err := loadCollar(b.Collar, run.settings)
	if err != nil {
		return nil, err
	}
	if c.Primitives.Seen == nil {
		return nil, errors.New("collar declares no seen primitive")
	}
	if w, err := p.ReadWitness(); err == nil && w.SessionID != b.Native.SessionID {
		// A witness from a session the records do not show continuing
		// to's conversation belongs to another, so the bound one stays.
		if _, err := followNativeSession(c, &b, w.SessionID, w.Transcript); err != nil {
			return nil, err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if b.Native.Transcript == "" {
		return nil, errors.New("no native transcript is bound")
	}
	tokens := make([]string, 0, len(candidates))
	for _, e := range candidates {
		tokens = append(tokens, e.Token)
	}
	seen, since, err := harness.SeenTokens(*c.Primitives.Seen, b.Native.Transcript, b.Native.SessionID, tokens)
	if err != nil {
		return nil, err
	}
	unseen := slices.DeleteFunc(candidates, func(e core.Envelope) bool {
		return seen[e.Token] || (!inQueue[e.ID] && e.CreatedAt.Before(since))
	})
	slices.SortStableFunc(unseen, func(a, b core.Envelope) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return unseen, nil
}
