# Contributing

Read [docs/design.md](docs/design.md) and [AGENTS.md](AGENTS.md) before you
change anything.

## Prepare the checkout

Enable the repository hooks from the checkout root:

```sh
git config core.hooksPath .githooks
test/gate.sh
```

The pre-push hook runs your global pre-push hook first, if you have one, then
the repository's checks.

A successful baseline ends with:

```text
gate: VERDICT PASS (status 0); this gate ran the Go checks and private-tmux acceptance scenarios.
```

The gate runs formatting, vet, unit tests, and tmux acceptance tests on the
working tree. Only one gate runs at a time on a machine. It ends with `PASS`,
`REFUSED`, or `UNKNOWN`; no verdict line means it didn't pass.

New shell and Python build-support files need an SPDX license identifier. Do
not hand-edit `CHANGELOG.md`; Release Please owns it.

## Make and check a change

Run the gate at each checkpoint:

```sh
test/gate.sh
```

CI runs the Go checks on Linux and macOS. Test rules are in
[AGENTS.md](AGENTS.md).

## Commit

Use a Conventional Commit subject:

```text
<type>[(scope)][!]: <description>
```

Allowed types are `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`,
`refactor`, `revert`, `style`, and `test`. Add a `BREAKING CHANGE:` footer when
callers must change how they use Gangline.

Stage only the files you changed. Don't use `--no-verify`.

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
