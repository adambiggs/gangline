#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Parse and lint every shell file from one canonical file list.
set -euo pipefail

cd "$(dirname "$0")/.."

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
test/source-guards-fixtures.sh

# Mandatory tests consume state, not wall time. A fake clock may hand code any
# timestamp it needs, but executable test code may not sleep, poll, or exercise
# timeout behaviour. Real harness probes are operator commands, not this suite.
# Include executable CI helpers as well as test/: hiding a slow test beside its
# workflow does not make it any less part of the suite.
timing_hits=""
for f in test/*.sh .github/workflows/*.sh; do
  [ -f "$f" ] || continue
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
for f in $files; do
  bash -n "$f"
  shellcheck -S warning "$f"
done
