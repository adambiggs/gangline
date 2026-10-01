package main

import (
	"fmt"
	"io"
	"strings"
)

const welcomeHelp = `gang — a team of CLI agents in tmux

Find yourself:
  gang whoami
  gang roster

Send a message:
  gang send NAME 'message'
  printf '%s\n' 'message' | gang send NAME
  NAME is a registered agent, not the team session.

Read your waiting queue:
  gang queue

Run 'gang help' for every command or 'gang COMMAND --help' for one command.
`

const commandInventory = `usage: gang <command> [arguments]

Start and end a team:
  up        start a team and join it
  hitch     add an agent
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
  snooze    schedule a provider-reset or explicit wake
  whoami    read this pane's identity
  attach    join the team in tmux
  teams     list known teams

Settings and discovery:
  curfew    set or show the team deadline
  collars   list harness collars
  collar    check an installed harness
  models    list models for a collar (-c)
  roles     list role briefs
  config    show effective configuration

Installation:
  version   print the release version
  upgrade   install or check the latest release

Help:
  help      show command help

Native integration:
  hook      read native hook JSON from stdin
`

var commandUsage = map[string]string{
	"up":         "usage: gang up [NAME] [HITCH OPTIONS]\n",
	"hitch":      "usage: gang hitch NAME [-c COLLAR] [-d DIR] [-m MODEL] [-e EFFORT]\n       [-t TASK] [-r ROLE] [--resume SESSION] [--stdin]\n       gang hitch NAME --recover\n",
	"rename":     "usage: gang rename OLD NEW\n",
	"send":       "usage: gang send NAME [--from SENDER] [--live-only] [--supersede] [--at DURATION|HH:MM|RFC3339] [BODY]\n       gang send NAME --clear\n       Without BODY, read stdin. Use -- before BODY when it begins with -.\n",
	"queue":      "usage: gang queue [NAME] [--json]\n",
	"interrupt":  "usage: gang interrupt [NAME] [-m|--message REASON]\n",
	"compact":    "usage: gang compact [NAME] [--resume TEXT]\n       gang compact [NAME] --recover\n",
	"statusline": "usage: gang statusline [--install]\n",
	"context":    "usage: gang context [NAME] [--json]\n       gang context --widget NAME | --clear\n",
	"log":        "usage: gang log [--agent NAME|HITCH_ID] [--type TYPE|KIND] [LOG.jsonl]\n",
	"limits":     "usage: gang limits [NAME] | -c|--collar COLLAR\n",
	"snooze":     "usage: gang snooze [--at DURATION|HH:MM|RFC3339] [--note TEXT] [--clear [ID] | --status]\n",
	"wait":       "usage: gang wait NAME [--timeout DURATION]\n",
	"curfew":     "usage: gang curfew [DURATION | HH:MM | RFC3339 | clear]\n",
	"status":     "usage: gang status [NAME] [--why] [--json]\n",
	"tick":       "usage: gang tick [--agent NAME|HITCH_ID]\n       gang tick --source watchdog --watchdog UNIT\n",
	"capture":    "usage: gang capture [NAME] [-n|--lines LINES]\n       gang capture --composer [NAME]\n",
	"whoami":     "usage: gang whoami\n",
	"roster":     "usage: gang roster [--json]\n",
	"attach":     "usage: gang attach\n",
	"teams":      "usage: gang teams\n",
	"drop":       "usage: gang drop NAME\n",
	"down":       "usage: gang down [-y|--yes]\n",
	"collars":    "usage: gang collars\n       gang collar check NAME\n       Bundled names are claude and codex.\n",
	"collar":     "usage: gang collar check NAME\n",
	"models":     "usage: gang models [-c COLLAR]\n",
	"roles":      "usage: gang roles\n",
	"config":     "usage: gang config\n",
	"upgrade":    "usage: gang upgrade [--check]\n",
	"hook":       "usage: gang hook < native-hook.json\n",
	"help":       "usage: gang help [COMMAND]\n",
	"version":    "usage: gang version\n",
}

type optionSpec struct {
	name, argument, meaning string
}

// These definitions drive both flag registration and help. A nonempty argument
// names a required value; an empty argument denotes a boolean switch.
var commandOptions = map[string][]optionSpec{
	"hitch": {
		{"c", "COLLAR", "harness collar"}, {"collar", "COLLAR", "harness collar"},
		{"d", "DIR", "working directory"}, {"dir", "DIR", "working directory"},
		{"m", "MODEL", "native model"}, {"model", "MODEL", "native model"},
		{"e", "EFFORT", "reasoning effort"}, {"effort", "EFFORT", "reasoning effort"},
		{"t", "TASK", "startup assignment"}, {"task", "TASK", "startup assignment"},
		{"r", "ROLE", "role brief"}, {"role", "ROLE", "role brief"},
		{"resume", "SESSION", "resume a native conversation, not a team"},
		{"recover", "", "recover the original startup message"},
		{"stdin", "", "read the assignment from stdin"},
	},
	"send": {
		{"from", "SENDER", "outside sender identity"},
		{"live-only", "", "refuse rather than queue"},
		{"supersede", "", "replace older queued work"},
		{"at", "DURATION|HH:MM|RFC3339", "schedule delivery"},
		{"clear", "", "clear this sender's scheduled messages to NAME"},
	},
	"interrupt": {
		{"m", "REASON", "reason to deliver after interrupt"},
		{"message", "REASON", "reason to deliver after interrupt"},
	},
	"compact": {
		{"resume", "TEXT", "continuation after compaction"},
		{"recover", "", "interrupt an unconfirmed compaction while the pane is busy"},
	},
	"statusline": {{"install", "", "install the native status line"}},
	"context": {
		{"widget", "NAME", "show an agent's context in the widget"},
		{"clear", "", "clear the widget"},
		{"json", "", "print the exact reading as JSON"},
	},
	"log": {
		{"agent", "NAME|HITCH_ID", "filter by agent name or hitch ID"},
		{"type", "TYPE|KIND", "show events or readings of this type"},
	},
	"limits": {
		{"c", "COLLAR", "query a collar without an agent"},
		{"collar", "COLLAR", "query a collar without an agent"},
	},
	"snooze": {
		{"at", "DURATION|HH:MM|RFC3339", "wake time instead of native reset"},
		{"note", "TEXT", "continuation to carry into the wake"},
		{"clear", "", "clear or withdraw own wake; lead may clear a routed wake or uncertain notice by ID"},
		{"status", "", "show own wake, and for the lead, routed or uncertain usage intents"},
	},
	"wait":   {{"timeout", "DURATION", "maximum wait; zero checks once"}},
	"status": {{"why", "", "include recorded wedge evidence"}, {"json", "", "print the agent, its evidence, and compaction as JSON"}},
	"tick": {
		{"agent", "NAME|HITCH_ID", "tick one agent"},
		{"source", "watchdog", "identify a watchdog tick"},
		{"watchdog", "UNIT", "watchdog generation token"},
	},
	"capture": {
		{"n", "LINES", "print only the last LINES lines"},
		{"lines", "LINES", "print only the last LINES lines"},
		{"composer", "", "print only the native composer"},
	},
	"roster": {{"json", "", "print agents and availability as JSON"}},
	"queue":  {{"json", "", "print pending messages as JSON"}},
	"models": {
		{"c", "COLLAR", "harness collar"},
		{"collar", "COLLAR", "harness collar"},
	},
	"down":    {{"y", "", "skip confirmation"}, {"yes", "", "skip confirmation"}},
	"upgrade": {{"check", "", "check for a release without installing"}},
}

func optionsFor(name string) []optionSpec {
	if name == "up" {
		return commandOptions["hitch"]
	}
	return commandOptions[name]
}

func optionSpelling(name string) string {
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

func optionHelp(name string) string {
	var out strings.Builder
	options := optionsFor(name)
	for i := 0; i < len(options); i++ {
		option := options[i]
		label := optionSpelling(option.name)
		if i+1 < len(options) && len(option.name) == 1 {
			alias := options[i+1]
			if len(alias.name) > 1 && alias.argument == option.argument && alias.meaning == option.meaning {
				label += ", " + optionSpelling(alias.name)
				i++
			}
		}
		fmt.Fprintf(&out, "  %-22s %s\n", label+optionArgument(option), option.meaning)
	}
	return out.String()
}

func optionArgument(option optionSpec) string {
	if option.argument == "" {
		return ""
	}
	return " " + option.argument
}

var commandDescription = map[string]string{
	"up":         "NAME names the lead agent (default lead) in the GANG_SESSION team\n(default gangline), not the team itself. Start and attach, or use --recover\nto recover that existing agent's startup.\n",
	"hitch":      "NAME is an agent's registered name in the configured team.\nLaunch a new agent and deliver its startup, or use --recover to recover\nretained startup input for an existing agent.\n",
	"rename":     "OLD and NEW are registered agent names in the configured team.\nRename the agent without restarting its harness.\n",
	"send":       "NAME is the recipient agent's registered name, not a team session.\nSend BODY or read stdin. An exact native hook proves delivery; a native\nqueue receipt proves acceptance. Otherwise input stays queued or unverified.\n",
	"queue":      "NAME filters pending work to one registered agent. Omit it to list\npending work for every agent in the configured team. Each message shows\nwhether it is ready, scheduled, blocked, or unknown, with its due time or\nthe reason it waits.\n",
	"interrupt":  "NAME is a registered agent; omit it for the current pane's agent.\nInterrupt its native turn and optionally deliver a reason afterward.\n",
	"compact":    "NAME is a registered agent; omit it for the current pane's agent.\nQueue the continuation as compaction starts; admit it once when the harness submits it.\n",
	"statusline": "Read native status-line JSON on stdin; --install fills an absent native setting.\n",
	"context":    "NAME is a registered agent; omit it for the current pane's agent.\nPrint its native context reading. --widget NAME selects an agent for the\ntmux widget; --clear empties it.\n",
	"log":        "Print the configured team's durable JSONL event log.\nThe agent filter accepts a registered NAME or a hitch ID.\n",
	"limits":     "NAME is a registered agent; omit it for the current pane's agent.\nRead its provider limits, or use --collar to query a collar without an agent.\n",
	"snooze":     "Schedule a wake for the calling agent. Without --at, use the most-used future native five-hour or weekly reset. A wake completes after its matching native turn succeeds. --status shows queued and uncertain wakes. --clear or a new --at withdraws a wake still queued for input; the lead can clear a notice or fallback wake by ID. Clearing cannot retract native input.\n",
	"wait":       "NAME is the registered agent to watch for a recorded idle boundary.\nA zero timeout checks once.\n",
	"curfew":     "Declare, inspect, or clear one team deadline.\n",
	"status":     "NAME is a registered agent; omit it for the current pane's agent.\nShow its status and activity; --why adds recorded wedge evidence.\n",
	"tick":       "Check deadlines and native recovery, then drain due inbox work.\n",
	"capture":    "NAME is a registered agent whose pane to capture. Without NAME,\ncapture the current tmux pane, even if unregistered. With --composer,\nomitting NAME selects the current pane's registered agent.\n",
	"whoami":     "Print the registered identity of the calling Gangline pane.\n",
	"roster":     "List the team's registered agents and conservative current states.\n",
	"attach":     "Attach this terminal to the configured team session.\n",
	"teams":      "List teams found in the teams directory.\n",
	"drop":       "NAME is the registered agent to stop, not the team session.\nCancel its pending work; report an observed native resume ID or unknown.\n",
	"down":       "Stop the configured GANG_SESSION team and remove its state. On a terminal,\nconfirm the session and agent count; use the yes option for scripts and\nnonterminal calls.\n",
	"collars":    "List embedded and operator-provided CUE collars. In 'collar check\nNAME', NAME identifies a collar, not an agent or team.\n",
	"collar":     "NAME is an installed harness collar, not an agent or team.\nProbe it in a throwaway private tmux session.\n",
	"models":     "Discover model and reasoning-effort identifiers through a collar's native catalog.\n",
	"roles":      "List embedded and operator-provided role briefs.\n",
	"config":     "Print persistent settings, effective values, and their sources.\n",
	"upgrade":    "Check or install the latest stable release into an installer-managed tree.\n",
	"hook":       "Read native hook JSON on stdin and resolve the pane by GANGLINE_HITCH_ID.\n",
	"help":       "Show the command inventory or detailed help for one command.\n",
	"version":    "Print the release version.\n",
}

const flagSyntaxHelp = "Flags accept - or --, VALUE or =VALUE, and one-letter flags also -cVALUE;\nswitches accept =true/=false. Flags may follow operands; -- ends flags.\n"

// indented prefixes each line of text with two spaces.
func indented(text string) string {
	return "  " + strings.ReplaceAll(strings.TrimSuffix(text, "\n"), "\n", "\n  ") + "\n"
}

func (cmd command) printHelp(name string) error {
	if name == "" {
		var out strings.Builder
		out.WriteString(commandInventory)
		out.WriteString("\nGlobal flags:\n  -h, --help             show help\n  --version              print the release version\n")
		out.WriteString(indented(flagSyntaxHelp))
		_, err := io.WriteString(cmd.stdout, out.String())
		return err
	}
	usage, ok := commandUsage[name]
	if !ok {
		return usageError("help: unknown command %q", name)
	}
	_, err := fmt.Fprintf(cmd.stdout, "%s\n%s", usage, commandDescription[name])
	if err != nil {
		return err
	}
	if flags := optionHelp(name); flags != "" {
		_, err = fmt.Fprintf(cmd.stdout, "\nOptions:\n%s", flags)
	}
	if err == nil {
		_, err = io.WriteString(cmd.stdout, "\nHelp:\n  -h, --help             show this help\n")
	}
	if err == nil && len(optionsFor(name)) != 0 {
		_, err = io.WriteString(cmd.stdout, "\n"+flagSyntaxHelp)
	}
	return err
}
