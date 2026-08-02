# ADR-0013: A PII control is a gate, not a history rewrite

- **Status:** Accepted
- **Date:** 2026-08-02

## Context

The operator's order was a strict no-PII policy for all public GitHub content,
with rock-solid controls. An audit against that order came first and found the
tree and the full commit history clean; platform secret scanning and push
protection were already enabled on the repository. That audit answers a
question about a moment. It says nothing about the next commit, and a policy
that holds only until the next commit is not a policy — it is a fact about the
past wearing a policy's clothes.

The gap between those two things is what this decision closes. An audit is a
snapshot; a control is what keeps the snapshot from going stale the moment
someone commits again.

## Decision

### Scope: everything this repository sends outward, from here forward

The control covers the working tree at any commit, every commit from the point
this control lands onward, the prose this repository's own contributors send
outward through it — issue and pull request bodies created via `gh` — and the
Pages site this repository publishes. It also covers public forks under the
operator's control, including `adambiggs/LHTB`: the policy is about what
becomes public GitHub content, not about which repository's `git remote`
happens to hold it.

It does not cover content this repository does not produce or send —
third-party comments on its issues, or a fork the operator does not control.
Those are outside what a repo-owned gate can enforce.

### What this decision deliberately does not do

**No history rewrite.** The audit already found the tree and history clean;
there is nothing to rewrite. This decision is prospective — it holds the
clean state clean — not retroactive. Should a future audit ever find
something the prospective gates missed, rewriting history to remove it is a
separate, heavier decision this ADR does not make and does not pre-authorize.

### Architecture: one scanner, three gates

`tools/pii-scan` is the only place PII patterns are defined. Three gates call
it rather than reimplementing any part of it, so they cannot drift into
checking different things: `.githooks/pre-push` scans the outgoing range
before a push leaves the machine; a CI workflow scans the pushed or
pull-request range on both a Linux and a macOS runner, catching the case
AGENTS.md already documents — a clone that never ran
`git config core.hooksPath .githooks`; and `CONTRIBUTING.md` points
contributors at `tools/pii-scan --stdin` for issue and PR bodies, the one
surface no git hook or push event can see because it never becomes a commit.

The scanner classifies by shape — home-directory paths, email addresses, IP
addresses, and the literal prefixes or markers that credential formats use —
each checked against an allowlist for values that are already deliberately
public: this repository's own git author identity, `noreply` addresses,
documentation and test domains and IP ranges reserved for exactly that
purpose. What the allowlist does not cover is flagged.

### Refusal: no tracked denylist

Alongside the built-in classes, an operator may keep a local, untracked
`.pii-scan-denylist` naming strings the general patterns cannot: a personal
hostname, a name, anything specific to this operator rather than to PII
shapes in general. The scanner refuses to run at all — not merely to skip the
file — if that path is ever tracked by git. A denylist entry is, by
definition, a string the operator does not want public; committing the file
that lists them would publish the exact thing it exists to keep private. This
is a hard refusal because there is no degraded mode that helps here — a
scanner that ran without it would report clean while blind to its own
compromise.

### Refusal: no vacuous pass

Every invocation of `tools/pii-scan` proves each pattern against a fixture
string built to match it before scanning anything real. A pattern that no
longer matches its own fixture — through a bad edit, a portability
difference between grep implementations, or any other cause — reddens the
gate immediately, on otherwise harmless input, rather than quietly scanning
nothing under that pattern and reporting success. The same applies one layer
down: before trusting any pattern result, the scanner first proves `grep -E`
itself executes, because a shim that resolves on `PATH` and then refuses is
indistinguishable from a working tool by name alone (`test/lint.sh`'s
`shellcheck --version` guard is the precedent this follows). Both refusals
are constitution law 8 applied to this specific tool: a check that could not
determine an answer must say so, not report the answer that happens to look
clean.

## Consequences

- The audited clean state is a starting point the gates are responsible for
  keeping true, not a claim this decision restates or re-proves.
- A future PR against this repository, or an issue or PR body sent through
  it, is checked by the same patterns and the same allowlist a push already
  goes through — there is exactly one place that logic can drift.
- An operator-specific denylist stays enforceable without ever becoming
  itself a published list of what it protects.
- If a pattern breaks, the failure mode is a gate that refuses to pass
  anything, not a gate that silently stops checking the one thing that
  broke.
- A finding that does surface a real leak is a decision this ADR does not
  make in advance: whether and how to address already-public history is
  weighed then, with the actual finding in hand, not pre-committed here.
