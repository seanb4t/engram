# Pitfalls Research — Bounded Reads (2026-09-18.01)

**Domain:** Adding size-bounded paging to a Qdrant-backed list/search store layer
(`internal/store`), mapping `ResourceExhausted` to a clear error, and bounding
provider HTTP error bodies (`internal/embed`, `internal/summarize`) — engram
(/Volumes/Code/github.com/seanb4t/engram).

**Researched:** 2026-09-18
**Confidence:** HIGH for anything cited against this repo's own shipped code
(`internal/store/store.go`, `internal/store/migrate.go`, `internal/store/store_test.go`)
and against grpc-go's and Qdrant's own documented/issue-tracked behavior (cited
inline with sources). MEDIUM for the testcontainer flakiness root cause (#497),
since the issue itself states "not investigated" and the leading theory (runner
resource pressure from N parallel Qdrant containers) is inferred from timestamps,
not proven. LOW/speculative flagged inline for anything this milestone has not
yet decided (e.g., whether `ListMemories` moves off `limit:0`=all — PROJECT.md
"Open for discuss-phase").

## Critical Pitfalls

### Pitfall 1: Fixing the *known* 4 MiB overflow sites while missing a sibling one — this bug class recurs by construction, not by surprise

**What goes wrong:**
#583 fixed `Store.ListScopes`'s full-payload Scroll. #585 immediately found the
*same* overflow in `Store.List`'s three modes (offset `Limit:0`=all, deep offset,
1000-record cursor pages). The milestone also names `ListScheduled` and the
256-batch operator sweeps (`migrate`/`revert`/`summarize-missing`/`spine-review`/
`reindex`) as siblings that request full payloads with no page-size ceiling tied
to payload size. Every one of these call sites shares the identical root cause
(`internal/server/tools.go:123`'s `qdrant.NewClient` sets no
`MaxCallRecvMsgSize`, so grpc-go's 4 MiB default applies uniformly), so fixing
one call site and calling the milestone done reproduces the #583→#585 sequence
a third time.

**Why it happens:**
The overflow is a property of (page size × per-record payload size), not of any
one function. Each call site was written independently, at different times, by
different phases, so a fix scoped to "the path that broke in prod" naturally
stops at that one function instead of walking every `WithPayload(true)` Scroll/
Get/Query call in the store layer.

**How to avoid:**
Before closing this milestone, enumerate every store-layer call that sets
`WithPayload(true)`/`qdrant.NewWithPayload(true)` or omits a payload-size-aware
limit, not just the five named in PROJECT.md. Grep once
(`rg -n 'WithPayload\(true\)|NewWithPayload\(true\)' internal/store/*.go`) and
treat every hit as in-scope until proven otherwise — this catches
`Store.Search`'s full-payload `k`-sized Query (#585 names it explicitly) and
`Store.Get`'s single-point full payload (safe: one record, bounded by
`ENGRAM_MEMORY_MAX_CONTENT_BYTES`-class caps if any exist, but verify — it is
not obviously safe just because it is "only one point," since memory `content`
is currently unbounded per PROJECT.md's "Open for discuss-phase").

**Warning signs:**
A fix PR whose regression test only covers the exact function named in the
issue, with no test asserting the *other* four named call sites also hold under
an equivalent oversized fixture.

**Phase to address:**
An early phase should do the full-inventory sweep and land the recall-gate/
paging fix as one coherent mechanism (e.g., a shared "bounded scroll" helper)
rather than five independent per-function patches — the repeated-defect pattern
above is itself evidence that per-function patching under-generalizes here.

---

### Pitfall 2: Bounding page size alone does not bound response size — record `content` is unbounded, so a *small* page can still overflow

**What goes wrong:**
`maxListLimit = 1000` (store.go:1458) bounds record *count*, not bytes. #585's
own PR notes "an average payload above about 4 KiB overflows" at 1000 records —
but there is no floor preventing a single record's `content` from being far
larger than 4 KiB. engram's memory `content` field is explicitly called out in
PROJECT.md as unbounded today (unlike the summary field, which already has
`ENGRAM_MEMORY_MAX_SUMMARY_BYTES`, default 512 bytes). A page of just 4 records
each holding 1.5 MiB of `content` overflows the same 4 MiB cap that a 1000-record
page of tiny records would not.

**Why it happens:**
Count-based limits (`maxListLimit`, `reindexBatch = 256`, `migrateBatch = 256`)
were designed as reasonable *pagination* ergonomics, not as a defense against
this specific gRPC ceiling — they predate the discovery that payload size, not
record count, is the actual constraint.

**How to avoid:**
Treat "cap page size" and "cap content size" as two independent, complementary
fixes, and do not let landing one read as having landed the other. PROJECT.md
already flags the `ENGRAM_MEMORY_MAX_CONTENT_BYTES` question as open for
discuss-phase — resolve it explicitly (even if the resolution is "not this
milestone, tracked as a follow-up issue") rather than letting the paging fix
implicitly stand in for it. If content stays unbounded, the paging fix must be
resilient to a single record's payload alone exceeding the cap (i.e., page size
1 can still fail, and that failure must map to a clear error, not `internal`).

**Warning signs:**
A regression test fixture that proves the count-based cap works (e.g., "1000
tiny records still fit") without a companion test proving a small page of large
records is handled (either succeeds via smaller effective page size, or fails
with a clear, named error rather than `internal`).

**Phase to address:**
Same phase as Pitfall 1's inventory — the "done means" bar in PROJECT.md
("every exposed path carries a real-Qdrant regression test holding more than
4 MiB of payload") should be read as requiring both a many-small-records
fixture AND a few-large-records fixture per path, not just one shape.

---

### Pitfall 3: Order-by-ties + concurrent inserts break keyset (cursor) paging silently — duplicates or skips with no error

**What goes wrong:**
`listByCursor` (store.go:1463) resumes via `qdrant.NewStartFromDatetime(c.C)`
plus a `seen` id set for records exactly at the boundary timestamp. This is
correct *only* if `created_at` collisions are rare enough that the `seen` set
(capped at `maxListLimit = 1000`, store.go:1479) never needs to hold more ids
than that. Two failure shapes exist today, both silent (no error returned):
1. **Tie overflow:** if more than 1000 records share the exact same `created_at`
   boundary, `decodeCursor` rejects a *client-replayed* oversized cursor
   (`ErrInvalidArgument`), but the *server* never detects that it under-counted
   `seen` on the page that produced the cursor — some boundary records are
   silently skipped rather than surfaced on the next page.
2. **Concurrent insert at/before the cursor boundary:** a new record inserted
   with a `created_at` earlier than a page already served, but before the
   in-flight next-page fetch runs, is invisible to the resumed scan by
   construction (keyset paging over a mutable, non-append-only key is
   inherently vulnerable to this) — this is expected/acceptable for created_at-
   keyed pagination in general, but it must not be *conflated* with the size-
   bounding work in this milestone as if fixing size bounds also fixed
   pagination correctness.

**Why it happens:**
Keyset pagination assumes the ordering key changes rarely relative to page
size. `created_at` has millisecond (or coarser, depending on stamp precision)
granularity, so bulk operations (a migration backfill, a bulk import, or many
records written in the same request-handling tick) can produce more same-
timestamp records than any one page's `seen` budget anticipates.

**How to avoid:**
Do not treat "bound the page byte size" and "keyset paging is correct under
ties/concurrent writes" as the same problem — this milestone's stated scope is
the former. If the phase touches `listByCursor` at all (e.g., to add a
byte-size-aware page shrink), add an explicit regression test for >1000
same-`created_at` records proving the *documented* behavior (skip-with-no-error
today, or a named error after the fix) rather than silently changing behavior
as a side effect of the size fix. If out of scope, say so explicitly in the
phase's SPEC/PLAN so a future auditor does not assume this milestone also
proved cursor correctness under ties.

**Warning signs:**
A "done" claim for cursor paging that cites only the 4 MiB fixture (many
records, one `created_at` each) with no same-timestamp tie fixture — the
existing `TestListScopesFullPayloadsOverGRPCLimit`-style fixture (store.go
uses distinct sequential UUIDs but a shared `time.Now()` at upsert time — check
whether all 40 records in that pattern actually share one truncated
`created_at`, which would make it an accidental tie-fixture already).

**Phase to address:**
If `listByCursor` needs a byte-size-aware shrink (see Pitfall 5), do the shrink
in a phase separate from, or explicitly scoped alongside, any change to the
`seen`-set/boundary logic — and add the tie-overflow regression test in
whichever phase touches this function, since it is adjacent code that is easy
to perturb without noticing.

---

### Pitfall 4: A two-phase ids→payload read reintroduces TOCTOU that the current one-shot Scroll does not have

**What goes wrong:**
#585 lists "fetch only the ids with `WithPayload(false)` and then load payloads
for just the returned page" as a candidate fix for offset-mode deep paging.
This converts one atomic-per-page Scroll into two round trips: (1) resolve the
ordered id window, (2) `Get`/`GetPoints` those ids' payloads. Between phase 1
and phase 2, a record can be deleted, superseded (`superseded_by` set), or
archived (`archived_at` set) — its id is still in the phase-1 window but
`GetPoints` either 404s that id (silently dropping it, changing the page's
count without adjusting `total`) or — worse — `Get` bypasses the recall gate
entirely (per this repo's own documented contract: "Store.Get stays ungated,"
repeated at every soft-hide condition in `store.go`, e.g. lines 1389, 1397,
1630, 1635), so a record that became superseded/archived/expired between
phase 1 and phase 2 could be returned to a *list* caller carrying gated state,
via a path that was never meant to bypass the gate.

**Why it happens:**
`Store.Get` is intentionally ungated (`get_memory` fetch-by-id is documented as
not recall-gated — this is a **feature** for the existing single-id `get_memory`
tool, where the caller already has the id and is explicitly asking for it
regardless of state). Reusing `Get`/`GetPoints` as the payload-fetch half of a
new two-phase *list* path silently imports that ungated behavior into a
recall-gated surface, which is exactly the class of bug the repo's own
"recall-gate AST test pins Scroll call sites" convention exists to prevent —
and a `Get`/`GetPoints`-based fetch is invisible to a Scroll-call-site AST
gate by construction.

**How to avoid:**
If a two-phase ids→payload design is adopted: (a) re-derive the id set's
current gate-relevant fields (or re-check the filter) in phase 2 rather than
trusting phase-1 membership as still valid, or (b) keep the filter conditions
applied in phase 1 as the sole gate and treat phase 2 strictly as a payload
hydration step that must re-verify each returned id is still in the id set
(diff against phase-1 ids; drop and adjust reported count for anything that
vanished, rather than silently shrinking the page or leaking a gated record).
Whichever approach is chosen, extend (or explicitly justify not extending) the
recall-gate AST pin to cover the new `Get`/`GetPoints` call site, since that
convention exists precisely to make "does this new Scroll/Get bypass the gate"
mechanically checkable rather than a code-review judgment call each time.

**Warning signs:**
A phase 2 fetch that reuses `Store.Get` or a bare `qdrant.GetPoints` call
without re-threading the same `Must` filter conditions (superseded/archived/
scheduled/owner) that phase 1 applied — check the diff for any new `client.Get`
call in a list/search path that does not also carry a `Filter` re-check.

**Phase to address:**
Whichever phase implements the two-phase ids→payload approach for offset-mode
deep paging (if that approach is chosen over an alternative — see Pitfall 1's
"whatever mechanism" framing) must include this TOCTOU re-verification as an
explicit acceptance criterion, not an implementation detail assumed correct by
construction.

---

### Pitfall 5: `qdrant.Get`/`GetPoints` does not preserve requested-id order — a two-phase read silently reorders `created_at`-sorted pages

**What goes wrong:**
Qdrant's own issue tracker and API documentation state that `GetPoints` does
not guarantee response order matches the requested id list — the server may
sort internally (e.g., by id) rather than by request order
([qdrant/qdrant#5071](https://github.com/qdrant/qdrant/issues/5071); API
reference notes "if the indices are not sorted, Qdrant will sort them
internally"). Every list/search path in this store orders by `created_at`
(store.go:1439, 1497, 1644). A two-phase design that resolves an ordered id
window in phase 1 via Scroll+`order_by`, then fetches payloads for those ids
via `GetPoints` in phase 2, will receive payloads back in an *unspecified*
order — not the `created_at` order phase 1 established — and must not assume
`GetPoints`'s response order is usable directly.

**Why it happens:**
It is an easy, unstated assumption that "I asked for these N ids, I get back
these N payloads in the same order" — true for many key-value style APIs, not
guaranteed for Qdrant's batch point-retrieval API.

**How to avoid:**
If phase 2 of a two-phase design uses `GetPoints`, re-sort the returned
payloads client-side using the id order established in phase 1 (build an
`id → Memory` map from the `GetPoints` response, then iterate the phase-1
ordered id list to reconstruct output order) — never assume response order.
Add a regression test with a page of records whose ids are NOT in `created_at`
order when sorted lexically/by-id (the two orderings must diverge for the test
to be meaningful), asserting the final returned order matches `created_at`.

**Warning signs:**
A two-phase implementation that appends directly to output from a `for _, p
:= range pts` loop over a `GetPoints` response without an intermediate
map-and-reorder step — this is the same `fromPayload(p.Id.GetUuid(), p.Payload)`
loop shape already used elsewhere in this file (e.g., store.go:1446,
1509, 1651) for *Scroll* results (which the codebase apparently treats as
order-preserving in request/response — verify this assumption too if any of
these loops is ever fed by a `GetPoints` call instead of `Scroll`).

**Phase to address:**
Same phase as Pitfall 4 — order preservation is a correctness requirement of
the same two-phase mechanism, not a separate concern, and should be proven by
the same regression test suite (a test with non-monotonic ids interleaved with
`created_at` order catches both TOCTOU-adjacent count issues and ordering bugs
in one fixture).

---

### Pitfall 6: A regression test fixture "under 4 MiB" measured the wrong number — grpc-go enforces the *decompressed* size, and the existing fixture already gets this right by accident, not by stated intent

**What goes wrong:**
grpc-go's `MaxRecvMsgSize`/`MaxCallRecvMsgSize` check applies to the message
size *after decompression*, not the compressed wire size (confirmed against
grpc-go's own issue tracker:
[grpc/grpc-go#4761](https://github.com/grpc/grpc-go/issues/4761), and
corroborated by community write-ups on the "received message after
decompression larger than max" error text this repo's own #583/#585 issues
quote verbatim). A test author who reasons "my fixture is highly compressible
(e.g., `strings.Repeat("x", n)`), so gRPC-level compression will keep the wire
message under 4 MiB, so this won't reproduce the bug" has the causality
backwards: compression (if enabled) would make the WIRE transfer smaller, but
the RECEIVE-SIZE check still fires because it re-measures after decompressing
back to the full logical size. The inverse mistake is equally real: assuming a
fixture's *logical* (decompressed) byte count is what must exceed 4 MiB, and
then padding it with a compressible filler in a way that happens to convince a
reviewer the fixture is "smaller than it looks" — it is the decompressed size
that must exceed the cap, full stop, regardless of compressibility.

**Why it happens:**
"Compression" and "message size limit" are easy to conflate — many engineers'
mental model is "compression makes messages smaller, so it should help avoid
size-limit errors," which is true for the *wire* size but false for a receiver
that checks decompressed size (which is exactly what protects a receiver from
a decompression-bomb-style attack — checking pre-decompression size would
defeat that protection entirely).

**How to avoid:**
This repo's shipped `TestListScopesFullPayloadsOverGRPCLimit`
(store_test.go:1798) already gets this right, and its own guard comment
(`if n*contentBytes <= 4<<20 { t.Fatalf(...) }`, store_test.go:1810) is the
correct pattern: assert the *logical* fixture size (`n * contentBytes`, the
decompressed content size Qdrant will actually return) exceeds the cap, not
some measured wire/gzip size. New regression tests for `Store.List`,
`ListScheduled`, and the operator sweeps should copy this exact guard-assert
pattern rather than reasoning freshly about compression each time. Also
verify (this repo does not appear to enable client-side gRPC compression
today — no `grpc.UseCompressor`/`WithDefaultCallOptions(grpc.CallContentSubtype`
hits found in `internal/`) that no future change silently enables compression
without re-confirming this reasoning still holds.

**Warning signs:**
A new fixture that sizes its content based on an estimated *compressed* size,
or a fixture that uses low-entropy filler content specifically because "it'll
transmit fast" (compression speed/ratio should never be a factor in choosing
fixture content — use whatever is simplest, since decompressed size is the
only thing that matters).

**Phase to address:**
Whichever phase writes new regression tests for `Store.List`/`ListScheduled`/
operator sweeps should explicitly reuse (or extract into a shared helper) the
`store_test.go:1810`-style guard-assert, so the reasoning is enforced by a
compile-time-adjacent check rather than re-derived per test file.

---

### Pitfall 7: A test client that sets its own `MaxCallRecvMsgSize` (or a different default) stops proving anything about production

**What goes wrong:**
Today, both the production client (`internal/server/tools.go:123`,
`qdrant.NewClient(&qdrant.Config{...})`) and the test clients (e.g.,
`internal/store/store_test.go:190`, `internal/e2e/spine_review_test.go:73`) set
no `MaxCallRecvMsgSize`, so both inherit grpc-go's identical 4 MiB default —
this is precisely why `TestListScopesFullPayloadsOverGRPCLimit` is a valid
regression test for the production bug. The milestone's own "done means" bar
(PROJECT.md: "Raising `MaxCallRecvMsgSize` alone only moves the ceiling — #583
rejected it as a fix; it is defense in depth at most") already names the
danger of adopting that mitigation in production. The less obvious version of
the same mistake: if a future PR adds `MaxCallRecvMsgSize` as defense-in-depth
to the *production* client (`tools.go:123`) without applying the identical
value to every test-client construction site (there are at least 9 separate
`qdrant.NewClient(&qdrant.Config{...})` call sites across
`internal/store/*_test.go`, `internal/e2e/*_test.go`, `internal/server/*_test.go`,
and `internal/retrievaleval/*_test.go`), the test suite's receive limit drifts
from production's, and every existing/new 4 MiB-boundary regression test
silently stops proving anything about the deployed binary.

**Why it happens:**
There is no single shared client-construction helper in this repo today for
test-side Qdrant clients — each test file independently calls
`qdrant.NewClient(&qdrant.Config{Host: host, Port: port})`, so a change to the
one production call site (`tools.go`) has no structural mechanism forcing a
matching update everywhere else.

**How to avoid:**
If this milestone adds `MaxCallRecvMsgSize` (or any other gRPC dial option) as
defense-in-depth to the production client, either (a) extract a single shared
client-construction function that both production and every test call site
use (closing the drift risk structurally, matching this repo's own stated
preference for correct-by-construction gates over per-site vigilance — see
CLAUDE.md's `internal/surfaces` conformance-gate pattern), or (b) explicitly
add a test (an AST/grep gate, in the spirit of the existing recall-gate AST
test) asserting every `qdrant.NewClient` call site in the tree uses the same
dial-option set. Do not rely on manual "remember to update the other 9 files"
discipline.

**Warning signs:**
A PR diff that touches `tools.go`'s `qdrant.NewClient` call but no test file —
or a regression test that starts passing not because the underlying fetch
shrank but because the test client's own limit was quietly raised somewhere in
its construction path.

**Phase to address:**
Whichever phase adds any gRPC dial-option defense-in-depth to the production
client must, in the same phase, either unify client construction or add the
drift-detecting gate — this is exactly the kind of gap the "regression tests
that don't actually go RED" quality gate in this research task is meant to
catch, and it will not be caught by `task test` passing (all clients agree with
each other; they just no longer agree with what's shipped, and there's no way
today to notice that from a green test run).

---

### Pitfall 8: `total`/count semantics silently change meaning when paging becomes multi-round-trip or gains a byte-size-aware page shrink

**What goes wrong:**
`Store.List` currently returns an *exact* `total` via `Count` with
`Exact: qdrant.PtrOf(true)` (store.go:1407) — a real count over the full
filtered set, computed once, independent of the page-fetch mechanism. Two
plausible fixes threaten this contract: (1) if offset-mode "all" (`Limit: 0`)
moves to internally-paged batches to stay under the byte cap, `total` must
still reflect the *whole* filtered set, not just what fits in memory across the
batches actually fetched before a caller-visible limit is reached — an easy
bug is computing `total` as "however many I managed to page through" instead
of the pre-existing exact `Count` call. (2) If a page's size is *shrunk*
dynamically because its records are unusually large (a byte-size-aware
response to Pitfall 2), the *returned item count* for that page becomes
smaller than the caller's requested `Limit` even though more matching records
exist and were not filtered out — this is a new, currently-nonexistent
semantic ("I asked for 50, I got 12, but there's no error and `total` says
500") that every caller (Connect API, console, `engram list` CLI, MCP
`list_memory`) needs to handle correctly, especially cursor-based paging logic
that decides "is this the last page" partly from `len(out) < limit`
(store.go:1521) — a page shrunk for byte reasons, not exhaustion reasons, must
not be mistaken by that check for "no next page."

**Why it happens:**
`total` and per-page `len(items)` are currently orthogonal by construction
(one is an independent `Count` call; the other is whatever a single Scroll
returned). Any fix that couples fetch behavior to *payload size* rather than
purely to *filter results* breaks that independence unless deliberately
re-established.

**How to avoid:**
Keep the `total` computation (the `Count` call) entirely independent of
whatever page-fetch strategy is chosen for size-bounding — do not let it
become "the count of what I actually paged through." For cursor mode, if a
page can legitimately come back shorter than `limit` for a reason *other than*
exhaustion (byte-size shrink), either (a) do not expose that shrink to the
caller at all — internally keep fetching in smaller sub-batches until `limit`
distinct records are assembled or the filtered set really is exhausted (this
preserves the existing `len(out) < limit` ⇒ no-next-page invariant exactly),
or (b) if (a) is infeasible under the byte cap, introduce an explicit signal
(distinct from `nextCursor == ""`) that this page was truncated for size
reasons and more data is available at the same cursor position — never overload
`nextCursor == ""`/short-page to mean two different things.

**Warning signs:**
A page-shrink implementation that changes the meaning of `len(out) < limit`
without updating every one of that function's callers (the offset-mode "last
page" check at store.go:1449, the cursor-mode exhaustion check at store.go:1521,
and anything downstream in `connectapi.go`/console/CLI that infers "done
paging" from an empty or short next-cursor).

**Phase to address:**
Whichever phase implements the actual size-bounding mechanism must treat
`total`/exhaustion-signal preservation as an explicit acceptance criterion with
its own test (assert `total` is unchanged by whatever internal batching
happens; assert a size-forced partial fetch does not present as "last page"
unless it truly is).

---

### Pitfall 9: `ResourceExhausted → clear error` becomes an information-leaking or over-generic error-mapping site if done casually

**What goes wrong:**
The milestone requires mapping Qdrant's `ResourceExhausted` (currently
surfaced by `connectError` as an opaque Connect `internal`, per #585's
reproduction) to "a clear error." Two opposite failure modes are both easy to
introduce here: (1) **leaking internals** — echoing the raw gRPC error text
(`"rpc error: code = ResourceExhausted desc = grpc: received message after
decompression larger than max 4194304"`) verbatim to an API caller exposes
implementation details (the exact byte ceiling, the fact that the backing
store is gRPC-based at all, an internal collection/point-count hint) that this
repo's own existing error-envelope convention (`field=<name> hint=<code>:
<text>`, documented in CLAUDE.md and `reference/errors.md`) is designed to
avoid — every other validator on both wires uses a bounded, named hint code,
not raw upstream text. (2) **over-broad catch** — a naive `strings.Contains(err.Error(), "ResourceExhausted")` or blanket gRPC-status-code
switch could also catch *legitimate* resource-exhaustion signals unrelated to
this bug (e.g., a genuine Qdrant memory/disk pressure `ResourceExhausted` that
has nothing to do with the 4 MiB receive cap), mapping them to a message that
falsely implies "your request was too large" when the real problem is
server-side capacity.

**Why it happens:**
The fastest fix to "surface a clear error" is often "just pass the upstream
error string through with a nicer HTTP-ish code," which satisfies "not
`internal`" without satisfying "clear and safe."

**How to avoid:**
Introduce a new named hint code (matching the existing `field=<name>
hint=<code>` envelope convention, e.g. `hint=result_too_large` or similar) for
the specific *receive-size* `ResourceExhausted` case, distinguished by
matching on the gRPC status code (`codes.ResourceExhausted`) AND ideally the
specific message shape (`"received message after decompression larger than
max"`) rather than the code alone, so a genuine server-capacity
`ResourceExhausted` is not mislabeled. The client-facing text should name the
*actionable* fact (e.g., "the result set is too large for one request; use a
smaller limit or narrower filter") without echoing the raw byte-count/gRPC
internals. Add this new hint to `reference/errors.md` per the existing
`internal/surfaces` conformance-gate convention (CLAUDE.md: "each server-side
conditional rule is declared once and machine-proven present on every surface
that advertises it") rather than a one-off `fmt.Errorf` that only one wire
happens to catch.

**Warning signs:**
An error-mapping change whose test asserts only "the HTTP status is no longer
500" without asserting the response body does NOT contain the raw upstream
error string, the literal byte ceiling, or the word "grpc"/"Qdrant" verbatim.

**Phase to address:**
The `ResourceExhausted`-mapping phase must land the hint code through the
`internal/surfaces` declare-once mechanism (if that mechanism covers error
hints — verify) or an equivalent single-declaration site, with a test
asserting the response text is scrubbed, not just re-coded.

---

### Pitfall 10: Bounding a provider error-body drain (#347/#457) with a limit that doesn't match reality, or that reintroduces the exact hang it fixes

**What goes wrong:**
#457's own fix suggestion — `io.Copy(io.Discard, io.LimitReader(resp.Body, N))`
— is correct in shape, but two implementation mistakes are easy: (1) choosing
`N` so small that legitimate provider error bodies (which can be verbose JSON
with nested validation detail) get truncated mid-structure in a way that makes
the surfaced snippet *look* correct but is actually cut off mid-field,
producing a misleading partial error to operators; (2) forgetting that
`io.LimitReader` bounds *bytes read*, not *time* — a slow-loris-style provider
that dribbles bytes one at a time up to the limit N over an arbitrarily long
duration is not bounded by `LimitReader` alone. #457 explicitly names
`WithTimeout(0)` as the scenario this protects against — a `LimitReader`-only
fix does not actually protect that scenario if the provider trickles the
allowed N bytes slowly forever; the drain call itself needs either a
`context.WithTimeout` wrapped around the copy, or a deadline set independent
of `http.Client.Timeout`, to actually close the gap #457 describes.

**Why it happens:**
"Bound the read" and "bound the time" are different axes, and `io.LimitReader`
only ever addresses the first. The fix's own text ("Draining a bounded prefix
still enables connection reuse... abandoning the connection when the remainder
exceeds N is strictly better than blocking") is correct for the *size* axis
but does not by itself close the *time* axis that motivated filing the issue
(`WithTimeout(0)` removing the only existing bound).

**How to avoid:**
Pair the byte-size `LimitReader` with an explicit per-drain deadline (e.g., a
short `context.WithTimeout` scoped to just the error-body drain, independent
of the overall request's `http.Client.Timeout`) so that even under
`WithTimeout(0)`, a slow/hostile body cannot hold the goroutine open
indefinitely. Choose `N` (the byte prefix) generously enough to capture
realistic provider error payloads (a few KiB, matching the existing
`ENGRAM_MEMORY_MAX_SUMMARY_BYTES`-style precedent of ~512 bytes to a few KiB
for "enough to be useful, not enough to be a vector") and add a test asserting
a body larger than N is truncated with an explicit "(truncated)"-style marker,
not silently cut.

**Warning signs:**
A fix that adds `io.LimitReader` to the drain but does not touch how/whether a
deadline applies to that specific `io.Copy` call — check whether the fix PR's
regression test actually exercises `WithTimeout(0)` plus a slow/large body
(the exact scenario #457 names), or only exercises a body that's merely large
but returns instantly (which `LimitReader` alone already handles fine, making
the test pass without proving the harder case).

**Phase to address:**
The embed/summarize bounded-error-body phase should test both axes
independently: a large-but-fast body (proves the byte bound) and a
slow-trickle body under `WithTimeout(0)` (proves the time bound) — landing
only the first test would look done but leave #457's actual concern open.

---

### Pitfall 11: Fixing `ListScopes`'s "discards successful hits on failure" (#456) by making `ListScopes` never fail, instead of making its caller resilient

**What goes wrong:**
#456 describes `cross_spine=true` recall discarding valid `search_memory`/
`list_memory` hits when the follow-up `Store.ListScopes` call (used only to
populate `searched_scopes`) fails. A tempting shortcut is to make
`ListScopes` itself more defensive (e.g., swallow its own errors and return an
empty scope list) rather than changing the caller's (`tools.go:1592-1595`,
`tools.go:1633-1636`) error-handling to distinguish "the actual recall failed"
from "the coverage-reporting side-call failed." Swallowing the error inside
`ListScopes` reintroduces exactly the ambiguity #456's own "why it is
currently correct" section says the design deliberately avoids: an empty
`searched_scopes` on error would be indistinguishable from a real "searched
nothing," defeating `REQ-cross-spine-result-provenance`.

**Why it happens:**
The path of least resistance for "don't discard successful hits" is "make the
failing sub-call not fail," which is simpler to write than "return hits AND a
distinct sentinel for unknown-coverage," but is exactly the wrong simplification
per this issue's own analysis.

**How to avoid:**
Implement the "third state" #456 itself proposes: return the already-computed
hits plus an explicit sentinel distinguishing "coverage unknown" (the
`ListScopes` side-call failed) from "coverage empty" (`scopes_truncated`
already means something specific — a bounded sample — and must not be
overloaded to also mean "unknown"). This requires a wire-visible field change
(MCP tool response + Connect proto, mirroring how `searched_scopes`/
`scopes_truncated` were added in a prior milestone) — plan for that surface
area rather than assuming a store-layer-only fix suffices.

**Warning signs:**
A fix that only touches `internal/store/store.go`'s `ListScopes` and does not
touch `tools.go`'s two call sites or the Connect/MCP response shape — #456's
own two named sites are the actual bug location, not `ListScopes` itself.

**Phase to address:**
The cross-spine-resilience phase must include the wire-shape change (new
sentinel field) as part of its scope, not just a store-layer retry/fallback,
and should reuse the existing `searched_scopes`/`scopes_truncated` precedent
for how a new field gets threaded through MCP + Connect + CLI consistently.

---

### Pitfall 12: Treating #497's testcontainer flakiness as "add a retry" instead of addressing the actual resource-pressure root cause — and this milestone's regression tests make the failure MORE likely, not less

**What goes wrong:**
#497 documents `internal/store`'s Qdrant testcontainer dying mid-run
(`connection refused`) 3 times in ~2 hours on 2026-08-12, including on a
docs-only PR, with the leading theory being CI-runner resource pressure from
several concurrent per-package Qdrant containers (`TestMain` in each
Qdrant-backed package, store_test.go:117, provisions its own container) plus
concurrent Go build/link load, not a code defect. This milestone's own "done
means" bar requires "a real-Qdrant regression test holding more than 4 MiB of
payload" for *every* exposed path — meaning this milestone adds several new
tests that each write multiple MiB of payload into the `internal/store`
container (mirroring `TestListScopesFullPayloadsOverGRPCLimit`'s existing ~5
MiB write). Every new large-payload fixture increases exactly the kind of
memory pressure #497's own theory blames for container death — a superficial
"retry on failure" fix to #497 would not address that this milestone is about
to make the underlying resource-pressure condition *worse*, right as CI
depends on that container's stability more than before.

**Why it happens:**
#497 and this milestone were filed/scoped independently, so the compounding
effect (more large fixtures → more memory pressure → more container deaths)
is not obvious from either issue read alone.

**How to avoid:**
Address #497 as an explicit, early deliverable of this milestone (it is
already named in PROJECT.md's target features, "since this milestone's
regression tests load exactly that CI job") — not as an afterthought once the
new large-payload tests are already flaking CI. Concretely: (a) mark the new
large-payload regression tests `testing.Short()`-skippable (the existing
pattern at store_test.go:1799-1801, `"writes about 5 MiB of payload; skipped
in -short"`) so they do not run in every CI invocation; (b) consider whether
`internal/store`'s container needs an explicit memory floor/reservation (or
whether CI should serialize Qdrant-backed packages rather than running them
in parallel, per #497's own "possible directions"); (c) capture container
exit reason (`docker inspect`/container logs) on test-container-death so a
future flake has evidence instead of inference, per #497's own suggestion.

**Warning signs:**
Any new 4 MiB-plus fixture test added to `internal/store/store_test.go`
without a `testing.Short()` skip guard, and no CI job change addressing #497's
container-lifetime/resource-pressure theory before those tests land.

**Phase to address:**
Address #497 (or at minimum its `testing.Short()` mitigation plus evidence-
capture) in the SAME phase, or an earlier phase, that starts adding new
4 MiB-plus regression fixtures — reversing that order (fixtures first,
stability later) guarantees a CI-stability regression window during the
milestone.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|------------------|
| Raise `MaxCallRecvMsgSize` on the Qdrant client without also bounding page size | Fast, one-line fix; unblocks the immediate `ResourceExhausted` | Moves the ceiling instead of removing it — #583 already rejected this as "the fix"; a bigger page or bigger record eventually re-triggers it, at a size that's harder to hit in tests | Only as an explicit, documented defense-in-depth LAYER alongside a real page/content-size bound — never as the sole fix (PROJECT.md states this outright) |
| Swallow `ListScopes` errors inside the function to avoid discarding cross-spine hits (#456) | Simple, localized change | Reintroduces the exact "coverage unknown vs. coverage empty" ambiguity `searched_scopes`/`scopes_truncated` exist to prevent | Never — the issue's own analysis already rejects this |
| Skip the byte-size-aware page shrink and just lower `maxListLimit`/`reindexBatch`/`migrateBatch` to a smaller fixed number | No code change beyond a constant | Still fails once average record size grows past whatever new fixed count was chosen (content is unbounded) — a numeric knob turn, not a fix | Acceptable ONLY as an interim mitigation shipped alongside, not instead of, a real fix, and only if content-size capping (`ENGRAM_MEMORY_MAX_CONTENT_BYTES`) is also resolved in the same milestone |
| Add a retry-on-connection-refused wrapper around the testcontainer test suite instead of investigating #497's resource-pressure theory | Immediately reduces CI red builds | Masks a real future defect the same way #497 itself documents happening on #494's rerun ("the actual defect was a real assertion failure in internal/e2e... dominated the log and initially read as flaky infra") | Never as the only fix; acceptable as a short-term stopgap only alongside the evidence-capture and short-test-skip mitigations |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|-------------------|
| Qdrant `Scroll`/`Query`/`Get` via `qdrant-go-client` | Assuming `WithPayload(true)` full-payload fetches are "safe" below some record count, when the real constraint is bytes | Compute/bound by estimated payload bytes (or content length), not solely by record count; treat `maxListLimit`/`reindexBatch`/`migrateBatch` as ergonomics, not safety bounds, until content size is also capped |
| Qdrant `GetPoints` | Assuming response order matches requested id order | Re-sort client-side by the id order established upstream (e.g., from a prior ordered Scroll); never rely on `GetPoints` response order (confirmed non-guaranteed: [qdrant/qdrant#5071](https://github.com/qdrant/qdrant/issues/5071)) |
| grpc-go `MaxRecvMsgSize`/`MaxCallRecvMsgSize` | Assuming compression reduces exposure to the receive-size cap | The cap applies to the DECOMPRESSED size ([grpc/grpc-go#4761](https://github.com/grpc/grpc-go/issues/4761)); compression changes wire size only, never the enforced limit |
| Connect error mapping (`connectError`) | Passing a Qdrant/gRPC error's raw text straight through on the new `ResourceExhausted` clear-error path | Map to a new named hint code in the existing `field=<name> hint=<code>` envelope; never echo raw gRPC/Qdrant error text to a caller |
| Embed/summarize provider HTTP clients | Bounding the error-body drain by bytes only (`io.LimitReader`) and treating that as closing #457 | Bound bytes AND time independently — `io.LimitReader` alone does not protect against a slow trickle under `WithTimeout(0)` |
| testcontainers-go Qdrant module | Adding large-payload regression fixtures to `internal/store` without accounting for #497's container-death pattern | Gate new multi-MiB fixtures behind `testing.Short()` (existing precedent at store_test.go:1799) and address #497's stability question in the same milestone, not after |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|-----------------|
| Two-phase ids→payload reads for deep offset paging | Extra Qdrant round trip per page; latency roughly doubles for deep-offset console/CLI paging even when the byte cap was never at risk | Only apply the two-phase path when the single-Scroll approach would actually risk the byte cap (e.g., large offsets specifically, not every page); consider whether a numeric-offset-free redesign (cursor-only) is cheaper overall than patching offset mode | Becomes visible as soon as the console's deep-offset paging (already flagged in #585 as "gets heavier with every page" even before this fix) is used interactively — the two-phase fix does not remove that O(offset) cost, it just avoids failing outright |
| Byte-size-aware page shrink implemented as "fetch max page, then locally truncate to fit under a byte estimate" | Wastes the fetch already done — the oversized fetch itself is what triggers `ResourceExhausted`, so truncating AFTER the fetch never avoids the failure it's meant to prevent | The shrink must happen BEFORE the Qdrant call (e.g., request a smaller `Limit`, or use `WithPayloadInclude` to shed fields, not truncate a response that already 4-MiB-overflowed on the wire) | Immediately — this is not a scale threshold, it's a logic error: you cannot locally truncate a response that already failed to arrive |
| 256-batch operator sweeps (`migrateBatch`, `reindexBatch`) re-deriving the backlog each pass under concurrent writes (#501 precedent) | A sweep that never converges, or double-processes records, under sustained concurrent writes during a long sweep | Any size-bounding change to these sweeps must preserve the existing re-derive-each-pass convergence property (`internal/migrate` design) rather than introducing a stale offset/cursor that concurrent writes can invalidate | At collection sizes large enough that a sweep takes multiple passes while writes continue — already a known/handled case (#501); a size-bounding patch must not silently regress it |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Echoing raw Qdrant/gRPC error text (including the literal byte ceiling, "grpc", collection internals) in the new `ResourceExhausted` clear-error response | Leaks internal architecture (backing store is gRPC-based, exact size ceiling, potentially collection/point-count hints) to any authenticated caller, aiding reconnaissance | Map to a bounded, named hint code with operator-actionable but implementation-silent text, per the existing `field=<name> hint=<code>` envelope convention |
| Provider (embed/summarize) error-body surfacing (#347) exposing the FULL non-2xx body, including any secrets a misconfigured self-hosted gateway might echo back (e.g., a reflected Authorization header in a verbose error page) | A bounded-prefix fix that is bounded in size but not in WHAT is captured could still leak a credential fragment if a gateway's error page echoes request headers | Bound the byte count (#347/#457 already specify this) AND consider whether the captured prefix should be scanned/redacted for header-echo patterns, or documented as "operator's own gateway's responsibility to not echo secrets in error bodies" — pick one explicitly rather than leaving it undecided |
| Two-phase ids→payload read bypassing the recall gate via `Store.Get`/`GetPoints` (Pitfall 4) | A superseded/archived/expired record briefly visible through a list surface that is supposed to hide it — a real authz-adjacent correctness bug, not just a UX glitch, since some of these states exist specifically to hide corrected-away or expired content | Re-verify gate-relevant filter conditions in phase 2 of any two-phase design; extend the recall-gate AST pin to cover any new `Get`/`GetPoints` call site introduced by this milestone |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| A byte-size-forced page shrink silently returning fewer records than the caller's `Limit` with no indication why | Console/CLI users see an unexplained, seemingly-arbitrary page size that doesn't match what they asked for, and may assume it's a bug or that they've reached the end when they haven't | Surface an explicit signal (new field or documented convention) distinguishing "fewer than requested because the set is exhausted" from "fewer than requested because of a size bound," mirroring how `scopes_truncated` already communicates a bounded-sample condition |
| `ResourceExhausted` mapped to a generic "internal server error"-flavored message that's merely a differently-coded 500 | Operators still can't tell whether to retry, narrow their filter, or file a bug — trading one opaque error for another | The clear-error text should name the actionable remediation (narrow the filter, use a smaller limit/date range) directly, matching the existing hint-code convention's intent |
| Deep offset-mode paging in the console silently getting slower page-by-page (already true today per #585) with no visible indication | Users clicking "next page" repeatedly see increasing latency with no explanation, may assume the app is broken | Independent of this milestone's core scope, but worth flagging: if the two-phase fix is adopted, consider whether the console should nudge deep-offset users toward cursor-based paging instead of silently absorbing the O(offset) cost |

## "Looks Done But Isn't" Checklist

- [ ] **`Store.List` full-payload fix:** Verify a fixture with a SMALL number of LARGE records (not just many tiny records) also passes — count-based caps alone do not prove byte-size safety (Pitfall 2).
- [ ] **Two-phase ids→payload read (if adopted):** Verify a test proves output order matches the original `created_at` ordering when phase-2 `GetPoints` ids are NOT naturally id-sorted in that order (Pitfall 5) — a fixture where lexical id order happens to match `created_at` order would pass even with a real ordering bug.
- [ ] **Two-phase ids→payload read (if adopted):** Verify a test proves a record deleted/superseded/archived between phase 1 and phase 2 is handled correctly (dropped with adjusted count, not silently gate-bypassed) (Pitfall 4).
- [ ] **`ResourceExhausted` clear-error mapping:** Verify the response body does NOT contain the raw upstream gRPC error string, byte ceiling, or "grpc"/"Qdrant" literal (Pitfall 9) — not just that the HTTP status changed.
- [ ] **Bounded provider error-body drain (#347/#457):** Verify a test exercises `WithTimeout(0)` combined with a slow-trickle body, not just a large-but-fast body (Pitfall 10).
- [ ] **Cross-spine `ListScopes` resilience (#456):** Verify the fix changes the wire shape (a new coverage-unknown sentinel) rather than only retrying/softening the store-layer call (Pitfall 11).
- [ ] **New 4 MiB-plus regression fixtures:** Verify each is gated behind `testing.Short()` per the existing precedent, and that #497's container-stability question was addressed before or alongside adding them (Pitfall 12).
- [ ] **Any new gRPC dial-option defense-in-depth (e.g., `MaxCallRecvMsgSize`) on the production client:** Verify every test-side `qdrant.NewClient` call site either shares the change or there's an explicit gate proving they can't silently drift (Pitfall 7).
- [ ] **`total`/exhaustion semantics:** Verify `total` is computed identically before and after whatever paging mechanism changed, and that "short page" still means "last page" everywhere it's checked (offset mode store.go:1449, cursor mode store.go:1521, and any Connect/console/CLI caller inferring done-ness from an empty next-cursor) (Pitfall 8).

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|-----------------|-------------------|
| A sibling overflow site missed in this milestone (Pitfall 1) surfaces in production later | MEDIUM | File a new issue following the exact #583→#585 template (mechanism, live log excerpt if available, regression test pattern to reuse); the fix mechanism from this milestone should generalize directly since the root cause is identical |
| A two-phase read's TOCTOU gap (Pitfall 4) lets a superseded/archived record leak through a list surface | MEDIUM-HIGH | Treat as a recall-gate defect (same severity class as the repo's existing recall-gate AST test protects against); patch the specific call site to re-verify filter membership in phase 2, then retroactively extend the AST pin to cover it so the class of bug can't recur silently |
| `total`/exhaustion semantics regressed (Pitfall 8) and shipped, causing console/CLI to report wrong page counts or stop paging early | LOW-MEDIUM | Since `total` is derived from an independent `Count` call, a fix is typically a revert-and-reapply of just the page-fetch change without touching the count path; add the missing regression test before re-landing |
| #497-class testcontainer flakiness reappears mid-milestone because new large fixtures compounded the resource-pressure condition (Pitfall 12) | LOW | `gh run rerun --failed` clears it short-term (established precedent); the durable fix is retroactively adding `testing.Short()` gates to the offending new fixtures and/or serializing Qdrant-backed packages in CI |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|-------------------|----------------|
| 1. Sibling overflow sites missed | Early phase: full-inventory sweep of `WithPayload(true)`/unbounded-scroll call sites, landed as one mechanism | A single grep-derived checklist of every call site, each with its own oversized-fixture regression test, tracked to 100% before phase close |
| 2. Content-size vs. page-size conflation | Same early phase, paired with an explicit discuss-phase resolution of `ENGRAM_MEMORY_MAX_CONTENT_BYTES` | A few-large-records fixture test alongside the many-small-records fixture, for every fixed path |
| 3. Cursor paging ties/concurrent inserts | Whichever phase touches `listByCursor`'s internals for byte-bounding; explicitly scoped in/out | A >1000-same-`created_at` tie fixture with documented (not accidental) behavior |
| 4 & 5. Two-phase read TOCTOU + GetPoints ordering | Phase implementing offset-mode deep-paging fix, IF a two-phase design is chosen | Delete/supersede-mid-fetch fixture; non-monotonic-id ordering fixture |
| 6. Fixture-size reasoning (compression) | Every phase writing a new oversized-payload regression test | Reuse/extend the `store_test.go:1810` guard-assert pattern; code review checks for compression-based size reasoning |
| 7. Test/prod client limit drift | Phase adding any gRPC dial-option defense-in-depth | Either unify client construction or add an explicit cross-file dial-option consistency gate |
| 8. `total`/exhaustion semantics | Phase implementing the actual size-bounding mechanism | Test asserting `total` unchanged; test asserting short-page-but-not-exhausted is distinguishable from real exhaustion |
| 9. `ResourceExhausted` error-mapping leaks | Dedicated error-mapping phase (or folded into the fix phase) | Response-body content assertion (no raw upstream text), new hint code registered via `internal/surfaces`-equivalent single-declaration site |
| 10. Provider error-body drain bound (byte + time) | Embed/summarize bounded-response phase | `WithTimeout(0)` + slow-trickle-body test, in addition to large-but-fast-body test |
| 11. Cross-spine `ListScopes` failure discarding hits | Dedicated cross-spine resilience phase | Wire-shape change (new sentinel) verified end-to-end (store → tools.go → MCP/Connect response), not just a store-layer retry |
| 12. Testcontainer flakiness (#497) | Same phase as, or before, the phase adding the first new multi-MiB fixture | `testing.Short()` gates present on all new large fixtures; evidence-capture (container logs/exit reason) added; CI green across at least a few real runs before declaring stable |

## Sources

- This repo, read directly (HIGH confidence): `internal/store/store.go` (List, listByCursor, ListScheduled, ListScopes, Search, Get, Reindex — lines cited inline above), `internal/store/migrate.go` (`migrateBatch = 256`), `internal/store/store_test.go` (`TestMain` container lifecycle, `TestListScopesFullPayloadsOverGRPCLimit` fixture pattern), `internal/server/tools.go:123` (`qdrant.NewClient`, no `MaxCallRecvMsgSize` set), `.planning/PROJECT.md` (milestone scope and "done means" bar).
- `gh issue view 497/585/583/456/347/457` (this repo's own issue tracker — HIGH confidence, primary source for the bug mechanisms and prior fix rationale).
- grpc-go receive-size-after-decompression semantics: [grpc/grpc-go#4761 "Make MaxCallRecvMsgSize errors clear whether it's compressed or uncompressed"](https://github.com/grpc/grpc-go/issues/4761) (HIGH confidence — grpc-go's own issue tracker, cross-checked against multiple independent write-ups describing the same "received message after decompression larger than max" error text this repo's own logs show).
- Qdrant `GetPoints` response-order non-guarantee: [qdrant/qdrant#5071 "GET points/<id> != POST points { ids: [<ids>] }"](https://github.com/qdrant/qdrant/issues/5071) and Qdrant's own API reference for Retrieve Points (HIGH confidence — primary-source issue tracker plus official API docs).

---
*Pitfalls research for: engram — milestone 2026-09-18.01 "Bounded Reads"*
*Researched: 2026-09-18*
