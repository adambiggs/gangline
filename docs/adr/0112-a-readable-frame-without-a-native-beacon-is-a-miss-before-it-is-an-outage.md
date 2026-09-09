---
id: 0112
status: accepted
date: 2026-08-22
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0112: A readable frame without a native beacon is a miss before it is an outage

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Claude can briefly redraw, cover or scroll its context beacon off-screen while its pane
and the beacon source remain healthy. The collar returns status 2 for that
readable-frame miss. Context lights report the first miss in a source failure epoch
without changing the last real light; alternating good and missed frames do not repeat
it. A consecutive miss latches unavailable, while an unreadable or malformed source
still fails on its first observation.

## Consequences

The threshold is carried by a window option and advances only on native context checks.
It adds no pane capture, retry loop, clock, parser or roster read.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
