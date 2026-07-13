---
phase: 17
slug: wired-write-handlers-full-crud-schedule
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-13
---

# Phase 17 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> State B run: register authored at plan time (all six 17-0N-PLAN.md carried
> `<threat_model>` STRIDE blocks), ASVS L1 → L1 grep/read-depth verification.
> Register built by extracting and deduplicating the `<threat_model>` blocks
> from 17-01 through 17-06 PLAN.md (19 unique threat IDs referenced across the
> six plans), then verifying each declared mitigation against the ACTUAL
> implemented code and tests — not against SUMMARY.md claims or plan intent.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|----------------|
| IdP token claims → owner authz key | Untrusted claim values (`email`, `sub`, `client_id`) cross into the resolved owner string that gates every read/write and is compared directly in store authz filters | owner claim value |
| operator config → claim-selection order | `ENGRAM_OWNER_CLAIM` ordering decides which claim (and whether the email-verified gate) governs a request | ordered claim list |
| pre-upgrade session cookie → post-upgrade owner space | A cookie sealed BEFORE the owner-encoding change carries a bare owner; if forwarded unchanged it re-enters the authz filters as a stale/colliding key | sealed session payload |
| transport (MCP/Connect) → deps.* | The verified caller crosses from the transport into shared business logic as an explicit value (`caller{Subj, Actor}`) | caller identity |
| deps.* → memStore | Business logic crosses into the store; authz enforcement lives beyond this boundary (DEC-cgb) | store operation + subject |
| deps.updateMemory → payload-only store method | A no-re-embed update crosses into the store; must carry an already-ownership-verified `cur`, never a caller-forgeable id | `cur Memory` (post-gate) |
| Connect wire message → internal args | Validated (Phase-15 protovalidate) proto messages cross into internal `*Args`; protoconv is the sole translator | proto request → `*Args` |
| internal result → Connect wire response | The canonical UUID + short_id cross back out; protoconv maps only the fetched-record ids (no leak of an unrelated record) | `mutationResult` → proto response |
| Connect client → by-id write RPC error surface | The error message returned to a browser (network-tab visible) must not disclose another owner's resolved UUID when the caller supplied only a short_id | error message text |
| deps.* error → Connect client (connectError) | Business rejections cross out mapped to precise codes; internal faults must not leak raw text | domain error → Connect code |
| proto contract → transport semantics | A write RPC must not advertise itself as side-effect-free (retry/caching hazard) | idempotency annotation |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-17-01 | Elevation of Privilege | `deps.*` caller threading / six Connect write handlers / MCP↔Connect authz parity | high | mitigate | Identity resolved once via `callerFromTokenInfo` (`internal/server/identity.go:82-95`), threaded explicitly — no write method re-derives it. All six Connect handlers (`internal/server/connectapi.go:254-328`) are thin adapters: `callerFromConnectContext` → protoconv → the SAME `deps.*` method the MCP tool calls → `connectError`; `grep -nE 'a\.d\.st\.' connectapi.go` shows only the documented `ListScopes` exception. `TestWriteParity` (`internal/server/connectapi_write_parity_test.go`, 6 RPC rows + independent per-lane spy fixtures) proves an identical store trace + identical mapped code for both lanes; a separate `go/parser`/`runtime.Caller`-based AST sub-test (`source_delegates_to_named_deps_methods`) proves each handler body textually invokes its named `deps.*` method, closing the shared-store-trace ambiguity (storeMemory/scheduleMemory share MintShortID+Upsert). Verified live: `go test ./internal/server/... -run TestWriteParity -v` passes all six RPC subtests + the AST sub-test | closed |
| T-17-02 | Information Disclosure | by-id write RPC `ErrNotFound` message (existence leak) | high | mitigate | `updateMemory`/`deleteMemory`/`setVisibility` re-wrap `store.ErrNotFound` with the caller's ORIGINAL input (`fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)`, tools.go:972 et al.), never the resolved UUID; `connectError` maps it to `CodeNotFound` without re-wrapping (D-11). `TestCrossOwnerRewrap` (`internal/server/connectapi_crossowner_test.go`) SPLITS short_id-input (message contains the short_id, excludes the resolved UUID) from direct-UUID-input (message echoes exactly the supplied UUID) for all three by-id write RPCs — the round-2 self-contradictory single assertion is fixed. Verified live against real Qdrant (`testDeps(t)`) | closed |
| T-17-03 | Spoofing / Elevation of Privilege | `ClaimIdentity` owner-claim gate (all ordered claims) | high | mitigate | `internal/auth/auth.go:121-164`: for every ordered claim, presence is checked separately from string conversion — a present-but-non-string value (number/object/array/`null`) rejects outright and is NEVER coerced to `""` and fallen through to a later claim (which would select a different authz bucket); the `email_verified` gate rejects a present-but-unverified email outright (D-05). Generalized from email-only (round-3 HIGH-1) to every ordered claim (round-4 HIGH-1). Verified by `TestClaimIdentityD05UnverifiedEmailNeverFallsThrough`, `TestClaimIdentityEmailSubPresenceTable`, `TestClaimIdentitySubClientIDPresenceTable`, `TestClaimIdentitySingleClaimEmailNonStringRejects` — all present and pass (`go test ./internal/auth/...`) | closed |
| T-17-04 | Spoofing / Elevation of Privilege (authz-key collision) | `ClaimIdentity` non-email owner encoding | high | mitigate | `namespacedOwner` (`internal/auth/auth.go:92-94`) produces a provably injective length-prefixed encoding `%d:%s:%d:%s` — no two distinct `(claim,value)` pairs collide, closing the ambiguous `<claim>:<value>` collision the review found (`("sub","x:y")` vs `("sub:x","y")`). `TestNamespacedOwnerInjectivity` and `TestNamespacedOwnerUnicodeInjectivity` (byte-length-prefix pin for multi-byte claim values) both present and pass | closed |
| T-17-05 | Tampering | write RPC idempotency annotation | high | mitigate | The Phase-15 idempotency-ban gate re-asserts green now that real write logic exists: `grep -n 'idempotency_level' proto/engram/v1/engram.proto` returns no match — no write RPC carries `NO_SIDE_EFFECTS`. `task proto:lint`'s grep gate + `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` (per 17-VERIFICATION.md) both confirm | closed |
| T-17-06 | Repudiation | Connect-lane `Memory.Actor` attribution | medium | mitigate | `callerFromTokenInfo` (`internal/server/identity.go:82-95`) falls back `Actor` to the resolved owner (`Subj.Owner()`) when `TokenInfo.UserID` is empty — the Connect cookie lane's TokenInfo carries only `Extra[owner_claim]`, never `UserID` (`internal/webauth/resolver.go:67`), so without this fallback every Connect-attributed `Memory.Actor` would be permanently empty. `TestCallerFromTokenInfoActorFallsBackToOwner` / `TestCallerFromTokenInfoUserIDWins` (`internal/server/identity_test.go`) and the `TestWriteParity/StoreMemory` row (lane-appropriate, non-cross-lane-equal actor assertion, round-4 MED) both present and pass | closed |
| T-17-07 | Elevation of Privilege | read-lane store bypass (Pitfall 1) | high | mitigate | All Connect read handlers with an MCP-side deps.* counterpart (ListMemories, SearchMemories, GetMemory, SearchDiscoveries) are rewired onto the 17-06 typed core `deps.*` methods (caller-threaded); `grep -nE 'a\.d\.st\.' connectapi.go` returns only the documented `ListScopes` exception (no MCP-side `deps.listScopes` counterpart exists). `TestConnectCookieLaneIsolation` stays green (verified live, including under the fail-closed `ENGRAM_REQUIRE_QDRANT=1` gate). No Connect field dropped: `TestConnectListMemoriesLimitZeroReturnsAll`, `TestConnectSearchDiscoveriesDefaultsK20`, `TestConnectSearchDiscoveriesEmptyScopeSpansAll`, `TestConnectGetMemoryEnqueuesUsageSignalExactlyOnce` all present and pass | closed |
| T-17-08 | Spoofing / Elevation of Privilege | reserved-namespace email guard | high | mitigate | `reservedOwnerNamespace` regex `^[0-9]+:` (`internal/auth/auth.go:37,144-146`) rejects a winning email value that would occupy the namespaced-owner grammar, so a crafted email cannot masquerade as a service owner. `TestClaimIdentityReservedNamespaceEmailGuard` present and passes | closed |
| T-17-09 | Elevation of Privilege | payload-only store method (`UpdatePayload`) | high | mitigate | `Store.UpdatePayload` (`internal/store/store.go:1475-1534`) takes an already-ownership-verified `cur` (obtained through `FetchForUpdate`'s gate) and writes by `cur.ID` — the identical gate-once contract as `store.Update`; no new cross-owner mutation path is introduced. `TestUpdatePayloadPreservesVectorBumpsUsageAndClearsProvenance` proves vector preservation (raw-client scroll before/after), usage-signal bump, and provenance key deletion | closed |
| T-17-10 | Denial of Service (valid request silently expired/rejected) | Timestamp → scheduling-window string (adapter boundary) | medium | mitigate | `windowBoundFloor`/`windowBoundCeil` (`internal/server/protoconv.go:140-170`) round scheduling bounds OUTWARD to whole seconds (not_before down, not_after up) before RFC3339Nano formatting, so the store's second-granular `.Unix()` flooring is a no-op — a sub-second `not_after` widens to the containing whole-second window instead of silently collapsing to immediate-expiry. `TestProtoconvNotAfterNearFutureSurvivesStoreFlooring`, `TestProtoconvWindowBoundFloorsAndCeils`, `TestProtoconvWindowBoundOnBoundaryIsUnchanged`, `TestProtoconvWindowBoundOrderingPreserved` all present and pass | closed |
| T-17-11 | Information Disclosure (contract regression) | Connect read field loss + MCP cursor-mode loss | medium | mitigate | The typed core read contract (`coreListRequest`/`coreListResult`/`coreSearchRequest`, tools.go:797-855) is a SUPERSET carrying every Connect field (offset/categories/visibility/exact total/cursor/cursor_mode/tags/window); no `[]any` in the shared path. MCP list closure sets `CursorMode: true` explicitly, preserving unconditional MCP cursor behavior. Regression tests for the split offset-mode/cursor-mode pagination semantics, the removed shared `Limit==0→20` default, and per-lane k defaults (MCP 8 / Connect 20) all present in `tools_test.go`/`connectapi_test.go` and pass | closed |
| T-17-12 | Information Disclosure | `connectError` `CodeInternal` branch | medium | mitigate | `connectError(ctx, err)` (`internal/server/connecterror.go`) maps every typed sentinel (incl. `store.ErrInvalidArgument`-wrapped parseWindow/rule-summary rejections, `store.ErrAmbiguousShortID`, `context.Canceled`/`context.DeadlineExceeded`) to a precise non-Internal code; only genuinely-unexpected errors hit `CodeInternal`, which logs via `slog.ErrorContext` and returns a fixed generic message (`"internal error"`) — never the underlying error text. `TestConnectError`'s `unexpected_error_maps_to_internal_without_leaking` subtest asserts the leaky detail (`"qdrant"`, an IP) does NOT appear in the client-facing message | closed |
| T-17-13 | Spoofing (test-authz drift) | fake-store authz reliance | medium | mitigate | The real Qdrant-backed isolation suite (`TestConnectCookieLaneIsolation` + cross-owner tests) remains part of the phase gate — the spy/fake authz can drift, so it is never the sole gate. Fail-closed in CI (round-6 MED): `requireQdrant()` (`internal/server/tools_test.go:133-143`) is the SOLE parser of `ENGRAM_REQUIRE_QDRANT`, returns a non-nil error on a malformed value (never coerced to false — round-8 LOW), and `TestMain`/`testDepsWithStore` act only on its result, `os.Exit(1)`/`t.Fatal`ing instead of skipping when required. CI sets `ENGRAM_REQUIRE_QDRANT: "1"` (`.github/workflows/ci.yaml:39`). The Qdrant testcontainer image is pinned to `qdrant/qdrant:v1.18.2` (not `latest`). Live-verified: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestConnectCookieLaneIsolation -v` ran the real-Qdrant suite (no skip) and passed | closed |
| T-17-14 | Spoofing / Elevation of Privilege (authz-key migration) | owner-encoding rollout: pre-upgrade session cookies + non-email records | high | mitigate | Three-part mitigation, all verified: (1) the reserved-namespace email guard (T-17-08) blocks a crafted email from occupying the namespace; (2) `SessionCodec.Seal` auto-injects `sessionPayloadVersion` (`internal/webauth/session.go:73-80`) into every minted cookie, and `Resolver.Resolve` rejects any session whose version doesn't match (`internal/webauth/resolver.go:59-63`) with a GENERIC client-facing error (`"invalid session cookie"` — no version disclosed, round-8 LOW) while logging the version internally; (3) the `engram migrate-remap-owner` runbook is documented in `docs-site/src/content/docs/reference/auth.md` with worked encoded-target examples (`3:sub:5:svc-1`, `9:client_id:5:app42`) and a global/non-transactional `--dry-run`+backup warning. `TestResolverRejectsLegacyVersionCookie` (forges a legacy cookie by bypassing `Seal` via the raw `sealBytes` path) and `TestSealAutoInjectsVersion` both present and pass | closed |
| T-17-15 | Spoofing / Elevation of Privilege (documented widening) | empty-string email fall-through under `[email, sub]` | low | accept | See Accepted Risks Log AR-17-01 | closed |
| T-17-INT | Tampering (integrity) | `update_mask` → `updateArgs.Content` mapping | high | mitigate | `updateArgs.Content` is `*string` (tools.go:525); `protoconv.updateMemoryRequestToArgs` (protoconv.go:48-68) populates the pointer ONLY when `"content"` is present in `update_mask.paths` — absent yields nil, never the proto zero value (no silent blanking). `deps.updateMemory` routes a nil-Content (+ nil-Tags) update to the payload-only `UpdatePayload` method (no re-embed, vector preserved). `TestProtoconvUpdateMemoryRequestToArgs` (tags-only-mask and content+summary-mask cases) and `TestWriteParity/UpdateMemory/mask_tags_only_preserves_content` both present and pass | closed |
| T-17-PROV | Tampering (integrity) | payload-only provenance clear (two-op non-atomicity) | low | accept | See Accepted Risks Log AR-17-02 | closed |
| T-17-SC | Tampering | dependency supply chain | low | accept | See Accepted Risks Log AR-17-03 | closed |
| T-17-V5 | Tampering / improper input handling | mask/enum trust boundary | low | accept | See Accepted Risks Log AR-17-04 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `high` count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|--------------|------|
| AR-17-01 | T-17-15 | An EMPTY-string `email` (`""`) with `email_verified:false` under an ordered `[email, sub]` list is classified as absent/empty BEFORE the `email_verified` gate is consulted, and FALLS THROUGH to the encoded `sub` owner (`internal/auth/auth.go:135-137`, the empty-string check runs before the verified check at :141) — a deliberate widening vs the fail-closed `[email]`-only case. Accepted because an empty email carries no email identity to protect: falling through to `sub` cannot impersonate a specific verified-email user, and the caller lands in its OWN `sub`-namespaced authz bucket, never another's. Distinct from T-17-03's rejection of a NON-EMPTY present-but-unverified email, which still fails closed. Pinned by `TestClaimIdentityEmailSubPresenceTable`'s `"empty string email with email_verified:false still falls through (round-5 LOW widening)"` case. | Sean (via `/gsd-secure-phase 17`) | 2026-07-13 |
| AR-17-02 | T-17-PROV | `Store.UpdatePayload`'s targeted `SetPayload`+`DeletePayload` two-op design (`internal/store/store.go:1475-1534`) is NOT atomic: if the `DeletePayload` provenance-clear fails after the `SetPayload` commits, the record is left with stale provenance METADATA only (mislabels a summary's source) — it never corrupts content or the vector, and a later write reconciles it. Accepted in preference to the alternative (a whole-payload `OverwritePayload`), which round-7 review proved is WORSE: a whole-payload write from a stale `FetchForUpdate` snapshot with no compare-and-swap can revert a concurrent content update's content/tags while that write's new re-embedded vector survives, causing durable content/vector desync (corrupted recall) — a strictly worse outcome than stale metadata. The two caller-visible consequences (partial-success "committed but RPC failed", and soft last-writer-wins `AccessCount` RMW matching the pre-existing `IncrementAccess`) are documented in the store method's godoc (`grep -niE 'committed but|last-writer|partial' internal/store/store.go` matches) and covered by `TestUpdatePayloadInjectedDeletePayloadFailure`. | Sean (via `/gsd-secure-phase 17`) | 2026-07-13 |
| AR-17-03 | T-17-SC | Zero new external packages across all six plans in this phase — confirmed via `git diff main...HEAD -- go.mod go.sum` (empty diff; no commits on this branch touch `go.mod`/`go.sum`). All new code (protoconv, connecterror, identity, store_iface, the spy fake, the parity/cross-owner test suites, session versioning) uses only already-vendored stdlib and existing dependencies (`timestamppb`, `fieldmaskpb`, `connectrpc.com/connect`, `go/parser`/`go/ast` stdlib for the AST delegation test). | Sean (via `/gsd-secure-phase 17`) | 2026-07-13 |
| AR-17-04 | T-17-V5 | `protoconv.go` deliberately does NOT re-validate `update_mask` presence/allowlist or `Visibility` enum-zero rejection — it trusts the Phase-15 `protovalidate` interceptor, which runs in the interceptor chain BEFORE any handler body executes (`internal/server/connectapi.go:344-363`: `newConnectValidateInterceptor(validator)` is registered and ordered after auth+CSRF, so an invalid mask/enum never reaches a write handler). Re-validating in protoconv would duplicate an already-enforced boundary check with no additional protection, since a malformed/unvalidated request cannot reach the conversion layer in production. Tests assume valid input at the protoconv boundary per this trust relationship. | Sean (via `/gsd-secure-phase 17`) | 2026-07-13 |

*Accepted risks do not resurface in future audit runs.*

---

## Unregistered Flags

None. All six `17-0N-SUMMARY.md` files were checked for a `## Threat Flags` section (`rg -ni "threat" 17-0*-SUMMARY.md`) — no matches in any of the six. No new attack surface was flagged by the executor during implementation beyond the plan-time STRIDE register.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open (blocking) | Open (non-blocking) | Run By |
|------------|----------------|--------|-------------------|------------------------|--------|
| 2026-07-13 | 19 | 19 | 0 | 0 | gsd-secure-phase (L1 grep/read-depth; register authored at plan time across six PLAN.md, deduplicated by threat ID; every `mitigate` disposition verified against live source code, live test execution (`go test ./internal/auth/... ./internal/webauth/... ./internal/server/... ./internal/store/... ./internal/config/... ./cmd/...`, all green), and one live fail-closed-gate run (`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestConnectCookieLaneIsolation -v`, passed without skip); every `accept` disposition recorded in the Accepted Risks Log above) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-13
