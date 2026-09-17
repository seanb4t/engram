# Phase 3: Plugin-First Delivery - Research

**Researched:** 2026-09-14
**Domain:** Go CLI subcommand extension (`internal/setup`, `cmd/engram/setup.go`) — plugin-CLI shell-out delivery for Claude Code / Codex, layered onto the existing MCP-registration writer abstraction.
**Confidence:** HIGH for third-party CLI argv/JSON shapes (every claim below marked `[VERIFIED]` was probed live, read-only, on this machine, this session — `claude` 2.1.271, `codex-cli` 0.154.0). MEDIUM for the exact Go surface (new fields/types) the plan should introduce — CONTEXT.md deliberately leaves this to plan-time discretion, and this research surfaces a load-bearing constraint (`internal/setup`'s stdlib-only-leaf gate) that CONTEXT.md's own discretion note did not anticipate. LOW/MEDIUM for Codex's `.codex-plugin/plugin.json` schema (conflicting secondary sources; resolved via the actual referenced JSON Schema, but not against Codex's own loader — no local validator to probe against without a write verb).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Phase Boundary:** Under `--apply`, a plugin-capable Claude Code or Codex receives engram's skills, hooks, and `/engram-setup` through its own plugin CLI — engram's own marketplace/plugin only — while a runtime whose plugin CLI is absent or broken, and opencode / `generic` always, keep the native skills-copy path. Delivery is mutually exclusive per runtime per run. Plugin state is reported as one of absent / installed-but-outdated / installed-and-current, and plugin delivery is its own result facet (text + JSON). `skill/engram/.codex-plugin/plugin.json` is added, release-please-synced, with a drift gate keeping its identity fields equal to `.claude-plugin/plugin.json`. `/engram-setup`'s generated prose reflects the plugin actions in the same change.

**Live facts (03-CONTEXT.md, read-only probes, 2026-09-14, this machine):**
- Codex CLI 0.154.0 ships `codex plugin` as a released surface: `add <PLUGIN[@MARKETPLACE]> --json`, `list --json [--marketplace NAME] [--available]`, `marketplace add|list|upgrade|remove`, `remove`. There is no `update` subcommand.
- Claude Code 2.1.270: `plugin install <plugin@marketplace> --json [--config k=v]`, `plugin update`, `plugin list --json`, `plugin marketplace add <github-repo|url|path> [--scope]`.
- This machine: Claude Code has marketplace `engram` (GitHub `seanb4t/engram`) and `engram@engram` 0.16.1 installed at `user` scope; Codex has no engram marketplace; `~/.agents/skills/` holds hand-made symlinks into Claude's marketplace clone (the "managed symlink" case REQ-plugin-skips-skills-copy names).

**Carried forward (decided 2026-09-13, engram `qy29m0j3d2` — do not re-ask):**
- Plugin-first for Claude Code and Codex; `--apply` is the only consent gate — no plugin opt-in flag, no extra first-run consent.
- 3-way plugin state + facet naming are in scope; Cursor is deferred.

**Update & version semantics:**
- **D-01:** "Outdated" means the installed plugin version (from `plugin list --json`) is SemVer-less-than the binary's ldflags version. Equal → `current`. Newer than the binary → reported `current` with a note (`plugin 0.17.0 is newer than this binary 0.16.1`); never downgraded. — Reversibility: reversible.
- **D-02:** Codex has no `plugin update`: an outdated engram plugin on Codex is updated by `codex plugin remove engram` then `codex plugin add engram@engram --json` — unambiguous and independent of `add`'s idempotency. Claude Code uses its native `claude plugin update engram@engram --json`.
- **D-03:** A dev/non-SemVer binary version (`dev`, `0.16.1-dev+sha`) never triggers an update by comparison; it installs only when the plugin is absent, and the row notes `current (dev build)`. A local build must not churn a real install.

**Marketplace source & scope:**
- **D-04:** When engram's marketplace is absent, `--apply` adds GitHub `seanb4t/engram`, default branch, unpinned — `claude plugin marketplace add seanb4t/engram`; Codex's `marketplace add` with the Git URL form its `--help` accepts (live-verify the exact argument shape at implementation, read-only). No ref pinning; currency is decided by D-01's version comparison, not by a ref.
- **D-05:** If a marketplace named `engram` already exists pointing elsewhere (fork, local path), setup uses it as-is — never re-points or replaces a source the operator configured — and the plugin facet reports the observed source so a fork stays visible.
- **D-06:** Claude Code plugin scope is `user` only, matching `mcp add --scope user` and the maintainer's existing install; no `--scope` flag is exposed. Codex has no scope concept.

**Existing native skills on the plugin path:**
- **D-07:** On the plugin path, setup adds nothing to the native skills directory (REQ-plugin-skips-skills-copy).
- **D-08:** Setup never deletes a native skills path — static copies from earlier `--apply` runs and hand-made symlinks alike. If engram skills are present there, the skills facet reports it, e.g. `native: 5 skills present at ~/.agents/skills (symlink) — remove manually to avoid duplicates`, with path, count, and symlink-vs-copy. — Reversibility: reversible (a future explicit prune verb can add deletion).
- **D-09:** An existing delimited engram index block in Codex's `AGENTS.md` is left in place and reported (`index block present`); no `AGENTS.md` write happens on the plugin path.

**Capability detection & fallback:**
- **D-10:** A present binary has a working plugin CLI iff one read probe — `<cli> plugin list --json` — exits 0 and parses as JSON. The same output is the 3-way-state input, so capability + state cost one subprocess per runtime (bounded by the existing 20s `execTimeout`). Non-zero exit, timeout, or unparseable output = no working plugin CLI.
- **D-11:** Marketplace presence is probed in preview too (`<cli> plugin marketplace list`, read-only), so the argv preview shows includes `marketplace add` only when absent (SC2: exact argv beforehand). Both CLIs print a text table here — parse as a coarse name match only, never as structure (the same posture as opencode's `mcp list`).
- **D-12:** When the plugin probe fails on a present runtime, registration and the native skills copy proceed exactly as today, and the plugin facet reads `unavailable: <reason>` (e.g. `plugin list --json exited 1`, `timed out after 20s`). The row's outcome is never failed by the probe (REQ-plugin-capability-detection: "never a failed runtime row").

### Claude's Discretion

- Exact `Plan` shape for plugin delivery: a new `SkillFormatPlugin` value on `SkillTarget` vs. plugin `Action`s in `Plan.Actions` plus a plugin-facet struct — research's ARCHITECTURE.md §4 sketches both; keep per-runtime authoring in each runtime's own file and the executor content-blind.
- The plugin facet's field names on `setupRuntimeRow` (flat scalars only — `TestOperatorViewFixturesHaveNoUnsanitizedNesting`) and how the facet folds into the exit taxonomy (a `failed` plugin install beside a `wrote` registration → `exitPartial` 8, like any other failed facet).
- SemVer parsing without a new dependency (`golang.org/x/mod/semver` if already in `go.sum` and maintained upstream — rule `xvqj44e5mk` — else a small stdlib parser); how `plugin list --json`'s `version` field is located for `engram@engram` on each CLI.
- Whether `claude plugin install`/`codex plugin add` refuse when already installed — live-verify with read verbs and `--help` only; the D-01/D-02 state machine never issues an install for a `current` plugin, so refusal semantics matter only for the `absent` path.
- `.codex-plugin/plugin.json` contents beyond the identity fields (`name`, `version`, `description`), whether Codex's loader needs an `interface` block for a CLI-only install (live-verify against `codex plugin add --help` / docs, read-only), and the drift gate's shape (a Go test comparing the two manifests' identity fields + a `release-please-config.json` `extra-files` entry for the new manifest).
- Where the plugin probe results live for Phase 4's drift comparison to reuse.

### Deferred Ideas (OUT OF SCOPE)

- A future explicit `engram setup --prune-native`-style verb to remove engram-managed native copies and the `AGENTS.md` index block after plugin delivery (D-08/D-09 report-only this milestone).
- `REQ-plugin-opencode` — opencode plugin delivery if/when opencode ships a plugin CLI.
- Ref-pinned marketplace installs (`seanb4t/engram@vX.Y.Z`) — rejected for now (D-04); revisit if version skew between marketplace HEAD and released binaries becomes a support issue.
- Out of scope per 03-CONTEXT.md `<domain>`: any removal/cleanup of native skills or `AGENTS.md` blocks (report only); drift comparison (Phase 4); the apply-time preserve gate (Phase 5); Cursor.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-------------------|
| REQ-plugin-capability-detection | Detect a working `plugin` CLI distinct from binary-on-PATH; fall back to native copy with a reason, never a failed row | D-10/D-12 mechanics below; live-verified `plugin list --json` exit/parse behavior for both CLIs; "Architecture Patterns" §Recommended Plugin Flow step 2 |
| REQ-plugin-install-or-update | Under `--apply`: add marketplace when absent, install when absent, update when outdated, no-op when current; preview shows exact argv; `--apply` is the sole gate | Live-verified exact argv + `--help` flag inventory for `claude plugin install/update/marketplace add` and `codex plugin add/marketplace add`; Pitfall 1 (`-y`/non-interactive requirement); D-01/D-02/D-03 semver design in "Don't Hand-Roll" and Code Examples |
| REQ-plugin-three-way-state | absent / installed-but-outdated / installed-and-current, comparing `plugin list` output against the binary's own version | D-01 comparator design (stdlib-only, see Pitfall 3); live JSON shapes for `claude plugin list --json` and `codex plugin list --json` |
| REQ-plugin-skips-skills-copy | Plugin delivery and native skills copy mutually exclusive per runtime per run; never delete a skills path without checking for a managed symlink | `internal/skills.Install`'s existing `FormatNone` no-op precedent; D-08's new `Lstat` seam requirement (Pitfall 4); "Architecture Patterns" §SkillFormatPlugin |
| REQ-plugin-facet-reported | Plugin delivery is its own result facet (text + JSON), exit taxonomy accounts for it | `setupRuntimeRow`'s existing flat-scalar facet precedent (Skills/Headers); `AggregateOutcome`/`Classify` exhaustive-switch touch points (Pitfall 6) |
| REQ-codex-plugin-manifest | `.codex-plugin/plugin.json` exists, release-please-synced, drift gate vs `.claude-plugin/plugin.json` | Live-fetched JSON Schema (agent-plugins.org) confirming minimal required fields; release-please-config.json extra-files precedent; Code Examples §Manifest |
| REQ-plugin-setupgen-regenerated | `/engram-setup` prose reflects plugin actions in the same change; CI drift gate stays green | `internal/setupgen/setupgen.go`'s existing `adds`-filter mechanism (only renders claude-code's `mcp add`); Pitfall 7 |

</phase_requirements>

## Summary

Plugin-first delivery is NOT a drop-in extension of the existing `Plan.Actions`/`Plan.Probe`/`execute()` shape the way ARCHITECTURE.md's 2026-09-13 pass originally sketched. That pass predates this session's locked decisions (D-01, D-10–D-12), which require a *content-aware* classification (parse `plugin list --json`, compare a SemVer string against the binary's own version) before any action is chosen — the shared executor's existing convergence model is a blind pre/post BYTE-COMPARE (`apply.go`'s `execute()`, steps 4–9) that has no way to decide "outdated" from two opaque reads. Plugin delivery therefore needs its own read-then-decide-then-act sequence, structurally different from the registration Probe's semantics, even though it can (and should) reuse the same bounded `Environment.Run`/`runSeam` execution primitive.

A second, more consequential finding: `internal/setup` is machine-gated as a **stdlib-only leaf package** (`TestSetupPackageIsStdlibOnlyLeaf`, `internal/setup/leafpurity_test.go:83-90` — quoted verbatim below). This forecloses CONTEXT.md's own suggested `golang.org/x/mod/semver` if the comparison lives inside `internal/setup`, which is where it structurally belongs (alongside the `AUTHORED-HERE` per-runtime argv and the single "one place a process runs" invariant `apply.go`'s package doc states). The recommended resolution is a small, local, stdlib-only SemVer-core comparator inside `internal/setup` — this exact technique (anchored regex + `strconv.ParseUint` on major/minor/patch, prerelease/build-suffix detection) is already precedented in this very repo, in `cmd/engram/buildversion.go`'s `patchCorePattern`/`nextPatch`, solving an adjacent problem.

Every third-party CLI fact below was probed live, read-only, this session, against `claude` 2.1.271 and `codex-cli` 0.154.0: `claude plugin list --json`'s exact array-of-objects shape (confirmed `id` = `"engram@engram"`, `version`, `scope`); `codex plugin list --json`'s `{"installed":[...],"available":[...]}` shape (confirmed the bare `--json` call, with no `--available` flag, already answers the INSTALLED-state question D-01 needs); the `--help` inventories for every install/update/marketplace-add/remove subcommand on both CLIs; and — not previously flagged in CONTEXT.md — that `claude plugin install`/`update` both REQUIRE `-y`/`--yes` when stdin/stdout is not a TTY, which is unconditionally true for `engram setup --apply`. Omitting `-y` will hang or fail every scripted Claude Code plugin install/update.

**Primary recommendation:** Give plugin delivery its own decision-and-execution path, parallel to (not folded into) `setup.Preview`/`setup.Apply`'s registration flow — a new optional `Runtime`-adjacent interface implemented only by claude-code and codex (mirroring the existing `optInOnlyRuntime` pattern), a single `plugin list --json` probe that answers both capability and state, a stdlib-only version comparator living inside `internal/setup`, and a facet composed into `setupRuntimeRow` the same additive way the Skills facet already is.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Plugin capability probe (`plugin list --json`) | `internal/setup` (per-runtime file) | — | AUTHORED-HERE invariant; same file already owns registration Probe/Detect |
| Plugin version comparison (SemVer, D-01/D-03) | `internal/setup` (new, stdlib-only helper) | — | Must stay inside the stdlib-only leaf per `leafpurity_test.go`; cannot move to `cmd/engram` without breaking "single execution owner" / AUTHORED-HERE |
| Plugin action execution (marketplace add / install / update / remove+add) | `internal/setup` | — | `apply.go`'s package doc: shared `execute()`-family is "the ONLY place any process gets run" |
| Plugin facet composition onto the report row | `cmd/engram/setup.go` | — | Mirrors `setupApplySkillsFacet`'s existing additive-facet composition pattern |
| Native-skills / AGENTS.md presence report (D-08/D-09) | `internal/skills` | `cmd/engram/setup.go` (surfacing into the row) | `internal/skills` already owns skill names, target dirs, and the AGENTS.md block markers |
| `.codex-plugin/plugin.json` manifest + drift gate | `skill/engram/.codex-plugin/` (data) + `internal/setupgen` or a sibling small package (gate) | `release-please-config.json` (sync) | Mirrors the existing `.claude-plugin/plugin.json` release-please sync entry |
| `/engram-setup` prose regeneration | `internal/setupgen` | — | Already the sole generator/CI-gate for this file |

## Standard Stack

### Core

No new third-party library. Plugin management is 100% shell-out to `claude`/`codex`'s own CLIs, exactly like MCP registration today.

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|---------------|
| `encoding/json` (stdlib) | Go stdlib | Parse `claude plugin list --json` / `codex plugin list --json` output | Already used in `internal/setup/generic.go`; zero new import |
| `regexp` + `strconv` (stdlib) | Go stdlib | A small, local SemVer-core comparator (see "Don't Hand-Roll") | Already the exact technique `cmd/engram/buildversion.go` uses for an adjacent version-derivation problem |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/mod/semver` | v0.40.0 (already in `go.sum`, `// indirect` — `[VERIFIED: go.mod:154]`) | Full RFC-compliant SemVer compare | **Only** if the version comparison is deliberately relocated OUT of `internal/setup` into `cmd/engram` (which is not a stdlib-only leaf) — see Pitfall 3 for why this is NOT the recommended default |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Local stdlib-only SemVer comparator in `internal/setup` | `golang.org/x/mod/semver` in `cmd/engram` | The x/mod route requires moving the version DECISION out of the runtime's own `internal/setup` file, in tension with the AUTHORED-HERE invariant and forcing either a second `Runtime`-adjacent decision seam in `cmd/engram` or exporting the argv-building logic across the package boundary. The local-comparator route keeps everything in one file, at the cost of a ~30-line hand-rolled parser (already precedented in this repo) |
| Folding plugin actions into `Plan.Actions`/`Plan.Probe` (ARCHITECTURE.md §1's original sketch) | A parallel plugin-only decision+execution path | The blind pre/post byte-compare `execute()` uses for registration cannot express "compare a parsed version field," so this sketch (written before D-01/D-10–D-12 were locked) undersells the complexity — see Summary |

**Installation:**

No `go get`/`npm install` — this phase's only `go.mod` change (if the x/mod route is chosen instead of the recommended local comparator) would be promoting an already-indirect dependency to direct, metadata-only:

```bash
go mod tidy   # promotes golang.org/x/mod from `// indirect` to direct, IF cmd/engram imports x/mod/semver
```

**Version verification:** `golang.org/x/mod v0.40.0` confirmed present via `[VERIFIED: go.mod:154, go.sum:325-326]` (`grep -n "x/mod" go.mod go.sum`, this session) and `[VERIFIED: go doc golang.org/x/mod/semver, local module cache]` confirming the exact exported API: `Build`, `Canonical`, `Compare`, `IsValid`, `Major`, `MajorMinor`, `Max`, `Prerelease`, `Sort`. This is the maintained upstream Go extended-stdlib module (rule `xvqj44e5mk` satisfied if used) — but see Pitfall 3 for why it cannot be imported from `internal/setup` itself.

## Package Legitimacy Audit

No new external package is installed by this phase. `golang.org/x/mod` is already present in `go.sum` as an indirect dependency of the existing toolchain (confirmed `[VERIFIED: go.mod:154]`); it is not being newly added, only (optionally) promoted to a direct import if the planner chooses the x/mod route in `cmd/engram` over the recommended local comparator. `golang.org/x/mod` is an official `golang.org/x/*` extended-standard-library module — no registry legitimacy check is warranted for an official Go org module already vendored in this repository's own dependency graph.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|--------------|---------|-------------|
| `golang.org/x/mod` | Go modules | Long-established (Go org extended stdlib) | N/A (Go org module) | `github.com/golang/mod` | OK (not newly installed — already indirect) | No action required; optional promotion to direct only if planner chooses the x/mod route |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
                         engram setup [--apply]
                                 │
                                 ▼
                    setupResolve (cmd/engram/setup.go)
                                 │
                for each selected Runtime (claude-code / codex / opencode / generic)
                                 │
                                 ▼
                       rt.Detect(env)  ──false──► Result{OutcomeNotPresent}
                                 │ true
                                 ▼
              ┌──────────────────────────────────────────────┐
              │      REGISTRATION LANE (unchanged, existing)  │
              │  rt.Plan(env,opts) → Plan{Actions,Probe,...}  │
              │  execute(): probe#1 → write actions → probe#2 │
              │  byte-compare → wrote / already-correct       │
              └──────────────────────────────────────────────┘
                                 │
                                 ▼  (NEW, this phase — parallel, not nested)
              ┌──────────────────────────────────────────────┐
              │      PLUGIN LANE (claude-code / codex only)   │
              │  1. type-assert rt as a plugin-capable        │
              │     Runtime — opencode/generic never do       │
              │  2. run ONE probe: `<cli> plugin list --json` │
              │     exit!=0 or unparseable → capability=false │
              │     → facet "unavailable: <reason>", STOP     │
              │  3. capable → find "engram" entry, parse       │
              │     installed version (stdlib json)           │
              │  4. compare installed vs resolvedVersion()      │
              │     (stdlib SemVer-core comparator, D-01/D-03)  │
              │     → absent / outdated / current(+note)       │
              │  5. preview-time only: run marketplace-list     │
              │     probe (coarse text match, D-11) to decide   │
              │     whether "marketplace add" appears in argv   │
              │  6. mutate==true: run the decided action        │
              │     sequence (marketplace add?; install /       │
              │     update-via-remove-then-add / nothing)        │
              │     through the SAME bounded runSeam            │
              └──────────────────────────────────────────────┘
                                 │
                                 ▼
              SkillFormat routing (cmd/engram/setup.go, D-07):
              plugin lane succeeded (absent→installed,
              outdated→updated, or current) → SkillFormatPlugin
              (no native write, no AGENTS.md write) ; plugin lane
              unavailable/not-applicable → SkillFormatNative /
              SkillFormatAgentsMD exactly as today
                                 │
                                 ▼
                setupRuntimeRow: Registration facet + Plugin
                facet + Skills facet → AggregateOutcome → row
```

### Recommended Project Structure

No new top-level directories. New files land inside the existing `internal/setup` package (mirroring `claudecode.go`/`codex.go`'s one-file-per-runtime discipline) and one new manifest file:

```
internal/setup/
├── plugin.go              # NEW: shared plugin-lane decision+execution (stdlib-only)
├── pluginversion.go        # NEW: local SemVer-core comparator (D-01/D-03) — or fold into plugin.go
├── claudecode.go           # MODIFIED: implements the plugin-capable interface
├── codex.go                # MODIFIED: implements the plugin-capable interface
skill/engram/
├── .claude-plugin/plugin.json   # UNCHANGED (identity source of truth)
├── .codex-plugin/plugin.json    # NEW
cmd/engram/
├── setup.go                 # MODIFIED: compose plugin facet, SkillFormatPlugin routing, row fields
internal/setupgen/
├── setupgen.go               # MODIFIED: render plugin actions in generated prose
release-please-config.json    # MODIFIED: extra-files entry for .codex-plugin/plugin.json
```

### Pattern 1: Optional-interface capability gate (mirrors `optInOnlyRuntime`)

**What:** A new, small, optional interface that only claude-code and codex implement, type-asserted once by the plugin lane's entry point — never a by-name (`rt.Name() == "claude-code"`) branch anywhere.

**When to use:** Any time a capability applies to a strict subset of registered `Runtime`s and the executor/composer must stay content-blind about WHICH runtimes those are.

**Example (existing precedent, `internal/setup/runtime.go`):**
```go
// Source: internal/setup/runtime.go:113-121 (read this session)
type optInOnlyRuntime interface {
    OptInOnly() bool
}
```
The plugin-capability interface should follow the exact same shape: a method (or small set of methods) the plugin lane's shared function type-asserts for, with claude-code/codex implementing it and opencode/generic never doing so.

### Pattern 2: Facet composition happens in `cmd/engram`, not inside the shared executor

**What:** `setupApplySkillsFacet` (`cmd/engram/setup.go:132-179`, read this session) already demonstrates the pattern: it takes a `registrationOutcome`, independently computes a second facet's outcome, folds them via `setup.AggregateOutcome`, and returns ONE outcome for the row — while writing the facet's own fields (`row.Skills`, `row.SkillsDest`, ...) directly onto the row struct.

**When to use:** The plugin facet should compose the SAME way — a new `setupApplyPluginFacet(row *setupRuntimeRow, registrationOutcome setup.Outcome, rt setup.Runtime, opts setup.Options, mutate bool) setup.Outcome` (or equivalent), called from `setupRuntimeRowFromResult` immediately alongside the existing skills-facet call, feeding its own outcome into the SAME `AggregateOutcome` chain (now three inputs, or two sequential two-input folds — `AggregateOutcome` is associative/commutative per its own doc comment, so `AggregateOutcome(AggregateOutcome(reg, plugin), skills)` is safe).

### Pattern 3: `Plan.Skills` decision depends on the PLUGIN LANE'S OUTCOME, not just the runtime's identity

**What:** Today, `claudeCodeRuntime.Plan()`/`codexRuntime.Plan()` unconditionally author `SkillFormatNative`/`SkillFormatAgentsMD` (`claudecode.go:143-146`, `codex.go:114-119`, both read this session). D-07 requires the OPPOSITE format (`SkillFormatPlugin`, a no-op for `skills.Install`) exactly when the plugin lane's own outcome for THIS run is absent→installed, outdated→updated, or already current — i.e., a decision that is only knowable AFTER the plugin probe has run, which happens AFTER `Plan()` returns in the current architecture.

**Why this matters:** `Plan()` is documented as pure/no-exec (`plan.go`'s package doc: "authors the exact invocation... without executing it"). The plugin capability probe is inherently an exec. This is a genuine sequencing tension the planner must resolve — see Open Questions.

**Two candidate resolutions (present both to planner, do not pre-select):**
1. Run the plugin-lane probe FIRST (before calling `rt.Plan()`), and pass its result into `Plan()` as a new field on `Options` (e.g. `Options.PluginDelivered bool`) — keeps `Plan()` pure in spirit (it still does no exec itself; the exec happened one call earlier, at the composition site in `cmd/engram`), and lets `Plan()`'s existing `switch opts.Auth` structure grow one more input without becoming impure.
2. Keep `Plan()` unconditionally authoring the NATIVE `SkillTarget`, and have `cmd/engram`'s row composer OVERRIDE the `SkillFormat` it acts on (never mutating `Plan.Skills` itself, but choosing which target `setupApplySkillsFacet` is invoked with) based on the plugin lane's already-computed outcome. This keeps `Plan()` fully name-symmetric with today's code (zero changes to its own signature) at the cost of a conditional living in `cmd/engram` that inspects a runtime's own delivery mode from outside that runtime's file — a mild AUTHORED-HERE tension, but confined to routing, not to any string/argv content.

### Anti-Patterns to Avoid

- **Reusing the registration Probe's pre/post byte-compare for plugin state:** D-01's "outdated" classification is a SEMANTIC comparison (parsed version fields), not a byte-identity check. Folding it into `execute()`'s existing `probe1==probe2` logic (`apply.go:355-367`) would either silently misclassify every plugin state as `wrote` (ambiguity-resolves-to-wrote, D-08's OWN registration invariant, is the WRONG default here — a `current` plugin authoring zero actions must classify as `already-correct`, not `wrote`) or require special-casing the executor by content, which the package's own doc comments explicitly forbid ("D-11 reports rather than diagnoses", `apply.go`).
- **Passing `--json` to `claude plugin install`/`update` without `-y`:** confirmed live this session (`claude plugin install --help`/`update --help`) that `-y, --yes` is required "when stdin or stdout is not a TTY" — true for every `engram setup --apply` invocation. Omitting it will hang interactively-invoked runs and fail non-interactive ones.
- **Importing `golang.org/x/mod/semver` inside any `internal/setup/*.go` non-test file:** `TestSetupPackageIsStdlibOnlyLeaf` fails the build the moment a non-stdlib import (any first path segment containing a `.`) appears there. See Pitfall 3.
- **Trusting a vendor CLI's own idempotency instead of engram's own comparison:** `code.claude.com/docs/en/plugins-reference` (fetched this session) does NOT document whether `plugin install`/`update` refuse or silently no-op on an already-current plugin. D-01's own state machine already resolves this correctly by NEVER issuing an install/update call for a `current` plugin — do not additionally rely on the CLI's own behavior as a safety net; it is unverified.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Full RFC SemVer comparison with prerelease-precedence rules | A general-purpose SemVer library from scratch | `golang.org/x/mod/semver` — ONLY if the comparison is deliberately placed in `cmd/engram` (not `internal/setup`) | It is already vendored, maintained by the Go team, and exposes exactly `Compare`/`IsValid`/`Prerelease` |
| Detecting "this is a dev/non-release build" | A brand-new ad-hoc string check | The SAME anchored-regex + prerelease/build-suffix technique `cmd/engram/buildversion.go`'s `patchCorePattern`/`nextPatch` already uses for an adjacent problem | Proven, already reviewed, and avoids introducing a second convention for "what does a bare release core look like" in the same binary |
| Parsing `codex mcp add`/`plugin add`-shaped human text output | A generic table/box-drawing parser | `--json` on every command this phase needs (`plugin list --json`, `plugin add --json`, `plugin marketplace add --json`) — confirmed present on EVERY codex plugin subcommand this session | Codex's plugin surface is fully JSON-capable, unlike its `mcp list` sibling opencode-style pitfall; there is no reason to parse text here at all except the coarse marketplace-name match D-11 explicitly scopes to text (`marketplace list` has no `--json`) |

**Key insight:** Every piece of state this phase needs (installed version, marketplace presence, install/update/remove result) is available as structured JSON from at least one of the two CLIs' own `--json` flags — the ONE deliberate exception is marketplace-name presence-checking (`marketplace list` is text-only on both CLIs), which D-11 already scopes to a coarse, never-structural comparison. Do not build a general parser for anything wider than that one case.

## Runtime State Inventory

This phase is not a rename/rebrand, but D-08/D-09 require explicitly enumerating pre-existing runtime state that the plugin-delivery routing decision must NOT disturb — the same "what does a source-tree-wide change leave behind at runtime" question a migration phase asks, scoped here to "what does SWITCHING a runtime from native-copy to plugin-delivery leave behind."

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — this phase touches no datastore (Qdrant, SQLite, etc.); plugin/marketplace state lives entirely in the third-party CLI's own on-disk cache (`~/.claude/plugins/...`, `~/.codex/...`), which engram never reads directly, only through `plugin list --json`/`marketplace list` | None |
| Live service config | Claude Code's own marketplace/plugin cache (`~/.claude/plugins/marketplaces/engram`, `~/.claude/plugins/cache/engram/engram`, confirmed present on this machine via `claude plugin list --json`/`marketplace list`, this session) is entirely managed BY Claude Code itself — engram never writes there directly, only shells out to `claude plugin ...` | None from engram's side; report-only via the plugin facet |
| OS-registered state | None | None |
| Secrets/env vars | None — plugin delivery carries no credential of any kind; `ENGRAM_TOKEN`/header env-var references are unaffected (registration lane, unchanged) | None |
| Build artifacts | A pre-existing plain native skills install (`~/.claude/skills/curating-memory/...` or `~/.agents/skills/...`) left by an EARLIER `--apply` run, now stale relative to a machine that has since become plugin-capable — confirmed this is a REAL possibility, not hypothetical: this machine's own `~/.agents/skills/` already holds "hand-made symlinks into Claude's marketplace clone" per 03-CONTEXT.md's Live facts | D-08: report presence (path, count, symlink-vs-copy) via the skills facet; never delete automatically. Requires a NEW `Lstat` seam on `internal/skills.Environment` (see Pitfall 4) since no existing seam can distinguish a symlink from a regular file/dir |

**Nothing found in category "Stored data," "OS-registered state," "Secrets/env vars":** confirmed by re-reading `internal/setup/environment.go`, `internal/skills/environment.go`, and `runtime.go`/`Options` this session — no datastore, OS-registration, or secret-bearing field exists anywhere in the plugin-delivery surface; every read/write this phase adds is either a subprocess exec (bounded, reported) or a report-only filesystem stat.

## Common Pitfalls

### Pitfall 1: `claude plugin install`/`update` require `-y`/`--yes` for non-interactive use — confirmed live, not previously named in CONTEXT.md's Live facts

**What goes wrong:** `engram setup --apply` runs non-interactively by construction (no stdin prompt loop anywhere in `cmd/engram`). `claude plugin install --help` and `claude plugin update --help`, both fetched live this session, state verbatim: `-y, --yes  Accept the displayed marketplace-declared command without the confirmation prompt ... (required when stdin or stdout is not a TTY)`. Without `-y`, a scripted `--apply` run against an absent/outdated engram plugin will either hang (if Claude Code blocks on a prompt with no TTY) or exit nonzero with a confirmation-required error — either way, a silent regression relative to every OTHER write this package performs (which are all single, pre-authored, non-interactive argv invocations).

**Why it happens:** Anthropic's plugin CLI treats a marketplace-declared install/update command as something a human should see and confirm at least once, because (per Anthropic's own docs, cited in PITFALLS.md) a plugin "can execute arbitrary code on your machine." This consent model was designed for interactive use; `engram setup --apply` is the second, non-interactive consumer this session confirms needs its own explicit `-y`.

**How to avoid:** Author `-y`/`--yes` (and `--json`) onto every `claude plugin install`/`claude plugin update` action this phase's `claudecode.go` authors. This is a SAFE default specifically because the SOURCE is engram's own marketplace/plugin (never a foreign one, D-04/anti-features) — the same reasoning `--apply` already uses as its own consent gate for MCP registration applies here without a SEPARATE flag, per the milestone's locked decision.

**Warning signs:** A test that scripts `claude plugin install` via a fake `Environment.Run` without asserting `-y`/`--yes` appears in the authored `Args` would pass even though the real invocation would hang — assert the exact `Action.Args` slice, not just that SOME action ran.

**Phase to address:** This phase (Task authoring `claudecode.go`'s plugin actions).

### Pitfall 2: `codex plugin list --json` (no `--available`) already answers the INSTALLED-state question — do not reach for `--available`

**What goes wrong:** CONTEXT.md's own open question asks whether `codex plugin list --json` reports "available from marketplaces" or "installed" plugins, and whether `--available` is needed. Live-verified this session: bare `codex plugin list --json` (no flags) returns `{"installed": [...], "available": []}` — BOTH keys are always present; `--available` only POPULATES the `available` array with uninstalled marketplace plugins. The `installed` array is populated on the BARE call. A naive reading of the CLI's flag name (`--available`) could lead an implementer to add it unnecessarily, or worse, to key the D-01 comparison off the WRONG array.

**Why it happens:** The flag's name ("available") reads as if it toggles between two views, when it actually only ADDS a second view alongside the always-present first one.

**How to avoid:** Author the codex plugin-list probe as `codex plugin list --json` (no `--available`), and locate the "engram" entry inside the `installed` array by matching `name == "engram" && marketplaceName == "engram"`. Never add `--available` unless a future feature genuinely needs the "known but not installed" set.

**Warning signs:** A probe scripted with `--available` in a test/fixture that nobody can explain the need for.

**Phase to address:** This phase (probe authoring in `codex.go`).

### Pitfall 3: `internal/setup` is a stdlib-only leaf package — `golang.org/x/mod/semver` cannot be imported there

**What goes wrong:** CONTEXT.md's "Claude's Discretion" section explicitly suggests `golang.org/x/mod/semver` "if already in `go.sum` and maintained upstream." It IS already in `go.sum` (confirmed `[VERIFIED: go.mod:154]`). But `internal/setup/leafpurity_test.go:83-90` (read verbatim this session) asserts:

```go
// Source: internal/setup/leafpurity_test.go:83-90 (read this session, verbatim)
firstSeg, _, _ := strings.Cut(importPath, "/")
if strings.Contains(firstSeg, ".") {
    nonStdlib = append(nonStdlib, offender{path, importPath})
}
...
if len(nonStdlib) > 0 {
    t.Fatalf("internal/setup imports non-stdlib package(s): %+v — full collected import set: %v", nonStdlib, allImports)
}
```

`golang.org/x/mod/semver`'s first import-path segment is `golang.org`, which contains a `.` — this test would fail the build the instant any `internal/setup/*.go` non-test file imports it. Since the D-01 comparison structurally belongs inside `internal/setup` (alongside the runtime's own AUTHORED-HERE argv and the package's "one place a process runs" invariant), this is a real, machine-enforced blocker to the naive reading of CONTEXT.md's discretion note.

**Why it happens:** The discretion note was written without re-checking `leafpurity_test.go` against the SPECIFIC package the comparison would land in; the test exists to prevent exactly this kind of "just import the convenient library" drift into a package the project has deliberately kept dependency-free.

**How to avoid:** Write a small, local, stdlib-only SemVer-core comparator inside `internal/setup` (a new `pluginversion.go` or similar), following the SAME anchored-regex + `strconv` technique `cmd/engram/buildversion.go`'s `patchCorePattern`/`nextPatch` already uses for the closely related "is this a plain release core" question. `x/mod/semver`'s own exported API (`IsValid`, `Compare`, `Prerelease`, confirmed via `go doc golang.org/x/mod/semver` this session) is a useful DESIGN REFERENCE for what the local comparator should expose, even though it cannot be imported directly from this package. If the planner instead prefers using `x/mod/semver` verbatim, the version DECISION (not just the comparator call) must be relocated to `cmd/engram`, which is NOT a stdlib-only leaf — but this then requires exporting either the plugin action-building logic or the probe-parsing logic across the package boundary, in tension with AUTHORED-HERE.

**Warning signs:** `go build ./...` or `task test` failing with `internal/setup imports non-stdlib package(s)` naming `golang.org/x/mod/semver`.

**Phase to address:** This phase, at the design step before any code is written (this determines which package the whole plugin-lane decision function lives in).

### Pitfall 4: D-08's symlink-vs-copy detection needs a `Lstat` seam that does not exist on `internal/skills.Environment` today

**What goes wrong:** `internal/skills.Environment` (`internal/skills/environment.go:22-44`, read this session) exposes exactly three methods: `ReadFile`, `WriteFile`, `MkdirAll`. None of them can distinguish a symlink from a regular file/directory. D-08 requires the skills facet to report `(symlink)` vs. a plain copy for a pre-existing native skills install — this is exactly the scenario 03-CONTEXT.md's own Live facts section confirms is REAL on this machine (`~/.agents/skills/` holds hand-made symlinks). Without a stat-capable seam, this report cannot be built at all, honestly, from a testable fake — only from an untestable direct `os.Lstat` call bypassing the Environment abstraction entirely (which would also break rule `m45p2b4bp7`'s "no test touches the real machine" posture, since a test scripting THIS check would need a real filesystem).

**Why it happens:** `internal/skills.Environment`'s three methods were sized exactly for `Install`'s own needs (read-compare-write, create-index) at the time it was designed (04-01 through 04-04) — nothing in that design anticipated a REPORT-ONLY presence check that needs to distinguish link types without writing anything.

**How to avoid:** Add a new field to `internal/skills.Environment`, e.g. `Lstat func(name string) (os.FileInfo, error)` (mirroring `os.Lstat`'s own signature), with `OSEnvironment.Lstat = os.Lstat` for production and a fake `Lstat` closure in test fixtures (`fakeSkillsEnv`, `cmd/engram/setup_test.go`). This is a purely additive struct-field change — zero impact on `Install`'s existing behavior, since `Install` itself never needs to call it.

**Warning signs:** A D-08 test that only checks "the report mentions a count," never distinguishing symlink from copy — the CONTEXT.md-specified example text (`native: 5 skills present at ~/.agents/skills (symlink) — remove manually to avoid duplicates`) explicitly names the symlink case, so a test that never exercises it is under-covering the requirement.

**Phase to address:** This phase (the D-08 report-only task).

### Pitfall 5: A Claude Code marketplace pointing to a fork/local path must be used AS-IS (D-05) — the coarse `marketplace list` parse must extract the SOURCE, not just presence

**What goes wrong:** `claude plugin marketplace list`'s text output (captured live this session) shows, per marketplace: a name line (`❯ engram`) followed by an indented `Source: GitHub (seanb4t/engram)` or `Source: Directory (/path)` line. D-05 requires the plugin facet to report the OBSERVED source (so a fork stays visible) — a coarse parse that only checks "is a marketplace named `engram` present" (a boolean) loses this. The facet's `Reason`/notes must carry the source line, not just a boolean.

**Why it happens:** D-11's "coarse name match only, never as structure" instruction could be misread as "only check presence," when the actual requirement (D-05) needs one MORE piece of information (the source) from the same text block, still without treating the WHOLE table as structured data.

**How to avoid:** Parse `marketplace list`'s output as: find the line matching `❯ engram` (or your normalized whitespace-trimmed form), then take the immediately-following `Source: ...` line verbatim as a string — two adjacent lines, matched by position relative to the name line, never a full table model. Codex's `marketplace list` output is a simple two-column table (`MARKETPLACE` / `ROOT`, confirmed live this session) — even simpler: split on whitespace, first column is the name, second is the root path, used directly as the "observed source" string.

**Warning signs:** A `--output json` fixture where the plugin facet reports `marketplace: present` with no source string at all — D-05's own success criterion requires the source be visible.

**Phase to address:** This phase (D-05/D-11 implementation).

### Pitfall 6: Adding a plugin facet touches THREE exhaustive-switch sites, not just `setupRuntimeRow`

**What goes wrong:** `setup.AggregateOutcome`'s `precedenceOrder` (`internal/setup/aggregate.go:8-16`, read this session) and `isRecognizedOutcome` are already exhaustive over the FIVE existing `Outcome` values — folding a plugin facet's outcome through `AggregateOutcome` is safe as long as the plugin facet only ever PRODUCES one of those five existing values (never a new sixth value). If the plugin lane's "unavailable" state (D-12) is modeled as a NEW `Outcome` constant rather than reusing an existing one, it must be added to `precedenceOrder`, `isRecognizedOutcome`, AND `Classify`'s exhaustive switch (`exit.go:48-74`) — three sites, in one commit, exactly as ARCHITECTURE.md's §4 (drift detection) already documented for its own hypothetical `OutcomePreserved`.

**Why it happens:** It is easy to add a new facet's field to `setupRuntimeRow` (purely additive, `omitempty`-tagged) while forgetting that the facet's OUTCOME (as opposed to its display string) must still resolve to one of the vocabulary `AggregateOutcome`/`Classify` already know about, or extend that vocabulary correctly everywhere at once.

**How to avoid:** Prefer modeling `unavailable: <reason>` as a STRING on the plugin facet field (like `Reason`/`Notes` are strings today) rather than as a new `Outcome` value, and let the plugin facet's AGGREGATE-able outcome be one of the five existing values (`OutcomeWouldWrite` for "no probe was possible, preview only," `OutcomeAlreadyCorrect` for "capability check failed, native path proceeds unaffected — this facet contributes nothing that should ever fail the row," per D-12's "never a failed row" requirement). If a genuinely new value is needed, budget it as its own explicit sub-task touching all three sites, exactly like the drift-detection research already scoped for `OutcomePreserved`.

**Warning signs:** A build failure in `exit_test.go`'s `TestClassifyExhaustiveOutcomeCombinations` after adding a plugin facet — this is the mechanical proof the exhaustive switch was NOT updated.

**Phase to address:** This phase (facet outcome design, before writing `setupApplyPluginFacet`-equivalent code).

### Pitfall 7: `setupgen.Cases()`/`Render()` only ever renders CLAUDE-CODE's Plan — plugin actions on Codex are invisible to the generated prose gate by construction

**What goes wrong:** `internal/setupgen/setupgen.go`'s `Check`/`Write` (lines 165-205, read this session) hardcode `setup.ClaudeCode.Plan` as the ONLY `PlanFunc` ever rendered — Codex's Plan is never fed through `Render` at all, for ANY feature, today. This means REQ-plugin-setupgen-regenerated's "reflects plugin actions in the same change" is automatically satisfied for Codex (there is nothing to regenerate there) but NOT automatically satisfied for Claude Code — the existing `adds` filter (matching `{"claude","mcp","add",...}`, lines 108-113) will keep finding exactly one such action even after plugin actions are appended to the SAME `Plan.Actions` slice (their `Args[0..2]` will be `{"claude","plugin",...}`, not `{"claude","mcp","add"}`), so the EXISTING gate will NOT fail mechanically — it will simply keep rendering a table that is now silently incomplete relative to what `--apply` actually does.

**Why it happens:** The gate's exactly-one-match filter is scoped to `mcp add` specifically; it has no assertion that would catch "a NEW kind of action exists in `Plan.Actions` that this renderer has never heard of."

**How to avoid:** Add a second filter loop in `Render` matching `{"claude","plugin",...}`-shaped actions, assert an appropriate count invariant (per however many plugin actions `claudecode.go`'s `Plan()` ends up authoring for the synthetic `Cases()` — note: `Cases()`'s four/five synthetic cases all use `HomeDir`-only synthetic environments with NO real plugin state, so `claudecode.go`'s `Plan()` cannot itself decide absent/outdated/current inside `Plan()` — see Open Questions on whether `Plan()` authors plugin actions at all, or whether they are entirely OUTSIDE `Plan.Actions` and therefore invisible to `setupgen` by design, which would make this pitfall moot for the plugin-action rendering half but would still require a DELIBERATE decision, documented, about whether `/engram-setup`'s prose says anything about plugin delivery at all).

**Warning signs:** `task surfaces:gen`/`--check-setup` staying green after `claudecode.go` grows plugin actions, with no new table appearing in `skill/engram/commands/engram-setup.md` — a silent gap, not a caught defect.

**Phase to address:** This phase, but genuinely depends on resolving the Open Question below (does `Plan()` author plugin actions as literal `Plan.Actions` entries, or does the plugin lane live entirely outside `Plan()`'s return value).

## Code Examples

### Live-verified `claude plugin list --json` shape (this session, read-only, `claude` 2.1.271)

```json
[
  {
    "id": "engram@engram",
    "version": "0.16.1",
    "scope": "user",
    "enabled": true,
    "installPath": "/Users/sean/.claude/plugins/cache/engram/engram/0.16.1",
    "installedAt": "2026-06-03T16:04:13.154Z",
    "lastUpdated": "2026-09-14T14:34:56.927Z"
  }
]
```
`id` is `"<plugin-name>@<marketplace-name>"` — match `id == "engram@engram"` (or split on the LAST `@` and compare both segments) rather than assuming any particular array position. `mcpServers` is present on SOME entries (omitted here) and absent on others — never assume its presence.

### Live-verified `codex plugin list --json` shape (this session, read-only, `codex-cli` 0.154.0)

```json
{
  "installed": [
    {
      "pluginId": "engram@engram",
      "name": "engram",
      "marketplaceName": "engram",
      "version": "0.16.1",
      "installed": true,
      "enabled": true,
      "source": {"source": "local", "path": "..."},
      "installPolicy": "AVAILABLE",
      "authPolicy": "ON_INSTALL"
    }
  ],
  "available": []
}
```
(Shape confirmed from this machine's real output for OTHER plugins — this machine has no engram entry on Codex, per 03-CONTEXT.md's Live facts; the `engram` row above is illustrative of the confirmed field names, not a literal capture.) The bare `--json` flag already includes BOTH `installed` and `available` keys (`available` empty unless `--available` is also passed) — locate the engram entry in `installed` by matching `name == "engram" && marketplaceName == "engram"`.

### Live-verified `--help` inventories (this session, read-only)

```
$ claude plugin install --help
Usage: claude plugin install|i [options] <plugin>
  --accept-command <sha256>
  --config <key=value>
  --json
  -s, --scope <scope>        (default: "user")
  -y, --yes                  Required when stdin or stdout is not a TTY

$ claude plugin update --help
Usage: claude plugin update [options] <plugin>
  --accept-command <sha256>
  --json
  -s, --scope <scope>        (default: user)
  -y, --yes                  Required when stdin or stdout is not a TTY

$ claude plugin marketplace add --help
Usage: claude plugin marketplace add [options] <source>
  --scope <scope>             user (default), project, or local
  --sparse <paths...>

$ codex plugin add --help
Usage: codex plugin add [OPTIONS] <PLUGIN[@MARKETPLACE]>
  -m, --marketplace <MARKETPLACE>
  --json

$ codex plugin marketplace add --help
Usage: codex plugin marketplace add [OPTIONS] <SOURCE>
  <SOURCE>: a local path, owner/repo[@ref], HTTPS Git URL, or SSH Git URL
  --ref <REF>
  --json

$ codex plugin remove --help
Usage: codex plugin remove [OPTIONS] <PLUGIN[@MARKETPLACE]>
  -m, --marketplace <MARKETPLACE>
  --json
```

Recommended authored argv, pending the planner's final `-y`/`--json` ordering convention:
- Claude Code marketplace add: `claude plugin marketplace add seanb4t/engram` (D-04's exact source string; `codex plugin marketplace add`'s `owner/repo` form accepts the IDENTICAL string `seanb4t/engram`, confirmed via `--help`'s `<SOURCE>: ... owner/repo[@ref] ...`).
- Claude Code install: `claude plugin install engram@engram --json -y`
- Claude Code update: `claude plugin update engram@engram --json -y`
- Codex marketplace add: `codex plugin marketplace add seanb4t/engram --json`
- Codex install: `codex plugin add engram@engram --json`
- Codex update-via-remove-then-add (D-02): `codex plugin remove engram@engram --json` then `codex plugin add engram@engram --json`

### `internal/setup/leafpurity_test.go`'s stdlib-only gate (verbatim, read this session)

```go
// Source: internal/setup/leafpurity_test.go:83-90
firstSeg, _, _ := strings.Cut(importPath, "/")
if strings.Contains(firstSeg, ".") {
    nonStdlib = append(nonStdlib, offender{path, importPath})
}
...
if len(nonStdlib) > 0 {
    t.Fatalf("internal/setup imports non-stdlib package(s): %+v — full collected import set: %v", nonStdlib, allImports)
}
```

### `golang.org/x/mod/semver`'s exported surface (verified via `go doc`, this session — for design reference only, NOT importable from `internal/setup`)

```
func Build(v string) string
func Canonical(v string) string
func Compare(v, w string) int
func IsValid(v string) bool
func Major(v string) string
func MajorMinor(v string) string
func Max(v, w string) string
func Prerelease(v string) string
func Sort(list []string)
```

### `cmd/engram/buildversion.go`'s existing anchored-SemVer-core regex (precedent for the recommended local comparator)

```go
// Source: cmd/engram/buildversion.go:29-31 (read this session)
var patchCorePattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
```
A version string that does NOT match this anchored pattern (no prefix, no prerelease, no build metadata — exactly `X.Y.Z`) is, per D-03's own examples (`dev`, `0.16.1-dev+sha`), a dev/non-comparable build. `resolvedVersion()` (`cmd/engram/buildversion.go:130-152`, read this session) is the SAME function `engram version` itself calls — it is the correct operand for D-01's comparison (not the raw `version` ldflags var, which is literally `"dev"` for every non-release build and therefore useless for comparison on its own).

### AGENTS.md skills-index block markers (verbatim, for D-09's presence check)

```go
// Source: internal/skills/agentsmd.go:24-27 (read this session)
const (
    BlockStartMarker = "<!-- engram:skills:start -->"
    BlockEndMarker   = "<!-- engram:skills:end -->"
)
```
D-09's "index block present" check can reuse the package's own (currently unexported) `scanBlock`/`blockState` machinery (`agentsmd.go`) directly, since it already classifies `blockAbsent`/`blockWellFormed`/`blockMalformed` from a file's raw bytes — no new parsing logic needed, only a call site.

### Minimal `.codex-plugin/plugin.json` (recommended, per live-fetched schema — see Open Questions for the `interface` ambiguity)

```json
{
  "name": "engram",
  "version": "0.16.1",
  "description": "Self-hosted, correctable, OAuth-secured memory for coding agents: session-start recall, curation discipline, and a two-tier per-workspace memory scope. Register the engram MCP server with /engram-setup."
}
```
Mirrors `skill/engram/.claude-plugin/plugin.json` (`[VERIFIED: skill/engram/.claude-plugin/plugin.json]`, read this session) field-for-field — `name`, `version`, `description` are the identity fields the drift gate should compare byte-for-byte. `[CITED: https://agent-plugins.org/schemas/1.0.0/plugin.schema.json]`, fetched this session: the schema's own top-level `required` array is `["$schema", "name"]` — `version`, `description`, `author`, and `interface` are ALL optional per the schema itself. No `author` field is included above, matching the existing `.claude-plugin/plugin.json`'s own omission of it.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|-------------------|---------------|--------|
| `engram setup --apply` writes skill files directly into every present native runtime's skills directory, unconditionally | A plugin-capable runtime receives skills through its OWN plugin/marketplace mechanism; only a non-plugin-capable runtime gets the native copy | This phase (2026-09-14 CONTEXT.md decisions) | Prevents the double-registration defect (`curating-memory` next to `engram:curating-memory`) that motivated this milestone |
| No distinction between "runtime binary absent" and "runtime present but its plugin subcommand doesn't work" | A dedicated capability probe (`plugin list --json`, D-10) distinct from `Detect()`'s binary-on-PATH check | This phase | A stale/old binary on PATH degrades safely to the native path instead of producing a failed row |

**Deprecated/outdated:** Nothing in this phase deprecates prior behavior for opencode/generic — those runtimes' native-copy path is explicitly unchanged (out of scope, `REQ-plugin-opencode` deferred).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|-----------------|
| A1 | The plugin-lane decision+execution should live inside `internal/setup` as a new, parallel-to-`execute()` mechanism, not folded into `Plan.Actions`/`Plan.Probe` | Summary, Architecture Patterns | If the planner instead folds plugin actions into `Plan.Actions` naively (per ARCHITECTURE.md's pre-D-01 sketch), the executor's blind byte-compare will misclassify plugin state — a `current` plugin would report `wrote` on every re-run, violating REQ-plugin-install-or-update's "does nothing when already current" |
| A2 | A local, stdlib-only SemVer-core comparator (not `golang.org/x/mod/semver`) should implement D-01/D-03 inside `internal/setup` | Pitfall 3, Don't Hand-Roll | If the planner instead imports `x/mod/semver` directly into `internal/setup`, `TestSetupPackageIsStdlibOnlyLeaf` fails the build immediately — this is a mechanical, not a subjective, risk |
| A3 | `-y`/`--yes` should be appended to every `claude plugin install`/`update` action this phase authors | Pitfall 1, Code Examples | Without it, a scripted `--apply` run against an absent/outdated Claude Code plugin will hang or fail non-interactively — a live, reproducible defect, not a hypothetical one |
| A4 | Codex's `.codex-plugin/plugin.json` does NOT need an `interface` block for a CLI-only (skills + MCP) install | Code Examples, Open Questions | If Codex's ACTUAL loader (as opposed to the generic `agent-plugins.org` schema) silently rejects a manifest lacking `interface`, the plugin would fail to install and the row would need to report a failed plugin facet — this can only be conclusively resolved by a live install attempt, which this research session is barred from performing (read-only, no write verbs) |
| A5 | The `unavailable: <reason>` plugin-capability-probe-failure state should be modeled as a STRING facet value (not a new `Outcome` enum constant) | Pitfall 6 | If modeled as a new `Outcome` value instead, three exhaustive-switch sites (`aggregate.go`'s `precedenceOrder`/`isRecognizedOutcome`, `exit.go`'s `Classify`) must all be touched in the same commit — a real but bounded scope increase, not a correctness risk if done consistently |

**If this table is empty:** N/A — five assumptions recorded above, all flagged for plan-time confirmation.

## Open Questions

1. **Does `Plan()` author plugin actions as literal entries in `Plan.Actions`, or does the plugin lane live entirely OUTSIDE `Plan()`'s return value (a separate function call from `cmd/engram`)?**
   - What we know: `Plan()` is documented as pure (no exec); the plugin capability probe is inherently an exec; `setupgen.Render()` only ever renders `Plan.Actions` content, so if plugin actions never appear there, `/engram-setup`'s generated prose has NOTHING to say about plugin delivery (which may be the CORRECT outcome — plugin delivery is a `--apply`-time decision that depends on live machine state, unlike the four/five synthetic `Cases()` `setupgen` renders today).
   - What's unclear: Whether REQ-plugin-setupgen-regenerated expects the generated prose to MENTION plugin delivery at all (e.g., a new sentence in the delegation-preview table's surrounding prose) or only requires the EXISTING tables to stay accurate (which they trivially do if plugin actions never enter `Plan.Actions`).
   - Recommendation: Resolve at plan time by re-reading `skill/engram/commands/engram-setup.md`'s CURRENT prose (outside the generated region) for any existing plugin-adjacent sentence that would now be stale, and treat "no new generated table, but hand-authored prose nearby gets one clarifying sentence, PLUS the generated tables stay byte-correct because plugin actions never enter Plan.Actions" as the leading candidate — it resolves Pitfall 7 for free.

2. **Does Codex's ACTUAL plugin loader require an `interface` block in `.codex-plugin/plugin.json`, or is the `agent-plugins.org` schema's `required: ["$schema","name"]` authoritative for Codex specifically?**
   - What we know: Three WebFetch passes over the SAME primary source (`plugin-json-spec.md`) gave inconsistent summaries on a first pass, but a careful re-fetch with an explicit verbatim-quote instruction, PLUS a direct fetch of the actual referenced JSON Schema, both agree `interface` is optional. The existing, WORKING `.claude-plugin/plugin.json` (no `interface` block) is independent evidence that Anthropic's own loader accepts a 3-field manifest.
   - What's unclear: Whether Codex's loader (a DIFFERENT vendor's implementation) enforces something the shared `agent-plugins.org` schema does not, and whether a CLI-only plugin (no `apps`/no custom UI) needs anything the schema's optional `interface.capabilities` field would otherwise declare.
   - Recommendation: Ship the minimal 3-field manifest (matching `.claude-plugin/plugin.json`'s own shape) as the primary attempt; flag this as a `checkpoint:human-verify` or an explicit post-ship live observation (the `2026-08-23.01` D-10/D-11 pattern this repo already uses for "verify once the real thing runs") rather than blocking the phase on a write-verb probe this research session cannot perform.

3. **Where does the plugin lane resolve the runtime's binary path — does it reuse `Result.Binary` from the registration lane's `execute()`, or does it call `env.LookPath` a second time?**
   - What we know: `apply.go`'s `execute()` resolves `binary` ONCE (via `plan.Actions[0].Args[0]`) and reuses it for every subsequent exec in that runtime's sequence — explicitly closing a TOCTOU window (`apply.go`'s own doc comment, step 3). The plugin lane, if implemented as a PARALLEL call from `cmd/engram` rather than nested inside `execute()`, would naturally re-resolve the binary via its own `env.LookPath` call unless the already-resolved path is threaded through.
   - What's unclear: Whether re-resolving is an acceptable, bounded cost (LookPath is cheap and side-effect-free) or whether the TOCTOU-closing discipline should extend to the plugin lane too.
   - Recommendation: Thread `Result.Binary` (already computed by the registration lane) into the plugin-lane's entry point as a parameter, avoiding a second `LookPath` call and staying consistent with the existing TOCTOU-closing discipline — this is low-cost and clearly the more disciplined choice, not a genuine tradeoff.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|-----------|
| `claude` CLI | Plugin-capability probe, marketplace/install/update actions | ✓ (this machine) | 2.1.271 `[VERIFIED]` | Absent/broken → D-12's native-path fallback, by design |
| `codex` CLI | Plugin-capability probe, marketplace/install/update actions | ✓ (this machine) | codex-cli 0.154.0 `[VERIFIED]` | Absent/broken → D-12's native-path fallback, by design |
| `golang.org/x/mod` module | Optional, only if the x/mod route is chosen over the local comparator | ✓ (already in `go.sum`, indirect) `[VERIFIED]` | v0.40.0 | Local stdlib-only comparator (recommended default) |

**Missing dependencies with no fallback:** none — the entire feature is designed with a fallback (the pre-existing native path) for every failure mode.
**Missing dependencies with fallback:** `claude`/`codex` CLI absence or a broken `plugin` subcommand both fall back to the native skills-copy path by explicit design (D-12).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (`go test`) |
| Config file | none — no `go.test.yml`/testify config in this package tree |
| Quick run command | `go test ./internal/setup/... ./cmd/engram/... ./internal/setupgen/... ./internal/skills/... -run <TestName> -count=1` |
| Full suite command | `task` (lint + test, per CLAUDE.md's "Task runner" convention) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| REQ-plugin-capability-detection | A present runtime with a probe that exits nonzero/times out/unparseable JSON falls back to native with a reason, never a failed row | unit | `go test ./internal/setup/... -run TestPluginCapabilityProbeFailureFallsBackToNative -count=1` (new) | ❌ Wave 0 |
| REQ-plugin-install-or-update | Absent→marketplace-add+install; outdated→update(or remove+add for codex); current→zero actions; preview shows exact argv | unit | `go test ./internal/setup/... -run TestPluginPlan -count=1` (new); `go test ./cmd/engram/... -run TestSetupPluginPreviewArgv -count=1` (new) | ❌ Wave 0 |
| REQ-plugin-three-way-state | absent/outdated/current classification against parsed `plugin list --json` version | unit | `go test ./internal/setup/... -run TestPluginVersionCompare -count=1` (new — exercises the new local SemVer-core comparator across the D-01/D-03 boundary table: equal, less, greater, dev-prerelease, non-SemVer) | ❌ Wave 0 |
| REQ-plugin-skips-skills-copy | A plugin-delivered runtime authors zero native skills writes and no `AGENTS.md` action; `skills.Install` never called for it | unit (negative-space) | `go test ./cmd/engram/... -run TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites -count=1` (new) | ❌ Wave 0 |
| REQ-plugin-facet-reported | Plugin facet appears in text AND JSON; a `failed` plugin facet next to a `wrote` registration reports both, exit code 8 (`exitPartial`) | unit + fixture | `go test ./cmd/engram/... -run TestSetupApplyJSONEmitsPluginFacet -count=1` (new); extend `TestOperatorViewFixturesHaveNoUnsanitizedNesting` (`cmd/engram/operator_output_test.go`) with a plugin-facet fixture | ❌ Wave 0 (test); fixture addition to existing file |
| REQ-codex-plugin-manifest | `.codex-plugin/plugin.json` exists; identity fields equal `.claude-plugin/plugin.json`'s; release-please-config.json has the sync entry | unit | `go test ./internal/setupgen/... -run TestPluginManifestIdentityMatches -count=1` (new — or wherever the planner locates the drift gate, see Open Question) | ❌ Wave 0 |
| REQ-plugin-setupgen-regenerated | Generated `/engram-setup` prose reflects plugin actions (or explicitly documents why it does not, per Open Question 1); `--check-setup` CI gate stays green | unit | `go test ./internal/surfacesgen/... -run TestCheckSetup -count=1` (extend existing `main_test.go` fixtures, per `checkFixture`'s tempdir pattern) | ✓ existing file, extend |

### Sampling Rate

- **Per task commit:** the quick-run command scoped to the package(s) touched by that task.
- **Per wave merge:** `go test ./internal/setup/... ./cmd/engram/... ./internal/setupgen/... ./internal/skills/... -count=1`.
- **Phase gate:** `task` (full lint + test) green before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/setup/plugin_test.go` — new file, covers REQ-plugin-capability-detection, REQ-plugin-three-way-state (version comparator table test).
- [ ] `internal/setup/pluginversion_test.go` (or folded into `plugin_test.go`) — the D-01/D-03 comparator's boundary table: `0.16.1` vs `0.16.1` (equal→current), `0.16.0` vs `0.16.1` (outdated), `0.17.0` vs `0.16.1` (newer-than-binary→current+note), `dev` (non-SemVer→never-update), `0.16.1-dev.0+gabc123` (prerelease→never-update).
- [ ] `cmd/engram/setup_test.go` extensions — plugin-lane fixtures scripted through `fakeSetupEnvWithRun` (the existing idiom already scripts `(path,args)→(RunResult,error)`; a plugin test scripts a `plugin list --json` response distinctly from the registration probe's response, keyed on `args` content).
- [ ] `cmd/engram/operator_output_test.go`'s `operatorViewFixtures()` — a new plugin-facet fixture (flat-scalar field), proving `TestOperatorViewFixturesHaveNoUnsanitizedNesting` stays green with the new field.
- [ ] `internal/skills/environment_test.go` (or wherever `fakeSkillsEnv`/`OSEnvironment` live) — a fake `Lstat` closure added to the skills-package test fixtures for D-08's symlink-vs-copy test.
- [ ] Framework install: none — `go test` is already fully configured; no new test framework needed.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|--------------------|
| V1 Architecture, Design and Threat Modeling | yes | This phase introduces a NEW trust surface (a marketplace source + a marketplace-declared install command neither engram nor the operator authors directly) — explicitly scoped to engram's OWN marketplace only (D-04/D-05), never a foreign one, matching Anthropic's own "only install from sources you trust" guidance |
| V5 Input Validation | yes | Plugin/marketplace names and version strings parsed from `--json` output are treated as untrusted third-party data — matched via exact string comparison (`id == "engram@engram"`), never interpolated into a shell string or executed |
| V14 Configuration | yes | `.codex-plugin/plugin.json`'s identity fields are release-please-synced and drift-gated against `.claude-plugin/plugin.json` — prevents an accidental identity mismatch shipping silently |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| A marketplace-declared install command executes something neither engram nor the operator wrote (Pitfall 8, PITFALLS.md) | Elevation of Privilege | Scope the marketplace source to engram's OWN repo only (hardcoded, D-04); never accept an operator-supplied marketplace URL; preview shows the exact argv before `--apply` runs it |
| A vendor CLI's `--json` output is trusted blindly for version comparison, and a crafted/corrupted cache entry could report a false "current" state, suppressing a real update | Tampering | The comparison is READ-ONLY and its worst failure mode is "does nothing" (never installs/updates when it should) or "installs when unnecessary" (idempotent by the CLI's own install semantics, per D-01's design) — never a destructive action driven by untrusted content |
| Shipping vendor-specific or private substrings into `.codex-plugin/plugin.json` under `skill/engram/` | Information Disclosure | `skill/engram/hooks/tests/test_no_residual_memory_oauth.py` (confirmed `[VERIFIED]` this session — bans `fzymgc`, `litellm`, pre-rebrand branding under `skill/engram/`, excluding only `hooks/tests/`) already scans every file under the shipped bundle, including any new `.codex-plugin/plugin.json` — mirroring `.claude-plugin/plugin.json`'s existing vendor-neutral description satisfies this by construction |

## Sources

### Primary (HIGH confidence)

- `claude plugin list --json`, `claude plugin marketplace list`, `claude plugin install --help`, `claude plugin update --help`, `claude plugin marketplace add --help` — live, read-only, this session, `claude` 2.1.271.
- `codex plugin list --json`, `codex plugin list --json --available`, `codex plugin marketplace list`, `codex plugin --help`, `codex plugin add --help`, `codex plugin marketplace add --help`, `codex plugin marketplace add --help`, `codex plugin remove --help` — live, read-only, this session, `codex-cli` 0.154.0.
- `internal/setup/plan.go`, `apply.go`, `claudecode.go`, `codex.go`, `runtime.go`, `aggregate.go`, `exit.go`, `leafpurity_test.go`, `environment.go` — read verbatim this session, HEAD of `feat/2026-09-13.01`.
- `internal/skills/install.go`, `environment.go`, `agentsmd.go` — read verbatim this session.
- `cmd/engram/setup.go`, `root.go`, `version.go`, `buildversion.go`, `operator_output_test.go` — read verbatim this session.
- `internal/setupgen/setupgen.go`, `setupgen_test.go`; `internal/surfacesgen/main.go`, `main_test.go` — read verbatim this session.
- `go.mod:154`, `go.sum:325-326` — `golang.org/x/mod v0.40.0 // indirect`, confirmed present via direct read this session.
- `go doc golang.org/x/mod/semver` (local module cache) — exact exported API confirmed this session.
- `skill/engram/.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `release-please-config.json` — read verbatim this session.
- `skill/engram/hooks/tests/test_no_residual_memory_oauth.py` — read verbatim this session.
- `https://agent-plugins.org/schemas/1.0.0/plugin.schema.json` — fetched this session; `required: ["$schema","name"]` confirmed via targeted WebFetch prompt.

### Secondary (MEDIUM confidence)

- `code.claude.com/docs/en/plugins-reference` — fetched this session; confirms `-y`/`--yes` requirement and the `{command,outcome,message,pluginId,scope,failureCode}` JSON envelope shape for `install`/`update --json`, but does NOT document refuse-vs-noop behavior for an already-current/installed plugin.
- `developers.openai.com/codex/plugins/build` and `raw.githubusercontent.com/openai/codex/main/.../plugin-json-spec.md` — fetched this session; converged (after a re-fetch with explicit verbatim-quote instructions) on `interface` being optional, consistent with the primary JSON Schema fetch above.
- `.planning/research/ARCHITECTURE.md`, `FEATURES.md`, `PITFALLS.md` (2026-09-13 milestone research pass) — read this session; ARCHITECTURE.md's plugin-delivery sketch (§1) is flagged in this document's Summary as predating and being superseded in complexity by this session's D-01/D-10–D-12 decisions.

### Tertiary (LOW confidence)

- A first WebFetch pass over `plugin-json-spec.md` (before the verbatim-quote re-fetch) reported `interface` as MANDATORY — contradicted by two subsequent, more careful fetches of the same and adjacent primary sources. Recorded here only to document the disagreement was investigated and resolved, not to be relied upon.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependency; every CLI fact live-verified this session.
- Architecture: MEDIUM — the exact Go surface (new types/fields) is a genuine plan-time design decision this research narrows but does not finalize (per CONTEXT.md's own "Claude's Discretion"), and this session surfaced a real, previously-unaddressed constraint (stdlib-only-leaf) that changes the recommended shape.
- Pitfalls: HIGH for the seven documented above (each grounded in a direct code read or a live CLI probe this session); MEDIUM for the Codex manifest `interface` question (Open Question 2 — cannot be conclusively resolved without a write-verb probe this session is barred from performing).

**Research date:** 2026-09-14
**Valid until:** 30 days (stable third-party CLI surfaces per this repo's own memory `r7n0nejp9f`, which measured Codex's flag surface holding across two minor-version bumps in six days) — re-verify `claude`/`codex` `--help` output if either CLI reports a NEW major/minor version by the time this phase is implemented.
