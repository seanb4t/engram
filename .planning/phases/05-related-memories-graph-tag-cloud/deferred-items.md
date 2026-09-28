# Deferred Items — Phase 05

## Deferred Items

- `requirements mark-complete` cannot flip GRAPH-01, GRAPH-02, GRAPH-03, TAGS-01, TAGS-02 (or,
  apparently, any requirement in this project) because `.planning/REQUIREMENTS.md`'s traceability
  table uses `Mapped` as its Status value, and `gsd-tools`' `cmdRequirementsMarkComplete` only
  flips a row whose Status reads `Pending` or `Gaps Found` (`/^(pending|gaps found)$/i`) — its
  checkbox-rollback safeguard (an ID whose row exists but was not flipped is treated as a rejected
  write) then reverts the `- [ ]` checkbox too, so neither surface ever reaches `Complete`.
  `git log -S "Mapped" -- .planning/REQUIREMENTS.md` shows this status value present since the
  project's original GSD bootstrap commit (`d2120f09`), and `rg -c '^\- \[x\]'
  .planning/REQUIREMENTS.md` finds zero completed checkboxes anywhere in the file — this is a
  pre-existing, project-wide convention/tool vocabulary mismatch, not something introduced by this
  phase or plan. `05-09-PLAN.md`'s own `requirements.ready-ids` confirmed all 5 IDs as
  `5/5 requirement(s) ready to mark complete` (the shared-ID gate cleared correctly), so the block
  is purely in `mark-complete`'s Status-value acceptance, not the readiness computation.
  **What was verified instead:** GRAPH-01/02/03 and TAGS-01/02 are proven shipped by this phase's
  live evidence — `TestConsoleRelatedView` (chromedp, real browser + Qdrant), the WCAG 2.2 AA
  audits in `surfaces.browser.test.ts`, and the full `05-01`..`05-09` plan chain's own test suites.
  status: open
  **Recommendation:** either (a) accept `Mapped` as a synonym for `Pending` in
  `cmdRequirementsMarkComplete`'s acceptance regex upstream in `gsd-tools`, or (b) migrate this
  project's `REQUIREMENTS.md` traceability Status column to the tool's own `Pending`/`Complete`/
  `Gaps Found`/`Blocked` vocabulary in a dedicated docs-only change — not silently, and not as part
  of a feature phase's own commit.
