---
phase: 8
slug: registry-docs-tail
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-21
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) |
| **Config file** | none — `go test` driven via `Taskfile.yaml` |
| **Quick run command** | `go test -short ./internal/surfaces/... ./cmd/engram/...` |
| **Full suite command** | `task test` (`go test ./...` + `uv run --with pytest pytest skill/engram/hooks/tests -q`) |
| **Estimated runtime** | ~45 seconds quick · ~4 minutes full |

---

## Sampling Rate

- **After every task commit:** Run `go test -short ./internal/surfaces/... ./cmd/engram/...`, plus
  the zero-occurrence acceptance gate below.
- **After every plan wave:** Run `task test`, plus a local reproduction of CI's `surfaces` job
  (generator → `buf generate` → `git diff --exit-code`).
- **Before `/gsd-verify-work`:** Full suite green, `task lint` green
  (`golangci-lint`/`rumdl`/`dprint`/`actionlint`), `go test ./internal/keylinks/...` green, and
  `docs-site` `pnpm build` green.
- **Max feedback latency:** 45 seconds

---

## Per-Task Verification Map

Task IDs are assigned when PLAN.md files are written; rows below are seeded at the requirement level
and are re-keyed to concrete task IDs by `/gsd-validate-phase`.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | REQ-sweep-scope-rule-registered | — | N/A | unit | `go test ./internal/surfaces -run TestValidateRules -v` | ✅ `internal/surfaces/rules_test.go` | ⬜ pending |
| TBD | TBD | TBD | REQ-sweep-scope-rule-registered | — | N/A | integration | `go test ./internal/surfaces -run TestSurfaceConformanceProseFiles -v` | ✅ `internal/surfaces/conformance_test.go` | ⬜ pending |
| TBD | TBD | TBD | REQ-sweep-scope-rule-registered | — | Rejects with `exitUsage` when neither `--scope` nor `--all-scopes` is supplied | unit | `go test ./cmd/engram -run 'RequiresScopeOrAllScopes' -v` | ✅ scan/verify · ❌ W0 summarize-missing | ⬜ pending |
| TBD | TBD | TBD | REQ-sweep-scope-rule-registered | — | N/A | acceptance (shell) | `rg -o -- '--scope <scope> or --all-scopes is required' cmd/engram/ \| wc -l` → **0** | ❌ W0 — gate is authored by this phase | ⬜ pending |
| TBD | TBD | TBD | REQ-sweep-scope-rule-registered | — | N/A | golden | `go test ./cmd/engram -run 'TestHelpGolden\|TestCatalogGolden' -v` | ✅ `cmd/engram/testdata/{help,catalog}.golden` | ⬜ pending |
| TBD | TBD | TBD | REQ-docs-record-state | — | N/A | manual (docs review) | N/A — prose correctness beyond `rumdl` is not machine-checkable | N/A | ⬜ pending |
| TBD | TBD | TBD | REQ-docs-record-state | — | N/A | build | `cd docs-site && pnpm build` | ❌ W0 — `guides/migrate.md` does not exist yet | ⬜ pending |
| TBD | TBD | TBD | REQ-claude-md-migrations-convention | — | N/A | manual (docs review) | N/A | ⬜ pending |
| TBD | TBD | TBD | *(all)* generated-artifact drift | — | N/A | CI parity | `go run ./internal/surfacesgen && go tool buf generate && git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/` (mirrors `.github/workflows/ci.yaml:291-298`) | ✅ CI job `surfaces` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] A rejection test for `summarize-missing` mirroring `TestSpineReviewScanRequiresScopeOrAllScopes`
      / `TestSpineReviewVerifyRequiresScopeOrAllScopes`. Confirmed missing from
      `cmd/engram/summarize_test.go` — the third conversion site currently has **zero** coverage
      pinning its "neither flag supplied" rejection, so converting it is unguarded.
- [ ] `docs-site/src/content/docs/guides/migrate.md` — does not exist yet; created by this phase.
      The Guides sidebar autogenerates from the directory (`docs-site/astro.config`), so only
      `title`/`description` frontmatter is required — no config edit.
- [ ] The zero-occurrence acceptance-gate assertion itself. It is a shell gate, not a `go test`, so
      it must be written into a plan's own `<verification>` block. Per CONTEXT.md D-01 it asserts
      **zero remaining occurrences**, never "three converted" — a count gate passes if a fourth
      sweep leaf lands between planning and execution.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Reference pages state the record-state vocabulary correctly | REQ-docs-record-state | Prose semantics are not machine-checkable; the risk is *plausible but wrong* text, which no linter catches | Read `reference/memory-record.md` and `reference/tools.md` against `internal/store/store.go` `activeWindowConditions` (~1013/1017). Confirm `not_before == now` reads as ACTIVE and `not_after == now` reads as EXPIRED, and that `expired` suppresses `scheduled`. |
| `guides/migrate.md` documents the mechanism end to end | REQ-docs-record-state | Completeness against a live CLI is a judgement, not an assertion | Walk the guide against `engram migrate --help` and `engram migration-status --help`; confirm every flag, the preview-vs-apply default, and the exit-code taxonomy appear and match. |
| CLAUDE.md's three D-05 sites agree with the live tree | REQ-claude-md-migrations-convention | The defect being fixed *is* doc-vs-code disagreement; only a human comparison closes it | Diff CLAUDE.md's `cmd/engram/` inventory against `rg -n 'Use:\s*"' cmd/engram/*.go`; confirm the "Not used here" line and the Memory contract vocabulary match what shipped. |
| Two staleness findings surfaced by research are closed | REQ-docs-record-state | Found outside the phase's original scope; needs an explicit check so they are not silently dropped | Confirm `reference/memory-record.md:75-79` no longer claims the Connect `Memory` message lacks `superseded_by`/`supersedes`/`not_before`/`not_after` (Phase 5 shipped fields 23-28), and `guides/upgrade.md:314-340` no longer says "there is no `engram migrate` command to run yet". |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 45s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
