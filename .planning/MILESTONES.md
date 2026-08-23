# Milestones — engram

## 2026-08-12.01 Record State & Schema Evolution (Shipped: 2026-08-22)

**Phases completed:** 9 phases (1–9), 46 plans, 121 tasks
**Requirements:** 27/27 verified · **Audit:** `tech_debt` (5/5 integration seams, 3/5 E2E flows, 0 blockers, Nyquist 9/9 COMPLIANT)
**Git range:** `1daafefd..HEAD` — 181 files, +26,418 / −1,392 (excluding `.planning/`)
**Timeline:** 2026-08-12 → 2026-08-22 (11 days) · **No git tag** — release-please owns the version namespace
**Closeout:** `override_closeout` · **Known verification overrides:** 8 newly acknowledged, 0 carried forward from a prior close (see STATE.md Deferred Items)
**Archived:** `milestones/2026-08-12.01-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md` + `2026-08-12.01-phases/` + `2026-08-12.01-quick/`

**Delivered:** a record's full state became reachable and legible on every lane, and payload
evolution got a real mechanism instead of another one-shot operator command. `schema_version`
landed as an absent-safe discriminator — stamped on every write, visible on the wire, and
structurally incapable of narrowing recall, proven by a runtime interceptor rather than by
convention. Behind it, `internal/migrate` turned migrations into an ordered registry of
additive-only steps that must declare their own reversibility at registration, swept to
convergence by `Store.Migrate` without a collection lock, and driven by `engram migrate` through
the existing `registerDestructive` admission gate with `backfill-short-ids` folded in as its first
registered customer. The operator tier collapsed to one serialization plus a view, so a json
document can no longer widen past the text sentence beside it, and `--output text` is now published
as explicitly unstable. Console and CLI both surface archived/superseded/expired/scheduled state
from one tested derivation. The milestone opened, deliberately, by repairing its own gates: key-link
patterns that could not match and a Qdrant testcontainer that masked real failures.

**Key accomplishments:**

1. **`schema_version` as an absent-safe payload discriminator** — monotonically stamped on every write, wire-visible on `get_memory` and `full=true` recall, with a runtime gRPC-interceptor gate proving it never reaches any Qdrant filter, so no legacy record can be hidden from recall by the field's own absence.
2. **A versioned `internal/migrate` step registry and sweep** — sealed `Reversibility`, `CheckAdditive`'s two-direction key-set diff, and re-derive-every-pass convergence proven under a live concurrent writer with no collection lock, all against real Qdrant with committed reviewer-reproducible RED patches.
3. **`engram migrate` (`status` / `revert`) as the operator CLI** — routed through a generalized `registerDestructive` admission gate with a deliberately named `--apply`-required union, folding `backfill-short-ids` in as the registered v0→v1 step with apply-path parity proven by call-sequence equality.
4. **One-serialization-plus-a-view typed operator renderer** — 15 commands converted with zero new per-report renderer code, `--output text` published as an explicitly unstable human-readable view (json is the contract), retiring the old text/json parity claim.
5. **Record state surfaced end to end** — eight additive Connect `Memory` fields, three orthogonal `include_*` opt-ins across List and Search on proto, CLI and console, a single tested `memoryStateWords` derivation, and a headless-Chrome test driving the real embedded SvelteKit bundle for the first time.
6. **Gate and CI integrity taken first, not last** — `internal/keylinks` closed the silent-no-op key-link shapes (39 patterns repaired across 20 plans), one shared CI Qdrant replaced four per-package testcontainers, and an AST gate proved no test store construction bypasses its package's collection-prefix seam.

### Known Tech Debt

All three items were parked in backlog phases before close; none blocks the milestone.

- **No full-stack E2E for `engram migrate`** (Phase 4) — `internal/e2e/` covers boot, tool surface, CLI exit codes, console rendering, spine-review and prune-expired, but the status → apply → status reconvergence path has no live-Qdrant coverage. Backlog 999.2.
- **CLAUDE.md's "every surface renders a record's derived state" overstates** (Phase 8) — the MCP lane exposes the raw fields (`SupersededBy`, `ArchivedAt`) but derives no state words. Backlog 999.3.
- **`schema_version` typed three ways in one proto file** (cross-cutting) — `schema_version` uint32 (`:52`) vs `version` int32 (`:186`) vs `current_version` int32 (`:204`). Backlog 999.4.

### Known Verification Overrides

8 open artifacts were acknowledged at close (`override_closeout`), 0 carried forward from a prior
close. Five are stale text for work the milestone audit independently confirms resolved (the
Phase 01 rumdl exclude, the Phase 05 dprint drift, and the Phase 07 SA1019 / escaped-pattern
findings). Three are genuine pre-existing debt: one pending research todo the migration registry
superseded in substance, the `ui/` environment gaps (`svelte-check` / TypeScript incompatibility,
no `lint` script), and two Phase 07 UAT checks that need a live server plus Qdrant. Full disclosure
in STATE.md `## Deferred Items`.

---

## v0.13.x Curation & Self-Evidence (Shipped: 2026-08-12)

**Phases completed:** 6 phases (1–5 plus inserted 03.1), 33 plans, 99 tasks
**Requirements:** 23/24 verified · **Audit:** `tech_debt` (6/6 integration seams, 4/4 E2E flows, 0 blockers, Nyquist 5/6 COMPLIANT)
**Git range:** `b7b9f051..HEAD` — 234 commits, 106 files, +23,671 / −1,842 (excluding `.planning/`)
**Timeline:** 2026-08-03 → 2026-08-12 (10 days) · **No git tag** — release-please owns the version namespace
**Archived:** `milestones/v0.13.x-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md` + `v0.13.x-phases/`

**Delivered:** the memory spine became maintainable without hand-curation, and the interface began
stating its own rules. `engram spine-review` resolves the structural spine predicates a command can
decide as the sixth instance of the existing Subject-less operator tier — never a new authorization
path — while the `curating-spine` skill carries the semantic judgments a CLI cannot make,
proposing every mutation and stopping at `store_rule`'s consent gate verbatim. Alongside it, one
exit-code taxonomy came to govern the whole binary, cobra began enforcing the flag exclusivity the
help text had only claimed, every RPC path gained a finite deadline, and each server-side
conditional rule is now declared once and machine-proven present on every surface that advertises
it. `supersede_memory` grew multi-target merges. Zero new Go dependencies across all six phases.

### Known Gaps

- **REQ-consent-adversarial-proof** (Phase 4) — **NOT SATISFIED**, not merely deferred. The
  adversarial cold read ran and exhausted its locked 3-run cap with every run landing NOT-TEMPTED:
  the reader's identity verdict was correct each time, so the confidently-wrong proposal the
  criterion must observe was never produced. Terminal verdict NOT-OBTAINED — neither pass nor fail
  — escalated and accepted as a non-result by Sean on 2026-08-11 rather than converted into a green
  checkbox. Tracked open at `WINDOWS.md` id 3. Closing it needs a fixture that reliably misleads on
  identity, not more runs of the same one.

**Known verification overrides:** 1 (Phase 4's phase-level override, which unblocks the phase
without asserting SC-3 was proven). Deferred items recorded in `STATE.md` § Deferred Items.

**Known tech debt:** two Phase 03 TDD commit-granularity deviations where RED was genuinely
observed but RED+GREEN landed combined (`WINDOWS.md` ids 1, 2); three waived Phase 03
human-verification items (two prose cold reads, one real-pty `--output` check); a stale rationale
comment at `internal/surfaces/toolclass.go:141-142` contradicting the shipped `idempotency_key`
support (the emitted annotation value is correct — only the justification is wrong); and
`TestExitCodeBaseline`'s env-var fragility (#476).

**Key accomplishments:**

- `reject-both` — any two of the paging trio (`--offset`/`--cursor-mode`/`--page-token`) are rejected before any network call, via cobra's declarative flag-group API rather than a third hand-rolled guard.
- `reindex`, `prune-expired`, `summarize-missing`, and `backfill-short-ids` now resolve every error through the same 2/4/5 exit-code vocabulary as the client verbs, via a new `classifyOperatorErr` modeled on `connectError`.
- `migrate-set-owner`, `migrate-remap-owner`, and every `serve` pre-flight guard now resolve through the shared 2/4/5 exit-code taxonomy; `ListenAndServe`'s own bind failure is the one deliberate, commented exit-1 exception; and two new gates (source-level and table-level) prove no classifiable operator path was missed.
- `exitTimeout=6` distinguishes a client-side deadline from an unreachable server, and every client setting (`--server`, `--token-file`, `--output`, `--insecure`, the new `--timeout`) now resolves through one `config.Load` + `config.ValidateClient` call in `clientFromFlags` instead of four hand-rolled resolvers.
- Every client RPC call site (`search`, `list`, `store`) now derives `context.WithTimeout` from the single resolved client timeout, and a hung server is empirically proven — against connect-go's real client behavior, not an assumption — to return within the window with exit 6, distinct from a closed port's exit 5.
- Rewrote `guides/cli.md`'s exit-code table and `guides/upgrade.md` with a new `## Unreleased` section naming every command this phase's exit-code unification, flag-group enforcement, and client timeout changed — and added `TestUpgradeGuideNamesEveryChangedCommand`, a mechanical gate deriving the required command list from the D-09 before-table itself so the guide cannot silently fall out of sync with what actually shipped.
- `scope-required-unless-cross-spine` declared once in a new stdlib-only `internal/surfaces` leaf package, reaching the server rejection, the cobra `--scope` help text, and five anchored prose regions via a new `task surfaces:gen` generator + CI drift job — proving the whole Phase-2 architecture end-to-end before widening to the rest of D-05's rule inventory.
- Every declared rule's canonical sentence is now machine-proven present on every surface its fields resolve to — across cobra Usage, jsonschema tags, MCP tool Descriptions, proto comments, docs-site, and skill markdown — with applicability derived from the rule's own fields rather than declared, and the gate demonstrated fail-first against a real corrupted region.
- Widened `internal/surfaces`'s registry from one tracer rule to five, converted every remaining in-scope `parseWindow`/`effectiveDiscoveryScope`/paging rejection to `conditionalErrf`, decided `errStaleSummary` stays outside the registry (its fields are shared with a create-only CLI command), and closed a latent gap in 02-02's own conformance-gate tests that the paging/schedule-only rules were the first to exercise.
- All 15 registered MCP tools now advertise readOnlyHint/destructiveHint/idempotentHint/openWorldHint from one internal/surfaces table gated in both directions against the real registration, published to docs-site/reference/tools.md via internal/surfacesgen, and demonstrated fail-first in three distinct ways this session.
- `engram catalog` now carries a per-command `blast_radius` classification derived from the same shared table the MCP tool annotations publish, and every command's `--help` output plus the bare catalog JSON are pinned behind deterministic goldens — closing two determinism hazards (env-derived flag defaults, and cobra's lazy `-h/--help` registration) neither the plan nor its RESEARCH pass anticipated.
- The docs-site deploy is green for the first time since 2026-08-02 and `reference/errors.md` is verified live with all ten hint codes; the stale `docs/v0.12.x-phase-01-context` branch was proven content-identical to its archived copy and deleted.
- A shared recursive cobra walker replacing seven single-level command traversals, plus the operator tier's first nested subcommand (`engram spine-review scan`) backed by a Subject-less, fully-paginated whole-spine scan in `internal/store/spine.go`.
- `--output json|text` backfilled onto all six pre-existing operator commands via the shared operator_output.go helpers, plus `operatorCommands()` as a concrete, gated structural predicate and a behaviourally-pinned three-group `--timeout` matrix.
- `registerDestructive` makes the destructive tier's `--apply` gate a runtime-enforced RunE choke point (not just a derived flag), flips `prune-expired` and `migrate-remap-owner` to preview-by-default, and adds the first end-to-end coverage any operator command has ever had.
- A pure four-tier (valid/moved/broken/unverifiable) citation classifier, a Subject-less citation enumeration over the phase's shared paginated iterator, a RESOLVED (not lexical) path-safety gate, an exact-segment repo-identity comparison, and the new `exitFindings=7` taxonomy code gated behind a registered `--fail-on` conditional rule.
- Ranked near-duplicate candidate pairs (A, B, score) over already-stored vectors via Qdrant's `NewQueryID`/`QueryBatch`, with no clustering, no default threshold, and no mutation on any path.
- A new orthogonal `archived_at` record state (D-12) — epoch-second integer, soft-hidden alongside `superseded_by`, driven by `engram spine-review archive`/`restore` with a deterministic concurrent-update race gate.
- `PurgeManifest`'s provenance is compiler-enforced (unexported fields, never a runtime check), `--apply` deletes only the intersection of a previewed, gate-passing set and a fresh re-derivation, and the extract-before-delete gate is honestly asymmetric: a server-set link is unforgeable, a milestone-summary marker is convention.
- Developer ruled `no-cap-otherwise-defaults`: the target-set cap is dropped entirely, and the three remaining planner recommendations (collapse-ambiguous, dedupe-silently, allow-heterogeneous) are adopted as written.
- `supersedes` promoted from a scalar to a set end to end, with a Kind-branching tolerant decoder closing the `GetStringValue()`-returns-empty seam (`supersede_memory` is MCP-only, so proto field-number reasoning never applied).
- `Store.Supersede`'s back-stamp-failure path is now a classified reconciliation pass — a `deletePoint` fault seam plus `reconcileSupersedeFailure` that removes the survivor through an internal ungated primitive, re-reads the FULL requested target set across every Qdrant payload-op chunk, clears dangling `superseded_by` links, and logs whatever it could not resolve — proved against a real pinned Qdrant (`v1.18.2`) with a forced mid-sequence partial-application failure.
- `supersede_memory`'s preflight split into `resolveAndAuthorizeSupersedeTargets` (set-shape + addressability/access, collapsed into one not-found class) and `validateSupersedeTargetState` (rule immutability + already-superseded), proven by two whole-rejection indistinguishability tests, a mixed-pass caller-order test, and four observed-red regression checks.
- `supersede_memory` now accepts `idempotency_key` with a target-set-keyed replay fingerprint (`mergeFingerprint`, composing the existing `contentFingerprint`) checked BEFORE the already-superseded stage — a same-key/different-target retry conflicts, a reordered/duplicated set replays (PD-01), and a losing simultaneous keyed merge recovers its answer from `resolveLostMergeRace` rather than surfacing the store's under-lock rejection.
- Multi-target rejection envelope documented in the error reference with verbatim server-rendered examples, `supersede_memory`'s `supersedes` argument retyped to an array across three docs-site pages, `idempotency_key` support and the honest three-sentence failure paragraph published, CLAUDE.md and the curating-memory skill brought current, and `TestSupersedeDocsMatchShippedContract` binds all of it to the production code so a future edit fails the build instead of drifting.
- Three capped runs of an identity-axis adversarial fixture against `curating-spine/SKILL.md`'s consent gate all returned the correct `overlapping` verdict and an explicit consent-stop — so the confident-wrong case SC-3 requires was never produced, and the honest terminal verdict is NOT-OBTAINED, escalated to the user.
- Expanded `curating-spine/SKILL.md` from 160 to 322 lines with the staleness axis, cheap-search ladder, reactive-recall trigger, and a durable `distinct`-pair marker — all added around the tracer's unchanged consent gate, which a post-expansion cold read against the shipped file confirms still stops at consent (`dilution: NOT DILUTED`).
- Reconciled four v0.13.x VALIDATION.md records against live HEAD facts — repointed one fictional test name, flipped three files to fully validated, and partially reconciled a fourth (04) whose one genuinely unproven requirement stays visibly unproven.

---

## v0.12.x Headless Reach & Diagnosability (Shipped: 2026-08-02)

**Phases completed:** 7 phases, 28 plans, 68 tasks
**Requirements:** 21/21 verified · **Audit:** `tech_debt` (5/5 integration seams, 2/2 E2E flows, 0 blockers)
**Git range:** `cccf5d27..HEAD` — 228 commits, 132 files, +12,510 / −944 (excluding `.planning/`)
**Timeline:** 2026-07-26 → 2026-08-02 (8 days) · **No git tag** — release-please owns the version namespace
**Archived:** `milestones/v0.12.x-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md` + `v0.12.x-phases/`

**Delivered:** engram became reachable by agents that are not a top-level MCP client, and what the
server decides and rejects became legible. One composed verifier chain serves both lanes; Connect
mounts headless; `engram search|store|list` is a real CLI; `cross_spine` spans MCP, the Connect
wire, and the CLI; and one `field=<name> hint=<code>` envelope replaced prose rejections across
both wires. Zero new Go dependencies across all seven phases.

**Known tech debt:** no phase has a reconciled Nyquist `VALIDATION.md` (six at `status: draft`,
phase 2 has none); the two-tier CLI error model has no REQ or decision ID recording its exemption
from the field+hint envelope.

**Key accomplishments:**

- Connect gains a bearer-token identity lane (reimplemented go-sdk-parity credential parse + expiry enforcement in `internal/auth`), and the CSRF exemption is re-keyed off a compiler-enforced, per-request lane stamp instead of any caller-controlled request signal.
- The reseal interceptor now gates on the same lane stamp the CSRF exemption reads (D-09), and a new MCP-vs-Connect parity suite proves the same bearer token resolves to the identical actor/owner on both lanes and is rejected identically when expired — retiring RESEARCH.md Assumption A1 by measurement.
- The verifier chain is now built exactly once per process and injected into both the MCP wrapper and the Connect bearer half (D-06); `connect.headless` gives operators a default-off, fail-closed switch to mount Connect without the web UI (D-10/D-11), and mounting is decided independently of bearer-inclusion so a UI-enabled deployment's Connect lane finally accepts every credential its MCP lane already does (D-12, REVIEWS.md HIGH-3).
- Ships the operator-facing surface for the headless Connect lane: a `guides/configure.md` subsection, a `memory.connect.headless` Helm value with a byte-identical default render, and one tracked follow-up issue — reversing REVIEWS.md MED-10's prior deferral so a Helm-deployed operator can actually reach `REQ-connect-headless-mount`.
- Wired `cross_spine=true` through `search_memory` end to end on the MCP lane — one tracer path from the tool argument through `effectiveSearchScope` to a now-conditional `ownerScopeFilter`, with a TDD RED observed as a real assertion failure before the store-layer edit, and both a handler-level and a store-level isolation/wiring pin.
- Wired `cross_spine=true` through `list_memory` end to end (mirroring plan 03-02's `search_memory` shape exactly), and made both cross-spine MCP verbs report `searched_scopes`/`scopes_truncated` so a zero-hit cross-spine result is distinguishable from a scope-confined miss — closing the MCP lane for wave 4's Connect wire mirror.
- Mirrored the proven MCP cross-spine lane onto the Connect wire via six additive protobuf fields (`cross_spine`, `searched_scopes`, `scopes_truncated` on `SearchMemories`/`ListMemories`), with the one deliberate divergence from `SearchDiscoveries` — explicit-field-only, never scope-inferred — pinned by a behavioral test.
- Documented the shipped `cross_spine` feature (03-02/03-03/03-04) across the three surfaces an agent actually reads — tool reference, `curating-memory` skill, `CLAUDE.md` — with `searched_scopes` worded as the authorized span it is, never as a hit-distribution report, and a load-bearing "when not to widen" subsection naming the ranking-dilution and extra-scan costs of the opt-in.
- One envelope (`argError`: Fields + Hint + Detail + Class) now carries `validateStoreDiscovery`'s five rejections end-to-end on both the MCP wire string and the Connect error code, closing the D-11a `CodeInternal` misclassification for that validator and proving the shape before the 04-04/04-05 sweep touches thirty more sites.
- A debug-level `slog.DebugContext` line now fires unconditionally on both the allow and deny arm at `internal/store`'s two authz chokepoints, carrying only the D-02 allowlist (satisfied policy IDs, an error count, decision/action/bucket) via a narrow `(authz.Decision).Log()` accessor — `Decision.diag` stays unexported, so a future `cedar.Diagnostic` field structurally cannot leak through a well-meaning call site.
- A 502 from the embedder now names both the status and the provider's own diagnostic text (bounded, verbatim), and both provider lanes drain their response bodies so the connection that carried the error survives it — proven by an httptrace-based `ReuseTracker` observed failing without the drain and passing with it.
- Every remaining single-field and relational rejection reachable from `internal/server/tools.go` (RESEARCH's sixteen Category A sites, three Category B relational sites, plus three unnumbered `parseWindow` sub-checks the plan's own verify gate forced into scope) now carries a machine-readable field attribution and remediation hint, proven by a 23-row matrix and a D-12 value-echo absence gate — with the MCP 401 auth body verified separate and pinned byte-for-byte before any reformat landed.
- `validateStoreRule`/`validateRuleSummary`/`listRules` converted to the field+hint envelope (closing D-11a's third and last unwrapped validator), and all seven of `connectapi.go`'s hand-wrapped `CodeInvalidArgument` sites removed so the failure CLASS — not a hand-wrap — selects the Connect code, proven by a 12-row table with five rows driven through the real `ListMemories`/`SearchMemories`/`StoreDiscovery` handlers and a recorded RED transcript.
- Required-ness for 24 fields across 13 MCP arg structs moved out of the go-sdk's inferred JSON schema and into engram's own Go validation — closing issue #360 at its actual cause (a `validateStoreArgs` a caller's oversized `summary` now names, not `content`) — with `delete_all`'s scope guard landing in the same commit as its schema relaxation, a koanf-configurable `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` bound (D-18), and the embeddings success-decode bound sized to `ENGRAM_EMBED_DIM` (D-16, completing plan 04-03).
- One new docs-site reference page (`errors.md`) makes the field+hint envelope, all ten hint codes, and the Connect class-to-code mapping discoverable; the `curating-memory` skill and `CLAUDE.md` teach an agent to branch on `field=`/`hint=` instead of message wording; `guides/configure.md` gains the `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` config entry and the authz decision-log operator note; and `guides/upgrade.md` names all three wire-visible breaking changes with an explicit "CLI exit codes did not change" reassurance — closing `REQ-error-hint-envelope` and `REQ-authz-decision-diagnostics`, the last two open requirements in the phase.
- `ENGRAM_OPENAI_CHAT_API_KEY` gives the chat/summarize lane its own credential, resolved by `cmp.Or` at the construction site and reachable for Helm users via `memory.summarize.chatApiKeySecret`.
- `reindex --resume` now re-embeds tags-only edits while a paired positive control proves it still skips genuinely unchanged records, and `--dry-run --resume` sizes the repair before it runs — both through one shared tag decoder and one shared skip predicate.
- Corrects `configure.md`'s now-false shared-key assertion, adds `reindex.md`'s pre-patch-resume repair section with its stated D-15 limit, extends `upgrade.md`'s v0.12.0 entries to five, and closes the whole phase with every gate green.
- Restructured the buried "propose a rule" permission into its own subsection with two observable triggers, an inline consent-gated protocol, and a decline record — then mirrored the corrected gate into the tool reference and CLAUDE.md, and closed the investigation requirement by citation.
- Added `### Rule hygiene` (duplicates/contradictions/rot with a code-verified four-row correction table and D-10's user-blessed deletion gate) and `### One-time rule backfill sweep` (D-12's reuse-the-trigger procedure) to `curating-memory/SKILL.md`, then mirrored the same invariants into the tool reference.
- Restored the milestone-completion cadence clause in `### Rule hygiene` citing rule `7smp8vy9hr`'s verified live content, closed `REQ-rule-capture-intervention` by citation to `06-COLD-READ.md`'s PASS, and closed the phase with a full green gate set — the live backfill sweep against the three D-03 candidates is reassigned to the orchestrator as a scaffolded `06-DEMONSTRATION.md`.

---

## v0.11.x Capture & Service Identity (Shipped: 2026-07-26)

**Phases completed:** 5 phases, 19 plans, 46 tasks

**Key accomplishments:**

- A self-contained `internal/authz` cedar-go v1.8.0 PDP (DecideBucket/DecideRecord over four embedded named policies) with a permanent D-08 policy-text regression suite — purely additive, zero `internal/store` wiring.
- Store's bulk read-filter builders (Search/List/ListScheduled/ListScopes/SearchDiscovery) now derive own/shared bucket access from the Plan-01 Cedar PDP via DecideBucket, while emitting byte-for-byte the same Qdrant filter shapes as the pre-Cedar hardcoded Subject switch — proven per-bucket (never per-record), fail-closed, and order-independent.
- GetReadable/getWritable/OwnedOrAbsent now decide via the Plan-01 Cedar PDP's DecideRecord in their record-found branch — a Deny is byte-for-byte indistinguishable from a missing id — and a new hand-authored ADR (engram-cdr1) documents the refinement, completing REQ-cedar-store-enforcement and Phase 22.
- A `Verifier.failClosed` field plus a `NewService` constructor in `internal/auth` that hard-rejects an authenticated service principal resolving to an empty owner at the OIDC verifier boundary, proven as the phase's first test (SC2/D-08/D-09/D-10).
- Opaque static-token bearer lane for `internal/auth`, verified with `crypto/subtle.ConstantTimeCompare` over the full token value, mapping each token to its own distinct owner via the existing `namespacedOwner("static_token", ownerID)` encoding — with a proven no-leak guarantee on the rejection path.
- `chainVerifier` combinator over `mcpauth.TokenVerifier` with a structural JWT-vs-opaque discriminator, D-02 OIDC try-order, and D-03 nil-mechanism deny-by-default guards — zero new interface, zero new Subject variant.
- Additive `service_auth.*` koanf config surface (client-credentials issuer/audience/owner-claims + static-token→owner map) with fail-fast parsing and self-gated validation — the config seam Plan 06 wires into the auth chain.
- Proved SC4/D-07 tenancy isolation and pinned the SC5/D-15/D-16 global-shared-read decision with two permanent tests against the unchanged Phase-22 store filters — zero new production code.
- withAuth (cmd/engram/serve.go) now composes the human OIDC, client-credentials OIDC, and static-token verifiers into a single auth.ChainVerifier at the ONE call site, proven behavior-preserving and lane-independent, with the global shared-read decision recorded as an ADR.
- Payload-only content-fingerprint stamp on `Memory`, a distinct `ErrIdempotencyConflict` sentinel, and two pure helpers — `idempotencyPointID` (deterministic UUIDv5 over owner/scope/key) and `contentFingerprint` (sha256 over client-authored fields) — with zero live call sites yet.
- Wired Plan 01's pure primitives into `store_memory`/`schedule_memory` via a shared `checkIdempotentReplay` check-before-embed helper — keyed replay is now zero-side-effect on match, rejects with `store.ErrIdempotencyConflict` on mismatch, and converges concurrent identical retries to exactly one Qdrant point under `-race`.
- Store.Supersede owner-gates a target via getWritable/ActionWrite, back-stamps its `superseded_by` with a single-key SetPayload (never a re-Upsert), and the recall gate soft-hides superseded records from Search/List while `Store.Get` stays fetchable.
- supersede_memory is now a registered MCP verb — supersedeArgs embeds storeArgs, deps.supersedeMemory resolves the target and delegates the create+back-stamp entirely to Store.Supersede's owner write gate, and connectError's sentinel switch is exhaustive for store.ErrAlreadySuperseded.
- `store.SearchOptions{Tags, Categories, CreatedAfter, CreatedBefore}` replaces Search/SearchReranked's positional tail, with a shared `categoryMatchCondition` OR-helper generalized out of `listFilter` so the list and search lanes cannot drift.
- Added a plural `categories []string` argument to `search_memory` and `list_memory`, wired as a pure passthrough into the already-Categories-capable `coreSearchRequest`/`coreListRequest`, with an explicit ANY/OR jsonschema description so agents don't assume AND symmetry with the adjacent `tags` field.
- Added `repeated string categories = 8` to `SearchMemoriesRequest` (D-10, human-approved as a one-way field-number commitment), wired it into the Connect handler, and proved MCP<->Connect parity with a same-filter-same-order test — closing the search-side half of SC2's category-filter parity gap.
- `ENGRAM_OPENAI_CHAT_BASE_URL` (cmp.Or fallback to the shared base URL) plus a hoisted `internal/openaiurl.Join` that fixes a live doubled-`/v1` bug in the summarizer's endpoint construction.
- Curated `memory`-category records can now carry optional, structured `citations` (file/commit/url/repo provenance anchors) through one relaxed `payload()` write gate, a single `storeArgs.Citations` field inherited by all three write tools via Go field embedding, and a Connect-side compact-view fix that closes an information-disclosure gap MCP's hand-written recall view never had.
- Closed Phase 25's flagged doc gap and finished Phase 26's guidance trio: a Citations section in the curating-memory skill (when to attach vs when not to, never a routine field), citations promoted out of the discovery-only field block and documented on store_memory/schedule_memory/supersede_memory, an explicit ANY/OR `categories` filter row on search_memory/list_memory contrasted against `tags`' ALL/AND, and the `ENGRAM_OPENAI_CHAT_BASE_URL` operator guide entry with the URL-shape rule and shared-API-key constraint spelled out.

---

## v0.10.x Hardening & Write Lane (Shipped: 2026-07-16)

**Phases completed:** 9 phases, 35 plans, 88 tasks

**Key accomplishments:**

- ENGRAM_EMBED_TIMEOUT (default 30s, 0=infinite) replaces the hardcoded embed client timeout, and joinEmbeddingsURL replaces the naive baseURL+"/v1/embeddings" concat with a shape-aware heuristic plus an ENGRAM_OPENAI_EMBEDDINGS_URL verbatim override.
- config.EmbedderIdentity(cfg) mints a v1:-prefixed SHA-256 stamp over the document-side embed config, persisted payload-only (json:"-") through store.Memory and stamped on store_memory/schedule_memory/update_memory/store_discovery/store_rule — with D-06 negative tests locking it off all three verbatim full-response MCP wire paths.
- `engram reindex` now stamps the embedder-config-identity onto every rewritten record via a guarded additive raw-map write that preserves the verbatim-payload owner-key invariant, with a resume skip predicate made identity-aware so a content-match-but-unstamped target is restamped, not silently skipped — completing SC3 across all 5 document-embed write sites.
- Permanent, skip-gated `TestRetrievalEval_AsymmetryDiffer` test that embeds one synthetic string through both `em.EmbedQuery` and `em.Embed` on the production embedder path and fails if the resulting vectors are identical — the Pitfall-12 correctness gate for asymmetric embedding configs.
- New guides/embedding-models.md recipes page with concrete OpenRouter/Gemini/OpenAI/local (TEI/Ollama/vLLM) env blocks, matching commented Helm recipes, and a corrected cross-linked embedding-instructions.md that fixes the stale Gemini task_type guidance.
- Committed a redacted, fail-closed evidence artifact proving the Gemini differ-assertion and qwen3-embedding-8b@4096 recall@8 PASS live, and confirmed the Gemini compat model-id is unchanged.
- Extended engram.proto with six additive write RPCs (StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory), buf.validate wire-shape enforcement including a FieldMask allowlist CEL for UpdateMemory, and regenerated gen/go + gen/ts — 11 RPCs total, additive-only vs main.
- Grep-ban build gate in both `task proto:lint` and the CI `buf` job blocking any RPC from ever being annotated `idempotency_level = NO_SIDE_EFFECTS` (the GET-reachable, CSRF-exploitable annotation per PITFALLS.md Pitfall 2)
- Hand-rolled protovalidate Connect interceptor enforcing buf.validate constraints at request time, wired innermost (after auth) in mountConnect, promoting the protovalidate runtime to a direct go.mod dependency
- Two new Go tests turn Phase 15's SC2/SC3/SC4 success criteria into automated proof: a protoreflect descriptor walk pinning the 11-RPC shape and per-field wire tables, and a table-driven negative matrix asserting the exact Connect code for all six write RPCs across four request shapes.
- HKDF-derived k_csrf sub-key of ui.cookie_key + HMAC-over-Owner double-submit CSRFSigner, stdlib-only, implementing D-08's Owner-only re-seal-stable token binding.
- Write-only Connect interceptor enforcing a session-bound HMAC double-submit token (CodePermissionDenied) between the subject and validate interceptors, threaded from serve.go's real webauth.CSRFSigner through Register/mountConnect, plus a permanent regression matrix proving D-05/D-06/SC2/SC3.
- Go 1.26 stdlib CrossOriginProtection wraps the whole assembled mux with a Connect-shaped permission_denied/403 deny handler, and webauth.Handler.Callback now mints the non-HttpOnly engram_csrf double-submit cookie end-to-end this phase.
- Ordered-list `ClaimIdentity` with a provably-injective non-email owner encoding, comma-list `ENGRAM_OWNER_CLAIM` config plumbing, and a versioned session cookie that invalidates legacy bare-owner cookies on the owner-encoding rollout
- A payload-only vector-preserving store update, a narrow memStore interface, and a single caller-identity seam now thread through every write handler — the four prerequisites 17-04's Connect handler wiring needs to compile.
- D-09 conversion layer (`internal/server/protoconv.go`) for all six write RPCs: mask-driven UpdateMemory mapping (landmine 2 nil-Content, round-8 bool-not-enum shared), outward-rounded RFC3339Nano scheduling-window formatting, and mutationResult/(id,short_id) -> response mappers, built RED->GREEN with exact-mapping table tests.
- Spy-based per-RPC MCP<->Connect delegation parity table with a source/AST wrapper-name assertion, split short_id/UUID cross-owner leak tests, and a fail-closed ENGRAM_REQUIRE_QDRANT CI gate — closing REQ-connect-write-authz-parity and #322.
- Transport-neutral coreListRequest/coreListResult/coreSearchRequest superset contract; deps.listMemory/searchMemory now return typed []store.Memory (no []any), caller-threaded like the write lane, with MCP recall shaping and per-lane defaults moved into the MCP tool closures.
- Handler.Reseal — a best-effort, void-return method that re-seals the AES-GCM session cookie with a fresh absolute expiry past a ½-TTL+skew threshold and refreshes the paired CSRF cookie's Max-Age, proven forward-monotonic under a 50-goroutine `-race` concurrency test, with a pinning test guaranteeing the resolver's hard-expiry check keeps zero skew tolerance.
- New innermost, best-effort `newConnectResealInterceptor` re-seals the session and CSRF cookies on every successful Connect response (read or write), fed `webauth.Handler.Reseal` via a `resealFunc` DI seam threaded from `serve.go` through `Register` into `mountConnect`.
- Structure-preserving re-vendor of the console Connect gen client (all 6 write RPCs + buf/validate dep, real `pnpm check` compile gate, CI drift guard) plus `--destructive` design tokens and a real `destructive` Button variant.
- Two composed Connect-ES interceptors — `attachCsrf` (echoes the `engram_csrf` cookie as `X-CSRF-Token`) and `retryOnce` (a single opportunistic auth-race retry on Unauthenticated/PermissionDenied) — plus a dedicated `engramWrite` client on `[retryOnce, attachCsrf]`, unit-tested against `createRouterTransport` including a composed test proving the retry re-reads a refreshed cookie.
- Host-authoritative DeleteConfirmDialog + ShareWarningInline, and hover/header dropdown-menu row/detail actions (Edit/Delete/Share), all pure presentational — no mutation wiring yet.
- Five memory mutation hooks and three discovery mutation hooks (`createMutation` wrappers over `engramWrite`), with a shared create/schedule-as-shared composite state machine that catches a secondary `SetVisibility` auth failure into a discriminated `created_private` result instead of ever re-issuing the primary create/schedule call.
- Slide-over create/edit forms driving the Plan-04 mutation hooks, backed by a single typed resume-envelope module that survives a real OIDC re-auth redirect without any form ever reading or deleting its own sessionStorage.
- WriteSurfaces host component orchestrating create/edit/delete/share across all three console routes, closing the D-09 re-auth resume loop end-to-end and shipping the rebuilt write-UX SPA in the embedded binary.
- Additive Memory.kind/citations proto fields (21/22) wired through memoryToProto so SearchDiscoveries stops silently dropping discovery fidelity, plus a regression test closing the already-fixed #303 short_id jsonschema gap.
- Collapsed embed.Client.embed()'s two-path body build into one map-based path and unified the reserved-param-key list between internal/config and internal/embed via a config-owned canonical slice (direction reversed from plan to avoid a real import cycle).
- MintShortID now gives up with an errors.Is-checkable ErrShortIDExhausted after 16 real Qdrant collision checks instead of retrying forever, and seen-map dedup hits are free (don't count against the cap).
- Helm chart ships `engram summarize-missing --all-scopes` as an opt-in `batch/v1` CronJob sharing the Deployment's image/env via a new `_helpers.tpl` named template, plus a `task chart:validate` guardrail that pins the shared env block against drift.
- Added a plain `.planning` exclude entry to `.rumdl.toml` (unblocking `task lint:markdown`/`task` default, blocked since Phase 20) and corrected two factual errors in the Phase 21 ROADMAP/REQUIREMENTS acceptance list.
- `Wait()` relocated to a test-only file for both queues, a shared `persistAndEnqueue` helper collapses the duplicated write-path tail, and a leaked-goroutine test is now hermetic — closing WR-03/IN-01/IN-02 from issue #335.
- GitHub App-token self-heal path shipped in `ci.yaml`'s `ui-drift` job; the human provisioned the self-heal App + both credentials and the credential-source was aligned to `secrets.` (10c9c5f1) — all 3 tasks complete. REQ-ci-renovate-spa-drift remains formally OPEN until a live Renovate PR is observed self-healing end-to-end (tracked by #369) — code and infra are done, only the live observation remains.

---

Historical record of shipped milestones. Newest first. Full per-milestone detail
in `.planning/milestones/vX.Y-ROADMAP.md` and `vX.Y-REQUIREMENTS.md`.

---

## v0.9.x — Recall Quality — ✅ SHIPPED 2026-07-10

**Phases:** 9–12 (4) · **Plans:** 12 · **Tasks:** 27 · **Requirements:** 6/6 satisfied
**Shipped:** PR #336 (squash-merge, commit `658795e9`) → `main` 2026-07-10
**Cycle:** opened 2026-07-09 → shipped 2026-07-10 (~2 days)
**Diffstat:** 96 files, +13,811 / −117 (Go deliverable + `gen/` regen + planning docs)
**Audit:** ✅ PASSED 2026-07-11 (`milestones/v0.9.x-MILESTONE-AUDIT.md`) — 11/11 integration links WIRED, 4/4 E2E flows COMPLETE, security 0-open, Nyquist compliant
**Closeout:** override_closeout — 1 acknowledged deferral (docs todo → GitHub #337); see STATE.md Deferred Items

**Delivered:** Recall is now measurably trustworthy — a labeled retrieval-quality eval
harness with always-on similarity scores and a dependency-free reranker (kills the #261
phrasing-sensitive miss: recall@8=1.00), summaries filled asynchronously off the write path,
and per-record usage signals that never touch ranking.

**Key accomplishments:**

1. **Retrieval eval harness** (`internal/retrievaleval`, `task eval:retrieval`) — env-gated Go test seeds the permanent #261 regression fixture through the prod doc-embed path into a Qdrant testcontainer and reports recall@k/MRR + baseline rank/score gap, so ranking/embedding changes are measured, not guessed.
2. **Ranking precision on the lightest lever** — a stdlib-only lexical-overlap reranker shared via `store.SearchReranked` on both MCP and Connect paths surfaces the #261 target at rank 1/8 for both near-verbatim queries (recall@8=1.00, MRR=1.000); no schema change, no reindex, no new dependency. Always-on `search_memory` similarity score shipped alongside.
3. **Asymmetric query/document embeddings reconciled** — `REQ-embedder-native-params` (#305) found already shipped under Phase 4 (native param passthrough + E5/nomic doc prefix); verified and closed, no plans built.
4. **Async-on-write summaries** (`internal/server/summaryqueue.go`) — bounded, non-blocking worker pool drains `Store.FillSummary` after upsert behind a two-switch AND-gate; a gateway outage degrades to "no summary yet" and never fails `store_memory`; drained after HTTP shutdown under a reusable RWMutex+closed concurrency kernel (CR-01), observable on OTLP.
5. **Per-memory usage signals** (`usagequeue.go`, proto 19/20) — `access_count`/`last_accessed_at` incremented only on get-by-id/update, hybrid OTLP-span + payload storage, exposed read-only on recall + Connect, with a hard D-08 invariant (backed by a negative-space test) that usage **never** affects ranking.
6. **Two durable Go kernels from code review** — CR-01 shutdown-safety (`net/http.Server.Shutdown` does not kill active handlers → guard close with RWMutex+closed) and the `*time.Time`-for-optional-timestamps convention (`omitempty` is a no-op on struct values), reused across both async queues.

**Known deferrals / tech debt (all tracked):** GitHub #334 (prod-parity #261 re-confirm on qwen3 @4096, blocked by #333), #335 (P11 review residuals), #333/#332/#331 (embed subsystem), #337 (embedding-model docs). Systemic: `.rumdl.toml` lacks a `.planning` exclude (331 markdown-lint failures on planning docs; Go deliverables clean).

**Deferred to v0.10.x (out of scope):** GitHub #322 (Connect write-lane + CSRF), #323 (session refresh-token rotation).

---

## Prior shipped (pre-GSD-milestone baseline)

These landed before GSD milestone tracking and are recorded here for continuity;
full detail in `.planning/PROJECT.md` (Validated) and `.planning/ROADMAP.md`.

- **v0.8.x Baseline** — Phases 1–7 (shipped). Authorization & isolation, recall semantics, memory kinds & tools (discovery/rule/short_id), embedder, ENGRAM_ koanf config, OTLP telemetry, web console + docs site + bundled plugin. 24 requirements, 56 ADR-locked decisions.
- **Connect Observe-Lane Auth Hardening** — Phase 8 (shipped PR #248/#266; R1–R4 verified 2026-07-08). Cookie/OIDC observe lane replaced the interim anonymous Connect mount.

---

*Latest milestone: v0.9.x — Recall Quality (2026-07-10).*
