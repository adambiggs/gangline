#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/gangline-rebase-worktrees.XXXXXX")"
trap 'rm -rf -- "$FIXTURE"' EXIT HUP INT TERM

failures=0

equal() {
  local label="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    printf 'ok   %s\n' "$label"
  else
    printf 'FAIL %s\n       expected [%s]\n       actual   [%s]\n' \
      "$label" "$expected" "$actual" >&2
    failures=$((failures + 1))
  fi
}

contains() {
  local label="$1" text="$2" needle="$3"
  case "$text" in
    *"$needle"*) printf 'ok   %s\n' "$label" ;;
    *) printf 'FAIL %s\n       missing [%s]\n' "$label" "$needle" >&2
       failures=$((failures + 1)) ;;
  esac
}

remote="$FIXTURE/origin.git"
source="$FIXTURE/source"
clean="$FIXTURE/clean"
dirty="$FIXTURE/dirty"
conflict="$FIXTURE/conflict"
signfail="$FIXTURE/signfail"
hidden="$FIXTURE/hidden"
bisecting="$FIXTURE/bisecting"
detached="$FIXTURE/detached"
prunable="$FIXTURE/prunable"
git init --bare -q "$remote"
git clone -q "$remote" "$source"
printf 'base\n' > "$source/README"
git -C "$source" add README
git -C "$source" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: root'
git -C "$source" branch -M main
git -C "$remote" fetch -q "$source" main:refs/heads/main
git -C "$source" fetch -q origin main:refs/remotes/origin/main
git -C "$source" branch --set-upstream-to=origin/main main
git -C "$source" worktree add -q -b clean "$clean" main
git -C "$source" worktree add -q -b dirty "$dirty" main
git -C "$source" worktree add -q -b conflict "$conflict" main
git -C "$source" worktree add -q -b signfail "$signfail" main
git -C "$source" worktree add -q -b hidden "$hidden" main
git -C "$source" worktree add -q -b bisecting "$bisecting" main
git -C "$source" worktree add -q --detach "$detached" main
git -C "$source" worktree add -q -b prunable "$prunable" main

printf 'remote one\n' >> "$source/README"
git -C "$source" add README
git -C "$source" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: remote advance one'
printf 'remote two\n' >> "$source/README"
git -C "$source" add README
git -C "$source" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: remote advance two'
git -C "$remote" fetch -q "$source" main:refs/heads/main

printf 'leave me alone\n' > "$dirty/UNCOMMITTED"
printf 'hidden local edit\n' > "$hidden/README"
git -C "$hidden" update-index --assume-unchanged README
printf 'local side\n' > "$conflict/README"
git -C "$conflict" add README
git -C "$conflict" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: local conflict'
conflict_head="$(git -C "$conflict" rev-parse HEAD)"
printf 'local signed commit\n' > "$signfail/LOCAL"
git -C "$signfail" add LOCAL
git -C "$signfail" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: local signed commit'
signfail_head="$(git -C "$signfail" rev-parse HEAD)"
signfail_gpg="$FIXTURE/signfail-gpg"
cat > "$signfail_gpg" <<'SH'
#!/usr/bin/env bash
exit 1
SH
chmod +x "$signfail_gpg"
git -C "$signfail" config commit.gpgSign true
git -C "$signfail" config gpg.program "$signfail_gpg"
git -C "$bisecting" bisect start main HEAD >/dev/null
bisect_head="$(git -C "$bisecting" rev-parse HEAD)"
detached_head="$(git -C "$detached" rev-parse HEAD)"
rm -rf -- "$prunable"

run_rc=0
run_out="$("$ROOT/tools/rebase-worktrees" "$source" 2>&1)" || run_rc=$?
equal "a skipped dirty or conflicting worktree makes the lead command nonzero" 1 "$run_rc"
contains "a clean stale worktree is rebased" "$run_out" "$clean: rebased"
contains "a dirty worktree is named and skipped" "$run_out" "$dirty: skipped-dirty"
contains "an index-hidden edit is named and skipped" "$run_out" "$hidden: skipped-dirty"
contains "a bisect is named and skipped" "$run_out" "$bisecting: skipped-dirty (BISECT_LOG)"
contains "a detached inspection worktree is named and skipped" "$run_out" "$detached: skipped-dirty (detached HEAD)"
contains "a missing administrative worktree is named prunable" "$run_out" "$prunable: skipped-dirty (prunable)"
contains "a conflicting worktree is named and skipped" "$run_out" "$conflict: skipped-conflict"
contains "an owned rebase failure says that it was safely aborted" "$run_out" \
  "$signfail: skipped-conflict (this invocation began and aborted its own rebase)"
contains "the main worktree is named current" "$run_out" "$source: already-current"
equal "the clean worktree now has origin main at HEAD" \
  "$(git -C "$clean" rev-parse origin/main)" "$(git -C "$clean" rev-parse HEAD)"
equal "a dirty worktree keeps its original uncommitted bytes" 'leave me alone' \
  "$(<"$dirty/UNCOMMITTED")"
equal "an index-hidden edit keeps its original bytes" 'hidden local edit' \
  "$(<"$hidden/README")"
equal "a bisect stays on the commit it selected" \
  "$bisect_head" "$(git -C "$bisecting" rev-parse HEAD)"
equal "a detached inspection worktree stays on its pinned commit" \
  "$detached_head" "$(git -C "$detached" rev-parse HEAD)"
equal "a conflicting rebase returns the exact pre-rebase head" \
  "$conflict_head" "$(git -C "$conflict" rev-parse HEAD)"
equal "a signing-failed rebase returns the exact pre-rebase head" \
  "$signfail_head" "$(git -C "$signfail" rev-parse HEAD)"
equal "a conflicting rebase leaves no unresolved index or worktree change" "" \
  "$(git -C "$conflict" status --porcelain)"
equal "a conflicting rebase leaves no operation behind" absent \
  "$([ ! -e "$(git -C "$conflict" rev-parse --git-path rebase-merge)" ] && [ ! -e "$(git -C "$conflict" rev-parse --git-path rebase-apply)" ] && printf absent || printf present)"

# The lock serializes independent invocations before either can fetch or rebase
# from the shared Git metadata.
lock_bin="$FIXTURE/lock-bin"
lock_log="$FIXTURE/lock.log"
source_common="$(git -C "$source" rev-parse --path-format=absolute --git-common-dir)"
REAL_FLOCK="$(command -v flock)"
mkdir -p "$lock_bin"
cat > "$lock_bin/flock" <<SH
#!/usr/bin/env bash
readlink "/proc/\$PPID/fd/\${1:?}" > '$lock_log'
exec '$REAL_FLOCK' "\$@"
SH
chmod +x "$lock_bin/flock"
lock_rc=0
PATH="$lock_bin:$PATH" "$ROOT/tools/rebase-worktrees" "$source" >/dev/null 2>&1 || lock_rc=$?
equal "a skipped worktree keeps the lock probe nonzero" 1 "$lock_rc"
# source-guard: producer@9584670e68dd: the PATH-local flock wrapper is the only lock_log writer and resolves the tool process's descriptor before delegating to flock.
equal "the lead tool acquires the shared repository lock" \
  "$source_common/rebase-worktrees.lock" "$(<"$lock_log")"

# A second rebase can begin after the clean seam but before this tool invokes
# Git. Its staged resolution belongs to that other operation: the tool must
# name the collision without aborting or otherwise changing it.
race_remote="$FIXTURE/race-origin.git"
race_source="$FIXTURE/race-source"
race_manual="$FIXTURE/race-manual"
race="$FIXTURE/race"
git init --bare -q "$race_remote"
git clone -q "$race_remote" "$race_source"
printf 'base\n' > "$race_source/README"
git -C "$race_source" add README
git -C "$race_source" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: race root'
git -C "$race_source" branch -M main
git -C "$race_remote" fetch -q "$race_source" main:refs/heads/main
git -C "$race_source" fetch -q origin main:refs/remotes/origin/main
git -C "$race_source" branch --set-upstream-to=origin/main main
git -C "$race_source" branch manualwork main
git -C "$race_source" worktree add -q "$race_manual" manualwork
git -C "$race_source" worktree add -q -b race "$race" main
printf 'manual side\n' > "$race_manual/README"
git -C "$race_manual" add README
git -C "$race_manual" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: manual conflict'
printf 'hold manual branch\n' > "$race_manual/UNCOMMITTED"
printf 'race side\n' > "$race/README"
git -C "$race" add README
git -C "$race" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: race conflict'
printf 'remote side\n' > "$race_source/README"
git -C "$race_source" add README
git -C "$race_source" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid \
  commit -qm 'test: race remote advance'
git -C "$race_remote" fetch -q "$race_source" main:refs/heads/main
race_git_bin="$FIXTURE/race-git-bin"
race_injected="$FIXTURE/race-injected"
REAL_GIT="$(command -v git)"
mkdir -p "$race_git_bin"
cat > "$race_git_bin/git" <<SH
#!/usr/bin/env bash
if [ "\${1:-}" = -C ] && [ "\${2:-}" = '$race' ] \\
    && [ "\${3:-}" = rebase ] && [ "\${4:-}" = --interactive ] \\
    && [ "\${5:-}" = origin/main ] \\
    && [ ! -e '$race_injected' ]; then
  : > '$race_injected'
  env -u REBASE_WORKTREES_SEQUENCE_MARKER -u GIT_SEQUENCE_EDITOR \
    '$REAL_GIT' -C '$race' rebase manualwork >/dev/null 2>&1 || :
  printf 'manual resolution\\n' > '$race/README'
  '$REAL_GIT' -C '$race' add README
fi
exec '$REAL_GIT' "\$@"
SH
chmod +x "$race_git_bin/git"
race_rc=0
race_out="$(PATH="$race_git_bin:$PATH" "$ROOT/tools/rebase-worktrees" "$race_source" 2>&1)" || race_rc=$?
equal "an operation appearing after the seam makes the lead command nonzero" 1 "$race_rc"
contains "the later operation is named rather than aborted" "$race_out" \
  "$race: skipped-conflict (a Git operation began before this rebase started)"
equal "the other rebase keeps its staged manual resolution" 'manual resolution' \
  "$(git -C "$race" show :README)"
equal "the other rebase remains in progress for its owner" present \
  "$([ -e "$(git -C "$race" rev-parse --git-path rebase-merge)" ] && printf present || printf absent)"
if [ -e "$(git -C "$race" rev-parse --git-path rebase-merge)" ]; then
  git -C "$race" rebase --abort
fi

[ "$failures" -eq 0 ] || exit 1
