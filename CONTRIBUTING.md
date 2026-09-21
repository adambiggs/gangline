# Contributing

Use this page to prepare a checkout, run the required checks, and create a
commit that CI accepts. Before you edit, read [CONSTITUTION.md](CONSTITUTION.md)
and [AGENTS.md](AGENTS.md); both are binding for this repository.

## Prepare the checkout

Enable the repository hooks from the checkout root:

```sh
git config core.hooksPath .githooks
test/gate.sh
```

A successful baseline ends with:

```text
gate: VERDICT PASS (status 0); this gate ran lint, smoke, and Go checks; shell integration runs in CI.
```

The gate runs fast shell lint, smoke checks, Go formatting and analysis, unit
tests, and the tmux acceptance scenario against the working tree. It serializes
with other local gate runs and reports `PASS`, `REFUSED`, or `UNKNOWN`; an
absent verdict is not success.

New shell and Python files need an SPDX license identifier. Do not hand-edit
`CHANGELOG.md`; Release Please owns it.

## Make and check a change

Keep changes inside Gangline's documented surface: tmux lifecycle, verified
delivery, direct observation, collars, native hooks and compaction, startup
prose, and optional context or capacity indicators.

Run the gate again at each coherent checkpoint:

```sh
test/gate.sh
```

CI runs full lint and integration after a push to `main`. Mandatory tests use
immediate state, event barriers, or fake clocks instead of sleeps and polling.
When a test needs native behavior, use a separately named disposable Gangline
session; never use a development team as the test subject.

## Commit

Use a Conventional Commit subject:

```text
<type>[(scope)][!]: <description>
```

Allowed types are `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`,
`refactor`, `revert`, `style`, and `test`. Add a `BREAKING CHANGE:` footer when
callers must change how they use Gangline.

Stage only the files you changed, then commit normally. Do not use
`--no-verify`; the pre-push hook runs the contribution gate and a fast check of
the pushed tree.

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
