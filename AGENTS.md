# AGENTS.md

Read [`CONSTITUTION.md`](CONSTITUTION.md) before you change anything here, and
[`CONTRIBUTING.md`](CONTRIBUTING.md) before you commit. This page is the part
neither of them is: what a competent agent gets *wrong* in this repository, in
roughly the order it bites.

## `bin/gang` may be live under a running team

Before you edit it, find out what actually runs:

```sh
readlink -f "$(command -v gang)"
```

**If that resolves into this working tree**, the `gang` on your `PATH` is a
symlink to the file you are about to change. There is no daemon and no build
step — every invocation re-reads the script from disk — so the edit is deployed
the moment you save it: to every attached agent, to a patrol running from cron,
and to your own ability to send a message. A save that breaks the parse breaks
the tool you would have used to say so. Keep checkpoints small. An uncommitted
batch here is not a draft; it is the substrate the team is presently running on.

**If it resolves anywhere else** — the installer's own clone, or wherever
`GANGLINE_HOME` points — your checkout is inert and the installed copy is what
runs. Both are ordinary setups. Which one you are in changes what saving means,
so read the link rather than assuming either.

Then there is the half that gets skipped by anyone who over-learns the first.
Most of this repo is **not** live:

- A role brief is pointed at by path and read at hitch time. Editing
  `roles/worker.md` reaches the next agent hitched — the ones already running
  hold a snapshot in their context, and only a re-read replaces it.
- A crontab entry freezes the flags it was written with. Updating Gangline never
  rewrites one; `gang vet` reports the drift instead.
- Anything already inside a context window is a copy that no edit on disk can
  reach.

That asymmetry is why swapping an agent's model is not an edit at all. `-m` is a
launch flag and no verb moves a running agent to another model: wrap the work,
drop the agent, hitch a fresh one.

## A guard you edited is a guard you removed

The suite goes red. The smallest diff that turns it green is in the test file,
and it is right there. Take it and you have deleted a decision.
[ADR-0005](docs/adr/0005-context-bands-are-absolute.md) says it about one ladder
and means it generally:

> A change that edits those checks to agree with a new default has removed the
> guard rather than passed it.

Adding cases is ordinary. **Changing what an existing assertion expects is a
claim that the old expectation was wrong, and it needs the same argument that
changing the behaviour needs.** Deleting one is that claim at its strongest. So
when a change requires editing an assertion, stop and find out what it was
protecting: the reasoning is usually in `docs/adr/`, and the code implementing
it usually points there.

Nor is green evidence on its own. A guard is a negative assertion, and a
negative assertion passes when its predicate could not evaluate anything at all
— so the checks written to catch regressions are exactly the ones that can stop
looking without saying so. CONTRIBUTING's "Measuring things" is the page to read
before you claim anything you measured.

## The laws are aimed at you

[`CONSTITUTION.md`](CONSTITUTION.md) is short and binding; read it there rather
than here. What it does not say is that several of its laws exist because agents
break them by reflex.

- **Laws 1 and 4** — the reflex to special-case the harness in front of you.
  Per-harness knowledge is a file in `profiles/`, never a branch in `bin/gang`,
  and a branch needs an ADR proving no universal surface can carry the value.
- **Law 5** — the reflex to build the general version first. If nothing invokes
  it the day it merges, it does not merge.
- **Law 7** — the reflex to add a supervisor, a watchdog, a reconciliation loop.
  Gangline may not grow a component whose job is watching another Gangline
  component.
- **Law 8** — the reflex to add a graceful fallback. A regex that stops matching
  a new TUI must break the command, visibly; a degraded mode that reports
  healthy is the defect this repo exists to refuse.
- **[ADR-0012](docs/adr/0012-instale-data-is-refused-from-documentation.md)** —
  the reflex to write "28 checks" into a document, because a count reads as
  evidence of having looked. It is evidence of having looked *once*. Counts,
  versions, sizes and tallies are refused from documentation, history sections
  included. Point at the command that answers instead.

## Enable the hooks before your first commit

```sh
git config core.hooksPath .githooks
```

A clone commits and pushes unchecked until you run this. With it, a
non-conforming commit message is refused locally in a second; without it, the
same message fails in CI minutes later, after you have moved on.

## Proof must earn its cost

Take the shortest path from the requested result to the repository's existing
acceptance gates. Run those gates at a coherent checkpoint. If one reports a
concrete failure, fix that failure and rerun the relevant gate.

Before adding or changing a test, read CONTRIBUTING's "Test runtime is a
contract." The no-wait rule is enforced by `test/lint.sh`; moving a timed test
outside `test/` does not exempt it.

## Where the answer lives

Restating something in the wrong document is how this repo grows duplication,
and duplication costs more to remove once two pages disagree.

| Document | Holds |
|---|---|
| `README.md` | what Gangline is, and why it exists |
| `CONSTITUTION.md` | the laws — link to them, do not re-argue them |
| `docs/reference.md` | exact commands, options, environment, output |
| `docs/operations.md` | running a team unattended, and recovering one |
| `docs/field-guide.md` | the mushing vocabulary, translated |
| `roles/*.md` | what an agent must do |
| `docs/adr/` | why something was chosen, or refused |
| `CONTRIBUTING.md` | how to commit, release, and measure |
| `CHANGELOG.md` | what shipped — release-please owns it; never hand-edit |

## Agent configuration in this repo

`AGENTS.md` is canonical and serves every harness. `CLAUDE.md` is a shim that
imports it. Gangline's whole thesis is that a team is multi-harness, so a brief
only one harness could read would be an odd thing for it to ship.

`.claude/settings.json` is the exception, and law 5 is the reason: it exists
because this repo is contributed to from Claude Code today. Shipping the same
config in every harness's own dialect would be speculative generality, which the
law forbids —
so when a contributor arrives on another harness, that is when their config file
earns its place.

What it denies is deliberately narrow. An unaimed `tmux kill-server` or
`kill-session` destroys sessions it was never pointed at, including the
operator's own, and no contributor task requires it. A tmux of your own is not
that command: give it an explicit socket, and `tmux -L probe kill-server` is a
different string that no rule here matches.

Situational hazards stay out of that file on purpose, because a checked-in rule
cannot know them and a rule that is wrong for the next contributor is worse than
prose that is right for you. The live one is this: if a team is running on this
machine, `gang down` ends all of it and `gang drop` ends one agent — including
agents you did not hitch and work you cannot see. Both are legitimate
contributor commands, neither is denied, and `gang roster` is what you check
before reaching for either.
