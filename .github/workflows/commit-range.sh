#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Print the commit range a commits-workflow event covers, or refuse.
#
#   commit-range.sh BASE HEAD DEFAULT_BRANCH [PUSHED_BRANCH]
#
# BASE is the pull request base or the push's before, and PUSHED_BRANCH the
# branch a push updated. GitHub reports a branch's first push with an all-zero
# before; the commits already on the default branch were checked when they
# landed there, so that push covers what the branch adds to it. The push that
# creates the default branch itself has nothing before it, so it covers every
# commit it carries. A base that is missing here, or a new branch with no
# default branch to fork from, has no range that is not a verdict over
# unrelated history.
set -euo pipefail

[ $# -eq 3 ] || [ $# -eq 4 ] \
  || { echo "usage: commit-range.sh BASE HEAD DEFAULT_BRANCH [PUSHED_BRANCH]" >&2; exit 2; }
base="$1" head="$2" default="$3" pushed="${4:-}"
zero=0000000000000000000000000000000000000000

if [ "$base" = "$zero" ] && [ -n "$default" ] && [ "$pushed" = "$default" ]; then
  printf '%s\n' "$head"
  exit 0
fi
if [ "$base" = "$zero" ] && [ -n "$default" ] \
   && git rev-parse --verify --quiet "refs/remotes/origin/$default^{commit}" >/dev/null; then
  base="$(git merge-base "refs/remotes/origin/$default" "$head")" || base=""
fi
if [ -n "$base" ] && [ "$base" != "$zero" ] \
   && git cat-file -e "${base}^{commit}" 2>/dev/null; then
  printf '%s..%s\n' "$base" "$head"
  exit 0
fi
echo "commits: refusing — the event has no usable base, so its commit range is indeterminate" >&2
exit 1
