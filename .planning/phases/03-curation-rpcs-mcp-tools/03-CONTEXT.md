# Phase 3: Curation RPCs & MCP Tools - Context

**Gathered:** 2026-09-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Seven Connect RPCs land on `EngramService`: `SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`,
`ListRules`, `ListScheduled`, `RelatedMemories`, and `ListTags`. Each wraps a store method that
already exists (`Supersede`, `ArchiveAs`/`RestoreAs`, `ListScheduled`, the `listRules` core,
`RelatedMemories`, `ListTags`) and delegates to the same core function as its MCP tool. Every new
mutating Procedure is CSRF-gated. Four new MCP tools ship with agent-facing guidance:
`archive_memory`, `restore_memory`, `related_memories`, and `list_tags`. No console UI is built in
this phase. Phase 4 consumes these RPCs.

Requirements: RPC-01..06.

**Roadmap open question already settled.** The roadmap flags the Archive/Restore authz gate (a new
`authz.Action` vs `ActionWrite`, CLI bypass) as an open decision for a research pass. Phase 1
decided it and shipped it (01-CONTEXT D-01..D-04, memory `y0bzh06c11`): one Cedar `ActionArchive`,
gated `ArchiveAs`/`RestoreAs` wrappers over the subject-less core, the CLI `spine-review` path
unchanged, owners may archive rules, and a non-owned record returns NotFound. Phase 3 wires those
wrappers. It does not re-decide them.

**Boundary widening from the MCP contracts (decided below):** `SupersedeMemory` gains
`validate_only`, and `list_rules` / `list_scheduled` are widened. Each change is additive and
lands on both lanes.

</domain>

<decisions>
## Implementation Decisions

### MCP exposure to agents
- **D-01:** `archive_memory` and `restore_memory` ship as MCP tools, as RPC-02 states. Agent
  guidance (curating-memory, with curating-spine consistent) says an agent archives or restores only
  after the user agrees in the conversation. This is the consent rule curating-spine already
  applies to every mutation. Frame archive against its siblings: `delete_memory` removes junk,
  `supersede_memory` records a reversal, and archive retires a record that is still true but no
  longer useful, reversibly.
- **D-02:** `related_memories` ships as an MCP tool for **on-demand use only**: when curating
  (dedup before a store, finding what a correction should supersede) or when the user asks. The
  guidance says never call it at session start and never as an automatic follow-up to a search.
  This is the same framing as `search_discovery`.
- **D-03:** `list_tags` ships as an MCP tool for **tag reuse**. The guidance says: before
  `store_memory`, check the existing tags in the scope and reuse one rather than invent a near
  duplicate. The tool also helps an agent choose a `tags` filter for search. It serves the
  zero-junk goal.
- **D-04:** Every new MCP tool ships with its agent guidance in the **same PR**: the
  curating-memory skill, CLAUDE.md §Memory contract, and docs-site `reference/tools.md` and
  `reference/memory-record.md`, each discriminating the tool against its sibling verbs (convention
  `yaj7dqz9qq`). Missing guidance is a verification gap, not a follow-up.
- **D-05:** **No client-tier CLI verbs** in this phase (e.g. `engram related`, `engram tags`,
  archive over Connect). Operator archive/restore already exists via `engram spine-review`. The
  follow-up is tracked as a **GitHub issue**, not a GSD backlog phase.

### Batch and preview on writes
- **D-06:** `ArchiveMemory` / `RestoreMemory` take **`repeated string ids`** (full UUID or
  short_id), and the MCP tools take the same list. The handler runs `ArchiveAs` / `RestoreAs` once
  per id and returns **one outcome per id** (e.g. archived / already-archived / restored /
  not-archived / not_found). The call claims no cross-record atomicity. That is honest because
  each per-id operation is idempotent and reversible. The shape mirrors
  `engram spine-review archive <ids…>`. Phase 4's multi-select archive and its undo toast use this
  directly. — **Reversibility:** one-way — a published request/response shape on Connect and two
  MCP tool schemas.
- **D-07:** An id the caller cannot archive (a shared record they do not own, or a nonexistent id)
  yields a **per-id `not_found`, and the call still succeeds**. Unowned and nonexistent ids are
  indistinguishable (Phase 1 D-04). The whole call fails only on malformed input (empty list,
  over-cap list, malformed id), through the standard `field=<f> hint=<code>` envelope, or on a
  missing auth context.
- **D-08:** `SupersedeMemory` gains **`bool validate_only`**. With it set, the server runs the full
  preflight (ownership, single-live-head, rule rejection, ambiguous short_id), which names every
  offending target exactly as a real call would. It returns the resolved targets and the would-be
  chain **without writing** anything. Phase 4's preview-before-commit dialog uses it, so the
  preview cannot disagree with the commit. A dry run does not consult or record the
  `idempotency_key` replay ledger. It is still a call on a write Procedure, so CSRF applies.
  — **Reversibility:** one-way — a published request field on Connect and MCP.
- **D-09:** `validate_only` lands on **both lanes**: the Connect RPC and the existing MCP
  `supersede_memory` tool. Sean's reasoning: prefer symmetry unless there is a real reason not to.
  The only argument for asymmetry was `fenpnam8ah`'s "MCP inputs unchanged" precedent, and that
  covers relaxing the recall gate, not write preflight. Agent guidance marks `validate_only` as
  optional (useful before a multi-target merge), so it does not become a routine extra round trip.

### ListRules / ListScheduled widening (both lanes)
- **D-10:** `ListRules` with **empty `scopes` returns every readable `rule:*` scope's rules**,
  enumerated server-side like `cross_spine`. The response reports the covered scopes and
  truncation, following the `searched_scopes` / `scopes_truncated` / `scopes_unknown` precedent.
  The same widening applies to MCP `list_rules`. Today an empty `scopes` is a rejection, so the
  change is additive. Explicit `scopes` keep today's contract (validated `rule:repo:` /
  `rule:project:` prefixes, oldest-first, up to 1000 per scope, compact index by default, `full`
  opt-in). — **Reversibility:** one-way — changes a rejection into a success on a published MCP
  tool.
- **D-11:** `ListScheduled` gains **`cross_spine`** (with the same three coverage fields) and an
  **opaque `cursor` / `next_cursor`**, on both the Connect RPC and MCP `list_scheduled`. Nothing
  else changes: owner-only authz, `state` = scheduled (default) / expired / all, the
  `created_after` / `created_before` window, and the existing soft-hide of superseded and archived
  records. Because of that soft-hide, archiving an expired record removes it from the Scheduled
  view, which is the triage flow CUR-04 wants. — **Reversibility:** one-way — new published
  request and response fields on both lanes.

### RelatedMemories / ListTags wire shape
- **D-12:** The proto `RelatedEdge` carries **`EdgeType type` + `oneof evidence { VectorEvidence
  (cosine score), TagEvidence (shared tags with weights, total weight), CitationEvidence (shared
  kind+ref pairs), SupersessionEvidence (direction, depth) }`**. The wire cannot carry
  type-mismatched evidence, and the generated TS gives Phase 5 a discriminated union. The MCP JSON
  stays flat, like the store's `RelatedEdge` struct. The parity test maps between the two shapes.
  — **Reversibility:** one-way — a published proto shape.
- **D-13:** RelatedMemories returns the anchor and its neighbours in the **compact summary view by
  default, with a `full` opt-in**, on both lanes, matching search and list. `truncated` is carried
  through.
- **D-14:** ListTags has **no server-side prefix field**. Phase 5's tag-chip autocomplete filters
  the returned top-N list on the client. When `more` is true, the UI says so honestly (e.g. "showing
  the N most-used tags"). Qdrant Facet has no prefix match on a keyword index, so a server prefix
  would need a text index or a scan. ListTags otherwise keeps the Phase 1 contract: empty scope =
  all readable, recall-visible exact counts, count-descending, default 100, max 1000, and `more`.

### Carried forward (decided earlier, do not re-ask)
- Archive authz: 01-CONTEXT D-01..D-05 (`ActionArchive`, `ArchiveAs`/`RestoreAs`, CLI unchanged,
  rules archivable by owner, not-owned → NotFound).
- RelatedMemories and ListTags semantics: 01-CONTEXT D-06..D-16 (one entry per candidate,
  rarity-weighted tags, citation kind+ref, full supersession chain, cross-spine reach, recall-gated
  non-supersession edges, per-type caps, `k` adjusts only the vector cap).
- Supersede semantics (`xq4jy6yq8n`): multi-target `supersedes` list; preflight → Upsert → one
  multi-ID SetPayload; reject names **every** offending target; `idempotency_key` fingerprint =
  content + target set, with replay checked before preflight; rules cannot be superseded.
  `SupersedeMemory` takes the `store_memory` field set plus `supersedes` and `idempotency_key`
  (RPC-01).
- The MCP recall input schemas stay unchanged with respect to the recall gate (`fenpnam8ah`). None
  of D-10 / D-11 / D-13 relaxes the gate.
- Authz is enforced only in `internal/store` (DEC-cgb). Zero new Go dependencies.
- Success criterion 1: the missing-CSRF-token → `permission_denied` test for each new mutating RPC
  (`SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`) is written first.

### Testing and verification
Governing rules: `m45p2b4bp7` (never test behaviour we do not own), `3p0zsqrhmb` (no tests for
tests, no gates of gates, no struct-shape assertions), and preference `x0krpn67b0` (no manual UAT;
everything code can exercise is an automated test). Between the two rules, **a test's subject is
the behaviour of code we own.**

**What gets tested (behaviour we own; Connect handler + MCP tool tests against a real Qdrant
testcontainer, `djkt37zdax`):**
- **D-15 (first tests, SC1/RPC-05):** for each new write Procedure (`SupersedeMemory`,
  `ArchiveMemory`, `RestoreMemory`), a cookie-lane request without the double-submit token →
  `permission_denied`, **paired with** a valid-token request that gets through the interceptor. The
  pairing makes the rejection non-vacuous. List the procedures explicitly in the test. Never
  iterate `csrfWriteProcedures`: a test driven by the map cannot notice a Procedure missing from
  it. Follow the `connectcsrf_lane_test.go` end-to-end shape. The CSRF test covers the
  `validate_only` path too (it is the same write Procedure).
- **D-16 (authz wiring, the highest-risk defect):** through BOTH lanes, a non-owner archiving or
  restoring a readable shared record gets a per-id `not_found`, and a re-read shows `archived_at`
  unchanged. This catches a handler that calls the subject-less `Archive`/`Restore` instead of
  `ArchiveAs`/`RestoreAs`. Phase 1 already proves the store gate. This test proves the wiring
  (`jvaycjtx3c`: two proofs, authz and wiring). Include anonymous vs authenticated callers, and
  use overlapping scope names so an isolation test cannot pass vacuously (`yr34zp1hqr`).
- **D-17 (batch outcomes):** a mixed id list (owned, already-archived, shared-not-owned,
  nonexistent, a short_id) yields the exact per-id outcome for each id. An empty list, an
  over-cap list, or a malformed id rejects with the `field=<f> hint=<code>` envelope.
- **D-18 (supersede, SC3):** an invalid target set names every offending target for each failure
  class, on both lanes. For `validate_only`: (a) a re-read shows no new record and no
  `superseded_by` stamp; (b) the same inputs committed for real afterwards produce the same
  resolved targets, or the same rejection naming the same targets, so the preview cannot disagree
  with the commit; (c) a dry run with an `idempotency_key`, followed by a real call with that key,
  performs the write and is not a replay (the dry run left the ledger untouched).
- **D-19 (list widenings):** `list_rules` with empty `scopes` returns rules from every readable
  `rule:*` scope and reports coverage. The existing test that asserts an empty `scopes` rejection
  is updated, not duplicated. `list_scheduled` with `cross_spine`: another actor's `shared`
  scheduled record stays invisible, so the deferred-reveal guarantee holds across scopes. Cursor
  pages cover the full set with no duplicates and no gaps. Coverage fields are asserted verbatim.
- **D-20 (lane parity, SC2):** the MCP↔Connect parity tests (`connectapi_parity_test.go`,
  `connectapi_write_parity_test.go`) get rows for all seven capabilities. Each row asserts that the
  same inputs give equivalent results and equivalent rejection envelopes on both lanes, including
  the `validate_only` field and the flat-vs-oneof `RelatedEdge` mapping.
- **D-21 (read shapes):** RelatedMemories and ListTags through Connect return compact vs `full`
  as requested, and never show another actor's private record. Phase 1 already tests the store
  semantics; these tests cover handler wiring and the wire mapping only.

**Success-criteria reinterpretations (sanctioned channel per `m45p2b4bp7`; ROADMAP.md is not
edited):**
- **D-22:** SC2's "each RPC delegates to the same core function as its MCP tool" is verified
  **behaviourally** by D-20's parity rows. The verifier may also confirm the delegation by reading
  the code. No test asserts the call graph or handler structure.
- **D-23:** SC4's "blast-radius annotations and self-describe catalog entry" is satisfied **by
  construction**. A new tool must get a `surfaces.Class` entry because the existing
  `TestToolAnnotationsBothDirections` and the catalog set-equality tests already enforce it, and
  the values are reviewed in the PR. No new test hard-codes per-tool annotation values: that would
  restate the config.
- **D-24:** SC4's "`buf breaking` green, gen regenerated, vendored SPA passes `ui-drift`" is
  satisfied by the existing CI jobs. No new test for these.
- **D-25:** D-04's agent guidance (skill, CLAUDE.md, docs-site) is verified by the verifier and
  reviewer reading it. No doc-presence or grep test. New hint codes are still caught by the
  existing `hintcodedocs_test.go`.
- **D-26:** `connectdescriptor_test.go` (red when the RPC count goes from 12 to 19): **keep** the
  IDEMPOTENCY_UNKNOWN-on-every-method check (our config, and security-relevant: no write RPC may
  be reachable over GET). **Delete** the RPC count/name/type map. **Do not add** field-shape pins
  for the new messages. They restate the `.proto`, `buf breaking` guards wire compatibility, and
  D-15..D-21 prove the new shapes by round trip. The existing read-lane field pins are left alone
  (Phase 3 does not change those messages).

**Not tested (owned by others; documented, not gated):** Connect's HTTP/code mapping, protovalidate
itself, Qdrant Facet exactness or cursor mechanics, buf/codegen output, go-sdk annotation
serialization.

**Non-vacuity and evidence:**
- **D-27:** while executing, confirm each new test goes red against a temporary, uncommitted
  mutation of the guarded code (e.g. drop the procedure from `csrfWriteProcedures`, swap
  `ArchiveAs`→`Archive`, drop the owner-only clause from the cross-spine scheduled filter), then
  restore the code. Record what was mutated in the plan SUMMARY. **No committed harness, patch
  files, or meta-test** (`3p0zsqrhmb` removed exactly that).
- **D-28:** no human UAT items. The phase has no UI. Anything a verifier flags as `human_needed`
  that code can exercise becomes an automated test (`x0krpn67b0`). VALIDATION.md `-run` commands
  are re-resolved against `go test -list`, and evidence counts `--- PASS` lines, never exit status
  (`gfh6q1ack4`, `bsbsvn4hbc`).
- **D-29:** if any new store read path issues a Qdrant call (e.g. cross-spine enumeration for
  ListScheduled/ListRules), widen all four vocabularies in `schemaversion_recallgate_test.go`
  (`ba1st8kzwz`).
- **D-30:** quality gate: `task` (lint + test) green. There is no new concurrency: the batch
  loop is sequential over `ArchiveAs`, whose per-id lock Phase 1 already race-tests
  (`concurrent_gates_test.go`). So no new `-race` requirement and no concurrency tests are added.

### Claude's Discretion
- Whether a duplicated id within one Archive/Restore list is deduplicated or reported once per
  occurrence (whichever it is, test it and document it).
- Exact proto message, field, and enum names, and field numbers (additive; `buf breaking` green).
- The per-id outcome enum's exact values and whether it reuses `store.ArchiveResult.Outcome`
  names.
- The cap on ids per Archive/Restore call (at most the shared 1000 maximum, via
  `rejectOverMaximum`).
- The shape of the `validate_only` response (e.g. the resolved target records plus the would-be
  chain, or the normal response with a `validated` marker), provided it names every resolved
  target and writes nothing.
- The ListRules / ListScheduled coverage field names, if they must differ from list_memory's (the
  default is to reuse the names).
- Blast-radius `Class` for each new tool/RPC in `internal/surfaces` (archive/restore are
  non-destructive and idempotent; the reads are ReadOnly), and each catalog entry.
- Whether Connect `ListScheduled` exposes a `full` knob (the store always reads the full view
  today; see 04-RESEARCH Pitfall 6 in the store comment).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/REQUIREMENTS.md` — RPC-01..06 (and CUR-01..05, which consume this phase)
- `.planning/ROADMAP.md` — Phase 3 goal and success criteria 1–4
- `.planning/notes/console-overhaul-exploration.md` — API gaps that motivated these RPCs
- `.planning/research/ARCHITECTURE.md`, `.planning/research/PITFALLS.md`,
  `.planning/research/CEDAR.md` — lane architecture, CSRF/resume pitfalls, Cedar actions

### Prior phase decisions
- `.planning/phases/01-store-prerequisites/01-CONTEXT.md` — D-01..D-16 (archive authz, related
  edges, ListTags)
- `.planning/phases/02-recall-first-search/02-CONTEXT.md` — hidden-count all-lanes precedent
  (D-01..D-03), `recall_gate_hidden` wire shape

### Store methods being wrapped
- `internal/store/spine.go` — `Archive`/`ArchiveAs`/`Restore`/`RestoreAs`, `ArchiveResult`
- `internal/store/relatedmemories.go` — `RelatedMemories`, `RelatedEdge`, `RelatedResult`
- `internal/store/listtags.go` — `ListTags`, `TagCount`, `recallVisibleFilter`
- `internal/store/store.go` — `ListScheduled` (owner-only, soft-hides superseded and archived),
  `Supersede`

### Lanes, CSRF, parity, annotations
- `proto/engram/v1/engram.proto` — `EngramService` (11 RPCs today)
- `internal/server/connectapi.go` — Connect handlers
- `internal/server/connectcsrf.go` — `csrfWriteProcedures` allowlist
- `internal/server/connectcsrf_test.go`, `internal/server/connectcsrf_lane_test.go` — CSRF test
  pattern
- `internal/server/connectapi_parity_test.go`, `internal/server/connectapi_write_parity_test.go`
  — MCP↔Connect parity rows
- `internal/server/tools.go` — MCP tool arg structs (`supersedeArgs`, `listScheduledArgs`) and
  `deps.*` cores
- `internal/server/rules.go` — `listRulesArgs`, `listRules` core, `validRuleScope`
- `internal/server/toolannotations.go`, `internal/surfaces/toolclass.go` — blast-radius `Class`
- `cmd/engram/catalog.go` — self-describe catalog
- `ui/src/lib/gen/` — generated TS client, committed and checked by the ui-drift gate

### Agent guidance and docs (same-PR surfaces, D-04)
- `skill/engram/skills/curating-memory/SKILL.md`
- `skill/engram/skills/curating-spine/SKILL.md`
- `CLAUDE.md` §Memory contract
- `docs-site/src/content/docs/reference/tools.md`,
  `docs-site/src/content/docs/reference/memory-record.md`,
  `docs-site/src/content/docs/reference/errors.md`

### Client contract (Phase 4 consumer)
- `.claude/skills/engram-connect-client/SKILL.md` — `engram` vs `engramWrite`, CSRF, per-RPC
  table (extend it with the new RPCs)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `deps.*` core functions in `internal/server/tools.go`: the shared entry points that both the
  Connect handlers and the MCP tools call (success criterion 2).
- `store.ArchiveAs` / `RestoreAs` (single id): the batch handler loops these.
- Cross-spine enumeration and the `searched_scopes` / `scopes_truncated` / `scopes_unknown` fields
  from `list_memory` / `search_memory`: reuse them for D-10 / D-11.
- `list_memory`'s opaque cursor mechanism: reuse it for D-11.
- The Phase 2 `hiddencount.go` all-lanes rollout: a template for adding one field across Connect,
  MCP, and CLI.

### Established Patterns
- `csrfWriteProcedures` map plus a positive missing-token → `permission_denied` test for each
  write Procedure.
- A rejected call returns one envelope, `field=<name> hint=<code>: <text>` (`argerror.go`,
  `connecterror.go`), and hint codes are documented in `reference/errors.md`
  (`hintcodedocs_test.go`).
- Blast-radius classes live in one place (`internal/surfaces`), and MCP annotations are derived
  from them.
- Adding a Qdrant recall read widens four vocabularies in `schemaversion_recallgate_test.go`
  (`ba1st8kzwz`). This is relevant only if a new store read path appears. The wrappers here should
  not add one.

### Integration Points
- `EngramService` in `proto/engram/v1/engram.proto` → `task proto:gen` → `gen/go`, `gen/ts`,
  `ui/src/lib/gen` (all committed; `buf breaking` and ui-drift CI).
- MCP tool registration (`registertools_test.go`) and `surfaces_test.go` / `catalog_test.go`,
  which enumerate every surface.

</code_context>

<specifics>
## Specific Ideas

- Sean's default is **lane symmetry**. When a widening is useful on Connect, put it on MCP too
  unless there is a concrete reason not to. Record a Connect-only asymmetry only with that reason
  stated.
- Archive is a consent-gated agent action, not a janitorial one.

</specifics>

<deferred>
## Deferred Ideas

- Client-tier CLI verbs for the new capabilities (`engram related`, `engram tags`, archive/restore
  over Connect). Tracked as GitHub issue #630, not a GSD backlog phase.
- A server-side ListTags prefix search (text/prefix index on `tags`), only if client-side
  filtering over the capped list proves insufficient in Phase 5.

### Reviewed Todos (not folded)
- "Security-review then install the three design/a11y registry skills" belongs to DSYS-03
  (Phase 4), as it did in Phases 1 and 2.

</deferred>

---

*Phase: 03-curation-rpcs-mcp-tools*
*Context gathered: 2026-09-26*
