# ADR-0004: Input occupancy is not clearance authority

- **Status:** Accepted
- **Date:** 2026-07-30

## Context

Gangline reports `gated (hook set)` when a harness-owned UI has displaced the
composer, and `send` refuses to inject into it. The refusal is sound: ordinary
text or Enter would act on the occupying UI rather than land as a message. The
name and refusal text make a stronger claim, however. They say the agent is
blocked on a decision only the operator can clear, while Gangline has established
only that something owns the input box.

Four observed cases separate those facts:

1. A permission prompt grants authority and is operator-only.
2. A native question UI asks the driver to make a decision. A teammate was
   blocked from answering one addressed explicitly to the lead, and the lead had
   to reach past Gangline with raw tmux keys.
3. A Codex latency notice occupied the input box but required no action and
   promised to clear itself. The context counter advanced from 144k to 162k while
   the roster called the agent gated: input occupancy and forward progress were
   simultaneously true.
4. A real operator question occupied the lead's pane while peers tried to hand
   work to it. The refusal was correct, but the team's routing point was down and
   no state surfaced that cost.

The first three differ in clearance authority or lifecycle. The fourth differs
in urgency, not authority. Collapsing them into `gated` makes a mechanically
observed fact answer a question Gangline did not ask.

## Decision

### Publish occupancy first, authority as a qualifier

The primary state is **`occupied`**: a harness-owned UI owns the input box, so
ordinary injection is unsafe. Authority or lifecycle is a qualifier on that
state, not another primary state:

- `occupied (operator-only)` — the instance was positively identified as granting
  authority or as addressed to the operator;
- `occupied (driver-answerable)` — a native question UI and its non-target driver
  were both positively identified;
- `occupied (self-clearing)` — the profile positively identified a transient UI
  that needs no answer; and
- `occupied (authority unknown)` — occupancy is determined, but its clearance
  authority or lifecycle is not.

One qualified state is enough because all four answers have the same immediate
transport consequence: `send` must not paste ordinary text. Separate primary
states would duplicate that invariant and invite a future caller to forget one.
Keeping `gated` with qualifiers is not enough, because the word itself says the
agent is blocked behind a human-controlled gate; that was false for the measured
self-clearing notice.

An implementation retires `gated` rather than keeping it as an alias. Scripts and
documentation that consume the state vocabulary move in the same change. An alias
would preserve the stronger false claim this decision removes.

### UI classification is profile knowledge; authorization is not guessed

Which UIs a harness paints, and what each means, belongs in its profile under law
4. `bin/gang` may consume a generic classification result; it must not contain a
harness name, harness-specific modal text, or menu layout. A broad occupancy
recognizer remains the safety boundary. Narrower profile evidence may refine the
modal kind and lifecycle.

Who may answer a particular question is a second input. The same native question
surface carried a worker-to-lead question and a lead-to-operator question in the
observed cases. Recognizing that surface therefore does not by itself establish
`driver-answerable`. Core policy may combine the profile's generic result with an
authenticated sender and an explicit driver or audience relationship; it must not
infer that relationship from a harness name, a window named `lead`, or arbitrary
prose that happens to mention a teammate. Gangline has no such machine-readable
relationship today.

An absent classifier, an unrecognized occupied UI, a malformed result, or a
classifier failure yields `occupied (authority unknown)`. So does a known question
UI whose intended driver cannot be established. Operationally either failure
closes: ordinary sends remain refused and no autonomous answer is attempted.
Reporting it as `operator-only` would spend could-not-determine as a positive
authority claim, so the diagnostic instead tells the operator to inspect the UI.

Token movement does not classify a modal. It proved that one observed agent kept
working, but does not prove that another visually similar notice will self-clear.
Likewise, the presence of options does not distinguish a permission grant from a
question. Those meanings require profile evidence at the harness versions that
declare them.

### Do not build driver-answering yet

Native question UIs may be answered by their positively established driver. When
that driver is an authenticated teammate it may act autonomously, but the target
agent must never answer its own question; a lead-to-operator question remains the
operator's. Gangline does not yet have a shipped profile contract that can safely
perform the operation. A generic `gang answer` with no profile able to satisfy
that contract would be an unused verb, forbidden by law 5.

The observed manual answer proves a consumer exists; it does not prove the
mechanism. Before driver-answering can land, at least one shipped profile must
demonstrate all of these on a current harness version:

- distinguish its native question UI from permission and unknown occupied UIs;
- establish the intended driver or audience without guessing from menu shape or
  free-form question prose;
- find an option by matching its visible **text**, never by ordinal position;
- keep the pane stable across reading and selection;
- read back the option actually taken before reporting success; and
- record the authenticated driver, target, and selected text, because a menu
  selection cannot carry Gangline's sender envelope.

That record is a new artifact and must receive a retention and deletion path in
the implementation decision. Until the whole contract has a profile and a live
consumer, the established driver uses the existing manual path. The existing
self-identity refusal remains a requirement, not an implementation shortcut to
revisit.

### Treat blocked inbound traffic as a separate concern

An occupied lead with attempted inbound traffic is more urgent than an occupied
agent nobody is trying to reach. That fact does not change who may clear the UI,
so it is not another modal class and is not solved by driver-answering.

Gangline currently refuses the message; it does not queue it. Calling that state
"queued inbound" would fabricate delivery. Surfacing the attempted handoff needs
separate state, lifecycle, and deletion decisions, and is tracked separately from
this ADR in [#29](https://github.com/adambiggs/gangline/issues/29). The safe current
procedure remains: preserve the refused message outside Gangline, report the block,
and retry after the occupancy clears.

## Consequences

- Injection safety does not weaken: every occupied state refuses ordinary sends.
- The roster stops turning known occupancy into an unsupported claim about human
  authority. Self-clearing notices no longer demand needless operator escalation,
  and unknown authority remains visibly unknown.
- Profiles gain responsibility for any authority refinement they offer. Profiles
  that cannot distinguish their UIs still work safely and report
  `occupied (authority unknown)`.
- A future driver-answer path is deliberately narrower than `send`, auditable,
  text-selected, read back, and never available to the target itself. No dormant
  verb or profile surface lands before those conditions can be exercised.
- Traffic blocked by a legitimately operator-only prompt remains possible,
  especially on the lead. Its urgency is a message-routing concern, not evidence
  that the prompt was misclassified.
