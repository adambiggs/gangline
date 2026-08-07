# Lead-ergonomics spec

Five operator-directed changes, each born from friction observed in a live
marathon session. This document is the implementation contract: it leaves no
design decisions open. Where a value must come from a live harness capture
rather than from this document, the requirement says so explicitly and gives the
capture procedure.

Read `CONSTITUTION.md`, `AGENTS.md`, and `CONTRIBUTING.md` first. They bind
every change here.

## Verified harness pins

Every claim below about harness behaviour was read out of the installed
binaries, not from memory or vendor prose.

| Harness | Version observed | How it was read |
|---|---|---|
| claude-code | 2.1.224 | hook registry and Notification construction site in the bundled executable |
| codex | 0.145.0 | embedded per-hook JSON schemas and the `HookEventNameWire` enum in the executable |

**claude-code 2.1.224** exposes a `Notification` hook. Its matcher field is
`notification_type`, whose complete value set is `permission_prompt`,
`idle_prompt`, `auth_success`, `elicitation_dialog`, `elicitation_complete`,
`elicitation_response`, `agent_needs_input`, `agent_completed`. The hook input
carries `hook_event_name: "Notification"` plus `message`, `title`, and
`notification_type`. Output is ignored on exit 0 (`stdout/stderr not shown`), so
a Notification hook cannot return `additionalContext` to its own agent — a note
raised there must travel as an ordinary Gangline message. `idle_prompt` is
raised by a timer whose threshold setting is `messageIdleNotifThresholdMs`, with
the message `Claude is waiting for your input`.

**codex 0.145.0** has no such event. Its complete hook set is `PreToolUse`,
`PermissionRequest`, `PostToolUse`, `PreCompact`, `PostCompact`, `SessionStart`,
`SessionEnd`, `UserPromptSubmit`, `SubagentStart`, `SubagentStop`, `Stop`. The
external notifier program the harness still carries is spelled `legacy_notify`
and fires `agent-turn-complete` with `thread-id`, `turn-id`, `cwd`, `client`,
`input-messages`, `last-assistant-message`. That is a turn-completion event,
which is what Gangline already consumes through the `Stop` hook, so it is not an
awaiting-input witness and nothing here wires it. Codex's only awaiting-input
witness is `PermissionRequest`, which the codex profile already installs.

This corrects the charter's premise: stall lights on codex cover permission
requests and nothing else, because codex 0.145.0 witnesses nothing else. That
gap is a harness fact to be stated, not a hole to be filled with a poller.

---

## 1. Contract truthfulness line

### Problem

The startup contract teaches one half of attribution — that peer messages name
their sender in the gang envelope — and leaves the complement unstated. An agent
that reads unenveloped text in its own pane has no supplied basis for
interpreting it.

### Change

In `bin/gang`, `startup_brief()` (two `printf` branches, doctrine-present and
doctrine-absent), insert one sentence immediately after
`Peer messages name their sender in the gang envelope.` and before
`To message a peer:`.

The sentence, byte-exact:

```
Gangline never delivers a message without one — the only unenveloped text it ever types into a pane is your harness's own compaction command — so any other unenveloped text arrived from the session keyboard, and Gangline cannot attribute it further.
```

Inside the single-quoted `printf` format the apostrophe is written `\047`, as
the surrounding sentences already do:
`your harness\047s own compaction command`.

The compaction clause is load-bearing, not hedging. `bin/gang` has exactly five
`inject` call sites: three carry `$ENVELOPE` (live send, spool drain, the hitch
brief itself) and two carry `$GANG_COMPACT_CMD` unenveloped
(`self_compact_after_stop`, `cmd_compact`). Without that clause the sentence is
false in the compaction case. Do not shorten it.

The sentence states mechanism only. It draws no conclusion about what an agent
should do with unattributed text; that is trust policy and it lives in the
operator's `DOCTRINE.md`, which every hitch already delivers.

### Tests

In `test/integration.sh`, beside the existing startup-contract assertions
(`startup carries the operator-authorized marathon rule`), add:

```sh
contains "startup states the complement of envelope attribution" \
  "$(pane alpha)" \
  "Gangline never delivers a message without one — the only unenveloped text it ever types into a pane is your harness's own compaction command — so any other unenveloped text arrived from the session keyboard, and Gangline cannot attribute it further."
```

Assert the same sentence against an agent hitched under
`GANG_CONFIG_DIR="$CONFIG_CASES/doctrine-present"`, so both `printf` branches are
witnessed. The doctrine-present fixture already exists in the suite.

### Documentation

`docs/reference.md` describes the startup contract; add the complement there in
the same words. No `CHANGELOG.md` edit.

### Commit

`feat(hitch): state the complement of envelope attribution`

---

## 2. Spool by default

### Problem

A dozen-plus live-delivery bounces off busy panes in one session. Each refusal
returns a full copy of the message body to the sender, who must then re-send it
by hand. Parking is already built, already verified, and already opt-in — the
default is simply the wrong way round.

### Flag surface after the change

```
gang send --to <name> [--from <sender>] [--live-only] [--supersede] --stdin
```

- **default** — attempt live delivery; on a refusal, park in the target's spool
  and report it as parked. On a failure after anything was typed, exit loudly
  and park nothing. Unchanged from today's `--spool`.
- **`--live-only`** — attempt live delivery and never park. This is today's
  no-flag behaviour, kept for availability probes.
- **`--supersede`** — drops the same sender's earlier spooled entries before
  parking this one. Now valid without a companion flag. Refused with
  `--live-only`, which never parks:
  `send: --supersede replaces an older message in the spool, and --live-only never parks one`
- **`--spool`** — accepted, does nothing, and says so on stderr once:
  `send: --spool is the default now and does nothing; drop it, and use --live-only for a delivery that must not park.`
  Exit status is unaffected.

`--spool` is deprecated rather than removed. Gangline is published, the flag is
documented in `docs/reference.md`, and `docs/DECISIONS.md` ("Unpublished renames
are complete") sets the precedent: after publication an external name is
preserved through normal deprecation. Announcing the no-op on stderr keeps it
from being a silent lie about what the flag did.

### The harnesses that cannot drain

A spool drains only on the target's own native `Stop` event. A profile that
declares no `GANG_STOP_HOOK` announces no turn boundary, so a message parked for
it would sit forever. Today `--spool` dies up front for such a target. Under a
default that parks, dying up front would break every ordinary send to
`opencode` and `pi`, whose profiles declare no Stop hook.

Correct behaviour: **for a target that cannot drain, the default degrades to
live-only, and the refusal names why nothing was parked.** The delivery attempt
still happens; only the parking is unavailable. The same applies to a window
carrying no spool identity, which `spool_write` today reaches as a fatal error.

Add beside the other spool helpers in `bin/gang`:

```sh
SPOOL_UNAVAILABLE_WHY=""
spool_available() { # $1 = window id, $2 = target name; 0 = a parked message would drain
  SPOOL_UNAVAILABLE_WHY=""
  if [ "${GANG_STOP_HOOK:-}" != 1 ]; then
    SPOOL_UNAVAILABLE_WHY="profile '$AGENT_PROFILE' declares no GANG_STOP_HOOK, so no native turn boundary reaches Gangline and nothing would ever drain a spool for '$2'"
    return 1
  fi
  if ! spool_existing "$1"; then
    SPOOL_UNAVAILABLE_WHY="the window of '$2' carries no spool identity — it is minted at hitch and adopt, so re-hitch or re-adopt '$2' before a refused message can park"
    return 1
  fi
  return 0
}
```

`spool_existing` still dies on a token Gangline did not mint. That is an
unaccountable directory, not an absent one, and it stays fatal.

### `cmd_send` control flow

Parse `--live-only` and `--supersede`; accept and warn on `--spool`; reject
`--supersede` with `--live-only`. After `resolve "$name"` and `envelope`:

1. If `--live-only`: call `send_live "$name"` at statement level so its refusal
   and exit status propagate unchanged, print the delivered line, return 0.
2. Otherwise capture: `err="$(send_live "$name" 2>&1)" || rc=$?`.
   - `rc` 0 — print the delivered line, return 0.
   - `rc` not 0 and not 3 — print `$err` to stderr and `exit "$rc"`. Unchanged.
   - `rc` 3 — continue.
3. On the refusal, evaluate `spool_available "$AGENT_ID" "$name"`. Evaluate it
   here, not earlier: the happy path should not read tmux options it will not
   use.
   - unavailable — print `$err` to stderr, then
     `printf 'send: NOT parked — %s. Send again when '\''%s'\'' is idle.\n' "$SPOOL_UNAVAILABLE_WHY" "$name" >&2`,
     and `exit 3`.
   - available — `spool_write` as today, print the existing `spooled for …
     NOT delivered` line.

The `GANG_STOP_HOOK` precondition moves out of the `--spool` branch entirely; it
now lives only in `spool_available`.

### Existing guards

Two assertions in `test/integration.sh` encode the opt-in default and must be
rewritten, not deleted. Their underlying claim survives; only the surface that
carries it changed.

- `a harness that announces no turn boundary refuses the spool` (with
  `naming the declaration a drain would need` and
  `and refusing the flag delivers nothing`). The old expectation — that the flag
  is refused before any delivery attempt — is wrong under a default that parks,
  because refusing before the attempt would refuse every ordinary send to a
  no-Stop-hook harness. Rewrite as the NOT-parked path below. The claim it
  guards, that Gangline never holds a message nothing would drain, is preserved
  exactly.
- `superseding without a spool is refused` / `because there is nothing for it to
  replace`. `--supersede` alone is now the ordinary parking form. Rewrite as the
  `--supersede --live-only` refusal, which is the same claim about the same
  emptiness.

### New tests

All fixtures refuse by the mechanism the suite already uses: a human draft typed
into the target composer with `tmux send-keys -l`. Every assertion reads an
artifact — the spool directory, the pane, or the reported count — never an exit
status alone.

1. **Parks with no flag.** Draft into `parker` (the existing `spoolable`
   fixture, `GANG_STOP_HOOK=1`), send with no flag, assert `spooled for parker`
   on stdout and `status parker` reporting one more entry.
2. **`--live-only` never parks.** Record the entry count, draft into `parker`,
   send with `--live-only`, assert it exits non-zero, that `status parker`
   reports the same count, and that the pane never received the body.
3. **A target that cannot drain is not parked, and says why.** Add
   `$RUN_ROOT/profiles/nodrain.sh` sourcing `profiles/bash.sh` with the same
   `GANG_LAUNCH` as `spoolable.sh` and **no** `GANG_STOP_HOOK`. Hitch it, draft
   into it, send with no flag, and assert: exit 3, stderr contains
   `NOT parked` and `GANG_STOP_HOOK`, the pane never received the body, and no
   spool directory exists for that window.
4. **`--spool` is an announced no-op.** Draft into `parker`, send with
   `--spool`, assert stderr contains `is the default now` and that the message
   parked all the same.
5. **`--supersede` stands alone.** Park two bodies from one sender, the second
   with `--supersede` and no other flag, assert `status parker` reports one.
6. **`--supersede --live-only` is refused**, naming `--live-only`.

### Documentation

- `bin/gang` `usage()` — the `gang send` line.
- `docs/reference.md` §`gang send` — rewrite the three `--spool` paragraphs to
  describe parking as the default, `--live-only` as the probe, the deprecated
  `--spool`, and the NOT-parked degradation with its reason. Present tense, no
  account of the change.
- `docs/DECISIONS.md` §"A refused delivery may be spooled, a failed one may not"
  — retitle to **"A refused delivery is parked, a failed one is not"** and edit
  the body so parking is the default and `--live-only` is the explicit probe.
  Delete `Spooling is opt-in per send,` and state instead that a profile whose
  harness announces no turn boundary degrades to live-only and names the missing
  declaration in its refusal. Leave every other sentence of that entry alone.
- `docs/operations.md` §"Sending messages safely" — one sentence that an
  unattended sender no longer needs to re-send a bounced message by hand.

### Commit

`feat(send)!: park a refused message by default`, with a `BREAKING CHANGE:`
footer naming the exit-status change (a refused send to a drainable target now
exits 0 as parked; pass `--live-only` for the old behaviour) and the `--spool`
deprecation.

---

## 3. Stall lights

### Problem

An agent waiting on input is invisible to everyone but whoever attaches. The
harnesses themselves witness it; nothing carries that witness anywhere.

### What may not be built

No patrol, no timer, no watcher (`CONSTITUTION.md` law 7, `docs/DECISIONS.md`
"Evidence is selected per predicate"). The only sources are native hook
deliveries. Gangline translates a fact it was handed and stops.

### There is no lead

`bin/gang` has no lead concept; `lead` is only the default name `gang up` gives
its first agent. Roles are coordination, which Gangline does not do. The notify
target is therefore an **optional operator declaration**, in the exact shape
`gang cutoff` already establishes: one session-scoped value, no default, no
inference, no enforcement.

### `gang notify`

```
gang notify <name>     declare which agent receives stall notes
gang notify clear      remove the declaration
gang notify            report it
```

Stored as `@gl_notify` on the session
(`tmux set-option -t "=$SESSION:" @gl_notify "$name"`). Validate `<name>` with
`valid_name` and refuse otherwise. Do **not** require the window to exist at
declaration time — the declaration and the hitch may arrive in either order,
as with `cutoff`. With nothing declared, `gang notify` prints
`no notify target declared` and exits 0.

Deletion path (law 6): the declaration is a session option and dies with the
session; `gang notify clear` removes it sooner. State that in
`docs/reference.md`.

Add the command to `usage()` and to the `docs/reference.md` command list.

### Profile surface

`profiles/claude-code.sh` — add `Notification` to the hooks JSON built for
`--settings`, beside the existing four:

```
"Notification":[{"hooks":[$_gl_cc_cmd]}]
```

and declare the awaiting-input value set as data:

```sh
# AWAITING INPUT IS THE HARNESS'S OWN WORD. Observed on claude-code 2.1.224:
# the Notification hook matches on notification_type, whose complete value set
# is permission_prompt, idle_prompt, auth_success, elicitation_dialog,
# elicitation_complete, elicitation_response, agent_needs_input,
# agent_completed. These four are the ones that mean a person is being waited
# on; the other four report something that finished. A value not in this list
# is not a stall — a renamed one stops raising notes rather than raising wrong
# ones, and re-verifying this list is what a version bump costs.
GANG_STALL_TYPES="permission_prompt idle_prompt elicitation_dialog agent_needs_input"
```

`profiles/codex.sh` — no wiring change; its `PermissionRequest` hook is already
installed. Add a comment recording the verified pin: codex 0.145.0's hook set
contains no Notification event, and its `legacy_notify` / `agent-turn-complete`
program reports turn completion, which the `Stop` hook already delivers, so it
is not an awaiting-input witness and is deliberately not wired.

`profiles/opencode.sh`, `profiles/pi.sh` — no hooks, so no stall lights. Not a
defect; a harness that reports nothing is reported on by nothing.

### Mechanism

Add to `bin/gang`:

```sh
GANG_STALL_REPEAT=600
```

as a script constant, not a config key. It has no consumer asking to tune it
(law 5), and the suite exercises the repeat path by writing an old stamp into
the window option rather than by waiting.

Why 600 and not less: too short and a single stall fills the target's pane with
duplicates of one fact, which is the friction this feature exists to remove; too
long and a genuinely long stall is announced once and, if missed, waits — and
that miss is recoverable with `gang roster`, while a flooded pane is not.

```sh
stall_note() { # $1 = window id, $2 = kind; never fatal
```

1. Read `@gl_notify` off the session. Empty — return 0. The operator declared no
   target; that is the off state, exactly as context lights are off by default.
2. Resolve the target name to a window. No such window — record
   `@gl_stall_failed` on the raising window with the reason and return 0.
3. Target window is the raising window — return 0, recording nothing. A stall
   note into the stalled pane is noise, and the delivery would be refused anyway.
4. Debounce. Read `@gl_stall` on the raising window as `<kind> <epoch>`. Return 0
   if the recorded kind equals `$2` **and** `now - epoch < GANG_STALL_REPEAT`. A
   malformed value is treated as absent and overwritten; this option is written
   only here.
5. Write `@gl_stall "$2 $(date +%s)"`.
6. Build the body, one line:
   `stall: <raising agent> is awaiting input (<kind>) — inspect with gang capture <raising agent>`
   Envelope it with the raising agent as sender — the fact originates in that
   window, and `agent_name_of` reads the name off the window, so the attribution
   law holds with no claimed identity anywhere.
7. Deliver through the ordinary path: live delivery, and on a refusal park it if
   `spool_available` says a parked message would drain. This is the same code
   change 2 installs; a stall note is an ordinary message and gets no private
   transport (law 1). **Synchronously — never backgrounded.** A hook that
   returns before its delivery has resolved reports a note nobody saw.
8. On any failure, record `@gl_stall_failed` on the raising window with the
   reason and return 0. A hook may not kill its harness; this mirrors
   `spool_drain_dispatch`, which records `@gl_spool_failed` the same way.

`gang status <name>` and `gang roster` report a set `@gl_stall_failed` beside
the spool-failure reporting they already do. That is where a silently broken
stall light becomes visible (law 8).

### `cmd_hook`

```sh
Notification) stall_note "$TMUX_PANE" "$(notification_type from payload)" ;;
```

guarded so only a `notification_type` present in the profile's
`GANG_STALL_TYPES` raises a note. Parse the field with the same
`python3 -c 'import json,sys; …'` one-liner shape `cmd_hook` already uses for
`hook_event_name`; an unparseable payload raises nothing.

`PermissionRequest` keeps `occupied_raise_write` and additionally calls
`stall_note "$TMUX_PANE" permission_prompt`. On claude-code both events fire for
one prompt; the debounce collapses them because the kind is identical. On codex
this is the only stall source there is.

Clear `@gl_stall` on `UserPromptSubmit`, `PostToolUse`, and `Stop` — each is
proof the agent moved. Clearing on `PostToolUse` is what lets a second
permission prompt inside one turn raise its own note: the tool that ran between
them ended the first stall.

### Tests

Drive `gang hook` with fabricated stdin, as the suite already does for the
malformed-config case. Every assertion reads the target's pane or a window
option; `gang hook` returns only after its delivery has resolved, so no
assertion depends on timing.

1. Declare `gang notify beta`, fire `Notification` / `idle_prompt` in `alpha`'s
   pane, assert `beta`'s pane carries `[gang:alpha#` and
   `alpha is awaiting input (idle_prompt)`.
2. Fire it again immediately; assert `beta`'s pane gained nothing.
3. Fire `Stop` in `alpha`, then `Notification` / `idle_prompt` again; assert a
   second note landed.
4. Rewrite `@gl_stall` on `alpha` with an epoch older than `GANG_STALL_REPEAT`,
   fire the same kind, assert a note landed. This is the repeat path, exercised
   through state and not through wall time.
5. Fire `Notification` / `auth_success`; assert nothing landed.
6. `gang notify clear`, fire `Notification` / `idle_prompt`, assert nothing
   landed and the hook exited 0.
7. `gang notify alpha`, fire in `alpha`; assert nothing landed and
   `@gl_stall_failed` is unset.
8. `gang notify ghost` (no such window), fire in `alpha`; assert
   `@gl_stall_failed` is set and `gang status alpha` reports it.
9. `gang notify` with a name `valid_name` rejects is refused.

### Documentation

- `docs/reference.md` — `gang notify` with its three forms, the `@gl_notify`
  deletion path, the note's shape, the debounce and its constant, and a table
  row for `GANG_STALL_TYPES` in the profile contract.
- `docs/operations.md` — stall lights under unattended operation: what raises
  them per harness, and the honest statement that codex 0.145.0 witnesses only
  permission requests.
- `docs/DECISIONS.md` — a new terse entry:

  > ## A stall light is a harness's own witness, forwarded
  >
  > Where a harness itself reports that it is waiting on a person, deliver that
  > fact as an ordinary attributed message to one optional operator-declared
  > target. Nothing polls, nothing infers a stall from a quiet pane, and nothing
  > infers a lead: the target is a declaration in the shape of the team cutoff,
  > and with none declared there are no stall lights. A repeated report of the
  > same kind inside one stall is one note, cleared by the harness's own next
  > move. A harness that reports nothing gets no substitute, and a delivery that
  > fails is recorded on the window for status to surface rather than killing
  > the hook.

### Commit

Two: `feat(hook): forward native awaiting-input events as stall notes` and
`feat(profiles): wire the claude-code Notification hook`.

---

## 4. Known-dialog triage

### Problem

Codex 0.145.0 raises a transient menu when a response is slow. Verified strings
from the installed binary, internal id `safety-buffering-prompt`:

- `Our systems are thinking a bit more about this request before responding.`
- `Hang tight or retry with a faster model for a quicker response, though it may be less capable of handling complex requests.`
- option: `Retry with a faster model`
- option: `Dismiss and keep waiting`
- footer: `No action is required. Codex will keep waiting, and this menu will close when the response is ready.`

It occupies the composer, so `GANG_OCCUPIED_REGEX` matches and every send to
that agent is refused as `occupied (authority unknown)` until a person
dismisses it — a menu whose own footer says no action is required.

### The three buckets

Operator doctrine, recorded here because it decides what may ever be
registered:

- **AUTO-ANSWER** — benign transients that carry no authority. The proven case
  is the menu above; the safe option is `Dismiss and keep waiting`, and the
  operator's standing rule is always that option.
- **STALL-LIGHT** — unknown dialogs. Behaviour is unchanged: occupied, refusal,
  and change 3 raises a note.
- **LOUD-MANUAL** — trust prompts, permission prompts, approval dialogs, and
  anything that grants or widens access. **Never** auto-answered. These are the
  prompt-injection boundary: a dialog's text can be written by whatever the
  agent just read, so a component that answers dialogs by matching their text
  must never be able to answer one that decides access.
  `docs/operations.md` already holds this stance and keeps it.

### Registry

Harness knowledge, so it lives in the profile as data (law 4). One newline-
separated record per dialog in `GANG_DIALOGS`, five `|`-separated fields:

```
id|anchor|rows|safe-label|keys
```

- `id` — a short slug for messages.
- `anchor` — a literal line from the dialog that identifies it.
- `rows` — how many selectable rows the dialog has.
- `safe-label` — the literal label of the row to select.
- `keys` — the `tmux send-keys` key names, space-separated, that select
  `safe-label` from the dialog's initial state.

`load_profile` refuses a malformed record: wrong field count, an empty field, a
`|` inside a field, or a non-numeric `rows`.

**`load_profile` also refuses any record whose `anchor` or `safe-label` matches**
`(trust|permission|approve|allow|full access|sandbox|credential|token|secret)`
**case-insensitively.** This is the LOUD-MANUAL bucket made mechanical. It runs
at `load_profile` rather than in `test/lint.sh` because it must also bind
operator-supplied profiles under `GANG_PROFILES`, which lint never reads, and
because it should fail at the moment a dangerous registry would be consulted.

### Where triage runs

Only where Gangline is already about to write to that pane:

- `send_live`, immediately before its occupancy refusal.
- `wait_ready`, in the hitch readiness loop.

**Not** in `occupied()` itself. `gang status` and `gang roster` call `occupied()`
and must stay read-only; a read command that presses keys is a surprise nobody
asked for. Those commands instead gain naming: a recognized dialog is reported
as `occupied (known transient: <id>)` rather than the bare
`occupied (authority unknown)`, so the operator gets the diagnosis without the
keystroke.

This narrows the charter's "send/status/hitch" to "send/hitch, with status
naming only". Recorded here so the reviewer sees it as a decision rather than an
omission.

### Algorithm

```sh
dialog_triage() { # $1 = window id; 0 = a known dialog was answered and cleared
```

1. `[ -n "${GANG_DIALOGS:-}" ]` or return 1.
2. Capture the pane once (`capture_joined`).
3. For each record: the `anchor` must appear, the `safe-label` must appear, and
   the number of rows matching `GANG_OCCUPIED_REGEX` must equal `rows` exactly.
   All three or no match. The row count is the fingerprint: a reworded dialog,
   or the same dialog grown a third option, does not match and is not answered.
4. More than one record matching is ambiguity — `die`, naming both ids.
5. `lock_pane` is already held by the caller on the `send_live` path; on the
   `wait_ready` path take it. Never press a key on an unlocked pane.
6. Send `keys` with `tmux send-keys -t "$1"`, one key name per call.
7. **Verify.** Re-read the pane: the `anchor` must be gone **and**
   `profile_input` must read a composer. If either fails, `die` naming the id,
   the keys sent, and `gang attach` — the window is now in a state Gangline
   caused and cannot account for, which is the loudest case there is.
8. On success, print one line to stderr:
   `answered the known transient '<id>' in '<name>' with its safe option (<safe-label>)`,
   so the answer is never invisible, and return 0. The caller re-reads occupancy
   and proceeds.

### The codex entry — capture required before landing

The entry's `rows`, `safe-label`, and `keys` fields must be pinned to an
observed rendering. The dialog's strings are verified above; **its on-screen row
layout and the key sequence that selects the second option are not**, and this
document will not invent them. `CONTRIBUTING.md` already requires this: "Marker
changes must name the harness version that was observed and add it to the
profile's verified pins."

Procedure:

1. Start a separately named disposable Gangline session on an explicit private
   tmux socket, per `AGENTS.md`. Never the live `gangline` session and never the
   development agent.
2. Hitch a codex agent and drive it until the menu appears (a long,
   heavily-reasoning request is the reliable trigger).
3. `tmux capture-pane -pJ -e` the pane and record it verbatim in the commit body.
4. From that capture, read: how many rows match `GANG_OCCUPIED_REGEX`, which row
   carries the selection marker on first paint, and therefore whether
   `Dismiss and keep waiting` is reached by `Down Enter`, by `Enter` alone, or by
   a digit.
5. Write the record, expected shape:

   ```sh
   # Verified against codex 0.145.0 — capture in the commit body.
   GANG_DIALOGS='safety-buffering|Our systems are thinking a bit more about this request before responding.|<rows>|Dismiss and keep waiting|<keys>'
   ```

6. Confirm the answer clears the menu in that same disposable session before
   landing.

If the menu cannot be reproduced, land everything else in this change — the
mechanism, the `load_profile` refusals, the status naming, the tests against
fixture profiles — with `GANG_DIALOGS` unset in `profiles/codex.sh`, and say so
in the commit body. An empty registry is inert, and law 5 is satisfied by the
fixture consumer plus a registry entry that lands the moment it is observed.

### Tests

Fixture profiles under `$RUN_ROOT/profiles`, sourcing `profiles/bash.sh`, with a
pane painted to look like a numbered menu. No real harness.

1. **A known dialog is answered and the send proceeds.** Paint the fixture
   dialog, send, assert the body reached the pane and stderr named the id.
2. **A dialog with one extra row is not answered.** Same anchor, `rows + 1` rows
   painted, assert the send refuses as occupied and that the pane still shows
   the dialog — the artifact, not the exit status.
3. **A dialog whose safe label is absent is not answered**, same assertions.
4. **Keys that do not clear the dialog fail loud.** A fixture whose declared
   `keys` do nothing; assert the command dies naming the id and `gang attach`.
5. **`load_profile` refuses a registry naming a security surface.** A fixture
   with `Trust this folder?` as its anchor; assert `gang roster` fails naming the
   profile file and the forbidden word.
6. **`load_profile` refuses a malformed record** — four fields, and a
   non-numeric `rows`.
7. **Two matching records are ambiguity**; assert the die names both ids.
8. **`gang status` names a known dialog and presses nothing.** Paint the dialog,
   run `status`, assert `known transient` in the output and that the pane is
   byte-identical afterwards.

Guard-order requirement (`docs/DECISIONS.md`, "A guard witnesses the artifact"):
tests 2, 3, and 8 must be shown to go red against a build that answers
unconditionally, not merely against the pre-feature build. Record that in the
commit body.

### Documentation

- `docs/reference.md` — the `GANG_DIALOGS` record format, the `load_profile`
  refusals, where triage runs, and the verification that follows a keystroke.
- `docs/operations.md` — replace `Gangline never answers permission dialogs.`
  with the precise current rule: Gangline answers only dialogs a profile
  enumerates as carrying no authority, verifies the answer cleared them, and
  never answers a permission, trust, or approval surface — those are refused
  into `occupied (authority unknown)` and, with a notify target declared, raise a
  stall note. Keep the prompt-injection reasoning.
- `docs/DECISIONS.md` §"Occupancy is not authority" — the sentence "Gangline does
  not autonomously answer native dialogs" is no longer true and must be
  rewritten, not annotated. Replacement for that sentence:

  > A profile may enumerate transient dialogs that carry no authority at all,
  > naming each one's whole shape and the keystrokes that pick its safe option;
  > Gangline answers such a dialog only where it is already about to write to
  > that pane, only on a whole-shape match, and only if the pane afterwards
  > proves the dialog gone and a composer present. A dialog that grants,
  > widens, or trusts is never enumerable — a registry matched against on-screen
  > text must not be able to answer the one dialog whose text an agent's own
  > reading can influence — and an unrecognized dialog stays occupied, refused,
  > and reported.

### Commit

Two: `feat(gang): answer enumerated benign dialogs where gang already writes`
and `feat(codex): register the slow-response transient` (the latter carrying the
pane capture in its body, and omitted if the menu could not be reproduced).

---

## 5. Issue #106 — pre-push lints a tree with no `.git`

### Problem

`.githooks/pre-push` exports the pushed commit with `git archive | tar` into a
temporary directory and runs `test/lint.sh` there. That tree has no `.git`, so
any lint check that shells out to git — `git ls-files` being the natural one —
dies with `fatal: not a git repository` and refuses every clean push. Latent
only because `test/lint.sh` does not currently invoke git.

### Change

Replace the archive export with a detached worktree of the pushed SHA.

Verified in this checkout: `git worktree add --detach` works even from a linked
worktree, where `.git` is a file pointing into the main repository's
`.git/worktrees/` directory rather than a directory of its own; the resulting
tree resolves `git ls-files`, and `git worktree remove --force` cleans it up
with no stale administrative entry. The repository
has no `.gitattributes`, so no `export-ignore` or `export-subst` behaviour is
lost with the archive.

In the per-ref loop, replacing the `work=…; git archive … | tar -x -C "$work"`
block:

```sh
n_ref=$((n_ref + 1))
work="$tmp/tree-$n_ref"
( unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX
  git worktree add --detach --quiet "$work" "$local_sha" ) || {
  echo "pre-push: cannot check out $local_sha into a worktree, so nothing was linted" >&2
  fail=1; continue
}
```

with `n_ref=0` initialized beside `fail=0`. The counter, not the SHA, names the
directory: two refs can push the same commit.

**The environment is the whole point of the fix.** Git exports `GIT_DIR` to its
hooks. A `git ls-files` running inside `$work` with `GIT_DIR` still pointing at
the main repository would read the *main* tree and report a pass over ground
nobody covered — the archive bug with a green face. Both the worktree creation
and the lint run must clear it:

```sh
if [ -x "$work/test/lint.sh" ]; then
  ( unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX
    cd "$work" && ./test/lint.sh && ./test/integration.sh ) || fail=1
else
  echo "pre-push: $local_sha carries no test/lint.sh, so nothing was linted" >&2
  fail=1
fi
( unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX
  git worktree remove --force "$work" ) 2>/dev/null || true
```

The unsets are scoped to subshells so the hook's own later `git rev-list`,
`git log`, `git cat-file`, and `git show` calls — which must read the main
repository — are untouched.

Backstop for an interrupted run, replacing the existing trap:

```sh
trap '( unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX
        for w in "$tmp"/tree-*; do
          [ -d "$w" ] && git worktree remove --force "$w" 2>/dev/null
        done
        git worktree prune 2>/dev/null ) || true
      rm -rf "$tmp"' EXIT
```

The `[ -d "$w" ]` guard absorbs the unmatched glob.

Everything else in the hook is unchanged: the missing-`lint.sh` refusal, the
conventional-commits range, and the PII scan extracted with `git show` all keep
their current form and their current reasoning.

### Regression test

In `test/integration.sh`, under `$RUN_ROOT`:

1. `git init` a throwaway repository with a local `user.name` and `user.email`
   and no remotes.
2. Copy in `.githooks/pre-push`, `.githooks/commit-msg`, and `tools/pii-scan`
   from `$ROOT`, preserving the executable bit.
3. Plant `test/lint.sh` as a stub that is executable and whose body is
   `git ls-files >/dev/null` — the exact shape issue #106 names — and
   `test/integration.sh` as an executable stub that exits 0.
4. Commit with a conforming Conventional Commits message.
5. Run the hook from the repository root with a fabricated ref line on stdin:
   `printf 'refs/heads/main %s refs/heads/main %s\n' "$sha" "$zero" | ./.githooks/pre-push`
6. Assert it exits 0.

`test/lint.sh` bans wall-time constructs in `test/*.sh`; the stub is written to
a temporary directory and never matches that glob, so no exemption is needed.

Per `docs/DECISIONS.md` ("A guard witnesses the artifact, and witnesses it in
order"), the test is not proven until it has been seen to fail. Run it against
the unmodified `git archive` hook, confirm it goes red with
`fatal: not a git repository`, and record that in the commit body. A check that
passes both ways is a guard, not evidence.

Add a second assertion, at the same cost, that the fix is doing what it claims:
make the stub write `git rev-parse --show-toplevel` to a file and assert the
recorded path is the worktree, not the main repository. That is the `GIT_DIR`
leak, caught directly.

### Documentation

`CONTRIBUTING.md` mentions `.githooks/pre-push` as the authoritative gate list;
no wording changes. No `docs/reference.md` change.

### Commit

`fix(githooks): lint the pushed commit in a worktree, not a git-less export`,
with `Closes #106` in the footer and the red-first evidence in the body.

---

## Cross-cutting requirements

### Portability

Pure bash 3.2: no associative arrays, no `${var^^}`, no `mapfile`, no `&>`. All
new registry and list data is newline- or space-separated strings parsed with
`case`, `read`, or `awk`. `test/lint.sh` runs `bash -n` and
`shellcheck -S warning` over `bin/gang`, `profiles/*.sh`, and `.githooks/*`, and
must stay clean.

### Suite isolation

`test/integration.sh` already exports `GANG_CONFIG_DIR="$RUN_ROOT/config"`, which
outranks `XDG_CONFIG_HOME` and `$HOME` in `config_load`, so the operator's real
`DOCTRINE.md` is unreachable from fixtures today. Harden it against a future
case that runs `env -u GANG_CONFIG_DIR`: add, beside the existing exports,

```sh
export XDG_CONFIG_HOME="$RUN_ROOT/xdg"
```

so the second precedence tier also lands inside the run root. Add one assertion
that `env -u GANG_CONFIG_DIR "$GANG" config` reports a config root under
`$RUN_ROOT` — the belt proven, not assumed.

### Timing

No new test may sleep, poll, or assert on elapsed time. The stall-light repeat
bound is exercised by writing an old epoch into `@gl_stall`; dialog triage and
delivery are synchronous, so their assertions follow the command that caused the
effect. The suite's fake `sleep` on `PATH` stays as it is.

### Docs discipline

No counts, versions, sizes, or tallies in standing documentation, except the
verified harness pins in this spec and in profile comments, which are dated
evidence a decision rests on. Edits describe the current system; no sentence
narrates the change that produced it.

### Order

1 and 5 are independent and can land first. 2 must land before 3, which reuses
`spool_available` for a refused stall note. 4 is independent of all of them but
should follow 3, so an unrecognized dialog already has a stall light to fall back
to. Land each as its own commit with the suite green at every checkpoint.

---

## Release

Release Please owns versioning; nothing here bumps a version by hand.

The flow, confirmed from repository history and `.github/workflows/release.yml`:
`release-please-action@v4` runs on every push to `main`, reads
`release-please-config.json` and `.release-please-manifest.json`, and maintains
the branch `release-please--branches--main--components--gangline` carrying a
`chore(main): release gangline <version>` commit. That commit updates
`CHANGELOG.md`, `.release-please-manifest.json`, `version.txt`,
`packaging/npm/package.json`, and `packaging/pypi/pyproject.toml`. Merging its
pull request is what tags and cuts the GitHub Release. Every prior release in
this repository followed exactly that path.

Steps for the implementer, after the five changes are merged to `main`:

1. Confirm the release pull request updated itself to include this work.
   `bump-minor-pre-major` is set and change 2 carries a `BREAKING CHANGE:`
   footer, so pre-1.0 that is a minor bump.
2. Read the generated `CHANGELOG.md` diff. Do not hand-edit it, and do not add
   changelog entries to the feature pull requests.
3. Confirm the packaging updaters fired — `packaging/npm/package.json` and
   `packaging/pypi/pyproject.toml` must carry the new version, and
   `version.txt` and `.release-please-manifest.json` must agree with it. These
   updaters fail silently when a path or JSON pointer drifts, which is why this
   is a read and not an assumption.
4. Merge the release pull request. The tag and GitHub Release follow from the
   merge.
5. Smoke-test the released tree in a separately named disposable Gangline
   session on an explicit private tmux socket: `gang up`, `gang notify`, a
   refused `gang send` that parks and then drains at the next turn boundary, and
   `gang roster`. Delete only that exact session afterwards.
