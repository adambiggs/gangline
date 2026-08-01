# Command and environment reference

This reference follows `bin/gang`'s command parser. Run `gang` with no arguments,
`gang help`, `gang -h`, or `gang --help` for the built-in synopsis.

## Team lifecycle

### `gang up [name] [-p profile] [-r role] [-d dir] [-m model]`

The zero-setup entry point. The default name is `lead`, profile is
`GANG_PROFILE` or `claude-code`, directory is the caller's current directory,
and role is `GANG_ROLE` or `lead`.

`up` runs the same hitch path described below, then attaches to the team and
selects the new window. When called from inside tmux, it switches the current
client instead of nesting an attachment. An explicit `-r` overrides
`GANG_ROLE`, including when a different name is supplied.

### `gang hitch <name> [-p profile] [-r role] [-d dir] [-m model] [--resume]`

Starts a harness in a new tmux window. `spawn` is an alias.

- `-p`, `--profile`: profile name; default `GANG_PROFILE` or `claude-code`.
- `-r`, `--role`: point the agent at this role brief after startup. Without a
  role, no role brief is sent.
- `-d`, `--dir`: working directory; default the caller's current directory.
- `-m`, `--model`: append the profile's declared model option and this model ID
  to its launch command. A profile without a model option refuses this flag.
- `--resume`: launch the harness so it picks up the last conversation in the
  working directory, instead of starting a fresh one. This is how a team is
  rebuilt after the tmux server dies and takes every window's state with it.
  A profile without a declared resume form refuses the flag rather than launching
  bare — see [ADR-0007](adr/0007-server-death-is-a-relaunch-not-a-restore.md),
  which also records why `opencode` and `pi` declare none.

Gangline does not remember the team. `--resume` recovers the harness
*conversation*, which nothing else can rebuild; names, roles, and context marks
are re-established by the hitch itself. The harness selects that conversation by
working directory and recency — no agent name or session id is involved, because
neither harness takes one — so `--resume` is only meaningful for one agent per
directory: two agents resuming in the same directory get the same conversation,
and Gangline cannot see that to refuse it. Where the directory holds no
conversation at all, the harness starts a fresh one and exits 0, and Gangline
reports a successful hitch rather than a blank agent (issue #52).

Model spelling belongs to the harness: Claude Code accepts aliases or model IDs,
Codex accepts a model ID, and opencode and Pi use provider/model forms (Pi also
supports its harness-specific thinking suffix).

Names may contain `A-Z`, `a-z`, `0-9`, `.`, `_`, and `-`, and may not start with
`.` or `-`. A name is simultaneously a window name, identity, command handle,
and part of context-hook output.

Hitch waits up to `GANG_BOOT_TIMEOUT` for a stable input box. A role brief that
cannot be delivered makes hitch fail. With no brief, a running window may still
be reported as hitched with a warning when it is occupied or never settles.
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
session is not running. Every piece of Gangline's state is a window option, so
tmux deletes all of it along with the windows.

## Messaging and compaction

### `gang send <name> --from <sender> [--wait] [--timeout seconds] --stdin`

Reads a message body from stdin and sends it in a nonce-bearing attributed
envelope through the verified delivery path. Positional message arguments are
refused because the sender's shell expands them before Gangline can inspect
them. `--from` may be replaced by `GANG_FROM`. There is no default identity.

Options:

- `--from <sender>`: required sender identity.
- `--stdin`: required; reads the complete body through EOF. An empty body is
  refused.
- `-w`, `--wait`: if the target is busy, wait for idle instead of using or
  refusing mid-turn delivery.
- `--timeout <seconds>`: whole-number timeout for `--wait`; default 300. When the
  timeout expires, the command fails before injection and delivers nothing.

Inside the team, the sender must equal the calling tmux window's name. Outside
the team, the trusted operator supplies it. Sender names follow the same grammar
as agent names.

A busy target is handled by profile declaration:

- a profile declaring mid-turn input receives the message immediately; success
  says `accepted mid-turn` because the harness decides when to consume it;
- a profile without that declaration is refused unless `--wait` was given.

Occupied targets are always refused. Delivery also serialises Gangline writers for
the pane and refuses a composer that is changing while a person types.

### `gang compact <name> [--from sender] [--resume-stdin]`

Submits the profile's compaction command through the same verified injection
path. A profile with no compaction command is refused. Verification proves the
command was delivered, not that the harness executed it successfully.

A busy peer cannot be compacted. Gangline permits an agent to compact its own
busy window because the slash command can queue behind the turn that ends when
its `gang compact` call returns. This is permission at the transport layer, not a
promise that every harness accepts its slash command there: Codex 0.145.0 rejects
`/compact` while a task is active, and a self-issued Gangline call keeps that task
active. Compact an idle Codex from another caller or let Codex auto-compact; do
not attach a self-issued Codex resume to a native command Codex will reject.

`--resume-stdin` reads the complete handoff through EOF and requires `--from` or
`GANG_FROM`, checked under the same identity rule as `send`. The old inline
`--resume <message>` form is refused because the sender's shell can alter its
prose before Gangline starts. Gangline starts a detached waiter that delivers
the attributed resume at a boundary where it cannot overtake its own compaction
command. Failure is stored on the window and printed by `status` and `patrol`.
Without `--resume-stdin`, no sender is required.

## Observation and waiting

### `gang status <name>`

Prints exactly one primary state line:

- `busy (tight tug)`
- `idle (slack tug)`
- `occupied (authority unknown)`
- `parked (waiting on <agent>)`, or `parked (gang wait in progress)` when the
  target of that wait cannot be read
- `expired (pty activity bound reached)`

It may then print diagnostic lines for a resume that failed after compaction or
an undelivered paste recorded in the input box. Scripts should match the first
line's state prefix rather than requiring single-line output.

Busy can be established by a declared busy marker, recent pty activity on a
profile known to be quiet at rest, or pane movement between captures. Occupancy
takes precedence over busy.

`occupied` says a harness-owned UI has the input box and nothing about who may
clear it. No shipped profile classifies the UI it found, so the qualifier is
always `(authority unknown)` today; a profile that earns a narrower one adds it
without changing the primary word
([ADR-0004](adr/0004-occupancy-is-not-authority.md)).

`parked` is an agent blocked inside Gangline's own `wait`. It is reported for every
waiter, because *available* and *idle* are different claims and only the first is
true here.

`expired` is neither busy nor idle: pty activity alone had been carrying the busy
verdict and has now spent `GANG_ACTIVITY_LIMIT` without any other signal agreeing.
Gangline reports that rather than resolving it, because an activity arm that has
run out is a different answer from an agent that was never busy — and resolving it
to idle is what would make a fabricated busy permanent.

### `gang context <name>`

Runs the profile's context reader. A successful shipped reader prints
`<used>k/<window>k (<percent>%)`. The command fails if the profile declares no
reader or the reader cannot establish a value.

### `gang wait <name> [timeout_seconds]`

Polls until idle holds twice consecutively, then prints `idle (slack tug)` and
exits 0. Default timeout: 300 seconds, which must be whole seconds. Two outcomes
end the call without idle ever holding:

| printed | exit | meaning |
| --- | --- | --- |
| `expired (pty activity bound reached)` | `2` | the activity-only arm spent `GANG_ACTIVITY_LIMIT`; waiting longer would turn a bounded policy into a permanent fabricated busy |
| `parked (…)` | `0` | the target is itself inside `gang wait`, on a profile that acts on mid-turn text |

An occupied target fails immediately, because nothing this command can wait for
will free an input box a UI is holding, and so does the timeout — neither is a
state this command reports.

When an agent calls `wait` from its own pane, Gangline records that fact for the
call's lifetime, so teammates reading its state see `parked` rather than `busy`.
Treat that as delivery availability, not response availability: a harness that
queues accepted text until the current turn ends holds the message until `gang
wait` itself returns. Only a profile witnessed acting on ordinary mid-turn text
(`GANG_MIDTURN_ACTS`) makes a parked agent reachable within the wait.

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
2. reports and stops on an occupied input box;
3. reads and parses context usage;
4. evaluates the band ladder against the window's last warned band;
5. re-arms warning state when usage falls;
6. injects one `[gang:patrol]` warning when a new band is crossed and the pane is
   safe;
7. while usage remains in the final band, repeats a distinct final-band reminder
   on every safe sweep. The repeat says it is not a fresh crossing and continues
   until usage drops out of that band.

A gang-issued compaction, an occupied UI, or a non-empty input box holds both a
crossing warning and a final-band repeat without advancing state, so a later
patrol retries. A busy agent is delivered to, not held: the warning is prose and
queues behind the turn it lands in. Lower steady bands remain quiet: only the
last rung is a permanently open question.
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
words, runs an optional profile-owned gate over the harness's files, checks that
a UTF-8 locale is available, and exits nonzero on drift. For an unpinned dotted-numeric
version it reports whether the installed build is newer than all pins, older than
all pins, or between pins when that ordering is unambiguous; otherwise the
`ROT RISK` remains deliberately unranked. Plain vet does **not** fire marker
regexes at a pane.

The profile-owned gate is also where harness-specific setup is checked, and a
finding names the edit that fixes it. `codex` and `opencode` gate the file
formats they parse. `claude-code` reports whether the context beacon
`profile_context` reads is wired as a `statusLine` command, distinguishing an
absent one, another statusline, and a beacon path that no longer exists; it reads
the user and managed settings scopes and names which it read. Project-scope
settings are not read — the directory an agent will be hitched in is not known at
vet time — and a settings file that exists but does not parse is reported as
undetermined, never as unconfigured. The gate is skipped where the harness is not
installed.

- `--file-issue`: for each version-pin `ROT RISK`, use `gh` to create a deduplicated
  GitHub issue in this repository. Failure to list open issues is fatal rather
  than risking a duplicate.
- `--probe`: after the version and format checks, launch each installed harness
  on a private `tmux -L` socket, give it a real turn, require its declared busy
  marker to be absent at rest, present while working, and absent after settling,
  then read its declared context value. Where a profile declares
  `GANG_MIDTURN_ACTS=1`, a second turn also attempts the asymmetric filesystem
  ordering probe described below.
- `--probe <profile>`: narrow both the ordinary vet report and probe to one
  installed profile. An unknown name is an error.

While probing, each phase it is about to wait in — input box, busy marker, pane
settling — is named on stderr with the bound it will give up at, so a long wait is
distinguishable from a hang. Those notes appear only when stderr is a terminal;
captured output and cron logs carry the rows alone. They are deliberately plain
sentences, because Gangline's output lands in the panes Gangline scrapes and a
spinner-shaped frame matches a shipped busy marker.

A probe spends harness tokens. Not installed, no declared busy marker, first-run
dialog, launch failure, or a turn that never starts or settles is reported as
**not probed**, not as success. Zero means every marker that was actually fired
passed; it says nothing about skipped profiles. Gates, compacting markers, and
alternate busy-regex branches an ordinary turn did not paint remain outside the
probe's coverage.

The mid-turn probe does not derive a verdict from pane text. Turn 1 creates a
start file, performs a slow action, and creates boundary file A as its last
action; while the start file exists and A does not, ordinary text asks the
running agent to create B immediately. B observed alone before A, followed by A,
prints `CONFIRMED`. Every other outcome prints `NOT PROBED` with a
could-not-determine reason. In particular, A-before-B and A plus B first seen in
the same filesystem poll cannot refute the declaration: a next turn may have
begun between observations. Profiles without the declaration never receive this
second turn.

`doctor` is an alias for `vet`.

### `gang profiles`

Lists offered profile names from `GANG_PROFILES` and the shipped directory,
deduplicated with custom files taking precedence. The shipped Bash test stand-in
is omitted unless Gangline's own test opt-in is set.

A profile is sourced shell, not a data-only record. Its declaration surface is:

| Declaration | Meaning |
|---|---|
| `GANG_LAUNCH` | required harness launch command |
| `GANG_RESUME_LAUNCH` | full launch command that resumes the most recent harness conversation in the working directory, used by hitch's `--resume`. A complete command rather than a flag, because a harness may spell resume as a subcommand. Declared only where the harness scopes sessions to a directory; a profile that declares none refuses `--resume` rather than launching bare (ADR-0007) |
| `GANG_MODEL_OPT` | model option used by hitch's `-m` |
| `GANG_BUSY_REGEX` | painted working marker |
| `GANG_OCCUPIED_REGEX` | modal marker, confirmed against composer absence |
| `GANG_COMPACT_CMD` | native compaction command |
| `GANG_COMPACTING_REGEX` | live compaction marker |
| `GANG_MIDTURN_INPUT=1` | composer safely takes input during a turn |
| `GANG_MIDTURN_ACTS=1` | harness *acts* on ordinary text inside the running turn, rather than queueing it until the turn ends. Strictly stronger than `GANG_MIDTURN_INPUT` and the only thing that makes a parked agent reachable mid-wait. Declared by `claude-code` alone; `vet --probe` can confirm it from B-before-A filesystem ordering but cannot refute it |
| `GANG_QUIET_AT_REST=1` | recent pty writes may be used as a busy signal |
| `GANG_SESSION_KEY=1` | hitch must mint and deliver a transcript marker |
| `GANG_VERSION_CMD` | installed-version command |
| `GANG_VERIFIED_VERSIONS` | space-separated verified prefixes, or `any` only when there are no scraped markers |
| `profile_input` | print the live composer, or fail when none exists |
| `profile_context` | print one parseable context-usage line |
| `profile_vet` | optional gate on harness files — formats the profile parses, and configuration its readers require |

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
| `GANG_CONTEXT_FLOOR` | first rung, in absolute tokens — the same number on every harness | `120000` |
| `GANG_CONTEXT_CAP` | ceiling for the last rung, in absolute tokens. Five rungs are derived across `[floor, min(90% of window, cap)]`, so both ends are token counts and only the spacing fits the window ([ADR-0006](adr/0006-the-band-ladder-spans-absolute-bounds.md)) | `350000` |
| `GANG_CONTEXT_BANDS` | explicit comma-separated ladder, bypassing that derivation; a `%` of the agent's window is an escape hatch, never a default ([ADR-0005](adr/0005-context-bands-are-absolute.md)) | `auto` |
| `GANG_PROFILES` | one custom profile directory searched before shipped files | none |
| `GANG_ROLES` | one custom role directory searched before shipped files | none |
| `GANG_BOOT_TIMEOUT` | seconds hitch waits for a ready input box | `30` |
| `NO_COLOR` | any non-empty value disables colour | unset |

Every process that addresses one team must agree on `GANG_SESSION`. Cron must
also receive `GANG_PROFILES`, any of the three ladder settings and `GANG_LOCK_DIR`
when the team overrides them — a patrol that disagrees about the lock directory
stops serialising with the other writers.

### Operational settings

These are useful when the corresponding path is in use:

| Variable | Meaning | Default |
|---|---|---|
| `GANG_LOCK_DIR` | shared directory for per-pane delivery locks; created `0700`, and refused if it is a symlink, not a directory, or not owned by you | `/tmp/gangline-<uid>` |
| `GANG_LOCK_WAIT` | lock acquisition polls at 0.2 seconds | `150` (30 seconds) |
| `GANG_CHURN_WAIT` | interval between pane-change samples | `0.5` seconds |
| `GANG_ACTIVITY_LIMIT` | how long pty activity **alone** may hold the busy verdict before the state becomes `expired`; whole seconds, and a non-numeric value is refused | `300` |
| `GANG_ACTIVITY_WINDOW` | how recently declared-quiet harness output counts busy | `5` seconds |
| `GANG_COMPACT_GRACE` | maximum wait for gang-issued compaction proof | `300` seconds |
| `GANG_RESUME_TIMEOUT` | detached resume wait ceiling | `900` seconds |
| `GANG_PROBE_DIR` | clean, already-trusted working directory for probes | private temporary directory |
| `GANG_PROBE_BOOT` | probe startup deadline | `45` seconds |
| `GANG_PROBE_TURN` | probe busy-marker observation window | `90` seconds |
| `GANG_PROBE_SETTLE` | probe settling deadline | `120` seconds |
| `GANG_PROBE_QUIET` | unchanged duration counted as settled | `5` seconds |
| `GANG_PROBE_RATE` | interval between probe captures | `0.25` seconds |
| `GANG_PROBE_PROMPT` | prompt used to drive the probe's real turn | built-in comparison prompt |
| `GANG_PROBE_ACTS_WAIT` | filesystem-ordering window for a declared mid-turn-acts probe | `45` seconds |
| `GANG_PROBE_ACTS_DELAY` | duration of the slow middle action in that fixture | `15` seconds |
| `GANG_PROBE_ACTS_PROMPT` | turn-1 fixture template; expands `@start@`, `@boundary@`, `@message@`, and `@delay@` | built-in three-action prompt |
| `GANG_PROBE_ACTS_MESSAGE` | injected ordinary-text template using the same placeholders | built-in create-B prompt |
| `GANG_STATUS_ROWS` | bottom pane rows scanned by fast painted-state checks | `20` |
| `GANG_BRIEF_GATE_WAIT` | post-brief delay before hitch's early gate check | `3` seconds |
| `GANG_CLEAR_PRESSES` | maximum verified `Ctrl-u` attempts when reclaiming a staged paste | `40` |
| `GANG_TEST_PROFILES=1` | expose the Bash stand-in used by Gangline's own suite | unset |

`GANG_ACTIVITY_LIMIT`, `GANG_COMPACT_GRACE` and `gang wait`'s timeout all default to
300 seconds, and that is a coincidence rather than a shared constant. They bound three
unrelated things — how long pty activity alone may support a busy verdict, how long
Gangline waits for proof that a compaction it issued ran, and how long one caller is
willing to poll all busy evidence. Changing any one of them must not move the others,
so do not fold them into a single value.

Changing compaction timing requires care, and the two readers part company at
expiry. On the resume leg, when a context baseline was captured, a missing drop
at `GANG_COMPACT_GRACE` sends the resume through the weaker screen-settling
fallback; after `GANG_RESUME_TIMEOUT`, delivery is attempted regardless, still
through the verified injection path. On the patrol leg, expiry is not clearance
either, but it is not evidence of a live compaction — so a gang-issued compaction
with no drop to show for it is reported once as never proved, its mark is cleared,
and the next sweep patrols that agent normally. Holding on an expired mark
suppressed the warning for the life of the window, since nothing rewrites one; and
a mark recorded while the context readout was unavailable carries a zero, which
the drop check skips, so expiry was the only end it could ever reach.
