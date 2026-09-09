---
id: 0141
status: accepted
date: 2026-08-31
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0141: An advisory dialog is reported, never dismissed

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

A native surface can own the input box while the agent behind it keeps working and while
the surface itself says no action is required.

## Consequences

Gangline still refuses to type through it, but reporting it as occupancy of unknown
authority sends a lead looking for a human decision that does not exist. A collar may
declare `collar_advisory` and name the surface; the state stays `!occupied!` and only
the words change. Gangline does not dismiss it: the keystroke would be a bare digit at
something that clears itself, and a menu that closes between the reading and the
keystroke takes that digit as message text.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
