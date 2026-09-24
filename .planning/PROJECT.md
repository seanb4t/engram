# engram

## What This Is

engram is a self-hosted, correctable, OAuth-secured memory MCP server for coding agents,
backed by Qdrant. It exposes an explicit, zero-junk memory contract (store / schedule /
search / list / get / update / delete plus discovery and rule kinds) over MCP, with a
ConnectRPC read API, a SvelteKit operator console, and an Astro Starlight docs site. It is
distributed as a container image and a Helm chart (server + Qdrant) for Kubernetes, and — since
v0.16.0 — as a Homebrew cask (`brew install seanb4t/tap/engram`) whose `engram setup` subcommand
registers the server with Claude Code, Codex, and opencode through their own `mcp add` CLIs and
installs the five curation skills in each runtime's native format.

This is a **retrospective baseline** extended by GSD-tracked milestones: engram is already
shipped. Every locked decision and every routed requirement below is **implemented and merged
to main**. This document records the as-built state so future milestones build on an accurate
foundation.

**Latest milestone — 2026-09-18.01 — Bounded Reads — ✅ COMPLETE 2026-09-22** (on branch
`feat/2026-09-18.01`, not yet merged or released): no Qdrant read or provider response can fail
because of unbounded size. Seven phases (1–7); 37 plans, 87 tasks, 20/20 requirements verified,
audit `tech_debt` (0 blockers, Nyquist 7/7, security 7/7). Full detail in
`.planning/milestones/2026-09-18.01-ROADMAP.md`.

**Active milestone — 2026-09-22.01 — Typed Decisions & Recall Ranking** — Phases 1–5 of 5 complete
(2026-09-24); see Current Milestone below.

## Current Milestone: 2026-09-22.01 Typed Decisions & Recall Ranking

**Goal:** Give engram a provider-neutral, advisory-only typed-decision capability (Jev as the
first backend) and use it to make curation and recall measurably better, while fixing the
lexical reranker's paraphrase regression (#605).

**Target features:**
- Provider-neutral decision interface (System One vocabulary: state + Choice/Score/Noul →
  probabilities), off by default; Jev backend over OpenRouter's Decisions API with its own
  base-URL/key settings (OpenRouter Go SDK evaluated first); chat-LLM emulator deferred
- Advisory relation verdicts on `engram spine-review consolidate` (confidence-tiered,
  `updates` option)
- Opt-in Jev relevance reranker on `search_memory`, gated on retrieval eval, with a fallback
  to vector order
- Per-hit relevance probability on search results (absolute "nothing relevant" signal)
- Lexical reranker regression (#605): independent paraphrase eval case, then keep/demote/replace
- Retrieval-eval fixes #353 / #354
- Operator correctness: #508, #476, #504, #502, #501, #503

**Blueprint:** `spike-findings-engram` skill (spikes 001–004, `.planning/spikes/`).

## Current State: 2026-09-18.01 — Bounded Reads ✅ COMPLETE (2026-09-22; ship PR pending)

**Delivered:** no Qdrant read or provider response can fail because of unbounded size — a request
either succeeds or fails with a clear, named error, never an opaque Connect `internal` / HTTP 500.
A unary client interceptor installed once in `store.NewQdrantClient` classifies a receive-limit
overflow (gRPC code AND message shape) into `store.ErrResponseTooLarge`, rendered as
`field=response hint=response_too_large` — Connect `resource_exhausted`, an MCP receiving
middleware, CLI exit `10` on both tiers. Every full-payload `internal/store` read now pages on a
byte budget through two shared primitives (`scrollOrderedPage` for `List`-shaped reads,
byte-budgeted `scrollAllPoints` for sweeps) sized from a per-view record ceiling derived from the
new always-enforced write caps (`ENGRAM_MEMORY_MAX_CONTENT_BYTES` 65536, `_MAX_TAGS` 128,
`_MAX_TAG_BYTES` 128 — decision A); searches are a payload-free vector Query plus a byte-budgeted
id-set fetch. Every recall count knob shares one documented maximum, 1000, and rejects above it
with `out_of_range` (decision B; Connect `limit: 0` redefined, announced BREAKING). The production
client carries a 64 MiB `MaxCallRecvMsgSize` backstop set in exactly one place. A cross-spine
recall keeps its hits when `ListScopes` fails, reporting `scopes_unknown` (#456). The embed and
summarize clients drain responses through a shared `internal/httpdrain` bounded by bytes and time,
and a zero request timeout now resolves to a configurable ceiling instead of unbounded (#457);
the provider error-body truncation is pinned (#347 closed). 7 phases, 37 plans, 87 tasks, 20/20
requirements. Audit `tech_debt` — 6/6 seams, 7/7 E2E flows, 0 blockers, Nyquist 7/7, security
7/7. Full detail archived at `milestones/2026-09-18.01-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`.

**What shipped:**
- **Test harness** — `internal/store/storetest` (lifecycle, `Dial`, two-shape `SeedOversized`) and one dial path for production and every test, AST-gated.
- **Error surface** — `ErrResponseTooLarge` sentinel, one renderer, Connect/MCP/CLI mapping, `errors.md` hint table bound to `argerror.go` by a doc gate.
- **Bounded reads** — `scrollOrderedPage`, byte-budget `scrollAllPoints`, two-phase search fetch, per-sweep projections for every operator sweep; a 27-site inventory with every site migrated or exempted in writing.
- **Contracts** — decision A (write caps) and decision B (recall maximum 1000, reject-over-clamp) recorded below; `scopes_unknown` additive on proto (MCP/Connect/CLI footer).
- **Provider bounds** — `internal/httpdrain`, six new `ENGRAM_{EMBED,SUMMARY}_{DRAIN_BYTES,DRAIN_TIMEOUT,MAX_TIMEOUT}` keys, upgrade-guide §18.

**Standing constraints held:** zero new Go dependencies; no test asserts grpc-go's own 4 MiB
default (rule `m45p2b4bp7`); the red-evidence mutation harness that phases 1–7 grew to 63 patches
was removed before close (`c1afd6c1`, rule `3p0zsqrhmb`: no tests for tests) — the behavioural
regression tests it pointed at remain.

**Carried tech debt:** GitHub #585/#456/#457 stay open until the ship PR closes them; phase 07's
`VALIDATION.md`/`SECURITY.md` were written retroactively at close — the `verify:post` hooks did not
dispatch for a second consecutive milestone, root cause undiagnosed; 07 WR-02
(`Config.Validate` does not cross-check a provider `Timeout` against its `MaxTimeout`, harmless).

**Closeout:** `override_closeout` — 2 newly acknowledged (the removed harness's timeout entries),
2 carried forward (see STATE.md Deferred Items).

<details>
<summary>Previous: 2026-09-13.01 — Setup v2 ✅ SHIPPED (2026-09-17, v0.17.0; observed 2026-09-18)</summary>

**Delivered:** `engram setup` is safe to re-run against a real machine. Preview reads the
runtime's actual registration back through the runtime's own read verb (`claude mcp get`,
`codex mcp get --json`), parses it totally, and classifies it as exactly one of
`already-correct` / `would-write` (naming the differing facet) / `preserved` — a registration
carrying anything setup did not author, including a hand-pasted literal header. `--apply`
consults that same classification before any action and performs zero registration writes on
`preserved` or `already-correct` — never Claude Code's tolerant `mcp remove` — closing the
2026-09-10 overwrite incident (gotcha `ryr82bf2s2`); a reproducible rewrite of an OAuth-shaped
Claude Code entry says up front that the user will need to log in again. Every observed header
value is redacted by construction (compared in locals, never retained). `--header NAME=ENVVAR`
(repeatable, `ENGRAM_HEADERS`-defaulted) expresses a gateway registration on Claude Code,
opencode, and `generic` as a bare env reference in each runtime's own syntax; Codex declines it
with a named reason rather than gaining a TOML writer. Claude Code and Codex receive skills,
hooks, and `/engram-setup` plugin-first through their own plugin CLIs (three-way plugin state;
the native copy stays for opencode/`generic` and as the fallback, never both). `osRun` names a
deadline kill instead of misreporting a clean exit (#560), and the binary generates byte-stable
man pages the cask installs (28 pages) and removes symmetrically. 5 phases, 19 plans, 45 tasks,
23/23 requirements. Audit `passed` (third pass) — 10/10 seams, 4/4 E2E flows, Nyquist 5/5,
security 5/5 (88 threats closed). Live observation of v0.17.0 recorded in
`05-RELEASE-0.17.0.md`. Full detail archived at
`milestones/2026-09-13.01-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`.

**What shipped:**
- **Read-only drift detection** — `Observe` → `Compare` in `internal/setup/drift.go`, one classification shared by preview and apply; `DisallowUnknownFields` + key-set diff (codex) and line-classified labels (claude-code) make unknown content `preserved`, never a false `already-correct`.
- **Apply-time preserve gate** — `case OutcomeAlreadyCorrect, OutcomePreserved: return res` sits above the write loop; no `--replace` flag exists, the row names the runtime's own manual step.
- **Custom auth headers** — `HeaderSpec{Name, EnvVar}`, per-runtime rendering with no shared formatter, `Authorization` rejected as a header name, values never on argv.
- **Plugin-first delivery** — `PluginRuntime` probe (`plugin list --json`), marketplace add only when absent (engram's own marketplace, never a foreign one re-pointed), install/update/remove-then-add per runtime, `skills.DetectPresence` read-only leftovers report.
- **Executor correctness + man pages** — `ctx.Err()` before `*exec.ExitError`; `engram man <dir>` via `cobra/doc.GenManTree` with an epoch-pinned header and a tree-restoring wrapper; cask `post_install`/`post_uninstall` hooks.

**Standing constraints held:** zero new Go dependencies (`cobra/doc` promoted indirect→direct);
`internal/setup` stays a stdlib-only leaf; no test invokes a real third-party CLI or touches the
operator's `$HOME` (rule `m45p2b4bp7` — the literal-echo shapes are maintainer-run observation
records, `04-OBSERVATIONS.md` and `05-RELEASE-0.17.0.md`).

**Carried tech debt:** Claude Code's `oauth-client` read-back label (`OAuth: client_id
configured, callback_port N`) is unrecognized content to the scanner, so a setup-authored
`oauth-client` registration re-reads `preserved` (safe, teachable); two accepted review
info-findings (03 IN-01 `setupNativePresenceSummary` on a bare symlinked dir; 04 IN-01 a dead
codex header-comparison branch); the `verify:post` step hooks (secure-phase, validate-phase)
never dispatched during this milestone — SECURITY.md and VALIDATION.md were reconciled
retroactively at close, root cause undiagnosed; five cross-milestone `WINDOWS.md` entries.

**Closeout:** `verified_closeout` — open-artifact audit clear, 0 acknowledged, 0 carried forward.

</details>

<details>
<summary>Previous: 2026-08-23.01 — Distribution & Agent Bootstrap ✅ SHIPPED (2026-09-12, v0.16.0)</summary>

**Delivered:** engram is installable in one command and self-configuring across every agent
runtime it targets. `brew install seanb4t/tap/engram` installs an unsigned static binary through a
cask whose post-install hook strips quarantine *before* the version gate can be SIGKILLed by
Gatekeeper, then asserts `engram version --output json` matches the declared artifact. `engram
setup` detects runtimes by their own binaries, previews the exact argv it would issue, and under
`--apply` registers the server with Claude Code, Codex, and opencode through their own `mcp add`
CLIs — engram parses or writes no third-party config format — converging on re-run and reporting
per-runtime rows with a three-way exit taxonomy. The five curation skills ride inside the binary,
byte-identical to the plugin, and land natively in each runtime (plus a delimited, re-detectable
AGENTS.md index for Codex). `/engram-setup` delegates to the binary when present and keeps a
first-class prose path when absent, the mechanical prose generated from the same Plans the CLI
executes with a CI gate that fails on drift. 6 phases (1–6), 21 plans, 49 tasks, 25/25
requirements. Audit `tech_debt` — 12/12 cross-phase seams wired, 8/8 E2E flows complete, 0
blockers, Nyquist 6/6 COMPLIANT. Full detail archived at
`milestones/2026-08-23.01-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT,INTEGRATION}.md`.

**What shipped:**
- **Homebrew cask, correct by construction** — `homebrew_casks:` (not the deprecated `brews:`) publishing to `seanb4t/homebrew-tap` via a dedicated tap-publisher App whose token is the bare `{{ .Env.HOMEBREW_TAP_TOKEN }}` form GoReleaser's raw-string regex requires (#516 — the v0.15.0 tap push failed on a conditional, invisible to `goreleaser check` and `--snapshot`). A newest-tag `skip_upload` guard computed in the workflow keeps a `workflow_dispatch` backfill from regressing the tap (D-15: accepted by construction, no staged rehearsal), and a read-only `verify-tap-credential.yaml` probe proves push access without a release. Observed live for v0.16.0 on all four platform archives plus a tagged `go install`.
- **`engram version --output json` and a real dev version** — the machine-readable install-time contract, pinned byte-equal to the text lane; local builds report `X.Y.Z-dev.0+g<hash>[.dirty]` from `debug.ReadBuildInfo` with the release-please-managed `lastRelease` const drift-tested against the manifest.
- **`engram setup` core** — env-first through `config.Load`, preview-by-default with `--apply` behind `registerDestructive`, exit 0 / 8 (partial) / 9 (failed) with every runtime's outcome reported independently, and `--help` naming every runtime and auth mode (D-00 correct-by-reading).
- **Runtime registration via the runtime's own CLI** — Codex and opencode through `mcp add` with read-verb probes for wrote/already-correct; Claude Code through a tolerant `mcp remove` clearing the slot before a fatal `mcp add` (its `add` refuses on existing, live-verified at both scopes); opencode's `Authorization=Bearer {env:…}` KEY=VALUE header replacing a confirmed-broken colon-space form; a `generic` opt-in pseudo-runtime emitting a portable `mcpServers` document with zero actions and no subprocess. Four auth modes (`oauth`, `oauth-client` + validated `--client-id`, `bearer`, `none`) with secrets only ever as env-var references, never on argv. A CLI that is absent or answers with an unexpected surface fails naming the runtime and what was expected.
- **Skills distribution from the binary** — `task skills:vendor` + `//go:embed all:data` with a byte-equality drift gate against `skill/engram/skills`; native installs for Claude Code (`~/.claude/skills`), Codex (`$HOME/.agents/skills`, human-confirmed its selector reads there), and opencode (XDG-aware config root); every SKILL.md carrying a `metadata.engram-summary` index entry; Codex's AGENTS.md index spliced in place through symlinks, hard-failing with byte offsets on any ambiguous prior state, and — after audit blocker B01 (#559) — preserving an unreadable index with zero writes, with only `fs.ErrNotExist` as the create case.
- **Slash-command delegation, equivalent by construction** — `/engram-setup`'s four-mode delegation tables and Claude-only fallback are generated from real setup Plans through `internal/setupgen`, exercised against actual Cobra, and guarded by a read-only local drift comparator plus a CI regenerate-and-diff gate whose both failure paths were proven.
- **Install documentation on shipped behavior** — canonical `guides/install.md` and `guides/agent-setup.md`, Quickstart/CLI/plugin entry points reconciled, and a D-10 post-release handoff that recorded the qualifying v0.16.0 observations before the requirement was checked off.

**Standing constraints held:** zero new Go dependencies (only `go.yaml.in/yaml/v3` promoted from
indirect to direct for skill frontmatter); every runtime writer is a shell-out to the runtime's
own CLI, so no TOML/JSONC parser entered the tree; verification never invoked a real third-party
CLI or touched the operator's `$HOME` from a test (rule `m45p2b4bp7`).

**Carried tech debt:** W01 — `osRun` converts a deadline-killed subprocess's `*exec.ExitError` to
exit -1 / nil error without consulting `ctx.Err()`, bypassing the executor's timeout path (the
process is still killed) — #560; a duplicate failed-count calculation, auth/runtime validation
ordered before the missing-URL check, and a stale "verbatim" capture comment in the setup core;
the "emits no warning" half of the native-format human check was never captured. Cursor support
(`REQ-register-cursor`), drift detection, hand-edit reconciliation, and shell completions /
man pages via the cask are carried as v2 candidates.

**Closeout:** `override_closeout` — 2 open artifacts acknowledged at close, 0 carried forward, both
Phase 04 deferred-items entries that no longer describe a live condition (a keylinks gate that now
passes; a one-run testcontainer flake). Full disclosure in STATE.md `## Deferred Items`.

**Carried caveat:** the deployed engram server still predates v0.11.x, so nothing from the last
five milestones is callable in practice until the next rollout — `engram setup` registers a client
against whatever server URL it is given, so the bootstrap works today; the server-side features do
not.

</details>

<details>
<summary>Previous: 2026-08-12.01 — Record State & Schema Evolution ✅ SHIPPED (2026-08-22)</summary>

**Delivered:** a record's full state — supersession, scheduling, archival, and its own schema
version — is now reachable and legible on every lane, and payload evolution has a real mechanism
instead of another one-shot operator command. `schema_version` landed as an absent-safe
discriminator: stamped on every write, visible on the wire, and structurally incapable of narrowing
recall — a record predating the field reads as v0 by absence and needs no backfill. Behind it,
`internal/migrate` turned migrations into an ordered registry of additive-only steps that must
declare their own reversibility at registration, swept to convergence by `Store.Migrate` without a
collection lock, and driven by `engram migrate` through the existing `registerDestructive`
admission gate. Nothing migrates automatically — startup at most warns. The operator tier collapsed
to one serialization plus a view, so a json document can no longer widen past the text sentence
beside it. 9 phases (1–9), 46 plans, 121 tasks, 27/27 requirements. Audit `tech_debt` — 5/5
cross-phase integration seams wired, 3/5 E2E flows traced, 0 blockers, Nyquist 9/9 COMPLIANT. Full
detail archived at `milestones/2026-08-12.01-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`.

**What shipped:**
- **`schema_version` as an absent-safe discriminator** — stamped by 100% of write paths (proven by test, not sample), wire-visible on `get_memory` and `full=true` recall rather than a `json:"-"` audit stamp, forward-compatible in both directions so a binary rollback across a schema change is safe. A runtime gRPC-interceptor gate proves the field never reaches any Qdrant recall or authz filter — the `superseded_by`/`archived_at` `IsEmpty` idiom has inverted cardinality here, and copying it would have excluded every pre-migration record from recall.
- **`internal/migrate` step registry and sweep** — a stdlib-only leaf package (mirroring `internal/surfaces`/`internal/openaiurl`) with sealed `Reversibility`, a registration invariant that makes both non-additive and reversibility-silent steps a build/test failure, and `Store.Migrate`'s re-derive-every-pass sweep. Convergence needs no collection lock because the write path stamps before the sweep runs; partial `SetPayload` application (qdrant/qdrant#9371) is survived and resumed, proven against a real pinned Qdrant with forced mid-sequence failures.
- **`engram migrate` (`status` / `revert`)** — routed through a generalized `registerDestructive` gate with a deliberately *named* `--apply`-required union rather than one derived from the same predicate. `status` reports a version histogram, not a scalar — mixed-version collections are legitimate mid-rollout. `revert` runs declared inverses in reverse order and refuses the whole range at the first irreversible step rather than leaving the collection between versions. `backfill-short-ids` became the registered v0→v1 step and a thin delegating alias.
- **Connect record-state parity** — eight additive `Memory` fields in one pass, proven by an exhaustive field-mapping round-trip test rather than by `buf breaking` passing and the code compiling. That mistake had recurred three times (v0.8.x, v0.11.x, v0.13.x); a reflection-based detector now makes a silently-dropped field structurally impossible.
- **Typed operator renderer** — text and json derive from ONE shared ordered field set across 15 commands, with zero new per-report renderer code, so field-set identity holds by construction. `--output text` is now published as an explicitly unstable human-readable view; json is the contract.
- **Record state on every surface** — three orthogonal `include_archived`/`include_superseded`/`include_scheduled` opt-ins thread from proto through `Store.List`/`Store.Search` to CLI flags and console checkboxes, with one tested `memoryStateWords` derivation filling a STATE column and achromatic console badges. `engram get` shipped, `engram migration-status` exposes the histogram over a sixth Connect read RPC, and a migration advisory renders on every console route — silent at zero, loading, and failure alike.
- **Gate and CI integrity, taken first** — `internal/keylinks` closed the silent-no-op key-link shapes (`\\`-escaped patterns that survive into `new RegExp` unmatchable rather than throwing), repairing 39 patterns across 20 plans and reassessing all 30 v0.13.x links. One shared CI Qdrant replaced four per-package testcontainers, with a go/ast gate proving no test store construction bypasses its package's collection-prefix seam.

**Standing constraints held:** zero new Go dependencies — every capability landed on stdlib or
already-vendored `cobra`/`qdrant-go-client`, as in the two prior milestones.

**Carried tech debt (all parked in backlog phases):** no full-stack E2E for `engram migrate` — the
status → apply → status reconvergence path has no live-Qdrant coverage (999.2); CLAUDE.md's "every
surface renders a record's derived state" overstates, since the MCP lane exposes raw fields but
derives no state words (999.3); and `schema_version` is typed three ways in one proto file — `uint32`
at `:52`, `int32` at `:186` and `:204` (999.4).

**Closeout:** `override_closeout` — 8 open artifacts acknowledged at close, 0 carried forward. Five
were stale text for work independently confirmed resolved; three are genuine (a superseded research
todo, the `ui/` svelte-check/TypeScript environment gaps, and two Phase 07 UAT checks needing a live
server plus Qdrant). Full disclosure in STATE.md `## Deferred Items`.

**Carried caveat:** the deployed engram server still predates v0.11.x, so nothing from the last four
milestones is callable in practice until the next rollout.

</details>

<details>
<summary>Previous: v0.13.x — Curation & Self-Evidence ✅ SHIPPED (2026-08-12)</summary>

**Delivered:** the memory spine is maintainable without hand-curation, and the interface states its
own rules. `engram spine-review` resolves the **structural** spine predicates a command can decide —
drifted citations, near-duplicate candidates, purge eligibility, an archive tier — as the sixth
instance of the existing Subject-less operator tier, never a new authorization path; a companion
`curating-spine` skill carries the **semantic** judgments a CLI cannot make ("is this still true",
"are these the same fact"), proposing every mutation and stopping at `store_rule`'s consent gate
verbatim. Alongside it, the CLI stopped being learnable only by trial: one exit-code taxonomy governs
every command, cobra enforces the flag exclusivity the help text had merely claimed, every RPC path
carries a finite deadline, and each server-side conditional rule is declared once and machine-proven
present on every surface that advertises it. `supersede_memory` grew multi-target merges so a
duplicate set collapses to one survivor with history preserved for every predecessor. 6 phases
(1–5 plus inserted 03.1), 33 plans, 99 tasks, 23/24 requirements. Audit `tech_debt` — 6/6 cross-phase
integration seams wired, 4/4 E2E flows traced, 0 blockers. Full detail archived at
`milestones/v0.13.x-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`.

**What shipped:**
- **One exit-code taxonomy, unified not bounded** — client verbs and all six operator commands resolve through the same 2/4/5 vocabulary via `classifyOperatorErr`, plus `exitTimeout=6` (client deadline, distinct from an unreachable server) and `exitFindings=7`. `MarkFlagsMutuallyExclusive` replaces hand-rolled guards, so the `fmt.Errorf` that bypasses `cliError` cannot reappear; `--timeout` gives every RPC a finite deadline, proven against connect-go's real hung-server behavior (#453/#467/#452).
- **Correct-by-reading surfaces** — `internal/surfaces` (stdlib-only) declares each conditional rule once; a conformance gate derives applicability from the rule's own fields and proves its canonical sentence present across cobra usage, jsonschema tags, MCP descriptions, proto comments, docs-site, and skill markdown. An unexported `declared` marker makes an off-registry rule a compile-time impossibility. All 15 MCP tools advertise `readOnlyHint`/`destructiveHint`/`idempotentHint`/`openWorldHint`, and every `--help` plus the bare catalog JSON is pinned behind goldens generated from the live cobra tree.
- **`engram spine-review`** — `scan`/`verify`/`consolidate`/`purge`/`archive`/`restore`: a four-tier citation classifier (valid/moved/broken/unverifiable) with a resolved-path safety gate, near-duplicate ranking that reuses stored vectors via `NewQueryID` with no re-embedding and no default threshold, an orthogonal `archived_at` soft-hidden state, and a `PurgeManifest` whose provenance is compiler-enforced rather than runtime-checked.
- **Preview-by-default destructive tier** — `registerDestructive` makes `--apply` a runtime RunE choke point; `prune-expired` and `migrate-remap-owner` flipped to preview-first. `--output json|text` with TTY auto-detection backfilled across all six pre-existing operator commands.
- **Multi-target merge supersession** — `supersedes` takes a set; the back-stamp-failure path became a classified reconciliation pass proved against a real pinned Qdrant with a forced mid-sequence partial failure; `idempotency_key` arrived with a target-set-keyed `mergeFingerprint` checked before the already-superseded stage, so a reordered set replays and a different set conflicts (#342 follow-on).
- **Semantic curation skill** — `curating-spine` judges staleness and near-duplicate identity using only shipped MCP tools, zero server-side code, reusing the consent block byte-identically.
- **Nyquist debt cleared** — all six phases at `status: validated`, closing the v0.12.x inheritance (six `status: draft` plus one phase with none), with #355's drifted `tools.go` anchors repaired by citing symbols instead of line numbers.

**Standing constraints held:** zero new Go dependencies — every capability landed on stdlib or
already-vendored `cobra`/`qdrant-go-client`, as research predicted.

**Known gap:** `REQ-consent-adversarial-proof` is **NOT SATISFIED**. The adversarial cold read ran
and exhausted its locked 3-run cap with every run landing NOT-TEMPTED — the reader's identity verdict
was correct each time, so the confidently-wrong proposal the criterion must observe was never
produced. Terminal verdict NOT-OBTAINED, neither pass nor fail; the non-result was accepted rather
than converted into a green checkbox. Tracked open at `WINDOWS.md` id 3.

**Carried tech debt:** two Phase 03 TDD deviations where RED was genuinely observed but RED+GREEN
landed in one commit (`WINDOWS.md` ids 1, 2); three waived human-verification items in Phase 03 (two
prose cold reads, one real-pty `--output` check); a stale rationale comment at
`internal/surfaces/toolclass.go:141-142` that contradicts the shipped `idempotency_key` support
(annotation value correct, comment wrong); and `TestExitCodeBaseline`'s env-var fragility (#476).
The v0.12.x two-tier CLI error model debt is now absorbed by the unified taxonomy.

</details>

<details>
<summary>Previous: v0.12.x — Headless Reach & Diagnosability ✅ SHIPPED (2026-08-02)</summary>

**Delivered:** engram is now reachable by agents that are not a top-level MCP client, and what the
server decides and rejects is legible to whoever is on the other end. A single composed verifier
chain serves both the MCP and Connect lanes, so a bearer token resolves to the identical actor on
either — and Connect can now be mounted headless, without the web UI. `engram search | store | list`
gives an agent a real CLI over the typed Connect stubs. Recall gained `cross_spine` on both the MCP
and Connect lanes and, in the milestone's last phase, on the CLI itself. Rejections stopped being
prose: one `field=<name> hint=<code>` envelope now names the failing field and a machine-stable
remediation code across every validator on both wires. 7 phases (1–7), 28 plans, 68 tasks, 21/21
requirements verified. Audit `tech_debt` — 5/5 cross-phase integration seams wired, 2/2 E2E flows
re-traced, 0 blockers. Full detail archived at
`milestones/v0.12.x-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`.

**What shipped:**
- **Shared auth chain + Connect bearer identity** — `buildAuthChain` is the sole verifier-construction site, injected into both lanes, so the two provably cannot drift; token expiry is actually enforced; the CSRF exemption is re-keyed off a compiler-enforced per-request lane stamp rather than any caller-controlled signal, and an unrecognized lane fails closed (#343).
- **Headless mount + headless CLI** — `connect.headless` is a default-off, fail-closed opt-in that refuses startup with zero configured auth lanes; `engram search | store | list` runs over the generated Connect stubs with agent-shaped output and a per-file client import-boundary gate (#343).
- **Cross-spine recall, end to end** — `cross_spine` on `search_memory`/`list_memory`, mirrored onto the Connect wire via six additive protobuf fields, with `searched_scopes`/`scopes_truncated` so a zero-hit cross-spine result is distinguishable from a scope-confined miss — then wired through to the CLI in Phase 7, closing the seam the milestone audit itself found (#344).
- **Diagnosability** — one `argError` envelope (Fields + Hint + Detail + Class) carries every rejection on both wires; authz decisions emit a bounded allowlist debug line at the two `internal/store` chokepoints; embedder 502s name the provider's own diagnostic text and drain their bodies so the connection survives; a new docs-site `reference/errors.md` makes all ten hint codes discoverable (#394/#360/#347).
- **Operator config & reindex correctness** — `ENGRAM_OPENAI_CHAT_API_KEY` gives the chat lane its own credential; `reindex --resume` re-embeds tags-only edits and `--dry-run --resume` sizes the repair first (#350/#345).
- **Rule capture, investigated then fixed** — the buried propose-a-rule permission became its own subsection with two observable triggers and an inline consent-gated protocol, plus rule-hygiene and backfill-sweep procedures; validated by a cold read in which a fresh agent with zero phase context unprompted named the trigger, proposed correctly, and stopped at consent (#351).

**Standing constraints held:** zero new Go dependencies across the whole milestone — every
capability landed on an existing seam (connect-go, cedar-go, `log/slog`, OTel, stdlib).

**Carried tech debt:** the two-tier CLI error model is undocumented (Phase 7's client-side
`validateScopeCrossSpine` returns a plain usage error with exit 2 rather than the field+hint
envelope — intentional and correctly scoped, but no REQ or decision ID records the exemption), and
no phase has a reconciled Nyquist `VALIDATION.md` (six sit at `status: draft`, phase 2 has none).

**Carried caveat:** the deployed engram server predates this merge, so no v0.12.x capability is
callable until the next release.

</details>

<details>
<summary>Previous: v0.11.x — Capture & Service Identity ✅ SHIPPED (2026-07-26)</summary>

**Delivered:** programmatic capture is now correct and re-runnable, and headless service principals
have a first-class, isolated identity. Authorization moved onto a real ABAC policy engine without
changing a single observable behavior; agents can write memory mechanically (idempotency keys),
correct it without losing history (supersession), and attach structured provenance (citations).
5 phases (22–26), 19 plans, 46 tasks, 11/11 requirements verified. Audit PASSED — 6/6 cross-phase
integration seams wired, 2/2 E2E flows complete, 0 blockers. Full detail archived at
`milestones/v0.11.x-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`.

**What shipped:**
- **Cedar authorization foundation** — `internal/authz` (cedar-go v1.8.0) decides over enumerable buckets; the store compiles the decision into the Qdrant filter. Byte-for-byte behavior-preserving; ADR `engram-cdr1` refines LOCKED `DEC-cgb` rather than overriding it (#362/#373).
- **Service identity** — pluggable verifier chain (OIDC user → client-credentials → static token) at one wiring site, with the milestone's #1 risk (a service principal resolving to `owner==""`) proven fail-closed as the phase's first test (#362/#373).
- **Correct, re-runnable capture** — optional `idempotency_key` with a deterministic UUIDv5 point ID and reject-not-overwrite semantics (#340); `supersede_memory` additive links with recall soft-hide (#342).
- **Provenance and recall precision** — optional structured citations on any category (#341), plus a `categories` OR-filter at MCP↔Connect parity applied ahead of vector ranking (#374).
- **Per-lane embedder/chat config** — `ENGRAM_OPENAI_CHAT_BASE_URL` and a shared shape-aware URL join that fixed a live doubled-`/v1` bug in the summarize lane (#350).

**Standing constraints held:** zero new store-layer authz **primitive**, and (except `cedar-go`)
zero new dependencies — every feature extended an existing seam.

**Carried caveat:** the deployed engram server predates this merge, so `supersede_memory`,
citations, and the `categories` filter are not callable until the next release.

</details>

<details>
<summary>Previous: v0.10.x — Hardening & Write Lane ✅ SHIPPED (2026-07-16)</summary>

**Delivered:** engram became production-solid and writable over Connect. The embedder-reliability
gaps from the v0.9.x eval brownouts were fixed, the Connect write lane shipped end-to-end with CSRF
+ stateless session-rotation hardening and full MCP↔Connect authz parity, and the correctness/CI
backlog was cleared. 9 phases (13–21), 19/20 requirements verified; the one deferred requirement
(REQ-ci-renovate-spa-drift's live self-heal observation) is post-merge-only and tracked by #369.
Full detail archived at `milestones/v0.10.x-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`.

**What shipped:**
- **Embedder reliability & options** — configurable HTTP timeout (#333), base-URL `/v1` join fix (#332), Gemini direct (#331), prod-parity #261 re-confirm (#334, closes #261), model docs + Helm recipes (#337).
- **Connect write lane + auth hardening** — 6 additive write RPCs + CSRF (#322), MCP↔Connect authz parity, stateless session rotation (#323), full console write UX.
- **Correctness & polish** — SearchDiscoveries proto fidelity (#307), MintShortID cap (#308), embed/discovery polish (#302/#303/#304), summarize CronJob (#269).
- **CI / maintenance hygiene** — Renovate vendored-SPA self-heal (#301, live obs pending #369), Phase-11 review residuals (#335), `.rumdl.toml` `.planning` exclude.

</details>

## Core Value

**Correctable recall precision** — a coding agent gets back the RIGHT memory for its context,
and wrong or stale memories can be corrected or superseded, so recall stays trustworthy as the
store grows. Everything ladders up to this: relevant/correct/current recall, explicit
zero-junk capture (no auto-extraction), and per-actor isolation that keeps each agent's recall
clean.

## Requirements

### Validated

Shipped and relied upon. Baseline IDs/phase mapping and the full v0.9.x requirement
text are archived in `.planning/milestones/v0.9.x-REQUIREMENTS.md` (which embeds the full
pre-close `REQUIREMENTS.md` snapshot).

**v0.8.x baseline (Phases 1–7):**

- ✓ **Authorization & Isolation** — per-actor isolation, typed Subject authz, configurable-claim owner (Phase 1)
- ✓ **Recall Semantics** — scheduled/windowed recall, cursor paging, summary-by-default (Phase 2)
- ✓ **Memory Kinds & Tools** — discovery kind, rule kind, short_id handle, schedule tools (Phase 3)
- ✓ **Embedder** — protocol-named vars, asymmetric query/document param passthrough (Phase 4)
- ✓ **Config & Transport** — ENGRAM_ koanf config, Config.Validate, fatal legacy guard, MCP path (Phase 5)
- ✓ **Telemetry & Observability** — slog + OTel over OTLP at every seam, non-blocking (Phase 6)
- ✓ **Web UI, Docs Site & Distribution** — operator console SPA, docs site, brand, bundled client plugin (Phase 7)
- ✓ **Connect Observe-Lane Auth Hardening** — cookie/OIDC observe lane (R1–R4) (Phase 8; PR #248/#266)

**v0.9.x — Recall Quality (Phases 9–12; shipped 2026-07-10, PR #336):**

- ✓ **REQ-retrieval-eval** — reproducible retrieval eval (`task eval:retrieval`, #261 fixture, recall@k/MRR) — v0.9.x
- ✓ **REQ-search-similarity-scores** — always-on per-result similarity score in `search_memory` — v0.9.x
- ✓ **REQ-ranking-precision** — dependency-free reranker kills phrasing-sensitivity (recall@8=1.00 on #261) — v0.9.x
- ✓ **REQ-embedder-native-params** — native query/document param passthrough + doc-side prefix — v0.9.x (already shipped under Phase 4)
- ✓ **REQ-async-summaries** — async-on-write summary fill off the write path, eval-gated — v0.9.x
- ✓ **REQ-usage-signals** — per-record usage counters (get/update), hybrid OTLP+payload, never affects ranking — v0.9.x

**v0.10.x — Hardening & Write Lane (Phase 13 shipped 2026-07-11):**

- ✓ **REQ-embed-timeout** — operator-tunable `ENGRAM_EMBED_TIMEOUT` replaces the hardcoded 30s embed HTTP timeout; summary-queue backoff budget re-derived from it (#333) — Phase 13
- ✓ **REQ-embed-baseurl-join** — shape-aware base-URL → `/embeddings` join across OpenAI/OpenRouter/Gemini shapes with operator override (#332) — Phase 13
- ✓ **REQ-embed-config-identity** — `v1:`-prefixed embedder-config-identity stamp on all 5 document-embed write sites (incl. `engram reindex`), payload-only (`json:"-"`, no wire leak), identity-aware reindex resume (DECISION 3) — Phase 13

**v0.10.x — Hardening & Write Lane (Phase 14 complete 2026-07-11):**

- ✓ **REQ-embed-gemini-direct** — direct Gemini embeddings via the OpenAI-compat `/v1beta/openai` endpoint using the instruction-prefix asymmetry (`ENGRAM_EMBED_QUERY/DOCUMENT_INSTRUCTION`, not the silent-no-op `task_type`); proven live by the skip-gated `TestRetrievalEval_AsymmetryDiffer` differ-case (query≠document @3072) with the confirmed `gemini-embedding-2` model-id (#331) — Phase 14
- ✓ **REQ-embed-prod-parity-eval** — #261 recall@8=1.00 re-confirmed live on the prod-parity `qwen3-embedding-8b`@4096 config; committed fail-closed eval evidence (`14-EVAL-EVIDENCE.md`); closes #261/#334 — Phase 14
- ✓ **REQ-embed-model-docs** — `docs-site` `guides/embedding-models.md` + Helm `values.yaml` commented recipes for OpenRouter/Gemini/OpenAI/local (TEI/Ollama/vLLM), each pairing base URL + model + dim + query instruction with an `engram reindex` callout (#337) — Phase 14

**v0.10.x — Hardening & Write Lane (Phases 15–16 complete 2026-07-11):**

- ✓ **REQ-connect-write-rpcs** — the six additive write RPCs (`StoreMemory`/`StoreDiscovery`/`UpdateMemory`/`DeleteMemory`/`SetVisibility`/`ScheduleMemory`) exist in the Connect `EngramService` wire contract with `buf.validate` annotations (UpdateMemory FieldMask allowlist CEL, category enum, SetVisibility zero-value rejection) + a hand-rolled `protovalidate` interceptor ordered after auth (401 before 400); additive-only (`buf breaking` clean), and provably unreachable over an unauthenticated GET (embedded `UnimplementedEngramServiceHandler` stubs + a `idempotency_level = NO_SIDE_EFFECTS` build gate mirrored in CI + descriptor & negative-matrix regression tests). Handler bodies deferred to Phases 16–19 (#322) — Phase 15
- ✓ **REQ-connect-csrf** — the write lane's transport CSRF defense lives in two coordinated layers before any write RPC runs: (1) Go 1.26 stdlib `net/http.CrossOriginProtection` wraps the whole top-level handler (same-origin primary defense, `cmd/engram/serve.go`) with a `SetDenyHandler` emitting a Connect-shaped `permission_denied`/403; (2) a `newConnectCSRFInterceptor` double-submit HMAC token check (HKDF sub-key of `ui.cookie_key`, bound to the resolved `Subject.Owner` only so it survives the Phase-18 re-seal), placed `subject → CSRF → validate`, gated to the six write Procedures via generated constants (the 5 read RPCs stay exempt). Non-HttpOnly+Secure `engram_csrf` cookie minted in `webauth.Handler.Callback`; permanent regression gates for no-anonymous-write across all 6 writes, read-allowlist exemption, and `TestConnectNoCORSHeaders`. Verified 9/9 must-haves; flagged for `/gsd-secure-phase` (#322) — Phase 16

**v0.10.x — Hardening & Write Lane (Phase 18 complete 2026-07-13):**

- ✓ **REQ-session-rotation** — authenticated Connect sessions renew via a stateless sliding-expiry re-seal: `webauth.Handler.Reseal` re-parses the `{owner,expiry}` cookie and, once remaining lifetime drops below `resealThreshold` (`sessionTTL/2`) plus a threshold-only `resealSkew` (60s), re-seals it with a fresh **absolute** `nowUTC().Add(sessionTTL)` expiry (never a delta) and refreshes the `engram_csrf` cookie Max-Age (D-08) — driven by `newConnectResealInterceptor`, wired innermost in `mountConnect` and NOT gated to the write-only allowlist so it fires on reads and writes (SC1). Zero server-side state (honors DEC-u9v); no new `ENGRAM_` var. The hard-expiry check in `resolver.go` stays byte-for-byte strict/fail-closed — skew is threshold-only (SC4, guarded by `TestResolveHardExpiryHasNoSkewTolerance`); a 50-goroutine `-race` test proves forward-monotonic concurrent re-seals (SC3). New hand-authored ADR `engram-slr8` documents rotation-under-statelessness + the no-revocation limitation (kill-switch = rotating `ENGRAM_UI_COOKIE_KEY`, not the phantom `ENGRAM_SESSION_KEY`) (SC2). Verified 5/5 must-haves (#323) — Phase 18. **Mandatory `/gsd-secure-phase 18` pending.**
- ✓ **REQ-connect-write-authz-parity** — all six Connect write RPCs (`StoreMemory`/`StoreDiscovery`/`UpdateMemory`/`DeleteMemory`/`SetVisibility`/`ScheduleMemory`) are thin proto/args adapters delegating to the same `deps.*` business-logic methods the MCP tools call — never `store.*` directly — through an explicit `caller{Subj, Actor}` seam (no ctx-derived resolution), a single `protoconv` conversion layer (D-09, sub-second outward rounding, `shared` as `*bool` with the Visibility enum reserved to SetVisibility), and a single `connectError` mapper (with `context.Canceled`/`DeadlineExceeded` arms). Proven by a per-RPC MCP↔Connect `TestWriteParity` (identical rule-unshare / stale-summary / cross-owner rejections + a `go/parser` AST sub-test asserting each handler body invokes its named `deps.*` method), a per-RPC `TestCrossOwnerRewrap` guaranteeing a `store.ErrNotFound` re-wrap echoes the caller's original short_id/UUID and never the resolved UUID (no existence leak, DEC-xa6), read handlers rewired onto the typed single-path core (D-07), and the `NO_SIDE_EFFECTS` idempotency ban re-asserted by the Phase-15 CI gate; hardened with a fail-closed `requireQdrant` CI gate pinned to Qdrant v1.18.2. Verified 5/5 must-haves (#322) — Phase 17

**v0.10.x — Hardening & Write Lane (Phases 19–21; shipped 2026-07-15/16):**

- ✓ **REQ-console-write-ux** — operator console can create/edit/delete/re-share/schedule memories & discoveries over the Connect write lane, attaching the CSRF token client-side with a single opportunistic auth-race retry and a `sessionStorage` resume envelope surviving the `/auth/login` redirect (live browser E2E UAT deferred → #366) — Phase 19
- ✓ **REQ-discovery-proto-fidelity** (#307), **REQ-shortid-mint-cap** (#308), **REQ-embed-param-key-sharing** (#304), **REQ-embed-body-build-collapse** (#302), **REQ-discovery-shortid-schema** (#303), **REQ-summarize-cronjob** (#269) — correctness & polish tail — Phase 20
- ✓ **REQ-p11-review-residuals** (#335), **REQ-lint-planning-exclude** — CI/maintenance hygiene — Phase 21
- ⏸ **REQ-ci-renovate-spa-drift** (#301) — Renovate vendored-SPA self-heal: code/infra/security/review complete and merged; live self-heal observation is post-merge-only and deferred (→ #369) — Phase 21

**v0.11.x — Capture & Service Identity (Phase 22 complete 2026-07-17):**

- ✓ **REQ-cedar-pdp-foundation** — `internal/authz` cedar-go v1.8.0 PDP: 4-policy `go:embed` corpus (own-records, shared-read, tenant-isolate, scoped empty-owner forbid), `DecideBucket`/`DecideRecord` API, forward-compat Principal/Memory schema (`tenant`/`roles` reserved-optional, `parents` empty) — Phase 22
- ✓ **REQ-cedar-store-enforcement** — store bulk filter-builders + id-addressed gates consult the PDP (O(buckets) decisions on recall, per-record only on the id-addressed path); Cedar Deny → uniform `ErrNotFound` (DEC-xa6); behavior-preserving (pre-existing isolation/sharing suite unchanged); hand-authored ADR `engram-cdr1` refines DEC-cgb — Phase 22

**v0.11.x — Capture & Service Identity (Phase 23 complete 2026-07-17):**

- ✓ **REQ-service-auth-chain** — `auth.ChainVerifier` composes 3 config-selectable lanes (OIDC user → OIDC client-credentials via a second `auth.NewService` verifier → static token) over the existing `mcpauth.TokenVerifier` seam, wired at the single `cmd/engram/serve.go` `withAuth` call site; a structural JWT-vs-opaque discriminator routes each bearer, deny-by-default via `errors.Join(ErrInvalidToken,…)`; human-only/no-config path byte-for-byte preserved; `internal/store` + `identity.go` untouched — Phase 23
- ✓ **REQ-service-owner-failclosed** — `internal/auth` rejects an authenticated service principal whose owner claim resolves empty at the verifier boundary (never the anonymous bucket); proven by the phase's FIRST test (`TestFailClosedRejectsEmptyOwner`); human/no-issuer fail-open-to-anonymous unchanged — Phase 23 (closes the milestone #1 risk)
- ✓ **REQ-static-token-auth** — `StaticTokenVerifier` maps each token to its own `namespacedOwner("static_token",…)` (never one shared owner), `crypto/subtle.ConstantTimeCompare` over the full token, never logged; token→owner orientation matches `config.ParseServiceStaticTokens`; code-review caught + fixed a critical map-orientation inversion (CR-01) so the lane authenticates end-to-end — Phase 23
- ✓ **REQ-service-principal-isolation** — a namespaced service principal can't read another owner's private records nor collide with anonymous/human owners (proven against the Phase-22-wired store filters, zero new store code); `shared`-visibility stays global cross-tenant for v0.11.x as an explicit written+tested decision (ADR `engram-svct`, per-tenant scoping deferred to full ABAC) — Phase 23

**v0.11.x — Capture & Service Identity (Phase 25 complete 2026-07-19):**

- ✓ **REQ-idempotent-capture** — optional `idempotency_key` on the shared `store_memory`/`schedule_memory` write path enables **strict replay-safety** (reject-not-upsert, #340). When a key is supplied, the Qdrant point ID is a deterministic UUIDv5 over a length-prefixed, injective `(owner, scope, key)` tuple (owner baked into the hash → cross-owner point-ID poisoning structurally impossible, D-09); a payload-only `IdempotencyFingerprint` (`json:"-"`, wire-invisible) is compared on replay and a mismatch returns the distinct `store.ErrIdempotencyConflict` sentinel (→ Connect `AlreadyExists`) **before** the embedder call, never a silent overwrite (D-08 check-before-embed, D-12 honest-concurrency boundary). Keyless writes are byte-for-byte unchanged (D-01); no new dependency, collection, index, or config knob; Connect read lane untouched. Verified 5/5 SCs live under `-race` vs real Qdrant; deep code review + `--auto` fix loop resolved 1 Critical (client-minted `short_id` lost under concurrent keyed Upsert) + 3 Warning + 2 Info to a clean re-review; threat-secure (4/4 threats closed, `24-SECURITY.md`) — Phase 24
- ✓ **REQ-supersession-links** — a memory can supersede another via additive `supersedes`/`superseded_by` `*string` payload links (#342). A dedicated `supersede_memory` MCP verb (DEC-90w precedent; `supersedeArgs` embeds `storeArgs` to inherit `idempotency_key`) stores the new/correcting record and back-stamps the target — correction is **explicit and history-preserving**, never deleting or overwriting. The target back-stamp uses a single-key `SetPayload` merge (the `SetVisibility` shape, vector-preserving), **not** a full re-Upsert (D-01 — avoids the CR-01 lost-write hazard a whole-payload replace would reintroduce). Superseded records are soft-hidden at the recall gate (`superseded_by IS EMPTY` added at **both** the Search and List call sites) yet stay fetchable by id via `get_memory` (ungated, D-09). Routes through the ownership **write** gate (`getWritable`/`ActionWrite`, 404-indistinguishable for a not-owned target — no read-grant path, SC3); a single live head is enforced by rejecting an already-superseded target (`store.ErrAlreadySuperseded` → Connect `CodeFailedPrecondition`), so cycles/self-supersession are structurally impossible and no auto/similarity supersede path exists (SC4). Zero new dependency; verified 9/9 must-haves + 4/4 SCs live under `-race` vs real Qdrant (testcontainers), STRIDE threat models mitigated in code — Phase 25

**v0.11.x — Capture & Service Identity (Phase 26 complete 2026-07-26):**

- ✓ **REQ-memory-citations** — a curated `memory`-category record can optionally carry structured `citations` (the discovery `Citation` shape reused verbatim, file/commit/url/repo anchors), **never auto-populated** (#341). `payload()`'s single `category == "discovery"` conditional was split into two independent gates so `kind` stays discovery-exclusive while `citations` writes for ANY category when non-empty (D-01) — a citation-less record's payload is byte-identical to pre-phase. `Citations` is declared once on the shared `storeArgs` and inherited by `schedule_memory`/`supersede_memory` via Go field embedding (D-04, the Phase-24 `IdempotencyKey` precedent); `validateCitations(cites, minCount)` is shared (discovery 1, memory 0). Omitted from compact recall, present on `full=true`/`get_memory` on both lanes — which required an explicit `pb.Citations = nil` in Connect's `shapeProtoMemories`, closing an information-disclosure gap MCP's hand-written allow-list never had (D-07). Citations are inert provenance: not embedded, never ranked on, never recall-gated — Phase 26
- ✓ **REQ-category-filter** — `search_memory`/`list_memory` accept an optional plural `categories` filter with **OR** semantics (the opposite of the adjacent `tags` field's AND, stated explicitly in the jsonschema so agents don't assume symmetry), composed as a hard Qdrant **pre-filter** appended to the authz outer-`Must` and therefore evaluated server-side before any candidate is scored (#374). `store.SearchOptions{Tags, Categories, CreatedAfter, CreatedBefore}` replaced `Search`/`SearchReranked`'s positional tail across ~25 call sites (D-09 — two adjacent `[]string` params would have been a transposition that compiles clean and returns wrong results); `categoryMatchCondition` is shared by the list and search lanes so they cannot drift. Connect parity closed via additive `SearchMemoriesRequest.categories = 8` (D-10, a one-way field-number commitment approved by the user at a blocking checkpoint), deliberately carrying **no** `buf.validate` allowlist since `discovery`/`rule` are legitimate *filter* values though not legitimate *write* values (D-11) — Phase 26
- ✓ **REQ-chat-base-url** — `ENGRAM_OPENAI_CHAT_BASE_URL` lets the chat/summarize client target a different host than the embedder; empty means inherit `ENGRAM_OPENAI_BASE_URL`, resolved with `cmp.Or` at the single `summarize.New` call site (D-12/D-15, #350). The provider-shape endpoint join was hoisted into a new stdlib-only leaf package `internal/openaiurl`, shared by `internal/embed` and `internal/summarize` without a backwards dependency edge (D-13/D-14) — this fixed a **live** doubled-`/v1` bug, since Phase 13's shape-aware join had only ever been applied to the embedder lane. Byte-identical at the shipped `http://localhost:4000` default, pinned by test — Phase 26

**v0.12.x — Headless Reach & Diagnosability (shipped 2026-08-02, 21/21 requirements):**

- ✓ **Headless client lane** — one composed verifier chain built exactly once per process and injected into both the MCP wrapper and the Connect bearer half, so the two cannot drift; token expiry enforced with the 401 body kept byte-identical; the CSRF exemption re-keyed off a compiler-enforced per-request lane stamp (never a caller-controlled signal) with an unrecognized lane failing closed; `connect.headless` a default-off, fail-closed opt-in that refuses startup with zero configured auth lanes; `engram search|store|list` over the generated Connect stubs behind a per-file import-boundary gate — v0.12.x Phases 1–2 (#343)
- ✓ **Cross-spine memory recall** — `cross_spine` on `search_memory`/`list_memory`, mirrored to the Connect wire via six additive protobuf fields, reporting `searched_scopes`/`scopes_truncated` so a zero-hit cross-spine result is distinguishable from a scope-confined miss; explicit-field-only, never scope-inferred (the one deliberate divergence from `SearchDiscoveries`, commented as non-copyable at its declaration) — v0.12.x Phase 3 (#344)
- ✓ **Authz diagnosability & failure legibility** — one `argError` envelope (Fields + Hint + Detail + Class) carries every rejection across both wires with the class, not a hand-wrap, selecting the Connect code; a bounded debug line fires on both the allow and deny arm at `internal/store`'s two authz chokepoints with `Decision.diag` kept unexported so a future `cedar.Diagnostic` structurally cannot leak; embedder 502s name the provider's own diagnostic text and drain their bodies so the connection survives; docs-site `reference/errors.md` publishes all ten hint codes — v0.12.x Phase 4 (#394/#347/#360)
- ✓ **Per-lane provider config & reindex resume correctness** — `ENGRAM_OPENAI_CHAT_API_KEY` resolved by `cmp.Or` at the construction site and reachable via `memory.summarize.chatApiKeySecret`; `reindex --resume` re-embeds tags-only edits through one shared tag decoder and skip predicate, with a paired positive control proving genuinely-unchanged records are still skipped, and `--dry-run --resume` sizing the repair first — v0.12.x Phase 5 (#350/#345)
- ✓ **Rule capture** — investigation found the propose-a-rule permission was buried inside a prohibition; the fix gave it its own subsection with two observable triggers, an inline consent-gated protocol, and a decline record, plus rule-hygiene and one-time-backfill-sweep procedures, mirrored into the tool reference and CLAUDE.md. Validated behaviorally by a cold read: a fresh agent with zero phase context unprompted named the trigger, proposed via the corrected protocol, and stopped at consent — the user-blessed gate provably intact — v0.12.x Phase 6 (#351)
- ✓ **CLI cross-spine wiring** — `--cross-spine` on `engram search|list` through one shared `validateScopeCrossSpine` guard firing before any dialing, a `renderCoverageFooter` coverage line, and `EffectiveSearchScope` pinning the client guard against the server's own rule at compile time; `--scope` and `--cross-spine` name each other in live `--help` on both commands, meeting the correct-by-reading bar — v0.12.x Phase 7

  *Closed with rationale rather than built:* **#356 (UI TS codegen drift)** was already shipped — `task proto:gen` vendors `gen/ts/` into `ui/src/lib/gen/`, CI's `buf` job enforces the drift check on that path, and the tree is byte-identical. **#346 (base-URL join edge cases)** is a deliberate non-fix: query/fragment joins stay non-canonicalizing as operator-error scope, pinned by `TestJoin`.

**v0.13.x — Curation & Self-Evidence (Phases 1–5 plus inserted 03.1; shipped 2026-08-12):**

- ✓ **REQ-flag-exclusivity-enforced** — cobra's `MarkFlagsMutuallyExclusive` rejects every documented mutually-exclusive combination before any network call, replacing hand-rolled guards — v0.13.x Phase 1 (#453)
- ✓ **REQ-exit-code-unified** — client verbs and all six operator commands resolve through one 2/4/5 vocabulary via `classifyOperatorErr`; `ListenAndServe`'s bind failure is the single commented exit-1 exception, and two gates (source-level and table-level) prove no classifiable path was missed — v0.13.x Phase 1 (#467)
- ✓ **REQ-exit-code-migration-safe** — pinned-current-behavior regression baseline authored before the change, plus a consumer audit and a `guides/upgrade.md` entry gated by `TestUpgradeGuideNamesEveryChangedCommand`, which derives the required command list from the before-table itself — v0.13.x Phase 1
- ✓ **REQ-cli-request-timeout** — every client RPC derives `context.WithTimeout` from one resolved timeout; `exitTimeout=6` distinguishes a client-side deadline from an unreachable server, proven against connect-go's real hung-server behavior rather than assumed — v0.13.x Phase 1 (#452)
- ✓ **REQ-client-config-unified** — `--server`, `--token-file`, `--output`, `--insecure`, `--timeout` all resolve through one `config.Load` + `config.ValidateClient` in `clientFromFlags`, replacing four hand-rolled resolvers — v0.13.x Phase 1
- ✓ **REQ-conditional-rules-stated** — five conditional rules declared once in the stdlib-only `internal/surfaces` leaf package, reaching server rejections, cobra help, and anchored prose regions through `task surfaces:gen` + a CI drift job — v0.13.x Phase 2
- ✓ **REQ-surface-conformance-gate** — each declared rule's canonical sentence is machine-proven present on every surface its fields resolve to, with applicability *derived* from the rule's own fields rather than declared, demonstrated fail-first against a real corrupted region; an unexported `declared` marker makes an off-registry rule a compile-time impossibility — v0.13.x Phase 2
- ✓ **REQ-mcp-tool-annotations** — all 15 MCP tools advertise `readOnlyHint`/`destructiveHint`/`idempotentHint`/`openWorldHint` from one table gated in both directions against the real registration and published to `docs-site/reference/tools.md` — v0.13.x Phase 2
- ✓ **REQ-help-output-pinned** — every command's `--help` plus the bare catalog JSON pinned behind goldens generated by walking the live cobra tree (so the pin cannot silently go partial), closing two determinism hazards neither the plan nor RESEARCH anticipated — v0.13.x Phase 2
- ✓ **REQ-spine-scan** — `engram spine-review scan` enumerates the whole spine through a Subject-less, fully-paginated `internal/store/spine.go`, on a shared recursive cobra walker replacing seven single-level traversals — v0.13.x Phase 3
- ✓ **REQ-citation-drift-verify** — a pure four-tier (valid/moved/broken/unverifiable) classifier over a resolved-path safety gate and exact-segment repo identity, with `exitFindings=7` behind a registered `--fail-on` conditional rule — v0.13.x Phase 3
- ✓ **REQ-near-duplicate-report** — ranked candidate pairs over already-stored vectors via `NewQueryID`/`QueryBatch`: no clustering, no default threshold, no mutation on any path — v0.13.x Phase 3
- ✓ **REQ-purge-extract-gated** — `PurgeManifest` provenance is compiler-enforced via unexported fields (never a runtime check); `--apply` deletes only the intersection of a previewed, gate-passing set and a fresh re-derivation — v0.13.x Phase 3
- ✓ **REQ-archive-tier** — an orthogonal `archived_at` epoch-second state soft-hidden alongside `superseded_by`, driven by `archive`/`restore` with a deterministic concurrent-update race gate — v0.13.x Phase 3
- ✓ **REQ-destructive-preview-default** — `registerDestructive` makes `--apply` a runtime RunE choke point rather than a derived flag; `prune-expired` and `migrate-remap-owner` flipped to preview-first, with the first end-to-end coverage any operator command has had — v0.13.x Phase 3
- ✓ **REQ-operator-output-flag** — `--output json|text` with TTY auto-detection backfilled onto all six pre-existing operator commands, plus `operatorCommands()` as a gated structural predicate — v0.13.x Phase 3
- ✓ **REQ-merge-supersession** — `supersedes` accepts a set, linking every predecessor to one survivor with history preserved and no `delete_memory` in the merge path; preflight split into set-shape/addressability and rule-immutability/already-superseded stages, with whole-rejection indistinguishability proven — v0.13.x Phase 03.1
- ✓ **REQ-merge-atomicity** — the back-stamp-failure path is a classified reconciliation pass that removes the survivor, re-reads the full target set across every payload-op chunk, and clears dangling links; proved against a real pinned Qdrant v1.18.2 with a forced mid-sequence partial failure — v0.13.x Phase 03.1
- ✓ **REQ-merge-idempotency** — `idempotency_key` with a target-set-keyed `mergeFingerprint` checked *before* the already-superseded stage: a reordered/duplicated set replays, a different set conflicts, and a losing simultaneous keyed merge recovers via `resolveLostMergeRace` — v0.13.x Phase 03.1
- ✓ **REQ-semantic-curation-skill** — `curating-spine` judges staleness and near-duplicate identity using only shipped MCP tools, zero server-side code, expanded to 322 lines around an unchanged consent gate — v0.13.x Phase 4
- ✓ **REQ-consent-never-perform** — every mutation is proposed, never performed; the consent block is byte-identical to `store_rule`'s, and four cold-read runs all observed the consent stop — v0.13.x Phase 4
- ✓ **REQ-nyquist-reconciled** — all six phases at `status: validated`, clearing the v0.12.x inheritance; the one genuinely unproven requirement stays visibly unproven rather than flipped green — v0.13.x Phase 5
- ✓ **REQ-citation-fixture-355** — #355's drifted `tools.go` line-number anchors repaired by citing symbols instead — v0.13.x Phase 5 (#355)

  *Not satisfied:* **REQ-consent-adversarial-proof** — see the Known gap in Current State. Recorded unmet rather than deferred.

- ✓ **REQ-keylink-pattern-matchable** — `\\`-escaped key-link patterns eliminated repo-wide (39 across 20 plans) with `internal/keylinks` guarding the shape permanently — 2026-08-12.01 Phase 1 (#479)
- ✓ **REQ-keylink-past-gates-reassessed** — all 30 v0.13.x Phase 1–2 links re-resolved against HEAD: 26 pinned, 3 pinned-via-target, 1 recorded unpinned rather than quietly repaired — 2026-08-12.01 Phase 1
- ✓ **REQ-ci-qdrant-container-stability** — one shared `services:` Qdrant replaces four per-package testcontainers, health-gated with container-death diagnostics and a go/ast conformance gate on the `newTestStore` seam — 2026-08-12.01 Phase 1 (#497)
- ✓ **REQ-schema-version-stamped** — 100% of write paths stamp `schema_version`, proven by test rather than sample; a pre-field record reads as v0 by absence, so adoption needs no backfill — 2026-08-12.01 Phase 2
- ✓ **REQ-schema-version-never-gates-recall** — a runtime gRPC-interceptor gate proves the field reaches no Qdrant filter on any recall path, backed by a recursive filter-key walker and an AST completeness layer — 2026-08-12.01 Phase 2
- ✓ **REQ-schema-version-wire-visible** — wire-visible on `get_memory` and `full=true` recall, deliberately unlike the `json:"-"` audit stamps; the compact `recallView` stays as hidden as before — 2026-08-12.01 Phase 2
- ✓ **REQ-schema-version-forward-compatible** — proven in BOTH directions by raw payload injection: a record one version ahead is never rejected, hidden, or downgraded, and the key-absent legacy case holds too — 2026-08-12.01 Phase 2
- ✓ **REQ-migration-step-registry** — stdlib-only `internal/migrate` leaf package with a single `Validate` invariant over ordering and idempotency — 2026-08-12.01 Phase 3
- ✓ **REQ-migration-additive-only-gated** — `CheckAdditive`'s two-direction key-set diff makes a non-additive step a test failure, not a review catch — 2026-08-12.01 Phase 3
- ✓ **REQ-migration-step-reversibility** — sealed `Reversibility` with positional-required `NewStep`, so a step silent about reversibility is not a representable state — 2026-08-12.01 Phase 3
- ✓ **REQ-migrate-partial-failure-resume** — forced mid-sequence partial `SetPayload` failure (qdrant/qdrant#9371) survived and resumed against a real pinned Qdrant, via a gRPC fault injector — 2026-08-12.01 Phase 3
- ✓ **REQ-migrate-converges-without-lock** — `TestMigrateConvergesWithoutLock` proves convergence under a live concurrent writer; stamp-then-sweep ordering is the stated dependency — 2026-08-12.01 Phase 3
- ✓ **REQ-migrate-command** — `engram migrate` runs the registry through `Store.Migrate` via `registerDestructive`, preview by default with `--apply` as a runtime choke point — 2026-08-12.01 Phase 4
- ✓ **REQ-migrate-status-histogram** — `engram migrate status` reports a version-distribution histogram with per-version future buckets and an absent count, never a scalar — 2026-08-12.01 Phase 4
- ✓ **REQ-migrate-preview-apply-parity** — apply acts on the intersection of the preview and a fresh re-derivation, proven by identity set rather than count — 2026-08-12.01 Phase 4
- ✓ **REQ-backfill-shortids-first-step** — registered as the v0→v1 step; the standalone command is a thin delegating alias with apply-path parity proven by call-sequence equality — 2026-08-12.01 Phase 4
- ✓ **REQ-migrate-revert** — reverse-inverse walk with a whole-range zero-write preflight; refuses at the first irreversible step rather than leaving the collection between versions — 2026-08-12.01 Phase 4
- ✓ **REQ-migrate-never-automatic** — startup runs a read-only `MigrateStatus` probe that may warn; it never invokes the sweep and never gates startup — 2026-08-12.01 Phase 4
- ✓ **REQ-connect-record-state-parity** — eight additive `Memory` fields (23–30) wired through `memoryToProto` in one pass — 2026-08-12.01 Phase 5 (#482)
- ✓ **REQ-connect-parity-roundtrip-proof** — a reflection-based detector plus decode-back comparator makes a silently-dropped field structurally impossible; all five anti-vacuity gates observed going RED — 2026-08-12.01 Phase 5
- ✓ **REQ-operator-renderer-typed** — one serialization plus a view across 15 commands, walking `json.Marshal`'s own bytes, so field-set identity holds by construction — 2026-08-12.01 Phase 6 (#481)
- ✓ **REQ-console-record-state** — achromatic state badges, a dim-iff-past row treatment with an accessibility carve-out, an unconditional schema chip, and clickable successor/predecessor links — 2026-08-12.01 Phase 7
- ✓ **REQ-cli-record-state** — a single tested `memoryStateWords` derivation fills a STATE column on `search` and `list`; `engram get` shipped alongside — 2026-08-12.01 Phase 7
- ✓ **REQ-migration-state-visible** — a sixth Connect read RPC exposes the histogram, `engram migration-status` renders it, and a bounded advisory footer/banner appears on CLI and every console route — 2026-08-12.01 Phase 7
- ✓ **REQ-sweep-scope-rule-registered** — one declared `RuleSweepScopeOrAllScopesRequired` composed at all three sweep leaves, with a `TestNoHandRolledSweepScopeGuards` zero-occurrence gate observed failing against a constructed defect — 2026-08-12.01 Phase 8 (#480)
- ✓ **REQ-docs-record-state** — `reference/memory-record.md` covers all 28 wire-visible keys (proven by set difference), plus a new evergreen `guides/migrate.md` — 2026-08-12.01 Phase 8
- ✓ **REQ-claude-md-migrations-convention** — CLAUDE.md now describes the schema-version registry instead of denying migrations exist, with derived (not hardcoded) verification gates — 2026-08-12.01 Phase 8

**2026-08-23.01 — Distribution & Agent Bootstrap (Phases 1–6; shipped 2026-09-12 as v0.16.0):**

- ✓ **REQ-version-json** — `engram version --output json` emits `{"version":"…"}`, text lane unchanged and pinned byte-equal; `--output bogus` exits 2 — 2026-08-23.01 Phase 1
- ✓ **REQ-cask-install-gate** — cask hook order OS-guard → `xattr` quarantine strip → version assertion → completions, never delegated to Homebrew's rescuing `generate_completions_from_executable`; pinned by `TestReleaseConfigCaskInstallGate` — 2026-08-23.01 Phase 1
- ✓ **REQ-cask-credential-verified** — dedicated tap-publisher App, `repositories: homebrew-tap` on the mint, read-only `workflow_dispatch`-only probe; probe passed from main (run 32860661930) — 2026-08-23.01 Phase 1
- ✓ **REQ-cask-reship-recovery** — newest-tag `SKIP_HOMEBREW_UPLOAD` guard templated into `skip_upload` via the guarded optional-env idiom; accepted by construction under D-15, no rehearsal — 2026-08-23.01 Phase 1
- ✓ **REQ-setup-detects-runtimes** — the runtime's own binary is the signal; a leftover config directory never reads as installed — 2026-08-23.01 Phase 2
- ✓ **REQ-setup-previews-by-default** — no runtime CLI executes without `--apply`; the preview shows the exact argv, not a summary — 2026-08-23.01 Phase 2
- ✓ **REQ-setup-non-interactive** — `--runtime` selection, no confirmation, `--output json`; scriptable without a TTY — 2026-08-23.01 Phase 2
- ✓ **REQ-setup-partial-failure-legible** — per-runtime rows and exit 0 / 8 / 9 via an exhaustively tested `Classify` — 2026-08-23.01 Phase 2
- ✓ **REQ-setup-correct-by-reading** — `--help` names every runtime and auth mode; the advertised `ENGRAM_URL`/`ENGRAM_AUTH` defaults actually reach the command (CR-01) — 2026-08-23.01 Phase 2
- ✓ **REQ-setup-idempotent** — second `--apply` reports `already-correct` on every native runtime via before/after read-verb probes; opencode degrades only in the safe direction — 2026-08-23.01 Phase 3
- ✓ **REQ-register-claude-code** — `claude mcp add` via a tolerant `remove` → fatal `add` sequence; `~/.claude.json` never hand-written — 2026-08-23.01 Phase 3
- ✓ **REQ-register-codex** — `codex mcp add` / `mcp get`; `~/.codex/config.toml` never read, parsed, or written — 2026-08-23.01 Phase 3
- ✓ **REQ-register-opencode** — `opencode mcp add` with the `KEY=VALUE` header form its CLI accepts; its config file never touched — 2026-08-23.01 Phase 3
- ✓ **REQ-register-generic-mcp** — opt-in `generic` pseudo-runtime printing a portable minified `mcpServers` document, zero actions, no subprocess — 2026-08-23.01 Phase 3
- ✓ **REQ-register-auth-modes** — `oauth`, `oauth-client`, `bearer`, `none` on every path or an explicit unsupported row; `TestNoSecretInArgs` proves no secret literal on any argv — 2026-08-23.01 Phase 3
- ✓ **REQ-register-cli-surface-drift-legible** — an absent or drifted CLI fails naming the runtime, argv, and stderr; never silently writes nothing — 2026-08-23.01 Phase 3
- ✓ **REQ-skills-embedded-in-binary** — vendor → `//go:embed all:data` with `TestSkillsEmbedMatchesVendored` byte-equality against the plugin — 2026-08-23.01 Phase 4
- ✓ **REQ-skills-native-format** — native installs for Claude Code, Codex (`$HOME/.agents/skills`, human-confirmed), and opencode; every SKILL.md carries `metadata.engram-summary` — 2026-08-23.01 Phase 4
- ✓ **REQ-skills-agents-md-fallback** — delimited re-detectable AGENTS.md index spliced in place through symlinks; unreadable index preserved with zero writes (B01 / #559 closed by 04-05) — 2026-08-23.01 Phase 4
- ✓ **REQ-engram-setup-delegates** — validated client-ID input and four-mode CLI delegation — 2026-08-23.01 Phase 5
- ✓ **REQ-engram-setup-prose-fallback** — first-class Claude fallback with credential-safe registration commands — 2026-08-23.01 Phase 5
- ✓ **REQ-delegation-equivalence-derived** — Plan-derived commands with read-only lint and CI drift checks — 2026-08-23.01 Phase 5
- ✓ **REQ-docs-install-path** — `guides/install.md` with the exact working Homebrew invocation; Quickstart and CLI guides now say how to get the binary — 2026-08-23.01 Phase 6
- ✓ **REQ-docs-setup-documented** — `guides/agent-setup.md` covers every runtime, preview/`--apply`, and the manual generic path — 2026-08-23.01 Phase 6
- ✓ **REQ-homebrew-cask-published** — v0.16.0 published `Casks/engram.rb` to `seanb4t/homebrew-tap`; four actual installs (macOS/Linux × amd64/arm64) recorded in the D-10 post-release handoff — 2026-08-23.01 Phase 6
- ✓ **REQ-osrun-deadline-error** — `osRun` returns `RunResult{}, ctx.Err()` for a deadline-killed or cancelled child (success-first, then ctx, then `*exec.ExitError` unwrap); `runSeam` names the bound (`timed out after 20s: …`); proven with a re-exec'd test-binary child, never a runtime CLI (#560 closed) — 2026-09-13.01 Phase 1
- ✓ **REQ-manpages-generated** — hidden `engram man <dir>` over `cobra/doc.GenManTree`: pinned `.TH "… " "Jan 1970" "engram <version>" "Engram Manual"`, no autogen footer, cobra's own `IsAvailableCommand()` set (28 pages, `completion` included, hidden/deprecated absent), shared tree snapshot-restored; byte-identical across runs; zero go.mod change — 2026-09-13.01 Phase 1
- ✓ **REQ-manpages-cask-installed** — cask `post_install` generates straight into `#{HOMEBREW_PREFIX}/share/man/man1` as the 4th binary exercise after completions; `post_uninstall` globs `engram.1`/`engram-*.1` with `rm_f`; ordering, single occurrence, glob literal, and the forbidden `manpage:` stanza pinned by `TestReleaseConfigCaskInstallGate` (live `brew` observation is post-release) — 2026-09-13.01 Phase 1
- ✓ **REQ-header-name-parameter** — repeatable `--header NAME=ENVVAR` (and `ENGRAM_HEADERS`) adds an ADDITIONAL secret-valued header alongside any `--auth` mode, rendered per runtime in its own file: Claude Code `"NAME: ${VAR}"`, opencode `NAME={env:VAR}`, generic `"NAME": "${VAR}"` in the existing `headers` map; auth header first, extras in a total case-insensitive-then-bytewise order — 2026-09-13.01 Phase 2
- ✓ **REQ-header-value-env-ref-only** — a header VALUE is never read, resolved, or printed: `internal/setup` has no `Getenv` of a header var; `setupParseHeaders` rejects anything but a POSIX identifier on the right-hand side and never echoes it; `TestNoSecretInArgs` (32 subtests) proves a sentinel value appears nowhere — 2026-09-13.01 Phase 2
- ✓ **REQ-header-bearer-unchanged** — the four shipped `--auth` modes' argv, `Accepted --auth modes` help block, generated `/engram-setup` rows, and the three zero-header generic `Config` literals are byte-identical to `2026-08-23.01` (`TestSetupGeneratedInvocations`, `TestHelpGolden`, `TestGenericHeaders/zero-header-byte-identity`) — 2026-09-13.01 Phase 2
- ✓ **REQ-header-codex-declined** — any `--header` on codex is a `failed` row via the new `ErrHeaderUnsupported` sentinel naming the header(s), the gap (`codex mcp add` exposes only `--bearer-token-env-var`; openai/codex#5180 closed config-only) and the remedy; other runtimes proceed; engram never writes Codex TOML — 2026-09-13.01 Phase 2
- ✓ **REQ-header-documented** — `--help` "Additional headers" paragraph + fifth example, the regenerated `/engram-setup` `bearer+header` row and prose, and `guides/agent-setup.md`'s gateway-header section all use the vendor-neutral `x-gateway-api-key=GATEWAY_KEY` and state the Codex limitation — 2026-09-13.01 Phase 2
- ✓ **REQ-plugin-capability-detection** — one `plugin list --json` read probe per present runtime decides capability AND the 3-way state; a non-zero exit, timeout, or unparseable payload falls back to the native skills copy with the reason on a `plugin` facet, never a failed row — 2026-09-13.01 Phase 3
- ✓ **REQ-plugin-install-or-update** — `--apply` adds engram's own unpinned `seanb4t/engram` marketplace when absent, installs when absent (`--scope user`, `-y --json`), updates when outdated (Claude `plugin update`; Codex `remove` then `add`, having no update verb), and authors ZERO actions when current; preview shows the exact argv; `--apply` remains the only consent gate — 2026-09-13.01 Phase 3
- ✓ **REQ-plugin-three-way-state** — absent / installed-but-outdated / installed-and-current from a local stdlib SemVer-core comparator against the ldflags version: never downgrades a plugin newer than the binary (reported current with a note), and a dev/non-release binary never version-triggers an update — 2026-09-13.01 Phase 3
- ✓ **REQ-plugin-skips-skills-copy** — plugin delivery and the native copy are mutually exclusive per runtime per run: `skills.Install`'s single call site is reachable only when `!Delivered()`, so a plugin-delivered runtime gets no user-scope skill files and no `AGENTS.md` block; pre-existing native copies and index blocks are REPORTED (count, path, symlink vs copy) and never deleted — 2026-09-13.01 Phase 3
- ✓ **REQ-plugin-facet-reported** — plugin delivery is its own flat-scalar facet in text and `--output json` beside registration and skills, so a `wrote` registration next to a `failed` plugin install stays visible and yields `exitPartial` (8) — 2026-09-13.01 Phase 3
- ✓ **REQ-codex-plugin-manifest** — `skill/engram/.codex-plugin/plugin.json` ships minimal (`$schema`, `name`, `version`, `description`), release-please-synced like its Claude twin, with `TestPluginManifestIdentityMatches` keeping their identity fields equal — 2026-09-13.01 Phase 3
- ✓ **REQ-plugin-setupgen-regenerated** — `setupgen.Render(planFn, pluginFn)` appends the Claude Code plugin-delivery table from the runtime's REAL `PluginActions` (never a re-typed literal), the four shipped tables stay byte-identical, and `/engram-setup` was regenerated in the same commit with the `--check-setup` drift gate green — 2026-09-13.01 Phase 3
- ✓ **REQ-drift-observed-registration** — preview reads the existing registration through the runtime's own read verb (`codex mcp get --json` structured; `claude mcp get` fixed-label text) and normalizes it to the Plan's shape — 2026-09-13.01 Phase 4
- ✓ **REQ-drift-three-way** — exactly `already-correct` / `would-write` / `preserved`, never collapsed; ambiguity resolves to `would-write` — 2026-09-13.01 Phase 4
- ✓ **REQ-drift-preserved-outcome** — `preserved` is first-class in text and JSON with a reason naming what setup cannot reproduce; outranks `already-correct` in aggregation — 2026-09-13.01 Phase 4
- ✓ **REQ-drift-facet-naming** — a `would-write` row names the differing facet(s) (url, auth, header-name, header-value) — 2026-09-13.01 Phase 4
- ✓ **REQ-drift-redaction** — observed header values are redacted by construction before storage, rendering, JSON, or logs; proven against both runtimes' actually-observed literal-echo shapes — 2026-09-13.01 Phase 4
- ✓ **REQ-apply-preserve-gate** — `--apply` consults the same classification and performs zero registration writes on `preserved` (never Claude Code's `mcp remove`) while still delivering the plugin facet; observed byte-identical on v0.17.0 — 2026-09-13.01 Phase 5
- ✓ **REQ-apply-rewrite-consequence** — a reproducible Claude Code remove-then-add on an OAuth-shaped entry states the re-login consequence in preview and apply `notes` — 2026-09-13.01 Phase 5
- ✓ **REQ-docs-setup-v2** — `install.md`, `agent-setup.md`, `plugin.md` describe the shipped behavior, gated per guide, with the post-release live observation recorded in `05-RELEASE-0.17.0.md` before check-off — 2026-09-13.01 Phase 5 (observed 2026-09-18)
- ✓ **REQ-oversized-fixture-helper** — a shared real-Qdrant helper seeds over-limit scopes in two shapes (many small / few large), self-asserting logical bytes — 2026-09-18.01 Phase 1
- ✓ **REQ-test-client-parity** — every test Qdrant client is built through `store.NewQdrantClient` with an explicitly named receive limit — 2026-09-18.01 Phase 1
- ✓ **REQ-exhausted-sentinel** — one typed sentinel, matching code AND receive-limit message shape, so a server-side `ResourceExhausted` is never relabeled — 2026-09-18.01 Phase 2
- ✓ **REQ-exhausted-connect** — Connect returns `resource_exhausted` with the named hint envelope, no raw upstream text or byte ceiling — 2026-09-18.01 Phase 2
- ✓ **REQ-exhausted-mcp** — MCP tools return the same envelope through a single receiving middleware — 2026-09-18.01 Phase 2
- ✓ **REQ-exhausted-cli-docs** — CLI exit `10` documented; the hint code published in `reference/errors.md` — 2026-09-18.01 Phase 2
- ✓ **REQ-byte-budget-pages** — pages end on an accumulated-byte budget as well as a record count — 2026-09-18.01 Phase 3
- ✓ **REQ-content-cap-decided** — decision A — content and tag write caps, enforced on every write path, legacy records stay readable — 2026-09-18.01 Phase 3
- ✓ **REQ-list-bounded** — `Store.List` stays bounded in offset, deep-offset and cursor modes on every surface (#585) — 2026-09-18.01 Phase 4
- ✓ **REQ-list-scheduled-bounded** — `list_scheduled` stays under the receive limit at a large explicit limit — 2026-09-18.01 Phase 4
- ✓ **REQ-search-k-bounded** — `search_memory`/`search_discovery` bound `k` at a documented maximum with bounded full-payload results — 2026-09-18.01 Phase 4
- ✓ **REQ-list-contract-unchanged** — `total`, `next_cursor`, ordering and recall gating unchanged; a budget-cut page is never the last page — 2026-09-18.01 Phase 4
- ✓ **REQ-list-limit-contract-decided** — decision B — one maximum of 1000, `limit: 0` resolves to it, over-maximum rejected with `out_of_range` — 2026-09-18.01 Phase 4
- ✓ **REQ-ci-store-green** — `internal/store` CI stays green with the oversized fixtures; #497 closed — 2026-09-18.01 Phase 5
- ✓ **REQ-bounded-read-mechanism** — every full-payload read goes through the shared mechanism, inventoried with written exemptions — 2026-09-18.01 Phase 5
- ✓ **REQ-sweeps-bounded** — `migrate`, `migrate revert`, `summarize-missing`, `spine-review` and `reindex` complete over over-limit scopes — 2026-09-18.01 Phase 5
- ✓ **REQ-recv-limit-backstop** — 64 MiB production `MaxCallRecvMsgSize` in exactly one place, never relied on by a regression test — 2026-09-18.01 Phase 5
- ✓ **REQ-cross-spine-partial** — hits survive a failed `ListScopes` with a wire-visible `scopes_unknown` on MCP, Connect and the CLI (#456) — 2026-09-18.01 Phase 6
- ✓ **REQ-provider-drain-bounded** — embed and summarize drains bounded by bytes and time under `WithTimeout(0)` (#457) — 2026-09-18.01 Phase 7
- ✓ **REQ-provider-error-body-closed** — bounded, truncated provider error body pinned by tests; #347 closed — 2026-09-18.01 Phase 7
- ✓ **EVAL-01** — the embedding differ gate compares by cosine distance (> 1e-3 epsilon, NaN/Inf/zero-norm/length-mismatch hard errors), not `reflect.DeepEqual` on `[]float32` (#353) — 2026-09-22.01 Phase 1
- ✓ **EVAL-02** — the retrieval-eval skip guard reads a resolved koanf config (package-local gate; `ENGRAM_RETRIEVAL_EVAL` deliberately not registered in `internal/config`), not raw `os.Getenv` (#354) — 2026-09-22.01 Phase 1
- ✓ **RANK-01** — the retrieval eval carries an independently authored (blind) 24-query paraphrase case over a 96-record, six-domain corpus beside #261, reporting recall@k/MRR per named ranker (vector-only, lexical, tuned grid, disabled Jev stub) (#605) — 2026-09-22.01 Phase 1
- ✓ **RANK-02** — the default ranking does not regress paraphrase vs vector-only (lexical MRR 0.817 vs 0.579) and keeps #261 at rank 1; D-05 kept `lexical`, shipped through the `store.rankCandidates` seam; evidence posted to #605 — 2026-09-22.01 Phase 1
- ✓ **DEC-01** — typed decisions are enabled by the `ENGRAM_DECISIONS_PROVIDER` enum (`""` default off | `jev`); unset constructs no decider and makes no call, so behavior and outbound traffic stay byte-identical — 2026-09-22.01 Phase 2
- ✓ **DEC-02** — callers use the provider-neutral `internal/decide.Decider` (System One: one `State` + batched `Choice`/`Score`/`Noul` questions → typed answers with verbatim probabilities/confidence), plus `DecideMany` over many states in input order; a second backend slots in beside `internal/decide/jev` — 2026-09-22.01 Phase 2
- ✓ **DEC-03** — the Jev backend calls `{base}/alpha/decisions` with its own `ENGRAM_DECISIONS_*` base URL/key/model (pinned `typesafe/jev-1.13`)/timeout; only the key falls back (to `ENGRAM_OPENAI_API_KEY`), the base URL never does (D-03); passed live against OpenRouter direct and the LiteLLM pass-through (`02-LIVE-CHECK.md`) — 2026-09-22.01 Phase 2
- ✓ **DEC-04** — decision calls are bounded (timeout, response bytes, drain), retry once (jittered) on 429/5xx only, and classify both the OpenRouter numeric and LiteLLM string error dialects by status into 16 named, `errors.Is`-able errors; the never-fail-the-surrounding-read contract is documented on the interface (first exercised by Phases 3–4) — 2026-09-22.01 Phase 2
- ✓ **DEC-05** — `github.com/OpenRouterTeam/go-sdk` v0.8.19 evaluated before any client code (`02-SDK-EVALUATION.md`, engram record `0bwxc0asap`): verdict ADOPT-AND-WRAP-CANDIDATE, resolved reject-hand-write; zero new Go dependencies — 2026-09-22.01 Phase 2
- ✓ **DEC-06** — every call emits one `decide` OTLP span (latency = span duration; provider, model, model snapshot, question count, input/output tokens, `cost_usd`, status), carrying no record content or secrets — 2026-09-22.01 Phase 2
- ✓ **CUR-01** — with `ENGRAM_DECISIONS_PROVIDER` set, `spine-review consolidate` attaches an advisory nested `verdict` object per candidate pair by default (`relation` over `duplicate`/`contradicts`/`updates`/`related`/`unrelated` with the full `probabilities` map, `same_subject`, `needs_review`, `model`) in `--output json` and the text view; `--no-verdicts` suppresses it, and with no provider the output is byte-identical — 2026-09-22.01 Phase 3
- ✓ **CUR-02** — `needs_review` is `p < threshold` (`--verdict-threshold` / `ENGRAM_DECISIONS_VERDICT_THRESHOLD`, default 0.9); state per record is summary + content head sharing one `ENGRAM_DECISIONS_VERDICT_STATE_CHARS` budget (1500); a per-pair failure renders its named error class and the sweep still exits 0; no per-run cap. Consolidate never mutates: its store surface is read-only (`NearDuplicates` + `RecordStates`), pinned by reflection — 2026-09-22.01 Phase 3
- ✓ **CUR-03** — gated `task eval:curation` (`internal/curationeval`) runs the shipped `internal/verdict` question set over a committed 70-pair blind-agreed synthetic corpus (no verbatim spine content; optional private local pair file, aggregates only), reporting accuracy by confidence bucket, multi-class Brier and a confusion matrix; live D-03 gate PASS 40/40 at p ≥ 0.9, Brier 0.138 (`03-EVAL-RESULTS.md`) — 2026-09-22.01 Phase 3
- ✓ **CUR-04** — `consolidate` with neither `--scope` nor `--all-scopes` returns the shared sweep-scope rule error instead of a silent zero-candidate report (#508) — 2026-09-22.01 Phase 3
- ✓ **RANK-03** — opt-in `ENGRAM_SEARCH_RANKER=jev` (default `lexical`; `jev` requires `ENGRAM_DECISIONS_PROVIDER`, else `Config.Validate` fails) stable-sorts the lexical-ranked `CandidateK` pool by Jev P(relevant) in one Decisions request via a server-held `store.RankHook`, after the pinned `rankCandidates` step, then truncates to k; a dedicated no-retry client (`jev.WithNoRetry`) bounded by `ENGRAM_SEARCH_RERANK_TIMEOUT` (2s) falls back to exactly the lexical order on any failure and the search still succeeds; `search_discovery` gets the same opt-in path (`SearchDiscoveryReranked`) with its default unchanged; Helm `memory.search.*` (D-10). Live eval: Jev paraphrase recall@8 1.000 / MRR 0.883 vs lexical 0.950 / 0.817 vs vector-only 1.000 / 0.579, #261 at rank 1, 0/26 fallbacks at 2s (`04-EVAL-JEV.md`); shipped opt-in regardless (D-02) — 2026-09-22.01 Phase 4
- ✓ **RANK-04** — with the Jev ranker on, every hit carries an omitempty `relevance` probability (0–1) beside the cosine `score` on MCP `search_memory`/`search_discovery`, Connect (`optional double relevance = 31`) and `engram search` (a data-derived RELEVANCE column); absent when the ranker did not run or fell back. Per-hit only (D-06): no-answer queries score 0.01–0.03 — 2026-09-22.01 Phase 4
- ✓ **RANK-05** — `internal/relevance` builds the decision state from summary + content head at 600 chars/candidate, shrinking uniformly (floor 100) so query + up to 100 candidates stay under a 28k-token estimate, inside Jev's 32k context at the recall maximum — 2026-09-22.01 Phase 4
- ✓ **OPS-01** — the exit-code baseline isolates itself from ambient env: `neutralizeEnvDerivedFlagDefaults` blanks env-derived flag defaults and values (string, and via `pflag.SliceValue.Replace` for `setup --runtime`/`--header`) with a cleanup restore, so `TestExitCodeBaseline` passes with `ENGRAM_REINDEX_TARGET`/`ENGRAM_MIGRATE_OWNER`/`ENGRAM_RUNTIME`/`ENGRAM_HEADERS` set; test-only (#476) — 2026-09-22.01 Phase 5
- ✓ **OPS-02** — `viewFields`' bare nested-object branch is pinned by a direct test; an empty nested object renders zero rows, and a blank array element renders its compact JSON literal so it keeps its row (#504) — 2026-09-22.01 Phase 5
- ✓ **OPS-03** — `ParsePlanKeyLinks` skips fieldless key_links items, while the satisfiability scanner still reads the raw items so malformed entries stay reported (#502) — 2026-09-22.01 Phase 5
- ✓ **OPS-04** — `TestMigrateBelowCursorInsertConverges` covers a record inserted mid-sweep below the migrate cursor, asserting convergence on a later pass with no production change (#501) — 2026-09-22.01 Phase 5
- ✓ **OPS-05** — `guides/cli.md` §Operator commands lists `migrate` (with `status`/`revert`) and `setup`, gated by a docs test derived from the live `operatorCommands()` tree (#503) — 2026-09-22.01 Phase 5

### Active

Milestone 2026-09-22.01 (Typed Decisions & Recall Ranking) — scoped requirements live in
`.planning/REQUIREMENTS.md`.

### Deferred (carry-forward for next milestone)

- [ ] **REQ-ci-renovate-spa-drift live observation** — confirm the self-heal on the first real Renovate `ui/` bump PR, then `/gsd-verify-work 21` (GitHub #369). Blocked in practice by #393: the `ui/` `postUpgradeTask` build OOMKills the shared Renovate pod, so no bump PR completes a rebase.
- [ ] **Full-stack console e2e harness** — compose + mock OIDC + Playwright, to un-defer Phase 19's live browser↔server↔OIDC UAT (GitHub #366)
- [ ] **Runtime reindex-boundary enforcement** — reject/quarantine reads whose embedder-identity hash mismatches live config (v0.10.x stamps the identity; enforcement is a later decision)
- [ ] **Renovate pod heap cap** — bound the `ui/` build's Node heap so an oversized build fails loud instead of OOMKilling the shared multi-tenant Renovate pod (GitHub #393)
- [ ] **Phase 13–15 review follow-ups** — #346 (deliberate non-fix; Phase 26's `TestJoin` pins the behavior — consider closing with that rationale), #353/#354 (eval-differ defects — fixed by 2026-09-22.01 Phase 1 as EVAL-01/EVAL-02; issues close with the milestone PR), #357, #358
- [ ] **`REQ-consent-adversarial-proof`** (v0.13.x) — unmet, not merely deferred. The 3-run cap produced only *correct* verdicts, so the confidently-wrong moment was never reached; closing it needs a fixture that reliably misleads on identity, not more runs of the same one (`WINDOWS.md` id 3).
- [ ] **Phase 03 TDD commit-granularity windows** (v0.13.x) — `WINDOWS.md` ids 1 and 2: RED was genuinely observed but RED+GREEN landed in combined commits. Process debt, not correctness debt.
- [ ] **`internal/surfaces/toolclass.go:141-142` stale rationale** (v0.13.x) — the comment says `supersede_memory` "explicitly supports none" for `idempotency_key`, contradicting the shipped Phase 03.1 behavior at `internal/server/tools.go:594`. The emitted annotation value is correct; only the justification is wrong. One-line fix.
- [ ] **Full-stack E2E for `engram migrate`** (2026-08-12.01) — `internal/e2e/` has zero coverage of the status → apply → status reconvergence path against a live Qdrant. Backlog phase 999.2.
- [ ] **Narrow CLAUDE.md's "every surface" record-state claim** (2026-08-12.01) — the MCP lane exposes `SupersededBy`/`ArchivedAt` as raw fields but derives no state words, so the sentence overstates. Backlog phase 999.3.
- [ ] **Unify `schema_version` proto typing** (2026-08-12.01) — typed three ways in one file: `schema_version` uint32 (`:52`), `version` int32 (`:186`), `current_version` int32 (`:204`). Backlog phase 999.4.
- [ ] **`ui/` toolchain gaps** (2026-08-12.01) — `npm run check` crashes on a pinned `svelte-check@4.7.3` / `typescript@7.0.2` incompatibility, and `ui/package.json` has no `lint` script, so plan verification lines naming it cannot run as written.
- [ ] **`REQ-register-cursor`** (2026-08-23.01 v2) — Cursor is the one target needing a config-file writer (`~/.cursor/mcp.json`, top-level `mcpServers`) and merge-never-replace is a real reachable defect: a real machine's file already held three unrelated servers. Its CLI surface was unverifiable on the researching machine.
- [ ] **`REQ-setup-drift-detection` / `REQ-setup-reconcile-hand-edits`** (2026-08-23.01 v2) — binary/plugin/server version-skew reporting, and reconciling an engram MCP entry a user hand-edited away from what `engram setup` writes. The update path today is idempotent re-install.
- [ ] **Setup-core maintenance observations** (2026-08-23.01) — duplicate failed-count calculation, auth/runtime validation ordered before the missing-URL check, a capture-display comment that says "verbatim" although output is quoted, and the never-captured "emits no warning" half of the Phase 4 native-format human check.
- [ ] **`oauth-client` read-back label** (2026-09-13.01) — teach `claudeCodeRuntime.Observe` the `OAuth: client_id configured, callback_port N` line so a setup-authored `oauth-client` registration re-reads `already-correct` instead of `preserved` (recorded in `05-RELEASE-0.17.0.md` (g); safe today by D-11).
- [ ] **`verify:post` hook lapse** (2026-09-13.01) — neither `secure-phase` nor `validate-phase` dispatched from `/gsd-verify-work` all milestone; SECURITY.md/VALIDATION.md were reconciled retroactively at close. Diagnose on the next milestone's first phase verification (engram gotchas `7bw5r7emsd`, `9dkaz4zaeg`).
- [ ] **Review-bot threads block bot automerge** (2026-09-13.01 close) — `protect-main` requires thread resolution and fovea/octopus comment on `renovate/*` PRs, so green automerge-eligible PRs sit BLOCKED until a human resolves them; exclude renovate branches in those reviewers' config. The self-hosted bot's `gomodTidy` still no-ops (engram `ty3xrfxqnq`, `z2eda4249b`).
- [ ] **`seanb4t/homebrew-tap` cask DSL deprecation** — `uninstall_postflight` → `uninstall_postflight_steps` (warned twice on the v0.17.0 upgrade).
- [ ] **`curating-spine` skill ignores the verdict object** (2026-09-22.01 Phase 3) — consolidate now emits an advisory nested `verdict` per pair, but the skill does not yet read it; teach it to read `relation`/`probabilities`/`needs_review` as a prior while keeping its explicit-consent contract unchanged. No GitHub issue filed yet.
- [ ] **Response-level "nothing relevant" signal for search** (2026-09-22.01 Phase 4, D-06) — the Jev ranker ships per-hit `relevance` only; a response-level `no_relevant_results` flag or threshold was declined for now, as was an eval bar gating the opt-in (D-02). The live no-answer values (0.01–0.03) are the evidence base if it is revisited. No GitHub issue filed yet.
- [ ] **Operator text view does not sanitize JSON object keys** (2026-09-22.01 Phase 5 security audit, informational) — keys print raw in `viewRow`/`flattenObject`/`humanizeKey`; no user-controlled key reaches them today (struct tags and proto field names only), but a future `map<>`/`Struct` field would print control characters unsanitized. Sanitize keys and add a hostile-key test. No GitHub issue filed yet.

> **Closed by v0.13.x:** the two-tier CLI error model gap (Phase 1 unified the taxonomy rather than
> documenting a boundary — what #467 actually asked for), the v0.12.x Nyquist `VALIDATION.md`
> reconciliation debt (all six phases now `status: validated`), and #355 (drifted `tools.go`
> anchors now cite symbols).

> The nine `from-beads` refactor items (#306/#309/#310/#312/#313/#315/#316/#318/#319) were **closed as
> stale on 2026-07-29** after two promotion cycles with zero delivery — reviewer polish, none
> correctness-affecting. Reopenable if the relevant code is touched.

> **REQ-connect-auth-posture (R1–R4)** was found **already shipped** (PR #248/#266) and reconciled to Phase 8 = Complete on 2026-07-08 — no longer active.

### Out of Scope

- **Auto-extraction of memories** — explicit, user-blessed capture is a core design invariant; automatic memory harvesting is deliberately excluded to keep recall zero-junk.
- **Prometheus `/metrics` scrape endpoint** — telemetry is OTLP-gRPC only (DEC-dwi).
- **Server-side rendering (SSR)** — the operator console is an adapter-static SPA and the docs site is static-only (DEC-0lu, DEC-ttb).
- **viper / config files / `MEM_*` env vars** — config is ENGRAM_-prefixed koanf only; `MEM_*` is a fatal startup guard (DEC-jgq, DEC-irq).
- **cocogitto, viper** — not used in this project.
- **Automatic migrations** — payload migrations exist as of `2026-08-12.01` (`internal/migrate` + `engram migrate`), but none ever runs on its own: not on startup, not on failure. Startup's read-only probe may warn; it never sweeps and never gates boot.
- **Separate Qdrant collections per memory kind** — discovery/rule/scheduled all live in the single Memory collection (DEC-2bv).
- **Parsing or writing a third-party agent runtime's config file** — every `engram setup` writer shells out to the runtime's own `mcp add`; building a TOML/JSONC parser or a marker-bounded editor for `~/.codex/config.toml` / opencode's config would be work for a problem that does not exist (2026-08-23.01). Cursor, the one target that would need it, is deferred, not exempted.
- **Auto-running `engram setup` from a `brew install` hook, or configuring a runtime the user did not select** — Homebrew swallows postflight failures as warnings, so a broken auto-setup would be invisible; and mutating an unselected runtime's config is the cardinal sin every comparable tool avoids. Detection reports; the user chooses (2026-08-23.01).
- **Signing / notarizing the macOS binary, and telemetry on detected runtimes** — the former costs a GoReleaser Pro licence plus Apple Developer membership rather than a code change and the tap is third-party, so quarantine stripping in the cask is the sanctioned shape; the latter contradicts the self-hosted, no-phone-home posture (2026-08-23.01).

## Context

- **Ecosystem:** Go 1.26 static binary (`CGO_ENABLED=0`, distroless), Qdrant gRPC vector store, OpenAI-compatible embeddings/chat gateway. UI/docs built with pnpm + Node (not in the server image).
- **Surfaces:** MCP tool server (primary, StreamableHTTP at `/mcp`), ConnectRPC `EngramService` v1 (5 read + 6 write RPCs), the `engram search|store|list` headless CLI over the generated Connect stubs, SvelteKit adapter-static operator console vendored via `go:embed`, Astro Starlight docs site on Cloudflare Workers.
- **Bounded reads (2026-09-18.01, complete 2026-09-22, ship PR pending):** every full-payload Qdrant read in `internal/store` goes through `scrollOrderedPage` (`orderedpage.go`) or a byte-budgeted `scrollAllPoints` view (`boundedread.go`/`spine.go`), sized from `DefaultRecordCaps()`; searches fetch payloads by id through `fetchPayloadBatch` (`searchfetch.go`), all with a batch-of-1 legacy fallback that fails loudly with `ErrResponseTooLarge`. A new read site must compose one of these primitives — `unbudgetedView` no longer exists. `store.MaxRecallLimit` (1000) is the one recall maximum. `storetest` is usable only from `package store_test` files (import cycle). Provider HTTP clients drain through `internal/httpdrain`.
- **Setup v2 (2026-09-13.01, shipped 2026-09-17 as v0.17.0):** `internal/setup/drift.go` owns the single `Observe` → `Compare` classification (`already-correct` / `would-write` + facets / `preserved`) that both preview and `--apply` consult; `apply.go` returns before the write loop on `preserved`/`already-correct` and re-observes after a real write; observed header values never leave a local (`Observation`/`ObservedHeader` carry no value field). `HeaderSpec` renders per runtime with no shared formatter; `ErrHeaderUnsupported` is Codex's decline. `plugin.go`'s `PluginRuntime` probes `plugin list --json` once per run and authors `marketplace add`/`install`/`update` (Codex: remove-then-add) against engram's own marketplace only; `skills.DetectPresence` is read-only. `cmd/engram/man.go` generates byte-stable pages the cask's `post_install` writes to `share/man/man1`. Verification records that stand in for live-CLI tests: `04-OBSERVATIONS.md` (literal-echo shapes) and `05-RELEASE-0.17.0.md` (v0.17.0 observed end-to-end under an isolated `HOME`/`CODEX_HOME`).
- **Distribution & agent bootstrap (2026-08-23.01, shipped 2026-09-12 as v0.16.0):** the binary ships as a Homebrew cask (`seanb4t/homebrew-tap`, `Casks/engram.rb`) published by GoReleaser's `homebrew_casks:` through a dedicated tap-publisher App — the token field MUST stay the bare `{{ .Env.HOMEBREW_TAP_TOKEN }}` form, since GoReleaser regex-matches it on the raw string and only a real tag exercises it. `internal/setup` is a stdlib-only leaf: a `Runtime` interface authoring `Plan`s of argv `Action`s executed through an injectable `Environment.Run` seam, with Claude Code / Codex / opencode as shell-out writers and `generic` as an opt-in zero-action portable-config emitter; secrets are env-var references, never argv. `internal/skills` embeds the five curation skills (`//go:embed all:data`, drift-gated byte-for-byte against `skill/engram/skills`) and installs them natively per runtime, with Codex additionally getting a delimited AGENTS.md index spliced in place (only `fs.ErrNotExist` is the create case). `internal/setupgen` renders `/engram-setup`'s mechanical prose from real Plans; `surfacesgen --check-setup` and CI's regenerate-and-diff keep it equal. `cmd/engram/releaseconfig_test.go` pins the cask hook ordering, the `SKIP_HOMEBREW_UPLOAD` guard, and the credential shape as own-config text assertions.
- **Typed decisions (2026-09-22.01 Phase 2):** `internal/decide` is the provider-neutral contract (`Decider`, `State`, `Question` via `Noul`/`Choice`/`Score` constructors, `Answer`, `Usage`, `Status(err)`), with client-side structural validation before any network call and a bounded `DecideMany` worker pool (`ENGRAM_DECISIONS_CONCURRENCY`, default 4). `internal/decide/jev` is a hand-written `net/http` client (`jev.go`, `wire.go`, `classify.go`); `decodeResponse` requires an answer for every requested question and rejects one whose type mismatches the request (WR-01). `internal/server/decider.go` builds the decider into `deps` only when the provider is set — nothing calls it yet; Phase 3 (curation verdicts) and Phase 4 (reranker) are the first callers and own the never-fail-the-read contract. Helm `memory.decisions.*` is off by default, with the key from `memory.decisions.apiKeySecret` (secretKeyRef). `task eval:decisions` (`ENGRAM_DECISIONS_LIVE=1`) is the live check.
- **Curation verdicts (2026-09-22.01 Phase 3):** `internal/verdict` owns the one question set (five-option relation Choice + `same_subject` Noul per pair, one Decisions request each), the summary-plus-content-head state truncation and the `decide.Result → Verdict` mapping; both `spine-review consolidate` and the gated `internal/curationeval` harness call it, so the eval measures the shipped contract. Consolidate's `runVerdictPass` makes one budgeted `Store.RecordStates` fetch and one `DecideMany` call per sweep, and its `spineConsolidateStore` interface is read-only by construction. The text lane renders the verdict from the marshaled JSON through `registerRowFieldRenderer`, and a generic `flattenNested` sanitizes every nested row field. Eval result: `updates` is the main confusion sink (gold `contradicts`/`related` predicted as `updates`), which drives the high `needs_review` rate (30/70).
- **Jev reranker (2026-09-22.01 Phase 4):** `internal/store` never imports `internal/decide`: ranking reaches Jev through `store.RankHook` (`rerank.go`), a server-built hook (`searchRankHook` in `internal/server/decider.go`) that is nil unless `ENGRAM_SEARCH_RANKER=jev`. `SearchReranked` runs the pinned `rankCandidates` step first, then `applyRankHook` stable-sorts by P(relevant) and `applyRelevance` stamps `Memory.Relevance` only when every hit got a value — any error, timeout or partial answer returns the lexical order untouched, so authz filtering always precedes the hook and a failed hook is invisible except in the missing field. `SearchDiscoveryReranked` reuses the same two helpers over discovery's own vector order (no lexical step). `internal/relevance` owns the Noul-per-candidate request and the D-08 budget (`DefaultCandidateChars` 600, `MinCandidateChars` 100, `DefaultTokenBudget` 28000, chars/4 estimate). The search path uses its own decider built with `jev.WithNoRetry()` and `ENGRAM_SEARCH_RERANK_TIMEOUT`; consolidate keeps the decisions timeout and single retry. `store.RankWithHook` is the shared composition the retrieval eval's opt-in `jev` row calls, so the eval measures the shipped path.
- **Identity:** OIDC bearer tokens on the MCP lane become the memory `actor`; the authz `owner` key is a configurable claim (default `email`). No issuer → single anonymous empty-owner bucket.
- **VCS/build:** git (branch + PR; never push to `main` directly); `task` runner; buf-generated `gen/` tree committed and CI-checked; release-please-driven releases (binary + image via goreleaser, OCI Helm chart).
- **Connect observe lane:** authenticated via the cookie/OIDC lane (sealed session → verified `sub`); mounted only when the UI is enabled, headless by default (R1–R4 shipped in PR #248/#266, reconciled 2026-07-08). The MCP lane's no-issuer anonymous empty-owner bucket is unaffected.
- **Authorization (v0.11.x, shipped 2026-07-26):** `internal/authz` is a cedar-go v1.8.0 PDP with a 4-policy `go:embed` corpus; `internal/store`'s bulk filter-builders call `DecideBucket` (O(buckets), never per-record) and the id-addressed gates call `DecideRecord` (Deny → uniform `ErrNotFound`). Callers authenticate through `auth.ChainVerifier` (OIDC user → OIDC client-credentials → static token) at the single `withAuth` site; a service principal is namespaced via `namespacedOwner("static_token", …)` and can never resolve to the anonymous empty-owner bucket.
- **Capture contract (v0.11.x, shipped 2026-07-26):** three additive payload keys coexist on the single Memory collection — `idempotency_fingerprint` (wire-invisible, `json:"-"`), `superseded_by`, and `citations`. Any whole-payload `Upsert` must round-trip all three or take the `store.TargetLocker`; targeted `SetPayload` is the merge-safe alternative. `contentFingerprint` hashes an **explicit** field list rather than using reflection, so any newly added client-authored `storeArgs` field must be added to it in the same change or keyed replays silently discard it.
- **Headless reach & diagnosability (v0.12.x, shipped 2026-08-02):** `buildAuthChain` is the sole verifier-construction site, injected into both the MCP wrapper and the Connect bearer half, so a bearer token resolves to the identical actor on either lane; `connect.headless` mounts Connect without the UI (default-off, refuses startup with zero auth lanes). The CSRF exemption reads a server-set per-request lane stamp — never a caller-controlled signal — and an unrecognized lane fails closed. Recall spans scopes via explicit `cross_spine` (never inferred) on `search_memory`/`list_memory`, the Connect wire, and the CLI, reporting `searched_scopes`/`scopes_truncated`. Every rejection carries one `argError` envelope (`field=<name> hint=<code>`) whose CLASS selects the Connect code; the ten hint codes are published at docs-site `reference/errors.md`. Zero new Go dependencies across the milestone.
- **Record state & schema evolution (2026-08-12.01, shipped 2026-08-22):** `internal/migrate` is a stdlib-only leaf package holding an ordered registry of additive-only steps, each required at registration to declare its own reversibility (sealed `Reversibility`, positional-required `NewStep`). `Store.Migrate` re-derives its backlog every pass and converges with no collection lock, because the write path stamps `schema_version` before the sweep runs; partial `SetPayload` application is assumed, survived, and resumed. `schema_version` is wire-visible and forward-compatible in both directions, and must NEVER appear in a recall or authz filter — the `superseded_by`/`archived_at` `IsEmpty` idiom has inverted cardinality here, so copying it would exclude every pre-migration record from recall. `engram migrate` (`status`/`revert`) goes through `registerDestructive`, previews by default, and refuses a range at its first irreversible step; `backfill-short-ids` is the registered v0→v1 step and now a delegating alias. The whole operator tier renders through one serialization plus a view, making `--output text` an explicitly unstable view over a json contract. Zero new Go dependencies.
- **Curation & self-evidence (v0.13.x, shipped 2026-08-12):** `internal/surfaces` is a stdlib-only leaf package holding the single declaration of every conditional rule *and* the MCP tool blast-radius table; a conformance gate derives applicability from each rule's own fields and proves its canonical sentence present on cobra usage, jsonschema, MCP descriptions, proto comments, docs-site, and skill markdown, while an unexported `declared` marker makes an off-registry rule impossible to construct from another package. One exit-code taxonomy (2/4/5 plus `exitTimeout=6`, `exitFindings=7`) governs every command, and `--help`/catalog goldens are generated by walking the live cobra tree so the pin cannot go partial. `engram spine-review` is the sixth Subject-less operator command — never a new authz path — with `registerDestructive` making `--apply` a runtime RunE choke point across the destructive tier. `supersede_memory` takes a target *set*, with `mergeFingerprint` (target-set-keyed) checked before the already-superseded stage. Semantic curation lives in the `curating-spine` skill, propose-never-perform, with zero server-side code. Zero new Go dependencies.
- **Recall quality (v0.9.x, shipped 2026-07-10):** a labeled retrieval eval harness (`task eval:retrieval`, `internal/retrievaleval`) with a permanent #261 regression fixture; an always-on `search_memory` similarity score; a stdlib-only lexical-overlap reranker shared via `store.SearchReranked` (MCP + Connect) — since 2026-09-22.01 Phase 1 its rank step is the single `store.rankCandidates` seam (body = the D-05 winner, `RerankHits`), the plug point for Phase 4's Jev reranker, and the eval measures named rankers over `SearchReranked`'s own `CandidateK` pool; async-on-write summary fill via a bounded worker pool (`internal/server/summaryqueue.go`) off the write path; and per-record usage signals (`access_count`/`last_accessed_at`, `usagequeue.go`) that never affect ranking. Two reusable Go kernels: CR-01 shutdown-safety (RWMutex+closed) and `*time.Time` for optional timestamps.

## Constraints

- **Tech stack**: Go + Qdrant + MCP go-sdk + koanf + connect-go + go-oidc — Established, ADR-locked; the memory contract and authz model depend on them.
- **Security**: Authorization is enforced in `internal/store` (Qdrant read filters + owner gates), never in handlers — Prevents handler-level authz drift; store is the single default-deny chokepoint (DEC-cgb, DEC-12c).
- **Security**: Unauthorized id-addressed ops return the same not-found as a missing id — Prevents cross-actor existence leaks (DEC-xa6).
- **Compatibility**: `short_id` is 10-char Crockford base32, accepted anywhere an id is — Stable public handle; legacy records backfilled via `engram backfill-short-ids` (DEC-zzq0, DEC-02ta).
- **Observability**: Telemetry is OTLP-gRPC only and never a hard startup dependency — No Prometheus scrape; missing collector yields no-op providers (DEC-dwi, DEC-uxh).
- **Config**: Single ENGRAM_ field registry via koanf; retired `MEM_*` vars are a fatal guard — No silent fallback or dual-read shim (DEC-jgq, DEC-irq).
- **Frontend**: Operator console is adapter-static + `go:embed`; docs site is static-only — SSR dropped end-to-end (DEC-0lu, DEC-ttb).
- **Testing**: UI/sanitizer tests run under vitest 4 browser mode (real Chromium) — jsdom/happy-dom retired so DOMPurify + bits-ui exercise a real DOM.

## Decisions

All 56 ADRs are **LOCKED** (precedence 0) and cannot be auto-overridden by any lower-precedence
source: **25 core** decisions (headline choices, below, grouped by delivering phase) plus **31
companion refinements** (folded 2026-07-08 — finer-grained decisions that each refine a core lock;
see the *Companion / Refinement Decisions* subsection). Source of record: `docs/adr/engram-*.md`.
Implementation plans behind each phase are cross-referenced in
`.planning/intel/merge-plans/context.md`.

<decisions>

### Phase 1 — Authorization & Isolation

#### DEC-cgb — Enforce per-actor authorization in the store layer, not in handlers — [LOCKED]

- **Source:** docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md
- **Decision:** Per-actor authorization is enforced inside `internal/store` via Qdrant read filters and owner-gate primitives, not in MCP handlers.
- **Scope:** internal/store, Qdrant read filters, authorization, MCP tool handlers, owner isolation

#### DEC-g37x — Use configurable OIDC claim as record owner (default: email) — [LOCKED]

- **Source:** docs/adr/engram-g37x-use-configurable-oidc-claim-as-record-owner-default-email.md
- **Decision:** The record authz owner key is a configurable OIDC claim (default `email`, via `ENGRAM_OWNER_CLAIM`) so ownership survives IdP `sub` rotation.
- **Scope:** owner authz key, ENGRAM_OWNER_CLAIM, OIDC claim resolution, migrate-remap-owner, session cookie sealing

#### DEC-kyz — Sharing grants read but never write (read/write gate asymmetry) — [LOCKED]

- **Source:** docs/adr/engram-kyz-sharing-grants-read-but-never-write-read-write-gate-asymmetr.md
- **Decision:** Access primitives are asymmetric — sharing grants read access only; owners retain exclusive write/delete/visibility control.
- **Scope:** getReadable, getWritable, ownedOrAbsent, DeleteAll, shared visibility, id-addressed store ops, authz gates

#### DEC-xa6 — Return 404 not-found for unauthorized id-addressed operations — [LOCKED]

- **Source:** docs/adr/engram-xa6-return-404-not-found-unauthorized-id-addressed-operations.md
- **Decision:** All owner/visibility mismatches return the same not-found error as a missing id, preventing cross-actor existence leaks.
- **Scope:** get_memory, update_memory, delete_memory, set_visibility, discovery overwrite, ownership authz, ErrNotFound

#### DEC-12c — Represent authz Subject as a sealed Go interface — [LOCKED]

- **Source:** docs/adr/engram-12c-represent-authz-subject-as-sealed-go-interface.md
- **Decision:** Authz caller identity is modeled as a sealed Go interface with default-deny exhaustive type switches in the store layer.
- **Scope:** internal/store, authz Subject, anonymous/authenticated variants, store enforcement gates, Qdrant owner payload

### Phase 2 — Recall Semantics

#### DEC-ambu — Recall returns summary by default with full-content opt-in — [LOCKED]

- **Source:** docs/adr/engram-ambu-recall-returns-summary-by-default-full-content-opt.md
- **Decision:** Search/list recall returns summary-shaped output by default with a `full=true` opt-in for full content; `get_memory` is unchanged.
- **Scope:** search_memory, list_memory, get_memory, Connect SearchMemories/ListMemories, memory contract, web UI

#### DEC-4xt7 — Tag-filtered recall: hard Qdrant filter, AND-default — [LOCKED]

- **Source:** docs/adr/engram-4xt7-tag-filtered-recall-hard-qdrant-filter-and-default.md
- **Decision:** The optional `tags` filter on search_memory/list_memory is a hard AND (contains-all) Qdrant pre-filter composed onto the authz envelope.
- **Scope:** search_memory, list_memory, Store.Search, Store.List, Qdrant tag filter, tags recall dimension

#### DEC-y1g — Gate recall via Qdrant filter; leave get_memory ungated — [LOCKED]

- **Source:** docs/adr/engram-y1g-gate-recall-via-qdrant-filter-leave-get-memory-ungated.md
- **Decision:** Temporal validity is enforced as Qdrant filter conditions on Search/List, while get_memory and by-id paths stay ungated for record management.
- **Scope:** Qdrant filter, Search, List, get_memory, temporal validity gate, store layer

#### DEC-1frj — Boundary id-set cursor with half-open date window for recall — [LOCKED]

- **Source:** docs/adr/engram-1frj-boundary-id-set-cursor-half-open-date-window-recall.md
- **Decision:** Adopt an opaque boundary id-set cursor over `created_at` with a half-open date window for deterministic O(limit)-per-page recall paging.
- **Scope:** list_memory, MCP recall paging, cursor pagination, date-window recall, Connect ListMemories API, Qdrant ordering

#### DEC-ef28 — Index owner/scope/created_at as Qdrant payload indexes — [LOCKED]

- **Source:** docs/adr/engram-ef28-index-owner-scope-created-at-as-qdrant-payload-indexes.md
- **Decision:** Create keyword and datetime Qdrant payload indexes on owner/scope/created_at, retiring scanCap and the approximate flag for exact server-side Count and filtering.
- **Scope:** Qdrant payload indexes, List/ListScheduled/ListScopes, ensureCollection, owner authz filtering, created_at date-range queries

### Phase 3 — Memory Kinds & Tools

#### DEC-2bv — Discovery is a 5th category in the single Memory collection — [LOCKED]

- **Source:** docs/adr/engram-2bv-discovery-is-5th-category-single-memory-collection.md
- **Decision:** Discovery is added as a 5th category on the existing Memory record in one Qdrant collection rather than a separate collection.
- **Scope:** discovery category, Memory record, Qdrant collection, internal/store/store.go, query-time isolation filter

#### DEC-90w — Add schedule_memory/list_scheduled tools; keep store_memory windowless — [LOCKED]

- **Source:** docs/adr/engram-90w-add-schedule-memory-list-scheduled-tools-keep-store-memory-w.md
- **Decision:** Add dedicated `schedule_memory` and `list_scheduled` MCP tools rather than adding temporal window params to `store_memory`.
- **Scope:** schedule_memory, list_scheduled, store_memory, MCP tool surface, temporal validity windows

#### DEC-iedk — Rules are always-shared with server-set immutable visibility; set_visibility rejects rules — [LOCKED]

- **Source:** docs/adr/engram-iedk-rules-are-always-shared-server-set-immutable-visibility-set.md
- **Decision:** Rule-category memories are server-set to `shared` and immutable; the `set_visibility` handler rejects any call targeting a rule.
- **Scope:** rule memory kind, set_visibility MCP handler, visibility, shared-read grant, GetReadable
- **Relates to:** DEC-kyz

#### DEC-zzq0 — Encode short_id as 10-char Crockford base32 — [LOCKED]

- **Source:** docs/adr/engram-zzq0-encode-short-id-as-10-char-crockford-base32.md
- **Decision:** Encode memory record `short_id` as a 10-char lowercase Crockford base32 token instead of ULID or Sqids.
- **Scope:** short_id, memory records, Crockford base32 encoding, Qdrant lookup

#### DEC-02ta — Resolve short_id at the handler layer, not inside store methods — [LOCKED]

- **Source:** docs/adr/engram-02ta-resolve-short-id-at-handler-layer-not-inside-store-methods.md
- **Decision:** Resolve `short_id` to UUID via a shared `Store.ResolvePointID` method called from each by-id handler rather than inside store methods.
- **Scope:** short_id resolution, Store.ResolvePointID, MCP by-id tools, Connect GetMemory RPC, ownership gates

### Phase 4 — Embedder

#### DEC-378 — Name embedder connection vars by protocol, not implementation — [LOCKED]

- **Source:** docs/adr/engram-378-name-embedder-connection-vars-by-protocol-not-implementation.md
- **Decision:** Rename embedder connection env vars from `MEM_LITELLM_*` to `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`, naming the wire protocol not the vendor.
- **Scope:** embedder, environment variables, ENGRAM_OPENAI_BASE_URL, ENGRAM_OPENAI_API_KEY, OpenAI-compatible /v1/embeddings API, embed.New
- **Note:** Also realizes the embedder half of REQ-config-prefix-koanf (Phase 5).

#### DEC-zyhq — Generic param-map passthrough over embedder profiles for asymmetric/cloud embedders — [LOCKED]

- **Source:** docs/adr/engram-zyhq-generic-param-map-passthrough-over-embedder-profiles-asymmet.md
- **Decision:** Expose query/document embedding params as raw provider-agnostic JSON maps (`ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS`) merged into the /v1/embeddings body, instead of per-provider profiles.
- **Scope:** embedder, embed.Client, ENGRAM_EMBED_QUERY_PARAMS, ENGRAM_EMBED_DOCUMENT_PARAMS, /v1/embeddings request body, cloud/gateway embedders

### Phase 5 — Config & Transport

#### DEC-jgq — Unify config under ENGRAM_ prefix via koanf internal/config — [LOCKED]

- **Source:** docs/adr/engram-jgq-unify-config-under-engram-prefix-via-koanf-internal-config.md
- **Decision:** Introduce `internal/config` (koanf v2) with a single field registry owning all `ENGRAM_` keys, retiring scattered EnvOr/getenv reads and the `MEM_` prefix.
- **Scope:** internal/config, koanf, ENGRAM_ env vars, cmd/engram, server.EnvOr, CLI flags

#### DEC-irq — Breaking config renames ship with a fatal legacy-env startup guard — [LOCKED]

- **Source:** docs/adr/engram-irq-breaking-config-renames-ship-fatal-legacy-env-startup-guard.md
- **Decision:** Retired `MEM_*` env vars trigger a fatal registry-derived startup guard (`config.CheckLegacy`) rather than a silent fallback or dual-read shim.
- **Scope:** config.CheckLegacy, field registry, MEM_* env vars, PersistentPreRunE, startup guard

#### DEC-bj6 — MCP transport at explicit configurable path; console at root when UI enabled — [LOCKED]

- **Source:** docs/adr/engram-bj6-mcp-transport-at-explicit-configurable-path-mem-mcp-path-con.md
- **Decision:** Mount the MCP StreamableHTTP transport at an explicit configurable path (default `/mcp`) instead of the root catch-all; the console takes root when the UI is enabled.
- **Scope:** MCP transport, MEM_MCP_PATH, HTTP routing, web console/UI, Helm chart memory.mcpPath, mountMCPRoutes seam

### Phase 6 — Telemetry & Observability

#### DEC-dwi — Export telemetry via OTLP only; omit a Prometheus scrape endpoint — [LOCKED]

- **Source:** docs/adr/engram-dwi-export-telemetry-via-otlp-only-omit-prometheus-scrape-endpoi.md
- **Decision:** Export metrics, traces, and logs exclusively over OTLP gRPC to a collector; add no Prometheus `/metrics` scrape endpoint.
- **Scope:** telemetry, OTLP gRPC exporter, OpenTelemetry Collector, Prometheus /metrics endpoint, Grafana LGTM backend, Helm chart

#### DEC-uxh — Telemetry is never a hard server startup dependency — [LOCKED]

- **Source:** docs/adr/engram-uxh-telemetry-is-never-hard-server-startup-dependency.md
- **Decision:** A telemetry setup failure or missing OTLP endpoint yields no-op providers and never aborts engram server startup.
- **Scope:** telemetry, OTLP exporter, server startup, Helm chart defaults, observability subsystem

### Phase 7 — Web UI, Docs Site & Distribution

#### DEC-8xe — Adopt ConnectRPC and protobuf/buf for the web UI API — [LOCKED]

- **Source:** docs/adr/engram-8xe-adopt-connectrpc-and-protobuf-buf-web-ui-api.md
- **Decision:** Adopt ConnectRPC with the protobuf/buf toolchain scoped to the web-UI API, reversing the prior "no protobuf" convention. MCP core stays as-is.
- **Scope:** ConnectRPC, protobuf, buf toolchain, web-UI API, connect-go, connect-es, MCP core

#### DEC-0lu — SvelteKit adapter-static SPA vendored via go:embed, SSR dropped — [LOCKED]

- **Source:** docs/adr/engram-0lu-sveltekit-adapter-static-spa-vendored-via-go-embed-ssr-dropp.md
- **Decision:** Build the engram frontend as a SvelteKit adapter-static SPA vendored via `go:embed`, dropping SSR entirely.
- **Scope:** frontend, SvelteKit, go:embed, SPA, BFF, engram binary, connect-es client

#### DEC-ttb — Deploy docs-site via Workers Static Assets without an SSR adapter — [LOCKED]

- **Source:** docs/adr/engram-ttb-deploy-docs-site-via-workers-static-assets-without-ssr-adapt.md
- **Decision:** Serve the Astro Starlight docs-site as static assets via a Cloudflare Workers assets binding, with no SSR adapter or worker script.
- **Scope:** docs-site, Cloudflare Workers Static Assets, Astro Starlight, wrangler.jsonc, @astrojs/cloudflare adapter

### Companion / Refinement Decisions (31 fine-grained ADRs — folded 2026-07-08)

All **LOCKED** (precedence 0). Each refines or implements a scope already governed at a coarser
grain by a core decision above — they add no new product scope. Full text lives in the ADR source
and `.planning/intel/merge-adrs/decisions.md`; the `refines →` note names the core lock elaborated.

**Phase 2 — Recall Semantics**

- **DEC-4y7p** — Explicit-first memory summary; missing ones filled by an offline `engram summarize-missing` sweep, never on the write path. *(refines DEC-ambu)* — `docs/adr/engram-4y7p-explicit-first-memory-summary-offline-operator-auto-fill.md`
- **DEC-ddiw** — Reject `update_memory` on content change when a client-authored summary is left unaddressed; auto-clear an auto summary instead. *(refines DEC-ambu)* — `docs/adr/engram-ddiw-reject-update-memory-content-change-unaddressed-client-summa.md`
- **DEC-ufz** — Soft-hide expired records at the recall gate; reclaim storage only via explicit `engram prune-expired`. *(refines DEC-y1g)* — `docs/adr/engram-ufz-soft-hide-expired-records-at-recall-opt-prune-expired-storag.md`
- **DEC-c0m** — Inject the `Store` clock via a `WithClock` functional option; keep public signatures stable. *(refines DEC-y1g)* — `docs/adr/engram-c0m-inject-store-clock-via-withclock-option-keep-public-signatur.md`

**Phase 3 — Memory Kinds & Tools**

- **DEC-0gy** — Dedicated `store_discovery`/`search_discovery` tools rather than overloading `store_memory`. *(refines DEC-2bv)* — `docs/adr/engram-0gy-dedicated-store-discovery-search-discovery-tools-not-overloa.md`
- **DEC-3l0** — Surface raw citation-pin/`created_at` aging signals for discovery trust; no server-computed freshness verdict. *(refines DEC-2bv)* — `docs/adr/engram-3l0-graceful-decay-over-binary-staleness-discovery-trust.md`
- **DEC-d386** — Session-start surfaces rules as a one-line-per-rule progressive-disclosure index via `list_rules`. *(refines DEC-iedk/DEC-ambu)* — `docs/adr/engram-d386-session-start-surfaces-rules-as-progressive-disclosure-index.md`
- **DEC-m4s8** — Reject malformed rule summaries (newline / >256 B / cleared); never silently normalize. *(refines DEC-iedk, DEC-ddiw)* — `docs/adr/engram-m4s8-reject-malformed-rule-summaries-newline-oversize-cleared-nev.md`

**Phase 5 — Config & Transport**

- **DEC-wtw** — Keep `config.Load` assembly-only; check well-formedness via a separate pure `Config.Validate()`. *(refines DEC-jgq)* — `docs/adr/engram-wtw-keep-config-load-assembly-only-validate-via-separate-config.md`
- **DEC-d24** — `Config.Validate` checks only the five data-plane fields; `listen_addr` is a serve-local guard. *(refines DEC-wtw)* — `docs/adr/engram-d24-validate-data-plane-fields-only-listen-addr-is-serve-local-g.md`

**Phase 6 — Telemetry & Observability**

- **DEC-6gb** — Instrument store/embed/auth with inline OTel spans, not a decorator layer. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-6gb-instrument-store-embed-auth-inline-spans-not-decorator-layer.md`
- **DEC-f7p** — Instrument three seams: HTTP, MCP method (`AddReceivingMiddleware`), downstream clients. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-f7p-instrument-at-three-seams-http-mcp-method-and-downstream-cli.md`
- **DEC-tdk** — Instrument MCP tools from one `AddReceivingMiddleware` seam, not per-handler. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-tdk-instrument-mcp-tools-via-addreceivingmiddleware-not-per-hand.md`
- **DEC-wot** — Spans carry `engram.owner` (opaque `sub`) only; exclude actor/email as PII. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-wot-spans-carry-engram-owner-only-exclude-actor-and-email-as-pii.md`
- **DEC-7qd** — Configure sampler + export interval via OTel-standard env vars; no `MEM_*` counterparts. *(refines DEC-dwi)* — `docs/adr/engram-7qd-reuse-otel-standard-env-vars-sampler-and-export-interval-add.md`
- **DEC-9tj** — Inject k8s resource attributes via the Helm chart Downward API, not a Go SDK detector. *(refines DEC-dwi)* — `docs/adr/engram-9tj-inject-k8s-resource-attributes-via-chart-downward-api-not-go.md`

**Phase 7 — Web UI, Docs Site & Distribution**

- **DEC-bgj** — Embed the BFF (OIDC login/callback, session, static serving) in the engram Go binary, not a Node runtime. *(refines DEC-0lu/DEC-8xe)* — `docs/adr/engram-bgj-embed-bff-engram-go-binary-not-node-runtime.md`
- **DEC-8q3** — Operator-console session cookie seals only `{sub, expiry}`; no OIDC tokens client-side (read-only v1 lane). *(refines DEC-g37x)* — `docs/adr/engram-8q3-session-cookie-seals-only-sub-expiry-no-oidc-tokens-stored-c.md`
- **DEC-u9v** — Stateless AES-GCM encrypted-cookie session, no server-side store (eventual write-phase custody). *(refines DEC-g37x)* — `docs/adr/engram-u9v-stateless-encrypted-cookie-session-no-server-side-store.md`
- **DEC-2xl** — Use `@tanstack/svelte-query` as the SPA's sole async data layer over connect-es. *(refines DEC-0lu)* — `docs/adr/engram-2xl-use-tanstack-svelte-query-as-spa-data-layer.md`
- **DEC-c4y** — Drive SPA shell state (scope/filters/query/selection) via URL query params. *(refines DEC-0lu)* — `docs/adr/engram-c4y-drive-spa-shell-state-via-url-parameters.md`
- **DEC-3nas** — Render user memory content via `marked` + DOMPurify allowlist as the sole `{@html}` entry point. *(refines DEC-0lu)* — `docs/adr/engram-3nas-render-user-memory-content-via-marked-dompurify-allowlist.md`
- **DEC-vxk** — SPA-fallback static handler serves `index.html` for extensionless `/ui/*` client routes. *(refines DEC-0lu)* — `docs/adr/engram-vxk-spa-fallback-static-handler-serve-index-html-client-routes.md`
- **DEC-4ag** — Drop the dashboard category-breakdown bar until a `listScopes` API extension provides real counts. *(refines DEC-0lu/DEC-8xe)* — `docs/adr/engram-4ag-gate-dashboard-category-breakdown-bar-listscopes-api-extensi.md`
- **DEC-lzz** — Adopt shadcn semantic tokens; retire the bespoke `eg-*`/`--cat-*` layer. *(refines DEC-0lu)* — `docs/adr/engram-lzz-adopt-shadcn-semantic-tokens-retire-bespoke-eg-cat-layer.md`
- **DEC-no3** — Ship the engram wordmark as inlined outlined SVG paths, not a webfont. *(refines DEC-0lu)* — `docs/adr/engram-no3-ship-engram-wordmark-as-outlined-svg-paths-not-webfont.md`
- **DEC-1h3k** — Two-tier vitest config: node tier + real-Chromium browser tier. *(refines the UI test-unification direction)* — `docs/adr/engram-1h3k-adopt-two-tier-vitest-config-node-real-chromium-browser.md`
- **DEC-om5b** — Run the node test tier on `environment:'node'`; drop happy-dom. *(refines DEC-1h3k)* — `docs/adr/engram-om5b-node-test-tier-environment-node-drop-happy-dom.md`
- **DEC-u5h** — Host the Astro Starlight docs site in-monorepo at `docs-site/` with tooling exemptions. *(refines DEC-ttb)* — `docs/adr/engram-u5h-host-docs-site-inside-engram-monorepo-at-docs-site.md`
- **DEC-1w7** — Deploy docs-site via a dedicated non-required GitHub Actions wrangler workflow. *(refines DEC-ttb)* — `docs/adr/engram-1w7-deploy-docs-site-via-repo-github-actions-wrangler-workflow.md`
- **DEC-50b** — engram plugin ships no bundled MCP server; `/engram-setup` is the sole registration path (plugin stays bundled). *(refines DEC-8xe distribution)* — `docs/adr/engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md`

</decisions>

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Authz enforced in store layer, not handlers (DEC-cgb, DEC-12c) | Single default-deny chokepoint; no handler-level authz drift | ✓ Good — shipped |
| Configurable-claim owner key, default email (DEC-g37x) | Ownership survives IdP `sub` rotation | ✓ Good — shipped |
| 404-uniform not-found for unauthorized id ops (DEC-xa6) | Prevents cross-actor existence leaks | ✓ Good — shipped |
| Summary-by-default recall with full opt-in (DEC-ambu) | Cuts recall token cost while preserving correctable full content | ✓ Good — shipped |
| Discovery/rule as extra categories in one collection (DEC-2bv, DEC-iedk) | Avoids collection sprawl; one authz/recall path | ✓ Good — shipped |
| 10-char Crockford base32 short_id (DEC-zzq0, DEC-02ta) | Stable, human-usable handle over ULID/Sqids | ✓ Good — shipped |
| ENGRAM_ koanf config + fatal legacy guard (DEC-jgq, DEC-irq) | One config surface; no silent fallback | ✓ Good — shipped |
| OTLP-only, non-blocking telemetry (DEC-dwi, DEC-uxh) | Observability without a hard startup dependency | ✓ Good — shipped |
| ConnectRPC + adapter-static SPA + static docs (DEC-8xe, DEC-0lu, DEC-ttb) | Read API + embeddable console without SSR complexity | ✓ Good — shipped |
| Cookie/OIDC Connect observe lane (R1–R4) | Mount-gated, cookie-only authz, obs parity, same-origin | ✓ Good — shipped (PR #248/#266) |
| Fold 31 companion ADRs + 24 plans into baseline (2026-07-08) | Complete the decision record; the original 50-doc bootstrap capped out | ✓ Good — merged, 0 conflicts |
| Eval-chosen ranking lever — stdlib lexical reranker over hybrid/cross-encoder (D-06, v0.9.x) | The #261 regression eval cleared the bar on the lightest lever; no new dep, no reindex | ✓ Good — recall@8=1.00; D-07/D-08 not needed |
| Always-on `search_memory` similarity score (D-04 supersession, v0.9.x) | Better DX than an opt-in flag; eval can assert score separation | ✓ Good — shipped |
| Async-on-write summaries via bounded worker pool off the write path (D-01/D-08, v0.9.x) | A gateway outage must never fail `store_memory`; drain-after-shutdown under CR-01 kernel | ✓ Good — shipped (#320); residuals #335 |
| Usage signals never affect ranking (D-08 invariant, v0.9.x) | Curation metadata, not a ranking input; usage-weighted recall is a separate future decision | ✓ Good — negative-space test enforced |
| PDP decides the predicate; the store enforces it as the Qdrant filter (ADR `engram-cdr1`, v0.11.x) | Bucket-level Cedar decisions — O(buckets), never O(records); cedar-go has no partial evaluation, so no residual compilation | ✓ Good — byte-for-byte behavior-preserving; isolation suite unchanged |
| Fail-closed at the verifier boundary, not the store (D-08/D-09/D-10, v0.11.x) | An authenticated service principal resolving to `owner==""` was the milestone's #1 risk; rejecting at the edge keeps the store's default-deny chokepoint unchanged | ✓ Good — proven as the phase's FIRST test |
| Global cross-tenant `shared` read for now (ADR `engram-svct`, v0.11.x) | Per-tenant scoping needs full ABAC; shipping the reserved attribute without the narrowing keeps the decision explicit and tested rather than implicit | ⚠️ Revisit — deferred to full ABAC |
| Strict replay-safety: reject, never upsert (D-08/D-12, v0.11.x) | A same-key/different-content write is a caller bug; silently overwriting would violate "explicit, zero-junk, correctable" | ✓ Good — `ErrIdempotencyConflict` before the embedder call |
| Supersession back-stamps via single-key `SetPayload`, not re-Upsert (D-01, v0.11.x) | A whole-payload replace reintroduces the CR-01 lost-write hazard on a vector-bearing record | ✓ Good — plus `TargetLocker` serializes Update vs Supersede |
| Options struct over positional params for store filters (D-09, v0.11.x) | Two adjacent `[]string` params (`tags`, `categories`) transpose silently — compiles clean, returns wrong results | ✓ Good — ~25 call sites migrated in one pass |
| Filter fields carry no `buf.validate` allowlist (D-11, v0.11.x) | `discovery`/`rule` are legitimate filter values though not legitimate write values; the write-domain allowlist must not be copied to the read domain | ✓ Good — unknown category matches nothing, never errors |
| One verifier chain built once, injected into both lanes (D-06, v0.12.x) | Two independently-built chains drift while each lane's own mocked tests keep passing; `withAuth` taking a built chain makes drift a compile error | ✓ Good — MCP↔Connect parity suite proves identical actor + identical expiry rejection |
| CSRF exemption keyed on a server-set lane stamp, never a caller signal (D-08/D-09, v0.12.x) | Keying on `X-CSRF-Token` absence lets a cookie caller opt itself out of CSRF — the milestone's #1 risk; an unstamped lane fails closed with no check attempted | ✓ Good — cookie caller cannot self-declare the bearer lane |
| Connect headless mount is opt-in and fail-closed (D-10/D-11, v0.12.x) | Mounting Connect headless grants a deployment a surface it does not have today; refusing startup with zero auth lanes mirrors `ownerClaimGuard` | ✓ Good — default-off; mounting and bearer-inclusion decided independently |
| `cross_spine` is explicit-field-only, never scope-inferred (D-03/D-14, v0.12.x) | A permissive default is how cross-scope leaks arrive; `SearchDiscoveries`' identical-looking inference is a deliberate, non-copyable divergence | ✓ Good — commented as non-copyable at its declaration |
| The failure CLASS selects the Connect code, not a hand-wrap (D-11a, v0.12.x) | Seven hand-wrapped `CodeInvalidArgument` sites made the code a property of the call site rather than the error; one envelope makes it a property of the failure | ✓ Good — but the `*argError` switch arm must stay first or all classes collapse (`667p88n2be`) |
| Agent-facing interfaces are correct-by-reading (D-00, v0.12.x) | A caller must learn the right invocation from help text and naming, never by running something and interpreting the failure; a validation error is a backstop, not a tutorial | ✓ Good — Sean's principle (`4aksmneehh`); raised to milestone-wide scope in backlog 999.2 |
| GSD never creates git tags in this repo (2026-08-02) | release-please owns the version tag namespace; a `vX.Y.x` milestone label is not semver and never had a Release, binary, image, or chart behind it | ✓ Good — `git.create_tag: false`; matches v0.9.x–v0.11.x precedent (`hjsg5wda5t`) |
| Unify the exit-code taxonomy rather than document a boundary (#467 before #453, v0.13.x) | Cobra's `MarkFlagsMutuallyExclusive` raises a plain `fmt.Errorf` that bypasses `cliError`/`ExitCode()`; adopting #453 first would have reintroduced, one command over, the exact undocumented split #467 exists to close | ✓ Good — the integration check found no bare `fmt.Errorf` on any new `spine-review` path |
| Rule applicability is *derived* from the rule's own fields, never declared (D-05, v0.13.x) | A declared applicability list is a second source of truth that drifts silently from the rule it describes; deriving it means a surface cannot fall out of scope without the fields changing | ✓ Good — gate demonstrated fail-first against a real corrupted region |
| Off-registry conditional rules are a compile error, not a runtime check (v0.13.x Phase 2) | An unexported `declared` field cannot be set from a composite literal in any other package, so a faithful-looking forged rule always carries `declared==false` | ✓ Good — proven with scratch repros outside the owning package, not by re-reading the commit's own tests |
| `spine-review` extends the Subject-less operator tier; it is never a new authz path (v0.13.x) | Composing the Subject-gated `Search`/`List` would silently scope an operator sweep to one actor — authorization stays in `internal/store` or the feature does not ship | ✓ Good — sixth instance of the existing tier |
| `consolidate` reports near-duplicates; it never merges them (v0.13.x Phase 3) | Every system surveyed showed threshold auto-merge silently destroys provenance, exceptions, and version distinctions | ✓ Good — no clustering, no default threshold, no mutation on any path |
| Record an adversarial non-result as NOT-OBTAINED rather than convert it to pass or fail (v0.13.x Phase 4) | The 3-run cap produced only *correct* verdicts, so the criterion's confidently-wrong case was never observed; scoring that as a pass would have made the artifact claim a proof it does not have | ⚠️ Revisit — honest, but `REQ-consent-adversarial-proof` stays unmet (`WINDOWS.md` id 3) |
| Every v1 runtime writer is a shell-out to the runtime's own `mcp add`; engram parses no third-party config format (2026-08-23.01 scoping) | Live post-synthesis verification showed `codex mcp add` and `opencode mcp add` both exist, so the proposed marker-bounded TOML/JSONC editing was work for a problem that does not exist and the one thing that would have pressured zero-new-deps; a CLI-contract dependency that breaks loudly beats a config-format dependency that drifts silently | ✓ Good — zero parsers; drift surfaces as a named failure (REQ-register-cli-surface-drift-legible) |
| Quarantine strip is the literal first cask-hook statement, and the gate is never delegated to `generate_completions_from_executable` (2026-08-23.01 Phase 1, D-09) | engram ships unsigned; invoking the binary before stripping gets it SIGKILLed by Gatekeeper instead of failing legibly, and Homebrew rescues `write_completion` failures to a warning so a broken binary would install green | ✓ Good — ordering now pinned by `TestReleaseConfigCaskInstallGate`; four live installs on v0.16.0 |
| Dedicated tap-publisher App with a bare `{{ .Env.HOMEBREW_TAP_TOKEN }}` token; never re-widen the release App to the tap (#516) | One App, one purpose: the credential that writes the tap cannot cut a release and vice versa. GoReleaser regex-matches `repository.token` on the RAW string and rejects any conditional — which broke the v0.15.0 tap push and was invisible to `goreleaser check` and `--snapshot` | ✓ Good — v0.16.0 published; the publish-time-only field now has a test-time pin |
| Accept the reship-recovery criterion by construction; never rehearse a backfill (2026-08-23.01 Phase 1, D-15) | A staged rehearsal would be a real `workflow_dispatch` against the real tap; the newest-tag `skip_upload` guard plus the read-only credential probe are the property, and asserting Homebrew's side of it would violate rule `m45p2b4bp7` | ✓ Good — guard step, both branches, and the guarded-env idiom pinned by `TestReleaseConfigCaskReshipRecovery` |
| Secrets are env-var references on argv, never values; `--token-file` is provenance only (2026-08-23.01 Phase 3, D-16) | A shell history or process table captures anything on a command line; Claude/Codex/opencode each resolve `${VAR}` / `{env:VAR}` at connect time, so engram never needs to hold the value | ✓ Good — `TestNoSecretInArgs` across every auth mode × runtime |
| Codex skills route `codex-native-plus-index`: native files at `$HOME/.agents/skills` plus a delimited AGENTS.md index (2026-08-23.01 Phase 4, checkpoint:decision) | All three v1 runtimes turned out to have a native skill format, so `REQ-skills-agents-md-fallback` had an empty trigger set; the developer chose to route Codex there so the index path has a live write and A1 (Codex's selector reads `$HOME/.agents/skills`) was human-confirmed rather than assumed | ✓ Good — surfaced as a blocking decision, not guessed; five skills listed on all three runtimes |
| Verify `engram setup --apply` only against a fake `$HOME` / fake `Environment`, never the operator's (2026-08-23.01 Phase 4) | An agent ran the plan's own documented `--apply` with a placeholder URL and overwrote real MCP registrations for all three runtimes — the second hand-restore in one milestone; `--apply` always does registration AND skills with no opt-out (D-12) | ✓ Good — `withFakeSetupEnv` and the skills `Environment` seam are the only verification path (`ryr82bf2s2`) |
| `/engram-setup`'s mechanical prose is generated from the same Plans the CLI executes, with a CI regenerate-and-diff gate (2026-08-23.01 Phase 5) | Two hand-maintained instruction sets diverge silently; a keyword or liveness check can pass while proving nothing — the failure shape this repo has hit before | ✓ Good — both gate failure paths (source mutation, committed-artifact corruption) proven RED |
| Only confirmed nonexistence is the AGENTS.md create case (2026-08-23.01 Phase 4, 04-05 / #559) | `installAgentsMDIndex` treated every index read error as "no index" and overwrote the operator's file; read and write permission are independent, so an unreadable-but-writable index lost every byte outside the managed block with no error. D-15 already refuses to guess at a region engram did not author — an unreadable file is the most ambiguous state of all, so `errors.Is(err, fs.ErrNotExist)` is the only create path and any other read error preserves the file with zero writes and surfaces through the runtime row and the partial exit | ✓ Good — one predicate at one call site; package- and CLI-boundary regressions; found by the milestone audit, not by the phase's own tests |
| `osRun` classifies success first, then any ctx termination, then the `*exec.ExitError` unwrap (2026-09-13.01 Phase 1, D-10/D-11/D-12 + review WR-01) | Go surfaces a ctx-killed child as an `*exec.ExitError` (exit −1), so #560 needed ctx checked before the unwrap; checking it before `runErr == nil` would let an unsynchronized deadline read discard a genuine success. A real nonzero exit coinciding with the 20s deadline is reported as the timeout — accepted and documented per `runSeam` call site (every measured runtime call finishes in <2s) | ✓ Good — real-subprocess tests + red-evidence patches; three-iteration review converged |
| Man pages via a hidden live-binary `engram man` invoked by the cask hook, never a build-time generator or the declarative `manpage:` stanza (2026-09-13.01 Phase 1, D-01..D-09) | Keeps the cask's fail-closed proof that the installed binary works (3rd→4th live exercise); Homebrew's stanza shares the `rescue`-to-warning subsystem already rejected for completions. Date pinned to a constant, `DisableAutoGenTag` on root, `Source` carries the ldflags version | ✓ Good — byte-stable across runs; `TestReleaseConfigCaskInstallGate` pins ordering/occurrence/glob/forbidden literal |
| Red-evidence registration is a phase-level orchestrator step, not a plan task (2026-09-13.01 Phase 1) | `TestRedEvidencePatchesAreLive`'s empty-map guard turns `task` red from the first commit that creates an active-milestone phase dir; no plan's `files_modified` covers `internal/store/redevidence_harness_test.go`, so the planner never plans it (precedent: 6ce098e0 needed a separate PR) | ✓ Good — 4 patches registered after the last plan, before verification; recorded as gotcha `xjz60c9h6t` |
| A custom header is an ADDITIONAL secret-valued header orthogonal to `--auth`, never an `Authorization` rename (2026-09-13.01 Phase 2, D-01..D-10, user framing correction) | The research's "bearer generalized" shape would have coupled the gateway header to the auth mode; the real need is `Authorization: Bearer ${ENGRAM_TOKEN}` AND `x-gateway-api-key: ${GATEWAY_KEY}` on the same registration. `Authorization` as a `--header` NAME is rejected (one owner per header); validation lives once at the CLI boundary, never per-runtime; the value is only ever an env-var reference, scheme included in the variable's value | ✓ Good — five REQs verified by execution; engram spine `4kygesmh7v` |
| The shipped skill bundle stays vendor-neutral: the canonical gateway example is `x-gateway-api-key`/`GATEWAY_KEY` everywhere, not LiteLLM's header (2026-09-13.01 Phase 2, blocking checkpoint → option C) | `skill/engram/hooks/tests/test_no_residual_memory_oauth.py` (from `d33822b0`) bans vendor substrings under `skill/engram/` so the shipped `/engram-setup` cannot presume the deployer's gateway; rather than loosen the guard or fork the example between internal tests and shipped prose, the example was renamed in every surface. LiteLLM stays named only in docs-site's embedder guides — a different subject | ✓ Good — guard untouched and green; one `refactor(setup)` commit across 14 files |
| Plugin delivery is its own lane with its own outcome, not extra `Plan.Actions` in the registration sequence (2026-09-13.01 Phase 3) | One `execute()` sequence cannot report a `wrote` registration beside a `failed` plugin install (SC4), and its blind pre/post byte-compare would misclassify an already-current plugin as `wrote`. The lane authors per-runtime argv via an exported optional `PluginRuntime` interface, runs it through the same `runSeam`, and the facet is composed in `cmd/engram` — the shared executor stays content-blind | ✓ Good — planner refinement of the orchestrator's first instruction, accepted; `exitPartial` proven with both facets visible |
| Plugin capability is a read probe, not a version table (2026-09-13.01 Phase 3, D-10/D-12) | Pinning "which CLI version has `plugin`" would gate third-party behavior we do not own (rule `m45p2b4bp7`) and goes stale; `plugin list --json` exiting 0 with parseable JSON both proves the capability and supplies the installed version, so one subprocess answers both questions | ✓ Good — probe failure falls back to native with the reason, never a failed row |
| On the plugin path engram reports native leftovers and deletes nothing (2026-09-13.01 Phase 3, D-08/D-09) | The milestone exists because `--apply` once overwrote a working config (`ryr82bf2s2`); a tool whose last incident was an unwanted WRITE does not earn a delete path in the same milestone. `DetectPresence` is `Lstat`/`ReadFile` only — a future explicit prune verb can own removal | ✓ Good — read-only by construction, proven by test and by source inspection |
| `ENGRAM_HEADERS` mirrors `--runtime`/`ENGRAM_RUNTIME` (`os.Getenv` slice default, no koanf registry row) (2026-09-13.01 Phase 2, D-07) | `registry.go` already documents that a `StringSliceVar` cannot round-trip the changed-flag overlay; a proper row needs slice-aware overlay work that is its own design | ✓ Good — flag replaces the env list; deferred: first-class registry row |
| `preserved` means "a facet the current Options do not account for", and ambiguity never reaches it (2026-09-13.01 Phase 4, D-01/D-09) | Claude Code and Codex have whole-entry semantics, so the honest predicate is "would my write destroy something I cannot re-create"; explicit argv intent does not override an unaccounted-for header, and an unreadable/ambiguous read resolves to `would-write` (extending apply.go's "never already-correct from ambiguity") so a `preserved` row can always name what it preserves | ✓ Good — three states per parsed runtime with unit coverage; opencode stays uncompared by decision (D-10), never a false `preserved` |
| Redaction by construction, not by filter: compare raw in memory, carry only the redacted form, never retain probe text (2026-09-13.01 Phase 4, D-02/D-03) | A regex over free-form CLI output is one miss away from a cleartext secret in `--output json`; rebuilding `Result.Registered` from the parsed-and-redacted `Observation` means a value never copied out of the subprocess buffer cannot leak. Redaction is unconditional — setup never branches on reference-vs-literal — while the planned side (a bare `${VAR}` name engram authored) renders in full | ✓ Good — proven at `Observe`, `Compare`, and the process boundary against the OBSERVED literal shape; code review's only Warning was a dropped `boundCapture`, fixed |
| A total parse for both scanners: unrecognized content is an unaccounted-for facet → `preserved` (2026-09-13.01 Phase 4, D-11) | A tolerant scanner that ignores lines it was not taught silently defeats D-01 — the exact overwrite this milestone exists to prevent; `Status:`/`Issue:`/the removal trailer are recognized chrome so an unreachable server never reads `preserved`, and Codex uses `DisallowUnknownFields` plus a key-set diff to NAME the unknown key | ✓ Good — a CLI release that adds a field makes setup cautious, not blind |
| The literal-header echo shape is a maintainer-run observation record, not an assumption (2026-09-13.01 Phase 4, D-05/D-08) | Rule `m45p2b4bp7` forbids a test that invokes a real CLI, and the research session could not mutate; a dated `04-OBSERVATIONS.md` (throwaway `probe-literal-04`, dummy `sk-` value, isolated `CODEX_HOME`) is what every literal-echo fixture quotes and cites | ✓ Good — confirmed `claude mcp get` exits 0 on a failed dial and echoes cleartext while `mcp add` prints `[REDACTED]`; Codex carries the literal under `transport.http_headers` |
| `preserved` outranks `already-correct` in row precedence (2026-09-13.01 Phase 4, planner decision) | Research suggested slotting it below `already-correct`; that would hide a preserved registration beneath a green plugin facet at the headline — the one row an operator actually reads | ✓ Good — `failed > wrote > preserved > already-correct > would-write > not-present`, pinned literally in `TestAggregatePrecedenceIsAuthoredNotDerived` |
| `--apply` compares first and never runs an action on `preserved` or `already-correct` (2026-09-13.01 Phase 5, D-01) | The 2026-09-10 incident was `--apply` blindly running Claude Code's remove-then-add; with Phase 4's classification available pre-write, a `preserved` row returns before `plan.Actions[0]` and an `already-correct` row is a true no-op — no remove-then-add churn, no OAuth logout on every re-run. Byte-compare survives only for runtimes without a scanner (opencode) and ambiguous reads | ✓ Good — `TestApplyPreservedNeverRunsClaudeCodeRemove` pins it with a panic-on-overrun harness; ten red-evidence patches registered |
| The OAuth re-login consequence is decided by observed SHAPE, rendered as a row note in both lanes, and never pauses `--apply` (2026-09-13.01 Phase 5, D-03/D-04) | No read verb exposes auth state and the `oauth-client` read-back shape is unobserved; a registration with no `Authorization` header that classifies `would-write` on Claude Code is warned conservatively (a false warning costs nothing, a missed one is a silent logout). `--apply` stays the only consent gate — preview-by-default is where the operator reads it "before it runs" | ✓ Good — `Observation.RewriteConsequence` authored in `claudecode.go`; bearer/preserved/already-correct rows never carry it |
| No `--replace-registration` flag: a `preserved` row names the manual step (2026-09-13.01 Phase 5, D-05) | engram never destroys what it cannot reproduce; the destructive step belongs to the runtime's own tool with its own confirmation semantics (`claude mcp remove engram --scope user`; Codex's `[mcp_servers.engram]` table). A second consent flag re-opens the incident class the moment it lands in a script | ✓ Good — `Observation.ManualRemediation` per runtime; the row and the guide both name it |
| Docs close out on a post-release observation, not from code (2026-09-13.01 Phase 5, D-06/D-07) | The `2026-08-23.01` D-10 precedent: guides split by reader intent (install = binary + cask contents incl. man pages; agent-setup = running setup; plugin = what the plugin is), each pinned by a docs gate; `05-POST-RELEASE.md` (tracker #567) lists the qualifying-release checks; `REQ-docs-setup-v2` stays open and the verification carries `post_release_status: pending` | ✓ Good — `05-RELEASE-0.17.0.md` recorded the v0.17.0 observation on 2026-09-18 and REQ-docs-setup-v2 closed; closes when `05-RELEASE-<ver>.md` records the observation after the next release |
| One Qdrant client constructor, `store.NewQdrantClient(host, port, ...grpc.DialOption)`, for production and every test; tests pass their receive limit in grpc's own vocabulary and pin a named 4 MiB (2026-09-18.01 Phase 1, D-01/D-02/D-04) | The package that owns the Qdrant client owns its dial options; passing `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(n))` through avoids a bespoke wrapper; a named 4 MiB is production's effective ceiling today and stays pinned after Phase 5's backstop, so a regression test proves the mechanism, not the backstop (rule `m45p2b4bp7`) | ✓ Good — production dial options unchanged; 11 test sites converged |
| Shared harness in `internal/store/storetest` (httptest idiom); harness-using `internal/store` tests are black-box `package store_test` (2026-09-18.01 Phase 1, D-06/D-07/D-10) | `storetest` imports `store`, so in-package tests cannot import it (cycle); the external test package is Go's sanctioned cycle-breaker. Four `TestMain`s, three `requireQdrant`s and four image-tag copies collapsed into one lifecycle, with each package's quirks (e2e early parse, retrievaleval opt-in gate) preserved | ✓ Good — CI shared-address count holds at 3 PASS + 1 SKIP |
| Client convergence is gate-enforced, not conventional: an AST gate allows `qdrant.NewClient` only inside `store.NewQdrantClient` (test files included), and the holder no-write check covers every allowlisted holder (2026-09-18.01 Phase 1, D-11/D-13) | The #583→#585 recurrence showed convention drifts; code review then closed function-value-alias and aliased-import bypasses in both gates, each proven RED by fixture | ✓ Good — four red-evidence patches registered; review loop converged clean |
| An over-receive-limit Qdrant response is classified ONCE by a unary client interceptor in `store.NewQdrantClient`'s base options into `store.ErrResponseTooLarge` (+ `ResponseTooLargeError` detail for logs), matching the status code AND grpc-go's receive-limit message shapes (2026-09-18.01 Phase 2, D-01..D-03) | One chokepoint covers every current and future Qdrant RPC in production and tests; the message-shape match is what keeps a server-sent `ResourceExhausted` (and qdrant-go-client's own retry-after handling, which sits outside ours) untouched | ✓ Good — four receive shapes matched, send shapes and server-side statuses pass through; 12 red-evidence patches live |
| Wire contract for the overflow: `field=response hint=too_large` via ONE shared renderer; Connect `resource_exhausted`; MCP via a receiving middleware reading go-sdk's `CallToolResult.GetError()`; CLI exit `10` on client AND operator tiers (2026-09-18.01 Phase 2, D-04..D-10) | `response` is a fixed pseudo-field so the mapper never couples to tool argument shapes; the middleware needs no edits to the 15 tool closures; one failure class, one exit code on both tiers. The 11th hint code is bound to errors.md by a go/parser doc gate | ✓ Good — no byte count or upstream text on any wire; deferred: MCP scrubbing of other unclassified errors (D-09) |
| Memory `content` gets an always-enforced write cap, `ENGRAM_MEMORY_MAX_CONTENT_BYTES` (default 65536), plus the tags caps `ENGRAM_MEMORY_MAX_TAGS` (128) and `ENGRAM_MEMORY_MAX_TAG_BYTES` (128); `0` is rejected (2026-09-18.01 Phase 3, decision A: D-01, D-09, D-10) | Byte-budget paging needs a provable per-record ceiling and the per-RPC count is derived from these caps, so a disabled cap would silently remove the bound; citations (50 × 16 KiB) already dominated once content was capped and tags were the other unbounded field; rejection reuses `too_long`/`too_many` so no new wire vocabulary; existing over-cap records are never rewritten and stay readable and trimmable | ✓ Good — enforced on every memory write path (MCP, Connect incl. UpdateMemory's field-mask lane, engram store); residual: payload fields with no write cap are budgeted by documented allowances, covered by the batch-of-1 fallback and the named error |
| Two shared bounded-read primitives, not per-site patches: an ordered-page keyset helper (`scrollOrderedPage`) and a byte-budget `scrollAllPoints`, both sized from a per-view record ceiling derived from the enforced write caps, with a batch-of-1 fallback and a named error instead of a silent skip (2026-09-18.01 Phase 3, D-02..D-07) | #583→#585 showed per-site fixes recur; a ceiling derived from caps makes the per-RPC count arithmetic rather than discovered at runtime, and measured-byte page budgets bound the caller's response too. Pages report `CutByBudget` so a budget-cut page is never mistaken for the last page | ✓ Good — 27-site inventory recorded and assigned; Phase 4/5 migrate onto these; 13 red-evidence patches |
| Decision B — one documented maximum, 1000, for every recall count knob (`limit` on Connect `ListMemories`/`list_memory`/`list_scheduled`, `k` on `SearchMemories`/`SearchDiscoveries`/`search_memory`/`search_discovery`, and `list_rules`' own ceiling); Connect `ListMemories`' `limit: 0` now resolves to that maximum (was: unbounded "all"); an over-maximum count is rejected, never clamped, via a new `out_of_range` hint (2026-09-18.01 Phase 4, D-01/D-02/D-03/D-10, REQ-list-limit-contract-decided) | The word "all" was never a documented number, and a silent cursor-mode clamp hid an under-fetched page from a caller who never wrote a bound; stating the same numeric constant on the wire schema, the CLI help, and the tool reference means a caller learns the ceiling by reading, never by triggering a rejection | ✓ Good — proto comments, both CLI flag help strings, the tool reference, the CLI guide, CLAUDE.md's memory contract, and a durable source-derived docs gate (`TestRecallMaximumIsStatedNumerically`) all state the same `store.MaxRecallLimit` constant; announced BREAKING in the upgrade guide |
| The four own-loop operator sweeps migrate onto `scrollAllPoints`, each with a per-sweep projection of the fields it actually reads, and no new characterization tests — the in-place suites are the regression net (2026-09-18.01 Phase 5, D-01/D-02/D-04) | One mechanism keeps the #583→#585 recurrence from repeating per site; a projection makes the per-RPC count arithmetic for each sweep instead of assuming full payloads | ✓ Good — all ten sweep rows route through a budgeted view; `unbudgetedView` deleted outright |
| `MaxCallRecvMsgSize` = 64 MiB, set in exactly one place as a backstop; its test asserts only that the option is passed through, never grpc-go's behavior (2026-09-18.01 Phase 5, D-05/D-06) | Raising the ceiling alone only moves it (#583); as defense in depth it must never be what a regression test relies on (rule `m45p2b4bp7`) | ✓ Good — `productionRecvLimit` in `qdrantDialOptions`, the sole body of `NewQdrantClient` |
| A failed `ListScopes` after a cross-spine recall keeps the hits and reports an additive `scopes_unknown` boolean, with `searched_scopes` ABSENT rather than an empty list; the cause is logged server-side (2026-09-18.01 Phase 6, D-01..D-06) | Discarding already-authorized hits turned a coverage-reporting failure into a recall failure (#456); absence-vs-empty keeps the three states (not cross-spine / known / unknown) distinguishable on both transports | ✓ Good — one fix inside `searchedScopes`, one shared CLI footer, proto change additive |
| Provider response drains are bounded by a timer that closes the body (no goroutine) in a shared `internal/httpdrain`, on all four embed/summarize sites; `0` skips the drain; defaults set before options so an explicit zero survives (2026-09-18.01 Phase 7, D-01..D-06) | A byte limit alone cannot stop a slow trickle, and a drain relying on `http.Client.Timeout` is unbounded under `WithTimeout(0)` (#457); one helper keeps the twin clients from drifting | ✓ Good — both axes proven per client, wiring gated by a go/parser test |
| `WithTimeout(d <= 0)` now resolves to a configurable ceiling (`ENGRAM_{EMBED,SUMMARY}_MAX_TIMEOUT`, default 10m), clamped in `New` after all options run (2026-09-18.01 Phase 7, D-07..D-09) | "Zero means no timeout" was the last escape hatch around every bound; the ceiling is a visible number an operator must write, and the doc comments state that an absurd ceiling is effectively unbounded (accepted, AR-1) | ✓ Good — upgrade guide §18 announces the changed meaning; WR-02 (no Timeout-vs-MaxTimeout cross-check) accepted as harmless |
| The red-evidence mutation harness (`TestRedEvidencePatchesAreLive`, 63 patches across phases 1–7) is removed; behavioural regression tests are the proof (2026-09-18.01 close-out, rule `3p0zsqrhmb`) | A harness proving that tests fail is a test for tests; it cost ~4.5 s per patch, repeatedly timed out under host load, and stranded applied mutations when killed | ✓ Good — removed in `c1afd6c1`; the bounds stay pinned by the behaviour tests the patches targeted |
| The shipped ranking is picked by a pre-committed, unit-tested rule (`decideRanking`, D-05), never by eye; on the live 96-record multi-domain corpus with blind paraphrase queries it kept `lexical` (paraphrase MRR 0.817 vs vector-only 0.579), reversing spike 004's 16-record single-domain finding (2026-09-22.01 Phase 1, D-05/D-07/D-10) | A human or agent choosing after seeing the numbers turns the eval into a rationalisation; the rule (eligibility, 0.05 tuned-only margin, best MRR, simplicity tie-break) and the tuning grid were fixed in code before any live number existed | ✓ Good — approved winner == rule winner; caveats carried to #605: lexical's paraphrase recall@8 0.950 vs vector-only 1.000, and the live AsymmetryDiffer SKIPs under a symmetric embed config |
| Ranking changes ship through one seam, `store.rankCandidates`, whose body is the whole diff for any future winner (2026-09-22.01 Phase 1, 01-06) | A re-run that disagrees with the shipped winner should change exactly one function and its pinning test, never a caller; Phase 4's Jev reranker plugs in at the same point | ✓ Good — `SearchReranked` → `rankCandidates` → `RerankHits`; no lexical code deleted, no tuned constant added |
| Paraphrase queries are authored blind — a fresh, tool-less subagent sees topic labels only, never the corpus (2026-09-22.01 Phase 1, D-01) | Spike 004's queries were written by the corpus author, so the fixture could not measure paraphrase robustness independently; the labels-only prompt with a `---8<---` provenance marker makes the independence boundary mechanical | ✓ Good — `01-BLIND-QUERIES.md` committed unedited; human sign-off recorded in `01-VERIFICATION.md` |
| `ENGRAM_RETRIEVAL_EVAL` is resolved by a package-local koanf gate and deliberately NOT registered in `internal/config` (2026-09-22.01 Phase 1, D-15 revised) | It is a test-only switch; a registry row would make it a production config knob and subject it to `Config.Validate()`. The gate mirrors production's prefix/precedence without touching the registry | ✓ Good — precedent for any future test-only `ENGRAM_*` flag |
| Hand-write the Jev client on `net/http` rather than adopt OpenRouter's Go SDK v0.8.19 (2026-09-22.01 Phase 2, D-05..D-07; engram `0bwxc0asap`) | The SDK passed the maintenance and typed-decode bars but hardcodes `/api/alpha/decisions` (unreachable through the LiteLLM pass-through), cannot decode LiteLLM's string `code` errors, and reads bodies unbounded — engram would own path, classification and bounds anyway, leaving only generated types as the benefit against a new dependency on an alpha API | ✓ Good — zero new Go deps; live PASS on both base URLs |
| Decisions config: provider enum off by default; the API key falls back to `ENGRAM_OPENAI_API_KEY` but the base URL never falls back (2026-09-22.01 Phase 2, D-01..D-03) | `ENGRAM_OPENAI_BASE_URL` is the LiteLLM `/v1` chat root, and the spike findings forbid assuming the chat gateway serves Decisions; the enum leaves room for the emulator backend without another toggle | ✓ Good — `Config.Validate` rejects `provider=jev` without a base URL |
| Decision failures are named by HTTP status class, never by message text, with one jittered retry on 429/5xx inside the timeout budget (2026-09-22.01 Phase 2, D-11/D-12) | OpenRouter and LiteLLM error bodies differ (`code` numeric vs string); structural classification survives both, and callers need `errors.Is` to degrade gracefully | ✓ Good — both dialects pinned by tests and observed live (LiteLLM 401 → `ErrDecisionAuth`) |
| `decodeResponse` is strict: every requested question must be answered, and an answer whose type differs from the request is rejected (2026-09-22.01 Phase 2, DEC-02; WR-01 fix) | A lenient decode copied answers by name regardless of request, which hid a mismatched test fixture and would let a Choice question silently return a Score answer | ✓ Good — `e3a60dbd` |
| Consolidate verdicts run by default whenever a decisions provider is configured, with `--no-verdicts` to suppress; the verdict is a nested object that is absent when verdicts did not run, with no separate top-level probability (`probabilities[relation]` is the single source) (2026-09-22.01 Phase 3, D-04/D-05) | Configuring a provider is already the opt-in; a second flag would be redundant. Keeping the absent-when-off object additive preserves the operator JSON contract, and the full probability map keeps close calls visible | ✓ Good — byte-identical output with no provider; CUR-01 verified |
| Verdicts are advisory and structurally unable to mutate: consolidate's store interface exposes only `NearDuplicates` + `RecordStates`; a per-pair failure renders its named error class and the sweep exits 0; no per-run cap (2026-09-22.01 Phase 3, D-10/D-11, CUR-02) | Design intent is explicit, never-automatic curation, so a type-level read-only surface is stronger than a review convention. Advisory output must never fail the sweep (the DEC-04 contract), and `DecideMany` concurrency already bounds the load | ✓ Good — `TestConsolidateStoreSurfaceIsReadOnly` pins the method set by reflection; a `--max-verdicts` cap was deferred |
| Curation eval corpus is author-with-label plus blind check, keeping only agreed pairs; round 1 (80/80 agreed) was rejected as cue-rich, and the hardened, shuffled round 2 kept 70/80; no OSS pair datasets were added (2026-09-22.01 Phase 3, D-01/D-02; user decision) | Fixed interleave order and recordB cue phrases let the labeler infer the class from wording instead of content, so a perfect agreement score showed a leak, not quality. The user declined external datasets, so the committed spine-shaped corpus is the measure | ✓ Good — all 10 round-2 disagreements were `contradicts`/`updates` confusions, the same boundary the live eval shows |
| Report-first eval with one hard gate: verdicts at p ≥ the needs-review threshold must reach accuracy ≥ 0.9; buckets, Brier and the confusion matrix are reported, never gated (2026-09-22.01 Phase 3, D-03) | The gate validates the 0.9 threshold without tuning the model to the corpus; per-bucket numbers stay comparable to the curation spike's reference figures | ✓ Good — live PASS 40/40 at p ≥ 0.9, Brier 0.138 (uniform 0.800), low bucket 0.400 / mid 0.950 |
| The Jev reranker is a registered ranker enum, `ENGRAM_SEARCH_RANKER` = `lexical` (default) \| `jev`, that ships opt-in with no eval bar; `jev` without a decisions provider fails `Config.Validate` (2026-09-22.01 Phase 4, D-01/D-02) | An enum leaves room for another ranker without a second toggle, and a clear startup rejection beats a silently inert opt-in. The eval records Jev beside lexical and vector-only for operators to judge; it never gates the option, and Jev is structurally excluded from the D-05 winner decision | ✓ Good — live Jev MRR 0.883 vs lexical 0.817 vs vector-only 0.579, recall@8 1.000, #261 rank 1; the default stays `lexical` regardless |
| Jev composes after lexical, never replaces it: the whole `CandidateK` pool (32–100) goes in one Decisions request, is stable-sorted by P(relevant), then truncated to k, so ties and any failure keep exactly the shipped lexical order (2026-09-22.01 Phase 4, D-03/D-04) | The fallback must be today's order, byte-for-byte, or a Jev outage would change results silently; one request per search bounds cost and latency | ✓ Good — `rankCandidates` stays pinned; `store.RankHook` keeps `internal/store` free of an `internal/decide` import |
| Per-hit `relevance` (0–1, omitempty) beside the cosine `score` on MCP, Connect (`optional double relevance = 31`) and the CLI; no response-level "nothing relevant" flag (2026-09-22.01 Phase 4, D-05/D-06) | Absence means "the ranker did not run or fell back", which a zero could not express; `double` keeps the Connect value bit-identical to the MCP float64. The caller decides from the per-hit values rather than a server threshold | ✓ Good — additive proto change, `buf breaking` clean; the CLI column is data-derived so lexical output is byte-identical |
| The search path gets its own decider: `jev.WithNoRetry()` plus `ENGRAM_SEARCH_RERANK_TIMEOUT` (2s), separate from the decisions timeout and single retry that consolidate keeps (2026-09-22.01 Phase 4, D-09) | A retry inside a synchronous search doubles tail latency for a ranking that has a free fallback; sweeps are offline and benefit from the retry | ✓ Good — 0/26 fallbacks at 2s in the live eval |
| Decision state per candidate is summary + content head at 600 chars, shrunk uniformly (floor 100) under a 28k-token chars/4 guard, so 100 candidates fit Jev's 32k context (2026-09-22.01 Phase 4, D-08) | Reuses Phase 3's state shape instead of a second truncation scheme; the guard is arithmetic at the recall maximum rather than discovered at runtime | ✓ Good — boundary, floor, and multi-byte cases pinned in `internal/relevance` |
| Helm exposes `memory.search.ranker` / `memory.search.rerankTimeout`, gated independently of `memory.decisions.provider` so `jev` without a provider still renders and reaches the server's rejection (2026-09-22.01 Phase 4, D-10) | Nesting under the provider gate would render nothing and leave the operator's opt-in silently inert | ✓ Good — default render byte-identical; `chart:validate` checksum re-pinned |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---

*Last updated: 2026-09-24 after Phase 5 (Operator Correctness) of milestone 2026-09-22.01*
