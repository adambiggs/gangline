package main

import "github.com/adambiggs/gangline/harness"

func applyLaunchPolicy(command harness.Command, collar string, settings settings) harness.Command {
	result := command
	result.Args = append(append([]string(nil), command.Args...), settings.LaunchArgs[collar]...)
	return result
}
