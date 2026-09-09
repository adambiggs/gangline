---
id: 0111
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0111: A native first-run prompt is answered by a person, and the wait for one is bounded

## Context

Startup must expose operator-owned prompts without waiting forever.

## Decision

Gangline answers no native dialog, so a positively identified first-run gate leaves
exactly one thing to do: a person answers it. Holding the caller's terminal until that
happens made an unanswered prompt stall the caller too — and the caller is often another
agent, which cannot answer a native prompt at all, so one gated boot stopped two agents
and neither could report why.

## Consequences

`GANG_GATE_LOOKS` bounds the observations `hitch` spends on a prompt that is still
unanswered, and reaching it exits 4: distinct from a hitch that failed and from one that
delivered, so a caller can tell the three apart. Nothing else changes — the window is
alive, the harness is running behind its prompt, and the attributed contract is
committed to the ordinary spool. What ends is Gangline's claim on the terminal.

Observations, not seconds. Every readiness pass that sees the gate returns on its first
reading, and a duration is spent instantly under a stopped clock, where the give-up path
could then never be driven at all.

`gang up` is exempt. The budget releases a caller who cannot answer, and `up` has
already attached its caller to a tmux client showing the prompt: there is no stalled
third party there, and the give-up would detach the one client that can answer —
reporting that nobody answered by removing the means to.

The budget cannot be recovered by waiting longer, because the prompt still owns the
composer. The cooperative tick changes what happens after it clears: `gang tick` retries
the existing spool without waiting for that idle agent to raise a boundary. Answering is
a native persisted choice, and codex's hook trust in particular is keyed on the hook
definition rather than on the bytes of the script the hook command names — so ordinary
development on `bin/gang` leaves an answered gate answered, while a `gang` at a
different path raises a new one.

Revisit if a supported harness safely answers its own first-run prompt or waits without
a bound.
