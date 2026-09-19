# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_collar via source
# SPDX-License-Identifier: Apache-2.0
[ -z "${name:-}" ] || export OTEL_RESOURCE_ATTRIBUTES="gang.agent=$name${OTEL_RESOURCE_ATTRIBUTES:+,$OTEL_RESOURCE_ATTRIBUTES}"
GANG_LAUNCH="codex -c check_for_update_on_startup=false"
GANG_RESUME_LAUNCH="codex resume {{session_id}} -c check_for_update_on_startup=false -c 'tui.resume_cwd=\"current\"'"
# A HOSTILE ROOT IS DECLINED, NOT ESCAPED. The hook TOML rides inside a
# single-quoted -c word, so one quote in the install path closes that word and
# the remainder of the path is shell code the new window runs under the
# operator's account. There is no escaping that survives both the TOML string
# and the shell word around it, so this collar does what its sibling
# claude-code.sh does: for a root bearing a quote, a backslash or a control
# character it installs no hooks at all. A hookless launch loses turn-boundary
# events; it does not execute a directory name.
if [ -n "${ROOT:-}" ] && [ -x "$ROOT/bin/gang" ]; then
  case "$ROOT" in
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
      # in the hitched window before drawing a composer. The operator answers it
      # there; no Gangline path presses a trust choice on their behalf.
      # The launch above passes a native Stop hook with -c, so this harness
      # announces its own turn boundaries to gang — which is what a spool needs
      # to drain, and what deferred self-compaction already relies on. Both are
      # declared here, beside the hooks that deliver them, so the hookless
      # launch above claims neither.
      #
      # TRUST IS KEYED TO THE INSTALL ROOT. Codex records hook trust against
      # the command string, which carries the path of the checkout gang runs
      # from, and keeps one entry per event. A gang installed at a second root
      # therefore prompts again and replaces the stored entry rather than
      # adding to it, so the first root's next Codex hitch prompts in turn.
      GANG_STOP_HOOK=1
      GANG_SELF_COMPACT=deferred
      # THE COMPOSER IS ONLY PAINT AT STOP. Codex runs its Stop hooks inside
      # the task that owns the turn: the composer already paints idle while
      # the hook runs, and an Enter typed there is dropped without a trace.
      # The task emits task_complete only after every Stop hook has returned,
      # and the rollout recorder appends that record to the session file at
      # once (observed on 0.151.0: on disk 3ms after the hook exited, and a
      # slash command entered after it ran natively). That persisted record
      # for the Stop payload's turn is the positive post-Stop witness;
      # collar_native_idle below reads it. Between the record and the release
      # of active_turn a submission starts a new turn rather than steering the
      # finished one, which is where Codex's own TUI submits its queued input.
      GANG_SELF_COMPACT_WITNESS=native-idle
      unset _gl_codex_hook _gl_codex_hook_flags _gl_codex_event
      ;;
  esac
fi
GANG_HARNESS_PROMPT="When an exec call yields a running session, keep its session id and do not re-poll it on a timer. Do independent work first, and continue the session only when its result is needed. A yield is not completion evidence.

Do not detach a long command expecting this Gangline window to wake itself. Plain background children do not survive Codex's exec boundary here, the sandbox may not reach the user service manager, and Gangline refuses self-addressed send and at messages."
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
# SELF-CLOSING ADVISORIES IN CODEX 0.151.0, enumerated from the TUI sources:
#
# - safety_buffering's retry form offers Retry with a faster model, Dismiss and
#   keep waiting, and Learn more. A cooperative tick chooses option 2 below:
#   it preserves the requested model, and this is the exact form Gangline dismisses.
# - safety_buffering without a faster retry offers Dismiss and keep waiting as
#   option 1 and Learn more as option 2. It remains advisory and closes when
#   the response arrives, but gang spends neither key: this is not the
#   retry-capable form, and the shared numbered shape is not authority to infer it.
# - rate_limits' approaching-limit prompt can close when workspace credits
#   become usable. Its choices switch model policy or keep the current choice,
#   so gang leaves it occupied and sends no key.
#
# Connector loading, request_user_input, and app-link views also disappear on
# external state changes, but they are functional work surfaces rather than
# advisory menus. The occupancy regex remains broad enough to protect all
# numbered surfaces; only the full text match below authorizes a keystroke.
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

# Codex carries the exact submitted composer body in UserPromptSubmit. Gangline
# compares it with the continuation it marked before Enter.
collar_submitted_prompt() { # $1 target unused, $2 UserPromptSubmit payload
  printf '%s' "$2" | python3 -c '
import json, sys
value = json.load(sys.stdin).get("prompt")
if not isinstance(value, str) or not value:
    raise SystemExit(2)
print(value, end="")
'
}

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
# it always reported. The separate dismissal reader below recognizes only the
# retry-capable three-choice form.
collar_advisory() { # $1 = tmux target; 0 + what it is, 1 = not an advisory surface
  local flat
  # READ IT ACROSS THE WRAP. Codex breaks its own prose to the pane width, and
  # those are hard line breaks rather than the soft wrap `capture-pane -J`
  # rejoins, so the sentence arrives in as many pieces as the pane is narrow.
  # Collapsing whitespace is what makes the reading independent of width.
  flat="$(tmux capture-pane -pJ -t "$1" 2>/dev/null | tr -s '[:space:]' ' ')" \
    || return 1
  case "$flat" in
    *'No action is required. Codex will keep waiting, and this menu will close when the response is ready.'*) ;;
    *) return 1 ;;
  esac
  printf 'Codex is waiting on its provider and says the menu closes by itself when the response is ready'
}

# TEXT ON THE ACTIVE SELECTED ROW AUTHORIZES THE KEY, NOT TEXT ELSEWHERE IN THE
# PANE. Start at the last row beginning with Codex's selected-menu marker,
# normalize its hard-wrapped continuation, and require every action label plus
# the self-closing footer. The footer must also be the pane's final nonblank
# content. That bottom anchor remains valid when the current view has no
# numbered marker at all, and excludes matching menu text in scrollback.
#
# The non-search selection view accepts a bare digit as an immediate shortcut,
# so option 2 needs no Enter and cannot accidentally submit a second action.
# Anchoring option 1 also excludes the no-retry variant where Dismiss is first.
collar_dismiss_advisory() { # $1 target; 0 + action, 1 no match, 2 failure + cause
  local pane
  pane="$(tmux capture-pane -pJ -t "$1" 2>/dev/null)" || return 1
  printf '%s\n' "$pane" | python3 -c '
import re
import sys

lines = sys.stdin.read().splitlines()
selected = [index for index, line in enumerate(lines) if re.match(r"^› [0-9]+\. ", line)]
if not selected:
    raise SystemExit(1)
surface = " ".join(" ".join(lines[selected[-1] :]).split())
parts = (
    "› 1. Retry with a faster model",
    "2. Dismiss and keep waiting",
    "3. Learn more",
    "No action is required. Codex will keep waiting, and this menu will close when the response is ready.",
)
if not surface.endswith(parts[-1]):
    raise SystemExit(1)
offset = 0
for index, part in enumerate(parts):
    found = surface.find(part, offset)
    if found < 0 or (index == 0 and found != 0):
        raise SystemExit(1)
    offset = found + len(part)
' || return 1
  tmux send-keys -t "$1" 2 || {
    printf 'Codex provider wait menu matched, but option 2 could not be sent'
    return 2
  }
  printf "dismissed Codex's provider wait menu with option 2"
}

# READ THE PANE PLAIN, NEVER THE DIM-STRIPPED READING. Codex draws both the
# queue header and the recall advertisement inside ANSI dim runs, and the awk
# in collar_input deletes a dim run wholesale — so the reading that serves the
# composer would drop the advertisement entirely and leave this function
# unable to tell a recognized queue from a rendering it no longer understands.
# The capture below is therefore plain `-pJ`, with no `-e` and no stripping.
collar_queued() { # $1 tmux target, $2 exact body evidence (optional).
                  # Without a body: 0 the harness holds parked input, 1 it
                  # does not, 2 unknown with a cause.
                  # With evidence: 0 that exact text is parked, 2 not confirmed.
                  # The evidence form never returns 1: text gang cannot find in
                  # the queue block is a reading this collar could not make,
                  # not proof the message entered the session.
  local pane
  pane="$(tmux capture-pane -pJ -t "$1" 2>/dev/null)" || {
    printf 'the Codex pane could not be read for parked-queue evidence'
    return 2
  }
  printf '%s\n' "$pane" | grep -qE '^• Queued follow-up inputs' || {
    [ $# -lt 2 ] || {
      printf 'the Codex follow-up queue is not on screen, so gang cannot confirm its delivery evidence is parked'
      return 2
    }
    return 1
  }
  ! printf '%s' "$pane" | tr -s '[:space:]' ' ' \
    | grep -qF 'shift + ← edit last queued message' || {
    [ $# -ge 2 ] || return 0
    printf '%s\n' "$pane" | python3 -c '
import re
import sys

want = " ".join(sys.argv[1].split())
lines = sys.stdin.read().split("\n")
header = re.compile(r"^\u2022 Queued follow-up inputs")
# The advertisement can wrap; its opening words cannot, and they are what
# closes the block.
hint = re.compile(r"^ *shift \+ \u2190")
opened = [i for i, line in enumerate(lines) if header.match(line)]
closed = [i for i, line in enumerate(lines) if hint.match(line)]
if not want or not opened or not closed or closed[-1] <= opened[0]:
    raise SystemExit(1)
block = " ".join(lines[opened[0] + 1 : closed[-1]]).replace("\u21b3", " ")
raise SystemExit(0 if want in " ".join(block.split()) else 1)
' "$2" && return 0
    printf 'the Codex follow-up queue is on screen but does not contain the exact delivery evidence supplied by gang'
    return 2
  }
  printf 'the Codex follow-up queue is on screen but no longer advertises the shift+Left recall this collar sends'
  return 2
}
# ENUMERATED ON CODEX 0.151.0. The exact generated app-server schema's
# HookEventName enum and the public hook documentation list the native hook
# events; neither contains Notification. legacy_notify / agent-turn-complete
# reports turn completion, which the Stop hook above already delivers; it is
# not an awaiting-input witness and is deliberately not wired.
# PermissionRequest is this collar's only stall source.

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

collar_cache_stamp() { # $1 target -> epoch of the last rollout write
  local file
  file="$(codex_session_file "$1")" || return 1
  stat -c %Y -- "$file"
}

# THE POST-STOP WITNESS IS THE ROLLOUT, NOT THE SCREEN. Answers 0 when the
# bound rollout holds a terminal record (task_complete or turn_aborted) for
# the turn the Stop payload names, 1 while that turn's newest record is still
# its start or a later turn has begun, and 2 when the rollout cannot answer:
# no rollout is bound, the file is unreadable, or the turn is unknown to it.
# Without a payload (a cooperative tick) the newest turn in the rollout is the
# one asked about. Prints the reason on stdout for the caller's diagnostics.
collar_native_idle() { # $1 = tmux target, $2 = native Stop payload or empty
  local rollout="" turn="" fields
  if [ -n "$2" ]; then
    fields="$(printf '%s' "$2" | python3 -c '
import json, sys
try:
    payload = json.load(sys.stdin)
except ValueError:
    raise SystemExit(1)
if not isinstance(payload, dict):
    raise SystemExit(1)
turn = payload.get("turn_id")
transcript = payload.get("transcript_path")
print(turn if isinstance(turn, str) else "")
print(transcript if isinstance(transcript, str) else "")
')" || { printf '%s' "the Stop payload is not readable JSON"; return 2; }
    turn="${fields%%$'\n'*}"
    rollout="${fields#*$'\n'}"
  fi
  case "$turn$rollout" in
    *[[:cntrl:]]*) printf '%s' "the Stop payload carries control characters"; return 2 ;;
  esac
  [ -n "$rollout" ] || rollout="$(codex_session_file "$1")" || rollout=""
  [ -n "$rollout" ] || { printf '%s' "no rollout is bound to this window"; return 2; }
  python3 "${BASH_SOURCE[0]%/*}/plugins/codex-native-idle.py" "$rollout" ${turn:+"$turn"}
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

collar_session_id() { # $1 = tmux target, $2 = native hook payload
  local fields value transcript
  # One parse reads both fields: the id on the first line, then the transcript
  # path (empty when the payload carries none) as the rest.
  fields="$(printf '%s' "$2" | python3 -c '
import json, sys
row = json.load(sys.stdin)
value = row.get("session_id", "")
if not isinstance(value, str) or not value or "\n" in value:
    raise SystemExit(1)
transcript = row.get("transcript_path")
if transcript is None:
    transcript = ""
if not isinstance(transcript, str):
    raise SystemExit(1)
sys.stdout.write(value + "\n" + transcript)
')" || return 1
  value="${fields%%$'\n'*}" transcript=""
  case "$fields" in *$'\n'*) transcript="${fields#*$'\n'}" ;; esac
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
            # A COMPLETION CARRYING AN ERROR ENDED WITHOUT ITS RESULT. The
            # provider content refusal is one: the pane shows a card over an
            # idle prompt, and a first token has usually streamed already, so
            # the conjunction below never sees it. Measured on codex-cli
            # 0.146.0 through 0.151.0, no errored completion carries a reply;
            # the errors were cyber_policy, server_overloaded, unauthorized
            # and other.
            error = payload.get("error")
            if isinstance(error, dict):
                info = error.get("codex_error_info")
                info = info if isinstance(info, str) and info else "unnamed"
                detail = error.get("message")
                detail = " ".join(detail.split())[:200] if isinstance(detail, str) else ""
                print(
                    "the codex turn that took the last input ended on a "
                    f"provider error ({info})" + (f": {detail}" if detail else "")
                )
                raise SystemExit(0)
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
  line="${line#›}"
  # `› ` is Codex's prompt chrome.  The reader promises composer bytes, so
  # retain a second blank as a draft byte but not this separator.
  line="${line# }"
  printf '%s' "$line"
}

# A NATIVE CONTEXT COMPACTION RETURNS TO THIS CURRENT-SCREEN FRAME with an
# empty composer. The history itself is not enough: an older recap can remain
# in scrollback, so require the final visible Codex prompt to be empty through
# collar_input and require the last recap heading to sit above it in the same
# capture. Numbered prompts are excluded by collar_input, which makes a hooks
# review or any other selected menu a non-match rather than a continuation
# target. This reader names a boundary only; bin/gang owns its one continuation.
collar_recap_boundary() { # $1 = tmux target; 0 current empty recap, 1 otherwise, 3 unreadable
  local box pane
  box="$(collar_input "$1")" || return $?
  if grep -q '[^[:space:]]' <<<"$box"; then
    return 1
  fi
  pane="$(tmux capture-pane -pJ -e -t "$1")" || return 3
  printf '%s\n' "$pane" | python3 -c '
import re
import sys

lines = sys.stdin.read().splitlines()
strip = re.compile(r"\x1b\[[0-9;]*[A-Za-z]")
plain = [strip.sub("", line).rstrip() for line in lines]
heads = [i for i, line in enumerate(plain) if line == "─ Conversation recap ─"]
prompts = [i for i, line in enumerate(plain) if line.startswith("›")]
if not heads or not prompts:
    raise SystemExit(1)
head = heads[-1]
prompt = prompts[-1]
if head >= prompt:
    raise SystemExit(1)
# A later transcript row is a newer screen state, not the recap landing. The
# Codex footer below a current composer names a model and cwd; it is allowed,
# but a second assistant row or another heading is not.
for line in plain[prompt + 1:]:
    if not line.strip():
        continue
    if " · " in line:
        continue
    raise SystemExit(1)
' || return 1
}
