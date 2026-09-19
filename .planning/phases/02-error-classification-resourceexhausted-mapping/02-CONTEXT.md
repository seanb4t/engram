# Phase 2: Error Classification & ResourceExhausted Mapping - Context

**Gathered:** 2026-09-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Classify a Qdrant response that exceeded the client's receive limit into ONE typed sentinel in
`internal/store` — matching both the gRPC `ResourceExhausted` code AND the receive-limit message
shape, so an unrelated server-capacity `ResourceExhausted` is never relabeled — then map that
sentinel once per lane at each lane's chokepoint: Connect (`connectError` → `resource_exhausted`,
never `internal`), MCP (a new single mapper), and the CLI (a documented exit code), with the hint
code documented in docs-site `reference/errors.md`. Covers REQ-exhausted-sentinel,
REQ-exhausted-connect, REQ-exhausted-mcp, REQ-exhausted-cli-docs. No read path is bounded here
(Phases 3–5); this phase makes an overflow a clear, named failure so later regression tests assert
the RIGHT failure mode.

</domain>

<decisions>
## Implementation Decisions

### Carried forward

- **D-00 (from Phase 1, user preference `1w3h5sy56m`):** choose by idiom and long-term
  maintenance, never by effort; prefer upstream vocabulary and hooks over bespoke wrappers.
- Phase 1 locked `store.NewQdrantClient(host, port, ...grpc.DialOption)` as the ONE client
  constructor for production and every test, with base options applied first — this phase's
  classifier rides on it.

### Classification

- **D-01:** Classify in a gRPC **unary client interceptor** installed in
  `store.NewQdrantClient`'s BASE dial options (`grpc.WithChainUnaryInterceptor`), so every Qdrant
  RPC — current read paths, the operator sweeps, and any future call site — is covered in one
  place, in production and tests alike (the #583→#585 lesson: per-site handling drifts). The
  interceptor converts a `codes.ResourceExhausted` status whose message matches grpc-go's
  receive-limit shape into the sentinel; every other error passes through untouched. The
  returned error MUST still satisfy `status.FromError` (implement `GRPCStatus()`), so existing
  status inspection (e.g. `store.go:614`'s `grpccodes.AlreadyExists` check) keeps working. —
  **Reversibility:** reversible — one interceptor, one registration site.
- **D-02:** Match on BOTH the status code and the receive-limit message shape (REQ-locked); a
  server-sent `ResourceExhausted` without that shape is NOT relabeled.
- **D-03:** Payload = stdlib pattern: `var ErrResponseTooLarge` (exported from `internal/store`)
  for `errors.Is`, plus an error type carrying the RPC method and the observed/limit byte counts,
  used for SERVER-SIDE LOGS ONLY. Byte counts and raw gRPC/Qdrant text never reach any wire
  (REQ-exhausted-connect).

### Wire contract (published — additive change)

- **D-04:** Envelope `field=response hint=too_large: <detail>` on BOTH lanes, via the existing
  `field=<name> hint=<code>` grammar and ONE renderer (not a second hand-built string). `response`
  is a fixed pseudo-field (the response overflowed, not an input — precedent: revert's
  `field=steps`), which keeps the mapper decoupled from every tool's argument shape. The detail
  text names the remedies generically (a smaller `limit`/`k`, or omitting `full`) and contains no
  byte ceiling and no upstream text. — **Reversibility:** one-way — a published hint vocabulary
  and envelope are a wire contract agents branch on. The user chose this exact option in discuss-phase (2026-09-19) — the door is already walked through; do not insert a checkpoint to re-ask.
- **D-05:** New hint code `too_large`, added as the 11th `HintCode` constant in
  `internal/server/argerror.go` (the `too_long`/`too_many` family); docs-site
  `reference/errors.md`'s hint-code table gains that row (its heading/count updates accordingly),
  and the class-to-Connect-code and CLI sections gain the `resource_exhausted` → exit `10` path.
  Verify at plan time whether `internal/surfaces`' single-declaration convention covers hint
  codes or only conditional-rule sentences (ROADMAP note) before assuming it applies.

### Connect lane

- **D-06:** One new arm in `connectError` (`internal/server/connecterror.go`):
  `errors.Is(err, store.ErrResponseTooLarge)` → `connect.CodeResourceExhausted` carrying the D-04
  envelope — never `CodeInternal`. Log the raw error (with D-03 detail) server-side, as the
  default arm already does for unknowns. Arm placement must respect the existing
  `errors.As(err, &ae)`-first ordering comment.

### CLI lane

- **D-07:** New dedicated exit code **`exitTooLarge = 10`** in `cmd/engram/client_common.go`
  (next after `exitSetupFailed = 9`); `exitCodeForConnectErr` maps `connect.CodeResourceExhausted`
  → 10 (today it falls to `exitGeneric = 1` — a visible change, documented).
  `TestExitCodeForConnectErrTable` / the exit-code baseline tests and the docs-site CLI guide's
  exit-code table (`guides/cli.md` "## Exit codes") update in the same change. —
  **Reversibility:** one-way — exit codes are a scripting contract. The user chose this exact option in discuss-phase (2026-09-19) — do not insert a checkpoint to re-ask.

### MCP lane

- **D-08:** The single MCP-side mapper is a **receiving middleware** on `tools/call`
  (`mcp.Middleware`, registered via `s.AddReceivingMiddleware` beside `instrumentTools`). It
  recovers the handler's original Go error via go-sdk v1.8.0's `CallToolResult.GetError()`,
  applies `errors.Is(err, store.ErrResponseTooLarge)`, and rewrites the result's text content to
  the D-04 envelope (`IsError` stays true). No edits to the 15 tool closures. Order it relative to
  `instrumentTools` so the RAW error (with D-03 detail) is still logged server-side exactly once.
- **D-09:** Scope this phase: map ONLY `ErrResponseTooLarge`; every other MCP error passes through
  unchanged, as today. The lane inconsistency (Connect scrubs unclassified errors to
  `internal error`; MCP returns raw error text) is recorded as a deferred item, not fixed here.

### Claude's Discretion

- Exact identifiers (`ErrResponseTooLarge`, detail type name, interceptor/middleware names) and
  file placement within `internal/store` / `internal/server`.
- Interceptor ordering relative to the otelgrpc stats handler; middleware ordering relative to
  `instrumentTools` (constrained by D-08's log-once requirement).
- Regression-test design: an END-TO-END overflow using Phase 1's `storetest.Dial` +
  `storetest.SeedOversized` at the named 4 MiB limit, asserting the sentinel (store), the
  `resource_exhausted` code + scrubbed envelope (Connect), the envelope text (MCP), and exit 10
  (CLI) — each RED before its mapping lands; plus a unit test of the classifier against synthetic
  statuses proving a non-matching `ResourceExhausted` is NOT relabeled. Never assert grpc-go's own
  default limit or message format as behavior (rule `m45p2b4bp7`) — the classifier's match is our
  code under test.
- Phase 2 red-evidence patches (register in `redEvidenceDirs` after the last plan).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & research
- `.planning/REQUIREMENTS.md` — REQ-exhausted-sentinel, REQ-exhausted-connect, REQ-exhausted-mcp,
  REQ-exhausted-cli-docs
- `.planning/ROADMAP.md` — Phase 2 goal (incl. the `internal/surfaces` verification note)
- `.planning/research/SUMMARY.md`, `.planning/research/PITFALLS.md` (Pitfall 3: leaking internals /
  over-broad catch), `.planning/research/STACK.md` (status/codes, `connect.CodeResourceExhausted`)
- `.planning/phases/01-test-harness-fixture-helper/01-CONTEXT.md` — D-01/D-02 constructor and
  dial-option pass-through this phase extends; D-04 named 4 MiB limit

### Code
- `internal/store/store.go` — `NewQdrantClient` (base options; the interceptor goes here);
  `:614` existing `status.FromError`/`grpccodes.AlreadyExists` idiom; `:1695` comment on
  ResourceExhausted surfacing as internal
- `internal/server/connecterror.go:55` — `connectError`, the single Connect mapper (ordering comment)
- `internal/server/argerror.go` — `HintCode` vocabulary (published contract note), `argError`
  envelope renderer, class → Connect code
- `internal/server/instrument.go` — `instrumentTools`, the receiving-middleware precedent
- `internal/server/tools.go:2262` — `s.AddReceivingMiddleware` registration site; 15 `mcp.AddTool`
  closures (unchanged)
- `cmd/engram/client_common.go:220-252,432` — exit-code constants, `exitCodeForConnectErr`
- `cmd/engram/client_common_test.go:34` `TestExitCodeForConnectErrTable`;
  `cmd/engram/exitcode_baseline_test.go:439` `TestExitCodeBaselineClaims`
- `internal/surfaces/` — single-declaration convention (verify hint-code coverage)
- `internal/store/storetest/` — `Dial`, `SeedOversized`, `RecvLimit` for real overflow tests
- go-sdk v1.8.0 `mcp/protocol.go:337-349` — `CallToolResult.SetError` / `GetError`;
  `mcp/server.go:419-433` — typed handler errors wrapped via `SetError`

### Docs
- `docs-site/src/content/docs/reference/errors.md` — hint-code table, class→Connect mapping, exit
  codes
- `docs-site/src/content/docs/guides/cli.md:353` — "## Exit codes" table (add 10)

### Rules & memories
- Rule `m45p2b4bp7` (no third-party behavior assertions), `xvqj44e5mk` (idiomatic/OSS)
- Memory `1w3h5sy56m` (idiom-over-effort framing), `7r10s08k9q` (4 MiB root cause)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `argError` + `HintCode` + `argClass`: the envelope grammar and renderer both lanes already use.
- `connectError`: typed-sentinel switch with a logging default arm.
- `instrumentTools`: a working `mcp.Middleware` for `tools/call`, including `CallToolResult` handling.
- `storetest.Dial` / `SeedOversized` / `RecvLimit` (Phase 1): produce a real overflow at a named limit.

### Established Patterns
- Classify once, map once per lane at the lane's existing chokepoint (research Architecture).
- Match typed sentinels via `errors.Is`/`errors.As`, never message strings — the ONE documented
  exception is D-02's receive-limit shape match inside the classifier.
- Exit codes are one-per-failure-class and pinned by a table test.

### Integration Points
- `store.NewQdrantClient` base options (interceptor); `Register` in `tools.go` (middleware);
  `connectError` (Connect arm); `exitCodeForConnectErr` (CLI); `errors.md` + `cli.md` (docs).

</code_context>

<specifics>
## Specific Ideas

- `field=response hint=too_large: …` is the literal envelope shape on MCP and in the Connect
  error message.
- `CodeResourceExhausted` → exit `10` (`exitTooLarge`).

</specifics>

<deferred>
## Deferred Ideas

- **MCP lane scrubbing of unclassified errors** — Connect returns a generic `internal error` for
  unknown errors while MCP returns raw error text; aligning the lanes is its own decision (D-09).
  Candidate backlog item.
- **Stream interceptor** — the Qdrant Go client is unary-only today; a defensive stream
  interceptor was not requested.

</deferred>

---

*Phase: 02-error-classification-resourceexhausted-mapping*
*Context gathered: 2026-09-19*
