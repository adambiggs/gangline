# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Usage: the launch record hitch registers, the ccusage join behind gang usage, and the event record drop and down append.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# ccusage is stood in for by executables on a private PATH prefix: one that
# prints the documented session shape, one that exits with an error, and two
# that print shapes gang must refuse. The real tool is never run here.

usage_bin="$RUN_ROOT/usage-bin"
# EVERY DROP IN THE SUITE APPENDS A RECORD, so this part reads a data root of
# its own rather than counting lines other parts wrote.
usage_data="$RUN_ROOT/usage-data"
usage_events="$usage_data/gangline/usage/events.jsonl"
mkdir -p "$usage_bin/present" "$usage_bin/failing" "$usage_bin/garbage" "$usage_bin/unjson"
cat > "$usage_bin/present/ccusage" <<'SH'
#!/usr/bin/env bash
# The session shape ccusage 20 prints: one Claude Code row whose period is the
# session id, one Codex row whose period is a rollout stem ending in it, and
# one Claude row keyed by a rollout-shaped stem, which only a Codex row may
# join by. Every call appends its argv, so a count of calls is a real count.
printf '%s\n' "$*" >> "${USAGE_FIXTURE_ARGV:-/dev/null}"
cat <<'JSON'
{"session":[
 {"agent":"claude","period":"11111111-aaaa-4bbb-8ccc-000000000001","inputTokens":10,"outputTokens":20,"cacheReadTokens":300,"cacheCreationTokens":40,"totalTokens":370,"modelsUsed":["claude-opus-5"],"modelBreakdowns":[{"modelName":"claude-opus-5","inputTokens":10,"outputTokens":20,"cacheReadTokens":300,"cacheCreationTokens":40}],"metadata":{"lastActivity":"2026-09-02T10:00:00.000Z"}},
 {"agent":"codex","period":"2026/09/02/rollout-2026-09-02T00-00-00-22222222-bbbb-4ccc-8ddd-000000000002","inputTokens":5,"outputTokens":6,"cacheReadTokens":7,"cacheCreationTokens":0,"totalTokens":18,"modelsUsed":["gpt-5.6"],"modelBreakdowns":[{"modelName":"gpt-5.6","inputTokens":5,"outputTokens":6,"cacheReadTokens":7,"cacheCreationTokens":0}],"metadata":{"lastActivity":"2026-09-02T10:00:00.000Z","reasoningOutputTokens":3}},
 {"agent":"claude","period":"rollout-33333333-cccc-4ddd-8eee-000000000003","inputTokens":1000,"outputTokens":1000,"cacheReadTokens":1000,"cacheCreationTokens":1000,"totalTokens":4000,"modelsUsed":["claude-opus-5"],"modelBreakdowns":[{"modelName":"claude-opus-5","inputTokens":1000,"outputTokens":1000,"cacheReadTokens":1000,"cacheCreationTokens":1000}],"metadata":{"lastActivity":"2026-09-02T10:00:00.000Z"}}
],"totals":{"inputTokens":15,"outputTokens":26,"cacheReadTokens":307,"cacheCreationTokens":40,"totalTokens":388}}
JSON
SH
cat > "$usage_bin/failing/ccusage" <<'SH'
#!/usr/bin/env bash
echo "fixture: no transcripts here" >&2
exit 3
SH
cat > "$usage_bin/garbage/ccusage" <<'SH'
#!/usr/bin/env bash
printf '%s\n' '{"session":[{"agent":"claude","period":"11111111-aaaa-4bbb-8ccc-000000000001","inputTokens":"lots","outputTokens":20,"cacheReadTokens":300,"cacheCreationTokens":40,"modelBreakdowns":[]}]}'
SH
cat > "$usage_bin/unjson/ccusage" <<'SH'
#!/usr/bin/env bash
printf 'Loading transcripts...\n'
SH
chmod +x "$usage_bin"/*/ccusage
usage_present="$usage_bin/present:$PATH"
usage_failing="$usage_bin/failing:$PATH"
usage_garbage="$usage_bin/garbage:$PATH"
usage_unjson="$usage_bin/unjson:$PATH"
# ABSENT IS PROVEN, NOT ASSUMED: the suite's own PATH is asked whether it finds
# a ccusage before it is used as the absent case, so a host that installed one
# reads as unknown rather than as a silently passing absence. The PATH is not
# trimmed to stage absence because the suite's tmux guard and python live on it.
usage_absent="$PATH"
usage_absent_proven=1
command -v ccusage >/dev/null 2>&1 && usage_absent_proven=0

usage_claude_id="11111111-aaaa-4bbb-8ccc-000000000001"
usage_codex_id="22222222-bbbb-4ccc-8ddd-000000000002"
usage_stray_id="33333333-cccc-4ddd-8eee-000000000003"
# --- The launch record hitch registers -------------------------------------
"$HITCH" usage-alpha -c bash -d /tmp -t 'github:gangline#421' >/dev/null
usage_alpha_id="$(window_id usage-alpha)"
# THE TEAM IS READ ONLY AFTER THIS PART HAS HITCHED INTO IT. This part may
# begin after the previous part dropped the session's last window, which is
# how tmux ends a session; gang hitch creates the session when it is absent,
# so the hitch above is what guarantees there is a team to read.
# Read the whole client response before selecting one line: closing a pipe at
# head while tmux writes the other shared-team windows can take this test's
# server down.
usage_team_created="$(tmux list-windows -t "=$GANG_SESSION" -F '#{session_created}' 2>/dev/null)" \
  || usage_team_created=""
usage_team_created="${usage_team_created%%$'\n'*}"
if [[ "$usage_team_created" =~ ^[0-9]+$ ]]; then
  pass "the team epoch is read from a live session this part hitched into"
else
  fail "the team epoch is read from a live session this part hitched into" \
    "no live session '$GANG_SESSION' to read: [$usage_team_created]"
fi
equal "hitch registers the launch directory" \
  "/tmp" "$(tmux show-options -wqv -t "$usage_alpha_id" @gl_dir)"
equal "hitch registers the task label verbatim" \
  "github:gangline#421" "$(tmux show-options -wqv -t "$usage_alpha_id" @gl_task)"
equal "hitch registers an empty model where the collar takes none" \
  "" "$(tmux show-options -wqv -t "$usage_alpha_id" @gl_model)"
usage_alpha_hitched="$(tmux show-options -wqv -t "$usage_alpha_id" @gl_hitched_at)"
if [[ "$usage_alpha_hitched" =~ ^[0-9]+$ ]]; then
  pass "hitch registers a whole-number wall-clock start"
else
  fail "hitch registers a whole-number wall-clock start" "read [$usage_alpha_hitched]"
fi

usage_bad_task_out="$("$HITCH" usage-bad-task -c bash -d /tmp -t $'a\tb' 2>&1)" \
  && fail "hitch refuses a task label with a control character" "hitch succeeded: [$usage_bad_task_out]" \
  || contains "hitch refuses a task label with a control character" \
       "$usage_bad_task_out" "control characters"
usage_long_task="$(printf 'x%.0s' $(seq 1 201))"
usage_long_task_out="$("$HITCH" usage-long-task -c bash -d /tmp -t "$usage_long_task" 2>&1)" \
  && fail "hitch refuses a task label longer than 200 characters" "hitch succeeded: [$usage_long_task_out]" \
  || contains "hitch refuses a task label longer than 200 characters" \
       "$usage_long_task_out" "longer than 200"
excludes "a refused task label leaves no window behind" \
  "$(window_names)" "usage-bad-task"

# --- gang usage with the documented shape on PATH --------------------------
"$HITCH" usage-beta -c bash -d /tmp >/dev/null
"$HITCH" usage-gamma -c bash -d /tmp >/dev/null
"$HITCH" usage-delta -c bash -d /tmp >/dev/null
usage_beta_id="$(window_id usage-beta)"
tmux set-option -w -t "$usage_alpha_id" @gl_session_id "$usage_claude_id"
tmux set-option -w -t "$usage_beta_id" @gl_session_id "$usage_codex_id"
tmux set-option -w -t "$(window_id usage-delta)" @gl_session_id "$usage_stray_id"
usage_argv="$RUN_ROOT/usage-argv"
usage_out="$(USAGE_FIXTURE_ARGV="$usage_argv" PATH="$usage_present" XDG_DATA_HOME="$usage_data" "$GANG" usage 2>&1)" \
  || fail "gang usage succeeds with ccusage present" "status $?: [$usage_out]"
tmux has-session -t "=$GANG_SESSION" 2>/dev/null \
  && pass "reading usage leaves the live team's tmux server intact" \
  || fail "reading usage leaves the live team's tmux server intact" \
       "session $GANG_SESSION disappeared"
equal "gang usage asks ccusage for the offline costless session report" \
  "session --json --no-cost --offline" "$(<"$usage_argv")"
usage_alpha_row="$(printf '%s\n' "$usage_out" | awk '$1 == "usage-alpha"')"
# Columns for a bash-collar agent with no model or effort: agent, harness,
# state, duration, input, output, cache-r, cache-w, usage, task.
equal "a Claude-shaped id joins by exact period" \
  "bash live 10 20 300 40 matched" \
  "$(printf '%s\n' "$usage_alpha_row" | awk '{ print $2, $3, $5, $6, $7, $8, $9 }')"
equal "the task label is a column of the report" \
  "github:gangline#421" "$(printf '%s\n' "$usage_alpha_row" | awk '{ print $10 }')"
equal "a Claude row keyed by a rollout-shaped stem joins by suffix for nobody" \
  "bash live unmatched" \
  "$(printf '%s\n' "$usage_out" | awk '$1 == "usage-delta" { print $2, $3, $5 }')"
equal "a Codex-shaped id joins by the rollout period's trailing id" \
  "5 6 7 0 matched" \
  "$(printf '%s\n' "$usage_out" | awk '$1 == "usage-beta" { print $5, $6, $7, $8, $9 }')"
equal "an unstamped agent is shown with no token columns" \
  "bash live unstamped" \
  "$(printf '%s\n' "$usage_out" | awk '$1 == "usage-gamma" { print $2, $3, $5 }')"
contains "the uncovered list names the unstamped agent" \
  "$usage_out" "usage-gamma: unstamped"
equal "the per-model roll-up sums ccusage's breakdown for claude-opus-5" \
  "1 10 20 300 40" \
  "$(printf '%s\n' "$usage_out" | awk '$1 == "claude-opus-5" { print $2, $3, $4, $5, $6 }')"
equal "the per-model roll-up sums ccusage's breakdown for gpt-5.6" \
  "1 5 6 7 0" \
  "$(printf '%s\n' "$usage_out" | awk '$1 == "gpt-5.6" { print $2, $3, $4, $5, $6 }')"
excludes "no Codex note is printed for a team without a codex collar" \
  "$usage_out" "experimental"
contains "gang usage prints the record path" \
  "$usage_out" "record: $usage_events"
usage_arity_out="$(XDG_DATA_HOME="$usage_data" "$GANG" usage --all extra 2>&1)" \
  && fail "gang usage refuses an argument it does not take" "succeeded: [$usage_arity_out]" \
  || contains "gang usage refuses an argument it does not take" \
       "$usage_arity_out" "unexpected argument 'extra'"

# --- ccusage absent, failing, and printing shapes gang refuses --------------
if [ "$usage_absent_proven" -eq 1 ]; then
  usage_absent_out="$(PATH="$usage_absent" XDG_DATA_HOME="$usage_data" "$GANG" usage 2>&1)" \
    || fail "gang usage succeeds without ccusage" "status $?: [$usage_absent_out]"
  # task is the optional final column. Alpha has one; beta and gamma do not,
  # so status is respectively the penultimate or final nonblank table field.
  equal "without ccusage every live row is marked absent" \
  "absent absent absent" \
    "$(printf '%s\n' "$usage_absent_out" | awk '$1 ~ /^usage-(alpha|beta|gamma)$/ { print $1 == "usage-alpha" ? $(NF - 1) : $NF }' | tr '\n' ' ' | sed 's/ $//')"
  contains "without ccusage the uncovered list says why" \
    "$usage_absent_out" "ccusage absent: no ccusage executable on PATH"
else
  unknown "without ccusage every live row is marked absent" \
    "a ccusage is on the suite's PATH, so absence cannot be staged on this host"
  unknown "without ccusage the uncovered list says why" \
    "a ccusage is on the suite's PATH, so absence cannot be staged on this host"
fi
usage_failing_out="$(PATH="$usage_failing" XDG_DATA_HOME="$usage_data" "$GANG" usage 2>&1)" \
  || fail "gang usage succeeds when ccusage exits with an error" "status $?: [$usage_failing_out]"
contains "a failing ccusage is named with its status and first error line" \
  "$usage_failing_out" "ccusage failed: ccusage exited 3: fixture: no transcripts here"
equal "a failing ccusage leaves the matched rows unfilled" \
  "failed" "$(printf '%s\n' "$usage_failing_out" | awk '$1 == "usage-alpha" { print $(NF - 1) }')"
usage_garbage_out="$(PATH="$usage_garbage" XDG_DATA_HOME="$usage_data" "$GANG" usage 2>&1)" \
  || fail "gang usage succeeds when ccusage prints a count that is not a number" "status $?: [$usage_garbage_out]"
contains "a token count that is not a whole number is refused by name" \
  "$usage_garbage_out" "ccusage malformed: session row 1 inputTokens is not a whole number: 'lots'"
equal "a malformed report fills no row, matched id or not" \
  "malformed" "$(printf '%s\n' "$usage_garbage_out" | awk '$1 == "usage-alpha" { print $(NF - 1) }')"
usage_unjson_out="$(PATH="$usage_unjson" XDG_DATA_HOME="$usage_data" "$GANG" usage 2>&1)" \
  || fail "gang usage succeeds when ccusage prints prose" "status $?: [$usage_unjson_out]"
contains "prose from ccusage is refused as not JSON" \
  "$usage_unjson_out" "ccusage malformed: ccusage output is not JSON"

# --- drop appends the record -----------------------------------------------
[ ! -e "$usage_events" ] \
  && pass "no usage record exists before the first drop" \
  || fail "no usage record exists before the first drop" "$usage_events exists"
usage_drop_out="$(PATH="$usage_present" XDG_DATA_HOME="$usage_data" "$GANG" drop usage-alpha 2>&1)" \
  || fail "drop succeeds with ccusage present" "status $?: [$usage_drop_out]"
contains "drop reports the join it recorded" \
  "$usage_drop_out" "usage: usage-alpha matched"
contains "drop names the record it appended to" \
  "$usage_drop_out" "usage: recorded in $usage_events"
usage_record_check="$(python3 - "$usage_events" "$usage_alpha_hitched" "$usage_claude_id" "$usage_team_created" <<'PY'
import json, socket, sys
path, hitched, session = sys.argv[1], int(sys.argv[2]), sys.argv[3]
rows = [json.loads(line) for line in open(path, encoding="utf-8") if line.strip()]
if len(rows) != 1:
    print(f"expected one record, found {len(rows)}"); raise SystemExit(0)
r = rows[0]
expected = {
    "v": 1, "agent": "usage-alpha", "collar": "bash", "model": None, "effort": None,
    "dir": "/tmp", "task": "github:gangline#421", "session_id": session,
    "hitched_at": hitched, "ended_by": "drop", "usage_status": "matched",
    "team_created": int(sys.argv[4]),
    "usage_note": None, "usage_agent": "claude", "input": 10, "output": 20,
    "cache_read": 300, "cache_write": 40, "last_activity": "2026-09-02T10:00:00.000Z",
    "models": [{"model": "claude-opus-5", "input": 10, "output": 20,
                "cache_read": 300, "cache_write": 40}],
    "host": socket.gethostname(),
}
for key, value in expected.items():
    if r.get(key) != value:
        print(f"{key}: expected {value!r}, recorded {r.get(key)!r}"); raise SystemExit(0)
if r.get("team") != r.get("team") or not isinstance(r.get("team"), str):
    print("team is not a string"); raise SystemExit(0)
if not isinstance(r.get("ended_at"), int) or r["ended_at"] < hitched:
    print(f"ended_at {r.get('ended_at')!r} is not an epoch at or after the start"); raise SystemExit(0)
if r.get("duration_s") != r["ended_at"] - hitched:
    print(f"duration_s {r.get('duration_s')!r} is not ended_at minus hitched_at"); raise SystemExit(0)
print("ok")
PY
)"
equal "the dropped agent's record carries the launch record and ccusage's reading" \
  "ok" "$usage_record_check"
usage_record_team="$(python3 -c 'import json,sys; print(json.loads(open(sys.argv[1]).readline())["team"])' "$usage_events")"
equal "the record names the team it was hitched into" "$GANG_SESSION" "$usage_record_team"

usage_after_out="$(PATH="$usage_failing" XDG_DATA_HOME="$usage_data" "$GANG" usage 2>&1)" \
  || fail "gang usage succeeds after a drop" "status $?: [$usage_after_out]"
equal "an ended agent prints what was recorded, not a fresh reading" \
  "bash ended 10 20 300 40 matched" \
  "$(printf '%s\n' "$usage_after_out" | awk '$1 == "usage-alpha" { print $2, $3, $5, $6, $7, $8, $9 }')"

# NOTHING A TEARDOWN SAYS MAY LAND ON AN AGENT'S SCREEN. The append crosses into
# the server's mount namespace through tmux run-shell, and tmux does not give a
# run-shell child's output back to the caller: it prints every line of it, and a
# line of its own for a nonzero exit, into a pane, forcing that pane into
# view-mode. With no -t and no client of its own the pane it picks belongs to
# whichever agent tmux found, and a pane in view-mode does not take a paste at
# its shell, so the next message delivered there is lost with nothing said
# anywhere. This path prints on success too, so it is every drop and every down,
# not an error branch.
#
# CALIBRATE THE INSTRUMENT ON THE FAULT IT MUST CATCH. A tmux that hands that
# output back to its caller cannot fail the claim below, and it reads exactly
# like a reader that cannot see a hijack, so the unguarded shape is driven once
# against a throwaway window made the session's active one — the pane an
# untargeted run-shell settles on — and a tmux that does not hijack leaves the
# claim unsettled rather than passing it.
usage_mode_active="$(tmux display-message -p -t "=$GANG_SESSION:" '#{window_id}')"
usage_mode_calib_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n usage-mode-calib 'exec cat')"
tmux select-window -t "$usage_mode_calib_id"
tmux run-shell "printf 'calibration\n'" >/dev/null 2>&1 || :
usage_mode_calibrated="$(tmux display-message -p -t "$usage_mode_calib_id" \
  '#{?pane_in_mode,#{pane_mode},none}')"
tmux kill-window -t "$usage_mode_calib_id"
tmux select-window -t "$usage_mode_active"
if [ "$usage_mode_calibrated" != view-mode ]; then
  unknown "a recorded teardown leaves every pane in the team out of any mode" \
    "the unguarded shape did not put the calibration pane in view-mode on this tmux, which left it in [$usage_mode_calibrated], so a hijack has nothing to show here and this claim cannot be read"
else
  pass "an unguarded run-shell does put this team's active pane in view-mode"
  # Its own data root: the records this part counts are indexed by position.
  "$HITCH" usage-mode-witness -c bash -d /tmp >/dev/null
  "$HITCH" usage-mode-ended -c bash -d /tmp >/dev/null
  PATH="$usage_present" XDG_DATA_HOME="$RUN_ROOT/usage-mode-data" \
    "$GANG" drop usage-mode-ended >/dev/null
  equal "a recorded teardown leaves every pane in the team out of any mode" "" \
    "$(tmux list-panes -s -t "=$GANG_SESSION" \
        -F '#{?pane_in_mode,#{window_name} #{pane_mode},}' | sed '/^$/d' | tr '\n' ' ')"
  PATH="$usage_present" XDG_DATA_HOME="$RUN_ROOT/usage-mode-data" \
    "$GANG" drop usage-mode-witness >/dev/null
fi

# THE REPORT IS PUBLISHED AFTER THE WORK, SO ITS ABSENCE PROVES NOTHING. A
# worker that never started, one that appended and then lost the server, and one
# that died mid-append all leave the caller with no report. What keeps the
# caller's diagnostic honest instead is that the staged copy of the event
# outlives the append: the caller owns that buffer and discards it only once a
# report has told it the event reached the record or a recovery file, so naming
# that buffer as the remaining copy is a fact rather than an inference. The
# ordering that has to hold is between the worker's append and its report, so
# the worker is driven directly here.
usage_worker_root="$RUN_ROOT/usage-worker"
usage_worker_sealed="$usage_worker_root/sealed"
mkdir -p "$usage_worker_root" "$usage_worker_sealed"
usage_worker_prepared="$(tail -n 1 "$usage_events")"
usage_worker_run() { # $1 buffer, $2 events, $3 fallback root, $4 report, $5 token
  printf '%s\n' "$usage_worker_prepared" | tmux load-buffer -b "$1" -
  GANG_USAGE_RECORD_INTERNAL=1 "$GANG" __usage-record-worker "$1" "$2" "$3" "$4" "$5"
}
usage_worker_buffer() { # $1 buffer name -> present|gone
  tmux list-buffers -F '#{buffer_name}' | grep -qxF "$1" && printf present || printf gone
}
usage_worker_token="0123456789abcdef0123456789abcdef"
usage_worker_said="$(usage_worker_run gang-usage-probe-ok \
  "$usage_worker_root/events.jsonl" "$usage_worker_root" \
  gang-usage-report-probe-ok "$usage_worker_token" 2>&1)" \
  || fail "the record worker ends on a green status" "status $?"
equal "the record worker says nothing where tmux would render it" "" "$usage_worker_said"
usage_worker_report="$(tmux save-buffer -b gang-usage-report-probe-ok -)"
equal "its report opens with this call's token and where the event ended up" \
  "gang-usage-report $usage_worker_token recorded" \
  "${usage_worker_report%%$'\n'*}"
equal "the event it reported on is in the record" "$usage_worker_prepared" \
  "$(cat "$usage_worker_root/events.jsonl")"
equal "and the staged copy is still there for the caller that owns it" present \
  "$(usage_worker_buffer gang-usage-probe-ok)"
# A worker that could neither append nor preserve the event says the staged copy
# is the only one, which is the caller's cue to keep it.
chmod 500 "$usage_worker_sealed"
usage_worker_kept="$(usage_worker_run gang-usage-probe-kept \
  "$usage_worker_sealed/events.jsonl" "$usage_worker_sealed" \
  gang-usage-report-probe-kept "$usage_worker_token" 2>&1)" \
  || fail "the record worker ends green when the append fails" "status $?"
equal "a failed append is silent where tmux would render it too" "" "$usage_worker_kept"
usage_worker_kept_report="$(tmux save-buffer -b gang-usage-report-probe-kept -)"
equal "an event that reached neither the record nor a recovery file is reported kept" \
  "gang-usage-report $usage_worker_token kept" \
  "${usage_worker_kept_report%%$'\n'*}"
contains "and the report carries what failed" "$usage_worker_kept_report" \
  "gang: usage record not written"
equal "and that staged copy is still there as well" present \
  "$(usage_worker_buffer gang-usage-probe-kept)"
chmod 700 "$usage_worker_sealed"
for usage_worker_leftover in $(tmux list-buffers -F '#{buffer_name}' \
    | sed -n '/^gang-usage-\(report-\)\?probe-/p'); do
  tmux delete-buffer -b "$usage_worker_leftover"
done

# A drop whose ccusage fails still ends the agent and records the failure.
usage_drop_failed_out="$(PATH="$usage_failing" XDG_DATA_HOME="$usage_data" "$GANG" drop usage-beta 2>&1)" \
  || fail "drop succeeds when ccusage fails" "status $?: [$usage_drop_failed_out]"
contains "drop records a failed ccusage read as such" \
  "$usage_drop_failed_out" "usage: usage-beta failed — ccusage exited 3: fixture: no transcripts here"
excludes "the dropped window is gone" "$(window_names)" "usage-beta"
equal "the failed read is the second record" \
  "usage-beta failed" \
  "$(python3 -c 'import json,sys; r=[json.loads(l) for l in open(sys.argv[1]) if l.strip()][1]; print(r["agent"], r["usage_status"])' "$usage_events")"

# Neither buffer outlives the teardown that made them: the caller reads its
# report, deletes it, and discards the staged copy the report accounted for.
equal "a recorded teardown leaves no usage buffer behind in the server" "" \
  "$(tmux list-buffers -F '#{buffer_name}' | grep '^gang-usage' | tr '\n' ' ')"

# The caller gets a private mount namespace in which its data root is read
# only. The tmux server was started outside that namespace and sees the same
# path writable, matching a sandboxed agent calling into its host tmux server.
# A mode bit cannot stage that boundary: caller and server have the same uid,
# so chmod would make the path writable or read-only for both of them. A real
# pane id exercises tmux's pane-client output routing, and the literal hashes
# prove that run-shell cannot interpret caller values as tmux formats.
if ! unshare -Ur -m true 2>/dev/null; then
  unknown "drop routes the append outside a read-only caller" \
    "unprivileged user and mount namespaces are unavailable here (unshare -Ur -m)"
  unknown "the server-side append records the agent dropped by a read-only caller" \
    "unprivileged user and mount namespaces are unavailable here (unshare -Ur -m)"
else
  "$HITCH" usage-readonly-caller -c bash -d /tmp >/dev/null
  usage_readonly_id="$(window_id usage-readonly-caller)"
  usage_readonly_pane="$(tmux list-panes -t "$usage_readonly_id" -F '#{pane_id}')"
  usage_readonly_data="$RUN_ROOT/usage readonly-##-data"
  usage_readonly_events="$usage_readonly_data/gangline/usage/events.jsonl"
  mkdir -p "$usage_readonly_data/gangline/usage"
  usage_readonly_out="$(
    unshare -Ur -m sh -eu -c '
      mount --bind "$1" "$1"
      mount -o remount,bind,ro "$1"
      env XDG_DATA_HOME="$1" PATH="$2" GANG_SESSION="$4" TMUX_TMPDIR="$5" \
        TMUX="$6,0,0" TMUX_PANE="$7" "$3" drop usage-readonly-caller
    ' usage-readonly-drop "$usage_readonly_data" "$usage_present" "$GANG" \
      "$GANG_SESSION" "$TMUX_TMPDIR" "$TMUX_SOCKET" "$usage_readonly_pane" 2>&1
  )" || fail "drop succeeds from a caller whose usage data root is read-only" \
    "status $?: [$usage_readonly_out]"
  contains "drop routes the append outside a read-only caller" \
    "$usage_readonly_out" "usage: recorded in $usage_readonly_events"
  usage_readonly_recorded="missing"
  if [ -f "$usage_readonly_events" ]; then
    usage_readonly_recorded="$(python3 -c 'import json,sys; print(json.loads(open(sys.argv[1]).readline())["agent"])' "$usage_readonly_events")"
  fi
  equal "the server-side append records the agent dropped by a read-only caller" \
    "usage-readonly-caller" "$usage_readonly_recorded"
fi

# A record that cannot be written is one stderr line, and the drop proceeds.
usage_blocked_data="$RUN_ROOT/usage-blocked-data"
usage_blocked_archive="$RUN_ROOT/usage archive-##-literal"
: > "$usage_blocked_data"
usage_drop_blocked_out="$(GANG_ARCHIVE_DIR="$usage_blocked_archive" \
  XDG_DATA_HOME="$usage_blocked_data" PATH="$usage_present" "$GANG" drop usage-gamma 2>&1)" \
  || fail "drop succeeds when the record cannot be written" "status $?: [$usage_drop_blocked_out]"
contains "an unwritable record is reported on the drop" \
  "$usage_drop_blocked_out" "usage record not written"
contains "an unwritable record names the path it could not append to" \
  "$usage_drop_blocked_out" "$usage_blocked_data/gangline/usage/events.jsonl"
contains "an unwritable record names the fallback that preserved its event" \
  "$usage_drop_blocked_out" "usage event preserved at $usage_blocked_archive/usage-unrecorded/"
usage_fallback_files=""
if [ -d "$usage_blocked_archive/usage-unrecorded" ]; then
  usage_fallback_files="$(find "$usage_blocked_archive/usage-unrecorded" -type f -name 'usage-*.jsonl' -print)"
fi
equal "one failed append preserves one fallback event" \
  "1" "$(printf '%s\n' "$usage_fallback_files" | sed '/^$/d' | wc -l | tr -d ' ')"
usage_fallback_recorded="missing"
if [ -n "$usage_fallback_files" ]; then
  usage_fallback_recorded="$(python3 -c 'import json,sys; print(json.loads(open(sys.argv[1]).readline())["agent"])' "$usage_fallback_files")"
fi
equal "the preserved fallback contains the event the primary path refused" \
  "usage-gamma" "$usage_fallback_recorded"
excludes "the drop with an unwritable record still ended the agent" \
  "$(window_names)" "usage-gamma"

# A PRESERVED EVENT IS ACCOUNTED FOR, so the caller discards the copy it staged
# and neither buffer outlives that teardown either.
equal "a preserved event leaves no usage buffer behind in the server" "" \
  "$(tmux list-buffers -F '#{buffer_name}' | grep '^gang-usage' | tr '\n' ' ')"

# THE CALLER'S OTHER BRANCHES ONLY RUN WHEN SOMETHING IS ALREADY BROKEN, and a
# branch a passing run never takes is the one that has to be right. Each is
# driven through a real teardown, with its own data root because the records
# this part counts are indexed by position.
usage_branch_data="$RUN_ROOT/usage-branch-data"
usage_branch_sealed="$RUN_ROOT/usage-branch-sealed"
mkdir -p "$usage_branch_sealed"
usage_branch_drop() { # $1 = agent, rest = environment for the drop
  local agent="$1"; shift
  "$HITCH" "$agent" -c bash -d /tmp >/dev/null
  env "$@" XDG_DATA_HOME="$usage_branch_data" PATH="$usage_present" \
    "$GANG" drop "$agent" 2>&1
}
usage_branch_identity() { # $1 = a buffer name -> the width of the identity in it
  local rest="${1#gang-usage-}"
  [ "$rest" != "$1" ] || { printf 'not a staged event: %s' "$1"; return; }
  case "$rest" in
    ''|*[!0-9a-f]*) printf 'not one minted identity: %s' "$rest" ;;
    *) printf '%s hex digits' "${#rest}" ;;
  esac
}
usage_branch_staged() { # -> the staging buffers left in the server, newest last
  tmux list-buffers -F '#{buffer_name}' | sed -n '/^gang-usage-[0-9a-f]/p' | tr '\n' ' '
}
# Take away whatever a case left, not whatever it was expected to leave. The
# checks have already read the difference by then, and a teardown that ends the
# run on a buffer that is not there would replace their verdict with its own.
usage_branch_discard() {
  local name
  for name in $(tmux list-buffers -F '#{buffer_name}' | sed -n '/^gang-usage/p'); do
    tmux delete-buffer -b "$name"
  done
}
# An event that reached neither the record nor a recovery file is the staged
# copy, and the caller keeps it and says where it is.
: > "$usage_branch_data"
chmod 500 "$usage_branch_sealed"
usage_kept_out="$(usage_branch_drop usage-kept \
  GANG_ARCHIVE_DIR="$usage_branch_sealed/archive")" \
  || fail "drop succeeds when neither the record nor a fallback can be written" "status $?"
chmod 700 "$usage_branch_sealed"
contains "an event that reached nothing is reported as still staged" \
  "$usage_kept_out" "gang: the prepared JSON remains in tmux buffer gang-usage-"
usage_kept_buffer="$(printf '%s\n' "$usage_kept_out" \
  | sed -n 's/.*remains in tmux buffer \(gang-usage-[0-9a-f]*\) .*/\1/p' | tail -n 1)"
equal "the staged event it names carries a whole call identity" "32 hex digits" \
  "$(usage_branch_identity "$usage_kept_buffer")"
equal "and that buffer is where the caller says it is" "$usage_kept_buffer " \
  "$(usage_branch_staged)"
usage_branch_discard
rm -f "$usage_branch_data"
# A report that is not there proves nothing about the append, and the staged
# copy is the caller's to keep saying so.
usage_missing_out="$(usage_branch_drop usage-noreport GANG_TEST_USAGE_REPORT_FAULT=missing)" \
  || fail "drop succeeds when its report is gone" "status $?"
contains "a report that is not there is reported as unknown, not as a failed append" \
  "$usage_missing_out" "gang: usage record left no report"
contains "and it does not claim the append did or did not happen" \
  "$usage_missing_out" "are both unknown"
usage_missing_buffer="$(usage_branch_staged)"
equal "a caller with no report keeps the copy it staged" "32 hex digits" \
  "$(usage_branch_identity "${usage_missing_buffer% }")"
usage_branch_discard
# A report under this call's name that is not this call's report is refused,
# and refusing it leaves both buffers where they are.
usage_foreign_out="$(usage_branch_drop usage-foreign GANG_TEST_USAGE_REPORT_FAULT=foreign)" \
  || fail "drop succeeds when its report is another call's" "status $?"
contains "a report without this call's token is refused" \
  "$usage_foreign_out" "gang: usage record report rejected"
excludes "a refused report is not read as a result" \
  "$usage_foreign_out" "a result nobody here asked for"
equal "a refused report leaves both buffers where they are" "2" \
  "$(tmux list-buffers -F '#{buffer_name}' | sed -n '/^gang-usage/p' | wc -l | tr -d ' ')"
usage_branch_discard
# The fault is the suite's to ask for, not the environment's to leak: an
# ordinary teardown that inherited the variable records as if it were unset.
usage_inherited_out="$(usage_branch_drop usage-inherited \
  -u GANG_TEST_COLLARS GANG_TEST_USAGE_REPORT_FAULT=missing)" \
  || fail "drop succeeds with an inherited report fault" "status $?"
excludes "an inherited report fault does not damage an ordinary teardown" \
  "$usage_inherited_out" "usage record left no report"
equal "and that teardown leaves no usage buffer behind either" "" \
  "$(usage_branch_staged)"
for usage_branch_agent in usage-kept usage-noreport usage-foreign usage-inherited; do
  excludes "the drop through a broken report still ended $usage_branch_agent" \
    "$(window_names)" "$usage_branch_agent"
done

# --- --all reads every record; an unreadable line is counted -----------------
# Five foreign lines: another team's record; a record from an earlier team of
# this name, ended after this one began; a line that is not JSON; a JSON line
# of the right version whose duration is text and whose status is an escape
# sequence; and a valid record carrying a C1 control in its note. The last two
# are somebody else's writes, and neither must crash the arithmetic nor reach
# the terminal.
printf '%s\n' '{"v":1,"agent":"elsewhere","collar":"codex","team":"another-team","ended_at":1,"ended_by":"down","usage_status":"unmatched","model":"gpt-5.6"}' \
  "{\"v\":1,\"agent\":\"reborn\",\"collar\":\"bash\",\"team\":\"$GANG_SESSION\",\"team_created\":1,\"ended_at\":$((usage_team_created + 1)),\"ended_by\":\"drop\",\"usage_status\":\"unmatched\"}" \
  'this line is not a record' \
  "{\"v\":1,\"agent\":\"forged\",\"team\":\"$GANG_SESSION\",\"team_created\":$usage_team_created,\"ended_at\":$((usage_team_created + 1)),\"ended_by\":\"drop\",\"duration_s\":\"not-a-duration\",\"usage_status\":\"\\u001b[2J\"}" \
  '{"v":1,"agent":"c1-control","team":"another-team","ended_at":1,"ended_by":"down","usage_status":"unmatched","usage_note":"\u009b2J"}' \
  >> "$usage_events"
usage_scoped_out="$(PATH="$usage_present" XDG_DATA_HOME="$usage_data" "$GANG" usage 2>&1)" \
  || fail "gang usage succeeds beside foreign record lines" "status $?: [$usage_scoped_out]"
excludes "a record from another team stays out of the team view" \
  "$usage_scoped_out" "elsewhere"
excludes "a record from an earlier team of the same name stays out of the team view" \
  "$usage_scoped_out" "reborn"
usage_all_out="$(PATH="$usage_present" XDG_DATA_HOME="$usage_data" "$GANG" usage --all 2>&1)" \
  || fail "gang usage --all succeeds" "status $?: [$usage_all_out]"
contains "--all shows a record from another team" \
  "$usage_all_out" "elsewhere"
contains "--all shows the earlier team's record under the same name" \
  "$usage_all_out" "reborn"
contains "--all names the Codex note when a codex collar is shown" \
  "$usage_all_out" "ccusage documents as experimental"
contains "unreadable record lines are counted, not skipped silently" \
  "$usage_all_out" "2 line(s) in $usage_events could not be read"
excludes "a forged record's status never reaches the terminal" \
  "$usage_all_out" "forged"
excludes "a forged record's escape byte never reaches the terminal" \
  "$usage_all_out" $'\033'
excludes "a foreign record's C1 control never reaches the terminal" \
  "$usage_all_out" $'\302\233'

# --- down records every window from one ccusage read -----------------------
usage_down_session="usage-down-$$"
GANG_SESSION="$usage_down_session" "$GANG" hitch downer-one -c bash -d /tmp >/dev/null
GANG_SESSION="$usage_down_session" "$GANG" hitch downer-two -c bash -d /tmp >/dev/null
tmux set-option -w -t "$(window_id_in "$usage_down_session" downer-one)" @gl_session_id "$usage_claude_id"
: > "$usage_argv"
usage_down_out="$(USAGE_FIXTURE_ARGV="$usage_argv" PATH="$usage_present" GANG_SESSION="$usage_down_session" XDG_DATA_HOME="$usage_data" "$GANG" down "$usage_down_session" 2>&1)" \
  || fail "down succeeds with ccusage present" "status $?: [$usage_down_out]"
equal "down reads ccusage once for the whole team" \
  "1" "$(grep -c 'session --json' "$usage_argv")"
contains "down records the matched window" "$usage_down_out" "usage: downer-one matched"
contains "down records the unstamped window" "$usage_down_out" "usage: downer-two unstamped"
equal "down appends one record per window, marked as ended by down" \
  "downer-one down downer-two down" \
  "$(python3 -c 'import json,sys
rows=[json.loads(l) for l in open(sys.argv[1]) if l.strip() and l.startswith("{") and "downer" in l]
print(" ".join(f"{r["agent"]} {r["ended_by"]}" for r in rows))' "$usage_events")"

usage_nolive_out="$(GANG_SESSION="$usage_down_session" XDG_DATA_HOME="$usage_data" "$GANG" usage 2>&1)" \
  || fail "gang usage succeeds with no live team" "status $?: [$usage_nolive_out]"
contains "with no live team gang usage says so" \
  "$usage_nolive_out" "no live team (session '$usage_down_session' not running)"
contains "with no live team gang usage still prints that team's records" \
  "$usage_nolive_out" "downer-one"
