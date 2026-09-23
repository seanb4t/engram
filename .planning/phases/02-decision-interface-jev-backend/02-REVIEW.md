---
phase: 02-decision-interface-jev-backend
reviewed: 2026-09-23T17:32:35Z
depth: standard
files_reviewed: 31
files_reviewed_list:
  - CLAUDE.md
  - Taskfile.yaml
  - charts/engram/templates/_helpers.tpl
  - charts/engram/values.yaml
  - docs-site/src/content/docs/guides/configure.md
  - docs-site/src/content/docs/guides/deploy.md
  - internal/config/config.go
  - internal/config/decisions_config_test.go
  - internal/config/decisions_docs_test.go
  - internal/config/registry.go
  - internal/config/validate.go
  - internal/decide/decide.go
  - internal/decide/decide_test.go
  - internal/decide/errors.go
  - internal/decide/jev/classify.go
  - internal/decide/jev/classify_test.go
  - internal/decide/jev/fixtures_test.go
  - internal/decide/jev/jev.go
  - internal/decide/jev/jev_test.go
  - internal/decide/jev/live_test.go
  - internal/decide/jev/telemetry_test.go
  - internal/decide/jev/validate_test.go
  - internal/decide/jev/wire.go
  - internal/decide/jev/wire_test.go
  - internal/decide/many.go
  - internal/decide/many_test.go
  - internal/decide/validate.go
  - internal/server/decider.go
  - internal/server/decider_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-09-23T17:32:35Z
**Depth:** standard
**Files Reviewed:** 31
**Status:** issues_found

## Summary

This phase adds `internal/decide` (a provider-neutral typed-decision contract) and
`internal/decide/jev` (a hand-written net/http client for OpenRouter's Decisions API),
wires it into `internal/server` behind `ENGRAM_DECISIONS_PROVIDER` (off by default,
DEC-01), and threads the new `ENGRAM_DECISIONS_*` config surface through
`internal/config`, the Helm chart, and docs.

This is an unusually well-defended implementation: every claim in the doc comments
(D-01 off-by-default, D-03 no base-URL fallback, D-09 pre-network validation, D-11 the
single jittered retry, D-12 status classification, D-13 span/log telemetry, E03 no
renormalization, E10 omit-not-zero for optional fields) has a corresponding test that
would fail if the claim were violated, and `go build`/`go vet`/`go test` are all clean
across the reviewed packages. I traced the full request/response lifecycle
(`jev.go` → `wire.go` → `classify.go` → `internal/decide/errors.go` → `internal/server/decider.go`),
the config → validation → registry → Helm chart → docs pipeline, and the retry/timeout
budget math (including the "not enough time left to retry" and "budget expired mid-wait"
edge cases) line by line against their tests. No bugs were found that risk incorrect
behavior, data loss, or a security exposure. The confirmed items below are two doc/config
completeness nits and one real (but narrow, currently unreachable) correctness gap in
answer-type trust that is worth closing before Phase 4 builds a consumer on top of it.

Explicitly verified against the review brief:
- **Bounded reads/drain + timeout budget + single jittered retry** — `jev.go`'s `attempt`
  bounds the error body to `maxErrorBodyBytes` (4096) and the success body to
  `c.maxResponseBytes+1` via `io.LimitReader`, always drains the remainder through
  `httpdrain.Drain` (itself double-bounded by bytes and time) before returning, and
  `Decide` computes exactly one budget via `context.WithTimeout` that covers both the
  first attempt and D-11's single jittered retry — verified against
  `TestJevRetryAndBounds`'s `short_budget_times_out_no_retry`, `budget_exhausted_skips_retry`
  and `large_error_body_bounded_and_drained` subtests, all of which pass.
- **Status classification for both OpenRouter (numeric `code`) and LiteLLM (string `code`)
  error bodies** — `classify.go`'s `errorEnvelope.Error.Code` is `json.RawMessage`
  specifically so both dialects unmarshal without error, and classification is by HTTP
  status alone, never by decoding `code` for control flow — confirmed by
  `TestJevErrorClassification`'s `litellm401_string_code`/`litellm403_synthetic` cases and
  the `synthetic400_string_code_never_reaches_control_flow` negative case.
- **No record content or secrets in errors, spans, or logs** — `TestJevTelemetryCarriesNoContent`
  drives sentinel values through `State`, `Instructions`, `WhenTrue` and the API key on both
  a success and a failure call, then scans every span attribute, span event, slog line and
  `err.Error()` string for the sentinels. `logDeciderEnabled` logs only the base URL's
  **host** (never userinfo/path/query) and which env var supplied the key, never the key
  value — confirmed by `TestDeciderEnabledLogLine`.
- **`provider=""` constructs nothing (DEC-01)** — `deciderFromConfig`'s `case "":` returns
  `(nil, nil)` before touching `cfg.Decisions.APIKey`/`BaseURL`/anything else; confirmed live
  by `TestDeciderFromConfigProviderUnset` (every other decisions field populated, still nil,
  zero requests reach the test server) and `TestBuildDepsFromEnvConstructsDecider`'s sibling
  proving construction itself makes no network call.
- **Helm `secretKeyRef` handling** — `decisions.apiKeySecret` renders as a `secretKeyRef`
  (never a literal value) only when `.name` is set, nested inside the `provider` gate so a
  default install renders zero `ENGRAM_DECISIONS_*` vars; `Taskfile.yaml`'s `chart:validate`
  asserts this in both directions (unset-provider-with-every-other-field-set still renders
  nothing; key renders as `secretKeyRef` with the right name/key) and a pinned checksum
  catches any future drift in the block. I recomputed the pinned checksum against the
  current `_helpers.tpl` and it matches exactly.

## Warnings

### WR-01: decodeResponse trusts the wire answer's own Type over the requested question's Type

**File:** `internal/decide/jev/wire.go:109-136`
**Issue:** `decodeResponse` iterates `req.Questions` (correctly limiting the answer set to
what was asked), but for each name it switches on `wr.Answers[name].Type` — the type the
*response* claims — without ever checking it against `req.Questions[name].Type`, the type
the *request* actually asked for. If a provider (a LiteLLM pass-through misconfiguration, a
future Jev dialect change, or a malformed/adversarial gateway sitting in front of the
Decisions endpoint) answers a `noul` question with a `choice`-shaped payload under the same
key, `decodeResponse` happily decodes it as a `decide.Answer{Type: QuestionChoice, ...}` and
returns `nil` error — there is no "answer type does not match requested question type"
branch into `ErrDecisionMalformedResponse`, even though D-12's whole design intent is that
every classifiable wire anomaly gets a named class rather than passing through silently.
Concretely: a caller that asked a `noul` question and reads `resp.Answers[name].Probability`
without first checking `.Type` gets a silent `0.0` (Go's zero value) when the provider
actually answered with a `choice` or `score` shape — indistinguishable from a legitimate
`P(true)=0` answer. Today this is latent (no caller exists yet — Phase 4's `search_memory`
reranker is the first consumer, per `tools.go`'s `decider` field doc comment), but it is
exactly the kind of gap that should close before a consumer is built on top of it, per the
same D-12 philosophy the rest of the package already lives by.
**Fix:** In the `for name := range req.Questions` loop, compare `wa.Type` against
`string(req.Questions[name].Type)` before (or as part of) the type switch, and return
`&decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: name}` on mismatch —
mirroring the existing "missing answer for a requested question" and "unknown answer type"
error paths immediately above/below it:
```go
for name := range req.Questions {
    wa, ok := wr.Answers[name]
    if !ok {
        return decide.Response{}, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: name}
    }
    if wa.Type != string(req.Questions[name].Type) {
        return decide.Response{}, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: name}
    }
    switch wa.Type {
    ...
```

## Info

### IN-01: Helm values table (deploy.md) omits decisions.apiKeySecret and drain rows

**File:** `docs-site/src/content/docs/guides/deploy.md:38-42`
**Issue:** The new Helm-values-to-env table lists `provider`, `baseURL`, `model`, `timeout`
and `concurrency`, but not `memory.decisions.apiKeySecret` (covered only in a prose
paragraph right after the table, mirroring `memory.openai.apiKeySecret`'s existing
precedent) or `memory.decisions.drainBytes`/`drainTimeout` (which are not exposed as Helm
values at all — only reachable via the registry's env-var defaults or a raw `extraEnv`
override, unlike `timeout`/`concurrency`, which get dedicated Helm values). This is
consistent with how `openai.apiKeySecret` is already handled, so it's not a regression, but
an operator scanning only the table (not the prose) for "what does `memory.decisions.*`
control" could miss that the key is Helm-configurable at all, or that drain tuning exists
only as a raw env var.
**Fix:** Either add an `apiKeySecret` row to the table (pointing at the prose, same
treatment `clientSecret`/`chatApiKeySecret` could arguably use too — out of scope here), or
leave as-is since it matches existing convention; no action required unless the project
wants to tighten this table's completeness bar generally.

### IN-02: maxErrorBodyBytes truncation can split a multi-byte UTF-8 rune

**File:** `internal/decide/jev/classify.go:54-56`
**Issue:** `classifyStatus` truncates `detail` with `detail = detail[:maxErrorBodyBytes]`,
a byte-index slice with no rune-boundary awareness. If a non-ASCII character straddles the
4096-byte cut point (plausible for an internationalized upstream error message, e.g. from a
LiteLLM deployment with translated error text), the resulting `Detail` string ends with an
invalid UTF-8 sequence. This is low-impact (the same pattern is used verbatim by
`internal/embed`/`internal/summarize` per this file's own doc comment, so it is not a
regression introduced by this phase, and `Detail` is only ever surfaced in error text/logs,
never re-parsed), but it is a latent quality issue worth flagging since it did travel into a
new package rather than being fixed at the point of introduction.
**Fix:** If ever revisited project-wide, truncate on a rune boundary, e.g. via
`strings.ToValidUTF8` after slicing, or scan backward from `maxErrorBodyBytes` for the last
complete rune start. Not required for this phase given the explicit copy-verbatim precedent.

---

_Reviewed: 2026-09-23T17:32:35Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
