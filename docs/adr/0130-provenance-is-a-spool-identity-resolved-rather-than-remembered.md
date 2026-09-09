---
id: 0130
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0130: Provenance is a spool identity, resolved rather than remembered

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

`gang hitch` and `gang adopt` record which context created an agent, because a lead's
helpers, an orphaned spool and a window with no recorded gang path are all easier to act
on when the creator can be named.

## Consequences

What is stamped is the hitcher's `@gl_spool` token, not its name: a name goes stale the
moment the hitcher is renamed, and `gang rename` deliberately leaves the token alone.
The name is resolved from the live windows at read time. The name witnessed at the stamp
is kept for one case only — no live window claims the token — and is reported as gone
rather than printed as though it were current. A registration that did not come from an
agent's window records a fixed sentinel that no minted token can spell. Harness session
ids stay out of it: the collar already stamps resumable session identity, and a second
copy would drift.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
