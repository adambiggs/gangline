# Gangline

Gangline runs Claude Code and Codex agents as one team in tmux. Each agent is
a normal harness in its own window, and agents message each other by name. A
message counts as delivered only when the harness confirms it received it.

Gangline doesn't manage the agents. They keep their own UI, tools,
permissions, and history, and your prompts decide how they work.

## Install

Gangline runs on macOS and Linux. It needs Git, Go 1.27 or later, tmux 3.2 or
later, and Claude Code or Codex. Run each harness by itself once first, so its
sign-in and trust prompts are out of the way.

Install the latest stable release:

```sh
curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
```

This installs the latest release's `gang` into `~/.local/bin`. Check it:

```sh
gang --version
gang collars
```

`gang upgrade --check` shows whether a newer release exists, and
`gang upgrade` installs it.

## Start a team

From the repository where the team should work:

```sh
gang up -c claude-code -m sonnet -e high
```

This starts the tmux session, launches the first agent as `lead`, and
attaches. Detach with `Ctrl-b d` and come back with `gang attach`.

Common commands:

```sh
gang roster
gang status lead --why
gang capture lead
gang hitch worker -c codex -d "$PWD" -r worker -t 'Run the focused checks.'
printf '%s\n' 'Report the result.' | gang send worker --from operator
gang log
```

From an agent's window, `gang send` signs the message with that agent's name.
From your own shell, pass `--from`.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="site/demo.gif">
  <source media="(prefers-color-scheme: light)" srcset="site/demo-light.gif">
  <img alt="A Gangline team working in native terminal windows" src="site/demo.gif">
</picture>

## Documentation

- [Operations](docs/operations.md)
- [CLI and collar reference](docs/reference.md)
- [Architecture](ARCHITECTURE.md)
- [Design decisions](docs/design.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)

Gangline is licensed under Apache-2.0.
