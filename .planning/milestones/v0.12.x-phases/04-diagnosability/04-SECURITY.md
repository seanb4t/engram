---
phase: 4
slug: diagnosability
status: complete
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-02
---

# v0.12.x Phase 4 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Retroactive audit — the phase shipped before `/gsd-secure-phase` was run for this milestone.

This phase deliberately makes internals **more visible** — authz decision logs, field-attributed
validation errors, provider error bodies. Its threats are therefore predominantly
information-disclosure, and its most important controls are *absence* guarantees.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Cedar PDP → server log | Authz decisions become a debug log line | Policy IDs, an error **count**, decision/action/bucket — never an expression trace |
| Validator → caller | A rejection names the failing field and a hint code | Field identifier + machine-stable hint — never the rejected value |
| Provider → server | Embeddings / chat responses, including error bodies | A **bounded** body prefix |
| Server → operator docs | Published error grammar and log field names | Hint codes, excluded-field list |

---

## Threat Register

Authored at plan time across `04-01`–`04-07-PLAN.md` — **31 rows**, resolving to 20 unique
`T-04-*` IDs (several re-verified at additional mitigation sites in later plans, plus one `T-04-SC`
per plan). All 31 were extracted and verified individually. Collapsed below to unique IDs.

| Threat ID | Category | Severity | Disposition | Mitigation / Evidence | Status |
|-----------|----------|----------|-------------|------------------------|--------|
| T-04-01 | Information Disclosure | high | mitigate | **Absence guarantee, mechanism-verified.** `Decision.diag` is unexported with no getter (`authz.go:62-65`), and the `DecisionLog` struct (`:73-106`) is a positive allowlist — the log call sites (`store.go:733-740,760-766`) can only emit `dl.*` fields, so a Cedar `DiagnosticError.Message` expression trace has no path to the buffer regardless of level. `TestDecisionLogNeverLeaksExpressionTrace` PASS (4 subtests) | closed |
| T-04-02 | Information Disclosure | medium | mitigate | **Absence guarantee.** The `got %q` value tail was removed from the hint path (`argerror.go:58-69`, D-12); zero live occurrences across `internal/server/*.go` (remaining matches are a comment and a test name). Re-verified at three mitigation sites (04-01, 04-04, 04-05). `TestHintNeverEchoesValue` PASS (7 subtests) | closed |
| T-04-03 | Denial of Service | medium | mitigate | **Bounded reads.** `io.LimitReader(resp.Body, maxErrorBodyBytes)` on the error path and `io.LimitReader(resp.Body, c.maxResponseBytes)` on the success path (`embed.go:294,306`); the success bound is sized from `ENGRAM_EMBED_DIM` with a parse-failure fallback that can never yield a zero bound (`tools.go:374-415`). `TestEmbedSuccessDecodeBounded` PASS | closed |
| T-04-04 | Denial of Service | medium | mitigate | Connection reuse preserved: `io.Copy(io.Discard, resp.Body)` after every bounded read on **both** lanes (`embed.go:295,309`; `summarize.go:182,191`). `TestEmbedNon2xxDrainsForReuse`, `TestSummarizeNon200DrainsForReuse` PASS | closed |
| T-04-05 | Information Disclosure | medium | accept | See AR-04-01 | closed |
| T-04-06 | Tampering | medium/high | mitigate | **Scope fence (D-08).** The MCP 401 body is byte-identical after the message reformat — `TestMCP401BodyByteIdentical` does a full-string `==` on both the body and `WWW-Authenticate` (3 subtests) PASS. Documented as explicitly out of scope in `guides/upgrade.md:127-129`, citing that test | closed |
| T-04-07 | Tampering | medium | mitigate | The `ConnectCode()` trio maps validation classes to `{InvalidArgument, FailedPrecondition, OutOfRange}` (`argerror.go:109-120`) — codes the CLI's `exitCodeForConnectErr` already groups, so the exit-code taxonomy cannot drift. `TestArgErrorConnectCodeTrio`, `TestConnectValidationCodeMapping` (12 subtests through real handlers), and an **unmodified** `TestExitCodeForConnectErrTable` all PASS | closed |
| T-04-08 | Tampering | high | mitigate | `TestSchemaRequiredMovedToGoLevel` PASS, 25 subtests against a ≥24 floor — required-ness genuinely moved to Go validation for every relaxed field | closed |
| T-04-09 | Tampering | high | mitigate | `errors.As(err, &ae)` is placed **first** in the `connectError` type switch (`connecterror.go:59-68`), with the ordering hazard documented in-line — so a typed `argError` cannot be shadowed by a broader case and fall through to `CodeInternal` | closed |
| T-04-10 | Information Disclosure | medium | mitigate | The `DecisionLog` struct excludes owner / memoryOwner / scope / category / visibility **by construction**, not by filtering. `TestDecisionLogCarriesOnlyAllowlistedFields` PASS | closed |
| T-04-11 | Denial of Service | low | mitigate | Logging is O(1) per request, not O(results): `decideBucket` runs at most twice per request (own + shared) and `decideRecord` once per id-addressed op (`store.go:714-725,744-752`). No log call sits inside a per-result loop | closed |
| T-04-12 | Tampering | medium | mitigate | The four validation entry points — `validateCitations` (`:834`), `parseWindow` (`:526`), `effectiveSearchScope` (`:1374`), `effectiveDiscoveryScope` (`:1351`) — contain zero bare `fmt.Errorf`/`errors.New`, so no rejection can escape the typed envelope. `TestValidationErrorAttributionMatrix` PASS, 23 subtests against a ≥21 floor | closed |
| T-04-13 | Information Disclosure | medium | mitigate | `validateStoreRule`/`validateRuleSummary` (`rules.go:62-98`) use `argErrf` exclusively. `TestStoreRuleValidationIsNotCodeInternal` PASS (7 subtests calling `connectError` directly) | closed |
| T-04-14 | Tampering | high | mitigate | **D-11a defect closed.** Zero hand-wrapped `connect.NewError(connect.CodeInvalidArgument, …)` remain in `connectapi.go`; only `CodeUnauthenticated` hand-wraps persist, which are out of scope. Validation rejections therefore reach Connect as caller-error codes, never `CodeInternal` | closed |
| T-04-15 | Tampering | high | mitigate | `Shared *bool` (`tools.go:733`) with a nil-only rejection (`:1640-1642`) — an explicit `false` is honored rather than being indistinguishable from unset | closed |
| T-04-16 | Tampering | high | mitigate | `validateUpdateArgs` is called only in the MCP closure (`tools.go:1939`) and is confirmed absent from `deps.updateMemory`'s body, so the Connect field-mask lane's legitimate nil `Content` is not rejected | closed |
| T-04-17 | DoS / Tampering | high | mitigate | `deleteAll`'s scope check is the **first and only** statement before the sole side-effecting `DeleteAll` call (`tools.go:1622-1626`) — no path reaches the destructive call unguarded. `TestDeleteAllRequiresScope` PASS | closed |
| T-04-18 | Tampering | medium | mitigate | The published error grammar matches the code: all 10 hint codes in `reference/errors.md` are set-equal to `argerror.go`'s `HintCode` constants | closed |
| T-04-19 | Information Disclosure | low | mitigate | `guides/configure.md:200-211` names the three **excluded** log fields verbatim (expression trace, error message text, owner/scope), and the documented field names match the actual log call in `store.go` | closed |
| T-04-SC | Tampering (supply chain) | high | accept | See AR-04-02 | closed |

*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-04-01 | T-04-05 | A bounded (4096-byte) prefix of a provider error body reaches the server log. Accepted because the caller is same-actor on that path and the alternative — discarding the body — is exactly the diagnosability defect this phase exists to fix. The residual server-log exposure is explicitly accepted, and unusually the acceptance analysis is written **in the code** at `embed.go:283-293` rather than only in a plan, so a future reader meets it at the site | Phase 4 plan (04-03) | 2026-08-01 |
| AR-04-02 | T-04-SC | No dependency added: `git diff 463aba94..a91e11ab -- go.mod go.sum` is empty across the full phase range (verified once for the whole phase, not per plan) | Phase 4 plan | 2026-08-01 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-02 | 31 rows / 20 unique | 31 | 0 | gsd-security-auditor (retroactive, ASVS L1 with L2/L3 depth on absence guarantees, block_on high) |

**Verdict: SECURED.** Register origin: `register_authored_at_plan_time: true`.

**The three absence guarantees were verified at mechanism level, not test level.** An absence
guarantee ("X never appears in Y") is only genuinely closed if you can point at the structure that
enforces it — a passing test shows it does not happen *today*. Accordingly:

- **No expression trace in the authz log** — closed on the unexported `diag` field plus the
  positive-allowlist `DecisionLog` struct, so the log call sites are structurally incapable of
  emitting it, not merely observed not to.
- **No rejected value echoed in a hint** — closed on the removal of the `got %q` tail plus a
  zero-live-occurrence source check across `internal/server/*.go`.
- **Bounded provider bodies** — closed on the `io.LimitReader` calls themselves, with the success
  bound sized from config and a fallback that cannot produce a zero bound.

**Register count correction.** The orchestrator's dispatch brief stated 24 rows; the true count is
**31** (20 unique IDs, several re-verified at later mitigation sites, plus one `T-04-SC` per plan).
This is the second undercount in this batch from the same cause — a row-count pattern that misses
the per-plan `*-SC` rows. Recorded so the miscount is attributed to the orchestrator's pre-count and
not mistaken for a gap in the plans.

No unregistered threats: the `## Threat Flags` heading is absent from all 7 SUMMARYs (confirmed by
heading extraction, not by an empty-section match), and independent review of the embedder wiring,
the `*bool` visibility toggle, the `delete_all` guard and the Connect handler sweep surfaced no
surface outside the register.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
