---
id: 0193
status: proposed
date: 2026-09-13
supersedes: []
superseded-by: []
tags: [codex, hooks, trust]
---

# ADR-0193: A hook-trust recovery is an attended native launch

## Context

Codex treats a changed Gangline hook command as untrusted and opens a native
review menu before its composer. The preflight correctly refuses that unattended
launch, but its held diagnostic had no usable team recovery surface. An
automatic answer would grant a decision that belongs to the person at the
terminal.

## Decision

`gang trust codex -d DIR` opens a disposable, unregistered window in the
current team with the collar's exact hook configuration and no preflight. It
prints the window selector and sends no native-menu key. The preflight marks
its held refusal, and roster prints the same command. After the operator
selects the review window, answers Codex's native trust choice, and quits it,
the exact re-run of the refused hitch safely reuses only that marked,
dead pane.

## Consequences

Recovery is visible without weakening the preflight or automating trust. The
temporary window is not an agent. This decision is falsified if the command can
answer the menu without a person, or opens a different hook configuration.
