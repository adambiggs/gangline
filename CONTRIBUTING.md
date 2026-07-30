# Contributing

Read `CONSTITUTION.md` first. Those laws bind every change here, and violating one
is a defect — not a style disagreement.

## Setup

```sh
git config core.hooksPath .githooks
```

Enables the `commit-msg` gate. It is not installed automatically; a clone without
it commits unchecked.

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
dogfooding, and it is the reason to keep checkpoints small. An uncommitted batch
is not a draft; it is the substrate the team is presently running on.

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
bump one of those files through a second mechanism. The initial manifest is
anchored at the unpublished `0.0.1`, and the verified release-please dry run
plans the first release as `0.1.0`.

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
an agent's prose have run by then — there is no point at which gang could validate,
warn, or refuse. That is a different claim from "the safe path is nicer," which is
a judgement someone can reasonably weigh the other way; this one says no validator
is possible. Reach for it sparingly, and only with that argument in hand.

**A profile must never lower the operator's security posture.** No shipped profile
may put a harness into a mode that skips approval prompts or disables its sandbox —
`--dangerously-bypass-approvals-and-sandbox`, `approval_policy = "never"`,
`sandbox_mode = "danger-full-access"`, `--yolo`, or whatever the next harness calls
it. A profile describes how to *drive* a harness, not how much authority to hand it.

The reason it is tempting is exactly why it is banned: an agent gated behind a
permission prompt is refused by `gang send` and no keystroke from a teammate can
clear it, so "just turn the prompts off" reads like a fix for a coordination
problem. It is not. It is a decision only the person at the keyboard can make, on
their own machine, in their own config — and shipping it as a default means every
future installer inherits a choice they never made and probably never read.

Put it in `~/.codex/config.toml` (or the harness's equivalent), never in
`profiles/`. If a gated agent is blocking a team, that is a thing to tell the
operator, not a thing to engineer around.

Every shell and Python file carries an `SPDX-License-Identifier` line; new ones
need it too.

Shell changes must pass `bash -n`, `shellcheck -S warning`, and
`test/integration.sh`; the `shell` workflow runs all three on every push. A local
green run is not a CI prediction on the current development box: local mawk
1.3.4 is more permissive than CI's gawk and hid a `substr` byte/character bug;
local Bash 5.1 is newer than macOS CI's 3.2, which rejected 37 locally valid test
constructs; and local ShellCheck 0.8.0 is more permissive than CI's newer parser,
which treated a comment beginning `shellcheck` as a directive and failed lint.

Measure with the thing under test, not a stand-in for it. Some agent harnesses
wrap `grep` in a shell function for their own interactive use; a measurement taken
at such a prompt reported that wrapper's exit codes as the tool's, when every
script here runs `/usr/bin/grep` and gets different ones. Run a measurement the way
the code runs it — in a script, through the same binary — or the number belongs to
the stand-in.

Every stand-in is cheaper to reach than the thing beneath it, and that is what
makes it attractive. A wrapper is already on `PATH`; a synopsis sits at the top of
the man page; a flag's first appearance is one grep of a changelog; a summary of
that changelog is one call. Four times here the cheap layer was measured and
reported as the expensive one: an agent's `grep` wrapper for the binary, a
man-page synopsis for the getopt string it describes, the release that added
`paste-buffer -p` for the release where pasting behaved, and a summarising fetch
for the changelog itself — which returned the right entries under three wrong
version headings, every one of them lower than the truth. That last part is the
warning worth keeping: a stand-in's errors are not scattered. They lean toward the
permissive answer, so the cheap measurement is likeliest to be wrong exactly where
being wrong lets you proceed.

The opposite error is quieter and has no such tell. Measuring whether old tmux
delivers a paste correctly meant running old tmux, which came with old Bash — and
readline only defaults `enable-bracketed-paste` on from Bash 5.1, so the old image
never requested mode 2004, tmux was right to send no brackets, and the payload
executed. The measurement said the floor was real. It was measuring Bash. Two
variables had moved together and the confound pointed the safe way, which is
exactly why it nearly stood: **a confound that produces the conservative answer is
invisible, because nothing downstream audits a result that agrees with the status
quo.** A permissive error gets caught when something turns out easier than
expected; a conservative one is simply believed. So when a measurement confirms the
thing you already enforce, that is the moment to ask what else moved — and an
instrument must control the receiver, or it measures the receiver.

Some floors cannot be established by reading at all. Whether text arrives at a TUI
as a paste rather than as keystrokes is a property of tmux, the terminal and the
harness together, and no changelog entry quantifies over that triple. Feature
existence is a fact about one file; behaviour is a fact about a system, and only
running the system reports it. Where the two disagree, the requirement to state is
the one the installer enforces, and its stated reason should say so rather than
cite a version that cannot carry it.

**An instrument that has only ever reported one verdict is not yet an instrument.**
A check that has always passed has never shown that it can fail, and its failure
path — the branch that matters — is an assumption wearing a test's clothes. Plant
controls that force each verdict out of it before trusting any of them. The tmux
floor cell ships with five runs behind it, four of them planted: a lowered floor
that must move both cells down with no edit to the workflow, proving the number is
read from `install.sh` and not restated; a raised floor and a broken gate that must
each come back could-not-determine; and a deliberately crippled probe, which is the
only run that ever exercises the failing exit. Relatedly, when a race did appear it
surfaced as could-not-determine rather than as a wrong verdict — a check that
degrades toward "I do not know" under conditions it did not anticipate is working,
and that is worth designing for rather than discovering.

A trap documented only where you met it is a trap the next site will hit. Twice
already this repository held the answer and a new site walked in anyway: the unix
socket path-length limit, documented twenty lines into the test suite, and
command-substitution `die` semantics, documented at the one call site that
deliberately relies on them. Both times the knowledge was attached to the place it
was learned rather than to the class it belongs to. Write it where the class lives
— a rule the suite enforces, or a comment on the helper every caller passes
through. Local knowledge that does not generalise is a note to the person who
already knew.

A fixture must answer for itself, or its failure will be read as a defect in the
thing under test. The tell is worth memorising, because it is visible at the moment
of confusion rather than in hindsight: **an identical failure set across two
different subjects is a statement about the instrument, not the subjects.** A probe
run against several tmux versions reported the same four failures on each; all four
were the probe's own assertions, demanding output from panes running `sleep` and
expecting a print from a command that reports through its exit status. Genuine
version differences do not line up that neatly.

Polarity decides how a broken predicate fails, and one of the three does not fail
at all. A positive assertion goes red — loudly, but naming the behaviour under
test while the real cause sits elsewhere in the output: misattributed, not silent.
A negative assertion goes GREEN, because the value it expects is the same value a
predicate returns when it could not evaluate anything at all — and guards are
negative assertions, so the checks written to catch regressions are exactly the
ones that can stop looking without saying so. A poll answers "not yet" forever and
spends its whole timeout: a miss's clothes on a failure a minute away from its
cause. This is why such a predicate needs a third answer, distinct from both
verdicts, that NAMES what it could not determine — a bare non-zero would have left
the negative assertion passing.

A changed default leaves stale claims that a grep for the old value will not find,
because docs also state defaults in words. Sweeping the numbers found five wrong
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
