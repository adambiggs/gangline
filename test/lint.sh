#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Parse and lint every shell file from one canonical file list. With --fast,
# as the local gate and pre-push run it, only the files changed against
# origin/main are shellchecked.
set -euo pipefail

cd "$(dirname "$0")/.."
: "${GANG_TEST_PATH_SHIM_GUARD:=$(pwd -P)/test/path-shim-guard.sh}"
export GANG_TEST_PATH_SHIM_GUARD

fast=0
case "${1:-}" in
  '') ;;
  --fast)
    [ "$#" -eq 1 ] || { echo "lint: --fast takes no other arguments" >&2; exit 2; }
    fast=1
    ;;
  *) echo "lint: unknown argument '$1'" >&2; exit 2 ;;
esac

# Mandatory tests consume state, not wall time. A fake clock may hand code any
# timestamp it needs, but executable test code may not sleep, poll, or exercise
# timeout behaviour. Real harness probes are operator commands, not this suite.
# Include executable CI helpers as well as test/: hiding a slow test beside its
# workflow does not make it any less part of the suite. The e2e lane drives real
# harnesses, whose time is its subject, and the gate bounds the run it launches.
wall_time_exempt="test/e2e.sh test/gate.sh"
timing_hits=""
for f in test/*.sh .github/workflows/*.sh; do
  [ -f "$f" ] || continue
  case " $wall_time_exempt " in *" $f "*) continue ;; esac
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

# ONE PROCESS PER FILE, NEVER THE SET. shellcheck holds a whole invocation's
# input at once and its memory cost grows faster than that input does, so the
# set costs far more than the sum of its files. Per-file analysis bounds the
# peak at the largest single file, which keeps this gate runnable under a
# memory cap:
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
  # Changed since this branch left origin/main, committed or not. Without that
  # ref there is nothing to compare against, so every file is linted.
  if base="$(git merge-base origin/main HEAD 2>/dev/null)"; then
    changed=" $( { git diff --name-only "$base"; git ls-files --others --exclude-standard; } | tr '\n' ' ') "
    kept=""
    for f in $files; do
      case "$changed" in *" $f "*) kept="$kept $f" ;; esac
    done
    files="$kept"
  fi
fi
for f in $files; do
  lint_one "$f"
done
