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

# WHAT THIS CHECK IS NOT ABLE TO SEE, stated because a claim of ownership that
# quietly excludes cases is worse than no claim. A file git ignores is outside
# this tree by definition, so an ignored collar can change what the LIVE
# checkout does while this reads settled — the snapshot is unaffected, since it
# does not carry that file either. And a relative symlink pointing outside the
# tree keeps its text in the copy and therefore resolves somewhere else; nothing
# in this repository is such a link, and one would have to be handled
# deliberately rather than assumed equivalent.
#
# What it does refuse to inherit: an operator's `status.showUntrackedFiles=no`
# would hide an untracked collar, a submodule ignore rule would hide moved
# submodule bytes, and the index can be told to lie about a file outright. Those
# are stated rather than trusted, and the reads never resolve a caller's
# environment (see the unset above).
tree_identity() { # prints one line; 0 = settled, 1 = unsettled, 2 = cannot tell
  local status head concealed
  git -C "$ROOT" rev-parse --show-toplevel >/dev/null 2>&1 || {
    printf 'unverifiable (not a git checkout)\n'
    return 2
  }
  # Scoped to ROOT with a pathspec: where ROOT is a subdirectory of a larger
  # repository, an unrelated edit elsewhere in that repository is not movement
  # in the tree this gate copies.
  status="$(git -C "$ROOT" status --porcelain --untracked-files=normal \
    --ignore-submodules=none -- . 2>/dev/null)" || {
    printf 'unverifiable (git status failed)\n'
    return 2
  }
  concealed="$(git -C "$ROOT" ls-files -v -- . 2>/dev/null)" || {
    printf 'unverifiable (git ls-files failed)\n'
    return 2
  }
  head="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null)" || head=no-commit
  # assume-unchanged (a lowercase tag) and skip-worktree (S) are standing
  # instructions to report a file as unchanged without looking at it. A verdict
  # resting on one is a promise, not a reading.
  if printf '%s\n' "$concealed" | grep -q '^[a-zS]'; then
    printf 'unsettled %s (the index is told not to look at some files)\n' "$head"
    return 1
  fi
  if [ -n "$status" ]; then
    printf 'unsettled %s\n' "$head"
    return 1
  fi
  printf 'settled %s %s\n' "$head" "$ROOT"
}

owned_refusal() { # $1 = the reading taken, so the refusal says which one it was
  printf '%s\n' \
    "gate: this run would not own the tree it is testing." \
    "      reading: $1" \
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

unverifiable_refusal() { # $1 = the reading that could not be taken
  printf '%s\n' \
    "gate: this run cannot tell whether it owns the tree it is testing:" \
    "      $1" \
    "      An unknown is not a pass. Nothing here can say whether the source" \
    "      moved under the run, so no verdict over it would be about a tree." >&2
}

# Tracked, staged and untracked-not-ignored, which together are what the next
# commit would carry. `ls-files --cached` still names a file deleted from the
# working tree, so the list is filtered by what is actually there: a deletion
# has to reach the snapshot as a deletion.
list_tree() { # $1 = destination list file, NUL separated
  local raw="$1.raw" f
  git -C "$ROOT" ls-files -z --cached --others --exclude-standard > "$raw" || return 1
  : > "$1"
  while IFS= read -r -d '' f; do
    # Symlink first: a link to a directory answers -d as well.
    if [ -L "$ROOT/$f" ] || [ -f "$ROOT/$f" ]; then
      printf '%s\0' "$f" >> "$1"
    elif [ -e "$ROOT/$f" ]; then
      # A submodule's gitlink is a directory in this listing, and it would reach
      # the byte comparison as one: cmp answers "Is a directory" and the tree
      # gets blamed for moving — a true refusal for a false reason, which is the
      # failure class this file exists to remove. Say what was actually found.
      printf '%s\n' \
        "gate: $ROOT/$f is neither a regular file nor a symlink, so this gate" \
        "      cannot copy it. A submodule in the tree needs a deliberate" \
        "      decision, not a silent omission." >&2
      return 1
    fi
  done < "$raw"
}

# THE COPY ITSELF IS A WINDOW, and this is what makes an edit that lands inside
# it an unusable snapshot rather than a mixture of two trees quietly tested as
# one. A symlink is compared by its target, because a relative link resolves
# against a different directory inside the copy and following it would report
# drift that never happened. The executable bit is compared as well: it is the
# only mode git records, and `chmod +x` moves a tree without moving a byte.
copy_drift() { # $1 = list file, $2 = snapshot root; prints what moved, if anything
  local f second="$1.second"
  list_tree "$second" || { printf 'the tree stopped being readable'; return 0; }
  cmp -s "$1" "$second" || { printf 'the set of files changed'; return 0; }
  while IFS= read -r -d '' f; do
    if [ -L "$ROOT/$f" ]; then
      [ "$(readlink "$ROOT/$f")" = "$(readlink "$2/$f" 2>/dev/null)" ] \
        || { printf '%s' "the symlink $f changed"; return 0; }
    else
      cmp -s "$ROOT/$f" "$2/$f" || { printf '%s' "$f changed"; return 0; }
      if [ -x "$ROOT/$f" ]; then
        [ -x "$2/$f" ] || { printf '%s' "the mode of $f changed"; return 0; }
      else
        [ ! -x "$2/$f" ] || { printf '%s' "the mode of $f changed"; return 0; }
      fi
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
  # AN OCCUPIED DESTINATION IS NOT A SNAPSHOT OF ANYTHING. Whatever is already
  # there survives the overlay and is committed alongside the copy, so the tree
  # under test would be this tree plus somebody else's leftovers — including a
  # file whose deletion here is exactly what was meant to be tested.
  if [ -e "$dest" ] && { [ ! -d "$dest" ] || [ -n "$(ls -A "$dest" 2>/dev/null)" ]; }; then
    printf '%s\n' \
      "gate: $dest already holds something." \
      "      A snapshot is the tree and nothing else, so name a destination" \
      "      that does not exist or is empty." >&2
    return 1
  fi
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
  list_tree "$work/list" || return 1
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
    case "$identity" in
      0) ;;
      1) owned_refusal "$line"; exit 1 ;;
      *) unverifiable_refusal "$line"; exit 1 ;;
    esac
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
# A FAILED RUN KEEPS ITS EVIDENCE, AND SAYS HOW THAT EVIDENCE DIES. Nothing
# collects these later — no gate run touches another run's snapshot — so the
# deletion is the reader's, stated as the exact command rather than left to be
# discovered as accumulated copies of the source under TMPDIR.
cleanup() {
  if [ "$keep" -eq 1 ]; then
    printf '\ngate: the snapshot that produced this verdict is kept for reading:\n' >&2
    printf '  %s\n' "$SNAP" >&2
    printf 'gate: nothing removes it but you:  rm -rf %s\n' "$WORK" >&2
  else
    rm -rf -- "$WORK"
  fi
}
trap cleanup EXIT HUP INT TERM

snapshot_into "$SNAP" "$WORK"

# Read the same way the ownership check reads, so an untracked-only tree is not
# announced as settled by a diagnostic that only looks at tracked files.
source_state="$(tree_identity)" || true
printf 'gate: testing a snapshot of %s\n' "$ROOT"
printf 'gate: source tree %s\n' "$source_state"

rc=0
( cd "$SNAP" && ./test/lint.sh && ./test/integration.sh ) || rc=$?
if [ "$rc" -ne 0 ]; then
  keep=1
  printf '\ngate: REFUSED (status %s)\n' "$rc" >&2
  exit "$rc"
fi
printf '\ngate: the snapshot passed lint and the integration suite.\n'
