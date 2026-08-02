#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# PROBE INSTRUMENT. This file exists to measure a rate, it lives only on a
# probe/* branch, and it never lands on main.
#
#   test/probe-loop.sh
#
# The `shell` workflow has been red on the macOS cell only while ubuntu stays
# green in the same run. The question this answers is which of two things that
# is: a real race in gang's delivery/occupancy path that macOS timing exposes
# and Linux timing hides, or a timing sensitivity in the suite's own fixtures.
# Calling it the second without separating the two is the degraded mode this
# repo exists to refuse.
#
# The instrument is a RATE. CI renders about once every twenty-five minutes, so
# one trial per push cannot tell a 15% event from a 2% one, and neither can it
# ever show that a fix moved anything. This loops the suspect units N times
# inside one cell, on both cells, and prints a rate per arm.
#
# WHAT IT DOES NOT DO: it does not replay the ~1050 checks that precede these
# units in test/integration.sh. It bootstraps the state they need instead — a
# shim tree and a hitched stand-in — which is cheap here only because that
# state is genuinely small. The risk in bootstrapping is a unit that CANNOT
# fail, so arm E exists to force a failure through this harness — and to force
# it in the SHAPE the CI failure has, status 1 with a paste already on the
# screen, rather than merely erroring somehow. If arm E does not show that pair,
# nothing else printed here means anything and the run exits non-zero saying so.
set -uo pipefail

# Same reason test/integration.sh does it, and the same failure if it is not
# done: handed only one half of tmux's pane/server pair, gang is told there is
# no server while still being handed a pane id on one, and it fails wide in
# exactly the paths this probe measures. TMUX_TMPDIR is deliberately kept — it
# is how this run stays off an operator's live socket.
unset TMUX TMUX_PANE

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
GANG="$ROOT/bin/gang"
export GANG_SESSION="gangprobe-$$"
export GANG_TEST_PROFILES=1
export GANG_PATROL_LOG=
export GANG_CHURN_WAIT=0.3
export GANG_BOOT_TIMEOUT=8

PROBE_N="${PROBE_N:-120}"
PROBE_ARMS="${PROBE_ARMS:-A Anolog B C D E}"
# The shipped bound is five. Two is enough to prove a hold ENDS and what status
# it ends with, which is the pair under measurement here; the suite's own case
# keeps the five and keeps the elapsed-time assertion that goes with it. This
# probe does not assert elapsed time, so it must not be read as covering it.
PROBE_HOLD="${PROBE_HOLD:-2}"
PROBE_OUT="${PROBE_OUT:-$ROOT/probe-artifacts}"

SHIM="$(mktemp -d)"
trap 'tmux kill-session -t "$GANG_SESSION" 2>/dev/null; tmux kill-session -t "$GANG_SESSION-h2" 2>/dev/null; rm -rf "$SHIM"' EXIT
mkdir -p "$SHIM/custom-profiles" "$PROBE_OUT"

# --- the environment, named once per cell -------------------------------------
#
# The whole hypothesis for one arm is about WHICH bash reads the fixture, so a
# run that did not say which one it got proves nothing about it.
fingerprint_cell() {
  {
    printf 'uname:       %s\n' "$(uname -a)"
    printf 'bash(path):  %s\n' "$(command -v bash)"
    printf 'bash(ver):   %s\n' "$(bash --version | head -1)"
    printf 'bash(this):  %s.%s\n' "${BASH_VERSINFO[0]}" "${BASH_VERSINFO[1]}"
    printf 'tmux:        %s\n' "$(tmux -V)"
    printf 'lockdir:     %s\n' "${GANG_LOCK_DIR:-/tmp/gangline-$(id -u)}"
    printf 'N:           %s\n' "$PROBE_N"
    printf 'arms:        %s\n' "$PROBE_ARMS"
    printf 'hold:        %s\n' "$PROBE_HOLD"
  } | tee "$PROBE_OUT/cell.txt"
}

# --- helpers, copied rather than sourced --------------------------------------
#
# test/integration.sh is not a sourceable library, and copying is the right cost
# here for a second reason: adr9 is restructuring that file concurrently, and a
# probe whose rates moved because someone else refactored a helper would be
# measuring the refactor. These are pinned by being duplicated.
id_of() { # $1 = window NAME -> @id
  local id name
  while read -r id name; do
    [ "$name" = "$1" ] && { printf '%s' "$id"; return 0; }
  done < <(tmux list-windows -t "$GANG_SESSION" -F '#{window_id} #W')
  # Never fall through to an empty -t: that is tmux's CURRENT pane, and this
  # probe may be run from inside a live gangline session.
  printf 'BUG: no window named %s in %s\n' "$1" "$GANG_SESSION" >&2
  exit 1
}
target_of() {
  local id; id="$(id_of "$1")" || exit 1
  [ -n "$id" ] || { printf 'BUG: empty target for %s\n' "$1" >&2; exit 1; }
  printf '%s' "$id"
}
pane_of() { tmux capture-pane -pJ -t "$(id_of "$1")"; }
pane_raw() { tmux capture-pane -p -t "$(id_of "$1")"; }
has() { case "$(pane_of "$1")" in *"$2"*) echo yes ;; *) echo no ;; esac; }
contains() { case "$1" in *"$2"*) echo yes ;; *) echo no ;; esac; }
send_text() {
  local target="$1" from="$2"; shift 2
  printf '%s' "$*" | "$GANG" send "$target" --from "$from" --stdin
}
WAIT_TIMEOUT=30
wait_for() { # $1 = agent ('' for none), $2 = what, $3 = value ending the wait, $4.. = producer
  local who="$1" what="$2" want="$3" got deadline rc
  shift 3
  deadline=$(( $(date +%s) + WAIT_TIMEOUT ))
  while :; do
    rc=0; got="$("$@")" || rc=$?
    [ "$rc" -eq 0 ] || { printf 'probe: producer refused waiting for %s (exit %s)\n' "$what" "$rc" >&2; return "$rc"; }
    [ "$got" = "$want" ] && return 0
    [ "$(date +%s)" -lt "$deadline" ] || break
    sleep 0.05
  done
  printf 'probe: TIMEOUT waiting for %s (wanted [%s], got [%s])\n' "$what" "$want" "$got" >&2
  return 1
}

# --- capture, at failure and never during a wait ------------------------------
#
# Everything here runs AFTER a trial has already been scored, so none of it can
# move the timing it is reporting on. That ordering is the whole reason this is
# a function called from the red branch rather than a poll.
dump() { # $1 = arm, $2 = iteration, $3 = agent, $4.. = free-text reason
  local arm="$1" iter="$2" who="$3"; shift 3
  local d="$PROBE_OUT/$arm/$iter" win
  mkdir -p "$d"
  printf '%s\n' "$*" > "$d/reason.txt"
  win="$(id_of "$who" 2>/dev/null)" || win=""
  # If the caller already captured, those bytes STAY. Arm D evaluates its check
  # from the file it archived, and a second read here would replace the evidence
  # with a capture of a later screen than the one the check judged — the exact
  # TOCTOU that would let a red carry a pane not showing the failure.
  if [ -n "$win" ] && [ ! -f "$d/pane.joined" ]; then
    # Raw AND joined, because the difference between them IS one of the two
    # hypotheses: -J joins WRAPPED lines, so two separately-submitted messages
    # can be joined into one logical line that a merge assertion then matches
    # with no merge having occurred.
    tmux capture-pane -p  -t "$win" > "$d/pane.raw"    2>/dev/null
    tmux capture-pane -pJ -t "$win" > "$d/pane.joined" 2>/dev/null
    tmux display-message -p -t "$win" '#{pane_width}x#{pane_height}' > "$d/pane.size" 2>/dev/null
    tmux show-options -w -t "$win" > "$d/window.options" 2>/dev/null
  fi
  ls -laR "${GANG_LOCK_DIR:-/tmp/gangline-$(id -u)}" > "$d/lockdir.txt" 2>&1
  [ ! -f "$SHIM/jitter.log" ] || cp "$SHIM/jitter.log" "$d/jitter.log"
  date '+%Y-%m-%dT%H:%M:%S' > "$d/at.txt"
}

# --- the fixtures -------------------------------------------------------------
#
# One profile per arm, differing in profile_input and nothing else. That is the
# single variable this probe moves, and it is moved by CONSTRUCTION rather than
# by hoping a platform behaves differently.
write_jitter() { # $1 = arm name, $2 = kind (random|constant|counter), $3 = log (yes|no)
  local f="$SHIM/custom-profiles/$1.sh" logline=""
  # Bracketed, so an EMPTY read is a visible `[]` rather than a blank line that
  # reads as a logging failure. Plain redirection and nothing else: the mechanism
  # under observation is expansion-context-sensitive, so an instrument that added
  # a command substitution could create or destroy the very freeze it logs.
  [ "$3" != yes ] || logline="printf '[%s]\\n' \"\$v\" >> \"$SHIM/jitter.log\""
  {
    printf 'GANG_LAUNCH="PS1='\''> '\'' bash --norc"\n'
    printf 'GANG_BUSY_REGEX=""\n'
    printf 'GANG_VERIFIED_VERSIONS="any"\n'
    printf 'profile_input() {\n'
    printf '  local v\n'
    case "$2" in
      # The shipped fixture, unchanged. On bash 3.2 an in-process $RANDOM
      # expansion marks the subshell lineage as seeded, and every later
      # command-substituted read replays the same value — so this line is the
      # one under suspicion, and it is reproduced here byte-for-byte rather
      # than paraphrased.
      random)   printf '  v="being-typed-$RANDOM"\n' ;;
      # Static by construction: exactly the box a frozen $RANDOM produces. This
      # is the forced failure that proves the harness can red.
      constant) printf '  v="being-typed-CONSTANT"\n' ;;
      # The pattern already used by the retryable fixture in test/integration.sh
      # and documented there: fresh by construction under ANY bash.
      # The forced-failure drive. An empty box is settled (no hold), reads CLEAR
      # (so the paste proceeds), and reads empty AGAIN afterwards — so
      # landing_zone's before and after match and inject dies 1 with the paste
      # already on the screen. That is rc=1 AND the mark present: both of the
      # assertions this probe measures, red by construction. Arm B was specified
      # for this job and could not do it — a static box is correctly refused 3,
      # which is a real measurement rather than a failure.
      empty)    printf '  v=""\n' ;;
      counter)  printf '  local n\n'
                printf '  n="$(cat "%s/keystrokes" 2>/dev/null)" || n=0\n' "$SHIM"
                printf '  n="$((n + 1))"\n'
                printf '  printf %%s "$n" > "%s/keystrokes"\n' "$SHIM"
                printf '  v="being-typed-$n"\n' ;;
    esac
    [ -z "$logline" ] || printf '  %s\n' "$logline"
    printf '  printf %%s "$v"\n'
    printf '}\n'
  } > "$f"
}

# --- the units ----------------------------------------------------------------

# Cluster 2. One agent per ARM rather than per iteration: the hitch is not what
# is under test, the send is, and re-hitching 120 times would put minutes of
# harness cost inside the measurement for nothing.
unit_typist() { # $1 = arm, $2 = iteration -> "ok" or a failure word
  local arm="$1" iter="$2" out rc mark verdict=ok
  mark="MARK_INTERLEAVED_$iter"
  out="$(GANG_PROFILES="$SHIM/custom-profiles" GANG_SEND_HOLD="$PROBE_HOLD" \
    send_text "typist-$arm" tester "$mark" 2>&1)"; rc=$?
  # The two checks that actually go red in CI, asserted here exactly as
  # test/integration.sh asserts them. 3 rather than 1 is load-bearing: it is
  # the only thing separating a refusal that typed NOTHING from a failure that
  # may have left a paste in the box.
  [ "$rc" = 3 ] || verdict="rc=$rc"
  if [ "$(has "typist-$arm" "$mark")" = yes ]; then
    verdict="$verdict,typed-over"
  fi
  if [ "$verdict" != ok ]; then
    dump "$arm" "$iter" "typist-$arm" \
      "expected rc=3 and no paste; got rc=$rc, verdict=$verdict"
    # The FULL text gang printed, verbatim and in its own file. A status code
    # says which branch was taken; only the prose says what gang BELIEVED about
    # the box on the way there — whether a hold engaged, and what it read.
    printf '%s\n' "$out" > "$PROBE_OUT/$arm/$iter/out.txt"
  fi
  printf '%s' "$verdict"
}

# Cluster 1. Marks are unique PER ITERATION, which matters more here than
# anywhere else in this file: reusing one pair across 120 iterations would let
# iteration 40's A match iteration 39's B on a scrolled screen and report a
# merge that never happened. A probe that manufactures its own positives is
# worse than no probe.
unit_merge() { # $1 = iteration -> "ok" or a failure word
  local iter="$1" a b joined raw verdict=ok d win
  a="MARK_RACE_A_$iter"; b="MARK_RACE_B_$iter"
  send_text alpha tester "$a" >/dev/null 2>&1 &
  send_text alpha tester "$b" >/dev/null 2>&1 &
  wait
  wait_for alpha "the first of two concurrent deliveries" yes has alpha "$a" || verdict="lost-a"
  wait_for alpha "the second of two concurrent deliveries" yes has alpha "$b" || verdict="$verdict,lost-b"
  # Capture to FILES first, then run the check against those exact bytes. The
  # suite reads the pane live and a dump would read it again; between the two
  # the screen can move, so the archived capture would be evidence about a
  # different screen than the one the check judged.
  d="$PROBE_OUT/D/$iter"; mkdir -p "$d"
  win="$(id_of alpha 2>/dev/null)" || win=""
  tmux capture-pane -pJ -t "$win" > "$d/pane.joined" 2>/dev/null
  tmux capture-pane -p  -t "$win" > "$d/pane.raw"    2>/dev/null
  # -N keeps trailing spaces, and that is the only thing on the screen that says
  # whether a row ran to the exact right margin. Stripped, a row that wrapped
  # and a row that simply ended are the same bytes — and that distinction is
  # what tells a real merge from a join. Discovered by measuring -J directly:
  # two envelopes merged into ONE submission are ~116 columns and therefore WRAP,
  # so a genuine merge also reads raw=0. raw=0 was never the artifact's
  # signature; it is common to both, and this is the field that separates them.
  tmux capture-pane -pN -t "$win" > "$d/pane.rawN"   2>/dev/null
  joined="$(grep -c "$a.*$b" "$d/pane.joined")"
  raw="$(grep -c "$a.*$b" "$d/pane.raw")"
  if [ "$joined" = 0 ] && [ "$raw" = 0 ] && [ "$iter" != 1 ]; then
    # Greens keep nothing but the first specimen. Stated rather than silent:
    # iteration 1's pane is retained so a run of all-green can still be shown to
    # have had marks on the screen the check could have matched.
    rm -rf "$d"
  fi
  if [ "$joined" != 0 ] || [ "$raw" != 0 ]; then
    verdict="$verdict,merged(joined=$joined,raw=$raw)"
    dump D "$iter" alpha "merge pattern matched: joined=$joined raw=$raw"
  fi
  printf '%s' "${verdict#ok,}"
}

# The premise under arm D's whole reading, measured on the cell instead of
# assumed from mine. The suite's pane_of is `capture-pane -pJ` and the merge
# check greps its output, so whether -J welds two SEPARATELY submitted lines
# decides what a cluster-1 red MEANS. Measured on tmux 3.2a/Linux it does not
# weld them — not even when the first row ends exactly at the margin — which
# makes a joined match ground truth for "one continuous line", i.e. evidence OF
# a merge rather than an artifact mimicking one. That is a statement about one
# tmux build; the cell that actually fails runs another, and only this settles
# it there. `nonl` is the case that must read joined>=1: it is genuinely one
# continuous line, so a 0 there means this probe is not measuring what it thinks.
h2_primitive() {
  local name width nl a script raw joined cols out="$PROBE_OUT/h2-primitive.txt"
  : > "$out"
  while read -r name width nl; do
    [ -n "$name" ] || continue
    a="MARK_RACE_A"
    while [ "${#a}" -lt "$width" ]; do a="$a."; done
    a="${a:0:$width}"
    script="$SHIM/h2-$name.sh"
    {
      printf '#!/usr/bin/env bash\n'
      if [ "$nl" = yes ]; then printf "printf '%%s\\\\n' %q\n" "$a"
      else                     printf "printf '%%s' %q\n"     "$a"; fi
      printf "printf '%%s\\\\n' 'MARK_RACE_B-tail'\n"
      printf 'sleep 600\n'
    } > "$script"
    chmod +x "$script"
    tmux kill-session -t "$GANG_SESSION-h2" 2>/dev/null
    tmux new-session -d -s "$GANG_SESSION-h2" -x 80 -y 24 "$script"
    sleep 1
    # The width is READ, never assumed: if -x were not honoured every case would
    # be mislabelled and the whole table would read as a tmux difference.
    cols="$(tmux display-message -p -t "$GANG_SESSION-h2" '#{pane_width}' 2>/dev/null)"
    raw="$(tmux capture-pane -p  -t "$GANG_SESSION-h2" 2>/dev/null \
      | grep -c 'MARK_RACE_A.*MARK_RACE_B')"
    joined="$(tmux capture-pane -pJ -t "$GANG_SESSION-h2" 2>/dev/null \
      | grep -c 'MARK_RACE_A.*MARK_RACE_B')"
    tmux kill-session -t "$GANG_SESSION-h2" 2>/dev/null
    printf '%-6s A=%-3s newline=%-4s pane=%-4s raw=%s joined=%s\n' \
      "$name" "$width" "$nl" "${cols:-?}" "$raw" "$joined" >> "$out"
  done <<'SPECS'
under 79 yes
exact 80 yes
over  90 yes
nonl  80 no
SPECS
  printf -- '--- -J primitive (tmux %s) ---\n' "$(tmux -V)"
  cat "$out"
}

# --- the run ------------------------------------------------------------------

fingerprint_cell
h2_primitive
# No `gang up` and no `gang down` anywhere in this file, deliberately. hitch
# creates the session on its own — test/integration.sh relies on exactly that —
# and teardown is the EXIT trap's kill-session aimed at $GANG_SESSION BY NAME.
# An unaimed team-level verb run from a probe is how a measurement ends
# somebody's work, including work it cannot see.

reds=0; trials=0
summary=""
# Arm E is the forced-failure drive, and this records not merely THAT it red but
# that it red in the SHAPE the CI failure has: status 1 and a paste on the
# screen. An arm that red for some third reason would prove the harness can
# error, which is not the same as proving it can detect this failure.
e_both=0
for arm in $PROBE_ARMS; do
  armreds=0
  case "$arm" in
    A)      write_jitter A      random   yes ;;
    Anolog) write_jitter Anolog random   no  ;;
    B)      write_jitter B      constant yes ;;
    C)      write_jitter C      counter  yes ;;
    D)      : ;;
    E)      write_jitter E      empty    yes ;;
  esac
  if [ "$arm" = D ]; then
    "$GANG" hitch alpha -p bash -d /tmp >/dev/null 2>&1
  else
    rm -f "$SHIM/jitter.log" "$SHIM/keystrokes"
    GANG_PROFILES="$SHIM/custom-profiles" \
      "$GANG" hitch "typist-$arm" -p "$arm" -d /tmp >/dev/null 2>&1
  fi
  i=0
  while [ "$i" -lt "$PROBE_N" ]; do
    i=$((i + 1))
    if [ "$arm" = D ]; then v="$(unit_merge "$i")"; else v="$(unit_typist "$arm" "$i")"; fi
    trials=$((trials + 1))
    if [ "$v" != ok ]; then
      armreds=$((armreds + 1)); reds=$((reds + 1))
      printf 'RED  arm=%s iter=%s %s\n' "$arm" "$i" "$v"
      if [ "$arm" = E ]; then
        case "$v" in *rc=1*) case "$v" in *typed-over*) e_both=1 ;; esac ;; esac
      fi
    fi
  done
  if [ "$arm" = D ]; then "$GANG" drop alpha >/dev/null 2>&1; else "$GANG" drop "typist-$arm" >/dev/null 2>&1; fi
  # The jitter log is the mechanism OBSERVED rather than inferred: a repeated
  # value is a frozen box, a changing one is not. Kept per arm, whatever the
  # rate was, because a GREEN arm whose values never changed would mean the
  # unit stopped testing what it names.
  if [ -f "$SHIM/jitter.log" ]; then
    mkdir -p "$PROBE_OUT/$arm"
    cp "$SHIM/jitter.log" "$PROBE_OUT/$arm/jitter.log"
    printf '  arm %s jitter: %s reads, %s distinct\n' "$arm" \
      "$(wc -l < "$SHIM/jitter.log" | tr -d ' ')" \
      "$(sort -u < "$SHIM/jitter.log" | wc -l | tr -d ' ')"
  fi
  summary="$summary$(printf '\n  arm %-7s %s/%s red' "$arm" "$armreds" "$PROBE_N")"
done

printf '\n--- probe summary ---%s\n' "$summary"
printf '  total %s/%s\n' "$reds" "$trials"
# This script's exit status is NOT a pass/fail verdict and must not be read as
# one: reds are the measurement. It exits non-zero only when the forced-failure
# arm failed to fail, because that is the one outcome that voids every other
# number printed above.
if case " $PROBE_ARMS " in *" E "*) true ;; *) false ;; esac; then
  if [ "$e_both" -ne 1 ]; then
    printf 'VOID: arm E (forced failure) never produced rc=1 WITH a paste on the screen.\n' >&2
    printf '      That pair is the CI signature this probe claims to detect, so until E\n' >&2
    printf '      shows it, no rate above is evidence of anything.\n' >&2
    exit 1
  fi
fi
