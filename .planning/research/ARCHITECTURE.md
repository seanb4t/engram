# Architecture Research: `engram setup` and Multi-Runtime Distribution

**Domain:** Go CLI subcommand + local-machine config-file writers, integrating into an existing cobra binary with a pinned command catalog.
**Milestone:** `2026-08-23.01` — Distribution & Agent Bootstrap.
**Researched:** 2026-08-23
**Confidence:** HIGH — every claim below is grounded in files read in this repository, cited by `file:line`. Sections that go beyond what the codebase already answers (multi-runtime partial-failure exit semantics, exact third-party config formats for Codex/Cursor/opencode) are flagged as open design questions rather than asserted.

## 1. Where the code lives

### The established leaf-package convention

`cmd/engram/` is entrypoint-only by written convention (`CLAUDE.md` "Layout" table) and by observed practice: every cobra command file (`spine_review.go`, `migrate.go`, `reindex.go`, ...) is a thin `cobra.Command` wrapper whose `RunE` delegates into `internal/*`. Four packages in `internal/` are **stdlib-only, single-purpose leaf packages** added by the last three milestones, each with a `*purity_test.go` or equivalent gate proving no non-stdlib import:

- `internal/surfaces` — declares conditional rules + the MCP/CLI blast-radius table (`internal/surfaces/toolclass.go`).
- `internal/migrate` — the ordered step registry (`internal/migrate/registry.go:27`, `Registry = []Step{...}` — **must** stay a package-level var literal, `internal/migrate/registry.go:13-24`).
- `internal/openaiurl` — shape-aware URL join, shared by two unrelated callers with no back-edge.
- `internal/keylinks` — pattern-matchability gate.
- `internal/migrate/leafpurity_test.go` is the concrete pattern: it parses `go.mod` for the module path and AST-walks the package's own imports to prove no cross-boundary dependency crept in.

**Recommendation: `internal/setup/`**, following this exact shape — stdlib-only (the constraint is absolute here: file I/O, JSON/text templating, `os/exec` for shelling out to `claude mcp add`, and `go:embed` are all stdlib), with:

| File | Responsibility |
|------|-----------------|
| `internal/setup/runtime.go` | The `Runtime` interface (§2) and the package-level `Runtimes = []Runtime{...}` registry var — mirrors `migrate.Registry`'s shape and its "must be a package-level literal" discipline, for the same reason: a runtime silently unregistered because it lives behind a lazy getter is a worse failure than a compile-time-visible list. |
| `internal/setup/plan.go` | `Plan`, `Action`, `Outcome` types — the diff a runtime computes before writing anything (§5). |
| `internal/setup/apply.go` | The atomic-write + backup primitives shared by every writer (§5) — one implementation, not one per runtime. |
| `internal/setup/claudecode.go`, `codex.go`, `cursor.go`, `opencode.go`, `genericmcp.go` | One `Runtime` implementation per file — **not** subpackages. Reasoning in §2. |
| `internal/setup/detect.go` | Shared detection helpers (PATH lookup, config-dir existence) reusable across writers. |

`cmd/engram/setup.go` is the thin entrypoint: flag registration, calling `setup.Detect()` / `setup.Plan()` / `setup.Apply()`, and rendering through the existing typed operator-view renderer (`cmd/engram/operator_view.go:1-14`, `viewFields` walks the **marshaled JSON bytes**, not the Go struct, so `--output text` and `--output json` derive from one serialization — the 2026-08-12.01 "operator renderer typed" precedent, `PROJECT.md` line ~55). `engram setup`'s report struct should be a plain struct/slice-of-struct so it falls out of `viewFields` for free — no new per-command renderer code, matching the established convention.

### Why not subpackages per runtime

Two structural reasons, both grounded in what already exists:

1. **The `Runtime` interface must live somewhere both the registry and every implementation can see without an import cycle.** Keeping implementations in the same package as the interface (as `internal/migrate` does with its one step in `v1_step.go` alongside `step.go`) avoids inventing an `internal/setup/runtimes/` parent package purely to hold the interface — one package, `internal/setup`, holds interface + registry + all implementations, exactly like `internal/migrate` holds `Step` + `Registry` + `v1_step.go` together.
2. **"A new runtime gets added later without touching the others" (the design goal from item 2) is satisfied by file-count, not package-count.** Adding `internal/setup/newruntime.go` plus one line in the `Runtimes` slice is the same shape as `internal/migrate`'s "append a new step" story — no other file changes. Subpackages would need `internal/setup` to know their concrete types anyway (Go has no dynamic plugin loading without cgo, which is out per the zero-new-dependency constraint and `CGO_ENABLED=0` in `.goreleaser.yaml:22`), so subpackages would buy nothing but import-cycle risk.

### Skill content: a new asset package, not a copy

The milestone's "Skills distribution" target requires the binary to carry the five skills' content (`skill/engram/skills/*/SKILL.md`) so a brew-installed binary with **no** Claude plugin present can still write native-format skill files or AGENTS.md-appended fallback text. `go:embed` patterns are relative to the embedding file's own directory and cannot reach outside it (`../../skill/...` is not embeddable from `internal/setup`). Two ways to satisfy this without a new dependency:

- **Recommended:** add a small companion Go file directly under `skill/engram/`, e.g. `skill/engram/assets.go` (`package skillassets`) with `//go:embed skills/*/SKILL.md`, imported by `internal/setup`. This keeps the markdown as the **single authored copy** — no duplication, no build-time copy step, no drift between "the plugin's skill" and "the binary's embedded skill". `skill/engram/` currently holds no `.go` files (confirmed: `find skill/engram -type f` shows only `.claude-plugin/`, `commands/`, `hooks/` Python, `skills/*/SKILL.md`), so this is a new component, not a modification of an existing one.
- Rejected: copying `skill/*/SKILL.md` into `internal/setup/embedded/` at build time. This works but needs a `go:generate` step or a `task` target to keep the copy fresh, reintroducing exactly the "two copies that can silently diverge" problem the milestone explicitly wants to avoid for the prose/CLI equivalence (§3) — no reason to accept that risk here too when a same-tree `go:embed` avoids it entirely.

## 2. The runtime-writer abstraction

### What varies vs. what stays common

| Axis | Claude Code | Codex | Cursor | opencode | Generic MCP client |
|------|-------------|-------|--------|----------|---------------------|
| Config format | Plugin marketplace JSON + `claude mcp add` CLI writes its own config | (research needed at implementation time — likely TOML `~/.codex/config.toml`) | JSON `.cursor/mcp.json`-shaped | (research needed) | Portable MCP JSON, no fixed schema owner |
| Config location | User-scope, resolved by the `claude` CLI itself | Runtime-specific dotfile | Project or user scope | Runtime-specific | Wherever the user points it |
| CLI exists to do the write? | **Yes** — `claude mcp add --transport http engram <url> --scope user ...` (`skill/engram/commands/engram-setup.md:41-45`, already the sanctioned "never hand-edit settings files" path, line 11-13) | Unknown, verify at implementation | Unknown, verify | Unknown, verify | No — hand-write |
| Native skill support | Yes (plugin marketplace + `SKILL.md`) | No — AGENTS.md fallback | No — AGENTS.md fallback | No — AGENTS.md fallback | N/A |

Given this spread, the interface must not assume a config format, a write mechanism (file vs. shell-out), or skill-format support. It should only assume: *presence is detectable*, *a desired-state diff can be computed without mutating anything*, and *applying a diff either writes files or shells out*.

```go
// internal/setup/runtime.go

// Runtime is one agent runtime engram can configure. Every writer —
// file-based or CLI-mediated — implements the same three-step contract:
// detect, plan (read-only diff), apply (write). Mirrors the migrate.Step /
// registerDestructive preview/apply split already established for every
// other mutating operator command.
type Runtime interface {
	// Name is the stable identifier used in the catalog, --runtime filter,
	// generated prose (§3), and Plan/Result reporting. Never a display
	// string — display text is a separate field on Detection/Action.
	Name() string

	// Detect reports whether this runtime is present on the machine
	// (binary on PATH, or a well-known config directory exists) and,
	// if present, where its config lives. Never mutates.
	Detect(env Environment) (Detection, error)

	// Plan computes the full set of Actions this runtime would perform
	// against the given ServerConfig, WITHOUT writing anything. Every
	// Action carries its own Outcome (NoopAlreadyCorrect / WouldWrite /
	// WouldCreate) so preview output can distinguish "nothing to do"
	// from "would change something" per-action (§5). Plan must be safe
	// to call even when Detect found the runtime absent (returns a
	// no-op plan, not an error) — Apply's preview/apply split lives at
	// the cmd/engram/setup.go layer via registerDestructive, matching
	// every other operator command.
	Plan(env Environment, cfg ServerConfig) (Plan, error)

	// Apply performs the actions in plan, either via direct file writes
	// (atomic-write + backup, §5) or by shelling out to the runtime's
	// own CLI (Claude Code's `claude mcp add`). Returns one Result per
	// Action so a partial failure across actions — and across runtimes,
	// at the orchestrator level — is representable and reportable (§5).
	Apply(env Environment, plan Plan) (Result, error)

	// SupportsNativeSkills reports whether this runtime has a first-
	// class skill format (Claude Code: true) or needs the AGENTS.md-
	// appended fallback (everyone else today). This is a capability
	// query, not a behavior switch the orchestrator branches on — each
	// Runtime's own Plan() decides what to do with skill content; the
	// orchestrator never special-cases a runtime by name.
	SupportsNativeSkills() bool
}
```

`Environment` abstracts `os.Getenv`, `exec.LookPath`, and the home/config directory so tests can inject a fake filesystem/PATH without `t.Setenv` races — the same seam class `cliNow` already establishes for the clock in `cmd/engram/destructive.go:22-25` (a package var, `t.Cleanup`-overridable).

### Adding a runtime later

1. New file `internal/setup/newruntime.go` implementing `Runtime`.
2. One line appended to the `Runtimes` registry var.
3. One row appended to `internal/surfaces/toolclass.go`'s table if the runtime introduces a new CLI flag surface (it won't — runtimes are selected by `--runtime`, not separate commands) — so in practice, no `internal/surfaces` change per runtime, only per **command**.
4. The generated prose block in `skill/engram/commands/engram-setup.md` (§3) picks up the new runtime automatically on next `task setupgen` (or equivalent) run, because it derives from the same `Runtimes` slice's `Name()`/description, not a hand-copied list.

No existing runtime's file is touched. This satisfies the milestone's explicit requirement.

## 3. Two-paths-must-agree: `/engram-setup` prose vs. `engram setup` binary

### The problem restated precisely

`skill/engram/commands/engram-setup.md` is **prose a model follows**, not code a compiler checks. Today it is the *only* path (`disable-model-invocation: true` at the top, `skill/engram/commands/engram-setup.md:4`, and its own step list at lines 15-59 is entirely manual: ask for a URL, ask for an auth mode, run one of four `claude mcp add` invocations from a hand-typed table at lines 40-45). The milestone requires this file to grow a **conditional delegation**: if `engram` is on PATH, hand off to `engram setup`; otherwise keep the current prose bootstrap. Once `internal/setup/claudecode.go` exists, its `Plan()`/`Apply()` will independently encode the *same* logic this markdown file's table already encodes by hand (which `claude mcp add` invocation for which auth mode). Two independently-maintained encodings of the same procedure is precisely the class of drift this repo has a documented allergy to — and this repo already has vacuous-gate scar tissue: multiple `PROJECT.md` "Carried tech debt" entries record tests that passed without proving what they claimed (`REQUIREMENTS.md`/`WINDOWS.md` items referenced across v0.13.x, `TestExitCodeBaseline`'s env-var fragility, #476).

### Options evaluated

| # | Option | Mechanism | Can it pass vacuously? | Verdict |
|---|--------|-----------|--------------------------|---------|
| A | **Generate the prose's fallback block from the same source the CLI reads** | New `internal/setupgen` (mirrors `internal/surfacesgen`, `internal/surfacesgen/main.go:1-9` — "regenerates every anchored interface-surface region... its only content source is `surfaces.Rules()`") reads `setup.Runtimes` and rewrites an HTML-comment-anchored region inside `engram-setup.md` (same anchor-pair idiom `internal/surfaces` already uses for markdown/proto). `task surfaces:gen`'s existing CI job (`Taskfile.yaml:245-262`, deliberately excluded from `task test`/`task lint` so it's never a hidden side effect) becomes the model: regenerate in a throwaway checkout, `git diff --exit-code`. | **No, not vacuously** — a `git diff --exit-code` after a full regeneration fails the instant the checked-in file differs from what the generator would produce *right now*, byte for byte. It cannot pass while carrying stale content, because "pass" and "byte-identical to fresh output" are the same condition by construction — there is no partial-match, substring, or count-based check to weaken. This is structurally stronger than every other doc-sync gate in this repo (`docsync_test.go`, `migrate_docs_test.go`), which are substring-containment checks over hand-authored prose and *can* pass on an accidentally-correct-looking string. | **Primary mechanism.** |
| B | **A test that executes both paths and diff-compares real output** | Run `engram setup --output json` (preview) against a fixture environment; separately, have an *agent* follow the markdown prose in the same fixture environment and record its tool calls; diff the two action sets. | For the CLI half, no. For the prose half, this requires an LLM subprocess in CI — nondeterministic across model versions, slow, and its "did the agent do the right thing" judgment is itself a soft check, i.e. a new vacuity surface (a model that vaguely does something plausible and the assertion is lenient enough to accept it). | **Reject as a CI gate.** Valuable as a **manual, one-time cold read** at phase-verification time — this repo already has exactly this precedent: v0.12.x Phase 6's rule-capture fix was "validated by a cold read: a fresh agent with zero phase context unprompted named the trigger... the user-blessed gate provably intact" (`PROJECT.md`, v0.12.x section). Recommend the same discipline here — a cold-read UAT step in the phase's verification loop, not an automated regression. |
| C | **Structural equivalence check only (substring containment)** | Assert the runtime names / commands appear somewhere in the markdown, à la `docsync_test.go`'s `TestUpgradeGuideNamesEveryChangedCommand` (extracts a `## Unreleased` section, checks each changed command name is `strings.Contains`-present). | **Yes, vacuously.** A stale or wrong invocation for the *right* runtime name would still pass — the string "codex" appearing anywhere in the file satisfies the check even if the exact command syntax next to it is wrong. This is the exact shape of check the codebase's own `docsync_test.go` uses for *lower-stakes* facts (a command name existing somewhere in an upgrade note) — acceptable there because the claim being checked ("this command's exit code changed, is it mentioned at all") is coarser than the claim here ("is the exact procedure identical to what the binary does"). | **Reject as the primary gate; acceptable only as a supplementary smoke check**, not a substitute for A. |
| D | **Hand-maintained "keep these in sync" comment/checklist, no gate** | A code comment or PR-template checklist item asking the author to remember. | **Trivially vacuous** — no enforcement at all; the exact anti-pattern the milestone context calls out ("the equivalence needs a derived gate rather than two hand-maintained instruction sets") and the exact pattern this repo's CLAUDE.md explicitly forbids for tool-owned/generated files. | **Reject outright.** |
| E | **Delete the fallback prose; always require the binary** | `/engram-setup` only ever runs `engram setup`. | Structurally eliminates drift by eliminating one of the two paths. | **Foreclosed by requirements**, not by feasibility — the milestone states explicitly: "the plugin installs standalone via `claude plugin install`, so the binary is never guaranteed present and the prose path stays first-class." Recorded here so a future reader does not re-propose it without knowing it was considered. |

### Recommendation

Adopt **A** as the enforced CI gate, generating an anchored block inside `skill/engram/commands/engram-setup.md` from `internal/setup.Runtimes` (name, detection description, and — critically — the *exact* invocation each runtime's `Plan()` would emit for a representative input, rendered as text by a `Describe() string` method on `Runtime` or by serializing a dry-run `Plan`). Pair it with **B as a manual cold-read UAT**, not automation, at the phase's verification step — this repo already treats a cold read as an acceptable, previously-used verification instrument for exactly this class of claim (an agent correctly following prose with no other context), so there is precedent rather than a new practice being introduced.

Concretely, this needs:
- `internal/setupgen/main.go` (new command package, sibling to `internal/surfacesgen`), content-sourced only from `internal/setup.Runtimes`, writing no environment/flag-derived content — same purity property `internal/surfacesgen/main.go:1-9`'s doc comment states for its own generator.
- Anchor comments in `skill/engram/commands/engram-setup.md` bracketing the fallback section, same idiom as `internal/surfaces`' anchor pairs.
- A `task setup:gen` (or folded into `task surfaces:gen`) Taskfile target, and a CI job that runs it in a throwaway checkout and fails on `git diff` — mirroring `Taskfile.yaml:245-262`'s `surfaces:gen` exactly.

## 4. Interaction with the command catalog + golden tests

### What `buildCatalog` requires

`cmd/engram/catalog.go:98-107` walks the **entire live cobra tree** (`walkCommands(root, commandWalkSkip)`) and, for every command found, looks up `surfaces.ClassForCommand(key)` — **and panics if the row is missing**:

```
panic(fmt.Sprintf(
    "catalog: command %q has no internal/surfaces blast-radius classification — "+
        "add a row to internal/surfaces/toolclass.go's operations table",
    key,
))
```

This means adding `setup` to the cobra tree **without** simultaneously adding a row to `internal/surfaces/toolclass.go`'s `operations` table breaks every build that constructs the catalog (including any test that calls `buildCatalog`) — this is a hard same-commit dependency, not a follow-up task. The row should mirror `migrate`'s shape (`internal/surfaces/toolclass.go:202-204`'s neighbors around the `version`/`backfill-short-ids` entries at lines 194-200): `MCPTool: ""` (setup has no MCP counterpart — it acts on the local machine running the CLI, not on the Qdrant-backed store; matches the documented convention that `reindex`, `migrate-remap-owner`, `prune-expired`, `summarize-missing`, `backfill-short-ids` and the bare self-describe invocation all carry an empty `MCPTool` column, per the `Operation` doc comment). `Class{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false}` — additive-only, re-runnable-to-convergence, same shape as `backfill-short-ids` (`toolclass.go:195-199`) and `migrate` itself, which is exactly the "idempotent re-install as the update path" property the milestone requires.

### `registerDestructive` gives preview-by-default "for free" — with one naming landmine

Because `setup`'s row has `ReadOnly: false`, `destructiveByClassification` (`cmd/engram/destructive.go:29-70`) will accept it, and `registerDestructive` (`cmd/engram/destructive.go:104-152`) can install it exactly like `migrate`/`prune-expired`/`backfill-short-ids`: `addApplyFlag` reads its `--apply` usage string from the shared `surfaces.RuleDestructiveRequiresApply` sentence (`destructive.go:76-84`), so `engram setup --help` will read the *identical* apply-flag wording as `engram migrate --help` with zero new prose to author — directly satisfying the milestone's "matching the `engram migrate` convention" requirement.

**Landmine:** `cmd/engram/cmdwalk.go:112-127`'s `operatorCommands()` — the structural predicate several output/timeout-parity gates derive from — excludes a command from the operator tier if `cmd.Flags().Lookup("server") != nil` (`cmdwalk.go:118`). `engram setup` plausibly needs a server URL to write into runtime configs (the same URL `/engram-setup`'s prose currently prompts for, `skill/engram/commands/engram-setup.md:17-23`). **If that flag is named `--server`**, `setup` is silently excluded from `operatorCommands()` and treated as a client-tier command by every gate keyed on that predicate — the opposite of what the milestone intends (it's explicitly framed as following the *operator*-tier `migrate` convention). Name the flag something else (`--server-url`, or reuse `config.FlagDefault` under a distinct name) to avoid tripping this heuristic. `cmdwalk.go:100-111`'s own doc comment already warns this predicate is "structural, not enumerated" and will silently reclassify a future command that happens to declare a `server` flag for unrelated reasons — this is that exact case materializing.

### Golden files that must regenerate

`cmd/engram/testdata/catalog.golden` and `cmd/engram/testdata/help.golden` are generated by walking the live tree under `-update` (`cmd/engram/golden_test.go:26`'s `updateGolden` flag, gated so regeneration is **never** a side effect of `task test`/`task lint`, only of `task surfaces:gen`, `golden_test.go:20-25`). Adding `setup` and `version --json`:

- Adds a new `"setup"` entry to `catalog.golden`'s `commands` array (name, summary, flags — including the shared `--apply`/`--output`/`--timeout` trio every other operator command carries) and a new `## engram setup` section to `help.golden`.
- Changes the **existing** `"version"` entry's `flags` array (currently empty, confirmed by `toolclass.go:202-204`'s existing `ReadOnly: true` row and the absence of any flags in `version.go:12-18`) to include the new `--json` bool flag. `version` already has a classification row — **no new `internal/surfaces` row is needed for `version --json`**, only the golden regeneration.
- Both changes are produced by running `task surfaces:gen` locally (`Taskfile.yaml:245-262`) after the command changes land, then committing the regenerated goldens alongside the code — the same workflow every prior `--help`-affecting change in this repo has followed (`golden_test.go`'s whole reason for existing).

`version.go:15`'s `Run: func(_ *cobra.Command, _ []string) {...}` (plain `Run`, not `RunE`) will need to become `RunE` to return an encoding error from a `--json` path — a small, mechanical, low-risk change but worth flagging since every other JSON-emitting command in this package returns an error via `RunE` (`catalog.go:219` `runSelfDescribe`), and `version` is currently the only surviving `Run`-only command aside from a bare help-printer.

## 5. Config-writing safety

No existing command in this repo writes to a file the user owns and a third-party tool also reads — the closest structural precedent is **`supersede_memory`'s multi-target merge**, which has the same "operate on several independent targets, one may fail, don't let partial failure corrupt state or block the others" shape as writing N runtime configs. Per `PROJECT.md`'s v0.13.x section: "the back-stamp-failure path became a classified reconciliation pass that removes the survivor, re-reads the full target set across every payload-op chunk, and clears dangling links; proved against a real pinned Qdrant... with a forced mid-sequence partial failure" (REQ-merge-atomicity). The pattern worth carrying over is **attempt every target independently, classify each outcome, never let one failure silently swallow or block another target's result** — not the Qdrant-specific mechanics.

Concrete recommendations, all zero-new-dependency (stdlib `os`, `io`, `os/exec`):

- **Backup before overwrite.** Read the existing file's bytes and mode; if non-empty, write a sibling `<path>.bak` (single generation is enough — this is a one-shot local safety net, not a version history) *before* any mutation. Skip entirely for `ActionCreate` (file doesn't exist yet — nothing to back up).
- **Atomic write.** Write desired content to a temp file in the **same directory** as the target (`<path>.tmp.<random>`), `fsync`, then `os.Rename` over the original — POSIX-atomic within one filesystem, the standard zero-dependency idiom for "never leave a half-written config file even on crash mid-write." Same-directory is load-bearing: `os.Rename` across filesystems is not atomic and can fail outright.
- **Idempotency as a first-class three-way outcome**, not a boolean "changed": `Action.Outcome ∈ {NoopAlreadyCorrect, WouldCreate, WouldWrite}` (preview) collapsing to `{Unchanged, Created, Written}` (apply result). "Already correct — nothing to do" must be structurally distinct from "wrote it" all the way to the rendered report — this is what makes a re-run of `engram setup` legibly converge (the milestone's explicit "idempotent re-install as the update path" requirement) rather than reporting "wrote 5 files" on every invocation regardless of whether anything actually changed. Comparison for `NoopAlreadyCorrect` should be **content-equality after format-aware normalization** (e.g. re-marshal-and-compare for JSON targets) rather than byte-identity, since a third-party tool may reformat whitespace between runs without the effective config differing.
- **Partial failure across multiple runtimes.** Each `Runtime.Apply()` call should be independent — a failure in runtime 3 must not prevent runtimes 4 and 5 from being attempted, and must not corrupt runtime 3's own file (atomic write already prevents the latter). The orchestrator (`cmd/engram/setup.go` or an `internal/setup.ApplyAll` helper) should collect a `Result` per runtime, `errors.Join`-style (the idiom `internal/migrate/registry.go`'s `Validate` already uses to accumulate multiple independent violations rather than stopping at the first), and report a **per-runtime** pass/fail table through the same typed operator-view renderer as everything else.
- **Open design question (not resolved by existing precedent):** exit-code semantics for "4 of 5 runtimes succeeded, 1 failed." This repo's exit-code taxonomy (2/4/5/6/7, `cmd/engram/catalog.go:126-147`) has no existing multi-target-partial-failure case to model this on — `supersede_memory`'s merge either fully succeeds or fully reconciles-and-fails as one call, it does not report N independent per-target pass/fail outcomes in one exit code. Recommend deciding this explicitly during planning (likely: non-zero on any failure, with the per-runtime detail carried in the JSON/text report body rather than encoded in the exit code itself) rather than inferring it from a precedent that doesn't actually match this shape.

## 6. Build order

Ordered by genuine dependency, not conceptual grouping:

1. **`engram version --json`** — no dependency on anything else in this milestone. Pure `cmd/engram/version.go` change (`Run` → `RunE`, add `--json` bool flag) + `task surfaces:gen` golden regen (no new `internal/surfaces` row needed — `version` is already classified `ReadOnly: true` at `toolclass.go:202-204`). This is the Homebrew cask's hard prerequisite per the milestone's own framing ("the codegraph cask's postflight version assertion does not port" without it) and should land **before or in parallel with** the cask work, never after.
2. **`internal/setup` core: `Runtime` interface, `Plan`/`Action`/`Outcome` types, atomic-write+backup helpers, empty `Runtimes` registry.** No cobra wiring yet. This is the "writer abstraction before individual runtimes" the milestone calls for, and it is genuinely prerequisite — every later step depends on this interface's shape being settled.
3. **`cmd/engram/setup.go` wired via `registerDestructive`, plus the mandatory `internal/surfaces/toolclass.go` row and the `--server-url`-not-`--server` naming decision (§4).** Must land in the same change as step 2's registry becoming non-empty enough to be user-visible, because `buildCatalog` panics on an unclassified command the moment it's added to the tree — this is not sequenceable across separate PRs without breaking every build in between unless the command and its classification row land atomically together. Golden regen (`task surfaces:gen`) is part of this step, not a follow-up.
4. **The two-paths-must-agree generator scaffold (`internal/setupgen`, anchors in `engram-setup.md`, CI drift job) — land immediately after step 3, even with only a stub runtime registered.** Sequencing this *before* the bulk of the individual runtimes (rather than retrofitting it once several runtimes already exist undocumented) means every runtime addition from this point forward is mechanically forced through the regenerate-and-diff gate as part of its own commit — closing the vacuous-gate risk from the start rather than needing a later cleanup pass to catch drift that already accumulated. This mirrors the RED-before-GREEN discipline this repo already holds itself to elsewhere (`PROJECT.md`'s v0.13.x "Carried tech debt" calls out exactly the two cases where that discipline slipped, as a cautionary note).
5. **Claude Code writer** (`internal/setup/claudecode.go`): richest existing precedent (`skill/engram/commands/engram-setup.md`'s current `claude mcp add` table, lines 40-45) to port into `Plan()`/`Apply()`, plus the shared `skill/engram/assets.go` `go:embed` package (§1) — build this alongside the Claude Code writer since it's both the first consumer of native-skill-format output and the shared dependency the AGENTS.md-fallback writers reuse for their raw skill text.
6. **Generic MCP client writer** — no CLI to shell out to, so it exercises the plain file-write + backup + atomic-rename path in isolation before the remaining three writers, which will reuse that same path.
7. **Codex, Cursor, opencode writers** — low-stakes ordering among these three; each needs its own config-format research at implementation time (flagged as unresolved by this research: exact Codex/Cursor/opencode config schema and location were not verified against current upstream docs in this pass). Each is independently addable per §2 once step 6 has proven out the shared file-write path.
8. **`/engram-setup` delegation logic** ("if `engram` is on PATH, hand off; otherwise keep prose") is a small, independent prose edit that can land as soon as step 3 produces a minimally functional `engram setup` — it does not need to wait for every runtime writer, since delegation only means "invoke the binary," and the fallback content's freshness is handled by step 4's generator on an ongoing basis.
9. **Homebrew cask** (`homebrew_casks:` in `.goreleaser.yaml`, cross-repo PAT, postflight quarantine strip, install-time version-assert gate) — depends only on step 1. Can proceed in parallel with steps 2–8; do not gate it behind `internal/setup` completion.
10. **Docs-site install documentation** — describes the shipped `engram setup` UX and the brew path; sequence last so it documents final behavior, though drafting can start once the cask (step 9) and at least the Claude Code writer (step 5) exist.

## Zero-new-Go-dependencies check

Every mechanism proposed above is satisfiable with stdlib alone: `os`/`io` for atomic writes and backups, `os/exec` for shelling out to `claude mcp add`, `encoding/json` for config marshal/diff, `embed` for skill-asset packaging, `go/parser`/`go/token` if a purity gate is added for `internal/setup` (mirroring `internal/migrate/leafpurity_test.go`), and the existing `github.com/spf13/cobra`/`pflag` for the new command. **No new Go dependency is required by any part of this design.** The one item flagged as unresolved (exact third-party config formats for Codex/Cursor/opencode) is a research gap, not a dependency risk — whatever format each turns out to use (JSON, TOML, YAML) is almost certainly stdlib-writable as a plain map/struct marshal; if any turns out to require a format stdlib cannot emit correctly (e.g. TOML has no stdlib encoder), **that would be a real zero-dependency violation and must be flagged explicitly during that runtime's planning phase**, not discovered silently at implementation time.

## Sources

All findings above are grounded in repository reads performed in this research pass:

- `.planning/PROJECT.md` (milestone framing, "Current Milestone: 2026-08-23.01 Distribution & Agent Bootstrap" section and prior-milestone precedent entries)
- `CLAUDE.md` (repo conventions: cmd/engram entrypoint-only, migrations, releases)
- `cmd/engram/catalog.go`, `cmd/engram/cmdwalk.go`, `cmd/engram/version.go`, `cmd/engram/root.go`, `cmd/engram/main.go`, `cmd/engram/destructive.go`, `cmd/engram/spine_review.go`, `cmd/engram/operator_view.go`, `cmd/engram/golden_test.go`, `cmd/engram/docsync_test.go`, `cmd/engram/migrate_docs_test.go`, `cmd/engram/client_common.go`
- `cmd/engram/testdata/catalog.golden`, `cmd/engram/testdata/help.golden`
- `internal/surfaces/toolclass.go`
- `internal/migrate/registry.go`, `internal/migrate/leafpurity_test.go`
- `internal/surfacesgen/main.go`
- `internal/config/config.go`
- `internal/store/spine.go`
- `skill/engram/commands/engram-setup.md`
- `skill/engram/.claude-plugin/plugin.json`
- `.goreleaser.yaml`
- `Taskfile.yaml` (`proto:gen`, `surfaces:gen` targets)
- `docs-site/src/content/docs/guides/quickstart.md`, `docs-site/src/content/docs/guides/cli.md`

---
*Architecture research for: engram `setup` subcommand + multi-runtime distribution*
*Researched: 2026-08-23*
