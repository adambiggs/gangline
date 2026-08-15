# Calibration record — the offline e2e lane

Dated 2026-08-15, against claude-code 2.1.233, on branch `e2e-stub`.

A green suite is a claim about a tree only if its assertions can go red. This
records what was mutated, what died, and what the exercise found wrong with the
lane itself. It is a historical record. Re-run the calibration rather than
trusting these results after either the lane or the collar moves.

## Method

Each case copies the working tree, installs one mutant at its real path in the
copy — `collars/claude-code.sh`, the file the lane exists to check — and runs
the scenario that should notice. A case that leaves the file unchanged is
reported as "did not land" and given no verdict, because a mutation that never
applied looks exactly like a mutant that survived. An unmutated control runs the
same way, so a copy that reddens everything by itself would be visible; it came
back green.

The first attempt got two of its own cases wrong: the Stop substitution never
matched, and the composer-glyph pattern was written as hex escapes `grep` does
not interpret. Both were reported as clean survivals. The did-not-land check
exists because of that.

## What the mutants proved

| Mutant | Scenario | Result |
|---|---|---|
| `collar_bricked`'s expected message reworded | `bricked` | died — 3 assertions |
| native `Stop` hook wired to `/bin/true` instead of gang | `turn` | died — the wait never returns |
| native `Stop` renamed to an event the harness does not know | `turn` | died, but see below |
| `GANG_MIDTURN_INPUT=steer` → `park` | `midturn` | died — 1 assertion |
| the composer glyph `collar_input` reads | `boot` | died — hitch finds no box |
| no mutation (control) | `turn` | green |

**The Stop mutant that matters is the silent one.** Renaming the event makes the
harness reject the whole settings file and draw a startup dialog, so the lane
reddens on a boot failure and proves nothing about turn boundaries. Pointing a
still-valid `Stop` entry at `/bin/true` leaves the harness healthy and only stops
the hook reaching Gangline. Under it the turn still runs, the pane still renders
the completion, and the two assertions that fail are exactly `gang wait --until
done` returning and the agent reading idle afterwards. That is the evidence that
those two readings are carried by the native hook and by nothing else.

## What calibration found wrong with the lane

**A weak assertion.** Under the reworded-message mutant the state assertion
failed while a second check — that the cause named `model_not_found` — still
passed. Both verdicts the transcript reader can reach mention that name: the
fatal one, and the "unrecognized message shape" it falls back to when the
sentence drifts. The check now requires the rejection wording, which only the
fatal branch produces, and it now dies with the mutant.

**A hang, not a failure.** The renamed-`Stop` mutant makes the harness draw a
settings-error dialog at startup. `hitch` correctly refuses to answer a native
first-run prompt on the operator's behalf and keeps waiting for a composer —
right at a keyboard, a hang in an unattended lane. The run sat there until an
outer timeout killed it, reporting nothing. `hitch` is now bounded inside the
lane and prints the frame that stopped it; the same mutant now reports
`hitch never reached a composer (status 124)`.

**A hold that fired on the wrong request.** Not a mutant but the same exercise.
claude-code issues an auxiliary session-title completion per submitted prompt,
quoting the user's message and arriving before the real turn. A stub keying on
prompt text alone froze that one instead, and every assertion still passed while
the turn under test ran unheld — because a request log records arrival, not
completion. The lane now identifies the agent's own turn by the standing
contract, holds at most once per run, and asserts the agent returns to rest.
