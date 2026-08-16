# Phase 6: Typed Operator Renderer - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-16
**Phase:** 6-Typed Operator Renderer
**Areas discussed:** Declaration shape, Sentence fidelity, Enforcement locus, Fate of the
existing gate, Coverage rule, Nesting, Conditional presence

---

## Declaration Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Ordered `[]Field` value | Each report builds an explicit ordered slice of fields (json key + value + how it renders in the sentence). `renderOperator` takes only that slice; both lanes walk it. Most explicit, no reflection, but each report gets rewritten as a builder. | ✓ |
| Struct stays, reflect the tags | Keep the existing `xxxDoc` structs as the declaration; reflect over json tags to derive the ordered field set, with the text template required to reference every field. Smallest diff to the 15 docs. | |
| Generic `Report[T]` + template | A generic Report type pairing the typed doc with a sentence template; the template engine proves every field is consumed. Type-safe, but adds a template layer to a CLI that has none. | |

**User's choice:** Ordered `[]Field` value
**Notes:** Selected with the illustrative `pruneReport` snippet, which is recorded in
CONTEXT.md D-01 as illustrative rather than a locked API.

---

## Sentence Fidelity

| Option | Description | Selected |
|--------|-------------|----------|
| Byte-identical, all 15 | Every current sentence survives character-for-character; the field set must reproduce bespoke prose. Strongest reading of SC2, and the existing pinned-sentence tests stay untouched as the regression gate. | ✓ |
| Semantically equivalent, prose may normalize | Facts and values preserved but wording may shift. Simpler mechanism; costs a rewrite of the pinned-sentence tests, weakening them as an independent check. | |
| Byte-identical except a named exception list | Default to byte-identical with an explicit documented list of deliberate normalizations. | |

**User's choice:** Byte-identical, all 15 (the recommended option)
**Notes:** No exception list. Rejecting the exception list means an unreproducible sentence
is a finding about the mechanism, not a licence to change the prose.

---

## Enforcement Locus

| Option | Description | Selected |
|--------|-------------|----------|
| Compile-time — drop `doc any` | `renderOperator` stops accepting a free-form doc; the only way to call it is with the field set. A widened json document becomes unrepresentable. Touches every call site. | ✓ |
| Runtime — json derived from the field set | Signature may still take a typed value, but json is built from the ordered field set so an undeclared field never reaches output. By-construction in effect, softer in the type system. | |
| Both — typed signature plus derived encoder | Belt and braces; redundancy costs nothing at runtime but adds refactor surface. | |

**User's choice:** Compile-time — drop `doc any`
**Notes:** Chosen as the literal reading of the phase goal's "enforced by construction, not
merely detected by test". The 15-call-site migration cost was accepted rather than
mitigated with a transitional signature.

---

## Fate of the Existing Gate

| Option | Description | Selected |
|--------|-------------|----------|
| Retire it — subsumed by construction | The row table is dead weight that can rot; delete it and record why in the summary. | ✓ |
| Keep as an independent behavioral backstop | Keep the rows as a second, differently-shaped regression check, accepting the maintenance and rot risk. | |
| Replace with a derived, both-ways gate | Drop hand-listed `facts` but keep a test walking `operatorCommands()` asserting json key set equals the sentence's referenced field set. | |

**User's choice:** Retire it — subsumed by construction
**Notes:** Two properties of the retired test were surfaced during discussion and recorded
in CONTEXT.md so they are not lost: its `facts` were hand-listed (exactly what SC1
rejects), and it only ever checked text→json, never json→text.

---

## Coverage Rule

Raised because "byte-identical prose" and "one keyed field set" collide on fields whose
text presence is prose rather than a placeholder — `pruneOutputDoc.BestEffort` appears as
the words "best-effort count", never as `{best_effort}`.

| Option | Description | Selected |
|--------|-------------|----------|
| Every field needs a `{key}` placeholder | Strictest: no prose-only coverage. Forces booleans to interpolate a rendered form that emits the same bytes. | ✓ |
| Placeholder OR declared prose coverage | A field is covered by a placeholder or by an explicit `Prose:` substring the renderer asserts is present in the rendered text. | |
| Placeholder, prose, or explicit `Silent` marker | As above plus an escape hatch for fields deliberately absent from the sentence. | |

**User's choice:** Every field needs a `{key}` placeholder
**Notes:** The `Silent` variant was rejected on the grounds that every future field can take
the escape hatch, reopening the widening hole. The consequence — prose must become value
renderers emitting identical bytes — was accepted explicitly.

---

## Nesting

Raised because four of the 15 reports are two-level and already render per-row text today.

| Option | Description | Selected |
|--------|-------------|----------|
| Nested `FieldSet` as a field value | A Field's value may itself be a FieldSet or `[]FieldSet`, recursing the same coverage rule one level down. Parent sentence covers the list via an aggregate. | ✓ |
| List is one opaque field, elements untyped | Coverage applies only at the top level; simplest, but the guarantee stops at the outer object. | |
| Flatten — aggregate only in the field set | Only scalar aggregates in the field set, detail list passed separately. Cleanest guarantee, but changes the json shape of four commands. | |

**User's choice:** Nested `FieldSet` as a field value
**Notes:** Flattening was rejected because it would break byte-identical output (the
selected Sentence Fidelity option) for archive, purge/restore, consolidate and verify.

---

## Conditional Presence

Raised because `archiveResultDoc.ID` is `json:"id,omitempty"` AND is dropped from its text
line — the two lanes already agree, but via two independent hand-written conditionals.

| Option | Description | Selected |
|--------|-------------|----------|
| One `Present` predicate, both lanes read it | When absent the field is omitted from json AND its placeholder plus surrounding literal drops from the sentence. One decision, no way to disagree. | ✓ |
| Optional segments in the template | Presence lives in the text template as an optional segment; the json encoder omits fields whose segment did not render. | |
| No optional fields — always emit both | Always emit the key and the segment. Simplest rule, but changes json shape and text line for failed-resolution rows. | |

**User's choice:** One `Present` predicate, both lanes read it
**Notes:** The template-segment variant was rejected as putting the presence decision in the
sentence rather than beside the value.

---

## Claude's Discretion

- The concrete Go API of `FieldSet` / `Field` — names, whether `Text` is a method or a
  struct field, placeholder syntax and when parse failure is detected.
- Whether placeholder-coverage failure surfaces at compile time, at `init()`, or at first
  render — constrained only by having to be reachable without a live store.
- Multi-line and list-joining rendering mechanics for the four two-level reports.
- Migration order and batching across plans.

## Deferred Ideas

- Applying the field-set mechanism to the client tier (`engram search`/`list`/`get`
  renderers in `client_common.go`) — same class of problem, different tier, outside this
  phase's boundary.
- Todo `2026-08-10-research-versioned-payload-migration-mechanism.md` was surfaced by the
  todo cross-reference at score 0.4 and reviewed but not folded — it matched on generic
  keywords and is already scoped into (completed) Phases 2–4.
