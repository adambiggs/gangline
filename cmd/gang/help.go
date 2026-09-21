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
  run       run a host command and return its result
  queue     read waiting messages
  flush     recover a harness-queued message
  interrupt stop an agent's current turn
  compact   compact an agent's context

Observe and control:
  roster    list the team
  status    inspect one agent
  tick      retry pending effects
  wait      wait for an agent boundary
  capture   read an agent's pane or composer
  context   read native context use
  log       read the event log
  replay    fold an event log offline
  usage     read token consumption
  cap       read provider-window history
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
	"run":       "usage: gang run -- COMMAND [ARG ...]\n       gang run --active\n       gang run --cancel ID\n",
	"flush":     "usage: gang flush [NAME]\n",
	"queue":     "usage: gang queue [NAME]\n",
	"interrupt": "usage: gang interrupt [NAME] [-m REASON] [--from SENDER]\n",
	"compact":   "usage: gang compact [NAME] [--resume TEXT]\n       gang compact NAME --recover\n",
	"context":   "usage: gang context [NAME]\n",
	"log":       "usage: gang log [NAME] [--since TIME] [--kind KIND]\n",
	"replay":    "usage: gang replay [EVENTS.jsonl]\n",
	"usage":     "usage: gang usage [--daily [DATE] | --since DATE]\n",
	"cap":       "usage: gang cap [check | watch [--clear] | forget]\n",
	"limits":    "usage: gang limits [NAME | --history]\n",
	"wait":      "usage: gang wait NAME --until idle|done [--timeout DURATION]\n       gang wait --limit [NAME] [--resume TEXT | --clear]\n",
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

func (cmd command) printHelp(name string) error {
	if name == "" {
		_, err := io.WriteString(cmd.stdout, commandInventory)
		return err
	}
	usage, ok := commandUsage[name]
	if !ok {
		return usageError("help: unknown command %q", name)
	}
	_, err := fmt.Fprint(cmd.stdout, usage)
	return err
}
