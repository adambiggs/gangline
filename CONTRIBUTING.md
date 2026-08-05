# Contributing

Read `CONSTITUTION.md` and `AGENTS.md` before changing the repository. Their
rules are binding.

## Setup

Enable the repository hooks:

```sh
git config core.hooksPath .githooks
```

The hooks are not enabled automatically. `.githooks/pre-push` is the
authoritative list of local push gates. Do not bypass it with `--no-verify`.

## Scope and implementation

- Add only behaviour with a current, concrete consumer.
- Keep harness-specific knowledge in `profiles/`; do not branch on harness names
  in `bin/gang` without the decision required by `CONSTITUTION.md`.
- Do not add supervisors, daemons, retry managers, private transports, or silent
  degraded modes.
- Prefer deletion when Gangline cannot make a surface safe or truthful.
- A profile may drive a harness but must never disable its sandbox, bypass its
  approvals, or otherwise lower the operator's security posture.
- Put operator security choices in the operator's harness configuration, never
  in a shipped profile.
- Add an SPDX license identifier to every new shell or Python file.

Before editing `bin/gang`, resolve the executable on `PATH`:

```sh
readlink -f "$(command -v gang)"
```

If it resolves into this checkout, every save is immediately live for all users
of that executable. Keep edits and checkpoints small.

## Tests

Run the repository gates:

```sh
test/lint.sh
test/integration.sh
```

The following rules are mandatory:

- The complete mandatory suite must finish in under five minutes; seconds are
  the normal case.
- Executable tests must not sleep, poll for eventual state, test timeout
  behaviour, or use wall-clock delay as evidence.
- Use an immediate fake clock when time is an input.
- Assert state that the command has already established.
- Keep indeterminate distinct from both pass and fail. A negative assertion must
  not pass because its fixture produced no value.
- Use a private tmux server and disposable session for integration tests. Never
  address the live `gangline` session.
- Real harness turns are explicit operator probes, not mandatory tests. Run them
  through `gang vet --probe` or a separately named tmux session. Never enroll the
  development agent to test Gangline.
- Existing assertions are guards. Changing or deleting one requires proving that
  its old expectation was wrong; do not edit it merely to make a behaviour change
  green.

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

Pull request titles must also be Conventional Commits. Commit bodies should state
what failed, what changed, and what proves the result. Marker changes must name
the harness version that was observed and add it to the profile's verified pins.

When multiple contributors share a checkout:

- Assign one writer per file.
- Inspect the worktree before staging.
- Stage exact paths with `git add -- <paths>`; do not use `git add -A` in a dirty
  shared tree.
- Do not commit, discard, or rewrite another contributor's work.
- Land a green checkpoint before handing a file to another writer.

## Public content

Commits and pushes pass through the repository PII gate. Issue and pull request
bodies do not; scan them before publishing:

```sh
tools/pii-scan --stdin < body.txt
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
  `docs/operations.md`, vocabulary in `docs/field-guide.md`, and product purpose
  in `README.md`.
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
