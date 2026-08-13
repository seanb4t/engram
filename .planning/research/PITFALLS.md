# Pitfalls Research

**Domain:** Record schema versioning + payload-migration mechanism + Connect wire-parity widening,
added to an already-shipped, populated Qdrant-backed memory store (engram)
**Researched:** 2026-08-12
**Confidence:** HIGH (Qdrant batch-atomicity behavior confirmed against an open upstream issue and
the vendored client's own documented chunk size; protobuf field-number permanence confirmed against
protobuf.dev/buf.build docs; all store-layer claims read directly from `internal/store/store.go` and
`cmd/engram/*.go` on this branch) / MEDIUM (schema-version-codec design space — few close precedents
for "flat struct + payload-only codec + no version-dispatch" systems; reasoned from this repo's own
`internal/webauth` session-version precedent plus general JSON-document-migration literature)

## Critical Pitfalls

### Pitfall 1: Treating "schema_version absent" as excludable narrows recall for every pre-migration record, silently

**What goes wrong:**
`Store.Search`/`Store.List`/`Store.SearchDiscovery` each append sibling `Must` conditions to the
authz-derived filter — `qdrant.NewIsEmpty("superseded_by")`, `qdrant.NewIsEmpty("archived_at")` —
to soft-hide records in a given state. The **very next** state this milestone adds is
`schema_version`, sitting textually right next to those two in the same functions. The natural
(wrong) move is to reuse the identical pattern — e.g. gate recall on `schema_version >= N` or
exclude `IsEmpty("schema_version")` "until migrated," mirroring how `superseded_by`/`archived_at`
gate on presence/absence. But `superseded_by`/`archived_at` are **optional** states a minority of
records carry; `schema_version` absent is the **majority** state at adoption — every record written
before this milestone. A filter condition built on that assumption removes 100% of existing data
from recall the moment it ships, with no error, no log line, just empty result sets that look like
an empty store.

**Why it happens:**
The codebase's own established idiom for "new orthogonal record state" IS "add a sibling `IsEmpty`/
`Range` condition to the recall gate" (see the comments at `store.go:1029-1034` explicitly modeling
this as the pattern to extend). Schema version looks like the same shape of feature but has the
opposite cardinality at rollout, so pattern-matching on the neighboring code produces exactly the
wrong answer.

**How to avoid:**
`schema_version` must **never** appear in `ownerScopeFilter`, `activeWindowConditions`, the
`superseded_by`/`archived_at` `IsEmpty` conditions, or any other Must/Should clause built by
`Search`/`SearchReranked`/`SearchDiscovery`/`List`/`ListScheduled`. Write a negative test — a "recall
gate blast radius" test, the mirror image of the existing archived/superseded soft-hide tests — that
constructs the filter for each of these five call sites and asserts no condition references the key
`schema_version`, then seeds a record with the key entirely absent and asserts it round-trips through
every recall path unchanged from today's behavior. If migration status ever needs to be *surfaced* to
an operator (e.g. "how many records are still v0"), that must be a separate `Count`/`Scroll` query
(the `spine-review scan` / `BackfillShortIDs` dry-run precedent), never a recall-path filter.

**Warning signs:** any diff that touches `ownerScopeFilter`, `activeWindowConditions`, or the four
`Search`/`List`/`SearchDiscovery`/`ListScheduled` filter-builders in the same commit that introduces
`schema_version`.

**Phase to address:** the record-schema-versioning phase (the phase that adds the `schema_version`
discriminator to `payload()`/`fromPayload()`) — the negative test must land in the SAME phase, not a
later hardening pass.

---

### Pitfall 2: `fromPayload` has no version-dispatch codec, so migrations can only ever be additive — there is no rollback path

**What goes wrong:**
`payload()`/`fromPayload()` are flat, unconditional codecs: every field is written/read the same way
regardless of what else is in the map (`if v, ok := p["key"]; ok { ... }`). Adding a `schema_version`
int to the struct does not, by itself, give the codec any way to interpret two *different* payload
shapes differently — it can only ever add new optional keys with zero-value defaults (the protobuf
"additive is safe" case). If a future migration needs to rename a key, restructure a nested value, or
change a field's meaning (the JSON-schema-migration literature's "structural" and "semantic" change
classes), there is no mechanism in this codebase today to make `fromPayload` branch on
`schema_version` and decode two shapes differently. Ship `schema_version` believing it enables
arbitrary future migrations, and the first non-additive migration request discovers there is no
version-dispatch seam to hang it on — a much bigger design problem discovered under time pressure.
The same gap means there is no rollback story: an old server binary reading a record a newer binary
migrated non-additively has no way to know it should refuse or reinterpret it (the closest local
precedent, `internal/webauth`'s `sessionPayloadVersion`, sidesteps this entirely by being
short-lived and reject-on-mismatch — a session cookie can be safely invalidated and re-minted; a
memory record cannot).

**Why it happens:**
"Add a version int, auto-inject on write, check on read" (the milestone's own stated design, mirroring
`sessionPayloadVersion`) sounds complete by analogy, but the analogy breaks exactly at the point
where sessions are disposable and records are not. The mirroring is correct for the *injection*
half (auto-stamp on write) and wrong for the *read* half (reject vs. tolerate-and-interpret).

**How to avoid:**
Scope this milestone's migrations to **additive-only** changes (new optional payload keys, absent =
zero-value/legacy), and say so explicitly in the design doc and the `engram migrate` command's own
`--help` text, so a future author doesn't assume the mechanism is more general than it is. If a
non-additive migration is anticipated, that is a separate, larger research question (a real
version-dispatch codec, or the Ditto-style "separate collection per breaking version" pattern) and
should be flagged as deferred/out-of-scope rather than silently assumed solved by `schema_version`
existing.

**Warning signs:** any migration step in this milestone that renames a payload key, changes a key's
type, or removes a key that other code still reads unconditionally.

**Phase to address:** scope this constraint explicitly in the migration-mechanism phase's design
notes; flag "non-additive schema evolution" as an explicit research gap for a later milestone rather
than something this milestone's `schema_version` field secretly solves.

---

### Pitfall 3: A migration step that does a whole-payload `Upsert` drops every out-of-band key it doesn't know about — this has happened before (CR-01)

**What goes wrong:**
`Store.Upsert` replaces a point's *entire* payload (`Payload: qdrant.NewValueMap(payload(m))`). A
migration written as "scroll records, decode into `Memory`, set `SchemaVersion`, `Upsert` it back"
looks correct and round-trips every field `fromPayload`/`payload` know about — but `Memory.payload()`
does NOT round-trip anything the codec doesn't have a field for, and `fromPayload` on an old/partial
decode can silently zero out fields a hand-rolled migration loop doesn't explicitly preserve (this is
exactly the shape of the v0.11.x CR-01 finding: "client-minted `short_id` lost under concurrent keyed
Upsert"). The current codec already carries several `json:"-"` payload-only fields
(`EmbedderIdentity`, `IdempotencyFingerprint`) specifically because they must never leave the Go
struct via JSON — but they are just as vulnerable to being dropped by a migration that builds its own
raw payload map instead of going through `payload()`, or that decodes with an older/leaner struct
shape than production data actually carries.

**How to avoid:** the migration mechanism's write primitive must be a **targeted `SetPayload`**
merge (`s.client.SetPayload` with only the changed key(s), the same shape `SetVisibility` and
`Supersede`'s back-stamp already use) — never a full `Upsert` — unless the migration step is proven
to run `Get` → `fromPayload` → mutate → `payload()` → `Upsert` with the CURRENT struct definition and
zero raw-map shortcuts. Add a test that seeds a record with every optional field populated
(`idempotency_fingerprint`, `superseded_by`, `citations`, `archived_at`, `not_before`, `embedder_identity`,
`summary_source`), runs the migration step, and asserts every one of those keys is byte-identical
afterward — the same "round-trip everything" proof `TestBackfillShortIDs`'s payload-preservation
assertion already does for `short_id`.

**Warning signs:** any migration code path that constructs `map[string]any{...}` by hand instead of
calling `payload()`, or that calls `s.client.Upsert` instead of `s.client.SetPayload`.

**Phase to address:** the migration-mechanism phase, as a hard constraint on the Store primitive the
mechanism is built on, verified by the round-trip test above before any migration ships.

---

### Pitfall 4: Qdrant's multi-ID `SetPayload` is not atomic — a migration sweep must reconcile by re-derivation, not trust the write response

**What goes wrong:**
This codebase already documents (`qdrantPayloadOpBatchSize`, store.go:126-139) that the pinned
Qdrant server chunks a multi-ID payload write by 32 points and that "a later chunk can error after an
earlier chunk has fully committed." This is independently confirmed upstream: Qdrant's own
maintainers, responding to a live bug report of a batch endpoint returning HTTP 400 while valid
points inside the batch were still persisted, state plainly: *"batch operations are not promised to
be atomic... For any operation error, the user must assume the operation is either not, partially, or
fully applied. The user is responsible for sending the same operation again."* A migration sweep that
submits a large ID batch to `SetPayload` and treats a non-error response as "every id in the batch is
now at the new version," or treats an error response as "nothing happened, retry the whole batch,"
will either under-count (miss records that DID get migrated before the error) or double-apply
work — both silent, both hard to detect after the fact.

**How to avoid:** reuse the exact pattern `Store.Supersede`'s `reconcileSupersedeFailure` already
established (D-15): after any batch write, **re-read** the actual state (a fresh `Scroll`/`Count`
against `schema_version < target`) rather than trusting the write call's own success/failure signal.
Keep migration batches at or below `qdrantPayloadOpBatchSize` (32) where practical, but design the
resume/reconcile logic to be correct regardless of batch size — a crash or partial chunk failure must
be indistinguishable, from the next run's point of view, from a clean interruption.

**Warning signs:** any migration retry logic that branches on the write call's returned error rather
than re-querying actual record state; a migration test that only exercises the "clean success" and
"clean total failure" paths and never a forced mid-batch partial failure (the `TestBackfillShortIDsResumesAfterMidRunFailure` / `03.1`'s "forced mid-sequence partial failure" precedents are the
bar to match).

**Phase to address:** the migration-mechanism phase; the partial-failure-resume test is a must-have,
not a nice-to-have, and should be authored against a Qdrant testcontainer only once #497 (testcontainer
stability) is confirmed fixed — see Pitfall 9.

---

### Pitfall 5: A version-chain skip predicate that checks equality (`schema_version == 0`) strands a record mid-chain after a crash

**What goes wrong:**
`BackfillShortIDs`'s skip predicate is a single boolean condition — "the `short_id` key is absent" —
because there is only one state to reach. A `schema_version` migration is not: it is a **chain**
(v0→v1→v2→...), and a crash can leave a record at any intermediate version. If the migration's
"what still needs work" filter is written as `schema_version == 0` (mirroring the single-predicate
style of the existing sweep commands) rather than `schema_version < target`, a record that reached v1
before a crash and needs one more hop to v2 is invisible to a re-run — it doesn't match `== 0`, so it
is silently left at v1 forever, with no error and no visible gap until something reads a field that
only exists from v2 onward.

**How to avoid:** the eligibility filter must be a `Range { Lt: target }` (or an explicit `IsEmpty`
OR `Range{Lt: target}` composite, mirroring how `activeWindowConditions` already composes
`IsEmpty` with a `Range` for the same "absent-or-below-threshold" shape) — never a single-value
equality match. Test by seeding records at every intermediate version (absent, 0, 1, ..., target-1)
in one collection and asserting a single `engram migrate` invocation walks every one of them to
`target`, not just the absent/0 case.

**Warning signs:** a migration filter or CLI flag that only distinguishes "migrated" vs
"not migrated" as a binary, with no representation of "partially migrated."

**Phase to address:** the migration-mechanism phase, as a design requirement for the eligibility
filter from the first implementation, not a later fix once multi-step chains actually exist.

---

### Pitfall 6: `buf breaking` passing and the code compiling is not evidence the Connect wire actually carries the new fields — this exact bug has already happened three times

**What goes wrong:**
The milestone's own scoping note states the failure mode outright: *"the Connect lane's `Memory`
message has ALWAYS omitted `superseded_by`, `supersedes`, `not_before`, `not_after`, and now
`archived_at`, so three milestones of shipped state (v0.8.x scheduling, v0.11.x supersession, v0.13.x
archive tier) are invisible to BOTH consumers of that lane."* That is: the proto schema was extended,
`buf breaking` passed (the change is additive), the Go code compiled, and CI was green — for three
consecutive milestones — while `memoryToProto` silently never mapped the new struct fields onto the
wire message. Green CI and a clean `buf breaking` run only prove the *schema* is additive; they say
nothing about whether the *mapping function* was updated. Widening the proto again in this milestone
(`schema_version` plus the five already-shipped-but-unwired fields) without changing how field-parity
is verified reproduces the identical bug a fourth time, just with a longer list of omitted fields.

**How to avoid:** do not rely on a hand-maintained, explicit list of "fields to check" in the test (the
`contentFingerprint` precedent shows explicit lists are the right tool when the risk is a *security*
property that must be deliberately, narrowly scoped — but field-parity is the opposite risk: the cost
of *forgetting* to add a field to an explicit list is exactly this bug). Instead, write an exhaustive
round-trip test — reflection over `store.Memory`'s exported, wire-eligible fields (excluding the
`json:"-"` payload-only audit fields, which are deliberately never on the wire), asserting each one
that `memoryToProto`/the proto `Memory` message defines is populated from a fully-populated source
record and decodes back losslessly. A test that must be updated by hand every time a field is added is
the same shape of trap that let this slip three times.

**Warning signs:** a PR that adds proto fields and updates `buf breaking`'s baseline, but whose diff
to `memoryToProto`/`shapeProtoMemories`/the read handlers is smaller than the number of new fields.

**Phase to address:** the Connect record-state parity phase (#482) — the exhaustive round-trip test
must land in the same phase as the field additions, and should be written to fail loudly (not skip)
if a future field is added to `store.Memory` without a corresponding proto mapping.

---

### Pitfall 7: `backfill-short-ids`'s default-apply flag is the exact opposite of the destructive-tier convention it's being folded into

**What goes wrong:**
`cmd/engram/backfill.go` declares `backfillShortIDsCmd.Flags().BoolVar(&backfillDryRun, "dry-run",
false, ...)` — the default is `false`, meaning **a bare `engram backfill-short-ids` invocation applies
by default**; `--dry-run` is the opt-in safety flag. This predates v0.13.x Phase 3's
`registerDestructive` convention, which flipped `prune-expired` and `migrate-remap-owner` to
preview-by-default (`--apply` as the opt-in escape hatch) specifically because default-apply on a
destructive/bulk-write command was judged a footgun worth fixing. The milestone's plan explicitly
folds `backfill-short-ids` into the new versioned `engram migrate`, subsuming it — but subsuming the
command's *behavior* unchanged would import the pre-v0.13.x default-apply footgun straight into a
brand-new command, at the exact moment operators are least familiar with it.

**How to avoid:** `engram migrate` must be registered through `registerDestructive` like every other
bulk-write operator command in this codebase, with `--apply` as the explicit opt-in and preview
(no writes) as the bare-invocation default — not `backfill-short-ids`'s inverted `--dry-run` shape.
This is a deliberate behavior change from `backfill-short-ids`'s current CLI contract, not a
preservation of it, and should be called out explicitly in the upgrade guide (mirroring
`guides/upgrade.md`'s existing pattern for other exit-code/behavior changes) since it changes what a
bare invocation does for anyone with `backfill-short-ids` in a script today.

**Warning signs:** an `engram migrate` implementation that reuses `backfillDryRun`-shaped flag wiring
verbatim, or that describes itself as "same as backfill-short-ids, now versioned" without addressing
the default-action inversion.

**Phase to address:** the migration-mechanism phase, at design time — this is a one-line flag-default
decision, but it must be made deliberately, not inherited by copy-paste.

---

## Moderate Pitfalls

### Pitfall 8: Preview and apply already race independently in this codebase's own destructive-tier pattern — `engram migrate` will inherit the same gap unless addressed

`prune.go`'s `prunePreview`/`pruneApplyRun` each independently call `pruneCutoffNow()` and re-query
Qdrant — there is no shared snapshot between the preview count an operator sees and the apply count
that actually runs. Between the two invocations (which may be minutes apart in an operator's
workflow), a new record can be written, or a concurrent process can change eligibility, so the
applied count can legitimately differ from what was previewed with no bug involved. `engram migrate`
subsuming this shape inherits the same gap. **Prevention:** either accept and *document* this as
advisory-only preview semantics (the honest answer, given no cross-request transaction primitive
exists here) — matching `spine-review purge`'s already-established "gate-passing set intersected with
a fresh re-derivation" pattern rather than trusting a stale preview — or, if exact preview/apply
parity is a requirement for this milestone, that is new scope worth calling out explicitly rather than
silently assuming the existing pattern already provides it. **Phase:** migration-mechanism phase,
CLI/console-surfacing phase (whichever writes the operator-facing preview text).

### Pitfall 9: `#497`'s testcontainer flakiness can mask genuine partial-failure-injection test failures, precisely in the area this milestone most needs to trust

The milestone explicitly orders the gate/CI-integrity phase first because "this milestone authors new
key-links and must be able to trust a red build." The same reasoning applies with more force to the
partial-failure-resume tests Pitfall 4 requires: a test that forces a mid-batch Qdrant failure and
asserts clean resume is *exactly* the kind of test a dying testcontainer will intermittently fail for
unrelated reasons, and a team that has just been burned by real infra flakiness is primed to
wave off a genuine regression as "probably #497 again." **Prevention:** don't author the
partial-failure-injection tests for `engram migrate` until #497 is verified fixed; once it is, run the
new tests in a tight loop (e.g. 20×) before trusting them as a permanent CI gate, the same discipline
`TestBackfillShortIDsResumesAfterMidRunFailure` and 03.1's forced-partial-failure test already model.
**Phase:** gate & CI integrity phase (prerequisite), migration-mechanism phase (consumer).

### Pitfall 10: `#479`'s `pattern:` key-link escaping bug will silently no-op any new key-link this milestone authors that needs `\\`

The milestone's own scope note states Phases 1-2's key-link gates using `pattern:` fields with `\\`
escaping were unmatchable no-ops. This milestone will author new key-links (registry entries for
`schema_version`, `RuleSweepScopeOrAllScopesRequired`, the migration command's conditional rules) in
`internal/surfaces`. Any of these that needs a `\\`-containing pattern (e.g. matching a version-number
regex, a Windows-style path, or an escaped brace in generated prose) will reproduce the identical
silent-no-op bug unless #479 is fixed and *proven* fixed first (a corrupted-region fail-first test,
the same proof `REQ-surface-conformance-gate` already required for the last conformance gate).
**Phase:** gate & CI integrity phase (prerequisite for all subsequent phases that touch
`internal/surfaces`).

### Pitfall 11: `contentFingerprint`'s explicit field list is the right tool for security scoping and the wrong instinct to reach for on `schema_version`

`contentFingerprint` deliberately hashes an explicit field list (not reflection) because it backs a
security-relevant idempotency-replay check, and this repo has already been bitten once by forgetting
to add a new client-authored field to it. `schema_version` is the *opposite* kind of field — it is
server-set/derived (like `EmbedderIdentity`, `IdempotencyFingerprint`), never client-authored, and
must use the `json:"-"` payload-only pattern those fields already establish. The risk is a developer,
having just internalized "new field → must add to contentFingerprint," reflexively adding
`schema_version` to it — which would make idempotency-replay detection depend on which schema version
happened to be current at write time, an unrelated and unintended coupling. **Prevention:** a one-line
code comment at the `schema_version` struct field pointing at the `EmbedderIdentity`/
`IdempotencyFingerprint` precedent (payload-only, `json:"-"`, never in `contentFingerprint`), and a
test asserting `contentFingerprint`'s output is unaffected by `schema_version`. **Phase:**
record-schema-versioning phase.

### Pitfall 12: `not_before`/`not_after` outward-rounding parity between the existing write path and the new Connect read path

The v0.10.x write lane already established "sub-second outward rounding" (D-09) when converting
`*time.Time` window fields across the proto boundary on write. `activeWindowConditions` compares
these fields at epoch-*second* granularity server-side. The new read-path `memoryToProto` mapping
(#482) needs the identical rounding discipline — if the write path rounds outward and the new read
path rounds inward (or doesn't round at all, truncating instead), a record whose window boundary sits
within the same second can appear open on one lane and closed on the other, a client-visible
inconsistency between what `search_memory` shows as active and what the Connect console shows.
**Phase:** Connect record-state parity phase, verified with a boundary-second test against both lanes
simultaneously.

### Pitfall 13: A migration sweep and an in-flight `supersede_memory` call on the same record are safe only if the migration never grabs a stale full-payload read-then-write

`Store.Supersede`'s back-stamp uses a targeted `SetPayload` merge specifically so it composes safely
with other single-key writers (Qdrant's `SetPayload` merges the named keys, leaving others untouched).
`Store.TargetLocker` only covers Supersede's own target set — a migration sweep is a new, independent
caller with no lock coordination with Supersede at all. This composition is safe *as long as* the
migration also writes via a single-key `SetPayload` (Pitfall 3's requirement) and never reads a full
payload map, mutates it in memory, and writes the whole thing back — the latter can silently clobber a
concurrent Supersede back-stamp that landed between the migration's read and its write. **Phase:**
migration-mechanism phase, verified by a concurrent-writer test pairing a migration sweep with an
in-flight `Supersede` call under `-race`, mirroring the existing 03.1 concurrent-update race-gate
precedent for `archive`/`restore`.

### Pitfall 14: A gap between "write-path auto-inject" and "sweep migration ships" leaves new writes version-less too, quietly breaking the "absent = v0, no backfill needed" invariant

The milestone's design is "auto-inject on write, explicit check on read," but these are two separate
code changes that could land in different commits or even different phases. If the write path (every
`store_memory`/`schedule_memory`/`supersede_memory` call site) isn't stamping the current
`schema_version` from the very first commit that introduces the field, there is a window where BOTH
old *and freshly-written* records are version-less — meaning "absent implies v0" quietly stops being
true for new data the moment any migration bumps the target version, since a truly-new record and a
truly-legacy record become indistinguishable by the `schema_version` field alone. **Prevention:** land
write-path auto-injection and the field definition in the same phase/commit, with a test proving 100%
of records written after that commit carry a non-absent `schema_version`, before any migration or read
logic depends on the absent-means-v0 assumption. **Phase:** record-schema-versioning phase.

## Minor Pitfalls

### Pitfall 15: `engram migrate` reaching for a raw `Upsert` "just this once" to also fix up the vector

If a future schema step's payload restructuring changes what `EmbedText` composes (unlikely for a
purely-additive change, but a natural temptation once a migration command exists and someone wants to
"also re-embed while we're in here"), the path of least resistance is a full `Upsert` with a
freshly-embedded vector — which reintroduces Pitfall 3's whole-payload lost-write hazard and blurs the
line between `engram migrate` (schema, payload-only, no embedder client) and `engram reindex`
(vector-rewrite, already has embedder-config-identity machinery for exactly this). **Prevention:**
state explicitly in the migration command's design that it is payload-only by construction (no
embedder dependency, `SetPayload` only); any future need to change vectors as part of a schema bump
routes through `reindex`'s existing machinery instead, never a bespoke `Upsert` inside `migrate`.
**Phase:** migration-mechanism phase (a documentation/type-boundary decision, not new code).

### Pitfall 16: A schema-version-aware filter added to one recall call site but not the other three

`Search`, `List`, `SearchDiscovery`, and `ListScheduled` (plus `spine.go`'s scan) each independently
build their own filter conditions today — the existing code comments already flag this as a known
drift risk ("a SIBLING condition ... never folded into either"). If any schema-version-aware behavior
is ever added to recall (even if Pitfall 1 is otherwise avoided — e.g. an operator-only "show only
records still at v0" flag), it must go through one shared helper reused at every call site, the same
discipline `categoryMatchCondition`/`activeWindowConditions` already establish, rather than being
added ad hoc to whichever function the author happened to be editing. **Phase:** wherever such a
filter is first introduced, if ever — not currently in scope per Pitfall 1's finding that
`schema_version` should never gate recall at all.

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|--------------------|-----------------|------------------|
| Skip the exhaustive proto-field-parity test, rely on manual review | Faster PR | Reproduces the exact 3-milestone #482 bug a 4th time | Never — this is precisely the gap that created #482 |
| Reuse `backfill-short-ids`'s default-apply flag shape for `engram migrate` | No new flag-wiring code | Footgun default reintroduced on a brand-new command | Never |
| Let the migration write via a raw `Upsert` "since it's simpler than SetPayload" | Simpler single-op code | CR-01-class lost-write risk on any concurrent writer | Never in production; acceptable only in a throwaway prototype never pointed at real data |
| Treat schema-version eligibility as a single boolean (`== 0`) instead of a range | Simpler filter, works for a single-step migration | Silently strands records mid-chain once a second migration step ships | Acceptable ONLY if the team commits to never shipping more than one schema bump ever — not realistic; build the range filter from the start |
| Trust the preview count as an exact prediction of apply's outcome | Simpler operator messaging | Operator distrust when preview and apply counts diverge under concurrent writes | Acceptable if explicitly documented as advisory-only in `--help`/docs |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|-------------------|
| Qdrant `SetPayload` (multi-ID) | Trusting a non-error response means every id in the batch is now migrated, or an error response means nothing happened | Re-derive actual state via `Scroll`/`Count` after any batch write (the `reconcileSupersedeFailure` pattern); assume partial application on both success and failure paths |
| Qdrant batch endpoint generally | Assuming HTTP-level atomicity ("400 means nothing was written") | Confirmed non-atomic upstream (qdrant/qdrant#9371): valid ops inside a rejected batch can still persist; design every batch write to be safely re-submittable |
| buf/protobuf field numbers | Treating a clean `buf breaking` run as proof the Go mapping code (`memoryToProto`) was updated | `buf breaking` only proves the *schema* is additive; pair every field addition with an exhaustive field-mapping round-trip test, not a manually-maintained list |
| `internal/surfaces` key-link registry | Authoring a `pattern:` entry with `\\` escaping before #479 is fixed | Verify #479's fix with a fail-first corrupted-region test before authoring any new `\\`-containing pattern |
| Qdrant testcontainer (`#497`) | Treating a flaky test failure as "probably infra" when it's a genuine migration-resume regression | Verify #497 fixed first; then run new partial-failure tests in a tight loop before trusting them as a permanent gate |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|-----------------|
| Unbounded single-batch `SetPayload` call across an entire collection scroll page | Looks fine on small test fixtures (a few hundred records, as in `TestBackfillShortIDs`), silently partial-applies once a batch exceeds Qdrant's internal 32-point chunk boundary in production | Cap batch size at or near `qdrantPayloadOpBatchSize`, and make resume correct regardless of batch size (Pitfall 4) | At the same production scale that already prompted `qdrantPayloadOpBatchSize` to be documented — any collection with more than a few dozen eligible records per sweep |
| Migration progress reporting relies only on a final total, no incremental output | Looks identical to a hang on a large corpus; operator can't tell "slow" from "stuck" | Reuse `BackfillShortIDs`'s cursor-paginated scroll loop shape, and surface periodic progress the way `reindex --resume --dry-run` already sizes work before running | Any collection large enough that a full scroll takes more than a few seconds — no hard number established in this codebase yet, worth a smoke test at realistic production record counts before ship |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Exposing raw `schema_version` on the Connect wire without considering it alongside the existing hint-code/error-envelope conventions | Low — `schema_version` is not itself sensitive, but a mismatched-version rejection surfaced verbatim to a caller (mirroring `resolver.go`'s deliberate choice to NOT disclose session payload-version mismatches on the browser-visible surface) could leak internal migration cadence | If any read path ever rejects (rather than tolerates) a version mismatch, keep the version detail server-log-only, matching the `sessionPayloadVersion` precedent's `slog.WarnContext` + generic client-facing error split |
| Letting a migration sweep run as an unauthenticated/Subject-less operator command (consistent with `spine-review`'s precedent) without confirming it truly has no per-record authz implications | Low in this design (schema bumps are payload-only bookkeeping, not visibility changes) — but worth confirming explicitly, not assuming | State explicitly in the migration-mechanism design that it is the sixth-or-later instance of the existing Subject-less operator tier, never a new authz path — the same discipline `spine-review` already established for `REQ-spine-scan` |

## "Looks Done But Isn't" Checklist

- [ ] **`schema_version` field added to `Memory`:** often missing the negative recall-gate test
      (Pitfall 1) — verify no `Search`/`List`/`SearchDiscovery`/`ListScheduled` filter references the
      key.
- [ ] **`engram migrate` command:** often missing the partial-failure-resume proof (Pitfall 4) —
      verify a forced mid-batch failure test passes reliably (looped, not single-run) against a
      confirmed-stable testcontainer.
- [ ] **Connect proto widening (#482):** often missing the exhaustive field-mapping test (Pitfall 6)
      — verify `buf breaking` passing is NOT the only evidence cited; confirm `memoryToProto` actually
      populates every one of the six new fields with a round-trip assertion.
- [ ] **`backfill-short-ids` subsumption:** often missing the default-apply-to-preview-default flip
      (Pitfall 7) — verify `engram migrate`'s bare invocation does not write.
- [ ] **Version-chain eligibility filter:** often implemented as equality (`== 0`) instead of range
      (`< target`) (Pitfall 5) — verify with intermediate-version fixtures, not just absent/current.
- [ ] **`not_before`/`not_after` on the Connect read path:** often missing the same outward-rounding
      discipline the write path already has (Pitfall 12) — verify a boundary-second test against both
      lanes.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|-----------------|-----------------|
| Recall gate silently narrowed by a `schema_version` condition (Pitfall 1) | LOW if caught by the negative test before ship; HIGH if it reaches production (silent data-invisibility incident, indistinguishable from an empty store without cross-referencing `Count`) | Remove the offending filter condition; add the negative test retroactively; audit logs/metrics for a `result_count` drop coinciding with the deploy |
| A migration step drops out-of-band keys via a raw `Upsert` (Pitfall 3) | MEDIUM–HIGH depending on how long it ran before detection | Identify affected records (compare `access_count`/`created_at` presence against expected), re-derive lost fields where still recoverable (e.g. `citations` from source-of-truth logs), accept permanent loss for fields with no other record (e.g. `idempotency_fingerprint` — falls back to allowing a future replay to appear as a fresh write) |
| A record strands mid-chain after a crash, invisible to an equality-based resume filter (Pitfall 5) | LOW once diagnosed | Fix the filter to a range predicate; the next `engram migrate --apply` run then picks up every stranded record automatically — no manual per-record repair needed if the range fix lands before data grows stale |
| Proto field silently unmapped despite green CI (Pitfall 6, recurrence of #482's own root cause) | LOW to detect (once the exhaustive test exists), MEDIUM to fix (wire the missing mapping, verify no other consumer depended on the old absent-field behavior) | Add the missing `memoryToProto` mapping; re-run the exhaustive round-trip test; audit whether any client code silently depended on the field being absent (unlikely here, since the fields are additive-only) |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|--------------------|----------------|
| 1 — recall gate narrowed by schema_version | record-schema-versioning phase | negative test: no recall filter-builder references `schema_version` |
| 2 — no version-dispatch codec / no rollback path | record-schema-versioning phase (scope decision) | design doc + `--help` text explicitly scope migrations to additive-only |
| 3 — whole-payload Upsert drops out-of-band keys | migration-mechanism phase | round-trip test: every optional payload key survives a migration step byte-identical |
| 4 — Qdrant batch non-atomicity | migration-mechanism phase | forced mid-batch partial-failure test, looped for stability, against reconcile-by-re-derivation logic |
| 5 — equality vs range eligibility filter | migration-mechanism phase | intermediate-version fixture test proves multi-hop resume |
| 6 — proto/Go mapping drift despite green CI | Connect record-state parity phase (#482) | exhaustive field-mapping round-trip test, not a hand-maintained list |
| 7 — backfill's default-apply flag shape | migration-mechanism phase | `engram migrate` bare invocation writes nothing; `guides/upgrade.md` entry documents the behavior change from `backfill-short-ids` |
| 8 — preview/apply TOCTOU | migration-mechanism phase, CLI/console-surfacing phase | explicit advisory-only documentation, or a two-consecutive-run parity test if exact parity is required |
| 9 — testcontainer flakiness masking real failures | gate & CI integrity phase (prerequisite) | #497 verified fixed before authoring partial-failure-injection tests; new tests looped 20× before trusted as a gate |
| 10 — `pattern:` key-link escaping bug | gate & CI integrity phase (prerequisite) | #479 verified fixed via fail-first corrupted-region test before any `\\`-containing pattern is authored |
| 11 — contentFingerprint reflex-add | record-schema-versioning phase | code comment + test proving `schema_version` never affects `contentFingerprint` output |
| 12 — not_before/not_after rounding parity | Connect record-state parity phase | boundary-second test against both the existing write-path rounding and the new read-path mapping |
| 13 — migration vs concurrent Supersede | migration-mechanism phase | concurrent-writer `-race` test pairing a migration sweep with an in-flight `Supersede` |
| 14 — write-path/sweep ordering gap | record-schema-versioning phase | test proving 100% of post-commit writes carry a non-absent `schema_version` |
| 15 — migrate reaching for Upsert to also re-embed | migration-mechanism phase | design doc states payload-only, no embedder dependency; any vector-touching need routes through `reindex` |
| 16 — schema-version filter added at one call site only | wherever first introduced (not currently in scope) | shared helper reused at all five recall/scan call sites, if ever built |

## Sources

- `internal/store/store.go` (this repo, `feat/v0.13` branch) — `payload()`/`fromPayload()`,
  `Search`/`SearchDiscovery`/`List` filter construction, `qdrantPayloadOpBatchSize` doc comment,
  `Store.Supersede`/`reconcileSupersedeFailure`, `Store.BackfillShortIDs` — read directly, HIGH
  confidence (primary source, not inferred)
- `internal/webauth/session.go`, `resolver.go`, `reseal.go` (this repo) — `sessionPayloadVersion`
  precedent, HIGH confidence
- `cmd/engram/backfill.go`, `cmd/engram/prune.go` (this repo) — current flag defaults and
  preview/apply structure, HIGH confidence
- `internal/server/idempotency.go` — `contentFingerprint` explicit-field-list rationale, HIGH
  confidence
- `.planning/PROJECT.md` (this repo) — milestone scope note on #482's 3-milestone recurrence, the
  v0.13.x CR-01 lost-write finding, #479/#497 gate ordering rationale, HIGH confidence
- [Qdrant issue #9371 — "Batch operations not atomic — valid points persisted despite HTTP 400
  error"](https://github.com/qdrant/qdrant/issues/9371) — maintainer confirmation that batch writes
  are not promised atomic and callers must assume partial application, HIGH confidence
- [protobuf.dev — Proto3 Language Guide, "Consequences of Reusing Field Numbers"](https://protobuf.dev/programming-guides/proto3/) — field-number permanence and reservation guidance, HIGH confidence
- [buf.build docs — Breaking change detection, FILE/PACKAGE/WIRE/WIRE_JSON categories](https://buf.build/docs/breaking/) — HIGH confidence
- [protobuf.dev — Proto Best Practices, "Don't Re-use a Tag Number"](https://protobuf.dev/best-practices/dos-donts/) — HIGH confidence
- [Code With Karani — "Your Migration Will Run Twice: Write It That Way"](https://www.codewithkarani.com/blog/why-migrations-must-be-idempotent) — idempotent/resumable backfill design (separate schema-change vs. backfill steps, resumable batched jobs with a progress-marker WHERE clause), MEDIUM confidence (general SQL-migration domain, adapted here to a document-store context)
- [jsonic.io — "JSON Schema Migration Strategy: Versioning & Transforms"](https://jsonic.io/guides/json-migrations) — schema-version-discriminator pattern, migration registry, lazy vs. batch migration, additive-vs-breaking change classification, MEDIUM confidence (closest available analog to engram's flat-payload-codec situation, not an exact match)
- [docs.ditto.live — "Schema Versioning"](https://docs.ditto.live/best-practices/schema-versioning) — schema_version-discriminator vs. separate-collection patterns for breaking changes in a schemaless document store, MEDIUM confidence
- [ArgoCD PR #27664 — dry-run silently applying on an old server](https://github.com/argoproj/argo-cd/pull/27664) — precedent for "preview output actively lying" as a recognized destructive-UX class, MEDIUM confidence (not this codebase's exact failure mode, but the same risk category as Pitfall 8)

---
*Pitfalls research for: Record State & Schema Evolution milestone (`2026-08-12.01`), engram*
*Researched: 2026-08-12*
