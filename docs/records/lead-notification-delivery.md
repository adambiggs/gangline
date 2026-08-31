# Lead notification delivery — design

> Status: Implemented 2026-08-30.

## Purpose

An optional `gang notify` target needs a durable, attributed indication when an
agent can no longer make unattended progress. The target is not a Gangline
role: it is an operator-selected receiver, as for stall notes.

## Facts that notify

Only four positively established transitions notify:

- `!blocked!`, after a collar's `collar_blocked` reader says that a turn ended
  without producing work;
- `!bricked!`, after a collar's fatal-turn reader says the current session
  cannot succeed; and
- `!dead!`, after tmux says every pane in the registered window has exited.
- `!harness-lost!`, after a collar's recorded host-process identity no longer
  names that harness process.

`~idle~`, a completed turn, quiet paint, and a dialog-shaped screen do not
notify. None proves that the assigned work finished or that a live turn stopped
being useful. A collar without the relevant reader provides no substitute.

For Claude's refusal path, `idle_prompt` is a wake rather than a verdict. It is
qualified by `state_now`: ordinary idleness stays silent, while a verified
blocked or bricked state is forwarded. It may be the only native wake after a
failed turn, so no Stop is assumed.

## Delivery and retention

Each note goes through the ordinary attributed send path. A free notify target
receives it live and verified. A busy or compacting target receives a committed
spool entry, which drains at its next verified opportunity; the source records
that the note was accepted. A target that is genuinely gone leaves the pending
note and a visible delivery failure on the affected agent, rather than a false
acceptance. A later cooperative tick retries that recorded note without
requiring a second native wake.

The source stores the notified state and emits at most one note for that
transition. A later transition is a different note. The accepted latch is
committed only after live delivery or a safely parked entry; until then a
pending record retains the exact transition for reconciliation. Thus a lead
that briefly has no window receives the original fact after it returns, not a
false claim that no event occurred.

## Dead-state reconciliation

Tmux's `pane-died` hook starts a fast cooperative tick when it fires, but retained
corpses can miss that hook. Gangline's existing cooperative tick reconciles the
same window state on its next pass. The comparison is only `!dead!` against the
notified-state latch; it does not add a new poller or reclassify every state.
Thus the guarantee is one dead note by the next cooperative tick, not an
untrue claim that every process exit is immediately observable. The body names
`tick`, the state reader that made the verdict.

Hook then tick, or repeated ticks while the notify target is absent, leave one
transition and one eventual note: neither may duplicate it.

## Codex process loss inside a live pane

For the ordinary Codex launch, tmux's `pane_pid` is the `codex` process. A
collar may establish a positive identity for that exact process — PID plus its
kernel start stamp — and the cooperative tick compares the current identity to
the recorded one. A missing, replaced, or zombie process after that positive
baseline is `!harness-lost!`: it is a re-hitch problem and produces the same
durable lead note as other unusable states. A wrapper shape whose pane process
cannot be positively identified as Codex supplies no liveness assertion.
`gang explain` says whether this identity is recorded, not recorded, lost, or
unreadable, so unavailable coverage is never shown as a healthy verdict.

This is deliberately **not** a health check. A process that remains alive but
is unable to spawn work is indistinguishable from one doing a long operation;
rollout silence and an unclosed task bracket are not evidence either way. That
case stays unknown until a live specimen establishes a stronger native signal.
An unclosed `task_started` bracket and a `turn_aborted` record whose reason is
`interrupted` are explicit negative controls: both occur in healthy Codex
sessions and neither participates in this predicate.
