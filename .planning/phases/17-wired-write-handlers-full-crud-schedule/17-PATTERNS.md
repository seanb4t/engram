# Phase 17: Wired Write Handlers (Full CRUD + Schedule) - Pattern Map

**Mapped:** 2026-07-12
**Files analyzed:** 13 (6 modified, 7 new)
**Analogs found:** 13 / 13 — every new/modified file has an in-repo analog; nothing falls to "no analog"

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/server/tools.go` (`deps.*` sig change, D-01/D-02/D-07) | service (business logic) | CRUD + request-response | itself, pre-refactor (`storeMemory`/`updateMemory`/`deleteMemory`/`setVisibility` current bodies) | exact — same file, signature-only change |
| `internal/server/identity.go` (`callerFromTokenInfo`, D-02) | utility (identity seam) | transform | `SubjectFromTokenInfo` (identity.go:21-29) | exact |
| `internal/server/connectapi.go` (6 new write RPC methods + 5 read RPCs rewired, D-07) | controller (Connect handler) | request-response | `GetMemory` (connectapi.go:183-213) | exact — explicit template per D-11 |
| `internal/server/protoconv.go` (NEW, D-09) | utility (proto↔args adapter) | transform | `memoryToProto` (connectapi.go:33-53) + `parseRFC3339`/`parseWindow` (connectapi.go:63-69, tools.go:445-473) | role-match |
| `internal/server/protoconv_test.go` (NEW, D-09) | test | transform | `connectapi_negative_test.go` (table-driven exact-code style) | role-match |
| `internal/server/store_iface.go` (NEW, D-10) | model/interface (store seam) | CRUD | `deps.st *store.Store` field decl (tools.go:35) + `store.Store` method set (store.go) | role-match — pure interface carve, no behavior analog needed |
| `internal/server/fakestore_test.go` (NEW, D-10) | test (fake/double) | CRUD | `testDeps(t)` (tools_test.go:192-214, real-Qdrant double) | role-match — same *purpose*, different mechanism (hermetic vs live) |
| `internal/server/connectapi_write_parity_test.go` (NEW, D-10) | test (parity) | event-driven/comparison | `TestRerankParityMCPAndConnect` (connectapi_test.go:356-432) | exact — explicitly named template |
| `internal/server/connectapi_negative_test.go` (MODIFIED — fix `d := &deps{}` nil-store panic, Pitfall 3) | test | request-response | itself, current `TestWriteRPCNegativeMatrix` (connectapi_negative_test.go:64) | exact |
| `internal/auth/auth.go` (`ClaimIdentity` → ordered list, D-04/D-05/D-06) | service (pure resolver) | transform | itself, current `ClaimIdentity` (auth.go:83-97) | exact — same function, signature change |
| `internal/auth/auth_test.go` (MODIFIED, Pitfall 5) | test | transform | itself, current `TestClaimIdentity` (auth_test.go:128-156) | exact |
| `internal/config/registry.go` (`ENGRAM_OWNER_CLAIM` comma-list parse, D-04) | config | transform | existing `ENGRAM_OWNER_CLAIM` registration (registry.go:52) | exact |
| `internal/webauth/resolver.go` (Actor fallback, Pitfall 2) | middleware/service (session resolver) | request-response | itself, current `Resolver.Resolve` (resolver.go:36-55) | exact |

## Pattern Assignments

### `internal/server/tools.go` — `deps.*` signature refactor (D-01/D-02/D-07)

**Analog:** the same file's current bodies (no external analog needed — this is a mechanical signature change applied uniformly).

**Current shape to replace** (tools.go:634-655, `storeMemory`):
```go
func (d *deps) storeMemory(ctx context.Context, a storeArgs) (string, string, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	m := a.toMemory(subj.Owner(), actorFromContext(ctx), d.clock())
	m.EmbedderIdentity = d.embedderIdentity
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	d.summaryQueue.tryEnqueue(m.ID)
	return m.ID, m.ShortID, nil
}
```
**Target shape (D-01):** `func (d *deps) storeMemory(ctx context.Context, c caller, a storeArgs) (string, string, error)` — replace `subj, err := subjectFromContext(ctx)` with reading `c.Subj` directly (no error path needed since `caller` construction, not the method, owns identity failure), and replace `actorFromContext(ctx)` with `c.Actor`. Apply this exact substitution to all six write methods (634, 667, 700, 918, 1010, 1030) AND all five read methods (`listMemory` 793, `listScheduled` 823, `searchMemory` 854, `searchDiscovery` 894, `getMemory` 987) AND `storeRule`/`listRules` (rules.go — same seam, not in the six but must not become an un-migrated second pattern per RESEARCH.md).

**The by-id re-wrap pattern to preserve verbatim** (tools.go:1030-1064, `setVisibility` — copy this shape for `updateMemory`/`deleteMemory`/`setVisibility`, only the identity line changes):
```go
func (d *deps) setVisibility(ctx context.Context, a setVisibilityArgs) error {
	subj, err := subjectFromContext(ctx) // becomes: read c.Subj, no err path
	if err != nil {
		return err
	}
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return err
	}
	rec, err := d.st.GetReadable(ctx, pid, subj)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: %s", store.ErrNotFound, a.ID) // ORIGINAL input, not pid
		}
		return err
	}
	if rec.Category == "rule" {
		return fmt.Errorf("rules are always shared — delete the rule instead of changing its visibility")
	}
	if err := d.st.SetVisibility(ctx, pid, subj, a.Shared); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return err
	}
	return nil
}
```
**Rule guard + stale-summary pattern to preserve in `updateMemory`** (tools.go:918-981) — this is the one method with real business logic beyond identity+re-wrap; copy the rule-category check (944-953) and `resolveSummaryUpdate` call (957) unchanged, only swap `subj`/`actor` sourcing.

**D-09 landmine fix (Pitfall 1) — `updateArgs.Content` must become `*string`:** current decl (tools.go:507-513):
```go
type updateArgs struct {
	ID      string    `json:"id" jsonschema:"the memory's full UUID or its short_id"`
	Content string    `json:"content"`
	Shared  *bool     `json:"shared,omitempty" jsonschema:"omit to keep current visibility; true=shared, false=private"`
	Tags    *[]string `json:"tags,omitempty" jsonschema:"omit to keep current tags; supply to replace the full set (empty array clears)"`
	Summary *string   `json:"summary,omitempty" jsonschema:"omit to keep current summary; ..."`
}
```
Follow the already-established `Shared`/`Tags`/`Summary` `*T`-optional convention: change `Content string` → `Content *string`, and in `updateMemory`, `nil` means "keep `cur.Content`" (mirrors how `a.Tags != nil` is handled at tools.go:969-971: `tags := cur.Tags; if a.Tags != nil { tags = *a.Tags }`). The MCP tool-registration call site keeps passing a non-nil pointer (unconditional-replace semantics unchanged); protoconv passes `nil` when `"content"` ∉ `update_mask.paths`.

---

### `internal/server/identity.go` — `callerFromTokenInfo` (D-02)

**Analog:** `SubjectFromTokenInfo` (identity.go:15-29), the exact function this new seam extends.

**Full current file** (14-56, reproduced above in research) — the pattern to extend:
```go
func SubjectFromTokenInfo(ti *mcpauth.TokenInfo) (store.Subject, error) {
	if ti == nil {
		return store.Anonymous(), nil
	}
	if v, ok := ti.Extra[auth.OwnerClaimExtraKey].(string); ok && v != "" {
		return store.Authenticated(v), nil
	}
	return nil, fmt.Errorf("validated token missing owner claim")
}
```
**Target `callerFromTokenInfo` shape:** wraps/subsumes this — resolve `Subj` exactly as above, then resolve `Actor` with the **Pitfall 2 fallback**: `actor := ti.UserID; if actor == "" { actor = <resolved owner value> }` (never assume `UserID` is populated — the Connect cookie lane's `TokenInfo` never sets it; see `webauth/resolver.go` below). Both `subjectFromContext` (789-791) and `subjectFromConnectContext` (48-56) become thin callers of this one function instead of parallel `SubjectFromTokenInfo` calls.

---

### `internal/server/connectapi.go` — six new write handlers (D-07/D-11)

**Analog:** `GetMemory` (connectapi.go:183-213) — the explicit D-11 template, both for the thin-adapter shape and the not-found re-wrap.

```go
func (a *engramAPI) GetMemory(ctx context.Context, req *connect.Request[engramv1.GetMemoryRequest]) (*connect.Response[engramv1.GetMemoryResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	pid, err := a.d.st.ResolvePointID(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, store.ErrInvalidArgument) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	m, err := a.d.st.GetReadable(ctx, pid, subj)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Re-wrap with the caller's ORIGINAL input so a resolved short id
			// never leaks another owner's real UUID into the error message.
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%w: %s", store.ErrNotFound, req.Msg.Id))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	a.d.usageQueue.tryEnqueue(pid)
	return connect.NewResponse(&engramv1.GetMemoryResponse{Memory: memoryToProto(m)}), nil
}
```
**Post-D-07 target shape (per RESEARCH.md §"Connect Write-Handler Wiring"):** this collapses to `a.d.getMemory(ctx, callerFromConnectContext(ctx), idArgs{ID: req.Msg.Id})` — the re-wrap logic already lives inside `deps.getMemory` and must NOT be duplicated in the handler. **Apply the identical collapse to the six new write handlers:** resolve caller once (`callerFromConnectContext(ctx)`, new helper alongside `subjectFromConnectContext`), convert `req.Msg` → args via `protoconv`, call the one `deps.*` method, convert result → proto response, map only `errors.Is(err, store.ErrNotFound)` → `connect.CodeNotFound` (re-wrap text is already correct coming out of `deps.*`).

**Anti-pattern to avoid (DEC-cgb):** do NOT re-check ownership/authz in the handler — `GetMemory`'s only authz-adjacent code is the identity resolution at the top; everything else is store-layer-enforced. The six new handlers must have the same shape: identity resolve → protoconv → one `deps.*` call → response/error map. No `if subj.Owner() != rec.Owner` anywhere in `connectapi.go`.

---

### `internal/server/protoconv.go` (NEW, D-09)

**Analog (Timestamp/RFC3339 convention):** `memoryToProto` (connectapi.go:33-53) already uses `timestamppb.New(t)`; `parseRFC3339` (connectapi.go:63-69) is the MCP-lane's inverse idiom. Do not cross the streams — protoconv converts `*timestamppb.Timestamp` → RFC3339 string via `.AsTime().Format(time.RFC3339)` to feed the existing, well-tested `parseWindow` (tools.go:445-473) unchanged (per RESEARCH.md A2, recommended over extending `parseWindow` to accept `*time.Time`).

**Citation mapping — zero semantic gap, pure struct literal:**
```go
// citationArg (tools.go:519-525) — field-for-field identical to proto Citation
type citationArg struct {
	Kind    string `json:"kind" jsonschema:"file|commit|url|repo"`
	Ref     string `json:"ref" jsonschema:"path, repo URL, or doc URL"`
	Locator string `json:"locator,omitempty" jsonschema:"e.g. 200-240 line range"`
	Pin     string `json:"pin,omitempty" jsonschema:"commit SHA, content-hash, @rev, or fetched-at"`
	Excerpt string `json:"excerpt,omitempty" jsonschema:"cached substance (<= ~50 lines)"`
}
```
**Visibility↔shared:** `VISIBILITY_SHARED ⇔ true`, everything else ⇔ `false`; the proto's zero value (`VISIBILITY_UNSPECIFIED`) is already validation-rejected upstream by `buf.validate` before the handler runs, so the adapter only ever sees `PRIVATE`/`SHARED` — no zero-value branch needed.

**FieldMask → updateArgs pointer fields:** the mask is already CEL-validated (non-empty, allowlist `{content,shared,tags,summary}`) before the handler runs — protoconv's job is purely "is `<field>` in `mask.Paths`? if yes, populate the pointer from `req.Msg.<Field>`; if no, leave `nil`." No re-validation.

---

### `internal/server/store_iface.go` (NEW, D-10)

**Analog:** the field declaration `st *store.Store` (tools.go:35) plus the enumerated call-site surface (RESEARCH.md tables) — this is a pure interface carve, not a pattern to copy from elsewhere in the repo (no existing narrow-interface-over-a-concrete-store precedent exists here per RESEARCH.md's "zero behavior change" framing).

**Required surface (verified against store.go:463-1508, exact signatures)** — copy method signatures as-is from `*store.Store`, do not paraphrase:
```go
type memStore interface {
	MintShortID(ctx context.Context, seen map[string]struct{}) (string, error)
	Upsert(ctx context.Context, m store.Memory, vec []float32) error
	ResolvePointID(ctx context.Context, idOrShort string) (string, error)
	OwnedOrAbsent(ctx context.Context, id string, subj store.Subject) error
	Get(ctx context.Context, id string) (store.Memory, error)
	FetchForUpdate(ctx context.Context, id string, subj store.Subject) (store.Memory, error)
	Update(ctx context.Context, cur store.Memory, content string, shared *bool, tags *[]string, summary *string, vec []float32) error
	Delete(ctx context.Context, id string, subj store.Subject) error
	GetReadable(ctx context.Context, id string, subj store.Subject) (store.Memory, error)
	SetVisibility(ctx context.Context, id string, subj store.Subject, shared bool) error
	List(ctx context.Context, scope string, subj store.Subject, opts store.ListOptions) ([]store.Memory, uint64, string, error)
	ListScheduled(ctx context.Context, scope string, subj store.Subject, state store.ScheduledState, opts store.ListOptions) ([]store.Memory, error)
	SearchReranked(ctx context.Context, scope string, subj store.Subject, query string, vec []float32, k uint64, tags []string, after, before time.Time) ([]store.Memory, error)
	SearchDiscovery(ctx context.Context, scope, kind string, subj store.Subject, vec []float32, k uint64) ([]store.Memory, error)
}
```
Note: leave `store.Update`'s `content string` positional param as-is at the store layer (Pitfall 1's fix is in `updateArgs`/`deps.updateMemory`, not in `store.Update`'s signature).

---

### `internal/server/fakestore_test.go` (NEW, D-10)

**Analog:** `testDeps(t)` (tools_test.go:192-214) — same *purpose* (a `deps` test double), different mechanism. `testDeps` requires live Qdrant (`t.Skip` if `testQdrantAddr == ""`); the new fake implements `memStore` in-memory (map-backed), hermetic, no skip condition. Read `tools_test.go:192-214` for the `deps{}` struct-literal wiring pattern (which fields are set) to replicate when constructing `&deps{st: fakeStore{...}, em: ..., ...}`.

---

### `internal/server/connectapi_write_parity_test.go` (NEW, D-10) — clone `TestRerankParityMCPAndConnect`

**Analog:** `TestRerankParityMCPAndConnect` (connectapi_test.go:356-432) — direct Go-level calls into both `deps.*` and `engramAPI.*`, no HTTP round-trip.

```go
// Source: internal/server/connectapi_test.go:356-432
func TestRerankParityMCPAndConnect(t *testing.T) {
	d := testDeps(t)
	api := &engramAPI{d: d}
	// ... seed records via d.st.Upsert ...

	mcpCtx := authedContext(t, "actor-A")
	actx := withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-A"}})

	mcpIDs := func(a searchArgs) []string {
		t.Helper()
		out, err := d.searchMemory(mcpCtx, a)
		if err != nil {
			t.Fatalf("MCP searchMemory: %v", err)
		}
		ids := make([]string, len(out))
		for i, v := range out {
			view, ok := v.(recallView)
			if !ok {
				t.Fatalf("MCP result %d unexpected type %T (want recallView)", i, v)
			}
			ids[i] = view.ID
		}
		return ids
	}
	connectIDs := func(req *engramv1.SearchMemoriesRequest) []string {
		t.Helper()
		resp, err := api.SearchMemories(actx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("Connect SearchMemories: %v", err)
		}
		ids := make([]string, len(resp.Msg.Memories))
		for i, m := range resp.Msg.Memories {
			ids[i] = m.Id
		}
		return ids
	}
	// ... assert mcpIDs(...) == connectIDs(...) for each scenario ...
}
```
**D-10's analogous pair (per RESEARCH.md Code Examples):**
```go
mcpErr := d.updateMemory(mcpCtx, caller{...}, updateArgs{...})
connectErr := api.UpdateMemory(actx, connect.NewRequest(&engramv1.UpdateMemoryRequest{...}))
// assert connect.CodeOf(mapErrToConnectCode(mcpErr)) == connect.CodeOf(connectErr)
```
Drive the shared scenario table (rule un-share attempt, stale-summary conflict DEC-ddiw, cross-owner id DEC-xa6, **plus the Pitfall 1 tags-only-content-preserved case and a Pitfall 2 `Actor` equality assertion** — RESEARCH.md flags both as easy to omit) through the **same** `deps.*` call from both closures, using the **fake store** (fakestore_test.go) instead of `testDeps(t)`'s live Qdrant, per D-10.

---

### `internal/server/connectapi_negative_test.go` — fix nil-store panic (Pitfall 3)

**Analog:** itself, current construction (connectapi_negative_test.go:64):
```go
d := &deps{} // st == nil today; safe only because Unimplemented never touches d.st
```
Once the six write bodies are real, this must be updated to inject either `testDeps(t)` or the D-10 fake (`fakestore_test.go`) — sequence this fix WITH or BEFORE the handler-wiring task, not after (Pitfall 3's exact CI-break warning).

---

### `internal/auth/auth.go` — `ClaimIdentity` ordered-list (D-04/D-05/D-06)

**Analog:** itself, current signature (auth.go:83-97):
```go
func ClaimIdentity(raw map[string]any, ownerClaim string) (owner, email, username string, err error) {
	email, _ = raw["email"].(string)
	username, _ = raw["preferred_username"].(string)
	owner, _ = raw[ownerClaim].(string)
	if ownerClaim == "email" {
		if verified, _ := raw["email_verified"].(bool); !verified {
			return "", "", "", fmt.Errorf("email not verified")
		}
	}
	return owner, email, username, nil
}
```
**Target signature:** `ClaimIdentity(raw map[string]any, ownerClaims []string) (owner, email, username string, err error)` — iterate `ownerClaims` in order; first non-empty `raw[claim]` wins **as the candidate**, but the `email_verified` gate (D-05) must fire **whenever `email` is the claim that won**, and an unverified-but-present email must reject outright (never fall through to the next claim in the list — that is the D-05 invariant). D-06 namespacing (`sub:<value>` etc.) happens at the CALLER of `ClaimIdentity`/`Authenticated()`, not inside this function or inside `store/subject.go` (confirmed: `store.Authenticated(sub string)` at subject.go:43-48 does no transformation, just wraps).

**Call sites requiring atomic update (Pitfall 5) — all three in the same commit:**
1. `auth.go:134` (`TokenVerifier`, shown above) — `ClaimIdentity(raw, v.ownerClaim)` → `ClaimIdentity(raw, v.ownerClaims)`; `v.ownerClaim string` field (auth.go:52) → `ownerClaims []string`; `New(ctx, issuer, audience, ownerClaim string)` (auth.go:61) → accepts the list (or a comma-string split internally).
2. `webauth/oidc.go:78` (`NewAuthenticator`/`Callback`) — same call, same signature change.
3. `auth_test.go:128-156` (`TestClaimIdentity`) — both `ClaimIdentity(raw, "email")` and `ClaimIdentity(raw, "preferred_username")` calls become `ClaimIdentity(raw, []string{"email"})` / `ClaimIdentity(raw, []string{"preferred_username"})`.

---

### `internal/config/registry.go` — `ENGRAM_OWNER_CLAIM` comma-list (D-04)

**Analog:** the existing singular registration (registry.go:52, `Default: "email"`) — RESEARCH.md's recommendation (A3/Open Question 3): reuse the singular key as a comma-separated list at the config-load boundary (split+trim immediately after koanf resolves the string), zero new config surface. No other `registry.go` entry uses a list-shaped env var — this is a fresh convention, not a copy of an existing list-parsing pattern in this repo.

---

### `internal/webauth/resolver.go` — Actor fallback (Pitfall 2)

**Analog:** itself, current `Resolver.Resolve` (resolver.go:36-55) — confirmed it builds `&mcpauth.TokenInfo{Extra: map[string]any{auth.OwnerClaimExtraKey: sess.Owner}}` with **no `UserID` set**, because `webauth.Session` (session.go:26-29) carries only `Owner`/`Expiry`. This is the concrete reason `callerFromTokenInfo` (identity.go, above) must fall back `Actor` to the resolved owner value rather than trusting `ti.UserID` blindly — do not attempt to extend the cookie schema (`SessionCodec`'s sealed payload) in this phase; that's flagged out-of-scope/invasive by RESEARCH.md.

## Shared Patterns

### Authz stays in the store layer (DEC-cgb)
**Source:** every `deps.*` method above (`d.st.GetReadable`/`FetchForUpdate`/`Delete`/`SetVisibility` gates)
**Apply to:** all six new Connect write handlers and all five rewired read handlers — the handler's only job is proto↔args conversion + error-code mapping; never re-check ownership.

### Not-found re-wrap with original input (DEC-xa6 / D-11)
**Source:** `tools.go:934-936` (`updateMemory`), `1020-1021` (`deleteMemory`), `1050-1051`/`1059-1060` (`setVisibility`), `connectapi.go:203-206` (`GetMemory`)
```go
if errors.Is(err, store.ErrNotFound) {
	return fmt.Errorf("%w: %s", store.ErrNotFound, a.ID) // caller's ORIGINAL input, never pid
}
```
**Apply to:** the three new by-id write methods already do this inside `deps.*` (no new production code in the Connect handlers themselves — see RESEARCH.md's explicit note that D-11's footprint is test-only once D-01/D-07 land).

### `*T`-optional presence-signaled update fields
**Source:** `updateArgs.Shared *bool` / `Tags *[]string` / `Summary *string` (tools.go:510-512) and their handling at tools.go:969-971 (`tags := cur.Tags; if a.Tags != nil { tags = *a.Tags }`)
**Apply to:** `updateArgs.Content` (currently plain `string`) must join this convention — the D-09/Pitfall-1 fix.

### Interceptor-resolved identity, before-handler-runs (Phase 16 D-02)
**Source:** `connectauth.go:18` (`newConnectSubjectInterceptor`), `mountConnect` interceptor chain (connectapi.go:241, 262-267)
**Apply to:** the caller resolution point for all Connect handlers — by handler time, identity is already resolved; handlers only read it (`callerFromConnectContext`), never re-derive it.

## No Analog Found

None — every file in scope maps to an existing in-repo analog (see table above). `store_iface.go` and `fakestore_test.go` are net-new *artifacts* but their pattern (interface-carve over a concrete struct; a hermetic test double standing in for `testDeps`) is directly modeled on cited existing code, not invented from RESEARCH.md's external examples.

## Metadata

**Analog search scope:** `internal/server/{tools,identity,connectapi,connectapi_test,connectapi_negative_test,tools_test}.go`, `internal/auth/{auth,auth_test}.go`, `internal/webauth/{resolver,session}.go`, `internal/store/{subject,store}.go`, `internal/config/registry.go`
**Files scanned:** 13 read directly (targeted offset/limit reads, no full-file loads except `identity.go` at 57 lines)
**Pattern extraction date:** 2026-07-12
