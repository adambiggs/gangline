---
id: 0198
status: accepted
date: 2026-09-16
supersedes: []
superseded-by: []
tags: [lifecycle]
---

# ADR-0198: An agent declares itself safe to drop

## Context

Teardown already trusts live window registrations over a caller's claim
(ADR-0122) and resolves hitch provenance from the current spool identity
rather than a remembered name (ADR-0130). Nothing yet lets a registered agent
authorize its own teardown: a drop today is an external judgment call, blind
to whether the agent's completion report was delivered or its own children
are still live.

## Decision

A registered agent may mark its own window safe-to-drop once its completion
report is delivered and every child it personally hitched is no longer live
or was closed through its own explicit closeout. Setting the mark and
accepting actionable delivery serialize on the guard delivery already takes:
nothing is accepted, and nothing addressed goes silently undelivered, at the
moment a window disqualifies itself from more work. Once set, the spool
identity accepts no further actionable delivery; resuming needs a new
registration, not a flag flip. The window's direct live hitcher normally
performs the drop; where hitch provenance can only be witnessed rather than
resolved to a live window (ADR-0130's gone case), the current root or
operator may drop the marked orphan explicitly. Safe-to-drop authorizes
teardown; it asserts nothing about task completion or work state.

## Consequences

A marked agent cannot receive more work, which keeps this substrate rather
than coordination state under ADR-0001 and ADR-0005: no working, waiting, or
progress schema forms around it, only a self-attested terminal fact one
destructive verb consults, the way ADR-0122 already consults live
registrations before acting. The decision is falsified by a drop that tears
down a window whose mark was set while a child it personally hitched was
still live, that honors a mark surviving past the registration that set it,
or that authorizes teardown from a witnessed-only hitcher name without
explicit root or operator action naming that orphan.
