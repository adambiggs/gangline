---
id: 0055
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0055: A window name carries last-witnessed state

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

A gang-managed window wraps its agent name in the glyph of the state Gangline last
witnessed, so a tmux status bar shows the team at a glance without asking anything.

## Consequences

It is written at the observation points and hook events that already determine state,
never by a patrol, so between observations it can be stale and `gang roster` remains the
live-computed truth. Addressing is always the bare name: every lookup strips the glyph,
and two windows that strip to one name are a refusal rather than a guess, because a
silently wrong target is worse than no target. tmux appends its own flags after the
name; that rendering is documented, not fought.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
