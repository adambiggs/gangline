#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Fast substrate contract checks. Every assertion reads state that already exists.
# Real-harness behavior belongs in a separately named disposable team.
set -euo pipefail

unset TMUX TMUX_PANE

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
GANG="$ROOT/bin/gang"
RUN_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-test.XXXXXX")"
TMUX_SOCKET="$RUN_ROOT/tmux-$(id -u)/default"

# The Bash fixture establishes every transition synchronously. Production waits
# are inputs here, not evidence, so replace their clock with an immediate one.
mkdir -p "$RUN_ROOT/bin"
cat > "$RUN_ROOT/bin/sleep" <<'SH'
#!/bin/sh
exit 0
SH
chmod +x "$RUN_ROOT/bin/sleep"
PATH="$RUN_ROOT/bin:$PATH"
export PATH

cleanup() {
  tmux -S "$TMUX_SOCKET" kill-server 2>/dev/null || true
  rm -rf -- "$RUN_ROOT"
}
trap cleanup EXIT HUP INT TERM

export TMUX_TMPDIR="$RUN_ROOT"
export GANG_SESSION="gangtest-$$"
export GANG_TEST_PROFILES=1
export GANG_CHURN_WAIT=0
export GANG_LOCK_DIR="$RUN_ROOT/locks"

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

window_id() { # $1 exact window name
  tmux list-windows -t "=$GANG_SESSION" -F '#{window_id} #{window_name}' |
    awk -v wanted="$1" '$2 == wanted { print $1; exit }'
}

pane() { tmux capture-pane -pJ -t "$(window_id "$1")"; }

# Start the private tmux server with the wrong session in its global environment.
# Hitched harnesses must receive the exact team identity in their launch command,
# rather than inheriting whichever session started the shared server.
GANG_SESSION=stale-session tmux new-session -d -s environment-seed

# Public profile surface: the bash fixture remains test-only.
profiles="$(GANG_TEST_PROFILES='' "$GANG" profiles | tr '\n' ' ')"
equal "the public profile list is the supported harness set" \
  "claude-code codex opencode pi " "$profiles"

# Codex receives the native hooks with live consumers on fresh and resumed launches. The
# command must remain one shell word even when Gangline is installed under a path
# containing spaces.
CODEX_STUB="$RUN_ROOT/codex-stub"
mkdir -p "$CODEX_STUB/bin"
cat > "$CODEX_STUB/bin/codex" <<'SH'
#!/bin/sh
while [ "$#" -gt 0 ]; do
  case "$1" in
    -c) printf '%s\n' "$2" >> "$CODEX_OPTIONS"; shift 2 ;;
    *) shift ;;
  esac
done
SH
chmod +x "$CODEX_STUB/bin/codex"

codex_launch() { # $1 fake install root, $2 GANG_LAUNCH|GANG_RESUME_LAUNCH
  local fake_root="$1" launch_name="$2" profile_file="$ROOT/profiles/codex.sh"
  GANG_TEST_PROFILES='' ROOT="$fake_root" bash -c \
    'set -euo pipefail; . "$1"; printf "%s" "${!2}"' \
    fixture "$profile_file" "$launch_name"
}

codex_hook_command() { # $1 launch line, $2 captured -c options
  local launch="$1" options="$2"
  : > "$options"
  CODEX_OPTIONS="$options" sh -c 'PATH="$1"; export PATH; eval "$2"' sh \
    "$CODEX_STUB/bin:$PATH" "$launch" </dev/null
  python3 - "$options" <<'PY'
import json
import re
import sys

events = {
    "UserPromptSubmit", "PostToolUse", "Stop", "PermissionRequest",
}
shape = re.compile(
    r'hooks\.([A-Za-z]+)=\[\{ hooks = \[\{ type = "command", '
    r'command = "((?:\\.|[^"\\])*)" \}\] \}\]'
)
seen = set()
commands = set()
for option in open(sys.argv[1], encoding="utf-8"):
    if not option.startswith("hooks."):
        continue
    match = shape.fullmatch(option.rstrip("\n"))
    if match is None:
        raise SystemExit(1)
    seen.add(match.group(1))
    commands.add(json.loads('"' + match.group(2) + '"'))
if seen != events or len(commands) != 1:
    raise SystemExit(1)
print(commands.pop())
PY
}

hook_receipts=""
for install_name in plain 'has space'; do
  install_root="$RUN_ROOT/$install_name"
  mkdir -p "$install_root/bin"
  cat > "$install_root/bin/gang" <<'SH'
#!/bin/sh
printf '%s\n' "$@" > "$0.args"
SH
  chmod +x "$install_root/bin/gang"
  for launch_var in GANG_LAUNCH GANG_RESUME_LAUNCH; do
    options="$RUN_ROOT/${install_name}-${launch_var}.options"
    command="$(codex_hook_command "$(codex_launch "$install_root" "$launch_var")" "$options")"
    "$install_root/bin/gang" true >/dev/null 2>&1 || true
    : > "$install_root/bin/gang.args"
    sh -c "$command" </dev/null
    args="$(tr '\n' ' ' < "$install_root/bin/gang.args")"
    hook_receipts="$hook_receipts${hook_receipts:+ | }$install_name/$launch_var=$args"
  done
done
equal "Codex native hooks survive fresh and resumed launch paths" \
  "plain/GANG_LAUNCH=hook  | plain/GANG_RESUME_LAUNCH=hook  | has space/GANG_LAUNCH=hook  | has space/GANG_RESUME_LAUNCH=hook " \
  "$hook_receipts"

claude_profile="$ROOT/profiles/claude-code.sh"
claude_off="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "$GANG_LAUNCH"' fixture "$claude_profile")"
excludes "disabled lights do not paint Claude context output" \
  "$claude_off" 'statusLine'
claude_on="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=100000,200000 bash -c \
  '. "$1"; printf "%s" "$GANG_LAUNCH"' fixture "$claude_profile")"
contains "enabled Claude lights wire their context source" \
  "$claude_on" 'statusLine'
claude_midturn="$(ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "${GANG_MIDTURN_INPUT:-}"' fixture "$claude_profile")"
equal "Claude delivery waits for an idle composer" "" "$claude_midturn"
claude_queued="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "${GANG_QUEUED_REGEX:-}"' fixture "$claude_profile")"
if [ -n "$claude_queued" ]; then
  pass "the Claude profile declares its parked-queue evidence"
else
  fail "the Claude profile declares its parked-queue evidence" \
    "GANG_QUEUED_REGEX is empty"
fi
if printf '%s\n' 'Press up to edit queued messages   ' | grep -Eq -- "$claude_queued"; then
  pass "the observed queue hint reads as parked input"
else
  fail "the observed queue hint reads as parked input" \
    "regex missed the observed 2.1.223 hint rendering"
fi
if printf '%s\n' 'a body quoting Press up to edit queued messages mid-line' |
  grep -Eq -- "$claude_queued"; then
  fail "a line quoting the hint is not parked-queue evidence" \
    "regex matched a quotation"
else
  pass "a line quoting the hint is not parked-queue evidence"
fi
claude_effort_opt="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "${GANG_EFFORT_OPT:-}"' fixture "$claude_profile")"
equal "the Claude profile spells effort as one joinable word" \
  "--effort=" "$claude_effort_opt"
claude_effort_cmd="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "${GANG_EFFORT_CMD:-}"' fixture "$claude_profile")"
CLAUDE_STUB="$RUN_ROOT/claude-stub"
mkdir -p "$CLAUDE_STUB/bin"
cat > "$CLAUDE_STUB/bin/claude" <<'SH'
#!/bin/sh
cat <<'HELP'
  --model <model>                       Model for the current session
  --effort <level>                      Effort level for the current session
                                        (low, medium, high, xhigh, max)
  --fallback-model <model>              Enable automatic fallback
HELP
SH
chmod +x "$CLAUDE_STUB/bin/claude"
claude_levels="$(PATH="$CLAUDE_STUB/bin:$PATH" sh -c "$claude_effort_cmd" | tr '\n' ' ')"
equal "the Claude effort vocabulary is read from the harness's own help, through the wrap" \
  "low medium high xhigh max " "$claude_levels"
cat > "$CLAUDE_STUB/bin/claude" <<'SH'
#!/bin/sh
echo '  --effort <level>                      Effort level for the current session'
SH
claude_drift="$(PATH="$CLAUDE_STUB/bin:$PATH" sh -c "$claude_effort_cmd")"
equal "a help shape the parser cannot finish yields nothing rather than a guess" \
  "" "$claude_drift"
# A prose parenthetical is not a vocabulary: only the comma-separated level
# list counts, so "(experimental)" cannot be adopted while the real list on
# the same row still is.
cat > "$CLAUDE_STUB/bin/claude" <<'SH'
#!/bin/sh
echo '  --effort <level>                      Effort selection (experimental); available values (low, medium, high)'
SH
claude_prose="$(PATH="$CLAUDE_STUB/bin:$PATH" sh -c "$claude_effort_cmd" | tr '\n' ' ')"
equal "a prose parenthetical beside the real list is never the vocabulary" \
  "low medium high " "$claude_prose"
# Two level-lists on one row is ambiguity, and ambiguity is nothing.
cat > "$CLAUDE_STUB/bin/claude" <<'SH'
#!/bin/sh
echo '  --effort <level>  Pick (low, medium) or later (high, xhigh)'
SH
claude_ambiguous="$(PATH="$CLAUDE_STUB/bin:$PATH" sh -c "$claude_effort_cmd")"
equal "two plausible lists on the option row yield nothing rather than a pick" \
  "" "$claude_ambiguous"
# The list is anchored to the --effort row's own block: a list belonging to
# the NEXT option must not be adopted when the effort row has none.
cat > "$CLAUDE_STUB/bin/claude" <<'SH'
#!/bin/sh
printf '  --effort <level>  Effort level for the current session\n  --other <x>  Choose (low, medium, high)\n'
SH
claude_neighbour="$(PATH="$CLAUDE_STUB/bin:$PATH" sh -c "$claude_effort_cmd")"
equal "a neighbouring option's list is not adopted as the effort vocabulary" \
  "" "$claude_neighbour"
# Help from a claude that FAILED is not evidence, however plausible it reads:
# the producer's exit status is honored, and its output lands in the
# could-not-determine channel.
cat > "$CLAUDE_STUB/bin/claude" <<'SH'
#!/bin/sh
printf '  --effort <level>                      Effort level for the current session\n                                        (low, medium, high, xhigh, max)\n'
exit 17
SH
claude_failed="$(PATH="$CLAUDE_STUB/bin:$PATH" sh -c "$claude_effort_cmd")"
equal "correct help from a failed producer yields nothing rather than a vocabulary" \
  "" "$claude_failed"

codex_profile="$ROOT/profiles/codex.sh"
codex_compact="$(GANG_TEST_PROFILES='' ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "$GANG_COMPACT_CMD"' fixture "$codex_profile")"
equal "the Codex profile keeps native compaction" "/compact" "$codex_compact"
codex_self_compact="$(GANG_TEST_PROFILES='' ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "$GANG_SELF_COMPACT"' fixture "$codex_profile")"
equal "the Codex profile defers self-compaction to its native Stop hook" \
  "deferred" "$codex_self_compact"
codex_effort_opt="$(GANG_TEST_PROFILES='' ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "${GANG_EFFORT_OPT:-}"' fixture "$codex_profile")"
equal "the Codex profile spells effort as one joinable config option" \
  "-c model_reasoning_effort=" "$codex_effort_opt"
codex_effort_cmd="$(GANG_TEST_PROFILES='' ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "${GANG_EFFORT_CMD:-}"' fixture "$codex_profile")"
CODEX_CATALOG_STUB="$RUN_ROOT/codex-catalog-stub"
mkdir -p "$CODEX_CATALOG_STUB/bin"
cat > "$CODEX_CATALOG_STUB/bin/codex" <<'SH'
#!/bin/sh
cat <<'JSON'
{"models":[
  {"slug":"gpt-5.6-sol","supported_reasoning_levels":[
    {"effort":"low"},{"effort":"medium"},{"effort":"xhigh"}]},
  {"slug":"gpt-5.6-mini","supported_reasoning_levels":[
    {"effort":"low"}]}
]}
JSON
SH
chmod +x "$CODEX_CATALOG_STUB/bin/codex"
codex_levels="$(GANG_MODEL=gpt-5.6-sol PATH="$CODEX_CATALOG_STUB/bin:$PATH" \
  sh -c "$codex_effort_cmd" | tr '\n' ' ')"
equal "the Codex effort vocabulary binds the exact model hitch will launch" \
  "low medium xhigh " "$codex_levels"
codex_unbound="$(GANG_MODEL='' PATH="$CODEX_CATALOG_STUB/bin:$PATH" \
  sh -c "$codex_effort_cmd")"
equal "a model the catalog cannot bind yields nothing rather than a guess" \
  "" "$codex_unbound"
# opencode and pi refuse -e by declaring nothing: their native effort forms
# are unverified, and an unverified spelling must not reach a launch line.
for unverified_profile in opencode pi; do
  unverified_file="$ROOT/profiles/$unverified_profile.sh"
  unverified_effort="$(GANG_TEST_PROFILES='' ROOT="$ROOT" bash -c \
    '. "$1"; printf "%s" "${GANG_EFFORT_OPT:-}"' fixture "$unverified_file")"
  equal "the $unverified_profile profile declares no effort spelling until one is verified" \
    "" "$unverified_effort"
done
codex_context_fixture="$RUN_ROOT/codex-context.jsonl"
cat > "$codex_context_fixture" <<'JSONL'
{"payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":50000},"model_context_window":300000}}}
{"payload":{"type":"message","role":"assistant","content":"later non-token event"}}
{"payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":120000},"model_context_window":300000}}}
JSONL
codex_context="$(GANG_TEST_PROFILES='' ROOT="$ROOT" bash -c \
  '. "$1"; codex_context_read "$2"' fixture "$codex_profile" "$codex_context_fixture")"
equal "Codex context reads the newest native token record" \
  "120k/300k (40%)" "$codex_context"

# Real tmux substrate: lifecycle, observation, verified attributed delivery and
# exact-name addressing. Gangline's command returns only after the state checked
# below has been established.
"$GANG" hitch alpha -p bash -d /tmp >/dev/null
contains "hitch creates an observable idle agent" "$($GANG status alpha)" "idle"
contains "roster lists the hitched profile" "$($GANG roster)" "alpha"
contains "roster is an immediate snapshot" \
  "$(GANG_CHURN_WAIT=not-a-duration $GANG roster)" "alpha"

mkdir -p "$RUN_ROOT/profiles"
export GANG_PROFILES="$RUN_ROOT/profiles"
modal_observed="test-boot-modal-observed-$$"
modal_painted="test-boot-modal-painted-$$"
modal_clear="test-boot-modal-clear-$$"
modal_probe_release="test-boot-modal-probe-release-$$"
cat > "$RUN_ROOT/profiles/boot-modal.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'printf \"FIRST_RUN_MODAL\\n\"; tmux wait-for -S \"$modal_painted\"; tmux wait-for \"$modal_clear\"; PS1=\"❯ \" exec bash --norc' fixture"
GANG_OCCUPIED_REGEX='FIRST_RUN_MODAL'
_gl_modal_real="\$(declare -f profile_input)"
eval "modal_real_input \${_gl_modal_real#profile_input}"
profile_input() {
  if [ -e "$RUN_ROOT/boot-modal-blocked" ]; then
    if [ -e "$RUN_ROOT/boot-modal-seen" ]; then
      tmux wait-for -S "$modal_observed"
      tmux wait-for "$modal_probe_release"
    else
      tmux wait-for "$modal_painted"
      : > "$RUN_ROOT/boot-modal-seen"
    fi
    return 1
  fi
  modal_real_input "\$1"
}
SH
touch "$RUN_ROOT/boot-modal-blocked"
modal_output="$RUN_ROOT/boot-modal.out"
GANG_BOOT_TIMEOUT=5 "$GANG" hitch boot-modal -p boot-modal -d /tmp \
  >"$modal_output" 2>&1 &
modal_hitch_pid=$!
tmux wait-for "$modal_observed"
contains "hitch reports a first-run modal before it is cleared" \
  "$(<"$modal_output")" "answer it with 'gang attach'"
rm -f -- "$RUN_ROOT/boot-modal-blocked"
tmux wait-for -S "$modal_clear"
tmux wait-for -S "$modal_probe_release"
if wait "$modal_hitch_pid"; then
  pass "the same hitch completes after the operator clears the modal"
else
  fail "the same hitch completes after the operator clears the modal" \
    "$(<"$modal_output")"
fi
contains "the resumed hitch delivers its startup contract" \
  "$(pane boot-modal)" "You are boot-modal in Gangline"
contains "the resumed hitch reports verified startup delivery" \
  "$(<"$modal_output")" "delivered startup contract to boot-modal"
contains "the resumed hitch retracts the first-run warning" \
  "$(<"$modal_output")" "now has an input box; hitch is continuing"
"$GANG" drop boot-modal >/dev/null

late_observed="test-late-composer-observed-$$"
late_launch="test-late-composer-launch-$$"
late_probe_release="test-late-composer-probe-release-$$"
cat > "$RUN_ROOT/profiles/late-composer.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'tmux wait-for \"$late_launch\"; PS1=\"❯ \" exec bash --norc' fixture"
_gl_late_real="\$(declare -f profile_input)"
eval "late_real_input \${_gl_late_real#profile_input}"
profile_input() {
  if [ -e "$RUN_ROOT/late-composer-blocked" ]; then
    if [ -e "$RUN_ROOT/late-composer-seen" ]; then
      tmux wait-for -S "$late_observed"
      tmux wait-for "$late_probe_release"
    else
      : > "$RUN_ROOT/late-composer-seen"
    fi
    return 1
  fi
  late_real_input "\$1"
}
SH
touch "$RUN_ROOT/late-composer-blocked"
late_output="$RUN_ROOT/late-composer.out"
GANG_BOOT_TIMEOUT=5 "$GANG" hitch late-composer -p late-composer -d /tmp \
  >"$late_output" 2>&1 &
late_hitch_pid=$!
tmux wait-for "$late_observed"
excludes "a blank slow boot is not reported as a first-run modal" \
  "$(<"$late_output")" "answer it with 'gang attach'"
rm -f -- "$RUN_ROOT/late-composer-blocked"
tmux wait-for -S "$late_launch"
tmux wait-for -S "$late_probe_release"
if wait "$late_hitch_pid"; then
  pass "a delayed composer completes its original hitch"
else
  fail "a delayed composer completes its original hitch" "$(<"$late_output")"
fi
excludes "a completed delayed composer leaves no first-run warning" \
  "$(<"$late_output")" "answer it with 'gang attach'"
"$GANG" drop late-composer >/dev/null

cat > "$RUN_ROOT/profiles/broken-observer.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_BUSY_REGEX='['
SH
"$GANG" hitch broken-observer -p broken-observer -d /tmp >/dev/null
if broken_roster="$("$GANG" roster 2>&1)"; then
  fail "roster fails when an agent row cannot be observed" "roster exited successfully"
else
  broken_roster_rc=$?
  equal "roster propagates the observation failure" "1" "$broken_roster_rc"
fi
contains "roster names the profile whose observation failed" \
  "$broken_roster" "broken-observer.sh"
"$GANG" drop broken-observer >/dev/null
contains "startup is one useful contract, not a bookkeeping turn" \
  "$(pane alpha)" "You are alpha in Gangline"
contains "startup ends instead of polling for work" \
  "$(pane alpha)" "End this turn."
excludes "startup contains no session-marker prompt" "$(pane alpha)" "Session marker"
excludes "startup does not ask for a reply to its synthetic sender" \
  "$(pane alpha)" "Reply to that sender"
equal "context lights leave no state when disabled" "|" \
  "$(tmux show-options -wqv -t "$(window_id alpha)" @gl_context_lights)|$(tmux show-options -wqv -t "$(window_id alpha)" @gl_key)"

# One optional cutoff is team state. Its two relative edges consume an explicit
# clock snapshot; no assertion waits for time to pass.
equal "a team starts without an invented cutoff" "no cutoff declared" \
  "$("$GANG" cutoff)"
if "$GANG" cutoff 90 >/dev/null 2>&1; then
  fail "a cutoff never guesses the unit of a bare number" "cutoff accepted 90"
else
  pass "a cutoff never guesses the unit of a bare number"
fi
clock_spec="$(python3 - <<'PY'
from datetime import datetime, timedelta

print((datetime.now() + timedelta(minutes=2)).strftime("%H:%M"))
PY
)"
clock_cutoff="$("$GANG" cutoff "$clock_spec")"
contains "a local clock time declares its next occurrence" "$clock_cutoff" "remaining"
declared_cutoff="$("$GANG" cutoff 1h30m)"
contains "a duration declares the team cutoff" "$declared_cutoff" "remaining"
cutoff_pair="$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_cutoff)"
if [[ "$cutoff_pair" =~ ^[0-9]+\ [0-9]+$ ]]; then
  pass "the cutoff stores one team declaration"
else
  fail "the cutoff stores one team declaration" "got [$cutoff_pair]"
fi

cutoff_now="$(date +%s)"
tmux set-option -t "=$GANG_SESSION:" @gl_cutoff "$(( cutoff_now + 40 )) $(( cutoff_now - 60 ))"
yellow_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
contains "half the declared span exposes a yellow time light" \
  "$yellow_time" "Yellow time light"
excludes "the yellow time light does not prescribe team strategy" \
  "$yellow_time" "converge"
repeat_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
equal "a time light is emitted once per declaration edge" "" "$repeat_time"
tmux set-option -t "=$GANG_SESSION:" @gl_cutoff "$(( cutoff_now + 10 )) $(( cutoff_now - 90 ))"
red_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
contains "four-fifths of the declared span exposes a red time light" \
  "$red_time" "Red time light"
excludes "the red time light does not prescribe checkpoint strategy" \
  "$red_time" "bank"
tmux set-option -t "=$GANG_SESSION:" @gl_cutoff unreadable
unavailable_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
contains "an unreadable team declaration fails visibly to its agents" \
  "$unavailable_time" "Time lights unavailable"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook >/dev/null
equal "the operator can remove the team cutoff" "cutoff cleared" \
  "$("$GANG" cutoff clear)"
equal "clearing a cutoff restores silence" "no cutoff declared" \
  "$("$GANG" cutoff)"

printf 'MARK_ALPHA' | "$GANG" send --to alpha --from tester --stdin >/dev/null
alpha_pane="$(pane alpha)"
contains "verified send reaches the intended pane" "$alpha_pane" "MARK_ALPHA"
contains "the delivered message is attributed" "$alpha_pane" "[gang:tester#"

if printf 'MARK_GHOST' | "$GANG" send --to ghost --from tester --stdin >/dev/null 2>&1; then
  fail "an unknown target is refused" "send exited successfully"
else
  pass "an unknown target is refused"
fi

# Effort at hitch: the same shape as -m with one difference the worlds pin —
# the join carries no space, because the separator belongs to the harness's
# own spelling. The refusals are separated the way the code separates them: no
# spelling at all, a spelling with no way to check a level, a checker that
# answers nothing, and a level outside the printed vocabulary.
if "$GANG" hitch effortless -p bash -d /tmp -e high >/dev/null 2>&1; then
  fail "a profile with no effort spelling refuses -e" "hitch accepted -e"
else
  pass "a profile with no effort spelling refuses -e"
fi
equal "and the refusal leaves no window behind" "" "$(window_id effortless)"

cat > "$RUN_ROOT/profiles/noverify.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_EFFORT_OPT='--effort='
SH
if noverify_out="$("$GANG" hitch noverify -p noverify -d /tmp -e low 2>&1)"; then
  fail "a profile that cannot check a level may not take one" "hitch accepted -e"
else
  pass "a profile that cannot check a level may not take one"
fi
contains "and the refusal names the missing declaration" \
  "$noverify_out" "GANG_EFFORT_CMD"
equal "a refused unverifiable effort leaves no window behind" "" \
  "$(window_id noverify)"

cat > "$RUN_ROOT/profiles/silent.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_EFFORT_OPT='--effort='
GANG_EFFORT_CMD='true'
SH
if silent_out="$("$GANG" hitch silent -p silent -d /tmp -e low 2>&1)"; then
  fail "a checker that produces no levels is refused" "hitch accepted -e"
else
  pass "a checker that produces no levels is refused"
fi
contains "as a broken declaration rather than a bad value" \
  "$silent_out" "could not determine"
equal "a refused silent checker leaves no window behind" "" \
  "$(window_id silent)"

# A checker that PRINTS a plausible vocabulary and then fails is refused too:
# output from a failed checker is not a vocabulary, and treating it as one
# would open a window on evidence the profile itself declared unreliable.
cat > "$RUN_ROOT/profiles/nonzero.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_EFFORT_OPT='--effort='
GANG_EFFORT_CMD='printf "high\n"; exit 17'
SH
if nonzero_out="$("$GANG" hitch effdied -p nonzero -d /tmp -e high 2>&1)"; then
  fail "a checker that fails after printing is refused" "hitch accepted its output"
else
  pass "a checker that fails after printing is refused"
fi
contains "and the refusal names the status, not the operator's level" \
  "$nonzero_out" "failed (status 17)"
equal "a refused failing checker leaves no window behind" "" \
  "$(window_id effdied)"

cat > "$RUN_ROOT/profiles/efforted.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_RESUME_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' resume-fixture"
GANG_EFFORT_OPT='--effort='
GANG_EFFORT_CMD="printf 'low\nmedium\nxhigh\n'"
SH
if bogus_out="$("$GANG" hitch effbad -p efforted -d /tmp -e bogus 2>&1)"; then
  fail "a level outside the vocabulary is refused" "hitch accepted bogus"
else
  pass "a level outside the vocabulary is refused"
fi
contains "naming the levels the profile takes" "$bogus_out" "low medium xhigh"
equal "a refused bad level leaves no window behind" "" "$(window_id effbad)"
# The neighbour that makes a substring test look like it works: high is not a
# level here and xhigh is, so an unanchored match would pass a level this
# harness never declared.
if "$GANG" hitch effsub -p efforted -d /tmp -e high >/dev/null 2>&1; then
  fail "a level that is only part of a declared one is refused too" \
    "hitch accepted high"
else
  pass "a level that is only part of a declared one is refused too"
fi
equal "a refused partial level leaves no window behind" "" "$(window_id effsub)"

"$GANG" hitch effok -p efforted -d /tmp -e xhigh >/dev/null
contains "the declared spelling joins the effort into the launch, with no space" \
  "$(tmux display-message -p -t "$(window_id effok)" '#{pane_start_command}')" \
  "--effort=xhigh"
# The other launch form, and POSITION is why it works rather than a second
# branch: the resume swap happens above the append, so both forms carry the
# flag by construction. Asserted anyway — construction is a reason to believe,
# not a receipt, and a flag surviving hitch and lost on the other form would
# be a renewal that quietly changed the agent.
"$GANG" hitch effres -p efforted -d /tmp --resume -e low >/dev/null
effres_line="$(tmux display-message -p -t "$(window_id effres)" '#{pane_start_command}')"
contains "the resume launch form is the one that ran" "$effres_line" "resume-fixture"
contains "and it carries the effort too" "$effres_line" "--effort=low"
"$GANG" drop effok >/dev/null
"$GANG" drop effres >/dev/null

# The real claude-code profile driven end-to-end through core: the exact -m
# binding and the joined -e must reach the launch line of a window built from
# the REAL profile's declarations — a core that stopped passing either would
# stay green against fixture profiles alone. The harness is a stub on PATH,
# reached two ways: through hitch's own environment for the vocabulary check,
# and through the tmux global environment for the pane. GANG_BOOT_TIMEOUT=0
# keeps the world clock-free: the stub never paints a claude composer, so
# hitch dies AFTER the launch facts this world asserts are already
# established in tmux, and the world reads them from the surviving window.
cat > "$CLAUDE_STUB/bin/claude" <<'SH'
#!/bin/sh
if [ "${1:-}" = "--help" ]; then
  printf '  --effort <level>                      Effort level for the current session\n                                        (low, medium, high, xhigh, max)\n'
  exit 0
fi
PS1='stub ' exec bash --norc
SH
if tmux_path="$(tmux show-environment -g PATH 2>/dev/null)"; then
  tmux_path="${tmux_path#PATH=}"
else
  tmux_path="$PATH"
fi
tmux set-environment -g PATH "$CLAUDE_STUB/bin:$tmux_path"
if PATH="$CLAUDE_STUB/bin:$PATH" GANG_BOOT_TIMEOUT=0 \
  "$GANG" hitch realmodel -p claude-code -d /tmp -m claude-opus-5 -e xhigh \
  >/dev/null 2>&1; then
  fail "a stub that never paints a composer cannot complete a hitch" \
    "hitch reported success"
else
  pass "a stub that never paints a composer cannot complete a hitch"
fi
tmux set-environment -g PATH "$tmux_path"
real_launch="$(tmux display-message -p -t "$(window_id realmodel)" '#{pane_start_command}')"
contains "the real profile's launch command is the one that ran" \
  "$real_launch" "claude --settings"
contains "the exact model binds into the real launch line" \
  "$real_launch" "--model claude-opus-5"
contains "and the joined effort rides beside it" \
  "$real_launch" "--effort=xhigh"
"$GANG" drop realmodel >/dev/null

"$GANG" hitch 1 -p bash -d /tmp >/dev/null
printf 'MARK_NUMERIC' | "$GANG" send --to 1 --from tester --stdin >/dev/null
contains "a numeric name reaches its exact window" "$(pane 1)" "MARK_NUMERIC"
excludes "numeric addressing does not fall through to another window" \
  "$(pane alpha)" "MARK_NUMERIC"

self_sent="test-self-attributed-send-$$"
printf -v self_send_command \
  'printf MARK_SELF_ATTRIBUTED | %q send --to 1 --stdin >/dev/null; tmux wait-for -S %q' \
  "$GANG" "$self_sent"
tmux send-keys -l -t "$(window_id alpha)" "$self_send_command"
tmux send-keys -t "$(window_id alpha)" Enter
tmux wait-for "$self_sent"
contains "a Gangline window derives its own sender identity" \
  "$(pane 1)" "[gang:alpha#"
contains "a self-attributed send reaches the intended peer" \
  "$(pane 1)" "MARK_SELF_ATTRIBUTED"

claimed_sent="test-claimed-inside-send-$$"
printf -v claimed_send_command \
  'printf MARK_FALSE_CLAIM | %q send --to 1 --from impostor --stdin >/dev/null 2>&1; tmux set-option -w @test_sender_rc "$?"; tmux wait-for -S %q' \
  "$GANG" "$claimed_sent"
tmux send-keys -l -t "$(window_id alpha)" "$claimed_send_command"
tmux send-keys -t "$(window_id alpha)" Enter
tmux wait-for "$claimed_sent"
equal "a Gangline window cannot override its observable identity" "1" \
  "$(tmux show-options -wqv -t "$(window_id alpha)" @test_sender_rc)"
excludes "a refused claimed identity delivers nothing" "$(pane 1)" "MARK_FALSE_CLAIM"

if printf 'MARK_UNSIGNED' | "$GANG" send --to 1 --stdin >/dev/null 2>&1; then
  fail "an outside caller must provide its identity" "send exited successfully"
else
  pass "an outside caller must provide its identity"
fi

alpha_delivery_id="$(window_id alpha)"
alpha_delivery_lock="$GANG_LOCK_DIR/$(printf '%s' "$alpha_delivery_id" | tr -c 'A-Za-z0-9' '_').lock"
mkdir -p "$GANG_LOCK_DIR"
ln -s "$$" "$alpha_delivery_lock"
if live_lock_refusal="$(printf 'MARK_LIVE_LOCK' |
  GANG_LOCK_WAIT=not-a-number "$GANG" send --to alpha --from tester --stdin 2>&1)"; then
  live_lock_rc=0
else
  live_lock_rc=$?
fi
equal "a live delivery lock refuses immediately" "3" "$live_lock_rc"
contains "a live delivery lock explains the contention" \
  "$live_lock_refusal" "another Gangline process is delivering"
excludes "a live delivery lock prevents the paste" "$(pane alpha)" "MARK_LIVE_LOCK"
rm -f -- "$alpha_delivery_lock"

ln -s 99999999 "$alpha_delivery_lock"
printf 'MARK_STALE_LOCK' |
  "$GANG" send --to alpha --from tester --stdin >/dev/null
contains "a stale delivery lock is recovered exactly once" \
  "$(pane alpha)" "MARK_STALE_LOCK"
[ ! -e "$alpha_delivery_lock" ] \
  && pass "a recovered delivery releases its lock" \
  || fail "a recovered delivery releases its lock" "$alpha_delivery_lock remains"

tmux send-keys -l -t "$(window_id 1)" HUMAN_DRAFT
draft_refusal=""
if draft_refusal="$(printf 'MARK_DRAFT' |
  "$GANG" send --to 1 --from tester --stdin 2>&1)"; then
  fail "delivery refuses a human draft" "send exited successfully"
else
  pass "delivery refuses a human draft"
fi
contains "a delivery refusal names a runnable inspection command" \
  "$draft_refusal" "gang capture 1"
if "$GANG" capture 1 >/dev/null; then
  pass "the inspection command named by the refusal runs"
else
  fail "the inspection command named by the refusal runs" "gang capture 1 failed"
fi
tmux send-keys -t "$(window_id 1)" C-u

# A harness may park the Enter in its own input queue: the fixture's composer
# flips to the queue hint once the strand flag exists, exactly as claude
# 2.1.223 leaves its box reading "Press up to edit queued messages" while the
# parked preview in the transcript looks like a submitted prompt. The box
# changing is therefore not proof of entry, and delivery must say so instead
# of reporting success.
cat > "$RUN_ROOT/queue-rc" <<'RC'
# --rcfile replaces ~/.bashrc but not /etc/bash.bashrc: Ubuntu's
# command-not-found handler there spends seconds per envelope line, which
# outlasts the immediate verification rhythm this suite runs on.
unset -f command_not_found_handle
PS1='❯ '
PROMPT_COMMAND='[ -f "$QUEUE_STRAND" ] && PS1="❯ Press up to edit queued messages"'
RC
mkdir -p "$RUN_ROOT/profiles"
cat > "$RUN_ROOT/profiles/queueing.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'QUEUE_STRAND=$RUN_ROOT/queue-strand exec bash --rcfile $RUN_ROOT/queue-rc' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
SH
export GANG_PROFILES="$RUN_ROOT/profiles"
"$GANG" hitch strand -p queueing -d /tmp >/dev/null
touch "$RUN_ROOT/queue-strand"
if strand_out="$(printf 'MARK_QUEUED' | "$GANG" send --to strand --from tester --stdin 2>&1)"; then
  fail "a submission the harness parks in its queue is not a delivery" \
    "send reported success"
else
  pass "a submission the harness parks in its queue is not a delivery"
fi
contains "the failure names the parked queue" \
  "$strand_out" "parked it in its own input queue"
contains "and hands over the manual recovery" "$strand_out" "press Up"
contains "the parked message is recorded against the window" \
  "$(tmux show-options -wqv -t "$(window_id strand)" @gl_staged)" "queue"
# With the queue still parked, the NEXT delivery is refused before anything is
# typed, named as the queue rather than blamed on a half-written draft.
if strand_more="$(printf 'MARK_SECOND' | "$GANG" send --to strand --from tester --stdin 2>&1)"; then
  strand_more_rc=0
else
  strand_more_rc=$?
fi
equal "a parked queue refuses the next delivery without typing" "3" "$strand_more_rc"
contains "naming the queue it is waiting on" \
  "$strand_more" "parked earlier input in its own queue"
"$GANG" drop strand >/dev/null

# A staged record is state; the box is fresher evidence. Staged input can
# flush outside gang's sight — an operator's Enter, a queue draining at a
# turn boundary — and status/roster must not report a paste the empty box
# proves gone. A box still holding content keeps the record and the report.
tmux set-option -w -t "$(window_id 1)" @gl_staged \
  "'MARK_GONE' is staged unsent in this box"
excludes "an empty box retires a staged record at status time" \
  "$("$GANG" status 1)" "undelivered input"
equal "and the retired record is gone" "" \
  "$(tmux show-options -wqv -t "$(window_id 1)" @gl_staged)"
tmux set-option -w -t "$(window_id 1)" @gl_staged \
  "'MARK_HELD' is staged unsent in this box"
tmux send-keys -l -t "$(window_id 1)" 'MARK_HELD draft'
contains "a non-empty box keeps the record and the report" \
  "$("$GANG" status 1)" "undelivered input"
contains "roster carries the same verdict" "$("$GANG" roster)" "undelivered-input"
tmux send-keys -t "$(window_id 1)" C-u
tmux set-option -uw -t "$(window_id 1)" @gl_staged

# Clearing a record is evidence the OBSTRUCTION is gone, never retroactive
# proof the recorded body was delivered. A refused delivery changes nothing;
# the next VERIFIED delivery to the same window retires the record, because
# verified success is only reachable through the provably clear box that is
# itself the gone-obstruction evidence.
tmux set-option -w -t "$(window_id 1)" @gl_staged \
  "'MARK_OLD' is staged unsent in this box"
tmux send-keys -l -t "$(window_id 1)" 'blocking draft'
if printf 'MARK_RETAIN' | "$GANG" send --to 1 --from tester --stdin >/dev/null 2>&1; then
  fail "a refused delivery does not clear another record" "send succeeded over a draft"
else
  pass "a refused delivery does not clear another record"
fi
contains "the record survives the refusal" \
  "$(tmux show-options -wqv -t "$(window_id 1)" @gl_staged)" "MARK_OLD"
tmux send-keys -t "$(window_id 1)" C-u
printf 'MARK_CLEARS' | "$GANG" send --to 1 --from tester --stdin >/dev/null
equal "a verified delivery to the same window retires the stale record" "" \
  "$(tmux show-options -wqv -t "$(window_id 1)" @gl_staged)"

# The drain-between-reads race: a prior obstruction can vanish AFTER
# stage_clear reads it (record retained on nonempty evidence) but BEFORE
# input_clear reads again — a queue draining at a turn boundary mid-send. The
# delivery then verifies cleanly and the stale record must not survive it.
# The fixture replays the race deterministically: eight tickets return a
# parked reading for exactly the box reads through stage_clear's landing
# zone — the occupied probe at cmd_send, inject, and stage_clear reads the
# box once each via input_painted, the settled pairs at inject and
# stage_clear read it twice each, and stage_clear's landing zone is the
# eighth — and every later read sees the real, drained composer. The ticket
# count is coupled to that read order; changing inject's guard sequence must
# update it. Leftover tickets fail loudly as a refused send, but EXHAUSTING
# them early would let stage_clear see the drained box and clear the record
# itself — both cores pass and the world silently stops exercising the race —
# so the fixture logs the caller of every parked reading and the world
# asserts the final ticket was consumed by stage_clear's landing-zone read,
# the coupling's one load-bearing position.
cat > "$RUN_ROOT/profiles/drain.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_drain_real="\$(declare -f profile_input)"
eval "drain_real_input \${_gl_drain_real#profile_input}"
profile_input() { # a parked reading per ticket, then the real drained box
  if [ -s "$RUN_ROOT/drain-tickets" ]; then
    sed -i '\$d' "$RUN_ROOT/drain-tickets"
    printf '%s<%s\n' "\${FUNCNAME[1]}" "\${FUNCNAME[2]}" >> "$RUN_ROOT/drain-reads.log"
    printf 'PARKED_OBSTRUCTION'
    return 0
  fi
  drain_real_input "\$1"
}
SH
: > "$RUN_ROOT/drain-tickets"
"$GANG" hitch drain -p drain -d /tmp >/dev/null
tmux set-option -w -t "$(window_id drain)" @gl_staged \
  "'MARK_OLD' is staged unsent in this box"
tmux set-option -w -t "$(window_id drain)" @gl_staged_box "BOXMEMO_NOT_MATCHING"
printf 'x\nx\nx\nx\nx\nx\nx\nx\n' > "$RUN_ROOT/drain-tickets"
if drain_out="$(printf 'MARK_DRAIN' | "$GANG" send --to drain --from tester --stdin 2>&1)"; then
  pass "a delivery whose prior obstruction drained mid-send still verifies"
else
  fail "a delivery whose prior obstruction drained mid-send still verifies" \
    "$drain_out"
fi
equal "and the verified delivery retires the drained record" "" \
  "$(tmux show-options -wqv -t "$(window_id drain)" @gl_staged)"
equal "the final parked reading was stage_clear's own landing-zone read" \
  "landing_zone<stage_clear" "$(tail -1 "$RUN_ROOT/drain-reads.log")"
"$GANG" drop drain >/dev/null

# Once queue evidence is declared, an UNREADABLE verification reread is
# ambiguity, not proof of submission. The fixture's composer flips to a
# sentinel after Enter and its profile_input grants exactly one readable look
# at that sentinel: the first reading breaks the change loop as a normal
# non-queue submission would, and the late queue-evidence reread finds the
# box unreadable. Falling through to success here is the hole; the send must
# die naming the uncertainty and record the body as unknown.
cat > "$RUN_ROOT/flicker-rc" <<'RC'
unset -f command_not_found_handle
PS1='❯ '
PROMPT_COMMAND='[ -f "$FLICKER_FLAG" ] && PS1="❯ POST_SENTINEL"'
RC
cat > "$RUN_ROOT/profiles/flicker.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'FLICKER_FLAG=$RUN_ROOT/flicker-flag exec bash --rcfile $RUN_ROOT/flicker-rc' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*QUEUE_HINT_NEVER_SHOWN\$'
profile_input() { # one readable look at the post-Enter sentinel, then nothing
  local line
  line="\$(tmux capture-pane -pJ -t "\$1" | grep '❯' | tail -1)" || return 1
  line="\${line#*❯}"
  case "\$line" in
    *POST_SENTINEL*)
      [ ! -e "$RUN_ROOT/flicker-seen" ] || return 1
      : > "$RUN_ROOT/flicker-seen" ;;
  esac
  printf '%s' "\$line" | tr -d '\302\240'
}
SH
"$GANG" hitch flicker -p flicker -d /tmp >/dev/null
touch "$RUN_ROOT/flicker-flag"
if flicker_out="$(printf 'MARK_FLICKER' | "$GANG" send --to flicker --from tester --stdin 2>&1)"; then
  fail "an unreadable verification reread is not a delivery" "send reported success"
else
  pass "an unreadable verification reread is not a delivery"
fi
contains "the ambiguity is named rather than spent as success" \
  "$flicker_out" "is unknown"
contains "the uncertain body is recorded against the window" \
  "$(tmux show-options -wqv -t "$(window_id flicker)" @gl_staged)" "is unknown"
"$GANG" drop flicker >/dev/null

# A profile-provided native compaction command uses the same verified injection
# primitive. The fixture makes execution immediately visible in its pane.
mkdir -p "$RUN_ROOT/profiles"
cat > "$RUN_ROOT/profiles/native.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_COMPACT_CMD="printf NATIVE_COMPACT"
SH
export GANG_PROFILES="$RUN_ROOT/profiles"
"$GANG" hitch compactable -p native -d /tmp >/dev/null
"$GANG" compact compactable >/dev/null
contains "compact submits the profile's native command" \
  "$(pane compactable)" "NATIVE_COMPACT"

# A self-request made inside an agent's own pane must not submit the native
# command during that turn. Stop consumes it once, after which a one-shot worker
# submits the profile command and exits. Both waits below are tmux event barriers,
# not clocks or polling loops.
self_executed="test-self-compact-executed-$$"
cat > "$RUN_ROOT/profiles/deferred.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_COMPACT_CMD="printf SELF_COMPACT; tmux wait-for -S $self_executed"
GANG_SELF_COMPACT=deferred
SH
"$GANG" hitch selfable -p deferred -d /tmp >/dev/null
self_id="$(window_id selfable)"
self_tmux_pane="$(tmux list-panes -t "$self_id" -F '#{pane_id}')"
self_requested="test-self-compact-requested-$$"
printf -v self_command 'GANG_SESSION=%q GANG_PROFILES=%q %q compact selfable; tmux wait-for -S %q' \
  "$GANG_SESSION" "$GANG_PROFILES" "$GANG" "$self_requested"
tmux send-keys -l -t "$self_id" "$self_command"
tmux send-keys -t "$self_id" Enter
tmux wait-for "$self_requested"
self_request="$(tmux show-options -wqv -t "$self_id" @gl_self_compact_requested)"
contains "self-compaction records one request inside the running agent" \
  "$(pane selfable)" "self-compaction scheduled for the end of this turn"
excludes "self-compaction does not submit before Stop" \
  "$(pane selfable)" "SELF_COMPACT"

if [ -n "$self_request" ]; then
  tmux wait-for "gang-self-compact-$self_request" &
  self_dispatch_waiter=$!
  tmux wait-for "$self_executed" &
  self_execute_waiter=$!
  printf '%s' '{"hook_event_name":"Stop"}' |
    TMUX_PANE="$self_tmux_pane" "$GANG" hook >/dev/null
  wait "$self_execute_waiter"
  wait "$self_dispatch_waiter"
  contains "native Stop submits the deferred self-compaction command" \
    "$(pane selfable)" "SELF_COMPACT"
  equal "the one-shot self-compaction worker exits without an error" "" \
    "$(tmux show-options -wqv -t "$self_id" @gl_self_compact_failed)"
else
  fail "self-compaction records one request inside the running agent" \
    "@gl_self_compact_requested is empty"
fi

# The universal hook endpoint records immediately against the firing pane.
alpha_id="$(window_id alpha)"
alpha_tmux_pane="$(tmux list-panes -t "$alpha_id" -F '#{pane_id}')"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' |
  TMUX_PANE="$alpha_tmux_pane" "$GANG" hook >/dev/null
turn_open="$(tmux show-options -wqv -t "$alpha_id" @gl_turn)"
contains "a native prompt hook opens the turn record" "$turn_open" "open"

printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$alpha_tmux_pane" "$GANG" hook >/dev/null
turn_closed="$(tmux show-options -wqv -t "$alpha_id" @gl_turn)"
contains "a native stop hook closes the turn record" "$turn_closed" "closed"

# Optional context guidance has two edge-triggered states and no clock path.
cat > "$RUN_ROOT/profiles/lights.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
profile_context() {
  tmux show-options -wqv -t "\$1" @test_context
}
SH
GANG_CONTEXT_LIGHTS=100000,200000 "$GANG" hitch lit -p lights -d /tmp >/dev/null
lit_id="$(window_id lit)"
lit_tmux_pane="$(tmux list-panes -t "$lit_id" -F '#{pane_id}')"
warming="$(printf '%s' '{"hook_event_name":"UserPromptSubmit"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
equal "context warm-up is silent before the first completed turn" "" "$warming"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook >/dev/null
tmux set-option -w -t "$lit_id" @test_context '150k/300k (50%)'
yellow="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
contains "enabled lights expose yellow" "$yellow" "Yellow context light"
repeat="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
equal "a context light is emitted once per context epoch" "" "$repeat"
tmux set-option -w -t "$lit_id" @test_context '250k/300k (83%)'
red="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
contains "enabled lights expose red" "$red" "Red context light"
tmux set-option -w -t "$lit_id" @test_context '50k/300k (17%)'
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook >/dev/null
tmux set-option -w -t "$lit_id" @test_context '150k/300k (50%)'
yellow_again="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
contains "dropping below yellow starts a new context epoch" \
  "$yellow_again" "Yellow context light"
tmux set-option -w -t "$lit_id" @test_context 'unreadable'
unavailable="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
contains "an enabled light source fails visibly to its own agent" \
  "$unavailable" "Context lights unavailable"
unavailable_repeat="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
equal "an unavailable context source reports once per failure epoch" \
  "" "$unavailable_repeat"
excludes "status does not inspect another agent's context" \
  "$("$GANG" status lit)" "context"

"$GANG" down >/dev/null
if tmux has-session -t "=$GANG_SESSION" 2>/dev/null; then
  fail "down removes the exact test session" "session still exists"
else
  pass "down removes the exact test session"
fi

printf '\n%s checks in %ss\n' "$checks" "$SECONDS"
[ "$fails" -eq 0 ]
