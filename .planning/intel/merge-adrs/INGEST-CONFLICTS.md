## Conflict Detection Report

Ingest set: 31 companion ADRs (all LOCKED, precedence 0, high confidence).
Checked against the 25 already-LOCKED baseline decisions in `.planning/PROJECT.md`
and existing `.planning/intel/` + `.planning/{REQUIREMENTS,ROADMAP}.md`.

Cycle detection: no cycles (in-set edges 8q3→u9v, m4s8→ddiw are terminal).
No UNKNOWN/low-confidence docs. No LOCKED-vs-LOCKED contradictions. No competing
acceptance variants (these are ADRs, not PRDs).

### BLOCKERS (0)

None. No ingest decision sets a mutually-exclusive knob to a value incompatible
with an existing locked decision on the same axis.

### WARNINGS (0)

None.

### INFO (11)

[INFO] 50b consistent with bundled skill/engram plugin baseline
  Found: docs/adr/engram-50b-... — engram plugin ships NO bundled MCP server; remove
    bundled .mcp.json; /engram-setup is the sole MCP-server registration path.
  Note: The Phase-7 baseline relocated the memory-curator client plugin into the repo as
    the bundled skill/engram plugin (packaging axis). 50b governs a different axis —
    whether that plugin auto-registers its MCP server via a shipped .mcp.json or via an
    explicit /engram-setup `claude mcp add`. A bundled plugin registering through
    /engram-setup rather than a .mcp.json is consistent. Not a contradiction.

[INFO] 8q3 + u9v describe staged session-cookie custody, not a conflict
  Found: docs/adr/engram-8q3-... seals only {sub, expiry} (v1 read lane);
    docs/adr/engram-u9v-... seals {access, refresh, sub}.
  Note: Same scope noun (session cookie) but different lifecycle stages — 8q3 is the
    current read-only v1 lane; u9v is the eventual write-phase custody model. Both refine
    baseline DEC-g37x. Complementary, staged; no incompatible value on one axis.

[INFO] Auto-resolved: rules refinements complement baseline rule locks
  Found: docs/adr/engram-d386-... (session-start rules progressive-disclosure index),
    docs/adr/engram-m4s8-... (reject malformed rule summaries).
  Note: Narrower-grain elaboration of baseline DEC-iedk / DEC-ambu. No override; the
    baseline locks remain authoritative and are refined, not contradicted.

[INFO] Auto-resolved: temporal-recall refinements complement DEC-y1g
  Found: docs/adr/engram-ufz-... (soft-hide expired at recall, opt-in prune-expired),
    docs/adr/engram-c0m-... (inject Store clock via WithClock).
  Note: Implementation-grain refinement of the baseline temporal gate DEC-y1g.

[INFO] Auto-resolved: telemetry refinements complement DEC-dwi / DEC-uxh
  Found: docs/adr/engram-6gb-..., engram-f7p-..., engram-tdk-..., engram-wot-...,
    engram-7qd-..., engram-9tj-... (span seams, PII minimization, OTel env vars, k8s attrs).
  Note: Fine-grained instrumentation decisions under the baseline observability locks.
    engram.owner-only span attrs (wot) reinforce, not weaken, existing PII posture.

[INFO] Auto-resolved: BFF/session refinements complement DEC-g37x
  Found: docs/adr/engram-bgj-... (BFF in Go binary), engram-u9v-..., engram-8q3-....
  Note: Refine how the operator-console session/BFF is built under the baseline auth lock.

[INFO] Auto-resolved: SPA-internal refinements complement DEC-0lu / DEC-8xe
  Found: docs/adr/engram-2xl-..., engram-3nas-..., engram-c4y-..., engram-lzz-...,
    engram-no3-..., engram-vxk-..., engram-4ag-....
  Note: Data layer, markdown/XSS sanitization, URL-state, theming, wordmark, static
    fallback, dashboard gating — all narrower-grain SPA elaboration of baseline console locks.

[INFO] Auto-resolved: docs-site refinements complement DEC-ttb
  Found: docs/adr/engram-u5h-... (docs-site in monorepo), engram-1w7-... (wrangler deploy).
  Note: Placement + deploy pipeline detail under the baseline docs-site lock.

[INFO] Auto-resolved: test-tier refinements are internally consistent
  Found: docs/adr/engram-1h3k-... (two-tier vitest), engram-om5b-... (node tier on
    environment:node, drop happy-dom).
  Note: om5b refines 1h3k (in-set). No baseline lock contradicted.

[INFO] Auto-resolved: config-validation refinements complement config baseline
  Found: docs/adr/engram-wtw-... (config.Load assembly-only), engram-d24-... (validate
    data-plane fields only; listen_addr serve-local guard).
  Note: d24 refines wtw (in-set); both refine the baseline config pipeline decision.

[INFO] Dangling cross-refs recorded (not blocking)
  Found: cross_refs to out-of-set ids (engram-s2ao, engram-1xv, engram-e38, engram-ew7,
    engram-edv, engram-mbnw), to baseline locks (engram-0lu, engram-kyz, engram-uxh,
    engram-ambu), and to non-id strings (PR #16, v0.3.0, 2026-06-02-generalize-engram-
    client-config, "two-tier vitest decision").
  Note: Targets are outside the ingest set; treated as dangling references for
    traceability only. Not cycles, not conflicts. See context.md.
