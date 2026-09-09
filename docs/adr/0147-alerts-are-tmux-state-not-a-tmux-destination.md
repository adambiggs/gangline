---
id: 0147
status: accepted
date: 2026-09-02
supersedes: []
superseded-by: []
tags: [tmux, alerts]
---

# ADR-0147: Alerts are tmux state, not a tmux destination

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

An alert asks for awareness, not navigation. Creating a normal window for one gave tmux
activity and bell policy permission to select it, made a persistent pseudo-agent part of
the team layout, and required a process solely to paint text already held in health
state. The alert center therefore writes active and unseen counts to session options,
renders them through one static status format, and opens detail only through a
client-requested `display-popup`.

## Consequences

Seen and resolved remain separate facts. Opening detail changes only the former; the
producer's recovery transition changes the latter. The key table is server-global, so
Gangline takes Prefix+A only when it is free, records the exact binding it installed,
and removes it only while that ownership witness still matches. Operator status content
and bindings are never inferred to be Gangline's from their appearance.

Binding changes serialize on an open descriptor for the tmux socket's existing
directory. The kernel releases that claim with the descriptor or process, so the
server-global decision needs no Gangline-created lock artifact or cleanup protocol.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
