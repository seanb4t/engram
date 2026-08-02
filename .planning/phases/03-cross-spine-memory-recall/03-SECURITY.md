---
phase: 3
slug: cross-spine-memory-recall
status: complete
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-02
---

# v0.12.x Phase 3 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Retroactive audit — the phase shipped before `/gsd-secure-phase` was run for this milestone.

This phase widens recall to span every scope a caller can read. Its central threat (T-03-01) is
that a cross-spine-shaped filter returns another owner's private record.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Caller → recall API | `cross_spine` arrives as an explicit boolean on MCP and Connect | Scope selection + the cross-spine flag |
| Handler → store filter | `effectiveSearchScope` decides the scope; the store composes the Qdrant filter | Subject identity → authz predicate |
| Store → Qdrant | The composed filter is the **only** thing standing between a query and every stored record | Owner/visibility conditions |
| Response → caller | `searched_scopes` / `scopes_truncated` report query coverage | Scope **names** the caller is authorized to read |

---

## Threat Register

Authored at plan time across `03-01`–`03-05-PLAN.md` — 19 disposition rows resolving to **8 unique
threat IDs** (`T-03-01`, `-02`, `-04`, `-SC` recur per plan as the same threat is re-verified
against each newly-touched code path; collapsed below with evidence from every plan that touched
them). Verified retroactively by `gsd-security-auditor` on 2026-08-02.

| Threat ID | Category | Severity | Disposition | Mitigation / Evidence | Status |
|-----------|----------|----------|-------------|------------------------|--------|
| T-03-01 | Elevation of Privilege | **critical** | mitigate | The scope clause is conditional (`if scope != ""`) while `s.ownerOrSharedCondition(ctx, subj)` is appended **unconditionally**, as a separate `Must` element, immediately after — `ownerScopeFilter` (`store.go:792-799`) and `listFilter` (`:1097-1102`). `Search` (`:930-942`) and `List` (`:1157-1165`) only ever `append` to `f.Must` afterward, never reassign it, so no later code path can drop the authz element. See the RED-by-mutation proof below | closed |
| T-03-02 | Elevation of Privilege | high | mitigate | `effectiveSearchScope` (`tools.go:1374-1382`) is the sole chokepoint, reached from every entry: both MCP closures (`:1824`, `:1871`), `deps.searchMemory` (`:1332`), `deps.listMemory` (`:1257`), and both Connect handlers (`connectapi.go:196`, `:266`). The one other path into `Store.List` — `listRules` — rejects an empty scope earlier via `validRuleScope`'s mandatory `rule:repo:`/`rule:project:` prefix (`rules.go:193-202`). Tests `TestEffectiveSearchScope`, `TestListRulesRejectsEmptyScope`, `TestConnectCrossSpineScopeRequired` PASS | closed |
| T-03-03 | Elevation of Privilege | high | mitigate | Connect reads `req.Msg.CrossSpine` **explicitly** (`connectapi.go:214,272`); it never infers cross-spine from an empty scope. The pre-existing `Scope==""` inference at `:345` is confined to `SearchDiscoveries` and is documented at `:322-331` as non-transferable legacy behavior. `TestConnectCrossSpineNotInferred` PASS | closed |
| T-03-04 | Information Disclosure | low | accept (store) / mitigate (docs) | See AR-03-01 | closed |
| T-03-05 | Information Disclosure | medium | mitigate | The cross-spine-with-ignored-scope log line is a fixed string (`"search_memory: cross_spine=true; ignoring supplied scope"`) that never interpolates the caller's `a.Scope` (`tools.go:1827-1831`, `:1874-1878`) — so a discarded scope name cannot leak into server logs | closed |
| T-03-06 | Tampering | high | mitigate | The isolation test guards against its own vacuity: an explicit truncation check (`store_test.go:4610-4617`) turns a scroll-page-full condition into a loud `t.Fatalf` rather than a silent pass. Falsifiability demonstrated twice — `03-RED-TRANSCRIPT.md` records the original store-level PASS→FAIL→PASS, and this audit independently reproduced it at the handler level with a different test | closed |
| T-03-07 | Denial of Service | medium | accept (store) / mitigate (docs) | See AR-03-02 | closed |
| T-03-SC | Tampering (supply chain) | high | accept | See AR-03-03 | closed |

*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-03-01 | T-03-04 | `searched_scopes` returns scope **names**, which is a small disclosure by construction. Bounded because `Store.ListScopes` (`store.go:1440-1445`) filters on exactly the same `ownerOrSharedCondition` as the search itself — a caller learns only the names of scopes it was already authorized to read — and `searchedScopes` (`tools.go:1416-1429`) extracts names only, dropping counts, so it is not a hit-distribution side channel. Docs state the meaning verbatim on both surfaces (`SKILL.md:277-278`, `tools.md:147-148`) | Phase 3 plan, verified 2026-08-02 | 2026-08-01 |
| AR-03-02 | T-03-07 | A cross-spine list can read widely. Accepted because `maxListLimit = 1000` (`store.go:1219`) caps cursor-mode pages, is unchanged by this phase, and is scope-agnostic — the same unbounded read is already reachable today by naming one large scope, so cross-spine adds no new capability. Mitigated in docs: the "When not to use cross-spine" subsection (`SKILL.md:280-294`) and the explicit-`limit` guidance (`tools.md:152-156`) | Phase 3 plan | 2026-08-01 |
| AR-03-03 | T-03-SC | No dependency added: `git diff 737178e2^..06a5b63c -- go.mod go.sum` is empty across the full phase range. `testcontainers-go/modules/qdrant` was already a test dependency | Phase 3 plan | 2026-08-01 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-02 | 8 unique (19 rows) | 8 | 0 | gsd-security-auditor (retroactive, ASVS L1 with L2/L3 depth on T-03-01, block_on high) |

**Verdict: SECURED.** Register origin: `register_authored_at_plan_time: true`.

**T-03-01 was proven, not inspected.** Because a defect here silently returns other owners' private
records, the audit verified it three independent ways rather than at the configured L1 depth:

1. **Source read** — the conditional scope clause and unconditional authz clause, plus confirmation
   that every downstream path only appends to `f.Must` and never reassigns it.
2. **Live tests against real Qdrant** — `TestCrossSpineAuthzIsolation` and `TestSearchCrossSpine`
   both `--- PASS`.
3. **Independent RED-by-mutation performed during this audit** — the auditor stripped the
   `ownerOrSharedCondition` append from `ownerScopeFilter` and re-ran
   `TestSearchMemoryCrossSpineIsolation`, observing
   `--- FAIL: … leaked owner B's private record: c5c50001-…003`; it then reverted and re-ran to
   `--- PASS`. The revert was confirmed byte-identical (`git diff --exit-code` clean) by the auditor
   and re-confirmed independently by the orchestrator before this file was written.

That third check is what distinguishes "the test passes" from "the test would notice." It used a
*different* test from the one `03-RED-TRANSCRIPT.md` records, so two independent tests are now
demonstrated falsifiable against the same invariant.

**Process gap noted (non-blocking).** Only `03-05-SUMMARY.md` carries a `## Threat Flags` section
(stating "None"); `03-01` through `03-04` omit the section entirely rather than asserting none. The
auditor found no unmapped attack surface in the diffs, in `03-REVIEW.md` (independent review, 0
critical/warning findings), or in its own reading of the store and server sources — so this reads as
a SUMMARY-template gap, not a missed threat. Worth closing in the template so "no flags" is
affirmative rather than inferred from silence.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
