package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func (cmd command) up(arguments []string) error {
	name := "lead"
	if len(arguments) != 0 && !strings.HasPrefix(arguments[0], "-") {
		name = arguments[0]
		arguments = arguments[1:]
	}
	if err := cmd.hitch(append([]string{name}, arguments...)); err != nil {
		return err
	}
	if file, ok := cmd.stdin.(*os.File); ok {
		if info, err := file.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return cmd.attach(nil)
		}
	}
	return nil
}

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

func (cmd command) send(arguments []string) error {
	options, err := parseSend(arguments)
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
	if options.At == "clear" {
		_, err = run.drive(core.TimedDeliveriesCleared{At: time.Now(), Recipient: core.AgentName(options.Name)})
		return err
	}
	body, err := readBody(cmd.stdin)
	if err != nil {
		return err
	}
	sender, err := cmd.sender(state, options.From)
	if err != nil {
		return err
	}
	if options.Supersede {
		if _, err := run.drive(core.TimedDeliveriesCleared{At: time.Now(), Recipient: core.AgentName(options.Name)}); err != nil {
			return err
		}
	}
	if options.LiveOnly {
		hitch, ok := activeByName(state, options.Name)
		if !ok || hitch.Activity != core.ActivityIdle {
			return refuseError("agent %q is not immediately deliverable", options.Name)
		}
	}
	now := time.Now()
	var notBefore time.Time
	if options.At != "" {
		notBefore, err = parseSchedule(options.At, now)
		if err != nil {
			return usageError("send: --at: %v", err)
		}
	}
	id, err := randomID("msg")
	if err != nil {
		return err
	}
	state, err = run.drive(core.SendRequested{
		At: now, Deadline: now.Add(deliveryTimeout), NotBefore: notBefore,
		Envelope: core.Envelope{ID: core.EnvelopeID(id), From: sender, To: core.AgentName(options.Name), Message: core.Message{Text: body}, CreatedAt: now},
	})
	if err != nil {
		return err
	}
	delivery, ok := state.Deliveries[core.EnvelopeID(id)]
	if !ok {
		return refuseError("send was rejected; inspect 'gang log'")
	}
	if delivery.Status == core.DeliveryUnverified {
		return commandError{status: exitUnknown, text: "delivery may have landed but could not be verified: " + delivery.Reason}
	}
	if delivery.Status == core.DeliveryFailed {
		return refuseError("delivery failed: %s", delivery.Reason)
	}
	if delivery.Status == core.DeliveryQueued {
		_, err = fmt.Fprintf(cmd.stdout, "%s\tqueued\n", id)
		return err
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s\t%s\n", id, delivery.Status)
	return err
}

func (cmd command) sender(state core.State, declared string) (core.Sender, error) {
	if pane := cmd.environment("TMUX_PANE"); pane != "" {
		for _, hitch := range state.Hitches {
			if hitch.Pane == pane && hitch.Status == core.HitchActive {
				if declared != "" && declared != string(hitch.Name) {
					return core.Sender{}, refuseError("--from cannot override observed pane identity %q", hitch.Name)
				}
				return core.Sender{Kind: core.SenderAgent, Name: hitch.Name, HitchID: hitch.ID}, nil
			}
		}
	}
	if declared == "" {
		return core.Sender{}, refuseError("sender is outside the team; provide --from NAME")
	}
	return core.Sender{Kind: core.SenderSelfDeclared, Name: core.AgentName(declared)}, nil
}

func (cmd command) interrupt(arguments []string) error {
	name := ""
	if len(arguments) != 0 && !strings.HasPrefix(arguments[0], "-") {
		name, arguments = arguments[0], arguments[1:]
	}
	reason := ""
	from := ""
	flags := quietFlagSet("interrupt")
	flags.StringVar(&reason, "m", "", "reason")
	flags.StringVar(&from, "from", "", "sender")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return usageError("interrupt: invalid arguments")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	if name == "" {
		name = string(state.Team.Name)
		if pane := cmd.environment("TMUX_PANE"); pane != "" {
			for _, hitch := range state.Hitches {
				if hitch.Pane == pane {
					name = string(hitch.Name)
					break
				}
			}
		}
	}
	_ = from
	hitch, ok := activeByName(state, name)
	if !ok {
		return refuseError("agent %q is not active", name)
	}
	if hitch.Activity != core.ActivityBusy && hitch.Activity != core.ActivityWedged {
		return refuseError("agent %q is not in an interruptible turn", name)
	}
	now := time.Now()
	_, err = run.drive(core.InterruptRequested{At: now, HitchID: hitch.ID, Reason: reason, Deadline: now.Add(operationTimeout)})
	return err
}

func (cmd command) compact(arguments []string) error {
	options, err := parseCompact(arguments)
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
	name := options.Name
	if name == "" {
		pane := cmd.environment("TMUX_PANE")
		for _, hitch := range state.Hitches {
			if hitch.Pane == pane {
				name = string(hitch.Name)
				break
			}
		}
	}
	hitch, ok := activeByName(state, name)
	if !ok {
		return refuseError("agent %q is not active", name)
	}
	if options.Recover {
		collar, err := loadCollar(hitch.Collar, run.settings)
		if err != nil {
			return err
		}
		backend, err := cmd.tmux(run.settings)
		if err != nil {
			return err
		}
		for _, action := range collar.Actions.CompactRecover {
			if err := backend.SendKeys(context.Background(), substrate.PaneID(hitch.Pane), action.Input()); err != nil {
				return err
			}
		}
		if hitch.PendingCompactID != "" {
			_, err = run.drive(core.CompactionFailedEvent{At: time.Now(), CompactionID: hitch.PendingCompactID, Reason: "operator requested native compaction recovery"})
		}
		return err
	}
	resume := options.Resume
	if resume == "" {
		resume = "Your context was just compacted. Re-read your brief and durable state, then resume your lane or report it complete."
	}
	id, err := randomID("compact")
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = run.drive(core.CompactionRequested{At: now, Compaction: core.Compaction{ID: core.CompactionID(id), HitchID: hitch.ID, Resume: core.Message{Text: resume}, Deadline: now.Add(operationTimeout)}})
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

func (cmd command) down(arguments []string) error {
	if err := exactly(arguments, 1, "down"); err != nil {
		return err
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	if arguments[0] != settings.Session {
		return refuseError("down requires the configured session name %q", settings.Session)
	}
	run := &runtime{cmd: cmd, settings: settings}
	state, err := run.load()
	if err != nil {
		return err
	}
	names := make([]string, 0)
	for _, hitch := range state.Hitches {
		if hitch.Status == core.HitchActive {
			names = append(names, string(hitch.Name))
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if err := cmd.drop([]string{name}); err != nil {
			return err
		}
	}
	paths, err := run.paths().Team(settings.Session)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(paths.Directory); err != nil {
		return fmt.Errorf("remove stopped team state: %w", err)
	}
	return nil
}

func (cmd command) tick(arguments []string) error {
	if err := noArguments(arguments, "tick"); err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.recover()
	if err != nil {
		return err
	}
	return cmd.observeWedges(run, state)
}

func (cmd command) hook(arguments []string) error {
	if err := noArguments(arguments, "hook"); err != nil {
		return err
	}
	id := core.HitchID(cmd.environment("GANGLINE_HITCH_ID"))
	if id == "" {
		return refuseError("hook has no GANGLINE_HITCH_ID")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	hitch, ok := state.Hitches[id]
	if !ok {
		return refuseError("hook hitch %q is not registered", id)
	}
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return err
	}
	payload, err := io.ReadAll(io.LimitReader(cmd.stdin, maximumMessageBytes+1))
	if err != nil {
		return err
	}
	boundary, hookEvent, err := harness.DetectTurnBoundary(collar, payload)
	if err != nil {
		return err
	}
	if boundary == harness.TurnStarted && hookEvent.Payload["prompt"] != "" {
		witness := run.deliveryWitnessPath(id)
		file, openErr := os.OpenFile(witness, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if openErr == nil {
			_, writeErr := io.WriteString(file, hookEvent.Payload["prompt"])
			closeErr := file.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	now := time.Now()
	switch boundary {
	case harness.TurnStarted:
		if hitch.Activity == core.ActivityIdle {
			_, err = run.drive(core.TurnStarted{At: now, HitchID: id})
		}
	case harness.TurnFinished:
		_, err = run.drive(core.TurnBoundaryReached{At: now, HitchID: id})
	case harness.TurnCompactionFinished:
		if hitch.PendingCompactID != "" {
			compact := state.Compactions[hitch.PendingCompactID]
			_, err = run.drive(core.CompactionCompleted{At: now, CompactionID: hitch.PendingCompactID})
			if err == nil {
				id, idErr := randomID("resume")
				if idErr != nil {
					return idErr
				}
				_, err = run.drive(core.SendRequested{
					At: now, Deadline: now.Add(deliveryTimeout),
					Envelope: core.Envelope{
						ID: core.EnvelopeID(id), From: core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"},
						To: hitch.Name, Message: compact.Resume, CreatedAt: now,
					},
				})
			}
		}
	case harness.TurnCompactionStarted:
		// The request event already records the durable start intent.
	default:
		// Activity and permission hooks are valid evidence but not boundaries.
	}
	return err
}

func (cmd command) hostRun(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "--" || len(arguments) == 1 {
		return usageError("run: expected -- COMMAND [ARG ...]")
	}
	process := exec.Command(arguments[1], arguments[2:]...)
	process.Stdin, process.Stdout, process.Stderr = cmd.stdin, cmd.stdout, cmd.stderr
	if err := process.Run(); err != nil {
		return commandError{status: exitNative, text: err.Error()}
	}
	return nil
}

func (cmd command) flush(arguments []string) error {
	if len(arguments) > 1 {
		return usageError("flush: expected at most one agent")
	}
	return cmd.tick(nil)
}

func (cmd command) queue(arguments []string) error {
	if len(arguments) > 1 {
		return usageError("queue: expected at most one agent")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	name := ""
	if len(arguments) == 1 {
		name = arguments[0]
	}
	for _, id := range state.DeliveryOrder {
		delivery := state.Deliveries[id]
		if delivery.Status != core.DeliveryQueued || name != "" && string(delivery.Envelope.To) != name {
			continue
		}
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", id, delivery.Envelope.To, delivery.Envelope.From.Name); err != nil {
			return err
		}
	}
	return nil
}

func (cmd command) log(arguments []string) error {
	if len(arguments) != 0 {
		return usageError("log filters are not available in the v1 event log; use gang log")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		return err
	}
	file, err := os.Open(paths.Events)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(cmd.stdout, file)
	return err
}

func (cmd command) replay(arguments []string) error {
	if len(arguments) > 1 {
		return usageError("replay: expected at most one event log")
	}
	reader := cmd.stdin
	if len(arguments) == 1 {
		file, err := os.Open(arguments[0])
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	entries, err := store.ReadLog(reader)
	if err != nil {
		return err
	}
	state := store.Replay(core.NewState(core.Team{ID: "replay", Name: "replay"}), entries)
	encoder := json.NewEncoder(cmd.stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(state)
}

func (cmd command) curfew(arguments []string) error {
	if len(arguments) > 1 {
		return usageError("curfew: expected duration, HH:MM, clear, or no argument")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	if len(arguments) == 0 {
		if state.Team.Curfew.IsZero() {
			_, err = fmt.Fprintln(cmd.stdout, "clear")
		} else {
			_, err = fmt.Fprintln(cmd.stdout, state.Team.Curfew.Format(time.RFC3339))
		}
		return err
	}
	if arguments[0] == "clear" {
		if state.Team.Curfew.IsZero() {
			return refuseError("team curfew is already clear")
		}
		_, err = run.drive(core.CurfewCleared{At: time.Now()})
		return err
	}
	now := time.Now()
	deadline, err := parseSchedule(arguments[0], now)
	if err != nil {
		return usageError("curfew: %v", err)
	}
	_, err = run.drive(core.CurfewSet{At: now, Deadline: deadline})
	return err
}

func (cmd command) whoami(arguments []string) error {
	if err := noArguments(arguments, "whoami"); err != nil {
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
	pane := cmd.environment("TMUX_PANE")
	for _, hitch := range state.Hitches {
		if hitch.Pane == pane && hitch.Status == core.HitchActive {
			_, err = fmt.Fprintln(cmd.stdout, hitch.Name)
			return err
		}
	}
	return refuseError("current pane is not a registered active agent")
}

func (cmd command) roster(arguments []string) error {
	porcelain := false
	flags := quietFlagSet("roster")
	flags.BoolVar(&porcelain, "porcelain", false, "machine format")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return usageError("roster: invalid arguments")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	hitches := make([]core.Hitch, 0, len(state.Hitches))
	for _, hitch := range state.Hitches {
		hitches = append(hitches, hitch)
	}
	sort.Slice(hitches, func(i, j int) bool { return hitches[i].Name < hitches[j].Name })
	for _, hitch := range hitches {
		if hitch.Status == core.HitchDropped {
			continue
		}
		if porcelain {
			_, err = fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\t%s\t%s\n", hitch.Name, hitch.Status, hitch.Activity, hitch.Collar, hitch.Pane)
		} else {
			_, err = fmt.Fprintf(cmd.stdout, "%-16s %-10s %-14s %s\n", hitch.Name, hitch.Status, hitch.Activity, hitch.Collar)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (cmd command) status(arguments []string) error {
	why := false
	name := ""
	flags := quietFlagSet("status")
	flags.BoolVar(&why, "why", false, "include evidence")
	if len(arguments) != 0 && !strings.HasPrefix(arguments[0], "-") {
		name, arguments = arguments[0], arguments[1:]
	}
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return usageError("status: invalid arguments")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	if name == "" {
		pane := cmd.environment("TMUX_PANE")
		for _, hitch := range state.Hitches {
			if hitch.Pane == pane {
				name = string(hitch.Name)
				break
			}
		}
	}
	hitch, ok := hitchByName(state, name)
	if !ok {
		return refuseError("agent %q is not registered", name)
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", hitch.Name, hitch.Status, hitch.Activity)
	if err == nil && why && hitch.WedgeEvidence != "" {
		_, err = fmt.Fprintln(cmd.stdout, hitch.WedgeEvidence)
	}
	return err
}

// v1 intentionally has no separate local accounting database. These commands
// return explicit unknowns instead of inventing totals from incomplete data.
func (cmd command) usage(arguments []string) error {
	if len(arguments) != 0 {
		return usageError("usage: filters are not available")
	}
	_, err := fmt.Fprintln(cmd.stdout, "unknown\tno local token-accounting samples")
	return err
}
func (cmd command) cap(arguments []string) error {
	if len(arguments) > 1 {
		return usageError("cap: invalid arguments")
	}
	_, err := fmt.Fprintln(cmd.stdout, "unknown\tno retained provider-window samples")
	return err
}

func (cmd command) wait(arguments []string) error {
	if len(arguments) < 1 {
		return usageError("wait: agent name required")
	}
	// Waiting is event-driven for callers: a non-idle observation refuses now;
	// callers can invoke again after their own hook/event barrier.
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
	if hitch.Activity != core.ActivityIdle {
		return refuseError("agent %q has not reached an idle boundary", arguments[0])
	}
	return nil
}
