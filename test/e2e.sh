#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# ISOLATED END-TO-END LANE. Not part of test/gate.sh or push/PR CI; locally it
# runs only when chosen, while scheduled CI drives it once daily and on dispatch.
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
# The summary line this lane ends on. Shared with the mandatory suite, which is
# where the fixture that drives its failing branch lives.
. "$ROOT/test/suite-tail.sh"
E2E_ARTIFACT_DIR="${GANG_E2E_ARTIFACT_DIR:-}"
case "$E2E_ARTIFACT_DIR" in
  '') E2E_ARTIFACT_RUN="" ;;
  /*) E2E_ARTIFACT_RUN="$E2E_ARTIFACT_DIR/run-$$" ;;
  *) echo "e2e: GANG_E2E_ARTIFACT_DIR must be an absolute path" >&2; exit 2 ;;
esac
if [ -n "$E2E_ARTIFACT_RUN" ]; then
  mkdir -p -- "$E2E_ARTIFACT_RUN" \
    || { echo "e2e: cannot create diagnostic root $E2E_ARTIFACT_RUN" >&2; exit 1; }
fi

# ONE HARNESS AT A TIME. A real claude-code process is heavy enough that two
# lanes, or this lane beside the mandatory suite's tmux server, can starve the
# box. `-o` closes the lock descriptor before exec, so the tmux server this
# lane starts does not inherit it and cannot hold the lock after the run ends:
# an inherited flock descriptor wedges the heavy lock until that server exits.
#
# THE LOCK FILE IS A PERMANENT SHARED INODE, created on first use and never
# unlinked. Removing it after unlocking is the classic two-inode race: a waiter
# already blocked on the old inode is holding a lock nobody else can see, and
# the next run locks a fresh file and starts beside it.
#
# BOUNDED, LIKE EVERY OTHER WAIT HERE. A stale holder — the gate's tmux server
# outliving a killed run has done exactly this — would otherwise park an
# unattended lane forever before a single trap exists to clean up after it.
HEAVY_LOCK="${GANG_E2E_LOCK:-/tmp/gangline-heavy.lock}"
E2E_LOCK_WAIT="${GANG_E2E_LOCK_WAIT:-900}"
if [ "${GANG_E2E_LOCKED:-0}" != 1 ]; then
  command -v flock >/dev/null 2>&1 \
    || { echo "e2e: flock is required to serialize a real harness run" >&2; exit 1; }
  export GANG_E2E_LOCKED=1
  echo "e2e: taking $HEAVY_LOCK — one harness at a time, so this waits on the gate" >&2
  exec flock -o -w "$E2E_LOCK_WAIT" "$HEAVY_LOCK" "$0" "$@" || {
    echo "e2e: $HEAVY_LOCK was held for ${E2E_LOCK_WAIT}s — find the holder with: fuser -v $HEAVY_LOCK" >&2
    exit 1
  }
fi

command -v claude >/dev/null 2>&1 \
  || { echo "e2e: claude is not installed, so there is no harness to drive" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 \
  || { echo "e2e: python3 cannot run here, so the stub server cannot start" >&2; exit 1; }
# Teardown proves the harness has left before it removes the world underneath
# it, and a lane that cannot prove that reports a clean exit it never checked.
[ -d /proc ] \
  || { echo "e2e: /proc is required to tell a departed harness from a leaked one" >&2; exit 1; }

# THE BUILD UNDER TEST IS PART OF THE RESULT. Every marker this lane exercises
# belongs to a specific claude-code, so a run that cannot name the build it drove
# proves nothing that can be recorded — and an unknown is not a pass.
HARNESS_BUILD="$(claude --version 2>/dev/null)" && [ -n "$HARNESS_BUILD" ] \
  || { echo "e2e: claude could not report its version, so this run could not be attributed to a build" >&2; exit 1; }

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
excludes() {
  case "$2" in
    *"$3"*) fail "$1" "unexpected [$3]" ;;
    *) pass "$1" ;;
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

# A BUDGET NOBODY SPENDS IS THE ONLY HONEST ONE. Exhausting the reads means the
# lane waited its whole allowance and the thing still had not happened; the
# assertion that follows may then pass anyway, because the last read's nap gave
# the world one more moment. That is how a documented bound quietly becomes
# informational, so exhaustion is a failure here in its own right and the
# following assertion stays only to say what was true when it gave up.
settled() {
  settle "$@" || fail "$1: the bounded wait exhausted its budget" \
    "$E2E_READS reads at ${E2E_NAP}s each were not enough"
}

RUN_ROOT=""
STUB_PID=""
WORLD_NAME=""
leaked=0
artifact_failed=0

preserve_world() {
  [ -n "$E2E_ARTIFACT_RUN" ] || return 0
  local dest file geometry
  dest="$E2E_ARTIFACT_RUN/$WORLD_NAME"
  mkdir -p -- "$dest" || return 1

  if ! timeout -k 2 10 "$GANG" capture "$AGENT" 200 \
    >"$dest/pane.txt" 2>&1; then
    printf '%s\n' 'e2e: pane capture was unavailable at teardown' \
      >>"$dest/pane.txt" || return 1
  fi
  geometry="$(timeout -k 2 10 tmux display-message -p \
    -t "$GANG_SESSION:$AGENT.0" '#{pane_width}x#{pane_height}' \
    2>/dev/null || printf unavailable)"
  printf 'scenario=%s\nharness=%s\ntmux=%s\npane=%s\n' \
    "$WORLD_NAME" "$HARNESS_BUILD" "$(tmux -V)" "$geometry" \
    >"$dest/environment.txt" || return 1

  for file in requests.jsonl stub.out stub.err hitch.out wait.out send.out; do
    [ ! -f "$RUN_ROOT/$file" ] || cp -- "$RUN_ROOT/$file" "$dest/$file" \
      || return 1
  done
  if [ -d "$RUN_ROOT/claude/projects" ]; then
    cp -R -- "$RUN_ROOT/claude/projects" "$dest/transcripts" || return 1
  fi
  return 0
}

teardown() {
  if [ -n "$RUN_ROOT" ] && ! preserve_world; then
    printf 'e2e: could not preserve diagnostics under %s\n' \
      "$E2E_ARTIFACT_RUN" >&2
    artifact_failed=1
  fi
  if [ -n "$STUB_PID" ] && kill "$STUB_PID" 2>/dev/null; then
    settle "stub exit" not_running "$STUB_PID" || leaked=1
  fi
  if [ -n "${TMUX_SOCKET:-}" ] && [ -S "$TMUX_SOCKET" ]; then
    # END THE AGENT BEFORE ASKING FOR THE SERVER. Gangline's tmux guard refuses
    # any kill-server aimed at a socket carrying a window with @gl_agent set,
    # and it is right to: those are live agents, whoever started them. This lane
    # hitches one, so its own teardown was refused, and because the status below
    # was thrown away the refusal surfaced only as a leak — a tmux server and a
    # live harness per run, with nothing in the report naming the cause. Drop
    # what this lane hitched and the refusal is untrue rather than unenforced;
    # the last window going takes the server with it, and the kill-server below
    # is then the ordinary belt-and-braces on a socket that carries nobody.
    # A SWALLOWED kill-server IS INDISTINGUISHABLE FROM A SERVER THAT NEVER
    # DIED, and this lane runs a real harness inside that server. Ask the socket
    # rather than trusting the exit status of the command that was supposed to
    # close it — and keep what that command said, because when the socket then
    # disagrees its message is the only account of why.
    local kill_said="" kill_rc=0
    "$GANG" drop "$AGENT" >/dev/null 2>&1 || true
    if ! server_gone; then
      kill_said="$(tmux -S "$TMUX_SOCKET" kill-server 2>&1)" || kill_rc=$?
    fi
    if ! settle "tmux exit" server_gone; then
      leaked=1
      printf 'e2e: kill-server exited %s and said: %s\n' \
        "$kill_rc" "${kill_said:-nothing}" >&2
    fi
  fi
  # THE HARNESS OUTLIVES ITS PANE, AND REMOVAL IS NOT THE TEST. A killed claude
  # is still flushing its own config directory after the window is gone, so a
  # removal can succeed and the harness then put the directory back — which it
  # did: roots holding nothing but claude/projects outlived runs that reported a
  # clean teardown, and nothing noticed. Wait for the harness to be gone first,
  # then remove, then say by name what is left if anything is.
  if [ -n "$RUN_ROOT" ]; then
    settle "harness exit" harness_gone || leaked=1
    rm -rf -- "$RUN_ROOT" 2>/dev/null || settle "teardown" rm -rf -- "$RUN_ROOT" || true
    if [ -e "$RUN_ROOT" ]; then
      printf 'e2e: COULD NOT REMOVE %s — delete it by hand\n' "$RUN_ROOT" >&2
      leaked=1
    fi
  fi
  RUN_ROOT=""
  STUB_PID=""
  WORLD_NAME=""
}

not_running() { ! kill -0 "$1" 2>/dev/null; }
server_gone() { ! tmux -S "$TMUX_SOCKET" list-sessions >/dev/null 2>&1; }
# NOTHING IS STILL STANDING IN THIS RUN'S WORLD. The harness is the only process
# launched with a working directory inside the temp root — the lane and every
# gang it runs stay in the checkout — so an open cwd there is the harness itself,
# asked directly rather than inferred from the exit of the terminal it was drawn
# in. A cwd whose directory has already been removed still reads as that path.
harness_gone() {
  local proc link
  for proc in /proc/[0-9]*; do
    link="$(readlink "$proc/cwd" 2>/dev/null)" || continue
    case "$link" in "$RUN_ROOT" | "$RUN_ROOT"/*) return 1 ;; esac
  done
  return 0
}

# A LEAK MUST BE ABLE TO REDDEN THE RUN. Cleanup that quietly fails leaves a
# real harness and a real tmux server behind on the operator's box while the
# lane reports success, so the last word on the way out is the leak's.
on_exit() {
  local rc=$?
  teardown
  [ "$artifact_failed" -eq 0 ] || {
    echo "e2e: requested diagnostics could not be preserved — the run is not green" >&2
    exit 1
  }
  [ "$leaked" -eq 0 ] || {
    echo "e2e: cleanup left something behind — the run is not green" >&2
    exit 1
  }
  exit "$rc"
}
# A TRAP THAT ONLY CLEANS UP IS NOT A TRAP. Returning from a signal handler
# resumes the script, so an interrupted lane would tear its world down and then
# carry on running scenarios against it while the parent holding the heavy lock
# still waited. Leave by the door.
on_signal() { teardown; exit 130; }
trap on_exit EXIT
trap on_signal HUP INT TERM

AGENT=dog
HOLD_MARKER=GANGLINE-E2E-HOLD
ERROR_MARKER=GANGLINE-E2E-ERROR
BLOCK_MARKER=GANGLINE-E2E-BLOCK
REPLY_PREFIX=E2E-STUB-REPLY
# Fixed and obviously fake. The stub never reads it; the harness only needs one
# it has been told to trust, and a stable value keeps that record reproducible.
STUB_API_KEY=sk-ant-e2e-stub-0000000000000000

# A FRESH WORLD PER SCENARIO: its own stub, its own request log, its own
# claude config, its own tmux server, its own session name. Nothing a scenario
# leaves behind can be read by the next one, so a green scenario is green on
# its own evidence.
world_up() { # $1 scenario name, used only to label optional diagnostics
  WORLD_NAME="$1"
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
            # This seed was calibrated on a native install. Scheduled CI
            # deliberately submits the same cold config to npm-global; if the
            # harness starts treating the label as authority, boot must fail.
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

world_down() {
  teardown
  [ "$artifact_failed" -eq 0 ] || {
    fail "the scenario's diagnostics could not be preserved" \
      "the requested artifact root is incomplete — see the message above"
    artifact_failed=0
  }
  [ "$leaked" -eq 0 ] || {
    fail "the scenario's world did not come down" \
      "a stub, a tmux server or a temp root outlived it — see the message above"
    leaked=0
  }
}

# THE STUB IS PART OF THE FIXTURE AND CAN FAIL QUIETLY. An unrecognised path, a
# body whose required fields have moved, a traceback in a worker thread: each
# one leaves a scenario's assertions technically true while the dialect drifts
# out from under them. "Fails loudly" is a claim about a check somebody makes,
# so this is that check.
stub_sound() { # $1 scenario name
  local odd
  odd="$(python3 -c '
import json, sys
for line in open(sys.argv[1], encoding="utf-8"):
    line = line.strip()
    if not line:
        continue
    entry = json.loads(line)
    if entry.get("unrecognised"):
        print(entry.get("method", "?"), entry.get("path", "?"))
' "$REQ_LOG" | sort -u | tr '\n' ' ')"
  equal "$1: the stub understood every request the harness sent" "" "$odd"
  if kill -0 "$STUB_PID" 2>/dev/null; then
    pass "$1: the stub server outlived the scenario it served"
  else
    fail "$1: the stub server died mid-scenario" "$(tail -3 "$RUN_ROOT/stub.err")"
  fi
  if grep -q Traceback "$RUN_ROOT/stub.err" 2>/dev/null; then
    fail "$1: the stub raised while answering" "$(tail -12 "$RUN_ROOT/stub.err")"
  else
    pass "$1: the stub answered without raising"
  fi
}

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
#
# `-k` MAKES THE BOUND A BOUND. timeout's first signal is a request, and a
# process that declines it keeps the lane waiting exactly as long as no bound
# at all would have.
E2E_HITCH_LIMIT="${GANG_E2E_HITCH_LIMIT:-120}"
hitch_agent() {
  timeout -k 10 "$E2E_HITCH_LIMIT" "$GANG" hitch "$AGENT" -c claude-code -d "$WORK" \
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
  settled "$1 idle" is_state idle
  return 0
}

state() { # current porcelain state word for the agent
  "$GANG" roster --porcelain 2>/dev/null | awk -F'\t' -v n="$AGENT" '$1 == n { print $3 }'
}

is_state() { [ "$(state)" = "$1" ]; }

say() { # $1 prompt text -> submitted as the operator
  printf '%s' "$1" | "$GANG" send --to "$AGENT" --from operator --stdin
}

# THE INSTRUMENT. The agent's own completed turns, in order, flattened. A
# scenario asserts against this rather than against the screen, because a pane
# shows what was drawn and this shows what was actually sent.
#
# THREE FILTERS, AND EVERY ONE OF THEM CARRIES A CLAIM.
#
# The path, because the harness also asks this server to count tokens, and a
# count carries the whole prompt without ever putting it in front of a model.
#
# The turn flag, because the harness runs side errands — the session-title call
# quotes the user's message verbatim — and a string found only there proves the
# text reached an errand, not the agent.
#
# The completion, because a request log records ARRIVAL. Reading text out of a
# request the harness is still blocked on would let a permanently held turn
# satisfy every assertion made about it.
#
# Search the unfiltered log and any of the three can supply the words the test
# is looking for while the turn under test never carried them.
requests() { python3 -c '
import json, sys

lines = []
for line in open(sys.argv[1], encoding="utf-8"):
    line = line.strip()
    if line:
        lines.append(json.loads(line))

answered = {e["for"] for e in lines if e.get("phase") == "complete"}
for entry in lines:
    if entry.get("phase") != "request":
        continue
    if entry.get("path") != "/v1/messages" or not entry.get("turn"):
        continue
    if entry["seq"] not in answered:
        continue
    print(entry["text"].replace("\n", " "))
' "$REQ_LOG"; }

# The stub opens the held FIFO for writing the instant it starts holding a
# response, so reading it returns exactly when the turn is live on screen. No
# poll, no sleep: the barrier is the turn itself.
#
# BOUNDED, BECAUSE A BARRIER NOBODY REACHES IS A HANG. If the harness stops
# making a request this stub recognises as the agent's own turn, the read below
# would otherwise block forever and the lane would look wedged rather than
# broken. The bound is generous against a measured sub-second arrival.
#
# IT ALSO SAYS WHICH REQUEST IT FROZE. Without that number the lane can only
# ask whether some turn was answered, and the answer to the boot turn is still
# on the same pane; with it, a scenario names the turn it drove.
E2E_HOLD_LIMIT="${GANG_E2E_HOLD_LIMIT:-60}"
HELD_SEQ=""
await_held() {
  HELD_SEQ="$(timeout "$E2E_HOLD_LIMIT" cat "$HELD" 2>/dev/null | tr -dc '0-9')"
  [ -n "$HELD_SEQ" ] && return 0
  fail "the turn never reached the stub" \
    "no request matched the hold marker and the agent-turn sentinel in ${E2E_HOLD_LIMIT}s"
  return 1
}
# BOUNDED FOR THE SAME REASON THE READ IS. Opening a FIFO for writing blocks
# until a reader arrives, so a stub that died after announcing the hold and
# before opening the gate would park this line forever.
release_held() {
  # shellcheck disable=SC2016 # the path is passed as an argument, not expanded here
  timeout "$E2E_HOLD_LIMIT" bash -c ': > "$1"' _ "$GATE" && return 0
  fail "the held turn could not be released" \
    "nothing opened the gate for reading in ${E2E_HOLD_LIMIT}s; the stub is gone"
  return 1
}

# ---------------------------------------------------------------- scenario 1
# BOOT REACHES A COMPOSER AND GANGLINE SEES IT. hitch does not return until it
# has delivered the startup contract into a box it could read, so its own exit
# status is the first half of the proof; the request log is the second, and the
# stronger one — the contract did not merely reach a box, it reached the model.
scenario_boot() {
  world_up boot
  if ! booted boot; then world_down; return; fi
  pass "boot: hitch delivered a startup contract to a real claude-code"
  equal "boot: the booted agent reads idle" idle "$(state)"

  local sent
  sent="$(requests)"
  contains "boot: the standing contract reached the model" \
    "$sent" "Gangline contract"
  contains "boot: the agent was told its own name" "$sent" "You are $AGENT in Gangline"
  stub_sound boot
  world_down
}

# ---------------------------------------------------------------- scenario 2
# A NATIVE Stop IS WHAT RELEASES a gang wait for the done condition. The turn is held open
# by the stub, so the wait is armed against a turn that provably cannot have
# finished yet; the release then happens only after tmux has stored the
# caller's channel. What returns the wait is therefore the harness's own Stop
# hook and nothing else.
scenario_turn() {
  world_up turn
  if ! booted turn; then world_down; return; fi

  say "$HOLD_MARKER please hold" >/dev/null
  if ! await_held; then world_down; return; fi
  equal "turn: a live turn reads busy" busy "$(state)"

  local wait_rc=0
  "$GANG" wait "$AGENT" --until "done" --timeout 120 >"$RUN_ROOT/wait.out" 2>&1 &
  local wait_pid=$!

  settle "wait armed" wait_armed || {
    fail "turn: gang wait never armed a boundary" "no channel appeared in tmux hooks"
    kill "$wait_pid" 2>/dev/null || true
    release_held || true
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
  # THE ANSWER TO THIS TURN, NOT TO SOME TURN. The prefix alone is on the pane
  # already: booting delivered a startup contract and the stub answered it, so a
  # bare-prefix check would pass even if the held turn's completion were never
  # drawn at all. The stub numbers every answer and told the lane which request
  # it froze, so the string below exists only if the harness rendered the reply
  # to the turn this scenario drove.
  #
  # The lane never types that string — not in a prompt, not in an envelope, not
  # in the contract — so the only route from the stub to this surface is the
  # harness rendering the answer.
  # source-guard: whole-surface@234c84c39d32: only the stub emits this numbered prefix and the lane never types it, so any producer that put it on the pane is the harness rendering that completion
  contains "turn: the harness rendered this turn's own completion" \
    "$("$GANG" capture "$AGENT" 200)" "$REPLY_PREFIX $HELD_SEQ"
  stub_sound turn
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
  world_up bricked
  if ! booted bricked; then world_down; return; fi

  say "$ERROR_MARKER now" >/dev/null
  settled "bricked classified" is_state bricked
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
  stub_sound bricked
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
  world_up midturn
  if ! booted midturn; then world_down; return; fi

  say "$HOLD_MARKER hold for the courier" >/dev/null
  if ! await_held; then world_down; return; fi
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
  settled "midturn envelope sent" envelope_in_requests "$token"

  # ONE REQUEST HAS TO CARRY BOTH. Asking the log separately for the token and
  # for the attribution lets two different requests answer — and the harness
  # makes requests of its own that quote the operator's text — so the envelope
  # could arrive stripped of its sender while both checks passed. The pair is
  # the claim: a completed turn of the agent's own that carried the envelope
  # with its attribution intact.
  local sent_log
  sent_log="$(requests | grep -- "$token" || true)"
  # source-guard: producer@7423509e937e: the unique courier token is composed by this scenario's single send, and requests retains only completed agent turns, so the matching line witnesses that delivery
  contains "midturn: the envelope reached the model on the agent's own turn" \
    "$sent_log" "$token"
  # source-guard: producer@138278c64bc5: the same token-filtered completed agent turn can carry this courier attribution only from the single send above; title and token-count errands are excluded by requests
  contains "midturn: that same turn carried its sender's attribution" \
    "$sent_log" "gang:self-declared:courier"
  # THE ENVELOPE'S OWN TURN HAS TO FINISH TOO, and this is what catches a lane
  # that asserted on a request the harness was still blocked on: the request
  # log records a request when it ARRIVES, so a permanently held turn would
  # satisfy every check above it while leaving the agent stuck.
  settled "midturn settled" is_state idle
  equal "midturn: the agent came back to rest afterwards" idle "$(state)"
  stub_sound midturn
  world_down
}

envelope_in_requests() { requests | grep -q -- "$1"; }

# THE SAME WINDOW, READ THROUGH A COLLAR THAT DECLARES NO BLOCKED READER. Its
# directory is written by the scenario that uses it, so this is a read of that
# world and not a second world of its own.
unread_state() {
  GANG_COLLARS="$RUN_ROOT/collars-unread" "$GANG" roster --porcelain 2>/dev/null \
    | awk -F'\t' -v n="$AGENT" '$1 == n { print $3 }'
}

# A READING, NEVER AN ABSENCE. `unread_state` prints nothing when the roster
# row is missing or the read itself fails, and an empty answer would satisfy
# "not busy" and then satisfy every check that reads the value afterwards. The
# instrument failing must not look like the subject changing.
unread_settled() {
  case "$(unread_state)" in
    busy | '') return 1 ;;
    *) return 0 ;;
  esac
}

# THE VOCABULARY IS READ OUT OF THE SUBJECT, never spelled again here. A list
# this lane kept for itself would go stale the first time gang renamed a word,
# and the check above would then reject a reading gang really printed.
roster_state_words() {
  local words
  words="$(sed -n '/^roster_state_word()/,/^}/p' "$GANG" \
    | sed -n 's/.*printf \([a-z][a-z-]*\) ;;.*/\1/p')"
  [ -n "$words" ] \
    || { printf 'e2e: could not read the porcelain state words out of %s\n' \
           "$GANG" >&2; exit 1; }
  printf '%s\n' "$words"
}

# ---------------------------------------------------------------- scenario 5
# A TURN THAT ENDED WITHOUT PRODUCING WORK, DRIVEN THROUGH A REAL HARNESS. This
# is the one state whose whole danger is that the window looks well: the
# composer is free, the pane is quiet, and every guard Gangline keeps is a
# guard on the composer. A fixture can only assert that the reader agrees with
# a record the fixture wrote, which is a claim about the reader and not about
# the harness. So the record here is written by claude-code, about a request a
# provider actually refused, and the counterfactual below is taken from the
# same live pane.
#
# THE COUNTERFACTUAL IS THE PROOF, not the verdict. `!blocked!` on its own
# would also be produced by a reader that fired on anything, so the scenario
# reads the identical window a second time through a collar that is this
# collar with `collar_blocked` removed. Whatever that read returns is what
# every other rule Gangline has — the occupancy regex, the busy regex, the
# composer, the turn bracket — makes of this pane; if it is `~idle~`, then the
# transcript reader is carrying the whole state and the defect is reproduced
# and closed in one pair of readings.
scenario_blocked() {
  world_up blocked
  if ! booted blocked; then world_down; return; fi

  say "$BLOCK_MARKER now" >/dev/null
  settled "blocked classified" is_state blocked
  local reading
  reading="$(state)"
  equal "blocked: a turn that produced no work reads blocked" blocked "$reading"
  if [ "$reading" != blocked ]; then
    note "RECORDED on $HARNESS_BUILD: it read '$reading' instead."
    note "$("$GANG" explain "$AGENT" 2>&1 | tail -8)"
  fi

  # THE READER'S OWN SENTENCE, which no other verdict in this collar produces:
  # the fatal branch says a broken response stream or an HTTP status, and the
  # unreadable branches say so. Accepting the state word alone would survive a
  # reader that reached the right verdict for the wrong record.
  contains "blocked: the reason is a turn that ended on an API error" \
    "$("$GANG" status "$AGENT" 2>&1)" "ended the latest turn on an API error"

  # WHAT THE REST OF GANGLINE MAKES OF THE SAME PANE. A collar identical to the
  # shipped one except that it declares no blocked reader leaves every other
  # rule in place, so this reading is the state this window had before the
  # reader existed — taken live, from the pane that is reading blocked right
  # now, rather than reasoned about.
  mkdir -p "$RUN_ROOT/collars-unread"
  {
    printf '# shellcheck shell=bash\n'
    printf '. %s\n' "$(printf '%q' "$ROOT/collars/claude-code.sh")"
    printf 'unset -f collar_blocked\n'
  } > "$RUN_ROOT/collars-unread/claude-code.sh"
  # WHAT THE READER-LESS COLLAR ACTUALLY SAYS, MEASURED RATHER THAN ASSUMED.
  # Its first answer is `-busy-`: the harness writes no Stop for a turn that
  # ends this way, so the turn bracket gang opened is still open and answers
  # before the busy regex is ever reached. A lead reading that window is told a
  # turn is running and a send is refused for a turn that already ended.
  #
  # WHAT HAPPENS NEXT IS NOT FIXED, and pinning it was this scenario's own bug.
  # From the same starting point — the turn classified on the first read — one
  # reading held that busy for the whole budget and another let go of it partway
  # through and read idle, which is the reported symptom exactly and the worse
  # of the two, because a send to a window reported idle is delivered into
  # nothing. Elapsed time does not account for the difference, so asserting
  # either value makes this scenario fail on the run that did the other. What is
  # asserted is what both readings share: neither is `blocked`, and neither
  # comes from anything that read the transcript. The second value is required
  # to be a state gang prints, so a roster that failed to answer cannot pass for
  # a subject that changed, and is then recorded with the build it came from
  # rather than pinned.
  local unread unread_first unread_explain unread_named word
  unread_first="$(unread_state)"
  equal "blocked: without the transcript reader the same window reads busy" \
    busy "$unread_first"
  # THE BRACKET, NOT THE PAINT, read at the first answer and before anything can
  # decay. Naming which rule answered is the difference between a turn that
  # never ended and a screen that still shows the last one.
  unread_explain="$(GANG_COLLARS="$RUN_ROOT/collars-unread" "$GANG" explain "$AGENT" 2>&1)"
  contains "blocked: that busy is an unclosed turn, not a screen the regex matched" \
    "$unread_explain" "GANG_BUSY_REGEX: not evaluated"
  contains "and that collar is the shipped one with only the reader removed" \
    "$unread_explain" "collar_blocked: not declared"
  # A BOUNDED LOOK FOR THE OTHER READING, whose absence is not a failure. The
  # `turn` scenario measures how quickly an ordinary turn's bracket closes; this
  # spends the same budget seeing which way this one goes, and says so.
  settle "blocked: looking for a reader-less reading that is not busy" \
    unread_settled || true
  unread="$(unread_state)"
  # THE SECOND READING IS A READING. Recording it is only worth anything if the
  # roster actually answered, and the exclusion below is vacuous on an empty
  # string, so the value is required to be one gang prints before it is used.
  unread_named=""
  for word in $(roster_state_words); do
    [ "$word" != "$unread" ] || unread_named="$unread"
  done
  equal "blocked: and the reader-less window answered the second read too" \
    "$unread" "$unread_named"
  note "RECORDED on $HARNESS_BUILD: reader-less state '$unread_first' then '$unread', over at most $E2E_READS reads at ${E2E_NAP}s each."
  excludes "blocked: and no rule but the transcript reader ever says blocked" \
    "$unread_first$unread" "blocked"

  # DELIVERY IS POSSIBLE HERE, WHICH IS WHY IT MUST BE REFUSED. The composer
  # accepts keystrokes; nothing but the state read stands between a sender and
  # a message reported as delivered into a window that will never act on it.
  local send_rc=0 send_out
  send_out="$(printf 'E2E-BLOCKED-BODY' | "$GANG" send --to "$AGENT" --from operator \
    --live-only --stdin 2>&1)" || send_rc=$?
  if [ "$send_rc" -eq 0 ]; then
    fail "blocked: gang send should refuse a blocked window" "it reported delivered"
  else
    contains "blocked: the refusal names the blocking reason" "$send_out" "is blocked ("
  fi
  # NEITHER OF THE TWO WRONG ANSWERS. A guard that ran only inside inject would
  # answer this window from whatever paint its finished turn left behind, and a
  # free composer would otherwise be read as an ordinary idle target.
  excludes "blocked: the refusal is not read off the screen the turn left" \
    "$send_out" "is mid-turn"
  excludes "blocked: nor is it the occupancy tier answering" \
    "$send_out" "is occupied"
  # THE BODY NEVER REACHED THE MODEL. The request log is the instrument that
  # separates a refusal from a paste that merely looked refused.
  local landed
  landed="$(requests | grep -c -- 'E2E-BLOCKED-BODY' || true)"
  equal "blocked: nothing the refusal covered reached the model" 0 "$landed"

  # A BOUNDARY THAT IS NOT COMING CANNOT BE WAITED FOR. Returning satisfied
  # here is the same lie as reading idle, one command further on.
  local wait_rc=0
  "$GANG" wait "$AGENT" --until idle --timeout 5 >"$RUN_ROOT/wait.out" 2>&1 || wait_rc=$?
  if [ "$wait_rc" -eq 0 ]; then
    fail "blocked: gang wait should refuse a blocked window" "it returned 0"
  else
    contains "blocked: gang wait refuses the boundary by name" \
      "$(cat "$RUN_ROOT/wait.out")" "cannot reach the requested boundary"
  fi

  # A BLOCKED WINDOW IS NOT A BRICKED ONE. The repairs differ — a bricked
  # session is re-hitched, this one is re-driven — so a reader that folded the
  # two together would be visible here and nowhere else.
  excludes "blocked: the fatal reader does not also claim this turn" \
    "$("$GANG" explain "$AGENT" 2>&1)" "collar_bricked: matched"
  stub_sound blocked
  world_down
}

# ----------------------------------------------------------------------------
SCENARIOS="boot turn bricked blocked midturn"

run() { # $1 scenario name, validated against the list above before it is called
  case " $SCENARIOS " in
    *" $1 "*) ;;
    *) echo "e2e: no scenario named '$1' (have: $SCENARIOS)" >&2; exit 2 ;;
  esac
  "scenario_$1"
}

chosen="$SCENARIOS"
[ "$#" -eq 0 ] || chosen="$*"
# AN EMPTY ARGUMENT IS NOT AN EMPTY SUITE. `test/e2e.sh "$maybe_scenario"` with
# nothing in the variable used to select no scenarios, run none, and exit green
# on a count of zero — a caller asking for one thing and being told everything
# was fine. Splitting here turns that into the argument error it always was.
# shellcheck disable=SC2086 # the scenario list is deliberately word-split
set -- $chosen
[ "$#" -gt 0 ] || { echo "e2e: no scenario named '' (have: $SCENARIOS)" >&2; exit 2; }
for scenario in "$@"; do
  run "$scenario"
done

printf '\n'
suite_tail "$checks" "$fails" "$SECONDS" "against $HARNESS_BUILD"
if [ -n "$margins" ]; then
  printf 'bounded waits, reads used of %s at %ss each:\n%s\n' \
    "$E2E_READS" "$E2E_NAP" "$margins"
fi
# A RUN THAT ASSERTED NOTHING IS NOT A PASS. Zero checks means every scenario
# returned before its first assertion, which is a broken lane wearing a green
# exit status.
[ "$checks" -gt 0 ] || { echo "e2e: no scenario made a single check" >&2; exit 1; }
[ "$fails" -eq 0 ]
