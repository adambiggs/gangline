# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# test/gate.sh itself: the tree it judges, the snapshot it takes, and the evidence a failure owes.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
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
printf '%s\n' "\$@" > "$gate_flock_args"
[ "\${1:-}" = -o ] || exit 91
shift
[ "\${1:-}" = /tmp/gangline-heavy.lock ] || exit 92
shift
exec "\$@"
SH
chmod +x "$gate_flock_bin/flock"
gate_default_out="$(env -u _GANGLINE_GATE_LOCKED \
  PATH="$gate_flock_bin:$PATH" "$gate_run/test/gate.sh" 2>&1)"
equal "the ordinary gate owns a close-on-exec heavy-test lock" \
  "$(printf '%s\n' -o /tmp/gangline-heavy.lock "$gate_run/test/gate.sh")" \
  "$(<"$gate_flock_args")"
equal "the no-argument gate runs lint, smoke, and the suite exactly once" \
  "$(printf 'integration\nlint\nsmoke')" "$(sort "$gate_order")"
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
checks=0 fails=0
pass() { checks=$((checks + 1)); }
fail() { checks=$((checks + 1)); fails=$((fails + 1)); }
pass; pass; pass
[ "${1:-}" != red ] || { fail; fail; }
suite_tail "$checks" "$fails" 7 "${2:-}"
SH
chmod +x "$tail_fix/run.sh"
equal "a suite whose checks all passed ends on a bare count" \
  "3 checks in 7s" "$("$tail_fix/run.sh")"
equal "and a suite that accumulated failures names them in that same line" \
  "5 checks, 2 FAIL in 7s" "$("$tail_fix/run.sh" red)"
equal "with the e2e lane's trailing clause after the verdict, not instead of it" \
  "5 checks, 2 FAIL in 7s against harness-x" \
  "$("$tail_fix/run.sh" red "against harness-x")"

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
