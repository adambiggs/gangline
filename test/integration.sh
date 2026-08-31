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
# evidence, so the settled-composer clock returns immediately. The fixture
# supplies every between-read change explicitly; spending 0.3s between those
# reads adds no evidence. Post-keystroke verification is different: gang counts
# its patience in reads rather than seconds, and a stopped clock spends five of
# them before the pane has necessarily echoed the key, so a composer that has
# not answered yet reads as one that never will.
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
#   test budget (this floor)     5 x 0.01s = 0.05s
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
# So the quiet budget is about 10x the measured reaction, with each individual
# floor still 2x it, and looks unbreakable right up until an I/O-starved box
# stretches one round trip past 50ms. Under sustained load —
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
# it. The shim is global to every 0.4 nap in gang and most of them are not
# waiting on a pane echo, so a blanket barrier deadlocks the rest. And tmux
# wait-for has no timeout, so an unsignalled channel converts a failing test
# into a HANGING suite, which is strictly worse than the flake it replaces.
# Raising the floor is not an option either: a test that passes by waiting
# longer on a slow box reports the box, not the tree.
#
# AND THE CLOCK IS COUNTED, BECAUSE A COMPRESSED CLOCK ON ITS OWN IS SHAPED
# EXACTLY LIKE EVIDENCE AND CARRIES NONE. Every wait above collapses to nothing,
# so a loop that spends its budget and a loop that busy-spins through it finish
# the same way and pass the same checks: an unbounded loop, an ignored timeout
# budget, a nap deleted outright — none of them can turn this suite red, and a
# defect of exactly that shape once sat under a fully green run.
#
# The instrument is the ledger below. Where an assertion sets
# GANG_TEST_CLOCK_LEDGER, this shim records every duration the code under test
# ASKED FOR, in order, before returning; the durations it asks for are the
# budget it means to spend, and now they are a readable artifact rather than a
# silence. A liveness claim brings its own scoped ledger and asserts a count off
# it, so the claim fails when the budget stops being consumed. Unscoped — which
# is nearly every invocation — nothing is written and nothing is slower.
#
# THIS IS THE ONE CLOCK THAT MAY BE STOPPED RATHER THAN SCALED, and only because
# the count, not the wall clock, is what the assertion reads. A scaled clock
# would prove the same thing more slowly and no more truly; a stopped clock with
# no ledger is what this repository already had, and it proved nothing at all.
mkdir -p "$RUN_ROOT/bin"
cat > "$RUN_ROOT/bin/sleep" <<'SH'
#!/bin/sh
[ -z "${GANG_TEST_CLOCK_LEDGER:-}" ] || printf '%s\n' "$1" >> "$GANG_TEST_CLOCK_LEDGER"
case "$1" in
  0.3) exit 0 ;;
  0.4) exec /bin/sleep 0.01 ;;
esac
exit 0
SH
chmod +x "$RUN_ROOT/bin/sleep"

# A BARRIER WITH NO CEILING CANNOT GO RED, ONLY QUIET. `tmux wait-for` is a
# client call with no timeout, so a signal that is sent and never arrives parks
# the run forever: no assertion fails, no summary prints, and because the gate
# serialises on one host lock, every other run queues behind it. Driven
# 2026-08-24, that happened on two different barriers, on two trees, in runs
# started by two different people, and each cost hours before anyone read a
# process list. The suite already owns this bin directory and it is already
# ahead of everything the run launches, so the ceiling goes here rather than at
# a hundred and fifty call sites.
#
# ITS OWN DIRECTORY, NOT THE ONE BESIDE IT. The compressed-clock shim above is
# something a case may legitimately need to be rid of — the `gang up` driver
# strips that directory off PATH so its attached client runs on the production
# clock. Sharing a directory meant the ceiling was stripped with it, and the run
# that proved the ceiling works is the same run that found the hole: the wedge
# was in a fixture launched by exactly that driver, so the wait that hung was
# the one call in the suite the ceiling could not see. A ceiling with a hole in
# it where the wedges happen is not a ceiling.

# WHAT IT BOUNDS AND WHAT IT LEAVES ALONE. Only a BLOCKING wait — `wait-for`
# with a channel. A `-S` signal already returns whether or not anyone waits, so
# bounding it would add a clock where there is no wait, and every other tmux
# command runs untouched.
#
# THE CEILING, MEASURED HERE RATHER THAN REASONED ABOUT:
#
#   signalled barrier, waiter already blocked   ~10ms   (measured 2026-08-24)
#   signalled barrier, latched then read back   ~17ms   (same box, under load)
#   slowest legitimate wait in this suite       a boot, single-digit seconds
#   this ceiling                                120s
#   whole-suite budget in CI                    900s
#
# A hundred and twenty seconds is more than an order of magnitude past the
# slowest wait any fixture here legitimately takes, so it cannot turn a slow box
# into a red; and it is small enough that one expiry still leaves the run time
# to finish and report inside the CI ceiling. It is deliberately not tight:
# this is the difference between a wedge and a verdict, not a performance
# assertion. GANG_TEST_WAIT_CEILING lowers it for the one fixture that proves
# expiry works, which would otherwise spend the whole ceiling proving it.
#
# THE REAL TMUX IS THE ONE THAT IS NOT A SCRIPT, AND IT IS RESOLVED ONCE.
# `command -v tmux` is not the answer: an agent already runs with its own tmux
# guard ahead of tmux on PATH, so in an agent's environment that names a shim,
# and a shim that calls a shim which resolves by "the first tmux that is not
# mine" selects this one straight back. A shim is a `#!` file and tmux is a
# binary, so that is the question asked instead — here, at generation, rather
# than in the shim, because the shim runs on every tmux call this suite makes
# and a PATH walk per call would be thousands of forks bought for one answer
# that never changes.
REAL_TMUX=""
while IFS= read -r tmux_candidate; do
  [ -x "$tmux_candidate" ] && [ ! -d "$tmux_candidate" ] || continue
  [ "$(head -c 2 "$tmux_candidate" 2>/dev/null)" = '#!' ] && continue
  REAL_TMUX="$tmux_candidate"
  break
done <<<"$(type -pa tmux 2>/dev/null || true)"
[ -n "$REAL_TMUX" ] || {
  echo "integration: found no tmux on PATH that is not itself a shim" >&2
  exit 1
}
mkdir -p "$RUN_ROOT/waitbin"
cat > "$RUN_ROOT/waitbin/tmux" <<SH
#!/bin/sh
: "\${GANG_TEST_WAIT_LEDGER:=$RUN_ROOT/wedged-barriers}"
GANG_TEST_WAIT_STATE='$RUN_ROOT/wait-expiry'
real='$REAL_TMUX'
SH
cat >> "$RUN_ROOT/waitbin/tmux" <<'SH'
set -u

# THE VERB IS NOT ALWAYS THE FIRST WORD. tmux takes its own options ahead of
# the command, so `tmux -S <socket> wait-for <channel>` is the same blocking
# wait as `tmux wait-for <channel>` and must be bounded the same. Reading only
# $1 bounds one spelling and silently stops bounding the other, which is the
# shape of guard that rots while everyone believes it holds. The argv is
# scanned for the first word that is not an option or an option's value, and
# the word after it decides whether this is a wait or a signal.
verb=""
verb_at=0
next=""
skip=0
seen=0
for word in "$@"; do
  seen=$((seen + 1))
  if [ "$skip" -eq 1 ]; then skip=0; continue; fi
  case "$word" in
    # The options that take a separate value; anything else beginning with a
    # dash carries its own or takes none.
    -[cfLST]) skip=1 ;;
    -*) ;;
    *) verb="$word"; verb_at=$seen; break ;;
  esac
done
if [ "$verb" = wait-for ]; then
  seen=0
  for word in "$@"; do
    seen=$((seen + 1))
    [ "$seen" -eq $((verb_at + 1)) ] || continue
    next="$word"
    break
  done
fi
[ "$verb" = wait-for ] && [ "$next" != -S ] || exec "$real" "$@"

ceiling=${GANG_TEST_WAIT_CEILING:-120}
expiry="$GANG_TEST_WAIT_STATE.$$"
rm -f "$expiry"
"$real" "$@" &
waiter=$!
# /bin/sleep BY PATH, because the shim beside this one shadows `sleep` and
# returns immediately for every duration — an unqualified sleep here would
# expire every barrier in the suite the instant it was asked to wait.
#
# ITS OWN STDIO, AND IT TAKES ITS SLEEPER WITH IT. This guard outlives nothing,
# and both halves of that are load-bearing. A caller reading a barrier inside a
# command substitution is reading a pipe, and a background process that
# inherited the write end holds it open whether or not it has anything to say —
# so a guard still napping after its barrier was answered kept the caller's
# substitution open for the whole ceiling. It is not a hang and it was not
# visible as one: the barrier passed, the assertion after it simply took two
# minutes. And killing the subshell does not kill the sleeper it forked, so the
# sleeper is killed by name from a trap rather than left orphaned.
#
# AND THE NAME IS ONE IT ALREADY HAS. `kill 0` is every process in the sender's
# process group, which here is the run that called this shim, so a trap that
# fired before it had learned the sleeper's pid would take the suite down with
# it. That gap is entered: hammering this teardown 300 times, the trap won a
# race with its own installation 3 times and twice of those it ran with no pid
# yet. The shim bounds every blocking barrier in the suite, so once in a few
# hundred teardowns is once or more per run, and its symptom would be the whole
# run vanishing at an unrelated line.
#
# AND THE SIGNALS ARE ONES THE CALLER CANNOT HAVE TURNED OFF. gang's spool
# drain runs under `trap "" HUP INT TERM`, an ignored disposition survives
# exec, and POSIX does not let a non-interactive shell un-ignore what was
# ignored when it started. Every process the drain forks — this shim, its
# guard, and the tmux client the guard has to cut off — therefore inherits
# SIGTERM as ignored, the guard's kill was a no-op, and the shim sat in `wait`
# forever without ever reporting the barrier it was bounding. The ceiling
# silently did not exist in the one place the suite's own barriers run.
# Measured: with the ceiling at 2s the shim was still blocked after 8s, and the
# same shim outside a drain reported at 2s. SIGKILL cannot be ignored, and
# SIGUSR1 is not in that ignore set, so the guard still ends its own sleeper
# rather than orphaning it.
napper=
( trap '[ -n "$napper" ] && kill -KILL "$napper" 2>/dev/null; exit 0' USR1
  /bin/sleep "$ceiling" &
  napper=$!
  wait "$napper"
  : > "$expiry"
  kill -KILL "$waiter" 2>/dev/null ) >/dev/null 2>&1 &
guard=$!
rc=0
wait "$waiter" || rc=$?
kill -USR1 "$guard" 2>/dev/null
wait "$guard" 2>/dev/null || true
if [ -e "$expiry" ]; then
  rm -f "$expiry"
  # NAMED WHERE IT OUTLIVES THE PANE. A fixture's stderr goes to a pane that is
  # about to be torn down, so the channel is also written where the run's own
  # summary reads it and refuses to call the run green.
  printf 'integration: BARRIER NEVER SIGNALLED after %ss: tmux %s\n' "$ceiling" "$*" >&2
  printf '%s\t%s\n' "$*" "$ceiling" >> "$GANG_TEST_WAIT_LEDGER" 2>/dev/null || true
  exit 111
fi
rm -f "$expiry"
exit "$rc"
SH
chmod +x "$RUN_ROOT/waitbin/tmux"
PATH="$RUN_ROOT/bin:$RUN_ROOT/waitbin:$PATH"
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
# The summary line this run ends on, and the subject of the fixture that drives
# its failing branch — which this run, being green, never reaches. Sourced
# ahead of the trap below rather than beside its own call, because the trap
# reads the barrier ledger and a run that dies before this point is exactly the
# run whose ledger has something in it.
. "$ROOT/test/suite-tail.sh"

summary_printed=0
role_pid=""
gate_pid=""
cleanup() {
  if [ -n "$role_pid" ]; then
    kill "$role_pid" 2>/dev/null || true
    wait "$role_pid" 2>/dev/null || true
  fi
  if [ -n "$gate_pid" ]; then
    kill "$gate_pid" 2>/dev/null || true
    wait "$gate_pid" 2>/dev/null || true
  fi
  if [ "$summary_printed" -eq 0 ]; then
    printf '\nRUN ENDED EARLY after %s checks in %ss — no verdict on the rest.\n' \
      "$checks" "$SECONDS"
    suite_wedged_barriers "$RUN_ROOT/wedged-barriers"
    if [ -s "$RUN_ROOT/unknowns" ]; then
      printf 'Gang could not verify these setup submissions: %s\n' \
        "$(tr '\n' ' ' < "$RUN_ROOT/unknowns")"
      printf 'Each was retried once and starved again, against the shim\n'
      printf 'above: five box readings, 0.05s apart, for one to differ. That\n'
      printf 'budget was missed. This run measured no cause for it and names\n'
      printf 'none.\n'
    fi
  fi
  [ -z "${guard_live_socket:-}" ] \
    || "$REAL_TMUX" -S "$guard_live_socket" kill-server 2>/dev/null || true
  tmux -S "$TMUX_SOCKET" kill-server 2>/dev/null || true
  rm -rf -- "$RUN_ROOT"
}
# A SIGNAL ENDS THE RUN; IT DOES NOT ANNOTATE IT. One handler for the exit and
# for the signals reads as if a signalled run stops here, and it does not: a
# bash signal handler RETURNS to the interrupted flow. The teardown below then
# runs at the signal — banner, tmux server, RUN_ROOT — and execution resumes
# with every fixture deleted, so the checks after it fail against nothing and
# name real assertions while doing it. Two banners print with different counts,
# which is the run stating twice, differently, how much it covered.
# Exiting from the signal handler is what stops that flow. The teardown itself
# is unchanged and still destroys everything this run created; what changes is
# that nothing executes afterwards. The disposition is cleared first so a second
# signal arriving during the exit cannot re-enter this and start a second
# teardown over the first.
on_signal() { # $1 = signal number, for the conventional 128 + signo status
  trap - HUP INT TERM
  exit "$((128 + $1))"
}
trap cleanup EXIT
trap 'on_signal 1' HUP
trap 'on_signal 2' INT
trap 'on_signal 15' TERM

export TMUX_TMPDIR="$RUN_ROOT"
export GANG_CONFIG_DIR="$RUN_ROOT/config"
export GANG_SESSION="gangtest-$$"
export GANG_TEST_COLLARS=1
export GANG_CHURN_WAIT=0
export GANG_LOCK_DIR="$RUN_ROOT/locks"
export GANG_ARCHIVE_DIR="$RUN_ROOT/archive"
export XDG_STATE_HOME="$RUN_ROOT/state"
export GANG_TEST_TICK_MODE=manual
# A COLLAR MAY READ ITS HARNESS'S OWN CONFIGURATION, and the Codex collar reads
# $CODEX_HOME to learn which model a hitch without -m will launch. Pointed at a
# directory this run owns and never creates, every such read is a definite
# absence rather than whatever the operator happens to have configured, so an
# assertion that forgets to name its own fixture home goes quiet instead of
# passing on this machine alone.
export CODEX_HOME="$RUN_ROOT/absent-codex-home"

checks=0
fails=0
unknowns=0

# NEITHER COLUMN, ON PURPOSE — a tmux that hands back a control byte raw is the
# case that put this here. The line itself and the clause it earns in the
# summary live in test/suite-tail.sh, where a fixture can drive the branch no
# ordinary green run takes; only the counter is this suite's.
unknown() { # $1 description, $2 why this run settles nothing either way
  unknowns=$((unknowns + 1))
  suite_unknown "$1" "$2"
}

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

clock_ledger() { # $1 = a name for this measurement -> an empty ledger path
  local path="$RUN_ROOT/clock-$1"
  : > "$path"
  printf '%s' "$path"
}

# A LEDGER THAT WAS NEVER WRITTEN IS NOT A BUDGET THAT WAS NEVER SPENT. The
# instrument can be absent — a shim shadowed on PATH, a name that never reached
# the command — and a count of zero read off nothing would let a liveness
# assertion pass on the strength of its own broken instrument. Missing
# instrument answers 'unknown', which matches no expected count.
clock_naps() { # $1 = ledger path, $2 = requested duration -> times it was asked for
  if [ ! -f "$1" ]; then printf 'unknown'; return; fi
  awk -v want="$2" '$0 == want { n = n + 1 } END { print n + 0 }' "$1"
}

# AND THE INSTRUMENT IS CALIBRATED BEFORE ANY BUDGET IS READ OFF IT, in both
# directions. A reader that answered 'unknown' to everything would satisfy the
# absent case while quietly making every count below unfailable, and a reader
# that counted every line would satisfy the counting case while reporting a
# budget nobody asked for. The absent case is the one no ordinary run reaches,
# which is exactly why it is driven here rather than trusted.
clock_cal="$(clock_ledger calibration)"
printf '%s\n' 1 1 2 >> "$clock_cal"
equal "the clock reader counts only the duration it was asked about" \
  "2 1 0" \
  "$(clock_naps "$clock_cal" 1) $(clock_naps "$clock_cal" 2) $(clock_naps "$clock_cal" 3)"
equal "and a ledger that was never written answers unknown rather than zero" \
  "unknown" "$(clock_naps "$RUN_ROOT/clock-never-written" 1)"

# THIS RUN'S OWN SIGNAL DISPOSITION IS SOMETHING IT CAN READ NOW. A bash signal
# handler returns to the interrupted flow, so a teardown installed on TERM does
# not end a run: it deletes RUN_ROOT and kills the private tmux server, and the
# remaining checks then execute against nothing and fail while naming real
# assertions. Whether that happens is decided by two things, and both are
# readable here without signalling anything — the handler must not return, and
# TERM must reach it rather than the teardown.
#
# The subshell clears EXIT first because a subshell INHERITS it: without that,
# proving the handler exits would run the teardown and delete this run's
# fixtures, which is the very failure under test.
# The status is taken on the assignment itself, because under `set -e` a
# command substitution that fails ENDS THE SCRIPT at the assignment and the
# line below it never runs — which is what a handler exiting 143 looks like to
# the shell reading it.
signal_handler_rc=0
signal_handler_out="$( ( trap - EXIT HUP INT TERM; on_signal 15; printf 'RETURNED\n' ) 2>&1 )" \
  || signal_handler_rc=$?
equal "the signal handler ends the run instead of returning to it" \
  "143 []" "$signal_handler_rc [$signal_handler_out]"
if trap -p TERM | grep -q on_signal; then
  pass "and a TERM reaches that handler rather than the teardown"
else
  fail "and a TERM reaches that handler rather than the teardown" \
    "TERM is [$(trap -p TERM)], which returns to the run after tearing it down"
fi

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

# THE ROLE INSTRUMENT OWNS A DIFFERENT TMUX SERVER AND DIFFERENT FIXTURE ROOT.
# It shares no mutable state with this suite, and it is already independently
# selectable for mutation calibration. Run it beside the substrate checks and
# join its complete verdict below: every check still runs, while two independent
# private fixtures no longer spend the mandatory gate's wall clock in series.
role_output="$RUN_ROOT/role-briefs.out"
"$ROOT/test/role-briefs.sh" > "$role_output" 2>&1 &
role_pid=$!

# THE GATE SELF-TEST BUILDS ONLY ITS OWN GIT FIXTURES. It reads the helpers and
# counters above but no tmux window or mutable fixture used by the substrate
# parts, so it can prove the snapshot machinery beside them. The subshell writes
# its counter delta for the parent to fold into the same final verdict; running
# it elsewhere must not make its assertions disappear from the suite count.
gate_output="$RUN_ROOT/integration-gate.out"
gate_counts="$RUN_ROOT/integration-gate.counts"
(
  trap - EXIT HUP INT TERM
  gate_checks_at_start="$checks"
  gate_fails_at_start="$fails"
  . "$ROOT/test/integration-gate.sh"
  printf '%s %s\n' \
    "$((checks - gate_checks_at_start))" "$((fails - gate_fails_at_start))" \
    > "$gate_counts"
) > "$gate_output" 2>&1 &
gate_pid=$!

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
# A focused part still receives this file's fake clock, helpers, and teardown,
# but source fragments deliberately share fixture state. The dependency manifest
# below names that closure before any fragment sources: a partial request that
# omits one refuses with the exact parts to add, rather than failing later on a
# raw tmux or missing-fixture read.
integration_parts="cli substrate hitch compose spool readiness hooks notify tick"
IFS=, read -r -a integration_selected_parts <<< "${GANG_INTEGRATION_PARTS:-all}"
for integration_selected_part in "${integration_selected_parts[@]}"; do
  [ "$integration_selected_part" = all ] || [[ " $integration_parts " == *" $integration_selected_part "* ]] || {
    printf 'integration: unknown focused part %s\n' "$integration_selected_part" >&2
    exit 2
  }
done

integration_part() {
  local part="$1" selected=",${GANG_INTEGRATION_PARTS:-all},"
  [[ "$selected" == *,all,* || "$selected" == *,"$part",* ]]
}

integration_part_dependencies() { # $1 = selectable fragment, stdout = explicit prerequisites
  case "$1" in
    cli) printf '\n' ;;
    substrate|spool|notify|tick) printf 'cli\n' ;;
    hitch|compose|readiness) printf 'cli substrate\n' ;;
    hooks) printf 'cli substrate spool\n' ;;
    *) return 2 ;;
  esac
}

for integration_declared_part in $integration_parts; do
  integration_part_dependencies "$integration_declared_part" >/dev/null || {
    printf 'integration: no dependency declaration for selectable part %s\n' \
      "$integration_declared_part" >&2
    exit 2
  }
done

if ! integration_part all; then
  for integration_selected_part in "${integration_selected_parts[@]}"; do
    integration_required_parts="$(integration_part_dependencies "$integration_selected_part")" \
      || { printf 'integration: no dependency declaration for focused part %s\n' \
             "$integration_selected_part" >&2; exit 2; }
    integration_missing_parts=""
    for integration_required_part in $integration_required_parts; do
      integration_part "$integration_required_part" \
        || integration_missing_parts="${integration_missing_parts:+$integration_missing_parts,}$integration_required_part"
    done
    [ -z "$integration_missing_parts" ] && continue
    printf 'integration: focused part %s requires %s; run GANG_INTEGRATION_PARTS=%s,%s\n' \
      "$integration_selected_part" "$integration_missing_parts" \
      "$integration_missing_parts" "$integration_selected_part" >&2
    exit 2
  done
fi
unset integration_declared_part integration_missing_parts integration_parts \
  integration_required_part integration_required_parts integration_selected_part integration_selected_parts

integration_part cli && . "$ROOT/test/integration-cli.sh"
integration_part substrate && . "$ROOT/test/integration-substrate.sh"
integration_part hitch && . "$ROOT/test/integration-hitch.sh"
integration_part compose && . "$ROOT/test/integration-compose.sh"
integration_part spool && . "$ROOT/test/integration-spool.sh"
integration_part readiness && . "$ROOT/test/integration-readiness.sh"
integration_part hooks && . "$ROOT/test/integration-codex-stop-hook.sh"
integration_part hooks && . "$ROOT/test/integration-hooks.sh"
integration_part notify && . "$ROOT/test/integration-notify.sh"
integration_part tick && . "$ROOT/test/integration-tick.sh"

# Join the isolated self-test at the same point where it used to run. Its output
# stays contiguous, and its checks and failures remain part of this suite's one
# summary rather than becoming a second verdict.
gate_rc=0
wait "$gate_pid" || gate_rc=$?
gate_pid=""
cat "$gate_output"
if [ "$gate_rc" -ne 0 ] || [ ! -s "$gate_counts" ]; then
  printf 'integration gate self-test ended without a readable count (status %s)\n' \
    "$gate_rc" >&2
  [ "$gate_rc" -ne 0 ] || gate_rc=1
  exit "$gate_rc"
fi
read -r gate_checks gate_fails < "$gate_counts"
checks=$((checks + gate_checks))
fails=$((fails + gate_fails))

# The focused role instrument is mandatory here and independently selectable so
# mutation calibration can run the exact AC that must turn red. Its output is
# held until the join so concurrent suites never interleave their evidence.
role_rc=0
wait "$role_pid" || role_rc=$?
role_pid=""
cat "$role_output"
[ "$role_rc" -eq 0 ] || exit "$role_rc"

# THE SAME TREE THIS RUN STARTED AGAINST, OR NO VERDICT. A source edit landing
# mid-run is not caught by either read — bash has already executed whatever it
# read — so this cannot make such a run safe. It can only stop the number below
# from being quoted as a fact about a tree that no longer exists, which is the
# form the last one took.
tree_moved=0
"$ROOT/test/gate.sh" --assert-unmoved "$TREE_AT_START" || tree_moved=1

summary_printed=1
printf '\n'
suite_tail "$checks" "$fails" "$SECONDS" "$(suite_unknown_clause "$unknowns")"
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
# AN EXPIRED BARRIER IS ALSO A VERDICT. It reaches the shim's caller as a
# nonzero status, which ends the run where that caller is this shell; a fixture
# inside a pane has nowhere to fail to, so the ledger is what keeps the run from
# ending green around it.
barriers_wedged=0
[ ! -s "$RUN_ROOT/wedged-barriers" ] || barriers_wedged=1
suite_wedged_barriers "$RUN_ROOT/wedged-barriers"
[ "$fails" -eq 0 ] && [ "$tree_moved" -eq 0 ] && [ "$barriers_wedged" -eq 0 ]
