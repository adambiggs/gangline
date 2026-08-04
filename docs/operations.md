# Operating a team

This guide covers the setup and failure paths that matter once `gang up` works.
For syntax, see the [command reference](reference.md).

## Permission prompts

Gangline launches each harness with the profile's `GANG_LAUNCH`. It does not add
a general permission bypass and never answers a modal. A prompt removes or takes
over the composer, so Gangline reports the agent as `occupied (authority unknown)`
and refuses all delivery until the operator clears it with `gang attach`.

A refusal is invisible to everyone but the sender, so an occupied agent also
carries what its occupancy is costing. `gang status` on it reports `INBOUND
REFUSED while occupied` with an attempt count and the most recent sender.
Refused, never queued: Gangline did not take the body and holds no copy, and the
sender remains responsible for its message. The record retires on the next
verified delivery to that agent, or with the window — clearing the modal alone
does not retire it, because the question it answers is whether traffic is getting
through.

Choose the unattended permission posture in the harness's persistent
configuration, where it remains visible and applies consistently:

- **Claude Code:** set the desired `permissions.defaultMode` in
  `~/.claude/settings.json`. The shipped profile adds no permission flag.
- **Codex:** set `approval_policy` in `~/.codex/config.toml`. The shipped profile
  disables only the startup update prompt for its process; it does not change
  approvals or sandbox mode.
- **opencode:** vanilla opencode allows tools unless your `opencode.json` has a
  permission rule set to `ask`. The shipped profile does not add `--auto`.
- **Pi:** core Pi has no tool-approval system. An extension can add one; if it
  changes modal chrome, shadow `pi.sh` and pin the extension's version as part of
  `GANG_VERSION_CMD`.

There is one narrow profile-owned grant. A role brief lives outside the project
when Gangline is installed elsewhere, and opencode otherwise asks before reading
that external directory. The opencode profile merges allow rules for the active
`GANG_ROLES` and shipped `roles/` directories into `OPENCODE_CONFIG_CONTENT` for
the launched process. It does not replace your other opencode configuration.

A role hitch checks for an occupied input box shortly after delivering the brief
and exits nonzero if one appears. This is a race that catches an early prompt, not
proof that no later one can occur.

## Question modals

A permission prompt and a question modal look alike on a terminal and are not the
same decision. A permission prompt asks for authority, which is the operator's to
grant. A question modal is the agent escalating a decision to whoever drives it —
in a Gangline team, usually the lead, sometimes by name in the question text.
Gangline refuses both, because no shipped profile's `GANG_OCCUPIED_REGEX` can tell
them apart on a screen, and a classifier that guessed wrong in the permissive
direction would let a peer grant authority nobody granted.

Configure the harness so the second kind does not arise. This is better than any
answer Gangline could give: the modal is not intercepted, it is never raised.

- **Claude Code:** deny the tool by bare name in `~/.claude/settings.json` —
  `"permissions": {"deny": ["AskUserQuestion"]}`. A bare tool name removes the
  tool from the model's context entirely rather than blocking calls to it, so the
  agent cannot raise the picker. `"permissions": {"defaultMode": "dontAsk"}` is
  the blunter form: it denies `AskUserQuestion` even where an allow rule names it.
  An agent that cannot ask its own UI escalates through Gangline instead, which is
  the routing you wanted.
- **Codex:** the equivalent tool is `request_user_input`, carried in the installed
  binary as `ToolRequestUserInputQuestion` — "one request_user_input question and
  its required options", the same header/options/other shape. Configure it as a
  nested table in `~/.codex/config.toml`:

  ```toml
  [tools.experimental_request_user_input]
  enabled = false
  ```

  A bare `experimental_request_user_input = false` under `[tools]` is not an
  equivalent spelling: Codex expects a structured value there and refuses to
  start. MCP servers can raise their own elicitation modals on Codex, and no tool
  setting reaches those.
- **opencode, Pi:** their modals are approval-shaped rather than a distinct
  question tool, so the permission posture above is the whole setting. Verify
  before assuming a harness has a separate question surface — Codex was assumed
  not to have one until somebody looked.

`gang vet` deliberately does not check for this the way it checks the Claude Code
context beacon. The beacon has one right answer on a host; this does not. User
scope is read by the operator's own adopted window as well as by every unattended
agent, so denying the tool there strips the picker from the sessions somebody is
sitting in front of. Which way that should go is the operator's call, and a vet
row that stated one would be advice that is wrong half the time.

This narrows how often occupancy fires; it does not retire the state. Tool
permissions do not reach harness chrome — a model picker, a plan-mode approval, or
whatever the next release paints are not tool calls and cannot be denied. So
`occupied` stays fail-closed, and Gangline ships no path that clears it: `gang
attach` and the keyboard are the whole remedy.

## Codex must be able to reach tmux

Gangline's control path is the tmux Unix socket. Under Codex's
`workspace-write` sandbox, network denial also denies `connect()` to Unix
sockets. Incoming messages can still arrive because an outside Gangline process
writes into the pane, but the Codex agent cannot run `gang send`, `roster`,
`status`, or a self-issued `gang compact` back through that socket.

Keep the filesystem sandbox while allowing the socket by choosing this in
`~/.codex/config.toml`:

```toml
sandbox_mode = "workspace-write"

[sandbox_workspace_write]
network_access = true
```

This enables ordinary network access too; Codex provides no Unix-socket-only
allowlist. Decide whether that trade is acceptable for the work.

Delivery also needs one lock directory shared by every Gangline process that can
write to a pane. It defaults to `/tmp/gangline-<uid>`, which every harness can
reach — including a `workspace-write` Codex sandbox — and which does not depend on
a login session having set anything up. The locks are empty between deliveries, and
the directory dies at reboot or on the distribution's `tmpfiles` sweep rather than
at logout.

Point `GANG_LOCK_DIR` somewhere else only if you preserve it in **every** shell and
cron job that runs `gang`. Writers with different lock directories stop serialising
against one another, so Gangline fails loudly rather than falling back to a second
path when the configured one cannot be used.

Because `/tmp` is shared, Gangline establishes that directory before trusting it:
it creates the root with mode `0700`, and refuses a path that is a symlink, is not
a directory, or is not owned by you. A root of your own that an older version left
more permissive is tightened instead of refused, so an upgrade needs no
intervention. Every refusal names what is wrong with the path, and none of them
quietly picks a different one.

## Codex hooks need a one-time trust decision

The Codex profile declares Gangline's hook commands at launch. The first time a
Codex dog is hitched after those commands are new or have changed, Codex stops the
hitch on its own trust gate before the agent can be briefed:

```
Hooks need review
<n> hooks are new or changed.
Hooks can run outside the sandbox after you trust them.

› 1. Review hooks
  2. Trust all and continue
  3. Continue without trusting (hooks won't run)
```

Answer **2, "Trust all and continue."** Gangline never ships a flag that skips this
check, so persisting trust by hand is the step — once, by you, rather than by a
profile that lowers your posture on your behalf. What the dialog counts is whatever
`profiles/codex.sh` currently wires; that file is the list of record and this page
deliberately does not restate it.

`gang hitch` cannot answer the gate for you and does not try. It refuses to paste a
brief past a screen that is not an input box, and says so:

```
'<name>' is up but is showing something other than its input box, so its hitch
message was not delivered — most likely a first-run prompt waiting on you. Answer
it with 'gang attach', then 'gang drop <name>' and hitch again.
```

That message is the tool working. Attach with `gang attach`, or aim at the window
directly with `tmux attach -t <session>:<name>`, answer the gate, detach, then drop
the stuck window and hitch again.

Trust is keyed to the hook command set rather than to the seat, which is what makes
this setup rather than a tax: a second hitch under a name never used before comes up
with no gate at all. Answer it once per machine per command set. Change what the
profile wires and the gate returns for the new set, exactly as it did the first
time.

Declining is honest and survivable. Option 3 says outright that the hooks will not
run, and Gangline declares no fact from them today, so nothing reports as working
that is not — the profile's own `GANG_PROBE_FACTS` note explains why that is
deliberate. What you lose is whatever is later built on those hooks, until trust is
granted. What happens if trust is revoked afterwards, or if a Codex release changes
how it stores or checks the hash, has not been established here and is not assumed
in either direction.

Gangline's hooks are additive. They are declared at launch rather than written to a
file, so an existing `~/.codex/hooks.json` — yours, or another tool's — is never
rewritten and keeps firing alongside them.

One gate is easy to mistake for this one. A working directory Codex has not seen
before raises its own directory-trust prompt, and it raises it first, before the
hook gate can appear. Same shape, same answer, different gate: expect it once per
new directory, separately from the once-per-machine hook step.

## A Codex seat needs explicit writable roots

Under `workspace-write`, a Codex agent that can edit a repository still cannot
commit in it. Git writes through `.git`, which the sandbox protects even when the
working tree around it is writable, so `git add` fails on a read-only
`.git/index.lock` rather than on anything the agent did wrong.

Name the paths in `~/.codex/config.toml`:

```toml
[sandbox_workspace_write]
writable_roots = [
  "<path to your repo>/.git",
  "<your home>/.local/state/gangline",
]
```

`writable_roots` takes a sequence; a boolean there is refused. The entries are
literal paths — nothing expands `~` or a variable for you, so write them out. The `.git` grant is
per repository and does not inherit — granting a parent directory that contains
several checkouts leaves each protected `.git` beneath it read-only, so every
repository a Codex dog must commit in needs its own entry. Gangline's own state
directory belongs in the list for the same reason: without it a Codex seat runs
commands that appear to succeed while failing to record anything.

## Shell-safe messages

`gang send` accepts message bodies only on stdin. Inline prose in argv is
refused: the sender's shell expands it **before Gangline starts**, so no
validator inside Gangline could detect that backticks or `$()` ran commands,
variables expanded, or globs became filenames. Delivery verification could
only prove that the already-altered text landed.

The shell still constructs the file or pipe that becomes stdin. Use a
single-quoted heredoc for literal prose, especially prose about code; an
unquoted heredoc performs the same expansions the stdin interface is designed
to avoid:

```sh
gang send worker --from lead --stdin <<'MESSAGE'
Review `bin/gang`; do not run it.
Treat $HOME and $(hostname) as examples, not expressions.
MESSAGE
```

For a body already held in a variable, quoted expansion into a pipe does not
re-evaluate the variable's contents:

```sh
printf '%s' "$body" | gang send worker --from lead --stdin
```

Empty stdin is refused. The legacy positional body form fails with a runnable
replacement naming the real target and sender.

`gang send --wait` is also deliberately bounded. Its timeout defaults to 300
seconds; if the target never reads idle, the command fails **without delivering
anything**. Use `--wait` only when blocking and possible non-delivery are the
intended behavior, and set `--timeout` explicitly when 300 seconds is wrong:

```sh
gang send worker --from lead --wait --timeout 3000 --stdin <<'MESSAGE'
report when the long run reaches a checkpoint
MESSAGE
```

## Context readouts and warnings

Every context consumer — `gang context`, the roster column, patrol, compaction
proof, and the in-turn band note `gang hook` returns — reads the same tier
order: the window's owned context fact where the harness's own wiring writes
one, then the active profile's `profile_context` scrape. Whichever tier
answers, the readout comes out in one shape, so nothing downstream can tell
which witness spoke.

The shipped profiles obtain the value differently:

- **Claude Code:** `statusline/claude-code-context.sh` formats the harness's own
  statusline payload into a `ctx <used>k/<window>k <percent>%` pane beacon and
  writes the same figures to the owned context fact in the same breath. A fresh
  fact is the readout; the beacon is the scrape it falls back to. Gang wires that
  script into the launch line of every window it hitches, so nothing has to be set
  up for a hitched agent. A window `gang adopt` registered was already running and
  carries none of that, so it paints a beacon only where the operator wired the
  script into `settings.json` themselves, as shown in the README.
- **Codex:** a hitch-time random marker links the window to exactly one rollout
  JSONL file. The profile reads its last `token_count` event. An adopted Codex
  window has no marker and cannot provide context.
- **opencode:** the pane provides used tokens, rounded percent, and model badge;
  the profile joins the window size from opencode's models cache and requires the
  percent to agree with the join.
- **Pi:** the profile reads Pi's native `<percent>%/<window>k` status-bar value.

A missing readout is not hidden: the dedicated command fails, roster displays
`-`, and patrol reports the agent as not patrolled.

### Ambient patrol

`gang patrol` is a one-shot sweep, and `gang cron` derives the crontab entry that
runs it every two minutes:

```sh
gang cron             # print the entry for this install
gang cron --install   # write it, replacing an existing entry for this session
```

The entry names an absolute `gang` — the one on your PATH when it belongs to this
install, since an update repoints that symlink while a crontab entry stays where
it was written, and this tree's own `bin/gang` otherwise. It carries every
`GANG_*` override exported where you ran the command, plus `TMUX_TMPDIR` and
`XDG_STATE_HOME` when they are set: the first is how tmux finds its socket, and a
sweep that cannot reach the server fails the same way a missing entry does.
Defaults are never written in — an entry outlives the version that chose them.

`--install` replaces an existing entry for this session where it sits, so the
order of your other jobs survives, and prints the line it displaced because a
crontab has no undo. An entry sweeping a different session is left alone, and one
commented out stays commented. `--refresh` is the same replacement without the
add: it is what `install.sh` runs on an update, so refreshing an entry you chose
never becomes adding one you did not.

`gang vet` reports the entry as absent, current, or stale. Absence is not drift —
plenty of installs want no ambient patrol — but an entry that no longer matches
what this install would write is, and vet prints both lines rather than leaving
you to diff from memory. This is the check that pays: a crontab is the one part
of an install that updating never touches, so an entry can go on running flags
the tool has outgrown with every other diagnostic reading clean.

A sweep whose stdout is not a terminal records itself; one run interactively and
it only prints. The destination is `GANG_PATROL_LOG`, defaulting to
`$XDG_STATE_HOME/gangline/patrol.log` and creating its directory. Set it empty for
no file. Each row carries an ISO-8601 local timestamp, then the same name, readout
and verdict the terminal shows, without colour escapes. `GANG_PATROL_LOG_MAX`
(1048576) is the size at which the file rolls to `patrol.log.1`, keeping one
previous generation and no more; deleting either file is the deletion path.

Routine `steady` rows are excluded by name and everything else is kept. That
direction is the point: filtering by a list of verdicts worth keeping cannot
report one that did not exist when the list was written, and a crontab line is not
something anybody revisits when the tool changes. A patrol only sweeps
`GANG_SESSION`, so a host running two teams runs two entries — write each from
the shell that team runs in, which is where `gang cron` reads the overrides it
carries. A patrol that disagrees about the lock directory stops serialising with
the other writers.

The automatic ladder starts at `GANG_CONTEXT_FLOOR` and ends at
`min(90% of the window, GANG_CONTEXT_CAP)`. Both bounds are absolute token
counts; only the spacing between them is fitted to the window. See
[ADR-0005](adr/0005-context-bands-are-absolute.md) and
[ADR-0006](adr/0006-the-band-ladder-spans-absolute-bounds.md) for the rationale.

| Condition | Agent and operator behavior |
| --- | --- |
| No rung crossed | No warning. |
| Lowest rung crossed | Information only: keep the continuation package current and continue the work already in hand. Renew at an appropriate checkpoint, not merely because this rung appeared. |
| A higher non-final rung crossed | Finish the current arc, update the package, then cycle. Later warnings close the door on starting more work before renewal. |
| Final rung reached | Update the package and cycle before the next action. Patrol repeats this instruction on every safe sweep until usage drops below the rung. |
| Window cannot reach the configured floor | Its ceiling becomes the sole rung and uses the final-rung instruction. |
| UI occupied, composer non-empty, or compaction pending/unproved | Injection is held without advancing warning state, then retried when the pane is safe. |

Crossings below the final rung are one-shot. `gang hook` and patrol share the
last-warned window option, and usage falling after compaction re-arms it.
`GANG_CONTEXT_BANDS` replaces the automatic ladder: bare entries are absolute
tokens and `%` entries are percentages of that agent's window. A profile may
override the floor or cap for its harness. Exact defaults are in the
[environment reference](reference.md#user-facing-settings).

### In-turn hook

`gang hook` consumes a harness hook event on stdin and, on the mapped
`UserPromptSubmit` and `PostToolUse` events, can return an `additionalContext`
warning during the agent's own turn. The Claude Code and Codex profiles wire
those events into the launch line at hitch, so a hitched agent carries this leg
with nothing to configure; the same verb ingests the turn-bracket facts the
reference describes.

It warns on both axes, sharing each one's last-warned window option with patrol,
so a crossing is spoken about once however it is noticed first. The reply has a
single `additionalContext` slot, so two crossings on one event travel joined in
it rather than one of them waiting for the next event.

The hook is deliberately silent on missing or malformed input so context
telemetry cannot block work. Patrol remains the harness-independent warning leg.

## The team's budget

`gang cutoff` declares when the work has to be done:

```sh
gang cutoff 2h        # a duration from now
gang cutoff 17:30     # a clock time today
gang cutoff           # what is declared, and what is left of it
gang cutoff clear     # remove it
```

One team has one cutoff. It is a declaration, not an estimate or an enforcement
mechanism. Gangline stores both the cutoff and when it was declared; declaring it
again, including through `gang hitch --cutoff`, replaces the team-wide budget and
restarts its span. The time ladder is relative to that declared span; see
[ADR-0009](adr/0009-time-bands-are-relative.md).

| Condition | Report and action |
| --- | --- |
| No cutoff declared | No budget row, note, or patrol log line. |
| Before the reserve | Each crossing is reported once with the remaining budget. The notes progress from checking the approach, to refusing scope growth, to banking durable results. |
| Inside `GANG_TIME_RESERVE` | Every safe patrol repeats the instruction to bank work, then permits continued improvement of what is already durable. |
| Past the cutoff | Every safe patrol restates the overrun, names the operator as the authority over the team-wide declaration, and still does not stop the agent. |
| Context or profile readout missing | The budget row still appears because it needs neither. A note is injected only when Gangline can resolve the profile and safely type into the pane. |

`GANG_TIME_RESERVE` accepts a percentage or a duration. It is removed from the
declared span before the ladder is derived; an unreadable reserve or one leaving
no usable span refuses the sweep. `GANG_TIME_BANDS` replaces the derivation with
comma-separated percentages of that usable span. Exact forms and defaults are in
the [environment reference](reference.md#user-facing-settings).

Patrol is the primary budget leg: it reports remaining time, repeats the reserve,
and restates overruns. The in-turn hook carries crossings only. Context and time
use separate last-warned options, and compaction changes neither elapsed time nor
the budget state. Routine `steady` rows are omitted from the patrol log; reserve
and overrun rows remain.

## Renewing context

Renewal is a cycle: a fresh agent replaces the old context and receives authored
current state. Native harness compaction is a separate operation described below.
Continuation state is split by ownership:

- The current lead alone maintains a durable `GANGLINE-TASK-LEDGER 1`. It holds
  only live team tasks, their owners and acceptance state, plus the private-note
  pressure policy. Every edit advances and reviews the ledger according to the
  closed grammar.
- Each dog maintains its own durable `GANGLINE-CONTINUATION 1` package. It names
  the ledger by absolute path, references only live tasks owned by that dog, and
  carries only that dog's active work, next actions, local blockers, binding
  references and dangerous refutations. It never copies task outcome, state,
  dependencies or other ledger fields.

The [command reference](reference.md) summarizes the required structure and
[ADR-0018](adr/0018-continuation-state-is-a-closed-reviewed-set.md) owns the
normative wire grammar.

Use this workflow:

1. **Establish shared authority.** The lead creates or updates the ledger at its
   durable absolute path. A worker asks the lead for ledger changes; it does not
   edit the ledger or copy missing task data into its package.
2. **Maintain one package per dog.** Keep it off `/tmp`, update it at checkpoints,
   remove notes when their declared expiry condition fires, and replace changed
   notes with new identifiers and explicit `Supersedes` links where required.
   Every claim carries an evidence category and provenance locator; binding bounds
   point to their source instead of copying governance prose.
3. **Review immediately before renewal.** Re-read the ledger, durable work and
   relevant environment. Set `Reviewed-At` to a host epoch strictly later than
   the window's review floor and any previously accepted package review. Reusing
   an inherited or already delivered timestamp is refused; a future timestamp is
   refused too.
4. **Cycle with the authored file.** For a self-renewal, the target, package
   `Writer`, and attributed sender are the same identity:

   ```sh
   gang cycle scout --from scout --resume-stdin < /workspace/team/scout.continuation
   ```

   Gangline stages and validates the complete package and its referenced ledger
   before retiring anything. It then retires the predecessor as `scout~spent`,
   hitches the replacement on the recorded launch facts, revalidates the live
   ledger and transition, and uses the ordinary pane-verified delivery path.
5. **Read the receipt literally.** Preflight acceptance is not delivery. Only the
   final `resumed <dog>` line establishes that the package reached the replacement
   and the transition became accepted. A valid structure still does not prove its
   claims true, complete, or sufficient to restore files, processes or credentials.

An older window without continuation state has a deliberate first-refusal path.
Its first structured attempt establishes an observable review floor and a
known-empty transition, preserves the live context, and refuses. Review the
package again after that floor, advance `Reviewed-At`, and retry. New windows made
by ordinary `hitch` or `adopt` already receive their floor and empty transition.

Every preflight refusal leaves the old cycle target running and the authored
files untouched. A failure after a candidate enters `pending` cannot be repaired
by cycling again or by `gang send`: direct messaging is not continuation
acceptance. Retire the lineage with `gang drop <dog>`, establish a genuinely new
one with an ordinary `gang hitch` or `gang adopt`, review a new package after its
new floor, and deliver that package through a later renewal. This loses the old
lineage state by explicit operator action; never treat `pending` as accepted or
empty.

Delete a dog's package when its referenced tasks are removed or the dog is
released. At team wrap, the lead deletes the ledger after no live task needs it.
Gangline does not author or delete either file.

### Native harness compaction

`gang compact <dog>` submits the profile's own compaction command on the same
window. It does not cycle the agent, restore a conversation, or change continuation
transition state. The success line proves submission only; the harness remains the
authority on whether compaction executed. A busy peer is refused. A self-issued
command is allowed to queue behind the current turn, but Codex rejects `/compact`
while a task is active, so use automatic compaction or have another caller compact
an idle Codex window.

`gang compact <dog> --from <dog> --resume-stdin` accepts only that dog's valid
structured package; arbitrary prose and a resume authored by another identity are
refused before the native command is issued. The resume is not typed immediately.
A detached worker waits for a declared compaction marker, context below half the
issue-time baseline, or bounded stable-screen evidence, with a timeout fallback
through the same verified injection path. At delivery it rechecks the active
target, ledger, review, task ownership and transition policy before marking the
candidate pending, then accepts it only after pane verification. `gang status`
and patrol report a detached failure. If that failure left `pending`, use the
explicit new-lineage recovery above.

## Understanding state

Gangline observes a terminal rather than receiving a harness lifecycle API.
State therefore combines several independent signals:

- a profile's busy regex against the pane;
- tmux's window-activity timestamp, only for profiles verified quiet at rest;
- pane content changing across two captures;
- a profile's modal regex, confirmed by the composer not being live;
- the conservative fallback that a missing expected composer is occupancy when no
  painted busy marker explains it.

Recent pty activity covers full-screen redraws that move the cursor without
changing pane cells. Pane churn remains as a fallback and also catches working
screens with no marker. A marker is the fast path, not the only busy signal.

Activity alone is bounded. When the pty timestamp is the only thing supporting busy,
it may carry that verdict for `GANG_ACTIVITY_LIMIT` — 300 seconds by default — and no
longer. Past that the state is `expired (pty activity bound reached)`, which Gangline
reports rather than resolves: an activity arm that has run out is a different answer
from an agent that was never busy, and quietly calling it idle is exactly how a
fabricated busy would become permanent. A send to an expired agent is refused unless
its profile declares a safe composer, because Gangline cannot otherwise know whether the
paste would land in the harness or in a live tool.

### An agent showing a busy marker reads as busy

The busy regex sees pane cells, not who painted them. Displaying a profile,
capture, or quoted marker can therefore make an idle contributor read busy. The
failure direction is conservative: quoted text can add a match but cannot erase
a real one, so sends or waits may be delayed but a working agent is never made to
read idle. No reliable composer or position test distinguishes the quote across
all harnesses. If an agent is visibly idle but reports busy, inspect its pane for
a quoted marker before treating the scraper as stale.

`pane_stable` proves only that captured cells did not change during the sample.
A silent tool call or compaction can hold still while work continues. Patrol
therefore also checks Gangline-owned compaction state and the input box before
injecting. The system prefers a delayed nudge or send over keystrokes aimed at an
unknown widget.

An agent blocked in its own `gang wait` reads `parked`, not `idle`. Gangline stores
the waiter's PID in `@gl_waiting` and reports every waiter as parked, because
*available* and *idle* are different claims and only the first can be true there. A
dead waiter PID is reclaimed when observed.

Parked is not the same as reachable. A profile that merely accepts mid-turn input
still queues the text until the running turn ends, and the running turn is the wait
itself: opencode was observed holding an accepted message until the wait call
returned. Only a harness witnessed *acting* on ordinary mid-turn text —
`GANG_MIDTURN_ACTS`, declared by `claude-code` alone — makes a parked agent answer
inside its own wait. Treat parked on any other profile as delivery availability and
account for the wait timeout before assigning work.

Observed live on 2026-07-30, a Codex contributor with `GANG_MIDTURN_ACTS` unset
waited the full 60 seconds on `lead` while lead's roster correctly reported
`parked (waiting on lead)`; the waiting state cleared when the call returned. That
is evidence for honest reporting without a claim of response availability, not
evidence that Codex acts on mid-turn input.

## Undelivered pastes

Delivery reads the composer before paste, after paste, and repeatedly after its
separate Enter. Failures after paste can leave text in the composer or make its
location unknowable. Blindly sending a clear key is unsafe because a modal may
now own that key.

Gangline records the incident in window options. It clears only when all of these
hold:

- the modal is gone;
- the composer is live and not changing;
- its exact rendering matches what Gangline recorded;
- repeated `Ctrl-u` presses visibly shrink it to empty.

Otherwise status prints a red `undelivered paste in the input box` detail,
roster adds `undelivered paste`, and patrol reports it. The record disappears
when the composer reads empty, even if a person cleared it.

## Diagnosing profile rot

Run plain vet first:

```sh
gang vet
```

It checks installed version words against profile pins, runs profile-owned file
format gates, and verifies that a UTF-8 locale is available. It never drives a
marker. Thus an all-OK result means the versions and declared file schemas still
match known observations; it does not prove today's TUI still paints the same
chrome. Themes, statuslines, and TUI extensions can move chrome without changing
the harness version.

Drive the actual marker when symptoms remain:

```sh
gang vet --probe codex
```

The probe uses its own explicit `tmux -L gangvet-<pid>` socket, launches the
harness exactly as its profile declares, waits for the composer, sends a real
prompt, and checks a transition: marker absent at rest, present during work,
absent after the pane settles. It then calls the profile's context reader. The
probe costs tokens and can take several minutes at its documented worst-case
bounds. A profile declaring `GANG_MIDTURN_ACTS=1` gets a second turn whose model
actions expose a filesystem boundary: B observed before the first turn's final
file A confirms the declaration.

A fresh temporary directory may trigger a harness trust dialog. Gangline never
answers it. Set `GANG_PROBE_DIR` to an **empty directory already trusted by that
harness**; do not use Gangline's own checkout, whose profile source contains the
marker strings being tested.

Interpret results narrowly:

- `MARKER DEAD`, `MARKER STUCK`, or a missing context readout is drift and exits
  nonzero;
- not installed, no marker declaration, an occupied input box, or no observed turn
  is **not probed**, not a pass;
- a zero exit covers only markers actually fired;
- the mid-turn declaration prints `CONFIRMED` only when B was observed while A
  was absent and A appeared later. A-before-B, both first seen in one poll, a
  fixture that does not start, and missing files are **not probed**. These
  outcomes cannot refute the declaration;
- occupied and compacting states are not exercised;
- one ordinary turn cannot exercise every alternate branch in a busy regex.

After live re-verification, shadow the profile through `GANG_PROFILES` if you
need a local repair. `gang vet` prints the custom file path when it is the one
loaded. `gang vet --file-issue` can file a deduplicated issue for version-pin
rot and for a tier conflict on a live agent, when the GitHub CLI is installed
and authenticated; a failure to file never unsays a printed finding or its
exit status.

## tmux socket safety

From inside a tmux pane, setting `TMUX_TMPDIR` does not redirect bare `tmux`:
tmux follows `$TMUX`. A diagnostic that owns a server must use an explicit
`-L` or `-S` on every operation, especially `kill-server`.

Gangline's probe does this and places a shim first on `PATH` so profile functions
that call bare `tmux` are directed to the same private socket. Teardown calls
`tmux -L <owned-name> kill-server` and removes only the socket name it minted.

For your own probes, use the same discipline:

```sh
tmux -L my-probe new-session -d -s test bash
tmux -L my-probe kill-server
rm -f "${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)/my-probe"
```

The same care applies one level up, on a socket you share with other people. A
bare `kill-server` there ends every session on it, and sessions you did not start
are not yours to end even by name — a test run stands sessions up for as long as
it needs them, and one killed out from under it fails as though the code under
test were broken. Kill by the explicit `-S` or `-L` naming a server you minted,
and leave everything else alone.
