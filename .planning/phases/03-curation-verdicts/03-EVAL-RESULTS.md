# Phase 3 Live Curation Eval Results (CUR-03, D-03)

## Provenance

- Date: 2026-09-24T05:05:29Z
- Git SHA: `ca28e8ca3cd1929ef344c134c2a36ba14628c8bb`
- Endpoint host: `openrouter.ai` (parsed from `ENGRAM_DECISIONS_BASE_URL`, host only — no userinfo, path or query)
- Model snapshot: `typesafe/jev-1.13-20260917`
- Threshold: `0.900`
- Committed corpus size: 70 pairs (`internal/curationeval.syntheticPairs`, D-01 — 16 duplicate, 12 contradicts, 10 updates, 16 related, 16 unrelated, blind-agreed per 03-04)
- Command: `task eval:curation` (invoked as `ENGRAM_DECISIONS_PROVIDER=jev ENGRAM_DECISIONS_BASE_URL=https://openrouter.ai/api task eval:curation`), one run, no local (`ENGRAM_CURATION_EVAL_PAIRS`) corpus configured in this environment

## Aggregate Report (verbatim, `go test -v` file:line prefix stripped)

```
CURATION-EVAL | corpus=committed pairs=70 scored=70 unavailable=0
CURATION-EVAL | unavailable_by_class
CURATION-EVAL | models=typesafe/jev-1.13-20260917 threshold=0.900
CURATION-EVAL | bucket=low n=10 correct=4 accuracy=0.400
CURATION-EVAL | bucket=mid n=20 correct=19 accuracy=0.950
CURATION-EVAL | bucket=high n=40 correct=40 accuracy=1.000
CURATION-EVAL | brier=0.138 uniform=0.800 n=70
CURATION-EVAL | gate threshold=0.900 n=40 correct=40 result=PASS
CURATION-EVAL | confusion gold=duplicate duplicate=16 contradicts=0 updates=0 related=0 unrelated=0
CURATION-EVAL | confusion gold=contradicts duplicate=0 contradicts=9 updates=3 related=0 unrelated=0
CURATION-EVAL | confusion gold=updates duplicate=0 contradicts=0 updates=10 related=0 unrelated=0
CURATION-EVAL | confusion gold=related duplicate=0 contradicts=0 updates=3 related=13 unrelated=0
CURATION-EVAL | confusion gold=unrelated duplicate=0 contradicts=0 updates=0 related=1 unrelated=15
CURATION-EVAL | needs_review=30
```

## Reading

The D-03 hard gate is met: at or above the 0.900 threshold, 40 of 40 scored verdicts matched gold (the high bucket independently reports the same 40/40, 1.000 accuracy). Measured against the spike's own reference numbers (p >= 0.9 accuracy 1.00 on 26 real pairs; 0.7-0.9 at 0.71; below 0.7 at 0.20), the high bucket lands exactly on the spike's figure, the mid bucket (0.950, n=20) comes in well above the spike's 0.71, and the low bucket (0.400, n=10) is roughly double the spike's 0.20 — a live 70-pair, five-class, blind-agreed corpus is not directly comparable to the spike's 26-pair set, but neither bucket regresses the earlier finding. `updates` fared perfectly in isolation — all 10 gold-`updates` pairs were predicted as `updates` — but is the corpus's main confusability sink from the other direction: 3 of 12 gold-`contradicts` pairs and 3 of 16 gold-`related` pairs were predicted as `updates`. This matches the confusability risk 03-04's blind-labeling round already flagged (every one of that round's 10 disagreements was a contradicts/updates confusion), and explains why `needs_review` (30 of 70) is high relative to overall accuracy — the model's own probability distribution is genuinely split on this boundary even when its top choice happens to be correct.
