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
a submit `witness`, and `inbox/{tmp,new,cur,failed}/`. State and witnesses are
replaced atomically. Pending messages live in `new/`; settled receipts retain
their latest outcome, while the audit log retains history.

Commands work over requested agents and pending work, independently of settled
history. Each agent has its own lock. `gang drop` removes the registration and
its files; `gang down` removes the team directory.

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
blocks an unconfirmed or altered compaction resume; a compaction-end hook
synchronously confirms completion before returning. Turn-end and
compaction-end hooks also schedule a detached tick for other work.

Claude Code's native prompt ID ties asynchronous failure or success to the
submit witness. A late callback cannot change a newer identified turn. Missing
native identity leaves attribution unknown until an identified successful
boundary establishes it. Failure state is saved before probing the pane so a
probe error cannot consume the boundary notice.

An observation failure reports unknown with its reason. It breaks continuous
screen observation; a later successful probe can establish busy or idle again.
A locked status read reports unavailable observation. A draft without native
busy evidence is blocked on unsubmitted input; observation never submits it.

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

Compaction waits for freshly observed native idle. Gangline submits the resume
to the native queue as compaction starts, ahead of later input. A synchronous
completion hook confirms that compaction finished in the same native session
before the queued resume may run. Refusal is failure; missing completion is
unconfirmed, and the submit hook blocks that resume. Uncertain native input is
never sent a second time automatically. Codex can merge a later Enter steer
into the same prompt; the exact resume envelope must lead that prompt. The
submit hook atomically admits that envelope once. If the composer is occupied
before the resume enters, Gangline fails the operation and withholds its
continuation rather than submitting it late. Recovery cancels a published
resume that has no recorded native submission.

## Watchdog

A whole-team tick replaces a transient user-scheduler timer and re-arms it
before observing agents. Gangline arms the initial timer when you hitch or
adopt an agent. A scoped hook
tick preserves an existing deadline so activity in one agent cannot postpone
idle peers. Whole-team ticks skip occupied agent locks; detached scoped ticks
can wait for a lock so native boundary notices survive contention.

Linux uses systemd user timers. macOS uses a transient launchd job and a plist
in the team directory, outside login-loaded LaunchAgents. The expiry shell
lets its tick child survive job removal so it can re-arm. Both depend on an
awake host and available user scheduler.

Timer transactions have their own nonblocking lock, released before agent
work. Generation tokens reject superseded timers. Last drop and team removal
disarm the timer. Contention or scheduler errors fail visibly; uncertain
launchd cleanup preserves the plist for a later attempt. Unsupported hosts
log `watchdog_unavailable` once per team and retain ordinary ticks.

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
| `options` | Effort and role-prompt argument templates. |
| `primitives` | Built-in native behaviors, readings, queue witnesses, and optional limits queries. |
| `actions` | Interrupt, compact, and recovery key sequences; optional refusal patterns. |
| `context_bands` | Named context thresholds by model. |

A `role_prompt` option places standing instructions in the native system
prompt and leaves the assignment in the first message. Other collars receive
both together. Custom collars without supported transcript discovery rely on
their native CLI to validate resume identities.

## Design principles

Read these before changing behavior. A conflicting change needs an explicit
decision about the principle or implementation.

- Use universal surfaces: tmux windows, keystrokes, pane capture, and a CLI.
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
