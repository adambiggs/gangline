# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Provider window history: what is sampled, what is alerted once, what is
# refused, and what an unreadable provider is reported as.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file in
# order and it reads that shell's fixtures, helpers and counters.

cap_root="$RUN_ROOT/cap"
cap_sessions="$RUN_ROOT/cap-sessions/2026/09/09"
mkdir -p "$cap_root" "$cap_sessions"

# A Codex session file carries its rate_limits snapshot as one event line among
# the transcript's own. `codex_bengalfox` is the pool of a model with its own
# allowance and is not the account's weekly window, so a run whose newest lines
# hold only that id must read as unreadable rather than as a low reading.
cap_session() { # $1 = file, $2 = limit id, $3 = used percent, $4 = reset epoch, $5 = stamp
  cat > "$1" <<JSON
{"timestamp":"$5","type":"message","payload":{"role":"user"}}
{"timestamp":"$5","type":"event_msg","payload":{"rate_limits":{"limit_id":"$2","limit_name":null,"primary":{"used_percent":$3,"window_minutes":10080,"resets_at":$4},"secondary":null,"plan_type":"pro","rate_limit_reached_type":null}}}
JSON
}

cap_reset_one=1789435568
cap_reset_two=1790045568
cap_session "$cap_sessions/rollout-one.jsonl" codex 22 "$cap_reset_one" "2026-09-09T18:00:00.000Z"

cap_env=(env "GANG_CAP_DIR=$cap_root" "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-sessions")
cap_readings="$cap_root/readings.jsonl"
cap_alerts="$cap_root/alerts.jsonl"

# A Claude reading costs a provider turn, so the reader is a seam: the collar's
# own tab-separated rows go in, and nothing here needs a live harness.
cap_claude_rows="$RUN_ROOT/cap-claude-rows"
cat > "$cap_claude_rows" <<ROWS
Current session	31	$cap_reset_one	1789430000
Current week (all models)	72	$cap_reset_one	1789430000
ROWS
cap_claude_reader="cat '$cap_claude_rows'"

cap_check() { # run one sampling pass with the fixtures above
  "${cap_env[@]}" "GANG_CAP_CLAUDE_READER=$cap_claude_reader" \
    "$GANG" cap check --claude-interval 0 "$@" 2>&1
}

cap_field() { # $1 = readings line number, $2 = key
  python3 -c '
import json, sys
line = open(sys.argv[1]).read().splitlines()[int(sys.argv[2]) - 1]
print(json.loads(line).get(sys.argv[3], "<absent>"))
' "$cap_readings" "$1" "$2"
}

cap_count() { # $1 = file -> lines, or 0 when it was never written
  if [ -f "$1" ]; then wc -l < "$1" | tr -d ' '; else printf 0; fi
}

# A GANG COMMAND THAT NOBODY OWNS THE STATUS OF IS NOT A CHECK. Under the
# suite's errexit the run dies at the command itself: before any status is read,
# before the captured stderr is looked at, and before the check that was going
# to assert either. A refusal then leaves no line behind at all, which reads
# exactly like a refusal that never happened. A command substitution inside an
# assertion's argument does not end the run, but it hands the refusal's text to
# the assertion as though it were output, so the refusal is reported as a
# content mismatch rather than as the refusal it is.
#
# So every gang invocation below is owned by exactly one of three things: this
# helper, the suite's own `refuses`, or a `wait` on its pid. Nothing else runs
# gang.
# The status is half of it. A refusal explains itself on stderr, and a failure
# that reports only two numbers throws that explanation away: the run says a
# pass it cannot name went wrong, when the tool had already said why. So an
# unexpected status carries what the command said into the failure.
cap_pass() { # $1 description, $2 expected status, rest = command -> $cap_out
  local description="$1" expected="$2" status=0
  shift 2
  cap_out="$("$@" 2>&1)" || status=$?
  if [ "$status" = "$expected" ]; then
    pass "$description"
  else
    fail "$description" "expected status [$expected], got [$status], saying: $cap_out"
  fi
}

# ONE PASS OVER A HEALTHY HOST. Both providers answer, and the percentages
# stored are the ones the providers printed.
cap_pass "a pass over a host that answers is an ordinary pass" 0 \
  cap_check --thresholds 70,90
cap_first="$cap_out"
contains "a crossing on the account weekly window alerts by name" \
  "$cap_first" "Current week (all models) is 72% used, at or past 70%"
excludes "and a window below the threshold does not" \
  "$cap_first" "codex weekly is 22%"
equal "the pass records one row per window read" 3 "$(cap_count "$cap_readings")"
equal "the Codex row carries the published percentage" 22 "$(cap_field 1 used)"
equal "and names the surface it was published on" \
  codex-session-file "$(cap_field 1 source)"
equal "the Claude session window is stored beside the weekly one" \
  other "$(cap_field 2 window)"
equal "and only the weekly window is alertable" weekly "$(cap_field 3 window)"
equal "one crossing is one alert" 1 "$(cap_count "$cap_alerts")"

# THE SAME READING AGAIN IS NOT A SECOND CROSSING. An alert that repeats every
# tick is read as decoration, and then the wall arrives unannounced.
cap_pass "and sampling it again is too" 0 cap_check --thresholds 70,90
cap_second="$cap_out"
excludes "a repeated reading above the threshold does not alert again" \
  "$cap_second" "at or past 70%"
equal "and writes no second alert record" 1 "$(cap_count "$cap_alerts")"

# A HIGHER THRESHOLD IN THE SAME WINDOW IS ITS OWN CROSSING.
sed -i 's/^Current week (all models)	72/Current week (all models)	91/' "$cap_claude_rows"
cap_pass "and so is the pass that climbs" 0 cap_check --thresholds 70,90
cap_third="$cap_out"
contains "climbing past the next threshold alerts once more" \
  "$cap_third" "at or past 90%"
excludes "without repeating the threshold already crossed" \
  "$cap_third" "at or past 70%"
equal "and the alert file holds one record per crossing" 2 "$(cap_count "$cap_alerts")"

# A NEW PROVIDER WINDOW RE-ARMS EVERY THRESHOLD, because the allowance the
# thresholds measure is the one that just reset.
sed -i "s/	$cap_reset_one	/	$cap_reset_two	/" "$cap_claude_rows"
cap_pass "and the pass that finds a new window" 0 cap_check --thresholds 70,90
cap_rolled="$cap_out"
contains "the window that replaces it alerts from the bottom again" \
  "$cap_rolled" "at or past 70%"
equal "and both of its crossings are recorded" 4 "$(cap_count "$cap_alerts")"

# A CLOSED WINDOW'S READING IS NOT THE PRESENT ONE. A session file outlives the
# window it recorded and replays that snapshot; admitting it would report a
# percentage that was true and is not, and re-arm thresholds that have fired.
cap_stale_before="$(cap_count "$cap_alerts")"
# The closed window's percentage is far from the live one, so a reading admitted
# from it would be visible in what gang cap reports rather than hidden behind a
# figure that happens to match.
sed -i "s/	$cap_reset_two	/	$cap_reset_one	/; s/^Current week (all models)	91	/Current week (all models)	5	/" \
  "$cap_claude_rows"
cap_pass "and the pass that finds a closed one" 0 cap_check --thresholds 70,90
cap_stale="$cap_out"
excludes "a reading from a window that has closed raises no alert" \
  "$cap_stale" "at or past"
equal "and adds no alert record" "$cap_stale_before" "$(cap_count "$cap_alerts")"
equal "the reading is kept, marked as the closed window it belongs to" \
  superseded "$(cap_field "$(cap_count "$cap_readings")" disposition)"
cap_pass "showing after a closed reading refuses nothing" 0 \
  "${cap_env[@]}" "$GANG" cap show
contains "and the live reading still names the open window" "$cap_out" "91% used"

# WHAT IS SHOWN ON DEMAND IS WHAT WAS READ, WITHOUT SPENDING A TURN.
cap_pass "showing what was read spends nothing and refuses nothing" 0 \
  "${cap_env[@]}" "$GANG" cap show
cap_shown="$cap_out"
contains "the standing reading is visible without sampling" "$cap_shown" "codex weekly: 22% used"
contains "and a crossed threshold stays visible for the window it fired in" \
  "$cap_shown" "ALERT at 70%, 90%"
# The age a reading carries advances between two reads of it, so the two forms
# are compared with that field held still rather than raced against the clock.
cap_ageless() { sed 's/read [0-9]*[a-z] ago/read AGE ago/'; }
cap_pass "the bare command refuses nothing either" 0 "${cap_env[@]}" "$GANG" cap
equal "the bare command shows rather than samples" \
  "$(printf '%s\n' "$cap_shown" | cap_ageless)" \
  "$(printf '%s\n' "$cap_out" | cap_ageless)"

# A PROVIDER THAT CANNOT BE READ IS REPORTED AS UNREADABLE. Silence on a parse
# failure is the blindness this history exists to end, so the failure is stored,
# named on stderr, and never stood in for by a number.
cap_broken="$RUN_ROOT/cap-broken/2026/09/09"
mkdir -p "$cap_broken"
cap_session "$cap_broken/rollout-broken.jsonl" codex '"most of it"' "$cap_reset_one" \
  "2026-09-09T19:00:00.000Z"
cap_pass "a pass that read no provider refuses" 1 \
  env "GANG_CAP_DIR=$cap_root" "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-broken" \
  "$GANG" cap check --providers codex
cap_malformed="$cap_out"
contains "a percentage that is not a number is named as the defect" \
  "$cap_malformed" "Codex used_percent is not a number"
equal "and no percentage is stored for that pass" \
  unreadable "$(cap_field "$(cap_count "$cap_readings")" status)"
equal "an unreadable pass stores no invented reading" \
  '<absent>' "$(cap_field "$(cap_count "$cap_readings")" used)"

refuses "a pass with no session directory to read fails rather than reporting quiet" \
  "no Codex session directory at" \
  env "GANG_CAP_DIR=$cap_root" "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-empty-sessions" \
  "$GANG" cap check --providers codex
cap_silent="$RUN_ROOT/cap-silent/2026/09/09"
mkdir -p "$cap_silent"
printf '%s\n' '{"timestamp":"2026-09-09T19:00:00.000Z","type":"message","payload":{"role":"user"}}' \
  > "$cap_silent/rollout-silent.jsonl"
refuses "and session files that carry no weekly window are named as carrying none" \
  "no weekly 'codex' rate_limits record" \
  env "GANG_CAP_DIR=$cap_root" "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-silent" \
  "$GANG" cap check --providers codex

# A POOL THAT IS NOT THE ACCOUNT'S WEEKLY WINDOW IS NOT READ AS ONE.
cap_other="$RUN_ROOT/cap-other/2026/09/09"
mkdir -p "$cap_other"
cap_session "$cap_other/rollout-other.jsonl" codex_bengalfox 2 "$cap_reset_one" \
  "2026-09-09T19:00:00.000Z"
refuses "another limit id is not mistaken for the account's weekly window" \
  "no weekly 'codex' rate_limits record" \
  env "GANG_CAP_DIR=$cap_root" "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-other" \
  "$GANG" cap check --providers codex

# AND BEING UNREADABLE FOR LONG ENOUGH IS ITSELF AN ALERT, once.
cap_blind_root="$RUN_ROOT/cap-blind"
cap_blind() {
  env "GANG_CAP_DIR=$cap_blind_root" "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-empty-sessions" \
    "$GANG" cap check --providers codex --stale-seconds 0 2>&1
}
cap_pass "a blind pass refuses rather than reporting a quiet host" 1 cap_blind
cap_blind_first="$cap_out"
contains "a provider that stops answering is alerted on" \
  "$cap_blind_first" "codex usage telemetry has been unreadable since"
cap_pass "and it keeps refusing while it stays blind" 1 cap_blind
cap_blind_second="$cap_out"
excludes "and the same blindness is not alerted on every pass" \
  "$cap_blind_second" "usage telemetry has been unreadable since"
equal "one blindness is one alert" 1 "$(cap_count "$cap_blind_root/alerts.jsonl")"

# THE RECORDED HISTORY REPLAYS THROUGH THE SAME ENGINE THAT WATCHES LIVE.
cap_pass "recorded history replays without refusing" 0 \
  "$GANG" cap replay \
  --readings "$ROOT/test/fixtures/codex-weekly-window-readings.jsonl" --thresholds 70,90
cap_replayed="$cap_out"
contains "the window that approached the wall alerts at 70" \
  "$cap_replayed" "crossed 70% at 70%"
contains "and again on entering the band that window peaked in" \
  "$cap_replayed" "crossed 90% at 92%"
equal "four recorded windows produce two alerts and no others" \
  "alerts: 2" "$(printf '%s\n' "$cap_replayed" | tail -1)"
cap_pass "replaying with every disposition named refuses nothing" 0 \
  "$GANG" cap replay \
  --readings "$ROOT/test/fixtures/codex-weekly-window-readings.jsonl" --verbose
equal "and the reading belonging to a closed window is refused there too" \
  1 "$(printf '%s\n' "$cap_out" | grep -c superseded)"

# THE SAMPLING TIMER RUNS THE SAME COMMAND A PERSON WOULD.
cap_pass "printing the timer units refuses nothing" 0 \
  "$GANG" cap watch --print-units --interval 15min
cap_units="$cap_out"
contains "the timer runs one sampling pass" "$cap_units" "cap check"
contains "and repeats on the interval it was given" "$cap_units" "OnUnitActiveSec=15min"
cap_unit_dir="$RUN_ROOT/cap-units"
cap_pass "installing the timer is an ordinary pass" 0 \
  "${cap_env[@]}" "GANG_CAP_UNIT_DIR=$cap_unit_dir" "$GANG" cap watch
equal "installing writes both halves of the timer" \
  "gangline-cap.service gangline-cap.timer" \
  "$(cd "$cap_unit_dir" && printf '%s ' * | sed 's/ $//')"
cap_pass "clearing it is an ordinary pass too" 0 \
  "${cap_env[@]}" "GANG_CAP_UNIT_DIR=$cap_unit_dir" "$GANG" cap watch --clear
equal "and clearing takes both away" "" "$(ls -A "$cap_unit_dir")"

refuses "an action gang cap does not have is named rather than guessed at" \
  "cap: unknown action 'sample'" "$GANG" cap sample

# ONE CROSSING IS ONE ALERT EVEN WHEN PASSES OVERLAP. A manual check and the
# timer's own pass can run at the same moment; a reading folded in without an
# ownership boundary lets every concurrent pass alert on the same crossing and
# lets each one overwrite the fired state the others just wrote.
cap_race_root="$RUN_ROOT/cap-race"
cap_race_rows="$RUN_ROOT/cap-race-rows"
cat > "$cap_race_rows" <<ROWS
Current week (all models)	72	$cap_reset_one	1789430000
ROWS
# WHAT THE LOCK IS FOR, ASSERTED FROM INSIDE IT. While a pass is folding a
# reading in, the history is exclusively its own: the reader runs inside that
# ownership, so a second attempt on the same lock from there must be refused.
# This says mutual exclusion outright rather than racing for it, which no
# arrangement of concurrent passes can do without a wall-clock delay.
cap_hold_root="$RUN_ROOT/cap-hold"
cap_hold_witness="$RUN_ROOT/cap-hold-witness"
cap_pass "a pass that owns the history completes" 0 \
  env "GANG_CAP_DIR=$cap_hold_root" \
  "GANG_CAP_CLAUDE_READER=if flock -n '$cap_hold_root/lock' -c true; then printf unheld > '$cap_hold_witness'; else printf held > '$cap_hold_witness'; fi; cat '$cap_race_rows'" \
  "$GANG" cap check --providers claude --claude-interval 0 --thresholds 70
equal "and nothing else can take it while that pass is inside" \
  held "$(cat "$cap_hold_witness")"

# The passes are released together and every one of them is waited on by pid,
# so a contender that refuses instead of taking its turn is a failure here
# rather than a line nobody reads. Serializing is the property: each pass must
# finish its own transaction, not step aside.
cap_race_pids=""
for cap_race_n in 1 2 3 4 5 6 7 8 9 10 11 12; do
  env "GANG_CAP_DIR=$cap_race_root" \
    "GANG_CAP_CLAUDE_READER=cat '$cap_race_rows'" \
    "$GANG" cap check --providers claude --claude-interval 0 --thresholds 70 \
    > "$cap_race_root.out.$cap_race_n" 2>&1 &
  cap_race_pids="$cap_race_pids $!"
done
cap_race_failed=0
cap_race_said=""
for cap_race_pid in $cap_race_pids; do
  wait "$cap_race_pid" || cap_race_failed=$((cap_race_failed + 1))
done
# A count on its own says how many turns were refused and nothing about why, so
# the refusals themselves are carried into the line that reports them.
if [ "$cap_race_failed" -ne 0 ]; then
  cap_race_said=" saying: $(cat "$cap_race_root".out.* | tr '\n' ' ')"
fi
equal "twelve overlapping passes record one alert for one crossing" \
  1 "$(cap_count "$cap_race_root/alerts.jsonl")"
equal "and every one of them completes its own turn" \
  0 "$cap_race_failed$cap_race_said"
equal "and exactly one of them announces the crossing" \
  1 "$(cat "$cap_race_root".out.* | grep -c 'at or past 70%')"

# A WINDOW WHOSE RESET HAS PASSED IS NOT THE STANDING ONE, even on a host that
# has recorded nothing yet. A session file keeps replaying the last snapshot it
# held, so the first reading on a quiet host can be an allowance already spent.
cap_expired_root="$RUN_ROOT/cap-expired"
cap_expired_sessions="$RUN_ROOT/cap-expired-sessions/2026/09/01"
mkdir -p "$cap_expired_sessions"
cap_session "$cap_expired_sessions/rollout-expired.jsonl" codex 99 1788000000 \
  "2026-08-30T18:00:00.000Z"
cap_pass "a pass with no live window left refuses" 1 \
  env "GANG_CAP_DIR=$cap_expired_root" \
  "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-expired-sessions" \
  "$GANG" cap check --providers codex --thresholds 70
cap_expired="$cap_out"
excludes "a first reading whose window already reset raises no alert" \
  "$cap_expired" "at or past 70%"
equal "and it is stored as the closed window it belongs to" \
  superseded "$(python3 -c '
import json, sys
print(json.loads(open(sys.argv[1]).read().splitlines()[-1]).get("disposition", "<absent>"))
' "$cap_expired_root/readings.jsonl")"
contains "and the pass says so rather than reporting a quiet provider" \
  "$cap_expired" "every window it published has closed"
cap_pass "showing a tree whose window closed refuses nothing" 0 \
  env "GANG_CAP_DIR=$cap_expired_root" "$GANG" cap show
excludes "and its percentage is never presented as the standing figure" \
  "$cap_out" "99% used"

# A TREE NOTHING HAS WRITTEN TO THIS WINDOW STILL ANSWERS WITH WHAT IT HOLDS.
touch -d '2026-08-30 18:00:00' "$cap_expired_sessions/rollout-expired.jsonl"
cap_pass "and a tree untouched this window refuses the same way" 1 \
  env "GANG_CAP_DIR=$RUN_ROOT/cap-cold" \
  "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-expired-sessions" \
  "$GANG" cap check --providers codex --thresholds 70
cap_cold="$cap_out"
contains "a tree untouched this window names the window it does hold" \
  "$cap_cold" "every window it published has closed"

# AND A WINDOW THAT WAS LIVE WHEN IT WAS READ IS NOT STILL LIVE AFTERWARDS.
cap_lapsed_root="$RUN_ROOT/cap-lapsed"
mkdir -p "$cap_lapsed_root"
cat > "$cap_lapsed_root/state.json" <<'JSON'
{"windows":{"codex\tcodex weekly":{"provider":"codex","label":"codex weekly","window":"weekly","used":64,"peak":64,"resets_at":1788000000,"observed":1787900000,"at":1787900000,"fired":[],"source":"codex-session-file"}},"providers":{}}
JSON
cap_pass "showing a window that lapsed since it was read refuses nothing" 0 \
  env "GANG_CAP_DIR=$cap_lapsed_root" "$GANG" cap show
contains "a window whose reset has passed is shown as closed, not as standing" \
  "$cap_out" "window closed"

# THE NEWEST READING IS THE ONE THE PROVIDER PUBLISHED LAST, not the one in the
# file the filesystem touched last. Sessions run concurrently, so a file can be
# appended to after another file has recorded a later snapshot.
cap_order_sessions="$RUN_ROOT/cap-order-sessions/2026/09/09"
mkdir -p "$cap_order_sessions"
cap_session "$cap_order_sessions/rollout-late.jsonl" codex 88 "$cap_reset_one" \
  "2026-09-09T19:00:00.000Z"
cap_session "$cap_order_sessions/rollout-early.jsonl" codex 11 "$cap_reset_one" \
  "2026-09-09T18:00:00.000Z"
touch -d '2026-09-09 11:00:00' "$cap_order_sessions/rollout-late.jsonl"
touch -d '2026-09-09 12:00:00' "$cap_order_sessions/rollout-early.jsonl"
cap_order_root="$RUN_ROOT/cap-order"
cap_pass "reading a tree of concurrent sessions is an ordinary pass" 0 \
  env "GANG_CAP_DIR=$cap_order_root" \
  "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-order-sessions" \
  "$GANG" cap check --providers codex --thresholds 70
cap_order="$cap_out"
contains "and the reading it took is the one it alerts on" \
  "$cap_order" "is 88% used, at or past 70%"
cap_pass "showing what that pass recorded refuses nothing" 0 \
  env "GANG_CAP_DIR=$cap_order_root" "$GANG" cap show
contains "the later published snapshot wins over the later written file" \
  "$cap_out" "codex weekly: 88% used"

# AND A SEARCH BOUND MUST NOT TURN A PRESENT READING INTO AN ABSENT ONE. A run
# of sessions on a model with its own pool is common, and the account's window
# is still on disk behind them.
cap_deep_sessions="$RUN_ROOT/cap-deep-sessions/2026/09/09"
mkdir -p "$cap_deep_sessions"
cap_session "$cap_deep_sessions/rollout-account.jsonl" codex 44 "$cap_reset_one" \
  "2026-09-09T10:00:00.000Z"
touch -d '2026-09-09 10:00:00' "$cap_deep_sessions/rollout-account.jsonl"
for n in $(seq -w 1 30); do
  cap_session "$cap_deep_sessions/rollout-pool-$n.jsonl" codex_bengalfox 3 \
    "$cap_reset_one" "2026-09-09T11:00:00.000Z"
  touch -d '2026-09-09 11:00:00' "$cap_deep_sessions/rollout-pool-$n.jsonl"
done
cap_deep_root="$RUN_ROOT/cap-deep"
cap_pass "reading past thirty other-pool sessions is an ordinary pass" 0 \
  env "GANG_CAP_DIR=$cap_deep_root" \
  "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-deep-sessions" \
  "$GANG" cap check --providers codex --thresholds 70
cap_pass "and showing what it found refuses nothing" 0 \
  env "GANG_CAP_DIR=$cap_deep_root" "$GANG" cap show
contains "an account reading behind thirty other-pool sessions is still found" \
  "$cap_out" "codex weekly: 44% used"

# AN OLDER OBSERVATION OF THE SAME WINDOW DOES NOT REPLACE A NEWER ONE.
cap_back_rows="$RUN_ROOT/cap-back-rows"
cap_back_root="$RUN_ROOT/cap-back"
cap_back() { # $1 = used, $2 = observation epoch
  cat > "$cap_back_rows" <<ROWS
Current week (all models)	$1	$cap_reset_one	$2
ROWS
  env "GANG_CAP_DIR=$cap_back_root" "GANG_CAP_CLAUDE_READER=cat '$cap_back_rows'" \
    "$GANG" cap check --providers claude --claude-interval 0 --thresholds 95
}
cap_pass "recording the standing reading is an ordinary pass" 0 \
  cap_back 60 1789430000
equal "and it says nothing, because nothing crossed" "" "$cap_out"
cap_pass "and so is refusing the one observed before it" 0 \
  cap_back 20 1789420000
equal "which is also silent" "" "$cap_out"
cap_pass "showing the standing reading after it refuses nothing" 0 \
  env "GANG_CAP_DIR=$cap_back_root" "$GANG" cap show
contains "a reading observed earlier than the standing one does not replace it" \
  "$cap_out" "60% used"
equal "and it is kept, marked as the stale duplicate it is" \
  stale "$(python3 -c '
import json, sys
print(json.loads(open(sys.argv[1]).read().splitlines()[-1]).get("disposition", "<absent>"))
' "$cap_back_root/readings.jsonl")"

# A PERCENTAGE IS ONLY EVER ONE A PROVIDER PUBLISHED, on every surface that
# prints one. Replay input and the state file are both outside the readers, so
# a row invented there must be refused rather than rendered as a measurement.
cap_forged_readings="$RUN_ROOT/cap-forged-readings.jsonl"
cat > "$cap_forged_readings" <<'JSON'
{"at":1789430000,"provider":"codex","label":"codex weekly","window":"weekly","status":"unreadable","reason":"no session files","used":42,"resets_at":1789435568,"observed":1789430000}
JSON
refuses "a replay row that records no reading cannot supply a percentage" \
  "line 1" "$GANG" cap replay --readings "$cap_forged_readings" --thresholds 70
cap_forged_root="$RUN_ROOT/cap-forged"
mkdir -p "$cap_forged_root"
cat > "$cap_forged_root/state.json" <<'JSON'
{"windows":{"codex\tcodex weekly":{"provider":"codex","label":"codex weekly","window":"weekly","used":37,"peak":37,"resets_at":1789435568,"observed":1789430000,"at":1789430000,"fired":[]}},"providers":{}}
JSON
refuses "a stored window with no published source is not printed as one" \
  "no source" env "GANG_CAP_DIR=$cap_forged_root" "$GANG" cap show

# BLINDNESS IS MEASURED FROM THE FIRST FAILURE, not from the last success. A
# host that sampled, paused, and then failed once has been unreadable for one
# pass, and calling that a long silence spends the alert on nothing.
cap_gap_root="$RUN_ROOT/cap-gap"
mkdir -p "$cap_gap_root"
cat > "$cap_gap_root/state.json" <<'JSON'
{"windows":{},"providers":{"codex":{"last_ok":1788000000}}}
JSON
cap_pass "a pass that could not read refuses even before it is called blind" 1 \
  env "GANG_CAP_DIR=$cap_gap_root" \
  "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-empty-sessions" \
  "$GANG" cap check --providers codex --stale-seconds 600
cap_gap="$cap_out"
excludes "one failed pass after a long pause is not reported as a long silence" \
  "$cap_gap" "usage telemetry has been unreadable since"
equal "and nothing is alerted for it" 0 "$(cap_count "$cap_gap_root/alerts.jsonl")"

# AN ALLOWED SOURCE NAME IS NOT PROVENANCE ON ITS OWN. Each provider publishes
# on one surface; a row naming the other provider's surface was not published by
# the provider it claims, whatever the name it carries.
cap_crossed_root="$RUN_ROOT/cap-crossed"
mkdir -p "$cap_crossed_root"
cat > "$cap_crossed_root/state.json" <<'JSON'
{"windows":{"codex\tcodex weekly":{"provider":"codex","label":"codex weekly","window":"weekly","used":42,"peak":42,"resets_at":1799435568,"observed":1789430000,"at":1789430000,"fired":[],"source":"claude-usage-turn"}},"providers":{}}
JSON
refuses "a row carrying the other provider's surface is not printed as a reading" \
  "no source codex publishes on" \
  env "GANG_CAP_DIR=$cap_crossed_root" "$GANG" cap show

# AN ALERT WAITING TO BE DELIVERED IS NOT TEXT TO BE REPRINTED ON TRUST. The
# state file is a surface a person can write to, and an alert read back from it
# reaches the operator as a measurement, so it is checked like one.
cap_forged_alert_root="$RUN_ROOT/cap-forged-alert"
mkdir -p "$cap_forged_alert_root"
cat > "$cap_forged_alert_root/state.json" <<'JSON'
{"windows":{},"providers":{},"pending":[{"kind":"threshold","text":"codex weekly is 66% used, invented in pending"}]}
JSON
cap_pass "an alert with no checkable fields refuses the pass" 1 \
  env "GANG_CAP_DIR=$cap_forged_alert_root" \
  "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-sessions" \
  "$GANG" cap check --providers codex
cap_forged_alert="$cap_out"
excludes "and its invented percentage never reaches the operator" \
  "$cap_forged_alert" "66%"

# AND A DELIVERABLE ALERT IS READ FROM ITS FIELDS, NOT FROM ITS SENTENCE.
cap_relabel_root="$RUN_ROOT/cap-relabel"
mkdir -p "$cap_relabel_root"
cat > "$cap_relabel_root/state.json" <<'JSON'
{"windows":{},"providers":{},"pending":[{"kind":"threshold","at":1789430000,"provider":"codex","label":"codex weekly","source":"codex-session-file","threshold":70,"used":74,"resets_at":1799435568,"text":"codex weekly is 66% used, invented in pending"}]}
JSON
cap_pass "a pass carrying an owed alert completes" 0 \
  env "GANG_CAP_DIR=$cap_relabel_root" \
  "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-sessions" \
  "$GANG" cap check --providers codex --thresholds 70
cap_relabel="$cap_out"
contains "an alert owed from an interrupted pass is delivered" \
  "$cap_relabel" "codex weekly is 74% used, at or past 70%"
excludes "in the words its own fields give it" "$cap_relabel" "invented in pending"

# ONE CROSSING IS ONE DURABLE RECORD, however often a pass is interrupted. A
# pass killed between writing the alert down and clearing what it owed leaves
# the same alert owed again, so the record is written by identity and the
# second attempt adds nothing.
cap_again_root="$RUN_ROOT/cap-again"
mkdir -p "$cap_again_root"
cat > "$cap_again_root/state.json" <<'JSON'
{"windows":{},"providers":{},"pending":[{"kind":"threshold","at":1789430000,"provider":"codex","label":"codex weekly","source":"codex-session-file","threshold":70,"used":74,"resets_at":1799435568}]}
JSON
cat > "$cap_again_root/alerts.jsonl" <<'JSON'
{"at":1789430000,"id":"threshold\tcodex\tcodex weekly\t1799435568\t70","kind":"threshold","label":"codex weekly","provider":"codex","resets_at":1799435568,"source":"codex-session-file","text":"codex weekly is 74% used, at or past 70%","threshold":70,"used":74}
JSON
cap_pass "and so does one whose alert is already on disk" 0 \
  env "GANG_CAP_DIR=$cap_again_root" \
  "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-sessions" \
  "$GANG" cap check --providers codex --thresholds 70
cap_again="$cap_out"
equal "an alert already written down is not written a second time" \
  1 "$(cap_count "$cap_again_root/alerts.jsonl")"
contains "and it is still delivered, because delivery is what was interrupted" \
  "$cap_again" "at or past 70%"

# AN ALERT IS NOT FORGOTTEN UNTIL ITS RECORD CAN BE READ BACK. A write stopped
# part way leaves a line with no end; appending to that line would splice the
# two into one unreadable line, and clearing what was owed on the strength of a
# call that returned would lose the crossing for good.
cap_torn_root="$RUN_ROOT/cap-torn"
mkdir -p "$cap_torn_root"
cat > "$cap_torn_root/state.json" <<'JSON'
{"windows":{},"providers":{},"pending":[{"kind":"threshold","at":1789430000,"provider":"codex","label":"codex weekly","source":"codex-session-file","threshold":70,"used":74,"resets_at":1799435568}]}
JSON
printf '%s' '{"at":1789430000,"id":"threshold	codex	codex weekly	1799435568	70","kind":"thres'   > "$cap_torn_root/alerts.jsonl"
cap_pass "a pass that finds a torn record still completes" 0 \
  env "GANG_CAP_DIR=$cap_torn_root" \
  "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-sessions" \
  "$GANG" cap check --providers codex --thresholds 70
cap_torn="$cap_out"
contains "the alert it owed is delivered" "$cap_torn" "at or past 70%"
equal "and lands as a record of its own, not spliced onto the broken one" \
  1 "$(python3 -c '
import json, sys
whole = 0
for line in open(sys.argv[1]):
    try:
        record = json.loads(line)
    except ValueError:
        continue
    if isinstance(record, dict) and record.get("kind") == "threshold":
        whole += 1
print(whole)
' "$cap_torn_root/alerts.jsonl")"
equal "and nothing is left owed once it is on disk" \
  '[]' "$(python3 -c '
import json, sys
print(json.dumps(json.load(open(sys.argv[1])).get("pending", "<absent>")))
' "$cap_torn_root/state.json")"

# AND AN ALERT WHOSE RECORD CANNOT BE WRITTEN AT ALL STAYS OWED, rather than
# being announced once and forgotten.
cap_unwritable_root="$RUN_ROOT/cap-unwritable"
mkdir -p "$cap_unwritable_root"
cp "$cap_torn_root/state.json" "$cap_unwritable_root/state.json"
cat > "$cap_unwritable_root/state.json" <<'JSON'
{"windows":{},"providers":{},"pending":[{"kind":"threshold","at":1789430000,"provider":"codex","label":"codex weekly","source":"codex-session-file","threshold":70,"used":74,"resets_at":1799435568}]}
JSON
mkdir -p "$cap_unwritable_root/alerts.jsonl"
cap_pass "a pass whose alert record cannot be written refuses" 1 \
  env "GANG_CAP_DIR=$cap_unwritable_root" \
  "GANG_CAP_CODEX_SESSIONS=$RUN_ROOT/cap-sessions" \
  "$GANG" cap check --providers codex --thresholds 70
cap_unwritable="$cap_out"
contains "and names the alert it could not write down" \
  "$cap_unwritable" "could not be read back"
equal "and keeps it owed for the next pass" \
  1 "$(python3 -c '
import json, sys
print(len(json.load(open(sys.argv[1])).get("pending", [])))
' "$cap_unwritable_root/state.json")"

# A UNIT VALUE SURVIVES THE OPERATOR'S OWN NOTIFIER. An unquoted space splits one
# assignment into several and systemd drops the rest with a warning nothing
# reads, so the timer would sample with values nobody wrote.
cap_pass "a spaced notifier command does not refuse the unit" 0 \
  env "GANG_CAP_NOTIFY=notify-send 'Gang cap' 100%" \
  "GANG_CAP_DIR=$RUN_ROOT/cap units" "$GANG" cap watch --print-units
cap_spaced="$cap_out"
equal "each environment value is one assignment, whatever is in it" \
  2 "$(printf '%s\n' "$cap_spaced" | grep -c '^Environment=')"
contains "a notifier command keeps its arguments" \
  "$cap_spaced" "Environment=GANG_CAP_NOTIFY=\"notify-send 'Gang cap' 100%%\""
contains "and a path with a space is not cut at the space" \
  "$cap_spaced" "Environment=GANG_CAP_DIR=\"$RUN_ROOT/cap units\""

# EVERYTHING THIS COMMAND WRITES CAN BE REMOVED.
cap_pass "removing the history is an ordinary pass" 0 \
  "${cap_env[@]}" "$GANG" cap forget
cap_forget="$cap_out"
contains "the recorded history names its own removal" "$cap_forget" "removed"
equal "and nothing it wrote is left behind" 0 "$(cap_count "$cap_readings")"
equal "including the lock it coordinated on" \
  absent "$(if [ -e "$cap_root/lock" ]; then printf present; else printf absent; fi)"
