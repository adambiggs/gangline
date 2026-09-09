---
id: 0131
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0131: A mandatory barrier must stay inside the wait ceiling

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

`tmux wait-for` has no timeout, so a barrier that is never answered parks a run forever,
prints nothing, and — because the gate serialises on one host lock — queues every other
run behind it. Two such wedges held that lock for 25 minutes and for 3h56m. The suite's
answer is a ceiling: a tmux shim at the front of the run's PATH that cuts a blocking
wait off and names it.

## Consequences

Because the ceiling is a PATH shim, a barrier only gets it when the command it runs is
one PATH resolves. `test/lint.sh` therefore refuses a blocking `wait-for` issued through
the spelling that deliberately leaves PATH behind — `REAL_TMUX`, which
`test/integration.sh` resolves to the tmux that is NOT a shim, or a tmux named by an
absolute path. A `-S` signal blocks on nothing and is left alone. The ceiling's reach
into a pane, where every measured wedge happened, is asserted rather than assumed: a
pane's environment comes from the tmux server rather than from the client that opened
the window.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
