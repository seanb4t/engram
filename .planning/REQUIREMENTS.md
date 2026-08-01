<!--
SPDX-License-Identifier: Apache-2.0
Copyright 2026 Sean Brandt
-->

# Requirements: engram — v0.12.x Headless Reach & Diagnosability

**Defined:** 2026-07-29
**Core Value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its
context, and wrong or stale memories can be corrected or superseded, so recall stays trustworthy
as the store grows.

**Milestone goal:** Make engram usable by agents that are **not** a top-level MCP client, and make
what the server decides and rejects legible to whoever is on the other end.

**ID convention:** this project uses semantic `REQ-<kebab-name>` IDs (continuing v0.8.x–v0.11.x),
not `CAT-NN`.

**Research:** `.planning/research/SUMMARY.md` (HIGH confidence; 4-dimension fan-out + synthesis).
Zero new Go dependencies are required for this entire milestone.

---

## v0.12.x Requirements

### Headless Client Lane (#343 — the milestone's structural spine)

- [x] **REQ-connect-bearer-identity**: A headless caller can authenticate to the ConnectRPC lane
  with a bearer token, verified by the **same composed** `auth.ChainVerifier` the MCP lane uses
  (OIDC user → client-credentials → static token). `withAuth`'s chain builder is extracted so both
  mount sites consume one verifier — never two independently-constructed chains that can drift.

- [x] **REQ-connect-token-expiry**: The Connect bearer path rejects a token whose
  `TokenInfo.Expiration` has passed. *(Closes a live gap: `newConnectSubjectInterceptor` calls
  `resolve()` directly and never routes through `mcpauth.RequireBearerToken`, whose private
  `verify()` is the only place expiration is enforced today. Reusing `ChainVerifier` alone makes
  `Expiration` decorative on this lane — including the static-token lane's long-lived sentinel,
  which exists solely to satisfy a check this lane would never run.)*

- [x] **REQ-connect-lane-provenance**: The resolver stamps an explicit, server-set marker recording
  **which lane** authenticated each request. The CSRF interceptor's exemption is decided from that
  marker alone and can never be influenced by attacker-controlled request content — not header
  presence, not cookie presence, not content type. A cookie-authenticated caller cannot exempt
  itself from CSRF by omitting `X-CSRF-Token`, and cannot self-declare the bearer lane. Bearer
  verification failure never falls through to the cookie lane.

- [x] **REQ-connect-headless-mount**: An operator can mount the Connect lane on a deployment with
  the web UI disabled, via a flag that defaults **off independently** of every existing UI and
  service-auth flag. A deployment that has no Connect surface today gains none on upgrade.

- [x] **REQ-cli-client-commands**: An agent with only a shell can `engram search`, `engram store`,
  and `engram list` against a remote server given a server URL and a token. The commands speak only
  the generated Connect stubs (`gen/go/engram/v1/engramv1connect`) and never import
  `internal/store`, `internal/authz`, or `internal/embed`.

- [x] **REQ-cli-agent-output**: CLI output is consumable by a non-interactive caller: structured
  JSON by default when not attached to a TTY, data on stdout and diagnostics on stderr, documented
  semantic exit codes mapped from engram's existing Connect error taxonomy, and no TTY prompt on
  any path.

- [x] **REQ-cli-credential-safety**: A token can be supplied without ever appearing in `argv`
  (env var or file), so it does not leak into `ps` output or shell history. TLS verification is on
  by default and cannot be disabled silently.

- [x] **REQ-cli-self-describing**: A bare invocation returns the full command / flag / exit-code
  catalog as structured output, so an agent can discover the surface without parsing help text.

### Cross-Spine Recall (#344)

- [ ] **REQ-cross-spine-search**: An agent can search curated memories across every scope it is
  permitted to see via `cross_spine=true` on `search_memory`, making `scope` optional — mirroring
  `search_discovery`'s existing `CrossSpine` semantics. Available at MCP↔Connect parity via an
  additive proto field. Opt-in; the default stays scope-confined.

- [x] **REQ-cross-spine-authz-verified**: Cross-spine search never widens the authorization filter.
  Before implementation, `Store.Search`'s filter-construction path is read end to end and it is
  confirmed **in writing** that the owner/authz `Must` clause is composed as a genuinely separate,
  unconditional entry from the scope clause — not a combined condition where omitting scope could
  drop part of the authz gate. Pinned by a two-owner isolation test against real Qdrant
  (testcontainers), never a mock. *(Research raised, and did not resolve, a disagreement here:
  architecture traced it as a one-line conditional in `ownerScopeFilter`; pitfalls warned memories
  lack the `discovery:*` namespace convention discoveries rely on, and that the single-scope path
  may have leaned on `scope` being non-empty as an unaudited narrowing signal. Resolved by
  verification, not analogy.)*

- [ ] **REQ-cross-spine-result-provenance**: A cross-spine result is visibly attributable to its
  originating scope, and the response reports which scopes were searched — so an agent can tell
  "found nothing here" from "searched everywhere I can see and found nothing."

### Rule Capture (#351)

- [ ] **REQ-rule-capture-investigation**: Determine why `store_rule` effectively never fires
  (one rule exists repo-wide against dozens of ordinary memories) by tracing actual invocation
  **attempts including failures** across the chain: the `curating-memory` skill's routing table →
  the session-start rules index → the `store_rule` tool description → the user-blessing gate.
  Deliverable is a written root cause distinguishing a mechanical/bug cause from a friction cause.

- [ ] **REQ-rule-capture-intervention**: Apply the fix the investigation identifies. If the cause
  is friction, the intervention must reduce **friction** without changing **who decides** — the
  user-blessed gate is a design invariant and any intervention that promotes a rule without
  explicit user instruction is out of scope, not a trade-off.

### Diagnosability (#394, #360, #347)

- [ ] **REQ-authz-decision-diagnostics**: An operator can see why the Cedar PDP allowed or denied a
  request, at debug level, through a field-allowlisted accessor over `authz.Decision`'s
  already-computed diagnostics. Logged on **both** the allow and deny paths (a deny-only reader
  looks complete while leaving unexpected allows undebuggable). Re-applies DEC-wot's owner-only
  disclosure rule at the reader, even though `cedar.Diagnostic` carries no PII fields by
  construction. Full Cedar expression traces are excluded.

- [ ] **REQ-validation-error-attribution**: An argument-validation rejection names the field that
  actually failed. Fixed at the branch-attribution mechanism, not by patching the one reported
  string — pinned by a matrix test with one case per single-field-invalid input asserting the
  correct field is named, rather than string-matching exact wording. *(Today an over-long `summary`
  reports `missing properties: ["content"]` while `content` is present and valid.)*

- [ ] **REQ-error-hint-envelope**: A rejection carries a structured remediation hint alongside the
  field attribution, so an agent's next attempt is informed rather than guessed. *(The error string
  is literally the next prompt the model acts on.)*

- [ ] **REQ-embed-provider-error-body**: A non-2xx response from the embeddings provider surfaces a
  bounded prefix of the provider's error body in the returned error alongside the status code, then
  drains and closes the body so the connection stays reusable. The chat/summarize lane is audited
  for the same gap and fixed if present, so the fix does not leave a matching bug behind the one
  being fixed.

### Operator Correctness (#350, #345)

- [ ] **REQ-per-lane-api-key**: An operator can point the chat/summarize client at a different
  provider **credential** than the embedder, mirroring the already-shipped base-URL split; empty
  means inherit the shared key. Byte-identical behavior when unset. Closes #350.

- [ ] **REQ-reindex-resume-tags**: `engram reindex --resume` re-embeds a record whose tags changed
  while its content did not. The target **lookup** is extended to fetch stored tags — not only the
  equality check — since tags are not fetched at all today, and comparing against an always-nil
  field either preserves the bug or silently defeats resume entirely. Tag comparison is
  order-independent.

- [ ] **REQ-reindex-stale-repair**: An operator can heal records that an earlier unpatched
  `--resume` run already skipped incorrectly, via a documented repair path — following the existing
  one-time-reconciliation command precedent (`migrate-remap-owner`, `backfill-short-ids`,
  `prune-expired`, `summarize-missing`).

---

## Future Requirements

Acknowledged, not in this roadmap.

### Cross-Spine Recall

- **REQ-cross-spine-coverage-receipt**: A full coverage receipt (scopes searched vs.
  skipped-with-reason, per-scope counts). Deferred — the scope space is small and enumerable today,
  so `REQ-cross-spine-result-provenance` covers the real need; revisit if scope counts grow.

- **REQ-cross-spine-scope-prefix**: Prefix/wildcard scope targeting (e.g. all `repo:*` spines but
  no workspace overlays). Deferred in favour of boolean parity with `search_discovery`; a second
  knob would need its interaction with `cross_spine` specified.

### Authorization

- **REQ-shared-read-tenant-scoping**: Narrow `shared` visibility from global cross-tenant to
  per-tenant. Deferred to full ABAC (ADR `engram-svct`).

- **REQ-cedar-partial-evaluation**: Replace the bucket-decision → store-filter pattern with Cedar
  residual compilation. Blocked upstream — not in cedar-go's stable core.

### Operations

- **REQ-reindex-boundary-enforcement**: Reject or quarantine reads whose embedder-identity hash
  mismatches live config. v0.10.x stamps the identity; enforcement is a separate decision.

---

## Out of Scope

Explicitly excluded. Anti-features from research are recorded here with their reasoning so a later
milestone does not relitigate them.

| Feature | Reason |
|---------|--------|
| **Auto-promoting a memory to a rule** | Violates the user-blessed gate — a locked design invariant. Independently, a 2025 study found agent-generated structure measurably underperforms human-authored structure, so this would also degrade quality, not just consent. |
| **Auto-extraction of memories** | Core design invariant since v0.8.x; explicit, user-blessed capture is what keeps recall zero-junk. |
| **Embedding-similarity auto-supersede** | v0.11.x shipped supersession deliberately with no similarity or write-through path; a single live head is enforced structurally. |
| **`cross_spine` defaulting to true** | A permissive default is how cross-scope leaks arrive; opt-in matches the milestone's own no-default-flip posture. |
| **Verbose Cedar expression-trace logging on every decision** | Cost and PII-leak risk; conflicts with DEC-wot. OPA's maintainers rejected the same thing for the same reasons. |
| **CLI `--auto-json` inferring format from response shape** | Non-deterministic format selection is a documented CLI-agent failure mode; format must be predictable from invocation context alone. |
| **A CLI that retries mutating calls on ambiguous failure without an idempotency key** | v0.11.x shipped `idempotency_key` precisely so replay-safety is explicit; a blind retry reintroduces the hazard it closed. |
| **#356 — UI TS codegen drift** | **Already shipped — verified against the live repo 2026-07-29.** `task proto:gen` vendors `gen/ts/` into `ui/src/lib/gen/`; CI's `buf` job enforces the drift check on that path; the vendored tree is byte-identical to `gen/ts`; and `ui/src/lib/gen/engram_pb.ts` is a deliberate hand-authored re-export barrel, not the drift surface the issue described. Close with rationale. |
| **#346 — base-URL join edge cases** | A deliberate non-fix: Phase 13 left query/fragment joins non-canonicalizing as operator-error scope, and Phase 26's `TestJoin` pins that behavior. Close with rationale. |
| **Prometheus `/metrics` scrape endpoint** | Telemetry is OTLP-gRPC only (DEC-dwi). |
| **New Go dependencies** | Research confirmed every capability lands on an existing seam — connect-go, `golang.org/x/oauth2/clientcredentials`, cedar-go, `log/slog`, OTel, and stdlib `io`/`net/http` cover the full surface. A new dependency in this milestone needs its own justification. |

---

## Defects Invisible to a Green Test Suite

This project's documented failure mode is defects that compile, vet, lint, and pass the full suite,
caught only by a reviewer reasoning from the contract. Research consolidated nine for this
milestone; each requirement above that carries one names its pinning test. Reviewers should treat
this as the review checklist, not the requirement text alone.

| Requirement | Invisible defect | Pinning test |
|-------------|------------------|--------------|
| REQ-connect-token-expiry | Happy-path tests never send an expired token; no linter flags "field exists, nothing reads it" | Feed a verifier stub returning `TokenInfo{Expiration: past}` with `err == nil`; assert rejection |
| REQ-connect-lane-provenance | Correct against every test sending *either* a valid cookie *or* a valid bearer — never the adversarial combination | Cookie caller omitting the CSRF header is still rejected; cookie caller cannot self-declare the bearer lane |
| REQ-connect-lane-provenance | Bearer failure silently falls through to cookie | Valid cookie + invalid bearer header simultaneously must not authenticate |
| REQ-connect-bearer-identity | Two independently-built chains drift; each lane's own mocked tests pass | Structural assertion that both mount sites call the same constructor |
| REQ-cross-spine-authz-verified | A mock can pass while the real Qdrant filter composition is wrong | Two owners, overlapping scope names, `cross_spine=true` with empty scope; owner B's private records never appear — real Qdrant |
| REQ-authz-decision-diagnostics | Deny-path logging "looks complete" while allows are silently unlogged | Field-allowlist test + assert both allow and deny paths emit |
| REQ-validation-error-attribution | Asserting the new exact string passes trivially while misattribution persists for every other field combination | Per-field matrix asserting the correct field is named, not exact wording |
| REQ-reindex-resume-tags | Comparing an always-nil tags field either preserves the bug or silently re-embeds everything — costlier, never an error | Content matches / tags differ → re-embed; content and tags both match → skip (paired positive control); plus same-elements-different-order |
| REQ-connect-headless-mount | Loosening the existing mount guard with an OR keeps every "UI enabled → mounted" test green even if the new condition is wrong | UI disabled AND headless flag unset must leave Connect unmounted, byte-for-byte today's behavior |

---

## Traceability

Populated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| REQ-connect-bearer-identity | v0.12.x Phase 1 | Complete |
| REQ-connect-token-expiry | v0.12.x Phase 1 | Complete |
| REQ-connect-lane-provenance | v0.12.x Phase 1 | Complete |
| REQ-connect-headless-mount | v0.12.x Phase 1 | Complete |
| REQ-cli-client-commands | v0.12.x Phase 2 | Complete |
| REQ-cli-agent-output | v0.12.x Phase 2 | Complete |
| REQ-cli-credential-safety | v0.12.x Phase 2 | Complete |
| REQ-cli-self-describing | v0.12.x Phase 2 | Complete |
| REQ-cross-spine-search | v0.12.x Phase 3 | Pending |
| REQ-cross-spine-authz-verified | v0.12.x Phase 3 | Complete |
| REQ-cross-spine-result-provenance | v0.12.x Phase 3 | Pending |
| REQ-rule-capture-investigation | v0.12.x Phase 6 | Pending |
| REQ-rule-capture-intervention | v0.12.x Phase 6 | Pending |
| REQ-authz-decision-diagnostics | v0.12.x Phase 4 | Pending |
| REQ-validation-error-attribution | v0.12.x Phase 4 | Pending |
| REQ-error-hint-envelope | v0.12.x Phase 4 | Pending |
| REQ-embed-provider-error-body | v0.12.x Phase 4 | Pending |
| REQ-per-lane-api-key | v0.12.x Phase 5 | Pending |
| REQ-reindex-resume-tags | v0.12.x Phase 5 | Pending |
| REQ-reindex-stale-repair | v0.12.x Phase 5 | Pending |

**Coverage:**

- v0.12.x requirements: 20 total
- Mapped to phases: 20
- Unmapped: 0 ✓

| Phase | Requirements |
|-------|--------------|
| v0.12.x Phase 1 — Shared Auth Chain & Connect Bearer Identity | 4 |
| v0.12.x Phase 2 — Headless CLI Client | 4 |
| v0.12.x Phase 3 — Cross-Spine Memory Recall | 3 |
| v0.12.x Phase 4 — Diagnosability | 4 |
| v0.12.x Phase 5 — Operator Config & Reindex Correctness | 3 |
| v0.12.x Phase 6 — Rule Capture — Investigation & Fix | 2 |

**Issue mapping:**

| Issue | Requirements |
|-------|--------------|
| #343 | REQ-connect-bearer-identity, REQ-connect-token-expiry, REQ-connect-lane-provenance, REQ-connect-headless-mount, REQ-cli-client-commands, REQ-cli-agent-output, REQ-cli-credential-safety, REQ-cli-self-describing |
| #344 | REQ-cross-spine-search, REQ-cross-spine-authz-verified, REQ-cross-spine-result-provenance |
| #351 | REQ-rule-capture-investigation, REQ-rule-capture-intervention |
| #394 | REQ-authz-decision-diagnostics |
| #360 | REQ-validation-error-attribution, REQ-error-hint-envelope |
| #347 | REQ-embed-provider-error-body |
| #350 | REQ-per-lane-api-key |
| #345 | REQ-reindex-resume-tags, REQ-reindex-stale-repair |
| #356 | *out of scope — already shipped, close with rationale* |

---
*Requirements defined: 2026-07-29*
*Last updated: 2026-07-29 after roadmap creation (Phases 1–6) — 20/20 requirements mapped*
