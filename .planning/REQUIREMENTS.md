# Requirements: engram — v0.13.x "Curation & Self-Evidence"

**Defined:** 2026-08-03
**Core Value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its
context, and wrong or stale memories can be corrected or superseded, so recall stays trustworthy as
the store grows.

**Milestone goal:** Make the memory spine maintainable without hand-curation, and make every
command, flag, and tool argument correct-by-reading — closing the two classes v0.12.x left to human
diligence.

**Standing constraints** (confirmed by research, not open questions):

- **Near-zero new Go dependencies.** Every capability in this milestone resolves to the stdlib or
  already-vendored code (`cobra` v1.10.2, `qdrant/go-client` v1.18.3). v0.12.x shipped seven phases
  with zero.

- **Authorization stays in `internal/store`.** `spine-review` is the sixth instance of the existing
  Subject-less operator tier (`reindex`, `migrate-remap-owner`, `prune-expired`,
  `summarize-missing`, `backfill-short-ids`) — not a new authz path. Composing the Subject-gated
  `Search`/`List` would silently scope an operator sweep to one actor.

- **No auto-extraction, no auto-mutation.** Every destructive or semantic judgment is proposed and
  consented to, never performed unilaterally. `store_rule`'s consent gate is the in-repo template.

- **MCP ↔ Connect parity** holds for anything touching the shared business-logic layer.

## v1 Requirements

Requirements for milestone v0.13.x. Each maps to exactly one roadmap phase.

### Interface Enforceability

- [ ] **REQ-flag-exclusivity-enforced**: Every documented mutually-exclusive flag combination on the
  `engram` CLI is rejected before any network call, using cobra's declarative flag-group API rather
  than a hand-rolled guard, so the constraint is enforced at the same place it is declared. (#453)

- [ ] **REQ-exit-code-unified**: Every command in the `engram` binary — client verbs and operator
  commands alike — resolves errors through one exit-code taxonomy (0 success, 2 usage/validation, 4
  not found, 5 unavailable), including errors raised by cobra's own flag-group validation, which
  today bypass `cliError`/`ExitCode()` and fall through to 1. (#467)

- [x] **REQ-exit-code-migration-safe**: The exit-code change ships with a table-driven regression
  test pinning the current behavior of every affected command *before* the change, an audit of
  known consumers, and a `guides/upgrade.md` entry naming every command whose exit status changes.
  (#467)

- [ ] **REQ-cli-request-timeout**: Every `engram` CLI RPC path applies a finite, operator-
  configurable deadline, so a hung or half-open server cannot block an invocation indefinitely; a
  timeout is reported with a documented exit code. (#452)

- [ ] **REQ-client-config-unified**: Every `engram` client flag/setting — `--server`,
  `--token-file`, `--output`, `--insecure`, and the new `--timeout` — resolves through the
  `internal/config` koanf registry rather than a per-setting hand-rolled resolver, so client
  configuration is declared in the same single source of truth that already owns the `ENGRAM_`
  server vars. (#452)

### Interface Discoverability

- [ ] **REQ-conditional-rules-stated**: Every server-side conditional-requirement rule (e.g.
  `effectiveSearchScope`'s "scope is required unless cross_spine is true") is stated wherever its
  argument is advertised — in cobra `Usage` text and in the `internal/server` jsonschema tag — so a
  caller learns the rule by reading rather than by triggering it.

- [ ] **REQ-surface-conformance-gate**: A conformance test asserts that each named conditional rule
  appears on both independent surfaces (the cobra tree, which already feeds `--help` and the
  self-describe catalog from one source via `buildCatalog`, and the MCP arg-struct tags), and fails
  CI when they diverge.

- [ ] **REQ-mcp-tool-annotations**: Every MCP tool declares `readOnlyHint` / `destructiveHint` /
  `idempotentHint`, so an agent can classify a tool's blast radius before calling it.

- [ ] **REQ-help-output-pinned**: Every command's `--help` output is pinned by a golden-file test,
  so an unreviewed change to the interface's primary teaching surface fails CI.

### Spine Curation — Structural (CLI)

- [ ] **REQ-spine-scan**: `engram spine-review scan` enumerates a memory spine through the existing
  Subject-less operator tier and reports inventory and health signals by scope and category, with
  no mutation on any path.

- [ ] **REQ-citation-drift-verify**: `engram spine-review verify` checks every stored citation
  anchor against the content cached in its `Excerpt` at write time, and classifies each as valid,
  moved-but-valid, or broken — with the moved tier reported separately from the broken tier, so
  ordinary refactoring does not train an operator to ignore the verifier.

- [ ] **REQ-near-duplicate-report**: `engram spine-review consolidate` reports near-duplicate
  candidates by querying Qdrant with records' already-stored vectors (no re-embedding), and never
  merges or mutates — the operator or an agent decides.

- [ ] **REQ-purge-extract-gated**: `engram spine-review purge` previews by default and mutates only
  under an explicit `--apply`, re-derives eligibility at apply time rather than acting on a stale
  candidate list, and refuses to run unless rule `7smp8vy9hr`'s extract-before-delete ordering is
  provably satisfied.

- [ ] **REQ-archive-tier**: A record can be archived — removed from recall but retained and
  restorable — as a state distinct from both supersession's soft-hide and purge's irreversible
  delete. *Open at definition time: whether this needs a genuine fourth record state or can extend
  `prune-expired`'s existing soft-hide shape. To be resolved in phase planning, not mid-build.*

### Spine Curation — Semantic (Skill)

- [ ] **REQ-semantic-curation-skill**: An agent skill judges record staleness ("is this still true
  against the tree it describes") and near-duplicate identity ("are these the same fact") using only
  already-shipped MCP tools, requiring zero new server-side code.

- [ ] **REQ-consent-never-perform**: Every mutation the skill identifies is proposed for user
  blessing and never performed unilaterally, reusing `store_rule`'s consent protocol rather than
  inventing a second consent shape.

- [ ] **REQ-consent-adversarial-proof**: The consent gate is proven by a cold-read test in which a
  confident, plausible, and *wrong* proposal still stops at consent — not merely by a test that a
  correct proposal is offered.

### Validation Debt

- [ ] **REQ-nyquist-reconciled**: Every v0.12.x `VALIDATION.md` row (six at `status: draft`, plus
  the one phase with no file) is re-resolved against `go test -list` with a nonzero, expected match
  count — not merely re-run and checked for exit 0 — and every v0.13.x phase reconciles its own
  before closing.

- [ ] **REQ-citation-fixture-355**: #355's drifted `tools.go` citation anchors are repaired, and the
  repair is used to calibrate `verify`'s false-positive rate before it ships.

## v2 Requirements

Deferred to a future milestone. Tracked but not in this roadmap.

### Curation

- **REQ-decay-scoring**: Access-frequency or recency decay as an automated purge-eligibility signal.
  Deferred: every surveyed system that automated this eroded trust; v0.13.x establishes the
  report-and-consent loop first.

- **REQ-auto-promotion**: Automatic promotion of a `gotcha` to a `rule` on a structural signal.
  Deferred: rules are user-blessed ground truth by design; research found no surviving auto-promote
  precedent anywhere.

- **REQ-git-blame-drift-escalation**: Escalating citation drift detection to commit-level line
  history (`git log -L`) when the `Excerpt` diff is inconclusive. Deferred until the `Excerpt`
  comparison proves insufficient in practice.

### Interface

- **REQ-surface-codegen-unification**: Generating the MCP arg-struct docs and the cobra tree from a
  single declaration. Deferred: research judged a conformance test proportionate and codegen
  over-engineered for two surfaces.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Auto-merge of near-duplicates above a similarity threshold | Every system surveyed showed threshold auto-merge silently destroys provenance, exceptions, and version distinctions. `consolidate` reports; it never merges. |
| Automatic supersession on a similarity hunch | Already a standing prohibition — supersession is an explicit correction of a specific identified record, never a write-through side effect. |
| Auto-extraction of memories from conversation or code | The project's foundational design intent across five milestones. Not revisited. |
| Superseding a rule | Rules are normative ground truth; the shipped contract is delete-instead. `spine-review` must not add a back door. |
| A new authorization path for operator sweeps | Authorization lives in `internal/store`. `spine-review` extends the existing Subject-less operator tier or it does not ship. |
| New Go dependencies for any capability in this milestone | Research confirmed stdlib + already-vendored `cobra`/`qdrant-go-client` suffice for all four capabilities. |
| Qdrant snapshot/backup tooling | Named as an open question by research (it is the assumed safety net behind purge recovery), but it is infrastructure, not curation. If absent, `REQ-purge-extract-gated`'s precondition gate carries the full weight — and that is the accepted posture. |

## Traceability

Which phases cover which requirements. Filled during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| REQ-flag-exclusivity-enforced | Phase 1 | Pending |
| REQ-exit-code-unified | Phase 1 | Pending |
| REQ-exit-code-migration-safe | Phase 1 | Complete |
| REQ-cli-request-timeout | Phase 1 | Pending |
| REQ-client-config-unified | Phase 1 | Pending |
| REQ-conditional-rules-stated | Phase 2 | Pending |
| REQ-surface-conformance-gate | Phase 2 | Pending |
| REQ-mcp-tool-annotations | Phase 2 | Pending |
| REQ-help-output-pinned | Phase 2 | Pending |
| REQ-spine-scan | Phase 3 | Pending |
| REQ-citation-drift-verify | Phase 3 | Pending |
| REQ-near-duplicate-report | Phase 3 | Pending |
| REQ-purge-extract-gated | Phase 3 | Pending |
| REQ-archive-tier | Phase 3 | Pending |
| REQ-semantic-curation-skill | Phase 4 | Pending |
| REQ-consent-never-perform | Phase 4 | Pending |
| REQ-consent-adversarial-proof | Phase 4 | Pending |
| REQ-nyquist-reconciled | Phase 5 | Pending |
| REQ-citation-fixture-355 | Phase 5 | Pending |
