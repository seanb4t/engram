---
phase: 2
slug: interface-discoverability
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-04
validated: 2026-08-11
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib), driven by `task` |
| **Config file** | `Taskfile.yaml` |
| **Quick run command** | `go test ./internal/server/... ./cmd/engram/...` |
| **Full suite command** | `go clean -testcache && task` (lint + test) |
| **Estimated runtime** | ~60–180 seconds full; ~20s quick |

**Test-cache hazard (memory `p1vqxqhxrm`):** Go caches test results on a package's own
inputs. `internal/e2e` shells out to the built binary, so a change in `cmd/engram` does
**not** invalidate its cache entry. This phase adds golden tests over CLI output, which
are subject to the same staleness. Every phase-completion and pre-PR gate MUST run
`go clean -testcache` first.

---

## Sampling Rate

- **After every task commit:** Run the quick command scoped to the touched package
- **After every plan wave:** Run `go clean -testcache && task`
- **Before `/gsd-verify-work`:** Full suite must be green after a cache clean
- **Max feedback latency:** 180 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | Tests Matched | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | REQ-conditional-rules-stated | see 02-01 `<threat_model>` | Generated region content is provenance-bounded — a rule sentence reaching a published surface came from the registry, not from arbitrary input | unit (tracer) | `go test ./internal/surfaces/... -run TestValidateRules -v -count=1` | 4 | ✅ green |
| 02-01-02 | 01 | 1 | REQ-conditional-rules-stated | see 02-01 `<threat_model>` | Drift job cannot be satisfied by a hand-edited region — regeneration is the only clean path | integration | `go clean -testcache && go test ./internal/surfaces/... ./internal/server/... ./cmd/engram/... -count=1 && go run ./internal/surfacesgen && git diff --exit-code -- docs-site/` | n/a | ✅ green |
| 02-02-01 | 02 | 2 | REQ-surface-conformance-gate | see 02-02 `<threat_model>` | Tool enumeration does not require live Qdrant/embedder credentials | unit (tdd) | `go clean -testcache && go test ./internal/server/... -run TestRegisterToolsEnumerable -v -count=1 && go test ./internal/server/... -count=1` | 1 | ✅ green |
| 02-02-02 | 02 | 2 | REQ-surface-conformance-gate | see 02-02 `<threat_model>` | A rule resolving to zero applicable surfaces fails loudly instead of passing vacuously | unit (tdd) | `go clean -testcache && go test ./internal/surfaces/... -run 'TestNormalizeFieldRoundTrip\|TestEveryRuleResolvesToNonEmptySurfaceSet' -v -count=1` | 2 | ✅ green |
| 02-02-03 | 02 | 2 | REQ-surface-conformance-gate | see 02-02 `<threat_model>` | Divergence between any two bound surfaces fails CI | integration | `go clean -testcache && go test ./internal/surfaces/... ./internal/server/... ./cmd/engram/... -count=1` | n/a | ✅ green |
| 02-03-01 | 03 | 3 | REQ-conditional-rules-stated | see 02-03 `<threat_model>` | Published `hint=` envelope codes stay stable for existing consumers across reclassification | integration | `go clean -testcache && go test ./internal/server/... -run TestNoUnregisteredConditionalRejection -v -count=1 && go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/ && task` | 1 | ✅ green |
| 02-03-02 | 03 | 3 | REQ-conditional-rules-stated | see 02-03 `<threat_model>` | `errors.Is(err, errStaleSummary)` keeps working for its four existing call sites | unit | `go clean -testcache && go test ./internal/server/... -count=1 && task lint:markdown` | n/a | ✅ green |
| 02-03-03 | 03 | 3 | REQ-conditional-rules-stated | see 02-03 `<threat_model>` | The documented carve-out is pinned, so a future conditional rejection cannot slip through unmarked | integration | `go clean -testcache && go test ./internal/surfaces/... ./internal/server/... ./cmd/engram/... -count=1 && go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` | n/a | ✅ green |
| 02-04-01 | 04 | 4 | REQ-mcp-tool-annotations | see 02-04 `<threat_model>` | Blast-radius claims are conservative — true only where they hold for every valid invocation | unit | `go clean -testcache && go test ./internal/surfaces/... -run 'TestValidateOperations\|TestOperationsCoverEveryTool' -v -count=1` | 5 | ✅ green |
| 02-04-02 | 04 | 4 | REQ-mcp-tool-annotations | see 02-04 `<threat_model>` | A newly registered tool cannot ship without a declared blast radius | unit (tdd) | `go clean -testcache && go test ./internal/server/... -run 'TestToolAnnotationsBothDirections\|TestRegisterToolsEnumerable' -v -count=1 && go test ./... -count=1 && go run ./internal/surfacesgen && git diff --exit-code -- docs-site/` | 2 | ✅ green |
| 02-05-00 | 05 | 5 | REQ-mcp-tool-annotations | — | N/A — human decision on a one-way published contract | checkpoint:decision (blocking) | none — `<AskUserQuestion>` gate | n/a | N/A |
| 02-05-01 | 05 | 5 | REQ-mcp-tool-annotations | see 02-05 `<threat_model>` | Catalog classification is derived from the shared table, never a second literal | unit | `go clean -testcache && go test ./cmd/engram/... -run 'TestCatalogBlastRadiusMatchesToolClasses\|TestCatalogExitCodesMatchMapper\|TestClientFilesImportBoundary' -v -count=1 && go test ./cmd/engram/... -count=1` | 3 | ✅ green |
| 02-05-02 | 05 | 5 | REQ-help-output-pinned | see 02-05 `<threat_model>` | Goldens cannot self-approve — regeneration must produce a reviewable diff, and the ldflags version is normalized | golden | `go clean -testcache && go test ./cmd/engram -run 'TestHelpGolden\|TestCatalogGolden' -v -count=1 && go test ./cmd/engram -run 'TestHelpGolden\|TestCatalogGolden' -update -count=1 && git diff --exit-code -- cmd/engram/testdata/` | 3 | ✅ green |
| 02-05-03 | 05 | 5 | REQ-help-output-pinned | see 02-05 `<threat_model>` | D-14's "unreviewed" interpretation is recorded where the verifier will read it | docs | `go clean -testcache && task` | n/a | ✅ green |
| 02-06-00 | 06 | 1 | REQ-conditional-rules-stated | — | Token is reissued with least privilege (Workers Scripts: Edit on the owning account only) | checkpoint:human-action (blocking) | none — requires Cloudflare dashboard + repo-secret access | n/a | N/A |
| 02-06-01 | 06 | 1 | REQ-conditional-rules-stated | — | Deploy success is confirmed by the page being live, not merely by a green job | manual | `gh run rerun 30774235923 --failed` then verify `reference/errors.md` resolves | n/a | N/A |
| 02-06-02 | 06 | 1 | REQ-conditional-rules-stated | — | Verify-by-content precedes deletion; nothing unique is discarded | manual | `git diff 7e762662 main -- .planning/milestones/v0.12.x-phases/01-*/` (compare by content, not path) | n/a | N/A |

*Threat refs: each plan carries its own `<threat_model>` block (T-02-01 … T-02-27 across the six plans, ASVS L1, block on `high`). Consult the owning plan for the specific threat a task mitigates.*

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Open items the researcher flagged as needing resolution before dependent tasks:

- [ ] Determine whether a proto comment-only edit dirties `gen/ts/` — decides whether
      `task surfaces:gen` must chain `task proto:gen` (D-07)
- [ ] Confirm the `TestClientFilesImportBoundary` predicate — relevant only if the shared
      blast-radius table (D-10/D-11) is placed somewhere other than a new leaf package
- [ ] `registerTools(s, d)` extraction from `Register()` — prerequisite for enumerating
      real tool registrations without `buildDepsFromEnv`

*Existing `go test` infrastructure otherwise covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Cloudflare `CLOUDFLARE_API_TOKEN` rotation | folded todo | Needs Cloudflare dashboard + repo-secret access; cannot be done from a coding session | Issue a token with Workers Scripts: Edit on the account owning the `engram-docs` worker; update the repo secret; `gh run rerun 30774235923 --failed`; verify `reference/errors.md` is live |
| `docs-site` rule regions render correctly after deploy | REQ-conditional-rules-stated (D-05 surface 5) | Requires the published site; anchored-region syntax must survive the Astro/Starlight markdown pipeline | View the deployed `reference/tools.md` and `guides/cli.md` and confirm the rule sentence renders as prose, not as a visible comment |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 180s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-08-11

---

## Validation Audit 2026-08-11

Every `-run` element across the thirteen Per-Task Verification Map rows that carry one was
re-resolved against `go test -list '.*' ./...` run fresh at HEAD, not trusted from a prior
transcript or from exit status. Each element's resolved count:

| Element | Resolved count |
|---|---|
| `TestValidateRules` | 4 |
| `TestRegisterToolsEnumerable` | 1 |
| `TestNormalizeFieldRoundTrip` | 1 |
| `TestEveryRuleResolvesToNonEmptySurfaceSet` | 1 |
| `TestNoUnregisteredConditionalRejection` | 1 |
| `TestValidateOperations` | 4 |
| `TestOperationsCoverEveryTool` | 1 |
| `TestToolAnnotationsBothDirections` | 1 |
| `TestCatalogBlastRadiusMatchesToolClasses` | 1 |
| `TestCatalogExitCodesMatchMapper` | 1 |
| `TestClientFilesImportBoundary` | 1 |
| `TestHelpGolden` | 1 |
| `TestCatalogGolden` | 2 |

Per D-08 the bar asserted for every element is at least one match, never an exact count, so the
counts above are a cross-check only, not a pinned assertion. No element resolved to zero.

Six rows (02-01-02, 02-02-03, 02-03-02, 02-03-03, 02-05-03) carry no `-run` pattern at all —
they are whole-package runs, codegen-regeneration-plus-`git diff --exit-code` drift checks, or a
lint pass — so their `Tests Matched` cell reads `n/a` rather than an invented number. Three more
rows (02-05-00, 02-06-00, 02-06-01, 02-06-02) are a `checkpoint:decision` and manual/
`checkpoint:human-action` rows with no automated command at all; their `Tests Matched` cell also
reads `n/a`, and their `Status` cell reads `N/A` rather than the automated-test `✅ green` marker,
because no automated test ran for them and marking them green would misrepresent a manual/gate
step as a passing test.

The Task ID cells were already real (`02-01-01` and so on, seeded per-task rather than `TBD`), so
none were backfilled.
