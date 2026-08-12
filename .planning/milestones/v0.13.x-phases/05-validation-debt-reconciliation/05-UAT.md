---
status: complete
phase: 05-validation-debt-reconciliation
source: [05-01-SUMMARY.md, 05-02-SUMMARY.md]
started: 2026-08-12
updated: 2026-08-12
human_verified: false
completion_note: "No user-facing surface; all checks agent-executed. See '## Note on this session'."
---

## Current Test

[testing complete]

## Note on this session

This phase changed no runtime behaviour. Its entire output is planning records, one Go comment
and one docs table cell — every deliverable is verifiable by reading a file or running a command,
and there is no user-facing surface a human could exercise that an agent cannot.

Four checkpoints were initially drafted for human verification. The user correctly pushed back
("why are you asking me to verify a file you have?") — all four were things the orchestrator had
already executed during `/gsd-validate-phase 5`. They were re-run directly instead and are
recorded below as `agent-verified` (mechanical assertions with their output) or `agent-judgment`
(the one reading call), never as user-confirmed. No test in this file was passed by a human.

The honest conclusion is that this phase had no genuine UAT surface, and generating a checklist
for it was the same bookkeeping-about-bookkeeping the phase was descoped to avoid.

## Tests

### 1. The repointed 03.1 command names tests that actually exist
expected: The REQ-merge-idempotency store-side command names `TestSupersedeConcurrentKeyedDisjointTargetsCannotBothLand` and `TestPayloadRoundTripsIdempotencyFingerprint`; `TestSupersedeIdempotency` appears nowhere in the file; running the command passes.
result: pass
source: agent-verified
evidence: "Markers confirmed in 03.1-VALIDATION.md (both replacement names present, `TestSupersedeIdempotency` absent). `go test ./internal/store/... -run 'TestSupersedeConcurrentKeyedDisjointTargetsCannotBothLand|TestPayloadRoundTripsIdempotencyFingerprint' -v` produced 2 `--- PASS` lines — counted, not inferred from exit status, since exit 0 is exactly what the old fictional pattern returned."

### 2. The three reconciled records show their evidence, not just a verdict
expected: Open `01-VALIDATION.md`, `02-VALIDATION.md` and `03.1-VALIDATION.md`. Each reads `status: validated` / `nyquist_compliant: true`, its verification-map column is headed `Tests Matched` with a real number in every row (or `n/a` for rows that carry no `-run` pattern), and each ends with a dated `## Validation Audit 2026-08-11` section stating that elements were re-resolved against `go test -list` at HEAD rather than checked for exit 0.
result: pass
source: agent-verified
evidence: "All three files: `status: validated` = 1, `nyquist_compliant: true` = 1, `Tests Matched` present, `## Validation Audit 2026-08-11` present. Independently re-derived during /gsd-validate-phase: all 24 top-level pattern elements resolve against a fresh `go test -list '.*' ./...` (1,047 names), zero unresolved, counts matching what the records claim."

### 3. Phase 04's unproven requirement is still visibly unproven
expected: Open `04-VALIDATION.md`. Frontmatter reads `status: validated` with `nyquist_compliant: false` — the documented PARTIAL state. The `REQ-consent-adversarial-proof` row still reads `⬜ pending`, its explanatory paragraph is intact, and the Sign-Off Approval still reads `pending`. Nothing about that requirement was flipped green.
result: pass
source: agent-verified
evidence: "`status: validated` + `nyquist_compliant: false` (the documented PARTIAL state), `REQ-consent-adversarial-proof.*⬜ pending` = 1, Sign-Off `**Approval:** pending` = 1. Nothing about that requirement was flipped green."

### 4. The audit notes are honest enough to be useful to a future reader
expected: Read one `## Validation Audit 2026-08-11` section cold, as someone who was not here. It should tell you what was resolved, how it was established (fresh `go test -list` at HEAD, explicitly not exit status), what was found to have drifted and what replaced it, and what was deliberately left alone. A note that only says "reconciled" would fail this. Judgement call — a regex can confirm the heading exists but cannot tell you whether the note earns its place.
result: pass
source: agent-judgment
evidence: "Performed during /gsd-validate-phase 5 as Manual-Only verification 1, and recorded in 05-VALIDATION.md's `## Validation Audit 2026-08-12`. The notes in 01, 02 and 03.1 each name what was resolved, how it was established (fresh `go test -list` at HEAD, explicitly not exit status), what was found to have drifted and was repointed, and what was deliberately left alone (`TBD` Task ID cells, and each file's own cautionary `go test -run X ./pkg/...` prose example). They clear the honest-read bar."
caveat: "This is the one item in this file resting on a reading judgement rather than a command. It is recorded as the orchestrator's judgement, not as user sign-off. A human who wants to overrule it should read one `## Validation Audit 2026-08-11` section cold and say so — nothing downstream depends on this verdict, since the phase's requirements each carry automated coverage independently."

### 5. #355's eval citations name symbols, not line numbers
expected: `internal/retrievaleval/retrieval_eval_test.go` cites `deps.searchMemory` / `server.Register` and `store.EmbedText` by symbol, carries no `tools.go:<number>` anchor, and `go vet ./internal/retrievaleval/...` plus `gofmt -l internal/retrievaleval/` are clean.
result: pass
source: automated
coverage_id: D1

### 6. The OpenRouter docs row points at a page that carries the row
expected: The OpenRouter row in `docs-site/src/content/docs/guides/embedding-instructions.md` links `[Embedding model recipes](/guides/embedding-models/)` and no longer defers to a row absent from its own page.
result: pass
source: automated
coverage_id: D2

### 7. REQUIREMENTS.md no longer asserts the disproven premise or the retired verify claim
expected: `REQ-nyquist-reconciled` and `REQ-citation-fixture-355` state what the live re-resolution actually found; no `six at 'status: draft'` premise and no word beginning `calibrat` remains.
result: pass
source: automated
coverage_id: D3

### 8. ROADMAP.md's Phase 5 entry is corrected without structural change
expected: Phase 5's checklist line, Goal, Depends-on and three success criteria are rewritten to the corrected claims; the `### Phase ` heading count is unchanged and `roadmap.validate` reports no warnings.
result: pass
source: automated
coverage_id: D4

## Summary

total: 8
passed: 8
issues: 0
pending: 0
skipped: 0
blocked: 0

Provenance of the 8 passes: 4 `automated` (deterministic coverage refs from 05-02-SUMMARY),
3 `agent-verified` (commands re-run with output recorded), 1 `agent-judgment` (a reading call).
0 human-verified.

## Gaps

[none yet]
