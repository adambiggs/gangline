#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Fast executable smoke for pre-push and pull-request CI. The full behavioural
# suite remains test/integration.sh.
set -euo pipefail

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
CONFIG_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-smoke.XXXXXX")"
trap 'rm -rf -- "$CONFIG_ROOT"' EXIT HUP INT TERM

# EVERY ROOT THIS FILE COULD WRITE THROUGH IS PRIVATE, not only the one the
# checks below happen to name. The commands here read and write nothing outside
# the config directory today, but `gang up` archives against whatever
# GANG_LOCK_DIR resolves to, and its default is the operator's own
# /tmp/gangline-$(id -u) — so a session-shaped check added later would act on
# the live team's locks and spools. Exported once here rather than left to a
# line each new check remembers to copy, because the check that forgets is the
# one that does the damage.
export GANG_LOCK_DIR="$CONFIG_ROOT/lock"
export GANG_ARCHIVE_DIR="$CONFIG_ROOT/archive"

# Smoke is routinely invoked from an attached development pane. The command
# surface is its subject; a cooperative pass against that pane's live team is
# not. Use the same suite-only switch as integration so no snapshot executable
# can mutate the operator's tmux session on EXIT.
export GANG_TEST_COLLARS=1
export GANG_TEST_TICK_MODE=manual

GANG_CONFIG_DIR="$CONFIG_ROOT" "$ROOT/bin/gang" help >/dev/null
GANG_CONFIG_DIR="$CONFIG_ROOT" "$ROOT/bin/gang" collars >/dev/null
GANG_CONFIG_DIR="$CONFIG_ROOT" "$ROOT/bin/gang" models -c claude-code \
  >/dev/null 2>&1
GANG_CONFIG_DIR="$CONFIG_ROOT" "$ROOT/bin/gang" roles >/dev/null
GANG_CONFIG_DIR="$CONFIG_ROOT" "$ROOT/bin/gang" config >/dev/null

echo "smoke: command surface passed"
