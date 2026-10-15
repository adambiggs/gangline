package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

type outsideOptions struct {
	credentialFile, listen, issuer, clientID, redirect string
	roots                                              reportDirectories
}

func outsideFlags(o *outsideOptions) *flag.FlagSet {
	return boundFlagSet("outside", map[string]any{"credential-file": &o.credentialFile, "listen": &o.listen, "issuer": &o.issuer, "client-id": &o.clientID, "redirect-uri": &o.redirect, "allow-read": &o.roots})
}

func (cmd command) outside(args []string) error {
	if len(args) == 0 {
		return usageError("outside: register, rotate, remove or serve required")
	}
	if args[0] == "serve" {
		return cmd.outsideServe(args[1:])
	}
	action := args[0]
	if action != "register" && action != "rotate" && action != "remove" {
		return usageError("outside: unknown action %q", action)
	}
	var o outsideOptions
	flags := outsideFlags(&o)
	positionals, err := parseOptions(flags, args[1:])
	if err != nil {
		return usageError("outside: %v", err)
	}
	for _, option := range optionsFor("outside") {
		if option.name != "credential-file" && flagWasSet(flags, option.name) {
			return usageError("outside %s: --%s is a serve option", action, option.name)
		}
	}
	if len(positionals) != 1 {
		return usageError("outside: one agent name required")
	}
	name := positionals[0]
	if err := validateNewAgentName(name); err != nil {
		return err
	}
	if action == "remove" && o.credentialFile != "" {
		return usageError("outside remove: --credential-file is not used")
	}
	if action != "remove" && o.credentialFile == "" {
		return usageError("outside %s: --credential-file PATH required", action)
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	paths := store.Paths{Root: run.settings.StateRoot}
	switch action {
	case "register":
		_, err = paths.RegisterOutside(run.settings.Session, name, o.credentialFile)
	case "rotate":
		err = paths.RotateOutside(run.settings.Session, name, o.credentialFile)
	case "remove":
		err = paths.RemoveOutside(run.settings.Session, name)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s outside registration %s\n", name, action)
	return err
}

func (run *runtime) sendOutside(o sendOptions, registration store.OutsideRegistration) error {
	if o.LiveOnly || o.Supersede || o.Clear || o.At != "" {
		return usageError("outside mailboxes take immediate queued sends without --live-only, --supersede, --clear or --at")
	}
	sender, err := run.sendSender(o)
	if err != nil {
		return err
	}
	reader := run.cmd.stdin
	if o.Body != nil {
		reader = strings.NewReader(*o.Body)
	}
	body, err := readBody(reader)
	if err != nil {
		return err
	}
	id, err := randomID("msg")
	if err != nil {
		return err
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	envelope := core.Envelope{ID: core.EnvelopeID(id), Token: token, To: registration.Name, Recipient: registration.ID, From: sender, Message: core.Message{Text: body}, CreatedAt: run.cmd.now()}
	if _, err := envelopeText(envelope); err != nil {
		return err
	}
	if err := (store.Paths{Root: run.settings.StateRoot}).QueueOutside(run.settings.Session, string(registration.Name), envelope); err != nil {
		return err
	}
	_, err = fmt.Fprintf(run.cmd.stdout, "%s\tqueued\n", id)
	return err
}

type outsideReceipt struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
}
type outsideSendIntent struct {
	Fingerprint string          `json:"fingerprint"`
	Envelope    *core.Envelope  `json:"envelope,omitempty"`
	Attempted   bool            `json:"attempted,omitempty"`
	Receipt     *outsideReceipt `json:"receipt,omitempty"`
}

func outsideSendFingerprint(to, body string) string {
	// Agent names cannot contain NUL, so the separator distinguishes the two
	// request fields without retaining the message in a settled receipt.
	digest := sha256.Sum256([]byte(to + "\x00" + body))
	return hex.EncodeToString(digest[:])
}

// outsideSend derives identity only from the current authenticated registration.
// The team's lock fences credential changes through publication and its receipt.
func (run *runtime) outsideSend(principal, to, body, key string) (receipt outsideReceipt, result error) {
	if key == "" {
		return receipt, usageError("send: idempotency key required")
	}
	if err := validateAgentName(to); err != nil {
		return receipt, err
	}
	if body == "" {
		return receipt, usageError("send: body required")
	}
	paths := store.Paths{Root: run.settings.StateRoot}
	identity, err := paths.ResolveOutsidePrincipal(run.settings.Session, principal)
	if err != nil {
		return receipt, err
	}
	var tick core.HitchID
	result = paths.WithOutside(run.settings.Session, identity, func(mailbox *store.OutsideMailbox) error {
		var intent outsideSendIntent
		found, err := mailbox.Get("send:"+key, &intent)
		if err != nil {
			return err
		}
		if found && intent.Fingerprint != outsideSendFingerprint(to, body) {
			return refuseError("idempotency key already names a different send")
		}
		if found && intent.Receipt != nil {
			receipt = *intent.Receipt
			return nil
		}
		if !found {
			a, err := run.resolve(to)
			if err != nil {
				return err
			}
			if a.Status != core.Active && a.Status != core.Booting {
				return inactiveRecipient(a)
			}
			id, err := randomID("msg")
			if err != nil {
				return err
			}
			token, err := randomEnvelopeToken()
			if err != nil {
				return err
			}
			envelope := core.Envelope{ID: core.EnvelopeID(id), Token: token, From: identity.Sender(), To: a.Name, Recipient: a.ID, Message: core.Message{Text: body}, CreatedAt: run.cmd.now()}
			wire, err := envelopeText(envelope)
			if err != nil {
				return err
			}
			collar, err := loadCollar(a.Collar, run.settings)
			if err != nil {
				return err
			}
			if err := refusePasteHazard(collar, wire); err != nil {
				return err
			}
			intent = outsideSendIntent{Fingerprint: outsideSendFingerprint(to, body), Envelope: &envelope}
			if err := mailbox.Put("send:"+key, intent); err != nil {
				return err
			}
		}
		if intent.Envelope == nil {
			return fmt.Errorf("outside send intent has no envelope or receipt")
		}
		p, err := run.team.Agent(intent.Envelope.Recipient)
		if err != nil {
			return err
		}
		// Do not reconcile or deliver while the mailbox transaction holds the team lock.
		lock, err := p.TryLock()
		if err != nil {
			return err
		}
		defer lock.Close()
		a, err := p.Read()
		if err != nil {
			return err
		}
		current, err := run.team.ResolveName(to)
		if err != nil {
			return err
		}
		if current != a.ID || a.Name != intent.Envelope.To {
			return refuseError("recipient registration changed")
		}
		if a.Status != core.Active && a.Status != core.Booting {
			return inactiveRecipient(a)
		}
		if _, failed, err := retainedStartup(p, "failed", a.LastFailed); err != nil {
			return err
		} else if failed {
			return refuseError("startup contract input is unverified; recover the recipient before sending")
		}
		published, err := envelopePublished(p, intent.Envelope.ID)
		if err != nil {
			return err
		}
		if !published && intent.Attempted {
			receipt = outsideReceipt{ID: string(intent.Envelope.ID), Outcome: "unknown"}
		} else {
			if !published {
				intent.Attempted = true
				if err := mailbox.Put("send:"+key, intent); err != nil {
					return err
				}
				if err := p.PublishNew(*intent.Envelope); err != nil && !errors.Is(err, store.ErrEnvelopeQueued) {
					return err
				}
			}
			receipt = outsideReceipt{ID: string(intent.Envelope.ID), Outcome: "queued"}
			tick = a.ID
		}
		return mailbox.Put("send:"+key, outsideSendIntent{Fingerprint: intent.Fingerprint, Receipt: &receipt})
	})
	if result == nil && tick != "" {
		if err := run.detachTick(tick); err != nil {
			_, result = fmt.Fprintf(run.cmd.stderr, "warning: queued %s; tick failed: %v\n", receipt.ID, err)
		}
	}
	return receipt, result
}

// Outside records never have native state or a pane.
func isOutsideID(id core.HitchID) bool { return strings.HasPrefix(string(id), "outside-") }
