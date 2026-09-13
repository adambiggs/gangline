---
id: 0189
status: accepted
date: 2026-09-13
supersedes: []
superseded-by: []
tags: [tick, operations]
---

# ADR-0189: A tick pass stops at its budget and resumes after a cursor

## Context

One cooperative tick worker visits every hitched window under a fixed 60-second
hard deadline; ADR-0166 bounded the worker to two passes so that a busy team
could not keep one owner alive until that deadline killed it. The deadline
still fired on a seventeen-agent team with nothing to blame but the roster: a
visit costs on the order of a second of tmux round trips, a Codex window several
times what a Claude Code window costs, and a pass measured on that team took
twelve seconds idle and twice that after pane writes, so two passes plus
contention reached the deadline. The controller then killed the worker, health
was committed failed, and every later command printed `last tick failed` for a
team whose only fault was its size. A second failure shared the alert: a
contender inside a harness sandbox, reading the lock through its own pid table,
could not see the owner and committed failed health for a reader limitation.

## Decision

A pass has a soft budget of two thirds of the worker's deadline, measured from
the worker's start. Once the budget is spent the pass stops before its next
visit, having made at least one, records the last visited window as the team's
cursor under its health directory, and reports the pass partial. The worker
commits `ok` health with a note counting the agents visited against the roster,
runs no in-process rerun, and arms one successor tick on a fresh deadline; that
successor is a continuation and arms no continuation of its own, though a dirty
marker set during its pass is still owed a successor. Every pass starts after
the cursor, so ordinary post-command ticks finish a roster too large for two
workers, and a pass that reaches everyone removes the cursor. The deadline
itself is the operator setting `GANG_TICK_DEADLINE`, whole seconds from sixty
to an hour, validated by `gang tick` before its controller starts and exported by the
controller to its worker, which accepts only that number. A contender that
cannot resolve the lock owner's pid namespace exits with status 77 and writes
no health; `gang tick` reports that as the command's failure, not the team's.

## Consequences

No pass is killed for being thorough: the budget leaves the deadline room for
the visit in flight and for committing the result, so health reflects what the
pass found rather than the roster's size. An agent is reached within a bounded
number of passes, at worst the roster divided by the visits one budget affords,
and a synchronous `gang tick` still returns inside one deadline. The cost is
that one pass no longer promises every agent: a caller that needs a particular
window visited runs `gang tick` until health carries no partial note. A team
whose passes are always partial ticks more often, not longer. Operators with
larger teams raise `GANG_TICK_DEADLINE`; the lock's expiry and reclaim ages
scale with it. A sandboxed reader no longer alerts on an owner it cannot see,
and a real dead namespace owner is still reclaimed by the next host reader.
Falsifiers: a worker killed by its controller on a team whose visits each fit
inside the budget; a cursor that fails to advance across two partial passes; a
partial pass that arms a chain of more than one continuation with no dirty
marker; failed health written by a contender that printed `which this process
cannot see`.
