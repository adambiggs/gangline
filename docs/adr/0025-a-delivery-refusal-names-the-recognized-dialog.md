---
id: 0025
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0025: A delivery refusal names the recognized dialog

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Detection is also a report.

## Consequences

A send whose verification fails asks the collar what is on the screen and quotes the
dialog's visible title in the refusal, because "the box read back unchanged" is accurate
and useless: that a dialog was there, and that answering it is the repair, was otherwise
discoverable only by experiment. Naming decides nothing — a collar with no reader, an
unreadable pane and an unrecognised frame each cost a sentence, never a delivery.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
