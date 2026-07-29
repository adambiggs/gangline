# Command and environment reference

This reference follows `bin/gang`'s command parser. Run `gang` with no arguments
for the built-in synopsis.

## Team lifecycle

### `gang up [name] [-p profile] [-r role] [-d dir] [-m model]`

The zero-setup entry point. The default name is `lead`, profile is
`GANG_PROFILE` or `claude-code`, directory is the caller's current directory,
and role is `GANG_ROLE` or `lead`.

`up` runs the same hitch path described below, then attaches to the team and
selects the new window. When called from inside tmux, it switches the current
client instead of nesting an attachment. An explicit `-r` overrides
`GANG_ROLE`, including when a different name is supplied.

### `gang hitch <name> [-p profile] [-r role] [-d dir] [-m model]`

Starts a harness in a new tmux window. `spawn` is an alias.

- `-p`, `--profile`: profile name; default `GANG_PROFILE` or `claude-code`.
- `-r`, `--role`: point the agent at this role brief after startup. Without a
  role, no role brief is sent.
- `-d`, `--dir`: working directory; default the caller's current directory.
- `-m`, `--model`: append the profile's declared model option and this model ID
  to its launch command. A profile without a model option refuses this flag.

Model spelling belongs to the harness: Claude Code accepts aliases or model IDs,
Codex accepts a model ID, and opencode and Pi use provider/model forms (Pi also
supports its harness-specific thinking suffix).

Names may contain `A-Z`, `a-z`, `0-9`, `.`, `_`, and `-`, and may not start with
`.` or `-`. A name is simultaneously a window name, identity, command handle,
and part of context-hook output.

Hitch waits up to `GANG_BOOT_TIMEOUT` for a stable input box. A role brief that
cannot be delivered makes hitch fail. With no brief, a running window may still
be reported as hitched with a warning when it is gated or never settles.
Profiles that require a session marker receive it even without a role.

### `gang adopt <name> -p <profile>`

Registers an existing window in the current team by setting its profile and
disabling automatic renaming. It does not launch, brief, inspect readiness, or
create a hitch-time session marker. Consequently, context reading for a Codex
window requires re-hitching rather than adopting it.

### `gang attach`

Executes `tmux attach` for `GANG_SESSION`.

### `gang drop <name>`

Kills the named window and prints `dropped <name>`. Window-owned state disappears
with it. This command resolves any named window in the team, including an
unadopted one.

### `gang down`

Lists the team's windows, then kills the entire `GANG_SESSION`. It fails when the
session is not running.

## Messaging and compaction

### `gang send <name> --from <sender> [--wait] [--timeout seconds] <message...>`

Sends a nonce-bearing attributed envelope through the verified delivery path.
`--from` may be replaced by `GANG_FROM`. There is no default identity.

Options must appear before the message body:

- `--from <sender>`: required sender identity.
- `-w`, `--wait`: if the target is busy, wait for idle instead of using or
  refusing mid-turn delivery.
- `--timeout <seconds>`: whole-number timeout for `--wait`; default 300.

Inside the team, the sender must equal the calling tmux window's name. Outside
the team, the trusted operator supplies it. Sender names follow the same grammar
as agent names.

A busy target is handled by profile declaration:

- a profile declaring mid-turn input receives the message immediately; success
  says `accepted mid-turn` because the harness decides when to consume it;
- a profile without that declaration is refused unless `--wait` was given.

Gated targets are always refused. Delivery also serialises Gangline writers for
the pane and refuses a composer that is changing while a person types.

### `gang compact <name> [--from sender] [--resume message]`

Pastes the profile's compaction command through the same verified injection path.
A profile with no compaction command is refused.

A busy peer cannot be compacted. An agent may compact its own busy window because
the slash command queues behind the turn that ends when its `gang compact` call
returns.

`--resume` requires `--from` or `GANG_FROM`, checked under the same identity rule
as `send`. Gangline starts a detached waiter that delivers the attributed resume
at a boundary where it cannot overtake its own compaction command. Failure is
stored on the window and printed by `status` and `patrol`. Without `--resume`, no
sender is required.

## Observation and waiting

### `gang status <name>`

Prints exactly one primary state line:

- `busy (tight tug)`
- `idle (slack tug)`
- `gated (hook set)`

It may then print diagnostic lines for a resume that failed after compaction or
an undelivered paste recorded in the input box. Scripts should match the first
line's state prefix rather than requiring single-line output.

Busy can be established by a declared busy marker, recent pty activity on a
profile known to be quiet at rest, or pane movement between captures. An agent
blocked inside Gangline's own wait is treated as available only when its profile
accepts mid-turn input. Gated takes precedence over busy.

### `gang context <name>`

Runs the profile's context reader. A successful shipped reader prints
`<used>k/<window>k (<percent>%)`. The command fails if the profile declares no
reader or the reader cannot establish a value.

### `gang wait <name> [timeout_seconds]`

Polls until idle holds twice consecutively, then prints `idle (slack tug)`.
Default timeout: 300 seconds. A gate fails immediately because only the operator
can clear it. The timeout must be whole seconds.

When an agent calls `wait` from its own pane, Gangline records that fact for the
call's lifetime. Profiles that accept mid-turn input can then report that waiting
agent as available to teammates.

### `gang capture <name> [lines]`

Prints the tail of the named window's pane after removing trailing blank rows.
Default: 40 lines. Unlike most per-agent commands, capture does not require the
window to have been adopted.

### `gang roster`

Prints every window in the team with name, profile, state, and context readout.
An unadopted window is shown rather than hidden. A missing custom profile is
reported as `profile not found (GANG_PROFILES?)`; an unreadable or undeclared
context value is `-`. A recorded stranded paste adds `undelivered paste`.

The state sampling wait is batched across the team, so roster does not add one
churn delay per window.

## Context warnings

### `gang patrol`

Performs one roster sweep. For each adopted agent it:

1. reports a failed resume or recorded undelivered paste;
2. reports and stops on a gate;
3. reads and parses context usage;
4. evaluates `GANG_CONTEXT_BANDS` against the window's last warned band;
5. re-arms warning state when usage falls;
6. injects one `[gang:patrol]` warning when a new band is crossed and the pane is
   safe.

A gang-issued compaction, painted busy marker, churning pane, or non-empty input
box holds the warning without advancing state, so a later patrol retries.
Unadopted windows and agents with missing profiles or readouts are reported as
not patrolled. With no team, patrol prints a no-team line and succeeds.

### `gang context-hook`

Harness hook entry point for in-turn warnings. It reads a JSON hook payload from
stdin, identifies the current agent through `TMUX_PANE`, applies the same band
ladder and window option as patrol, and emits a JSON `additionalContext` reply
only when a higher band is crossed.

Malformed JSON, a missing `TMUX_PANE` or profile binding, and an unavailable
context readout exit successfully without output. Once a profile is resolved,
profile-loading and band-configuration errors remain loud. The command is
intended for hook systems such as Claude Code's `UserPromptSubmit` and
`PostToolUse` events.

## Diagnostics and discovery

### `gang vet [--file-issue] [--probe [profile]]`

Plain vet visits every installed profile, including custom and test-only files.
It compares the harness's installed version with the profile's verified version
words, runs an optional profile-owned file-format gate, checks that a UTF-8
locale is available, and exits nonzero on drift. For an unpinned dotted-numeric
version it reports whether the installed build is newer than all pins, older than
all pins, or between pins when that ordering is unambiguous; otherwise the
`ROT RISK` remains deliberately unranked. Plain vet does **not** fire marker
regexes at a pane.

- `--file-issue`: for each version-pin `ROT RISK`, use `gh` to create a deduplicated
  GitHub issue in this repository. Failure to list open issues is fatal rather
  than risking a duplicate.
- `--probe`: after the version and format checks, launch each installed harness
  on a private `tmux -L` socket, give it a real turn, require its declared busy
  marker to be absent at rest, present while working, and absent after settling,
  then read its declared context value.
- `--probe <profile>`: narrow both the ordinary vet report and probe to one
  installed profile. An unknown name is an error.

A probe spends harness tokens. Not installed, no declared busy marker, first-run
dialog, launch failure, or a turn that never starts or settles is reported as
**not probed**, not as success. Zero means every marker that was actually fired
passed; it says nothing about skipped profiles. Gates, compacting markers, and
alternate busy-regex branches an ordinary turn did not paint remain outside the
probe's coverage.

`doctor` is an alias for `vet`.

### `gang profiles`

Lists offered profile names from `GANG_PROFILES` and the shipped directory,
deduplicated with custom files taking precedence. The shipped Bash test stand-in
is omitted unless Gangline's own test opt-in is set.

A profile is sourced shell, not a data-only record. The current contract includes
launch/model syntax; busy, gated, compacting, and quiet-at-rest observations;
compaction and mid-turn declarations; version pins; capture sizing; session-file
selection; and optional `profile_input`, `profile_context`, and `profile_vet`
functions. This is intentionally documented as current behavior rather than
rewriting ADR-0001's historical `~10 lines` premise; [issue
#13](https://github.com/adambiggs/gangline/issues/13) tracks the resulting
constitutional question.

### `gang roles`

Lists role names from `GANG_ROLES` and the shipped directory, deduplicated with
custom files taking precedence. Files whose names begin with `_` are shared
fragments and are not listed as roles.

## Environment

### User-facing settings

| Variable | Meaning | Default |
|---|---|---|
| `GANG_SESSION` | tmux session addressed by team commands | `gangline` |
| `GANG_PROFILE` | default profile for `up` and `hitch` | `claude-code` |
| `GANG_ROLE` | role used by `up` when `-r` is absent | `lead` |
| `GANG_FROM` | sender identity for `send` and compact resumes | none |
| `GANG_CONTEXT_BANDS` | comma-separated absolute token counts or percentages | `120000,180000,250000,350000` |
| `GANG_PROFILES` | one custom profile directory searched before shipped files | none |
| `GANG_ROLES` | one custom role directory searched before shipped files | none |
| `GANG_BOOT_TIMEOUT` | seconds hitch waits for a ready input box | `30` |
| `NO_COLOR` | any non-empty value disables colour | unset |

Every process that addresses one team must agree on `GANG_SESSION`. Cron must
also receive `GANG_PROFILES`, `GANG_CONTEXT_BANDS`, and `GANG_LOCK_DIR` when the
team overrides them.

### Operational settings

These are useful when the corresponding path is in use:

| Variable | Meaning | Default |
|---|---|---|
| `GANG_LOCK_DIR` | shared directory for per-pane delivery locks | `${XDG_RUNTIME_DIR:-/tmp}/gangline-<uid>` |
| `GANG_LOCK_WAIT` | lock acquisition polls at 0.2 seconds | `150` (30 seconds) |
| `GANG_CHURN_WAIT` | interval between pane-change samples | `0.5` seconds |
| `GANG_ACTIVITY_WINDOW` | how recently declared-quiet harness output counts busy | `5` seconds |
| `GANG_COMPACT_GRACE` | maximum wait for gang-issued compaction proof | `300` seconds |
| `GANG_RESUME_TIMEOUT` | detached resume wait ceiling | `900` seconds |
| `GANG_PROBE_DIR` | clean, already-trusted working directory for probes | private temporary directory |
| `GANG_PROBE_BOOT` | probe startup deadline | `45` seconds |
| `GANG_PROBE_TURN` | probe busy-marker observation window | `90` seconds |
| `GANG_PROBE_SETTLE` | probe settling deadline | `120` seconds |
| `GANG_PROBE_QUIET` | unchanged duration counted as settled | `5` seconds |

Changing compaction timing requires care: when a context baseline was captured,
a missing drop at `GANG_COMPACT_GRACE` sends the resume through the weaker
screen-settling fallback; after `GANG_RESUME_TIMEOUT`, delivery is attempted
regardless, still through the verified injection path.
