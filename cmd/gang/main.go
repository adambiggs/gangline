package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

var version = "dev"

const (
	exitOK      = 0
	exitError   = 1
	exitUsage   = 2
	exitRefused = 3
	exitNative  = 4
	exitUnknown = 5
)

type commandError struct {
	status int
	text   string
}

func (err commandError) Error() string { return err.text }

func usageError(format string, arguments ...any) error {
	return commandError{status: exitUsage, text: fmt.Sprintf(format, arguments...)}
}

func refuseError(format string, arguments ...any) error {
	return commandError{status: exitRefused, text: fmt.Sprintf(format, arguments...)}
}

type command struct {
	newScheduler  func() watchdogScheduler
	stdin         io.Reader
	terminalInput func() bool
	stdout        io.Writer
	stderr        io.Writer
	team          string
	getenv        func(string) string
	lookupEnv     func(string) (string, bool)
	getwd         func() (string, error)
	userHomeDir   func() (string, error)
	clock         func() time.Time
	newWatch      func(string) (changeWait, error)
	newTimeout    func(context.Context, time.Duration) (context.Context, context.CancelFunc)
	paneBackend   paneRegistry
	inputBackend  harnessInput
	settleInput   func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error
	awaitStartup  func(context.Context, substrate.PaneID, harness.Collar) (harness.Startup, substrate.Screen, error)
	detach        func(string, hookNotice) error
	afterUnlock   func()
	// schedulerLockWait runs when cleanup finds the watchdog scheduler lock
	// held, before it waits for the lock.
	schedulerLockWait func()
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd := command{
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		getenv:      os.Getenv,
		lookupEnv:   os.LookupEnv,
		getwd:       os.Getwd,
		userHomeDir: os.UserHomeDir,
	}
	args = versionCommand(args)
	if err := cmd.execute(args); err != nil {
		var commandErr commandError
		if errors.As(err, &commandErr) {
			fmt.Fprintf(stderr, "gang: %s\n", commandErr.text)
			if commandErr.status == exitUsage && len(args) != 0 {
				if usage, ok := commandUsage[args[0]]; ok {
					fmt.Fprint(stderr, usage)
				}
			}
			return commandErr.status
		}
		fmt.Fprintf(stderr, "gang: %v\n", err)
		return exitError
	}
	return exitOK
}

// versionCommand reads --version as the version command's spelling as a flag,
// so help and usage errors resolve it as that command in every position help
// accepts a command name.
func versionCommand(args []string) []string {
	if len(args) == 0 {
		return args
	}
	if args[0] == "--version" {
		return append([]string{"version"}, args[1:]...)
	}
	if args[0] != "help" && args[0] != "--help" && args[0] != "-h" {
		return args
	}
	at := 1
	if args[0] == "help" && len(args) > 2 && args[1] == "--" {
		at = 2
	}
	if len(args) > at && args[at] == "--version" {
		args = append([]string{}, args...)
		args[at] = "version"
	}
	return args
}

func (cmd command) execute(args []string) error {
	if len(args) == 0 {
		_, err := io.WriteString(cmd.stdout, welcomeHelp)
		return err
	}
	if args[0] == "help" && len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		if _, ok := commandUsage[args[1]]; !ok {
			return cmd.printHelp(args[1])
		}
	}
	if args[0] == "help" && helpRequested("help", args[1:]) {
		return cmd.printHelp("help")
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		if args[0] == "help" && len(args) > 1 && args[1] == "--" {
			args = append(args[:1:1], args[2:]...)
		}
		if len(args) == 3 && args[1] == "collar" && args[2] == "check" {
			return cmd.printHelp("collar")
		}
		if len(args) > 2 {
			if args[0] == "help" {
				if _, ok := commandUsage[args[1]]; !ok {
					return cmd.printHelp(args[1])
				}
			}
			return usageError("help: expected at most one command")
		}
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		return cmd.printHelp(name)
	}
	if helpRequested(args[0], args[1:]) {
		return cmd.printHelp(args[0])
	}

	name, arguments := args[0], args[1:]
	team, arguments, err := takeTeam(name, arguments)
	if err != nil {
		return err
	}
	cmd.team = team
	if len(optionsFor(name)) == 0 {
		for i, argument := range arguments {
			if argument == "--" {
				arguments = append(append([]string{}, arguments[:i]...), arguments[i+1:]...)
				break
			}
		}
	}
	switch name {
	case "version":
		if err := noArguments(arguments, "version"); err != nil {
			return err
		}
		_, err := fmt.Fprintf(cmd.stdout, "gangline %s\n", version)
		return err
	case "up":
		return cmd.up(arguments)
	case "hitch":
		return cmd.hitch(arguments)
	case "rename":
		return cmd.rename(arguments)
	case "send":
		return cmd.send(arguments)
	case "queue":
		return cmd.queue(arguments)
	case "interrupt":
		return cmd.interrupt(arguments)
	case "compact":
		return cmd.compact(arguments)
	case "statusline":
		return cmd.statusline(arguments)
	case "context":
		return cmd.context(arguments)
	case "log":
		return cmd.log(arguments)
	case "limits":
		return cmd.limits(arguments)
	case "snooze":
		return cmd.snooze(arguments)
	case "wait":
		return cmd.wait(arguments)
	case "curfew":
		return cmd.curfew(arguments)
	case "status":
		return cmd.status(arguments)
	case "tick":
		return cmd.tick(arguments)
	case "hook":
		return cmd.hook(arguments)
	case "capture":
		return cmd.capture(arguments)
	case "whoami":
		return cmd.whoami(arguments)
	case "roster":
		return cmd.roster(arguments)
	case "attach":
		return cmd.attach(arguments)
	case "teams":
		return cmd.teams(arguments)
	case "drop":
		return cmd.drop(arguments)
	case "down":
		return cmd.down(arguments)
	case "collars":
		return cmd.collars(arguments)
	case "collar":
		return cmd.collar(arguments)
	case "models":
		return cmd.models(arguments)
	case "roles":
		return cmd.roles(arguments)
	case "config":
		return cmd.config(arguments)
	case "upgrade":
		return cmd.upgrade(arguments)
	default:
		return usageError("unknown command %q (run 'gang help')", name)
	}
}

func helpRequested(name string, arguments []string) bool {
	_, _, help := splitOptions(specFlagSet(name), arguments)
	return help
}
