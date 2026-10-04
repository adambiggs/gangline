# Reference

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
| `gang rename OLD NEW` | Change a registered name and window title. |
| `gang drop NAME` | Stop an agent and fail its pending messages. An unregistered `NAME` is refused. |
| `gang down [-y, --yes]` | Confirm on a terminal (`[y/N]`, default no), then drop the selected team's agents and delete its runtime state and history. Before dropping any agent, it appends who ran it to `downs.jsonl` in the state root. Use `--yes` without a terminal. |
| `gang attach` | Attach to the selected team's tmux session. |
| `gang teams` | List teams in the configured state root. |

`up` and `hitch` accept:

| Option | Meaning |
| --- | --- |
| `-c`, `--collar COLLAR` | Harness collar; defaults to `GANG_COLLAR`. |
| `-d`, `--dir DIR` | Working directory; defaults to the current directory. |
| `-m`, `--model MODEL` | Native model identifier; the native CLI judges it. A collar's `hitch_guard` can warn or refuse at usage thresholds. |
| `-e`, `--effort EFFORT` | Native reasoning effort; requires `--model`. Refused when `gang models` lists the model without this effort. |
| `-t`, `--task TASK` | Startup assignment. |
| `-r`, `--role ROLE` | Role brief; `up` defaults to `lead`. |
| `--stdin` | Read the assignment from stdin. |
| `--resume SESSION` | Resume a native conversation by its native session ID. |
| `--recover` | Recover an agent's retained startup input. |

For collars whose transcripts Gangline reads, a resume is refused when the
native transcript stored under that session ID names a different session, or
when the ID contains path or glob characters. A session Gangline cannot check
is left to the native CLI, and `hitch` says why on stderr.
If the native CLI exits before `hitch` returns, `hitch` fails
with the pane's last lines and, when tmux collected one, the exit status. It
records them as the agent's failure reason and closes the pane.
A pane that shows no recognized startup screen within the startup wait
leaves the agent registered with its startup queued; `hitch` exits with
status 4, naming the pane and `gang hitch NAME --recover`, and the boot
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
input for a watchdog period, across at least two ticks, since messages to it
wait behind that input. The period is `watchdogTimeout` in
[`watchdog.go`](../cmd/gang/watchdog.go).

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
| `gang send NAME [OPTIONS] [BODY]` | Send BODY, or read stdin when absent, and report its receipt. |
| `gang queue [NAME] [--json]` | List pending messages with ID, recipient, sender, kind, a text excerpt, and state: `ready`, `scheduled` with its due time, `blocked` with the reason, or `unknown` with the reason. |
| `gang interrupt [NAME] [-m\|--message REASON]` | Interrupt the turn; deliver an optional reason after it stops. |
| `gang compact [NAME] [--resume TEXT]` | Compact at native idle; submit the continuation behind compaction, ahead of later input. Refuses, asking for a retry, while another gang operation holds the agent. Refuses while startup input is unverified, or while the previous compaction's resume note waits in the harness, until it is admitted, until one operation's timeout after that compaction completed, or, for a compaction past its deadline, until the pane stops showing it; a queued compaction waits until startup input is verified (`gang hitch NAME --recover`). |
| `gang compact NAME --recover` | Interrupt a submitted or unconfirmed compaction with the collar's recovery keys while the pane shows it running. Refuses without sending on an approval, trust, draft, idle, or unrecognized screen; when the resume note was never queued or the harness has taken it; and after an earlier recovery of the same compaction. Records the compaction as unconfirmed and reports the screen it left. |
| `gang curfew [DURATION\|HH:MM\|RFC3339\|clear]` | Show, set, or clear the team deadline. |
| `gang tick [--agent NAME\|HITCH_ID]` | Check deadlines, recover native failures, and drain due messages, for every agent or the one `--agent` names. |
| `gang wait NAME [--timeout DURATION]` | Wait up to DURATION (default 30s) for a recorded idle boundary; a zero timeout checks once. |

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
delivering it out of order. Recovery also cancels a continuation that was
published but never submitted to native input.

While the collar's `actions.compact.active` pattern matches the pane, the agent
reads as compacting: sends wait in Gangline's queue, a queued compaction waits,
and `gang compact NAME --recover` treats the pane as running. A harness that
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
| `--from SENDER` | Required outside registered panes; refused inside them. |
| `--live-only` | Refuse instead of queuing if the recipient cannot take input now. |
| `--supersede` | Clear this sender's older scheduled messages for this recipient first. |
| `--at DURATION\|HH:MM\|RFC3339` | Schedule delivery after a duration, at a local time, or at an exact deadline. |
| `--clear` | Clear this sender's scheduled messages for the recipient instead of sending; takes no body, `--at`, or `--live-only`. |

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
placeholder in words. A `snooze --note` that contains one is refused whatever
the snoozing agent's harness, since an overdue wake whose caller is gone goes
to the lead. A queued message that still carries one fails before it is typed.
`--source watchdog --watchdog UNIT` is the internal watchdog invocation.

## Inspect

| Command | Effect |
| --- | --- |
| `gang roster [--json]` | Show registered agents and current states, with the recorded reason for a failed agent or a blocked, wedged, or unknown one. The table cuts a long reason to its head and its end so that the row's characters fit the terminal's width; `--json` and `gang status --why` carry the whole text. An idle agent whose last turn ended with native background tasks pending reads `idle (N bg)`, and `background_tasks` in `--json`; it is still idle for delivery, and the next turn boundary clears the count. |
| `gang status [NAME] [--why] [--json]` | Show state; `--why` includes activity and compaction evidence. Pending background tasks qualify idle as in `gang roster`. |
| `gang capture [NAME] [-n\|--lines LINES]` | Print the native pane, or only its last LINES lines. |
| `gang capture --composer [NAME]` | Print draft input from the composer. |
| `gang context [NAME] [--json]` | Show native context usage or an unknown reading. |
| `gang context --widget NAME` / `gang context --clear` | Select a context widget for the tmux status line, or clear it. |
| `gang limits [NAME]` | Show observed provider limits. |
| `gang limits -c\|--collar COLLAR` | Query native account limits without a live agent, if supported. |
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

Window titles use `?name?` for changing or unknown state, `~name~` for idle,
`-name-` for work, and `!name!` for blocked, wedged, or failed agents.
Use a hitch ID in log filters to follow a registration across renames.

Each `activity_observed` event records what its reading was derived from.
`basis.screen` is the collar's reading of the pane: `blocked`, `compacting`,
`idle`, `busy`, `unsubmitted`, `unreadable`, or `unread` when the capture
failed. `basis.rule` is the rule that set the activity: `screen`, `open-turn`,
`interrupt-pending`, `compaction-record`, `turn-failure`, `wedge`,
`probe-failure`, or `permission-request`: a native permission request whose
hook arrived reads blocked on an unreadable screen until any later hook or an
idle screen. A screen that reads idle or busy wins, since dismissing the
prompt fires no hook and answering it fires one only when the tool finishes.
`basis.compaction` is the compaction record's status, and
`fingerprint` is a hash of the screen. Neither carries screen text, and
neither does the reason: a blocked reading names the native prompt by a hash
of the text the collar matched, so a prompt whose matched text differs logs a
new reading, and a wedged
reading names the collar's threshold. `gang capture NAME` prints the prompt.

## Discover and maintain

| Command | Effect |
| --- | --- |
| `gang collars` | List bundled and custom collars. |
| `gang collar check NAME` | Probe an installed harness in a private throwaway tmux session. |
| `gang models [-c COLLAR]` | List native models and efforts; also accepts `--collar`. |
| `gang roles` | List role briefs. |
| `gang config` | Print effective settings and their sources. |
| `gang statusline [--install]` | Render native status-line JSON; `--install` fills an absent `statusLine` in `settings.json` under `CLAUDE_CONFIG_DIR` (default `~/.claude`) and names that file. |
| `gang --version` or `gang version` | Print the version. |
| `gang upgrade [-y, --yes]` | Print the installed and latest stable versions, confirm on a terminal (`[y/N]`, default no), then install into the installer-managed checkout. Use `--yes` without a terminal. A current install exits 0 without asking. |
| `gang help [COMMAND]` | Show command help. |
| `gang hook` | Read native hook JSON from stdin; invoked by the collar integration. |

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

Set `GANG_CODEX_PERMISSION_PROFILE=gangline` in the config file or environment
to select that profile for Codex hitches. The named profile must exist in
Codex's own config. An unset value leaves profile selection to Codex's own
config. Conflicting sandbox or profile arguments supplied through Gangline are
refused. Codex may load other config layers that override the selected
permission mode.

When a Codex hitch directory is a linked Git worktree, Gangline passes its
exact worktree gitdir to Codex with `--add-dir`, whatever the profile setting.
Codex marks that gitdir read-only with an exact-path rule that a pattern grant
in a profile does not override, so without it the agent cannot stage, fetch
or commit in its own worktree. This lets the agent edit that worktree's Git
metadata, including files that can make later host Git commands execute code.
Staging, fetching and committing also write objects and refs in the common Git
directory, which Gangline does not grant; Codex's own config decides that
access, and it carries the same trust cost.

Codex refuses to start when given `--add-dir` under its read-only sandbox.
Gangline omits the grant when its launch arguments select that sandbox, but
cannot see one selected in Codex's own config: such a hitch into a linked
worktree fails at launch with Codex's error.

These variables are environment-only:

| Variable | Default and purpose |
| --- | --- |
| `GANG_CONFIG_DIR` | `${XDG_CONFIG_HOME:-~/.config}/gangline`; absolute config directory. |
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
windows do not inherit that per-hitch capability.

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
`options`, and hook `install_args`) takes effect when an agent is hitched. Gangline reads the collar again for each later operation, so edits to
its other fields apply to agents already running.

A harness can stream a reply with no busy marker on screen. When the
`hook-boundary` turn boundary sets `open_turn_quiet`, a turn whose submit was
witnessed reads busy on an idle-looking screen until its finish or failure
hook arrives, an interrupt brings the composer back, or the screen stays
unchanged for that long. A turn that ends without its hook, as after a
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
record. A witness from any other session is never followed: the agent fails
once, and its hitcher is told both session ids and to drop the agent and
re-hitch it with `--resume` for the session to continue, in the agent's own
directory, collar, and role. Drop tells the sender of each message still queued
for the agent. A message submitted into such a session stays unverified.

Each context band has a `name`, threshold `at` (a fraction from 0 to 1), and
optional `message`. A nonfinal band without a message advises saving state and
compacting at the next good stopping point; the last orders compaction now.
Messages use these tokens; token counts are integers and percents are rounded:

Usage bands use the same named fraction thresholds under `usage_bands.five_hour`
and `usage_bands.weekly`. They read native provider usage for the collar's
account and notify the active lead once per band and reset window. A collar
overlay can replace either window independently:

```cue
collar: {
	usage_bands: {
		five_hour: [{name: "early", at: 0.80}, {name: "full", at: 0.95}]
	}
}
```

An optional per-band `note` appends operator guidance to the measurement;
its default is empty. An optional `message` replaces the measurement template.
Both accept `{{band}}`, `{{collar}}`,
`{{window}}`, `{{threshold_percent}}`, `{{used_percent}}`,
`{{reset_at}}` (UTC RFC3339), and `{{snooze_command}}`. With no message,
the notice reports only provider, usage percent, and reset time. Unknown native
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
from its native session log. `gang limits -c codex` queries account limits
through a private native app server without creating a conversation or turn.
The Claude Code collar has no standalone limits query; use `gang limits NAME`
for readings observed from an agent.

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
timestamp. `--note` is delivered with the wake. A pending wake is durable;
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
