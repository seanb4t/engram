---
phase: 01-eval-foundation-lexical-reranker-fix
reviewed: 2026-09-23T11:55:20Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - cmd/engram/reindex.go
  - docs-site/src/content/docs/reference/tools.md
  - internal/retrievaleval/comparison_rankers.go
  - internal/retrievaleval/comparison_rankers_test.go
  - internal/retrievaleval/doc.go
  - internal/retrievaleval/fixtures.go
  - internal/retrievaleval/gate.go
  - internal/retrievaleval/gate_test.go
  - internal/retrievaleval/paraphrase_fixture.go
  - internal/retrievaleval/paraphrase_fixture_test.go
  - internal/retrievaleval/rankers.go
  - internal/retrievaleval/rankers_test.go
  - internal/retrievaleval/retrieval_eval_test.go
  - internal/retrievaleval/vector.go
  - internal/retrievaleval/vector_test.go
  - internal/server/connectapi_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/rerank.go
  - internal/store/rerank_test.go
  - internal/store/store.go
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-09-23T11:55:20Z
**Depth:** standard
**Files Reviewed:** 20 (`internal/server/tools_test.go` and `internal/store/store.go` reviewed against their diffs from `7abebe25`, per the reduced-scope instruction for large pre-existing files)
**Status:** issues_found

## Summary

This phase renames `store.candidateK`/`RerankHits`-as-shipped-step to an exported `CandidateK` + `rankCandidates` seam, adds a retrieval-quality eval package (`internal/retrievaleval`) with a D-05 mechanically-applied ranking decision, a blind multi-domain paraphrase corpus, a GH#261 regression fixture, a pluggable named-ranker roster, and a koanf-based env gate, plus a small plumbing change threading `*config.Config` out of `StoreAndEmbedderFromEnvNoEnsure` for the eval's symmetric-embed-config skip.

The code is well-documented, its decision logic (`decideRanking`) is exhaustively unit-tested against edge cases (ties, margin boundaries, ineligible baseline, missing baseline, disabled/shipped rows), and cross-checked call sites (`reindex.go`, `tools_test.go`, `retrieval_eval_test.go`) all correctly adopt the new 6-return-value signature of `StoreAndEmbedderFromEnvNoEnsure`. `go build ./...`, `go vet` (on the reviewed packages), and `go test ./internal/retrievaleval/... ./internal/store/...` all pass. No security issues, no dead code, no unused imports were found in the reviewed files.

One formatting defect (`gofmt`-non-compliant struct literal) was found that would fail `task fmt`/`task lint`, and one minor doc-wording note.

## Warnings

### WR-01: `rankers_test.go` fails `gofmt`

**File:** `internal/retrievaleval/rankers_test.go:177-181`
**Issue:** The `cases` struct literal in `TestDecideRanking` is not gofmt-formatted — the `wantReasonSubstr` field and its type are misaligned relative to the other three fields:
```go
cases := []struct {
    name              string
    rows              []variantSummary
    wantWinner        string
    wantReasonSubstr  string   // extra space before `string`
}{
```
Confirmed with `gofmt -l internal/retrievaleval/rankers_test.go` (flagged) and `gofmt -d` (shows the diff). Per CLAUDE.md, `task fmt`/`task lint` must be clean; this file currently is not, and would fail CI's `lint` job (gofmt is part of `golangci-lint`'s formatter checks) or at minimum `task fmt` drift-checks.
**Fix:**
```bash
gofmt -w internal/retrievaleval/rankers_test.go
```

## Info

### IN-01: Doc wording for the reranker mechanism is slightly at odds with the code comments' "shipped rank step" framing

**File:** `docs-site/src/content/docs/reference/tools.md:143,153-154`
**Issue:** The docs now describe ranking as "vector similarity and a lexical-overlap adjustment selected by the retrieval eval." This is accurate today (D-05 selected lexical), but the phrasing bakes in the *current* winner's mechanism ("lexical-overlap") into public docs, while the Go-side comments (`store.go`, `rerank.go`) are careful to describe the seam generically ("the D-05-selected rank step") specifically so a future eval re-run can change the winner without stale prose. If a future eval re-run picks a different (e.g. cosine-blend) variant, this docs line would need a matching edit that source comments already anticipate needing (`rerank.go`'s own doc comment: "If a future live eval re-run selects a different winner, this function's body changes to match"). Not a bug — the doc is correct as of this commit — but the phrasing couples the public contract description to an implementation detail that is documented elsewhere as intentionally revisitable.
**Fix:** Consider "a reranking step selected by the retrieval eval" (matching the source's "rank step" language) rather than naming the specific "lexical-overlap" mechanism, so a future D-05 re-run doesn't leave the docs describing the wrong algorithm until someone remembers to update this file too.

---

_Reviewed: 2026-09-23T11:55:20Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
