# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_collar via source
# SPDX-License-Identifier: Apache-2.0
GANG_LAUNCH="OPENCODE_DISABLE_AUTOUPDATE=1 opencode"
GANG_MODEL_OPT="-m"
# Observed on OpenCode 1.14.41: `opencode models` prints one bare
# provider/model id per line with no header.
collar_models() {
  python3 - <<'PY'
import subprocess

try:
    result = subprocess.run(
        ["opencode", "models"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        timeout=10,
    )
except (subprocess.TimeoutExpired, OSError, UnicodeError):
    raise SystemExit(1)
if result.returncode:
    raise SystemExit(result.returncode)
rows = result.stdout.splitlines()
if not rows or any(not row or row != row.strip() or any(c.isspace() for c in row) for row in rows):
    raise SystemExit(1)
print(*rows, sep="\n")
PY
}
GANG_BUSY_REGEX="esc interrupt"
GANG_QUIET_AT_REST=1
GANG_COMPACT_CMD="/compact"
GANG_MIDTURN_INPUT=1
GANG_OCCUPIED_REGEX='△ Permission required| {2,}esc *$'

opencode_models_json() { printf '%s/opencode/models.json' "${XDG_CACHE_HOME:-$HOME/.cache}"; }

collar_context() { # $1 = tmux target; hint row carries used+percent, catalog carries the window
  local cap row badge names
  cap="$(tmux capture-pane -pJ -t "$1")" || die "cannot read pane $1"
  row="$(printf '%s\n' "$cap" | grep -Eo '[0-9]+(\.[0-9]+)?[KM]? \([0-9]+%\)' | tail -1)" \
    || die "no context readout in the hint row — opencode paints it after the agent's first turn, and a narrow pane clips it mid-token (a clipped readout loses its closing paren, so it can never half-match)"
  badge="$(printf '%s\n' "$cap" | awk '/^[[:space:]]*╹▀/ { b = p } { p = $0 } END { print b }')"
  case "$badge" in
    *' · '*) names="${badge##*' · '}"; names="${names%%  *}"
             names="${names%"${names##*[![:space:]]}"}" ;;
    *) die "no agent·model badge above the composer border — opencode may still be booting" ;;
  esac
  python3 - "$(opencode_models_json)" "$names" "${row%% *}" "${row##*\(}" <<'PY' || die "cannot bind pane $1 to a context window"
import json, math, sys
path, names, used_s, pct_s = sys.argv[1:5]
try:
    cat = json.load(open(path, encoding="utf-8"))
except (OSError, ValueError) as e:
    print(f"models catalog unreadable at {path} ({e}) — opencode writes it at startup",
          file=sys.stderr)
    sys.exit(1)
unit = {"K": 1000, "M": 1000000}.get(used_s[-1], 1)
num = used_s.rstrip("KM")
used = round(float(num) * unit)
step = unit / (10 ** (len(num.split(".")[1]) if "." in num else 0))
pct = int(pct_s.rstrip("%)"))
wins = set()
for pid, p in cat.items():
    if not isinstance(p, dict):
        continue
    pname = p.get("name") or pid
    for mid, m in (p.get("models") or {}).items():
        if f"{m.get('name') or mid} {pname}" == names:
            ctx = (m.get("limit") or {}).get("context")
            if isinstance(ctx, int) and ctx > 0:
                wins.add(ctx)
if len(wins) != 1:
    n = len(wins)
    print(f"badge '{names}' matches {n} catalog windows — "
          + ("catalog or badge drifted; re-verify against the installed opencode"
             if n == 0 else "ambiguous join; refusing to guess"),
          file=sys.stderr)
    sys.exit(1)
win = wins.pop()
tol = math.ceil(step / 2 / win * 100) + 1
if abs(round(used / win * 100) - pct) > tol:
    print(f"painted {pct}% does not reproduce from {used}/{win} — wrong window joined "
          "(model switched under the badge, or catalog drift); re-verify", file=sys.stderr)
    sys.exit(1)
print(f"{round(used / 1000)}k/{round(win / 1000)}k ({pct}%)")
PY
}

collar_input() { # $1 = tmux target; prints the composer, fails if it has no keyboard
  local cap cur
  cap="$(tmux capture-pane -pJ -t "$1")" || return 1
  cur="$(tmux display-message -p -t "$1" '#{cursor_y}')" || return 1
  printf '%s\n' "$cap" | awk -v cur="$cur" '
    { line[NR] = $0 }
    /^[[:space:]]*╹▀/ { border = NR }
    END {
      if (!border) exit 1
      top = border - 1
      while (top > 1 && line[top - 1] ~ /^[[:space:]]*┃/) top--
      if (top > border - 2) exit 1
      if (cur + 1 < top || cur + 1 > border - 2) exit 1
      for (i = top; i <= border - 2; i++) {
        s = line[i]
        if (s ~ /┃[[:space:]]*$/) continue   # dropdown row, boxed both sides
        sub(/^[[:space:]]*┃ ?/, "", s)
        print s
      }
    }'
}
