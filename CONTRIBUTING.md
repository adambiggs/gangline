# Contributing

Read the [design principles](docs/internals.md#design-principles) and
[AGENTS.md](AGENTS.md) before changing behavior. The [internals](docs/internals.md)
page describes the packages and message path.

## Build and prepare the checkout

Use the Go toolchain declared in [go.mod](go.mod). Build a separate binary:

```sh
go build -o ./bin/gang ./cmd/gang
./bin/gang --help
```

Editing source does not change an installed `gang`. Test with the build you
just made, and keep a running team's executable in place.

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
working tree. Gate runs are serialized on the host. It ends with `PASS`,
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

The site renders the repository's Markdown with `tools/docsite`. Its page
inventory controls navigation. After changing docs or navigation, assemble
the site and render it using the steps in
[the Pages workflow](.github/workflows/pages.yml); the renderer checks local
links and heading anchors.

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
