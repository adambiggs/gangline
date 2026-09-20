# Operations

Gangline keeps native harnesses visible. This guide covers the few operational
choices that matter once a team is running.

## Native prompts

Authentication, repository access, permissions, and trust decisions stay in the
native harness. If an agent is occupied by one, inspect it with:

```sh
gang capture NAME
gang attach
```

Answer the prompt in that terminal, then inspect the agent again with `gang
status NAME`. Do not automate a security decision through Gangline.

## Messages

Use `gang send NAME` to give an agent work. Delivery is verified
at the recipient terminal. If it cannot safely accept input, Gangline keeps the
message pending and retries at a later native turn boundary. `gang status NAME`
and `gang roster` show that pending work.

Read the command's stderr before sending again. A status 3 refusal means nothing
was accepted; an ordinary successful send may mean the message is waiting rather
than already visible to the recipient.

## Context and limits

Long-running agents eventually need more context or hit a provider limit.
Gangline can show context warnings and request native compaction at an agent's
chosen checkpoint. It can also schedule one continuation after a provider reset
when configured to do so.

Use `gang config` to inspect the active settings. Keep thresholds and automatic
continuations conservative: they are assistance, not a substitute for choosing
when to pause, compact, or stop work.

## Recover a team

Start with the current evidence:

```sh
gang roster
gang status NAME
gang capture NAME
```

An `?unknown?` state is not idle. Restore the missing native condition or
inspect the terminal before acting. An `!occupied!` state needs an answer in the
native UI. If an agent process is no longer usable, remove its named window with
`gang drop NAME` and start a replacement with `gang hitch`.

Before ending a whole team, run `gang roster` and name the exact session in
`gang down SESSION`. Gangline preserves pending message material for recovery
when it can; check the command output for the location.
