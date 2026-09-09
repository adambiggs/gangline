---
id: 0023
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0023: Harness overlays are recognized by structural chrome

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

A dialog painted over a live composer owns the keyboard while the box under it still
reads as usable, so occupancy has to be settled by the collar's own reader rather than
by an occupancy regex a drawn composer dismisses.

## Consequences

Recognition is keyed to the frame, not to the words in it. This was pinned to one
dialog's exact title and exact guide row and rotted exactly as a copy pin does:
claude-code 2.1.241 dropped the guide the collar pinned, the pin stopped matching, and
the dialog owned input above a live composer again with nothing saying so. What does not
move between dialogs or between builds is the chrome — a band drawn from the left edge,
a title touching it, and a row of key hints closing the region it opened — and the
positional questions the pin was already paired with still answer the forgery case: a
body carrying the same rows lives after the composer's opening rule, so a message cannot
hide the box it sits in. The claim is held to captures of unrelated dialogs from more
than one build, because a rule fitted to a single frame is a pin with extra steps.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
