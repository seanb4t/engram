# Phase 3: Curation RPCs & MCP Tools - Pattern Map

**Mapped:** 2026-09-26
**Files analyzed:** 17
**Analogs found:** 17 / 17 (every file extends an existing tracked file; no phase in this milestone creates a brand-new source file)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `proto/engram/v1/engram.proto` | config/schema | request-response | itself (additive: mirror `UpdateMemoryRequest`/`StoreMemoryRequest`/`ListMemoriesRequest` shapes) | exact |
| `internal/server/connectapi.go` | controller | request-response | `UpdateMemory` handler (lines 486-496) for the 3 write RPCs; `ListMemories`/`SearchMemories` (lines 244-388) for the 4 read RPCs | exact |
| `internal/server/connectcsrf.go` | middleware | request-response | `csrfWriteProcedures` map (lines 33-40) | exact |
| `internal/server/protoconv.go` | utility | transform | `updateMemoryRequestToArgs`/`storeMemoryRequestToArgs` + `mutationResultToUpdateMemoryResponse` (lines 44-74, 178-187) | exact |
| `internal/server/tools.go` (new batch core `deps.archiveMemory`/`deps.restoreMemory`) | service | batch | `resolveAndAuthorizeSupersedeTargets` (lines 2415-2532) for id-resolution-loop shape; `store.ArchiveResult`/`ArchiveOutcome` (store/spine.go 709-747) for the outcome vocabulary | role-match (no batch primitive exists; this is the phase's one genuinely new piece — Pattern 2 below) |
| `internal/server/tools.go` (new `deps.relatedMemories`, `deps.listTagsCore` wrappers) | service | CRUD (read) | `deps.searchDiscovery` (lines 2111-2151) and `deps.listScheduled` (lines 1890-1923) — thin validate-then-delegate-to-store shape | exact |
| `internal/server/tools.go` (MCP registrations: `archive_memory`, `restore_memory`, `related_memories`, `list_tags`) | route (MCP tool registration) | request-response | `supersede_memory` / `list_rules` / `list_scheduled` `mcp.AddTool` blocks (lines 2952-2960, 3042-3078) | exact |
| `internal/surfaces/toolclass.go` | config | — | existing `Operation` rows for `supersede_memory`/`list_rules`/`spine-review archive`/`spine-review restore` (lines 138-157, 384-395) | exact |
| `internal/server/connectcsrf_test.go` (extend `csrfWriteCases`, `TestCSRFWriteProcedureAllowlist`) | test | request-response | existing 6-entry `csrfWriteCases` table (lines 107-161) | exact |
| `internal/server/connectcsrf_lane_test.go` | test | request-response | `TestBearerLaneExemptFromCSRF` / `TestCSRFCookieLaneStillEnforcesDoubleSubmit` (lines 38-63, 235-257) | exact |
| `internal/server/connectapi_write_parity_test.go` | test | request-response | `TestWriteParity`'s `t.Run("StoreMemory", ...)` subtest (lines 172-219) | exact |
| `internal/server/connectapi_parity_test.go` | test | request-response | the read-lane parity fixture helpers (`newParityLane`, `parityMCPCaller`, `parityConnectCtx`, `assertCodeParity`, lines 43-101) | exact |
| `internal/server/connectdescriptor_test.go` | test | request-response | `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` (whole file) — D-26 requires deleting the `wantReqResp` map and the per-name loop, keeping only the count assertion and the `IDEMPOTENCY_UNKNOWN` loop | exact (structural edit, not extension) |
| `skill/engram/skills/curating-memory/SKILL.md` | doc | — | existing verb-discrimination tables (e.g. `supersede_memory` vs `delete_memory` at lines 331-340; the MCP tool-name list at lines 503-504) | exact |
| `skill/engram/skills/curating-spine/SKILL.md` | doc | — | itself (consent-gate wording to keep consistent with archive/restore framing) | exact |
| `CLAUDE.md` §Memory contract | doc | — | itself (the existing tool-list sentence naming `store_memory`/`schedule_memory`/.../`delete_all`) | exact |
| `docs-site/src/content/docs/reference/tools.md`, `reference/memory-record.md` | doc | — | itself (existing per-tool sections) | exact |
| `.claude/skills/engram-connect-client/SKILL.md` | doc | — | itself (existing per-RPC `engram`/`engramWrite` table) | exact |

## Pattern Assignments

### `proto/engram/v1/engram.proto` (config/schema, request-response)

**Analog:** the file itself — every existing write RPC message and the `RecallGateHidden` shared-message precedent.

**Additive-message pattern to copy** (`UpdateMemoryRequest`/`Response`, lines 330-346):
```protobuf
message UpdateMemoryRequest {
  string id = 1 [(buf.validate.field).string.min_len = 1];
  string content = 2;
  ...
}
message UpdateMemoryResponse {
  string id = 1;
  string short_id = 2;
}
```
New RPCs (`SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`, `ListRules`, `ListScheduled`, `RelatedMemories`, `ListTags`) each get a `Request`/`Response` message pair appended after `ScheduleMemoryResponse` (line 400) and a new `rpc` line appended inside `service EngramService` (lines 402-416) — never inserted between existing lines, so field/method numbering stays append-only (`buf breaking` green).

**Shared-message-reuse pattern** (`RecallGateHidden`, lines 153-166): one message, reused verbatim across `ListMemoriesResponse`/`SearchMemoriesResponse` with independent field-number sequences per host message. Copy this shape for `ArchiveResult`/`RestoreResult`'s per-id outcome message (D-06) and for `RelatedEdge`'s shared oneof (D-12) — one message definition, referenced by multiple RPC response messages, never duplicated.

**oneof pattern for `RelatedEdge` (D-12)** — the store's flat, union-by-omission struct that must NOT be copied 1:1 into proto (`internal/store/relatedmemories.go:87-105`, quoted verbatim):
```go
type RelatedEdge struct {
	Type RelatedEdgeType `json:"type"`
	Score float32 `json:"score,omitempty"`
	SharedTags []WeightedTag `json:"shared_tags,omitempty"`
	TagWeight  float64       `json:"tag_weight,omitempty"`
	SharedCitations []CitationRef `json:"shared_citations,omitempty"`
	Direction SupersessionDirection `json:"direction,omitempty"`
	Depth     int                   `json:"depth,omitempty"`
}
```
Design a proto `EdgeType` enum (mirroring the four `RelatedEdgeType` consts at `internal/store/relatedmemories.go:39-52`) plus `oneof evidence { VectorEvidence vector = N; TagEvidence tag = N; CitationEvidence citation = N; SupersessionEvidence supersession = N; }` — never a flat message with all fields optional. See `Visibility` enum (lines 264-268) for the sibling small-enum pattern already in this file.

**Validation annotation pattern** (`StoreMemoryRequest`, lines 270-281; `ScheduleMemoryRequest`'s message-level CEL, lines 369-374): `[(buf.validate.field).string.min_len = 1]` for required strings, `option (buf.validate.message).cel = {...}` for relational rules (e.g. `ArchiveMemoryRequest.ids` non-empty + per-entry non-blank, mirroring the CEL shape already used for `update_memory.mask`).

---

### `internal/server/connectapi.go` (controller, request-response)

**Analog:** `UpdateMemory` (lines 486-496) for the three new write RPCs; `ListMemories`/`SearchMemories` (lines 244-388) for the four new read RPCs.

**Imports** (lines 6-26): no new imports needed — `connect`, `engramv1`, `store`, `surfaces` are already imported.

**Thin-adapter write pattern to copy verbatim** (lines 486-496):
```go
func (a *engramAPI) UpdateMemory(ctx context.Context, req *connect.Request[engramv1.UpdateMemoryRequest]) (*connect.Response[engramv1.UpdateMemoryResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	res, err := a.d.updateMemory(ctx, c, updateMemoryRequestToArgs(req.Msg))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(mutationResultToUpdateMemoryResponse(res)), nil
}
```
`SupersedeMemory`/`ArchiveMemory`/`RestoreMemory` slot into this exact shape: resolve caller → protoconv request→args → call the SAME `deps.*` the MCP tool calls → protoconv result→response → `connectError` on failure. The comment at lines 452-457 states this discipline explicitly as the rule for every write RPC in this file — extend it, do not restate it separately for the new RPCs.

**Thin-adapter read pattern to copy** (`ListMemories`, lines 244-331): resolve caller, parse any RFC3339 window bounds via `parseConnectWindowBound` (never a hand-rolled `time.Parse`), call the `deps.*` core, map the result via a proto-shaping helper (`shapeProtoMemories`, lines 160-174), return `connect.NewResponse(...)`. `ListRules`/`ListScheduled`/`RelatedMemories`/`ListTags` follow this shape; `RelatedMemories`/`ListTags` have **no MCP-side `deps.*` wrapper yet** (research RPC-04) — write the new `deps.relatedMemories`/`deps.listTagsCore` wrappers in `tools.go` first (see below), then call them here exactly as `ListMemories` calls `d.listMemory`.

**Coverage-field reuse pattern** (`searchedScopes`, lines 309-316, 380): `cov := a.d.searchedScopes(ctx, c, req.Msg.CrossSpine)` then copy `cov.Scopes`/`cov.Truncated`/`cov.Unknown` into the response's `searched_scopes`/`scopes_truncated`/`scopes_unknown` fields — this is the exact mechanism `ListRules`'s D-10 widening (empty `scopes` → enumerate every readable `rule:*` scope) and `ListScheduled`'s D-11 widening (`cross_spine`) must reuse, never a new coverage struct.

**Anti-pattern warning** (lines 260-269, 421-430): never hand-wrap a `connect.CodeInvalidArgument` at a boundary check — every rejection routes through the single `connectError(ctx, err)` mapper so the `*argError`'s `Class` (not a hand-picked code) selects the wire code. The `SearchDiscoveries` comment (lines 408-430) also documents a DELIBERATE one-off divergence (`Scope == ""` → cross-spine inference) that is explicitly NOT a pattern to copy onto the new RPCs — `cross_spine` must always be an explicit field read, never inferred from an empty scope.

---

### `internal/server/connectcsrf.go` (middleware, request-response)

**Analog:** `csrfWriteProcedures` (lines 28-40).

**Pattern to copy exactly**:
```go
var csrfWriteProcedures = map[string]bool{
	engramv1connect.EngramServiceStoreMemoryProcedure:    true,
	engramv1connect.EngramServiceStoreDiscoveryProcedure: true,
	engramv1connect.EngramServiceUpdateMemoryProcedure:   true,
	engramv1connect.EngramServiceDeleteMemoryProcedure:   true,
	engramv1connect.EngramServiceSetVisibilityProcedure:  true,
	engramv1connect.EngramServiceScheduleMemoryProcedure: true,
}
```
Add exactly three entries — `EngramServiceSupersedeMemoryProcedure`, `EngramServiceArchiveMemoryProcedure`, `EngramServiceRestoreMemoryProcedure` — keyed on the generated Procedure constants (never a hand-written path string, per the doc comment at lines 28-32). `ListRules`/`ListScheduled`/`RelatedMemories`/`ListTags` are reads and get **no** entry here — `newConnectCSRFInterceptor`'s map lookup (line 62) already treats an absent key as pass-through.

---

### `internal/server/protoconv.go` (utility, transform)

**Analog:** `updateMemoryRequestToArgs`/`mutationResultToUpdateMemoryResponse` (lines 44-74, 178-187); `storeMemoryRequestToArgs`/`idsToStoreMemoryResponse` (lines 96-109, 192-194).

**Request→Args pattern to copy**:
```go
func storeMemoryRequestToArgs(req *engramv1.StoreMemoryRequest) storeArgs {
	return storeArgs{
		Content:   req.GetContent(),
		Scope:     req.GetScope(),
		...
	}
}
```
`supersedeMemoryRequestToArgs` embeds this exact field set plus `Supersedes: req.GetSupersedes()` and `ValidateOnly: req.GetValidateOnly()` (D-08/D-09), mirroring how `scheduleMemoryRequestToArgs` (lines 127-144) embeds `storeArgs` plus its own extra fields. Use `req.Get<Field>()` accessors throughout (never direct field access) — every existing converter in this file does, so a nil sub-message never panics.

**Result→Response pattern to copy** (lines 181-198): a plain field-by-field struct literal, never a re-fetch. `ArchiveMemory`/`RestoreMemory`'s response (a repeated per-id outcome list) is the one shape with no existing 1:1 analog — build it as a `[]*engramv1.ArchiveOutcome`-shaped slice conversion function, following the same "no re-fetch, map exactly what the core returned" discipline.

**Do not re-validate here** (doc comment, lines 15-20): protoconv performs zero validation — the `buf.validate` CEL rules already ran in the interceptor chain (`connectapi.go` `mountConnect`). Do not add a presence/format check in a new converter function.

---

### `internal/server/tools.go` — new batch core `deps.archiveMemory`/`deps.restoreMemory` (service, batch — the phase's one genuinely new pattern)

**Analog A (id-resolution loop shape):** `resolveAndAuthorizeSupersedeTargets` (lines 2415-2532) — resolves each caller-supplied token via `d.st.ResolvePointID`, buckets failures separately from successes, and reports every offender by the caller's ORIGINAL input token, never the resolved UUID (404-indistinguishability, D-07's "unowned and nonexistent are indistinguishable").

**Analog B (per-id outcome vocabulary to plumb through unchanged):** `store.ArchiveResult`/`ArchiveOutcome` (`internal/store/spine.go:709-747`, verified):
```go
type ArchiveOutcome string

const (
	ArchiveOutcomeChanged  ArchiveOutcome = "changed"
	ArchiveOutcomeAlready  ArchiveOutcome = "already"
	ArchiveOutcomeNotFound ArchiveOutcome = "not_found"
)

type ArchiveResult struct {
	ID        string
	Requested string // the token the caller actually supplied
	Outcome   ArchiveOutcome
}
```

**Analog C (the single-id store method the batch core loops):** `ArchiveAs` (`internal/store/spine.go:837-866`, verified):
```go
func (s *Store) ArchiveAs(ctx context.Context, id string, subj Subject) (res ArchiveResult, err error) {
	unlock, lerr := s.locker.Lock(ctx, id)
	if lerr != nil {
		return ArchiveResult{ID: id}, lerr
	}
	defer unlock()

	cur, gerr := s.getWritable(ctx, id, subj, authz.ActionArchive)
	if gerr != nil {
		if errors.Is(gerr, ErrNotFound) {
			return ArchiveResult{ID: id, Outcome: ArchiveOutcomeNotFound}, gerr
		}
		return ArchiveResult{ID: id}, gerr
	}
	return s.archiveResolved(ctx, id, cur)
}
```

**Composition:** `deps.archiveMemory(ctx, c, ids []string)` validates shape first (`rejectOverMaximumCount("ids", uint64(len(ids)))`, non-empty, per-entry non-blank — mirroring the shape checks at lines 2424-2434), then loops `ids`, resolving each via `d.st.ResolvePointID` and calling `d.st.ArchiveAs(ctx, resolvedID, c.Subj)`; a resolution failure synthesizes its own `store.ArchiveResult{Requested: token, Outcome: ArchiveOutcomeNotFound}` row rather than calling `ArchiveAs` at all (Pitfall from RESEARCH.md Pattern 2) — this is what makes an unresolvable short_id indistinguishable from a resolved-but-not-owned id (D-07). **Never** build a store-level batch/multi-point primitive (Anti-Pattern in RESEARCH.md; the sequential per-id loop, each already lock-protected by `ArchiveAs`, IS the design per D-06/D-07). `deps.restoreMemory` mirrors this exactly, calling `d.st.RestoreAs` instead.

**Rejection envelope for shape violations** — reuse `argErrf`/`rejectOverMaximumCount` (`internal/server/tools.go:1836-1845`, `internal/server/argerror.go:145-152`), never a bespoke error type:
```go
func rejectOverMaximumCount(field string, count uint64) error {
	const maxCount = store.MaxRecallLimit
	if count <= maxCount {
		return nil
	}
	return argErrf(classMalformed, HintOutOfRange, field, "%s exceeds the maximum of %d", field, maxCount)
}
```

---

### `internal/server/tools.go` — new `deps.relatedMemories`/`deps.listTagsCore` wrappers (service, CRUD read)

**Analog:** `deps.searchDiscovery` (lines 2111-2151) — a thin validate-then-delegate-to-store wrapper with no `deps.*` predecessor of its own:
```go
func (d *deps) searchDiscovery(ctx context.Context, c caller, a searchDiscoveryArgs) ([]store.Memory, error) {
	if a.Query == "" {
		return nil, argErrf(classMalformed, HintRequired, "query", "query is required")
	}
	if err := rejectOverMaximumCount("k", a.K); err != nil {
		return nil, err
	}
	...
	return d.st.SearchDiscovery(ctx, scope, a.Kind, c.Subj, vec, a.K)
}
```
`deps.relatedMemories(ctx, c, a relatedArgs)` resolves `a.ID` via `d.st.ResolvePointID` (mirroring every other id-taking verb — never a new resolver, per RESEARCH.md's Don't-Hand-Roll table), rejects `k` over the maximum via `rejectOverMaximumCount`, then delegates straight to `d.st.RelatedMemories(ctx, resolvedID, c.Subj, a.K)` — the store method (`internal/store/relatedmemories.go:706-759`, verified) already composes the caller's read predicate into every sub-query and the final fetch (doc comment lines 4-11: *"never applied as a post-filter... no layer above internal/store may need to filter this method's output"*). **Never** add a post-fetch `if !canRead(...)` re-filter in this wrapper or in the Connect handler (Pitfall 4). `deps.listTagsCore(ctx, c, a listTagsArgs)` mirrors this shape over `d.st.ListTags(ctx, c.Subj, a.Scope, a.Limit)` (`internal/store/listtags.go:99-128`, verified — already rejects over-maximum internally via its own `rejectOverMaximum`, so the wrapper's only job is the empty-scope/full/limit-default plumbing, same as `deps.listScheduled`'s `a.Limit == 0` default at line 1899-1901).

---

### `internal/server/tools.go` — MCP tool registrations (route, request-response)

**Analog:** `list_scheduled` / `list_rules` (lines 2952-2960, 3066-3078) for a pure-read tool; `supersede_memory` (lines 3042-3054) for a tool with an id-set input.

**Read-tool registration pattern to copy exactly**:
```go
mcp.AddTool(s, &mcp.Tool{Name: "list_scheduled", Description: "...", Annotations: annotationsFor("list_scheduled")},
	func(ctx context.Context, _ *mcp.CallToolRequest, a listScheduledArgs) (*mcp.CallToolResult, any, error) {
		c, err := callerFromContext(ctx)
		if err != nil {
			return nil, nil, err
		}
		mems, err := d.listScheduled(ctx, c, a)
		return nil, map[string]any{"memories": mems}, err
	})
```
`related_memories`/`list_tags` follow this identically: `callerFromContext`, call the new `deps.*` core, return `(nil, map[string]any{...}, err)` — a nil `*mcp.CallToolResult` so go-sdk renders the structured result as the text content too (doc comment at line 3082-3087).

**Mutating id-set tool registration pattern** (`supersede_memory`, lines 3042-3054): note the custom-schema step (`supersedeMemoryInputSchema()`) only applies when a struct-tag `jsonschema` constraint (like `minItems`) can't express the bound — `archive_memory`/`restore_memory`'s `ids []string` needs the same `minItems: 1` treatment if the plan wants it advertised, otherwise a plain `jsonschema.For[T]`-inferred schema (every other tool's default) suffices:
```go
mcp.AddTool(s, &mcp.Tool{Name: "supersede_memory", Description: "...", InputSchema: supersedeSchema, Annotations: annotationsFor("supersede_memory")},
	func(ctx context.Context, _ *mcp.CallToolRequest, a supersedeArgs) (*mcp.CallToolResult, any, error) {
		c, err := callerFromContext(ctx)
		if err != nil {
			return nil, nil, err
		}
		id, sid, err := d.supersedeMemory(ctx, c, a)
		return textResult(fmt.Sprintf("stored %s, superseding %s", id, strings.Join(a.Supersedes, ", "))), map[string]string{"id": id, "short_id": sid}, err
	})
```
`archive_memory`/`restore_memory` return a per-id outcome list as the structured result (never `textResult` alone) — `err` stays nil whenever the batch call succeeds at the shape level, even if individual ids came back `not_found` (Pitfall 1: batch-succeeds ≠ every-id-succeeded).

**New arg structs** — follow `listScheduledArgs`/`idArgs` (lines 976-990) field-tagging convention: every field `omitempty` with a `jsonschema:"..."` doc tag (D-06a discipline, applied repo-wide):
```go
type idArgs struct {
	ID string `json:"id,omitempty" jsonschema:"the memory's full UUID or its short_id"`
}
```

---

### `internal/surfaces/toolclass.go` (config)

**Analog:** existing rows for `supersede_memory` (lines 138-147), `list_rules` (lines 154-157), `spine-review archive`/`spine-review restore` (lines 384-395, CLI-only today).

**Pattern to copy** — one `Operation` struct literal per new MCP tool, each with a comment justifying the four bools by D-09's conservative-stance rule ("false only when EVERY valid invocation is..."):
```go
{
	MCPTool: "list_rules", CLICommand: "",
	Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
},
```
`archive_memory`/`restore_memory` reuse the exact reasoning already written for `spine-review archive`/`spine-review restore` (lines 366-395: `Destructive: false` — reversible, content/tags/vector untouched, matching `set_visibility`'s precedent; `Idempotent: true` — an already-archived id reports `already` and issues no write) — add `MCPTool: "archive_memory"` as a **new** row (the CLI rows stay CLI-only per D-05: no CLI verb this phase) rather than editing the existing CLI-only rows. `related_memories`/`list_tags` are `ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false`, matching `list_rules`/`search_discovery`. `supersede_memory` needs no new row (tool already exists) — only its Connect-side annotation wiring changes, which reads the same existing row via `annotationsFor("supersede_memory")` (`internal/server/toolannotations.go:24-35`).

---

### `internal/server/connectcsrf_test.go` (test)

**Analog:** `csrfWriteCases` (lines 107-161) and `TestCSRFWriteProcedureAllowlist` (lines 168+).

**Pattern to copy** — one new `csrfWriteRPCCase` entry per new write RPC, each supplying a minimal protovalidate-valid payload:
```go
{
	name: "UpdateMemory",
	call: func(ctx context.Context, c engramv1connect.EngramServiceClient, h csrfHeaders) error {
		return doCSRFWrite(ctx, c.UpdateMemory, &engramv1.UpdateMemoryRequest{
			Id: "some-id", Content: "new content",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}},
		}, h)
	},
},
```
Add `SupersedeMemory`/`ArchiveMemory`/`RestoreMemory` entries; `TestCSRFWriteProcedureAllowlist`'s `wantWrite` slice grows from 6 to 9 names. **D-15 rule: never iterate `csrfWriteProcedures`** — list the three new Procedure names explicitly in the test, exactly as this existing slice already does by hand.

---

### `internal/server/connectcsrf_lane_test.go` (test)

**Analog:** `TestCSRFCookieCallerOmittingHeaderIsStillRejected` (lines 166-186) for the missing-token → `permission_denied` case; `TestCSRFCookieLaneStillEnforcesDoubleSubmit` (lines 235-257) for the paired valid-token success case (D-15 requires BOTH, paired, per RPC).

**Pattern to copy** — both tests already index `csrfWriteCases(...)[0]` (StoreMemory); D-15's new tests index the SupersedeMemory/ArchiveMemory/RestoreMemory entries added to `csrfWriteCases` above (by name/index, not by iterating the map):
```go
func TestCSRFCookieLaneStillEnforcesDoubleSubmit(t *testing.T) {
	...
	call := csrfWriteCases(timestamppb.New(time.Now().Add(time.Hour)))[0].call
	err := call(ctx, client, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken, hasHeader: true, headerValue: validToken})
	if err != nil {
		t.Fatalf("cookie-lane write with matching CSRF cookie+header: got err %v, want success", err)
	}
}
```
Write three pairs (missing-token/valid-token) — one pair per new write RPC — following this exact shape, selecting the new case by name instead of index `[0]` if that's clearer.

---

### `internal/server/connectapi_write_parity_test.go` (test)

**Analog:** `TestWriteParity`'s `t.Run("StoreMemory", ...)` subtest (lines 172-219).

**Pattern to copy** — one `t.Run` block per write RPC, each building two independent `spyDeps`, calling the MCP-lane `deps.*` method directly and the Connect handler method, then asserting code parity + store-trace parity:
```go
t.Run("StoreMemory", func(t *testing.T) {
	ctx := context.Background()
	dMCP, spMCP := newSpyDeps()
	dConn, spConn := newSpyDeps()
	mcpCaller := parityMCPCaller(t, owner, mcpActor)
	connCtx := parityConnectCtx(owner)
	api := &engramAPI{d: dConn}

	mcpID, _, mcpErr := dMCP.storeMemory(ctx, mcpCaller, storeArgs{ /* ... */ })
	connResp, connErr := api.StoreMemory(connCtx, connect.NewRequest(&engramv1.StoreMemoryRequest{ /* same inputs */ }))
	assertCodeParity(ctx, t, mcpErr, connErr)
	assertSameStoreTrace(t, spMCP, spConn)
})
```
Add `t.Run("SupersedeMemory", ...)`, `t.Run("ArchiveMemory", ...)`, `t.Run("RestoreMemory", ...)` blocks — including one exercising `validate_only=true` (D-18(a): re-read shows no new record) inside the `SupersedeMemory` block or as a sibling `t.Run`.

---

### `internal/server/connectapi_parity_test.go` (test)

**Analog:** the shared fixture helpers (`newParityLane`, `parityMCPCaller`, `parityConnectCtx`, `assertCodeParity`, `seedBothLanes`, lines 43-101) that both parity test files already share.

**Pattern to copy** — `ListRules`/`ListScheduled`/`RelatedMemories`/`ListTags` (read-only) get rows/subtests here using the SAME fixture helpers `connectapi_write_parity_test.go` uses, but without a store-mutation trace comparison (read RPCs have no write to trace) — compare returned data + code parity only, following the read-lane shape this file already establishes for `ListMemories`/`SearchMemories`.

---

### `internal/server/connectdescriptor_test.go` (test — structural edit, D-26)

**Analog:** the file's own `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` (full file, 230 lines).

**What to keep** (lines 116-131 exactly, unmodified logic):
```go
var opts *descriptorpb.MethodOptions
if o := md.Options(); o != nil {
	var ok bool
	opts, ok = o.(*descriptorpb.MethodOptions)
	if !ok {
		t.Fatalf("%s: unexpected options type %T", name, o)
	}
}
if opts.GetIdempotencyLevel() != descriptorpb.MethodOptions_IDEMPOTENCY_UNKNOWN {
	t.Errorf("%s: idempotency_level = %v, want IDEMPOTENCY_UNKNOWN (SC2/D-12 guard)", name, opts.GetIdempotencyLevel())
}
```
**What to change:** `methods.Len() != 12` (line 80) becomes `!= 19`. **What to DELETE** (D-26): the entire `wantReqResp` map (lines 84-100) and the per-method name/req/resp-type loop that consumes it (lines 102-114, 132-136) — `buf breaking` already guards wire compatibility and D-15..D-21's round-trip tests already prove the new shapes; a second descriptor-level name/type pin restates the `.proto`. The existing `assertFields` calls for `Memory`/`ScopeCount`/the read-lane messages (lines 151-230) are untouched — Phase 3 does not change those messages.

---

### `skill/engram/skills/curating-memory/SKILL.md` (doc, D-04)

**Analog:** the existing verb-discrimination table contrasting `supersede_memory` vs `delete_memory` (lines 331-340) and the MCP tool-name enumeration (lines 503-504).

**Pattern to copy** — add `archive_memory`/`restore_memory` to the same decision table those two rows already occupy (D-01's framing: "`delete_memory` removes junk, `supersede_memory` records a reversal, archive retires a record that is still true but no longer useful, reversibly" — the consent rule curating-spine already applies to every mutation). Add `related_memories` framed like `search_discovery`'s on-demand-only guidance (D-02: never at session start, never an automatic follow-up to a search). Add `list_tags` framed as the tag-reuse check before `store_memory` (D-03). Extend the MCP tool-name list at lines 503-504 with all four new names.

---

### `CLAUDE.md` §Memory contract (doc, D-04)

**Analog:** the existing sentence enumerating the tool set (`store_memory` / `schedule_memory` / `search_memory` / ... / `delete_all`) at the top of the Memory contract section, and the "Archived state" / "Supersession" paragraphs later in the same section.

**Pattern to copy** — add `archive_memory` / `restore_memory` / `related_memories` / `list_tags` to the tool enumeration sentence; add a short paragraph for each in the same terse, contract-stating register the existing "Archived state:" paragraph uses (e.g. "`archive_memory`/`restore_memory` take `repeated string ids`... one outcome per id... never a delete"), each discriminating the new verb from its siblings per D-04.

---

### `docs-site/src/content/docs/reference/tools.md`, `reference/memory-record.md` (doc, D-04)

**Analog:** the existing per-tool documentation sections for `supersede_memory`/`list_rules`/`list_scheduled` (same file, mirroring the same contract prose CLAUDE.md states).

**Pattern to copy** — one new subsection per new tool, in the same per-tool doc-section shape already used for every existing tool (name, when to call it, input/output shape, discrimination against siblings). `reference/errors.md` needs no new row unless a genuinely new `HintCode` is introduced (RESEARCH.md/CONTEXT.md do not require one — the existing `HintRequired`/`HintTooMany`/`HintOutOfRange` vocabulary already covers empty/over-cap/malformed batch input).

---

### `.claude/skills/engram-connect-client/SKILL.md` (doc)

**Analog:** the existing per-RPC table distinguishing `engram` (read client) vs `engramWrite` (CSRF-carrying write client) calls.

**Pattern to copy** — add one row per new RPC: `SupersedeMemory`/`ArchiveMemory`/`RestoreMemory` under `engramWrite` (CSRF applies), `ListRules`/`ListScheduled`/`RelatedMemories`/`ListTags` under `engram` (plain read client).

## Shared Patterns

### Thin Connect adapter delegating to the same `deps.*` core as the MCP tool (SC2)
**Source:** `internal/server/connectapi.go:452-457` (doc comment) and `UpdateMemory` (lines 486-496).
**Apply to:** every one of the 7 new Connect handlers, without exception.
```go
c, err := callerFromConnectContext(ctx)
if err != nil {
	return nil, connect.NewError(connect.CodeUnauthenticated, err)
}
res, err := a.d.<sameCoreMCPToolCalls>(ctx, c, <protoconvRequestToArgs>(req.Msg))
if err != nil {
	return nil, connectError(ctx, err)
}
return connect.NewResponse(<protoconvResultToResponse>(res)), nil
```

### The one rejection envelope (`argError`/`HintCode`)
**Source:** `internal/server/argerror.go` (whole file, esp. lines 85-152).
**Apply to:** every shape-violation rejection in the new batch core and the new proto validation (`field=<f> hint=<code>: <text>`). Never a bespoke error type; never a hand-wrapped Connect code — `connectError` (mapped from `argError.Class`) is the single mapper.

### Authz stays store-only (DEC-cgb)
**Source:** `store.ArchiveAs`/`RestoreAs` (`internal/store/spine.go:837-866, 948-977`) — `getWritable(ctx, id, subj, authz.ActionArchive)`.
**Apply to:** every new handler/tool/core — never add a second ownership check in `internal/server`. The highest-risk defect this phase can introduce is a handler calling the subject-less `Archive`/`Restore` instead of `ArchiveAs`/`RestoreAs` (D-16); the fix is always "call the owner-gated sibling," never "add a guard above the store."

### CSRF allowlist is Procedure-constant-keyed, never a hand path list
**Source:** `internal/server/connectcsrf.go:33-40`.
**Apply to:** the 3 new write RPCs — `SupersedeMemory`/`ArchiveMemory`/`RestoreMemory`. Read RPCs get no entry.

### Coverage-field triplet (`searched_scopes`/`scopes_truncated`/`scopes_unknown`)
**Source:** `internal/server/tools.go:2054-2109` (`searchedScopes`, `scopeCoverage`, `recallResultMap`).
**Apply to:** `ListRules`'s D-10 empty-scopes widening and `ListScheduled`'s D-11 `cross_spine` widening, on both lanes — reuse the existing struct and helper; do not invent a second coverage mechanism.

### Blast-radius Class registry is the single source for MCP annotations and the CLI catalog
**Source:** `internal/surfaces/toolclass.go` (`operations` slice) + `internal/server/toolannotations.go:24-35` (`annotationsFor`).
**Apply to:** every new MCP tool — add one `Operation` row; `annotationsFor(name)` and the catalog both derive from it automatically. A missing row is caught by construction (existing `TestToolAnnotationsBothDirections`/catalog set-equality tests), so no new test is needed for this (D-23).

## No Analog Found

None. Every file this phase touches extends a tracked file that already exists in the repository; the two genuinely new pieces of logic (the archive/restore batch-outcome core, and the `RelatedEdge` proto oneof) are new **functions/messages inside existing files**, not new files, and each has a documented closest analog above (Pattern 2 / the oneof design note) even though no 1:1 precedent exists yet.

## Metadata

**Analog search scope:** `internal/server/`, `internal/store/`, `internal/surfaces/`, `proto/engram/v1/`, `skill/engram/skills/`, `docs-site/src/content/docs/reference/`, `.claude/skills/engram-connect-client/`
**Files scanned:** `internal/server/connectapi.go`, `connectcsrf.go`, `connectcsrf_test.go`, `connectcsrf_lane_test.go`, `protoconv.go`, `tools.go`, `rules.go`, `toolannotations.go`, `argerror.go`, `hiddencount.go`, `connectapi_parity_test.go`, `connectapi_write_parity_test.go`, `connectdescriptor_test.go`; `internal/store/spine.go`, `relatedmemories.go`, `listtags.go`; `internal/surfaces/toolclass.go`; `proto/engram/v1/engram.proto`; `skill/engram/skills/curating-memory/SKILL.md` — all confirmed git-tracked (`git ls-files`) before citing.
**Pattern extraction date:** 2026-09-26
