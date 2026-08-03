---
phase: 6
slug: rule-capture-investigation-fix
status: complete
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-02
---

# v0.12.x Phase 6 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Retroactive audit — the phase shipped before `/gsd-secure-phase` was run for this milestone.

**This phase shipped essentially no Go code.** Its deliverable is prose: corrected agent-facing
guidance in `curating-memory/SKILL.md` and docs. The security question is therefore not "were new
mitigations added" but **"did loosening the prose that encourages an agent to propose a rule also
loosen any code gate that decides who may create one?"** It did not.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Agent ↔ user (consent) | The agent proposes a rule; only the user may bless it. Enforced by protocol, not by code | A rule candidate and a yes/no |
| Agent → `store_rule` | Creates always-shared, MUST-follow ground truth loaded at every session start | Rule content |
| Agent → `delete_memory` on a rule | The server has **no** category guard here — prose is the only control (D-10) | A rule deletion |
| Agent → `set_visibility` / `supersede_memory` on a rule | Server-enforced: rules are immutable via both | Rejected by `errRuleImmutable` |

---

## Threat Register

Authored at plan time across `06-01`–`06-03-PLAN.md`, 16 rows. Verified retroactively by
`gsd-security-auditor` on 2026-08-02 at effective ASVS L2 — the phase's own premise is that prose
*is* the control, so the audit traced code boundaries rather than accepting pattern presence.

| Threat ID | Category | Severity | Disposition | Mitigation / Evidence | Status |
|-----------|----------|----------|-------------|------------------------|--------|
| T-06-01 | Elevation of Privilege | high | mitigate | `store_rule` appears exactly **once** inside `### Proposing a rule` (`SKILL.md:57-121`), and only inside step 4's *accept* branch — so the protocol cannot be read as authorizing a call before consent | closed |
| T-06-02 | Elevation of Privilege | high | mitigate | `SKILL.md:183-189` places "call `delete_memory` only after the user has explicitly blessed it" in the same sentence as the tool name, and states plainly that the server has no rule guard on delete | closed |
| T-06-03 | Repudiation (of a decline) | medium | mitigate | Protocol step 3 is "ask once, then stop" (`SKILL.md:89-92`); a decline is recorded (`:98-104`) and re-checked before any future proposal (`:106-110`) | closed |
| T-06-04 | Tampering (of interpretation) | medium | mitigate | The decline record must state all three parts — what was proposed, that *rule status* was declined and when, and that the underlying fact remains true — and must be filed `category: decision` not `gotcha` so the sweep's own enumeration cannot re-surface it (`SKILL.md:98-104`) | closed |
| T-06-05 | Denial of Service (of the mechanism) | medium | mitigate | The trigger is scoped in the same paragraph that states it — "not to conversation, to text you are quoting, or to a requirement you are restating" (`SKILL.md:74-77`) — so it cannot fire on every normative-sounding sentence and train the user to decline reflexively | closed |
| T-06-06 | Spoofing (of consent) | high | mitigate | No Go code changed in the phase range: `git diff --exit-code ad922f27 be21fdbd -- '*.go' internal/ cmd/` exits 0. Consent was real and per-candidate — `06-DEMONSTRATION.md` records outcomes with `short_id`s, independently re-confirmed against the **live store** on 2026-08-02 (`n6m4as49mr` blessed, `hxwad6qr58` recording two declines) | closed |
| T-06-07 | Elevation of Privilege | high | mitigate | The bless branch requires a *separate* question about deleting the source record (`SKILL.md:230-235`), and this was honored in practice: "Source gotcha records were left in place for all three — the bless was not conditioned on deleting `r3bjakymtz`" | closed |
| T-06-08 | Elevation of Privilege (batched consent) | high | mitigate | `SKILL.md:245-248` — "a sweep of twenty candidates is twenty proposals and twenty user answers, not one approval applied twenty times" | closed |
| T-06-09 | Tampering (of the rule set) | medium | mitigate | On a near-duplicate, the agent must "surface both `short_id`s and let the user resolve; never pick a winner yourself" (`SKILL.md:196-201`) | closed |
| T-06-10 | Information Disclosure (loss, not leak) | medium | accept | See AR-06-01 | closed |
| T-06-11 | Tampering (of the candidate set) | medium | mitigate | Sweep step 2 explicitly skips `rule-declined`-tagged records (`SKILL.md:220-223`), so a declined candidate cannot silently re-enter the pool | closed |
| T-06-12 | Elevation of Privilege (live) | high | mitigate | `06-03-PLAN.md` Task 1 is `type="checkpoint:human-verify" gate="blocking"`; the demonstration shows real per-candidate consent and no auto-performed source deletion | closed |
| T-06-13 | Coercion | medium | mitigate | The plan instructs "The user's decision is the demonstration… Do not steer toward yes," and the outcome corroborates it: **2 of 3 candidates were declined**, and the record frames declines as a healthy result rather than a shortfall | closed |
| T-06-14 | Destruction of ground truth | high | mitigate | No `delete_memory` call against any `rule:*` scope occurred during the sweep; the plan's prohibitions required reporting any such call, and none was reported | closed |
| T-06-15 | Tampering (of project history) | medium | mitigate | `git show be21fdbd -- .planning/ROADMAP.md .planning/REQUIREMENTS.md` — edits confined to checkbox flips, closure notes, traceability rows and the progress row; no structural churn | closed |
| T-06-SC | Tampering (supply chain) | high | mitigate | `git diff --exit-code ad922f27 be21fdbd -- go.mod go.sum` exits 0 across the full phase range | closed |

*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-06-01 | T-06-10 | Correcting a rule is delete-then-re-store, which leaves **no supersession trail** — unlike memories, where `supersede_memory` preserves history. Accepted because rules are deliberately immutable via `supersede_memory` (that immutability is itself a control, see the code gates below), so history-preservation and immutability are in genuine tension and immutability was chosen. Mitigated as far as prose can: the agent must quote the old text before deleting (`SKILL.md:191-194`) | Phase 6 plan (D-09/D-10) | 2026-08-01 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-02 | 16 | 16 | 0 | gsd-security-auditor (retroactive, ASVS L1 verified at L2 depth, block_on high) |

**Verdict: SECURED.** Register origin: `register_authored_at_plan_time: true`.

**Code gates reconfirmed independently of the prose** — this was the load-bearing check, since a
prose-only phase that silently widened a write gate would be the worst case:

- `errRuleImmutable` retains exactly **3** call sites in `internal/server/tools.go`, unchanged:
  `updateMemory`'s un-share guard (`:1521`), `setVisibility` (`:1664`), and `supersedeMemory`
  (`:1733`). Rules remain immutable through both mutation verbs.
- `deleteMemory` (`:1597-1612`) carries **no** category/rule guard — confirmed present-tense, and
  this matches the phase's own documented asymmetry (D-10): prose is the only control on rule
  deletion. Recorded here as a known, deliberate property rather than an oversight.
- No path was created that lets an agent call `store_rule` without consent; consent is enforced by
  protocol, and no code change made it automatic.
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestListRulesHandlerCurationAdvisory`
  → `--- PASS` (a real pass, not a silent Qdrant skip). `task lint:markdown` clean, 125 files.

**Scope-diff caveat worth preserving.** A naive `git diff ad922f27 -- '*.go' internal/ cmd/` against
current `HEAD` *does* show Go changes — but those belong to Phase 7, which landed on the same
unbranched commit stream after Phase 6 closed. Diffed against the correct phase-close commit
(`ad922f27..be21fdbd`), the Go and dependency diff is empty, confirming the phase's own
"no Go touched" prohibition. Anyone re-verifying this phase must scope the diff to the phase range,
or they will conclude a violation that did not occur.

No unregistered threats: no `## Threat Flags` section in any of the three SUMMARYs, consistent with
a phase whose only shipped artifact is prose plus one planning-doc closure.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
