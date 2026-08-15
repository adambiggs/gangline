# AGENTS.md

Read [`CONSTITUTION.md`](CONSTITUTION.md) before changing this repository and
[`CONTRIBUTING.md`](CONTRIBUTING.md) before committing. They are binding.

## Check whether `gang` is live

Before editing `bin/gang`, run:

```sh
readlink -f "$(command -v gang)"
```

If it resolves into this checkout, every save changes the executable used by
attached agents immediately. There is no daemon or build step. Keep the script
parseable and make small green checkpoints.

Collars and role briefs are read at hitch time. Existing agents retain the copy
already in their context. Model selection is also a launch choice: drop the old
window and hitch a new one to change it.

`CONTRACT.md` is read at hitch time like the rest, but how it lands depends on
the collar. Where one declares `GANG_ROLE_PROMPT_OPT` it goes into the harness's
system prompt and a running agent keeps the copy it launched with; where none is
declared the agent is pointed at the path, so an edit reaches it the next time it
reads the file. Editing it can therefore change what a live team is held to.

## Preserve guards

An existing assertion records a decision. Changing its expectation requires
showing why the old behavior was wrong; deleting it requires the same proof.
Do not edit a test merely because it blocks a behavior change.

Green is not sufficient when a fixture produced no evidence. Keep unknown
distinct from both pass and fail, and require immediate observable readiness
before asserting state.

## Keep the agent surface small

Gangline is substrate: tmux lifecycle, attributed verified delivery, direct
observation, collars, shipped prose in `CONTRACT.md` and `roles/`, native hooks,
native compaction, and optional yellow/red context and team-time lights. It does
not coordinate work or supervise agents.

- Harness-specific knowledge belongs in `collars/`, not harness-name branches
  in `bin/gang`.
- Do not add the machinery classes `CONSTITUTION.md` bans (laws 1 and 7).
- Do not add speculative surfaces without a live consumer.
- Fail loudly when a native TUI or event shape can no longer be interpreted.
- Put operator security choices in operator configuration. Collars must not
  weaken sandboxes or approvals.
- Do not record changing counts, versions, sizes, or tallies in standing docs.
  Point to the command that measures them.

Durable rationale belongs in terse entries in
[`docs/DECISIONS.md`](docs/DECISIONS.md), not numbered decision essays or history
sections.

## Enable hooks before committing

```sh
git config core.hooksPath .githooks
```

Do not bypass them. Use Conventional Commits as specified in
`CONTRIBUTING.md`.

## Use the shortest proof

Run the ordinary gate at coherent checkpoints:

```sh
test/gate.sh
```

It copies the working tree — uncommitted work included — into a private
snapshot and runs `test/lint.sh`, `test/smoke.sh`, and `test/integration.sh`
from the copy, so
the complete gate is runnable *before* a commit. Run `test/lint.sh` and
`test/integration.sh` directly only against a tree that is already settled;
they refuse a tree they would not own and say so.

Mandatory tests contain no sleeps, polling, or timeout scenarios and must remain
well below their hard ceiling. Use immediate state, event barriers, or fake
clocks.

Where the behaviour under test *is* a timeout, a fake clock may be scaled rather
than stopped — stopping it inverts the assertion instead of removing the wait.
Such a fixture must record its measured margin: quiet-box latency, test budget,
production budget. Recording it is the price of the exception, since without
those numbers the exception is only permission to be timing-dependent.

When native behavior needs proof, drive a separately named disposable Gangline
session. Never use the development agent or the live `gangline` session as the
test subject. Delete only the exact disposable session afterward.

Inside an agent window `$TMUX` is set, so bare `tmux` — and `gang`, which takes
no socket flag — talks to the live server and `TMUX_TMPDIR` is ignored without
saying so. Start with
`unset TMUX TMUX_PANE`, then prove it with `tmux list-sessions` showing only
your own session, before anything spawns an agent. `test/integration.sh` does
this on its seventh line.

Do not invent extra test matrices, reviewer chains, or release blockers without
an operator request.

## Put information in one place

| Document | Holds |
|---|---|
| `README.md` | what Gangline is and why it exists |
| `CONSTITUTION.md` | binding project laws |
| `CONTRACT.md` | the standing terms every hitched agent is held to |
| `docs/DECISIONS.md` | terse durable decisions and rationale |
| `docs/reference.md` | exact commands, environment, and collar contract |
| `docs/operations.md` | unattended operation and recovery |
| `CONTRIBUTING.md` | repository gates, commits, releases, and measurement |
| `CHANGELOG.md` | release history owned by Release Please; never hand-edit |
| `docs/benchmarks.md` | external benchmark selection guidance and validity gates |
| `docs/*-plan.md`, `docs/*-spec.md` | dated implementation records; bodies stay historical and status headers stay current |
| `docs/e2e-lane-calibration.md` | what the offline e2e lane's assertions were proven able to fail on, and when |

`AGENTS.md` is canonical for every harness. `CLAUDE.md` imports it.
Harness-specific repository settings earn a file only when that harness is
actually used here.

## Respect tmux scope

`gang down` ends the entire configured team and `gang drop` ends one agent,
including work you may not have started. Run `gang roster` before either when a
team may be active.

Never run an unaimed `tmux kill-server` or `tmux kill-session`. Experiments use
an explicit private socket and exact disposable session names.
