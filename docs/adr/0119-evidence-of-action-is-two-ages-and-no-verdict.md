---
id: 0119
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0119: Evidence of action is two ages and no verdict

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Report pane activity age and last tool-call age separately, attach no health verdict,
and name absent, bounded, future, or unreadable evidence explicitly.

## Consequences

A verified delivery proves text reached a pane; a turn bracket proves a turn opened and
closed. Neither proves the recipient did anything, and an agent that answers in words
and runs nothing satisfies both. So `roster` and `status` carry the age of the last tool
call the harness recorded, and the age of the last write to the pane where that is the
only evidence left.

They are not collapsed into a health state. A long build and a wedge both go quiet, so
any threshold gang picked would be gang's judgement wearing the harness's authority. The
four tool-call answers are kept apart instead — an age, a bound when a scan limit was
reached first, `none` for a source read whole with no tool call in it, and unknown with
a reason — because `none` is a claim about the session and unknown is a claim about the
instrument.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
