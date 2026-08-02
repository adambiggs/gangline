# Contributing

Read `CONSTITUTION.md` first. Those laws bind every change here, and violating one
is a defect — not a style disagreement.

## Setup

```sh
git config core.hooksPath .githooks
```

Enables the `commit-msg` gate and the `pre-push` gate. Neither is installed
automatically; a clone without this commits and pushes unchecked.

`pre-push` runs the gates that take seconds; `.githooks/pre-push` itself is
the authoritative list of what those are, and a gate added there needs no
edit here to be announced. It does not run the integration suite, so passing
it is not a prediction that CI will pass. `git push --no-verify` skips it.

## Public content

The no-PII policy in
[ADR-0013](docs/adr/0013-a-pii-control-is-a-gate-not-a-history-rewrite.md)
covers commits and pushes automatically, through `pre-push` and CI. It does
not cover issue and pull request bodies — those never become a commit — so
run

```sh
tools/pii-scan --stdin < body.txt
```

before `gh issue create` or `gh pr create`.

## Commits

Conventional Commits:

```
<type>[(scope)][!]: <description>
```

- **type** — `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`,
  `revert`, `style`, `test`
- **scope** — optional, lowercase; a command or component (`patrol`, `vet`,
  `send`, `compact`, `up`, `down`, `context`, `profiles`)
- **!** — breaking change; pair it with a `BREAKING CHANGE:` footer saying what
  callers must update

```
fix(patrol): gate injection on pane stability
feat(down): tear the whole team down
refactor!: rename executable and protocol from gl to gang
```

The `commit-msg` hook rejects anything else, and the `commits` workflow re-runs
that same hook over every pushed commit — one regex, so the two cannot drift. The
workflow also checks the pull request title. **Your PR title must be a
Conventional Commit too:** this repository squashes with `COMMIT_OR_PR_TITLE`, so
when a PR has multiple commits, its title is the subject that lands on `main`.

An active ruleset makes the `conventional` check required on `main` for outside
contributors. Repository admins bypass that ruleset in `always` mode by design;
the local hook remains their commit gate. The bypass is also what makes the
automated release PR mergeable: a PR opened by the workflow's `GITHUB_TOKEN`
does not trigger workflow runs, so its required `conventional` check can never
post.

Subjects git generates itself — merge commits, `fixup!`, `squash!`, `amend!` —
pass through, since rejecting them breaks merge and interactive rebase.
`git revert` is not exempt: `revert` is a type, so edit its generated subject like
any other.

Write the body for someone deciding whether to trust the change: what failed, what
the fix is, and what proved it. Scraped-marker changes say which harness version
they were verified against — that pin is what `gang vet` reads.

### Committing beside another agent

An exact pathspec protects you from a peer's files, not from a peer inside your
file. `git commit -- bin/gang` commits that file's *content*, so a teammate's
uncommitted work in it lands under your message and your authorship.

The pathspec also has a blind spot that appears exactly when the tree is dirty:
`git commit -- <path>` reaches only *tracked* files, so on a brand-new file it
fails with "did not match any file(s) known to git" — and the recovery that
error tempts you toward is `git add -A`, the precise move this section exists to
prevent. Stage by the same pathspec instead: `git add -- <paths>`, then commit.

So split concurrent work by **file**, not by concern: a boundary git cannot
enforce is not a boundary. One writer per file at a time, and the way to hand it
over is to land a green checkpoint and push, not to hold a large batch. A batch
held in the tree blocks the next writer twice — they cannot commit, and they
cannot use the suite as evidence either, because a green run exercises the
uncommitted work and a red one is attributed to whoever runs it.

It also runs. `gang` on your `PATH` is a symlink into a tree — `install.sh` points
it at its own clone, and `GANGLINE_HOME` points it at yours — so on a contributor
checkout an edit to `bin/gang` is deployed the moment it is saved: to every
attached agent, to the patrol cron, and to whatever measurement is running, with
no commit, no install step and no restart. That is the right behaviour for
dogfooding — a word `docs/field-guide.md` notes lands literally here — and it is
the reason to keep checkpoints small. An uncommitted batch is not a draft; it is
the substrate the team is presently running on.

A corollary for anything that records what the tool did: the epoch that matters is
when the executed code changed, which is the save, not the merge. Date a regime
change in collected data from the first invocation that could have loaded the new
code — and per leg, since a cron leg picks it up on its next tick while a hook leg
picks it up immediately.

## Releases

[release-please](https://github.com/googleapis/release-please) owns releases; no
repository script or hand-written changelog step does. Pushes to `main` maintain
one release PR from the Conventional Commits since the last tag. Merging
that PR creates the tag and GitHub Release and writes `CHANGELOG.md`.
**Do not edit `CHANGELOG.md` by hand or add changelog entries to ordinary PRs.**

The repository setting **Settings → Actions → General → Workflow permissions →
Allow GitHub Actions to create and approve pull requests** must be enabled. It is
not enabled automatically. Without it, release-please creates its branch and
release commit, then fails at PR creation, so no release PR is maintained.

`.release-please-manifest.json` is the version source. A release commit updates
that manifest, `version.txt`, `packaging/npm/package.json`, and
`packaging/pypi/pyproject.toml`; do not introduce another version location or
bump one of those files through a second mechanism. release-please computes the
next version from the manifest and the commit history; take the version it plans
from a dry run rather than from any number written down here.

The npm and PyPI stubs are not published packages yet; install through
`install.sh`, not a registry. After every release, verify both packaging stubs
moved with the manifest: the extra-file updaters can leave a missing or malformed
JSONPath unchanged while the release workflow remains green.

## Before adding code

- **Law 5** — nothing lands without a live consumer. If nothing invokes it the day
  it merges, it does not merge.
- **Law 4** — per-harness knowledge is a profile, not a code path. A harness-specific
  branch in `bin/gang` needs an ADR in `docs/adr/`.
- **Law 9** — when in doubt, the answer is prose in an agent's prompt, not code here.

When a surface's misuse cannot be detected from inside the tool, removing the
surface is the fix and documentation is not. A message body passed as an argument
has already been through the sender's shell before `gang` sees it, so backticks in
an agent's prose have run by then — there is no point at which Gangline could validate,
warn, or refuse. That is a different claim from "the safe path is nicer," which is
a judgement someone can reasonably weigh the other way; this one says no validator
is possible. Reach for it sparingly, and only with that argument in hand.

**A profile must never lower the operator's security posture.** No shipped profile
may put a harness into a mode that skips approval prompts or disables its sandbox —
`--dangerously-bypass-approvals-and-sandbox`, `approval_policy = "never"`,
`sandbox_mode = "danger-full-access"`, `--yolo`, or whatever the next harness calls
it. A profile describes how to *drive* a harness, not how much authority to hand it.

The reason it is tempting is exactly why it is banned: an agent stopped behind a
permission prompt is refused by `gang send` and no keystroke from a teammate can
clear it, so "just turn the prompts off" reads like a fix for a coordination
problem. It is not. It is a decision only the person at the keyboard can make, on
their own machine, in their own config — and shipping it as a default means every
future installer inherits a choice they never made and probably never read.

Put it in `~/.codex/config.toml` (or the harness's equivalent), never in
`profiles/`. If an occupied agent is blocking a team, that is a thing to tell the
operator, not a thing to engineer around.

Every shell and Python file carries an `SPDX-License-Identifier` line; new ones
need it too.

Shell changes must pass `test/lint.sh` — `bash -n` and `shellcheck -S warning`
over every shell file in the repo — and `test/integration.sh`; the `shell`
workflow runs both on every push. Run the lint through that script rather than
naming files by hand: a hand-picked set is a shorter list than CI's, and it goes
green over the file it left out. A local green run is not a CI prediction, because
the tools are not the same implementations: local `awk` is mawk where CI runs
gawk, and mawk's byte-wise `substr` hid a byte/character bug outright; the local
Bash is far newer than the one macOS CI ships, which rejects `[[ ]]` constructs
that are valid here; and the local ShellCheck is older and more permissive than
CI's, which read a comment beginning `shellcheck` as a directive and failed lint.
A class of failure that cannot reproduce on this box is the normal case, not a
surprise.

## Writing about Gangline

`gang` is the command and always appears in a code span. Everywhere else the tool
is **Gangline**. They are not synonyms: the command is a face over the substrate,
so naming the binary in a sentence that is about the tool makes a claim about a
command line the sentence never meant. Reach for `gang` when the subject is an
invocation, or where two binaries are being contrasted; reach for Gangline for
what the tool can establish, refuses to claim, or does over time.

Two lowercase uses are deliberate and must survive a sweep. "A gangline is the
line down the middle of a dog team" is the common noun the tool is named after,
and the demo's `aria-label` says "gang up" because an HTML attribute cannot hold
a code span.

The verb for adding an agent to a team is hitch, never hire. The vocabulary is
mushing throughout, and a dog is hitched to the line.

Sweep the stem, not the word. A grep for `hire` does not match `hiring` and one
for `gang` does not match `Gang's`, so a sweep that reads clean has cleared only
the exact form it searched for.

## Measuring things

Nearly every defect this repository has produced has one shape: **a result that
could not be determined, spent as though it had been.** What follows is that shape
meeting a different surface each time.

### Measure the thing, not a stand-in

Some agent harnesses wrap `grep` in a shell function for their own interactive use,
and a measurement taken at such a prompt reported that wrapper's exit codes as the
tool's — when every script here runs `/usr/bin/grep` and gets different ones. Run a
measurement the way the code runs it, in a script and through the same binary, or
the number belongs to the stand-in.

Every stand-in is cheaper to reach than the thing beneath it, and that is the whole
attraction: a wrapper is already on `PATH`, a synopsis sits at the top of the man
page, a flag's first appearance is one grep of a changelog, a summary of that
changelog is one call. All four have been measured here and reported as the thing
beneath them. Nor are a stand-in's errors scattered — that summarising fetch
returned the right changelog entries under three wrong version headings, every one
of them lower than the truth. **They lean permissive, so the cheap measurement is
likeliest to be wrong exactly where being wrong lets you proceed.**

### A conservative result is the one nobody audits

The opposite error has no such tell. Measuring whether old tmux delivers a paste
meant running old tmux, which came with old Bash — and readline only defaults
`enable-bracketed-paste` on from Bash 5.1, so the old image never requested mode
2004, tmux was right to send no brackets, and the payload executed. The measurement
said the enforced floor was real. It was measuring Bash. **A confound that produces
the conservative answer is invisible, because nothing downstream audits a result
that agrees with the status quo.** A permissive error gets caught when something
turns out easier than expected; a conservative one is simply believed. When a
measurement confirms what you already enforce, that is the moment to ask what else
moved.

So an instrument must control the receiver or it measures the receiver — and
**the control's own state has to be a required observable**, or you cannot tell
"controlled" from "control silently did nothing." The same confound reappeared one
version lower, on a tmux 2.1 image whose Bash predates `enable-bracketed-paste`
entirely: the control could not be applied, and the instrument returned another
clean, wrong, conservative verdict. What refused it was a second instrument reading
the bracket bytes on the wire, plus the rule that the control must report `on` or
the run is could-not-determine. Two instruments that must agree, with disagreement
counting as no verdict rather than as a tiebreak, is what makes this catchable at
all — a tiebreak picks the wrong one here.

### Some floors cannot be read at all

Whether text arrives at a TUI as a paste rather than as keystrokes is a property of
tmux, the terminal and the harness together, and no changelog entry quantifies over
that triple. Feature existence is a fact about one file; behaviour is a fact about a
system, and only running the system reports it. Where the two disagree, state the
requirement the installer enforces, and give it a reason that names its own evidence
rather than a version that cannot carry it.

### An instrument that has only ever reported one verdict is not yet one

A check that has always passed has never shown that it can fail, and its failure
path — the branch that matters — is an assumption wearing a test's clothes. Plant
controls that force each verdict out of it before trusting any. The tmux floor cell
ships with five runs behind it, four of them planted: a lowered floor that must move
both cells with no edit to the workflow, proving the number is read from
`install.sh` and not restated; a raised floor and a broken gate that must each come
back could-not-determine; and a deliberately crippled probe, the only run that ever
exercises the failing exit. Relatedly, when a race did appear it surfaced as
could-not-determine rather than as a wrong verdict — a check that degrades toward
"I do not know" under conditions it did not anticipate is working, and that is worth
designing for rather than discovering.

### Write a trap where its class lives

A trap documented only where you met it is a trap the next site will hit. Twice
already this repository held the answer and a new site walked in anyway: the unix
socket path-length limit, documented twenty lines into the test suite, and
command-substitution `die` semantics, documented at the one call site that
deliberately relies on them. Both times the knowledge was attached to the place it
was learned rather than to the class it belongs to. Write it where the class lives
— a rule the suite enforces, or a comment on the helper every caller passes
through. Local knowledge that does not generalise is a note to the person who
already knew.

### A fixture must answer for itself

Or its failure will be read as a defect in the thing under test. The tell is worth
memorising, because it is visible at the moment of confusion rather than in
hindsight: **an identical failure set across two
different subjects is a statement about the instrument, not the subjects.** A probe
run against several tmux versions reported the same four failures on each; all four
were the probe's own assertions, demanding output from panes running `sleep` and
expecting a print from a command that reports through its exit status. Genuine
version differences do not line up that neatly.

### Polarity decides how a broken predicate fails

And one of the three does not fail at all. A positive assertion goes red — loudly, but naming the behaviour under
test while the real cause sits elsewhere in the output: misattributed, not silent.
A negative assertion goes GREEN, because the value it expects is the same value a
predicate returns when it could not evaluate anything at all — and guards are
negative assertions, so the checks written to catch regressions are exactly the
ones that can stop looking without saying so. A poll answers "not yet" forever and
spends its whole timeout: a miss's clothes on a failure a minute away from its
cause. This is why such a predicate needs a third answer, distinct from both
verdicts, that NAMES what it could not determine — a bare non-zero would have left
the negative assertion passing.

### A guard you edited is a guard you removed

Tests that exist to hold a decision are the executable form of that decision, and a
change that makes them agree with itself has not passed them. The default context
band ladder was made absolute, with rung-boundary checks added so the old behaviour
"cannot come back"; three days later a change flipped the default to percentages and
rewrote those same checks to assert proportionality. Nothing went red. The
protection was not overridden — it was edited into agreeing, and 28 checks across
the surface ended up encoding the reverted decision.

So when a change requires editing an existing assertion, stop and find out what that
assertion was protecting. Adding cases is ordinary; **changing what an existing one
expects is a claim that the old expectation was wrong, and it needs the same
argument as changing the behaviour would.** Deleting one is that claim in its
strongest form.

The other half of that failure is where the decision lived: only in a commit
message. Git holds history, but a register is what a proposal searches, so a
decision reachable only by knowing which commit to read will be silently reopened by
someone who did not know. Anything that will be re-proposed — a default, a rejected
design, a floor — belongs in `docs/adr/`, and the code that implements it should
point there so the reasoning is reachable without a search.

### A changed default leaves claims no grep will find

Because docs also state defaults in words. Sweeping the numbers found five wrong
facts here and missed a sixth — a sentence asserting the ladder was "entirely
absolute" — caught only by reading the section around a number already being
fixed. Change a default, then read every section that describes it.

The test drives `bin/gang` against a real tmux server with the `bash` profile — no
mocks, because a mocked tmux would agree with whatever the code already does. A fix
to delivery or addressing belongs there as a case that fails without the fix.

## License

Contributions land under Apache-2.0, the same license the repo ships under:
section 5 puts anything you intentionally submit for inclusion under those terms
unless you say otherwise in the pull request. There is no CLA to sign.
