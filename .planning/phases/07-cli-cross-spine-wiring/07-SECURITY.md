---
phase: 7
slug: cli-cross-spine-wiring
status: complete
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-02
---

# v0.12.x Phase 7 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Retroactive audit — the phase shipped before `/gsd-secure-phase` was run for this milestone.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Shell → CLI guard | `--scope` / `--cross-spine` arrive from an operator, agent or CI step | Recall scope selection |
| CLI → engram server | The client's guard runs *before* dialing; the server's `effectiveSearchScope` is the authoritative gate | Scope selection + `cross_spine` boolean |
| CLI → terminal | The coverage footer is written to stdout on cross-spine calls | A **count** of scopes searched — never scope names |

**Framing note.** The client-side guard is a pre-flight ergonomic check, not access control. The
server remains the authority; nothing here grants or withholds access.

---

## Threat Register

Authored at plan time across `07-01`/`07-02`/`07-03-PLAN.md`. Verified retroactively by
`gsd-security-auditor` on 2026-08-02, which independently re-ran every cited test.

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-07-01 | Tampering (guard drift) | `validateScopeCrossSpine` vs server | low | mitigate | The client guard (`cmd/engram/client_common.go:234-242`) independently reimplements the rule, with compile-linked parity against `internal/server.EffectiveSearchScope` (`internal/server/tools.go:1388-1390`) — an additive wrapper over the same `effectiveSearchScope` that gates both `SearchMemories` and `ListMemories` (`connectapi.go:196,266`). `TestValidateScopeCrossSpineParity` asserts the one-directional invariant: **the client never accepts what the server would reject.** Duplication is deliberate — a shared package would violate the client import boundary enforced by `TestClientFilesImportBoundary` | closed |
| T-07-02 | Information Disclosure | `renderCoverageFooter` (search) | low | mitigate | The footer emits only `len(searchedScopes)` (`client_common.go:263-273`), never the slice contents — so it cannot leak the names of scopes a reader might not otherwise know exist. `TestClientSearchCrossSpineEndToEnd` PASS | closed |
| T-07-03 | Denial of Service | guard fallthrough widening a query | low | mitigate | The guard's domain is 4 rows (scope × cross-spine, each set/unset); it has exactly 2 explicit rejection branches and returns `nil` for the 2 valid rows, so no unhandled combination can fall through to a silently-widened query. `TestValidateScopeCrossSpineParity` is 4-row and count-asserted, so a new row cannot be added without failing the test | closed |
| T-07-04 | Tampering | a second guard copy for `list` | low | mitigate | Exactly one `func validateScopeCrossSpine(` exists in `cmd/engram/*.go`; `listCmd.RunE` calls the shared function (`client_list.go:38`) rather than duplicating the rule | closed |
| T-07-05 | Information Disclosure | `renderCoverageFooter` (list) | low | mitigate | `listCmd` calls the same shared footer function (`client_list.go:85`), so the count-not-names property holds identically on both verbs. `TestClientListCrossSpineEndToEnd` PASS | closed |
| T-07-06 | Denial of Service | guard ordering — dial before reject | low | mitigate | The guard call (`client_list.go:38`) precedes `resolveOutputFormat` and `clientFromFlags` (`:41,45`), so an under-specified recall is rejected **before any dialing**. `TestClientListMissingScopeIsUsageErrorBeforeDialing` and `TestClientListScopeWithCrossSpineIsUsageErrorBeforeDialing` both PASS with a zero-RPC-call assertion | closed |
| T-07-07 | Information Disclosure | docs overstating footer content | low | mitigate | `guides/cli.md:84-86` states the footer "reports a **count** … never the scope names," matching the implementation exactly — so an operator cannot be misled into treating the footer as a scope enumeration | closed |
| T-07-08 | Repudiation | docs mischaracterising the guard as authz | low | mitigate | `cli.md:53-60` frames the rejection as a CLI-side pre-flight returning exit 2, explicitly contrasts it with server behavior, and never claims access control | closed |
| T-07-SC | Tampering (supply chain) | dependency surface | low | accept | See Accepted Risks Log AR-07-01 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-07-01 | T-07-SC | The phase adds no dependency; the risk accepted is the general one that any change *could*. Verified rather than asserted: `git diff --exit-code b4544d47 -- go.mod go.sum` is clean. Declared identically in all three plans; collapsed here to one phase-wide row | Phase 7 plan | 2026-08-02 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-02 | 9 | 9 | 0 | gsd-security-auditor (retroactive, ASVS L1, block_on high) |

**Verdict: SECURED.** Every threat in this register is `low` severity — the phase adds a client-side
convenience guard over an already-authorized server API and introduces no new trust boundary. The
security-relevant property is the one-directional divergence: the CLI is deliberately **stricter**
than the server (it rejects `--scope` together with `--cross-spine`, which the server would accept
while silently discarding the scope). Stricter-than-server is safe; the audit confirmed the
divergence runs only in that direction.

Register origin: `register_authored_at_plan_time: true`.

All three SUMMARYs explicitly state "None" under `## Threat Flags`, and an independent inspection of
the phase commit range (`ab133d77..d6f00970`) confirmed no file outside the register's cited surface
was touched.

**Scope note.** An `internal/` containment diff against the phase base `b4544d47` on the *current*
tree also shows `internal/e2e/cli_exitcode_test.go`, `internal/server/schemarequired_test.go` and
`internal/store/store_test.go`. Those trace to later, unrelated commits on this branch — the
retroactive Nyquist audits of phases 2, 3 and 4 run on 2026-08-02 — and are not part of phase 7's
changeset. Recorded so a future reader does not read them as containment-gate violations.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
