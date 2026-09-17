---
id: 0202
status: proposed
date: 2026-09-16
supersedes: []
superseded-by: []
tags: [messaging, operations]
---

# ADR-0202: Runtime state lives in the uid's runtime directory and moves only when a session opens

## Context

The host empties `/tmp` at boot, where Gangline keeps locks and spools. Every
process delivering to a pane must agree on one root, but `XDG_RUNTIME_DIR` is
present in panes and absent from `env -i` shells, and a running team cannot
change roots under its own spools.

## Decision

`libexec/gang-state-root`, sourced by `gang` and the tmux guard, resolves the
default root: `/tmp/gangline-UID` while that is a directory without a
`retired` file, else
`/run/user/UID/gangline` when `/run/user/UID` exists, else `/tmp/gangline-UID`.
It reads the uid, never `XDG_RUNTIME_DIR`. `GANG_LOCK_DIR` still overrides it.
A hitch that opens a session, with no override, moves off an existing `/tmp`
root only when no team recorded there answers and no window on the team server
claims one of its spools. It creates `retired` exclusively inside that root,
leaving the tree in place, then reads the spools windows claim: it moves
empty claimed reservations to the new root and archives the spools no window
claims. `lock_base` checks for the marker after establishing each path under
the old root, and a spool mint checks it after publishing its claim; either
refuses if it is there, and the mint withdraws its claim first. It creates the default root only by name,
never as a parent, and never recreates the `/tmp` root while a runtime
directory exists.

## Consequences

A team keeps its root through a checkout update; `gang down` then `gang up`
moves it. A `gang run` unit launched with the old root fails in `lock_base`
after the move. Timers carry no root unless one was set, so they resolve it
when they fire. A process that resolved the old root before the move refuses
and must be rerun; one already past its last `lock_base` call finishes that
operation on the old root. The old tree lasts until boot. Losing `/run/user/UID` at logout moves every process to
`/tmp` at once and loses what the runtime root held; `gang` warns when it
creates that fallback. Sandboxed Codex needs write access to the new root. Falsifier: two
`gang` processes for one uid, neither setting `GANG_LOCK_DIR`, resolve
different roots at the same moment.
