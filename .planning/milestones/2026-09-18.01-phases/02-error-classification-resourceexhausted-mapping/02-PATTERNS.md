# Phase 2: Error Classification & ResourceExhausted Mapping - Pattern Map

**Mapped:** 2026-09-19
**Files analyzed:** 11 (7 to modify, ~4 new)
**Analogs found:** 10 / 11

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/store/store.go` (new sentinel + interceptor, `NewQdrantClient` edit) | middleware (gRPC client interceptor) + model (sentinel) | transform (status classification) | `internal/store/store.go:591-621` (`ensureIndexes`'s `status.FromError`/`grpccodes.AlreadyExists` idiom) + `internal/store/store.go:69-102` (sentinel declarations) | exact |
| `internal/store/<name>_classifier_test.go` (new, unit) | test | transform | none in-repo (synthetic-status classifier test) — build from RESEARCH.md Pattern 1/Pitfall 1 | no analog |
| `internal/store/<name>_e2e_test.go` or extend `listscopes_oversized_test.go`'s sibling (new, integration) | test | request-response (real overflow) | `internal/store/listscopes_oversized_test.go` (full file) | exact |
| `internal/server/connecterror.go` (new switch arm) | middleware (error mapper) | request-response | `internal/server/connecterror.go` (full file, `connectError`) | exact |
| `internal/server/argerror.go` (new `HintTooLarge` const + extracted renderer helper) | utility (envelope grammar) | transform | `internal/server/argerror.go` (full file, `argError.Error()`) | exact |
| `internal/server/<new>.go` (MCP receiving-middleware mapper) | middleware | request-response | `internal/server/instrument.go` (full file, `instrumentTools`) | exact |
| `internal/server/tools.go` (edit `AddReceivingMiddleware` call site) | route/config (registration) | request-response | `internal/server/tools.go:2262` (`Register`'s existing call) | exact |
| `internal/server/<name>_test.go` (Connect-lane e2e) | test | request-response | `internal/server/tools_test.go:207-215` (`testDepsWithStore`) | exact |
| `internal/server/<name>_test.go` (MCP-lane e2e) | test | request-response | `internal/server/instrument.go` + `internal/server/tools_test.go:207-215` (combined) | role-match |
| `cmd/engram/client_common.go` (new `exitTooLarge` const + switch case) | config/utility (exit-code taxonomy) | transform | `cmd/engram/client_common.go:216-252,432-446` (existing constants + `exitCodeForConnectErr`) | exact |
| `cmd/engram/client_common_test.go` (edit row 46) | test | transform | `cmd/engram/client_common_test.go:31-68` (`TestExitCodeForConnectErrTable`) | exact |
| `cmd/engram/catalog.go` (new `doc.ExitCodes` row) | config (self-describe catalog) | transform | `cmd/engram/catalog.go:119-155` (`doc.ExitCodes` literal) | exact |
| `cmd/engram/catalog_test.go` (no code change expected; gate re-verifies) | test | transform | `cmd/engram/catalog_test.go:344-391` (`TestCatalogExitCodesMatchMapper`) | exact (gate, not edited unless allowlist needed) |
| `internal/e2e/cli_exitcode_test.go` (new subtest) | test | request-response (full binary) | `internal/e2e/cli_exitcode_test.go:44-90` (`TestCLIExitCodes`) | exact |
| `docs-site/src/content/docs/reference/errors.md` (hint table + class-to-Connect-code section edits) | config/doc | transform | `docs-site/src/content/docs/reference/errors.md:94-204` (hint-code table, class-to-Connect-code mapping, "one exit code with no counterpart" precedent) | exact |
| `docs-site/src/content/docs/guides/cli.md` (exit-code table row) | config/doc | transform | `docs-site/src/content/docs/guides/cli.md:353-378` ("## Exit codes" table) | exact |
| `docs-site/src/content/docs/guides/upgrade.md` (new numbered entry) | config/doc | transform | `docs-site/src/content/docs/guides/upgrade.md:126-133` (entry 4, exit-code-6 precedent) | exact |
| `internal/store/redevidence_harness_test.go` (`redEvidenceDirs` new entry, last plan only) | test/config | transform | `internal/store/redevidence_harness_test.go:106-113` (Phase 1's own entry) | exact |

## Pattern Assignments

### `internal/store/store.go` — sentinel + classifier interceptor (middleware, transform)

**Analog 1 (sentinel shape):** `internal/store/store.go:69-102`

```go
// ErrNotFound is returned when an id is absent OR not visible to the caller —
// the two are indistinguishable by design, so ownership never leaks across actors.
var ErrNotFound = errors.New("not found")

// ErrInvalidArgument tags errors caused by a malformed caller request ...
var ErrInvalidArgument = errors.New("invalid argument")

// ErrAmbiguousShortID means a short id matched more than one record — an
// invariant violation ...
var ErrAmbiguousShortID = errors.New("ambiguous short id")
```
Copy this exact idiom for the new sentinel: a package-level `var Err... = errors.New("...")` with a doc comment explaining WHY it's a distinct sentinel (not folded into an existing one) — same shape `ErrIdempotencyConflict`/`ErrAlreadySuperseded` use a few lines below (lines ~90-102).

**Analog 2 (status.FromError idiom to extend, `store.go:591-621`):**
```go
if _, err := s.client.CreateFieldIndex(ctx, req); err != nil {
    if st, ok := status.FromError(err); ok && st.Code() == grpccodes.AlreadyExists {
        continue
    }
    return fmt.Errorf("ensure index %q: %w", ix.field, err)
}
```
The new classifier interceptor's `status.FromError(err)` / `st.Code() == grpccodes.ResourceExhausted` check is the SAME idiom — copy the `ok &&` guard style, not a bare type assertion.

**Analog 3 (`NewQdrantClient` base dial options, `store.go:518-523`):**
```go
func NewQdrantClient(host string, port int, opts ...grpc.DialOption) (*qdrant.Client, error) {
	dialOpts := make([]grpc.DialOption, 0, 1+len(opts))
	dialOpts = append(dialOpts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	dialOpts = append(dialOpts, opts...)
	return qdrant.NewClient(&qdrant.Config{Host: host, Port: port, GrpcOptions: dialOpts})
}
```
D-01 requires the new interceptor added to `dialOpts` via `grpc.WithChainUnaryInterceptor(...)` BEFORE `append(dialOpts, opts...)` (base options block, matching the doc comment above this func: "applies the shared base dial options... FIRST, then appends the caller's own opts"). Do not touch the `opts...` passthrough itself.

**Core pattern (classifier, from RESEARCH.md Pattern 1/2 — no shipped precedent, use verbatim as starting shape):**
```go
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
	return newResponseTooLargeErr(method, msg)
}

var ErrResponseTooLarge = errors.New("qdrant response exceeded the client's receive limit")

func newResponseTooLargeErr(method, rawMsg string) error {
	return fmt.Errorf("%w: %w", ErrResponseTooLarge,
		status.Newf(grpccodes.ResourceExhausted, "%s: %s", method, rawMsg).Err())
}
```
Match ALL THREE grpc-go receive-shape substrings via `strings.Contains(msg, "received message") || strings.Contains(msg, "after decompression")` (Pitfall 1) — never just the two-number shape.

**Imports pattern** (already present, `store.go:6-30`): `errors`, `fmt`, `strings`, `grpccodes "google.golang.org/grpc/codes"`, `"google.golang.org/grpc/status"`, `"google.golang.org/grpc"` — all already imported; no new import lines expected beyond what's already there.

**Error handling:** pass-through-unchanged (`return err`) on any non-match — never wrap/relabel speculatively (Pitfall 2: qdrant-go-client's own `getRateLimitInterceptor` sits OUTSIDE this interceptor in the chain and depends on an untouched `err` for genuine server-side exhaustion).

---

### `internal/store/<name>_classifier_test.go` (test, transform) — NO ANALOG

No existing test in this repo drives a classifier against synthetic `status.Error` values. Build from RESEARCH.md's own Pattern 1 code block and Pitfall 1/2's negative-case guidance: table-driven, one subtest per grpc-go message shape (all 3 receive shapes) plus a negative case (a `ResourceExhausted` status with unrelated text, proving no relabel) plus a non-`ResourceExhausted` code case. Follow this repo's general table-test idiom, e.g. `cmd/engram/client_common_test.go:31-68`'s `cases := []struct{...}{}` + `t.Run(name, func(t *testing.T){...})` shape.

---

### `internal/store/<name>_e2e_test.go` (test, request-response, real overflow)

**Analog:** `internal/store/listscopes_oversized_test.go` (full file)

```go
func TestListScopesFullPayloadsOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_listscopes_" + uuid.NewString())
			st := store.NewTestStore(t, c, name)
			ctx := context.Background()
			if err := st.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("EnsureCollection: %v", err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("DeleteCollection(%q): %v", name, err)
				}
			})
			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})
			scopes, _, err := st.ListScopes(ctx, store.Authenticated(fx.Owner))
			if err != nil {
				t.Fatalf("ListScopes: %v (full-payload scroll exceeded the %d-byte named receive limit)", err, storetest.RecvLimit)
			}
			// ...
		})
	}
}
```
Copy this EXACT skeleton but replace the assertion: instead of asserting `ListScopes` succeeds, call `st.List` (or another Store method) against a fixture sized so it genuinely overflows, and assert `errors.Is(err, store.ErrResponseTooLarge)` plus (per D-01) that `status.FromError(err)` still recovers `codes.ResourceExhausted`. Package is `store_test` (external test package) — same import block: `context`, `testing`, `github.com/google/uuid`, `github.com/seanb4t/engram/internal/store`, `github.com/seanb4t/engram/internal/store/storetest`.

**storetest fixture pattern** — `internal/store/storetest/seed.go:179-230` (`SeedOversized`) and `storetest.go:43` (`RecvLimit = 4 << 20`): always dial with `storetest.Dial(t, storetest.RecvLimit)` and seed with `storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: storetest.FewLarge, Vector: [...]})` — never hand-roll a receive-limit constant (rule `m45p2b4bp7`).

---

### `internal/server/connecterror.go` — new switch arm (middleware, request-response)

**Analog:** `internal/server/connecterror.go` (full file, `connectError`)

**Imports pattern** (lines 6-14):
```go
import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/seanb4t/engram/internal/store"
)
```

**Core pattern — the switch, with the mandatory ordering comment (lines 55-68):**
```go
func connectError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	var ae *argError
	switch {
	// MUST stay first, before the errors.Is(err, store.ErrInvalidArgument)
	// case below: ...
	case errors.As(err, &ae):
		return connect.NewError(ae.ConnectCode(), err)
	case errors.Is(err, store.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	// ... existing arms ...
	}
}
```
Add the new arm as its OWN case, anywhere AFTER the `errors.As(err, &ae)` case (order among the `errors.Is` arms is not itself load-bearing — only staying after the `argError` case matters, per the comment at lines 61-66):
```go
case errors.Is(err, store.ErrResponseTooLarge):
	return connect.NewError(connect.CodeResourceExhausted, errors.New(renderHintEnvelope(
		[]string{"response"}, HintTooLarge,
		"the result set is too large for one request; use a smaller limit/k, or omit full")))
```
Log the RAW error server-side first (D-06): add `slog.ErrorContext(ctx, "connect handler: response too large", "error", err)` before constructing the scrubbed `connect.NewError` — mirroring the `default` arm's own `slog.ErrorContext` + generic-message split (lines 96-98) exactly.

**Error handling pattern (default arm, lines 96-98):**
```go
default:
	slog.ErrorContext(ctx, "connect handler: unexpected error", "error", err)
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
```
This is the "log detail server-side, generic message to wire" precedent D-03/D-04 extend for `ErrResponseTooLarge`.

---

### `internal/server/argerror.go` — new `HintTooLarge` + extracted renderer (utility, transform)

**Analog:** `internal/server/argerror.go` (full file)

**Imports** (lines 6-13): `errors`, `fmt`, `connectrpc.com/connect`, `github.com/seanb4t/engram/internal/store` — unchanged, no new imports needed for the constant; the extraction may not need new imports either.

**Core pattern — HintCode vocabulary (lines 26-37):**
```go
const (
	HintRequired            HintCode = "required"
	HintConditionalRequired HintCode = "conditional_required"
	HintTooLong             HintCode = "too_long"
	HintTooMany             HintCode = "too_many"
	HintEnum                HintCode = "enum"
	HintFormat              HintCode = "format"
	HintPrefix              HintCode = "prefix"
	HintOrdering            HintCode = "ordering"
	HintMutuallyExclusive   HintCode = "mutually_exclusive"
	HintNotApplicable       HintCode = "not_applicable"
)
```
Add `HintTooLarge HintCode = "too_large"` as the 11th constant, same block, same comment style (update the block comment at lines 22-25 — "the vocabulary is a published wire contract" — no change needed to its text, just the list).

**Renderer to extract (lines 85-94, `(*argError).Error()`):**
```go
func (e *argError) Error() string {
	fields := ""
	for i, f := range e.Fields {
		if i > 0 {
			fields += ","
		}
		fields += f
	}
	return "field=" + fields + " hint=" + string(e.Hint) + ": " + e.Detail
}
```
Per RESEARCH.md Pattern 3 (D-04's "ONE renderer" requirement): extract this string-building into a package-level function, e.g.:
```go
func renderHintEnvelope(fields []string, hint HintCode, detail string) string {
	joined := ""
	for i, f := range fields {
		if i > 0 {
			joined += ","
		}
		joined += f
	}
	return "field=" + joined + " hint=" + string(hint) + ": " + detail
}

func (e *argError) Error() string {
	return renderHintEnvelope(e.Fields, e.Hint, e.Detail)
}
```
Both the new Connect arm and the new MCP mapper call `renderHintEnvelope` directly — NEVER construct an `*argError` for this sentinel (its `Unwrap()` returns `store.ErrInvalidArgument` and its `ConnectCode()` only knows 3 classes — Anti-Pattern in RESEARCH.md).

**Precedent for the pseudo-field `"response"`:** `internal/store/revert.go:151-206` (`RevertRefusalError`) uses `field=steps` and `field=record_version` as fixed pseudo-fields not tied to any single tool argument — same rationale applies to `field=response` (the response overflowed, not an input).

---

### `internal/server/<new>.go` — MCP receiving-middleware mapper (middleware, request-response)

**Analog:** `internal/server/instrument.go` (full file, `instrumentTools`)

**Imports pattern** (lines 6-16):
```go
import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)
```
New mapper needs: `context`, `errors`, `log/slog`, `github.com/modelcontextprotocol/go-sdk/mcp`, `github.com/seanb4t/engram/internal/store` (for `errors.Is(err, store.ErrResponseTooLarge)`).

**Core middleware pattern (lines 25-64):**
```go
func instrumentTools(record recordFunc) mcp.Middleware {
	tracer := otel.Tracer("github.com/seanb4t/engram")
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			ctr, ok := req.(*mcp.CallToolRequest)
			if !ok || ctr.Params == nil {
				return next(ctx, method, req)
			}
			// ...
			res, err := next(ctx, method, req)
			// ...
			return res, err
		}
	}
}
```
The new mapper follows the SAME `mcp.Middleware` shape (guard on `method != "tools/call"`, call `next`, inspect result, return). Use the go-sdk mechanics from RESEARCH.md Pattern 4:
```go
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
			slog.ErrorContext(ctx, "mcp tool call: response too large", "err", toolErr)
			ctr.Content = []mcp.Content{&mcp.TextContent{Text: renderHintEnvelope(
				[]string{"response"}, HintTooLarge,
				"the result set is too large for one request; use a smaller limit/k, or omit full")}}
			ctr.IsError = true
			return ctr, err
		}
	}
}
```

**Registration site (`internal/server/tools.go:2262`):**
```go
s.AddReceivingMiddleware(instrumentTools(tm.Record))
```
D-08 recommends editing this SAME call to list the new mapper SECOND (innermost, sees the raw result first):
```go
s.AddReceivingMiddleware(instrumentTools(tm.Record), mapResponseTooLarge())
```
(go-sdk v1.8.0's `addMiddleware` iterates `slices.Backward` — first-listed argument ends up outermost, so listing `mapResponseTooLarge()` second makes it innermost.)

**Error handling / logging note (Pitfall 3):** `instrumentTools`'s own `if err != nil { ...ErrorContext... }` branch (line 54) NEVER fires for this error class — the Go `error` return from `next()` is `nil` for an ordinary typed-handler business error (it lives in `CallToolResult.err`, reachable only via `GetError()`). The new mapper's `slog.ErrorContext` call is the FIRST log line this error class gets — do not write a plan step assuming `instrumentTools` "already logs" this and needs ordering to avoid a double log.

---

### `internal/server/<name>_test.go` — Connect-lane e2e (test, request-response)

**Analog:** `internal/server/tools_test.go:207-215` (`testDepsWithStore`)

```go
func testDepsWithStore(t *testing.T) (*deps, *store.Store) {
	t.Helper()
	c := storetest.Dial(t, storetest.RecvLimit)
	st := newTestStore(t, c, testCollection("mem_eval_test"))
	if err := st.EnsureCollection(context.Background(), 3); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	return &deps{st: st, em: fakeEmbedder{}}, st
}
```
Reuse this helper directly (already builds a real-Qdrant-backed `*deps`); seed via `storetest.SeedOversized` (same as the store-lane test), then drive `d.ListMemories`/equivalent Connect handler and assert `connect.CodeOf(err) == connect.CodeResourceExhausted` and that the error message contains `field=response hint=too_large` and NOT the raw grpc-go text/byte ceiling.

---

### `cmd/engram/client_common.go` — new `exitTooLarge` constant + switch case (config, transform)

**Analog:** `cmd/engram/client_common.go:216-252` (constants) and `:432-446` (`exitCodeForConnectErr`)

**Core pattern (constants block, lines 216-252):**
```go
const (
	exitOK          = 0 // success
	exitGeneric     = 1 // generic/unclassified
	// ...
	exitSetupFailed = 9
)
```
Add immediately after `exitSetupFailed = 9`:
```go
	// exitTooLarge is produced when a Qdrant response exceeded the
	// client's receive limit (store.ErrResponseTooLarge) — D-07. A
	// visible behavior change: CodeResourceExhausted previously fell to
	// exitGeneric (1).
	exitTooLarge = 10
```

**Core pattern (mapper, lines 432-446):**
```go
func exitCodeForConnectErr(err error) int {
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated, connect.CodePermissionDenied:
		return exitAuth
	case connect.CodeNotFound:
		return exitNotFound
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeOutOfRange:
		return exitUsage
	case connect.CodeDeadlineExceeded:
		return exitTimeout
	case connect.CodeUnavailable, connect.CodeCanceled:
		return exitUnavailable
	default:
		return exitGeneric
	}
}
```
Add `case connect.CodeResourceExhausted: return exitTooLarge` — a new case, not folded into any existing one (`CodeResourceExhausted` is a distinct connect.Code(8), not currently listed, currently reached only via `default`).

---

### `cmd/engram/client_common_test.go` — row 46 edit (test, transform)

**Analog:** `cmd/engram/client_common_test.go:31-68` (`TestExitCodeForConnectErrTable`)

```go
cases := []struct {
	code connect.Code
	want int
}{
	// ...
	{connect.CodeResourceExhausted, exitGeneric},   // <- change to exitTooLarge
	// ...
}
if len(cases) != 16 {
	t.Fatalf("test table has %d entries, want 16 (one per connect.Code)", len(cases))
}
```
Change ONLY the `want` value on the `CodeResourceExhausted` row from `exitGeneric` to `exitTooLarge`; the `len(cases) != 16` guard stays (still 16 codes, no new row).

---

### `cmd/engram/catalog.go` — new `doc.ExitCodes` row (config, transform)

**Analog:** `cmd/engram/catalog.go:119-155` (`doc.ExitCodes` literal)

```go
doc.ExitCodes = []catalogExitCode{
	{Code: exitOK, Meaning: "success"},
	{Code: exitGeneric, Meaning: "unclassified internal error (backstop only ...)"},
	{Code: exitUsage, Meaning: "usage or validation error"},
	{Code: exitAuth, Meaning: "authentication or authorization failure"},
	{Code: exitNotFound, Meaning: "not found"},
	{Code: exitUnavailable, Meaning: "transport or server unavailable"},
	{Code: exitTimeout, Meaning: "request deadline exceeded"},
	{Code: exitFindings, Meaning: "findings reported under an explicit opt-in flag ..."},
	{Code: exitPartial, Meaning: "at least one runtime was registered successfully ..."},
	{Code: exitSetupFailed, Meaning: "every runtime engram setup attempted failed ..."},
}
```
Add a new row `{Code: exitTooLarge, Meaning: "a Qdrant response exceeded the client's receive limit; use a smaller limit/k, or omit full"}` — MANDATORY same-commit companion edit to the `client_common.go` change (Pitfall 4: `TestCatalogExitCodesMatchMapper` derives its expected set PROGRAMMATICALLY from `exitCodeForConnectErr`, so this row's absence fails that test immediately, not just doc drift). Then regenerate goldens: `go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -update -count=1`.

---

### `internal/e2e/cli_exitcode_test.go` — new subtest (test, request-response, full binary)

**Analog:** `internal/e2e/cli_exitcode_test.go:44-90` (`TestCLIExitCodes`)

```go
func TestCLIExitCodes(t *testing.T) {
	t.Run("search without --server or ENGRAM_SERVER_URL exits 2, no network call", func(t *testing.T) {
		_, stderr, code := runCLI(t, "search", "--scope", "repo:x", "--query", "q")
		if code != 2 {
			t.Fatalf("exit code = %d, want 2 (usage)\nstderr:\n%s", code, stderr)
		}
	})
	// ...
}
```
Add a new `t.Run(...)` subtest that starts a REAL server against a REAL Qdrant with the oversized fixture (via `startServer`, `internal/e2e/harness_test.go:236`, plus `storetest.SeedOversized`) and asserts `runCLI(t, "list", ...)` (or `search`) returns `code == 10`. Follow the exact `runCLI` helper (lines 18-38) unchanged.

---

### `docs-site/src/content/docs/reference/errors.md` — hint table + class-to-Connect-code (doc, transform)

**Analog:** same file, `## The ten hint codes` (lines 94-110) and `## The class-to-Connect-code mapping` (lines 172-188)

**Hint-code table pattern (lines 99-110):**
```
| Hint code | Meaning | What to do |
|---|---|---|
| `required` | The field was absent entirely. | Supply it — it was missing, not malformed. |
...
| `not_applicable` | ... | ... |
```
Add a new row: `| \`too_large\` | The response itself exceeded the client's receive limit — not a rejected input. | Retry with a smaller \`limit\`/\`k\`, or omit \`full\`. |`. Rename the heading `## The ten hint codes` → `## The eleven hint codes` (line 94) and update the transcription-count claim in the body text (line 96-97) to match.

**Class-to-Connect-code mapping pattern (lines 172-188):** `too_large`/`resource_exhausted` is explicitly NOT one of the three `argError` classes (Malformed/OutOfRange/Precondition) — per Pitfall 5, do NOT add it as a 4th row to that 3-row table (which states "all three map to exitUsage"). Instead add a NEW, separate paragraph/table row directly below, analogous in shape to the `## The one exit code with no hint-code or Connect-code counterpart` section (lines 190-203) — e.g. a new subsection stating `resource_exhausted` → exit `10`, distinct from the 3-class trio, with its own one-row table.

---

### `docs-site/src/content/docs/guides/cli.md` — exit-code table row (doc, transform)

**Analog:** same file, `## Exit codes` (lines 353-370)

```
| Code | Meaning |
|------|---------|
| 0 | Success (including an empty result set) |
...
| 9 | All attempted setup runtimes failed |
```
Add `| 10 | A Qdrant response exceeded the client's receive limit; retry with a smaller limit/k, or omit full |` as a new row.

---

### `docs-site/src/content/docs/guides/upgrade.md` — new numbered entry (doc, transform)

**Analog:** same file, entry `### 4. New exit code 6 for a request timeout` (lines 126-133)

```
### 4. New exit code 6 for a request timeout

A client-side request deadline being exceeded now reports **exit code
`6`**, distinct from exit `5` (transport or server unavailable). Code `5`
means "I could not reach the server or it refused the connection"; code `6`
means "the server accepted the request but did not answer before the
deadline." A caller currently treating `5` as "retry later" should decide
whether a timeout warrants raising `--timeout` instead of a bare retry.
```
Add a new numbered entry (append to the list, do not renumber existing entries) in this EXACT shape: state the OLD behavior (`CodeResourceExhausted` → `exitGeneric` (1)) and the NEW behavior (`exitTooLarge` (10)), and who should act (any script currently checking `exit 1` generically for this case).

---

### `internal/store/redevidence_harness_test.go` — new `redEvidenceDirs` entry (test/config, transform) — LAST PLAN ONLY

**Analog:** `internal/store/redevidence_harness_test.go:106-113` (Phase 1's own entry)

```go
var redEvidenceDirs = map[string]map[string]string{
	".planning/phases/01-test-harness-fixture-helper/red-evidence": {
		"01-01-storetest-raw-client-write.patch":       "TestQdrantClientIsHeldOnlyByStorePackage",
		"01-04-listscopes-full-payload-selector.patch": "TestListScopesFullPayloadsOverGRPCLimit",
		"01-05-bare-qdrant-newclient-in-test.patch":    "TestQdrantClientConstructedOnlyByNewQdrantClient",
		"01-05-ci-qdrant-image-drift.patch":            "TestQdrantImageMatchesCIService",
	},
}
```
Add ONE new top-level key `.planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence` mapping this phase's own patch filenames to their target test function names — same shape, added at the very end of the plan sequence (Pitfall 7: this gate is GREEN today; do not "fix" it, just extend the map).

## Shared Patterns

### Field/Hint Envelope Rendering
**Source:** `internal/server/argerror.go:85-94` (`(*argError).Error()`) + `internal/store/revert.go:151-206` (`RevertRefusalError`, the non-`argError` pseudo-field precedent)
**Apply to:** `internal/server/connecterror.go` (new arm), the new MCP middleware file — BOTH must call the SAME extracted `renderHintEnvelope(fields []string, hint HintCode, detail string) string` helper (D-04's "ONE renderer" requirement), never hand-build a second `"field=..." + "hint=..."` string.
```go
func renderHintEnvelope(fields []string, hint HintCode, detail string) string {
	joined := ""
	for i, f := range fields {
		if i > 0 {
			joined += ","
		}
		joined += f
	}
	return "field=" + joined + " hint=" + string(hint) + ": " + detail
}
```

### gRPC status classification idiom
**Source:** `internal/store/store.go:614` (`status.FromError(err); ok && st.Code() == grpccodes.AlreadyExists`)
**Apply to:** the new classifier interceptor in `internal/store/store.go` — same `ok &&` two-value guard, never a bare `err.(*status.Status)` assertion.

### Server-side-log, generic-wire-message split
**Source:** `internal/server/connecterror.go:96-98` (`default` arm: `slog.ErrorContext` + generic `connect.NewError(connect.CodeInternal, errors.New("internal error"))`)
**Apply to:** the new Connect arm (log raw `err` with `slog.ErrorContext`, then construct the scrubbed envelope) and the new MCP middleware (log raw `toolErr`, then rewrite `Content`) — D-03's "byte counts/raw text SERVER-SIDE LOGS ONLY" requirement is this exact split, already shipped once.

### Real-overflow test fixture
**Source:** `internal/store/storetest/storetest.go:43` (`RecvLimit`), `internal/store/storetest/seed.go:179-230` (`SeedOversized`), `internal/store/listscopes_oversized_test.go` (full usage example)
**Apply to:** every new regression test across all four lanes (store, Connect, MCP, CLI) that needs a REAL overflow — always `storetest.Dial(t, storetest.RecvLimit)` + `storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: storetest.FewLarge, Vector: [...]})`, never a hand-rolled byte ceiling (rule `m45p2b4bp7`).

### Mechanically-gated exit-code taxonomy
**Source:** `cmd/engram/client_common.go:216-252,432-446` (constants + `exitCodeForConnectErr`), `cmd/engram/catalog.go:119-155` (`doc.ExitCodes`), `cmd/engram/catalog_test.go:344-391` (`TestCatalogExitCodesMatchMapper`)
**Apply to:** the `exitTooLarge` addition — constant, mapper case, catalog row, and `client_common_test.go` row 46 are ONE atomic edit; `TestCatalogExitCodesMatchMapper` will fail immediately (not just doc-drift) if the catalog row is missing.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/store/<name>_classifier_test.go` | test | transform | No existing test in this repo drives a classifier purely against synthetic `status.Error` values (every existing store test hits real Qdrant). Build from RESEARCH.md's own Pattern 1 code block; use `cmd/engram/client_common_test.go:31-68`'s table-driven `t.Run` shape as the general Go-test-style analog. |

## Metadata

**Analog search scope:** `internal/store/`, `internal/store/storetest/`, `internal/server/`, `cmd/engram/`, `internal/e2e/`, `internal/surfaces/`, `docs-site/src/content/docs/reference/`, `docs-site/src/content/docs/guides/`
**Files scanned:** `store.go`, `revert.go`, `revert_test.go`, `connecterror.go`, `argerror.go`, `instrument.go`, `tools.go`, `tools_test.go`, `client_common.go`, `client_common_test.go`, `catalog.go`, `catalog_test.go`, `listscopes_oversized_test.go`, `storetest.go`, `seed.go`, `cli_exitcode_test.go`, `harness_test.go`, `redevidence_harness_test.go`, `internal/surfaces/rules.go`, `errors.md`, `cli.md`, `upgrade.md`
**Pattern extraction date:** 2026-09-19
**Tracked-source gate:** verified via `git ls-files --error-unmatch` — all 18 analog paths above are git-tracked source, none is a gitignored capability mirror.
