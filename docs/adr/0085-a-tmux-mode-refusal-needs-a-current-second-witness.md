---
id: 0085
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [tmux]
---

# ADR-0085: A tmux mode refusal needs a current second witness

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

Window-name glyphs report Gangline state and are not tmux mode evidence.

## Consequences

Delivery reads only `#{pane_in_mode}`; when the first read says a mode owns the pane, it
confirms that answer once at the refusal edge. Two positive reads preserve copy-mode
untouched, while a changed zero means tmux routes keys to the pty now and avoids parking
an idle recipient behind a mode that already ended.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
