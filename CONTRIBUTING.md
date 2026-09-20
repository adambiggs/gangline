# Contributing

Read `CONSTITUTION.md` and `AGENTS.md` before changing the repository.

## Setup

Enable the repository hooks:

```sh
git config core.hooksPath .githooks
```

Do not use `--no-verify`. New shell and Python files need an SPDX license
identifier. `.githooks/pre-push` runs the operator's outer contribution gate,
then fast lint and smoke on the pushed tree. CI runs full lint, integration,
and the commit-message check on pushes to `main`.

## Check your change

Run the gate before committing:

```sh
test/gate.sh
```

The gate runs fast lint and smoke against the working tree. CI runs full lint
and integration on pushes to `main`.

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
