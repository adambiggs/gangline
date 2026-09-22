package harness

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

type LaunchOptions struct {
	ResumeSession      string
	HookCommand        []string
	HookTimeoutSeconds int
	Model              string
	Effort             string
	RolePrompt         string
	Probe              bool
}

type Command struct {
	Name string
	Args []string
	Env  map[string]string
}

func (command Command) SpawnSpec(name, directory string) substrate.SpawnSpec {
	env := make(map[string]string, len(command.Env))
	for key, value := range command.Env {
		env[key] = value
	}
	return substrate.SpawnSpec{
		Name: name, Directory: directory, Command: command.Name,
		Args: append([]string(nil), command.Args...), Env: env,
	}
}

func RenderLaunch(collar Collar, options LaunchOptions) (Command, error) {
	args := collar.Launch.Args
	if options.ResumeSession != "" {
		if len(collar.Launch.ResumeArgs) == 0 {
			return Command{}, fmt.Errorf("collar %q does not declare resume arguments", collar.Name)
		}
		args = collar.Launch.ResumeArgs
	}

	values := map[string]string{"session_id": options.ResumeSession}
	rendered, err := renderArgs(args, values)
	if err != nil {
		return Command{}, fmt.Errorf("render %s launch: %w", collar.Name, err)
	}

	if collar.Hooks != nil && len(options.HookCommand) == 0 {
		return Command{}, fmt.Errorf("collar %q requires a hook command", collar.Name)
	}
	if collar.Hooks != nil {
		timeout := options.HookTimeoutSeconds
		if timeout <= 0 {
			timeout = 600
		}
		command := shellJoin(options.HookCommand)
		encoded, err := json.Marshal(command)
		if err != nil {
			return Command{}, fmt.Errorf("encode hook command: %w", err)
		}
		hookArgs, err := renderArgs(collar.Hooks.InstallArgs, map[string]string{
			"hook.command.json": string(encoded),
			"hook.timeout":      strconv.Itoa(timeout),
		})
		if err != nil {
			return Command{}, fmt.Errorf("render %s hooks: %w", collar.Name, err)
		}
		rendered = append(rendered, hookArgs...)
	}
	if options.Probe {
		rendered = append(rendered, collar.Launch.ProbeArgs...)
	}

	for _, option := range []struct {
		name  string
		value string
		spec  *Option
	}{
		{name: "model", value: options.Model, spec: &collar.Models.Option},
		{name: "effort", value: options.Effort, spec: collar.Options.Effort},
		{name: "role prompt", value: options.RolePrompt, spec: collar.Options.RolePrompt},
	} {
		if option.value == "" {
			continue
		}
		if option.spec == nil {
			return Command{}, fmt.Errorf("collar %q does not support %s", collar.Name, option.name)
		}
		optionArgs, err := renderArgs(option.spec.Args, map[string]string{"value": option.value})
		if err != nil {
			return Command{}, fmt.Errorf("render %s option: %w", option.name, err)
		}
		rendered = append(rendered, optionArgs...)
	}

	env := make(map[string]string, len(collar.Launch.Env))
	for key, value := range collar.Launch.Env {
		env[key] = value
	}
	return Command{Name: collar.Launch.Command, Args: rendered, Env: env}, nil
}

func renderArgs(args []string, values map[string]string) ([]string, error) {
	rendered := make([]string, len(args))
	for index, arg := range args {
		for _, match := range templatePattern.FindAllStringSubmatch(arg, -1) {
			value, ok := values[match[1]]
			if !ok {
				return nil, fmt.Errorf("argument %d contains unresolved template %q", index, match[1])
			}
			arg = strings.ReplaceAll(arg, match[0], value)
		}
		rendered[index] = arg
	}
	return rendered, nil
}

var templatePattern = regexp.MustCompile(`\{\{([a-z0-9_.-]+)\}\}`)

func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		if arg != "" && strings.IndexFunc(arg, func(char rune) bool {
			return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_@%+=:,./-", char)
		}) == -1 {
			quoted[index] = arg
			continue
		}
		quoted[index] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(quoted, " ")
}
