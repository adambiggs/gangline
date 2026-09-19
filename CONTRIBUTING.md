# Contributing

Read `CONSTITUTION.md` and `AGENTS.md` before changing the repository.

## Setup

Enable the repository hooks:

```sh
git config core.hooksPath .githooks
```

Do not use `--no-verify`. New shell and Python files need an SPDX license
identifier.

## Check your change

Run the gate before committing:

```sh
test/gate.sh
```

From a Gangline agent pane, use `gang run -- test/gate.sh` instead. CI runs the
full integration suite on pushes to `main`.

## Commit

Use Conventional Commits:

```text
<type>[(scope)][!]: <description>
```

Allowed types are `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`,
`refactor`, `revert`, `style`, and `test`. A breaking change includes a
`BREAKING CHANGE:` footer explaining the update callers need to make.

Stage only the paths you changed. Do not edit release files managed by Release
Please. Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
