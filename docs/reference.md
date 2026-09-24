# CLI and collar reference

`gang help` lists every command. `gang COMMAND --help` shows one command's
arguments.

## Install

Gangline runs on macOS and Linux. It needs Git, Go 1.27 or later, tmux 3.2 or
later, and Claude Code or Codex. Older shell-based releases also need Python.
The installer prints a PATH instruction if `~/.local/bin` is not on your PATH.

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
| `gang hitch NAME --recover` | Recover original startup text still present in the native composer. |
| `gang hitch NAME [OPTIONS]` | Launch a harness window and send it its contract, role, and assignment. |
| `gang adopt NAME -c COLLAR` | Register the current tmux pane as an agent. |
| `gang rename OLD NEW` | Rename an agent and its window. |
| `gang drop NAME` | Stop one agent and fail its pending messages. |
| `gang down SESSION` | Drop every agent and delete the team's state. |

Hitch options: `-c/--collar`, `-d/--dir`, `-m/--model`, `-e/--effort`,
`-t/--task`, `-r/--role`, `--resume SESSION`, and `--stdin` (read the
assignment from stdin). `-e` needs `-m`. `gang up` defaults to the `lead` role
and the current directory.

Before launching a resumed window for a collar with supported transcript
discovery, Gangline verifies its native session identity. Discovery supports
Codex session logs and Claude project transcripts, including their native
configuration-directory overrides. Missing transcripts and mismatched
identities are refused as unverified. Custom collars without supported
transcript discovery retain their native CLI's resume validation.

A failed agent keeps its name until you drop it.

### Messages and control

| Command | Effect |
| --- | --- |
| `gang send NAME [--from NAME] [--live-only] [--supersede] [--at TIME]` | Send stdin to an agent. |
| `gang send NAME --at clear` | Clear the sender's scheduled messages for this agent. |
| `gang queue [NAME]` | List queued messages. |
| `gang interrupt [NAME] [-m REASON]` | Interrupt an agent's turn. |
| `gang compact [NAME] [--resume TEXT]` | Queue compaction at native idle; resume only after confirmed completion. |
| `gang compact NAME --recover` | Run the collar's recovery actions for a stuck compaction. |

A send reports `delivered` when the exact native submit hook confirms it.
`accepted` means the native queue shows the sender and full one-time-token opener;
it exits successfully and says not to resend. Queue previews can truncate the
body, so acceptance does not claim full-text submission or that the agent read
or acted on it. `queued` remains in Gangline's spool. Without a submit witness
or an identifiable native queue entry, the result is `unverified` and exits
unsuccessfully. Native queue recognition is declared by the collar's optional
`queue_witness` primitive; startup assignments still require exact hook proof.

From an agent window, `send` uses that agent's name and refuses `--from`. From
anywhere else `--from` is required, and the envelope marks the name
`self-declared:`.

- `--live-only` sends only if the agent can take the message now.
- `--supersede` clears the sender's older scheduled messages for this recipient first.
- `--at` takes a duration (`45m`) or a local time (`14:30`).

A recognized native choice menu keeps the agent blocked and the message queued.
Send reports the observed prompt and choice on stderr; `--live-only` refuses
without queuing. Resolve the menu in the native pane, then run `gang tick`.
Gang does not choose a model or dismiss the menu.

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
| `gang limits [NAME]` | Show observed provider usage limits for an agent. |
| `gang limits -c COLLAR` | Query native account limits without a live agent. |
| `gang log [--agent NAME\|HITCH_ID] [--type TYPE\|KIND] [LOG.jsonl]` | Print the team's events. |
| `gang tick [--agent ID]` | Retry pending work and resume after provider errors. |
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

### Idle-team watchdog

On Linux with a systemd user manager or macOS with launchd, a whole-team
`gang tick` arms a transient watchdog for **60 seconds** (fixed, no backoff).
Linux uses `systemd-run --user --on-active`; macOS uses a `StartInterval`
user agent with `LaunchOnlyOnce`.
Neither needs a resident Gangline scheduler process. Expiry depends on
an awake host and an available user scheduler.
The minute interval bounds idle refresh latency without constant wakeups.
Systemd may coalesce expiry within its one-second accuracy window. The next
whole-team tick replaces the timer and pushes the deadline out. An agent-scoped
`gang tick --agent ID` arms only when none exists; it never postpones idle peers.
Hitch and adopt also arm the initial timer.

Watchdog ticks re-arm themselves and skip locked agents. Detached hook ticks
retain their lock wait so a native boundary notice survives contention; the
original hook does not wait. The audit `tick` event has `source` set to `watchdog`, `hook`, or
`command`; `gang log` exposes it. `--source watchdog --watchdog UNIT` carries an
internal generation token so a superseded timer cannot re-arm itself.
Last drop and `gang down` disarm the timer. A concurrent timer transaction can
make cleanup fail visibly; retry with `gang down SESSION`. Timer state and the
unavailable marker live in the team directory and `gang down` removes them.
The launchd plist also lives there, outside the login-loaded LaunchAgents
directory, and disarm removes it. An uncertain scheduler failure preserves
the plist for a later cleanup attempt.

On a host without a supported user scheduler, ordinary ticks work as before;
`watchdog_unavailable` is logged once per team and no timer is armed.
A scheduler command failure logs
`watchdog_failed`; agent work still proceeds and the command returns the error.
Run `gang tick` after repairing the user manager.
The watchdog requires the user manager to remain running, including across
logout for unattended teams; it does not configure login lingering.

The launchd adapter is covered by fake-command tests on Linux. Native macOS
timer replacement, self-rearm, and cleanup remain unverified.

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

A team's state is under `STATE_ROOT/teams/TEAM/`. It contains `team.json`,
`log.jsonl`, `names/NAME -> ID` claims, and `agents/ID/` directories.
Each agent directory contains `agent.json`, `lock`, `witness`, and
`inbox/{tmp,new,cur,failed}/`. `gang drop` removes its agent directory and name
claim; `gang down` deletes the team directory. Terminal inbox directories
retain the latest result; use `gang log` for history.

## Startup prose

The shipped contract and role briefs are built in. Files in `GANG_CONFIG_DIR`
override them:

- `CONTRACT.md` replaces the contract;
- `DOCTRINE.md` adds your own guidance; and
- `roles/NAME.md` replaces or adds a role brief.

Put persistent operator policy, including provider, model, effort, and staffing
choices, in `DOCTRINE.md`. Operator instructions take precedence over role
briefs, including local overrides. Gangline copies these files without
reflowing their text.

A collar with a `role_prompt` option puts standing prose in the harness's
system prompt and sends only the assignment as the first message. Other
collars receive the prose and assignment together in that message. Without a
task, the startup message states that no assignment was supplied.

Startup names the launching agent when its pane is registered; otherwise its
sender is `gangline:hitch`, identifying Gangline itself. The envelope is marked
`assignment` only when a task was supplied, or `startup` otherwise.

Gangline's context-band notices use `[gang:context-band#token]` tags without a message
ID. Default compaction resume, interrupt reasons, and capacity recovery carry
`gangline:` senders. A custom `--resume` note names
the registered caller; outside a registered pane it carries
`self-declared:compact`. Interrupt `-m` reasons are supplied by whoever ran the
command. A completed compaction's resume note is delivered before any other
queued message.

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
  `mid_turn`, the optional `telemetry` source, and an optional `limits_query`;
- `actions`: key sequences for interrupt, compact, and recovery, plus optional native refusal patterns; and
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

`gang limits -c codex` starts a private [native app server](https://learn.chatgpt.com/docs/app-server), reads its current
account limits, then closes it. It creates no thread or model turn and does
not answer login, trust, or permission requests. The query uses the collar's
launch executable and environment, with the CLI's current account settings;
interactive launch arguments and `GANG_LAUNCH_ARGS` do not apply. Its cost is
native process startup plus an account request, independent of team size and
session history. The optional `limits_query` primitive declares this support.
Collars without it, including the bundled Claude Code collar, return unknown;
use `gang limits NAME` for their observed agent readings. Failed queries never
substitute cached session readings.

An upward crossing of `context_bands` sends a note from
`context-band` through the agent's normal inbox. The note states the
reading and tells the agent to save its state and run
`gang compact --resume`. A jump over
several thresholds sends a note for each; repeated readings in the same band
do not repeat it. Downward readings send nothing and allow later upward
crossings. A model change starts band tracking afresh. Once a note has been
sent, the first reading after a completed compaction is the new baseline: bands
at or below it send nothing.
Unknown context or an unknown model cannot decide a crossing.

The `context_band_crossed` lifecycle event records the band in `status`, the
native reading in `readings`, and the note's envelope and ID. Inspect it with
`gang log --type context_band_crossed`; correlate the ID with delivery events
to distinguish queued notes from verified submissions. Notes wait behind
permission prompts just like other messages. Native telemetry is authoritative
when the collar declares it; other collars use their screen context and model
readings during `gang tick`.

Status and roster observe the current native pane; an unrecognized surface is
unknown. Delivery receipts retain their original uncertainty even when the pane
later becomes idle. Queued compaction waits for a free composer without expiring.
Dropping an agent prints its observed native resume session, or explicitly says
unknown when none was recorded.
