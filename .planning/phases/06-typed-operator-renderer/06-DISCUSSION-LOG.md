# Phase 6: Typed Operator Renderer - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-16 (re-discussion — supersedes the first pass logged in this file's prior revision)
**Phase:** 6-typed-operator-renderer
**Areas discussed:** Serialization format, Headline, Nested rendering, Field labels, Text-lane testing

---

## Why this re-discussion happened

The first pass locked a bespoke `{key}` / `[...]` template language as the mechanism. Four cross-AI
review cycles (commits `b723814e`, `5cbcebc9`, `68b03af3`, `1d5e54e9`) drove unresolved findings
8 → 5 → 4 → 5 and then stalled. Reading the findings together showed they clustered around the
template's *existence* — parser encoding, unmatched delimiters, nested spans, escape rules — rather
than around bugs in it.

Sean's objection, which reframed the phase: *"I'm struggling with a custom 'format', we're mixing
plain text and a truly structured document."* That is the correct diagnosis. The template existed
only to force prose and a document out of one declaration.

---

## Serialization format

| Option | Description | Selected |
|--------|-------------|----------|
| Bespoke template (first pass) | `{key}` placeholders + `[...]` conditional spans; identity by construction | |
| YAML | Structured text lane; real spec, real tooling | |
| TOON | Token-oriented notation | |
| JSON only, text is a view | One serialization; text is a non-parseable rendered view of the same struct | ✓ |

**User's choice:** JSON only — after explicitly asking "yaml / toon / json variant — pick one".

**Notes:** Sean's stated worry about the intermediate proposal was *"'basically yaml' worries me"* —
which was the right instinct. YAML and TOON were rejected on two grounds: each adds a dependency
against the project's zero-new-Go-deps record, and — deciding — each *looks* parseable, so someone
parses it and the project owes escaping and stability semantics it never designed. Two
machine-readable surfaces to keep in sync is the original divergence defect wearing a hat.

The reframe that settled it: these are not "a sentence and a document", they are **one document and
one view of it**. Once the text lane is not a format, there is nothing to keep in sync, and the
template language, its parser, its escape rule, and its whole negative-test suite are deleted rather
than fixed.

---

## Headline

| Option | Description | Selected |
|--------|-------------|----------|
| Keep headline, hand-written | One prose line per report, declared non-exhaustive | ✓ |
| Pure table, no prose | View is only the rendered struct; zero exempt surface | |
| Derived headline | Generated from designated summary fields | |

**User's choice:** Keep headline, hand-written.

**Notes:** Preserves nuance that field names cannot carry — `1 spared (ineligible since preview)`
teaches something `spared_count 1` does not. Accepted as safe because the headline is *additive prose
over a complete table*: since the table renders every field unconditionally, a headline can add
emphasis but is structurally incapable of hiding a field. Bounded to one line.

---

## Nested rendering

| Option | Description | Selected |
|--------|-------------|----------|
| Indented sub-block per row | Each row renders as its own field block; uniform with top level | |
| One line per row, inline fields | Rows collapse to `id=… outcome=…`; compact | ✓ |
| Ids only, counts at top level | Row detail lives only in JSON | |

**User's choice:** One line per row, inline fields.

**Notes:** Applies to the four two-level reports (archive, purge, consolidate, verify). Closest to
today's output and legible for a long list; a 50-row purge under the sub-block alternative would be
unreadable. Accepted cost: nested values render by a different rule than top-level ones, and very wide
rows wrap.

---

## Field labels

| Option | Description | Selected |
|--------|-------------|----------|
| Raw json tag name | `older_than`, `eligible_count` — literal correspondence to JSON | |
| Humanized label | `Older than`, `Eligible count` | ✓ |

**User's choice:** Humanized label.

**Notes:** Two consequences were surfaced at selection time and recorded as decisions rather than left
implicit:

1. **The chosen previews are asymmetric.** Top-level labels are humanized; the nested-row preview Sean
   selected shows raw inline keys (`id=01H8… outcome=ok`). This is defensible — top-level output is
   read, dense row output is scanned — and is now written down as deliberate (CONTEXT.md D-05) so a
   later reader does not "fix" it into consistency.

2. **Humanized labels + structural-only tests is a trap if done naively.** If the identity gate derives
   its expected labels by calling the same humanizer the renderer calls, both sides move together and a
   humanizer bug is invisible. That is the exact shape of durable record `01mdq5qq9j`, where a
   partition identity was invariant under the very mutation it appeared to guard. Resolved by
   decomposition (CONTEXT.md D-06): the identity gate asserts one rendered line per JSON key
   (correspondence, not label text), and a separate table test pins the humanizer on fixed pairs.

---

## Text-lane testing

| Option | Description | Selected |
|--------|-------------|----------|
| Structural only — no golden text | Assert the identity property; nothing about formatting | ✓ |
| Regenerable goldens, explicitly unstable | Keep text goldens with `-update` | |
| Structural + one smoke golden | Identity assertion plus one representative golden | |

**User's choice:** Structural only — no golden text.

**Notes:** The honest consequence of declaring the text lane unstable. Goldens on an
explicitly-unstable lane would create the illusion of a contract and a maintenance surface for output
nobody may depend on. JSON goldens are unaffected — that lane *is* the contract.

---

## Claude's Discretion

- Reflection over the struct vs. a small interface each doc implements.
- The humanizer's exact rule, and acronym/initialism handling.
- Column alignment mechanics (`text/tabwriter` is the stdlib fit).
- Whether the 15 `xxxSummary` functions are trimmed to headline producers or replaced outright.
- Migration order and batching across plans.
- Whether report enumeration needs an AST derivation or whether reflection over a registry suffices —
  the earlier design needed AST only to police constructor routing, which D-02 removes.

## Deferred Ideas

- **Applying the view mechanism to the client tier** (`client_common.go`) — same class of problem,
  different tier, outside this phase's boundary.
- **Hardening the red-evidence harness** so a compile failure cannot count as RED
  (`internal/store/redevidence_harness_test.go:303-316`, durable record `366pjeht8e`). Real, but a
  test-infrastructure defect this phase inherits rather than causes — it was in scope only because the
  superseded design leaned on that harness. File it rather than smuggle it in.
