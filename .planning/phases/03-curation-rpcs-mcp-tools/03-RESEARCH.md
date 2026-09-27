# Phase 3: Curation RPCs & MCP Tools - Research

**Researched:** 2026-09-26
**Domain:** Connect RPC + MCP tool wiring over existing `internal/store` methods (Go, connectrpc, buf, MCP go-sdk)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**MCP exposure to agents**
- D-01: `archive_memory` and `restore_memory` ship as MCP tools (RPC-02). Agent guidance
  (curating-memory, curating-spine consistent) says an agent archives/restores only after the user
  agrees in conversation — the same consent rule curating-spine applies to every mutation. Frame
  archive against siblings: `delete_memory` removes junk, `supersede_memory` records a reversal,
  archive retires a record that is still true but no longer useful, reversibly.
- D-02: `related_memories` ships as an MCP tool for **on-demand use only** — dedup before a store,
  finding what a correction should supersede, or when the user asks. Never at session start, never
  as an automatic follow-up to a search (same framing as `search_discovery`).
- D-03: `list_tags` ships as an MCP tool for **tag reuse** — check existing scope tags before
  `store_memory` and reuse rather than invent a near duplicate; also helps choose a `tags` search
  filter.
- D-04: Every new MCP tool ships with its agent guidance in the **same PR**: curating-memory skill,
  CLAUDE.md §Memory contract, docs-site `reference/tools.md` and `reference/memory-record.md`, each
  discriminating the tool against its sibling verbs (`yaj7dqz9qq`). Missing guidance is a
  verification gap, not a follow-up.
- D-05: **No client-tier CLI verbs** this phase (`engram related`, `engram tags`, archive over
  Connect). Operator archive/restore already exists via `engram spine-review`. Tracked as GitHub
  issue #630, not a GSD backlog phase.

**Batch and preview on writes**
- D-06: `ArchiveMemory`/`RestoreMemory` take **`repeated string ids`** (full UUID or short_id); MCP
  tools take the same list. The handler runs `ArchiveAs`/`RestoreAs` once per id and returns **one
  outcome per id** (archived / already-archived / restored / not-archived / not_found). No
  cross-record atomicity claimed — each per-id operation is idempotent and reversible. Mirrors
  `engram spine-review archive <ids…>`. Phase 4's multi-select archive/undo toast uses this
  directly. Reversibility: one-way (published Connect request/response shape + two MCP schemas).
- D-07: An id the caller cannot archive (unowned shared record, or nonexistent) yields a per-id
  `not_found`, and **the call still succeeds**. Unowned and nonexistent are indistinguishable
  (Phase 1 D-04). The whole call fails only on malformed input (empty list, over-cap list, malformed
  id) via `field=<f> hint=<code>`, or a missing auth context.
- D-08: `SupersedeMemory` gains **`bool validate_only`**. With it set, the server runs the full
  preflight (ownership, single-live-head, rule rejection, ambiguous short_id) — names every
  offending target exactly as a real call would — and returns the resolved targets + the would-be
  chain **without writing**. Phase 4's preview-before-commit dialog uses it, so preview cannot
  disagree with commit. A dry run does not consult or record the `idempotency_key` replay ledger.
  It is still a call on a write Procedure, so CSRF applies. Reversibility: one-way (published
  request field on Connect + MCP).
- D-09: `validate_only` lands on **both lanes** (Connect RPC + MCP `supersede_memory`). Sean:
  prefer symmetry unless there is a real reason not to; the only asymmetry precedent
  (`fenpnam8ah`, MCP recall inputs unchanged) covers relaxing the recall gate, not write preflight.
  Agent guidance marks it optional (useful before a multi-target merge) so it never becomes a
  routine extra round trip.

**ListRules / ListScheduled widening (both lanes)**
- D-10: `ListRules` with **empty `scopes` returns every readable `rule:*` scope's rules**,
  enumerated server-side like `cross_spine`; response reports covered scopes + truncation
  (`searched_scopes`/`scopes_truncated`/`scopes_unknown` precedent). Same widening on MCP
  `list_rules`. Today empty `scopes` is a rejection, so this is additive. Explicit `scopes` keep
  today's contract (validated `rule:repo:`/`rule:project:` prefixes, oldest-first, up to 1000 per
  scope, compact index by default, `full` opt-in). Reversibility: one-way (rejection → success on a
  published MCP tool).
- D-11: `ListScheduled` gains **`cross_spine`** (same three coverage fields) and an **opaque
  `cursor`/`next_cursor`**, on both Connect RPC and MCP `list_scheduled`. Nothing else changes:
  owner-only authz, `state` = scheduled (default)/expired/all, `created_after`/`created_before`
  window, existing soft-hide of superseded and archived records. Because of that soft-hide,
  archiving an expired record removes it from the Scheduled view (CUR-04's triage flow).
  Reversibility: one-way (new published fields both lanes).

**RelatedMemories / ListTags wire shape**
- D-12: The proto `RelatedEdge` carries **`EdgeType type` + `oneof evidence { VectorEvidence
  (cosine score), TagEvidence (shared tags with weights, total weight), CitationEvidence (shared
  kind+ref pairs), SupersessionEvidence (direction, depth) }`**. The wire cannot carry
  type-mismatched evidence; generated TS gives Phase 5 a discriminated union. MCP JSON stays flat,
  like the store's `RelatedEdge` struct. The parity test maps between the two shapes.
  Reversibility: one-way (published proto shape).
- D-13: RelatedMemories returns anchor + neighbours in **compact summary view by default, `full`
  opt-in**, on both lanes, matching search and list. `truncated` carried through.
- D-14: ListTags has **no server-side prefix field**. Phase 5's tag-chip autocomplete filters the
  returned top-N list client-side. When `more` is true, the UI says so honestly. Qdrant Facet has
  no prefix match on a keyword index; a server prefix would need a text index or a scan. Otherwise
  keeps the Phase 1 contract: empty scope = all readable, recall-visible exact counts,
  count-descending, default 100, max 1000, and `more`.

**Carried forward (decided earlier, do not re-ask)**
- Archive authz: 01-CONTEXT D-01..D-05 (`ActionArchive`, `ArchiveAs`/`RestoreAs`, CLI unchanged,
  rules archivable by owner, not-owned → NotFound). **This phase wires those wrappers; it does not
  re-decide them.**
- RelatedMemories and ListTags semantics: 01-CONTEXT D-06..D-16 (one entry per candidate,
  rarity-weighted tags, citation kind+ref, full supersession chain, cross-spine reach, recall-gated
  non-supersession edges, per-type caps, `k` adjusts only the vector cap).
- Supersede semantics (`xq4jy6yq8n`): multi-target `supersedes` list; preflight → Upsert → one
  multi-ID SetPayload; reject names **every** offending target; `idempotency_key` fingerprint =
  content + target set, replay checked before preflight; rules cannot be superseded.
  `SupersedeMemory` takes the `store_memory` field set plus `supersedes` and `idempotency_key`
  (RPC-01).
- MCP recall input schemas stay unchanged with respect to the recall gate (`fenpnam8ah`). None of
  D-10/D-11/D-13 relaxes the gate.
- Authz is enforced only in `internal/store` (DEC-cgb). Zero new Go dependencies.
- Success criterion 1: the missing-CSRF-token → `permission_denied` test for each new mutating RPC
  (`SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`) is written first.

**Testing and verification** (governing rules `m45p2b4bp7`, `3p0zsqrhmb`, `x0krpn67b0` — a test's
subject is the behaviour of code we own):
- D-15 (first tests, SC1/RPC-05): for each new write Procedure, a cookie-lane request without the
  double-submit token → `permission_denied`, **paired with** a valid-token request that gets
  through. List the procedures explicitly — never iterate `csrfWriteProcedures` itself. Follow
  `connectcsrf_lane_test.go`'s shape. Covers `validate_only` too (same write Procedure).
- D-16 (authz wiring — highest-risk defect): through BOTH lanes, a non-owner archiving/restoring a
  readable shared record gets a per-id `not_found`, and a re-read shows `archived_at` unchanged.
  Catches a handler calling subject-less `Archive`/`Restore` instead of `ArchiveAs`/`RestoreAs`.
  Include anonymous vs authenticated callers, overlapping scope names (no vacuous isolation pass).
- D-17 (batch outcomes): a mixed id list (owned, already-archived, shared-not-owned, nonexistent,
  a short_id) yields the exact per-id outcome for each. Empty list, over-cap list, or malformed id
  rejects with `field=<f> hint=<code>`.
- D-18 (supersede, SC3): invalid target set names every offending target per failure class, both
  lanes. For `validate_only`: (a) re-read shows no new record, no `superseded_by` stamp; (b) same
  inputs committed for real afterwards produce the same resolved targets or same rejection naming
  the same targets; (c) dry run with `idempotency_key` then a real call with that key performs the
  write, not a replay (dry run leaves the ledger untouched).
- D-19 (list widenings): `list_rules` empty `scopes` returns every readable `rule:*` scope +
  coverage (existing empty-`scopes`-rejection test updated, not duplicated). `list_scheduled`
  `cross_spine`: another actor's `shared` scheduled record stays invisible (deferred-reveal holds
  cross-scope). Cursor pages cover the full set, no dupes/gaps. Coverage fields asserted verbatim.
- D-20 (lane parity, SC2): `connectapi_parity_test.go`/`connectapi_write_parity_test.go` get rows
  for all seven capabilities — same inputs give equivalent results and rejection envelopes both
  lanes, including `validate_only` and the flat-vs-oneof `RelatedEdge` mapping.
- D-21 (read shapes): RelatedMemories/ListTags via Connect return compact vs `full` as requested,
  never show another actor's private record. Only handler wiring + wire mapping are new coverage
  (store semantics already tested Phase 1).
- D-22: SC2's "delegates to the same core function" is verified **behaviourally** by D-20's parity
  rows; verifier may also confirm by reading code. No test asserts the call graph/handler structure.
- D-23: SC4's "blast-radius annotations and self-describe catalog entry" satisfied **by
  construction**: `TestToolAnnotationsBothDirections` + catalog set-equality tests already enforce
  a `surfaces.Class` row exists for every registered tool. No new test hard-codes per-tool values.
- D-24: SC4's "`buf breaking` green, gen regenerated, ui-drift passes" satisfied by existing CI jobs.
  No new test.
- D-25: D-04's agent guidance verified by verifier/reviewer reading it. No doc-presence/grep test.
  New hint codes still caught by `hintcodedocs_test.go`.
- D-26: `connectdescriptor_test.go` (red when RPC count 12→19): **keep** the
  IDEMPOTENCY_UNKNOWN-on-every-method check. **Delete** the RPC count/name/type map. **Do not add**
  field-shape pins for the new messages — `buf breaking` guards wire compatibility, D-15..D-21 prove
  the new shapes by round trip. Existing read-lane field pins untouched.
- D-27: while executing, confirm each new test goes red against a temporary, uncommitted mutation of
  the guarded code (e.g. drop the procedure from `csrfWriteProcedures`, swap `ArchiveAs`→`Archive`,
  drop the owner-only clause from cross-spine scheduled filter), then restore. Record the mutation
  in the plan SUMMARY. No committed harness/patch files/meta-test.
- D-28: no human UAT items (no UI this phase). Anything code can exercise becomes automated
  (`x0krpn67b0`). `VALIDATION.md -run` commands re-resolved against `go test -list`; evidence counts
  `--- PASS` lines, never exit status.
- D-29: if any new store read path issues a Qdrant call (e.g. cross-spine enumeration for
  ListScheduled/ListRules), widen all four vocabularies in `schemaversion_recallgate_test.go`.
- D-30: quality gate `task` (lint + test) green. No new concurrency (sequential batch loop over
  `ArchiveAs`, whose per-id lock Phase 1 already race-tests) — no new `-race` requirement.

### Claude's Discretion
- Whether a duplicated id within one Archive/Restore list is deduplicated or reported once per
  occurrence (whichever it is, test it and document it).
- Exact proto message, field, and enum names, and field numbers (additive; `buf breaking` green).
- The per-id outcome enum's exact values and whether it reuses `store.ArchiveResult.Outcome` names.
- The cap on ids per Archive/Restore call (at most the shared 1000 maximum, via
  `rejectOverMaximum`).
- The shape of the `validate_only` response (e.g. resolved target records + would-be chain, or the
  normal response with a `validated` marker), provided it names every resolved target and writes
  nothing.
- The ListRules/ListScheduled coverage field names, if they must differ from list_memory's (default
  is to reuse the names).
- Blast-radius `Class` for each new tool/RPC in `internal/surfaces` (archive/restore are
  non-destructive and idempotent; the reads are ReadOnly), and each catalog entry.
- Whether Connect `ListScheduled` exposes a `full` knob (the store always reads the full view
  today).

### Deferred Ideas (OUT OF SCOPE)
- Client-tier CLI verbs for the new capabilities (`engram related`, `engram tags`, archive/restore
  over Connect). Tracked as GitHub issue #630, not a GSD backlog phase.
- A server-side ListTags prefix search (text/prefix index on `tags`), only if client-side filtering
  over the capped list proves insufficient in Phase 5.
- "Security-review then install the three design/a11y registry skills" belongs to DSYS-03
  (Phase 4), as it did in Phases 1 and 2.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RPC-01 | `SupersedeMemory` on Connect with `store_memory` field set + `supersedes` + `idempotency_key`, delegating to the same core the MCP tool calls, rejecting an invalid target set with every offender named | `deps.supersedeMemory` (`internal/server/tools.go:2594-2722`) already implements the full preflight/store delegation for MCP; this phase adds the Connect handler + `validate_only` (D-08/D-09) and the `SupersedeMemoryRequest`/`Response` proto messages. See Architecture Patterns Pattern 1, Code Examples 1 |
| RPC-02 | `ArchiveMemory`/`RestoreMemory` on Connect and as MCP tools, stamping/clearing `archived_at` on an owned record, never deleting, reversible | `store.ArchiveAs`/`RestoreAs` (`internal/store/spine.go:826-977`) already exist and are owner-gated via `authz.ActionArchive` (Phase 1). This phase adds a batch `deps.archiveMemory`/`restoreMemory` core, the Connect RPCs, and the MCP tools. See Pattern 2, Pitfall 1 |
| RPC-03 | `ListRules`/`ListScheduled` on Connect with the same contracts as the MCP tools, any widening decided and recorded | `deps.listRules` (`internal/server/rules.go:195-245`) and `deps.listScheduled` (`internal/server/tools.go:1890`) already exist for MCP; this phase adds Connect handlers plus D-10 (empty-scopes enumeration) / D-11 (`cross_spine` + cursor) widening on both lanes. See Pattern 3 |
| RPC-04 | `RelatedMemories`/`ListTags` on Connect and as MCP tools, wrapping STORE-02/STORE-03 | `store.RelatedMemories` (`internal/store/relatedmemories.go:706-759`) and `store.ListTags` (`internal/store/listtags.go:99-128`) are pure store methods with no `deps.*` wrapper yet. This phase adds both wrappers, both lanes, plus the `RelatedEdge` oneof proto design (D-12). See Pattern 4, Pitfall 4 |
| RPC-05 | Every new mutating Procedure in `csrfWriteProcedures`, routed through `engramWrite`, proven by a positive test per RPC; every new RPC has a parity-test row | `csrfWriteProcedures` (`internal/server/connectcsrf.go:33-40`) is a literal map keyed on generated Procedure constants — adding `SupersedeMemory`/`ArchiveMemory`/`RestoreMemory` is a one-line-per-RPC change. Test pattern in `connectcsrf_lane_test.go`. See Pitfall 2, Validation Architecture |
| RPC-06 | All proto changes additive, `buf breaking` green, gen regenerated + committed, ui-drift passes, blast-radius + catalog entries | Proto is additive-only by construction (new messages + new RPCs, no field renumbering). `internal/surfaces/toolclass.go` registry + `annotationsFor` (`internal/server/toolannotations.go`) already enforce "every registered tool has a Class row" by construction (D-23). See Pitfall 3, Pitfall 5 |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **VCS/PR flow:** git, branch + PR, never push to `main` directly. Conventional Commits; PR titles
  CI-validated.
- **Task runner:** `task` = lint + test. `task proto:lint` / `task proto:gen` for buf; the generated
  `gen/` tree is committed and CI-checked for drift (`buf` job).
- **License headers:** every in-scope `.go`/`.md` file carries the Apache-2.0 SPDX header
  (`task license:check`/`license:add`), scoped by `.licenserc.yaml` — **never hand-authored**. Do
  **not** add one to any file whose first line must be `---` YAML frontmatter or to `skill/**/SKILL.md`.
- **Lint/format:** `task lint` (golangci-lint, yamlfmt, actionlint, rumdl) and `task fmt` (gofmt,
  dprint, yamlfmt) must be clean.
- **Migrations:** irrelevant to this phase (no schema/payload version change; this phase only adds
  RPCs/tools over existing payload keys `archived_at`/`superseded_by`/`tags`).
- **Memory contract:** `store_rule`/`list_rules`/`archive_memory`/`restore_memory` etc. described
  in CLAUDE.md must be kept current — D-04 requires the same-PR CLAUDE.md §Memory contract update
  for every new MCP tool.
- **Not used here:** viper, cocogitto. No new Go dependency is expected or permitted without a
  package-legitimacy check (none is needed this phase — see Package Legitimacy Audit).
- **Issue tracking:** GitHub Issues, not markdown TODOs; durable project memory via engram MCP, not
  `MEMORY.md`.
- **Session completion:** run `task` (lint+test) before ship; branch + PR; Conventional Commits;
  file GitHub issues for remaining work (e.g. any residual RPC-lane asymmetry deliberately deferred).

## Summary

Phase 3 is a **pure wiring phase**: every store-level primitive it needs (`ArchiveAs`/`RestoreAs`,
`RelatedMemories`, `ListTags`, `Supersede`, `ListScheduled`, `listRules`) already exists and is
already authz-correct — Phase 1 and the pre-existing MCP surface did that work. What Phase 3 adds is
seven Connect RPCs, four new MCP tools (`archive_memory`, `restore_memory`, `related_memories`,
`list_tags`), the CSRF allowlist entries for the three new *write* Procedures
(`SupersedeMemory`/`ArchiveMemory`/`RestoreMemory`), the additive proto messages/RPCs, the
`surfaces.Class` blast-radius rows, and the MCP↔Connect parity test rows. The codebase's own
established pattern — visible in every prior write RPC (`StoreMemory`, `UpdateMemory`, …) and every
prior read RPC (`ListMemories`, `SearchMemories`) — is completely mechanical: a thin `engramAPI`
method resolves the caller via `callerFromConnectContext`, converts the proto request to the
existing `*Args` struct (a `protoconv.go` function), calls the **same** `deps.*` method the MCP tool
already calls (or a new `deps.*` method this phase adds, shared by both), converts the result back
to proto, and maps any error through the single `connectError` mapper. Two pieces of genuine design
work exist inside that mechanical shape: (1) `ArchiveMemory`/`RestoreMemory` and their MCP tools need
a **new batch core** (`deps.archiveMemory(ctx, c, ids []string)`) that loops `store.ArchiveAs` once
per id and assembles a per-id outcome list — no store-level batch primitive exists, by design (D-06,
D-07); and (2) `RelatedMemories`'s wire shape needs a **new proto oneof** (`RelatedEdge.evidence`)
because the store's `RelatedEdge` Go struct is a flat union-by-omission that a hand-typed protobuf
message must not copy verbatim (D-12). The Archive/Restore authz-gate question the ROADMAP flagged as
open ("a new `authz.Action` vs reusing `ActionWrite`; whether the CLI keeps its subject-less bypass")
is **already answered** by Phase 1 (D-01..D-05 there): `authz.ActionArchive` exists
(`internal/authz/authz.go:32`), `ArchiveAs`/`RestoreAs` already gate on it, and the CLI's
`spine-review archive/restore` already calls the unchanged subject-less `Archive`/`Restore`. This
phase's task is to **wire** those wrappers behind Connect + MCP, not to make that design decision.

**Primary recommendation:** treat every new RPC as "one more row in the existing thin-adapter
pattern" and resist inventing a second pattern. Build the batch archive/restore core and the
`RelatedEdge` oneof first (the two genuinely new pieces), then mechanically repeat the
proto→protoconv→deps→handler→MCP-tool→annotation→parity-row chain for each of the seven RPCs. Write
the CSRF-rejection test for each new write RPC before writing the handler (SC1, D-15) — the codebase
already has the exact test shape in `connectcsrf_lane_test.go` to copy.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Authz gate (owner-only archive/restore, supersede target ownership) | Database/Storage (`internal/store`) | — | DEC-cgb: authz is enforced ONLY in `internal/store`; already true for `ArchiveAs`/`RestoreAs`/`Supersede`/`RelatedMemories`/`ListTags` (Phase 1). This phase adds no new authz logic, only callers of it |
| Batch-outcome assembly (per-id archive/restore result list) | API/Backend (`internal/server` `deps.*`) | — | No store-level batch primitive; the loop-and-collect logic belongs in the shared `deps.*` core so both lanes get it identically (SC2) |
| CSRF double-submit enforcement | API/Backend (`internal/server/connectcsrf.go`) | — | `csrfWriteProcedures` is a Connect-interceptor-level allowlist; MCP has no analogous exposure (MCP tools are not reachable over the browser CSRF threat model) |
| Wire shape / oneof discrimination (`RelatedEdge`) | API/Backend (proto + protoconv) | — | The store's Go struct is intentionally flat (union-by-omission); the *wire* contract is where type-safety is added, per D-12 |
| Proto codegen (`gen/go`, `gen/ts`, `ui/src/lib/gen`) | CDN/Static (build artifact) | API/Backend (source of truth: `.proto`) | Generated, committed, CI-checked for drift; not hand-edited except `ui/src/lib/gen/engram_pb.ts` (see Pitfall 5) |
| Blast-radius classification / self-describe catalog | API/Backend (`internal/surfaces`) | CLI (`cmd/engram/catalog.go`, unaffected this phase — no new CLI verbs) | Single shared registry read by both the MCP `annotationsFor` and the CLI catalog; this phase only adds rows, no new reader |
| MCP tool consent framing (archive/restore/related/list_tags agent guidance) | API/Backend (skill + docs, D-04) | — | Not a runtime tier; a documentation/skill deliverable that must ship in the same PR as the tool |

## Standard Stack

No new external library is introduced by this phase. Every dependency Phase 3 touches is already
in `go.mod`/`buf.gen.yaml` and already used by the sibling RPCs this phase mirrors.

### Core (already in use, unchanged versions)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `connectrpc.com/connect` | pinned in `go.mod` (unchanged) | Connect RPC transport/interceptors | Already the sole RPC transport; every existing handler in `internal/server/connectapi.go` is built on it |
| `buf.build/go/protovalidate` | pinned in `go.mod` (unchanged) | Request-shape validation via buf.validate CEL rules on new proto messages | Already wired as the `newConnectValidateInterceptor` in `mountConnect` (`internal/server/connectapi.go:553-576`) |
| `github.com/modelcontextprotocol/go-sdk/mcp` | pinned in `go.mod` (unchanged) | MCP tool registration (`mcp.AddTool`), `mcp.ToolAnnotations` | Already the sole MCP tool-registration mechanism (`internal/server/tools.go`) |
| `github.com/cedar-policy/cedar-go` | pinned in `go.mod` (unchanged) | Authz PDP (`authz.ActionArchive` already defined) | Already the sole authz engine (Phase 1); no new action needed this phase |
| `buf` (build tool, not a Go module dep) | 1.73.0 confirmed installed `[VERIFIED: local shell — buf --version]` | proto lint/gen/breaking-change detection | Project's sole codegen/compat tool (`task proto:lint`/`proto:gen`, CI `buf` job) |

### Supporting
None new. This phase reuses `store.MaxRecallLimit`/`rejectOverMaximum` (`internal/store/store.go:1654,1663`), the existing `argError`/`HintCode` vocabulary (`internal/server/argerror.go`), and the existing `scopeCoverage`/`recallHidden` wire-shape helpers (`internal/server/tools.go:2054-2110`, `internal/server/hiddencount.go:202`).

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| A hand-rolled batch atomicity guarantee for Archive/Restore | A Qdrant multi-point SetPayload (like `Supersede`'s back-stamp) | Rejected by D-06/D-07 explicitly: "the call claims no cross-record atomicity" — the sequential per-id `ArchiveAs` loop is correct-by-design here, not a shortcut |
| A flat `RelatedEdge` proto message (mirroring the Go struct 1:1) | The `oneof evidence` design (D-12) | Flat would let a client read `SharedTags` on a vector-typed edge (a wire bug class); oneof makes it structurally impossible and gives Phase 5's TS client a discriminated union for free |

**Installation:** none — no `go get` / `npm install` needed. Proto regeneration only:
```bash
task proto:lint
task proto:gen
```

**Version verification:** `buf --version` confirmed `1.73.0` installed and on PATH
`[VERIFIED: local shell]`. Go toolchain confirmed `go1.27.1` `[VERIFIED: local shell]`, matching
`go.mod`'s `go 1.26.7` directive (toolchain is newer, which is compatible). No package registry
lookup is needed since no new package is added.

## Package Legitimacy Audit

**Not applicable — zero new external packages this phase.** Per CONTEXT.md's carried-forward
decision ("Authz is enforced only in `internal/store` (DEC-cgb). Zero new Go dependencies.") and the
milestone's own standing constraint, this phase adds no new `go.mod` entries and no new npm
packages. `go.mod`'s existing dependency set (`connectrpc.com/connect`, `buf.build/go/protovalidate`,
`github.com/modelcontextprotocol/go-sdk`, `github.com/cedar-policy/cedar-go`, `github.com/qdrant/go-client`)
is unchanged.

**Packages removed due to `[SLOP]` verdict:** none.
**Packages flagged as suspicious `[SUS]`:** none.

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────────────────┐
                         │           Caller (agent or SPA)           │
                         └───────────────┬───────────────┬───────────┘
                                          │               │
                              MCP stdio/HTTP        Connect RPC (HTTP/JSON or gRPC)
                                          │               │
                                          ▼               ▼
                         ┌────────────────────┐  ┌──────────────────────────┐
                         │  mcp.AddTool(...)   │  │  engramAPI (connectapi)   │
                         │  arg struct decode   │  │  callerFromConnectContext │
                         │  callerFromContext   │  │  protoconv: Req -> *Args  │
                         └─────────┬──────────┘  └────────────┬─────────────┘
                                   │                            │
                                   │      BOTH call the SAME     │
                                   └──────────► deps.* core ◄────┘
                                        (deps.archiveMemory,
                                         deps.restoreMemory,
                                         deps.relatedMemories,
                                         deps.listTagsCore,
                                         deps.supersedeMemory [existing],
                                         deps.listScheduled [existing],
                                         deps.listRules [existing])
                                                  │
                              validate args, resolve short_id -> UUID,
                              authorize (delegated), assemble batch outcomes
                                                  │
                                                  ▼
                                     internal/store.* (Phase 1, unchanged)
                                     ArchiveAs / RestoreAs / RelatedMemories /
                                     ListTags / Supersede / ListScheduled / List
                                                  │
                                                  ▼
                                              Qdrant

  Connect-only extra hop (write RPCs only):
  engramAPI request ──► newConnectCSRFInterceptor (csrfWriteProcedures allowlist)
                        ──► newConnectValidateInterceptor (buf.validate)
                        ──► engramAPI.<Method>
```

Trace the primary use case (an operator archives a shared-but-unowned record they can read) through
the diagram: SPA → Connect `ArchiveMemory` → CSRF interceptor checks `csrfWriteProcedures[...]` →
validate interceptor checks proto shape → `engramAPI.ArchiveMemory` → `callerFromConnectContext` →
`deps.archiveMemory(ctx, c, ids)` → loop calling `store.ArchiveAs(ctx, id, subj)` per id →
`getWritable(..., authz.ActionArchive)` returns `ErrNotFound` for the unowned record → the batch
core records `not_found` for that id and `changed`/`already` for the rest → the RPC still succeeds
(200/OK) with a per-id outcome list, per D-07.

### Recommended Project Structure

No new files are structurally required; this phase extends existing files following their existing
per-capability grouping:

```
proto/engram/v1/engram.proto        # + 7 new RPCs, + their Request/Response messages,
                                     #   + RelatedEdge/TagCount-shaped messages (additive only)
internal/server/
├── tools.go                        # + archiveArgs/restoreArgs, deps.archiveMemory/restoreMemory,
│                                    #   deps.relatedMemories, deps.listTagsCore (new); MCP
│                                    #   registrations for archive_memory/restore_memory/
│                                    #   related_memories/list_tags added to registerTools
├── connectapi.go                   # + SupersedeMemory/ArchiveMemory/RestoreMemory/ListRules/
│                                    #   ListScheduled/RelatedMemories/ListTags handlers (thin
│                                    #   adapters, mirroring the six write RPCs' existing shape)
├── connectcsrf.go                  # + 3 entries in csrfWriteProcedures (Supersede/Archive/Restore)
├── protoconv.go                    # + *RequestToArgs / *ResponseFrom converters for the 7 new RPCs
├── rules.go                        # unchanged (deps.listRules already exists) — Connect wrapper
│                                    #   lives in connectapi.go, per the existing pattern
├── connectcsrf_lane_test.go        # + 3 new csrfWriteCases entries (or sibling test per D-15)
├── connectapi_parity_test.go       # + rows for ListRules/ListScheduled/RelatedMemories/ListTags
├── connectapi_write_parity_test.go # + t.Run subtests for SupersedeMemory/ArchiveMemory/
│                                    #   RestoreMemory (D-20)
└── connectdescriptor_test.go       # RPC-count assertion updated 12→19 (D-26); RPC name/type map
                                     #   DELETED per D-26, not extended
internal/surfaces/toolclass.go      # + Operation rows: supersede_memory (annotations only, tool
                                     #   already exists — no row change needed), archive_memory,
                                     #   restore_memory, related_memories, list_tags
gen/go/…, gen/ts/…, ui/src/lib/gen/ # regenerated via task proto:gen; committed
skill/engram/skills/curating-memory/SKILL.md   # + agent guidance for the 4 new tools (D-04)
CLAUDE.md                                       # + §Memory contract updates (D-04)
docs-site/src/content/docs/reference/tools.md,
docs-site/src/content/docs/reference/memory-record.md  # + new tool docs (D-04)
```

### Pattern 1: Thin Connect adapter delegating to a shared `deps.*` core (SC2, RPC-01..04)

**What:** Every Connect handler in `internal/server/connectapi.go` follows one shape: resolve caller
→ convert request → call `deps.*` (the SAME method the MCP closure calls) → convert result → map
error through `connectError`. `internal/server/connectapi.go:452-457` states this explicitly as the
pattern for the six existing write RPCs; this phase's new RPCs must follow it identically.

**When to use:** Every new RPC in this phase, without exception — including the "read" RPCs
(ListRules/ListScheduled/RelatedMemories/ListTags), which get the identical shape minus CSRF.

**Example (existing `UpdateMemory`, the shape to copy):**
```go
// Source: internal/server/connectapi.go:486-496 [VERIFIED: internal/server/connectapi.go:486-496]
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
`SupersedeMemory` slots into this exact shape: `a.d.supersedeMemory(ctx, c, supersedeMemoryRequestToArgs(req.Msg))`
already exists as `deps.supersedeMemory` (`internal/server/tools.go:2594`); only the protoconv
function and the handler are new.

### Pattern 2: New batch core with per-id outcomes (RPC-02, D-06/D-07)

**What:** `store.ArchiveAs`/`RestoreAs` are single-id methods (`internal/store/spine.go:826-977`, no
batch primitive exists there by design — see D-06's "the call claims no cross-record atomicity").
The new `deps.archiveMemory`/`deps.restoreMemory` core loops over the caller's `ids []string`,
resolves each via `ResolvePointID` (the existing short_id-or-UUID resolver every other verb uses,
e.g. `resolveAndAuthorizeSupersedeTargets`, `internal/server/tools.go:2452`), calls `ArchiveAs`/
`RestoreAs` per resolved id, and assembles one outcome per **caller-supplied token** (not per
resolved id — D-06 says "one outcome per id" using the caller's own input, mirroring
`ArchiveResult.Requested` at `internal/store/spine.go:745`, which already exists precisely for this
purpose: *"Requested is the token the caller actually supplied... so a caller can correlate each row
back to its input."*).

**When to use:** `ArchiveMemory`/`RestoreMemory` on both lanes (Connect + the two new MCP tools).

**Example (the store-level per-id outcome type this core must plumb through unchanged):**
```go
// Source: internal/store/spine.go:709-747 [VERIFIED: internal/store/spine.go:709-747]
type ArchiveOutcome string

const (
	ArchiveOutcomeChanged  ArchiveOutcome = "changed"
	ArchiveOutcomeAlready  ArchiveOutcome = "already"
	ArchiveOutcomeNotFound ArchiveOutcome = "not_found"
)

type ArchiveResult struct {
	ID        string
	Requested string
	Outcome   ArchiveOutcome
}
```
A resolution failure (unresolvable short_id) never reaches `ArchiveAs` at all — the batch core must
synthesize its own `ArchiveResult{Requested: token, Outcome: ArchiveOutcomeNotFound}` row for that
case too, so a caller cannot distinguish "resolution failed" from "record not owned" (matching D-07's
"unowned and nonexistent are indistinguishable").

### Pattern 3: List widening via existing coverage-field precedent (RPC-03, D-10/D-11)

**What:** `ListMemories`/`SearchMemories` already carry `searched_scopes`/`scopes_truncated`/
`scopes_unknown` (`proto/engram/v1/engram.proto:121-136,203-215`) computed by
`deps.searchedScopes` (`internal/server/tools.go:2054`). `ListRules`'s D-10 widening (empty `scopes`
→ enumerate every readable `rule:*` scope) and `ListScheduled`'s D-11 widening (`cross_spine` +
cursor) reuse this exact mechanism rather than inventing new coverage semantics.

**When to use:** Both new widenings, on both lanes.

**Example (the coverage struct to reuse, not reinvent):**
```go
// Source: internal/server/tools.go:2054-2078 (signature; struct fields read from same region)
func (d *deps) searchedScopes(ctx context.Context, c caller, crossSpine bool) scopeCoverage
```

### Pattern 4: A new proto oneof for a Go union-by-omission struct (RPC-04, D-12)

**What:** `store.RelatedEdge` (`internal/store/relatedmemories.go:87-105`) is a flat struct where only
the fields matching `Type` are populated — a `Type: RelatedEdgeVector` entry has `Score` set and
`SharedTags`/`SharedCitations`/`Direction`/`Depth` all zero. This is fine for MCP's flat JSON (D-12:
"The MCP JSON stays flat, like the store's `RelatedEdge` struct") but must NOT be copied verbatim
into the Connect proto message — a flat proto message lets a client read `shared_tags` on a
vector-typed edge with no compile-time or wire-level signal that it is meaningless. D-12 mandates a
`oneof evidence` instead.

**Example (the store type driving the proto design — quoted verbatim, not paraphrased):**
```go
// Source: internal/store/relatedmemories.go:87-105 [VERIFIED: internal/store/relatedmemories.go:87-105]
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
The proto design (planner/executor to finalize exact field numbers): an `EdgeType` enum mirroring
`RelatedEdgeType`'s four values (`internal/store/relatedmemories.go:39-52`: supersession, citation,
tag, vector — quoted verbatim above as the const block), plus a `oneof evidence` with one message
per type (`VectorEvidence{score}`, `TagEvidence{shared_tags, tag_weight}`,
`CitationEvidence{shared_citations}`, `SupersessionEvidence{direction, depth}`), each field named to
match its Go struct counterpart 1:1 so the protoconv mapping is a mechanical field-by-field copy.

### Anti-Patterns to Avoid
- **A store-level batch Archive/Restore primitive:** D-06/D-07 explicitly reject atomicity across
  the id set; do not build a multi-point SetPayload analogous to `Supersede`'s back-stamp — the
  sequential per-id loop *is* the design, not a placeholder for a future atomic version.
- **Hand-wrapping a Connect error code instead of using `connectError`:** every existing handler
  states this discipline explicitly (`internal/server/connectapi.go:240-244,262-269`) — a
  hand-wrapped `connect.CodeInvalidArgument` at a boundary check silently overrides the classified
  `*argError`'s intended code.
- **Iterating `csrfWriteProcedures` in the new test:** D-15 explicitly forbids this ("Never iterate
  `csrfWriteProcedures`: a test driven by the map cannot notice a Procedure missing from it") —
  list the three new write Procedures explicitly in the test.
- **A flat `RelatedEdge` proto message:** rejected by D-12 (see Pattern 4).
- **A second RPC-count/name/type descriptor test extending the deleted map:** D-26 says delete the
  map, keep only the IDEMPOTENCY_UNKNOWN check plus the new count (12→19).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Owner-only write gate for archive/restore | A second authz check in the Connect/MCP layer | `store.ArchiveAs`/`RestoreAs`'s existing `getWritable(ctx, id, subj, authz.ActionArchive)` (`internal/store/spine.go:858,969`) | DEC-cgb: authz lives only in `internal/store`; a handler-level check would be redundant at best and a second, potentially-diverging source of truth at worst |
| Short-id-or-UUID resolution for archive/restore/related/tags target ids | A new resolver | `store.ResolvePointID` (already used by `resolveAndAuthorizeSupersedeTargets`, `internal/server/tools.go:2452`, and by every existing id-taking tool) | One resolver, one set of edge cases (ambiguous short_id, not-found) already tested |
| Scope-coverage reporting for the new `cross_spine`/empty-`scopes` widenings | A new truncation/coverage struct | `deps.searchedScopes` / `scopeCoverage` (`internal/server/tools.go:2054-2078`) | Byte-identical coverage semantics across every surface is the milestone's own "the entry point must not lie" rule; a second implementation risks silent divergence |
| Opaque cursor mechanism for `ListScheduled`'s new pagination | A new token scheme | `list_memory`'s existing opaque cursor (`internal/store` cursor machinery already used by `deps.listMemory`) | Explicitly named as reusable in 01-CONTEXT's canonical refs and this phase's own code_context |
| Blast-radius hint derivation for the 4 new MCP tools | Per-tool hard-coded `mcp.ToolAnnotations` literals | `annotationsFor(name)` reading `internal/surfaces.ClassForTool` (`internal/server/toolannotations.go:24-35`) | Single source of truth for both the MCP hints and the CLI catalog; a hard-coded literal could drift from the registry silently |
| Rejection envelope for malformed archive/restore batch input | A bespoke error type | `argErrf(classMalformed/classOutOfRange, HintRequired/HintTooMany, ...)` (`internal/server/argerror.go`) | Every existing rejection in this codebase uses this one envelope; a bespoke type breaks `connectError`'s single mapper and the documented hint-code vocabulary (`hintcodedocs_test.go`) |

**Key insight:** this phase's entire risk surface is *composition*, not *invention*. Every primitive
already exists and is already correct (Phase 1's store methods, the existing CSRF interceptor, the
existing coverage/cursor helpers, the existing error envelope, the existing annotation registry). The
only genuinely new code is the batch-outcome assembly loop and the `RelatedEdge` oneof — everything
else is "add one more row to an existing table/switch/registry."

## Runtime State Inventory

Not applicable — this is a greenfield wiring phase (new RPCs/tools over existing store data), not a
rename/refactor/migration phase. No stored data, live service config, OS-registered state, or
secrets change shape or key. **Nothing found in this category** — verified by reading every touched
file's existing payload-key usage (`archived_at`, `superseded_by`, `tags`) is unchanged by this
phase; only new *readers* (RPCs/tools) are added over those existing keys.

## Common Pitfalls

### Pitfall 1: Confusing "batch call succeeds" with "every id succeeded" (D-07)
**What goes wrong:** A handler or test treats a per-id `not_found` outcome as a call-level failure
(e.g. returning a Connect error code when any id in the batch didn't archive).
**Why it happens:** Every other id-taking verb in this codebase (`delete_memory`, `get_memory`) is
single-id and DOES fail the whole call on not-found — the batch shape is a new pattern for this
phase, easy to reflexively pattern-match to the old single-id shape.
**How to avoid:** The RPC/tool returns `CodeOK`/success whenever the *shape* is valid (non-empty
list, ids within the per-entry length/count cap); individual `not_found` outcomes live in the
response body, never in the error path. Only shape violations (empty list, over-cap, malformed
token) go through `argErrf`.
**Warning signs:** A test asserting `err != nil` for a batch containing one bad id alongside good
ones; a handler with an early-return inside the per-id loop.

### Pitfall 2: Forgetting the CSRF allowlist entry (RPC-05, silent security regression)
**What goes wrong:** A new write RPC ships without a `csrfWriteProcedures` entry — it compiles,
passes every other test, and is silently exempt from CSRF, exactly the class of bug v0.12.x Phase 1
research called out for the *original* six write RPCs ("a CSRF exemption keyed on
request-controlled input would be a full bypass on all six write RPCs").
**Why it happens:** `csrfWriteProcedures` (`internal/server/connectcsrf.go:33-40`) is a separate file
from `connectapi.go`; adding a handler there does not force a compile error if the CSRF map entry is
missing — Connect happily serves an unlisted write Procedure through the read path.
**How to avoid:** D-15's own discipline — write the missing-token → `permission_denied` test FIRST,
before the handler exists, so it starts red for the right reason (handler missing) and stays red
until BOTH the handler and the CSRF entry land. `RulePagingMutuallyExclusive`-style conditional-rule
gates don't cover this; only the explicit per-RPC test does.
**Warning signs:** A new write RPC passing all tests with no dedicated CSRF-rejection test naming it
specifically (per D-15, never derived by iterating the map).

### Pitfall 3: Reusing a deprecated/renumbered field number on an additive change
**What goes wrong:** `buf breaking` fails because a new field reuses a number a prior `deprecated =
true` field still occupies.
**Why it happens:** `proto/engram/v1/engram.proto:119` already has one example
(`ListMemoriesResponse.approximate`, field 3, `[deprecated = true]`) — a deprecated field still
OCCUPIES its number (this is a documented durable-record gotcha in STATE.md: "a `deprecated = true`
field still OCCUPIES its number. Reusing one is the single way an otherwise-additive change trips
`buf breaking`").
**How to avoid:** For the two messages this phase widens (`ListRulesRequest`-shaped and
`ListScheduledRequest`/`Response`), always add new fields at the NEXT unused number, never reuse a
gap left by a deprecated field. Run `task proto:lint && task proto:gen` and `buf breaking` locally
before considering a proto edit done.
**Warning signs:** `buf breaking` CI job fails with a `FIELD_NO_DELETE` or `FIELD_SAME_NUMBER`
category error.

### Pitfall 4: Post-filtering `RelatedMemories`/`ListTags` results in the Connect handler
**What goes wrong:** A handler adds a "hide this record if the caller can't read it" check after
calling `deps.relatedMemories`, thinking it's defense-in-depth.
**Why it happens:** The temptation exists because the wire shape (`RelatedResult`) exposes full
`Memory` objects, which looks like it needs re-checking at the boundary.
**How to avoid:** `store.RelatedMemories` already composes the caller's read predicate into every
sub-query AND the final payload fetch (`internal/store/relatedmemories.go:8-11`: "The caller's read
predicate... is composed into every Qdrant sub-query's filter and into the final payload fetch,
never applied as a post-filter... no layer above internal/store may need to filter this method's
output."). A handler-level re-filter is not just redundant, it risks silently narrowing the response
inconsistently with the store's own truncation/ceiling accounting (`Truncated` would then lie).
**Warning signs:** Any `if !canRead(...)` check inside `connectapi.go` or `tools.go` for these two
RPCs.

### Pitfall 5: Treating `ui/src/lib/gen/engram_pb.ts` as fully machine-generated
**What goes wrong:** Regenerating gen/ts and assuming `ui/src/lib/gen/` is now fully in sync, then
being surprised by an `ui-drift` gate failure or a hand-authored client helper reverting.
**Why it happens:** Per the phase's own additional_context: "note `ui/src/lib/gen/engram_pb.ts` is
hand-authored (see engram memory `b3jgtjn8yx`)" — some of `ui/src/lib/gen/` is generated by
`task proto:gen`, some is hand-maintained.
**How to avoid:** Before touching anything under `ui/src/lib/gen/`, confirm via `git log`/`git diff`
which files `task proto:gen` actually rewrites vs which are committed by hand (the executor should
run `task proto:gen` on a clean tree and `git status ui/src/lib/gen/` to see exactly what changed).
Never hand-edit a file the tool regenerates; never assume a file under that path is safe to
regenerate-and-forget without checking first. `[ASSUMED — memory `b3jgtjn8yx` content not directly
read this session; flagged for confirmation, see Assumptions Log A1]`
**Warning signs:** `ui-drift` CI gate failing after a proto-only change that "should" have been
purely additive.

## Code Examples

### 1. Existing MCP↔Connect delegation for a write verb with a target-set preflight (the shape `SupersedeMemory`'s Connect handler must copy)
```go
// Source: internal/server/tools.go:2594-2596,2604-2607 [VERIFIED: internal/server/tools.go:2594-2650]
func (d *deps) supersedeMemory(ctx context.Context, c caller, a supersedeArgs) (string, string, error) {
	if err := validateStoreArgs(a.storeArgs, d.maxSummaryBytes, d.writeCaps); err != nil {
		return "", "", err
	}
	// ... preflight (resolveAndAuthorizeSupersedeTargets, checkIdempotentMergeReplay,
	// validateSupersedeTargetState) then d.st.Supersede(...) ...
}
```
`SupersedeMemory`'s Connect handler needs only: `callerFromConnectContext` → build `supersedeArgs`
from the proto request (including the new `ValidateOnly bool` field) → if `ValidateOnly`, run the
same preflight stages (`resolveAndAuthorizeSupersedeTargets` + `validateSupersedeTargetState`)
WITHOUT calling `d.st.Supersede`, and return the resolved targets — this is the shape D-08 describes
("runs the full preflight... returns the resolved targets and the would-be chain without writing").

### 2. The CSRF lane test shape to copy for the three new write RPCs (D-15)
```go
// Source: internal/server/connectcsrf_lane_test.go:39-63 [VERIFIED: internal/server/connectcsrf_lane_test.go:39-63]
func TestBearerLaneExemptFromCSRF(t *testing.T) {
	d, _ := newSpyDeps()
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, csrfStubResolveWithLane(auth.LaneBearer), csrfTestVerify, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()
	call := csrfWriteCases(timestamppb.New(time.Now().Add(time.Hour)))[0].call // StoreMemory
	if err := call(ctx, client, csrfHeaders{actor: "actor-A"}); err != nil {
		t.Fatalf("bearer-lane write with no CSRF material: got err %v, want success", err)
	}
}
```
The cookie-lane missing-token variant (the one D-15 actually requires — a request WITHOUT the
double-submit token → `permission_denied`) lives elsewhere in the same file; the executor should
locate and mirror that exact case for `SupersedeMemory`/`ArchiveMemory`/`RestoreMemory`, paired
(per D-15) with a valid-token success case for the same RPC.

### 3. The write parity test's per-RPC subtest shape (D-20)
```go
// Source: internal/server/connectapi_write_parity_test.go:172-195 [VERIFIED: internal/server/connectapi_write_parity_test.go:172-195]
func TestWriteParity(t *testing.T) {
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
		// ... compare stored effect fields ...
	})
}
```
This is NOT a data-driven table — it is one `t.Run` subtest per RPC, each independently seeding two
`spyStore`s and asserting store-trace + code parity. `SupersedeMemory`/`ArchiveMemory`/
`RestoreMemory` each need their own `t.Run` block added here; `ListRules`/`ListScheduled`/
`RelatedMemories`/`ListTags` (read-only) get analogous rows in `connectapi_parity_test.go` instead
(the read-lane parity file, structured similarly but without a store-mutation trace comparison).

### 4. The `ArchiveAs` call this phase's batch core must call once per resolved id
```go
// Source: internal/store/spine.go:837-866 [VERIFIED: internal/store/spine.go:837-866]
func (s *Store) ArchiveAs(ctx context.Context, id string, subj Subject) (res ArchiveResult, err error) {
	// ... span/telemetry ...
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

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| The ROADMAP's Phase 3 goal text describes the Archive/Restore authz gate as an open design question | Phase 1 (this same milestone, 2026-09-26) already decided and shipped `authz.ActionArchive` + `ArchiveAs`/`RestoreAs` | 2026-09-26, milestone 2026-09-25.01 Phase 1 | Phase 3 planning must NOT re-open this; CONTEXT.md's `<domain>` section states this explicitly ("Roadmap open question already settled") |
| `EngramService` has 11 RPCs (5 read + 6 write, per `connectdescriptor_test.go`'s current pin) | Growing to 18 RPCs (12 read + 6 write... actually 12 including MigrateStatus, +7 new = 19 per D-26's own count) | This phase | `connectdescriptor_test.go`'s RPC-count assertion must be updated from whatever the pre-phase count is to the post-phase count, and its per-RPC name/type map DELETED (D-26) |

**Deprecated/outdated:** none directly deprecated by this phase; `ListMemoriesResponse.approximate`
(field 3, already `[deprecated = true]`) remains deprecated and unused, a pre-existing state this
phase must not disturb (Pitfall 3).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Engram memory `b3jgtjn8yx` documents that `ui/src/lib/gen/engram_pb.ts` is partly hand-authored, and that a proto regen does not fully resync `ui/src/lib/gen/` | Common Pitfalls, Pitfall 5 | If the memory's actual content differs (e.g. the file is fully hand-authored, or fully generated), the executor could either destructively regenerate a hand-maintained file or waste time hand-syncing a fully generated one. Low risk (the phase's own additional_context already asserts this fact independently of the memory id), but the specific memory content was not fetched this session — engram `get_memory` was not available/attempted in this research pass. **Action for planner:** have the first task that touches `ui/src/lib/gen/` run `task proto:gen` on a clean tree and diff, rather than trusting either this note or the memory blindly |
| A2 | The exact pre-phase `EngramService` RPC count in `connectdescriptor_test.go` is 12 (5 original read + `MigrateStatus` + 6 write), so the phase's post-change count is 19 | State of the Art | If a different phase between Phase 1 (2026-09-26) and now changed the proto and this research's read of `connectdescriptor_test.go` (also dated 2026-09-26, same day) is already stale, the arithmetic 12→19 could be off by one. Verified directly by reading the test file this session (`internal/server/connectdescriptor_test.go:74-77`, `methods.Len() != 12`), so this is `[VERIFIED]`, not assumed — retained here only as a flag that the count must be re-verified live at plan/execute time if the proto changed again in the interim |

**Note on A2:** A2 is included for completeness but is actually `[VERIFIED: internal/server/connectdescriptor_test.go:74-77]` as of this research session — listed here so the planner double-checks it has not drifted between research and execution (same-day risk only).

## Open Questions

1. **Exact field numbers and message names for the seven new RPCs' request/response types, and for `RelatedEdge`'s oneof.**
   - What we know: must be additive (`buf breaking` green), no field-number reuse (Pitfall 3), and
     the `RelatedEdge` shape must be a `oneof evidence` over four typed evidence messages (D-12).
   - What's unclear: the literal message/field names and numbers — explicitly left to "Claude's
     Discretion" in CONTEXT.md.
   - Recommendation: planner assigns exact names during plan-phase task breakdown; executor runs
     `task proto:lint` after drafting to catch numbering mistakes immediately, before writing any Go.

2. **Whether `ArchiveMemory`/`RestoreMemory`'s batch cap should be `store.MaxRecallLimit` (1000) or a smaller phase-specific constant.**
   - What we know: CONTEXT.md's discretion item says "at most the shared 1000 maximum, via
     `rejectOverMaximum`" — so 1000 is the ceiling, not necessarily the chosen cap.
   - What's unclear: whether a smaller default (e.g. matching Phase 4's expected multi-select UI
     size, likely far under 1000) is more honest about real usage.
   - Recommendation: default to reusing `rejectOverMaximum("ids", n)` unchanged (the 1000 ceiling)
     unless the plan has a concrete reason to narrow it — narrowing without a concrete UI-driven
     reason adds an arbitrary undocumented limit for no benefit.

3. **Whether the `validate_only` response reuses `SupersedeMemoryResponse` with an added `validated bool` marker, or is a distinct shape.**
   - What we know: D-08 requires it to "name every resolved target and write nothing"; CONTEXT.md's
     discretion item explicitly leaves the exact shape open.
   - What's unclear: whether Phase 4's preview dialog (the consumer) wants a response shape closer
     to the write response or closer to a full `RelatedResult`-style preview object.
   - Recommendation: since Phase 4 is not planned yet, favor the SIMPLER shape (reuse
     `SupersedeMemoryResponse`'s fields plus a `validated` marker and the resolved target list) —
     it is strictly easier to widen later than to narrow a bespoke preview message.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Building/testing all new code | ✓ `[VERIFIED: local shell]` | go1.27.1 (darwin/arm64) | — |
| `buf` CLI | `task proto:lint`/`proto:gen`, additive-change verification | ✓ `[VERIFIED: local shell]` | 1.73.0 | — |
| Docker | Qdrant testcontainer for `internal/store`/`internal/server` integration tests (`ENGRAM_REQUIRE_QDRANT=1`) | ✓ `[VERIFIED: local shell — docker info succeeded]` | not queried | — |
| `task` (go-task) | `task proto:gen`, `task lint`, `task test`, `task fmt` | Not explicitly probed this session; used throughout `CLAUDE.md`/`Taskfile.yaml` as the project's sole runner | — | If missing, every `task X` command in this document maps 1:1 to the underlying `go test`/`buf`/`golangci-lint` invocation visible in `Taskfile.yaml`, so the executor can fall back to the raw commands |

**Missing dependencies with no fallback:** none identified.
**Missing dependencies with fallback:** `task` itself (see above) — direct tool invocation is always available as a fallback since `Taskfile.yaml`'s `cmds:` are themselves the plain shell commands.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + a real Qdrant testcontainer for integration coverage |
| Config file | none — tests are plain `_test.go` files; the Qdrant testcontainer gate is `ENGRAM_REQUIRE_QDRANT=1` (an env var, not a config file) |
| Quick run command | `go test ./internal/server/... -run TestSupersedeMemory` (or the specific new test name; per-package, no container needed for handler-level unit tests using `spyStore`) |
| Full suite command | `task test` (= `task test:go` + `task test:python`); `task test:strict` additionally sets `ENGRAM_REQUIRE_QDRANT=1 ENGRAM_REQUIRE_BROWSER=1` so a missing Qdrant/browser FAILS rather than silently skips |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RPC-05/SC1 | Missing CSRF token on `SupersedeMemory`/`ArchiveMemory`/`RestoreMemory` → `permission_denied`, paired with a valid-token success | unit (spy-backed, no Qdrant needed) | `go test ./internal/server/ -run TestConnectCSRF` (exact name per new test) | ❌ Wave 0 — new subtests in `internal/server/connectcsrf_lane_test.go` |
| RPC-02/SC1(archive gate wiring)/D-16 | Non-owner archive/restore of a readable shared record → per-id `not_found`, re-read shows `archived_at` unchanged, both lanes | integration (real Qdrant via `storetest`) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run TestArchiveMemory` | ❌ Wave 0 |
| RPC-02/D-17 | Mixed batch (owned/already-archived/shared-not-owned/nonexistent/short_id) → exact per-id outcomes; empty/over-cap/malformed → `field=<f> hint=<code>` | unit + integration split | `go test ./internal/server/ -run TestArchiveMemoryBatch` | ❌ Wave 0 |
| RPC-01/SC3/D-18 | Invalid `SupersedeMemory` target set names every offender, both lanes; `validate_only` writes nothing and previews match commit | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run TestSupersedeMemory` | ❌ Wave 0 — Connect handler + `validate_only` are new; MCP-side tests for the non-`validate_only` path already exist and stay green |
| RPC-03/D-19 | `ListRules` empty-`scopes` cross-scope enumeration; `ListScheduled` `cross_spine` deferred-reveal holds cross-scope; cursor pages cover the full set | integration (real Qdrant, multi-scope fixtures) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run TestListRules\|TestListScheduled` | ❌ Wave 0 |
| RPC-04/D-21 | `RelatedMemories`/`ListTags` via Connect return compact vs `full`, never leak another actor's private record | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run TestRelatedMemories\|TestListTags` | ❌ Wave 0 |
| SC2/D-20 | All seven RPCs delegate to the same core as their MCP counterpart (behavioural parity, not structural) | unit (spy-backed) | `go test ./internal/server/ -run TestWriteParity\|TestConnectMemoryParityDetector` (existing tests extended with new subtests/rows) | Partial — files exist, new rows are Wave 0 |
| SC4/RPC-06 | Additive proto, `buf breaking` green, gen regenerated + ui-drift passes, blast-radius + catalog entries exist | CI job + existing structural test | `task proto:lint && task proto:gen && buf breaking --against '.git#branch=main'` plus `go test ./... -run TestToolAnnotationsBothDirections` | ✓ existing gates, no new test needed (D-23/D-24) |

### Sampling Rate
- **Per task commit:** the quick run command scoped to the RPC/tool the task just touched (e.g.
  `go test ./internal/server/ -run TestArchiveMemory`).
- **Per wave merge:** `task test` (unit + integration, skips Qdrant-gated tests if Docker
  unavailable — but Docker IS available in this environment, so run `task test:strict` when
  possible for the fail-closed guarantee).
- **Phase gate:** `task` (lint + test) green, plus `buf breaking` and `ui-drift` CI jobs green,
  before `/gsd:verify-work`.

### Wave 0 Gaps
- [ ] New CSRF-rejection test cases for `SupersedeMemory`/`ArchiveMemory`/`RestoreMemory` in
  `internal/server/connectcsrf_lane_test.go` — covers RPC-05/SC1.
- [ ] New `deps.archiveMemory`/`deps.restoreMemory` unit tests (batch outcome assembly, no Qdrant
  needed if built against a mockable `store.ArchiveAs`/`RestoreAs` seam, or integration if not) —
  covers RPC-02/D-06/D-07/D-17.
- [ ] New integration test proving the owner-only archive/restore gate holds through BOTH lanes
  (D-16) — the highest-risk defect this phase can introduce (a handler calling subject-less
  `Archive`/`Restore` instead of the gated `ArchiveAs`/`RestoreAs`).
- [ ] New `validate_only` round-trip test for `SupersedeMemory` (D-18(b)/(c)) — preview must not
  disagree with commit; a dry run with an idempotency key must not poison the replay ledger.
- [ ] New parity-test rows/subtests for all seven RPCs in `connectapi_parity_test.go` /
  `connectapi_write_parity_test.go` (D-20).
- [ ] `connectdescriptor_test.go`'s RPC-count assertion updated and its name/type map deleted
  (D-26) — this is itself a required code change, not just a test addition.
- Framework install: none — `go test`, `task`, and `buf` are already installed and confirmed
  working in this environment.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (indirectly) | Unchanged this phase — `callerFromConnectContext`/`callerFromContext` already resolve identity before any new handler runs; no new auth mechanism introduced |
| V3 Session Management | yes (indirectly) | CSRF double-submit token verification (`newConnectCSRFInterceptor`) extended to 3 new Procedures via `csrfWriteProcedures` — this IS this phase's primary session-integrity control surface (RPC-05) |
| V4 Access Control | yes | `authz.ActionArchive`-gated `ArchiveAs`/`RestoreAs` (already shipped, Phase 1); this phase's job is to never bypass it by accidentally calling the subject-less `Archive`/`Restore` from a caller-facing lane (D-16's exact threat) |
| V5 Input Validation | yes | `buf.validate` CEL rules on new proto messages (via `newConnectValidateInterceptor`) + the existing `argError`/`HintCode` envelope for MCP-lane and shared-core validation (empty batch, over-cap batch, malformed short_id/UUID token) |
| V6 Cryptography | no | No cryptographic operation touched by this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| A new write RPC shipped without a `csrfWriteProcedures` entry (silent CSRF bypass) | Tampering / Spoofing (forged cross-origin write) | D-15's mandatory first-test discipline: write the missing-token rejection test BEFORE the handler exists, so a missing allowlist entry is caught by a red test, not by omission |
| A handler calling the subject-less `Archive`/`Restore` instead of the owner-gated `ArchiveAs`/`RestoreAs` (privilege escalation: any authenticated caller archives/restores ANY record) | Elevation of Privilege | D-16's dedicated wiring test — a non-owner archiving/restoring a readable-but-unowned record must get `not_found`, verified through BOTH lanes, with overlapping scope names so the isolation check cannot pass vacuously |
| Information disclosure via distinguishable "not found" vs "not owned" responses on batch archive/restore | Information Disclosure | D-07's explicit indistinguishability requirement (mirrors Phase 1 D-04 / the existing `DEC-xa6` 404-indistinguishability invariant repo-wide) |
| A `validate_only` supersede call leaking which records exist/are owned via timing or error-message differences vs a real call | Information Disclosure | D-18(a)/(b): validate_only must go through the IDENTICAL preflight (`resolveAndAuthorizeSupersedeTargets` + `validateSupersedeTargetState`) a real call uses, so its observable behavior (errors, resolved targets) is provably identical to what a real call would produce — no separate, potentially-leakier code path |

## Sources

### Primary (HIGH confidence — direct `Read` of source files this session)
- `internal/store/spine.go` (lines 1-30, 709-978) — `ArchiveAs`/`RestoreAs`/`Archive`/`Restore`, `ArchiveResult`/`ArchiveOutcome`
- `internal/store/relatedmemories.go` (full file) — `RelatedMemories`, `RelatedEdge`, edge-type consts, assembly logic
- `internal/store/listtags.go` (full file) — `ListTags`, `TagCount`, `facetTags`, `recallVisibleFilter`
- `internal/store/store.go` (lines 1930-2020, 2620-2760) — `ListScheduled`, `Supersede`
- `internal/authz/authz.go` (lines 1-40) — `Action` constants including `ActionArchive`
- `proto/engram/v1/engram.proto` (full file) — current `EngramService` (12 RPCs), existing message shapes/comments to mirror
- `internal/server/connectapi.go` (full file) — every existing handler's thin-adapter shape, `mountConnect`'s interceptor ordering
- `internal/server/connectcsrf.go` (full file) — `csrfWriteProcedures`, `newConnectCSRFInterceptor`
- `internal/server/tools.go` (lines 900-1080, 2380-2960) — `supersedeArgs`, `listScheduledArgs`, `deps.supersedeMemory` and its preflight helpers, MCP tool registration tail
- `internal/server/rules.go` (full file) — `listRulesArgs`, `deps.listRules`, `validRuleScope`
- `internal/server/toolannotations.go` (full file) — `annotationsFor`
- `internal/surfaces/toolclass.go` (lines 1-170, 355-400) — `Class`/`Operation` registry, existing archive/restore/supersede rows
- `cmd/engram/catalog.go` (lines 1-60) — self-describe catalog derives from the same `internal/surfaces` table
- `internal/server/connectcsrf_lane_test.go` (lines 1-80) — CSRF test shape to copy
- `internal/server/connectdescriptor_test.go` (full file) — RPC-count/name/type test D-26 modifies
- `internal/server/connectapi_write_parity_test.go` (lines 1-210) — write-parity subtest shape
- `internal/server/protoconv.go` (function list) — request→args / result→response converter pattern
- `.planning/phases/01-store-prerequisites/01-CONTEXT.md` — locked Phase 1 decisions this phase wires
- `.planning/phases/03-curation-rpcs-mcp-tools/03-CONTEXT.md` — locked Phase 3 decisions (verbatim above)
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/STATE.md` — requirement text, phase goal/success criteria, standing invariants and durable gotchas
- Local shell: `buf --version` (1.73.0), `go version` (go1.27.1), `docker info` (succeeded) — environment availability

### Secondary (MEDIUM confidence)
- None — every claim in this document traces to a directly-read source file or the CONTEXT.md/REQUIREMENTS.md/ROADMAP.md/STATE.md documents themselves.

### Tertiary (LOW confidence)
- The existence and exact wording of engram memory `b3jgtjn8yx` (referenced by the orchestrator's additional_context, not independently fetched this session) — see Assumptions Log A1.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependency; every library/tool cited was directly confirmed installed or already present in `go.mod`
- Architecture: HIGH — every pattern cited is quoted from source files read this session, not inferred from training data
- Pitfalls: HIGH for Pitfalls 1-4 (derived from explicit CONTEXT.md decisions and directly-read source comments); MEDIUM for Pitfall 5 (the underlying claim about `ui/src/lib/gen/engram_pb.ts` traces to an unfetched engram memory, flagged in Assumptions Log A1)

**Research date:** 2026-09-26
**Valid until:** 30 days (stable internal codebase conventions; re-verify if Phase 1 or Phase 2 land further changes to `internal/store`, `internal/server`, or the proto before this phase executes)
