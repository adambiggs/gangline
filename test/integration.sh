#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Fast substrate contract checks. Every assertion reads state that already exists.
# Real-harness behavior belongs in a separately named disposable team.
set -euo pipefail

unset TMUX TMUX_PANE

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
GANG="$ROOT/bin/gang"

# A VERDICT IS ABOUT A TREE, so the tree has to hold still. This refuses to
# start against a working tree that is already moving — bash reads this script
# incrementally, gang re-reads collars and roles at hitch time, and an edit
# landing mid-run changes what executes. The identity is read again at the
# end, because starting settled is not the same as staying settled.
# test/gate.sh is the way to run this before a commit: it snapshots the
# working tree, uncommitted work included, and runs the gate from the copy.
TREE_AT_START="$("$ROOT/test/gate.sh" --assert-owned)"

RUN_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-test.XXXXXX")"
TMUX_SOCKET="$RUN_ROOT/tmux-$(id -u)/default"

export GIT_CONFIG_GLOBAL="$RUN_ROOT/gitconfig"
export GIT_CONFIG_SYSTEM=/dev/null

# The Bash fixture establishes every transition synchronously except one: a pane
# answers its terminal asynchronously. Production waits are inputs here, not
# evidence, so their clock returns immediately — but gang counts its patience in
# reads rather than seconds, and a stopped clock spends five of them in a few
# milliseconds, so a composer that has not echoed yet reads as one that never
# will.
#
# Gang's pane round-trip therefore keeps a floor, far below the duration it
# asked for. Without it, submission verifies only when the pasted payload is
# long enough to make each read slow enough for the echo to arrive: a fixture
# returning the right answer for a reason unrelated to the behaviour under test,
# which inverted the moment the pasted startup contract became a pointer. Keying
# on the requested duration leaves boot and churn clocks immediate, and no
# assertion depends on the floor's value.
#
# THE MARGIN, MEASURED 2026-08-09 on the deployed harness build rather than reasoned
# about, because the reasoning above is what a reader would otherwise have to
# re-derive. submit_verify presses Enter and reads the box up to five times,
# napping 0.4s between reads.
#
#   hermetic fixture pane, Enter to box change  ~5ms (re-measured 2026-08-12)
#   test budget (this floor)     5 x 0.05s = 0.25s
#   production budget            5 x 0.4s  = 2.0s
#
# THE FIRST NUMBER IS A PROPERTY OF THE PANE, NOT OF THE BOX, and leaving that
# implicit is what made this margin unreadable. A fixture shell that reads the
# operator's /etc/bash.bashrc answers the same Enter in ~180ms instead, because
# Debian installs a command_not_found_handle there that runs a Python program
# against a multi-megabyte apt database, and every envelope this suite delivers
# is an unrunnable command — so that handler sits on the Enter path of every
# submission gang verifies. Two fixture collars launched such a shell, spent
# most of this budget before anything had gone wrong, and starved at load 1.06.
# test/lint.sh keeps fixture shells hermetic; the ~5ms above holds only while
# it does.
#
# So the quiet margin is about 50x and looks unbreakable right up until an
# I/O-starved box stretches one round trip past 250ms. Under sustained load —
# CPU spinners plus dd+sync loops, not CPU bursts, which do not reproduce it —
# the same hitch failed 39 times in 40. UNDER THE SAME LOAD, PRODUCTION TIMING
# FAILED 0 IN 20: gang's real delivery survives a box this busy, and only the
# compressed clock does not. That is the more valuable half of the measurement
# and the reason this floor is a test artifact rather than a substrate finding.
#
# AN EVENT BARRIER WAS CONSIDERED AND REFUSED, so the next reader does not
# spend a day rediscovering it. The suite owns this shim, so the shim looks
# like the place to block on pane output instead of napping — with a latched
# tmux wait-for signalled from the fixture's PROMPT_COMMAND. Two things kill
# it. The shim is global to every 0.3/0.4 nap in gang and most of them are not
# waiting on a pane echo, so a blanket barrier deadlocks the rest. And tmux
# wait-for has no timeout, so an unsignalled channel converts a failing test
# into a HANGING suite, which is strictly worse than the flake it replaces.
# Raising the floor is not an option either: a test that passes by waiting
# longer on a slow box reports the box, not the tree.
mkdir -p "$RUN_ROOT/bin"
cat > "$RUN_ROOT/bin/sleep" <<'SH'
#!/bin/sh
case "$1" in
  0.3|0.4) exec /bin/sleep 0.05 ;;
esac
exit 0
SH
chmod +x "$RUN_ROOT/bin/sleep"
PATH="$RUN_ROOT/bin:$PATH"
export PATH

# AN UNVERIFIABLE SUBMISSION IS AN UNKNOWN, AND AN UNKNOWN COSTS ONE ASSERTION.
# The floor above is a race and a starved box can lose it: measured, the same
# setup hitch fails 39 times in 40 under sustained I/O load. That failure
# reaches gang through `die`, so `set -e` used to end the whole run — one
# starved paste cost every remaining check AND the summary, which is the one
# reading nobody can act on, on exactly the busy box the suite exists to speak
# about.
#
# Only SETUP hitches route through this: the ones whose output no assertion
# reads. Anything that inspects what hitch printed, or that means to watch a
# hitch fail, still calls gang directly and still ends the run, so no guard
# here has been softened. The retry is a second OBSERVATION, not a longer wait
# — a real defect fails both, while a starved pane usually loses only once —
# and the unknown is recorded either way so a quiet green and a green bought
# under load never read the same.
cat > "$RUN_ROOT/bin/hitch-guard" <<SH
#!/bin/sh
REAL="$GANG"
UNKNOWNS="$RUN_ROOT/unknowns"
ERR="$RUN_ROOT/hitch-stderr"
SH
cat >> "$RUN_ROOT/bin/hitch-guard" <<'SH'
rc=0
"$REAL" hitch "$@" 2>"$ERR" || rc=$?
if [ "$rc" -ne 0 ] \
  && grep -q 'submit NOT verified\|submission unverifiable' "$ERR"; then
  printf '%s\n' "$1" >> "$UNKNOWNS"
  "$REAL" drop "$1" >/dev/null 2>&1
  rc=0
  "$REAL" hitch "$@" 2>"$ERR" || rc=$?
fi
cat "$ERR" >&2
exit "$rc"
SH
chmod +x "$RUN_ROOT/bin/hitch-guard"
HITCH="$RUN_ROOT/bin/hitch-guard"

# A run that ends early still owes a reading. One retry rescues an ISOLATED
# starved submission, which is the case that used to cost every later check;
# it cannot rescue a box starving nearly all of them, and pretending otherwise
# would just be the floor raised by another name. So the second consecutive
# starve still ends the run — but it ends it OUT LOUD, naming the coverage
# reached and the reason, because a verdict nobody can read is what made the
# original failure expensive.
summary_printed=0
cleanup() {
  if [ "$summary_printed" -eq 0 ]; then
    printf '\nRUN ENDED EARLY after %s checks in %ss — no verdict on the rest.\n' \
      "$checks" "$SECONDS"
    if [ -s "$RUN_ROOT/unknowns" ]; then
      printf 'Gang could not verify these setup submissions: %s\n' \
        "$(tr '\n' ' ' < "$RUN_ROOT/unknowns")"
      printf 'Each was retried once and starved again, against the shim\n'
      printf 'above: five box readings, 0.05s apart, for one to differ. That\n'
      printf 'budget was missed. This run measured no cause for it and names\n'
      printf 'none.\n'
    fi
  fi
  tmux -S "$TMUX_SOCKET" kill-server 2>/dev/null || true
  rm -rf -- "$RUN_ROOT"
}
trap cleanup EXIT HUP INT TERM

export TMUX_TMPDIR="$RUN_ROOT"
export GANG_CONFIG_DIR="$RUN_ROOT/config"
export GANG_SESSION="gangtest-$$"
export GANG_TEST_COLLARS=1
export GANG_CHURN_WAIT=0
export GANG_LOCK_DIR="$RUN_ROOT/locks"
export GANG_ARCHIVE_DIR="$RUN_ROOT/archive"

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

submitted() { # $1 description, $2 agent
  equal "$1" "" "$("$GANG" composer "$2")"
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

mkdir -p "$RUN_ROOT/no-utf8-bin"
cat > "$RUN_ROOT/no-utf8-bin/locale" <<'SH'
#!/bin/sh
exit 1
SH
chmod +x "$RUN_ROOT/no-utf8-bin/locale"
refuses "startup refuses when no UTF-8 locale can be established" \
  "could not establish a UTF-8 locale" \
  env -u LC_ALL -u LC_CTYPE LANG=C PATH="$RUN_ROOT/no-utf8-bin:$PATH" \
    "$GANG" config

bare_window_name() { # $1 raw tmux window name
  local n="$1" first last
  [ "${#n}" -ge 3 ] || { printf '%s' "$n"; return; }
  first="${n:0:1}"; last="${n: -1}"
  case "$first" in
    -|'~'|'!'|'?') [ "$first" = "$last" ] && n="${n:1:${#n}-2}" ;;
  esac
  printf '%s' "$n"
}

window_id_in() { # $1 session, $2 bare window name
  local id name
  while read -r id name; do
    [ "$(bare_window_name "$name")" = "$2" ] && { printf '%s' "$id"; return; }
  done < <(tmux list-windows -t "=$1" -F '#{window_id} #{window_name}')
  return 1
}

window_id() { # $1 bare window name
  window_id_in "$GANG_SESSION" "$1"
}

window_names() { # optional $1 session -> bare names, one per line
  local session="${1:-$GANG_SESSION}" name
  while IFS= read -r name; do bare_window_name "$name"; printf '\n'; done \
    < <(tmux list-windows -t "=$session" -F '#W')
}

pane() { tmux capture-pane -pJ -t "$(window_id "$1")"; }
pane_all() { tmux capture-pane -pJ -S - -t "$(window_id "$1")"; }

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
} | awk '$0 != "hook" && $0 != "spawn" && $0 != "profiles" && $0 != "cutoff" && $0 != "-h" && $0 != "--help" && $0 != "help"' | sort -u)"
bare_error_commands="hitch adopt send flush mail interrupt compact context usage status capture composer whoami drop down"
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
excludes "top-level help omits the deprecated profiles name" "$top_help" "profiles"
excludes "top-level help omits the deprecated cutoff name" "$top_help" "cutoff"
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
"$GANG" profiles > "$RUN_ROOT/profiles-alias.out" 2> "$RUN_ROOT/profiles-alias.err"
equal "gang profiles aliases the collar list on stdout" \
  "$("$GANG" collars)" "$(<"$RUN_ROOT/profiles-alias.out")"
contains "gang profiles announces only on stderr" \
  "$(<"$RUN_ROOT/profiles-alias.err")" \
  "gang profiles is now gang collars; the old name still works and will be removed in 2.0."
excludes "the gang profiles announcement does not contaminate stdout" \
  "$(<"$RUN_ROOT/profiles-alias.out")" "removed in 2.0"
env -u GANG_TEST_COLLARS GANG_TEST_PROFILES=1 "$GANG" collars \
  > "$RUN_ROOT/test-profiles-alias.out" 2> "$RUN_ROOT/test-profiles-alias.err"
contains "the suite switch alias still exposes its concrete fixture" \
  "$(<"$RUN_ROOT/test-profiles-alias.out")" "bash"
contains "the suite switch alias announces its replacement" \
  "$(<"$RUN_ROOT/test-profiles-alias.err")" \
  "GANG_TEST_PROFILES is now GANG_TEST_COLLARS"

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

# Published config names stay live for 1.x, but two names for one setting are
# never silently normalized. Exercise every reachable layer arrangement in
# both directions with identical values; an unequal-only conflict check cannot
# satisfy these fixtures.
config_alias_root="$CONFIG_CASES/aliases"
mkdir -p "$config_alias_root"
config_pair_case() { # $1 new, $2 old, $3 arrangement
  local new="$1" old="$2" arrangement="$3" root="$config_alias_root/$1-$3" out=""
  mkdir -p "$root"
  case "$arrangement" in
    file-new-old)
      printf '%s\n' "$new=same" "$old=same" > "$root/config"
      out="$(env -u "$new" -u "$old" GANG_CONFIG_DIR="$root" "$GANG" config 2>&1 || true)" ;;
    file-old-new)
      printf '%s\n' "$old=same" "$new=same" > "$root/config"
      out="$(env -u "$new" -u "$old" GANG_CONFIG_DIR="$root" "$GANG" config 2>&1 || true)" ;;
    env-both)
      out="$(env -u "$new" -u "$old" "$new=same" "$old=same" \
        GANG_CONFIG_DIR="$root" "$GANG" config 2>&1 || true)" ;;
    env-new-file-old)
      printf '%s\n' "$old=same" > "$root/config"
      out="$(env -u "$new" -u "$old" "$new=same" GANG_CONFIG_DIR="$root" \
        "$GANG" config 2>&1 || true)" ;;
    env-old-file-new)
      printf '%s\n' "$new=same" > "$root/config"
      out="$(env -u "$new" -u "$old" "$old=same" GANG_CONFIG_DIR="$root" \
        "$GANG" config 2>&1 || true)" ;;
  esac
  contains "$new/$old $arrangement refuses both names" "$out" \
    "$new"
  contains "$new/$old $arrangement names the deprecated alias" "$out" \
    "deprecated alias $old"
  contains "$new/$old $arrangement is a two-name refusal" "$out" \
    "both set"
}
for config_pair in 'GANG_COLLAR GANG_PROFILE' 'GANG_COLLARS GANG_PROFILES'; do
  read -r config_new config_old <<<"$config_pair"
  for config_arrangement in file-new-old file-old-new env-both \
    env-new-file-old env-old-file-new; do
    config_pair_case "$config_new" "$config_old" "$config_arrangement"
  done
  differing_alias_out="$(env -u "$config_new" -u "$config_old" \
    "$config_new=new-value" "$config_old=old-value" \
    GANG_CONFIG_DIR="$config_alias_root/differing-$config_new" \
    "$GANG" config 2>&1 || true)"
  contains "$config_new/$config_old differing values still refuse" \
    "$differing_alias_out" "both set"
done
empty_alias_out="$(env -u GANG_COLLAR -u GANG_PROFILE GANG_COLLAR= \
  GANG_PROFILE= GANG_CONFIG_DIR="$config_alias_root/empty-env" \
  "$GANG" config 2>&1 || true)"
contains "empty-but-set config aliases still conflict" "$empty_alias_out" \
  "GANG_COLLAR"
contains "the empty-but-set conflict names the old spelling" "$empty_alias_out" \
  "GANG_PROFILE"

old_config_root="$config_alias_root/old-file"
mkdir -p "$old_config_root"
printf '%s\n' 'GANG_PROFILE=bash' > "$old_config_root/config"
env -u GANG_COLLAR -u GANG_PROFILE GANG_CONFIG_DIR="$old_config_root" \
  "$GANG" config > "$old_config_root/out" 2> "$old_config_root/err"
contains "gang config prints only the new key for an old file spelling" \
  "$(<"$old_config_root/out")" $'GANG_COLLAR=bash\t'
contains "gang config identifies the alias in the origin column" \
  "$(<"$old_config_root/out")" "deprecated alias GANG_PROFILE"
contains "a file config alias announces its replacement" \
  "$(<"$old_config_root/err")" \
  "GANG_PROFILE is now GANG_COLLAR (from $old_config_root/config line 1)"

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
refuses "the old suite-only collar switch has its own unpersistable refusal" \
  "GANG_TEST_PROFILES is a per-invocation switch" \
  env GANG_CONFIG_DIR="$CONFIG_CASES/test-switch" "$GANG" collars
refuses "both suite switch spellings in the environment conflict" \
  "GANG_TEST_COLLARS" env GANG_TEST_COLLARS=1 GANG_TEST_PROFILES=1 \
  GANG_CONFIG_DIR="$CONFIG_CASES/env" "$GANG" config

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
claude_external_record="${claude_external_dialog%%$'\n--\n'*}"
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
{ printf 'plain %s\n' 1 2 3 4 5 6 7 8 9; printf '%s' 'plain 10'; } \
  > "$claude_box_dir/absent"
claude_box_session="claude-box-$$"
tmux new-session -d -s "$claude_box_session" -n clipped -x 40 -y 10 \
  "cat '$claude_box_dir/clipped'; cat"
for claude_box_case in clipped-pair whole absent; do
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

# Real tmux substrate: lifecycle, observation, verified attributed delivery and
# exact-name addressing. Gangline's command returns only after the state checked
# below has been established.
printf 'unset BASHPID\n' > "$RUN_ROOT/no-bashpid"
BASH_ENV="$RUN_ROOT/no-bashpid" \
  "$HITCH" alpha -c bash -d /tmp >/dev/null
excludes "a collar with no launch choices emits no impossible-choice warning" \
  "$(<"$RUN_ROOT/hitch-stderr")" "hitching 'alpha' without"
alpha_id="$(window_id alpha)"
# Keep the Bash stand-in's composer immediately observable for ordinary
# doctrine-sized contracts while leaving the explicit pane-overflow fixture far
# beyond the grid below.
tmux resize-window -t "$alpha_id" -x 80 -y 30
equal "a hitched first turn writes the raw busy window glyph" \
  "-alpha-" "$(tmux display-message -p -t "$alpha_id" '#{window_name}')"
contains "Bash 3.2 can lock and deliver the startup contract" \
  "$(pane alpha)" "You are alpha in Gangline"
contains "hitch creates an observable idle agent" "$($GANG status alpha)" "~idle~"
equal "an idle observation writes the raw slack window glyph" \
  "~alpha~" "$(tmux display-message -p -t "$alpha_id" '#{window_name}')"
contains "roster lists the hitched collar" "$($GANG roster)" "alpha"
contains "roster is an immediate snapshot" \
  "$(GANG_CHURN_WAIT=not-a-duration $GANG roster)" "alpha"

alpha_tmux_pane="$(tmux list-panes -t "$alpha_id" -F '#{pane_id}')"
contains "whoami prints the collar field under its 1.0 name" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" whoami)" "collar: bash"
contains "bare status targets the calling agent window" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" status)" "~idle~"
contains "bare capture targets the calling agent window" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" capture)" \
  "You are alpha in Gangline"
equal "bare composer targets the calling agent window" "" \
  "$(TMUX_PANE="$alpha_tmux_pane" "$GANG" composer)"

"$GANG" hitch legacy-flag -p bash -d /tmp \
  > "$RUN_ROOT/legacy-flag.out" 2> "$RUN_ROOT/legacy-flag.err"
contains "the hitch flag alias still opens a registered window" \
  "$(TMUX_PANE="$(tmux list-panes -t "$(window_id legacy-flag)" -F '#{pane_id}')" \
    "$GANG" whoami)" "collar: bash"
contains "the hitch flag alias announces its replacement" \
  "$(<"$RUN_ROOT/legacy-flag.err")" \
  "hitch: -p/--profile is now -c/--collar"
"$GANG" drop legacy-flag >/dev/null
if both_flag_out="$("$GANG" hitch two-flags -c bash -p bash -d /tmp 2>&1)"; then
  fail "two spellings of the hitch collar flag refuse" "hitch succeeded"
else
  contains "the two-spelling refusal names both flags" "$both_flag_out" \
    "-c/--collar and -p/--profile"
fi
excludes "the two-spelling refusal opens no window" "$(window_names)" "two-flags"

# Calibrate custom-directory precedence against a state with no shadow file,
# then make the custom bash collar emit a marker the shipped fixture cannot.
shadow_dir="$RUN_ROOT/shadow-collars"
mkdir -p "$shadow_dir"
excludes "the shipped bash collar cannot emit the shadow marker" \
  "$(GANG_COLLARS="$shadow_dir" "$GANG" composer alpha)" "CUSTOM_COLLAR_MARKER"
cat > "$shadow_dir/bash.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
_shadow_real="\$(declare -f collar_input)"
eval "shadow_real_input \${_shadow_real#collar_input}"
collar_input() {
  [ ! -e "$RUN_ROOT/shadow-marker-on" ] || { printf 'CUSTOM_COLLAR_MARKER'; return; }
  shadow_real_input "\$1"
}
SH
GANG_COLLARS="$shadow_dir" "$HITCH" shadowed -c bash -d /tmp >/dev/null
: > "$RUN_ROOT/shadow-marker-on"
equal "a custom collar directory shadows the shipped collar" \
  "CUSTOM_COLLAR_MARKER" \
  "$(GANG_COLLARS="$shadow_dir" "$GANG" composer shadowed)"
"$GANG" drop shadowed >/dev/null

# Custom collars may keep the published 0.x function spelling through 1.x.
# The ordinary command calibrates that there is a real announcement for the
# hook-silence check below.
cat > "$RUN_ROOT/collars/legacy-contract.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
_legacy_input="\$(declare -f collar_input)"
eval "profile_input \${_legacy_input#collar_input}"
unset -f collar_input
SH
cat > "$RUN_ROOT/collars/dual-contract.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
profile_input() { printf 'legacy'; }
SH
"$GANG" hitch legacy-contract-a -c legacy-contract -d /tmp \
  > "$RUN_ROOT/legacy-contract-hitch.out" 2> "$RUN_ROOT/legacy-contract-hitch.err"
contains "a legacy collar function is announced during ordinary loading" \
  "$(<"$RUN_ROOT/legacy-contract-hitch.err")" \
  "collar 'legacy-contract' declares profile_input"
tmux send-keys -l -t "$(window_id legacy-contract-a)" LEGACY_INPUT
"$GANG" composer legacy-contract-a \
  > "$RUN_ROOT/legacy-contract-composer.out" \
  2> "$RUN_ROOT/legacy-contract-composer.err"
contains "profile_input forwards into the collar_input contract" \
  "$(<"$RUN_ROOT/legacy-contract-composer.out")" "LEGACY_INPUT"
contains "an ordinary composer command announces the legacy contract" \
  "$(<"$RUN_ROOT/legacy-contract-composer.err")" \
  "collar 'legacy-contract' declares profile_input"
tmux send-keys -t "$(window_id legacy-contract-a)" C-u
printf '%s\n' '{"hook_event_name":"Notification","notification_type":"fixture"}' \
  | TMUX_PANE="$(tmux list-panes -t "$(window_id legacy-contract-a)" -F '#{pane_id}')" \
    "$GANG" hook > "$RUN_ROOT/legacy-contract-hook.out" \
    2> "$RUN_ROOT/legacy-contract-hook.err"
equal "hook output stays silent for a legacy contract function" "" \
  "$(<"$RUN_ROOT/legacy-contract-hook.err")"
if dual_out="$("$GANG" hitch dual-contract -c dual-contract -d /tmp 2>&1)"; then
  fail "a collar defining both contract spellings refuses" "hitch succeeded"
else
  contains "the dual-contract refusal names its collar" "$dual_out" \
    "collar 'dual-contract' defines both collar_input and profile_input"
fi
excludes "the dual-contract refusal opens no window" "$(window_names)" "dual-contract"

"$GANG" hitch legacy-contract-b -c legacy-contract -d /tmp >/dev/null 2>&1
"$GANG" roster > "$RUN_ROOT/legacy-roster.out" 2> "$RUN_ROOT/legacy-roster.err"
contains "a roster over shared legacy collars announces at least once" \
  "$(<"$RUN_ROOT/legacy-roster.err")" \
  "collar 'legacy-contract' declares profile_input"
new_roster_session="gangtest-new-contract-$$"
GANG_SESSION="$new_roster_session" "$HITCH" new-contract-a -c bash -d /tmp >/dev/null
GANG_SESSION="$new_roster_session" "$HITCH" new-contract-b -c bash -d /tmp >/dev/null
GANG_SESSION="$new_roster_session" "$GANG" roster \
  > "$RUN_ROOT/new-roster.out" 2> "$RUN_ROOT/new-roster.err"
excludes "new contract collars calibrate the roster announcement" \
  "$(<"$RUN_ROOT/new-roster.err")" "declares profile_input"
GANG_SESSION="$new_roster_session" "$GANG" down "$new_roster_session" >/dev/null

# A running pre-rename window is healed by the hook itself, with no stderr
# announcement: internal tmux residue is Gangline's state, not operator input.
"$HITCH" legacy-option -c bash -d /tmp >/dev/null
legacy_option_id="$(window_id legacy-option)"
legacy_option_pane="$(tmux list-panes -t "$legacy_option_id" -F '#{pane_id}')"
tmux set-option -w -t "$legacy_option_id" @gl_profile bash
tmux set-option -uw -t "$legacy_option_id" @gl_collar
printf '%s\n' '{"hook_event_name":"UserPromptSubmit"}' \
  | TMUX_PANE="$legacy_option_pane" "$GANG" hook \
    > "$RUN_ROOT/legacy-option-hook.out" 2> "$RUN_ROOT/legacy-option-hook.err"
equal "a hook over @gl_profile residue is byte-silent on stderr" "" \
  "$(<"$RUN_ROOT/legacy-option-hook.err")"
contains "the residue-reading hook still writes the turn bracket" \
  "$(tmux show-options -wqv -t "$legacy_option_id" @gl_turn)" "open "
equal "the hook migrates the collar value before erasing residue" "bash" \
  "$(tmux show-options -wqv -t "$legacy_option_id" @gl_collar)"
equal "the migrated old window option is removed" "" \
  "$(tmux show-options -wqv -t "$legacy_option_id" @gl_profile)"
tmux set-option -w -t "$legacy_option_id" @gl_profile bash
contains "equal old and new window options resolve normally" \
  "$("$GANG" status legacy-option)" "-busy-"
tmux set-option -w -t "$legacy_option_id" @gl_profile codex
refuses "unequal old and new window options refuse without guessing" \
  "carries @gl_collar 'bash' and @gl_profile 'codex'" \
  "$GANG" status legacy-option
tmux set-option -uw -t "$legacy_option_id" @gl_profile

# Calibrate the migration-failure instrument before trusting its result. The
# failed argv is the positive witness that migration was attempted; old-state
# survival alone would also pass an implementation that never tried.
migration_bin="$RUN_ROOT/migration-bin"
migration_record="$RUN_ROOT/migration-tmux.argv"
mkdir -p "$migration_bin"
real_tmux="$(command -v tmux)"
cat > "$migration_bin/tmux" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
real=$(printf '%q' "$real_tmux")
record="\${TMUX_MIGRATION_RECORD:?}"
if [ "\${1:-}" = set-option ]; then
  for migration_arg in "\$@"; do
    if [ "\$migration_arg" = @gl_collar ]; then
      first=1
      for recorded_arg in "\$@"; do
        [ "\$first" -eq 1 ] || printf '\t' >> "\$record"
        printf '%s' "\$recorded_arg" >> "\$record"
        first=0
      done
      printf '\n' >> "\$record"
      exit 97
    fi
  done
fi
exec "\$real" "\$@"
SH
chmod +x "$migration_bin/tmux"
: > "$migration_record"
if TMUX_MIGRATION_RECORD="$migration_record" "$migration_bin/tmux" \
  set-option -w -t "$legacy_option_id" @gl_collar bash; then
  fail "the migration wrapper's fail branch is calibrated" "intercept succeeded"
else
  pass "the migration wrapper's fail branch is calibrated"
fi
contains "the calibrated fail branch records its exact argv" \
  "$(<"$migration_record")" \
  $'set-option\t-w\t-t\t'"$legacy_option_id"$'\t@gl_collar\tbash'
TMUX_MIGRATION_RECORD="$migration_record" "$migration_bin/tmux" \
  set-option -w -t "$legacy_option_id" @gl_migration_forwarded yes
equal "the migration wrapper forwards its unrelated branch" "yes" \
  "$(tmux show-options -wqv -t "$legacy_option_id" @gl_migration_forwarded)"
tmux set-option -uw -t "$legacy_option_id" @gl_migration_forwarded

: > "$migration_record"
tmux set-option -w -t "$legacy_option_id" @gl_profile bash
tmux set-option -uw -t "$legacy_option_id" @gl_collar
TMUX_MIGRATION_RECORD="$migration_record" PATH="$migration_bin:$PATH" \
  "$GANG" status legacy-option >/dev/null
contains "status attempted the intercepted @gl_collar migration" \
  "$(<"$migration_record")" \
  $'set-option\t-w\t-t\t'"$legacy_option_id"$'\t@gl_collar\tbash'
equal "a failed migration preserves the only old collar identity" "bash" \
  "$(tmux show-options -wqv -t "$legacy_option_id" @gl_profile)"
equal "the failed intercepted migration did not fabricate a new value" "" \
  "$(tmux show-options -wqv -t "$legacy_option_id" @gl_collar)"
tmux set-option -w -t "$legacy_option_id" @gl_collar bash
tmux set-option -uw -t "$legacy_option_id" @gl_profile
migration_wrapper_sha="$(shasum -a 256 "$migration_bin/tmux" | awk '{print $1}')"
sleep_stub_sha="$(shasum -a 256 "$RUN_ROOT/bin/sleep" | awk '{print $1}')"
printf 'instrument tmux=%s sha256=%s sleep=%s sha256=%s spec-sha256=%s\n' \
  "$migration_bin/tmux" "$migration_wrapper_sha" "$RUN_ROOT/bin/sleep" \
  "$sleep_stub_sha" \
  '52a740a3954c18b82f2b4461d92a38dfb17d8e1655d4e5796c4d0e07f97ac995'

GANG_CONTEXT_LIGHTS=off "$HITCH" ctx-agent -c ctx-known -d /tmp >/dev/null
ctx_agent_id="$(window_id ctx-agent)"
ctx_agent_pane="$(tmux list-panes -t "$ctx_agent_id" -F '#{pane_id}')"
equal "named context prints the collar reading byte-for-byte" \
  "42k/200k (21%)" "$("$GANG" context ctx-agent)"
equal "bare context targets the calling agent window" \
  "42k/200k (21%)" "$(TMUX_PANE="$ctx_agent_pane" "$GANG" context)"
equal "context answers while context lights are off" \
  "42k/200k (21%)" "$(GANG_CONTEXT_LIGHTS=off "$GANG" context ctx-agent)"
"$HITCH" ctx-failing -c ctx-fail -d /tmp >/dev/null
excludes "roster carries no context reading column" \
  "$("$GANG" roster)" "42k/200k"

ctx_fail_stdout="$RUN_ROOT/context-fail.stdout"
ctx_fail_stderr="$RUN_ROOT/context-fail.stderr"
if "$GANG" context ctx-failing >"$ctx_fail_stdout" 2>"$ctx_fail_stderr"; then
  fail "a collar context failure stays non-zero" "context unexpectedly succeeded"
else
  pass "a collar context failure stays non-zero"
fi
equal "a collar context failure fabricates no reading" "" "$(<"$ctx_fail_stdout")"
contains "a collar context failure keeps its own diagnostic" \
  "$(<"$ctx_fail_stderr")" "fixture context unavailable"

"$HITCH" ctx-missing -c ctx-none -d /tmp >/dev/null
refuses "a missing collar_context names the collar" \
  "collar 'ctx-none' declares no collar_context" \
  "$GANG" context ctx-missing

GANG_CONTEXT_LIGHTS=off "$HITCH" usage-inline -c usage-inline -d /tmp >/dev/null
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

"$HITCH" usage-modal -c usage-modal -d /tmp >/dev/null
usage_modal_out="$("$GANG" usage usage-modal)"
equal "modal usage returns the visible page raw" \
  $'MODAL_ONE\nMODAL_TWO' \
  "$(printf '%s\n' "$usage_modal_out" | sed 's/[[:space:]]*$//')"
equal "modal usage dismisses back to an empty composer" "" \
  "$("$GANG" composer usage-modal)"

"$HITCH" usage-confirm -c usage-confirm -d /tmp >/dev/null
usage_confirm_out="$("$GANG" usage usage-confirm)"
equal "usage presses the collar's confirmation key before capture" \
  "CONFIRMED_USAGE" \
  "$(printf '%s\n' "$usage_confirm_out" | sed 's/[[:space:]]*$//')"
equal "confirmed modal usage restores an empty composer" "" \
  "$("$GANG" composer usage-confirm)"

"$HITCH" usage-stuck -c usage-stuck -d /tmp >/dev/null
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

"$HITCH" usage-occupied -c usage-occupied -d /tmp >/dev/null
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

refuses "a collar with no GANG_USAGE_CMD refuses usage" \
  "declares no GANG_USAGE_CMD" "$GANG" usage alpha

# A SCREEN THAT DIFFERS BECAUSE GANG JUST TYPED INTO IT IS NOT A USAGE SCREEN.
# Until this fixture existed, gang pressed Enter and compared the very next
# capture, so whichever of two states that round trip caught decided which
# refusal came out: the shell finishing first gave "never changed after clear",
# gang looking first gave a rollover complaint about a scrollback that had not
# rolled over. Measured at two wrong refusals in twenty-five under sustained
# load, none in twenty quiet. Here the command never completes, so the losing
# state is permanent and the verdict is the same every time.
"$HITCH" usage-hold -c usage-hold -d /tmp >/dev/null
usage_hold_id="$(window_id usage-hold)"
usage_hold_ready="test-usage-hold-ready-$$"
usage_hold_marker="READY_USAGE_HOLD_$$"
printf -v usage_hold_cmd 'PROMPT_COMMAND=; PS1=%q""%q; clear' \
  "READY_USAGE_" "HOLD_$$❯ "
printf -v usage_hold_pipe \
  'needle=%q; event=%q; seen=; while IFS= read -r -n 1 char; do seen="${seen}${char}"; case "$seen" in *"$needle") tmux wait-for -S "$event"; exit 0;; esac; if [ "${#seen}" -gt 256 ]; then seen="${seen: -256}"; fi; done' \
  "$usage_hold_marker❯ " "$usage_hold_ready"
printf -v usage_hold_pipe_shell '%q' "$usage_hold_pipe"
tmux pipe-pane -O -t "$usage_hold_id" "bash -c $usage_hold_pipe_shell"
tmux send-keys -l -t "$usage_hold_id" "$usage_hold_cmd"
tmux send-keys -t "$usage_hold_id" Enter
tmux wait-for "$usage_hold_ready"
tmux pipe-pane -t "$usage_hold_id"
usage_hold_stdout="$RUN_ROOT/usage-hold.stdout"
usage_hold_stderr="$RUN_ROOT/usage-hold.stderr"
if "$GANG" usage usage-hold \
    >"$usage_hold_stdout" 2>"$usage_hold_stderr"; then
  fail "usage refuses a command the harness has not taken" \
    "usage unexpectedly succeeded"
else
  pass "usage refuses a command the harness has not taken"
fi
contains "an unconsumed command is named as an unverified submission" \
  "$(<"$usage_hold_stderr")" "submit NOT verified"
excludes "and is not reported as a scrollback that rolled over" \
  "$(<"$usage_hold_stderr")" "rolled over"
equal "an unconsumed command yields no usage content" "" \
  "$(<"$usage_hold_stdout")"
"$GANG" drop usage-hold >/dev/null

"$HITCH" usage-nochange -c usage-nochange -d /tmp >/dev/null
usage_nochange_id="$(window_id usage-nochange)"
usage_nochange_ready="test-usage-nochange-ready-$$"
usage_nochange_marker="READY_USAGE_NOCHANGE_$$"
printf -v usage_nochange_cmd 'PROMPT_COMMAND=; PS1=%q""%q; clear' \
  "READY_USAGE_" "NOCHANGE_$$❯ "
printf -v usage_nochange_pipe \
  'needle=%q; event=%q; seen=; while IFS= read -r -n 1 char; do seen="${seen}${char}"; case "$seen" in *"$needle") tmux wait-for -S "$event"; exit 0;; esac; if [ "${#seen}" -gt 256 ]; then seen="${seen: -256}"; fi; done' \
  "$usage_nochange_marker❯ " "$usage_nochange_ready"
printf -v usage_nochange_pipe_shell '%q' "$usage_nochange_pipe"
tmux pipe-pane -O -t "$usage_nochange_id" \
  "bash -c $usage_nochange_pipe_shell"
tmux send-keys -l -t "$usage_nochange_id" "$usage_nochange_cmd"
tmux send-keys -t "$usage_nochange_id" Enter
tmux wait-for "$usage_nochange_ready"
tmux pipe-pane -t "$usage_nochange_id"
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
"$HITCH" usage-rollover -c usage-inline -d /tmp >/dev/null
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

"$HITCH" usage-unknown -c usage-unknown -d /tmp >/dev/null
refuses "usage refuses an unknown render declaration" \
  "unknown GANG_USAGE_RENDER 'unknown'" "$GANG" usage usage-unknown

alpha_before_bare_help="$(pane alpha)"
alpha_composer_before_bare_help="$($GANG composer alpha)"
for incoherent_bare in hitch adopt send drop; do
  if incoherent_output="$(TMUX_PANE="$alpha_tmux_pane" "$GANG" "$incoherent_bare" 2>&1)"; then
    fail "bare gang $incoherent_bare refuses inside an agent" \
      "command unexpectedly succeeded: [$incoherent_output]"
  else
    contains "bare gang $incoherent_bare prints help inside an agent" \
      "$incoherent_output" "gang $incoherent_bare"
  fi
done
# A COMMAND THAT TAKES ONE AGENT NAME DEFAULTS TO THE CALLER. Bare flush,
# interrupt and usage inside an agent window therefore resolve to that agent
# rather than printing a synopsis, and each then refuses on something the
# fixture's own collar does not declare — which is the evidence that a target
# was resolved at all. drop, down, adopt, hitch and send's recipient stay
# without a self default on purpose and remain in the loop above.
for self_bare in flush:GANG_QUEUED_REGEX interrupt:GANG_INTERRUPT_KEY usage:GANG_USAGE_CMD; do
  self_bare_cmd="${self_bare%%:*}"
  self_bare_decl="${self_bare#*:}"
  if self_bare_out="$(TMUX_PANE="$alpha_tmux_pane" "$GANG" "$self_bare_cmd" 2>&1)"; then
    fail "bare gang $self_bare_cmd targets the calling agent" \
      "command unexpectedly succeeded: [$self_bare_out]"
  else
    contains "bare gang $self_bare_cmd targets the calling agent" \
      "$self_bare_out" "declares no $self_bare_decl"
    excludes "bare gang $self_bare_cmd prints no synopsis inside an agent" \
      "$self_bare_out" "gang $self_bare_cmd"
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

for meaningful_command in roster collars config curfew notify; do
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
cp -R "$ROOT/collars" "$installed_root/collars"
cp "$GANG" "$installed_root/bin/gang"
installed_gang="$installed_root/bin/gang"
excludes "byte-identical Gangline copies compare as current" \
  "$("$installed_gang" status alpha)" "binary-skew"
printf '\n# fixture changes the executable bytes\n' >> "$installed_gang"
changed_stamp="$(cksum "$installed_gang" | awk '{ print "cksum:" $1 ":" $2 }')"
contains "an uncommitted executable change produces binary skew" \
  "$("$installed_gang" status alpha)" \
  "binary-skew ($binary_stamp != current $changed_stamp)"

dirty_root="$RUN_ROOT/dirty-checkout"
mkdir -p "$dirty_root/bin"
# gang_root resolves the executing checkout physically. Keep this fixture's
# expected path in the same identity domain when TMPDIR traverses a symlink
# (macOS exposes /var/folders through /private/var/folders).
dirty_root="$(cd -P "$dirty_root" && pwd)"
cp -R "$ROOT/collars" "$dirty_root/collars"
cp "$GANG" "$dirty_root/bin/gang"
git -C "$dirty_root" init -q
git -C "$dirty_root" add -- bin/gang collars
git -C "$dirty_root" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: clean executable witness'
dirty_head="$(git -C "$dirty_root" rev-parse HEAD)"
excludes "a clean checkout executable emits no dirty-execution warning" \
  "$("$dirty_root/bin/gang" collars 2>&1)" "executing dirty"
printf '\n# named dirty-execution mutant\n' >> "$dirty_root/bin/gang"
dirty_warning="$("$dirty_root/bin/gang" collars 2>&1)"
contains "a dirty live executable warns on ordinary command dispatch" \
  "$dirty_warning" "WARNING: executing dirty $dirty_root/bin/gang"
contains "the dirty warning names the HEAD its bytes diverged from" \
  "$dirty_warning" "$dirty_head"
git -C "$dirty_root" checkout -q -- bin/gang
excludes "restoring the executable to HEAD removes the warning" \
  "$("$dirty_root/bin/gang" collars 2>&1)" "executing dirty"

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
"$GANG" adopt adopted -c bash >/dev/null
equal "adopt stamps the current binary identity" "$binary_stamp" \
  "$(tmux show-options -wqv -t "$adopted_id" @gl_binary_id)"
"$GANG" drop adopted >/dev/null

# THE OPERATOR-FACING HALF of the clipped-composer reading. A box that outgrew
# its pane is drawn and is taking input; what it lacks is room to render. Naming
# that a harness which never drew a box sends the reader after the wrong thing,
# and it is the reading that let a stranded paste look like a dead harness.
clipped_agent_rule="$(printf '─%.0s' $(seq 40))"
{ printf 'transcript %s\n' 1 2 3 4 5 6
  printf '%s\n' "$clipped_agent_rule"
  printf '%s\n' '❯ a pasted body that outgrew'
  printf '%s\n' '  the rows this pane had for'
  printf '%s' '  it, so the box never closes'
} > "$RUN_ROOT/clipped-agent-frame"
tmux new-window -d -t "=$GANG_SESSION" -n clipped-agent \
  "cat '$RUN_ROOT/clipped-agent-frame'; cat" >/dev/null
"$GANG" adopt clipped-agent -c claude-code >/dev/null
clipped_agent_read="$("$GANG" composer clipped-agent 2>&1 || true)"
contains "gang composer names a box its pane was too short to show" \
  "$clipped_agent_read" "taller than its pane"
excludes "gang composer does not blame the harness for a box it did draw" \
  "$clipped_agent_read" "has not drawn its input box"
"$GANG" drop clipped-agent >/dev/null

adopt_alias_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n adopt-alias "PS1='❯ ' bash --norc")"
"$GANG" adopt adopt-alias -p bash \
  > "$RUN_ROOT/adopt-alias.out" 2> "$RUN_ROOT/adopt-alias.err"
contains "the adopt flag alias registers the requested collar" \
  "$(TMUX_PANE="$(tmux list-panes -t "$adopt_alias_id" -F '#{pane_id}')" \
    "$GANG" whoami)" "collar: bash"
contains "the adopt flag alias uses its own deprecation prefix" \
  "$(<"$RUN_ROOT/adopt-alias.err")" \
  "adopt: -p/--profile is now -c/--collar"
"$GANG" drop adopt-alias >/dev/null

adopt_conflict_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n adopt-conflict "PS1='❯ ' bash --norc")"
refuses "both adopt flag spellings refuse before registration" \
  "-c/--collar and -p/--profile" \
  "$GANG" adopt adopt-conflict -p bash -c bash
equal "the refused adopt leaves the window unregistered" "" \
  "$(tmux show-options -wqv -t "$adopt_conflict_id" @gl_agent)"
tmux kill-window -t "$adopt_conflict_id"

# composer reads the box through the collar's styled reading, not the raw
# pane; a freshly hitched agent's box is definitively empty
equal "composer prints nothing for an empty box" \
  "" "$("$GANG" composer alpha)"

mkdir -p "$RUN_ROOT/collars"
export GANG_COLLARS="$RUN_ROOT/collars"

# Native session identity is a window fact, and every renewal consumes that
# exact fact rather than directory recency. This fixture exposes the common
# hook payload shape without launching a real harness.
cat > "$RUN_ROOT/collars/identity.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fresh-identity"
GANG_RESUME_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' resume-{{session_id}}"
collar_session_id() {
  [ ! -e "$RUN_ROOT/identity-stderr-on" ] || printf 'COLLAR-IDENTITY-STDERR\n' >&2
  printf '%s' "\$2" | python3 -c 'import json,sys; print(json.load(sys.stdin)["session_id"])'
}
SH
"$HITCH" identity -c identity -d /tmp >/dev/null
identity_id="$(window_id identity)"
identity_pane="$(tmux list-panes -t "$identity_id" -F '#{pane_id}')"
: > "$RUN_ROOT/identity-stderr-on"
printf '%s' '{"hook_event_name":"Stop","session_id":"native-identity-123"}' \
  | TMUX_PANE="$identity_pane" "$GANG" hook \
    2> "$RUN_ROOT/identity-hook.err"
rm -f -- "$RUN_ROOT/identity-stderr-on"
equal "a hook suppresses collar identity diagnostics" "" \
  "$(<"$RUN_ROOT/identity-hook.err")"
equal "the first native hook stamps its exact harness session id" \
  "native-identity-123" \
  "$(tmux show-options -wqv -t "$identity_id" @gl_session_id)"
equal "hitch records the window's Gangline agent identity" "identity" \
  "$(tmux show-options -wqv -t "$identity_id" @gl_agent)"

# Codex consumes that same native payload directly. This registered window has
# the aborted-hitch/adopt shape: no startup nonce and no rollout binding. The
# first hook must stamp both facts without a sessions-tree search.
codex_payload_file="$RUN_ROOT/codex-payload-session.jsonl"
cat > "$codex_payload_file" <<'JSONL'
{"type":"session_meta","payload":{"id":"codex-payload-123"}}
{"payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":60000},"model_context_window":300000}}}
JSONL
codex_payload_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n codex-payload "PS1='❯ ' bash --norc")"
tmux set-option -w -t "$codex_payload_id" @gl_agent codex-payload
tmux set-option -w -t "$codex_payload_id" @gl_collar codex
codex_payload_pane="$(tmux list-panes -t "$codex_payload_id" -F '#{pane_id}')"
equal "the codex payload fixture begins with no native session binding" "|" \
  "$(tmux show-options -wqv -t "$codex_payload_id" @gl_session_id)|$(tmux show-options -wqv -t "$codex_payload_id" @gl_session)"
printf '%s' \
  "{\"hook_event_name\":\"Stop\",\"session_id\":\"codex-payload-123\",\"transcript_path\":\"$codex_payload_file\"}" \
  | TMUX_PANE="$codex_payload_pane" "$GANG" hook
equal "a first Codex hook stamps an agent that has no startup nonce" \
  "codex-payload-123" \
  "$(tmux show-options -wqv -t "$codex_payload_id" @gl_session_id)"
equal "the Codex hook binds its native transcript path for context" \
  "$codex_payload_file" \
  "$(tmux show-options -wqv -t "$codex_payload_id" @gl_session)"
equal "Codex context uses the hook-bound transcript without a nonce" \
  "60k/300k (20%)" "$($GANG context codex-payload)"
"$GANG" drop codex-payload >/dev/null

# Calibrate the parser's failure direction: a native-looking payload with the
# identity field absent must remain UNSTAMPED rather than borrowing any other
# hook or cwd fact.
codex_missing_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n codex-missing-id "PS1='❯ ' bash --norc")"
tmux set-option -w -t "$codex_missing_id" @gl_agent codex-missing-id
tmux set-option -w -t "$codex_missing_id" @gl_collar codex
codex_missing_pane="$(tmux list-panes -t "$codex_missing_id" -F '#{pane_id}')"
printf '%s' \
  "{\"hook_event_name\":\"Stop\",\"transcript_path\":\"$codex_payload_file\"}" \
  | TMUX_PANE="$codex_missing_pane" "$GANG" hook
equal "a Codex payload missing session_id calibrates to UNSTAMPED" "" \
  "$(tmux show-options -wqv -t "$codex_missing_id" @gl_session_id)"
equal "a rejected Codex payload does not half-bind its transcript" "" \
  "$(tmux show-options -wqv -t "$codex_missing_id" @gl_session)"
"$GANG" drop codex-missing-id >/dev/null

identity_report="$(TMUX_PANE="$identity_pane" "$GANG" whoami)"
contains "whoami names the agent" "$identity_report" "agent: identity"
contains "whoami names the pane" "$identity_report" "pane: $identity_pane"
contains "whoami names the collar" "$identity_report" "collar: identity"
contains "whoami names the stamped native session" "$identity_report" \
  "harness session id: native-identity-123"
contains "whoami names the latest live payload id" "$identity_report" \
  "live payload id: native-identity-123"
contains "whoami names the team" "$identity_report" "team session: $GANG_SESSION"

self_before="$(pane identity)"
refuses "a self-send is refused under the intent it violates" \
  "a message that returns to its author is never intended" \
  bash -c 'printf SELF_RETURN | TMUX_PANE="$1" "$2" send --to identity --stdin' \
  fixture "$identity_pane" "$GANG"
equal "a refused self-send types nothing" "$self_before" "$(pane identity)"

tmux set-option -w -t "$identity_id" @gl_session_id bogus-native-id
printf '%s' '{"hook_event_name":"PostToolUse","session_id":"native-identity-123"}' \
  | TMUX_PANE="$identity_pane" "$GANG" hook
contains "status exposes a native session identity mismatch" \
  "$("$GANG" status identity)" \
  "stamped 'bogus-native-id' != live 'native-identity-123'"
contains "roster exposes a native session identity mismatch" \
  "$("$GANG" roster)" "identity-mismatch="
mismatch_before="$(pane alpha)"
refuses "a mismatched pane cannot send under the window's authority" \
  "identity mismatch" bash -c \
  'printf MISMATCH_SPOKE | TMUX_PANE="$1" "$2" send --to alpha --stdin' \
  fixture "$identity_pane" "$GANG"
equal "the mismatch refusal types nothing into its target" \
  "$mismatch_before" "$(pane alpha)"
tmux set-option -w -t "$identity_id" @gl_session_id native-identity-123
printf '%s' '{"hook_event_name":"Stop","session_id":"native-identity-123"}' \
  | TMUX_PANE="$identity_pane" "$GANG" hook
excludes "a matching next hook repairs the visible mismatch" \
  "$("$GANG" roster)" "identity-mismatch="

drop_identity_out="$("$GANG" drop identity)"
contains "drop prints the parting native session id before destroying its window" \
  "$drop_identity_out" "session id: native-identity-123"
contains "drop prints the exact explicit-id relaunch line" "$drop_identity_out" \
  "gang hitch identity --resume native-identity-123"
"$GANG" hitch identity -c identity -d "$RUN_ROOT" \
  --resume native-identity-123 >/dev/null
contains "explicit resume substitutes the quoted identity from any cwd" \
  "$(tmux display-message -p -t "$(window_id identity)" '#{pane_start_command}')" \
  "resume-native-identity-123"
"$GANG" drop identity >/dev/null
refuses "bare resume without a surviving stamped window refuses loudly" \
  "gang hitch identity --resume <session-id>" \
  "$GANG" hitch identity -c identity -d /tmp --resume
equal "a refused stamp-less resume launches nothing" "" "$(window_id identity)"

"$HITCH" survivor -c identity -d /tmp >/dev/null
survivor_id="$(window_id survivor)"
survivor_pane="$(tmux list-panes -t "$survivor_id" -F '#{pane_id}')"
printf '%s' '{"hook_event_name":"Stop","session_id":"surviving-native-id"}' \
  | TMUX_PANE="$survivor_pane" "$GANG" hook
refuses "explicit resume refuses a different session over a stamped window" \
  "stamped for harness session 'surviving-native-id', not requested session 'other-native-id'" \
  "$GANG" hitch survivor -c identity -d /tmp --resume other-native-id
tmux set-option -w -t "$survivor_id" remain-on-exit on
survivor_dead="test-survivor-dead-$$"
printf -v survivor_exit 'trap %q EXIT; exit' "tmux wait-for -S $survivor_dead"
tmux send-keys -l -t "$survivor_id" "$survivor_exit"
tmux send-keys -t "$survivor_id" Enter
tmux wait-for "$survivor_dead"
"$HITCH" survivor -c identity -d /tmp --resume >/dev/null
contains "bare resume reads the stamp from a surviving dead window" \
  "$(tmux display-message -p -t "$survivor_id" '#{pane_start_command}')" \
  "resume-surviving-native-id"
"$GANG" drop survivor >/dev/null

unadopted_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n unadopted "PS1='❯ ' bash --norc")"
unadopted_pane="$(tmux list-panes -t "$unadopted_id" -F '#{pane_id}')"
unadopted_target_before="$(pane alpha)"
refuses "an unadopted pane cannot send under its bare window name" \
  "no registered Gangline agent/collar identity" bash -c \
  'printf IDENTITY_ABSENT | TMUX_PANE="$1" "$2" send --to alpha --stdin' \
  fixture "$unadopted_pane" "$GANG"
equal "identity absence types nothing into the claimed target" \
  "$unadopted_target_before" "$(pane alpha)"
tmux kill-window -t "$unadopted_id"

residue_recovery_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n residue-recovery "PS1='❯ ' bash --norc")"
residue_recovery_pane="$(tmux list-panes -t "$residue_recovery_id" -F '#{pane_id}')"
tmux set-option -w -t "$residue_recovery_id" @gl_collar identity
residue_target_before="$(pane residue-recovery)"
if residue_target_out="$(printf RESIDUE_TARGET_BODY | "$GANG" send \
    --to residue-recovery --from tester --live-only --stdin 2>&1)"; then
  fail "collar residue without registration is not a send target" \
    "send unexpectedly succeeded"
else
  contains "the unregistered target refusal names adoption as the repair" \
    "$residue_target_out" "not a registered Gangline agent"
fi
# source-guard: whole-surface@f8a9746a76b8: a refused target must leave every visible pane byte unchanged regardless of producer
equal "the unregistered target receives no message bytes" \
  "$residue_target_before" "$(pane residue-recovery)"
refuses "collar residue without @gl_agent cannot send under a bare window name" \
  "no registered Gangline agent/collar identity" bash -c \
  'printf RESIDUE_BEFORE_ADOPT | TMUX_PANE="$1" "$2" send --to alpha --stdin' \
  fixture "$residue_recovery_pane" "$GANG"
if residue_adopt_out="$("$GANG" adopt residue-recovery -c identity 2>&1)"; then
  pass "the deliberate adopt remedy claims a collar-residue window"
else
  fail "the deliberate adopt remedy claims a collar-residue window" \
    "$residue_adopt_out"
fi
equal "adopt stamps the claimed residue window with its named identity" \
  "residue-recovery" \
  "$(tmux show-options -wqv -t "$residue_recovery_id" @gl_agent)"
if printf 'RESIDUE_AFTER_ADOPT' | TMUX_PANE="$residue_recovery_pane" \
    "$GANG" send --to alpha --stdin >/dev/null 2>&1; then
  pass "the named adopt remedy resolves the send refusal"
else
  fail "the named adopt remedy resolves the send refusal" \
    "send still refused after deliberate adoption"
fi
contains "the recovered pane's attributed message reaches its peer" \
  "$(pane alpha)" "RESIDUE_AFTER_ADOPT"
excludes "the pre-adoption refused body never reached the peer" \
  "$(pane alpha)" "RESIDUE_BEFORE_ADOPT"
"$GANG" drop residue-recovery >/dev/null

residue_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n displaced "PS1='❯ ' bash --norc")"
tmux set-option -w -t "$residue_id" @gl_agent lead
refuses "hitch refuses a target window registered to another identity" \
  "registered to Gangline agent 'lead'" \
  "$GANG" hitch displaced -c identity -d /tmp
refuses "adopt refuses a window registered to another Gangline identity" \
  "registered to Gangline agent 'lead'" "$GANG" adopt displaced -c bash
contains "the mismatch refusal names the requested window identity too" \
  "$("$GANG" adopt displaced -c bash 2>&1 || true)" "displaced"
tmux kill-window -t "$residue_id"

# IDENTITY IS THE REGISTRATION, NOT THE TITLE. tmux lets anyone rename a window,
# and gang paints its own state glyph into that title out of @gl_agent, so a
# resolver that matches on #W can be aimed at another lane's harness by a rename
# alone — and would paste, submit, VERIFY and report success against it. The
# registration is what hitch and adopt wrote; the title is decoration.
"$HITCH" registered-name -c bash -d /tmp >/dev/null
registered_name_id="$(window_id registered-name)"
tmux rename-window -t "$registered_name_id" borrowed-title
refuses "a window title cannot address the agent registered under another name" \
  "registered to Gangline agent 'registered-name'" \
  bash -c 'printf TITLE_SPOOF | "$1" send --to borrowed-title --from tester --stdin' \
  fixture "$GANG"
excludes "the title-addressed message never reached the registered harness" \
  "$(tmux capture-pane -pJ -t "$registered_name_id")" "TITLE_SPOOF"
if printf 'REGISTERED_REACHES' |
    "$GANG" send --to registered-name --from tester --stdin >/dev/null 2>&1; then
  pass "the registered identity still resolves through a changed window title"
else
  fail "the registered identity still resolves through a changed window title" \
    "send refused the agent under its own registered name"
fi
contains "and that message reaches the registered harness" \
  "$(tmux capture-pane -pJ -t "$registered_name_id")" "REGISTERED_REACHES"
equal "the agent's own name is its registration, whatever the title says" \
  "registered-name" \
  "$(TMUX_PANE="$(tmux list-panes -t "$registered_name_id" -F '#{pane_id}')" \
    "$GANG" whoami | sed -n 's/^agent: //p')"
# Teardown, not an assertion: a resolver that cannot find this window by its
# registration is exactly the defect above, and the fixture still has to go.
"$GANG" drop registered-name >/dev/null 2>&1 \
  || tmux kill-window -t "$registered_name_id"

# A REGISTRATION IS RAW BYTES, AND IT IS NOW THE TRUSTED IDENTITY. tmux escapes
# control characters out of a window TITLE — #W reads \033 back as six literal
# characters — so the title was never a way to paint somebody's terminal. An
# option value is returned exactly as written, so reading identity from
# @gl_agent opened that door. Names go out sanitized; matching keeps the bytes.
ctl_name="$(printf 'ctl\033[31mrogue')"
ctl_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n ctl-rogue "PS1='❯ ' bash --norc")"
ctl_pane="$(tmux list-panes -t "$ctl_id" -F '#{pane_id}')"
tmux set-option -w -t "$ctl_id" @gl_agent "$ctl_name"
tmux set-option -w -t "$ctl_id" @gl_collar bash
ctl_escapes() { case "$1" in *$'\033'*) printf raw ;; *) printf clean ;; esac; }
ctl_whoami="$(TMUX_PANE="$ctl_pane" "$GANG" whoami 2>&1)" || true
ctl_mail="$(TMUX_PANE="$ctl_pane" "$GANG" mail 2>&1)" || true
ctl_drop="$("$GANG" drop "$ctl_name" 2>&1)" || true
equal "a control-bearing registration never reaches the terminal raw" \
  "clean clean clean" \
  "$(ctl_escapes "$ctl_whoami") $(ctl_escapes "$ctl_mail") $(ctl_escapes "$ctl_drop")"
tmux kill-window -t "$ctl_id" 2>/dev/null || true

# A synchronous tty fixture paints the captured Codex menu and records every
# key Gangline sends. Each mutant changes one load-bearing observation.
cat > "$RUN_ROOT/dialog-fixture.py" <<'PY'
#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
import os
import subprocess
import sys
import tty

variant = os.environ.get("DIALOG_VARIANT", "known")
log_path = os.environ["DIALOG_KEY_LOG"]
ready = os.environ["DIALOG_READY"]
selected = 0
composer = False
draft = bytearray()
answered = 0
safe_index = 1
selected_glyph = "›"
external_frame = None
body = [
    "Our systems are thinking a bit more about this request before responding.",
    "Hang tight or retry with a faster model for a quicker response, though it may be less capable of handling complex requests.",
]
labels = ["Retry with a faster model", "Dismiss and keep waiting", "Learn more"]
footer = "No action is required. Codex will keep waiting, and this menu will close when the response is ready."
if variant == "trust":
    body = [
        "> You are in /tmp/fixture-cwd",
        "Do you trust the contents of this directory? Working with untrusted contents comes with higher risk of prompt injection. Trusting the directory allows project-local config, hooks, and exec policies to load.",
    ]
    labels = ["Yes, continue", "No, quit"]
    footer = "Press enter to continue"
    safe_index = 0
if variant == "external-import":
    with open(os.environ["DIALOG_CAPTURE"], encoding="utf-8") as stream:
        external_frame = stream.read().splitlines()
    labels = ["Yes, allow external imports", "No, disable external imports"]
    safe_index = 0
    selected_glyph = "❯"
if variant == "changed-byte":
    body[0] = body[0].replace("systems", "system")
if variant == "reordered":
    body[0], body[1] = body[1], body[0]
if variant == "authority":
    body = ["Permission required before this command can run.", "Do you want to allow full access?"]
    labels = ["Allow", "Deny", "Learn more"]
    footer = "Choose whether to approve this permission."
if variant == "extra-option":
    labels.insert(2, "Open diagnostics")

def paint():
    sys.stdout.write("\x1b[2J\x1b[H")
    if external_frame is not None:
        for line in external_frame:
            print(line)
        sys.stdout.flush()
        return
    for line in body:
        print("  " + line)
    print()
    for index, label in enumerate(labels):
        glyph = selected_glyph if index == selected or (variant == "two-glyph" and index == 1) else " "
        print(f"{glyph} {index + 1}. {label}")
    print()
    print("  " + footer)
    sys.stdout.flush()

def record(key):
    with open(log_path, "a", encoding="utf-8") as stream:
        stream.write(key + "\n")

tty.setraw(sys.stdin.fileno())
paint()
subprocess.run(["tmux", "wait-for", "-S", ready], check=True)
while True:
    char = os.read(sys.stdin.fileno(), 1)
    if composer:
        if char in (b"\r", b"\n"):
            sys.stdout.write("\r\n" + draft.decode("utf-8") + "\r\n❯ ")
            sys.stdout.flush()
            draft.clear()
        else:
            draft.extend(char)
            os.write(sys.stdout.fileno(), char)
        continue
    if char == b"\x1b":
        tail = os.read(sys.stdin.fileno(), 2)
        if tail == b"[B":
            record("Down")
            if variant != "wrong-move":
                selected = min(selected + 1, len(labels) - 1)
            paint()
        elif tail == b"[A":
            record("Up")
            if variant != "wrong-move":
                selected = max(selected - 1, 0)
            paint()
    elif char in (b"\r", b"\n"):
        record("Enter")
        if selected == safe_index and variant != "confirm-stuck":
            answered += 1
            if variant == "recurring" and answered < 5:
                sys.stdout.write("\x1b[2J\x1b[H❯ ")
                sys.stdout.flush()
                subprocess.run(
                    ["tmux", "wait-for", os.environ["DIALOG_RECUR_SIGNAL"]],
                    check=True,
                )
                selected = 0
                paint()
            else:
                composer = True
                sys.stdout.write("\x1b[2J\x1b[H❯ ")
                sys.stdout.flush()
        else:
            paint()
PY
chmod +x "$RUN_ROOT/dialog-fixture.py"
dialog_recurring_signal="dialog-recurring-next-$$"

cat > "$RUN_ROOT/collars/dialog.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' dialog"
GANG_OCCUPIED_REGEX='^› [0-9]+\. '
GANG_DIALOGS='safety-buffering-prompt|^› [0-9]+\. |Dismiss and keep waiting|Down|Enter'
GANG_DIALOG_LINES_safety_buffering_prompt='Our systems are thinking a bit more about this request before responding.
Hang tight or retry with a faster model for a quicker response, though it may be less capable of handling complex requests.
Retry with a faster model
Dismiss and keep waiting
Learn more
No action is required. Codex will keep waiting, and this menu will close when the response is ready.'
SH
cat > "$RUN_ROOT/collars/dialog-wrong.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_DIALOGS='safety-buffering-prompt|^› [0-9]+\. |Dismiss and keep waiting|Up|Enter'
SH
cat > "$RUN_ROOT/collars/dialog-recurring.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_LAUNCH="env DIALOG_VARIANT=recurring DIALOG_KEY_LOG='$RUN_ROOT/dialog-recurring.keys' DIALOG_READY='dialog-recurring-ready-$$' DIALOG_RECUR_SIGNAL='$dialog_recurring_signal' '$RUN_ROOT/dialog-fixture.py'"
SH
cat > "$RUN_ROOT/collars/dialog-observe.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_DIALOGS='operator-choice|^› [0-9]+\. |||'
GANG_DIALOG_LINES_operator_choice="\$GANG_DIALOG_LINES_safety_buffering_prompt"
SH
observe_boot_seen="dialog-observe-boot-seen-$$"
observe_boot_release="dialog-observe-boot-release-$$"
cat > "$RUN_ROOT/collars/dialog-observe-boot.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-observe.sh"
unset -f collar_input
GANG_LAUNCH="env DIALOG_VARIANT=known DIALOG_KEY_LOG='$RUN_ROOT/dialog-observe-boot.keys' DIALOG_READY='dialog-observe-boot-ready-$$' '$RUN_ROOT/dialog-fixture.py'"
SH
cat > "$RUN_ROOT/collars/dialog-leading-space.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-observe.sh"
GANG_DIALOGS='dead-record|^ +› [0-9]+\. |||'
GANG_DIALOG_LINES_dead_record="\$GANG_DIALOG_LINES_operator_choice"
SH
cat > "$RUN_ROOT/collars/dialog-single-space.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-observe.sh"
GANG_DIALOGS='dead-record|^ › [0-9]+\. |||'
GANG_DIALOG_LINES_dead_record="\$GANG_DIALOG_LINES_operator_choice"
SH
cat > "$RUN_ROOT/collars/dialog-class-space.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-observe.sh"
GANG_DIALOGS='dead-record|^[[:space:]]› [0-9]+\. |||'
GANG_DIALOG_LINES_dead_record="\$GANG_DIALOG_LINES_operator_choice"
SH
cat > "$RUN_ROOT/collars/dialog-optional-space.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-observe.sh"
GANG_DIALOGS='optional-space|^ *› [0-9]+\. |||'
GANG_DIALOG_LINES_optional_space="\$GANG_DIALOG_LINES_operator_choice"
SH
printf -v claude_external_record_q '%q' "$claude_external_record"
printf -v claude_external_lines_q '%q' "$claude_external_lines"
cat > "$RUN_ROOT/collars/dialog-claude-external.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_DIALOGS=$claude_external_record_q
GANG_DIALOG_LINES_external_import_trust=$claude_external_lines_q
SH
mkdir -p "$RUN_ROOT/dialog-observe-bin"
cat > "$RUN_ROOT/dialog-observe-bin/sleep" <<SH
#!/bin/sh
if [ ! -e "$RUN_ROOT/dialog-observe-sleep-seen" ]; then
  : > "$RUN_ROOT/dialog-observe-sleep-seen"
  printf '%s' "\$1" > "$RUN_ROOT/dialog-observe-sleep-argument"
  tmux wait-for -S "$observe_boot_seen"
  tmux wait-for "$observe_boot_release"
fi
exit 0
SH
chmod +x "$RUN_ROOT/dialog-observe-bin/sleep"

for dead_marker_case in leading-space single-space class-space; do
  if dead_marker_out="$("$GANG" hitch "dialog-$dead_marker_case" \
      -c "dialog-$dead_marker_case" -d /tmp 2>&1)"; then
    fail "$dead_marker_case dialog marker dead after normalization is refused" \
      "hitch unexpectedly succeeded"
  else
    contains "$dead_marker_case refusal names normalization" \
      "$dead_marker_out" "requires leading whitespace that dialog normalization removes"
  fi
  equal "$dead_marker_case dead marker opens no window" "" \
    "$(window_id "dialog-$dead_marker_case")"
done
if "$HITCH" dialog-optional-space -c dialog-optional-space -d /tmp \
    >/dev/null; then
  pass "a marker allowing zero leading spaces remains loadable"
else
  fail "a marker allowing zero leading spaces remains loadable" \
    "hitch refused a marker that matches normalized input"
fi
"$GANG" drop dialog-optional-space >/dev/null

dialog_start() { # $1 agent, $2 variant, $3 collar
  local name="$1" variant="$2" collar="$3" id command
  "$HITCH" "$name" -c "$collar" -d /tmp >/dev/null
  id="$(window_id "$name")"
  case "$name" in dialog-known) tmux resize-window -t "$id" -x 48 -y 24 ;; esac
  : > "$RUN_ROOT/$name.keys"
  printf -v command 'DIALOG_VARIANT=%q DIALOG_KEY_LOG=%q DIALOG_READY=%q DIALOG_CAPTURE=%q %q' \
    "$variant" "$RUN_ROOT/$name.keys" "dialog-ready-$name-$$" \
    "$ROOT/test/fixtures/claude-external-import.txt" "$RUN_ROOT/dialog-fixture.py"
  tmux send-keys -l -t "$id" "$command"
  tmux send-keys -t "$id" Enter
  tmux wait-for "dialog-ready-$name-$$"
}

dialog_start dialog-known known dialog
dialog_known_err="$RUN_ROOT/dialog-known.err"
if printf 'DIALOG_BODY_REACHED' | "$GANG" send --to dialog-known --from tester \
    --live-only --stdin >/dev/null 2>"$dialog_known_err"; then
  pass "a fully fingerprinted known dialog is auto-answered"
else
  fail "a fully fingerprinted known dialog is auto-answered" \
    "$(<"$dialog_known_err")"
fi
contains "the full-block match survives a narrow soft-wrapped pane and sends" \
  "$(pane dialog-known)" "DIALOG_BODY_REACHED"
contains "known-dialog triage names the exact registry id" \
  "$(<"$dialog_known_err")" "safety-buffering-prompt"
equal "known-dialog triage moves once and confirms only after verification" \
  $'Down\nEnter' "$(<"$RUN_ROOT/dialog-known.keys")"
"$GANG" drop dialog-known >/dev/null

for mutant in extra-option reordered changed-byte two-glyph authority; do
  dialog_start "dialog-$mutant" "$mutant" dialog
  mutant_before="$(pane "dialog-$mutant")"
  mutant_keys="$(<"$RUN_ROOT/dialog-$mutant.keys")"
  if printf "MUTANT_$mutant" | "$GANG" send --to "dialog-$mutant" \
      --from tester --live-only --stdin >/dev/null 2>&1; then
    fail "$mutant known-dialog mutant is refused" "send unexpectedly succeeded"
  else
    pass "$mutant known-dialog mutant is refused"
  fi
  equal "$mutant mutant leaves the dialog byte-for-byte on screen" \
    "$mutant_before" "$(pane "dialog-$mutant")"
  equal "$mutant mutant receives no auto-answer key" "$mutant_keys" \
    "$(<"$RUN_ROOT/dialog-$mutant.keys")"
  "$GANG" drop "dialog-$mutant" >/dev/null
done

dialog_start dialog-wrong wrong-move dialog-wrong
if wrong_out="$(printf WRONG_SELECTION | "$GANG" send --to dialog-wrong \
    --from tester --live-only --stdin 2>&1)"; then
  fail "confirm never fires while the glyph is on a non-safe row" \
    "send unexpectedly succeeded"
else
  contains "the wrong-selection refusal names the dialog" \
    "$wrong_out" "safety-buffering-prompt"
  contains "the wrong-selection refusal points at direct inspection" \
    "$wrong_out" "gang attach"
fi
equal "the wrong-selection mutant records its move but no confirm" \
  "Up" "$(<"$RUN_ROOT/dialog-wrong.keys")"
"$GANG" drop dialog-wrong >/dev/null

dialog_start dialog-confirm-stuck confirm-stuck dialog
if stuck_out="$(printf STUCK_CONFIRM | "$GANG" send --to dialog-confirm-stuck \
    --from tester --live-only --stdin 2>&1)"; then
  fail "a confirm key that does not clear the dialog fails loud" \
    "send unexpectedly succeeded"
else
  contains "the uncleared-dialog refusal names the registry id" \
    "$stuck_out" "safety-buffering-prompt"
  contains "the uncleared-dialog refusal points at gang attach" \
    "$stuck_out" "gang attach"
fi
equal "the stuck-confirm fixture received only the verified move and confirm" \
  $'Down\nEnter' "$(<"$RUN_ROOT/dialog-confirm-stuck.keys")"
"$GANG" drop dialog-confirm-stuck >/dev/null

dialog_start dialog-status known dialog
status_dialog_before="$(pane dialog-status)"
contains "status names a known transient without answering it" \
  "$("$GANG" status dialog-status)" \
  "!occupied! (known transient: safety-buffering-prompt)"
equal "status is read-only even for a recognized dialog" \
  "$status_dialog_before" "$(pane dialog-status)"
equal "status presses no dialog key" "" "$(<"$RUN_ROOT/dialog-status.keys")"
"$GANG" drop dialog-status >/dev/null

dialog_start dialog-observe known dialog-observe
observe_before="$(pane dialog-observe)"
contains "status names a recognized operator dialog without calling it transient" \
  "$("$GANG" status dialog-observe)" \
  "!occupied! (known operator dialog: operator-choice)"
if observe_send="$(printf OBSERVE_ONLY_BODY | "$GANG" send \
    --to dialog-observe --from tester --live-only --stdin 2>&1)"; then
  fail "an observe-only dialog refuses delivery" "send unexpectedly succeeded"
else
  contains "the refusal names the operator decision Gangline will not make" \
    "$observe_send" "known operator dialog 'operator-choice'"
fi
equal "observe-only recognition sends no key" "" \
  "$(<"$RUN_ROOT/dialog-observe.keys")"
# source-guard: whole-surface@f2d2c71674d2: no visible byte may change because observe-only recognition is forbidden to send any key
equal "observe-only recognition leaves the operator dialog unchanged" \
  "$observe_before" "$(pane dialog-observe)"
"$GANG" drop dialog-observe >/dev/null

# Bind the shipped Claude declaration to the matcher that consumes it. The
# fixture renders the stable suffix captured from Claude; the collar under
# test contributes its own marker and block, rather than restating either.
dialog_start dialog-claude-external external-import dialog-claude-external
claude_external_before="$(pane dialog-claude-external)"
contains "the shipped Claude record recognizes its rendered dialog" \
  "$("$GANG" status dialog-claude-external)" \
  "known operator dialog: external-import-trust"
if claude_external_send="$(printf SHIPPED_EXTERNAL_BODY | "$GANG" send \
    --to dialog-claude-external --from tester --live-only --stdin 2>&1)"; then
  fail "the shipped Claude operator dialog refuses delivery" \
    "send unexpectedly succeeded"
else
  contains "the shipped refusal names its exact dialog" \
    "$claude_external_send" "known operator dialog 'external-import-trust'"
fi
equal "the shipped Claude record sends no key" "" \
  "$(<"$RUN_ROOT/dialog-claude-external.keys")"
# source-guard: whole-surface@c171fc82a7df: an observe-only shipped record is forbidden to change any visible pane byte, whatever produced it
equal "the shipped Claude operator dialog remains byte-exact" \
  "$claude_external_before" "$(pane dialog-claude-external)"
"$GANG" drop dialog-claude-external >/dev/null

# Hitch names an observe-only prompt while preserving the original boot budget.
# The sleep shim is an event barrier: the notice is already written when it
# signals, then the fixture receives only the two manual operator keys below.
: > "$RUN_ROOT/dialog-observe-boot.keys"
PATH="$RUN_ROOT/dialog-observe-bin:$PATH" GANG_BOOT_TIMEOUT=3 \
  "$GANG" hitch dialog-observe-boot -c dialog-observe-boot -d /tmp \
  >"$RUN_ROOT/dialog-observe-boot.out" 2>&1 &
observe_boot_pid=$!
tmux wait-for "$observe_boot_seen"
equal "an observe-only branch waits before testing generic readiness" \
  "1" "$(<"$RUN_ROOT/dialog-observe-sleep-argument")"
contains "hitch names the operator dialog it is waiting on" \
  "$(<"$RUN_ROOT/dialog-observe-boot.out")" \
  "known operator dialog 'operator-choice'"
equal "hitch sends no key to the operator dialog" "" \
  "$(<"$RUN_ROOT/dialog-observe-boot.keys")"
observe_boot_id="$(window_id dialog-observe-boot)"
tmux send-keys -t "$observe_boot_id" Down Enter
tmux wait-for -S "$observe_boot_release"
if wait "$observe_boot_pid"; then
  pass "hitch continues after the operator answers the recognized dialog"
else
  fail "hitch continues after the operator answers the recognized dialog" \
    "$(<"$RUN_ROOT/dialog-observe-boot.out")"
fi
equal "only the operator's manual answer reaches the recognized dialog" \
  $'Down\nEnter' "$(<"$RUN_ROOT/dialog-observe-boot.keys")"
# source-guard: producer@b9a4f7b286cf: the successful hitch above is the sole producer of this nonce-addressed startup body after the fixture restores its composer
contains "the post-dialog startup contract is delivered" \
  "$(pane dialog-observe-boot)" "You are dialog-observe-boot in Gangline"
"$GANG" drop dialog-observe-boot >/dev/null

cat > "$RUN_ROOT/collars/dialog-ambiguous.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog.sh"
GANG_DIALOGS='safety-buffering-prompt|^› [0-9]+\. |Dismiss and keep waiting|Down|Enter
same-bytes|^› [0-9]+\. |Dismiss and keep waiting|Down|Enter'
GANG_DIALOG_LINES_same_bytes="\$GANG_DIALOG_LINES_safety_buffering_prompt"
SH
dialog_start dialog-ambiguous known dialog-ambiguous
if ambiguous_out="$("$GANG" status dialog-ambiguous 2>&1)"; then
  fail "two matching dialog records are refused as ambiguity" \
    "status unexpectedly succeeded"
else
  contains "ambiguity names the first matching id" \
    "$ambiguous_out" "safety-buffering-prompt"
  contains "ambiguity names the second matching id" "$ambiguous_out" "same-bytes"
fi
equal "an ambiguous registry presses no key" "" \
  "$(<"$RUN_ROOT/dialog-ambiguous.keys")"
"$GANG" drop dialog-ambiguous >/dev/null

cat > "$RUN_ROOT/collars/dialog-danger.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='permission-surface|^› [0-9]+\. |Allow||Enter'
GANG_DIALOG_LINES_permission_surface='Do you want to allow this tool to run?
Allow
Deny'
SH
refuses "load_collar mechanically rejects an authority-shaped dialog registry" \
  "forbidden authority word" "$GANG" hitch dialog-danger -c dialog-danger -d /tmp
equal "a refused authority registry opens no window" "" "$(window_id dialog-danger)"

cat > "$RUN_ROOT/collars/dialog-trust-unscoped.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='directory-trust-prompt|^› [0-9]+\. |Yes, continue||Enter'
GANG_DIALOG_LINES_directory_trust_prompt='Do you trust this directory?
Yes, continue
No, quit'
SH
refuses "directory trust remains forbidden without the hitch-directory declaration" \
  "forbidden authority word" \
  "$GANG" hitch dialog-trust-unscoped -c dialog-trust-unscoped -d /tmp
equal "an unscoped trust registry opens no window" "" \
  "$(window_id dialog-trust-unscoped)"

authority_probe_cases='approval|Approval required to continue
access|Grant access to your files
authorize|Authorize this device
admin|Grant admin rights to the agent
denial|Denial of safeguards is disabled
ask-again|Yes, and do not ask again
bypass|Bypass all safety checks
write-access|Enable write access to disk'
while IFS='|' read -r authority_slug authority_line; do
  [ -n "$authority_slug" ] || continue
  cat > "$RUN_ROOT/collars/dialog-authority-$authority_slug.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='authority-$authority_slug|^› [0-9]+\. |No||Enter'
GANG_DIALOG_LINES_authority_${authority_slug//-/_}='$authority_line
Yes
No'
SH
  refuses "authority registry rejects $authority_slug language" \
    "forbidden authority word" \
    "$GANG" hitch "authority-$authority_slug" \
      -c "dialog-authority-$authority_slug" -d /tmp
  equal "the $authority_slug authority refusal opens no window" "" \
    "$(window_id "authority-$authority_slug")"
done <<<"$authority_probe_cases"

cat > "$RUN_ROOT/collars/dialog-four-fields.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='broken|^› [0-9]+\. |Safe|Enter'
SH
cat > "$RUN_ROOT/collars/dialog-bad-id.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='Bad_ID|^› [0-9]+\. |Safe||Enter'
GANG_DIALOG_LINES_Bad_ID='Safe'
SH
cat > "$RUN_ROOT/collars/dialog-safe-absent.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='safe-absent|^› [0-9]+\. |Missing||Enter'
GANG_DIALOG_LINES_safe_absent='Present'
SH
cat > "$RUN_ROOT/collars/dialog-block-missing.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_DIALOGS='block-missing|^› [0-9]+\. |Safe||Enter'
SH
refuses "a four-field dialog record is refused at collar load" \
  "without exactly five fields" \
  "$GANG" hitch dialog-four-fields -c dialog-four-fields -d /tmp
refuses "a dialog id outside the slug alphabet is refused at collar load" \
  "invalid dialog id" "$GANG" hitch dialog-bad-id -c dialog-bad-id -d /tmp
refuses "a safe label absent from its declared block is refused at collar load" \
  "is not one of" \
  "$GANG" hitch dialog-safe-absent -c dialog-safe-absent -d /tmp
refuses "an unset dialog block is refused at collar load" \
  "has no non-empty GANG_DIALOG_LINES_block_missing" \
  "$GANG" hitch dialog-block-missing -c dialog-block-missing -d /tmp

: > "$RUN_ROOT/dialog-recurring.keys"
mkdir -p "$RUN_ROOT/recurring-bin"
cat > "$RUN_ROOT/recurring-bin/sleep" <<'SH'
#!/bin/sh
[ "${1:-}" != 1 ] || tmux wait-for -S "$DIALOG_RECUR_SIGNAL"
exit 0
SH
chmod +x "$RUN_ROOT/recurring-bin/sleep"
if recurring_out="$(DIALOG_RECUR_SIGNAL="$dialog_recurring_signal" \
    PATH="$RUN_ROOT/recurring-bin:$PATH" GANG_BOOT_TIMEOUT=2 \
    "$GANG" hitch dialog-recurring \
    -c dialog-recurring -d /tmp 2>&1)"; then
  fail "a recurring known transient consumes the hitch boot budget" \
    "hitch unexpectedly answered every recurrence and succeeded"
else
  contains "recurring known-transient exhaustion fails loud" \
    "$recurring_out" "startup message was not delivered"
fi
case "$(<"$RUN_ROOT/dialog-recurring.keys")" in
  $'Down\nEnter'|$'Down\nEnter\nDown\nEnter')
    pass "recurring dialogs cannot answer beyond the hitch boot budget" ;;
  *) fail "recurring dialogs cannot answer beyond the hitch boot budget" \
       "unexpected key sequence [$(<"$RUN_ROOT/dialog-recurring.keys")]" ;;
esac
"$GANG" drop dialog-recurring >/dev/null

# Calibrate the boot-dialog instrument first: one fingerprint byte is wrong,
# so the same trust-shaped screen must receive no key and reproduce the settled
# non-composer hitch failure. The successful twin then exercises the shipped
# empty-move shape and startup delivery, not merely dialog recognition.
cat > "$RUN_ROOT/collars/dialog-trust-boot.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="env DIALOG_VARIANT=trust DIALOG_KEY_LOG='$RUN_ROOT/dialog-trust-boot.keys' DIALOG_READY='dialog-trust-boot-ready-$$' '$RUN_ROOT/dialog-fixture.py'"
GANG_OCCUPIED_REGEX='^› [0-9]+\. '
GANG_DIALOGS='directory-trust-prompt|^› [0-9]+\. |Yes, continue||Enter'
GANG_DIALOG_HITCH_DIR_TRUST=directory-trust-prompt
GANG_DIALOG_LINES_directory_trust_prompt='Do you trust the contents of this directory? Working with untrusted contents comes with higher risk of prompt injection. Trusting the directory allows project-local config, hooks, and exec policies to load.
Yes, continue
No, quit
Press enter to continue'
SH
cat > "$RUN_ROOT/collars/dialog-trust-boot-miss.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/dialog-trust-boot.sh"
GANG_LAUNCH="env DIALOG_VARIANT=trust DIALOG_KEY_LOG='$RUN_ROOT/dialog-trust-boot-miss.keys' DIALOG_READY='dialog-trust-boot-miss-ready-$$' '$RUN_ROOT/dialog-fixture.py'"
GANG_DIALOG_LINES_directory_trust_prompt='Do you trust the content of this directory? Working with untrusted contents comes with higher risk of prompt injection. Trusting the directory allows project-local config, hooks, and exec policies to load.
Yes, continue
No, quit
Press enter to continue'
SH
: > "$RUN_ROOT/dialog-trust-boot-miss.keys"
if trust_miss_out="$(GANG_BOOT_TIMEOUT=1 "$GANG" hitch trust-boot-miss \
    -c dialog-trust-boot-miss -d /tmp 2>&1)"; then
  fail "a mutated trust fingerprint calibrates the boot-dialog guard red" \
    "hitch unexpectedly succeeded"
else
  pass "a mutated trust fingerprint calibrates the boot-dialog guard red"
fi
equal "the missed boot dialog receives no auto-answer key" "" \
  "$(<"$RUN_ROOT/dialog-trust-boot-miss.keys")"
contains "unknown-dialog recovery delivers the contract after manual clearance" \
  "$trust_miss_out" "gang send --to trust-boot-miss"
excludes "unknown-dialog recovery does not prescribe an unconditional drop" \
  "$trust_miss_out" "then 'gang drop"
"$GANG" drop trust-boot-miss >/dev/null

: > "$RUN_ROOT/dialog-trust-boot.keys"
if GANG_BOOT_TIMEOUT=3 "$GANG" hitch trust-boot -c dialog-trust-boot -d /tmp \
    >"$RUN_ROOT/dialog-trust-boot.out" 2>&1; then
  pass "hitch answers the known directory-trust dialog and continues"
else
  fail "hitch answers the known directory-trust dialog and continues" \
    "$(<"$RUN_ROOT/dialog-trust-boot.out")"
fi
equal "the preselected trust row needs only its empty-move confirmation" \
  "Enter" "$(<"$RUN_ROOT/dialog-trust-boot.keys")"
contains "the post-dialog hitch delivers its startup contract" \
  "$(pane trust-boot)" "You are trust-boot in Gangline"
contains "the post-dialog hitch reports verified startup delivery" \
  "$(<"$RUN_ROOT/dialog-trust-boot.out")" \
  "delivered startup contract to trust-boot"
submitted "the post-dialog startup contract was submitted" trust-boot
"$GANG" drop trust-boot >/dev/null

modal_observed="test-boot-modal-observed-$$"
modal_painted="test-boot-modal-painted-$$"
modal_clear="test-boot-modal-clear-$$"
modal_probe_release="test-boot-modal-probe-release-$$"
cat > "$RUN_ROOT/collars/boot-modal.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'printf \"FIRST_RUN_MODAL\\n\"; tmux wait-for -S \"$modal_painted\"; tmux wait-for \"$modal_clear\"; PS1=\"❯ \" exec bash --norc' fixture"
GANG_OCCUPIED_REGEX='FIRST_RUN_MODAL'
_gl_modal_real="\$(declare -f collar_input)"
eval "modal_real_input \${_gl_modal_real#collar_input}"
collar_input() {
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
GANG_BOOT_TIMEOUT=5 "$GANG" hitch boot-modal -c boot-modal -d /tmp \
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
submitted "the resumed startup contract was submitted" boot-modal
"$GANG" drop boot-modal >/dev/null

late_observed="test-late-composer-observed-$$"
late_launch="test-late-composer-launch-$$"
late_probe_release="test-late-composer-probe-release-$$"
cat > "$RUN_ROOT/collars/late-composer.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'tmux wait-for \"$late_launch\"; PS1=\"❯ \" exec bash --norc' fixture"
_gl_late_real="\$(declare -f collar_input)"
eval "late_real_input \${_gl_late_real#collar_input}"
collar_input() {
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
GANG_BOOT_TIMEOUT=5 "$GANG" hitch late-composer -c late-composer -d /tmp \
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

cat > "$RUN_ROOT/collars/broken-observer.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_BUSY_REGEX='['
SH
"$HITCH" broken-observer -c broken-observer -d /tmp >/dev/null
if broken_roster="$("$GANG" roster 2>&1)"; then
  fail "roster fails when an agent row cannot be observed" "roster exited successfully"
else
  broken_roster_rc=$?
  equal "roster propagates the observation failure" "1" "$broken_roster_rc"
fi
contains "roster names the collar whose observation failed" \
  "$broken_roster" "broken-observer.sh"
"$GANG" drop broken-observer >/dev/null
contains "startup is one useful contract, not a bookkeeping turn" \
  "$(pane alpha)" "You are alpha in Gangline"
contains "startup ends instead of polling for work" \
  "$(pane alpha)" "End this turn."
# The operative prose is pointed at rather than pasted, so the pane is held to
# naming the file and the sentences are proven where they now live. What each
# line requires is unchanged; only the surface carrying it moved. Newlines fold
# to spaces because the file wraps its sentences and a composer never did.
contract_prose() { tr '\n' ' ' < "$ROOT/CONTRACT.md" | tr -s ' '; }
contains "startup names the contract file it points at" \
  "$(pane alpha)" "CONTRACT.md"
contains "startup orders that contract read before anything else" \
  "$(pane alpha)" "before anything else"
contains "startup gives an unreadable contract a loud stop rather than a guess" \
  "$(pane alpha)" "say so and stop rather than improvising the contract"
contains "the contract says every agent belongs to an addressable team" \
  "$(contract_prose)" \
  "You are one agent on a Gangline team. Run \`gang send --to NAME --stdin\` to address any teammate by name."
contains "the contract makes teammate reachability the shared-state test" \
  "$(contract_prose)" \
  "Put unfinished work and supporting detail in files that teammates can read without you."
contains "the contract lets a result owner hitch help" \
  "$(contract_prose)" "You may hitch teammates to help."
contains "the contract requires one completed-result report" \
  "$(contract_prose)" "Send the lead one report when the result is complete."
excludes "the startup contract no longer spends a line on compaction" \
  "$(pane alpha)" "compact with"
contains "the contract carries the operator-authorized marathon rule" \
  "$(contract_prose)" \
  "When a decision is irreversible or doctrine does not cover it, record the question for the operator. Stop only the affected work and continue everything else."
contains "the contract states the complement of envelope attribution" \
  "$(contract_prose)" \
  "Treat an unenveloped message as session-keyboard input, not as a teammate's message."
contains "the contract makes crossed state explicit before stale instructions act" \
  "$(contract_prose)" \
  "If a teammate's message crossed one you just sent, say so in your next reply and state what is already true before acting on the stale message."
excludes "an absent doctrine leaves no doctrine origin in the base contract" \
  "$(pane alpha)" "Operator doctrine ("
excludes "startup contains no session-marker prompt" "$(pane alpha)" "Session marker"
excludes "startup does not ask for a reply to its synthetic sender" \
  "$(pane alpha)" "Reply to that sender"
equal "context lights leave no threshold state when disabled" "" \
  "$(tmux show-options -wqv -t "$(window_id alpha)" @gl_context_lights)"

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

# A NATIVE EVENT SHAPE GANG CANNOT INTERPRET IS A FAILURE, NOT A NO-OP. Exiting
# 0 with the parse error thrown away tells the harness everything worked while
# Gangline records nothing: a lost Stop leaves the turn bracket open, dispatches
# no deferred self-compaction and drains no spool; lost occupancy evidence
# suppresses stall reporting. The loudness cannot be a nonzero exit — that is
# the agent's own turn to break, for an event that concerns only Gangline — so
# it is stderr plus a fact on the window that status and roster carry.
for hook_shape in 'not json at all' '{"hook_event_name":""}' '{"no_event":"here"}' \
    '{"hook_event_name":"Renamed"}' '{"hook_event_name":"Notification"}'; do
  tmux set-option -uw -t "$alpha_id" @gl_hook_failed
  hook_shape_rc=0
  hook_shape_err="$(printf '%s' "$hook_shape" |
    TMUX_PANE="$alpha_pane_id" "$GANG" hook 2>&1 >/dev/null)" || hook_shape_rc=$?
  equal "an uninterpretable native event does not break the agent's own turn" \
    0 "$hook_shape_rc"
  contains "an uninterpretable native event says so on stderr" \
    "$hook_shape_err" "could not interpret"
  contains "an uninterpretable native event is recorded on its window" \
    "$(tmux show-options -wqv -t "$alpha_id" @gl_hook_failed)" "could not interpret"
done
contains "the uninterpreted-event fact is visible in status" \
  "$("$GANG" status alpha)" "native event NOT interpreted"
contains "roster carries an uninterpreted-event light" \
  "$("$GANG" roster)" "hook-failed"
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$alpha_pane_id" "$GANG" hook >/dev/null 2>&1
equal "an event gang can interpret retires the fact" "" \
  "$(tmux show-options -wqv -t "$alpha_id" @gl_hook_failed)"
excludes "and a well-shaped event stays byte-silent on stderr" \
  "$(printf '%s' '{"hook_event_name":"Stop"}' |
    TMUX_PANE="$alpha_pane_id" "$GANG" hook 2>&1 >/dev/null)" "could not interpret"

# AN UNREADABLE INVOCATION IS DECLINED, AND STDERR ALONE CANNOT PROVE IT. The
# argv branch must stop before the event path, and the notice it prints does not
# witness that: delete only its `exit 0` and the warning still appears, while
# the well-formed Stop below goes on to close the bracket and clear the very
# stamp the notice just wrote — half-honouring restored, with no evidence left
# behind. Measured against exactly that mutant, the two stderr readings above
# stayed green and the four assertions here went red. The payload is well-formed
# on purpose: a decline that needed a broken payload would prove nothing about
# argv.
tmux set-option -uw -t "$alpha_id" @gl_hook_failed
tmux set-option -w -t "$alpha_id" @gl_turn "open $(date +%s)"
hook_argv_rc=0
hook_argv_err="$(printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$alpha_pane_id" "$GANG" hook STRAY 2>&1 >/dev/null)" || hook_argv_rc=$?
equal "an argument gang hook cannot read does not break the harness caller" \
  0 "$hook_argv_rc"
contains "gang hook names an argument it does not take" \
  "$hook_argv_err" "does not take ('STRAY')"
excludes "and does not blame a payload it can read" \
  "$hook_argv_err" "not readable JSON"
equal "the event carried by an unreadable invocation is NOT acted on" \
  "open" "$(tmux show-options -wqv -t "$alpha_id" @gl_turn | cut -d' ' -f1)"
contains "and the decline outlives the event it declined" \
  "$(tmux show-options -wqv -t "$alpha_id" @gl_hook_failed)" "does not take ('STRAY')"
contains "a declined invocation is visible in status" \
  "$("$GANG" status alpha)" "native event NOT interpreted"
contains "and roster carries its light" "$("$GANG" roster)" "hook-failed"
# Leave alpha as the block above left it: stamp retired, bracket closed.
tmux set-option -uw -t "$alpha_id" @gl_hook_failed
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$alpha_pane_id" "$GANG" hook >/dev/null 2>&1

mkdir -p "$CONFIG_CASES/bad-file-value" "$CONFIG_CASES/bad-env-value"
printf '%s\n' 'GANG_BOOT_TIMEOUT=abc' > "$CONFIG_CASES/bad-file-value/config"
if bad_file_out="$(env -u GANG_BOOT_TIMEOUT GANG_CONFIG_DIR="$CONFIG_CASES/bad-file-value" \
  "$GANG" hitch config-bad-file -c bash -d /tmp 2>&1)"; then
  fail "a bad configured value is blamed on its file line" \
    "hitch unexpectedly succeeded"
else
  contains "a bad configured value is blamed on its file line" \
    "$bad_file_out" "from $CONFIG_CASES/bad-file-value/config line 1"
fi
tmux kill-window -t "=$GANG_SESSION:config-bad-file" 2>/dev/null || true
if bad_env_out="$(GANG_BOOT_TIMEOUT=abc GANG_CONFIG_DIR="$CONFIG_CASES/bad-env-value" \
  "$GANG" hitch config-bad-env -c bash -d /tmp 2>&1)"; then
  fail "a bad environment value is blamed on the environment" \
    "hitch unexpectedly succeeded"
else
  contains "a bad environment value is blamed on the environment" \
    "$bad_env_out" "from the environment"
fi
tmux kill-window -t "=$GANG_SESSION:config-bad-env" 2>/dev/null || true

mkdir -p "$CONFIG_CASES/missing-collar"
printf '%s\n' 'GANG_COLLAR=no-such-collar' > "$CONFIG_CASES/missing-collar/config"
if missing_collar_out="$(env -u GANG_COLLAR GANG_CONFIG_DIR="$CONFIG_CASES/missing-collar" \
  "$GANG" hitch config-missing-collar -d /tmp 2>&1)"; then
  fail "an unknown configured collar is blamed on its file line" \
    "hitch unexpectedly succeeded"
else
  contains "an unknown configured collar is blamed on its file line" \
    "$missing_collar_out" "from $CONFIG_CASES/missing-collar/config line 1"
fi
excludes "a refused configured collar opens no window" \
  "$(window_names)" "config-missing-collar"

mkdir -p "$CONFIG_CASES/nested" "$RUN_ROOT/nested-parent-dir" "$RUN_ROOT/nested-child-dir"
printf '%s\n' 'GANG_COLLAR=bash' 'GANG_SESSION=config-nested-session' \
  > "$CONFIG_CASES/nested/config"
env -u GANG_COLLAR -u GANG_SESSION GANG_CONFIG_DIR="$CONFIG_CASES/nested" \
  "$HITCH" nested-parent -d "$RUN_ROOT/nested-parent-dir" >/dev/null
nested_parent_id="$(window_id_in config-nested-session nested-parent)"
nested_done="test-config-nested-done-$$"
tmux send-keys -t "$nested_parent_id" \
  "$GANG hitch nested-child -d '$RUN_ROOT/nested-child-dir' >/dev/null; tmux wait-for -S '$nested_done'" Enter
tmux wait-for "$nested_done"
nested_child_id="$(window_id_in config-nested-session nested-child)"
equal "a nested hitch in another directory reloads the pinned config collar" \
  "bash" "$(tmux show-options -wqv -t "$nested_child_id" @gl_collar)"
contains "the nested hitch joins the session supplied only by the config file" \
  "$(window_names config-nested-session)" "nested-child"

DOCTRINE_CASES="$RUN_ROOT/doctrine-cases"
# S9 makes the ordinary doctrine contract taller than the Bash stand-in's
# original 80x24 composer while the explicit overflow world below remains much
# taller still. Size only this fixture lane; dialog fingerprints above retain
# their calibrated grid.
tmux set-option -g default-size 80x30
mkdir -p "$DOCTRINE_CASES/present"
printf '%s\n' 'MARK_DOCTRINE_PRESENT binds this hitch.' \
  > "$DOCTRINE_CASES/present/DOCTRINE.md"
GANG_CONFIG_DIR="$DOCTRINE_CASES/present" \
  "$HITCH" doctrine-present -c bash -d /tmp >/dev/null
contains "a doctrine-bearing hitch still carries its base identity contract" \
  "$(pane_all doctrine-present)" "You are doctrine-present in Gangline"
contains "a present operator doctrine is injected into the startup contract" \
  "$(pane doctrine-present)" "MARK_DOCTRINE_PRESENT binds this hitch."
# Doctrine is appended to the contract, never a replacement for it: a
# doctrine-bearing hitch must still send its agent to the contract file.
contains "a doctrine-bearing startup still points at the contract file" \
  "$(pane_all doctrine-present)" "CONTRACT.md"
submitted "the doctrine-bearing startup contract was submitted" doctrine-present
"$GANG" drop doctrine-present >/dev/null

GANG_CONFIG_DIR="$DOCTRINE_CASES/present" TMUX_PANE="$alpha_pane_id" \
  "$HITCH" doctrine-inside -c bash -d /tmp >/dev/null
contains "a hitch invoked from inside the team carries operator doctrine" \
  "$(pane doctrine-inside)" "MARK_DOCTRINE_PRESENT binds this hitch."
submitted "the inside-team doctrine contract was submitted" doctrine-inside
"$GANG" drop doctrine-inside >/dev/null

GANG_CONFIG_DIR="$DOCTRINE_CASES/present" GANG_SESSION=doctrine-cross-session \
  TMUX_PANE="$alpha_pane_id" \
  "$HITCH" doctrine-cross -c bash -d /tmp >/dev/null
cross_doctrine_pane="$(tmux capture-pane -pJ \
  -t "$(window_id_in doctrine-cross-session doctrine-cross)")"
contains "a cross-session hitch carries operator doctrine too" \
  "$cross_doctrine_pane" "MARK_DOCTRINE_PRESENT binds this hitch."
equal "the cross-session doctrine contract was submitted" "" \
  "$(GANG_SESSION=doctrine-cross-session "$GANG" composer doctrine-cross)"

mkdir -p "$DOCTRINE_CASES/multiline"
printf '%s\n' 'MARK_DOCTRINE_HEAD' 'middle doctrine line' 'MARK_DOCTRINE_TAIL' \
  > "$DOCTRINE_CASES/multiline/DOCTRINE.md"
GANG_CONFIG_DIR="$DOCTRINE_CASES/multiline" \
  "$HITCH" doctrine-multiline -c bash -d /tmp >/dev/null
contains "a multiline doctrine delivers its first line" \
  "$(pane doctrine-multiline)" "MARK_DOCTRINE_HEAD"
contains "a multiline doctrine delivers its last line" \
  "$(pane doctrine-multiline)" "MARK_DOCTRINE_TAIL"
submitted "the multiline doctrine contract was submitted" doctrine-multiline
"$GANG" drop doctrine-multiline >/dev/null

mkdir -p "$DOCTRINE_CASES/tag"
printf '%s\n' '# [gang: counterfeit] MARK_DOCTRINE_TAG' \
  > "$DOCTRINE_CASES/tag/DOCTRINE.md"
GANG_CONFIG_DIR="$DOCTRINE_CASES/tag" \
  "$HITCH" doctrine-tag -c bash -d /tmp >/dev/null
contains "a tag-shaped doctrine opener is visibly neutralised" \
  "$(pane doctrine-tag)" "# (gang: counterfeit] MARK_DOCTRINE_TAG"
excludes "a doctrine cannot add a second gang envelope opener" \
  "$(pane doctrine-tag)" "[gang: counterfeit]"
submitted "the tag-neutralised doctrine contract was submitted" doctrine-tag
"$GANG" drop doctrine-tag >/dev/null

for bad_doctrine in invalid-utf8 nul control-cr; do
  mkdir -p "$DOCTRINE_CASES/$bad_doctrine"
  case "$bad_doctrine" in
    invalid-utf8) printf '\377' > "$DOCTRINE_CASES/$bad_doctrine/DOCTRINE.md"; expected_doctrine="not valid UTF-8" ;;
    nul) printf 'before\000after' > "$DOCTRINE_CASES/$bad_doctrine/DOCTRINE.md"; expected_doctrine="contains a NUL byte" ;;
    control-cr) printf 'before\rafter' > "$DOCTRINE_CASES/$bad_doctrine/DOCTRINE.md"; expected_doctrine="contains control characters other than tab and newline" ;;
  esac
  if bad_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/$bad_doctrine" \
    "$GANG" hitch "doctrine-$bad_doctrine" -c bash -d /tmp 2>&1)"; then
    fail "a $bad_doctrine doctrine is refused before launch" \
      "hitch unexpectedly succeeded"
  else
    contains "a $bad_doctrine doctrine is refused before launch" \
      "$bad_doctrine_out" "$expected_doctrine"
  fi
  excludes "the refused $bad_doctrine doctrine opens no window" \
    "$(window_names)" "doctrine-$bad_doctrine"
done

mkdir -p "$DOCTRINE_CASES/large"
awk 'BEGIN { for (i = 0; i < 8193; i++) printf "x" }' \
  > "$DOCTRINE_CASES/large/DOCTRINE.md"
if large_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/large" \
  "$GANG" hitch doctrine-large -c bash -d /tmp 2>&1)"; then
  fail "large valid doctrine reaches its consumer's delivery boundary" \
    "hitch unexpectedly succeeded"
else
  contains "large valid doctrine reaches its consumer's delivery boundary" \
    "$large_doctrine_out" "startup contract to 'doctrine-large' was not delivered"
fi
contains "large valid doctrine is not refused before its window opens" \
  "$(window_names)" "doctrine-large"
"$GANG" drop doctrine-large >/dev/null

mkdir -p "$DOCTRINE_CASES/unreadable"
printf '%s\n' 'unreadable doctrine' > "$DOCTRINE_CASES/unreadable/DOCTRINE.md"
chmod 000 "$DOCTRINE_CASES/unreadable/DOCTRINE.md"
if unreadable_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/unreadable" \
  "$GANG" hitch doctrine-unreadable -c bash -d /tmp 2>&1)"; then
  fail "an unreadable doctrine is refused before launch" \
    "hitch unexpectedly succeeded"
else
  contains "an unreadable doctrine is refused before launch" \
    "$unreadable_doctrine_out" "not a readable regular file"
fi
chmod 600 "$DOCTRINE_CASES/unreadable/DOCTRINE.md"
excludes "the unreadable-doctrine refusal leaves no window" \
  "$(window_names)" "doctrine-unreadable"

mkdir -p "$DOCTRINE_CASES/pane-overflow"
awk 'BEGIN { for (i = 0; i < 2048; i++) printf "p" }' \
  > "$DOCTRINE_CASES/pane-overflow/DOCTRINE.md"
if pane_doctrine_out="$(GANG_CONFIG_DIR="$DOCTRINE_CASES/pane-overflow" \
  "$GANG" hitch doctrine-pane-overflow -c bash -d /tmp 2>&1)"; then
  fail "a doctrine the target pane cannot render fails at delivery" \
    "hitch unexpectedly succeeded"
else
  contains "a doctrine the target pane cannot render fails at delivery" \
    "$pane_doctrine_out" "startup contract to 'doctrine-pane-overflow' was not delivered"
fi
contains "a delivery-sized doctrine failure leaves its window for inspection" \
  "$(window_names)" "doctrine-pane-overflow"
"$GANG" drop doctrine-pane-overflow >/dev/null

# Hitch has the same refusal contract as send. Start a fixture whose composer
# already carries the collar's parked-queue evidence, so inject refuses before
# pasting and the public hitch boundary must preserve that status and message.
cat > "$RUN_ROOT/collars/doctrine-prequeued.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="PS1='❯ Press up to edit queued messages' bash --norc"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
SH
doctrine_refusal_rc=0
doctrine_refusal_out="$("$GANG" hitch doctrine-refusal \
  -c doctrine-prequeued -d /tmp 2>&1)" || doctrine_refusal_rc=$?
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
PS1='❯ '
PROMPT_COMMAND='if [ ! -e "$DOCTRINE_QUEUE_SEEN" ]; then : > "$DOCTRINE_QUEUE_SEEN"; elif [ -e "$DOCTRINE_QUEUE_ARM" ]; then PS1="❯ Press up to edit queued messages"; fi'
RC
cat > "$RUN_ROOT/collars/doctrine-queueing.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'DOCTRINE_QUEUE_SEEN=$RUN_ROOT/doctrine-queue-seen DOCTRINE_QUEUE_ARM=$RUN_ROOT/doctrine-queue-arm ENV=$RUN_ROOT/doctrine-queue-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
collar_input() {
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
  "$GANG" hitch doctrine-trailing -c doctrine-queueing -d /tmp 2>&1)"; then
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
trailing_marked="$(
  tmux show-options -wqv -t "$trailing_id" @gl_parked || exit
  printf '\034'
)" || trailing_marked=""
case "$trailing_marked" in
  *$'\034') trailing_body="${trailing_marked%$'\034'}" ;;
  *) fail "the parked doctrine body is readable byte-exactly" \
       "the tmux option read lost its sentinel"; trailing_body="" ;;
esac
trailing_blanks="$(printf '%s' "$trailing_body" | python3 -c '
import sys

body = sys.stdin.buffer.read().decode("utf-8")
if "TAIL_MARK\n" in body:
    after = body.partition("TAIL_MARK\n")[2]
    between, tail, _ = after.partition("End this turn.")
    if tail:
        print(sum(not line.strip() for line in between.splitlines()))
elif r"TAIL_MARK\n" in body:
    after = body.partition(r"TAIL_MARK\n")[2]
    between, tail, _ = after.partition("End this turn.")
    if tail:
        print(between.count(r"\n"))
')"
equal "a doctrine's two trailing blank lines survive byte-exact assembly" \
  "4" "$trailing_blanks"
"$GANG" drop doctrine-trailing >/dev/null

# One optional curfew is team state. Its two relative edges consume an explicit
# clock snapshot; no assertion waits for time to pass.
equal "a team starts without an invented curfew" "no curfew declared" \
  "$("$GANG" curfew)"
if "$GANG" curfew 90 >/dev/null 2>&1; then
  fail "a curfew never guesses the unit of a bare number" "curfew accepted 90"
else
  pass "a curfew never guesses the unit of a bare number"
fi
no_python_path="$RUN_ROOT/no-python-path"
mkdir -p "$no_python_path"
for required_command in date dirname git locale sed tmux; do
  ln -s "$(command -v "$required_command")" "$no_python_path/$required_command"
done
refuses "a clock curfew names a missing python3 dependency" \
  "python3 is required" env PATH="$no_python_path" /bin/bash "$GANG" curfew 09:00
clock_spec="$(python3 - <<'PY'
from datetime import datetime, timedelta

print((datetime.now() + timedelta(minutes=2)).strftime("%H:%M"))
PY
)"
clock_curfew="$("$GANG" curfew "$clock_spec")"
contains "a local clock time declares its next occurrence" "$clock_curfew" "remaining"
declared_curfew="$("$GANG" curfew 1h30m)"
contains "a duration declares the team curfew" "$declared_curfew" "remaining"
curfew_pair="$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_curfew)"
if [[ "$curfew_pair" =~ ^[0-9]+\ [0-9]+$ ]]; then
  pass "the curfew stores one team declaration"
else
  fail "the curfew stores one team declaration" "got [$curfew_pair]"
fi

curfew_now="$(date +%s)"
tmux set-option -t "=$GANG_SESSION:" @gl_curfew "$(( curfew_now + 40 )) $(( curfew_now - 60 ))"
yellow_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
contains "half the declared span exposes a yellow time light" \
  "$yellow_time" "Yellow time light"
excludes "the yellow time light does not prescribe team strategy" \
  "$yellow_time" "converge"
repeat_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
equal "a time light is emitted once per declaration edge" "" "$repeat_time"
tmux set-option -t "=$GANG_SESSION:" @gl_curfew "$(( curfew_now + 10 )) $(( curfew_now - 90 ))"
red_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
contains "four-fifths of the declared span exposes a red time light" \
  "$red_time" "Red time light"
excludes "the red time light does not prescribe checkpoint strategy" \
  "$red_time" "bank"
tmux set-option -t "=$GANG_SESSION:" @gl_curfew unreadable
unavailable_time="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook)"
contains "an unreadable team declaration fails visibly to its agents" \
  "$unavailable_time" "Time lights unavailable"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$(tmux list-panes -t "$(window_id alpha)" -F '#{pane_id}')" "$GANG" hook >/dev/null
equal "a Stop hook writes the raw idle window glyph" "~alpha~" \
  "$(tmux display-message -p -t "$alpha_id" '#{window_name}')"
equal "the operator can remove the team curfew" "curfew cleared" \
  "$("$GANG" curfew clear)"
equal "clearing a curfew restores silence" "no curfew declared" \
  "$("$GANG" curfew)"

: > "$RUN_ROOT/cutoff-alias.err"
cutoff_alias_out="$("$GANG" cutoff 90m 2>> "$RUN_ROOT/cutoff-alias.err")"
contains "gang cutoff still declares the team curfew" "$cutoff_alias_out" \
  "curfew "
equal "one cutoff invocation announces exactly once" "1" \
  "$(grep -c 'gang cutoff is now gang curfew' "$RUN_ROOT/cutoff-alias.err" || true)"
"$GANG" cutoff clear >/dev/null 2>> "$RUN_ROOT/cutoff-alias.err"
equal "two cutoff invocations each announce" "2" \
  "$(grep -c 'gang cutoff is now gang curfew' "$RUN_ROOT/cutoff-alias.err" || true)"

legacy_curfew="$(( $(date +%s) + 600 )) $(( $(date +%s) - 60 ))"
tmux set-option -t "=$GANG_SESSION:" @gl_cutoff "$legacy_curfew"
tmux set-option -u -t "=$GANG_SESSION:" @gl_curfew
contains "a pre-rename team curfew survives its first read" \
  "$("$GANG" curfew)" "curfew "
equal "the curfew read migrates the declaration byte-exact" "$legacy_curfew" \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_curfew)"
equal "the migrated cutoff option is removed" "" \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_cutoff)"
"$GANG" curfew clear >/dev/null

printf 'MARK_ALPHA' | "$GANG" send --to alpha --from tester --stdin >/dev/null
alpha_pane="$(pane alpha)"
contains "verified send reaches the intended pane" "$alpha_pane" "MARK_ALPHA"
contains "the delivered message is attributed" "$alpha_pane" "[gang:tester#"

"$HITCH" inside-target -c bash -d /tmp >/dev/null
printf 'MARK_INSIDE_SENDER' | TMUX_PANE="$alpha_tmux_pane" \
  "$GANG" send --to inside-target --stdin >/dev/null
contains "a sender in a glyphed window is attributed by its bare name" \
  "$(pane inside-target)" "[gang:alpha#"
"$GANG" drop inside-target >/dev/null

# Addressing strips one paired glyph and no more. Exercise every wrapper, the
# minimum three-byte wrapped name, a valid bare name ending in dash, and an
# unpaired human name that must remain literal.
strip_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n '-a-' "printf WRAPPER_TARGET; exec bash")"
for wrapped_name in '-a-' '~a~' '!a!' '?a?'; do
  tmux rename-window -t "$strip_id" -- "$wrapped_name"
  contains "bare addressing resolves wrapper $wrapped_name" \
    "$("$GANG" capture a)" "WRAPPER_TARGET"
done
tmux rename-window -t "$strip_id" -- '-foo--'
contains "paired stripping preserves a bare trailing dash" \
  "$("$GANG" capture foo-)" "WRAPPER_TARGET"
tmux rename-window -t "$strip_id" -- '-notes'
contains "an unpaired leading glyph remains part of the name" \
  "$("$GANG" capture -notes)" "WRAPPER_TARGET"
if unpaired_out="$("$GANG" capture notes 2>&1)"; then
  fail "an unpaired human name is not over-stripped" "resolved as notes"
else
  contains "the unpaired-name miss names the requested bare text" \
    "$unpaired_out" "no window 'notes'"
fi
tmux kill-window -t "$strip_id"

residue_plain_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n residue-plain "printf RESIDUE_PLAIN; exec bash")"
tmux set-option -w -t "$residue_plain_id" @gl_collar bash
residue_roster="$("$GANG" roster)"
contains "the unowned collar-residue window reaches roster observation" \
  "$residue_roster" "residue-plain"
equal "observation never renames a window Gangline does not own" \
  "residue-plain" \
  "$(tmux display-message -p -t "$residue_plain_id" '#{window_name}')"
tmux kill-window -t "$residue_plain_id"

amb_a_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n '-amb-' "printf AMBIGUOUS_A; exec bash")"
amb_b_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n '~amb~' "printf AMBIGUOUS_B; exec bash")"
if ambiguous_status="$("$GANG" status amb 2>&1)"; then
  fail "two windows with one bare address refuse" "status succeeded"
else
  contains "the ambiguity refusal names both raw windows" \
    "$ambiguous_status" "windows -amb- and ~amb~ both address it"
  excludes "ambiguity does not act on either window" "$ambiguous_status" "-busy-"
fi
"$GANG" notify amb >/dev/null
printf '%s\n' '{"hook_event_name":"PermissionRequest"}' \
  | TMUX_PANE="$alpha_tmux_pane" "$GANG" hook \
    > "$RUN_ROOT/ambiguous-hook.out" 2> "$RUN_ROOT/ambiguous-hook.err"
equal "an ambiguous notify target keeps the hook byte-silent" "" \
  "$(<"$RUN_ROOT/ambiguous-hook.err")"
contains "the silent hook records that the notify target is ambiguous" \
  "$(tmux show-options -wqv -t "$alpha_id" @gl_stall_failed)" "ambiguous"
cat > "$RUN_ROOT/collars/occupied-state.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_input() { return 1; }
SH
tmux set-option -w -t "$alpha_id" @gl_collar occupied-state
printf '%s\n' '{"hook_event_name":"PermissionRequest"}' \
  | TMUX_PANE="$alpha_tmux_pane" "$GANG" hook >/dev/null 2>&1
equal "permission occupancy emits the exact snagged state token" \
  "!occupied! (authority unknown)" \
  "$("$GANG" status alpha | head -1)"
tmux set-option -w -t "$alpha_id" @gl_occupied 'open not-a-timestamp'
malformed_occupied_rc=0
malformed_occupied_out="$("$GANG" status alpha 2>&1)" || malformed_occupied_rc=$?
equal "malformed permission evidence refuses instead of becoming absence" \
  "refused named" \
  "$([ "$malformed_occupied_rc" -ne 0 ] && printf refused || printf passed) $([[ "$malformed_occupied_out" = *'malformed @gl_occupied evidence'* ]] && printf named || printf unnamed)"
equal "the malformed permission evidence is not erased" \
  "open not-a-timestamp" \
  "$(tmux show-options -wqv -t "$alpha_id" @gl_occupied)"
tmux set-option -w -t "$alpha_id" @gl_collar bash
tmux set-option -w -t "$alpha_id" @gl_occupied 'open 1'
"$GANG" status alpha >/dev/null
equal "direct composer evidence clears an arbitrarily old permission witness" \
  "" "$(tmux show-options -wqv -t "$alpha_id" @gl_occupied)"
"$GANG" notify clear >/dev/null
tmux set-option -uw -t "$alpha_id" @gl_stall_failed
tmux kill-window -t "$amb_a_id"
tmux kill-window -t "$amb_b_id"

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
if "$GANG" hitch effortless -c bash -d /tmp -e high >/dev/null 2>&1; then
  fail "a collar with no effort spelling refuses -e" "hitch accepted -e"
else
  pass "a collar with no effort spelling refuses -e"
fi
equal "and the refusal leaves no window behind" "" "$(window_id effortless)"

cat > "$RUN_ROOT/collars/noverify.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_EFFORT_OPT='--effort='
SH
if noverify_out="$("$GANG" hitch noverify -c noverify -d /tmp -e low 2>&1)"; then
  fail "a collar that cannot check a level may not take one" "hitch accepted -e"
else
  pass "a collar that cannot check a level may not take one"
fi
contains "and the refusal names the missing declaration" \
  "$noverify_out" "GANG_EFFORT_CMD"
equal "a refused unverifiable effort leaves no window behind" "" \
  "$(window_id noverify)"

cat > "$RUN_ROOT/collars/silent.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_EFFORT_OPT='--effort='
GANG_EFFORT_CMD='true'
SH
if silent_out="$("$GANG" hitch silent -c silent -d /tmp -e low 2>&1)"; then
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
# would open a window on evidence the collar itself declared unreliable.
cat > "$RUN_ROOT/collars/nonzero.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_EFFORT_OPT='--effort='
GANG_EFFORT_CMD='printf "high\n"; exit 17'
SH
if nonzero_out="$("$GANG" hitch effdied -c nonzero -d /tmp -e high 2>&1)"; then
  fail "a checker that fails after printing is refused" "hitch accepted its output"
else
  pass "a checker that fails after printing is refused"
fi
contains "and the refusal names the status, not the operator's level" \
  "$nonzero_out" "failed (status 17)"
equal "a refused failing checker leaves no window behind" "" \
  "$(window_id effdied)"

cat > "$RUN_ROOT/collars/efforted.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_RESUME_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' resume-{{session_id}}"
GANG_EFFORT_OPT='--effort='
GANG_EFFORT_CMD="printf 'low\nmedium\nxhigh\n'"
SH
cat > "$RUN_ROOT/collars/choices.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/efforted.sh"
GANG_MODEL_OPT='--model'
SH
"$GANG" hitch choiceboth -c choices -d /tmp \
  >/dev/null 2> "$RUN_ROOT/choiceboth.err"
contains "a hitch warns for every supported choice the collar would default" \
  "$(<"$RUN_ROOT/choiceboth.err")" "hitching 'choiceboth' without -m and -e"
"$GANG" hitch choicemodel -c choices -d /tmp -e xhigh \
  >/dev/null 2> "$RUN_ROOT/choicemodel.err"
contains "a hitch warns when only its supported model choice is missing" \
  "$(<"$RUN_ROOT/choicemodel.err")" "hitching 'choicemodel' without -m"
"$GANG" drop choiceboth >/dev/null
"$GANG" drop choicemodel >/dev/null
if bogus_out="$("$GANG" hitch effbad -c efforted -d /tmp -e bogus 2>&1)"; then
  fail "a level outside the vocabulary is refused" "hitch accepted bogus"
else
  pass "a level outside the vocabulary is refused"
fi
contains "naming the levels the collar takes" "$bogus_out" "low medium xhigh"
equal "a refused bad level leaves no window behind" "" "$(window_id effbad)"
# The neighbour that makes a substring test look like it works: high is not a
# level here and xhigh is, so an unanchored match would pass a level this
# harness never declared.
if "$GANG" hitch effsub -c efforted -d /tmp -e high >/dev/null 2>&1; then
  fail "a level that is only part of a declared one is refused too" \
    "hitch accepted high"
else
  pass "a level that is only part of a declared one is refused too"
fi
equal "a refused partial level leaves no window behind" "" "$(window_id effsub)"

"$GANG" hitch effok -c efforted -d /tmp -e xhigh \
  >/dev/null 2> "$RUN_ROOT/effok.err"
excludes "a hitch does not demand a model choice its collar cannot take" \
  "$(<"$RUN_ROOT/effok.err")" "hitching 'effok' without -m"
contains "the declared spelling joins the effort into the launch, with no space" \
  "$(tmux display-message -p -t "$(window_id effok)" '#{pane_start_command}')" \
  "--effort=xhigh"
# The other launch form, and POSITION is why it works rather than a second
# branch: the resume swap happens above the append, so both forms carry the
# flag by construction. Asserted anyway — construction is a reason to believe,
# not a receipt, and a flag surviving hitch and lost on the other form would
# be a renewal that quietly changed the agent.
"$HITCH" effres -c efforted -d /tmp --resume fixture-session -e low >/dev/null
effres_line="$(tmux display-message -p -t "$(window_id effres)" '#{pane_start_command}')"
contains "the resume launch form is the one that ran" "$effres_line" "resume-fixture-session"
contains "and it carries the effort too" "$effres_line" "--effort=low"
"$GANG" drop effok >/dev/null
"$GANG" drop effres >/dev/null

# The real claude-code collar driven end-to-end through core: the exact -m
# binding and the joined -e must reach the launch line of a window built from
# the REAL collar's declarations — a core that stopped passing either would
# stay green against fixture collars alone. The harness is a stub on PATH,
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
  "$GANG" hitch realmodel -c claude-code -d /tmp -m claude-opus-5 -e xhigh \
  >/dev/null 2> "$RUN_ROOT/realmodel.err"; then
  fail "a stub that never paints a composer cannot complete a hitch" \
    "hitch reported success"
else
  pass "a stub that never paints a composer cannot complete a hitch"
fi
excludes "a hitch with both choices emits no missing-default warning" \
  "$(<"$RUN_ROOT/realmodel.err")" "hitching 'realmodel' without"
tmux set-environment -g PATH "$tmux_path"
real_launch="$(tmux display-message -p -t "$(window_id realmodel)" '#{pane_start_command}')"
contains "the real collar's launch command is the one that ran" \
  "$real_launch" "claude --settings"
contains "the exact model binds into the real launch line" \
  "$real_launch" "--model claude-opus-5"
contains "and the joined effort rides beside it" \
  "$real_launch" "--effort=xhigh"
"$GANG" drop realmodel >/dev/null

"$HITCH" 1 -c bash -d /tmp >/dev/null
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
# public misdiagnosis. `gang composer` is the collar's styled reading, so what
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
PS1='❯ '
PROMPT_COMMAND='[ -f "$QUEUE_STRAND" ] && PS1="❯ Press up to edit queued messages"'
RC
mkdir -p "$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/queueing.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'QUEUE_STRAND=$RUN_ROOT/queue-strand ENV=$RUN_ROOT/queue-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
SH
export GANG_COLLARS="$RUN_ROOT/collars"
"$HITCH" strand -c queueing -d /tmp >/dev/null
touch "$RUN_ROOT/queue-strand"
if strand_out="$(printf 'MARK_QUEUED' | "$GANG" send --to strand --from tester --stdin 2>&1)"; then
  fail "a submission the harness parks in its queue is not a delivery" \
    "send reported success"
else
  pass "a submission the harness parks in its queue is not a delivery"
fi
contains "the failure names the parked queue" \
  "$strand_out" "parked it in its own input queue"
contains "and hands over the automated recovery" "$strand_out" "gang flush strand"
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
# Recovery is the collar's word too. This collar declares the evidence and no
# recall key, so gang knows the composer is parked and does not know which
# keystroke loads the body back — which is a refusal, never a guessed keypress.
if norecall_out="$("$GANG" flush strand 2>&1)"; then
  fail "a collar with no declared recall key refuses to flush" "flush reported success"
else
  pass "a collar with no declared recall key refuses to flush"
fi
contains "and the refusal names the missing declaration" \
  "$norecall_out" "GANG_QUEUE_RECALL_KEY"
"$GANG" drop strand >/dev/null

# THE PARKED QUEUE, RECOVERED RATHER THAN DESCRIBED. The fixture's composer
# reads as the queue hint while its strand file exists, and the body the
# harness "parked" is the last command in the pane's own history — so the
# collar's declared recall key genuinely loads that body back into the box,
# the way Up does in claude's composer. The drain flag is what a queue entering
# the session looks like from outside: the next prompt after it appears clears
# the strand.
# The queue appears the way a real one does — as a consequence of a submission
# the harness swallowed, not as scenery arranged beforehand. Arming the fixture
# makes the next prompt raise the strand; the composer then reads as the hint
# while it is empty, and as its own contents once the recall key loads them.
# That distinction is the whole subject here, so the hint lives in the collar's
# reader rather than in the prompt string, where it would concatenate with the
# recalled body and make every readback look altered.
cat > "$RUN_ROOT/flush-rc" <<'RC'
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
cat > "$RUN_ROOT/collars/flushable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'FLUSH_STRAND=$RUN_ROOT/flush-strand FLUSH_DRAIN=$RUN_ROOT/flush-drain FLUSH_ARM=$RUN_ROOT/flush-arm FLUSH_SIGNAL=$RUN_ROOT/flush-signal FLUSH_PROBE=$RUN_ROOT/flush-probe FLUSH_PROBE_CHAN=$RUN_ROOT/flush-probe-chan ENV=$RUN_ROOT/flush-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
GANG_QUEUE_RECALL_KEY='Up'
collar_input() { # a composer that spans lines, and reads as the hint when empty
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
"$HITCH" parked -c flushable -d /tmp >/dev/null
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

# A DRAINED QUEUE AND A QUEUE THAT NEVER EXISTED ARE DIFFERENT ANSWERS. Both
# reach flush as an empty record, so an operator reaching for it a moment after
# the harness drained itself would otherwise be told the same sentence as one
# whose message never arrived. The record is retired the way a drain retires
# it — gang reading the composer back empty — rather than by unsetting it. The
# staged note is the obstruction record a real park leaves beside @gl_parked;
# both are present here, and the empty composer is what refutes them.
tmux set-option -w -t "$parked_id" @gl_staged \
  "'MARK_PARKED' was pasted and Enter sent, but the harness parked it in its own input queue"
flush_settle
"$GANG" status parked >/dev/null 2>&1 || :
equal "an empty composer retires the parked record" "" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_parked)"
if drained_record_out="$("$GANG" flush parked 2>&1)"; then
  drained_record_rc=0
else
  drained_record_rc=$?
fi
equal "flushing a drained queue refuses" "3" "$drained_record_rc"
contains "and says the park is gone rather than never recorded" \
  "$drained_record_out" "NOT parked any more"
excludes "so it cannot be read as a message that never arrived" \
  "$drained_record_out" "no record of a message parked"
# Hand the world back the way the next case expects to find it: this one
# retired the record on purpose, and the readback cases below need it.
tmux set-option -w -t "$parked_id" @gl_parked "$parked_record"

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

# COMPACTION IS A BRACKET AND CLOSING IT DELIVERS. The harness will not accept
# input while it compacts, and @gl_turn is CLOSED throughout, so nothing below
# the witness would have said so. What is asserted here is the thing we need
# rather than the thing anyone happened to see: a gang delivery landing
# immediately after PostCompact, not a parked message healing itself.
cat > "$RUN_ROOT/collars/bracketable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
SH
"$HITCH" bracket -c bracketable -d /tmp >/dev/null
bracket_id="$(window_id bracket)"
bracket_pane="$(tmux list-panes -t "$bracket_id" -F '#{pane_id}')"
bracket_hook() { # $1 = event name, $2... = extra JSON body
  printf '{"hook_event_name":"%s"%s}\n' "$1" "${2:-}" \
    | TMUX_PANE="$bracket_pane" "$GANG" hook >/dev/null 2>&1
}
bracket_hook Stop                       # a closed turn: the state gang used to trust
bracket_hook PreCompact ',"trigger":"manual"'
contains "PreCompact opens the compaction bracket" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction)" "open "
equal "and records the cause the harness named" "manual" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction_trigger)"
contains "a compacting agent reads busy though its turn bracket is closed" \
  "$("$GANG" status bracket 2>&1)" "-busy-"
bracket_out="$(printf 'MARK_MIDCOMPACT' \
  | "$GANG" send --to bracket --from tester --stdin 2>&1)" || :
contains "so a delivery into it is queued rather than typed" \
  "$bracket_out" "queued for bracket"
contains "and the message is waiting, not lost" "$("$GANG" roster)" "spooled=1"
excludes "and it has not entered the session before PostCompact" \
  "$(pane bracket)" "MARK_MIDCOMPACT"
# The drain is dispatched, not performed inline, and it signals when it is
# done. Waiting on that barrier is what makes the next two assertions read the
# finished state rather than a race.
tmux wait-for "gang-spool-drain-$bracket_id" &
bracket_waiter=$!
bracket_hook PostCompact
wait "$bracket_waiter"
equal "PostCompact closes the bracket" "closed" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction | cut -d' ' -f1)"
excludes "and closing it drains the spool the compaction filled" \
  "$("$GANG" roster)" "spooled="
# Read from the PANE, not the composer: a delivery that landed has left the
# box empty, because entering the session is what delivery means.
contains "so the message enters the session once the compaction ends" \
  "$(pane bracket)" "MARK_MIDCOMPACT"

# A REFUSED COMPACTION NEVER SENDS THE CLOSING EVENT. claude-code declines a short
# session after PreCompact has already fired, so the opening event cannot be
# trusted to be paired and an unpaired one holds the agent busy until it ages out.
# A turn event settles it: parked input raises nothing until it drains, so a turn
# witnesses a harness that is not compacting.
bracket_hook PreCompact ',"trigger":"manual"'
contains "an unpaired PreCompact leaves the bracket open" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction)" "open "
contains "and the refused compaction still reads busy" \
  "$("$GANG" status bracket 2>&1)" "-busy-"
bracket_hook UserPromptSubmit
equal "a turn opening settles the bracket a refusal left open" "closed" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction | cut -d' ' -f1)"
bracket_hook PreCompact ',"trigger":"manual"'
bracket_hook Stop
equal "and so does a turn ending" "closed" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction | cut -d' ' -f1)"
# Settling is not fabrication: a window that never carried a bracket must not
# acquire one from an ordinary turn.
"$HITCH" unbracketed -c bracketable -d /tmp >/dev/null
unbracketed_pane="$(tmux list-panes -t "$(window_id unbracketed)" -F '#{pane_id}')"
printf '{"hook_event_name":"Stop"}\n' \
  | TMUX_PANE="$unbracketed_pane" "$GANG" hook >/dev/null 2>&1
equal "a turn on a window with no bracket writes none" "" \
  "$(tmux show-options -wqv -t "$(window_id unbracketed)" @gl_compaction)"
"$GANG" drop unbracketed >/dev/null 2>&1 || :
"$GANG" drop bracket >/dev/null 2>&1 || :
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

# QUEUED IS NOT DELIVERED; QUEUED-THEN-WATCHED-TO-DRAIN IS. A collar declaring
# GANG_MIDTURN_INPUT=park says its harness cannot submit during a turn at all —
# it can only queue — so a park there is a landing gang watches rather than a
# failure. The rule the third state must not weaken is the report: "delivered"
# still requires gang to have seen the body leave the queue.
# The fixture raises its hint while the strand exists and LOWERS it when the
# strand goes, because a drain is the whole subject here — the queue-rc above
# only ever raises. The prompt hook also signals a channel once it has redrawn,
# so a test can wait for the new composer rather than sleep for it.
cat > "$RUN_ROOT/steer-rc" <<'RC'
HISTCONTROL=ignorespace
PROMPT_COMMAND='if [ -f "$STEER_STRAND" ]; then PS1="❯ Press up to edit queued messages"; else PS1="❯ "; fi
if [ -s "$STEER_SIGNAL" ]; then _steer_chan="$(cat "$STEER_SIGNAL")"; : > "$STEER_SIGNAL"
  tmux wait-for -S "$_steer_chan"; fi'
RC
cat > "$RUN_ROOT/collars/steering.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'STEER_STRAND=$RUN_ROOT/steer-strand STEER_SIGNAL=$RUN_ROOT/steer-signal ENV=$RUN_ROOT/steer-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
GANG_QUEUE_RECALL_KEY="Up"
GANG_MIDTURN_INPUT=park
# Declared so a refused message can park in gang's OWN spool: spool_available
# gates on it, and the ordering guard below needs a non-empty spool to exist
# before it can prove the native path yields to one. Nothing drains it here —
# this fixture raises no Stop — and nothing in this block asks it to.
GANG_STOP_HOOK=1
SH
: > "$RUN_ROOT/steer-signal"
"$HITCH" steer -c steering -d /tmp >/dev/null
steer_id="$(window_id steer)"
# A WITNESSED turn, not a suspected one. The park path is entered only on a
# positive busy verdict, because an idle target would submit rather than queue
# and the "no hint appeared" guard below would then cry failure over a message
# that landed.
tmux set-option -w -t "$steer_id" @gl_turn "open $(date +%s)"
: > "$RUN_ROOT/steer-strand"
# One look: the fixture's composer is already parked, so the watch finds the
# hint and reports the park rather than waiting out a drain that will not come.
# source-guard: producer@777bc4014c47: every claim here is bound to the window it names — the third-state sentence is read from THIS send's own captured output rather than from any pane, the parked record is read from steer's own @gl_parked, and the roster claims read steer's own row through steer_row rather than the whole table, where another agent's row could have satisfied them
if steer_out="$(printf 'MARK_STEER' | GANG_STEER_LOOKS=1 "$GANG" send --to steer --from tester --stdin 2>&1)"; then
  steer_rc=0
else
  steer_rc=$?
fi
equal "a park mid-turn is accepted, not refused" "0" "$steer_rc"
contains "and is reported as its own state rather than as a delivery" \
  "$steer_out" "parked in the harness queue of steer"
excludes "never as one" "$steer_out" "delivered to steer"
# The BODY, not the hint. This is the reading flush will compare a recalled
# composer against, so it has to be the box with the message in it — the same
# thing submit_parked records on the failing path.
contains "the body gang watched the harness take is recorded" \
  "$(tmux show-options -wqv -t "$steer_id" @gl_parked)" "MARK_STEER"
steer_row() { "$GANG" roster | grep '^steer  *' || :; }
# The fixture redraws its composer on its next prompt, so a strand that has
# gone away only stops showing once the shell has prompted again. Bound to
# steer's own window: flush_settle above belongs to a window this block dropped.
# Empty a steer window's spool before dropping it: a drop archives whatever is
# waiting, and these fixtures are not the subject of the archive guards further
# down. Clearing here keeps this block's leftovers out of their counts.
steer_drop() {
  local d
  d="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$(window_id "$1")" @gl_spool)"
  rm -f "$d"/* 2>/dev/null || :
  "$GANG" drop "$1" >/dev/null
}
steer_settled=0
steer_settle() { # returns once the fixture has redrawn its composer
  steer_settled=$((steer_settled + 1))
  local chan="test-steer-$steer_settled-$$"
  printf '%s' "$chan" > "$RUN_ROOT/steer-signal"
  tmux send-keys -t "$steer_id" Enter
  tmux wait-for "$chan"
}
contains "and roster says so as a boolean, never a count" \
  "$(steer_row)" "parked"
excludes "so nothing invents a number the harness never reports" \
  "$(steer_row)" "parked="

# THE THIRD STATE IS CLOSED BY A LATER READ, not by a watcher. The strand going
# away is what a drained queue looks like from outside; the next command that
# reports on the window re-reads the composer and retires the record.
rm -f "$RUN_ROOT/steer-strand"
steer_settle
excludes "a drained queue stops being reported as parked" \
  "$(steer_row)" "parked"
equal "and the record is retired rather than left standing" "" \
  "$(tmux show-options -wqv -t "$steer_id" @gl_parked)"
equal "leaving the mark that says it drained, not that it never existed" "1" \
  "$(tmux show-options -wqv -t "$steer_id" @gl_parked_drained)"

# THE SPOOL IS THE ORDERING AUTHORITY. A body typed into the harness now drains
# at that agent's next batch, while anything already spooled waits for its Stop —
# so parking ahead of a waiting message would deliver the newer one first.
: > "$RUN_ROOT/steer-strand"
# Seed the spool the only way a park collar can: a refusal that happens before
# any keystroke. A half-written draft is one, so this message is spooled rather
# than typed, and the next send meets a NON-EMPTY spool.
tmux send-keys -l -t "$steer_id" "HUMAN_DRAFT_HOLDING_THE_BOX"
printf 'MARK_OLDER' | "$GANG" send --to steer --from tester --stdin >/dev/null 2>&1 || :
steer_spooled="$(steer_row)"
contains "an older message waiting makes the next one spool behind it" \
  "$steer_spooled" "spooled="
tmux send-keys -t "$steer_id" C-u
steer_settle
tmux set-option -w -t "$steer_id" @gl_turn "open $(date +%s)"
if steer_order="$(printf 'MARK_NEWER' | GANG_STEER_LOOKS=1 "$GANG" send --to steer --from tester --stdin 2>&1)"; then
  steer_order_rc=0
else
  steer_order_rc=$?
fi
equal "the later message is accepted" "0" "$steer_order_rc"
excludes "but never by jumping the queue into the harness" \
  "$steer_order" "parked in the harness queue"
contains "it joins the spool the older one is waiting in" "$steer_order" "queued for steer"

# ONE OUTSTANDING BODY. The hint says something is parked and never what or how
# many, and the recall key returns the whole queue at once, so a second body
# would leave gang unable to say which one its record names.
steer_drop steer
# The strand is shared fixture state: leave it raised and the next window boots
# into a composer that already reads as parked, so its startup contract cannot
# be delivered. Each window raises it only once it is up.
rm -f "$RUN_ROOT/steer-strand"
"$HITCH" steer2 -c steering -d /tmp >/dev/null
steer2_id="$(window_id steer2)"
: > "$RUN_ROOT/steer-strand"
tmux set-option -w -t "$steer2_id" @gl_turn "open $(date +%s)"
printf 'MARK_FIRST' | GANG_STEER_LOOKS=1 "$GANG" send --to steer2 --from tester --stdin >/dev/null 2>&1 || :
if steer_second="$(printf 'MARK_SECOND' | GANG_STEER_LOOKS=1 "$GANG" send --to steer2 --from tester --stdin 2>&1)"; then :; fi
excludes "a second body is not parked on top of the first" \
  "$steer_second" "parked in the harness queue"
contains "it waits in the spool instead, named for the reason" \
  "$steer_second" "queued for steer2"

# --supersede RETIRES AN ATTRIBUTED MESSAGE, and the harness's queue is
# anonymous: nothing gang types retracts one body from it. Spooling keeps the
# flag's promise exactly rather than half-honouring it.
steer_drop steer2
rm -f "$RUN_ROOT/steer-strand"
"$HITCH" steer3 -c steering -d /tmp >/dev/null
: > "$RUN_ROOT/steer-strand"
tmux set-option -w -t "$(window_id steer3)" @gl_turn "open $(date +%s)"
if steer_sup="$(printf 'MARK_SUP' | GANG_STEER_LOOKS=1 "$GANG" send --to steer3 --from tester --supersede --stdin 2>&1)"; then :; fi
excludes "--supersede never parks in a queue it cannot retract from" \
  "$steer_sup" "parked in the harness queue"
contains "and says which promise it is keeping" "$steer_sup" "queued for steer3"

# --live-only IS THE PROBE FOR A DELIVERY THAT MUST NOT PARK, and this path can
# only park. Its answer stays the NOT-parked line, with nothing typed.
if steer_live="$(printf 'MARK_LIVE' | "$GANG" send --to steer3 --from tester --live-only --stdin 2>&1)"; then
  steer_live_rc=0
else
  steer_live_rc=$?
fi
equal "--live-only refuses rather than parking" "3" "$steer_live_rc"
contains "naming the mid-turn reason" "$steer_live" "not safely reachable mid-turn"

# FAIL LOUD WHEN THE DECLARED EVIDENCE STOPS MATCHING. Mid-turn input on a park
# collar can only queue, so an empty composer after the Enter is not proof of a
# submission — it is a rendering gang no longer models. Reporting a delivery
# there would be the one lie this whole path exists to prevent.
#
# A FRESH window, because the ordering guard above is checked first: a target
# already carrying spooled work refuses before it ever types, and this case has
# to reach the typing to say anything about the evidence.
steer_drop steer3
rm -f "$RUN_ROOT/steer-strand"
"$HITCH" steer4 -c steering -d /tmp >/dev/null
tmux set-option -w -t "$(window_id steer4)" @gl_turn "open $(date +%s)"
if steer_blind="$(printf 'MARK_BLIND' | GANG_STEER_LOOKS=1 "$GANG" send --to steer4 --from tester --stdin 2>&1)"; then
  steer_blind_rc=0
else
  steer_blind_rc=$?
fi
if [ "$steer_blind_rc" -eq 0 ]; then
  fail "a park collar whose hint never appears fails loudly" "send reported success"
else
  pass "a park collar whose hint never appears fails loudly"
fi
contains "naming the declaration that stopped matching" \
  "$steer_blind" "GANG_QUEUED_REGEX"
excludes "and never calling it a delivery" "$steer_blind" "delivered to steer4"
steer_drop steer4

# A keystroke gang cannot send by name is a broken declaration, refused before
# any window opens: tmux would deliver the letters into the composer instead.
cat > "$RUN_ROOT/collars/badkey.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_QUEUE_RECALL_KEY='M-x then y'
SH
if badkey_out="$("$GANG" hitch badkey -c badkey -d /tmp 2>&1)"; then
  fail "a recall key that is not a tmux key name is refused" "hitch accepted it"
else
  pass "a recall key that is not a tmux key name is refused"
fi
contains "naming the declaration rather than the operator" \
  "$badkey_out" "GANG_QUEUE_RECALL_KEY"
equal "and the refused declaration leaves no window behind" "" "$(window_id badkey)"

# INTERRUPTING IS A COLLAR'S KEYSTROKE AND A FACT GANG OWNS. The keystroke
# ends a turn the harness will never close for itself, so the bracket it opened
# has to be closed here or the target reads busy until that bound expires.
# Whether the harness actually stopped remains its own verdict; the fact does
# not claim otherwise.
if nokey_out="$("$GANG" interrupt alpha 2>&1)"; then
  fail "a collar with no declared interrupt key refuses to interrupt" \
    "interrupt reported success"
else
  pass "a collar with no declared interrupt key refuses to interrupt"
fi
contains "naming the declaration it would need" "$nokey_out" "GANG_INTERRUPT_KEY"

cat > "$RUN_ROOT/collars/badstop.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_INTERRUPT_KEY='ctrl then c'
SH
if badstop_out="$("$GANG" hitch badstop -c badstop -d /tmp 2>&1)"; then
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
# key to the COLLAR's declaration rather than to anything hard-coded in core:
# the bound key is C-g, so an interrupt that sent Escape would leave no mark.
cat > "$RUN_ROOT/interrupt-rc" <<'RC'
PS1='❯ '
bind -x '"\C-g": printf "INTERRUPT_KEY_RECEIVED\n"'
RC
cat > "$RUN_ROOT/collars/interruptible.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'ENV=$RUN_ROOT/interrupt-rc exec bash --posix' fixture"
GANG_INTERRUPT_KEY='C-g'
GANG_BUSY_REGEX='STILL_WORKING'
SH
"$HITCH" stoppable -c interruptible -d /tmp >/dev/null
stop_id="$(window_id stoppable)"
tmux set-option -w -t "$stop_id" @gl_turn "open $(date +%s)"
contains "an open turn bracket answers busy" \
  "$("$GANG" status stoppable)" "-busy-"
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
  "$("$GANG" status stoppable)" "~idle~"
"$GANG" drop stoppable >/dev/null

# AND IT MUST NOT MANUFACTURE THE OPPOSITE LIE. Gang saw a keystroke leave; it
# did not see a turn end. A harness that ignored the key is still painting its
# busy marker, and that evidence has to survive the interrupt — writing a fresh
# closed bracket would answer idle before anything looked at the pane, and the
# next send would enter mid-turn on gang's own say-so.
"$HITCH" stubborn -c interruptible -d /tmp >/dev/null
stubborn_id="$(window_id stubborn)"
tmux send-keys -l -t "$stubborn_id" 'printf STILL_WORKING\\n'
tmux send-keys -t "$stubborn_id" Enter
tmux set-option -w -t "$stubborn_id" @gl_turn "open $(date +%s)"
"$GANG" interrupt stubborn >/dev/null
equal "the bracket is dropped there too" "" \
  "$(tmux show-options -wqv -t "$stubborn_id" @gl_turn)"
stubborn_state="$("$GANG" status stubborn)"
contains "a target still painting work stays busy after the interrupt" \
  "$stubborn_state" "-busy-"
excludes "the interrupt never invents an idle the pane contradicts" \
  "$stubborn_state" "~idle~"
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
for stopping_collar in claude-code codex; do
  stopping_file="$ROOT/collars/$stopping_collar.sh"
  stopping_key="$(GANG_TEST_COLLARS='' ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
    '. "$1"; printf "%s" "${GANG_INTERRUPT_KEY:-}"' fixture "$stopping_file")"
  equal "the $stopping_collar collar declares the key that stops its turn" \
    "Escape" "$stopping_key"
done
for unstopping_collar in opencode pi; do
  unstopping_file="$ROOT/collars/$unstopping_collar.sh"
  unstopping_key="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
    '. "$1"; printf "%s" "${GANG_INTERRUPT_KEY:-}"' fixture "$unstopping_file")"
  equal "the $unstopping_collar collar declares no interrupt key until one is verified" \
    "" "$unstopping_key"
done

# SPOOLED DELIVERY. A refused delivery is a live target that cannot take input
# yet, and every refusal happens before a keystroke — so the body is still the
# sender's and can be parked. Nothing in this world polls or schedules: the only
# thing that drains a spool is a native Stop event, which the world fires by
# hand exactly as a harness would.
cat > "$RUN_ROOT/collars/nodrain.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
SH
"$HITCH" nodrain -c nodrain -d /tmp >/dev/null
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

cat > "$RUN_ROOT/collars/spoolable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_STOP_HOOK=1
_gl_spool_real="\$(declare -f collar_input)"
eval "spool_real_input \${_gl_spool_real#collar_input}"
collar_input() { # once per armed drain, report what the spool and the lock look like
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
"$HITCH" parker -c spoolable -d /tmp >/dev/null
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
contains "a refused delivery is parked rather than lost" "$spool_out" "queued for parker"
# THE DECISION THIS GUARD RECORDS SURVIVES; ONLY ITS WORDING MOVES. It was
# written so a parked message can never be reported as delivered, and that is
# still enforced below. What changed is that "NOT delivered" was the whole
# headline, and read beside "refused" and "wait for it to become idle" it told
# a sender their message had failed when it had been accepted — one routed a
# report around gang to escape it. The old expectation was defensible clause by
# clause and wrong in composition, which is why it is the wording that moves
# and not the rule.
contains "and does not let it be read as delivered" "$spool_out" "not yet in the session"
excludes "and never calls an accepted message refused" "$spool_out" "refused"
contains "it says what the sender must now do, which is nothing" \
  "$spool_out" "nothing further is needed from you"
# The drain is conditional on a turn nobody has taken yet, so the promise is
# conditional too: an agent that takes no further turn never drains, and the
# message must not claim otherwise.
contains "and promises the drain only on a turn actually being completed" \
  "$spool_out" "next completes a turn"
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
contains "and the deprecated form still parks" "$spool_noop_out" "queued for parker"
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
  BASH_ENV="$RUN_ROOT/no-bashpid" TMUX_PANE="$parker_pane_id" \
    "$GANG" hook >/dev/null
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
  "$claim_observed" "claimed=2"

# A larger queue is claimed as one delivery before the target's composer is read.
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_BUNDLE_ONE' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
printf 'MARK_BUNDLE_TWO' |
  "$GANG" send --to parker --from other --stdin >/dev/null
printf 'MARK_BUNDLE_THREE' |
  "$GANG" send --to parker --from third --stdin >/dev/null
tmux send-keys -t "$parker_id" C-u
: > "$RUN_ROOT/claim-watch"
tmux wait-for "gang-spool-drain-$parker_id" &
bundle_drain_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$bundle_drain_waiter"
bundle_claim_observed="$(cat "$RUN_ROOT/claim-observed")"
contains "one drain claims a three-message queue before reading the composer" \
  "$bundle_claim_observed" "claimed=3"
bundle_pane="$(pane parker)"
bundle_order="$(printf '%s\n' "$bundle_pane" |
  grep -oE 'MARK_BUNDLE_ONE|MARK_BUNDLE_TWO|MARK_BUNDLE_THREE' |
  awk '!seen[$0]++' | tr '\n' ' ')"
equal "the three-message bundle stays in stamp order" \
  "MARK_BUNDLE_ONE MARK_BUNDLE_TWO MARK_BUNDLE_THREE " "$bundle_order"

# A refused bundle returns every claim to its own live stamp.
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_REFUSED_ONE' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
printf 'MARK_REFUSED_TWO' |
  "$GANG" send --to parker --from other --stdin >/dev/null
printf 'MARK_REFUSED_THREE' |
  "$GANG" send --to parker --from third --stdin >/dev/null
tmux wait-for "gang-spool-drain-$parker_id" &
refused_bundle_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$refused_bundle_waiter"
contains "a refused bundle returns every message to the waiting queue" \
  "$("$GANG" status parker)" "spooled: 3"
refused_sending=0
for refused_entry in "$parker_spool_dir"/sending-*; do
  [ -f "$refused_entry" ] && refused_sending=$((refused_sending + 1))
done
equal "a refused bundle leaves no entry claimed" "0" "$refused_sending"
tmux send-keys -t "$parker_id" C-u
tmux wait-for "gang-spool-drain-$parker_id" &
refused_cleanup_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$refused_cleanup_waiter"

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
parker_failed="$parker_spool/failed-00000000000000000002-deadbeef"
printf '%s\n%s\n%s\n' other MARK_ARCHIVED_HELD \
  '[gang:other#deadbeef] MARK_ARCHIVED_HELD [/gang:other#deadbeef]' \
  > "$parker_failed"
parker_archive_names="$(cd "$parker_spool" && ls)"
parker_live_entry=""
for parker_entry in "$parker_spool"/[0-9]*; do
  [ -f "$parker_entry" ] || continue
  parker_live_entry="$parker_entry"
  break
done
[ -n "$parker_live_entry" ] \
  && cp "$parker_live_entry" "$RUN_ROOT/pre-archive-body"
"$GANG" drop parker >/dev/null
[ ! -d "$parker_spool" ] \
  && pass "dropping an agent deletes its spool" \
  || fail "dropping an agent deletes its spool" "$parker_spool survived"
parker_archive_count=0
parker_archive_dir=""
for archive_dir in "$GANG_ARCHIVE_DIR"/*; do
  [ -d "$archive_dir" ] || continue
  parker_archive_count=$((parker_archive_count + 1))
  parker_archive_dir="$archive_dir"
done
equal "dropping mail creates exactly one teardown archive" "1" \
  "$parker_archive_count"
[ -d "$parker_archive_dir/parker" ] \
  && pass "the teardown archive groups entries under the agent name" \
  || fail "the teardown archive groups entries under the agent name" \
    "$parker_archive_dir/parker is absent"
equal "the teardown archive preserves every entry filename" \
  "$parker_archive_names" "$(cd "$parker_archive_dir/parker" && ls)"
if cmp "$RUN_ROOT/pre-archive-body" \
  "$parker_archive_dir/parker/${parker_live_entry##*/}"; then
  pass "the archived entry preserves the composed body byte for byte"
else
  fail "the archived entry preserves the composed body byte for byte" \
    "archived bytes differ"
fi
[ -f "$parker_archive_dir/parker/${parker_failed##*/}" ] \
  && pass "a held failed entry is archived with waiting mail" \
  || fail "a held failed entry is archived with waiting mail" \
    "${parker_failed##*/} is absent"

"$HITCH" archive-second -c spoolable -d /tmp >/dev/null
archive_second_id="$(window_id archive-second)"
tmux send-keys -l -t "$archive_second_id" 'HUMAN_DRAFT'
printf 'MARK_SECOND_ARCHIVE' |
  "$GANG" send --to archive-second --from tester --stdin >/dev/null
"$GANG" drop archive-second >/dev/null
archive_count=0
for archive_dir in "$GANG_ARCHIVE_DIR"/*; do
  [ -d "$archive_dir" ] && archive_count=$((archive_count + 1))
done
equal "a second teardown claims a distinct archive directory" "2" \
  "$archive_count"

"$HITCH" archive-empty -c spoolable -d /tmp >/dev/null
"$GANG" drop archive-empty >/dev/null
archive_count_after_empty=0
for archive_dir in "$GANG_ARCHIVE_DIR"/*; do
  [ -d "$archive_dir" ] && archive_count_after_empty=$((archive_count_after_empty + 1))
done
equal "dropping an empty queue creates no archive directory" \
  "$archive_count" "$archive_count_after_empty"

# AN AGENT NAME BECOMES A PATH COMPONENT. spool_archive names the archive
# subdirectory after the agent whose mail it is holding, so a "name" carrying a
# parent reference moves pending messages out of the archive root entirely — on
# a plain gang drop, with no error, into a directory nobody chose. Identity is
# read from the registration now, so the registration is what gets validated
# before it is joined to a path: the check sits at the boundary that builds the
# destination, where no reader can route around it.
"$HITCH" traversal -c spoolable -d /tmp >/dev/null
traversal_id="$(window_id traversal)"
tmux send-keys -l -t "$traversal_id" 'HUMAN_DRAFT'
printf 'MARK_TRAVERSAL' |
  "$GANG" send --to traversal --from tester --stdin >/dev/null
traversal_escape="$RUN_ROOT/outside"
rm -rf -- "$traversal_escape"
tmux rename-window -t "$traversal_id" '../../outside'
tmux set-option -w -t "$traversal_id" @gl_agent '../../outside'
"$GANG" drop '../../outside' >/dev/null 2>&1 || true
traversal_archived=""
for archived_traversal in "$GANG_ARCHIVE_DIR"/*/*/[0-9]*; do
  [ -f "$archived_traversal" ] || continue
  grep -q MARK_TRAVERSAL "$archived_traversal" 2>/dev/null \
    && traversal_archived="$archived_traversal"
done
equal "a registration that is not a name never becomes an archive path" \
  "contained archived" \
  "$([ -e "$traversal_escape" ] && printf escaped || printf contained) $([ -n "$traversal_archived" ] && printf archived || printf lost)"
tmux kill-window -t "$traversal_id" 2>/dev/null || true

# A FOREIGN MAIL READ INSPECTS THE QUEUE AND NOTHING ELSE. It does not load the
# target's collar, claim entries, take the pane lock, or attempt delivery. A
# self-read below consumes, but archives before a byte reaches stdout.
"$HITCH" mailer -c spoolable -d /tmp >/dev/null
mailer_id="$(window_id mailer)"
tmux send-keys -l -t "$mailer_id" 'HUMAN_DRAFT'
printf 'MARK_MAIL_ONE' |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
printf 'MARK_MAIL_TWO' |
  "$GANG" send --to mailer --from other --stdin >/dev/null
printf 'MARK_MAIL_THREE' |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
mailer_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$mailer_id" @gl_spool)"
mail_names_before="$(cd "$mailer_spool" && ls)"
mail_status_before="$("$GANG" status mailer)"
mail_out="$("$GANG" mail mailer)"
contains "mail prints the first waiting body" "$mail_out" "MARK_MAIL_ONE"
contains "mail prints the second waiting body" "$mail_out" "MARK_MAIL_TWO"
contains "mail prints the third waiting body" "$mail_out" "MARK_MAIL_THREE"
contains "mail names the first sender" "$mail_out" "from tester"
contains "mail names the other sender" "$mail_out" "from other"
mail_order="$(printf '%s\n' "$mail_out" |
  grep -oE 'MARK_MAIL_ONE|MARK_MAIL_TWO|MARK_MAIL_THREE' |
  awk '!seen[$0]++' | tr '\n' ' ')"
equal "mail prints waiting bodies in stamp order" \
  "MARK_MAIL_ONE MARK_MAIL_TWO MARK_MAIL_THREE " "$mail_order"
mail_status_after="$("$GANG" status mailer)"
contains "mail leaves the waiting count unchanged before its read" \
  "$mail_status_before" "spooled: 3"
contains "mail leaves the waiting count unchanged after its read" \
  "$mail_status_after" "spooled: 3"
equal "mail leaves every entry filename untouched" "$mail_names_before" \
  "$(cd "$mailer_spool" && ls)"

tmux set-option -w -t "$mailer_id" @gl_collar no-such-collar
mail_without_collar="$("$GANG" mail mailer)"
contains "mail reads bodies without loading the target collar" \
  "$mail_without_collar" "MARK_MAIL_ONE"
if missing_collar_status="$("$GANG" status mailer 2>&1)"; then
  fail "the same target proves its collar is not loadable" \
    "status unexpectedly succeeded"
else
  pass "the same target proves its collar is not loadable"
fi
contains "status fails specifically on the missing collar" \
  "$missing_collar_status" "unknown collar 'no-such-collar'"
tmux set-option -w -t "$mailer_id" @gl_collar spoolable

mailer_failed="$mailer_spool/failed-00000000000000000005-facefeed"
printf '%s\n%s\n%s\n' other MARK_MAIL_HELD \
  '[gang:other#facefeed] MARK_MAIL_HELD [/gang:other#facefeed]' \
  > "$mailer_failed"
mail_with_held="$("$GANG" mail mailer)"
contains "mail prints a held body" "$mail_with_held" "MARK_MAIL_HELD"
contains "mail labels held delivery as unverified" \
  "$mail_with_held" "held (delivery NOT verified"
# READING YOUR OWN QUEUE CONSUMES IT. A message the addressee has already read
# is delivered again at its next turn boundary — the same body twice, once by
# hand and once by the spool. Reading it IS its delivery, so the read retires
# what it printed. Only for its own agent: a read by anybody else is an
# inspection and stays non-destructive.
mailer_bodies() { # $1 = a mail rendering -> the marks it printed, in order
  printf '%s\n' "$1" | grep -oE 'MARK_MAIL_[A-Z]+' | tr '\n' ' ' || true
}
mailer_self_pane="$(tmux list-panes -t "$(window_id mailer)" -F '#{pane_id}')"
mail_reference="$("$GANG" mail mailer)"
mailer_self_rc=0
TMUX_PANE="$mailer_self_pane" "$GANG" mail > "$RUN_ROOT/mail-self.out" 2>&1 \
  || mailer_self_rc=$?
mailer_self_out="$(grep -v '^gang: WARNING: executing dirty ' \
  "$RUN_ROOT/mail-self.out" || true)"
equal "bare mail reads the calling agent's own queue" \
  "0|$(mailer_bodies "$mail_reference")" \
  "$mailer_self_rc|$(mailer_bodies "$mailer_self_out")"
contains "a self-read says what it consumed, not what waits" \
  "$mailer_self_out" "consumed"
equal "a self-read retires exactly the waiting entries it printed" \
  "failed-00000000000000000005-facefeed" "$(cd "$mailer_spool" && ls)"
mail_after_self="$("$GANG" mail mailer)"
excludes "the consumed bodies are gone from the queue" \
  "$mail_after_self" "MARK_MAIL_ONE"
excludes "every consumed body, not merely the first" \
  "$mail_after_self" "MARK_MAIL_THREE"
contains "a self-read never consumes a held entry" \
  "$mail_after_self" "MARK_MAIL_HELD"
contains "and the held entry keeps saying delivery was not verified" \
  "$mail_after_self" "held (delivery NOT verified"

# An entry that lands after the read is not one the read printed, so nothing
# retires it: the next read is where it appears.
printf 'MARK_MAIL_LATER' |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
mail_later_rc=0
TMUX_PANE="$mailer_self_pane" "$GANG" mail > "$RUN_ROOT/mail-later.out" 2>&1 \
  || mail_later_rc=$?
mail_later_out="$(grep -v '^gang: WARNING: executing dirty ' \
  "$RUN_ROOT/mail-later.out" || true)"
equal "a message that arrives after a self-read survives it" \
  "0|MARK_MAIL_LATER MARK_MAIL_HELD " \
  "$mail_later_rc|$(mailer_bodies "$mail_later_out")"
equal "and the second read retires only what the second read printed" \
  "failed-00000000000000000005-facefeed" "$(cd "$mailer_spool" && ls)"

# MALFORMED SPOOL BYTES FAIL BEFORE CLAIM. This entry cannot be produced by
# spool_write, but corruption must not turn its known undelivered state into a
# held verdict saying it may have arrived.
mail_malformed="$mailer_spool/00000000000000000006-malformed"
printf '%s\n%s\n' tester malformed-fragment > "$mail_malformed"
mail_malformed_before="$(cd "$mailer_spool" && ls)"
mail_malformed_rc=0
TMUX_PANE="$mailer_self_pane" "$GANG" mail \
  >"$RUN_ROOT/mail-malformed.out" 2>&1 || mail_malformed_rc=$?
[ "$mail_malformed_rc" -ne 0 ] \
  && pass "a bodyless spool entry refuses a self-read" \
  || fail "a bodyless spool entry refuses a self-read" \
    "mail unexpectedly returned zero"
equal "a bodyless entry stays waiting under its original name" \
  "$mail_malformed_before" "$(cd "$mailer_spool" && ls)"
excludes "a bodyless entry creates no false held verdict" \
  "$(cd "$mailer_spool" && ls)" "sending-00000000000000000006-malformed"
rm -f -- "$mail_malformed"

# ARCHIVE FAILURE PRECEDES CLAIM. A self-read cannot turn mail with a known
# undelivered fate into a sending-/held entry merely because its recovery
# destination is misconfigured.
printf 'MARK_MAIL_ARCHIVE_REFUSAL' |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
mail_archive_refusal_before="$(cd "$mailer_spool" && ls)"
mail_archive_blocker="$RUN_ROOT/archive-not-a-directory"
: > "$mail_archive_blocker"
mail_archive_refusal_rc=0
GANG_ARCHIVE_DIR="$mail_archive_blocker" TMUX_PANE="$mailer_self_pane" \
  "$GANG" mail >"$RUN_ROOT/mail-archive-refusal.out" 2>&1 \
  || mail_archive_refusal_rc=$?
[ "$mail_archive_refusal_rc" -ne 0 ] \
  && pass "an unwritable archive refuses a self-read" \
  || fail "an unwritable archive refuses a self-read" \
    "mail unexpectedly returned zero"
contains "the archive refusal names the unusable recovery path" \
  "$(<"$RUN_ROOT/mail-archive-refusal.out")" "$mail_archive_blocker"
equal "archive refusal leaves the waiting spool byte-for-byte named" \
  "$mail_archive_refusal_before" "$(cd "$mailer_spool" && ls)"
excludes "archive refusal creates no false held delivery verdict" \
  "$(cd "$mailer_spool" && ls)" "sending-"
rm -f -- "$mail_archive_blocker"
mail_archive_recovery="$(TMUX_PANE="$mailer_self_pane" "$GANG" mail 2>&1)"
contains "the untouched message remains readable after archive repair" \
  "$mail_archive_recovery" "MARK_MAIL_ARCHIVE_REFUSAL"

# A SHELL FILTER MAY HIDE OUTPUT, BUT IT CANNOT DESTROY THE ONLY COPY. This is
# the live incident shape: tail consumes all of gang mail's stdout and prints
# only its end. The body prefix is absent from the filtered view, while the
# archive named on unfiltered stderr keeps the exact complete envelope.
mail_filter_body="MARK_MAIL_FILTER_HEAD
$(printf 'filter filler %02d\n' {1..40})
MARK_MAIL_FILTER_TAIL"
printf '%s' "$mail_filter_body" |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
TMUX_PANE="$mailer_self_pane" "$GANG" mail \
  2>"$RUN_ROOT/mail-filter.err" | tail -20 >"$RUN_ROOT/mail-filter.out"
excludes "tail hides the head of a long self-read as the incident requires" \
  "$(<"$RUN_ROOT/mail-filter.out")" "MARK_MAIL_FILTER_HEAD"
contains "the destructive read names its archive outside the stdout pipe" \
  "$(<"$RUN_ROOT/mail-filter.err")" "self-mail is destructive"
contains "the archive notice carries its deletion path" \
  "$(<"$RUN_ROOT/mail-filter.err")" "delete this read archive after recovery with: rm -rf --"
mail_filter_archive=""
for mail_archived_entry in "$GANG_ARCHIVE_DIR"/*/mailer/[0-9]*; do
  [ -f "$mail_archived_entry" ] || continue
  grep -q MARK_MAIL_FILTER_HEAD "$mail_archived_entry" \
    && mail_filter_archive="$mail_archived_entry"
done
[ -n "$mail_filter_archive" ] \
  && pass "the filtered-away head survives in the named archive" \
  || fail "the filtered-away head survives in the named archive" \
    "no archived entry contains MARK_MAIL_FILTER_HEAD"
contains "the archived entry keeps the filtered message's tail too" \
  "$(<"$mail_filter_archive")" "MARK_MAIL_FILTER_TAIL"

# Somebody else's read is an inspection. lead reading a teammate's queue must
# leave every entry exactly where the teammate's own next turn will find it.
printf 'MARK_MAIL_FOREIGN' |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
mail_foreign_before="$(cd "$mailer_spool" && ls)"
mail_foreign_out="$(TMUX_PANE="$alpha_tmux_pane" "$GANG" mail mailer)"
contains "another agent's read still prints the waiting body" \
  "$mail_foreign_out" "MARK_MAIL_FOREIGN"
equal "another agent's read consumes nothing" \
  "$mail_foreign_before" "$(cd "$mailer_spool" && ls)"
equal "and an operator outside the team consumes nothing either" \
  "$mail_foreign_before" \
  "$("$GANG" mail mailer >/dev/null; cd "$mailer_spool" && ls)"
"$GANG" drop mailer >/dev/null

"$HITCH" empty-mailbox -c spoolable -d /tmp >/dev/null
empty_mail_out="$("$GANG" mail empty-mailbox)"
contains "mail exits cleanly on an empty queue" \
  "$empty_mail_out" "no mail waiting for empty-mailbox"
"$GANG" drop empty-mailbox >/dev/null

# The legacy-contract fixtures finished their assertions above. Retire them so
# this roster probe's stderr belongs only to the age world under test.
"$GANG" drop legacy-contract-a >/dev/null
"$GANG" drop legacy-contract-b >/dev/null

# PORCELAIN IS EXACT TSV, NOT THE HUMAN GLYPH TABLE. First spend the exact-row
# assertion against the default roster and require it to fail; only then use it
# as the instrument for the scripting output.
cat > "$RUN_ROOT/collars/porcelain.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_BUSY_REGEX='MARK_PORCELAIN_BUSY'
SH
"$HITCH" porcelain-busy -c porcelain -d /tmp >/dev/null
"$HITCH" porcelain-idle -c porcelain -d /tmp >/dev/null
porcelain_busy_id="$(window_id porcelain-busy)"
porcelain_painted="porcelain-painted-$$"
tmux send-keys -l -t "$porcelain_busy_id" \
  "printf MARK_PORCELAIN_BUSY\\n; tmux wait-for -S $porcelain_painted"
tmux send-keys -t "$porcelain_busy_id" Enter
tmux wait-for "$porcelain_painted"
tmux set-option -w -t "$porcelain_busy_id" @gl_session_id sid-porcelain
porcelain_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$porcelain_busy_id" @gl_spool)"
mkdir -p "$porcelain_spool"
printf '%s\n%s\n%s\n' tester MARK_PORCELAIN_QUEUE \
  '[gang:tester#abcd1234] MARK_PORCELAIN_QUEUE [/gang:tester#abcd1234]' \
  > "$porcelain_spool/00000000100000000000-abcd1234"
mkdir -p "$RUN_ROOT/porcelain-bin"
cat > "$RUN_ROOT/porcelain-bin/date" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
case "${1:-}" in
  +%s) printf '105\n' ;;
  *) exec /usr/bin/date "$@" ;;
esac
SH
chmod +x "$RUN_ROOT/porcelain-bin/date"
expected_porcelain="$(printf \
  'porcelain-busy\tporcelain\tbusy\t1\t5\tsid-porcelain\nporcelain-idle\tporcelain\tidle\t0\t-\tUNSTAMPED')"
default_porcelain_probe="$(PATH="$RUN_ROOT/porcelain-bin:$PATH" \
  GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^porcelain-')"
if [ "$default_porcelain_probe" = "$expected_porcelain" ]; then
  fail "the exact TSV instrument rejects the decorated human roster" \
    "the human roster unexpectedly matched porcelain bytes"
else
  pass "the exact TSV instrument rejects the decorated human roster"
fi
actual_porcelain="$(PATH="$RUN_ROOT/porcelain-bin:$PATH" \
  GANG_ACTIVITY_WINDOW=0 "$GANG" roster --porcelain | grep '^porcelain-')"
equal "porcelain roster prints the exact six-column rows" \
  "$expected_porcelain" "$actual_porcelain"
equal "porcelain names contain no window-state glyph bytes" \
  $'porcelain-busy\nporcelain-idle' \
  "$(printf '%s\n' "$actual_porcelain" | cut -f1)"
equal "porcelain roster is empty when its session is absent" "" \
  "$(GANG_SESSION="porcelain-absent-$$" "$GANG" roster --porcelain)"
"$GANG" drop porcelain-busy >/dev/null
"$GANG" drop porcelain-idle >/dev/null

# QUEUE AGE COMES FROM THE OLDEST LIVE ENTRY'S FIXED-WIDTH STAMP. The chosen
# time is immediate input, not a wall-clock wait; prefix matching tolerates the
# one second in which the command itself runs.
"$HITCH" agebox -c spoolable -d /tmp >/dev/null
agebox_id="$(window_id agebox)"
agebox_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$agebox_id" @gl_spool)"
mkdir -p "$agebox_spool"
age_now="$(date +%s)"
one_hour_stamp="$(printf '%011d%09d' "$(( age_now - 3600 ))" 0)"
printf '%s\n%s\n%s\n' tester MARK_AGE_ONE_HOUR \
  '[gang:tester#aaaabbbb] MARK_AGE_ONE_HOUR [/gang:tester#aaaabbbb]' \
  > "$agebox_spool/$one_hour_stamp-aaaabbbb"
age_roster_out="$("$GANG" roster 2> "$RUN_ROOT/age-roster.err")"
contains "roster reports how long the oldest message has waited" \
  "$age_roster_out" "oldest=1h"
equal "roster parses a real padded stamp without stderr noise" "" \
  "$(<"$RUN_ROOT/age-roster.err")"
contains "status reports the same oldest-message age" \
  "$("$GANG" status agebox)" "the oldest has waited 1h"
two_hour_stamp="$(printf '%011d%09d' "$(( age_now - 7200 ))" 0)"
printf '%s\n%s\n%s\n' other MARK_AGE_TWO_HOURS \
  '[gang:other#ccccdddd] MARK_AGE_TWO_HOURS [/gang:other#ccccdddd]' \
  > "$agebox_spool/$two_hour_stamp-ccccdddd"
contains "queue age follows the older of two waiting entries" \
  "$("$GANG" roster)" "oldest=2h"
"$GANG" drop agebox >/dev/null

"$HITCH" agebad -c spoolable -d /tmp >/dev/null
agebad_id="$(window_id agebad)"
agebad_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$agebad_id" @gl_spool)"
mkdir -p "$agebad_spool"
printf '%s\n%s\n%s\n' tester MARK_BAD_STAMP \
  '[gang:tester#eeeeffff] MARK_BAD_STAMP [/gang:tester#eeeeffff]' \
  > "$agebad_spool/12345-abc"
agebad_row="$("$GANG" roster | grep '^agebad')"
contains "a malformed oldest stamp still reports known queue depth" \
  "$agebad_row" "spooled=1"
excludes "a malformed oldest stamp does not fabricate an age" \
  "$agebad_row" "oldest="
"$GANG" drop agebad >/dev/null

# PREEMPTION CARRIES ITS REASON THROUGH THE BOUNDARY IT CREATES. The fixture
# witnesses the collar-declared key independently and keeps a normal spool so
# backlog can prove it neither competes with nor absorbs the reason.
cat > "$RUN_ROOT/preempt-rc" <<RC
PS1='❯ '
bind -x '"\C-g": printf "%s\n" INTERRUPT_KEY_RECEIVED > "$RUN_ROOT/preempt-key"'
RC
cat > "$RUN_ROOT/collars/preemptible.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'ENV=$RUN_ROOT/preempt-rc exec bash --posix' fixture"
GANG_INTERRUPT_KEY='C-g'
GANG_STOP_HOOK=1
SH
"$HITCH" preempt -c preemptible -d /tmp >/dev/null
preempt_id="$(window_id preempt)"
tmux send-keys -l -t "$preempt_id" 'HUMAN_DRAFT'
printf 'MARK_PARKED_ONE' |
  "$GANG" send --to preempt --from tester --stdin >/dev/null
printf 'MARK_PARKED_TWO' |
  "$GANG" send --to preempt --from other --stdin >/dev/null
printf 'MARK_PARKED_THREE' |
  "$GANG" send --to preempt --from third --stdin >/dev/null
contains "the preemption world starts with three messages parked" \
  "$("$GANG" status preempt)" "spooled: 3"
tmux send-keys -t "$preempt_id" C-u
preempt_out="$("$GANG" interrupt preempt -m 'MARK_PREEMPT' --from tester)"
contains "a reasoned interrupt still reports the collar key" \
  "$preempt_out" "C-g"
[ -f "$RUN_ROOT/preempt-key" ] \
  && pass "the reasoned interrupt sends the collar-declared key" \
  || fail "the reasoned interrupt sends the collar-declared key" \
    "$RUN_ROOT/preempt-key is absent"
preempt_pane="$(pane preempt)"
contains "the reason reaches the boundary created by the interrupt" \
  "$preempt_pane" "MARK_PREEMPT"
contains "the interrupt reason carries sender attribution" \
  "$preempt_pane" "[gang:tester#"
contains "the reason never joins or consumes the existing queue" \
  "$("$GANG" status preempt)" "spooled: 3"
excludes "the reason lands before any parked backlog" \
  "$preempt_pane" "MARK_PARKED_ONE"
excludes "the preemption does not drain another sender's backlog" \
  "$preempt_pane" "MARK_PARKED_TWO"

# A STOP WITH A REASON SELF-TARGETS TOO. Bare `gang interrupt` already stops
# the calling window's own turn; the reason is the note that turn's author
# leaves for the next one, and it used to be the single self target that could
# not be spelled — the leading -m was read as an agent name and answered with a
# synopsis. The delivery has to be witnessed on the pane rather than in gang's
# own report, because a parser that resolved self and a self-send guard that
# refused afterwards both exit with the reason still in hand.
rm -f "$RUN_ROOT/preempt-key"
preempt_tmux_pane="$(tmux list-panes -t "$preempt_id" -F '#{pane_id}')"
self_reason_out="$(TMUX_PANE="$preempt_tmux_pane" \
  "$GANG" interrupt -m 'MARK_SELF_REASON' 2>&1)" || self_reason_out="REFUSED: $self_reason_out"
contains "a leading -m stops the calling window's own turn" \
  "$self_reason_out" "interrupted preempt with C-g"
[ -f "$RUN_ROOT/preempt-key" ] \
  && pass "the self-targeted stop sends the collar-declared key" \
  || fail "the self-targeted stop sends the collar-declared key" \
    "$RUN_ROOT/preempt-key is absent"
self_reason_pane="$(pane preempt)"
# source-guard: producer@1784c6b08bf6: the self-targeted interrupt above is the sole producer of MARK_SELF_REASON; no other sender, spool entry or fixture writes that literal
contains "the self-targeted reason reaches the boundary it created" \
  "$self_reason_pane" "MARK_SELF_REASON"
# source-guard: producer@ae27904d4c02: only a body whose sender is preempt itself carries this attribution, and the self-targeted reason is the one such body on this pane — every other envelope here is from tester, other or third
contains "and carries the calling agent as its author" \
  "$self_reason_pane" "[gang:preempt#"
contains "a self-targeted reason is never parked either" \
  "$("$GANG" status preempt)" "spooled: 3"

tmux send-keys -l -t "$preempt_id" 'HUMAN_DRAFT'
if preempt_refused="$("$GANG" interrupt preempt \
  -m 'MARK_UNDELIVERED' --from tester 2>&1)"; then
  fail "a reasoned interrupt refuses an occupied draft after stopping" \
    "interrupt unexpectedly succeeded"
else
  pass "a reasoned interrupt refuses an occupied draft after stopping"
fi
contains "an undelivered interrupt reason is handed back in full" \
  "$preempt_refused" "MARK_UNDELIVERED"
contains "a refused interrupt reason is never added to the queue" \
  "$("$GANG" status preempt)" "spooled: 3"
refuses "interrupt rejects an empty reason" \
  "interrupt: -m needs a non-empty message" \
  "$GANG" interrupt preempt -m '' --from tester
refuses "interrupt accepts only one reason" \
  "interrupt: -m may be passed only once" \
  "$GANG" interrupt preempt -m one -m two --from tester
refuses "interrupt rejects a sender when there is no message" \
  "--from names the author of a message" \
  "$GANG" interrupt preempt --from tester
contains "interrupt help names its reason option" \
  "$("$GANG" interrupt --help)" '-m "reason"'
tmux send-keys -t "$preempt_id" C-u
"$GANG" drop preempt >/dev/null

# A window with no spool identity is refused rather than given one here. Minting
# at the moment a message needs parking is exactly the race the identity exists
# to avoid, so gang says so instead of narrowing the window.
"$HITCH" identityless -c spoolable -d /tmp >/dev/null
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
"$GANG" adopt taken -c spoolable >/dev/null
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
cat > "$RUN_ROOT/collars/unverifiable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_STOP_HOOK=1
collar_input() { printf ''; }
SH
if "$GANG" hitch unverified -c unverifiable -d /tmp >/dev/null 2>&1; then
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
cat > "$RUN_ROOT/collars/wedging.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_STOP_HOOK=1
_gl_wedge_real="\$(declare -f collar_input)"
eval "wedge_real_input \${_gl_wedge_real#collar_input}"
collar_input() { # the real box, then a draft that refuses, then one that never changes
  if [ -f "$RUN_ROOT/wedge-block" ]; then printf 'BLOCKING_DRAFT'; return 0; fi
  if [ -f "$RUN_ROOT/wedge-stuck" ]; then printf ''; return 0; fi
  wedge_real_input "\$1"
}
SH
"$HITCH" wedged -c wedging -d /tmp >/dev/null
: > "$RUN_ROOT/wedge-block"
wedged_id="$(window_id wedged)"
wedged_pane_id="$(tmux list-panes -t "$wedged_id" -F '#{pane_id}')"
printf 'MARK_WEDGED' | "$GANG" send --to wedged --from tester --stdin >/dev/null
printf 'MARK_HELD_TWO' | "$GANG" send --to wedged --from other --stdin >/dev/null
printf 'MARK_HELD_THREE' | "$GANG" send --to wedged --from third --stdin >/dev/null
contains "the blocked messages are waiting" "$("$GANG" status wedged)" "spooled: 3"
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
  "$("$GANG" status wedged)" "spooled: 3"
tmux send-keys -t "$wedged_id" C-u
tmux wait-for "gang-spool-drain-$wedged_id" &
wedged_drain_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$wedged_pane_id" "$GANG" hook >/dev/null
wait "$wedged_drain_waiter"
wedged_status="$("$GANG" status wedged)"
contains "an unverified drain is reported, not swallowed" \
  "$wedged_status" "spool drain NOT verified"
contains "roster carries that verdict too" "$("$GANG" roster)" "spool-held=3"
excludes "and the entry is not left where it would be sent a second time" \
  "$wedged_status" "spooled:"
wedged_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$wedged_id" @gl_spool)"
wedged_quarantined=0
for wedged_entry in "$wedged_spool"/failed-*; do
  [ -f "$wedged_entry" ] && wedged_quarantined=$((wedged_quarantined + 1))
done
equal "every body in an unverified bundle is kept where a person can read it" "3" \
  "$wedged_quarantined"
# READ IT. A file of the right name is not a kept message: the promise gang
# makes when it holds a body instead of re-sending it is that the body is still
# there, and only reading it says so.
wedged_body="$(cat "$wedged_spool"/failed-* 2>/dev/null)" || wedged_body=""
contains "the body itself, not just a file with the right name" \
  "$wedged_body" "MARK_WEDGED"
contains "the second body is named as held" "$wedged_status" "MARK_HELD_TWO"
contains "the third body is named as held" "$wedged_status" "MARK_HELD_THREE"
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
"$HITCH" lingering -c spoolable -d /tmp >/dev/null
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
cat > "$RUN_ROOT/collars/drain.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_drain_real="\$(declare -f collar_input)"
eval "drain_real_input \${_gl_drain_real#collar_input}"
collar_input() { # a parked reading per ticket, then the real drained box
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
"$HITCH" drain -c drain -d /tmp >/dev/null
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
# sentinel after Enter and its collar_input grants exactly one readable look
# at that sentinel: the first reading breaks the change loop as a normal
# non-queue submission would, and the late queue-evidence reread finds the
# box unreadable. Falling through to success here is the hole; the send must
# die naming the uncertainty and record the body as unknown.
cat > "$RUN_ROOT/flicker-rc" <<'RC'
PS1='❯ '
PROMPT_COMMAND='[ -f "$FLICKER_FLAG" ] && PS1="❯ POST_SENTINEL"'
RC
cat > "$RUN_ROOT/collars/flicker.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'FLICKER_FLAG=$RUN_ROOT/flicker-flag ENV=$RUN_ROOT/flicker-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*QUEUE_HINT_NEVER_SHOWN\$'
collar_input() { # one readable look at the post-Enter sentinel, then nothing
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
"$HITCH" flicker -c flicker -d /tmp >/dev/null
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
cat > "$RUN_ROOT/collars/vanish.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_vanish_real="\$(declare -f collar_input)"
eval "vanish_real_input \${_gl_vanish_real#collar_input}"
collar_input() { # readable per ticket once the vanish flag is set, then not
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
"$HITCH" vanish -c vanish -d /tmp >/dev/null
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
# the two is a harness in trouble. This collar serves the draft for exactly the
# reads a refusal takes to settle and hands the naming look an empty box after
# it; a change in that count fails this check rather than quietly retargeting
# it, because the class named is asserted exactly.
cat > "$RUN_ROOT/collars/emptied.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_emptied_real="\$(declare -f collar_input)"
eval "emptied_real_input \${_gl_emptied_real#collar_input}"
collar_input() { # the draft per ticket once the flag is set, then an empty box
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
"$HITCH" emptied -c emptied -d /tmp >/dev/null
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
cat > "$RUN_ROOT/collars/fossil.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_BUSY_REGEX='Retrying in [0-9]+s'
GANG_QUIET_AT_REST=1
SH
"$HITCH" fossil -c fossil -d /tmp >/dev/null
tmux send-keys -l -t "$(window_id fossil)" \
  'echo "Retrying in 8s left by an interrupted loop"'
tmux send-keys -t "$(window_id fossil)" Enter
fossil_bracket="open $(( $(date +%s) - 400 ))"
tmux set-option -w -t "$(window_id fossil)" @gl_turn "$fossil_bracket"
# Positive control: recent pty activity preserves painted busy — the
# demotion must not fire while the pty leg credits the fresh paint.
fossil_active="$(GANG_ACTIVITY_WINDOW=100000 "$GANG" status fossil | head -1)"
equal "recent pty activity keeps the busy verdict itself, not its explanation" \
  "-busy-" "$fossil_active"
if printf 'MARK_ACTIVE' | GANG_ACTIVITY_WINDOW=100000 \
  "$GANG" send --to fossil --from tester --stdin >/dev/null 2>&1; then
  fail "recent pty activity keeps refusing delivery mid-turn" "send succeeded"
else
  pass "recent pty activity keeps refusing delivery mid-turn"
fi
excludes "the refused active-pane body never landed" \
  "$(pane fossil)" "MARK_ACTIVE"
contains "roster's immediate snapshot keeps the painted verdict" \
  "$("$GANG" roster)" "-busy-"
# The fossil verdict: no recent write, byte-stable pane, expired bracket.
fossil_status="$(GANG_ACTIVITY_WINDOW=0 "$GANG" status fossil)"
contains "frozen busy paint over an expired bracket reads expired, not busy" \
  "$fossil_status" "?unknown?"
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
cat > "$RUN_ROOT/collars/abandoned.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_BUSY_REGEX='Retrying in [0-9]+s'
GANG_QUIET_AT_REST=1
SH
"$HITCH" abandoned -c abandoned -d /tmp >/dev/null
abandoned_bracket="open $(( $(date +%s) - 400 ))"
tmux set-option -w -t "$(window_id abandoned)" @gl_turn "$abandoned_bracket"
# The guard the decay must not stomp, asserted first: an UNEXPIRED bracket is
# fresh owned state and outranks every quiet tier under it. Nothing about a
# still-bounded turn changes, however ready the pane looks.
equal "an unexpired bracket over the same quiet box is still a live turn" \
  "-busy-" \
  "$(GANG_TURN_LIMIT=100000 GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | head -1)"
equal "an abandoned turn decays to idle once its bracket expires" \
  "~idle~" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | head -1)"
contains "roster's snapshot decays with it — reading the box costs no churn wait" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^abandoned ')" "~idle~"
# Not a free pass over anything the box refutes: a draft sitting in the composer
# is the state the busy verdict exists to protect, and it keeps the answer
# could-not-determine on both readings.
tmux send-keys -l -t "$(window_id abandoned)" 'half a thought'
equal "a drafted box refuses the decay" \
  "?unknown? (turn-bracket bound reached)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | head -1)"
contains "roster's snapshot refuses it on the same evidence" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^abandoned ')" "?unknown?"
tmux send-keys -t "$(window_id abandoned)" C-u
equal "clearing the draft restores the decayed verdict" \
  "~idle~" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status abandoned | head -1)"
# Quiet must be MEASURED, never assumed. Hold the activity-only bound open past
# its limit with the pty credited as recent: the activity tier then reports
# could-not-determine, and an unknown tier cannot witness readiness.
tmux set-option -w -t "$(window_id abandoned)" @gl_activity_only_since \
  "$(( $(date +%s) - 400 ))"
equal "an unmeasurable pty keeps the answer could-not-determine" \
  "?unknown? (turn-bracket bound reached)" \
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

# The quiet leg must be measured, and a collar that does not declare
# quiet-at-rest measures nothing: its harness writes to the pty constantly at
# rest, so the activity tier reports inactive by abstention rather than by
# observation. Spending that as the positive evidence a decay requires would
# decay every abandoned turn on a harness gang cannot hear.
"$HITCH" assumed -c bash -d /tmp >/dev/null
tmux set-option -w -t "$(window_id assumed)" @gl_turn "open $(( $(date +%s) - 400 ))"
equal "a collar that never measures the pty cannot witness the quiet a decay needs" \
  "?unknown? (turn-bracket bound reached)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status assumed | head -1)"
"$GANG" drop assumed >/dev/null

# Each tier read is a separate moment, and a decay assembled across them can be
# a state no single instant held: the harness resuming between the quiet
# reading and the box reading would have its still-open turn discarded on
# evidence that had already gone stale. This collar writes to its own pty
# whenever gang reads its box — the harness moving underneath the decision,
# made deterministic — and the decay must refuse rather than report idle.
{ printf '. %s\nMOVING_ON=%s\n' "$ROOT/collars/bash.sh" "$RUN_ROOT/moving.on"; cat <<'SH'
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_QUIET_AT_REST=1
eval "$(declare -f collar_input | sed '1s/collar_input/moving_read/')"
collar_input() { # $1 = tmux target; reads the box, then writes below it
  local out rc=0
  out="$(moving_read "$1")" || rc=$?
  [ ! -e "$MOVING_ON" ] \
    || printf '\ntick\n' > "$(tmux display-message -p -t "$1" '#{pane_tty}')" 2>/dev/null
  printf '%s' "$out"
  return $rc
}
SH
} > "$RUN_ROOT/collars/moving.sh"
"$HITCH" moving -c moving -d /tmp >/dev/null
tmux set-option -w -t "$(window_id moving)" @gl_turn "open $(( $(date +%s) - 400 ))"
: > "$RUN_ROOT/moving.on"   # the harness starts moving only now that it is up
equal "a pane that moves while gang is deciding refuses the decay" \
  "?unknown? (the pane was written to while gang was deciding)" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" status moving | head -1)"
contains "and roster's snapshot refuses it on the same witness" \
  "$(GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^moving ')" "?unknown?"
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
"$HITCH" seam -c abandoned -d /tmp >/dev/null
tmux set-option -w -t "$(window_id seam)" @gl_turn "open $(( $(date +%s) - 400 ))"
equal "a write between the quiet reading and the witness refuses the decay" \
  "?unknown? (the pane was written to while gang was deciding)" \
  "$(PATH="$RUN_ROOT/bin-seam:$PATH" GANG_ACTIVITY_WINDOW=0 "$GANG" status seam | head -1)"
"$GANG" drop seam >/dev/null

# The delivery half of the same seam. Refusing the decay leaves
# could-not-determine, and send's fall-through delivers into a provably empty
# box on that verdict — correct when the verdict means stale evidence, wrong
# here, where it means gang WATCHED the screen being written to while it
# decided. A harness paints the opening of a turn with its composer still
# empty, so an empty box read out of a moving screen proves nothing and the
# paste lands in live work. The collar's busy marker is what the shim paints,
# so this is a turn starting mid-decision and not merely noise.
cat > "$RUN_ROOT/collars/seamsend.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_BUSY_REGEX='tick-after-quiet-read'
GANG_QUIET_AT_REST=1
SH
"$HITCH" seamsend -c seamsend -d /tmp >/dev/null
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

# A collar that declares no input reader has no box for gang to measure:
# landing_zone falls back to the whole pane, which is never empty, so the
# expired-witness refusal fires on a transcript rather than on a composer.
# Calling that "its input box" blames a draft nobody wrote — the class says
# what gang actually read.
cat > "$RUN_ROOT/collars/noreader.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
unset -f collar_input
SH
"$HITCH" noreader -c noreader -d /tmp >/dev/null
tmux set-option -w -t "$(window_id noreader)" @gl_turn \
  "open $(( $(date +%s) - 400 ))"
if noreader_out="$(printf 'MARK_NOREADER' | GANG_ACTIVITY_WINDOW=0 \
  "$GANG" send --to noreader --from tester --stdin 2>&1)"; then
  fail "an expired witness over a collar with no input reader refuses" \
    "send succeeded"
else
  pass "an expired witness over a collar with no input reader refuses"
fi
contains "and names the whole pane it actually measured" \
  "$noreader_out" "whole-pane:"
excludes "the refused body never landed" "$(pane noreader)" "MARK_NOREADER"
"$GANG" drop noreader >/dev/null

# Positive control for the stability leg: a churning pane preserves painted
# busy even with the activity credit forced off. The verdict is asserted
# EXACTLY: the broken state reads "?unknown? (busy paint frozen…)", whose
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
cat > "$RUN_ROOT/collars/ticker.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'i=0; while :; do i=\\\$((i + 1)); echo Retrying in 9s tick \\\$i; echo \"❯ \"; tmux wait-for -S \"gltick-\\\$GANG_SESSION\"; done' fixture"
GANG_BUSY_REGEX='Retrying in [0-9]+s'
GANG_QUIET_AT_REST=1
SH
if GANG_BOOT_TIMEOUT=0 "$GANG" hitch ticker -c ticker -d /tmp >/dev/null 2>&1; then
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
  "-busy-" \
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
cat > "$RUN_ROOT/collars/blindbox.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
_gl_blind_real="\$(declare -f collar_input)"
eval "blind_real_input \${_gl_blind_real#collar_input}"
collar_input() {
  [ ! -f "$RUN_ROOT/blindbox-flag" ] || return 1
  blind_real_input "\$1"
}
SH
"$HITCH" blindbox -c blindbox -d /tmp >/dev/null
tmux set-option -w -t "$(window_id blindbox)" @gl_turn "open $(( $(date +%s) - 400 ))"
touch "$RUN_ROOT/blindbox-flag"
if blind_out="$(printf 'MARK_BLIND' | "$GANG" send --to blindbox --from tester --stdin 2>&1)"; then
  fail "an expired bracket over an unreadable box still refuses" "send succeeded"
else
  pass "an expired bracket over an unreadable box still refuses"
fi
contains "as occupancy of unknown authority" "$blind_out" "authority unknown"
"$GANG" drop blindbox >/dev/null

# A collar-provided native compaction command uses the same verified injection
# primitive. Record execution outside the pane so the typed command cannot
# satisfy its own guard before the shell runs it.
mkdir -p "$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/native.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD="printf NATIVE_COMPACT > $RUN_ROOT/native-compact-executed"
SH
export GANG_COLLARS="$RUN_ROOT/collars"
"$HITCH" compactable -c native -d /tmp >/dev/null
"$GANG" compact compactable >/dev/null
equal "compact executes the collar's native command" \
  "NATIVE_COMPACT" "$(cat "$RUN_ROOT/native-compact-executed")"
# A COLLAR THAT DECLARES NO SLOT GETS NOTHING APPENDED. Instructions are typed
# at the harness's own summariser, so a harness that has not been driven to
# accept them must not have text pushed at it on the assumption that it does.
excludes "and appends no instructions to a collar that declares no slot" \
  "$(pane compactable)" "still outstanding in your lane"

# WHERE THE SLOT IS DECLARED, THE INSTRUCTIONS ARE WHAT GETS TYPED. A summary
# chosen without instruction keeps what reads as important, which is not what a
# lane needs to continue.
cat > "$RUN_ROOT/collars/native-slot.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD="printf SLOTTED_{{instructions}}"
SH
"$HITCH" slotted -c native-slot -d /tmp >/dev/null
"$GANG" compact slotted >/dev/null
slotted_pane="$(pane slotted)"
contains "a declared slot is filled with the continuation instructions" \
  "$slotted_pane" "still outstanding in your lane"
contains "which name the durable state a lane is resumed from" \
  "$slotted_pane" "SLOTTED_Keep the brief you were given, the durable state"
# Orientation, never direction: a default that told a finished agent to carry on
# would make it invent work, which is the worse failure of the two.
excludes "and never tell the agent to keep working" "$slotted_pane" "continue working"
excludes "the placeholder itself is never typed" "$slotted_pane" "{{instructions}}"
"$GANG" drop slotted >/dev/null 2>&1 || :

# A COMPACTION LANDS THE AGENT ON A TURN, NOT AN EMPTY COMPOSER. The continuation
# is typed behind the compaction command and carries Gangline's own attribution.
compactable_pane="$(pane compactable)"
contains "a continuation turn is typed behind the compaction command" \
  "$compactable_pane" "Re-read your brief and the durable state you wrote"
contains "attributed to Gangline itself, not to a peer" \
  "$compactable_pane" "[gang:gangline#"
# Orientation, never direction, for the same reason the keep-instructions are.
excludes "and the default continuation never tells the agent to keep working" \
  "$compactable_pane" "continue working"
"$GANG" compact compactable --resume "RESUME_MARKER_ONLY" >/dev/null
contains "--resume replaces the default continuation" \
  "$(pane compactable)" "RESUME_MARKER_ONLY"
compact_bad_rc=0
"$GANG" compact compactable --resume "   " >/dev/null 2>&1 || compact_bad_rc=$?
equal "a whitespace-only continuation is refused rather than typed" \
  "1" "$compact_bad_rc"
"$GANG" drop compactable >/dev/null 2>&1 || :

# THE CONTINUATION IS MEANT TO PARK, AND THE PARK IS THE LANDING. A harness that
# is compacting queues the turn typed behind the compaction command and submits it
# when the compaction ends. The composer reads clean while the compaction runs and
# carries the hint only once something is queued, which is what claude-code 2.1.227
# was driven doing; a fixture showing the hint earlier would refuse at the
# preflight and prove nothing. Same observed hint as the claude-code collar.
cat > "$RUN_ROOT/compact-queue-rc" <<'RC'
PS1='❯ '
PROMPT_COMMAND='if [ -f "$QUEUE_STRAND" ]; then
                  [ -z "$QUEUE_ARMED" ] || PS1="❯ Press up to edit queued messages"
                  QUEUE_ARMED=1
                fi'
RC
cat > "$RUN_ROOT/collars/compact-queueing.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'QUEUE_STRAND=$RUN_ROOT/compact-queue-strand ENV=$RUN_ROOT/compact-queue-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
GANG_COMPACT_CMD="touch $RUN_ROOT/compact-queue-strand"
SH
rm -f "$RUN_ROOT/compact-queue-strand"
"$HITCH" parking -c compact-queueing -d /tmp >/dev/null
if "$GANG" compact parking >/dev/null 2>&1; then
  pass "a continuation the harness parks is accepted, not reported as failed"
else
  fail "a continuation the harness parks is accepted, not reported as failed" \
    "compact reported the park as a failed delivery"
fi
# A DELIBERATE PARK LEAVES NO RECOVERY RECORD. flush exists to rescue a peer
# message the harness swallowed; this one drains itself when the compaction ends,
# and a record here would send an operator after a message already on its way.
equal "and records no parked message for flush to chase" "" \
  "$(tmux show-options -wqv -t "$(window_id parking)" @gl_parked)"
# The exception is scoped to the continuation and does not widen delivery. With
# the fixture's queue still showing, an ordinary peer message is refused on the
# same evidence gang has always refused it on, before anything is typed.
parking_rc=0
parking_out="$(printf 'MARK_PEER_PARKED' \
  | "$GANG" send --to parking --from tester --live-only --stdin 2>&1)" \
  || parking_rc=$?
equal "while an ordinary message into that same queue is still refused" \
  "3" "$parking_rc"
contains "on the parked-queue evidence, before anything is typed" \
  "$parking_out" "parked earlier input"
"$GANG" drop parking >/dev/null 2>&1 || :

# A self-request made inside an agent's own pane must not submit the native
# command during that turn. Stop consumes it once, after which a one-shot worker
# submits the collar command and exits. Both waits below are tmux event barriers,
# not clocks or polling loops.
self_executed="test-self-compact-executed-$$"
self_busy="$RUN_ROOT/self-compact-busy"
cat > "$RUN_ROOT/collars/deferred.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD="printf SELF_COMPACT; tmux wait-for -S $self_executed"
GANG_SELF_COMPACT=deferred
GANG_BUSY_REGEX='BUSY_DEFERRED'
_gl_self_input="\$(declare -f collar_input)"
eval "self_real_input \${_gl_self_input#collar_input}"
collar_input() {
  [ ! -e "$self_busy" ] || { printf ''; return; }
  self_real_input "\$1"
}
SH
"$HITCH" selfable -c deferred -d /tmp >/dev/null
self_id="$(window_id selfable)"
self_tmux_pane="$(tmux list-panes -t "$self_id" -F '#{pane_id}')"
self_requested="test-self-compact-requested-$$"
self_release="test-self-compact-release-$$"
self_released="test-self-compact-released-$$"
printf -v self_command ': > %q; printf BUSY_DEFERRED; GANG_SESSION=%q GANG_COLLARS=%q %q compact; tmux wait-for -S %q; tmux wait-for %q; rm -f -- %q; tmux wait-for -S %q' \
  "$self_busy" "$GANG_SESSION" "$GANG_COLLARS" "$GANG" "$self_requested" \
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
cat > "$RUN_ROOT/collars/nodeferred.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD="/compact"
GANG_BUSY_REGEX='BUSY_NODEFERRED_COMPACT'
_gl_nodeferred_input="\$(declare -f collar_input)"
eval "nodeferred_real_input \${_gl_nodeferred_input#collar_input}"
collar_input() {
  [ ! -e "$nodeferred_busy" ] || { printf ''; return; }
  nodeferred_real_input "\$1"
}
SH
"$HITCH" nodeferred -c nodeferred -d /tmp >/dev/null
nodeferred_id="$(window_id nodeferred)"
nodeferred_observed="test-nodeferred-compact-observed-$$"
nodeferred_release="test-nodeferred-compact-release-$$"
printf -v nodeferred_command ': > %q; printf BUSY_NODEFERRED_COMPACT; GANG_SESSION=%q GANG_COLLARS=%q %q compact nodeferred >/dev/null 2>&1 || :; tmux wait-for -S %q; tmux wait-for %q; rm -f -- %q' \
  "$nodeferred_busy" "$GANG_SESSION" "$GANG_COLLARS" "$GANG" \
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

# Stall lights forward only native awaiting-input witnesses to one optional
# declared target. Every outcome is synchronous: the hook returns after the
# note is accepted live, parked, or recorded as failed.
cat > "$RUN_ROOT/collars/stallable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
GANG_STALL_TYPES="idle_prompt agent_needs_input"
SH
equal "a team starts without an inferred notify target" \
  "no notify target declared" "$("$GANG" notify)"
"$HITCH" stall-raise -c stallable -d /tmp >/dev/null
"$HITCH" stall-target -c stallable -d /tmp >/dev/null
stall_raise_id="$(window_id stall-raise)"
stall_raise_pane="$(tmux list-panes -t "$stall_raise_id" -F '#{pane_id}')"
stall_target_id="$(window_id stall-target)"
stall_target_pane="$(tmux list-panes -t "$stall_target_id" -F '#{pane_id}')"
equal "a notify target may be declared without inference" \
  "notify target: stall-target" "$("$GANG" notify stall-target)"
equal "the notify declaration is readable" \
  "notify target: stall-target" "$("$GANG" notify)"

# A collar with no declared stall kinds has no Notification witness. In
# particular, an absent notification_type must not match an empty declaration.
cat > "$RUN_ROOT/collars/no-stalls.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
SH
"$HITCH" stall-silent -c no-stalls -d /tmp >/dev/null
stall_silent_id="$(window_id stall-silent)"
stall_silent_pane="$(tmux list-panes -t "$stall_silent_id" -F '#{pane_id}')"
stall_silent_before="$(pane_all stall-target)"
printf '%s' '{"hook_event_name":"Notification"}' |
  TMUX_PANE="$stall_silent_pane" "$GANG" hook >/dev/null
equal "a collar without a declared stall source raises no empty-kind note" \
  "$stall_silent_before" "$(pane_all stall-target)"
"$GANG" drop stall-silent >/dev/null

# Stop owns the universal turn boundary even when the window's old collar no
# longer resolves. Collar loading belongs only to collar-dependent work.
"$HITCH" stall-vanished -c stallable -d /tmp >/dev/null
stall_vanished_id="$(window_id stall-vanished)"
stall_vanished_pane="$(tmux list-panes -t "$stall_vanished_id" -F '#{pane_id}')"
tmux set-option -w -t "$stall_vanished_id" @gl_turn open
tmux set-option -w -t "$stall_vanished_id" @gl_collar vanished
if vanished_stop="$(printf '%s' '{"hook_event_name":"Stop"}' |
    TMUX_PANE="$stall_vanished_pane" "$GANG" hook 2>&1)"; then
  pass "a native Stop closes the turn after its collar vanishes"
else
  fail "a native Stop closes the turn after its collar vanishes" \
    "hook failed before the universal boundary: [$vanished_stop]"
fi
contains "the collar-independent Stop boundary is recorded" \
  "$(tmux show-options -wqv -t "$stall_vanished_id" @gl_turn)" "closed"
"$GANG" drop stall-vanished >/dev/null

printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_first="$(pane_all stall-target)"
contains "a native awaiting-input witness reaches the declared target" \
  "$stall_first" "stall: stall-raise is awaiting input (idle_prompt)"
contains "the stall note keeps the raising window's attribution" \
  "$stall_first" "[gang:stall-raise#"
stall_first_count="$(printf '%s\n' "$stall_first" | grep -oF 'awaiting input (idle_prompt)' | wc -l | tr -d ' ')"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_repeat_count="$(pane_all stall-target | grep -oF 'awaiting input (idle_prompt)' | wc -l | tr -d ' ')"
equal "the same native stall inside the debounce is one note" \
  "$stall_first_count" "$stall_repeat_count"

printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_after_move_count="$(pane_all stall-target | grep -oF 'awaiting input (idle_prompt)' | wc -l | tr -d ' ')"
equal "a native movement event opens a new stall epoch" \
  "$(( stall_first_count + 1 ))" "$stall_after_move_count"

stall_old_now="$(date +%s)"
tmux set-option -w -t "$stall_raise_id" @gl_stall \
  "idle_prompt $(( stall_old_now - 601 ))"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_old_count="$(pane_all stall-target | grep -oF 'awaiting input (idle_prompt)' | wc -l | tr -d ' ')"
equal "an old debounce stamp permits another native note" \
  "$(( stall_after_move_count + 1 ))" "$stall_old_count"

printf '%s' '{"hook_event_name":"Notification","notification_type":"auth_success"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
equal "a notification kind outside the collar declaration raises nothing" \
  "$stall_old_count" "$(pane_all stall-target | grep -oF 'awaiting input (idle_prompt)' | wc -l | tr -d ' ')"

printf '%s' '{"hook_event_name":"PermissionRequest"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "a native permission request is an awaiting-input witness" \
  "$(pane_all stall-target)" "awaiting input (permission_prompt)"

"$GANG" notify clear >/dev/null
stall_cleared_before="$(pane_all stall-target)"
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
equal "no declaration is the silent off state" \
  "$stall_cleared_before" "$(pane_all stall-target)"
equal "clear removes the session declaration" \
  "no notify target declared" "$("$GANG" notify)"

"$GANG" notify stall-raise >/dev/null
stall_self_before="$(pane_all stall-raise)"
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
equal "a stall note is never sent into the raising pane" \
  "$stall_self_before" "$(pane_all stall-raise)"
equal "self-target suppression records no delivery failure" "" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)"

# An existing tmux window that has not been adopted reaches the attempted
# delivery region and fails there. That failure must not stamp the debounce:
# adopting the same window makes an immediate retry observable.
tmux new-window -d -t "=$GANG_SESSION" -n stall-unadopted -c /tmp \
  "PS1='❯ ' bash --norc"
"$GANG" notify stall-unadopted >/dev/null
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_unaccepted_before="$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall)"
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "an unadopted notify target fails inside the delivery attempt" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)" \
  "exists but is not a gang agent"
equal "an unaccepted note leaves the debounce stamp unchanged" \
  "$stall_unaccepted_before" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall)"
"$GANG" adopt stall-unadopted -c stallable >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "adoption lets the unaccepted note retry without advancing time" \
  "$(pane_all stall-unadopted)" "awaiting input (agent_needs_input)"
equal "the accepted retry retires the attempted-delivery failure" "" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)"
"$GANG" drop stall-unadopted >/dev/null

"$GANG" notify ghost >/dev/null
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "a missing notify target is visible in status" \
  "$("$GANG" status stall-raise)" "stall note NOT accepted"
contains "roster carries a failed stall light" \
  "$("$GANG" roster)" "stall-note-failed"
equal "a missing target writes no debounce stamp" "" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall)"

"$HITCH" ghost -c stallable -d /tmp >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "a failed note retries once its target exists" \
  "$(pane_all ghost)" "awaiting input (agent_needs_input)"
equal "an accepted repair retires the delivery failure" "" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)"
excludes "status no longer reports the repaired stall light" \
  "$("$GANG" status stall-raise)" "stall note NOT accepted"

"$GANG" drop ghost >/dev/null
"$GANG" notify ghost >/dev/null
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_failure_before="$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
equal "movement does not retire a still-broken stall light" \
  "$stall_failure_before" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)"

"$GANG" notify stall-target >/dev/null
tmux send-keys -l -t "$stall_target_id" 'HUMAN_DRAFT'
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "a refused stall note is accepted into a drainable target's spool" \
  "$("$GANG" status stall-target)" "spooled: 1"
contains "parking a stall note records the debounce" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall)" "agent_needs_input"
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "a parked stall note is debounced without a duplicate" \
  "$("$GANG" status stall-target)" "spooled: 1"
tmux send-keys -t "$stall_target_id" C-u
tmux wait-for "gang-spool-drain-$stall_target_id" &
stall_target_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$stall_target_pane" "$GANG" hook >/dev/null
wait "$stall_target_waiter"
contains "the parked stall note drains through the ordinary delivery path" \
  "$(pane_all stall-target)" "awaiting input (agent_needs_input)"

# THE DEBOUNCE IS A CRITICAL SECTION, NOT A PAIR OF READS. Two native witnesses
# of one kind for one window can both read the stale stamp and both deliver, so
# the declared target gets the same envelope twice inside the interval the
# debounce exists to enforce. Every assertion above it is sequential and cannot
# see that. This one holds the first delivery open INSIDE the target's own
# collar_input on a tmux barrier — an event, not a clock — and fires the second
# while the first is provably still in flight.
stall_gate_inside="test-stall-gate-inside-$$"
stall_gate_release="test-stall-gate-release-$$"
cat > "$RUN_ROOT/collars/stall-gated.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
GANG_STALL_TYPES="idle_prompt agent_needs_input"
_gl_gate_real="\$(declare -f collar_input)"
eval "stall_gate_real_input \${_gl_gate_real#collar_input}"
collar_input() { # hold exactly one delivery open, once, on an armed barrier
  if [ -f "$RUN_ROOT/stall-gate-arm" ]; then
    rm -f "$RUN_ROOT/stall-gate-arm"
    tmux wait-for -S "$stall_gate_inside"
    tmux wait-for "$stall_gate_release"
  fi
  stall_gate_real_input "\$1"
}
SH
"$HITCH" stall-gated -c stall-gated -d /tmp >/dev/null
stall_gated_id="$(window_id stall-gated)"
"$GANG" notify stall-gated >/dev/null
tmux set-option -uw -t "$stall_raise_id" @gl_stall
tmux set-option -uw -t "$stall_raise_id" @gl_stall_failed
: > "$RUN_ROOT/stall-gate-arm"
( printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null 2>&1 ) &
stall_race_pid=$!
tmux wait-for "$stall_gate_inside"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null 2>&1 || true
tmux wait-for -S "$stall_gate_release"
wait "$stall_race_pid" || true
stall_gated_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$stall_gated_id" @gl_spool)"
stall_race_spooled=0
for stall_race_entry in "$stall_gated_spool"/[0-9]* \
    "$stall_gated_spool"/sending-* "$stall_gated_spool"/failed-*; do
  [ -f "$stall_race_entry" ] || continue
  stall_race_spooled=$((stall_race_spooled + 1))
done
stall_race_live="$(pane_all stall-gated |
  grep -cF 'awaiting input (idle_prompt)' || true)"
equal "concurrent native witnesses of one kind deliver exactly one note" \
  "1 0" "$stall_race_live $stall_race_spooled"
"$GANG" drop stall-gated >/dev/null
"$GANG" notify stall-target >/dev/null

if "$GANG" notify 'bad name' >/dev/null 2>&1; then
  fail "notify rejects a name outside the agent-name contract" \
    "invalid name was accepted"
else
  pass "notify rejects a name outside the agent-name contract"
fi
"$GANG" notify clear >/dev/null
"$GANG" drop stall-raise >/dev/null
"$GANG" drop stall-target >/dev/null

# Optional context guidance has two edge-triggered states and no clock path.
cat > "$RUN_ROOT/collars/lights.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_context() {
  tmux show-options -wqv -t "\$1" @test_context
}
SH
GANG_CONTEXT_LIGHTS=100000,200000 "$HITCH" lit -c lights -d /tmp >/dev/null
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

# ONE RELATIVE SPEC SERVES DIFFERENT NATIVE WINDOWS. The same percentages are
# copied at hitch; each hook resolves them against the window in its own native
# reading rather than spending one team's absolute thresholds on both harnesses.
for relative_case in small large zero; do
  GANG_CONTEXT_LIGHTS=50%,80% "$HITCH" "lit-$relative_case" \
    -c lights -d /tmp >/dev/null
done
lit_small_id="$(window_id lit-small)"
lit_small_pane="$(tmux list-panes -t "$lit_small_id" -F '#{pane_id}')"
tmux set-option -w -t "$lit_small_id" @test_context '130k/258k (50%)'
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$lit_small_pane" "$GANG" hook >/dev/null
small_yellow="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_small_pane" "$GANG" hook)"
contains "relative yellow resolves inside the smaller native window" \
  "$small_yellow" "Yellow context light"
tmux set-option -w -t "$lit_small_id" @test_context '210k/258k (81%)'
small_red="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_small_pane" "$GANG" hook)"
contains "relative red resolves inside the smaller native window" \
  "$small_red" "Red context light"

lit_large_id="$(window_id lit-large)"
lit_large_pane="$(tmux list-panes -t "$lit_large_id" -F '#{pane_id}')"
tmux set-option -w -t "$lit_large_id" @test_context '600k/1000k (60%)'
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$lit_large_pane" "$GANG" hook >/dev/null
large_yellow="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_large_pane" "$GANG" hook)"
contains "the same relative yellow resolves inside the larger native window" \
  "$large_yellow" "Yellow context light"
tmux set-option -w -t "$lit_large_id" @test_context '850k/1000k (85%)'
large_red="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_large_pane" "$GANG" hook)"
contains "the same relative red resolves inside the larger native window" \
  "$large_red" "Red context light"

lit_zero_id="$(window_id lit-zero)"
lit_zero_pane="$(tmux list-panes -t "$lit_zero_id" -F '#{pane_id}')"
tmux set-option -w -t "$lit_zero_id" @test_context '0k/0k (0%)'
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$lit_zero_pane" "$GANG" hook >/dev/null
zero_window="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_zero_pane" "$GANG" hook)"
contains "a zero native window is invalid rather than an immediate red light" \
  "$zero_window" "native context source reported a zero-token window"

GANG_CONTEXT_LIGHTS=350000,500000 "$HITCH" lit-impossible \
  -c lights -d /tmp >/dev/null
lit_impossible_id="$(window_id lit-impossible)"
lit_impossible_pane="$(tmux list-panes -t "$lit_impossible_id" -F '#{pane_id}')"
tmux set-option -w -t "$lit_impossible_id" @test_context '100k/258k (39%)'
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$lit_impossible_pane" "$GANG" hook >/dev/null
tmux set-option -w -t "$lit_impossible_id" @gl_context_light red
impossible="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_impossible_pane" "$GANG" hook)"
contains "an impossible absolute spec supersedes a stale red latch" \
  "$impossible" "red threshold 500000 cannot fire in this harness's 258000-token window"
impossible_repeat="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_impossible_pane" "$GANG" hook)"
equal "an impossible absolute spec reports once per invalid epoch" \
  "" "$impossible_repeat"

if mixed_light_out="$(GANG_CONTEXT_LIGHTS=50%,200000 "$GANG" hitch \
    lit-mixed-units -c lights -d /tmp 2>&1)"; then
  fail "context-light thresholds in mixed units are refused" \
    "hitch unexpectedly succeeded"
else
  contains "the mixed-unit refusal names the one-unit rule" \
    "$mixed_light_out" "must use the same unit for both thresholds"
fi
equal "the mixed-unit refusal opens no window" "" \
  "$(window_id lit-mixed-units)"

# A repository-local hooksPath shadows the operator's global path, so the
# tracked pre-push hook must delegate outward before it runs Gangline's gates.
# An ambient legacy depth marker must not suppress either delegation or local
# checks. Recursive chaining is an identity decision in Snubline's dispatcher.
delegation_root="$RUN_ROOT/pre-push-delegation"
delegation_hooks="$delegation_root/hooks"
delegation_config="$delegation_root/gitconfig"
delegation_record="$delegation_root/record"
delegation_input="$delegation_root/input"
mkdir -p "$delegation_hooks"
cat > "$delegation_hooks/pre-push" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cat > "$GANGLINE_OUTER_INPUT"
{
  printf 'argv:%s|%s\n' "$1" "$2"
  cat "$GANGLINE_OUTER_INPUT"
} > "$GANGLINE_OUTER_RECORD"
printf '%s\n' OUTER_STDOUT_MARKER
printf '%s\n' OUTER_STDERR_MARKER >&2
exit "${GANGLINE_OUTER_RC:-0}"
SH
chmod +x "$delegation_hooks/pre-push"
git config --file "$delegation_config" core.hooksPath "$delegation_hooks"
export GANGLINE_OUTER_INPUT="$delegation_input"
export GANGLINE_OUTER_RECORD="$delegation_record"
zero_oid="$(printf '%040d' 0)"
remote_oid="$(printf '%040d' 1)"
deletion_record="refs/heads/topic $zero_oid refs/heads/topic $remote_oid"
outer_success_output="$(printf '%s\n' "$deletion_record" |
  HOOK_DELEGATION_DEPTH=1 GIT_CONFIG_GLOBAL="$delegation_config" \
  "$ROOT/.githooks/pre-push" origin /tmp/remote 2>&1)"
equal "an ambient delegation marker cannot suppress the outer gate" \
  "argv:origin|/tmp/remote
$deletion_record" "$(cat "$delegation_record")"
# The outer gate's own two lines are the whole of this reading: no banner, no
# duplicate, no wrapping, nothing else. Exactly one dimension is normalised away
# and it is the only one that is not determined — with the output inherited
# rather than captured, the outer hook's stdout is block-buffered into this
# capture pipe while its stderr is not, so which marker lands first is a
# property of stdio, not of Gangline. Sorting settles that and nothing else.
equal "the outer gate's two lines are the whole of the output, once each" \
  "OUTER_STDERR_MARKER
OUTER_STDOUT_MARKER" \
  "$(printf '%s\n' "$outer_success_output" | LC_ALL=C sort)"

# Exercise the ordering property with local output after the outer gate. An
# empty-tree commit has no suite, so Gangline's own missing-gate refusal is the
# following line without paying for a nested integration run.
order_hooks="$RUN_ROOT/pre-push-ordering-hooks"
order_config="$RUN_ROOT/pre-push-ordering-gitconfig"
order_remote="$RUN_ROOT/pre-push-ordering-remote.git"
mkdir -p "$order_hooks"
GIT_CONFIG_GLOBAL="$order_config" git init -q --bare "$order_remote"
cat > "$order_hooks/pre-push" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cat > /dev/null
printf 'OUTER-VERDICT-MARKER\n' >&2
exit 0
SH
chmod +x "$order_hooks/pre-push"
git config --file "$order_config" core.hooksPath "$order_hooks"
order_oid="$(GIT_AUTHOR_NAME='Gangline test' \
  GIT_AUTHOR_EMAIL='test@gangline.invalid' \
  GIT_COMMITTER_NAME='Gangline test' \
  GIT_COMMITTER_EMAIL='test@gangline.invalid' \
  git -C "$ROOT" commit-tree \
  "$(git -C "$ROOT" mktree </dev/null)" -m 'feat: lintless ordering unit')"
order_output="$(
  cd "$ROOT"
  printf 'refs/heads/main %s refs/heads/main %s\n' \
    "$order_oid" "$(printf '%040d' 0)" |
  GIT_CONFIG_GLOBAL="$order_config" \
    "$ROOT/.githooks/pre-push" origin "$order_remote" 2>&1
)" || true
order_gate_line="$(printf '%s\n' "$order_output" |
  awk '/carries no test\/lint.sh/ { print NR; exit }')"
order_marker_line="$(printf '%s\n' "$order_output" |
  awk '/OUTER-VERDICT-MARKER/ { print NR; exit }')"
contains "the ordering fixture reaches Gangline's own gate" \
  "$order_output" "carries no test/lint.sh"
contains "the ordering fixture reaches the outer gate" \
  "$order_output" "OUTER-VERDICT-MARKER"
equal "the outer verdict is printed before Gangline's own output" \
  "before" \
  "$(if [ -n "$order_gate_line" ] && [ -n "$order_marker_line" ] \
        && [ "$order_marker_line" -lt "$order_gate_line" ]; then
       printf before
     else
       printf after
     fi)"

# Ordering after the fact is not the property. A capture that replayed the
# outer gate's output immediately before Gangline's own gates would satisfy
# every assertion above and still withhold each progress line until the outer
# process exited, which is the whole defect. So the outer fixture proves the
# live property from inside its own run, and needs no barrier to hang on: it
# writes, and then — still running — reads the file the local hook's own output
# is going to. Inherited, its lines are already there. Captured, they are in a
# temporary file this fixture cannot name.
#
# Both streams, because the contract is both and a capture of either one alone
# passes every other assertion here. stderr is unbuffered and needs nothing.
# stdout is written by awk rather than by this shell: a child's exit flushes it
# into the inherited descriptor, where this shell's own printf would still be
# sitting in a stdio buffer with nothing to distinguish it from a capture. The
# record names the streams that arrived rather than answering yes or no, so a
# failure says which one was withheld.
live_hooks="$RUN_ROOT/pre-push-live-hooks"
live_config="$RUN_ROOT/pre-push-live-gitconfig"
live_out="$RUN_ROOT/pre-push-live.out"
live_record="$RUN_ROOT/pre-push-live.record"
mkdir -p "$live_hooks"
cat > "$live_hooks/pre-push" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cat > /dev/null
awk 'BEGIN { print "OUTER-PROGRESS-STDOUT" }'
printf 'OUTER-PROGRESS-STDERR\n' >&2
record=""
if grep -Fxq 'OUTER-PROGRESS-STDOUT' "$GANGLINE_LIVE_OUT"; then record="stdout"; fi
if grep -Fxq 'OUTER-PROGRESS-STDERR' "$GANGLINE_LIVE_OUT"; then record="$record stderr"; fi
printf '%s\n' "${record# }" > "$GANGLINE_LIVE_RECORD"
exit 0
SH
chmod +x "$live_hooks/pre-push"
git config --file "$live_config" core.hooksPath "$live_hooks"
export GANGLINE_LIVE_OUT="$live_out"
export GANGLINE_LIVE_RECORD="$live_record"
printf '%s\n' "$deletion_record" |
  GIT_CONFIG_GLOBAL="$live_config" \
  "$ROOT/.githooks/pre-push" origin /tmp/remote > "$live_out" 2>&1
equal "both of the outer gate's streams are on screen before that gate returns" \
  "stdout stderr" "$(cat "$live_record")"

outer_refusal_rc=0
outer_refusal_output="$(printf '%s\n' "$deletion_record" |
  GANGLINE_OUTER_RC=23 GIT_CONFIG_GLOBAL="$delegation_config" \
  "$ROOT/.githooks/pre-push" origin /tmp/remote 2>&1)" || outer_refusal_rc=$?
equal "an outer refusal is the local hook's exact status" 23 "$outer_refusal_rc"
contains "an outer refusal is named before Gangline's gates run" \
  "$outer_refusal_output" "outer gate refused with status 23"

empty_global="$delegation_root/empty-global"
: > "$empty_global"
no_outer_rc=0
no_outer_output="$(printf '%s\n' "$deletion_record" |
  GIT_CONFIG_GLOBAL="$empty_global" \
  "$ROOT/.githooks/pre-push" origin /tmp/remote 2>&1)" || no_outer_rc=$?
equal "no global hook is a silent no-op before Gangline's gates" \
  "0|" "$no_outer_rc|$no_outer_output"

# `ln -sf` DEREFERENCES a symlink-to-directory: with the destination already a
# link to a directory it writes <that directory>/gang and leaves the link
# standing, so the installer mutates a directory nobody named and only fails
# afterwards, when it tries to execute the still-directory destination.
installer_root="$RUN_ROOT/installer"
installer_src="$installer_root/src"
mkdir -p "$installer_src/bin"
cat > "$installer_src/bin/gang" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
echo bash
SH
chmod +x "$installer_src/bin/gang"
git init -q "$installer_src"
git -C "$installer_src" config user.name 'Gangline installer test'
git -C "$installer_src" config user.email 'installer@fixture.invalid'
git -C "$installer_src" add .
git -C "$installer_src" commit -qm 'test: installer source fixture'

installer_bin="$installer_root/bin"
installer_decoy="$installer_root/decoy"
mkdir -p "$installer_bin" "$installer_decoy"
ln -s "$installer_decoy" "$installer_bin/gang"
installer_rc=0
GANGLINE_REPO="$installer_src" \
  GANGLINE_HOME="$installer_root/home" GANGLINE_BIN="$installer_bin" \
  sh "$ROOT/install.sh" >/dev/null 2>&1 || installer_rc=$?
equal "the installer replaces a symlinked destination instead of writing through it" \
  "0 absent $installer_root/ho""me/bin/gang" \
  "$installer_rc $([ -e "$installer_decoy/gang" ] && printf written || printf absent) $(readlink "$installer_bin/gang" || true)"

installer_dir_bin="$installer_root/bin-dir"
mkdir -p "$installer_dir_bin/gang"
installer_dir_rc=0
installer_dir_out="$(GANGLINE_REPO="$installer_src" \
  GANGLINE_HOME="$installer_root/home-dir" GANGLINE_BIN="$installer_dir_bin" \
  sh "$ROOT/install.sh" 2>&1)" || installer_dir_rc=$?
equal "the installer refuses a destination it cannot replace rather than filling it" \
  "refused empty named" \
  "$([ "$installer_dir_rc" -ne 0 ] && printf refused || printf installed) $([ -e "$installer_dir_bin/gang/gang" ] && printf filled || printf empty) $([[ "$installer_dir_out" = *'is not a file or a symlink'* ]] && printf named || printf unnamed)"

# The local sibling gate uses destination identity rather than remote-tracking
# names, and refuses non-commit refs instead of treating an empty traversal as
# a clean result. Its fixtures isolate the local hook from the host-global gate.
gate_root="$RUN_ROOT/local-pre-push"
gate_global="$gate_root/empty-global"
mkdir -p "$gate_root"
: > "$gate_global"

gate_bare() { # $1 name
  local repo="$gate_root/$1.git"
  rm -rf "$repo"
  GIT_CONFIG_GLOBAL="$gate_global" git init -q --bare "$repo"
  printf '%s\n' "$repo"
}

gate_repo() { # $1 name
  local repo="$gate_root/$1"
  rm -rf "$repo"
  GIT_CONFIG_GLOBAL="$gate_global" git init -q "$repo"
  git -C "$repo" config user.name 'Gangline gate test'
  git -C "$repo" config user.email 'gangline@fixture.invalid'
  mkdir -p "$repo/test" "$repo/.githooks"
  cp "$ROOT/.githooks/commit-msg" "$repo/.githooks/commit-msg"
  cat > "$repo/test/lint.sh" <<'SH'
#!/bin/sh
exit 0
SH
  cp "$repo/test/lint.sh" "$repo/test/integration.sh"
  chmod +x "$repo/test/"*.sh "$repo/.githooks/commit-msg"
  printf '%s\n' clean > "$repo/content"
  git -C "$repo" add .
  git -C "$repo" commit -qm 'test: local gate base'
  git -C "$repo" config core.hooksPath "$ROOT/.githooks"
  printf '%s\n' "$repo"
}

gate_bad_commit() { # $1 repo, $2 content
  printf '%s\n' "$2" > "$1/content"
  git -C "$1" add content
  git -C "$1" commit --no-verify -qm 'not a conventional commit'
}

gate_push() { # $1 repo, remaining git-push args
  local repo="$1"
  shift
  GATE_PUSH_RC=0
  GATE_PUSH_OUTPUT="$(GIT_CONFIG_GLOBAL="$gate_global" \
    git -C "$repo" push "$@" 2>&1)" || GATE_PUSH_RC=$?
}

gate_ref() { git --git-dir="$1" rev-parse --verify "$2" 2>/dev/null || true; }

pushurl_repo="$(gate_repo local-gate-pushurl)"
pushurl_private="$(gate_bare local-gate-pushurl-private)"
pushurl_public="$(gate_bare local-gate-pushurl-public)"
git -C "$pushurl_repo" remote add dest "$pushurl_private"
gate_bad_commit "$pushurl_repo" clean-pushurl
GIT_CONFIG_GLOBAL="$gate_global" git -C "$pushurl_repo" push \
  --no-verify -qu dest HEAD:refs/heads/topic
git -C "$pushurl_repo" remote set-url --add --push dest "$pushurl_public"
gate_push "$pushurl_repo" dest HEAD:refs/heads/public-topic
equal "the repository gate refuses a same-name pushurl destination escape" \
  "blocked absent named" \
  "$([ "$GATE_PUSH_RC" -ne 0 ] && printf blocked || printf leaked) $([ -z "$(gate_ref "$pushurl_public" refs/heads/public-topic)" ] && printf absent || printf received) $([[ "$GATE_PUSH_OUTPUT" = *'do not conform'* ]] && printf named || printf unnamed)"

retarget_repo="$(gate_repo local-gate-retarget)"
retarget_old="$(gate_bare local-gate-retarget-old)"
retarget_new="$(gate_bare local-gate-retarget-new)"
git -C "$retarget_repo" remote add dest "$retarget_old"
gate_bad_commit "$retarget_repo" clean-retarget
GIT_CONFIG_GLOBAL="$gate_global" git -C "$retarget_repo" push \
  --no-verify -qu dest HEAD:refs/heads/topic
git -C "$retarget_repo" remote set-url dest "$retarget_new"
gate_push "$retarget_repo" dest HEAD:refs/heads/topic
equal "the repository gate refuses a same-name retargeted destination escape" \
  "blocked absent named" \
  "$([ "$GATE_PUSH_RC" -ne 0 ] && printf blocked || printf leaked) $([ -z "$(gate_ref "$retarget_new" refs/heads/topic)" ] && printf absent || printf received) $([[ "$GATE_PUSH_OUTPUT" = *'do not conform'* ]] && printf named || printf unnamed)"

blob_repo="$(gate_repo local-gate-blob)"
blob_remote="$(gate_bare local-gate-blob)"
blob_oid="$(printf '%s\n' clean-blob | git -C "$blob_repo" hash-object -w --stdin)"
git -C "$blob_repo" update-ref refs/blobs/direct "$blob_oid"
gate_push "$blob_repo" "$blob_remote" refs/blobs/direct:refs/blobs/direct
blob_present=0
git --git-dir="$blob_remote" cat-file -e "$blob_oid" 2>/dev/null && blob_present=1
equal "the repository gate refuses a direct blob ref without destination receipt" \
  "blocked blob absent" \
  "$([ "$GATE_PUSH_RC" -ne 0 ] && printf blocked || printf leaked) $([[ "$GATE_PUSH_OUTPUT" = *'blob, not a commit'* ]] && printf blob || printf unnamed) $([ "$blob_present" -eq 0 ] && printf absent || printf received)"

blob_mirror="$(gate_bare local-gate-blob-mirror)"
gate_push "$blob_repo" --mirror "$blob_mirror"
mirror_present=0
git --git-dir="$blob_mirror" cat-file -e "$blob_oid" 2>/dev/null && mirror_present=1
equal "the repository gate refuses a mirror carrying a blob ref" \
  "blocked absent" \
  "$([ "$GATE_PUSH_RC" -ne 0 ] && printf blocked || printf leaked) $([ "$mirror_present" -eq 0 ] && printf absent || printf received)"

# The pre-push gate must run git-aware lint from the pushed tree even when Git
# gives the hook a GIT_DIR pointing at the main repository. A staged-only file
# makes a leaked main index distinguishable from the detached worktree's index.
hook_repo="$RUN_ROOT/pre-push-repo"
hook_probe="$RUN_ROOT/pre-push-probe"
mkdir -p "$hook_repo/.githooks" "$hook_repo/test" "$hook_probe"
cp "$ROOT/.githooks/pre-push" "$ROOT/.githooks/commit-msg" \
  "$hook_repo/.githooks/"
chmod +x "$hook_repo/.githooks/pre-push" "$hook_repo/.githooks/commit-msg"
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
hook_remote="$RUN_ROOT/pre-push-hook-remote.git"
GIT_CONFIG_GLOBAL="$gate_global" git init -q --bare "$hook_remote"
hook_missing=1111111111111111111111111111111111111111
hook_base_rc=0
hook_base_out="$({
  cd "$hook_repo"
  printf 'refs/heads/main %s refs/heads/main %s\n' "$hook_sha" "$hook_missing" |
    env GIT_CONFIG_GLOBAL="$gate_global" GIT_DIR="$hook_repo/.git" \
      ./.githooks/pre-push origin "$hook_remote"
} 2>&1)" || hook_base_rc=$?
equal "pre-push refuses an unavailable base of the pushed ref" \
  "refused named" \
  "$([ "$hook_base_rc" -ne 0 ] && printf refused || printf passed) $([[ "$hook_base_out" = *"remote base $hook_missing"* ]] && printf named || printf unnamed)"
hook_range_rc=0
hook_range_out="$({
  cd "$hook_repo"
  printf 'refs/heads/main %s refs/heads/main %s\n' "$hook_sha" "$hook_zero" |
    env GIT_CONFIG_GLOBAL="$gate_global" GIT_DIR="$hook_repo/.git" \
      ./.githooks/pre-push origin "$RUN_ROOT/missing-pre-push-remote"
} 2>&1)" || hook_range_rc=$?
equal "pre-push refuses an indeterminate new-ref range" \
  "refused named" \
  "$([ "$hook_range_rc" -ne 0 ] && printf refused || printf passed) $([[ "$hook_range_out" = *'pushed commit range is indeterminate'* ]] && printf named || printf unnamed)"
hook_unbounded_remote="$RUN_ROOT/pre-push-unbounded-remote.git"
GIT_CONFIG_GLOBAL="$gate_global" git init -q --bare "$hook_unbounded_remote"
mkdir -p "$hook_unbounded_remote/refs/pull/100"
printf '%s\n' "$hook_missing" > "$hook_unbounded_remote/refs/pull/100/head"
hook_unbounded_rc=0
hook_unbounded_out="$({
  cd "$hook_repo"
  printf 'refs/heads/main %s refs/heads/main %s\n' "$hook_sha" "$hook_zero" |
    env GIT_CONFIG_GLOBAL="$gate_global" GIT_DIR="$hook_repo/.git" \
      ./.githooks/pre-push origin "$hook_unbounded_remote"
} 2>&1)" || hook_unbounded_rc=$?
equal "pre-push refuses a nonempty advertisement with no usable boundary" \
  "refused named" \
  "$([ "$hook_unbounded_rc" -ne 0 ] && printf refused || printf passed) $([[ "$hook_unbounded_out" = *"none of the destination's advertised commits is available locally"* ]] && printf named || printf unnamed)"
mkdir -p "$hook_remote/refs/heads" "$hook_remote/refs/pull/100"
printf '%s\n' "$hook_sha" > "$hook_remote/refs/heads/main"
printf '%s\n' "$hook_missing" > "$hook_remote/refs/pull/100/head"
if hook_out="$({
  cd "$hook_repo"
  printf 'refs/heads/main %s refs/heads/main %s\n' "$hook_sha" "$hook_zero" |
    env GIT_CONFIG_GLOBAL="$gate_global" GIT_DIR="$hook_repo/.git" \
      HOOK_DELEGATION_DEPTH=1 \
      PROBE_DIR="$hook_probe" \
      ./.githooks/pre-push origin "$hook_remote"
} 2>&1)"; then
  pass "pre-push ignores an unrelated advertised commit absent locally"
else
  fail "pre-push ignores an unrelated advertised commit absent locally" "$hook_out"
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

# GATED TEARDOWN. `down` is the one irreversible verb, so it is exercised
# against a session of its own: a guard that is missing must not end this run.
teardown_session="teardown-probe-$$"
tmux new-session -d -s "$teardown_session" -n bystander \
  "sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
refuses "bare gang down names the session it would end" \
  "gang down" env GANG_SESSION="$teardown_session" "$GANG" down
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  pass "and the session it refused to end is still running"
else
  fail "and the session it refused to end is still running" "session is gone"
fi
refuses "gang down refuses a session name that is not this team" \
  "refusing to end a session you did not name" \
  env GANG_SESSION="$teardown_session" "$GANG" down some-other-name
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  pass "and the mismatched session is still running"
else
  fail "and the mismatched session is still running" "session is gone"
fi
teardown_pane="$(tmux list-panes -t "=$teardown_session" -F '#{pane_id}' | head -1)"
refuses "an agent cannot end the session it is running in" \
  "cannot end the session it is running in" \
  env GANG_SESSION="$teardown_session" TMUX_PANE="$teardown_pane" \
  "$GANG" down "$teardown_session"
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  pass "and the session its agent could not end is still running"
else
  fail "and the session its agent could not end is still running" "session is gone"
fi
teardown_socket="$(tmux display-message -p '#{socket_path}')"
refuses "a scrubbed pane cannot end a session on its own server" \
  "cannot tell whether it is running inside the team" \
  env -u TMUX_PANE GANG_SESSION="$teardown_session" \
  TMUX="$teardown_socket,1,0" "$GANG" down "$teardown_session"
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  pass "and the scrubbed pane left the session running"
else
  fail "and the scrubbed pane left the session running" "session is gone"
fi
refuses "gang down refuses a second argument" \
  "down: unexpected argument 'extra'" \
  env GANG_SESSION="$teardown_session" "$GANG" down "$teardown_session" extra
env GANG_SESSION="$teardown_session" "$GANG" hitch spoolable \
  -c spoolable -d /tmp >/dev/null
teardown_spoolable_id="$(window_id_in "$teardown_session" spoolable)"
tmux send-keys -l -t "$teardown_spoolable_id" 'HUMAN_DRAFT'
printf 'MARK_TEARDOWN_ARCHIVE' |
  env GANG_SESSION="$teardown_session" "$GANG" send \
    --to spoolable --from tester --stdin >/dev/null
teardown_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$teardown_spoolable_id" @gl_spool)"
teardown_entry=""
for spool_entry in "$teardown_spool"/[0-9]*; do
  [ -f "$spool_entry" ] || continue
  teardown_entry="$spool_entry"
  break
done
[ -n "$teardown_entry" ] && cp "$teardown_entry" "$RUN_ROOT/pre-down-body"
teardown_down_out="$(env -u TMUX -u TMUX_PANE \
  GANG_SESSION="$teardown_session" "$GANG" down "$teardown_session")"
contains "the named teardown reports where it archived pending mail" \
  "$teardown_down_out" "$GANG_ARCHIVE_DIR"
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  fail "a named teardown from outside the session still ends it" \
    "session still exists"
else
  pass "a named teardown from outside the session still ends it"
fi
teardown_archived_entry=""
for archived_entry in "$GANG_ARCHIVE_DIR"/*/spoolable/${teardown_entry##*/}; do
  [ -f "$archived_entry" ] || continue
  teardown_archived_entry="$archived_entry"
  break
done
if [ -n "$teardown_archived_entry" ] \
   && cmp "$RUN_ROOT/pre-down-body" "$teardown_archived_entry"; then
  pass "a whole-team teardown preserves its pending message in the archive"
else
  fail "a whole-team teardown preserves its pending message in the archive" \
    "${teardown_archived_entry:-archived entry is absent}"
fi
tmux new-session -d -s "$teardown_session" -n bystander \
  "sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
observer_session="${teardown_session}-obs"
tmux new-session -d -s "$observer_session" -n observer \
  "sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
observer_pane="$(tmux list-panes -t "=$observer_session" -F '#{pane_id}' | head -1)"
env GANG_SESSION="$teardown_session" TMUX="$teardown_socket,1,0" \
  TMUX_PANE="$observer_pane" "$GANG" down "$teardown_session" >/dev/null
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  fail "a pane in another session can end the named team" "session still exists"
else
  pass "a pane in another session can end the named team"
fi
tmux kill-session -t "=$observer_session"

"$GANG" down "$GANG_SESSION" >/dev/null
if tmux has-session -t "=$GANG_SESSION" 2>/dev/null; then
  fail "down removes the exact test session" "session still exists"
else
  pass "down removes the exact test session"
fi
[ ! -d "$lingering_spool" ] \
  && pass "and takes the spool of every window in it" \
  || fail "and takes the spool of every window in it" "$lingering_spool survived"

# THE GATE OWNS THE TREE IT JUDGES. Two failures wrote test/gate.sh: a mandatory
# assertion that could not pass while bin/gang was uncommitted, so the complete
# gate only ever ran after a commit; and a run whose source was edited while it
# ran, which reported the editor rather than the code. Both are one problem —
# the run and the working tree were not separated — so these fixtures drive the
# separation itself rather than either symptom.
gate_fix="$RUN_ROOT/gate-fixture"
mkdir -p "$gate_fix/bin" "$gate_fix/test"
cp "$ROOT/test/gate.sh" "$gate_fix/test/gate.sh"
cp "$GANG" "$gate_fix/bin/gang"
cp -R "$ROOT/collars" "$gate_fix/collars"
printf 'ignored.txt\n' > "$gate_fix/.gitignore"
printf 'DOOMED\n' > "$gate_fix/doomed.txt"
# Same identity domain as gang_root and the dirty-execution fixture above:
# macOS reaches TMPDIR through a symlink and the printed path is the physical
# one.
gate_fix="$(cd -P "$gate_fix" && pwd)"
git -C "$gate_fix" init -q
git -C "$gate_fix" add -A
git -C "$gate_fix" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: gate fixture'
gate_head="$(git -C "$gate_fix" rev-parse 'HEAD^{tree}')"

equal "a settled tree answers with the object name of its own bytes" \
  "settled $gate_head $gate_fix" \
  "$("$gate_fix/test/gate.sh" --assert-owned)"
printf '\n# fixture dirt\n' >> "$gate_fix/bin/gang"
refuses "a moving tree is refused as one no run can own" \
  "would not own the tree it is testing" \
  "$gate_fix/test/gate.sh" --assert-owned
refuses "the refusal hands over the command that does own a tree" \
  "test/gate.sh" "$gate_fix/test/gate.sh" --assert-owned
git -C "$gate_fix" checkout -q -- bin/gang

# An operator who has turned untracked reporting off must not thereby turn this
# check off: a new collar or role file is exactly the kind of untracked file
# that changes what a run executes, and inheriting `status.showUntrackedFiles`
# would report a tree nobody owns as settled.
printf 'stray\n' > "$gate_fix/untracked-collar.sh"
git -C "$gate_fix" config status.showUntrackedFiles no
equal "the fixture really did hide untracked files from ordinary status" "" \
  "$(git -C "$gate_fix" status --porcelain)"
refuses "an untracked file still makes the tree unownable" \
  "would not own the tree it is testing" \
  "$gate_fix/test/gate.sh" --assert-owned
git -C "$gate_fix" config --unset status.showUntrackedFiles

# NOR MAY A CALLER'S ENVIRONMENT DECIDE WHAT THIS TREE IS. The suite exports a
# private GIT_CONFIG_GLOBAL partway through its own setup, so a check that
# inherited it would answer one question before that line and a different one
# after, and could report movement that never happened. A configuration that
# ignores everything is the sharpest form of that: inherited, it turns every
# untracked file into no file at all.
printf 'gitignore-everything\n' > "$RUN_ROOT/gate-ignore-all"
printf '*\n' > "$RUN_ROOT/gate-excludes-all"
printf '[core]\n\texcludesFile = %s\n[status]\n\tshowUntrackedFiles = no\n' \
  "$RUN_ROOT/gate-excludes-all" > "$RUN_ROOT/gate-hostile-gitconfig"
refuses "a caller's git configuration cannot blind the ownership check" \
  "would not own the tree it is testing" \
  env GIT_CONFIG_GLOBAL="$RUN_ROOT/gate-hostile-gitconfig" \
  GIT_CONFIG_SYSTEM=/dev/null "$gate_fix/test/gate.sh" --assert-owned
# One door per probe, because a denylist that closes four of five reads exactly
# as green as one that closes all five.
refuses "nor the numbered configuration triples" \
  "would not own the tree it is testing" \
  env GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.excludesFile \
  GIT_CONFIG_VALUE_0="$RUN_ROOT/gate-excludes-all" \
  "$gate_fix/test/gate.sh" --assert-owned
refuses "nor the parameter channel that overrides even a pinned file" \
  "would not own the tree it is testing" \
  env GIT_CONFIG_PARAMETERS="'core.excludesFile=$RUN_ROOT/gate-excludes-all'" \
  "$gate_fix/test/gate.sh" --assert-owned
mkdir -p "$RUN_ROOT/gate-xdg/git" "$RUN_ROOT/gate-home"
printf '[core]\n\texcludesFile = %s\n[status]\n\tshowUntrackedFiles = no\n' \
  "$RUN_ROOT/gate-excludes-all" > "$RUN_ROOT/gate-xdg/git/config"
cp "$RUN_ROOT/gate-xdg/git/config" "$RUN_ROOT/gate-home/.gitconfig"
refuses "nor the configuration search path the suite itself moves" \
  "would not own the tree it is testing" \
  env XDG_CONFIG_HOME="$RUN_ROOT/gate-xdg" HOME="$RUN_ROOT/gate-home" \
  "$gate_fix/test/gate.sh" --assert-owned

rm -f "$gate_fix/untracked-collar.sh"
gate_identity="$("$gate_fix/test/gate.sh" --assert-owned)"
if "$gate_fix/test/gate.sh" --assert-unmoved "$gate_identity"; then
  pass "a tree that held still keeps the identity its run started with"
else
  fail "a tree that held still keeps the identity its run started with" \
    "the unchanged fixture reported movement"
fi
printf '\n# fixture dirt\n' >> "$gate_fix/bin/gang"
refuses "a tree that moved mid-run voids the verdict rather than passing it" \
  "THE SOURCE TREE MOVED DURING THIS RUN" \
  "$gate_fix/test/gate.sh" --assert-unmoved "$gate_identity"

# A COMMIT IS ALSO MOVEMENT. A tree that is settled at the start and settled at
# the end has still changed if the commit under it changed, and that reading is
# the one a teammate landing work mid-run produces.
git -C "$gate_fix" checkout -q -- bin/gang
gate_settled_identity="$("$gate_fix/test/gate.sh" --assert-owned)"
printf 'landed mid-run\n' > "$gate_fix/second.txt"
git -C "$gate_fix" add -A
git -C "$gate_fix" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: a commit landing mid-run'
equal "the moved tree is settled again, so only its commit changed" "" \
  "$(git -C "$gate_fix" status --porcelain)"
refuses "a commit landing mid-run voids the verdict too" \
  "THE SOURCE TREE MOVED DURING THIS RUN" \
  "$gate_fix/test/gate.sh" --assert-unmoved "$gate_settled_identity"

# THE EXACT PAIR THAT MADE THE SUITE UNRUNNABLE, from one set of bytes: the
# uncommitted executable warns on stderr where it lives, and is silent inside
# the snapshot, because the snapshot's own HEAD holds those same bytes. No
# assertion was relaxed to get there and there is no suite-only switch to relax
# one later.
gate_dirt='# named gate-snapshot mutant'
printf '%s\n' "$gate_dirt" >> "$gate_fix/bin/gang"
rm -f "$gate_fix/doomed.txt"
printf 'UNTRACKED_GATE_FILE\n' > "$gate_fix/untracked.txt"
printf 'IGNORED_GATE_FILE\n' > "$gate_fix/ignored.txt"
contains "the uncommitted executable warns where it lives" \
  "$("$gate_fix/bin/gang" collars 2>&1)" "WARNING: executing dirty"
gate_snap="$RUN_ROOT/gate-snapshot"
"$gate_fix/test/gate.sh" --snapshot "$gate_snap"
gate_snap_run="$RUN_ROOT/gate-snapshot-run.out"
if env -u GANG_COLLARS "$gate_snap/bin/gang" collars \
    > "$gate_snap_run" 2>&1; then
  pass "the executable in the snapshot runs to its ordinary output"
else
  fail "the executable in the snapshot runs to its ordinary output" \
    "it exited non-zero: [$(<"$gate_snap_run")]"
fi
# Silence is only evidence if the command got far enough to have spoken. The
# dirty warning is printed before dispatch, so an executable that died early
# would satisfy the exclusion below while proving nothing.
contains "and that output is the collar listing it was asked for" \
  "$(<"$gate_snap_run")" "bash"
excludes "the same bytes raise no dirty-execution warning in the snapshot" \
  "$(<"$gate_snap_run")" "executing dirty"
# WHY it is silent has to be the reason claimed. A snapshot with no commit at
# all is silent too, because the warning abstains when it cannot resolve a
# HEAD, and that silence would be an absent instrument reported as a clean one.
equal "the snapshot is silent because its own HEAD holds those exact bytes" \
  "$(cksum < "$gate_snap/bin/gang")" \
  "$(git -C "$gate_snap" show HEAD:bin/gang 2>/dev/null | cksum)"
contains "the snapshot carries the uncommitted work it was taken from" \
  "$(<"$gate_snap/bin/gang")" "$gate_dirt"
equal "the snapshot's own worktree is settled against its own HEAD" "" \
  "$(git -C "$gate_snap" status --porcelain)"
equal "an untracked file is part of the tree the gate tests" \
  "UNTRACKED_GATE_FILE" "$(<"$gate_snap/untracked.txt")"
if [ -e "$gate_snap/doomed.txt" ]; then
  fail "a deleted tracked file reaches the snapshot as a deletion" \
    "the snapshot restored a file the working tree no longer has"
else
  pass "a deleted tracked file reaches the snapshot as a deletion"
fi
if [ -e "$gate_snap/ignored.txt" ]; then
  fail "an ignored file is not part of the tree" \
    "the snapshot copied a file git was told to ignore"
else
  pass "an ignored file is not part of the tree"
fi

# AN EDIT THAT LANDS WHILE THE TREE IS BEING COPIED is the one corruption the
# snapshot cannot prevent by existing, so it is detected instead. The stand-in
# git edits the fixture on the second listing, which is exactly when a real
# editor's save would land, and needs no wall clock to do it.
# A relative symlink inside the tree is carried; one pointing out of it is not,
# because its text survives the copy and then resolves against another parent.
ln -s bin/gang "$gate_fix/gang-link"
ln -s /etc/hostname "$gate_fix/absolute-link"
gate_git_bin="$RUN_ROOT/gate-git"
mkdir -p "$gate_git_bin"
gate_real_git="$(command -v git)"
cat > "$gate_git_bin/git" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Only the NUL-separated listing is counted: that is the one the copy is built
# from and the one it is verified against, so the second of those two is the
# moment an editor's save would land inside the copy window. The index probe
# reads the same command with different flags and must not be mistaken for it.
gl_is_list=0
gl_has_z=0
for gl_arg in "\$@"; do
  [ "\$gl_arg" = ls-files ] && gl_is_list=1
  [ "\$gl_arg" = -z ] && gl_has_z=1
done
if [ "\$gl_is_list" = 1 ] && [ "\$gl_has_z" = 1 ]; then
  count="$RUN_ROOT/gate-ls-\${GATE_DRIFT_MODE:-none}"
  n="\$(cat "\$count" 2>/dev/null)" || n=0
  n=\$(( \${n:-0} + 1 ))
  printf '%s\n' "\$n" > "\$count"
  if [ "\$n" -ge 2 ]; then
    case "\${GATE_DRIFT_MODE:-none}" in
      content) printf 'LATE_EDIT\n' >> "$gate_fix/bin/gang" ;;
      list) printf 'LATE_FILE\n' > "$gate_fix/late-file.txt" ;;
      mode) chmod -x "$gate_fix/bin/gang" ;;
      dest) printf 'FOREIGN\n' > "\${GATE_DRIFT_DEST:-/dev/null}/foreign.txt" ;;
      link) nl='
'; ln -sfn "bin/gang\$nl" "$gate_fix/gang-link" ;;
    esac
  fi
fi
exec "$gate_real_git" "\$@"
SH
chmod +x "$gate_git_bin/git"
refuses "a file edited during the copy makes the snapshot unusable" \
  "moved while it was being copied (bin/gang changed)" \
  env GATE_DRIFT_MODE=content PATH="$gate_git_bin:$PATH" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-drift-content"
refuses "a file appearing during the copy makes the snapshot unusable" \
  "moved while it was being copied (the set of files changed)" \
  env GATE_DRIFT_MODE=list PATH="$gate_git_bin:$PATH" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-drift-list"
# `chmod +x` moves a tree without moving a byte, and the executable bit is the
# one mode git records, so a comparison that reads only contents would call
# 100755 and 100644 the same tree.
refuses "a mode changed during the copy makes the snapshot unusable" \
  "moved while it was being copied (the mode of bin/gang changed)" \
  env GATE_DRIFT_MODE=mode PATH="$gate_git_bin:$PATH" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-drift-mode"
chmod +x "$gate_fix/bin/gang"
# A target ending in a newline is a real target, and command substitution would
# compare it equal to one that does not.
refuses "a symlink retargeted during the copy makes the snapshot unusable" \
  "moved while it was being copied (the symlink gang-link changed)" \
  env GATE_DRIFT_MODE=link PATH="$gate_git_bin:$PATH" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-drift-link"
ln -sfn bin/gang "$gate_fix/gang-link"
# A destination checked once at the start is a promise about a directory another
# process can still reach. This one is written into after that check and before
# the commit.
refuses "bytes arriving in the destination during the copy are refused" \
  "holds something the copy did not put there" \
  env GATE_DRIFT_MODE=dest GATE_DRIFT_DEST="$RUN_ROOT/gate-drift-dest" \
  PATH="$gate_git_bin:$PATH" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-drift-dest"
ln -s ../outside-the-tree "$gate_fix/escaping-link"
refuses "a relative symlink out of the tree is refused, not quietly relocated" \
  "pointing out of the tree" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-escape"
rm -f "$gate_fix/escaping-link"

# A DANGLING LINK IS NOT A HARMLESS ONE, and treating it as one made a green
# snapshot read bytes the source never held. `../missingdir/file` resolves to
# nothing beside the source and to a real file beside the destination, because a
# relative link resolves against whichever parent it finds itself under. So
# where the link POINTS decides this, not whether the source end of it exists —
# the referent is placed beside the destination here to make that difference the
# only thing the check can be answering.
mkdir -p "$RUN_ROOT/gate-dangle-parent/missingdir"
printf 'DESTINATION_ONLY\n' > "$RUN_ROOT/gate-dangle-parent/missingdir/file"
ln -s ../missingdir/file "$gate_fix/dangling-escape"
refuses "a dangling relative symlink out of the tree is refused as well" \
  "pointing out of the tree" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-dangle-parent/tree"
rm -f "$gate_fix/dangling-escape"
# The other direction, so the fix above is a distinction and not a ban: a link
# dangling INSIDE the tree is carried, because the copy is a subset of the
# source and a name missing here is missing there too.
ln -s ./nodir/file "$gate_fix/dangling-inside"
gate_dangle_in_rc=0
"$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-dangle-inside" >/dev/null 2>&1 ||
  gate_dangle_in_rc=$?
equal "a symlink dangling inside the tree is carried, not refused" \
  "0 ./nodir/file" \
  "$gate_dangle_in_rc $(readlink "$RUN_ROOT/gate-dangle-inside/dangling-inside" 2>/dev/null || printf missing)"
rm -f "$gate_fix/dangling-inside"

# A COPY THAT FAILED IS NOT A SNAPSHOT. Bash suspends `set -e` throughout a
# function whose caller tests its status, which is exactly how --snapshot calls
# this one, so a failing copy would otherwise run on to commit and report a
# half-tree as the thing under test.
mkdir -p "$RUN_ROOT/gate-broken-tar"
cat > "$RUN_ROOT/gate-broken-tar/tar" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
echo "tar: refusing, for the fixture" >&2
exit 1
SH
chmod +x "$RUN_ROOT/gate-broken-tar/tar"
refuses "a copy that failed is refused rather than committed as a snapshot" \
  "could not copy" \
  env PATH="$RUN_ROOT/gate-broken-tar:$PATH" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-broken-snapshot"
if [ -e "$RUN_ROOT/gate-broken-snapshot/.git" ]; then
  fail "the refused copy left no committed snapshot behind" \
    "a repository was initialised over a copy that never happened"
else
  pass "the refused copy left no committed snapshot behind"
fi
refuses "a destination inside the tree is named, not blamed on the tree" \
  "copying the tree into itself" \
  "$gate_fix/test/gate.sh" --snapshot "$gate_fix/inside-snapshot"
if [ -e "$gate_fix/inside-snapshot" ]; then
  fail "and the refusal left nothing new in the tree it refused to copy" \
    "the refused destination was created anyway"
else
  pass "and the refusal left nothing new in the tree it refused to copy"
fi

# A destination that already holds something keeps it: the overlay is committed
# alongside the copy, so the tree under test would be this tree plus somebody
# else's leftovers — including a file whose deletion is what was being tested.
mkdir -p "$RUN_ROOT/gate-occupied"
printf 'STALE\n' > "$RUN_ROOT/gate-occupied/stale.txt"
refuses "a destination that already holds something is not a snapshot" \
  "already holds something" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-occupied"
# An empty directory is a fine destination; a symlink is not, and a DANGLING one
# answers no to -e, so it would otherwise reach mkdir and refuse with a message
# about the wrong thing.
mkdir -p "$RUN_ROOT/gate-empty-dest"
if "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-empty-dest" >/dev/null 2>&1; then
  pass "an empty directory is a destination a snapshot may use"
else
  fail "an empty directory is a destination a snapshot may use" \
    "the gate refused a destination that held nothing"
fi
ln -s "$RUN_ROOT/gate-nowhere" "$RUN_ROOT/gate-dangling-dest"
refuses "a destination that is a dangling symlink is refused where it is found" \
  "already holds something" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-dangling-dest"
# A LIVE link to an empty directory answers yes to every test a plain empty
# directory does, and the snapshot would be built in a place nobody named.
mkdir -p "$RUN_ROOT/gate-link-referent"
ln -s "$RUN_ROOT/gate-link-referent" "$RUN_ROOT/gate-live-dest"
refuses "a destination that is a live symlink is refused too" \
  "already holds something" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-live-dest"
if [ -e "$RUN_ROOT/gate-link-referent/.git" ]; then
  fail "and nothing was committed through it" \
    "a repository was initialised in the link's referent"
else
  pass "and nothing was committed through it"
fi

# A SUBMODULE'S GITLINK IS A DIRECTORY IN THIS LISTING, and it would reach the
# byte comparison as one: cmp answers "Is a directory" and the tree gets blamed
# for moving. That is a true refusal for a false reason, so the unsupported file
# type is named where it is found. (A named pipe cannot get this far: git lists
# neither a tracked nor an untracked FIFO, so it is simply not part of the tree.)
mkdir -p "$gate_fix/inner"
git -C "$gate_fix/inner" init -q
printf 'inner\n' > "$gate_fix/inner/held.txt"
git -C "$gate_fix/inner" add -A
git -C "$gate_fix/inner" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: inner repository'
git -C "$gate_fix" update-index --add --cacheinfo \
  "160000,$(git -C "$gate_fix/inner" rev-parse HEAD),inner"
refuses "a file this gate cannot carry is refused, not silently omitted" \
  "neither a regular file nor a symlink" \
  "$gate_fix/test/gate.sh" --snapshot "$RUN_ROOT/gate-gitlink"
git -C "$gate_fix" update-index --force-remove inner
rm -rf "$gate_fix/inner"

# UNKNOWN IS NOT A PASS. A tree whose state cannot be read is not a tree this
# run owns, and reporting that as ownership is the whole failure this arc is
# about wearing the opposite verdict's clothes.
mkdir -p "$RUN_ROOT/gate-nogit/test"
cp "$ROOT/test/gate.sh" "$RUN_ROOT/gate-nogit/test/"
refuses "a tree that is not a checkout at all cannot be owned" \
  "cannot tell whether it owns the tree" \
  "$RUN_ROOT/gate-nogit/test/gate.sh" --assert-owned
mkdir -p "$RUN_ROOT/gate-blindgit"
cat > "$RUN_ROOT/gate-blindgit/git" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
for gl_arg in "\$@"; do
  [ "\$gl_arg" = status ] && exit 42
done
exec "$gate_real_git" "\$@"
SH
chmod +x "$RUN_ROOT/gate-blindgit/git"
refuses "a reading that failed is refused rather than reported as settled" \
  "cannot tell whether it owns the tree" \
  env PATH="$RUN_ROOT/gate-blindgit:$PATH" \
  "$gate_fix/test/gate.sh" --assert-owned

# The index can be told to report a file as unchanged without looking at it.
# A verdict resting on that is a promise, not a reading.
printf '\n# concealed edit\n' >> "$gate_fix/bin/gang"
git -C "$gate_fix" update-index --assume-unchanged bin/gang
equal "the fixture really did conceal the edit from ordinary status" "" \
  "$(git -C "$gate_fix" status --porcelain -- bin/gang)"
refuses "an index told not to look at a file cannot report the tree settled" \
  "the index is told not to look at some files" \
  "$gate_fix/test/gate.sh" --assert-owned
git -C "$gate_fix" update-index --no-assume-unchanged bin/gang
git -C "$gate_fix" checkout -q -- bin/gang

# A HALF-FINISHED OPERATION IS ONE COMMIT FROM MOVING HEAD, and it can leave the
# working tree byte-clean while it waits. The merge here is real and changes
# nothing, which is exactly the shape that reads as settled.
gate_alt="$RUN_ROOT/gate-operation"
mkdir -p "$gate_alt/test"
cp "$ROOT/test/gate.sh" "$gate_alt/test/"
gate_alt="$(cd -P "$gate_alt" && pwd)"
git -C "$gate_alt" init -q
printf 'base\n' > "$gate_alt/shared.txt"
git -C "$gate_alt" add -A
git -C "$gate_alt" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: operation fixture base'
git -C "$gate_alt" branch -q side
printf 'both sides agree\n' > "$gate_alt/shared.txt"
git -C "$gate_alt" add -A
git -C "$gate_alt" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: on the trunk'
git -C "$gate_alt" checkout -q side
printf 'both sides agree\n' > "$gate_alt/shared.txt"
git -C "$gate_alt" add -A
git -C "$gate_alt" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: on the branch'
git -C "$gate_alt" checkout -q -
git -C "$gate_alt" -c user.name=fixture -c user.email=fixture@example.invalid \
  merge --no-commit --no-ff side >/dev/null 2>&1 || true
equal "the half-finished merge really did leave the bytes clean" "" \
  "$(git -C "$gate_alt" status --porcelain)"
if [ -e "$gate_alt/.git/MERGE_HEAD" ]; then
  pass "and really did leave the merge open"
else
  fail "and really did leave the merge open" "no MERGE_HEAD was written"
fi
refuses "a tree one commit from moving HEAD is not a settled tree" \
  "MERGE_HEAD is still in progress" \
  "$gate_alt/test/gate.sh" --assert-owned
git -C "$gate_alt" merge --abort 2>/dev/null || true

# A SPARSE CHECKOUT'S TRACKED PATHS ARE ABSENT FROM THE WORKING TREE, so a copy
# of that tree drops them while its own HEAD would claim to be the whole tree.
gate_sparse="$RUN_ROOT/gate-sparse"
mkdir -p "$gate_sparse/test"
cp "$ROOT/test/gate.sh" "$gate_sparse/test/"
gate_sparse="$(cd -P "$gate_sparse" && pwd)"
printf 'held out of the working tree\n' > "$gate_sparse/absent.txt"
git -C "$gate_sparse" init -q
git -C "$gate_sparse" add -A
git -C "$gate_sparse" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: sparse fixture'
git -C "$gate_sparse" update-index --skip-worktree absent.txt
rm -f "$gate_sparse/absent.txt"
equal "the sparse fixture really did hide the missing file from status" "" \
  "$(git -C "$gate_sparse" status --porcelain)"
refuses "a tree whose index hides files is not one this gate can copy" \
  "standing orders not to look" \
  "$gate_sparse/test/gate.sh" --snapshot "$RUN_ROOT/gate-sparse-snapshot"

# A CACHE CANNOT ANSWER FOR THE BYTES. core.fsmonitor answers from a daemon, and
# when its hook cannot run git prints a fatal line, exits zero, and reports
# nothing changed — a dirty tree read as a clean one by an instrument that never
# looked.
gate_fsm="$RUN_ROOT/gate-fsmonitor"
mkdir -p "$gate_fsm/test"
cp "$ROOT/test/gate.sh" "$gate_fsm/test/"
gate_fsm="$(cd -P "$gate_fsm" && pwd)"
printf 'watched\n' > "$gate_fsm/watched.txt"
git -C "$gate_fsm" init -q
git -C "$gate_fsm" add -A
git -C "$gate_fsm" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: fsmonitor fixture'
# A real protocol-v2 hook, kept outside the tree so it is not itself a change:
# it answers with an unchanging token and no changed paths, which is exactly
# what a healthy monitor says about a tree nobody has touched.
cat > "$RUN_ROOT/gate-fsmonitor-hook" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf 'gate-fixture-token\0'
SH
chmod +x "$RUN_ROOT/gate-fsmonitor-hook"
git -C "$gate_fsm" config core.fsmonitor "$RUN_ROOT/gate-fsmonitor-hook"
git -C "$gate_fsm" status --porcelain >/dev/null 2>&1
printf 'edited behind the cache\n' >> "$gate_fsm/watched.txt"
equal "the fixture's cache really did report a modified tree as clean" "" \
  "$(git -C "$gate_fsm" status --porcelain 2>/dev/null)"
equal "and git itself sees the change once the cache is not asked" \
  " M watched.txt" \
  "$(git -C "$gate_fsm" -c core.fsmonitor=false status --porcelain)"
refuses "a tree read through a broken cache is not a settled tree" \
  "would not own the tree it is testing" \
  "$gate_fsm/test/gate.sh" --assert-owned

# THE THIRD BRANCH NEEDS ITS OWN FIXTURE. A listing that cannot be read is a
# reading that was not taken, and a skip-worktree bit is a standing order rather
# than evidence of change: both are unknown, and neither is ownership.
mkdir -p "$RUN_ROOT/gate-blindls"
cat > "$RUN_ROOT/gate-blindls/git" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
for gl_arg in "\$@"; do
  [ "\$gl_arg" = -v ] && exit 42
done
exec "$gate_real_git" "\$@"
SH
chmod +x "$RUN_ROOT/gate-blindls/git"
refuses "a listing that could not be read is an unknown, not ownership" \
  "cannot tell whether it owns the tree" \
  env PATH="$RUN_ROOT/gate-blindls:$PATH" \
  "$gate_fsm/test/gate.sh" --assert-owned
git -C "$gate_fsm" config --unset core.fsmonitor
git -C "$gate_fsm" checkout -q -- watched.txt
git -C "$gate_fsm" update-index --skip-worktree watched.txt
equal "the skip-worktree fixture really did leave status empty" "" \
  "$(git -C "$gate_fsm" status --porcelain)"
refuses "a skip-worktree bit is a standing order, not a settled tree" \
  "the index is told not to look at some files" \
  "$gate_fsm/test/gate.sh" --assert-owned
# And it is an UNKNOWN, not a claim about movement: the bit proves only that git
# was told not to look, which is a reading nobody took rather than a change
# somebody made.
refuses "and it is refused as a reading nobody took, not as a change" \
  "cannot tell whether it owns the tree" \
  "$gate_fsm/test/gate.sh" --assert-owned
# The end of a run has the same three branches as its start: a tree that stopped
# being readable did not stay the tree the run began against.
refuses "a tree that stopped being readable at the end voids the verdict" \
  "THE SOURCE TREE MOVED DURING THIS RUN" \
  "$gate_fsm/test/gate.sh" --assert-unmoved "settled whatever $gate_fsm"
git -C "$gate_fsm" update-index --no-skip-worktree watched.txt

# A CHECK THAT DISAPPEARS ON A BIG ENOUGH INDEX is not a check. `grep -q` stops
# at the first match, and under `pipefail` the SIGPIPE that kills the writer of
# a long listing turns a successful match into a failed pipeline — so the
# concealed entry has to be found past more output than a pipe will hold.
mkdir -p "$RUN_ROOT/gate-bigindex"
cat > "$RUN_ROOT/gate-bigindex/git" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
gl_v=0
for gl_arg in "\$@"; do
  [ "\$gl_arg" = -v ] && gl_v=1
done
if [ "\$gl_v" = 1 ]; then
  printf 'h concealed-payload\n'
  awk 'BEGIN { for (i = 0; i < 40000; i++) print "H filler-path-long-enough-to-fill-a-pipe-" i }'
  exit 0
fi
exec "$gate_real_git" "\$@"
SH
chmod +x "$RUN_ROOT/gate-bigindex/git"
equal "the clean fixture is settled before the big index is introduced" \
  "settled $(git -C "$gate_fsm" rev-parse 'HEAD^{tree}') $gate_fsm" \
  "$("$gate_fsm/test/gate.sh" --assert-owned)"
refuses "a concealed entry is found past more output than a pipe holds" \
  "the index is told not to look at some files" \
  env PATH="$RUN_ROOT/gate-bigindex:$PATH" \
  "$gate_fsm/test/gate.sh" --assert-owned

# WHERE ROOT IS A SUBDIRECTORY of a larger repository, an edit elsewhere in that
# repository is not movement in the tree this gate would copy, and the identity
# must name the subtree rather than the repository containing it.
gate_nest="$RUN_ROOT/gate-nested"
mkdir -p "$gate_nest/project/test"
cp "$ROOT/test/gate.sh" "$gate_nest/project/test/"
printf 'sibling\n' > "$gate_nest/sibling.txt"
gate_nest="$(cd -P "$gate_nest" && pwd)"
git -C "$gate_nest" init -q
git -C "$gate_nest" add -A
git -C "$gate_nest" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: nested gate fixture'
printf 'edited outside the copied subtree\n' >> "$gate_nest/sibling.txt"
gate_nest_identity=refused
gate_nest_identity="$("$gate_nest/project/test/gate.sh" --assert-owned)" || true
equal "the identity names the subtree that would be copied, not its container" \
  "settled $(git -C "$gate_nest" rev-parse HEAD:project) $gate_nest/project" \
  "$gate_nest_identity"
# A COMMIT OUTSIDE THE SUBTREE MOVES HEAD WITHOUT MOVING ONE COPIED BYTE. An
# identity taken from the containing repository's HEAD would void a subtree run
# for a teammate's unrelated landing — the same false verdict, other direction.
git -C "$gate_nest" add -A
git -C "$gate_nest" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: a sibling-only commit'
# An index bit on a sibling is a standing order about bytes this gate never
# copies, so the listing that looks for such orders is scoped like the rest.
git -C "$gate_nest" update-index --assume-unchanged sibling.txt
if "$gate_nest/project/test/gate.sh" --assert-unmoved "$gate_nest_identity"; then
  pass "a commit that touches no copied byte is not movement in the subtree"
else
  fail "a commit that touches no copied byte is not movement in the subtree" \
    "an unrelated sibling commit voided the run"
fi
printf 'edited inside the subtree\n' >> "$gate_nest/project/test/gate.sh"
git -C "$gate_nest" add -A
git -C "$gate_nest" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: a commit inside the subtree'
refuses "a commit that does touch one is movement in the subtree" \
  "THE SOURCE TREE MOVED DURING THIS RUN" \
  "$gate_nest/project/test/gate.sh" --assert-unmoved "$gate_nest_identity"

# THE GATE'S OWN ORCHESTRATION, which every check above leaves untouched: they
# drive --assert-owned, --assert-unmoved and --snapshot, so dropping the suite
# from the no-argument run would leave all of them green. Stand-in gates record
# that they ran, in order, from inside the copy.
gate_run="$RUN_ROOT/gate-default"
mkdir -p "$gate_run/test"
cp "$ROOT/test/gate.sh" "$gate_run/test/gate.sh"
gate_run="$(cd -P "$gate_run" && pwd)"
gate_order="$RUN_ROOT/gate-default-order"
gate_where="$RUN_ROOT/gate-default-where"
cat > "$gate_run/test/lint.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf 'lint\n' >> "$gate_order"
printf 'lint %s\n' "\$PWD" >> "$gate_where"
SH
cat > "$gate_run/test/integration.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf 'integration\n' >> "$gate_order"
printf 'integration %s\n' "\$PWD" >> "$gate_where"
SH
chmod +x "$gate_run/test/lint.sh" "$gate_run/test/integration.sh"
git -C "$gate_run" init -q
git -C "$gate_run" add -A
git -C "$gate_run" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: default gate fixture'
printf 'uncommitted while the gate runs\n' > "$gate_run/scratch.txt"
: > "$gate_order"
: > "$gate_where"
gate_default_out="$("$gate_run/test/gate.sh" 2>&1)"
equal "the no-argument gate runs lint and then the suite, in that order" \
  "$(printf 'lint\nintegration')" "$(<"$gate_order")"
# WHERE they ran is the claim, and the gate's own report cannot witness it: that
# line prints the SOURCE path whatever directory the gates were run from.
gate_lint_where="$(awk '$1 == "lint" { print $2; exit }' "$gate_where")"
gate_suite_where="$(awk '$1 == "integration" { print $2; exit }' "$gate_where")"
equal "and runs both of them from one and the same directory" \
  "$gate_lint_where" "$gate_suite_where"
if [ -n "$gate_lint_where" ] && [ "$gate_lint_where" != "$gate_run" ]; then
  pass "and that directory is the copy, not the tree it was copied from"
else
  fail "and that directory is the copy, not the tree it was copied from" \
    "the gates ran in [$gate_lint_where], the source is [$gate_run]"
fi
contains "the gate names the tree it copied" "$gate_default_out" "$gate_run"
contains "an uncommitted tree is announced as one" \
  "$gate_default_out" "unsettled"
contains "a green gate says which gates were green" \
  "$gate_default_out" "passed lint and the integration suite"

# THE GATE IS THE ONE FILE A TEAMMATE'S SAVE CAN STILL CORRUPT. Bash reads a
# script while it runs it, so an edit landing mid-run is read from a stale byte
# offset and executed as whatever now sits there. Every other file under test
# runs from the snapshot, which nobody edits; this one runs from the live tree
# and sits in one place for the length of a whole suite. It has already happened
# — a run died on `dest: unbound variable` at a line holding no such name — and
# the diagnosis is only believable if the harness can produce it on demand.
#
# The edit is made BY the stand-in lint, which is the moment a teammate's save
# would land and needs no barrier to arrange: the gate is inside its own suite
# call and blocked on that process, so there is nothing here to synchronise and
# nothing that can deadlock a mandatory run. What replaces the file is a file of
# nothing but a line that fails under `set -u`, long enough that WHEREVER bash
# resumes reading it lands inside one — so a surviving run is the property being
# claimed and not a lucky offset.
gate_edit="$RUN_ROOT/gate-midrun"
mkdir -p "$gate_edit/test"
cp "$ROOT/test/gate.sh" "$gate_edit/test/gate.sh"
gate_edit_lines=$(( ($(wc -c < "$ROOT/test/gate.sh") / 8) + 64 ))
cat > "$gate_edit/test/lint.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Generate the whole replacement in one producer. The old yes-to-head pipeline
# gave yes a routine SIGPIPE once head had enough lines, and whether that
# diagnostic leaked was pipe-buffer timing, not gate self-read evidence.
awk 'BEGIN { for (i = 0; i < $gate_edit_lines; i++) print "\"\$dest\"" }' > "$gate_edit/test/gate.sh"
SH
cat > "$gate_edit/test/integration.sh" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
exit 0
SH
chmod +x "$gate_edit/test/lint.sh" "$gate_edit/test/integration.sh" \
  "$gate_edit/test/gate.sh"
git -C "$gate_edit" init -q
git -C "$gate_edit" add -A
git -C "$gate_edit" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: mid-run edit fixture'
gate_edit_rc=0
gate_edit_err="$("$gate_edit/test/gate.sh" 2>&1 >/dev/null)" || gate_edit_rc=$?
equal "an edit landing mid-run cannot corrupt the running gate" \
  "0 " "$gate_edit_rc $gate_edit_err"
# The replacement really was in place while the run was still going, so a green
# above is the gate surviving it rather than the edit never arriving.
if [ "$(head -n 1 "$gate_edit/test/gate.sh")" = '"$dest"' ]; then
  pass "and the edit really did land on the file the run was reading"
else
  fail "and the edit really did land on the file the run was reading" \
    "the fixture gate still starts [$(head -n 1 "$gate_edit/test/gate.sh")]"
fi

# A failed gate owes the verdict's evidence AND that evidence's deletion path.
cat > "$gate_run/test/integration.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf 'integration\n' >> "$gate_order"
exit 3
SH
chmod +x "$gate_run/test/integration.sh"
gate_fail_rc=0
gate_fail_out="$("$gate_run/test/gate.sh" 2>&1)" || gate_fail_rc=$?
equal "a failing suite is the gate's own exit status" "3" "$gate_fail_rc"
contains "a failed gate keeps the snapshot that produced the verdict" \
  "$gate_fail_out" "kept for reading"
gate_kept="$(printf '%s\n' "$gate_fail_out" | awk '/^  \// { print $1; exit }')"
if [ -n "$gate_kept" ] && [ -d "$gate_kept" ]; then
  pass "the kept snapshot is really there to read"
else
  fail "the kept snapshot is really there to read" \
    "the reported path [$gate_kept] is not a directory"
fi
# A deletion path is only a deletion path if it deletes THIS artifact. The
# command is taken from the message and run, and the snapshot has to be gone.
gate_removal="$(printf '%s\n' "$gate_fail_out" |
  sed -n 's/^gate: nothing removes it but you:  //p')"
contains "and says exactly how that snapshot dies" "$gate_removal" "rm -rf "
if [ -n "$gate_removal" ] && [ -d "$gate_kept" ]; then
  eval "$gate_removal"
  if [ -e "$gate_kept" ]; then
    fail "and that command is the one that ends it" \
      "[$gate_removal] left $gate_kept behind"
  else
    pass "and that command is the one that ends it"
  fi
else
  fail "and that command is the one that ends it" \
    "no removal command was printed for $gate_kept"
fi

# THE WIRING, not a restatement of it: both mandatory entry points are run
# against a tree they would not own and must refuse before doing any work.
gate_wire="$RUN_ROOT/gate-wiring"
mkdir -p "$gate_wire/test"
cp "$ROOT/test/gate.sh" "$ROOT/test/lint.sh" "$ROOT/test/integration.sh" \
  "$gate_wire/test/"
printf 'gate wiring fixture\n' > "$gate_wire/README"
git -C "$gate_wire" init -q
git -C "$gate_wire" add -A
git -C "$gate_wire" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: gate wiring fixture'
printf 'edited while the gate was starting\n' >> "$gate_wire/README"
refuses "lint refuses a tree it would not own" \
  "would not own the tree it is testing" "$gate_wire/test/lint.sh"
refuses "the mandatory suite refuses a tree it would not own" \
  "would not own the tree it is testing" "$gate_wire/test/integration.sh"

# The focused role instrument is mandatory here and independently selectable so
# mutation calibration can run the exact AC that must turn red.
"$ROOT/test/role-briefs.sh"

# THE SAME TREE THIS RUN STARTED AGAINST, OR NO VERDICT. A source edit landing
# mid-run is not caught by either read — bash has already executed whatever it
# read — so this cannot make such a run safe. It can only stop the number below
# from being quoted as a fact about a tree that no longer exists, which is the
# form the last one took.
tree_moved=0
"$ROOT/test/gate.sh" --assert-unmoved "$TREE_AT_START" || tree_moved=1

summary_printed=1
printf '\n%s checks in %ss\n' "$checks" "$SECONDS"
if [ "$tree_moved" -eq 1 ]; then
  printf 'THE SOURCE TREE MOVED DURING THIS RUN, so the count above is not a\n'
  printf 'verdict on any tree. The refusal above says what changed.\n'
fi
# Reported apart from both columns and folded into neither: an unknown is a
# submission this run could not verify, which is neither a pass nor a fail.
# Green with unknowns above zero means the coverage held while something missed
# the compressed clock's budget, which is worth knowing before the number gets
# quoted anywhere.
if [ -s "$RUN_ROOT/unknowns" ]; then
  printf '%s setup submission(s) gang could not verify, re-established: %s\n' \
    "$(wc -l < "$RUN_ROOT/unknowns" | tr -d ' ')" \
    "$(tr '\n' ' ' < "$RUN_ROOT/unknowns")"
  printf 'Something missed the compressed clock budget above. This run\n'
  printf 'measured no cause and names none.\n'
fi
[ "$fails" -eq 0 ] && [ "$tree_moved" -eq 0 ]
