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
| `gang rename OLD NEW` | Change a registered name and window title. |
| `gang drop NAME` | Stop an agent and fail its pending messages. |
| `gang down [-y, --yes]` | Confirm on a terminal, then drop the configured team's agents and delete its runtime state and history. Use `--yes` without a terminal. |
| `gang attach` | Attach to the configured team's tmux session. |
| `gang teams` | List teams in the configured state root. |

`up` and `hitch` accept:

| Option | Meaning |
| --- | --- |
| `-c`, `--collar COLLAR` | Harness collar. |
| `-d`, `--dir DIR` | Working directory; defaults to the current directory. |
| `-m`, `--model MODEL` | Native model identifier; the native CLI judges it. |
| `-e`, `--effort EFFORT` | Native reasoning effort; requires `--model`. Refused when `gang models` lists the model without this effort. |
| `-t`, `--task TASK` | Startup assignment. |
| `-r`, `--role ROLE` | Role brief; `up` defaults to `lead`. |
| `--stdin` | Read the assignment from stdin. |
| `--resume SESSION` | Resume a native conversation by its native session ID. |
| `--recover` | Recover an agent's retained startup input. |

For collars whose transcripts Gangline reads, a resume is refused when the
native transcript stored under that session ID names a different session, or
when the ID contains path or glob characters. A session Gangline cannot check
is left to the native CLI, and `hitch` says why on stderr.
`drop` reports an observed native resume session, or says it is unknown.
When Codex asks to trust a hitch directory, startup stays queued. After
choosing the native trust option, run `gang tick` to deliver the retained
contract and assignment.

## Message and control

| Command | Effect |
| --- | --- |
| `gang send NAME [OPTIONS] [BODY]` | Send BODY, or read stdin when absent, and report its receipt. |
| `gang queue [NAME]` | List pending message IDs, recipients, and senders. |
| `gang interrupt [NAME] [-m REASON]` | Interrupt the turn; deliver an optional reason after it stops. |
| `gang compact [NAME] [--resume TEXT]` | Compact at native idle; submit the continuation behind compaction, ahead of later input. |
| `gang compact NAME --recover` | Interrupt a submitted or unconfirmed compaction with the collar's recovery keys while the pane shows it running. Refuses without sending on an approval, trust, draft, idle, or unrecognized screen; when the resume note was never queued or the harness has taken it; and after an earlier recovery of the same compaction. Records the compaction as unconfirmed and reports the screen it left. |
| `gang curfew [DURATION\|HH:MM\|RFC3339\|clear]` | Show, set, or clear the team deadline. |
| `gang tick [--agent ID]` | Check deadlines, recover native failures, and drain due messages. |
| `gang wait NAME [--timeout DURATION]` | Wait for a recorded idle boundary; a zero timeout checks once. |

The resume note enters native input when compaction starts. A synchronous
completion hook must confirm that compaction finished before the note runs.
Codex may merge later Enter steers into the same prompt, with the resume note
first; Tab queues separate follow-up turns. If the note cannot enter ahead of
an occupied composer, the command fails and withholds the note rather than
delivering it out of order. Recovery also cancels a continuation that was
published but never submitted to native input.

Send options:

| Option | Meaning |
| --- | --- |
| `--from SENDER` | Required outside registered panes; refused inside them. |
| `--live-only` | Refuse instead of queuing if the recipient cannot take input now. |
| `--supersede` | Clear this sender's older scheduled messages for this recipient first. |
| `--at DURATION\|HH:MM\|RFC3339` | Schedule delivery after a duration, at a local time, or at an exact deadline. |
| `--at clear` | Clear this sender's scheduled messages for the recipient. |

`DURATION` uses Go duration syntax (for example, `1h30m` or `500ms`) for
`--at`, `curfew`, and `--timeout`.

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
| `gang snooze [--at TIME] [--note TEXT]` | Schedule a wake for the calling agent at its observed native reset, or at an explicit future time. |
| `gang snooze --status` / `gang snooze --clear [ID]` | Inspect or cancel a pending wake. The lead sees uncertain notices and fallback wakes, and can clear one by ID. |
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
| `gang statusline [--install]` | Render native status-line JSON; `--install` fills an absent `statusLine` in `settings.json` under `CLAUDE_CONFIG_DIR` (default `~/.claude`) and names that file. |
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
| `GANG_COLLAR` | Default collar; defaults to `claude`. |
| `GANG_COLLARS` | Absolute directory of custom `NAME.cue` collars and bundled collar overlays. |
| `GANG_LAUNCH_ARGS` | JSON object mapping collar names to extra launch argument arrays. |
| `GANG_CODEX_PERMISSION_PROFILE` | Codex permission profile name for hitched agents; unset by default. |
| `GANG_CAPACITY_TIMEOUT` | Positive duration budget for provider-error continuations. |

Set `GANG_CODEX_PERMISSION_PROFILE=gangline` in the config file or environment
to select that profile for Codex hitches. The named profile must exist in
Codex's own config. When the hitch directory is a linked Git worktree, Gangline
also grants its exact worktree gitdir writable access in that profile. An unset
value adds no Codex permission arguments. Conflicting sandbox or profile
arguments supplied through Gangline are refused. Codex may load other config
layers that override the selected permission mode.

Granting a worktree gitdir lets the agent edit Git metadata, including files
that can make later host Git commands execute code. Commits also need write
access to the common Git directory, which carries the same trust cost. Enable
this only for agents trusted with that access. The grant has not yet been
verified by a native commit in a sandboxed linked worktree.

These variables are environment-only:

| Variable | Default and purpose |
| --- | --- |
| `GANG_CONFIG_DIR` | `${XDG_CONFIG_HOME:-~/.config}/gangline`; absolute config directory. |
| `GANG_STATE_ROOT` | `${XDG_STATE_HOME:-~/.local/state}/gangline`; team state root. |
| `GANG_TMUX_SOCKET` | tmux's default socket; selects a separate server when set. |
| `GANG_TMUX` | `tmux`; executable used for tmux commands. |
| `GANG_AGENT_NONCE` | Per-hitch identity capability, inherited by the harness; only its digest is stored in the registration. |
| `GANG_AGENT_ID` | Set on a hitched pane; identifies its registration together with `GANG_AGENT_NONCE` and the tmux server generation, session and pane. |

Identity and delivery use tmux's pane registration and foreground command,
so they work when the caller cannot see host PIDs. Native ancestry checks and
detached-descendant cleanup remain enabled when the host process namespace is
visible. Otherwise `process_verification_unavailable` records the skipped
checks, and `roster` shows `[process-unavailable]` until the agent's own command
passes ancestry verification from the PID namespace recorded at hitch,
which logs `process_verification_available`. The process watchdog is
also skipped visibly with `watchdog_unavailable` and `[watchdog-unavailable]`,
and `hitch` and `snooze` warn on stderr; ordinary hooks and commands still tick
the team. The next armed timer clears the marker and logs `watchdog_available`. A pane-only drop cannot
prove that detached descendants exited. Every registered pane requires a server
generation, session, pane ID, and inherited capability. Missing registration or
capability is refused; launch a fresh hitch to establish them. Later ordinary
windows do not inherit that per-hitch capability.

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
| `GANGLINE_RELEASE_BASE_URL` | GitHub release download root; override for another release host. |
| `GANGLINE_CLAUDE_STATUSLINE` | Set to `1` to install Claude Code's status line without `claude` on `PATH`. |

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
a particular harness. A collar's canonical name is the harness binary it
launches: `claude` or `codex`.
Each uses the installed native CLI and its account settings. Use
`gang models -c COLLAR` for the model and effort identifiers the native
catalog lists; the native CLI may accept models it does not list. `hitch`
reads the catalog only for `--effort`. When the catalog cannot be read,
`hitch` prints its diagnostic and launches. Gangline leaves native
permissions, login, and trust decisions to the operator. Resumed collars must
include the requested native session ID in their submit witness.

In `GANG_COLLARS`, a `NAME.cue` matching a bundled collar overlays its fields;
other names require a complete collar.
Nested fields merge; a changed primitive
name replaces that primitive and its parameters. Lists replace the bundled
list. Each `context_bands` selector value replaces its entire band list;
other selectors remain. Each `usage_bands` window replaces its band list.
For example, `claude.cue` can set only:

```cue
collar: {
	context_bands: {
		"*": [{name: "early", at: 0.10}, {name: "late", at: 0.20}]
	}
}
```

What a collar renders into the launch command (`launch`, `models.option`,
`options`, and hook `install_args`) takes effect when an agent is hitched. Gangline reads the collar again for each later operation, so edits to
its other fields apply to agents already running.

Each context band has a `name`, threshold `at` (a fraction from 0 to 1), and
optional `message`. A nonfinal band without a message advises saving state and
compacting at the next good stopping point; the last orders compaction now.
Messages use these tokens; token counts are integers and percents are rounded:

Usage bands use the same named fraction thresholds under `usage_bands.five_hour`
and `usage_bands.weekly`. They read native provider usage for the collar's
account and notify the active lead once per band and reset window. A collar
overlay can replace either window independently:

```cue
collar: {
	usage_bands: {
		five_hour: [{name: "early", at: 0.80}, {name: "full", at: 0.95}]
	}
}
```

An optional per-band `note` appends operator guidance to the measurement;
its default is empty. An optional `message` replaces the measurement template.
Both accept `{{band}}`, `{{collar}}`,
`{{window}}`, `{{threshold_percent}}`, `{{used_percent}}`,
`{{reset_at}}` (UTC RFC3339), and `{{snooze_command}}`. With no message,
the notice reports only provider, usage percent, and reset time. Unknown native
window durations are ignored; Gangline does not estimate provider usage.

| Token | Value |
| --- | --- |
| `{{band}}` | Band name. |
| `{{threshold_percent}}` | Threshold percent. |
| `{{used_tokens}}` | Used tokens. |
| `{{limit_tokens}}` | Context limit in tokens. |
| `{{used_percent}}` | Reported usage percent. |
| `{{model}}` | Reported model. |
| `{{agent_name}}` | Agent name. |
| `{{compact_command}}` | `gang compact --resume 'Resume from FILE'`. |

Claude Code context readings come from its status line. Codex readings come
from its native session log. `gang limits -c codex` queries account limits
through a private native app server without creating a conversation or turn.
The Claude Code collar has no standalone limits query; use `gang limits NAME`
for readings observed from an agent.

`gang snooze` is a manual override for an active agent. On an attributable
provider cap refusal, Gangline automatically schedules a wake at the observed
native reset. With no `--at`, `gang snooze` uses a recent
native five-hour or weekly reset from that agent's collar, selecting the
most-used window. `--at` accepts a duration, local `HH:MM`, or RFC3339
timestamp. `--note` is delivered with the wake. A pending wake is durable;
repeating the command replaces it until it is submitted. The due wake is sent
to the caller, or the active lead if the caller is gone. `--status` shows
scheduled, queued, submitted, or failed wakes. For the lead it also lists
uncertain usage notices and fallback wakes; `--clear ID` removes one of those
intents. An agent can clear its own wake with `--clear`. A wake still queued
for native input is withdrawn from the recipient's inbox; clearing cannot
retract input the harness already accepted. See
[operations](operations.md).

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
