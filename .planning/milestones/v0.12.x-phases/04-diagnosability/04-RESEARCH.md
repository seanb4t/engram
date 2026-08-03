# Phase 4: Diagnosability - Research

**Researched:** 2026-08-01
**Domain:** Go structured-error design over two transports (MCP JSON-RPC / Connect RPC), Cedar (cedar-go v1.8.0) diagnostic disclosure, HTTP response-body handling for provider error bodies
**Confidence:** HIGH — every claim below is grounded in a `Read` of the live tree or the vendored dependency source in `$GOMODCACHE`, not inference. Where a claim is a judgment call rather than a fact, it is marked `[ASSUMED]` and listed in the Assumptions Log.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01** (log line emitted from `internal/store`, not `internal/authz`): `internal/store` emits it at the point it consumes a `Decision`. `internal/authz/authz.go:44-47` already designates this — the `diag` field "exists solely for future debug-level logging / OTel span attachment by internal/store." `internal/authz` emits zero `slog` calls today; that stays true. Reversibility: reversible.
- **D-02** (allowlist is policy IDs + error COUNT + decision/action/bucket, never raw Diagnostic): satisfied policy IDs from `Reasons`, the *count* of `Errors`, plus decision/action/bucket already in hand. Cedar error **messages** excluded (can embed entity values). No policy expression text, ever. Reversibility: costly.
- **D-03** (allowlist enforced STRUCTURALLY by a narrow accessor): `authz.Decision.diag` stays unexported; a dedicated exported accessor returns a narrow struct carrying exactly D-02's fields. Reversibility: reversible.
- **D-04** (both allowed and denied arms logged, at debug): Criterion 1 requires both. Reversibility: reversible.
- **D-05** (structured error type carries a machine-readable field name): rejections carry the failing field as structured data, not only prose. Reversibility: costly.
- **D-06** (sweep covers EVERY single-field rejection site, not a sample): every `validate*` function and every inline argument check in `internal/server/tools.go` that can reject on one field is enumerated and converted. Reversibility: reversible.
- **D-07** (matrix is table-driven, one case per single-field-invalid input): asserts the returned field identifier equals the deliberately-invalidated field. Reversibility: n/a — verification obligation.
- **D-08** (validation message TEXT reformatted field-first — user override, 2026-08-01): message text is normalized so the field leads. **Scope fence**: `e9yv53pmnv` concerns the MCP **401 auth body**, produced by the go-sdk's `RequireBearerToken`, NOT `internal/server/tools.go`'s argument validation. This decision covers argument-validation messages only; the 401 body must remain byte-identical, and the planner must verify that separation before reformatting anything. Reversibility: costly.
- **D-09** (hint is a machine-stable code plus human text, carried WITH the field in one envelope): the hint and D-05's field attribution travel together — one envelope, not two mechanisms. Reversibility: costly.
- **D-10** (hints authored per validation rule at the rejection site, no central lookup table): the constraint is known where the rejection happens; a central table drifts silently. Reversibility: reversible.
- **D-11** (Connect widens to semantically distinct standard error codes per failure class — user override, 2026-08-01): Connect/gRPC codes are a CLOSED enum — selecting among existing codes (`InvalidArgument`, `FailedPrecondition`, `OutOfRange`, `NotFound`, …), where today `internal/server/connectapi.go` collapses essentially everything to `CodeInvalidArgument` at `:149`, `:153`, `:160`, `:173`, `:230`, `:234`, `:243`. **MUST ship as a `feat!` commit** (or `BREAKING CHANGE:` footer). Verified against `release-please-config.json`: both `feat` and `feat!` bump MINOR to `0.12.0` under `bump-minor-pre-major: true` from `0.11.2` — the `!` is a changelog signal, not a version escalation. Reversibility: one-way.
- **D-12** (a hint never echoes the caller's rejected VALUE): names the field and states the constraint only. Same discipline as Phase 3's D-02 no-value-echo logging. Reversibility: reversible.
- **D-13** (reuse the chat lane's existing 4096-byte bound on the surfaced error-body prefix): `internal/summarize/summarize.go:181` already reads `io.LimitReader(resp.Body, 4096)`; embeddings copies that number. Reversibility: reversible.
- **D-14** (embeddings lane surfaces the body; BOTH lanes gain the drain): Surfacing is embeddings-only (`internal/embed/embed.go:248-249` discards the body entirely today; the chat lane already surfaces it). Draining is both — neither lane drains today; add a bounded discard after reading the prefix and before `Close` on both lanes. Reversibility: reversible.
- **D-15** (provider error body surfaced verbatim within the bound, not scrubbed): a provider's error body is the provider's own text, never caller data; the `Authorization` header is never echoed back. Planner must confirm no caller content reaches the embeddings request in a way a provider would reflect verbatim. Reversibility: reversible.
- **D-16** (bound the embeddings SUCCESS-path decode too, mirroring the sibling): `summarize.go:186-188` wraps its success decode in a 1 MiB `io.LimitReader`; `embed.go:252` decodes unbounded. Size the bound to `ENGRAM_EMBED_DIM` rather than copying 1 MiB blindly. Reversibility: reversible.

### Claude's Discretion

- Exact names for the diagnostics accessor, the structured validation-error type, and the hint code vocabulary.
- Which standard Connect code each validation-failure class maps to under D-11 — the planner proposes the mapping table; the shape is fixed, the specific assignments are not.
- Whether the hint code is a Go typed constant or a string, and how it serialises on each lane.
- The embeddings success-path decode bound under D-16.
- Whether the four fixes land as four plans or are grouped — they are independent, so wave structure is a planning judgment.

### Deferred Ideas (OUT OF SCOPE)

- Full Cedar expression traces at any log level — D-02's allowlist exists to make them unreachable.
- A configurable error-body bound — D-13 reuses the sibling's constant.
- Extending field attribution to the operator CLI commands (`reindex`, `prune-expired`, …) — revisit if operators ask.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-authz-decision-diagnostics | Operator sees why Cedar PDP allowed/denied, at debug, via a field-allowlisted accessor over `authz.Decision`; logged on both allow and deny; full traces excluded | `cedar.Diagnostic`/`DiagnosticReason`/`DiagnosticError` shape confirmed (Architecture Patterns §3); all production `Decision`-consumption call sites enumerated with exact per-request cardinality (Architecture Patterns §4); named policy IDs already exist for exactly this purpose (Code Examples) |
| REQ-validation-error-attribution | A rejection names the field that actually failed; matrix test, not string-matching | Full D-06 sweep inventory (Architecture Patterns §1) with a CRITICAL finding: several required-field rejections are enforced by the go-sdk's JSON-schema layer, structurally outside any `tools.go` code path (Common Pitfalls §1) |
| REQ-error-hint-envelope | A rejection carries a structured remediation hint alongside the field attribution | MCP wire-format capacity for structured error data proven empty (Common Pitfalls §2) — the envelope must be encoded inside the one string the MCP lane carries |
| REQ-embed-provider-error-body | Non-2xx embeddings response surfaces a bounded provider-error-body prefix + status, drains for reuse; chat/summarize lane audited for the same gap | `embed.go`/`summarize.go` read line-by-line (Architecture Patterns §5); existing sibling test pattern to mirror identified (Validation Architecture) |

</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Conventional Commits required (PR titles CI-validated); `main` protected — branch + PR only.
- SPDX Apache-2.0 header on every Go/Markdown file (`task license:check`).
- `task lint`/`task fmt` (golangci-lint, gofmt, dprint, yamlfmt, actionlint, rumdl) must be clean; `task` = lint + test.
- No viper, no database migrations, no cocogitto — config is koanf-based (`internal/config`), env-first `ENGRAM_` prefix with `--flag` overrides.
- Authorization is enforced in `internal/store`, never in handlers (standing invariant, ADR `engram-cdr1` refining LOCKED `DEC-cgb`) — D-01 is fully consistent with this: the log line reports a decision already made, it does not move where the decision is made.
- `curating-memory` skill and `docs-site/src/content/docs/` are the agent-facing surfaces obligated by convention `yaj7dqz9qq` — a new tool-facing contract (hint envelope, widened Connect codes) with no doc/skill update is an incomplete feature, not a follow-up (confirmed precedent: Phase 3 plan 03-05 did exactly this for `cross_spine`).
- File follow-ups for remaining work → GitHub Issues, not markdown TODOs.

## Summary

Four independent fixes share one discipline (bounded, structured, redaction-conscious disclosure), but they land in structurally different layers of the stack, and three of the four research questions surfaced a load-bearing constraint the CONTEXT.md decisions do not spell out. Read in order of how much they change the plan:

**The MCP wire format has no structured-error slot.** `github.com/modelcontextprotocol/go-sdk@v1.6.1/mcp/server.go:340-354` proves that when a `ToolHandlerFor` closure returns a non-nil `error`, the SDK discards whatever `*CallToolResult` the closure built (including any `StructuredContent` it set) and replaces it with a fresh result whose only content is `err.Error()` as plain text. There is no side channel. D-09's "machine-stable code" has no home except *inside that string* — the planner must pick a string convention (e.g. a `field=X code=Y: human text` prefix, or a JSON-encoded error string) that both a human and a parser can consume, because the SDK gives nothing else.

**D-06's stated sweep ("every `validate*` function and inline check in `tools.go`") does not reach the bug that motivated the requirement.** Issue #360's own repro (`store_memory`/`update_memory` misreporting `missing properties: ["content"]`) is a rejection from the go-sdk's automatic JSON-schema validation (`applySchema`, `mcp/tool.go:69-105`), which runs at `mcp/server.go:322-327` — **before** the `tools.go` closure is ever invoked. `storeArgs.Content`/`Scope`/`Source`/`Category` (and `searchArgs.Query`, `storeRuleArgs.Summary`, etc.) carry no `omitempty` tag, so the go-sdk's inferred schema marks them "required" and rejects a missing key with a bare, unstructured string — code that D-05's envelope cannot reach unless those fields are moved from schema-enforced-required to Go-enforced-required (relax the tag, add an explicit post-unmarshal check). This is the single most consequential finding in this research and is flagged loudly in Common Pitfalls §1 and must be resolved as a planning decision, not silently worked around.

**A live, un-flagged defect: several `tools.go`/`rules.go` validation functions already misclassify to `CodeInternal` on the Connect lane today**, not `CodeInvalidArgument`. `validateStoreDiscovery`/`validateCitations` (both reachable from the Connect-exposed `StoreDiscovery` and `StoreMemory` RPCs) return bare `fmt.Errorf(...)` with no `store.ErrInvalidArgument` wrap, so `connectError` (`internal/server/connecterror.go:49-85`) falls through every `errors.Is` case to the `default` branch — logging `"connect handler: unexpected error"` and returning a generic `"internal error"` to the caller. This is exactly the class of defect D-11's structured-code work should close, and D-06's sweep must include the wrap, not just the field name.

**Cedar's `Diagnostic` shape makes D-02's allowlist trivial and safe.** `cedar.Diagnostic{Reasons []DiagnosticReason, Errors []DiagnosticError}` (cedar-go v1.8.0 `types/authorize.go:46-63`) — `DiagnosticReason` is `{PolicyID, Position}` (policy source location, never caller data); only `DiagnosticError.Message` can embed entity values, and D-02 correctly excludes it. The four embedded policies already carry named, human-legible IDs (`own-records`, `shared-read`, `tenant-isolate`, `defense-empty-owner`) specifically for this future logging, per `internal/authz/policies.go:26-28`.

**D-04's "both arms" cost is a bounded per-request constant, not O(results).** Every production consumer of `authz.Decision` funnels through exactly two `Store` methods (`decideBucket`, `decideRecord`, `store.go:721-737`), each called once per invocation from the bulk-recall filter builders (`ownerOrSharedCondition` calls `decideBucket` exactly twice — `BucketOwn` + `BucketShared` — `store.go:685-686`) or the id-addressed gates (`GetReadable`/`getWritable`/`OwnedOrAbsent`, one `decideRecord` call each). None of these run inside a loop over result rows.

**Neither `embed.go` nor `summarize.go` drains a response body**, confirmed line-by-line; `embed.go` additionally discards the body entirely on the error path and decodes unbounded on the success path. `summarize.go`'s existing 4096-byte error-prefix pattern (`:180-183`) and 1 MiB success bound (`:186-189`) are the shapes to mirror, and an existing test (`TestSummarizeNon200IncludesStatusAndBody`, `summarize_test.go:132-148`) is the exact fixture pattern for the new embeddings-lane test.

**Primary recommendation:** treat this phase as three closely-related but separately-verified changes: (1) a `Store`-level Cedar-diagnostics log line at the two wrapper chokepoints, using a new narrow accessor; (2) a structured validation-error type authored at each `tools.go`/`rules.go`/`connectapi.go` rejection site, encoded as a parseable string on the MCP lane and as a typed Connect code + detail on the Connect lane, with an explicit, called-out decision about whether schema-level required-field rejections are in scope (they need a wire-schema change to be reachable at all); (3) a bounded read-and-drain helper shared by `embed.go` and `summarize.go`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Cedar decision logging | Store/Data layer (`internal/store`) | Authz/PDP (`internal/authz`, accessor only) | D-01 locks the log site in `internal/store`; `internal/authz` stays a pure decision function with a narrow read-only accessor, never a logger dependency |
| Field-attributed validation errors | API/Backend (`internal/server` — MCP tool closures, Connect handlers) | none | Both wire transports terminate in this package; the structured type must be constructible and error-mappable from both `tools.go`/`rules.go` (Go-level checks) and `connectapi.go` (Connect-only inline checks) |
| Remediation hint envelope | API/Backend (co-located with the validation error, D-10) | none | Hints are authored at the rejection site, not centrally — same tier as the validation error itself |
| Connect error-code mapping | API/Backend (`internal/server/connecterror.go`, `connectapi.go`) | CLI client (`cmd/engram/client_common.go` — already pre-groups the target codes) | The mapper is server-side; the CLI's `exitCodeForConnectErr` already treats `{InvalidArgument, FailedPrecondition, OutOfRange}` as one exit-code class, so D-11 is a compatible widening if it stays inside that trio |
| Provider error-body surfacing + drain | External Integration (`internal/embed`, `internal/summarize`) | none | Both are thin HTTP clients to an OpenAI-compatible endpoint; the fix is local to the two `Client.embed`/`Summarize` methods, no other tier is involved |

## Standard Stack

No new dependency of any kind is required. `go.mod` confirms every library this phase touches is already present:

| Library | Version | Purpose | Why no addition needed |
|---------|---------|---------|------------------------|
| `github.com/cedar-policy/cedar-go` | v1.8.0 [VERIFIED: go.mod:10] | Source of `Diagnostic`/`DiagnosticReason`/`DiagnosticError` D-02 reads | Already the sole PDP dependency since Phase 22 |
| `connectrpc.com/connect` | v1.20.0 [VERIFIED: go.mod:8] | Source of the closed `Code` enum D-11 selects from | Already the Connect transport dependency |
| `github.com/modelcontextprotocol/go-sdk` | v1.6.1 [VERIFIED: go.mod:18] | MCP tool registration/dispatch; source of the string-only error constraint | Already the MCP transport dependency |
| `log/slog` (stdlib) | go1.25+ toolchain | D-01's log line | Already used throughout `internal/server`, `internal/auth`, `internal/telemetry` |
| `io`, `net/http` (stdlib) | — | D-14's bounded drain | `io.LimitReader`/`io.Copy(io.Discard, …)` — no library needed |

**Installation:** none — zero new packages.

**Version verification:** confirmed directly from `go.mod` (not `npm view`/registry lookups — this is a Go project; the ecosystem-appropriate verification is reading `go.mod` against the vendored `$GOMODCACHE` source, both done live in this session).

## Package Legitimacy Audit

**Not applicable.** This phase adds zero new external packages (confirmed: `go.mod` unchanged is achievable — every capability is implemented with `cedar-go`, `connect-go`, `go-sdk`, and stdlib, all already present). `REQUIREMENTS.md`'s "Out of Scope" table states this milestone-wide: *"New Go dependencies — Research confirmed every capability lands on an existing seam."* No `package-legitimacy check` run is required; there is nothing to check.

## Architecture Patterns

### System Architecture Diagram

```
                     ┌─────────────────────────┐        ┌──────────────────────────┐
   MCP client         │ mcp.AddTool[In,Out]      │        │ Connect client            │
   (Authorization:    │ closure (tools.go)       │        │ (Connect RPC)             │
    Bearer <jwt>)      │                          │        │                           │
        │              │  1. applySchema()        │        │  1. otelconnect           │
        ▼              │     (go-sdk, BEFORE      │        │  2. accessLog             │
┌───────────────┐      │     closure runs) ───┐   │        │  3. subjectInterceptor    │
│ RequireBearer  │      │                       │   │        │     (401, Unauthenticated)│
│ Token (401,    │      │  2. closure runs:     │   │        │  4. csrfInterceptor       │
│ go-sdk/auth)   │─────▶│     callerFromContext │   │        │  5. validateInterceptor   │
└───────────────┘      │     deps.storeMemory/  │   │        │     (protovalidate, 400)  │
        NOT tools.go    │     searchMemory/etc.  │   │        │  6. connectapi.go handler │
        (D-08 fence)    │        │               │   │        │     inline checks:        │
                        │        ▼               │   │        │     created_after/before, │
                        │  validate*()/inline    │   │        │     cursor_mode∧offset,   │
                        │  checks (tools.go,     │◀──┼────────┼──── effectiveSearchScope  │
                        │  rules.go) ─────┐      │   │        │        │                  │
                        │        │        │      │   │        │        ▼                  │
                        │        ▼        │      │   │        │  a.d.storeMemory/         │
                        │  deps.* (shared │      │   │        │  searchMemory/etc.        │
                        │  business logic)│      │   │        │  (SAME functions as MCP)  │
                        │        │        │      │   │        │        │                  │
                        │        ▼        │      │   │        │        ▼                  │
                        │  internal/store │◀─────┼───┼────────┼────────┘                  │
                        │  .Search/List/  │      │   │        │  connectError(err)         │
                        │  Get/Update ... │      │   │        │  (connecterror.go:49-85)   │
                        │        │        │      │   │        │  switch errors.Is(...):    │
                        │        ▼        │      │   │        │   ErrNotFound → NotFound    │
                        │  decideBucket/  │      │   │        │   ErrInvalidArgument →      │
                        │  decideRecord   │      │   │        │     InvalidArgument         │
                        │  (store.go:     │      │   │        │   errRuleImmutable/         │
                        │  721-737)       │      │   │        │   errStaleSummary →         │
                        │        │        │      │   │        │     FailedPrecondition      │
                        │        ▼        │      │   │        │   UNWRAPPED bare error →    │
                        │  authz.PDP.     │      │   │        │     CodeInternal (BUG,      │
                        │  Decide{Bucket, │      │   │        │     see Pitfall 3)          │
                        │  Record}        │      │   │        └──────────────┬────────────┘
                        │  (cedar.Author- │      │   │                       │
                        │  ize) ──────────┼──────┘   │                       ▼
                        │        │ D-01 log site      │              Connect wire response
                        │        ▼ (allow+deny,        │              (typed Code + detail)
                        │  slog.Debug via     debug)   │
                        │  narrow accessor)             │
                        └────────────────────────────────┘
                                     │
                                     ▼
                          err.Error() as the ONLY
                          payload on a rejected MCP
                          tool call (CallToolResult.
                          Content[0].Text) — no
                          StructuredContent on error
                          (mcp/server.go:340-354)

  Embeddings/chat lane (separate from the above; no auth/validation overlap):

  internal/embed.Client.embed()          internal/summarize.Client.Summarize()
    resp, _ := c.http.Do(req)              resp, _ := c.http.Do(req)
    defer resp.Body.Close()  ◀── no drain    defer resp.Body.Close()  ◀── no drain (D-14)
    if !2xx: body DISCARDED (D-14 gap)       if !2xx: io.LimitReader(4096) + body (OK, D-13)
    else: json.Decode(resp.Body) UNBOUNDED   else: json.Decode(io.LimitReader(1MiB)) (OK, D-16 mirror)
    (D-16 gap)
```

### Recommended Project Structure

No new files/packages are structurally required; the four fixes are additive edits to existing files:

```
internal/authz/
├── authz.go            # add the D-03 narrow accessor (e.g. Decision.LogFields()/Diagnose())
internal/store/
├── store.go             # D-01 log line at decideBucket/decideRecord (store.go:721-737)
internal/server/
├── tools.go              # D-05/D-06 structured error type + sweep of validate*/inline checks
├── rules.go               # D-05/D-06 sweep of validateStoreRule/validateRuleSummary/listRules
├── connectapi.go           # D-06 sweep of the 7 connectapi.go-native inline checks; D-11 code selection
├── connecterror.go          # D-11 mapping table; fix the CodeInternal misclassification (Pitfall 3)
internal/embed/
├── embed.go                  # D-14/D-15/D-16: surface body, drain both paths, bound success decode
internal/summarize/
├── summarize.go                # D-14: add the drain only (surfacing already correct)
docs-site/src/content/docs/
├── reference/tools.md            # per-convention yaj7dqz9qq: document the hint envelope + widened codes
skill/engram/skills/curating-memory/
├── SKILL.md                       # if the hint vocabulary should shape agent retry behavior
```

### Pattern 1: The `applySchema`-before-handler chokepoint (why some rejections never reach `tools.go`)

**What:** `mcp.AddTool[In,Out]` (go-sdk `mcp/server.go:503-508`) wraps a typed closure in `toolForErr` (`mcp/server.go:285-390`). The generated handler (`mcp/server.go:315-390`) does, in order: (1) `applySchema(input, inputResolved)` at `:322` — validates the raw JSON `map[string]any` against the struct-derived JSON Schema and returns immediately on failure (`:323-327`), constructing the `CallToolResult` itself; (2) unmarshal into the typed `In` struct (`:330-337`); (3) **only then** call the registered closure (`:340`).

**When it matters:** any struct field lacking an `omitempty` JSON tag is marked `"required"` in the auto-inferred schema (jsonschema-go `infer.go`), and a missing key is rejected at step (1) — the `tools.go` closure body, including any `validate*` function it calls, never executes. Confirmed required (no `omitempty`) fields include: `storeArgs.Content/Scope/Source/Category` (`tools.go:430-433`), `searchArgs.Query` (`tools.go:535`), `updateArgs.ID/Content` (`tools.go:571,581` — the `Content *string` comment at `:572-580` explicitly documents "The MCP tool still requires content on every call (schema unchanged: no omitempty)"), `storeDiscoveryArgs.Content/Kind/Citations/Scope` (`tools.go:600-603`), `storeRuleArgs.Content/Scope/Summary` (`rules.go:26-28`), `idArgs.ID`/`setVisibilityArgs.ID,Shared` (`tools.go:566-567,634-635`).

**Example (the actual `applySchema` error construction — go-sdk source):**
```go
// Source: github.com/modelcontextprotocol/go-sdk@v1.6.1/mcp/server.go:315-327
th := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
    var input json.RawMessage
    if req.Params.Arguments != nil {
        input = req.Params.Arguments
    }
    var err error
    input, err = applySchema(input, inputResolved)
    if err != nil {
        var errRes CallToolResult
        errRes.SetError(fmt.Errorf("validating \"arguments\": %v", err))
        return &errRes, nil // <-- tools.go closure NEVER runs
    }
    // ... unmarshal into In, THEN call the closure
```

**How to avoid mis-scoping D-06:** the planner must explicitly decide, as its own decision (not silently): either (a) accept that schema-level required-field rejections stay unstructured strings (D-06's sweep only covers Go-level checks, and the plan should say so), or (b) relax `omitempty` on the fields that need Go-level structured rejection and add an explicit `validate*` presence check for each — which is itself a wire-schema change (tools advertise fewer JSON-Schema-required fields) that changes what MCP clients see in `tools/list`. Option (b) is the only way to make issue #360's *own* repro (an over-long `summary` misreported as missing `content`) covered by this phase's fix, because that specific repro is a required-field rejection.

### Pattern 2: MCP's error payload is a single string; there is no structured side channel

**What:** In `toolForErr`'s generated handler, when the closure returns `(res, out, err)` with `err != nil`, the code path is:
```go
// Source: github.com/modelcontextprotocol/go-sdk@v1.6.1/mcp/server.go:340-354
res, out, err := h(ctx, req, in)
if err != nil {
    if wireErr, ok := err.(*jsonrpc.Error); ok {
        return nil, wireErr
    }
    var errRes CallToolResult
    errRes.SetError(err)   // <-- res (whatever the closure built, incl. StructuredContent) is DISCARDED
    return &errRes, nil
}
```
`CallToolResult.SetError` (`mcp/protocol.go:132-138`) sets `Content = []Content{&TextContent{Text: err.Error()}}` and `IsError = true`. `StructuredContent` is populated **only** on the success path (`:356-390`, unconditionally overwriting whatever the closure set, from the `out` value) — never on the error path.

**When to use:** always, for MCP — this is not a design choice, it's the SDK's fixed behavior for `AddTool[In,Out]`-registered tools (which is what every engram tool uses; verified `tools.go:1535` onward).

**Consequence for D-09:** the hint code and field name must both be encoded inside the single string `err.Error()` returns. Two viable conventions: (a) a stable prefix grammar the client parses, e.g. `field=summary code=E_TOO_LONG: summary exceeds 500 bytes (got 720)`; (b) make `err.Error()` itself a compact JSON object, e.g. `{"field":"summary","hint_code":"E_TOO_LONG","message":"..."}`, which any JSON-capable agent parses trivially and a human still reads (ugly but legible). Either is a legitimate `[ASSUMED — Claude's Discretion]` choice; what is NOT legitimate is designing a `StructuredOutput`/`Out`-type-carried error field and expecting it to reach the MCP wire on the error path — it structurally cannot, per the code above.

### Pattern 3: `decideBucket`/`decideRecord` are the two DRY chokepoints for D-01's log line

**What:** every production path from `internal/store` into `internal/authz` goes through exactly two `(*Store)` methods:
```go
// Source: internal/store/store.go:718-737
func (s *Store) decideBucket(owner, kind string, action authz.Action, bucket authz.Bucket) authz.Decision {
    if s.decideBucketHook != nil {
        return s.decideBucketHook(owner, kind, action, bucket)
    }
    return s.authz.DecideBucket(owner, kind, action, bucket)
}

func (s *Store) decideRecord(owner, kind string, action authz.Action, memoryOwner, category, visibility, scope string) authz.Decision {
    if s.decideRecordHook != nil {
        return s.decideRecordHook(owner, kind, action, memoryOwner, category, visibility, scope)
    }
    return s.authz.DecideRecord(owner, kind, action, memoryOwner, category, visibility, scope)
}
```
All six production call sites funnel through these two wrappers: `ownerOrSharedCondition` (`store.go:685-686`, TWO `decideBucket` calls — `BucketOwn`, `BucketShared` — building the Search/List filter, once per request), `ownerOnlyCondition` (`store.go:712`, one `decideBucket` call, used by `ListScheduled`), `GetReadable` (`store.go:1532`), `getWritable` (`store.go:1560`), `OwnedOrAbsent` (`store.go:1595`), and a `DeleteAll` gate (`store.go:2034`).

**When to use:** placing D-01's `slog.DebugContext` call inside `decideBucket`/`decideRecord` themselves (rather than at all six call sites) logs every production consumption of a `Decision` exactly once, with `action` (and `bucket`, for the bucket variant) already in scope as literal parameters — satisfying D-02's "action and bucket already in hand" without threading anything new through the six callers. Caveat: `decideRecord` has no `bucket` concept (it is per-record), so D-02's field list ("policy IDs, error count, decision, action, and bucket") needs the `bucket` field made optional/omitted on that arm — a planning decision, not a blocker.

**Per-request cardinality (answers D-04's cost question):** `ownerOrSharedCondition` and `ownerOnlyCondition` are each called exactly once per `Search`/`List`/`ListScheduled` invocation (they build a filter *condition*, not a per-record gate) — confirmed by reading their call sites and doc comments (`store.go:666-698,700-716`: *"bucket-level decisions only, compiled into the Qdrant filter; no per-record Cedar eval on bulk paths"*, matching ADR `engram-cdr1`). `GetReadable`/`getWritable`/`OwnedOrAbsent` are each single-record id-addressed operations with exactly one `decideRecord` call. **Debug-level logging of both arms is therefore O(1) per request in every case — at most 2 log lines for a bulk Search/List call, 1 for an id-addressed op — never O(result count).**

### Pattern 4: `cedar.Diagnostic`'s shape makes D-02 exactly expressible

```go
// Source: github.com/cedar-policy/cedar-go@v1.8.0/types/authorize.go:45-63
type Diagnostic struct {
    Reasons []DiagnosticReason `json:"reasons,omitempty"`
    Errors  []DiagnosticError  `json:"errors,omitempty"`
}
type DiagnosticReason struct {
    PolicyID PolicyID `json:"policy"`
    Position Position `json:"position"`
}
type DiagnosticError struct {
    PolicyID PolicyID `json:"policy"`
    Position Position `json:"position"`
    Message  string   `json:"message"`
}
```
`cedar.Authorize(policies, entities, req) (Decision, Diagnostic)` (`authorize.go:18`) is what `authz.DecideRecord` calls (`internal/authz/authz.go:69`). `Reasons[i].PolicyID` is a policy identifier only (`Position` is the *policy source's* file/line/column, never caller data). `DiagnosticError.Message` is the only field that can embed evaluated entity values — D-02 correctly excludes it and keeps only `len(diag.Errors)`. The accessor D-03 calls for can therefore be implemented as:
```go
type LogFields struct {
    Allow      bool
    PolicyIDs  []string
    ErrorCount int
}
func (d Decision) LogFields() LogFields {
    ids := make([]string, len(d.diag.Reasons))
    for i, r := range d.diag.Reasons {
        ids[i] = string(r.PolicyID)
    }
    return LogFields{Allow: d.Allow, PolicyIDs: ids, ErrorCount: len(d.diag.Errors)}
}
```
This is a **sketch for a new accessor that does not exist yet** — no line of this snippet is copied from the repo; it demonstrates D-02/D-03's shape against the confirmed `Diagnostic` fields above, and every value it reads (`Reasons[i].PolicyID`, `len(d.diag.Errors)`) is directly backed by the cedar-go struct fields quoted above.

### Anti-Patterns to Avoid

- **Logging `DiagnosticError.Message` at any level, even truncated.** It is Cedar's own evaluated-condition error text and can contain entity attribute values (e.g. `"error evaluating condition: entity Memory::\"...\" attribute owner..."` shape) — this is exactly what D-02 excludes. Log the count only.
- **Pattern-matching on `err.Error()` text to recover structured fields for the Connect mapper.** Now that a structured type is being introduced anyway (D-05), `connectError`/`connectapi.go` should switch on the structured type's field/class, not re-parse strings — string-matching is what created the `#360`-style misattribution class in the first place (see `PITFALLS.md`'s framing of the requirement, and Common Pitfalls §1 below).
- **Adding a new sentinel `errors.New(...)` per validation message without wrapping `store.ErrInvalidArgument` (or the new structured type).** `validateStoreDiscovery`/`validateCitations`/parts of `validateStoreRule` already made this mistake (Common Pitfalls §3) — every new/touched validation error must be reachable by `connectError`'s switch, or it silently becomes `CodeInternal`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bounded HTTP body read + drain | A custom buffered-reader wrapper per client | `io.LimitReader(resp.Body, N)` for the surfaced prefix + `io.Copy(io.Discard, resp.Body)` (bounded implicitly by `defer resp.Body.Close()` terminating the read) for the remainder — stdlib `io`, already the pattern at `summarize.go:181` | `summarize.go` already ships the exact idiom; duplicating it differently in `embed.go` would be the same "fixed one lane, not its sibling" class of bug the codebase has hit before (the doubled-`/v1` URL-join bug, `internal/openaiurl`) |
| Cedar diagnostic serialization | A custom Cedar-diagnostic-to-JSON encoder | The narrow `LogFields`-shaped accessor (D-03) feeding `slog`'s structured `Any`/`Group` args | `cedar.Diagnostic` already has clean, small, JSON-tagged sub-structs (`DiagnosticReason`, `DiagnosticError`) — no need to write a serializer, just select fields |
| Connect error-code selection logic | A second, parallel string-matching mapper in `connectapi.go` alongside `connecterror.go`'s existing `errors.Is` switch | Extend the ONE existing `connectError` switch (`connecterror.go:49-85`) with new cases keyed on the new structured error type/sentinel | The switch already exists and is the codebase's own established single-mapper convention (its own doc comment: "never a hand-rolled per-handler error mapping") |

**Key insight:** every "don't hand-roll" item in this phase already has a shipped, working sibling in the same codebase (the summarize error-body pattern, the `cedar.Diagnostic` struct, the `connectError` switch) — the risk in this phase is not missing a library, it's failing to reuse the sibling correctly (the exact defect class D-14's audit clause was written to catch).

## Common Pitfalls

### Pitfall 1: D-06's literal scope ("every `validate*`/inline check in `tools.go`") does not cover the bug that created REQ-validation-error-attribution

**What goes wrong:** the plan implements a structured field-attribution type across every `validate*` function and inline check in `tools.go`/`rules.go`, ships the matrix test, and declares the requirement done — while `store_memory`'s own flagship bug (`#360`: an over-long `summary` reported as `missing properties: ["content"]`) is untouched, because that specific rejection is produced by go-sdk schema validation (Pattern 1 above), not by any code the sweep touched.

**Why it happens:** `storeArgs`, `searchArgs`, `storeDiscoveryArgs`, `storeRuleArgs`, `updateArgs`, `idArgs`, `setVisibilityArgs` all have fields with no `omitempty` tag, which the go-sdk's schema inference marks `"required"` and enforces BEFORE the `tools.go` closure runs (confirmed at `mcp/server.go:322-327`). No amount of editing `validate*` functions changes this, because those functions are never reached for a missing required field.

**How to avoid:** the plan MUST state explicitly whether schema-level required-field rejections are in scope. If yes, it needs an additional task class: relax the affected field(s) to `omitempty` (a wire-schema change — document it, since `tools/list` output changes) and add an explicit Go-level presence check that returns the structured error type. If no, document clearly in the plan that D-06's sweep is scoped to Go-level checks only, and that the original `#360` repro (required-field omission) is a known, deliberately-deferred residual — this is a legitimate scope decision but it must be a **decision**, not an oversight.

**Warning signs:** a matrix test case for "content missing" that constructs a valid JSON payload with `content` OMITTED (not empty-stringed) will still hit the schema layer and get the OLD unstructured string, not the new structured type — if that test is written as "empty string" instead of "key absent," it will pass without exercising the actual gap.

### Pitfall 2: MCP has no structured-error wire slot — do not design the envelope assuming one

**What goes wrong:** the plan designs `StructuredContent`/an `Out`-typed error payload for MCP tool calls, tests it against a raw `deps.*` unit test (which never goes through `toolForErr`), and it appears to work — then breaks/vanishes in a real MCP round-trip because `toolForErr` discards it (Pattern 2).

**Why it happens:** unit tests calling `deps.storeMemory` etc. directly bypass the SDK's `applySchema`/`toolForErr` wrapping entirely, so a test asserting "the returned error has a `Field` accessor" passes regardless of what actually crosses the wire.

**How to avoid:** any test claiming to verify the MCP-lane envelope must go through the actual `mcp.AddTool`-registered handler (or at minimum construct the equivalent `err.Error()` string and assert on it), not call `deps.*` directly and inspect a Go error's fields. The Connect lane, by contrast, genuinely can carry structured detail (`connect.NewError(code, err)` plus `connect.ErrorDetail` — not investigated in depth here since neither CONTEXT.md nor the requirements ask for Connect error details beyond the code; flagged in Open Questions).

### Pitfall 3: `validateStoreDiscovery`/`validateCitations`/parts of `validateStoreRule` are UNWRAPPED and already misclassify to `CodeInternal` on Connect — live today

**What goes wrong:** a `StoreDiscovery` or `StoreMemory` Connect RPC call with bad citations/content/kind returns `connect.CodeInternal` with the generic body `"internal error"` (the caller's real validation problem is logged server-side via `slog.ErrorContext(ctx, "connect handler: unexpected error", "error", err)` and never reaches them) — confirmed by reading `connectError`'s switch (`connecterror.go:49-85`: no case matches a bare `fmt.Errorf`, so it falls to `default`) against `validateStoreDiscovery`/`validateCitations` (`tools.go:638-687`, no `store.ErrInvalidArgument` wrap anywhere) and `validateStoreRule`'s content/scope checks (`rules.go:56-67`, likewise unwrapped — `validateRuleSummary` at `rules.go:79-90` is correctly wrapped, so this is an inconsistency WITHIN one file, not a uniform gap).

**Why it happens:** these functions predate the Connect write lane's error-mapping discipline and were never revisited when `connectError`'s sentinel-based switch was introduced.

**How to avoid:** the D-06 sweep must wrap (or otherwise make classifiable by) every one of these errors — this is a natural side effect of introducing the D-05 structured type (if `connectError` switches on the new type instead of `errors.Is` sentinels, the wrap becomes unconditional and this bug disappears by construction). Call this out explicitly as a fix, not merely a refactor, since it changes live Connect-lane behavior (arguably itself a bug fix, not a breaking change, since `CodeInternal` was never a documented contract for these inputs).

**Warning signs:** any existing Connect test asserting `CodeInternal` for a bad-citation/bad-content `StoreDiscovery`/`StoreMemory` call would need to flip to `CodeInvalidArgument` (or whatever D-11 assigns) — search for such assertions before touching `connectError` and update them, or they will fail for the right reason and look like a regression.

### Pitfall 4: combination-of-fields checks cannot be forced into D-05's single-field envelope

**What goes wrong:** a matrix-test author tries to write a single-field-invalid case for `parseWindow`'s `"not_before must be strictly before not_after"` (`tools.go:492-493`, two fields) or `connectapi.go`'s `"cursor_mode is mutually exclusive with offset"` (`connectapi.go:159-161`, two fields, Connect-only — MCP's `listArgs` has no `Offset` field at all, confirmed `tools.go:546-556`) and either can't make it single-field-invalid, or attributes it to an arbitrary one of the two fields, defeating the matrix's "correct field" assertion.

**Why it happens:** D-05/D-07 assume a rejection names ONE field; some existing checks are inherently relational.

**How to avoid:** CONTEXT.md already anticipates this ("those cannot name a single field and need their own treatment" — research question framing). The plan should carry an explicit second envelope shape (e.g. `fields: []string` instead of `field: string`, or a distinct `kind: "combination"` on the same envelope) for: `parseWindow`'s not_before/not_after-both-empty (`tools.go:469-471`), not_before/not_after ordering (`tools.go:492-493`), and `connectapi.go`'s cursor_mode/offset (`connectapi.go:159-161`, note this one is OUTSIDE `tools.go` and therefore outside D-06's literal text — flag as a scope question, see Open Questions).

### Pitfall 5: `internal/store` has zero existing `slog` usage — D-01 is a genuinely new import, not "add one line"

**What goes wrong:** the plan estimates D-01 as trivial ("just call `slog.DebugContext` where the Decision is consumed") without accounting for the fact that `internal/store/store.go` imports no logging package today (`rg "\"log/slog\"|slog\\."  internal/store/store.go` returns zero matches, confirmed this session) — unlike `internal/server/tools.go`, which already has 12+ `slog.*` call sites.

**Why it happens:** the phase description reads as "connect an already-computed value to a reader" (per `research/SUMMARY.md`), which undersells that this is `internal/store`'s first-ever logging statement.

**How to avoid:** no functional blocker — `log/slog` is stdlib, trivially importable — but the plan should note this explicitly so a reviewer isn't surprised by a new import in a package that has historically stayed logging-free (a deliberate choice per its own package doc: *"internal/authz emits zero slog calls today"* is explicit about authz; store's silence looks incidental by comparison, but the fix is still correct per D-01).

## Runtime State Inventory

Not applicable — this phase is not a rename/refactor/migration phase. No stored data, live service config, OS-registered state, secrets, or build artifacts are renamed or relocated. Confirmed: all four fixes are additive/corrective edits to existing Go source and doc files; no schema migration, no collection rename, no external service reconfiguration.

## Code Examples

### The four already-embedded, named Cedar policy IDs (for D-02's `Reasons[i].PolicyID` values)

```go
// Source: internal/authz/policies.go:26-34 (read live, verbatim)
// named ids make debug-level diagnostic logging actually useful for
// operators, instead of anonymous "policy0"/"policy1" auto-ids.
var policyFiles = map[string]cedar.PolicyID{
    "policies/own_records.cedar":         "own-records",
    "policies/shared_read.cedar":         "shared-read",
    "policies/tenant_isolate.cedar":      "tenant-isolate",
    "policies/defense_empty_owner.cedar": "defense-empty-owner",
}
```
This comment (written well before this phase) is direct evidence the named-ID scheme was deliberately set up in anticipation of exactly this logging work.

### The already-shipped D-13/D-16 shape to mirror in `embed.go` (from `summarize.go`)

```go
// Source: internal/summarize/summarize.go:179-189 (read live, verbatim)
defer func() { _ = resp.Body.Close() }()
if resp.StatusCode != http.StatusOK {
    body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
    return "", fmt.Errorf("chat completions: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
var out chatResp
// Bound the success-path decode too: a one-line summary response is tiny, so
// cap at 1 MiB to keep a misbehaving gateway from forcing an unbounded read.
if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
    return "", err
}
```
Note: even this shipped pattern does NOT drain the remainder after the `LimitReader` read — D-14's drain gap applies to `summarize.go` too, exactly as CONTEXT.md states. Adding the drain is an ADDITIONAL line after each of these two blocks (surfacing block and success-decode block), not a rewrite.

### The current `embed.go` gap (both D-14 and D-16, verbatim)

```go
// Source: internal/embed/embed.go:243-259 (read live, verbatim)
resp, err := c.http.Do(req)
if err != nil {
    return nil, err
}
defer func() { _ = resp.Body.Close() }()
if resp.StatusCode != http.StatusOK {
    return nil, fmt.Errorf("embeddings: status %d", resp.StatusCode) // body never read at all
}
var out embedResp
if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { // unbounded
    return nil, err
}
```

### `connectError`'s existing single-mapper switch (extension point for D-11)

```go
// Source: internal/server/connecterror.go:49-85 (read live, verbatim)
func connectError(ctx context.Context, err error) error {
    if err == nil {
        return nil
    }
    switch {
    case errors.Is(err, store.ErrNotFound):
        return connect.NewError(connect.CodeNotFound, err)
    case errors.Is(err, store.ErrInvalidArgument):
        return connect.NewError(connect.CodeInvalidArgument, err)
    case errors.Is(err, errRuleImmutable):
        return connect.NewError(connect.CodeFailedPrecondition, err)
    case errors.Is(err, errStaleSummary):
        return connect.NewError(connect.CodeFailedPrecondition, err)
    case errors.Is(err, store.ErrAmbiguousShortID):
        return connect.NewError(connect.CodeFailedPrecondition, err)
    // ... ErrIdempotencyConflict -> AlreadyExists, ErrAlreadySuperseded -> FailedPrecondition,
    // context.Canceled -> Canceled, context.DeadlineExceeded -> DeadlineExceeded
    default:
        slog.ErrorContext(ctx, "connect handler: unexpected error", "error", err)
        return connect.NewError(connect.CodeInternal, errors.New("internal error"))
    }
}
```
D-11's new cases are added as additional `case`s here (switching on the new structured validation-error type/sentinel), not a parallel mapper.

### The CLI's exit-code grouping that already anticipates D-11's trio (compatibility constraint)

```go
// Source: cmd/engram/client_common.go:237-249 (read live, verbatim)
func exitCodeForConnectErr(err error) int {
    switch connect.CodeOf(err) {
    case connect.CodeUnauthenticated, connect.CodePermissionDenied:
        return exitAuth
    case connect.CodeNotFound:
        return exitNotFound
    case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeOutOfRange:
        return exitUsage
    case connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeCanceled:
        return exitUnavailable
    default:
        return exitGeneric
    }
}
```
If D-11's mapping table selects codes ONLY from `{CodeInvalidArgument, CodeFailedPrecondition, CodeOutOfRange}`, the Phase 2 CLI needs **zero changes** — every validation-failure class still exits `2` (`exitUsage`). If the table reaches outside this trio (e.g. proposes `CodeNotFound` for some validation class, which would be semantically wrong anyway since `NotFound` already means id-addressed-record-absent per DEC-xa6), the CLI's exit-code table AND `docs-site/src/content/docs/guides/cli.md`'s exit-code table (`:52-61`) both need a coordinated update — call this out as a cross-cutting dependency if it happens.

### D-11's proposed mapping table (Claude's Discretion — proposal, not a lock)

| Failure class | Example (from the sweep) | Proposed Connect code | Rationale |
|---|---|---|---|
| Required field absent / malformed | `content is required`, `kind must be "map" or "fact"` | `CodeInvalidArgument` | The existing, correct default — a genuinely malformed argument |
| Value out of an explicit numeric/length bound | `content too large`, `too many citations`, `idempotency_key too large` | `CodeOutOfRange` | Distinct semantic: the shape is right, the magnitude isn't — matches gRPC's own guidance for `OUT_OF_RANGE` ("client specified an invalid range") |
| State/relationship precondition (not a single value) | `not_before must be strictly before not_after`, `errStaleSummary`, `errRuleImmutable`, `cursor_mode is mutually exclusive with offset` | `CodeFailedPrecondition` | Matches the codebase's OWN existing precedent — `errRuleImmutable`/`errStaleSummary` already map here (`connecterror.go:58-61`) — this class already exists, D-11 just needs to widen the trigger set consistently |
| Malformed timestamp / enum value | `created_after: %w` (parse failure), `state must be one of scheduled\|expired\|all` | `CodeInvalidArgument` | A parse/enum failure is the textbook `InvalidArgument` case |

This table is a starting proposal for the planner to refine against the full sweep inventory below — CONTEXT.md is explicit that "the shape of the decision is fixed, the specific assignments are not."

## Full D-06 Sweep Inventory (the criterion-2 matrix's row set)

**Category A — Go-level checks in `tools.go`/`rules.go` (single field, structurally reachable by a `tools.go`-scoped D-05 fix):**

| # | Site | Field | Current message | Wrapped w/ `store.ErrInvalidArgument`? |
|---|------|-------|------------------|----|
| 1 | `tools.go:639-640` `validateStoreDiscovery` | `content` | `"content is required"` | No — Pitfall 3 |
| 2 | `tools.go:642-643` `validateStoreDiscovery` | `content` | `"content too large: %d bytes (max %d)"` | No |
| 3 | `tools.go:645-646` `validateStoreDiscovery` | `kind` | `"kind must be \"map\" or \"fact\", got %q"` | No |
| 4 | `tools.go:648-649` `validateStoreDiscovery` | `scope` | `"scope is required"` | No |
| 5 | `tools.go:651-652` `validateStoreDiscovery` | `scope` | `"scope must be a discovery scope (start with \"discovery:\"), got %q"` | No |
| 6 | `tools.go:667-668` `validateCitations` (minCount=1) | `citations` | `"at least one citation is required"` | No |
| 7 | `tools.go:669-670` `validateCitations` (minCount>1, currently dead — only called w/ 0 or 1) | `citations` | `"at least %d citation(s) required"` | No |
| 8 | `tools.go:672-673` `validateCitations` | `citations` | `"too many citations: %d (max %d)"` | No |
| 9 | `tools.go:676-677` `validateCitations` | `citations[i].kind` | `"citation %d: kind must be one of file\|commit\|url\|repo, got %q"` | No |
| 10 | `tools.go:679-680` `validateCitations` | `citations[i].ref` | `"citation %d: ref is required (the source anchor)"` | No |
| 11 | `tools.go:682-683` `validateCitations` | `citations[i].excerpt` | `"citation %d: excerpt too large: %d bytes (max %d)"` | No |
| 12 | `tools.go:776` `checkIdempotentReplay` | `idempotency_key` | `"idempotency_key too large: %d bytes (max %d)"` | **Yes** |
| 13 | `tools.go:1108-1109` `listScheduled` | `created_after` | `"created_after: %w"` | No (wraps the parse error, not the sentinel) |
| 14 | `tools.go:1112-1113` `listScheduled` | `created_before` | `"created_before: %w"` | No |
| 15 | `tools.go:1123-1124` `listScheduled` | `state` | `"state must be one of scheduled\|expired\|all, got %q"` | No |
| 16 | `tools.go:1169-1170` `effectiveDiscoveryScope` | `scope` | `"scope is required unless cross_spine is true"` | No |
| 17 | `tools.go:1192-1193` `effectiveSearchScope` (shared by search_memory + list_memory, MCP closures + Connect handlers, 4 call sites) | `scope` | `"scope is required unless cross_spine is true"` | No |
| 18 | `tools.go:1568-1570` search_memory closure (inline) | `created_after` | `"created_after: %w"` | No |
| 19 | `tools.go:1572-1574` search_memory closure (inline) | `created_before` | `"created_before: %w"` | No |
| 20 | `tools.go:1615-1617` list_memory closure (inline) | `created_after` | `"created_after: %w"` | No |
| 21 | `tools.go:1619-1621` list_memory closure (inline) | `created_before` | `"created_before: %w"` | No |
| 22 | `rules.go:56-57` `validateStoreRule` | `content` | `"content is required"` | No — Pitfall 3 (Connect-unreachable today; store_rule has no Connect RPC, so latent) |
| 23 | `rules.go:59-60` `validateStoreRule` | `content` | `"content too large: %d bytes (max %d)"` | No |
| 24 | `rules.go:62-63` `validateStoreRule` | `scope` | `"scope is required"` | No |
| 25 | `rules.go:65-66` `validateStoreRule` | `scope` | `"scope must be rule:repo:<repo> or rule:project:<project>, got %q"` | No |
| 26 | `rules.go:80-81` `validateRuleSummary` (used by store_rule AND update_memory's rule-summary guard) | `summary` | `"summary is required for a rule (it is the one-line index entry)"` | **Yes** |
| 27 | `rules.go:83-84` `validateRuleSummary` | `summary` | `"rule summary must be a single line (no newlines); it is the index entry"` | **Yes** |
| 28 | `rules.go:86-87` `validateRuleSummary` | `summary` | `"summary too long: %d bytes (max %d)"` | **Yes** |
| 29 | `rules.go:186` `listRules` | `scopes` | `"at least one rule scope is required"` | No |
| 30 | `rules.go:190` `listRules` (per-element, in a loop) | `scopes[i]` | `"scope must be rule:repo:<repo> or rule:project:<project>, got %q"` | No |
| 31 | `summary.go:34` `resolveSummaryUpdate` (`errStaleSummary`) | `summary` | fixed sentinel text | **Yes** (already wired: `connecterror.go:60-61` → `CodeFailedPrecondition`) |

**Category B — Combination-of-fields checks (Pitfall 4; need a distinct envelope shape, cannot be single-`field`):**

| # | Site | Fields involved | Current message |
|---|------|-----------------|------------------|
| 32 | `tools.go:469-471` `parseWindow` | `not_before`, `not_after` (both empty) | `"schedule_memory requires not_before and/or not_after (use store_memory for unscheduled records)"` |
| 33 | `tools.go:472-474` `parseWindow` | `category` (value `"discovery"`) vs. the fact scheduling was attempted | `"discovery is not schedulable; use store_discovery"` — arguably single-field (`category`), borderline |
| 34 | `tools.go:492-493` `parseWindow` | `not_before`, `not_after` (ordering) | `"not_before must be strictly before not_after"` |
| 35 | `connectapi.go:159-161` (Connect-only, NOT in `tools.go` — see Open Questions) | `cursor_mode`, `offset` | `"cursor_mode is mutually exclusive with offset"` |

**Category C — Schema-level required-field rejections (Pitfall 1; NOT reachable by any `tools.go` code today; representative, not exhaustive — any struct field below without `omitempty` is in this class):**

| Struct | Fields (no `omitempty`) | Source |
|---|---|---|
| `storeArgs` (also embedded into `scheduleArgs`, `supersedeArgs`) | `content`, `scope`, `source`, `category` | `tools.go:430-433` |
| `searchArgs` | `query` | `tools.go:535` |
| `listRulesArgs` | `scopes` (key-presence only — empty array still satisfies "required") | `rules.go:34` |
| `updateArgs` | `id`, `content` (a `*string`; comment at `tools.go:572-580` confirms schema-required) | `tools.go:571,581` |
| `storeDiscoveryArgs` | `content`, `kind`, `citations`, `scope` | `tools.go:600-603` |
| `storeRuleArgs` | `content`, `scope`, `summary` | `rules.go:26-28` |
| `idArgs` | `id` | `tools.go:566-567` |
| `setVisibilityArgs` | `id`, `shared` | `tools.go:633-635` |
| `supersedeArgs` | `supersedes` (plus inherited `storeArgs` fields) | `tools.go:528` |

This phase's plan must state, per Pitfall 1, whether Category C is in scope (requires relaxing `omitempty` + adding a Go-level check per field) or explicitly deferred.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `field=X code=Y: text` or JSON-string convention is a reasonable MCP-lane encoding for D-09's envelope | Pattern 2 | If the planner picks a different convention, low risk — this is explicitly Claude's Discretion per CONTEXT.md; flagged only as one workable option, not a recommendation to lock |
| A2 | `parseWindow`'s `"discovery is not schedulable"` check (tools.go:472-474) is single-field-attributable to `category` rather than a combination | Sweep table (#33) | Low — either classification works for the matrix test; worth the planner's explicit call |
| A3 | The proposed D-11 mapping table (Code Examples) is a reasonable default split across {InvalidArgument, OutOfRange, FailedPrecondition} | Code Examples | Medium — this is explicitly "Claude's Discretion... the planner proposes the mapping table" per CONTEXT.md; treat as a strong starting point, not a lock, and reconcile against the full Category A/B inventory before finalizing |
| A4 | Realistic `ENGRAM_EMBED_DIM` values for D-16's bound sizing top out around 4096 (Qwen3-Embedding, documented in `docs-site/guides/embedding-models.md:44`), implying a per-response JSON size on the order of 55-60 KiB for a single embedding vector (float32 as JSON text ≈ 12-14 bytes/element including comma) | Common Pitfalls / Code Examples (D-16 sizing) | Low-Medium — if a provider batches multiple embeddings into one response despite `input` being a single string, or uses a very high-precision float representation, the true response could be larger; a bound like `max(64 KiB, dim*32)` gives headroom without blind-copying `summarize.go`'s 1 MiB |

**If this table is empty:** N/A — populated above.

## Open Questions

1. **Is `connectapi.go`'s cursor_mode/offset check (line 159-161) in scope for D-05/D-06, given it lives outside `tools.go`?**
   - What we know: D-06's text names `internal/server/tools.go` specifically; this check is in `connectapi.go`, Connect-only, with no MCP equivalent (MCP's `listArgs` has no `offset` field at all).
   - What's unclear: whether the structured envelope should reach Connect-native validation guards too (created_after/before parse errors at `connectapi.go:149,153,230,234` and the `effectiveSearchScope` guard calls at `:173,243` are effectively duplicates of `tools.go` checks reached via a different path, so they likely inherit the fix "for free" if the underlying `tools.go` function is fixed — but the cursor_mode/offset check has no `tools.go` counterpart to inherit from).
   - Recommendation: treat it as in-scope (it's a `tools.go`-sibling boundary check on the same request surface) but flag explicitly in the plan since it's textually outside D-06's stated file.

2. **Does Connect's `connect.ErrorDetail` mechanism have a role in carrying the D-05/D-09 envelope on the Connect lane, beyond the bare `Code` + `err.Error()` message?**
   - What we know: `connect.NewError(code, err)` is the only mechanism used in this codebase today (`connecterror.go`); connect-go v1.20.0 supports attaching typed error details (`connect.Error.AddDetail`) but this was not investigated in this session (out of the research budget for this pass).
   - What's unclear: whether a richer, machine-parseable Connect error detail (vs. just `err.Error()`'s string, which IS structurable) is warranted, or whether the plain message text (now field-first per D-08) plus the widened code (D-11) is sufficient.
   - Recommendation: default to NOT using `ErrorDetail` unless the planner has a specific client need — the message string approach keeps MCP and Connect symmetric (both carry the same envelope-as-string), which is simpler to keep in parity per Phase 3's established discipline of MCP↔Connect symmetry testing.

3. **Root cause of the exact segmentio/encoding decode anomaly in issue #360's repro (byte-count-independent failure correlated with summary length)?**
   - What we know: the go-sdk's JSON decode path uses `github.com/segmentio/encoding/json` (`internal/json/json.go:13,20-24`, confirmed read), not stdlib `encoding/json`; the failure does not correlate with simple total payload size (issue #360's own table: row 1 succeeded at 2785 bytes, row 5 failed at ~1812 bytes).
   - What's unclear: the exact segmentio/encoding decoder behavior that would make a present `content` key vanish from the intermediate `map[string]any` for specific summary/content size combinations — not root-caused in this session (would require an isolated repro against the vendored `segmentio/encoding@v0.5.4`/`v0.5.3`, both present in `$GOMODCACHE`, not read this session).
   - Recommendation: this exact mechanism does NOT need to be root-caused for the phase's design to proceed correctly — Pitfall 1's fix (moving required-field enforcement Go-side) makes the mechanism irrelevant, since the field would no longer be schema-required at all. If the planner wants a smoking-gun repro before committing to the omitempty-relaxation approach, budget a short isolated spike against `segmentio/encoding` directly.

## Environment Availability

Not applicable — no new external tool, service, runtime, or CLI dependency. This phase edits existing Go source only; build/test/lint tooling (`go`, `task`, `golangci-lint`) is already required and available per the existing `Taskfile.yaml`/CI, unchanged by this phase.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go's standard `testing` package + `go test` (no third-party test framework) |
| Config file | none — `Taskfile.yaml`'s `test` task runs `go test ./...` |
| Quick run command | `go test ./internal/server/... ./internal/store/... ./internal/embed/... ./internal/summarize/... -run <TestName> -v` |
| Full suite command | `task` (lint + test; wraps `go test ./...` plus `golangci-lint`, `gofmt`, etc.) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-authz-decision-diagnostics | Both allow and deny arms emit a debug-level log line carrying exactly the D-02 allowlist fields | unit | `go test ./internal/store/... -run TestDecideBucketLogsAllowAndDeny -v` (new) | ❌ Wave 0 — but the `decideBucketHook`/`decideRecordHook` test seam already exists (`store_test.go:4248-4404,4749-4862`) and the slog-capture-to-buffer idiom already exists (`internal/auth/auth_test.go:50-62`: `slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))` + `t.Cleanup` restore) |
| REQ-authz-decision-diagnostics | No full Cedar expression trace ever appears in a log line, at any level | unit | assert the log buffer NEVER contains a `DiagnosticError.Message`-shaped string across allow+deny+multi-policy cases | ❌ Wave 0 |
| REQ-validation-error-attribution | A matrix, one case per single-field-invalid input, asserts the correct field name | unit | `go test ./internal/server/... -run TestValidationErrorAttributionMatrix -v` (new, table-driven) | ❌ Wave 0 — must cover every row in the Full D-06 Sweep Inventory (Category A at minimum; explicitly note Category C's disposition per Pitfall 1) |
| REQ-error-hint-envelope | A rejection's envelope carries the field AND a hint code+text together | unit | extend the same matrix test to assert both fields of the envelope | ❌ Wave 0 |
| REQ-embed-provider-error-body | A non-2xx embeddings response includes a bounded provider-body prefix + status in the error, and the connection is reusable after | unit + integration | `go test ./internal/embed/... -run TestEmbedNon2xxIncludesStatusAndBody -v` (new, mirrors `summarize_test.go:132-148`'s `TestSummarizeNon200IncludesStatusAndBody`); a second test for drain/reuse needs `httptrace.WithClientTrace` + `GotConn.Reused` (no existing helper — see Wave 0 gap) | ❌ Wave 0 |
| REQ-embed-provider-error-body | The chat/summarize lane gains the same drain (audit clause) | unit | extend `summarize_test.go` with a drain/reuse assertion using the same new httptrace helper | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** targeted `go test ./internal/<touched-package>/... -run <TestName> -v`
- **Per wave merge:** `go test ./internal/server/... ./internal/store/... ./internal/embed/... ./internal/summarize/... ./internal/authz/...`
- **Phase gate:** `task` (full lint + test suite) green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] A connection-reuse assertion helper — confirmed absent from the entire codebase (`rg -n "httptrace|GotConn|\.Reused\b"` over `internal/` returns zero matches, this session). Needs either `httptrace.WithClientTrace` checking `GotConn.Reused` across two sequential requests to the same `httptest.Server`, or a wrapped `net.Listener` counting `Accept()` calls. This is the ONLY genuinely new test infrastructure this phase needs — everything else (slog capture, httptest fixtures, the `decideBucketHook`/`decideRecordHook` seam) already exists.
- [ ] `TestValidationErrorAttributionMatrix` (or equivalent) covering every row of the Full D-06 Sweep Inventory Category A (31 rows) plus explicit Category B combination-shape tests (4 rows) — does not exist yet.
- [ ] A Cedar-diagnostics debug-log test in `internal/store` — does not exist yet, but every building block (`decideBucketHook`, `slog.SetDefault`+buffer pattern) is already proven elsewhere in the codebase.

*(No framework install needed — `go test` and `httptest`/`httptrace` are stdlib.)*

## Security Domain

`security_enforcement` is absent from `.planning/config.json`, so treat as enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No (this phase doesn't touch auth) | n/a — D-08's scope fence exists precisely to keep this phase OUT of the auth path |
| V3 Session Management | No | n/a |
| V4 Access Control | Indirectly — D-01/D-02/D-03 are a **disclosure control** on top of an unchanged access-control decision | The narrow accessor (D-03) is itself the ASVS-relevant control: it structurally prevents an over-broad diagnostic leak (V7.1-style "insufficient logging" turned inside-out into "excessive logging") |
| V5 Input Validation | Yes | The entire D-05/D-06/D-07 sweep IS input validation error reporting; no change to WHAT is validated, only how the rejection is reported |
| V6 Cryptography | No | n/a |
| V7 Error Handling & Logging | Yes — the core of this phase | D-02's allowlist, D-12's no-value-echo rule, D-15's verbatim-but-bounded provider body are all textbook ASVS V7 controls (log what's needed for debugging, never secrets/PII; bound untrusted external content before it reaches a log or error) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Cedar diagnostic message leaking entity attribute values (e.g. another actor's owner string) into an operator-visible log | Information Disclosure | D-02's allowlist (policy IDs + count only, `DiagnosticError.Message` excluded) — confirmed structurally enforceable since `Message` is a single, isolable field on `DiagnosticError` (Pattern 4) |
| A validation-error hint echoing the caller's own oversized/malicious input value back into a log or Connect response | Information Disclosure / minor DoS via unbounded echo | D-12's no-value-echo rule, consistent with Phase 3's D-02 precedent |
| An unbounded provider error-body read enabling a slow/hostile embeddings gateway to force excessive memory use or block a connection indefinitely | Denial of Service | D-13/D-16's bounded `io.LimitReader` on both the error-body read and the success decode; D-14's drain ensures the underlying TCP connection is still released/reusable rather than left half-read |
| A validation-error message accidentally becoming a NEW wire contract that then can't be changed (repeating the `e9yv53pmnv` MCP-401-body class of lock-in) | Tampering (of the operator's own upgrade path) / not a caller-facing threat but a maintenance hazard | D-08's explicit scope fence (verified this session: the MCP 401 body is produced by `go-sdk/auth`'s `RequireBearerToken`/`verify()`, `auth/auth.go:69-97,99+`, via `http.Error(w, errmsg, code)` at `:90` — architecturally separate from and running BEFORE any `tools.go` code executes; confirmed by direct `Read` of both `internal/auth/bearer.go` and the vendored go-sdk `auth/auth.go`) |

## Sources

### Primary (HIGH confidence — read live this session, either in-repo or in `$GOMODCACHE`)

- `internal/server/tools.go` (full structure + targeted reads: lines 84-1786 spanning every `validate*`/inline check, every `mcp.AddTool` closure)
- `internal/server/rules.go` (full file, 220 lines)
- `internal/server/connectapi.go` (full file, 459 lines)
- `internal/server/connecterror.go` (lines 49-85, the `connectError` switch)
- `internal/server/connectobs_test.go` (full file, 77 lines — the slog-capture test idiom)
- `internal/server/summary.go` (lines 14-38, `resolveSummaryUpdate`/`errStaleSummary`)
- `internal/authz/authz.go` (full file, 93 lines)
- `internal/authz/policies.go` (full file, 68 lines)
- `internal/store/store.go` (targeted: lines 312-321, 660-737, 1510-1600, plus a grep confirming zero `slog` usage in the whole file)
- `internal/store/store_test.go` (lines 4248-4404, 4720-4762 — the `decideBucketHook`/`decideRecordHook` test seam and its documented ErrNotFound-uniformity guarantee)
- `internal/embed/embed.go` (full file, 259 lines)
- `internal/embed/embed_test.go` (lines 100-170 — existing httptest fixture patterns)
- `internal/summarize/summarize.go` (full file, 201 lines)
- `internal/summarize/summarize_test.go` (lines 120-190 — `TestSummarizeNon200IncludesStatusAndBody`, the fixture to mirror)
- `internal/auth/bearer.go` (full file, 168 lines — the D-08 scope-fence proof)
- `internal/config/registry.go:31` (`ENGRAM_EMBED_DIM` default `1024`)
- `docs-site/src/content/docs/guides/embedding-models.md` (dim values up to 4096, Qwen3-Embedding recipe)
- `docs-site/src/content/docs/guides/cli.md` (lines 40-79, the exit-code table and its Connect-taxonomy-derivation claim)
- `docs-site/src/content/docs/reference/tools.md` (header structure only, for the docs-surface obligation)
- `cmd/engram/client_common.go` (lines 185-250, the exit-code constants and `exitCodeForConnectErr` mapper)
- `go.mod` (dependency versions: `cedar-go v1.8.0`, `connect-go v1.20.0`, `go-sdk v1.6.1`)
- `$GOMODCACHE/github.com/modelcontextprotocol/go-sdk@v1.6.1/mcp/tool.go` (full file — `applySchema`)
- `$GOMODCACHE/github.com/modelcontextprotocol/go-sdk@v1.6.1/mcp/server.go` (lines 285-390, 470-510 — `toolForErr`, `AddTool`)
- `$GOMODCACHE/github.com/modelcontextprotocol/go-sdk@v1.6.1/mcp/protocol.go` (lines 60-144 — `CallToolResult`, `SetError`)
- `$GOMODCACHE/github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go` (lines 69-110 — `RequireBearerToken`, `verify`)
- `$GOMODCACHE/github.com/modelcontextprotocol/go-sdk@v1.6.1/internal/json/json.go` (full file — confirms `segmentio/encoding/json` as the underlying decoder)
- `$GOMODCACHE/github.com/google/jsonschema-go@v0.4.3/jsonschema/validate.go` (full file, 907 lines — `Resolved.Validate`, the "required: missing properties" error construction)
- `$GOMODCACHE/github.com/cedar-policy/cedar-go@v1.8.0/types/authorize.go` (full file — `Diagnostic`/`DiagnosticReason`/`DiagnosticError`)
- `$GOMODCACHE/github.com/cedar-policy/cedar-go@v1.8.0/authorize.go:18` (`Authorize` signature)
- `$GOMODCACHE/connectrpc.com/connect@v1.20.0/code.go` (full closed `Code` enum)
- GitHub issue #360 (`gh issue view 360`) — the exact repro table and minimal-repro JSON for the flagship validation-attribution bug

### Secondary (MEDIUM confidence)

- `.planning/research/SUMMARY.md`, `FEATURES.md`, `PITFALLS.md` (prior milestone-level research feeding this phase — read for framing, not re-verified line-by-line since it predates this session's direct code reads, which superseded/confirmed its claims)
- `.planning/BACKLOG.md`, `.planning/PROJECT.md` (issue-to-requirement traceability)

### Tertiary (LOW confidence)

- None — every substantive claim in this document traces to a live `Read` or `Bash`/`rg` result from this session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, confirmed against `go.mod` directly
- Architecture (MCP wire-format constraint, Cedar diagnostic shape, per-request cardinality, drain gaps): HIGH — every claim traced to a specific file:line read live this session, several against vendored dependency source
- D-06 sweep inventory: HIGH for Category A/B (every row read live); MEDIUM for Category C's completeness (representative, explicitly not claimed exhaustive — a struct-tag grep across all `*Args` types would close this if the planner wants full enumeration)
- Pitfalls: HIGH — Pitfall 1 (schema-vs-Go-level gap) and Pitfall 3 (CodeInternal misclassification) are both freshly-discovered, evidence-backed findings not present in any prior research artifact for this phase

**Research date:** 2026-08-01
**Valid until:** 30 days (stable internal codebase; the go-sdk/cedar-go/connect-go dependency versions are pinned in `go.mod` and unlikely to change mid-milestone) — re-verify if `go.mod` bumps `modelcontextprotocol/go-sdk`, `cedar-policy/cedar-go`, or `connectrpc.com/connect` before this phase executes.
