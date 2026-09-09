---
id: 0028
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0028: Parked bodies are recovered through their collar and verified

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Gangline already owns every piece of evidence the manual recovery uses, so it performs
the recovery instead of printing the keystrokes. The collar declares the key that loads
the parked body, and the loaded composer is read back against THE BODY GANGLINE COMPOSED
before any Enter — never against a second reading of the composer.

## Consequences

Two readings taken at different moments are two renderings of one body, and a harness is
free to render them differently: claude-code shows a pasted multi-line body as `[Pasted
text #N +M lines]`, which carries none of its text, and expands it on recall. Comparing
those two could not succeed for any body of more than one line, and the refusal it
produced left that body sitting unsent in the composer.

The comparison is whole — containment would accept a truncated, altered or appended
remainder — and it is over what a pane capture can carry, which is the body's text with
every run of blank space collapsed. A capture pads every row to the pane width, gives
continuation rows the composer's gutter, and re-flows a body line too long for the box
across rows where the break is indistinguishable from one the body wrote. Byte equality
against a body is therefore not strictness, it is unsatisfiable.

The cost is stated rather than hidden: two bodies differing only in blank space — a
trailing space, a line break where the other has a space — canonicalise alike. No
comparison against a capture could have separated them; the padding erases the first and
the re-flow the second. Every difference in the body's text still refuses.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
