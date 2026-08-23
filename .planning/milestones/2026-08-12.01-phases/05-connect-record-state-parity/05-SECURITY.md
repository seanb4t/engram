---
phase: 05
slug: connect-record-state-parity
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-16
---

# Phase 05 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Register origin: **authored at plan time** — all four PLAN.md files carried a parseable
`<threat_model>` block, so this audit verified existing mitigations rather than
reconstructing a register retroactively. `T-05-SC` was declared identically in plans 05-01…05-03
and is recorded once; plan 05-04 amended it (see below).

Two audit rounds are recorded here. Round 1 (2026-08-16, tree at `36a969bc`) covered plans
05-01…05-03 — threats `T-05-01`…`T-05-15`. Round 2 (2026-08-16, tree at `86d0389f`) covered the
gap-closure plan 05-04, which executed after round 1 and left the ledger stale. 05-04's plan
numbered its own threats `T-05-10`…`T-05-14`, colliding with IDs round 1 had already assigned —
the plan was authored from `05-VERIFICATION.md`, which does not carry the threat-ID namespace.
They are renumbered `T-05-16`…`T-05-20` below, each row naming its original plan-time ID so the
mapping stays auditable.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Qdrant payload → `store.Memory` → `memoryToProto` → Connect response | Persisted record state crosses into a client-visible wire message; the `json:"-"` convention is the only thing keeping two audit stamps on the store side of this line. | Record state; two server-set audit stamps that must NOT cross |
| `.proto` schema → committed `gen/` trees → every Connect client binary | A field number/type, once merged, is consumed by clients this repo does not build. | Wire schema — a permanent one-way commitment |
| Recall/authz filter ← `store.Memory` fields | Not crossed by this phase; listed because reading a new field here is the highest-impact way this change could go wrong. | Recall eligibility (availability) |
| `store.Memory` json tags → the enforced Connect field set | The `json:"-"` tag is the sole mechanism keeping two audit stamps off the wire; this phase promotes it from convention to test-enforced invariant, which also means a careless edit to the invariant is now the way to open the hole. | Field-inclusion policy |
| The parity detector's own range | A guard that can never fire is indistinguishable from a guard that fires correctly, unless something proves it can reject. | Assurance itself |
| Connect write RPC → `formatWindowBound` → store `.Unix()` encode → both read lanes | A scheduling bound crosses three representations; an inconsistency between the two read lanes means two callers disagree about when a record reveals or expires. | Scheduling bounds (availability of scheduled records) |
| CLI stdout → operator | The rendered JSON is what an operator reads to answer "what version is this record"; an omitted key reads as "no answer" rather than "v0". | Operator-facing record metadata |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-05-01 | Information Disclosure | `memoryToProto` / `store.Memory` json tags | high | mitigate | Verified: exactly **2** real `json:"-"` struct tags remain in `type Memory` (`EmbedderIdentity` store.go:106, `IdempotencyFingerprint` store.go:129); neither `embedder_identity` nor `idempotency_fingerprint` appears in `engram.proto`. Parity was closed by adding a missing mapping, never by deleting an exclusion. | closed |
| T-05-02 | Tampering | proto field numbers 23-30 and their presence models | medium | mitigate | Verified: `go tool buf breaking --against '.git#branch=main'` exits 0; `task proto:lint` exits 0. The three explicit-presence keywords are present at their ratified numbers (`optional string superseded_by = 23`, `optional uint32 schema_version = 28`, `optional string summary_model = 29`). Numbers/presence models ratified once by Sean (D-14, 2026-08-15). | closed |
| T-05-03 | Denial of Service | recall gate / authz filter | high | mitigate | Verified: this phase adds no filter code (`git diff` touches only `memoryToProto` and the `.proto`). The shipped Phase-2 guard `internal/store/schemaversion_recallgate_test.go` recursively walks filter keys and proves `schema_version` is unreachable from every `recallEntryPointSeeds` member, with an allowlist of operator-only emitters (`Migrate`, `revert`, `migrate status`) each carrying a written D-16 justification; it asserts non-empty walked key sets so the absence check cannot go vacuous. Green in the full suite. | closed |
| T-05-04 | Spoofing | request-side messages | low | accept | Verified: none of the eight fields appears in any `*Request` message (0 occurrences of each across all request messages). All eight are server-set and response-only, so no new client-writable surface exists. | closed |
| T-05-05 | Information Disclosure | `storeToProtoFieldAlias` | high | mitigate | Verified: `maps.Equal(storeToProtoFieldAlias, want)` at connectapi_parity_test.go:167 pins the map by WHOLE-MAP equality — width and content together — so adding a second entry fails. Source-occurrence counts were explicitly rejected for this gate because they survive a widening. | closed |
| T-05-06 | Repudiation | the parity test itself | high | mitigate | Verified: five executor-performed RED proofs recorded verbatim in 05-02-SUMMARY.md with a "which gate trips" table, each reverted before commit; the permanent negative fixture (`permanent negative fixture is rejected`, :179) asserts an exactly-specified rejection every run; `assertDecodeBackCoversAllFields` pins which fields the comparator visited. "The test passes" is explicitly rejected as acceptance. | closed |
| T-05-07 | Tampering | test skip paths | medium | mitigate | Verified: `connectapi_parity_test.go` contains no `testDepsWithStore` call and no `t.Skip(` call — it is a pure unit test requiring no live Qdrant, so it cannot silently skip. | closed |
| T-05-08 | Information Disclosure | `unmappedStoreFields` name matching | high | mitigate | Verified: exact byte equality plus the single alias entry only; the `near-miss names are not fuzzily paired` sub-test (:186) requires prefix, substring, and case variants to be reported as unmapped. | closed |
| T-05-09 | Tampering | scheduling-window bounds across lanes | high | mitigate | Verified: `mcpWireBounds` (connectapi_boundary_second_test.go:42-69) reads the MCP bound out of the SERIALIZED json form — `json.Marshal` → `map[string]json.RawMessage` → key presence → decode — so the json tags that lane ships through are traversed, not bypassed. Expected values are computed with local `time.Truncate`/`Add` arithmetic rather than by calling the production helper, so the check is not a tautology. Proven RED against a `not_before` tag rename. | closed |
| T-05-10 | Information Disclosure | `not_after` truncation | high | mitigate | Verified: rounding asserted OUTWARD (`not_before` floors, `not_after` widens to the containing whole second) — a floor would collapse a scheduled expiry to immediate-expiry and drop a record out of recall early. Both the sub-second (:97) and exact-threshold (:194) cases are covered. | closed |
| T-05-11 | Repudiation | operator's view of record version | medium | mitigate | Verified: `decodeFirstMemory` (client_schemaversion_json_test.go:25-52) gates on a decoded `map[string]json.RawMessage` key rather than a stdout substring, so a value merely containing the text cannot register as a pass and a duplicate key cannot survive the decode. Paired with a permanent negative fixture (unassigned → key absent) proving it can fail. | closed |
| T-05-12 | Tampering | test fixture isolation in the shared Qdrant collection | medium | mitigate | Verified: each boundary sub-test derives a per-run unique scope via `boundarySecondScope(<subtest>)` and uses that same value for the write and the deferred `DeleteAll` (:98/:100, :194/:196). Residual accepted — see Accepted Risks R-05-02. | closed |
| T-05-13 | Denial of Service | silent test skip with no Qdrant | medium | mitigate | Verified: the boundary test acquires its store via `testDepsWithStore(t)`, which routes an absent Qdrant to `failOrSkipNoQdrant` (tools_test.go:193-203); under `ENGRAM_REQUIRE_QDRANT` that `t.Fatal`s instead of skipping, so CI cannot go green with the gate silently skipped. The mechanism is inherited from the shared helper rather than spelled in the boundary file. | closed |
| T-05-14 | Tampering | `store.Memory` json tags on the MCP read lane | high | mitigate | Verified: key PRESENCE in the decoded json map is asserted before any value comparison (`decoded["not_before"]`, :60-62), so a `json:"-"`, an `omitempty` misfire, or a tag rename on `NotBefore`/`NotAfter` fails loudly instead of passing through a Go-struct-level comparison. Carries its own RED proof. | closed |
| T-05-15 | Repudiation | `schema_version` presence under D-14's explicit presence | high | mitigate | Verified: the `zero-value source: timestamps unset, optional scalars still assigned` sub-test (connectapi_parity_test.go:646) asserts `Has(fd)` on `schema_version`/`summary_model` for a zero-value source, spelled at the pointer level so the nil-safe getters cannot mask it, and carries RED PROOF 3 against the conditional-assignment mutation. Residual seam accepted — see Accepted Risks R-05-01. | closed |
| T-05-16 | Tampering | vendored console bundle `internal/webauth/static/**` | medium | mitigate | Verified (round 2; plan-time ID `T-05-10`): `console_browser_test.go:315-333`'s `hydrationPollExpr` requires an `<h1>` containing the console heading; the served `internal/webauth/static/index.html` carries that string only inside `<title>` (`index.html:6`) and contains **zero** `<h1` occurrences, so the wait is unsatisfiable by the static shell. `internal/webauth/static.go:18` carries `//go:embed all:static`; 05-04-SUMMARY records the GH #106 mutation turning the test RED and the revert turning it GREEN. Non-vacuity is a genuine pre-condition, not decoration: `assertClean` (`:511-530`) fails when `len(o.successAppURLs) == 0` BEFORE checking failures, and `sweepConsoleAssets` (`:579-582`) fails when `len(refs) == 0` BEFORE requesting anything. | closed |
| T-05-17 | Information Disclosure | AES-256 cookie key sealing a real session in a test | low | mitigate | Verified (round 2; plan-time ID `T-05-11`): `console_browser_test.go:193-196` draws 32 bytes from `crypto/rand`; the key lives in a local variable, is never written to a file, and is hex-encoded only into the subprocess env map (`:206`). `harness_test.go:214-221`'s `childEnv` builds the child environment from `PATH`/`HOME` plus explicitly passed vars with no `os.Environ()` inheritance — so a developer's real key cannot leak in and the test key cannot leak out. | closed |
| T-05-18 | Spoofing | stub OIDC discovery document | low | accept | Verified (round 2; plan-time ID `T-05-12`): `stubOIDCProvider` (`console_browser_test.go:158-169`) registers exactly one handler, `GET /.well-known/openid-configuration`; the mux has no JWKS and no token handler. `ENGRAM_OIDC_ISSUER` is never set (`rg` finds it only in the comment at `:187` documenting the omission), so the MCP bearer lane stays anonymous. The `accept`'s factual premise was checked, not assumed. | closed |
| T-05-19 | Elevation of Privilege | `chromedp.NoSandbox` | low | accept | Verified (round 2; plan-time ID `T-05-13`): applied once, at `console_browser_test.go:373`, in a test-only file driving a locally-discovered browser against `fixture.srv.baseURL()` (loopback). `git diff 6f6abc8b..HEAD --stat` confirms 05-04 touched no production path — `internal/webauth`, `ui/`, `cmd/engram`, `internal/server`, `internal/store` and `charts/` are all absent from its changed-file list. | closed |
| T-05-20 | Denial of Service | CI runtime (browser on the `test` job) | low | accept | Verified (round 2; plan-time ID `T-05-14`): the `ci.yaml` diff is a single comment + `ENGRAM_REQUIRE_BROWSER: "1"` inside the **existing** `Test` step's `env:` block of the **existing** `test` job — no new job, no matrix; the job list (test, golangci, license, chart, workflows, python, buf, surfaces, ui-drift, ui-test, commit-lint) is unchanged in count and shape. `Taskfile.yaml` adds the same var to the existing `test:strict` env block. Browser work is bounded by an explicit 90s context and 45s poll timeout. | closed |
| T-05-SC | Tampering | Go module supply chain — `github.com/chromedp/chromedp` | medium | accept→**mitigate** (amended by 05-04) | **Round 1's text was falsified by 05-04 and is replaced, not appended to.** It claimed `go.mod`/`go.sum` untouched across `059807ab..HEAD` on an `accept` disposition; 05-04 added a Go module, so that premise no longer holds and the disposition is now `mitigate`. Verified (round 2), all four clauses independently: (1) `go.mod:13` pins `github.com/chromedp/chromedp v0.16.0` — a real semver tag, not an `@latest` pseudo-version; (2) every new `go.sum` entry carries both `h1:` and `/go.mod h1:` hashes, and `go mod verify` reports `all modules verified`; (3) `.golangci.yaml:5` sets `modules-download-mode: readonly`, so an unrecorded module fails lint; (4) test-only proven by execution rather than file-location inference — `go list -deps ./cmd/engram \| rg chromedp` exits 1 (absent from the shipped binary's graph) while `go list -deps -test ./internal/e2e` contains it. **Full new-module inventory** (not just the module the plan named): direct — `chromedp/cdproto`, `chromedp/chromedp`; indirect — `chromedp/sysutil`, `go-json-experiment/json`, `gobwas/httphead`, `gobwas/pool`, `gobwas/ws`; go.sum-only for module-graph completeness — `ledongthuc/pdf`, `orisano/pixelmatch`, and a `golang.org/x/sys v0.6.0/go.mod` entry. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-05-01 | T-05-15 | The `schema_version` presence gate covers the MAPPER only. Plan 05-03's CLI test renders a stub-built `engramv1.Memory`, so no test in this phase spans the mapper→renderer seam end to end. Stated in the plan rather than papered over; tracked as open seam issue #499. | Sean (plan-time, 2026-08-15) | 2026-08-16 |
| R-05-02 | T-05-12 | `mem_eval_test` is a shared collection; `defer`s do not run on a killed process, and a whole-collection query still sees these points. Per-run unique scopes bound the blast radius to cross-run confusion, not to data loss. Accepted explicitly at plan time. | Sean (plan-time, 2026-08-15) | 2026-08-16 |
| R-05-03 | T-05-04 | None of the eight fields is added to any request message, so generic mass-assignment is not reachable here. Treated as canon covered by `/gsd-secure-phase` rather than minted as a bespoke prohibition. | Sean (plan-time, 2026-08-15) | 2026-08-16 |
| ~~R-05-04~~ | T-05-SC | **WITHDRAWN 2026-08-16 (round 2).** Read: "No package-manager install task and no new external dependency in this phase." Plan 05-04 added `github.com/chromedp/chromedp` plus six transitive modules, so the premise is false and the risk was never eligible for acceptance on those grounds. `T-05-SC` moves from `accept` to `mitigate` and is closed on verified controls — exact-version pinning, recorded `go.sum` checksums, `readonly` lint mode, and test-only build-graph isolation. Kept struck rather than deleted so the withdrawal is legible. | Sean (plan-time, 2026-08-15); withdrawn by round-2 audit | 2026-08-16 |
| R-05-05 | T-05-19, T-05-20 | `chromedp.NoSandbox` and one headless Chrome on the existing CI `test` job are accepted as test-only costs. `NoSandbox` is required because Ubuntu 24.04 restricts unprivileged user namespaces for a non-root CI user; the browser is bounded by an explicit 90s context and 45s poll timeout and adds no job or matrix. Neither reaches a production code path, container, or deployment manifest. | Sean (plan-time, 2026-08-16, plan 05-04) | 2026-08-16 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Round | Tree | Threats Total | Closed | Open | Run By |
|------------|-------|------|---------------|--------|------|--------|
| 2026-08-16 | 1 — plans 05-01…05-03 | `36a969bc` | 16 | 16 | 0 | Claude Opus 5 (`/gsd-secure-phase 05`, ASVS L1, block_on=high) |
| 2026-08-16 | 2 — plan 05-04 (gap closure, delta) | `86d0389f` | 21 | 21 | 0 | Claude Opus 5 (`/gsd-secure-phase 05`, ASVS L1, block_on=high) |

**Round 1** — method: State B (no prior SECURITY.md; plans and summaries present). Register parsed
from the three PLAN.md `<threat_model>` blocks, deduplicated on `T-05-SC`. Each mitigation was
verified against the tree at `36a969bc` by direct source inspection plus two executed gates
(`task proto:lint`, `go tool buf breaking --against '.git#branch=main'`, both exit 0). No
mitigation was closed on the strength of a SUMMARY claim alone.

One correction made during round 1: a file-wide count of `json:"-"` in `internal/store/store.go`
returns 5, which would have read as a T-05-01 regression. Three of those hits are comment prose;
scoped to `type Memory struct` the count is the expected 2 real struct tags. Recorded here because
the naive count is the more obvious check and gives the wrong answer.

**Round 2** — method: State A delta audit. Scope was the six threats plan 05-04 introduced or
amended (`T-05-16`…`T-05-20`, `T-05-SC`); `T-05-01`…`T-05-15` were treated as settled from round 1
and not re-verified.

Why round 2 was needed at all, recorded because the failure mode is structural and will recur: a
`--gaps-only` run lands AFTER `/gsd-secure-phase` and `/gsd-verify-work` have written their
artifacts, so those artifacts go stale with nothing watching. `execute-phase`'s `aggregate_results`
security gate reads only the `threats_open` field, so it reported `0` — true of the pre-05-04 tree,
not of what shipped — while this ledger simultaneously asserted `go.mod`/`go.sum` were untouched and
05-04 had added seven modules. A stale ledger is indistinguishable from a current clean one under
that gate. What caught it was the `gsd-verifier`, because its re-verification was scoped to ALL FOUR
plans and it compared each artifact's audit SHA against the phase's commit range; a re-verification
scoped to the gap plan alone would have shipped the false claim.

Two corrections made during round 2, both replacements rather than appends: the stale `T-05-SC` row
(see the register) and the withdrawal of `R-05-04`, whose acceptance rationale rested on the same
falsified "no new external dependency" premise. Supply-chain verification deliberately enumerated
the FULL module delta rather than only the module 05-04's plan named — a supply-chain threat checked
against one of seven new modules is not checked. Test-only isolation was proven by executing
`go list -deps ./cmd/engram` and confirming absence from the shipped binary's build graph, not by
inferring it from the `_test.go` filename.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter
- [x] Round 2: plan 05-04's threat model folded in, ID collision resolved (`T-05-10`…`T-05-14` → `T-05-16`…`T-05-20`)
- [x] Round 2: stale `T-05-SC` row and `R-05-04` acceptance corrected against the shipped tree

**Approval:** verified 2026-08-16 (round 1, tree `36a969bc`); re-verified 2026-08-16 (round 2, tree `86d0389f`)
