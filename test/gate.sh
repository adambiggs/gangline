#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# THE MANDATORY GATE, RUN AGAINST A TREE THE RUN OWNS.
#
# The suite could not pass on a dirty tree at all. One mandatory assertion
# requires a `gang roster` stderr capture to be exactly empty, and the live
# executable deliberately warns on stderr whenever its own bytes diverge from
# HEAD. Both halves are right, and together they meant the complete gate could
# only ever run AFTER a commit, at pre-push, on the clean worktree that hook
# builds.
#
# And a run that reads the tree it is testing does not own it. Bash reads a
# script incrementally, `gang` re-reads collars and roles at hitch time, and the
# installer hashes the tree: an edit made while the suite is running changes
# what executes mid-run.
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

# THE ORDINARY GATE OWNS THE HOST'S HEAVY-TEST LOCK. Keeping acquisition here
# means callers cannot accidentally omit the descriptor rule. `flock -o`
# retains the lock in its small parent while closing the lock fd in this script,
# so disposable tmux servers spawned by integration cannot inherit the open
# file description and keep the lock alive after a killed gate. Helper modes do
# not run the heavy suite and need no lock. The marker deliberately remains in
# the process tree: nested gate fixtures are already serialized by this parent.
#
# A NONBLOCKING ATTEMPT SEPARATES ACQUISITION FROM WAITING. Its distinct status
# belongs only to the probe, which runs no gate step and therefore cannot
# collide with a step's exit status. Before joining the queue, the waiter reads
# the owner's record from the locked inode and corroborates it against a second
# kernel lock. An empty, changing, or uncorroborated record is reported as
# unknown rather than naming a predecessor.
GATE_HEAVY_LOCK=/tmp/gangline-heavy.lock
GATE_HEAVY_OWNER_LOCK="${GATE_HEAVY_LOCK}.owner"
GATE_LOCK_OWNED=0

gate_release_heavy_lock() {
  [ "$GATE_LOCK_OWNED" -eq 1 ] || return 0
  # Clear while both locks still belong to this run. A waiter can therefore
  # never corroborate the record after the run that wrote it has released.
  : > "$GATE_HEAVY_LOCK"
  flock -u "$GATE_HEAVY_OWNER_FD" 2>/dev/null || true
  exec {GATE_HEAVY_OWNER_FD}>&-
  flock -u "$GATE_HEAVY_LOCK_FD" 2>/dev/null || true
  exec {GATE_HEAVY_LOCK_FD}>&-
  GATE_LOCK_OWNED=0
}

gate_close_inherited_locks() {
  [ "$GATE_LOCK_OWNED" -eq 1 ] || return 0
  # Closing a duplicate leaves the gate shell's open file descriptions locked;
  # unlocking here would release them for every process that shares them.
  exec {GATE_HEAVY_OWNER_FD}>&-
  exec {GATE_HEAVY_LOCK_FD}>&-
  GATE_LOCK_OWNED=0
}

gate_report_lock_holder() {
  local before after verify_fd verify_rc=0
  local lock_pid_field lock_started_field lock_cwd_field lock_lease_field
  local lock_pid="" lock_started="" lock_cwd="" lock_age=unknown lock_now
  IFS= read -r before < "$GATE_HEAVY_LOCK" || before=""
  exec {verify_fd}>> "$GATE_HEAVY_OWNER_LOCK"
  flock -E 201 -n "$verify_fd" || verify_rc=$?
  if [ "$verify_rc" -eq 0 ]; then
    flock -u "$verify_fd" 2>/dev/null || true
  fi
  exec {verify_fd}>&-
  IFS= read -r after < "$GATE_HEAVY_LOCK" || after=""

  if [ "$verify_rc" -eq 201 ] && [ -n "$before" ] && [ "$before" = "$after" ]; then
    IFS=$'\t' read -r lock_pid_field lock_started_field lock_cwd_field lock_lease_field \
      <<<"$before"
    case "${lock_pid_field:-}" in pid=[0-9]*) lock_pid="${lock_pid_field#pid=}" ;; esac
    case "${lock_started_field:-}" in started=[0-9]*) lock_started="${lock_started_field#started=}" ;; esac
    case "${lock_cwd_field:-}" in cwd=?*) lock_cwd="${lock_cwd_field#cwd=}" ;; esac
    case "${lock_lease_field:-}" in lease=?*) ;; *) lock_pid="" ;; esac
  fi
  if ! [[ "$lock_pid" =~ ^[0-9]+$ && "$lock_started" =~ ^[0-9]+$ ]] \
      || [ -z "$lock_cwd" ]; then
    lock_pid=unknown
    lock_cwd=unknown
  else
    lock_now="$(date +%s)"
    if [ "$lock_now" -ge "$lock_started" ]; then
      lock_age=$((lock_now - lock_started))
    fi
  fi
  printf 'gate: waiting on %s (pid=%s cwd=%s held_for=%ss)\n' \
    "$GATE_HEAVY_LOCK" "$lock_pid" "$lock_cwd" "$lock_age" >&2
}

if [ $# -eq 0 ] && [ "${_GANGLINE_GATE_LOCKED:-}" != 1 ]; then
  command -v flock >/dev/null 2>&1 \
    || { echo "gate: flock is required to serialize the mandatory suite" >&2; exit 1; }
  exec {GATE_HEAVY_LOCK_FD}>> "$GATE_HEAVY_LOCK"
  lock_rc=0
  flock -E 200 -n "$GATE_HEAVY_LOCK_FD" || lock_rc=$?
  if [ "$lock_rc" -eq 200 ]; then
    gate_report_lock_holder
    flock "$GATE_HEAVY_LOCK_FD"
  elif [ "$lock_rc" -ne 0 ]; then
    exit "$lock_rc"
  fi

  # Primary ownership makes any surviving owner lock an invariant violation,
  # not something to wait behind silently.
  : > "$GATE_HEAVY_LOCK"
  exec {GATE_HEAVY_OWNER_FD}>> "$GATE_HEAVY_OWNER_LOCK"
  owner_rc=0
  flock -E 201 -n "$GATE_HEAVY_OWNER_FD" || owner_rc=$?
  if [ "$owner_rc" -ne 0 ]; then
    echo "gate: the owner-verification lock survived without the heavy lock; refusing" >&2
    exit "$owner_rc"
  fi
  GATE_LOCK_OWNED=1
  trap gate_release_heavy_lock EXIT
  lock_owner_pid=$$
  if [ -r /proc/self/status ]; then
    lock_owner_pid="$(awk '/^NSpid:/ { print $2; exit }' /proc/self/status)"
    [ -n "$lock_owner_pid" ] || lock_owner_pid=$$
  fi
  lock_owner_cwd="$(cd -P "$(dirname "$0")/.." && pwd)"
  lock_started="$(date +%s)"
  lock_lease="${lock_owner_pid}-${lock_started}-${RANDOM}-${BASHPID}"
  printf 'pid=%s\tstarted=%s\tcwd=%q\tlease=%s\n' \
    "$lock_owner_pid" "$lock_started" "$lock_owner_cwd" "$lock_lease" \
    > "$GATE_HEAVY_LOCK"
  export _GANGLINE_GATE_LOCKED=1
fi

# WHAT COUNTS AS THIS TREE MUST NOT DEPEND ON WHO ASKED. The suite exports a
# private GIT_CONFIG_GLOBAL partway through its own setup, so a check inheriting
# it would answer one question before that line and a different one after, and
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

# WHAT THIS CHECK IS NOT ABLE TO SEE. A file the repository itself
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

# THE LAST LINE CARRIES THE VERDICT, because the exit status does not survive
# the ordinary invocation. `test/gate.sh 2>&1 | tail -30` is how this gets run
# whenever the full output will not fit the reader, and `$?` is then tail's —
# zero whatever the gate decided. What is left in the transcript is a plausible
# tail of suite output and a success status, with nothing in it marking the
# verdict as discarded. A reader who concludes green from the absence of a FAIL
# is reading a truncation.
#
# So the run ends on one line of a fixed shape that a reader can assert on by
# PRESENCE, printed on stdout because a pipe without `2>&1` still carries it.
# UNKNOWN is a third word rather than a refusal: a run killed before it decided
# has produced no verdict on the tree, and calling that a refusal claims
# evidence the run never collected.
gate_verdict() { # $1 = exit status, $2 = 1 if the gate reached a decision
  local word
  if [ "${2:-0}" -ne 1 ]; then
    word=UNKNOWN
  elif [ "$1" -eq 0 ]; then
    word=PASS
  else
    word=REFUSED
  fi
  printf 'gate: VERDICT %s (status %s)\n' "$word" "$1"
}

# EACH HEAVY STEP RUNS IN A SESSION THIS GATE CREATED. That gives teardown one
# exact process group: descendants remain inside it, while the flock parent and
# unrelated gates do not. Before any signal, both the recorded parent and the
# live session id are read and compared in a command that has already finished.
# A mismatch is a refusal to kill, never permission to widen the target.
gate_stop_step() { # $1 pid, $2 expected parent pid, $3 step name
  local pid="$1" expected_parent="$2" name="$3" actual_parent sid
  actual_parent="$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ' || true)"
  sid="$(ps -o sid= -p "$pid" 2>/dev/null | tr -d ' ' || true)"
  if [ "$actual_parent" != "$expected_parent" ] || [ "$sid" != "$pid" ]; then
    printf 'gate: refusing to kill %s: pid=%s ppid=%s sid=%s, expected ppid=%s sid=%s\n' \
      "$name" "$pid" "${actual_parent:-unknown}" "${sid:-unknown}" \
      "$expected_parent" "$pid" >&2
    return 1
  fi
  kill -TERM -- "-$pid" 2>/dev/null || true
  kill -KILL -- "-$pid" 2>/dev/null || true
}

gate_step_tree() { # $1 process-group id
  local pgid="$1"
  printf 'gate: PROCESS TREE (pid ppid pgid elapsed command)\n' >&2
  ps -eo pid=,ppid=,pgid=,etime=,args= 2>/dev/null \
    | awk -v group="$pgid" '$3 == group { print "  " $0 }' >&2
}

# Output is a lease, not a total-duration budget. Every complete line renews it.
# The file remains the exact trailing transcript used in a stall report and by
# the integration all-parts attestation below. A partial final line is retained
# at EOF, but a program that never completes that line has not made new terminal
# output visible and does not renew the lease.
gate_monitored_step() { # $1 name, $2 output, $3 live, $4 cwd, rest = argv
  local name="$1" output="$2" live="$3" cwd="$4"
  local fifo pidfile pid parent fd line read_rc rc
  shift 4
  fifo="$WORK/$name.fifo"
  pidfile="$WORK/$name.pid"
  mkfifo "$fifo"
  (
    cd "$cwd"
    # The gate shell, not a test process, owns both lock descriptions. Closing
    # them at this boundary prevents a tmux server or other descendant from
    # extending either lock beyond the gate's lifetime.
    gate_close_inherited_locks
    exec setsid "$@"
  ) > "$fifo" 2>&1 &
  pid=$!
  parent=$BASHPID
  printf '%s %s %s\n' "$pid" "$parent" "$name" > "$pidfile"
  exec {fd}< "$fifo"
  : > "$output"
  while :; do
    line=""
    read_rc=0
    IFS= read -r -t "$GATE_QUIET_SECONDS" -u "$fd" line || read_rc=$?
    if [ "$read_rc" -eq 0 ] || [ -n "$line" ]; then
      printf '%s\n' "$line" >> "$output"
      [ "$live" -eq 0 ] || printf '%s\n' "$line"
    fi
    if [ "$read_rc" -eq 0 ]; then
      continue
    fi
    if [ "$read_rc" -le 128 ]; then
      break
    fi

    printf '\ngate: STALLED: %s produced no output for %ss\n' \
      "$name" "$GATE_QUIET_SECONDS" >&2
    gate_step_tree "$pid"
    printf 'gate: LAST OUTPUT (%s, up to 30 lines)\n' "$name" >&2
    tail -n 30 "$output" >&2
    : > "$WORK/$name.stalled"
    if ! gate_stop_step "$pid" "$parent" "$name"; then
      exec {fd}<&-
      rm -f -- "$pidfile" "$fifo"
      return 125
    fi
    wait "$pid" 2>/dev/null || true
    exec {fd}<&-
    rm -f -- "$pidfile" "$fifo"
    return 124
  done
  exec {fd}<&-
  rc=0
  wait "$pid" || rc=$?
  rm -f -- "$pidfile" "$fifo"
  return "$rc"
}

gate_cancel_branch() { # $1 branch pid, $2 step names it may currently own
  local branch_pid="$1" step_name pidfile step_pid step_parent recorded_name branch_parent
  shift
  for step_name in "$@"; do
    pidfile="$WORK/$step_name.pid"
    [ -f "$pidfile" ] || continue
    read -r step_pid step_parent recorded_name < "$pidfile" || continue
    if gate_stop_step "$step_pid" "$step_parent" "$recorded_name"; then
      rm -f -- "$pidfile" "$WORK/$step_name.fifo"
    else
      # Ownership refusal is already loud. Do not turn it into an unbounded
      # wait in EXIT while this gate still holds the host lock.
      rm -f -- "$pidfile" "$WORK/$step_name.fifo"
    fi
  done
  if [ -n "$branch_pid" ] && kill -0 "$branch_pid" 2>/dev/null; then
    branch_parent="$(ps -o ppid= -p "$branch_pid" 2>/dev/null | tr -d ' ' || true)"
    if [ "$branch_parent" = "$BASHPID" ]; then
      kill -TERM "$branch_pid" 2>/dev/null || true
      kill -KILL "$branch_pid" 2>/dev/null || true
    else
      printf 'gate: refusing to kill monitor pid=%s ppid=%s, expected ppid=%s\n' \
        "$branch_pid" "${branch_parent:-unknown}" "$BASHPID" >&2
      return 1
    fi
  fi
}

gate_lint_branch() {
  local rc=0
  gate_close_inherited_locks
  gate_monitored_step lint "$WORK/lint.out" 0 "$SNAP" ./test/lint.sh || rc=$?
  printf '%s\n' "$rc" > "$WORK/lint.status"
  return "$rc"
}

gate_suite_branch() {
  local smoke_rc=0 integration_rc=0 rc
  gate_close_inherited_locks
  gate_monitored_step smoke "$WORK/smoke.out" 1 "$SNAP" ./test/smoke.sh || smoke_rc=$?
  printf '%s\n' "$smoke_rc" > "$WORK/smoke.status"
  if [ -e "$WORK/smoke.stalled" ]; then
    printf '%s\n' 125 > "$WORK/integration.status"
    return "$smoke_rc"
  fi
  gate_monitored_step integration "$WORK/integration.out" 1 "$SNAP" \
    env -u GANG_INTEGRATION_PARTS -u GANG_INTEGRATION_REQUIRE_ALL_PROBE \
    GANG_INTEGRATION_REQUIRE_ALL=1 ./test/integration.sh || integration_rc=$?
  printf '%s\n' "$integration_rc" > "$WORK/integration.status"
  rc="$smoke_rc"
  [ "$rc" -ne 0 ] || rc="$integration_rc"
  return "$rc"
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
# file, so once the run reaches the snapshot gates there is nothing left to read.
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
        '  (no argument)     snapshot this tree and run lint + smoke + integration there' \
        '  --snapshot DIR    build that snapshot in DIR and stop' \
        '  --assert-owned    print this tree'"'"'s identity, or refuse a tree that is moving' \
        '  --assert-unmoved  refuse if the identity is no longer the one given'
      exit 0 ;;
    '') ;;
    *)
      echo "gate: unknown argument '$1'" >&2
      # A refused invocation is a decision: the gate answered, and answered no.
      gate_verdict 2 1
      exit 2 ;;
  esac

  WORK="$(mktemp -d "${TMPDIR:-/tmp}/gangline-gate.XXXXXX")"
  SNAP="$WORK/tree"
  keep=0
  decided=0
  GATE_QUIET_SECONDS="${GANG_GATE_QUIET_SECONDS:-300}"
  if ! [[ "$GATE_QUIET_SECONDS" =~ ^[0-9]+([.][0-9]+)?$ ]] \
      || ! awk -v seconds="$GATE_QUIET_SECONDS" \
        'BEGIN { exit !(seconds > 0) }'; then
    printf 'gate: GANG_GATE_QUIET_SECONDS must be a positive number, got %q\n' \
      "$GATE_QUIET_SECONDS" >&2
    exit 2
  fi
  command -v setsid >/dev/null 2>&1 || {
    echo "gate: setsid is required to isolate each mandatory step" >&2
    exit 1
  }
  # A FAILED RUN KEEPS ITS EVIDENCE, AND SAYS HOW THAT EVIDENCE DIES. Nothing
  # collects these later — no gate run touches another run's snapshot — so the
  # deletion is the reader's, stated as the exact command rather than left to be
  # discovered as accumulated copies of the source under TMPDIR.
  lint_monitor_pid=""
  suite_monitor_pid=""
  cleanup() {
    local status=$? step_pidfile step_pid step_parent step_name
    for step_pidfile in "$WORK"/*.pid; do
      [ -f "$step_pidfile" ] || continue
      read -r step_pid step_parent step_name < "$step_pidfile" || continue
      if gate_stop_step "$step_pid" "$step_parent" "$step_name"; then
        wait "$step_pid" 2>/dev/null || true
      fi
      rm -f -- "$step_pidfile"
    done
    if [ -n "$lint_monitor_pid" ]; then
      if gate_cancel_branch "$lint_monitor_pid" lint; then
        wait "$lint_monitor_pid" 2>/dev/null || true
      fi
    fi
    if [ -n "$suite_monitor_pid" ]; then
      if gate_cancel_branch "$suite_monitor_pid" smoke integration; then
        wait "$suite_monitor_pid" 2>/dev/null || true
      fi
    fi
    if [ "$keep" -eq 1 ]; then
      printf '\ngate: the snapshot that produced this verdict is kept for reading:\n' >&2
      printf '  %s\n' "$SNAP" >&2
      # Quoted, because an unquoted path with a space in it is a command that
      # deletes something else.
      printf 'gate: nothing removes it but you:  rm -rf %q\n' "$WORK" >&2
    else
      rm -rf -- "$WORK"
    fi
    gate_release_heavy_lock
    gate_verdict "$status" "$decided"
  }
  # A SIGNAL ENDS THE GATE; IT DOES NOT ANNOTATE IT. One handler for the exit
  # and for the signals reads as if a signalled gate stops here, and it does
  # not: a bash signal handler returns to the interrupted flow. Bash holds the
  # handler until the foreground child returns, so what resumes after the
  # teardown is whatever follows the gates — the verdict itself. A signalled
  # run therefore deletes its own snapshot and then reads lint's output out of
  # it: `cat: .../lint.out: No such file or directory`, and no verdict line at
  # all. What a reader is left holding is the suite's own green count with
  # nothing after it saying the gate never decided.
  # The teardown is unchanged and still removes what this run created; what
  # changes is that nothing runs after it. The disposition is cleared first so a
  # second signal cannot re-enter this over the first.
  gate_on_signal() { # $1 = signal number, for the conventional 128 + signo
    trap - HUP INT TERM
    exit "$((128 + $1))"
  }
  trap cleanup EXIT
  trap 'gate_on_signal 1' HUP
  trap 'gate_on_signal 2' INT
  trap 'gate_on_signal 15' TERM

  snapshot_rc=0
  gate_monitored_step snapshot "$WORK/snapshot.out" 1 "$ROOT" \
    "$0" --snapshot "$SNAP" || snapshot_rc=$?
  if [ "$snapshot_rc" -ne 0 ]; then
    keep=1
    decided=1
    printf '\ngate: REFUSED (status %s)\n' "$snapshot_rc" >&2
    exit "$snapshot_rc"
  fi

  # Read the same way the ownership check reads, so an untracked-only tree is not
  # announced as settled by a diagnostic that only looks at tracked files.
  source_state="$(tree_identity)" || true
  printf 'gate: testing a snapshot of %s\n' "$ROOT"
  printf 'gate: source tree %s\n' "$source_state"

  # LINT OVERLAPS THE BEHAVIOURAL PATH. They read the same immutable snapshot
  # and write separate roots, so serialising them spends lint's entire runtime
  # without ordering evidence. Each branch is monitored independently; the
  # first stalled branch cancels its sibling before the verdict releases the
  # lock, while an ordinary failure still allows every mandatory step to run.
  lint_out="$WORK/lint.out"
  integration_out="$WORK/integration.out"
  if [ -n "${GANG_INTEGRATION_PARTS+x}" ]; then
    printf 'gate: ignoring GANG_INTEGRATION_PARTS=%q; mandatory gate runs every declared integration part.\n' \
      "$GANG_INTEGRATION_PARTS"
  fi
  gate_lint_branch &
  lint_monitor_pid=$!
  gate_suite_branch &
  suite_monitor_pid=$!
  wait -n "$lint_monitor_pid" "$suite_monitor_pid" 2>/dev/null || true
  if [ -e "$WORK/lint.stalled" ] || [ -e "$WORK/smoke.stalled" ] \
      || [ -e "$WORK/integration.stalled" ]; then
    gate_cancel_branch "$lint_monitor_pid" lint || true
    gate_cancel_branch "$suite_monitor_pid" smoke integration || true
  fi
  wait "$lint_monitor_pid" 2>/dev/null || true
  wait "$suite_monitor_pid" 2>/dev/null || true
  lint_monitor_pid=""
  suite_monitor_pid=""
  lint_rc="$(cat "$WORK/lint.status" 2>/dev/null || printf 125)"
  smoke_rc="$(cat "$WORK/smoke.status" 2>/dev/null || printf 125)"
  integration_rc="$(cat "$WORK/integration.status" 2>/dev/null || printf 125)"
  cat "$lint_out"
  if [ "$lint_rc" -eq 0 ] && [ "$smoke_rc" -eq 0 ] \
      && [ "$integration_rc" -eq 0 ] \
      && ! grep -Fx 'integration: every declared part ran' "$integration_out" >/dev/null; then
    integration_rc=1
    printf 'gate: integration did not attest that every declared part ran.\n' >&2
  fi
  rc="$lint_rc"
  [ "$rc" -ne 0 ] || rc="$smoke_rc"
  [ "$rc" -ne 0 ] || rc="$integration_rc"
  if [ "$rc" -ne 0 ]; then
    keep=1
    decided=1
    printf '\ngate: REFUSED (status %s)\n' "$rc" >&2
    exit "$rc"
  fi
  decided=1
  printf '\ngate: the snapshot passed lint, smoke, and the integration suite.\n'
}

# The exit shares this line, so it is read with the call rather than after it:
# a return to the top level would send bash back to the file for one more
# command, at an offset a run-long edit has already moved.
main "$@"; exit
