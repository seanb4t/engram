# Project Research Summary

**Project:** engram — Setup v2 (milestone 2026-09-13.01)
**Domain:** Go CLI installer extension — plugin-first delivery, custom-header auth, drift detection/reconcile, cobra completions/manpages for an already-shipped multi-runtime `engram setup`
**Researched:** 2026-09-13
**Confidence:** HIGH — every claim in STACK/ARCHITECTURE/PITFALLS is grounded in a live `--help`/probe run against installed `claude` 2.1.270 and `codex-cli` 0.154.0 on this machine, or a direct read of this repo's own shipped `internal/setup/*.go`. MEDIUM on opencode (its CLI could not execute this session — AMFI/Gatekeeper killed it) and on Codex's plugin CLI maturity (best source is an in-flight upstream PR, no stable docs page).

## Executive Summary

This milestone widens an already-shipped, well-architected writer abstraction (`internal/setup`) rather than building anything from scratch. All four researchers converge on the same substrate: a `Runtime` interface with per-runtime `Plan()` methods that author argv shelled out to each runtime's own CLI, a five-value `Outcome` enum, and a shared content-blind executor. Every one of the five target features (plugin delivery, custom headers, drift detection, completions/manpages, the `osRun` fix) is additive to this substrate — new `Action`s appended to an existing `Plan.Actions` slice, a new `SkillFormat` value, a new `Options.Headers` field, a new `ObservedRegistration` parser per runtime — never a parallel structure or a new execution mechanism. Zero new Go dependencies are needed anywhere; `cobra/doc` is already an indirect dependency needing only promotion to direct.

Two corrections must be surfaced to the roadmapper before phases are cut, independently confirmed by all four researchers: (1) **shell completions are already fully shipped** (Phase 1, PR #515, hand-rolled cask hooks) — only man pages are new work, and PROJECT.md's own milestone text is stale on this point (it also wrongly claims a `generate_completions_from_executable` hook exists, which this repo deliberately rejected and tests against by occurrence count); and (2) **Codex's `mcp add` has no generic header flag** — only `--bearer-token-env-var` (fixed to `Authorization`), so a LiteLLM-style `x-litellm-api-key` header is structurally inexpressible on Codex today and must be declined explicitly (`ErrAuthModeUnsupported`), never coerced onto the wrong header name.

The dominant risk across the milestone is not any single feature's mechanics but a cross-cutting failure mode: **an `--apply` that "succeeds" while silently doing the wrong thing** — the exact shape of the 2026-09-10 incident this milestone exists to prevent. This recurs in at least four independent forms: (a) drift detection as scoped is preview-only text, but `--apply`'s write path must itself consult the same comparison or it re-creates the incident; (b) Claude Code's remove-then-add convergence mechanism is destructive and directly collides with "preserve an unreproducible registration" on the same runtime; (c) Codex's `mcp add` silently overwrites hand-edited TOML fields wholesale, so "reconcile" there can only mean skip-whole-runtime or overwrite-whole-runtime, never a partial merge; and (d) **a verified secret-leak vector**: both `claude mcp get` and `codex mcp get --json` echo literal HTTP header values in cleartext for registrations engram didn't write, so any drift-detection code that renders probe output into `--output json`, logs, or generated `/engram-setup` prose must redact header values unconditionally rather than trying to distinguish "safe-looking" references from literal secrets.

## Key Findings

### Recommended Stack

No new Go dependencies anywhere. Plugin management, custom headers, and drift probing are all argv/JSON interactions with `claude`/`codex`'s own CLIs — the same shell-out pattern `mcp add` already uses. `cobra/doc`'s `GenManTree` (for man pages) is already an indirect dependency (via cobra's own `go.mod`); promoting it to direct is metadata-only, following the `go.yaml.in/yaml/v3` precedent from the prior milestone.

**Core technologies:**
- `claude plugin` CLI (marketplace add/install/update/list --json) — complete, scriptable plugin lifecycle, zero new engram code needed to talk to it beyond new Plan/Action authorship
- `codex plugin` CLI (marketplace add/add/list/remove) — structurally parallel to claude's, but **no confirmed `--json`** on any subcommand and no `plugin validate` — text-output-only, lower report fidelity
- `claude mcp add -H/--header "Name: value"` — already fully general and repeatable; custom-header support for claude needs zero new CLI capability, only parameterizing the hardcoded header name
- `codex mcp get --json` — the one genuinely structured, reliable drift-comparison target across all three runtimes
- `cobra/doc` `GenManTree` — man-page generation, zero new dependency, mirrors the existing `completion` command's shape exactly

### Expected Features

**Must have (table stakes):**
- Detect plugin-CLI *capability* distinctly from binary-on-PATH (a runtime binary can predate its own `plugin` subcommand)
- Plugin delivery and native skills-copy delivery are mutually exclusive per runtime per run — never both, since duplicate/stale skills is the exact reported defect (backlog 999.5/999.6)
- Header **name** becomes a parameter (not hardcoded `Authorization`) for claude-code/opencode/generic; value stays an env-var reference, never a literal, on every runtime including the new shape
- Reject the `(runtime, header-name)` combination a runtime's CLI cannot express (Codex + non-Authorization name) the same way `oauth-client` is already rejected for opencode — never silently downgrade
- Three-way drift classification (identical / reproducible-diff / non-reproducible-so-preserved), never collapsing "differs" and "cannot reproduce" into one bucket
- Man pages generated from the live cobra command tree via a new hidden `engram man` command, installed/removed by the cask exactly like completions already are

**Should have (differentiators):**
- Naming *which specific facet* differs (URL vs. auth mode vs. header name vs. value-reference) rather than a bare "differs," reusing the same parsed comparison structure
- One `setup --apply` call transparently routing plugin-vs-skills-copy per runtime — no surveyed prior-art multi-runtime installer (getmcp, mx setup) has to make this choice at all

**Defer / explicitly decline:**
- Any TOML parser/writer to give Codex a custom-header capability its own CLI doesn't expose — decline cleanly via `ErrAuthModeUnsupported`, never spend the "zero new deps" budget here
- Write-time reconciliation (`Reconcile` hook merging a preserved header into a live write) — ship read-only drift reporting first as an independently shippable slice; defer the mutating half
- A generic "arbitrary key=value config passthrough" flag — reopens the parsed-third-party-config-format risk this repo has structurally avoided since v0.16.0
- Cursor support (`REQ-register-cursor`) — explicitly deferred by PROJECT.md, out of scope

### Architecture Approach

Every feature is a widening of the existing `internal/setup` substrate, never a parallel structure: plugin actions append to the same `Plan.Actions` slice the existing `mcp add`/`mcp remove` actions live in; a new `SkillFormatPlugin` value routes plugin-delivered runtimes to a no-op skills path (reusing `FormatNone`'s already-tested code path, semantically distinct so it's never silently collapsed); `Options.Headers []HeaderSpec` is additive to `Options`; a new `ObservedRegistration` struct + one parser per runtime (`parseClaudeCodeRegistration`, `parseCodexRegistration` via stdlib JSON) feed a single, centrally-auditable `compareRegistration` function; a new `OutcomePreserved` value is the one genuine vocabulary widening in the whole milestone, requiring touching `aggregate.go`'s precedence table, `exit.go`'s exhaustive switch, and `cmd/engram/setup.go`'s summary tally together, in one commit.

**Major components:**
1. `internal/setup/{claudecode,codex,opencode,generic}.go` — each runtime's own file authors its own plugin actions, header-rendering, and registration parser (AUTHORED-HERE discipline: never a shared cross-runtime formatting helper)
2. `internal/setup/observe.go` (new) — `ObservedRegistration` + `compareRegistration`, the one centralized place the "cannot reproduce → preserve" decision is made
3. `internal/setupgen/setupgen.go` — must regenerate `/engram-setup`'s prose in the SAME commit as any `Plan.Actions` change, enforced by the existing CI drift gate
4. `cmd/engram/man.go` (new) — hidden `engram man <dir>` command wrapping `doc.GenManTree`, invoked by the cask hook exactly like `completion` already is
5. `internal/setup/environment.go` (`osRun`) — three-line fix: check `ctx.Err()` before the `errors.As(*exec.ExitError)` branch, so a deadline-killed subprocess is classified as a seam error, not a clean nonzero exit

### Critical Pitfalls

1. **`--apply` has no opt-out from unconditional writes, and drift detection alone doesn't close it** — a preview-time-only comparison does nothing to stop `--apply`'s remove-then-add/overwrite sequence from running the instant it's invoked. Decide explicitly whether `--apply` itself consults the same comparison and refuses to replace an unreproducible registration — write a fixture test that runs `--apply` (not just preview) against a pre-seeded unreproducible registration and asserts zero write actions.
2. **Codex custom headers are structurally inexpressible for non-Authorization names** — `--bearer-token-env-var` hardcodes the header name; reusing it for `x-litellm-api-key` would silently write the wrong header while reporting success. Route any non-`Authorization` header name to `ErrAuthModeUnsupported` for Codex specifically, proven by a fixture test.
3. **Three incompatible per-runtime header syntaxes** (claude: `"Name: value"` colon-space; opencode: `KEY=VALUE` equals — the colon-space form was already live-reproduced to fail outright once and fixed) — a shared "format a header" helper would silently reintroduce the exact regression opencode already had fixed for bearer mode. Keep header rendering authored per-runtime file; test the literal separator character per runtime.
4. **Verified secret-leak vector in drift-detection probes** — both `claude mcp get` and `codex mcp get --json` echo literal header VALUES in cleartext for registrations engram didn't write (confirmed live on real production registrations this session). Redact header values unconditionally before storing/rendering probe output for comparison; never try to distinguish a safe-looking reference from a literal secret.
5. **Claude Code's remove-then-add convergence collides with "preserve"** — the existing convergence mechanism for claude-code IS destructive (no in-place update primitive exists), so "preserve an unreproducible registration" must gate whether the remove step runs at all, not just annotate a preview. Also: a forced remove-then-add on an OAuth-authenticated registration forces an unprompted re-login even when only a cosmetic field differed — surface the re-login consequence explicitly before the rewrite runs.
6. **Codex's `mcp add` silently overwrites hand-edited TOML wholesale** — no merge primitive exists; "reconcile" for Codex can only mean skip-whole-runtime (preserve) or overwrite-whole-runtime, never a partial preserve. Record this asymmetry explicitly as a Codex-specific limitation, distinct from claude-code's and opencode's resolutions.
7. **Plugin-first delivery introduces a new trust/consent surface** — `marketplace add` fetches and caches a marketplace definition before any plugin installs, and `-y`/non-interactive plugin install accepts "the displayed marketplace-declared command" (authored by the marketplace, not engram) sight-unseen. This machine's own live state (marketplace added, plugin not installed, no plain skills) proves this partial state is real, not hypothetical — test against it explicitly, and decide whether plugin install needs consent beyond the existing `--apply` gate.

## Implications for Roadmap

Researchers largely converge on a dependency order; the build order below follows ARCHITECTURE.md's explicit sequencing and FEATURES.md's MVP recommendation, reconciled where they diverge (ARCHITECTURE puts headers before plugin for merge-risk reasons; FEATURES puts headers before drift for the same reason — both agree headers must precede drift).

### Phase 1: `osRun` deadline fix (#560)
**Rationale:** Zero dependency on anything else in the milestone; smallest, independently shippable, lowest risk. All four researchers flag it as a natural first phase.
**Delivers:** A three-line fix in `internal/setup/environment.go` checking `ctx.Err()` before the `errors.As(*exec.ExitError)` branch, so a deadline-killed subprocess correctly reports a seam error instead of a clean `-1`/nil.
**Avoids:** Nothing new introduced; this is a correctness fix carried as tech debt (W01) from the prior milestone.

### Phase 2: Man pages
**Rationale:** Fully independent of every other item; small; zero interaction with `Options`/`Plan`. Land early as a second quick win.
**Delivers:** A new hidden `engram man <dir>` command wrapping `doc.GenManTree`, wired into `.goreleaser.yaml`'s existing `post.install`/`post.uninstall` hook pattern exactly like `completion` already is; `cobra/doc` promoted from indirect to direct in `go.mod`.
**Addresses:** `REQ-shell-completions-and-manpages` (the man-pages half only — shell completions require zero action, this is the scope correction to surface).
**Avoids:** Pitfall 11 (reintroducing `generate_completions_from_executable` or its silent-failure-swallowing equivalent for man pages; non-deterministic `GenManTree` output without `DisableAutoGenTag = true`).

### Phase 3: Custom auth headers
**Rationale:** Smallest, most mechanical change for three of four runtimes (parameter generalization of an already-shipped code path); directly fixes the reported real-world breakage (gotcha `ryr82bf2s2`); its parsed header-shape is a hard prerequisite for drift detection's comparison structure.
**Delivers:** `Options.Headers []HeaderSpec{Name, EnvVar}`, a new `--auth header`/`--header NAME=ENVVAR` CLI surface, per-runtime `case "header":` arms in `claudecode.go`/`opencode.go`/`generic.go`, and Codex's explicit `ErrAuthModeUnsupported` decline path for non-`Authorization` header names.
**Uses:** `claude mcp add -H`, opencode's `--header KEY=VALUE`, `generic.go`'s existing `Headers map[string]string`.
**Avoids:** Pitfall 2 (Codex header-name coercion), Pitfall 3 (shared cross-runtime header-formatting helper reintroducing the opencode colon-space regression).

### Phase 4: Plugin-first delivery
**Rationale:** Independent of headers but the highest complexity and the one item with a genuine external-capability confidence gap (Codex's plugin CLI maturity — no confirmed `--json`, best source an in-flight PR). Sequence after headers only to reduce merge risk in shared files (`claudecode.go`, `codex.go`), not because of a real dependency.
**Delivers:** Plugin-management `Action`s appended to `claudecode.go`/`codex.go`'s `Plan()`; a new `SkillFormatPlugin` value routing plugin-delivered runtimes to a no-op skills path; a new `skill/engram/.codex-plugin/plugin.json` manifest; additive `Plugin`-facet report-row fields; `internal/setupgen` regenerated in the same commit (hard, CI-gate-enforced dependency).
**Addresses:** Backlog 999.5, the plugin-first delivery target feature.
**Avoids:** Pitfall 8 (marketplace-add as an unconsented trust surface — test against the live "marketplace added, plugin not installed" partial state), Pitfall 9 (double-registration/orphaned-symlink risk — route to `SkillFormatNone`-equivalent, never `os.RemoveAll` a path without `os.Lstat` checking for a managed symlink first).

### Phase 5: Drift detection, read-only half
**Rationale:** Depends on headers (Phase 3) being stable, since the comparison surface must exist before it can be compared against; largest and riskiest single slice — do not start until Phases 1–4 are merged.
**Delivers:** `ObservedRegistration` struct + per-runtime parsers (Codex via stdlib JSON — the reliable case; claude-code via bounded text-prefix scans; opencode explicitly NOT parsed, a documented omission) + `compareRegistration` + a new `OutcomePreserved` value touching `aggregate.go`/`exit.go`/`cmd/engram/setup.go` together; preview-only surfacing, `--output json` opt-in.
**Implements:** The `ObservedRegistration`/`compareRegistration` architecture component (§4 of ARCHITECTURE.md).
**Avoids:** Pitfall 4 (secret-leak via probe echo — redact unconditionally), Pitfall 7 (never parse opencode's box-drawing table; Codex's `--json` is the only genuinely structured comparison target; claude-code stays a coarse whole-text comparison).

### Phase 6: Drift detection, reconcile half — recommend deferring or scoping tightly within this milestone
**Rationale:** Depends on Phase 5; the optional `Reconcile` hook and write-time header-merging is new surface the executor doesn't have today (`execute()` never branches on probe content by design). Ship read-only first; treat reconciliation as a deliberately separable follow-on.
**Delivers:** An optional `Runtime.Reconcile(observed, opts) (Plan, error)` hook, and — critically — `--apply`'s write path actually consulting the same comparison before writing (closing Pitfall 1, the incident's root cause).
**Addresses:** `REQ-setup-reconcile-hand-edits`.
**Avoids:** Pitfall 1 (apply-time gate, not just preview), Pitfall 5 (claude-code's remove-then-add must be gated by the preserve decision, not run unconditionally), Pitfall 6 (surface the OAuth re-login consequence before a claude-code rewrite runs), Pitfall 10 (Codex: skip-whole-runtime vs. overwrite-whole-runtime, no partial option — the first Codex reconcile fixture to write, since it's the least flexible CLI).

### Phase Ordering Rationale

- Headers before drift is a hard dependency (drift's comparison needs the header vocabulary to exist); headers before plugin is a soft one (shared-file merge-risk reduction only).
- Man pages and the `osRun` fix are fully independent of the other three features and of each other — sequenced early purely as low-risk, high-confidence quick wins that de-risk the milestone's velocity before the two hard slices (plugin, drift) land.
- Drift detection is deliberately split into read-only and reconcile halves because the reconcile half is genuinely new executor surface (probe-gated write branching) that the shipped design has never had, and because Pitfall 1 makes clear that the reconcile half — not the read-only half — is what actually closes the incident this milestone exists to prevent. If the milestone's scope needs trimming, the read-only half is the safe cut point; the reconcile half is the one that must not be silently dropped without an explicit decision, since "drift detection ships but `--apply` stays unconditional" reproduces the exact incident.

### Research Flags

Needs research/live-verification during phase planning (flagged MEDIUM confidence by researchers, not yet resolved):
- **Phase 4 (Plugin delivery):** Whether `codex plugin` exists as a stable surface at all, whether `claude plugin install`/`codex plugin` refuse-on-already-installed vs. silently overwrite, and whether the `.codex-plugin/plugin.json` manifest needs the richer `interface` block for a CLI-only install — all three need a live spike against the installed binaries before locking `Action.Args`.
- **Phase 5/6 (Drift detection):** What each runtime's read verb actually prints for a header whose value is NOT a bare `${VAR}`/`{env:VAR}` reference (Pitfall 4's live-verification prerequisite) — this session could not test it (STRICT: no mutating commands) and it must happen in-phase before comparison/rendering code is trusted.
- **opencode (all phases touching it):** The CLI could not be exercised live this session (Gatekeeper/AMFI kill) — re-verify header repeatability and any drift-relevant behavior live at implementation time on a machine where the binary runs.

Standard patterns (skip deep research-phase):
- **Phase 1 (`osRun` fix):** A three-line, fully-specified diff against engram's own code; no external CLI surface involved.
- **Phase 2 (Man pages):** Directly mirrors the already-shipped, already-tested `completion` command pattern; `GenManTree`'s signature is confirmed via `go doc` against the vendored cobra version.
- **Phase 3 (Custom headers) for claude-code/opencode/generic:** Parameter generalization of an already-shipped, already-tested code path — only the Codex sub-task needs a capability-gap decision, not deep research.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Live `--help`/probe verification against installed `claude` 2.1.270 and `codex-cli` 0.154.0 this session; MEDIUM only for opencode (unexecutable this session, carried from prior milestone's live verification) |
| Features | MEDIUM-HIGH | HIGH for claude plugin CLI, header syntax, Codex MCP flags (primary docs + shipped source); MEDIUM for Codex's plugin CLI (in-flight PR, not stable docs); explicit corrections surfaced (completions already shipped, Codex header gap) |
| Architecture | HIGH | Every claim cites a real file:line read against this repo's shipped code this session; MEDIUM only on exact third-party CLI syntax not yet live-verified (Codex plugin syntax, Codex custom-header capability) |
| Pitfalls | HIGH | Grounded in this repo's own shipped code and live, read-only `--help`/probe output; MEDIUM for opencode-specific claims (carried from prior milestone, not re-verified this session); one pitfall (secret-leak) independently verified live against real production registrations on this machine |

**Overall confidence:** HIGH — this milestone extends a substrate all four researchers read directly, with corrections cross-verified across independent research passes.

### Gaps to Address

- **Codex plugin CLI stability:** No `--json` confirmed on any `codex plugin` subcommand; best source is an in-flight upstream PR (#21396), not a released, documented feature. Treat Codex's plugin-delivery report fidelity as text-only until verified against the actual installed binary at implementation time.
- **opencode's live CLI behavior:** Unverifiable this session due to a Gatekeeper/AMFI kill unrelated to opencode's own release integrity. All opencode facts are carried forward from the prior milestone's live verification (1.18.20) plus this session's read of a real on-disk config file — re-verify at implementation time on a working machine.
- **Whether a non-bare-reference header value gets echoed by any runtime's read verb:** Directly required before drift-detection rendering code can be trusted (Pitfall 4); this session's STRICT no-mutation constraint prevented testing it, so it is a blocking prerequisite task for Phase 5, not an assumption to carry forward.
- **Consent/UX for plugin marketplace-add:** PROJECT.md is silent on whether a first-run plugin install needs an extra opt-in beyond `--apply`. This is a design decision the roadmap should surface explicitly to the user before Phase 4 execution, not resolve implicitly in code.
- **Whether `--apply` itself will be gated by drift detection, or whether that's deferred:** This is the single highest-stakes scope decision in the milestone (Pitfall 1) — the roadmap should make this an explicit, named requirement rather than letting it default to "preview-only" by omission.

## Sources

### Primary (HIGH confidence)
- Live `claude --version`, `claude plugin --help`, `claude plugin marketplace/install/update/list --help`, `claude mcp --help/get/list --help`, real `claude mcp get engram` output (redacted) — claude 2.1.270, this session
- Live `codex --version`, `codex plugin --help`, `codex plugin marketplace/add/list/remove --help`, `codex mcp --help/add/get/list --help`, real `codex mcp get engram --json` output (redacted) — codex-cli 0.154.0, this session
- Direct reads of `~/.codex/config.toml`, `~/.claude.json`, `~/.config/opencode/opencode.json` (redacted) — this session, ground truth for the reconcile-hand-edits motivating scenario (a literal, unresolved `x-litellm-api-key` header found live on this machine)
- Direct read of a real installed dual-target plugin (`~/.agents/plugins/fzymgc-house-skills/homelab/`) — ground truth for the `.codex-plugin/plugin.json` manifest shape
- This repo's own `internal/setup/{claudecode,codex,opencode,generic,plan,environment,apply,aggregate,exit}.go`, `cmd/engram/setup.go`, `internal/setupgen/setupgen.go`, `.goreleaser.yaml`, `cmd/engram/releaseconfig_test.go` — read directly this session
- `go doc github.com/spf13/cobra/doc GenManTree` against vendored cobra v1.10.2 — this session

### Secondary (MEDIUM confidence)
- code.claude.com/docs/en/plugins-reference, /discover-plugins — full claude plugin CLI syntax and `--json` version gating (v2.1.268+)
- litellm.ai/docs/mcp — the `x-litellm-api-key` gateway header shape this milestone must reproduce
- github.com/openai/codex/pull/21396 — Codex plugin/marketplace subcommand definitions (in-flight, not yet a stable release)
- opencode.ai/docs/mcp-servers/ — `--header KEY=VALUE` syntax, cross-checked against this repo's own carried-forward live verification

### Tertiary (LOW confidence)
- codex.danielvaughan.com blog posts — Codex plugin cache paths, corroborating detail only, single unofficial source
- github.com/openai/codex/issues/5180 — open feature request for custom headers, evidence of immaturity rather than a settled capability

---
*Research completed: 2026-09-13*
*Ready for roadmap: yes*
