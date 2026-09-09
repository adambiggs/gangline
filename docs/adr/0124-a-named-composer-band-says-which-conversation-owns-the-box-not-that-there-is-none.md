---
id: 0124
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0124: A named composer band says which conversation owns the box, not that there is none

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

claude-code draws the composer between two full-width rules and burns the ACTIVE
conversation's name into the opening one as soon as that conversation has a name. A
backgrounded parent session carries its own title; a selected in-process subagent
carries its task. The two frames are drawn identically and the name inside is free text,
so the band alone cannot say whose box it is.

## Consequences

Reading the band as "no composer drawn" answered both cases wrongly in different
directions: a titled parent went unreachable — status called a healthy agent occupied by
something it could not name, and delivery refused — while the child's box was refused
for a reason that named nothing an operator could act on.

The footer settles it. The permission-mode control belongs to the parent conversation
and is drawn under its composer in every mode; a selected child carries that child's own
controls, and the switcher marks the conversation in use with a filled ring rather than
with the keyboard caret. A named frame proven to be the parent's is read as the parent's
composer. Anything else is status 4 — a composer is drawn and it is not this agent's —
which Gangline carries through instead of flattening into absence, because driven on
2.1.241 typing into the selected child's box resumed the CHILD in the child's own
transcript. Refusal is the safe direction and every uncertainty takes it.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
