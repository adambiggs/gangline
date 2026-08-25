# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_collar via source
# SPDX-License-Identifier: Apache-2.0
[ -z "${name:-}" ] || export OTEL_RESOURCE_ATTRIBUTES="gang.agent=$name${OTEL_RESOURCE_ATTRIBUTES:+,$OTEL_RESOURCE_ATTRIBUTES}"
GANG_LAUNCH="pi"
GANG_MODEL_OPT="--model"
# Observed on Pi 0.80.6: `pi --list-models` prints the six-column table parsed
# below; provider plus model is the native selection id.
collar_models() {
  python3 - <<'PY'
import os
import subprocess

env = os.environ.copy()
env["NO_COLOR"] = "1"
try:
    result = subprocess.run(
        ["pi", "--list-models"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        timeout=10,
        env=env,
    )
except (subprocess.TimeoutExpired, OSError, UnicodeError):
    raise SystemExit(1)
if result.returncode:
    raise SystemExit(result.returncode)
lines = result.stdout.splitlines()
header = ["provider", "model", "context", "max-out", "thinking", "images"]
if not lines or lines[0].split() != header:
    raise SystemExit(1)
rows = [line.split() for line in lines[1:]]
if not rows or any(len(row) != len(header) for row in rows):
    raise SystemExit(1)
print(*(f"{row[0]}/{row[1]}" for row in rows), sep="\n")
PY
}
GANG_BUSY_REGEX="Working\\.\\.\\."
GANG_COMPACT_CMD="/compact"
GANG_MIDTURN_INPUT=1
GANG_OCCUPIED_REGEX='^→ '

collar_context() { # $1 = tmux target; Pi's status bar renders "30.7%/272k" natively
  local pane m pct win
  # A PANE THAT COULD NOT BE READ IS NOT A PANE WITH NO READOUT. The capture is
  # taken into a variable before grep sees it, because grep's verdict on empty
  # input reads exactly like its verdict on a pane carrying no readout, and the
  # refusal that reaches the operator then names the wrong fault.
  pane="$(tmux capture-pane -pJ -t "$1")" \
    || refuse "cannot read pane $1 — whether Pi's status bar is showing a context readout is unknown, not absent"
  m="$(printf '%s\n' "$pane" | grep -Eo '[0-9]+(\.[0-9]+)?%/[0-9]+k' | tail -1)" \
    || m=""
  # EMPTY IS THE MISS, and the miss is checked rather than inherited: without
  # pipefail a grep that found nothing hands its status to tail, the assignment
  # succeeds, and a pane with no readout prints as one with an empty reading.
  [ -n "$m" ] || die "no context readout visible in Pi's status bar"
  pct="${m%%\%*}"; win="${m##*/}"; win="${win%k}"
  awk -v p="$pct" -v w="$win" 'BEGIN{printf "%.0fk/%sk (%s%%)\n", p*w/100, w, p}'
}

collar_input() { # $1 = tmux target; prints Pi's input area, 1 = it has none,
                 # 3 = a pane that could not be read at all
  local pane
  # A PANE THAT COULD NOT BE READ IS NOT A PANE WITH NO BOX. The capture is
  # taken into a variable before awk sees it, because awk's verdict on empty
  # input reads exactly like its verdict on a pane carrying no composer.
  pane="$(tmux capture-pane -pJ -t "$1")" || return 3
  printf '%s\n' "$pane" | awk '
    /^──────────/ { r1 = r2; r2 = NR }
    { line[NR] = $0 }
    END {
      if (!r1) exit 1
      for (i = r1 + 1; i < r2; i++)
        if (line[i] ~ /^→ /) exit 1
      for (i = r1 + 1; i < r2; i++) print line[i]
    }'
}
