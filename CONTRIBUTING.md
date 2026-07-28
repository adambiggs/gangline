# Contributing

Read `CONSTITUTION.md` first. Those laws bind every change here, and violating one
is a defect — not a style disagreement.

## Setup

```sh
git config core.hooksPath .githooks
```

Enables the `commit-msg` gate. It is not installed automatically; a clone without
it commits unchecked.

## Commits

Conventional Commits:

```
<type>[(scope)][!]: <description>
```

- **type** — `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`,
  `revert`, `style`, `test`
- **scope** — optional, lowercase; a command or component (`patrol`, `vet`,
  `send`, `compact`, `up`, `down`, `context`, `profiles`)
- **!** — breaking change; pair it with a `BREAKING CHANGE:` footer saying what
  callers must update

```
fix(patrol): gate injection on pane stability
feat(down): tear the whole team down
refactor!: rename executable and protocol from gl to gang
```

The `commit-msg` hook rejects anything else, and the `commits` workflow re-runs
that same hook over every pushed commit — one regex, so the two cannot drift. A
ruleset makes the `conventional` check required on the default branch (adding
`shell` to it is a ruleset change); repository admins bypass it, so for the
maintainer the hook is still the gate that matters.
Subjects git generates itself — merge commits, `fixup!`, `squash!`, `amend!` —
pass through, since rejecting them breaks merge and interactive rebase.
`git revert` is not exempt: `revert` is a type, so edit its generated subject like
any other.

Write the body for someone deciding whether to trust the change: what failed, what
the fix is, and what proved it. Scraped-marker changes say which harness version
they were verified against — that pin is what `gang vet` reads.

## Before adding code

- **Law 5** — nothing lands without a live consumer. If nothing invokes it the day
  it merges, it does not merge.
- **Law 4** — per-harness knowledge is a profile, not a code path. A harness-specific
  branch in `bin/gang` needs an ADR in `docs/adr/`.
- **Law 9** — when in doubt, the answer is prose in an agent's prompt, not code here.

Every shell and Python file carries an `SPDX-License-Identifier` line; new ones
need it too.

Shell changes must pass `bash -n`, `shellcheck -S warning`, and
`test/integration.sh`; the `shell` workflow runs all three on every push. The test
drives `bin/gang` against a real tmux server with the `bash` profile — no mocks,
because a mocked tmux would agree with whatever the code already does. A fix to
delivery or addressing belongs there as a case that fails without the fix.

## License

Contributions land under Apache-2.0, the same license the repo ships under:
section 5 puts anything you intentionally submit for inclusion under those terms
unless you say otherwise in the pull request. There is no CLA to sign.
