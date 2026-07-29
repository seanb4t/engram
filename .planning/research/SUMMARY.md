# Project Research Summary

**Project:** engram v0.12.x — "Headless Reach & Diagnosability"
**Domain:** Additive integration work on a shipped, self-hosted, multi-tenant Go memory-MCP server (Connect/MCP dual-transport, Qdrant-backed, Cedar-authorized)
**Researched:** 2026-07-29
**Confidence:** HIGH

## Executive Summary

This milestone makes engram usable by agents that are not a top-level MCP client (a headless CLI over ConnectRPC, issue #343, plus its dependent items #344/#347/#350/#360) and makes what the server decides and rejects legible (#394, and investigatively #351). Every capability is additive to the existing Connect/MCP/store architecture — none require restructuring a shipped component's public contract, and **zero new Go dependencies are required**: connect-go, `golang.org/x/oauth2/clientcredentials`, cedar-go, `log/slog`, OTel, and stdlib `io`/`net/http` already cover the full surface. Several items are also smaller than their issue text implies: #356 (UI TS codegen drift) is already fully shipped and should be closed with rationale, not carried into this milestone, and #394's diagnostics wiring is "connect an already-computed value to a reader," not a new authz primitive.

The milestone's structural spine is #343 (headless Connect mounting + bearer auth), and it is also its highest-risk item. Two research passes converged on the same root cause from different angles: `newConnectCSRFInterceptor` today has exactly one identity source (the cookie lane), so any CSRF-exemption logic that keys off request-controlled signals (header/cookie presence) rather than resolver-set provenance is a full CSRF bypass on all six write RPCs. Separately — and this is a finding the individual research passes could not cross-reference against each other, so it is elevated here — the pitfalls research identified that Connect's interceptor chain calls `resolve()` directly and **never routes through `mcpauth.RequireBearerToken`**, whose private `verify()` is where `TokenInfo.Expiration` is actually enforced. A Connect bearer resolver built by "just calling `auth.ChainVerifier`" (the stack research's literal recommended shape) therefore makes `Expiration` decorative on the Connect lane — including the static-token lane's 100-year sentinel, which exists *solely* to satisfy a check that lane will never run. This is a security-critical, silently-passing defect class: it compiles, vets, lints, and passes a happy-path test suite cleanly, and is only caught by a test that constructs a `TokenInfo` with a past `Expiration` and confirms the Connect resolver rejects it. Any plan for #343 must explicitly build this check, not assume it comes free from reusing `ChainVerifier`.

The remaining items (#344 cross-spine search, #394 diag wiring, #347 embed error bodies, #350 per-lane key, #345 reindex resume) are architecturally independent of the #343 spine and of each other (aside from two items sharing `tools.go`), and can run in parallel once the spine's provenance-tagging groundwork lands. #351 (why rule capture never fires) is scoped as investigate-then-fix, not a known implementation — the feature research's deep dive gives concrete, consent-preserving intervention candidates but explicitly warns against jumping to a UX fix before confirming the break isn't mechanical.

## Key Findings

### Recommended Stack

Nothing new to install. `connectrpc.com/connect` v1.20.0 supplies both the CLI's client construction and the server's bearer-resolver adapter via the same interceptor idiom already used four times in `mountConnect`. `golang.org/x/oauth2/clientcredentials` (already a subpackage of a direct dependency) covers CLI-side OIDC client-credentials token acquisition — no second OIDC library. `cedar-go` v1.8.0 already computes the `Decision.diag` value #394 needs to surface; only an accessor is missing. Bounded HTTP error-body reads (#347) are pure stdlib (`io.LimitReader` + `io.Copy(io.Discard, ...)`). CI's codegen-drift check for `gen/ts/` → `ui/src/lib/gen/` already exists and passes (`.github/workflows/ci.yaml:139-147`) — confirming the milestone-context note that #356 is out of scope.

**Core technologies (all already in go.mod):**
- `connectrpc.com/connect` v1.20.0 — CLI client + server bearer resolver, same interceptor shape both directions
- `golang.org/x/oauth2/clientcredentials` v0.36.0 — CLI OIDC token acquisition (RFC 6749 §4.4 grant, not a second OIDC client library)
- `github.com/cedar-policy/cedar-go` v1.8.0 — source of `Decision.diag`; needs only an exported, PII-safe accessor
- `log/slog` + `go.opentelemetry.io/otel/trace` v1.44.0 — debug logging + span events for authz decisions
- stdlib `io`, `net/http` — bounded error-body read + drain-for-reuse in `internal/embed`

### Expected Features

**Must have (table stakes):** JSON output on `engram search|store|list` (default-on when piped); stdout=data/stderr=diagnostics separation; documented, semantic exit codes mapped from engram's existing error taxonomy; non-interactive-by-default CLI (no TTY prompts — the primary caller cannot answer one); token/server-URL via `ENGRAM_`-prefixed env vars with flag override; actionable validation errors naming the true offending field (direct target of #360); foreign-scope results visibly tagged with their source scope on cross-spine search; authz denial reasons reaching a reader instead of being silently computed and dropped (#394).

**Should have (differentiators):** a self-describing CLI (bare invocation/`--help` returns the full command/flag/error catalog — cheap given cobra already builds the tree internally); a structured error envelope with a `hint`/`next_actions` field, extending the #360 fix; a `scopes_searched` marker on cross-spine results so an agent can distinguish "found nothing" from "searched everywhere and found nothing"; friction-reducing (not consent-removing) interventions for rule capture.

**Defer:** a full cross-spine "coverage receipt" (scopes searched vs. skipped-with-reason) — the scope space is small and enumerable today, so this is speculative; any CLI manifest/introspection richer than the closed 3-command surface.

**Explicitly out of scope / anti-features:** auto-promoting a memory to a rule (violates the user-blessed gate, and PKM literature shows agent-generated structure measurably underperforms human-authored structure); embedding-similarity auto-supersede; a CLI `--auto-json` mode inferring format from response shape (non-deterministic format selection is a documented CLI-agent failure mode); flipping `cross_spine` to default-true; verbose Cedar expression-trace logging on every denial (OPA's own maintainers rejected this for cost and PII-leak reasons — directly conflicts with DEC-wot); a CLI that internally retries mutating calls on ambiguous failure without an idempotency key.

### Architecture Approach

Six items land at six mostly-disjoint seams in the existing Connect/MCP/store architecture. The MCP lane, `internal/store`, and `internal/authz` are untouched by the auth-lane work. The spine (#343) requires extracting `withAuth`'s chain-builder into a reusable function so both the MCP wrapper and the new Connect bearer resolver consume one composed `mcpauth.TokenVerifier` — never two independently-constructed chains that can drift.

**Major components:**
1. **Connect bearer resolver + composed resolver** (new) — verifies bearer tokens via the shared `auth.ChainVerifier`, stamps an explicit lane-provenance value (`auth.LaneBearer`/`auth.LaneCookie`) into context; the composed resolver routes by *structural presence* of the `Authorization` header, never try-then-fallback across the two lanes.
2. **CSRF interceptor** (modified) — reads lane provenance from context, never from request headers/cookies; exempts only genuinely bearer-authenticated requests.
3. **CLI subcommands** (`cmd/engram/search.go`/`store.go`/`list.go`, new) — thin cobra commands that speak only the generated Connect stubs (`gen/go/engram/v1/engramv1connect`), deliberately never importing `internal/store`/`internal/authz`/`internal/embed` directly (that's the operator-command pattern, not the client-command pattern).
4. **`cross_spine` on `search_memory`** (`tools.go`, `store.go`, proto field 9, additive) — mirrors `search_discovery`'s existing `CrossSpine` mechanism at the args/handler layer.
5. **`authz.Decision` safe accessor** (`internal/authz`, new) — a `slog.LogValuer`-based, field-allowlisted view exposed to `internal/store`'s two existing decision chokepoints (`decideBucket`/`decideRecord`), never the raw `cedar.Diagnostic`.
6. **Per-lane API key** (`internal/config`, `tools.go`, mechanical) — mirrors the already-shipped base-URL split for chat vs. embed.

### Critical Pitfalls

1. **CSRF exemption keyed on request-controlled input** — must be decided inside the resolver at verification time and carried forward as an explicit, non-inferable context value; never derived from header/cookie presence at the CSRF interceptor. This is the milestone's #1 named risk and must be the first slice proven with a negative test.
2. **Connect bearer resolver silently drops `Expiration` enforcement** — see Executive Summary; `RequireBearerToken`'s header-parse + expiration check must be explicitly re-implemented or extracted for the Connect path, since Connect never passes through the go-sdk's HTTP middleware that performs it today.
3. **Two independently-built `ChainVerifier`s drift** — `withAuth` must be refactored to expose its composed verifier to both mount sites, not reconstructed a second time for Connect.
4. **`cross_spine` copying `SearchDiscoveries`' pattern verbatim** — see the disagreement section below; must be individually verified, not assumed safe by analogy.
5. **Deny-only or unredacted `authz.Decision.diag` logging** — must log both allow and deny at debug level, with a reviewed field allowlist re-applying DEC-wot's owner-only-no-actor-no-email rule (`cedar.Diagnostic` has no PII fields by construction, but the reader discipline still must be explicit).

## Cross-Cutting Finding: The `cross_spine` Disagreement (#344) — Present, Not Resolved

The architecture research and the pitfalls research reached different emphases on the same change, and a planner must resolve this by verification, not by picking a side up front.

**Architecture's position:** `cross_spine` on `search_memory` is a precise structural mirror of `search_discovery`'s existing implementation. Concretely: add a `CrossSpine bool` arg (byte-for-byte the discovery precedent), extract a shared `effectiveScope(scope, crossSpine)` helper, and change `ownerScopeFilter` from unconditionally appending `qdrant.NewMatch("scope", scope)` to a conditional append — the exact one-line change `SearchDiscovery` already makes. It further notes the authz interaction is orthogonal today: `SearchDiscovery` still unconditionally appends `s.ownerOrSharedCondition(subj)` regardless of scope, because scope and authz are two independent filter dimensions in the same Qdrant `Must` list.

**Pitfalls' position:** this shape cannot be assumed safe to copy verbatim, for two reasons. First, discoveries live in a `discovery:*` namespace convention — a memory's `scope` is an arbitrary repo/workspace/worktree string with no equivalent confining namespace, so "everywhere" means something structurally different for the two categories. Second, and more load-bearing: the *fact* that `ownerScopeFilter`'s authz `Must` clause composes unconditionally and independently of the scope clause must be **verified by reading `Store.Search`'s filter-building code end to end**, not assumed by analogy to `SearchDiscovery`'s separate code path — the existing single-scope code path may have implicitly relied on `scope` being non-empty as an unaudited secondary narrowing signal somewhere in that path, and making scope optional would exercise that path on an input it was never designed for.

**What a planner must do before treating this as a one-line change:**
1. Read `Store.Search`'s full filter-construction path (not just `ownerScopeFilter` in isolation) and confirm in writing that the authz/owner `Must` clause is built as a genuinely separate, unconditional entry from the scope `Must` clause — never a single combined condition where omitting scope could silently omit part of the authz gate.
2. Add `cross_spine` as a named `SearchOptions` struct field (continuing the D-09 struct-over-positional-param discipline), not a bare boolean threaded positionally.
3. Write `TestCrossSpineSearchNeverBypassesOwnerFilter` — two different authenticated owners with records in overlapping scope names; owner A searches `cross_spine=true` with empty scope; assert owner B's private records never appear — against **real Qdrant** (testcontainers), not a mock, since a mock could paper over exactly the filter-composition bug this disagreement is about.

Both research passes agree the *proto/args/handler* layer (field 9, `CrossSpine` arg, `effectiveScope` helper) is safe and mechanical. The disagreement is entirely about whether the *store-layer filter composition* is safe to assume unconditional versus requiring active verification — treat it as requiring verification.

## Cross-Cutting Finding: Items Invisible to a Green Test Suite

This project's documented failure mode is defects that compile, vet, lint, and pass the full suite, caught only by a reviewer reasoning from the contract rather than from what the tests happen to exercise. Consolidated list, for reviewer use:

| # | Item | Why it's invisible | The test that catches it |
|---|------|---------------------|---------------------------|
| 1 | Connect bearer resolver skips `Expiration` enforcement | Happy-path tests (valid, non-expired token) never exercise the missing check; `go vet`/lint cannot flag "a struct field exists but nothing reads it" | `TestConnectBearerResolverRejectsExpiredTokenInfo` — feed a `TokenVerifier` stub returning `TokenInfo{Expiration: past}` with `err == nil`; assert rejection |
| 2 | CSRF exemption keyed on request-controlled signal | Looks correct against every test that only sends *either* a valid cookie *or* a valid bearer header — never the adversarial combination | `TestCSRFCookieCallerOmittingHeaderIsStillRejected` + `TestCSRFCookieCallerCannotSelfDeclareBearerLane` (cookie session + garbage `Authorization` header) |
| 3 | Combined resolver falls through bearer-failure to cookie | Same blind spot as #2 — no test sends a valid cookie *and* an invalid bearer header simultaneously | `TestBearerFailureNeverFallsThroughToCookie` |
| 4 | Two independently-constructed `ChainVerifier`s drift | Each lane has its own mocked-chain tests that individually pass; divergence only shows up when a real caller gets different acceptance behavior between lanes | `TestAuthChainSharedBetweenLanes` — structural assertion that both mount sites call the same constructor |
| 5 | `cross_spine` widening the authz filter, not just the scope filter | A mock-based test can pass even if the real Qdrant filter composition is wrong; single-owner tests don't exercise cross-owner isolation | `TestCrossSpineSearchNeverBypassesOwnerFilter` (real Qdrant, two owners) |
| 6 | `authz.Decision.diag` logged deny-only or unredacted | Deny-path logging "looks complete" — nobody notices the allow path is silently unlogged until debugging an unexpected allow; PII risk depends on `cedar.Diagnostic`'s actual field shape, which nobody has had reason to inspect before | Field-allowlist test + an assertion that both allow and deny paths produce a log line |
| 7 | #360 validation-error fix pins the one reported string instead of fixing branch-attribution | A test asserting the new exact string passes trivially, while the underlying misattribution mechanism (which field a schema fallback names) remains broken for every other multi-field-invalid combination | A matrix test: one case per single-field-invalid input (bad content, bad summary, bad category, bad scope...), asserting the *correct* field is named each time, not string-matching exact wording |
| 8 | `reindex --resume` tags fix changes only the comparison line | `reindexTarget`/`reindexTargetContents` never fetch the target's stored `tags` at all — a comparison against an always-nil field either always "matches" (bug persists) or always "mismatches" (resume stops working, but nothing errors — it just silently re-embeds everything, costing more without ever surfacing as a failure) | `TestReindexResumeSkipsOnContentMatchTagsDiffer` (including a same-elements-different-order case) + `TestReindexResumeSkipsWhenContentAndTagsBothMatch` as the paired positive control |
| 9 | Headless mount defaults on, or is derived from an already-true condition | The "smallest diff" version of this fix is loosening `mountConnect`'s existing `if resolve == nil` guard with an OR — every existing test that only checks "UI enabled → mounted" still passes even if the new condition is wrong | `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag` — UI disabled AND headless flag unset must leave Connect unmounted, byte-for-byte today's behavior |

## Build Order and Dependencies

```
Item 1 (bearer resolver + lane provenance + headless mount + Expiration check)
   │
   ├──► Item 2 (CSRF provenance exemption) — strictly needs Item 1's lane stamp
   │        │
   │        ▼
   └──► Item 3 (CLI): search/list need only Item 1 (read RPCs aren't CSRF-gated);
                       store needs Item 1 AND Item 2 before it can be called "done"

Independent, parallelizable with the spine and with each other:
   Item 4 — cross_spine on search_memory (#344)         — touches tools.go, store.go, proto
   Item 5 — authz.Decision.diag reader (#394)            — touches internal/authz, internal/store only
   Item 6 — per-lane API key (#350)                      — touches internal/config, tools.go (summarizerFromConfig)
   Item 7 — reindex --resume tags fix (#345)              — touches internal/store, fully separate code path
   Item 8 — #360 validation-error root-cause fix          — touches validation/schema layer, fully separate
   Item 9 — #347 bounded embed error-body read            — touches internal/embed only, stdlib-only fix
   Item 10 — #351 rule-capture investigation → fix        — investigate first; intervention (if UX) touches curating-memory skill + session-start surfacing, not core server code
```

**Sequencing notes:**
- Within the spine, Items 1 and 2 are sequential (2 needs 1's stamp), and the CSRF negative test (`TestCSRFCookieCallerOmittingHeaderIsStillRejected`) should be written *before* the exemption branch is wired — mirroring the project's own "prove fail-closed as the phase's first test" precedent from v0.11.x.
- Item 3's `store` subcommand must not be considered shippable until Item 2's negative tests are green — shipping a write-capable CLI against a CSRF exemption still keyed on header-absence would ship the exact vulnerability the milestone's risk section names.
- Item 4 (cross_spine) and Item 6 (per-lane key) both touch `tools.go` in different functions (`searchMemory`/`effectiveScope` vs. `summarizerFromConfig`) — low but non-zero merge risk if landed in the same PR; sequence or keep diffs small and independently reviewed.
- #351 is explicitly investigate-then-fix per the milestone framing — do not plan a specific UX intervention before confirming (via a trace of actual `store_rule` invocation attempts, including failures) whether the gap is behavioral/friction or mechanical/bug. If mechanical, this could be a small, independent fix with no dependency on anything else in the milestone.

## Implications for Roadmap

### Phase 1: Headless Connect Spine (bearer auth + provenance + headless mount)
**Rationale:** Structural dependency root for the entire CLI story (#343) and the milestone's highest-risk item (CSRF bypass class). Must land, and be proven fail-closed, before anything else in the "headless reach" half of the milestone.
**Delivers:** Extracted shared `ChainVerifier` builder; new Connect bearer resolver with explicit `Expiration` enforcement; lane-provenance context stamp; composed (bearer+cookie) resolver with no cross-lane fallback; CSRF interceptor reading provenance, not request signals; independently-defaulted-off headless-mount config flag.
**Addresses:** Table-stakes CLI credential plumbing (env var token/server-URL) from FEATURES.md; the milestone's explicit #1 risk.
**Avoids:** Pitfalls 1, 2, 3, 4, 5 (CSRF-on-request-signal, cross-lane fallthrough, missing Expiration check, drifted ChainVerifier, silent exposure-flip-on-upgrade). Ship the negative tests listed in the "invisible to a green suite" table as this phase's definition of done, not follow-up work.

### Phase 2: CLI Subcommands (`engram search|store|list`)
**Rationale:** Depends on Phase 1. Read commands are buildable/testable the moment the headless mount + bearer resolver exist; the write command additionally requires Phase 1's CSRF fix to be safe to ship.
**Delivers:** cobra subcommands under `cmd/engram/`, speaking only the generated Connect client (no `internal/store`/`internal/authz`/`internal/embed` imports); JSON-default-when-piped output; stderr-only diagnostics; documented exit codes mapped from the Connect error-code taxonomy; file/stdin-based credential input (never a bare `--token` flag value).
**Uses:** connect-go client idioms, `clientcredentials`, `internal/config` field-registry extension for `ENGRAM_CLIENT_*` vars.
**Avoids:** Pitfalls 7, 8, 9 (token leakage via argv/env/crash-reports, exit-code collapse, silent version-skew no-ops).

### Phase 3: Cross-Spine Memory Search (#344)
**Rationale:** Independent of the spine; can run in parallel with Phase 1/2, but carries the milestone's one unresolved architecture/pitfalls disagreement and must not be treated as a trivial mirror of `search_discovery` without the verification step.
**Delivers:** `cross_spine` arg on `search_memory` (MCP + Connect, additive proto field 9); shared `effectiveScope` helper; verified-unconditional authz `Must` clause in `Store.Search`; per-result scope tagging.
**Addresses:** Table-stakes foreign-scope tagging and the differentiator scopes-searched marker from FEATURES.md.
**Avoids:** Pitfall 10 — requires the `Store.Search` filter-composition read-through and the real-Qdrant, two-owner isolation test described in the disagreement section above, before this can be called done.

### Phase 4: Diagnosability (#394 authz decision logging, #360 validation-error attribution, #347 embed error bodies)
**Rationale:** Three independent, small, PII/legibility-shaped fixes sharing a common design discipline (bounded, structured, redaction-conscious disclosure) even though they touch different subsystems. Grouping them lets one internal convention (a shared diagnostic-envelope shape) land coherently rather than as three ad hoc mechanisms.
**Delivers:** `authz.Decision` safe `slog.LogValuer` accessor logged at both allow and deny; #360's root-cause validation-branch-attribution fix (not a string substitution) with a per-field matrix test; #347's bounded-read-and-drain fix in `internal/embed` (and audit `internal/summarize` for the same gap).
**Addresses:** Table-stakes authz-denial-reaches-a-reader and actionable-error-naming-the-true-field from FEATURES.md.
**Avoids:** Pitfalls 11, 12 (deny-only/unredacted diag logging, string-patched validation error masking the real branch bug).

### Phase 5: Operator/Correctness Tail (#350 per-lane API key, #345 reindex --resume tags fix, #351 rule-capture)
**Rationale:** Fully independent, mechanical-or-investigative items with no shared files or dependencies on the above phases; good candidates for parallel execution once assigned, or for filling schedule gaps around the spine work.
**Delivers:** `OpenAIConfig.ChatAPIKey` mirroring the shipped `ChatBaseURL` split (#350); `reindexTarget`/`reindexTargetContents` extended to fetch and order-independently compare target `tags`, plus a documented one-time repair path for records an unpatched resume run already skipped incorrectly (#345); #351's investigation output, and — only if the investigation finds a friction/UX cause rather than a mechanical bug — one of the consent-preserving intervention candidates from FEATURES.md's deep dive (session-start reminder, single-turn propose-and-confirm, widened trigger surface in the `curating-memory` skill routing table).
**Avoids:** Pitfall 13 (reindex resume's silent staleness); the auto-promotion/auto-supersede anti-features named explicitly Out-of-Scope in PROJECT.md.

### Phase Ordering Rationale

- Phase 1 must lead because it is the only item with downstream structural dependents (Phase 2's `store` subcommand, and the shared lane-provenance mechanism nothing else in the milestone needs but everything else can safely ignore).
- Phases 3, 4, and 5 have zero file overlap with Phase 1/2 and with each other (excepting the noted `tools.go` co-location between cross_spine and per-lane-key edits) and can be assigned to parallel workstreams once Phase 1 is underway, per the architecture research's own "Wave 1/Wave 2" framing.
- Grouping #394/#360/#347 into one phase (rather than three scattered ones) is a deliberate roadmap choice to let a shared diagnostic-disclosure convention emerge, per the feature research's explicit dependency note.
- #351 is placed last/independently specifically because its own milestone framing separates investigation from fix — sequencing it early would risk committing to an intervention before the root cause is confirmed.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1 (Headless Connect Spine):** needs a focused read-through of the go-sdk's `mcpauth.RequireBearerToken`/`verify()` internals (already partially done by pitfalls research, but the exact extraction/reuse shape for a transport-agnostic Expiration-check helper needs to be nailed down at plan time) and a security-focused plan review given the CSRF-bypass and confused-deputy risk classes.
- **Phase 3 (Cross-Spine Memory Search):** needs the `Store.Search` filter-composition verification described in the disagreement section as an explicit planning step, not an assumption carried in from research.

Phases with standard patterns (skip research-phase):
- **Phase 2 (CLI Subcommands):** connect-go client idioms and cobra subcommand structure are already fully mapped to concrete file paths and precedents in ARCHITECTURE.md.
- **Phase 4 (Diagnosability):** each of the three fixes has a fully-specified target file, function, and test shape in both STACK.md and PITFALLS.md.
- **Phase 5 (Operator/Correctness Tail):** #350 and #345 are mechanical, precedent-mirroring changes with concrete line-level guidance already produced; #351's investigation step is itself the research (not a phase needing external research).

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Verified against Context7 current docs (connect-go, oauth2) plus direct source reads of go.mod and every integration seam; zero speculative recommendations |
| Features | MEDIUM | Web/community consensus across multiple independent sources for CLI/agent-UX conventions and PKM/capture-friction literature; not validated against engram's own usage telemetry (no such telemetry exists yet) |
| Architecture | HIGH | Every claim grounded in direct source reads with file/line citations; no speculation about unread code |
| Pitfalls | HIGH | Grounded directly in current `internal/server`/`internal/auth`/`internal/webauth`/`internal/store` source, including a direct read of the go-sdk's `auth/auth.go` from the module cache to confirm the Expiration-enforcement gap |

**Overall confidence:** HIGH — with one explicit gap (below) that must be closed during Phase 3 planning, not assumed away.

### Gaps to Address

- **`Store.Search` filter-composition verification (Phase 3):** the architecture and pitfalls research disagree on whether the authz `Must` clause composes unconditionally and independently of the scope clause. This is not a research gap that can be closed by more reading here — it requires a planner/implementer to read the current filter-construction code end to end at plan time and write the isolation test *before* implementing the feature, per the disagreement section above.
- **#351's root cause is genuinely unknown** — the feature research provides strong candidate interventions but explicitly states the investigation (trace actual `store_rule` invocation attempts, including failures) must happen first; treat any specific intervention chosen at roadmap time as provisional pending that trace.
- **Feature-research confidence (MEDIUM) reflects literature synthesis, not engram-specific data** — the CLI-UX and rule-capture-friction conclusions are well-supported externally but unvalidated against this project's own agents/users; treat FEATURES.md's differentiator tier as directional rather than committed scope until validated post-ship.

## Sources

### Primary (HIGH confidence)
- Context7 `/connectrpc/connect-go` — client-side `connect.NewClient`, interceptor shapes
- Context7 `/golang/oauth2` — `clientcredentials` grant implementation
- Direct source reads: `cmd/engram/serve.go`, `internal/server/connectapi.go`/`connectcsrf.go`/`connectreseal.go`/`connectauth.go`/`identity.go`/`tools.go`, `internal/store/store.go`, `internal/authz/authz.go`, `internal/auth/chain.go`/`auth.go`, `internal/webauth/resolver.go`/`handlers.go`, `internal/config/registry.go`/`config.go`, `proto/engram/v1/engram.proto`, `.github/workflows/ci.yaml`, `.planning/PROJECT.md`
- `github.com/modelcontextprotocol/go-sdk@v1.6.1` `auth/auth.go` — read directly from module cache to confirm `RequireBearerToken`'s `verify()` is never invoked in the Connect interceptor chain
- `github.com/cedar-policy/cedar-go@v1.8.0/types/authorize.go` — confirms `cedar.Diagnostic` carries no PII fields by construction
- GitHub issue #356 (`gh issue view 356`) — confirmed already-shipped scope

### Secondary (MEDIUM confidence)
- The CLI Spec, cli-agent-spec, InfoQ, Terry Li's seven-patterns article — agent-facing CLI structured-output/exit-code/credential conventions
- Cloudflare AI Search, Elastic cross-project search, claude-context #374 — per-result scope tagging and coverage-receipt precedent for cross-spine search
- Van Kleek et al. (Finders/Keepers, Note to Self), isophist's PKM synthesis (2025 AGENTS.md study) — capture-friction root-cause literature for #351
- OPA #2897, Cedarling decision logs — precedent against always-on full authz trace logging

---
*Research completed: 2026-07-29*
*Ready for roadmap: yes*
