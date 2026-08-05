---
phase: 2
slug: interface-discoverability
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-04
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

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | REQ-conditional-rules-stated | see 02-01 `<threat_model>` | Generated region content is provenance-bounded — a rule sentence reaching a published surface came from the registry, not from arbitrary input | unit (tracer) | `go test ./internal/surfaces/... -run TestValidateRules -v -count=1` | ❌ W0 | ⬜ pending |
| 02-01-02 | 01 | 1 | REQ-conditional-rules-stated | see 02-01 `<threat_model>` | Drift job cannot be satisfied by a hand-edited region — regeneration is the only clean path | integration | `go clean -testcache && go test ./internal/surfaces/... ./internal/server/... ./cmd/engram/... -count=1 && go run ./internal/surfacesgen && git diff --exit-code -- docs-site/` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 2 | REQ-surface-conformance-gate | see 02-02 `<threat_model>` | Tool enumeration does not require live Qdrant/embedder credentials | unit (tdd) | `go clean -testcache && go test ./internal/server/... -run TestRegisterToolsEnumerable -v -count=1 && go test ./internal/server/... -count=1` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 2 | REQ-surface-conformance-gate | see 02-02 `<threat_model>` | A rule resolving to zero applicable surfaces fails loudly instead of passing vacuously | unit (tdd) | `go clean -testcache && go test ./internal/surfaces/... -run 'TestNormalizeFieldRoundTrip\|TestEveryRuleResolvesToNonEmptySurfaceSet' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-02-03 | 02 | 2 | REQ-surface-conformance-gate | see 02-02 `<threat_model>` | Divergence between any two bound surfaces fails CI | integration | `go clean -testcache && go test ./internal/surfaces/... ./internal/server/... ./cmd/engram/... -count=1` | ❌ W0 | ⬜ pending |
| 02-03-01 | 03 | 3 | REQ-conditional-rules-stated | see 02-03 `<threat_model>` | Published `hint=` envelope codes stay stable for existing consumers across reclassification | integration | `go clean -testcache && go test ./internal/server/... -run TestNoUnregisteredConditionalRejection -v -count=1 && go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/ && task` | ❌ W0 | ⬜ pending |
| 02-03-02 | 03 | 3 | REQ-conditional-rules-stated | see 02-03 `<threat_model>` | `errors.Is(err, errStaleSummary)` keeps working for its four existing call sites | unit | `go clean -testcache && go test ./internal/server/... -count=1 && task lint:markdown` | ✅ | ⬜ pending |
| 02-03-03 | 03 | 3 | REQ-conditional-rules-stated | see 02-03 `<threat_model>` | The documented carve-out is pinned, so a future conditional rejection cannot slip through unmarked | integration | `go clean -testcache && go test ./internal/surfaces/... ./internal/server/... ./cmd/engram/... -count=1 && go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` | ❌ W0 | ⬜ pending |
| 02-04-01 | 04 | 4 | REQ-mcp-tool-annotations | see 02-04 `<threat_model>` | Blast-radius claims are conservative — true only where they hold for every valid invocation | unit | `go clean -testcache && go test ./internal/surfaces/... -run 'TestValidateOperations\|TestOperationsCoverEveryTool' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-04-02 | 04 | 4 | REQ-mcp-tool-annotations | see 02-04 `<threat_model>` | A newly registered tool cannot ship without a declared blast radius | unit (tdd) | `go clean -testcache && go test ./internal/server/... -run 'TestToolAnnotationsBothDirections\|TestRegisterToolsEnumerable' -v -count=1 && go test ./... -count=1 && go run ./internal/surfacesgen && git diff --exit-code -- docs-site/` | ❌ W0 | ⬜ pending |
| 02-05-00 | 05 | 5 | REQ-mcp-tool-annotations | — | N/A — human decision on a one-way published contract | checkpoint:decision (blocking) | none — `<AskUserQuestion>` gate | N/A | ⬜ pending |
| 02-05-01 | 05 | 5 | REQ-mcp-tool-annotations | see 02-05 `<threat_model>` | Catalog classification is derived from the shared table, never a second literal | unit | `go clean -testcache && go test ./cmd/engram/... -run 'TestCatalogBlastRadiusMatchesToolClasses\|TestCatalogExitCodesMatchMapper\|TestClientFilesImportBoundary' -v -count=1 && go test ./cmd/engram/... -count=1` | ❌ W0 | ⬜ pending |
| 02-05-02 | 05 | 5 | REQ-help-output-pinned | see 02-05 `<threat_model>` | Goldens cannot self-approve — regeneration must produce a reviewable diff, and the ldflags version is normalized | golden | `go clean -testcache && go test ./cmd/engram -run 'TestHelpGolden\|TestCatalogGolden' -v -count=1 && go test ./cmd/engram -run 'TestHelpGolden\|TestCatalogGolden' -update -count=1 && git diff --exit-code -- cmd/engram/testdata/` | ❌ W0 | ⬜ pending |
| 02-05-03 | 05 | 5 | REQ-help-output-pinned | see 02-05 `<threat_model>` | D-14's "unreviewed" interpretation is recorded where the verifier will read it | docs | `go clean -testcache && task` | ❌ W0 | ⬜ pending |
| 02-06-00 | 06 | 1 | REQ-conditional-rules-stated | — | Token is reissued with least privilege (Workers Scripts: Edit on the owning account only) | checkpoint:human-action (blocking) | none — requires Cloudflare dashboard + repo-secret access | N/A | ⬜ pending |
| 02-06-01 | 06 | 1 | REQ-conditional-rules-stated | — | Deploy success is confirmed by the page being live, not merely by a green job | manual | `gh run rerun 30774235923 --failed` then verify `reference/errors.md` resolves | N/A | ⬜ pending |
| 02-06-02 | 06 | 1 | REQ-conditional-rules-stated | — | Verify-by-content precedes deletion; nothing unique is discarded | manual | `git diff 7e762662 main -- .planning/milestones/v0.12.x-phases/01-*/` (compare by content, not path) | N/A | ⬜ pending |

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

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
