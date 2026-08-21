# Requirements: engram — Milestone `2026-08-12.01` Record State & Schema Evolution

**Defined:** 2026-08-12
**Core Value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its
context, and wrong or stale memories can be corrected or superseded, so recall stays trustworthy as
the store grows.

**Milestone goal:** a record's full state — supersession, scheduling, archival, and its own schema
version — is reachable and legible on every lane, and payload evolution has a real mechanism
instead of another one-shot operator command.

## v1 Requirements

### Gate & CI Integrity

- [x] **REQ-keylink-pattern-matchable**: A plan's key-link `pattern:` field actually compiles and matches what it claims. The `\\`-escaped form that survives verbatim into `new RegExp` — silently unmatchable rather than throwing — is eliminated from this repo's plans, and a guard detects the shape so a future plan cannot reintroduce it (#479).
- [x] **REQ-keylink-past-gates-reassessed**: v0.13.x Phases 1 and 2's key-links are re-resolved against the tool, since both passed verification while their gates were no-ops. Whatever those gates were meant to pin is either genuinely pinned or recorded as unpinned — a past "key-links passed" is not treated as evidence.
- [x] **REQ-ci-qdrant-container-stability**: A full `go test ./...` CI run no longer fails from `internal/store`'s Qdrant testcontainer dying mid-run. Whatever mitigation is chosen (shared container, serialized Qdrant-backed packages, memory cap), the job captures the container's exit reason on failure so a recurrence is diagnosable from evidence rather than inferred (#497).

### Record Schema Versioning

- [x] **REQ-schema-version-stamped**: Every record write stamps the current `schema_version`. A record written before the field existed reads as v0 by absence, so adoption requires no backfill. Proven by a test asserting that 100% of write paths stamp — not a sample.
- [x] **REQ-schema-version-never-gates-recall**: `schema_version` never appears in any Qdrant recall or authz filter condition. Carried by an explicit negative test, because the adjacent `superseded_by`/`archived_at` `IsEmpty` recall-gate idiom has inverted cardinality here and copying it would silently exclude every pre-migration record from recall.
- [x] **REQ-schema-version-wire-visible**: `schema_version` is a wire-visible `store.Memory` field, not a `json:"-"` internal audit stamp like `EmbedderIdentity`/`IdempotencyFingerprint`. A record is never rejected, hidden, or downgraded for carrying an old version — the divergence from `sessionPayloadVersion`'s hard-reject-on-mismatch is deliberate and tested.
- [x] **REQ-schema-version-forward-compatible**: A binary reading a record whose `schema_version` is NEWER than its own constant still reads it, ignoring keys it does not understand, and never rejects or hides it. This is what makes rolling the binary back across a schema change safe, and it is tested in both directions — older-than and newer-than — not only the older-than case.

### Migration Mechanism

- [x] **REQ-migration-step-registry**: A stdlib-only `internal/migrate` leaf package holds the ordered migration-step registry, with zero Qdrant or authz dependency, imported by `internal/store` — mirroring the `internal/surfaces`/`internal/openaiurl` leaf precedent. Step ordering and idempotency are checked by a single `Validate` invariant over the registry.
- [x] **REQ-migration-additive-only-gated**: A registered step may only ADD payload keys — never remove or rename one. This is enforced by a step-registration invariant that makes a non-additive step a build or test failure, not a review catch. The step interface is shaped so a per-version decoder can attach later without breaking existing steps.
- [x] **REQ-migrate-command**: `engram migrate` runs the registry through a `Store.Migrate` sweep, registered via `registerDestructive` so it previews by default and `--apply` is a runtime choke point, with `--output json|text` matching the rest of the operator tier.
- [x] **REQ-migrate-status-histogram**: `engram migrate status` reports a version-distribution histogram across the collection, not a single scalar version — mixed-version collections are legitimate mid-rollout, and a scalar would misreport them.
- [x] **REQ-migrate-preview-apply-parity**: `--apply` acts only on the intersection of the previewed, gate-passing set and a fresh re-derivation, reusing the shipped `spine-review purge` pattern. A preview that does not match what apply does is a defect, not an accepted approximation.
- [x] **REQ-migrate-partial-failure-resume**: The sweep survives Qdrant's batch `SetPayload` non-atomicity (confirmed upstream, qdrant/qdrant#9371 — partial application within a chunk must always be assumed). Proven against a real pinned Qdrant with a forced mid-sequence partial failure, then a resume that converges.
- [x] **REQ-migrate-converges-without-lock**: The sweep needs no collection lock, because the write path stamps the current version before the sweep runs — so new writes arrive already-current and never create new work, leaving a finite, strictly shrinking backlog. The stamp-then-sweep ordering is a stated dependency, not an accident.
- [x] **REQ-backfill-shortids-first-step**: `backfill-short-ids` is registered as the v0→v1 step, giving the mechanism a real first customer. The standalone command becomes a thin delegating alias (soft deprecation, per the `migrate-set-owner` precedent — never hard removal), and its `--dry-run: false` apply-by-default is reconciled with `registerDestructive`'s preview-by-default via a `guides/upgrade.md` entry gated by a test.
- [x] **REQ-migration-step-reversibility**: Every registered step declares whether it is reversible. A reversible step supplies its inverse; an irreversible one (a vector rewrite, an external side effect that cannot be undone) is marked so explicitly and names why. The declaration is mandatory — the same registration invariant that enforces additive-only rejects a step that is silent about reversibility, so "nobody thought about it" is not a representable state.
- [x] **REQ-migrate-revert**: `engram migrate` can run declared inverses in reverse order to return the collection to an earlier schema version. It previews by default like `--apply`, and it refuses the whole operation at the first irreversible step in the range rather than reverting partially and leaving the collection between versions. An irreversible step's recovery path is a collection snapshot, and the refusal message says so.
- [x] **REQ-migrate-never-automatic**: No migration ever runs automatically on startup. At most, startup emits a non-blocking warning that pending migrations exist. This diverges deliberately from the single-binary-server norm (Gitea, Grafana, Vaultwarden all auto-migrate) because it contradicts this project's explicit-never-automatic stance.

### Connect Record-State Parity

- [x] **REQ-connect-record-state-parity**: `proto`'s `Memory` message carries `superseded_by`, `supersedes`, `not_before`, `not_after`, `archived_at`, and `schema_version` — added together in ONE additive pass and wired through `memoryToProto`, so the Connect lane's field list mirrors `store.Memory`'s scheduling, supersession, archival and version state (#482).
- [x] **REQ-connect-parity-roundtrip-proof**: The proof is an exhaustive field-mapping round-trip test, not `buf breaking` passing and the code compiling. This gap has recurred three times (v0.8.x, v0.11.x, v0.13.x) precisely because green codegen was mistaken for evidence that the mapping exists.

### Operator Output

- [x] **REQ-operator-renderer-typed**: Operator command output derives text and json from ONE shared ordered field set, so a json document cannot widen past what its text sentence states. Field-set identity holds by construction rather than by a test over hand-built rows (#481).

### State Surfacing

- [ ] **REQ-console-record-state**: The operator console shows a record's archived, superseded, and scheduled state, which it cannot render at all today.
- [x] **REQ-cli-record-state**: `engram search`/`list`/`get` surface the same state, so the CLI and console agree on what a record is.
- [ ] **REQ-migration-state-visible**: Pending-migration state is visible to an operator through the same surfaces, not only by running the migrate command.

### Registry & Docs

- [ ] **REQ-sweep-scope-rule-registered**: The `--scope`-or-`--all-scopes` requirement shared by `summarize-missing` and `spine-review scan` is a registered `surfaces.ConditionalRule` rather than a hand-rolled `usageErrorf`, with its canonical sentence anchored on every surface its fields resolve to (#480).
- [ ] **REQ-docs-record-state**: `reference/memory-record.md` and `reference/tools.md` document the full record state including `schema_version`, and the migration mechanism has an operator-facing guide.
- [ ] **REQ-claude-md-migrations-convention**: CLAUDE.md's Conventions line "Not used here: database migrations" is revised to describe what this milestone actually ships, so the normative doc does not contradict the code.

## v2 Requirements

Deferred. Tracked, not in this roadmap.

- **REQ-migration-resumability**: Progress/resume for very large collections, mirroring `reindex --resume`. Deferred because no size data exists — v0.13.0 has never run against a real deployed collection, so the cost cannot be justified yet.
- **REQ-version-dispatch-codec**: A `fromPayload`/`payload()` branch interpreting genuinely incompatible shapes per version. Deferred deliberately: additive-only plus sweep composes (a rename is write-new → sweep → drop-old), dispatch cannot help on filtered fields at all, and every supported version's decoder must then be maintained indefinitely.
- **REQ-consent-adversarial-proof**: Carried from v0.13.x, still unmet. Needs a fixture that reliably misleads on identity, not more runs of the same one (`WINDOWS.md` id 3).

## Out of Scope

| Feature | Reason |
|---------|--------|
| Automatic migration on startup | Contradicts the explicit-never-automatic design stance and the shipped `registerDestructive` preview tier. A non-blocking warning is the sanctioned shape. |
| Reverting an irreversible step | A step that rewrites vectors or causes an external side effect cannot be undone by deleting payload keys. `engram migrate` refuses rather than reverting partially; recovery there is a collection snapshot. Reverting *reversible* steps IS in scope — see `REQ-migrate-revert`. |
| Automatic revert on failure | A failed `--apply` does not auto-revert. The sweep is resumable and idempotent, so the recovery path is re-running it, not unwinding it — auto-revert would turn one partial write into two. |
| Collection locking during migration | Unnecessary — the sweep provably converges once the write path stamps. Also needs a multi-replica coordination primitive engram does not have, and costs recall downtime for a service whose job is availability mid-session. |
| Folding `migrate-remap-owner`, `summarize-missing`, `reindex` into the registry | None is version-driven — they key off IdP claim changes, ongoing async fill, and embedder config identity. Folding them in would make `migrate status` report misleading pending counts. |
| Cross-spine `ListScopes` failure handling (#456) | The issue states the current behavior is deliberate and documented, not a defect. Revisit only if it shows up in practice. |
| Full-stack console e2e harness (#366) | Large, and overlaps #497's infrastructure story. |

## Traceability

Which phases cover which requirements. Confirmed during roadmap creation. Phase IDs match
`.planning/ROADMAP.md`'s structural `### Phase N:` headers for this active milestone (bare, per
this repo's structural invariant — see ROADMAP.md's `## Progress` note).

| Requirement | Phase | Status |
|-------------|-------|--------|
| REQ-keylink-pattern-matchable | Phase 1 | Complete |
| REQ-keylink-past-gates-reassessed | Phase 1 | Complete |
| REQ-ci-qdrant-container-stability | Phase 1 | Complete |
| REQ-schema-version-stamped | Phase 2 | Complete |
| REQ-schema-version-never-gates-recall | Phase 2 | Complete |
| REQ-schema-version-wire-visible | Phase 2 | Complete |
| REQ-schema-version-forward-compatible | Phase 2 | Complete |
| REQ-migration-step-registry | Phase 3 | Complete |
| REQ-migration-additive-only-gated | Phase 3 | Complete |
| REQ-migration-step-reversibility | Phase 3 | Complete |
| REQ-migrate-partial-failure-resume | Phase 3 | Complete |
| REQ-migrate-converges-without-lock | Phase 3 | Complete |
| REQ-migrate-command | Phase 4 | Complete |
| REQ-migrate-status-histogram | Phase 4 | Complete |
| REQ-migrate-preview-apply-parity | Phase 4 | Complete |
| REQ-backfill-shortids-first-step | Phase 4 | Complete |
| REQ-migrate-revert | Phase 4 | Complete |
| REQ-migrate-never-automatic | Phase 4 | Complete |
| REQ-connect-record-state-parity | Phase 5 | Complete |
| REQ-connect-parity-roundtrip-proof | Phase 5 | Complete |
| REQ-operator-renderer-typed | Phase 6 | Complete |
| REQ-console-record-state | Phase 7 | Pending |
| REQ-cli-record-state | Phase 7 | Complete |
| REQ-migration-state-visible | Phase 7 | Pending |
| REQ-sweep-scope-rule-registered | Phase 8 | Pending |
| REQ-docs-record-state | Phase 8 | Pending |
| REQ-claude-md-migrations-convention | Phase 8 | Pending |

**Coverage:**

- v1 requirements: 27 total
- Mapped to phases: 27
- Unmapped: 0

**Phase 3 note:** the original single "Migration Mechanism" cluster (11 requirements, 41% of the
milestone) was split during roadmap creation into Phase 3 (registry, additive-only/reversibility
invariants, `Store.Migrate` sweep, partial-failure resume, lock-free convergence — 5 requirements)
and Phase 4 (`engram migrate` CLI: status/preview-apply-parity/revert/never-automatic,
`backfill-short-ids` fold-in — 6 requirements), avoiding one oversized phase. Total requirement
count for the cluster (11) and the milestone (27) is unchanged by the split.

---
*Requirements defined: 2026-08-12*
*Last updated: 2026-08-12 after roadmap creation for milestone `2026-08-12.01` (8 phases; Phase 3 split into Phase 3/4)*
