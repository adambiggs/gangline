# Contributing

Read the [design principles](docs/internals.md#design-principles) and
[AGENTS.md](AGENTS.md) before changing behavior. The [internals](docs/internals.md)
page describes the packages and message path.

## Build and prepare the checkout

Install the Go toolchain declared in [go.mod](go.mod). On macOS, install `tmux`,
`flock`, `timeout`, and `python3` for the full gate. Check that the commands are
available before building; each line prints an executable path:

```sh
for tool in go git tmux flock timeout python3; do command -v "$tool" || exit 1; done
```

Build a checkout-local binary and print its command help:

```sh
go build -o ./bin/gang ./cmd/gang
./bin/gang --help
```

The build writes `bin/gang`, which Git ignores. Source edits do not change an
installed `gang`. Use `./bin/gang` for manual tests; leave a running team's
executable in place.

Enable the repository hooks from the checkout root, then run the gate. The
configuration command prints nothing on success. The gate's last line states
its verdict:

```sh
git config core.hooksPath .githooks
test/gate.sh
```

The pre-push hook runs your global pre-push hook first, if you have one, then
checks the pushed Go trees with `test/go.sh --push`. This runs formatting, vet,
sum-type, package-boundary, and unit checks. The full `test/gate.sh` also runs
distribution and private tmux acceptance scenarios. On macOS, run the gate
where it can inspect host processes and create private tmux sessions; a
restricted sandbox can produce `operation not permitted` failures. The live
provider checks launch Claude Code and Codex when their commands and
authentication are available. A native trust or hook review prompt can refuse
the gate and leave its private socket path in the output for review. On Linux,
the gate reports a skip for bus-dependent acceptance if the systemd user bus
is unreachable.

A successful baseline ends with:

```text
gate: VERDICT PASS (status 0); test/go.sh completed; see acceptance result above.
```

The gate checks the working tree and serializes runs on the host. It ends with
`PASS`, `REFUSED`, or `UNKNOWN`; no verdict line means the result is unknown.

The site build needs the Node release pinned in [.nvmrc](.nvmrc) and npm.
Select that release with your Node manager, then check the active version:

```sh
node --version
```

The printed version must match `.nvmrc` before running the site commands.

New shell and Python build-support files need an SPDX license identifier. Do
not hand-edit `CHANGELOG.md`; Release Please owns it.

## Make and check a change

Run the gate at each checkpoint:

```sh
test/gate.sh
```

CI runs the Go checks on Linux and macOS. Test rules are in
[AGENTS.md](AGENTS.md).

The Astro site in `site/` renders the repository's Markdown directly. After
changing docs or navigation, install its locked dependencies and build it:

```sh
npm ci --prefix site
npm run build --prefix site
```

The install populates Git-ignored `site/node_modules/`. The build writes
Git-ignored `site/dist/` and reports the local link and navigation checks.

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
