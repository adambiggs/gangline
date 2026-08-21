# Mandatory gate runtime measurement — 2026-08-21

> Status: Completed on 2026-08-21; retained as a dated performance record.

## Finding

The mandatory gate had exceeded its five-minute ceiling without reporting the
rule violation. Four complete runs observed before this work took 335s, 351s,
353s, and 355s: 35–55s over budget.

## Measurement method

One snapshot of the unchanged tree was profiled. `/usr/bin/time` measured the
snapshot, lint, smoke, and integration phases. A disposable ShellCheck wrapper
recorded each file invocation, and boundary timestamps bracketed the existing
sourced integration parts. The same disposable snapshot was then used for
candidate clock-call counts and assertion-interval diagnostics. None of those
probes was committed.

The first integration probe was invoked under the heavy lock without exporting
the marker the ordinary gate exports. Its final gate-selftest consequently
waited on the lock held by its own parent. That blocked tail and the incomplete
integration total are not evidence and are excluded below. The completed part
timings precede that mistake and remain valid; the exact disposable session and
fixture root were stopped and removed.

## Baseline ranking

The phase ranking made the critical path unambiguous:

| Rank | Phase | Elapsed |
|---:|---|---:|
| 1 | integration | more than 335.55s before its final two parts |
| 2 | lint | 34.72s |
| 3 | smoke | 0.53s |
| 4 | snapshot construction | 0.32s |

Within the completed integration path, the existing sourced parts ranked:

| Rank | Part | Elapsed |
|---:|---|---:|
| 1 | `integration-hooks.sh` | 87.196s |
| 2 | `integration-spool.sh` | 73.891s |
| 3 | `integration-hitch.sh` | 50.376s |
| 4 | `integration-readiness.sh` | 47.237s |
| 5 | `integration-compose.sh` | 34.148s |
| 6 | `integration-substrate.sh` | 31.229s |
| 7 | `integration-cli.sh` | 11.472s |

No individual case explained the overrun. The longest diagnostic
inter-assertion intervals were distributed across independent fixture worlds:

| Rank | Interval ending at | Part | Elapsed |
|---:|---|---|---:|
| 1 | spool identity mint skips occupied identities | spool | 3.212s |
| 2 | relative yellow threshold resolves in a smaller native window | hooks | 2.519s |
| 3 | porcelain instrument rejects decorated human output | spool | 2.287s |
| 4 | attached prompt clears and `gang up` delivers | hitch | 2.271s |
| 5 | preemption fixture establishes three parked messages | spool | 2.254s |
| 6 | forced spool token redraw is witnessed | spool | 2.152s |
| 7 | mail prints the first waiting body | spool | 2.076s |
| 8 | roster propagates an observation failure | hitch | 2.026s |
| 9 | hard-failed replacement leaves three messages waiting | spool | 2.016s |
| 10 | roster carries an automatic-resume refusal | hooks | 2.007s |

ShellCheck was not the critical path once overlap was possible. Its slowest
files were `bin/gang` at 8.920s, `integration-hooks.sh` at 5.240s, and
`integration-spool.sh` at 2.396s. The lint phase peaked at 1,099,776 KiB while a
separately timed integration run peaked at 14,604 KiB. Their isolated maxima
sum to 1,114,380 KiB; the authoritative aggregate proof remains the documented
2 GiB `systemd-run` command on a host exposing the user service manager.

## Change

No assertion, case, or fixture was removed or collapsed.

- The settled-composer fake clock is immediate. Its fixture supplies every
  between-read change explicitly, so the former 0.3s production wait added no
  evidence.
- Post-keystroke pane reaction retains a 0.01s floor. The measured quiet-pane
  reaction is about 0.005s, so each floor has 2x margin and the five-read test
  budget has 10x margin. Production retains its five 0.4s reads.
- The focused role instrument and gate-selftest run beside the substrate parts.
  Each already owns an independent fixture root; the role instrument also owns
  a separate tmux server. Their output and counters rejoin the same verdict.
- Full lint runs beside smoke and integration over the immutable gate snapshot.
  Its output is kept contiguous and its status is joined before the gate verdict.

The first candidate, which changed only the fake clock and overlapped the role
instrument, still took 341.75s and was rejected as insufficient.

## Load conditions and observed spread

This work did not have an isolated machine. The arc began under explicit notice
that two other agents were working on the box, and they were neither paused nor
coordinated around the measurements. The shared heavy-test lock proves that no
second Gangline gate or end-to-end lane ran at the same time; it does not
serialize builds, analysis, filesystem traffic, or any other work those agents
performed.

The sandbox's process namespace could not enumerate host-wide work, so the
other agents' exact CPU, memory, and I/O load during the 293.69s interval is
unknown. The run was therefore neither controlled-quiet nor controlled-loaded.
No synthetic-load repeat was attempted: without host-wide observation it would
not reproduce the earlier load, and adding blind pressure to a box with active
work and a history of memory-pressure failure would not be a responsible
measurement.

The complete observations available are:

| Tree | Known load condition | Elapsed | Ceiling delta |
|---|---|---:|---:|
| before this change | two or three other agents working; otherwise uncontrolled | 335s | 35s over |
| before this change | two or three other agents working; otherwise uncontrolled | 351s | 51s over |
| before this change | two or three other agents working; otherwise uncontrolled | 353s | 53s over |
| before this change | two or three other agents working; otherwise uncontrolled | 355s | 55s over |
| after this change | two other agents assigned; their instantaneous load unknown | 293.69s | 6.31s under |

These are not paired quiet/loaded measurements of the same tree. The last row
shows that the changed suite crossed the ceiling once under uncontrolled
concurrent-agent conditions. Its 2% sample margin does not establish that every
loaded run will remain below the ceiling; the four earlier rows preserve the
observed host variability that makes another measurement necessary if the gate
approaches the limit again.

## Final observation

The ordinary uninstrumented `test/gate.sh` passed all lint, smoke, 1,611
integration checks, and 129 focused role checks in **293.69s** with peak RSS
1,099,776 KiB. The five-minute ceiling therefore has **6.31s measured margin**
on this run, not a load-independent guarantee. `test/gate.sh` is the current
measurement command; the numbers in this record are dated evidence for the
scheduling decision, not standing estimates.
This sandbox did not expose the user service manager, so the aggregate 2 GiB
`systemd-run` proof could not be repeated here; the isolated maxima above are
the memory evidence available for this change.
