---
id: 0082
status: accepted
date: 2026-08-13
supersedes: []
superseded-by: []
tags: [gates]
---

# ADR-0082: Startup contracts park behind native first-run gates

## Context

A first-run prompt can outlive the boot bound before any agent turn or Stop event
exists.

## Decision

On positive pane evidence of an operator-owned startup prompt, commit the attributed
startup envelope to the ordinary spool and keep the direct hitch as foreground owner
until a composer appears; `up` exposes the same prompt while observing it.

## Consequences

A first-run gate can outlive any boot bound before an agent turn exists, so a Stop-only
spool would strand the contract and a longer wait would only move the failure. On
positive pane evidence of an operator-owned startup prompt, hitch immediately commits
the attributed startup envelope to the ordinary window spool and owns it until the
universal tty surface exposes a composer. Direct `hitch` keeps that observer in the
foreground; `up` exposes the gated window in its tmux client while the same invocation
observes beside it. The verified drain retries a pre-keystroke refusal and accounts for
an exact entry retired by a crossed native drain. This remains correct if an operator
declines configured hooks at a native security gate, adds no harness event or persistent
fact, and leaves the envelope inspectable if hitch is interrupted.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
