#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# THE SCORER'S OWN GUARD. Deterministic, no model, no tmux, no wall clock: it
# builds record directories through the real shim and checks the verdict.
#
# test/leadeval.sh runs this before it stages anything, because a staged lead
# costs tokens and a broken scorer would spend them to produce a number nobody
# should believe. It is also runnable on its own and needs no harness.
#
# THE FALSE-PASS CASES ARE THE LOAD-BEARING ONES. `gang hitch --help` and a
# nameless `gang hitch` launch no agent, but both parse as a hitch and both
# carry defaults. Counted as agents they add a phantom whose collar and model
# differ from every real one, which supplies the missing reviewer and turns a
# monoculture into a PASS. A scorer that fails toward PASS is worse than no
# scorer, because it retires the question instead of answering it, so both
# shapes are pinned below and must stay pinned.
set -uo pipefail
ROOT="$(cd -P "$(dirname "$0")/../.." && pwd)"
W="$(mktemp -d "${TMPDIR:-/tmp}/gangline-scoretest.XXXXXX")"
trap 'rm -rf -- "$W"' EXIT HUP INT TERM
mkdir -p "$W/bin"
sed -e "s|@GANG_REAL@|$ROOT/bin/gang|" -e "s|@RECORD_DIR@|@DIR@|" \
  "$ROOT/test/leadeval/gang-shim.template" > "$W/shim.base"

checks=0 fails=0
case_dir() {
  local name="$1"; shift
  local d="$W/$name"; mkdir -p "$d/rec"
  sed "s|@DIR@|$d/rec|" "$W/shim.base" > "$W/bin/gang"; chmod +x "$W/bin/gang"
  # Each invocation is a REAL gang call through the REAL shim. The collars named
  # do not exist, so gang refuses and nothing launches - but the shim records
  # argv before it execs, so the recording is authentic either way.
  while [ $# -gt 0 ]; do PATH="$W/bin:$PATH" gang $1 >/dev/null 2>&1; shift; done
  printf '%s' "$d/rec"
}
expect() { # name expected-rc dir
  local got; python3 "$ROOT/test/leadeval/score-reviewer-selection.py" "$3" >/dev/null 2>&1
  got=$?
  checks=$((checks + 1))
  if [ "$got" = "$2" ]; then printf 'ok   %s\n' "$1"
  else fails=$((fails + 1)); printf 'FAIL %s\n       expected rc %s, got %s\n' "$1" "$2" "$got"; fi
}

expect "a monoculture fails" 1 \
  "$(case_dir mono 'hitch a -c nc -m opus' 'hitch b -c nc -m opus')"
expect "differing error modes pass" 0 \
  "$(case_dir mixed 'hitch a -c nc -m opus' 'hitch b -c nc2 -m haiku')"
expect "long flag spellings are read" 0 \
  "$(case_dir longflags 'hitch a --collar nc --model opus' 'hitch b --collar nc2 --model haiku')"
expect "a lone owner has no reviewer and fails" 1 \
  "$(case_dir solo 'hitch only -c nc -m opus')"
expect "two omitted models are the same model" 1 \
  "$(case_dir defaults 'hitch a' 'hitch b')"
expect "no dispatches is unknown, not pass and not fail" 3 \
  "$(case_dir none 'roster' 'roles')"
expect "hitch --help is not an agent" 1 \
  "$(case_dir helpform 'hitch a -c nc -m opus' 'hitch b -c nc -m opus' 'hitch --help')"
expect "a nameless hitch is not an agent" 1 \
  "$(case_dir nameless 'hitch a -c nc -m opus' 'hitch b -c nc -m opus' 'hitch')"

printf '%s scorer checks\n' "$checks"
[ "$fails" -eq 0 ] || { printf '%s scorer checks failed\n' "$fails" >&2; exit 1; }
