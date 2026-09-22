# AGENTS.md

Read [`docs/design.md`](docs/design.md) before changing anything and
[`CONTRIBUTING.md`](CONTRIBUTING.md) before committing.

## The running binary is not your source tree

`gang` is a compiled binary. Check which one is live with
`readlink -f "$(command -v gang)"`. Editing source changes nothing until a new
binary is installed, and never replace the binary a live team is using as part
of a test.

Collars, role briefs, the contract, and the model are all read when an agent
is hitched. A running agent keeps what it started with; drop and re-hitch it
to pick up changes.

## Tests

- Run the gate from `CONTRIBUTING.md` at each checkpoint.
- Mandatory tests have no sleeps, polling, or real timeouts. Use direct state,
  event barriers, or fake clocks. If the behavior under test is a timeout,
  scale the fake clock and write down the measured margin.
- A check that only prints on success proves nothing when it prints nothing.
  Keep "unknown" separate from pass and fail.
- Don't add test matrices or release blockers the operator didn't ask for.

## Don't touch the live team

- Test against a separately named throwaway session, never the live `gangline`
  session or your own agent. Delete only that session afterward.
- Inside an agent window `$TMUX` points at the live server, and so do `tmux`
  and `gang`. Run `unset TMUX TMUX_PANE` and confirm `tmux list-sessions` shows
  only your session before starting anything.
- `gang down` ends the whole team and `gang drop` ends one agent. Run
  `gang roster` first.
- Never run `tmux kill-server` or `tmux kill-session` without an exact target.
- Never kill a process just because it holds a lock. Confirm it's yours
  (`/proc/PID/cwd`, or a file only your tree has) in a separate command that
  finishes before the kill.

## Docs

| File | Holds |
|---|---|
| `README.md` | what Gangline is and why |
| `ARCHITECTURE.md` | packages and the message path |
| `docs/design.md` | principles and decisions |
| `docs/reference.md` | commands, environment, collar contract |
| `docs/operations.md` | running unattended and recovery |
| `CONTRIBUTING.md` | setup, the gate, commits |
| `CHANGELOG.md` | written by Release Please; never edit by hand |

Don't put counts, versions, or sizes in docs; point to the command that
measures them. `CLAUDE.md` just imports this file.
