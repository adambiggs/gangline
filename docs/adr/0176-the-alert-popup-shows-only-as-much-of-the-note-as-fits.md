---
id: 0176
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [alerts]
---

# ADR-0176: The alert popup shows only as much of the note as fits

## Context

Operator alerts must remain visible and attributable without becoming a second
transport.

## Decision

`gang alerts --open` on a terminal, which is how the Prefix+A popup runs it, measures
that terminal and prints at most the rows of the failure note that leave the headline,
the transition line, any binding-conflict line, and the close prompt on screen, each
counted at the rows the terminal wraps it to. A note that would overflow ends with a row
that counts the hidden lines and names `gang alerts` as the way to read all of them; a
terminal too narrow for even that pointer gets no note rows. Rows break between
characters, never inside one, and a character outside ASCII is counted at two columns so
that no row can spill over on a terminal at least two columns wide; a character counted
wider than the whole terminal takes a row of its own and overflows it, which is as
bounded as a terminal that narrow can be. A terminal that cannot be measured gets a
short pointer alone, because a guessed size could scroll the popup again. A terminal
with too few rows or columns for those fixed lines themselves scrolls as any text does;
the bound is on the note, which is the only part whose length is open.

## Consequences

The popup is a scrolling terminal the size of its interior. A note that wraps past the
rows left for it scrolls the headline and the close prompt off the screen tmux repaints
the popup from, so the operator sees the tail of a note with nothing that says what it
is, which alert it belongs to, or how to close it. `gang alerts` without `--open`, the
porcelain form, and any output that is not a terminal print the whole note as before.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
