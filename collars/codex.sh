# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_collar via source
# SPDX-License-Identifier: Apache-2.0
[ -z "${name:-}" ] || export OTEL_RESOURCE_ATTRIBUTES="gang.agent=$name${OTEL_RESOURCE_ATTRIBUTES:+,$OTEL_RESOURCE_ATTRIBUTES}"
GANG_LAUNCH="codex -c check_for_update_on_startup=false"
GANG_RESUME_LAUNCH="codex resume {{session_id}} -c check_for_update_on_startup=false"
# A HOSTILE ROOT IS DECLINED, NOT ESCAPED. The hook TOML rides inside a
# single-quoted -c word, so one quote in the install path closes that word and
# the remainder of the path is shell code the new window runs under the
# operator's account. There is no escaping that survives both the TOML string
# and the shell word around it, so this collar does what its sibling
# claude-code.sh does: for a root bearing a quote, a backslash or a control
# character it installs no hooks at all. A hookless launch loses turn-boundary
# events; it does not execute a directory name.
# THE PREFLIGHT IS PART OF THIS COLLAR, so it is found beside this file rather
# than under the install root: a collar copied into GANG_COLLARS carries its own
# helpers, and the copy answers for itself instead of reaching back into the
# shipped one. The hook command still names $ROOT/bin/gang, because a hook has
# to reach the Gangline that installed it.
_gl_codex_dir="${BASH_SOURCE[0]%/*}"
if [ -n "${ROOT:-}" ] && [ -x "$ROOT/bin/gang" ]; then
  case "$ROOT$_gl_codex_dir" in
    *[\'\"\\]*|*[[:cntrl:]]*) ;;
    *)
      _gl_codex_hook="[{ hooks = [{ type = \"command\", command = \"\\\"$ROOT/bin/gang\\\" hook\" }] }]"
      # Codex runs this Stop handler instead of the generic gang hook. It
      # blocks only the first objectively proved unreported turn; every allowed
      # boundary delegates back to `gang hook` so ordinary turn bookkeeping,
      # spool delivery, and deferred compaction retain their existing owner.
      # CODEX TRUSTS THIS EVENT/COMMAND STRING, NOT THE CONTENTS OF THE
      # EXECUTABLE IT NAMES.  An in-place edit to codex-stop-hook.py therefore
      # runs on later turns without a new native trust prompt; changing this
      # command or its path mints a new hash and requires a person to trust it.
      # This is trust-on-first-use over a name, not code attestation.  Do not
      # add a changing version token here: it would impose a re-trust dialog on
      # every helper edit.  A Gangline-side content check has no consumer yet;
      # build one only if this boundary must become an enforced control.
      # The native bound is the outer fuse for the helper.  Its three Gangline
      # subprocesses each have their own smaller bound, so neither a wedged
      # lock nor a failed recovery can hold Codex at Stop indefinitely.
      _gl_codex_stop_hook="[{ hooks = [{ type = \"command\", command = \"python3 \\\"$_gl_codex_dir/plugins/codex-stop-hook.py\\\" \\\"$ROOT/bin/gang\\\"\", timeout = 15 }] }]"
      _gl_codex_hook_flags=""
      # PreCompact/PostCompact are wired for the same reason claude-code wires
      # them: @gl_turn is closed for the whole of a compaction, and the turn
      # witness outranks the pane, so without the bracket a compacting codex
      # agent reads IDLE and gang delivers into it. Codex declares no queue
      # evidence, so that delivery would be reported submitted when the harness
      # had parked it. Verified firing on 0.146.0.
      for _gl_codex_event in UserPromptSubmit PostToolUse PermissionRequest \
                             PreCompact PostCompact; do
        _gl_codex_hook_flags+=" -c 'hooks.$_gl_codex_event=$_gl_codex_hook'"
      done
      _gl_codex_hook_flags+=" -c 'hooks.Stop=$_gl_codex_stop_hook'"
      GANG_LAUNCH="$GANG_LAUNCH$_gl_codex_hook_flags"
      GANG_RESUME_LAUNCH="$GANG_RESUME_LAUNCH$_gl_codex_hook_flags"
      # THE HOOKS INSTALLED ABOVE ARE WHAT CODEX ASKS ABOUT. Their command
      # carries this install root, so a new install, an upgrade or a worktree
      # presents hashes codex has never seen and it opens its hooks-review menu
      # before drawing a composer. Nothing in Gangline can answer that menu, so
      # an unattended hitch waits on a person until its foreground gate bound.
      # The preflight runs
      # in the pane ahead of the harness and turns that wait into an immediate,
      # explained refusal; it grants no trust. It wraps only this branch,
      # because a launch that installs no hooks raises no review.
      _gl_codex_preflight="python3 '$_gl_codex_dir/plugins/codex-hooks-preflight.py'"
      GANG_LAUNCH="$_gl_codex_preflight $GANG_LAUNCH"
      GANG_RESUME_LAUNCH="$_gl_codex_preflight $GANG_RESUME_LAUNCH"
      unset _gl_codex_preflight
      # The launch above passes a native Stop hook with -c, so this harness
      # announces its own turn boundaries to gang — which is what a spool needs
      # to drain, and what deferred self-compaction already relies on. Both are
      # declared here, beside the hooks that deliver them, so the hookless
      # launch above claims neither.
      GANG_STOP_HOOK=1
      GANG_SELF_COMPACT=deferred
      unset _gl_codex_hook _gl_codex_stop_hook _gl_codex_hook_flags _gl_codex_event
      ;;
  esac
fi
unset _gl_codex_dir
GANG_MODEL_OPT="-m"
# CONTEXT-LIGHT DEFAULTS FOR A NARROW WINDOW. Observed 2026-08-24 on this
# installation: a codex agent reports a 258k window, and every model this
# catalog enumerates is that class, so one pair covers them all rather than a
# per-model split this harness gives no evidence for. It is later than the
# claude-code default on purpose: these models hold quality right up to their
# auto-compaction limit, so a warning sized to a wide window's runway fires
# while plenty of useful completion remains — noise, not signal.
#
# Fractions rather than token counts, so the pair survives a provider changing
# the window; `collar_context` reads the live one. Omitting -m leaves the model
# unknown, and hitch already warns about that.
collar_context_lights() { # $1 model; 0 with thresholds, 1 = no default for it
  case "$1" in
    '') return 1 ;;
    *) printf '75%%,90%%\n' ;;
  esac
}
# The native JSON catalog is complete for this installed harness. Each row
# carries its slug and the reasoning efforts that exact model advertises, so
# `gang models` can show both and hitch can refuse an id absent from the same
# source before opening a window. Observed on codex 0.146.0.
collar_models() {
  python3 - <<'PY'
import json
import subprocess

try:
    result = subprocess.run(
        ["codex", "debug", "models"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        timeout=10,
    )
except (subprocess.TimeoutExpired, OSError, UnicodeError):
    raise SystemExit(1)
if result.returncode:
    raise SystemExit(result.returncode)
try:
    catalog = json.loads(result.stdout)
except (TypeError, ValueError, json.JSONDecodeError):
    raise SystemExit(1)
models = catalog.get("models") if isinstance(catalog, dict) else None
if not isinstance(models, list) or not models:
    raise SystemExit(1)
seen = set()
for item in models:
    if not isinstance(item, dict):
        raise SystemExit(1)
    slug = item.get("slug")
    rows = item.get("supported_reasoning_levels")
    if not isinstance(slug, str) or not slug or slug in seen:
        raise SystemExit(1)
    if not isinstance(rows, list):
        raise SystemExit(1)
    levels = []
    for row in rows:
        effort = row.get("effort") if isinstance(row, dict) else None
        if not isinstance(effort, str) or not effort or effort in levels:
            raise SystemExit(1)
        levels.append(effort)
    seen.add(slug)
    print(slug, *([",".join(levels)] if levels else []), sep="\t")
PY
}
# REASONING EFFORT IS MODEL-SCOPED. The option includes its separator because
# bin/gang joins it to the level with no space. Unquoted `high` reached
# turn_context.payload.effort as "high"; an unquoted invented value reached the
# provider and was refused there, so the raw-string fallback documented by
# `codex --help` is observed rather than inferred.
GANG_EFFORT_OPT="-c model_reasoning_effort="
# `codex debug models` is the harness's own live vocabulary. GANG_MODEL is the
# exact model hitch is about to pass with -m; when hitch passes none, the model
# Codex will use is read from $CODEX_HOME/config.toml (default
# ~/.codex/config.toml). An absent, unreadable, malformed, or unbound configured
# model, an alias the catalog cannot bind, a failed or wedged command (bounded
# by the timeout), malformed JSON, a missing field, a duplicate, or whitespace
# inside a level all produce NOTHING. The reader names that as a broken
# GANG_EFFORT_CMD rather than blaming the operator's level. Validate the whole
# catalog row before printing so a plausible prefix can never escape from an
# answer the parser could not finish. The final `|| true` is part of that
# protocol: bin/gang distinguishes failure from a bad level by EMPTY OUTPUT.
#
# tomllib is imported WHERE IT IS USED rather than at the top. It arrived in
# python3 3.11, and importing it up front made an explicit model — which never
# opens the config at all — stop answering on every older python3 that used to.
# Its decode error is a ValueError, so the handler names no module the import
# may have failed to bind.
GANG_EFFORT_CMD="python3 -c '
import json, os, subprocess

try:
    result = subprocess.run(
        [\"codex\", \"debug\", \"models\"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        timeout=10,
    )
except (subprocess.TimeoutExpired, OSError):
    raise SystemExit(1)
if result.returncode:
    raise SystemExit(result.returncode)

catalog = json.loads(result.stdout)
models = catalog.get(\"models\") if isinstance(catalog, dict) else None
model = os.environ.get(\"GANG_MODEL\", \"\")
if not model:
    config_home = os.environ.get(\"CODEX_HOME\") or os.path.expanduser(\"~/.codex\")
    try:
        import tomllib
        with open(os.path.join(config_home, \"config.toml\"), \"rb\") as stream:
            config = tomllib.load(stream)
    except (ImportError, OSError, UnicodeError, ValueError):
        raise SystemExit(1)
    model = config.get(\"model\") if isinstance(config, dict) else None
if not isinstance(model, str) or not model or not isinstance(models, list):
    raise SystemExit(1)

matches = [
    item for item in models
    if isinstance(item, dict) and item.get(\"slug\") == model
]
if len(matches) != 1:
    raise SystemExit(1)

rows = matches[0].get(\"supported_reasoning_levels\")
if not isinstance(rows, list) or not rows:
    raise SystemExit(1)
if any(
    not isinstance(row, dict)
    or not isinstance(row.get(\"effort\"), str)
    or not row[\"effort\"]
    or any(char.isspace() for char in row[\"effort\"])
    for row in rows
):
    raise SystemExit(1)

levels = [row[\"effort\"] for row in rows]
if len(levels) != len(set(levels)):
    raise SystemExit(1)
print(*levels, sep=\"\\n\")
' || true"
GANG_BUSY_REGEX="esc to interrupt"
GANG_QUIET_AT_REST=1
GANG_OCCUPIED_REGEX='^› [0-9]+\. '
# THIS IS IN-SESSION TYPOGRAPHY. Every menu codex draws inside a session uses
# U+203A (bytes 342 200 272). Its pre-session screens are drawn by another code
# path that does not share the alphabet: the first-run sign-in menu observed on
# 0.146.0 rows with ASCII ">" (0x3E), so this marker scores zero against it.
# Any screen a fresh codex draws before a session exists is outside this
# marker's reach.
#
# Enumerated on 0.146.0 against a cold CODEX_HOME: the sign-in menu, the
# browser-wait screen carrying the authorize URL, the device-code screen, and
# the API-key entry screen. NOTHING this collar declares reaches any of them —
# collar_input keys on the same U+203A, and the busy footer belongs to a turn
# that cannot exist yet. The
# API-key screen is a real bordered text field and is still not a composer by
# that definition, which is the definition working: gang reads it as occupied
# (authority unknown), refuses /usage, spools rather than delivers, and hitch
# fails without naming what it saw.
#
# Widening this to ^[›>] is REFUSED. Pre-session and in-session are different
# code paths, not different builds, and the ASCII form is ordinary transcript
# text: a delivered message quoting "> 1. …" would read as an occupied
# composer inside a live session.
GANG_COMPACT_CMD="/compact"
# A rollout snapshot older than this is still printable evidence, but is not
# current enough to drive a light or arm a reset wake. The next API turn
# refreshes it. Local file reads need no hook throttle.
GANG_USAGE_LIMIT_MAX_AGE=300

# Provider-limit decisions read the exact target session's latest native
# rate_limits event, never the interactive usage transcript above. The event is
# refreshed by API turns and its timestamp therefore exposes staleness rather
# than hiding it. Verified on codex 0.146.0.
collar_usage_limits() { # $1 = tmux target; print label<TAB>used<TAB>reset<TAB>observed
  local file
  file="$(codex_session_file "$1")" || return 1
  python3 -c '
from datetime import datetime
import json
import sys

path = sys.argv[1]

def newest_lines(path):
    with open(path, "rb") as stream:
        stream.seek(0, 2)
        position = stream.tell()
        carry = b""
        while position:
            size = min(65536, position)
            position -= size
            stream.seek(position)
            parts = (stream.read(size) + carry).split(b"\n")
            carry = parts[0]
            yield from reversed(parts[1:])
        if carry:
            yield carry

record = None
for raw in newest_lines(path):
    if b"\"rate_limits\"" not in raw:
        continue
    try:
        candidate = json.loads(raw)
    except ValueError:
        continue
    payload = candidate.get("payload")
    limits = payload.get("rate_limits") if isinstance(payload, dict) else None
    if isinstance(limits, dict):
        record = candidate
        break
if record is None:
    raise SystemExit(1)

stamp = record.get("timestamp")
if not isinstance(stamp, str):
    raise SystemExit(1)
try:
    observed = int(datetime.fromisoformat(stamp.replace("Z", "+00:00")).timestamp())
except ValueError:
    raise SystemExit(1)

limits = record["payload"]["rate_limits"]
name = limits.get("limit_name") or limits.get("limit_id") or "provider"
rows = []
for key in ("primary", "secondary"):
    window = limits.get(key)
    if not isinstance(window, dict):
        continue
    used = window.get("used_percent")
    minutes = window.get("window_minutes")
    reset = window.get("resets_at")
    if isinstance(used, bool) or isinstance(minutes, bool) or isinstance(reset, bool):
        raise SystemExit(1)
    if not isinstance(used, (int, float)) or not isinstance(minutes, int) or not isinstance(reset, int):
        raise SystemExit(1)
    if used < 0 or used > 100 or int(used) != used or minutes <= 0 or reset <= 0:
        raise SystemExit(1)
    if minutes == 300:
        window_name = "5-hour"
    elif minutes == 10080:
        window_name = "weekly"
    else:
        window_name = f"{minutes}-minute"
    rows.append((f"{name} {window_name}", int(used), reset, observed))
if not rows:
    raise SystemExit(1)
for row in rows:
    print(*row, sep="\t")
' "$file"
}
GANG_MIDTURN_INPUT=1
# Escape stops an active turn; the busy marker above is the harness's own
# "esc to interrupt" footer.
GANG_INTERRUPT_KEY="Escape"
# On codex-cli 0.151.0 shift+Left loads the last queued follow-up
# back into the composer without submitting it, and the queue block disappears
# as it does. The harness advertises this key itself, and collar_queued below
# refuses to call a queue recognized unless it is still advertising it.
#
# RECALL IS A RECOVERY, NOT THE ONLY ONE. Codex drains its own follow-up queue
# as soon as the open turn closes, so a parked body is a delayed delivery
# rather than a lost one and flush is worth reaching for only while a turn is
# wedged open.
#
# AND IT REACHES ONE-LINE BODIES ONLY. The recalled composer indents its
# continuation lines under `› `, and collar_input keeps the LAST line matching
# `^›`, so a multi-line body reads back short. cmd_flush compares the whole
# recalled reading against the whole recorded body, so for any envelope of more
# than one line that comparison cannot succeed and flush refuses without
# pressing Enter. The refusal is correct; the recovery is simply unavailable.
# Widening collar_input to close that gap would change the reading every
# delivery depends on, which is the trade this collar does not make.
GANG_QUEUE_RECALL_KEY="S-Left"

# CODEX PARKS INPUT WHERE GANG'S COMPOSER READING CANNOT SEE IT. Enter during a
# running turn is accepted into a native follow-up queue: the composer goes
# back to its placeholder, exactly as a submitted message leaves it, and the
# queued bodies are drawn ABOVE the composer under a header. A collar that
# answered this with GANG_QUEUED_REGEX would be matched against the composer
# alone and would report every parked delivery as submitted, so the pane is
# what gets read here.
#
# THE HEADER ALONE IS NOT ENOUGH. An agent can be handed text that quotes it,
# so the recall advertisement has to be on screen too — and a header with no
# advertisement is a native rendering this collar no longer understands, which
# is an unknown rather than a queue gang could offer to flush.
# CODEX SAYS SO ITSELF. Its provider-latency menu owns the input box while the
# agent behind it keeps working, and the menu's own last line states that no
# action is required and that it closes when the response is ready. That
# sentence is the evidence: it is Codex declaring the surface advisory, so it
# is what gets matched rather than any particular menu's wording above it.
#
# A MISS COSTS NOTHING. Where this does not match, gang reports the occupancy
# it always reported; nothing here decides whether to type, and no key is ever
# sent at the menu.
collar_advisory() { # $1 = tmux target; 0 + what it is, 1 = not an advisory surface
  local pane
  pane="$(tmux capture-pane -pJ -t "$1" 2>/dev/null)" || return 1
  printf '%s\n' "$pane" | grep -qF \
    'No action is required. Codex will keep waiting, and this menu will close when the response is ready.' \
    || return 1
  printf 'Codex is waiting on its provider and says the menu closes by itself when the response is ready'
}

# READ THE PANE PLAIN, NEVER THE DIM-STRIPPED READING. Codex draws both the
# queue header and the recall advertisement inside ANSI dim runs, and the awk
# in collar_input deletes a dim run wholesale — so the reading that serves the
# composer would drop the advertisement entirely and leave this function
# unable to tell a recognized queue from a rendering it no longer understands.
# The capture below is therefore plain `-pJ`, with no `-e` and no stripping.
collar_queued() { # $1 tmux target, $2 the body gang composed (optional).
                  # Without a body: 0 the harness holds parked input, 1 it
                  # does not, 2 unknown with a cause.
                  # With a body: 0 that exact body is parked, 2 not confirmed.
                  # The body form never returns 1: a body gang cannot find in
                  # the queue block is a reading this collar could not make,
                  # not proof the message entered the session.
  local pane
  pane="$(tmux capture-pane -pJ -t "$1" 2>/dev/null)" || {
    printf 'the Codex pane could not be read for parked-queue evidence'
    return 2
  }
  printf '%s\n' "$pane" | grep -qE '^• Queued follow-up inputs *$' || {
    [ $# -lt 2 ] || {
      printf 'the Codex follow-up queue is not on screen, so gang cannot confirm the body it composed is parked'
      return 2
    }
    return 1
  }
  ! printf '%s\n' "$pane" | grep -qE '^ *shift \+ ← edit last queued message *$' || {
    [ $# -ge 2 ] || return 0
    printf '%s\n' "$pane" | python3 -c '
import re
import sys

want = " ".join(sys.argv[1].split())
lines = sys.stdin.read().split("\n")
header = re.compile(r"^\u2022 Queued follow-up inputs *$")
hint = re.compile(r"^ *shift \+ \u2190 edit last queued message *$")
opened = [i for i, line in enumerate(lines) if header.match(line)]
closed = [i for i, line in enumerate(lines) if hint.match(line)]
if not want or not opened or not closed or closed[-1] <= opened[0]:
    raise SystemExit(1)
block = " ".join(lines[opened[0] + 1 : closed[-1]]).replace("\u21b3", " ")
raise SystemExit(0 if want in " ".join(block.split()) else 1)
' "$2" && return 0
    printf 'the Codex follow-up queue is on screen but does not read back as the body gang composed'
    return 2
  }
  printf 'the Codex follow-up queue is on screen but no longer advertises the shift+Left recall this collar sends'
  return 2
}
# Verified on codex 0.145.0: the native hook set contains no Notification
# event. legacy_notify / agent-turn-complete reports turn completion, which the
# Stop hook above already delivers; it is not an awaiting-input witness and is
# deliberately not wired. PermissionRequest is this collar's only stall source.
# UNVERIFIABLE BY ENUMERATION on 0.146.0, which is a stronger statement than
# unverified. There is no enumeration surface to find: `codex debug` exposes
# models, app-server and prompt-input, and none lists an event. Reading the
# names out of the binary is not a substitute — that method was tested against
# events known to be real, returned zero for every one of them, and so reports
# absence it cannot see. Its silence is not evidence.
#
# The only route left is probing event names one at a time and proving a
# negative from the misses. Recorded this way so nobody goes looking for a menu
# that does not exist: claude-code has one and this harness does not, which is
# a difference between the harnesses rather than a gap in what we bothered to
# check.

# HOOKS ARE TRUSTED, NOT MERELY INSTALLED (observed on codex 0.146.0). Trust is
# persisted in config.toml's [hooks.state] table. The KEY is a fixed sentinel,
# /<session-flags>/config.toml:<event>:0:0 — it does not move with the
# checkout, which is what makes a pre-seeded trust table addressable at all.
# What varies is the VALUE: a hash over the hook COMMAND, which embeds $ROOT,
# and over the EVENT, so each event wired above needs its own hash.
# It is deterministic — no salt, no nonce; the same root recomputed in a fresh
# sandbox reproduces the same bytes. A checkout whose commands have been
# trusted once boots straight to its composer; a fresh clone, a moved checkout,
# a container, or any edit to the command above meets this first:
#
#     Hooks need review
#     N hooks are new or changed.
#     Hooks can run outside the sandbox after you trust them.
#   › 1. Review hooks
#     2. Trust all and continue
#     3. Continue without trusting (hooks won't run)
#
# Nothing here answers it. A collar that pressed "Trust all and continue" would
# be granting the operator's approval to run hooks outside the sandbox on their
# behalf, and option 3 is a keystroke away from a codex agent with no Stop hook
# at all. Cooperative ticks can still retry accepted mail against the live
# composer, but no native turn fact, wait boundary, or inside-harness deferred
# self-compaction request will arrive.
#
# GANG_OCCUPIED_REGEX matches this menu, so the window reads !occupied!
# (authority unknown) and no key is sent. Hitch parks its startup contract,
# remains in the foreground, and delivers through the composer after either
# operator choice; it never assumes configured hooks were trusted.

codex_session_file() { # $1 = tmux target -> this window's bound rollout path
  local file
  file="$(tmux show-options -wqv -t "$1" @gl_session)" || file=""
  [ -n "$file" ] && [ -f "$file" ] || return 1
  printf '%s' "$file"
}

# THE LIVE PROCESS, NOT THE SESSIONS DIRECTORY. A Codex process holds its
# thread-writer lock open for the life of the conversation, and its rollout
# from its first turn onward. A restarted harness therefore presents a
# different exact id even when it was launched in the same pane and cwd. Gang's
# ordinary sandbox cannot see the tmux server's host /proc namespace, so this
# bounded read runs through tmux's server-side run-shell and returns through
# one cleanup-owned temporary file. The helper reads the lock as the authority
# and the rollout as corroboration wherever one exists; it refuses an id that
# either witness contradicts, and refuses when neither settles a single answer.
collar_live_session_id() { # $1 = tmux target; print the exact id, or return 1
  local pane_pid tmp command live=""
  pane_pid="$(tmux display-message -p -t "$1" '#{pane_pid}' 2>/dev/null)" \
    || pane_pid=""
  case "$pane_pid" in ''|*[!0-9]*) return 1 ;; esac
  tmp="$(mktemp "${TMPDIR:-/tmp}/gangline-codex-live-id.XXXXXX")" || return 1
  # THE FAILURE GUARD BELONGS INSIDE THE COMMAND, NOT AROUND THE CLIENT CALL.
  # tmux renders a run-shell that exits nonzero into the TARGET PANE: it names
  # the window [tmux], drops the pane into view-mode over the harness TUI, and
  # prints "'<command>' returned 1" there. Redirecting this client's own output
  # cannot reach that -- the server draws it, and the pane it draws over is the
  # agent's. A probe that finds no id yet is an ordinary answer on this path
  # (the harness has not opened its lock, or has only just been launched), so
  # every unremarkable miss covered the agent's screen and took its keystrokes
  # into a copy-mode overlay. Exiting zero inside the command is what the
  # server reads; emptiness of the temp file is what tells this caller.
  command="$(shell_quote "$ROOT/libexec/gang-codex-live-id") $(shell_quote "$pane_pid") > $(shell_quote "$tmp") 2>/dev/null || :"
  tmux run-shell -t "$1" "$command" >/dev/null 2>&1 || :
  IFS= read -r live < "$tmp" || live=""
  rm -f -- "$tmp"
  case "$live" in ''|*[!A-Za-z0-9._:-]*) return 1 ;; esac
  printf '%s' "$live"
}

# A HARNESS ROOT IS A NARROWER WITNESS THAN A PANE. The pane may remain alive
# after Codex exits, but a descendant scan would mistake one ordinary sandboxed
# tool call for the harness. At hitch/adopt Gangline records this root's PID
# together with Linux's non-reusable process start stamp. Later tick passes ask
# for the same positive witness; a process that is merely alive is never spent
# as a health verdict.
collar_harness_identity() { # $1 = tmux target; PID<TAB>start stamp, 0/1/2
  local pane_pid tmp status command observed="" probe_rc=""
  pane_pid="$(tmux display-message -p -t "$1" '#{pane_pid}' 2>/dev/null)" \
    || pane_pid=""
  case "$pane_pid" in
    ''|*[!0-9]*) printf 'the Codex pane-root process id is unreadable'; return 2 ;;
  esac
  tmp="$(mktemp "${TMPDIR:-/tmp}/gangline-codex-process-identity.XXXXXX")" \
    || { printf 'could not reserve a Codex liveness probe result'; return 2; }
  status="${tmp}.status"
  # See collar_live_session_id above: a nonzero run-shell result paints an
  # overlay in the target pane. The helper's status is therefore carried in a
  # cleanup-owned side file while the server-side command itself succeeds.
  command="$(shell_quote "$ROOT/libexec/gang-process-identity") --codex $(shell_quote "$pane_pid") > $(shell_quote "$tmp") 2>/dev/null; printf '%s\\n' \$? > $(shell_quote "$status"); :"
  tmux run-shell -t "$1" "$command" >/dev/null 2>&1 || :
  IFS= read -r observed < "$tmp" || observed=""
  IFS= read -r probe_rc < "$status" || probe_rc=""
  rm -f -- "$tmp" "$status"
  case "$probe_rc" in
    0)
      if [[ "$observed" =~ ^[0-9]+$'\t'[0-9]+$ ]] \
        && [ "${observed%%$'\t'*}" = "$pane_pid" ]; then
        printf '%s' "$observed"
        return 0
      fi
      printf 'the Codex liveness probe returned an invalid identity'
      return 2 ;;
    1) return 1 ;;
    2) printf 'the Codex pane-root liveness witness is unreadable'; return 2 ;;
    *) printf 'the Codex liveness probe returned no readable status'; return 2 ;;
  esac
}

# A STOPPED TURN CAN STILL HOLD NATIVE WORK. Codex keeps background terminal
# processes under the harness process, spawned-agent edges in its state store,
# and automatic goal continuations in its goal store. None is intent: each is a
# resource the harness has actually retained after the turn. Observed on Codex
# 0.149.1. This reader walks only a bounded process set and a bounded number/tail
# of child records, and it is called only from Gangline's existing native-hook
# pass — never by a patrol.
collar_waiting() { # $1 target, $2 Stop payload; print witness, 0 held, 1 absent, 2 unknown
  local pane_pid session_id codex_home
  pane_pid="$(tmux display-message -p -t "$1" '#{pane_pid}' 2>/dev/null)" \
    || pane_pid=""
  session_id="$(tmux show-options -wqv -t "$1" @gl_session_id 2>/dev/null)" \
    || session_id=""
  case "$pane_pid" in ''|*[!0-9]*) printf 'the Codex harness process id is unreadable'; return 2 ;; esac
  case "$session_id" in
    ''|*[!A-Za-z0-9._:-]*) printf 'the Codex session id is unreadable'; return 2 ;;
  esac
  codex_home="${CODEX_HOME:-$HOME/.codex}"
  python3 - "$pane_pid" "$session_id" "$codex_home" <<'PY'
import glob
import json
import os
import sqlite3
import sys
from urllib.parse import quote

root_pid = int(sys.argv[1])
thread_id = sys.argv[2]
codex_home = sys.argv[3]
max_processes = 128
max_children = 64
tail_bytes = 262144
# BOUNDED CONVERGENCE, NOT A POLL. A tree read while Codex is starting or
# reaping a tool call can differ between two adjacent reads, and a single
# unlucky read is cached as the whole idle period's answer. The walk is
# therefore repeated until two consecutive readings agree, with no delay
# between them, and a tree that never settles says exactly that -- which is a
# different fact from a tree that settled on an answer.
max_tree_reads = 4


class TreeGone(Exception):
    """The Codex harness process itself is no longer there."""


class TreeRaced(Exception):
    """This probe could not read its own place in the tree."""


def unknown(message):
    print(message)
    raise SystemExit(2)


def proc_text(pid, leaf):
    with open(f"/proc/{pid}/{leaf}", encoding="utf-8") as stream:
        return stream.read().strip()


def own_ancestry():
    """Every process between this probe and init.

    At a Stop boundary the probe runs under Codex, so its own branch of the
    tree is Gangline's work rather than work Codex is holding. An ancestor
    that exits mid-climb truncates the chain and would readmit that branch as
    held work, so a truncated climb is repeated rather than accepted short.
    """
    for _ in range(max_tree_reads):
        chain = set()
        cursor = os.getpid()
        try:
            while cursor > 1 and cursor not in chain:
                chain.add(cursor)
                cursor = int(proc_text(cursor, "stat").rsplit(") ", 1)[1].split()[1])
        except FileNotFoundError:
            continue
        return chain
    raise TreeRaced


def process_table():
    """Parent to children, from one pass over /proc.

    Read the parent recorded by each process rather than
    /proc/PID/task/*/children. The per-thread children files list only the
    children of the thread that forked them, and Codex is multithreaded and
    forks its tool calls from worker threads, so the main thread's file is
    empty for a harness that is running a command right now. A thread that
    has since exited takes its file with it while its child keeps running,
    which no reading of those files can recover.
    """
    if not os.path.isdir(f"/proc/{root_pid}"):
        raise TreeGone
    table = {}
    for entry in os.listdir("/proc"):
        if not entry.isdigit():
            continue
        pid = int(entry)
        try:
            parent = int(proc_text(pid, "stat").rsplit(") ", 1)[1].split()[1])
        except (FileNotFoundError, IndexError, ValueError):
            continue
        table.setdefault(parent, []).append(pid)
    return table


def printable(value):
    return "".join(char if " " <= char < "\x7f" else "?" for char in value)[:32]


def held_witness(pid, own_tree):
    """Name pid when it is work Codex is holding, else None.

    Codex's own session-lifetime helpers run its shipped binaries out of
    CODEX_HOME; a tool call runs a program from outside it, and often not a
    shell -- an uncontained command is exec'd directly and a sandboxed one
    runs under a container helper. A Codex-owned process is therefore walked
    through rather than skipped, because the command sits beneath it.
    """
    try:
        state = proc_text(pid, "stat").rsplit(") ", 1)[1].split()[0]
    except (FileNotFoundError, IndexError):
        return None
    if state == "Z":
        return None
    try:
        exe = os.readlink(f"/proc/{pid}/exe")
    except FileNotFoundError:
        return None
    except PermissionError:
        exe = ""
    if exe.endswith(" (deleted)"):
        exe = exe[: -len(" (deleted)")]
    if exe and os.path.realpath(exe).startswith(own_tree):
        return None
    try:
        command = proc_text(pid, "comm")
    except FileNotFoundError:
        return None
    return f"a live Codex child process ({printable(command)})"


def held_read(ancestors, own_tree):
    table = process_table()
    queue = [root_pid]
    seen = {root_pid}
    while queue:
        parent = queue.pop(0)
        for child in table.get(parent, ()):
            if child in seen:
                continue
            seen.add(child)
            if len(seen) > max_processes:
                unknown(
                    f"the Codex child-process probe exceeded its {max_processes}-process bound"
                )
            if child in ancestors:
                continue
            queue.append(child)
            witness = held_witness(child, own_tree)
            if witness is not None:
                return witness
    return ""


try:
    own_tree = os.path.realpath(codex_home) + os.sep
    ancestors = own_ancestry()
    held = ""
    settled = False
    for attempt in range(max_tree_reads):
        reading = held_read(ancestors, own_tree)
        if attempt and reading == held:
            settled = True
            break
        held = reading
    if not settled:
        unknown(
            f"the Codex child-process tree did not settle across {max_tree_reads} reads"
        )
    if held:
        print(held)
        raise SystemExit(0)
except TreeGone:
    unknown("the Codex harness process is no longer running")
except TreeRaced:
    unknown("the Codex probe could not read its own place in the process tree")
except (OSError, UnicodeError, ValueError):
    unknown("the Codex child-process tree is unreadable")


def open_readonly(path):
    uri = f"file:{quote(os.path.abspath(path))}?mode=ro"
    return sqlite3.connect(uri, uri=True, timeout=0.05)


def newest_task_state(path):
    try:
        with open(path, "rb") as stream:
            stream.seek(0, 2)
            size = stream.tell()
            start = max(0, size - tail_bytes)
            stream.seek(start)
            data = stream.read(tail_bytes)
    except OSError:
        return "unknown"
    if start:
        cut = data.find(b"\n")
        if cut < 0:
            return "unknown"
        data = data[cut + 1 :]
    if data and not data.endswith(b"\n"):
        data = data.rsplit(b"\n", 1)[0] + b"\n"
    for raw in reversed(data.splitlines()):
        if b'"task_started"' not in raw and b'"task_complete"' not in raw:
            continue
        try:
            record = json.loads(raw)
        except (UnicodeError, ValueError):
            return "unknown"
        payload = record.get("payload")
        kind = payload.get("type") if isinstance(payload, dict) else None
        if kind == "task_started":
            return "running"
        if kind == "task_complete":
            return "complete"
    return "unknown" if start else "pending"


state_paths = glob.glob(os.path.join(codex_home, "state_[0-9]*.sqlite"))
if state_paths:
    def state_version(path):
        stem = os.path.basename(path)[len("state_") : -len(".sqlite")]
        return int(stem) if stem.isdigit() else -1

    state_path = max(state_paths, key=state_version)
    try:
        with open_readonly(state_path) as database:
            rows = database.execute(
                """
                WITH RECURSIVE descendants(child_thread_id) AS (
                    SELECT child_thread_id FROM thread_spawn_edges
                    WHERE parent_thread_id = ? AND status = 'open'
                    UNION ALL
                    SELECT edge.child_thread_id FROM thread_spawn_edges AS edge
                    JOIN descendants ON edge.parent_thread_id = descendants.child_thread_id
                    WHERE edge.status = 'open'
                )
                SELECT descendants.child_thread_id, threads.rollout_path
                FROM descendants
                LEFT JOIN threads ON threads.id = descendants.child_thread_id
                LIMIT ?
                """,
                (thread_id, max_children + 1),
            ).fetchall()
    except (OSError, sqlite3.Error):
        unknown("the Codex spawned-task store is unreadable")
    if len(rows) > max_children:
        unknown("the Codex spawned-task probe exceeded its 64-task bound")
    for _, rollout_path in rows:
        if not isinstance(rollout_path, str) or not rollout_path:
            unknown("a Codex spawned task has no readable session record")
        state = newest_task_state(rollout_path)
        if state in {"running", "pending"}:
            print("an unfinished Codex spawned task")
            raise SystemExit(0)
        if state == "unknown":
            unknown("a Codex spawned task has no readable exit record within the bounded scan")

goals_path = os.path.join(codex_home, "goals_1.sqlite")
if os.path.exists(goals_path):
    try:
        with open_readonly(goals_path) as database:
            row = database.execute(
                "SELECT status FROM thread_goals WHERE thread_id = ?",
                (thread_id,),
            ).fetchone()
    except (OSError, sqlite3.Error):
        unknown("the Codex goal-wakeup store is unreadable")
    if row and row[0] in {"active", "usage_limited"}:
        print("an armed Codex goal continuation")
        raise SystemExit(0)

raise SystemExit(1)
PY
}

collar_session_id() { # $1 = tmux target, $2 = native hook payload
  local value transcript
  value="$(printf '%s' "$2" | python3 -c '
import json, sys
value = json.load(sys.stdin).get("session_id", "")
if not isinstance(value, str) or not value:
    raise SystemExit(1)
print(value)
')" || return 1
  transcript="$(printf '%s' "$2" | python3 -c '
import json, sys
value = json.load(sys.stdin).get("transcript_path")
if value is None:
    raise SystemExit(0)
if not isinstance(value, str):
    raise SystemExit(1)
print(value, end="")
')" || return 1
  if [ -n "$transcript" ]; then
    tmux set-option -w -t "$1" @gl_session "$transcript" || return 1
  fi
  printf '%s\n' "$value"
}

codex_context_read() { # $1 = rollout path; prints "<used>k/<win>k (<pct>%)"
  python3 -c '
import json, sys
path = sys.argv[1]
info = None
def newest_lines(path):
    with open(path, "rb") as f:
        f.seek(0, 2)
        pos = f.tell()
        carry = b""
        while pos:
            size = min(65536, pos)
            pos -= size
            f.seek(pos)
            parts = (f.read(size) + carry).split(b"\n")
            carry = parts[0]
            yield from reversed(parts[1:])
        if carry:
            yield carry
for raw in newest_lines(path):
        if b"\"token_count\"" not in raw:
            continue
        try:
            rec = json.loads(raw)
        except ValueError:
            continue
        p = rec.get("payload") or {}
        if p.get("type") == "token_count" and p.get("info"):
            info = p["info"]
            break
if info is None:
    print("no token_count event yet — codex reports usage after its first turn",
          file=sys.stderr)
    sys.exit(1)
try:
    used = info["last_token_usage"]["total_tokens"]
    win = info["model_context_window"]
    pct = round(100 * used / win)
except (KeyError, TypeError, ZeroDivisionError) as e:
    print(f"token_count schema drifted ({e!r} in {path}) — "
          "re-verify against the installed codex and update collars/codex.sh",
          file=sys.stderr)
    sys.exit(1)
print(f"{round(used / 1000)}k/{round(win / 1000)}k ({pct}%)")
' "$1" || die "unreadable codex context in $1"
}

# EVIDENCE OF ACTION, WHICH IS NOT EVIDENCE OF HEALTH. Codex records each tool
# call as a response_item whose payload type names the call family. Two families
# are in the rollouts this was built against — function_call and
# custom_tool_call — and a build that adds a third would otherwise make an agent
# that is working look like one that has never acted. So a call-shaped payload
# this does not recognize is reported as unknown by name, never counted and
# never ignored.
codex_action_read() { # $1 = rollout path
  python3 -c '
import datetime, json, sys

BOUND = 2000
KNOWN = {"function_call", "custom_tool_call", "local_shell_call"}

def newest_lines(path):
    with open(path, "rb") as f:
        f.seek(0, 2)
        pos = f.tell()
        carry = b""
        while pos:
            size = min(65536, pos)
            pos -= size
            f.seek(pos)
            parts = (f.read(size) + carry).split(b"\n")
            carry = parts[0]
            yield from reversed(parts[1:])
        if carry:
            yield carry

def epoch(rec):
    stamp = rec.get("timestamp")
    if not isinstance(stamp, str) or not stamp:
        raise ValueError("record carries no readable timestamp")
    text = stamp[:-1] + "+00:00" if stamp.endswith("Z") else stamp
    return int(datetime.datetime.fromisoformat(text).timestamp())

scanned = 0
oldest = None
try:
    for raw in newest_lines(sys.argv[1]):
        if not raw.strip():
            continue
        try:
            rec = json.loads(raw)
        except ValueError:
            continue
        if not isinstance(rec, dict) or rec.get("type") != "response_item":
            continue
        payload = rec.get("payload") or {}
        kind = payload.get("type") if isinstance(payload, dict) else None
        scanned += 1
        if isinstance(kind, str) and kind in KNOWN:
            print(f"at {epoch(rec)}")
            sys.exit(0)
        if isinstance(kind, str) and kind.endswith("_call") and kind not in KNOWN:
            print(f"this codex rollout records a call family gang does not read: {kind}")
            sys.exit(2)
        oldest = rec
        if scanned >= BOUND:
            break
except (OSError, UnicodeError, ValueError, OverflowError):
    print("bound codex rollout is unreadable")
    sys.exit(2)

if scanned >= BOUND and oldest is not None:
    try:
        print(f"before {epoch(oldest)}")
    except ValueError:
        print("bound codex rollout is unreadable")
        sys.exit(2)
    sys.exit(0)
sys.exit(1)
' "$1"
}

collar_last_action() { # $1 target -> "at <epoch>" | "before <epoch>";
                       # 0 printed, 1 = no tool call in the source, 2 = unknown
  local file
  file="$(codex_session_file "$1")" || {
    printf 'no codex rollout is bound to this window yet'
    return 2
  }
  codex_action_read "$file"
}

# A TURN THAT ENDED WITHOUT PRODUCING WORK, STATED BY CODEX RATHER THAN INFERRED.
# Codex closes every turn with exactly one terminator, so "the turn ended" is a
# record and not an absence. What it does not have is a terminator meaning the
# turn ended BADLY: there is no error-typed record in this harness's vocabulary,
# and every turn_aborted observed carries reason "interrupted", which is a person
# pressing Esc — a turn that ended, produced nothing, and leaves a window that
# takes the next turn normally. Keying on the abort would mark every interrupted
# window blocked.
#
# THE CONJUNCTION IS THE SIGNAL, NOT ANY ONE FIELD. An absent last_agent_message
# alone is worthless: turns that ran to dozens of tool calls end with no closing
# message and are perfectly healthy. Blocked is a task_complete carrying no
# last_agent_message AND no time_to_first_token_ms AND a turn body holding
# nothing but the input that opened it. Duration is deliberately NOT part of it:
# the observed population runs from 949ms to 83s, so it separates nothing.
#
# ONLY THE NEWEST TURN COUNTS, which is what carries the second half of the
# claim. A codex turn only ever begins from an input, so a task_started newer
# than a hollow completion is evidence that something was sent, not that the
# window recovered by itself; the newest-terminator rule retires old evidence
# exactly as a newer user turn retires it on the claude-code side.
#
# AN UNFINISHED BRACKET IS ABSENT, AND MUST STAY ABSENT. A turn still running
# and a harness that died mid-turn look identical from here, and nothing in a
# rollout can separate them — a process that dies does not get to record that it
# did. So this reader reports NOTHING for a panicked harness, on purpose. That
# case is a liveness question about the process, not a state question about the
# record, and it is answered elsewhere; it must not be folded in here.
codex_blocked_read() { # $1 = rollout path
  python3 -c '
import json, sys

# Payload types that are the harness doing something. A turn containing any of
# them produced work, whatever it ended with.
# WHAT COUNTS AS THE HARNESS DOING SOMETHING. A turn holding any of these
# produced work, whatever it ended with.
WORK = {
    # response_item payloads
    "reasoning", "custom_tool_call", "custom_tool_call_output",
    "function_call", "function_call_output", "local_shell_call",
    "local_shell_call_output", "web_search_call", "tool_search_call",
    "tool_search_output", "mcp_tool_call_end",
    # event_msg payloads
    "item_completed", "agent_message", "patch_apply_end", "web_search_end",
    "sub_agent_activity", "context_compacted", "thread_goal_updated",
    # record types whose payload carries no type of its own
    "compacted",
}
# The input that opened the turn, and the bookkeeping around it. Neither is the
# harness working. Every name here was checked against the rollouts rather than
# assumed benign for looking harmless, because this is the side of the line a
# mistake is dangerous on: a work record wrongly called bookkeeping turns a turn
# that worked into a blocked window.
BOOKKEEPING = {
    "message", "user_message", "token_count", "thread_settings_applied",
    "turn_context", "world_state", "inter_agent_communication_metadata",
    # Never observed inside a turn body — a second one always lands outside a
    # bracket — but a terminator with no start before it makes the walk meet it
    # on the way to byte zero. Naming it keeps that case reporting the shape it
    # actually is, rather than choking on a record that was never the problem.
    "session_meta",
}


def newest_lines(path):
    with open(path, "rb") as f:
        f.seek(0, 2)
        pos = f.tell()
        carry = b""
        while pos:
            size = min(65536, pos)
            pos -= size
            f.seek(pos)
            parts = (f.read(size) + carry).split(b"\n")
            carry = parts[0]
            yield from reversed(parts[1:])
        if carry:
            yield carry


scanning_body = False
try:
    for raw in newest_lines(sys.argv[1]):
        if not raw.strip():
            continue
        try:
            rec = json.loads(raw)
        except ValueError:
            # An append still in flight is not a record yet. Skipping it can only
            # hide a terminator, which lands on absent rather than on blocked.
            continue
        if not isinstance(rec, dict):
            continue
        payload = rec.get("payload")
        payload = payload if isinstance(payload, dict) else {}
        kind = payload.get("type")
        record = rec.get("type")

        if not scanning_body:
            if kind == "task_started":
                # The newest bracket event opens a turn nothing has closed.
                raise SystemExit(1)
            if kind in ("turn_aborted", "task_aborted"):
                raise SystemExit(1)
            if kind != "task_complete":
                continue
            message = payload.get("last_agent_message")
            first_token = payload.get("time_to_first_token_ms")
            if message or first_token is not None:
                raise SystemExit(1)
            scanning_body = True
            continue

        if kind == "task_started":
            print(
                "the codex turn that took the last input ended without "
                "producing work (no reply, and no first token)"
            )
            raise SystemExit(0)
        if kind in ("task_complete", "turn_aborted", "task_aborted"):
            # Two terminators with no start between them is a shape this reader
            # cannot account for; saying so is the answer, not guessing past it.
            print("bound codex rollout closes a turn that never opened")
            raise SystemExit(2)
        if record == "response_item" and kind == "message":
            if payload.get("role") == "assistant":
                raise SystemExit(1)
            continue
        # A payload carrying no type of its own is named by its record instead,
        # so turn_context and world_state are classified rather than skipped.
        name = kind if isinstance(kind, str) and kind else record
        if name in WORK:
            raise SystemExit(1)
        if name in BOOKKEEPING:
            continue
        # UNKNOWN IS THE DEFAULT, AND THAT IS THE POINT. Falling through to "not
        # work" would let any payload type this reader has never seen turn a turn
        # that worked into a blocked window, which is the one failure direction
        # that matters here. Matching a suffix instead only buys the next name:
        # this vocabulary already carries _call, _call_output, _call_end and bare
        # _output, and _call_output is listed above precisely because a "_call"
        # test does not reach it. So an unclassified name costs an honest
        # unknown, and the rollout corpus becomes a real negative control — after
        # this inversion a new unknown is a name nobody has classified rather
        # than a silent pass.
        print(f"this codex rollout records a turn payload gang does not read: {name}")
        raise SystemExit(2)
except (OSError, UnicodeError, ValueError, OverflowError):
    print("bound codex rollout is unreadable")
    raise SystemExit(2)

# Reaching byte zero inside a body means the turn has no start in this file.
if scanning_body:
    print("bound codex rollout holds no start for its newest turn")
    raise SystemExit(2)
raise SystemExit(1)
' "$1"
}

collar_blocked() { # $1 target; print reason, 0 blocked, 1 absent, 2 unknown
  local file
  file="$(codex_session_file "$1")" || return 1
  codex_blocked_read "$file"
}

collar_context() { # $1 = tmux target; file-based — reads the rollout, never the pane
  local file
  file="$(codex_session_file "$1")" \
    || die "window has no usable @gl_session — Codex context lookup requires a native hook payload with transcript_path"
  codex_context_read "$file"
}

collar_input() { # $1 = tmux target; prints the composer, 1 = no composer,
                 # 3 = a pane that could not be read at all
  local pane line
  # A PANE THAT COULD NOT BE READ IS NOT A PANE WITH NO BOX. The capture is
  # taken into a variable before awk sees it, because awk's verdict on empty
  # input reads exactly like its verdict on a pane carrying no composer.
  pane="$(tmux capture-pane -pJ -e -t "$1")" || return 3
  line="$(printf '%s\n' "$pane" | awk '
    { # A dim run ends at the next escape, whatever closes it — 0m here, but the
      gsub(/\033\[2m[^\033]*/, "")
      gsub(/\033\[[0-9;]*[A-Za-z]/, "")   # the rest of -e: attributes, zero width
      if ($0 ~ /^›/) last = $0
    }
    END { if (!length(last)) exit 1; print last }')" || return 1
  case "$line" in '› '[0-9]*'. '*) return 1 ;; esac
  printf '%s' "${line#›}"
}
