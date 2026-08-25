#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Parse and lint every shell file from one canonical file list.
set -euo pipefail

cd "$(dirname "$0")/.."

fast=0
case "${1:-}" in
  '') ;;
  --fast)
    [ "$#" -eq 1 ] || { echo "lint: --fast takes no other arguments" >&2; exit 2; }
    fast=1
    ;;
  *) echo "lint: unknown argument '$1'" >&2; exit 2 ;;
esac

# A gate that reads the tree it is judging must own that tree, or its verdict
# belongs to whatever the tree happened to be at each read. test/gate.sh runs
# the whole gate against a private snapshot of the working tree, which is how
# this suite becomes runnable before a commit rather than only at pre-push.
test/gate.sh --assert-owned >/dev/null

python3 --version >/dev/null 2>&1 || {
  echo "lint: python3 cannot run here, so the source-guard dataflow check cannot run" >&2
  exit 1
}
python3 test/source-guards.py --discover test
if [ "$fast" -eq 0 ]; then
  test/source-guards-fixtures.sh
fi

# THE E2E LANE IS THE ONE FILE ALLOWED TO SPEND WALL TIME, and this is the check
# that keeps that exemption honest. It drives a real claude-code TUI, so it
# cannot be written against a fake clock — but the rule below was never about
# all test code, it was about the MANDATORY suite. One isolated daily/dispatch
# workflow runs the lane specifically because real harness time is its subject;
# the exemption still holds only while the lane stays outside the gate, hooks,
# and push/PR workflows.
#
# WHAT THIS CAN AND CANNOT SEE. It reads the ordinary automatic paths and looks
# for the lane's name, exempting only the named workflow after proving that
# workflow exposes exactly schedule and workflow_dispatch. A call assembled
# from a variable or reached through a helper this list does not name would
# pass it. The check is a tripwire on the ordinary way in, not a proof of
# unreachability; the rule it guards is stated in CONTRIBUTING.md.
E2E_LANE=test/e2e.sh
E2E_WORKFLOW=.github/workflows/e2e.yml
if [ -f "$E2E_LANE" ]; then
  auto_hits=""
  for f in test/gate.sh test/smoke.sh test/integration.sh \
    .github/workflows/*.yml .github/workflows/*.yaml \
    .github/workflows/*.sh .githooks/*; do
    [ -f "$f" ] || continue
    [ "$f" = "$E2E_WORKFLOW" ] && continue
    grep -Hn 'e2e\.sh' "$f" >/dev/null 2>&1 || continue
    auto_hits="${auto_hits}${auto_hits:+
}$(grep -Hn 'e2e\.sh' "$f")"
  done
  if [ -n "$auto_hits" ]; then
    printf '%s\n' \
      "lint: $E2E_LANE may run only in $E2E_WORKFLOW outside explicit local use, and another automatic path now names it:" \
      "$auto_hits" \
      "Take it back out; the lane must stay outside the gate, hooks, and push/PR workflows." >&2
    exit 1
  fi

  if [ -f "$E2E_WORKFLOW" ]; then
    if ! grep -Eq '^[[:space:]]+run: test/e2e\.sh[[:space:]]*$' "$E2E_WORKFLOW"; then
      echo "lint: $E2E_WORKFLOW does not run $E2E_LANE directly" >&2
      exit 1
    fi
    e2e_triggers="$(awk '
      /^on:[[:space:]]*$/ { in_on = 1; saw_on = 1; next }
      in_on && /^[^[:space:]#]/ { in_on = 0 }
      !in_on { next }
      /^[[:space:]]*$/ || /^[[:space:]]*#/ { next }
      /^  [[:alnum:]_-]+:[[:space:]]*$/ {
        trigger = $0
        sub(/^  /, "", trigger)
        sub(/:[[:space:]]*$/, "", trigger)
        print trigger
        next
      }
      /^    / { next }
      { print "<unreadable line " FNR ">" }
      END { if (!saw_on) print "<no readable on block>" }
    ' "$E2E_WORKFLOW" | sort)"
    expected_triggers="$(printf '%s\n' schedule workflow_dispatch)"
    if [ "$e2e_triggers" != "$expected_triggers" ]; then
      printf '%s\n' \
        "lint: $E2E_WORKFLOW must expose exactly schedule and workflow_dispatch; found:" \
        "${e2e_triggers:-<none>}" >&2
      exit 1
    fi
  fi
fi

# Mandatory tests consume state, not wall time. A fake clock may hand code any
# timestamp it needs, but executable test code may not sleep, poll, or exercise
# timeout behaviour. Real harness probes are operator commands, not this suite.
# Include executable CI helpers as well as test/: hiding a slow test beside its
# workflow does not make it any less part of the suite.
timing_hits=""
for f in test/*.sh .github/workflows/*.sh; do
  [ -f "$f" ] || continue
  [ "$f" = "$E2E_LANE" ] && continue
  hits="$(awk '
    /^[[:space:]]*#/ { next }
    {
      if ($0 ~ /(^|[;&|()[:space:]])(sl[e]ep|time[o]ut)([;&|()[:space:]]|$)/ ||
          $0 ~ /(wait[_]for|absence[_]window)[[:space:]]*[(]/) {
        print FILENAME ":" FNR ":" $0
      }
    }
  ' "$f")"
  [ -z "$hits" ] || timing_hits="${timing_hits}${timing_hits:+
}${hits}"
done
if [ -n "$timing_hits" ]; then
  printf '%s\n' "lint: mandatory tests may not consume wall time:" "$timing_hits" >&2
  exit 1
fi

# A FIXTURE SHELL MUST NOT READ THE OPERATOR'S SYSTEM BASH STARTUP FILES.
# --rcfile and --init-file replace ~/.bashrc and leave /etc/bash.bashrc, where
# Debian installs a command_not_found_handle that runs a Python program against
# a multi-megabyte apt database. Every envelope this suite delivers is an
# unrunnable command, so that handler lands on the Enter path of every
# submission gang verifies and spends most of the compressed clock's budget
# before anything has gone wrong. A fixture shell therefore takes its rc file
# through ENV in posix mode, where bash reads no system rc at all, or takes no
# rc with --norc. Checked here rather than left to a line each new fixture
# remembers to copy, because the fixtures that starved were the ones that
# forgot the line.
#
# The test is textual, so it is written against a shell WORD and never against
# the line: a safe flag mentioned in a trailing comment, or sitting inside a
# longer token or an assignment's value, is not the flag the launcher runs
# under. The calibration below is what keeps that true, since a predicate this
# shape is one careless anchor away from admitting exactly the line it exists
# to refuse.
hermetic_awk='
  FILENAME ~ /lint\.sh$/ { next }       # the checker has to name what it refuses
  /^[[:space:]]*#/ { next }
  { line = $0
    hash = index(line, " #")
    if (hash > 0) line = substr(line, 1, hash - 1) }
  line ~ /--rcfile|--init-file/ { print FILENAME ":" FNR ":" $0; next }
  line ~ /GANG_LAUNCH=/ && line ~ /(^|[^[:alnum:]_-])bash([^[:alnum:]_-]|$)/ &&
    line !~ /[[:space:]]--(norc|posix)([[:space:]"'"'"']|$)/ {
      print FILENAME ":" FNR ":" $0 }
'

# A CHECKER PROVES ITSELF BEFORE IT JUDGES, and proves both directions: a
# predicate that refused everything would pass a bad-lines-only calibration
# while making the guard useless, so the safe lines are asserted too.
hermetic_cal="$(mktemp -d "${TMPDIR:-/tmp}/gangline-lint.XXXXXX")"
cat > "$hermetic_cal/bad" <<'CAL'
GANG_LAUNCH="bash -i" # --norc
GANG_LAUNCH="env DECOY=--posix bash -i"
GANG_LAUNCH="bash -i --norcnot"
GANG_LAUNCH="sh -c 'exec bash --rcfile /x/rc' fixture"
GANG_LAUNCH="bash --init-file /x/rc"
CAL
cat > "$hermetic_cal/safe" <<'CAL'
GANG_LAUNCH="PS1='x' bash --norc"
GANG_LAUNCH="sh -c 'ENV=/x/rc exec bash --posix' fixture"
GANG_LAUNCH="python3 '/x/argv-witness.py' prefix fresh"
CAL
cal_bad="$(awk "$hermetic_awk" "$hermetic_cal/bad" | wc -l | tr -d ' ')"
cal_safe="$(awk "$hermetic_awk" "$hermetic_cal/safe")"
rm -rf -- "$hermetic_cal"
if [ "$cal_bad" != 5 ] || [ -n "$cal_safe" ]; then
  printf '%s\n' \
    "lint: the hermetic-fixture-shell check does not hold its own calibration, so its verdict about the tree means nothing." \
    "rejected $cal_bad of 5 known-bad launch lines; wrongly rejected: ${cal_safe:-none}" >&2
  exit 1
fi

shell_leaks="$(awk "$hermetic_awk" test/*.sh collars/*.sh)"
if [ -n "$shell_leaks" ]; then
  printf '%s\n' \
    "lint: a fixture shell may not read /etc/bash.bashrc — carry its rc with ENV=<file> bash --posix, or take none with bash --norc:" \
    "$shell_leaks" >&2
  exit 1
fi

# A COMMENT INSIDE AN EXPANDED HEREDOC IS NOT A COMMENT YET. These fixtures
# write collars and hook scripts through `cat <<TAG`, and with the delimiter
# unquoted the shell expands the body before anything is a comment at all: a
# backtick pair in explanatory prose becomes command substitution, runs its
# contents, and drops the result into the file being written. That is how a
# `$$` in a comment about pids came to run a pid as a command every time the
# spool fixture generated its collar, and the words around it never reached
# the collar. The prose reads fine in the source, which is what makes it worth
# a check rather than a habit — quote the delimiter, or write the prose without
# backticks.
#
# Only backticks are refused, and only where the body is expanded. `$(...)` in
# an expanded body is how these fixtures deliberately interpolate the run's own
# paths, so refusing it would refuse the mechanism; a backtick has no such use
# here, since every real substitution in this tree is already written `$(...)`.
heredoc_awk='
  FILENAME ~ /lint\.sh$/ { next }       # the checker has to name what it refuses
  { line = $0
    probe = line
    gsub(/<<</, "   ", probe)           # a here-string opens no body
    if (inbody) {
      t = line
      if (dash) sub(/^\t+/, "", t)
      if (t == delim) { inbody = 0; next }
      if (expand) {
        scrub = line
        gsub(/\\./, "", scrub)
        if (index(scrub, "`") > 0) print FILENAME ":" FNR ":" $0
      }
      next
    }
    if (match(probe, /<<-?[[:space:]]*[A-Za-z_'"'"'"\\][A-Za-z0-9_'"'"'"\\]*/)) {
      tok = substr(probe, RSTART, RLENGTH)
      sub(/^<</, "", tok)
      dash = 0
      if (substr(tok, 1, 1) == "-") { dash = 1; sub(/^-/, "", tok) }
      sub(/^[[:space:]]*/, "", tok)
      expand = (tok ~ /^['"'"'"\\]/) ? 0 : 1
      gsub(/['"'"'"\\]/, "", tok)
      delim = tok
      inbody = 1
    }
  }
'

# A CHECKER PROVES ITSELF BEFORE IT JUDGES. Both directions again: the safe
# calibration carries the quoted delimiter, the escaped backtick, and the
# here-string, because each of those is a way this predicate could have been
# written to refuse the tree it is supposed to pass.
heredoc_cal="$(mktemp -d "${TMPDIR:-/tmp}/gangline-lint.XXXXXX")"
cat > "$heredoc_cal/bad" <<'CAL'
cat > out <<SH
# a `$$` in prose is a substitution here
echo "`hostname`"
SH
cat > out2 <<-SH
	# indented body, `date` still runs
SH
CAL
cat > "$heredoc_cal/safe" <<'CAL'
cat > out <<'SH'
# a `$$` in prose is prose, because the delimiter is quoted
SH
cat > out2 <<SH
# an escaped \`$$\` reaches the file as written
echo "\$(date)"
SH
grep -q x <<<"a `backtick outside any body` b"
# a `backtick` in an ordinary comment
CAL
cal_bad="$(awk "$heredoc_awk" "$heredoc_cal/bad" | wc -l | tr -d ' ')"
cal_safe="$(awk "$heredoc_awk" "$heredoc_cal/safe")"
rm -rf -- "$heredoc_cal"
if [ "$cal_bad" != 3 ] || [ -n "$cal_safe" ]; then
  printf '%s\n' \
    "lint: the expanded-heredoc check does not hold its own calibration, so its verdict about the tree means nothing." \
    "rejected $cal_bad of 3 known-bad body lines; wrongly rejected: ${cal_safe:-none}" >&2
  exit 1
fi

heredoc_ticks="$(awk "$heredoc_awk" test/*.sh collars/*.sh bin/gang)"
if [ -n "$heredoc_ticks" ]; then
  printf '%s\n' \
    "lint: a backtick in an expanded heredoc body runs as a command when the file is written — quote the delimiter, or drop the backticks:" \
    "$heredoc_ticks" >&2
  exit 1
fi

# .githooks is globbed rather than listed: hooksPath points the whole directory
# at git, so a hook added later is a shell file this repo runs, and it should
# not also need an edit here to be read.
files="bin/gang install.sh collars/*.sh statusline/*.sh test/*.sh .githooks/*"
for f in .github/workflows/*.sh; do
  [ -f "$f" ] || continue
  files="$files $f"
done

# A gate that cannot run must not report that it passed. And resolving on PATH
# is not the ability to run: a version-manager shim resolves and then refuses
# ("No version is set"), which read as a code problem the first time it blocked
# a push here. Ask the tool to execute, not its name to exist.
shellcheck --version >/dev/null 2>&1 || {
  echo "lint: shellcheck cannot run here (not installed, or a shim with nothing behind it), so the lint CI runs cannot run" >&2
  exit 1
}

# ONE PROCESS PER FILE, NEVER THE SET. shellcheck holds a whole invocation's
# input at once and its memory cost grows faster than that input does, so the
# set costs far more than the sum of its files. Handed this repo as one
# invocation it reached 6.1 GB, and on 2026-08-12 the kernel OOM killer took it
# twice on an 11.6 GB host — the mandatory gate was itself the largest single
# allocation on the machine. Per file the peak is whatever the largest single
# file costs, which is what makes this gate runnable under a memory cap:
#
#   systemd-run --user --scope -p MemoryMax=2G -p MemorySwapMax=0 -- test/gate.sh
#
# Nothing here reads across files, so a file at a time is the same verdict.
# Keeping it that way is a size question, and test/integration.sh is the file
# that answers it: it is split into sourced parts for exactly this reason.
lint_one() {
  local f="$1"
  bash -n "$f"
  shellcheck -S warning "$f"
}

if [ "$fast" -eq 1 ]; then
  # Pre-push keeps production, hook, lint-entry, and smoke files on the
  # canonical list's one-process-at-a-time memory discipline. Test fixtures and
  # the source-guard checker's own fixtures are left to full lint in CI.
  for f in $files; do
    case "$f" in
      test/lint.sh|test/smoke.sh) ;;
      test/*) continue ;;
    esac
    lint_one "$f"
  done
else
  for f in $files; do
    lint_one "$f"
  done
fi
