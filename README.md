# Gangline

Gangline unites CLI AI coding harnesses into one harmonious mushing team via
tmux, then gives that team an unopinionated toolkit for productive, fully
autonomous, long-horizon sessions.

Claude Code and Codex are first-class today; a collar is how another harness
joins.

[![Gangline demonstration](site/demo.gif)](https://gangline.ai/#demo)

Gangline brings together:

- one harmonious mushing team: named native sessions and attributed, verified
  delivery; and
- long-horizon work: context and cache bands, provider caps and curfews,
  harness and model choice, direct observation, and recovery.

The mechanisms are the product. A lead is the one fixed role; the doctrine and
other role briefs are starter defaults you replace. Gangline is not a task
graph, supervisor, daemon, or database.

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

`gang up` opens a tmux session and attaches you to a window named `lead`.
Detach with `Ctrl-b d`; from the same directory, confirm the running team with:

```sh
gang roster
```

Use `gang --help` for the command list and `gang <command> --help` for a
command's options.

## One team, long-horizon work

One team is more than tmux windows beside one another: Gangline delivers
attributed messages through each recipient's terminal and reports delivery only
after it sees them accepted. It works between top-level native sessions; it
does not replace a harness's own subagents.

Long-horizon work keeps a team moving for the long haul. Context and cache bands,
provider caps, and curfews make capacity visible; harness and model choice let
the team spend quota and cost deliberately. `gang roster`, `gang capture`, and
`gang status` give a human direct evidence after a long run, so they can decide
whether to drop an agent or resume it.

## Documentation

- [Reference](docs/reference.md) — commands and configuration
- [Operations](docs/operations.md) — recovery and longer-running teams
- [Design](docs/design.md) — decisions that shape the project
- [Contributing](CONTRIBUTING.md) — local checks and commit conventions
- [Security](SECURITY.md) — private vulnerability reporting

Gangline is licensed under Apache-2.0.
