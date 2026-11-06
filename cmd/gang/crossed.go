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
// the ones to's transcript does not show. A message older than the
// transcript records Gangline reads is judged only while it is still queued,
// since its delivery may lie in an earlier session or outside the window.
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
	inQueue, have := map[core.EnvelopeID]bool{}, map[core.EnvelopeID]bool{}
	var candidates []core.Envelope
	for _, e := range queued {
		if sent(e) {
			inQueue[e.ID], have[e.ID] = true, true
			candidates = append(candidates, e)
		}
	}
	ended := map[string]bool{}
	err = run.team.ReadRecent(crossedLogWindow, func(line []byte) bool {
		return bytes.Contains(line, []byte(`"send_queued"`)) || bytes.Contains(line, []byte(`"delivery_failed"`)) ||
			bytes.Contains(line, []byte(`"send_cancelled"`))
	}, func(event core.Event) error {
		switch {
		case event.Type == "send_queued" && event.Envelope != nil:
			if e := *event.Envelope; sent(e) && !have[e.ID] {
				have[e.ID] = true
				candidates = append(candidates, e)
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
	if b.Native.Transcript == "" {
		return nil, errors.New("no native transcript is bound")
	}
	tokens := make([]string, 0, len(candidates))
	for _, e := range candidates {
		tokens = append(tokens, e.Token)
	}
	seen, since, err := harness.SeenTokens(*c.Primitives.Seen, b.Native.Transcript, tokens)
	if err != nil {
		return nil, err
	}
	unseen := slices.DeleteFunc(candidates, func(e core.Envelope) bool {
		return seen[e.Token] || (!inQueue[e.ID] && e.CreatedAt.Before(since))
	})
	slices.SortStableFunc(unseen, func(a, b core.Envelope) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return unseen, nil
}
