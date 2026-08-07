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
| claude-code | 2.1.224 | hook registry and Notification construction site in the bundled executable; `/usage` driven live in a disposable session |
| codex | 0.145.0 | embedded per-hook JSON schemas and the `HookEventNameWire` enum in the executable; `/usage` driven live in a disposable session |

The live captures were taken in a separately named disposable session
(`ergo-probe-claude`, `ergo-probe-codex`) on an explicit private tmux socket, and
only those exact sessions were deleted afterwards.

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

### `--supersede` is sender-scoped, not topic-scoped

`--supersede` drops **every** message the same sender has waiting for that
target, not only ones on the same subject. That is what the code does and what
`docs/DECISIONS.md` says; what is missing is the operator-facing caution, and
its absence has already cost a message — an amendment batch was destroyed by a
later `--supersede` from the same sender on an unrelated topic.

Add to `docs/reference.md` §`gang send`, beside the existing `--supersede`
sentence:

> It is scoped to the sender, not to a subject: a sender with two unrelated
> messages waiting for one target loses the first when the second carries the
> flag. Pass it only when the newer message genuinely replaces everything that
> sender has parked.

**Do not add topic scoping.** A subject, thread, or topic key would be a
coordination schema, and `docs/DECISIONS.md` §"Gangline is substrate, not
coordination" forbids exactly that: Gangline defines no reporting protocol and
no message taxonomy. The sender knows which of its own messages are still
relevant; the substrate does not and should not learn.

### A held entry is unverified, not undelivered

Observed in the live session: a spool drain pasted a body, watched the composer
change, pressed Enter, and the harness parked the submission in its own input
queue rather than submitting it. `submit_verify`'s post-Enter queue check caught
that correctly, the entry was held rather than re-sent, and `gang flush` later
refused because the parked-body record was gone. The harness then drained its
own queue, so the message **did** reach its target while Gangline was reporting
it as not verified.

Every step of that was right, and the spec changes no mechanism:

- Holding rather than re-sending is the rule (`docs/DECISIONS.md`: Gangline
  never sends a message a second time on the chance the first did not arrive).
  A false negative in this direction is the safe one; a second copy is not.
- `flush` refusing was right too. By the time it ran, the harness had drained
  its own queue, so the recorded body had been retired by `stage_clear` and
  there was nothing to recall. Pressing the recall key on that composer would
  have submitted whatever happened to be there.

What is wrong is only the **words**. "Held" and "not verified" are read by an
operator as "not delivered", and they do not mean that. Fix the reporting:

- `gang status <name>` — the held-entry line reads
  `held (delivery NOT verified — it may still have arrived): <fragment>`
  rather than any phrasing that asserts non-delivery.
- `docs/reference.md` §`gang send` — state that a held entry means Gangline
  could not verify the delivery, not that the message failed to arrive: a
  harness that parks a submission in its own queue may drain it later.
- `docs/operations.md` — the recovery is to read the target before re-sending
  by hand. Gangline will not decide that for the operator, and the reason it
  will not is that it cannot see the harness's internal queue.

**Fixture.** Add a suite case that reproduces the whole sequence against the
existing `spoolable` fixture: arm the fixture to paint queue evidence after
Enter, drive a drain, assert the entry is held and that the reported wording
says unverified rather than undelivered, then clear the queue evidence to
simulate the harness draining itself and fire another Stop event. **Assert that
nothing is re-sent** — the held entry stays held and the target's pane gains no
second copy. That last assertion is the point of the fixture: the arrival must
not become a reason to deliver again.

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
separated record per dialog in `GANG_DIALOGS`, six `|`-separated fields:

```
id|anchor|footer|labels|safe-label|keys
```

- `id` — a short slug for messages.
- `anchor` — a literal line that opens the dialog and identifies it.
- `footer` — a literal line that closes it. Anchor and footer bound the region
  the fingerprint is taken over.
- `labels` — every selectable row label, comma-separated, in painted order.
- `safe-label` — which of those labels carries no authority and is the one to
  select. Must appear in `labels`.
- `keys` — the `tmux send-keys` key names, space-separated, that select
  `safe-label` from the dialog's initial state.

**A row count would not work, and observation is why.** In the live codex
0.145.0 capture below, the selection marker *and* the row number are painted
only on the currently-selected row; every other row is blank-prefixed:

```
› 1. Show usage                View recent account token usage.
     Redeem usage limit reset  No usage limit resets available.
```

Counting lines that match `GANG_OCCUPIED_REGEX` therefore returns 1 for a
two-row dialog and would return 1 for a dialog that grew a third option — the
exact change the fingerprint exists to catch. Bounding the region and accounting
for every line in it is what actually detects a dialog that is no longer the one
the profile enumerated.

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
3. For each record, all of the following or no match: the `anchor` line appears;
   the `footer` line appears after it; every declared label appears exactly once
   in the region strictly between them; and every non-blank line in that region
   contains one of the declared labels. The last clause is the fingerprint — an
   unaccounted line in the region means the dialog is not the one enumerated,
   whether it was reworded or grew an option, and it is not answered.
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

The entry's `footer`, `labels`, and `keys` fields must be pinned to an observed
rendering of **this** dialog. Its strings are verified above; its painted layout
is not, and this document will not invent it. `CONTRIBUTING.md` already requires
this: "Marker changes must name the harness version that was observed and add it
to the profile's verified pins."

Corroborating evidence from a different codex 0.145.0 dialog, captured live (the
`/usage` selection menu, reproduced in item 7): row 1 carries the selection
marker on first paint, and the footer reads
`Press enter to confirm or esc to go back`. If the slow-response menu paints the
same way, `keys` is `Down Enter` and the labels are
`Retry with a faster model,Dismiss and keep waiting`. Expect that; verify it
anyway, because a different dialog is corroboration and not observation.

Procedure:

1. Start a separately named disposable Gangline session on an explicit private
   tmux socket, per `AGENTS.md`. Never the live `gangline` session and never the
   development agent.
2. Hitch a codex agent and drive it until the menu appears (a long,
   heavily-reasoning request is the reliable trigger).
3. `tmux capture-pane -pJ -e` the pane and record it verbatim in the commit body.
4. From that capture, read: the line that closes the dialog, every row label in
   painted order, which row carries the selection marker on first paint, and
   therefore whether `Dismiss and keep waiting` is reached by `Down Enter`, by
   `Enter` alone, or by a digit.
5. Write the record, expected shape:

   ```sh
   # Verified against codex 0.145.0 — capture in the commit body.
   GANG_DIALOGS='safety-buffering|Our systems are thinking a bit more about this request before responding.|<footer>|Retry with a faster model,Dismiss and keep waiting|Dismiss and keep waiting|<keys>'
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
2. **A dialog that grew a row is not answered.** Same anchor and footer, one
   undeclared non-blank line painted in the region between them; assert the send
   refuses as occupied and that the pane still shows the dialog — the artifact,
   not the exit status. This is the assertion the row-count design would have
   passed while pressing a key.
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

## 6. On-demand context query

### Problem

Gangline already computes per-agent context for `GANG_CONTEXT_LIGHTS` and the
band ladder, but that computation is reachable only when a threshold crosses.
An agent deciding whether to compact, or an operator sizing a team's remaining
room, has no way to ask.

### Surface: `gang context [name]`, and no roster column

Both halves are decided on mechanism.

**The query is a command.** `context_now()` already is the one computation; the
threshold path (`context_light_read`) and the query become its two readers.
Nothing is measured twice, cached, or reconciled.

**There is no roster column, and `profile_context` is why.** That function is
built to die when it cannot read its native source: `claude-code` dies with
`no ctx beacon in pane` for any window whose lights were not enabled at hitch
and for every adopted window; `codex` dies without the `@gl_key` that only a
hitch mints, which adopted windows never have. `cmd_roster` fails loudly when a
row cannot be observed — the suite guards that with
`roster fails when an agent row cannot be observed`. A context column would
therefore kill `gang roster` for ordinary, correctly-configured teams, and
`roster` is the one command that has to answer when everything else is
uncertain. It stays as it is.

### `cmd_context`

1. `resolve "$name"`, then `context_now "$AGENT_ID"`, then print its output
   raw — the profile's own `<used>k/<window>k (<pct>%)` format, unparsed.
2. **Do not route through `context_light_read`.** That function returns early
   when `GANG_CONTEXT_LIGHTS` is off, and a query that only answers when
   notifications are enabled is not an on-demand query. Lights are the
   notification; this is the read.
3. A profile declaring no `profile_context` refuses, naming the profile.
4. `profile_context`'s own `die` message stands and the command exits non-zero.
   Absent evidence is reported as absent; no value is fabricated and no fallback
   is invented.

### The availability is not uniform, and that is stated

With lights off, `gang context` answers on `codex` (its source is the rollout
file, needing only the hitch-time `@gl_key`), on `opencode`, and on `pi` (both
read the pane). It cannot answer on `claude-code`, whose source is a statusline
beacon that only an enabled-lights hitch wires. The existing refusal already
says exactly that, and it is left to say it rather than being softened.
`docs/reference.md` records the per-profile availability so an operator is not
surprised by a refusal that is really a launch choice.

### Self-targeting

Bare `gang context` targets the window it runs in, per item 8. This is the
canonical use — an agent asking its own remaining room before deciding to
compact — so it joins that item's table.

### Tests

1. A fixture profile with a `profile_context` returning a known reading: bare
   and named invocations both print it, byte-equal.
2. A fixture whose `profile_context` dies: the command exits non-zero and the
   profile's own message reaches stderr, with no value on stdout.
3. A fixture declaring no `profile_context`: refused, naming the profile.
4. **Lights off still answers.** Same fixture with `GANG_CONTEXT_LIGHTS` unset;
   assert the reading is printed. This is the assertion that the query did not
   inherit the notification gate.
5. `gang roster` output contains no context column — a guard against a future
   change reintroducing the failure mode above.

### Documentation

`docs/reference.md` — the command, the raw-output guarantee, per-profile
availability, and why roster carries no column. `docs/DECISIONS.md` §"Context
lights are optional and minimal" gains one sentence: the same computation is
exposed on demand as a query, which reads whether or not lights are enabled,
because signalling and asking are different acts.

### Commit

`feat(gang): expose the context computation as an on-demand query`

---

## 7. `gang usage <name>`

### Problem

Plan and quota usage sits behind an interactive command in each harness. Reading
it means attaching to a window, typing, reading, and dismissing — per agent, by
hand.

### What was observed

Both harnesses were driven live in a disposable session. The two renderings are
not the same shape, and the difference is the whole design.

**claude-code 2.1.224 — `/usage` is a full-screen modal.** Submitting it
replaces the pane with a tabbed page (`Settings  Status  Config  Usage  Stats`)
carrying session cost, session and weekly limit bars with reset times, and a
contributing-factors section. No composer is on screen. `Escape` dismisses it,
and the composer returns reading empty through the profile's own `profile_input`
parser — verified by running that parser over the post-dismissal capture. The
page is scrollable: the capture ends with a `↓` marker and more content below.

**codex 0.145.0 — `/usage` is a two-step, and it renders inline.** Submitting it
opens a selection menu:

```
  Usage
  View account usage or redeem an earned reset.

› 1. Show usage                View recent account token usage.
     Redeem usage limit reset  No usage limit resets available.

  Press enter to confirm or esc to go back
```

`Show usage` is preselected, so a further `Enter` confirms it. The content then
lands **in the transcript**, not in an overlay, and the composer is already
restored with no key pressed. There is nothing to dismiss.

The command name is verified: typing `/us` in codex 0.145.0 matches exactly one
command, `/usage  view account usage or use a usage limit reset`.

This refutes the charter's "dismiss the UI (profile-declared key, e.g. Esc)" as a
universal step. It is right for claude-code and wrong for codex, so dismissal is
per-profile data that may legitimately be empty.

### Profile surface

```sh
GANG_USAGE_CMD          # the command to type, e.g. /usage
GANG_USAGE_CONFIRM_KEY  # keys pressed after submit to reach the content; may be empty
GANG_USAGE_RENDER       # modal | inline
GANG_USAGE_DISMISS_KEY  # key that closes it; empty when the harness self-restores
```

`profiles/claude-code.sh`:

```sh
# Verified on claude-code 2.1.224: /usage opens a full-screen tabbed modal with
# no composer, and Escape restores an empty composer. The page scrolls; gang
# returns the visible screen and does not drive the scrollbar.
GANG_USAGE_CMD="/usage"
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="modal"
GANG_USAGE_DISMISS_KEY="Escape"
```

`profiles/codex.sh`:

```sh
# Verified on codex 0.145.0: /usage opens a selection menu with "Show usage"
# preselected; one Enter confirms it and the content is appended to the
# transcript with the composer already restored, so nothing dismisses it.
GANG_USAGE_CMD="/usage"
GANG_USAGE_CONFIRM_KEY="Enter"
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
```

`profiles/opencode.sh`, `profiles/pi.sh`, `profiles/bash.sh` declare none.
`gang usage` refuses for them, naming the missing declaration — the same shape
`gang flush` and `gang interrupt` already use for an undeclared profile.

### `cmd_usage`

1. `resolve "$name"`. Refuse if `GANG_USAGE_CMD` is empty:
   `usage: profile '<p>' declares no GANG_USAGE_CMD, so gang does not know this harness's usage command`.
   Refuse an unknown `GANG_USAGE_RENDER`.
2. `lock_pane`. Every keystroke below is under the pane lock.
3. **Same predicates as a send, and no weaker.** `occupied` refuses.
   `busy` refuses on 0 and on could-not-determine — note that this is *stricter*
   than `send_live`, which falls through could-not-determine to a provably empty
   box. Typing a UI command mid-turn cannot be verified the way a message
   delivery can: there is no envelope to read back, so the fall-through's
   residual-risk argument does not carry here. `composer_settled` refuses.
   `input_clear` must be true.
4. Capture the pane as `before`.
5. `inject "$AGENT_ID" "$GANG_USAGE_CMD" head` — the same path `gang compact`
   uses to submit a native command.
6. Press each key in `GANG_USAGE_CONFIRM_KEY`, if any.
7. **Prove the screen changed.** Bounded look loop, the shape `cmd_flush`
   already uses for its recall readback: capture, compare with `before`, break on
   difference, and after the bound refuse with
   `usage: the screen of '<name>' never changed after <cmd>, so gang has no usage content to report and pressed nothing further`.
   No new timing primitive; `cmd_flush`'s loop is the precedent and `bin/gang`
   is not bound by the suite's no-sleep rule.
8. Extract the content:
   - `modal` — the whole after-capture, trailing blank rows trimmed, exactly as
     `cmd_capture` trims.
   - `inline` — the after-capture's lines beyond the last non-blank line of
     `before`. The transcript appended; the answer is what it appended.
9. Dismiss: press `GANG_USAGE_DISMISS_KEY` if non-empty.
10. **Restore is part of the contract.** Re-read: `profile_input` must read a
    box, and `input_clear` must be true. If either fails, `die` naming the
    agent, the key pressed, and `gang attach` — and print the captured content
    anyway on stdout before dying, because the content was already read and
    withholding it helps nobody. Exit non-zero.
11. `lock_release`, print the content raw to stdout, nothing else on stdout.

The content is the harness's own UI text, unparsed. Gangline does not summarize
it, extract numbers from it, or decide anything from it: parsing and pacing
policy are the operator's.

Known limitation, stated rather than engineered around: on a harness whose page
scrolls, `gang usage` returns the visible screen. Driving a scrollbar to
reassemble a page would be a second product, and the operator can attach.
`docs/reference.md` says so.

### Tests

Fixture profile over `profiles/bash.sh` whose `GANG_USAGE_CMD` is a shell line
that paints a known block, exercising both `modal` and `inline` extraction with
two fixtures. No real harness.

1. **Content is returned raw**, byte-equal to the painted block.
2. **`inline` returns only the appended lines**, not the pre-existing transcript.
3. **Restoration is verified.** After a successful run the fixture's composer
   reads empty through `profile_input`.
4. **A mutant that skips dismissal fails.** Fixture whose declared dismiss key
   does nothing; assert the command exits non-zero, names `gang attach`, and
   **still printed the content**. This is the test the amendment names
   explicitly, and it is the one that proves restoration is a contract rather
   than a hope.
5. **A screen that never changes refuses**, naming the command, with nothing
   further pressed and the composer left empty.
6. **A busy target refuses**, and a could-not-determine target refuses — the
   assertion that `cmd_usage` is stricter than `send_live` and did not
   accidentally inherit the fall-through.
7. **An occupied target refuses.**
8. **A profile declaring no `GANG_USAGE_CMD` refuses**, naming the variable.

### Documentation

`docs/reference.md` — the command, the four profile variables in the profile
contract table, the predicates, the raw-output guarantee, and the scrolling
limitation. `docs/operations.md` — reading usage across a team without
attaching.

### Commit

`feat(gang): report a harness's own usage page without attaching`, with both
live captures in the body.

---

## 8. Self-targeting defaults

### Problem

An agent asking about itself must know and type its own name, and a bare
invocation answers with a naked error instead of help.

### Rule

**Bare invocation only.** A command invoked with zero arguments, where the only
missing argument is a target name and self-targeting is coherent, targets the
window it runs in. Any argument at all selects the existing signature unchanged.

The bare-only rule exists to kill an ambiguity rather than to be terse.
`gang capture <name> [lines]` with one numeric argument could mean a window
named `40` or forty lines of self; `valid_name` permits a numeric name, so
inferring would be a silent fallback and law 8 forbids it. Zero arguments is the
one form that cannot be misread.

Self is resolved exactly as `send_sender` resolves sender identity: `self_window`
then `agent_name_of`. Outside a Gangline window there is no self, and the
command prints its own help with one line saying so — not an error.

### Verdicts

| Command | Bare | Reason |
|---|---|---|
| `gang status` | self | An agent reading its own state is the common case. |
| `gang capture` | self | Reading your own pane; default line count applies. |
| `gang composer` | self | Reading your own input box. |
| `gang compact` | self | Self-compaction is already a first-class path. |
| `gang usage` | self | Reading your own plan usage. |
| `gang context` | self | An agent asking its own remaining room before compacting is the canonical use. |
| `gang interrupt` | help | Incoherent: it would run inside the turn it drops, and the bracket it edits is your own. |
| `gang flush` | help | Incoherent: recovering your own parked queue needs your harness idle, which it is not while you are running. |
| `gang drop` | help | Destructive, and self-targeting makes it destructive *by omission*. |
| `gang hitch` | help | The name argument is a new agent's, so there is no self to default to. |
| `gang adopt` | help | Names a window that is not yet an agent; self already is one. |
| `gang send` | help | `--to` is the message's destination; sending to yourself is incoherent. |
| `gang up`, `roster`, `attach`, `profiles`, `config`, `cutoff`, `notify`, `down` | unchanged | Already meaningful bare; none of them is a bare-error case today. |

`gang down` ends the whole team and takes no target, so it is not a
self-targeting question. It keeps its current behaviour.

### Implementation

One helper:

```sh
self_name() { # -> this window's agent name; nonzero outside a Gangline window
```

built on `self_window` and `agent_name_of`. Each self-targeting command, on zero
arguments, calls it and falls back to `cmd_help <command>` plus the line
`(no target given, and this shell is not a Gangline agent window)` when it fails.

`usage_die` is replaced at every bare-invocation site by `cmd_help <command>` and
exit 1. **No command may answer a bare invocation with a naked error.** That is
the acceptance criterion, and it is testable: invoke every command in the usage
list bare, and assert each one's output contains its own synopsis.

### Tests

1. Bare `gang status`, `capture`, `composer`, `usage` inside a fixture agent
   window each report on that window. Drive them with `TMUX_PANE` set to the
   fixture's pane, as the suite already does for `gang hook`.
2. Bare `gang compact` inside a fixture window compacts that window.
3. Bare `gang interrupt`, `flush`, `drop`, `hitch`, `adopt`, `send` each print
   their own synopsis and exit non-zero, and — the real assertion — leave the
   fixture window untouched.
4. Bare `gang status` outside any Gangline window prints the synopsis and the
   not-an-agent line.
5. **Sweep:** every command name in `usage()` invoked bare produces output
   containing that command's synopsis line. This is the no-naked-error rule as
   one assertion, and it fails for any command a future change adds without help.

### Documentation

`docs/reference.md` — a short section stating the bare-only rule, the table
above, and that self resolves the way a sender does.

### Commit

`feat(gang): default a bare command to the window it runs in`

---

## 9. Narrow-terminal help

### Problem

The operator reads help over SSH on a phone. Wide single-line usage blocks wrap
mid-token and the output becomes unreadable.

### Width budget: 48 columns

Argued, not asserted. Phone-SSH portrait sits near 40 columns on common
terminals, and 80 is the floor everywhere else. The two directions of error are
not symmetric: too wide turns a synopsis into hash across a wrap boundary, which
is unrecoverable by the reader; too narrow only costs vertical lines, which
scroll. So the budget is set by the narrow case with a small margin — 48 — which
soft-wraps at most one line at 40 while keeping each synopsis fragment long
enough to still read as a command. The longest unavoidable fragment,
`gang hitch <name> [-p <profile>] [-d <dir>]`, is 42 columns with a two-space
indent, so 48 accommodates the real content rather than being chosen to fit a
number.

### Shape

`gang --help` lists commands one per line, name and one-line purpose, no
column-aligned table:

```
gang — drive native CLI agents in tmux

  up        start a team and join it
  hitch     add an agent
  send      deliver a message to an agent
  ...

gang <command> --help for one command.
```

Per-command help stacks its synopsis, one form per line, optional groups on
their own lines:

```
gang send
  Deliver a message to one agent.

  gang send --to <name> --stdin
    [--from <sender>]
    [--live-only]
    [--supersede]

  Parks on refusal. --live-only never parks.
```

Every subcommand gets `--help`, routed through the same `cmd_help <command>`
that item 8 reuses for bare invocations with no self default. `cmd_help` with no
argument prints the top-level list.

Help text lives in one place — a single `case` in `cmd_help` — so a command
cannot gain a synopsis in one place and not the other.

### Test

Assert that no line of `gang --help`, and no line of `gang <cmd> --help` for
every command in the list, exceeds 48 columns.

**Measure in characters, not bytes.** Local `awk` is mawk and CI's is gawk, and
`length($0)` on a line containing `—` or `❯` differs between them: mawk counts
bytes, gawk counts characters. A byte-counting assertion would pass locally and
fail in CI, or worse, pass in both while measuring the wrong thing. Measure with
`python3 -c 'import sys; …len(line)…'`, which the suite already depends on
elsewhere, and have the failure print the offending command, line, and its
width.

Add the same sweep as item 8's test 5 — every command in the list has help — so
the two acceptance criteria are checked over one enumeration.

### Documentation

`docs/reference.md` keeps the full syntax; help stays a pointer to it, and the
budget is recorded beside the help section so a future edit knows the constraint
is deliberate.

### Commit

`feat(gang): make help legible at phone-SSH widths`

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
to. 6 and 7 are independent. 9 must land before 8, which routes bare invocations
through `cmd_help`; landing 8 first would mean writing that router twice. Land
each as its own commit with the suite green at every checkpoint.

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
