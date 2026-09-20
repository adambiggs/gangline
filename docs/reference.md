# Reference

`gang --help` is the command inventory. `gang <command> --help` describes the
arguments, output, and failure cases for one command. This page explains the
small set of ideas needed to use that help.

## Requirements

Gangline supports macOS and Linux. It needs Bash, Python 3, tmux 3.2 or later,
and a supported native harness: Claude Code or Codex.

Run each harness directly once before starting a team. Its normal sign-in,
repository, and trust prompts belong to that harness; Gangline does not answer
them.

## Repository gate

`test/gate.sh` runs fast lint and smoke against the working tree, with one run
per host at a time and a 900-second run limit. Its final line says `PASS`,
`REFUSED`, or `UNKNOWN`. Fast lint checks files changed from `origin/main`; CI
runs the full lint and integration suites.

`test/release.sh` is the pre-release lane. It serializes on the same lock and
runs lint, smoke, and full integration.

## Start and inspect a team

Start in the repository where the agents should work:

```sh
gang up -c claude-code -m sonnet -e high
```

This opens a tmux session and attaches the current terminal to its first agent.
Use `Ctrl-b d` to detach. From a terminal outside the team, inspect it with:

```sh
gang roster
gang status lead
gang attach
```

`roster` is the overview, `status` gives one agent's current state, and
`attach` joins the team in tmux.

## Add agents and send work

Add a named native harness with `gang hitch`. The command's help lists the
available choices for its harness, working directory, model, effort, and role.

```sh
gang hitch worker -c codex -d "$PWD"
printf '%s\n' 'Inspect the failing parser tests and report the proof.' |
  gang send worker --from operator
```

Messages travel through the recipient's own terminal. Gangline reports a
message as delivered only after it sees the terminal accept it. A message that
cannot be delivered immediately is kept for a later safe opportunity; a refusal
leaves the sender responsible for it.

## Observe and control

| Need | Command |
| --- | --- |
| See every agent | `gang roster` |
| Inspect one agent | `gang status NAME` |
| Read a terminal | `gang capture NAME` |
| Stop a current turn | `gang interrupt NAME` |
| Ask an agent to compact context | `gang compact NAME` |
| Remove one agent | `gang drop NAME` |
| End a named team | `gang down SESSION` |

The state symbols are deliberately conservative:

| State | Meaning |
| --- | --- |
| `-busy-` | Gangline has evidence that the agent is working. |
| `~wait~` | The agent has work pending before its next turn. |
| `~idle~` | The agent is ready for input. |
| `!occupied!` | A native dialog owns its input. |
| `?unknown?` | Gangline cannot establish the state safely. |

Use `gang capture NAME` to see what a native dialog says. Answer a sign-in,
permission, or trust prompt in that agent's terminal; Gangline will not choose
for you.

## Exit status

An exit status of 0 means the requested operation happened. Status 3 is a
refusal: it did not happen, and stderr explains why. Status 1 is an error, so
read stderr before retrying; it does not prove that no change happened.

## Configuration

`gang config` shows the effective value and source of every persistent setting.
The environment wins over `$GANG_CONFIG_DIR/config`; the default configuration
directory is `$XDG_CONFIG_HOME/gangline`, or `~/.config/gangline`. The file is
parsed as `NAME=VALUE` lines, not sourced. Blank values, duplicates, unknown
keys, and invalid values fail the command that reads them.

| Key | Default | Value and effect |
| --- | --- | --- |
| `GANG_COLLAR` | `claude-code` | Collar selected by `up`, `hitch`, and `models` when `-c` is absent. |
| `GANG_SESSION` | `gangline` | tmux session name addressed by the command. |
| `GANG_COLLARS` | unset | Absolute directory of custom `NAME.sh` collars; these override shipped collars by name. |
| `GANG_LOCK_DIR` | `/run/user/UID/gangline`, or `/tmp/gangline-UID` without logind | Shared delivery-lock and team-record directory. Every process addressing a team must use the same absolute path. |
| `GANG_ARCHIVE_DIR` | `${XDG_STATE_HOME:-~/.local/state}/gangline/archive` | Directory that receives consumed and teardown-archived messages. Gangline does not delete these archives. |
| `GANG_NOTIFY` | `lead` | Existing agent that receives automatic blocked, failed, or stalled-state messages. The originating agent's status retains delivery failures. |
| `GANG_CONTEXT_LIGHTS` | `collar` | `off`, `collar`, `YELLOW,RED`, or whitespace-separated `COLLAR/MODEL=SPEC` entries. Thresholds are positive token counts or matching percentages below 100. The most specific selector wins and `*` is the fallback. |
| `GANG_CONTEXT_BANDS` | unset | `off` or `COLLAR/MODEL=NAME@THRESHOLD:TEMPLATE|...` entries separated by semicolons, including a `*` default. Thresholds strictly increase and use one unit: tokens or percentages. Templates may use the placeholders printed by `gang config`. |
| `GANG_CACHE_BANDS` | unset | Same grammar as context bands, using cache age rather than context use and including a `*` default. |
| `GANG_CACHE_COMPACTION` | `claude-code=3600:300 codex=1800:180` | Space-separated `COLLAR=TTL:MARGIN` or `COLLAR=off` entries. Values are whole seconds; TTL is positive and margin is smaller than TTL. |
| `GANG_AUTO_RESUME` | `off` | `off` or a percentage from `1%` through `100%`; resumes one failed Claude stream when the provider window reaches that use. |
| `GANG_SCOPE` | `off` | `on` launches each agent in its own systemd user scope; `off` uses the tmux server's cgroup. |

These environment-only settings are intentionally not accepted in the config
file:

| Variable | Default | Purpose |
| --- | --- | --- |
| `GANG_CONFIG_DIR` | described above | Absolute root containing `config`, `DOCTRINE.md`, `CONTRACT.md`, and `roles/`. |
| `GANG_ACTIVITY_WINDOW` | `5` | Seconds in which recent pane activity is credited without a second reading. |
| `GANG_ACTIVITY_LIMIT` | `300` | Seconds after which unchanged activity no longer proves a turn is live. |
| `GANG_TURN_LIMIT` | `300` | Seconds after which an unmatched native turn boundary is stale. |
| `GANG_CHURN_WAIT` | `0.5` | Seconds between the two pane reads used to prove visible activity. Fractions are allowed. |
| `GANG_BOOT_TIMEOUT` | `30` | Startup-readiness budget in seconds. |
| `GANG_GATE_LOOKS` | `60` | Maximum immediate observations of an unrecognized first-run prompt. |
| `GANG_CLEAR_PRESSES` | `40` | Maximum erase-key presses when clearing a composer. |
| `GANG_TICK_DEADLINE` | `60` | Tick worker deadline in whole seconds, from 60 through 3600. |
| `GANGLINE_REPO` | the public Git repository | Source used by `gang upgrade` and the installer. |

`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, and `XDG_STATE_HOME` relocate configuration,
event/cap history, and durable state in the standard way. `CODEX_HOME` and
`CLAUDE_CONFIG_DIR` are provider-owned locations read by their collars. Names
beginning with `GANG_TEST_`, `GANG_TICK_INTERNAL`, `GANG_TMUX_`, or
`GANGLINE_` other than `GANGLINE_REPO` are internal process/test protocol, not
operator configuration. `GANG_MODEL`, `GANG_COLLAR_FILE`,
`GANG_RUN_TEAM_ROOT`, `GANG_COMPACT_KEEP`, and `GANG_COMPACT_RESUME` are also
values passed between Gangline and a loaded collar or child process, not knobs.

Provider-cap history has its own environment because `gang cap watch` copies
selected values into a user service:

| Variable | Default | Purpose |
| --- | --- | --- |
| `GANG_CAP_THRESHOLDS` | `70,90` | Comma-separated increasing percentages that each alert once per provider window. |
| `GANG_CAP_CLAUDE_INTERVAL` | `3300` | Minimum seconds between Claude readings, because each reading spends a provider turn. |
| `GANG_CAP_NOTIFY` | unset | Shell command that receives each alert on standard input and must finish within 30 seconds. |
| `GANG_CAP_DIR` | `${XDG_DATA_HOME:-~/.local/share}/gangline/cap` | History, state, and lock directory. |
| `GANG_CAP_UNIT_DIR` | `${XDG_CONFIG_HOME:-~/.config}/systemd/user` | Alternate directory for generated timer/service files; setting it writes files without activating them. |
| `GANG_CAP_CODEX_SESSIONS` | Codex's session directory | Alternate Codex session source. |
| `GANG_CAP_CLAUDE_READER` | native Claude reader | Shell command that returns Claude's published usage row. |
| `GANG_CAP_CLAUDE_COLLAR` | shipped Claude collar | Collar file used by the native Claude reader. |

## Startup prose

| Slot | Purpose |
| --- | --- |
| `DOCTRINE.md` | The user's own standing rules, sent to every agent. Absent by default. |
| `CONTRACT.md` | The standing delivery and reporting terms sent to every agent; an operator copy overrides the shipped file. |
| `roles/NAME.md` | Replaceable role guidance selected with `gang hitch --role NAME`; operator roles override shipped roles by name. |

## Collar contract

A collar is a Bash file sourced by Gangline. It must define `GANG_LAUNCH`; all
other declarations and functions are optional. Gangline clears the complete
surface before loading each collar, validates declarations before launch, and
fails loudly on malformed values or function results it cannot interpret.

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
| `GANG_COMPACT_CMD` | Composer command; `{{instructions}}` is replaced by the continuation text. |
| `GANG_SELF_COMPACT` | `deferred` when safe self-compaction waits for a native boundary. |
| `GANG_SELF_COMPACT_WITNESS` | Empty, `unavailable`, or `native-idle`; the last requires `collar_native_idle`. |
| `GANG_USAGE_LIMIT_INTERVAL`, `GANG_USAGE_LIMIT_MAX_AGE` | Whole-second sampling interval and maximum age for provider-limit rows. |

Optional functions use exit status `0` for a positive reading, `1` for an
absent/negative reading, and `2` or `3` only where noted by the description.
Text results go to standard output.

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
| `collar_queued TARGET BODY` | Report whether BODY is in the harness's native queue; status 2 prints why it is unknown. |
| `collar_session_id TARGET PAYLOAD`, `collar_live_session_id TARGET` | Print the native session identifier from a hook payload or live harness. |
| `collar_harness_identity TARGET` | Print `PID<TAB>START_STAMP`; status 2 means unreadable. |
| `collar_last_action TARGET` | Print `at EPOCH` or `before EPOCH`; status 2 prints why it is unknown. |
| `collar_cache_stamp TARGET` | Print the epoch of the provider cache source. |
| `collar_usage_limits TARGET` | Print `LABEL<TAB>USED<TAB>RESET<TAB>OBSERVED` rows. |
| `collar_usage_limits_error STATUS` | Explain a failed usage-limit read. |
| `collar_auto_resume_record TARGET KIND` | Print the failed native turn identifier eligible for one resume. |
| `collar_submitted_prompt TARGET PAYLOAD` | Record the prompt submitted at a native boundary. |
| `collar_native_idle TARGET PAYLOAD` | Positively witness post-Stop idleness. |
| `collar_recap_boundary TARGET` | Report a current empty recap; status 3 means unreadable. |

See [Operations](operations.md) for context and recovery guidance.
