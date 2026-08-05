# Operating a team

Gangline keeps control in native harnesses and tmux. There is no background
coordinator to recover work, approve a dialog, or decide what an agent should do.
An unattended session succeeds when the harnesses have the access they need, the
operator can observe them, and agents compact at natural checkpoints.

## Before leaving a team unattended

Check the direct surfaces:

```sh
gang roster
gang status lead
gang capture lead
```

Confirm that each harness has already completed first-run setup, authentication,
and repository trust prompts. Gangline never answers permission dialogs. An
occupied dialog appears as `occupied (authority unknown)` and blocks input until
the operator resolves it in `gang attach`.

Native sandboxes must be able to reach the tmux server and the `gang` executable.
For Codex, this commonly means the tmux socket and Gangline checkout need to be
inside paths its sandbox permits. Fix that in the operator's Codex configuration;
shipped profiles never disable sandboxing or bypass approvals.

Use a stable `GANG_SESSION`, `GANG_PROFILES`, and `GANG_LOCK_DIR` for every shell
that addresses the team.

## Sending messages safely

Pass message prose on standard input so shell syntax remains data:

```sh
gang send --to worker --stdin <<'TASK'
Run the parser test whose fixture contains $(literal shell text).
TASK
```

A quoted heredoc delimiter prevents the caller's shell from expanding the body.
Gangline then verifies the target composer changed before submitting it.

Delivery refuses when:

- a native dialog or picker owns input;
- a person has a draft in the composer;
- the target cannot safely accept mid-turn input;
- the state evidence is indeterminate; or
- another delivery owns that pane's lock.

A failure after a paste may leave staged text in the composer. `gang status` and
`gang roster` report it. Inspect with `gang capture` or `gang attach`; the next
safe delivery clears only a staged rendering that still exactly matches what
Gangline recorded.

## Compaction

The startup contract tells every agent to use native compaction at natural
checkpoints:

```sh
gang compact NAME
```

Do this after finishing a coherent arc and recording durable state in the
repository, not in the middle of a half-applied edit. Native harness compaction
owns the summary and context transition.

Codex self-compaction is deferred to its Stop event because its active turn
cannot submit `/compact` into its own composer. The request is one-shot. If the
native command cannot be submitted, `status` and `roster` retain the failure
instead of claiming success.

## Optional context lights

Context lights are disabled unless the operator supplies absolute thresholds at
hitch time:

```sh
GANG_CONTEXT_LIGHTS=120000,200000 gang hitch worker -p codex
```

Yellow asks the agent to compact at its next natural checkpoint. Red asks it to
finish the current arc and compact now. A light is emitted once per context
epoch; usage falling below yellow resets the epoch. These are advisory native
hook messages, not patrols or automatic actions.

If an enabled light reports unavailable, the profile's context source failed.
Inspect the native harness directly. Disabled lights perform no context read and
add no prompt or roster noise.

## Understanding observation

Gangline selects evidence for each predicate rather than averaging it. Fresh
native events outrank owned file state, which outranks pane scraping. Missing,
expired, or contradictory evidence produces an explicit indeterminate state.

`busy` means Gangline has positive evidence of work. `idle` means it has positive
evidence the composer is ready. `occupied` means a native UI owns the composer.
`parked` means an agent is itself blocked in `gang wait`. `expired` means the
available evidence can no longer answer truthfully.

Profiles contain the only harness-specific regexes and parsers. A native TUI
update can invalidate them. The intended repair loop is direct: reproduce the
wrong observation in a disposable team, inspect the actual pane, update the
profile, and run the fast repository gates. Gangline does not ship a diagnostic
agent or automated strategy-rot machinery.

## Disposable real-harness smoke tests

Never test Gangline against the development agent or the live `gangline` team.
Use an explicitly named, disposable session:

```sh
GANG_SESSION=gangline-smoke-codex gang hitch probe -p codex -d "$PWD"
GANG_SESSION=gangline-smoke-codex gang capture probe
GANG_SESSION=gangline-smoke-codex gang down
```

Drive only the native behavior under test, inspect the pane and tmux options, and
delete the exact smoke session afterward. Mandatory tests use a private tmux
server and stand-in profile; they do not spend real harness turns.

## Recovery

### A harness is blocked on a dialog

Run `gang attach`, answer it in the native TUI, then re-run `gang status`. Do not
send prose into a dialog and do not teach Gangline to answer it.

### The tmux server was lost

Gangline persists no roster. Recreate only the agents the operator chooses:

```sh
gang hitch lead -p claude-code -d "$PWD" --resume
gang hitch worker -p codex -d "$PWD" --resume
```

`--resume` uses the profile's native, directory-scoped continuation command and
refuses profiles that cannot make that request safely. This is relaunch, not a
claim that Gangline reconstructed the old team.

### A profile no longer recognizes its TUI

Capture the real pane, update only that harness's profile, and require loud
failure for shapes the parser cannot identify. A degraded fallback that reports
healthy is worse than an explicit break.

## tmux scope

`gang down` destroys every window in `GANG_SESSION`; `gang drop` destroys one
named window. Always run `gang roster` first when other people or agents may be
using the same session.

For experimental tmux work, use an explicit private socket (`tmux -L NAME ...` or
`tmux -S PATH ...`). Never run an unaimed `tmux kill-server` or `kill-session` on
a shared server.
