# Feature Research

**Domain:** Headless-agent CLI + cross-scope semantic search + explicit-capture UX + authz/error
diagnosability, for an existing self-hosted memory MCP server (engram, v0.12.x milestone)
**Researched:** 2026-07-29
**Confidence:** MEDIUM (web/community consensus across multiple independent sources; no
project-specific benchmarking; the explicit-capture findings synthesize PIM/PKM literature
that has not been validated against engram's own usage data)

## Feature Landscape

### Table Stakes (Users Expect These)

Things a headless-CLI-consuming subagent, or an operator debugging a denial, will consider
broken if missing — not differentiators, just the entry price for #343/#347/#360/#394.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| JSON output on `engram search\|store\|list` (default-on when piped, or explicit `--json`) | The primary caller is a subagent with `Bash` and no MCP tools — it parses stdout programmatically. Every agent-CLI convention surveyed (CLI Spec, cli-agent-spec, InfoQ, Terry Li's seven patterns) treats structured stdout as non-negotiable; a `--json` flag alone is the *minimum*, default-on-when-piped is the emerging bar. | LOW | engram already has typed Connect stubs (`gen/go/`) to marshal from — this is a thin CLI-layer concern, not a new data model. |
| stdout=data / stderr=diagnostics separation | An agent piping `engram search ... | jq` breaks the instant a progress spinner or log line lands in stdout. | LOW | Cobra's default `fmt.Println` habits must be audited; route all non-payload output through stderr. |
| Semantic, documented exit codes (0 ok; distinct codes for auth-failure / not-found / validation / conflict) | The agent branches on `$?` *before* parsing the body — a single "1 for everything" code forces it to parse prose to find out what happened, which is unreliable. | LOW-MEDIUM | Map onto engram's existing error taxonomy (`ErrNotFound`, `ErrIdempotencyConflict`, `ErrAlreadySuperseded`, 401/403) — the store layer already distinguishes these; the CLI just needs to preserve the distinction through to a process exit code instead of collapsing to a generic failure. |
| Non-interactive by default, `--yes`/no TTY prompts | The primary caller cannot answer a stdin prompt; a CLI that blocks on confirmation hangs the whole agent loop with no timeout signal. | LOW | engram's writes (`store`, presumably future mutating subcommands) must never gate on an interactive confirm; this is a design constraint on any subcommand added later, not just the three shipped in #343. |
| Token + server URL via env var, flag override | Bearer-token identity via `auth.ChainVerifier` (already shipped in v0.11.x) needs a CLI-side carrier. Every agent-CLI credential-injection source surveyed agrees: env var is the safe default (never printed, never in shell history via a flag), CLI flag is the override, never require an interactive OAuth browser flow for the headless path. | LOW | `ENGRAM_` prefix is already the project's env-var convention (`ENGRAM_OPENAI_BASE_URL` etc.) — `ENGRAM_TOKEN`/`ENGRAM_SERVER_URL` (or similar) is the obvious extension, consistent with the existing `internal/config` koanf field registry rather than a bespoke CLI-only config path. |
| Actionable error body naming the failing field + a concrete fix, not a bare status/stack trace | This is the direct target of #360 (over-long `summary` misreported as missing `content`). Every LLM-tool-error-design source agrees: the agent literally acts on the error string; a wrong or vague field name sends it down the wrong self-repair path. | LOW-MEDIUM | The fix for #360 specifically is a misclassification bug (wrong field cited), not a missing-feature — but the underlying *pattern* (validation errors must name the true offending field/constraint) generalizes to every future argument-validation surface. |
| Foreign-scope results visibly tagged on cross-spine search | `cross_spine=true` (#344) returns records outside the caller's default scope. Every cross-tenant/cross-project search product surveyed (Cloudflare AI Search, Elastic CPS) tags each result with its source scope — never silently merges scopes into an undifferentiated list, because the caller (here, a coding agent) needs to know a "convention" memory came from a *different* repo before treating it as locally authoritative. | LOW | engram's existing record shape already carries `scope` — cross-spine search needs to *surface* it in the result payload (likely already does implicitly via the `scope` field on each memory), not invent a new mechanism. |
| Authz denial reason reaches a reader (not silently dropped) | `authz.Decision.diag` is computed on every decision today and has zero readers (#394) — this is a pure legibility gap, not a new capability. Cedar's own `Diagnostics` API (policy IDs that fired, in permit or forbid) is the baseline operators expect from *any* Cedar deployment; leaving it uncollected is below the bar Cedar itself sets. | LOW | Wiring is debug-level logging / span events per the milestone scope — bounded, no new authz primitive, consistent with DEC-wot's owner-only PII rule already governing this data. |

### Differentiators (Competitive Advantage)

Where engram can go further than the table-stakes bar, aligned with the milestone's actual
scope (#343/#344/#351/#394/#347/#360) and the project's Core Value (correctable recall
precision + explicit zero-junk capture).

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Self-describing CLI (bare `engram` or `engram --help` returns full command/flag/error catalog) | Multiple agent-CLI specs (cli-agent-spec's tool-manifest pattern, Terry Li's "bare invocation returns the command tree") converge on this as the single highest-leverage move: one call replaces N `--help` round-trips for a subagent discovering the surface for the first time, and removes prose-parsing as a failure mode entirely. | LOW-MEDIUM | cobra already generates structured command trees internally; exposing it as a `--json`-flavored manifest is largely plumbing, not new design. Natural fit for `engram search|store|list` since it's a small, closed command set — cheap to get exactly right once rather than grow ad hoc. |
| Structured error envelope with a `fix`/`hint`/`next_actions` field (not just a message) | Goes beyond "state what's wrong" to "state the runnable next command" — the geodocs.dev agent-error spec and multiple production write-ups (Karigor AI Labs' "error-as-directive") show this measurably reduces agent retry loops vs. plain-text errors, because the agent doesn't have to infer the fix from a description. | MEDIUM | Natural extension of the #360 fix — once the CLI/Connect error mapper correctly names the true offending field, adding a `hint` string ("summary exceeds N bytes; shorten or omit") is a small additive step with outsized payoff for autonomous callers. |
| `cross_spine` search result carries a coverage/provenance marker, not just a foreign-scope tag | Beyond simple tagging, a production cross-repo-search thread (claude-context #374) argues a caller needs to know *whether* the search actually covered the intended universe — which scopes were searched vs. skipped, and why — to avoid confidently under-searching. For engram this is lighter-weight: at minimum, each cross-spine result should carry the `scope` it came from (table stakes); a differentiator is making it clear in the CLI/MCP response summary how many distinct scopes were searched, so an agent doesn't mistake "found nothing" for "searched everywhere and found nothing." | LOW | Cheap relative to full receipt schemas seen in the wild — engram's scope space is small and enumerable (unlike a 103-collection code-search fleet), so a one-line `scopes_searched: [...]` alongside results is sufficient; do not build the full "selection receipt" apparatus those large-scale systems needed. |
| Rule-capture friction reduction that keeps the explicit-consent gate (see Item 3 below) | The milestone explicitly targets "why rule capture never fires" (#351). The intervention surface below is a genuine differentiator *if* implemented as friction reduction rather than automation — most competitors in the PKM space either stay fully manual (low capture) or quietly slide into auto-extraction (violates engram's design invariant). Landing correctly in between is the differentiator. | MEDIUM | See dedicated section below — this is the highest-value, highest-risk item in the milestone and deserves its own analysis, not a one-line table entry. |
| Provider error-body surfacing on failed embeds (#347) | Bounded prefix of the upstream (OpenAI-compatible) provider's actual error text, rather than a generic "embed failed" — turns an opaque 5xx into an actionable diagnostic for an operator (and, if surfaced through a tool-call path, for an agent deciding whether to retry). Aligns with the LLM-tool-error research consensus that raw vendor error text (bounded, not full-length) beats a swallowed generic failure. | LOW-MEDIUM | Scope is explicitly bounded ("bounded prefix... then drain for keep-alive") — this is a deliberate anti-unbounded-leak design already locked into the milestone description, consistent with the PII-boundary posture used elsewhere (DEC-wot). |

### Anti-Features (Commonly Requested, Often Problematic)

Features that look like natural extensions of this milestone's scope but would violate an
explicit design invariant, over-scope a bounded CLI, or trade a real problem for a worse one.
**Anything that erodes explicit-consent capture is called out explicitly and is Out-of-Scope by
project decision, not merely deprioritized.**

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| Auto-promoting a memory to a `rule` after N similar memories accumulate, or an agent-inferred "this looks normative" heuristic | Directly "solves" the low rule-capture-rate problem measured in the milestone (one rule repo-wide vs. dozens of memories) — tempting because it's a mechanical fix to a mechanically-measured gap. | This is auto-extraction of normative ground truth without user blessing — the exact thing `store_rule`'s design (`curating-memory` skill: "propose it to the user and let them bless it — never promote one unilaterally") exists to prevent. It also compounds: a wrongly-promoted rule is *shared* and *immutable* by design (DEC-iedk), so a bad auto-promotion is maximally hard to walk back. **Explicitly Out-of-Scope** per PROJECT.md ("Auto-extraction of memories... deliberately excluded to keep recall zero-junk"). | Lower the *friction* of the existing explicit ask (see Item 3 intervention candidates below): a session-start or write-time *prompt/offer*, never a promotion. |
| An agent-side heuristic that silently supersedes a memory when new content looks similar enough (embedding-similarity auto-supersede) | Would make the store "self-heal" stale facts without a round trip, which sounds like it strengthens correctable recall. | `supersede_memory`'s design explicitly forbids this: "Never automatic. Do not supersede on a similarity hunch or as a write-through side effect" (curating-memory skill). A false-positive auto-supersede silently hides a record from recall — the opposite of correctable. | Keep supersession explicit; if reducing friction is the goal, reduce the *call* friction (e.g., a CLI `engram search` result that echoes the exact `supersede_memory` invocation an agent could run), never the *decision*. |
| A CLI `--auto-json` mode that infers whether output should be JSON from response size/shape rather than TTY/flag | Feels convenient — "just do the right thing." | Non-deterministic format selection is exactly the failure mode the CLI-Spec / cli-agent-spec literature calls out: an agent that gets JSON on one call and text on a structurally-identical one cannot build reliable parsing logic, and the ambiguity is undetectable until it silently breaks a downstream `jq` pipeline. | TTY-detection (`isatty`) plus an explicit override flag is the only supportable rule; document the default in `--help`/manifest output so it's a contract, not a guess. |
| Global-default cross-spine search (i.e., flip `cross_spine` to default `true` for all recall) | "Search everywhere" is the more useful default per general search-UX research (uxmovement: global-with-scoped-narrowing beats scoped-by-default for discovery). | engram's per-actor isolation and scope model exist specifically so an agent's recall stays clean and repo-local by default; the milestone explicitly frames `cross_spine` as an **opt-in** parameter mirroring `search_discovery`'s existing `CrossSpine`, not a default flip. Flipping the default would also silently change relevance characteristics for every existing caller (Core Value: "per-actor isolation that keeps each agent's recall clean") and contradicts the "Posture note" precedent already set for headless Connect mounting ("opt-in only, never a default flip"). | Keep `cross_spine=false` default; the differentiator opportunity is in the *quality* of opt-in cross-spine results (provenance tagging above), not in making it the default. |
| Verbose/full Cedar expression-trace logging on every denial (dump every attribute value considered) | Looks like the most complete diagnostic — "give operators everything." | OPA's own maintainers rejected always-on full tracing for this exact reason (github.com/open-policy-agent/opa#2897): traces are expensive to compute/copy, don't scale with data-set-sized rule bodies, and routinely leak sensitive input values (claims, PII) into logs. Directly conflicts with DEC-wot's owner-only PII rule already governing `authz.Decision.diag`. | Log which policy ID(s) fired (permit/forbid) and a short rule-name-level reason — the Cedar `Diagnostics.reason()` shape — at debug level; reserve full trace-level verbosity (if ever built) behind an explicit, separately-gated debug flag never on by default. |
| A CLI that retries mutating calls internally on ambiguous failure (e.g., silently retries `store` on a timeout) | Feels resilient — "the CLI should just handle transient errors for the agent." | Every agent-CLI and tool-error-handling source surveyed agrees this is an anti-pattern for *mutating* operations without an idempotency key: a timeout doesn't tell you whether the write landed, so a blind retry risks duplicate records. engram already has `idempotency_key` support (v0.11.x) — the correct fix is to require/encourage it on the CLI's `store` path, not to paper over the ambiguity with hidden internal retries. | Surface the ambiguous-timeout case explicitly to the caller with the exit-code/error-envelope pattern ("write may have landed; retry is safe only with an idempotency_key" / "retry not attempted — supply --idempotency-key to retry safely"); let the agent decide. |

## Feature Dependencies

```
Bearer-token identity on Connect lane (#343, shipped foundation via auth.ChainVerifier)
    └──requires──> Connect mounted headless, opt-in (#343)
                       └──requires──> engram search|store|list subcommands (#343)
                                          └──requires──> JSON output contract (table stakes)
                                          └──requires──> Semantic exit codes (table stakes)
                                          └──requires──> Token/URL via env var (table stakes)
                                          └──enhances──> Self-describing CLI / manifest command (differentiator)

cross_spine=true on search_memory (#344)
    └──requires──> existing scope model + per-actor isolation (v0.8.x, shipped)
    └──enhances──> Foreign-scope result tagging (table stakes)
                       └──enhances──> Coverage/provenance marker (differentiator)

Rule-capture friction reduction (#351)
    └──requires──> curating-memory skill's existing explicit-ask gate (shipped, must be preserved)
    └──conflicts──> Auto-promotion / auto-supersede anti-features (explicitly excluded)

authz.Decision.diag reaching a reader (#394)
    └──requires──> Cedar's existing Diagnostics API (shipped, internal/authz)
    └──requires──> DEC-wot PII boundary (existing constraint, not new)
    └──conflicts──> Full expression-trace logging (anti-feature)

Argument-validation error quality (#360)
    └──requires──> correct root-cause field attribution (bug fix, not new feature)
    └──enhances──> Structured error envelope with hint/next_actions (differentiator)

Embed provider error-body surfacing (#347)
    └──requires──> bounded-prefix truncation (already scoped in milestone description)
    └──enhances──> Structured error envelope pattern (shared design with #360)
```

### Dependency Notes

- **The CLI subcommands (#343) require the JSON/exit-code/env-var table stakes to exist as a
  baseline before any differentiator (manifest command, structured hints) is worth building.**
  Sequencing: ship the boring, correct contract first — it is also the highest-leverage item
  since #344's `cross_spine` and future subcommands will all ride on the same output/error
  conventions.
- **Foreign-scope tagging (table stakes) must land before the coverage/provenance differentiator** —
  the differentiator is additive metadata on top of the same result shape, not a separate feature.
- **Rule-capture friction reduction explicitly must NOT touch `store_rule`'s user-blessed gate.**
  Any intervention that changes *when the system offers to help* is fine; any intervention that
  changes *who decides* is the anti-feature. This is the single most important dependency
  boundary in the milestone — see the dedicated analysis below.
- **The authz-diagnostics and argument-validation-error work share a design pattern** (bounded,
  structured, PII-conscious disclosure) even though they touch different subsystems — a single
  internal convention (e.g., a shared "diagnostic envelope" shape) would let both land coherently
  rather than as two ad hoc mechanisms.
- **`Decision.diag` wiring (#394) has zero new-primitive risk** — it is strictly "connect an
  existing computed value to a reader," so it has no dependency on any other item in this list
  and can be sequenced independently/early.

## MVP Definition (v0.12.x scope only — not a general product MVP)

### Launch With (v0.12.x)

Minimum for the milestone's own stated goals ("make engram usable by agents that aren't a
top-level MCP client, and make what the server decides and rejects legible").

- [ ] `engram search|store|list` with default-JSON-when-piped, stderr-only diagnostics,
      documented exit codes, `ENGRAM_TOKEN`/`ENGRAM_SERVER_URL` env vars — table stakes, the
      spine of #343
- [ ] `cross_spine=true` on `search_memory` with per-result scope tagging — table stakes for #344
- [ ] `authz.Decision.diag` reaching debug logging/span events under the existing PII boundary — #394
- [ ] #360's root-cause fix (report the true offending field, not a misattributed one) — bug fix,
      table stakes
- [ ] #347's bounded provider-error-body surfacing — table stakes, already scoped narrowly

### Add After Validation (v0.12.x stretch, still in scope)

- [ ] Self-describing manifest command (`engram <no-args>` returns the command/flag/error catalog)
      — cheap, high-leverage, but not blocking the milestone's core headless-reach goal
- [ ] Structured error envelope with `hint`/hoped-for-next-command field, generalizing #360's fix —
      wait until the root-cause fix (table stakes) is proven correct before layering UX on top
- [ ] Rule-capture friction interventions (see below) — genuinely the milestone's #351, but should
      land only after root-causing *why* capture doesn't fire (the milestone explicitly frames
      this as "investigate... then fix the actual break point" — do not guess at the intervention
      before the investigation)

### Future Consideration (beyond v0.12.x)

- [ ] Cross-spine "coverage receipt" (scopes searched vs. skipped-with-reason) — worth building
      only if/when engram's scope space grows large enough that "did I search everywhere?" becomes
      a real ambiguity; today's scope space is small and enumerable, so this is speculative
- [ ] Any richer manifest/introspection beyond the closed 3-command `search|store|list` surface —
      defer until the CLI surface itself grows past what a static `--help` can cover well

## Deep Dive: Item 3 — Why Rule Capture Never Fires, and What to Do About It

This is flagged by the downstream roadmapper as the most important, and most easily
mis-implemented, item in this research. The distinction below is load-bearing: **every
candidate intervention is scored explicitly on whether it changes friction or changes who
decides.** Only the former is compatible with engram's design invariant.

### What the research says about *why* explicit capture is underused

Two independent bodies of evidence converge on the same mechanism, from different angles:

1. **Classic PIM research** (Van Kleek et al., *Finders/Keepers* and *Note to Self*) found that
   time/effort and "cost of use" are the dominant reasons people abandon explicit capture tools
   in favor of ad hoc scraps — not a lack of perceived value in the captured artifact itself.
   Successful tools win on capture *speed*, not on organizational power.
2. **PKM-literature synthesis** (isophist, reviewing 27 years of research including a 2025 study
   of 2,303 `AGENTS.md`-style context files) locates the failure more precisely: users are
   "unwilling or unable to make structure explicit" *at the moment of capture*, because the cost
   of formalizing (here: deciding "is this normative enough to be a rule, and am I willing to
   assert that") is immediate and certain, while the payoff is speculative and deferred. Notably,
   the same research found that when an *agent* was left to auto-generate structure/context on
   its own initiative, the result was **worse** than no structure at all (and >20% more expensive)
   — only human-authored structure helped. This is direct evidence *against* solving low capture
   with auto-extraction, independent of engram's design invariant: auto-generated "rules" would
   likely be lower-value than the status quo, not just illegitimate.

Applied to engram specifically: `store_rule` requires the agent to (a) recognize a fact as
*normative* (not just durable — a narrower bar than ordinary memory), (b) interrupt its own flow
to *propose* it to the user, and (c) wait for explicit blessing before calling the tool. Every one
of those three steps is a "cost of formalizing" moment per the research above, and the
`curating-memory` skill currently states the *rule* (propose, never promote unilaterally) but
gives the agent very little procedural help doing so cheaply or repeatedly. The skill's guidance
is correct in principle but currently amounts to "remember to ask" — exactly the failure mode
Fogg's Tiny-Habits research says will not stick without a lower-friction trigger.

### Concrete, testable intervention candidates

Each candidate below states its mechanism, what specifically it changes, and — critically — an
explicit verdict on the explicit-consent boundary.

| # | Intervention | Mechanism | Consent boundary | Testable signal |
|---|---|---|---|---|
| 1 | **Session-start rule-candidate surfacing is a *reminder*, never a write.** Nudge the agent (in the session-start memory index, already surfaced today per `list_rules`) with a one-line count/hint: *"N rules exist for this repo. If the user states a MUST-follow constraint, propose `store_rule` — don't add it to a memory."* | Adds a proactive, low-cost trigger at the exact moment the agent's attention is already on memory (session start), rather than relying on the agent to recall the skill's guidance unprompted mid-conversation. | **Explicit-preserving.** No content is written; it is a trigger to *ask*, not a decision made on the agent's behalf. | A/B on rule-store call rate before/after the reminder is added to session-start context; cheap to instrument (count `store_rule` MCP calls per week). |
| 2 | **Propose-and-confirm as a single low-cost interaction, not two.** Today the skill says "propose it to the user and let them bless it" — but doesn't specify *how* cheaply. Standardize on: the agent states the candidate rule text plus a single explicit yes/no ask in the same turn ("Should I store this as a rule: `<text>`?"), rather than a longer negotiation. This mirrors the PIM research's "reduce decisions between intent and capture" finding, applied to the *proposal* step rather than the capture step. | Lowers the friction of the *ask*, not the friction of the *consent* — the user still explicitly says yes/no every time. | **Explicit-preserving.** The blessing is still a distinct, required user action per candidate. | Measure the fraction of proposed-but-never-confirmed rule candidates; if most proposals die from ambiguous or effortful confirmation UX, tightening the ask should raise the confirm rate without changing the total propose rate. |
| 3 | **Widen the trigger surface in the `curating-memory` skill's routing table**, so agents recognize normative-sounding language more reliably (e.g., explicit "always/never/must" phrasing in a user's message), *and propose*, rather than defaulting to ordinary `store_memory` because the bar for "is this a rule" felt ambiguous in the moment. | This targets a plausible root cause the milestone's own investigation (#351) needs to confirm: agents may be silently routing normative statements to ordinary memory because the routing decision is genuinely ambiguous today, not because they forget rules exist. | **Explicit-preserving.** Widening *when the agent proposes* doesn't touch *who decides* — the user still blesses or declines every candidate. | Instrument how often a memory later gets manually corrected/promoted by the user into a rule after the fact (a proxy for "agent under-proposed and user had to initiate") vs. how often a proposal is declined (a proxy for "agent over-proposed"). |
| 4 | **Investigate — before intervening — whether #351's "break" is mechanical rather than behavioral.** The milestone description explicitly separates "investigate why rule capture never fires" from "fix the actual break point," implying the team does not yet know if this is a UX/friction problem (addressed by 1-3 above) or a bug (e.g., a tool-call the agent attempts silently failing, a schema mismatch, or the skill's trigger conditions never actually firing in practice for reasons unrelated to friction). | N/A — this is not an intervention but a precondition. | N/A | Grep/trace actual `store_rule` invocation attempts (including failed ones) across recent sessions before assuming a UX fix is the right lever; a near-zero *attempt* rate points to a UX/friction fix (1-3), while a nonzero attempt rate with failures points to a mechanical bug. |

**Explicitly rejected as anti-features (do not implement, even if they would numerically raise
the rule count):**

- Auto-promoting a memory to a rule based on repetition, similarity clustering, or any heuristic
  — this is auto-extraction of normative ground truth, the exact thing `store_rule`'s design was
  built to prevent (DEC-iedk's always-shared-immutable visibility makes a wrong auto-promotion
  disproportionately costly to correct), and it is separately named Out-of-Scope in PROJECT.md.
- An agent "confidence threshold" that silently stores a rule without an explicit user
  confirmation when the agent judges the statement "clearly normative enough" — this removes the
  human blessing step entirely under a plausible-sounding heuristic; it is functionally identical
  to auto-promotion with extra steps.
- Any intervention that measures success purely by "rule count went up" without also measuring
  false-positive proposals (declined asks) — optimizing for raw count without a quality signal
  would implicitly pressure future iterations toward the rejected interventions above.

## Sources

- [The CLI Spec](https://clispec.dev/) — structured output / exit code / schema-introspection conventions for agent-consumed CLIs
- [cli-agent-spec (GitHub)](https://github.com/romamo/cli-agent-spec) — 74 documented CLI failure modes for agent callers, exit-code retryability contract
- [Keep the Terminal Relevant: Patterns for AI Agent Driven CLIs (InfoQ)](https://www.infoq.com/articles/ai-agent-cli/) — non-interactive defaults, versioned output contracts, MCP wrapping
- [Seven patterns for agent-facing CLIs (Terry Li)](https://terryli.hm/posts/seven-patterns-for-agent-facing-clis/) — JSON-only-by-default, HATEOAS envelope, bare-invocation command tree
- [Secrets Management for AI Agents: Credential Injection (AgentPatterns.ai)](https://agentpatterns.ai/security/secrets-management-for-agents/) — env-var injection before session start, wrapper-script credential isolation
- [Anthropic CLI credential-resolution docs (anthropics/skills)](https://github.com/anthropics/skills/blob/main/skills/claude-api/shared/anthropic-cli.md) — flag > env var > profile precedence pattern
- [googlecloudplatform/agent-aware-cli (Decision Hub)](https://hub.decision.ai/skills/googlecloudplatform/agent-aware-cli) — headless-auth requirement, --dry-run for mutating commands
- [Cloudflare AI Search: Search across multiple instances](https://developers.cloudflare.com/ai-search/how-to/search-multiple-sources/) — per-result source tagging, partial-failure-not-hard-failure
- [Elastic: Cross-project search scope management](https://www.elastic.co/docs/explore-analyze/cross-project-search/cross-project-search-manage-scope) — default-scope-per-space, query-level override
- [claude-context #374: Support Global Knowledge Base Across Multiple Projects (GitHub)](https://github.com/zilliztech/claude-context/issues/374) — coverage/selection-receipt proposal for large-scale cross-repo search
- [Global vs. Scoped Search (UX Movement)](https://uxmovement.com/navigation/global-vs-scoped-search-which-gives-better-results/) — global-default-with-scoped-narrowing UX argument
- [Finders/Keepers longitudinal PIM study (Van Kleek et al., MIT)](https://people.csail.mit.edu/emax/papers/chi2011-finders-keepers.pdf) — time/effort as dominant capture-tool abandonment cause
- [Note to Self (Van Kleek et al., CHI 2009)](https://doi.org/10.1145/1518701.1518924) — capture-speed-over-organization as the winning tool property
- [Why everyone hates building knowledge bases (isophist)](https://www.isophist.com/p/why-everyone-hates-building-knowledge) — structure-explicitness cost, 2025 AGENTS.md study findings, incremental/system-assisted formalization
- [The second brain was a write-only system (FlowVerify)](https://www.flowverify.co/blog/second-brain-write-only-pkm-2026) — capture-vs-retrieval bottleneck framing
- [Capture must be frictionless (How to Think AI)](https://www.howtothink.ai/learn/capture-must-be-frictionless) — Fogg Tiny Habits 30-second capture threshold
- [Authorization (Cedar Policy Language Reference)](https://docs.cedarpolicy.com/auth/authorization.html) — default-deny, forbid-overrides-permit, diagnostics/determining-policies model
- [Diagnostics in cedar_policy (docs.rs)](https://docs.rs/cedar-policy/latest/cedar_policy/struct.Diagnostics.html) — `reason()`/`errors()` API shape
- [OPA #2897: Option to include explanation (trace output) in decision logs (GitHub)](https://github.com/open-policy-agent/opa/issues/2897) — rejection of always-on full tracing, rule-tracing compromise, PII-in-trace risk
- [Cedarling decision logs (Janssen docs)](https://docs.jans.io/v1.3.0/cedarling/cedarling-logs/) — production decision-log schema with configurable claim inclusion
- [Returning Tool Errors to an LLM for Graceful Recovery (AI/TLDR)](https://ai-tldr.dev/learn/llm-apis/function-calling/handling-tool-errors/) — validation/transient/permanent error taxonomy and phrasing
- [How to Handle Tool Errors in an AI Agent (dreaming.press)](https://dreaming.press/posts/how-to-handle-tool-errors-in-an-ai-agent.html) — return-not-raise default, execution-vs-infrastructure failure split
- [Agent Error Handling Documentation Specification (Geodocs.dev)](https://geodocs.dev/ai-agents/agent-error-handling-documentation-spec) — structured error field spec (code/field/allowed_values/hint/retryable/severity)
- [Error Messages Are the New Prompts (Karigor AI Labs)](https://karigor.ai/blog/error-messages-are-the-new-prompts) — error-as-directive pattern with NEXT_ACTIONS

---
*Feature research for: engram v0.12.x — Headless Reach & Diagnosability*
*Researched: 2026-07-29*
