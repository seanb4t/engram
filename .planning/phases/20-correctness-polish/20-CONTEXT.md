# Phase 20: Correctness & Polish - Context

**Gathered:** 2026-07-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Close six independent correctness gaps surfaced during v0.9.x code review, each removing a
specific silent-failure or drift risk. Purely hardening the existing surface — **no new
capabilities**. The six gaps (each = one GitHub issue):

1. **#307 (REQ-discovery-proto-fidelity)** — `SearchDiscoveries` carries `kind`/`citations`/`summary` on the Connect wire instead of silently dropping them.
2. **#308 (REQ-shortid-mint-cap)** — `MintShortID` bounds its collision-retry loop and returns an explicit exhaustion error instead of looping indefinitely.
3. **#304 (REQ-embed-param-key-sharing)** — `config.ParseEmbedParams` and `embedReq`'s wire contract share a single reserved-param-key list (cannot silently desync).
4. **#302 (REQ-embed-body-build-collapse)** — `embed.Client.embed()`'s two-path body build (struct-marshal vs map-merge) collapses into a single map-based path.
5. **#303 (REQ-discovery-shortid-schema)** — `storeDiscoveryArgs.ID` jsonschema advertises `short_id` support, matching the skill docs.
6. **#269 (REQ-summarize-cronjob)** — Helm chart ships `engram summarize-missing` as a `batch/v1` CronJob reusing the Deployment's image/env plumbing via a shared `_helpers.tpl`.

These are independent of the write-lane and embedder feature tracks; they can be scheduled and
landed flexibly.

</domain>

<decisions>
## Implementation Decisions

### Discovery wire fidelity (#307)
- **D-01:** Carry discovery fidelity by **extending the shared `Memory` proto message additively** — add `kind` (field 21) and `citations` (`repeated Citation`, field 22). `citations` reuses the existing `Citation` message (`proto/engram/v1/engram.proto:122`). Populate both in `memoryToProto` (`internal/server/connectapi.go:32`) for discovery records; they stay empty on plain memories. Rejected the "dedicated `Discovery` message + change `SearchDiscoveriesResponse` field type" alternative — changing field 1's type is wire-breaking for the existing console client and violates the Phase-15 additive-only discipline.
- **D-02:** **Scope is wire-fidelity only.** The fields must arrive over Connect and the console gen client must be re-vendored so the types exist — but *rendering* `kind`/`citations` in the console discovery view is **out of scope** (belongs to a future console phase). See Deferred Ideas.
- **D-03:** `summary` is already mapped by `memoryToProto` (`connectapi.go:45`) via `store.Memory.Summary`; the researcher should confirm whether the SC1 "summary dropped" symptom is real or already satisfied, and scope the fix to the genuinely-missing `kind`/`citations`.

### MintShortID collision cap (#308)
- **D-04:** Cap the retry loop at **16 Qdrant-checked attempts** (extra headroom over the ~8 that is already astronomically safe in a 32^10 Crockford-base32 space), then return a **wrapped sentinel error** (e.g. `ErrShortIDExhausted`) that bubbles up as a normal write failure through `store_memory`/`store_discovery`/`store_rule`.
- **D-05:** `seen`-map duplicate hits (the in-batch dedup `continue` at `store.go:1786`) **do not consume** the attempt budget — only real Qdrant collision checks count against the cap.
- **D-06:** The exhaustion path flows through the existing telemetry wrapper (`store.go:1767-1772` records the error on the span + `RecordStoreOp`); no new metric required, but the sentinel must be `errors.Is`-checkable.

### Summarize-missing CronJob (#269)
- **D-07:** Ship the CronJob **opt-in / disabled by default** (`cronjob.enabled: false` in `values.yaml`) — it is only meaningful when a summary model is configured (`ENGRAM_SUMMARY_MODEL`), so defaulting off avoids a no-op scheduled pod on every install.
- **D-08:** Default knobs when enabled: **daily schedule**, `concurrencyPolicy: Forbid` (never overlap sweeps), `restartPolicy: OnFailure`, `successfulJobsHistoryLimit: 3` / `failedJobsHistoryLimit: 1`, all schedule/policy values surfaced as `values.yaml` overrides.
- **D-09:** The CronJob **reuses the Deployment's image + env** by factoring the shared container env block into a new `charts/engram/templates/_helpers.tpl` named template, consumed by both `memory-mcp.yaml` and the new CronJob template (no env duplication / drift). `_helpers.tpl` does not exist yet — it is created this phase.

### Landing strategy
- **D-10:** The planner **splits the work by subsystem** into ~4 plans so each closes its own GitHub issue with an atomic commit and the independent fixes can parallelize:
  - Plan A — proto + discovery: **#307** (proto Memory extend + re-vendor gen client + handler populate) and **#303** (discovery short_id jsonschema).
  - Plan B — embed cleanups: **#304** (shared reserved-key list) and **#302** (single map-based body build) — same file (`internal/embed/embed.go`), naturally grouped.
  - Plan C — store/shortid: **#308** (MintShortID cap).
  - Plan D — helm: **#269** (CronJob + `_helpers.tpl`).
  - (Grouping is guidance; final plan boundaries follow the planner's file-overlap analysis.)

### Claude's Discretion
- **#304, #302, #303** are mechanical refactors / schema-doc fixes with no user-facing decision — implemented at Claude's discretion within the decisions above. For **#303 specifically**, verify the *actual* residual gap first: `storeDiscoveryArgs.ID` at `internal/server/tools.go:550` already reads `"…supply the full UUID or short_id to replace in place"`, so the issue may be already (partly) resolved or may point at a different discovery arg — reconcile against issue #303 and the skill/docs before changing anything.
- Exact sentinel-error naming, `_helpers.tpl` template-name, and proto field comments are Claude's call, consistent with existing repo conventions.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` — REQ-discovery-proto-fidelity / -shortid-mint-cap / -embed-param-key-sharing / -embed-body-build-collapse / -discovery-shortid-schema / -summarize-cronjob (each links its GH issue).
- `.planning/ROADMAP.md` §"Phase 20: Correctness & Polish" — goal + 6 success criteria (the authoritative acceptance list).
- GitHub issues: **#307** (discovery proto fidelity), **#308** (MintShortID cap), **#304** (embed param-key sharing), **#302** (embed body-build collapse), **#303** (discovery short_id schema), **#269** (summarize CronJob).

### #307 — discovery proto fidelity
- `proto/engram/v1/engram.proto` — `Memory` message (L13-38; next free fields 21/22), `Citation` message (L122), `SearchDiscoveriesResponse` (L91, `repeated Memory discoveries`).
- `internal/server/connectapi.go` — `memoryToProto` (L32), `memoriesToProto` (L54), `SearchDiscoveries` handler (L226, uses `memoriesToProto`).
- Phase-15 additive-proto discipline + buf lint gate (`task proto:lint` / `task proto:gen`); Phase-19 `19-01-PLAN.md` = the re-vendor-console-gen-client pattern (`gen/ts`, `task ui:build`).

### #308 — MintShortID cap
- `internal/store/store.go` — `MintShortID` (L1763; unbounded `for {}` at L1779, telemetry wrapper L1767-1772).
- Callers: `internal/server/tools.go:657/693/758`, `internal/server/rules.go:134`, `internal/store/store.go:1860`.

### #304 / #302 — embed cleanups
- `internal/embed/embed.go` — `embedReq` struct (L136), `embed()` (L210), two-path body build (L233 `json.Marshal(embedReq{...})` vs the map-merge branch).
- `internal/config/config.go` — `ParseEmbedParams` (reserved-param-key list to unify).

### #303 — discovery short_id schema
- `internal/server/tools.go` — `storeDiscoveryArgs` (L543, `.ID` at L550), `validateStoreDiscovery` (L575).
- `docs-site/src/content/docs/reference/tools.md` — store_discovery skill/tool docs (the "matching the skill docs" target).
- `skill/engram/` discovering skill — the client-facing doc the schema must match.

### #269 — summarize CronJob
- `cmd/engram/summarize.go` — the `summarize-missing` cobra command (Use/Short/RunE) the CronJob invokes.
- `charts/engram/templates/memory-mcp.yaml` — current Deployment (inline container env block to factor out).
- `charts/engram/values.yaml` — `summarize:` config block (L95-99); add a `cronjob:` block here.
- `charts/engram/templates/_helpers.tpl` — **to be created** (shared env named-template).
- `docs-site/src/content/docs/guides/configure.md` — Auto-summary / summarize-missing operator guidance.

### Codebase maps
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/STRUCTURE.md` — repo conventions for proto/Helm/store layering.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`memoryToProto` (`connectapi.go:32`)** — the single store→proto conversion point; extending it (plus the additive proto fields) is the entire server-side surface for #307. `SearchDiscoveries` already routes through `memoriesToProto`, so no handler restructuring is needed.
- **`Citation` proto message (`engram.proto:122`)** — already exists (used by `StoreDiscoveryRequest`); #307 reuses it for the new `Memory.citations` field, no new message.
- **`MintShortID` telemetry wrapper (`store.go:1767-1772`)** — already records errors on the span; the new exhaustion sentinel rides this path for free.

### Established Patterns
- **Additive-only proto + buf lint gate (Phase 15)** — new fields get the next free field numbers; never renumber/retype existing fields. `gen/` is committed and CI drift-checked (`buf` job).
- **Re-vendor console gen client (Phase 19, 19-01)** — a `Memory`-message change requires regenerating `gen/ts` and rebuilding the embedded SPA (`task ui:build`) so CI ui-drift stays green.
- **Helm named-template factoring** — `_helpers.tpl` is the standard place for the shared env block; both Deployment and CronJob `include` it.

### Integration Points
- Proto change (#307) → `task proto:gen` → `gen/go` + `gen/ts` regenerate → console SPA rebuild. This is the one cross-cutting item; #308/#302/#304/#303/#269 are self-contained to their subsystem.
- New CronJob template + `_helpers.tpl` edit to `memory-mcp.yaml` must keep the existing Deployment env byte-identical after the factor-out (helm-template diff = no-op for the Deployment).

</code_context>

<specifics>
## Specific Ideas

- #308 cap = **16** (user chose extra headroom over the recommended 8; both are effectively never reached).
- #269 CronJob **disabled by default**; `concurrencyPolicy: Forbid`; daily schedule.
- #307 explicitly **wire-only** — do not add console rendering this phase.

</specifics>

<deferred>
## Deferred Ideas

- **Console rendering of discovery `kind`/`citations`** — once #307 puts the fields on the wire and in the gen client, a future console phase can surface citations in the discovery detail view. Out of scope here (wire fidelity only).
- **Optional CronJob follow-ups** — alerting/metrics on summarize-missing sweep outcomes, or a `--older-than`-style bound on the sweep, if operational need arises. Not required by #269.

None — discussion otherwise stayed within phase scope.

</deferred>

---

*Phase: 20-correctness-polish*
*Context gathered: 2026-07-15*
