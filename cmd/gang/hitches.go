package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func (cmd command) hitch(arguments []string) error {
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	directory, err := cmd.getwd()
	if err != nil {
		return err
	}
	options, err := parseHitch(arguments, settings.Collar, directory)
	if err != nil {
		return err
	}
	directory, err = filepath.Abs(options.Directory)
	if err != nil {
		return fmt.Errorf("resolve hitch directory: %w", err)
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return refuseError("hitch directory %q is not an accessible directory", directory)
	}
	collar, err := loadCollar(options.Collar, settings)
	if err != nil {
		return err
	}
	if options.Effort != "" && options.Model == "" {
		return usageError("hitch: --effort requires --model")
	}
	if options.Model != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		catalog, discoverErr := harness.DiscoverModels(ctx, collar)
		cancel()
		if discoverErr != nil {
			return discoverErr
		}
		if harness.ValidateModel(catalog, options.Model) == harness.ModelUnrecognized {
			return refuseError("model %q is not recognized by collar %q", options.Model, options.Collar)
		}
		if options.Effort != "" && harness.ValidateEffort(catalog, options.Model, options.Effort) == harness.ModelUnrecognized {
			return refuseError("effort %q is not recognized for model %q", options.Effort, options.Model)
		}
	}
	assignment := options.Task
	if options.Stdin {
		assignment, err = readBody(cmd.stdin)
		if err != nil {
			return err
		}
	}
	if assignment == "" {
		assignment = "Begin the role you were assigned and wait for an attributed Gangline message."
	}
	prose, err := cmd.startupProse(options.Role)
	if err != nil {
		return err
	}
	startup := composeStartup(options.Name, prose, assignment)
	now := time.Now()
	hitchRaw, err := randomID("hitch")
	if err != nil {
		return err
	}
	envelopeRaw, err := randomID("startup")
	if err != nil {
		return err
	}
	hitchID := core.HitchID(hitchRaw)
	run := &runtime{cmd: cmd, settings: settings}
	record := startupRecord{
		Model: options.Model, Effort: options.Effort, Resume: options.Resume,
		RolePrompt: composeStartup(options.Name, prose, ""),
		Event: core.SendRequested{
			At: now, Deadline: now.Add(deliveryTimeout),
			Envelope: core.Envelope{
				ID: core.EnvelopeID(envelopeRaw), From: core.Sender{Kind: core.SenderSelfDeclared, Name: "hitch"},
				To: core.AgentName(options.Name), Message: core.Message{Text: startup}, CreatedAt: now,
			},
		},
	}
	if err := run.writeStartup(hitchID, record); err != nil {
		return fmt.Errorf("queue startup assignment: %w", err)
	}
	state, err := run.drive(core.HitchRequested{
		At: now, BootDeadline: now.Add(bootTimeout),
		Hitch: core.Hitch{ID: hitchID, Name: core.AgentName(options.Name), Collar: options.Collar, Role: options.Role, Directory: directory},
	})
	if err != nil {
		return err
	}
	hitch := state.Hitches[hitchID]
	switch hitch.Status {
	case core.HitchActive:
		_, err = fmt.Fprintf(cmd.stdout, "%s\t%s\n", options.Name, hitch.Pane)
		return err
	case core.HitchBooting:
		return commandError{status: exitNative, text: fmt.Sprintf("%s launched in %s but is not ready; resolve its native prompt, then run 'gang tick'", options.Name, hitch.Pane)}
	default:
		return refuseError("hitch %s failed to launch: %s", options.Name, hitch.WedgeEvidence)
	}
}
func (cmd command) adopt(arguments []string) error {
	if len(arguments) < 1 {
		return usageError("adopt: agent name required")
	}
	name := arguments[0]
	if err := validateAgentName(name); err != nil {
		return err
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	collar := settings.Collar
	flags := quietFlagSet("adopt")
	flags.StringVar(&collar, "c", collar, "harness collar")
	flags.StringVar(&collar, "collar", collar, "harness collar")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 {
		return usageError("adopt: expected NAME -c COLLAR")
	}
	if _, err := loadCollar(collar, settings); err != nil {
		return err
	}
	pane := cmd.environment("TMUX_PANE")
	if pane == "" {
		return refuseError("adopt must run inside the pane being adopted")
	}
	directory, err := cmd.getwd()
	if err != nil {
		return err
	}
	id, err := randomID("hitch")
	if err != nil {
		return err
	}
	run := &runtime{cmd: cmd, settings: settings}
	_, err = run.drive(core.AdoptRequested{At: time.Now(), Pane: pane, Hitch: core.Hitch{
		ID: core.HitchID(id), Name: core.AgentName(name), Collar: collar, Directory: directory,
	}})
	return err
}
func (cmd command) rename(arguments []string) error {
	if err := exactly(arguments, 2, "rename"); err != nil {
		return err
	}
	if err := validateAgentName(arguments[1]); err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	hitch, ok := activeByName(state, arguments[0])
	if !ok {
		return refuseError("agent %q is not active", arguments[0])
	}
	if _, exists := activeByName(state, arguments[1]); exists {
		return refuseError("agent name %q is already active", arguments[1])
	}
	backend, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	if err := backend.Rename(context.Background(), substrate.PaneID(hitch.Pane), arguments[1]); err != nil {
		return err
	}
	_, err = run.drive(core.RenameRequested{At: time.Now(), HitchID: hitch.ID, Name: core.AgentName(arguments[1])})
	return err
}
func (cmd command) drop(arguments []string) error {
	name, err := requiredName(arguments, "drop")
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	hitch, ok := activeByName(state, name)
	if !ok {
		return refuseError("agent %q is not active", name)
	}
	now := time.Now()
	state, err = run.drive(core.DropRequested{At: now, HitchID: hitch.ID, Deadline: now.Add(operationTimeout)})
	if err != nil {
		return err
	}
	if state.Hitches[hitch.ID].Status != core.HitchDropped {
		return refuseError("agent %q was not stopped; inspect 'gang log'", name)
	}
	return nil
}
