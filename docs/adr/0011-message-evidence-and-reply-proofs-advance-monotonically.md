---
id: 0011
status: accepted
date: 2026-09-02
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0011: Message evidence and reply proofs advance monotonically

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

Keep message metadata immutable and put prompt, delivery, and settlement facts in
separate monotonic options, so concurrent native and transport writers cannot lose one
another's proof.

## Consequences

Legacy peer spool entries lacking correlation stay unknown. A launch-installed Stop
promise cannot be adopted, and a Claude hitch refuses unless the operator explicitly
disables its finite native consecutive-block cap. A correlated reply discharges the
request once either arrival witness stands beside the settlement proof: the reply is
itself evidence the request reached the debtor, each arrival proof has exactly one
writer that has already run, and a record failing closed on the weaker missing witness
left the debtor a Stop block its own reply could not clear. A settlement proof with no
arrival witness at all stays unknown. Join retained metadata and proofs in one linear
pass so the audit trail does not make Stop queries quadratic.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
