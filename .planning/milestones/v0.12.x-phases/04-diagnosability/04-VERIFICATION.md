---
phase: 04-diagnosability
verified: 2026-08-02T19:17:00Z
status: passed
score: 4/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 4/4
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 4: Diagnosability Verification Report

**Phase Goal:** What the server decided, and why it rejected something, reaches whoever needs it —
the operator debugging a denial, the agent retrying a rejected call.

**Verified:** 2026-08-02T19:17:00Z
**Status:** passed
**Re-verification:** Yes — re-run after `verification.status` reported `stale`. The trigger was
`cb737042` (`fix(planning): normalize coverage verification kinds`), which changed 04-04/05/06/07
SUMMARY.md `coverage[].kind: static` → `kind: other` — `static` is not in `uat.classify-coverage`'s
enum, and `other` is the correct GSD value for the same evidence (region-scoped `rg`/`grep`
zero-count checks). Confirmed by direct diff read (`git show cb737042 -- .planning/phases/04-*`):
every changed line is a `kind:` value swap; no `ref`, `status`, `description`, or `requirement`
field moved. No source file changed in that commit.

## Method

This re-verification did not trust SUMMARY.md, VALIDATION.md, or the prior VERIFICATION.md's
claims. Every truth below was re-checked against the current tree in this session: source files
were read directly, the named pinning tests were run fresh with `-v -count=1` under
`ENGRAM_REQUIRE_QDRANT=1` (so a testcontainers skip cannot masquerade as a pass) and their
`--- PASS:` output inspected, and `git diff dc98ec0c..HEAD` (the commit that recorded the original
4/4 pass) was used to positively confirm which files changed since the original verification and
which didn't — rather than assuming the tree was static.

**What changed in the repo since the original 2026-08-01 pass, and its relevance:**

| Commit | What | Touches Phase 4's diagnosability contract? |
|---|---|---|
| `80bd7d5f` | Added `internal/server/schemarequired_test.go` (`TestSchemaGeneratedRequiredExcludesRelaxedFields`) — new test closing a gap found by a retroactive Nyquist audit (D-06a's schema half was previously untested) | Yes — new evidence, strictly additive, re-run below |
| `17c348a9` | `04-VALIDATION.md` audit narrative documenting the above gap + a doc defect (row 4-A-02 named an unrunnable `go test` path) | Docs only |
| `cb737042` | `kind: static` → `kind: other` in four SUMMARY.md `coverage:` blocks | Docs only, enum-conformance value fix |
| `8d372719` | Added `04-SECURITY.md` (retroactive security audit) | Docs only |
| Phase 5 / 7 work (`b59a30b6`, `5fd8b051`, `36b5150b`, `327fa9d6`, `119cb2f8`, `b582e82a`, `7821500f`, `d3f47669`, `c84fad6f`, `5c304d64`) | `internal/store/store.go` (reindex tag-awareness), `internal/server/tools.go` (per-lane chat API key, exported `EffectiveSearchScope`), `internal/config/registry.go` (`openai.chat_api_key`), `cmd/engram/*` (cross-spine flag) | No — confirmed by direct diff read below |

`git diff dc98ec0c..HEAD -- internal/server/argerror.go internal/server/rules.go internal/server/connecterror.go internal/server/connectapi.go internal/authz/authz.go internal/embed/embed.go internal/summarize/summarize.go` is **empty** — the five files carrying Phase 4's three absence guarantees (Cedar diagnostic allowlisting, hint-value redaction, provider error-body bounding) are byte-identical to the tree the original VERIFICATION.md certified. `internal/store/store.go`'s diff was inspected in full: the only change is Phase 5's reindex tag-equality logic (`tagsEqual`/`tagsFromPayload`/`WouldUpsert`); `decideBucket`/`decideRecord` (Phase 4's log chokepoints, cited at `store.go:731-773` in the original report) do not appear in the diff at all. `internal/server/tools.go`'s diff is Phase 5's `chatAPIKey` resolution and Phase 7's exported `EffectiveSearchScope` wrapper — neither touches an argument-validation or hint-emission path.

## Goal Achievement

### Observable Truths (the four ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | At debug level, both an allowed and a denied authorization decision emit a log line carrying field-allowlisted Cedar diagnostics; no full expression trace is ever emitted | ✓ VERIFIED | `internal/authz/authz.go`, `internal/store/store.go` unchanged since original pass (confirmed above). Re-ran `TestDecisionLogNeverLeaksExpressionTrace` fresh (`go test ./internal/authz/... -run TestDecisionLogNeverLeaksExpressionTrace -v -count=1`, `ENGRAM_REQUIRE_QDRANT=1`): `--- PASS` on all 4 subtests (allow/deny/multi-policy/error-carrying). Direct re-read of `authz.go` confirms `diag` is still unexported with `Log()` the sole accessor. |
| 2 | An argument-validation rejection names the field that actually failed, proven by a matrix with one case per single-field-invalid input rather than by matching exact wording | ✓ VERIFIED | `internal/server/argattribution_test.go` unchanged. Re-ran `TestValidationErrorAttributionMatrix` fresh under `ENGRAM_REQUIRE_QDRANT=1`: all subtests `--- PASS`, including the `#360` regression subtest via the newly-added `TestIssue360SummaryLengthNamesSummary` (2 subtests, both PASS) which is now backed by `TestSchemaGeneratedRequiredExcludesRelaxedFields` proving the *schema* half of the fix (D-06a) is also enforced, closing a gap the original verification's own test suite left silently unguarded. |
| 3 | A rejection carries a structured remediation hint alongside the field attribution | ✓ VERIFIED | `internal/server/argerror.go` unchanged. Re-ran `TestHintNeverEchoesValue` fresh (7 subtests) — all `--- PASS`. Re-ran `TestConnectValidationCodeMapping` fresh (12 subtests, 5 handler-driven) — all `--- PASS`, confirming D-11's Connect code mapping and D-11a's `CodeInternal` closure are still live, not decorative. |
| 4 | A non-2xx embeddings response surfaces a bounded prefix of the provider's error body alongside the status code and drains the body for connection reuse; the chat/summarize lane has been audited for the same gap and fixed if it shares it | ✓ VERIFIED | `internal/embed/embed.go`, `internal/summarize/summarize.go` unchanged. Re-ran `TestEmbedNon2xxIncludesStatusAndBody`, `TestEmbedNon2xxDrainsForReuse`, `TestSummarizeNon200DrainsForReuse` fresh — all `--- PASS`. |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Fresh test evidence this session (exact commands, all `--- PASS`)

```
ENGRAM_REQUIRE_QDRANT=1 go test ./internal/authz/... -run TestDecisionLogNeverLeaksExpressionTrace -v -count=1
ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestHintNeverEchoesValue -v -count=1
ENGRAM_REQUIRE_QDRANT=1 go test ./internal/embed/... -run 'TestEmbedNon2xxIncludesStatusAndBody|TestEmbedNon2xxDrainsForReuse' -v -count=1
ENGRAM_REQUIRE_QDRANT=1 go test ./internal/summarize/... -run TestSummarizeNon200DrainsForReuse -v -count=1
ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run 'TestSchemaGeneratedRequiredExcludesRelaxedFields|TestValidationErrorAttributionMatrix|TestConnectValidationCodeMapping|TestIssue360SummaryLengthNamesSummary' -v -count=1
```

All named tests produced `--- PASS:` for every case (or subtest); no test was matched-to-nothing
(each command's package output confirmed `RUN`/`PASS` pairs for the exact requested names, ruling
out the "matches nothing, exits 0" trap).

### Required Artifacts

Unchanged from the original pass — re-confirmed present, substantive, and wired by direct read in
this session:

| Artifact | Status | Details |
|----------|--------|---------|
| `internal/server/argerror.go` | ✓ VERIFIED | Byte-identical to `dc98ec0c` (`git diff` empty); wired into `connecterror.go` first in the type switch |
| `internal/authz/authz.go` | ✓ VERIFIED | Byte-identical to `dc98ec0c`; `diag` unexported, `Log()` sole accessor |
| `internal/store/store.go` (`decideBucket`/`decideRecord`) | ✓ VERIFIED | Diff since `dc98ec0c` is Phase 5 reindex tag-awareness only; the two log chokepoints do not appear in the diff |
| `internal/embed/embed.go` | ✓ VERIFIED | Byte-identical to `dc98ec0c` |
| `internal/summarize/summarize.go` | ✓ VERIFIED | Byte-identical to `dc98ec0c` |
| `internal/server/tools.go`, `rules.go`, `connectapi.go` | ✓ VERIFIED | `rules.go`/`connectapi.go` byte-identical; `tools.go`'s diff is Phase 5/7 additions (`chatAPIKey`, exported `EffectiveSearchScope`) that don't touch a validation/hint path — `TestConnectValidationCodeMapping` re-run confirms zero hand-wrapped `CodeInvalidArgument` sites remain |
| `internal/config/registry.go` (`memory.max_summary_bytes`) | ✓ VERIFIED | Diff since `dc98ec0c` only adds `openai.chat_api_key` (Phase 5, unrelated); the `memory.max_summary_bytes` entry is untouched |
| `docs-site/src/content/docs/reference/errors.md` | ✓ VERIFIED | No changes since original pass; still 10/10 hint-code rows |

### Adversarial Checklist (this phase's three absence guarantees, re-checked)

| Check | Finding |
|-------|---------|
| **No Cedar expression trace in the authz log** | `TestDecisionLogNeverLeaksExpressionTrace` re-run green, 4 subtests, including the error-carrying case with a planted sentinel in `DiagnosticError.Message`. `internal/authz/authz.go` and `internal/store/store.go`'s `decideBucket`/`decideRecord` confirmed byte-for-byte unchanged since the original pass — no regression surface exists. |
| **No rejected value echoed in a hint** | `TestHintNeverEchoesValue` re-run green, 7 subtests. `internal/server/argerror.go` confirmed byte-identical to the originally-verified tree. |
| **Bounded provider error bodies** | `TestEmbedNon2xxIncludesStatusAndBody`, `TestEmbedNon2xxDrainsForReuse`, `TestSummarizeNon200DrainsForReuse` all re-run green. `internal/embed/embed.go` and `internal/summarize/summarize.go` confirmed byte-identical to the originally-verified tree. |
| **D-06a's previously-unguarded schema half (found by the 2026-08-02 retroactive Nyquist audit)** | `TestSchemaGeneratedRequiredExcludesRelaxedFields` re-run green — asserts, for all 24 relaxed fields across 13 arg structs, that each is absent from the *generated* MCP schema's `required` set (not just enforced in Go). This closes a real gap the original 4/4 pass did not catch (the schema-generation half of issue #360's fix had zero automated coverage at original verification time); it is now covered and green. |

### Requirements Coverage

Unchanged — all four requirements remain `[x]` / `Complete` in `.planning/REQUIREMENTS.md`, and the
supporting evidence (envelope + Connect mapping + Cedar log allowlist + provider-body bound) is
re-confirmed live in this session, not re-read from a prior claim.

| Requirement | Status | Evidence |
|-------------|--------|----------|
| REQ-authz-decision-diagnostics | ✓ SATISFIED | `TestDecisionLogNeverLeaksExpressionTrace` re-run green; `store.go`/`authz.go` unchanged |
| REQ-validation-error-attribution | ✓ SATISFIED | `TestValidationErrorAttributionMatrix` + `TestSchemaGeneratedRequiredExcludesRelaxedFields` (new, closes the audit gap) re-run green |
| REQ-error-hint-envelope | ✓ SATISFIED | `TestHintNeverEchoesValue` + `TestConnectValidationCodeMapping` re-run green |
| REQ-embed-provider-error-body | ✓ SATISFIED | `TestEmbedNon2xxIncludesStatusAndBody`, `TestEmbedNon2xxDrainsForReuse`, `TestSummarizeNon200DrainsForReuse` re-run green |

### Repo Quality Gates (run fresh this session)

| Gate | Result |
|------|--------|
| `go vet ./...` | clean |
| `task` (lint + full Go + Python test suite) | all green — 15 Go packages `ok`, 33 Python tests passed |
| `git diff --exit-code -- go.mod go.sum` | zero diff (no new dependencies) |
| `task license:check` | 0 invalid (242 valid, 932 ignored — count grew from unrelated Phase 5/7 files, still 0 invalid) |
| `task proto:lint` | clean |
| `task proto:gen && git diff --exit-code -- gen/ ui/src/lib/gen/` | zero diff |

### Anti-Patterns Found

None. Re-scanned all phase-touched core files (`argerror.go`, `tools.go`, `rules.go`,
`connecterror.go`, `connectapi.go`, `authz.go`, `store.go`, `embed.go`, `summarize.go`,
`registry.go`) plus `docs-site/src/content/docs/reference/errors.md` for
`TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` — zero matches.

### Human Verification Required

None. All four criteria have direct, freshly-run, passing test evidence against the current tree;
no behavior-dependent truth was left unexercised.

### Gaps Summary

None. This re-verification confirms the phase goal still holds against the current codebase: the
five source files carrying Phase 4's three absence guarantees are unchanged since the original
2026-08-01 pass, all named pinning tests re-ran green under `ENGRAM_REQUIRE_QDRANT=1`, and the one
piece of genuine post-verification change to this phase's scope — `TestSchemaGeneratedRequiredExcludesRelaxedFields`,
added by a retroactive Nyquist validation audit — is additive evidence that closes a real gap
(the schema-generation half of #360's fix was previously untested) rather than a regression. The
`stale` status that triggered this re-run was correctly conservative but ultimately a false alarm:
the triggering commit (`cb737042`) only corrected an enum value (`kind: static` → `kind: other`) in
SUMMARY.md coverage metadata, touching zero source files.

---

*Verified: 2026-08-02T19:17:00Z*
*Verifier: Claude (gsd-verifier)*
