# Conflict Detection Report

Ingest of 50 classified planning docs (25 ADR @ precedence 0, all LOCKED; 25 SPEC @
precedence 1). Cross-ref graph checked for cycles (3-color DFS, depth cap 50) — none found.
No UNKNOWN/low-confidence docs. No PRDs (no competing acceptance variants possible).

## BLOCKERS (0)

None. No two LOCKED ADRs contradict on a shared scope; the 25 ADRs cover disjoint decision
scopes. Mode is `new` with no existing locked CONTEXT.md, so no ingest-vs-existing lock
conflicts are possible.

### WARNINGS (0)

None. No PRDs are present, so there are no divergent acceptance criteria / competing variants
to resolve. All SPEC↔ADR overlaps resolve cleanly by precedence (see INFO).

### INFO (6)

[INFO] Expected ADR↔SPEC design-doc overlap — auto-resolved by precedence
  Found: 20 of 25 SPECs are the design docs behind a locked ADR on the same scope (e.g.
    short-id-handle-design ↔ engram-zzq0/engram-02ta; per-actor-memory-isolation-design +
    typed-subject-authz-core-design ↔ engram-cgb/kyz/xa6/y1g/g37x/12c; scheduled-memories ↔
    engram-90w/y1g; windowed-cursor-recall ↔ engram-1frj/ef28; auto-summary ↔ engram-ambu).
  Note: This is NOT a conflict. ADR wins on the locked decision (precedence 0); the SPEC
    supplies the WHAT/context/requirements. Full pairing map in intel/context.md.

[INFO] Auto-resolved: DEC-g37x (ADR) governs configurable-claim owner over its SPEC
  Found: SPEC 2026-06-29-configurable-claim-owner-design.md states it supersedes ADR
    engram-hvg (not in set) and moves the owner key to a configurable claim (default email).
  Note: The in-set LOCKED ADR engram-g37x encodes the same decision — SPEC and in-set ADR
    agree; ADR governs. engram-hvg is out-of-set historical context.

[INFO] Historical: client-config-generalization SPEC superseded by out-of-set ADR engram-50b
  Found: SPEC 2026-06-02-generalize-engram-client-config-design.md (deployment-neutral client
    config + /engram-setup, four auth postures).
  Note: Later superseded by ADR engram-50b, which is NOT in this ingest set. Treated as
    historical context only (intel/context.md), not routed as an active requirement.

[INFO] Historical: vitest-browser-mode SPEC supersedes out-of-set decision engram-cv92
  Found: SPEC 2026-06-27-vitest-browser-mode-ui-test-unification-design.md (Status DRAFT)
    references and supersedes decision engram-cv92 (not in set).
  Note: Active direction captured as CON-ui-real-dom-tests / REQ-ui-test-unification;
    engram-cv92 is historical.

[INFO] Deferred requirements carried forward: Connect API auth posture R1–R4
  Found: SPEC 2026-06-09-connect-auth-posture-addendum.md records an interim anonymous Connect
    API mount into the single empty-owner bucket, with cookie/OIDC observe-lane auth
    requirements R1–R4 explicitly DEFERRED.
  Note: Not a conflict; flagged so the roadmapper carries R1–R4 forward before the observe
    lane is exposed to real identities (REQ-connect-auth-posture, CON-connect-auth-deferred).

[INFO] Dangling cross-references to out-of-set decisions
  Found: SPEC cross_refs point to ADRs/decisions not in this ingest set — engram-hvg,
    engram-lkm, engram-3hp9, engram-cv92, engram-50b.
  Note: Recorded for traceability in intel/context.md; none block synthesis. If any encode a
    still-active decision, add them to a follow-up ingest via --manifest.

---

## Follow-up merge — 2026-07-08 (`/gsd-ingest-docs --mode merge`)

Folded the docs left out of the original 50-doc bootstrap. Two passes, both conflict-clean;
no destination file was blocked.

**Pass A — 31 companion ADRs** (precedence 0, all LOCKED): 0 BLOCKERS, 0 WARNINGS, 11 INFO
(all auto-resolved as refinements of the 25 baseline locks). engram-50b adjudicated CONSISTENT
with the bundled skill/engram plugin (removes only the auto-registering .mcp.json). No cross-ref
cycles. Detail: `.planning/intel/merge-adrs/INGEST-CONFLICTS.md`.

**Pass B — 24 implementation plans** (precedence 3, DOC/context): 0 BLOCKERS, 0 WARNINGS, 0 INFO;
24 traceability context topics grouped by phase (including the deferred Phase 8 cookie/OIDC
auth-lane plan). Detail: `.planning/intel/merge-plans/INGEST-CONFLICTS.md`.

Manifests: `.planning/intel/merge-adrs.manifest.yaml`, `.planning/intel/merge-plans.manifest.yaml`.
Superseded ADRs (engram-lkm / engram-hvg / engram-e38 / engram-1xv) remain deliberately excluded.
