# Phase 2: Setup Command Core - Context

**Gathered:** 2026-08-29
**Status:** Ready for planning

<domain>
## Phase Boundary

`engram setup` exists as a real, preview-by-default, fully scriptable cobra command that detects
which supported agent runtimes are present on the machine and reports the exact command it would
issue to each — before it writes anything. Alongside it lands `internal/setup`: the `Runtime`
interface, the `Plan`/`Action`/`Outcome` types, the `Environment` injection seam, and the
package-level `Runtimes` registry.

`cmd/engram/catalog.go` panics if any cobra command lacks a row in
`internal/surfaces/toolclass.go`, and `destructiveByClassification` (`cmd/engram/destructive.go:57`)
panics on the same absence — so registering the `setup` command and adding its classification row
land together in this one phase, never sequenced across two.

**In scope:** the `setup` command and its full flag surface; its `internal/surfaces/toolclass.go`
row; its `internal/config/registry.go` rows; two new exit codes and their catalog/golden churn;
`internal/setup`'s core types and `Environment` seam; real `Detect()` and real `Plan()` for
claude-code, codex, and opencode.

**Out of scope:** `Apply()` actually shelling out to any runtime CLI (Phase 3); the generic-MCP
portable-config emitter (Phase 3, REQ-register-generic-mcp); skills distribution (Phase 4); the
`/engram-setup` delegation gate and `internal/setupgen` (Phase 5); Cursor (deferred to v2 at
scoping); install documentation (Phase 6).

</domain>

<decisions>
## Implementation Decisions

### Flag surface and auth vocabulary

- **D-01:** The server URL is supplied by **`--url`**, never `--server`. This is not a style
  preference: `cmdwalk.go:118`'s `operatorCommands()` predicate reads the LIVE flag set
  (`cmd.Flags().Lookup("server") != nil`) and excludes any command carrying a flag literally named
  `server`. A `setup` command spelled `--server` would silently drop out of operator-tier
  classification — the opposite of this milestone's intent, and it would fail by absence rather
  than by error. `--url` also has zero collision with `addClientFlags`.
  Accepted consequence: this binary now carries a third spelling of "where engram lives"
  (`--server` on the client tier, `ENGRAM_SERVER_URL` in the registry, `--url` here). Justified
  because `setup` writes a URL into someone else's config rather than dialing it itself.

- **D-02:** `--url`'s value is the **MCP endpoint, verbatim**. engram never appends `/mcp`, never
  strips it, and never guesses. Whatever the caller passes is what gets written into the runtime's
  config.
  This is the only correct choice for two documented deployment shapes that no suffix rule could
  derive: a gateway-fronted route (`https://gw.example.com/mcp/engram`, per
  `skill/engram/commands/engram-setup.md:21-23`) and an `ENGRAM_MCP_PATH=/` deployment restoring
  the legacy root catch-all (ADR engram-bj6, cited at `engram-setup.md:26`).
  Accepted consequence: a user who passes a bare base URL gets a registration that 404s, and
  engram will not warn. A suffix heuristic was explicitly rejected — it would cry wolf on a
  legitimate root-mounted deployment, and engram would be encoding an assumption about a
  deployment it cannot see.
  — **Reversibility:** one-way — flipping to append-`/mcp`-by-default later silently rewrites what
  every existing scripted invocation means, and breaks exactly the two deployment shapes this
  decision exists to keep correct.

- **D-03:** Auth mode is selected by **one `--auth` enum flag**: `oauth | oauth-client | bearer |
  none`. These four values are exactly the four rows of the table already shipped at
  `skill/engram/commands/engram-setup.md:40-45`, so Phase 5's generated-equivalence proof compares
  like to like rather than reconciling two vocabularies.
  Validated through the established path — a bad value routes to `usageErrorf` → `exitUsage` (2),
  the same shape `config.ValidateOutputFormat` gives `--output`. `--help` then lists the accepted
  modes by construction, which is what REQ-setup-correct-by-reading and success criterion 5 ask for.
  Rejected: **inferring** the mode by probing the URL for a 401 — a network call inside a command
  whose entire contract is "changes nothing, and here is exactly what it would do", where an
  unreachable server would turn a preview into a failure.
  Rejected: **separate per-mode flags** with the mode implied by which are present — that needs
  cobra's `MarkFlagsMutuallyExclusive`, which raises a plain `fmt.Errorf` bypassing
  `cliError`/`ExitCode()`. That is precisely the defect v0.13.x unified #453 with #467 to close,
  and adopting it here would reintroduce it one command over.

- **D-04:** `--url`, `--auth`, and `--runtime` each get a row in `internal/config/registry.go`
  (`ENGRAM_`-prefixed, env-first with flag override). **`--apply` deliberately gets no row.**
  Grounds, measured rather than assumed: 45 of the registry's 48 entries carry an `Env` row.
  Env-with-flag-override IS the koanf idiom this repo committed to; omission is the exception, and
  each of the three existing exceptions carries a written justification for why that key is not
  deployment state — `client.token_file` (D-13: a credential must never reach argv),
  `client.output` ("per-invocation rendering choice, not a deployment setting"), and
  `client.insecure` (the flag's help text already promises no env fallback). None of those
  rationales extends to `--url` or `--auth`, which are deployment facts. Enrolling them is the
  least-surprising behavior.
  `--apply` is excluded on the `client.insecure` precedent: an exported environment variable that
  silently converts a preview into a mutation is the same class of harm. Verified that no existing
  mutating operator command (`migrate`, `prune-expired`, `reindex`) has an `apply` registry row.
  — **Reversibility:** costly — an `ENGRAM_` var is a published contract; removing one later is a
  breaking change for any operator who exported it.

- **D-05:** The bearer credential is supplied by **`--token-file`**, carrying a PATH — never the
  secret. This mirrors `client.token_file` (`internal/config/registry.go:91`) exactly, including
  its deliberate absence of an `Env` row for the credential itself. No secret ever reaches argv or
  the process table, satisfying REQ-register-auth-modes' constraint at the flag-surface layer that
  Phase 2 owns.

### Exit-code taxonomy

- **D-06:** Partial success is a **new `exitPartial = 8`**; total failure is a **new
  `exitSetupFailed = 9`**. Three-way is then structurally unambiguous: `0` / `8` / `9`, testable by
  a CI caller without parsing JSON — which is what REQ-setup-partial-failure-legible requires.
  Neither code has an `exitCodeForConnectErr` producer, so each needs an entry in
  `nonConnectProducedCodes` (`cmd/engram/catalog_test.go:348`) naming `engram setup` as its path —
  the same named-allowlist mechanism `exitFindings` already uses, which keeps
  `TestCatalogExitCodesMatchMapper`'s set-equality gate a real gate in both directions rather than
  weakening it to a subset check.
  **Exit 1 was not available.** `catalog.go:132` redefines `exitGeneric` as "unclassified internal
  error (backstop only — not a general-purpose failure code)"; D-02 of that phase deliberately
  typed every classified path. Reusing 1 for total failure would reverse a published,
  golden-pinned decision.
  Rejected: reusing `exitFindings = 7` — its published meaning is bound to an explicit opt-in flag
  (`spine-review verify --fail-on`), and setup's partial failure is neither opt-in nor a finding;
  reusing it would force rewording a golden-pinned string another command owns.
  **Churn this requires, in ONE commit** (the existing gate is unsatisfiable if split):
  `client_common.go`'s const block, `catalog.go`'s `doc.ExitCodes`, `catalog_test.go:231`'s
  `wantExitCodes`, `catalog_test.go:348`'s `nonConnectProducedCodes`, and
  `cmd/engram/testdata/catalog.golden`.
  — **Reversibility:** one-way — a process exit code is a published contract the moment a script
  branches on it.

- **D-07:** A runtime that simply is not installed is an **expected outcome, never a failure**. It
  is reported as a `not-present` row and does not affect exit status. Failure means "I tried and
  could not", not "there was nothing to do" — a machine with only Claude Code installed must exit
  `0`, not `8`.
  This requires `not-present` to be a **first-class `Outcome` value** alongside `already-correct` /
  `would-write` / `wrote` / `failed`, never modeled as an absence.

- **D-08:** A **preview run (no `--apply`) exits nonzero only for usage or config errors** — a bad
  `--auth` value or a malformed `--url` gives `exitUsage` (2). Everything else exits `0`, including
  a report that a runtime's CLI is missing. Preview's job is to tell you, not to fail, which keeps
  `engram setup` safe to run unconditionally in a script's inspection step. Consistent with
  `migrate` and `prune-expired`, both of which exit 0 on a preview with pending work.

### Runtime selection and detection

- **D-09:** **Phase 2 ships real `Detect()` AND real `Plan()`** for claude-code, codex, and
  opencode. `Apply()` returns a not-yet-implemented error until Phase 3.
  This deliberately **overrides the research's build-order item 2**
  (`.planning/research/ARCHITECTURE.md:191`), which proposed Phase 2 ship an *empty* `Runtimes`
  registry. That is unsatisfiable against this phase's own success criterion 1, which requires
  `setup` to report which runtimes are present and show "the exact command or content it would
  issue per runtime" — an empty registry reports nothing on every machine, and the roadmap's goal
  statement would be untrue at phase close.
  **Planner and Phase 3 must be told this explicitly:** the `claude mcp add` / `codex mcp add` /
  `opencode mcp add` invocation strings are AUTHORED HERE, in `Plan()`. Phase 3 owns *executing*
  them, not re-deriving them. A Phase 3 that re-derives the strings creates the two-encodings drift
  this milestone exists to prevent.
  **Consequence for success criterion 2 (resolved 2026-08-30, user-blessed at plan-check):** a
  stubbed `Apply()` makes the original criterion 2 — "`engram setup --apply` twice converges,
  reporting 'already correct' distinctly from 'wrote it'" — unreachable this phase: every runtime
  attempted under `--apply` reports `failed`, on both runs. D-09 as originally written reasoned
  about criterion 1 only and was silent on criterion 2; that was an oversight, not a decision.
  Criterion 2 and REQ-setup-idempotent have therefore MOVED TO PHASE 3, where `Apply()` executes.
  Phase 2's criterion 2 now proves only what it can: the five-value `Outcome` vocabulary and
  deterministic classification from a `Plan()`. ROADMAP.md and REQUIREMENTS.md were amended to
  match, so no artifact claims a behavior this phase does not deliver.

- **D-10:** A bare `engram setup` (no `--runtime`) targets **every detected runtime**. Detection is
  the feature; the milestone's own pitch is "detects what's on the machine, shows what it would
  write, and wires it up." Preview-by-default is what makes a multi-runtime `--apply` safe.

- **D-11:** `--runtime` is a `StringSliceVar` — settled by precedent, not by choice: it is this
  binary's established idiom for multi-value flags (`--tags`, `--categories`, `--id`, `--class`),
  and it accepts both `--runtime a --runtime b` and `--runtime a,b` for free.
  Handling splits by KIND of wrongness:
  - An **unknown name** is a usage error → `usageErrorf` → exit 2, and the message lists the valid
    names. Same shape as a bad `--output`.
  - A **valid name for an absent runtime** is a `not-present` row and exits 0, per D-07.

  These are genuinely different: the first is a fact about the invocation, the second a fact about
  the machine, and the remedies share nothing.

- **D-12:** Detection is **`exec.LookPath` on the runtime's own binary, and nothing else** —
  routed through the injectable `Environment` seam so tests need no `t.Setenv` races. A config
  directory is never consulted.
  This satisfies REQ-setup-detects-runtimes ("a config directory left behind by an uninstalled
  runtime does not read as installed") **by construction** — a stale directory is never read, so it
  cannot produce a false positive. Accepted, documented consequence: a runtime installed outside
  PATH reads as absent. That is a false negative, which is the safe direction.
  Rejected: `<runtime> --version` capture — it makes a read-only preview shell out to three
  third-party binaries, where a hung CLI stalls the command and would need a timeout this phase
  would have to define.

### Preview mechanism, classification, and report shape

- **D-13:** `setup`'s mandatory `internal/surfaces/toolclass.go` row is
  `Class{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false}`.
  `Destructive: true` follows the table's own stated conservative rule ("false only when EVERY
  valid invocation is purely additive"): `claude mcp add engram <url>` over an existing `engram`
  entry overwrites it. `Idempotent: true` is REQ-setup-idempotent restated as a hint.
  The row's comment MUST say why `Destructive` is true, since `setup` reads as install-time
  convenience and a reviewer will otherwise question it.
  `CLICommand: "setup"`, `MCPTool: ""` — setup has no MCP counterpart, which the table's `Operation`
  doc comment already establishes as a legitimate, deliberate empty column, never "unclassified".

- **D-14:** `setup` routes through **`registerDestructive`**, not a bespoke preview/apply path.
  `destructiveByClassification`'s admission gate is `!class.ReadOnly` (generalized in Phase 4's
  D-16 precisely so an additive-but-mutating command still gets the preview contract), so D-13's row
  admits it. `--apply` is then registered by `addApplyFlag`, whose usage string is composed from
  `surfaces.RuleDestructiveRequiresApply.Sentence` — referenced from the registry, never re-typed —
  so `TestSurfaceConformanceCobraUsage` is satisfied by construction rather than by a hand-copied
  string staying in sync by convention.
  Where an inherited mechanism is meaningless for `setup` (e.g. `cliNow`'s preview cutoff clock,
  which exists for store-sweep commands), say so in a comment rather than silently not using it.

- **D-15:** The report is **one typed document rendered through `renderOperator`**, with the
  per-runtime outcomes as a `runtimes` array. `renderOperatorView` already handles arrays of
  objects: `viewRow` (`cmd/engram/operator_view.go:161`) renders each element as a dense
  `key=value key=value` line with RAW keys, while top-level fields get humanized labels — the
  documented D-05 asymmetry, not an inconsistency to fix. This preserves the
  one-serialization-plus-a-view invariant, so text and JSON cannot drift.
  **Accepted consequence, recorded so a reviewer does not file it as a defect:** the exact
  invocation lives as a FIELD on each runtime's row, so the text lane renders it densely
  (`name=claude-code state=present outcome=would-write command="claude mcp add ..."`) rather than
  as a copy-pasteable block. That still satisfies REQ-setup-previews-by-default's "the exact
  command, not a summary of it" — it is exact, merely not pretty. A separate verbatim block was
  considered and rejected as a second Phase-1-D-07-style divergence not worth taking twice.
  Note `sanitizeViewValue` (`operator_view.go:223`) is what stops a value from forging report
  structure; a command string containing quotes or newlines passes through it.

- **D-16:** Under `--auth bearer`, preview prints the literal command with the credential redacted
  **as its provenance**: `Bearer <from /path/to/token>`. Every part of the command that is not the
  secret is exact. A fixed `Bearer ***` mask was rejected: on a machine with several token files,
  *which* credential would be used is the one detail worth previewing, and a wrong-file mistake
  would otherwise stay invisible until after `--apply`.

### Claude's Discretion

- The precise Go shapes of `Plan`, `Action`, `Outcome`, `Result`, and `Environment` — field names,
  whether `Outcome` is a string enum or an int, file placement within `internal/setup` — are the
  planner's, subject to D-07's requirement that `not-present` be a first-class `Outcome` value and
  D-09's requirement that `Plan()` carry the exact invocation string.
  `.planning/research/ARCHITECTURE.md:60-104` sketches an interface; treat it as a starting point,
  not a contract — its `SupportsNativeSkills()` method serves Phase 4 and need not land here.
- Whether the two new exit-code consts live in `client_common.go`'s existing block or in a
  `setup`-owned file, subject to D-06's one-commit churn requirement.
- The `--help` prose wording, subject to success criterion 5: it must teach targetable runtimes,
  what `--apply` does, and the accepted auth modes without the caller having to run it and
  interpret a failure.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition and requirements
- `.planning/ROADMAP.md` § "Phase 2: Setup Command Core" — goal, 6 requirements, 5 success
  criteria. Read D-09 before evaluating criterion 1.
- `.planning/REQUIREMENTS.md` lines 30–35 — REQ-setup-detects-runtimes,
  REQ-setup-previews-by-default, REQ-setup-idempotent, REQ-setup-non-interactive,
  REQ-setup-partial-failure-legible, REQ-setup-correct-by-reading.
- `.planning/REQUIREMENTS.md` lines 39–44 — the Phase 3 REQ-register-* set, read here only to know
  what this phase must NOT do.
- `.planning/REQUIREMENTS.md` line 67 — REQ-register-cursor, deferred to v2 at scoping.

### The structural constraints that force this phase's shape
- `cmd/engram/cmdwalk.go` lines 100–128 — `operatorCommands()`'s live `server`-flag predicate.
  This is why D-01 exists; read the doc comment above it, which explains why the predicate is
  structural rather than an enumeration.
- `cmd/engram/catalog.go` — `buildCatalog`'s panic backstop for a command with no classification row.
- `cmd/engram/destructive.go` lines 30–68 — `destructiveByClassification`, its `!ReadOnly`
  admission gate (and the M9 name-debt note), and lines 70–90's `addApplyFlag` /
  `applyRequested`.
- `internal/surfaces/toolclass.go` lines 1–80 — `Class` and `Operation` semantics, including the
  conservative `Destructive` rule D-13 applies and the empty-column convention.

### Output, exit codes, and config plumbing
- `cmd/engram/client_common.go` lines 42–55 (`addClientFlags` — the `--server` this phase must not
  reuse), 219–236 (the exit-code const block D-06 extends), 249–252 (`usageErrorf`).
- `cmd/engram/catalog.go` lines 125–150 — `doc.ExitCodes` and the `exitGeneric`-is-a-backstop and
  `exitFindings`-has-no-connect-producer comments.
- `cmd/engram/catalog_test.go` lines 231 (`wantExitCodes`), 335–370
  (`nonConnectProducedCodes` + `TestCatalogExitCodesMatchMapper`'s both-directions set equality).
- `cmd/engram/testdata/catalog.golden` — gains two exit-code entries and a `setup` command entry.
- `cmd/engram/testdata/help.golden` — gains a `setup` line.
- `cmd/engram/operator_output.go` lines 16–36, 70–89 — `addOperatorOutputFlag` and
  `renderOperator`'s one-serialization-plus-a-view invariant.
- `cmd/engram/operator_view.go` lines 115–200 (`valueKind`, `viewScalar`, `viewRow`, `humanizeKey`),
  223 (`sanitizeViewValue`), 266+ (`renderOperatorView`) — the array/row rendering D-15 relies on.
- `internal/config/registry.go` lines 84–100 — `client.server_url`'s `ENGRAM_SERVER_URL` row and
  the three deliberately-Env-less entries whose written rationales D-04 weighs.
- `internal/config/client_validate.go` line 58 — `ValidateOutputFormat`, the enum-validation shape
  D-03 mirrors for `--auth`.

### Multi-value flag precedent (D-11)
- `cmd/engram/client_list.go:129,131`, `cmd/engram/client_search.go:111,115`,
  `cmd/engram/spine_review_archive.go:249,256`, `cmd/engram/spine_review_purge.go:424` —
  `StringSliceVar` usage across the binary.

### The prose path this phase's Plan() must agree with
- `skill/engram/commands/engram-setup.md` — the shipped `/engram-setup` slash command. Lines
  17–27 (URL determination, the `/mcp` mount at line 24, ADR engram-bj6, and the gateway-route
  example), 29–37 (the four auth modes D-03 adopts verbatim), 38–45 (the `claude mcp add`
  invocation table D-09's `Plan()` must reproduce), 46–52 (the
  never-put-the-secret-on-the-command-line notes D-05/D-16 implement).
- `docs/adr/engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md` — why the
  plugin ships no bundled MCP server and `/engram-setup` is the canonical wiring path.

### Milestone research (read critically — D-09 overrides one item)
- `.planning/research/ARCHITECTURE.md` §1 (lines 20–45, package layout), §2 (lines 55–120, the
  `Runtime` interface sketch and the Environment seam), §3 (the two-paths-must-agree problem and
  the `internal/setupgen` generated-block mechanism — Phase 5's, not this phase's), line 183
  (per-runtime independent Apply and `errors.Join`-style accumulation), line 191 (build order —
  **D-09 deliberately overrides item 2's empty-registry proposal**).
- `.planning/research/PITFALLS.md` — Pitfall 5 (the new CLI re-introducing the hand-editing
  problem the prose path solved), Pitfall 8 (per-runtime config-path conventions; D-12 makes this
  moot for detection), and the detection false-positive/false-negative discussion around lines
  462–487.
- `.planning/research/SUMMARY.md` § "Post-Synthesis Live Verification" — the observation that all
  three v1 runtimes have a native `mcp add` CLI, so engram parses no third-party config format.

### Prior-phase context carried forward
- `.planning/phases/01-version-homebrew-distribution/01-CONTEXT.md` — D-11's ownership boundary
  ("we do not test or red-gate what we do not own") and D-07's precedent for a deliberate, pinned
  divergence from the one-serialization-plus-a-view invariant.

### Codebase maps
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/STRUCTURE.md`,
  `.planning/codebase/TESTING.md` — repo-wide idioms this phase must match.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `registerDestructive` + `addApplyFlag` + `applyRequested` (`cmd/engram/destructive.go`): the
  entire preview-by-default contract, already generalized to `!ReadOnly` so an
  additive-but-mutating command qualifies. D-14 consumes this whole.
- `renderOperator` / `renderOperatorView` / `viewRow` (`cmd/engram/operator_output.go`,
  `operator_view.go`): a typed operator report renderer that already handles an array of objects as
  dense rows. D-15 consumes this rather than writing a renderer.
- `usageErrorf` / `cliError` / `ExitCode()` (`cmd/engram/client_common.go`): the typed exit path
  D-03 and D-11 route enum-validation failures through.
- `nonConnectProducedCodes` (`cmd/engram/catalog_test.go:348`): the existing, named mechanism for
  advertising an exit code with no connect-error producer. D-06 adds two entries; the pattern is
  already proven by `exitFindings`.
- `internal/config/registry.go` + `config.FlagDefault`: env-first flag defaulting with a
  `(default: ENGRAM_X)` help suffix, so D-04's rows make `--help` self-describing for free.
- `internal/migrate`'s `Registry` shape: the package-level-literal discipline
  `internal/setup.Runtimes` should mirror — a runtime hidden behind a lazy getter is a worse
  failure than a compile-time-visible list.

### Established Patterns
- **A command with no `internal/surfaces` row is a build-time panic, twice over** (`catalog.go`'s
  backstop and `destructiveByClassification`). This is what makes D-13 non-optional and is the
  stated reason the roadmap refuses to split this phase.
- **Structural predicates over enumerations** — `operatorCommands()` reads the live flag set rather
  than maintaining a list. Powerful, and silent when you trip it: hence D-01.
- **Typed cause, never message text** — `classifyOperatorErr` (`cmd/engram/operror.go:69`) is a
  switch of `errors.Is`/`errors.As` arms with a true passthrough default. Setup's failure
  classification should follow the same shape.
- **`errors.Join`-style accumulation** — `internal/migrate/registry.go`'s `Validate` accumulates
  multiple independent violations rather than stopping at the first. D-07's per-runtime independent
  outcomes want the same idiom.
- **Injectable seams as package vars** — `cliNow` (`cmd/engram/destructive.go:25`), `t.Cleanup`-
  restored. D-12's `Environment` is the same seam class for `exec.LookPath` and the home dir.
- **Golden-file pinning** — `testdata/catalog.golden` and `testdata/help.golden` are pinned; both
  move this phase.

### Integration Points
- `cmd/engram/root.go` — `setup` is registered on the command tree here.
- `internal/surfaces/toolclass.go`'s `operations` slice — one new row (D-13).
- `internal/config/registry.go`'s entry slice + `envToKey` — three new rows (D-04).
- `cmd/engram/client_common.go`'s exit-code const block, `catalog.go`'s `doc.ExitCodes`,
  `catalog_test.go`'s `wantExitCodes` and `nonConnectProducedCodes`, `testdata/catalog.golden` —
  all five move together in ONE commit (D-06).
- New package `internal/setup` — no existing importers; `cmd/engram/setup.go` is its only consumer
  this phase.

</code_context>

<specifics>
## Specific Ideas

- The four auth modes are to be taken **verbatim** from `skill/engram/commands/engram-setup.md`'s
  existing table (lines 38–45), not re-invented, so Phase 5's equivalence gate has a single
  vocabulary to compare.
- The `--url` help text should carry the `(default: ENGRAM_SERVER_URL)` suffix the binary's other
  URL-ish flags carry, which D-04's registry row makes available.
- Standing user principle, restated from Phase 1 D-11 and rule `m45p2b4bp7`: **do not test or
  red-gate third-party behavior.** Verify engram's own detection, planning, flag validation, exit
  codes, and report shape. Do not assert that `claude mcp add` behaves a particular way.

</specifics>

<deferred>
## Deferred Ideas

- **Warning on a suffix-less `--url`** — considered under D-02 and rejected as a heuristic that
  cries wolf on root-mounted deployments. If real user confusion appears after Phase 6 ships the
  install docs, revisit with evidence rather than by anticipation.
- **`<runtime> --version` capture during detection** — rejected under D-12 (shelling out from a
  read-only preview needs a timeout policy this phase would have to invent). It is genuine
  groundwork for Phase 3's REQ-register-cli-surface-drift-legible; take it up there, where the
  flag-surface contract is the actual subject.
- **An `orphaned-config` detection state** (config dir present, binary absent) — rejected under
  D-12 as per-runtime path knowledge acquired for a diagnostic. Worth reconsidering only if
  Phase 6's docs reveal users hitting it.
- **A separate copy-pasteable verbatim command block in the text lane** — rejected under D-15 as a
  second divergence from one-serialization-plus-a-view. Revisit only if the dense `viewRow`
  rendering proves genuinely unusable in Phase 6 documentation screenshots.
- **`--url`/`--auth`/`--runtime` accepted as a positional or config file** — not discussed as
  needed; the flag surface plus env backing covers REQ-setup-non-interactive.

</deferred>

---

*Phase: 2-Setup Command Core*
*Context gathered: 2026-08-29*
