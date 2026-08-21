# TD-0003: commit messages carry literal escape sequences the gate does not read

- **Status:** Open
- **Date:** 2026-08-21
- **Scope:** `.githooks/commit-msg`, and the history it has already accepted

## Problem

`git commit -m` stores its argument verbatim: a `\n` written into it stays two
characters. Messages have landed carrying `\n\n` where a blank line between body
and footer was meant, so the stored prose reads

    ... local to the example that consumes it.\n\nProof: test/gate.sh

The message gate reads every message and refuses a subject that is not a
Conventional Commit, and a `BREAKING CHANGE:` line sitting in body prose rather
than in the footer block. It looks at no escape sequence, so this form is
accepted. Nothing repairs it afterwards either: rewriting history is refused
here, which makes the gate the only place the artifact is cheap.

## Evidence

Measured over the whole history:

```sh
git log --all --format='%H' | while read -r h; do
  git log -1 --format='%B' "$h" | grep -q '\\n' && echo "$h"
done
```

On the date above that named 46 commits. Forty-two carry the doubled `\n\n`
form; the other four carry a single `\n` inside quoted technical prose —
`printf "high\n"; exit 17`, `printf '%s\n' "$pane" | grep -qE`, and a
`$'ok\nnow do the …'` send argument. Every one of those four is correct as
written, so a refusal of the two characters would reject conforming messages.
The doubled form appears in none of them.

## Direction

If the gate takes this, it refuses the doubled form only, and its refusal names
the cause rather than the shape: `-m` does not interpret escapes, so the message
wants `-F` or a heredoc. Widening it to a single `\n` needs a way to tell quoted
text from prose, which the four commits above show the gate does not have.
