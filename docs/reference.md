# Command and environment reference

`gang --help` is the authoritative command synopsis.

## Lifecycle

### `gang up [name] [hitch flags]`

Hitches one agent, named `lead` when omitted, then attaches or switches the
current tmux client to it. `GANG_PROFILE` selects the harness.

### `gang hitch <name> [-p profile] [-d dir] [-m model] [--resume]`

Starts a native harness in a named tmux window and delivers one startup contract.
The launch environment carries the exact `GANG_SESSION` plus any custom profile
and lock paths, so harness commands cannot drift to another session on the same
tmux server.

- `-p` selects a profile.
- `-d` selects the harness working directory.
- `-m` passes a harness-native model choice through the profile's model option.
- `--resume` uses the profile's directory-scoped native resume command. Profiles
  without a safe resume command refuse it.

Names use letters, digits, dot, dash, and underscore, may not begin with dot or
dash, and must be unique in the team. `hitch` is reserved as the startup-envelope
sender.

When context lights are enabled, their thresholds are copied to the new window.
Codex also binds that window to the useful startup envelope's nonce so later
token events can be found without a marker turn.

### `gang adopt <name> -p <profile>`

Registers an existing window in `GANG_SESSION`. Adoption does not inject startup
text or retroactively add launch-time native hooks. A profile whose context
source requires hitch-time identity may therefore report context unavailable.

### `gang attach`

Attaches to `GANG_SESSION`.

### `gang drop <name>`

Kills the exact agent window. Its tmux-owned state dies with it.

### `gang down`

Kills the exact team session and every window in it.

## Delivery and compaction

### `gang send --to <name> [--from <sender>] --stdin`

Reads the full message body from standard input. Inside the team, Gangline derives
the sender from the calling window and refuses `--from`. Calls from outside the
team must supply `--from`. Gangline wraps the body in a nonce-bound envelope,
serializes writers per pane, verifies the paste changed the target composer,
submits it, and reports success only after verification.

Gangline refuses a missing or occupied composer, a human draft, indeterminate
state, and unsafe mid-turn input. A profile may declare that its native harness
accepts ordinary mid-turn input.

### `gang compact <name>`

Submits the profile's native compaction command through the same verified input
path. An external request refuses a busy or indeterminate target.

When a Codex agent requests its own compaction, Gangline records a one-shot
request. Its native Stop hook submits `/compact` after the active turn releases
the composer. `status` and `roster` expose pending or failed self-compaction.

## Observation

### `gang status <name>`

Prints one current state:

- `busy (tight tug)` — a native event, terminal activity, or profile marker
  witnesses active work;
- `idle (slack tug)` — the evidence positively witnesses readiness;
- `occupied (authority unknown)` — a native UI owns the composer;
- `expired (...)` — the available evidence can no longer determine the answer.

It also reports staged input and pending or failed self-compaction.

### `gang roster`

Prints every session window with its profile and current state. Unadopted windows
are shown but not treated as agents. Context usage belongs to each agent and is
not reported to the lead or operator.

### `gang capture <name> [lines]`

Prints the tail of the target pane after trimming trailing blank terminal rows.

### `gang wait <name> [timeout_seconds]`

Waits until the target is positively idle, occupied, or indeterminate.
This is an operator command; mandatory tests do not exercise its wall-clock
behavior.

## Discovery and hooks

### `gang profiles`

Lists shipped and custom harness profiles. The Bash substrate fixture is hidden
unless `GANG_TEST_PROFILES=1`.

### `gang hook`

Internal endpoint for native harness events. It reads one JSON payload from
standard input. Prompt/tool events open the turn fact, Stop closes it and may
dispatch deferred self-compaction, and permission requests raise occupancy.
Hooks are silent unless an enabled context
light crosses an edge.

## Environment

| Variable | Meaning |
|---|---|
| `GANG_PROFILE` | default profile for `up` and `hitch` |
| `GANG_SESSION` | exact tmux session Gangline addresses |
| `GANG_CONTEXT_LIGHTS` | `off`, or absolute `yellow,red` token thresholds |
| `GANG_PROFILES` | custom profile directory searched before shipped profiles |
| `GANG_BOOT_TIMEOUT` | harness startup readiness bound |
| `GANG_LOCK_DIR` | shared per-pane delivery-lock directory |

Operational evidence bounds also use `GANG_CHURN_WAIT`,
`GANG_ACTIVITY_WINDOW`, `GANG_ACTIVITY_LIMIT`, `GANG_TURN_LIMIT`,
and `GANG_OCCUPIED_LIMIT`. Their defaults live once in
`bin/gang`; change them only with evidence about the native harness surface.

Every process addressing one team must agree on `GANG_SESSION`, `GANG_PROFILES`,
and `GANG_LOCK_DIR`.

## Profile contract

A profile is a Bash file sourced by `bin/gang`. Harness-specific behavior belongs
there, never in a harness-name branch in the core script.

| Declaration | Purpose |
|---|---|
| `GANG_LAUNCH` | required native launch command |
| `GANG_RESUME_LAUNCH` | optional safe native resume command |
| `GANG_MODEL_OPT` | optional native model flag |
| `GANG_BUSY_REGEX` | pane evidence of an active turn |
| `GANG_OCCUPIED_REGEX` | pane evidence that a native UI owns input |
| `GANG_QUIET_AT_REST=1` | harness terminal becomes quiet when idle |
| `GANG_MIDTURN_INPUT=1` | ordinary text may safely enter during a turn |
| `GANG_COMPACT_CMD` | native compaction command |
| `GANG_SELF_COMPACT=deferred` | self-compaction must wait for Stop |
| `GANG_SESSION_KEY=1` | context lookup needs the startup-envelope nonce |
| `profile_input target` | print human-authored composer contents, or fail if absent |
| `profile_context target` | print `usedk/windowk (percent%)`, or fail loudly |

Profiles may install native event hooks by composing them into their launch
command. They must not weaken sandboxing, approvals, or operator permissions.
