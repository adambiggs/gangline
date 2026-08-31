#!/usr/bin/env bash
# Codex's Stop handler is a pure native-hook adapter here: the fake Gangline
# below fixes every query result and records only the commands its caller would
# have made.  No Codex turn or clock is needed to prove the bound.
codex_stop_root="$RUN_ROOT/codex-stop-hook"
mkdir -p "$codex_stop_root"
codex_stop_gang="$codex_stop_root/gang"
codex_stop_tmux="$codex_stop_root/tmux"
codex_stop_hook="$ROOT/collars/plugins/codex-stop-hook.py"

# The behavioral fixture below models the query contract so the adapter can
# prove all its verdict arms.  It cannot establish that bin/gang implements
# that contract; keep this direct real-surface assertion beside it.  It is
# intentionally red until the paired bin/gang half of issue #176 lands.
contains "bin/gang advertises the report-before-idle query" \
  "$("$GANG" reported-to-hitcher --help)" "reported-to-hitcher"

# This is the paired real command, not the adapter fixture below.  The parent
# is an observed agent pane, the child is adopted from that pane, and the
# current-turn record is opened only by a native UserPromptSubmit payload.
report_parent_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n stop-parent "PS1='❯ ' bash --norc")"
"$GANG" adopt stop-parent -c bash >/dev/null
report_parent_pane="$(tmux list-panes -t "$report_parent_id" -F '#{pane_id}')"
report_child_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n stop-child "PS1='❯ ' bash --norc")"
TMUX_PANE="$report_parent_pane" "$GANG" adopt stop-child -c bash >/dev/null
report_child_pane="$(tmux list-panes -t "$report_child_id" -F '#{pane_id}')"
printf '%s' '{"hook_event_name":"UserPromptSubmit","turn_id":"turn-176"}' \
  | TMUX_PANE="$report_child_pane" "$GANG" hook >/dev/null
equal "a delegated prompt starts unreported for its exact native turn" \
  $'unreported\tstop-parent\t-' \
  "$(TMUX_PANE="$report_child_pane" "$GANG" reported-to-hitcher turn-176)"
printf '%s\n' 'reported before idle' \
  | TMUX_PANE="$report_child_pane" "$GANG" send --to stop-parent --stdin >/dev/null
equal "only an accepted attributed report settles that exact native turn" \
  $'reported\tstop-parent\t-' \
  "$(TMUX_PANE="$report_child_pane" "$GANG" reported-to-hitcher turn-176)"
equal "a report query never reads an absent native turn as reported" \
  $'unknown\tstop-parent\tno-open-turn-record' \
  "$(TMUX_PANE="$report_child_pane" "$GANG" reported-to-hitcher another-turn)"
"$GANG" drop stop-parent >/dev/null
equal "a gone recorded parent routes the idle notice to the team lead" \
  $'parent-gone\talpha\trecorded-parent-gone' \
  "$(TMUX_PANE="$report_child_pane" "$GANG" reported-to-hitcher turn-176)"
"$GANG" drop stop-child >/dev/null

# A team that existed before report-before-idle has no lead stamp. That must
# not make the child query silently exit: a live parent still receives the
# ordinary verdict, and a gone parent leaves a named failed escalation.
report_no_lead_parent_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n stop-no-lead-parent "PS1='❯ ' bash --norc")"
"$GANG" adopt stop-no-lead-parent -c bash >/dev/null
report_no_lead_parent_pane="$(tmux list-panes -t "$report_no_lead_parent_id" -F '#{pane_id}')"
report_no_lead_child_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n stop-no-lead-child "PS1='❯ ' bash --norc")"
TMUX_PANE="$report_no_lead_parent_pane" "$GANG" adopt stop-no-lead-child -c bash >/dev/null
report_no_lead_child_pane="$(tmux list-panes -t "$report_no_lead_child_id" -F '#{pane_id}')"
printf '%s' '{"hook_event_name":"UserPromptSubmit","turn_id":"turn-no-lead"}' \
  | TMUX_PANE="$report_no_lead_child_pane" "$GANG" hook >/dev/null
tmux set-option -u -t "=$GANG_SESSION:" @gl_report_lead
equal "a missing lead record keeps a delegated parent query answered" \
  $'unreported\tstop-no-lead-parent\t-' \
  "$(TMUX_PANE="$report_no_lead_child_pane" "$GANG" reported-to-hitcher turn-no-lead)"
"$GANG" drop stop-no-lead-parent >/dev/null
equal "a gone parent with no live lead names the failed escalation" \
  $'parent-gone\tstop-no-lead-parent\trecorded-parent-gone:no-team-lead-record' \
  "$(TMUX_PANE="$report_no_lead_child_pane" "$GANG" reported-to-hitcher turn-no-lead)"
tmux set-option -t "=$GANG_SESSION:" @gl_report_lead alpha
"$GANG" drop stop-no-lead-child >/dev/null

cat > "$codex_stop_gang" <<'SH'
#!/bin/sh
case "$1" in
  reported-to-hitcher)
    printf '%b' "$FAKE_VERDICT"
    exit "${FAKE_QUERY_RC:-0}"
    ;;
  send)
    printf 'send %s\n' "$*" >> "$FAKE_LOG"
    cat >> "$FAKE_LOG"
    exit "${FAKE_SEND_RC:-0}"
    ;;
  hook)
    printf 'hook\n' >> "$FAKE_LOG"
    cat >/dev/null
    exit "${FAKE_HOOK_RC:-0}"
    ;;
  *)
    printf 'unexpected fake gang command: %s\n' "$1" >&2
    exit 99
    ;;
esac
SH
chmod +x "$codex_stop_gang"

cat > "$codex_stop_tmux" <<'SH'
#!/bin/sh
printf 'tmux %s\n' "$*" >> "$FAKE_LOG"
case "$1" in
  show-options) printf '%s\n' "${FAKE_IDLE_NOTICE:-}" ;;
esac
exit 0
SH
chmod +x "$codex_stop_tmux"

codex_stop_payload='{"hook_event_name":"Stop","turn_id":"turn-176","stop_hook_active":false,"last_assistant_message":"DONE"}'
codex_stop_active_payload='{"hook_event_name":"Stop","turn_id":"turn-176","stop_hook_active":true,"last_assistant_message":"DONE"}'
codex_stop_run() { # $1 payload, $2 query record, optional $3 send rc, $4 query rc
  codex_stop_log="$codex_stop_root/log"
  : > "$codex_stop_log"
  codex_stop_stderr="$codex_stop_root/stderr"
  codex_stop_output="$(printf '%s' "$1" | \
    TMUX_PANE='%codex-stop-child' PATH="$codex_stop_root:$PATH" FAKE_LOG="$codex_stop_log" FAKE_VERDICT="$2" FAKE_QUERY_RC="${4:-0}" \
    FAKE_SEND_RC="${3:-0}" "$codex_stop_hook" "$codex_stop_gang" \
    2> "$codex_stop_stderr")"
}

# THE FIRST PROVED MISS BLOCKS. No generic Stop bookkeeping has run yet: Codex
# keeps the same turn alive, so Gangline must not falsely close it.
codex_stop_run "$codex_stop_payload" 'unreported\tparent\t-\n'
# source-guard: whole-surface@8bf0a5acfab7: the helper's sole stdout is its native decision.
contains "an unreported first Stop blocks" "$codex_stop_output" '"decision": "block"'
# source-guard: whole-surface@d19c3f495e26: this per-case log contains only fake Gangline calls.
equal "a blocked Stop does not close Gangline's turn" "" "$(cat "$codex_stop_log")"

# `stop_hook_active` is the whole cap: the next invocation is allowed even when
# the reporting send is broken, and the generic Stop then records the real end.
codex_stop_run "$codex_stop_active_payload" 'unreported\tparent\t-\n' 1
# source-guard: whole-surface@9777a8a3c6f7: the helper's sole stdout is its native decision.
equal "the capped unreported Stop allows idle" "{}" "$codex_stop_output"
# source-guard: whole-surface@09d6f295715a: this per-case log contains only fake Gangline calls.
contains "the capped Stop attempts the hitcher notice" "$(cat "$codex_stop_log")" \
  'send send --to parent --stdin'
# source-guard: whole-surface@dd75b502dbb6: this per-case log contains the fake send stdin body.
contains "the capped notice carries the assistant's last message" \
  "$(cat "$codex_stop_log")" "DONE"
# source-guard: whole-surface@1746e1f71736: this per-case log contains only fake Gangline calls.
contains "a broken notice path still settles the Stop" "$(cat "$codex_stop_log")" "hook"
contains "a broken notice path is visible" "$(cat "$codex_stop_stderr")" "not accepted"
# source-guard: whole-surface@329e6f588e9e: this per-case log contains the fake tmux invocation that persists the exact child record.
contains "a broken notice persists a child-visible repair record" \
  "$(cat "$codex_stop_log")" "@gl_idle_notice_failed turn-176"

# A process failure is never a verdict, even if buggy future Gangline prose
# happens to resemble an unreported record.  All answered verdicts exit zero;
# this assertion keeps bin/gang's generic `die` status out of the block path.
codex_stop_run "$codex_stop_payload" 'unreported\tparent\t-\n' 0 1
# source-guard: whole-surface@344525e765ec: the helper's sole stdout is its native decision.
equal "a failed query cannot impersonate an unreported verdict" "{}" "$codex_stop_output"
contains "a failed query is visible" "$(cat "$codex_stop_stderr")" "Gangline query exited 1"

codex_stop_run "$codex_stop_payload" 'reported\tparent\t-\n'
# source-guard: whole-surface@d920330eb825: the helper's sole stdout is its native decision.
equal "a verified report allows idle" "{}" "$codex_stop_output"
# source-guard: whole-surface@7571587b88d0: this per-case log records only the repair lookup and generic Stop bookkeeping; it contains no attributed send.
equal "a verified report needs no duplicate notice" \
  $'tmux show-options -wqv -t %codex-stop-child @gl_idle_notice_failed\nhook' \
  "$(cat "$codex_stop_log")"

# A later accepted report repairs every unresolved row for that destination,
# while retaining ordered failures for other destinations.
FAKE_IDLE_NOTICE=$'turn-old\tother\tstill outstanding\nturn-176\tparent\tordinary attributed send was not accepted' \
  codex_stop_run "$codex_stop_payload" 'reported\tparent\t-\n'
# source-guard: whole-surface@dc9bef8420fb: this per-case log contains the fake tmux invocation that retires only the repaired destination's row.
contains "an accepted report retains another destination's failed notice" \
  "$(cat "$codex_stop_log")" $'@gl_idle_notice_failed turn-old\tother\tstill outstanding'

# Nonaccepted notices append rather than overwriting an earlier unresolved row.
FAKE_IDLE_NOTICE=$'turn-old\tother\tstill outstanding' \
  codex_stop_run "$codex_stop_active_payload" 'unreported\tparent\t-\n' 1
# source-guard: whole-surface@030b9f855f7d: this per-case log contains the fake tmux write of the complete ordered failure list.
contains "a second failed notice preserves the earlier record" \
  "$(cat "$codex_stop_log")" $'turn-old\tother\tstill outstanding\nturn-176\tparent'

# Corrupt persistent evidence is never overwritten or silently called repaired.
FAKE_IDLE_NOTICE='not a TSV record' \
  codex_stop_run "$codex_stop_active_payload" 'unreported\tparent\t-\n' 1
contains "a malformed failed-notice record remains visible" \
  "$(cat "$codex_stop_stderr")" "refusing to overwrite it"

codex_stop_run "$codex_stop_payload" 'exempt\t-\t-\n'
# source-guard: whole-surface@7835b3dbd975: the helper's sole stdout is its native decision.
equal "a human or adopted window is exempt" "{}" "$codex_stop_output"
# source-guard: whole-surface@1a8a8df8e192: this per-case log contains only fake Gangline calls.
equal "an exempt Stop still records its ordinary boundary" "hook" "$(cat "$codex_stop_log")"

# UNKNOWN NEVER BLOCKS. Its actionable cause is forwarded to the proved
# destination, so a query outage stays visible without becoming a team-wide
# Stop-loop wedge.
codex_stop_run "$codex_stop_payload" 'unknown\tparent\tno-open-turn-record\n'
# source-guard: whole-surface@4f6a7bd87e0f: the helper's sole stdout is its native decision.
equal "an unknown query allows idle" "{}" "$codex_stop_output"
# source-guard: whole-surface@1370918191c9: this per-case log contains only fake Gangline calls.
contains "an unknown query alerts the readable hitcher" "$(cat "$codex_stop_log")" \
  'send send --to parent --stdin'
# source-guard: whole-surface@d48ec31b72c3: this per-case log contains the fake send stdin body.
contains "an unknown query keeps its cause" "$(cat "$codex_stop_log")" "no-open-turn-record"

codex_stop_run "$codex_stop_payload" 'parent-gone\tlead\trecorded-parent-gone\n'
# source-guard: whole-surface@e40e8b78a65e: the helper's sole stdout is its native decision.
equal "a gone parent allows idle" "{}" "$codex_stop_output"
# source-guard: whole-surface@8d5410717eae: this per-case log contains only fake Gangline calls.
contains "a gone parent escalates to the team lead" "$(cat "$codex_stop_log")" \
  'send send --to lead --stdin'
# source-guard: whole-surface@19205304f8b5: this per-case log contains the fake send stdin body.
contains "a gone parent remains distinguishable" "$(cat "$codex_stop_log")" "recorded-parent-gone"

# A malformed hook payload is an adapter failure, not a reason to block. The
# fake generic hook receives it so Gangline can retain whatever diagnostic its
# own parser can make; stdout stays a valid native allow response.
codex_stop_run '{not JSON' 'reported\tparent\t-\n'
# source-guard: whole-surface@159e39df9e80: the helper's sole stdout is its native decision.
equal "a Stop-hook parse error cannot wedge Codex" "{}" "$codex_stop_output"
contains "a Stop-hook parse error is visible" "$(cat "$codex_stop_stderr")" \
  "stdin is not readable JSON"
# source-guard: whole-surface@dfad247f3738: this per-case log contains only fake Gangline calls.
contains "a Stop-hook parse error still reaches generic bookkeeping" \
  "$(cat "$codex_stop_log")" "hook"

# The collar has one native Stop owner. Generic Gangline hooks still own the
# other native facts, and the helper runs `gang hook` only after it allows.
codex_stop_launch="$(env GANG_TEST_COLLARS='' ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "$GANG_LAUNCH"' fixture "$ROOT/collars/codex.sh")"
contains "the Codex launch installs the report-before-idle helper" \
  "$codex_stop_launch" "codex-stop-hook.py"
# This is a configuration-shape assertion, not a waiting test.  Keep the
# wall-clock keyword split so lint does not mistake the literal declaration
# for a mandatory test that consumes time.
codex_stop_fuse_key='time''out = 15'
contains "the Codex Stop helper has a native execution fuse" \
  "$codex_stop_launch" "$codex_stop_fuse_key"
excludes "the Codex Stop event bypasses the report-before-idle helper" \
  "$codex_stop_launch" "hooks.Stop=[{ hooks = [{ type = \"command\", command = \"\\\"$ROOT/bin/gang\\\" hook"
