package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func (cmd command) senderCommand(args []string) error {
	if len(args) == 0 {
		return usageError("sender: add, list or remove required")
	}
	action := args[0]
	if action != "add" && action != "list" && action != "remove" {
		return usageError("sender: unknown action %q", action)
	}
	count := 2
	if action == "list" {
		count = 1
	}
	if len(args) != count {
		return usageError("sender %s: expected %d name arguments", action, count-1)
	}
	if count == 2 {
		if err := validateNewAgentName(args[1]); err != nil {
			return err
		}
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	if action == "list" {
		rows, err := run.team.ListSenders()
		if err != nil {
			return err
		}
		for _, r := range rows {
			if _, err := fmt.Fprintln(cmd.stdout, r.Name); err != nil {
				return err
			}
		}
		return nil
	}
	a, err := run.observedAgent()
	if err != nil {
		return err
	}
	if a != nil && (!run.leadCaller(a) || a.Status != core.Active) {
		return refuseError("sender %s requires the active lead or operator shell; ask the lead to run gang sender %s %s", action, action, args[1])
	}
	lock, err := run.lockTeam()
	if err != nil {
		return err
	}
	defer lock.Close()
	name := args[1]
	if action == "remove" {
		if err := run.team.RemoveSender(name); err != nil {
			return refuseError("cannot remove registered sender %q: %v; gang sender list shows registered names", name, err)
		}
		_, err = fmt.Fprintf(cmd.stdout, "%s registered sender removed\n", name)
		return err
	}
	for host := range run.settings.Peers {
		if strings.EqualFold(host, name) {
			return refuseError("sender name %q is a peer host; choose another name (gang sender list lists registered senders)", name)
		}
	}
	if strings.EqualFold(run.settings.Host, name) {
		return refuseError("sender name %q is this host; choose another name", name)
	}
	path, err := run.team.AddSender(name)
	if errors.Is(err, store.ErrNameTaken) {
		return refuseError("sender name %q is already claimed; choose another name (gang roster and gang sender list show registrations)", name)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.stdout, path)
	return err
}

func (run *runtime) sendSender(o sendOptions) (core.Sender, error) {
	if o.As == "" {
		return run.sender(o.From)
	}
	// Preserve inherited caller verification even when using a bearer credential.
	if _, err := run.observedAgent(); err != nil {
		return core.Sender{}, err
	}
	path := o.Credential
	if path == "" {
		path = run.cmd.environment("GANG_SENDER_CREDENTIAL")
	}
	route := "supply --credential FILE or GANG_SENDER_CREDENTIAL using the key issued by the operator or lead for --as " + o.As
	if path == "" {
		return core.Sender{}, refuseError("registered sender credential is missing; %s", route)
	}
	credential, err := os.ReadFile(path)
	if err != nil {
		return core.Sender{}, refuseError("cannot read registered sender credential: %v; %s", err, route)
	}
	sender, err := run.team.AuthenticateSender(o.As, credential)
	if err != nil {
		return core.Sender{}, refuseError("registered sender credential refused: %v; %s", err, route)
	}
	return sender, nil
}
