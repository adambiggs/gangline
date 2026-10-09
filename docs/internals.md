# Internals

## Packages and state

Gangline is a Go module built into the `gang` binary.

| Package | Responsibility |
| --- | --- |
| `core` | Pure state transitions: state and event in, new state and actions out. |
| `store` | Agent state, inboxes, locks, and the audit log. |
| `substrate` | Terminal operations, implemented through tmux. |
| `harness` | Collar loading, native launch, screen interpretation, and readings. |
| `cmd/gang` | Commands that coordinate these packages. |

`core` has no I/O or clock. `harness` and `substrate` do not import it.
Collars live in `harness/collars/` or `GANG_COLLARS`.

Team files live under `STATE_ROOT/teams/TEAM/`. `team.json` holds the curfew;
`log.jsonl` is the append-only audit log. Name claims in `names/` point to
immutable hitch IDs. Each `agents/ID/` contains `agent.json`, a `lock`,
a submit `witness`, and `inbox/{tmp,new,cur,failed}/`. A turn-end hook
leaves a `background` count when native background tasks are still pending,
and the next turn boundary removes it. A permission-request hook leaves a
`permission` witness that any later hook or an idle screen removes.
An unverified native session change fails the hitch and saves the rejected
normalized witness and registered session in `native-session-conflict`.
Later submit hooks cannot overwrite this private diagnostic snapshot; another
rejected boundary can replace it. It is not the raw hook payload. Copy it
before dropping the agent, which removes it with the registration.
`status-hooks` coalesces auxiliary display observations under its own short
writer lock. Terminal boundaries remain pending until a sweep handles their
native outcomes and acknowledges the sequence. Roster reads cannot consume
that recovery work. Acknowledgment prunes older observations, so storage
grows with pending boundaries rather than settled history. Both files are
removed with the agent. State
and witnesses are replaced atomically. Pending messages live in `new/`; settled receipts retain
their latest outcome, while the audit log retains history.

Commands work over requested agents and pending work, independently of settled
history. Each agent has its own lock. `gang drop` removes the registration and
its files; `gang down` removes the team directory. Registration, drop, and
whole-team teardown share a nonblocking lock under `STATE_ROOT/team-locks/`
so the agent list cannot change while the confirmation prompt is open.

## Message path

`gang send` publishes an envelope to the recipient's inbox. Before typing,
Gangline records its intent and verifies that the native harness is the pane's
foreground process. A free composer can receive input during a turn when the
collar supports it. A native prompt or unsubmitted draft keeps input blocked.

After submission, an exact native hook witness establishes `delivered`.
A freshly observed queue entry can establish `accepted` when its declared
layout includes the sender and full one-time-token opener and the composer
is empty. A truncated preview does not establish full-message submission.
Startup assignments and context-band notices require exact hook proof.

Without either receipt, input remains unverified and the command fails
visibly. A later exact hook can reconcile a retained uncertain or accepted
receipt. If a process dies between intent and outcome, recovery records the
uncertainty and does not automatically paste the message again. Explicit
startup recovery may submit the original envelope still identified exactly
in the composer.

## Native events and observation

Most hooks append an event and exit without acquiring the agent lock. Submit
hooks publish a witness and schedule detached reconciliation. A submit hook
blocks an altered compaction resume and admits the exact queued continuation
once; a compaction-end hook records completion durably. Blocking the current
compaction's own note fails that compaction, since input waits behind a note
that will not arrive. The next agent access cancels stale context-band notices
before delivery. Submit, activity, permission-request, turn-end,
compaction-start, compaction-end, and blocked submit hooks schedule detached
scoped ticks. Hook-first as much as possible; pane scraping only for additional
validation or telemetry, or where hooks leave a gap. These ticks reconcile
retained native boundaries and repaint status without capturing a pane while
hook evidence is fresh. Native prompt identity rejects a delayed finish from
an older turn; receipt time alone cannot order turns.

The freshness and stale-probe interval are defined by `statusHookFreshness` in
`cmd/gang/status_hooks.go`. Expired evidence triggers a pane check, whose
result remains authoritative until another hook or the next probe deadline.
The log names each status capture's gap: missing or expired hooks, startup,
input recovery, interrupt completion, compaction validation, capacity recovery,
terminal capacity before judging a wake, or a collar requiring screen telemetry.
The stale check catches permission dismissal without a hook, terminal-only
prompts and capacity errors, native exits, and stable-screen wedges. Codex has
no mapped failure hook; its transcript telemetry and these gap checks remain.
Delivery separately validates composer contents, foreground ownership and
receipts before sending input. A hook cannot authorize typing over a draft.

Claude Code's native prompt ID ties asynchronous failure or success to the
submit witness. A late callback cannot change a newer identified turn. Missing
native identity leaves attribution unknown until an identified successful
boundary establishes it. Failure state is saved before probing the pane so a
probe error cannot consume the boundary notice.

An observation failure reports unknown with its reason. It breaks continuous
screen observation; a later successful probe can establish busy or idle again.
A roster or status read of an agent whose state another operation holds
reports the saved record without probing the pane. A draft holds message
delivery; its pane symbol remains idle unless a witnessed turn is open.
Observation never submits it.

## Context and compaction

Readings are recorded as `observation` events with a kind, source, and status.
Sources include `native-hook`, `session-log`, `status-line`, and `screen`;
statuses are `observed`, `unknown`, and `error`.

Claude Code context is input tokens plus cache reads and writes from its
status line. Codex uses `last_token_usage.total_tokens` and
`model_context_window` from native `token_count` records. These native formats
are parsed explicitly; unavailable or invalid readings are not estimated.

An upward context-band crossing queues a notice and records the deciding
reading in `context_band_crossed`. Publication intent is persisted beside the
native cursor so interrupted publication can resume. Unknown readings do not
reset crossings. A lower reading or model change permits later crossings.
After a notice and completed compaction, the next reading establishes a new
baseline.

Compaction waits for freshly observed native idle and a recorded finish for
any witnessed native turn. Gangline submits the resume
to the native queue as compaction starts, ahead of later input. If native work
restarts before the compaction submit key, a collar's `compact_defer_clear`
action withdraws the exact staged command and retains the request and resume
for the next idle boundary. A changed composer is left untouched. A collar's
compaction spinner confirms the command started before Gangline submits the
resume; a fresh refusal withholds it before it enters native pending input.
A native completion hook records that compaction finished in the same native
session.
The submit hook admits the exact queued resume even if that evidence is still
missing, so the continuation cannot be stranded. Missing completion remains
unconfirmed. Refusal is failure. Uncertain native input is
never sent a second time automatically. Codex can merge a later Enter steer
into the same prompt; the exact resume envelope must lead that prompt. The
submit hook atomically admits that envelope once. If the composer is occupied
before the resume enters, Gangline fails the operation and withholds its
continuation rather than submitting it late. Recovery cancels a published
resume that has no recorded native submission.

## Watchdog

A whole-team tick replaces a transient user-scheduler timer and re-arms it
before observing agents. The timer supplies a five-second fallback between
native hook refreshes; scheduling and observation work can delay it.
Gangline arms the initial timer when you hitch
an agent. A scoped hook tick preserves an existing deadline so activity in one agent cannot postpone
idle peers, except while `[watchdog-unavailable]` is shown, when it replaces the
recorded timer because that timer may have elapsed. Whole-team ticks skip occupied agent locks; detached scoped ticks
wait for a lock so native boundary notices survive contention. Activity,
permission-request, and compaction-start ticks skip an occupied agent rather
than accumulate waiting refreshes.

The next timer counts from arming before the sweep, not from its completion
or a fixed clock grid. A scoped hook tick preserves the pending deadline
unless it must repair an unavailable timer. Hook ticks and watchdog sweeps
can overlap; each agent lock serializes work on that agent, and the separate
scheduler lock serializes timer replacement. Sweeps skip occupied agents.
The cadence is neither a minimum refresh interval nor a maximum staleness:
hooks can refresh sooner, while scheduler delay, contention, an unreadable
pane, and the collar's open-turn quiet fallback can delay a conclusive state.

Linux uses systemd user timers. macOS uses a transient launchd job and a plist
in the team directory, outside login-loaded LaunchAgents. The expiry shell
lets its tick child survive job removal so it can re-arm. Both depend on an
awake host and available user scheduler.

Timer transactions have their own lock, released before agent work. Callers
that arm never wait for it. Generation tokens reject superseded timers. Last
drop and team removal disarm the timer and wait a bounded time for a holder of
the lock; one that outlasts the wait fails the drop or down with the lock's
path and leaves the timer armed. Contention or scheduler errors fail visibly; uncertain
launchd cleanup preserves the plist for a later attempt. Unsupported hosts
log `watchdog_unavailable` once per outage and retain ordinary ticks; the next
armed timer logs `watchdog_available`. An elapsed timer's tick that cannot
re-arm it, because another tick holds the scheduler lock or the transaction
fails, also marks the outage so the next caller that can arm replaces it. So
does any caller whose disarm or arm fails.

Replacement costs bounded scheduler stop/start work and, on macOS, plist
write/removal. Whole-team observation visits registered agents. Scoped ticks
with an armed timer read its marker. Scheduler calls have bounded budgets;
the implementation constants are in [`watchdog.go`](../cmd/gang/watchdog.go).

## Teardown and recovery

Gangline records native process identity at spawn. Drop validates that identity
when acquiring kernel handles and records selected teardown targets before
signalling. This protects against reused process IDs and supports incomplete
drop recovery. It can find observable descendants, including separate process
groups; descendants already reparented out of the recorded tree may be missed.

Provider errors can end a turn without a Stop hook. Tick can recognize such
an error and send a continuation, backing off within the configured capacity
budget. Native choice prompts remain the operator's decision.

## Collar schema

A collar is a CUE file with a top-level `collar` value checked against
[`harness/schema/collar.cue`](../harness/schema/collar.cue). Its filename must
match `collar.name`. Unknown fields or primitive names fail before launch.

| Field | Declares |
| --- | --- |
| `launch` | New, resumed, and probe commands. |
| `hooks` | Native callback arguments and payload interpretation. |
| `models` | Model discovery and selection. |
| `options` | Effort and role-prompt-file argument templates. |
| `primitives` | Built-in native behaviors, readings, queue witnesses, and optional limits queries. |
| `actions` | Interrupt, compact, and recovery key sequences; optional refusal and running-compaction patterns. |
| `context_bands` | Named context thresholds by model. |
| `usage_bands` | Named native provider-usage thresholds by window. |

A `role_prompt_file` option passes the path of an agent's private startup file
to a native system-prompt-file argument. Gangline writes the composed contract,
doctrine, and role brief to that file before launch and keeps it until drop.
The assignment stays in the first message. Collars without this option receive
the standing instructions in that message. Custom collars without supported
transcript discovery leave resume identities to their native CLI.

## Design principles

Read these before changing behavior. A conflicting change needs an explicit
decision about the principle or implementation.

- Use universal surfaces: tmux panes, keystrokes, pane capture, and a CLI.
- Name every message's sender, distinguishing observed and declared identity.
- Reserve `delivered` for verified submission; preserve uncertainty.
- Describe native differences in collars so the CLI stays harness-neutral.
- Build behavior with a current caller and a cleanup path for every file.
- Fail visibly when an operation or observation cannot be established.
- Prefer instructions an agent can follow over additional machinery.
- Keep agent work independent; hooks must not hold up the harness, and settled
  history must not make operations slower.
- Choose the smallest fix that addresses the cause and state its cost per
  operation and how that cost grows.
- Leave trust, login, approval, and sandbox choices with the operator; collars
  cannot loosen them.

See [contributing](../CONTRIBUTING.md) for the local gate and commit rules.
