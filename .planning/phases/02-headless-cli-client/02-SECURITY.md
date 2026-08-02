---
phase: 2
slug: headless-cli-client
status: complete
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-02
---

# v0.12.x Phase 2 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Retroactive audit — the phase shipped before `/gsd-secure-phase` was run for this milestone.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Shell → CLI process | Operator or agent invokes `engram search\|list\|store`; flags and env are attacker-influenced in a hostile-CI scenario | Server URL, bearer credential (via `ENGRAM_TOKEN` or `--token-file`), query text |
| CLI → engram server | Connect/HTTP over TLS to a remote engram instance | Bearer credential in the `Authorization` header; memory content in both directions |
| CLI → terminal/pipe | stdout consumed by an agent or `jq`; stderr read by a human | JSON documents (stdout), diagnostics and the `--insecure` warning (stderr) |
| CLI package → engram internals | `cmd/engram/client_*.go` must not reach into server-side packages | None — the boundary is enforced structurally |

---

## Threat Register

Authored at plan time across `02-01`/`02-02`/`02-03-PLAN.md` (`<threat_model>` blocks). Verified
retroactively against the shipped implementation by `gsd-security-auditor` on 2026-08-02.

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01 | Information Disclosure | credential intake, `addClientFlags` / `resolveToken` | high | mitigate | Only `--token-file` (a path) is registered; no `--token` flag exists anywhere in the tree. Every command sets `Args: cobra.NoArgs`, so a credential cannot ride in as a bare word. `client_common.go:38-39`; `client_search.go:31`, `client_list.go:35`, `client_store.go:36`. Tests `TestNoTokenFlagAnywhere`, `TestClientCommandsAcceptNoPositionalArgs` | closed |
| T-02-02 | Tampering / Information Disclosure | `newHTTPClient`, `--insecure` | high | mitigate | `InsecureSkipVerify: insecure` with zero-value `false`; `--insecure` is a `BoolVar` with **no** `os.Getenv` fallback, so the environment cannot silently disable TLS verification. `client_common.go:113-119`. Tests `TestTLSVerificationOnByDefault`, `TestInsecureIsNotSetByEnvironment` (asserts `ENGRAM_INSECURE`/`ENGRAM_TLS_INSECURE`/`ENGRAM_SKIP_VERIFY` all leave the flag false), `TestInsecureWarnsOnStderrAndStdoutStaysJSON` | closed |
| T-02-03 | Information Disclosure | error and warning text on every path | high | mitigate | The resolved token is never formatted into an error or warning string — `resolveToken`, `wrapRPCError` and `bearerInterceptor` pass it only into the `Authorization` header. Test `TestTokenNeverAppearsInOutput` covers success, auth-failure and transport-failure paths | closed |
| T-02-04 | Elevation of Privilege | client package boundary | medium | mitigate | No `client_*.go` implementation file imports any `internal/*` package. Enforced by a per-file AST gate, not a package-level one (operator commands in the same package legitimately import `internal/store`). `TestClientFilesImportBoundary` asserts the allowlist, that the allowlist itself holds no `internal/` path, and the denylist | closed |
| T-02-05 | Denial of Service | any client command | medium | mitigate | No client code path reads standard input, so no invocation can block on a prompt. Test `TestNoClientPathReadsStandardInput` | closed |
| T-02-06 | Spoofing | bearer header construction | medium | mitigate | `req.Header().Set("Authorization", "Bearer "+token)` — exactly one space, and no header is set at all when the token is empty. `client_common.go:98-107`. Tests `TestClientSearchSendsBearerHeader`, `TestClientSearchNoTokenSendsNoAuthHeader` | closed |
| T-02-07 | Information Disclosure | token file permissions | low | accept | See Accepted Risks Log AR-02-01 | closed |
| T-02-08 | Tampering | rendering of server-returned content | low | accept | See Accepted Risks Log AR-02-02 | closed |
| T-02-09 | Repudiation / Tampering | `storeCmd`'s RPC call | high | mitigate | Exactly one `client.StoreMemory(...)` call with an immediate `return wrapRPCError(err)`; no loop or retry construct exists in the file. `client_store.go:58-75`. Test `TestClientStoreNeverRetries` covers Unavailable, Internal and DeadlineExceeded — the three classes a well-meaning retry would target | closed |
| T-02-10 | Spoofing / Elevation of Privilege | `storeCmd`'s flag surface | high | mitigate | No flag for `actor`, `owner`, or any response-only field (`id`, `short_id`, `score`, `created_at`, …). `client_store.go:86-96`. Test `TestClientStoreNoActorOrOwnerFlag` | closed |
| T-02-11 | Information Disclosure | `list`'s paging cursor | low | accept | See Accepted Risks Log AR-02-03 | closed |
| T-02-12 | Denial of Service | `list --limit` | low | accept | See Accepted Risks Log AR-02-04 | closed |
| T-02-13 | Tampering | `rootCmd` argument handling | high | mitigate | `RunE: runSelfDescribe` and `Args: cobra.NoArgs` are present together, so a mistyped verb cannot print the catalog and exit 0. `root.go:43-44`. Test `TestRootUnknownSubcommandStillErrors` | closed |
| T-02-14 | Repudiation | exit-code catalog | high | mitigate | `buildCatalog`'s `ExitCodes` is derived from the shared `exitOK`…`exitUnavailable` constants, never a parallel literal, so the advertised taxonomy cannot drift from the mapper. `catalog.go:83-90`. Test `TestCatalogExitCodesMatchMapper` (bidirectional set equality) | closed |
| T-02-15 | Information Disclosure | catalog contents | medium | mitigate | `catalogFlag` carries only `Name`/`Type`/`Default`/`Usage`, and `Default` is `f.DefValue` — the *declared* default, not a resolved runtime value — so no field can carry a resolved credential. `catalog.go:35-40`. Covered transitively by `TestTokenNeverAppearsInOutput`, which shares the output writer | closed |
| T-02-16 | Denial of Service | self-describe path | low | accept | See Accepted Risks Log AR-02-05 | closed |
| T-02-17 | Information Disclosure | build version in the catalog | low | accept | See Accepted Risks Log AR-02-06 | closed |
| T-02-SC | Tampering (supply chain) | dependency surface | high | mitigate | `git diff --stat HEAD -- go.mod go.sum` clean; `golang.org/x/term` stays `// indirect` (not promoted); the test allowlist contains exactly the six declared packages and no `internal/` entry | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-02-07 | The client reads the token file with `os.ReadFile` and performs no `os.Stat` mode check. File permissions are the operator's responsibility; a mode check would be advisory only and could not prevent a world-readable file from being used. Verified: `resolveToken` (`client_common.go:71-89`) calls only `os.ReadFile`, matching the accepted rationale exactly | Phase 2 plan (`02-01-PLAN.md` threat model) | 2026-07-31 |
| AR-02-02 | T-02-08 | Memory content and summaries are rendered verbatim in the text/table lane with no terminal escaping. The JSON lane (`renderJSON` via protojson) escapes correctly, and JSON is the default for non-TTY consumers. Verified: `renderMemoryTable`/`truncateSummary` (`client_common.go:334-375`) render verbatim as stated | Phase 2 plan | 2026-07-31 |
| AR-02-03 | T-02-11 | The opaque page token is passed through as a flag value. It is server-minted and every page is re-authorized server-side, so possession of a cursor grants nothing a fresh call would not | Phase 2 plan (`02-02-PLAN.md`) | 2026-08-01 |
| AR-02-04 | T-02-12 | `list --limit` carries no client-side ceiling; the server owns the bound and rejects an over-large value with `CodeInvalidArgument`, which the shared mapper turns into exit 2. Duplicating the ceiling client-side would create two places for the rule to drift | Phase 2 plan (`02-02-PLAN.md`) | 2026-08-01 |
| AR-02-05 | T-02-16 | The catalog is built by walking the in-memory cobra tree. Verified: `buildCatalog`/`runSelfDescribe` (`catalog.go:52-139`) import only `encoding/json`, `sort`, `cobra` and `pflag` — no network or filesystem call is reachable | Phase 2 plan (`02-03-PLAN.md`) | 2026-07-31 |
| AR-02-06 | T-02-17 | `catalogDoc.Version` reports the same ldflags-injected `version` var that `engram version` already exposes. No new information is disclosed | Phase 2 plan (`02-03-PLAN.md`) | 2026-07-31 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-02 | 18 | 18 | 0 | gsd-security-auditor (retroactive, ASVS L1, block_on high) |

**Verdict: SECURED.** All 17 registered threats plus one recurring supply-chain check verified
against the live implementation and passing tests; full `cmd/engram` package suite green.

Register origin: `register_authored_at_plan_time: true` — all three plans carry `<threat_model>`
blocks, so the auditor verified stated mitigations rather than building a register retroactively.

No unregistered threats were self-reported: none of the three SUMMARYs carries a `## Threat Flags`
section, confirmed by search rather than assumed from absence.

**Cross-phase note (not a Phase 2 finding).** `client_search.go` and `client_list.go` now carry
`--cross-spine` flags (`searchCrossSpine`, `listCrossSpine`, `validateScopeCrossSpine`,
`renderCoverageFooter`). `git log -S` attributes these to `327fa9d6`, a **Phase 7** commit. That
surface is outside this phase's register by construction and is audited under
`07-SECURITY.md`; it is recorded here only so a future reader does not mistake it for Phase 2
surface that escaped review.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
