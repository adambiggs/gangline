package main

import "github.com/adambiggs/gangline/core"

func windowTitle(a core.Agent) string {
	mark := "?"
	switch a.Status {
	case core.Failed:
		mark = "!"
	case core.Active:
		switch a.Activity {
		case core.Idle:
			mark = "~"
		case core.Busy, core.Compacting, core.Interrupting:
			mark = "-"
		case core.Blocked, core.Wedged:
			mark = "!"
		}
	}
	return mark + string(a.Name) + mark
}
