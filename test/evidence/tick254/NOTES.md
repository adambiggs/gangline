# Issue #254 — tick can read a dropped pane from a stale roster

Base: main 6720dc60c9ef8868066f067e078205f82a8224c2. No overlapping open or
closed PR found (`gh pr list --search 254`, and a search for
`tick_full_pass_once` across PRs/issues, both empty). Issue #239 (closed) is
unrelated: its fix (765f2ba) repaints a visited window's glyph and is not
about a roster entry vanishing mid-pass.

## Root cause

`tick_full_pass_once` (bin/gang) snapshots `tmux list-windows` once, then for
each saved window id runs a sequence of pane reads/writes — `busy` (guarding
self-compaction and cache-compaction eligibility), a `tmux set-option` that
claims delivery ownership, and a final `state_now` read that repaints the
glyph. If `gang drop <id>` removes the window between the snapshot and any of
these, two low-level primitives called from `busy()` — `busy_painted` and
`activity_hold_clear` — treated "target no longer resolves" the same as
"target exists but is unreadable" and called `die`. Because several of
`busy()`'s callers invoke it directly rather than inside a command
substitution, that `die` was not contained to the one roster entry.

Confirmed reproduction: a deterministic test seam placed right after
`tick_full_pass_once` reads a window as present (past `launch_dead`) and
before its later pane operations, paired with a real `gang drop` of that
window while the pass is paused there. Against the unfixed code this reliably
produced the alert `tick could not read the state of tick-stale-victim:
cannot clear the activity-only bound on pane @N — refusing to fabricate its
state` — the same wording reported in the issue.

## Fix

- `window_gone()` (bin/gang, near `busy_painted`): `tmux list-panes -t "$1"`
  reliably fails once a window is gone. `tmux display-message` does not — it
  expands an unresolvable target to an empty string and still exits 0 (the
  reason `launch_dead` already uses `list-panes` instead), so the helper
  deliberately avoids `display-message`.
- `busy_painted` and `activity_hold_clear` now check `window_gone` before
  dying: gone → set global `WINDOW_GONE=1` and return a harmless value
  (not-busy / cleared); still present but unreadable → `die` exactly as
  before. `WINDOW_GONE` is declared at top level (bound under `set -u`) and
  reset at the top of `busy()`, so every direct caller of `busy()` can check
  it immediately after the call without `busy()` growing a fourth return
  code that its many other call sites would have to learn.
- `tick_full_pass_once` and `cache_compaction_candidate` check `WINDOW_GONE`
  right after each `busy()` call and stop touching that window id for the
  rest of the iteration (`continue`/`return 1`) instead of proceeding.
- The `@gl_tick_delivery` ownership write checks `window_gone` on failure and
  skips silently when gone, instead of alerting.
- A new test seam (`GANG_TEST_TICK_VICTIM` /
  `GANG_TEST_TICK_VICTIM_READY_FIFO` / `_RELEASE_FIFO`) fires once per pass,
  gated on the visited agent's name, right after `launch_dead` confirms the
  window present and before any further pane operation for it.

## Evidence: red before, green after (test/integration-tick.sh, new block)

Same test, same assertions, run via `GANG_INTEGRATION_PARTS=cli,tick
test/integration.sh` (the `tick` part depends on `cli`) against a committed
snapshot each time (`test/integration.sh` refuses a dirty tree):

- **Red** (seam present, fix logic reverted): `no activity-bound alert names
  the vanished entry` — **FAIL**, `unexpected [cannot clear the
  activity-only bound]`; full log kept at
  `/tmp/tick254-red-run5.log` (not committed; host-local, ephemeral).
- **Green** (fix applied): all six new assertions pass — `the normal drop
  path still succeeds while a pass is mid-walk on it`, `the pass carrying the
  vanished entry still completes cleanly`, `no unreadable-pane alert names
  the vanished entry`, `no activity-bound alert names the vanished entry`,
  `no delivery-ownership alert names the vanished entry`, `the still-live
  roster entry is visited in the same pass`. Full scoped run: 1367 checks, no
  failures, `RUN ENDED EARLY` did not fire (kept at
  `/tmp/tick254-green-run2.log`, host-local, ephemeral).
- `test/gate.sh` (lint + smoke, the mandatory per-change gate) passed
  separately via `gang run -- test/gate.sh` on the fixed tree.

An earlier attempt at `window_gone()` used `tmux display-message`, matching
an existing precedent elsewhere in `tick_full_pass_once`'s own end-of-loop
guard; the red/green cycle above caught it failing to suppress the alert
(display-message's "resolves to nothing but still exits 0" behavior), which
is why `window_gone` uses `list-panes` instead. A first pass at the fix also
missed that `WINDOW_GONE` needed a top-level `""` initialization — under
`set -u` an unguarded `[ -z "$WINDOW_GONE" ]` before `busy()` had ever run
died with "unbound variable" partway through the green confirmation run; that
surfaced from the same scoped integration run, not from `gate.sh` (lint/smoke
alone would not have exercised this path).

## Cross-provider review (codex, `tick254review`, reviewing 2c76ae6/17ad745)

Verdict: CONCERNS. Confirmed sound: `window_gone`'s use of `list-panes` for
window IDs (server-global, no cross-session ambiguity), the WINDOW_GONE
same-call freshness (busy() resets it at entry, no reentrancy found), the
fail-closed branch (window_gone false still reaches the original `die`), and
that the regression test is non-vacuous (proved the victim passed
`launch_dead`, then a real `gang drop`, then the roster continued).

Real bug found: `busy_painted` returning 1 (its "not painted" value) means
either "genuinely idle" or "gone" — `WINDOW_GONE=1` is set in the latter case,
but `busy()` did not check the flag before falling through to
`decay_witness`/`recently_active`/`composer_live`, which could still die on
the same gone window (`decay_witness`'s own die, and `composer_live`'s
`live_rc -eq 3` die in the `snapshot` branch `state_now` uses). Fixed by
returning immediately (`[ -z "$WINDOW_GONE" ] || return 1`) right after the
`busy_painted` if-block, and by adding the same window_gone-before-die guard
to the `decay_witness` and `composer_live` die sites inside `busy()` — the
two the reviewer named as still reachable. Every OTHER call in `busy()` that
can set `WINDOW_GONE` (via `activity_hold_clear`) already returns
immediately afterward, so this closes the one live gap the review found in
`busy()` itself.

Not fixed, and still flagged by the review as open: `window_gone`'s
"list-panes failed" boolean does not distinguish "window resolved to
nothing" from "tmux itself failed" (transport/socket error) — a genuinely
present-but-unreadable window could misclassify as gone under that narrower
failure mode. Accepted as a residual gap alongside the ones below rather
than fixed, given the evidenced reproduction only exercises the
"target no longer resolves" case.

## Unproven / residual risk

- `tick_identity_verify`, `harness_identity_tick_backfill`,
  `harness_identity_verify` were not individually audited for die-on-gone-window:
  they only call collar-declared functions (`collar_live_session_id`,
  `collar_harness_identity`) `if declare -F ... >/dev/null`. The disposable
  test collar used here (`tick-native`) does not declare them, so the new
  test seam does not exercise this risk. A real collar (Claude Code, Codex)
  that does declare them is unaudited for this specific race.
- `occupied()`, `cache_bands_read`, `context_light_read`, and the rest of
  `cache_compaction_candidate`'s body past its own `busy` call were not
  audited for other die sites on a gone window; the fix only stops the
  *known* one (its own `busy` call) from being reached.
- `launch_dead()` treats "tmux list-panes target unresolvable" the same as
  "pane exists, remain-on-exit corpse" (both `return 0`, i.e. "dead") — a
  fully-gone window at that specific check reads as "dead" rather than
  "gone," which still routes to `glyph_write`/`state_note` against a gone
  window. Not fixed here: the evidenced reproduction needs the window to
  survive past that check (it is the seam's placement), and widening the fix
  to cover this too was out of scope for a minimal, evidence-driven repair.
  Recorded as a related latent gap.
