package main

import "github.com/adambiggs/gangline/core"

func paneTitle(a core.Agent) string {
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
			if a.Activity == core.Blocked && a.Evidence == heldInputEvidence {
				mark = "~"
			}
		}
	}
	return mark + string(a.Name) + mark
}
