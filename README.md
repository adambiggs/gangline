# Gangline

Gangline runs Claude Code and Codex as named windows in one tmux session, so
they can hand each other work by name and you can watch the whole thing happen.

[![Gangline demonstration](site/demo.gif)](https://gangline.ai/#demo)

Gangline provides a small set of shared primitives:

- start, attach to, observe, and stop native harnesses;
- send attributed messages through their terminals and verify delivery; and
- report a conservative state: `-busy-`, `~wait~`, `~idle~`, `!occupied!`, or
  `?unknown?`.

What it deliberately is not: a task graph, supervisor, daemon, or database.
Gangline connects native agents; it does not manage them. Each agent keeps the
terminal, tools, permissions, and subscription it already had.

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

## How it fits

Gangline operates between native harness sessions. It does not replace
subagents a harness creates for itself: those remain owned by their parent
session. A Gangline team is a collection of independent top-level processes in
tmux windows, which can use Claude Code and Codex side by side.

Messages are typed into the recipient's own terminal and are reported as
delivered only after Gangline sees them land. When it cannot establish the
truth, it says so instead of guessing. Native dialogs remain native: Gangline
will not answer a permission or trust prompt for you.

## Documentation

- [Reference](docs/reference.md) — commands and configuration
- [Operations](docs/operations.md) — recovery and longer-running teams
- [Design](docs/design.md) — decisions that shape the project
- [Contributing](CONTRIBUTING.md) — local checks and commit conventions
- [Security](SECURITY.md) — private vulnerability reporting

Gangline is licensed under Apache-2.0.
