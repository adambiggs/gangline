#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Every shell file in the repo, parsed and linted. CI and the pre-push hook both
# run THIS, because a second copy of the list is a list that drifts, and the
# drift is silent in the worst direction: the shorter copy stays green over a
# file it never opened. That is not hypothetical — the local gate was three
# files against CI's eight globs, and the two warnings that turned CI red lived
# in test/integration.sh, which only CI's copy named.
set -euo pipefail

cd "$(dirname "$0")/.."

# site/demo/record.sh ships in the repo and was unchecked until a bug in it cost
# two demo takes. Every shell file here is checked or none is.
#
# .githooks is globbed rather than listed: hooksPath points the whole directory
# at git, so a hook added later is a shell file this repo runs, and it should
# not also need an edit here to be read.
files="bin/gang install.sh profiles/*.sh statusline/*.sh test/*.sh .githooks/* site/demo/record.sh .github/workflows/*.sh"

# A gate that cannot run must not report that it passed.
command -v shellcheck >/dev/null 2>&1 || {
  echo "lint: shellcheck is not installed, so the lint CI runs cannot run here" >&2
  exit 1
}

# bash -n reads one script per call; shellcheck takes the set.
for f in $files; do bash -n "$f"; done
shellcheck -S warning $files
