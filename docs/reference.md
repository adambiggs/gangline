# Command and environment reference

This reference follows `bin/gang`'s command parser. Run `gang` with no arguments,
`gang help`, `gang -h`, or `gang --help` for the built-in synopsis.

## Team lifecycle

### `gang up [name] [hitch's flags]`

The zero-setup entry point. The default name is `lead`, profile is
`GANG_PROFILE` or `claude-code`, directory is the caller's current directory,
and role is `GANG_ROLE` or `lead`.

`up` runs the same hitch path, then attaches to the team and selects the new
window. Every flag it is given is handed to `gang hitch` untouched, so whatever
`hitch` takes, `up` takes — see `gang hitch` for the list, which is not repeated
here because a second copy of it is one release away from disagreeing with the
first. When called from inside tmux, it switches the current client instead of
nesting an attachment. An explicit `-r` overrides `GANG_ROLE`, including when a
different name is supplied.

### `gang hitch <name> [-p profile] [-r role] [-d dir] [-m model] [--resume] [--cutoff duration|clock]`

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
- `--cutoff`: declare the team's cutoff while hitching, in the forms `gang cutoff`
  itself takes — a duration or an `HH:MM` clock time, described under The cutoff
  below. Settled before the window is opened, so a word Gangline will not accept
  leaves no agent behind and no declaration disturbed.

Gangline does not remember the team. `--resume` recovers the harness
*conversation*, which nothing else can rebuild; names, roles, and context marks
are re-established by the hitch itself. The harness selects that conversation by
working directory and recency — no agent name or session id is involved, because
neither harness takes one — so `--resume` is only meaningful for one agent per
directory: two agents resuming in the same directory get the same conversation,
and Gangline cannot see that to refuse it. Where the directory holds no
conversation at all, the harness starts a fresh one and exits 0, and Gangline
reports a successful hitch rather than a blank agent (issue #52).

There is no per-agent cutoff, so hitching one more dog re-declares the whole
team's budget. Last wins, and it re-spans from the moment of the hitch rather
than inheriting what was left of the declaration it replaces. The hitch prints
the cutoff it declared and says whose it is: a team-wide budget that moved is not
something to discover later. This is a second way in, not a second cutoff — the
same storage, forms and rules as `gang cutoff`.

Model spelling belongs to the harness: Claude Code accepts aliases or model IDs,
Codex accepts a model ID, and opencode and Pi use provider/model forms (Pi also
supports its harness-specific thinking suffix).

Names may contain `A-Z`, `a-z`, `0-9`, `.`, `_`, and `-`, and may not start with
`.` or `-`. A name is simultaneously a window name, identity, command handle,
and part of `gang hook` output.

Hitch waits up to `GANG_BOOT_TIMEOUT` for a stable input box. A role brief that
cannot be delivered makes hitch fail. With no brief, a running window may still
be reported as hitched with a warning when it is occupied or never settles.
Profiles that require a session marker receive it even without a role.

What hitch was told is kept on the window it describes: the profile, role,
directory and model, one window option per fact, beside a record naming the
format they are written in. `gang cycle` is what reads them. `--resume` and
`--cutoff` are deliberately not among them — replaying a resume form would
restore the conversation a cycle exists to discard, and the cutoff is the whole
team's, held on the session rather than on any window. The format record is
written last, after every fact it vouches for, so a hitch that died partway
cannot read as a complete one. All of it is window options, so tmux deletes it
along with the window: the record dies with the agent it describes, and nothing
sweeps it (law 6).

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

### `gang cycle <name> [-m|--model <model>] [--from <sender>] [--resume-stdin]`

Replaces the named agent with a plain launch on its recorded profile, role,
directory, effort and model; `-m` overrides the recorded model. The harness
conversation does not return, and the session-scoped cutoff is unchanged. See
[ADR-0015](adr/0015-a-context-is-renewed-by-cycling-not-by-summary.md) for the
renewal decision.

#### Structured continuation input

`--resume-stdin` consumes one complete `GANGLINE-CONTINUATION 1` package through
EOF. It requires `--from` or `GANG_FROM`; the target name, attributed sender and
package `Writer` must be identical. Arbitrary prose is not continuation input.

The package must satisfy all of these boundaries:

| Boundary | Requirement |
| --- | --- |
| Raw file | Non-empty UTF-8 with the exact LF-only closed grammar; BOM, NUL, CR, unknown fields, reordered fields and trailing content are refused. |
| Headers | Exact `Writer`, `Reviewed-At` and absolute `Ledger` fields. The review epoch is later than the target's floor and prior accepted review, and is not in the future. |
| Task references | At least one heading-only task reference. Every referenced task exists live in the captured ledger and names the package writer as owner. Each reference has a Next Actions or Local Blockers note. |
| Sections and notes | After `Task References`, the exact section order is `Active Work`, `Next Actions`, `Local Blockers`, `Binding Bounds`, `Dangerous Refutations`. A note's ordered fields are `Task`, `Supersedes`, `Remove`, `Evidence`, `Provenance`, `Text`; evidence is `verified`, `claimed`, `inferred`, `unverified` or `refuted`. Identity, immutable retained bodies, supersession, provenance, expiry and pressure transitions are checked against prior accepted state. |
| Ledger | Exact `GANGLINE-TASK-LEDGER 1` grammar. Its absolute path and writer become session-pinned; later observations are byte-identical at the same revision or a valid monotonic transition. |
| Outer guard | `GANG_RESUME_MAX` is checked before structure. Passing the byte guard does not bypass any structural, ledger, review or transition check. |

[ADR-0018](adr/0018-continuation-state-is-a-closed-reviewed-set.md) is the
normative grammar. The authoring workflow is in
[Operating a team](operations.md#renewing-context).

Without `--resume-stdin`, no package is delivered. The replacement still carries
the lineage's existing `empty` or `accepted` continuation state; a missing,
malformed or `pending` state is refused before retirement.

#### Preflight and first refusal

Before retiring the target, `cycle` checks the launch record, recorded directory,
profile and role, the optional model spelling, and every applicable continuation
boundary above. A window created before continuation state existed deliberately
refuses its first structured attempt: Gangline records a review floor and a
known-empty transition, leaves the context intact, and requires a newly reviewed
package with a later `Reviewed-At`.

For an admitted package, `cycle` rechecks the candidate under the target and
ledger critical sections and records `pending` before retirement. It consumes the
package from stdin but never edits or deletes the authored file.

#### Success effects

- The predecessor leaves the agent namespace and becomes `<name>~spent`, with its
  pane and scrollback readable. `send` and `status` refuse it; `capture` can still
  read the named window, roster shows it unadopted, and `gang drop <name>~spent`
  closes it. The next cycle closes the previous spent window.
- The replacement is hitched and briefed on the recorded launch facts. It takes
  the predecessor's position when tmux permits, while the spent window moves to
  the end.
- With a package, the ledger and transition are revalidated at the delivery
  event, the attributed envelope is pane-verified, and only then does `pending`
  become `accepted`. The final `resumed <name>` line is the delivery receipt and
  reports the package size and any observed growth.
- Without a package, the carried continuation state is installed and nothing is
  delivered into the new context.

Self-cycle is supported because the predecessor remains alive as the spent
window while its replacement takes the agent name.

#### Failure effects

- Any preflight refusal leaves the target running with its original context;
  nothing is typed or retired and the authored files are untouched.
- After retirement, a harness launch, briefing, positioning or delivery can still
  fail loudly. A positioning failure does not undo a working replacement. A
  launch or briefing failure leaves durable work untouched and the predecessor
  readable if it was already retired.
- A delivery failure after `pending` leaves the replacement lineage pending. A
  direct `gang send`, another cycle, or another compact cannot accept or clear it.
  Run `gang drop <name>`, establish a genuinely new lineage with ordinary `hitch`
  or `adopt`, then review a new package after that window's floor.

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
the pane, and a composer that is changing while a person types is waited out
rather than refused: what was in the box first leaves first, so the paste never
jumps a half-written line. The sender is told it is queued behind operator typing
while it waits, and the delivery line says how long the send was held. The wait is
bounded by `GANG_SEND_HOLD`; when the bound runs out, nothing was delivered and
the body is still the sender's to send again. Ordering is promised between the
operator's text and the held send, and nowhere else — concurrent senders are not a
queue. A target found occupied *during* a hold is refused there and then, in the
same words and with the same status as one found occupied before it: waiting ends
no occupancy, and the record it leaves on that agent is what tells an operator
their dialog is stalling traffic.

### `gang compact <name> [--from <sender>] [--resume-stdin]`

Submits the profile's native compaction command through verified pane injection.
A profile without a compact command is refused. A successful return proves the
command was submitted, not that the harness executed it. Without
`--resume-stdin`, no sender is required and continuation transition state is not
initialized, reset or advanced.

#### Required structured input

With `--resume-stdin`, the input is the same complete structured package required
by `cycle`; the [closed boundary](#structured-continuation-input) applies in full.
`--from` or `GANG_FROM` is required, and the target, sender and package `Writer`
must be the same identity. The obsolete inline `--resume <message>` form is
refused.

#### Preflight

Gangline stages the package, applies `GANG_RESUME_MAX`, validates the package and
captured ledger, checks review and transition state, and rejects reserved
tag-shaped content before submitting the native command. Occupancy is then
refused. A busy peer is refused because compaction could cut its live turn; a
self-issued command may queue behind the caller's turn. That transport permission
does not make native execution self-safe: Codex rejects `/compact` while a task is
active, so compact an idle Codex from another caller or allow Codex to compact
automatically.

#### Success effects

- The native command is pane-verified and marked as issued. The command's success
  line still labels execution unconfirmed.
- With a package, a detached worker waits until a declared live marker, context
  below half the issue-time baseline, bounded non-busy stable-screen evidence, or
  the overall timeout makes a delivery attempt due. The immediate command output
  says the structured resume is queued; it is not a delivery receipt.
- At the delivery event the worker confirms the same active target, recaptures and
  revalidates the ledger, repeats review and transition checks, records `pending`,
  injects through the attributed verified path, and then records `accepted`.

#### Failure effects

- A package, ledger, review, transition, occupancy or peer-busy refusal before
  submission issues no native command and leaves the authored files untouched.
- Native-command delivery can succeed while native execution later fails; the
  immediate caller cannot distinguish those outcomes.
- A detached validation or delivery failure is stored on the window and reported
  by `gang status` and patrol. If it occurs after `pending`, only explicit
  `gang drop`, ordinary `hitch` or `adopt`, and a newly reviewed package establish
  a usable new lineage; direct send and another renewal do not repair it.

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

Busy takes the highest evidence tier still fresh (ADR-0008): a live turn
bracket fed by the harness's own hook events where the profile wires them, then
a declared busy marker, recent pty activity on a profile known to be quiet at
rest, or pane movement between captures. Occupancy takes precedence over busy,
and is itself witnessed by a permission-request raise where one is wired —
retired only when the pane proves the dialog gone — with the modal scrape as
the tier below.

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

Prints the best fresh context witness (ADR-0008): a figure the owned tier
recorded — on claude-code, the shipped statusline script writes it as it paints
the beacon — and the profile's own reader when no fresh fact exists. A
successful readout prints `<used>k/<window>k (<percent>%)`. The command fails
if the profile declares no reader or no tier can establish a value.

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

## The cutoff

### `gang cutoff [<duration|clock>|clear]`

Declares the team's wall-clock budget, shows it, or ends it. A duration is one or
more whole `<n>h`, `<n>m`, `<n>s` groups — `90m`, `2h`, `1h30m`. A clock time is
`HH:MM` on a 24-hour clock, resolved to its next occurrence in the host's own
timezone. Anything else is refused, a bare number included: gang does not pick a
unit on the operator's behalf.

The cutoff is session-scoped — one team, one cutoff
([ADR-0009](adr/0009-time-bands-are-relative.md)). Declaring again replaces the
previous declaration and re-spans the budget from now, so what is stored is
always the cutoff together with the moment it was declared. With no argument the
command prints the declaration, or `no cutoff declared`; an explicit query is
answered rather than met with silence. `clear` removes it, and apart from the
session ending it is the only thing that does.

Once declared, every `gang patrol` row carries the team's position in that budget;
with nothing declared a sweep says nothing about time at all.

Nothing is enforced. Declaring a cutoff records operator intent and changes no
behaviour on its own.

## The warning legs

### `gang patrol`

Performs one sweep in this order for each adopted agent:

| Stage | Observable result and control flow |
| --- | --- |
| Stored delivery diagnostics | Reports a failed structured resume, a detached resume whose outcome is still unknown, and inbound sends refused while occupancy blocked the agent. Then continues. |
| Cutoff position | Reports the current time rung and remaining budget when a cutoff exists. This needs no profile or context readout, so later context failure does not erase the row. No cutoff produces no budget row or log line. |
| Undelivered paste | Reports the record and clears it only when the composer can be proved empty. Then continues. |
| Occupancy | Reports the occupied UI and stops processing that agent; Gangline does not answer or clear it. |
| Budget injection | Sends one `[gang:patrol]` note on a new time rung, repeats it in the banking reserve, and restates an overrun. Occupancy or a non-empty composer holds it without advancing state. Gang-issued compaction does not hold a budget note. |
| Profile and context | Resolves the profile and reads context. Unadopted windows and missing profiles or readouts are reported as not patrolled. A missing readout stops only the context leg because budget work already ran; an unresolved or unloadable profile prevents safe injection. |
| Context ladder | Re-arms when usage falls, sends one warning on a new rung, and repeats the final-rung instruction on every safe sweep until usage falls. Gang-issued compaction, occupancy or a non-empty composer holds injection without advancing the rung. A busy pane may receive the prose mid-turn according to its profile. |

Time and context keep separate last-warned state. Crossings below their repeating
terminal conditions remain quiet after the first delivery. With no team, patrol
prints the no-team result and succeeds. Ladder policy belongs to
[ADR-0006](adr/0006-the-band-ladder-spans-absolute-bounds.md) and
[ADR-0009](adr/0009-time-bands-are-relative.md).

### `gang cron [--install|--refresh]`

Derives the crontab entry that runs `gang patrol` every two minutes for this
install, and with no argument prints it. The command it names is the `gang` on
PATH when that resolves to this install tree, and this tree's own `bin/gang`
otherwise. It carries every `GANG_*` variable exported in the calling
environment, minus `GANG_FROM`, `GANG_PROFILE_FILE`, `GANG_SESSION_KEY` and
`GANG_TEST_PROFILES`, plus `TMUX_TMPDIR` and `XDG_STATE_HOME` when set. Values
are shell-quoted only where they need it, and `%` is escaped for cron, which
reads a bare one as a newline. Defaults are never carried.

`--install` writes the entry, replacing an existing one for this session where it
sits and printing the line it displaced; with none present it appends. An entry
sweeping another session, and one commented out, are both left as they are. It
reports `already current` and writes nothing when the entry in force is byte-identical.

`--refresh` is the same replacement with the append removed: present entries are
updated, an absent one is reported and nothing is written. It is what `install.sh`
runs, so updating Gangline refreshes an entry the operator chose without ever
adding one they did not. It is a silent success on a host with no `crontab`
command, where `--install` fails loudly instead.

### `gang hook`

Reads one native JSON hook event from stdin, identifies the window through
`TMUX_PANE`, and applies this mapping:

| `hook_event_name` | Window fact | Output |
| --- | --- | --- |
| `UserPromptSubmit` | Open the turn bracket. | Emit JSON `additionalContext` only when a context or time rung is newly crossed. |
| `PostToolUse` | Refresh the open turn bracket. | Same crossing-only `additionalContext` behavior. |
| `Stop` | Close the turn bracket. | Silent. |
| `PreCompact` | Open the compaction bracket and record its trigger. | Silent. |
| `PostCompact` | Close the compaction bracket and record its trigger. | Silent. |
| `PermissionRequest` | Raise occupancy until the pane proves the dialog gone. | Always silent; stdout must never answer the permission decision. |

Interface rules:

- When both ladders cross on one eligible event, their notes share the single
  `additionalContext` field. With no crossing there is no reply. Patrol's repeats
  do not run on the hook leg.
- Malformed JSON, an unknown event, missing `TMUX_PANE`, missing profile binding,
  or unavailable context is a silent success so telemetry cannot block the work
  it observes. After a profile resolves, profile-loading and band-configuration
  errors remain loud.
- Open brackets expire at their configured bounds. A closed turn bracket that is
  not followed by a later event stops outranking continued pane output after
  `GANG_TURN_LIMIT` on profiles declared quiet at rest; the reader falls through
  to its scrape tiers instead of inventing idle.
- The event payload carries no context count. Profiles that own a context fact
  write it through their own declared surface.

The evidence-tier rationale is in
[ADR-0008](adr/0008-evidence-is-tiered-per-predicate.md) and the rule for a
declared fact that never arrives is in
[ADR-0017](adr/0017-a-declared-fact-that-never-arrives-is-not-a-missing-tier.md).

## Diagnostics and discovery

### `gang vet [--file-issue] [--probe [profile]]`

Options:

| Form | Behavior |
| --- | --- |
| `gang vet` | Visits installed shipped, custom and test profiles; compares installed version words with profile pins; runs profile-owned file/configuration gates; checks for a UTF-8 locale; reports the patrol cron entry; and compares fresh live evidence tiers where a team exists. It does not drive a marker. |
| `gang vet --file-issue` | Performs plain vet, then uses authenticated `gh` to file deduplicated issues for version-pin and live tier-conflict `ROT RISK` findings. It lists open issues before filing. Filing failure never retracts the printed finding; a version-pin filing failure is fatal rather than risking a duplicate. |
| `gang vet --probe` | Performs plain vet, then launches each installed harness on a private `tmux -L` socket, drives a real turn, checks the declared busy marker and context reader, reads back declared owned facts, and attempts the mid-turn ordering probe where declared. |
| `gang vet --probe <profile>` | Restricts both ordinary checks and the live probe to one installed profile. An unknown profile is an error. |

Verdicts:

| Output | Meaning |
| --- | --- |
| `ROT RISK` | Version pins, a profile-owned gate, or two fresh witnesses disagree. The run exits nonzero. An unpinned dotted version is ranked as newer, older or between pins only when that ordering is unambiguous. |
| `fact '<name>' CONFIRMED` | The harness's declared wiring wrote that fact during the driven turn and the live reader found it. |
| `fact '<name>' DECLARED AND MISSING` | A leg that drove the declared fact completed but its owned record was absent. This is drift and exits nonzero. |
| `not probed` / `NOT PROBED` | The probe could not honestly exercise or determine the predicate. This is not a pass and cannot refute the declaration. |
| `mid-turn acts CONFIRMED` | File B was observed while boundary file A was absent, followed by A. Only that B-before-A ordering confirms action inside the running turn. |
| patrol cron `absent` | No entry for this session; a permitted operator choice, not drift. |
| patrol cron `current` | The installed entry matches what `gang cron` derives now. |
| patrol cron `stale` | The current and derived entries differ; both are printed and vet exits nonzero. |

Coverage limits:

- Plain vet checks version strings, file/configuration gates and live witness
  agreement. It never fires a marker, so a clean result does not prove current
  TUI chrome.
- A live probe spends harness tokens. A missing install or marker declaration,
  first-run dialog, launch failure, missing input box, turn that never starts, or
  pane that never settles is `not probed`. Interactive stderr names the phase and
  its bound; captured output contains the verdict rows.
- A zero probe exit covers only markers and facts actually exercised. Occupancy,
  compaction that needs a naturally full context, alternate regex branches and
  every `not probed` row remain outside that claim.
- Mid-turn A-before-B or both files first observed together is
  could-not-determine, not a refutation: another turn may have begun between
  observations.
- Profile gates name the configuration source they could read. Missing,
  malformed and out-of-scope settings remain distinct; an unreadable predicate
  is never reported as clean absence.

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
| `GANG_PROBE_FACTS` | space-separated facts whose owned records `vet --probe` must find the harness's own wiring writing across a driven turn; a declared fact missing after that turn is drift |
| `profile_input` | print the live composer, or fail when none exists |
| `profile_context` | print one parseable context-usage line |
| `profile_vet` | optional gate on harness files — formats the profile parses, and configuration its readers require |

The scrape declarations above are every predicate's bottom tier. Per predicate,
evidence is tiered — owned event > owned file > pane scrape — and every reader
takes the highest tier still fresh, refusing a fact past its bound rather than
trusting it as last-known-good (ADR-0008). A profile adds a tier above the
scrape by wiring the harness's own surfaces into the launch line at hitch — an
inline settings string naming `gang hook`, a notify program — never by a
generated file on disk. Degradation is per predicate, not per profile: a
harness with event-witnessed turns and scrape-witnessed dialogs is a portfolio,
not a partial migration.

Gang's own issued-compaction mark is substrate and exists on every profile; the
table names what each harness's side witnesses:

| Predicate | claude-code | codex | opencode | pi |
|---|---|---|---|---|
| busy | turn bracket from the wired hook events, then the scrape arms | scrape | scrape | scrape |
| compacting | event bracket, the only witness of the harness's own auto-compaction; the scrape deliberately undeclared — a manual compaction paints the busy regex's own bar, and one glyph must not carry two predicates | no scrapable marker — the mark alone | no scrapable marker — the mark alone | painted marker |
| occupied | permission-request raise, retired by the pane, above the modal scrape | modal scrape | modal scrape | modal scrape |
| context | written by the shipped statusline script beside the beacon it paints, above the beacon scrape | owned file — the rollout is read, never the pane | pane hint row, with the window size joined from the catalog file | native readout scrape |

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
| `GANG_FROM` | sender identity for `send` and structured `cycle` or `compact` resumes | none |
| `GANG_CONTEXT_FLOOR` | first rung, in absolute tokens — the same number on every harness | `120000` |
| `GANG_CONTEXT_CAP` | ceiling for the last rung, in absolute tokens. Five rungs are derived across `[floor, min(90% of window, cap)]`, so both ends are token counts and only the spacing fits the window ([ADR-0006](adr/0006-the-band-ladder-spans-absolute-bounds.md)) | `350000` |
| `GANG_CONTEXT_BANDS` | explicit comma-separated ladder, bypassing that derivation; a `%` of the agent's window is an escape hatch, never a default ([ADR-0005](adr/0005-context-bands-are-absolute.md)) | `auto` |
| `GANG_TIME_RESERVE` | banking room held back from the end of a declared cutoff, so the last time rung fires where banking has to start rather than where the budget stops. A whole percentage below 100, or a duration (`15m`). The absolute form is an operator's to pick rather than a convenience: writing results out costs roughly the same however much budget is left, so a proportional reserve shrinks below the cost of banking exactly when the budget is short ([ADR-0009](adr/0009-time-bands-are-relative.md)) | `10%` |
| `GANG_TIME_BANDS` | explicit comma-separated ladder for the budget, bypassing that derivation; every rung is a `%` of what the reserve leaves — the same span the derivation divides — and an absolute duration is refused, naming the rung it should have been. The escape hatch inverts here: on the context axis absolute is the design and `%` is the hatch ([ADR-0009](adr/0009-time-bands-are-relative.md)) | `auto` |
| `GANG_PROFILES` | one custom profile directory searched before shipped files | none |
| `GANG_ROLES` | one custom role directory searched before shipped files | none |
| `GANG_BOOT_TIMEOUT` | seconds hitch waits for a ready input box | `30` |
| `NO_COLOR` | any non-empty value disables colour | unset |

Every process that addresses one team must agree on `GANG_SESSION`. Cron must
also receive `GANG_PROFILES`, any of the ladder settings and `GANG_LOCK_DIR`
when the team overrides them — a patrol that disagrees about the lock directory
stops serialising with the other writers.

### Operational settings

These are useful when the corresponding path is in use:

| Variable | Meaning | Default |
|---|---|---|
| `GANG_LOCK_DIR` | shared directory for per-pane delivery locks; created `0700`, and refused if it is a symlink, not a directory, or not owned by you | `/tmp/gangline-<uid>` |
| `GANG_LOCK_WAIT` | lock acquisition polls at 0.2 seconds | `150` (30 seconds) |
| `GANG_SEND_HOLD` | how long a send waits out a composer somebody is typing into before handing the body back to its sender; whole seconds, and a non-numeric value is refused | `120` seconds |
| `GANG_CHURN_WAIT` | interval between pane-change samples | `0.5` seconds |
| `GANG_ACTIVITY_LIMIT` | how long pty activity **alone** may hold the busy verdict before the state becomes `expired`; whole seconds, and a non-numeric value is refused | `300` |
| `GANG_ACTIVITY_WINDOW` | how recently declared-quiet harness output counts busy | `5` seconds |
| `GANG_COMPACT_GRACE` | maximum wait for gang-issued compaction proof | `300` seconds |
| `GANG_TURN_LIMIT` | how long a turn bracket's claim holds without renewal — an **open** one holds busy without a heartbeat, and past the same age a **closed** one stops outranking a pane that is still being written to; expiry is could-not-determine, and the verdict falls to the scrape tiers | `300` seconds |
| `GANG_COMPACTION_LIMIT` | how long an open compaction bracket holds the compacting verdict; undershooting drops a live compaction to a tier that reads it idle, so the error direction is chosen long | `300` seconds |
| `GANG_OCCUPIED_LIMIT` | how long a raised occupancy stays credible; the pane retires the raise by proving the dialog gone, and expiry stops believing it without clearing it | `900` seconds |
| `GANG_CONTEXT_FACT_LIMIT` | how long a written context figure is preferred over the beacon scrape; a stale figure reads low across a band edge, so the bound is short and the scrape does the work in any doubt | `60` seconds |
| `GANG_RESUME_TIMEOUT` | detached resume wait ceiling | `900` seconds |
| `GANG_RESUME_MAX` | outer byte guard for a structured package passed to `cycle --resume-stdin` or `compact --resume-stdin`; whole bytes, and a non-numeric value is refused. Over it, cycle refuses before retirement and compact refuses before issuing the native command. Under it, the full structural, ledger, review and transition checks still apply. Gangline never trims a body; verified delivery reports its size and observed growth | `65536` |
| `GANG_PATROL_LOG` | where a non-interactive sweep records itself; empty writes no file | `$XDG_STATE_HOME/gangline/patrol.log` |
| `GANG_PATROL_LOG_MAX` | size at which that log rolls to a single `.1`; whole bytes, and a non-numeric value is refused | `1048576` |
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
| `GANG_NOW` | test-only clock seam: the epoch cutoff arithmetic is measured against, so a case about a time boundary reaches it without waiting for it. Whole seconds, and a non-numeric value is refused | unset (the system clock) |

`GANG_ACTIVITY_LIMIT`, `GANG_COMPACT_GRACE`, `GANG_TURN_LIMIT`,
`GANG_COMPACTION_LIMIT` and `gang wait`'s timeout all default to 300 seconds,
and that is a coincidence rather than a shared constant. Each bounds a
different thing, each fact bound's comment in bin/gang argues its own error
direction, and changing any one of them must not move the others — do not fold
them into a single value. `GANG_SEND_HOLD` is deliberately not one of that family:
a foreground sender blocked on its own send and a background waiter holding a
resume sit at opposite ends of the same trade, and the different number is the
point of it.

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
