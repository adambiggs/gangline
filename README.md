# Gangline

A harness harness. Gangline unifies any CLI coding agent it can get its hands on —
Claude Code, Pi, whatever ships next — into a tmux-powered team, using each for its
strengths, with minimal harness-specific integration points.

The name: a gangline is the single rope that hitches many dogs, each in its own
harness, to one sled and one musher. The integration point is the attachment clip —
never surgery inside the dog.

Read `CONSTITUTION.md` before touching anything. Law 1: tmux is not a hack, it IS
the substrate.

## Install

```sh
ln -sf "$(pwd)/bin/gl" ~/.local/bin/gl
```

Requires tmux ≥ 3.2 (bracketed paste via `paste-buffer -p`).

## Quickstart

```sh
gl spawn ada -p claude-code -d ~/my/repo   # ada is now a tmux window running claude
gl send ada --from operator "read the failing test in ci and fix it"
gl status ada                              # busy | idle
gl capture ada                             # what's on ada's screen
gl roster                                  # everyone, with state
gl attach                                  # watch the whole team live
gl kill ada
```

Every message an agent receives is prefixed `[gl:<sender>]` — agents always know
who is speaking. Sends into a busy agent are refused (or `--wait`). Sends are
verified in the pane before submit; an unverified send is a loud failure.

## Layout

```
bin/gl        the whole tool
profiles/     one small file per harness: launch cmd, busy marker
docs/adr/     decisions
```

## Components

1. **Communication substrate** — `bin/gl` (this is it, in full).
2. **Self-compaction primitives** — durable-state files, arc-boundary handoffs,
   compact-trigger via each harness's own command. Lands when it has a consumer.
