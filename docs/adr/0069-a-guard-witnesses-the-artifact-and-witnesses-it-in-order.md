---
id: 0069
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0069: A guard witnesses the artifact, and witnesses it in order

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Assert the thing a defect actually produces — the text left on the pane, the body
recorded in the window option — and not a status that merely travels with it. A refusal
and a delivery that failed after typing both exit non-zero, so an exit-status assertion
is green on the very defect it was written to catch, and a passing count then reports
ground nobody covered. The same trap catches the fixture that cannot produce the
artifact at all: a pane with no busy marker cannot paint a turn, so a probe built on one
proves nothing about typing into live work, however carefully it is run.

## Consequences

Witnessing the right artifact is half of it. A guard is proven by reverting its fix and
watching the evidence come back on screen — but red once is not proof; red in a defined
order is. A keystroke sent is not a keystroke observed: the send returns when the key is
enqueued, so a capture taken afterwards reads whichever moment it happens to catch, and
a run where the harness had not yet acted goes green for no reason anyone chose. Order
the observation behind the same input path the work travels, so that when the probe
reports, what is being tested has either happened or never will.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
