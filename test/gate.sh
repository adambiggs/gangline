#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# THE MANDATORY GATE, RUN AGAINST A TREE THE RUN OWNS.
#
# Two failures wrote this file, and they are the same failure.
#
# The suite could not pass on a dirty tree at all. One mandatory assertion
# requires a `gang roster` stderr capture to be exactly empty, and the live
# executable deliberately warns on stderr whenever its own bytes diverge from
# HEAD. Both halves are right, and together they meant the complete gate could
# only ever run AFTER a commit, at pre-push, on the clean worktree that hook
# builds. A day of work was landed on focused checks and lint because the full
# gate was not runnable on the tree that held the work.
#
# And a run that reads the tree it is testing does not own it. Bash reads a
# script incrementally, `gang` re-reads collars and roles at hitch time, and the
# installer hashes the tree: an edit made while the suite is running changes
# what executes mid-run. That has already cost this team a failure that belonged
# to the editor rather than to the code, and a review of files that moved under
# the reviewer.
#
# So the gate copies the working tree — tracked, staged and untracked alike —
# into a private snapshot, commits it there, and runs from that copy. The
# snapshot is what a reviewer already did by hand with a clean clone, except
# that it carries the uncommitted work, which is the whole point of running
# before committing. Because the snapshot's own HEAD holds those exact bytes,
# `bin/gang` is clean by construction inside it and the dirty-execution warning
# never fires — no assertion is relaxed and no suite-only environment switch
# exists to relax one later.
#
#   test/gate.sh                    snapshot this working tree and run the gate
#   test/gate.sh --snapshot DIR     build that snapshot in DIR and stop
#   test/gate.sh --assert-owned     print this tree's identity, or refuse it
#   test/gate.sh --assert-unmoved X refuse if the identity is no longer X
#
# The last two are what test/lint.sh and test/integration.sh call on themselves.
# They are a backstop, not the mechanism: they prove a tree was already moving
# before a run, and that it moved across one, but only the snapshot can stop an
# edit from landing mid-run.
set -euo pipefail

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_PREFIX

# WHAT COUNTS AS THIS TREE MUST NOT DEPEND ON WHO ASKED. The suite exports a
# private GIT_CONFIG_GLOBAL partway through its own setup, so a caller reading
# the tree before and after that point would be reading two different
# definitions of "ignored" and could report movement that never happened. These
# reads therefore always resolve the operator's real configuration, which is
# also the configuration bin/gang's own dirty-execution warning resolves — the
# behaviour this check exists to predict. Writes into the snapshot go the other
# way and are pinned hermetically; see snap_git.
unset GIT_CONFIG_GLOBAL GIT_CONFIG_SYSTEM GIT_CONFIG_COUNT GIT_CONFIG_NOSYSTEM

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"

# An operator's own git configuration decides what `status` is willing to see.
# `status.showUntrackedFiles=no` would leave an untracked collar invisible here
# and let this check report a tree nobody owns as owned, so the reporting mode
# is stated rather than inherited. Ignore rules are inherited on purpose: a file
# the operator's excludes hide is not part of this tree.
tree_identity() { # prints one line: the identity, or the reason there is none
  local top status head
  top="$(git -C "$ROOT" rev-parse --show-toplevel 2>/dev/null)" || {
    printf 'unverifiable (not a git checkout)\n'
    return 0
  }
  status="$(git -C "$ROOT" status --porcelain --untracked-files=normal)" || {
    printf 'unverifiable (git status failed)\n'
    return 0
  }
  head="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null)" || head=no-commit
  if [ -n "$status" ]; then
    printf 'moving %s\n' "$head"
    return 1
  fi
  printf 'settled %s %s\n' "$head" "$top"
}

owned_refusal() {
  printf '%s\n' \
    "gate: this run would not own the tree it is testing." \
    "      $ROOT has uncommitted changes, and the suite reads bin/gang," \
    "      collars/, roles/ and its own script while it runs, so an edit landing" \
    "      mid-run changes what executes. The dirty executable also warns on" \
    "      stderr, which one mandatory assertion reads as failure." \
    "" \
    "      Run the full gate against an isolated snapshot of exactly these" \
    "      bytes instead:" \
    "" \
    "          test/gate.sh" \
    "" >&2
}

# Tracked, staged and untracked-not-ignored, which together are what the next
# commit would carry. `ls-files --cached` still names a file deleted from the
# working tree, so the list is filtered by what is actually there: a deletion
# has to reach the snapshot as a deletion.
list_tree() { # $1 = destination list file, NUL separated
  local raw="$1.raw" f
  git -C "$ROOT" ls-files -z --cached --others --exclude-standard > "$raw"
  : > "$1"
  while IFS= read -r -d '' f; do
    if [ -e "$ROOT/$f" ] || [ -L "$ROOT/$f" ]; then
      printf '%s\0' "$f" >> "$1"
    fi
  done < "$raw"
}

# THE COPY ITSELF IS A WINDOW, and this is what makes an edit that lands inside
# it an unusable snapshot rather than a mixture of two trees quietly tested as
# one. A symlink is compared by its target, because a relative link resolves
# against a different directory inside the copy and following it would report
# drift that never happened.
copy_drift() { # $1 = list file, $2 = snapshot root; prints what moved, if anything
  local f second="$1.second"
  list_tree "$second"
  cmp -s "$1" "$second" || { printf 'the set of files changed'; return 0; }
  while IFS= read -r -d '' f; do
    if [ -L "$ROOT/$f" ]; then
      [ "$(readlink "$ROOT/$f")" = "$(readlink "$2/$f" 2>/dev/null)" ] \
        || { printf '%s' "the symlink $f changed"; return 0; }
    else
      cmp -s "$ROOT/$f" "$2/$f" || { printf '%s' "$f changed"; return 0; }
    fi
  done < "$1"
}

# Committing the snapshot is what makes bin/gang clean against its own HEAD, so
# the dirty-execution warning has nothing to report and the stderr assertions
# read what they were written to read. The identity is local to the snapshot
# because the fixtures that build commits need one, and the operator's own git
# configuration is excluded so a global hooks path, commit template or signing
# requirement cannot reach in here.
snap_git() {
  GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git "$@"
}

snapshot_into() { # $1 = destination directory, $2 = scratch directory
  local dest="$1" work="$2" drift phys
  mkdir -p "$dest"
  phys="$(cd -P "$dest" && pwd)" || return 1
  # A destination inside the tree is copied into itself. The drift check would
  # catch it and blame the working tree for moving, which is a true refusal for
  # a false reason — the failure this whole file exists to stop.
  case "$phys" in
    "$ROOT"|"$ROOT"/*)
      printf '%s\n' \
        "gate: $phys is inside $ROOT." \
        "      That would be copying the tree into itself; name a destination" \
        "      outside the tree." >&2
      return 1 ;;
  esac
  list_tree "$work/list"
  [ -s "$work/list" ] || {
    echo "gate: $ROOT holds no files to test" >&2
    return 1
  }
  # `set -e` is suspended for the whole body of a function whose caller tests
  # its status, and --snapshot does exactly that. Every step that can fail is
  # therefore checked here by hand: a half-copied tree reported as a snapshot is
  # the failure this file exists to prevent, not one it may introduce.
  tar -C "$ROOT" --null -T "$work/list" -cf - | tar -C "$dest" -xf - || {
    printf '%s\n' "gate: could not copy $ROOT into $dest" >&2
    return 1
  }
  drift="$(copy_drift "$work/list" "$dest")"
  if [ -n "$drift" ]; then
    printf '%s\n' \
      "gate: the working tree moved while it was being copied ($drift)," \
      "      so this snapshot is a mixture of two trees and no verdict over it" \
      "      would be about either one. Stop editing $ROOT and run again." >&2
    return 1
  fi
  { snap_git -c init.defaultBranch=gate init -q "$dest" \
    && snap_git -C "$dest" config user.name 'Gangline gate snapshot' \
    && snap_git -C "$dest" config user.email 'gate@gangline.invalid' \
    && snap_git -C "$dest" config commit.gpgsign false \
    && snap_git -C "$dest" add -A \
    && snap_git -C "$dest" commit -q --no-verify -m 'gate: working-tree snapshot'
  } || {
    printf '%s\n' \
      "gate: could not commit the snapshot in $dest, so the executable there" \
      "      would be measured against a HEAD that does not hold its bytes." >&2
    return 1
  }
}

case "${1:-}" in
  --assert-owned)
    [ $# -eq 1 ] || { echo "gate: --assert-owned takes no arguments" >&2; exit 2; }
    identity=0
    line="$(tree_identity)" || identity=$?
    [ "$identity" -eq 0 ] || { owned_refusal; exit 1; }
    printf '%s\n' "$line"
    exit 0 ;;
  --assert-unmoved)
    [ $# -eq 2 ] || { echo "gate: --assert-unmoved takes one identity" >&2; exit 2; }
    now="$(tree_identity)" || true
    [ "$now" = "$2" ] || {
      printf '%s\n' \
        "gate: THE SOURCE TREE MOVED DURING THIS RUN. It was [$2] at the start" \
        "      and [$now] now, so the checks were not all taken against one" \
        "      tree and no count over them is a verdict on either. Run" \
        "      test/gate.sh, which copies the working tree first and cannot be" \
        "      edited out from under itself." >&2
      exit 1
    }
    exit 0 ;;
  --snapshot)
    [ $# -eq 2 ] || { echo "gate: --snapshot takes one directory" >&2; exit 2; }
    scratch="$(mktemp -d "${TMPDIR:-/tmp}/gangline-gate-snap.XXXXXX")"
    rc=0
    snapshot_into "$2" "$scratch" || rc=$?
    rm -rf -- "$scratch"
    exit "$rc" ;;
  -h|--help)
    printf '%s\n' \
      'usage: test/gate.sh [--snapshot DIR | --assert-owned | --assert-unmoved IDENTITY]' \
      '  (no argument)     snapshot this working tree and run lint + integration there' \
      '  --snapshot DIR    build that snapshot in DIR and stop' \
      '  --assert-owned    print this tree'"'"'s identity, or refuse a tree that is moving' \
      '  --assert-unmoved  refuse if the identity is no longer the one given'
    exit 0 ;;
  '') ;;
  *) echo "gate: unknown argument '$1'" >&2; exit 2 ;;
esac

WORK="$(mktemp -d "${TMPDIR:-/tmp}/gangline-gate.XXXXXX")"
SNAP="$WORK/tree"
keep=0
cleanup() {
  if [ "$keep" -eq 1 ]; then
    printf '\ngate: the snapshot that produced this verdict is kept at\n  %s\n' \
      "$SNAP" >&2
  else
    rm -rf -- "$WORK"
  fi
}
trap cleanup EXIT HUP INT TERM

snapshot_into "$SNAP" "$WORK"

source_head="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo 'no commit')"
source_state=settled
git -C "$ROOT" diff --quiet HEAD 2>/dev/null || source_state=uncommitted
printf 'gate: testing a snapshot of %s\n' "$ROOT"
printf 'gate: source HEAD %s, working tree %s\n' "$source_head" "$source_state"

rc=0
( cd "$SNAP" && ./test/lint.sh && ./test/integration.sh ) || rc=$?
if [ "$rc" -ne 0 ]; then
  keep=1
  printf '\ngate: REFUSED (status %s)\n' "$rc" >&2
  exit "$rc"
fi
printf '\ngate: the snapshot passed lint and the integration suite.\n'
