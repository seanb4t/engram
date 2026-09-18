---
phase: quick-260918-idl
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/store/store.go
  - internal/store/store_test.go
  - ui/src/lib/queries.ts
  - ui/src/lib/queries.test.ts
  - ui/src/routes/+page.svelte
  - ui/src/routes/observe/+page.svelte
  - ui/src/routes/page.browser.test.ts
  - internal/webauth/static/**
  - internal/e2e/console_browser_test.go
autonomous: true
requirements:
  - GH-500
  - QUICK-260918-idl

estimate:
  tokens: 100000
  raw_tokens: 100000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "`Store.ListScopes` succeeds when the caller's readable set carries more than 4 MiB of full payload. It returns exact per-scope counts instead of failing with gRPC `ResourceExhausted`, which the Connect `ListScopes` handler surfaced as `internal` (HTTP 500)"
    - "`Store.ListScopes` stays ONE bounded `Scroll` call: `scanCap` is still 1000, and `approximate` still means `len(points) == scanCap`. It remains the same classified recall transmitter in `schemaversion_recallgate_test.go`, and that gate stays green unchanged"
    - "The regression test fails with `ResourceExhausted` when the old full-payload selector is put back. The failing output is recorded in the SUMMARY, so the test is proven non-vacuous"
    - "Cross-spine `search_memory`/`list_memory`/`ListMemories`/`SearchMemories` no longer fail on a large store. `deps.searchedScopes` calls `Store.ListScopes` (`internal/server/tools.go`), so every cross-spine recall inherited Defect 1"
    - "The console root route's Recent memories panel calls `ListMemories` with `crossSpine: true` and an empty scope. It renders the caller's most recent readable records across every scope instead of `failed to load` (GH #500). The server's explicit cross_spine contract (D-04, `TestConnectCrossSpineNotInferred`) is unchanged"
    - "The root route builds its query key through the shared `listMemoriesKey` helper. `crossSpine` is appended at index 9, so indexes 1–8 keep their positions and `applyToMemoryCaches` still reads visibility from `key[3]`"
    - "`internal/webauth/static/` is byte-identical to a fresh `task ui:build` of the fixed `ui/` source. A second `task ui:build` leaves `git status` clean for that directory"
    - "A real headless-Chrome e2e run against the real engram binary renders the seeded fixture marker on `/ui/` itself, and no `ListScopes`/`ListMemories` RPC returns HTTP >= 400 in either navigation"
    - "No gRPC client receive-size limit is raised anywhere, and `go.mod`/`go.sum`/`ui/package.json`/`ui/pnpm-lock.yaml` are byte-unchanged"
  artifacts:
    - path: "internal/store/store.go"
      provides: "ListScopes scrolling with a scope-only payload selector and reading the scope key directly"
      contains: 'NewWithPayloadInclude("scope")'
    - path: "internal/store/store_test.go"
      provides: "real-Qdrant regression test whose full payloads exceed grpc-go's 4 MiB default receive limit"
      contains: "TestListScopesFullPayloadsOverGRPCLimit"
    - path: "ui/src/lib/queries.ts"
      provides: "listMemoriesKey with a trailing, explicit crossSpine slot at index 9"
      contains: "crossSpine"
    - path: "ui/src/routes/+page.svelte"
      provides: "recentQ requesting a cross-spine recent-activity feed"
      contains: "crossSpine: true"
    - path: "ui/src/routes/page.browser.test.ts"
      provides: "browser-mode Vitest pinning recentQ's request args and cache-key shape"
      contains: "crossSpine"
    - path: "internal/webauth/static/index.html"
      provides: "regenerated vendored SPA carrying the fixed root route"
    - path: "internal/e2e/console_browser_test.go"
      provides: "root-route Recent-memories render assertion plus a failed-RPC gate for ListScopes/ListMemories"
      contains: "EngramServiceListMemoriesProcedure"
  key_links:
    - from: "internal/store/store.go"
      to: "Qdrant Scroll (points API)"
      via: "Store.ListScopes asks Qdrant for only the scope payload key, so one response is at most scanCap scope strings"
      pattern: 'NewWithPayloadInclude[(]"scope"[)]'
    - from: "internal/server/tools.go"
      to: "internal/store/store.go"
      via: "deps.searchedScopes calls Store.ListScopes on every cross_spine recall, so the store fix is a prerequisite for the UI fix working live"
      pattern: "d[.]st[.]ListScopes[(]ctx, c[.]Subj[)]"
    - from: "ui/src/routes/+page.svelte"
      to: "internal/server/connectapi.go"
      via: "recentQ sends ListMemories with cross_spine=true, which passes effectiveSearchScope for an empty scope"
      pattern: "crossSpine: true"
    - from: "ui/src/routes/+page.svelte"
      to: "ui/src/lib/queries.ts"
      via: "recentQ's queryKey is built by the shared listMemoriesKey helper, never an inline array"
      pattern: "listMemoriesKey[(]"
    - from: "internal/e2e/console_browser_test.go"
      to: "internal/webauth/static/index.html"
      via: "the e2e binary embeds the vendored SPA at build time; the root-route poll only passes against the regenerated bundle"
      pattern: "rootRoutePollExpr"
  prohibitions:
    - statement: "MUST NOT fix Defect 1 by raising the gRPC client receive-message size limit on any qdrant client (production or test). That only moves the ceiling; the fix is to stop requesting payload ListScopes never reads"
      category: correctness
      status: resolved
      verification: test
      reason: "Task 1 acceptance runs a repo-wide zero-count gate over internal/ and cmd/ Go sources for the grpc receive-limit option names"
    - statement: "MUST NOT replace ListScopes' Scroll with a different Qdrant RPC. The AST-derived recall-gate classification lists Store.ListScopes as a Scroll transmitter, and its interceptor recognizes only Query/Scroll/Count; widening that gate is out of scope"
      category: scope
      status: resolved
      verification: test
      reason: "TestRecallEmissionSetIsCompleteAndClassified and TestSchemaVersionNeverGatesRecall run unchanged in Task 1's verify; a region-scoped gate asserts exactly one s.client.Scroll( call inside ListScopes"
    - statement: "MUST NOT change the server to infer cross_spine from an empty scope on ListMemories/SearchMemories (D-04, pinned by TestConnectCrossSpineNotInferred). The fix is on the client"
      category: scope
      status: resolved
      verification: test
      reason: "files_modified contains no internal/server file; TestConnectCrossSpineNotInferred runs in the final task gate"
---

<objective>
Fix the two independent defects that break the operator console root route (`/ui/`), which is live on engram.fzymgc.house and shows "failed to load scopes" and "failed to load" for Recent memories:

1. **Defect 1 (store):** `Store.ListScopes` scrolls up to `scanCap = 1000` points with FULL payloads in a single gRPC response. The store has grown past grpc-go's 4 MiB default client receive limit, so the scroll fails with `ResourceExhausted` and Connect returns `internal`. The fix, already decided: request only the `scope` payload key, keep it a `Scroll`, and keep scanCap/approximate unchanged.
2. **Defect 2 (UI, GH #500):** the root route's `recentQ` calls `ListMemories` with an empty scope and no `crossSpine`. Under D-04 the server correctly rejects that with `invalid_argument`. The fix: send `crossSpine: true` so the panel becomes a global recent-activity feed.

Coupling (verified at planning time): cross-spine `ListMemories` calls `deps.searchedScopes`, which calls `Store.ListScopes` (`internal/server/tools.go:1639-1652`). If only the UI fix were deployed, the live panel would change from a 400 to a 500. Both fixes must land in the same PR, and Task 1 goes first.

Purpose: restore the console's landing page and every cross-spine recall path on a production-sized store, with regression coverage at the store tier (real Qdrant, proven RED) and at the browser tier (real headless Chrome against the real binary).

Output: 3 atomic Conventional Commits on branch `fix/console-root-route`: store fix + test, UI fix + vendored SPA, e2e assertion.

Decomposition note: tracer-first is deliberately not applied (the `--no-tracer` shape). This is a bugfix on proven architecture. Each task is already verified end-to-end at its own tier: Task 1 against real Qdrant, Tasks 2–3 in a real browser. A thin tracer slice would add no information.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@CLAUDE.md
@internal/store/store.go
@internal/store/store_test.go
@internal/store/spine.go
@internal/store/schemaversion_recallgate_test.go
@internal/server/tools.go
@internal/server/connectapi.go
@ui/src/routes/+page.svelte
@ui/src/routes/page.browser.test.ts
@ui/src/routes/observe/+page.svelte
@ui/src/lib/queries.ts
@ui/src/lib/queries.test.ts
@ui/src/lib/mutations/memory.ts
@internal/e2e/console_browser_test.go
@internal/e2e/harness_test.go

<interfaces>
Facts observed live at planning time. Line numbers are as of HEAD c640c3a8.

- `internal/store/store.go:1663-1703`: `func (s *Store) ListScopes(ctx context.Context, subj Subject) (out []ScopeCount, more bool, err error)`. Its body has `const scanCap = 1000`, one `s.client.Scroll(ctx, &qdrant.ScrollPoints{...})` with `Filter` = `ownerOrSharedCondition(ctx, subj)`, `Limit` = scanCap, and a full-payload `WithPayload` selector. It then aggregates `counts[fromPayload(p.Id.GetUuid(), p.Payload).Scope]++` and returns `len(pts) == scanCap` as `more`.
- `internal/store/spine.go:558-565` is the in-repo precedent for a partial selector plus a direct key read: it passes `qdrant.NewWithPayloadInclude("short_id", "scope")` and reads `p.Payload["scope"].GetStringValue()`.
- `internal/store/store.go:678` `fromPayload` checks for each key before reading it, so a partial payload would not panic. Still, decoding a whole `Memory` for one key is wasteful and misleading, so read the key directly (see Task 1).
- `internal/store/store.go:795` `Store.Upsert(ctx, m Memory, vec []float32)` writes `payload(m)` straight to Qdrant with NO content-size validation, so a test can seed oversized content.
- `internal/store/store_test.go:310` `testStore(t)` uses collection `testCollection("mem_eval_test")` with dim 3 and is skipped when no Qdrant is available unless `ENGRAM_REQUIRE_QDRANT=1`. `dialTestClient` (line 170) builds `qdrant.NewClient(&qdrant.Config{Host, Port})` with no gRPC options. It therefore has the same 4 MiB default receive limit as production (`internal/server/tools.go:123-129`, which only adds an otel stats handler), and that is what makes the RED reproducible. `TestMain` boots a testcontainers Qdrant if `ENGRAM_QDRANT_TEST_ADDR` is unset (Docker 29.8.0 is available locally).
- `internal/store/store_test.go:1748` `(s *Store) DeleteAllRaw(ctx, scope)` and `:1758` `TestListScopes` (the pattern to follow). `:1873` `TestListExactTotalPastOldCap` is the precedent for skipping a bulk-write test under `-short`. Tests in this file run sequentially.
- `internal/server/tools.go:1639` `func (d *deps) searchedScopes(ctx, c caller, crossSpine bool) ([]string, bool, error)` calls `d.st.ListScopes(ctx, c.Subj)` when crossSpine is true.
- `ui/src/lib/queries.ts:47-52` `listMemoriesKey(scope, categories, visibility, limit, offset, includeArchived, includeSuperseded, includeScheduled)` returns a 9-element array. Its only production caller is `ui/src/routes/observe/+page.svelte:29`, and it is pinned in `ui/src/lib/queries.test.ts:14-15`. `ui/src/lib/mutations/memory.ts:209` reads `key[3]` of every `['listMemories', …]` query as the visibility filter.
- `ui/src/routes/+page.svelte:15-18` `recentQ` is `queryKey: ['listMemories', '', [], '', PAGE_LIMIT, 0]` with `queryFn: () => engram.listMemories({ scope: '', limit: BigInt(PAGE_LIMIT), offset: 0n, categories: [], visibility: '' })`.
- `ui/src/lib/gen/engram/v1/engram_pb.ts:354` `ListMemoriesRequest.crossSpine: boolean`.
- `ui/src/routes/page.browser.test.ts` already mocks `engram.listMemories` as `listMemoriesSpy` (hoisted), renders `RootPage` inside a `QueryClientProvider` with a per-test `qc`, and runs in the Vitest `browser` project (Playwright chromium; Playwright browsers are present under `~/Library/Caches/ms-playwright`).
- Vitest projects (`ui/vite.config.ts`): `node` covers `src/**/*.test.ts` excluding `*.browser.test.ts`, and `browser` covers `src/**/*.browser.test.ts`.
- `Taskfile.yaml` `ui:build` runs `pnpm install --frozen-lockfile`, `pnpm build`, then `rm -rf ../internal/webauth/static` and copies `build/.` into it. CI job `ui vendored-asset drift` fails if the committed tree differs.
- `internal/e2e/console_browser_test.go`: `TestConsoleBundleRendersRecordInBrowser` (line 357) navigates `/ui/` for hydration only, then `/ui/observe?scope=<fixtureScope>` for the marker. The workaround prose is at lines 46-49 (`fixtureScope` doc), 311-321 (`markerPollExpr` doc, whose closing sentence says 05-04 left the vendored bundle alone), and 336-356 (the "Route 2 is required because…" paragraph). `browserObserver.attach` records every HTTP >= 400 into `failedURLs`, but `assertClean` only fails on URLs under `consoleAssetPathPrefix`. `engramv1connect` is already imported. The run context is `context.WithTimeout(browserCtx, 90*time.Second)`. Each server boot gets its own Qdrant collection (`harness_test.go:330`, per-port). Chrome is present at `/Applications/Google Chrome.app/...` (`findChrome` macOS fallback). The e2e `TestMain` builds the engram binary from the working tree, so the vendored static is embedded at test time.
- MemoryList's error text is `failed to load — retry from the toolbar`. The root route's scopes error text is `failed to load scopes`, and its loading text is `loading scopes…`.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: ListScopes requests only the scope payload key, with a real-Qdrant regression test proven RED</name>
  <files>internal/store/store.go, internal/store/store_test.go</files>
  <precondition>Docker is running (testcontainers boots Qdrant for internal/store), or ENGRAM_QDRANT_TEST_ADDR points at a live Qdrant gRPC port.</precondition>
  <behavior>
    - TestListScopesFullPayloadsOverGRPCLimit: seed 40 records into one uniquely named scope, each owned by a unique owner and carrying 128 KiB of content (5 MiB total, above grpc-go's 4194304-byte default). `ListScopes(ctx, Authenticated(<that owner>))` returns a nil error, and that scope's count is exactly 40.
    - The test guards its own non-vacuity: it fails fast if `n * contentBytes <= 4 << 20`, so a later edit cannot quietly shrink the fixture below the limit.
    - Pre-fix (full-payload selector restored): the same test fails with an error containing `ResourceExhausted` and `larger than max 4194304`.
    - The existing TestListScopes (per-scope counts, a foreign owner excluded, approximate=false) still passes unchanged.
  </behavior>
  <action>
    RED first. In internal/store/store_test.go, directly after TestListScopes, add TestListScopesFullPayloadsOverGRPCLimit:
    - Skip under testing.Short() with a message saying it writes about 5 MiB of payload. Mirror TestListExactTotalPastOldCap's skip.
    - Build the store with testStore(t) and use context.Background().
    - Use scope "ls-grpc-limit-test:project:big" and owner "sub-ls-grpc-limit". Both are unique to this test, so records written by sibling tests (sub-A, sub-B) cannot enter the result.
    - Register a deferred cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) BEFORE the first upsert, so a mid-loop failure still cleans up.
    - Declare constants n = 40 and contentBytes = 128 << 10. Add a t.Fatalf guard if n*contentBytes <= 4<<20, with a message saying the fixture no longer exceeds grpc-go's default 4 MiB client receive limit.
    - Build the content once with strings.Repeat("x", contentBytes).
    - Upsert n records via s.Upsert. Each gets ID fmt.Sprintf("c2222222-0000-0000-0000-%012d", i), Scope = scope, Owner = owner, CreatedAt = time.Now().UTC(), and vector []float32{0.1, 0.2, 0.3}. Store.Upsert bypasses API content-size validation, which is intentional here.
    - Call s.ListScopes(ctx, Authenticated(owner)). On error, t.Fatalf with the error; the message must say the full-payload scroll exceeded the gRPC receive limit. Otherwise, build a map from the result and assert counts[scope] == n.
    - Do NOT assert on `approximate`. The shared collection may legitimately carry `shared` records from elsewhere, and this test pins the request shape, not scanCap.
    - This test covers OUR request shape against real Qdrant, not Qdrant's own behavior (rule m45p2b4bp7).

    Run the test once on the unmodified code and confirm it FAILS with ResourceExhausted, using the Task 1 verify command restricted to this test name. Save the failing line(s) verbatim for the SUMMARY.

    GREEN. In internal/store/store.go, Store.ListScopes:
    - Replace the full-payload WithPayload selector with qdrant.NewWithPayloadInclude("scope"). This follows the spine.go:558 precedent.
    - Replace the fromPayload(...).Scope aggregation with a direct read of the scope key, p.GetPayload()["scope"].GetStringValue(). This matches spine.go's idiom and avoids decoding a whole Memory for one field.
    - Keep everything else byte-identical in behavior: the single s.client.Scroll call, the ownerOrSharedCondition filter, scanCap = 1000, the sort, and more = len(pts) == scanCap.
    - Do NOT switch to Facet or any other Qdrant RPC. The recall-gate AST classification lists Store.ListScopes as a Scroll transmitter, and its interceptor recognizes only Query/Scroll/Count.
    - Do NOT raise the gRPC client's receive-message size limit on any qdrant client, in production or test. Leave the other Scroll sites in store.go alone; they are bounded by caller page limits and out of scope.
    - Extend the ListScopes doc comment with one or two sentences: the scan requests only the scope payload key because the aggregation needs nothing else, and requesting full payloads for up to scanCap points overflowed grpc-go's default 4 MiB client receive limit in production (ResourceExhausted, surfaced by Connect as internal).

    Re-run the new test and the recall-gate tests (Task 1 verify) and confirm GREEN. Then prove RED deliberately: restore ONLY the old full-payload selector in ListScopes (the direct scope read works with a full payload too), run the new test, observe ResourceExhausted, restore the fix, and re-run to green. Keep the RED command, the failing output line, and the green re-run for the SUMMARY. Do not register a red-evidence patch in redEvidenceDirs: no milestone is open, and that harness is active-milestone-only.

    Commit (one atomic commit, explicit pathspec, never a gsd commit verb):
    - Stage with git add -- internal/store/store.go internal/store/store_test.go, then run git commit -F with a message file in the session scratchpad, followed by the same explicit pathspec.
    - Subject: fix(store): request only scope payload in ListScopes to stay under gRPC 4MiB limit
    - Body: a short paragraph naming the live symptom, which is ListScopes returning Connect internal from Scroll ResourceExhausted, and noting that it also unblocks every cross_spine recall, since searchedScopes calls ListScopes.
    - Then the line: Quick task: 260918-idl
    - Final paragraph, with both trailers adjacent: Co-Authored-By: Claude Opus 5 <noreply@anthropic.com> and Claude-Session: https://claude.ai/code/session_01BCmz8dAK9R6JqM8Fa9aYj8
  </action>
  <verify>
    <automated>ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestListScopes|TestListScopesFullPayloadsOverGRPCLimit|TestRecallEmissionSetIsCompleteAndClassified|TestSchemaVersionNeverGatesRecall|TestFilterWalkerSeesEveryPosition)$' -count=1 -v</automated>
  </verify>
  <acceptance_criteria>
    - The verify output contains `--- PASS: TestListScopesFullPayloadsOverGRPCLimit` and `--- PASS: TestListScopes`, and contains no `--- SKIP` for either. This guards against the false-green trap bsbsvn4hbc: a `-run` that matches nothing still reports `ok`.
    - The verify output contains `--- PASS: TestRecallEmissionSetIsCompleteAndClassified` and `--- PASS: TestSchemaVersionNeverGatesRecall`, with no edits to schemaversion_recallgate_test.go (`git diff --quiet c640c3a8 -- internal/store/schemaversion_recallgate_test.go` exits 0 after the commit; c640c3a8 is this branch's base).
    - Region-scoped positive gate: `sed -n '/^func (s \*Store) ListScopes(/,/^}/p' internal/store/store.go | rg -o 'NewWithPayloadInclude\("scope"\)' | wc -l` prints 1.
    - Region-scoped negative gate (it would print 1 on the pre-fix code): `sed -n '/^func (s \*Store) ListScopes(/,/^}/p' internal/store/store.go | rg -o 'NewWithPayload\(true\)' | wc -l` prints 0.
    - Still exactly one Scroll call in ListScopes: `sed -n '/^func (s \*Store) ListScopes(/,/^}/p' internal/store/store.go | rg -o 's\.client\.Scroll\(' | wc -l` prints 1.
    - No receive-limit raise anywhere: `rg -o 'MaxCallRecvMsgSize|MaxRecvMsgSize' --glob '*.go' internal cmd | wc -l` prints 0.
    - The SUMMARY records the RED run (the command plus the failing line containing `ResourceExhausted` and `4194304`) and the green re-run.
    - `git log -1 --format=%B` contains `Quick task: 260918-idl`, `Co-Authored-By: Claude Opus 5`, and `Claude-Session:`. `git show --stat HEAD` lists exactly the two Go files.
  </acceptance_criteria>
  <done>ListScopes scrolls a scope-only payload and passes a real-Qdrant test that seeds 5 MiB of full payload and demonstrably fails on the old selector. The recall gate is green and untouched, and one fix(store) commit is on fix/console-root-route.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Root-route Recent memories sends cross_spine (#500) via the shared key helper, plus the regenerated vendored SPA</name>
  <files>ui/src/lib/queries.ts, ui/src/lib/queries.test.ts, ui/src/routes/+page.svelte, ui/src/routes/observe/+page.svelte, ui/src/routes/page.browser.test.ts, internal/webauth/static/**</files>
  <precondition>pnpm is on PATH (/opt/homebrew/bin/pnpm) and ui/node_modules is installed. If it is missing, `task ui:build` installs it with --frozen-lockfile, or run `pnpm -C ui install --frozen-lockfile` first. If the browser project reports a missing Playwright browser, run `pnpm -C ui exec playwright install chromium` (browser binaries only, no new dependency).</precondition>
  <behavior>
    - page.browser.test.ts, new describe block for the Recent memories cross-spine feed (GH #500). After rendering RootPage with no resume envelope, listMemoriesSpy is called, and its first argument matches { scope: '', crossSpine: true }.
    - Same describe: after the call, the QueryClient cache (qc.getQueryCache().findAll({ queryKey: ['listMemories'] })) holds exactly one entry. Its queryKey[3] is '' (visibility, where applyToMemoryCaches reads it) and its queryKey[9] is true (crossSpine).
    - queries.test.ts: listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20, false, false, false, false) equals the previous 9-element array plus a trailing false. A call differing only in crossSpine yields a different key whose indexes 0–8 are identical.
    - Pre-fix, the new browser assertions fail: listMemoriesSpy receives no crossSpine field, and the key has no index 9.
  </behavior>
  <action>
    RED first:
    - Add the behavior cases above to ui/src/routes/page.browser.test.ts, reusing the existing hoisted listMemoriesSpy, renderRoot(), and the per-test qc. Wait for the call with expect.poll, the same way the existing tests poll gotoSpy.
    - Add the key cases to ui/src/lib/queries.test.ts. Update the existing "builds a stable list query key" expectation to the 10-argument form with a trailing false.
    - Run both Task 2 verify commands and confirm the new assertions fail. Save the failing assertion lines for the SUMMARY.

    GREEN:
    - In ui/src/lib/queries.ts, add a trailing required parameter crossSpine: boolean to listMemoriesKey and append it as the LAST array element (index 9). Do not reorder or insert anywhere else: indexes 1–5 stay scope/categories/visibility/limit/offset, and mutations/memory.ts reads key[3].
    - The parameter is required, not defaulted, so every caller states its cross-spine intent explicitly. This mirrors the server's D-04 rule that cross_spine is never inferred.
    - In ui/src/routes/observe/+page.svelte, pass false as the new last argument to listMemoriesKey. Observe's request and its `enabled: !!pp.scope` gate are unchanged.
    - In ui/src/routes/+page.svelte, change recentQ's queryKey to listMemoriesKey('', [], '', PAGE_LIMIT, 0, false, false, false, true), importing listMemoriesKey from $lib/queries alongside PAGE_LIMIT. Add crossSpine: true to the engram.listMemories request object; keep scope '' and the other fields as they are.
    - Update the one-line comment above recentQ to say it is a cross-spine recent-activity feed across every readable scope (cross_spine is explicit per D-04; see #500).
    - Do NOT touch any internal/server file. The server's rejection of an empty scope without cross_spine is correct by design and pinned by TestConnectCrossSpineNotInferred.
    - Re-run both verify commands to green, then run the full UI suite with pnpm -C ui test.

    Vendored SPA (memory fdve9fqqy1):
    - Run task ui:build from the repo root. It regenerates internal/webauth/static/ with rm -rf plus a copy, so hashed chunk names change and old ones are deleted.
    - Stage everything with git add -A -- internal/webauth/static ui/src/lib/queries.ts ui/src/lib/queries.test.ts ui/src/routes/+page.svelte ui/src/routes/observe/+page.svelte ui/src/routes/page.browser.test.ts.
    - Run task ui:build a SECOND time and confirm git status --porcelain -- internal/webauth/static is empty relative to the index. This is the same determinism the CI drift job checks.

    Commit (explicit pathspec, never a gsd commit verb): run git commit -F with a message file, followed by the same pathspec list as the git add above.
    - Subject: fix(ui): send cross_spine for root-route recent memories
    - Body: one paragraph. The root route's Recent memories query sent an empty scope without cross_spine and was rejected invalid_argument by design (D-04). It now requests a cross-spine recent-activity feed. The listMemoriesKey helper gains an explicit trailing crossSpine slot. The vendored SPA is regenerated. Refs #500.
    - Then the line: Quick task: 260918-idl
    - Final paragraph: the same two trailers as Task 1.
  </action>
  <verify>
    <automated>pnpm -C ui exec vitest run --project browser src/routes/page.browser.test.ts &amp;&amp; pnpm -C ui exec vitest run --project node src/lib/queries.test.ts</automated>
  </verify>
  <acceptance_criteria>
    - Both verify commands exit 0, and the browser run lists the new cross-spine test(s) as passed, not skipped.
    - `pnpm -C ui test` exits 0 (both projects).
    - `rg -o 'crossSpine: true' ui/src/routes/+page.svelte | wc -l` prints at least 1, and `rg -o 'listMemoriesKey\(' ui/src/routes/+page.svelte | wc -l` prints 1.
    - The inline 6-element key is gone: `rg -o "queryKey: \['listMemories'" ui/src/routes/+page.svelte | wc -l` prints 0.
    - The regenerated bundle carries the fix: `rg -l 'crossSpine' internal/webauth/static | wc -l` prints at least 1.
    - After the commit, `task ui:build && test -z "$(git status --porcelain -- internal/webauth/static ui/)"` exits 0. This is the CI `ui vendored-asset drift` equivalent.
    - `git diff --quiet c640c3a8 -- internal/server ui/package.json ui/pnpm-lock.yaml go.mod go.sum` exits 0 (c640c3a8 is this branch's base): no server change and no dependency change.
    - The SUMMARY records the RED assertion output from before the +page.svelte change.
  </acceptance_criteria>
  <done>The root route requests a cross-spine recent feed through listMemoriesKey, with crossSpine at key index 9. Browser and node Vitest pin the request args and key shape. The vendored SPA is regenerated and deterministic, and one fix(ui) commit references #500.</done>
</task>

<task type="auto">
  <name>Task 3: e2e browser test asserts the root route renders the fixture via Recent memories, with no failed ListScopes/ListMemories RPC</name>
  <files>internal/e2e/console_browser_test.go</files>
  <precondition>Docker is running (the e2e TestMain boots Qdrant), and Chrome exists at /Applications/Google Chrome.app or ENGRAM_CHROME_PATH is set. Task 2's regenerated internal/webauth/static is committed, because the e2e binary embeds it at build time.</precondition>
  <action>
    Edit internal/e2e/console_browser_test.go only.

    1. Add a helper, rootRoutePollExpr(marker string) string, built the same way markerPollExpr JSON-encodes the marker. The JS expression reads document.body.innerText once and returns true only when the text includes the marker AND no longer includes the root route's scopes loading text ("loading scopes"). This means both root-route queries have settled before the test moves on, so the next navigation cannot abort an in-flight ListScopes. Add a doc comment: the marker can only reach the root page through the Recent memories panel's cross-spine ListMemories, because the scope tiles render scope names and counts, never a record's summary.

    2. Add a failedRPCs map[string]string field to browserObserver and initialize it in newBrowserObserver. In attach's EventResponseReceived handler, in the Status >= 400 branch only (never from EventLoadingFailed, so a navigation-aborted request cannot false-fail), also record the URL and "HTTP <status>" into failedRPCs when the URL contains engramv1connect.EngramServiceListScopesProcedure or engramv1connect.EngramServiceListMemoriesProcedure. Keep recording into failedURLs exactly as today. In assertClean, add a check that fails the test if failedRPCs is non-empty, naming each URL and status. The existing asset, exception, and non-vacuity checks stay unchanged.

    3. In TestConsoleBundleRendersRecordInBrowser:
       - Raise the run context from 90*time.Second to 150*time.Second, so the three sequential 45-second polls each keep their own timeout as the binding one and every failure names the right stage.
       - After the hydration poll succeeds, and BEFORE navigating to observeURL, run a chromedp.Poll of rootRoutePollExpr(marker) on the same page with a 45-second timeout and 200 ms interval. On error, call logConsoleDiagnostics and t.Fatalf with a message naming the root-route Recent memories render (cross_spine ListMemories, #500). If it returns without error but false, t.Fatal.
       - Then evaluate document.body.innerText into a string and t.Fatalf if it contains "failed to load", quoting the body. That substring covers both the scopes error and MemoryList's error.
       - Keep the existing observe navigation and marker poll unchanged. It remains the scoped round trip through the link the scope tile navigates to. obs.assertClean(t) and sweepConsoleAssets stay at the end.

    4. Update the stale workaround prose so no comment still claims the root route is broken or that a fix is out of scope:
       - The fixtureScope doc (around lines 46-49): it is now the scope the seed record is written under, the record the root route's Recent memories panel must render, and the scope the test navigates to via /ui/observe?scope=.
       - The markerPollExpr doc (around 311-321): drop its closing sentence, the one claiming 05-04 left the vendored bundle alone. That claim stops being true once Task 2 regenerates the bundle. Keep the no-data-testid point.
       - The TestConsoleBundleRendersRecordInBrowser doc (around 336-356): it now says route 1 (/ui/) proves hydration AND the round trip through the cross-spine Recent memories feed, which was fixed in #500 (it previously sent an empty scope without cross_spine and was rejected by D-04). Route 2 (/ui/observe?scope=) proves the scoped round trip through the same link a scope tile navigates to. Remove the whole paragraph that justified route 2 as a workaround for the root-route defect.
       - Keep all other comments intact.

    5. Prove the new assertions are non-vacuous (RED) against the pre-fix vendored bundle, without committing anything:
       - Run git restore --source=c640c3a8 --worktree -- internal/webauth/static. This is no-overlay mode: it deletes the new hashed chunks and restores the old bundle, while the index keeps Task 2's tree.
       - Run the Task 3 verify command and confirm it FAILS at the new root-route stage. The expected failure is the root-route poll timing out, with diagnostics showing "failed to load — retry from the toolbar".
       - Run git restore --source=HEAD --worktree -- internal/webauth/static and confirm git status --porcelain -- internal/webauth/static prints nothing.
       - Re-run the verify command to green.
       - Keep the RED failure line and the green PASS line for the SUMMARY.
       - If Chrome or Docker is unavailable, say so plainly in the SUMMARY and do NOT claim the test passed. A SKIP is not a pass, which is why the verify command sets ENGRAM_REQUIRE_BROWSER=1 and ENGRAM_REQUIRE_QDRANT=1.

    Commit (explicit pathspec, never a gsd commit verb): run git add -- internal/e2e/console_browser_test.go, then git commit -F with a message file, followed by the same path.
    - Subject: test(e2e): assert console root route renders recent memories
    - Body: one paragraph. The root route now proves the round trip through the cross-spine Recent memories feed, and a failed ListScopes/ListMemories RPC fails the test. The workaround prose from 05-04 is retired. Refs #500.
    - Then the line: Quick task: 260918-idl
    - Final paragraph: the same two trailers as Task 1.
  </action>
  <verify>
    <automated>ENGRAM_REQUIRE_QDRANT=1 ENGRAM_REQUIRE_BROWSER=1 go test ./internal/e2e/ -run '^TestConsoleBundleRendersRecordInBrowser$' -count=1 -v</automated>
  </verify>
  <acceptance_criteria>
    - The verify output contains `--- PASS: TestConsoleBundleRendersRecordInBrowser` and no `--- SKIP`.
    - The SUMMARY records the RED run against the base-commit (`c640c3a8`) vendored bundle: the command plus the failure line naming the root-route stage. After the restore, `test -z "$(git status --porcelain -- internal/webauth/static)"` exited 0.
    - `rg -o 'EngramServiceListMemoriesProcedure|EngramServiceListScopesProcedure' internal/e2e/console_browser_test.go | wc -l` prints at least 2, and `rg -o 'rootRoutePollExpr' internal/e2e/console_browser_test.go | wc -l` prints at least 2 (definition plus use).
    - The stale workaround prose is gone: `rg -o 'currently-shipped console bug|deliberately untouched by this plan' internal/e2e/console_browser_test.go | wc -l` prints 0.
    - `go vet ./internal/e2e/` exits 0, and `golangci-lint run ./internal/e2e/...` reports no issues.
    - `git show --stat HEAD` lists exactly internal/e2e/console_browser_test.go, and the message carries `Quick task: 260918-idl` plus both trailers.
  </acceptance_criteria>
  <done>A real headless Chrome against the real binary renders the seeded marker on /ui/ through Recent memories, and a 4xx/5xx on ListScopes or ListMemories fails the test. The test is proven RED against the pre-fix bundle, the workaround comments are retired, and one test(e2e) commit exists.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| browser → Connect API (`/engram.v1.EngramService/*`) | Cookie-authenticated console session. Request fields (`scope`, `cross_spine`) are client-controlled |
| engram server → Qdrant gRPC | The server-built Scroll request (filter plus payload selector). Response size is driven by stored data volume |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-260918-01 | Denial of Service | `Store.ListScopes` Scroll response size | high | mitigate | Task 1: a scope-only payload selector bounds one response to at most scanCap (1000) scope strings regardless of content/summary/citation volume. TestListScopesFullPayloadsOverGRPCLimit pins it against real Qdrant with 5 MiB of full payload, and the receive limit is not raised |
| T-260918-02 | Information Disclosure | root-route `ListMemories` with `cross_spine=true` | medium | mitigate | Authz stays in `internal/store` (`listFilter`/`ownerOrSharedCondition`). cross_spine only removes the scope clause and never the owner/shared clause, as pinned by the existing TestCrossSpineAuthzIsolation and TestListCrossSpine. Task 2 changes only which of the caller's readable scopes are listed, and the final gate re-runs `go test ./...` |
| T-260918-03 | Information Disclosure | ListScopes payload selector | low | accept | A narrower selector returns strictly less data to the server process and changes nothing on the wire to the client (ListScopesResponse is scope+count only) |
| T-260918-04 | Tampering | D-04 explicit-cross_spine contract | low | mitigate | No `internal/server` file changes (the Task 2 acceptance runs `git diff --quiet` over `internal/server`). TestConnectCrossSpineNotInferred runs in the final `task` gate |
| T-260918-SC | Tampering | npm/pnpm/go installs | low | accept | No new dependencies. `task ui:build` runs `pnpm install --frozen-lockfile` against the existing lockfile, and acceptance asserts `ui/package.json`, `ui/pnpm-lock.yaml`, `go.mod`, and `go.sum` are unchanged. `playwright install chromium`, if needed, fetches browser binaries for an already-pinned devDependency |
</threat_model>

<verification>
Run from the repo root on branch `fix/console-root-route`, after all three commits:

1. `task lint`: clean (golangci-lint, yamlfmt, actionlint, rumdl).
2. `ENGRAM_REQUIRE_QDRANT=1 ENGRAM_REQUIRE_BROWSER=1 task test`: green. This includes `go test ./...` (store, server incl. TestConnectCrossSpineNotInferred and TestListCrossSpine, e2e incl. TestConsoleBundleRendersRecordInBrowser) and the python hook tests. Report any pre-existing failure separately, with the exact command and error, never folded into this change.
3. `task license:check`: clean (no new files; existing SPDX headers intact; nothing added under `.planning/**`).
4. `pnpm -C ui test`: green (node + browser projects).
5. `task ui:build && test -z "$(git status --porcelain -- internal/webauth/static ui/)"` exits 0.
6. `git log --oneline c640c3a8..HEAD` shows exactly three commits, in order fix(store), fix(ui), test(e2e), each with `Quick task: 260918-idl` in its body. `git branch --show-current` prints `fix/console-root-route`.
7. No edits to `.planning/WINDOWS.md` or `.planning/ROADMAP.md` (`git diff --quiet c640c3a8 -- .planning/WINDOWS.md .planning/ROADMAP.md` exits 0), and no executor edits to `.planning/STATE.md`, which the orchestrator owns. WINDOWS.md entry id 4 (the #500 workaround) is noted in the SUMMARY for the orchestrator to close, not edited here.
</verification>

<success_criteria>
- The live defect chain is closed at both tiers. ListScopes no longer requests payload it never reads, so neither the console scope tiles nor any cross_spine recall can overflow the 4 MiB gRPC receive limit. The root route's Recent memories panel sends cross_spine and renders a cross-scope recent feed.
- Each fix has a regression test that was observed failing on the old code and passing on the new: the store test with ResourceExhausted, the browser Vitest with a missing crossSpine, and the e2e with the root-route poll timing out against the base-commit (c640c3a8) bundle.
- Three atomic Conventional Commits on `fix/console-root-route`, with explicit pathspecs and required trailers. The vendored SPA is in sync, and `task` is green.
</success_criteria>

<output>
Create `.planning/quick/260918-idl-fix-console-root-route-listscopes-grpc-4/260918-idl-SUMMARY.md` with `status: complete` in its frontmatter. Record each task's RED evidence (command plus failing line) and green re-run, the three commit hashes, whether the e2e browser test actually ran (never report a SKIP as a pass), any pre-existing failures kept separate, and the note that WINDOWS.md id 4 and GH #500 are ready for the orchestrator/PR to close. Do NOT commit the SUMMARY or PLAN; the orchestrator handles the docs commit.
</output>
