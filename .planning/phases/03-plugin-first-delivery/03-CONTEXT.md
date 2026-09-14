# Phase 3: Plugin-First Delivery - Context

**Gathered:** 2026-09-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Under `--apply`, a plugin-capable Claude Code or Codex receives engram's skills, hooks, and
`/engram-setup` through its own plugin CLI — engram's own marketplace/plugin only — while a runtime
whose plugin CLI is absent or broken, and opencode / `generic` always, keep the native skills-copy
path. Delivery is mutually exclusive per runtime per run. Plugin state is reported as one of
absent / installed-but-outdated / installed-and-current, and plugin delivery is its own result facet
(text + JSON). `skill/engram/.codex-plugin/plugin.json` is added, release-please-synced, with a drift
gate keeping its identity fields equal to `.claude-plugin/plugin.json`. `/engram-setup`'s generated
prose reflects the plugin actions in the same change.

**Live facts (read-only probes, 2026-09-14, this machine):**
- Codex CLI **0.154.0 ships `codex plugin`** as a released surface: `add <PLUGIN[@MARKETPLACE]> --json`,
  `list --json [--marketplace NAME] [--available]`, `marketplace add|list|upgrade|remove`, `remove`.
  There is **no `update` subcommand**. This resolves the ROADMAP's live-verify concern about Codex.
- Claude Code **2.1.270**: `plugin install <plugin@marketplace> --json [--config k=v]`, `plugin update`,
  `plugin list --json`, `plugin marketplace add <github-repo|url|path> [--scope]`.
- This machine: Claude Code has marketplace `engram` (GitHub `seanb4t/engram`) and `engram@engram`
  **0.16.1** installed at `user` scope; Codex has **no** engram marketplace; `~/.agents/skills/` holds
  hand-made symlinks into Claude's marketplace clone (the "managed symlink" case REQ-plugin-skips-skills-copy names).

Out of scope: opencode plugin delivery (`REQ-plugin-opencode`, deferred); any removal/cleanup of
native skills or `AGENTS.md` blocks (report only — see D-08/D-09); drift comparison (Phase 4); the
apply-time preserve gate (Phase 5); Cursor.

</domain>

<decisions>
## Implementation Decisions

### Carried forward (decided 2026-09-13, engram `qy29m0j3d2` — do not re-ask)
- Plugin-first for Claude Code and Codex; **`--apply` is the only consent gate** — no plugin opt-in flag,
  no extra first-run consent (the ROADMAP's open question is answered: PROJECT.md's model is that
  `--apply` consents to everything setup writes, and a marketplace/plugin install is one of those writes).
- 3-way plugin state + facet naming are in scope; Cursor is deferred.

### Update & version semantics
- **D-01:** "Outdated" means the installed plugin version (from `plugin list --json`) is **SemVer-less-than**
  the binary's ldflags version. Equal → `current`. **Newer than the binary** (marketplace auto-updated ahead
  of a stale local binary) → reported `current` with a note (`plugin 0.17.0 is newer than this binary 0.16.1`);
  **never downgraded**. — **Reversibility:** reversible.
- **D-02:** Codex has no `plugin update`: an outdated engram plugin on Codex is updated by
  `codex plugin remove engram` **then** `codex plugin add engram@engram --json` — unambiguous and independent
  of `add`'s idempotency. Claude Code uses its native `claude plugin update engram@engram --json`.
- **D-03:** A dev / non-SemVer binary version (`dev`, `0.16.1-dev+sha`) **never triggers an update by
  comparison**; it installs only when the plugin is absent, and the row notes `current (dev build)`. A local
  build must not churn a real install.

### Marketplace source & scope
- **D-04:** When engram's marketplace is absent, `--apply` adds **GitHub `seanb4t/engram`, default branch,
  unpinned** — `claude plugin marketplace add seanb4t/engram`; Codex's `marketplace add` with the Git URL
  form its `--help` accepts (live-verify the exact argument shape at implementation, read-only). No ref
  pinning; currency is decided by D-01's version comparison, not by a ref.
- **D-05:** If a marketplace named `engram` already exists pointing **elsewhere** (fork, local path), setup
  **uses it as-is** — never re-points or replaces a source the operator configured — and the plugin facet
  reports the observed source so a fork stays visible.
- **D-06:** Claude Code plugin scope is **`user` only**, matching `mcp add --scope user` and the maintainer's
  existing install; no `--scope` flag is exposed. Codex has no scope concept.

### Existing native skills on the plugin path
- **D-07:** On the plugin path, setup adds **nothing** to the native skills directory (REQ-plugin-skips-skills-copy).
- **D-08:** Setup **never deletes** a native skills path — static copies from earlier `--apply` runs and
  hand-made symlinks alike. If engram skills are present there, the skills facet **reports** it, e.g.
  `native: 5 skills present at ~/.agents/skills (symlink) — remove manually to avoid duplicates`, with path,
  count, and symlink-vs-copy. — **Reversibility:** reversible (a future explicit prune verb can add deletion).
- **D-09:** An existing delimited engram index block in Codex's `AGENTS.md` is **left in place** and
  reported (`index block present`); no `AGENTS.md` write happens on the plugin path.

### Capability detection & fallback
- **D-10:** A present binary has a *working* plugin CLI iff **one read probe** — `<cli> plugin list --json` —
  exits 0 **and** parses as JSON. The same output is the 3-way-state input, so capability + state cost one
  subprocess per runtime (bounded by the existing 20s `execTimeout`). Non-zero exit, timeout, or unparseable
  output = no working plugin CLI.
- **D-11:** Marketplace presence is probed in **preview** too (`<cli> plugin marketplace list`, read-only),
  so the argv preview shows includes `marketplace add` **only when absent** (SC2: exact argv beforehand).
  Both CLIs print a text table here — parse as a **coarse name match only**, never as structure (the same
  posture as opencode's `mcp list`).
- **D-12:** When the plugin probe fails on a present runtime, registration and the **native skills copy
  proceed exactly as today**, and the plugin facet reads `unavailable: <reason>` (e.g.
  `plugin list --json exited 1`, `timed out after 20s`). The row's outcome is never failed by the probe
  (REQ-plugin-capability-detection: "never a failed runtime row").

### Claude's Discretion
- Exact `Plan` shape for plugin delivery: a new `SkillFormatPlugin` value on `SkillTarget` vs. plugin
  `Action`s in `Plan.Actions` plus a plugin-facet struct — research's ARCHITECTURE.md §4 sketches both;
  keep per-runtime authoring in each runtime's own file and the executor content-blind.
- The plugin facet's field names on `setupRuntimeRow` (flat scalars only — `TestOperatorViewFixturesHaveNoUnsanitizedNesting`)
  and how the facet folds into the exit taxonomy (a `failed` plugin install beside a `wrote` registration
  → `exitPartial` 8, like any other failed facet).
- SemVer parsing without a new dependency (`golang.org/x/mod/semver` if already in `go.sum` and
  maintained upstream — rule `xvqj44e5mk` — else a small stdlib parser); how `plugin list --json`'s
  `version` field is located for `engram@engram` on each CLI.
- Whether `claude plugin install`/`codex plugin add` refuse when already installed — live-verify with
  read verbs and `--help` only; the D-01/D-02 state machine never issues an install for a `current` plugin,
  so refusal semantics matter only for the `absent` path.
- `.codex-plugin/plugin.json` contents beyond the identity fields (`name`, `version`, `description`), whether
  Codex's loader needs an `interface` block for a CLI-only install (live-verify against `codex plugin
  add --help` / docs, read-only), and the drift gate's shape (a Go test comparing the two manifests'
  identity fields + a `release-please-config.json` `extra-files` entry for the new manifest).
- Where the plugin probe results live for Phase 4's drift comparison to reuse.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — Phase 3 section: goal, five success criteria, the live-verify list.
- `.planning/REQUIREMENTS.md` — `REQ-plugin-capability-detection`, `REQ-plugin-install-or-update`,
  `REQ-plugin-three-way-state`, `REQ-plugin-skips-skills-copy`, `REQ-plugin-facet-reported`,
  `REQ-codex-plugin-manifest`, `REQ-plugin-setupgen-regenerated` (lines 51–57); `REQ-plugin-opencode` is deferred (line 69).
- `.planning/PROJECT.md` §"Current Milestone: 2026-09-13.01" (plugin-first bullet) and §"Out of Scope".

### Research (this milestone)
- `.planning/research/FEATURES.md` — plugin-CLI table (`--json` on Claude's subcommands; Codex's
  `plugin marketplace add`/`plugin add`/`list`/`remove` via openai/codex PR #21396 — now confirmed released
  in 0.154.0), anti-features (no install without `--apply`; never reinstall a marketplace-managed plugin;
  never belt-and-suspenders native copies).
- `.planning/research/ARCHITECTURE.md` §4 — `Plan.Actions` plugin actions, `SkillFormatPlugin`, the
  `setupRuntimeRow` skills-facet fields to mirror; §"live-verify" caveats.
- `.planning/research/PITFALLS.md` — plugin-related pitfalls.

### Prior-milestone precedent (skills distribution)
- `.planning/milestones/2026-08-23.01-phases/04-skills-distribution/04-CONTEXT.md` — D-05 (per-runtime
  `SkillTarget` authored in the runtime's own file), D-15 (never guess at a region engram did not author),
  D-16 (symlink rationale), the `codex-native-plus-index` routing decision.
- `.planning/milestones/2026-08-23.01-phases/04-skills-distribution/04-05-SUMMARY.md` — #559: only
  confirmed nonexistence is the `AGENTS.md` create case.

### Manifests & release plumbing
- `skill/engram/.claude-plugin/plugin.json` — the identity fields the `.codex-plugin` twin must match.
- `.claude-plugin/marketplace.json` — marketplace `engram`, plugin source `./skill/engram`.
- `release-please-config.json` `extra-files` — the `$.version` sync entry to duplicate for
  `skill/engram/.codex-plugin/plugin.json`.

### Durable memory (engram spine)
- `qy29m0j3d2` — the milestone decision (plugin-first, `--apply` is the only consent gate, 3-way state).
- `ryr82bf2s2` — never verify `--apply` against the operator's real `$HOME`; use fake seams.
- `xjz60c9h6t` — Phase 1 orchestration traps (key_links bracket escaping, red-evidence registration,
  isolation sentinel).
- Rule `m45p2b4bp7` — never gate third-party behavior (no CLI version tables); rule `xvqj44e5mk` — prefer
  established OSS over hand-rolled, verify upstream before adopting a `go.sum` module.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/setup/plan.go:67-90,148-169` — `SkillFormat` (`none`/`native`/`agents-md`), `SkillTarget`,
  `Plan{Runtime, Actions, Probe, Config, Skills}`; `Action{Args, Tolerant, Description}` (`:93-116`) is
  already a generic argv + tolerance + label — plugin actions fit without a new mechanism.
- `internal/setup/apply.go` — `execute()` runs Probe → Actions → Probe, converts `Plan()` errors to
  `failed` rows, `runSeam` bounds every subprocess at 20s; Phase 1's `osRun` ctx classification.
- `internal/setup/claudecode.go`, `codex.go` — per-runtime `Plan()` arms (Phase 2 added header rendering
  and the Codex decline; extend, don't fork).
- `internal/skills/install.go:44-71` — `Target`, `Install(env, target, skills) Report`; the native path
  this phase must **not** invoke for a plugin-delivered runtime.
- `cmd/engram/setup.go:370-375` — the seven flat skills-facet row fields (`Skills`, `SkillsDest`,
  `SkillsIndex`, `SkillsDigest`, `SkillsBytes`, `SkillsContent`) — the plugin facet mirrors this shape;
  `:559-561` `exitPartial`/`exitSetupFailed` routing; `client_common.go:220+` exit taxonomy.
- `internal/setupgen/setupgen.go` — `Cases()` (now 5 cases) + `Render()` behind the CI drift gate
  (`go run ./internal/surfacesgen --check-setup`).
- `cmd/engram/root.go:19` `version` (ldflags) — D-01's comparison operand.

### Established Patterns
- Every runtime writer shells out to the runtime's own CLI; no third-party config file is parsed or written.
- Preview may run read verbs (`mcp get`, now `plugin list --json`, `marketplace list`); `--apply` runs writes.
- Text-table CLI output is compared coarsely (opencode `mcp list`), structured JSON is parsed.
- Result rows are flat scalars; every new facet gets `omitempty` JSON and a `viewRow` line.
- Tests use `withFakeSetupEnv` / fake `Environment` scripting `(path,args) → (RunResult, error)`; no real CLI.
- Regenerated artifacts land in the same commit as their cause; key_links use bracket escaping; red-evidence
  is registered by the orchestrator after the last plan.

### Integration Points
- `internal/setup/{claudecode,codex}.go`: plugin probe + marketplace probe + install/update/remove-add actions
  authored per runtime; opencode/generic untouched.
- `internal/setup/plan.go`: plugin facet vocabulary (state enum absent/outdated/current, `unavailable`).
- `cmd/engram/setup.go`: skip `skills.Install` for plugin-delivered runtimes; D-08/D-09 presence report;
  new flat row fields; exit taxonomy.
- `skill/engram/.codex-plugin/plugin.json` (new) + `release-please-config.json` + a Go drift test.
- `internal/setupgen` + `skill/engram/commands/engram-setup.md` regeneration; `docs-site/.../guides/plugin.md`
  and `agent-setup.md` (docs close-out is Phase 5, but the prose regen is this phase's REQ).

</code_context>

<specifics>
## Specific Ideas

- Preview text for the maintainer's own machine today: Claude Code → `plugin: current (0.16.1 == 0.16.1)`,
  skills facet `native: 0 files (plugin-delivered)`; Codex → `plugin: absent → marketplace add + add`,
  skills `native: 5 skills present at ~/.agents/skills (symlink) — remove manually to avoid duplicates`.
- Argv shapes to author (live-verify exact forms read-only): Claude Code
  `claude plugin marketplace add seanb4t/engram`, `claude plugin install engram@engram --json`,
  `claude plugin update engram@engram --json`; Codex `codex plugin marketplace add <git URL>`,
  `codex plugin add engram@engram --json`, `codex plugin remove engram` + `add` for update.
- Negative-space tests: a plugin-delivered runtime's Plan authors **zero** native skills writes and **no**
  `AGENTS.md` action; a `current` plugin authors **zero** plugin actions; a failed probe authors the full
  native path and the facet string contains the probe's exit/timeout reason.

</specifics>

<deferred>
## Deferred Ideas

- A future explicit `engram setup --prune-native`-style verb to remove engram-managed native copies and
  the `AGENTS.md` index block after plugin delivery (D-08/D-09 report-only this milestone).
- `REQ-plugin-opencode` — opencode plugin delivery if/when opencode ships a plugin CLI.
- Ref-pinned marketplace installs (`seanb4t/engram@vX.Y.Z`) — rejected for now (D-04); revisit if
  version skew between marketplace HEAD and released binaries becomes a support issue.

</deferred>

---

*Phase: 03-plugin-first-delivery*
*Context gathered: 2026-09-14*
