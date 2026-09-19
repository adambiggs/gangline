---
id: 0013
status: accepted
date: 2026-08-30
---

# ADR-0013: A mandatory barrier stays inside the wait ceiling

## Context

`tmux wait-for` has no timeout. A barrier nobody answers parks a run forever,
prints nothing, and holds the gate's host lock for everyone else: two such
wedges held it for 25 minutes and for 3h56m.

## Decision

The suite puts a tmux shim at the front of its PATH that cuts off a blocking
wait at a ceiling and names it. `test/lint.sh` refuses a blocking `wait-for`
issued through `REAL_TMUX` or an absolute tmux path, which would bypass the
shim.

## Consequences

A wedged barrier is a named failure, not a silent hold. A `-S` signal blocks on
nothing and is left alone.
