#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# OPT-IN END-TO-END LANE. Not part of test/gate.sh and not part of CI.
#
# Everything else in test/ proves Gangline against fixtures: a shell pretending
# to be a harness, a fake clock, a pane whose every transition is synchronous.
# That is the right trade for a mandatory gate, and it leaves one thing
# unproven — that the collar, the pane regexes, the native hooks and the turn
# bracket still describe the harness that is actually installed.
#
# This lane closes that gap by booting a REAL claude-code TUI in a disposable
# tmux session and pointing it at test/e2e/stub.py instead of a provider. No
# network, no account, no cost. The stub's request log is the instrument: it
# says what entered the model's context, which is the only honest way to prove
# that a message Gangline reported as delivered was in fact delivered.
#
#   test/e2e.sh            run every scenario
#   test/e2e.sh boot       run one by name
#
# WHY THIS IS NOT MANDATORY. A real TUI boot costs seconds, and the lane holds
# one turn open on purpose. test/lint.sh refuses wall time in the mandatory
# suite and names this file as the one exemption; the waits below are bounded,
# fail loudly, and print their measured margins.
set -euo pipefail

# INSIDE AN AGENT WINDOW $TMUX IS SET, and tmux then ignores TMUX_TMPDIR
# without saying so, which would put this lane's disposable session on the live
# server next to a real team. test/integration.sh does the same on its seventh
# line and for the same reason.
unset TMUX TMUX_PANE

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
GANG="$ROOT/bin/gang"
STUB="$ROOT/test/e2e/stub.py"

# ONE HARNESS AT A TIME. A real claude-code process is heavy enough that two
# lanes, or this lane beside the mandatory suite's tmux server, can starve the
# box. `-o` closes the lock descriptor before exec, so the tmux server this
# lane starts does not inherit it and cannot hold the lock after the run ends:
# an inherited flock descriptor wedges the heavy lock until that server exits.
HEAVY_LOCK="${GANG_E2E_LOCK:-/tmp/gangline-heavy.lock}"
if [ "${GANG_E2E_LOCKED:-0}" != 1 ]; then
  command -v flock >/dev/null 2>&1 \
    || { echo "e2e: flock is required to serialize a real harness run" >&2; exit 1; }
  export GANG_E2E_LOCKED=1
  echo "e2e: taking $HEAVY_LOCK — one harness at a time, so this waits on the gate" >&2
  exec flock -o "$HEAVY_LOCK" "$0" "$@"
fi

command -v claude >/dev/null 2>&1 \
  || { echo "e2e: claude is not installed, so there is no harness to drive" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 \
  || { echo "e2e: python3 cannot run here, so the stub server cannot start" >&2; exit 1; }

HARNESS_BUILD="$(claude --version 2>/dev/null || echo unknown)"

checks=0
fails=0
pass() { checks=$((checks + 1)); printf 'ok   %s\n' "$1"; }
fail() {
  checks=$((checks + 1))
  fails=$((fails + 1))
  printf 'FAIL %s\n       %s\n' "$1" "$2"
}
equal() {
  if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "expected [$2], got [$3]"; fi
}
contains() {
  case "$2" in
    *"$3"*) pass "$1" ;;
    *) fail "$1" "missing [$3]" ;;
  esac
}
note() { printf '     %s\n' "$1"; }

# BOUNDED, NOT PATIENT. Every wait in this lane either rides an event barrier
# or comes through here, where it has a hard read budget and says so when it
# misses. The reads are cheap tmux/file state, and the margin each scenario
# actually used is printed at the end of the run, because a bound nobody
# measures is only permission to be timing-dependent.
E2E_READS="${GANG_E2E_READS:-200}"
E2E_NAP="${GANG_E2E_NAP:-0.05}"
margins=""
settle() { # $1 label, rest: predicate command -> 0 when it holds
  local label="$1" reads=0
  shift
  while [ "$reads" -lt "$E2E_READS" ]; do
    if "$@"; then
      margins="${margins}${margins:+
}  $label: $reads of $E2E_READS reads"
      return 0
    fi
    reads=$((reads + 1))
    sleep "$E2E_NAP"
  done
  margins="${margins}${margins:+
}  $label: EXHAUSTED $E2E_READS reads"
  return 1
}

RUN_ROOT=""
STUB_PID=""

teardown() {
  [ -z "$STUB_PID" ] || kill "$STUB_PID" 2>/dev/null || true
  if [ -n "${TMUX_SOCKET:-}" ]; then
    tmux -S "$TMUX_SOCKET" kill-server 2>/dev/null || true
  fi
  # THE HARNESS OUTLIVES ITS PANE BY A MOMENT. A killed claude is still
  # flushing its own config directory when the window is already gone, so the
  # first removal can lose that race and take the run's exit status with it —
  # which is how a fully green scenario first reported failure here. Retry,
  # and if the directory still will not go, say so by name instead of leaving
  # it behind silently.
  if [ -n "$RUN_ROOT" ] && ! rm -rf -- "$RUN_ROOT" 2>/dev/null; then
    settle "teardown" rm -rf -- "$RUN_ROOT" \
      || printf 'e2e: COULD NOT REMOVE %s — delete it by hand\n' "$RUN_ROOT" >&2
  fi
  RUN_ROOT=""
  STUB_PID=""
}
trap teardown EXIT HUP INT TERM

AGENT=dog
HOLD_MARKER=GANGLINE-E2E-HOLD
ERROR_MARKER=GANGLINE-E2E-ERROR
REPLY_PREFIX=E2E-STUB-REPLY
# Fixed and obviously fake. The stub never reads it; the harness only needs one
# it has been told to trust, and a stable value keeps that record reproducible.
STUB_API_KEY=sk-ant-e2e-stub-0000000000000000

# A FRESH WORLD PER SCENARIO: its own stub, its own request log, its own
# claude config, its own tmux server, its own session name. Nothing a scenario
# leaves behind can be read by the next one, so a green scenario is green on
# its own evidence.
world_up() {
  RUN_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-e2e.XXXXXX")"
  REQ_LOG="$RUN_ROOT/requests.jsonl"
  GATE="$RUN_ROOT/gate"
  HELD="$RUN_ROOT/held"
  WORK="$RUN_ROOT/work"
  mkdir -p "$WORK"
  mkfifo "$GATE" "$HELD"
  : > "$REQ_LOG"

  # THE SENTINEL THAT TELLS THE AGENT'S TURN FROM THE HARNESS'S ERRANDS. The
  # collar passes CONTRACT.md through --append-system-prompt, so every request
  # the hitched agent makes carries the contract's own heading and none of the
  # harness's auxiliary completions do. Taken from the file rather than typed
  # here, so a reworded contract breaks this loudly instead of silently making
  # every request look like a side errand.
  TURN_SENTINEL="$(head -1 "$ROOT/CONTRACT.md" | sed 's/^#* *//')"
  [ -n "$TURN_SENTINEL" ] \
    || { echo "e2e: CONTRACT.md has no heading to identify an agent turn by" >&2; return 1; }
  python3 "$STUB" --log "$REQ_LOG" --port-file "$RUN_ROOT/port" \
    --gate "$GATE" --held "$HELD" --turn-sentinel "$TURN_SENTINEL" \
    >"$RUN_ROOT/stub.out" 2>"$RUN_ROOT/stub.err" &
  STUB_PID=$!
  settle "stub bind" test -s "$RUN_ROOT/port" \
    || { echo "e2e: stub never bound a port" >&2; return 1; }
  PORT="$(cat "$RUN_ROOT/port")"

  # A COLD CLAUDE CONFIG DIRECTORY DRAWS ONBOARDING, not a composer: the
  # claude-code collar enumerates the five pre-session frames it can draw, and
  # none of them accepts delivery. These two files are the whole difference
  # between a harness that boots to a prompt and a lane that hangs on a theme
  # picker. The project entry answers the trust dialog for this run's working
  # directory, which is empty and disposable.
  mkdir -p "$RUN_ROOT/claude"
  python3 - "$RUN_ROOT/claude/.claude.json" "$WORK" "$STUB_API_KEY" <<'PY'
import json
import sys

path, work, key = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path, "w", encoding="utf-8") as handle:
    json.dump(
        {
            "hasCompletedOnboarding": True,
            "theme": "dark",
            "autoUpdates": False,
            "installMethod": "native",
            # A KEY IN THE ENVIRONMENT IS A FIRST-RUN GATE OF ITS OWN, and not
            # one the collar can answer: the harness draws a two-choice
            # approval box whose default is No, hitch correctly reports a
            # native prompt, and the lane waits forever for an operator who
            # does not exist. The harness identifies a key by its last twenty
            # characters, so the answer is recorded here the same way.
            "customApiKeyResponses": {"approved": [key[-20:]], "rejected": []},
            "projects": {
                work: {
                    "hasTrustDialogAccepted": True,
                    "hasCompletedProjectOnboarding": True,
                    "allowedTools": [],
                    "history": [],
                }
            },
        },
        handle,
    )
PY
  printf '%s\n' '{"theme":"dark","includeCoAuthoredBy":false}' \
    > "$RUN_ROOT/claude/settings.json"

  TMUX_SOCKET="$RUN_ROOT/tmux-$(id -u)/default"
  export TMUX_TMPDIR="$RUN_ROOT"
  export GANG_CONFIG_DIR="$RUN_ROOT/config"
  export GANG_LOCK_DIR="$RUN_ROOT/locks"
  export GANG_ARCHIVE_DIR="$RUN_ROOT/archive"
  export GANG_SESSION="gangline-e2e-$$"

  # The tmux server this lane starts inherits these, so the harness launched
  # into one of its windows reaches the stub and nothing else.
  export CLAUDE_CONFIG_DIR="$RUN_ROOT/claude"
  export ANTHROPIC_BASE_URL="http://127.0.0.1:$PORT"
  export ANTHROPIC_API_KEY="$STUB_API_KEY"
  export DISABLE_TELEMETRY=1
  export DISABLE_ERROR_REPORTING=1
  export DISABLE_AUTOUPDATER=1
  export DISABLE_BUG_COMMAND=1
  export DISABLE_NON_ESSENTIAL_MODEL_CALLS=1
  export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
}

world_down() { teardown; }

# EVERY LAUNCH CHOICE IS MADE HERE. hitch warns when a supported choice is
# omitted and lets the collar pick, and a lane that took a silent default would
# be proving Gangline against whatever that default happened to be that month.
#
# BOUNDED, BECAUSE HITCH IS ALLOWED TO WAIT FOREVER. Facing a native first-run
# prompt it correctly declines to answer for the operator and keeps waiting for
# a composer, which is right at a keyboard and a hang in an unattended lane.
# Calibration produced exactly that: a mutated collar made the harness draw a
# settings-error dialog, and the run sat on it until an outer timeout killed
# it. A lane that hangs reports nothing, so this fails instead — and prints the
# frame that stopped it, since the whole question is which prompt appeared.
E2E_HITCH_LIMIT="${GANG_E2E_HITCH_LIMIT:-120}"
hitch_agent() {
  timeout "$E2E_HITCH_LIMIT" "$GANG" hitch "$AGENT" -c claude-code -d "$WORK" \
    -m "${GANG_E2E_MODEL:-claude-sonnet-4-5}" -e "${GANG_E2E_EFFORT:-low}"
}

# Each scenario begins the same way and none of them can continue without it.
booted() { # 0 when a real agent is hitched and at rest, 1 with the reason said
  local rc=0
  hitch_agent >"$RUN_ROOT/hitch.out" 2>&1 || rc=$?
  if [ "$rc" -ne 0 ]; then
    fail "$1: hitch never reached a composer (status $rc)" "$(tail -3 "$RUN_ROOT/hitch.out")"
    note "$("$GANG" capture "$AGENT" 40 2>&1 | tail -12)"
    return 1
  fi
  settle "$1 idle" is_state idle || true
  return 0
}

state() { # current porcelain state word for the agent
  "$GANG" roster --porcelain 2>/dev/null | awk -F'\t' -v n="$AGENT" '$1 == n { print $3 }'
}

is_state() { [ "$(state)" = "$1" ]; }

say() { # $1 prompt text -> submitted as the operator
  printf '%s' "$1" | "$GANG" send --to "$AGENT" --from operator --stdin
}

# THE INSTRUMENT. Everything the harness asked the model for, in order. A
# scenario asserts against this rather than against the screen, because a pane
# shows what was drawn and this shows what was actually sent.
requests() { python3 -c '
import json, sys
for line in open(sys.argv[1], encoding="utf-8"):
    line = line.strip()
    if line:
        print(json.loads(line)["text"].replace("\n", " "))
' "$REQ_LOG"; }

request_count() { grep -c '"path": "/v1/messages"' "$REQ_LOG" 2>/dev/null || true; }

# The stub opens the held FIFO for writing the instant it starts holding a
# response, so reading it returns exactly when the turn is live on screen. No
# poll, no sleep: the barrier is the turn itself.
#
# BOUNDED, BECAUSE A BARRIER NOBODY REACHES IS A HANG. If the harness stops
# making a request this stub recognises as the agent's own turn, the read below
# would otherwise block forever and the lane would look wedged rather than
# broken. The bound is generous against a measured sub-second arrival.
E2E_HOLD_LIMIT="${GANG_E2E_HOLD_LIMIT:-60}"
await_held() {
  timeout "$E2E_HOLD_LIMIT" cat "$HELD" >/dev/null && return 0
  fail "the turn never reached the stub" \
    "no request matched the hold marker and the agent-turn sentinel in ${E2E_HOLD_LIMIT}s"
  return 1
}
release_held() { : > "$GATE"; }

# ---------------------------------------------------------------- scenario 1
# BOOT REACHES A COMPOSER AND GANGLINE SEES IT. hitch does not return until it
# has delivered the startup contract into a box it could read, so its own exit
# status is the first half of the proof; the request log is the second, and the
# stronger one — the contract did not merely reach a box, it reached the model.
scenario_boot() {
  world_up
  if ! booted boot; then world_down; return; fi
  pass "boot: hitch delivered a startup contract to a real claude-code"
  equal "boot: the booted agent reads idle" idle "$(state)"

  local sent
  sent="$(requests)"
  contains "boot: the standing contract reached the model" \
    "$sent" "Gangline contract"
  contains "boot: the agent was told its own name" "$sent" "You are $AGENT in Gangline"
  world_down
}

# ---------------------------------------------------------------- scenario 2
# A NATIVE Stop IS WHAT RELEASES a gang wait for the done condition. The turn is held open
# by the stub, so the wait is armed against a turn that provably cannot have
# finished yet; the release then happens only after tmux has stored the
# caller's channel. What returns the wait is therefore the harness's own Stop
# hook and nothing else.
scenario_turn() {
  world_up
  if ! booted turn; then world_down; return; fi

  say "$HOLD_MARKER please hold" >/dev/null
  await_held
  equal "turn: a live turn reads busy" busy "$(state)"

  local wait_rc=0
  "$GANG" wait "$AGENT" --until "done" --timeout 120 >"$RUN_ROOT/wait.out" 2>&1 &
  local wait_pid=$!

  settle "wait armed" wait_armed || {
    fail "turn: gang wait never armed a boundary" "no channel appeared in tmux hooks"
    kill "$wait_pid" 2>/dev/null || true
    release_held
    world_down
    return
  }
  # The wait is armed and the turn is still held, so nothing that has already
  # happened could have released it.
  if kill -0 "$wait_pid" 2>/dev/null; then
    pass "turn: the wait is still blocked while the turn is held"
  else
    fail "turn: the wait returned before the turn ended" \
      "it cannot have been released by this turn's Stop"
  fi

  release_held
  wait "$wait_pid" || wait_rc=$?
  equal "turn: the done wait returned on the native Stop" 0 "$wait_rc"
  equal "turn: the agent reads idle after its turn" idle "$(state)"
  # The reply prefix exists only in this stub's completions. The lane never
  # types it — not in a prompt, not in an envelope, not in the contract — so no
  # keystroke this suite makes can put it on the pane, and the only route from
  # the stub to this surface is the harness rendering the answer.
  # source-guard: whole-surface@6386eeff5e7a: only the stub emits this prefix and the lane never types it, so any producer that put it on the pane is the harness rendering a completion
  contains "turn: the harness rendered the stub's completion" \
    "$("$GANG" capture "$AGENT" 200)" "$REPLY_PREFIX"
  world_down
}

wait_armed() { # the caller's channel is stored in a tmux pane-exited hook
  tmux show-hooks -g pane-exited 2>/dev/null | grep -q 'wait-for -S gang-wait-'
}

# ---------------------------------------------------------------- scenario 3
# A FATAL TURN IS SEEN AS FATAL. collar_bricked reads the harness's own
# transcript for a synthetic assistant record carrying error=model_not_found,
# and matches its message against an exact sentence. Both halves of that
# declaration are unprovable against a fixture: only a real claude-code writes
# that record, and only a provider answering 404 makes it write one. The stub
# is that provider.
#
# THE MESSAGE SHAPE IS THE FRAGILE HALF and this is the only place it is
# checked end to end. The reader returns `unknown` rather than `bricked` when
# the sentence drifts, so a drift shows up here as a wrong state and not as
# silence — which is why the reading is asserted exactly rather than accepted
# from a list.
scenario_bricked() {
  world_up
  if ! booted bricked; then world_down; return; fi

  say "$ERROR_MARKER now" >/dev/null
  settle "bricked classified" is_state bricked || true
  local reading
  reading="$(state)"
  equal "bricked: a fatal provider answer reads bricked" bricked "$reading"
  if [ "$reading" != bricked ]; then
    note "RECORDED on $HARNESS_BUILD: it read '$reading' instead."
    note "$("$GANG" explain "$AGENT" 2>&1 | tail -6)"
  fi

  # THE FATAL CAUSE, NOT MERELY THE WORDS model_not_found. Both verdicts the
  # reader can reach mention that name — the fatal one and the "unrecognized
  # message shape" it falls back to when the sentence drifts — so a check for
  # the name alone survives a broken reader. Calibration caught exactly that:
  # with the message pattern mutated, this assertion passed while the state
  # assertion above failed. The rejection wording belongs only to the fatal
  # branch.
  contains "bricked: the cause is the fatal rejection, not an unreadable record" \
    "$("$GANG" status "$AGENT" 2>&1)" "was rejected (model_not_found)"
  # A fatal turn cannot reach a boundary, and a wait that blocked on one would
  # hang until its deadline. gang refuses instead, which is the property that
  # keeps this state useful to a caller.
  local wait_rc=0
  "$GANG" wait "$AGENT" --until "done" --timeout 5 >"$RUN_ROOT/wait.out" 2>&1 || wait_rc=$?
  if [ "$wait_rc" -eq 0 ]; then
    fail "bricked: gang wait should refuse a fatal turn" "it returned 0"
  else
    contains "bricked: gang wait refuses a fatal turn by name" \
      "$(cat "$RUN_ROOT/wait.out")" "fatal turn"
  fi
  world_down
}

# ---------------------------------------------------------------- scenario 4
# MID-TURN ATTRIBUTED DELIVERY REACHES THE MODEL. The collar declares
# GANG_MIDTURN_INPUT=steer, so an envelope arriving while a turn is live is
# committed to the attributed spool and then typed into the composer. Screen
# evidence would only show that it was typed. The request log shows that the
# harness sent it to the model on the following turn, with its attribution
# intact, which is the property the whole system exists to provide.
scenario_midturn() {
  world_up
  if ! booted midturn; then world_down; return; fi

  say "$HOLD_MARKER hold for the courier" >/dev/null
  await_held
  equal "midturn: the turn is live" busy "$(state)"

  local send_rc=0 token=E2E-COURIER-TOKEN
  printf 'read this: %s' "$token" \
    | "$GANG" send --to "$AGENT" --from courier --stdin >"$RUN_ROOT/send.out" 2>&1 \
    || send_rc=$?
  equal "midturn: gang accepted a send into a live turn" 0 "$send_rc"
  [ "$send_rc" -eq 0 ] || note "$(tail -3 "$RUN_ROOT/send.out")"
  # WHICH PATH IT TOOK IS THE POINT. A collar that declared no mid-turn input
  # would refuse this send outright, and one declaring the direct path would
  # type before anything owned the envelope. Only the steer path reports
  # landing out of the attributed spool, so that wording is what separates the
  # behaviour under test from the two ways of not having it.
  contains "midturn: it landed through the collar's declared mid-turn input" \
    "$(cat "$RUN_ROOT/send.out")" "collar-declared mid-turn input"

  release_held
  local rc=0
  "$GANG" wait "$AGENT" --until "done" --timeout 120 >/dev/null 2>&1 || rc=$?
  equal "midturn: the held turn reached its boundary once released" 0 "$rc"
  # The held turn ends first; the steered envelope becomes the turn after it.
  settle "midturn envelope sent" envelope_in_requests "$token" || true

  local sent
  sent="$(requests)"
  contains "midturn: the envelope reached the model" "$sent" "$token"
  contains "midturn: it arrived attributed to its sender" "$sent" "gang:courier"
  # THE ENVELOPE'S OWN TURN HAS TO FINISH TOO, and this is what catches a lane
  # that asserted on a request the harness was still blocked on: the request
  # log records a request when it ARRIVES, so a permanently held turn would
  # satisfy every check above it while leaving the agent stuck.
  settle "midturn settled" is_state idle || true
  equal "midturn: the agent came back to rest afterwards" idle "$(state)"
  world_down
}

envelope_in_requests() { requests | grep -q -- "$1"; }

# ----------------------------------------------------------------------------
SCENARIOS="boot turn bricked midturn"

run() { # $1 scenario name, validated against the list above before it is called
  case " $SCENARIOS " in
    *" $1 "*) ;;
    *) echo "e2e: no scenario named '$1' (have: $SCENARIOS)" >&2; exit 2 ;;
  esac
  "scenario_$1"
}

chosen="$SCENARIOS"
[ "$#" -eq 0 ] || chosen="$*"
for scenario in $chosen; do
  run "$scenario"
done

printf '\n%s checks in %ss against %s\n' "$checks" "$SECONDS" "$HARNESS_BUILD"
if [ -n "$margins" ]; then
  printf 'bounded waits, reads used of %s at %ss each:\n%s\n' \
    "$E2E_READS" "$E2E_NAP" "$margins"
fi
[ "$fails" -eq 0 ]
