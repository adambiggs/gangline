---
id: 0014
status: accepted
date: 2026-08-31
---

# ADR-0014: Teardown authority follows the tmux server actually reached

## Context

Inside a pane `$TMUX` outranks `TMUX_TMPDIR`, and tmux 3.2a silently ignores a
`TMUX_TMPDIR` whose directory is absent and falls back to the default socket. A
kill aimed at a sandbox reached the live server and ended a 13-agent team
(#187). A team on a private socket also looked ended from any shell that had
lost that environment.

## Decision

The tmux guard asks tmux which server an invocation would reach and authorizes
teardown only from that server's live `@gl_agent` registrations; caller records
may corroborate but never authorize. An absent `TMUX_TMPDIR` refuses every
unaimed tmux command. `hitch` records the team's socket so `gang teams` and
`gang attach` can find it, and they ask the server rather than believe the
record.

## Consequences

Unreadable registrations refuse teardown. The guard is a guardrail, not a
boundary: one variable still runs the command anyway.
