# Reference

For named mailbox registrations and their remote MCP endpoint, see
[outside agents](outside-agents.md).

`gang --help` lists commands. `gang help COMMAND` or `gang COMMAND --help`
shows arguments and flags for the installed binary. From an operator shell,
supply an explicit name when inspecting or controlling a single agent.

Flags accept `-` or `--`, `VALUE` or `=VALUE`, and one-letter flags also
`-cVALUE`; switches accept `=true` or `=false`. Flags may follow operands, and
`--` ends flags. `--help` anywhere before `--` prints help and exits 0.

Every command that acts on a team accepts `--team TEAM`. Without it,
`GANG_SESSION` selects the team. Agent operands such as `NAME` always name
agents in the selected team.

## Start and stop

| Command | Effect |
| --- | --- |
| `gang up [NAME] [AGENT OPTIONS]` | Start the selected team with its lead agent named `NAME` (default `lead`), then attach when stdin is a terminal. |
| `gang hitch NAME [OPTIONS]` | Launch an agent and deliver startup instructions and any task. |
| `gang hitch NAME --recover` | Recover retained startup from its visible draft or a lone collapsed paste, or resume a queued startup its boot deadline failed once the pane shows an idle composer or a recognized prompt. |
| `gang rehitch --note TEXT [--upgrade]` | Restart your harness at the next confirmed idle boundary and resume its native conversation. |
| `gang rename OLD NEW` | Change a registered name and its pane's display title. |
| `gang drop NAME` | Stop an agent and fail its pending messages. An unregistered `NAME` is refused. |
| `gang down [-y, --yes]` | Confirm on a terminal (`[y/N]`, default no), then retain its audit log, drop the selected team's agents and delete only its runtime state. Before dropping any agent, it appends who ran it to `downs.jsonl` in the state root. Use `--yes` without a terminal. |
| `gang attach [NAME]` | Attach to the named agent's pane in the selected team. Without `NAME`, select the lead, or the first registered agent if there is no lead. |
| `gang teams` | List teams in the configured state root. |

Save unfinished work first and name it in `--note TEXT`. The restarted harness
returns to its window's place: the same index, or, when the session has
`renumber-windows` on, the same place before the window that followed it. An
agent that shared its window with other panes gets a new window where tmux
places it, and one whose window was its session's only window starts a new
session. A place that could not be read, or that was lost during the restart
because another window took the index or the following window closed, also
gives way to tmux's placement without moving any other window, and the team
log records why in a `rehitch_position_lost` event. The harness
executable is resolved from PATH; use `--upgrade` to update it before requesting
a restart. `--upgrade` runs the collar's harness update command first; an update
failure leaves the current harness running.

Agents are independent panes. Their names and status appear on their own
borders, so they can share a window without overwriting each other's titles.
Window tabs display the active pane's status and keep the current tab styling.
Selecting an unregistered pane shows the window name.
Gangline copies the effective tab formats into window-local options. Later
global styling changes take effect after unsetting that window's
`window-status-format` and `window-status-current-format` options; the next
refresh adapts the inherited formats again.
Window names and layout remain the operator's presentation. For example:

```sh
gang hitch worker --split lead
gang hitch helper --split worker --vertical
gang attach helper
```

Capture, delivery, status and individual drops address the named agent's pane
in either layout. An operator shell split is allowed. A pane running a
configured agent without registration refuses hitching and is identified by
its window and pane position; it is not adopted or removed automatically.

`up` and `hitch` accept:

| Option | Meaning |
| --- | --- |
| `-c`, `--collar COLLAR` | Harness collar; defaults to `GANG_COLLAR`. |
| `-d`, `--dir DIR` | Working directory; defaults to the current directory. |
| `-m`, `--model MODEL` | Native model identifier; the native CLI judges it. A collar's `hitch_guard` can warn or refuse at usage thresholds. |
| `-e`, `--effort EFFORT` | Native reasoning effort; requires `--model`. Refused when `gang models` lists the model without this effort. |
| `--context-bands EARLY,LATE` | Set this agent's context-band thresholds as percentages, for example `--context-bands 10,20`. Requires a collar with an early and late band for every model selector. |
| `-t`, `--task TASK` | Startup assignment. |
| `--title TEXT` | Explicit native session display title; also accepted by `up`. Must be a nonempty single line without tabs or control characters. |
| `-r`, `--role ROLE` | Role brief; `up` defaults to `lead`. |
| `--stdin` | Read the assignment from stdin. |
| `--split NAME` | Place the new agent beside the named agent in its window; the target must already have a registered pane in the selected team. Without this option, create a new window. |
| `--vertical` | Split top/bottom instead of side by side; requires `--split NAME`. |
| `--window INDEX` | `hitch` only: create the agent's window at this tmux window index. An index already in use is refused before the name is claimed, and no other window moves. Requires the team's session; cannot be combined with `--split`. |
| `--resume SESSION` | Resume a native conversation by its native session ID. |
| `--recover` | Recover an agent's retained startup input. |

For example, `gang up --context-bands 10,20` gives the lead earlier notices,
while `gang hitch worker --context-bands 20,30` sets the worker's thresholds
independently.

A command run from an agent's pane, including any script its native CLI
starts, carries that agent's identity and team. Within that team, `down`,
a `curfew` change, and dropping an agent the caller did not hitch are the
lead's. The lead is the agent started in the `lead` role by `gang up` or the
operator; neither its name nor a `--role lead` given by another agent confers
that authority. A hitch run from an agent's pane records that agent as the hitcher,
which gets a `[gang:hitch#…]` notice when the agent fails, recovers, holds
undelivered input, or reads as idle with an aged background report. The background
notice names the report's count and receipt time; current task state is unknown.
Its threshold is measured from receipt, so a delayed callback starts a fresh
clock and can cause another warning. Repeated observations and the hitcher's
receipt of the notice do not repeat the warning for that stored report.

Each line of `downs.jsonl` records the time, team, retained log path, agent count, the caller's
agent name and inherited `GANG_AGENT_ID`, its process and parent process IDs,
the parent's command line where `/proc` shows it, the working directory, and
`TMUX_PANE`.

Audit logs survive `down` under `STATE_ROOT/logs/TEAM/LOG_ID.jsonl`.
The command prints the retained path and records it as `retained_log` in
`STATE_ROOT/downs.jsonl` before stopping agents. Read it with
`gang log --all LOG.jsonl`; the active log path is unchanged. A private marker
in runtime state makes retries reuse the same retained log, while a recreated
team gets a separate identity. The name and log ID do not establish task or
cross-host attribution. Final teardown events and late writes to the same
open log remain in the retained file. If retention fails, teardown stops.

Failed pane-hold diagnostics are retained beside the audit as
`LOG_ID-hold-HITCH_ID-ATTEMPT.log`, before their original temporary path is
removed or reused. These are the existing startup error bytes, not native
transcripts. Successful holds produce no diagnostic log. No retention period
or automatic expiry is configured: logs remain until an explicit retention
policy removes them. Runtime teardown is not such a policy.

## Message and control

| Command | Effect |
| --- | --- |
| `gang sender add NAME` | From the operator shell or active lead pane, register NAME and print the path of a random key file under the team's state directory, readable only by the user. Refuses agent, outside-mailbox, registered-sender and enrolled peer-host names. |
| `gang sender list` | List registered sender names. |
| `gang sender remove NAME` | From the operator shell or active lead pane, revoke NAME and delete its key file and registration; queued messages retain their authenticated attribution. |
| `gang send NAME [OPTIONS] [BODY]` | Send BODY, or read stdin when absent, and report its receipt. |
| `gang send HOST:NAME [BODY]` | From an active registered agent, or with `--as NAME` and its registered sender credential, forward BODY over ssh to agent NAME on `GANG_PEERS` host HOST, where the forced command of this host's key runs `gang peer-receive` to deliver it, and print `HOST:ID` with that host's receipt. Once ssh has run, the sending host's team log records a `peer_sent` event under the sender with `HOST:NAME`, the message ID, and the outcome, `refused`, or `unknown`. No later outcome on HOST reaches this host. A lost reply prints `unknown` and exits 5. |
| `gang queue [NAME] [--json] [--limit N\|--all]` | List pending messages with ID, recipient, sender, kind, a text excerpt, and state: `ready`, `scheduled` with its due time, `blocked` with the reason, or `unknown` with the reason. |
| `gang interrupt [NAME] [-m\|--message REASON]` | Interrupt the turn; deliver an optional reason after it stops. |
| `gang title [NAME] TEXT` | Set the native harness session title when idle with a free composer. Quote TEXT as one argument; omit NAME for the current pane's agent. |
| `gang clear --note TEXT` | Start a clean native conversation at a confirmed safe boundary; reload current standing guidance and role before the required continuation, then release retained mail. Self only. |
| `gang compact [NAME] [--note TEXT]` | Compact at native idle; submit the continuation behind compaction, ahead of later input. |
| `gang compact NAME --interrupt` | Cancel a queued, unsubmitted compaction. Interrupt a submitted or unconfirmed compaction with the collar's recovery keys while the pane shows it running. A failed deferred compaction can instead clear its exact staged command from an idle composer and retry queued delivery; edited input remains untouched. It records an interrupted compaction as unconfirmed and reports the screen it left. |
| `gang curfew [DURATION\|HH:MM\|RFC3339\|clear]` | Show, set, or clear the team deadline. |
| `gang tick [--agent NAME\|HITCH_ID]` | Check deadlines, recover native failures, and drain due messages, for every agent or the one `--agent` names. |
| `gang wait NAME [--timeout DURATION]` | Wait up to DURATION (default 30s) for a recorded idle boundary; a zero timeout checks once. |

After startup, an idle tick submits the collar's native title action with the
exact `--title TEXT` value when supplied. For example, `gang hitch worker
--title 'worker@ser5: cache refresh'` names the native session without changing
the hitch name or routing. When omitted, the title is `NAME@TEAM: TASK`, using
a shortened first line of the task. Without a task,
the title is `NAME@TEAM`. Submission waits for a free composer and a finished
native turn. Gangline records the attempt before typing and does not repeat
it automatically; internal rehitch preserves the record for the resumed
session. `gang title` uses the same guarded submission path.
An unconfirmed or failed automatic title is logged as an `observation` with
an `automatic title:` reason and does not fail the hitch. Submission does not
prove that the native session list persisted the title.

At or after a curfew deadline, a tick notifies each active agent once for
that deadline, including agents started later. The notice names the team,
deadline and observation time. The deadline remains set until changed or
cleared. Agents remain active; `up`, `hitch`, and `attach` remain available.

While the collar's `actions.compact.active` pattern matches the pane, the agent
reads as compacting: sends wait in Gangline's queue, a queued compaction waits,
and `gang compact NAME --interrupt` treats the pane as running. A harness that
queues input typed during compaction witnesses it only when compaction ends,
so a send typed then could not be confirmed. For such a collar the resume note
also waits for the pattern, or a busy screen, after the compact Enter: a
command that leaves the composer without either fails as possibly run and
withholds the note. An empty composer does not show that a compaction started,
and an idle-looking screen does not show that none will: the harness may hold
the command behind a turn still streaming, or still be running its
pre-compaction hooks.

Keep a `--note` note short, on one line, and pointed at a state file. Without
`--note`, the note tells the agent to re-read its brief and durable state and
names `gang log --agent NAME --type send_queued --all`, which records the messages
queued for it. Every
failed compaction queues a `[gang:compact#…]` notice to the agent saying whether
its context was compacted, and a copy to the agent that requested it when the
failure comes after its command returned.

### Clear at a phase boundary

Use `gang compact` within continuing work. Use `gang clear --note TEXT` for a
major phase change that needs a clean context in the same logical agent.
`gang rehitch` restarts the harness and resumes its existing conversation.
Clear uses the collar's native `/clear` without restarting the process or pane;
identity, routing, children, configured model and permissions stay in place.

Before clear or compact, bring the relevant planning state up to date. Keep the
clear note small: name the authoritative change or document, its current
revision/state, the next action, and any essential unsaved or in-flight context.
For a project using OpenSpec, point to the relevant change and revision instead
of copying its plan or the conversation history. For example:

```sh
gang clear --note 'Continue change cache-refresh at revision abc123, implementation complete; next: verify the documented acceptance. One child review is pending.'
```

The note is required. Finish the current turn after queuing clear. Mail waits
immediately, while existing native work finishes. Drafts, native queued input,
permission prompts and unresolved accepted or unverified receipts prevent clear.
The running process must already have emitted its SessionStart hook; updating a
collar does not install hooks into an older process. Unsupported sessions refuse
before native clear is submitted.

The first new-context prompt contains the current delivery contract, configured
doctrine and role, followed by the continuation. Its synchronous submit hook
requires a clear-source SessionStart with a different native ID and admits that
exact continuation once. Only its verified submission releases retained due
mail; future scheduling and expiry still apply. The old transcript remains on
disk but is not transferred into the new context.

`gang status --why` reports the clear phase and blocker; `gang status --json`
includes the previous native ID and current binding. An uncertain command or
continuation is held rather than automatically repeated. See
[clear recovery](operations.md#clear-recovery). Clear does not operate on a
named peer and has no relaunch fallback.

Send options:

| Option | Meaning |
| --- | --- |
| `--from SENDER` | Required outside registered panes unless using `--as`; refused inside registered panes. |
| `--as SENDER` | Authenticate a registered sender with a key file. |
| `--credential FILE` | Read the key for `--as` from FILE, taking precedence over `GANG_SENDER_CREDENTIAL`; requires `--as`. |
| `--live-only` | Refuse instead of queuing if the recipient cannot take input now. |
| `--supersede` | Clear this sender's older scheduled messages for this recipient first. |
| `--at DURATION\|HH:MM\|RFC3339` | Schedule delivery after a duration, at a local time, or at an exact deadline. |
| `--clear` | Clear this sender's scheduled messages for the recipient instead of sending; takes no body, `--at`, or `--live-only`. |

A `registered:NAME` envelope identifies a registered sender whose key file Gangline checked; this separates registered identities from self-declared names, while any process running as the same user can read the file.

A `HOST:registered:NAME` envelope identifies a registered sender checked by HOST's Gangline; the receiving host trusts that check as it trusts `HOST:NAME`.

A service can supply its key through a private copy such as a user unit's
`LoadCredential` file and call `gang send RECIPIENT --as NAME --credential FILE`.
Removing and adding NAME issues a new key; copies of the revoked key cannot
send again. `registered` is reserved as a peer-host prefix.

`DURATION` uses Go duration syntax (for example, `1h30m` or `500ms`) for
`--at`, `curfew`, and `--timeout`.

A send prints the message ID and `delivered`, `accepted`, `queued`,
`unverified`, or `failed`. See [message receipts](concepts.md#messages-and-envelopes).
`--source watchdog --watchdog UNIT` is the internal watchdog invocation.

## Inspect

| Command | Effect |
| --- | --- |
| `gang roster [--json] [--allow-partial] [--limit N\|--all]` | Show registered agents and current states, with the recorded reason for a failed agent or a blocked, wedged, or unknown one. The table cuts a long reason to its head and its end so that the row's characters fit the terminal's width; `--json` and `gang status --why` carry the whole text. An idle agent with a stored background report reads `idle (N bg DURATION)`: N is the reported task count and DURATION is time since receipt, not verified pending work or idle duration. JSON fields `background_tasks` and `background_since` carry that count and receipt time. The agent remains idle for delivery; the next turn boundary clears the report, but delayed hooks can replace it. An absent count does not certify that no background work remains. |
| `gang status [NAME] [--why] [--json]` | Show state; `--why` includes activity and compaction evidence. Background reports qualify idle as in `gang roster`; `--why` names the report's receipt time, age and count, and states that current task state is unknown. |
| `gang capture [NAME] [-n\|--lines LINES]` | Print the native pane, or only its last LINES lines. |
| `gang capture --composer [NAME]` | Print draft input from the composer. |
| `gang context [NAME] [--json]` | Show native context usage or an unknown reading. |
| `gang context --widget NAME` / `gang context --clear` | Select a context widget for the tmux status line, or clear it. |
| `gang limits` | Query every configured collar's account limits and pace without a live agent. |
| `gang limits NAME` | Show the provider limits an agent observed, with pace. |
| `gang limits -c\|--collar COLLAR` | Query one collar's account limits without a live agent, if supported. |
| `gang snooze [--at TIME] [--note TEXT]` | Schedule a wake for the calling agent at its observed native reset, or at an explicit future time. |
| `gang snooze --status` / `gang snooze --clear [ID]` | Inspect or cancel a pending wake. The lead sees every agent's wake and uncertain notices, and can clear a notice or fallback wake by ID. |
| `gang log [--agent NAME\|HITCH_ID] [--type TYPE\|KIND] [--limit N\|--all] [LOG.jsonl]` | Print recent matching JSONL events from a team or saved log, oldest first. Use `--all` for a full export, including in pipes. See `gang log --help` for the default limit. The command scans the log to select its tail. |
| `gang whoami` | Print the calling pane's registered identity. |

List commands (`queue`, `roster`, `teams`, `sender list`, `collars`, `models`,
`roles`, and `snooze --status`) show a bounded prefix in their existing listing
order. Use `--limit N` to change the limit or `--all` for every entry; each
command's help states the default. A truncated list reports the displayed
and total entry counts on stderr. This limit applies to redirected and JSON
output too. Use `--all` for complete exports, teardown inventories, and checks
that an entry is absent. Display limits do not limit the underlying state
scan or change pending work.

Queue and roster JSON include `total` (the full matching entry count) and
`truncated` (whether entries were omitted). Callers needing complete state
must pass `--all` or check these fields.

`roster --allow-partial` opts into incomplete inventory when a native agent's
initial state read fails or its active, present pane's collar cannot be loaded.
The command omits those entries, retains readable peers, and exits successfully.
Without the flag, roster remains strict; `status` and other commands remain
strict too. The text form prints an incomplete-inventory notice and failure
stages. JSON adds `partial` (boolean) and `errors` (array), always present with
the flag, including `false` and `[]` when neither isolated failure occurs.
The default JSON shape is unchanged.

Each error has exactly `kind`, `hitch_id`, `name`, and `stage`. `kind` is
`native`; `stage` is `state_read` or `collar_load`. A state-read error has null
identity fields: even its directory name is not reported as an identity.
A collar-load error uses the successfully read record's hitch ID and name.
Raw errors, paths and collar contents are not included. Failed entries do not
become fabricated failed-state rows. Collar loading can fail after deadline
or recovery updates, so a partial result does not mean state was untouched.

Errors cover the full attempted native inventory before pagination, with one
entry per isolated failure; their count is not an authoritative missing-agent
count. `total` counts successfully projected rows, including outside
registrations, before pagination. `truncated` describes pagination alone;
`partial`, `truncated`, and `watchdog_available` are independent. An all-failed
native scan can return `agents: []`, `total: 0`, `partial: true`; that does not
establish an empty team. Missing state and stale name bindings retain their
existing skip behavior, and a locked agent still uses its saved row. Shared
inventory, name-binding, later state reread, projection, outside-registration,
mutation, audit, cleanup and output failures remain fatal.

Consumers must install a producer supporting the flag before requesting it.
A caller that requests partial results must treat `partial: true`, nonempty
`errors`, or missing, malformed or inconsistent partial fields as incomplete
coverage. Healthy rows can remain visible, but incomplete coverage must not
be presented as a complete team or used to establish absence.

`--json` on `queue`, `roster`, `status`, and `context` prints one JSON object
for scripts. It carries exact values the human forms abbreviate: queue
timestamps and context token counts. Availability conditions such as
`process_available` and `watchdog_available` are fields. An unobserved context
reading prints `status` `unknown` with its `reason` and null counts, and the
command still exits unknown. The human forms may change layout.

Native agent rows in `roster --json` and `status --json` include
`configured_model`, `configured_effort`, `observed_model`, and
`observed_effort`. Each is a string or explicit `null`; outside registrations
have null values. Configured values are the explicit hitch `--model` and
`--effort` options, retained for new launches. Missing options and older
records remain null. Model identifiers are preserved in full, including any
provider namespace; an alias such as `haiku` is not expanded into a guessed ID.
Launch-policy arguments or native defaults may differ from those hitch options.

Observed values are the last retained native metadata, not a guarantee of
current backend settings. Codex supplies model and optional effort through
`turn_context`; an omitted effort clears the preceding effort observation.
Claude's status-line payload supplies a model ID but no effort. Screen labels,
collar names, process environments and personal profiles are not model sources.
No routing provider is inferred from a model namespace. A clear preserves the
configured launch values and resets native observations; a rehitch records
only options explicitly passed to that new launch. Rehitch preserves the native
conversation's last observations, which can be stale until a new reading arrives.
After Claude compaction or clear, its observed model can be null until a new
status-line payload arrives; null is not evidence of a model change.

Launch values and native effort live in an optional per-agent side record;
the lifecycle state and audit schemas remain readable by older binaries.
Missing, unreadable or unsupported metadata leaves these values unknown and
does not block inventory or lifecycle commands.
Values attach only to the matching registered launch. Native effort also
requires an exact match to the saved state snapshot. An older writer's state
change can therefore make effort unknown until a new observation is retained,
while configuration stays bound to the same launch. A failed launch without a
registration reports nulls. The side record is removed with its agent.

Native `observation` events can also carry `token-usage` readings. These retain
session-cumulative input/output counters from the status-line payload or the
session log, separately from context occupancy and provider quota utilization.
Each reading names its native session, source fields and counter semantics;
session-log readings also carry their source timestamp and ending byte offset.
An absent source timestamp stays absent. The enclosing event records when
Gangline observed it.

When the bounded session-log reader skips unread source bytes, it retains an
`observation-gap` reading with `source-window-skipped`, the native session and
the first readable byte offset. It has no inferred source timestamp. Native
compaction readings also retain their session and source offset. These records
use the existing audit-before-cursor-save path; no extra collector is started.
On an initial resumed read, a skipped window may contain history from before
the current hitch; the gap does not prove missed work during that hitch.
An interrupted cursor save can repeat the same gap with the same source
identity. Older binaries can decode these readings but their metadata views
omit the new boundary kinds; use a reader that projects them for this coverage.

The saved session-log context reading also carries its session and ending byte
offset as a partial source witness. Each refresh with an eligible witness reads
at most 64 KiB ending at that offset, through the same open file as the normal
bounded suffix read, even if no new bytes arrived. It decodes only that complete
record with the native decoder and compares its timestamp, context used count
and context limit. A changed measurement retains `source-context-mismatch` as an
unknown observation gap before resetting the stale context to unknown; metadata
and summary expose `source_context_mismatch`. Later suffix measurements can
supply a new context. The already consumed prefix is not replayed or recovered.

This comparison detects only a changed, decodable context record at the saved
position. It does not identify a replacement generation, compare other bytes,
or detect rewrites preserving that measurement. Missing or legacy unbound
context, compaction-reset context, wrong-session context, oversized or malformed
records and records without a complete terminating newline stay unverifiable;
none certifies continuity. Existing size/session refusals still apply. An older
saved context acquires the binding only after a new context reading. The extra
read is bounded independently of transcript length, in addition to the existing
header and tail budgets. Concurrent rewrites are not an atomic snapshot.

The binding uses reading fields introduced in commit `80d70795`. After a bound
context is saved, rollback to a binary predating that commit is unsupported:
its strict decoder rejects those fields and leaves agent state unreadable.
Context audit copies omit the binding; paired usage already carries its own
identity. Compatible predecessors at or after that commit accept the state and
raw gap kind but do not perform this check or project its gap. They can discard
the binding when saving a later context. Failed audit writes keep the witness
and cursor pending. Failed state saves can repeat the gap; after a successful reset,
an unchanged source adds no further mismatch. A retained compatible reader is
still needed to project this gap after rollback. No counters are summed, and a
matching witness or absent gap never proves continuity.

Missing counters remain absent with `unknown` or `partial` status. A reported
zero is retained as zero. An invalid usage counter becomes an unknown usage
reading without discarding valid context readings or stalling the source cursor.
Cache inclusion is `unverified`, so a cached-input
counter must not be added to input tokens. Repeated snapshots are not additive;
lower values or a new session need separate coverage segments. These raw
counters alone do not establish task usage, complete session coverage, costs
or per-agent time. The existing `gang log --all LOG.jsonl` reader preserves
them; no new polling or event publication path is introduced.

The session-log cursor and derived native state advance only after observations
are appended to the audit; an audit failure deliberately leaves them pending.
If saving the cursor then fails, retry can repeat an observation
with the same native session and source offset. Consumers must deduplicate
those source identities; append-before-save is not exactly-once delivery or
a power-loss durability guarantee.

Status-line observations append only when readings change. Returning to an
earlier state appends again: writes follow actual changes, not repeated samples.
The comparison
includes the hitch, registration, native session, unknown reasons and source
timestamps; it excludes only the quota timestamp generated on each invocation.
A private receipt under the existing short hook lock records the last audit
location and record. Suppression requires those exact bytes still to exist in
that log, so a missing, truncated or replaced stream cannot inherit a marker
without its evidence. A restored stream containing the exact record remains
valid evidence. Work per observation is bounded by that snapshot's size, not
the accumulated history. The native input lock is not required.

The audit append precedes receipt replacement. Receipt failure is reported and
retry may append a duplicate; losing the receipt also permits a duplicate.
An unreadable receipt fails visibly. This does not repair existing partial log
records, provide a heartbeat, or establish duration, uninterrupted coverage or
exactly-once delivery. Retained history is unchanged; the receipt is removable
runtime state and no retention expiry is introduced.

For an allowlisted metadata page without message bodies or free-text diagnostics,
use the same audit reader with an explicit file:

```sh
gang log --metadata --offset 0 --max-bytes 65536 LOG.jsonl
```

The `gangline.observability-log.v1` JSON result contains activity observations,
native session bindings, cumulative token readings and lifecycle/outage markers.
It excludes envelopes, reasons, prompts, tool payloads and compaction continuation
text. Unknown usage fields remain absent; an observed zero stays zero. Host,
team and task attribution are not inferred from names or a file path. The caller
must supply its separately verified source scope. This is a file reader, not a
new collector or a live roster endpoint.

The default read budget is 64 KiB, with an explicit maximum of one MiB.
This keeps schema-validation work bounded as well as file I/O; callers can
request smaller pages for interactive views.
Each request reads at most that budget plus one record-boundary byte. It returns
the file size at open, requested offset, read end, SHA-256 of those read bytes,
next complete-record offset, read time, `more`, events and structured gaps.
Observation events can include `native_boundaries` for session-log compaction
checkpoints, completion and skipped source windows. Only fixed boundary kinds,
source identities, offsets and supplied timestamps are projected.
Within the selected page and hitch/source/session scope, comparable counter
decreases, source-offset rewinds and conflicting replays produce explicit gaps.
An identical replay at a previously seen source offset does not establish a
reset. These comparisons cannot distinguish every replacement or restart;
absence of a gap is not proof of continuity. Unknown readings and a new page
do not provide a usage baseline, and no historical bytes outside the budget
are scanned to establish one.
`--agent` and `--type` can filter the page; prefer hitch identity to a reused
display name. `--all`, `--limit` and `--team` do not apply in metadata mode.

For an interactive view of the latest bounded audit window, use:

```sh
gang log --summary --max-bytes 65536 LOG.jsonl
```

The `gangline.observability-summary.v1` result reads only the file's tail within
the same byte budget, plus one boundary byte. It skips an initial partial record
with a `tail_starts_inside_record` gap and marks excluded earlier history with
`omitted_head`. Read offsets, hash, read time and gaps have the same meanings as
the metadata page. `more` means bytes remain after the last complete record,
not an instruction to poll; this command performs one read, without pagination.
It does not accept `--metadata`, `--offset`, `--agent`, `--type`, `--all`,
`--limit` or `--team`.

`subjects` groups observations by hitch identity, never by name. It returns the
last activity observation and the last usage observation for each distinct
source/session pair in append order, retaining their record offsets and original
timestamps. A later unknown reading replaces an earlier known reading in the
same pair; missing identity readings remain separate. All selected lifecycle and
outage markers, including observations carrying `native_boundaries`, remain in
`boundaries`. Boundary copies retain the same event offsets and are not
additional measurements. Subjects are sorted by hitch, and usage groups by
source/session key, not time. Within each identified group, "last"
means the greatest append offset. Missing-identity entries and boundaries can
grow to the read budget; no fixed entry count is promised. A hitch seen only in
a boundary has an empty usage list and null activity. These observations may predate a drop,
session change or gap. Match them to an independently verified current roster
before presenting current subjects; the summary itself does not establish
current registration, session, freshness or continuity. An absent subject means
only that this window contains no projected record for it.

Each subject also returns a `projected_observation_window`: the first and last
projected event's observation time and byte offset in append order, plus the
projected record count. It excludes event types not supported by the metadata
projection; these are not all appends or a registration lifetime. Coverage is
`partial`, freshness is `unknown`, and coverage before the first and after the
last projected event is `unknown`, even when the complete file fits.
`observation_times_ordered` describes only this hitch's projected timestamps;
it is false for a regression or missing timestamp. Equal timestamps do not
establish native event order. The page's `observation_time_out_of_order` gaps
instead compare all decoded records, including other hitches and unprojected
events. Neither check establishes continuous observation.

For callers using the single bounded summary instead of reconstructing metadata
pages, `state_intervals` retains adjacent activity observations in append order,
with compact endpoints (activity, original offsets and timestamps; subject
identity belongs to the containing subject). Each pair includes counts of
intervening projected boundaries (this hitch or no hitch) and gap records (any
hitch, since gap records lack subject identity), by offset after the first
endpoint through the second. These counts do not count omitted/unprojected
lifecycle facts or unseen gaps. `observation_spacing_seconds` is only the spacing
between increasing endpoint timestamps. It is null for any intervening counted
break, equal/missing endpoint times, a disorder in this hitch's projected window,
or an endpoint after `read_at`. Writer observation times and reader `read_at`
can come from different clocks; this refusal does not diagnose clock failure.
Usage `source_at` remains the source-supplied time when available; activity
events do not supply a separate source timestamp.

Every pair has `coverage: "unknown"` and null `state_duration_seconds`:
change-only observations cannot establish that either state held throughout the
interval, even when the activities match or no gap was detected. Lifecycle,
compaction and outage boundaries and detected gaps remain separately visible in
the same result. No interval is extended to read time or to omitted history. No
spacing is summed or attributed to busy, idle, runtime-blocked, compute or
semantic task-blocker time. A runtime-blocked observation is not a task blocker.

Cumulative readings are not added or subtracted. `window_usage` and aggregate
`state_duration_seconds` remain null: the selected evidence does not establish
attributable usage baselines or uninterrupted state intervals. Replays and
session/source groups are not additive; missing counters remain unknown and
cache inclusion retains its recorded meaning. The extra projection uses only
the already bounded page, with no additional file reads. Each selected activity
record participates in at most two endpoint pairs; the output can grow with
records inside the byte budget, rather than only with subjects. The byte budget
limits input reads, not serialized response bytes. Building coverage takes time
and memory linear in the projected records and gap records. A caller must use its authorized fixed
file and host/team scope and fit this one read within its existing collection
budget; no per-subject requests are needed.

An unterminated tail or record cut by the budget is left unconsumed and named in
`gaps`; retry from `next_offset`, increasing the budget if a record exceeds it.
A first record larger than the page is marked `record_exceeds_budget`. If that
offset does not advance, do not poll indefinitely: the source needs a complete
record or a larger allowed page. A record beyond the maximum budget remains
unavailable through this mode; the reader does not silently skip it. Invalid complete records are skipped
with a gap rather than their decoder error or contents. Within-page observation
time disorder is also reported; concurrent writers can append out of timestamp
order, so that marker alone does not prove clock failure or damaged data. A non-boundary offset or offset past the
current end is refused. Nonregular files are refused.

Pages do not prove continuity across file replacement, restart, compaction or
clock changes. The page hash witnesses only its returned byte range; it is not
a team incarnation or global source identity. Records can be missing before,
between or after pages, and empty results do not establish zero usage. Activity
timestamps mark observations, not continuous busy/idle time or exact compute.
Session cumulative snapshots must not be summed. This projection supplies no
duration totals, cross-page deduplication or historical task assignment.

`gang log --type session_bound` joins native sessions to hitches. Gangline
logs a `session_bound` event whenever it binds a hitch to a native session
it did not hold before, once the native harness reports that session with
its transcript: at the first native report, which for a launch with
`--resume` confirms the requested session, at a verified session change,
including one a drop finds before any tick saved it, and at the first save
of an agent that held a reported session before it was bound. `binding`
carries `session_id`, `previous_session_id` on a change, the `collar`, and
the parent hitch `hitched_by`. A rehitch keeps its hitch ID and resumed
session, so its binding stands. A save that fails after its binding is
logged binds again at the next save, so read bindings by `hitch_id` and
`session_id`. The event stays in the log after the agent is dropped.

`gang log` warns on stderr of a line that holds no valid event, such as a
record a full disk cut short, prints the intact event a later write appended
to that line, and continues the export.

Pane borders use `?name?` for changing or unknown state, `~name~` for idle,
`-name-` for work, and `!name!` for native decisions, wedges, or failures.
An unsubmitted operator draft holds message delivery but keeps the idle
symbol, or the work symbol while a witnessed turn is still open.
Hooks and a five-second watchdog refresh the symbols; `gang status` and
`gang roster` also observe and refresh them. See
[native events and observation](internals.md#native-events-and-observation).
Use a hitch ID in log filters to follow a registration across renames.

Each `activity_observed` event records what its reading was derived from.
`basis.screen` is the collar's reading of the pane: `blocked`, `compacting`,
`idle`, `busy`, `unsubmitted`, `unreadable`, or `unread` when no screen was
read successfully. `basis.rule` is the rule that set the activity: `screen`, `open-turn`,
`interrupt-pending`, `compaction-record`, `turn-failure`, `wedge`,
`probe-failure`, `native-hook`, or `permission-request`. A fresh native
permission request reads blocked until a later hook unless the collar sets
`primitives.blocked.params.permission_hook` to `screen`. The Codex collar
uses that setting because its request hook precedes automatic approval; status
reads the pane and reports blocked only for a visible input choice. An
unreadable pane stays unknown. An idle screen can confirm dismissal; dismissal
itself fires no hook. Hook-derived readings carry no screen fingerprint.
`basis.compaction` is the compaction record's status, and
`fingerprint` is a hash of the screen. Neither carries screen text, and
neither does the reason: a blocked reading names the native prompt by a hash
of the text the collar matched, so a prompt whose matched text differs logs a
new reading, and a wedged
reading names the collar's threshold. `gang capture NAME` prints the prompt.

A native turn that fails reads unknown with the reason
`native turn failed: CLASS: MESSAGE` under rule `turn-failure`. A turn the
provider refused for its content (Codex class `cyber_policy`) reads blocked
with `native turn refused: CLASS: MESSAGE` instead. The next submitted
prompt clears the failure.

Provider errors such as an exhausted retry limit or an HTTP 429 or 5xx also
read unknown with the native error text. A structured native error class can
leave the reason without a class prefix. Inspect `gang status NAME --why`,
then send an explicit continuation when retrying is appropriate.
`gang hitch NAME --recover` recovers startup delivery, not execution: it
reports an already delivered startup without submitting it again, even when
that turn failed. Delivery does not establish that the assignment completed.

## Discover and maintain

| Command | Effect |
| --- | --- |
| `gang collars` | List bundled and custom collars. |
| `gang collar check NAME` | Probe an installed harness in a private throwaway tmux session, in the current repository with the hooks a hitch installs. A hook still awaiting native review, Gangline's included, makes the check report unknown; review hooks where a hitched agent starts. |
| `gang models [-c COLLAR]` | List native models and efforts; also accepts `--collar`. |
| `gang roles` | List role briefs. |
| `gang config` | Print effective settings and their sources. |
| `gang statusline [--install]` | Render native status-line JSON; `--install` fills an absent `statusLine` in `settings.json` under `CLAUDE_CONFIG_DIR` (default `~/.claude`) and names that file. |
| `gang --version` or `gang version` | Print the version. |
| `gang upgrade [-y, --yes]` | Print the installed and latest stable versions, confirm on a terminal (`[y/N]`, default no), then install into the installer-managed checkout. Use `--yes` without a terminal. A current install exits 0 without asking. |
| `gang help [COMMAND]` | Show command help. |
| `gang hook` | Read native hook JSON from stdin; invoked by the collar integration. |
| `gang peer-authorize HOST PUBKEY_FILE` | Print the `~/.ssh/authorized_keys` line that binds enrolled `GANG_PEERS` host HOST's public key to `gang peer-receive` for the selected team: `command="GANG peer-receive --team TEAM --peer HOST",restrict TYPE KEY gangline-peer-HOST`, where GANG is this `gang`'s path. PUBKEY_FILE holds exactly one OpenSSH public key line with no options; its comment is dropped. Stderr names the setup steps on HOST. A dedicated restricted key supplies the binding; unrestricted SSH access to the receiving account can invoke `peer-receive --peer` directly. |
| `gang peer-receive --peer HOST` | The forced command `gang peer-authorize` prints; its dedicated restricted-key setup binds HOST to an enrolled host. It runs only under this account's sshd session beneath a root sshd; that check does not authenticate the key or distinguish host-root from namespace-local root UIDs; receiving-account shell and state access are trusted. Read one JSON request (`host`, `team`, `id`, `from`, `to`, `body`, and optional `class`) from stdin and deliver it from sender `HOST:NAME` or `HOST:registered:NAME` as message `peer-HOST.ID`. Print one JSON line with the message ID and outcome, or the error that published nothing. Delivery of a repeated ID is at-least-once: one that arrives after the first copy is delivered, or after a later message's receipt replaces its receipt, is delivered again. [Peer labels](concepts.md#messages-and-envelopes) states what the binding leaves open. |

## Configuration

Settings come from `$GANG_CONFIG_DIR/config`, one `NAME=VALUE` per line.
Blank lines and comments beginning with `#` are allowed. The file is parsed,
not sourced; unknown, duplicate, or blank settings fail. Environment variables
override file values. Run `gang config` to see effective values and defaults.

| Setting | Controls |
| --- | --- |
| `GANG_SESSION` | Team and tmux session name when `--team` is absent; defaults to `gangline`. |
| `GANG_COLLAR` | Default collar; defaults to `claude`. |
| `GANG_COLLARS` | Absolute directory of custom `NAME.cue` collars and bundled collar overlays. |
| `GANG_LAUNCH_ARGS` | JSON object mapping collar names to extra launch argument arrays. |
| `GANG_CODEX_PERMISSION_PROFILE` | Codex permission profile name for hitched agents; unset by default. |
| `GANG_CAPACITY_TIMEOUT` | Positive duration budget for continuations after a capacity error the collar recognizes. |
| `GANG_BACKGROUND_IDLE_NOTICE` | Positive age of a stored background report before an idle agent's hitcher is told; defaults to `30m`. The warning preserves the reported count and receipt time without certifying current task state. |
| `GANG_PEERS` | JSON object of enrolled peer host names to `{"ssh": ALIAS, "user": LOGIN, "gang": PATH, "team": TEAM}`: the ssh alias and optional login that reach the host, the absolute path of its `gang`, and its team. An enrolled host's messages are accepted through the forced command `gang peer-authorize` prints for its key. `gang send HOST:NAME` runs `ssh ALIAS PATH peer-receive --team TEAM`; HOST runs its forced command for the key in its place. No two host names may differ only by case. Unset means no peers. |
| `GANG_HOST` | This host's name as its peers enroll it in their `GANG_PEERS` and authorize its key; required to send to a peer, which refuses a request naming any other host. |

Set `GANG_CODEX_PERMISSION_PROFILE=gangline` in the config file or environment
to select that profile for Codex hitches. The named profile must exist in
Codex's own config. An unset value leaves profile selection to Codex's own
config. Codex may load other config layers that override the selected
permission mode.

Gangline does not add Git write roots or select an approval policy or
reviewer; operator-supplied arguments, including explicit `--add-dir` grants,
are passed through unchanged. See
[sandboxed development teams](operations.md#sandboxed-development-teams).

These variables are environment-only:

| Variable | Default and purpose |
| --- | --- |
| `GANG_CONFIG_DIR` | `${XDG_CONFIG_HOME:-~/.config}/gangline`; absolute config directory. |
| `GANG_SENDER_CREDENTIAL` | Key file used by `gang send --as NAME` unless `--credential FILE` supplies one; ignored by sends without `--as`. |
| `GANG_STATE_ROOT` | `${XDG_STATE_HOME:-~/.local/state}/gangline`; team state root. |
| `GANG_TMUX_SOCKET` | tmux's default socket; selects a separate server when set. |
| `GANG_TMUX` | `tmux`; executable used for tmux commands. |
| `GANG_AGENT_NONCE` | Per-hitch identity capability, inherited by the harness; only its digest is stored in the registration. |
| `GANG_AGENT_ID` | Set on a hitched pane; identifies its registration together with `GANG_AGENT_NONCE` and the tmux server generation, session and pane. |

Identity and delivery use tmux's pane registration and foreground command,
so they work when the caller cannot see host PIDs. tmux on macOS names the
foreground process after its executable file, so a harness launched through a
symlink, such as a native Claude Code install, is recognised from the process
table by the name it was invoked as; that needs the host process namespace
visible. Native ancestry checks and
detached-descendant cleanup remain enabled when the host process namespace is
visible. Otherwise `process_verification_unavailable` records the skipped
checks. `roster` and `status --json` report current process visibility from
the caller's namespace after checking the pane registration; an absent pane
or exited process is unavailable. The agent's own command passing ancestry
verification from the PID namespace recorded at hitch logs
`process_verification_available`. The process watchdog is
also skipped visibly with `watchdog_unavailable` and `[watchdog-unavailable]`,
and `hitch` and `snooze` warn on stderr; ordinary hooks and commands still tick
the team. The next armed timer clears the marker and logs `watchdog_available`. A pane-only drop cannot
prove that detached descendants exited. Later ordinary panes do not inherit
the per-hitch capability.

`--team TEAM` selects another team in the same state root and tmux server for
one command. For a separate team, keep its selection in the shell environment
for every command. A separate state root and socket also isolate its files and
server. For a disposable team, create both in a temporary directory:

```sh
export GANG_SESSION=review
export GANG_STATE_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gang-XXXXXX")"
export GANG_TMUX_SOCKET="$GANG_STATE_ROOT/tmux.sock"
gang up
```

`gang down` ends only the agents' panes and keeps the state root. To discard
the team, end its server and remove the root:

```sh
gang down --yes
tmux -S "$GANG_TMUX_SOCKET" kill-server
rm -rf "$GANG_STATE_ROOT"
```

Installer variables:

| Variable | Default and purpose |
| --- | --- |
| `GANGLINE_HOME` | `~/.local/share/gangline`; retained release checkout. |
| `GANGLINE_BIN` | `~/.local/bin`; installed command directory. |
| `GANGLINE_REPO` | Public Gangline Git repository; release source. |
| `GANGLINE_RELEASE_BASE_URL` | GitHub release download root; override for another release host. |
| `GANGLINE_CLAUDE_STATUSLINE` | Set to `1` to install Claude Code's status line without `claude` on `PATH`. |

## Startup instructions

Files under `GANG_CONFIG_DIR` can customize the bundled instructions:

| File | Effect |
| --- | --- |
| `CONTRACT.md` | Replace the delivery contract. |
| `DOCTRINE.md` | Add operator policy. |
| `roles/NAME.md` | Replace or add a role brief. |

Operator policy takes precedence over role briefs. Put persistent staffing,
provider, model, and effort choices in `DOCTRINE.md`. These files are read
when the agent is hitched.

## Harnesses and platforms

A collar is a CUE file that tells Gangline how to launch and communicate with
a particular harness. A collar's canonical name is the harness binary it
launches: `claude` or `codex`.
Each uses the installed native CLI and its account settings. Use
`gang models -c COLLAR` for the model and effort identifiers the native
catalog lists; the native CLI may accept models it does not list. `hitch`
reads the catalog only for `--effort`. When the catalog cannot be read,
`hitch` prints its diagnostic and launches. Gangline leaves native
permissions, login, and trust decisions to the operator. Resumed collars must
include the requested native session ID in their submit witness.

In `GANG_COLLARS`, a `NAME.cue` matching a bundled collar overlays its fields;
other names require a complete collar.
Nested fields merge; a changed primitive
name replaces that primitive and its parameters. Lists replace the bundled
list. Each `context_bands` selector value replaces its entire band list;
other selectors remain. Each `usage_bands` window replaces its band list.
For example, `claude.cue` can set only:

```cue
collar: {
	context_bands: {
		"*": [{name: "early", at: 0.10}, {name: "late", at: 0.20}]
	}
}
```

What a collar renders into the launch command (`launch`, `models.option`,
`options`, and hook `install_args`) takes effect when an agent is hitched. An
`options.role_prompt_file` template receives the path to a private file in the
agent's state directory containing the unchanged composed startup prompt;
the native CLI reads it as a system prompt. Claude Code uses
`--append-system-prompt-file`. A collar without this option receives the
standing instructions in the startup message instead. The prompt file is
removed with the agent's state on drop. The assignment remains in the startup
message either way.
An agent hitched with `--context-bands` keeps its thresholds until re-hitched;
Gangline reads band names, messages, and model selectors from the collar for
each later operation. A later selector with a different band count uses its
collar thresholds until the agent is re-hitched. Without the option, the collar
also supplies thresholds.

A native submit precedes UI paint. Gangline bridges an idle-looking frame
with a brief busy display grace when the collar leaves open turns to the
screen. That grace never records a native finish. Its duration is
`submitPaintWindow` in [`activity.go`](../cmd/gang/activity.go).

A harness can stream a reply with no busy marker on screen. When the
`hook-boundary` turn boundary sets `open_turn_quiet`, a turn whose submit was
witnessed reads busy on an idle-looking screen until its finish or failure
hook arrives, an interrupt brings the composer back, or the screen stays
unchanged for that long after the current submit. A turn that ends without its hook, as after a
keyboard Escape, reads busy for that window; a shorter window lets a pause
in streamed text read as idle.

A harness can also queue a prompt typed while a turn runs and start it as its
own turn, with no submit hook, once that turn ends. When the turn boundary
sets `queued_turns: "claude-transcript"`, a finish hook leaves the turn open
while the Claude Code transcript still holds a queued prompt or records one
leaving the queue for the next turn. If the transcript cannot be read, the
finish hook closes the turn. A queued turn's finish also clears a native turn
failure when the transcript records the queued prompt after the failed one. A
queued turn's failure is recorded against the queued prompt's id when the
transcript records that prompt after the witnessed one, and a later submit
clears it. A failure with no prompt id clears when the witnessed turn, or a
turn the transcript records after it, finishes.
A wake typed while a turn runs is judged by the turn the transcript records
running it, under the prompt id of the wake or of any prompt dequeued into
that turn with it. No finish hook judges the wake while it waits in the
queue, a wake the running turn absorbs is judged by that turn, and if the
transcript cannot be read the wake is judged by the turn it was typed into.

A harness can move a conversation to a new native session, as Claude Code
does when its conversation moves to the background, and keep submitting from
the same pane into the new session. When the turn boundary sets
`session_moves: "claude-transcript"`, gang follows a submit witness into a new
session only when the new session's transcript sits beside the old one and the
old transcript ends with Claude Code's record that the conversation continued
in that session, with no prompt or reply after it. Moves are followed record by
record. A mapped hook that names a spawned agent (`agent_id`) is rejected even
when it reports the registered session ID. A submit hook from another session
that leaves a transcript the collar maps missing, null or empty is also
rejected. Gang keeps the agent's registration and witness and logs a
`native_hook` event with status `foreign-session`. For rejected submits it
also keeps the raw payload in
`STATE_ROOT/teams/TEAM/foreign-submits/HITCH_ID.json`, which `gang status NAME
--why` names. A witness from any other session is never followed: the agent
fails, and its failure reason names the recovery.

Each context band has a `name`, threshold `at` (a fraction from 0 to 1), and
optional `message`. A nonfinal band without a message advises saving state and
compacting at the next good stopping point; the last orders compaction now.
Messages use these tokens; token counts are integers and percents are rounded:

Usage bands are named thresholds under `usage_bands.five_hour` and
`usage_bands.weekly`. A band's `ahead` is a fraction from 0 to 1 of the window:
the band crosses when the window's used share runs ahead of its elapsed share
by at least that much. A window three days into a seven-day week has 43%
elapsed, so a band with `ahead: 0.10` crosses at 53% used. Bands read native
provider usage for the collar's account and notify the active lead once per
band and reset window. Of several windows of one kind, the one furthest ahead
of its pace is measured. A collar
overlay can replace either window independently. When one reading crosses
several bands in a window, identical messages are sent once; every crossed
band is recorded, and distinct operator guidance is preserved:

```cue
collar: {
	usage_bands: {
		five_hour: [{name: "early", ahead: 0.15}, {name: "late", ahead: 0.30}]
	}
}
```

An optional per-band `note` appends operator guidance to the measurement;
its default is empty. An optional `message` replaces the measurement template.
Both accept `{{band}}`, `{{collar}}`,
`{{window}}`, `{{threshold_percent}}` (the band's `ahead` as a percent),
`{{used_percent}}`, `{{elapsed_percent}}`, `{{pace_points}}` (used minus
elapsed, signed), `{{exhaustion_at}}` (when the average rate so far reaches
100%, UTC RFC3339, or "after the reset"), `{{reset_at}}` (UTC RFC3339), and
`{{snooze_command}}`. With no message, the notice reports only provider, use,
elapsed share, pace, projected exhaustion, and reset time. Unknown native
window durations are ignored; Gangline does not estimate provider usage.

| Token | Value |
| --- | --- |
| `{{band}}` | Band name. |
| `{{threshold_percent}}` | Threshold percent. |
| `{{used_tokens}}` | Used tokens. |
| `{{limit_tokens}}` | Context limit in tokens. |
| `{{used_percent}}` | Reported usage percent. |
| `{{model}}` | Reported model. |
| `{{agent_name}}` | Agent name. |
| `{{compact_command}}` | `gang compact --note 'Resume from FILE'`. |

Claude Code context readings come from its status line. Codex readings come
from its native session log. Both collars also have a standalone limits
query that needs no agent. Codex's runs a private native app server without
creating a conversation or turn. Claude Code's runs `claude -p /usage` as a
local command, refuses a result that reports a model turn, and reads the
account usage that command caches in Claude Code's global configuration.
`/usage` fetches again only when its cache is not recent, so the reading's age
is the cache's. Its windows include model-scoped weekly
limits, labelled `seven_day/MODEL`, which the status line does not report.

A collar overlay can name costly models in `hitch_guard`. `up` and `hitch`
check a `--model` the guard names against the collar's provider usage before
anything starts; a hitch without `--model`, or with another model, is not
checked. Thresholds are fractions from 0 to 1 for `five_hour`, `weekly`, or
both, and a guard needs at least one:

```cue
collar: {
	hitch_guard: {models: ["costly-model"], five_hour: 0.8, weekly: 0.9}
}
```

The reading is the freshest one an agent on the same collar recorded in the
last five minutes, or else the collar's limits query, as in
`gang limits -c codex`. Every five-hour or weekly window in that reading
counts, whichever limit bucket reports it, as for usage bands. A window at or
past its threshold that has not reset makes `hitch` warn, then launch; with
`refuse: true`, `hitch` refuses instead. A stale or missing reading never
refuses. Bundled collars set no guard.

`gang snooze` is a manual override for an active agent. On an attributable
provider cap refusal, Gangline automatically schedules a wake at the observed
native reset. With no `--at`, `gang snooze` uses a recent
native five-hour or weekly reset from that agent's collar, selecting the
most-used window. `--at` accepts a duration, local `HH:MM`, or RFC3339
timestamp. `--note` is delivered with the wake. A manual wake notice names
its ID, original caller and due time. A pending wake is durable;
repeating the command replaces it until it is submitted. After a wake is
delivered, the agent can schedule a follow-up while its native turn runs.
The follow-up waits for the earlier wake's outcome before delivery, even if
its due time passes; both wakes appear in `--status`. The due wake is sent
to the caller, or the active lead if the caller is gone. `--status` shows
scheduled, queued, submitted, or failed wakes. For the lead it also lists
every other agent's wake with its due time and note, and uncertain usage
notices; `--clear ID` removes a notice or a wake routed to the lead. An
agent can clear both its scheduled wake and retained outcome with `--clear`.
A wake still queued for native input is withdrawn from the recipient's inbox;
clearing cannot
retract input the harness already accepted.

The team log records each wake stage under the agent that scheduled it, so
`gang log --all --agent NAME` shows the life of NAME's wakes: `snooze_scheduled`
(with its due time once known), `snooze_cleared` (cleared, replaced, or
superseded), `snooze_rearmed` (given a new ID because its recipient is gone),
`snooze_completed`, and `snooze_failed`. A wake pulled out of the native
queue back into the composer has no turn of its own: the recipient's next turn
end logs `snooze_failed`, and the wake still completes if it is submitted again
and its turn finishes. Delivery of a due wake is logged as
`send_queued` and `delivery_*` events under its recipient. A usage-cap
rejection of the wake's turn and the wake that replaces it are logged as
`snooze_cap_rejected` and `snooze_rearmed` under the agent whose turn was
rejected. See [operations](operations.md).

Linux uses systemd user timers for the watchdog, or a detached sleeper
process where no systemd user manager runs; macOS uses transient launchd user
agents. On a host without a supported user scheduler, ordinary commands and
ticks remain available. See the [watchdog guide](guides.md#keep-an-unattended-team-moving)
and [internals](internals.md#watchdog).

## Exit status

| Status | Meaning |
| --- | --- |
| `0` | Successful command; a send may still be queued or accepted into native input. |
| `1` | Execution or I/O error. |
| `2` | Bad usage; the message names the problem and, for a bad value, the accepted form. A known command's usage follows. |
| `3` | Refused operation. |
| `4` | Native harness needs attention, including a send with a failed delivery receipt. |
| `5` | Unknown result, such as input without a confirmed receipt. |

## Private evidence joins

`gang evidence verify --assignment ID [--require ci|usage|deployment] MANIFEST.json` checks an
explicit local evidence chain. `gang evidence show` takes the same arguments
and prints the same JSON view. Neither command captures a session, installs
hooks, runs a recorded command, fetches CI, or changes live team state.
Artifacts must be regular files reachable within the manifest's directory;
paths and symlinks escaping that directory are refused. Artifact reads stop
after their declared size plus one byte; oversized or changed content fails
integrity. The manifest input is bounded to 4 MiB.

`verify` exits zero only for `pass`, refused for `fail`, and unknown for
`incomplete` or `not_exercised`. `show` exits zero when it successfully produces
the view; inspect its `status` to learn the evidence verdict. Missing artifacts,
partial native records and absent results never become a passing chain.
The default stage checks through CI. `--require usage` separately checks
retained generation charges and phase summaries; it cannot pass a code, review
or CI edge. `--require deployment` checks the full CI chain plus the explicitly
selected installed readback and acceptance. Retention, local restore and
backend-effort coverage remain unavailable or not exercised.

### Collect, record and restore locally

Preserve an explicitly enrolled manifest and its declared artifacts in a new
private directory:

```sh
gang evidence collect --assignment ID MANIFEST.json NEW_BUNDLE
```

To add historical records, supply a JSON array using the record schema below:

```sh
gang evidence record --assignment ID MANIFEST.json RECORDS.json NEW_BUNDLE
```

`record` appends records using the existing identity rules: identical repeated
records coalesce; conflicting identities are refused. It does not replace the
input manifest. Imported artifact paths resolve relative to **MANIFEST.json**,
not the records file. Both commands copy every declared `available` artifact,
including metadata not referenced by the selected target. They check declared
sizes and digests using the same bounded, directory-confined reader as `verify`.
Unknown payload bytes are copied unchanged. `withheld` and `unavailable`
references stay in the manifest without opening or copying their paths. Only
explicit inputs are read; there is no discovery of sessions or live files.

The bundle contains `artifacts/RECORD_ID`, the exact `source-manifest.json`,
and, for `record`, the exact `imported-records.json`. The self-contained
`manifest-content.json` rewrites available artifact paths into `artifacts/`.
After file and directory synchronization succeeds, `manifest.json` is published
as a no-replace hard link to that content file, then the directory and parent
are synchronized. Directories are private to their owner, as are bundle files.
An existing destination is refused. Collection leaves source files unchanged
and never deletes partial output. Before publication, a failure leaves no
`manifest.json`; a synchronization failure after publication reports failure
with durability unconfirmed even if the complete manifest is visible. Inspect
and retain failed destinations as needed; retry into a new directory.

Success prints JSON with `status: collected`, copied/withheld/unavailable
artifact counts, and input/output manifest digests. It establishes preservation,
not acceptance or source authentication. `--require` applies only to
`verify`/`show`. Missing or changed available inputs stop collection; withheld
markers do not become complete evidence. A failed chain can still be preserved.

Restore by copying the **whole bundle** to a new empty local directory, then
use its manifest. For example:

```sh
mkdir -m 700 RESTORED
cp -a NEW_BUNDLE/. RESTORED/
gang evidence verify --assignment ID RESTORED/manifest.json
gang evidence show --assignment ID RESTORED/manifest.json
```

These queries need only restored files, not the original manifest, runtime or
source checkout. Copying and re-verifying preserves the existing verdict and
coverage; it does not infer retention guarantees, expand enrollment, resume a
native session or enable automatic teardown. No offsite service is involved.

### Manifest records

A manifest has `version`, `scope`, and `records`; `target_code` is required for
the through-CI and deployment checks and may be omitted for usage. Deployment
also requires `target_deployment`, naming one deployment record; it never picks
the newest report implicitly. The scope holds
`host`, `team_instance`, `hitch` and `assignment` identities. Each record has a
unique `id`, a `kind`, `provenance` (`manual_import` or `producer_export`), and
one corresponding payload. Optional `required_checks.tests` lists exact argv
arrays; `required_checks.jobs` lists required CI job names. The verifier checks
these when supplied and reports the accepted argv/cwd and CI run/attempt/jobs.
Without such a policy it reports `required_checks: caller_declared`; it does
not discover repository requirements. Use the schema version accepted by the installed
reader; unsupported versions and conflicting duplicate record identities fail.
Identical duplicate imports do not create another attempt. Give distinct test
executions, reviews and CI attempts distinct IDs even when their bytes match.

| Kind / payload | Fields and checks |
| --- | --- |
| `artifact` | `path`, `sha256`, `size`, `status`; optional `checkpoint` names an existing checkpoint. `available` bytes must match their digest and size. `withheld` and `unavailable` sources remain incomplete and are not opened and their path/content is not printed. Checkpoint identifiers are displayed as unverified metadata; they do not replace the bytes or prove code identity. |
| `run` | `native_artifact`, `harness`, `session`, `turn`, `implementer`; optional `requested_model` and `requested_effort` are imported assertions. The Codex reader checks session identity and a successful terminal record for the selected turn. `harness: claude` reads a retained Claude Code transcript with `sessionId` and selects `turn` by `promptId`; the supported completion and witness rules are below. Other formats remain incomplete. Observed model is separate from requested model; backend effort remains unverified. |
| `test` | `run`, `result_artifact`. The structured runner result must name the same run and tested input tree, with a successful numeric subprocess exit. Printed success or the outer worker's exit is insufficient. |
| `code` | `run`, `native_call`, `repository` (`owner/repo`), `commit`, `commit_artifact`, `tree`, and arrays `tests`, `reviews`, `ci`. `target_code` selects this record. The raw Git commit object must hash to the commit and name the tested tree. The selected native turn must contain a tool result with `native_call` whose output names the exact commit or tree. Coverage distinguishes `native_tool_result_commit` from `native_tool_result_tree`. Only a commit match witnesses that commit in the session; a tree match witnesses tested content and does not establish that the session made the commit. Neither authenticates authorship. Record all required checks in the arrays; an empty array is incomplete. |
| `review` | `result_artifact`. The reviewer attestation must target the exact repository/commit, approve it without unresolved findings, name a reviewer distinct from the implementer after whitespace/case normalization, and retain its report. |
| `ci` | `jobs_artifact`, `run_id`, `attempt`, `required_jobs`. Retain the GitHub jobs API response. Each required job must have a unique matching name, repository URL, run/attempt, source commit, completed status and successful conclusion. A local gate is not this evidence. |
| `deployment` | `code`, `result_artifact`. With `--require deployment`, join a structured readback to the selected code and assignment; mismatched candidate, installed and accepted revisions or digests fail. |
| `usage` | `summary_artifact`, `charges_artifact`; optional `repair_of` names a preceding usage record, and `requested_model`/`requested_effort` are caller-declared request metadata. All imported phases must name one native session. Charge IDs must belong to the phase's declared generations and timestamps must fall within its boundaries. |

For a retained Claude Code transcript, the selected main-session prompt must
have a matching `promptId`, user role and text content. Its assistant
`tool_use.id` and user `tool_result.tool_use_id` must match `native_call` within
that prompt. The result must not be an error; only string content or `text`
blocks can witness a code identity. Prompt text, tool arguments, assistant
answers and tool results from other prompts or sidechains cannot supply the
witness. Compaction summaries do not start a selected prompt.

Success requires an assistant `stop_reason: end_turn` followed by a native
`system` / `turn_duration` boundary for that prompt. EOF, a status-line sample,
a stop-hook summary, a tool result alone or a maximum-token stop is insufficient.
A later continuation of the same prompt must establish its own completed
sequence. Relevant records require the selected `sessionId` and timestamps;
conflicting UUID records fail, identical repeats coalesce. Records are ordered
by timestamp, as queued prompts can be appended out of order. The shared
transcript reader bounds the inspected tail; an evicted prompt remains
incomplete rather than being inferred from later output. A partial final line
also leaves the evidence incomplete.

For Claude, the view's `native` object names the selected harness/session/prompt
and tool call, available request/message/record IDs, and the tool result's end
byte in the retained artifact. It never prints prompt, argument or output
bodies. Native raw bytes, including usage and unknown fields, remain in the
artifact; this adapter does not derive billing or backend effort from them.
The existing test/tree, independent review and matching hosted-CI requirements
still apply. An adapter passing synthetic fixtures does not establish a real
Claude task's full chain or installed-version format coverage; retain and enroll
that task's actual artifacts before making those claims.

A deployment result has `version: 1`, `id` matching the deployment record,
`assignment`, `repository`, an RFC3339 `observed_at`, and these fields:

```json
{
  "candidate": {"commit": "FULL_GIT_COMMIT", "sha256": "BINARY_SHA256"},
  "installed": {"commit": "FULL_GIT_COMMIT", "sha256": "BINARY_SHA256"},
  "acceptance": {
    "build": {"commit": "FULL_GIT_COMMIT", "sha256": "BINARY_SHA256"},
    "status": "pass",
    "exit_code": 0
  },
  "report_artifact": "retained-host-result",
  "review_artifact": "retained-readback-review"
}
```

The example uses placeholders for full lowercase commit/digest values. The
candidate commit must equal the tested/reviewed/CI commit; installed and
accepted build identities must equal that candidate. An unknown acceptance or
missing exit/report/review is incomplete; a failed acceptance or mismatch
fails. The view includes the observation time, actual reported installed
identity and acceptance, including mismatches. Missing selected deployment
evidence never falls back to a CI-only pass.

Import a retained host result into this small schema explicitly, retaining its
original bytes and readback review as hashed artifacts. The verifier checks
the structured joins and artifact integrity; it does not interpret prose
reviews, authenticate host reports, inspect a live binary or prove the import
faithfully describes its source. Record manual-import provenance and review
that mapping. A matching historical readback proves the reported outcome at
that time, not the currently installed version. Existing CI-only manifests
remain unchanged; older readers reject the new record kind or required stage.

### Retained usage metadata

Usage imports existing local JSON exports; it makes no provider request. A
phase summary contains `session.id`, `boundaries` (`task_started` then
`task_complete`, each with `utc`) and `generation_ids`. Optional `hitch_id`
must match the manifest. Reported model/effort come from `context.model` and
`context.effort`, or `requested_native_model`/`requested_native_effort`.
They are retained summary claims, not backend attestation. Repair summaries
may carry `repairs`, `frontier_assistance` and `owner_code_edits`; first-result
summaries may carry `frontier_assistance_before_first_result`. Assistance is
shown as a classification without repeating its free text. A repair count
requires a preceding `repair_of` phase in the same session.

A charge export contains `native_session`, `phase`, `complete`, optional
`failure`, and `rows`. Each row supplies `requested_id`, `id`, `http_status`,
`created_at`, `model`, `provider_name` and decimal-string `total_cost_usd`.
Successful matching rows are summed exactly in USD; the export's precomputed
total is not trusted. Identical rows count once; conflicting same-ID rows,
generation IDs assigned to multiple phases, or duplicate phases under new
record IDs fail. Deduplication applies within this explicit manifest, not
across accounts or manifests. Billing-account identity remains unavailable.

`actual_usd` is populated only when the declared generation coverage passes.
Missing rows remain incomplete; `known_subtotal_usd` labels the available
subset, and no known charges means null, not zero. Invalid/conflicting metadata
suppresses totals. Attempts retain their own charges, missing IDs and assistance
classification; review-assistance cost remains unavailable. Native cumulative
token snapshots and explicit phase deltas are shown separately and never
added together. Estimates and subscription dollar conversions are not
implemented. These are provider-reported generation charges, not an invoice,
an authenticated provider assertion or a total for all work on the task.

Only the documented metadata fields are projected into the result. Unknown
fields, commands, prompts and free-text assistance are not printed or stored in
the projected records; parse failures use fixed diagnostics. Original artifact
bytes are still read locally for integrity and remain in their original private
scope. This is not sanitization of source files or a general secret detector.
Supply only an explicitly authorized local artifact directory. A usage pass
does not establish native turn identity or independently bind the caller's
assignment label, and does not imply through-CI or installed acceptance.

Structured test results have `version`, `id`, `run`, `input_tree`, `argv`, `cwd`,
`exit_code`, `outcome` and `output_artifact`. The result ID matches the test
record; successful outcome is `exited` with numeric exit zero. Obtain these
values from the actual subprocess wrapper. Include all relevant tracked and
untracked test inputs in the recorded Git tree. A missing exit status remains
unknown. Do not put credentials in argv or environment records.

Structured review results have `version`, `id`, `repository`, `commit`,
`reviewer`, `verdict`, `unresolved` and `report_artifact`. Approval is `approved`
with an empty unresolved-findings array. A review of another revision does not
approve the target. Retain the original report and its repairs.

This is verification of explicitly imported local evidence, not source
authentication. Hashes establish byte integrity; they cannot prove that a
runner, reviewer or CI service authored a file. Retain producer provenance and
fetch hosted CI through an authorized route. The verifier does not infer that
your required-check list is complete. The JSON view lists checked source
locators and missing/broken edges so that another reviewer can refute the join.
