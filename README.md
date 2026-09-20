# Gangline

Gangline starts Claude Code and Codex agents in tmux, carries attributed
messages between them with verified delivery, and keeps unattended sessions
alive while context and provider quota remain visible. A collar is the small
adapter that teaches Gangline how to launch and read a CLI harness.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="site/demo.gif">
  <source media="(prefers-color-scheme: light)" srcset="site/demo-light.gif">
  <img alt="Gangline demonstration" src="site/demo.gif">
</picture>

Each agent is a named native session. Gangline supplies message delivery,
context and cache bands, provider caps and curfews, harness and model choice,
direct observation, and recovery. It is not a task graph, supervisor, daemon,
or database.

## Quick start

You need macOS or Linux, Bash, tmux 3.2 or later, Python 3, and either Claude
Code or Codex. Run the harness you plan to use once first, so it can complete
its normal sign-in and repository prompts.

Install Gangline, move to a repository where the team should work, and start a
team:

```sh
curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
cd ~/src/my-project
gang up -c claude-code -m sonnet -e high
```

`gang up` opens a tmux session and attaches you to `lead`, the fixed first
agent.
Detach with `Ctrl-b d`; from the same directory, confirm the running team with:

```sh
gang roster
```

Use `gang --help` for the command list and `gang <command> --help` for a
command's options.

## One team, long-running work

Gangline delivers attributed messages through each recipient's terminal and
reports delivery only after it sees them accepted. It works between top-level
native sessions; it does not replace a harness's own subagents.

Context and cache bands, provider caps, and curfews make capacity visible during
long runs. `gang roster`, `gang capture`, and `gang status` provide the evidence
needed to decide whether to resume an agent or remove it with `gang drop`.

## Documentation

- [Architecture](docs/architecture.md) — layers, message path, and code map
- [Reference](docs/reference.md) — commands and configuration
- [Operations](docs/operations.md) — recovery and longer-running teams
- [Design](docs/design.md) — decisions that shape the project
- [Contributing](CONTRIBUTING.md) — local checks and commit conventions
- [Security](SECURITY.md) — private vulnerability reporting

Gangline is licensed under Apache-2.0.
