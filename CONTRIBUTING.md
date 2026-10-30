# Contributing

Read the [design principles](docs/internals.md#design-principles) and
[AGENTS.md](AGENTS.md) before changing behavior. The [internals](docs/internals.md)
page describes the packages and message path.

## Build and prepare the checkout

Install the Go toolchain declared in [go.mod](go.mod). Check the mandatory
gate's tools before building; each line prints an executable path:

```sh
for tool in go git; do command -v "$tool" || exit 1; done
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
runs `test/gate.sh` on each pushed tree. CI runs the same mandatory lane:
formatting, vet, sum types, package boundaries, and deterministic tests.
It prints each part's wall time and the total, without a killing deadline.

A successful baseline ends with:

```text
gate: VERDICT PASS (status 0); deterministic checks completed.
```

The gate checks the working tree. It ends with `PASS`, `REFUSED`, or `UNKNOWN`;
no verdict line means the result is unknown.

Run the pre-release lane with its additional host tools:

```sh
for tool in tmux flock python3; do command -v "$tool" || exit 1; done
test/acceptance.sh
```

The release workflow runs acceptance before building assets and stops on
failure. This lane runs distribution HTTP fixtures, persistence, subprocess,
private tmux, and live provider checks, with per-part and total wall times.
Tests needing real services or writable filesystem fixtures use the
`acceptance` build tag; `go test -tags=acceptance ./...` includes them alongside
the deterministic tests. Fake-clock tests use `testing/synctest`.

Acceptance serializes runs on the host. On macOS, run it where it can inspect
host processes and create private tmux sessions. The provider checks launch
Claude Code and Codex when their commands and authentication are available;
skips print their reasons and leave those providers unverified. A native
prompt can refuse acceptance and leave its private socket path for review.
On Linux, an unreachable systemd user bus skips the bus-dependent lifecycle
check with its reason.

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

Test rules are in [AGENTS.md](AGENTS.md).

The Astro site in `site/` renders the repository's Markdown directly. After
changing docs or navigation, install its locked dependencies and build it:

```sh
npm ci --prefix site
npm run build --prefix site
```

The install populates Git-ignored `site/node_modules/`. The build writes
Git-ignored `site/dist/` and reports the local link and navigation checks.
GitHub Pages publishes the site when a release is created.

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
