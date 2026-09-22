# CLI and collar reference

`gang help` is the executable command inventory. `gang COMMAND --help` prints
the accepted shape for one command.

## Requirements and installation

Gangline supports macOS and Linux and requires Git, Go 1.27 or later, tmux 3.2
or later, and Claude Code or Codex. The installer selects the newest stable
`gangline-v*` tag, retains that checkout for inspectable upgrades, and builds a
static binary with `CGO_ENABLED=0`.

```sh
curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
```

`GANGLINE_HOME` changes the retained checkout, `GANGLINE_BIN` changes the binary
directory, and `GANGLINE_REPO` changes the release source. Defaults are
`~/.local/share/gangline`, `~/.local/bin`, and the public Git repository.

## Commands

### Team lifecycle

| Command | Effect |
| --- | --- |
| `gang up [NAME] [HITCH OPTIONS]` | Hitch the first agent, named `lead` by default, and attach when stdin is a terminal. |
| `gang hitch NAME [OPTIONS]` | Launch a harness window and deliver its contract, role, and assignment. |
| `gang adopt NAME -c COLLAR` | Register the current tmux pane without launch or startup delivery. |
| `gang rename OLD NEW` | Rename an active hitch and its tmux window. |
| `gang drop NAME` | Stop one active or failed hitch and visibly fail its pending sends. |
| `gang down SESSION` | Drop every active or failed hitch and remove that team's v1 state directory. |

Hitch options are `-c/--collar`, `-d/--dir`, `-m/--model`, `-e/--effort`,
`-t/--task`, `-r/--role`, `--resume`, and `--stdin`. Effort
requires an explicit model. `--stdin` reads the assignment body from stdin;
otherwise `--task` supplies it.

`up` defaults the role to `lead` and the working directory to the caller's
current directory. An explicit `--role` or `--dir` overrides that default.
A safely deferred assignment to a ready, live recipient stays pending until
native acceptance or recipient drop. Attempts back off from 100ms to a 30s cap;
time does not expire the message. The same policy covers queued sends and
compaction continuations. Startup readiness retains its deadline while no native
process answers. Hook settings are launch-time configuration; changing the binary
does not replace arguments captured by an existing harness.
A failed hitch retains its name until `gang drop NAME` removes that generation;
another hitch or rename cannot reuse it first.

### Delivery and control

| Command | Effect |
| --- | --- |
| `gang send NAME [--from NAME] [--live-only] [--supersede] [--at TIME]` | Read a body from stdin and deliver or queue it. |
| `gang send NAME --at clear` | Clear timed deliveries for the recipient. |
| `gang queue [NAME]` | List queued delivery ID, recipient, and sender rows. |
| `gang interrupt [NAME] [-m REASON]` | Interrupt an active or wedged native turn. |
| `gang compact [NAME] [--resume TEXT]` | Submit the collar's compaction action and queue a continuation. |
| `gang compact NAME --recover` | Apply the collar's declared compaction-recovery actions. |

Inside an active Gangline pane, `send` derives the sender from the pane and
refuses an overriding `--from`. Outside the team, `--from` is required and the
wire envelope marks it `self-declared:`.

The maximum message size is 1 MiB for the JSON-encoded rendered envelope,
including attribution and escaping. Refusal is on stderr and directs the sender
to put details in a state file and send its path; input is never split or
truncated. Native hook objects have a separate 2 MiB budget for the prompt,
paste wrapper, and metadata. A failed or oversized pending hook receipt leaves
the delivery unverified instead of waiting indefinitely.

`--live-only` requires recorded idle or a busy recipient with mid-turn support. `--supersede`
clears older timed work for that recipient before sending. `--at` accepts a
duration such as `45m` or a local `HH:MM` time.

### Observation and recovery

| Command | Effect |
| --- | --- |
| `gang roster [--porcelain]` | List active hitches and recorded activity. |
| `gang status [NAME] [--why]` | Show one hitch and any recorded wedge evidence. |
| `gang capture [NAME] [LINES]` | Render a parsed pane screen. |
| `gang capture --composer [NAME]` | Read the collar-recognized composer. |
| `gang context [NAME]` | Read the latest recorded native context use and active collar band; unknown readings exit 5. |
| `gang context --widget NAME\|off` | Show one agent’s context in this team’s tmux status line, or restore the prior session setting. |
| `gang statusline [--install]` | Render Claude’s native stdin payload; `--install` repairs absent or retired Gangline status-line settings. |
| `gang limits [NAME]` | Read the latest native provider-limit readings; unsupported or unavailable readings exit 5. |
| `gang log [--agent NAME\|HITCH_ID] [--type TYPE\|KIND]` | Print the configured team’s authoritative JSONL log, optionally filtered. |
| `gang replay [--agent NAME\|HITCH_ID] [--type TYPE\|KIND] [EVENTS.jsonl]` | Without filters, fold a file/stdin log into state JSON. With filters, emit matching event JSONL using the full history for attribution. |
| `gang tick` | Retry pending effects, release safe queued delivery, and observe wedges. |
| `gang wait NAME [--timeout DURATION]` | Block on event-log appends until the agent is idle; defaults to 30 seconds and `--timeout 0` checks once. |
| `gang whoami` | Print the active hitch bound to the current pane. |
| `gang teams` | List team directories with event logs in the v1 state root. |
| `gang attach` | Attach to the configured tmux team. |

Gangline projects the recorded hitch state into each managed tmux window name:
`?name?` is starting, booting, or dropping; `~name~` is idle; `-name-` is
working; and `!name!` is blocked, wedged, or failed. `gang tick` repairs a
window name that drifted from the event log.

### Discovery and installation

| Command | Effect |
| --- | --- |
| `gang collars` | List embedded and operator CUE collars. |
| `gang collar check NAME` | Probe an installed harness on disposable private tmux sockets. |
| `gang models [-c COLLAR]` | Ask the collar's native catalog for models and efforts. |
| `gang roles` | List shipped and operator role briefs. |
| `gang config` | Print effective settings and their source. |
| `gang curfew [DURATION\|HH:MM\|clear]` | Show, set, or clear the recorded team deadline. |
| `gang --version` | Print the built release version. |
| `gang upgrade [--check]` | Run the retained release installer's upgrade path. |

## Exit status

| Status | Meaning |
| --- | --- |
| `0` | The documented operation completed; a send may be durably queued. |
| `1` | An execution or I/O error occurred. |
| `2` | Command usage was invalid. |
| `3` | Gangline refused before the requested state change. |
| `4` | A native condition needs attention, such as a startup prompt. |
| `5` | The result is unknown, including delivery typed without a matching witness. |

## Configuration and state

The config file is `$GANG_CONFIG_DIR/config`, defaulting to
`${XDG_CONFIG_HOME:-~/.config}/gangline/config`. It is parsed as strict
`NAME=VALUE` data; it is never sourced. Environment values override file values.

| Setting | Default | Purpose |
| --- | --- | --- |
| `GANG_SESSION` | `gangline` | Team and tmux session name. |
| `GANG_COLLAR` | `claude-code` | Default collar for launch and model discovery. |
| `GANG_COLLARS` | unset | Absolute directory of operator `NAME.cue` collars. |
| `GANG_LAUNCH_ARGS` | unset | JSON object of collar names to extra launch-argument arrays. |

Runtime-only variables are `GANG_CONFIG_DIR`, `GANG_STATE_ROOT`, `GANG_TMUX`,
and `GANG_TMUX_SOCKET`. The state default is
`${XDG_STATE_HOME:-~/.local/state}/gangline`; tmux uses its default socket unless
an explicit socket is supplied. `GANG_TMUX` selects the tmux executable for
tests and native collar probes.

Each team is stored at `STATE_ROOT/v1/TEAM/`. `events.jsonl` is authoritative;
`snapshot.json` records an integrity-checked checkpoint. `gang down SESSION`
removes that directory after every active or failed hitch is dropped.

## Startup prose

Gangline embeds its shipped contract and role briefs from `internal/prose/`.
Operator files under `GANG_CONFIG_DIR` override them:

- `CONTRACT.md` supplies standing terms;
- `DOCTRINE.md` adds optional operator guidance; and
- `roles/NAME.md` replaces or adds a role brief.

The collar's `role_prompt` option puts standing prose into a native system
prompt when the harness supports it. The startup envelope still carries the
assignment.

## CUE collar contract

A custom collar is a CUE file with a top-level `collar` value validated against
`harness/schema/collar.cue`. Its filename and `collar.name` must agree.

The value declares:

- `launch`: command, normal/resume/probe arguments, and environment;
- `hooks`: launch arguments containing `{{hook.command.json}}`, plus native
  event and payload mappings; `{{statusline.command.json}}` renders the installed
  binary’s status-line callback;
- `models`: catalog and selected-model primitives and the model option;
- `options`: optional effort and role-prompt argument templates;
- `primitives`: optional `mid_turn` capability (false when absent), startup, composer, submit, submit witness, turn boundary,
  runtime blocked, context, provider limits, and wedge operations. Optional `telemetry`
  selects `claude-status-line` or `codex-session-log` for structured observations;
- `actions`: interrupt, compact, and recovery; and
- `context_bands`: ordered named thresholds per model selector.

Logic stays in committed Go primitives. A collar selects and parameterizes
those primitives; unknown primitive names fail during collar validation.
Shipped collars are embedded CUE values under `harness/collars/` and pass
through the same loader as operator collars.

A send to a busy recipient uses native mid-turn submission when its collar
advertises `primitives.mid_turn`. It reports delivered only after native
submission is verified, while the recipient remains busy. Collars without this
capability keep sends queued until idle. The sending command waits through deferred native submissions until acceptance
or recipient drop; `gang tick` never retypes an abandoned in-flight
submission whose outcome is unknown.

`GANG_CAPACITY_TIMEOUT` is a positive duration (default `5m`) accepted in the
operator configuration and environment. It bounds provider-capacity recovery
owned by `gang tick`, using exponential delays from `100ms` to `30s`. It does
not set a user-message expiry. The collar's optional `capacity` primitive
recognizes terminal provider failures; native busy/retry surfaces remain busy.

`GANG_DELIVERY_TIMEOUT` is no longer accepted: live sends do not have an expiry
setting. Existing pending events can contain older deadlines; those timestamps
do not expire their delivery. Unknown input outcomes remain unverified and are
not retried automatically.

Native hook invocations appear in the event log as `native_hook` records. The
`id` pairs receipt (`received`) with its outcome (`completed`, `ignored`, or
`failed`); `native_event` names the harness event and `hitch_id` attributes it.
Failures carry `reason`. These observations preserve hook evidence without
changing replayed lifecycle state. A normal contended hook waits for the event
writer; a late hook after recipient drop records an ignored outcome.


### Structured observations

Both shipped collars append `observation` events to the team log. Each event
identifies its hitch, collar, native session, and a batch of `readings`. A reading
has `kind`, `source`, and `status`; optional `at` is the native measurement time,
while the enclosing event's `at` is Gangline's collection time. Missing native
timestamps stay absent. Status is `observed`, `unknown`, or `error`, with a
`reason` for missing or malformed evidence. Context uses `used`, `limit`, and
`percent` (0–100 scale); provider limits use labelled `limits` with percentages
and Unix `reset_at` seconds. Context may exceed the nominal limit.

Sources are `native-hook`, `session-log`, `status-line`, and `screen`. Kinds
include `turn-started`, `turn-finished`, `compaction-started`,
`compaction-finished`, `compaction-checkpoint`, `context`, `provider-limits`,
`error`, `blocked`, `model`, and `activity`. A native session checkpoint is evidence of
compaction, not a second completion event. Hooks and session logs can witness
the same turn or compaction: select one source when counting, and preserve the
other as corroboration. Gangline's delivery intents/outcomes, dialog transitions,
and requested compactions retain their existing event types. Startup trust is
recorded as a screen observation; `hitch_ready` is its directly observed
clearance. Observation events do not cause lifecycle transitions.

Claude's collar installs the absolute `gang statusline` command for new hitches.
The installer replaces an absent status line or the retired
`statusline/claude-code-context.sh` command in `~/.claude/settings.json`, preserving
unrelated custom settings. Existing hitches keep their launch settings until
re-hitched. Standalone Claude invocations can render without a Gangline identity.
Context is the native input plus cache-read and cache-creation tokens. When the
payload supplies `transcript_path`, matching latest assistant usage corroborates
its measurement timestamp. Post-compaction readings remain unknown until a
provably newer measurement arrives; an unversioned delayed callback cannot make
old context current. Payload fields follow the native
[status-line contract](https://code.claude.com/docs/en/statusline).

Codex context and limits come from `event_msg.token_count` in the hook-provided
session log. Context uses `last_token_usage.total_tokens` and
`model_context_window`, never cumulative session usage. Native `turn_context` records supply the model
for context bands. Session metadata must
match the hook's `session_id`; changed paths and truncated logs fail visibly.
Complete-record cursors commit with their readings in one event. Earlier history
on resume and incomplete final records are not current hitch evidence. Hooks,
`gang tick`, `gang context`, and `gang limits` collect known transcripts; there
is no watcher. Events without a subsequent callback become visible at the next
explicit collection. This format is a native implementation surface, not a
stable hook API; malformed recognized records fail loudly, as described in the
native [hook documentation](https://developers.openai.com/codex/hooks).

Turn and compaction hooks record a snapshot of the latest available context and
limits with their original source/time. This does not assert that a measurement
was taken at that boundary. Missing readings stay unknown. Claude's terminal
`StopFailure` hook records the provider error and turn end; Codex session-log
errors are observations and do not themselves manufacture a turn end. Unsupported
provider windows remain unknown. Readings are projected atomically to
`readings/HITCH_ID.json` under the team directory and rebuilt from the log if
removed. `gang down` deletes observations, readings, cursors, and widget state.

Filters match event types or reading kinds, for example
`gang log --agent worker --type context`. Agent names refer to their name at the
time of each event; use the immutable hitch ID across renames. The agent filter targets the recipient of delivery events. Delivery, timeout, and
compaction outcomes are joined through the full history. A filtered observation
contains only matching readings. Filtered output is an evidence projection, not
a complete log suitable for reconstructing team state.

The optional widget appends a context value to this team's session `status-right`.
It updates from callbacks for the selected hitch. Global tmux options and user
configuration files are untouched. Disabling restores the previous local value
or inheritance; if the operator has since changed `status-right`, Gangline
refuses to overwrite that change.
