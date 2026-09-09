# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0

# An adoption runs from an ordinary client, where tmux 3.2a can take down the
# server if display-message targets the adopted pane.  Pane facts use the
# list/format reader; keep this structural guard so the dangerous form cannot
# return with a future refactor.
if rg -Fq "display-message -p -t \"\$id\" '#{pane_current_path}'" "$ROOT/bin/gang"; then
  fail "adoption never reads pane path through display-message" \
    "bin/gang targets pane_current_path through display-message"
else
  pass "adoption never reads pane path through display-message"
fi
# test/gate.sh itself: the tree it judges, the snapshot it takes, and the evidence a failure owes.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# Every no-argument gate below is a disposable fixture, never the mandatory
# gate. A focused integration run has no outer gate process to pass this marker
# down, so without it each fixture re-execs through the host-wide heavy lock and
# can outlive the focused parent as a waiter. The fixtures have private trees
# and state under RUN_ROOT; they need the nested-gate disposition, not a slot in
# the host-wide mandatory queue.
export _GANGLINE_GATE_LOCKED=1
GATE_FIXTURE_EVENT_CEILING="${GANG_TEST_GATE_EVENT_CEILING:-120}"
case "$GATE_FIXTURE_EVENT_CEILING" in
  0*|*[!0-9]*)
    fail "the gate fixture event ceiling is a positive whole number" \
      "GANG_TEST_GATE_EVENT_CEILING=[$GATE_FIXTURE_EVENT_CEILING]"
    GATE_FIXTURE_EVENT_CEILING=120
    ;;
esac

# CHILD OUTPUT IS THE EVENT; THIS CLOCK ONLY MAKES A MISSING EVENT LOUD. The
# gate fixtures below used short timed reads as if the timeout meant the child
# had finished. Under load that confused a slow child with a dead one, then a
# later liveness probe raced the rescue command. A blocking reader now owns the
# event, while this independent guard gives a broken fixture enough time to
# print a named failure inside CI instead of wedging the whole integration job.
# The sleeper has private stdio. `wait -n` leaves the losing child unreaped, so
# its PID cannot be reused before this function stops and joins it.
gate_fixture_event() { # $1 fd, $2 outcome variable, $3 value variable, $4 path prefix
  local fd="$1" outcome="$2" destination="$3" prefix="$4"
  local state="$prefix.state" eof="$prefix.eof"
  local reader ceiling finished value
  rm -f -- "$state" "$eof"
  (
    value=""
    if IFS= read -r -u "$fd" value; then
      printf '%s\n' "$value" > "$state"
    else
      : > "$eof"
    fi
  ) &
  reader=$!
  /bin/sleep "$GATE_FIXTURE_EVENT_CEILING" >/dev/null 2>&1 &
  ceiling=$!
  finished=""
  wait -n -p finished "$reader" "$ceiling" 2>/dev/null || true
  if [ "$finished" = "$reader" ]; then
    kill -KILL "$ceiling" 2>/dev/null || true
    wait "$ceiling" 2>/dev/null || true
    if [ -e "$state" ]; then
      printf -v "$outcome" '%s' event
      printf -v "$destination" '%s' "$(<"$state")"
    else
      printf -v "$outcome" '%s' eof
      printf -v "$destination" '%s' ""
    fi
  elif [ -e "$state" ]; then
    kill -KILL "$ceiling" 2>/dev/null || true
    wait "$ceiling" 2>/dev/null || true
    wait "$reader" 2>/dev/null || true
    printf -v "$outcome" '%s' event
    printf -v "$destination" '%s' "$(<"$state")"
  else
    kill -KILL "$reader" 2>/dev/null || true
    wait "$reader" 2>/dev/null || true
    printf -v "$outcome" '%s' deadline
    printf -v "$destination" '%s' ""
  fi
  rm -f -- "$state" "$eof"
}

# A DEADLINE ACTS ON ONE SESSION IT CREATED, WITHOUT A PRIOR LIVENESS PROBE.
# A process that finished before the identity read needs no rescue. Anything
# still present must have both the recorded parent and its own session id before
# the wrapper receives TERM and performs its child join.
gate_fixture_stop_session() { # $1 pid, $2 description
  local pid="$1" description="$2" actual_parent sid
  actual_parent="$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ' || true)"
  sid="$(ps -o sid= -p "$pid" 2>/dev/null | tr -d ' ' || true)"
  if [ -z "$actual_parent" ] && [ -z "$sid" ]; then
    return 0
  fi
  if [ "$actual_parent" = "$BASHPID" ] && [ "$sid" = "$pid" ]; then
    kill -TERM "$pid" 2>/dev/null || true
    return 0
  fi
  fail "$description remains scoped to its disposable session" \
    "pid=$pid ppid=${actual_parent:-unknown} sid=${sid:-unknown}"
  return 1
}

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
# that they all ran from inside the copy. Their order is deliberately not a
# claim: lint and integration read the same immutable snapshot and their private
# outputs cannot change each other's evidence, so serial order was only wall
# time and was wrong to preserve once it broke the mandatory ceiling.
gate_run="$RUN_ROOT/gate-default"
mkdir -p "$gate_run/test"
gate_run_lock="$RUN_ROOT/gate-default.lock"
sed "s|GATE_HEAVY_LOCK=/tmp/gangline-heavy.lock|GATE_HEAVY_LOCK=$gate_run_lock|" \
  "$ROOT/test/gate.sh" > "$gate_run/test/gate.sh"
chmod +x "$gate_run/test/gate.sh"
gate_run="$(cd -P "$gate_run" && pwd)"
gate_order="$RUN_ROOT/gate-default-order"
gate_where="$RUN_ROOT/gate-default-where"
gate_parts="$RUN_ROOT/gate-default-parts"
gate_overlap="$RUN_ROOT/gate-default-overlap"
gate_overlap_ready="$RUN_ROOT/gate-default-overlap-ready"
gate_overlap_release="$RUN_ROOT/gate-default-overlap-release"
mkfifo "$gate_overlap_ready" "$gate_overlap_release"
exec 7<> "$gate_overlap_ready"
exec 8<> "$gate_overlap_release"
cat > "$gate_run/test/lint.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf 'lint\n' >> "$gate_order"
printf 'lint %s\n' "\$PWD" >> "$gate_where"
if [ "\${GATE_PROVE_OVERLAP:-0}" = 1 ]; then
  printf 'lint-ready\n' >&7
  IFS= read -r -u 8
fi
exit "\${GATE_FAIL_LINT:-0}"
SH
cat > "$gate_run/test/integration.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf 'integration\n' >> "$gate_order"
printf 'integration %s\n' "\$PWD" >> "$gate_where"
printf 'parts=<%s> require=<%s>\n' "\${GANG_INTEGRATION_PARTS-}" \
  "\${GANG_INTEGRATION_REQUIRE_ALL-}" > "$gate_parts"
if [ "\${GATE_PROVE_OVERLAP:-0}" = 1 ]; then
  IFS= read -r overlap_state <&7
  [ "\$overlap_state" = lint-ready ]
  : > "$gate_overlap"
  printf 'release\n' >&8
fi
printf 'integration: every declared part ran\n'
SH
cat > "$gate_run/test/smoke.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf 'smoke\n' >> "$gate_order"
printf 'smoke %s\n' "\$PWD" >> "$gate_where"
SH
chmod +x "$gate_run/test/lint.sh" "$gate_run/test/smoke.sh" \
  "$gate_run/test/integration.sh"
git -C "$gate_run" init -q
git -C "$gate_run" add -A
git -C "$gate_run" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: default gate fixture'
printf 'uncommitted while the gate runs\n' > "$gate_run/scratch.txt"
: > "$gate_order"
: > "$gate_where"
gate_flock_bin="$RUN_ROOT/gate-flock-bin"
gate_flock_args="$RUN_ROOT/gate-flock-args"
mkdir -p "$gate_flock_bin"
cat > "$gate_flock_bin/flock" <<SH
#!/bin/sh
case "\$*" in
  '-E 200 -n '*) printf 'primary-probe\n' >> "$gate_flock_args" ;;
  '-E 201 -n '*) printf 'owner-probe\n' >> "$gate_flock_args" ;;
  '-u '*) printf 'unlock\n' >> "$gate_flock_args" ;;
  *) printf 'unexpected <%s>\n' "\$*" >> "$gate_flock_args"; exit 91 ;;
esac
exit 0
SH
chmod +x "$gate_flock_bin/flock"
# The fixture marker has to cover a focused integration run too, which has no
# outer test/gate.sh from which to inherit it. A flock that would otherwise
# record its invocation proves the stand-in gate did not join the host-wide
# mandatory queue. The next check deliberately unsets the marker and therefore
# still proves that an ordinary user-facing gate acquires that queue's lock.
gate_fixture_flock="$RUN_ROOT/gate-fixture-flock"
gate_fixture_flock_calls="$RUN_ROOT/gate-fixture-flock-calls"
export GATE_FIXTURE_FLOCK_CALLS="$gate_fixture_flock_calls"
cat > "$gate_fixture_flock" <<'SH'
#!/bin/sh
printf '%s\n' "$@" >> "$GATE_FIXTURE_FLOCK_CALLS"
exit 93
SH
chmod +x "$gate_fixture_flock"
mkdir -p "$RUN_ROOT/gate-fixture-flock-bin"
ln -s "$gate_fixture_flock" "$RUN_ROOT/gate-fixture-flock-bin/flock"
gate_fixture_out="$(PATH="$RUN_ROOT/gate-fixture-flock-bin:$PATH" \
  "$gate_run/test/gate.sh" 2>&1)"
if [ ! -e "$gate_fixture_flock_calls" ]; then
  pass "a gate fixture inherits the nested-gate marker outside an ordinary gate"
else
  fail "a gate fixture inherits the nested-gate marker outside an ordinary gate" \
    "fixture flock was invoked with [$(cat "$gate_fixture_flock_calls")]"
fi
contains "the marked fixture still runs its declared suite" \
  "$gate_fixture_out" "integration: every declared part ran"
# The nested-marker probe is another stand-in gate over the same instruments.
# Its evidence proves only that it did not take flock, so it must not join the
# ordinary gate's exactly-once counters below.
: > "$gate_order"
: > "$gate_where"
gate_default_out="$(env -u _GANGLINE_GATE_LOCKED GANG_INTEGRATION_PARTS=cli \
  GANG_GATE_QUIET_SECONDS=1 GATE_PROVE_OVERLAP=1 \
  PATH="$gate_flock_bin:$PATH" "$gate_run/test/gate.sh" 2>&1)"
equal "the ordinary gate probes and owns both heavy-test locks" \
  "$(printf 'primary-probe\nowner-probe\nunlock\nunlock')" \
  "$(<"$gate_flock_args")"
equal "the no-argument gate runs lint, smoke, and the suite exactly once" \
  "$(printf 'integration\nlint\nsmoke')" "$(sort "$gate_order")"
if [ -e "$gate_overlap" ]; then
  pass "lint overlaps the smoke-and-integration branch"
else
  fail "lint overlaps the smoke-and-integration branch" \
    "integration did not receive lint's live handoff"
fi
exec 7>&- 8>&-
# WHERE they ran is the claim, and the gate's own report cannot witness it: that
# line prints the SOURCE path whatever directory the gates were run from.
gate_lint_where="$(awk '$1 == "lint" { print $2; exit }' "$gate_where")"
gate_smoke_where="$(awk '$1 == "smoke" { print $2; exit }' "$gate_where")"
gate_suite_where="$(awk '$1 == "integration" { print $2; exit }' "$gate_where")"
equal "and runs all three from one and the same directory" \
  "$gate_lint_where $gate_lint_where" "$gate_smoke_where $gate_suite_where"
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
  "$gate_default_out" "passed lint, smoke, and the integration suite"
contains "the mandatory gate names a selector it ignored" \
  "$gate_default_out" "ignoring GANG_INTEGRATION_PARTS=cli"
equal "the mandatory gate does not pass a focused selector to integration" \
  "parts=<> require=<1>" "$(<"$gate_parts")"
if [ ! -s "$gate_run_lock" ]; then
  pass "the ordinary gate clears its owner record before releasing"
else
  fail "the ordinary gate clears its owner record before releasing" \
    "the released inode still contains [$(<"$gate_run_lock")]"
fi

# A relative invocation remains relative to the caller, but snapshot creation
# changes to the repository root before re-executing the helper mode. The gate
# must therefore resolve its own executable before that directory change.
gate_subdir_rc=0
gate_subdir_out="$(
  cd "$gate_run/test" || exit
  GANG_GATE_QUIET_SECONDS=1 ./gate.sh 2>&1
)" || gate_subdir_rc=$?
equal "a gate invoked from its test directory can snapshot itself" \
  "0" "$gate_subdir_rc"
contains "the subdirectory invocation completes the mandatory suite" \
  "$gate_subdir_out" "passed lint, smoke, and the integration suite"

# A failed lint is one result, not permission to omit the behavioural evidence.
: > "$gate_order"
: > "$gate_where"
: > "$gate_flock_args"
gate_failed_lint_rc=0
GATE_FAIL_LINT=1 env -u _GANGLINE_GATE_LOCKED \
  PATH="$gate_flock_bin:$PATH" "$gate_run/test/gate.sh" >/dev/null 2>&1 \
  || gate_failed_lint_rc=$?
equal "a failed lint still runs smoke and integration" \
  "$(printf 'integration\nlint\nsmoke')" "$(sort "$gate_order")"
equal "the failed lint remains the gate status" "1" "$gate_failed_lint_rc"

# A step status is never also the lock probe's private conflict status.
: > "$gate_order"
: > "$gate_flock_args"
gate_status_75_rc=0
GATE_FAIL_LINT=75 env -u _GANGLINE_GATE_LOCKED \
  PATH="$gate_flock_bin:$PATH" "$gate_run/test/gate.sh" >/dev/null 2>&1 \
  || gate_status_75_rc=$?
equal "a step status of 75 is returned without rerunning the gate" \
  "75" "$gate_status_75_rc"
equal "a step status of 75 still runs each mandatory step once" \
  "$(printf 'integration\nlint\nsmoke')" "$(sort "$gate_order")"
# THE LAST LINE IS THE ONLY PART OF A RUN A PIPED READER IS SURE TO SEE, and
# the status is the part it is sure to lose: `test/gate.sh 2>&1 | tail -30` is
# how this is invoked whenever the output will not fit, and `$?` is then tail's.
# Asserting on the last line rather than anywhere in the output is the point —
# a verdict that can be followed by anything is a verdict a truncation can cut
# off, and the reader is left concluding green from the absence of a FAIL.
equal "a green gate ends on a verdict a truncated read still carries" \
  "gate: VERDICT PASS (status 0)" "$(printf '%s\n' "$gate_default_out" | tail -n 1)"

# A WAITER NEEDS TO DISTINGUISH A QUEUE FROM A HANG before it joins the queue.
# This copy differs only in its lock path, so the fixture can own the inode
# without reading or changing the host-wide lock used by the gate around it.
gate_wait="$RUN_ROOT/gate-wait"
cp -R "$gate_run" "$gate_wait"
gate_wait_lock="$RUN_ROOT/gate-wait.lock"
sed "s|GATE_HEAVY_LOCK=/tmp/gangline-heavy.lock|GATE_HEAVY_LOCK=$gate_wait_lock|" \
  "$ROOT/test/gate.sh" > "$gate_wait/test/gate.sh"
chmod +x "$gate_wait/test/gate.sh"
git -C "$gate_wait" add test/gate.sh
git -C "$gate_wait" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: private lock fixture'
gate_wait_ready="$RUN_ROOT/gate-wait-ready"
gate_wait_release="$RUN_ROOT/gate-wait-release"
gate_wait_stream="$RUN_ROOT/gate-wait-stream"
gate_wait_once="$RUN_ROOT/gate-wait-once"
mkfifo "$gate_wait_ready" "$gate_wait_release" "$gate_wait_stream"
exec 7<> "$gate_wait_ready"
exec 8<> "$gate_wait_release"
cat > "$gate_wait/test/lint.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
if mkdir "$gate_wait_once" 2>/dev/null; then
  printf 'ready\n' >&7
  IFS= read -r -u 8
fi
printf 'lint finished\n'
SH
chmod +x "$gate_wait/test/lint.sh"
git -C "$gate_wait" add test/lint.sh
git -C "$gate_wait" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: hold the first gate lint'
(
  cd "$gate_wait" || exit
  exec env -u _GANGLINE_GATE_LOCKED ./test/gate.sh 7>&7 8>&8
) > /dev/null 2>&1 &
gate_wait_holder=$!
gate_wait_ready_outcome=""
gate_wait_state=""
gate_fixture_event 7 gate_wait_ready_outcome gate_wait_state \
  "$RUN_ROOT/gate-wait-ready-event"
equal "the private holder publishes its readiness event" \
  "event" "$gate_wait_ready_outcome"
equal "the private holder acquired its lock before the waiter starts" \
  "ready" "$gate_wait_state"
gate_wait_record="$(<"$gate_wait_lock")"
gate_wait_record_pid="${gate_wait_record%%$'\t'*}"
gate_wait_record_pid="${gate_wait_record_pid#pid=}"
if [[ "$gate_wait_record_pid" =~ ^[0-9]+$ ]]; then
  pass "the gate itself writes a numeric PID into the owner record"
else
  fail "the gate itself writes a numeric PID into the owner record" \
    "record=[$gate_wait_record]"
fi
contains "the gate itself writes its working directory into the owner record" \
  "$gate_wait_record" "cwd=$gate_wait"
contains "the gate itself writes an acquisition epoch into the owner record" \
  "$gate_wait_record" "started="
env -u _GANGLINE_GATE_LOCKED "$gate_wait/test/gate.sh" > "$gate_wait_stream" 2>&1 &
gate_wait_pid=$!
exec 9< "$gate_wait_stream"
gate_wait_line=""
gate_wait_line_outcome=""
gate_fixture_event 9 gate_wait_line_outcome gate_wait_line \
  "$RUN_ROOT/gate-wait-event"
equal "the queued gate publishes its holder report event" \
  "event" "$gate_wait_line_outcome"
printf 'release\n' >&8
gate_wait_holder_rc=0
wait "$gate_wait_holder" || gate_wait_holder_rc=$?
gate_wait_pid_rc=0
wait "$gate_wait_pid" || gate_wait_pid_rc=$?
equal "the private lock holder exits after release" "0" "$gate_wait_holder_rc"
equal "the queued gate exits after acquiring the released lock" "0" "$gate_wait_pid_rc"
contains "a queued gate immediately names the lock holder" \
  "$gate_wait_line" "pid=$gate_wait_record_pid"
contains "the lock report names the holder's working directory" \
  "$gate_wait_line" "cwd=$gate_wait"
contains "the lock report says how long the holder has run" \
  "$gate_wait_line" "held_for="
if [ ! -s "$gate_wait_lock" ]; then
  pass "the last gate clears the corroborated owner record on release"
else
  fail "the last gate clears the corroborated owner record on release" \
    "the inode still contains [$(<"$gate_wait_lock")]"
fi
exec 7>&- 8>&- 9>&-

# A primary lock alone does not corroborate bytes left by an earlier owner.
gate_stale_ready="$RUN_ROOT/gate-stale-ready"
gate_stale_release="$RUN_ROOT/gate-stale-release"
gate_stale_stream="$RUN_ROOT/gate-stale-stream"
mkfifo "$gate_stale_ready" "$gate_stale_release" "$gate_stale_stream"
exec 7<> "$gate_stale_ready"
exec 8<> "$gate_stale_release"
printf 'pid=999999\tstarted=1\tcwd=/dead/predecessor\tlease=stale\n' > "$gate_wait_lock"
(
  exec 6>> "$gate_wait_lock"
  flock 6
  printf 'ready\n' >&7
  IFS= read -r -u 8
) &
gate_stale_holder=$!
gate_stale_ready_outcome=""
gate_stale_state=""
gate_fixture_event 7 gate_stale_ready_outcome gate_stale_state \
  "$RUN_ROOT/gate-stale-ready-event"
equal "the legacy holder publishes its readiness event" \
  "event" "$gate_stale_ready_outcome"
equal "the legacy holder acquired the primary lock before its waiter" \
  "ready" "$gate_stale_state"
env -u _GANGLINE_GATE_LOCKED "$gate_wait/test/gate.sh" > "$gate_stale_stream" 2>&1 &
gate_stale_waiter=$!
exec 9< "$gate_stale_stream"
gate_stale_line=""
gate_stale_hint=""
gate_stale_line_outcome=""
gate_stale_hint_outcome=""
gate_fixture_event 9 gate_stale_line_outcome gate_stale_line \
  "$RUN_ROOT/gate-stale-line-event"
if [ "$gate_stale_line_outcome" = event ]; then
  gate_fixture_event 9 gate_stale_hint_outcome gate_stale_hint \
    "$RUN_ROOT/gate-stale-hint-event"
fi
equal "the stale-lock waiter publishes its holder report event" \
  "event" "$gate_stale_line_outcome"
equal "the stale-lock waiter publishes its recovery hint event" \
  "event" "$gate_stale_hint_outcome"
printf 'release\n' >&8
gate_stale_holder_rc=0
wait "$gate_stale_holder" || gate_stale_holder_rc=$?
gate_stale_waiter_rc=0
wait "$gate_stale_waiter" || gate_stale_waiter_rc=$?
equal "the legacy lock holder exits after release" "0" "$gate_stale_holder_rc"
equal "the stale-lock waiter exits after acquiring the lock" "0" "$gate_stale_waiter_rc"
contains "an uncorroborated predecessor is reported as unknown" \
  "$gate_stale_line" "pid=unknown cwd=unknown held_for=unknowns"
contains "an unknown holder report gives a local recovery hint" \
  "$gate_stale_hint" "find the holder with: fuser -v $gate_wait_lock"
excludes "an uncorroborated predecessor is never named as the holder" \
  "$gate_stale_line" "/dead/predecessor"
exec 7>&- 8>&- 9>&-

# The optional real-harness lane shares the same inode, so its wrapper must
# publish and clear the same record even when setup itself refuses.
e2e_owner_lock="$RUN_ROOT/e2e-owner.lock"
e2e_owner_ready="$RUN_ROOT/e2e-owner-ready"
e2e_owner_release="$RUN_ROOT/e2e-owner-release"
e2e_owner_bin="$RUN_ROOT/e2e-owner-bin"
mkdir -p "$e2e_owner_bin"
mkfifo "$e2e_owner_ready" "$e2e_owner_release"
exec 7<> "$e2e_owner_ready"
exec 8<> "$e2e_owner_release"
cat > "$e2e_owner_bin/claude" <<'SH'
#!/usr/bin/env bash
printf 'ready\n' >&7
IFS= read -r -u 8
printf 'fixture claude 1.0\n'
SH
chmod +x "$e2e_owner_bin/claude"
GANG_E2E_LOCK="$e2e_owner_lock" PATH="$e2e_owner_bin:$PATH" \
  "$ROOT/test/e2e.sh" no-such-scenario >/dev/null 2>&1 &
e2e_owner_pid=$!
e2e_owner_ready_outcome=""
e2e_owner_state=""
gate_fixture_event 7 e2e_owner_ready_outcome e2e_owner_state \
  "$RUN_ROOT/e2e-owner-ready-event"
equal "the end-to-end lane publishes its readiness event" \
  "event" "$e2e_owner_ready_outcome"
equal "the end-to-end lane reached setup beneath its private lock" \
  "ready" "$e2e_owner_state"
e2e_owner_record="$(<"$e2e_owner_lock")"
contains "the end-to-end lane writes its cwd into the shared record" \
  "$e2e_owner_record" "cwd=$ROOT"
contains "the end-to-end lane writes a lease into the shared record" \
  "$e2e_owner_record" "lease="
printf 'release\n' >&8
e2e_owner_rc=0
wait "$e2e_owner_pid" || e2e_owner_rc=$?
equal "the end-to-end setup refusal keeps its status" "2" "$e2e_owner_rc"
if [ ! -s "$e2e_owner_lock" ]; then
  pass "the end-to-end lane clears its owner record before release"
else
  fail "the end-to-end lane clears its owner record before release" \
    "the released inode still contains [$(<"$e2e_owner_lock")]"
fi
exec 7>&- 8>&-

# THE TIMEOUT EXCEPTION IS SCALED, NOT STOPPED. The healthy output-gap
# measurement is 104s, the production quiet budget is 300s, and the fixture
# quiet budget is 1s. The nested gate's exit is the event that ends this
# fixture. A separate 120s test-harness ceiling does not claim the gate is late;
# it makes a missing event a named failure and leaves time for cleanup inside
# the integration job's 2700s ceiling.
# The blocked step writes two lines and its PID first; both are independent
# witnesses that it ran before the watchdog acts. The fake flock owns a marker
# for exactly as long as the nested ordinary gate, so its disappearance proves
# the failure returned through the lock holder rather than merely killing a
# child behind a lock that remained held.
gate_stall="$RUN_ROOT/gate-stall"
cp -R "$gate_run" "$gate_stall"
gate_stall_block="$RUN_ROOT/gate-stall-block"
gate_stall_stream="$RUN_ROOT/gate-stall-stream"
gate_stall_pidfile="$RUN_ROOT/gate-stall-child.pid"
gate_stall_lock="$RUN_ROOT/gate-stall.lock"
gate_stall_lint_block="$RUN_ROOT/gate-stall-lint-block"
gate_stall_event="$RUN_ROOT/gate-stall-event"
gate_stall_release="$RUN_ROOT/gate-stall-release"
gate_stall_wrapper="$RUN_ROOT/gate-stall-wrapper"
mkfifo "$gate_stall_block" "$gate_stall_lint_block" \
  "$gate_stall_event" "$gate_stall_release"
exec 9<> "$gate_stall_event"
exec 10<> "$gate_stall_release"
cat > "$gate_stall/test/lint.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
exec 8<> "$gate_stall_lint_block"
while :; do
  IFS= read -r -t 0.3 -u 8 || true
  printf 'lint pulse\n'
done
SH
cat > "$gate_stall/test/integration.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
printf 'integration last line one\n'
printf 'integration last line two\n'
printf '%s\n' "\$\$" > "$gate_stall_pidfile"
exec 7<> "$gate_stall_block"
IFS= read -r -u 7
SH
chmod +x "$gate_stall/test/lint.sh" "$gate_stall/test/integration.sh"
git -C "$gate_stall" add test/lint.sh test/integration.sh
git -C "$gate_stall" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: blocked integration with live lint sibling'
sed "s|GATE_HEAVY_LOCK=$gate_run_lock|GATE_HEAVY_LOCK=$gate_stall_lock|" \
  "$gate_stall/test/gate.sh" > "$gate_stall/test/gate.sh.new"
mv "$gate_stall/test/gate.sh.new" "$gate_stall/test/gate.sh"
chmod +x "$gate_stall/test/gate.sh"
git -C "$gate_stall" add test/gate.sh
git -C "$gate_stall" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: isolate the stalled gate lock'
cat > "$gate_stall_wrapper" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set +e
gate_pid=""
stop_gate() {
  trap - TERM
  if [ -n "\$gate_pid" ]; then
    kill -TERM "\$gate_pid" 2>/dev/null || true
    wait "\$gate_pid" 2>/dev/null || true
  fi
  exit 124
}
trap stop_gate TERM
"$gate_stall/test/gate.sh" &
gate_pid=\$!
wait "\$gate_pid"
rc=\$?
gate_pid=""
printf 'complete\\n' >&9
IFS= read -r -u 10 || true
exit "\$rc"
SH
chmod +x "$gate_stall_wrapper"
env -u _GANGLINE_GATE_LOCKED GANG_GATE_QUIET_SECONDS=1 \
  setsid "$gate_stall_wrapper" \
  > "$gate_stall_stream" 2>&1 &
gate_stall_gate_pid=$!
gate_stall_outcome=""
gate_stall_state=""
gate_fixture_event 9 gate_stall_outcome gate_stall_state \
  "$RUN_ROOT/gate-stall-completion"
case "$gate_stall_outcome:$gate_stall_state" in
  event:complete)
    pass "a quiet integration step fails within the scaled watchdog budget"
    printf 'release\n' >&10
    ;;
  deadline:*)
    fail "a quiet integration step fails within the scaled watchdog budget" \
      "the nested gate emitted no completion event within the ${GATE_FIXTURE_EVENT_CEILING}s fixture ceiling"
    gate_fixture_stop_session "$gate_stall_gate_pid" "the stalled fixture" || true
    ;;
  *)
    fail "a quiet integration step fails within the scaled watchdog budget" \
      "the nested gate's completion channel closed without an event"
    gate_fixture_stop_session "$gate_stall_gate_pid" "the stalled fixture" || true
    ;;
esac
gate_stall_rc=0
wait "$gate_stall_gate_pid" 2>/dev/null || gate_stall_rc=$?
if [ "$gate_stall_outcome" != event ]; then
  gate_stall_rc="$gate_stall_outcome"
fi
gate_stall_out="$(<"$gate_stall_stream")"
exec 9>&- 10>&-
if [ -s "$gate_stall_pidfile" ]; then
  gate_stall_child_pid="$(cat "$gate_stall_pidfile")"
else
  gate_stall_child_pid=""
fi
gate_stall_child_alive_after_gate=0
if [ -n "$gate_stall_child_pid" ] && \
    kill -0 "$gate_stall_child_pid" 2>/dev/null; then
  gate_stall_child_alive_after_gate=1
fi
gate_stall_lock_rc_after_gate=0
flock -n "$gate_stall_lock" true || gate_stall_lock_rc_after_gate=$?
equal "a stalled integration is the gate's quiet-expiry status" "124" "$gate_stall_rc"
contains "the stall names the step and quiet budget" \
  "$gate_stall_out" "STALLED: integration produced no output for 1s"
contains "the stall prints its process tree" "$gate_stall_out" "PROCESS TREE"
contains "the stall keeps the first trailing line" \
  "$gate_stall_out" "integration last line one"
contains "the stall keeps the last trailing line" \
  "$gate_stall_out" "integration last line two"
if [ -n "$gate_stall_child_pid" ] && \
    [ "$gate_stall_child_alive_after_gate" -eq 0 ]; then
  pass "the stalled step leaves no descendant process"
else
  fail "the stalled step leaves no descendant process" \
    "after gate exit, fixture child [$gate_stall_child_pid] alive=$gate_stall_child_alive_after_gate"
fi
if [ "$gate_stall_lock_rc_after_gate" -eq 0 ]; then
  pass "the stalled gate releases its heavy-test lock"
else
  fail "the stalled gate releases its heavy-test lock" \
    "after gate exit, the private lock probe exited $gate_stall_lock_rc_after_gate"
fi

# A STALL MARKER PRECEDES THE SLOW REPORT AND THE BRANCH'S STATUS WRITE. If the
# sibling finishes during that window, the main shell may observe the marker
# after cancellation has prevented the status file. Drive that exact state
# directly so scheduler timing cannot turn the regression test green: the
# marker itself must carry the watchdog result used for the final verdict.
gate_status_race="$RUN_ROOT/gate-status-race"
cp -R "$gate_run" "$gate_status_race"
gate_status_race_lock="$RUN_ROOT/gate-status-race.lock"
sed -e "s|GATE_HEAVY_LOCK=$gate_run_lock|GATE_HEAVY_LOCK=$gate_status_race_lock|" \
  -e '/^gate_lint_branch() {/a\
  : > "$WORK/lint.out"\
  printf '\''124\\n'\'' > "$WORK/lint.stalled"\
  return 0' \
  "$gate_status_race/test/gate.sh" > "$gate_status_race/test/gate.sh.new"
mv "$gate_status_race/test/gate.sh.new" "$gate_status_race/test/gate.sh"
chmod +x "$gate_status_race/test/gate.sh"
git -C "$gate_status_race" add test/gate.sh
git -C "$gate_status_race" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: stall marker before status fixture'
gate_status_race_rc=0
gate_status_race_out="$(env -u _GANGLINE_GATE_LOCKED \
  "$gate_status_race/test/gate.sh" 2>&1)" || gate_status_race_rc=$?
equal "a stall marker remains the gate verdict before its status write" \
  "124" "$gate_status_race_rc"
excludes "a recorded stall can never fall through to a passing verdict" \
  "$gate_status_race_out" "VERDICT PASS"
gate_status_race_kept="$(printf '%s\n' "$gate_status_race_out" \
  | awk '/^  \/.*gangline-gate\./ { print $1; exit }')"
[ -z "$gate_status_race_kept" ] || rm -rf -- "$gate_status_race_kept"

# A PID ownership mismatch is a safety refusal, not permission to wait forever
# on the child the gate deliberately declined to signal.
gate_refusal="$RUN_ROOT/gate-refusal"
cp -R "$gate_stall" "$gate_refusal"
gate_refusal_block="$RUN_ROOT/gate-refusal-block"
gate_refusal_stream="$RUN_ROOT/gate-refusal-stream"
gate_refusal_pidfile="$RUN_ROOT/gate-refusal-child.pid"
gate_refusal_lock="$RUN_ROOT/gate-refusal.lock"
gate_refusal_ready="$RUN_ROOT/gate-refusal-ready"
gate_refusal_release="$RUN_ROOT/gate-refusal-release"
gate_refusal_wrapper="$RUN_ROOT/gate-refusal-wrapper"
mkfifo "$gate_refusal_block" "$gate_refusal_ready" "$gate_refusal_release"
exec 9<> "$gate_refusal_ready"
exec 10<> "$gate_refusal_release"
sed -e "s|$gate_stall_block|$gate_refusal_block|g" \
  -e "s|$gate_stall_pidfile|$gate_refusal_pidfile|g" \
  "$gate_refusal/test/integration.sh" > "$gate_refusal/test/integration.sh.new"
mv "$gate_refusal/test/integration.sh.new" "$gate_refusal/test/integration.sh"
sed -e "s|GATE_HEAVY_LOCK=$gate_stall_lock|GATE_HEAVY_LOCK=$gate_refusal_lock|" \
  -e 's/parent=\$BASHPID/parent=$((BASHPID + 100000))/' \
  "$gate_refusal/test/gate.sh" > "$gate_refusal/test/gate.sh.new"
mv "$gate_refusal/test/gate.sh.new" "$gate_refusal/test/gate.sh"
chmod +x "$gate_refusal/test/gate.sh" "$gate_refusal/test/integration.sh"
git -C "$gate_refusal" add test/gate.sh test/integration.sh
git -C "$gate_refusal" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: force watchdog ownership refusal'
cat > "$gate_refusal_wrapper" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set +e
gate_pid=""
stop_gate() {
  trap - TERM
  if [ -n "\$gate_pid" ]; then
    kill -TERM "\$gate_pid" 2>/dev/null || true
    wait "\$gate_pid" 2>/dev/null || true
  fi
  exit 124
}
trap stop_gate TERM
"$gate_refusal/test/gate.sh" &
gate_pid=\$!
wait "\$gate_pid"
rc=\$?
gate_pid=""
printf 'complete\\n' >&9
IFS= read -r -u 10 || true
exit "\$rc"
SH
chmod +x "$gate_refusal_wrapper"
env -u _GANGLINE_GATE_LOCKED GANG_GATE_QUIET_SECONDS=1 \
  setsid "$gate_refusal_wrapper" > "$gate_refusal_stream" 2>&1 &
gate_refusal_gate_pid=$!
gate_refusal_outcome=""
gate_refusal_state=""
gate_fixture_event 9 gate_refusal_outcome gate_refusal_state \
  "$RUN_ROOT/gate-refusal-completion"
case "$gate_refusal_outcome:$gate_refusal_state" in
  event:complete)
    pass "an ownership refusal exits instead of wedging the gate"
    printf 'release\n' >&10
    ;;
  deadline:*)
    fail "an ownership refusal exits instead of wedging the gate" \
      "the nested gate emitted no completion event within the ${GATE_FIXTURE_EVENT_CEILING}s fixture ceiling"
    gate_fixture_stop_session "$gate_refusal_gate_pid" "the refusal fixture" || true
    ;;
  *)
    fail "an ownership refusal exits instead of wedging the gate" \
      "the nested gate's completion channel closed without an event"
    gate_fixture_stop_session "$gate_refusal_gate_pid" "the refusal fixture" || true
    ;;
esac
gate_refusal_rc=0
wait "$gate_refusal_gate_pid" 2>/dev/null || gate_refusal_rc=$?
if [ "$gate_refusal_outcome" != event ]; then
  gate_refusal_rc="$gate_refusal_outcome"
fi
gate_refusal_out="$(<"$gate_refusal_stream")"
exec 9>&- 10>&-
gate_refusal_lock_rc=0
flock -n "$gate_refusal_lock" true || gate_refusal_lock_rc=$?
equal "an ownership refusal keeps its distinct gate status" "125" "$gate_refusal_rc"
equal "an ownership refusal releases the heavy-test lock" "0" "$gate_refusal_lock_rc"
contains "an ownership refusal says why the child was not killed" \
  "$gate_refusal_out" "refusing to kill integration"
if [ -s "$gate_refusal_pidfile" ]; then
  gate_refusal_child_pid="$(<"$gate_refusal_pidfile")"
  gate_refusal_sid="$(ps -o sid= -p "$gate_refusal_child_pid" 2>/dev/null \
    | tr -d ' ' || true)"
  if [ -z "$gate_refusal_sid" ]; then
    pass "the refused fixture child vanished before test rescue"
  elif [ "$gate_refusal_sid" = "$gate_refusal_child_pid" ]; then
    pass "the refused fixture child remains scoped for test rescue"
    kill -KILL -- "-$gate_refusal_child_pid" 2>/dev/null || true
  else
    fail "the refused fixture child remains scoped for test rescue" \
      "pid=$gate_refusal_child_pid sid=${gate_refusal_sid:-unknown}"
  fi
else
  fail "the refused fixture child was witnessed before rescue" \
    "the fixture did not write its pid"
fi

# A healthy long step must keep extending its lease. Each pulse arrives at less
# than one third of the scaled 1s quiet budget, six times in a row; success proves
# the budget resets on output rather than becoming a total-duration ceiling.
gate_pulse="$RUN_ROOT/gate-pulse"
cp -R "$gate_run" "$gate_pulse"
gate_pulse_wait="$RUN_ROOT/gate-pulse-wait"
mkfifo "$gate_pulse_wait"
cat > "$gate_pulse/test/integration.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
exec 7<> "$gate_pulse_wait"
for n in 1 2 3 4 5 6; do
  IFS= read -r -t 0.3 -u 7 || true
  printf 'pulse %s\n' "\$n"
done
printf 'integration: every declared part ran\n'
SH
chmod +x "$gate_pulse/test/integration.sh"
git -C "$gate_pulse" add test/integration.sh
git -C "$gate_pulse" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: pulsing integration fixture'
gate_pulse_rc=0
gate_pulse_out="$(GANG_GATE_QUIET_SECONDS=1 \
  "$gate_pulse/test/gate.sh" 2>&1)" || gate_pulse_rc=$?
equal "a step that prints just under the quiet bound passes" "0" "$gate_pulse_rc"
contains "the pulsing step ran through its final pulse" "$gate_pulse_out" "pulse 6"
excludes "a pulsing step is not called stalled" "$gate_pulse_out" "STALLED"

# REQUIRE_ALL is the gate's evidence, not a permission to print its verdict. A
# nested real focused run reaches the actual omitted-part branch. Its own gate
# self-test skips this probe, so this remains one child rather than recursing.
if [ "${GANG_INTEGRATION_REQUIRE_ALL_PROBE:-0}" != 1 ]; then
  require_all_probe_rc=0
  require_all_probe_out="$(env GANG_INTEGRATION_PARTS=cli GANG_INTEGRATION_REQUIRE_ALL=1 \
    GANG_INTEGRATION_REQUIRE_ALL_PROBE=1 "$ROOT/test/integration.sh" 2>&1)" \
    || require_all_probe_rc=$?
  equal "a required full run refuses a focused selector" \
    "1" "$require_all_probe_rc"
  contains "the required run names every part cli omitted" \
    "$require_all_probe_out" \
    "required full run omitted declared parts: substrate,hitch,compose,spool,readiness,hooks,notify,usage,cap,tick"
  excludes "a focused required run never attests every part ran" \
    "$require_all_probe_out" "integration: every declared part ran"

  # THE ONE PROBE HERE THAT ENDED THE WHOLE SUITE INSTEAD OF REPORTING. A bare
  # assignment leaves the child's status to `set -e`, which kills this subshell
  # before it writes its counts; the parent then has a status and nothing to
  # read, and a run of thousands of checks ends early with a verdict on none of
  # them. The status is a check of its own now, and the child's own output is
  # what the failure carries, so a nested run that dies says why here.
  focused_probe_rc=0
  focused_probe_out="$(env -u GANG_INTEGRATION_REQUIRE_ALL GANG_INTEGRATION_PARTS=cli \
    GANG_INTEGRATION_REQUIRE_ALL_PROBE=1 "$ROOT/test/integration.sh" 2>&1)" \
    || focused_probe_rc=$?
  if [ "$focused_probe_rc" -ne 0 ]; then
    fail "a green focused run ends on a green status" \
      "status $focused_probe_rc; its last lines were [$(printf '%s\n' "$focused_probe_out" | tail -n 5)]"
  else
    pass "a green focused run ends on a green status"
  fi
  contains "a focused run carries its scope in the terminal summary" \
    "$(printf '%s\n' "$focused_probe_out" | tail -n 1)" \
    "focused parts cli (full suite: cli substrate hitch compose spool readiness hooks notify usage cap tick)"

  readiness_dependency_rc=0
  readiness_dependency_out="$(env -u GANG_INTEGRATION_REQUIRE_ALL \
    GANG_INTEGRATION_PARTS=cli,substrate,readiness \
    GANG_INTEGRATION_REQUIRE_ALL_PROBE=1 "$ROOT/test/integration.sh" 2>&1)" \
    || readiness_dependency_rc=$?
  equal "a focused readiness run refuses before using compose's fixture" \
    "2" "$readiness_dependency_rc"
  contains "the refusal pins readiness's compose dependency" \
    "$readiness_dependency_out" "focused part readiness requires compose"
  contains "the repair selector retains every part already selected" \
    "$readiness_dependency_out" \
    "run GANG_INTEGRATION_PARTS=cli,substrate,compose,readiness"
fi

cat > "$gate_run/test/integration.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf 'integration\n' >> "$gate_order"
exit 0
SH
chmod +x "$gate_run/test/integration.sh"
refuses "the gate refuses a suite without every-part attestation" \
  "integration did not attest that every declared part ran" \
  "$gate_run/test/gate.sh"

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
printf 'integration: every declared part ran\n'
exit 0
SH
cp "$gate_edit/test/integration.sh" "$gate_edit/test/smoke.sh"
chmod +x "$gate_edit/test/lint.sh" "$gate_edit/test/smoke.sh" "$gate_edit/test/integration.sh" \
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
# The refusal ends on the same line in the same place, after the kept-snapshot
# notes rather than before them, and it carries a status the gate never chose
# for itself — so the word is decided by whether this run reached a verdict,
# not by a list of statuses somebody remembered to keep current.
equal "a refused gate ends on that same verdict, carrying its status" \
  "gate: VERDICT REFUSED (status 3)" "$(printf '%s\n' "$gate_fail_out" | tail -n 1)"
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

# A SIGNALLED GATE STOPS AT THE SIGNAL. One handler for the exit and for the
# signals does not end a run — a bash signal handler returns to the interrupted
# flow — and bash holds it until the foreground child returns, so the teardown
# lands after the gates and what resumes is the verdict itself. The run then
# reads lint's output out of the directory it has just deleted, ends 1 instead
# of the conventional signal status, and reports no verdict at all.
#
# The signal is sent BY the stand-in suite, which needs no barrier and no clock:
# the gate is inside that call and blocked on it, so the handler runs the moment
# it returns. The line immediately before that signal is evidence only execution
# can produce. It must already be in the gate's transcript when the handler
# exits, because cleanup removes the captured file before any later replay.
#
# FINDING THE GATE IS THE PART THAT CAN GO WRONG SILENTLY. A bash subshell
# shares its parent's cmdline, so the nearest ancestor matching the gate is the
# fork that ran this suite, not the gate: killing that one makes the gate report
# a failed suite and refuse, which looks the same whether the fix is present or
# not. The walk therefore climbs to the HIGHEST ancestor still carrying this
# fixture's own gate path and stops at the first that does not — and the path
# being this fixture's own is what keeps it off any other gate on the host,
# including the run executing this line.
gate_signal="$RUN_ROOT/gate-signalled"
mkdir -p "$gate_signal/test"
cp "$ROOT/test/gate.sh" "$gate_signal/test/gate.sh"
gate_signal="$(cd -P "$gate_signal" && pwd)"
gate_signal_found="$RUN_ROOT/gate-signalled-found"
for gate_signal_stub in lint smoke; do
  cat > "$gate_signal/test/$gate_signal_stub.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
exit 0
SH
done
cat > "$gate_signal/test/integration.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
target=""
p=\$PPID
while [ -n "\$p" ] && [ "\$p" != 1 ]; do
  cmd=\$(tr '\\0' ' ' < /proc/\$p/cmdline 2>/dev/null)
  case "\$cmd" in
    *"$gate_signal/test/gate.sh"*) target=\$p ;;
    *) [ -n "\$target" ] && break ;;
  esac
  set -- \$(sed 's/^.*) //' /proc/\$p/stat 2>/dev/null)
  p=\$2
done
printf '%s\\n' "\${target:-no-gate-in-ancestry}" > "$gate_signal_found"
printf '%s\\n' 'integration output before the gate signal'
[ -n "\$target" ] && kill -TERM "\$target"
exit 0
SH
chmod +x "$gate_signal/test/lint.sh" "$gate_signal/test/smoke.sh" \
  "$gate_signal/test/integration.sh"
git -C "$gate_signal" init -q
git -C "$gate_signal" add -A
git -C "$gate_signal" -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: signalled gate fixture'
gate_signal_rc=0
# The lock marker is INHERITED rather than unset: a nested fixture gate that
# re-execs under the real heavy lock would block on whatever holds it, up to and
# including the run that is executing this line.
gate_signal_out="$("$gate_signal/test/gate.sh" 2>&1)" || gate_signal_rc=$?
# A stub that never found the gate leaves a run nobody interrupted, and every
# assertion below it passes for that reason rather than for the property.
if [ "$(cat "$gate_signal_found" 2>/dev/null)" -gt 0 ] 2>/dev/null; then
  pass "the stand-in suite signalled the gate itself, not the subshell running it"
else
  fail "the stand-in suite signalled the gate itself, not the subshell running it" \
    "the ancestry walk recorded [$(cat "$gate_signal_found" 2>/dev/null)]"
fi
equal "a signalled gate ends on the signal rather than on a status it computed" \
  "143" "$gate_signal_rc"
# The selector calibrations above recursively re-enter this self-test, but only
# their requested output is read back; their counters never join this parent's
# verdict. The outer run owns this assertion once, where its failure is visible.
if [ "${GANG_INTEGRATION_REQUIRE_ALL_PROBE:-0}" != 1 ]; then
  contains "and preserves integration output produced before the signal" \
    "$gate_signal_out" "integration output before the gate signal"
fi
if printf '%s\n' "$gate_signal_out" | grep -q 'No such file or directory'; then
  fail "and nothing below the teardown reads the snapshot it deleted" \
    "the run continued past its own teardown: $(printf '%s\n' "$gate_signal_out" \
      | grep 'No such file or directory' | head -n 1)"
else
  pass "and nothing below the teardown reads the snapshot it deleted"
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

# THE LINE EVERYONE ACTUALLY READS, IN THE FORM NO PASSING RUN PRINTS. A suite
# cannot fail on purpose to demonstrate its own failing summary, so the branch
# that has to be right is the one every green run skips — and it went in
# untested for exactly that reason. Both suites now end on one shared function
# so the branch is reachable from here, driven from the same file they source
# with the counters a failed run would hand it.
tail_fix="$RUN_ROOT/suite-tail"
mkdir -p "$tail_fix"
cp "$ROOT/test/suite-tail.sh" "$tail_fix/suite-tail.sh"
cat > "$tail_fix/run.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
. "$(dirname "$0")/suite-tail.sh"
checks=0 fails=0 unknowns=0
pass() { checks=$((checks + 1)); }
fail() { checks=$((checks + 1)); fails=$((fails + 1)); }
unknown() { unknowns=$((unknowns + 1)); suite_unknown "$1" "$2"; }
pass; pass; pass
case "${1:-}" in
  red) fail; fail ;;
  unknown) unknown 'a claim this host cannot settle' 'the substrate answers neither way' ;;
esac
clause="${2:-}"
[ -n "$clause" ] || clause="$(suite_unknown_clause "$unknowns")"
suite_tail "$checks" "$fails" 7 "$clause"
SH
chmod +x "$tail_fix/run.sh"
equal "a suite whose checks all passed ends on a bare count" \
  "3 checks in 7s" "$("$tail_fix/run.sh")"
equal "and a suite that accumulated failures names them in that same line" \
  "5 checks, 2 FAIL in 7s" "$("$tail_fix/run.sh" red)"
equal "with the e2e lane's trailing clause after the verdict, not instead of it" \
  "5 checks, 2 FAIL in 7s against harness-x" \
  "$("$tail_fix/run.sh" red "against harness-x")"

# AND THE THIRD COLUMN, WHICH IS NOT A COLUMN. A claim the host could not settle
# is neither proved nor refuted, so it must not move the check count in either
# direction — a run that quietly counted it as a pass would report coverage it
# never had. On this host the suite's own unknown branch is unreachable (the
# tmux here returns raw user-option bytes, so the claim IS settled), which is
# exactly why the branch is driven from a fixture rather than left to a run that
# happens to be standing somewhere else.
equal "a claim the run could not settle is named without being counted either way" \
  "?    a claim this host cannot settle
       the substrate answers neither way
3 checks in 7s (1 unknown — see the ? lines above)" \
  "$("$tail_fix/run.sh" unknown)"

# AND THE BRANCH A WEDGED RUN TAKES, which is the one no run of any colour
# reaches while every barrier it waits on is answered. A barrier that expires is
# neither a failed assertion nor a slow box: it is a channel this suite waited
# on that nothing ever signalled, and before the wait was bounded it printed
# nothing at all, because the run never got as far as its own summary. The shim
# writes the channel down precisely so a fixture in a dying pane still leaves a
# record, so the record is what is driven here.
tail_ledger="$tail_fix/wedged"
: > "$tail_ledger"
equal "an empty barrier ledger says nothing at all" "" \
  "$(. "$ROOT/test/suite-tail.sh"; suite_wedged_barriers "$tail_ledger")"
equal "and a ledger that was never written says nothing either" "" \
  "$(. "$ROOT/test/suite-tail.sh"; suite_wedged_barriers "$tail_fix/absent")"
printf 'wait-for test-channel-nobody-signalled\t120\n' > "$tail_ledger"
tail_wedged_out="$(. "$ROOT/test/suite-tail.sh"; suite_wedged_barriers "$tail_ledger")"
contains "a recorded expiry is named rather than left to a process list" \
  "$tail_wedged_out" "BARRIER(S) NEVER SIGNALLED"
contains "and the channel it waited on is in that report" \
  "$tail_wedged_out" "test-channel-nobody-signalled"
contains "and how long it waited, so a ceiling is not mistaken for a slow box" \
  "$tail_wedged_out" "waited 120s"

# AND BOTH SUITES REACH IT THROUGH THAT FILE. A private copy of the summary in
# either one is a copy the three checks above do not cover, which is how this
# branch went unexercised in the first place.
for tail_suite in integration.sh e2e.sh; do
  if grep -q '^\. "\$ROOT/test/suite-tail\.sh"$' "$ROOT/test/$tail_suite" \
    && grep -q 'suite_tail "\$checks" "\$fails" "\$SECONDS"' "$ROOT/test/$tail_suite"; then
    pass "test/$tail_suite ends through the shared summary rather than a copy"
  else
    fail "test/$tail_suite ends through the shared summary rather than a copy" \
      "it does not both source test/suite-tail.sh and end on suite_tail"
  fi
done

# THE SERVERS A RUN STARTS DO NOT OUTLIVE IT. A tmux server daemonises, so the
# only thing that ends one is something that goes looking for it. A teardown
# written as an EXIT trap over a list of sockets does neither when the run is
# killed outright, and covers only the sockets on that list when it does run.
# Both gaps leave a server holding a pty for as long as the host is up, and
# neither is visible from a green run: the run passes, and the survivors are
# found days later in a process list.
#
# The stand-in below is a run of that shape — a private root, a server on the
# root's own socket, a second under a nested root the named teardown never
# mentions, and a third on a `-L` label outside the root entirely. It is ended
# three ways: an early exit, a signal its trap can answer, and a SIGKILL no
# trap can. Each ending is asked the same question.
#
# WHAT COUNTS AS GONE IS READ FROM THE KERNEL, not from the socket file. A run
# that removed its root while a server was still bound inside it leaves a
# server whose socket path no longer exists, and that answers every `tmux -S`
# probe exactly as a server that is not running does.
reaper_live_servers() { # $1 run root, $2 socket outside it -> one server pid per line
  local root="$1" outside="${2:-}" proc comm link inode path
  local -A bound=()
  # The bound path is the last field and may hold a space, so what is read here
  # is the remainder of the line rather than one word of it. Reading it either
  # way would make this instrument agree with a reaper that has the same fault.
  while read -r inode path; do bound["$inode"]="$path"; done \
    < <(awk 'FNR > 1 && NF >= 8 {
          inode = $7
          for (i = 1; i <= 7; i++) { sub(/^[^ \t]+[ \t]+/, "") }
          print inode, $0
        }' /proc/net/unix)
  for proc in /proc/[0-9]*; do
    { read -r comm < "$proc/comm"; } 2>/dev/null || continue
    [ "$comm" = "tmux: server" ] || continue
    while IFS= read -r link; do
      inode="${link#socket:[}"
      path="${bound[${inode%]}]:-}"
      [ -n "$path" ] || continue
      if [ "$path" != "${path#"$root"/}" ] \
        || { [ -n "$outside" ] && [ "$path" = "$outside" ]; }; then
        printf '%s\n' "${proc#/proc/}"
        break
      fi
    done < <(find "$proc/fd" -maxdepth 1 -type l -printf '%l\n' 2>/dev/null \
      | grep '^socket:\[')
  done | sort -u
}

reaper_wait_host_identity() { # $1 host pid, $2 start time -> return when it is gone
  "$(suite_reaper_python)" - "$1" "$2" <<'PY'
import errno
import os
import select
import sys

pid = int(sys.argv[1])
expected = sys.argv[2]
try:
    handle = os.pidfd_open(pid)
except OSError as error:
    if error.errno == errno.ESRCH:
        sys.exit(0)
    raise
try:
    with open("/proc/%d/stat" % pid, encoding="utf-8", errors="replace") as stream:
        stat = stream.read()
    if stat[stat.rindex(")") + 1:].split()[19] != expected:
        sys.exit(0)
    select.select([handle], [], [], None)
finally:
    os.close(handle)
PY
}

reaper_fix="$RUN_ROOT/suite-reaper"
mkdir -p "$reaper_fix"
reaper_label="gangline-reaper-outside-$$"
reaper_label_socket="/tmp/tmux-$(id -u)/$reaper_label"
# That label resolves under the host's own tmux directory rather than any run
# root, so this run adopts it as well: if the stand-in's reaper never fires,
# this run's does, and the label server does not become the leak under test.
suite_reaper_track "$reaper_label_socket"
# THIS FRAGMENT RUNS BESIDE THE SUITE, NOT AFTER IT, so the run's own tmux
# server may not exist yet when the barriers below are first used: the part that
# creates it is running in another process at the same time. A barrier on a
# server this fragment starts itself is up before it is waited on. It goes under
# the run root like everything else here, so the run's teardown reaps it.
reaper_barrier_root="$reaper_fix/barrier"
mkdir -p "$reaper_barrier_root"
reaper_barrier_socket="$reaper_barrier_root/tmux-$(id -u)/default"
TMUX_TMPDIR="$reaper_barrier_root" tmux new-session -d -s reaper-barrier -n idle \
  "PS1='> ' bash --norc"
equal "the reaper fixtures have a barrier server of their own" \
  "reaper-barrier" \
  "$(tmux -S "$reaper_barrier_socket" list-sessions -F '#{session_name}')"
cat > "$reaper_fix/run.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
unset TMUX TMUX_PANE
. "$ROOT/test/suite-python.sh"
. "$ROOT/test/suite-reaper.sh"
reaper_root="\$1"
reaper_mode="\$2"
mkdir -p "\$reaper_root/nested"
suite_reaper_start "\$reaper_root"
suite_reaper_track "$reaper_label_socket"
# The teardown a run of this shape writes by hand: the one socket it knows the
# name of, and the sweep that covers the rest of what it started.
cleanup() {
  tmux -S "\$reaper_root/tmux-\$(id -u)/default" kill-server 2>/dev/null || true
  suite_reaper_sweep "\$reaper_root"
}
on_signal() { trap - HUP INT TERM; exit "\$((128 + \$1))"; }
trap cleanup EXIT
trap 'on_signal 1' HUP
trap 'on_signal 2' INT
trap 'on_signal 15' TERM
TMUX_TMPDIR="\$reaper_root" tmux new-session -d -s reaper-primary -n seed \\
  "PS1='> ' bash --norc"
TMUX_TMPDIR="\$reaper_root/nested" tmux new-session -d -s reaper-nested -n seed \\
  "PS1='> ' bash --norc"
env -u TMUX_TMPDIR tmux -L "$reaper_label" new-session -d -s reaper-outside \\
  -n seed "PS1='> ' bash --norc"
mkfifo "\$reaper_root/hold"
tmux -S "$reaper_barrier_socket" wait-for -S "\$reaper_mode-reaper-up"
# The fifo is how this run is held still while its servers are counted; the
# caller either releases it or kills this process while it waits.
read -r _ < "\$reaper_root/hold" || true
# An early exit is a run that stops in the middle of its own work, which is
# what a failed check under set -e does.
[ "\$reaper_mode" = early ] && false
exit 0
SH
chmod +x "$reaper_fix/run.sh"

for reaper_case in early term kill; do
  reaper_root="$reaper_fix/$reaper_case"
  SUITE_REAPER_DONE_SOCKET="$reaper_barrier_socket" \
    SUITE_REAPER_DONE_CHANNEL="$reaper_case-reaper-done" \
    SUITE_REAPER_TMUX="$REAL_TMUX" \
    "$reaper_fix/run.sh" "$reaper_root" "$reaper_case" &
  reaper_pid=$!
  tmux -S "$reaper_barrier_socket" wait-for "$reaper_case-reaper-up"
  reaper_before="$(reaper_live_servers "$reaper_root" "$reaper_label_socket" | wc -l | tr -d ' ')"
  equal "the stand-in run starts three servers on three sockets to lose" \
    "3" "$reaper_before"
  case "$reaper_case" in
    early) : > "$reaper_root/hold" ;;
    term) kill -TERM "$reaper_pid" 2>/dev/null || true ;;
    kill) kill -KILL "$reaper_pid" 2>/dev/null || true ;;
  esac
  wait "$reaper_pid" 2>/dev/null || true
  tmux -S "$reaper_barrier_socket" wait-for "$reaper_case-reaper-done"
  reaper_after="$(reaper_live_servers "$reaper_root" "$reaper_label_socket")"
  case "$reaper_case" in
    early) reaper_ending="a run that stops in the middle of its own work" ;;
    term) reaper_ending="a run stopped by a signal its trap can answer" ;;
    kill) reaper_ending="a run killed outright, where no trap runs at all" ;;
  esac
  equal "$reaper_ending leaves no tmux server behind" "" "$reaper_after"
  if [ -e "$reaper_root" ]; then
    fail "$reaper_ending leaves no fixture root behind" "$reaper_root is still there"
  else
    pass "$reaper_ending leaves no fixture root behind"
  fi
done

# A NEW SESSION IS STILL IN THE CGroup THAT STARTED IT. The gate gives each
# mandatory step a killable execution boundary, and the harness that invoked
# the gate may give the whole run one too. A watcher left in that boundary is
# killed beside an abruptly ended parent, before it can sweep a server started
# outside the boundary. This fixture gives the stand-in run a transient unit,
# leaves its adopted server beside that unit, and waits for the exact watcher
# process to be gone before reading the server. That makes the result an
# ordering fact, not a race with the sweep.
reaper_unit_nonce="${SUITE_REAPER_TOKEN:0:16}"
reaper_cgroup_probe="gangline-reaper-probe-$reaper_unit_nonce"
reaper_cgroup_available=0
systemd-run --user --quiet --wait --pipe --collect --service-type=exec \
  --unit="$reaper_cgroup_probe" /bin/true >/dev/null 2>&1 \
  && reaper_cgroup_available=1
if [ "$reaper_cgroup_available" -eq 1 ]; then
  reaper_cgroup_root="$reaper_fix/cgroup-parent"
  reaper_cgroup_label="gangline-reaper-cgroup-$reaper_unit_nonce"
  reaper_cgroup_socket="/tmp/tmux-$(id -u)/$reaper_cgroup_label"
  reaper_cgroup_unit="gangline-reaper-parent-$reaper_unit_nonce"
  suite_reaper_track "$reaper_cgroup_socket"
  env -u TMUX_TMPDIR tmux -L "$reaper_cgroup_label" new-session -d \
    -s reaper-cgroup -n seed "PS1='> ' bash --norc"
  equal "the cgroup fixture starts its adopted server outside the parent boundary" \
    "1" "$(reaper_live_servers "$reaper_cgroup_root" "$reaper_cgroup_socket" | wc -l | tr -d ' ')"
  cat > "$reaper_fix/cgroup-parent.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
. "$ROOT/test/suite-python.sh"
. "$ROOT/test/suite-reaper.sh"
suite_reaper_start "$reaper_cgroup_root"
suite_reaper_track "$reaper_cgroup_socket"
"$(suite_reaper_python)" - "$reaper_cgroup_root" \
  > "$reaper_fix/cgroup-watcher" <<'PY'
import os
import sys

root = os.fsencode(sys.argv[1])
matches = []
for entry in os.listdir("/proc"):
    if not entry.isdigit():
        continue
    try:
        with open("/proc/%s/cmdline" % entry, "rb") as stream:
            argv = stream.read().split(b"\0")
        if b"--watch" not in argv or root not in argv:
            continue
        with open("/proc/%s/stat" % entry, encoding="utf-8", errors="replace") as stream:
            stat = stream.read()
        started = stat[stat.rindex(")") + 1:].split()[19]
        matches.append((int(entry), started))
    except (OSError, ValueError, IndexError):
        continue
if len(matches) != 1:
    sys.stderr.write("expected one watcher for %r, found %r\n" % (sys.argv[1], matches))
    sys.exit(1)
sys.stdout.write("%s %s\n" % matches[0])
PY
tmux -S "$reaper_barrier_socket" wait-for -S reaper-cgroup-up
tmux -S "$reaper_barrier_socket" wait-for reaper-cgroup-hold
kill -KILL "\$\$"
SH
  chmod +x "$reaper_fix/cgroup-parent.sh"
  systemd-run --user --quiet --wait --pipe --collect --service-type=exec \
    --unit="$reaper_cgroup_unit" "$reaper_fix/cgroup-parent.sh" &
  reaper_cgroup_client=$!
  tmux -S "$reaper_barrier_socket" wait-for reaper-cgroup-up
  read -r reaper_cgroup_watcher reaper_cgroup_started \
    < "$reaper_fix/cgroup-watcher"
  tmux -S "$reaper_barrier_socket" wait-for -S reaper-cgroup-hold
  reaper_cgroup_rc=0
  wait "$reaper_cgroup_client" || reaper_cgroup_rc=$?
  equal "the cgroup fixture's parent really was killed outright" \
    "255" "$reaper_cgroup_rc"
  reaper_cgroup_wait_rc=0
  reaper_wait_host_identity "$reaper_cgroup_watcher" "$reaper_cgroup_started" \
    || reaper_cgroup_wait_rc=$?
  equal "the cgroup fixture observes the exact watcher process exit" \
    "0" "$reaper_cgroup_wait_rc"
  equal "a parent execution boundary killed outright leaves no adopted tmux server behind" \
    "" "$(reaper_live_servers "$reaper_cgroup_root" "$reaper_cgroup_socket")"
  if [ -e "$reaper_cgroup_root" ]; then
    fail "and its watcher removes the fixture root" "$reaper_cgroup_root is still there"
  else
    pass "and its watcher removes the fixture root"
  fi
else
  unknown "a watcher survives the cgroup that ends its parent" \
    "this host has no reachable systemd user manager for an isolated fixture"
fi

# A CHILD PID NAMESPACE DIES WITH ITS INIT. Starting the watcher inside that
# namespace therefore gives it the same fate as the parent even after setsid.
# The adopted server below starts outside the child namespace, as a server a
# host-side fixture or an already-running tmux server does. The readiness log
# records both namespace and cgroup identities before the parent is released;
# the done barrier then proves that this exact watcher survived PID 1's death
# long enough to sweep.
reaper_namespace_probe="gangline-reaper-namespace-probe-$reaper_unit_nonce"
reaper_namespace_available=0
systemd-run --user --quiet --wait --pipe --collect --service-type=exec \
  --unit="$reaper_namespace_probe" /usr/bin/unshare -Urpf --mount-proc \
  /bin/true >/dev/null 2>&1 && reaper_namespace_available=1
if [ "$reaper_namespace_available" -eq 1 ]; then
  reaper_namespace_root="$reaper_fix/namespaced-parent"
  reaper_namespace_label="gangline-reaper-namespace-$reaper_unit_nonce"
  reaper_namespace_socket="/tmp/tmux-$(id -u)/$reaper_namespace_label"
  reaper_namespace_unit="gangline-reaper-namespace-parent-$reaper_unit_nonce"
  reaper_namespace_log="$reaper_fix/namespace-watch.log"
  suite_reaper_track "$reaper_namespace_socket"
  env -u TMUX_TMPDIR tmux -L "$reaper_namespace_label" new-session -d \
    -s reaper-namespace -n seed "PS1='> ' bash --norc"
  cat > "$reaper_fix/namespace-parent.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
. "$ROOT/test/suite-python.sh"
. "$ROOT/test/suite-reaper.sh"
export SUITE_REAPER_DONE_SOCKET="$reaper_barrier_socket"
export SUITE_REAPER_DONE_CHANNEL=reaper-namespace-done
export SUITE_REAPER_TMUX="$REAL_TMUX"
export SUITE_REAPER_LOG="$reaper_namespace_log"
suite_reaper_start "$reaper_namespace_root"
suite_reaper_track "$reaper_namespace_socket"
tmux -S "$reaper_barrier_socket" wait-for -S reaper-namespace-up
tmux -S "$reaper_barrier_socket" wait-for reaper-namespace-hold
kill -KILL "\$\$"
SH
  chmod +x "$reaper_fix/namespace-parent.sh"
  systemd-run --user --quiet --wait --pipe --collect --service-type=exec \
    --unit="$reaper_namespace_unit" /usr/bin/unshare -Urpf --mount-proc \
    "$reaper_fix/namespace-parent.sh" &
  reaper_namespace_client=$!
  tmux -S "$reaper_barrier_socket" wait-for reaper-namespace-up
  reaper_namespace_armed="$(awk -F '\t' '$2 == "armed" { print; exit }' \
    "$reaper_namespace_log")"
  reaper_parent_cgroup="$(sed -n 's/.*\tparent-cgroup=\([^\t]*\).*/\1/p' \
    <<<"$reaper_namespace_armed")"
  reaper_watcher_cgroup="$(sed -n 's/.*\twatcher-cgroup=\([^\t]*\).*/\1/p' \
    <<<"$reaper_namespace_armed")"
  reaper_parent_pidns="$(sed -n 's/.*\tparent-pidns=\([^\t]*\).*/\1/p' \
    <<<"$reaper_namespace_armed")"
  reaper_watcher_pidns="$(sed -n 's/.*\twatcher-pidns=\([^\t]*\).*/\1/p' \
    <<<"$reaper_namespace_armed")"
  # source-guard: whole-surface@3388e2a61e37: the armed watcher is the log's only writer before release
  equal "the namespaced parent's watcher is armed from another cgroup" \
    "different" \
    "$([ -n "$reaper_parent_cgroup" ] && [ "$reaper_parent_cgroup" != "$reaper_watcher_cgroup" ] && printf different || printf same)"
  # source-guard: whole-surface@a6710c760af6: the armed watcher is the log's only writer before release
  equal "and from outside the child PID namespace" "different" \
    "$([ -n "$reaper_parent_pidns" ] && [ "$reaper_parent_pidns" != "$reaper_watcher_pidns" ] && printf different || printf same)"
  tmux -S "$reaper_barrier_socket" wait-for -S reaper-namespace-hold
  reaper_namespace_rc=0
  wait "$reaper_namespace_client" || reaper_namespace_rc=$?
  case "$reaper_namespace_rc" in
    0 | 255)
      pass "the namespace fixture reports its forcibly ended init"
      ;;
    *)
      fail "the namespace fixture reports its forcibly ended init" \
        "systemd-run exited $reaper_namespace_rc, expected 0 or 255"
      ;;
  esac
  tmux -S "$reaper_barrier_socket" wait-for reaper-namespace-done
  equal "a child PID namespace ending leaves no adopted tmux server behind" \
    "" "$(reaper_live_servers "$reaper_namespace_root" "$reaper_namespace_socket")"
  if [ -e "$reaper_namespace_root" ]; then
    fail "and its outside watcher removes the fixture root" \
      "$reaper_namespace_root is still there"
  else
    pass "and its outside watcher removes the fixture root"
  fi
else
  unknown "a watcher survives the PID namespace that ends its parent" \
    "this host cannot create the isolated user and PID namespace fixture"
fi

# WHAT A RUN OWNS IS WHAT IT WROTE DOWN. Selection is a prefix of the socket's
# bound path, so a directory argument alone would let one unset variable reach
# the team's own socket. A directory is swept only while it carries the marker a
# run wrote into it.
reaper_unclaimed_root="$reaper_fix/unclaimed"
mkdir -p "$reaper_unclaimed_root"
TMUX_TMPDIR="$reaper_unclaimed_root" tmux new-session -d -s reaper-unclaimed \
  -n seed "PS1='> ' bash --norc"
equal "a server stands in a directory no run has claimed" \
  "1" "$(reaper_live_servers "$reaper_unclaimed_root" "" | wc -l | tr -d ' ')"
suite_reaper_sweep "$reaper_unclaimed_root"
equal "sweeping an unclaimed directory leaves its server standing" \
  "1" "$(reaper_live_servers "$reaper_unclaimed_root" "" | wc -l | tr -d ' ')"
if [ -e "$reaper_unclaimed_root/.suite-reaper" ]; then
  fail "and writes nothing into it" "a marker appeared"
else
  pass "and writes nothing into it"
fi
tmux -S "$reaper_unclaimed_root/tmux-$(id -u)/default" kill-server 2>/dev/null || true

reaper_claimed_root="$reaper_fix/claimed"
mkdir -p "$reaper_claimed_root"
suite_reaper_claim "$reaper_claimed_root" > /dev/null
TMUX_TMPDIR="$reaper_claimed_root" tmux new-session -d -s reaper-claimed \
  -n seed "PS1='> ' bash --norc"
equal "the same sweep ends a server under a directory the run did claim" \
  "" "$(suite_reaper_sweep "$reaper_claimed_root"
        reaper_live_servers "$reaper_claimed_root" "")"

# A CLAIM IS A PROMISE THAT THE DIRECTORY HOLDS NOTHING BUT THIS RUN'S WORK,
# because a sweep ends what is bound inside it and then removes it. A directory
# that already holds someone else's tmux server says otherwise, and so does one
# that contains the directory tmux puts this user's sockets in by default —
# which is where the team's own server lives, and which an unset TMPDIR would
# hand straight to a run root. The protected directory is a fixture here, so
# this drives the refusal without writing into the live one; the live one is
# only read.
if [ -e "/tmp/tmux-$(id -u)/.suite-reaper" ]; then
  fail "the host's own tmux directory carries no run's marker" \
    "/tmp/tmux-$(id -u)/.suite-reaper exists"
else
  pass "the host's own tmux directory carries no run's marker"
fi
reaper_stand_in_host="$reaper_fix/stand-in-host/tmux-$(id -u)"
mkdir -p "$reaper_stand_in_host"
ln -s "$reaper_stand_in_host" "$reaper_fix/stand-in-link"
for reaper_shared in \
  "protects|$reaper_stand_in_host" \
  "protects|${reaper_stand_in_host%/*}" \
  "spelling|$reaper_stand_in_host/." \
  "spelling|$reaper_stand_in_host//" \
  "spelling|${reaper_stand_in_host%/*}/./tmux-$(id -u)" \
  "spelling|$reaper_fix/stand-in-link"
do
  reaper_shared_why="${reaper_shared%%|*}"
  reaper_shared_path="${reaper_shared#*|}"
  reaper_shared_rc=0
  reaper_shared_said="$(
    export SUITE_REAPER_HOST_TMUX="$reaper_stand_in_host"
    suite_reaper_claim "$reaper_shared_path" 2>&1)" || reaper_shared_rc=$?
  equal "claiming [$reaper_shared_path] is refused" "2" "$reaper_shared_rc"
  case "$reaper_shared_why" in
    protects) reaper_shared_needle="holds this host's own tmux sockets" ;;
    *) reaper_shared_needle="does not name a run root" ;;
  esac
  contains "and [$reaper_shared_path] is refused by name" \
    "$reaper_shared_said" "$reaper_shared_needle"
  if [ -e "$reaper_stand_in_host/.suite-reaper" ]; then
    fail "and [$reaper_shared_path] writes no marker into the protected directory" \
      "a marker appeared"
    rm -f "$reaper_stand_in_host/.suite-reaper"
  else
    pass "and [$reaper_shared_path] writes no marker into the protected directory"
  fi
done
reaper_occupied_root="$reaper_fix/occupied"
mkdir -p "$reaper_occupied_root"
TMUX_TMPDIR="$reaper_occupied_root" tmux new-session -d -s reaper-occupied \
  -n seed "PS1='> ' bash --norc"
reaper_occupied_rc=0
reaper_occupied_said="$(suite_reaper_claim "$reaper_occupied_root" 2>&1)" \
  || reaper_occupied_rc=$?
equal "claiming a directory that already holds a server is refused" \
  "2" "$reaper_occupied_rc"
contains "and the refusal says a server is bound under it" \
  "$reaper_occupied_said" "already bound under it"
equal "so that server is still standing" \
  "1" "$(reaper_live_servers "$reaper_occupied_root" "" | wc -l | tr -d ' ')"
tmux -S "$reaper_occupied_root/tmux-$(id -u)/default" kill-server 2>/dev/null || true

# A PATH IS NOT AN IDENTITY: a directory can be removed and remade at the same
# path by a later run. A watcher carries the token it wrote, so it declines the
# successor rather than reaping a run it was never armed for.
reaper_gen_root="$reaper_fix/generation"
mkdir -p "$reaper_gen_root"
reaper_gen_first="$(suite_reaper_claim "$reaper_gen_root")"
reaper_gen_second="$(suite_reaper_claim "$reaper_gen_root")"
if [ -n "$reaper_gen_first" ] && [ "$reaper_gen_first" != "$reaper_gen_second" ]; then
  pass "each claim of the same directory writes a different token"
else
  fail "each claim of the same directory writes a different token" \
    "got [$reaper_gen_first] then [$reaper_gen_second]"
fi
TMUX_TMPDIR="$reaper_gen_root" tmux new-session -d -s reaper-generation \
  -n seed "PS1='> ' bash --norc"
"$(suite_reaper_python)" "$(suite_reaper_program)" \
  --sweep "$reaper_gen_root" "$reaper_gen_first"
equal "a sweep holding the earlier run's token leaves the later run standing" \
  "1" "$(reaper_live_servers "$reaper_gen_root" "" | wc -l | tr -d ' ')"
"$(suite_reaper_python)" "$(suite_reaper_program)" \
  --sweep "$reaper_gen_root" "$reaper_gen_second"
equal "the token the directory actually carries ends it" \
  "" "$(reaper_live_servers "$reaper_gen_root" "")"

# TMPDIR IS THE CALLER'S TO CHOOSE, so a run root can hold a space. The kernel
# writes the bound path as the last field of its socket table and a plain split
# would cut it at that space, losing the server rather than ending it.
reaper_space_root="$reaper_fix/holds a space"
mkdir -p "$reaper_space_root"
suite_reaper_claim "$reaper_space_root" > /dev/null
TMUX_TMPDIR="$reaper_space_root" tmux new-session -d -s reaper-space \
  -n seed "PS1='> ' bash --norc"
equal "a server under a root holding a space is there to be found" \
  "1" "$(reaper_live_servers "$reaper_space_root" "" | wc -l | tr -d ' ')"
suite_reaper_sweep "$reaper_space_root"
equal "and the teardown reaches it" \
  "" "$(reaper_live_servers "$reaper_space_root" "")"

# A NEIGHBOUR'S NAME IS NOT THIS RUN'S TO ENCODE. Finding the servers a root
# holds means reading a name for every process on the host, and those names
# belong to whoever started them. This suite builds fixtures out of raw bytes
# and several runs share a box, so a process whose name is not valid UTF-8 is
# an ordinary neighbour, not a fault. It must not decide whether this run may
# start: claim reads the same names, and the suites exit on a claim that fails.
#
# THE NEIGHBOUR CANNOT OUTLIVE THIS RUN. A process holding a name like that
# refuses every other run on the host until it goes, so it is arranged to die
# of this shell's death rather than of this shell's teardown. The write end is
# opened read-write before the copy starts, which never blocks and means the
# copy's own open cannot block either: from then on the only thing keeping it
# alive is a descriptor that closes when this process does, however it goes.
# Every process started while it is open is started with it closed instead.
# Inherited, it is a writer on the copy's own stdin, so the close below would
# leave the copy reading a pipe nothing can ever end; a tmux server inherits
# it the same way and outlives the run holding it.
reaper_neighbour="$reaper_fix/$(printf 'nm\377x')"
reaper_neighbour_in="$reaper_fix/neighbour-in"
reaper_neighbour_out="$reaper_fix/neighbour-out"
cp "$(command -v cat)" "$reaper_neighbour"
mkfifo "$reaper_neighbour_in" "$reaper_neighbour_out"
exec 7<> "$reaper_neighbour_in"
"$reaper_neighbour" < "$reaper_neighbour_in" > "$reaper_neighbour_out" 7>&- &
reaper_neighbour_pid=$!
printf 'up\n' >&7
# The copy cannot answer before it has been exec'd, and its name is set by the
# exec, so this line returning is the name being there. Nothing is waited on.
IFS= read -r reaper_neighbour_ack < "$reaper_neighbour_out"
equal "the neighbour answers from under a name of its own" \
  "up" "$reaper_neighbour_ack"
reaper_neighbour_named=no
LC_ALL=C grep -q "$(printf '\377')" "/proc/$reaper_neighbour_pid/comm" \
  && reaper_neighbour_named=yes
equal "and that name really is not UTF-8" "yes" "$reaper_neighbour_named"
reaper_neighbour_root="$reaper_fix/beside-a-neighbour"
mkdir -p "$reaper_neighbour_root"
reaper_neighbour_rc=0
reaper_neighbour_token="$(suite_reaper_claim "$reaper_neighbour_root" 2>&1)" \
  || reaper_neighbour_rc=$?
equal "a claim beside a name that is not UTF-8 is not refused" \
  "0" "$reaper_neighbour_rc"
case "$reaper_neighbour_token" in
  '' | *[!0-9a-f]*)
    fail "and what it returns is a token rather than a traceback" \
      "got [$reaper_neighbour_token]" ;;
  *) pass "and what it returns is a token rather than a traceback" ;;
esac
TMUX_TMPDIR="$reaper_neighbour_root" tmux new-session -d -s reaper-neighbour \
  -n seed "PS1='> ' bash --norc" 7>&-
equal "a server under that root is still there to be found" \
  "1" "$(reaper_live_servers "$reaper_neighbour_root" "" | wc -l | tr -d ' ')"
suite_reaper_sweep "$reaper_neighbour_root"
equal "and the sweep beside that neighbour still ends it" \
  "" "$(reaper_live_servers "$reaper_neighbour_root" "")"
exec 7>&-
wait "$reaper_neighbour_pid" 2>/dev/null || true
if kill -0 "$reaper_neighbour_pid" 2>/dev/null; then
  kill -KILL "$reaper_neighbour_pid" 2>/dev/null || true
  fail "the neighbour goes when this run stops holding it open" \
    "pid $reaper_neighbour_pid outlived the descriptor"
else
  pass "the neighbour goes when this run stops holding it open"
fi

# A NEWLINE HAS NO REPRESENTATION in the marker, the adopted-socket list, or the
# kernel's own table, so a run root or socket holding one is refused where it
# enters rather than going quietly missing at teardown.
reaper_newline_root="$(printf '%s/holds a\nnewline' "$reaper_fix")"
reaper_newline_rc=0
reaper_newline_said="$(suite_reaper_start "$reaper_newline_root" 2>&1)" \
  || reaper_newline_rc=$?
equal "a run root holding a newline is refused" "1" "$reaper_newline_rc"
contains "and the refusal names it as unreadable" \
  "$reaper_newline_said" "cannot be a run root"
reaper_newline_rc=0
reaper_newline_said="$(suite_reaper_track "$reaper_newline_root/sock" 2>&1)" \
  || reaper_newline_rc=$?
equal "an adopted socket holding a newline is refused too" "1" "$reaper_newline_rc"

# A SOCKET PATH MAY LEGALLY END IN A SPACE. Recording one and trimming it on the
# way back out records one path and looks for another, so the adopted list keeps
# every byte but the separator.
reaper_extra_root="$reaper_fix/adopted"
mkdir -p "$reaper_extra_root"
suite_reaper_claim "$reaper_extra_root" > /dev/null
reaper_extra_socket="$reaper_fix/outside sock "
( export SUITE_REAPER_ROOT="$reaper_extra_root"
  suite_reaper_track "$reaper_extra_socket" )
equal "an adopted socket path keeps its trailing space" \
  "$reaper_extra_socket" \
  "$(sed -n '1s/$//p' "$reaper_extra_root/.suite-reaper-extra")"

# THE WATCH IS EITHER PROVIDED OR REFUSED. It rests on Linux pidfds, a readable
# socket table, and a systemd user manager; on a host without one a watcher would report
# a protection it cannot give, and the run it was meant to cover would leak in
# exactly the way this file exists to stop.
equal "this host can be watched" "0" \
  "$("$(suite_reaper_python)" "$(suite_reaper_program)" --watchable >/dev/null 2>&1; echo $?)"
reaper_nosockets_rc=0
reaper_nosockets_said="$(
  export SUITE_REAPER_UNIX_SOCKETS="$reaper_fix/absent-table"
  suite_reaper_start "$reaper_fix/unwatchable" 2>&1)" || reaper_nosockets_rc=$?
equal "a host whose socket table cannot be read refuses to start a watch" \
  "1" "$reaper_nosockets_rc"
contains "and says which piece is missing" \
  "$reaper_nosockets_said" "cannot be read"
if [ -e "$reaper_fix/unwatchable/.suite-reaper" ]; then
  fail "a refused start claims nothing" "it wrote a marker anyway"
else
  pass "a refused start claims nothing"
fi
reaper_nosystemd_rc=0
reaper_nosystemd_said="$(
  # Emptying the search path is how a host without systemd-run is presented here;
  # it is confined to this subshell, which does nothing else.
  # shellcheck disable=SC2123
  PATH="$reaper_fix/empty-path"
  suite_reaper_start "$reaper_fix/ungrouped" 2>&1)" || reaper_nosystemd_rc=$?
equal "a host without systemd-run refuses too, rather than sharing the run's boundary" \
  "1" "$reaper_nosystemd_rc"
contains "and says so" "$reaper_nosystemd_said" "no systemd-run"

# A ROOT THAT WOULD NOT GO IS NAMED. Removal is best-effort — a teardown carries
# on either way — but a directory that survives one is a fixture the next run
# inherits, and a removal that failed silently reads exactly like one that
# worked. Declining a directory this caller does not own is not that, and says
# nothing.
reaper_watch_stuck_root="$reaper_fix/watcher-will-not-go"
cat > "$reaper_fix/stuck-parent.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
. "$ROOT/test/suite-python.sh"
. "$ROOT/test/suite-reaper.sh"
export SUITE_REAPER_DONE_SOCKET="$reaper_barrier_socket"
export SUITE_REAPER_DONE_CHANNEL=reaper-stuck-done
export SUITE_REAPER_TMUX="$REAL_TMUX"
suite_reaper_start "$reaper_watch_stuck_root"
mkdir -p "$reaper_watch_stuck_root/held"
: > "$reaper_watch_stuck_root/held/file"
chmod 500 "$reaper_watch_stuck_root/held"
tmux -S "$reaper_barrier_socket" wait-for -S reaper-stuck-up
tmux -S "$reaper_barrier_socket" wait-for reaper-stuck-hold
SH
chmod +x "$reaper_fix/stuck-parent.sh"
"$reaper_fix/stuck-parent.sh" &
reaper_stuck_parent=$!
tmux -S "$reaper_barrier_socket" wait-for reaper-stuck-up
tmux -S "$reaper_barrier_socket" wait-for -S reaper-stuck-hold
wait "$reaper_stuck_parent"
tmux -S "$reaper_barrier_socket" wait-for reaper-stuck-done
if [ -d "$reaper_watch_stuck_root" ]; then
  pass "a root the detached watcher could not remove survives for inspection"
else
  fail "a root the detached watcher could not remove survives for inspection" \
    "$reaper_watch_stuck_root is gone"
fi
reaper_watch_stuck_said=""
if reaper_watch_stuck_line="$(sed -n '1p' \
    "$reaper_watch_stuck_root/.suite-reaper-watch-error" 2>&1)"; then
  reaper_watch_stuck_said="$reaper_watch_stuck_line"
fi
contains "the detached watcher leaves its removal diagnostic in that root" \
  "$reaper_watch_stuck_said" \
  "$reaper_watch_stuck_root could not be removed and is still there to read"
chmod 700 "$reaper_watch_stuck_root/held"
suite_reaper_claim "$reaper_watch_stuck_root" > /dev/null
equal "the watcher-stuck fixture is removable after its permission is restored" \
  "" "$(suite_reaper_sweep "$reaper_watch_stuck_root" 2>&1)"

reaper_stuck_root="$reaper_fix/will-not-go"
mkdir -p "$reaper_stuck_root/held"
: > "$reaper_stuck_root/held/file"
suite_reaper_claim "$reaper_stuck_root" > /dev/null
chmod 500 "$reaper_stuck_root/held"
reaper_stuck_said="$(suite_reaper_sweep "$reaper_stuck_root" 2>&1)"
contains "a run root that could not be removed is named" \
  "$reaper_stuck_said" "$reaper_stuck_root could not be removed"
contains "and the teardown says the sweep did not come back clean" \
  "$reaper_stuck_said" "exited 4"
if [ -d "$reaper_stuck_root" ]; then
  pass "and it really is still there to read"
else
  fail "and it really is still there to read" "$reaper_stuck_root is gone"
fi
chmod 700 "$reaper_stuck_root/held"
suite_reaper_claim "$reaper_stuck_root" > /dev/null
equal "the same teardown says nothing once the root can go" \
  "" "$(suite_reaper_sweep "$reaper_stuck_root" 2>&1)"
if [ -e "$reaper_stuck_root" ]; then
  fail "and the root is gone" "$reaper_stuck_root is still there"
else
  pass "and the root is gone"
fi
reaper_unowned_root="$reaper_fix/not-ours"
mkdir -p "$reaper_unowned_root"
equal "declining a directory this caller does not own says nothing" \
  "" "$(suite_reaper_sweep "$reaper_unowned_root" 2>&1)"
if [ -d "$reaper_unowned_root" ]; then
  pass "and leaves it exactly as it was"
else
  fail "and leaves it exactly as it was" "$reaper_unowned_root was removed"
fi

# A TEARDOWN TRAPPED DIRECTLY ON A SIGNAL RUNS AND THEN RETURNS to the flow it
# interrupted, so the run carries on with its fixtures deleted and its claim on
# its own directory gone — and every server it starts after that point belongs
# to no run and is swept by nobody. Both suites answer a signal with a handler
# that exits, whatever order the dispositions are written in.
for reaper_trapped in test/integration.sh test/role-briefs.sh; do
  equal "$reaper_trapped binds its teardown to no signal" "" \
    "$(grep -E '^trap .*cleanup.*(HUP|INT|TERM|QUIT)' "$ROOT/$reaper_trapped")"
  for reaper_signal in HUP INT TERM; do
    contains "$reaper_trapped answers $reaper_signal with a handler that exits" \
      "$(grep -E "^trap .*$reaper_signal\$" "$ROOT/$reaper_trapped")" "on_signal"
  done
  contains "$reaper_trapped's handler exits on the signal it was given" \
    "$(sed -n '/^on_signal()/,/^}/p' "$ROOT/$reaper_trapped")" 'exit "$((128 + $1))"'
done
