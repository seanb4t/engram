# Phase 4: Skills Distribution - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-09
**Phase:** 4-skills-distribution
**Areas discussed:** Embed + drift gate, Who writes files, Native targets, AGENTS.md block

The user selected all four offered gray areas and chose a concrete option on every one of the
sixteen questions — nothing was deferred to Claude's discretion.

---

## Embed + drift gate

### Q1 — canonical source of truth for embedded skill content

| Option | Description | Selected |
|--------|-------------|----------|
| skill/ canonical, vendor into internal/ | Taskfile copies `skill/engram/skills/` → `internal/skills/data/`; `//go:embed` reads the copy. Mirrors `internal/webauth/static`. New drift job; plugin authorship unchanged. | ✓ |
| internal/ canonical, generate skill/ | Skills live in `internal/skills/data/`, embedded in place; `skill/` becomes `internal/surfacesgen` output, reusing the existing `interface-surface drift` CI job for zero new CI. Makes the plugin dir generated. | |
| You decide | Planner picks, subject to one canonical side and a mechanical gate. | |

**User's choice:** skill/ canonical, vendor into internal/.
**Notes:** `//go:embed` cannot traverse `..`, so no package under `internal/` can reach `skill/`
directly; a symlink does not help because embed does not follow symlinks. The deciding factor
against inverting was that `skill/engram/` is what `claude plugin install` consumes — making a
consumption surface into generated output.

### Q2 — what proves the vendored copy still matches

| Option | Description | Selected |
|--------|-------------|----------|
| Go test byte-compare | Test in `internal/skills` walks embedded FS and on-disk `../../skill/engram/skills/`; set-equality of paths + byte-equality per path. No toolchain, runs in `go test ./...` on every branch. | ✓ |
| CI drift job (mirror ui-drift) | New ci.yaml job: re-run vendor, `git diff --exit-code`. Authoritative but `pull_request`/`push:main` only — inherits backlog 999.1's hole verbatim. | |
| Both | Go test locally + CI job at PR time; the end state 999.1 proposes for the SPA. Two mechanisms asserting one property. | |
| You decide | Planner picks, subject to: mechanical, fires before PR time, set-equality not containment. | |

**User's choice:** Go test byte-compare.
**Notes:** The asymmetry that decided it: the vendored SPA needs a pnpm build to verify (hence
999.1's commit-timestamp *proxy*), but a skills vendor is a plain file copy, so exact content
equality is checkable with no toolchain at all. Set-equality rather than containment was called
out explicitly — containment passes vacuously on a superset (memories `bqhy5v5hq9`,
`8583e0yqa1`).

### Q3 — satisfying "the binary can produce the full content"

| Option | Description | Selected |
|--------|-------------|----------|
| Preview carries it, json lane only | Text row: paths + per-skill digest + byte count. `--output json` nests full content. No new verb, no new flag, no goldens move. | ✓ |
| A flag on setup | `engram setup --show-skills` dumps full content in the text lane. 7th flag; both goldens move; new `destructive_test.go` row. | |
| A dedicated `engram skills` command | `list` / `show <name>`. New registry rows, golden churn, conditional-rule conformance. | |
| You decide | Planner picks, subject to criterion 1 being satisfiable plugin-free and the text lane staying readable. | |

**User's choice:** Preview carries it, json lane only.
**Notes:** The tension surfaced was between REQ-setup-previews-by-default ("shows the exact
command or **content** that would be issued, not a summary of it") and Phase 3 D-15's dense
`key=value` row — 58,522 bytes across five skills, 31,386 of it in `curating-memory` alone.
Noted during discussion: the `engram skills` option's Phase 5 justification is weaker than it
appears, because `internal/setupgen` can import `internal/skills` in-process.

### Q4 — what the vendor step copies and how inventory is determined

| Option | Description | Selected |
|--------|-------------|----------|
| Whole subtree, structural discovery | Copy wholesale; `//go:embed all:data`; inventory is whatever the embedded FS walks. A sixth skill ships with zero code change. | ✓ |
| Structural + a pinned count canary | Same, plus a test asserting the inventory is exactly the published five, so an addition turns the suite red deliberately. | |
| Enumerate the five explicitly | Vendor step and a Go var both name the five. Fully explicit; a forgotten sixth silently never ships. | |
| You decide | Planner picks, subject to `all:` prefix and set-equality drift. | |

**User's choice:** Whole subtree, structural discovery.
**Notes:** The `all:` prefix is load-bearing — `internal/webauth/static_test.go:38` records a
bare `//go:embed` silently excluding `_`-prefixed subtrees, a failure this repo already hit.
Aligns with the repo's stated preference for structural predicates over enumerations that fail
by absence (memory `zcev96ng18`).

---

## Who writes files

### Q1 — where filesystem-write logic lives

| Option | Description | Selected |
|--------|-------------|----------|
| setup declares, skills installs | `Plan.SkillTarget` authored per runtime; `internal/skills` owns embed + `Install` + its FS seam; `cmd/engram` composes. `internal/setup` stays zero-I/O; T-02-01 never reopens. | ✓ |
| internal/setup does it all | `Environment` gains `ReadFile`/`WriteFile`/`MkdirAll`; `Action` gains a file-write variant; one executor. Requires moving the vendored skills into `internal/setup/data/`; reopens T-02-01. | |
| setup declares, cmd/engram installs | Same declarative `SkillTarget`, but the write loop lives in `cmd/engram/setup.go`; `internal/skills` is content-only. Splits the apply path across the cmd/internal boundary. | |
| You decide | Planner picks, subject to leafpurity staying green and per-runtime paths authored in each runtime's own file. | |

**User's choice:** setup declares, skills installs.
**Notes:** Partly forced rather than chosen — `leafpurity_test.go` fails on *any* same-module
import, so `internal/setup` can never import `internal/skills`. Also noted: for a file write,
Phase 3 D-08's convergence model gets simpler, since "already-correct" is just comparing
existing bytes to embedded bytes — no read→write→read, and no assumption about a third party's
output determinism.

### Q2 — reporting two facets on one runtime row

| Option | Description | Selected |
|--------|-------------|----------|
| One aggregated Outcome + facet fields | One `Outcome`, precedence `failed > wrote > already-correct > would-write > not-present`; facets as `registration=`/`skills=` row fields. `exit.go` untouched. | ✓ |
| Two rows per runtime | Each facet gets its own honest Outcome; nothing aggregated. Strains REQ-setup-partial-failure-legible's "per runtime"; the "N/M present" count needs redefining. | |
| Second Outcome field on Result | `Result.SkillsOutcome` consumed by `Classify`. Most precise; reopens the pinned exhaustive combination table. | |
| You decide | Planner picks, subject to mixed state never rendering as already-correct. | |

**User's choice:** One aggregated Outcome + facet fields.
**Notes:** Same posture Phase 3's D-16 took toward `Classify`. The precedence generalizes D-08's
invariant: mixed state resolves *up* to `wrote`, never down to `already-correct`.

### Q3 — the unit of failure

| Option | Description | Selected |
|--------|-------------|----------|
| Fully independent, accumulate everywhere | Registration and skills attempted independently; all skills attempted; failures accumulate via the `errors.Join` idiom. Nothing skipped because something else failed. | ✓ |
| Independent across facets, atomic within skills | Same independence, but skills are all-or-nothing via stage + rename, so no half-populated skills dir. Does not generalize to the AGENTS.md single-file path. | |
| Registration gates skills | Skip skills if `mcp add` fails; skills instructing `mcp__engram__*` calls are misleading with no registration. A transient failure costs the skills too; "skipped" is a sixth state. | |
| You decide | Planner picks, subject to errors.Join accumulation and an actionable Reason. | |

**User's choice:** Fully independent, accumulate everywhere.
**Notes:** Read as REQ-setup-partial-failure-legible's "no runtime's failure silently discards
another's success" applied one level down.

### Q4 — the overwrite contract

| Option | Description | Selected |
|--------|-------------|----------|
| Overwrite; byte-compare decides the outcome | Equal → `already-correct`, no write; differing/absent → write, report `wrote`, name the path. No marker, so installed bytes == vendored bytes == plugin bytes. | ✓ |
| Provenance header gates overwrite | Overwrite only when engram's header is intact; protects hand edits. Installed file is no longer byte-identical to the plugin's; drift identity needs strip-then-compare. | |
| Refuse to overwrite anything that differs | Never clobber; report failed with the path. Decisive flaw: a skill engram wrote from an older release blocks its own update. | |
| You decide | Planner picks, subject to REQ-setup-reconcile-hand-edits staying v2. | |

**User's choice:** Overwrite; byte-compare decides the outcome.
**Notes:** `setup` is already `Destructive: true` (Phase 2 D-13) and the milestone states
idempotent re-install *is* the update path. REQ-setup-reconcile-hand-edits is explicitly v2.

---

## Native targets

Framed up front, following Phase 3's treatment of the bearer-token expansion syntax: **which
runtimes have a native skill format, and what a runtime does with two same-named skills, are
third-party facts that were deliberately not asserted.** Both are recorded as research
preconditions. These questions were about policy only.

### Q1 — overlap with the Claude plugin, which already ships these five skills

| Option | Description | Selected |
|--------|-------------|----------|
| Write unconditionally; coexistence is a research precondition | Always install to the native user-scope dir; whether a runtime dedupes, shadows or shows both is for the researcher to establish. Keeps Phase 2 D-12 (LookPath-only detection) intact. | ✓ |
| Detect the plugin and stand down | Report `skills=already-correct source=plugin`, write nothing. No duplicates by construction; reverses P2 D-12 and reads a third-party config layout. | |
| Delegate to the runtime's plugin CLI | `claude plugin install engram@…`. One source of truth, consistent with the shell-out posture; needs network and undercuts the plugin-free goal. | |
| You decide | Planner picks after research, subject to P2 D-12 only being reversed deliberately. | |

**User's choice:** Write unconditionally; coexistence is a research precondition.

### Q2 — install scope

| Option | Description | Selected |
|--------|-------------|----------|
| User scope only | Mirrors Phase 3's `--scope user`; destinations derive from the existing `Environment.HomeDir` seam. Sidesteps the symlinked-project-AGENTS.md footgun. | ✓ |
| Project scope (cwd) | Guidance travels with the repo in version control. Incoherent with a globally registered server; makes the result cwd-dependent. | |
| Both, behind a --scope flag | Most flexible, mirrors `claude mcp add --scope` naming. 7th flag, both goldens, a `destructive_test.go` row, a conditional-rule entry — for a knob no requirement asks for. | |
| You decide | Planner picks, subject to using `Environment.HomeDir` and applying uniformly to both install shapes. | |

**User's choice:** User scope only.
**Notes:** This question also settles which AGENTS.md the fallback targets. The concrete footgun
raised: this repo's own `./AGENTS.md` is a symlink to `CLAUDE.md`, so a cwd-scoped fallback
would write straight through into a project's real instruction file.

### Q3 — the `generic` pseudo-runtime

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — skills ride in generic's deliverable | Row grows a skills payload alongside `Config`; full content in the json lane per D-03. Outcome stays `would-write` in both lanes; `exit.go` not reopened. | ✓ |
| No — generic is registration-only | Explicit "none" SkillTarget; no machine to write to. Smallest change; a user on an unsupported client gets no curation guidance at all. | |
| You decide | Planner picks, subject to P3 D-16 and D-15 holding. | |

**User's choice:** Yes — skills ride in generic's deliverable.
**Notes:** REQ-register-generic-mcp frames generic as "a documented manual path rather than a
dead end"; a dead end for the guidance is still a dead end, and this is the one class of user
with no other way to obtain it.

### Q4 — opt-out flags

| Option | Description | Selected |
|--------|-------------|----------|
| No opt-out — setup does both, always | `--runtime` remains the only selector. No new flags, no golden movement, no conditional-rule entry. | ✓ |
| A --skills=false style opt-out | Register without touching skill files; useful in CI or where guidance is managed elsewhere. 7th flag plus its pinned surface. | |
| Two selectors: --skills-only and --skills=false | Full independence both directions, matching the failure model. Two flags plus a mutually-exclusive flag-group rule. | |
| You decide | Planner picks, subject to any new flag registering a conditional rule and moving goldens deliberately. | |

**User's choice:** No opt-out — setup does both, always.
**Notes:** The overwrite contract already makes a re-run cheap and honest, which answers the
usual reason to want a skip. Promotion to a flag later is purely additive — the same disposition
Phase 3's D-12 gave `--timeout`.

---

## AGENTS.md block

### Q1 — what goes inside the delimited block

| Option | Description | Selected |
|--------|-------------|----------|
| Index in the block, full SKILL.md files beside it | ~1KB block: one line per skill with its on-disk path; verbatim SKILL.md files written next to AGENTS.md. Mirrors engram's own rules-index progressive disclosure; preserves byte-identity. | ✓ |
| Full text of all five, inlined | Most literal reading of the requirement; one location, nothing to fetch. ~15K tokens added to every session on that runtime, permanently. | |
| Derived condensed guidance, inlined | Small, self-contained, tuned for always-on context. Decisive flaw: a second authored artifact, so "cannot drift" stops being true unless the condensation is itself generated and drift-gated. | |
| You decide | Planner picks, subject to the skills' bytes staying identical to the plugin's and the block being replaceable in place. | |

**User's choice:** Index in the block, full SKILL.md files beside it.
**Notes:** Sizing that framed the question: 58,522 bytes total, ~15K tokens, in a file loaded
into *every* session, for five skills of which a typical session needs zero or one.

### Q2 — source of each skill's index line

| Option | Description | Selected |
|--------|-------------|----------|
| A new authored frontmatter key | Each SKILL.md gains a short authored one-liner (working name `summary:`) shipped by the plugin and embedded with it. Mirrors engram's own rule contract, where `summary` must be a single line and *is* the index entry. | ✓ |
| Frontmatter `description` verbatim | Zero new keys, drift-proof by construction. These are exhaustive trigger lists, not index lines — `curating-memory`'s is ~1,400 chars; the five total ~3.4KB. | |
| `description`, mechanically truncated | First sentence or clause. Truncation is what engram's own summary guidance warns against; `curating-spine`'s load-bearing "never mutates without consent" clause would be silently dropped. | |
| You decide | Planner picks, subject to the index text being sourced from the embedded files and never dropping a negation. | |

**User's choice:** A new authored frontmatter key.
**Notes:** Recorded as a research precondition — confirm Claude Code and any other native
consumer tolerate an unknown frontmatter key without warning or rejecting the skill.

### Q3 — malformed or ambiguous AGENTS.md

| Option | Description | Selected |
|--------|-------------|----------|
| Fail loudly, never guess | Only zero blocks (append) or one well-formed block (replace) are writable. 2+ blocks, unmatched begin, or unmatched end → `OutcomeFailed` with marker offsets named; nothing written. | ✓ |
| Repair and converge | Collapse duplicates, treat unmatched markers as absent and append fresh. Always converges; engram silently rewrites a region it did not author. | |
| Fail on duplicates, tolerate unmatched | Handles the likeliest accident without a hard stop; leaves a stray marker orphaned forever and appends past it each run. | |
| You decide | Planner picks, subject to no partial writes and a Reason a human can act on. | |

**User's choice:** Fail loudly, never guess.
**Notes:** Generalizes Phase 3 D-08's invariant — ambiguity resolves to the safe report, never
to a guess — and honors "byte-for-byte untouched outside the block" in the case where honoring
it is hard.

### Q4 — write mechanics and symlinks

| Option | Description | Selected |
|--------|-------------|----------|
| In-place write, through symlinks | Read whole file, splice in memory, one `os.WriteFile` call. Follows symlinks, so a dotfiles-managed AGENTS.md keeps working. Not atomic; window is one write syscall of a few KB. | ✓ |
| Atomic rename onto the resolved target | `EvalSymlinks`, stage in the resolved dir, rename. Atomic *and* dotfiles-preserving; writes a path the operator did not name, and cross-device rename needs a fallback. | |
| Atomic rename, refuse symlinks | Durable; never silently replaces a symlink. Hard-fails a very common dotfiles setup whose only remedy is to un-manage the file. | |
| You decide | Planner picks, subject to never silently replacing a symlink and reporting the path actually written. | |

**User's choice:** In-place write, through symlinks.
**Notes:** The trap named during discussion: `os.WriteFile` on an existing symlink writes
*through* it, but temp-file + `os.Rename` *replaces* it with a regular file — silently breaking
a stow/chezmoi/yadm setup, which is a common way `~/.codex/AGENTS.md` exists at all. Loss of
atomicity was accepted explicitly.

---

## Claude's Discretion

The user chose a concrete option on all sixteen questions; no area was answered "you decide".
The residual discretion recorded in CONTEXT.md is implementation shape only — delimiter syntax,
the `SkillTarget` discriminator, the filesystem seam's signature, field names and json tags,
the `summary:` key's exact name and length bound, file modes and umask, whether
`internal/skills` gets its own leaf-purity gate, and where the outcome-aggregation precedence
lives.

## Deferred Ideas

Full dispositions with revisit triggers are in `04-CONTEXT.md` `<deferred>`. In brief:
`--skills=false` / `--skills-only`; a dedicated `engram skills list|show` command; a
`--show-skills` flag; stage-and-rename atomicity; plugin detection and stand-down; delegating
to `claude plugin install`; a provenance header on installed skill files; a pinned five-name
inventory canary; repairing a malformed AGENTS.md; distributing the plugin's hooks and
`/engram-setup`; and backlog Phase 999.1, which D-02 deliberately does **not** close.
