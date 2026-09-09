---
id: 0117
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0117: A prompt the harness draws into an empty box is not a human's line

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

A composer carrying the harness's own suggestion is empty. Returning that suggestion as
composer contents makes Gangline see a half-written line, refuse to paste into it, and
leave a freshly hitched agent registered, running, and unreachable.

## Consequences

Recognising one is pinned copy paired with the position it must hold, not a colour: a
theme chooses colours, the harness chooses words. Pinned copy fails safe — a reworded
prompt stops matching and the collar reports a draft again, which refuses delivery.
Nothing here may turn a line somebody is typing into an empty box.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
