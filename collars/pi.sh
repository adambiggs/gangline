# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_collar via source
# SPDX-License-Identifier: Apache-2.0
GANG_LAUNCH="pi"
GANG_MODEL_OPT="--model"
GANG_BUSY_REGEX="Working\\.\\.\\."
GANG_COMPACT_CMD="/compact"
GANG_MIDTURN_INPUT=1
GANG_OCCUPIED_REGEX='^→ '

collar_context() { # $1 = tmux target; Pi's status bar renders "30.7%/272k" natively
  local m pct win
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo '[0-9]+(\.[0-9]+)?%/[0-9]+k' | tail -1)" \
    || die "no context readout visible in Pi's status bar"
  pct="${m%%\%*}"; win="${m##*/}"; win="${win%k}"
  awk -v p="$pct" -v w="$win" 'BEGIN{printf "%.0fk/%sk (%s%%)\n", p*w/100, w, p}'
}

collar_input() { # $1 = tmux target; prints Pi's input area, fails if it has none
  tmux capture-pane -pJ -t "$1" | awk '
    /^──────────/ { r1 = r2; r2 = NR }
    { line[NR] = $0 }
    END {
      if (!r1) exit 1
      for (i = r1 + 1; i < r2; i++)
        if (line[i] ~ /^→ /) exit 1
      for (i = r1 + 1; i < r2; i++) print line[i]
    }'
}
