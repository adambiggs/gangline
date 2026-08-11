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

# bash -n reads one script per call; shellcheck takes the set.
for f in $files; do bash -n "$f"; done
shellcheck -S warning $files
