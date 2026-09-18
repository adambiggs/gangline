---
id: 0208
status: proposed
date: 2026-09-18
supersedes: []
superseded-by: []
tags: [messaging, hooks]
---

# ADR-0208: Gangline records no peer reply obligation

## Context

ADR-0010 through ADR-0012, ADR-0148 through ADR-0151, ADR-0157, ADR-0163
through ADR-0165, ADR-0186, ADR-0188 and ADR-0201 made a verified peer message
owe a reply that the Stop hook enforced. Across the recorded friction corpus
that enforcement blocked 624 Stops, 78% of the replies it forced were
acknowledgement or status shapes, the machinery spawned at least 13 issues of
its own, and nothing records it saving a reply that would otherwise have been
lost.

## Decision

Gangline keeps no reply-debt state and its Stop hook never blocks on one. `send`
loses `--ack`, `--no-reply` and the deferred hold; status, explain, alerts and
roster carry no debt lines. An assignment is answered by its completion report,
and `safe-to-drop` proves that report by the `@gl_delivered_<token>` marker
delivery sets on the recipient's window.

## Consequences

This removes coordination state, consistent with ADR-0001 and ADR-0005.
Legacy spool entries still deliver as peer mail. Agents may leave a message
unanswered and nothing will stop them.

Falsifier: a hitched child idles without reporting its assignment and its lead
cannot see that it never did (the #176 shape).
