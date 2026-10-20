package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func parseTitle(args []string) (name, title string, err error) {
	positionals, err := parseOptions(boundFlagSet("title", nil), args)
	if err != nil {
		return "", "", usageError("title: %v", err)
	}
	switch len(positionals) {
	case 1:
		title = positionals[0]
	case 2:
		name, title = positionals[0], positionals[1]
		if err := validateAgentName(name); err != nil {
			return "", "", err
		}
	default:
		return "", "", usageError("title: expected [NAME] TEXT; quote TEXT as one argument")
	}
	if strings.TrimSpace(title) == "" || strings.ContainsAny(title, "\n\t") {
		return "", "", refuseError("title requires a nonempty single line without tabs")
	}
	if err := validateMessageText(title); err != nil {
		return "", "", err
	}
	return name, title, nil
}

func (cmd command) title(args []string) (result error) {
	name, title, err := parseTitle(args)
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	a, err := run.resolve(name)
	if err != nil {
		return err
	}
	l, a, err := run.acquireBounded(a.ID)
	if errors.Is(err, store.ErrLocked) {
		return refuseError("%s is busy with another gang operation; retry gang title when it finishes", a.Name)
	}
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, run.release(l)) }()
	if a.Status != core.Active {
		return inactiveRecipient(a)
	}
	if a.Input != nil {
		return refuseError("native input is already in progress; inspect %s before retrying gang title", a.Name)
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	action, err := harness.RenderAction(c.Actions.Title, map[string]string{"title": title})
	if err != nil {
		return err
	}
	if err := refusePasteHazard(c, action.Text); err != nil {
		return err
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	b = run.registeredInput(a, b)
	v, err := run.inputState(l, &a, b, c)
	if err != nil {
		return err
	}
	if !v.Free {
		return refuseError("title not submitted: %s", v.Reason)
	}
	ctx, cancel := cmd.timeout(operationTimeout)
	defer cancel()
	screen, err := run.captureRegistered(ctx, b, a)
	if err != nil {
		return err
	}
	if _, err := run.observeScreen(l, &a, c, screen); err != nil {
		return err
	}
	if a.Activity != core.Idle {
		return refuseError("title requires native idle: %s %s", a.Activity, a.Evidence)
	}
	before, err := harness.ActionRefusals(c.Actions.Title, screen)
	if err != nil {
		return err
	}
	input, err := harness.SubmitInput(c.Primitives.Submit, action.Text)
	if err != nil {
		return err
	}
	if err := sendHarnessKeys(ctx, b, substrate.PaneID(a.Pane), c, input); err != nil {
		return err
	}
	settle, err := harness.SubmitSettle(c.Primitives.Submit)
	if err != nil {
		return err
	}
	settleComposer := func() error {
		if cmd.settleInput != nil {
			return cmd.settleInput(ctx, b, substrate.PaneID(a.Pane), c, settle)
		}
		return harness.AwaitOwnedComposerSettle(ctx, b.Capture, substrate.PaneID(a.Pane), c, v.Owner, settle)
	}
	if err := settleComposer(); err != nil {
		return refuseError("title not submitted; inspect the composer: %v", err)
	}
	screen, err = b.Capture(ctx, substrate.PaneID(a.Pane))
	if err != nil {
		return err
	}
	if blocker, blocked, err := harness.InputBlocked(c, screen); err != nil {
		return err
	} else if blocked {
		return refuseError("title not submitted; native input blocked: %s; inspect the staged composer input", blocker.Evidence)
	}
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil {
		return err
	}
	if composer.TailOccupied || composer.Owner != v.Owner || !harness.SameComposerText(composer.Text, action.Text) {
		return refuseError("title not submitted: native composer did not read back as the title command; inspect the composer")
	}
	busy, err := harness.Busy(c, screen)
	if err != nil {
		return err
	}
	compacting, err := harness.CompactionActive(c, screen)
	if err != nil {
		return err
	}
	if busy || compacting {
		return refuseError("title not submitted: native task started during paste; title command remains in the composer")
	}
	if err := sendHarnessKeys(ctx, b, substrate.PaneID(a.Pane), c, substrate.Keys{Names: action.Keys, Submit: action.Submit}); err != nil {
		return commandError{status: exitUnknown, text: fmt.Sprintf("title submission unconfirmed; inspect %s: %v", a.Name, err)}
	}
	if err := harness.AwaitComposerText(ctx, b.Capture, substrate.PaneID(a.Pane), c, "", settle); err != nil {
		return commandError{status: exitUnknown, text: "title submit key sent; native outcome unconfirmed: " + err.Error()}
	}
	screen, err = b.Capture(ctx, substrate.PaneID(a.Pane))
	if err != nil {
		return commandError{status: exitUnknown, text: "title submit key sent; native outcome unconfirmed: " + err.Error()}
	}
	after, err := harness.ActionRefusals(c.Actions.Title, screen)
	if err != nil {
		return err
	}
	if len(after) > len(before) {
		return commandError{status: exitNative, text: "native title refused: " + strings.TrimSpace(after[len(after)-1])}
	}
	return commandError{status: exitUnknown, text: "title command submitted; native title persistence unconfirmed"}
}
