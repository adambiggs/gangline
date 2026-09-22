# CLI and collar reference

`gang help` lists every command. `gang COMMAND --help` shows one command's
arguments.

## Install

Gangline runs on macOS and Linux. It needs Git, Go 1.27 or later, tmux 3.2 or
later, and Claude Code or Codex.

```sh
curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
```

| Variable | Default | Sets |
| --- | --- | --- |
| `GANGLINE_HOME` | `~/.local/share/gangline` | where the release checkout lives |
| `GANGLINE_BIN` | `~/.local/bin` | where `gang` is installed |
| `GANGLINE_REPO` | the public repository | where releases come from |

## Commands

### Team lifecycle

| Command | Effect |
| --- | --- |
| `gang up [NAME] [HITCH OPTIONS]` | Hitch the first agent (`lead` by default) and attach. |
| `gang hitch NAME [OPTIONS]` | Launch a harness window and send it its contract, role, and assignment. |
| `gang adopt NAME -c COLLAR` | Register the current tmux pane as an agent. |
| `gang rename OLD NEW` | Rename an agent and its window. |
| `gang drop NAME` | Stop one agent and fail its pending messages. |
| `gang down SESSION` | Drop every agent and delete the team's state. |

Hitch options: `-c/--collar`, `-d/--dir`, `-m/--model`, `-e/--effort`,
`-t/--task`, `-r/--role`, `--resume SESSION`, and `--stdin` (read the
assignment from stdin). `-e` needs `-m`. `gang up` defaults to the `lead` role
and the current directory.

A failed agent keeps its name until you drop it.

### Messages and control

| Command | Effect |
| --- | --- |
| `gang send NAME [--from NAME] [--live-only] [--supersede] [--at TIME]` | Send stdin to an agent. |
| `gang send NAME --at clear` | Clear the agent's scheduled messages. |
| `gang queue [NAME]` | List queued messages. |
| `gang interrupt [NAME] [-m REASON]` | Interrupt an agent's turn. |
| `gang compact [NAME] [--resume TEXT]` | Compact an agent's context and queue a follow-up message. |
| `gang compact NAME --recover` | Run the collar's recovery actions for a stuck compaction. |

From an agent window, `send` uses that agent's name and refuses `--from`. From
anywhere else `--from` is required, and the envelope marks the name
`self-declared:`.

- `--live-only` sends only if the agent can take the message now.
- `--supersede` clears the agent's older scheduled messages first.
- `--at` takes a duration (`45m`) or a local time (`14:30`).

A message can be at most 1 MiB once rendered. Longer ones are refused, never
split; put the details in a file and send its path.

### Observation

| Command | Effect |
| --- | --- |
| `gang roster [--porcelain]` | List agents and their state. |
| `gang status [NAME] [--why]` | Show one agent; `--why` adds wedge evidence. |
| `gang capture [NAME] [LINES]` | Print an agent's screen. |
| `gang capture --composer [NAME]` | Print what's in an agent's composer. |
| `gang context [NAME]` | Show an agent's context use. |
| `gang context --widget NAME\|off` | Show one agent's context in the tmux status line, or turn it off. |
| `gang statusline [--install]` | Render Claude's status line; `--install` sets it up. |
| `gang limits [NAME]` | Show provider usage limits. |
| `gang log [--agent NAME\|HITCH_ID] [--type TYPE\|KIND]` | Print the team's events. |
| `gang replay [--agent NAME\|HITCH_ID] [--type TYPE\|KIND] [EVENTS.jsonl]` | Rebuild state from an event file, or filter it. |
| `gang tick` | Retry pending work and resume after provider errors. |
| `gang wait NAME [--timeout DURATION]` | Wait until an agent is idle (default 30s; `0` checks once). |
| `gang whoami` | Print the agent this pane belongs to. |
| `gang teams` | List teams. |
| `gang attach` | Attach to the team's tmux session. |

Window names show state: `?name?` changing, `~name~` idle, `-name-` working,
`!name!` blocked, wedged, or failed.

Filters match event types or reading kinds, for example
`gang log --agent worker --type context`. Use the hitch ID to follow an agent
across renames.

### Setup and discovery

| Command | Effect |
| --- | --- |
| `gang collars` | List collars. |
| `gang collar check NAME` | Test an installed harness in throwaway tmux sessions. |
| `gang models [-c COLLAR]` | List a harness's models and efforts. |
| `gang roles` | List role briefs. |
| `gang config` | Print settings and where each came from. |
| `gang curfew [DURATION\|HH:MM\|clear]` | Show, set, or clear the team deadline. |
| `gang --version` | Print the version. |
| `gang upgrade [--check]` | Check for or install the latest release. |

## Exit status

| Status | Meaning |
| --- | --- |
| `0` | Done. A send may only be queued. |
| `1` | Execution or I/O error. |
| `2` | Bad usage. |
| `3` | Refused before changing anything. |
| `4` | The harness needs attention, such as a startup prompt. |
| `5` | Unknown result, such as a message typed but never confirmed. |

## Configuration

Settings come from `$GANG_CONFIG_DIR/config` (default
`${XDG_CONFIG_HOME:-~/.config}/gangline/config`), one `NAME=VALUE` per line.
The file is parsed, never sourced. Environment variables override it.

| Setting | Default | Sets |
| --- | --- | --- |
| `GANG_SESSION` | `gangline` | team and tmux session name |
| `GANG_COLLAR` | `claude-code` | default collar |
| `GANG_COLLARS` | unset | directory of your own `NAME.cue` collars |
| `GANG_LAUNCH_ARGS` | unset | JSON object mapping collar names to extra launch arguments |
| `GANG_CAPACITY_TIMEOUT` | `5m` | how long `gang tick` keeps retrying after provider errors |

Environment only:

| Variable | Default | Sets |
| --- | --- | --- |
| `GANG_CONFIG_DIR` | `${XDG_CONFIG_HOME:-~/.config}/gangline` | config directory |
| `GANG_STATE_ROOT` | `${XDG_STATE_HOME:-~/.local/state}/gangline` | state directory |
| `GANG_TMUX_SOCKET` | tmux's default | tmux socket |
| `GANG_TMUX` | `tmux` | tmux executable |

A team's state is under `STATE_ROOT/v1/TEAM/`. `gang down` deletes it.

## Startup prose

The shipped contract and role briefs are built in. Files in `GANG_CONFIG_DIR`
override them:

- `CONTRACT.md` replaces the contract;
- `DOCTRINE.md` adds your own guidance; and
- `roles/NAME.md` replaces or adds a role brief.

A collar with a `role_prompt` option puts this prose in the harness's system
prompt. The assignment is still sent as the first message.

## Collars

A collar is a CUE file with a top-level `collar` value, checked against
`harness/schema/collar.cue`. The filename must match `collar.name`. It
declares:

- `launch`: the command and its arguments for a new, resumed, or probe run;
- `hooks`: launch arguments that wire in `gang hook`, and how to read each hook
  payload;
- `models`: how to list models and pass the chosen one;
- `options`: argument templates for effort and the role prompt;
- `primitives`: which built-in Go behaviors to use for startup, the composer,
  submission, turn ends, prompts, context, limits, and wedges, plus
  `mid_turn` and the optional `telemetry` source;
- `actions`: key sequences for interrupt, compact, and recovery; and
- `context_bands`: named context thresholds per model.

Unknown fields or primitive names fail before launch.

## Readings

Gang records context use, provider limits, turn and compaction events, and
errors as `observation` events. Each reading has a `kind`, a `source`
(`native-hook`, `session-log`, `status-line`, or `screen`), and a `status`
(`observed`, `unknown`, or `error`, with a `reason` when not observed).

- **Claude** readings come from its
  [status line](https://code.claude.com/docs/en/statusline). Context is input
  tokens plus cache reads and writes.
- **Codex** readings come from `token_count` records in its session log.
  Context is `last_token_usage.total_tokens` out of `model_context_window`.
  The log format isn't a stable API, so gang fails loudly if it changes.

Readings are collected by hooks and by `gang tick`, `gang context`, and
`gang limits`. Nothing polls in the background.
