---
phase: 04-diagnosability
plan: 04
subsystem: api
tags: [go, mcp, connect-rpc, error-handling, validation, auth]

requires:
  - phase: 04-diagnosability
    provides: "04-01's argError envelope (argErrf/argErrFieldsf, HintCode vocabulary, argClass table) and its connectError *argError dispatch ordering"
provides:
  - "Every single-field and relational rejection reachable from internal/server/tools.go converted to the argError envelope (RESEARCH sweep rows 6-21, 32-34, plus three parseWindow sub-checks the inventory did not separately number)"
  - "internal/auth/bearer401_test.go::TestMCP401BodyByteIdentical — the D-08 scope-fence pin, byte-identical 401 body + WWW-Authenticate header"
  - "internal/server/argattribution_test.go::TestValidationErrorAttributionMatrix — the criterion-2 matrix, 23 named subtests"
affects: [04-05, 04-06]

actuals:
  tokens: 9200
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Zero-value &deps{}/caller{} test doubles for rejection-only unit tests: any argErrf-returning branch that returns BEFORE the first d.st/d.em call can be exercised without a live Qdrant, confirmed per-function by reading control flow rather than assumed"
    - "Region-scoped, comment-stripped awk|rg zero-count verify gates as a STRICTER scope authority than a plan's own row-numbered prose when the two disagree"

key-files:
  created:
    - internal/auth/bearer401_test.go
    - internal/server/argattribution_test.go
  modified:
    - internal/server/tools.go

key-decisions:
  - "D-08 scope fence VERIFIED by reading before any reformat landed: github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go's verify() (lines 99-140) returns the literal \"no bearer token\" at line 104 when the Authorization header does not parse as a well-formed bearer credential, written via http.Error(w, errmsg, code) at line 90 -- entirely inside the go-sdk, before any tools.go code runs. The fence rests on a true premise."
  - "Row 33 (parseWindow's discovery-not-schedulable check) classified single-field on category per the plan's explicit resolution of RESEARCH's borderline note, not relational -- the caller changes exactly one thing (the category) to fix it."
  - "Rows 12 and 34 dropped their manual `: %w` wrap of store.ErrInvalidArgument now that argError.Unwrap() supplies it automatically -- leaving both would have doubled the sentinel in the message text."
  - "D-12 (no value echo) closed the two remaining `got %q` tails (citation kind, listScheduled state) and every RFC3339 parse-failure's caller-value interpolation (listScheduled + the four inline MCP closure parses + parseWindow's own not_before/not_after parses): Detail now states the constant format/enum requirement, never the caller's rejected string."
  - "Deviation (Rule 3, blocking-gate): parseWindow's individual not_before/not_after time.Parse failures and its not-after-in-the-past check were converted too, beyond RESEARCH's three explicitly-numbered rows (32/33/34) -- the plan's own region-scoped verify gate requires ZERO bare fmt.Errorf across the WHOLE function, and RESEARCH's Full D-06 Sweep Inventory did not separately enumerate these three sub-checks. Documented as an inventory gap, not a scope decision worked around."

patterns-established:
  - "assertEnvelope(t, err, wantFields, wantHint) helper: non-nil-error-first guard, then full-SET equality (reflect.DeepEqual) on argFieldsOf and exact equality on argHintOf -- never membership, never message wording, per D-07's explicit instruction that criterion 2's matrix must catch a neighbouring-field misattribution"

requirements-completed: [REQ-validation-error-attribution, REQ-error-hint-envelope]

coverage:
  - id: D1
    description: "The MCP 401 auth body (produced by the go-sdk's RequireBearerToken, not tools.go) is byte-identical before and after the D-08 reformat, and the separation was verified by reading first"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: unit
        ref: "internal/auth/bearer401_test.go#TestMCP401BodyByteIdentical"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every single-field rejection reachable from tools.go names its field as machine-readable data and carries a hint code, proven by a matrix with one case per single-field-invalid input rather than by matching exact wording"
    requirement: "REQ-validation-error-attribution"
    verification:
      - kind: unit
        ref: "internal/server/argattribution_test.go#TestValidationErrorAttributionMatrix (23 subtests, >=21 floor)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The two relational parseWindow checks (both-bounds-absent, ordering) carry BOTH field names, not an arbitrary single pick"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: unit
        ref: "internal/server/argattribution_test.go#TestValidationErrorAttributionMatrix/window_both_bounds_absent, /window_ordering_violation"
        status: pass
    human_judgment: false
  - id: D4
    description: "No converted rejection echoes the caller's rejected value, proven by an absence gate observed firing"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: unit
        ref: "internal/server/argattribution_test.go#TestHintNeverEchoesValue (7 subtests, RED transcript recorded)"
        status: pass
      - kind: static
        ref: "region-scoped whole-file `got %q` zero-count verify gate"
        status: pass
    human_judgment: false
  - id: D5
    description: "validateCitations's six rejections do not map to Connect CodeInternal (D-11a, the second of three closed defects)"
    requirement: "REQ-validation-error-attribution"
    verification:
      - kind: unit
        ref: "internal/server/argattribution_test.go#TestCitationValidationIsNotCodeInternal"
        status: pass
    human_judgment: false

duration: ~25min
completed: 2026-08-01
status: complete
---

# Phase 4 Plan 04: The `tools.go` Sweep + the Criterion-2 Attribution Matrix Summary

**Every remaining single-field and relational rejection reachable from `internal/server/tools.go` (RESEARCH's sixteen Category A sites, three Category B relational sites, plus three unnumbered `parseWindow` sub-checks the plan's own verify gate forced into scope) now carries a machine-readable field attribution and remediation hint, proven by a 23-row matrix and a D-12 value-echo absence gate — with the MCP 401 auth body verified separate and pinned byte-for-byte before any reformat landed.**

## Performance

- **Duration:** ~25 min
- **Tasks:** 3
- **Files modified:** 3 (2 created, 1 modified)
- **Commits:** 3

## The 401 Scope Fence — Proof (Task 1, run first, no production change)

Read `github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go` this session and confirmed, by
reading rather than by carrying RESEARCH's claim forward unverified:

1. `verify()` (lines 99-140) returns the literal `"no bearer token"` at **line 104** when the
   `Authorization` header does not parse as exactly two fields with a case-insensitive `"bearer"`
   scheme.
2. `RequireBearerToken`'s returned handler (lines 72-96) writes that string via
   `http.Error(w, errmsg, code)` at **line 90** — before any `internal/server` or `internal/auth`
   code executes for that request.
3. Nothing in `internal/server/tools.go` contributes to this specific body.

All three premises held. `internal/auth/bearer401_test.go::TestMCP401BodyByteIdentical` pins the
body and the `WWW-Authenticate` header for a missing header, a non-bearer scheme (`Basic ...`), and
a bare `"Bearer"` with no credential — full-string `==` throughout, zero `strings.Contains` (gated
by a comment-stripped zero-count check). The pinned values were captured by first asserting a
deliberately-wrong body (`"WRONG-EXPECTED-VALUE"`), running the test, reading the actual value
(`"no bearer token"`, all three cases) out of the failure output, then restoring the correct
literal — not hand-written from the source read alone.

No production file changed in this task.

## The Sweep (Task 2)

Converted every remaining rejection in `internal/server/tools.go`:

| Site | Rows | Field(s) | Class | Hint |
|---|---|---|---|---|
| `validateCitations` (6 branches) | 6-11 | `citations`, `citations[i].kind`, `citations[i].ref`, `citations[i].excerpt` | malformed/out_of_range | required, too_many, enum, required, too_long |
| `checkIdempotentReplay` | 12 | `idempotency_key` | out_of_range | too_long |
| `listScheduled` (3 branches) | 13-15 | `created_after`, `created_before`, `state` | malformed | format, format, enum |
| `effectiveDiscoveryScope` | 16 | `scope` | malformed | conditional_required |
| `effectiveSearchScope` (4 call sites: `listMemory`, `searchMemory`, 2 Connect handlers) | 17 | `scope` | malformed | conditional_required |
| 4 inline MCP closure window parses (`search_memory`/`list_memory`) | 18-21 | `created_after`, `created_before` | malformed | format |
| `parseWindow` both-bounds-absent | 32 | `not_before`, `not_after` | malformed | required |
| `parseWindow` discovery-not-schedulable | 33 | `category` | precondition | not_applicable |
| `parseWindow` ordering violation | 34 | `not_before`, `not_after` | precondition | ordering |
| `parseWindow` not_before/not_after `time.Parse` failures, not-after-in-the-past *(unnumbered, converted anyway — see Deviation below)* | — | `not_before`, `not_after`, `not_after` | malformed/out_of_range | format, format, ordering |

**Deviation (Rule 3 — blocking gate, not a scope choice):** `parseWindow`'s own verify gate is
`awk '/^func parseWindow\(/,/^}/' | rg -v '^\s*//' | rg -c 'fmt\.Errorf'` — region-scoped to the
**whole function**, not to rows 32-34 alone. RESEARCH's Full D-06 Sweep Inventory never separately
numbered the individual `not_before`/`not_after` parse failures or the not-after-in-the-future
check, but they live inside `parseWindow` and the gate cannot pass while any of them remains a bare
`fmt.Errorf`. Converted all three (single-field: `not_before`/format, `not_after`/format,
`not_after`/ordering) rather than narrowing the sweep to match the row-numbered prose. This is an
inventory gap in RESEARCH, not a premise this task worked around — documented here loudly per the
session's stated culture (two prior premises already correctly falsified this phase).

D-12 (no value echo) applied throughout: dropped the citation-kind and `listScheduled`-state
`got %q` tails (RESEARCH rows 9, 15), and every RFC3339 parse-failure's `%w`-wrapped
`time.Parse` error (which carried the caller's raw string) was replaced with the constant format
requirement. Rows 12 and 34 (and the newly-converted `not_before`/`not_after` parses) dropped their
manual `: %w` wrap of `store.ErrInvalidArgument` — `argError.Unwrap()` now supplies it, and keeping
both would have doubled the sentinel in the message.

No arg struct's `json`/`jsonschema` tag was touched (`git diff` confirms) — that is plan 04-06's
D-06a work.

## The Matrix, the Gate, the Pin (Task 3)

`internal/server/argattribution_test.go`:

- **`TestValidationErrorAttributionMatrix`** — **23 named subtests** (floor was `>=21`): 04-01's
  five `validateStoreDiscovery` rows plus 18 rows from this plan's directly-callable functions
  (`validateCitations` x6, `checkIdempotentReplay` x1, `listScheduled` x3, `effectiveDiscoveryScope`
  x1, `effectiveSearchScope` x1, `parseWindow` x6). Every assertion is full-**SET** equality
  (`reflect.DeepEqual`) on `argFieldsOf(err)` and exact equality on `argHintOf(err)` — never message
  wording. `assertEnvelope` asserts a non-nil error **first**, guarding the vacuous-row failure mode
  (a row whose input was actually accepted must fail loudly, not pass silently).

  **Coverage note on RESEARCH rows 18-21** (the four inline `created_after`/`created_before` parses
  inside the `search_memory`/`list_memory` MCP closures, `tools.go:1568-1574,1615-1621`): these are
  **not** independently driven through a full MCP client/server round trip in this matrix. They live
  inside `mcp.AddTool` closures reachable only via `Register()`, which builds its deps from
  environment-configured Qdrant/embedder state (`buildDepsFromEnv`) — there is no injectable test
  double, and standing up an `mcp.NewInMemoryTransports()`-based harness just to re-prove a rejection
  shape already byte-identical (same `argErrf(classMalformed, HintFormat, <field>, "<field> must be
  RFC3339")` call, confirmed by code review) to the tested `listScheduled` rows 13/14 was judged
  disproportionate. The matrix already clears the `>=21` floor (23) without them; the whole-file
  `got %q` zero-count gate and `go vet ./...` still cover those four sites unconditionally. This is
  recorded in the test's own doc comment, not just here.

- **`TestHintNeverEchoesValue`** (D-12 negative gate) — 7 sentinel-marker subtests (citation kind,
  `listScheduled` state, `listScheduled` created_after, `parseWindow` not_before, idempotency_key,
  discovery kind, discovery scope-prefix). **RED transcript captured this session:** the
  citation-kind check's `argErrf` call was temporarily reverted to append the pre-conversion
  `, got %q", i, c.Kind` tail; the subtest failed:

  ```
  === RUN   TestHintNeverEchoesValue
  === RUN   TestHintNeverEchoesValue/citation_kind_no_echo
      argattribution_test.go:246: err.Error() = "field=citations[i].kind hint=enum: citation 0: kind must be one of file|commit|url|repo, got \"SENTINEL-9f2xQ\"" contains forbidden marker "SENTINEL-9f2xQ"
  --- FAIL: TestHintNeverEchoesValue (0.00s)
      --- FAIL: TestHintNeverEchoesValue/citation_kind_no_echo (0.00s)
  FAIL
  ```

  The revert was undone; `git diff --exit-code -- internal/server/tools.go` confirmed zero net
  change. The subtest passes again in the committed tree.

- **`TestCitationValidationIsNotCodeInternal`** — pins the second of D-11a's three
  bare-unwrapped-error defects: `validateCitations`'s six rejections no longer map to
  `connect.CodeInternal` (`validateStoreRule` is 04-05's).

## Gates

| Gate | Result |
|---|---|
| `go vet ./...` (after every task) | clean |
| 4 region-scoped, comment-stripped `fmt.Errorf` zero-counts (`validateCitations`, `parseWindow`, `effectiveSearchScope`, `effectiveDiscoveryScope`) | 0/0/0/0 |
| whole-file comment-stripped `got %q` zero-count | 0 |
| `internal/auth/bearer401_test.go` `strings.Contains` zero-count | 0 |
| `TestMCP401BodyByteIdentical` | PASS (3 subtests) |
| `TestValidationErrorAttributionMatrix` | PASS (23 subtests, `-ge 21`) |
| `TestHintNeverEchoesValue` | PASS (7 subtests, RED recorded) |
| `TestCitationValidationIsNotCodeInternal` | PASS (6 subtests) |
| `go test ./internal/server/... -count=1` | ok |
| `go test ./internal/server/... -count=1 -shuffle=on` | ok |
| `task` (lint + full repo suite) | all green |
| `task license:check` | 0 invalid |
| `git diff --exit-code -- go.mod go.sum` | zero diff |

## Task Commits

1. **Task 1 — pin the MCP 401 body byte-for-byte before the D-08 reformat** — `fa3dfc98` (test)
2. **Task 2 — field-attribute every remaining tools.go argument rejection** — `c3e363c0` (feat!, `BREAKING CHANGE:` footer)
3. **Task 3 — matrix the field attribution, gate the value echo, pin the citations mapping** — `b8823908` (test)

## Files Created/Modified

- `internal/auth/bearer401_test.go` — new: `TestMCP401BodyByteIdentical`, 3 subtests, the D-08 fence
- `internal/server/argattribution_test.go` — new: the criterion-2 matrix, the D-12 negative gate, the citations CodeInternal pin
- `internal/server/tools.go` — 22 lines changed across `parseWindow`, `validateCitations`, `checkIdempotentReplay`, `listScheduled`, `effectiveDiscoveryScope`, `effectiveSearchScope`, and the two MCP closures (`search_memory`/`list_memory`)

## Decisions Made

See `key-decisions` in the frontmatter. The one deviation from the plan's row-numbered prose
(`parseWindow`'s three unnumbered sub-checks) is documented above and is a Rule 3 (auto-fix
blocking issue) response to the plan's own verify gate, not a scope expansion beyond what the gate
demanded.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `parseWindow`'s region-scoped verify gate covers three sub-checks RESEARCH's row inventory never numbered**
- **Found during:** Task 2
- **Issue:** The plan's action text says "Convert rows 6 through 21 (Category A) and 32 through 34
  (Category B)". `parseWindow`'s individual `not_before`/`not_after` `time.Parse` failures and its
  not-after-in-the-future check are none of those three numbers, but they are bare `fmt.Errorf`
  calls inside `parseWindow`, and the plan's own `<verify>` block requires the WHOLE function to
  contain zero bare `fmt.Errorf` (region-scoped `awk`, not row-scoped).
- **Fix:** Converted all three: `not_before` parse failure → `argErrf(classMalformed, HintFormat,
  "not_before", ...)`; `not_after` parse failure → same shape on `not_after`; not-after-in-the-past
  → `argErrf(classOutOfRange, HintOrdering, "not_after", "not_after must be in the future")` (drops
  the pre-existing `a.NotAfter` value echo in the same edit, closing a latent D-12 gap this
  conversion incidentally fixed).
- **Files modified:** `internal/server/tools.go`
- **Verification:** the region-scoped gate now returns 0; all three added as extra matrix rows (23
  total, above the 21 floor).
- **Committed in:** `c3e363c0` (Task 2)

**2. [Scope note, not a fix] RESEARCH rows 18-21 not independently driven through a full MCP round trip**
- **Found during:** Task 3
- **Issue:** The four inline `created_after`/`created_before` parses inside the `search_memory`/
  `list_memory` MCP closures are reachable only through `Register()`, which builds its deps from
  environment-configured Qdrant/embedder state — there is no injectable test double for a
  lightweight per-field unit test at that call depth.
- **Resolution:** Not converted into a test row. The matrix clears its `>=21` floor (23) without
  them; the conversion itself is covered by `go vet`, the region-adjacent code (they call the exact
  same `parseRFC3339` + `argErrf(classMalformed, HintFormat, ...)` shape as tested rows 13/14), and
  the whole-file value-echo gate. Documented in the matrix's own doc comment and here, not hidden.

---

**Total deviations:** 1 auto-fixed (Rule 3, blocking-gate-driven scope extension), 1 documented
scope note (not a fix — a coverage-method decision, explicitly not hidden).
**Impact on plan:** No architectural change. Both are conversions/test-design choices inside the
already-approved envelope and matrix shape; neither required a checkpoint.

## Issues Encountered

None beyond the two items above. All twelve `<verify>` gates across the three tasks passed on first
attempt.

## Requirements Status

`REQ-validation-error-attribution` and `REQ-error-hint-envelope` are declared in this plan's
frontmatter and this plan closes the `tools.go` half of the D-06 sweep — but **REQUIREMENTS.md was
NOT updated to mark either complete**, per the standing note that both requirements also require
04-05 (`rules.go`/`summary.go` sweep, the Connect `cursor_mode`⊕`offset` combination check) and
04-06 (D-06a's schema-level `omitempty` extension) to land first (D-06: every site, not a sample).
04-01's executor already had to revert a premature `requirements.mark-complete` for the same reason;
this plan does not repeat that mistake.

## Next Phase Readiness

- `internal/server/tools.go` carries zero bare `fmt.Errorf` values in any of the four gated
  functions (`validateCitations`, `parseWindow`, `effectiveSearchScope`, `effectiveDiscoveryScope`),
  and zero value-echo tails file-wide.
- Plan 04-05 can proceed against a `tools.go` that is fully converted — `effectiveSearchScope`'s new
  class means 04-05 can stop hand-wrapping it at the Connect boundary (noted per the plan's
  instruction: 4 call sites are `listMemory`, `searchMemory`, and both Connect handlers in
  `connectapi.go:172,242`).
- Plan 04-06 (D-06a) can proceed independently — no arg struct tag was touched here.
- No blockers. `task` (lint + full suite) is green on the final tree; `go.mod`/`go.sum` show zero
  diff.

---
*Phase: 04-diagnosability*
*Completed: 2026-08-01*

## Self-Check: PASSED

All created files confirmed present on disk (`internal/auth/bearer401_test.go`,
`internal/server/argattribution_test.go`, this SUMMARY.md). All three task commits
(`fa3dfc98`, `c3e363c0`, `b8823908`) confirmed present in `git log --oneline --all`.
