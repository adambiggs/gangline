# CLI and collar reference

`gang help` is the executable command inventory. `gang COMMAND --help` prints
the accepted shape for one command.

## Requirements and installation

Gangline supports macOS and Linux and requires Git, Go 1.27 or later, tmux 3.2
or later, and Claude Code or Codex. The installer selects the newest stable
`gangline-v*` tag, retains that checkout for inspectable upgrades, and builds a
static binary with `CGO_ENABLED=0`.

```sh
curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
```

`GANGLINE_HOME` changes the retained checkout, `GANGLINE_BIN` changes the binary
directory, and `GANGLINE_REPO` changes the release source. Defaults are
`~/.local/share/gangline`, `~/.local/bin`, and the public Git repository.

## Commands

### Team lifecycle

| Command | Effect |
| --- | --- |
| `gang up [NAME] [HITCH OPTIONS]` | Hitch the first agent, named `lead` by default, and attach when stdin is a terminal. |
| `gang hitch NAME [OPTIONS]` | Launch a harness window and deliver its contract, role, and assignment. |
| `gang adopt NAME -c COLLAR` | Register the current tmux pane without launch or startup delivery. |
| `gang rename OLD NEW` | Rename an active hitch and its tmux window. |
| `gang drop NAME` | Stop one active or failed hitch and cancel its pending work. |
| `gang down SESSION` | Drop every active or failed hitch and remove that team's v1 state directory. |

Hitch options are `-c/--collar`, `-d/--dir`, `-m/--model`, `-e/--effort`,
`-t/--task`, `-r/--role`, `--resume`, and `--stdin`. Effort
requires an explicit model. `--stdin` reads the assignment body from stdin;
otherwise `--task` supplies it.

`up` defaults the role to `lead` and the working directory to the caller's
current directory. An explicit `--role` or `--dir` overrides that default.
A safely deferred startup assignment is retried by the hitch command until it
is delivered or its startup-delivery deadline expires.
A failed hitch retains its name until `gang drop NAME` removes that generation;
another hitch or rename cannot reuse it first.

### Delivery and control

| Command | Effect |
| --- | --- |
| `gang send NAME [--from NAME] [--live-only] [--supersede] [--at TIME]` | Read a body from stdin and deliver or queue it. |
| `gang send NAME --at clear` | Clear timed deliveries for the recipient. |
| `gang queue [NAME]` | List queued delivery ID, recipient, and sender rows. |
| `gang interrupt [NAME] [-m REASON]` | Interrupt an active or wedged native turn. |
| `gang compact [NAME] [--resume TEXT]` | Submit the collar's compaction action and queue a continuation. |
| `gang compact NAME --recover` | Apply the collar's declared compaction-recovery actions. |

Inside an active Gangline pane, `send` derives the sender from the pane and
refuses an overriding `--from`. Outside the team, `--from` is required and the
wire envelope marks it `self-declared:`.

`--live-only` refuses unless the recipient is recorded idle. `--supersede`
clears older timed work for that recipient before sending. `--at` accepts a
duration such as `45m` or a local `HH:MM` time.

### Observation and recovery

| Command | Effect |
| --- | --- |
| `gang roster [--porcelain]` | List active hitches and recorded activity. |
| `gang status [NAME] [--why]` | Show one hitch and any recorded wedge evidence. |
| `gang capture [NAME] [LINES]` | Render a parsed pane screen. |
| `gang capture --composer [NAME]` | Read the collar-recognized composer. |
| `gang context [NAME]` | Read native context use and the active collar band. |
| `gang limits [NAME]` | Read provider-limit rows visible in the native TUI. |
| `gang log` | Print the configured team's authoritative JSONL log. |
| `gang replay [EVENTS.jsonl]` | Fold a log from a file or stdin and print state as JSON. |
| `gang tick` | Retry pending effects, release safe queued delivery, and observe wedges. |
| `gang wait NAME [--timeout DURATION]` | Block on event-log appends until the agent is idle; defaults to 30 seconds and `--timeout 0` checks once. |
| `gang whoami` | Print the active hitch bound to the current pane. |
| `gang teams` | List team directories with event logs in the v1 state root. |
| `gang attach` | Attach to the configured tmux team. |

Gangline projects the recorded hitch state into each managed tmux window name:
`?name?` is starting, booting, or dropping; `~name~` is idle; `-name-` is
working; and `!name!` is blocked, wedged, or failed. `gang tick` repairs a
window name that drifted from the event log.

### Discovery and installation

| Command | Effect |
| --- | --- |
| `gang collars` | List embedded and operator CUE collars. |
| `gang collar check NAME` | Probe an installed harness on disposable private tmux sockets. |
| `gang models [-c COLLAR]` | Ask the collar's native catalog for models and efforts. |
| `gang roles` | List shipped and operator role briefs. |
| `gang config` | Print effective settings and their source. |
| `gang curfew [DURATION\|HH:MM\|clear]` | Show, set, or clear the recorded team deadline. |
| `gang --version` | Print the built release version. |
| `gang upgrade [--check]` | Run the retained release installer's upgrade path. |

## Exit status

| Status | Meaning |
| --- | --- |
| `0` | The documented operation completed; a send may be durably queued. |
| `1` | An execution or I/O error occurred. |
| `2` | Command usage was invalid. |
| `3` | Gangline refused before the requested state change. |
| `4` | A native condition needs attention, such as a startup prompt. |
| `5` | The result is unknown, including delivery typed without a matching witness. |

## Configuration and state

The config file is `$GANG_CONFIG_DIR/config`, defaulting to
`${XDG_CONFIG_HOME:-~/.config}/gangline/config`. It is parsed as strict
`NAME=VALUE` data; it is never sourced. Environment values override file values.

| Setting | Default | Purpose |
| --- | --- | --- |
| `GANG_SESSION` | `gangline` | Team and tmux session name. |
| `GANG_COLLAR` | `claude-code` | Default collar for launch and model discovery. |
| `GANG_COLLARS` | unset | Absolute directory of operator `NAME.cue` collars. |
| `GANG_LAUNCH_ARGS` | unset | JSON object of collar names to extra launch-argument arrays. |

Runtime-only variables are `GANG_CONFIG_DIR`, `GANG_STATE_ROOT`, `GANG_TMUX`,
and `GANG_TMUX_SOCKET`. The state default is
`${XDG_STATE_HOME:-~/.local/state}/gangline`; tmux uses its default socket unless
an explicit socket is supplied. `GANG_TMUX` selects the tmux executable for
tests and native collar probes.

Each team is stored at `STATE_ROOT/v1/TEAM/`. `events.jsonl` is authoritative;
`snapshot.json` is an integrity-checked loading shortcut. `gang down SESSION`
removes that directory after every active or failed hitch is dropped.

## Startup prose

Gangline embeds its shipped contract and role briefs from `internal/prose/`.
Operator files under `GANG_CONFIG_DIR` override them:

- `CONTRACT.md` supplies standing terms;
- `DOCTRINE.md` adds optional operator guidance; and
- `roles/NAME.md` replaces or adds a role brief.

The collar's `role_prompt` option puts standing prose into a native system
prompt when the harness supports it. The startup envelope still carries the
assignment.

## CUE collar contract

A custom collar is a CUE file with a top-level `collar` value validated against
`harness/schema/collar.cue`. Its filename and `collar.name` must agree.

The value declares:

- `launch`: command, normal/resume/probe arguments, and environment;
- `hooks`: launch arguments containing `{{hook.command.json}}`, plus native
  event and payload mappings;
- `models`: catalog and selected-model primitives and the model option;
- `options`: optional effort and role-prompt argument templates;
- `primitives`: startup, composer, submit, submit witness, turn boundary,
  runtime blocked, context, provider limits, and wedge operations;
- `actions`: interrupt, compact, and recovery; and
- `context_bands`: ordered named thresholds per model selector.

Logic stays in committed Go primitives. A collar selects and parameterizes
those primitives; unknown primitive names fail during collar validation.
Shipped collars are embedded CUE values under `harness/collars/` and pass
through the same loader as operator collars.
