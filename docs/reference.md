# Command and configuration reference

`gang --help` is the authoritative command inventory, and
`gang <command> --help` prints that command's synopsis. Help has a deliberate
48-character line budget so it remains legible in narrow phone-SSH terminals;
this reference carries the complete command contract.

`gang` with no arguments prints a getting-started page written for an agent
rather than an inventory: what it is, how to send, that reading its own mail
consumes it, what each exit status means, and where the inventory is. It exits
0 on stdout, so "run gang" is a complete onboarding instruction for a
freshly hitched agent of any harness. The executable-skew warning is emitted
before it, since this is the first command an agent runs.

When a command's only missing argument is an agent name, an invocation from
inside a Gangline window that omits that name targets that window. An agent
reading or stopping its own state does not have to know its own name. What is
omitted is the name, not every argument: a leading flag is not a name, so
`gang interrupt -m "reason"` is a self-targeted stop carrying its reason, while
a positional argument is always read as the name — a numeric argument to
`capture` remains an agent name rather than becoming a self-targeted line count.
Self is resolved from the calling tmux pane in the same way as a message sender.

| Bare command | Result |
|---|---|
| `status`, `capture`, `composer`, `compact`, `context`, `mail`, `limits`, `wait-limit`, `interrupt`, `flush` | Target the calling agent. |
| `drop` | Print help; destructive commands never target by omission. |
| `hitch`, `adopt`, `rename`, `send`, `wait`, `explain`, `down` | Print help; the missing name is not a self target. |
| `up`, `roster`, `attach`, `teams`, `tick`, `collars`, `models`, `roles`, `config`, `curfew`, `notify`, `upgrade` | Keep their ordinary bare meaning. |

Resolving self is not a promise that the command proceeds. `gang compact`
self-targets and is then refused, or deferred, on that same agent's own turn
evidence. Who the target is and what its state allows are separate answers.

Outside a Gangline window, a bare self-targeting command prints its synopsis and
states that no target or Gangline agent window was available.

Every command names an argument it does not consume, and refuses. Nothing is
accepted and discarded: a word Gangline drops silently has told its caller that
the word was understood, and the reading that comes back is then of something
nobody asked for. `gang hook` names it and declines the event instead of dying,
because its caller is a harness configuration rather than a person; see below.

## Installation

The bootstrap command in `README.md` downloads `install.sh` from `main`, but the
installer resolves the newest stable `gangline-vMAJOR.MINOR.PATCH` tag and puts
that release in the install tree. An existing install is moved directly from
its current release to that tag; local changes in the install tree are refused.

`gang upgrade --check` contacts the same repository and compares the installed
`version.txt` with its latest stable release. `gang upgrade` installs that
release over the tree and command link currently running. Both are explicit
network operations. Gangline performs no automatic release check during other
commands. `GANGLINE_REPO` continues to select a different tagged source for a
custom install. `gang upgrade` refuses a branch checkout and points to
`git pull --ff-only`; only the detached release tree produced by the installer
is replaced.

## Lifecycle

### `gang up [name] [hitch flags]`

Hitches the team's lead, names its window `lead` when omitted, then attaches or
switches the current tmux client to it. It attaches the shipped `lead` role by
default; an explicit `-r` or `--role` selects another role. `GANG_COLLAR`
selects the harness. If a native first-run gate appears, `up` exposes it before
waiting for startup-contract delivery, so answering that prompt remains the
only operator step.

### `gang hitch <name> [-c harness] [-d dir] [-m model] [-e effort] [-r|--role role] [-l|--lights lights] [--resume [session-id]]`

Starts a native harness in a named tmux window and delivers one startup contract.
That contract names the agent and carries `CONTRACT.md`, which holds the
standing terms. Any supported `-m` or `-e` choice omitted at hitch warns before
the collar silently chooses that default.

How it reaches the agent depends on the collar. Where one declares
`GANG_ROLE_PROMPT_OPT`, the contract's bytes are passed through that launch
option into the harness's own system prompt, so every agent holds it
unconditionally and still holds it after a compaction. The startup contract does
not restate what that prompt already says. It carries the one thing the prompt
cannot: where to look, and an instruction to report it, if the attachment is not
there — Gangline cannot verify that a launch option reached the model. Where a collar declares no such
option, the startup contract points at the path and tells the agent to read it
first and again after a compaction, and to say so and stop rather than improvise
if it cannot. Either way the bytes are never pasted into a composer, so the
contract's length is bounded by nothing the pane can render, and Gangline
resolves and validates the file before opening a window.
If `$GANG_CONFIG_DIR/DOCTRINE.md` is present, readable,
valid UTF-8 prose, the contract attributes and
appends it byte-exactly. Every hitch carries doctrine; Gangline cannot infer
which caller is the operator. `adopt` still injects no startup text.

The launch environment carries the exact `GANG_SESSION`, the absolute resolved
`GANG_CONFIG_DIR`, and any custom collar and lock paths, so harness commands
cannot drift to another session or configuration layer on the same tmux server.
File-layer settings therefore reach nested hitches. Other per-invocation
environment overrides do not become sticky inside the agent.

If a first-run prompt owns the screen before the composer appears, `hitch`
directs the operator to `gang attach` as soon as the collar's occupied pattern
provides positive pane evidence. It immediately parks the attributed contract
in the window's ordinary spool: answering the native prompt is the only manual
step. Direct `hitch` stays in the foreground; `gang up` exposes that gated
window in its tmux client while the same invocation observes beside it. The
verified drain runs when the composer appears and retries pre-keystroke
refusals, so delivery does not depend on a native hook the operator may decline.
After positive gate evidence, `GANG_BOOT_TIMEOUT` is one observation slice and
`GANG_GATE_LOOKS` bounds how many slices see the prompt still unanswered. At the
bound `hitch` stops waiting and exits 4 — distinct from a failed hitch and from
a delivered one — with the window alive, the harness running behind its prompt,
and the attributed contract committed to the spool. A prompt is answered by a
person, and holding the caller's terminal until one arrives stalls whoever
called: often another agent, which cannot answer a native prompt at all, so one
gated boot becomes two stopped agents. Raise `GANG_GATE_LOOKS` where the
terminal is free to keep waiting. `gang up` is exempt: it has already put its
caller in a tmux client looking at the prompt, so there is no stalled third
party, and giving up would detach the one client that can answer.

Gangline starts no resident watcher. A hitch that stops waiting — at that bound
or on an interrupt — leaves the attributed entry inspectable. After the prompt
is answered, any later Gangline invocation supplies a one-shot tick that retries
that spool through the ordinary delivery gates; no turn from the recipient is
required merely to create the opportunity. Drop and hitch again only when the
native process itself must be replaced. The printed `--resume` form applies only
if a native session was stamped, and a pre-turn gate normally has no resumable
identity.
`gang up` needs a terminal for the tmux client that exposes the prompt; without
one it refuses and leaves the contract queued rather than pretending the prompt
was exposed.
If hitch has no positive prompt evidence, it fails with the manual attach/send
recovery because it cannot truthfully call an unknown stable screen a startup
gate.
If nothing is running in the window — the pane exited, or tmux destroyed the
window along with it — hitch ends the wait there instead of spending the boot
budget, and refuses with the launch command, plus the exit status and the pane's
last line wherever the corpse was held. A launch that died is not a slow boot,
and the attach recovery above does not apply to it.

- `-c` selects the harness, by the name of the collar that drives it.
- `-d` selects the harness working directory.
- `-m` passes a harness-native model choice through the collar's model option.
  Before a window opens, a complete collar catalog must contain that exact id.
  A harness with no complete catalog may instead expose a native recognition
  check. Recognition proves that the installed harness understands the name,
  not that the current account may use it; an unreadable check is unknown and
  refused rather than treated as acceptance. A model option with neither form
  of validation is refused. Use `gang models -c <harness>` for discovery.
- `-e` passes a harness-native reasoning-effort level through the collar's
  effort option, joined with no space, on the plain and `--resume` launch forms
  alike. The collar prints its level vocabulary and a level outside it is
  refused before any window opens. A collar that declares no effort spelling
  refuses the flag, and a vocabulary that cannot be determined is refused as a
  broken declaration rather than a bad value.
- `-r`, `--role` attaches the named role brief to this hitch only. `gang hitch`
  has no role default and never infers one from the agent name. Role names use
  letters, digits, dot, dash, and underscore and may not begin with dot or dash.
- `-l`, `--lights` sets context lights for this agent alone: `collar` to take
  the collar's own default for the chosen model, `off` for none, or explicit
  `yellow,red` tokens or `yellow%,red%`. It overrides `GANG_CONTEXT_LIGHTS`,
  whose own built-in value is `collar`. The collar is sourced already knowing
  this agent's request, because a collar may wire its native context source at
  launch and lights armed over a source nobody painted report a miss on every
  turn.
- `--resume <session-id>` substitutes that exact native session identity into
  the collar's resume template. Bare `--resume` is only valid when a dead,
  still-existing window registered to the same agent carries `@gl_session_id`.
  A missing id refuses with `gang whoami` and the explicit-id remedy; there is
  no latest/continue fallback. Collars without an explicit-id resume template
  refuse either form.

Names use letters, digits, dot, dash, and underscore, may not begin with dot or
dash, and must be unique in the team. `hitch` is reserved as the startup-envelope
sender.

With `GANG_SCOPE=on` the launch is wrapped in a transient systemd user scope,
`gangline-<session>-<agent>.scope`, with `MemoryAccounting=yes` stated rather
than inherited. `systemd-run --scope` execs the harness, so the pane process,
its tty, its environment, and its exit status are the harness's own; only the
cgroup changes. The setting is off unless declared, and a host that has no
`systemd-run` or no reachable user manager refuses the hitch instead of
launching an unscoped agent.

Gangline tries the process's direct user bus first. If that bus is unusable but
systemd exposes the local host machine transport, the scope preflight and stale
unit check retry as the current user through `<user>@.host`. This lets a caller
inside a PID-isolated sandbox hitch a scoped agent without granting the caller a
different filesystem policy; the harness still launches from tmux's host
context.

The scope name is stable, so a name still held by something an earlier agent
left behind refuses the hitch and names the unit to stop, rather than failing
inside tmux as a window that died at launch. `adopt` registers a window Gangline
did not launch and so cannot scope one; an adopted agent stays in whatever
cgroup its window already had.

The tmux server that holds the team is scoped the same way, as
`gangline-<session>.scope`, so its death is a named unit stopping rather than an
anonymous process exit inside whatever login session started it. Only the
`tmux new-session` that actually forks a server is wrapped: against a server
that is already running, `new-session` forks nothing and the scope would hold
only the client that exits a moment later, so that case prints a warning saying
the server stays outside the accounting and hitches anyway.

The reason is blast radius. `systemd-oomd` kills the descendant *leaf* cgroup
holding the most swap, and a tmux server inherits the cgroup of whatever started
it, so by default every agent on a team shares one leaf and one kill takes all
of them. One scope per agent makes each agent its own leaf and its own name in
the kill message.

Two consequences belong to the operator, not to Gangline. A scope lives under
the systemd user manager, so scoped agents also come under whatever
`ManagedOOMMemoryPressure` policy that manager carries; that policy has its own
trigger — the manager cgroup's own memory pressure — and is not a fallback for
the swap policy. And a swap kill only selects a candidate holding more than 5%
of total swap, so splitting a team finely enough can leave every agent under
that bar and the swap policy selecting nobody where it previously selected the
team. `oomctl` prints the live policy; `GANG_SCOPE` changes what the victim pool
looks like, not what the thresholds are.

The thresholds settled at hitch are written to the window on every hitch,
including a hitch that settles on none, and nothing re-resolves them later. A
`--resume` respawns the window it is registered to and window options outlive a
respawn, so the empty write is what stops an earlier launch's thresholds from
staying armed over a launch that wired no native source to read them. A resume
that omits `-m` therefore resolves no collar default, exactly as a first hitch
would; `gang drop` prints no model in its relaunch line, so pass `-m` or `-l`
again to keep the lights the agent had. Which thresholds those are is
decided in one pass: `-l`/`--lights` if given, otherwise `GANG_CONTEXT_LIGHTS`,
whose built-in value `collar` asks the collar for its own default for the
hitched model. A collar answers per model because one harness runs models whose
native windows differ several-fold, and the same fraction leaves very different
absolute runway in each. A collar that declares no default, or none for that
model, leaves the lights off, and so does a collar with no native context source
to read — a default never arms a light its own collar cannot take a reading for.
An explicitly configured threshold still arms there, because that was an ask.

Percentages are relative to each native window and serve mixed-window teams.
Absolute tokens remain supported for an observed fixed window. Place both high
enough not to impose an artificial context disadvantage, but below the harness's
observed automatic-compaction boundary. An absolute red threshold above the
reported native window produces an invalid-light notice rather than staying
silently unreachable.

### `gang adopt <name> -c <harness>`

Registers an existing window in `GANG_SESSION`. `-c` names the harness already
running there, by the name of the collar that drives it. Adoption does not inject
startup text or retroactively add launch-time native hooks. A collar whose
context source requires hitch-time identity may therefore report context
unavailable.

Both hitch and adopt also stamp provenance: `@gl_hitched_by` holds the `@gl_spool`
token of the agent window the command ran in, or `operator` when it did not run in
one, and `@gl_hitched_by_name` holds the hitcher's name as witnessed at that moment.
`gang status` resolves the token back to whatever that window is called now, so a
later `gang rename` of the hitcher does not rot the record; the witnessed name is
printed, and said to be gone, only when no live window claims the token.

Both hitch and adopt stamp the agent name in `@gl_agent` and the executable
identity in `@gl_binary_id`. Reusing a window whose recorded identity names
another agent refuses. The binary identity is
`cksum:` plus the POSIX checksum and byte size of the invoked `bin/gang`, in a
checkout or an installed tree alike. It therefore witnesses the executable
bytes rather than repository metadata. If the checksum cannot be read, the
stamp is `unavailable`; identity evidence never blocks lifecycle commands.
When the invoked `bin/gang` is tracked in a git checkout but differs from HEAD,
every operator command prints one stderr warning naming the path and HEAD. The
native hook endpoint stays silent except for a crossed light. Restoring the file
to HEAD removes the warning; the live symlink/install path is unchanged.

### `gang rename <old> <new>`

Changes the registered identity of one agent and rewrites its window title.
The old name is resolved from `@gl_agent`, never from the decorative title;
the new name follows the same validation and uniqueness rules as a hitch name.
After the command succeeds, roster, status and delivery resolve the new name,
and attempts to send to the old name fail rather than reaching it by title.

Renaming does not replace the window or restart its harness. Its `@gl_spool`
token is unchanged, so messages already parked for the agent remain reachable
and drain normally under the new name.

### `gang attach`

Attaches to `GANG_SESSION` on this shell's own tmux socket.

When the team is not on that socket, `attach` reads the socket `hitch` recorded
for it, says on stderr which socket it is crossing to, and attaches there. A
team with no record, or one whose recorded socket no longer answers, is refused
with which of those two it was.

### `gang teams`

Lists every team Gangline has written down and the tmux socket each is reachable
on, newest state first asked of the server rather than read off the file: `live`,
or `gone` distinguishing a socket nothing answers on from a server that answers
without that session.

`hitch` records `GANG_SESSION` and its socket under `GANG_LOCK_DIR/teams/` on
every hitch; `down` removes the record for the team it ends. A session name that
is not usable as a filename is not recorded, and `hitch` says so — the team runs
either way, only discovery is unavailable for it.

This exists because tmux clients discover the default socket while a private
`TMUX_TMPDIR` moves it, so a live team can be unreachable from a shell that lost
that environment and look exactly like a team that has gone.

### `gang tick`

Runs one cooperative full pass synchronously. Every other invocation that can
address the live team launches the same pass detached after its main work and
preserves its own exit status. `tick` exists as the deterministic operator and
test entry point; ordinary use does not need to call it explicitly.

One pass visits every hitched window, retries every waiting spool through the
same occupancy, turn, copy-mode, composer and verified-submission gates used by
`send`, then retries deferred self-compaction only where those gates leave it
safe. It also asks collars that declare `collar_live_session_id` whether the
process currently holding the pane is the registered native session. A
contradiction becomes `session-lost`, blocks delivery, and fails the pass rather
than letting a restarted harness impersonate the old agent.

One per-team kernel flock admits a worker for its whole lifetime; the generation
symlink carries diagnostic and recovery metadata rather than mutual exclusion.
Owner death releases the flock in-kernel, and metadata retirement holds that
same guard across its decision and unlink. The empty guard file remains under
`GANG_LOCK_DIR` until the operator removes that lock root. A concurrent
candidate touches a dirty marker and exits immediately; the owner consumes that
marker with another pass. Dead, zombie, and replaced generations are reclaimed.
The internal worker accepts only the controller's fixed production budget; a
caller cannot feed shell arithmetic a different deadline. A matching owner
at least the hard 60-second worker deadline fails health instead of looking
like clean contention. The lock stamp uses the same monotonic clock domain as
the controller deadline, so suspend and wall-clock adjustment do not spend
that budget. At twice the published budget, Linux may reclaim only an exact
tick-worker leader: it sends one SIGKILL through a generation-bound pidfd,
confirms exit for at most one second, and retires the unchanged lock. It never
signals a bare PID or process-group number. A legacy pid-only lock is retired
when the live PID is positively not a tick worker for this team, but never
authorizes termination because it has no generation or monotonic acquisition
stamp. Ambiguous identity, a tick-shaped live legacy owner, and failed
termination retain the lock and fail loudly. Recovery is
cooperative, so it begins on the next Gangline invocation rather than in a
resident watcher.

The worker and its descendants are also killed by their owning deadline
controller at 60 seconds. HUP, INT, TERM, or ALRM caught by that controller
kills and reaps its still-owned worker group before the controller re-raises the
signal. A detached failure cannot change the command that spawned it. It writes
`health` and `tick.log` under
`${XDG_STATE_HOME:-$HOME/.local/state}/gangline/tick/<team-key>/`; the next
invocation, `status`, and `roster` report the last failure. An attached client
also gets a silent-until-failed status-right segment, a display-message flash,
and a `gangline-alerts` window with activity and bell monitoring. A later clean
pass replaces failed health with `ok`; `down` removes that team's health files.

### `gang drop <name>`

Prints the window's stamped native session id, then kills the exact agent
window. Whether that id comes with a relaunch command depends on the collar: one
that declares a resume launch gets the exact
`gang hitch <name> --resume <session-id>` line, and one that witnesses its
harness's id without declaring such a launch gets the id alone, said to be a
record of the session rather than a way back into it — `hitch` takes no
`--resume` for that collar, so quoting the command would print a line it
refuses. If the collar has supplied no stamp, `drop` says that instead. Its
tmux-owned state and spool die with it after any pending messages are archived.

### `gang down <session>`

Kills the exact team session and every window in it, archiving each window's
pending spool first. The session name is required and must match the team this
shell is pointed at; `down` refuses a name that does not match, and refuses
outright when it is run from a pane inside that session. There is no override:
an agent must not be able to end the team it is running in.

## Delivery and compaction

### `gang talk <name> [--from <sender>] [--live-only] [--supersede]`

Opens `${VISUAL:-${EDITOR:-vi}}` with a private empty draft, then gives its
non-empty contents to `send` after the editor exits successfully. It is the
terminal-authoring complement to `send`: closing an empty draft cancels without
sending, while every non-empty draft gets the same attributed, verified
delivery and exit result as `gang send --to <name> --stdin`.

`talk` requires stdin, stdout, and stderr to be terminals. Scripts and agents
always use `send --stdin`, which remains non-interactive and unchanged. An
operator outside a Gangline agent window defaults to the clearly claimed sender
`operator`; an agent's sender is observed as it is for `send`. `--from`,
`--live-only`, and `--supersede` retain their `send` meanings. An editor that
exits non-zero sends nothing and returns its status. The temporary draft is
removed before delivery begins.

### `gang send --to <name> [--from <sender>] [--live-only] [--supersede] --stdin`

Reads the full message body from standard input. Inside the team, Gangline derives
the sender from the calling window and refuses `--from`, but only when the pane
carries matching `@gl_agent` and collar registration and no recorded native
session mismatch. An unadopted window name is not an identity. Self-send to the
same pane also refuses. Calls from outside the team must supply `--from`.

A sender Gangline observed and a sender a caller claimed are different facts,
so the envelope says which it carries. Where Gangline read the name off the
calling window the envelope names it plainly; where it could not see a window
and `--from` supplied the name, both tags carry it as `self-declared:<name>` —
including from the operator's own shell, which is the case that cannot be told
apart from a sandboxed process reaching the socket with the tmux environment
stripped. This is a label, not a check: nothing proves who called, the claimed
name still stands as claimed, and `:` is not a usable agent name, so an
observed envelope can never be mistaken for a declared one or the reverse.

Gangline wraps the body in a nonce-bound envelope,
serializes writers per pane, verifies the paste changed the target composer,
submits it, and reports success only after verification.

Gangline refuses a missing or occupied composer, a human draft, tmux copy-mode,
unknown state, and unsafe mid-turn input. Copy-mode is operator-owned: Gangline
does not cancel it, and checks `#{pane_in_mode}` before every paste, submit,
queue-recall, clear, and interrupt key. A positive mode read is confirmed once
before refusal, so a mode that ended between reads does not strand an idle
recipient; window-name state glyphs never enter this decision. A collar may
declare that its native harness accepts ordinary mid-turn input, or that a free
mid-turn composer accepts attributed spool entries as native steering.

A refusal on a box that is not provably empty classifies what Gangline read, in
one word with the look or recovery that settles it: `draft` (a human line
Gangline did not write), `staged` (Gangline's own undelivered body, byte-identical
to the rendering it recorded), `unattributed` (Gangline recorded an undelivered
paste here but cannot match it to this box, so the author is exactly what is
unknown), `parked` (the harness's declared queue evidence), `whole-pane` (the
collar declares no input reader, so the reading is the pane rather than a box),
`cleared` (the box read empty when Gangline looked again, so the obstruction
left between the decision and its naming), or `unreadable`. The classifying
look is taken after the decision it names and changes nothing about it; a
refusal stands even when the box clears underneath it. A suggested-prompt placeholder is not among
them: the collar's styled reading strips it, so it never reaches a refusal —
`gang capture` is where it looks like a draft, and `gang composer` is the reading
that settles it.

Where a collar can recognise its harness's own dialog chrome, a refusal that
meets one names it. An overlay dialog can own the keyboard while the composer
under it still reads as usable, so a delivery into that pane goes nowhere; the
refusal quotes the dialog's visible title and says to answer it. Recognition is
structural — the frame rather than the wording — so it survives a harness
rewording its dialogs, and it decides nothing: a collar that declares no reader,
a pane that cannot be read, and a frame nothing recognises each cost the
sentence, not the delivery.

An expired busy witness alone does not veto delivery: could-not-determine
falls through to direct box evidence, a provably empty composer proceeds
under the full submission verification, and anything less refuses naming both
the expired witness and the box state. No reader writes turn state — not
ordinary delivery and not status. A tick delivery into a hook-enabled target
records the positive open edge it creates before Enter, so the native
UserPromptSubmit/Stop pair can overwrite it even when that turn opens and
closes before verification returns. A malformed value is reported as
unreadable, never repaired, and eligibility
is re-derived from live evidence on every send.

The frozen-paint demotion requires an expired bracket, so a window with no
native hooks and no mid-turn-input declaration whose pane keeps a matching
busy marker stays refused until the marker scrolls off or the agent is
dropped.

An unexpected queue is not delivered: where a collar declares queue evidence,
a hint already present before a new delivery prevents Gangline from pasting
another body. A hint first observed after Gangline presses Enter is different:
it can describe parked input or race a turn the session already accepted, so
Gangline reports the submission outcome as unknown. It retains the exact body
and composer evidence for conditional recovery, but status requires transcript
or current-context evidence before `gang flush` or re-send; it does not infer a
drop-and-resume repair. The exception is a claim that arrived through a collar's
explicit `steer` path; there, native mid-turn queueing is the declared
destination. Gangline records the exact composer even though the attributed
spool claim retires, so status can expose that declared parked landing and
`gang flush` can verify a later recall. An unreadable verification capture after
Enter remains ambiguity and fails closed.

This detection is scoped to verified harness renderings: the claude-code pin
is the composer hint observed on 2.1.223, and a harness version whose
rendering has not been observed narrows the guarantee back to box-change
verification without refusing sends. A reworded hint is the event that
reopens the decision between stronger session-record verification and a
version gate.

Auto-mode environment occupancy is separately pinned to the claude-code
2.1.239 frame captured in `test/fixtures/claude-auto-mode-environment.txt`.
The frame's pure top band must touch its exact title, and its exact guide must
be the last nonblank row before the composer opens. Those two positional
questions keep matching message content readable, at the cost of narrowing
the guarantee back to ordinary composer evidence when the native dialog is
reworded or rearranged. Either change is the event that reopens whether to
update the exact pin or add a version gate.

A cleared staged record means the obstruction is gone — it is never
retroactive proof that the recorded body was delivered.

A refused delivery parks by default. The live delivery is attempted first; if
it is refused, the nonce-bound envelope is written to a per-target spool and
reported as parked, not delivered. A `steer` collar then tries the whole queue
under that same pane lock: only a free composer may take its already-attributed
claim. A failure after anything was typed is never parked, because that body's
fate is unknown and a second copy would be a second message. Pass `--live-only`
for an availability probe that must return a refusal to its caller instead of
parking it; without the spool, it cannot use steering.

Every cooperative tick retries ordinary mail across all windows. The target's
own native Stop event remains an immediate opportunity. On a `steer` collar,
PostToolUse also tries the spool while the turn remains open; a free composer
accepts its claim as native steering, while an occupied composer defers without
typing. An open compaction is not a steering composer for peer mail: its native
queue belongs only to its attributed continuation, and PostCompact drains peer
entries after that bracket closes. Hitch itself drains a startup contract
parked before any turn existed once it observes the composer. Every route uses
the same oldest-first verified delivery path. The pane delivery lock is taken
before the first entry is
claimed and held through delivery and claim retirement, so crossed native
workers cannot split or reorder the queue. Copy-mode and other pre-keystroke
refusals leave entries live and unclaimed for the next tick or native
opportunity. A
drain that cannot read a composer after an idle native boundary leaves its
entries waiting and records a visible drain failure; it never types through
that uncertainty. A
drain that cannot verify, or one that dies between the submission and the
entry's retirement, leaves that entry held: `status` says its delivery was not
verified and may still have arrived, `status` and `roster` report how many are
held, the bodies
stay readable under `GANG_LOCK_DIR`, and Gangline never sends them again. A
harness may accept a submission into its own queue and drain it later; read the
target before re-sending by hand.

A delivery whose Enter was pressed and whose screen then stopped answering is
neither of those. It cannot be parked — the body may already be in front of its
recipient — and it is not a plain failure, which would invite a second copy sent
by hand. `send` exits **5** and reports `delivered but UNVERIFIED`; a drained
bundle in that state is recorded under `unverified-` rather than `failed-`, and
`status`, `roster`, `mail` and the teardown archive name it apart from an entry
Gangline watched fail to enter. Neither is ever sent again. Read the recipient
before sending it a second time.

A body written but never committed — `spool_stage` wrote it and the sender died
before `spool_commit` renamed it into the deliverable namespace — is counted and
named the same way, once the process that wrote it is gone.

`--supersede` retires the same sender's earlier waiting messages once the newer
message is accepted, whether the newer one parks or is delivered live. The
retirement and the acceptance happen together or not at all. The sender's older
messages are first moved out of the namespace a drain reads — reversibly, with
nothing destroyed — and only then is delivery attempted: live if the target's
composer is free, otherwise the body is written and parked. The retirement
becomes final once that has succeeded. So a supersession that cannot retire
refuses before anything is typed or written at all, and a delivery that fails
after the move puts the older messages back where a drain will claim them. The
recipient never sees a superseded message and the message that replaced it as
one bundle, and no refusal leaves the older ones silently retired.

A refusal that happens after the body has been written names the file it is in.
A refusal from the retirement preflight names none, because at that point no body
has been written. In the one case where restoring a moved message also fails,
gang says which messages it could not put back: those are still on disk and are
archived at teardown, but they are no longer deliverable, so that refusal is the
notice to go and read them.

It is scoped to the sender, not to a subject: a sender with two unrelated
messages waiting for one target loses the first when the second carries the flag,
whether that second message parks or is delivered live. Pass it only when the
newer message genuinely replaces everything that sender has parked.

A collar that declares no `GANG_STOP_HOOK` still receives the ordinary live
attempt. A pre-keystroke refusal parks for a later cooperative tick, which
retries the same verified path without inventing native turn evidence. Such a
collar still cannot satisfy `gang wait`, raise an immediate Stop opportunity,
or witness an inside-harness deferred self-compaction request.

Spool entries live under `GANG_LOCK_DIR`, keyed to an identity minted into the
target's window options at `hitch` or `adopt` — never later, so that senders
arriving together cannot mint competing ones. After a live refusal, a window
without that identity says the message was not parked and names re-hitch or
re-adopt as the repair. Re-adopting a window keeps the identity it already
carries, so nothing already parked under it is stranded. When a window dies,
`gang drop` and `gang down` name what is still waiting and what is held, then
move both under `GANG_ARCHIVE_DIR`, grouped by teardown and agent, before
deleting the spool. Empty queues create no archive directory.

A window that dies any other way — an external `kill-window`, or a tmux server
that goes away with every window option in it — leaves its spool directory
behind with nothing pointing at it. The hitch that opens a session sweeps those:
a directory no live window claims is archived down the same path, removed, and
reported on stdout. What it cannot archive, and what Gangline did not mint, are
named and left untouched, and `roster` names whatever remains under the spool
root so a person can read and retire it. A window list that cannot be read is
reported rather than treated as "nobody holds anything".

One tmux server per `GANG_LOCK_DIR`. The spool root is keyed by that directory
and the register of live spool identities is the tmux server's window list, so a
second server sharing one lock root would read the first server's spools as
unclaimed. This is the same assumption `spool_mint` already makes when it draws
an identity no live window holds.

### `gang at <duration|HH:MM> --to <name> [--from <sender>] --stdin` / `gang at --to <name> --clear`

Parks an attributed message now and delivers it once the clock passes. The time
is a duration (`90m`, `2h30m`) or a local clock time (`17:00`), which means its
next occurrence. The envelope is minted at park time, while the sending window
can still be observed: re-attributing it when the timer runs would downgrade an
identity Gangline watched to one it was merely told, because a clock has no pane.

There are no new delivery semantics. The message is written into the target's
own spool as an ordinary three-line entry under a name beginning with a dot, and
every path that drains, counts, ages or supersedes a queue matches `[0-9]*`, so
nothing sees it while it waits. When the time passes, one transient systemd user
timer renames it into that namespace and asks for a drain: from that moment it is
ordinary waiting mail, delivered and verified like any other message, at the next
opportunity its target exposes. Gangline runs no watcher, scheduler or retry loop.

`gang roster` marks a target holding one as `timed=<n>`, and `gang status <name>`
names when each comes due. Status reads the timer as well as the entry, because
the two can outlive each other: a message whose timer is gone is reported as one
nothing will promote, and a host that cannot answer that question is reported as
unreadable rather than as either. A message already past its due time and still
parked is a timer that did not run, and says so.

If the timer cannot be created, nothing is parked. If the agent is dropped first,
the entry is archived with the rest of its mail and arrives under a visible name,
because a spool archives every child. `--clear` cancels every timed send parked
for a target, stopping each timer before removing the message it would deliver.

### `gang flush [name]`

Recovers a message the harness parked in its own input queue, as a verified
operation. Gangline presses the collar's declared recall key, reads the loaded
composer back against the body it recorded alongside the queue hint, submits
it, and verifies the submission the way any delivery is verified. When that
record came from a hint first observed after Enter, transcript or current-
context evidence must first confirm that the body is parked.

The readback is against the body Gangline composed, never against a second
reading of the composer: two readings taken at different moments are two
renderings of one body, and a harness that shows a pasted multi-line body as a
placeholder and expands it on recall makes them differ in kind. It is whole —
containment would accept a truncated, altered or appended remainder — and it is
over what a pane capture can carry, which is the body's text with every run of
blank space collapsed. A capture pads every row to the pane width, gives
continuation rows the composer's gutter, and re-flows a body line too long for
the box across rows, so two bodies differing only in blank space cannot be told
apart by any comparison against one. Every difference in the body's text
refuses.

It refuses before pressing anything when the collar declares no queue evidence
or no recall key, when the composer shows no parked-queue evidence, when
Gangline holds no record of the parked message, and when it recorded the park
without the body it parked. It refuses after the recall key when that key
loaded nothing, when the composer stopped reading back, or when the readback is
not the recorded body; the Enter is not pressed in any of these.

A refusal after the recall key reports what the recall left behind, because by
then it has changed the world: the loaded body is visible and unsent in the
composer, the Enter was not pressed, and whether a copy is still waiting in the
harness's own queue was never read — so submitting the visible draft by hand
may deliver it twice, and Gangline does not promise it drains on its own.

### `gang interrupt [name] [-m "reason"] [--from <sender>]`

Sends the keystroke the collar declares as its harness's turn-stop key and
drops Gangline's turn bracket, so the interrupted turn neither leaves a bracket
answering busy until its bound expires nor a written one answering idle. State
then comes from the pane: a harness that stopped goes idle, and one that
ignored the key stays busy and unreachable. Whether the harness stops remains
the harness's verdict. A collar that declares no interrupt key refuses the
command, and an occupied composer or tmux mode refuses it too — that keystroke
is often what a native dialog or copy-mode reads as an answer.

With `-m`, Gangline attributes the reason like an ordinary message, holds one
pane lock across the stop key and the boundary it creates, then delivers the
reason before releasing that lock. The reason is never spooled: a queued stop
would arrive after the work it was meant to stop. If the composer does not
return or delivery cannot be verified, the reason is printed back to the caller
in full and reported as not delivered and not parked. Existing backlog remains
untouched for the next cooperative tick or native delivery opportunity.

Omitting the name stops the calling window's own turn, with or without a
reason. A reason that returns to its own author is refused on `send` and
intended here: it is written to be read after the turn it ended. Gangline
cannot promise that delivery, because the caller runs inside the turn it is
stopping — a harness that ends that turn by killing the tool call takes the
sending process with it before the reason is typed. Whether it survives is the
same harness verdict the stop itself is.

### `gang compact [<name>] [--resume <turn>]`

Submits the collar's native compaction command through the same verified input
path. An external request refuses a busy or unknown target.

A continuation turn is typed immediately behind the compaction command, enveloped
from the reserved sender `gangline`. A harness that is compacting parks it and
submits it when the compaction ends; a harness that refused the compaction takes
it at once. `--resume` supplies that turn, and a default fires when the agent
supplies none.

When a hooked Codex or claude-code agent requests its own compaction, Gangline
records a one-shot request. Its native Stop hook submits `/compact` after the
active turn releases the composer, then the continuation behind it. A later
cooperative tick also retries the request once pending mail is drained and the
same turn and composer gates prove it safe; copy-mode therefore cannot park the
request behind a boundary the idle recipient will never raise. `status` and
`roster` expose pending or failed self-compaction. A guarded claude-code launch
that cannot install hooks cannot witness deferred self-compaction requests from
inside that harness, but tick delivery remains available for requests already
recorded.

### `gang notify [<name>|clear]`

Declares, shows, or clears the optional agent that receives stall and unusable-state notes. The
target is stored as the session option `@gl_notify`; it need not exist when
declared, and no target is inferred. `gang notify clear` removes the declaration
sooner, and ending the tmux session deletes it with the rest of the team state.

When a collar's native hook witnesses an awaiting-input event, Gangline sends
one ordinary attributed message:
`stall: <agent> is awaiting input (<kind>) — inspect with gang capture <agent>`.
Repeated events of the same kind within 600 seconds are one note. A native
prompt, tool, or Stop event clears that debounce because it proves movement; an
older stamp permits a fresh note without any patrol or timer. A delivery failure
is recorded on the raising window for `status` and `roster`, and is retired only
after a later note is accepted live or parked.

An `idle_prompt` is only a wake: when its collar's current reader proves
`!blocked!` or `!bricked!`, Gangline sends one state note. `pane-died` may send
the corresponding `!dead!` note promptly; the cooperative `gang tick` also
reconciles dead panes and retained failed notes, so the durable guarantee is
delivery by the next tick. Notes accepted while the receiver is busy are parked
through the ordinary spool. A missing receiver leaves the exact pending state
and a visible `state note NOT accepted` record for later retry.

### `gang curfew [<duration|HH:MM>|clear]`

Declares, shows, or clears one optional wall-clock curfew for the team. Durations
carry their units, such as `90m`, `2h`, or `1h30m`; a bare number is refused.
`HH:MM` means the next occurrence of that local 24-hour clock time.

The declaration creates two relative advisory edges: yellow halfway through its
span and red after four-fifths. Native prompt and tool hooks expose each edge once
to hook-enabled agents. There is no default, per-agent budget, patrol, or automatic
action. Declaring again replaces and restarts the span; `clear` removes it.

## Observation

Gang-managed tmux windows wrap the bare agent name in the glyph of the state
Gangline last witnessed: `-name-`, `~name~`, `!name!`, or `?name?`. This is an
at-a-glance hint and can be stale between existing observation points and native
hook events; `gang roster` remains the live-computed truth. Both occupied and
bricked use the snagged-line `!` glyph because either needs operator attention;
the live roster word distinguishes them. Addressing always uses the bare name,
so `gang send --to pii-impl` never changes. tmux appends its own flags after the
name: a last-active busy window renders as `3:-name--`, where the trailing pair
is tmux's flag rather than part of the agent name. Gangline does not set the
operator's tmux status formats.

### `gang mail [name]`

Prints every message waiting in that agent's spool, oldest first, then every
held entry, each with its sender and its entry filename, each body exactly as it
would go onto the wire. A held entry says which kind it is: one Gangline typed
and could not confirm warns its reader they may have seen the body already. Another agent's or the operator's read is inspection and
touches nothing. The addressee's own read is delivery: it consumes each waiting
entry so a later native delivery opportunity cannot deliver the same message again. Before
printing an entry, it moves it into a human-readable directory under
`GANG_ARCHIVE_DIR`; the path and its explicit deletion command go to stderr, so
an ordinary stdout filter cannot destroy the only copy or hide its recovery
location. Held entries are never consumed. Mail takes no delivery lock and needs
no loadable collar, because a queue is files on disk and the harness may be the
reason you are reading it.

Read archives are durable and have no automatic expiry. Their stderr notice
names the read-scoped root and the exact command that deletes it. Delete that
root after its recovery or audit purpose ends; see
[Operations](operations.md#sending-messages-safely) for retention.

### `gang wait <name> --until idle|done [--timeout <seconds>]`

Blocks the caller on a native target boundary without polling `status`.
An already-idle target returns immediately for either condition. Otherwise the
target collar must declare `GANG_STOP_HOOK=1`, and the window must already carry
readable native turn evidence rather than relying on that declaration alone.
Each caller owns one temporary, uniquely named tmux `wait-for` channel and a
sparse caller-owned hook key. The target's next native `Stop` signals it after
closing the turn bracket. `idle` then re-reads live state and arms another
boundary while positive busy or occupied evidence remains; `done` returns after
that first Stop.

`done` does not track turns. If the target is already working, that active
turn's completion may match. A wait is an explicit barrier chosen by its caller,
not a supervisor: Gangline starts no daemon and records no durable state. Each
boundary has a foreground fail-loud deadline. `--timeout` chooses its positive
whole number of seconds; without the flag, `GANG_TURN_LIMIT` supplies the bound.

The registration pins both the target's window id and active pane id. A missing,
replaced, ambiguous, or pane-switched target fails loudly rather than transferring
the wait. `?unknown?`, `!bricked!` and `!blocked!` fail instead of hanging. Natural
pane-process exit plus Gangline's `drop` and `down` release temporary
registrations, so those teardown paths are observed and reported as a vanished
target. Direct `tmux kill-pane` and `tmux kill-window` do not emit the supported
exit hook on tmux 3.2a and bypass Gangline's compensating release. The foreground
deadline then refuses loudly and removes the temporary hook; use `gang drop` or
`gang down` to observe teardown immediately.

The implementation uses only `wait-for -S`; tmux channel locks are not used
because a dead lock holder can leave them locked indefinitely. A successful
wait consumes its native signal exactly once; cleanup never signals and waits
on the channel again, because that can race the returning waiter and block
forever. The deadline is a Python one-shot alarm supervising one fresh
Bash/tmux process group; Python 3 is already required by Gangline, and no GNU
`timeout` command is assumed. Deadline and foreground signals kill and reap
that exact group before caller-owned hook cleanup. Tmux has no channel-delete
operation, so a caller killed or bounded at the same instant as a native signal
can leave an unreachable nonce-named latch in tmux memory until that server
exits. No wait option, file, or daemon is created; the supervisor lives only as
long as the foreground command.

### `gang explain <name>`

Runs the ordinary live state classification once and prints the agent, pinned
active pane, collar, and resulting state. It then reports the optional collar
fatal-turn reader and both collar-owned state rules, `GANG_OCCUPIED_REGEX` and
`GANG_BUSY_REGEX`, as matched, did not match, not declared, not evaluated
because higher-priority evidence settled that part of the classification first,
or could not determine. Matched fatal evidence includes its cause. A matched
regex includes its exact ERE and the first pane line that matched.

The diagnostic instruments the regex evaluations inside that same state read.
It does not recapture the pane afterward, so a moving TUI cannot make the
explanation describe a screen different from the one classified. Regex capture
targets the pinned pane rather than following a later active-pane selection.
The command writes no diagnostic option or file; ordinary state observation may
still refresh the window glyph and the transient evidence that `status` itself
maintains.

### `gang status [name]`

Prints one current state:

- `-busy-` — a native event, terminal activity, or collar marker
  witnesses active work;
- `~wait~ (...)` — the last native turn boundary witnessed a harness resource
  still held for background work;
- `~idle~` — the evidence positively witnesses readiness;
- `!occupied! (authority unknown)` — a native UI owns the composer;
- `!dead! (nothing is running in this window)` — every pane in the held window
  has exited;
- `!bricked! (...)` — collar-native fatal evidence says the current session's
  turns cannot succeed until its cause is repaired;
- `!blocked! (...)` — collar-native evidence says the turn this window was
  given ended without producing work, and none is coming unattended; its
  composer is free, so nothing else gang reads can see this;
- `!harness-lost! (...)` — a collar-recorded harness root process is gone or
  replaced while its tmux pane remains alive;
- `?unknown? (...)` — the available evidence can no longer determine the answer.

Waiting is event-derived rather than a patrol. At Stop, an optional bounded
collar probe may name a live child shell, an unfinished native task record, or
an armed native continuation; Gangline records that held resource. The next
recognized native event retires the record, while hook silence leaves it
honestly stale. Mere intent to return later holds nothing and remains idle.
The waiting and idle window names share the slack `~name~` glyph; the state word
in `status` and `roster` carries the distinction.

Blocking evidence is checked after fatal evidence and before ordinary busy
paint: unrecoverable outranks recoverable, so a window that is both is reported
as the one that cannot be repaired in place, and a turn-ending record means a
busy marker still on the screen is retained paint. A blocked window refuses
delivery and names its reason rather than accepting a paste nothing will act
on; the message stays spooled and drains at that window's next real turn
boundary. Gang answers no dialog, retries no turn and drops no window for it.

Fatal evidence is checked after native UI occupancy and before ordinary busy
paint. That lets an operator-owned recovery UI remain occupied while preventing
an instant fatal turn from masquerading as work. A collar reader has three
outcomes: matched with a cause, absent, or unknown with a cause. Unknown remains
`?unknown?`; it is never spent as either a fatal match or a clean session.
The claude-code collar seeks backward to the newest complete top-level semantic
transcript record. An unterminated final JSONL append is not yet a record;
complete malformed records remain unknown. `isMeta` local-command notices and
tool-result-only user records are not turns. A selected-model API error is
fatal, a newer real user turn proves recovery has begun, and other API errors
such as rate limits are not this fatal shape. Selected-model failures are also
ineligible for automatic continuation because replay cannot repair the choice.
Two provider-side classes are fatal: an exhausted retry sequence, which leaves
`error=server_error` with `apiErrorStatus=529`; and a response stream that died
mid-turn, which leaves `error=server_error` with no `apiErrorStatus` key at all.
The absent key is the discriminator, not the sentence — the same record has been
observed saying the response stopped arriving, that the server errored
mid-response, and that the connection was lost. Any other `server_error` status
remains nonfatal.

A turn bracket left open by an interruption the harness never reported decays
once it passes `GANG_TURN_LIMIT`: an expired bracket over a quiet, stable pane
whose input box is on screen and provably empty reads `~idle~`, because that is
the same positive evidence idle means everywhere else. A drafted box, a frozen
busy marker, or a pty whose quietness cannot be measured keeps it `?unknown?`, and
an unreadable or future-stamped bracket is unknown rather than abandoned. Within
its bound the bracket still outranks the tiers beneath it, and a provably empty
box already accepted delivery while the state read `?unknown?`.

A closed bracket remains a native turn boundary after its age bound. If later
TUI chrome is still writing to the pane while the harness's own composer is on
screen and empty, the closed boundary and current composer agree on `~idle~`.
A draft, absent composer, or unreadable composer keeps the state `?unknown?`;
recent paint alone supplies no new turn boundary.

A decay refused because the pane was being written to during the decision reads
`?unknown? (the pane was written to while gang was deciding)`, and that reason
refuses delivery rather than falling through to the empty-box check. A harness
can paint the opening of a turn with its composer still empty, so an empty box
read out of a moving screen does not witness an idle target.

Under the state, `status` prints two ages and no verdict on them: when the
harness last recorded a tool call, and when the pane was last written to. A
delivery can be verified into a pane and a turn can open and close without the
recipient running a single command, and neither the state nor delivery
verification can see that. The tool-call reading has four outcomes and they are
kept apart: an age, `more than <age>` when a scan bound was reached first, `none`
when the source was read whole and holds no tool call, and `UNKNOWN` with the
reason when no reading could be taken. A collar that declares no source is
`UNKNOWN`, never `none`. A source stamped in the future is `UNKNOWN` naming the
clock disagreement rather than a tool call a moment ago.

Neither age is a health state and `status` says so. A long-running tool and a
stalled agent both go quiet; which one this is belongs to the reader.

`roster` carries the same tool-call reading as `last-tool=<age>`,
`last-tool><age>`, `last-tool=none`, or `last-tool=?`. The pane-write age
appears on a row only in the last of those, where it is the only evidence left.

An undelivered-input report is followed by a `box:` line classifying what is in
that box now, in the same vocabulary a refusal uses. The record says what
Gangline did; the `box:` line says what is there at reading time.

It also reports staged input and the current box reading, pending or failed
self-compaction, the number of messages spooled for that target and how long
the oldest has waited, held-message
details and their directory, a spool drain that could not be verified, a stall
note that could not be accepted, live native-session loss, the last cooperative
tick failure, and binary
skew when the window has no hitch/adopt stamp or its executable-byte witness
differs from the invoked `gang` binary. An unavailable witness is reported
explicitly instead of treated as either match or mismatch.

### `gang whoami`

From inside an agent pane, prints the Gangline agent name, pane id, collar,
stamped harness session id, latest live hook-payload id, team session, and any
recorded identity mismatch. A native hook stamps `@gl_session_id` on first
sighting and compares every later live id against it. The cooperative tick also
records a collar's independently observed live id. A mismatch is visible as
`session-lost` in status and roster and blocks all injection and spool drains
until the intended native session is re-established and its identity matches.

### `gang context [name]`

Prints the target collar's native context reading raw, in its own
`usedk/windowk (percent%)` format. The query reads the collar source whether or
not context lights are enabled; asking and edge-triggered signalling are
separate acts. A missing or unreadable source fails loudly and no value is
fabricated.

Codex context becomes available when a native hook payload binds its
`transcript_path`; this works equally for hitched and adopted windows once a
hook has fired.
opencode and Pi read their panes and can answer with lights off when their native
readout is visible. claude-code can answer only when lights were enabled at
hitch, because that launch choice installs its statusline beacon. Adopted
windows answer only when their required native source is independently present.

### `gang limits [name]`

Reads the non-interactive provider-limit source declared by the target's collar.
It prints one native window per line with percent used, the reset in local time,
and the age of the native sample. The command may target an agent mid-turn
because it never drives that agent's composer.

Claude Code's collar runs the harness's headless usage command behind a portable
50-second process bound, so a wedged reader makes the source unavailable before
Claude's native hook bound. A missing or incompatible `timeout` command is
reported by name. Codex's collar reads the exact target rollout stamped by its
native hook and selects the newest rate-limit event. That event's timestamp is
the sample clock, so an idle Codex reading truthfully grows stale until its next
API turn. It remains visible to
this reporting command even after it is too old to drive a warning or wake. A missing source, failed
reader, malformed row, missing reset, or missing observation time fails loudly.

A successful read records the most constrained native window on the target's
tmux window. `status` and `roster` report that ephemeral evidence without
sampling again; it dies with the agent window.

### `gang wait-limit [name] [--resume <turn>]` / `gang wait-limit [name] --clear`

Reads the same non-interactive provider limits as `gang limits`, chooses the
highest percentage used (later reset on a tie), and schedules one transient
systemd user timer for that reset. The optional turn is delivered after reset;
otherwise the continuation only reports that the window reset and asks the
agent to re-read its assignment.

A collar may declare a maximum actionable age for warning evidence. Codex
marks an event more than five minutes old stale until its next API turn, but a
future absolute reset remains sufficient to arm a wake without spending quota
on a refresh. Claude's headless read is fresh on demand.

The timer invokes `gang wait-limit <name> --fire <reset> --unit <unit>` with the
team's pinned session and configuration environment. Firing uses ordinary
attributed, verified delivery, including the ordinary spool when a target is
temporarily busy. A firing does nothing unless the agent still exists and its
stored declaration names the same reset and the same unit. The unit name is
derived from the session, the window id and the reset, so that comparison
refuses a timer that outlived its tmux server whenever any of the three
differs — it does not refuse one whose team was recreated holding all three.
The declaration is stored before `systemd-run` arms the timer, after the
transient unit derived from that name is stopped and proven gone. An older
callback therefore cannot consume the new declaration, while a reset arriving
during arming finds and delivers it. A failed declaration write leaves nothing
armed; a failed timer arm clears both declaration options and refuses.

The unit is collected after it runs. `--clear` stops the pending timer and the
service that timer may already have started, and it removes the declaration
whether or not those stops succeeded: a unit that already fired and was
collected is not loaded, and `systemctl stop` reports that as an error. A unit
still active after the stop is reported as an error naming the exact
`systemctl --user stop` to run, with the declaration already cleared. Drop and
down attempt the same cleanup. There is no watcher process.

The pending reset, transient unit, and optional continuation live as tmux window
options and die with the window. Status and roster expose pending, overdue,
unreadable, and failed wake states. A user manager or timer creation failure is
an error and leaves no wake declaration.

`status` reads the declared unit as well as the declaration whenever the reset
is still in the future. The declaration lives on the tmux window and the timer
lives in the user manager, so a manager restarted under a surviving tmux server
leaves a declaration promising a wake nothing will deliver. A unit that reports
inactive or failed is named as armed nowhere; a state that cannot be read at all
is reported unverified rather than assumed. A reset that has already passed is
not checked, because a fired unit is expected to be gone.

### `GANG_AUTO_RESUME`

An operator-declared provider-used percentage arms the wake above without anyone
typing `wait-limit`. It is off unless declared, is stamped on the window at
hitch or adopt time like the usage lights, and is a separate declaration from
them: warning an agent and arming its return are different decisions at
different percentages, and an agent may run auto-resume with the lights off.

At the first sample at or above the threshold whose reset is still in the
future, the agent's own turn hook passes that validated sample through the same
arming transaction as `gang wait-limit` and reports the result in the same
turn. It takes no second native reading while arming. Arming happens once per
provider window: the reset that was decided for is recorded on the window, so
later samples in the same window arm nothing, and a wake cleared with `--clear`
is not armed again over the operator. An existing manual wake is also
authoritative: automatic sampling records its provider window as handled but
does not stop its timer or replace its optional `--resume` body. A refused arm
is reported in the turn, recorded where status reads it, and not retried for
that window. An overdue declaration, or a future declaration whose timer is
provably inactive or failed, is dead residue and is replaced; the successful
advisory says when that replacement discarded a custom `--resume` turn.

A reading too old to act on arms nothing. A collar may cap the age of
actionable evidence, and Codex's percentage comes from a session event that goes
stale while the agent is idle. A stale percentage is poor evidence for a
threshold decision even though a still-future absolute reset remains sufficient
to arm against once that decision has been made, so the sample is refused and
the agent is told, rather than arming from a number that may no longer hold.

On claude-code this declaration also opts into one-hop recovery from a provider
stream failure. `idle_prompt` supplies the native idle witness and transcript
path; the collar selects the newest non-sidechain assistant record and returns
its UUID only when `error` is nonempty and `isApiErrorMessage` is true. Gangline
then closes the missing turn bracket and submits one continuation for that UUID.
The exact attributed envelope is recorded before submission and must match the
next native prompt byte-for-byte. If that owned continuation fails, or ownership
cannot be established, no next continuation is sent; `status` reports
`automatic resume refused` and roster reports `auto-resume-failed`. A later
ordinary native prompt clears the one-hop episode but preserves that refusal
record until a later automatic continuation is positively owned or the window
is dropped. Collars without both native readers do not attempt stream-failure
recovery.

This is deliberately an over-approximation of the provider limit. Gangline
cannot observe the harness's own refusal — the only in-band evidence is pane
prose, which is not a data contract — and an agent that has been refused takes
no further turns, so the last moment Gangline can arm from the agent's own hook
is before the cap rather than at it. A continuation may therefore arrive at a
reset for an agent that never capped; it carries the ordinary continuation turn
and lands on the ordinary spool. If a provider window goes from below the
threshold to exhausted between two samples, nothing is armed, and status shows
no pending wake; the threshold is the operator's lever, as it is for the lights.

### `gang roster`

Prints every session window with its collar and current state. Unadopted windows
are shown but not treated as agents. Context is deliberately not a roster
column: ordinary adopted windows and lights-off claude-code windows have no
readable source, and one absent value must not make the team inventory fail.
Use `gang context <name>` when a reading is wanted. Each row compares the
window's binary stamp with the invoked `gang` binary and visibly marks skew; the
comparison runs only for this snapshot. A non-empty queue reports both its depth
and the age of its oldest waiting entry.

Every agent in the team is listed, including the ones after an agent whose pane
would not answer. A state Gangline could not read is that one agent's fact: its
row carries `?unknown?` and the marker `state-unreadable`, the refusal naming
the reading it could not take goes to stderr immediately above that row, and the
command then exits nonzero. Roster is the check run before `gang down` or `gang
drop`, so a listing that stopped at the first unreadable pane looked from
outside exactly like a smaller team.

The exit status reports whether every row's reading could be TAKEN, and nothing
more. A reading that was taken and did not settle is an ordinary `?unknown?`
row with its own witness named in the phrase, and roster exits 0 on it, exactly
as `gang status` does for the same agent. Both unknowns are unknowns to whoever
is deciding: read the rows, not the status.

`gang roster --porcelain` is the scripting interface. It prints one unpadded,
uncoloured TSV row per window with these columns in order: `name`, `collar`,
`state`, `spooled`, `oldest_age_s`, and `session_id`. State is one lowercase
word: `busy`, `waiting`, `idle`, `occupied`, `dead`, `bricked`, `session-lost`, or
`unknown` for the human states. `unknown` covers both a state Gangline determined it could not
settle and one it could not read at all; the human row separates them and the
porcelain word does not.
unadopted windows and missing collars read `unadopted` and `collar-missing`.
`spooled` is an integer. `oldest_age_s` is integer seconds or `-` for an empty
queue or an unreadable age. A row Gangline could not produce at all falls back
to the name, the collar, `unknown`, and `-` in every remaining field. `session_id` is the exact stamp or `UNSTAMPED`.
With no running session it prints no rows and exits successfully, like the human
roster.

### `gang capture [name] [lines]`

Prints the tail of the target pane after trimming trailing blank terminal rows.

### `gang composer [name]`

Prints what a human actually typed into the agent's input box, via the
collar's styled reading. `capture` shows the raw pane, where a harness's dim
suggested-prompt placeholder is indistinguishable from a real draft; `composer`
strips styling, so placeholder text vanishes. Empty output means an empty box —
a whitespace-only reading (prompt padding) counts as empty, the same rule
delivery uses. Fails loudly when no input box is on screen or the collar
declares no `collar_input`.

## Discovery and hooks

### `gang collars`

Lists shipped and custom harness collars as two tab-separated columns: the
collar name, and what an agent hitched on it can be resumed onto. `resume` means
the collar declares both halves resuming needs — a launch line with a session
slot, and a `collar_session_id` that witnesses the native id to put in it.
`resume-id-only` means it declares the launch and no witness: `hitch --resume`
relaunches onto an id the operator supplies, while the bare form has nothing to
read and `gang drop` prints `UNSTAMPED`. `no-resume` means it declares no resume
launch at all, so `hitch` takes no `--resume` for it — a collar in this class may
still WITNESS its harness's id, and `gang drop` then prints that id as a record
of the session rather than as a relaunch command; `unknown` means the collar
would not load. The capability is marked here because the harness is chosen
here: `gang drop` reporting `UNSTAMPED` afterwards is too late to inform the
choice it should have. The Bash substrate fixture is hidden unless
`GANG_TEST_COLLARS=1`.

### `gang models [-c <harness>]`

Reads the selected collar's native model catalog and prints deterministic,
sorted TSV. The first field is the exact model id accepted by `gang hitch -m`;
an optional second field is that model's comma-separated reasoning-effort
vocabulary. Codex reads its JSON catalog, OpenCode uses its native model-list
command, and Pi parses the provider and model columns of its native table.
Failed, empty, duplicate, or malformed native output fails loudly and is never
partially printed as a catalog.

Some harnesses expose recognition but no complete list. Such a collar may
declare documented aliases: `gang models` prints those aliases and says on
stderr that full names cannot be enumerated. Its native recognition check can
still refuse an unrecognized `-m` before hitch, but it cannot prove provider or
account access. The claude-code collar has this shape.

### `gang roles`

Lists every discovered Markdown role file as three tab-separated columns:
name, origin, and status. Operator roles show their terminal-safe path; shipped
roles use the stable origin `shipped`. A winning file is `ok` only when it is a
nonempty usable role at that instant. Invalid names and unusable files remain
visible with their specific defect, and only `ok` names appear in an unknown
role refusal's accepted-value list.

### `gang config`

Prints every effective operator setting with its origin: built-in default,
config file and line, or environment, including when the environment overrides
a file line. It also reports whether the doctrine file and operator roles
directory are present, with the terminal-safe path to each slot. Dynamic text
is terminal-safe: control bytes are rendered visibly rather than written raw.
The command takes no arguments and needs no tmux server.

### `gang hook`

Internal endpoint for native harness events. It reads one JSON payload from
standard input and takes no arguments. An invocation carrying one is an
invocation Gangline cannot read: the argument is named on stderr and recorded on
the window for `status` and `roster`, and the event is not acted on. It is the
one command that does not die on unconsumed arity, because a hook must not be
fatal to the harness that fired it; declining the event is the refusal.
Prompt/tool events open the turn fact, Stop closes it and may
dispatch deferred self-compaction and a spool drain, and permission requests
raise occupancy. PreCompact opens the compaction bracket and PostCompact closes
it and drains. A harness that refuses a compaction raises the opening event and
never the closing one, so any turn event also settles a bracket left open.
Permission occupancy has no timed decay: a readable composer clears it, and a
malformed witness refuses rather than becoming ordinary absence.
Awaiting-input events listed by `GANG_STALL_TYPES`, plus permission requests,
raise a stall note only when `gang notify` has declared a target. A collar that
declares no `GANG_STALL_TYPES` raises none, because it has no verified
awaiting-input event to wire: an agent of that collar going idle — after a
self-compaction as much as after any other turn — tells nobody, and whoever is
waiting on it has to look. Read that as what the collar could establish, not as
proof the harness has no such event. Some harnesses enumerate their event set
and some expose no enumeration surface at all, and on those a silent binary is
an unreadable instrument rather than an absent event. Each collar records which
of the two it was in, and the operational consequence is the same either way.
Hooks are silent unless an enabled context light or declared team-time light
crosses an edge. Context-source warm-up is silent until the first native turn
completes; an unreadable source after that boundary fails visibly.

## Configuration

The precedence is environment variable, then config file, then built-in default.
An environment variable that is set but empty still outranks the file. Values
read from the file are shell variables, not exported into harnesses or helper
commands.

`GANG_CONFIG_DIR` is environment-only and defaults to
`${XDG_CONFIG_HOME:-$HOME/.config}/gangline`. The resolved directory must be
absolute. It contains:

- `config`, the optional settings file;
- `DOCTRINE.md`, optional operator prose delivered in every hitch contract;
- `CONTRACT.md`, an optional operator contract that replaces the shipped one;
- `roles/`, optional operator role briefs that replace shipped roles
  file-for-file by name.

The settings file is parsed, never sourced. Blank lines and lines whose first
non-blank character is `#` are ignored. Every other line is `NAME=VALUE`: leading
blanks are ignored, the first `=` separates the exact key, and trailing spaces
and tabs are removed from the literal value. Quotes, `#`, backticks, `$`, and
further `=` characters have no special meaning. Empty values, duplicate or
unknown keys, blanks in a key, NUL bytes, and control characters other than tab
and newline are fatal. A missing final newline is accepted.

Exactly these keys are settable:

| Key | Built-in default | Meaning |
|---|---|---|
| `GANG_COLLAR` | `claude-code` | default collar for `up` and `hitch` |
| `GANG_SESSION` | `gangline` | exact tmux session Gangline addresses |
| `GANG_COLLARS` | unset | custom collar directory searched before shipped collars |
| `GANG_LOCK_DIR` | `/tmp/gangline-$(id -u)` | shared delivery locks and per-target spools |
| `GANG_ARCHIVE_DIR` | `${XDG_STATE_HOME:-$HOME/.local/state}/gangline/archive` | pending-message archive written before windows die |
| `GANG_CONTEXT_LIGHTS` | `collar` | `collar` to take each collar's own default for the hitched model, `off`, `yellow,red` token thresholds, or `yellow%,red%` relative thresholds; `gang hitch -l` overrides it for one agent |
| `GANG_USAGE_LIGHTS` | `off` | `off` or increasing provider-used thresholds such as `90%,95%` |
| `GANG_AUTO_RESUME` | `off` | `off` or one provider-used percentage such as `97%` at which a reset wake is armed automatically |
| `GANG_SCOPE` | `off` | `off`, or `on` to launch each hitched harness, and the tmux server gang forks, in its own transient systemd user scope |
| `GANG_TMUX_GUARD` | `on` | `on` to put a `tmux` shim at the front of every hitched agent's `PATH`; it refuses commands that could be retargeted by an absent `TMUX_TMPDIR`, and refuses destructive commands when the server tmux resolves has live or unreadable `@gl_agent` registrations, or `off` to launch agents with an untouched `PATH` |
| `GANG_BOOT_TIMEOUT` | `30` | initial startup readiness bound; after a positively identified gate, one foreground observation slice in seconds |
| `GANG_GATE_LOOKS` | `60` | observations of an unanswered native first-run prompt before `hitch` stops waiting and exits 4 |
| `GANG_CHURN_WAIT` | `0.5` | stable-pane observation interval |
| `GANG_ACTIVITY_WINDOW` | `5` | recent terminal-activity window |
| `GANG_TURN_LIMIT` | `300` | native turn-fact bound and default `gang wait` boundary timeout |

Collar declarations are refused because `load_collar` clears them before
sourcing the selected collar; put those values in a custom collar and point
`GANG_COLLARS` at it. `GANG_TEST_COLLARS` is suite-only,
`GANG_CONFIG_DIR` cannot set the file that is already being read, and internal
variables are refused. Any malformed file refuses every command, including
native hooks; recovery is in `docs/operations.md`.

`hitch` also exports `GANG_TMUX_GUARD_LOG_DIR` into guarded agent launches. It
is the launch-time team log root, retained when a test redirects
`GANG_LOCK_DIR` so each teardown verdict remains attributable. It is internal
provenance, not a settable configuration key.

Doctrine is never written by Gangline. It must be a readable regular file with
no NUL, no controls other than tab and newline, and valid UTF-8. Byte count does
not predict deliverability: a doctrine the target cannot accept fails loudly at
hitch through the verified paste, while system-prompt prose meets the harness,
operating-system, and model-context boundaries that actually consume it. Delete
`DOCTRINE.md` to remove the slot; a hitched copy dies with its window.

The contract uses the same prose validation and the same operator-first
resolution: `$GANG_CONFIG_DIR/CONTRACT.md` before the shipped `CONTRACT.md`. It
is the one prose slot that is not optional — a root without one refuses every
hitch, naming both paths. Its bytes are never pasted: they ride the system
prompt where the collar declares that option, and where it does not, only the
path is sent. A role brief joins them in that prompt on the same collars.
Nothing downstream reads those bytes back, so Gangline applies only the prose
shape checks it can prove before launch. Their consumers apply the real size
bounds; Gangline does not substitute an unrelated byte threshold for those
interfaces.

A hitch may be role-less, but the contract is always present, so a role-less
hitch still carries one into the system prompt where the collar has that option.
The contract and the role brief share a single option rather than passing it
twice: a collar declares one spelling, and repeating it would be a guess about
whether that harness concatenates the values or keeps the last. The composite
is Gangline's preamble, then the contract, harness-specific guidance where the
collar declares it, and the role brief where one was named.

Role briefs use the same prose validation. Resolution checks
`$GANG_CONFIG_DIR/roles/<name>.md` before the shipped `roles/<name>.md`; an
operator file replaces the shipped file whole, and an unusable override refuses
instead of falling back. A named role must be nonempty. Where the selected
collar declares `GANG_ROLE_PROMPT_OPT`, Gangline passes the validated bytes by
value through that launch option, after the contract and under the same
preamble, and puts only an attribution pointer in the startup contract.
Otherwise it puts the body in the startup contract before operator doctrine.
The system-prompt form includes Gangline's own preamble stating that later
operator doctrine governs any disagreement.

Every process addressing one team must agree on `GANG_SESSION`, `GANG_COLLARS`,
`GANG_LOCK_DIR`, and the resolved `GANG_CONFIG_DIR`.

## Collar contract

A collar is a Bash file sourced by `bin/gang`. Harness-specific behavior belongs
there, never in a harness-name branch in the core script.

| Declaration | Purpose |
|---|---|
| `GANG_LAUNCH` | required native launch command |
| `GANG_RESUME_LAUNCH` | optional explicit native resume template containing exactly one `{{session_id}}` slot |
| `GANG_MODEL_OPT` | optional native model flag |
| `GANG_MODEL_ALIASES` | optional newline-separated documented aliases when the harness has no complete catalog; discovery labels them as incomplete |
| `collar_models` | print the complete native catalog as `model-id[<TAB>comma-separated-efforts]`; nonzero, empty, duplicate, or malformed output is unknown and refused |
| `collar_model_check model` | when no complete catalog exists, return 0 recognized, 1 unrecognized, or 2 unknown; recognition is not account availability |
| `GANG_EFFORT_OPT` | optional native effort option, declared whole with its separator; the level joins with no space |
| `GANG_EFFORT_CMD` | prints the effort vocabulary, one level per line, given `GANG_MODEL`; empty output means could-not-determine |
| `GANG_ROLE_PROMPT_OPT` | optional native option whose next argument is a system-prompt addition passed by value |
| `GANG_HARNESS_PROMPT` | optional harness-specific prose included in that system-prompt addition; requires `GANG_ROLE_PROMPT_OPT` |
| `GANG_BUSY_REGEX` | pane evidence of an active turn |
| `GANG_OCCUPIED_REGEX` | pane evidence that a native UI owns input |
| `collar_bricked target` | inspect native fatal-turn evidence; print a cause and return 0 fatal, return 1 with no output when absent, or print a cause and return 2 when unreadable |
| `collar_blocked target` | inspect native evidence that the turn this window was given ended without producing work; print a reason and return 0 blocked, return 1 with no output when absent, or print a cause and return 2 when unreadable. Declared by the `claude-code` and `codex` collars. Declare it only where the harness exposes such evidence; a collar that declares nothing simply cannot answer, which `gang explain` reports as `not declared`. A turn still in flight is absent, never blocked — a running turn and a harness that died inside one are indistinguishable from a transcript, so that case is left to process liveness rather than claimed here |
| `collar_waiting target payload` | optional bounded Stop-time probe for native background resources; print the held-resource witness and return 0 waiting, return 1 with no output when none is held, or print a cause and return 2 when the native sources are unreadable. The result is cached only until later recognized hook traffic; intent without a held resource stays idle |
| `collar_last_action target` | optional; print `at <epoch>` for the newest tool call the harness recorded, or `before <epoch>` when a scan bound was reached first and the newest call is older than that time. Return 0 having printed one of those, 1 with no output when the source holds no tool call at all, or print a reason and return 2 when no reading could be taken. A collar that does not declare it leaves the reading unknown, which is a distinct answer from an agent that has run nothing |
| `GANG_QUEUED_REGEX` | input-box evidence that the harness parked input in a native queue instead of submitting |
| `GANG_QUEUE_RECALL_KEY` | tmux key name that loads the parked message back into the composer, used by `flush` |
| `GANG_INTERRUPT_KEY` | tmux key name that stops an active turn, used by `interrupt` |
| `GANG_STOP_HOOK=1` | the launch command installs a native Stop hook reaching `gang hook`, so this harness supplies an immediate native turn boundary; cooperative ticks retry spools independently |
| `GANG_STALL_TYPES` | space-separated native `Notification` kinds that mean the harness is awaiting a person |
| `GANG_QUIET_AT_REST=1` | harness terminal becomes quiet when idle |
| `GANG_MIDTURN_INPUT=1` | ordinary text may safely enter during a turn |
| `GANG_MIDTURN_INPUT=park` | declares that mid-turn composer input can only stage or queue; it never authorizes a composer keystroke |
| `GANG_MIDTURN_INPUT=steer` | commit to the attributed spool first, then allow a free composer to accept its claim as native mid-turn steering; PostToolUse supplies later opportunities |
| `GANG_COMPACT_CMD` | native compaction command |
| `GANG_SELF_COMPACT=deferred` | self-compaction must wait for Stop |
| `collar_usage_limits target` | print `label<TAB>percent-used<TAB>reset-epoch<TAB>observed-epoch` rows from a non-interactive native source; absence declares provider-limit awareness unavailable |
| `collar_usage_limits_error status` | optionally explain a collar-specific native-reader failure status; Gangline sanitizes and surfaces it in hooks and explicit commands |
| `GANG_USAGE_LIGHT_INTERVAL` | minimum seconds between hook-driven native usage reads; zero disables reuse, while explicit commands remain fresh |
| `GANG_USAGE_LIMIT_MAX_AGE` | maximum seconds a native sample may drive a light; zero accepts any age before its reset |
| `collar_input target` | print human-authored composer contents; 1 when the harness has drawn no composer, 3 when the pane itself could not be read, and a collar may declare further statuses only when Gangline has learned their meaning. `claude-code` declares 4 for a selected in-process subagent composer and 6 for the background-sessions composer that creates a new session; Gangline carries both through as named refusals rather than flattening them into absence. Any status Gangline does not recognise is normalized to a loud status 5 unknown that preserves the raw value, never absence |
| `collar_overlay target` | optional; print the visible title of a native overlay dialog owning the screen, return 1 when none is recognised, and 3 when the pane could not be read. Naming only: Gangline quotes it in a refusal a delivery already reached, so an absent reader, an unreadable pane and an unrecognised frame each cost the sentence and never the decision |
| `collar_context_lights model` | optional; print this collar's default `yellow,red` or `yellow%,red%` thresholds for that model, or return 1 with no output where it has no default for it. Consulted only where the collar also declares `collar_context`. A malformed spec, or any other status, is refused under the collar's name rather than the operator's setting |
| `collar_context target` | print `usedk/windowk (percent%)`; return 2 when a readable native frame transiently carries no readout, or otherwise fail loudly, keeping a refused pane read distinct from both |
| `collar_session_id target payload` | print the exact native session id witnessed by a hook, or fail without fabricating one |
| `collar_live_session_id target` | optional independent probe of the native session currently holding the pane; print its exact id, or return nonzero when no safe reading is available. The cooperative tick compares it with the registered id and treats a contradiction as session loss |
| `collar_harness_identity target` | optional positive root-process witness; print `pid<TAB>kernel-start-stamp` and return 0 only when the collar can demonstrate the pane root is its live harness, return 1 when no identity is recorded, or return 2 with a cause when unreadable. Gangline records it only at hitch/adopt and tick treats a later missing or changed witness as `!harness-lost!` |
| `collar_auto_resume_record target notification-kind` | optional native failed-turn discriminator; print one stable error-record identity, return 1 for an ordinary idle turn, or return 2 when the native record cannot be read |
| `collar_auto_resume_prompt target payload` | print the exact native prompt from a prompt-submission event so Gangline can prove whether its marked continuation owns that turn |

The shipped Codex live-id probe asks tmux to run in the server's host namespace,
then inspects the active pane process's open descriptors through `/proc`. It
accepts an id only when exactly one value is shared by a
`thread-writer-locks/<id>.lock` descriptor and that process's rollout JSONL.
The bounded helper writes through a cleanup-owned temporary file because the
calling agent's sandbox may be unable to read host `/proc` directly. Ambiguous,
missing, or unreadable evidence is a loud probe failure, never a guessed id.

Collars may install native event hooks by composing them into their launch
command. They must not weaken sandboxing, approvals, or operator permissions.

### Codex hook trust boundary

Codex trusts a native hook by its event and command string. It does **not**
attest the contents of the executable named by that command. Thus trust once
granted for Gangline's Codex Stop hook covers later in-place edits to
`collars/plugins/codex-stop-hook.py` indefinitely. Changing the configured
command or its path creates a new hash and requires native re-trust.

This is trust-on-first-use over a name, not over the code that runs. Anyone
able to edit that helper can change what executes at every Codex turn end on
this machine without a new Codex prompt. That grants no capability beyond a
writer who can already alter the Gangline checkout and its collars; it is a
boundary to state, not a sandbox Gangline claims to provide.

When Codex has not trusted the current command, Gangline refuses the hitch
before opening a composer and holds the window with this final line (with its
live count and directory substituted):

```
gang: N codex hook(s) are untrusted here — run the codex line above in CWD once, answer 'Trust all and continue', then re-hitch (this held window carries the full list: gang capture <name> 40).
```

The refusal prints the exact `codex` command above that line. From a terminal
in that same working directory, run that command, choose Codex's native
**Trust all and continue**, then re-run `gang hitch`. The native menu requires
a terminal; Gangline deliberately cannot grant trust. There is no Gangline
pre-hitch provisioning command: the held refusal is the once-per-machine,
operator-visible recovery path. An edit that changes the configured command or
path can therefore cause every later Codex hitch on that machine to refuse
until a person completes those steps.

A pane reading Gangline could not take is unknown, and unknown is neither an
absent composer nor a settled one. Reporting it is the collar's obligation:
status 3, produced by reading the capture into a variable and checking that
status before any parser sees the bytes. Piped straight into one, a refused
capture arrives as that parser's verdict on empty input and is indistinguishable
from a pane carrying no composer — and a collar that loses the difference there
cannot have it restored downstream. The same rule binds every other pane reading
a collar takes. `collar_context` spends nothing, since the command ends either
way, but a refused capture reaching its parser makes it name a missing context
readout on a pane nobody read; it reads into a variable too, and refuses with a
status of its own. Gangline asks the pane directly where a
collar answers 1, but that probe can only turn an absence back into an unknown
when the transport is still refusing; a pane that answers proves the transport
is up now and says nothing about the read that already happened. With such a
collar, a refusal that heals in between is read as an absent box by the
predicates that look once — occupancy, and `gang composer`.

Nothing spends a single absence as permission to type. The settled check takes
two looks and compares their statuses as well as their contents, so a box drawn
for one look and absent for the other is refused whichever way it moved: a
harness painting or dropping its composer and a collar reporting a refused read
as an absence both arrive here, and neither is a box nobody is typing into.
Predicates with no room for a third answer — occupancy, busy or idle, an unmoved
decay witness — refuse out loud and name the reading they could not take rather
than guess.

Gangline does not answer native dialogs. A screen matching
`GANG_OCCUPIED_REGEX` is occupied whoever drew it: `status` and `roster` report
`!occupied! (authority unknown)`, delivery refuses and parks, and `hitch` keeps
waiting and names the prompt for the operator. Answering one is `gang attach`.

A collar may refuse its own launch. Where the harness can be asked, before it
starts, whether it is about to gate its own boot on such a dialog, the collar
composes that check into its launch command and exits non-zero rather than
opening a window that will wait on a person. `hitch` reports it as a launch that
died and quotes the last line of its pane, so the check states its remediation
there; a check whose refusal must outlive the pane holds the corpse with
`remain-on-exit`, and `gang drop` clears it. Codex's collar does this for the
native hooks-review gate its own hooks raise. Such a check reads state and
refuses; it never grants a native trust or answers a dialog on the operator's
behalf.
