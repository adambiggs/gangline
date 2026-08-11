#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

cd "$(dirname "$0")/.."
CHECKER="${SOURCE_GUARD_CHECKER:-$PWD/test/source-guards.py}"
FIXTURE_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-source-guards.XXXXXX")"
trap 'rm -rf -- "$FIXTURE_ROOT"' EXIT

checks=0
fails=0
CHECK_STATUS=0
CHECK_OUTPUT=""

pass() { checks=$((checks + 1)); printf 'ok   %s\n' "$1"; }
fail() { checks=$((checks + 1)); fails=$((fails + 1)); printf 'FAIL %s\n' "$1" >&2; }

check() {
  set +e
  CHECK_OUTPUT="$(python3 "$CHECKER" "$@" 2>&1)"
  CHECK_STATUS=$?
  set -e
}

accepts() { # $1 description, $2 fixture
  check --ledger /dev/null "$2"
  if [ "$CHECK_STATUS" -eq 0 ]; then pass "$1"; else fail "$1"; fi
}

refusal_matches() { # $1 fixture, optional $2 expected diagnostic
  local fixture="$1" expected="${2:-combined surface}"
  check --ledger /dev/null "$fixture"
  if [ "$CHECK_STATUS" -eq 1 ] &&
     grep -Fq "$fixture:" <<< "$CHECK_OUTPUT" &&
     grep -Fq "$expected" <<< "$CHECK_OUTPUT"; then
    return 0
  fi
  return 1
}

refuses() { # $1 description, $2 fixture, optional $3 expected diagnostic
  if refusal_matches "$2" "${3:-combined surface}"; then pass "$1"; else fail "$1"; fi
}

cat > "$FIXTURE_ROOT/direct.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
contains "delivery landed" "$(pane alpha)" "MARK_DELIVERED"
SH
refuses "a direct combined-pane containment is rejected" "$FIXTURE_ROOT/direct.sh" "pane"

cat > "$FIXTURE_ROOT/literal.sh" <<'SH'
contains "delivery landed" "$(tmux capture-pane -pJ -t alpha)" "MARK_DELIVERED"
SH
refuses "literal tmux capture-pane is a producer" "$FIXTURE_ROOT/literal.sh" "tmux capture-pane"

cat > "$FIXTURE_ROOT/gang-capture.sh" <<'SH'
contains "delivery landed" "$(TMUX_PANE=%1 "$GANG" capture)" "MARK_DELIVERED"
SH
refuses "Gangline capture output is a combined surface" "$FIXTURE_ROOT/gang-capture.sh" "gang capture"

cat > "$FIXTURE_ROOT/alias.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
captured="$(pane alpha)"
evidence="$captured"
contains "delivery landed" "$evidence" "MARK_DELIVERED"
SH
refuses "capture taint follows assignments" "$FIXTURE_ROOT/alias.sh" "pane"

cat > "$FIXTURE_ROOT/parameter.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
captured="$(pane alpha)"
trimmed="prefix ${captured#*prefix} suffix"
contains "delivery landed" "$trimmed" "MARK_DELIVERED"
SH
refuses "parameter expansion and concatenation retain taint" "$FIXTURE_ROOT/parameter.sh" "pane"

cat > "$FIXTURE_ROOT/wrapper.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
screen() { pane "$1"; }
evidence="$(screen alpha)"
contains "delivery landed" "$evidence" "MARK_DELIVERED"
SH
refuses "capture-producer taint follows wrapper functions" "$FIXTURE_ROOT/wrapper.sh" "screen"

cat > "$FIXTURE_ROOT/multiline-wrapper.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
screen() {
  local target="$1"
  pane "$target"
}
evidence="$(screen alpha)"
contains "delivery landed" "$evidence" "MARK_DELIVERED"
SH
refuses "producer closure reads commands in multiline wrapper bodies" \
  "$FIXTURE_ROOT/multiline-wrapper.sh" "screen"

cat > "$FIXTURE_ROOT/assertion-wrapper.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
shows() { contains "$1" "$2" "$3"; }
shows "delivery landed" "$(pane alpha)" "MARK_DELIVERED"
SH
refuses "positive assertion wrappers remain guarded" "$FIXTURE_ROOT/assertion-wrapper.sh" "pane"

cat > "$FIXTURE_ROOT/multiline.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
captured="$(
  pane alpha
)"
[ -n "$captured" ] && contains "delivery landed" "$captured" "MARK_DELIVERED"
SH
refuses "multiline substitutions and command prefixes remain guarded" "$FIXTURE_ROOT/multiline.sh" "pane"

cat > "$FIXTURE_ROOT/elif.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
if false; then :
elif contains "delivery landed" "$(pane alpha)" "MARK_DELIVERED"; then :
fi
SH
refuses "an elif positive guard remains classified" "$FIXTURE_ROOT/elif.sh" "pane"

cat > "$FIXTURE_ROOT/heredoc.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
captured="$(pane alpha)"
filtered="$(sed 's/prefix//' <<EOF
$captured
EOF
)"
contains "delivery landed" "$filtered" "MARK_DELIVERED"
SH
refuses "heredoc transformations retain taint" "$FIXTURE_ROOT/heredoc.sh" "pane"

cat > "$FIXTURE_ROOT/backticks.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
contains "delivery landed" "`pane alpha`" "MARK_DELIVERED"
SH
refuses "legacy command substitutions remain guarded" "$FIXTURE_ROOT/backticks.sh" "pane"

cat > "$FIXTURE_ROOT/equal.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
equal "pane is exact" "MARK_DELIVERED" "$(pane alpha)"
SH
refuses "exact equality is a positive combined-surface guard" "$FIXTURE_ROOT/equal.sh" "pane"

cat > "$FIXTURE_ROOT/case.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
captured="$(pane alpha)"
case "$captured" in
  *MARK_DELIVERED*) pass "delivery landed" ;;
esac
SH
refuses "case-pattern containment is guarded" "$FIXTURE_ROOT/case.sh" "pane"

cat > "$FIXTURE_ROOT/grep.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
grep -Fq "MARK_DELIVERED" <<< "$(pane alpha)" && printf 'ok delivery landed\n'
SH
refuses "quiet grep is a positive combined-surface guard" "$FIXTURE_ROOT/grep.sh" "pane"

cat > "$FIXTURE_ROOT/bracket.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
captured="$(pane alpha)"
[[ "$captured" == *MARK_DELIVERED* ]] && printf 'ok delivery landed\n'
SH
refuses "Bash pattern equality is a positive combined-surface guard" \
  "$FIXTURE_ROOT/bracket.sh" "pane"

cat > "$FIXTURE_ROOT/bracket-mixed.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
captured="$(pane alpha)"
[[ "$captured" == *MARK_DELIVERED* && "$USER" != root ]] && printf 'ok delivery landed\n'
SH
refuses "an unrelated negative conjunct cannot hide a positive Bash match" \
  "$FIXTURE_ROOT/bracket-mixed.sh" "pane"

cat > "$FIXTURE_ROOT/read-transfer.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
captured="$(pane alpha)"
read -r line <<< "$captured"
contains "delivery landed" "$line" "MARK_DELIVERED"
SH
refuses "read transfers capture taint to its destination" \
  "$FIXTURE_ROOT/read-transfer.sh" "pane"

cat > "$FIXTURE_ROOT/mapfile-transfer.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
captured="$(pane alpha)"
mapfile -t lines <<< "$captured"
contains "delivery landed" "${lines[0]}" "MARK_DELIVERED"
SH
refuses "mapfile transfers capture taint to its array" \
  "$FIXTURE_ROOT/mapfile-transfer.sh" "pane"

cat > "$FIXTURE_ROOT/printf-v-transfer.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
captured="$(pane alpha)"
printf -v copied '%s' "$captured"
contains "delivery landed" "$copied" "MARK_DELIVERED"
SH
refuses "printf -v transfers capture taint to its destination" \
  "$FIXTURE_ROOT/printf-v-transfer.sh" "pane"

cat > "$FIXTURE_ROOT/file-transfer.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
pane alpha > /tmp/source-guard-captured-pane
copied="$(cat /tmp/source-guard-captured-pane)"
contains "delivery landed" "$copied" "MARK_DELIVERED"
SH
refuses "a literal file round-trip retains capture provenance" \
  "$FIXTURE_ROOT/file-transfer.sh" "pane"

cat > "$FIXTURE_ROOT/file-shorthand.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
pane alpha > /tmp/source-guard-captured-pane
contains "delivery landed" "$(< /tmp/source-guard-captured-pane)" "MARK_DELIVERED"
SH
refuses "Bash file-read shorthand retains capture provenance" \
  "$FIXTURE_ROOT/file-shorthand.sh" "pane"

cat > "$FIXTURE_ROOT/file-grep.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
pane alpha > /tmp/source-guard-captured-pane
grep -q MARK_DELIVERED /tmp/source-guard-captured-pane && printf 'ok delivery landed\n'
SH
refuses "quiet grep retains literal-file capture provenance" \
  "$FIXTURE_ROOT/file-grep.sh" "pane"

cat > "$FIXTURE_ROOT/custom-assertion.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
shows() { case "$2" in *"$3"*) printf 'ok %s\n' "$1" ;; *) printf 'FAIL %s\n' "$1" ;; esac; }
shows "delivery landed" "$(pane alpha)" "MARK_DELIVERED"
SH
refuses "a positive case-based assertion wrapper is discovered" \
  "$FIXTURE_ROOT/custom-assertion.sh" "pane"

cat > "$FIXTURE_ROOT/multiple-single.sh" <<'SH'
value="$(single_source_reader)" pane_text="$(pane alpha)"
contains "file value lands" "$value" "MARK_FILE"
SH
accepts "a first assignment is not tainted by a later pane assignment" "$FIXTURE_ROOT/multiple-single.sh"

cat > "$FIXTURE_ROOT/multiple-pane.sh" <<'SH'
value="$(single_source_reader)" pane_text="$(pane alpha)"
contains "pane value lands" "$pane_text" "MARK_PANE"
SH
refuses "each assignment on one line keeps its own source" "$FIXTURE_ROOT/multiple-pane.sh" "pane"

cat > "$FIXTURE_ROOT/named.sh" <<'SH'
run_transcript="$(some_reader)"
contains "report names success" "$run_transcript" "MARK_SUCCESS"
SH
refuses "source-shaped transcript names enter the dataflow" "$FIXTURE_ROOT/named.sh" "name:run_transcript"

cat > "$FIXTURE_ROOT/named-argument.sh" <<'SH'
contains "report names success" "$run_transcript" "MARK_SUCCESS"
SH
refuses "a source-shaped assertion argument is guarded directly" \
  "$FIXTURE_ROOT/named-argument.sh" "name:run_transcript"

cat > "$FIXTURE_ROOT/named-alias.sh" <<'SH'
run_transcript="$(single_source_reader)"
evidence="$run_transcript"
contains "report names success" "$evidence" "MARK_SUCCESS"
SH
refuses "source-shaped assignment taint survives a generic alias" \
  "$FIXTURE_ROOT/named-alias.sh" "name:run_transcript"

cat > "$FIXTURE_ROOT/reassigned.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
evidence="$(pane alpha)"
evidence="$(single_source_reader)"
contains "single report names success" "$evidence" "MARK_SUCCESS"
SH
accepts "single-source reassignment clears old capture taint" "$FIXTURE_ROOT/reassigned.sh"

cat > "$FIXTURE_ROOT/local-reset.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
evidence="$(pane alpha)"
local evidence
contains "new local value is empty" "$evidence" "MARK_SUCCESS"
SH
accepts "a bare local declaration clears outer capture taint" "$FIXTURE_ROOT/local-reset.sh"

cat > "$FIXTURE_ROOT/function-definition.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
shows() { contains "function body" "$(pane alpha)" "MARK_SUCCESS"; }
SH
accepts "defining an assertion wrapper is not executing its guard" \
  "$FIXTURE_ROOT/function-definition.sh"

cat > "$FIXTURE_ROOT/negative.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
excludes "nothing was delivered" "$(pane alpha)" "MARK_FORBIDDEN"
SH
accepts "negative guards get stricter when another source appears" "$FIXTURE_ROOT/negative.sh"

cat > "$FIXTURE_ROOT/negative-shell-forms.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
! contains "delivery is absent" "$(pane alpha)" "MARK_FORBIDDEN"
! grep -Fq "MARK_FORBIDDEN" <<< "$(pane alpha)"
captured="$(pane alpha)"
[[ "$captured" != *MARK_FORBIDDEN* ]] && printf 'ok absent\n'
case "$captured" in
  *MARK_FORBIDDEN*) fail "forbidden text is absent" ;;
  *) pass "forbidden text is absent" ;;
esac
SH
accepts "negated shell forms are not mislabeled as positive guards" \
  "$FIXTURE_ROOT/negative-shell-forms.sh"

cat > "$FIXTURE_ROOT/single.sh" <<'SH'
command_output="$(single_source_command)"
contains "command reports success" "$command_output" "MARK_SUCCESS"
SH
accepts "ordinary single-source output is outside this gate" "$FIXTURE_ROOT/single.sh"

cat > "$FIXTURE_ROOT/selector.sh" <<'SH'
contains "whoami names the agent" "$(TMUX_PANE="$(window_id alpha)" gang whoami)" "agent: alpha"
SH
accepts "a pane ID used only as a selector is not pane evidence" "$FIXTURE_ROOT/selector.sh"

mkdir -p "$FIXTURE_ROOT/discovery"
printf '%s\n' '# benign discovery control' > "$FIXTURE_ROOT/discovery/plain.sh"
cp "$FIXTURE_ROOT/direct.sh" "$FIXTURE_ROOT/discovery/zz-new-test.sh"
check --ledger /dev/null --discover "$FIXTURE_ROOT/discovery"
if [ "$CHECK_STATUS" -eq 1 ] &&
   grep -Fq 'zz-new-test.sh:' <<< "$CHECK_OUTPUT"; then
  pass "discovery scans a newly added shell test"
else
  fail "discovery scans a newly added shell test"
fi
mv "$FIXTURE_ROOT/discovery/zz-new-test.sh" \
  "$FIXTURE_ROOT/discovery/source-guards-fixtures.sh"
check --ledger /dev/null --discover "$FIXTURE_ROOT/discovery"
if [ "$CHECK_STATUS" -eq 0 ]; then
  pass "discovery excludes only its own heredoc fixture corpus"
else
  fail "discovery excludes only its own heredoc fixture corpus"
fi

mkdir -p "$FIXTURE_ROOT/empty-discovery"
check --ledger /dev/null --discover "$FIXTURE_ROOT/empty-discovery"
if [ "$CHECK_STATUS" -eq 2 ] && grep -Fq 'found no shell tests' <<< "$CHECK_OUTPUT"; then
  pass "an empty discovery root cannot report green"
else
  fail "an empty discovery root cannot report green"
fi
check --ledger /dev/null --discover "$FIXTURE_ROOT/missing-discovery"
if [ "$CHECK_STATUS" -eq 2 ] && grep -Fq 'does not exist' <<< "$CHECK_OUTPUT"; then
  pass "a missing discovery root cannot report green"
else
  fail "a missing discovery root cannot report green"
fi
check --ledger /dev/null --discover "$FIXTURE_ROOT/discovery" "$FIXTURE_ROOT/direct.sh"
if [ "$CHECK_STATUS" -eq 2 ] && grep -Fq 'mutually exclusive' <<< "$CHECK_OUTPUT"; then
  pass "discovery and an explicit file cannot silently compete"
else
  fail "discovery and an explicit file cannot silently compete"
fi

# Emit-ledger intentionally produces a file that cannot be used until a person
# records a decision. This keeps regeneration from becoming wholesale blessing.
python3 "$CHECKER" --emit-ledger "$FIXTURE_ROOT/direct.sh" > "$FIXTURE_ROOT/pending.allow"
check --ledger "$FIXTURE_ROOT/pending.allow" "$FIXTURE_ROOT/direct.sh"
if [ "$CHECK_STATUS" -eq 2 ] && grep -Fq 'REVIEW-NOTE' <<< "$CHECK_OUTPUT"; then
  pass "an emitted ledger is red until its review note is written"
else
  fail "an emitted ledger is red until its review note is written"
fi
sed 's/REVIEW_REQUIRED/reviewed fixture has an execution-only witness/' \
  "$FIXTURE_ROOT/pending.allow" > "$FIXTURE_ROOT/direct.allow"
check --ledger "$FIXTURE_ROOT/direct.allow" "$FIXTURE_ROOT/direct.sh"
if [ "$CHECK_STATUS" -eq 0 ]; then
  pass "a reviewed fingerprint admits its recorded occurrence"
else
  fail "a reviewed fingerprint admits its recorded occurrence"
fi
printf '%s\n' 'contains "delivery landed" "$(pane alpha)" "MARK_DELIVERED"' \
  >> "$FIXTURE_ROOT/direct.sh"
check --ledger "$FIXTURE_ROOT/direct.allow" "$FIXTURE_ROOT/direct.sh"
if [ "$CHECK_STATUS" -eq 1 ] && grep -Fq 'combined surface' <<< "$CHECK_OUTPUT"; then
  pass "copying a reviewed guard exceeds the fingerprint multiset"
else
  fail "copying a reviewed guard exceeds the fingerprint multiset"
fi
: > "$FIXTURE_ROOT/removed.sh"
check --ledger "$FIXTURE_ROOT/direct.allow" "$FIXTURE_ROOT/removed.sh"
if [ "$CHECK_STATUS" -eq 1 ] && grep -Fq 'stale reviewed fingerprint' <<< "$CHECK_OUTPUT"; then
  pass "removing a reviewed guard makes its ledger entry stale"
else
  fail "removing a reviewed guard makes its ledger entry stale"
fi

cat > "$FIXTURE_ROOT/annotated-source.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
contains "operator sees warning" "$(pane alpha)" "MARK_WARNING"
SH
check --ledger /dev/null "$FIXTURE_ROOT/annotated-source.sh"
token="$(grep -Eo 'producer@[0-9a-f]{12}' <<< "$CHECK_OUTPUT" | head -1 | cut -d@ -f2)"
cat > "$FIXTURE_ROOT/annotated.sh" <<SH
pane() { tmux capture-pane -pJ -t "\$1"; }
# source-guard: whole-surface@$token: every visible producer is valid evidence here
contains "operator sees warning" "\$(pane alpha)" "MARK_WARNING"
SH
accepts "a reasoned whole-surface classification is accepted" "$FIXTURE_ROOT/annotated.sh"
sed 's/MARK_WARNING/MARK_CHANGED/' "$FIXTURE_ROOT/annotated.sh" > "$FIXTURE_ROOT/stale-annotation.sh"
refuses "editing an annotated assertion invalidates its fingerprint" \
  "$FIXTURE_ROOT/stale-annotation.sh" "fingerprint no longer matches"

cat > "$FIXTURE_ROOT/invalid-kind.sh" <<SH
pane() { tmux capture-pane -pJ -t "\$1"; }
# source-guard: trust-me@$token: this kind is outside the classification contract
contains "operator sees warning" "\$(pane alpha)" "MARK_WARNING"
SH
refuses "an unknown annotation kind grants no exemption" "$FIXTURE_ROOT/invalid-kind.sh"

cat > "$FIXTURE_ROOT/short-reason.sh" <<SH
pane() { tmux capture-pane -pJ -t "\$1"; }
# source-guard: producer@$token: too short
contains "operator sees warning" "\$(pane alpha)" "MARK_WARNING"
SH
refuses "a correct fingerprint cannot excuse a vacuous rationale" \
  "$FIXTURE_ROOT/short-reason.sh"

cat > "$FIXTURE_ROOT/blank-separated.sh" <<SH
pane() { tmux capture-pane -pJ -t "\$1"; }
# source-guard: producer@$token: an execution artifact independently binds this claim

contains "operator sees warning" "\$(pane alpha)" "MARK_WARNING"
SH
refuses "an annotation is adjacent, not file-scoped across a blank" \
  "$FIXTURE_ROOT/blank-separated.sh"

cat > "$FIXTURE_ROOT/comment-separated.sh" <<SH
pane() { tmux capture-pane -pJ -t "\$1"; }
# source-guard: producer@$token: an execution artifact independently binds this claim
# unrelated explanation breaks adjacency
contains "operator sees warning" "\$(pane alpha)" "MARK_WARNING"
SH
refuses "an annotation cannot cross an unrelated comment" \
  "$FIXTURE_ROOT/comment-separated.sh"

cat > "$FIXTURE_ROOT/stray-source-comment.sh" <<SH
pane() { tmux capture-pane -pJ -t "\$1"; }
# source-guard: producer@$token: an execution artifact independently binds this claim
# source-guard: this malformed replacement must clear the valid annotation
contains "operator sees warning" "\$(pane alpha)" "MARK_WARNING"
SH
refuses "a malformed replacement cannot inherit an earlier annotation" \
  "$FIXTURE_ROOT/stray-source-comment.sh"

cat > "$FIXTURE_ROOT/weak.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
# source-guard: whole-surface@000000000000: intended
contains "operator sees warning" "$(pane alpha)" "MARK_WARNING"
SH
refuses "a vague or stale label is not an exemption" "$FIXTURE_ROOT/weak.sh"

cat > "$FIXTURE_ROOT/first-source.sh" <<'SH'
pane() { tmux capture-pane -pJ -t "$1"; }
contains "first warning" "$(pane alpha)" "MARK_FIRST"
SH
check --ledger /dev/null "$FIXTURE_ROOT/first-source.sh"
first_token="$(grep -Eo 'producer@[0-9a-f]{12}' <<< "$CHECK_OUTPUT" | head -1 | cut -d@ -f2)"
cat > "$FIXTURE_ROOT/scoped.sh" <<SH
pane() { tmux capture-pane -pJ -t "\$1"; }
# source-guard: producer@$first_token: an execution artifact independently binds this claim
contains "first warning" "\$(pane alpha)" "MARK_FIRST"
contains "first warning" "\$(pane alpha)" "MARK_FIRST"
SH
refuses "one annotation cannot blanket a later identical guard" \
  "$FIXTURE_ROOT/scoped.sh" "first warning"

mkdir -p "$FIXTURE_ROOT/a" "$FIXTURE_ROOT/b"
cp "$FIXTURE_ROOT/annotated-source.sh" "$FIXTURE_ROOT/a/same.sh"
cp "$FIXTURE_ROOT/annotated-source.sh" "$FIXTURE_ROOT/b/same.sh"
python3 "$CHECKER" --emit-ledger "$FIXTURE_ROOT/a/same.sh" |
  sed 's/REVIEW_REQUIRED/reviewed fixture has an execution-only witness/' \
  > "$FIXTURE_ROOT/path.allow"
check --ledger "$FIXTURE_ROOT/path.allow" "$FIXTURE_ROOT/b/same.sh"
if [ "$CHECK_STATUS" -eq 1 ] && grep -Fq 'combined surface' <<< "$CHECK_OUTPUT"; then
  pass "identical guards in different files have different fingerprints"
else
  fail "identical guards in different files have different fingerprints"
fi

printf '%s\n' 'not-a-ledger-entry' > "$FIXTURE_ROOT/malformed.allow"
check --ledger "$FIXTURE_ROOT/malformed.allow" "$FIXTURE_ROOT/direct.sh"
if [ "$CHECK_STATUS" -eq 2 ] && grep -Fq 'expected DIGEST' <<< "$CHECK_OUTPUT"; then
  pass "a malformed ledger fails loud with status two"
else
  fail "a malformed ledger fails loud with status two"
fi

direct_digest="$(cut -f1 "$FIXTURE_ROOT/direct.allow")"
printf '%s\t%s\t%s\n' "$direct_digest" 0 \
  'reviewed fixture has an execution-only witness' > "$FIXTURE_ROOT/zero-count.allow"
check --ledger "$FIXTURE_ROOT/zero-count.allow" "$FIXTURE_ROOT/direct.sh"
if [ "$CHECK_STATUS" -eq 2 ] && grep -Fq 'expected DIGEST' <<< "$CHECK_OUTPUT"; then
  pass "a ledger occurrence count must be positive"
else
  fail "a ledger occurrence count must be positive"
fi
printf '%s\t%s\t%s\n' "$direct_digest" 1 short > "$FIXTURE_ROOT/short-note.allow"
check --ledger "$FIXTURE_ROOT/short-note.allow" "$FIXTURE_ROOT/direct.sh"
if [ "$CHECK_STATUS" -eq 2 ] && grep -Fq 'expected DIGEST' <<< "$CHECK_OUTPUT"; then
  pass "a ledger review note must carry a decision"
else
  fail "a ledger review note must carry a decision"
fi

# The helper itself distinguishes an ordinary policy refusal (1) from a broken
# checker (2); otherwise every refusal fixture could pass against sys.exit(2).
cat > "$FIXTURE_ROOT/broken.py" <<'PY'
#!/usr/bin/env python3
raise SystemExit(2)
PY
real_checker="$CHECKER"
CHECKER="$FIXTURE_ROOT/broken.py"
if refusal_matches "$FIXTURE_ROOT/direct.sh"; then
  fail "a broken checker cannot satisfy refusal fixtures"
else
  pass "a broken checker cannot satisfy refusal fixtures"
fi
CHECKER="$real_checker"

# Canonical repo-relative fingerprints must not depend on the spelling of the
# scanned paths. Use the complete set because the ledger is intentionally stale
# if even one reviewed migration site is omitted.
repo_files=()
for file in test/*.sh; do
  [ "$file" = test/source-guards-fixtures.sh ] && continue
  repo_files+=("$file")
done
check --ledger "$PWD/test/source-guards.allow" "${repo_files[@]}"
relative_status=$CHECK_STATUS
absolute_files=()
for file in "${repo_files[@]}"; do absolute_files+=("$PWD/$file"); done
check --ledger "$PWD/test/source-guards.allow" "${absolute_files[@]}"
if [ "$relative_status" -eq 0 ] && [ "$CHECK_STATUS" -eq 0 ]; then
  pass "repo fingerprints are stable across relative and absolute paths"
else
  fail "repo fingerprints are stable across relative and absolute paths"
fi

printf '%s\n' "$checks source-guard fixture checks"
[ "$fails" -eq 0 ] || {
  printf '%s\n' "$fails source-guard fixture checks failed" >&2
  exit 1
}
