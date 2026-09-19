# Contributing

Read `CONSTITUTION.md` and `AGENTS.md` before changing the repository.

## Setup

Enable the repository hooks, and do not bypass them with `--no-verify`:

```sh
git config core.hooksPath .githooks
```

`.githooks/pre-push` runs the operator's outer contribution gate, production and
hook lint, the fast smoke, and the commit-message check. CI runs the rest on
every push to `main`.

That outer gate can spend minutes while git's connection to the remote sits
idle. Configure SSH keepalives, or a green gate is followed by `Connection to
github.com closed by remote host` and the ref never lands:

```sh
git config --global core.sshCommand \
  'ssh -o ServerAliveInterval=30 -o ServerAliveCountMax=20'
```

The Linux integration suite needs a reachable systemd user manager for its
reaper; `systemd-run --user --wait --pipe --collect --service-type=exec /bin/true`
is the probe.

Add an SPDX license identifier to every new shell or Python file.

## The gate

```sh
test/gate.sh                 # from a plain shell
gang run -- test/gate.sh     # from a Gangline agent pane
```

The gate runs `test/lint.sh` and `test/smoke.sh` against a private snapshot of
the working tree, uncommitted work included, and prints its verdict on the last
line. From an agent pane, commit only after Gangline delivers the run's result.
`test/integration.sh` is the full suite; CI runs it on every push to `main`.

`test/release.sh` runs lint, smoke and full integration against one settled
tree. Before merging a Release Please pull request, confirm Release Please
created it, approve its `action_required` run, and require its `release` job to
pass on the current head SHA.

`test/e2e.sh` boots a real claude-code TUI against a local stub server. It is
opt-in and runs daily in CI; it never gates a commit.

## Commits

Use Conventional Commits:

```text
<type>[(scope)][!]: <description>
```

Allowed types are `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`,
`refactor`, `revert`, `style`, and `test`. Use a lowercase scope when present.
Pair `!` with a `BREAKING CHANGE:` footer that tells callers what to update.
Pull request titles follow the same format.

A commit message describes the change, not the process that produced it: no
agent, reviewer, session, harness or model names, no attribution trailers. The
body states what failed, what changed, and what proves it.

Stage exact paths with `git add -- <paths>` in a shared checkout, and never
commit or discard another contributor's work.

A decision that still shapes the code is recorded as a paragraph in
[`docs/design.md`](docs/design.md), landed with the change.

Release Please owns release commits, tags, `version.txt`, package metadata and
`CHANGELOG.md`; never edit them by hand.

## Public content

The pre-push hook's Snubline gate is the only PII scan; there is no CI
backstop. Issue and pull request bodies reach no hook, so scan them first:

```sh
snub scan-text < body.txt
```

## License

Contributions are submitted under Apache-2.0 unless the pull request explicitly
states otherwise.
