#!/usr/bin/env bash
# Repeats the tmux exit and hitch tests beside the full suite until a deadline,
# keeping every failing package's output. Runs only on the hunt branch.
set -u
out="$1"
deadline=$(( $(date +%s) + $2 ))
focus='Exit|Interrupt|AcquireTree|StartupInput|HoldFailed|Spawn|Launch|Release'
mkdir -p "$out"
round=0
while [ "$(date +%s)" -lt "$deadline" ]; do
  round=$((round + 1))
  go test -count=1 -timeout=90s -skip '^Test(CommandLifecycleOnPrivateTmux|LiveClaudeCode|LiveCodex)$' ./... > "$out/full-$round.txt" 2>&1 &
  full=$!
  go test -count=5 -timeout=170s -run "$focus" ./substrate/tmux ./acceptance > "$out/focus-$round.txt" 2>&1
  focus_status=$?
  wait "$full"
  full_status=$?
  echo "round $round full=$full_status focus=$focus_status" | tee -a "$out/rounds.txt"
  [ "$full_status" -eq 0 ] && rm -f "$out/full-$round.txt"
  [ "$focus_status" -eq 0 ] && rm -f "$out/focus-$round.txt"
done
grep -h -E -e '--- FAIL|panic:|_test.go:[0-9]+:' "$out"/full-*.txt "$out"/focus-*.txt 2>/dev/null | sort | uniq -c | sort -rn > "$out/summary.txt"
cat "$out/summary.txt"
ls "$out"
! grep -q -v ' full=0 focus=0$' "$out/rounds.txt"
