---
phase: 05
slug: validation-debt-reconciliation
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-12
---

# Phase 05 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Register origin: authored at plan time. Both `05-01-PLAN.md` and `05-02-PLAN.md` carry a
`<threat_model>` block, so this audit **verifies the declared mitigations exist** rather than
building a register retroactively. Neither SUMMARY declared a `## Threat Flags` section, so the
register below is the union of the two plan registers (the shared supply-chain row `T-05-SC`
appears in both and is recorded once).

Scope note, load-bearing for every severity call below: this phase changed **no executable
behaviour**. Its entire code-tree delta is comments inside one `_test.go` file plus one markdown
table cell; everything else is `.planning/**` prose. Nothing here reaches the binary, no runtime
trust boundary is crossed, and no dependency manifest was touched.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| planning record → future reader | A `VALIDATION.md` is the only durable evidence that a shipped requirement was actually tested; an auditor, `/gsd-audit-milestone`, or a future maintainer trusts it without re-deriving it | Test-coverage claims, requirement verdicts (non-sensitive, repo-public) |
| planning artifact → GSD parser | `ROADMAP.md` is read by scope-sensitive parsers; an invented heading changes what the tool sees with no error and no warning | Milestone scope, phase membership, progress state |
| published docs → operator | `embedding-instructions.md` is user-facing configuration guidance; a wrong cross-ref sends an operator to a page that does not answer the question | Provider configuration guidance (public) |

No runtime trust boundary is crossed by this phase.

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-05-01 | Tampering | `03.1`/`01`/`02`-VALIDATION.md rows | high | mitigate | Every reconciled row's pattern re-resolved against `go test -list '.*' ./...` at HEAD before its status changed; a zero-resolving element halts the task instead of being routed around. **Verified:** 23 pattern elements across the three files re-resolved live (1,047 names at execution, 1,047 at re-verification); zero unresolved. Each file carries a dated `## Validation Audit 2026-08-11` section recording the method | closed |
| T-05-02 | Repudiation | `04-VALIDATION.md` `REQ-consent-adversarial-proof` row | high | mitigate | The row, its explanatory paragraph and `nyquist_compliant: false` declared out of bounds for every task. **Verified:** `rg -c 'REQ-consent-adversarial-proof.*⬜ pending'` → exactly 1; `nyquist_compliant: false` intact; the sibling manual row for `REQ-consent-never-perform` also still `⬜ pending`. The unproven requirement remains visibly unproven | closed |
| T-05-03 | Tampering | `.planning/milestones/**` | medium | mitigate | No task listed an archived path in `<files>`. **Verified:** `git diff --name-only 3d1c643b..HEAD -- .planning/milestones/` → 0 files, checked after every commit including the two post-review fixes | closed |
| T-05-04 | Information disclosure | validation prose | low | accept | Records name test functions and commit SHAs already public in this repository | closed (accepted) |
| T-05-05 | Tampering | `.planning/ROADMAP.md` structure | high | mitigate | Scoped `Edit` only, never `Write`. **Verified:** `^### Phase ` heading count 6, unchanged from base `3d1c643b`; zero version-bearing or ✅-bearing `###` headings outside `<details>`; `roadmap.validate` → `{"warnings":[]}`; all `**v0.N.x —` milestone-summary lines byte-identical to base. Exactly one Progress row differs from base — v0.13.x Phase 5's own — which is this phase's own completion entry and therefore inside the prohibition's "outside Phase 5's own entry" carve-out. See the incident note below | closed |
| T-05-06 | Tampering | `internal/retrievaleval/retrieval_eval_test.go` | medium | mitigate | Comment-only edit; `go vet` and `gofmt -l` asserted clean, and `git diff --stat -- cmd/engram/` asserted empty to catch `task fmt` collateral. **Verified:** every changed line in the file's diff is a comment line — no code line changed; `gofmt -l internal/retrievaleval/` → 0 files; `cmd/engram/` → 0 files changed across the whole phase | closed |
| T-05-07 | Information disclosure | `docs-site` guidance | low | accept | The edited row names a public provider (OpenRouter) and links a public guide page | closed (accepted) |
| T-05-SC | Tampering | npm/pip/cargo/go installs | low | accept | No package-manager install task in either plan; nothing to audit. **Verified:** no dependency manifest (`go.mod`, `go.sum`, `package.json`, `package-lock.json`, `requirements.txt`, `Cargo.toml`, `Cargo.lock`, `pyproject.toml`) appears in `git diff --name-only 3d1c643b..HEAD` | closed (accepted) |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Incident note — T-05-05, transient corruption of a shipped milestone's record

Recorded because it is the one place in this phase where a tampering threat's target was, in fact,
briefly modified — and a register that only showed the clean end state would be exactly the false
green this phase existed to close.

This phase's own GSD tracking write (`82ae50f9`) flipped the **shipped v0.12.x** Phase 5 Progress
row from `Complete | 2026-08-01` to `In Progress|  `, and left v0.13.x Phase 5's own row untouched
at `0/2 | Not started`. `phase.complete 05` later targeted the same wrong row a second time
(harmlessly — by then the row already read `Complete`, so the rewrite only re-padded whitespace).

Cause is upstream, not a plan violation: `rowMatch` in gsd-core `src/phase.cts` (~1988) matches a
Progress row by bare leading numeral and ignores the `Milestone` column, and phase numbering
restarts per milestone in this repo (rule `rvmts69cz1`). Both mutations were made by GSD's own
verbs, not by any task's `Edit`.

Disposition: the row this phase damaged was restored byte-identical to base in `3f4c4602`, and
v0.13.x Phase 5's own row was given the value `phase.complete` failed to write. Three other v0.12.x
rows carry the same defect from earlier phases (Phase 1 blank-dated; Phases 2 and 4 wearing a
plausible `Complete` with v0.13.x dates) — these are pre-existing, were explicitly deferred by the
user during the phase-05 discussion, and are **not** phase-05 threats. Tracked in durable memory as
`dffmk92a8q`, extended by `cvvrwjbsnz`; by user decision it is spine-tracked and not filed upstream.

Standing guard for any future ROADMAP-touching tracking commit:

```
git diff <before>..<after> -- .planning/ROADMAP.md | rg '^[+-]\| '
```

Revert any changed row whose `Milestone` cell is not the active milestone, and cross-check every
`Complete` row's date against its milestone's ship date — a date later than the ship date is this
defect wearing a healthy-looking disguise.

---

## Audit note — an unsatisfiable acceptance criterion (not a threat)

Recorded so a future reader does not mistake it for an open finding. Both plans asserted that
`rg -c '⬜ pending'` should report **no match** in `01`/`02`/`03.1`-VALIDATION.md. That check can
never pass on a correctly-reconciled file: the legend line directly beneath each table
(`*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*`) defines the marker vocabulary and therefore
necessarily contains the string. This audit confirmed all three matches are that legend line and
none is a table row — the boilerplate-vs-rows trap. The substantive property (no `⬜ pending` table
row remains in those three files) holds. No security impact; noted as a criterion-authoring defect.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-05-01 | T-05-04 | Validation prose names test functions and commit SHAs that are already public in this repository; no non-public identifier is introduced | Sean | 2026-08-12 |
| R-05-02 | T-05-07 | The edited docs row names a public provider and links a public guide page already published on the docs site | Sean | 2026-08-12 |
| R-05-03 | T-05-SC | No package-manager install task exists in either plan and no dependency manifest was modified, so there is no supply-chain surface to audit for this phase | Sean | 2026-08-12 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-12 | 8 | 8 | 0 | `/gsd-secure-phase 5` (orchestrator, ASVS L1 verification; short-circuit per secure-phase.md §3 — register authored at plan time, threats_open 0, asvs_level 1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-12
