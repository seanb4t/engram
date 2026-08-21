---
phase: 8
slug: registry-docs-tail
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-21
updated: 2026-08-21
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

## What the automated lanes actually cover

Measured against the linter and CI configs on 2026-08-21. Recorded here because three gates the
first draft of this phase's plans cited do not read the files they were credited with.

| Lane | Covers | Does NOT cover |
|------|--------|----------------|
| `task lint` → `rumdl check .` | `CLAUDE.md`, repo-root markdown | **`docs-site/**` — excluded in `.rumdl.toml:28`.** Also `.planning/**`, `CHANGELOG.md` |
| `task lint` | golangci-lint, yamlfmt, actionlint, ruff | **dprint** — it lives under `fmt:*`, never under `lint` (`Taskfile.yaml:78`) |
| `dprint` (via `task fmt`) | `**/*.json`, `**/*.toml` (`dprint.json:6`) | all markdown, everywhere |
| `cd docs-site && pnpm build` | frontmatter validity, MDX/route compilation | **internal links** — no `starlight-links-validator`, no `experimental.validateLinks`; rumdl's MD057 is disabled (`.rumdl.toml:14`) |
| `task license:check` | `CLAUDE.md`, Go sources | **`docs-site/**` — `paths-ignore` in `.licenserc.yaml:44`** |

Consequences carried into the plans:

- **`docs-site` prose is manual-review-only.** No linter reads `guides/migrate.md`,
  `guides/upgrade.md`, `reference/memory-record.md`, or `reference/tools.md`. Plans 08-02 and 08-03
  say so in-plan rather than citing `task lint` as prose verification.
- **Internal links are checked by a purpose-built loop**, not by the Astro build. The loop resolves
  each `](/path/)` to `docs-site/src/content/docs/<path>.{md,mdx}` (or `<path>/index.md`) and reports
  what is missing. Verified 2026-08-21: it reports nothing on the current tree and reports
  `BROKEN /guides/migrate` for the link 08-02 and 08-03 are about to add.
- **`docs-site/node_modules` is ABSENT on a fresh checkout**, so `pnpm build` cannot run without
  `pnpm install --frozen-lockfile`. That is a stated `<precondition>` on the three docs tasks, not an
  assumption.

---

## Sampling Rate

- **After every task commit:** Run `go test -short ./internal/surfaces/... ./cmd/engram/...`, plus
  the task's own acceptance gates.
- **After every plan wave:** Run `task test`, plus a local reproduction of CI's `surfaces` job —
  which is exactly `task surfaces:gen && git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/`
  run against a committed tree. `task surfaces:gen` IS that job's sequence (generator → `proto:gen`
  → vendored TS re-copy → golden `-update`); do not hand-assemble a subset of it.
- **Before `/gsd-verify-work`:** Full suite green, `task lint` green, `go test ./internal/keylinks/...`
  green, and `docs-site` `pnpm build` green (or explicitly reported BLOCKED with its command and error).
- **Max feedback latency:** 45 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-T1 | 08-01 | 1 | REQ-sweep-scope-rule-registered | T-08-01 | Rejects with `exitUsage` when neither `--scope` nor `--all-scopes` is supplied | unit | `go test ./cmd/engram -run 'TestSummarizeMissingRequiresScopeOrAllScopes' -v` | ❌ W0 — authored by this task | ⬜ pending |
| 08-01-T2 | 08-01 | 1 | REQ-sweep-scope-rule-registered | — | N/A | unit | `go test ./internal/surfaces -run TestValidateRules -v` | ✅ `internal/surfaces/rules_test.go` | ⬜ pending |
| 08-01-T2 | 08-01 | 1 | REQ-sweep-scope-rule-registered | T-08-04 | N/A | integration | `go test ./internal/surfaces -run TestSurfaceConformanceProseFiles -v` | ✅ `internal/surfaces/conformance_test.go` | ⬜ pending |
| 08-01-T2 | 08-01 | 1 | REQ-sweep-scope-rule-registered | T-08-02 | The rule resolves to `summarize-missing` ALONE on the cobra lane — never to `consolidate` or `purge`, neither of which enforces it | integration | `go test ./cmd/engram -run TestSurfaceConformanceCobraUsage -v` | ✅ `cmd/engram/surfaces_test.go` | ⬜ pending |
| 08-01-T2 | 08-01 | 1 | REQ-sweep-scope-rule-registered | — | N/A | golden | `go test ./cmd/engram -run 'TestHelpGolden\|TestCatalogGolden' -v` (goldens MOVE — the Usage composition changes them; regenerate via `task surfaces:gen`) | ✅ `cmd/engram/testdata/{help,catalog}.golden` | ⬜ pending |
| 08-01-T3 | 08-01 | 1 | REQ-sweep-scope-rule-registered | T-08-01 | Present-but-empty `--scope` is the same behavior class as absent | unit | `go test ./cmd/engram -run 'TestRequireSweepScope\|TestSweepLeaves' -v` | ❌ W0 — authored by this task | ⬜ pending |
| 08-01-T3 | 08-01 | 1 | REQ-sweep-scope-rule-registered | T-08-02 | The sentence is present on the three enforcing leaves' `--all-scopes` Usage and ABSENT on `consolidate`/`purge` | unit | `go test ./cmd/engram -run TestSweepLeavesUsageStatesRegisteredRule -v` | ❌ W0 — authored by this task | ⬜ pending |
| 08-01-T3 | 08-01 | 1 | REQ-sweep-scope-rule-registered | — | N/A | acceptance (shell) | `rg -o -F -- '--scope <scope> or --all-scopes is required' cmd/engram/ \| wc -l` → **0** (reports **3** today) | ❌ W0 — gate is authored by this phase | ⬜ pending |
| 08-03-T1 | 08-03 | 2 | REQ-docs-record-state | T-08-07, T-08-08, T-08-09 | N/A | acceptance (shell) | derived json-key set difference, backtick-anchored: `comm -23 <(sed -n '/type migrateOutputDoc struct/,/^}/p;/type migrateStatusReportDoc struct/,/^}/p;/type revertOutputDoc struct/,/^}/p' cmd/engram/migrate_family.go \| rg -o 'json:"[a-z_]+' \| sed 's/json:"//' \| sort -u) <(rg -o '\`[a-z_]+\`' docs-site/src/content/docs/guides/migrate.md \| tr -d '\`' \| sort -u)` → empty; left side must have **21** members | ❌ W0 — page created by this task | ⬜ pending |
| 08-03-T1 | 08-03 | 2 | REQ-docs-record-state | — | N/A | acceptance (shell) | internal-link existence loop over `guides/migrate.md` → no output | ❌ W0 | ⬜ pending |
| 08-03-T1 | 08-03 | 2 | REQ-docs-record-state | — | N/A | build | `cd docs-site && pnpm build` (**requires** `pnpm install --frozen-lockfile`; `node_modules` is absent) | ❌ W0 — `guides/migrate.md` does not exist yet | ⬜ pending |
| 08-03-T2 | 08-03 | 2 | REQ-docs-record-state | T-08-11 | N/A | acceptance (shell) | `tr '\n' ' ' < docs-site/src/content/docs/guides/upgrade.md \| rg -o 'That sweep does not exist in this release' \| wc -l` → **0** (reports **1** today). The `tr` is load-bearing: the phrase is line-wrapped mid-sentence, so a plain single-line `rg` reports 0 in BOTH directions | ✅ `guides/upgrade.md` | ⬜ pending |
| 08-02-T1 | 08-02 | 3 | REQ-docs-record-state | T-08-05 | N/A | acceptance (shell) | derived field-table set difference: `comm -23 <(sed -n '/^type Memory struct/,/^}/p' internal/store/store.go \| rg -o 'json:"[a-z_]+' \| sed 's/json:"//' \| sort -u) <(rg -o '^\| [^\|]* \| \`[a-z_]+\`' docs-site/src/content/docs/reference/memory-record.md \| rg -o '\`[a-z_]+\`' \| tr -d '\`' \| sort -u)` → empty. Reports 7 keys today | ✅ `reference/memory-record.md` | ⬜ pending |
| 08-02-T2 | 08-02 | 3 | REQ-docs-record-state | T-08-04 | N/A | acceptance (shell) | state-word order extraction over `## get_memory` → `archived superseded expired scheduled` (reports the exact reverse today); and `rg -o -F '\`not_after\` in the past' … \| wc -l` → **0** (reports 1 today) | ✅ `reference/tools.md` | ⬜ pending |
| 08-02-T2 | 08-02 | 3 | REQ-docs-record-state | T-08-12 | Generated anchor bodies are unmodified | integration | `go test ./internal/surfaces -run TestSurfaceConformanceProseFiles -v` | ✅ | ⬜ pending |
| 08-04-T1 | 08-04 | 4 | REQ-claude-md-migrations-convention | T-08-14, T-08-15 | N/A | acceptance (shell) | ROW-scoped derived inventory gate (see 08-04 Task 1 `<verify>`) → `$missing` empty. Reports **26** name/word misses today | ✅ `cmd/engram/testdata/catalog.golden` (regenerated by 08-01) | ⬜ pending |
| 08-04-T2 | 08-04 | 4 | REQ-claude-md-migrations-convention | T-08-17 | N/A | acceptance (shell) | section-scoped presence + no-list/no-table invariants (see 08-04 Task 2 `<verify>`) | ✅ `CLAUDE.md` | ⬜ pending |
| — | *(all)* | — | generated-artifact drift | — | N/A | CI parity | `task surfaces:gen && git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/` on a COMMITTED tree (mirrors `.github/workflows/ci.yaml:291-298`) | ✅ CI job `surfaces` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] A rejection test for `summarize-missing` mirroring `TestSpineReviewScanRequiresScopeOrAllScopes`.
      Confirmed missing from `cmd/engram/summarize_test.go` — it is the **only** sweep leaf with zero
      coverage pinning its "neither flag supplied" rejection. `spine-review verify` is already covered
      by `cmd/engram/spine_review_verify_test.go:526`. Authored by 08-01 Task 1, deliberately against
      unmodified `main` so it is a characterization pin rather than a test written to new behavior.
- [ ] `cmd/engram/sweep_scope_test.go` — the three-leaf agreement test, the present-but-empty case,
      the direct helper table test, and `TestSweepLeavesUsageStatesRegisteredRule` (the explicit
      leaf whitelist plus its negative half for `consolidate`/`purge`). Authored by 08-01 Task 3.
- [ ] `docs-site/src/content/docs/guides/migrate.md` — does not exist yet; created by 08-03. The
      Guides sidebar autogenerates from the directory (`docs-site/astro.config.mjs:28-35`), so only
      `title`/`description` frontmatter is required — no config edit.
- [ ] `docs-site/node_modules` — absent on a fresh checkout. `pnpm install --frozen-lockfile` is a
      precondition of every task whose acceptance includes `pnpm build`.
- [ ] The shell acceptance gates themselves. They are not `go test`s, so each lives in its plan's own
      `<verify>`/`<acceptance_criteria>`. Per CONTEXT.md D-01 the sweep gate asserts **zero remaining
      occurrences**, never "three converted" — a count gate passes if a fourth sweep leaf lands
      between planning and execution.

---

## Manual-Only Verifications

Prose semantics on `docs-site/**` are manual-only **by construction**, not by omission: per the lane
table above, no linter reads that tree and the build validates no links. These rows are the gate.

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Reference pages state the record-state vocabulary correctly | REQ-docs-record-state | The risk is *plausible but wrong* text. Nothing lints `docs-site` — `.rumdl.toml:28` excludes it and dprint is markdown-blind — so no automated gate can catch a fluent inverted sentence | Read `reference/memory-record.md` and `reference/tools.md` against `internal/store/store.go`'s `activeWindowConditions` (function at `:1006`; the **`Lte` bound is at `:1012`** and the **`Gt` bound at `:1016`**). Confirm `not_before == now` reads as ACTIVE and `not_after == now` reads as EXPIRED, that `expired` suppresses `scheduled`, and that both pages present the half-open window as the same convention `createdRangeCondition` (`:1289-1301`, `Gte`/`Lt`) already uses |
| The forward-version / rollback claim is stated with both narrowings | REQ-docs-record-state | An unqualified "a newer record is always safe" reads as "rollback is safe" and costs data | Read the schema-version section against `internal/store/store.go:335-352`. Confirm it separates read/recall safety from the D-06 older-binary-rewrite loss AND names the `Store.Upsert` narrowing, and that `guides/upgrade.md`'s rollback-hazard paragraph says the same thing |
| `guides/migrate.md` documents the mechanism end to end | REQ-docs-record-state | Completeness against a live CLI is a judgement, not an assertion | Walk the guide against `engram migrate --help`, `engram migrate status --help`, `engram migrate revert --help`, `engram migration-status --help`; confirm every flag, the preview-vs-apply default, and the exit-code taxonomy appear and match |
| `guides/migrate.md` states no guarantee the code does not make | REQ-docs-record-state | The three claims at issue are prose-shaped; no grep separates a correct one from a fluent wrong one | Check the convergence sentence against the PA-3 guard (`internal/store/migrate.go:498-528`), the re-run sentence against the no-persisted-cursor comment (`:125-150`), and the revert refusal section against BOTH `internal/store/revert.go:367-382` (preflight, zero-write) and `:455-500` (race-discovered, after partial writes). The words "strictly shrinking" and "resumable" must not appear |
| CLAUDE.md's three D-05 sites agree with the live tree | REQ-claude-md-migrations-convention | The defect being fixed *is* doc-vs-code disagreement; only a human comparison closes it | Diff CLAUDE.md's `cmd/engram/` row against `cmd/engram/testdata/catalog.golden`'s top-level names; confirm the tier split (client-tier `get`/`search`/`list`/`store`/`migration-status` vs operator-tier sweeps) matches which commands take a server URL and token; confirm the migrations bullet's boundary matches `.planning/REQUIREMENTS.md`'s Out of Scope row and its automation half matches `internal/server/tools.go:490-535` |
| Two staleness findings surfaced by research are closed | REQ-docs-record-state | Found outside the phase's original scope; needs an explicit check so they are not silently dropped | Confirm `reference/memory-record.md`'s `### Archiving` section no longer claims the Connect `Memory` message lacks `superseded_by`/`supersedes`/`not_before`/`not_after` (fields 23-28 shipped) and carries **no tracker citation at all**, and that `guides/upgrade.md`'s section 12 no longer asserts the forward sweep is absent |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 45s
- [ ] Every acceptance gate was observed RED before the work (each plan names its before-value)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
