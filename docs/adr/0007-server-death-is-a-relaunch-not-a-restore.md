# ADR-0007: Recovery from tmux server death is a relaunch, not a restore

- **Status:** Accepted
- **Date:** 2026-07-31

## Context

All Gangline state is tmux window options and dies with the window. That is
deliberate and correct: law 6 asks every artifact for a deletion path, and window
options have the best one there is — they are gone when the window is.

The harness *conversations* outlast it. Each harness writes its own to disk and a
dead server does not touch them — which is the only reason any of this is
possible. What died was Gangline's route back to them. Every shipped profile
launched bare: no `--continue`, no `--resume`, on any of the four. So a crash, an
OOM kill, or an accidental `kill-server` left every agent's thread sitting on
disk with nothing in the tool able to ask for it, and the operator's only path
was to leave Gangline and drive each harness by hand.

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
ordinary hitch that asks the harness for the last conversation in a directory:

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

### The harness selects by directory and recency, never by agent

Neither resume form carries an agent name or a session id, because neither harness
offers a way to ask for one. `--continue` and `resume --last` mean *the most recent
conversation in this directory*, which is the agent's own only when exactly one
agent ever worked there. Two agents hitched in the same directory resume the same
conversation. Gangline cannot refuse that: after a server death there is no team
left for it to compare against, and each `hitch` arrives alone. So the constraint
is documented rather than enforced — **`--resume` is only meaningful for one agent
per directory** — and law 7 is the reason that is the right split.

A second gap sits in the same seam and is still open. Where the directory holds no
conversation at all, both declaring harnesses start a fresh one and exit 0 rather
than failing: `claude -c` in an empty directory answers normally, and codex maps a
`--last` lookup that finds nothing to a fresh session. Gangline sees a live agent
and reports a successful hitch. So the flag's promise is *ask for the thread*, not
*prove you got it* — the blank-agent outcome this ADR refuses from an undeclared
profile can still arrive through a declared one. That is issue #52.

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
anyway. What is recovered is the expensive part, which re-hitching does not
rebuild: the harness conversation.

Two of four shipped harnesses cannot offer it, and that is now visible at the
point of asking rather than discovered afterwards.

`roles/_common.md` says what a `kill-server` costs, beside the warning that
already said what causes it.
