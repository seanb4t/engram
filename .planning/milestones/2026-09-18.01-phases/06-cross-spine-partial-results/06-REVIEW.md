---
phase: 06-cross-spine-partial-results
reviewed: 2026-09-20T00:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - internal/server/tools.go
  - internal/server/connectapi.go
  - cmd/engram/client_common.go
  - cmd/engram/client_list.go
  - cmd/engram/client_search.go
  - proto/engram/v1/engram.proto
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 6: Code Review Report

**Reviewed:** 2026-09-20T00:00:00Z
**Depth:** standard
**Files Reviewed:** 6
**Status:** clean

## Summary

Reviewed the full diff (`5b75fa01..HEAD`) implementing REQ-cross-spine-partial (#456): four call sites (MCP `search_memory`/`list_memory` closures in `tools.go`, Connect `ListMemories`/`SearchMemories` in `connectapi.go`) that previously discarded already-authorized hits when the follow-up `ListScopes` coverage query failed now return the hits, with a new `scopes_unknown` wire signal distinguishing "coverage unknown" from "coverage truncated."

Traced the three-state contract end to end on both transports:

- **`(*deps).searchedScopes`** (`tools.go:1845`) now returns a `scopeCoverage{Scopes, Truncated, Unknown}` value with no error. On `ListScopes` failure it logs once via `slog.ErrorContext` and returns `scopeCoverage{Unknown: true}` — `Scopes` stays `nil` (the zero value, never an allocated empty slice) and `Truncated` stays `false` (its zero value), so `Unknown` and `Truncated` can never both be `true` by construction — the two fields are set on mutually exclusive return paths.
- **`recallResultMap`** (`tools.go:1889`) checks `cov.Unknown` first and returns immediately with only `"scopes_unknown": true` set, never touching `"searched_scopes"`/`"scopes_truncated"` on that path — matching the MCP-map three-state contract (absent / populated+truncated-flag / unknown-only).
- **Connect handlers** (`connectapi.go:308-321`, `371-377`) assign `SearchedScopes: cov.Scopes` directly; a `nil` slice serializes as proto3-absent, so the unknown path never emits a legitimate-looking empty list on the wire. Both handlers compute `cov` *after* the hits (`res`/`ms`) are already resolved and never gate the response on `cov`, so hits are never discarded on this path — confirmed the `err` returned by `searchedScopes` no longer exists at all (signature changed from three return values including `error` to a single value), which is what the phase intent describes as making the discard structurally unrepresentable.
- **`renderCoverageFooter`** (`client_common.go:341`) checks `scopesUnknown` before `scopesTruncated`/the count branch and returns immediately (`return err` inside the `if scopesUnknown` block), so it can never fall through to print a false zero-count line — matches the required branch order exactly.
- **Proto** (`engram.proto`): `scopes_unknown` added as field 7 on `ListMemoriesResponse` and field 4 on `SearchMemoriesResponse` — both additive, no renumbering of existing fields (`searched_scopes`/`scopes_truncated` keep their original numbers on both messages).

Checked for the forbidden fix (swallowing the `ListScopes` error into a zero value): the error is still real, wrapped in `slog.ErrorContext` with the underlying `err` attached, and is never coerced into a false "coverage known, zero scopes" result — `Unknown: true` is a distinct branch from the success path, not a default fallback.

Checked for backend-text leakage: only `err.Error()` (the caller-injected error text) reaches the log line via `slog.ErrorContext(ctx, "...", "error", err)`; nothing from `err` is written into the `scopeCoverage` struct or any proto/map field returned to the caller. Verified against `TestCrossSpineCoverageUnknownConnectSearch`/`...List` which explicitly assert the injected error string never appears in `fmt.Sprintf("%+v", resp.Msg)`.

Checked authorization: `d.st.ListScopes(ctx, c.Subj)` / `a.d.st.ListScopes(ctx, subj)` call sites are unchanged from before this phase (per the diff, only the error-handling shape around the call changed, not the call itself or its argument), and the hits themselves come from the pre-existing `d.searchMemory`/`d.listMemory` / `a.d.searchMemory`/`a.d.listMemory` calls whose authz filtering this phase does not touch. The coverage query failing does not widen what the caller can see — it only removes the auxiliary "which scopes did I search" receipt.

Ran `go build ./...`, `go vet` (project-wide, no new findings attributable to these files), and `golangci-lint run` on the two affected packages: all clean.

All reviewed files meet quality standards. No issues found.
