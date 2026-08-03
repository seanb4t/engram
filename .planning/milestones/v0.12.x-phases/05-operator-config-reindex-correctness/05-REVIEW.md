<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

---
phase: 05-operator-config-reindex-correctness
reviewed: 2026-08-01T22:50:16Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - internal/store/store.go
  - internal/server/tools.go
  - internal/config/config.go
  - internal/config/registry.go
  - cmd/engram/reindex.go
  - internal/store/reindex_test.go
  - internal/server/embed_wiring_test.go
  - cmd/engram/reindex_test.go
  - charts/engram/values.yaml
  - charts/engram/templates/_helpers.tpl
  - Taskfile.yaml
  - docs-site/src/content/docs/guides/configure.md
  - docs-site/src/content/docs/guides/reindex.md
  - docs-site/src/content/docs/guides/upgrade.md
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 5: Code Review Report

**Reviewed:** 2026-08-01T22:50:16Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** clean

## Summary

Reviewed all Go, Helm, and docs changes for v0.12.x Phase 5 (`git diff dc98ec0c..HEAD`)
against the six adversarial targets in scope: the three-conjunct resume skip predicate,
tag-comparison order-independence/aliasing, `--dry-run --resume` write-boundary safety,
`ChatAPIKey` credential handling, test-assertion strength, and `reindexSummary` wording
stability.

**The nil-vs-empty tag risk (item 1) does not materialize.** `tagsFromPayload` is careful
to leave `var tags []string` unassigned (nil) unless at least one list item is appended,
so both an absent `"tags"` key (raw points) and a present-but-empty `"tags": []` list
(the normal `payload()` write path for an untagged `Memory`, which always writes an
`[]any{}` rather than omitting the key) decode to the same nil value on both the source
and target sides. `tagsEqual`'s `len(a) != len(b)` fast path then treats nil and `[]string{}`
as equal by construction (D-10), and `TestTagsEqual`'s `"nil-versus-empty-equal"` case
plus `TestReindexResumeTags`'s EDGE 4 subtest (traced by elimination against real Qdrant
across four consecutive resume runs) both pin this directly — not just as an isolated
unit case but end-to-end through the actual resume path.

**No aliasing bug in the tag comparison.** `tagsEqual` calls `slices.Clone(a)` and
`slices.Clone(b)` before `sort.Strings`, so the sort never mutates a slice that aliases
the caller's `Memory.Tags` or the target snapshot's `reindexTarget.tags`.

**`--dry-run --resume` cannot reach a write.** `ensureCollection` is gated by
`if !opts.DryRun` before the scroll loop; inside the loop, `opts.DryRun` short-circuits
to `res.WouldUpsert++; continue` before the `embed(...)` call and before the `Upsert`
call. The only Qdrant call reachable under `DryRun` besides the source scroll is the
read-only `CollectionExists` check and `reindexTargetContents`'s `Get` (also read-only).
`TestReindexDryRunResume` confirms both the existing-target and nonexistent-target cases
write nothing and create nothing.

**`ChatAPIKey` never reaches a log line or error string.** `summarize.go`'s only use of
`apiKey` is `req.Header.Set("Authorization", "Bearer "+c.apiKey)`; no `fmt.Errorf`,
`slog`, or `Printf` call in the diff or in `internal/summarize` interpolates it. The
embedder lane (`embedderFromConfig` → `embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, ...)`)
is byte-identical to its pre-phase form.

**Tests assert real outcomes, not just "no error."** `TestReindexResumeTags`'s EDGE 2
subtest is a standalone run (not a subtraction from EDGE 1's totals) asserting
`Unchanged == 2` after nothing was mutated — a genuine positive control, and the
plan's SUMMARY documents a RED reading where forcing `tagsEqual` to always return
`false` fails exactly that subtest (proving it's load-bearing, not vacuous).
`TestSummarizerFromConfigChatAPIKey` asserts on the literal `Authorization` header
value received by two separate `httptest` servers, including the case where only the
chat key (not the chat base URL) is set, proving the two knobs are independent.

**`reindexSummary`'s non-resume dry-run line is unchanged.** The `dryRun && !resume`
branch still returns `fmt.Sprintf("dry-run: %d record(s) would be re-embedded into %q at dim %d", res.Scanned, target, dim)` — same format string, same single input
(`res.Scanned`). Note for completeness: `ReindexResult.Skipped` now accrues for
empty-content records even under a non-resume dry run (previously dry run only ever
incremented `Scanned`), but since the non-resume dry-run summary line only reads
`res.Scanned`, the printed text is unaffected; this is an internal counting refinement
documented in the updated `ReindexResult` doc comment, not a behavior regression, and
has no other caller (`Reindex` has exactly one call site, `cmd/engram/reindex.go`).

The Helm chart change mirrors the pre-existing `memory.openai.apiKeySecret` pattern
exactly (same guard-on-`.name` shape, same lack of `.key` validation) — not a new
defect, just consistency with established chart convention. The `EXPECTED_CHECKSUM`
re-pin in `Taskfile.yaml` was independently recomputed against the current
`_helpers.tpl` and matches. `go build ./...` and `go vet` are clean on the reviewed
packages. The three docs files were checked against the actual code/format strings
line-by-line (registry entries, `reindexSummary` output strings, the resume predicate's
three conjuncts, the D-15 limit) and found accurate.

All reviewed files meet quality standards. No issues found.

---

_Reviewed: 2026-08-01T22:50:16Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
