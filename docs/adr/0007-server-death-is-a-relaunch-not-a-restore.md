# ADR-0007: Recovery from tmux server death is a relaunch, not a restore

- **Status:** Accepted
- **Date:** 2026-07-31

## Context

All Gangline state is tmux window options and dies with the window. That is
deliberate and correct: law 6 asks every artifact for a deletion path, and window
options have the best one there is — they are gone when the window is.

The harness *conversations* died with it too, and nothing recovered them. Every
shipped profile launched bare: no `--continue`, no `--resume`, on any of the four.
A tmux server crash, an OOM kill, or an accidental `kill-server` destroyed the
whole team's memory irrecoverably.

This is not a hypothetical failure by the repo's own account. `roles/_common.md`
spends twenty-seven lines warning agents about the accidental-`kill-server` path
and says "This is not hypothetical." So the failure was considered likely enough
to warn every agent about, while having no answer and no ADR admitting it.

Partial death was already handled well: `roster_row` and `patrol_one` are
subshelled, so one sick agent costs one row. Total death was not handled at all.

## Decision

### Gangline does not persist the roster

A restore needs state that outlives the session — which team existed, which agents
were in it, which directory and role each had. That is exactly the state law 6
exists to prevent. A roster file goes stale, resurrects teams the operator
abandoned on purpose, and needs a deletion path of its own to answer for it.

So the operator supplies the memory and Gangline supplies the flag. Recovery is an
ordinary hitch that asks the harness to pick up its own last conversation:

```
gang hitch reviewer -p claude-code -d ~/work/api --resume
```

Nothing is written down, so nothing has to be cleaned up, and no command can
resurrect a team by accident.

### Profiles declare a full launch line, not a flag

`GANG_RESUME_LAUNCH` is the complete command, not an option appended to
`GANG_LAUNCH`. Codex's resume is a *subcommand* — `codex resume --last` — which an
appended flag cannot express. A declaration that can only spell three of the four
shapes invites the fourth to be written wrong.

### A profile that declares nothing refuses

`--resume` against a profile with no `GANG_RESUME_LAUNCH` fails loud and names the
profile. It does not fall back to a bare launch. That fallback is the silent
downgrade law 8 forbids and issue #47 names in another costume: the operator asked
for the thread back, got a blank agent, and nothing on any surface said so.

### Two of the four shipped harnesses do not declare it

The declaration exists because the answer differs per harness, and the difference
is a safety property rather than a spelling:

- **claude-code** — `claude --continue`. Its own help says "Continue the most
  recent conversation in the current directory". Directory-scoped.
- **codex** — `codex resume --last`. Directory-filtered by default; `--all` is
  documented as "disables cwd filtering", which is what establishes that the
  default filters.
- **opencode** — *not declared.* `opencode session list` returns a byte-identical
  list from two different directories, so sessions are global to the machine and
  `--continue` resumes the most recent one wherever the agent is standing. Two
  opencode agents resuming would land in the same conversation, and neither would
  be the one asked for.
- **pi** — *not declared.* `--continue` is documented as "Continue previous
  session" with no scoping stated, and none was established here. An undeclared
  harness refuses, which is the safe direction; a declaration nobody verified is
  the unsafe one.

### Directory scoping is the safety property, and Gangline cannot check it

Even on a declaring harness, two agents hitched in the *same* directory resume the
same conversation. Gangline cannot refuse that: after a server death there is no
team left for it to compare against, and each `hitch` arrives alone. So the
constraint is documented rather than enforced — **`--resume` is only meaningful
for one agent per directory** — and law 7 is the reason that is the right split.

## Alternatives considered

**A persisted roster and a `gang revive` verb.** Rejected: durable state that
outlives every window, goes stale silently, and needs a deletion path answering
for a team that may have been ended on purpose.

**`GANG_RESUME_OPT`, a flag appended to the existing launch line.** Rejected for
codex's subcommand shape, as above.

**Falling back to a bare launch when nothing is declared.** Rejected: it converts
"this harness cannot do that" into "here is a fresh agent wearing the name of the
one you lost."

## Consequences

A server death still loses Gangline's own state — names, roles, context marks,
staged notes. Those are rebuilt by re-hitching, which the operator was going to do
anyway. What is recovered is the expensive part nobody could rebuild: the harness
conversation.

Two of four shipped harnesses cannot offer it, and that is now visible at the
point of asking rather than discovered afterwards.

`roles/_common.md` says what a `kill-server` costs, beside the warning that
already said what causes it.
