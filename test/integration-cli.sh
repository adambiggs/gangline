# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Dispatcher surface: help coverage, arity, user configuration, the shipped collar declarations, and the hook wiring each harness receives at launch.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# Help coverage is derived from the dispatcher, so missing help cannot hide by
# also being absent from a hand-maintained help inventory. The exclusions are
# deliberate non-operator routes: the native callback, the hitch alias, the
# announced deprecated command aliases, and help's own dispatcher arms.
dispatch_commands="$({
  sed -n '/^case "$cmd" in$/,/^esac$/p' "$GANG" |
    awk '
      /^  [^*].*\)/ {
        arm=$1; sub(/\).*/, "", arm)
        n=split(arm, names, "|")
        for (i=1; i<=n; i++) print names[i]
      }
    '
} | awk '$0 != "hook" && $0 != "-h" && $0 != "--help" && $0 != "help"' | sort -u)"
bare_error_commands="hitch adopt send flush mail interrupt compact context usage limits wait-limit status capture composer whoami drop down"
meaningful_bare_commands="up roster attach collars roles config curfew notify"
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
contains "top-level help names the collar command" "$top_help" "collars"
contains "top-level help names the curfew command" "$top_help" "curfew"
excludes "top-level help omits the removed profiles name" "$top_help" "profiles"
excludes "top-level help omits the removed cutoff name" "$top_help" "cutoff"
contains "gang collars help prints the new synopsis" \
  "$("$GANG" collars --help)" "gang collars"
contains "gang curfew help prints the new synopsis" \
  "$("$GANG" curfew --help)" "gang curfew"
contains "gang roster help names its scripting mode" \
  "$("$GANG" roster --help)" "--porcelain"
equal "every dispatched operator command has a bare classification" \
  "$dispatch_commands" "$classified_commands"

# ARITY IS REFUSED, NOT DISCARDED. A command that quietly drops a word it never
# consumes has told the operator that word was understood, and the reading they
# get back is of something they did not ask for. Every row is one argument past
# what its command accepts, and every expectation is that command's own
# refusal: a command with no arity check runs its ordinary path and fails, if
# it fails at all, saying something else entirely, so no row can pass on an
# unrelated error. The names are compared against the dispatcher, so a command
# added without an arity refusal cannot ship quietly.
#
# NOTHING HERE MAY REACH A SIDE EFFECT UNDER THE REGRESSION IT WOULD CATCH. For
# most rows a nonexistent agent is the containment: `drop ghost STRAY` cannot
# become a drop, because with its check removed it reports no such agent. For
# hitch and up that reasoning inverts — a name that does not exist is what they
# CREATE, so an unknown word discarded by hitch's parser would launch a real
# ghost agent here, before the suite's own fixture team exists, and up would go
# on to attach to it. Measured: against a hitch whose unknown-argument arm
# discards, the uncontained row launched an agent and delivered its contract.
#
# Those two rows therefore carry two containments, and only one of them is
# independent. `-d` is argv, read by the very parser whose regression this
# probes: it stops the arm-level mutant that was measured, but a refactor that
# stops recognising -d and discards what it does not know drops the containment
# token along with the stray word, leaving dir at its default. Measured too —
# that mutant launched an agent through the -d containment alone. GANG_COLLAR is
# the one that does not depend on the parser: it is read from the environment
# before the loop runs, cannot be dropped by it, and hitch resolves the collar
# before any window opens. Under the same mutant it refused, naming the
# environment. Two separate regressions must now land before this test can
# create anything. Do not simplify either away.
#
# gang hook is not in this table and is not exempt from the rule; it is covered
# below, where its own contract makes recording the argument the refusal and
# dying the wrong answer. gang up delegates its arguments to hitch and its
# refusal says so.
# A containment is only a containment while it names nothing that exists, and one
# that quietly stopped containing would read exactly like one never needed.
arity_absent_dir="$RUN_ROOT/nonexistent/gangline-arity-probe"
arity_absent_collar="gangline-arity-probe-absent-collar"
[ ! -e "$arity_absent_dir" ] \
  && pass "the hitch and up probes name a working directory that does not exist" \
  || fail "the hitch and up probes name a working directory that does not exist" \
    "$arity_absent_dir exists"
# Read the inventory rather than driving hitch at it. A lifecycle command cannot
# test its own precondition: if an operator's GANG_COLLARS did expose a collar
# under this name — which is the case that voids the containment below, and the
# only case worth checking — a hitch would OPEN that agent and leave the window
# behind while recording the failure. gang collars enumerates the same
# directories collar_file resolves through, and hides only the bash stand-in,
# so it answers this question without acting on it.
# One collar per line, and resolution is by exact name: a substring reading
# would call the containment broken because some LONGER name contains it, which
# is a red the containment does not deserve.
# The read and the match are separate statements on purpose. Piped together
# under `set -o pipefail` they share one status, and an inventory that could not
# be read is nonzero exactly like a name that is not in it — so the arm meaning
# "absent" would also catch "never looked", and record a verified absence from a
# fixture that produced no value. An unreadable inventory gets its own arm and
# its own red: unknown is not this containment's precondition.
if arity_collar_listing="$("$GANG" collars)"; then
  if printf '%s\n' "$arity_collar_listing" \
    | grep -qxF -- "$arity_absent_collar"; then
    fail "and a collar name that resolves to nothing" \
      "$arity_absent_collar is in the collar inventory"
  else
    pass "and a collar name that resolves to nothing"
  fi
else
  fail "and a collar name that resolves to nothing" \
    "gang collars could not be read, so whether $arity_absent_collar resolves is unknown"
fi
arity_probes=(
  "up|ghost -d $arity_absent_dir STRAY|hitch: unknown argument 'STRAY'|GANG_COLLAR=$arity_absent_collar"
  "hitch|ghost -d $arity_absent_dir STRAY|hitch: unknown argument 'STRAY'|GANG_COLLAR=$arity_absent_collar"
  "adopt|ghost STRAY|adopt: unknown argument 'STRAY'"
  "send|--to ghost --stdin STRAY|send: unknown argument 'STRAY'"
  "flush|ghost STRAY|flush: unexpected argument 'STRAY'"
  "mail|ghost STRAY|mail: unexpected argument 'STRAY'"
  "interrupt|ghost STRAY|interrupt: unexpected argument 'STRAY'"
  "compact|ghost STRAY|compact: unexpected argument 'STRAY'"
  "context|ghost STRAY|context: unexpected argument 'STRAY'"
  "usage|ghost STRAY|usage: unexpected argument 'STRAY'"
  "limits|ghost STRAY|limits: unexpected argument 'STRAY'"
  "wait-limit|ghost STRAY|wait-limit: unknown argument 'STRAY'"
  "notify|ghost STRAY|notify: unexpected argument 'STRAY'"
  "curfew|30m STRAY|curfew: unexpected argument 'STRAY'"
  "status|ghost STRAY|status: unexpected argument 'STRAY'"
  "capture|ghost 5 STRAY|capture: unexpected argument 'STRAY'"
  "composer|ghost STRAY|composer: unexpected argument 'STRAY'"
  "whoami|STRAY|whoami: takes no arguments"
  "roster|STRAY|roster: expected no arguments or --porcelain"
  "attach|STRAY|attach: takes no arguments"
  "drop|ghost STRAY|drop: unexpected argument 'STRAY'"
  "down|ghost STRAY|down: unexpected argument 'STRAY'"
  "collars|STRAY|collars: takes no arguments"
  "roles|STRAY|roles: takes no arguments"
  "config|STRAY|config: takes no arguments"
)
arity_probe_commands="$(printf '%s\n' "${arity_probes[@]}" | cut -d'|' -f1 | sort -u)"
equal "every dispatched operator command refuses arity it cannot consume" \
  "$dispatch_commands" "$arity_probe_commands"
for arity_probe in "${arity_probes[@]}"; do
  arity_cmd="${arity_probe%%|*}"
  arity_rest="${arity_probe#*|}"
  arity_argv="${arity_rest%%|*}"
  arity_rest="${arity_rest#*|}"
  arity_expected="${arity_rest%%|*}"
  arity_env=""
  [ "$arity_rest" = "$arity_expected" ] || arity_env="${arity_rest#*|}"
  # shellcheck disable=SC2086 # probe argv and env are deliberately word-split
  refuses "gang $arity_cmd refuses a stray argument" \
    "$arity_expected" env $arity_env "$GANG" "$arity_cmd" $arity_argv
done
# help is a dispatcher arm rather than a command function, and it discarded
# everything past the page it was asked for.
refuses "gang help refuses a stray argument" \
  "help: unexpected argument 'STRAY'" "$GANG" help status STRAY

# A LEADING FLAG IS STILL AN ARGUMENT TO NAME. interrupt reads a leading flag as
# an omitted name, so its options are parsed before self is resolved: from
# outside a Gangline window the old order answered a malformed option with the
# synopsis and the not-an-agent line, naming everything except the word that was
# wrong. Both probes run outside any agent pane, which is where that order was
# visible.
refuses "gang interrupt names a malformed leading option" \
  "interrupt: unexpected argument '--STRAY'" "$GANG" interrupt --STRAY
refuses "gang interrupt names a leading option that lost its value" \
  "interrupt: -m needs a value" "$GANG" interrupt -m

# THE ONE COMMAND THAT RECORDS INSTEAD OF DYING — here only as far as stderr,
# from a caller whose pane gang cannot identify. That is a real case, because a
# hook can fire outside a registered pane, and it is ALL this reading proves:
# stderr survives a regression that goes on to process the event, so declining,
# stamping and leaving the turn fact alone are pinned beside the other hook
# events instead, where a fixture window exists to witness them. The payload is
# well-formed on purpose: a notice that needed a broken payload would prove
# nothing about argv.
hook_argv_out="$(printf '{"hook_event_name":"Stop"}' \
  | "$GANG" hook STRAY 2>&1)" || true
contains "gang hook names an argument it does not take" \
  "$hook_argv_out" "does not take ('STRAY')"
excludes "gang hook does not blame the payload for an argv it can read" \
  "$hook_argv_out" "not readable JSON"
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
  command_wide="$(printf '%s\n' "$command_help" \
    | grep -v '^gang: WARNING: executing dirty ' | help_width_failure)"
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

# Public collar surface: the bash fixture remains test-only.
collars="$(GANG_TEST_COLLARS='' "$GANG" collars | tr '\n' ' ')"
equal "the public collar list is the supported harness set" \
  "claude-code codex opencode pi " "$collars"
# The 1.x compatibility layer is gone in 2.0, so every pre-rename spelling is
# refused as unknown rather than accepted with an announcement. These assertions
# replace the ones that proved the aliases worked: the removal is the decision
# now, and an alias quietly coming back is what they exist to catch.
refuses "the removed profiles command name is unknown" \
  "unknown command 'profiles'" "$GANG" profiles
refuses "the removed cutoff command name is unknown" \
  "unknown command 'cutoff'" "$GANG" cutoff
refuses "the removed spawn alias is unknown" \
  "unknown command 'spawn'" "$GANG" spawn probe -c bash
refuses "the removed --profile flag is unknown" \
  "hitch: unknown argument '--profile'" "$GANG" hitch probe --profile bash
refuses "the removed send --spool flag is unknown" \
  "send: unknown argument '--spool'" "$GANG" send --to probe --spool --stdin
env -u GANG_TEST_COLLARS GANG_TEST_PROFILES=1 "$GANG" collars \
  > "$RUN_ROOT/test-profiles-alias.out" 2> "$RUN_ROOT/test-profiles-alias.err"
excludes "the removed suite switch alias exposes no test fixture" \
  "$(<"$RUN_ROOT/test-profiles-alias.out")" "bash"
equal "and it announces nothing, because it is not read at all" "" \
  "$(<"$RUN_ROOT/test-profiles-alias.err")"

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
printf '%s\n' 'GANG_SESSION=file-report' 'GANG_COLLAR=codex' \
  > "$CONFIG_CASES/report/config"
config_report="$(GANG_CONFIG_DIR="$CONFIG_CASES/report" \
  GANG_SESSION=env-report "$GANG" config)"
contains "gang config attributes an environment override to both layers" \
  "$config_report" \
  $'GANG_SESSION=env-report\tenvironment (overriding config line 1)'
contains "gang config attributes a file-layer value to its line" \
  "$config_report" \
  $'GANG_COLLAR=codex\t'"$CONFIG_CASES/report/config line 2"
contains "gang config attributes an untouched built-in value to the default" \
  "$config_report" $'GANG_TURN_LIMIT=300\tdefault'
excludes "gang config no longer publishes an occupancy timer" \
  "$config_report" "GANG_OCCUPIED_LIMIT="
for environment_only in GANG_ACTIVITY_LIMIT GANG_CLEAR_PRESSES; do
  excludes "gang config keeps $environment_only outside persistent config" \
    "$config_report" "$environment_only="
done

# 2.0 removed the pre-rename config spellings, so there is no second name for
# one setting to normalize or conflict with. In a config file the old spelling
# is an unknown key and refuses; in the environment it is a variable Gangline
# does not read, and the current name answers alone.
config_alias_root="$CONFIG_CASES/aliases"
mkdir -p "$config_alias_root/file" "$config_alias_root/env"
printf '%s\n' 'GANG_PROFILE=bash' > "$config_alias_root/file/config"
refuses "a config file naming a removed key refuses as unknown" \
  "has unknown key GANG_PROFILE" \
  env -u GANG_COLLAR -u GANG_PROFILE GANG_CONFIG_DIR="$config_alias_root/file" \
    "$GANG" config
printf '%s\n' 'GANG_PROFILES=/tmp' > "$config_alias_root/file/config"
refuses "and the plural spelling refuses the same way" \
  "has unknown key GANG_PROFILES" \
  env -u GANG_COLLARS -u GANG_PROFILES GANG_CONFIG_DIR="$config_alias_root/file" \
    "$GANG" config
removed_env_out="$(env -u GANG_COLLAR -u GANG_PROFILE GANG_COLLAR=bash \
  GANG_PROFILE=codex GANG_CONFIG_DIR="$config_alias_root/env" \
  "$GANG" config 2>&1)"
contains "the removed environment spelling does not conflict with the current one" \
  "$removed_env_out" $'GANG_COLLAR=bash\t'
excludes "and it is not reported as an origin at all" \
  "$removed_env_out" "GANG_PROFILE"

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
  env GANG_CONFIG_DIR="$CONFIG_CASES/unknown" "$GANG" collars

mkdir -p "$CONFIG_CASES/collar-declaration"
printf '%s\n' 'GANG_LAUNCH=claude' > "$CONFIG_CASES/collar-declaration/config"
collar_decl_out="$(env GANG_CONFIG_DIR="$CONFIG_CASES/collar-declaration" "$GANG" collars 2>&1 || true)"
contains "a collar declaration is refused under its own name" \
  "$collar_decl_out" "GANG_LAUNCH is a collar declaration, not operator configuration"
contains "the collar-declaration refusal points at the supported escape hatch" \
  "$collar_decl_out" "point GANG_COLLARS at its directory"

mkdir -p "$CONFIG_CASES/test-switch"
printf '%s\n' 'GANG_TEST_COLLARS=1' > "$CONFIG_CASES/test-switch/config"
refuses "the suite-only collar switch cannot be persisted" \
  "GANG_TEST_COLLARS is a per-invocation switch" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/test-switch" "$GANG" collars
printf '%s\n' 'GANG_TEST_PROFILES=1' > "$CONFIG_CASES/test-switch/config"
refuses "the removed suite-switch spelling refuses as an unknown key" \
  "has unknown key GANG_TEST_PROFILES" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/test-switch" "$GANG" collars

mkdir -p "$CONFIG_CASES/bootstrap"
printf '%s\n' 'GANG_CONFIG_DIR=/tmp/elsewhere' > "$CONFIG_CASES/bootstrap/config"
refuses "the config root cannot bootstrap itself from inside the file" \
  "GANG_CONFIG_DIR bootstraps the config file being read" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/bootstrap" "$GANG" collars

mkdir -p "$CONFIG_CASES/duplicate"
printf '%s\n' 'GANG_SESSION=first' 'GANG_SESSION=second' > "$CONFIG_CASES/duplicate/config"
refuses "a duplicated config key names both lines" \
  "duplicates GANG_SESSION on lines 1 and 2" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/duplicate" "$GANG" collars

mkdir -p "$CONFIG_CASES/empty"
printf '%s\n' 'GANG_SESSION=' > "$CONFIG_CASES/empty/config"
refuses "an empty config value says to delete the line" \
  "an empty value; delete the line to take the default" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/empty" "$GANG" collars

mkdir -p "$CONFIG_CASES/nul"
printf 'GANG_SESSION=alpha\000omega\n' > "$CONFIG_CASES/nul/config"
refuses "a NUL byte is refused instead of silently disappearing" \
  "contains a NUL byte" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/nul" "$GANG" collars

mkdir -p "$CONFIG_CASES/crlf" "$CONFIG_CASES/escape"
printf 'GANG_BOOT_TIMEOUT=30\r\n' > "$CONFIG_CASES/crlf/config"
refuses "a CRLF config line is refused as control-bearing" \
  "contains control characters other than tab and newline" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/crlf" "$GANG" collars
printf 'GANG_SESSION=alpha\033omega\n' > "$CONFIG_CASES/escape/config"
refuses "a bare escape in a config value is refused" \
  "contains control characters other than tab and newline" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/escape" "$GANG" collars

mkdir -p "$CONFIG_CASES/no-final-newline"
printf 'GANG_SESSION=config-file-session   ' > "$CONFIG_CASES/no-final-newline/config"
trimmed_roster="$(env -u GANG_SESSION GANG_CONFIG_DIR="$CONFIG_CASES/no-final-newline" "$GANG" roster)"
contains "a newline-free final value parses after trailing blanks are stripped" \
  "$trimmed_roster" "from-file"

for root_source in GANG_CONFIG_DIR XDG_CONFIG_HOME HOME; do
  case "$root_source" in
    GANG_CONFIG_DIR)
      relative_out="$(GANG_CONFIG_DIR=relative "$GANG" collars 2>&1 || true)" ;;
    XDG_CONFIG_HOME)
      relative_out="$(env -u GANG_CONFIG_DIR XDG_CONFIG_HOME=relative "$GANG" collars 2>&1 || true)" ;;
    HOME)
      relative_out="$(env -u GANG_CONFIG_DIR -u XDG_CONFIG_HOME HOME=relative "$GANG" collars 2>&1 || true)" ;;
  esac
  contains "a relative root from $root_source is refused under that source" \
    "$relative_out" "$root_source must resolve Gangline's config root to an absolute path"
done

# Codex receives the native hooks with live consumers on fresh and resumed launches. The
# command must remain one shell word even when Gangline is installed under a path
# containing spaces. The compaction pair earns its place the same way the turn
# hooks do: @gl_turn is closed for the whole of a compaction and the turn witness
# outranks the pane, so without the bracket a compacting Codex agent reads idle
# and gang delivers into it — and Codex declares no queue evidence, so that
# delivery reports submitted for input the harness parked.
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
  local fake_root="$1" launch_name="$2" collar_file="$ROOT/collars/codex.sh"
  GANG_TEST_COLLARS='' ROOT="$fake_root" bash -c \
    'set -euo pipefail; . "$1"; printf "%s" "${!2}"' \
    fixture "$collar_file" "$launch_name"
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
    "PreCompact", "PostCompact",
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
codex_resume="$(codex_launch "$ROOT" GANG_RESUME_LAUNCH)"
contains "Codex resume declares an explicit native session slot" \
  "$codex_resume" "codex resume {{session_id}}"
excludes "Codex resume never resolves by recency" "$codex_resume" "--last"

# A QUOTE IN THE ROOT IS NOT ESCAPABLE HERE. The hook TOML is carried inside a
# single-quoted -c word, so one quote in the install path closes that word and
# the remainder of the path is shell code the new window runs under the
# operator's account. The sibling collar's answer is this collar's: decline the
# hooks, and declare nothing a hookless launch cannot deliver.
codex_hostile_sentinel="$RUN_ROOT/codex-hostile-sentinel"
codex_hostile_root="$RUN_ROOT/codex'; touch \"\$CODEX_PWNED\"; :'guard"
mkdir -p "$codex_hostile_root/bin"
printf '%s\n' '#!/bin/sh' '# SPDX-License-Identifier: Apache-2.0' \
  > "$codex_hostile_root/bin/gang"
chmod +x "$codex_hostile_root/bin/gang"
codex_hostile_launch="$(codex_launch "$codex_hostile_root" GANG_LAUNCH)"
rm -f "$codex_hostile_sentinel"
CODEX_PWNED="$codex_hostile_sentinel" \
  CODEX_OPTIONS="$RUN_ROOT/codex-hostile.options" \
  sh -c 'PATH="$1"; export PATH; eval "$2"' sh \
  "$CODEX_STUB/bin:$PATH" "$codex_hostile_launch" </dev/null >/dev/null 2>&1 || true
if [ -e "$codex_hostile_sentinel" ]; then
  fail "a quote-bearing root never becomes shell code in a Codex launch" \
    "the launch executed text taken from the install path"
else
  pass "a quote-bearing root never becomes shell code in a Codex launch"
fi
excludes "a quote-bearing root gets no Codex hooks at all" \
  "$codex_hostile_launch" "hooks."
excludes "a quote-bearing resumed root gets no Codex hooks either" \
  "$(codex_launch "$codex_hostile_root" GANG_RESUME_LAUNCH)" "hooks."
codex_hostile_collar="$ROOT/collars/codex.sh"
codex_hostile_declarations="$(GANG_TEST_COLLARS='' ROOT="$codex_hostile_root" bash -c \
  'unset GANG_STOP_HOOK GANG_SELF_COMPACT; . "$1"; printf "%s|%s" "${GANG_STOP_HOOK:-}" "${GANG_SELF_COMPACT:-}"' \
  fixture "$codex_hostile_collar")"
equal "an unhooked Codex launch claims neither Stop nor deferred compaction" \
  "|" "$codex_hostile_declarations"

claude_collar="$ROOT/collars/claude-code.sh"
claude_role_prompt_opt="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "${GANG_ROLE_PROMPT_OPT:-}"' fixture "$claude_collar")"
equal "Claude declares the system-prompt option used for role briefs" \
  "--append-system-prompt" "$claude_role_prompt_opt"
claude_harness_prompt="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "${GANG_HARNESS_PROMPT:-}"' fixture "$claude_collar")"
contains "Claude scopes its task list to the native harness session" \
  "$claude_harness_prompt" "task list is scoped to this harness session."
contains "Claude says other Gangline windows cannot read harness-local task IDs" \
  "$claude_harness_prompt" "Agents in other Gangline windows cannot read it, so do not cite its task IDs to them."
excludes "Claude does not repeat the contract's shared-file remedy" \
  "$claude_harness_prompt" "Put shared state in a file"
claude_off="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "$GANG_LAUNCH"' fixture "$claude_collar")"
excludes "disabled lights do not paint Claude context output" \
  "$claude_off" 'statusLine'
claude_on="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=100000,200000 bash -c \
  '. "$1"; printf "%s" "$GANG_LAUNCH"' fixture "$claude_collar")"
contains "enabled Claude lights wire their context source" \
  "$claude_on" 'statusLine'
claude_hook_events="$(python3 - "$claude_off" <<'PY'
import json
import sys

launch = sys.argv[1]
settings = launch.split("--settings '", 1)[1].rsplit("'", 1)[0]
print(*sorted(json.loads(settings)["hooks"]), sep=" ")
PY
)"
equal "Claude installs every native event Gangline consumes" \
  "Notification PermissionRequest PostCompact PostToolUse PreCompact Stop UserPromptSubmit" \
  "$claude_hook_events"
claude_stall_types="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "${GANG_STALL_TYPES:-}"' fixture "$claude_collar")"
equal "Claude declares only native kinds that await a person" \
  "permission_prompt idle_prompt elicitation_dialog agent_needs_input" \
  "$claude_stall_types"
claude_external_dialog="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s\n--\n%s" "$GANG_DIALOGS" "$GANG_DIALOG_LINES_external_import_trust"' \
  fixture "$claude_collar")"
# shellcheck disable=SC2034  # read in test/integration-substrate.sh
claude_external_record="${claude_external_dialog%%$'\n--\n'*}"
# shellcheck disable=SC2034  # read in test/integration-substrate.sh
claude_external_lines="${claude_external_dialog#*$'\n--\n'}"
contains "Claude names the external-import trust prompt" \
  "$claude_external_dialog" "external-import-trust"
contains "Claude fingerprints the security warning below the variable paths" \
  "$claude_external_dialog" "Only use Claude Code with files you trust."
contains "Claude's external-import record has no answerable row or key" \
  "$claude_external_dialog" "external-import-trust|^❯ [0-9]+\\. |||"
claude_resume="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "$GANG_RESUME_LAUNCH"' fixture "$claude_collar")"
contains "Claude resume declares an explicit native session slot" \
  "$claude_resume" "claude --resume {{session_id}}"
excludes "Claude resume never resolves by directory recency" \
  "$claude_resume" "--continue"
codex_dialog_block="$(env ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "$GANG_DIALOG_LINES_safety_buffering_prompt"' \
  fixture "$ROOT/collars/codex.sh")"
contains "the shipped Codex fingerprint includes the third captured option" \
  "$codex_dialog_block" "Learn more"
codex_trust_record="$(env ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s\n--\n%s" "$GANG_DIALOGS" "$GANG_DIALOG_LINES_directory_trust_prompt"' \
  fixture "$ROOT/collars/codex.sh")"
contains "the shipped Codex registry names directory trust as a known dialog" \
  "$codex_trust_record" "directory-trust-prompt"
contains "the Codex directory-trust fingerprint retains the stable question" \
  "$codex_trust_record" "Do you trust the contents of this directory?"
excludes "the variable cwd line is not part of the directory-trust fingerprint" \
  "$codex_trust_record" "> You are in"
claude_hook_declarations="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  'unset GANG_STOP_HOOK GANG_SELF_COMPACT; . "$1"; printf "%s|%s" "${GANG_STOP_HOOK:-}" "${GANG_SELF_COMPACT:-}"' \
  fixture "$claude_collar")"
equal "a hooked Claude launch declares Stop and deferred self-compaction together" \
  "1|deferred" "$claude_hook_declarations"

# A COMPOSER TALLER THAN ITS PANE keeps its opening rule and its caret and loses
# the closing rule and the status lines under it. Observed 2026-08-11 on a pane
# that shrank by 21 rows: the reader wants the last two
# full-width rules, saw one, and refused. Refusing is right — content below the
# fold cannot be read, so nothing may paste against it — but it refused with the
# status that means no box at all, so `gang send` took the "not taking input"
# leg and left the body stranded unsent in a composer nobody could see. The
# clipped box gets its own status so the refusal can name what it is.
claude_box_dir="$RUN_ROOT/claude-box"
mkdir -p "$claude_box_dir"
claude_box_rule="$(printf '─%.0s' $(seq 40))"
{ printf 'transcript %s\n' 1 2 3 4 5 6
  printf '%s\n' "$claude_box_rule"
  printf '%s\n' '❯ a pasted body that outgrew'
  printf '%s\n' '  the rows this pane had for'
  printf '%s' '  it, so the box never closes'
} > "$claude_box_dir/clipped"
{ printf 'transcript %s\n' 1 2 3 4
  printf '%s\n' "$claude_box_rule"
  printf '%s\n' 'transcript 5'
  printf '%s\n' "$claude_box_rule"
  printf '%s\n' '❯ a pasted body under a rule'
  printf '%s\n' '  the transcript drew earlier'
  printf '%s' '  so a pair of rules is on screen'
} > "$claude_box_dir/clipped-pair"
{ printf 'transcript %s\n' 1 2 3 4 5
  printf '%s\n' "$claude_box_rule"
  printf '%s\n' '❯ hello'
  printf '%s\n' "$claude_box_rule"
  printf '%s\n' '  ctx 1k/2k 1%'
  printf '%s' '  bypass permissions on'
} > "$claude_box_dir/whole"
{ printf 'transcript\n'
  printf '%s\n' '──────── child (agent A) ───────────────'
  printf '%s\n' '❯ '
  printf '%s\n' "$claude_box_rule"
  printf '%s\n' '  transcript warning'
  printf '%s\n' '  ctx 1k/2k 1%'
  printf '%s\n' '  ⏵⏵ mode · ← for agents'
  printf '\n'
  printf '%s\n' '  ◯ main'
  printf '%s' '  ● general-purpose task'
} > "$claude_box_dir/subagent"
{ printf 'plain %s\n' 1 2 3 4 5 6 7 8 9; printf '%s' 'plain 10'; } \
  > "$claude_box_dir/absent"
claude_box_session="claude-box-$$"
tmux new-session -d -s "$claude_box_session" -n clipped -x 40 -y 10 \
  "cat '$claude_box_dir/clipped'; cat"
for claude_box_case in clipped-pair whole subagent absent; do
  tmux new-window -d -t "=$claude_box_session" -n "$claude_box_case" \
    "cat '$claude_box_dir/$claude_box_case'; cat"
done
claude_box_status() { # $1 = window name -> collar_input's status against it
  local rc=0
  ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
    '. "$1"; collar_input "$2" >/dev/null' fixture "$claude_collar" \
    "=$claude_box_session:$1" || rc=$?
  printf '%s' "$rc"
}
claude_box_text() { # $1 = window name -> what the collar read there
  ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
    '. "$1"; collar_input "$2"' fixture "$claude_collar" \
    "=$claude_box_session:$1" || true
}
equal "a whole composer reads as a readable box" "0" "$(claude_box_status whole)"
contains "a whole composer reads back what was typed in it" \
  "$(claude_box_text whole)" "hello"
equal "a selected child composer is not the parent agent input" "1" \
  "$(claude_box_status subagent)"
equal "a pane with no box at all refuses as an absent box" \
  "1" "$(claude_box_status absent)"
equal "a composer clipped by a short pane refuses as clipped, not as absent" \
  "2" "$(claude_box_status clipped)"
equal "a clipped composer under an earlier transcript rule also refuses as clipped" \
  "2" "$(claude_box_status clipped-pair)"
equal "a clipped composer offers no reading for a paste to be checked against" \
  "" "$(claude_box_text clipped)"
tmux kill-session -t "=$claude_box_session"

claude_unhooked_root="$RUN_ROOT/claude'guard"
mkdir -p "$claude_unhooked_root/bin"
printf '%s\n' '#!/bin/sh' '# SPDX-License-Identifier: Apache-2.0' \
  > "$claude_unhooked_root/bin/gang"
chmod +x "$claude_unhooked_root/bin/gang"
claude_unhooked_declarations="$(ROOT="$claude_unhooked_root" GANG_CONTEXT_LIGHTS=off bash -c \
  'unset GANG_STOP_HOOK GANG_SELF_COMPACT; . "$1"; printf "%s|%s" "${GANG_STOP_HOOK:-}" "${GANG_SELF_COMPACT:-}"' \
  fixture "$claude_collar")"
equal "an unhooked Claude launch claims neither Stop nor deferred compaction" \
  "|" "$claude_unhooked_declarations"
claude_midturn="$(ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "${GANG_MIDTURN_INPUT:-}"' fixture "$claude_collar")"
# THIS EXPECTATION CHANGED, and the old one was wrong about the harness rather
# than merely cautious. It read empty — "text may not enter during a turn" —
# which was the right refusal only while a park was treated as terminal. It is
# not: measured on 2.1.229, a mid-turn Enter on this harness CANNOT submit, it
# can only queue, and the queue drains by itself at the next tool batch or turn
# boundary. So the old value denied a landing gang can watch end to end.
#
# `1` would be the other wrong answer, and worse than the old one: it means text
# enters the session directly, sending this collar down a path whose own queue
# check then fails with the body already typed and NOT spooled. `park` is the
# third answer, and the three are mutually exclusive readings of one question.
equal "Claude delivery enters through the harness's own queue" "park" "$claude_midturn"
claude_queued="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "${GANG_QUEUED_REGEX:-}"' fixture "$claude_collar")"
if [ -n "$claude_queued" ]; then
  pass "the Claude collar declares its parked-queue evidence"
else
  fail "the Claude collar declares its parked-queue evidence" \
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
  '. "$1"; printf "%s" "${GANG_EFFORT_OPT:-}"' fixture "$claude_collar")"
equal "the Claude collar spells effort as one joinable word" \
  "--effort=" "$claude_effort_opt"
claude_effort_cmd="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "${GANG_EFFORT_CMD:-}"' fixture "$claude_collar")"
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

codex_collar="$ROOT/collars/codex.sh"
codex_compact="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "$GANG_COMPACT_CMD"' fixture "$codex_collar")"
equal "the Codex collar keeps native compaction" "/compact" "$codex_compact"
codex_self_compact="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "$GANG_SELF_COMPACT"' fixture "$codex_collar")"
equal "the Codex collar defers self-compaction to its native Stop hook" \
  "deferred" "$codex_self_compact"
codex_stall_types="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
  'unset GANG_STALL_TYPES; . "$1"; printf "%s" "${GANG_STALL_TYPES:-}"' \
  fixture "$codex_collar")"
equal "Codex invents no Notification kinds its hook set cannot raise" \
  "" "$codex_stall_types"
codex_effort_opt="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "${GANG_EFFORT_OPT:-}"' fixture "$codex_collar")"
equal "the Codex collar spells effort as one joinable config option" \
  "-c model_reasoning_effort=" "$codex_effort_opt"
codex_effort_cmd="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "${GANG_EFFORT_CMD:-}"' fixture "$codex_collar")"
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
for unverified_collar in opencode pi; do
  unverified_file="$ROOT/collars/$unverified_collar.sh"
  unverified_effort="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
    '. "$1"; printf "%s" "${GANG_EFFORT_OPT:-}"' fixture "$unverified_file")"
  equal "the $unverified_collar collar declares no effort spelling until one is verified" \
    "" "$unverified_effort"
done
codex_context_fixture="$RUN_ROOT/codex-context.jsonl"
cat > "$codex_context_fixture" <<'JSONL'
{"payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":50000},"model_context_window":300000}}}
{"payload":{"type":"message","role":"assistant","content":"later non-token event"}}
{"payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":120000},"model_context_window":300000}}}
JSONL
codex_context="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
  '. "$1"; codex_context_read "$2"' fixture "$codex_collar" "$codex_context_fixture")"
equal "Codex context reads the newest native token record" \
  "120k/300k (40%)" "$codex_context"

codex_limits_fixture="$RUN_ROOT/codex-limits.jsonl"
cat > "$codex_limits_fixture" <<'JSONL'
{"timestamp":"2026-08-13T20:00:00Z","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","limit_name":null,"primary":{"used_percent":91.0,"window_minutes":300,"resets_at":1786670000},"secondary":{"used_percent":84.0,"window_minutes":10080,"resets_at":1787200000}}}}
{"timestamp":"2026-08-13T20:05:00Z","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","limit_name":null,"primary":{"used_percent":96.0,"window_minutes":300,"resets_at":1786670300},"secondary":null}}}
JSONL
codex_limits="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c '
  . "$1"
  fixture_path="$2"
  codex_session_file() { printf "%s" "$fixture_path"; }
  collar_usage_limits ignored
' fixture "$codex_collar" "$codex_limits_fixture")"
equal "Codex provider limits come from the target rollout's newest native event" \
  $'codex 5-hour\t96\t1786670300\t1786651500' \
  "$codex_limits"

codex_boolean_limits_fixture="$RUN_ROOT/codex-boolean-limits.jsonl"
cat > "$codex_boolean_limits_fixture" <<'JSONL'
{"timestamp":"2026-08-13T20:05:00Z","payload":{"rate_limits":{"limit_id":"codex","primary":{"used_percent":true,"window_minutes":300,"resets_at":1786670300},"secondary":null}}}
JSONL
if GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c '
  . "$1"
  fixture_path="$2"
  codex_session_file() { printf "%s" "$fixture_path"; }
  collar_usage_limits ignored
' fixture "$codex_collar" "$codex_boolean_limits_fixture" >/dev/null 2>&1; then
  fail "Codex refuses a boolean provider percentage" "boolean parsed as one percent"
else
  pass "Codex refuses a boolean provider percentage"
fi

claude_limits_stub="$RUN_ROOT/claude-limits-stub"
mkdir -p "$claude_limits_stub"
cat > "$claude_limits_stub/claude" <<'SH'
#!/bin/sh
cat <<'OUT'
You are currently using your subscription to power your Claude Code usage

Current session: 93% used · resets Aug 13, 4:49pm (America/Vancouver)
Current week (all models): 96% used · resets Aug 19, 10am (America/Vancouver)
Current week (Fable): 3% used · resets Aug 19, 10am (America/Vancouver)
OUT
SH
chmod +x "$claude_limits_stub/claude"
claude_limits="$(PATH="$claude_limits_stub:$PATH" GANG_TEST_COLLARS='' ROOT="$ROOT" \
  bash -c '. "$1"; collar_usage_limits ignored' fixture "$claude_collar")"
contains "Claude provider limits use the headless native usage command" \
  "$claude_limits" $'Current week (all models)\t96\t'
contains "Claude provider limits accept a reset hour with no minute field" \
  "$claude_limits" $'Current week (Fable)\t3\t'
equal "Claude throttles its heavyweight usage-light subprocess" "60" \
  "$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
    '. "$1"; printf "%s" "$GANG_USAGE_LIGHT_INTERVAL"' fixture "$claude_collar")"

mkdir -p "$RUN_ROOT/collars"
export GANG_COLLARS="$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/ctx-known.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_context() { printf '42k/200k (21%%)\n'; }
SH
cat > "$RUN_ROOT/collars/ctx-fail.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_context() { die 'fixture context unavailable'; }
SH
cat > "$RUN_ROOT/collars/ctx-none.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
unset -f collar_context
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
cat > "$RUN_ROOT/collars/usage-inline.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="env ENV='$RUN_ROOT/usage-bashrc' bash --posix"
GANG_USAGE_CMD='u'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
SH
cat > "$RUN_ROOT/collars/usage-confirm.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="env ENV='$RUN_ROOT/usage-confirm-bashrc' bash --posix"
GANG_USAGE_CMD='c'
GANG_USAGE_CONFIRM_KEY="Enter"
GANG_USAGE_RENDER="modal"
GANG_USAGE_DISMISS_KEY="Escape"
SH
cat > "$RUN_ROOT/collars/usage-modal.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_USAGE_CMD='printf "\033[H\033[2JMODAL_ONE\nMODAL_TWO\n"; IFS= read -r -n 1 _'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="modal"
GANG_USAGE_DISMISS_KEY="Escape"
SH
cat > "$RUN_ROOT/collars/usage-stuck.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_USAGE_CMD='printf "\033[H\033[2JMODAL_STUCK\n"; while :; do IFS= read -r -n 1 _; [ "$_" = q ] && break; done'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="modal"
GANG_USAGE_DISMISS_KEY="C-g"
SH
cat > "$RUN_ROOT/collars/usage-hold.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
# A usage command the harness begins and never finishes, so it has provably not
# consumed the submission when gang takes its first screen reading. Nothing here
# waits: the pane is blocked on its own input for the whole run, which is the
# losing side of a race held open by state instead of by delay.
GANG_USAGE_CMD='read -r _'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
SH
cat > "$RUN_ROOT/collars/usage-nochange.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_USAGE_CMD="clear"
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
SH
cat > "$RUN_ROOT/collars/usage-occupied.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_USAGE_CMD='printf SHOULD_NOT_RUN'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
GANG_OCCUPIED_REGEX='OCCUPIED_USAGE'
_gl_usage_occupied_input="$(declare -f collar_input)"
eval "usage_occupied_real_input ${_gl_usage_occupied_input#collar_input}"
collar_input() {
  tmux capture-pane -pJ -t "$1" | grep -q OCCUPIED_USAGE && return 1
  usage_occupied_real_input "$1"
}
SH
cat > "$RUN_ROOT/collars/usage-unknown.sh" <<'SH'
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_USAGE_CMD='printf SHOULD_NOT_RUN'
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="unknown"
GANG_USAGE_DISMISS_KEY=""
SH
