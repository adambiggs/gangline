---
id: 0188
status: accepted
date: 2026-09-13
supersedes: [0186]
superseded-by: []
tags: [delivery, replies]
---

# ADR-0188: A sender marks the acknowledgement that owes no turn

## Context

ADR-0186 holds every correlated peer message whose matched threads are all
replies, on the ground that such a message is a pure acknowledgement chain and
earns no turn. The immutable thread modes cannot see the case that defeats
that ground: a lead reads an owner's readiness report, and in that same turn
sends the ruling the owner is waiting for. The ruling correlates only
to the report, which is a reply, so it is classed a no-reply envelope and held
for up to thirty minutes. The owner is already idle when it is sent, so no
ordinary delivery arrives to carry it; the owner acts on the last state it saw,
sends a second report that crosses the held ruling, and the lead restates.
Every occurrence costs two extra turns and one round of stale work, and the
hold saves a turn only when another ordinary delivery arrives within its window
to carry the envelope, which an idle owner never has. The same class appears in
the other direction: an owner's clarifying question sent in the turn that read
a ruling matches only that reply and is held too.

Three remedies were weighed. Releasing held envelopes at the recipient's next
turn boundary does nothing here, because the recipient has no next boundary: it
is already idle, and a busy recipient's next boundary is exactly what the
ordinary parked spool already provides. Inferring intent from message text was
rejected by ADR-0186 itself: "approved, go ahead" is as short as "noted". Only
the sender knows whether the message needs acting on, and the costs are not
symmetric: a bare acknowledgement that wakes its reader costs one bounded turn
that opens no debt, while a held ruling costs the stale work, two turns, and the
thirty-minute deadline.

## Decision

A correlated peer message whose matched threads are all replies wakes its
recipient like any other message: live when the recipient is idle, parked in
the ordinary spool when it is mid-turn. It still opens no debt, as ADR-0164
holds. The sender opts into the quiet path with `gang send --ack`, which means a
bare acknowledgement of replies read this turn with nothing for the reader to
act on. An `--ack` message takes the deadline-backed hold ADR-0186 describes,
unchanged: accepted into the hidden spool namespace, promoted by the next
ordinary composer-verified drain, forced by the thirty-minute deadline service.

`--ack` is refused before anything is typed when the message answers any
matched request, because that peer is waiting for the answer, and when it
matches no thread at all, because a fresh request must wake. It is refused with
`--live-only`, which never holds, and with `--no-reply`, which waives a
request. An `--ack` whose supersession inherits a request correlation is set
aside with a stderr line and wakes, as ADR-0186 already requires of inherited
threads. Gangline's own reply-release stop alerts keep the quiet path: they are
authored by Gangline, so the sender is known to have nothing to act on.

The contract tells agents to send a bare acknowledgement with `--ack` and
everything else, rulings included, without it.

## Consequences

The crossed pair cannot recur from the classifier: a ruling sent in the turn
that read a report reaches an idle owner at once. The thirty-minute hold now
serves only messages whose sender declared them inert, so an idle recipient of
an unmarked all-reply message pays one turn it would previously have avoided
when other traffic happened to arrive within thirty minutes. Every other part of
ADR-0186 stands: the hold, promotion order, deadline service, retry budget, and
the visibility of held entries in `status`, `roster` and `mail` are unchanged,
so this record supersedes only ADR-0186's default for unmarked messages; the
rest of that record stands as written.

Ping-pong does not return: the woken recipient's message opens no debt, so an
acknowledgement chain still ends on any message, and the reader of a bare
acknowledgement owes nothing for it. An agent that forgets `--ack` costs its
reader one turn; an agent that marks a ruling `--ack` recreates the failure, and
the refusal on answered requests catches the common case where the reader is
demonstrably waiting.

This decision is falsified by a correlated all-reply message sent without
`--ack` that is held rather than delivered to an idle recipient, by an `--ack`
message that wakes an idle recipient while the deadline service is armable, by
an `--ack` accepted on a message that answers a matched request, or by a woken
all-reply message that opens reply debt on its recipient.
