---
id: 0122
status: accepted
date: 2026-08-31
supersedes: []
superseded-by: []
tags: [tmux, lifecycle]
---

# ADR-0122: The teardown guard trusts live window registrations, not a caller record

## Context

Caller environment can be redirected, while destructive tmux authority must follow the
server actually reached.

## Decision

Resolve the tmux server the invocation will actually reach and authorize teardown only
from its live `@gl_agent` registrations; caller records may corroborate but never
authorize it.

## Consequences

Inside a pane `$TMUX` outranks `TMUX_TMPDIR`, so the command that ended a live team read
as aimed at a sandbox. A guard that matched shapes would have passed it. The shim gang
puts on an agent's PATH asks tmux which socket the invocation would actually reach, then
asks that server for live `@gl_agent` registrations. Reproducing tmux's path rules in
the guard diverged when tmux 3.2a silently ignored a `TMUX_TMPDIR` whose directory was
absent and retargeted its default socket. Such a root now refuses every unaimed tmux
command, not only teardown, because ordinary fixture traffic was redirected by the same
fallback. A caller testing Gangline legitimately replaces `GANG_SESSION` and
`GANG_LOCK_DIR`, so the team record can corroborate a name but cannot authorize a
teardown. An answering server whose registrations cannot be read refuses regardless of
whether the caller is in a pane: unreadable state is not evidence that teardown is safe.
An unreachable explicit private socket reaches real tmux for its normal error.

It is a guardrail rather than a boundary, and says so: one variable runs the command
anyway. Every teardown verdict, including a fall-open, is written under both the caller
root and the launch-time team root, because the latter remains forensic evidence when
the caller redirects the former.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
