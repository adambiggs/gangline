---
id: 0177
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0177: Agent panes do not inherit the team's tmux address

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

New teams use tmux's named `gangline` socket rather than its discoverable `default`
socket, and `hitch` removes `TMUX` from the harness launch while retaining `TMUX_PANE`
for attribution and native hooks. An unguarded client invoked by an absolute path,
copied process path, subshell, or clean environment therefore has no implicit route to
the team server. Each `gang` invocation validates the session's recorded socket and
carries that address only inside its own process; the PATH guard receives it separately
so an agent's non-destructive `tmux wait-for` barriers continue to address the team.
Other unaimed tmux commands do not consume that internal value. A live legacy team on
the caller's current or default socket remains addressable during rollout.

## Consequences

This is a same-UID isolation limit, not a privilege boundary. The named socket address
is derivable from Gangline's fixed label as well as available in the launch environment.
A process can therefore bypass the PATH shim and aim a real client with `-L gangline` or
`-S "$GANG_TMUX_SOCKET"`; preventing that requires a separate privilege boundary. The
supported guarantee is no implicit route, not that an agent cannot reach an address it
deliberately supplies.

The PATH guard remains the diagnostic layer for explicitly aimed tmux commands. When its
stderr is not a terminal, it repeats its refusal on stdout so `2>/dev/null` cannot hide
it even when an agent harness captures both streams. The named socket disappears with
the tmux server, and `gang down` removes the reachability record as before.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
