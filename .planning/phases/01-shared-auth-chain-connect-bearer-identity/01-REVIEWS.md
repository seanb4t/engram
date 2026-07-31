<!--
SPDX-License-Identifier: Apache-2.0
Copyright 2026 Sean Brandt
-->
---
phase: 1
milestone: v0.12.x
reviewers: [codex]
reviewers_requested: [codex, opencode]
reviewers_dropped:
  opencode:
    model: openrouter/moonshotai/kimi-k3
    reason: no_response
    detail: >-
      Four attempts, all exit 124 (timeout kill) with zero bytes on stdout AND stderr:
      1800s backgrounded (153KB argv prompt), 2700s backgrounded (4.5KB lean prompt, --pure),
      540s foreground (4.5KB lean prompt), 400s on the alternate opencode/kimi-k3 route.
      A 90s trivial control prompt ("Reply with exactly: CONTROL-OK") ALSO timed out on the
      same route that had answered an identical-shaped smoke test in seconds earlier in the
      session. Ruled out by test: prompt size (4.5KB fails), argv transport, backgrounding
      (foreground fails identically), the broken claude-mem.js plugin (--pure fails), model
      route (opencode/kimi-k3 fails too), and local SQLite state (PRAGMA quick_check = ok;
      opencode.db untouched since 13:01 while failing runs ran at 15:26+, so they block
      before reaching it). Remaining hypothesis: the provider endpoint stopped responding
      mid-session. NOT retried further to avoid unbounded wall-clock.
reviewed_at: 2026-07-31
plans_reviewed: [01-01-PLAN.md, 01-02-PLAN.md, 01-03-PLAN.md, 01-04-PLAN.md]
verdict: HIGH — revise before executing
---

# Cross-AI Plan Review — v0.12.x Phase 1: Shared Auth Chain & Connect Bearer Identity

> **Single-reviewer review.** `--opencode` was explicitly requested and could not be run (see
> `reviewers_dropped` above). Per the review workflow, an explicitly-named lane that cannot run is
> an error, not a silent reduction in scope — this review is recorded as one-eyed. There is no
> Consensus section below, because consensus across one reviewer is not consensus. Re-run
> `/gsd-review --phase 1 --opencode` when the provider recovers to add a second opinion.

## Codex Review

**Model:** codex-cli 0.146.0 · **Sandbox:** read-only · **Repo access:** yes (no `REVIEWED-WITHOUT-REPO-ACCESS` marker) · **Tokens:** ~169.6k

## Summary

The core design is defensible: provenance is server-set, bearer verification is exclusive, unknown lanes fail closed, expiry is centralized, and the existing mount gate remains intact. However, the plans are not ready to execute. They contain two central test defects, an incomplete resolver-signature migration that will break compilation, and a wiring rule that withholds Connect bearer authentication from UI-enabled deployments unless `connect.headless` is also set—contradicting the phase’s unqualified bearer-parity criterion. Overall, the architecture can achieve the goal, but the current plans cannot reliably prove or deliver it.

## Strengths

- The intended CSRF data flow is sound if implemented exactly: resolver success returns `TokenInfo + Lane`; the subject interceptor stamps both before dispatch; CSRF reads the context lane after the write-procedure gate. The existing order is subject → CSRF → validation → reseal at `internal/server/connectapi.go:376-394`, and current CSRF request reads occur only after the write gate at `internal/server/connectcsrf.go:61-85`.

- No cookie session can obtain `LaneBearer` merely through cookie presence, content type, method, header casing, or a garbage credential. `Authorization` intentionally selects the candidate lane, but `LaneBearer` is returned only after verifier success; bearer failure returns an error and never consults cookies. A request carrying both a valid cookie and a valid bearer becomes bearer-authenticated by design. The current cookie resolver itself only examines the sealed session cookie and independently rejects missing, invalid, expired, legacy, or empty-owner sessions at `internal/webauth/resolver.go:37-67`.

- The planned parser is deterministic and matches the pinned SDK’s basic rule: `strings.Fields`, exactly two fields, case-insensitive `Bearer`. Tabs, repeated spaces, leading/trailing whitespace, and Unicode whitespace are separators; extra fields do not qualify. The SDK applies the same structure before calling the verifier at `github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go:99-108`.

- Expiry centralization is compatible with production token issuers. OIDC `TokenInfo` receives `idt.Expiry` at `internal/auth/auth.go:262-266`; static tokens receive a future sentinel at `internal/auth/static_token.go:16-22,77-80`; the zero-expiration cookie `TokenInfo` at `internal/webauth/resolver.go:67` never passes through the bearer verifier. The existing `serve_test.go` paths therefore do not break: no-auth never constructs a verifier, the static-token test uses the sentinel, and the human-only test sends no token (`cmd/engram/serve_test.go:101-117,137-181,184-200`).

- The mount-regression shape is strong. Today Connect is registered only when `resolve != nil` at `internal/server/connectapi.go:362-365`, while `serve.go` creates that resolver only inside the UI-enabled block at `cmd/engram/serve.go:139-175`. Keeping the gate unchanged and asserting `connectResolverFor(chain, false, nil) == nil` prevents a configured service-auth lane from silently exposing Connect.

- Wave 2 has no production-file collision: 01-02 owns reseal and bearer-parity tests; 01-03 owns config and `serve.go`. Nothing in 01-02 requires Plan 04. Plan 04’s documentation can depend only on 01-03. The dependency model is therefore valid once Plan 01’s missing test-file migration is added.

## Concerns

- **HIGH** — Changing `connectResolver` from two returns to three without migrating all existing call sites will make Wave 1 fail to compile. Existing two-return resolvers appear in `internal/server/connectauth_test.go:42-44,68-70`, `internal/server/connectcsrf_test.go:48-54,196`, `internal/server/connectapi_cookie_test.go:44-58,101-104`, `internal/server/connectapi_negative_test.go:76-90`, `internal/server/connectreseal_wire_test.go:180-186`, and `internal/server/connectapi_test.go:795-796`. None of those files is owned by 01-01. Go cannot pass those functions to the new three-return `connectResolver`.

- **HIGH** — `TestCSRFCookieCallerCannotSelfDeclareBearerLane` cannot construct the request the plan claims. The mandated reused `csrfHeaders` contains only actor, CSRF cookie, and CSRF header fields, and `doCSRFWrite` sets only those headers (`internal/server/connectcsrf_test.go:56-80`). The plan requires using that helper verbatim while asserting that a garbage `Authorization` header is present (`01-01-PLAN.md:360,370-374,391-396`). As written, the phase’s central CSRF-forgery test would either be impossible to write or would pass without sending the attack input.

- **HIGH** — Plan 03 incorrectly treats `connect.headless` as both a mount switch and a bearer-auth enablement switch. It selects the bearer half only when `headless == true` (`01-03-PLAN.md:374-378`). Therefore an existing deployment with UI enabled, a configured MCP verifier, and `connect.headless=false` remains cookie-only. A token accepted by MCP is rejected by Connect, violating success criterion 1 and D-06. Current source confirms UI already supplies the Connect surface independently at `cmd/engram/serve.go:143-178`; enabling bearer there would not expose a new route.

- **MEDIUM** — The “single parse, single decision” guarantee is contradicted by the planned implementation. `NewConnectResolver` parses once to choose the bearer arm (`01-01-PLAN.md:264-270`), but `newConnectBearerResolver` then re-reads and re-parses `req.Header()` (`01-01-PLAN.md:243-249`). Normal HTTP headers will not mutate between synchronous calls, so this is not presently a remote bypass, but it violates the stated load-bearing structural guarantee and creates a TOCTOU/drift seam.

- **MEDIUM** — The confused-deputy boundary is deterministic but semantically debatable. `Authorization: Bearer a b`, bare `Bearer`, and comma-coalesced duplicate values are recognizable bearer attempts yet fall through to cookie when both lanes exist. That remains CSRF-protected, so it is not a direct bypass, but it contradicts the stronger interpretation “once the Bearer scheme is declared, fail closed.” Duplicate headers, comma-coalescing, tabs, Unicode whitespace, leading/trailing whitespace, and invalid UTF-8 are not covered by the proposed test matrix.

- **MEDIUM** — The fail-first substitution is too narrow and currently amounts to manual mutation theater. It proves only that one test detects one temporary header-presence branch, then removes the evidence from executable code (`01-01-PLAN.md:421-427`). It would not catch provenance being derived incorrectly upstream, exemption based on cookie absence/content type/method, or later erosion of test sensitivity. Moreover, the helper defect above means the mutation may not fail at all. The genuine red-green test is `TestBearerLaneExemptFromCSRF`: today every bearer-stamped write without CSRF fails at `internal/server/connectcsrf.go:65-85`; the correct exemption makes it pass.

- **MEDIUM** — `EnforceExpiry` does not replicate the SDK completely. The SDK rejects `(nil, nil)` before dereferencing `TokenInfo` at `github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go:118-120`; the proposed decorator immediately reads `ti.Expiration`, causing a panic. Current production verifiers appear not to return `(nil, nil)`, but a verifier regression would become a request-level denial of service instead of a controlled rejection.

- **MEDIUM** — The planned `errors.Join(mcpauth.ErrInvalidToken, ErrTokenExpired)` changes the MCP wire body. The SDK returns `err.Error()` when the verifier reports `ErrInvalidToken`, before reaching its own fixed expiry messages (`go-sdk/auth/auth.go:107-137`). Thus MCP would likely emit `"invalid token\ntoken expired"` rather than today’s `"token expired"`, contradicting the plan’s claim that the doubled enforcement produces an identically shaped 401 (`01-01-PLAN.md:240-241`).

- **MEDIUM** — Several planned tests are infeasible or self-defeating against current fixtures. `stubOIDCVerifier` returns a zero-expiration `TokenInfo` at `internal/server/connectapi_service_auth_parity_test.go:43-52`, yet Plan 02 says to reuse it verbatim under `EnforceExpiry`; its happy parity tests will reject. Separately, `buildAuthChain` only accepts configuration and constructs real OIDC/static verifiers (`cmd/engram/serve.go:297-340`), so `TestBuildAuthChainWrapsWithExpiry` cannot make it return a past-expiration `TokenInfo` without an unplanned injection seam.

- **MEDIUM** — The official Helm chart cannot enable the new feature. It exposes UI configuration at `charts/engram/values.yaml:136-151` and renders `ENGRAM_UI_ENABLED` at `charts/engram/templates/_helpers.tpl:74-80`, but has no generic extra-env path or `connect.headless` value. Deferring this to an issue means a Helm operator cannot satisfy `REQ-connect-headless-mount` through the supported chart.

- **LOW** — `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag` duplicates an existing test almost exactly: `TestMountConnectSkipsWhenResolverNil` already asserts the Connect path returns 404 at `internal/server/connectapi_test.go:776-789`. The valuable new coverage is the upstream non-nil-chain/flag-false case, not another direct nil-resolver test.

## Suggestions

- Expand Plan 01 ownership to every existing resolver fixture and migrate each to an explicit lane. Add a compile/build gate immediately after the signature change.

- Extend `csrfHeaders` with an `authorization` field and modify `doCSRFWrite` to set it. Add an end-to-end test using the real `NewConnectResolver(bearer, cookie)` through subject → CSRF, not merely a pre-stamped lane stub.

- Make mounting and bearer inclusion separate decisions:

  - Return nil only when `cookieResolve == nil && !headless`.
  - Require a non-nil chain when `headless`.
  - Whenever Connect is mounted, pass the configured chain as the bearer half, including UI-enabled/headless-false deployments.

- Parse `Authorization` exactly once. Pass the extracted token directly to a verifier helper instead of delegating to a resolver that re-reads the request.

- Decide explicitly whether any case-insensitive `Bearer` scheme commits to bearer even when credential syntax is malformed. Whichever policy is chosen, pin duplicate, comma-coalesced, ASCII/Unicode whitespace, and mixed-credential cases.

- Treat the manual mutation as supplementary evidence. Use `TestBearerLaneExemptFromCSRF` for genuine red-green and retain permanent end-to-end negative tests for self-declaration and failed-bearer fallthrough.

- Add a nil-`TokenInfo` rejection to `EnforceExpiry` and assert exact MCP and Connect status/body behavior. If byte-for-byte MCP errors matter, use an error type whose `Error()` matches the SDK while supporting `errors.Is`.

- Replace the infeasible builder-expiry test with a unit test of a small composition helper or a structural assertion that `buildAuthChain` wraps exactly once. Give parity fixtures explicit future expirations.

- Add `memory.connect.headless`, render `ENGRAM_CONNECT_HEADLESS`, and run `task chart:validate` in this phase.

## Risk Assessment

**HIGH.** The intended security architecture is mostly sound, and the mount default-off property is well designed. But the current plan set cannot compile as scoped, its central CSRF attack test cannot send the claimed attack header, UI-enabled deployments miss the promised bearer behavior, and expiry handling has panic and response-shape gaps. Those are actionable defects in implementation and verification, not stylistic reservations; the plans should be revised before execution.
---
