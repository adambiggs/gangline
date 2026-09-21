package main

import (
	"errors"
	"fmt"
	"io"
	"os"
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
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
	getenv        func(string) string
	lookupEnv     func(string) (string, bool)
	getwd         func() (string, error)
	userHomeDir   func() (string, error)
	newAppendWait func(string, int64) (appendWait, error)
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

func (cmd command) execute(args []string) error {
	if len(args) == 0 {
		_, err := io.WriteString(cmd.stdout, welcomeHelp)
		return err
	}
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		_, err := fmt.Fprintf(cmd.stdout, "gangline %s\n", version)
		return err
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		if len(args) > 2 {
			return usageError("help: expected at most one command")
		}
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		return cmd.printHelp(name)
	}
	if len(args) == 2 && args[1] == "--help" {
		return cmd.printHelp(args[0])
	}

	name, arguments := args[0], args[1:]
	switch name {
	case "up":
		return cmd.up(arguments)
	case "hitch":
		return cmd.hitch(arguments)
	case "adopt":
		return cmd.adopt(arguments)
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
	case "context":
		return cmd.context(arguments)
	case "log":
		return cmd.log(arguments)
	case "replay":
		return cmd.replay(arguments)
	case "limits":
		return cmd.limits(arguments)
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
