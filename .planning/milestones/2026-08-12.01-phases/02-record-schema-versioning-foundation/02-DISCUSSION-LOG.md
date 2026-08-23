# Phase 2: Record Schema Versioning Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-13
**Phase:** 2-Record Schema Versioning Foundation
**Areas discussed:** Stamping seam & partial writes, Downgrade guard, v0 encoding & wire shape, Negative recall-gate proof

---

## Stamping Seam & Partial Writes

### Where does the stamp actually happen?

| Option | Description | Selected |
|--------|-------------|----------|
| Inside `payload()` | store.go:545 is the ONE place a full record becomes a Qdrant payload; Upsert, Update, Supersede, Reindex all funnel through it. Makes an unstamped full write unrepresentable. | ✓ |
| In `Store.Upsert` | One level up at store.go:744. Still single-point for full writes, but leaves payload() usable without a stamp. | |
| Per-caller at each write entrypoint | Most legible line-by-line, but lets a future 5th write path silently skip it. | |

**User's choice:** Inside `payload()` (recommended option)

### Do the partial `setPayloadKeys` mutations stamp?

| Option | Description | Selected |
|--------|-------------|----------|
| Never stamp on partial writes | Only a full payload() rebuild can honestly claim currency. A partial SetPayload stamping v1 would assert "all v1 keys present" while writing only one key, making Phase 3's sweep skip records still needing migration. | ✓ |
| Stamp on all partial writes too | Literal reading of "every write path stamps"; converts the sweep's backlog count into a lie. | |
| Stamp only on semantic-state partial writes | Stamp on SetVisibility/archive/back-stamp, skip IncrementAccess; creates a per-call-site judgment. | |

**User's choice:** Never stamp on partial writes (recommended option)
**Notes:** Accepted cost recorded — a v0 record touched by SetVisibility stays v0 until the sweep reaches it.

### How is "100% of write paths stamp — not a sample" proven?

| Option | Description | Selected |
|--------|-------------|----------|
| Structural: assert the seam is the only door | Source-level conformance gate (internal/keylinks / internal/surfaces idiom) that every Qdrant write routes through the stamping seam, plus behavioral round-trips. Catches the write path that doesn't exist yet. | ✓ |
| Behavioral enumeration over exported write methods | Concrete and readable, but hand-maintained; a 5th write path is silently uncovered. | |
| Both, with the source-level gate load-bearing | Highest cost, strongest guarantee. | |

**User's choice:** Structural (recommended option)
**Notes:** Memory `x6v6qxqd6f` records that a Phase 1 AST scan was itself bypassed via a local variable holding the literal — carried into CONTEXT as an anchor-on-identity warning for the planner.

### Where does the current-version constant live?

| Option | Description | Selected |
|--------|-------------|----------|
| Create `internal/migrate` now, constant only | Dependency direction right from day one; Phase 3 grows the registry into it; no mid-milestone symbol move after Phase 5 freezes proto field numbers. | ✓ |
| Define in `internal/store`, move in Phase 3 | Smallest Phase 2 diff, but the move lands after Phase 5 has referenced it. | |
| Define in `internal/store` permanently | Inverts the dependency REQ-migration-step-registry requires. | |

**User's choice:** Create `internal/migrate` now (recommended option)

---

## Downgrade Guard (the criterion collision)

> Surfaced during codebase scout: criterion 1 ("every write path stamps the current version")
> and criterion 5 ("never downgraded") cannot both hold under unconditional stamping, because
> Update and Supersede both re-Upsert through payload().

### What is the stamping rule?

| Option | Description | Selected |
|--------|-------------|----------|
| Monotonic `max(current, existing)` | The only rule satisfying criteria 1 and 5 simultaneously. New record → 0 → stamps current; v2 record edited by v1 binary stays v2. | ✓ |
| Always stamp current | Literal criterion 1; silently downgrades on exactly the rollback case criterion 5 exists for. | |
| Preserve if non-zero, else current | A genuine v0 record gets current-stamped on edit while never migrated — the same false-currency claim rejected for partial writes. | |

**User's choice:** Monotonic max (recommended option)

### Is the v2-stamp-on-v1-rewrite honest?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — additive-only makes it safe, document why | v2-only keys are lost by the rewrite even though the stamp says v2. Accepted limitation of rolling back across a migration; recovery is re-running the sweep. Recorded as a decision, not left as a discovery. | ✓ |
| No — `fromPayload` must preserve unknown keys | Strictly correct and makes the stamp truthful, but a new field and a new codec contract; Phase 3 scope at the earliest. | |
| No — refuse the write instead | Safest, but reinstates the hard-reject-on-mismatch behavior (webauth sessionPayloadVersion) this field deliberately diverges from. | |

**User's choice:** Yes, with the limitation documented (recommended option)

### What version does `Store.Reindex` carry?

| Option | Description | Selected |
|--------|-------------|----------|
| Same monotonic rule, no special case | Reindex is a full rewrite through payload(), so it inherits max(current, existing) like every other full write. | ✓ |
| Preserve the source version exactly | Version-transparency for an embedder migration; needs a carve-out in payload() or a separate path. | |
| You decide at plan time | Defer to how reindexTargetContents / payloadKeysEqual behaves. | |

**User's choice:** Same monotonic rule (recommended option)
**Notes:** The third option's concern was recorded anyway as an explicit research flag in CONTEXT (D-07) — `reindexTargetContents` compares src/tgt payloads and a differing `schema_version` key may break `payloadKeysEqual`. To be answered by reading, not guessing.

### How is forward-compatibility proven with only one binary?

| Option | Description | Selected |
|--------|-------------|----------|
| Raw payload injection, both directions | Write via raw Qdrant SetPayload at current±1 plus an unknown key; assert decode, full recallability, get_memory, no downgrade, and that a subsequent Update preserves the higher version. Real Qdrant. | ✓ |
| Override the version constant in tests | Cleaner to read, but tests constant plumbing rather than decode-unknown-payload behavior. | |
| Both | Two mechanisms, more code, fewer blind spots. | |

**User's choice:** Raw payload injection, both directions (recommended option)
**Notes:** The constant-override mechanism was preserved as a deferred idea — a reasonable secondary if the planner wants to exercise the stamping path independently.

---

## v0 Encoding & Wire Shape

### Go type

| Option | Description | Selected |
|--------|-------------|----------|
| Named type from `internal/migrate` | `SchemaVersion migrate.Version` where `type Version int`. Zero value == v0 == absent, so criterion 2 falls out of Go semantics. Type-checks the max comparison. | ✓ |
| Plain `int` | Same zero-value benefit, nothing distinguishes it from any other int at a call site. | |
| `*int` | Distinguishes absent from zero — but they are defined here to be the same state, so it adds nil-deref surface for nothing. | |

**User's choice:** Named type from `internal/migrate` (recommended option)

### JSON tag

| Option | Description | Selected |
|--------|-------------|----------|
| `json:"schema_version"`, always emitted | A v0 record serializes as `"schema_version": 0` — the honest answer, and observable on exactly the pre-migration records criterion 2 cares about. | ✓ |
| `json:"schema_version,omitempty"` | Matches the struct's omitempty habit, but hides the field precisely when it reads 0. | |
| You decide at plan time | | |

**User's choice:** Always emitted, no omitempty (recommended option)
**Notes:** This is the deliberate divergence from `EmbedderIdentity` / `IdempotencyFingerprint`, whose `json:"-"` tags are documented in-code as "deliberate and load-bearing".

### Which read surfaces expose it in this phase?

| Option | Description | Selected |
|--------|-------------|----------|
| `full=true` recall + `get_memory` only | Falls out of the plain json tag with zero extra code (shapeRecall returns store.Memory verbatim on full). Compact recallView untouched. | ✓ |
| Also add it to compact `recallView` | Every recall result carries the version; widens the agent-facing summary and pre-empts Phase 7. | |
| Now, and note compact as a Phase 7 question | | |

**User's choice:** `full=true` + `get_memory` only (recommended option)
**Notes:** The compact-recall question was recorded as a deferred idea for Phase 7 regardless.

### Qdrant payload index

| Option | Description | Selected |
|--------|-------------|----------|
| No index in Phase 2 — Phase 3 adds it if needed | Phase 2's job is semantics, not sweep performance; avoids an index on a field criterion 4 bars from recall. | |
| Add the index now | ensureIndexes is Phase 2 territory and Phase 3/4 will count by version. | ✓ |
| No index, plus an anchored note that it would only ever serve the sweep | | |

**User's choice:** Add the index now — **not** the recommended option
**Notes:** Consequence surfaced and accepted: an existing index makes it easier for a future filter to reach for the field, so the criterion-4 gate (not inconvenience) becomes the only thing holding that line. Recorded in CONTEXT D-12 with a `costly` reversibility rating, since dropping an index on a large collection is an operator action rather than a code change.

---

## Negative Recall-Gate Proof

### Load-bearing proof mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Filter introspection — walk the built `*qdrant.Filter` | Recursively walk Must/Should/MustNot/nested, collect the SET of referenced field keys, assert `schema_version` absent. Direct evidence about the object sent to Qdrant. | ✓ |
| Behavioral — a v0 record stays recallable everywhere | Proves the user-visible outcome, but a filter could reference the field and still pass if test data happens to match. | |
| Source-level conformance gate | Catches code no test exercises, but memory x6v6qxqd6f records an AST scan already bypassed once via a local variable. | |

**User's choice:** Filter introspection (recommended option)

### Coverage completeness

| Option | Description | Selected |
|--------|-------------|----------|
| Derived set + assert non-empty coverage | Don't hand-list; assert the enumeration matches what exists and that the walked-filter count is non-zero and equals expected. | ✓ |
| Hand-enumerated list of the six builders | Readable, goes stale silently on the seventh recall path. | |
| Both — enumerate and assert the enumeration is complete | | |

**User's choice:** Derived set + non-empty coverage assertion (recommended option)
**Notes:** Grounded in memory `x6v6qxqd6f` — Phase 1's gate passed while scanning nothing, and `len(findings) > 0` caught one of two bypass shapes. Set-equality over enumerated shapes.

### Fail-first proof

| Option | Description | Selected |
|--------|-------------|----------|
| Inject a real filter condition, prove RED, revert | Recorded in VERIFICATION.md as evidence. Matches Phase 1's own conclusion: prove red then revert clean, and inject into a real package rather than only a fixture. | ✓ |
| Committed negative fixture the test runs against | Survives as a permanent artifact, but proves the walker works on a fixture rather than on the real builders. | |
| Both | | |

**User's choice:** Real injection, prove RED, revert (recommended option)
**Notes:** A permanent committed fixture remains welcome as a secondary artifact but is explicitly not the proof.

### Gate scope

| Option | Description | Selected |
|--------|-------------|----------|
| Recall filters + Cedar-derived authz conditions | The authz conditions compile into the same `*qdrant.Filter`, so walking the composed filter from ownerScopeFilter covers both in one pass. | ✓ |
| Recall filters only | Excludes half of what actually gets sent. | |
| Also the operator-tier sweeps | Would make Phase 3 unimplementable — its sweep MUST filter by schema_version to find its backlog. | |

**User's choice:** Recall + authz (recommended option)

---

## Claude's Discretion

No area was answered "you decide" — every question resolved to an explicit choice. Left open to
planning judgment in CONTEXT.md:

- The exact name/signature of the `internal/migrate` version symbol (`Version`,
  `CurrentVersion` are indicative).
- The shape and home of the filter-walking helper (test-only vs an exported introspection aid).
- Whether D-06's operator-facing limitation note lands in `guides/upgrade.md`, a doc comment,
  or both — Phase 8 owns the docs tail, but the decision that it must be written down is D-06's.

Also recorded as an explicit **open question for the planner** rather than a preference: what
`migrate.CurrentVersion` equals in Phase 2, given that Phase 4 registers `backfill-short-ids`
as the v0→v1 step.

## Deferred Ideas

- Unknown-payload-key passthrough on `Memory` (would make D-06's stamp truthful) — Phase 3 at
  the earliest.
- Refusing writes from a binary older than the record's version — rejected as reinstating
  `sessionPayloadVersion`'s hard-reject behavior.
- `schema_version` on compact `recallView` — properly Phase 7's state-surfacing question.
- A test-only override of the version constant — viable secondary proof mechanism for D-08.
- Dropping the D-12 index if Phase 3/4 turn out not to need it — an operator action, not a code
  change.

### Reviewed todo (not folded)

- `research-versioned-payload-migration-mechanism`
  (`.planning/todos/pending/2026-08-10-research-versioned-payload-migration-mechanism.md`,
  `resolves_phase: 3`, score 0.6) — its ask was "research, not a decision", that research
  shipped and became this milestone. Background for Phase 2; delivered by Phases 3–4.
