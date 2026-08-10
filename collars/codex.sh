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
if [ -n "${ROOT:-}" ] && [ -x "$ROOT/bin/gang" ]; then
  case "$ROOT" in
    *[\'\"\\]*|*[[:cntrl:]]*) ;;
    *)
      _gl_codex_hook="[{ hooks = [{ type = \"command\", command = \"\\\"$ROOT/bin/gang\\\" hook\" }] }]"
      _gl_codex_hook_flags=""
      for _gl_codex_event in UserPromptSubmit PostToolUse Stop PermissionRequest; do
        _gl_codex_hook_flags+=" -c 'hooks.$_gl_codex_event=$_gl_codex_hook'"
      done
      GANG_LAUNCH="$GANG_LAUNCH$_gl_codex_hook_flags"
      GANG_RESUME_LAUNCH="$GANG_RESUME_LAUNCH$_gl_codex_hook_flags"
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
GANG_MODEL_OPT="-m"
# REASONING EFFORT IS MODEL-SCOPED. The option includes its separator because
# bin/gang joins it to the level with no space. Unquoted `high` reached
# turn_context.payload.effort as "high"; an unquoted invented value reached the
# provider and was refused there, so the raw-string fallback documented by
# `codex --help` is observed rather than inferred.
GANG_EFFORT_OPT="-c model_reasoning_effort="
# `codex debug models` is the harness's own live vocabulary. GANG_MODEL is the
# exact model hitch is about to pass with -m; an empty model, an alias the catalog
# cannot bind, a failed or wedged command (bounded by the timeout), malformed
# JSON, a missing field, a duplicate, or whitespace inside a level all produce
# NOTHING. The reader names that as a
# broken GANG_EFFORT_CMD rather than blaming the operator's level. Validate the
# whole catalog row before printing so a plausible prefix can never escape from
# an answer the parser could not finish. The final `|| true` is part of that
# protocol: bin/gang distinguishes failure from a bad level by EMPTY OUTPUT.
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
if not model or not isinstance(models, list):
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
# The marker was authored where we always look, so it fits what we always see —
# any screen a fresh codex draws before a session exists is outside its reach.
GANG_COMPACT_CMD="/compact"
# Verified on codex 0.146.0: /usage opens a selection menu with "Show usage"
# preselected; one Enter confirms it and the content is appended to the
# transcript with the composer already restored, so nothing dismisses it.
GANG_USAGE_CMD="/usage"
GANG_USAGE_CONFIRM_KEY="Enter"
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
GANG_MIDTURN_INPUT=1
# Escape stops an active turn; the busy marker above is the harness's own
# "esc to interrupt" footer.
GANG_INTERRUPT_KEY="Escape"
# Verified on codex 0.145.0: the native hook set contains no Notification
# event. legacy_notify / agent-turn-complete reports turn completion, which the
# Stop hook above already delivers; it is not an awaiting-input witness and is
# deliberately not wired. PermissionRequest is this collar's only stall source.
# UNKNOWN on 0.146.0: that build offers no live enumeration of hook events —
# `codex debug` exposes models, app-server and prompt-input and none of them
# lists one — so re-verifying this means probing event names one at a time to
# prove a negative. Left unproven rather than repinned.

# HOOKS ARE TRUSTED, NOT MERELY INSTALLED (observed on codex 0.146.0). Trust is
# persisted per hook in config.toml's [hooks.state] table under a hash of the
# hook COMMAND, which embeds $ROOT. A checkout whose command has been trusted
# once boots straight to its composer; a fresh clone, a moved checkout, a
# container, or any edit to the command above meets this first:
#
#     Hooks need review
#     4 hooks are new or changed.
#     Hooks can run outside the sandbox after you trust them.
#   › 1. Review hooks
#     2. Trust all and continue
#     3. Continue without trusting (hooks won't run)
#
# It is deliberately NOT in GANG_DIALOGS. That registry answers dialogs, and
# bin/gang screens every record in it for authority language — this block trips
# that screen on "trust" and "sandbox", which is the screen working, not
# failing. A collar that pressed "Trust all and continue" would be granting the
# operator's approval to run hooks outside the sandbox on their behalf, and
# option 3 is a keystroke away from a codex agent with no Stop hook at all: no
# spool drain, no deferred self-compaction, gang waiting on turn boundaries
# that never arrive.
#
# What gang does today: GANG_OCCUPIED_REGEX matches this menu, so the window
# reads !occupied! (authority unknown), delivery is refused and hitch fails.
# Safe, and unhelpful — the operator sees a state word rather than the one-line
# instruction, which is to run codex once in this checkout and choose "Review
# hooks". Naming it needs a registry that declares no keystroke; there is none
# yet.

# Verified against codex 0.145.0 from the live capture cited in the landing
# commit. Numeric prefixes move with the selection and are normalized by core;
# all three option labels and every explanatory line remain fingerprint bytes.
# The safety-buffering prompt is PERMANENTLY UNVERIFIABLE ON DEMAND: it appears
# when the provider is slow to start responding, and nothing here can induce
# that. Its pin will therefore never advance on evidence, and saying so is
# honest where a version implying a re-check nobody schedules would not be.
GANG_DIALOGS='safety-buffering-prompt|^› [0-9]+\. |Dismiss and keep waiting|Down|Enter
directory-trust-prompt|^› [0-9]+\. |Yes, continue||Enter'
GANG_DIALOG_HITCH_DIR_TRUST=directory-trust-prompt
GANG_DIALOG_LINES_safety_buffering_prompt='Our systems are thinking a bit more about this request before responding.
Hang tight or retry with a faster model for a quicker response, though it may be less capable of handling complex requests.
Retry with a faster model
Dismiss and keep waiting
Learn more
No action is required. Codex will keep waiting, and this menu will close when the response is ready.'
# Verified against codex 0.145.0 from the live first-run capture cited in the
# landing commit. The cwd-bearing first line is variable and excluded; -J joins
# the question's visual wraps before the stable block is matched.
GANG_DIALOG_LINES_directory_trust_prompt='Do you trust the contents of this directory? Working with untrusted contents comes with higher risk of prompt injection. Trusting the directory allows project-local config, hooks, and exec policies to load.
Yes, continue
No, quit
Press enter to continue'

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

collar_input() { # $1 = tmux target; prints the composer, fails if there is none
  local line
  line="$(tmux capture-pane -pJ -e -t "$1" | awk '
    { # A dim run ends at the next escape, whatever closes it — 0m here, but the
      gsub(/\033\[2m[^\033]*/, "")
      gsub(/\033\[[0-9;]*[A-Za-z]/, "")   # the rest of -e: attributes, zero width
      if ($0 ~ /^›/) last = $0
    }
    END { if (!length(last)) exit 1; print last }')" || return 1
  case "$line" in '› '[0-9]*'. '*) return 1 ;; esac
  printf '%s' "${line#›}"
}
