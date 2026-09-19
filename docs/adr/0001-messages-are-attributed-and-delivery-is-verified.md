---
id: 0001
status: accepted
date: 2026-08-04
---

# ADR-0001: Messages are attributed and delivery is verified

## Context

A message crosses from one harness's pane into another's composer, where being
typed, being accepted and being submitted are different facts. A sender that
cannot be seen can still claim any name: a harness's sandboxed command surface
strips the tmux environment, so its `--from` read exactly like a pane Gangline
had watched (#131).

## Decision

Every message names its sender and travels in a nonce-bound envelope. Gangline
reports delivery only after the target composer visibly accepted and submitted
it. It reads the sender off the calling window where it can see one and refuses
a claimed name there; a name it could not observe goes on the wire as
`self-declared:<name>`.

## Consequences

Gangline is single-tenant and claims no authentication: the marking is a label,
and the contract tells receivers to treat a self-declared sender as unverified.
A delivery Gangline cannot verify exits 5 rather than being reported sent or
retried.
