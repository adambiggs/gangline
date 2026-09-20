# Gangline

Gangline connects CLI coding harnesses as one team in tmux. It starts Claude
Code and Codex in named windows, delivers attributed messages between them, and
keeps context, provider limits, and token use visible so you can manage quota
and cost during long-running work.

Use Gangline when several native harness sessions need to share work without
giving up their own terminal UI, tools, permissions, or session history.
Gangline supplies the team transport and operational evidence. It does not plan
the work or supervise the agents.

## Install

You need macOS or Linux, Git, Bash, Python 3, tmux 3.2 or later, and at least
one supported harness. Run that harness directly once so it can finish its own
sign-in and repository prompts.

Install the latest stable release:

```sh
curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
```

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="site/demo.gif">
  <source media="(prefers-color-scheme: light)" srcset="site/demo-light.gif">
  <img alt="A Gangline team working in native terminal windows" src="site/demo.gif">
</picture>

The installer places `gang` in `~/.local/bin` by default and tells you if that
directory is not on `PATH`. Confirm the command is available:

```sh
gang --version
gang collars
```

Run `gang upgrade --check` later to check for a newer stable release. Run
`gang upgrade` to install it.

## Start a team

Move to the repository where the team should work, then start the lead:

```sh
cd ~/src/my-project
gang up -c claude-code -m sonnet -e high
```

`gang up` creates a tmux session and attaches you to `lead`, the fixed first
role. Detach with `Ctrl-b d`. From a shell outside the team, verify the running
session:

```console
$ gang roster
lead             claude-code  ~idle~             hitcher=operator
```

Your row may include more live evidence, such as recent tool activity, context
use, or queued messages. Return to the team with `gang attach`.

## Add a teammate

List the choices exposed by a harness, then hitch a named agent:

```sh
gang models -c claude-code
gang hitch worker -c claude-code -d "$PWD" -m sonnet -e high -r worker \
  -t 'Inspect the parser failure and report the evidence.'
```

To use Codex instead, run `gang models -c codex` and pass one listed model and
effort to `gang hitch`.

Send more work from an operator shell with an explicit sender:

```sh
printf '%s\n' 'Run the focused test and report the result.' |
  gang send worker --from operator
```

Inside an agent window, omit `--from`; Gangline reads the sender from tmux.
Delivery succeeds only when Gangline can verify the recipient accepted the
message. If the recipient is busy, the message waits for a safe native turn
boundary.

## Choose the next page

- [Operations](docs/operations.md) — run, inspect, recover, and stop a team.
- [Architecture](docs/architecture.md) — understand the runtime model and
  message path.
- [CLI and configuration reference](docs/reference.md) — look up every command,
  setting, state, and collar declaration.
- [Design principles](CONSTITUTION.md) — understand the constraints that keep
  Gangline small.
- [Design decisions](docs/design.md) — understand non-obvious behavior before
  changing it.
- [Contributing](CONTRIBUTING.md) — prepare a checkout and run the required
  checks.
- [Security](SECURITY.md) — report a vulnerability privately.

Gangline is licensed under Apache-2.0.
