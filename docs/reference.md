# Reference

`gang --help` lists commands. `gang help COMMAND` or `gang COMMAND --help`
shows arguments and flags for the installed binary. From an operator shell,
supply an explicit name when inspecting or controlling a single agent.

## Start and stop

| Command | Effect |
| --- | --- |
| `gang up [NAME] [AGENT OPTIONS]` | Start the configured team with its lead agent named `NAME` (default `lead`), then attach an interactive terminal. |
| `gang hitch NAME [OPTIONS]` | Launch an agent and deliver startup instructions and any task. |
| `gang hitch NAME --recover` | Recover retained startup from its visible draft or a lone collapsed paste. |
| `gang adopt NAME [-c COLLAR]` | Register the current pane without launching or delivering startup instructions. |
| `gang rename OLD NEW` | Change a registered name and window title. |
| `gang drop NAME` | Stop an agent and fail its pending messages. |
| `gang down SESSION` | Drop the team's agents and delete its runtime state and history. |
| `gang attach` | Attach to the configured team's tmux session. |
| `gang teams` | List teams in the configured state root. |

`up` and `hitch` accept:

| Option | Meaning |
| --- | --- |
| `-c`, `--collar COLLAR` | Harness collar. |
| `-d`, `--dir DIR` | Working directory; defaults to the current directory. |
| `-m`, `--model MODEL` | Native model identifier from `gang models`. |
| `-e`, `--effort EFFORT` | Native reasoning effort; requires `--model`. |
| `-t`, `--task TASK` | Startup assignment. |
| `-r`, `--role ROLE` | Role brief; `up` defaults to `lead`. |
| `--stdin` | Read the assignment from stdin. |
| `--resume SESSION` | Resume a native conversation. |
| `--recover` | Recover an agent's retained startup input. |

`adopt` also accepts `--collar`. Resume checks for the bundled collars require
a matching native transcript. An unverifiable native session is refused.
`drop` reports an observed native resume session, or says it is unknown.

## Message and control

| Command | Effect |
| --- | --- |
| `gang send NAME [OPTIONS] [BODY]` | Send BODY, or read stdin when absent, and report its receipt. |
| `gang queue [NAME]` | List pending message IDs, recipients, and senders. |
| `gang interrupt [NAME] [-m REASON]` | Interrupt the turn; deliver an optional reason after it stops. |
| `gang compact [NAME] [--resume TEXT]` | Compact at native idle; deliver the continuation after confirmed completion. |
| `gang compact NAME --recover` | Run the collar's recovery actions for a stuck compaction. |
| `gang curfew [DURATION\|HH:MM\|clear]` | Show, set, or clear the team deadline. |
| `gang tick [--agent ID]` | Check deadlines, recover native failures, and drain due messages. |
| `gang wait NAME [--timeout DURATION]` | Wait for a recorded idle boundary; a zero timeout checks once. |

Send options:

| Option | Meaning |
| --- | --- |
| `--from SENDER` | Required outside registered panes; refused inside them. |
| `--live-only` | Refuse instead of queuing if the recipient cannot take input now. |
| `--supersede` | Clear this sender's older scheduled messages for this recipient first. |
| `--at DURATION\|HH:MM` | Schedule delivery after a duration or at a local time. |
| `--at clear` | Clear this sender's scheduled messages for the recipient. |

A send prints the message ID and `delivered`, `accepted`, `queued`, or
`unverified`. See [message receipts](concepts.md#messages-and-envelopes).
Oversized rendered messages are refused without splitting; put details in a
file and send its path. `tick --agent` takes a hitch ID, not an agent name.
`--source watchdog --watchdog UNIT` is the internal watchdog invocation.

## Inspect

| Command | Effect |
| --- | --- |
| `gang roster [--porcelain]` | Show registered agents and current states; `--porcelain` gives machine-readable rows. |
| `gang status [NAME] [--why]` | Show state; `--why` includes activity and compaction evidence. |
| `gang capture [NAME] [LINES]` | Print the native pane. |
| `gang capture --composer [NAME]` | Print draft input from the composer. |
| `gang context [NAME]` | Show native context usage or an unknown reading. |
| `gang context --widget NAME\|off` | Select a context widget for the tmux status line, or turn it off. |
| `gang limits [NAME]` | Show observed provider limits. |
| `gang limits -c COLLAR` | Query native account limits without a live agent, if supported. |
| `gang log [--agent NAME\|HITCH_ID] [--type TYPE\|KIND] [LOG.jsonl]` | Print JSONL events from a team or saved log, with optional filters. |
| `gang whoami` | Print the calling pane's registered identity. |

Window titles use `?name?` for changing or unknown state, `~name~` for idle,
`-name-` for work, and `!name!` for blocked, wedged, or failed agents.
Use a hitch ID in log filters to follow a registration across renames.

## Discover and maintain

| Command | Effect |
| --- | --- |
| `gang collars` | List bundled and custom collars. |
| `gang collar check NAME` | Probe an installed harness in a private throwaway tmux session. |
| `gang models [-c COLLAR]` | List native models and efforts; also accepts `--collar`. |
| `gang roles` | List role briefs. |
| `gang config` | Print effective settings and their sources. |
| `gang statusline [--install]` | Render native status-line JSON; `--install` fills an absent Claude Code setting. |
| `gang --version` or `gang version` | Print the version. |
| `gang upgrade [--check]` | Install or check a stable release in an installer-managed checkout. |
| `gang help [COMMAND]` | Show command help. |
| `gang hook` | Read native hook JSON from stdin; invoked by the collar integration. |

## Configuration

Settings come from `$GANG_CONFIG_DIR/config`, one `NAME=VALUE` per line.
Blank lines and comments beginning with `#` are allowed. The file is parsed,
not sourced; unknown, duplicate, or blank settings fail. Environment variables
override file values. Run `gang config` to see effective values and defaults.

| Setting | Controls |
| --- | --- |
| `GANG_SESSION` | Team and tmux session name; defaults to `gangline`. |
| `GANG_COLLAR` | Default collar; defaults to `claude-code`. |
| `GANG_COLLARS` | Absolute directory of custom `NAME.cue` collars. |
| `GANG_LAUNCH_ARGS` | JSON object mapping collar names to extra launch argument arrays. |
| `GANG_CAPACITY_TIMEOUT` | Positive duration budget for provider-error continuations. |

These variables are environment-only:

| Variable | Default and purpose |
| --- | --- |
| `GANG_CONFIG_DIR` | `${XDG_CONFIG_HOME:-~/.config}/gangline`; absolute config directory. |
| `GANG_STATE_ROOT` | `${XDG_STATE_HOME:-~/.local/state}/gangline`; team state root. |
| `GANG_TMUX_SOCKET` | tmux's default socket; selects a separate server when set. |
| `GANG_TMUX` | `tmux`; executable used for tmux commands. |

For a separate team, keep its selection in the shell environment for every
command. A separate state root and socket also isolate its files and server:

```sh
export GANG_SESSION=review
export GANG_STATE_ROOT="$HOME/.local/state/gangline-review"
export GANG_TMUX_SOCKET="$GANG_STATE_ROOT/tmux.sock"
gang up
```

Installer variables:

| Variable | Default and purpose |
| --- | --- |
| `GANGLINE_HOME` | `~/.local/share/gangline`; retained release checkout. |
| `GANGLINE_BIN` | `~/.local/bin`; installed command directory. |
| `GANGLINE_REPO` | Public Gangline Git repository; release source. |

## Startup instructions

Files under `GANG_CONFIG_DIR` can customize the bundled instructions:

| File | Effect |
| --- | --- |
| `CONTRACT.md` | Replace the delivery contract. |
| `DOCTRINE.md` | Add operator policy. |
| `roles/NAME.md` | Replace or add a role brief. |

Operator policy takes precedence over role briefs. Put persistent staffing,
provider, model, and effort choices in `DOCTRINE.md`. These files are read
when the agent is hitched.

## Harnesses and platforms

A collar is a CUE file that tells Gangline how to launch and communicate with
a particular harness. The bundled collars are `claude-code` and `codex`.
Each uses the installed native CLI and its account settings. Use
`gang models -c COLLAR` for available
model and effort identifiers. Gangline leaves native permissions, login, and
trust decisions to the operator.

Claude Code context readings come from its status line. Codex readings come
from its native session log. `gang limits -c codex` queries account limits
through a private native app server without creating a conversation or turn.
The Claude Code collar has no standalone limits query; use `gang limits NAME`
for readings observed from an agent.

Linux uses systemd user timers for the watchdog; macOS uses transient launchd
user agents. On a host without a supported user scheduler, ordinary commands
and ticks remain available. See the [watchdog guide](guides.md#keep-an-unattended-team-moving)
and [internals](internals.md#watchdog).

## Exit status

| Status | Meaning |
| --- | --- |
| `0` | Successful command; a send may still be queued. |
| `1` | Execution or I/O error. |
| `2` | Bad usage. |
| `3` | Refused operation. |
| `4` | Native harness needs attention. |
| `5` | Unknown result, such as input without a confirmed receipt. |
