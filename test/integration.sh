#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Fast substrate contract checks. Every assertion reads state that already exists.
# Real-harness behavior belongs in a separately named disposable team.
set -euo pipefail

unset TMUX TMUX_PANE

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
GANG="$ROOT/bin/gang"

# A VERDICT IS ABOUT A TREE, so the tree has to hold still. This refuses to
# start against a working tree that is already moving — bash reads this script
# incrementally, gang re-reads collars and roles at hitch time, and an edit
# landing mid-run changes what executes. The identity is read again at the
# end, because starting settled is not the same as staying settled.
# test/gate.sh is the way to run this before a commit: it snapshots the
# working tree, uncommitted work included, and runs the gate from the copy.
TREE_AT_START="$("$ROOT/test/gate.sh" --assert-owned)"

RUN_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-test.XXXXXX")"
TMUX_SOCKET="$RUN_ROOT/tmux-$(id -u)/default"

export GIT_CONFIG_GLOBAL="$RUN_ROOT/gitconfig"
export GIT_CONFIG_SYSTEM=/dev/null

# The Bash fixture establishes every transition synchronously except one: a pane
# answers its terminal asynchronously. Production waits are inputs here, not
# evidence, so their clock returns immediately — but gang counts its patience in
# reads rather than seconds, and a stopped clock spends five of them in a few
# milliseconds, so a composer that has not echoed yet reads as one that never
# will.
#
# Gang's pane round-trip therefore keeps a floor, far below the duration it
# asked for. Without it, submission verifies only when the pasted payload is
# long enough to make each read slow enough for the echo to arrive: a fixture
# returning the right answer for a reason unrelated to the behaviour under test,
# which inverted the moment the pasted startup contract became a pointer. Keying
# on the requested duration leaves boot and churn clocks immediate, and no
# assertion depends on the floor's value.
#
# THE MARGIN, MEASURED 2026-08-09 on the deployed harness build rather than reasoned
# about, because the reasoning above is what a reader would otherwise have to
# re-derive. submit_verify presses Enter and reads the box up to five times,
# napping 0.4s between reads.
#
#   hermetic fixture pane, Enter to box change  ~5ms (re-measured 2026-08-12)
#   test budget (this floor)     5 x 0.05s = 0.25s
#   production budget            5 x 0.4s  = 2.0s
#
# THE FIRST NUMBER IS A PROPERTY OF THE PANE, NOT OF THE BOX, and leaving that
# implicit is what made this margin unreadable. A fixture shell that reads the
# operator's /etc/bash.bashrc answers the same Enter in ~180ms instead, because
# Debian installs a command_not_found_handle there that runs a Python program
# against a multi-megabyte apt database, and every envelope this suite delivers
# is an unrunnable command — so that handler sits on the Enter path of every
# submission gang verifies. Two fixture collars launched such a shell, spent
# most of this budget before anything had gone wrong, and starved at load 1.06.
# test/lint.sh keeps fixture shells hermetic; the ~5ms above holds only while
# it does.
#
# So the quiet margin is about 50x and looks unbreakable right up until an
# I/O-starved box stretches one round trip past 250ms. Under sustained load —
# CPU spinners plus dd+sync loops, not CPU bursts, which do not reproduce it —
# the same hitch failed 39 times in 40. UNDER THE SAME LOAD, PRODUCTION TIMING
# FAILED 0 IN 20: gang's real delivery survives a box this busy, and only the
# compressed clock does not. That is the more valuable half of the measurement
# and the reason this floor is a test artifact rather than a substrate finding.
#
# AN EVENT BARRIER WAS CONSIDERED AND REFUSED, so the next reader does not
# spend a day rediscovering it. The suite owns this shim, so the shim looks
# like the place to block on pane output instead of napping — with a latched
# tmux wait-for signalled from the fixture's PROMPT_COMMAND. Two things kill
# it. The shim is global to every 0.3/0.4 nap in gang and most of them are not
# waiting on a pane echo, so a blanket barrier deadlocks the rest. And tmux
# wait-for has no timeout, so an unsignalled channel converts a failing test
# into a HANGING suite, which is strictly worse than the flake it replaces.
# Raising the floor is not an option either: a test that passes by waiting
# longer on a slow box reports the box, not the tree.
mkdir -p "$RUN_ROOT/bin"
cat > "$RUN_ROOT/bin/sleep" <<'SH'
#!/bin/sh
case "$1" in
  0.3|0.4) exec /bin/sleep 0.05 ;;
esac
exit 0
SH
chmod +x "$RUN_ROOT/bin/sleep"
PATH="$RUN_ROOT/bin:$PATH"
export PATH

# AN UNVERIFIABLE SUBMISSION IS AN UNKNOWN, AND AN UNKNOWN COSTS ONE ASSERTION.
# The floor above is a race and a starved box can lose it: measured, the same
# setup hitch fails 39 times in 40 under sustained I/O load. That failure
# reaches gang through `die`, so `set -e` used to end the whole run — one
# starved paste cost every remaining check AND the summary, which is the one
# reading nobody can act on, on exactly the busy box the suite exists to speak
# about.
#
# Only SETUP hitches route through this: the ones whose output no assertion
# reads. Anything that inspects what hitch printed, or that means to watch a
# hitch fail, still calls gang directly and still ends the run, so no guard
# here has been softened. The retry is a second OBSERVATION, not a longer wait
# — a real defect fails both, while a starved pane usually loses only once —
# and the unknown is recorded either way so a quiet green and a green bought
# under load never read the same.
cat > "$RUN_ROOT/bin/hitch-guard" <<SH
#!/bin/sh
REAL="$GANG"
UNKNOWNS="$RUN_ROOT/unknowns"
ERR="$RUN_ROOT/hitch-stderr"
SH
cat >> "$RUN_ROOT/bin/hitch-guard" <<'SH'
rc=0
"$REAL" hitch "$@" 2>"$ERR" || rc=$?
if [ "$rc" -ne 0 ] \
  && grep -q 'submit NOT verified\|submission unverifiable' "$ERR"; then
  printf '%s\n' "$1" >> "$UNKNOWNS"
  "$REAL" drop "$1" >/dev/null 2>&1
  rc=0
  "$REAL" hitch "$@" 2>"$ERR" || rc=$?
fi
cat "$ERR" >&2
exit "$rc"
SH
chmod +x "$RUN_ROOT/bin/hitch-guard"
# shellcheck disable=SC2034  # every hitch from test/integration-substrate.sh on runs through it
HITCH="$RUN_ROOT/bin/hitch-guard"

# A run that ends early still owes a reading. One retry rescues an ISOLATED
# starved submission, which is the case that used to cost every later check;
# it cannot rescue a box starving nearly all of them, and pretending otherwise
# would just be the floor raised by another name. So the second consecutive
# starve still ends the run — but it ends it OUT LOUD, naming the coverage
# reached and the reason, because a verdict nobody can read is what made the
# original failure expensive.
summary_printed=0
cleanup() {
  if [ "$summary_printed" -eq 0 ]; then
    printf '\nRUN ENDED EARLY after %s checks in %ss — no verdict on the rest.\n' \
      "$checks" "$SECONDS"
    if [ -s "$RUN_ROOT/unknowns" ]; then
      printf 'Gang could not verify these setup submissions: %s\n' \
        "$(tr '\n' ' ' < "$RUN_ROOT/unknowns")"
      printf 'Each was retried once and starved again, against the shim\n'
      printf 'above: five box readings, 0.05s apart, for one to differ. That\n'
      printf 'budget was missed. This run measured no cause for it and names\n'
      printf 'none.\n'
    fi
  fi
  tmux -S "$TMUX_SOCKET" kill-server 2>/dev/null || true
  rm -rf -- "$RUN_ROOT"
}
trap cleanup EXIT HUP INT TERM

export TMUX_TMPDIR="$RUN_ROOT"
export GANG_CONFIG_DIR="$RUN_ROOT/config"
export GANG_SESSION="gangtest-$$"
export GANG_TEST_COLLARS=1
export GANG_CHURN_WAIT=0
export GANG_LOCK_DIR="$RUN_ROOT/locks"
export GANG_ARCHIVE_DIR="$RUN_ROOT/archive"

checks=0
fails=0

pass() {
  checks=$((checks + 1))
  printf 'ok   %s\n' "$1"
}

fail() {
  checks=$((checks + 1))
  fails=$((fails + 1))
  printf 'FAIL %s\n       %s\n' "$1" "$2"
}

equal() { # $1 description, $2 expected, $3 actual
  if [ "$2" = "$3" ]; then
    pass "$1"
  else
    fail "$1" "expected [$2], got [$3]"
  fi
}

submitted() { # $1 description, $2 agent
  equal "$1" "" "$("$GANG" composer "$2")"
}

contains() { # $1 description, $2 haystack, $3 literal needle
  case "$2" in
    *"$3"*) pass "$1" ;;
    *) fail "$1" "missing [$3]" ;;
  esac
}

excludes() { # $1 description, $2 haystack, $3 literal needle
  case "$2" in
    *"$3"*) fail "$1" "unexpected [$3]" ;;
    *) pass "$1" ;;
  esac
}

refuses() { # $1 description, $2 expected message, rest = command
  local description="$1" expected="$2" output
  shift 2
  if output="$("$@" 2>&1)"; then
    fail "$description" "command unexpectedly succeeded: [$output]"
  else
    contains "$description" "$output" "$expected"
  fi
}

mkdir -p "$RUN_ROOT/no-utf8-bin"
cat > "$RUN_ROOT/no-utf8-bin/locale" <<'SH'
#!/bin/sh
exit 1
SH
chmod +x "$RUN_ROOT/no-utf8-bin/locale"
refuses "startup refuses when no UTF-8 locale can be established" \
  "could not establish a UTF-8 locale" \
  env -u LC_ALL -u LC_CTYPE LANG=C PATH="$RUN_ROOT/no-utf8-bin:$PATH" \
    "$GANG" config

bare_window_name() { # $1 raw tmux window name
  local n="$1" first last
  [ "${#n}" -ge 3 ] || { printf '%s' "$n"; return; }
  first="${n:0:1}"; last="${n: -1}"
  case "$first" in
    -|'~'|'!'|'?') [ "$first" = "$last" ] && n="${n:1:${#n}-2}" ;;
  esac
  printf '%s' "$n"
}

window_id_in() { # $1 session, $2 bare window name
  local id name
  while read -r id name; do
    [ "$(bare_window_name "$name")" = "$2" ] && { printf '%s' "$id"; return; }
  done < <(tmux list-windows -t "=$1" -F '#{window_id} #{window_name}')
  return 1
}

window_id() { # $1 bare window name
  window_id_in "$GANG_SESSION" "$1"
}

window_names() { # optional $1 session -> bare names, one per line
  local session="${1:-$GANG_SESSION}" name
  while IFS= read -r name; do bare_window_name "$name"; printf '\n'; done \
    < <(tmux list-windows -t "=$session" -F '#W')
}

pane() { tmux capture-pane -pJ -t "$(window_id "$1")"; }
pane_all() { tmux capture-pane -pJ -S - -t "$(window_id "$1")"; }

# THE SUITE IS ONE PROGRAM, SPLIT ONLY SO THAT IT CAN BE LINTED. Each part below
# is sourced in order into this shell and reads the fixtures, helpers and
# counters established above, so the split moves no assertion and changes no
# ordering. It exists because shellcheck holds a whole file at once and its cost
# grows faster than that file does: as a single 398 KB script this suite alone
# blew through a 3 GB ceiling in fourteen seconds, and the file set it dominated
# reached 6.1 GB, which the kernel OOM killer took twice on 2026-08-12 on an
# 11.6 GB host that was power-cycled hours later. A part at a time the same
# coverage peaks in the hundreds of MB and the whole gate fits under 2 GB.
#
# A part is a fragment, so shellcheck cannot see across the boundary. A variable
# that crosses one carries a directive naming the file at the other end; that
# directive is the record that the crossing was deliberate.
. "$ROOT/test/integration-cli.sh"
. "$ROOT/test/integration-substrate.sh"
. "$ROOT/test/integration-hitch.sh"
. "$ROOT/test/integration-compose.sh"
. "$ROOT/test/integration-spool.sh"
. "$ROOT/test/integration-readiness.sh"
. "$ROOT/test/integration-hooks.sh"
. "$ROOT/test/integration-gate.sh"

# The focused role instrument is mandatory here and independently selectable so
# mutation calibration can run the exact AC that must turn red.
"$ROOT/test/role-briefs.sh"

# THE SAME TREE THIS RUN STARTED AGAINST, OR NO VERDICT. A source edit landing
# mid-run is not caught by either read — bash has already executed whatever it
# read — so this cannot make such a run safe. It can only stop the number below
# from being quoted as a fact about a tree that no longer exists, which is the
# form the last one took.
tree_moved=0
"$ROOT/test/gate.sh" --assert-unmoved "$TREE_AT_START" || tree_moved=1

summary_printed=1
printf '\n%s checks in %ss\n' "$checks" "$SECONDS"
if [ "$tree_moved" -eq 1 ]; then
  printf 'THE SOURCE TREE MOVED DURING THIS RUN, so the count above is not a\n'
  printf 'verdict on any tree. The refusal above says what changed.\n'
fi
# Reported apart from both columns and folded into neither: an unknown is a
# submission this run could not verify, which is neither a pass nor a fail.
# Green with unknowns above zero means the coverage held while something missed
# the compressed clock's budget, which is worth knowing before the number gets
# quoted anywhere.
if [ -s "$RUN_ROOT/unknowns" ]; then
  printf '%s setup submission(s) gang could not verify, re-established: %s\n' \
    "$(wc -l < "$RUN_ROOT/unknowns" | tr -d ' ')" \
    "$(tr '\n' ' ' < "$RUN_ROOT/unknowns")"
  printf 'Something missed the compressed clock budget above. This run\n'
  printf 'measured no cause and names none.\n'
fi
[ "$fails" -eq 0 ] && [ "$tree_moved" -eq 0 ]
