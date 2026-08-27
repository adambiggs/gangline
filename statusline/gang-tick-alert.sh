#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# One-shot body for the dedicated tmux alerts window.
set -euo pipefail

log="${1:-}"
printf '\nGangline cooperative tick alert — %s\n\n' "$(date)"
if [ -n "$log" ] && [ -r "$log" ]; then
  cat "$log"
else
  printf 'The tick log could not be read. Run gang status.\n'
fi
printf '\a\n'
exit 1
