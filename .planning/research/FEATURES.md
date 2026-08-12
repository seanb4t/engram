# Feature Landscape

**Domain:** Curation-over-time for a self-hosted, explicit, zero-junk memory store (v0.13.x
"Curation & Self-Evidence"); self-describing CLI/MCP surfaces for coding agents.
**Researched:** 2026-08-03

This research covers two capability clusters for the v0.13.x milestone: (1) `engram
spine-review` — a curation operation split across verify/consolidate/purge/archive/review, and
(2) a correct-by-reading audit of every `cmd/engram/*` flag and `internal/server` MCP tool. Both
are scoped against engram's locked design invariant: **explicit, zero-junk, correctable — never
auto-extraction, never automatic supersession.** Existing capture/correction primitives
(`supersede_memory`, structured `citations`, `rules`, `prune-expired`, usage signals) are the
substrate this milestone curates, not features to re-build.

## Table Stakes

Capabilities an operator/agent will expect from a "curation" release; missing any of these
makes `spine-review` feel like a stub, or the self-describe audit feel cosmetic.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Citation liveness/reference check** (Verify) | The store already carries structured `citations` (`kind`: file/commit/url/repo, `ref`, `locator`, `pin`). A curation pass that never checks whether those anchors still resolve is not curation. Matches the DOCER/docverity "reference checker" pattern: deterministic, no LLM, exact/near-match of a token against the current source tree. | Low–Medium | Dispatch by citation `kind`: file/locator → does the path+line still exist and roughly match; commit → is the sha reachable; url → HTTP liveness (link-rot check, à la `linkspector`/`urldn-link-check`); repo → resolvable. #355's drifted `tools.go` line-number citations are the fixture this verb exists to catch. |
| **Drift tolerance, not exact-match false alarms** (Verify) | Research is unanimous that naive "does this line number still say X" checking floods false positives on ordinary refactors (Ratol & Robillard's Fraco paper on fragile comments; DocPrism's 98%→14% flag-rate fix via filtering). A `locator` that has shifted a few lines but the referenced content is still present nearby should not fail; a deleted file/function/commit should. | Medium | Reuse the same class of signal DOCER uses: presence/absence of the referenced token in the current tree, not brittle exact-position matching. This is the difference between a useful gate and a noisy one nobody trusts. |
| **Consolidate surfaces candidates, never auto-merges** (Consolidate) | Every well-regarded prior art (Obsidian's Dualyze Notes, Prior Art, iwwx duplicate-finder) computes a similarity score, buckets it into a label (Duplicate / Merge candidate / Related), and stops — a human/agent decides, a draft is created alongside the originals, nothing is deleted or rewritten automatically. This is also the *only* pattern compatible with engram's own locked invariant that supersession is "never automatic (no similarity or write-through path)." | Low | Engram already embeds every memory (Qdrant vectors) — near-duplicate detection is a self-referential similarity query against the existing index, not new infrastructure. Report pairs above a threshold with the score and both `short_id`s; the *action* is the existing `supersede_memory` (correction) or `delete_memory` (junk), invoked by the agent under `curating-memory`'s existing gates. |
| **Purge respects extract-before-delete ordering** (Purge) | The milestone's own framing names this explicitly: "nothing enforces its ordering, detects when it is overdue, or verifies the extract happened before the delete." The archival literature converges on the same rule: a record moves through **live → archived (reversible, reasoned, audited) → purged (irreversible)** — never live straight to gone. `prune-expired` already exists; the gap is proving the audit trail/extraction step ran first, not adding a new delete primitive. | Low–Medium | Extend `prune-expired`'s existing `--dry-run`-shaped safety pattern rather than inventing a fourth record state (matches DEC-2bv's "no collection sprawl" and the "zero new dependency" tone of prior milestones). A dry-run listing *is* the extract step for an agent to consume before the real run. |
| **Review surfaces structural mismatch signals only** (Review) | The one "review" precedent already in this codebase is v0.12.x Phase 6: `store_rule` capture is a semantic judgment ("is this normative") that was deliberately routed to an agent-mediated, consent-gated procedure — never a CLI verb that classifies and promotes on its own. Research on note-taking/KB systems has no example of an automatic re-categorization feature that survived scrutiny; every one found (Obsidian merge tooling) stops at "propose," matching this precedent. | Low | `spine-review`'s Review output is a report of structural candidates (e.g., a `discovery` whose citation `pin`s have aged past DEC-3l0's graceful-decay signal; a `memory` that keeps getting hit by `list_rules`-adjacent queries) — never a mutation. Promotion stays `store_rule` after explicit user blessing, exactly as today. |
| **MCP tool descriptions state what/when/returns/does-NOT** | Anthropic's own tool-design guidance (and the MCP builder skill) is explicit: a description is the *only* contract an agent reads before calling. The pass/fail review bar for the Anthropic Directory requires stating what a tool does, when to use it, what it returns, and — critically — what it does **not** do, to disambiguate siblings (`search_memory` vs `search_discovery` vs `list_rules`). | Low | This is the audit engram already committed to (D-00, `4aksmneehh`): server-side conditional-requirement rules like `effectiveSearchScope`'s `cross_spine`/`scope` interaction must be stated on the argument itself, not only in `CLAUDE.md` or docs-site prose. |
| **MCP tool annotations (`readOnlyHint`/`destructiveHint`/`idempotentHint`)** | The March-2025 MCP spec addition exists precisely so a host can auto-approve reads and gate destructive calls without parsing prose. Every MCP-server review guide surveyed treats "most servers skip these entirely" as the single most common quality gap. | Low | Concrete, checkable: `delete_memory`/`delete_all`/`supersede_memory` → `destructiveHint: true`; `search_memory`/`list_memory`/`get_memory`/`list_rules` → `readOnlyHint: true`; idempotency-keyed `store_memory` → `idempotentHint` conditional on the key being present. |
| **`MarkFlagsMutuallyExclusive` replaces hand-rolled guards** | #453 is a documented-but-unenforced gap: `client_list.go` *says* `--offset`/`--cursor-mode`/`--page-token` are exclusive in help text but validates none of it, while two other call sites (`client_common.go:236`, `migrate.go:85`) each hand-roll their own guard. Cobra ships the declarative mechanism for exactly this and this repo uses it zero times — the fix is systematic, not a third bespoke check. | Low | Declaring exclusivity at flag-registration time makes help text and validation the same fact, so they cannot drift apart — the failure mode a hand-rolled guard cannot prevent. |
| **One exit-code taxonomy, or a named boundary** | #467: `engram search` returns exit 2 for a contradictory flag pair; `engram migrate-remap-owner` returns exit 1 for the same class, per D-09's deliberate carve-out — but no surface states which taxonomy governs which command. CLI guideline prior art (clig.dev, and the ranged-exit-code convention seen in Cobra ecosystem tooling: 0 success / 1–9 runtime / 10–19 bad input / 20–29 config) gives a ready-made scheme to adopt or explicitly diverge from. | Low | The requirement is not "invent a new scheme" — it's "pick one and document the one deliberate exemption" (the two-tier CLI error model already flagged as carried tech debt). |
| **CLI request timeout with a finite default** | #452: no RPC path applies a context deadline, so a cron-invoked `engram search|store|list` can hang forever. Every CLI-guideline source treats an unbounded network call as a baseline reliability bug, not a feature. | Low | `--timeout` flag + exit-code mapping for the deadline-exceeded case — small, mechanical, but table stakes for anything meant to run unattended. |

## Differentiators

Not expected by default, but genuinely distinguishes engram's curation/self-evidence surface
from comparable systems.

| Feature | Value Proposition | Complexity | Notes |
|---------|--------------------|------------|-------|
| **Pre-auth, pre-config machine-readable schema surface** | The emerging "CLI Spec" pattern (clispec.dev) argues a `schema` command should let an agent discover a tool's full capability — commands, args, types, defaults, required-ness, global flags, error taxonomy — **without** having run `--help` first, and **without** needing credentials or a config file, because that is precisely the moment an agent knows nothing about the tool. Engram already shipped a v0.12.x Phase 2 self-describe JSON catalog; extending it to state *conditional* requirements inline (not just flat schema) — e.g. "`--cross-spine` widens `--scope`; they name each other" — goes beyond what most CLIs do today. | Medium | Builds directly on the already-shipped catalog. The differentiator is inline conditional-requirement documentation, not the existence of the catalog itself (that part is arguably already table stakes, delivered). |
| **Citation-anchored verify as a live regression fixture** | #355 (drifted `tools.go` citation anchors) is explicitly scoped as *both* a bug and the reference fixture that proves the verify step works. Very few systems researched pair "we detect drift" with "here is a pinned, intentionally-drifted example that keeps the detector honest" — most drift-detection tools (DOCER, docverity) validate against a corpus, not a single committed adversarial fixture co-located with the feature. | Low | Reuses work already scoped; the differentiator is methodological rigor (a permanent negative-space test), consistent with this project's existing testing culture (e.g. the #261 retrieval-eval regression fixture). |
| **Consolidate reuses the existing vector index — no new dependency** | Nearly every RAG/note-app dedup tool surveyed either ships its own embedding model (Obsidian's Prior Art Pro) or a separate similarity engine (MinHash LSH, cross-encoders). Engram's memories are already embedded for recall; a self-referential nearest-neighbor query against the same Qdrant collection gets near-duplicate detection for free, honoring the "zero new Go dependencies" thread running through v0.11.x–v0.12.x. | Low | Genuine differentiator: cheapest possible implementation of a capability competitors build as a separate subsystem. |

## Anti-Features

Patterns repeatedly validated in the wild that this project must **not** adopt, because they
would silently mutate the store without user consent — directly contradicting the "explicit,
zero-junk, correctable" invariant and the existing locked decisions (`supersede_memory` has "no
similarity or write-through path"; usage signals "never affect ranking"; rules are "proposed,
never promoted unilaterally").

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|---------------------|
| **Threshold-based auto-merge on Consolidate** | The single most consistent failure mode in the research: RAG dedup pipelines that auto-collapse chunks above a cosine threshold silently destroy provenance, exception clauses, and version distinctions ("same intro ≠ same meaning"; dedup can "break citations without breaking retrieval" because the *mix* of what's retrievable quietly narrows). One production case study (Humza Tareen's adjudication-layer post) shows even a well-tuned single threshold rejects valid distinct records; the fix there was a human/LLM review band, never blind auto-merge. Engram's own locked decision already forbids this for supersession. | Consolidate reports candidate pairs + score; a human or the `curating-memory` skill decides whether to `supersede_memory` (correction) or leave both (legitimately distinct). Never wire a threshold directly to a write. |
| **LLM/semantic "is this still true" judgment inside the CLI** | This is precisely the split the milestone already made at scoping time: structural predicates (drifted citation, orphaned record, lapsed window) go to `engram spine-review`; semantic judgments ("is this record still true," "are these two the same fact") go to an agent procedure. Folding semantic verification into the CLI (as DocChecker/Cascade/DocPrism do for code-comment drift, at real cost — false-positive rates from 14% to 98% depending on technique) would blur a boundary this project already drew deliberately, mirroring v0.12.x Phase 6's routing of rule-capture. | Keep Verify's CLI half strictly structural (does the anchor resolve); route "is the claim still accurate" to the curation skill, same as the existing `curating-memory`/`discovering` split. |
| **Automatic importance-decay-driven archival or promotion** | Systems like AMV-L, Z3rno-style tiering, and MemTier's cognitive-weight loop are legitimate designs *elsewhere*, but they all silently move records between visibility tiers based on computed utility scores — exactly the kind of unattended mutation engram's v0.9.x usage-signal invariant ("usage signals never affect ranking… curation metadata, not a ranking input") was written to forbid. Extending usage counters into an automatic archive/promote job would quietly reverse that locked decision. | Usage signals (`access_count`/`last_accessed_at`) stay observational inputs a human/agent *reads* during Review; any archive/promote action stays an explicit, agent-initiated call, never a background scorer. |
| **Auto-promoting a memory to `rule` (or any category) based on structural signal** | Rules are "always-shared," normative, and the existing decision record is unambiguous: "an agent proposes a rule candidate… `store_rule` is invoked only after the user blesses it (never promoted unilaterally)." Extending Review to auto-reclassify a record — even from a confident structural signal like repeated cross-session hits — would be a silent category mutation exactly as forbidden as auto-supersession. | Review emits a report of *candidates for promotion*; the actual `store_rule` call requires the same consent-gated protocol already shipped in v0.12.x Phase 6. |
| **Hard-deleting on Purge without a prior archive/extract step** | Every archival system surveyed (ai-memory MCP's three-state live/archived/gone model, Z3rno's soft-delete-then-audit-trail) treats "irreversible" as a distinct, later, deliberate step from "no longer live." Wiring Purge to the same code path as an unreviewed delete reintroduces exactly the ordering bug the milestone names: "verifies the extract happened before the delete" is a stated non-negotiable, not a nice-to-have. | Purge only acts on records already surfaced (and, where relevant, extracted/exported) by a prior dry-run/Verify pass; `--dry-run` stays the default posture, matching `prune-expired`'s existing shape. |
| **Global prose repeating a per-tool conditional rule instead of stating it at the point of use** | Both Anthropic's tool-writing guidance and the `agent-interface-design` skill converge on this: restating the same behavioral instruction in a global preamble (README/CLAUDE.md) *and* the tool description is how a codebase grows contradictions over time — one gets updated, the other doesn't. Engram's own D-00 principle ("correct-by-reading") requires the rule live where the argument is used. | Conditional requirements (e.g. `effectiveSearchScope`'s `cross_spine`/`scope` interaction) must be stated in the MCP tool's own `inputSchema` description / the flag's own help string. `CLAUDE.md`/docs-site may *index* it, but must not be the sole location. |
| **Adding a usage example to compensate for an underspecified schema** | The `agent-interface-design` skill names this directly as a diagnostic, not a fix: "the urge to add a usage example… usually means a parameter is underspecified." For #453/#467/the self-evident-surface audit, the correct fix is pushing the constraint into the schema (an enum, `MarkFlagsMutuallyExclusive`, a typed exit-code range) — not writing a longer example that an agent may or may not generalize from correctly. | Where a constraint can be expressed structurally (mutual exclusivity, enum, min/max), express it there first; reserve prose examples for genuinely bespoke syntax that cannot be typed. |

## Feature Dependencies

```
citations (shipped, v0.11.x)         → Verify (checks citation ref/locator liveness)
embeddings/vector index (shipped)    → Consolidate (self-referential nearest-neighbor query)
supersede_memory (shipped, v0.11.x)  → Consolidate's *action* (never the detector itself)
prune-expired (shipped, v0.10.x)     → Purge (extends dry-run/older-than shape; no new primitive)
usage signals (shipped, v0.9.x)      → Review (read-only input signal; never a mutation trigger)
rules + curating-memory skill        → Review's promotion path (consent-gated, agent-mediated)
v0.12.x Phase 2 self-describe JSON   → Self-evident surface audit / schema differentiator
v0.12.x Phase 6 rule-capture routing → precedent for Verify/Consolidate/Review's CLI-vs-skill split
```

Structural predicates (drifted citation, near-duplicate score, lapsed window, orphaned record) →
resolved by `engram spine-review` (CLI, deterministic, no LLM required for the base case).
Semantic predicates ("still true," "same fact," "should be promoted") → resolved by the curation
skill (agent judgment, consent-gated) — this is the split the milestone already locked at
scoping; the research confirms it is the same split every comparable system converges on when it
tries to avoid the auto-merge/auto-promote failure modes above.

## MVP Recommendation

Prioritize:
1. **Verify** — citation liveness/reference check (deterministic; reuses the shipped `citations`
   shape; #355 is the ready-made fixture).
2. **Self-evident surface audit's mechanical fixes** — `MarkFlagsMutuallyExclusive` (#453), CLI
   timeout (#452), one exit-code taxonomy or a documented boundary (#467), MCP tool annotations
   — all low-complexity, all closing named, already-filed gaps.
3. **Consolidate (report-only)** — near-duplicate candidate surfacing via the existing vector
   index, propose-only, never wired to `supersede_memory` automatically.

Defer:
- **Full archive-tier redesign** (a genuine fourth record state distinct from
  superseded/scheduled-expired): the research supports the *pattern* but engram's existing
  soft-hide + `not_after` + `prune-expired` shape likely covers the milestone's stated need
  without a new store primitive — revisit only if `prune-expired`'s dry-run extension proves
  insufficient.
- **Any decay/promotion scoring model** (AMV-L/Z3rno-style tiering): explicitly out of scope —
  see Anti-Features. Would require reopening the v0.9.x usage-signal invariant.

## Sources

- Detecting outdated code element references (DOCER), *Empirical Software Engineering* 2023 — https://link.springer.com/article/10.1007/s10664-023-10397-6 (HIGH — peer-reviewed)
- "Wait, wasn't that code here before?" (DOCER Tool / GitHub Action), arXiv 2023 — https://arxiv.org/html/2307.04291 (HIGH)
- DocPrism: Local Categorization, External Filtering, arXiv 2511.00215 — https://arxiv.org/html/2511.00215v1 (HIGH — false-positive-rate data)
- Cascade: test-generation-based doc/code consistency, arXiv 2604.19400 — https://arxiv.org/html/2604.19400v1 (HIGH)
- CoCC / DocChecker code-comment consistency papers — https://doi.org/10.48550/arxiv.2403.00251, https://arxiv.org/html/2306.06347v3 (HIGH)
- `docverity` (MCP-integrated doc-drift checker, reference + coverage passes) — https://github.com/deveshagarwal/docverity (MEDIUM — project README, cross-checked against the peer-reviewed drift-detection literature above)
- Dualyze Notes / Prior Art / iwwx duplicate-finder (Obsidian near-duplicate UX) — https://community.obsidian.md/plugins/dualyze-notes, https://community.obsidian.md/plugins/prior-art, https://github.com/iwwx/obsidian-duplicate-finder (MEDIUM — plugin docs)
- RAG dedup failure modes — "The Dedup Rule That Broke Our RAG," "The RAG Dedup Step That Broke Silently," CACD (arXiv 2607.24332), adjudication-layer case study — https://medium.com/@sparknp1/the-dedup-rule-that-broke-our-rag-02a4d58acf25, https://tianpan.co/blog/2026-06-03-the-rag-dedup-step-that-broke-silently-and-flooded-your-top-k-with-near-duplicates, https://arxiv.org/html/2607.24332, https://humzakt.github.io/blog/llm-adjudication-dedup-false-positives.html (MEDIUM/HIGH mixed — one arXiv paper, three practitioner case studies)
- Memory retention/decay/archive patterns — AMV-L (arXiv 2603.04443), MemTier (arXiv 2605.03675), ai-memory MCP archival docs, Z3rno memory-lifecycle docs — https://arxiv.org/pdf/2603.04443, https://arxiv.org/html/2605.03675, https://alphaonedev.github.io/ai-memory-mcp/archival.html (MEDIUM/HIGH mixed)
- Anthropic, "Writing effective tools for AI agents" — https://www.anthropic.com/engineering/writing-tools-for-agents (HIGH — vendor-authoritative)
- Anthropic MCP builder skill / tool-design reference — https://github.com/anthropics/claude-plugins-official/blob/main/plugins/mcp-server-dev/skills/build-mcp-server/references/tool-design.md, https://github.com/anthropics/skills/blob/main/skills/mcp-builder/reference/mcp_best_practices.md (HIGH)
- MCP tool schema design guide — https://gingerlabs.ai/blog/mcp-tool-schema-design (MEDIUM)
- CLI design guidelines — clig.dev, "The CLI Spec" (clispec.dev), ThoughtWorks CLI DX blog — https://clig.dev/, https://clispec.dev/, https://www.thoughtworks.com/en-us/insights/blog/engineering-effectiveness/elevate-developer-experiences-cli-design-guidelines (MEDIUM)
- `structcli` (Cobra self-describing/AI-native add-on: `--jsonschema`, help topics, ranged exit codes) — https://github.com/leodido/structcli/blob/main/docs/ai-native.md (MEDIUM — project docs, useful as prior art for #467/#453)
- Link-rot/citation checkers — `linkspector`, `urldn-link-check`, `paper-verify`, `CiteSentry` — https://github.com/UmbrellaDocs/linkspector, https://github.com/URLdn/link-check, https://github.com/nolainjin/paper-verify, https://github.com/mkassaf/CiteSentry (MEDIUM — project READMEs)
- Internal: `.planning/PROJECT.md` (v0.13.x milestone scope, locked decisions DEC-2bv/DEC-90w/DEC-iedk/D-00), `CLAUDE.md` (memory contract, supersede/rule/discovery design intent) (HIGH — first-party)
