package main

import (
	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// followNativeSession reports whether session, named by a hook record with its
// transcript, is a's native session. An agent with no session yet takes any.
// A session the harness's own records show continuing a's conversation
// replaces a's session and transcript, and transcript reading starts over in
// the new transcript. Any other session belongs to another conversation.
func followNativeSession(c harness.Collar, a *core.Agent, session, transcript string) (bool, error) {
	if a.Native.SessionID == "" || session == a.Native.SessionID {
		return true, nil
	}
	moved, err := harness.SessionMoved(c.Primitives.TurnBoundary, a.Native.Transcript, a.Native.SessionID, session, transcript)
	if err != nil || !moved {
		return false, err
	}
	a.Native.SessionID, a.Native.Transcript, a.Native.Offset = session, transcript, 0
	return true, nil
}

// confirmTranscript saves the transcript a hook record reported for a's
// session when a holds none, as for a requested resume session. A session
// move is judged from that transcript, so retained hooks confirm it before a
// submit witness naming another session is judged.
func confirmTranscript(l *store.LockedAgent, a *core.Agent, records []store.StatusHook) error {
	if a.Native.SessionID == "" || a.Native.Transcript != "" {
		return nil
	}
	for _, n := range records {
		if n.SessionID == a.Native.SessionID && n.Transcript != "" {
			a.Native.Transcript = n.Transcript
			return l.Save(*a)
		}
	}
	return nil
}
