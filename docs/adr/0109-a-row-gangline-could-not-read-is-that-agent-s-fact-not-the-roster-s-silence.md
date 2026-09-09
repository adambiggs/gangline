---
id: 0109
status: accepted
date: 2026-08-18
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0109: A row Gangline could not read is that agent's fact, not the roster's silence

## Context

One unreadable agent must stay visible without making the rest of the team disappear.

## Decision

Print an unreadable agent as `?unknown?` with `state-unreadable`, continue reading every
other row, and return nonzero because the complete roster could not be read.

## Consequences

`gang roster` is the check run before `gang down` or `gang drop`. It ran each row in a
subshell under `set -e`, so the first agent whose pane refused a read ended the whole
listing: the rows before it printed, the rows after it did not, and from outside that is
indistinguishable from a smaller team. Loud, but the loudness was about the wrong scope.

The refusal belongs to one agent. Its row carries `?unknown?` and the marker
`state-unreadable`, the refusal naming the reading it could not take reaches stderr
immediately above that row, every other agent is still read and printed, and the command
exits nonzero so nothing spends the listing as an all-clear. The predicates are
unchanged: occupancy and busy still refuse out loud rather than express an unknown they
cannot express, because callers spend their answers as permission. What changed is where
that refusal stops.

The porcelain word is `unknown` for both a state Gangline determined it could not settle
and one it could not read at all. The human row separates them and the machine row does
not; the two are the same answer to the question porcelain is asked, which is whether
this agent's state is known.

The exit status does not carry that answer, and saying it did was an overclaim. It
reports whether every row's reading could be TAKEN. A reading that WAS taken and did not
settle prints `?unknown?` with its own witness and exits 0, which is what `gang status`
has always done for the same agent, and making roster alone disagree would put the same
fact in two channels that contradict each other. The rows are where an unknown is
reported; the status says only whether gang could look.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
