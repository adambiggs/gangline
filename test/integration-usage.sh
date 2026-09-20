# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Usage: the launch record hitch registers and the ccusage join behind gang usage.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# ccusage is stood in for by executables on a private PATH prefix: one that
# prints the documented session shape, one that exits with an error, and two
# that print shapes gang must refuse. The real tool is never run here.

usage_bin="$RUN_ROOT/usage-bin"
usage_data="$RUN_ROOT/usage-data"
mkdir -p "$usage_bin/present" "$usage_bin/filtered-gap" "$usage_bin/failing" \
  "$usage_bin/garbage" "$usage_bin/unjson"
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
],"daily":[
 {"agent":"all","period":"2026-09-02","inputTokens":1015,"outputTokens":1026,"cacheReadTokens":1307,"cacheCreationTokens":1040,"totalTokens":4388,"modelsUsed":["claude-opus-5","gpt-5.6"],"modelBreakdowns":[
  {"modelName":"claude-opus-5","inputTokens":1010,"outputTokens":1020,"cacheReadTokens":1300,"cacheCreationTokens":1040},
  {"modelName":"gpt-5.6","inputTokens":5,"outputTokens":6,"cacheReadTokens":7,"cacheCreationTokens":0}
 ],"metadata":{"agents":["claude","codex"]}}
],"totals":{"inputTokens":15,"outputTokens":26,"cacheReadTokens":307,"cacheCreationTokens":40,"totalTokens":388}}
JSON
SH
cat > "$usage_bin/filtered-gap/ccusage" <<'SH'
#!/usr/bin/env bash
# ccusage 20 omits Claude sessions from the filtered aggregate, while its
# exact-session view still returns their in-span entries.
printf '%s\n' "$*" >> "${USAGE_FIXTURE_ARGV:-/dev/null}"
case " $* " in
  *" --id 11111111-aaaa-4bbb-8ccc-000000000001 "*)
    cat <<'JSON'
{"sessionId":"11111111-aaaa-4bbb-8ccc-000000000001","entries":[
 {"model":"claude-opus-5","inputTokens":4,"outputTokens":8,"cacheReadTokens":100,"cacheCreationTokens":10,"timestamp":"2026-09-02T09:00:00.000Z"},
 {"model":"claude-opus-5","inputTokens":6,"outputTokens":12,"cacheReadTokens":200,"cacheCreationTokens":30,"timestamp":"2026-09-02T10:00:00.000Z"}
]}
JSON
    ;;
  *" --id "*)
    printf '%s\n' 'null'
    ;;
  *" --since "*)
    cat <<'JSON'
{"session":[
 {"agent":"codex","period":"2026/09/02/rollout-2026-09-02T00-00-00-22222222-bbbb-4ccc-8ddd-000000000002","inputTokens":5,"outputTokens":6,"cacheReadTokens":7,"cacheCreationTokens":0,"modelBreakdowns":[{"modelName":"gpt-5.6","inputTokens":5,"outputTokens":6,"cacheReadTokens":7,"cacheCreationTokens":0}],"metadata":{"lastActivity":"2026-09-02T10:00:00.000Z"}}
]}
JSON
    ;;
  *)
    cat <<'JSON'
{"session":[
 {"agent":"claude","period":"11111111-aaaa-4bbb-8ccc-000000000001","inputTokens":10,"outputTokens":20,"cacheReadTokens":300,"cacheCreationTokens":40,"modelBreakdowns":[{"modelName":"claude-opus-5","inputTokens":10,"outputTokens":20,"cacheReadTokens":300,"cacheCreationTokens":40}],"metadata":{"lastActivity":"2026-09-03T10:00:00.000Z"}},
 {"agent":"codex","period":"2026/09/02/rollout-2026-09-02T00-00-00-22222222-bbbb-4ccc-8ddd-000000000002","inputTokens":5,"outputTokens":6,"cacheReadTokens":7,"cacheCreationTokens":0,"modelBreakdowns":[{"modelName":"gpt-5.6","inputTokens":5,"outputTokens":6,"cacheReadTokens":7,"cacheCreationTokens":0}],"metadata":{"lastActivity":"2026-09-02T10:00:00.000Z"}},
 {"agent":"codex","period":"2026/09/02/rollout-2026-09-02T00-00-00-33333333-cccc-4ddd-8eee-000000000003","inputTokens":9,"outputTokens":9,"cacheReadTokens":9,"cacheCreationTokens":0,"modelBreakdowns":[{"modelName":"gpt-5.6","inputTokens":9,"outputTokens":9,"cacheReadTokens":9,"cacheCreationTokens":0}],"metadata":{"lastActivity":"2026-09-02T10:00:00.000Z"}}
]}
JSON
    ;;
esac
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
usage_filtered_gap="$usage_bin/filtered-gap:$PATH"
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
usage_daily_argv="$RUN_ROOT/usage-daily-argv"
usage_daily_out="$(USAGE_FIXTURE_ARGV="$usage_daily_argv" PATH="$usage_present" \
  XDG_DATA_HOME="$usage_data" "$GANG" usage --daily 2026-09-02 2>&1)" \
  || fail "gang usage daily succeeds with ccusage present" "status $?: [$usage_daily_out]"
equal "daily usage reads the filtered aggregate and discovers unmatched sessions" \
  "session --json --no-cost --offline --since 2026-09-02 --until 2026-09-02
session --json --no-cost --offline" \
  "$(<"$usage_daily_argv")"
equal "daily usage splits each Gangline agent and model into quota-relevant classes" \
  "usage-alpha claude-opus-5 10 20 300 40 95.4%" \
  "$(printf '%s\n' "$usage_daily_out" | awk '$1 == "usage-alpha" { print $1, $2, $3, $4, $5, $6, $7 }')"
equal "daily usage keeps Codex output inclusive of its reported thinking" \
  "usage-beta gpt-5.6 5 6 7 0 4.6%" \
  "$(printf '%s\n' "$usage_daily_out" | awk '$1 == "usage-beta" { print $1, $2, $3, $4, $5, $6, $7 }')"
contains "daily usage labels the output column as including thinking" \
  "$usage_daily_out" "output(+thinking)"
contains "agent attribution says its share is local rather than account-wide" \
  "$usage_daily_out" "local agent attribution; not account-quota share"
usage_gap_argv="$RUN_ROOT/usage-filtered-gap-argv"
# Alpha continued after the selected day. Its registration still overlaps the
# day, so later lastActivity must not turn its exact in-day entries into zero.
tmux set-option -w -t "$usage_alpha_id" @gl_hitched_at \
  "$(date -d '2026-09-02 08:00:00' +%s)"
usage_gap_out="$(USAGE_FIXTURE_ARGV="$usage_gap_argv" PATH="$usage_filtered_gap" \
  XDG_DATA_HOME="$usage_data" "$GANG" usage --daily 2026-09-02 2>&1)" \
  || fail "daily usage recovers a session omitted from ccusage's filtered aggregate" \
       "status $?: [$usage_gap_out]"
tmux set-option -w -t "$usage_alpha_id" @gl_hitched_at "$usage_alpha_hitched"
equal "a filtered report first reads the aggregate, then discovers and reads the omitted id" \
  "session --json --no-cost --offline
session --json --no-cost --offline --id $usage_claude_id --since 2026-09-02 --until 2026-09-02
session --json --no-cost --offline --id $usage_stray_id --since 2026-09-02 --until 2026-09-02
session --json --no-cost --offline --since 2026-09-02 --until 2026-09-02" \
  "$(sort "$usage_gap_argv")"
equal "the exact-session fallback restores a continued Claude session's in-day classes" \
  "usage-alpha claude-opus-5 10 20 300 40 95.4%" \
  "$(printf '%s\n' "$usage_gap_out" | awk '$1 == "usage-alpha" { print $1, $2, $3, $4, $5, $6, $7 }')"
excludes "filtered zero-usage sessions do not become uncovered noise" \
  "$usage_gap_out" "usage-delta: unmatched"
# Focused usage runs retain the substrate prerequisite's unstamped windows,
# while full runs may have dropped them before reaching this part. The coverage
# property is that this fixture's own unstamped agent remains in that bounded
# group, not that no prerequisite window shares it.
usage_gap_unstamped="$(printf '%s\n' "$usage_gap_out" \
  | awk '/^  unstamped \(/ { print; exit }')"
contains "filtered coverage keeps unstamped agents visible in a bounded summary" \
  "$usage_gap_unstamped" "usage-gamma"
usage_since_argv="$RUN_ROOT/usage-since-argv"
usage_since_out="$(USAGE_FIXTURE_ARGV="$usage_since_argv" PATH="$usage_present" \
  XDG_DATA_HOME="$usage_data" "$GANG" usage --since 2026-09-02 2>&1)" \
  || fail "gang usage since succeeds with ccusage present" "status $?: [$usage_since_out]"
equal "since usage reads the filtered aggregate and discovers unmatched sessions" \
  "session --json --no-cost --offline --since 2026-09-02
session --json --no-cost --offline" \
  "$(<"$usage_since_argv")"
contains "since usage keeps the same per-agent and per-model split" \
  "$usage_since_out" "usage-alpha"
usage_gamma_id="$(window_id usage-gamma)"
tmux set-option -w -t "$usage_gamma_id" @gl_session_id "$usage_claude_id"
usage_ambiguous_out="$(PATH="$usage_present" XDG_DATA_HOME="$usage_data" \
  "$GANG" usage --daily 2026-09-02 2>&1)" \
  || fail "daily usage survives one session registered to two agents" \
       "status $?: [$usage_ambiguous_out]"
tmux set-option -w -t "$usage_gamma_id" @gl_session_id ""
contains "daily attribution names a session registered to two agents as ambiguous" \
  "$usage_ambiguous_out" "ambiguous local attribution: $usage_claude_id"
equal "ambiguous sessions are excluded rather than assigned to either agent" "" \
  "$(printf '%s\n' "$usage_ambiguous_out" | awk '$1 == "usage-alpha" || $1 == "usage-gamma"')"
equal "the remaining attributed sessions retain the whole local-share denominator" \
  "usage-beta gpt-5.6 5 6 7 0 100.0%" \
  "$(printf '%s\n' "$usage_ambiguous_out" | awk '$1 == "usage-beta" { print $1, $2, $3, $4, $5, $6, $7 }')"
usage_arity_out="$(XDG_DATA_HOME="$usage_data" "$GANG" usage extra 2>&1)" \
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
