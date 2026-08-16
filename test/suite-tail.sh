# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
#
# THE ONE LINE EVERYONE ACTUALLY READS may not contradict the run. A bare count
# tailing a run that failed reads as green to a person and to an agent, and one
# of each has reported gates-green off it while a FAIL sat further up.
#
# It lives here, in one function both suites call, because the branch that has
# to be right is the one a passing run never takes: a suite cannot fail on
# purpose to prove its own summary, so the summary has to be reachable from a
# fixture that can. A copy per suite is a copy the fixture does not cover.
suite_tail() { # $1 checks, $2 fails, $3 seconds, optional $4 = trailing clause
  local verdict=""
  [ "$2" -eq 0 ] || verdict=", $2 FAIL"
  printf '%s checks%s in %ss%s\n' "$1" "$verdict" "$3" "${4:+ $4}"
}
