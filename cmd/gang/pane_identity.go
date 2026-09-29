package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

type paneRegistry interface {
	RegisterPane(context.Context, substrate.PaneID) (tmux.PaneIdentity, error)
	Identity(context.Context, substrate.PaneID) (tmux.Identity, error)
	AcquireTree(context.Context, substrate.PaneID, tmux.Identity) (*tmux.Owned, error)
	RemovePane(context.Context, substrate.PaneID, tmux.Identity) error
	RemoveRegisteredNativePane(context.Context, tmux.PaneIdentity, tmux.Identity) error
	RemoveRegisteredPane(context.Context, tmux.PaneIdentity) error
	CheckPane(context.Context, tmux.PaneIdentity) (bool, error)
	ProcessVisibility(context.Context, substrate.PaneID) (bool, error)
	VerifyCaller(context.Context, substrate.PaneID) error
	SendRegisteredKeys(context.Context, tmux.PaneIdentity, string, substrate.Keys) error
}

func (run *runtime) registry() (paneRegistry, error) {
	if run.cmd.paneBackend != nil {
		return run.cmd.paneBackend, nil
	}
	return run.cmd.tmux(run.settings)
}

func paneIdentity(a core.Agent) tmux.PaneIdentity {
	return tmux.PaneIdentity{Generation: a.Registration.Generation, Session: a.Registration.Session, Pane: a.Pane}
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (run *runtime) checkRecipient(a core.Agent) error {
	if a.Registration.Generation == "" {
		return run.verifyNativeIdentity(a)
	}
	b, err := run.registry()
	if err != nil {
		return err
	}
	present, err := b.CheckPane(context.Background(), paneIdentity(a))
	if err != nil {
		return err
	}
	if !present {
		return refuseError("registered pane is absent from tmux")
	}
	return nil
}

func (run *runtime) verifyCaller(a core.Agent) error {
	b, err := run.registry()
	if err != nil {
		return err
	}
	if a.Registration.Generation != "" && a.Registration.TokenHash != "" {
		token := run.cmd.environment("GANG_AGENT_NONCE")
		if token == "" || tokenHash(token) != a.Registration.TokenHash {
			return refuseError("hitch token does not match the registered pane")
		}
		if err := run.checkRecipient(a); err != nil {
			return err
		}
	}
	if a.Registration.TokenHash == "" {
		if err := run.verifyNativeIdentity(a); err != nil {
			return err
		}
		if a.Registration.Generation != "" {
			if err := run.checkRecipient(a); err != nil {
				return err
			}
		}
	}
	visible, err := b.ProcessVisibility(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		return err
	}
	if !visible {
		if run.cmd.environment("TMUX_PANE") == "" {
			return refuseError("sandboxed hitch identity requires the current pane")
		}
		if a.Registration.Generation == "" {
			return refuseError("hitch has no pane token and host ancestry is unavailable; re-hitch this agent")
		}
		if err := run.noteProcessUnavailable(a); err != nil {
			return err
		}
	} else if err := b.VerifyCaller(context.Background(), substrate.PaneID(a.Pane)); err != nil {
		return err
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	input, err := run.input()
	if err != nil {
		return err
	}
	return requireHarnessForeground(context.Background(), input, substrate.PaneID(a.Pane), c)
}

// This marker is independent of the agent lock: a sender may already hold it.
func (run *runtime) noteProcessUnavailable(a core.Agent) error {
	p, err := run.team.Agent(a.ID)
	if err != nil {
		return err
	}
	marker := filepath.Join(p.Directory, "process-unavailable")
	f, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := run.record(a, core.Event{Type: "process_verification_unavailable", Reason: "host process visibility unavailable; ancestry verification and detached-descendant cleanup skipped"}); err != nil {
		return errors.Join(err, os.Remove(marker))
	}
	return nil
}

func (run *runtime) processLimited(a core.Agent) (bool, error) {
	p, err := run.team.Agent(a.ID)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(filepath.Join(p.Directory, "process-unavailable"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

type paneInput struct {
	harnessInput
	registry paneRegistry
	identity tmux.PaneIdentity
	command  string
}

func (b paneInput) SendKeys(ctx context.Context, pane substrate.PaneID, keys substrate.Keys) error {
	if string(pane) != b.identity.Pane {
		return refuseError("input pane differs from registered pane")
	}
	return b.registry.SendRegisteredKeys(ctx, b.identity, b.command, keys)
}
func (run *runtime) registeredInput(a core.Agent, input harnessInput) harnessInput {
	// The server checks both identity and foreground again when keys are sent.
	b, err := run.registry()
	if err != nil {
		return failedPaneInput{harnessInput: input, err: err}
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return failedPaneInput{harnessInput: input, err: err}
	}
	identity := paneIdentity(a)
	if a.Registration.Generation == "" {
		identity, err = b.RegisterPane(context.Background(), substrate.PaneID(a.Pane))
		if err == nil {
			err = run.verifyNativeIdentity(a)
		}
		if err != nil {
			return failedPaneInput{harnessInput: input, err: err}
		}
	}
	return paneInput{input, b, identity, filepath.Base(c.Launch.Command)}
}

type failedPaneInput struct {
	harnessInput
	err error
}

func (b failedPaneInput) SendKeys(context.Context, substrate.PaneID, substrate.Keys) error {
	return b.err
}

var _ harnessInput = paneInput{}

// Legacy and adopted callers have no inherited capability. Native identity is
// mandatory before their pane can be attributed or bound for an input operation.
func (run *runtime) verifyNativeIdentity(a core.Agent) error {
	if a.Process.PID == 0 {
		return refuseError("hitch has no native process identity; re-hitch this agent")
	}
	b, err := run.registry()
	if err != nil {
		return err
	}
	visible, err := b.ProcessVisibility(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		return err
	}
	if !visible {
		return refuseError("legacy native identity cannot be verified here; re-hitch this agent for sandbox support")
	}
	actual, err := b.Identity(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		return err
	}
	expected := a.Process
	same := expected.PID == actual.PID && expected.BootID == actual.BootID
	if expected.UniqueID != 0 {
		same = same && expected.UniqueID == actual.UniqueID
	} else {
		same = same && expected.Started == actual.Started
	}
	if expected.Namespace != "" {
		same = same && expected.Namespace == actual.Namespace
	}
	if !same {
		return refuseError("pane process differs from its registered native identity; re-hitch this agent")
	}
	return nil
}
