# Gangline

Gangline unites CLI AI coding harnesses into one harmonious mushing team via
tmux, then gives that team an unopinionated toolkit for effective, productive,
fully autonomous, long-horizon sessions.

Claude Code and Codex are first-class today. Gangline puts them in named tmux
windows, verifies their message delivery, and keeps the human able to inspect
the team directly. A collar is how another native harness joins.

[![Gangline demonstration](site/demo.gif)](https://gangline.ai/#demo)

The two pillars are:

- one harmonious mushing team: named native sessions, attributed and verified
  delivery, and each harness used for its own strengths; and
- long-horizon work: an unopinionated toolkit for context and cache bands,
  provider caps and curfews, harness and model choice, direct observation, and
  recovery.

The mechanisms are the product. A lead is the one fixed role; the doctrine and
every other role brief ship as starter defaults that setup expects you to
replace. Gangline is not a task graph, supervisor, daemon, or database. It
connects native sessions without taking over their terminals, tools,
permissions, or subscriptions.

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

## The two pillars

Pillar one is a team that is more than tmux windows beside one another.
Gangline delivers attributed messages through each recipient's terminal and
reports delivery only after it sees the terminal accept them. It operates
between independent top-level native sessions; it does not replace subagents a
harness creates for itself.

Pillar two is a team that can work for the long haul. Context and cache bands,
provider caps, and curfews make capacity visible; harness and model choice let
the team spend quota deliberately. `gang roster`, `gang capture`, and `gang
status` give a human direct evidence after a long run, so they can decide
whether to drop an agent or resume its work. Native dialogs remain native:
Gangline will not answer a permission or trust prompt for you.

## Documentation

- [Reference](docs/reference.md) — commands and configuration
- [Operations](docs/operations.md) — recovery and longer-running teams
- [Design](docs/design.md) — decisions that shape the project
- [Contributing](CONTRIBUTING.md) — local checks and commit conventions
- [Security](SECURITY.md) — private vulnerability reporting

Gangline is licensed under Apache-2.0.
