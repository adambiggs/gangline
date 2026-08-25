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

# NEITHER COLUMN, ON PURPOSE. Some claims can only be settled on a substrate a
# given run may not be standing on, and the two honest answers there are
# "proved" and "could not be proved" — never a pass bought with absent
# evidence. A check that cannot fail on this host is not a check, so it is not
# counted as one; it is named, with the reason and with where the coverage does
# exist. Here for the same reason the summary itself is: the branch that has to
# be right is the one an ordinary green run never takes, so it has to be
# reachable from a fixture that can take it.
suite_unknown() { # $1 description, $2 why this run settles nothing either way
  printf '?    %s\n       %s\n' "$1" "$2"
}

# A BARRIER THIS SUITE WAITED ON AND NOBODY SIGNALLED is neither a failed
# assertion nor a slow box, and it used to be neither of them out loud: the wait
# had no ceiling, so the run simply stopped, printed nothing, and held the
# host's heavy lock until somebody read a process list. The wait is bounded now,
# and the shim that bounds it writes the channel down — a fixture inside a pane
# has nowhere to fail to, so the record has to outlive the pane. This reads that
# record. It lives here for the same reason the summary does: the branch that
# has to be right is the one an ordinary green run never takes, so it has to be
# reachable from a fixture that can take it.
suite_wedged_barriers() { # $1 = the ledger the wait shim appends to
  local wedged_argv wedged_ceiling
  [ -s "$1" ] || return 0
  printf '\nBARRIER(S) NEVER SIGNALLED — a channel this run waited on was cut\n'
  printf 'off at its ceiling rather than answered. This is not a slow box: the\n'
  printf 'wait was bounded so it could be reported instead of parking the run.\n'
  while IFS="$(printf '\t')" read -r wedged_argv wedged_ceiling; do
    printf '  tmux %s (waited %ss)\n' "$wedged_argv" "$wedged_ceiling"
  done < "$1"
}

# And the count rides the one line everyone reads, because a green run with
# unknowns above zero is a different reading from a green run without them.
suite_unknown_clause() { # $1 = how many claims this run could not settle
  [ "$1" -eq 0 ] || printf '(%s unknown — see the ? lines above)' "$1"
}
