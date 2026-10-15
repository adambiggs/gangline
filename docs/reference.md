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
agents in the selected team. In a hitched pane, `--team` may name only the
team `GANG_SESSION` selects; any other team is refused with exit status 3,
and setting `GANG_SESSION` is the deliberate way to act on another team
there. `log` refuses `--team` together with a `LOG.jsonl` operand. `teams`, `collars`,
`collar`, `models`, `roles`, `config`, `statusline`, `upgrade`, `help`,
`version`, and `hook` do not act on one team and do not accept `--team`.

## Start and stop

| Command | Effect |
| --- | --- |
| `gang up [NAME] [AGENT OPTIONS]` | Start the selected team with its lead agent named `NAME` (default `lead`), then attach when stdin is a terminal. |
| `gang hitch NAME [OPTIONS]` | Launch an agent and deliver startup instructions and any task. |
| `gang hitch NAME --recover` | Recover retained startup from its visible draft or a lone collapsed paste, or resume a queued startup its boot deadline failed once the pane shows an idle composer or a recognized prompt. |
| `gang rename OLD NEW` | Change a registered name and its pane's display title. |
| `gang drop NAME` | Stop an agent and fail its pending messages. An unregistered `NAME` is refused. |
| `gang down [-y, --yes]` | Confirm on a terminal (`[y/N]`, default no), then drop the selected team's agents and delete its runtime state and history. Before dropping any agent, it appends who ran it to `downs.jsonl` in the state root. Use `--yes` without a terminal. |
| `gang attach [NAME]` | Attach to the named agent's pane in the selected team. Without `NAME`, select the lead, or the first registered agent if there is no lead. |
| `gang teams` | List teams in the configured state root. |

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
| `-r`, `--role ROLE` | Role brief; `up` defaults to `lead`. |
| `--stdin` | Read the assignment from stdin. |
| `--split NAME` | Place the new agent beside the named agent in its window; the target must already have a registered pane in the selected team. Without this option, create a new window. |
| `--vertical` | Split top/bottom instead of side by side; requires `--split NAME`. |
| `--resume SESSION` | Resume a native conversation by its native session ID. |
| `--recover` | Recover an agent's retained startup input. |

For example, `gang up --context-bands 10,20` gives the lead earlier notices,
while `gang hitch worker --context-bands 20,30` sets the worker's thresholds
independently.

A resume is refused when the session ID starts with `-`, since the native CLI
would read it as an option. For collars whose transcripts Gangline reads, a
resume is also refused when the
native transcript stored under that session ID names a different session, or
when the ID contains path or glob characters. A session Gangline cannot check
is left to the native CLI, and `hitch` says why on stderr.
If the native CLI exits before `hitch` returns, `hitch` fails
with the pane's last lines and, when tmux collected one, the exit status. It
records them as the agent's failure reason and closes the pane.
A pane that shows no recognized startup screen within the startup wait
leaves the agent registered with its startup queued; `hitch` exits with
status 4, naming the agent and `gang hitch NAME --recover`, and the boot
deadline still fails the agent if startup is not resumed.
A startup blocked on a native prompt keeps holding its pane after `hitch`
returns, so an answer that ends the native CLI fails the agent at the next
tick or roster with its last lines and exit status, and the pane stays until
`drop`.
`drop` reports an observed native resume session, or says it is unknown.

A command run from an agent's pane, including any script its native CLI
starts, carries that agent's identity and team. Within that team, `down` and
a change to `curfew` are refused unless the caller is the lead, and `drop
NAME` is refused unless the caller is the lead or the agent that hitched
`NAME`; an agent with no recorded hitcher is the lead's to drop. The lead is
the agent started in the `lead` role by `gang up` or the operator; neither its
name nor a `--role lead` given by another agent confers that authority. The operator
outside any agent pane is not restricted. A hitch run from an agent's pane
records that agent as the hitcher. When a hitched agent later fails, its
hitcher, if active, gets a `[gang:hitch#…]` notice naming the agent and the
reason. When `gang hitch NAME --recover` resumes the startup of an agent whose
boot deadline failed it, the hitcher gets a notice that the agent recovered,
unless the hitcher ran the recovery itself. The hitcher gets one too when a
message to the agent is withdrawn from its composer and not delivered, when a message paste may remain in the
agent's composer, and when the agent has read as holding unsubmitted composer
input past the notice threshold, since messages to it wait behind that input.
The threshold is `heldInputNoticeAfter` in
[`authority.go`](../cmd/gang/authority.go). The hitcher also gets one notice,
and the agent's log a `background_idle` event, when the agent has stayed idle
for `GANG_BACKGROUND_IDLE_NOTICE` since a turn that ended with native
background tasks pending. Each such turn is announced once.

Each line of `downs.jsonl` records the time, team, agent count, the caller's
agent name and inherited `GANG_AGENT_ID`, its process and parent process IDs,
the parent's command line where `/proc` shows it, the working directory, and
`TMUX_PANE`.
When Codex asks to trust a hitch directory, startup stays queued. After
choosing the native trust option, run `gang tick` to deliver the retained
contract and assignment.

## Message and control

| Command | Effect |
| --- | --- |
| `gang sender add NAME` | From the operator shell or active lead pane, register NAME and print the path of a random key file under the team's state directory, readable only by the user. Refuses agent, outside-mailbox, registered-sender and enrolled peer-host names. |
| `gang sender list` | List registered sender names. |
| `gang sender remove NAME` | From the operator shell or active lead pane, revoke NAME and delete its key file and registration; queued messages retain their authenticated attribution. |
| `gang send NAME [OPTIONS] [BODY]` | Send BODY, or read stdin when absent, and report its receipt. |
| `gang send HOST:NAME [BODY]` | From an active registered agent, or with `--as NAME` and its registered sender credential, forward BODY over ssh to agent NAME on `GANG_PEERS` host HOST, where the forced command of this host's key runs `gang peer-receive` to deliver it, and print `HOST:ID` with that host's receipt. A refusal from that host names it as the host where any command in its reason runs. Once ssh has run, the sending host's team log records a `peer_sent` event under the sender with `HOST:NAME`, the message ID, and the outcome, `refused`, or `unknown`. No reply within two minutes, or a last line that is not a reply to this message, prints `unknown` and exits 5: the message may have been published there, so send it again only once no `gang peer-receive` runs on that host and its `gang queue` and `gang log` both lack the ID. |
| `gang queue [NAME] [--json]` | List pending messages with ID, recipient, sender, kind, a text excerpt, and state: `ready`, `scheduled` with its due time, `blocked` with the reason, or `unknown` with the reason. |
| `gang interrupt [NAME] [-m\|--message REASON]` | Interrupt the turn; deliver an optional reason after it stops. Refuses without sending keys while the pane shows a native approval or trust prompt. |
| `gang compact [NAME] [--resume TEXT]` | Compact at native idle; submit the continuation behind compaction, ahead of later input. Refuses, asking for a retry, while another gang operation holds the agent. Refuses while startup input is unverified, or while the previous compaction's resume note waits in the harness, until it is admitted, until one operation's timeout after that compaction completed, or, for a compaction past its deadline, until the pane stops showing it; a queued compaction waits until startup input is verified (`gang hitch NAME --recover`). |
| `gang compact NAME --interrupt` | Interrupt a submitted or unconfirmed compaction with the collar's recovery keys while the pane shows it running. A failed deferred compaction can instead clear its exact staged command from an idle composer and retry queued delivery; edited input remains untouched. Busy recovery refuses without sending on an approval, trust, draft, idle, or unrecognized screen; when the resume note was never queued or the harness has taken it; and after an earlier recovery of the same compaction. It records an interrupted compaction as unconfirmed and reports the screen it left. |
| `gang curfew [DURATION\|HH:MM\|RFC3339\|clear]` | Show, set, or clear the team deadline. |
| `gang tick [--agent NAME\|HITCH_ID]` | Check deadlines, recover native failures, and drain due messages, for every agent or the one `--agent` names. |
| `gang wait NAME [--timeout DURATION]` | Wait up to DURATION (default 30s) for a recorded idle boundary; a zero timeout checks once. |

At or after a curfew deadline, a tick notifies each active agent once for
that deadline, including agents started later. The notice names the team,
deadline and observation time. The deadline remains set until changed or
cleared. Agents remain active; `up`, `hitch`, and `attach` remain available.

If native work starts after the idle check but before compaction submission,
`actions.compact_defer_clear` withdraws the exact staged command and keeps its
resume note queued for the next idle boundary. Gangline leaves changed input
untouched and reports a withdrawal it cannot confirm as a failure.

The resume note enters native input when compaction starts and runs when the
harness takes it. Gangline admits that exact note once; until a completion hook
confirms that compaction finished, the compaction stays unconfirmed.
Codex may merge later Enter steers into the same prompt, with the resume note
first; Tab queues separate follow-up turns. If the note cannot enter ahead of
an occupied composer, the command fails and withholds the note rather than
delivering it out of order. After confirmed completion, an unchanged continuation
that was published but never submitted stays queued for the next free composer,
ahead of later messages. An altered or unknown continuation is withheld.

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

A recorded native turn failure leaves the agent's activity unknown, but a
queued compaction still starts once the screen reads idle with no turn open.
A provider capacity failure (`rate_limit`, `overloaded`, `server_error`,
`billing_error`) would fail the compaction the same way, so it keeps waiting
until a later turn finishes: one `compaction_waiting` event names the class,
`gang compact` and `gang status NAME --why` show it as the compaction's reason, and
an empty `compaction_waiting` marks the end of the wait.

For a harness without a native queue witness, sends also wait after the
compaction ends until the harness admits the queued resume note, whose submit
hook must run first. If that hook blocks the note, the compaction fails, the
agent is told its note was withheld, and held messages go out at once. If no
admission arrives within one operation's timeout, the hold ends and held
messages go out at the agent's next tick.

Gangline never presses Enter on a composer that holds anything other than the
compact command, nor on one the command has already left. Claude Code shows a long or multi-line paste as a placeholder
and would submit it as an ordinary prompt, so such a resume note fails there:
Gangline clears the composer with the collar's `compact_clear` keys and submits
nothing. Keep the note short, on one line, and pointed at a state file. Without
`--resume`, the note tells the agent to re-read its brief and durable state and
names `gang log --agent NAME --type send_queued`, which records the messages
queued for it. Every
failed compaction queues a `[gang:compact#…]` notice to the agent saying whether
its context was compacted, and a copy to the agent that requested it when the
failure comes after its command returned.

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
Oversized rendered messages are refused without splitting; put details in a
file and send its path.
For a Claude Code recipient, a message, hitch task, `compact --resume` note,
or `interrupt -m` reason that contains one of Claude Code's
numbered placeholders in brackets, for pasted text, an image, or truncated
text, is refused: Claude Code can replace that token with the content of an
earlier paste, so the prompt it submits is no longer the message. Describe the
placeholder in words. Text for a Claude Code recipient is also refused when it
holds an invisible format character, such as a zero-width space or byte order
mark, or a Unicode line or paragraph separator: Claude Code drops or rewrites
them and leaves that paste unsubmitted. The refusal names the code points to
remove. Tabs are delivered; Claude Code shows each as four spaces. A
`snooze --note` that contains either is refused whatever
the snoozing agent's harness, since an overdue wake whose caller is gone goes
to the lead. A queued message that still carries either fails before it is typed.
`--source watchdog --watchdog UNIT` is the internal watchdog invocation.

## Inspect

| Command | Effect |
| --- | --- |
| `gang roster [--json]` | Show registered agents and current states, with the recorded reason for a failed agent or a blocked, wedged, or unknown one. The table cuts a long reason to its head and its end so that the row's characters fit the terminal's width; `--json` and `gang status --why` carry the whole text. An idle agent whose last turn ended with native background tasks pending reads `idle (N bg DURATION)`, where DURATION is the time since that turn ended, and `background_tasks` and `background_since` in `--json`; it is still idle for delivery, and the next turn boundary clears the count. |
| `gang status [NAME] [--why] [--json]` | Show state; `--why` includes activity and compaction evidence. Pending background tasks qualify idle as in `gang roster`, and `--why` names when the agent went idle with them. |
| `gang capture [NAME] [-n\|--lines LINES]` | Print the native pane, or only its last LINES lines. |
| `gang capture --composer [NAME]` | Print draft input from the composer. |
| `gang context [NAME] [--json]` | Show native context usage or an unknown reading. |
| `gang context --widget NAME` / `gang context --clear` | Select a context widget for the tmux status line, or clear it. |
| `gang limits` | Query every configured collar's account limits and pace without a live agent. |
| `gang limits NAME` | Show the provider limits an agent observed, with pace. |
| `gang limits -c\|--collar COLLAR` | Query one collar's account limits without a live agent, if supported. |
| `gang snooze [--at TIME] [--note TEXT]` | Schedule a wake for the calling agent at its observed native reset, or at an explicit future time. |
| `gang snooze --status` / `gang snooze --clear [ID]` | Inspect or cancel a pending wake. The lead sees every agent's wake and uncertain notices, and can clear a notice or fallback wake by ID. |
| `gang log [--agent NAME\|HITCH_ID] [--type TYPE\|KIND] [LOG.jsonl]` | Print JSONL events from a team or saved log, with optional filters. |
| `gang whoami` | Print the calling pane's registered identity. |

`--json` on `queue`, `roster`, `status`, and `context` prints one JSON object
for scripts. It carries exact values the human forms abbreviate: queue
timestamps and context token counts. Availability conditions such as
`process_available` and `watchdog_available` are fields. An unobserved context
reading prints `status` `unknown` with its `reason` and null counts, and the
command still exits unknown. The human forms may change layout.

Pane borders use `?name?` for changing or unknown state, `~name~` for idle,
`-name-` for work, and `!name!` for native decisions, wedges, or failures.
An unsubmitted operator draft holds message delivery but keeps the idle
symbol, or the work symbol while a witnessed turn is still open.
Native submit, activity, permission, turn-end, failure, and compaction hooks
persist pending status records and refresh the affected agent through a
detached tick. Fresh records drive the symbol without a pane capture,
including after a skipped detached refresh.
See `statusHookFreshness` in `cmd/gang/status_hooks.go` for the freshness and
stale-probe interval. After expiry, a pane observation supplies the symbol
until a newer hook or another probe is due. Capture reasons appear in the log.
Hooks do not report every permission dismissal, native prompt, process exit,
capacity error, or wedge; those remain bounded fallback observations. Startup,
input recovery, compaction, interrupts, and wake outcomes can need additional
validation sooner. Delivery still checks the composer and native receipts. The watchdog supplies a
five-second whole-team fallback on an awake host with a working user scheduler.
Scheduler delays, occupied agent locks, and slow captures can delay a reading;
an unreadable capture remains unknown. `gang status` and `gang roster` also
observe and refresh the symbols.
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
with `native turn refused: CLASS: MESSAGE` instead: the agent is alive and
takes the next message, so rebrief it, and replace it only if the rebrief is
refused too. Codex reports a failed turn only in its session log, which ticks,
`gang status`, and `gang roster` read. The next submitted prompt clears the
failure.

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
| `gang peer-authorize HOST PUBKEY_FILE` | Print the `~/.ssh/authorized_keys` line that binds enrolled `GANG_PEERS` host HOST's public key to `gang peer-receive` for the selected team: `command="GANG peer-receive --team TEAM --peer HOST",restrict TYPE KEY gangline-peer-HOST`, where GANG is this `gang`'s path. PUBKEY_FILE holds exactly one OpenSSH public key line with no options; its comment is dropped. Stderr names HOST's steps: keep the key pair for this link only, add an ssh alias for this host with `IdentityFile`, `IdentitiesOnly yes`, and `ControlPath none`, set that alias as `ssh` in its `GANG_PEERS` entry for this host, and set `GANG_HOST=HOST`. sshd uses the first line whose key matches, so a line that already authorizes the key without the forced command leaves it unbound. A dedicated restricted key supplies the binding; unrestricted SSH access to the receiving account can invoke `peer-receive --peer` directly. Refuses an unenrolled host, a team name or `gang` path that cannot appear in the command, and any other key file. |
| `gang peer-receive --peer HOST` | The forced command `gang peer-authorize` prints; its dedicated restricted-key setup binds HOST to an enrolled host. Before reading stdin, it refuses unless its parent is an `sshd` or `sshd-session` process of this user whose parent is an `sshd` or `sshd-session` process of root, and refuses without `--peer` or with an unenrolled HOST. This ancestry screen does not authenticate the key or distinguish host-root from namespace-local root UIDs; receiving-account shell and state access are trusted. Read one JSON request (`host`, `team`, `id`, `from`, `to`, `body`, and optional `class`) from stdin and deliver it from sender `HOST:NAME` or `HOST:registered:NAME` as message `peer-HOST.ID`; a request whose `host` is not HOST is refused. Print one JSON line with the message ID and outcome, or the error that published nothing. Refuses an ID whose envelope or retained receipt this host still holds, so a concurrent duplicate never replaces a queued message; delivery of a repeated ID is still at-least-once, because one that arrives after the first copy is delivered, or after a later message's receipt replaces its receipt, is delivered again. Refused inside a registered agent pane. [Peer labels](concepts.md#messages-and-envelopes) states what the binding leaves open. |

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
| `GANG_CAPACITY_TIMEOUT` | Positive duration budget for provider-error continuations. |
| `GANG_BACKGROUND_IDLE_NOTICE` | Positive duration an agent stays idle with native background tasks pending before its hitcher is told; defaults to `30m`. |
| `GANG_PEERS` | JSON object of enrolled peer host names to `{"ssh": ALIAS, "user": LOGIN, "gang": PATH, "team": TEAM}`: the ssh alias and optional login that reach the host, the absolute path of its `gang`, and its team. An enrolled host's messages are accepted through the forced command `gang peer-authorize` prints for its key. `gang send HOST:NAME` runs `ssh ALIAS PATH peer-receive --team TEAM`; HOST runs its forced command for the key in its place. No two host names may differ only by case. Unset means no peers. |
| `GANG_HOST` | This host's name as its peers enroll it in their `GANG_PEERS` and authorize its key; required to send to a peer, which refuses a request naming any other host. |

Set `GANG_CODEX_PERMISSION_PROFILE=gangline` in the config file or environment
to select that profile for Codex hitches. The named profile must exist in
Codex's own config. An unset value leaves profile selection to Codex's own
config. Conflicting sandbox or profile arguments supplied through Gangline are
refused. Codex may load other config layers that override the selected
permission mode.

Gangline does not add Git write roots for linked worktrees or primary
checkouts. Codex protects `.git` and a linked worktree's resolved Git directory
by default; metadata writes use Codex's native approval path. Operator-supplied
arguments, including explicit `--add-dir` grants, are passed through unchanged.
Gangline does not select an approval policy or reviewer. For unattended Git,
configure `approval_policy = "on-request"` and
`approvals_reviewer = "auto_review"` in Codex's own config; see
[sandboxed development teams](operations.md#sandboxed-development-teams).
With `approval_policy = "never"`, protected Git writes cannot escalate.

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
checks, and `roster` shows `[process-unavailable]` until the agent's own command
passes ancestry verification from the PID namespace recorded at hitch,
which logs `process_verification_available`. The process watchdog is
also skipped visibly with `watchdog_unavailable` and `[watchdog-unavailable]`,
and `hitch` and `snooze` warn on stderr; ordinary hooks and commands still tick
the team. The next armed timer clears the marker and logs `watchdog_available`. A pane-only drop cannot
prove that detached descendants exited. Every registered pane requires a server
generation, session, pane ID, and inherited capability. Missing registration or
capability is refused; launch a fresh hitch to establish them. Later ordinary
panes do not inherit that per-hitch capability.

`--team TEAM` selects another team in the same state root and tmux server for
one command. For a separate team, keep its selection in the shell environment
for every command. A separate state root and socket also isolate its files and
server:

```sh
export GANG_SESSION=review
export GANG_STATE_ROOT="$HOME/.local/state/gangline-review"
export GANG_TMUX_SOCKET="$GANG_STATE_ROOT/tmux.sock"
gang up
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
record. A submit hook from another session that names a spawned agent
(`agent_id`), or that leaves a transcript the collar maps missing, null or
empty, comes from a session the harness runs beside the conversation, not one
that replaces it. Gang keeps the agent's registration and witness, logs a
`native_hook` event with status `foreign-session`, and keeps the raw payload in
`STATE_ROOT/teams/TEAM/foreign-submits/HITCH_ID.json`, which `gang status NAME
--why` names. A witness from any other session is never followed: the agent
fails once, and its hitcher is told both session ids and to drop the agent and
re-hitch it with `--resume` for the session to continue, in the agent's own
directory, collar, and role. Drop tells the sender of each message still queued
for the agent. A message submitted into such a session stays unverified.

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
| `{{compact_command}}` | `gang compact --resume 'Resume from FILE'`. |

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
past its threshold that has not reset makes
`hitch` warn on stderr with the window, used percent, the reading's age and
source, and the reset time (UTC RFC3339), then launch. With `refuse: true`,
`hitch` refuses instead. When no fresh reading exists or the query fails,
`hitch` says so and launches; a stale or missing reading never refuses. The
five-minute window keeps an old reading from refusing a launch the account
would accept, at the cost of an unchecked launch when no agent reported
recently and the collar has no query. Bundled collars set no guard.

`gang snooze` is a manual override for an active agent. On an attributable
provider cap refusal, Gangline automatically schedules a wake at the observed
native reset. With no `--at`, `gang snooze` uses a recent
native five-hour or weekly reset from that agent's collar, selecting the
most-used window. `--at` accepts a duration, local `HH:MM`, or RFC3339
timestamp. `--note` is delivered with the wake. A manual wake notice names
its ID, original caller and due time. A pending wake is durable;
repeating the command replaces it until it is submitted. The due wake is sent
to the caller, or the active lead if the caller is gone. `--status` shows
scheduled, queued, submitted, or failed wakes. For the lead it also lists
every other agent's wake with its due time and note, and uncertain usage
notices; `--clear ID` removes a notice or a wake routed to the lead. An
agent can clear its own wake with `--clear`. A wake still queued
for native input is withdrawn from the recipient's inbox; clearing cannot
retract input the harness already accepted.

The team log records each wake stage under the agent that scheduled it, so
`gang log --agent NAME` shows the life of NAME's wakes: `snooze_scheduled`
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

Linux uses systemd user timers for the watchdog; macOS uses transient launchd
user agents. On a host without a supported user scheduler, ordinary commands
and ticks remain available. See the [watchdog guide](guides.md#keep-an-unattended-team-moving)
and [internals](internals.md#watchdog).

## Exit status

| Status | Meaning |
| --- | --- |
| `0` | Successful command; a send may still be queued, or may have failed when its paste was withdrawn. |
| `1` | Execution or I/O error. |
| `2` | Bad usage; the message names the problem and, for a bad value, the accepted form. A known command's usage follows. |
| `3` | Refused operation. |
| `4` | Native harness needs attention. |
| `5` | Unknown result, such as input without a confirmed receipt. |
