# Design

Decisions that still shape the code, each with the incident, measured cost or
real alternative behind it, in the order they were made.

## Messages are attributed and delivery is verified

2026-08-04 · [#131](https://github.com/adambiggs/gangline/issues/131)

A message crosses from one harness's pane into another's composer, where being
typed, being accepted and being submitted are different facts. A sender that
cannot be seen can still claim any name: a harness's sandboxed command surface
strips the tmux environment, so its `--from` read exactly like a pane Gangline
had watched.

Every message names its sender and travels in a nonce-bound envelope. Gangline
reports delivery only after the target composer visibly accepted and submitted
it. It reads the sender off the calling window where it can see one and refuses
a claimed name there; a name it could not observe goes on the wire as
`self-declared:<name>`. Gangline is single-tenant and claims no authentication:
the marking is a label, and the contract tells receivers to treat a
self-declared sender as unverified. A delivery Gangline cannot verify exits 5
rather than being reported sent or retried.

## Configuration is parsed, never sourced

2026-08-07 · [89cc406](https://github.com/adambiggs/gangline/commit/89cc406)

Operator choices need a durable file. Sourcing a shell file would execute
operator text on every command and every native hook, and silently ignoring a
typo would claim a setting Gangline did not apply. The config file therefore
mirrors the environment names in strict scalar lines. Gangline parses it,
refuses unknown or duplicated keys, and lets a set environment variable win. A
config file cannot run code, and a misspelt key fails loudly at the next
command instead of being ignored.

## The irreversible verb is the one that demands an argument

2026-08-11 · [#72](https://github.com/adambiggs/gangline/issues/72),
[791d28f](https://github.com/adambiggs/gangline/commit/791d28f)

Every argument-taking command answered a bare invocation with its usage; `gang
down` did not, so running it bare to see what it wanted ended the team, and
`gang down lead` ignored its extra argument and ran a full teardown. `gang down`
now requires the exact session it ends and refuses to run from inside that
session. Ending a team always takes a deliberate, named argument; the cost is
typing the session name, which `gang teams` prints.

## Teardown archives mail before deleting its spool

2026-08-11 · [9d73d6e](https://github.com/adambiggs/gangline/commit/9d73d6e)

A message parked for an agent is the only copy of what its sender said.
Teardown used to unlink the spool with it, and a `gang mail | tail` destroyed
the head of a lead's directive with nothing to recover it from.

`gang drop` and `gang down` move every waiting or held entry into a
human-readable archive before deleting the spool, and refuse to end anything if
that archive cannot be written. `gang mail` moves each entry into the same
archive before printing it, so a shell filter can hide what it printed but
cannot erase the only copy. Archives are recovery state: Gangline never deletes
them on its own, and the read prints the command that does.

## Native continuation owns compaction recovery

2026-08-11 · [f13e296](https://github.com/adambiggs/gangline/commit/f13e296)

A compaction left an agent at an empty composer holding only its summary. With
nothing to take the next turn, the agent sat idle and its work silently
stopped. Every compaction Gangline submits is now followed by a continuation
turn that tells the agent to re-read its brief and saved state, and `gang
compact --resume` replaces that turn's text. No compaction lands idle; what the
continuation asks the agent to re-read is operator prose, not Gangline state.

## Contribution safety scanning lives outside the repository

2026-08-12 · [fee26a8](https://github.com/adambiggs/gangline/commit/fee26a8)

One pre-push gate scans every repository's pushed tree for personal data. A
second scanner copy inside Gangline is a second thing to keep correct, and the
copy that lags is the one that reports clean.

Gangline carries no scanner, scanning CI or scanning tests. Its pre-push hook
delegates to the executable global hook first and then runs its own lint and
commit checks; an absent global hook is a no-op. A clone without the operator's
gate pushes unscanned, and public CI runs no PII scan; that gap is accepted
rather than closed with a vendored copy. The outer gate can spend minutes in
inference, so it writes straight to the terminal rather than being captured and
replayed, and its progress shows instead of reading as a hang.

## The contract rides the system prompt where a collar has one

2026-08-12 · [eac1447](https://github.com/adambiggs/gangline/commit/eac1447)

A contract typed into a pane is lost to the next compaction, and a paste is
bounded by what the composer renders and by pane geometry. A byte cap on the
contract admitted a body the composer could not render while refusing prose a
system prompt could carry.

The standing terms live in `CONTRACT.md`, resolved operator-first and validated
before a window opens; a missing or unreadable contract refuses the hitch. Where
a collar declares `GANG_ROLE_PROMPT_OPT`, the contract and role brief go into
the harness's system prompt; otherwise the startup message points the agent at
the file. Doctrine is pasted. The terms survive compaction without a re-read
wherever the harness has the option. A running agent keeps the contract it
launched with, so an edit reaches live agents only through a re-hitch or, for
pointer collars, the next read of the file.

## Occupancy is not authority

2026-08-13 · [6683d92](https://github.com/adambiggs/gangline/commit/6683d92)

Through 1.x, Gangline recognised each harness dialog by a per-build fingerprint
and drove keys to dismiss some of them. It bought dismissing one Codex wait
screen and repeating a trust choice `hitch -d` had already made, and cost the
most version-fragile TUI machinery in core: strings that rot into a silent
fallback while everyone believes coverage holds.

When a harness-owned UI occupies the composer, ordinary input is refused and
Gangline does not decide who may clear it. Collars recognise occupancy with
`GANG_OCCUPIED_REGEX`; unknown authority fails closed, and an occupied composer
is answered by a person through `gang attach`. 2.0 removed the fingerprint
registry, its per-dialog key driving and both collars' records.

## The mandatory gate fits under a memory ceiling

2026-08-13 · [77b3a22](https://github.com/adambiggs/gangline/commit/77b3a22)

One `shellcheck` over the whole repository reached 6.1 GB. On 2026-08-12 the OOM
killer took it twice on an 11.6 GB host, which was power-cycled hours later
after a starvation livelock. A gate every agent must run cannot be the largest
allocation on the machine, so it must pass under

```sh
systemd-run --user --scope -p MemoryMax=2G -p MemorySwapMax=0 -- test/gate.sh
```

`test/lint.sh` runs one `shellcheck` per file, and a test file that grows until
it alone will not fit is split into sourced parts. Sourced parts keep the suite
one program with one set of fixtures and counters. shellcheck cannot see across
a part boundary, so a variable that crosses one carries a directive naming the
file at the other end.

## A dead Claude stream is resumed from two native witnesses

2026-08-15 · [7e9c2be](https://github.com/adambiggs/gangline/commit/7e9c2be)

Claude Code emits no Stop when a provider stream dies. The turn just ends, an
agent waiting on it looks idle, and the error prose varies between releases.

Gangline requires two native witnesses: the later `idle_prompt` notification,
which proves the harness is waiting and binds the transcript path, and the
newest top-level assistant record carrying `error`, `isApiErrorMessage` and a
UUID. Under `GANG_AUTO_RESUME` it closes the turn and submits one attributed
continuation per error UUID; a failure of that continuation gets no second
hop. The dead turn is told from ordinary idleness by record structure, not by
its sentence. When ownership of the turn cannot be proved Gangline refuses and
records the refusal for status and roster. Collars without such a record
declare no equivalent.

## One agent per killable cgroup, or one kill ends the team

2026-08-17 · [461ba29](https://github.com/adambiggs/gangline/commit/461ba29)

A tmux server inherits the cgroup of whatever started it, so every agent on a
team lived in the login session's scope. `systemd-oomd` kills the leaf cgroup
holding the most swap, and a session full of dormant agents is by construction
that leaf: on 2026-08-16 one kill ended a whole team at once.

With `GANG_SCOPE=on`, each hitched launch runs in its own transient systemd user
scope, so each agent is its own leaf and is named in the kill message. Where a
scope cannot be created the hitch is refused rather than run unscoped. Losing
one named agent replaces losing the team. Scoped agents also fall under the user
manager's memory-pressure policy, which can take one agent for pressure it did
not cause; the thresholds remain the operator's.

## Mandatory tests are immediate

2026-08-21 · [#114](https://github.com/adambiggs/gangline/issues/114)

The integration suite stubbed `sleep` on PATH, so liveness and timeout-budget
defects structurally could not fail it, and wall-clock waits went flaky under
team load. Mandatory tests do not sleep, poll, or use wall-clock delay as
evidence. They assert state the command has already established, through
immediate reads, event barriers or fake clocks, and `test/lint.sh` enforces the
ban across `test/`. Where the behaviour under test is a timeout, a fake clock
may be scaled rather than stopped, and the fixture records its measured margin.
Real harness turns run only in the opt-in e2e lane.

## A mandatory barrier stays inside the wait ceiling

2026-08-30 · [0875226](https://github.com/adambiggs/gangline/commit/0875226)

`tmux wait-for` has no timeout. A barrier nobody answers parks a run forever,
prints nothing, and holds the gate's host lock for everyone else: two such
wedges held it for 25 minutes and for 3h56m. The suite puts a tmux shim at the
front of its PATH that cuts off a blocking wait at a ceiling and names it, and
`test/lint.sh` refuses a blocking `wait-for` issued through `REAL_TMUX` or an
absolute tmux path, which would bypass the shim. A wedged barrier is a named
failure, not a silent hold. A `-S` signal blocks on nothing and is left alone.

## Teardown authority follows the tmux server actually reached

2026-08-31 · [#187](https://github.com/adambiggs/gangline/issues/187)

Inside a pane `$TMUX` outranks `TMUX_TMPDIR`, and tmux 3.2a silently ignores a
`TMUX_TMPDIR` whose directory is absent and falls back to the default socket. A
kill aimed at a sandbox reached the live server and ended a 13-agent team. A
team on a private socket also looked ended from any shell that had lost that
environment.

The tmux guard asks tmux which server an invocation would reach and authorizes
teardown only from that server's live `@gl_agent` registrations; caller records
may corroborate but never authorize. An absent `TMUX_TMPDIR` refuses every
unaimed tmux command. `hitch` records the team's socket so `gang teams` and
`gang attach` can find it, and they ask the server rather than believe the
record. Unreadable registrations refuse teardown. The guard is a guardrail, not
a boundary: one variable still runs the command anyway.

## Every option reader reports failure instead of clearing or omitting state

2026-09-01 · [b6b2950](https://github.com/adambiggs/gangline/commit/b6b2950)

Window-option readers collapsed a failed read into an empty value. An unreadable
option was reported as unset, an unreadable self-compaction request retired a
failure that was still standing, and a caller reading through a pipe kept
grep's status, so an unreadable record left a tick pass silent and green.

Every window-option read failure propagates to its caller and the unread state
is preserved; no reader clears it, omits it, or renders it as absent. Status and
roster show an unreadable value as unknown rather than inventing one. A reader
that grows a failure status means sweeping its call sites for pipes and command
substitutions, since neither trips `set -e`.

## A scoped hitch has an immutable cgroup identity

2026-09-05 · [b877b49](https://github.com/adambiggs/gangline/commit/b877b49)

`gang rename` changes an agent's registered name without restarting its pane.
Scope units named after that name stayed active after the name was freed, so the
next hitch of the name was refused by a unit the registry said was gone.

Each scoped hitch mints an immutable 16-hex-digit identity and launches in
`gangline-<session>-<hitch-name>-<hitch-id>.scope`, recorded in the window's
`@gl_scope`. Rename leaves the unit alone, so a replacement can reuse a
registered name. Moving a live process into a newly named scope was rejected: it
turns a metadata rename into a second platform mutation whose partial failure
would leave registration and cgroup contradicting each other.

## A refused delivery is parked, a failed one is not

2026-09-06 · [62f9c7f](https://github.com/adambiggs/gangline/commit/62f9c7f)

A refusal happens before any keystroke, so the body is still the sender's. A
failure after a paste has an unknown fate, and a second copy of a message that
may have landed is worse than one loud failure.

A refused message is parked in the target's spool by default; `--live-only`
refuses instead. Entries are claimed out of the spool before delivery. An entry
whose delivery could not be verified, or whose drain died mid-flight, is held
and counted, never re-sent. A window's spool identity is minted at hitch and
adopt, where nothing can race it. Gangline never sends a message twice on the
chance the first did not arrive, and never holds one without naming it. A
boundary with no readable composer records a drain failure and leaves every
entry unclaimed.

## Release Please holds no tag-creation override

2026-09-08 · [7d16573](https://github.com/adambiggs/gangline/commit/7d16573)

GitHub's Create a Reference call refuses an Actions `GITHUB_TOKEN` for a tag
whose target's `.github/workflows/` differs from the default branch, and refuses
before checking whether the ref exists. A tag-first ordering therefore failed
with `Resource not accessible by integration` on 2.11.1.

Release Please issues no separate tag-creation call; a release is tagged by its
Create a release call, as every release through 2.11.0 was. A release commit
whose workflow files have fallen behind `main` publishes only once its tag
exists, and that tag is pushed from outside Actions under a credential holding
the workflow scope. Tag name and target stay Release Please's.
