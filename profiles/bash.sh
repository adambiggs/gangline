# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
GANG_LAUNCH="PS1='❯ ' bash --norc"
GANG_BUSY_REGEX=""

profile_input() { # $1 = tmux target; same shape as a real TUI's input box
  local line
  line="$(tmux capture-pane -pJ -t "$1" |
    awk '{ i = index($0, "❯")
           if (i > 0 && (i == 1 || substr($0, 1, i - 1) ~ /[^ \t]/)) line = $0 }
         END { print line }')" || return 1
  case "$line" in *❯*) ;; *) return 1 ;; esac
  printf '%s' "${line#*❯}" | tr -d '\302\240'
}

profile_context() { # $1 = tmux target; reads a beacon the pane was told to print
  local m
  m="$(tmux capture-pane -pJ -t "$1" | grep -Eo 'ctx [0-9]+k/[0-9]+k [0-9]+%' | tail -1)" \
    || die "no ctx beacon in pane"
  m="${m#ctx }"
  printf '%s (%s)\n' "${m% *}" "${m##* }"
}
