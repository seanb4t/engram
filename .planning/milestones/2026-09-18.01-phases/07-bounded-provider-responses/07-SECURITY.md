---
phase: "7"
slug: "bounded-provider-responses"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-22"
---

# Phase 7 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Written retroactively on 2026-09-22 (secure-phase State B): the verify:post hook did not dispatch
during execution. The register was authored at plan time across all five plans' `<threat_model>`
blocks (`register_authored_at_plan_time: true`), ASVS L1 is configured, and no threat is open, so
per the short-circuit rule each mitigation was verified at L1 depth (grep of the shipped tree at
`af432014`) and by re-running the phase's named tests, all green. No SUMMARY carries a
`## Threat Flags` section.

Plan 07-05's register assumed the red-evidence harness. That harness was removed in `c1afd6c1`
under rule `3p0zsqrhmb` (no tests for tests), so the five harness-shaped threats are
re-dispositioned below with the rationale recorded, not silently dropped.

---

## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| provider response → engram goroutine | Body size and arrival rate are both the provider's to choose, on the embed and summarize lanes |
| provider error body → operator log | A non-2xx body is surfaced in an error the server logs |
| record content → summarize request → provider | Untrusted record content can be reflected back into the error path |
| environment → process configuration | Every `ENGRAM_` value is operator text parsed and range-checked at load |
| configuration → client construction | A bound parsed but never passed to its client is indistinguishable from an enforced one |
| published documentation → operator behavior | A changed meaning that is not written down is a silent break |
| test harness → CI wall clock | A deliberately slow test server holds a CI worker as long as it keeps writing |

---

## Threat Register

| Threat ID | Category | Severity | Disposition | Status | Evidence |
|-----------|----------|----------|-------------|--------|----------|
| T-07-01-01 | DoS | high | mitigate | CLOSED | `httpdrain.Drain(resp.Body` ×2 in `internal/embed/embed.go`; `internal/httpdrain/httpdrain.go` uses `io.LimitReader` + `time.AfterFunc`; `TestEmbedDrainBoundedByBytes`, `TestEmbedDrainBoundedByTimeUnderZeroRequestTimeout` green |
| T-07-01-02 | DoS | high | mitigate | CLOSED | `embed.go:243-254` resolves a non-positive timeout to `c.maxTimeout`; `TestEmbedTimeoutCeiling` green |
| T-07-01-03 | DoS | high | mitigate | CLOSED | `httpdrain.go`: zero `go` statements, one `defer t.Stop()` |
| T-07-01-04 | Tampering | high | mitigate | CLOSED | `TestEmbedDrainOptionsHonorZero` green |
| T-07-01-05 | Info Disclosure | medium | mitigate | CLOSED | `maxErrorBodyBytes` bound in `embed.go`; `TestEmbedNon2xxErrorBodyTruncated` green |
| T-07-01-06 | DoS | medium | accept | CLOSED | Accepted risks log AR-1 |
| T-07-01-07 | DoS | medium | mitigate | CLOSED | `internal/testhttp/trickle.go:45` returns on `r.Context().Done()`; handler bounded by chunks × pause |
| T-07-01-08 | Repudiation | low | accept | CLOSED | Accepted risks log AR-2 |
| T-07-01-SC | Tampering | low | accept | CLOSED | Accepted risks log AR-3 |
| T-07-02-01 | DoS | high | mitigate | CLOSED | `internal/config/validate.go:107-109` rejects a non-parseable or non-positive `ENGRAM_EMBED_MAX_TIMEOUT` (summarize twin alongside); `TestValidateProviderBounds` green |
| T-07-02-02 | Tampering | high | mitigate | CLOSED | Byte bounds parsed by the shared exported `config.ParseNonNegativeIntCap` on both validation and runtime sides |
| T-07-02-03 | Tampering | medium | mitigate | CLOSED | All six keys validated at load; `TestProviderBoundRegistryEntries` green |
| T-07-02-04 | Info Disclosure | low | mitigate | CLOSED | Rejections quote the value with `%q` and name the variable (`validate.go:107-109`); no secret-bearing field |
| T-07-02-05 | Repudiation | low | accept | CLOSED | Accepted risks log AR-4 |
| T-07-02-06 | DoS | medium | accept | CLOSED | Accepted risks log AR-1 |
| T-07-02-SC | Tampering | low | accept | CLOSED | Accepted risks log AR-3 |
| T-07-03-01 | DoS | high | mitigate | CLOSED | `httpdrain.Drain(resp.Body` ×2 in `internal/summarize/summarize.go`; `TestSummarizeDrainBoundedByBytes`, `TestSummarizeDrainBoundedByTimeUnderZeroRequestTimeout` green |
| T-07-03-02 | DoS | high | mitigate | CLOSED | Summarize ceiling clamp in `New`; `TestSummarizeTimeoutCeiling` green |
| T-07-03-03 | Tampering | high | mitigate | CLOSED | One shared `internal/httpdrain`; zero `io.Discard` drains left in either lane |
| T-07-03-04 | Info Disclosure | medium | mitigate | CLOSED | `summarize.go:49` `const maxErrorBodyBytes = 4096`, used at `:301`; `TestSummarizeNon200ErrorBodyTruncated` green |
| T-07-03-05 | Tampering | high | mitigate | CLOSED | `TestSummarizeDrainOptionsHonorZero` green |
| T-07-03-06 | DoS | medium | accept | CLOSED | Accepted risks log AR-1 |
| T-07-03-07 | Repudiation | low | accept | CLOSED | Accepted risks log AR-2 |
| T-07-03-SC | Tampering | low | accept | CLOSED | Accepted risks log AR-3 |
| T-07-04-01 | Tampering | high | mitigate | CLOSED | `TestProviderBoundOptionsWiredIntoBothClients` (go/parser walk of the real constructors) green |
| T-07-04-02 | Tampering | high | mitigate | CLOSED | `config.ParseNonNegativeIntCap(` in `internal/server/tools.go` (3 sites incl. the pre-existing summary-bytes one) |
| T-07-04-03 | DoS | high | mitigate | CLOSED | `TestProviderBoundHelpersParseAndDefault` asserts zero/negative fall back to the default |
| T-07-04-04 | Repudiation | high | mitigate | CLOSED | `guides/upgrade.md:471` `### 18.` plus the `§18` act-on-this row at `:40` |
| T-07-04-05 | Info Disclosure | low | accept | CLOSED | Accepted risks log AR-5 |
| T-07-04-06 | Tampering | medium | mitigate | CLOSED | `docs-site/**` excluded in `.licenserc.yaml`; `upgrade.md` line 1 is `---` |
| T-07-04-SC | Tampering | low | accept | CLOSED | Accepted risks log AR-3 |
| T-07-05-01 | Tampering | high | mitigate (re-pointed) | CLOSED | Mitigation re-pointed from the removed red-evidence harness to the behavioural regression tests above: removing either drain axis, the ceiling clamp or the zero-honoring defaults turns a named behaviour test RED. See AR-6 |
| T-07-05-02 | Spoofing | high | n/a | CLOSED | Component removed (`c1afd6c1`): no registered patches remain to spoof a proof |
| T-07-05-03 | Tampering | high | n/a | CLOSED | Component removed (`c1afd6c1`): no earlier-phase patches remain to go stale |
| T-07-05-04 | Repudiation | high | mitigate | CLOSED | Milestone audit 3-source cross-reference: REQ-provider-drain-bounded and REQ-provider-error-body-closed are `[x]`, in `07-VERIFICATION.md`, and in `07-05-SUMMARY.md` |
| T-07-05-05 | Repudiation | medium | mitigate | CLOSED | #347 verified CLOSED against GitHub (2026-09-21T18:57:27Z), recorded in `07-05-SUMMARY.md`. #457 stays OPEN until the ship PR closes it — tracked as milestone-audit tech debt |
| T-07-05-06 | DoS | medium | n/a | CLOSED | Component removed (`c1afd6c1`): the harness timeout no longer exists |
| T-07-05-07 | Tampering | medium | n/a | CLOSED | Component removed (`c1afd6c1`): no patch authoring remains |
| T-07-05-SC | Tampering | low | accept | CLOSED | Accepted risks log AR-3 |

*Status: open · closed. Disposition: mitigate · accept · transfer · n/a (component removed).*

---

## Accepted Risks Log

| ID | Threats | Rationale |
|----|---------|-----------|
| AR-1 | T-07-01-06, T-07-02-06, T-07-03-06 | An absurdly large ceiling is effectively unbounded. Accepted at decision time (D-08): the knob forces an operator to write a visible number, and the doc comments state the limitation rather than overselling the control |
| AR-2 | T-07-01-08, T-07-03-07 | An abandoned drain costs one TCP handshake and is the correct outcome; a log line per abandonment is hot-path noise with no operator action |
| AR-3 | T-07-0x-SC | No package-manager install or manifest change; `internal/httpdrain` depends only on the standard library |
| AR-4 | T-07-02-05 | The summarize trio is skipped when no summary model is set, mirroring `summarize.timeout`'s existing gate; both sides are asserted in the table |
| AR-5 | T-07-04-05 | The warn log echoes transport tuning numbers, not credentials, matching the two existing timeout helpers |
| AR-6 | T-07-05-01 | User-blessed rule `3p0zsqrhmb` forbids tests for tests. A later refactor removing a bound is caught by the behavioural tests that assert the bound, which is what the rule requires; the meta-harness added no protection a behaviour test does not already give |

---

## Security Audit Trail

## Security Audit 2026-09-22

| Metric | Count |
|--------|-------|
| Threats found | 39 |
| Closed | 39 |
| Open | 0 |

---

## Sign-Off

- [x] Every plan-time threat has a disposition and evidence
- [x] Accepted risks documented
- [x] `threats_open: 0`

**Approval:** verified 2026-09-22 (retroactive, State B, ASVS L1)
