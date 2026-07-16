---
phase: 20-correctness-polish
reviewed: 2026-07-15T00:00:00Z
depth: standard
files_reviewed: 15
files_reviewed_list:
  - proto/engram/v1/engram.proto
  - internal/server/connectapi.go
  - internal/server/connectapi_test.go
  - internal/server/connectdescriptor_test.go
  - internal/server/tools_test.go
  - internal/embed/embed.go
  - internal/config/embedparams.go
  - internal/config/embedparams_test.go
  - internal/store/store.go
  - internal/store/store_test.go
  - charts/engram/templates/_helpers.tpl
  - charts/engram/templates/memory-mcp.yaml
  - charts/engram/templates/summarize-cronjob.yaml
  - charts/engram/values.yaml
  - Taskfile.yaml
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 20: Code Review Report

**Reviewed:** 2026-07-15
**Depth:** standard
**Files Reviewed:** 15
**Status:** issues_found

## Narrative Findings (AI reviewer)

### Summary

Phase 20 bundles six small correctness/polish fixes: additive `kind`/`citations`
proto fields (#307) with a read-path `citationsToProto` mapper, a verification-only
`storeDiscoveryArgs.ID` schema regression test (#303), single-source reserved-embed-key
list (#304), the `embed()` single-path body build, a bounded `MintShortID` retry cap
with `ErrShortIDExhausted`, and a Helm `_helpers.tpl` env-template extraction feeding a
new `summarize-cronjob.yaml` (#269).

Correctness of the load-bearing changes checks out. The `MintShortID` cap logic is
sound: the loop bounds real Qdrant `Count` checks at `maxMintAttempts` (16), the
`seen`-map budget accounting (`attempts--` then loop `attempts++` nets zero) is exactly
covered by `TestMintShortIDSeenMapDoesNotConsumeBudget`, exhaustion returns an
`errors.Is`-checkable error, and all four callers (`tools.go` x3, `rules.go`, plus the
batch caller at `store.go:1874`) propagate the error rather than using an empty id. The
`embed()` single map path produces `model`+`input` correctly for both empty and
non-empty params (covered by the embed_test "no params configured produces exactly
model+input" case); alphabetical key ordering from map marshalling is JSON-semantically
identical, as the comment claims. The `Citation` proto reuse on the read path is safe
(the validate interceptor only runs on requests, so out-of-enum stored `kind` values
serialize on responses without rejection). The `_helpers.tpl` checksum guard is
internally consistent — I reproduced `0a35aae0...366f9` from the committed file, and the
awk end-anchor `^{{- end -}}$` correctly stops at the block terminator without tripping
on the nested `{{- end }}` lines.

Two process/robustness gaps are worth fixing; nothing blocks shipping.

## Warnings

### WR-01: `chart:validate` guardrail is orphaned — never runs in CI or the default task chain

**File:** `Taskfile.yaml:117-152`, `.github/workflows/ci.yaml:77-84`
**Issue:** The new `chart:validate` target encodes the phase's real invariants — the
default render must NOT emit a CronJob (D-07), `--set ...cronjob.enabled=true` must emit
one with `concurrencyPolicy: Forbid` and the daily schedule (D-08), and the
`engram.containerEnv` block must match `EXPECTED_CHECKSUM` (the D-09 no-drift pin). But
nothing invokes it: it is absent from `default` (`lint` + `test`), absent from the
`lint` deps (`lint:go/yaml/actions/markdown/python`), and the CI `chart` job runs only
`helm lint charts/engram`. A drift guard that no automated gate executes provides false
confidence — the containerEnv template can drift, or a future edit can break the
CronJob-default/`concurrencyPolicy` contract, and CI stays green. The checksum pin is
especially load-bearing because the Deployment and CronJob share that block by design;
silent drift there is exactly what the pin exists to catch.
**Fix:** Wire it into an automated gate, e.g. add to the CI chart job:
```yaml
  chart:
    name: helm chart
    steps:
      - uses: actions/checkout@...
      - uses: azure/setup-helm@...
      - name: chart validate
        run: task chart:validate   # (add Task setup) — or inline the guards
```
or add `chart:validate` to the `default`/`lint` dependency chain so `task` runs it.

### WR-02: `MintShortID` termination guarantee has a hole for a degenerate candidate generator

**File:** `internal/store/store.go:1790-1821`
**Issue:** `maxMintAttempts` was introduced specifically to guarantee `MintShortID`
never hangs. The `seen`-map skip path deliberately does not consume the budget:
```go
if _, dup := seen[cand]; dup {
    attempts-- // seen-map hits don't consume the real-collision-check budget
    continue
}
```
With the loop post-statement `attempts++`, a `seen` hit nets zero, so if `gen()` ever
returns *only* values already in `seen`, the loop spins forever — the very
non-termination the cap was added to eliminate. This is unreachable with the production
`shortid.New` generator (random over a 32^10 space; `seen` is negligibly small), so the
real-world impact is nil, but the `mintCandidate` seam is injectable and the stated
guarantee is now conditional rather than absolute.
**Fix:** Bound total iterations (including `seen` skips) with a separate spin cap so
termination is unconditional, e.g.:
```go
const maxSpins = maxMintAttempts * 100
for attempts, spins := 0, 0; attempts < maxMintAttempts; spins++ {
    if spins >= maxSpins {
        return "", fmt.Errorf("%w: generator degenerate (only seen candidates)", ErrShortIDExhausted)
    }
    cand, genErr := gen()
    // ... on seen-hit: continue without incrementing attempts
    // ... on real check: attempts++
}
```

## Info

### IN-01: `embed()` discards the `json.Marshal` error

**File:** `internal/embed/embed.go:242`
**Issue:** `body, _ := json.Marshal(m)` silently drops the marshal error. Safe today
because `m` holds only JSON-origin scalars/maps (operator params come from
`ParseEmbedParams`'s `json.Unmarshal`, plus string `model`/`input`), and this matches
the pre-refactor behavior. But the discarded error is a latent trap: if a future caller
of `WithQueryParams`/`WithDocumentParams` ever passes a non-marshalable value
(func/chan/cyclic), the request silently posts an empty/garbage body instead of failing.
**Fix:** `body, err := json.Marshal(m); if err != nil { return nil, err }`.

### IN-02: No end-to-end test that discovery citations round-trip on the Connect wire

**File:** `internal/server/connectapi_test.go:520-551, 885-973`
**Issue:** `TestMemoryToProtoMapsKindAndCitations` covers the `citationsToProto` unit
mapping, but the `SearchDiscoveries` handler tests (`seedDiscoveries`) never populate
`Citations`, so no test asserts that a stored discovery's citations actually survive the
`SearchDiscoveries` -> `memoriesToProto` -> wire path introduced this phase. Functionally
covered transitively (memoriesToProto just calls memoryToProto), but the #307 delivery —
citations reaching Connect discovery consumers — has no integration assertion.
**Fix:** Add a `Citations` field to one `seedDiscoveries` record and assert
`resp.Msg.Discoveries[i].Citations` is non-empty and field-equal in an existing
`TestConnectSearchDiscoveries*` case.

---

_Reviewed: 2026-07-15_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
