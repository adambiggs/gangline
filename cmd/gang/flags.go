package main

import (
	"flag"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

var agentNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type hitchOptions struct {
	Name      string
	Collar    string
	Directory string
	Model     string
	Effort    string
	Task      string
	Role      string
	Resume    string
	Recover   bool
	Stdin     bool
}

func parseHitch(arguments []string, defaultCollar, defaultDirectory string) (hitchOptions, error) {
	if len(arguments) == 0 || strings.HasPrefix(arguments[0], "-") {
		return hitchOptions{}, usageError("hitch: agent name required")
	}
	options := hitchOptions{Name: arguments[0], Collar: defaultCollar, Directory: defaultDirectory}
	flags := quietFlagSet("hitch")
	flags.StringVar(&options.Collar, "c", options.Collar, "harness collar")
	flags.StringVar(&options.Collar, "collar", options.Collar, "harness collar")
	flags.StringVar(&options.Directory, "d", options.Directory, "working directory")
	flags.StringVar(&options.Directory, "dir", options.Directory, "working directory")
	flags.StringVar(&options.Model, "m", "", "native model")
	flags.StringVar(&options.Model, "model", "", "native model")
	flags.StringVar(&options.Effort, "e", "", "reasoning effort")
	flags.StringVar(&options.Effort, "effort", "", "reasoning effort")
	flags.StringVar(&options.Task, "t", "", "task label")
	flags.StringVar(&options.Task, "task", "", "task label")
	flags.StringVar(&options.Role, "r", "", "role brief")
	flags.StringVar(&options.Role, "role", "", "role brief")
	flags.StringVar(&options.Resume, "resume", "", "native session id")
	flags.BoolVar(&options.Recover, "recover", false, "recover the original startup message")
	flags.BoolVar(&options.Stdin, "stdin", false, "read assignment from stdin")
	if err := flags.Parse(arguments[1:]); err != nil {
		return hitchOptions{}, usageError("hitch: %v", err)
	}
	if flags.NArg() != 0 {
		return hitchOptions{}, usageError("hitch: unexpected argument %q", flags.Arg(0))
	}
	if err := validateAgentName(options.Name); err != nil {
		return hitchOptions{}, err
	}
	if options.Collar == "" || options.Directory == "" {
		return hitchOptions{}, usageError("hitch: collar and directory must not be empty")
	}
	if options.Recover && len(arguments) != 2 {
		return hitchOptions{}, usageError("hitch: --recover takes only NAME")
	}
	return options, nil
}

type sendOptions struct {
	Name      string
	From      string
	LiveOnly  bool
	Supersede bool
	At        string
}

type waitOptions struct {
	Name    string
	Timeout time.Duration
}

func parseWait(arguments []string) (waitOptions, error) {
	if len(arguments) == 0 || strings.HasPrefix(arguments[0], "-") {
		return waitOptions{}, usageError("wait: agent name required")
	}
	options := waitOptions{Name: arguments[0], Timeout: operationTimeout}
	flags := quietFlagSet("wait")
	flags.DurationVar(&options.Timeout, "timeout", options.Timeout, "maximum wait")
	if err := flags.Parse(arguments[1:]); err != nil {
		return waitOptions{}, usageError("wait: %v", err)
	}
	if flags.NArg() != 0 {
		return waitOptions{}, usageError("wait: unexpected argument %q", flags.Arg(0))
	}
	if err := validateAgentName(options.Name); err != nil {
		return waitOptions{}, err
	}
	if options.Timeout < 0 {
		return waitOptions{}, usageError("wait: --timeout must not be negative")
	}
	return options, nil
}

func parseSend(arguments []string) (sendOptions, error) {
	if len(arguments) == 0 || strings.HasPrefix(arguments[0], "-") {
		return sendOptions{}, usageError("send: recipient required")
	}
	options := sendOptions{Name: arguments[0]}
	flags := quietFlagSet("send")
	flags.StringVar(&options.From, "from", "", "outside sender identity")
	flags.BoolVar(&options.LiveOnly, "live-only", false, "refuse rather than queue")
	flags.BoolVar(&options.Supersede, "supersede", false, "replace older queued work")
	flags.StringVar(&options.At, "at", "", "delivery time")
	if err := flags.Parse(arguments[1:]); err != nil {
		return sendOptions{}, usageError("send: %v", err)
	}
	if flags.NArg() != 0 {
		return sendOptions{}, usageError("send: unexpected argument %q", flags.Arg(0))
	}
	if err := validateAgentName(options.Name); err != nil {
		return sendOptions{}, err
	}
	if options.From != "" {
		if err := validateAgentName(options.From); err != nil {
			return sendOptions{}, usageError("send: invalid --from identity: %v", err)
		}
	}
	if options.LiveOnly && options.At != "" {
		return sendOptions{}, usageError("send: --live-only and --at cannot be combined")
	}
	return options, nil
}

type compactOptions struct {
	Name    string
	Resume  string
	Recover bool
}

func parseCompact(arguments []string) (compactOptions, error) {
	options := compactOptions{}
	if len(arguments) != 0 && !strings.HasPrefix(arguments[0], "-") {
		options.Name = arguments[0]
		arguments = arguments[1:]
	}
	flags := quietFlagSet("compact")
	flags.StringVar(&options.Resume, "resume", "", "continuation turn")
	flags.BoolVar(&options.Recover, "recover", false, "recover a stuck compaction")
	if err := flags.Parse(arguments); err != nil {
		return compactOptions{}, usageError("compact: %v", err)
	}
	if flags.NArg() != 0 {
		return compactOptions{}, usageError("compact: unexpected argument %q", flags.Arg(0))
	}
	if options.Name != "" {
		if err := validateAgentName(options.Name); err != nil {
			return compactOptions{}, err
		}
	}
	if options.Recover && options.Resume != "" {
		return compactOptions{}, usageError("compact: --recover and --resume cannot be combined")
	}
	return options, nil
}

func quietFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func validateAgentName(name string) error {
	if !agentNamePattern.MatchString(name) || name == "hitch" || name == "gangline" {
		return usageError("invalid agent name %q", name)
	}
	return nil
}

func exactly(arguments []string, count int, name string) error {
	if len(arguments) == count {
		return nil
	}
	return usageError("%s: expected %d argument%s", name, count, plural(count))
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func noArguments(arguments []string, name string) error {
	if len(arguments) == 0 {
		return nil
	}
	return usageError("%s: unexpected argument %q", name, arguments[0])
}

func requiredName(arguments []string, commandName string) (string, error) {
	if err := exactly(arguments, 1, commandName); err != nil {
		return "", err
	}
	if err := validateAgentName(arguments[0]); err != nil {
		return "", fmt.Errorf("%s: %w", commandName, err)
	}
	return arguments[0], nil
}
