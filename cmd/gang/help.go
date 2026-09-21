package main

import (
	"fmt"
	"io"
)

const welcomeHelp = `gang — a team of CLI agents in tmux

Find yourself:
  gang whoami
  gang roster

Send a message:
  printf '%s\n' 'message' | gang send NAME

Read your waiting queue:
  gang queue

Run 'gang help' for every command or 'gang COMMAND --help' for one command.
`

const commandInventory = `usage: gang <command> [arguments]

Start and end a team:
  up        start a team and join it
  hitch     add an agent
  adopt     register an existing window
  rename    change an agent's registered name
  drop      remove one agent
  down      end the team

Talk to agents:
  send      deliver a message
  queue     read waiting messages
  interrupt stop an agent's current turn
  compact   compact an agent's context

Observe and control:
  roster    list the team
  status    inspect one agent
  tick      retry pending effects
  idle      check for an idle boundary
  capture   read an agent's pane or composer
  context   read native context use
  log       read the event log
  replay    fold an event log offline
  limits    read current provider limits
  whoami    read this pane's identity
  attach    join the team in tmux
  teams     list known teams

Settings and discovery:
  curfew    set or show the team deadline
  collars   list harness collars
  models    list a collar's models
  roles     list role briefs
  config    show effective configuration

Installation:
  --version print the release version
  upgrade   install or check the latest release
`

var commandUsage = map[string]string{
	"up":        "usage: gang up [NAME] [HITCH OPTIONS]\n",
	"hitch":     "usage: gang hitch NAME [-c COLLAR] [-d DIR] [-m MODEL] [-e EFFORT] [-t TASK] [-r ROLE] [--resume SESSION] [--stdin]\n",
	"adopt":     "usage: gang adopt NAME -c COLLAR\n",
	"rename":    "usage: gang rename OLD NEW\n",
	"send":      "usage: gang send NAME [--from SENDER] [--live-only] [--supersede] [--at TIME]\n",
	"queue":     "usage: gang queue [NAME]\n",
	"interrupt": "usage: gang interrupt [NAME] [-m REASON]\n",
	"compact":   "usage: gang compact [NAME] [--resume TEXT]\n       gang compact NAME --recover\n",
	"context":   "usage: gang context [NAME]\n",
	"log":       "usage: gang log\n",
	"replay":    "usage: gang replay [EVENTS.jsonl]\n",
	"limits":    "usage: gang limits [NAME]\n",
	"idle":      "usage: gang idle NAME\n",
	"curfew":    "usage: gang curfew [DURATION | HH:MM | clear]\n",
	"status":    "usage: gang status [NAME] [--why]\n",
	"tick":      "usage: gang tick\n",
	"capture":   "usage: gang capture [NAME] [LINES]\n       gang capture --composer [NAME]\n",
	"whoami":    "usage: gang whoami\n",
	"roster":    "usage: gang roster [--porcelain]\n",
	"attach":    "usage: gang attach\n",
	"teams":     "usage: gang teams\n",
	"drop":      "usage: gang drop NAME\n",
	"down":      "usage: gang down SESSION\n",
	"collars":   "usage: gang collars\n       gang collar check NAME\n",
	"collar":    "usage: gang collar check NAME\n",
	"models":    "usage: gang models [-c COLLAR]\n",
	"roles":     "usage: gang roles\n",
	"config":    "usage: gang config\n",
	"upgrade":   "usage: gang upgrade [--check]\n",
}

var commandDescription = map[string]string{
	"up":        "Start a team, hitch its lead, and attach this terminal.\n",
	"hitch":     "Launch a native harness in a new team pane and deliver its contract, role, and assignment.\n",
	"adopt":     "Register an existing pane without launching a harness or delivering startup prose.\n",
	"rename":    "Change a registered agent name without restarting its harness.\n",
	"send":      "Read a message from stdin. Delivery is submitted from an empty composer and verified by a native hook, or remains pending for a turn boundary.\n",
	"queue":     "List pending delivery identifiers, recipients, and senders.\n",
	"interrupt": "Send the collar's native interrupt and optionally deliver a reason after the turn stops.\n",
	"compact":   "Request the collar's native compaction and place a continuation behind it.\n",
	"context":   "Print the collar's native context reading without estimating missing data.\n",
	"log":       "Print the configured team's durable JSONL event log.\n",
	"replay":    "Fold a recorded JSONL event stream without contacting tmux or a harness.\n",
	"limits":    "Read current provider limits from the collar's native source.\n",
	"idle":      "Succeed only when an agent is already at a recorded idle boundary.\n",
	"curfew":    "Declare, inspect, or clear one team deadline.\n",
	"status":    "Show one agent's recorded status and activity; --why adds recorded wedge evidence.\n",
	"tick":      "Run one bounded retry pass over durable pending effects.\n",
	"capture":   "Print pane content, or only the native composer with --composer.\n",
	"whoami":    "Print the registered identity of the calling Gangline pane.\n",
	"roster":    "List the team's registered agents and conservative current states.\n",
	"attach":    "Attach this terminal to the configured team session.\n",
	"teams":     "List teams found in the versioned state root.\n",
	"drop":      "Stop one registered agent pane and cancel its pending work.\n",
	"down":      "Drop every live agent, then remove the stopped team's runtime state.\n",
	"collars":   "List embedded and operator-provided CUE collars.\n",
	"collar":    "Probe one installed harness in a throwaway private tmux session.\n",
	"models":    "Discover model and reasoning-effort identifiers through a collar's native catalog.\n",
	"roles":     "List embedded and operator-provided role briefs.\n",
	"config":    "Print persistent settings, effective values, and their sources.\n",
	"upgrade":   "Check or install the latest stable release into an installer-managed tree.\n",
}

func (cmd command) printHelp(name string) error {
	if name == "" {
		_, err := io.WriteString(cmd.stdout, commandInventory)
		return err
	}
	usage, ok := commandUsage[name]
	if !ok {
		return usageError("help: unknown command %q", name)
	}
	_, err := fmt.Fprintf(cmd.stdout, "%s\n%s", usage, commandDescription[name])
	return err
}
