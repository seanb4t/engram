# Phase 2: Error Classification & ResourceExhausted Mapping - Research

**Researched:** 2026-09-19
**Domain:** gRPC client-side error classification (grpc-go/qdrant-go-client), Connect error mapping, MCP receiving-middleware error mapping, CLI exit-code taxonomy
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Carried forward**
- **D-00 (from Phase 1, user preference `1w3h5sy56m`):** choose by idiom and long-term
  maintenance, never by effort; prefer upstream vocabulary and hooks over bespoke wrappers.
- Phase 1 locked `store.NewQdrantClient(host, port, ...grpc.DialOption)` as the ONE client
  constructor for production and every test, with base options applied first — this phase's
  classifier rides on it.

**Classification**
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

**Wire contract (published — additive change)**
- **D-04:** Envelope `field=response hint=too_large: <detail>` on BOTH lanes, via the existing
  `field=<name> hint=<code>` grammar and ONE renderer (not a second hand-built string). `response`
  is a fixed pseudo-field (the response overflowed, not an input — precedent: revert's
  `field=steps`), which keeps the mapper decoupled from every tool's argument shape. The detail
  text names the remedies generically (a smaller `limit`/`k`, or omitting `full`) and contains no
  byte ceiling and no upstream text. — **Reversibility:** one-way — a published hint vocabulary
  and envelope are a wire contract agents branch on. The user chose this exact option in
  discuss-phase (2026-09-19) — the door is already walked through; do not insert a checkpoint to
  re-ask.
- **D-05:** New hint code `too_large`, added as the 11th `HintCode` constant in
  `internal/server/argerror.go` (the `too_long`/`too_many` family); docs-site
  `reference/errors.md`'s hint-code table gains that row (its heading/count updates accordingly),
  and the class-to-Connect-code and CLI sections gain the `resource_exhausted` → exit `10` path.
  Verify at plan time whether `internal/surfaces`' single-declaration convention covers hint
  codes or only conditional-rule sentences (ROADMAP note) before assuming it applies.

**Connect lane**
- **D-06:** One new arm in `connectError` (`internal/server/connecterror.go`):
  `errors.Is(err, store.ErrResponseTooLarge)` → `connect.CodeResourceExhausted` carrying the D-04
  envelope — never `CodeInternal`. Log the raw error (with D-03 detail) server-side, as the
  default arm already does for unknowns. Arm placement must respect the existing
  `errors.As(err, &ae)`-first ordering comment.

**CLI lane**
- **D-07:** New dedicated exit code **`exitTooLarge = 10`** in `cmd/engram/client_common.go`
  (next after `exitSetupFailed = 9`); `exitCodeForConnectErr` maps `connect.CodeResourceExhausted`
  → 10 (today it falls to `exitGeneric = 1` — a visible change, documented).
  `TestExitCodeForConnectErrTable` / the exit-code baseline tests and the docs-site CLI guide's
  exit-code table (`guides/cli.md` "## Exit codes") update in the same change. —
  **Reversibility:** one-way — exit codes are a scripting contract. The user chose this exact
  option in discuss-phase (2026-09-19) — do not insert a checkpoint to re-ask.

**MCP lane**
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

### Deferred Ideas (OUT OF SCOPE)
- **MCP lane scrubbing of unclassified errors** — Connect returns a generic `internal error` for
  unknown errors while MCP returns raw error text; aligning the lanes is its own decision (D-09).
  Candidate backlog item.
- **Stream interceptor** — the Qdrant Go client is unary-only today; a defensive stream
  interceptor was not requested.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-exhausted-sentinel | `internal/store` classifies a Qdrant response that exceeded the client's receive limit into one typed sentinel, matching both the gRPC `ResourceExhausted` code and the receive-limit message shape, so an unrelated server-capacity `ResourceExhausted` is not relabeled. | Architecture Pattern 1 (verified interceptor chain position relative to qdrant-go-client's own `getRateLimitInterceptor`) and Pattern 2 (multi-`%w` sentinel composition); Pitfalls 1, 2, 6 document the exact grpc-go message shapes to match and why an over-broad match breaks qdrant's own rate-limit handling |
| REQ-exhausted-connect | Connect RPCs return `resource_exhausted` — never `internal` — for that sentinel, carrying a named hint code in the `field=<name> hint=<code>` envelope; the message contains no raw gRPC/Qdrant text and no byte ceiling. | Architecture Pattern 3 (verified `connectError` ordering constraint and why the sentinel must not be constructed as an `*argError`); Validation Architecture maps this to `testDepsWithStore`-based integration test |
| REQ-exhausted-mcp | MCP tools return the same named hint envelope for that sentinel through a single MCP-side mapper, rather than each tool closure surfacing the raw error. | Architecture Pattern 4 (verified `CallToolResult.SetError`/`GetError` mechanics and `AddReceivingMiddleware` ordering semantics); Pitfall 3 (instrumentTools does not currently log this error class — the new mapper introduces the first log line) |
| REQ-exhausted-cli-docs | The `engram` CLI maps the new code to a documented exit code, and the hint code is documented in docs-site `reference/errors.md`. | Pitfall 4 (the mechanical `TestCatalogExitCodesMatchMapper` gate requiring a same-commit `catalog.go` edit) and Pitfall 5 (the UN-gated `errors.md` hint-code count, requiring a manual doc edit with no test safety net) |
</phase_requirements>

## Summary

This phase classifies exactly one gRPC failure shape — a Qdrant response that overflowed the
client's receive limit — into a typed store-layer sentinel, then maps that sentinel once at each
of three existing chokepoints (`connectError`, a new MCP receiving-middleware, `exitCodeForConnectErr`).
Every API this phase needs already exists in the pinned stack; zero new Go dependencies. The
central technical risk is NOT "how do I detect ResourceExhausted" (trivial: `status.FromError`) —
it is that **qdrant-go-client already installs its own gRPC interceptor** (`getRateLimitInterceptor`,
a `grpc.WithChainUnaryInterceptor`) that inspects every `ResourceExhausted` status looking for a
`retry-after` trailer, and this phase's new classifier interceptor sits, by construction, **inside**
that existing interceptor in the call chain — closer to the wire, not wrapping it. This session
verified the exact chain position by reading `qdrant-go-client`'s dial-option composition order and
grpc-go's own `WithChainUnaryInterceptor` semantics: our classifier will be the position *closest to
the real network call*, and qdrant's own rate-limit interceptor sits *around* it. This is exactly
right for D-02's requirement (only relabel a message-shape match) — it means our classifier's
pass-through of a genuine server-side `ResourceExhausted` (no message-shape match) reaches qdrant's
rate-limit interceptor completely undisturbed, so that interceptor's own `retry-after` handling for
real server exhaustion keeps working unmodified.

The second load-bearing finding is about the actual grpc-go message text this project will see:
this repo's own Phase 1 RED observation (`01-04-SUMMARY.md`) captured the literal string
`"grpc: received message after decompression larger than max 4194304"` — a **single-number**
format that only appears when the response was compression-negotiated. Reading grpc-go v1.83.2
source directly confirms this is one of **three distinct receive-side message shapes** the pinned
version can produce (two of which carry both the observed and the limit byte counts; this one
carries only the limit). A classifier matching only the two-number shape would silently miss the
shape this project's own tests already observed firing. The message-shape match must cover all
three.

The third finding changes how the MCP-lane mapper should be designed: go-sdk v1.8.0's typed
`AddTool` handlers convert every ordinary handler error into `CallToolResult.SetError` — which means
the error **never surfaces as the `MethodHandler`-level Go `error` return** that `instrumentTools`
inspects. Reading `instrument.go` directly confirms `instrumentTools`'s own error-logging branch
(`if err != nil { ...ErrorContext... }`) therefore **never fires** for an ordinary tool-call
business error today; only the `outcome=error` **metric** reflects it. D-08's "log once" requirement
is not preserving an existing log line — it is *introducing* the first one.

**Primary recommendation:** build the sentinel as a stdlib-only Go 1.20+ multi-`%w` composition
(`store.ErrResponseTooLarge` plus a `status.Newf(...).Err()`), install the classifier via
`grpc.WithChainUnaryInterceptor` inside `store.NewQdrantClient`'s existing base-options block (no
caller-side test changes needed — every test already converges on this constructor per Phase 1),
add exactly one new `connectError` arm and one new `AddReceivingMiddleware` call, and treat the
`catalog.go`/`catalog.golden`/`errors.md`/`cli.md` companion edits as mandatory, mechanically
gated, same-commit work, not follow-up polish.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Detect receive-limit overflow | Store / gRPC client (`internal/store`) | — | The failure is a transport-layer event on the Qdrant client connection; only a client interceptor sees the raw `status.Error` before any wrapping |
| Classify into a typed sentinel | Store (`internal/store`) | — | D-01: one interceptor, one place, covers every Store call site including future ones |
| Map sentinel → Connect code | API / Backend (`internal/server`, `connectError`) | — | `connectError` is already the single production Connect-error chokepoint |
| Map sentinel → MCP envelope | API / Backend (`internal/server`, new `mcp.Middleware`) | — | No per-tool-closure mapping exists today; a receiving middleware is the MCP-native chokepoint analogous to `instrumentTools` |
| Map Connect code → CLI exit code | API / Backend (`cmd/engram`, `exitCodeForConnectErr`) | — | Already the single D-10 mapper; needs one new `case` |
| Publish hint vocabulary | Docs / Backend (`argerror.go` + `docs-site/reference/errors.md`) | — | `HintCode` constants are a published wire contract; the doc page transcribes them by hand (no gate enforces the transcription — see Pitfalls) |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `google.golang.org/grpc/status`, `.../codes` | v1.83.2 (pinned) | Detect `codes.ResourceExhausted` on a Qdrant RPC error; construct a `GRPCStatus()`-implementing sentinel | `[VERIFIED: google.golang.org/grpc@v1.83.2 module source, read this session]` — already imported in `internal/store/store.go` (`grpccodes "google.golang.org/grpc/codes"`, `"google.golang.org/grpc/status"`), already used at `store.go:614` for the `AlreadyExists` idiom this phase extends |
| `connectrpc.com/connect` | v1.21.0 (pinned, matches `go.mod`) | `connect.CodeResourceExhausted` (value `8`, wire name `resource_exhausted`) | `[VERIFIED: connectrpc.com/connect@v1.21.0/code.go:67-70,126-127, read this session]` — `Code.String()` for `CodeResourceExhausted` returns `"resource_exhausted"` exactly, matching REQ-exhausted-connect's literal wire-name requirement |
| `github.com/modelcontextprotocol/go-sdk/mcp` | v1.8.0 (pinned, matches `go.mod`) | `mcp.Middleware`, `Server.AddReceivingMiddleware`, `CallToolResult.GetError()`/`SetError()` | `[VERIFIED: go-sdk@v1.8.0 mcp/protocol.go:332-349, mcp/server.go:395-434, mcp/shared.go:133-137, read this session]` |
| `github.com/qdrant/go-client/qdrant` | v1.19.2 (pinned) | `Client.Scroll`/`Get`/etc.'s own error-wrap (`newQdrantErr`/`*QdrantError`) and the client's own pre-installed `getRateLimitInterceptor` — both load-bearing context, not APIs this phase calls directly | `[VERIFIED: qdrant-go-client@v1.19.2 qdrant/error.go, qdrant/config.go:141-188, qdrant/grpc_client.go:34-55, read this session]` |
| stdlib `errors`, `fmt` | go1.26.7 (module `go` directive) | `errors.New` sentinel; Go 1.20+ multi-`%w` composition to attach both a plain sentinel and a `GRPCStatus()`-carrying status error to one returned error | `[VERIFIED: go.mod:3 "go 1.26.7"]` — multi-`%w` (`fmt.Errorf("%w: %w", a, b)`) has been supported since Go 1.20, well within the pinned toolchain |

**No other core additions.** Every mechanism (interceptor installation, envelope rendering, exit-code
mapping) reuses an already-vendored API and an already-shipped in-repo pattern.

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/seanb4t/engram/internal/store/storetest` | in-repo, Phase 1 | `storetest.Dial(t, storetest.RecvLimit)` + `storetest.SeedOversized` | Every regression test in this phase that needs a REAL overflow, at any of the four lanes |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Message-shape substring match | A pre-flight byte-size estimate before the RPC | Qdrant does not report response size before transfer; STACK.md's own research already rejected this, and this session's reading of grpc-go confirms there is no cheaper signal than matching the transport's own error |
| Multi-`%w` sentinel composition | A custom `*ResponseTooLargeError` struct with hand-written `Is`/`Unwrap`/`GRPCStatus` methods | Functionally equivalent; multi-`%w` is fewer lines and avoids a bespoke type when D-03's two requirements (a plain `errors.Is` sentinel, plus a `GRPCStatus()`-satisfying error) are both native stdlib/status-package capabilities — matches D-00's idiom-over-effort framing |
| A second `AddReceivingMiddleware` call | Combining the new mapper into `instrumentTools` itself | Keeps the two concerns (instrumentation vs. error mapping) as two composable middlewares, matching the file's own doc comment describing `instrumentTools` as *one* of potentially several `mcp.Middleware`s, not a monolith |

**Installation:**
No `go install`/`go get` needed. Verify no drift:
```bash
go mod tidy && git diff --exit-code go.mod go.sum
```

**Version verification:** confirmed directly against `go.mod` and the pinned module cache this
session — `grpc v1.83.2`, `connectrpc.com/connect v1.21.0`, `github.com/modelcontextprotocol/go-sdk
v1.8.0`, `github.com/qdrant/go-client v1.19.2`, module `go 1.26.7`. `[VERIFIED: go.mod, read this
session]`

## Package Legitimacy Audit

**Not applicable — this phase adds zero new Go dependencies.** Every mechanism reuses an
already-pinned module (`google.golang.org/grpc`, `connectrpc.com/connect`,
`github.com/modelcontextprotocol/go-sdk`, `github.com/qdrant/go-client`) or the Go standard
library. No `go.mod`/`go.sum` change is expected; `git diff --exit-code go.mod go.sum` after
implementation should be a no-op.

## Architecture Patterns

### System Architecture Diagram

```
Qdrant server (real network response, may exceed the client's receive limit)
        │
        ▼
grpc-go transport layer (rpc_util.go: recvMsg / decompress)
        │  status.Errorf(codes.ResourceExhausted, "<one of 3 receive-side message shapes>")
        ▼
qdrant-go-client's OWN chained interceptors, in dial-option order:
  1. getMetadataInterceptor (WithUnaryInterceptor — always outermost, single-slot)
  2. getRateLimitInterceptor (WithChainUnaryInterceptor — outer of the CHAIN)
        │  (checks: status.Code()==ResourceExhausted AND a "retry-after" trailer present+parseable
        │   → wraps as *QdrantResourceExhaustedError; otherwise passes err through UNCHANGED)
        ▼
  3. ★ NEW: this phase's classifier (WithChainUnaryInterceptor, installed inside
     store.NewQdrantClient's BASE dial options — INNER of the chain, closest to the real call)
        │  (checks: status.Code()==ResourceExhausted AND message matches one of 3 receive-shapes
        │   → wraps as store.ErrResponseTooLarge-composed error; otherwise passes err through UNCHANGED)
        ▼
  4. the real network invoker (actual RPC)
        │
        ▼  (error returns UP through 3 → 2 → 1, then out of the interceptor chain)
qdrant-go-client's Client.Scroll/List/etc. wraps once more: newQdrantErr(err, "Scroll", collection)
        │  (*QdrantError.Unwrap() returns the inner error — errors.Is/As and status.FromError still work)
        ▼
internal/store.Store.List / ListScheduled / Search / (any current or future call site)
        │  returns the error UNMODIFIED (verified: store.go:1465-1467 for List)
        ▼
   ┌────────────────────┬─────────────────────────┬──────────────────────────┐
   ▼                    ▼                         ▼                          
Connect lane          MCP lane                  (direct store test)
connectError()        NEW mcp.Middleware        errors.Is(err, store.ErrResponseTooLarge)
  new case:            (2nd AddReceivingMiddleware
  errors.Is(err,        call): rewrites
  ErrResponseTooLarge)  CallToolResult.Content
  → CodeResourceExhausted to the D-04 envelope,
  + D-04 envelope        keeps IsError=true,
                         logs raw error ONCE
   │                    │
   ▼                    ▼
cmd/engram             engram MCP client sees
exitCodeForConnectErr   `field=response hint=too_large: ...`
  new case:
  CodeResourceExhausted
  → exitTooLarge (10)
```

### Recommended Project Structure

No new packages or files beyond what the phase's own components need:
```
internal/store/
├── store.go              # NewQdrantClient gains the classifier interceptor in base dialOpts;
│                          # new exported var ErrResponseTooLarge (+ unexported composer helper)
internal/server/
├── connecterror.go        # one new case in connectError's switch
├── argerror.go            # new HintCode = "too_large" constant (11th)
├── <new file>.go          # the MCP receiving-middleware mapper + its Register wiring
cmd/engram/
├── client_common.go       # exitTooLarge = 10; new case in exitCodeForConnectErr
├── catalog.go             # new {Code: exitTooLarge, Meaning: ...} row (MANDATORY companion edit)
docs-site/src/content/docs/reference/
├── errors.md              # "eleven hint codes"; too_large row; resource_exhausted → exit 10 note
docs-site/src/content/docs/guides/
├── cli.md                 # new exit-code table row
├── upgrade.md             # new numbered entry (exitGeneric → exitTooLarge is a behavior change)
```

### Pattern 1: Classify once via a gRPC unary client interceptor, composed with the existing chain

**What:** A `grpc.UnaryClientInterceptor` installed via `grpc.WithChainUnaryInterceptor` inside
`store.NewQdrantClient`'s base dial options (alongside the existing `grpc.WithStatsHandler`).
**When to use:** Any client-side transport failure that must be classified identically across every
RPC method, in production and every test, without touching call sites.
**Verified chain position:** `[VERIFIED: qdrant-go-client@v1.19.2 qdrant/grpc_client.go:34-55]`
qdrant-go-client's own dial options are built as
`[metadataInterceptor, rateLimitInterceptor] + keepAliveParams + config.GrpcOptions` — `config.GrpcOptions`
is `store.NewQdrantClient`'s `dialOpts` parameter, appended LAST. Combined with
`[VERIFIED: google.golang.org/grpc@v1.83.2 clientconn.go:517-536]`'s chaining rule ("first interceptor
is outermost, last is innermost… `WithUnaryInterceptor` is always prepended ahead of the whole
chain"), the phase's new interceptor — being inside `config.GrpcOptions`, appended after
qdrant's own two interceptors — becomes the **innermost** link: it sees the raw `status.Error`
directly from the network call, before qdrant's `getRateLimitInterceptor` (or any caller-supplied
interceptor added via the `opts ...grpc.DialOption` passthrough) gets a chance to touch it.

**Why the message-shape check matters here specifically:** if this phase's classifier relabeled
EVERY `ResourceExhausted` (code-only match), it would swallow the status BEFORE qdrant's own
`getRateLimitInterceptor` (which sits OUTSIDE it) ever saw the original status — silently breaking
that interceptor's own `retry-after` → `*QdrantResourceExhaustedError` conversion for a genuine
server-side rate-limit event. D-02's message-shape requirement is not just "don't mislabel a
different error class" — it is structurally necessary for qdrant-go-client's own pre-existing
feature to keep working, because our interceptor sits inside it in the call chain.

```go
// Source: pattern derived from qdrant-go-client@v1.19.2 qdrant/config.go:156-188
// (getRateLimitInterceptor — the ACTUAL shipped code this phase's interceptor sits inside of),
// read this session. This is a RECOMMENDED shape, not verified-shipped code.
func classifyResponseTooLarge(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	err := invoker(ctx, method, req, reply, cc, opts...)
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != grpccodes.ResourceExhausted {
		return err
	}
	msg := st.Message()
	if !strings.Contains(msg, "received message") && !strings.Contains(msg, "after decompression") {
		return err // a genuine server-side ResourceExhausted — D-02: do not relabel it
	}
	return newResponseTooLargeErr(method, msg) // composes ErrResponseTooLarge + GRPCStatus()
}
```

### Pattern 2: Multi-`%w` sentinel that is BOTH `errors.Is`-comparable AND `status.FromError`-inspectable

**What:** `store.ErrResponseTooLarge` is a plain `errors.New` sentinel (matching the file's own
established idiom — `ErrNotFound`, `ErrInvalidArgument`, `ErrAmbiguousShortID` are all this shape,
`[VERIFIED: internal/store/store.go:69,76,81]`); the interceptor returns a composed error wrapping
BOTH it and a `status.Newf(...).Err()`.
**Why this satisfies D-01's constraint:** `[VERIFIED: google.golang.org/grpc@v1.83.2
status/status.go:96-126]` — `status.FromError` recovers a `GRPCStatus()`-implementing error via
`errors.As` through an arbitrary `Unwrap()` chain (not just a direct type assertion), so this
survives being wrapped one more time by qdrant-go-client's own `*QdrantError` (whose `Unwrap()`
returns the inner error, `[VERIFIED: qdrant-go-client@v1.19.2 qdrant/error.go:24-26]`). One
caveat: when recovered via the `errors.As` (wrapped) path rather than a direct assertion,
`status.FromError(err).Message()` is **overwritten** with the outermost error's `.Error()` text
(`status/status.go:122-124`) — `.Code()` is unaffected. This means any code doing
`status.FromError(err).Code() == codes.ResourceExhausted` on the final `*QdrantError`-wrapped error
(e.g. a future caller mirroring `store.go:614`'s idiom) will work correctly; a caller inspecting
`.Message()` will see the wrapped chain's text, not the raw grpc-go string — which is actually a
mild, incidental assist toward D-03's "byte counts server-log-only" intent, though it must not be
relied on as the actual scrubbing mechanism (that is the Connect/MCP mapper's job, D-04).

```go
// Source: pattern only (not shipped) — composition style verified idiomatic against
// stdlib multi-%w (Go 1.20+, module pinned 1.26.7) and grpc-go's status package.
var ErrResponseTooLarge = errors.New("qdrant response exceeded the client's receive limit")

func newResponseTooLargeErr(method, rawMsg string) error {
	// rawMsg (the raw grpc-go text, possibly containing a byte ceiling) is carried ONLY
	// inside this composed error for SERVER-SIDE LOGGING (D-03) — never surfaced on any wire.
	return fmt.Errorf("%w: %w", ErrResponseTooLarge,
		status.Newf(grpccodes.ResourceExhausted, "%s: %s", method, rawMsg).Err())
}
```

### Pattern 3: One new `connectError` arm, respecting the existing ordering comment

**What:** `errors.Is(err, store.ErrResponseTooLarge) → connect.NewError(connect.CodeResourceExhausted, <D-04 envelope>)`.
**Verified constraint:** `[VERIFIED: internal/server/connecterror.go:59-68]` — the `errors.As(err,
&ae)`-first case MUST stay first; `*argError`'s own `Unwrap()` returns `store.ErrInvalidArgument`
(`[VERIFIED: internal/server/argerror.go:99-101]`), so if a future edit reordered the switch, every
class would silently collapse to `CodeInvalidArgument`. This phase's new sentinel is a DIFFERENT
sentinel (`store.ErrResponseTooLarge`, never wrapped as an `*argError`), so it does not collide with
that specific hazard — but the D-04 envelope text must still be produced through the SAME renderer
`*argError.Error()` already uses (`field=<name> hint=<code>: <detail>`), without constructing an
actual `*argError` (whose `ConnectCode()` only returns one of three codes — never
`CodeResourceExhausted` — and whose `Unwrap()` would misdirect `errors.Is(err,
store.ErrInvalidArgument)` checks elsewhere). The idiomatic resolution, given `argError.Error()`'s
string-building is currently NOT factored out as a standalone function
(`[VERIFIED: internal/server/argerror.go:85-94]`, the only renderer today is the method body
itself): extract that string-building into a small package-level helper (e.g.
`renderHintEnvelope(fields []string, hint HintCode, detail string) string`) that BOTH
`(*argError).Error()` and the new Connect/MCP mapping call — this is "ONE renderer" (D-04's
requirement) without constructing an `*argError` for a sentinel that must NOT go through its
`ConnectCode()`/`Unwrap()` machinery.

### Pattern 4: MCP mapper as a second `AddReceivingMiddleware`, ordered relative to `instrumentTools`

**What:** A new `mcp.Middleware` that, for `method == "tools/call"`, calls `next()`, inspects the
returned `*mcp.CallToolResult` via `res.GetError()`, and — on a match — rewrites `res.Content` to the
D-04 envelope while logging the raw error once, before returning.
**Verified mechanics:**
- `[VERIFIED: go-sdk@v1.8.0 mcp/server.go:419-434]` — every ordinary typed-handler error becomes
  `errRes.SetError(err); return &errRes, nil` — the `MethodHandler`-level Go `error` return is
  `nil`; the real error lives only in the unexported `CallToolResult.err` field, reachable via
  `GetError()`.
- `[VERIFIED: go-sdk@v1.8.0 mcp/protocol.go:337-349]` — `SetError` populates `Content` with
  `err.Error()` **only if `Content` was empty** — for an ordinary handler error this is always true,
  so `res.Content[0].Text` already carries the RAW error text (today's D-09-documented behavior:
  "MCP returns raw error text").
- `[VERIFIED: internal/server/instrument.go:47-61]` — `instrumentTools` reads `res, err :=
  next(...)` and its own `slog.ErrorContext` branch fires only `if err != nil` — which, per the
  point above, is **never true** for this error class. `instrumentTools` never mutates `res`.
  **Consequence:** wherever the new mapper sits relative to `instrumentTools`, it sees the
  UNMODIFIED, unscrubbed `res` (since `instrumentTools` neither logs nor rewrites it for this
  case) — so either ordering (innermost, added second in the same call; or outermost, added via a
  second call) satisfies D-08's "log exactly once," because `instrumentTools` contributes zero
  logging for this error class in EITHER ordering. Recommend **innermost** (edit the existing
  single call to `s.AddReceivingMiddleware(instrumentTools(tm.Record), <newMapper>)`, listing the
  new mapper SECOND) so it is the first thing to see the truly-raw result, closest to the tool
  dispatch — the more defensive choice if a THIRD middleware is ever added later.
- `[VERIFIED: go-sdk@v1.8.0 mcp/shared.go:133-137]` — `addMiddleware` iterates
  `slices.Backward(middleware)`: within one variadic call, the FIRST-listed argument ends up
  OUTERMOST; a middleware added via a LATER, SEPARATE `AddReceivingMiddleware` call always wraps
  AROUND everything registered by prior calls (becomes outermost). Both routes are available;
  editing the existing single call (Pattern above) is simpler than adding a second call site.

```go
// Source: pattern only — mechanics (GetError/SetError/IsError) verified against go-sdk@v1.8.0
// mcp/protocol.go:266-349, read this session.
func mapResponseTooLarge() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			res, err := next(ctx, method, req)
			ctr, ok := res.(*mcp.CallToolResult)
			if !ok || method != "tools/call" {
				return res, err
			}
			toolErr := ctr.GetError()
			if !errors.Is(toolErr, store.ErrResponseTooLarge) {
				return res, err
			}
			slog.ErrorContext(ctx, "mcp tool call: response too large", "err", toolErr) // D-08: log RAW once
			ctr.Content = []mcp.Content{&mcp.TextContent{Text: renderHintEnvelope(
				[]string{"response"}, HintTooLarge, "the result set is too large for one request; use a smaller limit/k, or omit full")}}
			ctr.IsError = true
			return ctr, err
		}
	}
}
```

### Anti-Patterns to Avoid

- **Constructing `store.ErrResponseTooLarge` as (or wrapped by) an `*argError`:** its `Unwrap()`
  returns `store.ErrInvalidArgument` and its `ConnectCode()` only knows three classes — this sentinel
  must never enter that type's machinery, or `connectError`'s `errors.As(err, &ae)`-first case will
  silently steal it and misclassify it as `CodeInvalidArgument`.
- **Matching on `codes.ResourceExhausted` alone:** breaks qdrant-go-client's own
  `getRateLimitInterceptor` (Pattern 1) for genuine server-side rate limiting, since our classifier
  sits inside it in the chain.
- **Assuming the grpc-go message always carries an "observed" byte count:** the single-number shape
  this project has actually observed (`rpc_util.go:1039`) carries only the configured limit, never
  the actual oversized size. A D-03 error type field named `Observed` will be unpopulated/zero for
  that shape — document this rather than silently defaulting it.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| gRPC status inspection | A custom error-classification package | `google.golang.org/grpc/status`+`codes`, already imported in `store.go` | Already the file's own idiom at `store.go:614` |
| A "GRPCStatus-carrying sentinel" type | A hand-written struct with `Is`/`Unwrap`/`GRPCStatus` methods | `fmt.Errorf("%w: %w", sentinel, status.Newf(...).Err())` | stdlib multi-`%w` (Go 1.20+) does exactly this with zero new types |
| MCP error-envelope string | A second hand-built `field=... hint=...` formatter | Extract `argError.Error()`'s existing grammar into a shared renderer function | D-04 explicitly requires ONE renderer; `argerror.go` does not yet expose one as a standalone function |
| Exit-code taxonomy | A parallel exit-code table for the new failure | `cmd/engram/client_common.go`'s existing `exitCodeForConnectErr` + `catalog.go`'s `doc.ExitCodes` (both already the single sources of truth, mechanically cross-checked by `TestCatalogExitCodesMatchMapper`) | Two already-shipped, gate-enforced single-sources-of-truth exist; a third list would immediately drift |

**Key insight:** every mechanism this phase needs is either already shipped in this exact codebase
(the `status.FromError` idiom, the `field=`/`hint=` envelope, the `exitCodeForConnectErr` mapper) or
a one-line stdlib/status-package composition. The actual difficulty is entirely in *where things sit
in an existing chain* (interceptor ordering, middleware ordering, switch-arm ordering) — not in
inventing new machinery.

## Common Pitfalls

### Pitfall 1: Matching only the two-number grpc-go message shape misses the shape this project has already observed live

**What goes wrong:** grpc-go v1.83.2 produces THREE distinct receive-side `ResourceExhausted`
message shapes depending on whether/how the response was compressed:
- `rpc_util.go:798` — `"grpc: received message larger than max (%d vs. %d)"` (uncompressed wire
  check; two numbers)
- `rpc_util.go:1008` — `"grpc: message after decompression larger than max (%d vs. %d)"` (legacy
  `Decompressor` interface path; two numbers)
- `rpc_util.go:1039` — `"grpc: received message after decompression larger than max %d"` (modern
  `encoding.Compressor` path; **one number**, no observed size)

`[VERIFIED: google.golang.org/grpc@v1.83.2/rpc_util.go:795-798,1006-1008,1036-1039, read this
session]`. Phase 1's own live RED capture (`01-04-SUMMARY.md`, a real `go test` run against real
Qdrant) recorded exactly the third, single-number shape: `"grpc: received message after
decompression larger than max 4194304"` — meaning Qdrant's responses in THIS environment are
compression-negotiated, and a classifier written only against the two-number shapes (the more
"obvious" ones to copy from grpc-go's error text) would never match this project's own actual
traffic.

**Why it happens:** the two-number shapes look more complete/informative and are easier to find by
searching grpc-go source for "ResourceExhausted"; the single-number shape is easy to miss unless the
actual observed traffic is checked (which this project's own Phase 1 test already did).

**How to avoid:** match on a substring common to all three receive shapes and absent from every
send-side shape — `strings.Contains(msg, "received message") || strings.Contains(msg,
"after decompression")` covers all three (verified against the literal strings above) while
excluding the four *send*-side "`trying to send message larger than max`" messages
(`stream.go:991,1496,1779`, `server.go:1215`), none of which contain "received" or "decompression".

**Warning signs:** a unit test suite for the classifier that only feeds it the two-number shape and
never the single-number one — it would pass, but silently fail to relabel the shape that actually
fires in this project's own CI.

**Phase to address:** this phase, in the classifier's own unit tests (synthetic statuses, per rule
`m45p2b4bp7` — never asserting grpc-go's own behavior as the test oracle, only proving the
classifier's OWN matching logic against constructed inputs covering all three shapes plus negative
cases).

### Pitfall 2: The classifier interceptor sits INSIDE qdrant-go-client's own rate-limit interceptor — a code-only match breaks it

**What goes wrong:** `qdrant.NewClient` already installs `getRateLimitInterceptor`
(`[VERIFIED: qdrant-go-client@v1.19.2 qdrant/config.go:156-188]`) as a `WithChainUnaryInterceptor`,
positioned BEFORE `config.GrpcOptions` in the dial-option list
(`[VERIFIED: qdrant/grpc_client.go:34-53]`). Since chained interceptors compose first-listed=outermost
(`[VERIFIED: grpc@v1.83.2 clientconn.go:517-536]`), and this phase's new interceptor lives inside
`config.GrpcOptions` (i.e. inside `store.NewQdrantClient`'s own `opts` parameter), the new
interceptor is **innermost** — it sees the RAW `status.Error` first, and whatever it RETURNS is what
`getRateLimitInterceptor` sees as `err` from calling its own `invoker`. If the new interceptor
relabeled every `codes.ResourceExhausted` regardless of message shape, `getRateLimitInterceptor`'s
own `status.FromError(err); st.Code() != codes.ResourceExhausted` check would still pass (our
sentinel preserves the code via `GRPCStatus()`) — but `st.Message()` would now be OUR composed
message, not the original, and if a genuine server-side rate-limit event's message happened to
NOT match our (over-broad) filter, we'd have already stolen it before qdrant's own trailer-based
logic ever ran.

**Why it happens:** it's easy to reason about "my interceptor" in isolation and forget that
`qdrant.NewClient` already wires its own interceptors around whatever `config.GrpcOptions` supplies.

**How to avoid:** always pass through unchanged (`return err`) unless BOTH the code AND the
message-shape match (D-02, already locked) — this makes the interceptor's failure-to-match case
byte-for-byte transparent to everything downstream in the chain, including qdrant's own logic.

**Warning signs:** a test asserting the classifier's behavior in isolation (feeding it a synthetic
status directly) without ALSO asserting, via a real end-to-end Qdrant round trip, that a
non-message-shape-matching `ResourceExhausted` still reaches `connectError`'s DEFAULT arm (not the
new one) — the isolation test alone cannot catch an over-broad match that happens to still return
something with the right `.Code()`.

**Phase to address:** this phase — the CONTEXT's own "Claude's Discretion" section already asks for
"a unit test of the classifier against synthetic statuses proving a non-matching ResourceExhausted
is NOT relabeled," which is exactly the right test; this pitfall explains WHY it matters beyond the
stated D-02 requirement alone.

### Pitfall 3: `instrumentTools` does not currently log a tool-call business error at all — D-08's "still logged" is new behavior, not preserved behavior

**What goes wrong:** `[VERIFIED: internal/server/instrument.go:47-61]` — `instrumentTools`'s
`slog.ErrorContext(ctx, "tool call failed", ...)` branch is gated on `err != nil` at the
`MethodHandler` return, which per Pitfall/Pattern above is `nil` for every ordinary `AddTool` typed
handler error (the error lives in `CallToolResult.err`, not the Go `error` return). A plan that
assumes "instrumentTools already logs raw tool errors, so the new mapper just needs to run
*before* it (or after) so it doesn't clobber that existing log line" is planning against behavior
that does not exist.

**Why it happens:** `classifyOutcome`'s METRIC does correctly account for `res.IsError`, which
creates the appearance that error visibility is already handled — but the metric and the slog line
are two independent code paths, and only the metric currently inspects `IsError`.

**How to avoid:** the new MCP mapper (this phase's own new middleware) must be the thing that logs
the raw error, not something that defers to `instrumentTools` doing it. Either middleware ordering
(innermost or outermost relative to `instrumentTools`) achieves "logged exactly once," because
`instrumentTools` contributes zero logging either way for this error class — the "log once"
constraint is trivially satisfiable, not a genuine ordering hazard, once this fact is known.

**Warning signs:** a plan step that says "verify instrumentTools's existing error log still fires"
— it never fired for this error class before this phase, so there is nothing to "still" do; the new
middleware is the FIRST log line this error class has ever gotten server-side.

**Phase to address:** this phase, when writing the new MCP mapper.

### Pitfall 4: Adding `exitTooLarge` to `exitCodeForConnectErr` without updating `catalog.go` breaks a MECHANICAL gate immediately

**What goes wrong:** `[VERIFIED: cmd/engram/catalog_test.go:370-391]` — `TestCatalogExitCodesMatchMapper`
derives `mapperCodes` by calling `exitCodeForConnectErr` across every `connect.Code` 1-16
PROGRAMMATICALLY (not from a hand-maintained list) and asserts SET EQUALITY against `catalog.go`'s
hand-authored `doc.ExitCodes` slice. The moment `exitCodeForConnectErr` gains a `case
connect.CodeResourceExhausted: return exitTooLarge`, this test's derived `mapperCodes` set gains
`10` automatically — and fails immediately unless `catalog.go`'s `doc.ExitCodes` literal
ALSO gains a matching `{Code: exitTooLarge, Meaning: "..."}` row, in the same commit.

**Why it happens:** the two files are edited for different reasons (one for the mapper logic, one
for user-facing documentation) and it is easy to land the first without remembering the second.

**How to avoid:** treat `client_common.go` (constant + switch case), `client_common_test.go`
(`TestExitCodeForConnectErrTable`'s literal row, line 46, currently `{connect.CodeResourceExhausted,
exitGeneric}`), and `catalog.go` (`doc.ExitCodes` row) as ONE atomic edit. Then regenerate goldens:
`go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -update -count=1`
(`[VERIFIED: Taskfile.yaml:281, "task gen:golden"]`) or both `TestHelpGolden` and `TestCatalogGolden`
go red against the stale committed fixture.

**Warning signs:** `go test ./cmd/engram/...` failing on `TestCatalogExitCodesMatchMapper` or
`TestCatalogGolden`/`TestHelpGolden` after only touching `client_common.go`.

**Phase to address:** this phase, CLI lane task.

### Pitfall 5: `errors.md`'s "ten hint codes" claim has NO mechanical gate today — a missed doc edit will not be caught by any test

**What goes wrong:** `errors.md`'s own text says the hint-code table is "checked off one by one
against that file" (`argerror.go`) — this reads as if a test enforces it. `[VERIFIED: searched
internal/server/*_test.go and internal/store/*_test.go for any test comparing errors.md's hint
table against argerror.go's HintCode constants — none found, read this session]`. The only doc-gate
test touching `errors.md` (`supersededocs_test.go`'s `TestSupersedeDocsMatchShippedContract`) checks
the two `supersede_memory` worked-example STRINGS, not the hint-code table or its count. If Phase 2
adds `HintTooLarge` to `argerror.go` but forgets to update `errors.md`'s "The ten hint codes"
heading (→ eleven) and table, nothing in `task test` will catch the omission.

**Why it happens:** the doc page's own confident phrasing ("checked off... this table cannot list a
code the server does not emit") describes an aspiration, not a shipped gate — easy to trust at face
value without checking.

**How to avoid:** update `errors.md`'s heading, table, and the "class-to-Connect-code" section (as a
NEW, separate row/callout — `resource_exhausted` is not one of the three `argError` classes, so it
must not be folded into the existing three-row table that says "all three map to exitUsage").
Optionally (not required by D-05, but closes exactly this gap): add a small test asserting
`len(HintCode consts)` == the count `errors.md`'s "The ten/eleven hint codes" heading states, or that
every `HintCode` string constant appears in the doc's table. Flag this as an option for the planner,
not a lock — CONTEXT does not require it.

**Warning signs:** `errors.md` still says "ten hint codes" or omits `too_large` after
`argerror.go` ships the 11th constant, with `task test` fully green.

**Phase to address:** this phase, docs task — and optionally, a new self-check test, at the
planner's discretion.

### Pitfall 6: The observed byte count is not always available from grpc-go's own error text

**What goes wrong:** D-03 asks for an error type "carrying the RPC method and the
observed/limit byte counts." Per Pitfall 1, the single-number shape this project has actually
observed (`rpc_util.go:1039`) carries ONLY the limit, never the observed size — there is no way to
recover the actual oversized byte count from that specific grpc-go message text.

**Why it happens:** grpc-go's own message text is not designed as a machine-parseable API; its
format varies by code path and was not written with this project's D-03 requirement in mind.

**How to avoid:** design the D-03 error type so "observed" is optional/best-effort (parsed from the
message when the two-number shape happens to fire; absent otherwise) rather than assuming it is
always populated. The RPC method name IS always reliably available — it's the interceptor's own
`method` parameter, never parsed from text.

**Warning signs:** a D-03 error type with a required (non-pointer, non-`-1`-sentineled) `Observed
int` field and a test that only exercises the two-number shape, never noticing the field goes to
zero silently under the single-number shape.

**Phase to address:** this phase, when designing the D-03 detail type.

### Pitfall 7: `TestRedEvidencePatchesAreLive` is GREEN today, not red — do not "fix" a gate that isn't broken

**What goes wrong:** it would be reasonable to assume, from the phase's own goal framing, that this
gate is currently failing and needs Phase 2 to "close" it. `[VERIFIED: this session ran `go test
-run '^TestRedEvidencePatchesAreLive$' -count=1 ./internal/store/...` against the current working
tree — result: PASS, all four of Phase 1's registered patches still reproduce RED as expected]`. The
gate's own empty-map guard (`redevidence_harness_test.go:200-212`) only fires if `redEvidenceDirs`
is EMPTY while an active-milestone phase directory exists — and it is not empty (Phase 1's entry is
present and valid).

**Why it happens:** the phase-goal text conflates "this phase's own directory has no red-evidence
registered yet" (true — Phase 2 hasn't shipped a patch) with "the gate is red" (false — the gate
only cares about what IS registered, and Phase 1's registration is still valid).

**How to avoid:** Phase 2's actual obligation is the same as every phase's: at the END (last plan),
add ONE new entry to `redEvidenceDirs` mapping this phase's own red-evidence directory to its own
new patches, each hand-verified (`git apply --check`/`apply`/`go test -run '^Target$'`/`apply -R`)
before registration — exactly Phase 1's own precedent (`01-05-SUMMARY.md`).

**Warning signs:** a plan task that reads "make `TestRedEvidencePatchesAreLive` pass" as if it is
currently failing — verify with a live `go test` run before writing that framing into a plan.

**Phase to address:** this phase, last plan (matching Phase 1's own sequencing).

## Code Examples

Already covered inline under Architecture Patterns above (Patterns 1-4) — each labeled with its
verified source citations. No further standalone examples needed; the four patterns are the
complete mechanism this phase builds.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Connect returns `internal` (HTTP 500) for a receive-limit overflow | `resource_exhausted` (HTTP 429) with a scrubbed hint envelope | This phase | Existing Connect clients branching on `CodeInternal` for this case must widen to `CodeResourceExhausted`; documented as an additive-but-visible change (D-04) |
| `exitCodeForConnectErr` maps `CodeResourceExhausted → exitGeneric (1)` | `→ exitTooLarge (10)` | This phase | A one-way, script-visible exit-code contract change (D-07) — must be documented in `guides/upgrade.md` alongside the exit-code-6/exit-code-7 precedents already there |
| MCP tools return raw Go error text for every failure | THIS ONE sentinel gets a scrubbed envelope; every other error is unchanged (D-09, deferred scope) | This phase | The Connect/MCP lane inconsistency (Connect scrubs unknowns to `internal error`; MCP still returns raw text for everything else) is explicitly NOT fixed here — recorded as a deferred backlog item per D-09 |

**Deprecated/outdated:** none — this is pure additive-error-surface work; nothing existing is
removed.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The recommended `renderHintEnvelope` extraction (Pattern 3) is the best way to satisfy D-04's "ONE renderer" without constructing an `*argError` | Architecture Patterns, Pattern 3 | LOW — this is a code-shape recommendation, not a locked decision; the planner/executor may choose an equivalent factoring (e.g. a shared unexported helper method) as long as the two call sites (argError.Error and the new mapping) share one string-building implementation, which is the actual D-04 requirement |
| A2 | Innermost middleware ordering (edit the existing `AddReceivingMiddleware` call to list the new mapper second) is preferable to a second separate call | Architecture Patterns, Pattern 4 | LOW — both orderings verified to satisfy D-08's "log once" constraint equally; this is a stylistic recommendation the CONTEXT explicitly leaves to Claude's Discretion |
| A3 | Adding a mechanical test tying `errors.md`'s hint-code count to `argerror.go`'s `HintCode` constants is optional, not required | Common Pitfalls, Pitfall 5 | LOW — D-05 does not require this; flagged only as a risk-closing option for the planner to accept or decline |

**If this table is empty:** N/A — see above; all three entries are LOW-risk implementation-shape
recommendations, not unverified factual claims. Every factual claim about grpc-go/qdrant-go-client/
go-sdk/connect-go behavior in this document was verified this session by reading the pinned module
source directly (see inline `[VERIFIED: ...]` tags) — none rests on training-data recall of
third-party behavior (per rule `m45p2b4bp7`, the classifier's OWN matching logic — not any
third-party default — is what future tests must assert against).

## Open Questions

1. **Exact D-03 detail-type field names and shape.**
   - What we know: it must carry the RPC method (always available) and, best-effort, an
     observed/limit byte count pair (limit always available from the message text; observed only
     when the two-number shape fires — see Pitfall 6).
   - What's unclear: whether the planner wants a single unexported struct type, or the multi-`%w`
     composition alone (Pattern 2) is sufficient — CONTEXT's D-03 says "an error type carrying...",
     which the multi-`%w` composition around a formatted `status.Newf` message already satisfies
     without a separate struct, but a struct would make `Observed`/`Limit` programmatically
     queryable server-side (e.g. for a future metric) rather than only string-embedded.
   - Recommendation: default to the multi-`%w` composition (simplest, matches D-00's idiom-over-effort
     framing) unless the plan surfaces a concrete need to query the byte counts programmatically
     server-side beyond a log line.

2. **Whether to add the optional hint-code-count self-check test (Pitfall 5).**
   - What we know: no such gate exists today; `errors.md`'s claim of being "checked off" is
     currently just prose.
   - What's unclear: whether closing this gap is in this phase's scope or a separate backlog item —
     D-05 only requires the doc edit itself, not a new gate.
   - Recommendation: mention it as a plan-time checkpoint option (cheap to add, closes a real
     drift risk class), but do not block the phase on it.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker (testcontainers) or a reachable Qdrant at `ENGRAM_QDRANT_TEST_ADDR` | Every real-overflow regression test (store/Connect/MCP/CLI lanes) via `storetest.Dial`/`storetest.SeedOversized` | ✓ (verified this session — Docker Desktop 29.8.0 reachable, testcontainers booted a real `qdrant/qdrant:v1.19.1` container and ran `TestRedEvidencePatchesAreLive` successfully) | qdrant/qdrant:v1.19.1 (pinned, `storetest.QdrantImage`) | None needed — already available |
| `go` toolchain 1.26.7 | Building/testing | ✓ (module `go.mod` pins it; the working tree already builds) | — | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none — everything needed was verified present and working
in this session.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (table-driven), real-Qdrant integration via `internal/store/storetest` |
| Config file | none — `go.mod`/`Taskfile.yaml` govern build/test invocation |
| Quick run command | `go test ./internal/store/... ./internal/server/... ./cmd/engram/... -short -count=1` |
| Full suite command | `task` (lint + test; runs `TestRedEvidencePatchesAreLive` and other `-short`-skipped integration tests) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-exhausted-sentinel | Classifier matches code+message-shape; does NOT relabel a non-matching ResourceExhausted | unit (synthetic statuses) | `go test ./internal/store/ -run '^Test<ClassifierName>$' -v -count=1` | ❌ Wave 0 — new test file, name TBD at plan time; `go test -list '.*' ./internal/store/...` confirmed no existing test with this name today `[VERIFIED: ran this session]` |
| REQ-exhausted-sentinel | End-to-end: `Store.List` over an oversized scope (via `storetest.SeedOversized`) returns `errors.Is(err, store.ErrResponseTooLarge)` | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^Test<E2EName>$' -v -count=1` | ❌ Wave 0 |
| REQ-exhausted-connect | `ListMemories` over the same oversized fixture returns `connect.CodeResourceExhausted`, message scrubbed of raw grpc text | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^Test<ConnectName>$' -v -count=1` | ❌ Wave 0 — `testDepsWithStore(t)` (`[VERIFIED: internal/server/tools_test.go:207-215]`) is the existing helper to build the Qdrant-backed `deps`; reuse directly |
| REQ-exhausted-mcp | `list_memory` tool call over the same fixture returns `field=response hint=too_large: ...` via the new middleware, raw error logged once | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^Test<MCPName>$' -v -count=1` | ❌ Wave 0 |
| REQ-exhausted-cli-docs | `exitCodeForConnectErr(CodeResourceExhausted) == exitTooLarge` | unit (existing table, extended) | `go test ./cmd/engram/ -run '^TestExitCodeForConnectErrTable$' -v -count=1` | ✅ EXISTING (`[VERIFIED: cmd/engram/client_common_test.go:34-72]` — row 46 needs its `want` value changed, not a new file) |
| REQ-exhausted-cli-docs | `catalog.go`'s exit-code table matches the mapper (mechanical) | unit | `go test ./cmd/engram/ -run '^TestCatalogExitCodesMatchMapper$' -v -count=1` | ✅ EXISTING (`[VERIFIED: cmd/engram/catalog_test.go:370-391]`) |
| REQ-exhausted-cli-docs | `engram list` against a real oversized-fixture server exits 10 | integration (full binary + real server + real Qdrant) | `go test ./internal/e2e/ -run '^TestCLIExitCodes$' -v -count=1` (extend the existing table) or a new dedicated e2e test using `startServer` (`[VERIFIED: internal/e2e/harness_test.go:236]`) | ✅ EXISTING FILE, subtest TBD (`[VERIFIED: internal/e2e/cli_exitcode_test.go:49]`) |

All `-run` patterns above were re-resolved against `go test -list '.*' ./...` this session
(`[VERIFIED: ran this session]`) for the EXISTING tests named; new-test names are marked ❌ Wave 0
since they do not exist yet and the plan will name them.

### Sampling Rate

- **Per task commit:** `go test ./internal/store/... ./internal/server/... ./cmd/engram/... -short -count=1`
- **Per wave merge:** `task` (full suite, includes `TestRedEvidencePatchesAreLive` and every
  real-Qdrant integration test, `-short` NOT passed)
- **Phase gate:** full suite green, plus a live-observed RED/GREEN pair for each of the four lanes'
  new tests (store, Connect, MCP, CLI) before registering this phase's red-evidence patches

### Wave 0 Gaps

- [ ] `internal/store/<name>_test.go` — classifier unit test (synthetic statuses, all 3 message
  shapes + negative cases) — covers REQ-exhausted-sentinel
- [ ] `internal/store/<name>_test.go` or extend `store_test.go`/`listscopes_oversized_test.go`'s
  sibling — end-to-end `Store.List` overflow via `storetest.SeedOversized` — covers
  REQ-exhausted-sentinel
- [ ] `internal/server/<name>_test.go` — Connect-lane end-to-end (`testDepsWithStore` + real
  overflow) — covers REQ-exhausted-connect
- [ ] `internal/server/<name>_test.go` — MCP-lane end-to-end — covers REQ-exhausted-mcp
- [ ] `cmd/engram/client_common_test.go` row 46 edit (existing file, not a gap in coverage, a
  required edit) — covers REQ-exhausted-cli-docs
- [ ] `internal/e2e/cli_exitcode_test.go` new subtest (or new file) — full-stack exit-10 proof —
  covers REQ-exhausted-cli-docs
- [ ] This phase's own `redEvidenceDirs` entry + hand-verified patches (last plan, per Pitfall 7)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | unaffected — this phase is error-classification only |
| V3 Session Management | no | unaffected |
| V4 Access Control | no | unaffected |
| V5 Input Validation | no (this phase classifies a RESPONSE overflow, not a request) | — |
| V7 Error Handling and Logging (ASVS 4.x V7 / legacy V7 "Error Handling") | **yes** | The core of D-03/D-04: raw upstream error text (byte ceilings, internal transport details) must reach ONLY server-side logs, never a client-facing wire response — exactly the pattern this project's own `argError`/`connectError` "generic message, log detail server-side" convention already implements for the `default` arm (`connecterror.go:96-98`) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Information disclosure via raw gRPC/Qdrant error text (byte ceiling, backend transport identity) reaching a Connect/MCP client | Information Disclosure | D-04's scrubbed `field=response hint=too_large: <generic remedy text>` envelope, never echoing `st.Message()`/the raw grpc-go string on either wire — verified this project's existing `connectError` default arm already does exactly this for unknown errors (`slog.ErrorContext` + a generic `"internal error"` message) |
| A casual code-only match mis-relabeling a genuine server-side resource exhaustion as "your request was too big," misleading an operator away from the real remedy (server capacity, not client request shape) | Denial of Service (operator misdiagnosis, not a technical DoS) | D-02's message-shape match, verified structurally necessary in this session's reading of `getRateLimitInterceptor` (Pitfall 2) — not merely a nice-to-have |

## Sources

### Primary (HIGH confidence — module source read directly this session)
- `google.golang.org/grpc@v1.83.2` — `rpc_util.go` (lines 785-1049: `recvMsg`, `recvAndDecompress`,
  `decompress`, `recv` — all three receive-side `ResourceExhausted` message shapes and their exact
  trigger conditions), `stream.go`/`server.go` (send-side shapes, for exclusion), `dialoptions.go`
  (`WithUnaryInterceptor`/`WithChainUnaryInterceptor` semantics, lines 585-602), `clientconn.go`
  (`chainUnaryClientInterceptors`, lines 517-546), `status/status.go` (`FromError`, lines 96-126)
- `connectrpc.com/connect@v1.21.0` — `code.go` (`CodeResourceExhausted` value/wire-name, lines
  67-70,126-127)
- `github.com/modelcontextprotocol/go-sdk@v1.8.0` — `mcp/protocol.go` (`CallToolResult`,
  `SetError`/`GetError`, lines 261-370), `mcp/server.go` (typed-handler error wrapping, lines
  380-434), `mcp/shared.go` (`addMiddleware`'s ordering, lines 133-137)
- `github.com/qdrant/go-client@v1.19.2` — `qdrant/points.go` (every `Client.*` method's
  `newQdrantErr` wrap, lines 19-94), `qdrant/error.go` (`QdrantError`/`QdrantResourceExhaustedError`
  and their `Unwrap()`), `qdrant/config.go` (`getMetadataInterceptor`/`getRateLimitInterceptor`,
  lines 141-188), `qdrant/grpc_client.go` (dial-option composition order, lines 34-55)
- This repo's own working tree at this session's HEAD: `internal/store/store.go` (`NewQdrantClient`
  lines 518-523; the `AlreadyExists` idiom line 614; `List` lines 1372-1476; the `ListScopes`
  root-cause comment lines 1690-1695; sentinel declarations lines 69-102),
  `internal/server/connecterror.go` (full file), `internal/server/argerror.go` (full file),
  `internal/server/instrument.go` (full file), `internal/server/tools.go` (line 2262 registration
  site, line 2416 `list_memory` registration, line 1467 `d.st.List` call), `internal/server/connectapi.go`
  (lines 235-280, `ListMemories`), `cmd/engram/client_common.go` (lines 200-447),
  `cmd/engram/client_common_test.go` (lines 31-72), `cmd/engram/catalog.go` (lines 115-157),
  `cmd/engram/catalog_test.go` (lines 335-391), `internal/store/redevidence_harness_test.go` (full
  file), `internal/store/storetest/storetest.go` (full file), `internal/server/tools_test.go` (lines
  190-215), `cmd/engram/client_list_test.go` (lines 218-243), `internal/e2e/harness_test.go` (lines
  78-260), `internal/e2e/cli_exitcode_test.go` (lines 49-90), `internal/surfaces/rules.go` (full
  file), `docs-site/src/content/docs/reference/errors.md` (full file), `docs-site/src/content/docs/guides/cli.md`
  (lines 353-378), `docs-site/src/content/docs/guides/upgrade.md` (exit-code precedent entries)
- Live commands run this session: `go test -run '^TestRedEvidencePatchesAreLive$' -count=1 -v
  ./internal/store/...` (PASS, all four Phase 1 patches confirmed RED-then-reverted);
  `go test -list '.*' ./internal/store/... ./internal/server/... ./cmd/engram/...` (confirmed no
  pre-existing test names collide with this phase's planned new tests); `rg` scans confirming no
  test ties `errors.md`'s hint-code table to `argerror.go`'s constants

### Secondary (MEDIUM confidence)
- `.planning/phases/01-test-harness-fixture-helper/01-04-SUMMARY.md` — the live RED capture of the
  exact single-number grpc-go message text this project's own traffic produces (a project-internal
  record of a prior session's real `go test` run, not re-run in this session but corroborated
  against the grpc-go source read directly here)

### Tertiary (LOW confidence)
- None — every claim in this document was either verified against pinned module source read this
  session, verified against this repo's own working tree read this session, or verified by a live
  command run this session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; every API verified against pinned module source
- Architecture: HIGH — the interceptor-chain-ordering and middleware-ordering findings were derived
  from reading the actual composition code in grpc-go, qdrant-go-client, and go-sdk, not inferred
- Pitfalls: HIGH — five of seven pitfalls are grounded in direct source reads or live command output
  this session; the remaining two (design-shape recommendations) are explicitly logged as
  LOW-risk Assumptions, not asserted as fact

**Research date:** 2026-09-19
**Valid until:** 30 days, OR immediately upon any bump of `google.golang.org/grpc`,
`github.com/qdrant/go-client`, `github.com/modelcontextprotocol/go-sdk`, or `connectrpc.com/connect`
in `go.mod` — the message-shape and interceptor-ordering findings are version-specific and must be
re-verified against the pinned source on any such bump.
