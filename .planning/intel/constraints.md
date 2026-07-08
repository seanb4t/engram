# Constraints

Technical constraints (api-contract | schema | nfr | protocol) extracted from the SPEC/design
docs (precedence 1). These are the concrete contracts and non-functional bounds the
implementation must honor. Where a constraint is also locked by an ADR, the ADR governs;
the constraint here records the technical shape.

---

## api-contract

### CON-embedder-body-params — Query/document embedder request-body params
- source: docs/superpowers/specs/2026-07-01-asymmetric-cloud-embedder-params-design.md
- type: api-contract
- content: Query and document embedding params are provider-agnostic JSON maps merged into
  the OpenAI-compatible `/v1/embeddings` request body; a document-side text instruction
  supports asymmetric and both-side-prefix models; `input_type`/`task`/`task_type` fields
  pass through. Reindex boundary must be respected when params change.
- governed-by: DEC-zyhq, DEC-378

### CON-engramservice-read-rpcs — EngramService Connect read API
- source: docs/superpowers/specs/2026-06-09-engram-web-ui-design.md; docs/superpowers/specs/2026-06-10-engram-operator-console-spa-design.md
- type: api-contract
- content: The web console consumes a ConnectRPC `EngramService` v1 read API (Search/List/Get
  memories, discovery search) over connect-go (server) / connect-es (client). MCP core is
  unchanged. Cookie/OIDC auth on the observe lane; offset pagination in v1.
- governed-by: DEC-8xe, DEC-0lu, DEC-bj6

### CON-recall-shape — Summary-by-default recall output
- source: docs/superpowers/specs/2026-06-25-auto-summary-curated-memories-design.md
- type: api-contract
- content: search_memory/list_memory and Connect SearchMemories/ListMemories return
  summary-shaped output by default; `full=true` opts into full content; `get_memory` always
  returns full. `summary_source` carries provenance (client vs auto-generated).
- governed-by: DEC-ambu

### CON-recall-paging — Cursor + date-window recall params
- source: docs/superpowers/specs/2026-06-29-windowed-cursor-recall-design.md
- type: api-contract
- content: list_memory/search_memory/list_scheduled accept optional
  `created_after`/`created_before` (RFC3339, half-open `[after, before)`) and list_memory
  paginates via an opaque boundary id-set `cursor` returning `{memories, next_cursor}`.
- governed-by: DEC-1frj, DEC-ef28

### CON-tag-filter — AND-default tag pre-filter
- source: (locked) docs/adr/engram-4xt7-tag-filtered-recall-hard-qdrant-filter-and-default.md
- type: api-contract
- content: The optional `tags` filter is a hard AND (contains-all) Qdrant pre-filter applied
  before vector ranking on search_memory and before listing on list_memory.
- governed-by: DEC-4xt7

---

## schema

### CON-short-id-field — 10-char Crockford base32 short_id
- source: docs/superpowers/specs/2026-07-06-short-id-handle-design.md
- type: schema
- content: Each memory record carries a server-minted `short_id`: a 10-char lowercase
  Crockford base32 handle, accepted anywhere an id is accepted, resolved to the UUID Qdrant
  point id at the handler layer via `Store.ResolvePointID`.
- governed-by: DEC-zzq0, DEC-02ta

### CON-temporal-window — not_before / not_after epoch payloads
- source: docs/superpowers/specs/2026-06-12-scheduled-memories-design.md
- type: schema
- content: Temporal validity is stored as epoch-second Qdrant payload fields `not_before`
  (deferred reveal) and `not_after` (expiry). store_memory stays windowless; schedule_memory
  sets the window.
- governed-by: DEC-90w, DEC-y1g

### CON-owner-payload — Owner authz key on every record
- source: docs/superpowers/specs/2026-06-06-per-actor-memory-isolation-design.md; docs/superpowers/specs/2026-06-29-configurable-claim-owner-design.md
- type: schema
- content: Each record carries an `owner` payload set from a configurable OIDC claim (default
  `email`, `ENGRAM_OWNER_CLAIM`); anonymous callers map to a single empty-owner bucket.
  Pre-isolation records (missing owner key) are invisible until backfilled via
  migrate-remap-owner.
- governed-by: DEC-g37x, DEC-cgb

### CON-qdrant-payload-indexes — owner/scope/created_at indexes
- source: docs/superpowers/specs/2026-06-29-windowed-cursor-recall-design.md
- type: schema
- content: Keyword indexes on owner/scope and a datetime index on created_at are created in
  ensureCollection, enabling exact server-side Count and range filtering; scanCap and the
  approximate flag are retired.
- governed-by: DEC-ef28

### CON-discovery-category — Discovery as 5th category, one collection
- source: docs/superpowers/specs/2026-06-05-discovery-memory-type-design.md
- type: schema
- content: Discovery is a 5th category on the existing Memory record in the single Qdrant
  collection, carrying kind (map|fact), citations with aging pins, and summary; lives in a
  separate `discovery:repo:*` scope.
- governed-by: DEC-2bv

---

## protocol

### CON-authz-store-layer — Store-layer default-deny enforcement
- source: docs/superpowers/specs/2026-06-06-per-actor-memory-isolation-design.md; docs/superpowers/specs/2026-06-08-typed-subject-authz-core-design.md
- type: protocol
- content: Authorization is enforced in internal/store via Qdrant read filters and owner-gate
  primitives (getReadable/getWritable/ownedOrAbsent), keyed on a sealed `Subject` sum with
  exhaustive default-deny switches. Handlers do not enforce authz.
- governed-by: DEC-cgb, DEC-12c

### CON-read-write-asymmetry — Shared grants read, never write
- source: docs/superpowers/specs/2026-06-06-per-actor-memory-isolation-design.md
- type: protocol
- content: Sharing grants read access only; owners keep exclusive write/delete/visibility
  control. Shared read requires a non-empty owner-claim value. Rules are always-shared and
  reject set_visibility.
- governed-by: DEC-kyz, DEC-iedk

### CON-not-found-uniformity — 404 for unauthorized id ops
- source: docs/superpowers/specs/2026-06-06-per-actor-memory-isolation-design.md
- type: protocol
- content: All owner/visibility mismatches on id-addressed operations return the same
  not-found error as a missing id, preventing cross-actor existence leaks.
- governed-by: DEC-xa6

### CON-recall-gate-asymmetry — Recall gated, get_memory ungated
- source: docs/superpowers/specs/2026-06-12-scheduled-memories-design.md
- type: protocol
- content: Temporal validity and isolation are enforced as Qdrant filter conditions on
  Search/List; get_memory and by-id paths remain ungated for record management.
- governed-by: DEC-y1g

### CON-mcp-transport-path — MCP at explicit path
- source: (locked) docs/adr/engram-bj6-mcp-transport-at-explicit-configurable-path-mem-mcp-path-con.md
- type: protocol
- content: MCP StreamableHTTP transport mounts at an explicit configurable path (default
  `/mcp`, `MEM_MCP_PATH`); the console occupies root when the UI is enabled.
- governed-by: DEC-bj6

### CON-connect-auth-deferred — Interim anonymous Connect mount; deferred R1–R4
- source: docs/superpowers/specs/2026-06-09-connect-auth-posture-addendum.md
- type: protocol
- content: The Connect API is mounted interim-anonymous into the single empty-owner bucket;
  full cookie/OIDC observe-lane auth requirements R1–R4 are DEFERRED and must be carried into
  the roadmap before the observe lane is exposed to real identities.

---

## nfr

### CON-telemetry-otlp-only — OTLP-only export, no Prometheus scrape
- source: docs/superpowers/specs/2026-06-07-observability-logging-telemetry-design.md
- type: nfr
- content: Metrics, traces, and logs export exclusively over OTLP gRPC to a collector (Grafana
  LGTM backend); no Prometheus `/metrics` scrape endpoint is added.
- governed-by: DEC-dwi

### CON-telemetry-nonblocking — Telemetry never blocks startup
- source: docs/superpowers/specs/2026-06-07-observability-logging-telemetry-design.md; docs/superpowers/specs/2026-06-11-telemetry-at-every-seam-design.md
- type: nfr
- content: Telemetry setup failure or a missing OTLP endpoint yields no-op providers; the
  server never aborts startup on telemetry. Spans/domain-latency metrics instrument
  store/embed/auth/HTTP/MCP seams.
- governed-by: DEC-uxh, DEC-dwi

### CON-config-fatal-legacy — Fatal legacy-env guard, no silent fallback
- source: docs/superpowers/specs/2026-06-14-engram-config-prefix-koanf-design.md; docs/superpowers/specs/2026-06-14-config-validation-design.md
- type: nfr
- content: Config is unified under `ENGRAM_` via koanf with a single field registry; retired
  `MEM_*` vars trigger a fatal `config.CheckLegacy` startup guard; `Config.Validate()` runs
  early and loudly at every entrypoint. No silent fallback or dual-read shim.
- governed-by: DEC-jgq, DEC-irq

### CON-ui-real-dom-tests — Real-Chromium UI test gate
- source: docs/superpowers/specs/2026-06-27-vitest-browser-mode-ui-test-unification-design.md
- type: nfr
- content: UI/sanitizer tests run under vitest 4 browser mode (real Chromium via Playwright),
  retiring jsdom/happy-dom emulators, so DOMPurify + bits-ui are exercised against a real DOM
  in the CI gate.
- status-note: SPEC Status DRAFT.

### CON-docs-static-only — Docs site static, no SSR
- source: docs/superpowers/specs/2026-06-09-docs-site-astro-starlight-cloudflare-design.md
- type: nfr
- content: The Astro Starlight docs-site ships as static assets via a Cloudflare Workers
  assets binding — no SSR adapter or worker script. Frontend SPA is adapter-static, vendored
  via go:embed.
- governed-by: DEC-ttb, DEC-0lu
