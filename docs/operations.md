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
Gangline refuses both, because no shipped profile's `GANG_GATED_REGEX` can tell
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
  its required options", the same header/options/other shape. It is behind
  `experimental_request_user_input` in the `[tools]` table of
  `~/.codex/config.toml`; set it `false` to be sure of it, since whether the
  build defaults it on is not established here. MCP servers can raise their own
  elicitation modals on Codex, and no tool setting reaches those.
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
`status`, or self-compaction back through that socket.

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

Every context consumer goes through the active profile's `profile_context`:
`gang context`, the roster column, patrol, compaction proof, and the optional
in-turn context hook.

The shipped profiles obtain the value differently:

- **Claude Code:** `statusline/claude-code-context.sh` formats the harness's own
  statusline payload into a `ctx <used>k/<window>k <percent>%` pane beacon. Wire
  it into `settings.json` as shown in the README.
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

`gang patrol` is a one-shot sweep, suitable for cron. Create the log directory
first because the shell opens a redirect before it starts `gang`:

```sh
mkdir -p "$HOME/.local/state/gangline"
```

```cron
*/2 * * * * $HOME/.local/bin/gang patrol 2>&1 | grep -v ' steady ' >> $HOME/.local/state/gangline/patrol.log || true
```

The negative filter keeps unknown future errors while removing routine steady
rows. A patrol only sweeps `GANG_SESSION`; use one cron entry per session. Carry
`GANG_PROFILES`, any of the three ladder settings, `GANG_LOCK_DIR`, and
`GANG_CONTEXT_LOG` into cron too when the team overrides them: a patrol that
disagrees about the lock directory stops serialising with the other writers, and
one that disagrees about the log writes its measurements into a second dataset.

The ladder is derived, and both of its ends are absolute token counts. It starts at
`GANG_CONTEXT_FLOOR` (120000) on every harness, because context rot tracks how long
a context is rather than how full its window is — the same token count earns the
same warning everywhere. It ends at `min(90% of the window, GANG_CONTEXT_CAP)`, so
no agent goes unwarned past 350000 however large its window, and the last rung
always sits below the window with enough room left to issue a compaction and have it
land. Five rungs are placed across that span at fixed fractions, closer together
toward the top, so the warnings arrive faster as the situation gets worse
([ADR-0006](adr/0006-the-band-ladder-spans-absolute-bounds.md)).

The note escalates with the rung, and what escalates is the ask rather than the
volume. Below the top it asks for a compaction at the next arc boundary and states
how many bands remain before that stops being optional, which puts the deadline in
front of the agent as arithmetic from its first warning instead of springing it at
the last. At the top rung it asks the agent to stop where it is, close what it is
holding into handoff assets, and compact immediately. That is not a request to
compact mid-thought: an arc boundary is only ever wanted because it is a moment with
something coherent to write down, so writing it down is what makes this moment that
boundary, and waiting for a better one spends the last of the window on the work
whose handoff was the thing at risk.

A window too small to reach the floor keeps a single rung at its ceiling: it cannot
be warned about rot it has no room to suffer, so the one hazard left is exhaustion.
That rung is both the first crossing and the top of the ladder, so it carries the
top rung's instruction — there is nothing above it to escalate to.

Setting `GANG_CONTEXT_BANDS` to a comma-separated ladder replaces the derivation
entirely. Bare entries are absolute tokens; `%` entries are a percentage of that
agent's window — an escape hatch for an unusual one, never a default
([ADR-0005](adr/0005-context-bands-are-absolute.md)). A profile may export the floor
or the cap to set it for its own harness. The last warned band is a tmux window
option, shared with `context-hook`, and re-arms when usage falls after compaction.

### In-turn hook

`gang context-hook` consumes a harness hook event on stdin and can return an
`additionalContext` warning during the agent's own turn. It is designed for
Claude Code's `UserPromptSubmit` and `PostToolUse` events. Configure those events
to run:

```text
gang context-hook
```

The hook is deliberately silent on missing or malformed input so context
telemetry cannot block work. Patrol remains the harness-independent warning leg.

## Compaction and handoff

The robust self-compaction form is:

```sh
gang compact <self> --from <self> --resume-stdin <<'RESUME'
checkpoint and next action
RESUME
```

Gangline allows the calling agent to be busy: its compaction command can queue
behind the current turn. A different busy agent is refused because forced
compaction would discard its live work.

That transport permission does not make every harness's command self-safe. Codex
0.145.0 was observed rejecting `/compact` three times while a task was active;
a Codex agent invoking Gangline from its own pane is still in that task, so it
cannot self-compact by this route. Let Codex compact automatically, or have
another caller wait for it to become idle and then run `gang compact codex-name`
(with an attributed resume if needed). Do not pair the rejected self-issued Codex
command with `--resume-stdin`: Gangline proves the slash command was submitted, not
that Codex executed it, and a fallback can eventually deliver the resume without
a context drop. For every profile, the native compaction command remains the
authority on whether the request actually runs.

The resume is not typed immediately after the slash command. Harness queues can
hand ordinary text to the turn still running while keeping a slash command for
the boundary, causing the resume to overtake and be consumed before compaction.
Gangline instead starts a detached waiter.

The waiter uses the first available safe signal:

1. a declared live compaction marker (the shipped Pi profile currently declares
   one; the other shipped profiles do not);
2. for a gang-issued compaction with a readable baseline, context falling below
   half that baseline;
3. after the compaction grace or without context introspection, a bounded series
   of non-busy and stable-screen observations;
4. at the overall timeout, an attempted delivery through the ordinary verified
   injection path.

The context-drop threshold deliberately favours a late resume: a compaction that
reclaims less waits for the fallback instead of declaring success early. If the
detached delivery fails, the error lives in `@gl_resume_failed` and is reported
by status and patrol.

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
its profile declares a safe composer, because gang cannot otherwise know whether the
paste would land in the harness or in a live tool.

### An agent showing a busy marker reads as busy

The busy regex is matched against pane text without asking who put it there, and
`capture-pane` hands over cells rather than provenance, so it cannot be asked. An
agent that displays a profile, quotes a capture, or reviews this repository puts a
busy marker on its own screen and reads busy off its own source. The people that
lands on are almost exclusively the people working on gangline, since the markers
live in this repo's files — a dogfooding tax rather than a bug most operators can
hit.

It is accepted because it fails in the safe direction, and that direction is a
property rather than a hope: pane text can only **add** matches, never subtract the
one a real turn paints. So a contaminated pane makes an idle agent read busy — sends
queue or are refused, and `gang wait` runs to its timeout — and it can never make a
working agent read idle. Nothing is delivered wrongly; something is delayed.

There is no guard because the two available shapes both fail worse. Occupancy has a
structural companion — a dialog watched live owns the screen, so a composer still
being there proves the words are talk *about* a prompt — and busy has no equivalent:
the composer is painted through a turn on some harnesses and dropped on others, so
neither its presence nor its absence settles anything. Position does not work either;
prose-quoted markers were measured inside the bottom twenty rows in 49 samples and
outside it in the rest. And conjoining the two signals that *can* tell a live turn
from static text — recent pty activity and churn — would inverse the failure
direction, because both are unavailable unless a profile declares itself quiet at
rest, and an `AND` against an unavailable signal reads every busy agent as idle. That
trades this bounded cost for an unbounded one.

If you are working on gangline and an agent reads busy while plainly idle, check
whether its pane is showing a marker before suspecting the scraper.

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
itself: opencode 1.18.8 was observed holding an accepted message until the wait call
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
bounds.

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
- occupied and compacting states are not exercised;
- one ordinary turn cannot exercise every alternate branch in a busy regex.

After live re-verification, shadow the profile through `GANG_PROFILES` if you
need a local repair. `gang vet` prints the custom file path when it is the one
loaded. `gang vet --file-issue` can file a deduplicated issue for version-pin rot
when the GitHub CLI is installed and authenticated.

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
