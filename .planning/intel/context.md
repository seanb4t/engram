# Context Notes

Running background/context notes with source attribution. No standalone DOC-type docs were
present in this ingest set; these notes capture historical supersessions, the expected
ADR↔SPEC design-doc pairing map, and dangling cross-references to out-of-set decisions.

---

## Historical / superseded

### CTX-client-config-generalize — Client config generalization (HISTORICAL)
- source: docs/superpowers/specs/2026-06-02-generalize-engram-client-config-design.md
- note: Design to make the bundled engram client MCP config deployment-neutral and add an
  `/engram-setup` command covering four auth postures (OIDC OAuth, bearer/headers, litellm
  gateway). This SPEC was later **superseded by ADR engram-50b, which is NOT in this ingest
  set**. Treat its client-config-generalization content as historical context only — it is
  not a competing locked decision and must not be routed as an active requirement.

### CTX-configurable-claim-supersession — SPEC supersedes out-of-set ADR engram-hvg
- source: docs/superpowers/specs/2026-06-29-configurable-claim-owner-design.md
- note: This SPEC states it supersedes ADR engram-hvg (not in set). The in-set locked ADR
  DEC-g37x (`engram-g37x`) encodes the same configurable-claim owner decision, so SPEC and
  in-set ADR agree. engram-hvg content is historical.

### CTX-vitest-supersession — SPEC supersedes out-of-set decision engram-cv92
- source: docs/superpowers/specs/2026-06-27-vitest-browser-mode-ui-test-unification-design.md
- note: SPEC (Status DRAFT) references and supersedes decision engram-cv92 (not in set).
  Historical; the active direction is real-Chromium vitest browser mode (see
  CON-ui-real-dom-tests).

---

## Expected ADR ↔ SPEC design-doc pairings (NOT conflicts)

Many SPECs in this set are the design docs behind a locked ADR. This overlap is expected: the
ADR wins on the locked decision (precedence 0), the SPEC supplies the WHAT / context /
requirements. Recorded here for traceability; each is an INFO auto-resolution in
INGEST-CONFLICTS.md, never a blocker.

- short_id: SPEC short-id-handle-design ↔ DEC-zzq0 (10-char Crockford) + DEC-02ta (resolve at handler)
- isolation/authz: SPEC per-actor-memory-isolation-design + typed-subject-authz-core-design
  ↔ DEC-cgb, DEC-kyz, DEC-xa6, DEC-y1g, DEC-g37x, DEC-12c
- configurable owner: SPEC configurable-claim-owner-design ↔ DEC-g37x
- discovery: SPEC discovery-memory-type-design ↔ DEC-2bv
- scheduled: SPEC scheduled-memories-design ↔ DEC-90w, DEC-y1g
- windowed/cursor recall: SPEC windowed-cursor-recall-design ↔ DEC-1frj, DEC-ef28
- summary recall: SPEC auto-summary-curated-memories-design + memory-display-summary-ux-design ↔ DEC-ambu
- rule kind: SPEC rule-memory-kind-design ↔ DEC-iedk
- embedder params: SPEC asymmetric-cloud-embedder-params-design ↔ DEC-zyhq, DEC-378
- config: SPEC engram-config-prefix-koanf-design + config-validation-design ↔ DEC-jgq, DEC-irq, DEC-378
- telemetry: SPEC observability-logging-telemetry-design + telemetry-at-every-seam-design ↔ DEC-dwi, DEC-uxh
- web UI: SPEC engram-web-ui-design + operator-console-spa-design + operator-console-redesign-design
  ↔ DEC-8xe, DEC-0lu, DEC-bj6
- docs site: SPEC docs-site-astro-starlight-cloudflare-design + docs-site-landing-redesign-design ↔ DEC-ttb

Standalone SPECs (no directly-paired locked ADR in set): config-validation-design,
engram-brand-identity-design, vitest-browser-mode-ui-test-unification-design,
relocate-memory-curator-into-engram-design, connect-auth-posture-addendum.

---

## Dangling cross-references (out-of-set decisions)

SPEC cross_refs pointing to ADRs/decisions not present in this ingest set. Recorded for
traceability; none block synthesis.

- ADR engram-hvg — referenced by typed-subject-authz-core-design and configurable-claim-owner-design
- ADR engram-lkm — referenced by windowed-cursor-recall-design (in-set DEC-1frj also cross-refs it)
- ADR engram-3hp9 — referenced by DEC-1frj (boundary-cursor)
- decision engram-cv92 — superseded by vitest-browser-mode SPEC
- ADR engram-50b — superseded the client-config-generalize SPEC (see CTX-client-config-generalize)

---

## Cross-ref graph (in-set edges, acyclic)

- docs-site-astro-starlight → engram-web-ui-design
- telemetry-at-every-seam → observability-logging-telemetry
- operator-console-spa → engram-web-ui-design
- memory-display-summary-ux → auto-summary-curated-memories
- connect-auth-posture-addendum → engram-web-ui-design
- DEC-iedk (rules) → DEC-kyz (sharing)
- typed-subject-authz-core → DEC-kyz

Cycle detection (3-color DFS, depth cap 50): no cycles.
