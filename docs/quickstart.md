# Your first team

Ask a lead to find how to run a repository's tests. It will recruit a worker,
get the answer, and report back. You'll inspect their messages and shut down
the team when you're done.

## Install

Gangline runs on Linux and macOS. You need Git, tmux, and Go compatible with
the [`go.mod`](../go.mod) file. This walkthrough uses both Claude Code and
Codex; install their CLIs and run each directly in your repository to complete
login and trust prompts.

Install a stable Gangline release:

```sh
curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
```

The installer puts `gang` in `~/.local/bin`. If that directory is missing from
your `PATH`, add it to your shell startup file. For the current shell:

```sh
export PATH="$HOME/.local/bin:$PATH"
gang --version
gang collars
```

You should see a version and the `claude-code` and `codex` collar names. A
collar is a CUE file that tells Gangline how to launch and communicate with a
particular harness.

## Give the lead a task

From a shell outside tmux, in the repository you want to inspect, start a
named team:

```sh
export GANG_SESSION=first-team
gang up -c claude-code
```

`gang up` opens the `lead` window and attaches your terminal to tmux. Type this
request into the lead's Claude Code prompt:

> Find how to run this repository's tests. Hitch a Codex worker named scout
> to inspect the test setup without changing files. Ask it to send you the
> test command and the file paths that support its answer. Summarize the
> result for me, and leave scout running until I finish checking.

The lead can now hitch `scout`, send the assignment, and use its reply to
answer you. This is an instruction to the lead; its exact wording and tool
choices depend on the agent and your repository's policy.

## Watch the work

Use tmux's `Ctrl-b w` window chooser to visit `scout` and return to `lead`.
Either agent may ask you to approve a command; answer in that agent's window.
A window title marked `!name!` signals an agent that needs attention.
To inspect from your shell, detach with `Ctrl-b d` and run:

```sh
gang roster
gang capture scout
gang capture lead
```

The roster should list `lead` and `scout`. The worker's window shows its
inspection, and the lead's window shows the result when it arrives. If you
want to narrow the task, run `gang attach` and tell the lead:

> Include only commands documented in this repository. If none is documented,
> say what you found and leave the command unverified.

Detach again to check message delivery:

```sh
gang log --agent scout --type input_finished
gang log --agent lead --type input_finished
```

A record with `status` set to `delivered` confirms that a message was
submitted to that recipient. `accepted` means the native queue owns the
input; it does not confirm a reply. To read all events for the worker, use
`gang log --agent scout`.

## Check the result and stop

Read the lead's answer and open the files it names. You should be able to
connect its recommended command to the repository's test setup. That answer,
the named worker in the roster, and a delivery record verify the workflow.

From your shell, save the log if you want to keep it, then stop the team:

```sh
gang log > first-team-log.jsonl
gang roster
gang down first-team
gang teams
unset GANG_SESSION
```

`gang down` stops the registered agents and deletes the team's runtime state
and history. `first-team` should be absent from `gang teams`. For manual
staffing, messaging, and stopping a turn, continue with the
[coordination guide](guides.md#coordinate-agents).
