---
phase: 02-headless-cli-client
plan: 02
subsystem: api
tags: [cobra, connect-go, cli, exit-codes, protojson]

# Dependency graph
requires:
  - phase: 02-headless-cli-client plan 01 (engram search tracer)
    provides: "the shared client foundation (clientFromFlags, resolveOutputFormat,
      exitCodeForConnectErr, renderJSON, renderMemoryTable, cliError/usageErrorf/wrapRPCError)
      and the clienttest_test.go real-Connect-handler harness (stubEngramService,
      startStubServer, resetClientFlags, runClient)"
provides:
  - "cmd/engram/client_list.go: `engram list` — paged, filtered recall binding all eleven
    ListMemoriesRequest fields (minus the deprecated approximate response field) to flags"
  - "cmd/engram/client_store.go: `engram store` — the phase's only write verb, exactly one
    StoreMemory attempt per invocation, never retried on any error class"
affects: [02-03 (self-describe catalog must enumerate list and store alongside search)]

# Actuals (#2632)
actuals:
  tokens: 5866
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Both new commands follow client_search.go's RunE shape byte-for-byte: validate ->
      resolve format -> clientFromFlags -> one RPC call -> wrapRPCError -> render. Neither
      needed a new client_common.go helper, confirming Plan 01's tracer was thick enough."
    - "store's semantic pre-check (empty --content/--scope) duplicates the wire's
      buf.validate min_len=1 constraint deliberately as a no-round-trip ergonomic gate; the
      server's rejection remains authoritative and maps to the same exit code (2) either way."

key-files:
  created:
    - cmd/engram/client_list.go
    - cmd/engram/client_list_test.go
    - cmd/engram/client_store.go
    - cmd/engram/client_store_test.go
  modified: []

key-decisions:
  - "list's text-output footer (total, next_page_token) is written to stdout alongside the
    table, not stderr — it is data a paging caller consumes, not a diagnostic (D-07)."
  - "No client-side enforcement of cursor_mode/offset mutual exclusivity in list, or of the
    category enum in store: the server owns both rules via CodeInvalidArgument, which the
    shared exitCodeForConnectErr mapper already turns into exit 2. Duplicating either
    client-side would create two places for the rule to drift."

patterns-established: []

requirements-completed: [REQ-cli-client-commands, REQ-cli-agent-output]

coverage:
  - id: D1
    description: "engram list completes against a real in-process Connect server, writing one
      JSON object carrying memories, the exact total, and next_page_token to stdout, exit 0"
    requirement: "REQ-cli-client-commands"
    verification:
      - kind: integration
        ref: "cmd/engram/client_list_test.go#TestClientListEndToEndJSON"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every ListMemoriesRequest filter flag (scope, limit, offset, categories,
      visibility, tags, full, created-after, created-before, page-token, cursor-mode) reaches
      its corresponding wire field"
    requirement: "REQ-cli-client-commands"
    verification:
      - kind: integration
        ref: "cmd/engram/client_list_test.go#TestClientListPassesFiltersToRequest"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_list_test.go#TestClientListCursorModeReachesRequest"
        status: pass
    human_judgment: false
  - id: D3
    description: "An empty list result exits 0 and emits memories:[] — never null"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: integration
        ref: "cmd/engram/client_list_test.go#TestClientListEmptyResultIsEmptyArray"
        status: pass
    human_judgment: false
  - id: D4
    description: "list exit codes: Unauthenticated->3, NotFound->4, InvalidArgument->2;
      missing --server/ENGRAM_SERVER_URL->2 with zero calls attempted"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: integration
        ref: "cmd/engram/client_list_test.go#TestClientListExitCodes"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_list_test.go#TestClientListMissingServerURLIsUsageError"
        status: pass
    human_judgment: false
  - id: D5
    description: "list exposes no flag or text-output column for the deprecated approximate
      response field"
    requirement: "REQ-cli-client-commands"
    verification:
      - kind: unit
        ref: "cmd/engram/client_list_test.go#TestClientListNoDeprecatedApproximateFlag"
        status: pass
    human_judgment: false
  - id: D6
    description: "list's --output=text path renders a human table, not JSON, containing the
      returned short_id"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: integration
        ref: "cmd/engram/client_list_test.go#TestClientListTextOutput"
        status: pass
    human_judgment: false
  - id: D7
    description: "engram store writes a memory through a real in-process Connect server and
      returns id + short_id as one JSON object, exit 0"
    requirement: "REQ-cli-client-commands"
    verification:
      - kind: integration
        ref: "cmd/engram/client_store_test.go#TestClientStoreEndToEndJSON"
        status: pass
    human_judgment: false
  - id: D8
    description: "Every StoreMemoryRequest field (content, scope, source, category, tags,
      repo, workspace, worktree, base-dir, summary) reaches its corresponding wire field"
    requirement: "REQ-cli-client-commands"
    verification:
      - kind: integration
        ref: "cmd/engram/client_store_test.go#TestClientStorePassesFieldsToRequest"
        status: pass
    human_judgment: false
  - id: D9
    description: "An empty --content or --scope is rejected locally with exit 2 and zero
      network calls, before any RPC is attempted"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: integration
        ref: "cmd/engram/client_store_test.go#TestClientStoreRequiresContentAndScope"
        status: pass
    human_judgment: false
  - id: D10
    description: "A failed StoreMemory call is attempted exactly once and never retried, for
      Unavailable, Internal, and DeadlineExceeded — the three error classes a well-meaning
      retry would target"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: integration
        ref: "cmd/engram/client_store_test.go#TestClientStoreNeverRetries"
        status: pass
    human_judgment: false
  - id: D11
    description: "store declares no flag for actor, owner, or any response-only field (id,
      short_id, score, created_at, access_count, last_accessed_at)"
    requirement: "REQ-cli-client-commands"
    verification:
      - kind: unit
        ref: "cmd/engram/client_store_test.go#TestClientStoreNoActorOrOwnerFlag"
        status: pass
    human_judgment: false
  - id: D12
    description: "store's --category help text names all four accepted wire values"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: unit
        ref: "cmd/engram/client_store_test.go#TestClientStoreCategoryHelpNamesLegalValues"
        status: pass
    human_judgment: false
  - id: D13
    description: "store exit codes: InvalidArgument->2, Unauthenticated->3,
      PermissionDenied->3"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: integration
        ref: "cmd/engram/client_store_test.go#TestClientStoreExitCodes"
        status: pass
    human_judgment: false
  - id: D14
    description: "store's --output=text path renders plain text (not JSON) containing the
      returned short id"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: integration
        ref: "cmd/engram/client_store_test.go#TestClientStoreTextOutput"
        status: pass
    human_judgment: false
  - id: D15
    description: "Both new commands build their client through the single shared
      clientFromFlags constructor and classify errors through the single shared
      exitCodeForConnectErr mapper — no second constructor or mapper exists"
    requirement: "REQ-cli-client-commands"
    verification:
      - kind: other
        ref: "grep -c 'clientFromFlags(cmd)' client_list.go == 1; grep -v NewEngramServiceClient
          client_list.go == 0; grep -c 'StoreMemory(' client_store.go == 1"
        status: pass
    human_judgment: false

duration: ~15min
completed: 2026-08-01
status: complete
---

# Phase 2 Plan 2: engram list + engram store Summary

**`engram list` (all eleven `ListMemoriesRequest` fields bound to flags, minus the deprecated
`approximate` response field) and `engram store` (the phase's only write verb, proven to attempt
exactly one `StoreMemory` call and never retry across three ambiguous-failure error classes) join
`engram search` as the complete three-verb CLI surface, both built entirely on Plan 01's
`clientFromFlags`/`exitCodeForConnectErr`/`renderJSON`/`renderMemoryTable` foundation with zero new
helpers added to `client_common.go`.**

## Performance

- **Duration:** ~15 min
- **Tasks:** 2/2 completed
- **Files modified:** 4 (all created)
- **Commits:** 2 task commits + this metadata commit

## Accomplishments
- `engram list` completes against a real in-process Connect server: every filter flag (`--scope`,
  `--limit`, `--offset`, `--categories`, `--visibility`, `--tags`, `--full`, `--created-after`,
  `--created-before`, `--page-token`, `--cursor-mode`) is proven to reach its `ListMemoriesRequest`
  field by name, individually asserted (not spot-checked). No flag or text-table column exists for
  the proto's `[deprecated = true]` `approximate` response field.
- `engram store` completes against a real in-process Connect server, returning `id` and `short_id`
  as one JSON object. Every `StoreMemoryRequest` field is bound to a flag; no flag exists for
  `actor`, `owner`, or any response-only field (`id`, `short_id`, `score`, `created_at`,
  `access_count`, `last_accessed_at`) — proven by a structural lookup test that fails loudly if a
  future contributor adds one back.
- `TestClientStoreNeverRetries` proves a failed write is attempted **exactly once**, for
  `CodeUnavailable`, `CodeInternal`, and `CodeDeadlineExceeded` — the three error classes a
  well-meaning retry would target. The stub's `storeCalls` counter increments in
  `stubEngramService.StoreMemory`'s method body (verified by reading `clienttest_test.go` before
  writing the test, per the plan's explicit trap warning), so it genuinely observes a retry rather
  than only firing on the success path.
- `store` rejects an empty `--content` or `--scope` locally with exit 2 and zero network calls,
  before building the client — a no-round-trip ergonomic pre-check that deliberately duplicates the
  wire's `buf.validate.field.string.min_len = 1` constraint; the server's own rejection remains
  authoritative and maps to the same exit code either way.
- Both commands were written from `client_search.go`'s exact structure (`Args: cobra.NoArgs`, the
  validate → resolve-format → build-client → one-call → wrap-error → render `RunE` ordering, an
  `init()` calling `addClientFlags` then `rootCmd.AddCommand`). Neither needed a new
  `client_common.go` helper — the tracer's foundation was thick enough, exactly as the plan's
  objective predicted.

## Task Commits

Each task was committed atomically:

1. **Task 1: `engram list` — paged, filtered recall over the proven client** - `184fc1d9` (feat, tdd)
2. **Task 2: `engram store` — one write, one attempt, never retried** - `807a03ec` (feat, tdd)

**Plan metadata:** committed alongside this SUMMARY (see final commit below)

_Note: both tasks were TDD. Each command's full test suite was observed RED against a temporary
`return nil` stub inserted as the first line of `RunE` (all behavior-asserting tests fail; the two
pure-structural tests — `TestClientListNoDeprecatedApproximateFlag`'s flag lookup and
`TestClientStoreNoActorOrOwnerFlag`/`TestClientStoreCategoryHelpNamesLegalValues`, which only
inspect the registered flag set — correctly stayed green, since `init()` still runs). The stub was
reverted immediately after each RED observation, confirmed via a second full-suite run before the
task's commit._

## Files Created/Modified
- `cmd/engram/client_list.go` - `engram list`: eleven flags mirror `ListMemoriesRequest` 1:1 (minus
  the deprecated `approximate` field), `RunE` issues one `ListMemories` call, text output adds a
  stdout data footer (total, next_page_token) after the unranked (no-score) table
- `cmd/engram/client_list_test.go` - 8 integration/unit tests: end-to-end JSON, full ten-field
  filter round-trip (plus a dedicated cursor-mode test), empty-result `[]`, three exit codes,
  missing-server-URL usage error, the no-deprecated-flag structural guard (flag lookup + text-header
  content), text output
- `cmd/engram/client_store.go` - `engram store`: ten flags mirror `StoreMemoryRequest` 1:1, no flag
  for `actor`/`owner`/response-only fields, local pre-check rejects empty `--content`/`--scope`
  before any network call, `RunE` issues exactly one `StoreMemory` call with no retry on any error
  class
- `cmd/engram/client_store_test.go` - 9 integration/unit tests: end-to-end JSON, full ten-field
  round-trip, empty-content/empty-scope local rejection, the never-retries test across three error
  codes, the no-actor/owner/response-field structural guard, `--category` help-text content, three
  exit codes, text output

## Decisions Made
- `list`'s text-output footer (total count, and next page token when non-empty) is written to
  stdout immediately after the table, not to stderr — it is data a paging caller consumes to decide
  whether to request another page, not a diagnostic (D-07's stdout/stderr split is about data vs.
  diagnostics, not about "table vs. everything else").
- Neither command duplicates a server-owned validation rule client-side: `list` does not enforce
  `cursor_mode`/`offset` mutual exclusivity, and `store` does not enforce the `category` enum
  beyond naming its legal values in `--help` text. Both rules already map to `CodeInvalidArgument` →
  exit 2 through the shared `exitCodeForConnectErr` mapper; adding a second client-side check would
  create two places for the rule to drift, which the plan's action text explicitly warned against.

## Deviations from Plan

None — plan executed exactly as written. One test-authoring correction, folded into Task 1's
development before its commit (not a deviation from the plan's design, since it was purely a defect
in the test I was writing, not in the plan or in `client_list.go`):

**1. [Rule 1 - Test bug] `TestClientListEndToEndJSON`'s `total` field was typed `uint64` instead of
`string`**
- **Found during:** Task 1, first GREEN run after reverting the RED stub
- **Issue:** `protojson`'s default marshaling renders a `uint64` field as a JSON string (e.g.
  `"total":"42"`), not a JSON number — a documented protojson behavior (large `uint64` values can
  exceed JSON's safe integer range). The test's anonymous struct declared `Total uint64
  \`json:"total"\`` and `json.Unmarshal` correctly rejected the string-typed wire value.
- **Fix:** Changed the field to `Total string \`json:"total"\`` and compared against the literal
  `"42"`, with a comment explaining why.
- **Files modified:** `cmd/engram/client_list_test.go`
- **Verification:** `go test ./cmd/engram/... -run TestClientList -v` — all 8 tests PASS
- **Committed in:** `184fc1d9` (part of Task 1's commit — the file had not been committed yet)

---

**Total deviations:** 0 (one same-task test-authoring fix folded into its originating commit; not
tracked as a separate deviation since it changed neither the plan's design nor `client_list.go`'s
implementation)

## Issues Encountered

None. `ssh-add -T /Users/sean/.ssh/seanb4t_ed25519.pub` confirmed the 1Password SSH-signing agent
was live before either commit; both commits carry a real SSH signature (`git cat-file commit`
shows a `gpgsig` block on each).

## User Setup Required

None — no external service configuration required. All automated verification in this plan runs
against an in-process `httptest` server wrapping the real generated Connect handler
(`clienttest_test.go`'s `startStubServer`/`stubEngramService`, inherited unchanged from Plan 01). A
reachable engram server with the Connect lane mounted is required only for real-world manual use of
`engram list`/`engram store`.

## Next Phase Readiness

`engram search`, `engram list`, and `engram store` are all complete and satisfy
`REQ-cli-client-commands` and `REQ-cli-agent-output` jointly (both marked complete in
REQUIREMENTS.md by this plan; `REQ-cli-credential-safety` was already complete from Plan 01;
`REQ-cli-self-describing` remains for Plan 03). Plan 03's bare-invocation self-describe catalog can
now enumerate all three subcommands and their full flag/exit-code surface — nothing in this plan
changed `cmd/engram/root.go`, `cmd/engram/client_common.go`, or `cmd/engram/clienttest_test.go`
(all three verified byte-identical via `git diff --exit-code`), so Plan 03 inherits exactly the
foundation Plan 01 left it. No blockers.

---
*Phase: 02-headless-cli-client*
*Completed: 2026-08-01*

## Self-Check: PASSED

All 5 claimed files verified present on disk; both task commits (`184fc1d9`, `807a03ec`)
verified present in `git log --oneline --all`.
