# Contributing

Use this page to prepare a checkout, run the required checks, and create a
commit that CI accepts. Before you edit, read [docs/design.md](docs/design.md)
and [AGENTS.md](AGENTS.md); both are binding for this repository.

## Prepare the checkout

Enable the repository hooks from the checkout root:

```sh
git config core.hooksPath .githooks
test/gate.sh
```

The pre-push hook first runs the user's global pre-push hook, selected by global
`core.hooksPath` or `${XDG_CONFIG_HOME:-~/.config}/git/hooks`, when executable.
It passes the original remote arguments and ref updates to both hook stages;
a global refusal stops the push before repository checks run. A scoped callback
guard lets a global hook call the repository hook without recursing; the outer
invocation still runs the repository checks once.

A successful baseline ends with:

```text
gate: VERDICT PASS (status 0); this gate ran the Go checks and private-tmux acceptance scenarios.
```

The gate runs Go formatting and analysis, unit tests, and the tmux acceptance
scenarios against the working tree. It serializes
with other local gate runs and reports `PASS`, `REFUSED`, or `UNKNOWN`; an
absent verdict is not success.

New shell and Python build-support files need an SPDX license identifier. Do
not hand-edit `CHANGELOG.md`; Release Please owns it.

## Make and check a change

Keep changes inside Gangline's documented surface: tmux lifecycle, verified
delivery, direct observation, collars, native hooks and compaction, startup
prose, and optional context or capacity indicators.

Run the gate again at each coherent checkpoint:

```sh
test/gate.sh
```

CI repeats the Go checks on Linux and macOS. Mandatory tests use immediate
state, event barriers, or fake clocks instead of sleeps and polling.
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
`--no-verify`; the pre-push hook runs the Go checks against the pushed tree.

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
