# Phase 5: Apply-Time Preserve Gate & Documentation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-16
**Phase:** 5-Apply-Time Preserve Gate & Documentation
**Areas discussed:** already-correct in the apply lane, OAuth re-login warning, Preserved escape hatch, Docs + post-release closeout

---

## already-correct in the apply lane

### Q1 — With pre-write classification available in apply, what does `--apply` do on an `already-correct` registration?

| Option | Description | Selected |
|--------|-------------|----------|
| Compare first; skip the write on already-correct | Observe→Compare before any action; already-correct → zero actions; would-write → run → wrote; preserved → zero actions; byte-compare retired for drift-capable runtimes | ✓ |
| Write-then-compare stays; add only the preserved short-circuit | Minimal executor change; every re-run on Claude Code still logs the user out | |
| Compare first, write on would-write, then re-observe to confirm | Same as first plus a post-write re-Compare before reporting wrote | |

**User's choice:** Compare first; skip the write on already-correct.

### Q2 — After a `would-write` write succeeds, what populates the row's `Registered` field?

| Option | Description | Selected |
|--------|-------------|----------|
| Re-probe once, render via Observe | Second probe after the write, rebuilt through Observe→redact→render; outcome stays wrote | ✓ |
| No second probe; Registered shows the pre-write observation | Stale state shown right after changing it | |
| No second probe; Registered empty on wrote | Drops the present-state affordance on the one outcome where it changed | |

**User's choice:** Re-probe once, render via Observe.

**Continue prompt:** "Next area".

---

## OAuth re-login warning

### Q1 — When is a Claude Code registration treated as 'OAuth-authenticated' for the re-login warning?

| Option | Description | Selected |
|--------|-------------|----------|
| By shape: observed auth mode is oauth/oauth-client | No Authorization/bearer header observed + would-write on Claude Code → warn; conservative | ✓ |
| Only when login state is observable | No read verb exposes it today; SC3 would ship dead | |
| Always, on any Claude Code remove-then-add | Bearer users told about an OAuth login they never had | |

**User's choice:** By shape.

### Q2 — Where does the re-login warning render, and does `--apply` pause on it?

| Option | Description | Selected |
|--------|-------------|----------|
| Row note in both lanes; apply proceeds without pausing | Typed consequence on the Notes field in preview and apply; --apply is the only consent gate | ✓ |
| Row note plus a stderr line emitted before the remove runs | Second rendering path outside the row model | |
| Apply pauses for confirmation | Contradicts the locked consent-gate decision; breaks scripted use | |

**User's choice:** Row note in both lanes; apply proceeds without pausing.

**Continue prompt:** "Next area".

---

## Preserved escape hatch

### Q1 — How does an operator move past a `preserved` registration they actually want replaced?

| Option | Description | Selected |
|--------|-------------|----------|
| No flag; the row tells them the manual step | Reason names the facet and the exact manual command; engram never destroys what it can't reproduce | ✓ |
| Add --replace-registration | New consent surface the milestone ruled out; re-opens the incident class in scripts | |
| Defer the decision to the backlog | Manual hint now, flag designed later | |

**User's choice:** No flag; the row tells them the manual step.

**Continue prompt:** "Next area".

---

## Docs + post-release closeout

### Q1 — How is the post-release live observation handled for REQ-docs-setup-v2 in this phase?

| Option | Description | Selected |
|--------|-------------|----------|
| D-10 precedent: pass with a POST-RELEASE.md handoff | Docs truthful pre-merge; 05-POST-RELEASE.md; verification passes with post_release_status: pending; REQ unchecked until 05-RELEASE-<ver>.md | ✓ |
| Block Phase 5 verification on the observation | Circular wait on a release cut from an unmerged branch | |
| Check it off from code alone | Contradicts the requirement text | |

**User's choice:** D-10 precedent.

### Q2 — How is the content split across the three guides, and is it gated?

| Option | Description | Selected |
|--------|-------------|----------|
| By reader intent, each gated like agent-setup.md | install.md = binary + cask contents incl. man pages; agent-setup.md = running setup (plugin-first, headers, preserved, apply gate, re-login note, remediation); plugin.md = what the plugin is; a docs gate per guide | ✓ |
| Everything in agent-setup.md, others link to it | One page, one gate; install.md never mentions man pages | |
| By reader intent, no new docs gates | Two ungated guides can drift silently | |

**User's choice:** By reader intent, each gated.

**Continue prompt:** "Next — wrap up".

---

## Readiness check

**Prompt:** "We've discussed the apply lane, OAuth warning, escape hatch, and docs closeout. Which gray areas remain unclear?"
**User's choice:** I'm ready for context.

## Claude's Discretion

- Sharing the observe/compare/render sequence between the two `execute()` branches without duplication.
- Typed constants for the re-login note and per-runtime remediation hints (AUTHORED-HERE).
- Whether setupgen's generated `/engram-setup` prose gains preserved/apply-gate wording; `--help` wording.
- "Unreleased" notice wording; `05-POST-RELEASE.md` checklist details (reuse the 06 shape).
- Apply-lane fixture shape for SC1/SC2; red-evidence patches.

## Deferred Ideas

- `--replace-registration` — rejected for this milestone.
- stderr pre-write warning / interactive pause in `--apply` — rejected.
- Observing the `oauth-client` read-back shape — not needed under D-03; opportunistic in POST-RELEASE.
