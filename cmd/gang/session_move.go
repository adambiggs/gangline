package main

import (
	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
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
