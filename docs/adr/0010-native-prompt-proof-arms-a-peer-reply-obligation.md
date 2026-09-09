---
id: 0010
status: accepted
date: 2026-09-03
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0010: Native prompt proof arms a peer reply obligation

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

Arm one durable obligation per verified message from an observed peer, not per hitch
relationship or visually latest prompt, and arm it on the native prompt proof alone.

## Consequences

Delivery proof means the harness accepted the paste; a harness that queues typed input
mid-turn submits it only at a later boundary, and the record's metadata lands before the
paste at all, so a request with delivery proof or none is a message in flight. Demanding
a reply to it blocked the debtor for a message it had not read, and escalated a record
that read as owed seconds later. The prompt proof is the harness's own witness that the
request entered context and needs no second witness. Clear the obligation once Gangline
has accepted an outbound message correlated to that peer request, typed into the peer's
composer or parked in its spool: a parked reply drains only at the creditor's own
boundary, so a debtor held until delivery was blocked at every Stop for an answer it had
already given, and each block invited another copy. The drain rewrites the same
monotonic settlement beside its delivery proof; a spool lost after acceptance loses the
reply, not the settlement, and the parked entry stays visible to `status` and `mail`
until it drains. The reply body is prose and is never parsed for an acknowledgement
phrase. Mark the correlated envelope as a reply so its recipient does not acquire
reciprocal debt, at every stage of that record's arrival evidence: the mode is stamped
in the same immutable metadata as the rest, so a partial reply record is as much a
non-obligation as a complete one and blocking on one named no action. Operator and
self-declared input changes none of this state, while orphaned or malformed provenance
blocks Stop as unknown. Message-scoped records preserve crossed turns and multiple
senders without adding a coordinator or a protocol outside the existing verified
transport.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
