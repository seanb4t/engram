# Phase 05 Source and Edge Audit

Planning-only mapping; no implementation or verification result is asserted.

| Source | ID | Required outcome / constraint | Plans | Status |
|---|---|---|---|---|
| GOAL | Phase 5 | Two entry points with generated registration equivalence | 01–03 | COVERED |
| REQ | REQ-engram-setup-delegates | PATH detection and CLI delegation including client-ID input | 01, 02 | COVERED |
| REQ | REQ-engram-setup-prose-fallback | First-class four-choice Claude fallback | 02 | COVERED |
| REQ | REQ-delegation-equivalence-derived | Real Plan derivation and failing drift gates | 02, 03 | COVERED |
| CONTEXT | D-01 | Plain PATH presence | 02 task 1 | COVERED |
| CONTEXT | D-02 | Preview, rows, confirmation, apply | 02 task 1 | COVERED |
| CONTEXT | D-03 | All detected runtimes, no runtime selector | 02 tasks 1–2 | COVERED |
| CONTEXT | D-04 | Four auth choices; credential-safe mapping as amended by D-18 | 01, 02 | COVERED |
| CONTEXT | D-05 | Claude registration fallback preserved as amended | 02 task 1 | COVERED |
| CONTEXT | D-06 | Fallback registration scope, no skills instructions | 02 task 1 | COVERED |
| CONTEXT | D-07 | Single nonblocking brew pointer | 02 task 1 | COVERED |
| CONTEXT | D-08 | Claude-only fallback | 02 task 1 | COVERED |
| CONTEXT | D-09 | Source is actual ClaudeCode.Plan Args | 02 task 1 | COVERED |
| CONTEXT | D-10 | Generate tables/invocations, author surrounding prose | 02 task 1 | COVERED |
| CONTEXT | D-11 | Existing anchor syntax and writer | 02 task 1 | COVERED |
| CONTEXT | D-12 | Single surfaces:gen path | 01 task 3; 02 task 1; 03 task 1 | COVERED |
| CONTEXT | D-13 | CI regenerate/diff | 03 tasks 1–2 | COVERED |
| CONTEXT | D-14 | Local failing lint gate | 03 task 1 | COVERED |
| CONTEXT | D-15 | Injected Plan argv mutation | 03 task 2 | COVERED |
| CONTEXT | D-16 | Command stays plugin-only | 02 task 1 | COVERED |
| CONTEXT | D-17 | Client-ID CLI, validation, help, goldens, all four modes | 01 tasks 1–3; 02 | COVERED |
| CONTEXT | D-18 | ENGRAM_TOKEN / MCP_CLIENT_SECRET prerequisites, no secret argv | 01 task 3; 02 task 1 | COVERED |
| CONTEXT | D-19 | Argv and input equivalence, actual Cobra flag conformance | 02 tasks 1–2 | COVERED |
| CONTEXT | D-20 | Read-only local check, existing CI lane, mutant controls | 03 tasks 1–2 | COVERED |
| RESEARCH | Responsibility map | CLI owns validation; runtime owns argv; generator consumes it | 01, 02 | COVERED |
| RESEARCH | Pure generation | Synthetic environment, structural unique action selection, Action.Command | 02 task 1 | COVERED |
| RESEARCH | Auth conflicts | Context D-17–D-19 supersede original deferral and credential text | 01, 02 | COVERED |
| RESEARCH | Drift/anchor safety | Exact bytes, no committed duplicate golden, outside-region preservation, error propagation | 02 task 1; 03 | COVERED |
| RESEARCH | Runtime safety | Fake environments; no live registration or user-home writes | All plans | COVERED |
| RESEARCH | Single generation / lint conflict | Check-only dispatch before all writers | 03 task 1 | COVERED |
| RESEARCH | Security | Input validation, quoting, confirmation, report as data, source-sensitive gates | Threat models in all plans | COVERED |

## Spec-less edge resolution

The probe returned three `unclassified/unresolved` items, each with only a requirement ID.
There is no hidden edge predicate to invent. These explicit planning assumptions resolve the
unspecified edge boundary into the accepted context and observable checks; none is a backstop.

| Probe requirement | Explicit resolution | Verification / truth |
|---|---|---|
| REQ-engram-setup-delegates | Binary presence alone selects delegation; absent binary selects fallback. Invalid required client-ID fails before effects; unsupported rows remain visible and nonzero apply is never auto-retried. | 01 input tests; 02 generated invocation tests and authored routing source review; 01/02 must_haves truths |
| REQ-engram-setup-prose-fallback | Four Claude add choices remain available without the binary, with env prerequisites; no non-Claude prose registration or skills installation is added. | 02 exact real-Plan row comparisons, anchor preservation, and prose source review; 02 must_haves truths |
| REQ-delegation-equivalence-derived | Equivalence is registration argv plus auth inputs, not remove/add sequence, credential acquisition, skills distribution, or third-party execution. | 02 live Cobra conformance; 03 mutation plus two byte-comparison lanes; 02/03 must_haves truths |

No SPEC prohibitions were supplied by the orchestrator; no new generic prohibition descriptors
or backstops are invented. Accepted context exclusions are stated in task scope. Deferred ideas
(version floor, full-section generation, additional prose runtimes/skills, command embedding)
are excluded. Schema gate: no ORM schema path or database mutation. Package gate: no installation.

## Dependencies and ownership

Wave 1 supplies ClientID; wave 2 consumes it and produces setupgen plus published prose;
wave 3 extends that same generator with checks. Shared files force sequential waves. Every
task lists exact modified files, with at most five files per task; no same-wave file overlap.
Estimate calibration returned factor 1, sample_count 0, confidence low.
