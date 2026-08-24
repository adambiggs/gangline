#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# HOW THIS TMUX HANDS BACK A USER OPTION THAT HOLDS A CONTROL BYTE.
#
# Gangline reads an agent's identity out of @gl_agent, and what that read
# returns is not the same on every supported tmux: 3.2 hands back the byte that
# was written, while 3.4 serializes it as the four visible characters `\033`.
# Only the first of those can paint somebody's terminal, so only the first can
# fail a resolver's sanitization. A suite that does not ask which world it is in
# reports a pass either way and means it in only one of them.
#
#   raw       the byte survives the round trip; sanitization is under test here
#   escaped   tmux serialized it; nothing on this host can carry the hostile byte
#
# Anything else ends this loudly rather than picking one: a third representation
# is a substrate Gangline has not been taught to read, and guessing which half
# of it is safe is how a guard rots while everyone still believes it.
#
# Printed by test/integration.sh as an instrument reading, and asserted to be
# `raw` by the tmux cell in .github/workflows/shell.yml that exists to provide
# that world in routine CI.
set -euo pipefail

# The live server is not the subject. Inside an agent window $TMUX is set and a
# bare tmux talks to it while TMUX_TMPDIR is ignored without saying so.
unset TMUX TMUX_PANE

# A CLASSIFIER THAT WAS NEVER CALIBRATED IS AN OPINION. This probe's whole
# output is one classification, and the branch that matters most is the one no
# supported substrate reaches — so it is driven from known bytes here rather
# than left to a host that may never produce it. Both other directions are
# driven too: a classifier answering 'raw' to everything would pass a raw-only
# calibration while making the probe useless.
classify() { # $1 = what tmux handed back -> raw | escaped | unreadable
  case "$1" in
    *$'\033'*) printf 'raw' ;;
    'probe\033[31mbyte') printf 'escaped' ;;
    *) printf 'unreadable' ;;
  esac
}

cal_raw="$(classify "$(printf 'probe\033[31mbyte')")"
cal_escaped="$(classify 'probe\033[31mbyte')"
cal_neither="$(classify 'probe[31mbyte')"
if [ "$cal_raw $cal_escaped $cal_neither" != 'raw escaped unreadable' ]; then
  printf 'this probe does not hold its own calibration, so its verdict about any tmux means nothing: the raw, escaped and unrecognised forms classified as [%s %s %s].\n' \
    "$cal_raw" "$cal_escaped" "$cal_neither" >&2
  exit 1
fi

PROBE_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-optbytes.XXXXXX")"
PROBE_SOCKET="$PROBE_ROOT/s"
PROBE_SESSION="gangprobe-optbytes-$$"

cleanup() {
  tmux -S "$PROBE_SOCKET" kill-session -t "=$PROBE_SESSION" 2>/dev/null || true
  tmux -S "$PROBE_SOCKET" kill-server 2>/dev/null || true
  rm -rf -- "$PROBE_ROOT"
}
trap cleanup EXIT HUP INT TERM

# `cat` with no argument reads the pane's terminal and stays, so the window
# outlives the option write without any wait standing between the two.
tmux -S "$PROBE_SOCKET" new-session -d -s "$PROBE_SESSION" -n probe cat

tmux -S "$PROBE_SOCKET" set-option -w -t "=$PROBE_SESSION:probe" \
  @gl_probe "$(printf 'probe\033[31mbyte')"
read_back="$(tmux -S "$PROBE_SOCKET" show-options -wqv \
  -t "=$PROBE_SESSION:probe" @gl_probe)"

verdict="$(classify "$read_back")"
case "$verdict" in
  raw|escaped) printf '%s\n' "$verdict" ;;
  *)
    printf '%s answered a control-bearing user option as [%s], which is neither the raw byte nor the escaped form Gangline reads. This probe has no verdict about that substrate and will not invent one.\n' \
      "$(tmux -V)" "$read_back" >&2
    exit 1
    ;;
esac
