# ADR-0015: A context is renewed by cycling, not by summary

- **Status:** Accepted
- **Date:** 2026-08-02

## Context

ADR-0003 kept self-compaction a pillar: measure an agent's context, warn it
across a band, let it act. The act, today, injects the harness's own compact
command — a statistical summarizer that compresses the old window into a
prelude for the new one.

Two findings, both from running teams rather than from argument, say the
summarizer is the wrong half to lean on. First, the handoff an agent authors
and the summary the harness generates are independent halves that never see
each other: the summary carries no confidence labels, drops claims by
narrative peripherality rather than by importance, and cannot be steered
portably — the compact verb is universal across profiles, its steering is
not (law 9). Repeated summarization is summary-of-summary, the degradation
the literature calls context collapse. Gangline's working brake was never
the summary; it was re-grounding against files and commits.

Second, the channel that *is* trustworthy ratchets if it is operated as an
append-only log. A handoff that accretes superseded deltas is re-ingested
whole at every renewal, so the post-resume floor climbs toward the band
until a compaction buys almost no headroom at all — and nothing says so,
because no surface reports that renewal has stopped renewing. The handoff's
size answers to `wc -c` and the floor to the roster's context column; the
episode that measured this lives in the session archive, not here.

There is also an asymmetry the summarizer cannot offer: a context built
fresh re-reads role briefs, project instructions, and memory from disk. A
long-lived context holds a snapshot of all of them, and only a re-read
replaces it.

## Decision

**The authored handoff is the only channel.** It carries current state only,
with its epistemic labels intact. History lives where it already lives — the
session archive and the transcript on disk — complete, cold, and read on
demand. A complete cold backstop outranks a lossy warm one.

**The renewal act is a cycle: reset the agent's context without touching its
files or its work in flight, then deliver the role brief re-read from disk
and the handoff.** The universal spelling is wrap → retire → re-hitch with the
same flags; wrap and re-hitch are verbs that already existed, and the shape is
already how a model swap is defined. The middle step retires rather than drops
— the predecessor's window is kept and marked spent, so its closing remarks
outlive the context that wrote them, and that mark is also what lets a
successor read which verb renewed it. A profile may declare a cheaper
in-process spelling where its harness has one — an in-place context clear, a
fork at the first message — because which renewal verb a harness offers is a
harness fact, and harness facts live in profiles (law 4). The contract does
not move with the spelling.

**The channel is guarded at the verb.** A resume payload over a declared
budget is refused, loudly, with the remedy named: fold superseded state out;
history belongs in the archive. Gangline never trims the body — it is the
sender's, the same law `send` already obeys. The budget is declared policy
with an operator override, the pattern the band ladders set (ADR-0005,
ADR-0006), and it is measured in bytes, because bytes are what Gangline can
measure without inventing a token estimate it cannot verify. The refusal is
also what kills the silent ratchet: the moment the handoff outgrows its
budget, the one person compacting is told, at the moment it matters.

**Long work runs detached.** Logs with terminators, reconciled after the
fact — never harness-process state that a lifecycle event can kill. This is
what makes cycling safe to do at any checkpoint, and it is already how the
suite's own long runs are read.

**Refused, and this list is the durable core of the decision (laws 7 and
8):** no summarizer component inside Gangline; no memory daemon or curator;
no watcher comparing floors across cycles; no auto-folding of anyone's
handoff; no warn-but-proceed mode on the budget — a warning that lets the
oversized payload through is the degraded mode that reports healthy.

ADR-0003 is not superseded. The loop — measure, warn, act — and the delivery
substrate under it are unchanged. What changes is what the act delivers: a
cycle instead of a summary.

## Consequences

- The compact verb remains until the cycle verb lands; migrating it becomes
  a profile-spelling question, not a second loop. The build routes through
  the owner of `bin/gang` as a proposal before code.
- Two facts want a render before that proposal, and neither is decided here:
  whether a harness's background task handles survive its in-process clear,
  and who cycles the lead — a worker is cycled by the lead; the lead's own
  renewal must come from outside its dying context.
- The handoff contract in `roles/_common.md` gains the state bound, the
  livelock conduct line, and the exclusion of the handoff's own arrival.
- Whether a cycle worked is certified by the successor: a context that is new,
  recorded facts that came back correct, a handoff that was the only channel,
  and the verb that delivered it. It certifies by reading the substrate at its
  own hand rather than by believing its handoff — a handoff is written before
  the act that delivers it, so it can predict its own arrival but never witness
  it, and a prediction taken for a datum is the fabricated positive this
  decision's own measurement is most exposed to. A receipt from the caller
  corroborates and is welcome, but it is not required: such a receipt needs a
  caller still able to write one, and in a self-cycle the caller is the thing
  being retired, which would leave the renewal least attestable exactly where it
  is most autonomous.
- The successor can read the verb, and not only the outcome, because retirement
  leaves marks a drop does not, and the attestation depends on them by name: a
  predecessor kept as a `<name>~spent` window with `@gl_profile` unset, and
  `@gl_resume_bytes` set on a window that did not exist before the act.
  `cmd_cycle` is the only writer of either — `cmd_compact` records the same
  measurement on the window its agent is already in, and a drop leaves nothing
  behind to read at all. Naming them here is the point: a later change to
  retirement then breaks the attestation visibly, where an unnamed dependency
  would let it go on reporting a verb it could no longer see.
- The day-one consumer (law 5) is the lead's own next renewal — the first
  cycle runs on the team that decided this.
