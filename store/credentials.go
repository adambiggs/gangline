package store

import (
	"fmt"

	"github.com/adambiggs/gangline/core"
)

func credentialEvent(kind string, id core.HitchID, name core.AgentName, origin core.CredentialOrigin) core.Event {
	return core.Event{Source: "command", Type: kind, At: origin.At, HitchID: id, Name: name, Actor: &origin.Actor, Command: origin.Command}
}

func validateCredentialOrigin(origin core.CredentialOrigin) error {
	if origin.At.IsZero() {
		return fmt.Errorf("credential change requires its occurrence time")
	}
	_, err := core.EncodeEvent(credentialEvent("sender_added", "validation", "validation", origin))
	return err
}

func (p TeamPaths) auditCredential(kind string, id core.HitchID, name core.AgentName, origin core.CredentialOrigin) error {
	if err := p.Append(credentialEvent(kind, id, name, origin)); err != nil {
		return fmt.Errorf("%s %s completed, but credential audit append failed: %w", kind, name, err)
	}
	return nil
}
