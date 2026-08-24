# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_collar via source
# SPDX-License-Identifier: Apache-2.0
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
      _gl_codex_hook_flags=""
      # PreCompact/PostCompact are wired for the same reason claude-code wires
      # them: @gl_turn is closed for the whole of a compaction, and the turn
      # witness outranks the pane, so without the bracket a compacting codex
      # agent reads IDLE and gang delivers into it. Codex declares no queue
      # evidence, so that delivery would be reported submitted when the harness
      # had parked it. Verified firing on 0.146.0.
      for _gl_codex_event in UserPromptSubmit PostToolUse Stop PermissionRequest \
                             PreCompact PostCompact; do
        _gl_codex_hook_flags+=" -c 'hooks.$_gl_codex_event=$_gl_codex_hook'"
      done
      GANG_LAUNCH="$GANG_LAUNCH$_gl_codex_hook_flags"
      GANG_RESUME_LAUNCH="$GANG_RESUME_LAUNCH$_gl_codex_hook_flags"
      # THE HOOKS INSTALLED ABOVE ARE WHAT CODEX ASKS ABOUT. Their command
      # carries this install root, so a new install, an upgrade or a worktree
      # presents hashes codex has never seen and it opens its hooks-review menu
      # before drawing a composer. Nothing in Gangline can answer that menu, so
      # an unattended hitch waits on a person indefinitely. The preflight runs
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
      unset _gl_codex_hook _gl_codex_hook_flags _gl_codex_event
      ;;
  esac
fi
unset _gl_codex_dir
GANG_MODEL_OPT="-m"
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
# at all: no spool drain, no deferred self-compaction, gang waiting on turn
# boundaries that never arrive.
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
