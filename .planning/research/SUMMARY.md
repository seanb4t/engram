# Project Research Summary

**Project:** engram — Record State & Schema Evolution
**Milestone:** `2026-08-12.01`
**Domain:** Payload schema versioning + eager migration mechanism + Connect wire-parity widening + typed operator renderer, in an already-shipped Go + Qdrant single-binary service
**Researched:** 2026-08-12
**Confidence:** HIGH overall (every architectural and pitfall claim is read directly off internal/store, internal/surfaces, internal/webauth, cmd/engram, proto/engram/v1 at HEAD; the migration-registry shape is MEDIUM — no canonical Go library exists for a db-less, Qdrant-native migration ledger, so it is a designed pattern extrapolated from in-repo precedent, not an imported one)

## Executive Summary

This milestone adds four tightly-coupled capabilities to an already-shipped memory store: a schema_version payload discriminator, an eager sweep-based migration mechanism (engram migrate), additive Connect wire parity for six Memory fields (five already exist in Go, unwired on the wire), and a typed operator renderer to make JSON/text output drift structurally unrepresentable. All four research tracks converge on the same conclusion: zero new Go dependencies are required. Every capability lands on a seam the codebase already has — internal/webauth session payload version for the version-stamp mechanics, registerDestructive preview/apply tier for the migration CLI, BackfillShortIDs/RemapOwner idempotent-scan-and-repair shape for the sweep, and internal/surfaces sealed-registry discipline (minus its literal conformance-gate machinery) for the migration-step registry.

The single highest-risk finding, surfaced independently by both the architecture and pitfalls research, is a cardinality trap: the codebase's own idiom for "new orthogonal record state" is to add a sibling IsEmpty condition to the recall gate (as done for superseded_by/archived_at). Applying that exact idiom to schema_version — which is absent on nearly 100% of existing records at adoption, the inverse cardinality of the optional states it sits next to in the code — would silently exclude every pre-migration record from recall with no error, indistinguishable from an empty store. This must become an explicit negative requirement (a "recall gate blast radius" test) landed in the same phase that introduces the field, not deferred to a hardening pass. A second structural finding narrows the scope of what the migration mechanism can safely promise: fromPayload/payload() are flat, unconditional codecs with no version-dispatch branch, so schema_version alone enables only additive schema evolution (new optional keys) — not renaming, restructuring, or reinterpreting existing keys. That constraint should be stated explicitly in the design, not discovered under pressure by a future migration author. A third finding, orthogonal to correctness but shaping where the sweep is even needed: tolerant read-time decoding (the Phase 03.1 supersedesFromPayload precedent) fully handles representation-only schema changes, but structurally cannot fix a field that participates in a Qdrant filter/index condition — those filters run server-side against the raw stored payload, before decode-time tolerance ever runs. Only filter-relevant fields require the explicit sweep.

The main deployment-facing risk to steer around is the field's dominant default: auto-migration on server startup (Gitea, Grafana, Vaultwarden all do this, and all have documented operator friction from it — Grafana's community forum has repeated threads asking to disable it). That default directly contradicts engram's own "explicit, never automatic" design stance and its already-shipped, audited registerDestructive precedent; the recommended shape is a non-blocking, logged startup warning ("N records at a stale schema version; run engram migrate --apply") with the actual mutation staying a deliberate, registerDestructive-gated operator action. Finally, issue 482 (Connect lane silently omitting five already-shipped state fields for three consecutive milestones) recurred specifically because a clean buf breaking run and green CI were mistaken for proof that memoryToProto was updated — they prove only that the schema is additive, not that the mapping function changed. This milestone must not repeat that mistake a fourth time; the required proof is an exhaustive round-trip test over store.Memory's wire-eligible fields, not a hand-maintained checklist.

## Key Findings

### Recommended Stack

Nothing new to install; go.mod/go.sum should be unchanged by this milestone (verify with git diff go.mod go.sum before merge). The stack decision is entirely about which existing seam each capability extends.

Core technologies (existing, reused):
- Go stdlib encoding/json — schema_version discriminator and migration-report JSON extend store.Memory's existing payload()/fromPayload() round-trip pair; no new marshal mechanism needed.
- github.com/qdrant/go-client v1.18.3 (pinned) — migration writes are a SetPayload single-key merge, the exact shape Store.Supersede's back-stamp already uses; no new query surface.
- github.com/spf13/cobra v1.10.2 (pinned) — engram migrate registers through registerDestructive, the existing choke point migrate-remap-owner/prune-expired already use.

Supporting patterns (named, not imported): the "Schema Versioning Pattern" (document-store literature: version field on the document, absent = legacy, no backfill to adopt) already matches sessionPayloadVersion's shape; eager background-sweep migration (not lazy/migrate-on-read) matches every migration engram has shipped to date; idempotent scan-and-repair with Qdrant itself as the ledger (no separate migrations table) matches BackfillShortIDs/RemapOwner; an ordered step registry as a plain Go slice/const table (not a DAG or file-discovery mechanism), mirroring internal/surfaces's rule registry and toolclass table.

Explicitly rejected: golang-migrate/goose/atlas (relational, .sql-file-based, no Qdrant driver, violates the zero-new-dependency constraint); a new schema_migrations-equivalent Qdrant collection (new infrastructure for a property schema_version less-than target already gives for free — and would additionally violate the locked invariant DEC-2bv, one Qdrant collection ever); lazy migrate-on-read (would smear version-handling across every read call site instead of concentrating it in one sweep command); reflection-based struct-tag walking for the typed renderer (new dependency or hand-rolled reflect walker, inconsistent with this codebase's compile-time-visible-structure preference).
### Expected Features

Must have (table stakes) — all P1 per FEATURES.md's prioritization matrix:
- schema_version discriminator, absent = v0, auto-injected on write / explicit-checked on read
- engram migrate status — a read-only version-distribution histogram (not a single scalar; Qdrant has no single schema-version row, so "current version" is inherently a per-collection distribution), reported before any write
- engram migrate --apply via registerDestructive (preview-by-default, --apply opt-in — reusing, not reinventing, the choke point)
- Ordered migration registry (v0 to v1 chain), subsuming backfill-short-ids as its step body — and explicitly NOT subsuming migrate-remap-owner/summarize-missing/reindex (none of those three are schema-version-driven; folding them in would make the registry's applicability predicate lie)
- Connect record-state proto parity (six additive Memory fields)
- Typed operator renderer (structural fix for the JSON/text drift class issue 481 exists to close)
- Console + CLI surfacing of archived/superseded/scheduled/migration state
- Idempotent re-run and bounded/cancellable runtime (--timeout, SIGTERM) — both already-shipped properties of every existing sweep command, must not regress

Should have (differentiators): per-collection version histogram instead of a single "dirty"-flag-style scalar (sidesteps golang-migrate's best-known operator pain point by construction); migration report routed through the new typed renderer from its first commit rather than a bespoke hand-rolled struct; shared --scope/--all-scopes guard (RuleSweepScopeOrAllScopesRequired, issue 480) reused by migrate rather than a bespoke flag; console/CLI record-state surfacing that no comparable self-hosted tool (Gitea, Grafana, Vaultwarden) exposes in its UI at all.

Defer (v2+): migration progress/resumability tuning for very large collections (no real deployment size data exists yet — v0.13.0 itself is undeployed); migrate status --output json wired into a CI health-check gate; refined precedence rules for compound record state (archived AND superseded simultaneously) — don't pre-guess, wait for real console usage.

Anti-features (rejected explicitly, not by omission): automatic migration on server startup (contradicts the explicit-only design stance and the shipped registerDestructive precedent — use a logged, non-blocking warning instead); down-migrations/rollback (Qdrant has no DDL/schema layer, "down" would mean writing a second payload-mutation function to trust, and it papers over the tension that in-place migration mutation is the one place this milestone diverges from supersession's additive/history-preserving philosophy — use Qdrant snapshot/restore as the documented rollback story instead); retrofitting migrate-remap-owner/summarize-missing/reindex into the version-driven registry (none are schema-version-triggered — already correctly scoped out by PROJECT.md, not an open question); a tolerant decoder as the sole schema-evolution mechanism forever (works only for representation-only changes — see Architecture finding on filter-relevant fields below).

### Architecture Approach

The system's existing three-tier operator-command shape (internal/store does the Qdrant work, cmd/engram is a thin cobra adapter, pure sharable logic lives in its own leaf package) is not being replaced — this milestone is one more instance of it. internal/store gains a SchemaVersion field on Memory plus a new Store.Migrate sweep method (Subject-less, scroll-batch-resume, mirroring BackfillShortIDs/Reindex/RemapOwner). A new pure leaf package, internal/migrate, holds the ordered step registry — zero import of internal/store/qdrant/authz, imported BY internal/store (never the reverse), operating on raw map payloads rather than the typed Memory struct. internal/server memoryToProto gains six additive field mappings from the already-shipped-but-unwired Go fields plus the new SchemaVersion. cmd/engram gets a new file (not a reuse of the existing migrate.go, which already owns migrate-remap-owner) wiring engram migrate through registerDestructive.

Major components:
1. internal/store — owns Memory, the payload codec, every Qdrant filter (authz + recall gates), and the new Store.Migrate sweep (Subject-less, same authz-bypass precedent as Reindex/BackfillShortIDs/RemapOwner)
2. internal/migrate (NEW) — pure, dependency-free ordered step registry; reuses internal/surfaces's sealed-marker plus ValidateX() invariant-checker plus copy-returning accessor discipline, but explicitly rejects its literal anchored-text conformance-gate machinery (a migration step is one executable transform applied once to stored data — there is no second "surface" to byte-compare against)
3. internal/server (connectapi.go) — the single memoryToProto conversion chokepoint gains 6 field mappings, numbers 23 to 28 (message currently ends at citations = 22) — a one-way door under buf breaking CI gating
4. cmd/engram — new file for engram migrate, thin RunE calling exactly st.Migrate(ctx, opts), routed through registerDestructive; renderOperator gets the typed-renderer refactor (issue 481)
5. Operator console (SvelteKit) — currently "cannot render the v0.13.x archive tier at all"; gains archived/superseded/scheduled/schema-version surfacing, gated behind proto parity plus the typed renderer both landing first

Suggested build order (from ARCHITECTURE.md's dependency analysis): (1) gate/CI integrity fixes (issue 479 pattern-escaping bug, issue 497 testcontainer flakiness) must land first because every later phase authors new internal/surfaces key-links and needs a build that can actually go red; (2) record schema versioning foundation (field plus codec plus internal/migrate scaffolding, no sweep/proto/CLI yet — pure and independently testable); (3) the migration mechanism (Store.Migrate plus engram migrate CLI); (4) Connect record-state parity (issue 482, strictly after 2/3 since the Go type must be locked before the one-way proto cut); (5) typed operator renderer (issue 481, can run parallel to 4, must precede 6); (6) console plus CLI surfacing (depends on 4 and 5); (7) registry plus docs tail (RuleSweepScopeOrAllScopesRequired, reference docs, CLAUDE.md's "Not used here" line revision).

### Critical Pitfalls

1. Recall-gate cardinality trap — reusing the IsEmpty(superseded_by)/IsEmpty(archived_at) soft-hide idiom for schema_version would silently exclude ~100% of pre-migration records from recall (absence is the majority state, not a minority optional one, at adoption). schema_version must never appear in any of ownerScopeFilter, activeWindowConditions, or any Search/List/SearchDiscovery/ListScheduled filter-builder. A negative "recall gate blast radius" test is required in the same phase that introduces the field, not a later hardening pass.
2. No version-dispatch codec, meaning additive-only migrations, no rollback path — fromPayload/payload() are flat, unconditional codecs; adding schema_version does not by itself let the codec interpret two different payload shapes. Scope this milestone's migrations to additive-only changes explicitly (in design doc and --help text); flag non-additive schema evolution as a deferred, separate research question rather than something schema_version secretly already solves.
3. Filter-relevant fields structurally require the sweep; representation-only ones don't — tolerant decode (Phase 03.1's supersedesFromPayload precedent) is free and correct for fields never read by a Qdrant filter condition, but has zero effect on filter-condition fields (owner, visibility, superseded_by, archived_at, not_before, not_after, any future indexed key) because those filters run server-side against the raw stored payload before any Go decode ever executes. Audit each new/evolving field against ownerScopeFilter/activeWindowConditions/the two IsEmpty gates before deciding whether a step needs the write-back sweep.
4. Whole-payload Upsert in a migration step drops out-of-band keys (recurrence risk of CR-01) — the migration write primitive must be a targeted SetPayload merge (same shape as SetVisibility/Supersede's back-stamp), never a full Upsert, unless proven to round-trip through payload()/fromPayload() with the current struct definition. Requires a round-trip test seeding every optional field and asserting byte-identical survival.
5. Qdrant multi-ID SetPayload is not atomic (confirmed upstream, qdrant/qdrant issue 9371) — a migration sweep must reconcile by re-derivation (fresh Scroll/Count against schema_version less-than target) after any batch write, never trust the write call's own success/failure signal, mirroring reconcileSupersedeFailure (D-15).
6. buf breaking passing plus code compiling is NOT evidence the wire mapping is correct — this exact failure (issue 482) has already recurred three times: proto extended, buf breaking green, CI green, while memoryToProto silently never mapped the new fields. Requires an exhaustive round-trip test (not a hand-maintained field list) landed in the same phase as the field additions.
## Implications for Roadmap

Research strongly converges on the seven-step build order ARCHITECTURE.md derives from the dependency graph, cross-validated against FEATURES.md's dependency notes and PITFALLS.md's phase-mapping table. All three research files independently arrive at the same phase boundaries — this is a rare case of unanimous agreement across tracks, which should carry weight in the roadmap.

### Phase 1: Gate and CI Integrity (prerequisite)
Rationale: Every later phase authors new internal/surfaces key-links (at minimum RuleSweepScopeOrAllScopesRequired) and new partial-failure-injection tests. Issue 479 (a pattern escaping bug that silently no-ops key-link gates) and issue 497 (Qdrant testcontainer flakiness) must both be fixed and proven fixed first, or later phases either author silently-broken gates or get real regressions waved off as "probably infra."
Delivers: A build that can actually go red for the work this milestone is about to add.
Avoids: Pitfalls 9, 10.

### Phase 2: Record Schema Versioning Foundation
Rationale: Pure and independently testable — no Qdrant sweep, no proto, no CLI needed yet. Must land before the migration mechanism (which needs schema_version to filter on) and before the proto cut (which needs the Go type locked, since proto field numbers are a one-way door).
Delivers: store.Memory.SchemaVersion, the current-version constant, payload()/fromPayload() wiring (server-set-only stamp on write; absent-reads-as-v0 on read, mirroring sessionPayloadVersion but not mirroring its reject-on-mismatch half — a memory record is a persistent asset, not a disposable session). Scaffolds the new internal/migrate leaf package (sealed-marker registry shape plus ValidateSteps()), even with zero steps registered yet.
Addresses: schema_version discriminator (FEATURES.md P1).
Avoids: Pitfall 1 (negative recall-gate test, must land in THIS phase), Pitfall 11 (contentFingerprint reflex-add), Pitfall 14 (write-path/sweep-timing gap — write-path auto-injection must land in the same commit as the field, proven by a test that 100% of post-commit writes carry a non-absent version).

### Phase 3: Migration Mechanism (engram migrate)
Rationale: Depends on Phase 2's schema_version field and internal/migrate scaffold. Independent of the proto/console work — this is pure store plus CLI.
Delivers: Store.Migrate sweep method (Subject-less, scroll-batch-resume, range-based eligibility filter less-than target not equality zero); new cmd/engram file (not migrate.go, which is already migrate-remap-owner's) wiring the command through registerDestructive with --apply as opt-in, preview as bare-invocation default — a deliberate behavior change from backfill-short-ids's inverted --dry-run default, called out in the upgrade guide. Subsumes (soft-deprecates, does not delete) backfill-short-ids.
Uses: registerDestructive, targeted SetPayload (never Upsert), qdrantPayloadOpBatchSize-aware batching with reconcile-by-re-derivation.
Avoids: Pitfalls 3, 4, 5, 7, 8, 13, 15.

### Phase 4: Connect Record-State Parity (issue 482)
Rationale: Strictly after Phase 2/3 — schema_version's Go and wire type must be fully locked before the proto cut, since field numbers 23 to 28 are a permanent one-way door once shipped.
Delivers: Six additive Memory proto fields (five already exist in Go, unwired; schema_version is the one genuinely new field); memoryToProto mapping; buf breaking stays clean.
Implements: Connect wire chokepoint (internal/server/connectapi.go).
Avoids: Pitfall 6 (exhaustive round-trip test, not a hand-maintained list — required in this SAME phase, not a follow-up), Pitfall 12 (not_before/not_after outward-rounding parity between write path and new read path, boundary-second test).

### Phase 5: Typed Operator Renderer (issue 481)
Rationale: Can run in parallel with Phase 4 (orthogonal files, no shared code), but must complete before Phase 6 — every operator command rendering the six new fields needs the JSON/text-drift-proof shape to exist first, not retrofitted after the fields already flow through the old renderer.
Delivers: renderOperator structural refactor — a shared Field slice walked by both Text() and MarshalJSON(), making json-wider-than-text structurally unrepresentable rather than merely detectable-by-test.
Uses: Concrete Field-slice construction (rejects reflection-based tag-walking and a fully generic OperatorReport wrapper — see STACK.md's alternatives table).

### Phase 6: Console and CLI State Surfacing
Rationale: Depends on Phase 4 (fields must exist on the wire) and Phase 5 (typed renderer must exist to receive them safely) — both are hard blockers, not soft ones.
Delivers: Operator console renders archived/superseded/scheduled/schema-version state (today it "cannot render the v0.13.x archive tier at all"); CLI surfaces the same through the typed renderer.
Addresses: FEATURES.md's named differentiator — no comparable self-hosted tool (Gitea/Grafana/Vaultwarden) exposes this in its UI.

### Phase 7: Registry and Docs Tail
Rationale: Trailing cleanup. RuleSweepScopeOrAllScopesRequired registration has no forward dependency on the other phases and could move earlier, but documenting a mechanism before it exists invites drift — keep it last.
Delivers: Shared --scope/--all-scopes guard registered and reused by migrate, summarize-missing, spine-review scan; reference/memory-record.md and reference/tools.md updated; CLAUDE.md's "Not used here: database migrations" line revised to state what IS now used and its scope.

### Phase Ordering Rationale

- The gate/CI-integrity phase is first because it is a structural prerequisite for trusting every subsequent phase's tests, independently confirmed by both ARCHITECTURE.md's build order and PITFALLS.md's phase-mapping table (pitfalls 9, 10).
- Schema versioning precedes the migration mechanism because the sweep needs a field to filter on; both precede the proto cut because proto field numbers are irrevocable once shipped and the Go representation must be settled first.
- The typed renderer and Connect parity phases are mutually independent (no shared files) but both must complete before console/CLI surfacing, which depends on both.
- This ordering directly avoids the recall-gate cardinality trap (by putting the negative test in the phase that introduces the field, not later) and the issue 482 recurrence (by putting the exhaustive round-trip test in the same phase as the field additions, not a follow-up).

### Research Flags

Needs deeper research during planning:
- Phase 3 (migration mechanism): the ordered step-registry pattern is a designed shape, not an imported one (MEDIUM confidence in STACK.md) — the exact internal/migrate API surface (step struct shape, ValidateSteps() invariants, StepsFrom(v int) helper) should be nailed down at plan time, and the partial-failure-resume test design (Pitfall 4/5) needs explicit attention since it is the highest-complexity test in this milestone.
- Phase 6 (console plus CLI surfacing): the operator-UI soft-hidden-state conventions (archived/superseded/scheduled badges, precedence rules for compound state) are synthesized from general product convention, not a single citable spec (MEDIUM confidence in FEATURES.md) — validate against real console usage rather than pre-guessing precedence for compound states.

Phases with standard, well-documented in-repo patterns (skip research-phase):
- Phase 2 (schema versioning foundation): directly mirrors the already-shipped sessionPayloadVersion pattern, HIGH confidence, primary source read directly.
- Phase 4 (Connect parity): directly mirrors the existing memoryToProto additive-field pattern and buf breaking CI gate, HIGH confidence.
- Phase 5 (typed renderer): the Field-slice pattern is a straightforward internal refactor with a clear precedent in issue 481's own analysis, HIGH confidence.
## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH (schema-version discriminator, Qdrant payload-versioning affordances, typed-renderer field-set-by-construction) / MEDIUM (exact migration-step registry shape — no canonical Go library exists for a db-less, Qdrant-native migration ledger) | All claims read directly from in-repo code; the one genuinely designed (not sourced) piece is the step-registry API shape |
| Features | MEDIUM | Cross-checked against multiple named tools' own docs/community reports for migration-CLI conventions (goose/golang-migrate/Atlas, Gitea/Grafana/Vaultwarden); the operator-UI soft-hidden-state guidance is synthesized from widely-replicated product convention, not a single citable spec |
| Architecture | HIGH | Every claim read directly off the live internal/store, internal/surfaces, internal/webauth, cmd/engram, proto/engram/v1 trees at HEAD — not inferred from a generic pattern |
| Pitfalls | HIGH (Qdrant batch-atomicity confirmed against an open upstream issue; protobuf field-number permanence confirmed against protobuf.dev/buf.build docs; all store-layer claims read directly) / MEDIUM (schema-version-codec design space — few close precedents for "flat struct plus payload-only codec plus no version-dispatch" systems, reasoned from this repo's own sessionPayloadVersion precedent plus general JSON-document-migration literature) | |

Overall confidence: HIGH — this is a well-precedented internal extension of an already-shipped system, not greenfield design; the MEDIUM-confidence areas are narrow (the exact step-registry API shape, and general document-store migration literature used only to name already-independently-discovered patterns).

### Gaps to Address

- backfill-short-ids deprecation shape (open decision): ARCHITECTURE.md recommends the soft, discoverable-alias shape migrate-set-owner already established (Deprecated equals "use: ...", never a hard removal) as a recommendation grounded in codebase practice, not a PROJECT.md requirement. Confirm this explicitly at plan time for Phase 3 rather than assuming it.
- Whether internal/migrate ships with zero registered steps at Phase 2, or Phase 2 and 3 merge (open decision): STACK.md and ARCHITECTURE.md both frame Phase 2 as scaffolding the package "even before any real step is registered" — no concrete v0 to v1 transform is named anywhere in the research (the closest is backfill-short-ids's subsumption, which happens in Phase 3). Decide at plan time whether Phase 2 ships an empty-but-validated registry or whether the first real step (backfill-short-ids subsumption) lands in the same phase.
- Additive-only migrations: written scope constraint vs. attempted version-dispatch codec (open decision, PITFALLS.md Pitfall 2): the research recommends stating "additive-only" explicitly in the design doc and --help text rather than attempting a real version-dispatch codec this milestone. This should be a deliberate, written decision at plan time, not an implicit assumption a future author discovers is false under pressure.
- Preview/apply exact parity: advisory-only vs. hard requirement (open decision, Pitfall 8): the existing prune.go pattern already has this gap (preview and apply independently re-query Qdrant, no shared snapshot). The research offers two paths — document as advisory-only (matching existing precedent), or treat exact parity as new scope requiring a two-consecutive-run parity test. This needs an explicit decision, not inheritance-by-default, since the two paths have materially different implementation costs.
- No real production-scale data exists for sizing migration-sweep performance or progress-reporting needs (v0.13.0 itself is undeployed) — defer progress/resumability tuning to v1.x per FEATURES.md's MVP framing, and flag this as an assumption rather than a proven-sufficient design.

## Sources

### Primary (HIGH confidence)
- internal/store/store.go, internal/store/spine.go (this repo, read directly) — Memory, payload()/fromPayload(), filter construction, BackfillShortIDs/RemapOwner/Reindex/Supersede, qdrantPayloadOpBatchSize
- internal/webauth/session.go, resolver.go, reseal.go (this repo) — sessionPayloadVersion precedent
- internal/surfaces/rules.go, toolclass.go, surfaces.go (this repo) — sealed-registry discipline
- cmd/engram/backfill.go, migrate.go, reindex.go, destructive.go, prune.go (this repo) — operator-command conventions
- internal/server/connectapi.go (memoryToProto), internal/server/idempotency.go (contentFingerprint) (this repo)
- proto/engram/v1/engram.proto (this repo)
- .planning/PROJECT.md, .planning/intel/decisions.md (DEC-2bv, DEC-xa6) (this repo)
- GitHub issues 481, 482 (this repo, read via gh issue view)
- Qdrant issue qdrant/qdrant 9371 — batch non-atomicity, maintainer confirmation
- protobuf.dev, buf.build docs — field-number permanence and breaking-change categories

### Secondary (MEDIUM confidence)
- Gitea/Grafana/Vaultwarden docs and community forums — auto-migration-on-startup default and its documented operator friction
- goose/golang-migrate/Atlas docs (atlasgo.io, pkg.go.dev) — status/dry-run/version verb conventions, the "dirty flag" pain point
- Qdrant official docs (Fundamentals FAQ, embedding-model-migration tutorial) — confirms no built-in payload schema versioning, migration tooling scoped to vectors not payload shape
- bool.dev, jsonic.io — named "Schema Versioning Pattern" and lazy-vs-eager migration dichotomy, corroborating already-independently-derived in-repo pattern
- docs.ditto.live — schema_version-discriminator vs. separate-collection pattern for breaking changes
- Code With Karani — idempotent/resumable migration design (general SQL-migration domain, adapted)
- ArgoCD PR 27664 — precedent for "preview output actively lying" as a recognized destructive-UX risk class

### Tertiary (LOW confidence)
- go-render (github.com/jimeh/go-render) — surfaced only as a rejected reflection-based alternative for the typed renderer, not independently vetted for production use

---
Research completed: 2026-08-12
Ready for roadmap: yes
