# Reference

`gang --help` is the command inventory. `gang <command> --help` describes the
arguments, output, and failure cases for one command. This page explains the
small set of ideas needed to use that help.

## Requirements

Gangline supports macOS and Linux. It needs Bash, Python 3, tmux 3.2 or later,
and a supported native harness: Claude Code or Codex.

Run each harness directly once before starting a team. Its normal sign-in,
repository, and trust prompts belong to that harness; Gangline does not answer
them.

## Repository gate

`test/gate.sh` runs fast lint and smoke against the working tree, with one run
per host at a time and a 900-second run limit. Its final line says `PASS`,
`REFUSED`, or `UNKNOWN`. Fast lint checks files changed from `origin/main`; CI
runs the full lint and integration suites.

`test/release.sh` is the pre-release lane. It serializes on the same lock and
runs lint, smoke, and full integration.

## Start and inspect a team

Start in the repository where the agents should work:

```sh
gang up -c claude-code -m sonnet -e high
```

This opens a tmux session and attaches the current terminal to its first agent.
Use `Ctrl-b d` to detach. From a terminal outside the team, inspect it with:

```sh
gang roster
gang status lead
gang attach
```

`roster` is the overview, `status` gives one agent's current state, and
`attach` joins the team in tmux.

## Add agents and send work

Add a named native harness with `gang hitch`. The command's help lists the
available choices for its harness, working directory, model, effort, and role.

```sh
gang hitch worker -c codex -d "$PWD"
printf '%s\n' 'Inspect the failing parser tests and report the proof.' |
  gang send --to worker --from operator --stdin
```

Messages travel through the recipient's own terminal. Gangline reports a
message as delivered only after it sees the terminal accept it. A message that
cannot be delivered immediately is kept for a later safe opportunity; a refusal
leaves the sender responsible for it.

## Observe and control

| Need | Command |
| --- | --- |
| See every agent | `gang roster` |
| Inspect one agent | `gang status NAME` |
| Read a terminal | `gang capture NAME` |
| Stop a current turn | `gang interrupt NAME` |
| Ask an agent to compact context | `gang compact NAME` |
| Remove one agent | `gang drop NAME` |
| End a named team | `gang down SESSION` |

The state symbols are deliberately conservative:

| State | Meaning |
| --- | --- |
| `-busy-` | Gangline has evidence that the agent is working. |
| `~wait~` | The agent has work pending before its next turn. |
| `~idle~` | The agent is ready for input. |
| `!occupied!` | A native dialog owns its input. |
| `?unknown?` | Gangline cannot establish the state safely. |

Use `gang capture NAME` to see what a native dialog says. Answer a sign-in,
permission, or trust prompt in that agent's terminal; Gangline will not choose
for you.

## Exit status

An exit status of 0 means the requested operation happened. Status 3 is a
refusal: it did not happen, and stderr explains why. Status 1 is an error, so
read stderr before retrying; it does not prove that no change happened.

## Configuration

`gang config` shows the configuration effective for the current command. Keep
machine-specific settings in your environment or Gangline configuration, not in
an agent's prompt. See [Operations](operations.md) for context and recovery
guidance.
