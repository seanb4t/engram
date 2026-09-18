# Phase 5: Apply-Time Preserve Gate & Documentation - Context

**Gathered:** 2026-09-16
**Status:** Ready for planning

<domain>
## Phase Boundary

`--apply` consults Phase 4's drift classification **before** writing and performs zero
registration-write actions against a `preserved` registration — including never running Claude
Code's tolerant `mcp remove` — while still applying that runtime's skills/plugin lanes. This closes
the root cause of the 2026-09-10 overwrite incident (gotcha `ryr82bf2s2`). A reproducible rewrite
on Claude Code that would discard an OAuth login says so upfront in both lanes. The three guides
(`install.md`, `agent-setup.md`, `plugin.md`) are brought current with everything this milestone
shipped — plugin-first delivery, the header shape, `preserved` + the apply gate, man pages — and
the docs requirement is closed out by a post-release live observation (the `2026-08-23.01` D-10
pattern), never from code alone. This phase owns the `mutate == true` branch of `execute()` that
Phase 4 deliberately left byte-identical. No new flags, no new consent surface, no new runtime.

</domain>

<decisions>
## Implementation Decisions

### Carried forward (decided in earlier phases — do not re-ask)
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

### The apply lane
- **D-01:** `--apply` runs the same `Observe → Compare` as preview **before any action** for a
  runtime implementing `DriftRuntime`: `already-correct` → zero registration actions, outcome
  `already-correct`; `would-write` → run `Plan.Actions` → `wrote`; `preserved` → zero registration
  actions (Claude Code's `mcp remove` included), outcome `preserved`. The post-write byte-compare
  is **retired for drift-capable runtimes**. Runtimes without a scanner (opencode) and ambiguous
  reads keep today's write-then-byte-compare path unchanged (D-09/D-10). Re-running setup on a
  converged Claude Code install is a true no-op — no remove-then-add, no OAuth logout.
  — **Reversibility:** costly — `wrote`/`already-correct` become pre-write claims for parsed runtimes;
  the results table, the generated `/engram-setup` prose, and the exit taxonomy's documented
  meaning of `already-correct` ("does not guarantee no write ran") all change with it.
- **D-02:** After a `would-write` write succeeds, the executor **re-probes once** and rebuilds
  `Result.Registered` through the same `Observe → redact → render` path preview uses, so the row
  shows the NEW registration (redacted) and the last raw-capture site in `apply.go`
  (`displayCapture(probe2…)`) is closed. The outcome stays `wrote` regardless of what the re-observe
  says — a post-write read is never used to claim `already-correct` or to fail the row (D-09).

### The OAuth re-login consequence (REQ-apply-rewrite-consequence)
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

### The `preserved` escape hatch
- **D-05:** **No new flag.** engram never destroys what it cannot reproduce. The `preserved` row's
  reason names the facet(s) AND the exact manual step to clear the registration in the runtime's own
  tool (`claude mcp remove engram --scope user`; for Codex, the `[mcp_servers.engram]` table in
  `config.toml`), after which a re-run reads `would-write`. The human performs the destructive step
  with the runtime's own confirmation semantics. `--replace-registration` was considered and
  rejected as a second consent surface that re-opens the incident class if it lands in a script.

### Docs + post-release closeout (REQ-docs-setup-v2)
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

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements
- `.planning/ROADMAP.md` §"Phase 5: Apply-Time Preserve Gate & Documentation" — goal and four
  success criteria
- `.planning/REQUIREMENTS.md` — `REQ-apply-preserve-gate`, `REQ-apply-rewrite-consequence`,
  `REQ-docs-setup-v2`
- `.planning/PROJECT.md` — Key Decisions table (Phase 4 rows), "json is the contract"

### Phase 4 (hard dependency — the classification this gate consumes)
- `.planning/phases/04-drift-detection-read-only/04-CONTEXT.md` — D-01..D-12
- `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` — observed read-verb shapes;
  the `oauth-client` shape is unobserved
- `.planning/phases/04-drift-detection-read-only/04-01-SUMMARY.md` — `DriftRuntime`, `Observation`,
  `Compare`, and the explicit "mutate branch is Phase 5's" handoff
- `.planning/phases/04-drift-detection-read-only/04-05-SUMMARY.md` — claude-code `Observe`
- `.planning/phases/04-drift-detection-read-only/04-REVIEW-FIX.md` — WR-01 `boundCapture` rule

### Code contracts
- `internal/setup/apply.go` — `execute()`: the `!mutate` branch (Phase 4) and the `mutate` branch
  this phase rewrites; `describeSeamError`/`describeFailure`/`toleratedNote`
- `internal/setup/drift.go` — `Facet`, `Observation`, `DriftRuntime`, `Compare`, `renderObservation`
- `internal/setup/claudecode.go` — `claudeCodeRemoveAction` (tolerant remove + its Notes-bound
  Description), `Observe`, `claudeCodeBearerForm`
- `internal/setup/codex.go` — `Observe`, whole-entry semantics
- `internal/setup/plan.go` — `Outcome`, `Result.Notes` (tolerant-action consequences), `Result.Registered`
- `internal/setup/exit.go`, `aggregate.go` — `OutcomePreserved` placement (unchanged here)
- `internal/setup/apply_test.go` — `fakeEnvWithRun`/`scriptedRun` harness
- `cmd/engram/setup.go` — `setupRuntimeRowFromResult`, `setupApplySummary`, `setupLongDescription`
- `cmd/engram/agent_setup_docs_test.go` — the docs-gate shape to replicate for `install.md` and `plugin.md`
- `cmd/engram/man.go` — `engram man <dir>` (Phase 1), wired in the cask hook (`.goreleaser.yaml`)
- `internal/setupgen/` — generated `/engram-setup` prose + drift check (`go run ./internal/surfacesgen --check-setup`)

### Docs surface
- `docs-site/src/content/docs/guides/install.md`, `agent-setup.md`, `plugin.md`

### The D-10 precedent (post-release closeout)
- `.planning/milestones/2026-08-23.01-phases/06-install-documentation/06-CONTEXT.md` D-10
- `.planning/milestones/2026-08-23.01-phases/06-install-documentation/06-POST-RELEASE.md` — handoff shape
- `.planning/milestones/2026-08-23.01-phases/06-install-documentation/06-RELEASE-0.16.0.md` — observation shape
- `.planning/milestones/2026-08-23.01-phases/06-install-documentation/06-VERIFICATION.md` — `post_release_status` frontmatter

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Observe`/`Compare`/`renderObservation` and the `DriftRuntime` type assertion already exist in
  `execute()`'s `!mutate` branch — the apply lane reuses them, it does not re-derive them.
- `Result.Notes` already carries tolerant-action consequences (`claudeCodeRemoveAction.Description`)
  — the OAuth re-login note lands beside it through the same field.
- `agent_setup_docs_test.go` is the docs-gate template; `06-POST-RELEASE.md` / `06-RELEASE-0.16.0.md`
  are the closeout templates.
- `TestSetupJSONNeverLeaksProbeLiteral` and `TestRedactionUnconditional` extend to the apply lane's
  post-write re-observe.

### Established Patterns
- Executor stays content-blind: per-runtime strings (remediation hints, consequence notes) are
  authored in the runtime's own file; the shared executor only sequences.
- Ambiguity never converges: an unparseable read or a runtime without a scanner takes the
  write-then-byte-compare path exactly as today.
- Every rendered `Result` field is a bounded flat scalar string.
- No test invokes a real CLI or touches `$HOME` (rule `m45p2b4bp7`); the D-10 live observation is
  a human handoff, not a test.

### Integration Points
- `execute()`'s `mutate == true` branch (`apply.go` ~L376–440): insert the pre-action
  classification; short-circuit on `already-correct`/`preserved`; post-write re-observe on
  `would-write`; keep the byte-compare only when `!isDrift` or `Observe` failed.
- `cmd/engram/setup.go`: `setupApplySummary` already counts `preserved` (Phase 4); `--help` gains
  the apply-gate sentence.
- Docs: three guides + three gates; `05-POST-RELEASE.md` + `post_release_status: pending` in
  `05-VERIFICATION.md`.

</code_context>

<specifics>
## Specific Ideas

- The acceptance the user cares about: a pre-seeded Claude Code registration carrying
  `x-litellm-api-key` (the real gateway shape) under `--apply` yields a `preserved` row, **zero**
  `claude mcp remove`/`add` invocations recorded by the fake environment, the plugin/skills lanes
  still executed, exit 0 — and a re-run of a converged install issues zero `mcp` write calls.
- The re-login note should read like the existing tolerant-remove note: a consequence stated once,
  on the row, before anything runs.

</specifics>

<deferred>
## Deferred Ideas

- `--replace-registration` (an explicit overwrite flag) — considered and rejected for this milestone;
  revisit only with its own consent design.
- A stderr pre-write warning line or an interactive pause in `--apply` — rejected; the row note is
  the single rendering path.
- Observing the `oauth-client` read-back shape — not needed under D-03 (shape-based detection);
  capture opportunistically in `05-POST-RELEASE.md` if convenient.

</deferred>

---

*Phase: 05-apply-time-preserve-gate-documentation*
*Context gathered: 2026-09-16*
