# Contributing

Read `CONSTITUTION.md` and `AGENTS.md` before changing the repository. Their
rules are binding.

## Setup

Enable the repository hooks:

```sh
git config core.hooksPath .githooks
```

The hooks are not enabled automatically. `.githooks/pre-push` is the
authoritative list of local push gates: the operator's outer contribution gate,
production and hook lint, the fast executable smoke, and commit messages. It
names the test lint, checker self-tests, and full integration suite it skips;
CI runs those checks on every push to `main`. Do not bypass the hook with
`--no-verify`.

## Scope and implementation

- Add only behaviour with a current, concrete consumer.
- Keep harness-specific knowledge in `collars/`; do not branch on harness names
  in `bin/gang` without the decision required by `CONSTITUTION.md`.
- Do not add the machinery classes `CONSTITUTION.md` bans (laws 1, 7, and 8).
- Prefer deletion when Gangline cannot make a surface safe or truthful.
- A collar may drive a harness but must never disable its sandbox, bypass its
  approvals, or otherwise lower the operator's security posture.
- Put operator security choices in the operator's harness configuration, never
  in a shipped collar.
- Add an SPDX license identifier to every new shell or Python file.

Before editing `bin/gang`, resolve the executable on `PATH`:

```sh
readlink -f "$(command -v gang)"
```

If it resolves into this checkout, every save is immediately live for all users
of that executable. Keep edits and checkpoints small.

## Tests

Run the repository gate:

```sh
test/gate.sh
```

It snapshots the working tree, uncommitted work included, into a private copy,
commits it there, and runs `test/lint.sh`, `test/smoke.sh`, and
`test/integration.sh` from that
copy. The complete gate is therefore runnable before a commit, and no edit
landing mid-run can change what executes. `test/lint.sh` and
`test/integration.sh` still run directly against an already-settled tree, and
refuse one they would not own.

The following rules are mandatory:

- The complete mandatory suite must finish in under five minutes; seconds are
  the normal case.
- Executable tests must not sleep, poll for eventual state, test timeout
  behaviour, or use wall-clock delay as evidence.
- Use an immediate fake clock when time is an input.
- Assert state that the command has already established.
- Keep unknown distinct from both pass and fail. A negative assertion must
  not pass because its fixture produced no value.
- Treat panes, captures, transcripts, and logs as combined surfaces. A positive
  text claim against one must name whether the whole surface is the intended
  evidence or identify an independent producer witness. `test/source-guards.py`
  discovers capture producers and positive `contains`, `equal`, quiet `grep`,
  `[[ … ]]`, wrapper, and `case` guards. For entered text, an
  empty composer after verified Enter is the producer witness; for execution,
  prefer an artifact only execution can create; for timing, assert both sides
  of the boundary. A later feature can otherwise supply an old guard's text
  without changing either feature or guard incorrectly. A legitimate claim
  about any visible source uses the adjacent, statement-fingerprinted
  `source-guard: whole-surface@DIGEST: rationale`; a conjunctive source witness
  uses `source-guard: producer@DIGEST: rationale`. Run the checker once without
  the annotation to get the exact forms it will accept. The introduction-time
  migration ledger is closed to new entries: copying, moving, changing, or
  deleting one of its reviewed assertions requires a fresh inline decision.
- Use a private tmux server and disposable session for integration tests. Never
  address the live `gangline` session.
- Real harness turns are explicit operator smoke tests, not mandatory tests. Run
  them in a separately named disposable tmux session. Never enroll the
  development agent to test Gangline.
- Preserve existing assertions as required by `AGENTS.md`.

`test/lint.sh` enforces the shell timing ban across `test/` and executable CI
helpers. `.github/workflows/shell.yml` enforces the suite ceiling.

## Commits

Use Conventional Commits:

```text
<type>[(scope)][!]: <description>
```

Allowed types are `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`,
`refactor`, `revert`, `style`, and `test`. Use a lowercase scope when present.
Pair `!` with a `BREAKING CHANGE:` footer that tells callers what to update.

A commit message describes the change, not the process that produced it: no
agent, reviewer, session, harness, or model names, no attribution trailers, and
no provenance at all. Where a change came from, who found it, and on which pass
are facts about how this repository is worked on rather than about the change,
and a rephrasing that keeps the fact while dropping the name is still process.
The change describes itself.

Pull request titles must also be Conventional Commits. Commit bodies should state
what failed, what changed, and what proves the result. Marker changes must name
the harness version that was observed and add it to the collar's verified pins.

When multiple contributors share a checkout:

- Assign one writer per file.
- Inspect the worktree before staging.
- Stage exact paths with `git add -- <paths>`; do not use `git add -A` in a dirty
  shared tree.
- Do not commit, discard, or rewrite another contributor's work.
- Land a green checkpoint before handing a file to another writer.

## Public content

The local pre-push hook delegates to the operator's installed Snubline gate,
which is the only PII scan Gangline runs — there is no CI backstop, so a clone
without Snubline pushes unscanned. Issue and pull request bodies reach no hook at
all, so scan them, and anything else you are about to publish, with the installed
Snubline scanner first:

```sh
scanner="$(git config --global --get snubline.piiScanner)" || {
  echo "Snubline scanner is not configured" >&2
  exit 1
}
bash "$scanner" --stdin < body.txt
```

## Releases

- Release Please owns release commits, tags, GitHub Releases, and `CHANGELOG.md`.
- Do not edit `CHANGELOG.md` manually or add changelog entries to ordinary pull
  requests.
- `.release-please-manifest.json` is the version source. Release commits update
  `version.txt` and the npm and PyPI package metadata with it; do not add another
  version source or bump these files independently.
- The npm and PyPI stubs are not published packages. Install with `install.sh`.
- Keep GitHub Actions permission to create pull requests enabled so Release
  Please can maintain its release PR.

## Documentation and measurement

- Use **Gangline** for the project and `gang` for the command.
- Use *hitch* for adding an agent; never *hire*.
- Put command syntax and configuration in `docs/reference.md`, operations in
  `docs/operations.md`, and product purpose in `README.md`.
- Do not write changing counts, versions, sizes, or tallies into documentation.
  Point to the command that reports them.
- When a default changes, read every section describing its semantics; a literal
  search alone is insufficient.
- Measure the same executable, path, environment, and substrate the product uses.
- Require fixture readiness as an immediate observable before asserting product
  state.
- Report "could not determine" when evidence is absent or contradictory. Never
  spend missing evidence as a conservative pass.

## License

Contributions are submitted under Apache-2.0 unless the pull request explicitly
states otherwise. No contributor license agreement is required.
