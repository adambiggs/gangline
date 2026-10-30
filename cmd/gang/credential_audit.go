package main

import "github.com/adambiggs/gangline/core"

func (run *runtime) credentialOrigin(command string) (core.CredentialOrigin, error) {
	caller, err := run.observedAgent()
	if err != nil {
		return core.CredentialOrigin{}, err
	}
	actor := core.CredentialActor{Kind: "operator_shell"}
	if caller != nil {
		actor = core.CredentialActor{Kind: "agent", Name: caller.Name, HitchID: caller.ID}
	}
	return core.CredentialOrigin{At: run.cmd.now(), Actor: actor, Command: command}, nil
}
