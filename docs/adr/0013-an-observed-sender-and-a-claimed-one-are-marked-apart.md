---
id: 0013
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0013: An observed sender and a claimed one are marked apart

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Gangline reads the sender off the calling window where it can see one and refuses a
claimed name there; where it cannot see one the name stands as claimed. Both remain
true, and the envelope now says which of the two it is carrying: a name supplied by
`--from` because no window was visible goes on the wire as `self-declared:<name>`, in
both tags.

## Consequences

The two used to arrive identical, so any process that could reach the socket and the
executable could sign as a peer and be read as one — a harness's sandboxed command
surface strips the tmux environment, which makes `--from` mandatory there and made that
case indistinguishable from a pane Gangline had watched. The operator's own shell is
marked by the same rule, because it is the case a sandboxed process cannot be told apart
from.

This is a label, not a check. Nothing here proves who is calling and nothing here may:
authentication, generation fencing and anti-tamper are banned, and the repair for
presenting a claim as an observation is to stop doing that. The marking cannot collide
with an observed sender because `:` is not a usable agent name, and the contract tells
receivers to treat a marked sender as unverified.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
