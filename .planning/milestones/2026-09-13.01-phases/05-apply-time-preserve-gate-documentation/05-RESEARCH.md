# Phase 5: Apply-Time Preserve Gate & Documentation - Research

**Researched:** 2026-09-16
**Domain:** Go CLI executor design (destructive-write gating on a pre-computed read-only classification) + technical documentation closeout
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Carried forward (decided in earlier phases — do not re-ask):**
- `--apply` is the **only** consent gate; plugin-first delivery; Cursor deferred (`qy29m0j3d2`).
- Phase 4 D-01: `preserved` iff the observed entry carries a facet the current `Options` do not
  account for; argv intent does not override. D-04: `preserved` is a non-failed attempt (exit 0).
  D-09: ambiguity (no scanner, unparseable read, probe seam error) resolves to `would-write`, never
  `preserved`/`already-correct`. D-10: opencode authors no scanner and is never compared. D-11/D-12:
  total parse; closed typed `Facet` enum in stable order. D-02/D-03: observed header values are
  compared raw in a local and never retained; rendered fields are rebuilt from the redacted
  `Observation`; `Registered`/`Drift`/`Reason` go through `boundCapture`.
- Phase 3 D-12: the plugin lane is independent of registration outcome — a `preserved` registration
  row still carries its plugin/skills facets, and those lanes still apply.
- Phase 4's `04-OBSERVATIONS.md` pins the read-verb shapes; the `oauth-client` read-back shape is
  **unobserved** (Phase 4 D-07) and stays so in this phase — see D-03 below.

**The apply lane:**
- **D-01:** `--apply` runs the same `Observe → Compare` as preview **before any action** for a
  runtime implementing `DriftRuntime`: `already-correct` → zero registration actions, outcome
  `already-correct`; `would-write` → run `Plan.Actions` → `wrote`; `preserved` → zero registration
  actions (Claude Code's `mcp remove` included), outcome `preserved`. The post-write byte-compare
  is **retired for drift-capable runtimes**. Runtimes without a scanner (opencode) and ambiguous
  reads keep today's write-then-byte-compare path unchanged (D-09/D-10). Re-running setup on a
  converged Claude Code install is a true no-op — no remove-then-add, no OAuth logout.
  — **Reversibility: costly** — `wrote`/`already-correct` become pre-write claims for parsed runtimes;
  the results table, the generated `/engram-setup` prose, and the exit taxonomy's documented
  meaning of `already-correct` ("does not guarantee no write ran") all change with it.
- **D-02:** After a `would-write` write succeeds, the executor **re-probes once** and rebuilds
  `Result.Registered` through the same `Observe → redact → render` path preview uses, so the row
  shows the NEW registration (redacted) and the last raw-capture site in `apply.go`
  (`displayCapture(probe2…)`) is closed. The outcome stays `wrote` regardless of what the re-observe
  says — a post-write read is never used to claim `already-correct` or to fail the row (D-09).

**The OAuth re-login consequence (REQ-apply-rewrite-consequence):**
- **D-03:** A Claude Code registration is treated as OAuth-authenticated **by shape**: the observed
  registration carries no `Authorization`/bearer header (reads as `oauth` or `oauth-client`). When
  such a registration classifies `would-write` on Claude Code — i.e. remove-then-add will run —
  the consequence fires. Deliberately conservative: it warns even if the user never completed a
  login, because a false "you will need to log in again" costs nothing and a missed one is a
  silent logout. No new observation is required and auth *state* is never parsed.
- **D-04:** The consequence is a **typed row note** rendered in the Claude Code row's existing
  `Notes` field (where the tolerant-remove consequence already lands) in **both** preview and apply.
  `--apply` does **not** pause: preview-by-default is where the operator reads it "before it runs",
  and `--apply` is the milestone's only consent gate. No stderr side-channel, no TTY prompt.

**The `preserved` escape hatch:**
- **D-05:** **No new flag.** engram never destroys what it cannot reproduce. The `preserved` row's
  reason names the facet(s) AND the exact manual step to clear the registration in the runtime's own
  tool (`claude mcp remove engram --scope user`; for Codex, the `[mcp_servers.engram]` table in
  `config.toml`), after which a re-run reads `would-write`. The human performs the destructive step
  with the runtime's own confirmation semantics. `--replace-registration` was considered and
  rejected as a second consent surface that re-opens the incident class if it lands in a script.

**Docs + post-release closeout (REQ-docs-setup-v2):**
- **D-06:** Follow the `2026-08-23.01` Phase 6 **D-10 precedent**: docs are written truthfully
  pre-merge (an "unreleased as of vX.Y.Z" notice wherever a behavior is not yet in a cut release);
  the phase writes `05-POST-RELEASE.md` listing the qualifying-release checks (`brew install` of the
  next release; `engram setup` plugin-first + `--header` + `preserved` + the apply gate observed on
  a real machine; man pages present in the cask); Phase 5's verification **passes** with
  `post_release_status: pending`; `REQ-docs-setup-v2` stays unchecked until a
  `05-RELEASE-<ver>.md` observation is recorded. The milestone audit sees the open handoff.
- **D-07:** Content is split **by reader intent** and every guide is **gated** like Phase 4's
  `agent_setup_docs_test.go` (a `migrate_docs_test.go`-shaped zero-occurrence-plus-positive-control
  test per guide): `install.md` = getting the binary + what the cask installs (binary, completions,
  **man pages** — `man engram-setup`); `agent-setup.md` = running `engram setup`: plugin-first
  delivery per runtime, the `--header` shape, the results table incl. `preserved`, the apply gate,
  the OAuth re-login note, and the manual remediation; `plugin.md` = what the plugin is and does,
  pointing at `agent-setup.md` for installation. Cross-links between the three.

### Claude's Discretion
- How the `mutate` branch shares the observe/compare/render sequence with the `!mutate` branch
  without duplicating it (a common pre-action classification step; the byte-compare retained only
  on the non-drift path), keeping the executor content-blind and `internal/setup` stdlib-only.
- The typed constant(s) for the OAuth re-login note and the per-runtime manual-remediation hint —
  authored in each runtime's own file (AUTHORED-HERE), never composed in the shared executor.
- Whether `internal/setupgen`'s generated `/engram-setup` prose gains `preserved`/apply-gate
  wording under its existing drift check, and how `--help` states the gate.
- The exact "unreleased" notice wording and where `05-POST-RELEASE.md`'s checklist sits relative
  to the `06-POST-RELEASE.md` precedent (reuse its shape).
- The apply-lane fixture shape that proves SC1 (`--apply` against a pre-seeded unreproducible
  registration issues zero registration writes while skills/plugin actions still run) and SC2
  (`mcp remove` never runs on `preserved`) through the existing `fakeEnvWithRun`/`scriptedRun`
  harness and the `cmd/engram` process boundary.
- Red-evidence patches for the gate (registered by the orchestrator after the last plan).

### Deferred Ideas (OUT OF SCOPE)
- `--replace-registration` (an explicit overwrite flag) — considered and rejected for this milestone;
  revisit only with its own consent design.
- A stderr pre-write warning line or an interactive pause in `--apply` — rejected; the row note is
  the single rendering path.
- Observing the `oauth-client` read-back shape — not needed under D-03 (shape-based detection);
  capture opportunistically in `05-POST-RELEASE.md` if convenient.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-apply-preserve-gate | `--apply` consults the same classification before writing and performs zero write actions for a `preserved` registration — including never running Claude Code's `mcp remove` step — while still applying skills/plugin actions for that runtime. Proven by a fixture test that runs `--apply` (not only preview) against a pre-seeded unreproducible registration and asserts no registration write was issued. | See "The pre-action classification gate" and "Validation Architecture" §REQ-apply-preserve-gate below — exact insertion point in `execute()` (`apply.go:376-434`), the `scriptedRun` panic-on-overrun mechanism as the zero-write proof, and confirmation that the plugin/skills lanes already run unconditionally of registration outcome (`cmd/engram/setup.go:787-825`). |
| REQ-apply-rewrite-consequence | When a reproducible difference on Claude Code requires remove-then-add of an existing registration, preview and apply state that an OAuth-authenticated registration will need to log in again before the rewrite runs. | See "The OAuth re-login consequence: design options" — the by-shape (`obs.Auth == AuthNone`) trigger, and a recommended `Observation`-field extension pattern that keeps `apply.go`/`drift.go` content-blind while `claudecode.go` authors the condition and text. |
| REQ-docs-setup-v2 | `guides/install.md`, `guides/agent-setup.md`, and `guides/plugin.md` describe the shipped behavior with a post-release live observation recorded before the requirement is checked off. | See "Documentation gap inventory" (per-guide diff against what has already shipped) and "Validation Architecture" §REQ-docs-setup-v2 (code-gated half + Manual-Only post-release half, `05-POST-RELEASE.md` shape lifted from the `06-POST-RELEASE.md`/`06-VERIFICATION.md` precedent). |
</phase_requirements>

## Summary

This phase rewrites exactly one branch of one function — `execute()`'s `mutate == true` arm in
`internal/setup/apply.go` (currently lines 376-434, byte-identical since before Phase 4, confirmed
by `04-01-SUMMARY.md`'s own pin against commit `ff5a6f94`) — plus one small, per-runtime, currently
nonexistent code path (the OAuth-consequence note), plus three documentation files. There is no new
package, no new interface, and no new flag. Every mechanism the apply-time gate needs already exists
and already runs on the preview (`!mutate`) side: `rt.(DriftRuntime)`, `Observe`, `Compare`,
`renderObservation`, `OutcomePreserved`, `boundCapture`. Phase 5's job is to make the `mutate==true`
branch run that SAME sequence **before** touching `plan.Actions`, branch on the resulting
`Outcome` (skip all registration actions on `already-correct`/`preserved`, run them only on
`would-write`), and re-probe once after a real write to rebuild `Result.Registered` the same
redaction-safe way preview already does.

The plugin and skills lanes already execute independently of the registration `Outcome` —
`setupRuntimeRowFromResult` (`cmd/engram/setup.go:787`) calls `PluginApply`/`setupApplySkillsFacet`
unconditionally whenever `r.Present`, never gated on `r.Outcome` — so SC1's "skills/plugin actions
still run on a preserved row" is **already true today** and needs no new code; the plan's job is to
add a fixture test that PROVES it stays true once the registration lane's own write path is gated,
not to build new plumbing for it.

The OAuth re-login consequence (REQ-apply-rewrite-consequence) is the one genuinely new code
surface: nothing in the shipped codebase renders a rewrite-consequence note today, and no
`Observation` field carries anything comparable to what D-03/D-04 describe. The cleanest fit with
the existing `WholeEntryNote`/AUTHORED-HERE pattern is either (a) extending
`claudeCodeWholeEntryNote`'s composed sentence to also cover the would-write case (not only
preserved), or (b) a new `Observation.RewriteConsequence` field, populated only by
`claudeCodeRuntime.Observe` when it detects `AuthNone` (never by `codexRuntime.Observe`, which
always leaves it empty), surfaced generically by the executor onto `Result.Notes` whenever Outcome
resolves to `OutcomeWouldWrite` for a `DriftRuntime`. Both keep `apply.go`/`drift.go` free of any
by-name branch; the plan should pick one explicitly (this is a decision to record, not default into).

Documentation is a closeout, not a discovery task: `agent-setup.md` already documents `preserved`
with the whole-entry/never-merge/never-shown clauses Phase 4 shipped (`agent_setup_docs_test.go`
already gates this) — but its `already-correct` row text ("does not guarantee that no write ran")
becomes **stale** the moment this phase ships D-01, and must be corrected to state the new pre-write
guarantee for claude-code/codex. `install.md` has zero mentions of plugin, header, or man pages
today and needs all three. `plugin.md` needs no structural change (D-07 already describes it as
"what the plugin is/does, point at agent-setup.md") but may gain a preserved/apply-gate cross-link
sentence.

**Primary recommendation:** Insert one pre-action classification block at the top of `execute()`'s
`mutate == true` branch (immediately after the existing action-validation loop and `LookPath` call,
before the probe1 dispatch), reusing `rt.(DriftRuntime)` + `Observe` + `Compare` verbatim from the
`!mutate` branch; branch the write-action loop on the resulting Outcome; keep the post-write
byte-compare path only for non-`DriftRuntime`/ambiguous cases. Author the OAuth-consequence
condition and text in `claudecode.go` (never `apply.go`), gate its rendering on a generic,
content-blind executor rule ("would-write + a non-empty per-runtime consequence string → append to
Notes"). Update `agent-setup.md`'s `already-correct` row and write `05-POST-RELEASE.md` in the
`06-POST-RELEASE.md` shape with `post_release_status: pending`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Pre-write drift classification (reuse) | `internal/setup` (executor: `apply.go`) | `internal/setup` (`drift.go` — pure `Compare`/`Observe`) | Already the shared, content-blind classification engine; Phase 5 only changes WHEN it is consulted (before vs. never, under `mutate==true`), never what it computes. |
| Registration write-action gating | `internal/setup` (executor: `apply.go`) | — | The executor is the single place `plan.Actions` is ever run; gating belongs exactly where the run loop already lives, not in `cmd/engram`. |
| OAuth re-login consequence text/condition | `internal/setup` (runtime file: `claudecode.go`) | `internal/setup` (executor: generic Notes-append rule) | AUTHORED-HERE invariant: per-runtime prose and the by-shape trigger belong in the runtime's own file; the executor only plumbs a possibly-empty string through, mirroring `WholeEntryNote`. |
| Manual-remediation hint (preserved row) | `internal/setup` (runtime files: `claudecode.go`, `codex.go`) | — | Same AUTHORED-HERE constant pattern already used for `claudeCodeWholeEntryNote`/`codexWholeEntryNote`; extend those constants rather than invent a new field, unless the plan finds a reason to split them. |
| Plugin/skills independence from registration outcome | `cmd/engram` (`setup.go`: `setupRuntimeRowFromResult`) | — | Already implemented (Phase 3 D-12) — no work here, only a fixture test proving it. |
| Results-table / `--help` / `/engram-setup` prose updates | `cmd/engram` (`setup.go`) + `internal/setupgen` | — | Rendering/generation layer; must not re-derive argv or drift text, only reflect what `internal/setup` already composes. |
| Guide content (`install.md`/`agent-setup.md`/`plugin.md`) | `docs-site` (Markdown) | `cmd/engram` (docs-gate tests) | Documentation tier; gated by process-boundary tests in `cmd/engram`, mirroring `agent_setup_docs_test.go`. |
| Post-release observation record | `.planning/` (`05-POST-RELEASE.md`, `05-RELEASE-<ver>.md`) | — | Human-in-the-loop artifact, not code; D-06/D-10 pattern. |

## Standard Stack

No new libraries. This phase is a pure rewrite of existing Go code in `internal/setup`/`cmd/engram`
plus Markdown documentation. `internal/setup` is machine-gated stdlib-only
(`TestSetupPackageIsStdlibOnlyLeaf`, `internal/setup/leafpurity_test.go`) — confirmed by reading the
package's own imports across every file touched (`apply.go`, `drift.go`, `claudecode.go`,
`codex.go`, `plugin.go`, `plan.go`): every import is `context`, `errors`, `fmt`, `strings`, `time`,
`unicode/utf8`, `net/url`, `slices`, `encoding/json`, `path/filepath`, `regexp`, `strconv`, `io` —
all standard library `[VERIFIED: internal/setup/apply.go:1-13, drift.go:1-19, claudecode.go:1-12,
codex.go:1-15, plugin.go:1-31]`.

### Core
No new packages required. The phase reuses:

| Symbol | Package | Purpose | Why reuse (never re-derive) |
|--------|---------|---------|------------------------------|
| `rt.(DriftRuntime)` / `Observe` | `internal/setup` | Parse a runtime's own read-verb output into an `Observation` | Already the single AUTHORED-HERE parser per runtime (`04-RESEARCH.md` Pitfall 3); re-deriving in the apply lane creates the two-encodings drift the milestone exists to prevent. |
| `Compare` | `internal/setup/drift.go` | The single D-01 predicate (`preserved`/`would-write`/`already-correct`) | Pure function, already unit-tested for all three outcomes per runtime (Phase 4). |
| `renderObservation` | `internal/setup/drift.go` | Rebuild `Result.Registered` from the redacted `Observation` | D-03's redaction-by-construction; the ONLY way `Registered` is ever built for a `DriftRuntime` as of Phase 4. |
| `boundCapture` | `internal/setup/apply.go` | Bound any rendered field derived from third-party output | WR-01's fix (`04-REVIEW-FIX.md`) — must be applied to every new rendered field this phase adds (the OAuth-note text itself is engram-authored so needs no bound, but any observation-derived text it's composed alongside still does). |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Reusing `Observe`/`Compare` inside the `mutate` branch | Re-probing and re-parsing independently in a new apply-only code path | Rejected by CONTEXT.md's own framing ("the apply-time gate consults the same classification Phase 4 introduces") — a second parse path is exactly the two-encodings drift risk 04-RESEARCH.md Pitfall 3 warns against. |
| A new `Observation.RewriteConsequence` field for the OAuth note | Extending `claudeCodeWholeEntryNote`'s constant text to also apply outside `preserved` | Both are viable AUTHORED-HERE-compliant options (see "OAuth re-login consequence: design options" below); the plan must pick one and record why — not left to fall out of implementation. |

**Installation:** N/A — no packages to install.

## Package Legitimacy Audit

**Not applicable.** This phase introduces zero new external dependencies (Go, npm, or otherwise) —
confirmed by reading every file this phase touches or extends (`apply.go`, `drift.go`,
`claudecode.go`, `codex.go`, `plugin.go`, `plan.go`, `cmd/engram/setup.go`,
`cmd/engram/agent_setup_docs_test.go`, `cmd/engram/man.go`, `internal/setupgen/setupgen.go`) —
every import already present is standard library. `internal/setup`'s own machine gate
(`TestSetupPackageIsStdlibOnlyLeaf`) will fail the build the moment a non-stdlib import reaches that
package, so any accidental dependency addition is caught mechanically, not just by this audit.

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────────────────┐
                    │  cmd/engram/setup.go: setupApplyRun                      │
                    │  (one call to setup.Apply per selected runtime)          │
                    └───────────────────────┬───────────────────────────────────┘
                                             │ setup.Apply(ctx, env, rt, opts)
                                             ▼
┌────────────────────────────────────────────────────────────────────────────────────┐
│ internal/setup/apply.go: execute(ctx, env, rt, opts, mutate=true)                    │
│                                                                                        │
│  1. Detect / Plan / validate Actions / LookPath   (UNCHANGED)                        │
│  2. probe1 := runSeam(Plan.Probe)                 (UNCHANGED — already runs first)   │
│                                                                                        │
│  ── NEW: pre-action classification (mirrors !mutate branch) ──                       │
│  3. dr, isDrift := rt.(DriftRuntime)                                                 │
│     if hasProbe && isDrift && probe1Err == nil:                                     │
│         obs, ok := dr.Observe(probe1.Stdout+probe1.Stderr, opts)                    │
│         if ok:                                                                       │
│             d := Compare(obs, opts)                                                  │
│             switch d.Outcome {                                                       │
│             case OutcomeAlreadyCorrect:                                              │
│                 → res.Outcome = already-correct; ZERO write actions run; return      │
│             case OutcomePreserved:                                                   │
│                 → res.Outcome = preserved; ZERO write actions run                    │
│                   (Claude Code's mcp remove NEVER dispatched); return                │
│             case OutcomeWouldWrite:                                                  │
│                 → fall through to step 4 (run plan.Actions as today)                 │
│                   [NEW: OAuth-consequence note appended to Notes here, if any]       │
│             }                                                                        │
│     else: (no probe / not DriftRuntime / probe1Err / !ok)                           │
│         → fall through unchanged (D-09/D-10 ambiguity path, today's behavior)        │
│                                                                                        │
│  4. Run plan.Actions in order (tolerant/fatal per Action.Tolerant)  (UNCHANGED)      │
│  5. probe2 := runSeam(Plan.Probe)  (only if hasProbe)                                │
│     if isDrift-and-ok-on-probe2: Registered = renderObservation(Observe(probe2))     │
│         (NEW — replaces raw displayCapture(probe2…) for DriftRuntime rows)           │
│     else: Registered = displayCapture(probe2…)  (UNCHANGED — ambiguous/no-scanner)   │
│  6. Outcome = wrote  (UNCHANGED — D-02: post-write read never claims already-correct)│
└────────────────────────────────────────────────────────────────────────────────────┘
                                             │
                  ┌──────────────────────────┴───────────────────────────┐
                  ▼                                                       ▼
   ┌───────────────────────────────┐                    ┌───────────────────────────────┐
   │ setup.PluginApply(...)         │                    │ setupApplySkillsFacet(...)     │
   │ ALREADY unconditional on       │                    │ / setupReportNativeSkills(...) │
   │ r.Present — runs regardless    │                    │ ALREADY unconditional on       │
   │ of registration Outcome        │                    │ r.Present — runs regardless    │
   │ (cmd/engram/setup.go:806-812,  │                    │ of registration Outcome        │
   │ UNCHANGED this phase)          │                    │ (cmd/engram/setup.go:813-816,  │
   └───────────────────────────────┘                    │  UNCHANGED this phase)          │
                                                          └───────────────────────────────┘
```

### Recommended Project Structure

No new files/directories. Edits land in:

```
internal/setup/
├── apply.go       # execute()'s mutate==true branch — the pre-action gate (D-01, D-02)
├── claudecode.go  # OAuth re-login consequence text/condition (D-03/D-04); extend
│                  #   claudeCodeWholeEntryNote or add a per-runtime consequence field (D-05 hint)
├── codex.go       # extend codexWholeEntryNote with its own manual-remediation hint (D-05)
├── apply_test.go  # new Apply-lane preserve-gate tests (SC1/SC2), mirroring
│                  #   TestApplyConvergesCodex/TestApplyConvergesClaudeCode's shape
cmd/engram/
├── setup.go            # setupLongDescription's apply-gate sentence (Claude's discretion)
├── setup_test.go       # process-boundary fixture proving SC1/SC2 through runClient
├── agent_setup_docs_test.go  # extend with new legs; author install_docs_test.go / plugin_docs_test.go
internal/setupgen/
├── setupgen.go     # only if the plan decides prose needs preserved/apply-gate wording (discretion)
docs-site/src/content/docs/guides/
├── install.md      # add plugin/header/man-page content (currently absent)
├── agent-setup.md  # fix already-correct row text; add apply-gate + OAuth-consequence content
├── plugin.md        # cross-link sentence (minor)
.planning/phases/05-apply-time-preserve-gate-documentation/
├── 05-POST-RELEASE.md   # D-06/D-10 handoff artifact
```

### Pattern 1: Pre-action classification gate (D-01)

**What:** Before running any write `Action`, the apply lane runs the exact same
`Observe → Compare` sequence the preview lane already runs, and branches the write-action loop on
the result.

**When to use:** Only for a runtime implementing `DriftRuntime` (claude-code, codex) with a probe
wired and a successful, parseable probe1 read. Every other case (no probe, no `DriftRuntime`, probe
seam error, unparseable output) MUST fall through to the existing write-then-byte-compare path
unchanged — this is D-09's ambiguity-resolves-to-`wrote` invariant, restated for the apply lane: an
ambiguous read must never be treated as `already-correct` or `preserved`, because both of those
outcomes SKIP the write, and skipping a write on ambiguous grounds is exactly the kind of silent
no-op this design must not introduce.

**Example (reasoning, not verified code — the `mutate==true` branch does not yet contain this; it
mirrors the ALREADY-SHIPPED `!mutate` branch at `apply.go:326-373` `[VERIFIED:
internal/setup/apply.go:326-373]`):**
```go
// Source: internal/setup/apply.go — EXISTING !mutate branch, quoted verbatim as the
// pattern the mutate branch must mirror for its own pre-action classification.
if !mutate {
    res.Outcome = OutcomeWouldWrite
    dr, isDrift := rt.(DriftRuntime)
    if !hasProbe || !isDrift {
        res.Drift = notComparedNote(name, "runtime authors no registration scanner")
        return res
    }
    if probe1Err != nil {
        res.Drift = notComparedNote(name, quoteArgs(plan.Probe)+": "+probe1Err.Error())
        return res
    }
    obs, ok := dr.Observe(probe1.Stdout+probe1.Stderr, opts)
    if !ok {
        res.Drift = notComparedNote(name, fmt.Sprintf("%s exited %d: output not recognized as a registration", quoteArgs(plan.Probe), probe1.ExitCode))
        return res
    }
    d := Compare(obs, opts)
    res.Outcome = d.Outcome
    // ... Facets/Drift/Registered/Reason composed from d and obs ...
    return res
}
```
The `mutate == true` branch's new pre-action block reuses this SAME `dr, isDrift :=
rt.(DriftRuntime)` / `hasProbe` / `probe1Err` / `dr.Observe` / `Compare` sequence — sharing it as a
local helper (or inlining it a second time) is the plan's call, but the four inputs
(`hasProbe`, `isDrift`, `probe1Err`, the parsed `Observation`) must be the exact same values probe1
already produced earlier in `execute()` (both branches already share `probe1`/`probe1Err`, computed
once before the `if !mutate` split — see `apply.go:318-324` `[VERIFIED: internal/setup/apply.go:318-324]`).

### Pattern 2: Post-write re-observe rebuilds `Registered` the redaction-safe way (D-02)

**What:** After a successful `would-write` write, `apply.go` currently does
`res.Registered = displayCapture(probe2.Stdout + probe2.Stderr)` — the LAST raw-capture site in the
package `[VERIFIED: internal/setup/apply.go:434]`. D-02 requires this replaced, for a `DriftRuntime`,
with the same `Observe → renderObservation` path preview uses, so a header value the second probe
echoes back can never reach `Result.Registered` unredacted.

**When to use:** Only after a real write ran (`OutcomeWrote` path); the byte-compare that decides
`wrote` vs. `already-correct` for NON-drift-capable runtimes is retained unchanged (opencode, or any
runtime whose probe1 was ambiguous) — D-02 is explicit that Outcome stays `wrote` regardless of what
the re-observe says; the re-observe is for `Registered`'s rendering only, never for reclassifying
Outcome.

### Pattern 3: AUTHORED-HERE per-runtime consequence text (extends `WholeEntryNote`'s existing shape)

**What:** `Observation.WholeEntryNote` is already a fixed, per-runtime-authored sentence
(`claudeCodeWholeEntryNote`/`codexWholeEntryNote`) appended to a `preserved` row's `Reason`
`[VERIFIED: internal/setup/drift.go:226-229, claudecode.go:335-342, codex.go:292-298]`. This is the
existing, proven pattern for "a runtime-specific sentence, composed once, plumbed through a
content-blind executor" — the OAuth re-login consequence and the D-05 manual-remediation hint should
both follow it rather than inventing a new mechanism.

**When to use:** Any new per-runtime, fixed-or-conditional sentence this phase needs to surface on
`Result.Notes` or `Result.Reason`.

### Anti-Patterns to Avoid
- **A by-name branch in `apply.go` for "is this claude-code":** CONTEXT.md's discretion section is
  explicit that the OAuth-consequence condition/text must be AUTHORED-HERE in `claudecode.go`, never
  composed in the shared executor. `apply.go` may only consult a generic field or interface method
  (e.g., "does this Observation carry a non-empty consequence string"), never `rt.Name() ==
  "claude-code"`.
- **Re-deriving argv or drift text in `cmd/engram` or `internal/setupgen`:** every existing pattern
  in this codebase (the package doc comment's AUTHORED-HERE invariant, `plan.go`'s comments on
  `Action.Command()`) forbids a second encoding of what `internal/setup` already composed. The
  results-table wording change (already-correct's meaning) must be edited in the doc-comment PROSE
  (`plan.go`'s `Result` doc comment, `cmd/engram/setup.go`'s `setupLongDescription`,
  `agent-setup.md`), never re-derived from a parallel classification.
- **Treating an ambiguous probe read as license to skip a write:** D-09's invariant is symmetric —
  ambiguity must never become `already-correct` OR `preserved`. A bug that treats "I couldn't parse
  this" as "so it must be safe to skip" would silently stop registering runtimes whose CLI output
  format drifted, which is worse than the incident this milestone fixes.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Parsing a runtime's own registration read-back | A new apply-lane-specific parser | The EXISTING `DriftRuntime.Observe` implementations (`claudecode.go`, `codex.go`) | Already AUTHORED-HERE, already unit-tested for every Phase 4 fixture shape (`claudecode_test.go`, `codex_test.go`, `04-OBSERVATIONS.md`'s live-verified shapes). A second parser is the two-encodings drift 04-RESEARCH.md Pitfall 3 names explicitly. |
| Deciding preserved/would-write/already-correct | A new apply-lane comparison function | `drift.go`'s `Compare` (pure, already exhaustively tested) | `Compare` is declared "once, pure" by its own doc comment; a second comparison function is a direct violation of that stated invariant and an instant drift risk between preview and apply. |
| Bounding/quoting third-party captured text | A new truncation/quoting helper | `boundCapture`/`displayCapture`/`quoteWord` (`apply.go`, `quote.go`) | Already the fixed WR-01/WR-02 discipline this whole package follows; a parallel helper would need its own security review. |

**Key insight:** This phase's entire value is that it does NOT introduce new parsing, comparison, or
rendering logic for the registration state itself — every one of those already exists and is already
proven correct for the preview lane. The only genuinely new logic is (1) the branch that decides
whether to skip the write loop, and (2) the OAuth-consequence text/condition. Anything beyond that
in a plan's task list should be treated with suspicion — it likely duplicates existing, tested code.

## Runtime State Inventory

Not applicable — this phase is not a rename/refactor/migration phase. It changes when an EXISTING
write path runs, not what any stored/registered state means. No OS-registered state, stored data
keys, secrets, or build artifacts change name or meaning as a result of this phase. **Nothing found
in this category** — verified by reading every file this phase touches; none references a renamed
identifier, key, or path.

## Common Pitfalls

### Pitfall 1: Collapsing the apply-lane classification back into "a single read compared against
nothing" (the exact mistake `04-RESEARCH.md` Pitfall 1 already warns a future reader not to make)

**What goes wrong:** A future edit "simplifies" the apply lane by reusing the preview classification
call directly as a boolean ("did preview say already-correct?") rather than re-running `Observe →
Compare` against a FRESH probe1 read taken inside the SAME `Apply` call.

**Why it happens:** It looks like a harmless refactor — "we already know the answer from preview" —
but Preview and Apply are separate process invocations (the operator runs `engram setup` then
`engram setup --apply` as two commands); there is no cross-call state to reuse, and the registration
could have changed between the two invocations.

**How to avoid:** The pre-action classification MUST run fresh, inside `Apply`'s own `execute()`
call, against probe1's OWN freshly-captured output — exactly as `04-RESEARCH.md`'s own doc comment
in `apply.go` (lines 99-104) already states this distinction explicitly. This is not new pitfall
territory; it is the SAME distinction restated for a second consumer.

**Warning signs:** Any code path that accepts a `Drift`/`Observation` value as a function PARAMETER
into the `mutate==true` branch from outside `execute()`'s own probe1 read.

### Pitfall 2: The write-action loop advancing past `claudeCodeRemoveAction` for a `preserved` row

**What goes wrong:** The existing `mutate==true` loop (`apply.go:382-418`) iterates `plan.Actions` in
order; `claudeCodeRemoveAction` is `plan.Actions[0]` for every claude-code auth mode
`[VERIFIED: internal/setup/claudecode.go:161-204]`. If the new pre-action classification block is
inserted AFTER the loop starts (or the loop is entered before checking `d.Outcome`), the tolerant
`mcp remove` still fires even though the row will end up `preserved` — this is the exact regression
SC2 exists to catch.

**Why it happens:** The natural diff-minimizing edit is to wrap the EXISTING loop in a conditional,
which is correct only if the conditional is evaluated and can `return` BEFORE the loop's first
iteration — a subtle off-by-one in "before vs. inside the loop" placement produces code that looks
identical in a diff review but runs the first action every time.

**How to avoid:** The classification block must `return res` directly on `already-correct` and
`preserved` — mirroring the `!mutate` branch's own early-return shape (`apply.go:335-350`) — never
merely set a boolean flag consulted inside the loop.

**Warning signs:** A test asserting `res.Outcome == OutcomePreserved` passes, but a SEPARATE test
counting `Run` invocations for the same fixture shows `len(calls) > 1` (i.e., `mcp remove` still
ran). SC2's fixture test must assert BOTH the outcome AND the call count/argv, not outcome alone.

### Pitfall 3: `Options.Auth` argv intent silently overriding the `preserved` classification

**What goes wrong:** Someone "fixes" a perceived bug by having the apply lane consult
`opts.Auth`/`opts.Headers` to decide whether to skip the classification (e.g., "if the user
explicitly asked for `--auth bearer` with a header, maybe they WANT the overwrite").

**Why it happens:** It feels intuitively right that explicit user intent should win. But this is
EXACTLY the incident (`ryr82bf2s2`) this milestone exists to prevent, and Phase 4's D-01 already
settled this: "argv intent does not override" `[VERIFIED: 05-CONTEXT.md:28]`. `Compare`'s decision
is derived ENTIRELY from the observed registration vs. the requested Options' STRUCTURE (URL, auth
mode, header names) — never from a meta-signal about how emphatically the user asked.

**How to avoid:** `--replace-registration` was explicitly considered and rejected (D-05) as the
sanctioned way to express "yes, really overwrite this." No other mechanism should be built to
achieve the same effect implicitly.

**Warning signs:** Any new code reading `opts.*` inside the classification gate itself, rather than
only inside `Compare`'s own already-existing, already-tested comparison.

### Pitfall 4: The OAuth re-login note firing on codex or on a bearer-authenticated claude-code registration

**What goes wrong:** The consequence note is implemented as a blanket "claude-code + would-write"
rule rather than gating on `obs.Auth == AuthNone` — firing it on every claude-code rewrite, including
ones where the observed registration was already bearer-authenticated (where there is no OAuth
session to lose) or on codex (which has no destructive remove-then-add at all — `codex mcp add`
overwrites in a single non-tolerant call `[VERIFIED: internal/setup/codex.go:47-49, 176-179]`, so a
codex rewrite never has the "no claude-code registration remains mid-sequence" destructive window
D-03/D-04 describe).

**Why it happens:** "Claude Code + would-write" is a simpler condition to state than "Claude Code +
would-write + the observed shape has no Authorization/bearer header" and might look sufficient at a
glance since claude-code is the only runtime with the destructive remove-then-add shape at all.

**How to avoid:** Gate strictly on `obs.Auth == AuthNone` (per D-03's literal wording: "carries no
`Authorization`/bearer header") — `AuthBearer` and `AuthForeign` (a header IS present, just not the
engram bearer form) are NOT the OAuth shape and must not trigger the note.

**Warning signs:** A test fixture whose `Options.Auth == "bearer"` and whose observed registration IS
that runtime's own bearer form still gets the OAuth note appended.

## Code Examples

Verified patterns from the existing, shipped codebase (no external docs needed — this is an
internal-code-reuse phase):

### The exact byte-identical `mutate == true` branch this phase rewrites
```go
// Source: internal/setup/apply.go:376-441 (VERIFIED — read this session, quoted verbatim;
// pinned unchanged since commit ff5a6f94 per 04-01-SUMMARY.md's own claim)
if hasProbe && probe1Err != nil {
    res.Outcome = OutcomeFailed
    res.Reason = describeSeamError(name, quoteArgs(plan.Probe), probe1Err)
    return res
}

var notes []string
for _, action := range plan.Actions {
    rr, runErr := runSeam(ctx, env, binary, action.Args[1:])
    switch {
    case runErr != nil:
        res.Outcome = OutcomeFailed
        res.Reason = describeSeamError(name, action.Command(), runErr)
        if len(notes) > 0 {
            res.Notes = strings.Join(notes, "; ")
        }
        return res
    case rr.ExitCode != 0 && action.Tolerant:
        notes = append(notes, toleratedNote(action, rr.ExitCode, rr.Stderr))
    case rr.ExitCode != 0:
        res.Outcome = OutcomeFailed
        res.Reason = describeFailure(name, action.Command(), rr.ExitCode, rr.Stderr)
        if len(notes) > 0 {
            res.Notes = strings.Join(notes, "; ")
        }
        return res
    case action.Tolerant:
        if action.Description != "" {
            notes = append(notes, action.Description)
        }
    }
}
if len(notes) > 0 {
    res.Notes = strings.Join(notes, "; ")
}

if !hasProbe {
    res.Outcome = OutcomeWrote
    return res
}

probe2, probe2Err := runSeam(ctx, env, binary, plan.Probe[1:])
if probe2Err != nil {
    res.Outcome = OutcomeWrote
    return res
}
res.Registered = displayCapture(probe2.Stdout + probe2.Stderr)

if probe1Err == nil && probe1.Stdout == probe2.Stdout && probe1.Stderr == probe2.Stderr {
    res.Outcome = OutcomeAlreadyCorrect
} else {
    res.Outcome = OutcomeWrote
}
return res
```
This entire block must become the fallback path (ambiguous/no-scanner cases) plus the "run
`plan.Actions`" body of the new `would-write` case — the write-action loop itself
(`for _, action := range plan.Actions`) stays byte-identical; only what happens BEFORE it (the new
classification, which may `return` early) and the `res.Registered =` line AFTER it (D-02's
redaction-safe rebuild) change.

### The claude-code remove action this gate must never run on `preserved` (D-01/SC2)
```go
// Source: internal/setup/claudecode.go:87-93 (VERIFIED — read this session)
var claudeCodeRemoveAction = Action{
    Args:     []string{"claude", "mcp", "remove", "engram", "--scope", "user"},
    Tolerant: true,
    Description: "clear any prior registration (tolerant of \"not found\"); " +
        "if the following registration action fails or is interrupted, no " +
        "claude-code registration remains — re-run --apply to recover",
}
```
This is `plan.Actions[0]` for every claude-code auth mode (`claudecode.go:161-204`,
`[VERIFIED: internal/setup/claudecode.go:161-204]`). SC2 is proven by asserting this action's argv
never appears among the `runCall`s a `scriptedRun` fake recorded when the fixture's probe1 output
classifies `preserved`.

### The `scriptedRun` panic-on-overrun mechanism — the SC1/SC2 proof shape
```go
// Source: internal/setup/detect_test.go:56-70 (VERIFIED — read this session)
func scriptedRun(calls *[]runCall, results ...scriptedResult) func(context.Context, string, []string) (RunResult, error) {
    i := 0
    return func(_ context.Context, path string, args []string) (RunResult, error) {
        *calls = append(*calls, runCall{Path: path, Args: args})
        if i >= len(results) {
            panic(fmt.Sprintf("scriptedRun: called %d times, only %d result(s) scripted", i+1, len(results)))
        }
        r := results[i]
        i++
        return r.Result, r.Err
    }
}
```
Because `scriptedRun` **panics** if called more times than scripted, a fixture that scripts EXACTLY
one result (probe1, classifying `preserved`) and then calls `Apply` proves zero further `Run` calls
occurred (the test would panic-fail if the executor advanced to `claudeCodeRemoveAction`) — this is
the existing idiom `TestPreviewNeverExecutesWriteAction` (`apply_test.go:576-590`) already uses for
the analogous preview-side guarantee; the new SC1/SC2 test reuses the identical idiom for `Apply`.

### The plugin/skills independence from registration outcome — already shipped, needs only a proof test
```go
// Source: cmd/engram/setup.go:787-825 (VERIFIED — read this session)
func setupRuntimeRowFromResult(ctx context.Context, env setup.Environment, rt setup.Runtime, r setup.Result, headers string, mutate bool, includeContent bool) setupRuntimeRow {
    row := setupRuntimeRow{ /* ... registration fields from r ... */ }
    if r.Present {
        row.Headers = headers
        row.Registration = string(r.Outcome)

        var p setup.PluginResult
        if mutate {
            p = setup.PluginApply(ctx, env, rt, r.Binary, setupVersion())
        } else {
            p = setup.PluginPreview(ctx, env, rt, r.Binary, setupVersion())
        }
        outcome := setupApplyPluginFacet(&row, r.Outcome, p)
        if p.Delivered() {
            outcome = setupReportNativeSkills(&row, outcome, r.Skills)
        } else {
            outcome = setupApplySkillsFacet(&row, outcome, r.Skills, mutate, includeContent)
        }
        row.Outcome = string(outcome)
    }
    return row
}
```
Note: `PluginApply`/`setupApplySkillsFacet` are called unconditionally on `r.Present` — NOT on
`r.Outcome`. A `preserved` `r.Outcome` still reaches this same code, so plugin/skills already run.
This confirms SC1's second half ("while still applying its skills/plugin actions") requires **no
code change** — only a fixture test asserting it stays true once the registration write is gated.

### Man page naming — `engram-setup.1` (VERIFIED via cobra source, not assumed)
```go
// Source: github.com/spf13/cobra@v1.10.2/doc/man_docs.go:38-44 (VERIFIED — read this session;
// go.mod pins the exact same version: "github.com/spf13/cobra v1.10.2")
func GenManTree(cmd *cobra.Command, header *GenManHeader, dir string) error {
    return GenManTreeFromOpts(cmd, GenManTreeOptions{
        Header:           header,
        Path:             dir,
        CommandSeparator: "-",
    })
}
```
`cmd/engram/man.go`'s `writeManPages` calls the plain `doc.GenManTree(root, manHeader(), dir)`
`[VERIFIED: cmd/engram/man.go:87]`, which — per the quoted source above — hardcodes
`CommandSeparator: "-"`, NOT the package's own default of `"_"` (that default only applies to a
direct `GenManTreeFromOpts` call with no `CommandSeparator` set). So the generated file for the
`setup` subcommand is `engram-setup.1`, confirmed independently by `.goreleaser.yaml`'s cask
uninstall hook glob `Dir.glob("#{HOMEBREW_PREFIX}/share/man/man1/engram{,-*}.1")`
`[VERIFIED: .goreleaser.yaml:239]`, which only makes sense against dash-separated filenames.
`install.md` can therefore truthfully document `man engram-setup`.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Apply always writes, then byte-compares two reads to decide `wrote` vs. `already-correct` | Apply classifies BEFORE writing (for `DriftRuntime`s), skipping the write entirely on `already-correct`/`preserved` | This phase | `already-correct`/`wrote` become PRE-write claims for claude-code/codex; `preserved` is a NEW zero-write outcome under `--apply` (it already existed under preview since Phase 4). |
| `Result.Registered` after a write is the raw, bounded, quoted two-read capture | `Result.Registered` after a write is rebuilt from the redacted `Observation`, for `DriftRuntime`s | This phase (D-02) | Closes the last raw-capture site in `apply.go` (`displayCapture(probe2…)`); a header value an updated runtime CLI starts echoing back can no longer leak through this field. |

**Deprecated/outdated:**
- The `agent-setup.md` sentence "After `--apply` it means the observed state matches; it does not
  guarantee that no write ran" (for `already-correct`) — accurate for the shipped Phase 4 behavior,
  becomes FALSE for claude-code/codex once D-01 ships (for those two runtimes, `already-correct`
  under `--apply` NOW guarantees no write ran). The row must be corrected, not merely left as
  "still technically true for opencode/generic" — a reader cannot tell which runtime the caveat
  applies to without the correction naming them.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A new `Observation.RewriteConsequence` field (or, alternatively, extending `WholeEntryNote`'s text to cover the would-write case) is the right mechanism for the OAuth re-login note, keeping `apply.go`/`drift.go` content-blind. | "OAuth re-login consequence: design options" / Pattern 3 | Low — this is an implementation-shape recommendation, not a locked decision; CONTEXT.md explicitly leaves the "typed constant(s)" to Claude's discretion. If the plan picks a different mechanism that still satisfies AUTHORED-HERE and D-03/D-04's literal requirements, no rework of THIS document is needed. Flagged only so the planner does not have to re-derive the design space from scratch. |
| A2 | The manual-remediation hint (D-05: "the exact manual step... `claude mcp remove engram --scope user`; for Codex, the `[mcp_servers.engram]` table") is best delivered by extending the existing `claudeCodeWholeEntryNote`/`codexWholeEntryNote` constants rather than a new field. | Pattern 3 / Summary | Low — same reasoning as A1; both constants already exist and are already appended to a `preserved` row's `Reason`, so extending their text is the minimal-surface-area option, but a new field is equally valid under AUTHORED-HERE. |
| A3 | `internal/setupgen`'s generated `/engram-setup` prose does NOT need new preserved/apply-gate table rows — its three tables (delegation preview, claude-code fallback argv, claude-code plugin-delivery argv) render ARGV, not outcome classification, so there is nothing structurally new for them to show. | "Documentation gap inventory" below | Low-Medium — this is explicitly a "Claude's Discretion" item per CONTEXT.md; if the plan's checkpoint or the orchestrator decides the hand-authored prose SURROUNDING the anchored region should gain a sentence (not the generated table itself), that is a small doc edit, not a `setupgen.go` code change. Flagged so the planner doesn't default into unnecessary `setupgen.go` work. |

**If this table is empty:** N/A — see above; both items are explicitly delegated to plan-time
discretion by CONTEXT.md, not asserted here as fact.

## Documentation gap inventory

Verified by reading each guide's full current content this session.

### `install.md` (137 lines) `[VERIFIED: docs-site/src/content/docs/guides/install.md:1-137]`
- Zero occurrences of "plugin" anywhere in the file.
- Zero occurrences of "header" anywhere in the file.
- Zero occurrences of "man page"/"man1"/"engram man"/"engram-setup.1" anywhere in the file — the
  word "man" does not appear at all except inside unrelated words.
- One occurrence of "completion" (line 28: "generate bash, zsh, and fish completions") — the cask
  install-hooks paragraph exists but names only completions, not the (Phase 1-shipped) man pages the
  SAME hook now also writes (`.goreleaser.yaml:204-219` `[VERIFIED: .goreleaser.yaml:204-219]`).
- **D-07 requires this guide to cover:** what the cask installs (binary, completions, man pages —
  `man engram-setup`). All three man-page-related sentences are net-new content, not a correction.

### `agent-setup.md` (258 lines) `[VERIFIED: docs-site/src/content/docs/guides/agent-setup.md:1-258]`
- ALREADY documents the `preserved` outcome row with the whole-entry, never-merge, and
  never-shown clauses (line 193) — gated by `agent_setup_docs_test.go`'s `preservedRowPattern`
  legs 1-4. **No change needed to this row's CONTENT for REQ-apply-preserve-gate itself** — the row
  already describes preview's `preserved` classification correctly; it says nothing about
  `--apply`'s behavior on a `preserved` row today because `--apply` does not yet consult the
  classification.
- The `already-correct` row (line 192) says: "After `--apply` it means the observed state matches;
  it does not guarantee that no write commands ran." **This sentence must be corrected** once D-01
  ships — for claude-code/codex it becomes a pre-write guarantee (no write DID run). Recommend the
  row state the distinction per-runtime-class explicitly (drift-capable vs. not), since silently
  deleting the caveat would leave opencode/generic under-described (D-09/D-10's ambiguity path is
  unaffected by this phase and still performs the old write-then-compare).
- No mention anywhere of an "apply gate" or of `--apply` ever skipping a write action — this is
  entirely new content this phase must add (SC3/D-04's OAuth-consequence sentence, and D-01's
  general "declines to write" behavior for `preserved`).
- No mention of the OAuth re-login consequence — entirely new content (REQ-apply-rewrite-consequence).
- Already documents man pages? No — `agent-setup.md` does not cover installation at all (that is
  `install.md`'s job per D-07's split), so no change needed here for man pages.

### `plugin.md` (90 lines) `[VERIFIED: docs-site/src/content/docs/guides/plugin.md:1-90]`
- Already correctly scoped per D-07: describes what the plugin is/does and points to
  `agent-setup.md` for MCP registration ("See [Agent Setup](/guides/agent-setup/) for runtime
  support and credential requirements", line 54-55).
- No mention of `preserved`/apply-gate — likely does not need one given its scope (it is about the
  plugin/hooks, not the registration classification), but the plan should confirm this is
  intentional rather than an oversight, since the standalone-fallback section (`claude mcp remove
  engram --scope user` at line 76) is EXACTLY the manual-remediation command D-05 requires the
  `preserved` row to name — a one-sentence cross-link here ("if setup reports `preserved`, see
  Agent Setup's Read results section for what to do") would tie the two together at low cost.

### The docs-gate pattern to replicate (D-07)
`cmd/engram/agent_setup_docs_test.go`'s shape — read verbatim this session
`[VERIFIED: cmd/engram/agent_setup_docs_test.go:1-180]` — is: (1) a relative-path constant to the
guide, (2) one or more `regexp.MustCompile` patterns targeting specific table rows/lines, (3) a
`<guide>Violations(doc string) []error` pure function checked against BOTH the live file
(`Test<Guide>DocumentsX`, skips if file absent, fails if empty) AND an in-memory positive-control
fixture (`Test<Guide>DriftGateFiresOnInjectedViolation`, one subtest per violation class PLUS one
"clean" case that must produce zero violations). Two new files following this exact shape —
`install_docs_test.go` (man-page/plugin/header legs) and a `plugin_docs_test.go` or an extension of
`agent_setup_docs_test.go` itself (apply-gate/OAuth-consequence legs) — satisfy D-07's "gated like
Phase 4's `agent_setup_docs_test.go`" requirement.

## Open Questions

1. **Does the OAuth re-login note need its own `Observation` field, or can it reuse/extend
   `WholeEntryNote`?**
   - What we know: `WholeEntryNote` is currently ONLY surfaced on a `preserved` row's `Reason`
     (`apply.go:367-369` `[VERIFIED: internal/setup/apply.go:367-369]`); the OAuth note needs to
     surface on a `would-write` row's `Notes`, a different field and a different outcome.
   - What's unclear: whether reusing the SAME constant string (composed once, containing both the
     whole-entry warning and the OAuth caveat) and rendering it in different fields per-outcome is
     cleaner than a second, dedicated field that's empty except for claude-code+AuthNone.
   - Recommendation: the plan should decide explicitly and record the choice as a phase decision —
     CONTEXT.md's own discretion section confirms this is intentionally left open, not an oversight.

2. **Should `internal/setupgen`'s hand-authored prose (the text AROUND the anchored
   `<!-- engram:rule:start setup-commands -->` region in `skill/engram/commands/engram-setup.md`,
   NOT the generated tables themselves) gain a sentence about the apply gate / preserved outcome?**
   - What we know: the generated tables render ARGV only, never outcome text, so they need no
     structural change (see A3 above); the surrounding hand-authored prose already explains
     preview/apply/confirmation steps in some detail (`skill/engram/commands/engram-setup.md:116-134`
     `[VERIFIED]`).
   - What's unclear: whether a slash-command user needs to be told explicitly that `--apply` will
     never destroy a `preserved` registration, or whether that is adequately covered by the guide
     link.
   - Recommendation: low priority; a one-sentence addition is cheap if the plan-checker or orchestrator
     wants it, but is not required by any of the three phase requirements as literally stated.
</open_questions>

## Environment Availability

Skipped — this phase has no external runtime/service dependencies beyond what the existing
`internal/setup` package already requires (which Phase 3's own STATE.md entry already documents:
codex-cli, claude, opencode binaries; no NEW dependency is added by this phase, and no test may
invoke a real one — rule `m45p2b4bp7`, confirmed structurally by `internal/setup/detect_test.go`'s
`fakeEnv`/`fakeEnvWithRun` harness being the ONLY seam every existing Apply/Preview test uses).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go's built-in `testing` package (stdlib) — no third-party test framework in this repo |
| Config file | none — `go test` via `Taskfile.yaml`'s `test`/`test:go` tasks |
| Quick run command | `go test ./internal/setup/... ./cmd/engram/... -run '<TestName>' -v` |
| Full suite command | `task` (lint + test — `Taskfile.yaml:45-51`) or `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-apply-preserve-gate (SC1) | `--apply` against a pre-seeded `preserved`-classifying registration issues zero registration-write `Run` calls, while plugin/skills actions still run | unit (package-level, `internal/setup`) | `go test ./internal/setup/ -run TestApplyPreservedIssuesZeroWrites -v` | ❌ Wave 0 — new test, mirrors `TestPreviewNeverExecutesWriteAction` (`apply_test.go:576`) and `TestApplyConvergesClaudeCode` (`apply_test.go:420`)'s scripted-Run shape |
| REQ-apply-preserve-gate (SC1, process boundary) | Same guarantee proven through `cmd/engram`'s CLI entry point, including the plugin/skills facets still populating on the row | unit (process-boundary, `cmd/engram`) | `go test ./cmd/engram/ -run TestSetupApplyPreservedRuntimeSkipsRegistrationWrite -v` | ❌ Wave 0 — new test, mirrors `TestSetupJSONNeverLeaksProbeLiteral`'s `withFakeSetupEnv`/`scriptedSetupRun` harness (`cmd/engram/setup_test.go:1112`) |
| REQ-apply-preserve-gate (SC2) | Claude Code's `mcp remove` argv never appears among recorded `Run` calls for a `preserved`-classifying fixture | unit (package-level) | `go test ./internal/setup/ -run TestApplyPreservedNeverRunsClaudeCodeRemove -v` | ❌ Wave 0 — new test; assert on `runCall.Args` containing `["mcp","remove","engram",...]`, not merely on call COUNT (a count-only assertion could pass if some other action replaced the remove call) |
| REQ-apply-preserve-gate (already-correct pre-write) | `--apply` against an already-converged registration performs the SAME zero-write proof as `preserved` (both are pre-write skip cases under D-01) | unit (package-level) | `go test ./internal/setup/ -run TestApplyAlreadyCorrectIssuesZeroWrites -v` | ❌ Wave 0 — new test; distinct fixture from SC1's (converged vs. unreproducible) so both branches of the new `switch` are exercised |
| REQ-apply-preserve-gate (would-write still writes + re-run converges) | A `would-write`-classifying fixture still runs `plan.Actions` and reaches `wrote`; running `Apply` AGAIN against the post-write state reaches `already-correct` with zero further writes | unit (package-level) | `go test ./internal/setup/ -run TestApplyConvergesClaudeCode -run TestApplyConvergesCodex -v` (existing tests — must be UPDATED, not just left passing, since their scripted call counts will change once the pre-action classification consumes probe1 differently) | ✅ exists, needs update — `internal/setup/apply_test.go:33` (`TestApplyConvergesCodex`), `:420` (`TestApplyConvergesClaudeCode`) |
| REQ-apply-preserve-gate (D-02 redaction-safe re-observe) | `Result.Registered` after a real write is rebuilt via `Observe`/`renderObservation`, never raw `displayCapture` output, for a `DriftRuntime` | unit (package-level) | `go test ./internal/setup/ -run TestApplyWroteRegisteredIsRedacted -v` | ❌ Wave 0 — new test; mirror `TestThirdPartyCaptureIsQuotedForDisplay`'s hostile-string fixture (`apply_test.go:658`) but assert the field is the `renderObservation` shape (`url=... auth=... headers=...`), not merely "quoted" |
| REQ-apply-rewrite-consequence | A `would-write`-classifying Claude Code fixture whose observed auth is `AuthNone` carries the OAuth re-login sentence in BOTH `Preview`'s and `Apply`'s `Notes`/`Reason`; a `bearer`-shaped or codex fixture does NOT carry it | unit (package-level) | `go test ./internal/setup/ -run TestOAuthReLoginConsequence -v` | ❌ Wave 0 — new test; must include the Pitfall 4 negative cases (bearer-authenticated claude-code, codex) as explicit "must NOT contain" assertions, not only the positive case |
| REQ-docs-setup-v2 (install.md gate) | `install.md` documents man pages, plugin, and header content per D-07 | unit (docs-gate, `cmd/engram`) | `go test ./cmd/engram/ -run TestInstallGuideDocumentsSetupV2 -v` | ❌ Wave 0 — new file `cmd/engram/install_docs_test.go`, `agent_setup_docs_test.go`-shaped (violations func + live-file test + positive control) |
| REQ-docs-setup-v2 (agent-setup.md apply-gate legs) | `agent-setup.md` states the apply gate, the corrected `already-correct` semantics, and the OAuth re-login consequence | unit (docs-gate) | `go test ./cmd/engram/ -run TestAgentSetupGuideDocumentsDrift -v` (existing — extend `agentSetupGuideDriftViolations` with new legs) | ✅ exists, needs extension — `cmd/engram/agent_setup_docs_test.go:50` |
| REQ-docs-setup-v2 (plugin.md, if cross-link added) | Optional cross-link sentence, if the plan adds one | unit (docs-gate) | `go test ./cmd/engram/ -run TestPluginGuideCrossLinksAgentSetup -v` | ❌ Wave 0, optional — only if the plan decides to add the cross-link (see Open Question 2-adjacent minor item) |
| REQ-docs-setup-v2 (post-release, human observation) | `05-POST-RELEASE.md` checklist items observed on a real qualifying release: `brew install`, `engram setup` plugin-first + `--header` + `preserved` + apply gate, man pages present in the cask | **Manual-Only** (post-release, human) | N/A — no automated command; see `05-POST-RELEASE.md` | N/A — this is the D-06/D-10 pattern: the requirement's checkbox stays open (`post_release_status: pending`) until a human records `05-RELEASE-<ver>.md` after a real qualifying release, exactly as `06-RELEASE-0.16.0.md`/`06-VERIFICATION.md`'s `post_release_status: complete` did for the prior milestone. |

### Sampling Rate
- **Per task commit:** `go test ./internal/setup/... ./cmd/engram/... -run '<TestsTouchedThisTask>' -v`
- **Per wave merge:** `go test ./internal/setup/... ./cmd/engram/... -count=1` (full package, no `-run` filter)
- **Phase gate:** `task` (lint + full `go test ./...`) green before `/gsd-verify-work`; `go test
  ./internal/keylinks/ -count=1` immediately after plan-checker passes (STATE.md's Phase 1 gate,
  binding on every later phase); `docs-site` build/lint if the plan touches Astro content
  (confirm via `docs-site`'s own `package.json` scripts if the plan modifies frontmatter or links).

### Wave 0 Gaps
- [ ] `internal/setup/apply_test.go` — new tests: `TestApplyPreservedIssuesZeroWrites`,
      `TestApplyPreservedNeverRunsClaudeCodeRemove`, `TestApplyAlreadyCorrectIssuesZeroWrites`,
      `TestApplyWroteRegisteredIsRedacted`, `TestOAuthReLoginConsequence` (or equivalent names the
      plan settles on) — covers REQ-apply-preserve-gate and REQ-apply-rewrite-consequence at the
      package level.
- [ ] `cmd/engram/setup_test.go` — new test: `TestSetupApplyPreservedRuntimeSkipsRegistrationWrite`
      (process-boundary SC1/SC2 proof, following `TestSetupJSONNeverLeaksProbeLiteral`'s
      `withFakeSetupEnv`/`scriptedSetupRun` harness shape) — covers REQ-apply-preserve-gate at the
      CLI process boundary.
- [ ] `cmd/engram/install_docs_test.go` — new file, `agent_setup_docs_test.go`-shaped — covers
      REQ-docs-setup-v2's `install.md` leg.
- [ ] `cmd/engram/agent_setup_docs_test.go` — extend `agentSetupGuideDriftViolations` with new legs
      (already-correct correction, apply-gate statement, OAuth-consequence statement) plus matching
      new subtests in `TestAgentSetupGuideDriftGateFiresOnInjectedViolation`.
- [ ] `internal/setup/apply_test.go`'s EXISTING `TestApplyConvergesCodex`/`TestApplyConvergesClaudeCode`
      need their scripted-`Run`-call sequences and counts re-verified against the new pre-action
      classification's probe consumption — these are not gaps in coverage but MUST be re-run and
      possibly re-scripted once the mutate branch changes shape (their current 3-call/4-call counts
      assume the OLD unconditional-write sequence).
- [ ] No new test framework or fixture library install needed — `internal/setup/detect_test.go`'s
      existing `fakeEnv`/`scriptedRun`/`scriptedResult` harness already provides everything the new
      tests need.

## Security Domain

`security_enforcement` is not set in `.planning/config.json` (absent = enabled per the phase
contract), so this section is required.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | Yes (indirectly) | This phase does not implement authentication itself, but its central concern IS the consequence of a destructive action on an authenticated session (OAuth token invalidation via `claude mcp remove`) — the control is DISCLOSURE (D-03/D-04's typed note), not prevention, since the remove-then-add is a legitimate reproducible-difference case the milestone still allows to run. |
| V3 Session Management | Yes (indirectly) | Same reasoning as V2 — an OAuth session tied to a claude-code MCP registration is effectively invalidated by `mcp remove`; the control is informing the operator BEFORE the action runs (preview) and stating the fact again at the point of execution (apply), never silently. |
| V4 Access Control | No | This phase adds no new authz surface; it operates entirely on the local operator's own machine via CLIs the operator already controls. |
| V5 Input Validation | No new surface | `Options`/header validation is inherited unchanged from Phase 2 (D-02/D-03 there); this phase reads no new user input. |
| V6 Cryptography | No | No credential material is newly handled; the existing redaction discipline (`redactedValue`, `boundCapture`) is REUSED, never re-implemented. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Destructive overwrite of state the tool did not author and cannot reproduce (the `ryr82bf2s2` incident this milestone exists to close) | Tampering / (self-inflicted) Denial of Service | The `preserved` classification + apply-time gate (D-01) — this phase's entire purpose. Verified structurally: `Compare`'s existing exhaustive unit tests already cover every Phase 4 facet combination; this phase adds no NEW facet logic, only a NEW consumer of the existing, proven classification. |
| Silent loss of an authenticated session as a side effect of an unrelated "fix the registration" action | Tampering / Repudiation (the operator did not consent to re-authenticating) | The typed, always-rendered OAuth re-login note (D-03/D-04) — disclosed in BOTH preview and apply, never only as a post-hoc surprise. |
| Header/credential value leaking through a rendered report field via a re-probe read after a write | Information Disclosure | D-02's redaction-safe `Registered` rebuild via `Observe`/`renderObservation` (never raw `displayCapture` of probe2's output) — closes the LAST raw-capture site in `apply.go`, extending the redaction-by-construction discipline (`redactedValue`, unconditional per `drift.go`'s own doc comment) that Phase 4 already established for the preview lane. |
| A malicious or misbehaving third-party runtime CLI flooding a rendered field (`Notes`/`Reason`/`Registered`) from stdout/stderr | Denial of Service (operator-terminal/JSON-consumer flooding) | `boundCapture`/`maxCapturedBytes` — REUSED unchanged; any NEW rendered field this phase adds (the OAuth note, extended `WholeEntryNote` text) is engram-AUTHORED (a fixed constant), not third-party-derived, so it needs no NEW bound — but if the plan composes it adjacent to any observation-derived text (e.g., a facet name), that composition must still route through `boundCapture` at the SAME point Phase 4 already established (`apply.go:370-372`). |

## Sources

### Primary (HIGH confidence — read directly this session)
- `internal/setup/apply.go` (442 lines, full file read) — the executor this phase rewrites.
- `internal/setup/drift.go` (460 lines, full file read) — `Facet`, `Observation`, `DriftRuntime`,
  `Compare`, `renderObservation`, the D-01 predicate.
- `internal/setup/claudecode.go` (524 lines, full file read) — `claudeCodeRemoveAction`, `Observe`,
  `claudeCodeBearerForm`, `claudeCodeWholeEntryNote`.
- `internal/setup/codex.go` (553 lines, full file read) — `Observe`, whole-entry semantics,
  `codexWholeEntryNote`.
- `internal/setup/plan.go` (322 lines, full file read) — `Outcome`, `Result` doc comment (its own
  stated Phase-5 handoff), `Action`, `Plan`.
- `internal/setup/plugin.go` (448 lines, full file read) — the parallel plugin-delivery lane,
  confirming its independence from registration `Outcome`.
- `internal/setup/apply_test.go` (847 lines, full file read) — the existing Apply/Preview test
  shapes to extend.
- `internal/setup/detect_test.go` (partial read, lines 1-100) — `fakeEnv`/`scriptedRun`/
  `scriptedResult`/`fakeEnvWithRun` harness definitions.
- `internal/setup/exit.go`, `internal/setup/aggregate.go` (full files read) — `OutcomePreserved`
  placement in `Classify`/`precedenceOrder` (unchanged this phase, confirmed).
- `cmd/engram/setup.go` (partial reads, ~lines 100-300, 700-1068) — `setupRuntimeRowFromResult`,
  `setupApplySummary`, `setupApplyRun`, `setupLongDescription`, `setupApplySkillsFacet`,
  `setupApplyPluginFacet`.
- `cmd/engram/setup_test.go` (partial reads, ~lines 1100-1300) — `TestSetupJSONNeverLeaksProbeLiteral`,
  `TestSetupApplySummaryCountsPreserved` verbatim.
- `cmd/engram/agent_setup_docs_test.go` (180 lines, full file read) — the docs-gate pattern to
  replicate for `install.md`/further `agent-setup.md` legs.
- `cmd/engram/man.go` (156 lines, full file read) — `writeManPages`, `doc.GenManTree` call site.
- `internal/setupgen/setupgen.go` (296 lines, full file read) — the three generated tables and
  confirmation they render argv only, never outcome text.
- `skill/engram/commands/engram-setup.md` (full file read) — the generated region plus surrounding
  hand-authored prose.
- `docs-site/src/content/docs/guides/install.md`, `agent-setup.md`, `plugin.md` (full files read) —
  current content, confirming the documentation gaps stated above.
- `.planning/milestones/2026-08-23.01-phases/06-install-documentation/06-POST-RELEASE.md`,
  `06-RELEASE-0.16.0.md`, `06-VERIFICATION.md` (frontmatter + relevant sections read) — the D-10
  precedent shape (`post_release_status`, checklist structure, observation-record structure).
- `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` (first ~200 lines read) — the
  live-verified header-echo shapes, confirming the `x-litellm-api-key` gateway example and the
  `oauth-client` shape's absence.
- `.planning/phases/04-drift-detection-read-only/04-REVIEW-FIX.md`, `04-01-SUMMARY.md` (grepped +
  relevant sections read) — WR-01's fix and the "mutate branch pinned against `ff5a6f94`" claim.
- `.planning/phases/04-drift-detection-read-only/04-CONTEXT.md` (D-07 section read) — confirms
  `oauth-client` read-back is explicitly deferred to this phase.
- `go.mod` (header read) — Go 1.26.3, `github.com/spf13/cobra v1.10.2` pinned.
- `github.com/spf13/cobra@v1.10.2/doc/man_docs.go` (relevant functions read from the local module
  cache at the exact pinned version) — `GenManTree`'s `CommandSeparator: "-"`, confirming
  `engram-setup.1` naming.
- `.goreleaser.yaml` (grepped relevant lines) — the cask man-page install/uninstall hook and its
  `engram{,-*}.1` glob, corroborating the dash-separated naming independently.
- `Taskfile.yaml` (grepped) — `task`/`task test`/`task test:go` command shapes.
- `.planning/config.json` — `workflow.nyquist_validation: true` (Validation Architecture section
  required); no `security_enforcement` key (Security Domain section required, per absent=enabled).
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` (Phase 5 sections), `.planning/STATE.md`
  (Phase 4 learnings bullet) — read in full per the required-reading instructions.

### Secondary (MEDIUM confidence)
None — every claim in this document traces to a file read this session or to the CONTEXT.md
locked-decision text itself.

### Tertiary (LOW confidence)
None — this phase required no web search; it is an internal-codebase research task with an
already-vendored, version-pinned dependency (`cobra`) available for direct verification.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, confirmed by reading every touched file's imports.
- Architecture: HIGH — the pattern to follow (reuse the `!mutate` branch's sequence) is stated
  explicitly in both CONTEXT.md and the existing code's own doc comments; the only genuinely
  open design choice (OAuth-note mechanism) is flagged honestly as an assumption/open question,
  not asserted as settled.
- Pitfalls: HIGH — every pitfall listed is grounded in an explicit CONTEXT.md decision (D-01, D-03,
  D-05, D-09) or a structural property of the existing code (action ordering, `scriptedRun`'s
  panic-on-overrun), not speculation.
- Documentation gaps: HIGH — every claim about what a guide currently does/does not say is a direct
  grep/read result from this session, not an inference from a summary.

**Research date:** 2026-09-16
**Valid until:** Stable for the life of this phase (internal-code-reuse research, not
fast-moving external-ecosystem research) — re-verify only if Phase 4's shipped shape in
`internal/setup` changes before this phase's plan is executed, or if `spf13/cobra` is upgraded past
`v1.10.2` before the man-page documentation ships (re-check `CommandSeparator` behavior if so).
