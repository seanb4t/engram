// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package curationeval measures the advisory relation verdicts
// internal/verdict produces (spine-review consolidate's verdict pass)
// against labeled pairs of short project-memory notes (CUR-03). Two
// corpora share the same harness (D-01):
//
//   - The committed corpus, syntheticPairs (pairs.go): synthetic,
//     blind-checked (D-02) note pairs across all five verdict.Relations()
//     classes — see pairs.go's file comment for the full
//     authoring/blind-labeling procedure. This is the only corpus D-03's
//     hard gate applies to.
//   - A private local corpus, loaded by loadLocalPairs (localfile.go) from
//     a JSON Lines file named by ENGRAM_CURATION_EVAL_PAIRS — for
//     measuring against real, non-public spine content without ever
//     committing it. Its recommended location,
//     internal/curationeval/testdata/local/, is excluded by .gitignore.
//
// Both corpora flow through the SAME pipeline: evaluate (evaluate.go)
// sends each pair through the shared verdict.NewRequest/verdict.FromResult
// contract — the exact question set spine-review consolidate ships,
// including the updates relation (D-06) — via one decide.Decider.DecideMany
// call. metrics.go turns the resulting []prediction into accuracy by
// confidence bucket (bucketOf/accuracyByBucket, D-03's low/mid/high
// buckets), a multi-class Brier score (brier), a five-by-five confusion
// matrix (confusion), and formatReport's aggregate-only report — every
// line prefixed "CURATION-EVAL | ", carrying predictions only, never pair
// text, so downstream records (plan 03-08) can never leak spine content.
//
// thresholdGate is the package's one hard gate (D-03): among predictions
// whose chosen relation's probability is at or above the resolved
// threshold, accuracy must be at least 0.9 (integer arithmetic,
// correct*10 >= 9*n); zero such predictions is VACUOUS, never a pass. The
// gate applies to the committed corpus only — the local corpus (when
// configured) reports aggregates with no gate, since its content and size
// are the operator's own and not a fixed acceptance bar.
//
// The whole live measurement is gated behind ENGRAM_CURATION_EVAL,
// resolved by resolveEvalGate's package-local koanf load (gate.go) —
// production's ENGRAM_ prefix and precedence, deliberately NOT registered
// in internal/config (Phase 1 D-15). TestCurationEval (eval_test.go) is
// the gated live test `task eval:curation` runs; off by default, it never
// makes a provider call and go test ./... never depends on one.
package curationeval
