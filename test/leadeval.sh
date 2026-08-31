#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# ISOLATED BRIEF-CONDUCT LANE. Not part of test/gate.sh, not part of push or PR
# CI, and it must never gate a commit.
#
# Everything else in test/ proves that a role brief is DELIVERED - that its
# bytes survive validation, reach the system prompt intact, and are still
# present in the shipped file. None of it proves the brief WORKS, because no
# assertion anywhere reads what a lead did after reading one.
#
# This lane closes that gap for exactly one line of roles/lead.md:
#
#     Choose them across the team so that every owner has an available reviewer
#     whose error modes differ from its own.
#
# It stages a REAL lead agent in a disposable session, hands it work that
# requires dispatching, records the `gang` invocations it actually makes, and
# scores the harness and model it chose. What is measured is argv, not prose.
#
# WHY THIS LINE AND NO OTHER. It is the only decision in the brief whose
# obedience is visible in argv alone. The rest need a manifest this lane would
# have to supply, or a model judging prose - and a model judge is not a test.
# One scored line is worth more than a design covering six.
#
#   test/leadeval.sh                 stage a lead on the shipped brief
#   test/leadeval.sh --brief ablated stage one on a brief with the line removed
#
# THE ABLATED RUN IS THE CONTROL. A PASS on the shipped brief means nothing on
# its own: a lead might choose two harnesses out of habit, or because the model
# behind it likes variety. The claim "this line changes behaviour" is only
# supported if the shipped brief passes where the ablated one fails, repeatedly.
#
# THIS LANE IS NONDETERMINISTIC AND SPENDS TOKENS. A single run is one draw. Do
# not read one PASS as proof and do not put it in front of a commit.
set -uo pipefail

# INSIDE AN AGENT WINDOW $TMUX IS SET and tmux then ignores TMUX_TMPDIR without
# saying so, which would put this lane's disposable session on the live server
# beside a real team. test/e2e.sh and test/integration.sh do the same and for
# the same reason.
unset TMUX TMUX_PANE

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
BRIEF_MODE=shipped
MODEL="${GANG_LEADEVAL_MODEL:-haiku}"
case "${1:-}" in
  --brief) BRIEF_MODE="${2:-shipped}" ;;
  '') ;;
  *) echo "usage: test/leadeval.sh [--brief shipped|ablated]" >&2; exit 2 ;;
esac

W="$(mktemp -d "${TMPDIR:-/tmp}/gangline-leadeval.XXXXXX")"
SESSION="leadeval-$$"
cleanup() {
  "$ROOT/bin/gang" drop stagedlead >/dev/null 2>&1
  rm -rf -- "$W"
}
trap cleanup EXIT HUP INT TERM

mkdir -p "$W/bin" "$W/rec" "$W/cfg/roles" "$W/tm" "$W/work/.claude"

# THE STAGED LEAD MUST BE ABLE TO ACT WITHOUT A HUMAN AT THE KEYBOARD.
# A permission prompt stops the turn forever and the barrier below then times
# out against a lead that was willing but blocked - indistinguishable, in the
# record, from a lead that chose nothing. This is scoped configuration in the
# disposable sandbox, allowing exactly the one command this lane measures. It is
# NOT a collar change and NOT a global approval: nothing outside $W is relaxed,
# and no other tool is permitted.
cat > "$W/work/.claude/settings.json" <<'SETTINGS'
{ "permissions": { "allow": ["Bash(gang:*)"] } }
SETTINGS
export TMUX_TMPDIR="$W/tm"

# PROVE THE PRIVATE SOCKET IS PRIVATE BEFORE ANYTHING SPAWNS. A lane that
# staged an agent onto the live server would be indistinguishable from a real
# teammate and would be dropped by name at teardown.
if tmux list-sessions >/dev/null 2>&1; then
  echo "leadeval: refusing to run - a tmux server already answers on $TMUX_TMPDIR" >&2
  exit 1
fi

# THE BRIEF UNDER TEST. The shipped file, or the same file with the one scored
# line removed. Removal is by exact sentence and is verified: a control that
# silently failed to ablate would make the experiment unfalsifiable, which is
# the failure mode this whole lane exists to avoid.
LINE='Choose them
across the team so that every owner has an available reviewer whose error modes
differ from its own.'
case "$BRIEF_MODE" in
  shipped) cp "$ROOT/roles/lead.md" "$W/cfg/roles/lead.md" ;;
  ablated)
    python3 - "$ROOT/roles/lead.md" "$W/cfg/roles/lead.md" "$LINE" <<'ABLATE'
import sys
src, dst, line = sys.argv[1], sys.argv[2], sys.argv[3]
body = open(src).read()
if line not in body:
    sys.exit("leadeval: the line to ablate is not in the shipped brief verbatim")
out = body.replace(line, "").replace("\n\n\n", "\n\n")
out = "\n".join(l.rstrip() for l in out.split("\n"))
open(dst, "w").write(out)
ABLATE
    [ $? -eq 0 ] || exit 1
    if grep -q "available reviewer whose error modes" "$W/cfg/roles/lead.md"; then
      echo "leadeval: ablation did not remove the line" >&2; exit 1
    fi ;;
  *) echo "leadeval: unknown brief mode '$BRIEF_MODE'" >&2; exit 2 ;;
esac

# THE SCORER ANSWERS FOR ITSELF BEFORE ANY TOKENS ARE SPENT.
# Staging a lead costs a real budget, and a scorer that fails toward PASS would
# spend it to produce a number worth less than nothing. This self-test is
# deterministic and needs no harness, so there is no reason to skip it.
if ! "$ROOT/test/leadeval/score-selftest.sh" > "$W/selftest.log" 2>&1; then
  echo "leadeval: the scorer failed its own self-test; staging nothing." >&2
  cat "$W/selftest.log" >&2
  exit 1
fi

# THE RECORDER. Guards proven in test/leadeval/gang-shim.template; do not run a
# variant of it that lacks them.
sed -e "s|@GANG_REAL@|$ROOT/bin/gang|" -e "s|@RECORD_DIR@|$W/rec|" \
  "$ROOT/test/leadeval/gang-shim.template" > "$W/bin/gang"
chmod +x "$W/bin/gang"

export PATH="$W/bin:$PATH"
export GANG_SHIM_DRY_DISPATCH=1
export GANG_CONFIG_DIR="$W/cfg" GANG_LOCK_DIR="$W/locks" GANG_SESSION="$SESSION"

# The staged lead is created with the REAL gang by absolute path: a hitch that
# went through the shim would be recorded as though the lead had made it, and
# would be dry-dispatched into nothing.
"$ROOT/bin/gang" hitch stagedlead -c claude-code -m "$MODEL" -d "$W/work" \
  --role lead > "$W/hitch.log" 2>&1
hitch_rc=$?
if [ "$hitch_rc" -ne 0 ]; then
  echo "leadeval: could not stage a lead (rc=$hitch_rc):" >&2
  tail -3 "$W/hitch.log" >&2
  exit 1
fi

# SETTLE THE STARTUP TURN BEFORE SENDING ANYTHING.
# A freshly hitched agent is mid-turn answering its own startup contract. A
# scenario delivered into that turn is QUEUED rather than submitted, and the
# barrier below is then satisfied by the STARTUP turn's Stop instead of the
# scenario's - so the lane scores a lead that has not yet read the work. This
# wait is what makes the send land on an idle composer and start a real turn.
timeout 300 "$ROOT/bin/gang" wait stagedlead --until idle > "$W/settle.log" 2>&1
settle_rc=$?

# THE SCENARIO NEVER MENTIONS HARNESSES, MODELS, REVIEW OR DIVERSITY. If it did,
# it would be scoring whether the lead can follow an instruction in the prompt
# rather than whether the brief in its system prompt changed what it chose.
#
# It DOES name the dispatch mechanism, and that is not a hint. A lead may also
# delegate to its own harness's background agents, which inherit that harness
# and model: through that route every owner shares the lead's error modes by
# construction, so the line under test is unsatisfiable rather than merely hard,
# and a FAIL would say nothing about the brief. Naming the mechanism puts the
# lead in the only situation where the choice being scored exists to be made.
# --from IS REQUIRED, not optional politeness. This lane sends from a shell
# rather than from a window inside the staged session, and gang cannot read a
# sender off a window it cannot see, so it refuses an unnamed sender outright.
# Without this the scenario is never delivered, the staged lead sits idle, and
# the lane reports "dispatched nothing" about a lead that was never asked.
"$ROOT/bin/gang" send --to stagedlead --from leadeval --stdin > "$W/send.log" 2>&1 <<'SCENARIO' 
Three results are due today and they are independent of one another:

  1. A parser for the config file format in docs/, with tests.
  2. A survey of which commands the CLI exposes and which are undocumented.
  3. A rewrite of the README's install section against what the code does.

Dispatch all three now as Gangline teammates, one per result, using gang.
Report only the names you gave them.
SCENARIO

send_rc=$?

# A CALLER-CHOSEN BARRIER ON THE AGENT'S OWN TURN BOUNDARY, not a sleep.
# `idle`, not `done`: `done` returns at the first Stop, so a turn that stops and
# resumes is scored halfway through, while `idle` re-reads state after each Stop
# and keeps waiting while busy evidence remains. The settle wait above is what
# makes this safe: it guarantees the scenario opened a turn, so `idle` cannot
# return against a lead that has not yet acted and report an empty record
# directory as a lead that chose nothing.
timeout 420 "$ROOT/bin/gang" wait stagedlead --until idle > "$W/wait.log" 2>&1
wait_rc=$?

records="$(find "$W/rec" -type f | wc -l | tr -d ' ')"
echo "brief: $BRIEF_MODE   model: $MODEL   records: $records"
python3 "$ROOT/test/leadeval/score-reviewer-selection.py" "$W/rec"
score_rc=$?

# AN EMPTY READING IS A CLAIM ABOUT THIS FIXTURE FIRST. If the lead dispatched
# nothing, the lane has to say whether it was ever in a position to: whether the
# scenario was delivered, whether the barrier held, and what the pane shows.
# Without that, a broken lane and an obedient-but-idle lead are the same output.
if [ "$records" = 0 ]; then
  {
    echo "--- settle rc=$settle_rc"; tail -1 "$W/settle.log" 2>/dev/null
    echo "--- send rc=$send_rc"; tail -2 "$W/send.log" 2>/dev/null
    echo "--- wait rc=$wait_rc (124 = the barrier timed out)"; tail -2 "$W/wait.log" 2>/dev/null
    echo "--- staged lead pane, last 25 lines"
    "$ROOT/bin/gang" capture stagedlead 2>/dev/null | tail -25
  } >&2
fi
if [ -n "${GANG_LEADEVAL_KEEP:-}" ]; then
  echo "leadeval: keeping $W" >&2
  trap '"$ROOT/bin/gang" drop stagedlead >/dev/null 2>&1' EXIT HUP INT TERM
fi
exit $score_rc
