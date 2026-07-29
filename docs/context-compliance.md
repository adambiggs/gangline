# Context-compliance measurement

This instrument measures whether context-band notes reach an actionable seam and
what is known when context later drops. It does not add an escalation tier,
change a note, trigger compaction, or alter warning deduplication.

Run the retained-data analysis with:

```sh
gang context-report
```

The report always co-reports `proven-compliant`, `proven-non-compliant`, and
`could-not-determine`. It prints no compliance rate. If CTD dominates, or the
retained data has a structural gap, its verdict is `THIS DATASET CANNOT ANSWER
THE QUESTION`.

## Records and questions

Rows are versioned TSV records. Values are percent-encoded, so one append is one
physical line. Every row carries `ts`, `kind`, `session`, `window_id`, `agent`,
`profile`, `leg`, `hook_event`, `seam`, `tokens`, `window`, `band`, `threshold`,
and the complete resolved `thresholds` vector. A band index is therefore
interpretable after harness window sizes or ladder defaults change.

| Record or field | What it answers |
| --- | --- |
| `liveness`, `first_success` | Did this agent window complete an atomic logger write before it had a measured event? A liveness-only dataset means the logger worked and no measured event occurred. No liveness and no valid event row is CTD, never evidence of a quiet week. |
| `note`, `leg`, `hook_event`, `seam`, `note_count` | Which leg actually delivered each note, whether delivery was at patrol/UserPromptSubmit (`yes`) or PostToolUse (`no`), and its ordinal since the prior context drop. Patrol rows follow verified pane injection; hook rows follow JSON actually written to stdout. |
| `compact_request`, `requester`, `issue_tokens`, `first_threshold` | Who asked gang to submit compaction and the context `A` and first rung `F` at issue time. A request is not proof that the harness executed it. |
| `context_drop`, `pre_*`, `sample_staleness`, `sample_quality` | The last successful sample before the observed drop, its timestamp and age, and the post-drop observation. `pre_tokens` is not the true unsampled peak or exact tokens-at-compaction. |
| `notes_since_drop`, `last_note_ts` | How many delivered notes preceded the drop and which retained note should be the last one. A mismatch with retained note rows is CTD. |
| `provenance`, `provenance_candidates`, `request_quality` | Whether cause is independently established as self-issued or peer-issued, or remains among harness auto-compaction and unknown causes. `@gl_compacting` establishes a request only; it never proves execution. |
| `evidence_availability`, `evidence_result`, `issue_tokens`, `first_threshold` | Whether patrol's independent drop proof was structurally available. With the current branch, it is unavailable when `A <= 2F`; that CTD is kept distinct from available evidence that did not establish cause (issue #22). |
| `read_ctd`, `reason`, `prior_log_gap` | Which read, state, intended oversized row, or logger interval could not be determined. These records enter neither proven verdict. |
| `retention_gap` | Where bounded rotation deleted an older generation. Counts after that point are lower bounds and the report says so. |

From those fields, `gang context-report` prints the last sampled tokens and band
at every context drop; notes since the preceding drop; the last delivery leg;
whether its seam delivery was first or Nth; PostToolUse note count; and bands
spent by a non-seam note with no seam note following. These are the five study
questions, not facts an analyst must reconstruct from raw rows.

## Failure, bounds, and deletion

Each row is at most 4096 bytes and is appended with one `write(2)` to an
`O_APPEND` descriptor. Rotation uses an adjacent `fcntl` lock, so a killed writer
cannot leave a stale lock. The active file is bounded by
`GANG_CONTEXT_LOG_MAX_BYTES` (8 MiB by default) and exactly one rotation is
retained. A superseded rotation creates a `retention_gap` row.

The default path is
`${XDG_STATE_HOME:-$HOME/.local/state}/gangline/context-events.tsv`; override it
with `GANG_CONTEXT_LOG`. A failed write is banked on the agent window, printed by
patrol, passed into `gang context-report`, and copied into the next successful
row. A live `@gl_context_log_error` is structural CTD: it makes the report refuse
to present an otherwise well-formed retained file as a complete dataset. A first
write is retried until its persistent liveness row succeeds.

Delete every persistent artifact explicitly with:

```sh
gang context-report --clear
```

That removes the active file, its `.1` rotation, and the adjacent `.lock`, then
clears live logger-error and liveness options. Per-window band, sample, and
request state remains tmux window state and dies when that window is deleted.
