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
  tick      drain due inbox work
  wait      wait for an idle boundary
  capture   read an agent's pane or composer
  context   read native context use
  statusline render or install the native context footer
  log       read the event log
  limits    read current provider limits
  whoami    read this pane's identity
  attach    join the team in tmux
  teams     list known teams

Settings and discovery:
  curfew    set or show the team deadline
  collars   list harness collars
  models    list models for a collar (-c)
  roles     list role briefs
  config    show effective configuration

Installation:
  --version print the release version
  upgrade   install or check the latest release

Native integration:
  hook      read native hook JSON from stdin
`

var commandUsage = map[string]string{
	"up":         "usage: gang up [NAME] [HITCH OPTIONS]\n",
	"hitch":      "usage: gang hitch NAME [-c COLLAR] [-d DIR] [-m MODEL] [-e EFFORT] [-t TASK] [-r ROLE] [--resume SESSION] [--stdin]\n       gang hitch NAME --recover\n",
	"adopt":      "usage: gang adopt NAME -c COLLAR\n",
	"rename":     "usage: gang rename OLD NEW\n",
	"send":       "usage: gang send NAME [--from SENDER] [--live-only] [--supersede] [--at TIME]\n",
	"queue":      "usage: gang queue [NAME]\n",
	"interrupt":  "usage: gang interrupt [NAME] [-m REASON]\n",
	"compact":    "usage: gang compact [NAME] [--resume TEXT]\n       gang compact NAME --recover\n",
	"statusline": "usage: gang statusline [--install]\n",
	"context":    "usage: gang context [NAME]\n       gang context --widget NAME|off\n",
	"log":        "usage: gang log [--agent NAME|HITCH_ID] [--type TYPE|KIND] [LOG.jsonl]\n",
	"limits":     "usage: gang limits [NAME] | -c COLLAR\n",
	"wait":       "usage: gang wait NAME [--timeout DURATION]\n",
	"curfew":     "usage: gang curfew [DURATION | HH:MM | clear]\n",
	"status":     "usage: gang status [NAME] [--why]\n",
	"tick":       "usage: gang tick [--agent ID]\n",
	"capture":    "usage: gang capture [NAME] [LINES]\n       gang capture --composer [NAME]\n",
	"whoami":     "usage: gang whoami\n",
	"roster":     "usage: gang roster [--porcelain]\n",
	"attach":     "usage: gang attach\n",
	"teams":      "usage: gang teams\n",
	"drop":       "usage: gang drop NAME\n",
	"down":       "usage: gang down SESSION\n",
	"collars":    "usage: gang collars\n       gang collar check NAME\n",
	"collar":     "usage: gang collar check NAME\n",
	"models":     "usage: gang models [-c COLLAR]\n",
	"roles":      "usage: gang roles\n",
	"config":     "usage: gang config\n",
	"upgrade":    "usage: gang upgrade [--check]\n",
	"hook":       "usage: gang hook < native-hook.json\n",
}

var commandDescription = map[string]string{
	"up":         "Start a team, hitch its lead, and attach this terminal.\n",
	"hitch":      "Launch a native harness in a new team pane and deliver its contract, role, and assignment.\n",
	"adopt":      "Register an existing pane without launching a harness or delivering startup prose.\n",
	"rename":     "Change a registered agent name without restarting its harness.\n",
	"send":       "Read a message from stdin. An exact native hook proves delivered; a native queue receipt proves accepted (do not resend). Otherwise input stays queued or is unverified.\n",
	"queue":      "List pending delivery identifiers, recipients, and senders.\n",
	"interrupt":  "Send the collar's native interrupt and optionally deliver a reason after the turn stops.\n",
	"compact":    "Request the collar's native compaction and place a continuation behind it.\n",
	"statusline": "Read native status-line JSON on stdin; --install fills an absent native setting.\n",
	"context":    "Print the collar's native context reading without estimating missing data.\n",
	"log":        "Print the configured team's durable JSONL event log.\n",
	"limits":     "Read provider limits from an agent, or query a collar's native account without an agent using -c.\n",
	"wait":       "Watch agent.json until an agent reaches a recorded idle boundary or the deadline expires. A zero timeout checks once.\n",
	"curfew":     "Declare, inspect, or clear one team deadline.\n",
	"status":     "Show one agent's recorded status and activity; --why adds recorded wedge evidence.\n",
	"tick":       "Check deadlines and native recovery, then drain due inbox work.\n",
	"capture":    "Print pane content, or only the native composer with --composer.\n",
	"whoami":     "Print the registered identity of the calling Gangline pane.\n",
	"roster":     "List the team's registered agents and conservative current states.\n",
	"attach":     "Attach this terminal to the configured team session.\n",
	"teams":      "List teams found in the teams directory.\n",
	"drop":       "Stop one registered agent pane and cancel its pending work.\n",
	"down":       "Drop every live agent, then remove the stopped team's runtime state.\n",
	"collars":    "List embedded and operator-provided CUE collars.\n",
	"collar":     "Probe one installed harness in a throwaway private tmux session.\n",
	"models":     "Discover model and reasoning-effort identifiers through a collar's native catalog.\n",
	"roles":      "List embedded and operator-provided role briefs.\n",
	"config":     "Print persistent settings, effective values, and their sources.\n",
	"upgrade":    "Check or install the latest stable release into an installer-managed tree.\n",
	"hook":       "Read native hook JSON on stdin and resolve the pane by GANGLINE_HITCH_ID.\n",
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
