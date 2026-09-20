# CLI and configuration reference

Use this page to look up Gangline's command surface, states, settings, and
collar interface. For the flags and failure cases of one command, run its own
help page:

```sh
gang COMMAND --help
```

Start with the [operations guide](operations.md) if you want a task-oriented
path through these commands.

## Requirements

Gangline supports macOS and Linux. It needs Git for installation, Bash, Python
3, tmux 3.2 or later, and a supported native harness. Claude Code and Codex are
first-class today.

Run each harness directly once before starting a team. Its sign-in, repository,
permission, and trust prompts stay in that harness; Gangline does not answer
them.

## Team lifecycle

| Command | Purpose |
| --- | --- |
| `gang up [NAME] [HITCH OPTIONS]` | Start a team, hitch the lead, and attach this terminal. |
| `gang hitch NAME [OPTIONS]` | Launch and register one native harness window. |
| `gang adopt NAME -c HARNESS` | Register an existing window in this team without startup prose or hooks. |
| `gang rename OLD NEW` | Change the registered name without restarting the window. |
| `gang drop NAME` | Archive waiting messages and remove one agent window. |
| `gang down SESSION` | Archive waiting messages and end one named team. |

`gang hitch` accepts `-c/--collar`, `-d/--dir`, `-m/--model`, `-e/--effort`,
`-t/--task`, `-r/--role`, `-l/--lights`, `--resume`, and `--stdin`. Names may
contain letters, digits, `.`, `-`, and `_`; they cannot start with `.` or `-`.
Run `gang hitch --help` before scripting the launch path.

`gang drop` prints the native session identifier and a resume command when the
collar supports recovery. Capture terminal-only evidence before dropping the
window.

## Messages and execution

| Command | Purpose |
| --- | --- |
| `gang send NAME [--from SENDER] [--live-only] [--supersede]` | Deliver the stdin body or park it for a safe boundary. |
| `gang send NAME --at TIME [--from SENDER]` | Hold the stdin body until a duration or local `HH:MM` time passes. |
| `gang send NAME --at clear` | Cancel that recipient's timed messages. |
| `gang run -- COMMAND [ARG ...]` | Run a host command for the calling agent and send back its result. |
| `gang run --active` | List this agent's active host runs and recovery commands. |
| `gang run --cancel RUN_ID` | Cancel one host run owned by this agent. |
| `gang queue [NAME]` | Read queued messages; an agent consumes its own queue. |
| `gang flush [NAME]` | Recover an exactly matched body from a harness-owned input queue. |
| `gang interrupt [NAME] [-m REASON] [--from SENDER]` | Send the collar's stop key and optionally deliver a reason afterward. |
| `gang compact [NAME] [--resume TEXT]` | Request native compaction and a continuation turn. |
| `gang compact NAME --recover` | Apply the collar's recovery keys to a stuck compaction surface. |

`gang send` reads its body from a pipe, file, heredoc, or editor. Inside a
registered window, the sender comes from tmux. Outside the team, `--from` is
required.

`gang run` starts outside the requesting harness's sandbox. It inherits the
agent's working directory and command environment, stores combined output in
durable state, and sends a bounded result back when the command exits.

## Observation and control

| Command | Purpose |
| --- | --- |
| `gang roster [--porcelain]` | Show every window, collar, live state, queued work, and provenance. |
| `gang status [NAME] [--why]` | Show one agent's detailed state or explain how the state was derived. |
| `gang tick` | Run one bounded synchronous retry pass. |
| `gang wait NAME --until idle\|done [--timeout SECONDS]` | Block an outside shell on a native boundary. |
| `gang wait --limit [NAME] [--resume TEXT]` | Schedule a continuation after a provider reset. |
| `gang wait --limit [NAME] --clear` | Remove that scheduled continuation. |
| `gang capture [NAME] [LINES]` | Print recent pane content. |
| `gang capture --composer [NAME]` | Print the native input composer. |
| `gang context [NAME]` | Print the harness's native context reading. |
| `gang log [NAME] [--since N] [--kind KIND]` | Read durable JSONL events for the team or one agent. |
| `gang limits [NAME]` | Read current provider usage windows through a collar. |
| `gang limits --history` | Read retained account-window samples and pace. |
| `gang usage [--daily [DATE]\|--since DATE]` | Join live agents to local token attribution from `ccusage`. |
| `gang cap` | Read retained weekly provider-window history. |
| `gang cap check` | Sample provider windows now and emit threshold alerts. |
| `gang cap watch [--clear]` | Install or remove periodic sampling. |
| `gang cap forget` | Delete the provider-window history Gangline wrote. |
| `gang whoami` | Print the calling pane's registered identity. |
| `gang attach` | Attach to the configured team, including its recorded socket. |
| `gang teams` | List recorded teams and their sockets. |

`gang roster --porcelain` prints tab-separated fields in this order: name,
collar, state, queued count, oldest queued age in seconds, native session ID,
hitcher state, and hitcher name.

`gang wait` is for an operator shell or external script. It refuses inside an
agent window because team reports already arrive at native turn boundaries.

## Discovery, settings, and upgrades

| Command | Purpose |
| --- | --- |
| `gang curfew` | Show the team deadline. |
| `gang curfew DURATION\|HH:MM` | Set or replace the team deadline. |
| `gang curfew clear` | Remove the team deadline. |
| `gang collars` | List harness collars and their resume capability. |
| `gang models [-c HARNESS]` | List native model identifiers and effort values. |
| `gang roles` | List usable and invalid role briefs with their source. |
| `gang config` | Print each persistent setting, its effective value, and source. |
| `gang --version` | Print the installed release version. |
| `gang upgrade --check` | Compare the installed release with the latest stable tag. |
| `gang upgrade` | Install the latest stable release over an installer-managed tree. |

Ordinary commands do not check the network for updates. Source checkouts update
with Git; `gang upgrade` is for installer-managed release trees.

## States and exit status

The state symbols are conservative:

| State | Meaning |
| --- | --- |
| `-busy-` | Native evidence says the agent is working. |
| `~wait~` | Work is pending before the next turn. |
| `~idle~` | The native composer is ready for input. |
| `!occupied!` | A native dialog owns the input surface. |
| `!dead!`, `!bricked!`, `!blocked!` | The window cannot take the expected next turn; inspect its status. |
| `!harness-lost!`, `!session-lost!` | The registered native identity no longer matches a usable session. |
| `?unknown?` | Gangline cannot establish the state safely. |

Exit status `0` means the command completed its documented operation. For
`send`, it can mean Gangline accepted the body into its queue, not that the
native session has read it yet.

Exit status `3` is a refusal made before the operation. Stderr says what was
refused. Exit status `1` is an error and does not prove that nothing changed.
Status `4` reports a command-specific native condition that did not settle,
such as an unreadable roster row or an unanswered startup gate. Status `5`
means delivery keystrokes landed but Gangline lost verification; inspect the
recipient before retrying.

## Configuration

`gang config` shows the effective value and source of every persistent setting.
The environment wins over `$GANG_CONFIG_DIR/config`; the default configuration
directory is `$XDG_CONFIG_HOME/gangline`, or `~/.config/gangline`. The file is
parsed as `NAME=VALUE` lines, not sourced. Blank values, duplicates, unknown
keys, and invalid values fail the command that reads them.

| Key | Default | Value and effect |
| --- | --- | --- |
| `GANG_COLLAR` | `claude-code` | Collar selected by `up`, `hitch`, and `models` when `-c` is absent. |
| `GANG_SESSION` | `gangline` | tmux session addressed by the command. |
| `GANG_COLLARS` | unset | Absolute directory of custom `NAME.sh` collars; these override shipped collars by name. |
| `GANG_LOCK_DIR` | `/run/user/UID/gangline`, or `/tmp/gangline-UID` without logind | Shared delivery-lock and team-record directory. Every process addressing a team must use the same absolute path. |
| `GANG_ARCHIVE_DIR` | `${XDG_STATE_HOME:-~/.local/state}/gangline/archive` | Directory that receives consumed and teardown-archived messages. Gangline does not delete these archives. |
| `GANG_NOTIFY` | `lead` | Existing agent that receives automatic blocked, failed, or stalled-state messages. The originating agent's status retains delivery failures. |
| `GANG_CONTEXT_LIGHTS` | `collar` | `off`, `collar`, `YELLOW,RED`, or whitespace-separated `COLLAR/MODEL=SPEC` entries. Thresholds are positive token counts or matching percentages below 100. The most specific selector wins and `*` is the fallback. |
| `GANG_CONTEXT_BANDS` | unset | `off` or `COLLAR/MODEL=NAME@THRESHOLD:TEMPLATE\|...` entries separated by semicolons, including a `*` default. Thresholds strictly increase and use one unit: tokens or percentages. Templates may use the placeholders printed by `gang config`. |
| `GANG_CACHE_BANDS` | unset | Same grammar as context bands, using cache age rather than context use and including a `*` default. |
| `GANG_CACHE_COMPACTION` | `claude-code=3600:300 codex=1800:180` | Space-separated `COLLAR=TTL:MARGIN` or `COLLAR=off` entries. Values are whole seconds; TTL is positive and margin is smaller than TTL. |
| `GANG_AUTO_RESUME` | `off` | `off` or a percentage from `1%` through `100%`; resumes one failed Claude stream when the provider window reaches that use. |
| `GANG_SCOPE` | `off` | `on` launches each agent in its own systemd user scope; `off` uses the tmux server's cgroup. |

These runtime variables are intentionally not accepted in the config file:

| Variable | Default | Purpose |
| --- | --- | --- |
| `GANG_CONFIG_DIR` | described above | Absolute root containing `config`, `DOCTRINE.md`, `CONTRACT.md`, and `roles/`. |
| `GANG_ACTIVITY_WINDOW` | `5` | Seconds in which recent pane activity is credited without a second reading. |
| `GANG_ACTIVITY_LIMIT` | `300` | Seconds after which unchanged activity no longer proves a turn is live. |
| `GANG_TURN_LIMIT` | `300` | Seconds after which an unmatched native turn boundary is stale. |
| `GANG_CHURN_WAIT` | `0.5` | Seconds between pane reads used to prove visible activity. Fractions are allowed. |
| `GANG_BOOT_TIMEOUT` | `30` | Startup-readiness budget in seconds. |
| `GANG_GATE_LOOKS` | `60` | Maximum immediate observations of an unrecognized first-run prompt. |
| `GANG_CLEAR_PRESSES` | `40` | Maximum erase-key presses when clearing a composer. |
| `GANG_TICK_DEADLINE` | `60` | Tick worker deadline in whole seconds, from 60 through 3600. |
| `GANGLINE_REPO` | public Git repository | Source used by `gang upgrade` and the installer. |

`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, and `XDG_STATE_HOME` relocate configuration,
capacity history, and durable state. `CODEX_HOME` and `CLAUDE_CONFIG_DIR` are
provider-owned locations read by their collars. Names beginning with
`GANG_TEST_`, `GANG_TICK_INTERNAL`, `GANG_TMUX_`, or `GANGLINE_` other than
`GANGLINE_REPO` are internal protocol, not operator configuration.
`GANG_MODEL`, `GANG_COLLAR_FILE`, `GANG_RUN_TEAM_ROOT`, `GANG_COMPACT_KEEP`, and
`GANG_COMPACT_RESUME` are passed between Gangline and a collar or child process;
they are not settings.

The installer accepts three location overrides:

| Variable | Default | Purpose |
| --- | --- | --- |
| `GANGLINE_REPO` | public Git repository | Release-tag source used for installation and upgrades. |
| `GANGLINE_HOME` | `~/.local/share/gangline` | Installer-managed release tree. |
| `GANGLINE_BIN` | `~/.local/bin` | Directory that receives the `gang` command link. |

Provider-cap history has its own environment because `gang cap watch` copies
selected values into a user service:

| Variable | Default | Purpose |
| --- | --- | --- |
| `GANG_CAP_THRESHOLDS` | `70,90` | Comma-separated increasing percentages that alert once per provider window. |
| `GANG_CAP_CLAUDE_INTERVAL` | `3300` | Minimum seconds between Claude readings; each reading spends a provider turn. |
| `GANG_CAP_NOTIFY` | unset | Shell command that receives each alert on stdin and must finish within 30 seconds. |
| `GANG_CAP_DIR` | `${XDG_DATA_HOME:-~/.local/share}/gangline/cap` | History, state, and lock directory. |
| `GANG_CAP_UNIT_DIR` | `${XDG_CONFIG_HOME:-~/.config}/systemd/user` | Alternate directory for generated timer and service files; setting it writes files without activating them. |
| `GANG_CAP_CODEX_SESSIONS` | Codex session directory | Alternate Codex session source. |
| `GANG_CAP_CLAUDE_READER` | native Claude reader | Shell command that returns Claude's published usage row. |
| `GANG_CAP_CLAUDE_COLLAR` | shipped Claude collar | Collar file used by the native Claude reader. |

## Startup prose

| Slot | Purpose |
| --- | --- |
| `DOCTRINE.md` | Optional operator-owned rules sent to every agent. |
| `CONTRACT.md` | Standing delivery and reporting terms; an operator copy overrides the shipped file. |
| `roles/NAME.md` | Replaceable role guidance selected with `gang hitch --role NAME`; operator roles override shipped roles by name. |

The lead is the fixed first role. Treat non-lead role briefs as starter material
and replace them with the operator's own division of work. Keep operator policy
in `DOCTRINE.md`, not in the Gangline core.

## Collar contract

A collar is a Bash file sourced by Gangline. It must define `GANG_LAUNCH`; all
other declarations and functions are optional. Gangline clears the complete
surface before loading each collar, validates declarations before launch, and
fails on malformed values or function results it cannot interpret.

| Declaration | Meaning |
| --- | --- |
| `GANG_LAUNCH` | Required shell command for a fresh harness session. |
| `GANG_RESUME_LAUNCH` | Resume command containing exactly one `{{session_id}}`. |
| `GANG_MODEL_OPT`, `GANG_MODEL_ALIASES` | Model option spelling and newline-separated aliases. |
| `GANG_EFFORT_OPT`, `GANG_EFFORT_CMD` | Effort option spelling and command that prints the accepted vocabulary. |
| `GANG_ROLE_PROMPT_OPT` | Harness option that takes the contract, collar guidance, and role brief as one value. |
| `GANG_HARNESS_PROMPT` | Short harness-specific guidance attached at launch. |
| `GANG_BUSY_REGEX`, `GANG_OCCUPIED_REGEX`, `GANG_QUEUED_REGEX` | Extended regular expressions for visible busy, dialog, and queued-input states. |
| `GANG_QUEUE_RECALL_KEY`, `GANG_INTERRUPT_KEY` | tmux key names for recalling queued input and interrupting a turn. |
| `GANG_COMPACT_RECOVER_KEYS` | Space-separated tmux key names used to recover a refused compaction surface. |
| `GANG_STOP_HOOK` | `1` when the launch installs Gangline's native Stop hook. |
| `GANG_STALL_TYPES` | Space-separated native notification kinds that represent a stalled harness. |
| `GANG_QUIET_AT_REST` | `1` when absent pane activity is meaningful at rest. |
| `GANG_MIDTURN_INPUT` | Empty, `1`, `park`, or `steer`, describing native mid-turn input support. |
| `GANG_COMPACT_CMD` | Composer command; `{{instructions}}` is replaced by continuation text. |
| `GANG_SELF_COMPACT` | `deferred` when safe self-compaction waits for a native boundary. |
| `GANG_SELF_COMPACT_WITNESS` | Empty, `unavailable`, or `native-idle`; the last requires `collar_native_idle`. |
| `GANG_USAGE_LIMIT_INTERVAL`, `GANG_USAGE_LIMIT_MAX_AGE` | Whole-second sampling interval and maximum age for provider-limit rows. |

Optional functions return `0` for a positive reading, `1` for an absent or
negative reading, and `2` or `3` only where the description says so. Text
results go to stdout.

| Function | Contract |
| --- | --- |
| `collar_models` | Print supported model rows. |
| `collar_model_check MODEL` | Return recognized, unrecognized, or unknown. |
| `collar_context_lights MODEL` | Print the collar's default `YELLOW,RED` thresholds. |
| `collar_context TARGET` | Print `used<TAB>window`; return 3 when the native source is unreadable. |
| `collar_input TARGET` | Print the current composer; return 3 when the pane cannot be read. |
| `collar_overlay TARGET` | Print the visible overlay title. |
| `collar_advisory TARGET`, `collar_dismiss_advisory TARGET` | Detect an advisory and, when safe, dismiss it while naming the action. |
| `collar_bricked TARGET`, `collar_blocked TARGET` | Print the reason for a fatal or blocked completed turn; status 2 means unknown. |
| `collar_queued TARGET BODY` | Report whether `BODY` is in the harness queue; status 2 explains an unknown reading. |
| `collar_session_id TARGET PAYLOAD`, `collar_live_session_id TARGET` | Print the native session identifier from a hook payload or live harness. |
| `collar_harness_identity TARGET` | Print `PID<TAB>START_STAMP`; status 2 means unreadable. |
| `collar_last_action TARGET` | Print `at EPOCH` or `before EPOCH`; status 2 explains an unknown reading. |
| `collar_cache_stamp TARGET` | Print the epoch of the provider cache source. |
| `collar_usage_limits TARGET` | Print `LABEL<TAB>USED<TAB>RESET<TAB>OBSERVED` rows. |
| `collar_usage_limits_error STATUS` | Explain a failed usage-limit read. |
| `collar_auto_resume_record TARGET KIND` | Print the failed native turn identifier eligible for one resume. |
| `collar_submitted_prompt TARGET PAYLOAD` | Record the prompt submitted at a native boundary. |
| `collar_native_idle TARGET PAYLOAD` | Positively witness post-Stop idleness. |
| `collar_recap_boundary TARGET` | Report a current empty recap; status 3 means unreadable. |

See [Operations](operations.md) for recovery procedures and
[Architecture](architecture.md) for the boundary between collars and the core.
