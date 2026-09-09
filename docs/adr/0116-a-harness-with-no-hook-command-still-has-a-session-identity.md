---
id: 0116
status: accepted
date: 2026-08-27
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0116: A harness with no hook command still has a session identity

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Gangline requires an exact native source for a registered session id; the ordinary
source is a hook payload, so a collar whose harness ships no hook command cannot stamp
one that way. `drop` then reports UNSTAMPED after the conversation is already gone, and
every agent on that harness is unresumable — a fact the operator needed at hitch time,
when the harness was chosen.

## Consequences

Where the harness says its id somewhere else, the collar carries it out from there.
OpenCode's plugin bus is such a place: the collar composes a plugin into its launch the
way its siblings compose native hooks, and the plugin turns session events into the one
payload that stamps identity.

What a collar carries out this way claims nothing further. The payload is shaped as an
event the collar gives no other meaning, so identity is recorded and no turn bracket
opens that no event would close; a collar that cannot witness turn boundaries still
declares no Stop hook. A stamp is an identity, not a way back, and the two halves stay
separately earned.

An optional live-session probe is a separate exact source. The cooperative tick uses it
to compare the process currently holding the pane with the registered id; agreement can
establish a missing first stamp, while contradiction records session loss and blocks
delivery. It grants no turn-boundary meaning.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
