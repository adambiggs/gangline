---
id: 0003
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [roles]
---

# ADR-0003: The shipped lead and worker briefs carry reusable delegation terms

## Context

Arc instructions vary, but the ownership and review terms of delegation do not.

## Decision

Ship reusable ownership, review, evidence, gate, and reporting terms in the `lead` and
`worker` briefs; keep each arc message specific to that arc.

## Consequences

`lead` and `worker` ship as the two ends of one delegation. A team with a lead brief and
none for the agent it delegates to left every arc owner briefed by a message written
fresh for that arc, so the terms an owner is always held to — commission the review,
fail a test before fixing, leave evidence a lead can read, go through the push gate,
send one report — were rewritten per arc, unevenly, and reached an owner after its first
turn or not at all. Those terms are the same in every arc, so they belong in prose
delivered with the contract, leaving the arc's own message to carry only what is
particular to that arc. Neither brief is required: a hitch may still be role-less, and
an operator file replaces either whole.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
