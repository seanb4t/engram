---
phase: 07-cli-cross-spine-wiring
verified: 2026-08-02T18:00:00Z
status: passed
score: 10/10 decisions verified (D-00..D-09)
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 10/10 decisions verified (D-00..D-09)
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 7: CLI Cross-Spine Wiring Verification Report

**Phase Goal:** `engram search` and `engram list` can reach cross-spine recall, and a caller learns
how from help text alone.
**Verified:** 2026-08-02 (re-verification against current tree)
**Status:** passed
**Re-verification:** Yes — after a purely cosmetic change (SPDX comment header stripped from the
three plan SUMMARY.md files, `797ea24f`) tripped `verification.status` into `stale`. No source,
test, or documentation content changed. This report re-derives every prior finding independently
against the current tree rather than carrying the old report forward.

## Why This Is a Re-Verification, Not a Rubber Stamp

`git diff --stat 8d372719 HEAD -- cmd/engram/ internal/server/tools.go docs-site/ CLAUDE.md` (the
prior verification's HEAD forward to the current HEAD) returns **empty** — zero lines changed in
any file this phase's evidence depends on. `git log --oneline -1 -- cmd/engram/client_common.go
cmd/engram/client_search.go cmd/engram/client_list.go internal/server/tools.go` shows the last touch
to any of them was `b582e82a feat(07-02): wire --cross-spine through engram list`, which predates
the original verification. Every command below was re-run in this session against the current
working tree, not assumed from the prior report.

## Goal Achievement

This phase tracks CONTEXT.md decisions D-00..D-09 rather than REQ-* IDs. Each decision was
re-checked against the shipped code in this session (not the SUMMARY narrative, and not the prior
VERIFICATION.md's prose).

### Decision-by-Decision Verification

| # | Decision | Status | Evidence (this session) |
|---|----------|--------|--------------------------|
| D-00 | CLI is correct-by-reading: `--scope`/`--cross-spine` name each other in help, both commands | VERIFIED | Live `go run ./cmd/engram search\|list --help \| grep -iE 'scope\|cross-spine'` shows, on both commands: `--cross-spine ... mutually exclusive with --scope` and `--scope ... mutually exclusive with --cross-spine`. `TestScopeCrossSpineFlagsNameEachOther` re-run with `-v`: `--- PASS` (both `search` and `list` subtests). |
| D-01 | `--cross-spine` passthrough bool; empty-scope-without-cross-spine rejected client-side, exit 2, before dialing | VERIFIED | Live binary against unreachable `http://127.0.0.1:1`: `search` and `list` both print `Error: --scope is required unless --cross-spine is set` / `exit status 2` instantly, no connection-refused symptom. `TestClientSearchMissingScopeIsUsageErrorBeforeDialing` / `TestClientListMissingScopeIsUsageErrorBeforeDialing` re-run: `--- PASS` both. |
| D-02 | One shared guard helper, never per-command | VERIFIED | `rg -c 'func validateScopeCrossSpine' cmd/engram/*.go` returns exactly one nonzero hit (`client_common.go:1`); same for `renderCoverageFooter`. `rg -n 'validateScopeCrossSpine\(' cmd/engram/client_search.go cmd/engram/client_list.go` shows exactly one call site each (line 40 and line 38). |
| D-03 | Parity test pins client guard against server's `EffectiveSearchScope`, with the one documented D-04 divergence named | VERIFIED | `TestValidateScopeCrossSpineParity` re-run with `-v`: 4 named subtests, all `--- PASS`, including `scope_set,_cross-spine_on_(D-04:_client_is_stricter_here)`. Read the test body again in this session — it still computes `clientErr`/`serverErr` independently per row and asserts the one-directional invariant, not equality. |
| D-04 | `--scope` + `--cross-spine` mutually exclusive, exit 2 before dialing, both flags name each other | VERIFIED | Live binary: `search --scope repo:x --cross-spine` against unreachable server → `Error: --scope and --cross-spine are mutually exclusive`, `exit status 2`. `TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing` / `TestClientListScopeWithCrossSpineIsUsageErrorBeforeDialing` re-run: `--- PASS` both. |
| D-05 | Coverage footer on stdout, count-based (never scope names), text lane only | VERIFIED | Read `renderCoverageFooter` (`client_common.go:263`) again — emits `searched_scopes: <count>` (+ `scopes_truncated: true` when applicable), never the scope slice. `TestClientSearchCrossSpineEndToEnd` re-run: `--- PASS`; its body asserts the stub scope names are absent from stdout while the count is present. |
| D-06 | Footer only on cross-spine calls; non-cross-spine text output byte-identical to pre-phase baseline | VERIFIED | `TestClientSearchNoFooterWithoutCrossSpine` / `TestClientListFooterUnchangedWithoutCrossSpine` re-run: both `--- PASS`. Confirmed again this session that "want" is built from the real `renderMemoryTable` production function (not a canned string) with the stub's `SearchedScopes`/`ScopesTruncated` populated — proving gating on the caller's flag, not response emptiness. |
| D-07 | Self-describe catalog carries `--cross-spine` on both commands, same guidance string as `--help` | VERIFIED | Live `go run ./cmd/engram \| jq` (this session) shows `cross-spine` bool/`default:false` on both `search` and `list`, usage string identical to `--help` output. `TestCatalogCarriesCrossSpineGuidance` re-run: `--- PASS` both subtests. |
| D-08 | docs-site CLI reference documents the rule/flags/footer; list example no longer exits 2; upgrade note added | VERIFIED | `docs-site/guides/cli.md` unchanged since prior verification (no diff since `8d372719`); still contains the "Recall scope selection" section stating mutual exclusion, pre-flight rejection, and the coverage footer format. `docs-site/guides/upgrade.md`'s `## v0.12.0` section still carries the CLI cross-spine reach item. |
| D-09 | CLAUDE.md records CLI reachability, no flag syntax | VERIFIED | `CLAUDE.md` — the `search_memory`/`list_memory` cross_spine sentence (now around line 89) is unchanged: names the CLI verbs, states they "reach the same capability and report the same two fields," no flag syntax. `rg -c 'cross-spine' CLAUDE.md` returns 0. |

**Score:** 10/10 decisions verified, 0 present-behavior-unverified, 0 overrides.

### Re-Verification Regression Checks (commands actually run this session)

```
$ go test ./cmd/engram/... -run \
  'TestScopeCrossSpineFlagsNameEachOther|TestCatalogCarriesCrossSpineGuidance|TestClientSearchMissingScopeIsUsageErrorBeforeDialing|TestClientListMissingScopeIsUsageErrorBeforeDialing|TestValidateScopeCrossSpineParity|TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing|TestClientListScopeWithCrossSpineIsUsageErrorBeforeDialing|TestClientSearchCrossSpineEndToEnd|TestClientListCrossSpineEndToEnd|TestClientSearchNoFooterWithoutCrossSpine|TestClientListFooterUnchangedWithoutCrossSpine' -v
--- PASS: TestCatalogCarriesCrossSpineGuidance (subtests: search, list)
--- PASS: TestValidateScopeCrossSpineParity (subtests: all 4, incl. D-04 divergence)
--- PASS: TestScopeCrossSpineFlagsNameEachOther (subtests: search, list)
--- PASS: TestClientListCrossSpineEndToEnd
--- PASS: TestClientListMissingScopeIsUsageErrorBeforeDialing
--- PASS: TestClientListScopeWithCrossSpineIsUsageErrorBeforeDialing
--- PASS: TestClientListFooterUnchangedWithoutCrossSpine
--- PASS: TestClientSearchCrossSpineEndToEnd
--- PASS: TestClientSearchMissingScopeIsUsageErrorBeforeDialing
--- PASS: TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing
--- PASS: TestClientSearchNoFooterWithoutCrossSpine
ok  	github.com/seanb4t/engram/cmd/engram	0.366s

$ go run ./cmd/engram search --server http://127.0.0.1:1 --query "test"
Error: --scope is required unless --cross-spine is set
exit status 2

$ go run ./cmd/engram search --server http://127.0.0.1:1 --query "test" --scope repo:x --cross-spine
Error: --scope and --cross-spine are mutually exclusive
exit status 2

$ go run ./cmd/engram list --server http://127.0.0.1:1
Error: --scope is required unless --cross-spine is set
exit status 2

$ go run ./cmd/engram search --help | grep -iE 'scope|cross-spine'
      --cross-spine     span every scope you can read; mutually exclusive with --scope
      --scope string    limit recall to one scope; omit and pass --cross-spine to span every scope you can read; mutually exclusive with --cross-spine
(identical output shape for `list --help`)

$ go run ./cmd/engram | jq '.commands[] | select(.name=="search" or .name=="list") | {name, cross_spine: (.flags[]? | select(.name=="cross-spine"))}'
{ "name": "list",   "cross_spine": {"name":"cross-spine","type":"bool","default":"false","usage":"span every scope you can read; mutually exclusive with --scope"} }
{ "name": "search", "cross_spine": {"name":"cross-spine","type":"bool","default":"false","usage":"span every scope you can read; mutually exclusive with --scope"} }

$ go build ./... && go vet ./...
(exit 0, both)

$ task
lint: all green (markdown, yaml, actions, go, python)
test:go — all packages ok, including cmd/engram, internal/server, internal/e2e
test:python — 33 passed
```

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/client_common.go` `validateScopeCrossSpine` | D-01/D-02/D-04 guard | VERIFIED | One definition (`client_common.go:234`), called from `client_search.go:40` and `client_list.go:38` only |
| `cmd/engram/client_common.go` `renderCoverageFooter` | D-05/D-06 footer | VERIFIED | One definition (`client_common.go:263`), called from `client_search.go:73` and `client_list.go:85` only, gated on the request's `crossSpine` flag |
| `cmd/engram/client_search.go` `--cross-spine` flag + wiring | D-01/D-00 | VERIFIED | Var, flag, request field, Usage string all present and live-confirmed |
| `cmd/engram/client_list.go` `--cross-spine` flag + wiring | D-01/D-00 | VERIFIED | Var, flag, request field, Usage string all present and live-confirmed |
| `internal/server/tools.go` `EffectiveSearchScope` | D-03 sole authorized export | VERIFIED | Present, pure delegation (`return effectiveSearchScope(scope, crossSpine)`), unchanged since original verification |
| `cmd/engram/client_common_test.go` parity + naming tests | D-03/D-00 | VERIFIED | Present and passing, re-run individually this session |
| `cmd/engram/catalog_test.go` `TestCatalogCarriesCrossSpineGuidance` | D-07 | VERIFIED | Present, passing, re-run this session |
| `docs-site/guides/cli.md`, `upgrade.md` | D-08 | VERIFIED | Unchanged since prior pass, content re-read this session |
| `CLAUDE.md` cross_spine sentence | D-09 | VERIFIED | Unchanged, no flag syntax leaked |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `searchCmd.RunE` | `validateScopeCrossSpine` | called before `resolveOutputFormat`/`clientFromFlags` | WIRED — source re-read + zero-RPC-call assertion re-run passing |
| `listCmd.RunE` | `validateScopeCrossSpine` | called first in `RunE` | WIRED — same pattern re-confirmed |
| `SearchMemoriesRequest.CrossSpine` | `searchCrossSpine` var | request literal field | WIRED — source re-read + `TestClientSearchCrossSpineEndToEnd` re-run passing |
| `ListMemoriesRequest.CrossSpine` | `listCrossSpine` var | request literal field | WIRED — source re-read + `TestClientListCrossSpineEndToEnd` re-run passing |
| text branch (both commands) | `renderCoverageFooter` | called after table render, gated on the flag | WIRED — source re-read; byte-identical baseline tests re-run passing |
| self-describe catalog | live `cobra` flag Usage | `collectFlags` emits `pflag.Flag.Usage` verbatim | WIRED — live `jq` query re-run this session + `TestCatalogCarriesCrossSpineGuidance` re-run passing |

### Requirements Coverage

No `REQ-*` IDs are mapped to this phase (unchanged from original verification). CONTEXT.md's
D-00..D-09 decision IDs remain the phase's traceability contract; all ten accounted for above.

### Anti-Patterns Found

None. `rg -n 'TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER' cmd/engram/client_common.go
cmd/engram/client_search.go cmd/engram/client_list.go internal/server/tools.go` returns nothing
(exit 1, no matches) — re-run this session.

### Containment (re-checked, with a caveat)

`git diff --name-only b4544d47 HEAD -- internal/` now returns four files, not the single
`internal/server/tools.go` the original report cited. Investigated: the three extra files
(`internal/e2e/cli_exitcode_test.go`, `internal/server/schemarequired_test.go`,
`internal/store/store_test.go`) were added by **later, unrelated phases** (`5c304d64`
phase-02, `c84fad6f` phase-03, `80bd7d5f` phase-04) that landed on this branch after Phase 7's
original verification — this branch is now several phases downstream of Phase 7. `git diff b4544d47
HEAD -- internal/server/tools.go` still shows exactly the same pure additive delegating wrapper
(`EffectiveSearchScope` → `effectiveSearchScope`) with no other line changed. Phase 7's own
containment claim holds; the base-commit diff technique is simply no longer a clean single-phase
diff now that the branch has moved forward — not a Phase 7 regression.

### Behavioral Spot-Checks / Gate Re-Runs (this session)

| Check | Command | Result |
|-------|---------|--------|
| Named guard/footer/parity/catalog tests (11 total) | `go test ./cmd/engram/... -run '<11 names>' -v` | 11x `--- PASS`, 0 FAIL/SKIP |
| Parity test subtests individually | (included above) | 4x `--- PASS` incl. named D-04 divergence subtest |
| Full package build/vet | `go build ./...`, `go vet ./...` | both exit 0 |
| Full repo gate | `task` (lint + full test suite, all languages) | all green, 33 python tests passed |
| Zero new deps since phase base | `git diff --exit-code b4544d47 -- go.mod go.sum` | clean |
| Live `--help` on both commands | `go run ./cmd/engram search\|list --help` | bidirectional naming confirmed |
| Live catalog JSON | `go run ./cmd/engram \| jq` | `cross-spine` present on both commands, correct type/default |
| Live regression (3 scenarios) | direct binary invocation against unreachable server | guard fires before dial in all 3, exit 2 |

### Human Verification Required

None. Nothing changed since the prior human-verification-free pass; every must-have remains either
a compile-time-pinned invariant, a passing behavioral test re-run this session, or a directly
re-observed live binary result.

### Gaps Summary

No gaps, and no regressions. All ten CONTEXT.md decisions (D-00 through D-09) remain satisfied, with
every cited test and live command independently re-run against the current tree in this session
(not carried forward from the prior report's prose). The `stale` trigger was confirmed to be exactly
what it claimed: an SPDX header strip on the three plan SUMMARY.md files, with zero effect on any
source, test, or documentation file this phase's evidence depends on.

---

_Verified: 2026-08-02 (re-verification)_
_Verifier: Claude (gsd-verifier)_
