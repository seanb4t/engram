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
| *(populated by the planner)* | | | | | | | | | ⬜ pending |

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
