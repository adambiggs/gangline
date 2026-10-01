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
	options := hitchOptions{Collar: defaultCollar, Directory: defaultDirectory}
	flags := boundFlagSet("hitch", map[string]any{
		"c": &options.Collar, "collar": &options.Collar,
		"d": &options.Directory, "dir": &options.Directory,
		"m": &options.Model, "model": &options.Model,
		"e": &options.Effort, "effort": &options.Effort,
		"t": &options.Task, "task": &options.Task,
		"r": &options.Role, "role": &options.Role,
		"resume": &options.Resume, "recover": &options.Recover, "stdin": &options.Stdin,
	})
	positionals, err := parseOptions(flags, arguments)
	if err != nil {
		return hitchOptions{}, usageError("hitch: %v", err)
	}
	if len(positionals) == 0 {
		return hitchOptions{}, usageError("hitch: agent name required")
	}
	if len(positionals) > 1 {
		return hitchOptions{}, usageError("hitch: unexpected argument %q", positionals[1])
	}
	options.Name = positionals[0]
	if err := validateAgentName(options.Name); err != nil {
		return hitchOptions{}, err
	}
	if options.Collar == "" || options.Directory == "" {
		return hitchOptions{}, usageError("hitch: collar and directory must not be empty")
	}
	if options.Recover && !onlyFlagSet(flags, "recover") {
		return hitchOptions{}, usageError("hitch: --recover takes only NAME")
	}
	return options, nil
}

type sendOptions struct {
	Name      string
	Body      *string
	From      string
	LiveOnly  bool
	Supersede bool
	At        string
	Clear     bool
}

type waitOptions struct {
	Name    string
	Timeout time.Duration
}

func parseWait(arguments []string) (waitOptions, error) {
	options := waitOptions{Timeout: operationTimeout}
	flags := boundFlagSet("wait", map[string]any{"timeout": &options.Timeout})
	positionals, err := parseOptions(flags, arguments)
	if err != nil {
		return waitOptions{}, usageError("wait: %v", err)
	}
	if len(positionals) == 0 {
		return waitOptions{}, usageError("wait: agent name required")
	}
	if len(positionals) > 1 {
		return waitOptions{}, usageError("wait: unexpected argument %q", positionals[1])
	}
	options.Name = positionals[0]
	if err := validateAgentName(options.Name); err != nil {
		return waitOptions{}, err
	}
	if options.Timeout < 0 {
		return waitOptions{}, usageError("wait: --timeout must not be negative")
	}
	return options, nil
}

func parseSend(arguments []string) (sendOptions, error) {
	options := sendOptions{}
	flags := boundFlagSet("send", map[string]any{
		"from": &options.From, "live-only": &options.LiveOnly,
		"supersede": &options.Supersede, "at": &options.At, "clear": &options.Clear,
	})
	positionals, err := parseOptions(flags, arguments)
	if err != nil {
		return sendOptions{}, usageError("send: %v", err)
	}
	if len(positionals) == 0 {
		return sendOptions{}, usageError("send: recipient required")
	}
	if len(positionals) > 2 {
		return sendOptions{}, usageError("send: unexpected argument %q", positionals[2])
	}
	options.Name = positionals[0]
	if len(positionals) == 2 {
		body := positionals[1]
		options.Body = &body
	}
	if options.Clear && (options.Body != nil || options.At != "" || options.LiveOnly) {
		return sendOptions{}, usageError("send: --clear takes no body, --at, or --live-only")
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
	flags := boundFlagSet("compact", map[string]any{
		"resume": &options.Resume, "recover": &options.Recover,
	})
	positionals, err := parseOptions(flags, arguments)
	if err != nil {
		return compactOptions{}, usageError("compact: %v", err)
	}
	if len(positionals) > 1 {
		return compactOptions{}, usageError("compact: unexpected argument %q", positionals[1])
	}
	if len(positionals) == 1 {
		options.Name = positionals[0]
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

// parseOptions accepts flags on either side of operands and preserves -- as
// the point after which every token is an operand.
func parseOptions(flags *flag.FlagSet, arguments []string) ([]string, error) {
	flagArguments, positionals := partitionOptions(flags, arguments)
	if err := flags.Parse(flagArguments); err != nil {
		return nil, err
	}
	return positionals, nil
}

func partitionOptions(flags *flag.FlagSet, arguments []string) ([]string, []string) {
	flagArguments, positionals, _ := splitOptions(flags, arguments)
	return flagArguments, positionals
}

// splitOptions separates flags from operands before --, reports whether -h or
// -help appeared in flag position with either dash count, and splits an
// attached one-letter value (-cVALUE) into the flag and its value. A token
// whose text before = looks like a flag name (-name=VALUE) stays whole so a
// mistyped long flag is reported.
func splitOptions(flags *flag.FlagSet, arguments []string) (flagArguments, positionals []string, help bool) {
	for _, word := range scanOptions(flags, arguments) {
		switch {
		case word.terminator:
		case word.option == "":
			positionals = append(positionals, word.tokens...)
		default:
			help = help || word.option == "h" || word.option == "help"
			flagArguments = append(flagArguments, word.tokens...)
		}
	}
	return flagArguments, positionals, help
}

// optionWord is one flag with its value tokens, one operand, or the --
// terminator. An operand has no option name.
type optionWord struct {
	tokens     []string
	option     string
	terminator bool
}

func scanOptions(flags *flag.FlagSet, arguments []string) []optionWord {
	var words []optionWord
	ended := false
	for i := 0; i < len(arguments); i++ {
		argument := arguments[i]
		if !ended && argument == "--" {
			ended = true
			words = append(words, optionWord{tokens: []string{argument}, terminator: true})
			continue
		}
		if ended || !strings.HasPrefix(argument, "-") || argument == "-" {
			words = append(words, optionWord{tokens: []string{argument}})
			continue
		}
		name, _, assigned := strings.Cut(strings.TrimLeft(argument, "-"), "=")
		option := flags.Lookup(name)
		if option == nil && !strings.HasPrefix(argument, "--") && len(argument) > 2 && (!assigned || len(name) == 1 || !flagNamePattern.MatchString(name)) {
			if short := flags.Lookup(argument[1:2]); short != nil && !isBoolFlag(short) {
				words = append(words, optionWord{tokens: []string{argument[:2], argument[2:]}, option: short.Name})
				continue
			}
		}
		word := optionWord{tokens: []string{argument}, option: name}
		if option != nil && !assigned && !isBoolFlag(option) && i+1 < len(arguments) {
			i++
			word.tokens = append(word.tokens, arguments[i])
		}
		words = append(words, word)
	}
	return words
}

// teamOption is the selector shared by every command in teamCommands. It is
// taken from the arguments before the command parses its own options.
var teamOption = optionSpec{"team", "TEAM", "team to act on (default GANG_SESSION)"}

// takeTeam removes --team and its value from a team command's arguments,
// classifying tokens with that command's own options so an option value or an
// operand after -- is never taken as the selector, and ---team is left for the
// command to refuse like any other three-dash option. The last --team wins.
func takeTeam(name string, arguments []string) (team string, rest []string, err error) {
	if !teamCommands[name] {
		return "", arguments, nil
	}
	selected := false
	for _, word := range scanOptions(specFlagSet(name), arguments) {
		if word.option != teamOption.name || strings.HasPrefix(word.tokens[0], "---") {
			rest = append(rest, word.tokens...)
			continue
		}
		value, assigned := "", false
		if _, after, ok := strings.Cut(word.tokens[0], "="); ok {
			value, assigned = after, true
		} else if len(word.tokens) == 2 {
			value, assigned = word.tokens[1], true
		}
		if !assigned {
			return "", nil, usageError("%s: flag needs an argument: --team", name)
		}
		team, selected = value, true
	}
	if selected && strings.TrimSpace(team) == "" {
		return "", nil, usageError("%s: --team must not be blank", name)
	}
	return team, rest, nil
}

var flagNamePattern = regexp.MustCompile(`^[a-z][a-z-]*$`)

func isBoolFlag(option *flag.Flag) bool {
	boolean, ok := option.Value.(interface{ IsBoolFlag() bool })
	return ok && boolean.IsBoolFlag()
}

// specFlagSet registers a command's options without bindings, so arguments can
// be classified without running the command.
func specFlagSet(name string) *flag.FlagSet {
	flags := quietFlagSet(name)
	for _, option := range helpOptions(name) {
		if option.argument == "" {
			flags.Bool(option.name, false, option.meaning)
		} else {
			flags.String(option.name, "", option.meaning)
		}
	}
	return flags
}

func boundFlagSet(name string, bindings map[string]any) *flag.FlagSet {
	flags := quietFlagSet(name)
	for _, option := range optionsFor(name) {
		binding, ok := bindings[option.name]
		if !ok {
			panic("missing flag binding: " + name + " " + option.name)
		}
		switch value := binding.(type) {
		case *string:
			if option.argument == "" {
				panic("missing flag argument: " + name + " " + option.name)
			}
			flags.StringVar(value, option.name, *value, option.meaning)
		case *bool:
			if option.argument != "" {
				panic("unexpected flag argument: " + name + " " + option.name)
			}
			flags.BoolVar(value, option.name, *value, option.meaning)
		case *time.Duration:
			if option.argument == "" {
				panic("missing flag argument: " + name + " " + option.name)
			}
			flags.DurationVar(value, option.name, *value, option.meaning)
		default:
			panic("unsupported flag binding: " + name + " " + option.name)
		}
	}
	if len(bindings) != len(optionsFor(name)) {
		panic("extra flag binding: " + name)
	}
	return flags
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	set := false
	flags.Visit(func(option *flag.Flag) {
		if option.Name == name {
			set = true
		}
	})
	return set
}

func onlyFlagSet(flags *flag.FlagSet, name string) bool {
	only := true
	flags.Visit(func(option *flag.Flag) {
		only = only && option.Name == name
	})
	return only
}

func parseCollarFlags(commandName string, arguments []string, defaultCollar string) (string, error) {
	collar := defaultCollar
	flags := boundFlagSet(commandName, map[string]any{"c": &collar, "collar": &collar})
	positionals, err := parseOptions(flags, arguments)
	if err != nil {
		return "", err
	}
	if len(positionals) != 0 {
		return "", usageError("unexpected argument %q", positionals[0])
	}
	return collar, nil
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
