---
phase: 04-diagnosability
verified: 2026-08-01T19:53:32Z
status: passed
score: 4/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 4: Diagnosability Verification Report

**Phase Goal:** What the server decided, and why it rejected something, reaches whoever needs it —
the operator debugging a denial, the agent retrying a rejected call.

**Verified:** 2026-08-01T19:53:32Z
**Status:** passed
**Re-verification:** No — initial verification

## Method

This verification did not trust SUMMARY.md claims. Every truth below was checked against the live
tree: source files were read directly, the named pinning tests were run with `-v -count=1` and their
`--- PASS:` output inspected, `git log -p`/`-S` was used to confirm commit-boundary claims (not
prose), and every repo quality gate was executed fresh in this session (not read from a prior run).

## Goal Achievement

### Observable Truths (the four ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | At debug level, both an allowed and a denied authorization decision emit a log line carrying field-allowlisted Cedar diagnostics; no full expression trace is ever emitted | ✓ VERIFIED | `internal/store/store.go:731-773` — `decideBucket`/`decideRecord` call `slog.DebugContext` **unconditionally** (not gated on `d.Allow`) with exactly `allow`, `action`, `bucket` (bucket-arm only), `policy_ids`, `policy_error_count`. `internal/authz/authz.go`: `Decision.diag` stays unexported; `(Decision).Log()` is the only accessor and never reads `DiagnosticError.Message`/`Position`. **Negative gate present and run:** `TestDecisionLogNeverLeaksExpressionTrace` (`internal/authz/authz_test.go:128`) constructs allow/deny/multi-policy/**error-carrying** `Decision` values with a sentinel planted in `DiagnosticError.Message` and asserts the marshaled `DecisionLog` never contains it — ran green, all 4 subtests. **`internal/authz` emits zero `slog` calls**, confirmed by direct read of `authz.go` (only import is `cedar-go`) and by `rg -v '^\s*//' -g '!*_test.go' internal/authz/ \| rg -c 'log/slog\|slog\.'` returning empty (0). |
| 2 | An argument-validation rejection names the field that actually failed, proven by a matrix with one case per single-field-invalid input rather than by matching exact wording | ✓ VERIFIED | `internal/server/argattribution_test.go` — `TestValidationErrorAttributionMatrix`, **23 named subtests**, ran green. `assertEnvelope` (line 198) asserts `reflect.DeepEqual` on the **field-name SET** and exact equality on the hint code — never on message text. Confirmed by direct read: no `strings.Contains`/message assertion anywhere in the matrix. |
| 3 | A rejection carries a structured remediation hint alongside the field attribution | ✓ VERIFIED | `internal/server/argerror.go` — `argError{Fields, Hint, Detail, Class}` is the single envelope; every converted rejection site (`argErrf`/`argErrFieldsf`) carries both. `TestHintNeverEchoesValue` (7 subtests, D-12 negative gate) ran green with a recorded RED transcript proving the gate can fail. D-11a's three previously-bare-unwrapped validators (`validateStoreDiscovery`, `validateCitations`, `validateStoreRule`) are closed — `TestStoreDiscoveryValidationIsNotCodeInternal` (5), `TestCitationValidationIsNotCodeInternal` (6), `TestStoreRuleValidationIsNotCodeInternal` (7) all ran green (18 subtests total, no `CodeInternal`). |
| 4 | A non-2xx embeddings response surfaces a bounded prefix of the provider's error body alongside the status code and drains the body for connection reuse; the chat/summarize lane has been audited for the same gap and fixed if it shares it | ✓ VERIFIED | `internal/embed/embed.go:282-296` — non-2xx branch reads a bounded (4096-byte) verbatim prefix into the error alongside `resp.StatusCode`, then drains via `io.Copy(io.Discard, resp.Body)`. `internal/summarize/summarize.go:180-191` — **audit correctly asymmetric**: surfacing was already present (untouched), the drain was **added** to both the error branch (line 182) and the success branch (line 191) — confirmed by direct read, this is the half of the audit the executor could have quietly skipped and did not. `TestEmbedNon2xxDrainsForReuse`/`TestSummarizeNon200DrainsForReuse` (httptrace-based `ReuseTracker`) both ran green. |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/server/argerror.go` | Field+hint error envelope, 10-code `HintCode` vocabulary, `ConnectCode()` class mapping | ✓ VERIFIED | Exists, substantive, wired into `connecterror.go`'s `*argError` dispatch (placed first, ahead of the `store.ErrInvalidArgument` sentinel arm) |
| `internal/authz/authz.go` (`DecisionLog`/`Log()`) | D-02 allowlist accessor, D-03 structural enforcement | ✓ VERIFIED | `diag` unexported; `Log()` sole accessor; field set pinned by `TestDecisionLogCarriesOnlyAllowlistedFields` (3-field struct, name-set equality) |
| `internal/store/store.go` (`decideBucket`/`decideRecord`) | Unconditional debug log at both chokepoints | ✓ VERIFIED | Both functions log unconditionally; confirmed O(1)/request by direct read — every production call funnels through these two functions |
| `internal/embed/embed.go` | Bounded error-body surfacing, drain, bounded success decode | ✓ VERIFIED | `maxErrorBodyBytes=4096`; `WithMaxResponseBytes` wired from `ENGRAM_EMBED_DIM` in `tools.go:embedderFromConfig` |
| `internal/summarize/summarize.go` | Drain added (surfacing pre-existing) | ✓ VERIFIED | Two `io.Copy(io.Discard, resp.Body)` drains, error+success branches |
| `internal/testhttp/reuse.go` | Shared connection-reuse test helper | ✓ VERIFIED | `ReuseTracker` used by both `embed_test.go` and `summarize_test.go`; no test-framework import (`rg -c '"testing"'` = 0) |
| `internal/server/tools.go`, `rules.go`, `connectapi.go` | Every single-field/relational rejection converted; all 7 hand-wrapped `CodeInvalidArgument` sites removed | ✓ VERIFIED | `grep -c "connect.NewError(connect.CodeInvalidArgument" internal/server/connectapi.go` = 0 (was 7 pre-phase, confirmed against the `04-CONTEXT.md`-cited line numbers `:149,:153,:160,:173,:230,:234,:243`). `TestConnectValidationCodeMapping` (12 subtests, 5 driven through real `ListMemories`/`SearchMemories`/`StoreDiscovery` handlers, including a `CodeOutOfRange` and a `CodeFailedPrecondition` row) ran green — the mapping is live, not decorative. |
| `internal/config/registry.go` (`memory.max_summary_bytes`) | Koanf-configurable summary bound, default 512, 0 disables | ✓ VERIFIED | `{Key: "memory.max_summary_bytes", Env: "ENGRAM_MEMORY_MAX_SUMMARY_BYTES", Default: "512"}`; enabled by default (not a dormant knob) |
| `docs-site/src/content/docs/reference/errors.md` | Envelope reference, all 10 hint codes, class→code table | ✓ VERIFIED | Exists, 10/10 hint-code rows present |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/authz.Decision` | `internal/store` debug log | `(Decision).Log()` accessor | ✓ WIRED | Only unexported→accessor read path; confirmed no direct `diag` field access outside `authz.go` |
| `argError` (tools.go/rules.go rejections) | Connect error codes | `connectError`'s `errors.As(err, &ae)` → `ae.ConnectCode()` | ✓ WIRED | Live-handler test (`TestConnectValidationCodeMapping`) proves the handler no longer overrides the class with a hand-wrap |
| `embed.go`/`summarize.go` non-2xx body read | Connection pool reuse | `io.Copy(io.Discard, resp.Body)` before `Close` | ✓ WIRED | `ReuseTracker`-based tests directly observe `Reused()>0` on a second request through the same client |
| `ENGRAM_EMBED_DIM` | `embed.WithMaxResponseBytes` | `embedderFromConfig` in `tools.go` | ✓ WIRED | `rg -q 'WithMaxResponseBytes' internal/server/tools.go` found; confirmed by direct read of the dimension-derived bound calculation |

### Adversarial Checklist (from verification instructions)

| Check | Finding |
|-------|---------|
| **Criterion 1's prohibition — negative test** | Confirmed present and run: `TestDecisionLogNeverLeaksExpressionTrace`, 4 cases (allow/deny/multi-policy/error-carrying), asserts absence of a sentinel marker across a JSON-marshaled `DecisionLog`. `internal/authz` verified to emit zero `slog` calls (grep + direct source read). |
| **Criterion 2 — field, not wording** | `TestValidationErrorAttributionMatrix`, 23 subtests, `reflect.DeepEqual` on the full field-name **set** plus exact hint-code equality; zero message-text assertions anywhere in the matrix (direct read). |
| **#360 regression** | `TestIssue360SummaryLengthNamesSummary` reproduces the exact repro (valid `content`, oversized `summary`) and asserts the field set is `["summary"]`, explicitly rejecting `"content"` in the set. Paired with `TestIssue360PositiveControl` (previously-succeeding shapes still succeed). The bound (`ENGRAM_MEMORY_MAX_SUMMARY_BYTES`, default `512`) is **enabled by default**, so the regression is closed in shipped behavior, not merely behind an opt-in flag. The executor's honest RED-transcript note ("with the bound disabled the failure mode is 'no error at all', not 'names content'") was verified accurate by reading `validateStoreArgs`: the four checks are field-independent once required-ness lives in Go, so disabling one check does not make an unrelated check misfire — this is expected, correct post-fix behavior, not evidence the fix is incomplete. |
| **`delete_all` safety edge** | `git log --all -S'Scope string \`json:"scope,omitempty"\`' -- internal/server/tools.go` resolves to exactly one commit, `98c9bc36`, and `git show 98c9bc36` contains both the `scopeArgs.Scope` tag relaxation and the new `deps.deleteAll` presence check in the same diff — no commit in history has one without the other. `TestDeleteAllRequiresScope` inspects a spy store's call log and fails the test if any `DeleteAll` call was recorded for a rejected empty-scope request — proves ordering, not merely error presence. |
| **Criterion 4's asymmetry** | Confirmed by direct read: `internal/summarize/summarize.go` gained exactly two `io.Copy(io.Discard, resp.Body)` drains (error path line 182, success path line 191) and no change to its pre-existing body-surfacing code — the audit found and fixed the shared drain gap without altering the lane that was already correct. |
| **D-11's mapping is not a no-op** | All seven `connectapi.go` hand-wraps confirmed gone (`grep -c` = 0). `TestConnectValidationCodeMapping` drives 5 of its 12 rows through the *real* `ListMemories`/`SearchMemories`/`StoreDiscovery` handlers and observes `CodeOutOfRange` and `CodeFailedPrecondition` alongside `CodeInvalidArgument` — a green suite asserting `CodeInvalidArgument` everywhere is explicitly what this test structure rules out, since it would fail on the non-`InvalidArgument` rows. |
| **RESEARCH rows 18-21 coverage note** | Adjudicated: **acceptable coverage, not a gap.** Verified by direct read that the four inline MCP-closure window parses (`tools.go:1808,1812,1855,1859`) use the byte-identical construction (`argErrf(classMalformed, HintFormat, "created_after"/"created_before", "... must be RFC3339")`) as the tested `listScheduled` rows (`tools.go:1287,1291`). The untested sites are the same three-line call already proven by the matrix at a different call site, not new logic; `go vet` covers compilation, and the whole-file `got %q` zero-count gate covers the value-echo discipline. Standing up an `mcp.NewInMemoryTransports()` harness (the only way to reach these closures, since `Register()` requires env-configured Qdrant/embedder state with no injectable double) to re-prove an identical three-line pattern would be disproportionate. This is a documented, low-risk coverage note, not a silent gap — it is called out in both the plan's SUMMARY and the test's own doc comment. |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|------------|--------|----------|
| REQ-authz-decision-diagnostics | 04-02, 04-07 | ✓ SATISFIED | Code (04-02) + operator docs (04-07, `guides/configure.md`'s Logging section naming exact field names and the volume bound) |
| REQ-validation-error-attribution | 04-01, 04-04, 04-05, 04-06 | ✓ SATISFIED | Matrix (23 cases) + #360 regression + schema-level D-06a relocation (25-case `TestSchemaRequiredMovedToGoLevel`) |
| REQ-error-hint-envelope | 04-01, 04-04, 04-05, 04-06, 04-07 | ✓ SATISFIED | Envelope + D-11a closure (3/3 validators) + Connect mapping live + agent-facing skill guidance (`curating-memory` SKILL.md "Reading a rejection") |
| REQ-embed-provider-error-body | 04-03, 04-06 | ✓ SATISFIED | Bounded surfacing + both-lane drain + `ENGRAM_EMBED_DIM`-sized success bound |

All four requirements are marked `[x]` and `Complete` in `.planning/REQUIREMENTS.md`, consistent with the code evidence above (not merely trusted from the file).

### Repo Quality Gates (run fresh this session)

| Gate | Result |
|------|--------|
| `go vet ./...` | clean |
| `task` (lint + full Go + Python test suite) | all green — 15 Go packages `ok`, 33 Python tests passed |
| `git diff --exit-code -- go.mod go.sum` | zero diff (no new dependencies) |
| `task license:check` | 0 invalid (241 valid, 878 ignored) |
| `task proto:lint` | clean |
| `task proto:gen && git diff --exit-code -- gen/ ui/src/lib/gen/` | zero diff |
| `task ui:build && git status --short` | zero diff (no untracked/modified files after build) |
| `task chart:validate` | OK (checksum-pinned `engram.containerEnv` block unchanged; Helm lint clean) |

### Anti-Patterns Found

None. Scanned all phase-touched core files (`argerror.go`, `tools.go`, `rules.go`, `connecterror.go`,
`connectapi.go`, `authz.go`, `store.go`, `embed.go`, `summarize.go`, `config.go`, `registry.go`,
`validate.go`) for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` — zero matches. No stub returns, no empty
handlers, no hardcoded empty data flowing to a rejection path.

### Human Verification Required

None. All four criteria have direct, run, passing test evidence; no behavior-dependent truth was
left unexercised.

### Gaps Summary

None. All four ROADMAP success criteria for Phase 4 are independently verified against the live
codebase: tests were run (not read as claims), commit history was inspected for the `delete_all`
ordering guarantee, and the two places most likely to hide a decorative fix — the D-11 Connect code
mapping and the #360 regression's default-enabled status — were specifically checked and found live,
not cosmetic.

---

*Verified: 2026-08-01T19:53:32Z*
*Verifier: Claude (gsd-verifier)*
