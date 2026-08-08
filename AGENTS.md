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

Collars are read at hitch time. Existing agents retain the copy already in their
context. Model selection is also a launch choice: drop the old window and hitch
a new one to change it.

## Preserve guards

An existing assertion records a decision. Changing its expectation requires
showing why the old behavior was wrong; deleting it makes the strongest version
of that claim. Do not edit a test merely because it blocks a behavior change.

Green is not sufficient when a fixture produced no evidence. Keep unknown
distinct from both pass and fail, and require immediate observable readiness
before asserting state.

## Keep the agent surface small

Gangline is substrate: tmux lifecycle, attributed verified delivery, direct
observation, collars, native hooks, native compaction, and optional yellow/red
context and team-time lights. It does not coordinate work or supervise agents.

- Harness-specific knowledge belongs in `collars/`, not harness-name branches
  in `bin/gang`.
- Do not add patrols, schedulers, daemons, watchdogs, retry managers, or
  reconciliation loops.
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

Run the ordinary gates at coherent checkpoints:

```sh
test/lint.sh
test/integration.sh
```

Mandatory tests contain no sleeps, polling, or timeout scenarios and must remain
well below their hard ceiling. Use immediate state, event barriers, or fake
clocks.

When native behavior needs proof, drive a separately named disposable Gangline
session. Never use the development agent or the live `gangline` session as the
test subject. Delete only the exact disposable session afterward.

Do not invent extra test matrices, reviewer chains, or release blockers without
an operator request.

## Put information in one place

| Document | Holds |
|---|---|
| `README.md` | what Gangline is and why it exists |
| `CONSTITUTION.md` | binding project laws |
| `docs/DECISIONS.md` | terse durable decisions and rationale |
| `docs/reference.md` | exact commands, environment, and collar contract |
| `docs/operations.md` | unattended operation and recovery |
| `CONTRIBUTING.md` | repository gates, commits, releases, and measurement |
| `CHANGELOG.md` | release history owned by Release Please; never hand-edit |

`AGENTS.md` is canonical for every harness. `CLAUDE.md` imports it.
Harness-specific repository settings earn a file only when that harness is
actually used here.

## Respect tmux scope

`gang down` ends the entire configured team and `gang drop` ends one agent,
including work you may not have started. Run `gang roster` before either when a
team may be active.

Never run an unaimed `tmux kill-server` or `tmux kill-session`. Experiments use
an explicit private socket and exact disposable session names.
