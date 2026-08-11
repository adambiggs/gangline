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

# WHAT COUNTS AS THIS TREE MUST NOT DEPEND ON WHO ASKED, and the way to get that
# is to fix the configuration once rather than to chase the channels that can
# change it. The suite exports a private GIT_CONFIG_GLOBAL and a private
# XDG_CONFIG_HOME partway through its own setup, so a check inheriting either
# would answer one question before those lines and a different one after, and
# could report movement that never happened. A denylist loses that race by
# construction: GIT_CONFIG_GLOBAL, GIT_CONFIG_SYSTEM, the numbered
# GIT_CONFIG_COUNT triples, XDG_CONFIG_HOME and HOME are five doors to the same
# room, and GIT_CONFIG_PARAMETERS overrides even a pinned file.
#
# So every git call in this file — reads and snapshot writes alike — runs
# against one stated configuration: the repository's own. .git/config,
# .gitignore and .git/info/exclude decide what this tree is; nothing outside it
# does. An operator's global excludes therefore do not hide a file from this
# gate, which is the deliberate cost: what the gate copies and what the gate
# calls settled are then the same set, in every environment, for every caller.
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null
unset GIT_CONFIG_COUNT GIT_CONFIG_NOSYSTEM GIT_CONFIG_PARAMETERS

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"

# WHAT THIS CHECK IS NOT ABLE TO SEE, stated because a claim of ownership that
# quietly excludes cases is worse than no claim. A file the repository itself
# ignores is outside this tree by definition, so an ignored collar can change
# what the LIVE checkout does while this reads settled — the snapshot is
# unaffected, since it does not carry that file either. And the snapshot is a
# copy of a working tree, not of a repository: it is a fresh single-commit
# history in the destination's default object format, so nothing that reads the
# source's refs, reflog, object format or an operation's metadata is equivalent
# inside it.
#
# What it refuses to trust rather than merely inherit: `status.showUntrackedFiles`,
# a submodule ignore rule, an index instructed not to look at a file, and an
# operation left half-finished with a commit still to come.
subtree_identity() { # the object name of exactly the bytes this gate copies
  local prefix
  prefix="$(git -C "$ROOT" rev-parse --show-prefix 2>/dev/null)" || prefix=""
  if [ -n "$prefix" ]; then
    git -C "$ROOT" rev-parse "HEAD:${prefix%/}" 2>/dev/null || printf 'no-commit'
  else
    git -C "$ROOT" rev-parse 'HEAD^{tree}' 2>/dev/null || printf 'no-commit'
  fi
}

index_conceals() { # 0 = the index is under standing orders not to look
  local tags
  tags="$(git -C "$ROOT" ls-files -v -- . 2>/dev/null)" || return 1
  # A lowercase tag is assume-unchanged; S is skip-worktree. Both are standing
  # instructions to report a file without reading it, and a sparse checkout
  # leaves tracked paths absent from the working tree entirely.
  #
  # A here-string, not a pipe: `grep -q` stops at the first match, and under
  # `set -o pipefail` the SIGPIPE that kills the writer of a large index turns a
  # successful match into a failed pipeline — so the check would disappear on
  # exactly the trees big enough to need it.
  LC_ALL=C grep -q '^[a-zS]' <<<"$tags"
}

operation_in_progress() { # prints the operation's name, 0 = one is under way
  local dir name
  dir="$(git -C "$ROOT" rev-parse --git-dir 2>/dev/null)" || return 1
  case "$dir" in /*) ;; *) dir="$ROOT/$dir" ;; esac
  for name in MERGE_HEAD CHERRY_PICK_HEAD REVERT_HEAD rebase-merge rebase-apply; do
    if [ -e "$dir/$name" ]; then printf '%s' "$name"; return 0; fi
  done
  return 1
}

tree_identity() { # prints one line; 0 = settled, 1 = unsettled, 2 = cannot tell
  local status head operation
  git -C "$ROOT" rev-parse --show-toplevel >/dev/null 2>&1 || {
    printf 'unverifiable (not a git checkout)\n'
    return 2
  }
  # Scoped to ROOT with a pathspec: where ROOT is a subdirectory of a larger
  # repository, an unrelated edit elsewhere in that repository is not movement
  # in the tree this gate copies.
  # core.fsmonitor answers from a daemon's cache, and a healthy monitor that has
  # not noticed a write answers "nothing changed" for a modified tree. The
  # pathspec below is what actually forces git to stat the bytes here — measured,
  # not assumed — and the pin says so out loud rather than leaving the guarantee
  # resting on a side effect of scoping.
  status="$(git -C "$ROOT" -c core.fsmonitor=false status --porcelain \
    --untracked-files=normal --ignore-submodules=none -- . 2>/dev/null)" || {
    printf 'unverifiable (git status failed)\n'
    return 2
  }
  git -C "$ROOT" ls-files -v -- . >/dev/null 2>&1 || {
    printf 'unverifiable (git ls-files failed)\n'
    return 2
  }
  # The identity of a SUBTREE is its own tree object, not the containing
  # repository's HEAD: a commit that touches only a sibling moves HEAD without
  # moving one byte this gate would copy, and voiding a run for that is the same
  # false verdict in the other direction.
  head="$(subtree_identity)"
  # A verdict resting on a standing order is a promise, not a reading.
  # The bit proves only that git was TOLD not to look. It is not evidence that
  # anything changed, so it belongs with the readings that could not be taken.
  if index_conceals; then
    printf 'unverifiable (the index is told not to look at some files)\n'
    return 2
  fi
  # Settled bytes under a half-finished operation are settled for one more
  # moment: the commit that ends it moves HEAD, and the snapshot carries none of
  # that state anyway.
  if operation="$(operation_in_progress)"; then
    printf 'unsettled %s (%s is still in progress)\n' "$head" "$operation"
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

# A relative symlink is the one thing a copy cannot carry faithfully: its text
# survives, and the text resolves against the copy's parent. An ABSOLUTE link is
# safe — it names the same referent from anywhere — and a link that resolves
# inside the tree is safe, because the copy carries the referent too.
link_escapes() { # $1 = path relative to ROOT; 0 = the copy would read elsewhere
  local target here rest part missing
  target="$(readlink "$ROOT/$1")" || return 0
  # An absolute target names the same path read from either tree.
  case "$target" in /*) return 1 ;; esac
  here="$(cd -P "$(dirname "$ROOT/$1")" 2>/dev/null && pwd)" || return 0
  # A RELATIVE TARGET RESOLVES AGAINST A DIFFERENT PARENT IN THE COPY, so where
  # it lands has to be worked out rather than assumed. Walk the target's
  # directory a component at a time: one that exists is resolved physically, so
  # a symlink on the way cannot hide the answer, and once one is missing nothing
  # under it exists either and the rest is joined by name. A `..` past that
  # point would move the answer with nothing left to check it against, so it
  # refuses instead of guessing.
  #
  # Dangling is not the same as harmless. `../missing/file` reads nothing here
  # and can read a real file beside the destination, which is bytes the source
  # never had — so what matters is where the link points, not whether the
  # source end of it happens to exist.
  rest="$(dirname "$target")" missing=0
  while [ -n "$rest" ]; do
    part="${rest%%/*}"
    if [ "$part" = "$rest" ]; then rest=; else rest="${rest#*/}"; fi
    case "$part" in ''|.) continue ;; esac
    if [ "$missing" -eq 0 ] && [ -d "$here/$part" ]; then
      here="$(cd -P "$here/$part" 2>/dev/null && pwd)" || return 0
      here="${here%/}"
      continue
    fi
    missing=1
    case "$part" in ..) return 0 ;; esac
    here="$here/$part"
  done
  case "$here" in "$ROOT"|"$ROOT"/*) return 1 ;; esac
  return 0
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
    if [ -L "$ROOT/$f" ]; then
      if link_escapes "$f"; then
        printf '%s\n' \
          "gate: $ROOT/$f is a relative symlink pointing out of the tree." \
          "      Its text is carried unchanged, so in the copy it resolves" \
          "      somewhere else and the suite would read different bytes" \
          "      through the same name. That needs a deliberate decision." >&2
        return 1
      fi
      printf '%s\0' "$f" >> "$1"
    elif [ -f "$ROOT/$f" ]; then
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
      # Through files, not command substitution: a target ending in a newline is
      # a real target, and $( ) would compare it equal to one that does not.
      readlink "$ROOT/$f" > "$1.link-a" 2>/dev/null || : > "$1.link-a"
      readlink "$2/$f" > "$1.link-b" 2>/dev/null || : > "$1.link-b"
      cmp -s "$1.link-a" "$1.link-b" \
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
# The two listings name the same files in different orders — the source's is
# tracked-then-untracked, the snapshot's is index order — so they are compared as
# sets. NUL-terminated throughout, because a newline in a filename is a filename,
# not a separator; a `sort` that cannot do that is refused rather than worked
# around, since the alternative silently stops comparing what it claims to.
same_set() { # $1, $2 = NUL-separated listings
  printf 'a\0' | LC_ALL=C sort -z >/dev/null 2>&1 || {
    printf '%s\n' \
      "gate: this sort cannot read NUL-separated input, so the snapshot's" \
      "      contents cannot be compared with the tree's." >&2
    return 1
  }
  LC_ALL=C sort -z < "$1" > "$1.sorted"
  LC_ALL=C sort -z < "$2" > "$2.sorted"
  cmp -s "$1.sorted" "$2.sorted"
}

snap_git() {
  git "$@"
}

snapshot_into() { # $1 = destination directory, $2 = scratch directory
  local dest="$1" work="$2" drift phys
  # AN OCCUPIED DESTINATION IS NOT A SNAPSHOT OF ANYTHING. Whatever is already
  # there survives the overlay and is committed alongside the copy, so the tree
  # under test would be this tree plus somebody else's leftovers — including a
  # file whose deletion here is exactly what was meant to be tested.
  if { [ -e "$dest" ] || [ -L "$dest" ]; } \
     && { [ ! -d "$dest" ] || [ -L "$dest" ] \
          || [ -n "$(ls -A "$dest" 2>/dev/null)" ]; }; then
    printf '%s\n' \
      "gate: $dest already holds something." \
      "      A snapshot is the tree and nothing else, so name a destination" \
      "      that does not exist or is empty." >&2
    return 1
  fi
  # Decided BEFORE the directory is created, so a refusal leaves nothing new in
  # the tree it just refused to copy.
  phys="$(cd -P "$(dirname "$dest")" 2>/dev/null && pwd)/$(basename "$dest")" \
    || return 1
  case "$phys" in
    "$ROOT"|"$ROOT"/*)
      printf '%s\n' \
        "gate: $phys is inside $ROOT." \
        "      That would be copying the tree into itself; name a destination" \
        "      outside the tree." >&2
      return 1 ;;
  esac
  mkdir -p "$dest"
  # A tree the index is told not to read cannot be copied faithfully: a sparse
  # checkout leaves tracked paths absent from the working tree, and the copy
  # would drop them silently while its own HEAD claims to be the whole tree.
  if index_conceals; then
    printf '%s\n' \
      "gate: the index of $ROOT is under standing orders not to look at some" \
      "      files — assume-unchanged, skip-worktree, or a sparse checkout." \
      "      A copy of this working tree would silently be missing them." >&2
    return 1
  fi
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
  # -f so the index holds exactly what was copied: a file this repository's own
  # .gitignore names is still a file the copy carries, and the comparison below
  # is only exact if the index agrees.
  { snap_git -c init.defaultBranch=gate init -q "$dest" \
    && snap_git -C "$dest" config user.name 'Gangline gate snapshot' \
    && snap_git -C "$dest" config user.email 'gate@gangline.invalid' \
    && snap_git -C "$dest" config commit.gpgsign false \
    && snap_git -C "$dest" add -A -f \
    && snap_git -C "$dest" ls-files -z > "$work/snapped" \
    && same_set "$work/list" "$work/snapped" \
    && snap_git -C "$dest" commit -q --no-verify -m 'gate: working-tree snapshot'
  } || {
    # The destination was empty when this began, so anything the copy did not
    # put there arrived while it was running. Checked here rather than only up
    # front, because a check taken once at the start is a promise about a
    # directory another process can still reach.
    if [ -s "$work/snapped" ] && ! same_set "$work/list" "$work/snapped"; then
      printf '%s\n' \
        "gate: $dest holds something the copy did not put there, so committing" \
        "      it would test this tree plus somebody else's bytes." >&2
      return 1
    fi
    printf '%s\n' \
      "gate: could not commit the snapshot in $dest, so the executable there" \
      "      would be measured against a HEAD that does not hold its bytes." >&2
    return 1
  }
}

# BASH READS A SCRIPT WHILE IT RUNS IT, so an edit that lands during a run is
# read from a stale byte offset and executed as whatever now sits there. This
# is the only file in the gate that runs from the live tree — lint and the
# suite run from the snapshot, which nobody edits — and it sits in one place
# for the length of a whole suite. That is what makes it the one file a
# teammate's save can corrupt mid-run, which it has: a run once died on
# `dest: unbound variable` at a line holding no such name.
#
# A function body is one command, so it is read whole before any of it runs.
# Everything that can wait goes inside, and the call is the last line in the
# file, so once the run reaches the suite there is nothing left to read.
main() {
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
      # Quoted, because an unquoted path with a space in it is a command that
      # deletes something else.
      printf 'gate: nothing removes it but you:  rm -rf %q\n' "$WORK" >&2
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
}

# The exit shares this line, so it is read with the call rather than after it:
# a return to the top level would send bash back to the file for one more
# command, at an offset a run-long edit has already moved.
main "$@"; exit
