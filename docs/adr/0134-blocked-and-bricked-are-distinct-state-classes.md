---
id: 0134
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0134: Blocked and bricked are distinct state classes

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

`!blocked!` is not folded into `!bricked!`.

## Consequences

Bricked says the session cannot work and is repaired by re-hitching; blocked says this
input got no work and the same window is revived by re-driving it. Collapsing them would
send an owner down the wrong repair. It sits below both `!occupied!` and `!bricked!` —
unrecoverable outranks recoverable — and above `-busy-`, since a turn-ending record
makes a busy marker still on the screen retained paint.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
