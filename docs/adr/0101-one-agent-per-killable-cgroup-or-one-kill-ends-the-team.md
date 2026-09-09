---
id: 0101
status: accepted
date: 2026-08-17
supersedes: []
superseded-by: []
tags: [lifecycle]
---

# ADR-0101: One agent per killable cgroup, or one kill ends the team

## Context

Agent lifecycle actions span tmux and host processes, so partial success can leave live
or durable state.

## Decision

A tmux server inherits the cgroup of whatever started it, so every agent on a team is a
process in the login session's scope. `systemd-oomd` selects the descendant *leaf*
cgroup holding the most swap, and a long-lived session full of dormant agents is by
construction the largest holder of swapped-out anon memory: idleness is the
qualification for being chosen, not a defence. One kill ends every agent at once.

## Consequences

`GANG_SCOPE=on` wraps each hitched launch in a transient systemd user scope, so each
agent is its own leaf, holds its own swap, and is named in the kill message. This is a
platform-specific launch prefix rather than a branch on any harness: the collar still
declares the whole launch line, and the scope is composed around it. It is off unless
the operator declares it, and where it cannot be honoured the hitch is refused rather
than quietly run unscoped.

The trade is deliberate and is not free. A scope lives under the systemd user manager,
so scoped agents also fall under its memory-pressure policy — a separate policy with its
own trigger, which can take one agent for pressure that no agent caused, and can act
again after its own delay. And a swap kill selects only a candidate holding more than 5%
of total swap, so a team split finely enough can leave every agent under that bar, where
the swap policy selects nobody rather than the team. Losing one named agent is still a
better outcome than losing every agent at once, but the thresholds are the operator's
and this changes only the victim pool.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
