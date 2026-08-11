# The Gangline contract

Follow operator doctrine and your role brief when either conflicts with this
contract.

## Messages

Run `gang send --to NAME --stdin` to message a teammate.

Gangline delivers each message in an envelope that names its sender. Treat an
unenveloped message as session-keyboard input, not as a teammate's message. Do
not label a message with a sender that Gangline did not supply.

Send review comments, questions, and handoffs directly to the teammate that
needs them. Send only the information the recipient needs to act. Put supporting
detail in a file and send its path.

## Hitching

Choose the teammate's model and reasoning effort before running `gang hitch`.
Pass both choices with `-m` and `-e`. If `gang hitch` refuses either flag, report
the refusal. Do not retry without both choices.

## Compacting

Finish the current edit before running `gang compact`. Record any unfinished
work in a file first.

## The marathon rule

Resolve reversible questions yourself under operator doctrine. When a decision
is irreversible or doctrine does not cover it, record the question for the
operator and stop only the affected work. Continue all other work.
