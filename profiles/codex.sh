# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
_gl_codex_hook="[{ hooks = [{ type = \"command\", command = \"\\\"$ROOT/bin/gang\\\" hook\" }] }]"
_gl_codex_hook_flags=""
for _gl_codex_event in UserPromptSubmit PostToolUse Stop PermissionRequest; do
  _gl_codex_hook_flags+=" -c 'hooks.$_gl_codex_event=$_gl_codex_hook'"
done
GANG_LAUNCH="codex -c check_for_update_on_startup=false$_gl_codex_hook_flags"
GANG_RESUME_LAUNCH="codex resume --last -c check_for_update_on_startup=false$_gl_codex_hook_flags"
unset _gl_codex_hook _gl_codex_hook_flags _gl_codex_event
GANG_MODEL_OPT="-m"
GANG_BUSY_REGEX="esc to interrupt"
GANG_QUIET_AT_REST=1
GANG_OCCUPIED_REGEX='^› [0-9]+\. '
GANG_COMPACT_CMD="/compact"
GANG_SELF_COMPACT=deferred
GANG_MIDTURN_INPUT=1
GANG_SESSION_KEY=1

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

codex_context_read() { # $1 = rollout path; prints "<used>k/<win>k (<pct>%)"
  python3 -c '
import json, sys
path = sys.argv[1]
info = None
with open(path, encoding="utf-8", errors="replace") as f:
    for line in f:
        if "\"token_count\"" not in line:
            continue
        try:
            rec = json.loads(line)
        except ValueError:
            continue
        p = rec.get("payload") or {}
        if p.get("type") == "token_count" and p.get("info"):
            info = p["info"]
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
          "re-verify against the installed codex and update profiles/codex.sh",
          file=sys.stderr)
    sys.exit(1)
print(f"{round(used / 1000)}k/{round(win / 1000)}k ({pct}%)")
' "$1" || die "unreadable codex context in $1"
}

profile_context() { # $1 = tmux target; file-based — reads the rollout, never the pane
  local key file
  key="$(tmux show-options -wqv -t "$1" @gl_key)"
  [ -n "$key" ] \
    || die "window has no @gl_key — Codex context lights require a hitch-time startup-envelope nonce; adopted windows have none"
  file="$(tmux show-options -wqv -t "$1" @gl_session)"
  if [ -z "$file" ] || [ ! -f "$file" ]; then
    file="$(codex_session_for "$key")"
    tmux set-option -w -t "$1" @gl_session "$file"
  fi
  codex_context_read "$file"
}

profile_input() { # $1 = tmux target; prints the composer, fails if there is none
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
