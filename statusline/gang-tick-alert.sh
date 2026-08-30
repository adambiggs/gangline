#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# One-shot body for the dedicated tmux alerts window.
set -euo pipefail

log="${1:-}"
record="" state="" at="" note="" extra=""
if [ -n "$log" ] && [ -r "$log" ] && IFS= read -r record < "$log"; then
  # A failed controller can reach this body after a newer clean controller has
  # replaced the shared log. Snapshot once, then decline before any alert
  # output: a second read could mix the two tick results again.
  IFS=$'\t' read -r state at note extra <<<"$record"
  if [ "$state" = ok ] && [ -z "$extra" ]; then
    case "$at" in
      ''|*[!0-9]*) ;;
      *) case "$note" in 'tick completed for '?*) exit 0 ;; esac ;;
    esac
  fi
else
  record='The tick log could not be read. Run gang status.'
fi
printf '\nGangline cooperative tick alert — %s\n\n' "$(date)"
printf '%s\n' "$record"
printf '\a\n'
exit 1
