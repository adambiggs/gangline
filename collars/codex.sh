# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_collar via source
# SPDX-License-Identifier: Apache-2.0
_gl_codex_hook="[{ hooks = [{ type = \"command\", command = \"\\\"$ROOT/bin/gang\\\" hook\" }] }]"
_gl_codex_hook_flags=""
for _gl_codex_event in UserPromptSubmit PostToolUse Stop PermissionRequest; do
  _gl_codex_hook_flags+=" -c 'hooks.$_gl_codex_event=$_gl_codex_hook'"
done
GANG_LAUNCH="codex -c check_for_update_on_startup=false$_gl_codex_hook_flags"
GANG_RESUME_LAUNCH="codex resume {{session_id}} -c check_for_update_on_startup=false$_gl_codex_hook_flags"
unset _gl_codex_hook _gl_codex_hook_flags _gl_codex_event
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
GANG_COMPACT_CMD="/compact"
# Verified on codex 0.145.0: /usage opens a selection menu with "Show usage"
# preselected; one Enter confirms it and the content is appended to the
# transcript with the composer already restored, so nothing dismisses it.
GANG_USAGE_CMD="/usage"
GANG_USAGE_CONFIRM_KEY="Enter"
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
GANG_SELF_COMPACT=deferred
GANG_MIDTURN_INPUT=1
GANG_SESSION_KEY=1
# Escape stops an active turn; the busy marker above is the harness's own
# "esc to interrupt" footer.
GANG_INTERRUPT_KEY="Escape"
# The launch above passes a native Stop hook with -c, so this harness announces
# its own turn boundaries to gang — which is what a spool needs to drain, and
# what deferred self-compaction already relies on.
GANG_STOP_HOOK=1
# Verified on codex 0.145.0: the native hook set contains no Notification
# event. legacy_notify / agent-turn-complete reports turn completion, which the
# Stop hook above already delivers; it is not an awaiting-input witness and is
# deliberately not wired. PermissionRequest is this collar's only stall source.

# Verified against codex 0.145.0 from the live capture cited in the landing
# commit. Numeric prefixes move with the selection and are normalized by core;
# all three option labels and every explanatory line remain fingerprint bytes.
GANG_DIALOGS='safety-buffering-prompt|^› [0-9]+\. |Dismiss and keep waiting|Down|Enter'
GANG_DIALOG_LINES_safety_buffering_prompt='Our systems are thinking a bit more about this request before responding.
Hang tight or retry with a faster model for a quicker response, though it may be less capable of handling complex requests.
Retry with a faster model
Dismiss and keep waiting
Learn more
No action is required. Codex will keep waiting, and this menu will close when the response is ready.'

codex_sessions_dir() { printf '%s/sessions' "${CODEX_HOME:-$HOME/.codex}"; }

codex_session_for() { # $1 = marker -> the one rollout that recorded it as user input
  local dir hits
  dir="$(codex_sessions_dir)"
  [ -d "$dir" ] || die "no codex sessions tree at $dir"
  hits="$(grep -rlF -- "$1" "$dir" 2>/dev/null)" \
    || die "marker not in any rollout under $dir — the hitch message may not be flushed yet"
  printf '%s\n' "$hits" | python3 -c '
import json, sys
marker = sys.argv[1]
mine = []
for path in (l.strip() for l in sys.stdin if l.strip()):
    with open(path, encoding="utf-8", errors="replace") as f:
        for line in f:
            if marker not in line:
                continue
            try:
                rec = json.loads(line)
            except ValueError:
                continue
            p = rec.get("payload") or {}
            if p.get("type") == "message" and p.get("role") == "user":
                mine.append(path)
                break
if len(mine) != 1:
    n = len(mine)
    print(f"marker matches {n} rollouts as user input — "
          + ("not flushed yet, or the marker line never landed" if n == 0
             else "the marker was repeated; re-hitch the agent: " + " ".join(mine)),
          file=sys.stderr)
    sys.exit(1)
print(mine[0])
' "$1" || die "cannot bind this agent to a codex rollout"
}

codex_session_file() { # $1 = tmux target -> this window's bound rollout path
  local key file
  key="$(tmux show-options -wqv -t "$1" @gl_key)"
  [ -n "$key" ] || return 1
  file="$(tmux show-options -wqv -t "$1" @gl_session)"
  if [ -z "$file" ] || [ ! -f "$file" ]; then
    file="$(codex_session_for "$key")" || return 1
    tmux set-option -w -t "$1" @gl_session "$file"
  fi
  printf '%s' "$file"
}

collar_session_id() { # $1 = tmux target; rollout metadata is the native id contract
  local file
  file="$(codex_session_file "$1")" || return 1
  python3 -c '
import json, sys
with open(sys.argv[1], encoding="utf-8", errors="replace") as stream:
    record = json.loads(next(stream))
payload = record.get("payload") or {}
value = payload.get("id") or payload.get("session_id")
if record.get("type") != "session_meta" or not isinstance(value, str) or not value:
    raise SystemExit(1)
print(value)
' "$file"
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
    || die "window has no usable @gl_key — Codex context lookup requires a hitch-time startup-envelope nonce; adopted windows have none"
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
