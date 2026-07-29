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
that same hook over every pushed commit — one regex, so the two cannot drift. The
workflow also checks the pull request title. **Your PR title must be a
Conventional Commit too:** this repository squashes with `COMMIT_OR_PR_TITLE`, so
when a PR has multiple commits, its title is the subject that lands on `main`.

An active ruleset makes the `conventional` check required on `main` for outside
contributors. Repository admins bypass that ruleset in `always` mode by design;
the local hook remains their commit gate. The bypass is also what makes the
automated release PR mergeable: a PR opened by the workflow's `GITHUB_TOKEN`
does not trigger workflow runs, so its required `conventional` check can never
post.

Subjects git generates itself — merge commits, `fixup!`, `squash!`, `amend!` —
pass through, since rejecting them breaks merge and interactive rebase.
`git revert` is not exempt: `revert` is a type, so edit its generated subject like
any other.

Write the body for someone deciding whether to trust the change: what failed, what
the fix is, and what proved it. Scraped-marker changes say which harness version
they were verified against — that pin is what `gang vet` reads.

## Releases

[release-please](https://github.com/googleapis/release-please) owns releases; no
repository script or hand-written changelog step does. Pushes to `main` maintain
one release PR from the Conventional Commits since the last tag. Merging
that PR creates the tag and GitHub Release and writes `CHANGELOG.md`.
**Do not edit `CHANGELOG.md` by hand or add changelog entries to ordinary PRs.**

The repository setting **Settings → Actions → General → Workflow permissions →
Allow GitHub Actions to create and approve pull requests** must be enabled. It is
not enabled automatically. Without it, release-please creates its branch and
release commit, then fails at PR creation, so no release PR is maintained.

`.release-please-manifest.json` is the version source. A release commit updates
that manifest, `version.txt`, `packaging/npm/package.json`, and
`packaging/pypi/pyproject.toml`; do not introduce another version location or
bump one of those files through a second mechanism. The initial manifest is
anchored at the unpublished `0.0.1`, and the verified release-please dry run
plans the first release as `0.1.0`.

The npm and PyPI stubs are not published packages yet; install through
`install.sh`, not a registry. After every release, verify both packaging stubs
moved with the manifest: the extra-file updaters can leave a missing or malformed
JSONPath unchanged while the release workflow remains green.

## Before adding code

- **Law 5** — nothing lands without a live consumer. If nothing invokes it the day
  it merges, it does not merge.
- **Law 4** — per-harness knowledge is a profile, not a code path. A harness-specific
  branch in `bin/gang` needs an ADR in `docs/adr/`.
- **Law 9** — when in doubt, the answer is prose in an agent's prompt, not code here.

**A profile must never lower the operator's security posture.** No shipped profile
may put a harness into a mode that skips approval prompts or disables its sandbox —
`--dangerously-bypass-approvals-and-sandbox`, `approval_policy = "never"`,
`sandbox_mode = "danger-full-access"`, `--yolo`, or whatever the next harness calls
it. A profile describes how to *drive* a harness, not how much authority to hand it.

The reason it is tempting is exactly why it is banned: an agent gated behind a
permission prompt is refused by `gang send` and no keystroke from a teammate can
clear it, so "just turn the prompts off" reads like a fix for a coordination
problem. It is not. It is a decision only the person at the keyboard can make, on
their own machine, in their own config — and shipping it as a default means every
future installer inherits a choice they never made and probably never read.

Put it in `~/.codex/config.toml` (or the harness's equivalent), never in
`profiles/`. If a gated agent is blocking a team, that is a thing to tell the
operator, not a thing to engineer around.

Every shell and Python file carries an `SPDX-License-Identifier` line; new ones
need it too.

Shell changes must pass `bash -n`, `shellcheck -S warning`, and
`test/integration.sh`; the `shell` workflow runs all three on every push. A local
green run is not a CI prediction on the current development box: local mawk
1.3.4 is more permissive than CI's gawk and hid a `substr` byte/character bug;
local Bash 5.1 is newer than macOS CI's 3.2, which rejected 37 locally valid test
constructs; and local ShellCheck 0.8.0 is more permissive than CI's newer parser,
which treated a comment beginning `shellcheck` as a directive and failed lint.

Measure with the thing under test, not a stand-in for it. Some agent harnesses
wrap `grep` in a shell function for their own interactive use; a measurement taken
at such a prompt reported that wrapper's exit codes as the tool's, when every
script here runs `/usr/bin/grep` and gets different ones. Run a measurement the way
the code runs it — in a script, through the same binary — or the number belongs to
the stand-in. Same reason the suite refuses mocks: a mock agrees with whatever the
code already does.

The test
drives `bin/gang` against a real tmux server with the `bash` profile — no mocks,
because a mocked tmux would agree with whatever the code already does. A fix to
delivery or addressing belongs there as a case that fails without the fix.

## License

Contributions land under Apache-2.0, the same license the repo ships under:
section 5 puts anything you intentionally submit for inclusion under those terms
unless you say otherwise in the pull request. There is no CLA to sign.
