---
id: 0127
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0127: A held window with no live pane is dead, not a screen state

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

Tmux reports pane death directly.

## Consequences

A remain-on-exit corpse can retain an old composer or transcript marker, but screen
classifiers cannot turn those bytes back into a running process. State observation
therefore asks whether every pane has exited before reading occupancy, fatal evidence,
or activity. Human status says `!dead!`, and porcelain says `dead`; one live split pane
is enough to keep the window on its ordinary classifier.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
