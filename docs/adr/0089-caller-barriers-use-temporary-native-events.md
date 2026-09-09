---
id: 0089
status: accepted
date: 2026-09-04
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0089: Caller barriers use temporary native events

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

`gang wait` is an opt-in operator or external-script barrier, never a patrol or
supervisor.

## Consequences

A caller pane inside the target team is resolved by the same session membership rule
that gives `gang send` its observed identity and is refused: delivery is push-based, so
an agent ends its turn and receives the report at that boundary instead of blocking it.
Each allowed caller owns a unique temporary tmux `wait-for` channel and sparse hook key
pinned to the target window and active pane; ownership is re-read before cleanup because
tmux reuses the lowest free ordinary hook-array slot. Native Stop closes the turn before
signalling it, while natural pane exit and Gangline teardown release it as a loud
vanished-target failure. Direct tmux kill commands bypass that release and are
documented as unsupported teardown for a waited target. A foreground deadline,
defaulting to the existing native turn-fact bound, is a Python one-shot alarm around one
fresh Bash/tmux process group. Python is already a required dependency; this avoids GNU
`timeout`, while deadline and foreground signals kill and reap that exact group before
the caller cleans its hook. No daemon, option, or file records the wait. A successful
waiter consumes its signal once; cleanup does not signal and wait again, because that
second wait can race the returning client and deadlock. Tmux offers no deletion for a
latch stranded by `SIGKILL` or a boundary race, so that nonce-named memory can last
until server exit. `?unknown?` and a Stop declaration with no native turn evidence are
refused rather than waited through, and `done` deliberately promises only the next Stop
— it does not claim to identify or own a logical turn. Tmux channel locks are excluded
because a dead holder can leak one indefinitely.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
