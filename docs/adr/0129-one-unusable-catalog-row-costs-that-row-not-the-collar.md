---
id: 0129
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0129: One unusable catalog row costs that row, not the collar

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Drop and diagnose an unusable model-catalog row while retaining every readable row; fail
the catalog only when no readable row remains or ids conflict.

## Consequences

A collar's model enumerator is a producer Gangline does not control. An id spelled with
characters this vocabulary has no use for — OpenRouter's `~` routing prefix, reaching
gang through opencode — used to refuse the whole catalog, and with it every hitch on
that collar on the host, including hitches for models on providers whose rows read fine.

A row Gangline cannot read is dropped and named on stderr instead. The model a caller
asked for is the only row that has to be usable. What stays fatal is what would leave
nothing readable behind: a producer whose every row is unusable, and a repeated id,
where the ambiguity is over which row wins rather than over whether a row can be used at
all.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
