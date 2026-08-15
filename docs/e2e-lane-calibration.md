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

## What mutation could not find

Mutating the collar asks whether an assertion notices a broken subject. It says
nothing about an assertion that would pass against a working subject for the
wrong reason, and nothing about the paths where the lane fails to report at all.
Independent review found both classes, and they were the more serious.

**The instrument was aggregated, so the assertions were weaker than their
words.** Every logged request was searched as one blob. The harness also asks
this server to count tokens and to title the session, and both bodies quote the
prompt, so a string the agent's turn never carried could satisfy a check —
including the pair in `midturn`, where the envelope's token and its attribution
were searched for separately and could have come from two different requests.
The log now marks each request with whether it was the agent's own turn and
whether it was answered to the last byte, the lane reads only completed agent
turns, and the attribution is asserted against the same request as the token.

**A pane check that an earlier turn already satisfied.** Booting delivers a
startup contract and the stub answers it, so the reply prefix is on the pane
before the scenario begins. Checking for the bare prefix would have passed with
the held turn's completion never drawn. Every answer now carries the sequence
number of the request it answers, and the stub tells the lane which request it
froze, so the check names one turn's reply.

**Ways to finish without reporting.** Cleanup failure could not redden a run, so
a leaked tmux server and a live harness could outlive a green lane; a signal
handler cleaned up and then let the script keep running against the world it had
just destroyed; the heavy lock, the release of a held turn, and `timeout`'s own
grace period were unbounded; an exhausted read budget was discarded and the next
read allowed to pass; and `test/e2e.sh ""` selected no scenario, asserted
nothing, and exited green. Each is now a failure with a name.

## Calibrating the answers to review

Every assertion added above was then mutated in a tree copy, at its real path,
by the same method. An unmutated control ran green.

| Mutant | Scenario | What died |
|---|---|---|
| answers stop carrying their request number | `turn` | the rendered-completion check, alone |
| the stub never records a completion | `boot` | both request-log checks |
| no request is ever the agent's own turn | `boot` | both request-log checks |
| the dialect requires a field the harness does not send | `boot` | the stub-understood check, and boot never settles |
| cleanup cannot confirm the stub died | `boot` | the world-did-not-come-down check |
| `test/e2e.sh ""` (no mutation) | — | exits 2 instead of green on zero checks |
| one read of budget (no mutation) | `boot` | the bounded waits, then the leak check |
| no mutation (control) | `turn` | nothing |

**The signal handler was compared directly.** Both the fixed lane and a copy
carrying the old `trap teardown EXIT HUP INT TERM` were interrupted mid-scenario
with `TERM` aimed at the lane itself rather than at the `flock` holding the lock.
The fixed lane exits 130 and stops. The old one tears the world down, returns
into the script, and runs straight into the next scenario against it —
`/stub.out: Permission denied`, the temp root having already been cleared. That
is the finding reproduced rather than argued.

## What is proven weakly, and how

**Roots that outlived a clean teardown.** Temp roots holding nothing but
`claude/projects` were found after runs that reported success: the harness
recreates its config directory after removal wins the race. Teardown now waits
for the harness before removing and rechecks afterwards. The wait is a real
instrument — a `claude` process was observed holding `cwd` inside the run root
while a scenario ran, which is what it looks for — but in every run since, it has
returned on its first read. Nothing has yet made it wait, so it is proven live
and not proven load-bearing. The recheck after removal is what would catch a
recurrence.

**The turn filter cannot be shown to redden anything on its own.** Excluding the
harness's side errands narrows what counts as evidence; no assertion exists that
passes only because they are excluded. The mutant that proves the lane consults
the flag at all sets it false for every request, and the request-log checks die.
That the flag is set *correctly* rests on the stub deciding it, not on a test.
