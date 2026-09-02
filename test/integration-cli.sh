# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Dispatcher surface: help coverage, arity, user configuration, the shipped collar declarations, and the hook wiring each harness receives at launch.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# Help coverage is derived from the dispatcher, so missing help cannot hide by
# also being absent from a hand-maintained help inventory. The exclusions are
# deliberate non-operator routes: the native callback, help's own dispatcher
# arms, and the retired `usage` name, whose arm exists only to name the command
# that replaced it.
dispatch_commands="$({
  sed -n '/^case "$cmd" in$/,/^esac$/p' "$GANG" |
    awk '
      /^  [^*].*\)/ {
        arm=$1; sub(/\).*/, "", arm)
        n=split(arm, names, "|")
        for (i=1; i<=n; i++) print names[i]
      }
    '
} | awk '$0 != "hook" && $0 != "reported-to-hitcher" && $0 != "__tick-worker" && $0 != "usage" && $0 != "-h" && $0 != "--help" && $0 != "help"' | sort -u)"
bare_error_commands="hitch adopt rename talk send at flush mail interrupt compact context limits wait-limit wait status explain capture composer whoami drop down"
meaningful_bare_commands="up roster attach teams alerts tick collars models roles config curfew notify upgrade"
classified_commands="$(printf '%s\n' $bare_error_commands $meaningful_bare_commands | sort -u)"

# ONE MONOTONIC READER AND ONE ELAPSED-SINCE DECISION. Callers retain their
# own operation and cleanup; this helper supplies only the clock domain and the
# inclusive duration edge shared by those callers.
clock_helper="$ROOT/libexec/gang-clock"
clock_now=""
if [ -x "$clock_helper" ] && clock_now="$("$clock_helper" now 2>/dev/null)" \
   && [[ "$clock_now" =~ ^[0-9]+$ ]]; then
  pass "the shared clock returns one monotonic nanosecond reading"
  clock_future_rc=0
  "$clock_helper" elapsed "$clock_now" 3600000000000 >/dev/null 2>&1 \
    || clock_future_rc=$?
  equal "a duration that has not elapsed returns the live not-yet verdict" \
    1 "$clock_future_rc"
  clock_elapsed_rc=0
  "$clock_helper" elapsed 0 0 >/dev/null 2>&1 || clock_elapsed_rc=$?
  equal "the elapsed-since edge is inclusive" 0 "$clock_elapsed_rc"
else
  fail "the shared clock returns one monotonic nanosecond reading" \
    "$clock_helper is absent or returned an unreadable value"
fi

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
contains "top-level help names the model command" "$top_help" "models"
contains "top-level help names the curfew command" "$top_help" "curfew"
excludes "top-level help omits the removed profiles name" "$top_help" "profiles"
excludes "top-level help omits the removed cutoff name" "$top_help" "cutoff"
# ONBOARDING IS `run gang`. No arguments used to print this same inventory to
# stderr and exit 1, which told an agent that asking where it was had been a
# mistake. It is a page of its own now, on stdout, and the inventory stays
# exactly one command further on.
welcome_rc=0
welcome_err="$RUN_ROOT/welcome.err"
welcome="$(env -u TMUX_PANE "$GANG" 2>"$welcome_err")" || welcome_rc=$?
equal "bare gang answers instead of refusing" "0" "$welcome_rc"
equal "and says nothing on stderr" "" "$(<"$welcome_err")"
equal "the welcome page fits the phone-SSH width" "" \
  "$(printf '%s\n' "$welcome" | help_width_failure)"
contains "the welcome page carries the send one-liner" \
  "$welcome" "gang send --to NAME --stdin"
source_version="$(<"$ROOT/version.txt")"
equal "source gang reports the release-owned version" \
  "gang $source_version" "$(env GANG_CONFIG_DIR="$RUN_ROOT/no-config" "$GANG" --version)"
version_install="$RUN_ROOT/version-install"
mkdir -p "$version_install/bin" "$version_install/collars"
cp "$GANG" "$version_install/bin/gang"
cp "$ROOT/collars/bash.sh" "$version_install/collars/bash.sh"
printf '%s\n' 9.8.7 > "$version_install/version.txt"
installed_version="$(env GANG_CONFIG_DIR="$RUN_ROOT/no-config" \
  "$version_install/bin/gang" --version)"
equal "an installed gang reports its adjacent release version" \
  "gang 9.8.7" "$installed_version"
contains "it warns that reading your own mail consumes it" \
  "$welcome" "consumes it"
contains "it says what a refusal means" "$welcome" "did NOT happen"
contains "and hands the inventory over to gang help" "$welcome" "gang help"
excludes "the welcome page is not the inventory" "$welcome" "start and end a team"
contains "which gang help still prints" "$("$GANG" help)" "start and end a team"
contains "gang collars help prints the new synopsis" \
  "$("$GANG" collars --help)" "gang collars"
contains "gang models help prints the discovery synopsis" \
  "$("$GANG" models --help)" "gang models"
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
if arity_collar_listing="$("$GANG" collars | cut -f1)"; then
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
  "rename|ghost renamed STRAY|rename: unexpected argument 'STRAY'"
  "talk|ghost STRAY|talk: unknown argument 'STRAY'"
  "send|--to ghost --stdin STRAY|send: unknown argument 'STRAY'"
  "flush|ghost STRAY|flush: unexpected argument 'STRAY'"
  "at|--to ghost STRAY|at: unknown argument 'STRAY' — a message body is not an argument"
  "mail|ghost STRAY|mail: unexpected argument 'STRAY'"
  "interrupt|ghost STRAY|interrupt: unexpected argument 'STRAY'"
  "compact|ghost STRAY|compact: unexpected argument 'STRAY'"
  "context|ghost STRAY|context: unexpected argument 'STRAY'"
  "limits|ghost STRAY|limits: unexpected argument 'STRAY'"
  "wait-limit|ghost STRAY|wait-limit: unknown argument 'STRAY'"
  "wait|ghost --until idle STRAY|wait: unexpected argument 'STRAY'"
  "notify|ghost STRAY|notify: unexpected argument 'STRAY'"
  "curfew|30m STRAY|curfew: unexpected argument 'STRAY'"
  "status|ghost STRAY|status: unexpected argument 'STRAY'"
  "explain|ghost STRAY|explain: unexpected argument 'STRAY'"
  "capture|ghost 5 STRAY|capture: unexpected argument 'STRAY'"
  "composer|ghost STRAY|composer: unexpected argument 'STRAY'"
  "whoami|STRAY|whoami: takes no arguments"
  "roster|STRAY|roster: expected no arguments or --porcelain"
  "attach|STRAY|attach: takes no arguments"
  "teams|STRAY|teams: takes no arguments"
  "alerts|STRAY|alerts: expected no arguments, --porcelain, or --open"
  "tick|STRAY|tick: takes no arguments"
  "drop|ghost STRAY|drop: unexpected argument 'STRAY'"
  "down|ghost STRAY|down: unexpected argument 'STRAY'"
  "collars|STRAY|collars: takes no arguments"
  "models|STRAY|models: unexpected argument 'STRAY'"
  "roles|STRAY|roles: takes no arguments"
  "config|STRAY|config: takes no arguments"
  "upgrade|STRAY|upgrade: unexpected argument 'STRAY'"
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
collars="$(GANG_TEST_COLLARS='' "$GANG" collars | cut -f1 | tr '\n' ' ')"
equal "the public collar list is the supported harness set" \
  "claude-code codex opencode pi " "$collars"
# WHICH COLLARS CAN BE RESUMED, said where -c is chosen. Both halves are needed:
# a launch line with a session slot and a collar that witnesses the id to put in
# it. Asserted per collar rather than as one blob, so a collar losing the
# capability names itself.
collar_caps="$(GANG_TEST_COLLARS='' "$GANG" collars)"
for collar_cap_row in "claude-code resume" "codex resume" \
                      "opencode resume" "pi no-resume"; do
  equal "gang collars marks ${collar_cap_row% *} ${collar_cap_row#* }" \
    "${collar_cap_row#* }" \
    "$(printf '%s\n' "$collar_caps" | awk -F '\t' -v n="${collar_cap_row% *}" \
        '$1 == n { print $2 }')"
done
# THE HALF THAT STILL RESUMES. hitch consults collar_session_id only for the
# bare form, so a collar with a resume launch and no witness relaunches onto an
# id the operator supplies. Reported as no-resume it told an operator the
# session was lost for good. The public inventory asserted above is exactly the
# shipped set, so the fixture gets a collar directory of its own rather than the
# shared one, which does not exist yet here in any case.
mkdir -p "$RUN_ROOT/id-only-collars"
cat > "$RUN_ROOT/id-only-collars/id-only.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_RESUME_LAUNCH="PS1='❯ ' bash --norc {{session_id}}"
SH
equal "a collar that relaunches onto an id it never witnesses says so" \
  "resume-id-only" \
  "$(GANG_TEST_COLLARS='' GANG_COLLARS="$RUN_ROOT/id-only-collars" "$GANG" collars \
      | awk -F '\t' '$1 == "id-only" { print $2 }')"
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

# A BODY WHERE A FLAG GOES IS THE MISTAKE THIS REFUSAL CATCHES, so it names
# where the body should have gone. A flag-shaped unknown is a different mistake
# and keeps the bare refusal.
refuses "a positional body names the way to send one" \
  "gang send --to <name> --stdin" "$GANG" send --to probe "hello there"
positional_out="$("$GANG" send --to probe --stdin -x 2>&1)" || :
excludes "while a flag-shaped unknown argument is left alone" \
  "$positional_out" "reads it from stdin"

# EVERY REFUSAL PRINTS THROUGH ONE FUNCTION, and operator bytes reach it from
# every argument parser there is. What is asserted here is that function's
# property; one parser is just the way to reach it.
# ESC IS NOT THE ONLY BYTE A TERMINAL OBEYS. 0x9b is CSI on its own, and it is
# not a control CHARACTER under this suite's UTF-8 locale — it is an invalid
# byte there, which a character class built out of the locale's own controls
# does not cover. So the argument below carries both, and what is asserted is
# that neither survives into what an operator's terminal renders.
control_out="$("$GANG" drop probe \
  "$(printf 'STRAY\033[31mPAINT\233 31mCSI\ngang: this line is not gang')" 2>&1)" || :
excludes "an operator argument emits no terminal control bytes" \
  "$control_out" "$(printf '\033')"
if printf '%s' "$control_out" | LC_ALL=C grep -q "$(printf '\233')"; then
  fail "and no raw C1 control byte either" "0x9b reached the terminal"
else
  pass "and no raw C1 control byte either"
fi
contains "while the text around it is still readable" "$control_out" "31mCSI"
contains "while its readable text still names what was refused" \
  "$control_out" "STRAY"
contains "and a pasted newline cannot forge a line of gang's own" \
  "$control_out" "gang: gang: this line is not gang"

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
# THE HARNESS IS WHAT THIS RECORDS, NOT EVERY CALLER OF THAT NAME. The collar's
# hooks preflight asks the same binary about its hook trust before launching it,
# as `codex app-server`, and passes the launch's own -c overrides through. A
# stub that recorded those too would let this assertion pass on the preflight's
# probe alone, with the harness invocation it is about never reached — the check
# would still be green while witnessing nothing.
#
# So the stub answers the probe as the harness would, with a hook list of its
# own, and records nothing for it. Answering after reading the request rather
# than before removes the race the other order carries: a stub that exits first
# closes the pipe under the caller's write, and the preflight would report a
# hook state it could not read instead of launching.
cat > "$CODEX_STUB/bin/codex" <<'SH'
#!/bin/sh
if [ "${1:-}" = "app-server" ]; then
  while IFS= read -r line; do
    case "$line" in *'"hooks/list"'*) break ;; esac
  done
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"data":[]}}'
  exit 0
fi
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

codex_hook_commands() { # $1 launch line, $2 captured -c options -> event->command JSON
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
    r'command = "((?:\\.|[^"\\])*)"(?:, time' r'out = [0-9]+)? \}\] \}\]'
)
seen = set()
commands = {}
for option in open(sys.argv[1], encoding="utf-8"):
    if not option.startswith("hooks."):
        continue
    match = shape.fullmatch(option.rstrip("\n"))
    if match is None:
        raise SystemExit(1)
    seen.add(match.group(1))
    commands[match.group(1)] = json.loads('"' + match.group(2) + '"')
if seen != events or set(commands) != events:
    raise SystemExit(1)
print(json.dumps(commands, sort_keys=True))
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
    commands="$(codex_hook_commands "$(codex_launch "$install_root" "$launch_var")" "$options")"
    command="$(printf '%s' "$commands" | python3 -c '
import json
import sys
print(json.load(sys.stdin)["UserPromptSubmit"])
')"
    stop_command="$(printf '%s' "$commands" | python3 -c '
import json
import sys
print(json.load(sys.stdin)["Stop"])
')"
    "$install_root/bin/gang" true >/dev/null 2>&1 || true
    : > "$install_root/bin/gang.args"
    sh -c "$command" </dev/null
    args="$(tr '\n' ' ' < "$install_root/bin/gang.args")"
    hook_receipts="$hook_receipts${hook_receipts:+ | }$install_name/$launch_var=$args"
    contains "the Codex Stop event routes through its report-before-idle helper" \
      "$stop_command" "codex-stop-hook.py"
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
# 2.0 removed the known-dialog registry, so no shipped collar declares one and
# the occupancy regex is the whole recognition surface.
claude_dialog_decls="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s|%s" "${GANG_DIALOGS:-}" "${GANG_DIALOG_HITCH_DIR_TRUST:-}"' \
  fixture "$claude_collar")"
equal "the Claude collar declares no known-dialog registry" "|" \
  "$claude_dialog_decls"
claude_resume="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "$GANG_RESUME_LAUNCH"' fixture "$claude_collar")"
contains "Claude resume declares an explicit native session slot" \
  "$claude_resume" "claude --resume {{session_id}}"
excludes "Claude resume never resolves by directory recency" \
  "$claude_resume" "--continue"
codex_dialog_decls="$(env ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s|%s" "${GANG_DIALOGS:-}" "${GANG_DIALOG_HITCH_DIR_TRUST:-}"' \
  fixture "$ROOT/collars/codex.sh")"
equal "the Codex collar declares no known-dialog registry either" "|" \
  "$codex_dialog_decls"
codex_occupied="$(env ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "$GANG_OCCUPIED_REGEX"' fixture "$ROOT/collars/codex.sh")"
contains "and its occupancy regex still matches the numbered rows those dialogs draw" \
  "$codex_occupied" '^› [0-9]+\. '
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
equal "a selected child composer is not the parent agent input" "4" \
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

# THE BACKGROUND-SESSIONS VIEW DRAWS A DIFFERENT COMPOSER. Typing into it does
# not reach the active conversation: it starts a new session. The rows are a
# byte-for-byte pane capture from claude-code 2.1.251, painted at the same
# geometry, so the shipped reader has to classify the native surface itself
# rather than a reconstruction of its labels.
claude_background_session="claude-background-$$"
claude_background_ready="claude-background-ready-$$"
printf -v claude_background_command \
  "tmux resize-window -x 100 -y 30; cat %q; tmux wait-for -S %q; cat" \
  "$ROOT/test/fixtures/claude-background-sessions.txt" \
  "$claude_background_ready"
tmux new-session -d -s "$claude_background_session" -n view -x 100 -y 30 \
  "$claude_background_command"
tmux wait-for "$claude_background_ready"
background_status=0
ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; collar_input "$2" >/dev/null' fixture "$claude_collar" \
  "=$claude_background_session:view" || background_status=$?
equal "the background-sessions view is not the active agent composer" \
  "6" "$background_status"
tmux kill-session -t "=$claude_background_session"

# THE PINNED AUTO-MODE FRAME NEEDS TWO STRUCTURAL QUESTIONS. The fixture is the
# right-trimmed live 2.1.239 capture; each derived pane changes only its relation
# to a real composer. No test restates the native copy the collar recognizes.
claude_nux_dir="$RUN_ROOT/claude-auto-nux"
mkdir -p "$claude_nux_dir"
claude_nux_capture="$ROOT/test/fixtures/claude-auto-mode-environment.txt"
claude_nux_title="$(sed -n '2p' "$claude_nux_capture")"
claude_nux_guide="$(sed -n '14p' "$claude_nux_capture")"
claude_nux_rule="$(printf '─%.0s' $(seq 100))"
cp "$claude_nux_capture" "$claude_nux_dir/overlay"
printf '%s\n%s\n%s\n%s\n%s\n' \
  "$claude_nux_rule" '❯' "$claude_nux_rule" '  ctx 1k/200k 1%' \
  '  bypass permissions on' >> "$claude_nux_dir/overlay"
cp "$claude_nux_capture" "$claude_nux_dir/transcript-tail"
printf '%s\n%s\n%s\n%s\n%s\n%s\n' \
  'ordinary transcript tail' "$claude_nux_rule" '❯' "$claude_nux_rule" \
  '  ctx 1k/200k 1%' '  bypass permissions on' \
  >> "$claude_nux_dir/transcript-tail"
awk 'NR == 14 { $0 = $0 " changed" } { print }' "$claude_nux_capture" \
  > "$claude_nux_dir/reworded"
printf '%s\n%s\n%s\n%s\n%s\n' \
  "$claude_nux_rule" '❯' "$claude_nux_rule" '  ctx 1k/200k 1%' \
  '  bypass permissions on' >> "$claude_nux_dir/reworded"
printf 'ordinary transcript\n  %s\n  %s\n%s\n%s\n%s\n%s\n%s\n' \
  "$claude_nux_title" "$claude_nux_guide" "$claude_nux_rule" '❯' \
  "$claude_nux_rule" '  ctx 1k/200k 1%' '  bypass permissions on' \
  > "$claude_nux_dir/prose-only"
printf 'ordinary transcript\n%s\n%s\n  %s\n  %s\n  HUMAN_DRAFT\n%s\n%s\n%s\n' \
  "$claude_nux_rule" '❯ [gang:tester]' "$claude_nux_title" \
  "$claude_nux_guide" "$claude_nux_rule" '  ctx 1k/200k 1%' \
  '  bypass permissions on' > "$claude_nux_dir/message-body"
cp "$claude_nux_capture" "$claude_nux_dir/clipped"
printf '%s\n%s\n' "$claude_nux_rule" '❯ clipped body' \
  >> "$claude_nux_dir/clipped"
# A SECOND, UNRELATED DIALOG. The rule that recognises the frame is only worth
# anything if it was never fitted to the one dialog that motivated it, so the
# world carries a model picker captured from a different claude build. It shares
# no word of its title or its guide with the NUX above; what it shares is the
# chrome. Its capture is 120 columns wide and its window is too, so this case is
# the dialog at its own geometry; the case below is the same dialog at a
# geometry that is not its own.
claude_picker_capture="$ROOT/test/fixtures/claude-model-picker.txt"
# Read out of the captures, never restated here: a test that spelled these out
# would agree with itself while the collar read something else entirely. The
# collar trims what it prints, so the expectations are trimmed the same way.
claude_picker_title="$(sed -n '2p' "$claude_picker_capture" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')"
claude_nux_title_bare="$(printf '%s' "$claude_nux_title" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')"
cp "$claude_picker_capture" "$claude_nux_dir/picker"
# EVERY OTHER PANE HERE IS THE CAPTURE AT ITS OWN WIDTH, and that is what let a
# reader that measured the band pass this whole world while failing on the same
# dialog elsewhere. A band is only ever as wide as the pane it was drawn in: put
# a 100-glyph band in an 80-column pane and it wraps, the first eighty glyphs
# leave with the row that scrolls off the top, and what a capture can still be
# joined out of is a twenty-glyph remainder of the identical dialog. This pane
# is the overlay case at a width that is not its capture's, and it is the one
# case in this file whose verdict depends on the band not being measured.
cp "$claude_nux_dir/overlay" "$claude_nux_dir/narrow"
# UPSTREAM 3.2a RECALCULATES A DETACHED NEW SESSION FROM ITS CALLING CLIENT
# after honouring -x/-y. Ubuntu backported the later fix, which is why this
# fixture kept its requested geometry on the distro build while the raw-bytes
# source build silently painted it at the caller's eighty columns. Resize from
# inside the new pane before painting: that pins the manual size on both worlds,
# and the file is still first rendered at the geometry this case claims.
claude_nux_session="claude-nux-$$"
tmux new-session -d -s "$claude_nux_session" -n overlay -x 100 -y 24 \
  "tmux resize-window -x 100 -y 24; cat '$claude_nux_dir/overlay'; cat"
for claude_nux_case in transcript-tail reworded prose-only message-body clipped; do
  tmux new-window -d -t "=$claude_nux_session" -n "$claude_nux_case" \
    "cat '$claude_nux_dir/$claude_nux_case'; cat"
done
tmux new-window -d -t "=$claude_nux_session" -n picker \
  "cat '$claude_nux_dir/picker'; cat"
tmux resize-window -t "=$claude_nux_session:picker" -x 120 -y 30
# ITS OWN SESSION, BECAUSE A RESIZE IS NOT A RENDERING. Widening a window pads
# what is already drawn, which is why the picker above can be created here and
# grown; narrowing one REFLOWS it, and the reflow rebuilds rows this case exists
# to observe unrebuilt. The pane has to be PAINTED at eighty columns, so it gets
# a session whose pane pins eighty before painting rather than a resize down
# after it.
claude_narrow_session="claude-narrow-$$"
tmux new-session -d -s "$claude_narrow_session" -n narrow -x 80 -y 24 \
  "tmux resize-window -x 80 -y 24; cat '$claude_nux_dir/narrow'; cat"
claude_nux_status() { # $1 window, $2 session -> shipped collar_input status
  local rc=0
  ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
    '. "$1"; collar_input "$2" >/dev/null' fixture "$claude_collar" \
    "=${2:-$claude_nux_session}:$1" || rc=$?
  printf '%s' "$rc"
}
claude_nux_text() { # $1 window -> shipped collar_input reading
  ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
    '. "$1"; collar_input "$2"' fixture "$claude_collar" \
    "=$claude_nux_session:$1" || true
}
equal "the captured NUX immediately before a composer hides that box" "1" \
  "$(claude_nux_status overlay)"
equal "the captured rows followed by transcript prose leave the composer readable" \
  "0" "$(claude_nux_status transcript-tail)"
# THIS EXPECTATION CHANGED, and the old one was wrong about the harness rather
# than merely strict. It required a reworded dialog to fall back to ordinary
# composer evidence, on the reasoning that recognition was pinned to exact copy
# and copy that moved could no longer be trusted. The reword then happened:
# 2.1.241 dropped the guide row this pinned, the pin stopped matching, and the
# dialog owned input above a live composer again with nothing saying so — which
# is the whole of issue #143. Recognition is keyed to the frame now, so a
# dialog whose words move is still a dialog, and that is the repair.
equal "a reworded captured dialog is still a dialog" "1" \
  "$(claude_nux_status reworded)"
equal "the two pinned prose rows without native chrome leave the composer readable" \
  "0" "$(claude_nux_status prose-only)"
equal "the pinned prose inside a message leaves its composer readable" "0" \
  "$(claude_nux_status message-body)"
contains "and a human draft beside that prose remains visible" \
  "$(claude_nux_text message-body)" "HUMAN_DRAFT"
equal "a clipped composer behind the dialog keeps its clipped verdict" "2" \
  "$(claude_nux_status clipped)"
equal "an unrelated captured dialog sharing no copy is recognised too" "1" \
  "$(claude_nux_status picker)"
# THE GUARD ON MEASURING THE BAND. Read the width off the pane rather than
# restating it: what makes this case the one it is, is that the band on screen
# is a fraction of the band in the capture.
claude_narrow_band="$(tmux capture-pane -pJ -t "=$claude_narrow_session:narrow" \
  | sed -n '1p' | tr -cd '▔' | wc -m)"
claude_captured_band="$(sed -n '1p' "$claude_nux_capture" | tr -cd '▔' | wc -m)"
# source-guard: whole-surface@190aa57aa71c: this window is a session created two lines above whose single command paints one file this case wrote, so every row on the pane has the same producer and there is no second writer a reading could be confused between; the claim is about how much of that one painting the pane width left on screen, not about who drew any row of it
equal "the narrow pane leaves only a remainder of the band on screen" "1" \
  "$([ "$claude_narrow_band" -gt 0 ] \
    && [ "$claude_narrow_band" -lt "$claude_captured_band" ] && printf 1 || printf 0)"
equal "and that remainder is still the dialog that owns the box" "1" \
  "$(claude_nux_status narrow "$claude_narrow_session")"

# NAMING IS A SEPARATE QUESTION FROM OWNERSHIP, and it is the half issue #143
# asks a failed send to answer. The reader above decides whether a box may be
# typed into; this one only supplies the title a refusal quotes, so it must
# stay silent everywhere the screen is ordinary.
claude_overlay_title() { # $1 window, $2 session -> the collar's title, or empty
  ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
    '. "$1"; collar_overlay "$2"' fixture "$claude_collar" \
    "=${2:-$claude_nux_session}:$1" || true
}
equal "the overlay reader names the dialog over a live composer" \
  "$claude_nux_title_bare" "$(claude_overlay_title overlay)"
equal "and names an unrelated dialog it was never fitted to" \
  "$claude_picker_title" "$(claude_overlay_title picker)"
equal "a reworded dialog is still named rather than going quiet" \
  "$claude_nux_title_bare" "$(claude_overlay_title reworded)"
equal "an ordinary composer has no dialog to name" "" \
  "$(claude_overlay_title transcript-tail)"
equal "and neither does the same prose carried inside a message" "" \
  "$(claude_overlay_title message-body)"
equal "nor the pinned prose without any native chrome" "" \
  "$(claude_overlay_title prose-only)"
equal "a dialog whose band is clipped by the pane is still named" \
  "$claude_nux_title_bare" \
  "$(claude_overlay_title narrow "$claude_narrow_session")"
tmux kill-session -t "=$claude_narrow_session"
tmux kill-session -t "=$claude_nux_session"

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
# `1` would still be wrong: it types directly, before attribution owns a safe
# landing. The live ruling is `steer`: commit to Gangline's spool first, then a
# free composer may accept the claimed entry as native mid-turn steering.
equal "Claude delivery attributes before native mid-turn steering" "steer" "$claude_midturn"
claude_model_aliases="$(ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "${GANG_MODEL_ALIASES:-}"' fixture "$claude_collar")"
equal "the Claude collar declares only its documented model aliases" \
  $'fable\nopus\nsonnet' "$claude_model_aliases"
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

cat > "$CLAUDE_STUB/bin/claude" <<'SH'
#!/bin/sh
case "$2" in
  opus-5)
    printf '"opus-5" is not a model this version of Claude Code recognizes\n' ;;
esac
printf 'Error: Input must be provided either through stdin or as a prompt argument when using --print\n'
exit 1
SH
claude_recognized="$(PATH="$CLAUDE_STUB/bin:$PATH" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    collar_model_check opus
    printf "%s" "$?"
  ' fixture "$claude_collar")"
equal "Claude's empty-prompt native probe recognizes a documented alias" \
  "0" "$claude_recognized"
claude_unrecognized="$(PATH="$CLAUDE_STUB/bin:$PATH" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    output="$(collar_model_check opus-5)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
equal "Claude's native warning rejects an invented model id despite its exit status" \
  $'1\tnative validator rejected it as unrecognized' "$claude_unrecognized"

claude_brick_transcript="$RUN_ROOT/claude-model-brick.jsonl"
cat > "$claude_brick_transcript" <<'JSONL'
{"type":"user","isSidechain":false,"message":{"role":"user","content":"work"}}
{"type":"assistant","uuid":"fatal-model-record","isSidechain":false,"isApiErrorMessage":true,"error":"model_not_found","message":{"content":[{"type":"text","text":"There's an issue with the selected model (opus-5). It may not exist or you may not have access to it. Run /model to pick a different model."}]}}
{"type":"user","isSidechain":false,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"fixture","content":"done"}]}}
{"type":"user","isSidechain":false,"isMeta":true,"message":{"role":"user","content":"<local-command-caveat>DO NOT respond to these messages</local-command-caveat>"}}
{"type":"system","subtype":"turn_duration"}
JSONL
# An append without its terminating newline is not yet a JSONL record. The
# reader must ignore this in-flight suffix while retaining the fatal record.
printf '%s' '{"type":"assistant","isSidechain":false' >> "$claude_brick_transcript"
claude_brick_read="$(CLAUDE_TRANSCRIPT="$claude_brick_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    collar_bricked fixture
  ' fixture "$claude_collar")"
# source-guard: producer@1eee9490261d: the exact transcript fixture independently supplies the newest top-level model_not_found record, followed only by records the native transcript marks non-semantic and one incomplete append
equal "Claude keeps fatal model evidence across non-turn records and an in-flight append" \
  "selected model 'opus-5' was rejected (model_not_found)" "$claude_brick_read"

claude_recovery_transcript="$RUN_ROOT/claude-model-recovery.jsonl"
cat > "$claude_recovery_transcript" <<'JSONL'
{"type":"assistant","isSidechain":false,"isApiErrorMessage":true,"error":"model_not_found","message":{"content":[{"type":"text","text":"There's an issue with the selected model (opus-5). It may not exist or you may not have access to it. Run /model to pick a different model."}]}}
{"type":"user","isSidechain":false,"message":{"role":"user","content":"recovery turn"}}
JSONL
claude_recovery_read="$(CLAUDE_TRANSCRIPT="$claude_recovery_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_bricked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@99fb655af212: the fixture supplies a real string-content user turn newer than the fatal assistant record
equal "a real newer Claude user turn clears older fatal model evidence" \
  $'1\t' "$claude_recovery_read"

claude_malformed_transcript="$RUN_ROOT/claude-model-malformed.jsonl"
cat > "$claude_malformed_transcript" <<'JSONL'
{"type":"assistant","isSidechain":false,"isApiErrorMessage":true,"error":"model_not_found","message":{"content":[{"type":"text","text":"There's an issue with the selected model (opus-5). It may not exist or you may not have access to it. Run /model to pick a different model."}]}}
{"type":"assistant"
JSONL
claude_malformed_read="$(CLAUDE_TRANSCRIPT="$claude_malformed_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_bricked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@70b77ad72519: the fixture supplies a newline-terminated malformed record newer than the fatal assistant record
equal "a complete malformed Claude record remains loud unknown evidence" \
  $'2\tbound Claude transcript is unreadable' "$claude_malformed_read"

# A TURN THAT ENDED ON AN API ERROR THE FATAL READER DOES NOT OWN. The window
# accepts keystrokes, so every composer guard gang has says it is fine; the
# transcript says the turn it was given produced nothing. These read the shape
# the harness sets, never the error's prose, which is per-model and varies.
claude_blocked_transcript="$RUN_ROOT/claude-blocked-refusal.jsonl"
cat > "$claude_blocked_transcript" <<'JSONL'
{"type":"user","isSidechain":false,"message":{"role":"user","content":"work"}}
{"type":"system","subtype":"model_refusal_no_fallback"}
{"type":"assistant","uuid":"refusal-record","isSidechain":false,"isApiErrorMessage":true,"error":"invalid_request","message":{"content":[{"type":"text","text":"API Error: safeguards flagged this message."}]}}
{"type":"system","subtype":"turn_duration"}
JSONL
claude_blocked_read="$(CLAUDE_TRANSCRIPT="$claude_blocked_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_blocked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@5f3f4d796f76: this fixture alone puts a newest top-level API-error record, of a class the fatal reader does not own, at the tail
equal "Claude reports a turn that ended on an unowned API error as blocked" \
  $'0\tClaude Code ended the latest turn on an API error (invalid_request)' \
  "$claude_blocked_read"

claude_blocked_not_bricked="$(CLAUDE_TRANSCRIPT="$claude_blocked_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_bricked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@ac6db7d11c77: the same fixture holds an API-error class that only the blocked reader claims
equal "a blocked turn is not reported as a fatal one" $'1\t' \
  "$claude_blocked_not_bricked"

# THE ALLOWLIST MUST NOT GROW BACK. An error class no rule names is still a turn
# that produced nothing, and reporting absent here is the original defect.
claude_unnamed_transcript="$RUN_ROOT/claude-blocked-unnamed.jsonl"
cat > "$claude_unnamed_transcript" <<'JSONL'
{"type":"assistant","uuid":"unnamed-record","isSidechain":false,"isApiErrorMessage":true,"message":{"content":[{"type":"text","text":"API Error: something the harness has not shipped yet."}]}}
JSONL
claude_unnamed_read="$(CLAUDE_TRANSCRIPT="$claude_unnamed_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_blocked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@ca5777110662: this fixture alone holds an API-error record carrying no error name at all
equal "an API error the harness does not name is still blocked" \
  $'0\tClaude Code ended the latest turn on an API error it did not name' \
  "$claude_unnamed_read"

claude_blocked_model_transcript="$RUN_ROOT/claude-blocked-model.jsonl"
cat > "$claude_blocked_model_transcript" <<'JSONL'
{"type":"assistant","uuid":"fatal-model-record","isSidechain":false,"isApiErrorMessage":true,"error":"model_not_found","message":{"content":[{"type":"text","text":"There's an issue with the selected model (opus-5). It may not exist or you may not have access to it. Run /model to pick a different model."}]}}
JSONL
claude_blocked_model_read="$(CLAUDE_TRANSCRIPT="$claude_blocked_model_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_blocked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@69b9342fd9a4: this fixture holds the selected-model failure class the fatal reader owns and the blocked reader must decline
equal "the blocked reader declines a class the fatal reader owns" $'1\t' \
  "$claude_blocked_model_read"

claude_blocked_recovery_transcript="$RUN_ROOT/claude-blocked-recovery.jsonl"
cat > "$claude_blocked_recovery_transcript" <<'JSONL'
{"type":"assistant","uuid":"refusal-record","isSidechain":false,"isApiErrorMessage":true,"error":"invalid_request","message":{"content":[{"type":"text","text":"API Error: safeguards flagged this message."}]}}
{"type":"user","isSidechain":false,"message":{"role":"user","content":"recovery turn"}}
JSONL
claude_blocked_recovery_read="$(CLAUDE_TRANSCRIPT="$claude_blocked_recovery_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_blocked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@69c8884c0369: this fixture holds a real string-content user turn newer than the blocking API-error record
equal "a real newer Claude user turn clears older blocking evidence" $'1\t' \
  "$claude_blocked_recovery_read"

claude_blocked_malformed_transcript="$RUN_ROOT/claude-blocked-malformed.jsonl"
cat > "$claude_blocked_malformed_transcript" <<'JSONL'
{"type":"assistant","isSidechain":false,"isApiErrorMessage":true,"error":"invalid_request","message":{"content":[{"type":"text","text":"API Error: safeguards flagged this message."}]}}
{"type":"assistant"
JSONL
claude_blocked_malformed_read="$(CLAUDE_TRANSCRIPT="$claude_blocked_malformed_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_blocked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@bf97e61b3b9f: this fixture holds a newline-terminated malformed record newer than the blocking API-error record
equal "a complete malformed Claude record is unknown rather than blocked" \
  $'2\tbound Claude transcript is unreadable' "$claude_blocked_malformed_read"

claude_auto_fatal_transcript="$RUN_ROOT/claude-auto-resume-model-fatal.jsonl"
cat > "$claude_auto_fatal_transcript" <<'JSONL'
{"type":"assistant","uuid":"fatal-model-record","isSidechain":false,"isApiErrorMessage":true,"error":"model_not_found","message":{"content":[{"type":"text","text":"There's an issue with the selected model (opus-5). It may not exist or you may not have access to it. Run /model to pick a different model."}]}}
JSONL
claude_auto_fatal="$(CLAUDE_TRANSCRIPT="$claude_auto_fatal_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_auto_resume_record fixture idle_prompt)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@a38d42d05df7: the fixture supplies a complete fatal error record with an otherwise resumable UUID
equal "Claude auto-resume refuses selected-model failures" $'1\t' \
  "$claude_auto_fatal"

claude_auto_tail_transcript="$RUN_ROOT/claude-auto-resume-tail.jsonl"
cat > "$claude_auto_tail_transcript" <<'JSONL'
[]
{"type":"assistant","uuid":"latest-auto-error","isSidechain":false,"isApiErrorMessage":true,"error":"server_error","message":{"role":"assistant"}}
JSONL
# Claude appends its JSONL in place. Leave a newer record incomplete to prove
# it is not evidence yet, while the non-object before the selected assistant
# proves this read did not parse history that can no longer affect its answer.
printf '%s' '{"type":"assistant"' >> "$claude_auto_tail_transcript"
claude_auto_tail_read="$(CLAUDE_TRANSCRIPT="$claude_auto_tail_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    collar_auto_resume_record fixture idle_prompt
  ' fixture "$claude_collar")"
# source-guard: producer@973709e65fa5: the fixture uniquely supplies latest-auto-error on its newest complete assistant, while the older non-object and unfinished suffix independently make a byte-zero or in-flight parse fail
equal "Claude auto-resume reads only the newest relevant complete tail" \
  "latest-auto-error" "$claude_auto_tail_read"

# A complete malformed append can outrank the selected assistant, so it remains
# loud unknown evidence. Keep this fixture free of older malformed data: status
# 2 must come from the tail the assertion names.
claude_auto_bad_tail_transcript="$RUN_ROOT/claude-auto-resume-bad-tail.jsonl"
cat > "$claude_auto_bad_tail_transcript" <<'JSONL'
{"type":"assistant","uuid":"superseded-auto-error","isSidechain":false,"isApiErrorMessage":true,"error":"server_error","message":{"role":"assistant"}}
{"type":"assistant"
JSONL
claude_auto_bad_tail="$(CLAUDE_TRANSCRIPT="$claude_auto_bad_tail_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_auto_resume_record fixture idle_prompt)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@8ae53ae485d5: the fixture's sole unreadable complete record is the malformed append after an otherwise resumable assistant, so status 2 witnesses that newer tail
equal "Claude auto-resume keeps a malformed complete tail loud" \
  $'2\t' "$claude_auto_bad_tail"

claude_retry_transcript="$RUN_ROOT/claude-retry-error.jsonl"
cat > "$claude_retry_transcript" <<'JSONL'
{"type":"user","isSidechain":false,"message":{"role":"user","content":"work"}}
{"type":"assistant","isSidechain":false,"isApiErrorMessage":true,"error":"rate_limit","message":{"content":[{"type":"text","text":"Retrying in 2s"}]}}
JSONL
claude_retry_read="$(CLAUDE_TRANSCRIPT="$claude_retry_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_bricked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@15da07e96e08: the exact transcript fixture independently supplies the newest top-level transient rate-limit record
equal "Claude's transient rate-limit record is not fatal model evidence" \
  $'1\t' "$claude_retry_read"

claude_busy_regex="$(GANG_CONTEXT_LIGHTS=off ROOT="$ROOT" bash -c \
  '. "$1"; printf "%s" "$GANG_BUSY_REGEX"' fixture "$claude_collar")"
if printf '%s\n' 'API Error: 529 Overloaded. This is a server-side issue.' |
  grep -qE -- "$claude_busy_regex"; then
  pass "Claude's live HTTP 529 retry paint is busy"
else
  fail "Claude's live HTTP 529 retry paint is busy" "$claude_busy_regex"
fi

claude_529_transcript="$RUN_ROOT/claude-529-error.jsonl"
cat > "$claude_529_transcript" <<'JSONL'
{"type":"user","isSidechain":false,"message":{"role":"user","content":"work"}}
{"type":"assistant","uuid":"overloaded-529-record","isSidechain":false,"isApiErrorMessage":true,"error":"server_error","apiErrorStatus":529,"message":{"content":[{"type":"text","text":"API Error: 529 Overloaded. This is a server-side issue, usually temporary — try again in a moment. If it persists, check https://status.claude.com."}]}}
JSONL
claude_529_read="$(CLAUDE_TRANSCRIPT="$claude_529_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    collar_bricked fixture
  ' fixture "$claude_collar")"
# source-guard: producer@c83f29d35b38: the fixture supplies the native terminal assistant record whose exact error/status pair the collar reads
equal "Claude surfaces a terminal HTTP 529 turn" \
  "Claude Code ended the latest turn on HTTP 529 (server_error)" "$claude_529_read"

claude_other_server_error_transcript="$RUN_ROOT/claude-other-server-error.jsonl"
cat > "$claude_other_server_error_transcript" <<'JSONL'
{"type":"assistant","isSidechain":false,"isApiErrorMessage":true,"error":"server_error","apiErrorStatus":500,"message":{"content":[{"type":"text","text":"temporary provider failure"}]}}
JSONL
claude_other_server_error_read="$(CLAUDE_TRANSCRIPT="$claude_other_server_error_transcript" ROOT="$ROOT" \
  GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_bricked fixture)"; rc=$?
    printf "%s\\t%s" "$rc" "$output"
  ' fixture "$claude_collar")"
# source-guard: producer@0bafed54f094: the fixture differs from the native 529 record only in apiErrorStatus, so absence is evidence that the collar did not generalize server_error
equal "other Claude server errors remain nonfatal" \
  $'1\t' "$claude_other_server_error_read"

# A RESPONSE STREAM THAT DIED CARRIES NO HTTP STATUS. Claude Code writes the
# same terminal assistant record for it as for an exhausted 529 retry except
# that apiErrorStatus is absent entirely, and the sentence it carries varies
# between builds and failures while that structure does not. The three bodies
# below are the ones observed; each is asserted through the same reader, so a
# collar that started matching prose instead of shape would keep only one.
claude_stream_read() { # $1 = the record's visible text
  local transcript="$RUN_ROOT/claude-stream-$RANDOM.jsonl"
  python3 - "$transcript" "$1" <<'PY'
import json, sys
record = {
    "type": "assistant",
    "isSidechain": False,
    "isApiErrorMessage": True,
    "error": "server_error",
    "message": {"content": [{"type": "text", "text": sys.argv[2]}]},
}
with open(sys.argv[1], "w") as out:
    out.write(json.dumps(record) + "\n")
PY
  CLAUDE_TRANSCRIPT="$transcript" ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_bricked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar"
}
# source-guard: the fixture is the native terminal assistant record, built from
# the observed field set rather than from any pane rendering of it
for claude_stream_text in \
  "API Error: The response stopped arriving. The response above may be incomplete." \
  "API Error: Server error mid-response. The response above may be incomplete." \
  "API Error: Connection lost mid-response. The response above may be incomplete."
do
  equal "a broken Claude response stream is a fatal turn (${claude_stream_text:11:24}...)" \
    $'0\tClaude Code ended the latest turn on a broken response stream (server_error)' \
    "$(claude_stream_read "$claude_stream_text")"
done

# THE DISCRIMINATOR IS THE ABSENT KEY, so a record that carries a status must
# not reach the stream verdict — apiErrorStatus=500 above already proves the
# nonfatal side, and this proves the fatal side is not reached by prose alone.
claude_stream_with_status="$RUN_ROOT/claude-stream-with-status.jsonl"
cat > "$claude_stream_with_status" <<'JSONL'
{"type":"assistant","isSidechain":false,"isApiErrorMessage":true,"error":"server_error","apiErrorStatus":503,"message":{"content":[{"type":"text","text":"API Error: The response stopped arriving. The response above may be incomplete."}]}}
JSONL
# source-guard: producer@3de10462a30f: the fixture is the native record built here, and the reader is driven with the transcript path as its only input
equal "the same sentence with an HTTP status is not the stream verdict" \
  $'1\t' \
  "$(CLAUDE_TRANSCRIPT="$claude_stream_with_status" ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c '
      . "$1"
      tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
      output="$(collar_bricked fixture)"; rc=$?
      printf "%s\t%s" "$rc" "$output"
    ' fixture "$claude_collar")"

# A LATER REAL TURN OUTRANKS THE TERMINAL RECORD, which is what makes the
# verdict recover rather than stick to a window for the rest of its life.
claude_stream_recovered="$RUN_ROOT/claude-stream-recovered.jsonl"
cat > "$claude_stream_recovered" <<'JSONL'
{"type":"assistant","isSidechain":false,"isApiErrorMessage":true,"error":"server_error","message":{"content":[{"type":"text","text":"API Error: The response stopped arriving. The response above may be incomplete."}]}}
{"type":"user","isSidechain":false,"message":{"role":"user","content":"continue"}}
JSONL
# source-guard: producer@4952826df386: the fixture is the native record pair built here, and the reader is driven with the transcript path as its only input
equal "a real turn after a broken stream clears the fatal verdict" \
  $'1\t' \
  "$(CLAUDE_TRANSCRIPT="$claude_stream_recovered" ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c '
      . "$1"
      tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
      output="$(collar_bricked fixture)"; rc=$?
      printf "%s\t%s" "$rc" "$output"
    ' fixture "$claude_collar")"

# EVIDENCE OF ACTION. A wedged agent keeps talking, so text and thinking blocks
# are not action and a tool_use block is; the four verdicts below are the four
# things gang is allowed to say about it, and none of them is a health state.
claude_action_read() { # $1 = transcript path
  CLAUDE_TRANSCRIPT="$1" ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c '
    . "$1"
    tmux() { printf "%s" "$CLAUDE_TRANSCRIPT"; }
    output="$(collar_last_action fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$claude_collar"
}
claude_acted="$RUN_ROOT/claude-acted.jsonl"
cat > "$claude_acted" <<'JSONL'
{"type":"assistant","isSidechain":false,"timestamp":"2026-08-24T10:00:00.000Z","message":{"content":[{"type":"tool_use","name":"Bash","input":{}}]}}
{"type":"assistant","isSidechain":false,"timestamp":"2026-08-24T10:05:00.000Z","message":{"content":[{"type":"text","text":"done"}]}}
JSONL
equal "a witnessed tool call is reported at its own time" \
  $'0\tat 1787565600' "$(claude_action_read "$claude_acted")"

# THE WEDGE ITSELF: an agent that answered twice, in words, and ran nothing.
claude_wedged="$RUN_ROOT/claude-wedged.jsonl"
cat > "$claude_wedged" <<'JSONL'
{"type":"user","isSidechain":false,"timestamp":"2026-08-24T10:00:00.000Z","message":{"role":"user","content":"reproduce the issue"}}
{"type":"assistant","isSidechain":false,"timestamp":"2026-08-24T10:00:05.000Z","message":{"content":[{"type":"text","text":"I will reproduce the issue."}]}}
{"type":"assistant","isSidechain":false,"timestamp":"2026-08-24T13:56:29.000Z","message":{"content":[{"type":"thinking","thinking":"considering"},{"type":"text","text":"Building the reproduction."}]}}
JSONL
equal "an agent that only talked reports no tool call at all" \
  $'1\t' "$(claude_action_read "$claude_wedged")"

# UNKNOWN IS NOT NONE. A window with no bound transcript has taken no reading;
# reporting that as an agent which has never acted would invent the finding.
equal "an unbound window is unknown rather than inactive" \
  $'2\tno Claude transcript is bound to this window yet' \
  "$(claude_action_read "$RUN_ROOT/claude-never-written.jsonl")"
claude_action_garbage="$RUN_ROOT/claude-action-garbage.jsonl"
printf 'this is not a record\n' > "$claude_action_garbage"
equal "an unreadable transcript is unknown rather than inactive" \
  $'2\tbound Claude transcript is unreadable' \
  "$(claude_action_read "$claude_action_garbage")"

# THE SCAN BOUND IS ANSWERED AS A BOUND. Past it the reader may say the newest
# tool call is older than the oldest record it read, and may not say there is
# none: the difference is a claim about the whole session it did not make.
claude_action_bounded="$RUN_ROOT/claude-action-bounded.jsonl"
python3 - "$claude_action_bounded" <<'PY'
import json, sys
rows = [json.dumps({
    "type": "assistant", "isSidechain": False,
    "timestamp": "2026-08-24T09:00:00.000Z",
    "message": {"content": [{"type": "tool_use", "name": "Bash", "input": {}}]},
})]
rows += [json.dumps({
    "type": "assistant", "isSidechain": False,
    "timestamp": "2026-08-24T10:00:00.000Z",
    "message": {"content": [{"type": "text", "text": "talking"}]},
}) for _ in range(2500)]
with open(sys.argv[1], "w") as out:
    out.write("\n".join(rows) + "\n")
PY
equal "a tool call past the scan bound is reported as a bound" \
  $'0\tbefore 1787565600' "$(claude_action_read "$claude_action_bounded")"

codex_collar="$ROOT/collars/codex.sh"

# CODEX NAMES ITS TOOL CALLS IN MORE THAN ONE WAY. The rollout that started
# issue #150 was read as having no tool calls because only `function_call` was
# counted; the calls in it are `custom_tool_call`. Both families are asserted
# here so that reading can never regress to one of them, and a third, unknown
# family is asserted to be reported by name rather than silently counted as
# inaction — the direction that would make a working agent look wedged.
codex_action_read() { # $1 = rollout path
  CODEX_ROLLOUT="$1" ROOT="$ROOT" GANG_TEST_COLLARS='' bash -c '
    . "$1"
    tmux() { printf "%s" "$CODEX_ROLLOUT"; }
    output="$(collar_last_action fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$codex_collar"
}
for codex_action_family in function_call custom_tool_call local_shell_call; do
  codex_action_rollout="$RUN_ROOT/codex-action-$codex_action_family.jsonl"
  # source-guard: the fixture is the native rollout record shape — a
  # response_item envelope with its own timestamp and a payload naming the call
  printf '{"type":"response_item","timestamp":"2026-08-24T10:00:00.000Z","payload":{"type":"%s"}}\n' \
    "$codex_action_family" > "$codex_action_rollout"
  equal "codex reports a $codex_action_family as a tool call" \
    $'0\tat 1787565600' "$(codex_action_read "$codex_action_rollout")"
done
codex_action_talking="$RUN_ROOT/codex-action-talking.jsonl"
cat > "$codex_action_talking" <<'JSONL'
{"type":"response_item","timestamp":"2026-08-24T10:00:00.000Z","payload":{"type":"reasoning"}}
{"type":"response_item","timestamp":"2026-08-24T10:00:05.000Z","payload":{"type":"message"}}
JSONL
equal "a codex agent that only reasoned and spoke reports no tool call" \
  $'1\t' "$(codex_action_read "$codex_action_talking")"
codex_action_new="$RUN_ROOT/codex-action-new-family.jsonl"
printf '{"type":"response_item","timestamp":"2026-08-24T10:00:00.000Z","payload":{"type":"brand_new_call"}}\n' \
  > "$codex_action_new"
equal "an unrecognized codex call family is named, not counted as inaction" \
  $'2\tthis codex rollout records a call family gang does not read: brand_new_call' \
  "$(codex_action_read "$codex_action_new")"
equal "a codex window with no bound rollout is unknown rather than inactive" \
  $'2\tno codex rollout is bound to this window yet' \
  "$(codex_action_read "$RUN_ROOT/codex-never-written.jsonl")"

# A CODEX TURN THAT ENDED WITHOUT PRODUCING WORK. Codex has no error-typed
# terminator at all, so the signal is a conjunction on an ordinary completion:
# no closing message, no first token, and a turn body holding nothing but the
# input. Each field alone is worthless — turns that ran to dozens of tool calls
# end with no closing message — so every assertion below removes exactly one leg
# of the conjunction and requires the reader to fall silent.
codex_blocked_read() { # $1 = rollout path
  CODEX_ROLLOUT="$1" ROOT="$ROOT" GANG_TEST_COLLARS='' bash -c '
    . "$1"
    tmux() { printf "%s" "$CODEX_ROLLOUT"; }
    output="$(collar_blocked fixture)"; rc=$?
    printf "%s\t%s" "$rc" "$output"
  ' fixture "$codex_collar"
}
codex_blocked_reason='the codex turn that took the last input ended without producing work (no reply, and no first token)'

codex_hollow="$RUN_ROOT/codex-blocked-hollow.jsonl"
cat > "$codex_hollow" <<'JSONL'
{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"[gang:lead] do the thing"}]}}
{"type":"event_msg","payload":{"type":"user_message","message":"[gang:lead] do the thing"}}
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"time_to_first_token_ms":null,"duration_ms":1498}}
JSONL
# The whole conjunction, and nothing else in the body but the input that opened it.
equal "codex reports a turn that ended with nothing produced as blocked" \
  "0$(printf '\t')$codex_blocked_reason" "$(codex_blocked_read "$codex_hollow")"

codex_worked="$RUN_ROOT/codex-blocked-worked.jsonl"
cat > "$codex_worked" <<'JSONL'
{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}
{"type":"event_msg","payload":{"type":"user_message","message":"[gang:lead] do the thing"}}
{"type":"response_item","payload":{"type":"custom_tool_call","id":"c1"}}
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"time_to_first_token_ms":null,"duration_ms":1498}}
JSONL
# Same completion shape; one tool call in the body is the leg that removes it.
equal "a codex turn that worked and closed without a message is not blocked" \
  $'1\t' "$(codex_blocked_read "$codex_worked")"

codex_spoke="$RUN_ROOT/codex-blocked-spoke.jsonl"
cat > "$codex_spoke" <<'JSONL'
{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}
{"type":"event_msg","payload":{"type":"user_message","message":"[gang:lead] do the thing"}}
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"time_to_first_token_ms":842,"duration_ms":1498}}
JSONL
# Empty body, but the model did emit a first token, so the turn was answered.
equal "a codex turn whose model produced a first token is not blocked" \
  $'1\t' "$(codex_blocked_read "$codex_spoke")"

codex_compacted="$RUN_ROOT/codex-blocked-compaction.jsonl"
cat > "$codex_compacted" <<'JSONL'
{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}
{"type":"event_msg","payload":{"type":"context_compacted"}}
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"time_to_first_token_ms":null,"duration_ms":1498}}
JSONL
# Byte-identical completion; the compaction record in the body is what makes it healthy.
equal "a codex compaction turn is not blocked" $'1\t' \
  "$(codex_blocked_read "$codex_compacted")"

codex_interrupted="$RUN_ROOT/codex-blocked-interrupted.jsonl"
cat > "$codex_interrupted" <<'JSONL'
{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}
{"type":"event_msg","payload":{"type":"user_message","message":"[gang:lead] do the thing"}}
{"type":"event_msg","payload":{"type":"turn_aborted","turn_id":"t1","reason":"interrupted"}}
JSONL
# The terminator a person produces with Esc.
equal "an interrupted codex turn is not blocked — the interruption was the intervention" \
  $'1\t' "$(codex_blocked_read "$codex_interrupted")"

codex_inflight="$RUN_ROOT/codex-blocked-inflight.jsonl"
cat > "$codex_inflight" <<'JSONL'
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t0","last_agent_message":null,"time_to_first_token_ms":null,"duration_ms":900}}
{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"[gang:lead] and again"}]}}
JSONL
# Newest bracket event is a start nothing closes, sitting over an older hollow
# completion. A live turn and a harness that died mid-turn are identical here.
equal "a codex turn still in flight is absent, never blocked" $'1\t' \
  "$(codex_blocked_read "$codex_inflight")"

codex_retired="$RUN_ROOT/codex-blocked-retired.jsonl"
cat > "$codex_retired" <<'JSONL'
{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}
{"type":"event_msg","payload":{"type":"user_message","message":"[gang:lead] do the thing"}}
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"time_to_first_token_ms":null,"duration_ms":1498}}
{"type":"event_msg","payload":{"type":"task_started","turn_id":"t2"}}
{"type":"event_msg","payload":{"type":"user_message","message":"[gang:lead] relaunched you"}}
{"type":"response_item","payload":{"type":"custom_tool_call","id":"c1"}}
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t2","last_agent_message":"done","time_to_first_token_ms":700,"duration_ms":9000}}
JSONL
# A later turn that started and produced work — what a delivered intervention
# leaves behind, and what retires the hollow completion before it.
equal "a newer codex turn retires older blocking evidence" $'1\t' \
  "$(codex_blocked_read "$codex_retired")"

codex_newcall="$RUN_ROOT/codex-blocked-new-family.jsonl"
cat > "$codex_newcall" <<'JSONL'
{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}
{"type":"response_item","payload":{"type":"brand_new_call","id":"c1"}}
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"time_to_first_token_ms":null,"duration_ms":1498}}
JSONL
# UNKNOWN IS THE DEFAULT, and these are the names that proved it has to be. A
# suffix test caught brand_new_call and let three real vocabulary members
# through, each of which turned a turn that worked into a blocked window: the
# families here are _call, _call_output, _call_end and bare _output, so matching
# a suffix only ever buys the next name. Every payload type present in the
# rollouts is now classified by name, which is what makes an unknown mean
# "nobody has classified this" rather than "nothing happened".
equal "an unrecognized codex payload type is unknown rather than nothing produced" \
  $'2\tthis codex rollout records a turn payload gang does not read: brand_new_call' \
  "$(codex_blocked_read "$codex_newcall")"

for codex_worked_family in mcp_tool_call_end tool_search_output thread_goal_updated; do
  codex_worked_new="$RUN_ROOT/codex-blocked-work-$codex_worked_family.jsonl"
  {
    printf '{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}\n'
    printf '{"type":"event_msg","payload":{"type":"user_message","message":"[gang:lead] do the thing"}}\n'
    printf '{"type":"response_item","payload":{"type":"%s","id":"c1"}}\n' "$codex_worked_family"
    printf '{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"time_to_first_token_ms":null,"duration_ms":1498}}\n'
  } > "$codex_worked_new"
  equal "a codex turn whose only work is $codex_worked_family is not blocked" \
    $'1\t' "$(codex_blocked_read "$codex_worked_new")"
done

# THE TRUE POSITIVE SURVIVES THE INVERSION. A genuine blocked turn body also
# carries a typeless turn_context and world_state and a developer-role message.
# Classifying by payload type alone would leave those unnamed, and the inverted
# default would then turn the one frame this reader exists for into an unknown.
codex_typeless="$RUN_ROOT/codex-blocked-typeless.jsonl"
cat > "$codex_typeless" <<'JSONL'
{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}
{"type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"<permissions>"}]}}
{"type":"world_state","payload":{}}
{"type":"turn_context","payload":{}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"[gang:lead] do the thing"}]}}
{"type":"event_msg","payload":{"type":"user_message","message":"[gang:lead] do the thing"}}
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"time_to_first_token_ms":null,"duration_ms":1498}}
JSONL
equal "typeless records and a developer message do not hide a blocked turn" \
  "0$(printf '\t')$codex_blocked_reason" "$(codex_blocked_read "$codex_typeless")"

codex_headless="$RUN_ROOT/codex-blocked-headless.jsonl"
cat > "$codex_headless" <<'JSONL'
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"time_to_first_token_ms":null,"duration_ms":1498}}
JSONL
# A completion with no start anywhere in the file.
equal "a codex completion with no start is unknown rather than blocked" \
  $'2\tbound codex rollout holds no start for its newest turn' \
  "$(codex_blocked_read "$codex_headless")"

# THE SAME SHAPE, REACHED PAST A RECORD THE WALK MEETS ON THE WAY OUT. A
# session_meta never appears inside a turn body, but a terminator with no start
# before it puts one in the walk's path. Leaving it unclassified answered with
# the record it choked on instead of the shape that was actually wrong.
codex_headless_meta="$RUN_ROOT/codex-blocked-headless-meta.jsonl"
cat > "$codex_headless_meta" <<'JSONL'
{"type":"session_meta","payload":{}}
{"type":"event_msg","payload":{"type":"user_message","message":"[gang:lead] do the thing"}}
{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null,"time_to_first_token_ms":null,"duration_ms":1498}}
JSONL
equal "a startless codex turn reports its shape, not the record the walk met" \
  $'2\tbound codex rollout holds no start for its newest turn' \
  "$(codex_blocked_read "$codex_headless_meta")"

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
mkdir -p "$CODEX_CATALOG_STUB/bin" "$CODEX_CATALOG_STUB/home"
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
cat > "$CODEX_CATALOG_STUB/home/config.toml" <<'TOML' # snubline-ignore: home-path -- fixture subpath under CODEX_CATALOG_STUB, not a user or machine home directory
model = "gpt-5.6-sol"
TOML
chmod +x "$CODEX_CATALOG_STUB/bin/codex"
codex_models="$(PATH="$CODEX_CATALOG_STUB/bin:$PATH" ROOT="$ROOT" \
  bash -c '. "$1"; collar_models' fixture "$codex_collar")"
equal "the Codex native catalog carries every slug and its own efforts" \
  $'gpt-5.6-sol\tlow,medium,xhigh\ngpt-5.6-mini\tlow' "$codex_models"
codex_levels="$(GANG_MODEL=gpt-5.6-sol PATH="$CODEX_CATALOG_STUB/bin:$PATH" \
  sh -c "$codex_effort_cmd" | tr '\n' ' ')"
equal "the Codex effort vocabulary binds the exact model hitch will launch" \
  "low medium xhigh " "$codex_levels"
# THIS EXPECTATION CHANGED because an empty GANG_MODEL means hitch passes no
# -m, not that Codex launches no model. Codex then uses its configured model,
# and this paired config/catalog fixture proves the binding comes from that
# selection rather than a guessed catalog default.
codex_configured="$(GANG_MODEL='' CODEX_HOME="$CODEX_CATALOG_STUB/home" \
  PATH="$CODEX_CATALOG_STUB/bin:$PATH" \
  sh -c "$codex_effort_cmd")"
equal "the Codex effort vocabulary binds the configured model hitch will launch" \
  $'low\nmedium\nxhigh' "$codex_configured"
codex_explicit_over_config="$(GANG_MODEL=gpt-5.6-mini \
  CODEX_HOME="$CODEX_CATALOG_STUB/home" PATH="$CODEX_CATALOG_STUB/bin:$PATH" \
  sh -c "$codex_effort_cmd")"
equal "an explicit launch model outranks the configured one" \
  "low" "$codex_explicit_over_config"
# EVERY WAY OF NOT KNOWING STAYS THE SAME ANSWER: empty output at status 0, the
# could-not-determine channel bin/gang reads as a broken declaration rather than
# as a bad level. Reading configuration widened how a model can be chosen, not
# how one can be guessed, so a catalog default must never leak out of any of
# these. The unbound case is the older of the two and predates the config read.
mkdir -p "$CODEX_CATALOG_STUB/home-absent"
for codex_quiet_case in unbound config-unbound config-absent config-malformed config-nonstring; do
  codex_quiet_home="$CODEX_CATALOG_STUB/home-absent"
  codex_quiet_model=""
  case "$codex_quiet_case" in
    unbound) codex_quiet_model="gpt-5.6-nope" ;;
    config-unbound)
      codex_quiet_home="$CODEX_CATALOG_STUB/home-unbound"
      mkdir -p "$codex_quiet_home"
      printf 'model = "gpt-5.6-nope"\n' > "$codex_quiet_home/config.toml" ;;
    config-malformed)
      codex_quiet_home="$CODEX_CATALOG_STUB/home-malformed"
      mkdir -p "$codex_quiet_home"
      printf 'model = "gpt-5.6-sol\n' > "$codex_quiet_home/config.toml" ;;
    config-nonstring)
      codex_quiet_home="$CODEX_CATALOG_STUB/home-nonstring"
      mkdir -p "$codex_quiet_home"
      printf '[model]\nslug = "gpt-5.6-sol"\n' > "$codex_quiet_home/config.toml" ;;
  esac
  codex_quiet="$(GANG_MODEL="$codex_quiet_model" CODEX_HOME="$codex_quiet_home" \
    PATH="$CODEX_CATALOG_STUB/bin:$PATH" sh -c "$codex_effort_cmd")"
  equal "the Codex effort vocabulary answers nothing rather than a guess ($codex_quiet_case)" \
    "" "$codex_quiet"
done
# THE CONFIG READER IS OPTIONAL, THE ANSWER IS NOT. tomllib arrived in python3
# 3.11; importing it beside json made an explicit model stop answering on every
# older interpreter, which is a wider outage than the one the config read
# closed. Shadowing the module proves both halves at once — the explicit model
# still answers, and the configured one goes quiet, which it can only do if the
# shadow actually took.
mkdir -p "$CODEX_CATALOG_STUB/no-tomllib"
printf 'raise ImportError("tomllib")\n' > "$CODEX_CATALOG_STUB/no-tomllib/tomllib.py"
codex_old_python="$(GANG_MODEL=gpt-5.6-sol \
  PYTHONPATH="$CODEX_CATALOG_STUB/no-tomllib" \
  PATH="$CODEX_CATALOG_STUB/bin:$PATH" sh -c "$codex_effort_cmd" | tr '\n' ' ')"
equal "an explicit launch model answers on a python3 without tomllib" \
  "low medium xhigh " "$codex_old_python"
codex_old_python_config="$(GANG_MODEL='' CODEX_HOME="$CODEX_CATALOG_STUB/home" \
  PYTHONPATH="$CODEX_CATALOG_STUB/no-tomllib" \
  PATH="$CODEX_CATALOG_STUB/bin:$PATH" sh -c "$codex_effort_cmd")"
equal "a configured model needs the config reader and says so by silence" \
  "" "$codex_old_python_config"
# opencode and pi refuse -e by declaring nothing: their native effort forms
# are unverified, and an unverified spelling must not reach a launch line.
for unverified_collar in opencode pi; do
  unverified_file="$ROOT/collars/$unverified_collar.sh"
  unverified_effort="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
    '. "$1"; printf "%s" "${GANG_EFFORT_OPT:-}"' fixture "$unverified_file")"
  equal "the $unverified_collar collar declares no effort spelling until one is verified" \
    "" "$unverified_effort"
done

MODEL_LIST_STUB="$RUN_ROOT/model-list-stub"
mkdir -p "$MODEL_LIST_STUB"
cat > "$MODEL_LIST_STUB/opencode" <<'SH'
#!/bin/sh
printf 'anthropic/claude-sonnet\nopenai/gpt-5\n'
SH
cat > "$MODEL_LIST_STUB/pi" <<'SH'
#!/bin/sh
cat <<'ROWS'
provider   model           context  max-out  thinking  images
anthropic  claude-sonnet  200K     64K      yes       yes
openai     gpt-5           400K     128K     yes       yes
ROWS
SH
chmod +x "$MODEL_LIST_STUB/opencode" "$MODEL_LIST_STUB/pi"
opencode_models="$(PATH="$MODEL_LIST_STUB:$PATH" \
  bash -c '. "$1"; collar_models' fixture "$ROOT/collars/opencode.sh")"
equal "the OpenCode collar preserves native provider/model ids" \
  $'anthropic/claude-sonnet\nopenai/gpt-5' "$opencode_models"
pi_models="$(PATH="$MODEL_LIST_STUB:$PATH" \
  bash -c '. "$1"; collar_models' fixture "$ROOT/collars/pi.sh")"
equal "the Pi collar joins its native provider and model columns" \
  $'anthropic/claude-sonnet\nopenai/gpt-5' "$pi_models"

for otel_collar in bash claude-code codex opencode pi; do
  otel_attributes="$(name=limitsmith \
    OTEL_RESOURCE_ATTRIBUTES='service.name=operator,team=observability' \
    GANG_TEST_COLLARS='' bash -c '
      ROOT="$1"
      . "$2"
      sh -c '\''printf "%s" "$OTEL_RESOURCE_ATTRIBUTES"'\''
    ' fixture "$ROOT" "$ROOT/collars/$otel_collar.sh")"
  equal "the $otel_collar collar exports its agent identity and preserves operator resource attributes" \
    "gang.agent=limitsmith,service.name=operator,team=observability" \
    "$otel_attributes"
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
claude_timeout_args="$RUN_ROOT/claude-timeout.args"
: > "$claude_timeout_args"
cat > "$claude_limits_stub/claude" <<'SH'
#!/bin/sh
cat <<'OUT'
You are currently using your subscription to power your Claude Code usage

Current session: 93% used · resets Aug 13, 4:49pm (America/Vancouver)
Current week (all models): 96% used · resets Aug 19, 10am (America/Vancouver)
Current week (Fable): 3% used · resets Aug 19, 10am (America/Vancouver)
OUT
SH
cat > "$claude_limits_stub/timeout" <<SH
#!/bin/sh
printf '%s\n' "\$*" >> "$claude_timeout_args"
case "\$1" in 1|50) shift; exec "\$@" ;; *) exit 2 ;; esac
SH
chmod +x "$claude_limits_stub/claude" "$claude_limits_stub/timeout"
claude_limits="$(PATH="$claude_limits_stub:$PATH" GANG_TEST_COLLARS='' ROOT="$ROOT" \
  bash -c '. "$1"; collar_usage_limits ignored' fixture "$claude_collar")"
contains "Claude provider limits use the headless native usage command" \
  "$claude_limits" $'Current week (all models)\t96\t'
contains "Claude provider limits accept a reset hour with no minute field" \
  "$claude_limits" $'Current week (Fable)\t3\t'
equal "Claude throttles its heavyweight usage-light subprocess" "60" \
  "$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
    '. "$1"; printf "%s" "$GANG_USAGE_LIGHT_INTERVAL"' fixture "$claude_collar")"
equal "Claude uses the bound runner's portable form with hook margin" \
  $'1 true\n50 claude -p /usage' "$(<"$claude_timeout_args")"

claude_bad_timeout="$RUN_ROOT/claude-bad-timeout"
mkdir -p "$claude_bad_timeout"
cat > "$claude_bad_timeout/timeout" <<'SH'
#!/bin/sh
exit 2
SH
chmod +x "$claude_bad_timeout/timeout"
claude_bad_bound="$(PATH="$claude_bad_timeout:$PATH" \
  GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c '
    . "$1"
    collar_usage_limits ignored >/dev/null 2>&1
    rc=$?
    printf "%s\t" "$rc"
    collar_usage_limits_error "$rc"
  ' fixture "$claude_collar")"
contains "Claude names an installed but incompatible bound runner" \
  "$claude_bad_bound" $'65\t'"collar 'claude-code' found a 'timeout' command"
claude_no_bound_runner="$RUN_ROOT/claude-no-bound-runner"
mkdir -p "$claude_no_bound_runner"
claude_shell="$(command -v bash)"
claude_missing_bound="$(PATH="$claude_no_bound_runner" \
  GANG_TEST_COLLARS='' ROOT="$ROOT" "$claude_shell" -c '
    . "$1"
    collar_usage_limits ignored >/dev/null 2>&1
    rc=$?
    printf "%s\t" "$rc"
    collar_usage_limits_error "$rc"
  ' fixture "$claude_collar")"
contains "Claude names a missing bound runner" "$claude_missing_bound" \
  $'64\t'"collar 'claude-code' requires the 'timeout' command"

mkdir -p "$RUN_ROOT/collars"
export GANG_COLLARS="$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/cataloged.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_MODEL_OPT='--model'
collar_models() { printf 'zeta\tlow,high\nalpha\n'; }
SH
cataloged_models="$($GANG models -c cataloged 2> "$RUN_ROOT/cataloged-models.err")"
equal "model discovery sorts a collar's complete native catalog" \
  $'alpha\nzeta\tlow,high' "$cataloged_models"
equal "a valid complete catalog needs no diagnostic" "" \
  "$(grep -v '^gang: WARNING: executing dirty ' "$RUN_ROOT/cataloged-models.err")"

cat > "$RUN_ROOT/collars/empty-models.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_MODEL_OPT='--model'
collar_models() { :; }
SH
refuses "an empty native model catalog is unknown, not an empty success" \
  "model enumeration produced no ids" "$GANG" models -c empty-models

cat > "$RUN_ROOT/collars/malformed-models.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_MODEL_OPT='--model'
collar_models() { printf 'same\nsame\n'; }
SH
refuses "duplicate native model ids make the collar catalog unreadable" \
  "returned a model catalog Gangline cannot interpret" \
  "$GANG" models -c malformed-models

# ONE UNREADABLE ROW USED TO COST THE WHOLE COLLAR. opencode's OpenRouter
# provider spells ids with a `~` routing prefix that this vocabulary cannot use,
# and refusing the catalog over them blocked every opencode hitch on the host —
# including hitches for models on providers whose rows read fine. The requested
# model's row is the only one that has to be usable.
cat > "$RUN_ROOT/collars/skippable-models.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_MODEL_OPT='--model'
collar_models() {
  printf 'zeta\topus,haiku\nopenrouter/~anthropic/claude-fable-latest\nalpha\n'
}
SH
skippable_err="$RUN_ROOT/skippable-models.err"
skippable_models="$($GANG models -c skippable-models 2> "$skippable_err")"
equal "an unusable id costs its own row and nothing else" \
  $'alpha\nzeta\topus,haiku' "$skippable_models"
contains "and the dropped row is named rather than dropped in silence" \
  "$(grep -v '^gang: WARNING: executing dirty ' "$skippable_err")" \
  "openrouter/~anthropic/claude-fable-latest"

# A CATALOG WITH NOTHING LEFT IN IT IS STILL A BROKEN PRODUCER, said out loud.
# Skipping rows must not turn an enumerator that emits nothing usable into a
# quiet empty success.
cat > "$RUN_ROOT/collars/all-unusable-models.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_MODEL_OPT='--model'
collar_models() { printf 'a~b\nc~d\n'; }
SH
refuses "a catalog whose every row is unusable is still unreadable" \
  "returned a model catalog Gangline cannot interpret" \
  "$GANG" models -c all-unusable-models

claude_alias_err="$RUN_ROOT/claude-alias-models.err"
claude_alias_models="$($GANG models -c claude-code 2> "$claude_alias_err")"
equal "Claude discovery prints only aliases documented by its native help" \
  $'fable\nopus\nsonnet' "$claude_alias_models"
contains "Claude discovery says full model names cannot be enumerated" \
  "$(<"$claude_alias_err")" "cannot enumerate full model names"
contains "Claude discovery separates recognition from account availability" \
  "$(<"$claude_alias_err")" "not availability to this account"

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

# ---------------------------------------------------------------------------
# THE GUARD ON THE ONE PROCESS WHOSE DEATH ENDS THE WHOLE TEAM. Inside a pane
# $TMUX names the current server and outranks TMUX_TMPDIR, so a teardown that
# reads as aimed at a sandbox lands on the live server; on 2026-08-17 that took
# a team while the sandbox server survived. The shim gang puts at the front of
# an agent's PATH decides whether the resolved socket still carries a Gangline
# registration. The record a caller names is corroboration only: testing
# Gangline itself legitimately replaces it with an empty sandbox.
guard_shim="$ROOT/libexec/gang-tmux-guard/tmux"
guard_home="$RUN_ROOT/tmux-guard"
guard_bin="$guard_home/bin"
guard_state="$guard_home/state"
guard_ran="$guard_home/real-tmux-argv"
guard_servers="$guard_home/agent-servers"
guard_unreadable_servers="$guard_home/unreadable-servers"
guard_unreachable_labels="$guard_home/unreachable-labels"
guard_option_unavailable_servers="$guard_home/option-unavailable-servers"
guard_agent_rows="$guard_home/agent-rows"
guard_fake_uid="guard-test-$$"
mkdir -p "$guard_bin" "$guard_state/teams"
cat > "$guard_bin/tmux" <<SH
#!/bin/sh
socket=""
label=""
if [ "\${1:-}" = -S ]; then
  socket="\${2:-}"
  shift 2
elif [ "\${1:-}" = -L ]; then
  label="\${2:-}"
  shift 2
fi
if [ "\${1:-}" = display-message ]; then
  if [ -n "\$socket" ] \
     && grep -Fqx "\$socket" "$guard_option_unavailable_servers"; then
    exit 1
  fi
  if [ -n "\$label" ] && grep -Fqx "\$label" "$guard_unreachable_labels"; then
    exit 1
  fi
  if [ -n "\$socket" ]; then
    printf '%s\\n' "\$socket"
  elif [ -n "\$label" ]; then
    printf '%s\\n' "\${TMUX_TMPDIR:-/tmp}/tmux-$guard_fake_uid/\$label"
  elif [ -n "\${TMUX:-}" ]; then
    printf '%s\\n' "\${TMUX%%,*}"
  else
    printf '%s\\n' "\${TMUX_TMPDIR:-/tmp}/tmux-$guard_fake_uid/default"
  fi
  exit 0
fi
if [ "\${1:-}" = list-windows ]; then
  target=""
  shift
  while [ \$# -gt 0 ]; do
    case "\$1" in
      -t) target="\${2:-}"; shift 2; continue ;;
      -t*) target="\${1#-t}" ;;
    esac
    shift
  done
  if [ "\$target" = '@guard-option-missing' ]; then
    exit 1
  fi
  if [ -n "\$socket" ] \
     && grep -Fqx "\$socket" "$guard_option_unavailable_servers"; then
    exit 1
  fi
  if grep -Fqx "\$socket" "$guard_unreadable_servers"; then
    exit 1
  elif grep -Fqx "\$socket" "$guard_servers"; then
    case "\$target" in
      ''|guardteam|=guardteam) cat "$guard_agent_rows" ;;
    esac
  fi
  exit 0
fi
printf '%s\n' "\$*" > "$guard_ran"
if [ -n "\$label" ] && grep -Fqx "\$label" "$guard_unreachable_labels"; then
  printf 'error connecting to unreachable label %s\n' "\$label" >&2
  exit 1
fi
exit 0
SH
printf '%s\n' '#!/bin/sh' "printf '%s\\n' '$guard_fake_uid'" > "$guard_bin/id"
chmod +x "$guard_bin/tmux" "$guard_bin/id"
guard_team_socket="$guard_home/team-socket"
printf '%s\n' "$guard_team_socket" > "$guard_state/teams/guardteam"
printf '%s\n' "$guard_team_socket" > "$guard_servers"
: > "$guard_unreadable_servers"
: > "$guard_unreachable_labels"
: > "$guard_option_unavailable_servers"
printf 'guardteam\tguard-agent\n' > "$guard_agent_rows"

guard_session=guardteam
# THE ENVIRONMENT IS THE INPUT UNDER TEST, so each call states it and the next
# starts from nothing: an inherited $TMUX would decide a later case silently.
# EVERY CALL STATES ITS WHOLE ENVIRONMENT, because the environment is the input
# under test: an inherited $TMUX from the previous case would decide the next one
# silently, and a run reading it as unset is a different test from one reading it
# as the team's socket. `-` is the spelling for "this variable is not set".
guard_run() { # $1 TMUX, $2 TMUX_TMPDIR, $3 GANG_TMUX_GUARD, rest = argv;
              # prints "<rc>\n<stderr>" and records whether the real tmux ran
  local want_tmux="$1" want_tmpdir="$2" want_guard="$3" rc=0 err
  shift 3
  [ "$want_tmux" != - ] || want_tmux=""
  [ "$want_tmpdir" != - ] || want_tmpdir=""
  [ "$want_guard" != - ] || want_guard=""
  rm -f -- "$guard_ran"
  err="$(PATH="$ROOT/libexec/gang-tmux-guard:$guard_bin:/usr/bin:/bin" \
    GANG_SESSION="$guard_session" GANG_LOCK_DIR="$guard_state" \
    TMUX="$want_tmux" TMUX_TMPDIR="$want_tmpdir" GANG_TMUX_GUARD="$want_guard" \
    "$guard_shim" "$@" 2>&1 >/dev/null)" || rc=$?
  printf '%s\n%s' "$rc" "$err"
}
guard_reached_tmux() { [ -f "$guard_ran" ]; }

# QUIET WINDOW-OPTION READS COLLAPSE TWO STATES IN TMUX 3.2A. Both an unset
# option on a readable window and the same option on a nonexistent window exit
# zero with no output. The shim already fronts these reads for every hitched
# agent, so it must expose the target record's availability in the status while
# retaining tmux's empty successful result for the readable, unset control.
guard_out="$(guard_run "$guard_team_socket,1,0" - - \
  show-options -wqv -t @guard-option-readable @gl_probe)"
equal "an unset option on a readable window remains a successful empty read" \
  0 "$(printf '%s' "$guard_out" | head -1)"
if guard_reached_tmux; then
  pass "the readable option lookup reaches tmux"
else
  fail "the readable option lookup reaches tmux" "show-options never ran"
fi
guard_out="$(guard_run "$guard_team_socket,1,0" - - \
  show-options -wqv -t @guard-option-missing @gl_probe)"
equal "a quiet option lookup exposes an unavailable window record" \
  1 "$(printf '%s' "$guard_out" | head -1)"
contains "the unavailable option lookup names the record it could not read" \
  "$guard_out" "window option target @guard-option-missing is unavailable"
contains "the unavailable option lookup is recorded" \
  "$(cat "$guard_state/tmux-guard.log" 2>/dev/null)" \
  "unavailable-window-option"
if guard_reached_tmux; then
  fail "an unavailable option lookup stops before its ambiguous read" \
    "show-options ran and returned the same empty success as an unset option"
else
  pass "an unavailable option lookup stops before its ambiguous read"
fi
guard_option_unavailable_socket="$guard_home/option-unavailable-socket"
printf '%s\n' "$guard_option_unavailable_socket" \
  > "$guard_option_unavailable_servers"
guard_out="$(guard_run - - - -S "$guard_option_unavailable_socket" \
  show-options -wqv -t @guard-option-server-lost @gl_probe)"
equal "a quiet option lookup distinguishes an unavailable tmux server" \
  2 "$(printf '%s' "$guard_out" | head -1)"
contains "the unavailable-server lookup names the record it could not read" \
  "$guard_out" \
  "tmux server for window option target @guard-option-server-lost is unavailable"
contains "the unavailable-server lookup is recorded separately" \
  "$(cat "$guard_state/tmux-guard.log" 2>/dev/null)" \
  "unavailable-window-option-server"

# THE 2026-08-17 COMMAND, verbatim in shape: a sandbox TMUX_TMPDIR set, and
# $TMUX quietly deciding otherwise. This is the assertion the guard exists for.
guard_out="$(guard_run "$guard_team_socket,1,0" "$guard_home/sandbox" - kill-server)"
equal "a sandbox-looking kill-server inside a pane is refused" \
  3 "$(printf '%s' "$guard_out" | head -1)"
contains "and the refusal names the team it would have ended" \
  "$guard_out" "team 'guardteam'"
if guard_reached_tmux; then
  fail "a refused kill-server never reaches tmux" "the real tmux ran"
else
  pass "a refused kill-server never reaches tmux"
fi
contains "a refusal outlives the pane it was made in" \
  "$(cat "$guard_state/tmux-guard.log" 2>/dev/null)" "refused"

# ADVICE THAT NAMES A REFUSED COMMAND IS A TRAP. A refusal is only fail-closed
# if the route it hands the reader is one that opens; a route this same guard
# refuses leaves them with nothing but GANG_TMUX_GUARD=off, which is the
# mechanism the guard exists to stand in front of. So the advised route is
# driven here rather than read: the aimed kill-server is refused on a socket
# carrying an agent window, and the SAME command must pass once the agents the
# refusal is about are gone.
guard_out="$(guard_run - - - -S "$guard_team_socket" kill-server)"
equal "an explicitly aimed kill-server is refused while the socket carries an agent" \
  3 "$(printf '%s' "$guard_out" | head -1)"
contains "so the refusal advises ending the agents, not aiming the same command again" \
  "$guard_out" "gang drop"
printf '' > "$guard_agent_rows"
guard_out="$(guard_run - - - -S "$guard_team_socket" kill-server)"
equal "and that advised route opens: the same kill-server passes once no agent is on it" \
  0 "$(printf '%s' "$guard_out" | head -1)"
if guard_reached_tmux; then
  pass "the advised route reaches tmux rather than being refused a second time"
else
  fail "the advised route reaches tmux rather than being refused a second time" \
    "kill-server never ran"
fi
printf 'guardteam\tguard-agent\n' > "$guard_agent_rows"

# THE SAME TRAP ON THE OTHER TWO REFUSALS. kill-session refuses a target that
# carries an agent window, so advice naming an aimed kill-session is a route
# this guard closes; the route that opens is the same one, after the agents.
guard_out="$(guard_run - - - -S "$guard_team_socket" kill-session -t guardteam)"
equal "an aimed kill-session at a team session is refused" \
  3 "$(printf '%s' "$guard_out" | head -1)"
contains "and it too advises ending the agents rather than aiming again" \
  "$guard_out" "gang drop"
guard_out="$(guard_run "$guard_team_socket,1,0" - - kill-session)"
equal "an untargeted kill-session inside a pane is refused" \
  3 "$(printf '%s' "$guard_out" | head -1)"
contains "and it advises the route that opens" "$guard_out" "gang drop"
printf '' > "$guard_agent_rows"
guard_out="$(guard_run - - - -S "$guard_team_socket" kill-session -t guardteam)"
equal "and that route opens for kill-session once no agent is on the target" \
  0 "$(printf '%s' "$guard_out" | head -1)"
printf 'guardteam\tguard-agent\n' > "$guard_agent_rows"


# THE RECURRING BYPASS. A test of Gangline legitimately replaces both values
# that locate a team record with a fresh empty sandbox. The protected socket is
# still the one $TMUX names, and its own registration must keep the guard shut.
guard_sandbox_state="$guard_home/empty-sandbox"
mkdir -p "$guard_sandbox_state"
rm -f -- "$guard_ran"
guard_rc=0
guard_out="$(PATH="$ROOT/libexec/gang-tmux-guard:$guard_bin:/usr/bin:/bin" \
  GANG_SESSION=probe GANG_LOCK_DIR="$guard_sandbox_state" \
  TMUX="$guard_team_socket,1,0" TMUX_TMPDIR="$guard_home/sandbox" \
  "$guard_shim" kill-server 2>&1 >/dev/null)" || guard_rc=$?
equal "an empty sandbox record cannot blind the live-server guard" 3 "$guard_rc"
if guard_reached_tmux; then
  fail "an empty sandbox record still cannot reach tmux" "the real tmux ran"
else
  pass "an empty sandbox record still cannot reach tmux"
fi
contains "the sandbox-bypassed refusal is still logged" \
  "$(cat "$guard_sandbox_state/tmux-guard.log" 2>/dev/null)" "refused"

# This one drives a separately owned tmux server, not the fake above: tmux
# format strings do not turn a literal backslash-t into a field separator. A
# broken guard can only destroy this disposable server, never the integration
# server that is carrying the rest of the suite.
guard_real_bin="$guard_home/real-bin"
guard_real_state="$guard_home/real-state"
guard_team_log="$guard_home/team-log"
guard_live_socket="$guard_home/live-server"
guard_live_session="guard-live-$$"
mkdir -p "$guard_real_bin" "$guard_real_state" "$guard_team_log"
printf '%s\n' '#!/bin/sh' "printf '%s\\n' '$guard_fake_uid-real'" > "$guard_real_bin/id"
chmod +x "$guard_real_bin/id"
"$REAL_TMUX" -S "$guard_live_socket" new-session -d -s "$guard_live_session" -n agent 'exec cat'
"$REAL_TMUX" -S "$guard_live_socket" set-option -w -t "=$guard_live_session:agent" @gl_agent probe-agent
guard_rc=0
guard_out="$(PATH="$ROOT/libexec/gang-tmux-guard:$guard_real_bin:$(dirname "$REAL_TMUX"):/usr/bin:/bin" \
  GANG_SESSION=probe GANG_LOCK_DIR="$guard_real_state" \
  GANG_TMUX_GUARD_LOG_DIR="$guard_team_log" TMUX="$guard_live_socket,1,0" \
  "$guard_shim" kill-server 2>&1 >/dev/null)" || guard_rc=$?
equal "a real tmux registration survives an empty sandbox record" 3 "$guard_rc"
contains "the real-server refusal names its registered team" "$guard_out" "team '$guard_live_session'"
if "$REAL_TMUX" -S "$guard_live_socket" has-session -t "=$guard_live_session" >/dev/null 2>&1; then
  pass "a refused real-server teardown leaves its disposable server alive"
else
  fail "a refused real-server teardown leaves its disposable server alive" \
    "the guard allowed kill-server to end $guard_live_session"
fi
"$REAL_TMUX" -S "$guard_live_socket" kill-server >/dev/null 2>&1 || true
# source-guard: producer@44c7b12a6b77: the direct real-server guard invocation above is the sole writer to this fresh team-log directory
contains "the original team root receives the sandbox refusal" \
  "$(cat "$guard_team_log/tmux-guard.log" 2>/dev/null)" "refused"

# TMUX ITSELF SETTLES THE TARGET. tmux 3.2a silently ignores a TMUX_TMPDIR
# whose directory is absent and falls back to its ordinary socket root. The
# guard used to build the missing path itself, fail to read it, and then pass
# the teardown to a tmux that reached somewhere else. This fixture uses the
# real tmux and a unique label: the fallback can destroy only this disposable
# server, and the EXIT trap owns that exact label until the explicit cleanup.
# The parent exists while the child deliberately does not, so absence is the
# input under test rather than a failed fixture setup.
guard_fallback_home="$guard_home/fallback"
guard_missing_tmpdir="$guard_fallback_home/missing-tmux-root"
guard_fallback_label="gangline-guard-fallback-$$"
guard_fallback_session="guard-fallback-$$"
mkdir -p "$guard_fallback_home"
if [ ! -e "$guard_missing_tmpdir" ]; then
  pass "the retargeting probe starts with its TMUX_TMPDIR absent"
else
  fail "the retargeting probe starts with its TMUX_TMPDIR absent" \
    "$guard_missing_tmpdir exists"
fi
env -u TMUX -u TMUX_PANE -u TMUX_TMPDIR \
  "$REAL_TMUX" -L "$guard_fallback_label" new-session -d \
    -s "$guard_fallback_session" -n agent 'exec cat'
env -u TMUX -u TMUX_PANE -u TMUX_TMPDIR \
  "$REAL_TMUX" -L "$guard_fallback_label" set-option -w \
    -t "=$guard_fallback_session:agent" @gl_agent fallback-agent
guard_fallback_socket="$(env -u TMUX -u TMUX_PANE \
  TMUX_TMPDIR="$guard_missing_tmpdir" \
  "$REAL_TMUX" -L "$guard_fallback_label" display-message -p '#{socket_path}')"
equal "real tmux answers with a socket outside the missing TMUX_TMPDIR" \
  "$(env -u TMUX -u TMUX_PANE -u TMUX_TMPDIR \
    "$REAL_TMUX" -L "$guard_fallback_label" display-message -p '#{socket_path}')" \
  "$guard_fallback_socket"
guard_rc=0
guard_out="$(env -u TMUX -u TMUX_PANE \
  PATH="$ROOT/libexec/gang-tmux-guard:$(dirname "$REAL_TMUX"):/usr/bin:/bin" \
  GANG_SESSION=probe GANG_LOCK_DIR="$guard_real_state" \
  TMUX_TMPDIR="$guard_missing_tmpdir" \
  "$guard_shim" -L "$guard_fallback_label" kill-server 2>&1 >/dev/null)" \
  || guard_rc=$?
equal "a kill-server under an absent TMUX_TMPDIR fails closed" 3 "$guard_rc"
contains "the retargeted refusal names tmux's actual socket" \
  "$guard_out" "$guard_fallback_socket"
if env -u TMUX -u TMUX_PANE -u TMUX_TMPDIR \
  "$REAL_TMUX" -L "$guard_fallback_label" has-session \
    -t "=$guard_fallback_session" >/dev/null 2>&1; then
  pass "the absent-TMUX_TMPDIR refusal leaves its disposable server alive"
else
  fail "the absent-TMUX_TMPDIR refusal leaves its disposable server alive" \
    "the guard allowed kill-server to end $guard_fallback_session"
fi
env -u TMUX -u TMUX_PANE -u TMUX_TMPDIR \
  "$REAL_TMUX" -L "$guard_fallback_label" kill-server >/dev/null 2>&1 || true
guard_fallback_label=""

# THE SAME FALLBACK PRECEDES TEARDOWN. Before a fixture reaches kill-server,
# ordinary new-session, hitch, send-keys, and kill-pane traffic is already on
# the wrong server. Reject the bad root for every tmux command, proven here by
# a synchronous creation that must leave no server behind. A second unique
# label contains the pre-fix failure and is owned by the EXIT trap.
guard_ordinary_label="gangline-guard-ordinary-$$"
guard_ordinary_session="guard-ordinary-$$"
guard_rc=0
guard_out="$(env -u TMUX -u TMUX_PANE \
  PATH="$ROOT/libexec/gang-tmux-guard:$(dirname "$REAL_TMUX"):/usr/bin:/bin" \
  GANG_SESSION=probe GANG_LOCK_DIR="$guard_real_state" \
  TMUX_TMPDIR="$guard_missing_tmpdir" \
  "$guard_shim" -L "$guard_ordinary_label" new-session -d \
    -s "$guard_ordinary_session" -n fixture 'exec cat' 2>&1 >/dev/null)" \
  || guard_rc=$?
equal "ordinary tmux traffic under an absent TMUX_TMPDIR fails closed" 3 "$guard_rc"
contains "and says the fixture root does not exist" \
  "$guard_out" "$guard_missing_tmpdir"
if env -u TMUX -u TMUX_PANE -u TMUX_TMPDIR \
  "$REAL_TMUX" -L "$guard_ordinary_label" has-session \
    -t "=$guard_ordinary_session" >/dev/null 2>&1; then
  fail "the refused ordinary command creates no fallback server" \
    "$guard_ordinary_session exists on the default socket root"
else
  pass "the refused ordinary command creates no fallback server"
fi
env -u TMUX -u TMUX_PANE -u TMUX_TMPDIR \
  "$REAL_TMUX" -L "$guard_ordinary_label" kill-server >/dev/null 2>&1 || true
guard_ordinary_label=""

# AIMED COMMANDS ARE THE POINT OF THE RULE, so they have to keep working — a
# guard that refused these would only teach agents to bypass it.
guard_out="$(guard_run "$guard_team_socket,1,0" - - -S "$guard_home/private-socket" kill-server)"
equal "an explicitly aimed kill-server runs" 0 "$(printf '%s' "$guard_out" | head -1)"
if guard_reached_tmux; then
  pass "and reaches the real tmux with its own socket"
else
  fail "and reaches the real tmux with its own socket" "the real tmux never ran"
fi
mkdir -p "$guard_home/sandbox"
if [ -d "$guard_home/sandbox" ]; then
  pass "the private TMUX_TMPDIR control has an existing root"
else
  fail "the private TMUX_TMPDIR control has an existing root" \
    "$guard_home/sandbox is not a directory"
fi
guard_out="$(guard_run - "$guard_home/sandbox" - kill-server)"
equal "a kill-server with TMUX unset and a private TMUX_TMPDIR runs" \
  0 "$(printf '%s' "$guard_out" | head -1)"
contains "an unprotected teardown fall-open is recorded" \
  "$(cat "$guard_state/tmux-guard.log" 2>/dev/null)" "fall-open-no-gangline-agent"
guard_unreadable_socket="$guard_home/unreadable-socket"
printf '%s\n' "$guard_unreadable_socket" > "$guard_unreadable_servers"
guard_out="$(guard_run - - - -S "$guard_unreadable_socket" kill-server)"
# TMUX JUST ANSWERED THE SOCKET-RESOLUTION PROBE, so this is a live server whose
# registrations cannot be classified, not an already-gone aimed socket. The old
# expectation let it fall open merely because the caller was outside a pane;
# that absence says nothing about whether the live server carries a team.
equal "a live server with unreadable registrations fails closed" \
  3 "$(printf '%s' "$guard_out" | head -1)"
contains "the unreadable live-server refusal is recorded" \
  "$(cat "$guard_state/tmux-guard.log" 2>/dev/null)" "refused-unreadable-server"
if guard_reached_tmux; then
  fail "the unreadable live server is not torn down" "the teardown reached tmux"
else
  pass "the unreadable live server is not torn down"
fi
guard_out="$(guard_run "$guard_team_socket,1,0" - - list-sessions)"
equal "a command that is not a teardown runs untouched" \
  0 "$(printf '%s' "$guard_out" | head -1)"
equal "and the guard says nothing about it" "" \
  "$(printf '%s' "$guard_out" | tail -n +2)"

# -L RESOLVES THROUGH TMUX_TMPDIR the same way tmux resolves it, so a label
# that names the team's own socket is the same command by another spelling.
guard_label_home="$guard_home/labelled"
mkdir -p "$guard_label_home/tmux-$guard_fake_uid"
guard_label_socket="$guard_label_home/tmux-$guard_fake_uid/team"
printf '%s\n' "$guard_label_socket" > "$guard_state/teams/guardteam"
printf '%s\n' "$guard_label_socket" >> "$guard_servers"
guard_out="$(guard_run - "$guard_label_home" - -L team kill-server)"
equal "a -L label resolving to the team's socket is refused" \
  3 "$(printf '%s' "$guard_out" | head -1)"
printf '%s\n' "$guard_team_socket" > "$guard_state/teams/guardteam"

# A FAILED -L PROBE DOES NOT MEAN THE PANE SOCKET. The selector overrides
# $TMUX, so substituting this pane's socket after the probe fails both names the
# wrong server and refuses an aimed command that tmux itself would reject as
# unreachable. The fake's final invocation supplies that ordinary error and is
# also the execution witness: a guard refusal never creates guard_ran.
guard_unreachable_label="unreachable-label-$$"
printf '%s\n' "$guard_unreachable_label" > "$guard_unreachable_labels"
guard_out="$(guard_run "$guard_team_socket,1,0" - - \
  -L "$guard_unreachable_label" kill-server)"
equal "an unreachable -L inside a pane reaches tmux for its ordinary error" \
  1 "$(printf '%s' "$guard_out" | head -1)"
contains "the unreachable -L error names the selected label" \
  "$guard_out" "$guard_unreachable_label"
if guard_reached_tmux; then
  pass "the unreachable -L reaches real tmux rather than the pane server"
else
  fail "the unreachable -L reaches real tmux rather than the pane server" \
    "the final -L invocation never reached tmux"
fi

# KILL-SESSION IS DECIDED BY WHICH SESSION IT LANDS ON. No target is the pane's
# own session, which is the team; another name on the same server is aimed
# somewhere real and runs, loudly, because the server is still the team's.
guard_out="$(guard_run "$guard_team_socket,1,0" - - kill-session)"
equal "a kill-session with no target is refused" 3 "$(printf '%s' "$guard_out" | head -1)"
contains "and says it would end the session the pane is in" "$guard_out" "names no target"
guard_out="$(guard_run "$guard_team_socket,1,0" - - kill-session -t =guardteam)"
equal "a kill-session naming the team itself is refused" \
  3 "$(printf '%s' "$guard_out" | head -1)"
guard_out="$(guard_run "$guard_team_socket,1,0" - - kill-session -t =gangtest-other)"
equal "a kill-session naming another session on that server runs" \
  0 "$(printf '%s' "$guard_out" | head -1)"
contains "and is loud about sharing the team's server" \
  "$guard_out" "same tmux server that holds team"

# A CALLER-SUPPLIED RECORD IS NO LONGER AUTHORITY. The agent registration on
# the reached server survives a different GANG_SESSION and still blocks it.
guard_session=unrecorded
guard_out="$(guard_run "$guard_team_socket,1,0" - - kill-server)"
guard_session=guardteam
equal "a team with no recorded socket is still protected" 3 "$(printf '%s' "$guard_out" | head -1)"
if guard_reached_tmux; then
  fail "and its teardown never reaches the real tmux" "the real tmux ran"
else
  pass "and its teardown never reaches the real tmux"
fi

# AN OVERRIDE IS EXPLICIT AND RECORDED. Nothing here is a security boundary; an
# agent that means to end its own server may say so, and saying so is written
# down beside the refusals.
: > "$guard_state/tmux-guard.log"
guard_out="$(guard_run "$guard_team_socket,1,0" - off kill-server)"
equal "an explicit override runs the teardown" 0 "$(printf '%s' "$guard_out" | head -1)"
contains "and the override is recorded like a refusal" \
  "$(cat "$guard_state/tmux-guard.log" 2>/dev/null)" "override"

# NO TMUX BEYOND THE SHIM IS A BROKEN INSTALL, said out loud rather than
# answered with a success nothing ran.
guard_bare="$guard_home/bare-bin"
mkdir -p "$guard_bare"
for guard_tool in date id dirname; do
  guard_where="$(command -v "$guard_tool")" \
    && ln -sf "$guard_where" "$guard_bare/$guard_tool"
done
guard_rc=0
guard_out="$(PATH="$ROOT/libexec/gang-tmux-guard:$guard_bare" GANG_SESSION=guardteam \
  GANG_LOCK_DIR="$guard_state" "$guard_shim" kill-server 2>&1)" || guard_rc=$?
equal "a PATH with no real tmux refuses rather than reporting success" 127 "$guard_rc"
contains "and says which shim was the only tmux it found" \
  "$guard_out" "no tmux on PATH beyond this shim"

# TWO GUARD DIRECTORIES ON PATH USED TO SELECT EACH OTHER FOREVER. An installed
# guard and a checkout guard both ahead of tmux left each shim excluding only
# its own directory, so each picked the other and re-execed without end; and
# because the real tmux is resolved before the command is dispatched, an
# ordinary read hung exactly as a teardown did. Both orders are driven, because
# the defect is symmetric and a fix that only skips what precedes it would pass
# one of them.
#
# BOUNDED BY CPU, NOT BY THE CLOCK. A shim that selects another shim is an
# unbounded exec loop, so this case needs a ceiling to reach a verdict rather
# than wedge the run. A CPU-second limit is not a wall-clock wait: resolving
# tmux is one PATH walk and one fork, microseconds of processor time however
# loaded the box is, while a loop re-entering that walk consumes it without
# bound and is killed at the limit. A shim killed there has execed no tmux at
# all, so the log the second assertion reads separates the two outcomes without
# depending on the status the first one names.
guard_dup_home="$guard_home/two-guards"
guard_dup_bin="$guard_dup_home/bin"
guard_dup_log="$guard_dup_home/ran"
mkdir -p "$guard_dup_bin" "$guard_dup_home/installed" "$guard_dup_home/checkout"
cp -R "$ROOT/libexec/gang-tmux-guard" "$guard_dup_home/installed/gang-tmux-guard"
cp -R "$ROOT/libexec/gang-tmux-guard" "$guard_dup_home/checkout/gang-tmux-guard"
printf '%s\n' '#!/bin/sh' "printf '%s\\n' \"\$*\" >> '$guard_dup_log'" 'exit 0' \
  > "$guard_dup_bin/tmux"
chmod +x "$guard_dup_bin/tmux"
for guard_first in installed checkout; do
  case "$guard_first" in
    installed) guard_second=checkout ;;
    *) guard_second=installed ;;
  esac
  : > "$guard_dup_log"
  guard_rc=0
  ( ulimit -t 1
    PATH="$guard_dup_home/$guard_first/gang-tmux-guard:$guard_dup_home/$guard_second/gang-tmux-guard:$guard_dup_bin:/usr/bin:/bin" \
      GANG_SESSION=guardteam GANG_LOCK_DIR="$guard_state" \
      exec "$guard_dup_home/$guard_first/gang-tmux-guard/tmux" list-sessions
  ) >/dev/null 2>&1 || guard_rc=$?
  equal "an ordinary command survives a second guard directory on PATH ($guard_first first)" \
    0 "$guard_rc"
  # source-guard: whole-surface@e19cd4ffadb8: the log is truncated immediately above, and the only executable named tmux behind the two guard directories on that PATH is the fake that writes it
  equal "and reaches the real tmux exactly once ($guard_first first)" \
    "list-sessions" "$(cat "$guard_dup_log" 2>/dev/null)"
done
