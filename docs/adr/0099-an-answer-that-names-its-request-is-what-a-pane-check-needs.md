---
id: 0099
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0099: An answer that names its request is what a pane check needs

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

Correlate each pane answer to the exact request under test; arrival of an unrelated
completion proves nothing about that request.

## Consequences

Every completion this stub writes looks alike, and booting already put one on the pane,
so a check for the answer's prefix passes whether or not the turn under test was ever
drawn. Each answer carries the sequence number of the request it answers and the stub
tells the lane which request it froze, so a scenario can name one turn's reply instead
of accepting any.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
