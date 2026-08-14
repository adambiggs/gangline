#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Fast executable smoke for pre-push and pull-request CI. The full behavioural
# suite remains test/integration.sh.
set -euo pipefail

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
CONFIG_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-smoke.XXXXXX")"
trap 'rm -rf -- "$CONFIG_ROOT"' EXIT HUP INT TERM

GANG_CONFIG_DIR="$CONFIG_ROOT" "$ROOT/bin/gang" help >/dev/null
GANG_CONFIG_DIR="$CONFIG_ROOT" "$ROOT/bin/gang" collars >/dev/null
GANG_CONFIG_DIR="$CONFIG_ROOT" "$ROOT/bin/gang" roles >/dev/null
GANG_CONFIG_DIR="$CONFIG_ROOT" "$ROOT/bin/gang" config >/dev/null

echo "smoke: command surface passed"
