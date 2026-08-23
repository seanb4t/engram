# Project Research Summary

**Project:** engram
**Domain:** CLI distribution (Homebrew cask) + multi-runtime agent-config bootstrap (`engram setup`)
**Milestone:** 2026-08-23.01 — Distribution & Agent Bootstrap
**Researched:** 2026-08-23
**Confidence:** MEDIUM overall — HIGH on distribution mechanics, MEDIUM on the agent-runtime config surfaces that make up most of the new code

## Executive Summary

This milestone builds on solved ground for half its scope and open ground for the other half, and the two halves should not be graded on the same curve. Homebrew cask publishing via GoReleaser's `homebrew_casks:` is HIGH confidence: version-pinned docs, a real sibling cask already running in production (`seanb4t/homebrew-tap` `Casks/codegraph.rb`), and this repo's own dependency graph all agree on the shape. The hard parts here are known and enumerable — cross-repo credential scoping, the release-please/GoReleaser tag-vs-artifact ordering gap, and (because this binary is deliberately unsigned) getting the quarantine-strip-before-version-gate ordering right in the cask's `postflight` block.

The other half — writing native config for Claude Code, Codex, Cursor, and opencode — is MEDIUM confidence across all four independent research passes, and all four converge on the same warning: third-party agent-runtime docs move fast, are not version-pinned the way GoReleaser's are, and in opencode's case actively contradict themselves across two currently-live docs pages (V1 vs V2 MCP schema). This asymmetry should drive build order directly: land the distribution half and the `internal/setup` writer *abstraction* first, where the ground is solid, and treat each runtime-specific writer as needing a fresh, direct verification pass against that runtime's current behavior immediately before its own implementation — not as something settled by this research round.

The core architectural risk is not any single runtime's config format — it's the "two paths must agree" problem: `/engram-setup` (hand-maintained prose) and `engram setup` (Go code) must produce equivalent outcomes, and this repo has documented history of vacuous gates (keyword-presence checks, independent liveness checks) that look like they prove equivalence while proving nothing. The mitigation converges from architecture and pitfalls research on the same answer — generate the prose's mechanical content from the CLI's own source of truth, with a `git diff --exit-code` gate that cannot pass on stale content — and this should be treated as load-bearing infrastructure, not a follow-up test.

## Post-Synthesis Live Verification (orchestrator, 2026-08-23)

> Added by the `/gsd-new-milestone` orchestrator AFTER synthesis, by running the real binaries
> installed on this machine. This supersedes the corresponding "MEDIUM/LOW confidence,
> re-verify just-in-time" guidance below wherever the two disagree — these are HIGH-confidence
> observations from live tools, not documentation reads. Open questions 1, 2 and 9 are resolved
> in the table further down.

**Three of four target runtimes have a native `mcp add` CLI command. Only Cursor needs a file writer.**

| Runtime | Write path | Verified how | Format engram must parse |
|---------|-----------|--------------|--------------------------|
| Claude Code | `claude mcp add --transport http engram <url> --scope user` (+ `--header`, `--client-id`) | already shipped in `/engram-setup` prose | none |
| Codex | `codex mcp add <NAME> --url <URL>` (+ `--bearer-token-env-var`, `--oauth-client-id`, `--env`) | `codex mcp add --help`, codex-cli **0.148.0** | **none — TOML never touched** |
| opencode | `opencode mcp add [name] --url <URL> --header KEY=VALUE` (+ `auth`, `logout`, `debug`, `list`) | `opencode mcp add --help`, opencode **1.18.15** | **none — JSONC never touched** |
| Cursor | file write: `~/.cursor/mcp.json` | read live: plain JSON, top-level `mcpServers` key | plain JSON (stdlib `encoding/json`) |

**Consequences for the roadmap:**

1. **The TOML/JSONC stdlib gap is retired, not mitigated.** The "surgical marker-bounded text editing"
   design that all four researchers converged on is no longer needed for any runtime. Zero-new-Go-dependencies
   is under no pressure from this milestone. Do not build a TOML text editor.
2. **opencode's V1/V2 schema divergence — named as the round's single highest-risk item — is moot.**
   engram never writes that file, so which schema the binary reads is not engram's concern.
3. **Phase 7 collapses.** What was scoped as "three high-risk writers needing just-in-time schema
   re-verification" is now two shell-out writers (structurally identical to the Claude Code writer)
   plus one plain-JSON file writer.
4. **The merge-never-replace requirement is live and real for Cursor.** The verifying machine's
   `~/.cursor/mcp.json` already contains three unrelated servers (`MCP_DOCKER`, `codegraph`, `firecrawl`).
   Clobbering that file is a real, reachable defect, not a hypothetical.
5. **A new dependency appears: runtime CLI availability.** Shelling out means each runtime's binary must
   be on PATH and support these flags. Flag/version drift in a third-party CLI is now a live failure mode,
   which argues for pinning the observed versions above and failing legibly on an unexpected flag surface
   rather than silently writing nothing.

**Not verified here:** Cursor was not on PATH on this machine (only `~/.cursor/` exists), so no Cursor
*CLI* surface was probed — the file-writer conclusion stands on the config file read, and the possibility
of a Cursor CLI remains genuinely unchecked. Native *skill*-format questions (open questions 3 and 4) are
untouched by this verification and remain open.

## Key Findings

### Recommended Stack

GoReleaser v2.10+ `homebrew_casks:` (already satisfied by this repo's `~> v2` CI pin) replaces the deprecated `brews:` formula path. No archive changes needed — the existing single `tar.gz` `id: default` archive works as-is. `engram version --json` and shell-completion generation both ride on `cobra`/`cobra/doc`, whose transitive deps are *already* indirect requirements in `go.mod` — importing `cobra/doc` should be a zero-diff `go mod tidy`. Cross-repo publishing to `seanb4t/homebrew-tap` requires a dedicated credential (PAT or GitHub App token) distinct from the release workflow's own `GITHUB_TOKEN`. Unsigned-binary distribution is handled by a `postflight` xattr quarantine strip, not code signing — signing would require GoReleaser Pro + an Apple Developer account, correctly out of scope.

**Core technologies:**
- GoReleaser `homebrew_casks:` — cask generation/push, replaces deprecated `brews:` — matches the org's already-shipped `codegraph.rb` pattern
- `cobra`/`cobra/doc` (already vendored) — completions, man pages, `version --json` — genuinely zero new dependency
- `encoding/json` (stdlib) — JSON config-writer targets (Claude Code, Cursor); NOT sufficient for Codex (TOML) or comment-bearing opencode files (JSONC)
- Marker-delimited plain-text append (no library) — AGENTS.md fallback for runtimes with no native skill format

### Expected Features

**Must have (table stakes):**
- Runtime detection via binary-on-PATH as primary signal (config-dir presence alone produces both false positives and false negatives — every serious prior-art tool avoids it as the sole gate)
- Preview-by-default / `--apply` to mutate, matching engram's own established operator-command convention (`migrate`, `prune-expired`)
- Show the exact bytes that would be written, not a summary — table stakes across every serious multi-runtime tool surveyed (`mx setup --dry-run`, `zowe --dry-run`)
- Merge into existing config, never replace whole file — every serious prior-art tool (`getmcp`, `mx setup`, `mcp-config`) treats overwriting a foreign entry as the cardinal sin
- Idempotent re-run converging to the same state, with "already correct" reported distinctly from "wrote it"
- Claude Code: shell out to `claude mcp add`, never hand-write `~/.claude.json`/`.mcp.json` — the existing `/engram-setup` prose already does this correctly and must not regress
- Non-interactive `--yes`/`--json`/`--runtime <name>` flags from v1, not deferred — every serious multi-runtime tool has these from day one

**Should have (differentiators):**
- Single first-party binary configuring 4 runtimes in one pass, vs. generic third-party tools that can't know engram's own auth modes
- `/engram-setup` conditionally delegating to `engram setup` — genuinely novel prior-art shape, needs a derived (not hand-maintained) equivalence gate

**Defer (v2+):**
- Drift/version-skew detection across binary, plugin, and server versions — explicitly out of scope
- Auto-reconciling a manually hand-edited engram MCP entry that has drifted from what `engram setup` would write
- Team/org-level Cursor config — no CLI surface exists for this even in principle

**Anti-features (deliberately not building):** silently overwriting existing config entries; auto-running `engram setup` from a `brew install` postinstall hook (Homebrew postflight failures are swallowed, `rescue => e; opoo e`, making a broken auto-setup invisible); telemetry on detected runtimes; mutating a runtime's config without the user selecting it; guessing opencode's schema version and writing the wrong one silently.

### Architecture Approach

`internal/setup/` follows this repo's established stdlib-only leaf-package convention (mirroring `internal/migrate`, `internal/surfaces`): a `Runtime` interface (`Detect`/`Plan`/`Apply`/`SupportsNativeSkills`) implemented one file per runtime in the *same* package (not subpackages — avoids invented parent packages and import-cycle risk), registered in a package-level `Runtimes` slice literal. `cmd/engram/setup.go` is a thin entrypoint wired through `registerDestructive`, the same preview/`--apply` machinery every other mutating operator command already uses — this gives `engram setup --help` identical apply-flag wording to `engram migrate --help` for free.

**Major components:**
1. `internal/setup/runtime.go` — the `Runtime` interface + registry, mirroring `migrate.Registry`'s "must be a package-level literal" discipline
2. `internal/setup/{claudecode,codex,cursor,opencode,genericmcp}.go` — one writer per runtime, each independently addable without touching the others
3. `internal/setup/apply.go` — shared atomic-write + backup primitives (temp file in same dir + `os.Rename`, one implementation reused by every file-based writer)
4. `internal/setupgen/` (new, mirrors `internal/surfacesgen`) — generates the mechanical parts of `/engram-setup`'s prose fallback from `setup.Runtimes`, with a CI `git diff --exit-code` gate
5. `skill/engram/assets.go` — a same-tree `go:embed` package carrying the five skills' Markdown, giving a brew-installed binary with no Claude plugin present the content needed for native-format or AGENTS.md-fallback skill writes, with zero duplication risk

**Hard sequencing constraints found in the code:**
- `cmd/engram/catalog.go:100-107` panics if any cobra command lacks a row in `internal/surfaces/toolclass.go` — adding `setup` to the tree and adding its classification row **must land in the same commit/PR**, not sequenced across two.
- `cmd/engram/cmdwalk.go:118`'s `operatorCommands()` predicate excludes any command with a flag literally named `--server` — if `engram setup` needs a server-URL flag, it must be named something else (e.g. `--server-url`) or it silently falls out of operator-tier classification, the opposite of the milestone's intent.
- `engram version --json` is a hard prerequisite for the cask's postflight install-time gate (`system_command engram, args: ["version", "--json"]`) — it has no dependency on anything else in this milestone and should land first or in parallel with the cask work, never after.
- Quarantine-strip ordering: the `postflight` block must strip `com.apple.quarantine` from the installed binary as its literal first action, before any `system_command` invocation of that binary — reversing this order means the version-assertion gate itself gets SIGKILLed by Gatekeeper rather than failing cleanly.

### Critical Pitfalls

1. **Cross-repo credential scoping (Pitfall 1)** — the release workflow's `GITHUB_TOKEN` and release-please App token cannot write to `seanb4t/homebrew-tap`; a dedicated credential (scoped PAT or, better, a second GitHub App installed only on the tap) must be provisioned and verified with a manual `gh api` call *before* the first real release depends on it.
2. **Tag-cut-but-cask-unpublished (Pitfall 2)** — release-please and GoReleaser share only "the tag exists" as a signal; any failure between tag creation and the cask push (proxy delay, credential failure) leaves a published GitHub Release with a stale or missing cask. Recovery already exists in this repo's `workflow_dispatch` re-ship path — extend it, don't invent a new mechanism, and rehearse it once against a throwaway tag.
3. **Quarantine-strip-before-gate ordering (Pitfall 3)** — the single most consequential pitfall for this milestone specifically, because `engram version --json` as an install-time correctness gate is a named milestone prerequisite.
4. **TOML/JSONC vs. zero-new-Go-dependencies (Pitfall 6, cross-cutting)** — see dedicated section below.
5. **The two-paths-must-agree vacuity trap (Pitfall 12, cross-cutting)** — see dedicated section below.

## Cross-Cutting Risks (found independently by multiple researchers)

### TOML/JSONC stdlib gap vs. zero-new-Go-dependencies

STACK, FEATURES, ARCHITECTURE, and PITFALLS all independently flag the same fact: Go's stdlib has no TOML support (needed for Codex's `~/.codex/config.toml`) and `encoding/json` rejects comments outright (needed for opencode's `opencode.jsonc`), while the milestone's standing constraint is zero new Go dependencies. All four converge on the same mitigation, described with matching detail: **do not attempt a general parse/merge round-trip for either format.** Instead do surgical, marker-bounded *text-level* editing — detect a `[mcp_servers.engram]` table header (or equivalent) via line-oriented scanning/regex, and only ever append (if absent) or replace the exact byte range between that header and the next top-level header (if present), leaving everything else in the file byte-for-byte untouched. This sidesteps needing a parser for the write path entirely; a similarly narrow lexical check suffices for the idempotency-detection read path. PITFALLS additionally flags this as a decision point that must be *recorded explicitly* if it ever needs revisiting (e.g., if line-oriented editing proves too fragile against nested inline tables or multi-line strings) — this project's "zero new deps" constraint is described as standing, i.e. requiring a deliberate, recorded exception rather than a quiet one. No disagreement across researchers on this point; treat it as settled guidance.

### The two-paths-must-agree vacuity problem

ARCHITECTURE and PITFALLS both treat this as a primary risk and converge on the same underlying diagnosis and the same primary mitigation, with PITFALLS adding sharper detail on *why naive gates fail*:

- **Diagnosis (both agree):** `/engram-setup` (hand-maintained markdown) and `engram setup` (Go code) must produce equivalent outcomes when the slash command delegates, and nothing structurally prevents drift between them. This repo has documented history of exactly this failure shape (a regex character class swallowing a token boundary; independent liveness checks that each look green while proving nothing about equivalence).
- **Naive gates that would pass vacuously (PITFALLS, itemized):** keyword/string-presence matching between the two files; independent liveness checks with no cross-comparison; freshness/recency proxies (mtime ordering). ARCHITECTURE's own evaluated-options table (option C, "structural equivalence via substring containment") independently arrives at the same rejection for the same reason.
- **Primary mitigation (both agree, same mechanism):** generate the mechanical content — the mode→command table already in `engram-setup.md` — from a single Go-side source of truth (`internal/setup.Runtimes`), with a `git diff --exit-code` CI gate after a full regeneration in a throwaway checkout. This is structurally non-vacuous because "pass" and "byte-identical to fresh output" are the same condition by construction; there is no partial-match check to weaken. ARCHITECTURE names the concrete implementation: a new `internal/setupgen` package mirroring the existing `internal/surfacesgen`, with anchor-comment regions in the markdown, following the same `task surfaces:gen`-in-throwaway-checkout CI pattern already proven for `internal/surfaces`.
- **Secondary mitigation, where they add distinct value:** ARCHITECTURE proposes a *manual cold-read UAT* at phase-verification time — citing this repo's own v0.12.x precedent of a fresh agent with zero phase context correctly following prose as an accepted verification instrument — for the natural-language parts that can't be generated (framing, tone, troubleshooting prose). PITFALLS frames the same non-generatable-content problem as needing a *behavioral* dry-run-diff test: run `engram setup --dry-run` for a given runtime/mode combination, capture the machine-readable plan, and prove both the prose-delegation branch and the CLI branch converge on the same resulting config state by diffing the *result*, never the *instructions*. These are complementary, not conflicting — generate what's mechanical, gate what's generated with a byte-diff, and verify what's irreducibly prose with either a cold read (ARCHITECTURE) or a behavioral convergence test (PITFALLS); both explicitly reject textual-similarity checking as the *primary* proof mechanism for the same reason.

No disagreement between the two reports on the core mitigation; they emphasize complementary secondary layers rather than proposing competing solutions.

## Implications for Roadmap

Based on research, suggested phase structure (ordered by genuine dependency per ARCHITECTURE's build-order analysis, not conceptual grouping):

### Phase 1: `engram version --json`
**Rationale:** No dependency on anything else in this milestone; hard prerequisite for the cask's postflight gate. Land first or in parallel with cask work, never after.
**Delivers:** `--json` flag on the existing `version` command (`Run` → `RunE` for error return), no new `internal/surfaces` row needed (already classified `ReadOnly: true`).
**Avoids:** Pitfall 3 (a cask that installs green but whose install-time correctness gate has nothing to invoke).

### Phase 2: Homebrew cask distribution
**Rationale:** Depends only on Phase 1 (needs `version --json` to exist for the postflight gate). Can proceed in parallel with the `internal/setup` work.
**Delivers:** `homebrew_casks:` block in `.goreleaser.yaml`, cross-repo credential, `postflight` quarantine-strip-then-version-assert hook, and a forked-and-adapted cask rehearsal target (`task release:rehearse-cask`, unsigned-appropriate variant of `codegraph-go`'s).
**Addresses:** the milestone's "one command" install promise.
**Avoids:** Pitfalls 1 (credential scoping), 2 (tag/artifact ordering gap), 3 (quarantine ordering), 4 (`brew audit` failure classes), 15 (rehearsal must positively assert the quarantine strip, not just copy the signed sibling's rehearsal).

### Phase 3: `internal/setup` core abstraction
**Rationale:** Every runtime writer depends on this interface's shape being settled first; the milestone explicitly calls for the writer abstraction before individual runtimes.
**Delivers:** `Runtime` interface, `Plan`/`Action`/`Outcome` types, shared atomic-write+backup primitives (same-directory temp file + rename, symlinked-dotfiles-safe), an injectable-home-directory test seam, empty `Runtimes` registry.
**Uses:** stdlib only (`os`, `io`, `os/exec`, `encoding/json`).
**Avoids:** Pitfall 7 (non-atomic writes / cross-filesystem rename), Pitfall 14 (tests mutating a contributor's real dotfiles).

### Phase 4: `cmd/engram/setup.go` wiring + two-paths generator scaffold
**Rationale:** Must land atomically with Phase 3's registry becoming non-empty enough to be user-visible — `buildCatalog` panics on an unclassified command the instant it's added to the tree, so the command and its `internal/surfaces/toolclass.go` row are a same-commit dependency, not sequenceable across PRs. The generator scaffold (`internal/setupgen`, CI drift gate) should land here too, even with only a stub runtime registered, so every subsequent runtime addition is mechanically forced through the regenerate-and-diff gate from the start.
**Delivers:** `setup` registered via `registerDestructive` (preview/`--apply` for free, matching `migrate`'s convention), the `internal/surfaces` classification row, golden-file regeneration, and the `internal/setupgen` anchor-comment/CI-diff mechanism.
**Avoids:** Pitfall 12 (two-paths vacuity) from the start, rather than retrofitting once several runtimes exist undocumented; the `cmdwalk.go` `--server` flag-naming landmine.

### Phase 5: Claude Code writer
**Rationale:** Richest existing precedent (`/engram-setup`'s current `claude mcp add` table) to port; lowest-risk runtime since a confirmed, versioned CLI command exists. Also the first consumer of the shared `skill/engram/assets.go` `go:embed` skill-content package.
**Delivers:** shell-out-based `Plan()`/`Apply()` for Claude Code, reusing `/engram-setup`'s existing auth-mode question flow as CLI prompts/flags.
**Addresses:** FEATURES' "shell out, never hand-write `~/.claude.json`" table-stakes requirement.
**Avoids:** Pitfall 5 (reimplementing Claude Code's config write by hand, regressing behavior the prose path already got right).

### Phase 6: Generic MCP client writer
**Rationale:** No CLI to shell out to, so it exercises the plain file-write + backup + atomic-rename path in isolation before the three higher-risk runtimes reuse it.
**Delivers:** JSON merge-write to a portable MCP config shape.
**Uses:** the shared atomic-write primitive from Phase 3.

### Phase 7: Cursor, Codex, opencode writers
**Rationale:** Each needs its own config-format re-verification at implementation time — this is the phase group where the confidence gradient bites hardest. Cursor is lowest-risk of the three (fully primary-sourced JSON schema, though the `type: "stdio"` field requirement is disputed across sources). Codex needs the TOML-avoidance surgical-text-edit approach (see Cross-Cutting Risks) and a resolved answer to "does a native `codex mcp add` exist" before committing to file-write-only. opencode is the highest-risk of all four runtimes — a confirmed, live V1/V2 schema divergence between two currently-published docs pages means writing the wrong shape produces a config the server silently never reads.
**Delivers:** per-runtime `Plan()`/`Apply()`, with opencode possibly shipping as AGENTS.md-fallback-only this milestone if the schema question can't be closed in time (explicitly sanctioned degradation per FEATURES' MVP definition).
**Avoids:** Pitfall 6 (TOML/deps collision), Pitfall 8 (non-uniform config paths — hardcode each as a cited constant, never derive), Pitfall 9 (duplicate registrations across differently-keyed identity signals), Pitfall 11 (version skew writing a config shape an older runtime doesn't understand).

### Phase 8: AGENTS.md fallback + `/engram-setup` delegation
**Rationale:** The delegation logic is a small, independent prose edit that can land as soon as Phase 4 produces a minimally functional `engram setup` — it doesn't need to wait for every runtime writer, since delegation only means "invoke the binary." The AGENTS.md fallback needs its own idempotency discipline (marker-delimited whole-block replacement, not checksum-only or naive append).
**Delivers:** conditional delegation in `/engram-setup`, marker-bounded AGENTS.md-appended skill guidance for runtimes without native skill formats.
**Avoids:** Pitfall 13 (AGENTS.md marker-deletion and in-region-edit-loss failure modes).

### Phase 9: Docs-site install documentation
**Rationale:** Sequence last so it documents final, shipped behavior; drafting can start once the cask (Phase 2) and Claude Code writer (Phase 5) exist. Must reflect the Homebrew 6.0.0 tap-trust nuance (the fully-qualified `brew install seanb4t/tap/engram` form auto-trusts and is the genuinely one-command path; the two-step tap-then-install form now requires an additional `brew trust` step on fresh installs).

### Phase Ordering Rationale

- Distribution (cask) and the config-writer core are independent after `version --json` lands, and can run in parallel — neither blocks the other.
- Within the config-writer track, the order is strictly risk-ascending: shared infrastructure → highest-confidence runtime (Claude Code, confirmed CLI) → no-CLI baseline (generic MCP) → the three runtimes needing fresh verification (Cursor, Codex, opencode), cheapest-to-verify first.
- The two-paths generator scaffold is deliberately front-loaded (Phase 4, not Phase 8) specifically to avoid the "retrofit a gate onto already-accumulated drift" failure mode this repo has hit before.

### Research Flags

Phases likely needing deeper research during planning (`--research-phase`):
- **Phase 7 (Codex/Cursor/opencode writers):** all three carry MEDIUM-or-lower confidence per every research pass; opencode specifically has a confirmed, currently-live schema self-contradiction that must be resolved against a real installed binary, not docs alone, before implementation. Budget explicit re-verification time here, not assumed research reuse.
- **Phase 2 (cask):** mostly HIGH confidence, but the September 2026 Homebrew Gatekeeper policy's third-party-tap scoping has no single canonical citation (inferred consistently across three sources) — worth a fast re-check if this phase's implementation window slips past a Homebrew version bump.

Phases with standard patterns (skip research-phase):
- **Phase 1 (`version --json`):** mechanical, fully specified by existing `cobra`/`cmd/engram` conventions.
- **Phase 3 (`internal/setup` core):** directly modeled on `internal/migrate`'s already-proven leaf-package shape; no open questions.
- **Phase 5 (Claude Code writer):** CLI command already confirmed and already exercised by the shipped `/engram-setup` prose.

## Consolidated Open Questions

Merged and deduplicated across all four research files. Each marked (a) cheap to resolve before planning, or (b) needing just-in-time resolution during its own phase; scope-changing ones flagged explicitly.

| # | Question | Source(s) | Resolution timing | Scope impact if unresolved |
|---|----------|-----------|--------------------|-----------------------------|
| 1 | Does a native `codex mcp add` (or equivalent) CLI command exist? | STACK, FEATURES, ARCHITECTURE | **RESOLVED 2026-08-23 (orchestrator, live)** — YES. `codex mcp add <NAME> (--url <URL> \| -- <COMMAND>...)` on codex-cli 0.148.0, with `--bearer-token-env-var`, `--oauth-client-id`, `--env`. | **Scope REDUCED**: Codex is a shell-out writer like Claude Code. The TOML-avoidance surgical-text-editing design is UNNECESSARY — `~/.codex/config.toml` is never touched by engram. |
| 2 | Which opencode MCP config schema (V1 `mcp.<name>` vs V2 `mcp.servers.<name>`) does the installed binary expect, and is there a native add command? | FEATURES, ARCHITECTURE, PITFALLS | **RESOLVED 2026-08-23 (orchestrator, live)** — native add EXISTS: `opencode mcp add [name] --url <URL> --header KEY=VALUE` on opencode 1.18.15 (also `list`/`auth`/`logout`/`debug`). | **Scope REDUCED / risk RETIRED**: the V1-vs-V2 schema divergence is MOOT — engram never writes opencode's config file. This was flagged as the round's highest-risk item; it is now closed. |
| 3 | Exact Codex `.agents/skills` / opencode `.opencode/agents` native skill-file schema, if any | FEATURES | (b) Just-in-time, during each runtime's Phase 7 sub-task | Not scope-changing — AGENTS.md fallback is already the sanctioned default absent a confirmed native format |
| 4 | Does Cursor require an explicit `type: "stdio"` field on stdio MCP entries, or is it inferred? | FEATURES | (a) Cheap — one source claims required, two primary-looking Cursor docs pages show working examples without it; a single test-fixture install against real Cursor resolves this before Phase 7 | Not scope-changing — affects correctness of one field, not the writer's overall shape |
| 5 | Third-party-tap scoping of Homebrew's September 2026 Gatekeeper-removal policy | STACK | (a) Cheap — re-check before Phase 2 ships if the window spans a Homebrew version bump | Not scope-changing for this milestone (only official taps are in scope for removal, and `seanb4t/homebrew-tap` is third-party) but worth a calendar note |
| 6 | Exit-code semantics for "N of M runtimes succeeded, 1 failed" in `engram setup` | ARCHITECTURE | (a) Cheap — no existing precedent to model on; decide explicitly during Phase 4 planning rather than inferring from `supersede_memory`'s different-shaped merge semantics | Not scope-changing, but must be decided deliberately, not left implicit |
| 7 | `engram version --json` naming: bespoke bool flag vs. reusing the existing `--output json\|text` operator convention | STACK | (a) Cheap — a naming-consistency call, not a technical blocker; decide during Phase 1 planning | Not scope-changing |
| 8 | Whether the plugin's own bundled `.mcp.json` already self-registers on `claude plugin install`, narrowing what `engram setup` needs to additionally write for Claude Code | STACK | (a) Cheap — confirm during Phase 5 planning before implementing the Claude Code writer | Not scope-changing, but affects the writer's scope within Phase 5 |
| 9 | Whether line-oriented text editing (no TOML parser) proves too fragile for real-world Codex config files | PITFALLS | **RESOLVED 2026-08-23 (orchestrator, live)** — MOOT via Q1. No TOML is parsed or written at all. | **Risk RETIRED**: the zero-new-Go-dependencies constraint is no longer under any pressure from this milestone. |

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH on GoReleaser/Homebrew/cobra mechanics; MEDIUM on agent-runtime config-file specifics | Distribution tooling verified against version-pinned docs and a real running sibling cask in this org; runtime config formats are third-party docs without comparable version pins |
| Features | MEDIUM-HIGH | Primary vendor docs for all four runtimes plus two independent third-party universal-installer tools (`getmcp`, `mx setup`) corroborating the same config-path table — but explicit UNVERIFIED markers remain for Codex/opencode CLI-native write paths |
| Architecture | HIGH | Every claim grounded in files read directly in this repo, cited by file:line; the only items flagged as open are the same third-party config-format unknowns Stack/Features/Pitfalls also flag |
| Pitfalls | HIGH for Homebrew-cask mechanics and release-pipeline integration (verified against this org's own shipped sibling cask and workflow); MEDIUM for cross-checked external claims (Gatekeeper quarantine behavior); MEDIUM/LOW for single-source runtime config-format claims (Codex TOML shape, opencode JSON shape) | |

**Overall confidence:** MEDIUM — genuinely bimodal, not an average. Treat the distribution phases (1-2, 9) as near-execution-ready; treat the three non-Claude-Code runtime writers (Phase 7) as carrying real, named unknowns that all four researchers independently flagged and all recommend re-verifying immediately before writing parser/writer code, not at some later point.

### Gaps to Address

- opencode's MCP config schema divergence (open question #2 above) is the single highest-risk unresolved item in this entire research round — it is not a documentation-quality problem that will resolve itself with more reading; it needs a hands-on test against a real installed binary.
- The Codex CLI-native-add-command question (open question #1) gates a real implementation-complexity decision (shell out vs. build a TOML-avoidance text editor) and is cheap enough to resolve that it should happen before roadmap phase sequencing locks in Phase 7's Codex sub-task scope, not during it.
- No research pass could confirm Cursor's User Rules (global, personal) storage location — it appears to live in Cursor's internal application state, not a user-editable file, and is likely simply out of reach for an external CLI. This should be explicitly scoped out of Phase 7 rather than silently attempted and silently failing.
- The multi-runtime partial-failure exit-code question (open question #6) has no existing precedent in this codebase to model on and needs a deliberate decision, not an inference from `supersede_memory`'s differently-shaped all-or-nothing semantics.

## Sources

### Primary (HIGH confidence)
- `seanb4t/homebrew-tap` `Casks/codegraph.rb`, `seanb4t/codegraph-go` `.goreleaser.yaml` and `Taskfile.yml` (fetched via `gh api`) — ground truth for a real, running sibling cask in this exact tap
- This repo's own `go.mod`/`go.sum`, `cmd/engram/*.go`, `internal/surfaces/`, `internal/migrate/`, `.goreleaser.yaml`, `.github/workflows/release.yaml`, `skill/engram/commands/engram-setup.md` — read directly, not recalled
- Context7 `/websites/goreleaser` — `customization/homebrew_casks`, `deprecations`, `customization/ci/actions`, `customization/notarize`
- Context7 `/websites/code_claude` — MCP scopes, `claude mcp add` syntax, plugin/skill/hook format
- `github.com/openai/codex/blob/main/codex-rs/core/src/agents_md.rs` — primary source confirming AGENTS.md discovery/merge logic
- `github.com/anomalyco/opencode/blob/9afbdc10/packages/opencode/src/config/config.ts` — primary source, zod schema (matches V1 docs shape only)

### Secondary (MEDIUM confidence)
- Homebrew Gatekeeper/tap-trust policy: `workbrew.com/blog/homebrew-5-0-0`, `brew.sh/2026/06/11/homebrew-6.0.0`, `docs.brew.sh/Tap-Trust`, `github.com/orgs/Homebrew/discussions/6537`, `Homebrew/homebrew-cask#246786`
- cursor.com/help/customization/mcp, cursor.com/docs/mcp, cursor.com/docs/rules — Cursor MCP/rules schema
- developers.openai.com/codex/mcp, /config-reference, /guides/agents-md — Codex TOML schema, AGENTS.md precedence
- Third-party universal installers: `@getmcp/cli` (npm, 2026-02-18), MemNexus `mx setup` blog (2026-02-15), `MarcusJellinghaus/mcp-config`, `metamcp.org` — cross-runtime config-path corroboration

### Tertiary (LOW confidence, needs validation)
- opencode.ai/docs/mcp-servers/ vs opencode.ai/v2/docs/mcp-servers — confirmed live self-contradiction on MCP config shape (V1 vs V2), needs resolution against a real installed binary
- Aggregated web-search results for Codex `config.toml` and Cursor `type: "stdio"` requirement — single-source or conflicting-source claims flagged inline throughout research files

---
*Research completed: 2026-08-23*
*Ready for roadmap: yes*
