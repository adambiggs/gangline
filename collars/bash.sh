# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_collar via source
# SPDX-License-Identifier: Apache-2.0
GANG_LAUNCH="PS1='❯ ' bash --norc"
GANG_BUSY_REGEX=""

collar_input() { # $1 = tmux target; same shape as a real TUI's input box
  local pane line
  # A PANE THAT COULD NOT BE READ IS NOT A PANE WITH NO BOX. The capture is
  # taken into a variable before awk sees it, because awk's verdict on empty
  # input reads exactly like its verdict on a pane carrying no composer.
  # Status 3 is the collar contract's word for a read that was refused.
  pane="$(tmux capture-pane -pJ -t "$1")" || return 3
  line="$(printf '%s\n' "$pane" |
    awk '{ i = index($0, "❯")
           if (i > 0 && (i == 1 || substr($0, 1, i - 1) ~ /[^ \t]/)) line = $0 }
         END { print line }')" || return 1
  case "$line" in *❯*) ;; *) return 1 ;; esac
  printf '%s' "${line#*❯}" | tr -d '\302\240'
}

collar_context() { # $1 = tmux target; reads a beacon the pane was told to print
  local pane m
  # A PANE THAT COULD NOT BE READ IS NOT A PANE WITH NO READOUT. The capture is
  # taken into a variable before grep sees it, because grep's verdict on empty
  # input reads exactly like its verdict on a pane carrying no readout, and the
  # refusal that reaches the operator then names the wrong fault.
  pane="$(tmux capture-pane -pJ -t "$1")" \
    || refuse "cannot read pane $1 — whether a ctx beacon is on that screen is unknown, not absent"
  m="$(printf '%s\n' "$pane" | grep -Eo 'ctx [0-9]+k/[0-9]+k [0-9]+%' | tail -1)" \
    || m=""
  # EMPTY IS THE MISS, and the miss is checked rather than inherited: without
  # pipefail a grep that found nothing hands its status to tail, the assignment
  # succeeds, and a pane with no readout prints as one with an empty reading.
  [ -n "$m" ] || die "no ctx beacon in pane"
  m="${m#ctx }"
  printf '%s (%s)\n' "${m% *}" "${m##* }"
}
