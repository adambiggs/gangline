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
export GANG_CONFIG_DIR="$RUN_ROOT/config"
export XDG_CONFIG_HOME="$RUN_ROOT/xdg"
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

refuses() { # $1 description, $2 expected message, rest = command
  local description="$1" expected="$2" output
  shift 2
  if output="$("$@" 2>&1)"; then
    fail "$description" "command unexpectedly succeeded: [$output]"
  else
    contains "$description" "$output" "$expected"
  fi
}

window_id() { # $1 exact window name
  tmux list-windows -t "=$GANG_SESSION" -F '#{window_id} #{window_name}' |
    awk -v wanted="$1" '$2 == wanted { print $1; exit }'
}

pane() { tmux capture-pane -pJ -t "$(window_id "$1")"; }
pane_all() { tmux capture-pane -pJ -S - -t "$(window_id "$1")"; }

# Help coverage is derived from the dispatcher, so missing help cannot hide by
# also being absent from a hand-maintained help inventory. The exclusions are
# deliberate non-operator routes: the native callback, the hitch alias, and
# help's own dispatcher arms.
dispatch_commands="$({
  sed -n '/^case "$cmd" in$/,/^esac$/p' "$GANG" |
    awk '
      /^  [^*].*\)/ {
        arm=$1; sub(/\).*/, "", arm)
        n=split(arm, names, "|")
        for (i=1; i<=n; i++) print names[i]
      }
    '
} | awk '$0 != "hook" && $0 != "spawn" && $0 != "-h" && $0 != "--help" && $0 != "help"' | sort -u)"
bare_error_commands="hitch adopt send flush interrupt compact context usage status capture composer drop"
meaningful_bare_commands="up roster attach profiles config cutoff down"
classified_commands="$(printf '%s\n' $bare_error_commands $meaningful_bare_commands | sort -u)"

help_width_failure() { # stdin = help; prints every line wider than 48 chars
  python3 -c 'import sys
for number, line in enumerate(sys.stdin.read().splitlines(), 1):
    if len(line) > 48:
        print(f"line {number} ({len(line)}): {line}")'
}

top_help="$($GANG --help)"
top_wide="$(printf '%s\n' "$top_help" | help_width_failure)"
equal "top-level help fits the phone-SSH width" "" "$top_wide"
equal "every dispatched operator command has a bare classification" \
  "$dispatch_commands" "$classified_commands"
help_inventory="$(printf '%s\n' "$top_help" | awk '/^  [a-z]/ { print $1 }' | sort -u)"
equal "help names the classified command inventory" \
  "$classified_commands" "$help_inventory"
while read -r help_command; do
  [ -n "$help_command" ] || continue
  if command_help="$($GANG "$help_command" --help 2>&1)"; then
    contains "help exists for gang $help_command" "$command_help" "gang $help_command"
  else
    fail "help exists for gang $help_command" "$command_help"
  fi
  command_wide="$(printf '%s\n' "$command_help" | help_width_failure)"
  equal "gang $help_command help fits the phone-SSH width" "" "$command_wide"
done <<<"$dispatch_commands"

for bare_command in $bare_error_commands; do
  if bare_output="$(env -u TMUX_PANE "$GANG" "$bare_command" 2>&1)"; then
    fail "bare gang $bare_command prints help and refuses outside an agent" \
      "command unexpectedly succeeded: [$bare_output]"
  else
    contains "bare gang $bare_command prints its own synopsis" \
      "$bare_output" "gang $bare_command"
  fi
done

# Start the private tmux server with the wrong session in its global environment.
# Hitched harnesses must receive the exact team identity in their launch command,
# rather than inheriting whichever session started the shared server.
GANG_SESSION=stale-session tmux new-session -d -s environment-seed

# Public profile surface: the bash fixture remains test-only.
profiles="$(GANG_TEST_PROFILES='' "$GANG" profiles | tr '\n' ' ')"
equal "the public profile list is the supported harness set" \
  "claude-code codex opencode pi " "$profiles"
xdg_config_report="$(env -u GANG_CONFIG_DIR "$GANG" config)"
contains "the fallback config root stays inside the isolated run root" \
  "$xdg_config_report" "$RUN_ROOT/xdg/gangline"

# User configuration is parsed as a lower-precedence layer, never executed or
# sourced. Each case names its own root so malformed fixtures cannot poison the
# commands that follow.
CONFIG_CASES="$RUN_ROOT/config-cases"
mkdir -p "$CONFIG_CASES/file" "$CONFIG_CASES/env"
tmux new-session -d -s config-file-session -n from-file
tmux new-session -d -s config-env-session -n from-env
printf '%s\n' 'GANG_SESSION=config-file-session' > "$CONFIG_CASES/file/config"
config_file_roster="$(env -u GANG_SESSION GANG_CONFIG_DIR="$CONFIG_CASES/file" "$GANG" roster)"
contains "the config file supplies the session a real roster addresses" \
  "$config_file_roster" "from-file"
config_env_roster="$(GANG_CONFIG_DIR="$CONFIG_CASES/file" GANG_SESSION=config-env-session "$GANG" roster)"
contains "the environment session outranks the config file" \
  "$config_env_roster" "from-env"
excludes "the overridden config session is not addressed" \
  "$config_env_roster" "from-file"

mkdir -p "$CONFIG_CASES/literal-command"
printf 'GANG_SESSION=$(touch "%s")\n' "$CONFIG_CASES/was-executed" \
  > "$CONFIG_CASES/literal-command/config"
literal_command_report="$(env -u GANG_SESSION \
  GANG_CONFIG_DIR="$CONFIG_CASES/literal-command" "$GANG" config)"
if [ ! -e "$CONFIG_CASES/was-executed" ]; then
  pass "the config file is parsed and never executed"
else
  fail "the config file is parsed and never executed" \
    "the command substitution created $CONFIG_CASES/was-executed"
fi
contains "the unexecuted command text remains the literal configured value" \
  "$literal_command_report" \
  "GANG_SESSION=\$(touch \"$CONFIG_CASES/was-executed\")"

mkdir -p "$CONFIG_CASES/literal-metacharacters"
literal_meta_marker="$CONFIG_CASES/metacharacters-executed"
printf '%s\n' "GANG_LOCK_DIR=space # quote ' double \" backtick \` \$(touch $literal_meta_marker) equals=a=b" \
  > "$CONFIG_CASES/literal-metacharacters/config"
literal_meta_report="$(env -u GANG_LOCK_DIR \
  GANG_CONFIG_DIR="$CONFIG_CASES/literal-metacharacters" "$GANG" config)"
contains "quotes, backticks, command text, hashes, spaces, and equals stay literal" \
  "$literal_meta_report" \
  "GANG_LOCK_DIR=space # quote ' double \" backtick \` \$(touch $literal_meta_marker) equals=a=b"
if [ ! -e "$literal_meta_marker" ]; then
  pass "literal config metacharacters execute nothing"
else
  fail "literal config metacharacters execute nothing" \
    "the literal value created $literal_meta_marker"
fi

mkdir -p "$CONFIG_CASES/report"
printf '%s\n' 'GANG_SESSION=file-report' 'GANG_PROFILE=codex' \
  > "$CONFIG_CASES/report/config"
config_report="$(GANG_CONFIG_DIR="$CONFIG_CASES/report" \
  GANG_SESSION=env-report "$GANG" config)"
contains "gang config attributes an environment override to both layers" \
  "$config_report" \
  $'GANG_SESSION=env-report\tenvironment (overriding config line 1)'
contains "gang config attributes a file-layer value to its line" \
  "$config_report" \
  $'GANG_PROFILE=codex\t'"$CONFIG_CASES/report/config line 2"
contains "gang config attributes an untouched built-in value to the default" \
  "$config_report" $'GANG_CLEAR_PRESSES=40\tdefault'

config_escape=$'safe\033unsafe'
sanitised_report="$(GANG_CONFIG_DIR="$CONFIG_CASES/env" \
  GANG_SESSION="$config_escape" "$GANG" config)"
contains "gang config makes an environment control byte visible" \
  "$sanitised_report" "GANG_SESSION=safe?unsafe"
excludes "gang config never writes the raw environment control byte" \
  "$sanitised_report" "$config_escape"

mkdir -p "$CONFIG_CASES/doctrine-present" "$CONFIG_CASES/doctrine-absent"
printf '%s\n' 'placeholder' > "$CONFIG_CASES/doctrine-present/DOCTRINE.md"
contains "gang config reports a present doctrine slot" \
  "$(GANG_CONFIG_DIR="$CONFIG_CASES/doctrine-present" "$GANG" config)" \
  $'doctrine\t'"$CONFIG_CASES/doctrine-present/DOCTRINE.md"$'\tpresent'
contains "gang config reports an absent doctrine slot" \
  "$(GANG_CONFIG_DIR="$CONFIG_CASES/doctrine-absent" "$GANG" config)" \
  $'doctrine\t'"$CONFIG_CASES/doctrine-absent/DOCTRINE.md"$'\tabsent'

mkdir -p "$CONFIG_CASES/unknown"
printf '%s\n' 'GANG_NOPE=value' > "$CONFIG_CASES/unknown/config"
refuses "an unknown config key names its file, line, and key" \
  "$CONFIG_CASES/unknown/config line 1 has unknown key GANG_NOPE" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/unknown" "$GANG" profiles

mkdir -p "$CONFIG_CASES/profile-declaration"
printf '%s\n' 'GANG_LAUNCH=claude' > "$CONFIG_CASES/profile-declaration/config"
profile_decl_out="$(env GANG_CONFIG_DIR="$CONFIG_CASES/profile-declaration" "$GANG" profiles 2>&1 || true)"
contains "a profile declaration is refused under its own name" \
  "$profile_decl_out" "GANG_LAUNCH is a profile declaration, not operator configuration"
contains "the profile-declaration refusal points at the supported escape hatch" \
  "$profile_decl_out" "point GANG_PROFILES at its directory"

mkdir -p "$CONFIG_CASES/test-switch"
printf '%s\n' 'GANG_TEST_PROFILES=1' > "$CONFIG_CASES/test-switch/config"
refuses "the suite-only profile switch cannot be persisted" \
  "GANG_TEST_PROFILES is a per-invocation switch" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/test-switch" "$GANG" profiles

mkdir -p "$CONFIG_CASES/bootstrap"
printf '%s\n' 'GANG_CONFIG_DIR=/tmp/elsewhere' > "$CONFIG_CASES/bootstrap/config"
refuses "the config root cannot bootstrap itself from inside the file" \
  "GANG_CONFIG_DIR bootstraps the config file being read" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/bootstrap" "$GANG" profiles

mkdir -p "$CONFIG_CASES/duplicate"
printf '%s\n' 'GANG_SESSION=first' 'GANG_SESSION=second' > "$CONFIG_CASES/duplicate/config"
refuses "a duplicated config key names both lines" \
  "duplicates GANG_SESSION on lines 1 and 2" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/duplicate" "$GANG" profiles

mkdir -p "$CONFIG_CASES/empty"
printf '%s\n' 'GANG_SESSION=' > "$CONFIG_CASES/empty/config"
refuses "an empty config value says to delete the line" \
  "a key with no value states nothing — delete the line to take the default" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/empty" "$GANG" profiles

mkdir -p "$CONFIG_CASES/nul"
printf 'GANG_SESSION=alpha\000omega\n' > "$CONFIG_CASES/nul/config"
refuses "a NUL byte is refused instead of silently disappearing" \
  "contains a NUL byte" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/nul" "$GANG" profiles

mkdir -p "$CONFIG_CASES/crlf" "$CONFIG_CASES/escape"
printf 'GANG_BOOT_TIMEOUT=30\r\n' > "$CONFIG_CASES/crlf/config"
refuses "a CRLF config line is refused as control-bearing" \
  "contains control characters other than tab and newline" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/crlf" "$GANG" profiles
printf 'GANG_SESSION=alpha\033omega\n' > "$CONFIG_CASES/escape/config"
refuses "a bare escape in a config value is refused" \
  "contains control characters other than tab and newline" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/escape" "$GANG" profiles

mkdir -p "$CONFIG_CASES/no-final-newline"
printf 'GANG_SESSION=config-file-session   ' > "$CONFIG_CASES/no-final-newline/config"
trimmed_roster="$(env -u GANG_SESSION GANG_CONFIG_DIR="$CONFIG_CASES/no-final-newline" "$GANG" roster)"
contains "a newline-free final value parses after trailing blanks are stripped" \
  "$trimmed_roster" "from-file"

for root_source in GANG_CONFIG_DIR XDG_CONFIG_HOME HOME; do
  case "$root_source" in
    GANG_CONFIG_DIR)
      relative_out="$(GANG_CONFIG_DIR=relative "$GANG" profiles 2>&1 || true)" ;;
    XDG_CONFIG_HOME)
      relative_out="$(env -u GANG_CONFIG_DIR XDG_CONFIG_HOME=relative "$GANG" profiles 2>&1 || true)" ;;
    HOME)
      relative_out="$(env -u GANG_CONFIG_DIR -u XDG_CONFIG_HOME HOME=relative "$GANG" profiles 2>&1 || true)" ;;
  esac
  contains "a relative root from $root_source is refused under that source" \
    "$relative_out" "$root_source must resolve Gangline's config root to an absolute path"
done

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
claude_hook_declarations="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  'unset GANG_STOP_HOOK GANG_SELF_COMPACT; . "$1"; printf "%s|%s" "${GANG_STOP_HOOK:-}" "${GANG_SELF_COMPACT:-}"' \
  fixture "$claude_profile")"
equal "a hooked Claude launch declares Stop and deferred self-compaction together" \
  "1|deferred" "$claude_hook_declarations"
claude_unhooked_root="$RUN_ROOT/claude'guard"
mkdir -p "$claude_unhooked_root/bin"
printf '%s\n' '#!/bin/sh' '# SPDX-License-Identifier: Apache-2.0' \
  > "$claude_unhooked_root/bin/gang"
chmod +x "$claude_unhooked_root/bin/gang"
claude_unhooked_declarations="$(ROOT="$claude_unhooked_root" GANG_CONTEXT_LIGHTS=off bash -c \
  'unset GANG_STOP_HOOK GANG_SELF_COMPACT; . "$1"; printf "%s|%s" "${GANG_STOP_HOOK:-}" "${GANG_SELF_COMPACT:-}"' \
  fixture "$claude_profile")"
equal "an unhooked Claude launch claims neither Stop nor deferred compaction" \
  "|" "$claude_unhooked_declarations"
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

mkdir -p "$RUN_ROOT/profiles"
export GANG_PROFILES="$RUN_ROOT/profiles"
cat > "$RUN_ROOT/profiles/ctx-known.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_SESSION_KEY=1
profile_context() { printf '42k/200k (21%%)\n'; }
SH
cat > "$RUN_ROOT/profiles/ctx-fail.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
profile_context() { die 'fixture context unavailable'; }
SH
cat > "$RUN_ROOT/profiles/ctx-none.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
unset -f profile_context
SH
cat > "$RUN_ROOT/usage-bashrc" <<'SH'
PS1='❯ '
u() {
  printf '\033[1A\r\033[K'
  local i=1
  while [ "$i" -le 30 ]; do
    printf 'USAGE_%02d\n' "$i"
    i=$((i + 1))
  done
}
SH
cat > "$RUN_ROOT/usage-confirm-bashrc" <<'SH'
PS1='❯ '
c() {
  IFS= read -r _
  printf '\033[H\033[2JCONFIRMED_USAGE\n'
  IFS= read -r -n 1 _
}
SH
cat > "$RUN_ROOT/profiles/usage-inline.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="bash --init-file '$RUN_ROOT/usage-bashrc'"
GANG_USAGE_CMD='u'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
SH
cat > "$RUN_ROOT/profiles/usage-confirm.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="bash --init-file '$RUN_ROOT/usage-confirm-bashrc'"
GANG_USAGE_CMD='c'
GANG_USAGE_CONFIRM_KEY="Enter"
GANG_USAGE_RENDER="modal"
GANG_USAGE_DISMISS_KEY="Escape"
SH
cat > "$RUN_ROOT/profiles/usage-modal.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_USAGE_CMD='printf "\033[H\033[2JMODAL_ONE\nMODAL_TWO\n"; IFS= read -r -n 1 _'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="modal"
GANG_USAGE_DISMISS_KEY="Escape"
SH
cat > "$RUN_ROOT/profiles/usage-stuck.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_USAGE_CMD='printf "\033[H\033[2JMODAL_STUCK\n"; while :; do IFS= read -r -n 1 _; [ "$_" = q ] && break; done'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="modal"
GANG_USAGE_DISMISS_KEY="C-g"
SH
cat > "$RUN_ROOT/profiles/usage-nochange.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_USAGE_CMD="clear"
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
SH
cat > "$RUN_ROOT/profiles/usage-occupied.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_USAGE_CMD='printf SHOULD_NOT_RUN'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
GANG_OCCUPIED_REGEX='OCCUPIED_USAGE'
_gl_usage_occupied_input="$(declare -f profile_input)"
eval "usage_occupied_real_input ${_gl_usage_occupied_input#profile_input}"
profile_input() {
  tmux capture-pane -pJ -t "$1" | grep -q OCCUPIED_USAGE && return 1
  usage_occupied_real_input "$1"
}
SH
cat > "$RUN_ROOT/profiles/usage-unknown.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_USAGE_CMD='printf SHOULD_NOT_RUN'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="unknown"
GANG_USAGE_DISMISS_KEY=""
SH

# Real tmux substrate: lifecycle, observation, verified attributed delivery and
# exact-name addressing. Gangline's command returns only after the state checked
# below has been established.
"$GANG" hitch alpha -p bash -d /tmp >/dev/null
contains "hitch creates an observable idle agent" "$($GANG status alpha)" "idle"
contains "roster lists the hitched profile" "$($GANG roster)" "alpha"
contains "roster is an immediate snapshot" \
  "$(GANG_CHURN_WAIT=not-a-duration $GANG roster)" "alpha"

alpha_id="$(window_id alpha)"
alpha_tmux_pane="$(tmux list-panes -t "$alpha_id" -F '#{pane_id}')"
contains "bare status targets the calling agent window" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" status)" "idle"
contains "bare capture targets the calling agent window" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" capture)" \
  "You are alpha in Gangline"
equal "bare composer targets the calling agent window" "" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" composer)"

GANG_CONTEXT_LIGHTS=off "$GANG" hitch ctx-agent -p ctx-known -d /tmp >/dev/null
ctx_agent_id="$(window_id ctx-agent)"
ctx_agent_pane="$(tmux list-panes -t "$ctx_agent_id" -F '#{pane_id}')"
equal "named context prints the profile reading byte-for-byte" \
  "42k/200k (21%)" "$("$GANG" context ctx-agent)"
equal "bare context targets the calling agent window" \
  "42k/200k (21%)" "$(TMUX_PANE="$ctx_agent_pane" "$GANG" context)"
equal "context answers while context lights are off" \
  "42k/200k (21%)" "$(GANG_CONTEXT_LIGHTS=off "$GANG" context ctx-agent)"
if [ -n "$(tmux show-options -wqv -t "$ctx_agent_id" @gl_key)" ]; then
  pass "a session-key profile mints @gl_key even with context lights off"
else
  fail "a session-key profile mints @gl_key even with context lights off" \
    "@gl_key was empty"
fi
"$GANG" hitch ctx-failing -p ctx-fail -d /tmp >/dev/null
excludes "roster carries no context reading column" \
  "$("$GANG" roster)" "42k/200k"

ctx_fail_stdout="$RUN_ROOT/context-fail.stdout"
ctx_fail_stderr="$RUN_ROOT/context-fail.stderr"
if "$GANG" context ctx-failing >"$ctx_fail_stdout" 2>"$ctx_fail_stderr"; then
  fail "a profile context failure stays non-zero" "context unexpectedly succeeded"
else
  pass "a profile context failure stays non-zero"
fi
equal "a profile context failure fabricates no reading" "" "$(<"$ctx_fail_stdout")"
contains "a profile context failure keeps its own diagnostic" \
  "$(<"$ctx_fail_stderr")" "fixture context unavailable"

"$GANG" hitch ctx-missing -p ctx-none -d /tmp >/dev/null
refuses "a missing profile_context names the profile" \
  "profile 'ctx-none' declares no profile_context" \
  "$GANG" context ctx-missing

GANG_CONTEXT_LIGHTS=off "$GANG" hitch usage-inline -p usage-inline -d /tmp >/dev/null
usage_inline_id="$(window_id usage-inline)"
tmux resize-window -t "$usage_inline_id" -x 80 -y 12
usage_marker_ready="test-usage-marker-ready-$$"
printf -v usage_marker_cmd 'printf "INLINE_OLD_MARKER\\n"; tmux wait-for -S %q' \
  "$usage_marker_ready"
tmux send-keys -l -t "$usage_inline_id" "$usage_marker_cmd"
tmux send-keys -t "$usage_inline_id" Enter
tmux wait-for "$usage_marker_ready"
expected_usage="$(awk 'BEGIN { for (i=1; i<=30; i++) printf "USAGE_%02d\n", i }')"
usage_inline_out="$("$GANG" usage usage-inline)"
equal "inline usage returns every line taller than the pane" \
  "$expected_usage" "$(printf '%s\n' "$usage_inline_out" | sed 's/[[:space:]]*$//')"
excludes "inline usage excludes the pre-existing transcript" \
  "$usage_inline_out" "INLINE_OLD_MARKER"
equal "inline usage restores an empty composer" "" \
  "$("$GANG" composer usage-inline)"

"$GANG" hitch usage-modal -p usage-modal -d /tmp >/dev/null
usage_modal_out="$("$GANG" usage usage-modal)"
equal "modal usage returns the visible page raw" \
  $'MODAL_ONE\nMODAL_TWO' \
  "$(printf '%s\n' "$usage_modal_out" | sed 's/[[:space:]]*$//')"
equal "modal usage dismisses back to an empty composer" "" \
  "$("$GANG" composer usage-modal)"

"$GANG" hitch usage-confirm -p usage-confirm -d /tmp >/dev/null
usage_confirm_out="$("$GANG" usage usage-confirm)"
equal "usage presses the profile's confirmation key before capture" \
  "CONFIRMED_USAGE" \
  "$(printf '%s\n' "$usage_confirm_out" | sed 's/[[:space:]]*$//')"
equal "confirmed modal usage restores an empty composer" "" \
  "$("$GANG" composer usage-confirm)"

"$GANG" hitch usage-stuck -p usage-stuck -d /tmp >/dev/null
usage_stuck_stdout="$RUN_ROOT/usage-stuck.stdout"
usage_stuck_stderr="$RUN_ROOT/usage-stuck.stderr"
if "$GANG" usage usage-stuck >"$usage_stuck_stdout" 2>"$usage_stuck_stderr"; then
  fail "usage refuses when its dismissal does not restore the composer" \
    "usage unexpectedly succeeded"
else
  pass "usage refuses when its dismissal does not restore the composer"
fi
contains "failed usage restoration still prints the captured content" \
  "$(<"$usage_stuck_stdout")" "MODAL_STUCK"
contains "failed usage restoration names the key and gang attach" \
  "$(<"$usage_stuck_stderr")" "after C-g"
contains "failed usage restoration points at gang attach" \
  "$(<"$usage_stuck_stderr")" "gang attach"

usage_before_refusals="$(pane usage-inline)"
tmux set-option -w -t "$usage_inline_id" @gl_turn "open $(date +%s)"
refuses "usage refuses a busy target" "is mid-turn" \
  "$GANG" usage usage-inline
tmux set-option -w -t "$usage_inline_id" @gl_turn broken
refuses "usage refuses a could-not-determine target" \
  "cannot determine whether 'usage-inline' is mid-turn" \
  "$GANG" usage usage-inline
tmux set-option -uw -t "$usage_inline_id" @gl_turn
equal "usage readiness refusals type nothing" \
  "$usage_before_refusals" "$(pane usage-inline)"

"$GANG" hitch usage-occupied -p usage-occupied -d /tmp >/dev/null
usage_occupied_id="$(window_id usage-occupied)"
usage_occupied_ready="test-usage-occupied-ready-$$"
printf -v usage_occupied_cmd 'printf OCCUPIED_USAGE; tmux wait-for -S %q; IFS= read -r _' \
  "$usage_occupied_ready"
tmux send-keys -l -t "$usage_occupied_id" "$usage_occupied_cmd"
tmux send-keys -t "$usage_occupied_id" Enter
tmux wait-for "$usage_occupied_ready"
usage_occupied_before="$(pane usage-occupied)"
refuses "usage refuses an occupied target" "occupied (authority unknown)" \
  "$GANG" usage usage-occupied
equal "an occupied usage refusal types nothing" \
  "$usage_occupied_before" "$(pane usage-occupied)"

refuses "a profile with no GANG_USAGE_CMD refuses usage" \
  "declares no GANG_USAGE_CMD" "$GANG" usage alpha

"$GANG" hitch usage-nochange -p usage-nochange -d /tmp >/dev/null
usage_nochange_id="$(window_id usage-nochange)"
usage_nochange_ready="test-usage-nochange-ready-$$"
printf -v usage_nochange_cmd \
  'PROMPT_COMMAND=%q; clear' "tmux wait-for -S $usage_nochange_ready; PROMPT_COMMAND="
tmux send-keys -l -t "$usage_nochange_id" "$usage_nochange_cmd"
tmux send-keys -t "$usage_nochange_id" Enter
tmux wait-for "$usage_nochange_ready"
usage_nochange_stdout="$RUN_ROOT/usage-nochange.stdout"
usage_nochange_stderr="$RUN_ROOT/usage-nochange.stderr"
if "$GANG" usage usage-nochange \
    >"$usage_nochange_stdout" 2>"$usage_nochange_stderr"; then
  fail "usage refuses when the native screen never changes" \
    "usage unexpectedly succeeded"
else
  pass "usage refuses when the native screen never changes"
fi
contains "an unchanged usage screen names the command" \
  "$(<"$usage_nochange_stderr")" "after clear"
equal "an unchanged usage screen prints no content" "" \
  "$(<"$usage_nochange_stdout")"
equal "an unchanged usage screen leaves the composer empty" "" \
  "$("$GANG" composer usage-nochange)"

tmux set-option -g history-limit 5
"$GANG" hitch usage-rollover -p usage-inline -d /tmp >/dev/null
tmux set-option -g history-limit 2000
usage_rollover_id="$(window_id usage-rollover)"
tmux resize-window -t "$usage_rollover_id" -x 80 -y 12
usage_rollover_stdout="$RUN_ROOT/usage-rollover.stdout"
usage_rollover_stderr="$RUN_ROOT/usage-rollover.stderr"
if "$GANG" usage usage-rollover \
    >"$usage_rollover_stdout" 2>"$usage_rollover_stderr"; then
  fail "usage refuses a rolled-over inline scrollback" \
    "usage unexpectedly succeeded"
else
  pass "usage refuses a rolled-over inline scrollback"
fi
contains "a rolled-over usage read names the lost origin" \
  "$(<"$usage_rollover_stderr")" "scrollback of 'usage-rollover' rolled over"
equal "a rolled-over usage read prints no content" "" \
  "$(<"$usage_rollover_stdout")"

"$GANG" hitch usage-unknown -p usage-unknown -d /tmp >/dev/null
refuses "usage refuses an unknown render declaration" \
  "unknown GANG_USAGE_RENDER 'unknown'" "$GANG" usage usage-unknown

alpha_before_bare_help="$(pane alpha)"
alpha_composer_before_bare_help="$($GANG composer alpha)"
for incoherent_bare in hitch adopt send flush interrupt usage drop; do
  if incoherent_output="$(TMUX_PANE="$alpha_tmux_pane" "$GANG" "$incoherent_bare" 2>&1)"; then
    fail "bare gang $incoherent_bare refuses inside an agent" \
      "command unexpectedly succeeded: [$incoherent_output]"
  else
    contains "bare gang $incoherent_bare prints help inside an agent" \
      "$incoherent_output" "gang $incoherent_bare"
  fi
done
equal "incoherent bare commands leave the calling pane untouched" \
  "$alpha_before_bare_help" "$(pane alpha)"
equal "incoherent bare commands leave the composer untouched" \
  "$alpha_composer_before_bare_help" "$($GANG composer alpha)"
"$GANG" drop ctx-agent >/dev/null
"$GANG" drop ctx-failing >/dev/null
"$GANG" drop ctx-missing >/dev/null
"$GANG" drop usage-inline >/dev/null
"$GANG" drop usage-modal >/dev/null
"$GANG" drop usage-confirm >/dev/null
"$GANG" drop usage-stuck >/dev/null
"$GANG" drop usage-occupied >/dev/null
"$GANG" drop usage-nochange >/dev/null
"$GANG" drop usage-rollover >/dev/null
"$GANG" drop usage-unknown >/dev/null

outside_status="$(env -u TMUX_PANE "$GANG" status 2>&1 || true)"
contains "bare status outside an agent prints its synopsis" \
  "$outside_status" "gang status"
contains "bare status outside an agent explains why self is unavailable" \
  "$outside_status" "this shell is not a Gangline agent window"

for meaningful_command in roster profiles config cutoff; do
  if meaningful_output="$($GANG "$meaningful_command" 2>&1)"; then
    excludes "bare gang $meaningful_command keeps its ordinary meaning" \
      "$meaningful_output" "gang — drive native CLI agents in tmux"
  else
    fail "bare gang $meaningful_command keeps its ordinary meaning" \
      "$meaningful_output"
  fi
done
binary_stamp="$(tmux show-options -wqv -t "$alpha_id" @gl_binary_id)"
if [[ "$binary_stamp" =~ ^cksum:[0-9]+:[0-9]+$ ]]; then
  pass "hitch stamps the documented binary identity"
else
  fail "hitch stamps the documented binary identity" "got [$binary_stamp]"
fi
excludes "status is quiet when the stamped binary is current" \
  "$("$GANG" status alpha)" "binary-skew"
excludes "roster is quiet when the stamped binary is current" \
  "$("$GANG" roster)" "binary-skew"
tmux set-option -w -t "$alpha_id" @gl_binary_id cksum:1:2
contains "status warns when the running window has another binary identity" \
  "$("$GANG" status alpha)" "binary-skew (cksum:1:2 != current $binary_stamp)"
contains "roster warns when the running window has another binary identity" \
  "$("$GANG" roster)" "binary-skew (cksum:1:2 != current $binary_stamp)"
tmux set-option -w -t "$alpha_id" @gl_binary_id "$binary_stamp"

installed_root="$RUN_ROOT/installed"
mkdir -p "$installed_root/bin"
cp -R "$ROOT/profiles" "$installed_root/profiles"
cp "$GANG" "$installed_root/bin/gang"
installed_gang="$installed_root/bin/gang"
excludes "byte-identical Gangline copies compare as current" \
  "$("$installed_gang" status alpha)" "binary-skew"
printf '\n# fixture changes the executable bytes\n' >> "$installed_gang"
changed_stamp="$(cksum "$installed_gang" | awk '{ print "cksum:" $1 ":" $2 }')"
contains "an uncommitted executable change produces binary skew" \
  "$("$installed_gang" status alpha)" \
  "binary-skew ($binary_stamp != current $changed_stamp)"

tmux set-option -uw -t "$alpha_id" @gl_binary_id
contains "status warns when a pre-witness window is unstamped" \
  "$("$GANG" status alpha)" "binary-skew (window unstamped; current $binary_stamp)"
contains "roster warns when a pre-witness window is unstamped" \
  "$("$GANG" roster)" "binary-skew (window unstamped; current $binary_stamp)"
tmux set-option -w -t "$alpha_id" @gl_binary_id "$binary_stamp"

mkdir -p "$RUN_ROOT/no-identity"
cat > "$RUN_ROOT/no-identity/git" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
exit 127
SH
chmod +x "$RUN_ROOT/no-identity/git"
contains "a broken git command cannot abort roster" \
  "$(PATH="$RUN_ROOT/no-identity:$PATH" "$GANG" roster)" "alpha"
cat > "$RUN_ROOT/no-identity/cksum" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
exit 127
SH
chmod +x "$RUN_ROOT/no-identity/cksum"
unavailable_status="$(PATH="$RUN_ROOT/no-identity:$PATH" "$GANG" status alpha)"
contains "an unavailable witness is explicit without aborting status" \
  "$unavailable_status" \
  "binary-identity unavailable (window $binary_stamp; current unavailable)"
contains "an unavailable witness does not abort roster" \
  "$(PATH="$RUN_ROOT/no-identity:$PATH" "$GANG" roster)" \
  "binary-identity unavailable (window $binary_stamp; current unavailable)"

adopted_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n adopted "PS1='❯ ' bash --norc")"
"$GANG" adopt adopted -p bash >/dev/null
equal "adopt stamps the current binary identity" "$binary_stamp" \
  "$(tmux show-options -wqv -t "$adopted_id" @gl_binary_id)"
"$GANG" drop adopted >/dev/null

# composer reads the box through the profile's styled reading, not the raw
# pane; a freshly hitched agent's box is definitively empty
equal "composer prints nothing for an empty box" \
  "" "$("$GANG" composer alpha)"

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
contains "startup requires deliberate model and effort choices when hitching" \
  "$(pane alpha)" \
  "When you hitch a teammate, choose its model and reasoning effort deliberately and pass them as -m and -e"
contains "startup carries the operator-authorized marathon rule" \
  "$(pane alpha)" \
  "Marathon rule: never halt the session to wait on the operator — resolve forks by doctrine and report decisions past-tense; when a fork genuinely needs the operator (irreversible, outside doctrine), state it in your report, park that one lane, and keep every other lane moving."
contains "startup states the complement of envelope attribution" \
  "$(pane alpha)" \
  "Gangline never delivers a message without one — the only unenveloped text it ever types into a pane is your harness's own compaction command — so any other unenveloped text arrived from the session keyboard, and Gangline cannot attribute it further."
excludes "an absent doctrine leaves no doctrine origin in the base contract" \
  "$(pane alpha)" "Operator doctrine ("
excludes "startup contains no session-marker prompt" "$(pane alpha)" "Session marker"
excludes "startup does not ask for a reply to its synthetic sender" \
  "$(pane alpha)" "Reply to that sender"
equal "context lights leave no state when disabled" "|" \
  "$(tmux show-options -wqv -t "$(window_id alpha)" @gl_context_lights)|$(tmux show-options -wqv -t "$(window_id alpha)" @gl_key)"

mkdir -p "$CONFIG_CASES/literal-lock"
literal_lock="$CONFIG_CASES/lock with space # literal"
printf '%s\n' "GANG_LOCK_DIR=$literal_lock" > "$CONFIG_CASES/literal-lock/config"
printf '%s\n' 'MARK_CONFIG_LITERAL_LOCK' |
  env -u GANG_LOCK_DIR GANG_CONFIG_DIR="$CONFIG_CASES/literal-lock" \
    "$GANG" send --to alpha --from config-test --stdin >/dev/null
if [ -d "$literal_lock" ]; then
  pass "a config value keeps spaces and a literal hash in the lock path"
else
  fail "a config value keeps spaces and a literal hash in the lock path" \
    "gang did not create its lock base at [$literal_lock]"
fi

mkdir -p "$CONFIG_CASES/broken-hook"
printf '%s\n' 'GANG_UNKNOWN=value' > "$CONFIG_CASES/broken-hook/config"
alpha_pane_id="$(tmux list-panes -t "$alpha_id" -F '#{pane_id}')"
if hook_config_out="$(printf '%s' '{"hook_event_name":"Stop"}' |
  env GANG_CONFIG_DIR="$CONFIG_CASES/broken-hook" TMUX_PANE="$alpha_pane_id" \
    "$GANG" hook 2>&1)"; then
  fail "a malformed config makes a native hook fail visibly" \
    "hook unexpectedly succeeded: [$hook_config_out]"
else
  contains "a malformed config makes a native hook fail visibly" \
    "$hook_config_out" "$CONFIG_CASES/broken-hook/config line 1"
fi

mkdir -p "$CONFIG_CASES/bad-file-value" "$CONFIG_CASES/bad-env-value"
printf '%s\n' 'GANG_BOOT_TIMEOUT=abc' > "$CONFIG_CASES/bad-file-value/config"
if bad_file_out="$(env -u GANG_BOOT_TIMEOUT GANG_CONFIG_DIR="$CONFIG_CASES/bad-file-value" \
  "$GANG" hitch config-bad-file -p bash -d /tmp 2>&1)"; then
  fail "a bad configured value is blamed on its file line" \
    "hitch unexpectedly succeeded"
else
  contains "a bad configured value is blamed on its file line" \
    "$bad_file_out" "from $CONFIG_CASES/bad-file-value/config line 1"
fi
tmux kill-window -t "=$GANG_SESSION:config-bad-file" 2>/dev/null || true
if bad_env_out="$(GANG_BOOT_TIMEOUT=abc GANG_CONFIG_DIR="$CONFIG_CASES/bad-env-value" \
  "$GANG" hitch config-bad-env -p bash -d /tmp 2>&1)"; then
  fail "a bad environment value is blamed on the environment" \
    "hitch unexpectedly succeeded"
else
  contains "a bad environment value is blamed on the environment" \
    "$bad_env_out" "from the environment"
fi
tmux kill-window -t "=$GANG_SESSION:config-bad-env" 2>/dev/null || true

mkdir -p "$CONFIG_CASES/missing-profile"
printf '%s\n' 'GANG_PROFILE=no-such-profile' > "$CONFIG_CASES/missing-profile/config"
if missing_profile_out="$(env -u GANG_PROFILE GANG_CONFIG_DIR="$CONFIG_CASES/missing-profile" \
  "$GANG" hitch config-missing-profile -d /tmp 2>&1)"; then
  fail "an unknown configured profile is blamed on its file line" \
    "hitch unexpectedly succeeded"
else
  contains "an unknown configured profile is blamed on its file line" \
    "$missing_profile_out" "from $CONFIG_CASES/missing-profile/config line 1"
fi
excludes "a refused configured profile opens no window" \
  "$(tmux list-windows -t "=$GANG_SESSION" -F '#W')" "config-missing-profile"

mkdir -p "$CONFIG_CASES/nested" "$RUN_ROOT/nested-parent-dir" "$RUN_ROOT/nested-child-dir"
printf '%s\n' 'GANG_PROFILE=bash' 'GANG_SESSION=config-nested-session' \
  > "$CONFIG_CASES/nested/config"
env -u GANG_PROFILE -u GANG_SESSION GANG_CONFIG_DIR="$CONFIG_CASES/nested" \
  "$GANG" hitch nested-parent -d "$RUN_ROOT/nested-parent-dir" >/dev/null
nested_parent_id="$(tmux list-windows -t '=config-nested-session' \
  -F '#{window_id} #W' | awk '$2 == "nested-parent" { print $1; exit }')"
nested_done="test-config-nested-done-$$"
tmux send-keys -t "$nested_parent_id" \
  "$GANG hitch nested-child -d '$RUN_ROOT/nested-child-dir' >/dev/null; tmux wait-for -S '$nested_done'" Enter
tmux wait-for "$nested_done"
nested_child_id="$(tmux list-windows -t '=config-nested-session' \
  -F '#{window_id} #W' | awk '$2 == "nested-child" { print $1; exit }')"
equal "a nested hitch in another directory reloads the pinned config profile" \
  "bash" "$(tmux show-options -wqv -t "$nested_child_id" @gl_profile)"
contains "the nested hitch joins the session supplied only by the config file" \
  "$(tmux list-windows -t '=config-nested-session' -F '#W')" "nested-child"

DOCTRINE_CASES="$RUN_ROOT/doctrine-cases"
mkdir -p "$DOCTRINE_CASES/present"
printf '%s\n' 'MARK_DOCTRINE_PRESENT binds this hitch.' \
  > "$DOCTRINE_CASES/present/DOCTRINE.md"
GANG_CONFIG_DIR="$DOCTRINE_CASES/present" \
  "$GANG" hitch doctrine-present -p bash -d /tmp >/dev/null
contains "a doctrine-bearing hitch still carries its base identity contract" \
  "$(pane_all doctrine-present)" "You are doctrine-present in Gangline"
contains "a present operator doctrine is injected into the startup contract" \
  "$(pane doctrine-present)" "MARK_DOCTRINE_PRESENT binds this hitch."
contains "a doctrine-bearing startup states the complement of envelope attribution" \
  "$(pane_all doctrine-present)" \
  "Gangline never delivers a message without one — the only unenveloped text it ever types into a pane is your harness's own compaction command — so any other unenveloped text arrived from the session keyboard, and Gangline cannot attribute it further."
"$GANG" drop doctrine-present >/dev/null

GANG_CONFIG_DIR="$DOCTRINE_CASES/present" TMUX_PANE="$alpha_pane_id" \
  "$GANG" hitch doctrine-inside -p bash -d /tmp >/dev/null
contains "a hitch invoked from inside the team carries operator doctrine" \
  "$(pane doctrine-inside)" "MARK_DOCTRINE_PRESENT binds this hitch."
"$GANG" drop doctrine-inside >/dev/null

GANG_CONFIG_DIR="$DOCTRINE_CASES/present" GANG_SESSION=doctrine-cross-session \
  TMUX_PANE="$alpha_pane_id" \
  "$GANG" hitch doctrine-cross -p bash -d /tmp >/dev/null
cross_doctrine_pane="$(tmux capture-pane -pJ -t '=doctrine-cross-session:doctrine-cross')"
contains "a cross-session hitch carries operator doctrine too" \
  "$cross_doctrine_pane" "MARK_DOCTRINE_PRESENT binds this hitch."

mkdir -p "$DOCTRINE_CASES/multiline"
printf '%s\n' 'MARK_DOCTRINE_HEAD' 'middle doctrine line' 'MARK_DOCTRINE_TAIL' \
  > "$DOCTRINE_CASES/multiline/DOCTRINE.md"
GANG_CONFIG_DIR="$DOCTRINE_CASES/multiline" \
  "$GANG" hitch doctrine-multiline -p bash -d /tmp >/dev/null
contains "a multiline doctrine delivers its first line" \
  "$(pane doctrine-multiline)" "MARK_DOCTRINE_HEAD"
contains "a multiline doctrine delivers its last line" \
  "$(pane doctrine-multiline)" "MARK_DOCTRINE_TAIL"
"$GANG" drop doctrine-multiline >/dev/null

mkdir -p "$DOCTRINE_CASES/tag"
printf '%s\n' '# [gang: counterfeit] MARK_DOCTRINE_TAG' \
  > "$DOCTRINE_CASES/tag/DOCTRINE.md"
GANG_CONFIG_DIR="$DOCTRINE_CASES/tag" \
  "$GANG" hitch doctrine-tag -p bash -d /tmp >/dev/null
contains "a tag-shaped doctrine opener is visibly neutralised" \
  "$(pane doctrine-tag)" "# (gang: counterfeit] MARK_DOCTRINE_TAG"
excludes "a doctrine cannot add a second gang envelope opener" \
  "$(pane doctrine-tag)" "[gang: counterfeit]"
"$GANG" drop doctrine-tag >/dev/null

for bad_doctrine in invalid-utf8 nul control-cr; do
  mkdir -p "$DOCTRINE_CASES/$bad_doctrine"
  case "$bad_doctrine" in
    invalid-utf8) printf '\377' > "$DOCTRINE_CASES/$bad_doctrine/DOCTRINE.md"; expected_doctrine="not valid UTF-8" ;;
    nul) printf 'before\000after' > "$DOCTRINE_CASES/$bad_doctrine/DOCTRINE.md"; expected_doctrine="contains a NUL byte" ;;
    control-cr) printf 'before\rafter' > "$DOCTRINE_CASES/$bad_doctrine/DOCTRINE.md"; expected_doctrine="contains control characters other than tab and newline" ;;
  esac
  if bad_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/$bad_doctrine" \
    "$GANG" hitch "doctrine-$bad_doctrine" -p bash -d /tmp 2>&1)"; then
    fail "a $bad_doctrine doctrine is refused before launch" \
      "hitch unexpectedly succeeded"
  else
    contains "a $bad_doctrine doctrine is refused before launch" \
      "$bad_doctrine_out" "$expected_doctrine"
  fi
  excludes "the refused $bad_doctrine doctrine opens no window" \
    "$(tmux list-windows -t "=$GANG_SESSION" -F '#W')" "doctrine-$bad_doctrine"
done

mkdir -p "$DOCTRINE_CASES/over-ceiling"
awk 'BEGIN { for (i = 0; i < 8193; i++) printf "x" }' \
  > "$DOCTRINE_CASES/over-ceiling/DOCTRINE.md"
if ceiling_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/over-ceiling" \
  "$GANG" hitch doctrine-over-ceiling -p bash -d /tmp 2>&1)"; then
  fail "an over-ceiling doctrine is refused before launch" \
    "hitch unexpectedly succeeded"
else
  contains "an over-ceiling doctrine is refused before launch" \
    "$ceiling_out" "exceeds the 8192-byte category-error ceiling"
fi
excludes "the over-ceiling refusal leaves no window" \
  "$(tmux list-windows -t "=$GANG_SESSION" -F '#W')" "doctrine-over-ceiling"

mkdir -p "$DOCTRINE_CASES/unreadable"
printf '%s\n' 'unreadable doctrine' > "$DOCTRINE_CASES/unreadable/DOCTRINE.md"
chmod 000 "$DOCTRINE_CASES/unreadable/DOCTRINE.md"
if unreadable_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/unreadable" \
  "$GANG" hitch doctrine-unreadable -p bash -d /tmp 2>&1)"; then
  fail "an unreadable doctrine is refused before launch" \
    "hitch unexpectedly succeeded"
else
  contains "an unreadable doctrine is refused before launch" \
    "$unreadable_doctrine_out" "not a readable regular file"
fi
chmod 600 "$DOCTRINE_CASES/unreadable/DOCTRINE.md"
excludes "the unreadable-doctrine refusal leaves no window" \
  "$(tmux list-windows -t "=$GANG_SESSION" -F '#W')" "doctrine-unreadable"

mkdir -p "$DOCTRINE_CASES/pane-overflow"
awk 'BEGIN { for (i = 0; i < 2048; i++) printf "p" }' \
  > "$DOCTRINE_CASES/pane-overflow/DOCTRINE.md"
if pane_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/pane-overflow" \
  "$GANG" hitch doctrine-pane-overflow -p bash -d /tmp 2>&1)"; then
  fail "a doctrine the target pane cannot render fails at delivery" \
    "hitch unexpectedly succeeded"
else
  contains "a doctrine the target pane cannot render fails at delivery" \
    "$pane_doctrine_out" "startup contract to 'doctrine-pane-overflow' was not delivered"
fi
contains "a delivery-sized doctrine failure leaves its window for inspection" \
  "$(tmux list-windows -t "=$GANG_SESSION" -F '#W')" "doctrine-pane-overflow"
"$GANG" drop doctrine-pane-overflow >/dev/null

# Hitch has the same refusal contract as send. Start a fixture whose composer
# already carries the profile's parked-queue evidence, so inject refuses before
# pasting and the public hitch boundary must preserve that status and message.
cat > "$RUN_ROOT/profiles/doctrine-prequeued.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="PS1='❯ Press up to edit queued messages' bash --norc"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
SH
doctrine_refusal_rc=0
doctrine_refusal_out="$("$GANG" hitch doctrine-refusal \
  -p doctrine-prequeued -d /tmp 2>&1)" || doctrine_refusal_rc=$?
equal "a startup delivery refusal preserves the inject refusal status" \
  "3" "$doctrine_refusal_rc"
contains "a startup delivery refusal has one diagnostic prefix" \
  "$doctrine_refusal_out" \
  "gang: startup contract to 'doctrine-refusal' was not delivered: refusing to deliver"
excludes "a startup delivery refusal does not nest a second diagnostic prefix" \
  "$doctrine_refusal_out" ": gang: refusing to deliver"
"$GANG" drop doctrine-refusal >/dev/null

# A queued-composer fixture records the exact startup body before Enter. That
# record is the witness for the trailing bytes the pane itself cannot display
# unambiguously.
cat > "$RUN_ROOT/doctrine-queue-rc" <<'RC'
unset -f command_not_found_handle
PS1='❯ '
PROMPT_COMMAND='if [ ! -e "$DOCTRINE_QUEUE_SEEN" ]; then : > "$DOCTRINE_QUEUE_SEEN"; elif [ -e "$DOCTRINE_QUEUE_ARM" ]; then PS1="❯ Press up to edit queued messages"; fi'
RC
cat > "$RUN_ROOT/profiles/doctrine-queueing.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'DOCTRINE_QUEUE_SEEN=$RUN_ROOT/doctrine-queue-seen DOCTRINE_QUEUE_ARM=$RUN_ROOT/doctrine-queue-arm exec bash --rcfile $RUN_ROOT/doctrine-queue-rc' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
profile_input() {
  local box
  box="\$(tmux capture-pane -pJ -t "\$1" | awk '
    { line[NR] = \$0
      if (index(\$0, "❯")) start = NR
      if (\$0 != "") last = NR }
    END {
      if (!start) exit 1
      s = line[start]; sub(/^.*❯/, "", s); print s
      for (i = start + 1; i <= last; i++) print line[i]
    }' | sed 's/[[:space:]]*\$//')" || return 1
  printf '%s' "\$box" | tr -d '\302\240'
}
SH
mkdir -p "$DOCTRINE_CASES/trailing"
printf 'TAIL_MARK\n\n\n' > "$DOCTRINE_CASES/trailing/DOCTRINE.md"
: > "$RUN_ROOT/doctrine-queue-arm"
if trailing_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/trailing" \
  "$GANG" hitch doctrine-trailing -p doctrine-queueing -d /tmp 2>&1)"; then
  fail "the trailing-newline witness parks the startup contract" \
    "hitch unexpectedly succeeded"
else
  case "$trailing_out" in
    *"parked it in its own input queue"*)
      pass "the trailing-newline witness parks the startup contract" ;;
    *) fail "the trailing-newline witness parks the startup contract" "$trailing_out" ;;
  esac
fi
trailing_id="$(window_id doctrine-trailing)"
trailing_body="$(tmux show-options -wqv -t "$trailing_id" @gl_parked)"
trailing_blanks="$(printf '%s' "$trailing_body" | awk '
  /TAIL_MARK/ { after_tail = 1; next }
  /End this turn\./ { print blanks; exit }
  after_tail && /^[[:space:]]*$/ { blanks++ }
')"
equal "a doctrine's two trailing blank lines survive byte-exact assembly" \
  "4" "$trailing_blanks"
"$GANG" drop doctrine-trailing >/dev/null

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
# The command a refusal names is the one that answers the question it raises.
# That was `gang capture`, whose raw pane renders a dim suggested-prompt
# placeholder identically to a half-written line — the reading that produced a
# public misdiagnosis. `gang composer` is the profile's styled reading, so what
# it prints is what a human typed; the next check spends it on this very box.
contains "a delivery refusal names a runnable inspection command" \
  "$draft_refusal" "gang composer 1"
if "$GANG" composer 1 >/dev/null; then
  pass "the inspection command named by the refusal runs"
else
  fail "the inspection command named by the refusal runs" "gang composer 1 failed"
fi
contains "and classifies what it refused on" "$draft_refusal" "[draft:"
# the refusal above is the barrier proving the draft is on screen
contains "composer prints what a human typed" \
  "$("$GANG" composer 1)" "HUMAN_DRAFT"
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
# The record says what gang did; the status line says what is in the box now.
# An operator reading a raw capture here sees the harness's queue hint and has
# to know what it means — the classification says it, with the verb that fixes
# it, so a parked queue is never diagnosed as somebody's half-written line.
strand_status="$("$GANG" status strand)"
contains "status classifies the parked box, not just the record" \
  "$strand_status" "box: parked:"
contains "and points at the recovery for that class" \
  "$strand_status" "gang flush strand"
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
# Recovery is the profile's word too. This profile declares the evidence and no
# recall key, so gang knows the composer is parked and does not know which
# keystroke loads the body back — which is a refusal, never a guessed keypress.
if norecall_out="$("$GANG" flush strand 2>&1)"; then
  fail "a profile with no declared recall key refuses to flush" "flush reported success"
else
  pass "a profile with no declared recall key refuses to flush"
fi
contains "and the refusal names the missing declaration" \
  "$norecall_out" "GANG_QUEUE_RECALL_KEY"
"$GANG" drop strand >/dev/null

# THE PARKED QUEUE, RECOVERED RATHER THAN DESCRIBED. The fixture's composer
# reads as the queue hint while its strand file exists, and the body the
# harness "parked" is the last command in the pane's own history — so the
# profile's declared recall key genuinely loads that body back into the box,
# the way Up does in claude's composer. The drain flag is what a queue entering
# the session looks like from outside: the next prompt after it appears clears
# the strand.
# The queue appears the way a real one does — as a consequence of a submission
# the harness swallowed, not as scenery arranged beforehand. Arming the fixture
# makes the next prompt raise the strand; the composer then reads as the hint
# while it is empty, and as its own contents once the recall key loads them.
# That distinction is the whole subject here, so the hint lives in the profile's
# reader rather than in the prompt string, where it would concatenate with the
# recalled body and make every readback look altered.
cat > "$RUN_ROOT/flush-rc" <<'RC'
unset -f command_not_found_handle
PS1='❯ '
HISTCONTROL=ignorespace
PROMPT_COMMAND='if [ -f "$FLUSH_DRAIN" ]; then rm -f "$FLUSH_STRAND" "$FLUSH_DRAIN"; fi
if [ -f "$FLUSH_ARM" ]; then rm -f "$FLUSH_ARM"; : > "$FLUSH_STRAND"; fi
if [ -s "$FLUSH_SIGNAL" ]; then _flush_chan="$(cat "$FLUSH_SIGNAL")"; : > "$FLUSH_SIGNAL"
  tmux wait-for -S "$_flush_chan"; fi'
_flush_probe() {   # what the composer holds, read where input ordering places it
  printf '%s' "$READLINE_LINE" > "$FLUSH_PROBE"
  tmux wait-for -S "$(cat "$FLUSH_PROBE_CHAN")"
}
bind -x '"\C-t": _flush_probe'
RC
cat > "$RUN_ROOT/profiles/flushable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'FLUSH_STRAND=$RUN_ROOT/flush-strand FLUSH_DRAIN=$RUN_ROOT/flush-drain FLUSH_ARM=$RUN_ROOT/flush-arm FLUSH_SIGNAL=$RUN_ROOT/flush-signal FLUSH_PROBE=$RUN_ROOT/flush-probe FLUSH_PROBE_CHAN=$RUN_ROOT/flush-probe-chan exec bash --rcfile $RUN_ROOT/flush-rc' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
GANG_QUEUE_RECALL_KEY='Up'
profile_input() { # a composer that spans lines, and reads as the hint when empty
  local box
  box="\$(tmux capture-pane -pJ -t "\$1" | awk '
    { line[NR] = \$0
      if (index(\$0, "❯")) start = NR
      if (\$0 != "") last = NR }
    END {
      if (!start) exit 1
      s = line[start]; sub(/^.*❯/, "", s); print s
      for (i = start + 1; i <= last; i++) print line[i]
    }' | sed 's/[[:space:]]*\$//' \\
       | sed '/\[TS\]/{ s/\[TS\]//; s/\$/ /; }')" || return 1
  if [ -f "$RUN_ROOT/flush-strand" ] && ! printf '%s' "\$box" | grep -q '[^[:space:]]'; then
    printf 'Press up to edit queued messages'
    return 0
  fi
  printf '%s' "\$box" | tr -d '\302\240'
}
SH
: > "$RUN_ROOT/flush-signal"
"$GANG" hitch parked -p flushable -d /tmp >/dev/null
parked_id="$(window_id parked)"

# The fixture raises and lowers its strand from a prompt hook, so a world that
# arranges one has to know that hook has finished before it looks. The barrier
# is an event through the pane, not a wait, and it is armed by the settling
# command ITSELF: a hook still pending from an earlier command finds the channel
# file empty and cannot fire it early, so the wait returns after the settling
# command's own hook and nothing is left in flight to type into the composer
# later. Its leading space keeps it out of the history the recall key reads.
# What the composer holds, observed in ORDER rather than at a moment. Capturing
# the pane straight after flush returns is timing luck: tmux send-keys returns
# once the key is enqueued, so a mutant Enter may not have been consumed, nor
# the new prompt painted, when the capture happens — and the world would report
# the still-edited line and pass. The probe key travels the same input path
# behind anything flush sent, so by the time it runs, that Enter has either been
# processed or never existed. It reads the line without submitting it, so the
# correct case is not disturbed by being watched.
flush_probed=0
flush_probe() {
  flush_probed=$((flush_probed + 1))
  local chan="test-flush-probe-$flush_probed-$$"
  printf '%s' "$chan" > "$RUN_ROOT/flush-probe-chan"
  : > "$RUN_ROOT/flush-probe"
  tmux wait-for "$chan" &
  local waiter=$!
  tmux send-keys -t "$parked_id" C-t
  wait "$waiter"
  cat "$RUN_ROOT/flush-probe" 2>/dev/null || true
}

flush_settled=0
flush_settle() {
  flush_settled=$((flush_settled + 1))
  local chan="test-flush-$flush_settled-$$"
  tmux send-keys -l -t "$parked_id" " printf %s $chan > $RUN_ROOT/flush-signal"
  tmux send-keys -t "$parked_id" Enter
  tmux wait-for "$chan"
}

# THE RECORD IS THE WHOLE COMPOSER, NOT ITS FIRST LINE. A record that stops at
# the first line is satisfied by a recalled message whose remainder was
# truncated, altered or extended, and flush would submit that while reporting
# the readback verified.
: > "$RUN_ROOT/flush-arm"
if printf 'MARK_MULTI head\nMARK_MULTI_TAIL' |
  "$GANG" send --to parked --from tester --stdin >/dev/null 2>&1; then
  fail "a multiline body the harness parks is a failed delivery" "send reported success"
else
  pass "a multiline body the harness parks is a failed delivery"
fi
contains "and every line of it is recorded, not just the first" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_parked)" "MARK_MULTI_TAIL"
: > "$RUN_ROOT/flush-drain"
flush_settle

: > "$RUN_ROOT/flush-arm"
if printf 'MARK_PARKED' | "$GANG" send --to parked --from tester --stdin >/dev/null 2>&1; then
  fail "the flush world starts from a message the harness parked" "send reported success"
else
  pass "the flush world starts from a message the harness parked"
fi
parked_record="$(tmux show-options -wqv -t "$parked_id" @gl_parked)"
contains "gang records exactly which body the harness parked" \
  "$parked_record" "MARK_PARKED"

# Nothing to read a composer back against is a refusal, not a blind keypress:
# without the record, the recall key would load and submit whatever happened to
# be there.
tmux set-option -uw -t "$parked_id" @gl_parked
if unrecorded_out="$("$GANG" flush parked 2>&1)"; then
  unrecorded_rc=0
else
  unrecorded_rc=$?
fi
equal "an unrecorded parked message refuses the flush" "3" "$unrecorded_rc"
contains "naming the record it will not proceed without" \
  "$unrecorded_out" "no record of a message parked"
tmux set-option -w -t "$parked_id" @gl_parked "$parked_record"

: > "$RUN_ROOT/flush-drain"
if flush_out="$("$GANG" flush parked 2>&1)"; then
  pass "flush recovers the parked message as a verified operation"
else
  fail "flush recovers the parked message as a verified operation" "$flush_out"
fi
contains "and reports what it verified" "$flush_out" "read back against gang's record"
equal "a verified flush retires the parked record" "" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_parked)"
equal "and the staged record with it" "" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_staged)"

# The composer must still read as parked. With the queue drained, there is
# nothing to recover, and a recorded body is not evidence that outlives it.
tmux set-option -w -t "$parked_id" @gl_parked "$parked_record"
if drained_out="$("$GANG" flush parked 2>&1)"; then
  drained_rc=0
else
  drained_rc=$?
fi
equal "a composer showing no queue evidence refuses the flush" "3" "$drained_rc"
contains "rather than pressing the recall key blindly" \
  "$drained_out" "none of the parked-queue evidence"

# THE READBACK IS LOAD-BEARING, AND IT IS THE WHOLE BODY. Here the recall key
# loads a message that begins with exactly the recorded one and carries extra
# text after it — the case a containment check waves through. The Enter must not
# be pressed, because pressing it would submit words nobody sent.
parked_raw="${parked_record#"${parked_record%%[![:space:]]*}"}"
parked_raw="${parked_raw%"${parked_raw##*[![:space:]]}"}"
: > "$RUN_ROOT/flush-arm"
tmux send-keys -l -t "$parked_id" "$parked_raw EXTRA_WORDS_NOBODY_SENT"
tmux send-keys -t "$parked_id" Enter
flush_settle
if mismatch_out="$("$GANG" flush parked 2>&1)"; then
  fail "a readback that does not match the record is not flushed" \
    "flush reported success"
else
  pass "a readback that does not match the record is not flushed"
fi
# NOT performed, not merely NOT verified: the refusal has to be the readback's
# own, because the re-queue verdict fails this send too and would let a missing
# readback pass as a working one.
contains "refused by the readback rather than by anything downstream of it" \
  "$mismatch_out" "flush NOT performed"
contains "and says the Enter was not pressed" "$mismatch_out" "Enter was NOT pressed"
contains "the recalled body is recorded against the window" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_staged)" "read back as something other than"
excludes "and the message gang recorded was never submitted twice" \
  "$mismatch_out" "flushed the parked message"
# THE COMPOSER, not gang's account of it. Everything above is gang reporting on
# itself; only the line still sitting there says the Enter was withheld, because
# a submitted line leaves it.
contains "the recalled body is still sitting in the composer, unsent" \
  "$(flush_probe)" "EXTRA_WORDS_NOBODY_SENT"
tmux send-keys -t "$parked_id" C-u

# AND IT IS BYTE-EQUAL, WITH NOTHING NORMALIZED AWAY. Trimming trailing blank
# space is line-oriented in every tool that offers it, so it erases the
# difference between a line that ends in two spaces and one that does not —
# a hard line break in Markdown, and a different message. A pane capture pads
# every line to the pane width, so trailing whitespace cannot survive one at
# all; this fixture's composer carries it through a token instead, which is
# what lets the world state the difference the comparison must not discard.
: > "$RUN_ROOT/flush-drain"
flush_settle
: > "$RUN_ROOT/flush-arm"
if printf 'MARK_TS head[TS]' |
  "$GANG" send --to parked --from tester --stdin >/dev/null 2>&1; then
  fail "a body whose line ends in blank space parks like any other" \
    "send reported success"
else
  pass "a body whose line ends in blank space parks like any other"
fi
ts_record="$(tmux show-options -wqv -t "$parked_id" @gl_parked)"
case "$ts_record" in
  *"] ") pass "and the blank space is part of what gang recorded" ;;
  *) fail "and the blank space is part of what gang recorded" "got [$ts_record]" ;;
esac
# The same body with that line ending stripped: byte-different, and identical
# under any per-line trailing-space normalization.
ts_tampered="${ts_record# }"
ts_tampered="${ts_tampered% }"
tmux send-keys -l -t "$parked_id" "$ts_tampered"
tmux send-keys -t "$parked_id" Enter
flush_settle
if ts_out="$("$GANG" flush parked 2>&1)"; then
  fail "a recalled body differing only in a line's trailing space is not flushed" \
    "flush reported success"
else
  pass "a recalled body differing only in a line's trailing space is not flushed"
fi
contains "refused by the readback, not by anything downstream of it" \
  "$ts_out" "flush NOT performed"
contains "and the body is recorded as sitting unsent, never as submitted" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_staged)" "read back as something other than"
contains "with the composer agreeing: it is still sitting there" \
  "$(flush_probe)" "MARK_TS head"
tmux send-keys -t "$parked_id" C-u
"$GANG" drop parked >/dev/null

# A keystroke gang cannot send by name is a broken declaration, refused before
# any window opens: tmux would deliver the letters into the composer instead.
cat > "$RUN_ROOT/profiles/badkey.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_QUEUE_RECALL_KEY='M-x then y'
SH
if badkey_out="$("$GANG" hitch badkey -p badkey -d /tmp 2>&1)"; then
  fail "a recall key that is not a tmux key name is refused" "hitch accepted it"
else
  pass "a recall key that is not a tmux key name is refused"
fi
contains "naming the declaration rather than the operator" \
  "$badkey_out" "GANG_QUEUE_RECALL_KEY"
equal "and the refused declaration leaves no window behind" "" "$(window_id badkey)"

# INTERRUPTING IS A PROFILE'S KEYSTROKE AND A FACT GANG OWNS. The keystroke
# ends a turn the harness will never close for itself, so the bracket it opened
# has to be closed here or the target reads busy until that bound expires.
# Whether the harness actually stopped remains its own verdict; the fact does
# not claim otherwise.
if nokey_out="$("$GANG" interrupt alpha 2>&1)"; then
  fail "a profile with no declared interrupt key refuses to interrupt" \
    "interrupt reported success"
else
  pass "a profile with no declared interrupt key refuses to interrupt"
fi
contains "naming the declaration it would need" "$nokey_out" "GANG_INTERRUPT_KEY"

cat > "$RUN_ROOT/profiles/badstop.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_INTERRUPT_KEY='ctrl then c'
SH
if badstop_out="$("$GANG" hitch badstop -p badstop -d /tmp 2>&1)"; then
  fail "an interrupt key that is not a tmux key name is refused" "hitch accepted it"
else
  pass "an interrupt key that is not a tmux key name is refused"
fi
contains "naming that declaration too" "$badstop_out" "GANG_INTERRUPT_KEY"
equal "and it leaves no window behind either" "" "$(window_id badstop)"

# A TARGET THAT WITNESSES THE KEYSTROKE. Every other assertion about interrupt
# reads something gang wrote — its own report, its own tmux option — so all of
# them hold just as well when no key leaves at all. This fixture binds the
# declared key to a mark of its own, which is the only artifact in the world
# that cannot exist unless the key arrived at the pane. It is also what pins the
# key to the PROFILE's declaration rather than to anything hard-coded in core:
# the bound key is C-g, so an interrupt that sent Escape would leave no mark.
cat > "$RUN_ROOT/interrupt-rc" <<'RC'
unset -f command_not_found_handle
PS1='❯ '
bind -x '"\C-g": printf "INTERRUPT_KEY_RECEIVED\n"'
RC
cat > "$RUN_ROOT/profiles/interruptible.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'exec bash --rcfile $RUN_ROOT/interrupt-rc' fixture"
GANG_INTERRUPT_KEY='C-g'
GANG_BUSY_REGEX='STILL_WORKING'
SH
"$GANG" hitch stoppable -p interruptible -d /tmp >/dev/null
stop_id="$(window_id stoppable)"
tmux set-option -w -t "$stop_id" @gl_turn "open $(date +%s)"
contains "an open turn bracket answers busy" \
  "$("$GANG" status stoppable)" "busy"
contains "interrupt reports the key it sent" \
  "$("$GANG" interrupt stoppable)" "C-g"
# Ordering barrier, not a wait on the thing under test: the pane consumes its
# input in order, so a command that runs at all runs after the keystroke ahead
# of it was processed. A missing or wrong keystroke therefore fails the mark
# below rather than hanging here — which it did until these two Enters existed.
# A line editor reads Escape as the start of a sequence and swallows whatever
# arrives next, so an interrupt key that leaves the pane mid-prefix would eat
# the first byte of this barrier's own command and the suite would wait forever
# on a signal that could never come.
tmux send-keys -t "$stop_id" Enter Enter
stop_settled="test-interrupt-settled-$$"
tmux wait-for "$stop_settled" &
stop_waiter=$!
tmux send-keys -l -t "$stop_id" "tmux wait-for -S $stop_settled"
tmux send-keys -t "$stop_id" Enter
wait "$stop_waiter"
contains "and the declared key actually reached the pane" \
  "$(pane stoppable)" "INTERRUPT_KEY_RECEIVED"
equal "an interrupt drops the bracket nothing else will close" "" \
  "$(tmux show-options -wqv -t "$stop_id" @gl_turn)"
contains "so the keystroke cannot strand a false busy" \
  "$("$GANG" status stoppable)" "idle"
"$GANG" drop stoppable >/dev/null

# AND IT MUST NOT MANUFACTURE THE OPPOSITE LIE. Gang saw a keystroke leave; it
# did not see a turn end. A harness that ignored the key is still painting its
# busy marker, and that evidence has to survive the interrupt — writing a fresh
# closed bracket would answer idle before anything looked at the pane, and the
# next send would enter mid-turn on gang's own say-so.
"$GANG" hitch stubborn -p interruptible -d /tmp >/dev/null
stubborn_id="$(window_id stubborn)"
tmux send-keys -l -t "$stubborn_id" 'printf STILL_WORKING\\n'
tmux send-keys -t "$stubborn_id" Enter
tmux set-option -w -t "$stubborn_id" @gl_turn "open $(date +%s)"
"$GANG" interrupt stubborn >/dev/null
equal "the bracket is dropped there too" "" \
  "$(tmux show-options -wqv -t "$stubborn_id" @gl_turn)"
stubborn_state="$("$GANG" status stubborn)"
contains "a target still painting work stays busy after the interrupt" \
  "$stubborn_state" "busy"
excludes "the interrupt never invents an idle the pane contradicts" \
  "$stubborn_state" "idle"
if midturn_out="$(printf 'MARK_MIDTURN' |
  "$GANG" send --to stubborn --from tester --stdin 2>&1)"; then
  fail "and it stays unreachable while that work is painted" "send entered mid-turn"
else
  pass "and it stays unreachable while that work is painted"
fi
# Refused FOR THAT REASON. A bare non-zero exit is satisfied by any refusal this
# pane could raise — a draft, a lock, an occupied composer — so on its own it
# stops pinning the interrupt's consequence the moment anything else refuses.
contains "refused because the turn is still running, not for some other reason" \
  "$midturn_out" "not safely reachable mid-turn"
excludes "and nothing was typed into it" "$(pane stubborn)" "MARK_MIDTURN"
"$GANG" drop stubborn >/dev/null

# The shipped harnesses that stop on Escape say so themselves; the ones whose
# interrupt gang has not observed declare nothing and refuse the command.
for stopping_profile in claude-code codex; do
  stopping_file="$ROOT/profiles/$stopping_profile.sh"
  stopping_key="$(GANG_TEST_PROFILES='' ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
    '. "$1"; printf "%s" "${GANG_INTERRUPT_KEY:-}"' fixture "$stopping_file")"
  equal "the $stopping_profile profile declares the key that stops its turn" \
    "Escape" "$stopping_key"
done
for unstopping_profile in opencode pi; do
  unstopping_file="$ROOT/profiles/$unstopping_profile.sh"
  unstopping_key="$(GANG_TEST_PROFILES='' ROOT="$ROOT" bash -c \
    '. "$1"; printf "%s" "${GANG_INTERRUPT_KEY:-}"' fixture "$unstopping_file")"
  equal "the $unstopping_profile profile declares no interrupt key until one is verified" \
    "" "$unstopping_key"
done

# SPOOLED DELIVERY. A refused delivery is a live target that cannot take input
# yet, and every refusal happens before a keystroke — so the body is still the
# sender's and can be parked. Nothing in this world polls or schedules: the only
# thing that drains a spool is a native Stop event, which the world fires by
# hand exactly as a harness would.
cat > "$RUN_ROOT/profiles/nodrain.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
SH
"$GANG" hitch nodrain -p nodrain -d /tmp >/dev/null
nodrain_id="$(window_id nodrain)"
nodrain_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$nodrain_id" @gl_spool)"
tmux send-keys -l -t "$nodrain_id" 'HUMAN_DRAFT'
if nohook_out="$(printf 'MARK_NOHOOK' |
  "$GANG" send --to nodrain --from tester --stdin 2>&1)"; then
  fail "a target with no turn boundary does not park a refusal" \
    "send reported success"
else
  equal "a target with no turn boundary keeps refusal status" "3" "$?"
fi
contains "naming the declaration a drain would need" "$nohook_out" "GANG_STOP_HOOK"
contains "and says the message was not parked" "$nohook_out" "NOT parked"
excludes "the refusing target received no body" "$(pane nodrain)" "MARK_NOHOOK"
[ ! -d "$nodrain_spool" ] \
  && pass "nothing undrainable was put on disk" \
  || fail "nothing undrainable was put on disk" "$nodrain_spool exists"
"$GANG" drop nodrain >/dev/null
if super_out="$(printf 'MARK_LONE_SUPERSEDE' |
  "$GANG" send --to alpha --from tester --supersede --live-only --stdin 2>&1)"; then
  fail "superseding a live-only send is refused" "send accepted incompatible flags"
else
  pass "superseding a live-only send is refused"
fi
contains "because live-only never parks" "$super_out" "--live-only never parks"

cat > "$RUN_ROOT/profiles/spoolable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_STOP_HOOK=1
_gl_spool_real="\$(declare -f profile_input)"
eval "spool_real_input \${_gl_spool_real#profile_input}"
profile_input() { # once per armed drain, report what the spool and the lock look like
  local lock dir live=no holder="" waiting=0 f
  lock="$GANG_LOCK_DIR/\$(printf '%s' "\$1" | tr -c 'A-Za-z0-9' '_').lock"
  if [ -f "$RUN_ROOT/claim-watch" ] && [ -L "\$lock" ]; then
    rm -f "$RUN_ROOT/claim-watch"
    dir="$GANG_LOCK_DIR/spool/\$(tmux show-options -wqv -t "\$1" @gl_spool)"
    for f in "\$dir"/sending-*; do [ -f "\$f" ] && waiting=\$((waiting + 1)); done
    holder="\$(readlink "\$lock" 2>/dev/null)" || holder=""
    [ -n "\$holder" ] && kill -0 "\$holder" 2>/dev/null && live=yes
    printf 'holder-alive=%s claimed=%s\n' "\$live" "\$waiting" \
      > "$RUN_ROOT/claim-observed"
  fi
  spool_real_input "\$1"
}
SH
"$GANG" hitch parker -p spoolable -d /tmp >/dev/null
parker_id="$(window_id parker)"
parker_pane_id="$(tmux list-panes -t "$parker_id" -F '#{pane_id}')"
parker_token="$(tmux show-options -wqv -t "$parker_id" @gl_spool)"
if [ -n "$parker_token" ]; then
  pass "a hitched agent already has the spool identity a sender will need"
else
  fail "a hitched agent already has the spool identity a sender will need" \
    "@gl_spool is empty"
fi
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
spool_out="$(printf 'MARK_SPOOLED' |
  "$GANG" send --to parker --from tester --stdin)"
contains "a refused delivery is parked rather than lost" "$spool_out" "spooled for parker"
contains "and says plainly that it was not delivered" "$spool_out" "NOT delivered"
excludes "nothing was typed into the refusing target" "$(pane parker)" "MARK_SPOOLED"
contains "status reports what is waiting for that target" \
  "$("$GANG" status parker)" "spooled: 1"
contains "roster carries the same count" "$("$GANG" roster)" "spooled=1"

if live_only_out="$(printf 'MARK_LIVE_ONLY' |
  "$GANG" send --to parker --from tester --live-only --stdin 2>&1)"; then
  fail "live-only refuses instead of parking" "send unexpectedly succeeded"
else
  pass "live-only refuses instead of parking"
fi
contains "live-only reports the original refusal" "$live_only_out" "draft"
contains "live-only leaves the waiting count unchanged" \
  "$("$GANG" status parker)" "spooled: 1"
excludes "live-only typed nothing into the target" "$(pane parker)" "MARK_LIVE_ONLY"

spool_noop_out="$(printf 'MARK_ANNOUNCED' |
  "$GANG" send --to parker --from tester --spool --supersede --stdin 2>&1)"
contains "the deprecated spool flag announces its no-op" \
  "$spool_noop_out" "is the default now"
contains "and the deprecated form still parks" "$spool_noop_out" "spooled for parker"
contains "its supersession leaves one replacement waiting" \
  "$("$GANG" status parker)" "spooled: 1"

# Two messages from one sender are two messages. Only the sender's explicit
# flag makes a newer one replace an older, and it replaces only its OWN.
printf 'MARK_OTHER_SENDER' |
  "$GANG" send --to parker --from other --stdin >/dev/null
printf 'MARK_STALE' | "$GANG" send --to parker --from tester --stdin >/dev/null
contains "a second message from one sender does not replace the first" \
  "$("$GANG" status parker)" "spooled: 3"
printf 'MARK_LATEST' |
  "$GANG" send --to parker --from tester --supersede --stdin >/dev/null
contains "until the sender says the newer one supersedes them" \
  "$("$GANG" status parker)" "spooled: 2"

# EVERY SENDER WRITES UNDER THE ONE IDENTITY. Minting on the way to parking a
# message would let two senders arriving together mint two, and the message that
# lost would sit in a directory nothing points at, reported as waiting and
# deleted by nothing.
equal "parking a message never re-mints the window's spool identity" \
  "$parker_token" "$(tmux show-options -wqv -t "$parker_id" @gl_spool)"
parker_spool_dir="$GANG_LOCK_DIR/spool/$parker_token"
parker_under_token=0
for parker_entry in "$parker_spool_dir"/[0-9]*; do
  [ -f "$parker_entry" ] && parker_under_token=$((parker_under_token + 1))
done
equal "so every parked message is reachable under it" "2" "$parker_under_token"

tmux send-keys -t "$parker_id" C-u
: > "$RUN_ROOT/claim-watch"
tmux wait-for "gang-spool-drain-$parker_id" &
parker_drain_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$parker_drain_waiter"
parker_drained="$(pane parker)"
contains "the target's own Stop drains what was parked for it" \
  "$parker_drained" "MARK_LATEST"
contains "including the message from the other sender" \
  "$parker_drained" "MARK_OTHER_SENDER"
contains "each drained message keeps its own sender's attribution" \
  "$parker_drained" "[gang:other#"
excludes "a superseded message is never delivered" "$parker_drained" "MARK_STALE"
excludes "nor the one it superseded" "$parker_drained" "MARK_SPOOLED"
excludes "nor the deprecated-form predecessor" "$parker_drained" "MARK_ANNOUNCED"
drain_order="$(printf '%s\n' "$parker_drained" |
  grep -oE 'MARK_OTHER_SENDER|MARK_LATEST' | awk '!seen[$0]++' | tr '\n' ' ')"
equal "and the spool drains oldest first" "MARK_OTHER_SENDER MARK_LATEST " \
  "$drain_order"
excludes "a drained spool reports nothing waiting" \
  "$("$GANG" status parker)" "spooled:"

# Supersession follows the replacement's settled outcome. A verified live B
# retires waiting A; a later Stop has no predecessor left to deliver.
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_LIVE_PREDECESSOR' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
tmux send-keys -t "$parker_id" C-u
printf 'MARK_LIVE_REPLACEMENT' |
  "$GANG" send --to parker --from tester --supersede --stdin >/dev/null
contains "a live superseding replacement is delivered" \
  "$(pane parker)" "MARK_LIVE_REPLACEMENT"
excludes "its predecessor is no longer waiting" \
  "$("$GANG" status parker)" "spooled:"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
excludes "a retired predecessor never arrives after its replacement" \
  "$(pane parker)" "MARK_LIVE_PREDECESSOR"

# WHAT THE DRAIN LOOKED LIKE FROM INSIDE THE PANE, read by the fixture the first
# time gang asked it for the composer while holding the delivery lock. Both
# facts are invariants a background drain has to satisfy every time: the lock
# names a process that is actually alive, or the next sender reads it as stale,
# deletes it, and pastes into the same composer concurrently; and the entry is
# already claimed out of the live spool, or a second drain can deliver the same
# body again, and so can the next turn boundary if this one dies between the
# Enter and the retirement.
claim_observed="$(cat "$RUN_ROOT/claim-observed" 2>/dev/null)" || claim_observed=""
contains "the delivery lock a background drain holds names a live process" \
  "$claim_observed" "holder-alive=yes"
contains "and the entry it is delivering is already claimed out of the live spool" \
  "$claim_observed" "claimed=1"

# An entry a drain claimed and never retired — what a killed worker leaves — is
# never picked up again, and never hides: the ones behind it still drain.
parker_inflight="$parker_spool_dir/sending-00000000000000000001-abadcafe"
printf '%s\n%s\n%s\n' tester MARK_INTERRUPTED \
  '[gang:tester#abadcafe] MARK_INTERRUPTED [/gang:tester#abadcafe]' \
  > "$parker_inflight"
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_BEHIND_IT' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
tmux send-keys -t "$parker_id" C-u
tmux wait-for "gang-spool-drain-$parker_id" &
parker_second_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$parker_second_waiter"
parker_after_second="$(pane parker)"
excludes "a claimed entry is never delivered by a later drain" \
  "$parker_after_second" "MARK_INTERRUPTED"
contains "and the messages behind it are not lost with it" \
  "$parker_after_second" "MARK_BEHIND_IT"
[ -f "$parker_inflight" ] \
  && pass "it stays on disk where a person can read it" \
  || fail "it stays on disk where a person can read it" "$parker_inflight is gone"
parker_held_status="$("$GANG" status parker)"
contains "and status names it rather than losing it quietly" \
  "$parker_held_status" "held (delivery NOT verified — it may still have arrived): MARK_INTERRUPTED"
contains "naming the directory it is readable in, not an empty one" \
  "$parker_held_status" "read them under $parker_spool_dir"
rm -f "$parker_inflight"

# Everything gang parks has a deletion path, and this is it.
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_DIES_WITH_WINDOW' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
parker_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$parker_id" @gl_spool)"
[ -d "$parker_spool" ] \
  && pass "a spooled message is on disk beside the delivery locks" \
  || fail "a spooled message is on disk beside the delivery locks" "$parker_spool is absent"
"$GANG" drop parker >/dev/null
[ ! -d "$parker_spool" ] \
  && pass "dropping an agent deletes its spool" \
  || fail "dropping an agent deletes its spool" "$parker_spool survived"

# A window with no spool identity is refused rather than given one here. Minting
# at the moment a message needs parking is exactly the race the identity exists
# to avoid, so gang says so instead of narrowing the window.
"$GANG" hitch identityless -p spoolable -d /tmp >/dev/null
tmux set-option -uw -t "$(window_id identityless)" @gl_spool
tmux send-keys -l -t "$(window_id identityless)" 'HUMAN_DRAFT'
if identityless_out="$(printf 'MARK_NO_IDENTITY' |
  "$GANG" send --to identityless --from tester --stdin 2>&1)"; then
  fail "a window with no spool identity refuses to park a message" \
    "send reported the message parked"
else
  pass "a window with no spool identity refuses to park a message"
fi
contains "and says what would have to happen instead" \
  "$identityless_out" "re-hitch or re-adopt"
# Refusing is only half of it. Minting on the way past is the race the identity
# exists to avoid, so the refusal must also leave nothing behind — a window that
# came out of this with a token would have been given one at exactly the moment
# two senders could each give it a different one.
equal "and the refusal mints nothing on its way out" "" \
  "$(tmux show-options -wqv -t "$(window_id identityless)" @gl_spool)"
"$GANG" drop identityless >/dev/null

# ADOPTION MINTS IT TOO, and nothing tested that. An adopted window is an agent
# by every other measure, so a spool identity it never received would make
# a refused send fail to park for a target the operator had just enrolled.
tmux new-window -d -t "=$GANG_SESSION" -n taken -c /tmp \
  "sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
"$GANG" adopt taken -p spoolable >/dev/null
taken_id="$(window_id taken)"
taken_token="$(tmux show-options -wqv -t "$taken_id" @gl_spool)"
if [ -n "$taken_token" ]; then
  pass "an adopted agent receives the spool identity a sender will need"
else
  fail "an adopted agent receives the spool identity a sender will need" \
    "@gl_spool is empty"
fi
# And it is a usable one, not merely a present one.
tmux send-keys -l -t "$taken_id" 'HUMAN_DRAFT'
# Tolerated rather than asserted here so a missing identity reports as the two
# red checks it is, instead of aborting the run under set -e and taking every
# later world with it.
printf 'MARK_ADOPTED' |
  "$GANG" send --to taken --from tester --stdin >/dev/null 2>&1 || true
contains "and it can park a refused message under it" \
  "$("$GANG" status taken)" "spooled: 1"
"$GANG" drop taken >/dev/null

# A body that was already typed has an unknown fate, so it is NOT parked: a
# second copy of a message that may have landed is worse than one that failed
# loudly. This composer never changes, so the paste is unverifiable.
cat > "$RUN_ROOT/profiles/unverifiable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_STOP_HOOK=1
profile_input() { printf ''; }
SH
if "$GANG" hitch unverified -p unverifiable -d /tmp >/dev/null 2>&1; then
  fail "a composer that never changes cannot complete a hitch" "hitch reported success"
else
  pass "a composer that never changes cannot complete a hitch"
fi
if unverified_out="$(printf 'MARK_UNVERIFIED' |
  "$GANG" send --to unverified --from tester --stdin 2>&1)"; then
  fail "an unverified delivery is not spooled" "send reported success"
else
  pass "an unverified delivery is not spooled"
fi
contains "it failed rather than refused" "$unverified_out" "delivery NOT verified"
unverified_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$(window_id unverified)" @gl_spool)"
unverified_parked=0
for unverified_entry in "$unverified_spool"/*; do
  [ -f "$unverified_entry" ] && unverified_parked=$((unverified_parked + 1))
done
equal "so nothing was parked for a body that may already have landed" "0" \
  "$unverified_parked"
"$GANG" drop unverified >/dev/null

# A drain that cannot verify its delivery quarantines that entry out of the
# glob and says so. It never sends it again on the chance the first attempt
# missed, and the entries behind it are not lost with it.
cat > "$RUN_ROOT/profiles/wedging.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_STOP_HOOK=1
_gl_wedge_real="\$(declare -f profile_input)"
eval "wedge_real_input \${_gl_wedge_real#profile_input}"
profile_input() { # the real box, then a draft that refuses, then one that never changes
  if [ -f "$RUN_ROOT/wedge-block" ]; then printf 'BLOCKING_DRAFT'; return 0; fi
  if [ -f "$RUN_ROOT/wedge-stuck" ]; then printf ''; return 0; fi
  wedge_real_input "\$1"
}
SH
"$GANG" hitch wedged -p wedging -d /tmp >/dev/null
: > "$RUN_ROOT/wedge-block"
wedged_id="$(window_id wedged)"
wedged_pane_id="$(tmux list-panes -t "$wedged_id" -F '#{pane_id}')"
printf 'MARK_WEDGED' | "$GANG" send --to wedged --from tester --stdin >/dev/null
contains "the blocked message is waiting" "$("$GANG" status wedged)" "spooled: 1"
rm -f "$RUN_ROOT/wedge-block"
: > "$RUN_ROOT/wedge-stuck"
if hard_supersede_out="$(printf 'MARK_HARD_REPLACEMENT' |
  "$GANG" send --to wedged --from tester --supersede --stdin 2>&1)"; then
  fail "a hard-failed replacement does not report success" \
    "send unexpectedly succeeded"
else
  pass "a hard-failed replacement does not report success"
fi
contains "the replacement failed after typing" \
  "$hard_supersede_out" "delivery NOT verified"
contains "a hard failure supersedes nothing" \
  "$("$GANG" status wedged)" "spooled: 1"
tmux send-keys -t "$wedged_id" C-u
tmux wait-for "gang-spool-drain-$wedged_id" &
wedged_drain_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$wedged_pane_id" "$GANG" hook >/dev/null
wait "$wedged_drain_waiter"
wedged_status="$("$GANG" status wedged)"
contains "an unverified drain is reported, not swallowed" \
  "$wedged_status" "spool drain NOT verified"
contains "roster carries that verdict too" "$("$GANG" roster)" "spool-held=1"
excludes "and the entry is not left where it would be sent a second time" \
  "$wedged_status" "spooled:"
wedged_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$wedged_id" @gl_spool)"
wedged_quarantined=0
for wedged_entry in "$wedged_spool"/failed-*; do
  [ -f "$wedged_entry" ] && wedged_quarantined=$((wedged_quarantined + 1))
done
equal "the unverified body is kept where a person can read it" "1" \
  "$wedged_quarantined"
# READ IT. A file of the right name is not a kept message: the promise gang
# makes when it holds a body instead of re-sending it is that the body is still
# there, and only reading it says so.
wedged_body="$(cat "$wedged_spool"/failed-* 2>/dev/null)" || wedged_body=""
contains "the body itself, not just a file with the right name" \
  "$wedged_body" "MARK_WEDGED"
contains "and the sender it was parked under" "$wedged_body" "tester"
# READ OUT OF THE REPORT ITSELF, not recomputed beside it. Holding a message
# instead of re-sending it is only honest if the report says where it went, and
# a check that derives the path independently cannot see the report naming an
# empty one.
contains "and the report hands over the directory it is readable in" \
  "$wedged_status" "read them under $wedged_spool"

# A harness may accept the submission into its own queue after Gangline's
# verification failed and drain it later. The held record is therefore not a
# reason to send a second copy on another Stop event.
tmux send-keys -t "$wedged_id" Enter
rm -f "$RUN_ROOT/wedge-stuck"
wedged_arrived="$(pane wedged)"
wedged_before_count="$(printf '%s\n' "$wedged_arrived" | grep -o 'MARK_WEDGED' | wc -l | tr -d ' ')"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$wedged_pane_id" "$GANG" hook >/dev/null
wedged_after_count="$(pane wedged | grep -o 'MARK_WEDGED' | wc -l | tr -d ' ')"
equal "a later native boundary never re-sends an unverified held entry" \
  "$wedged_before_count" "$wedged_after_count"
"$GANG" drop wedged >/dev/null

# One spool is deliberately left alive for the teardown below to account for.
"$GANG" hitch lingering -p spoolable -d /tmp >/dev/null
lingering_id="$(window_id lingering)"
tmux send-keys -l -t "$lingering_id" 'HUMAN_DRAFT'
printf 'MARK_LINGERS' |
  "$GANG" send --to lingering --from tester --stdin >/dev/null
lingering_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$lingering_id" @gl_spool)"

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
# The wording of a real no-rendering leg: the box could not be read after the
# paste, so there is nothing to match it against later. Calling what turns up in
# that box a human draft would invent the one fact gang is missing, and would
# invent it against gang's own adjacent record of having pasted there.
tmux set-option -w -t "$(window_id 1)" @gl_staged \
  "'MARK_HELD' was pasted into this box and the box could not be read afterwards — it may be sitting there unsent"
tmux send-keys -l -t "$(window_id 1)" 'MARK_HELD draft'
contains "a non-empty box keeps the record and the report" \
  "$("$GANG" status 1)" "undelivered input"
contains "a staged record gang cannot match to the box claims no author" \
  "$("$GANG" status 1)" "box: unattributed:"
# A rendering that exists and does not match is the same missing fact: the box
# has moved since gang recorded it, and who moved it is exactly what is unknown.
tmux set-option -w -t "$(window_id 1)" @gl_staged_box "MARK_HELD something else"
contains "and a rendering that does not match settles nothing either" \
  "$("$GANG" status 1)" "box: unattributed:"
# Same box, now byte-identical to the rendering gang recorded when it staged
# its OWN body — the one comparison that separates gang's text from a person's,
# and the same equality stage_clear retires a record on.
tmux set-option -w -t "$(window_id 1)" @gl_staged_box "$("$GANG" composer 1)"
contains "a box matching gang's recorded rendering is classified as its own" \
  "$("$GANG" status 1)" "box: staged:"
tmux set-option -uw -t "$(window_id 1)" @gl_staged_box
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
  local n
  n="\$(cat "$RUN_ROOT/drain-tickets" 2>/dev/null)"
  if [ "\${n:-0}" -gt 0 ]; then
    printf '%s' "\$((n - 1))" > "$RUN_ROOT/drain-tickets"
    printf '%s<%s\n' "\${FUNCNAME[1]}" "\${FUNCNAME[2]}" >> "$RUN_ROOT/drain-reads.log"
    printf 'PARKED_OBSTRUCTION'
    return 0
  fi
  drain_real_input "\$1"
}
SH
printf '0' > "$RUN_ROOT/drain-tickets"
"$GANG" hitch drain -p drain -d /tmp >/dev/null
tmux set-option -w -t "$(window_id drain)" @gl_staged \
  "'MARK_OLD' is staged unsent in this box"
tmux set-option -w -t "$(window_id drain)" @gl_staged_box "BOXMEMO_NOT_MATCHING"
printf '8' > "$RUN_ROOT/drain-tickets"
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

# An EXPIRED turn bracket is could-not-determine, not busy: stale owned state
# must not veto a delivery that fresh box evidence proves safe. A provably
# empty composer proceeds under the full submission verification; anything
# less refuses naming both the expired witness and the box state. A FRESH
# open bracket keeps refusing mid-turn exactly as before. Delivery leaves
# @gl_turn byte-identical: native hooks write it lock-free, tmux has no
# atomic compare-and-delete, so any reader's unset can erase a fresh hook
# stamp landing between the read and the unset — the invariant pinned here
# and in the malformed world below is that NO reader on delivery's
# transitive path writes the bracket at all.
stale_bracket="open $(( $(date +%s) - 400 ))"
tmux set-option -w -t "$(window_id 1)" @gl_turn "$stale_bracket"
if printf 'MARK_TURNFALL' | "$GANG" send --to 1 --from tester --stdin >/dev/null 2>&1; then
  pass "an expired bracket over a provably empty box does not veto delivery"
else
  pass_rc=$?
  fail "an expired bracket over a provably empty box does not veto delivery" \
    "send refused with rc $pass_rc"
fi
contains "and the delivery actually landed" "$(pane 1)" "MARK_TURNFALL"
equal "delivery leaves the turn bracket to its native owner, byte-identical" \
  "$stale_bracket" "$(tmux show-options -wqv -t "$(window_id 1)" @gl_turn)"
tmux set-option -w -t "$(window_id 1)" @gl_turn "open $(( $(date +%s) - 400 ))"
tmux send-keys -l -t "$(window_id 1)" 'half a draft'
if veto_draft="$(printf 'MARK_NODRAFT' | "$GANG" send --to 1 --from tester --stdin 2>&1)"; then
  fail "an expired bracket over a drafted box still refuses" "send succeeded"
else
  pass "an expired bracket over a drafted box still refuses"
fi
contains "the refusal names the expired witness" \
  "$veto_draft" "no usable busy witness"
contains "with the bracket's own reason" "$veto_draft" "turn-bracket bound reached"
contains "and the box state, not a mid-turn claim" \
  "$veto_draft" "not provably empty"
tmux send-keys -t "$(window_id 1)" C-u
tmux set-option -w -t "$(window_id 1)" @gl_turn "open $(date +%s)"
if fresh_veto="$(printf 'MARK_FRESH' | "$GANG" send --to 1 --from tester --stdin 2>&1)"; then
  fail "a fresh open bracket still refuses mid-turn" "send succeeded"
else
  pass "a fresh open bracket still refuses mid-turn"
fi
contains "with the mid-turn refusal, not the expired one" \
  "$fresh_veto" "not safely reachable mid-turn"
excludes "the refused fresh-bracket body never landed" "$(pane 1)" "MARK_FRESH"
tmux set-option -uw -t "$(window_id 1)" @gl_turn

# The box-vanishes backstop: occupied's read sees a composer, then the box
# disappears before the expired fall-through's own read — one readable look,
# then nothing. The refusal must name the expired witness and the unreadable
# box rather than claim mid-turn work.
cat > "$RUN_ROOT/profiles/vanish.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_vanish_real="\$(declare -f profile_input)"
eval "vanish_real_input \${_gl_vanish_real#profile_input}"
profile_input() { # readable per ticket once the vanish flag is set, then not
  local n
  if [ ! -f "$RUN_ROOT/vanish-flag" ]; then vanish_real_input "\$1"; return; fi
  n="\$(cat "$RUN_ROOT/vanish-tickets" 2>/dev/null)"
  if [ "\${n:-0}" -gt 0 ]; then
    printf '%s' "\$((n - 1))" > "$RUN_ROOT/vanish-tickets"
    vanish_real_input "\$1"
    return
  fi
  return 1
}
SH
"$GANG" hitch vanish -p vanish -d /tmp >/dev/null
tmux set-option -w -t "$(window_id vanish)" @gl_turn "open $(( $(date +%s) - 400 ))"
printf '1' > "$RUN_ROOT/vanish-tickets"
touch "$RUN_ROOT/vanish-flag"
if vanish_out="$(printf 'MARK_VANISH' | "$GANG" send --to vanish --from tester --stdin 2>&1)"; then
  fail "a box that vanishes before the fall-through's read still refuses" \
    "send succeeded"
else
  pass "a box that vanishes before the fall-through's read still refuses"
fi
contains "naming the expired witness" "$vanish_out" "no usable busy witness"
contains "and the unreadable box" "$vanish_out" "cannot be read"
excludes "the refused vanished-box body never landed" \
  "$(pane vanish)" "MARK_VANISH"
"$GANG" drop vanish >/dev/null

# The classification look is taken AFTER the decision it names, so the
# obstruction can leave in the gap between them — an operator's C-u, a queue
# draining at a turn boundary. That look then reads an empty box successfully,
# which is the opposite finding from a box that cannot be read, and only one of
# the two is a harness in trouble. This profile serves the draft for exactly the
# reads a refusal takes to settle and hands the naming look an empty box after
# it; a change in that count fails this check rather than quietly retargeting
# it, because the class named is asserted exactly.
cat > "$RUN_ROOT/profiles/emptied.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_emptied_real="\$(declare -f profile_input)"
eval "emptied_real_input \${_gl_emptied_real#profile_input}"
profile_input() { # the draft per ticket once the flag is set, then an empty box
  local n
  if [ ! -f "$RUN_ROOT/emptied-flag" ]; then emptied_real_input "\$1"; return; fi
  n="\$(cat "$RUN_ROOT/emptied-tickets" 2>/dev/null)"
  if [ "\${n:-0}" -gt 0 ]; then
    printf '%s' "\$((n - 1))" > "$RUN_ROOT/emptied-tickets"
    emptied_real_input "\$1"
    return
  fi
  printf ''
}
SH
"$GANG" hitch emptied -p emptied -d /tmp >/dev/null
tmux send-keys -l -t "$(window_id emptied)" 'MARK_LEAVING'
# Six: the box reads a refused send takes to settle — the busy verdict's
# composer and emptiness pair, the settled-composer pair, the parked-queue
# preflight, and the emptiness read that refuses. The seventh is the naming
# look, and it gets an empty box.
printf '6' > "$RUN_ROOT/emptied-tickets"
touch "$RUN_ROOT/emptied-flag"
if emptied_out="$(printf 'MARK_EMPTIED' |
  "$GANG" send --to emptied --from tester --stdin 2>&1)"; then
  fail "a box emptying under the refusal is still a refusal" "send succeeded"
else
  pass "a box emptying under the refusal is still a refusal"
fi
contains "and the naming look reports the box gone, not a harness that cannot be read" \
  "$emptied_out" "[cleared:"
excludes "the refused body never landed" "$(pane emptied)" "MARK_EMPTIED"
"$GANG" drop emptied >/dev/null

# A malformed bracket is REPORTED, never repaired: the reader-path clear was
# the same erase-fresh-evidence race one call deeper — cmd_send reaches
# turn_witness through busy(), and an unset there can land on top of a fresh
# hook stamp. Status names the unreadable value, delivery falls through on
# box evidence, and the value survives both byte-identical until the hooks
# that own the bracket rewrite it.
tmux set-option -w -t "$(window_id 1)" @gl_turn "gibberish not-a-stamp"
malformed_status="$("$GANG" status 1)"
contains "a malformed bracket reads as unreadable, not busy" \
  "$malformed_status" "turn-bracket value unreadable"
equal "and status leaves the malformed value in place" \
  "gibberish not-a-stamp" \
  "$(tmux show-options -wqv -t "$(window_id 1)" @gl_turn)"
if printf 'MARK_MALFORMED' | "$GANG" send --to 1 --from tester --stdin >/dev/null 2>&1; then
  pass "a malformed bracket over a provably empty box does not veto delivery"
else
  malformed_rc=$?
  fail "a malformed bracket over a provably empty box does not veto delivery" \
    "send refused rc $malformed_rc"
fi
contains "and that delivery landed" "$(pane 1)" "MARK_MALFORMED"
equal "delivery leaves even a malformed bracket untouched, byte-identical" \
  "gibberish not-a-stamp" \
  "$(tmux show-options -wqv -t "$(window_id 1)" @gl_turn)"
tmux set-option -uw -t "$(window_id 1)" @gl_turn

# The issue-#102 shape: an Escape-interrupted turn leaves a fossil busy
# marker in the transcript — "Retrying in Ns" — matching the busy regex
# forever while the process sits idle. Frozen paint witnesses a live turn
# only while something repaints it: over an expired bracket, with no recent
# pty activity and a byte-stable pane, it is could-not-determine. The
# fixture is quiet-at-rest so the activity leg genuinely reads
# #{window_activity} — the deterministic activity inputs are the window
# bounds themselves: an enormous window makes the fresh paint "recent"
# under any load, a zero window makes every stamp old. Roster's immediate
# snapshot keeps the painted verdict by design — it cannot probe stability
# without consuming the churn wait.
cat > "$RUN_ROOT/profiles/fossil.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_BUSY_REGEX='Retrying in [0-9]+s'
GANG_QUIET_AT_REST=1
SH
"$GANG" hitch fossil -p fossil -d /tmp >/dev/null
tmux send-keys -l -t "$(window_id fossil)" \
  'echo "Retrying in 8s left by an interrupted loop"'
tmux send-keys -t "$(window_id fossil)" Enter
fossil_bracket="open $(( $(date +%s) - 400 ))"
tmux set-option -w -t "$(window_id fossil)" @gl_turn "$fossil_bracket"
# Positive control: recent pty activity preserves painted busy — the
# demotion must not fire while the pty leg credits the fresh paint.
fossil_active="$(GANG_ACTIVITY_WINDOW=100000 "$GANG" status fossil | head -1)"
equal "recent pty activity keeps the busy verdict itself, not its explanation" \
  "busy (tight tug)" "$fossil_active"
if printf 'MARK_ACTIVE' | GANG_ACTIVITY_WINDOW=100000 \
  "$GANG" send --to fossil --from tester --stdin >/dev/null 2>&1; then
  fail "recent pty activity keeps refusing delivery mid-turn" "send succeeded"
else
  pass "recent pty activity keeps refusing delivery mid-turn"
fi
excludes "the refused active-pane body never landed" \
  "$(pane fossil)" "MARK_ACTIVE"
contains "roster's immediate snapshot keeps the painted verdict" \
  "$("$GANG" roster)" "busy"
# The fossil verdict: no recent write, byte-stable pane, expired bracket.
fossil_status="$(GANG_ACTIVITY_WINDOW=0 "$GANG" status fossil)"
contains "frozen busy paint over an expired bracket reads expired, not busy" \
  "$fossil_status" "expired"
contains "naming the frozen paint beside the bracket's reason" \
  "$fossil_status" "busy paint frozen"
if printf 'MARK_FOSSIL' | GANG_ACTIVITY_WINDOW=0 \
  "$GANG" send --to fossil --from tester --stdin >/dev/null 2>&1; then
  pass "a fossil busy marker does not veto delivery to a provably empty box"
else
  fossil_rc=$?
  fail "a fossil busy marker does not veto delivery to a provably empty box" \
    "send refused rc $fossil_rc"
fi
contains "and the delivery landed under the fossil" "$(pane fossil)" "MARK_FOSSIL"
equal "the fossil's bracket is left to its native owner, byte-identical" \
  "$fossil_bracket" \
  "$(tmux show-options -wqv -t "$(window_id fossil)" @gl_turn)"
"$GANG" drop fossil >/dev/null

# The other half of the issue-#102 shape, on the path no harness reports: a turn
# ended by RAW keys in the pane. `gang interrupt` closes the fact it ended, and
# an Escape typed straight into the pane closes nothing — the bracket stays open
# and only ever gets older, so could-not-determine would be that agent's
# permanent verdict while it sits provably ready. The tiers under the expired
# event decide it instead. Time is an input here, never a wait: the bracket's
# age and the activity window are injected.
cat > "$RUN_ROOT/profiles/abandoned.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_BUSY_REGEX='Retrying in [0-9]+s'
GANG_QUIET_AT_REST=1
SH
"$GANG" hitch abandoned -p abandoned -d /tmp >/dev/null
abandoned_bracket="open $(( $(date +%s) - 400 ))"
tmux set-option -w -t "$(window_id abandoned)" @gl_turn "$abandoned_bracket"
# The guard the decay must not stomp, asserted first: an UNEXPIRED bracket is
# fresh owned state and outranks every quiet tier under it. Nothing about a
# still-bounded turn changes, however ready the pane looks.
equal "an unexpired bracket over the same quiet box is still a live turn" \
  "busy (tight tug)" \
  "$(GANG_TURN_LIMIT=100000 GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | head -1)"
equal "an abandoned turn decays to idle once its bracket expires" \
  "idle (slack tug)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | head -1)"
contains "roster's snapshot decays with it — reading the box costs no churn wait" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^abandoned ')" "idle (slack tug)"
# Not a free pass over anything the box refutes: a draft sitting in the composer
# is the state the busy verdict exists to protect, and it keeps the answer
# could-not-determine on both readings.
tmux send-keys -l -t "$(window_id abandoned)" 'half a thought'
equal "a drafted box refuses the decay" \
  "expired (turn-bracket bound reached)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | head -1)"
contains "roster's snapshot refuses it on the same evidence" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^abandoned ')" "expired"
tmux send-keys -t "$(window_id abandoned)" C-u
equal "clearing the draft restores the decayed verdict" \
  "idle (slack tug)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | head -1)"
# Quiet must be MEASURED, never assumed. Hold the activity-only bound open past
# its limit with the pty credited as recent: the activity tier then reports
# could-not-determine, and an unknown tier cannot witness readiness.
tmux set-option -w -t "$(window_id abandoned)" @gl_activity_only_since \
  "$(( $(date +%s) - 400 ))"
equal "an unmeasurable pty keeps the answer could-not-determine" \
  "expired (turn-bracket bound reached)" \
  "$(GANG_ACTIVITY_WINDOW=100000 "$GANG" status abandoned | head -1)"
tmux set-option -uw -t "$(window_id abandoned)" @gl_activity_only_since
# The decay widens nothing: this send already landed through the
# could-not-determine fall-through, and the bracket is still not a reader's to
# write.
if printf 'MARK_DECAYED' | GANG_ACTIVITY_WINDOW=0 \
  "$GANG" send --to abandoned --from tester --stdin >/dev/null 2>&1; then
  pass "delivery to a decayed agent lands as an ordinary idle delivery"
else
  abandoned_rc=$?
  fail "delivery to a decayed agent lands as an ordinary idle delivery" \
    "send refused rc $abandoned_rc"
fi
contains "and that delivery landed" "$(pane abandoned)" "MARK_DECAYED"
equal "the decayed verdict leaves the bracket to its native owner, byte-identical" \
  "$abandoned_bracket" \
  "$(tmux show-options -wqv -t "$(window_id abandoned)" @gl_turn)"
"$GANG" drop abandoned >/dev/null

# The quiet leg must be measured, and a profile that does not declare
# quiet-at-rest measures nothing: its harness writes to the pty constantly at
# rest, so the activity tier reports inactive by abstention rather than by
# observation. Spending that as the positive evidence a decay requires would
# decay every abandoned turn on a harness gang cannot hear.
"$GANG" hitch assumed -p bash -d /tmp >/dev/null
tmux set-option -w -t "$(window_id assumed)" @gl_turn "open $(( $(date +%s) - 400 ))"
equal "a profile that never measures the pty cannot witness the quiet a decay needs" \
  "expired (turn-bracket bound reached)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status assumed | head -1)"
"$GANG" drop assumed >/dev/null

# Each tier read is a separate moment, and a decay assembled across them can be
# a state no single instant held: the harness resuming between the quiet
# reading and the box reading would have its still-open turn discarded on
# evidence that had already gone stale. This profile writes to its own pty
# whenever gang reads its box — the harness moving underneath the decision,
# made deterministic — and the decay must refuse rather than report idle.
{ printf '. %s\nMOVING_ON=%s\n' "$ROOT/profiles/bash.sh" "$RUN_ROOT/moving.on"; cat <<'SH'
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_QUIET_AT_REST=1
eval "$(declare -f profile_input | sed '1s/profile_input/moving_read/')"
profile_input() { # $1 = tmux target; reads the box, then writes below it
  local out rc=0
  out="$(moving_read "$1")" || rc=$?
  [ ! -e "$MOVING_ON" ] \
    || printf '\ntick\n' > "$(tmux display-message -p -t "$1" '#{pane_tty}')" 2>/dev/null
  printf '%s' "$out"
  return $rc
}
SH
} > "$RUN_ROOT/profiles/moving.sh"
"$GANG" hitch moving -p moving -d /tmp >/dev/null
tmux set-option -w -t "$(window_id moving)" @gl_turn "open $(( $(date +%s) - 400 ))"
: > "$RUN_ROOT/moving.on"   # the harness starts moving only now that it is up
equal "a pane that moves while gang is deciding refuses the decay" \
  "expired (the pane was written to while gang was deciding)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status moving | head -1)"
contains "and roster's snapshot refuses it on the same witness" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^moving ')" "expired"
"$GANG" drop moving >/dev/null

# The earlier seam, and the one the moving fixture cannot reach: the quiet stamp
# is read INSIDE recently_active, so a witness captured after that call leaves
# the reading it rests on outside the guarded interval. A write landing there is
# present in both witnesses, agrees with itself, and passes as if nothing moved —
# the pty accepted as quiet at the instant it was not. Deterministic through a
# tmux shim that writes to the pane exactly when the inactive path clears its
# activity-only bound: after the stamp was read, before any witness could be.
mkdir -p "$RUN_ROOT/bin-seam"
{ printf '#!/usr/bin/env bash\nreal=%s\n' "$(command -v tmux)"; cat <<'SH'
seen=0; target=""; prev=""
for a in "$@"; do
  [ "$a" != @gl_activity_only_since ] || seen=1
  [ "$prev" != -t ] || target="$a"
  prev="$a"
done
if [ "$seen" = 1 ] && [ -n "$target" ]; then
  "$real" "$@"; rc=$?
  tty="$("$real" display-message -p -t "$target" '#{pane_tty}' 2>/dev/null)"
  [ -z "$tty" ] || printf '\ntick-after-quiet-read\n' > "$tty" 2>/dev/null
  exit "$rc"
fi
exec "$real" "$@"
SH
} > "$RUN_ROOT/bin-seam/tmux"
chmod +x "$RUN_ROOT/bin-seam/tmux"
"$GANG" hitch seam -p abandoned -d /tmp >/dev/null
tmux set-option -w -t "$(window_id seam)" @gl_turn "open $(( $(date +%s) - 400 ))"
equal "a write between the quiet reading and the witness refuses the decay" \
  "expired (the pane was written to while gang was deciding)" \
  "$(PATH="$RUN_ROOT/bin-seam:$PATH" GANG_ACTIVITY_WINDOW=0 "$GANG" status seam | head -1)"
"$GANG" drop seam >/dev/null

# The delivery half of the same seam. Refusing the decay leaves
# could-not-determine, and send's fall-through delivers into a provably empty
# box on that verdict — correct when the verdict means stale evidence, wrong
# here, where it means gang WATCHED the screen being written to while it
# decided. A harness paints the opening of a turn with its composer still
# empty, so an empty box read out of a moving screen proves nothing and the
# paste lands in live work. The profile's busy marker is what the shim paints,
# so this is a turn starting mid-decision and not merely noise.
cat > "$RUN_ROOT/profiles/seamsend.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_BUSY_REGEX='tick-after-quiet-read'
GANG_QUIET_AT_REST=1
SH
"$GANG" hitch seamsend -p seamsend -d /tmp >/dev/null
tmux set-option -w -t "$(window_id seamsend)" @gl_turn "open $(( $(date +%s) - 400 ))"
seamsend_out=""
if seamsend_out="$(printf 'MARK_LIVE_SEND' | PATH="$RUN_ROOT/bin-seam:$PATH" \
  GANG_ACTIVITY_WINDOW=0 "$GANG" send --to seamsend --from tester --stdin 2>&1)"; then
  fail "a turn painted during the decision refuses the delivery" "send succeeded"
else
  pass "a turn painted during the decision refuses the delivery"
fi
contains "naming the moving screen rather than a stale witness" \
  "$seamsend_out" "moving screen"
excludes "and nothing was typed into it" "$(pane seamsend)" "MARK_LIVE_SEND"
equal "so no paste is left staged there either" "" \
  "$(tmux show-options -wqv -t "$(window_id seamsend)" @gl_staged)"
"$GANG" drop seamsend >/dev/null

# A profile that declares no input reader has no box for gang to measure:
# landing_zone falls back to the whole pane, which is never empty, so the
# expired-witness refusal fires on a transcript rather than on a composer.
# Calling that "its input box" blames a draft nobody wrote — the class says
# what gang actually read.
cat > "$RUN_ROOT/profiles/noreader.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
unset -f profile_input
SH
"$GANG" hitch noreader -p noreader -d /tmp >/dev/null
tmux set-option -w -t "$(window_id noreader)" @gl_turn \
  "open $(( $(date +%s) - 400 ))"
if noreader_out="$(printf 'MARK_NOREADER' | GANG_ACTIVITY_WINDOW=0 \
  "$GANG" send --to noreader --from tester --stdin 2>&1)"; then
  fail "an expired witness over a profile with no input reader refuses" \
    "send succeeded"
else
  pass "an expired witness over a profile with no input reader refuses"
fi
contains "and names the whole pane it actually measured" \
  "$noreader_out" "whole-pane:"
excludes "the refused body never landed" "$(pane noreader)" "MARK_NOREADER"
"$GANG" drop noreader >/dev/null

# Positive control for the stability leg: a churning pane preserves painted
# busy even with the activity credit forced off. The verdict is asserted
# EXACTLY: the broken state reads "expired (busy paint frozen…)", whose
# explanation contains the word busy, so a substring check would false-green
# on the feature's own text. Two determinism traps solved here: the tick
# lines are UNIQUE (a periodic screen scrolls into byte-identical captures
# and genuinely reads stable), and the stability probe's wait runs under a
# fake clock that returns once the pane has actually repainted — the wait
# becomes an event barrier on the change real time would have delivered.
# The ticker paints a ❯ line so its composer reads provably empty — if the
# stability leg were deleted, the demotion would fire and this send would
# DELIVER, turning the send assertions red alongside the verdict.
mkdir -p "$RUN_ROOT/churn-bin"
cat > "$RUN_ROOT/churn-bin/sleep" <<'SH'
#!/bin/sh
# Fake clock for the churn probe: return once the pane has repainted.
[ -n "$CHURN_PANE" ] || exit 0
base="$(tmux capture-pane -pJ -t "$CHURN_PANE" 2>/dev/null)" || exit 0
i=0
while [ "$i" -lt 200 ]; do
  now="$(tmux capture-pane -pJ -t "$CHURN_PANE" 2>/dev/null)" || exit 0
  [ "$now" = "$base" ] || exit 0
  i=$((i + 1))
done
exit 0
SH
chmod +x "$RUN_ROOT/churn-bin/sleep"
cat > "$RUN_ROOT/profiles/ticker.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'i=0; while :; do i=\\\$((i + 1)); echo Retrying in 9s tick \\\$i; echo \"❯ \"; tmux wait-for -S \"gltick-\\\$GANG_SESSION\"; done' fixture"
GANG_BUSY_REGEX='Retrying in [0-9]+s'
GANG_QUIET_AT_REST=1
SH
if GANG_BOOT_TIMEOUT=0 "$GANG" hitch ticker -p ticker -d /tmp >/dev/null 2>&1; then
  fail "an always-churning ticker cannot complete a hitch" "hitch reported success"
else
  pass "an always-churning ticker cannot complete a hitch"
fi
tmux set-option -w -t "$(window_id ticker)" @gl_turn "open $(( $(date +%s) - 400 ))"
# Event barrier, not a poll: every ticker iteration signals, so waiting for
# one guarantees at least one full paint is on screen before any verdict.
tmux wait-for "gltick-$GANG_SESSION"
CHURN_PANE="$(tmux list-panes -t "$(window_id ticker)" -F '#{pane_id}')"
equal "a churning pane keeps the busy verdict itself, not its explanation" \
  "busy (tight tug)" \
  "$(CHURN_PANE="$CHURN_PANE" PATH="$RUN_ROOT/churn-bin:$PATH" \
     GANG_ACTIVITY_WINDOW=0 "$GANG" status ticker | head -1)"
if ticker_out="$(printf 'MARK_TICKER' | CHURN_PANE="$CHURN_PANE" \
  PATH="$RUN_ROOT/churn-bin:$PATH" GANG_ACTIVITY_WINDOW=0 \
  "$GANG" send --to ticker --from tester --stdin 2>&1)"; then
  fail "a churning pane keeps refusing delivery mid-turn" "send succeeded"
else
  pass "a churning pane keeps refusing delivery mid-turn"
fi
contains "with the mid-turn refusal" "$ticker_out" "not safely reachable mid-turn"
excludes "the refused churning-pane body never landed" \
  "$(pane ticker)" "MARK_TICKER"
"$GANG" drop ticker >/dev/null

# An expired bracket over a box that cannot be read at all is refused by the
# occupancy guard upstream of the busy witness — no readable composer on a
# settled screen means a UI of unknown authority owns input, and that refusal
# fires before the bracket is ever weighed. The in-branch unreadable refusal
# stays as a backstop for a box that vanishes between those two reads.
cat > "$RUN_ROOT/profiles/blindbox.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_blind_real="\$(declare -f profile_input)"
eval "blind_real_input \${_gl_blind_real#profile_input}"
profile_input() {
  [ ! -f "$RUN_ROOT/blindbox-flag" ] || return 1
  blind_real_input "\$1"
}
SH
"$GANG" hitch blindbox -p blindbox -d /tmp >/dev/null
tmux set-option -w -t "$(window_id blindbox)" @gl_turn "open $(( $(date +%s) - 400 ))"
touch "$RUN_ROOT/blindbox-flag"
if blind_out="$(printf 'MARK_BLIND' | "$GANG" send --to blindbox --from tester --stdin 2>&1)"; then
  fail "an expired bracket over an unreadable box still refuses" "send succeeded"
else
  pass "an expired bracket over an unreadable box still refuses"
fi
contains "as occupancy of unknown authority" "$blind_out" "authority unknown"
"$GANG" drop blindbox >/dev/null

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
self_busy="$RUN_ROOT/self-compact-busy"
cat > "$RUN_ROOT/profiles/deferred.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_COMPACT_CMD="printf SELF_COMPACT; tmux wait-for -S $self_executed"
GANG_SELF_COMPACT=deferred
GANG_BUSY_REGEX='BUSY_DEFERRED'
_gl_self_input="\$(declare -f profile_input)"
eval "self_real_input \${_gl_self_input#profile_input}"
profile_input() {
  [ ! -e "$self_busy" ] || { printf ''; return; }
  self_real_input "\$1"
}
SH
"$GANG" hitch selfable -p deferred -d /tmp >/dev/null
self_id="$(window_id selfable)"
self_tmux_pane="$(tmux list-panes -t "$self_id" -F '#{pane_id}')"
self_requested="test-self-compact-requested-$$"
self_release="test-self-compact-release-$$"
self_released="test-self-compact-released-$$"
printf -v self_command ': > %q; printf BUSY_DEFERRED; GANG_SESSION=%q GANG_PROFILES=%q %q compact; tmux wait-for -S %q; tmux wait-for %q; rm -f -- %q; tmux wait-for -S %q' \
  "$self_busy" "$GANG_SESSION" "$GANG_PROFILES" "$GANG" "$self_requested" \
  "$self_release" "$self_busy" "$self_released"
tmux send-keys -l -t "$self_id" "$self_command"
tmux send-keys -t "$self_id" Enter
tmux wait-for "$self_requested"
self_request="$(tmux show-options -wqv -t "$self_id" @gl_self_compact_requested)"
contains "the deferred self-compaction request is made while the pane paints busy" \
  "$(pane selfable)" "BUSY_DEFERRED"
equal "deferred self-compaction leaves the busy composer's reading empty" \
  "" "$("$GANG" composer selfable)"
contains "self-compaction records one request inside the running agent" \
  "$(pane selfable)" "self-compaction scheduled for the end of this turn"
excludes "self-compaction does not submit before Stop" \
  "$(pane selfable)" "SELF_COMPACT"
tmux wait-for -S "$self_release"
tmux wait-for "$self_released"

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

# Without the deferred declaration, the same self-call takes the direct path
# and puts the native command into the tty while the caller's turn is active.
nodeferred_busy="$RUN_ROOT/nodeferred-compact-busy"
cat > "$RUN_ROOT/profiles/nodeferred.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/profiles/bash.sh"
GANG_COMPACT_CMD="/compact"
GANG_BUSY_REGEX='BUSY_NODEFERRED_COMPACT'
_gl_nodeferred_input="\$(declare -f profile_input)"
eval "nodeferred_real_input \${_gl_nodeferred_input#profile_input}"
profile_input() {
  [ ! -e "$nodeferred_busy" ] || { printf ''; return; }
  nodeferred_real_input "\$1"
}
SH
"$GANG" hitch nodeferred -p nodeferred -d /tmp >/dev/null
nodeferred_id="$(window_id nodeferred)"
nodeferred_observed="test-nodeferred-compact-observed-$$"
nodeferred_release="test-nodeferred-compact-release-$$"
printf -v nodeferred_command ': > %q; printf BUSY_NODEFERRED_COMPACT; GANG_SESSION=%q GANG_PROFILES=%q %q compact nodeferred >/dev/null 2>&1 || :; tmux wait-for -S %q; tmux wait-for %q; rm -f -- %q' \
  "$nodeferred_busy" "$GANG_SESSION" "$GANG_PROFILES" "$GANG" \
  "$nodeferred_observed" "$nodeferred_release" "$nodeferred_busy"
tmux send-keys -l -t "$nodeferred_id" "$nodeferred_command"
tmux send-keys -t "$nodeferred_id" Enter
tmux wait-for "$nodeferred_observed"
contains "without deferral the same self-call types the compact command mid-turn" \
  "$(pane nodeferred)" "/compact"
equal "the undeferred path stamps no pending self-compaction request" "" \
  "$(tmux show-options -wqv -t "$nodeferred_id" @gl_self_compact_requested)"
tmux wait-for -S "$nodeferred_release"
"$GANG" drop nodeferred >/dev/null

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

# The pre-push gate must run git-aware lint from the pushed tree even when Git
# gives the hook a GIT_DIR pointing at the main repository. A staged-only file
# makes a leaked main index distinguishable from the detached worktree's index.
hook_repo="$RUN_ROOT/pre-push-repo"
hook_probe="$RUN_ROOT/pre-push-probe"
mkdir -p "$hook_repo/.githooks" "$hook_repo/tools" "$hook_repo/test" "$hook_probe"
cp "$ROOT/.githooks/pre-push" "$ROOT/.githooks/commit-msg" \
  "$hook_repo/.githooks/"
cp "$ROOT/tools/pii-scan" "$hook_repo/tools/"
chmod +x "$hook_repo/.githooks/pre-push" "$hook_repo/.githooks/commit-msg" \
  "$hook_repo/tools/pii-scan"
cat > "$hook_repo/test/lint.sh" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu
: "${PROBE_DIR:?}"
git ls-files >/dev/null
git rev-parse --absolute-git-dir > "$PROBE_DIR/gitdir"
git ls-files main-index-only > "$PROBE_DIR/index"
SH
cat > "$hook_repo/test/integration.sh" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
exit 0
SH
chmod +x "$hook_repo/test/lint.sh" "$hook_repo/test/integration.sh"
git -C "$hook_repo" init -q
git -C "$hook_repo" config user.name 'Gangline Test'
git -C "$hook_repo" config user.email 'gangline-test@example.invalid'
git -C "$hook_repo" add .
git -C "$hook_repo" commit -q -m 'test: seed hook worktree'
hook_sha="$(git -C "$hook_repo" rev-parse HEAD)"
touch "$hook_repo/main-index-only"
git -C "$hook_repo" add main-index-only
hook_zero=0000000000000000000000000000000000000000
if hook_out="$({
  cd "$hook_repo"
  printf 'refs/heads/main %s refs/heads/main %s\n' "$hook_sha" "$hook_zero" |
    env GIT_DIR="$hook_repo/.git" PROBE_DIR="$hook_probe" \
      ./.githooks/pre-push
} 2>&1)"; then
  pass "pre-push lints the pushed commit under Git's hook environment"
else
  fail "pre-push lints the pushed commit under Git's hook environment" "$hook_out"
fi
hook_gitdir="$(<"$hook_probe/gitdir")"
case "$hook_gitdir" in
  "$hook_repo/.git/worktrees/"*)
    pass "pre-push lint reads a detached-worktree git directory" ;;
  *) fail "pre-push lint reads a detached-worktree git directory" "$hook_gitdir" ;;
esac
if [ "$hook_gitdir" != "$hook_repo/.git" ]; then
  pass "pre-push lint does not read the main repository git directory"
else
  fail "pre-push lint does not read the main repository git directory" "$hook_gitdir"
fi
equal "pre-push lint does not read the main repository index" \
  "" "$(<"$hook_probe/index")"

"$GANG" down >/dev/null
if tmux has-session -t "=$GANG_SESSION" 2>/dev/null; then
  fail "down removes the exact test session" "session still exists"
else
  pass "down removes the exact test session"
fi
[ ! -d "$lingering_spool" ] \
  && pass "and takes the spool of every window in it" \
  || fail "and takes the spool of every window in it" "$lingering_spool survived"

printf '\n%s checks in %ss\n' "$checks" "$SECONDS"
[ "$fails" -eq 0 ]
