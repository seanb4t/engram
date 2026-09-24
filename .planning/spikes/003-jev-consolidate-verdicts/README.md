---
spike: 003
idea: jev-typed-decisions
name: jev-consolidate-verdicts
type: standard
validates: "Given ~39 labeled pairs of real spine records, when Jev classifies each as duplicate / contradicts / related / unrelated, then accuracy is useful and its confidence separates right from wrong verdicts"
verdict: VALIDATED
related: [001]
tags: [jev, spine-review, consolidate, curation, calibration, supersession]
---

# Spike 003: Jev relation verdicts on real spine pairs

## What This Validates

Given labeled pairs of real engram spine records, when Jev answers a 4-way Choice (duplicate /
contradicts / related / unrelated) plus a same-subject Noul for each pair, then its accuracy
and calibration are good enough to power advisory verdicts on
`engram spine-review consolidate` candidates.

## How to Run

```sh
cd .planning/spikes/003-jev-consolidate-verdicts
go run main.go          # reads pairs.json → writes verdicts.json, prints metrics
open viewer.html        # load verdicts.json; filter wrong / low-confidence; record judgments
```

`pairs.json`, `verdicts.json` and `judgments.json` are **gitignored**: they hold verbatim spine
memory content, and this repo is public. To rebuild the fixture, run the build scripts kept
in the session scratchpad, or rebuild it from the spine the same way (below).

## Fixture

39 pairs from `repo:github.com/seanb4t/engram`, built read-only through engram MCP. Each side
is cut to about 1,500 characters, and two sides were kept longer so the deciding claim stays in.

| Gold | n | Source |
|---|---|---|
| contradicts | 11 | 10 real supersession links; 1 real unlinked contradiction (signing ON vs signing OFF) |
| duplicate | 10 | 7 real (supersession refinements, search neighbors, rule ↔ the record it restates); 3 synthetic paraphrases |
| related | 9 | search neighbors: same subject area, compatible facts |
| unrelated | 9 | random cross-subject pairs |

One pair is marked ambiguous and left out of scoring. Real duplicates are scarce in this spine:
most supersessions reverse a claim rather than restate it.

## Investigation Trail

1. Fixture assembled by a read-only subagent from supersession links, search neighbors and
   random pairs. It dropped three pairs whose deciding claim fell past the truncation point
   or whose disagreement came only from a config change over time.
2. Ran all 39 pairs at concurrency 4.
3. Read every miss (below) instead of stopping at the accuracy number.

## Results

**VALIDATED**, with one clear boundary weakness.

| Measure | Value |
|---|---|
| Accuracy (38 scored) | **0.84** |
| Brier score, 4-class | **0.182** (0 = perfect, uniform guess = 0.75) |
| Top-choice probability ≥ 0.9 | **n=26, accuracy 1.00** |
| Top-choice probability 0.7–0.9 | n=7, accuracy 0.71 |
| Top-choice probability < 0.7 | n=5, accuracy 0.20 |
| Cost, all 39 pairs | $0.0021 |
| Latency (concurrency 4, bigger inputs) | p50 447 ms · p90 1.6 s · max 3.5 s |

Confusion (rows = gold, columns = Jev):

| | duplicate | contradicts | related | unrelated |
|---|---|---|---|---|
| duplicate | 6 | 0 | 3 | 0 |
| contradicts | 0 | 9 | 2 | 0 |
| related | 0 | 1 | 8 | 0 |
| unrelated | 0 | 0 | 0 | 9 |

**Confidence is the finding.** Two-thirds of pairs came back at ≥ 0.9 and every one of them
was right. Every miss sat below 0.8. So an advisory consolidate column can show verdicts at
≥ 0.9 plainly and mark the rest "needs review", on this sample. That threshold is not
settled: with n=38, one more miss in the top bucket would move it.

**Where it misses.** All but one miss land on `related`, which is the conservative choice:

- **Refinement vs duplicate** (p12, p13, p17): B restates A's fact and widens it. Jev reads
  "adds a third defect" as a different fact. That is arguably right; the gold label is
  softest here.
- **Change over time vs contradiction** (p03, p11): "the deployed version is now 0.14.0" and
  "signing is now OFF" supersede an earlier state rather than disagree about the same moment.
  Jev calls them related. A 5th option, `updates` ("B is a newer state or a more complete
  version of the same fact"), would likely absorb most of these misses and the refinement
  misses. That is the obvious next iteration, and not run here.
- **One false contradiction** (p25, p=0.59): two different causes for the same kind of
  worktree failure.

Jev never mixed `unrelated` with anything else, and never called a duplicate a contradiction.

**Bonus (the use case in action):** ambiguous pair p18 holds a spine record saying
`--reset-phase-numbers` must **not** be passed. That conflicts with rule `rvmts69cz1`
(ALWAYS pass it). A stale record contradicting a user-blessed rule is exactly what a
curation pass should surface, and it came out of fixture building by accident.

Caveats: the gold labels come from one labeler (a subagent), and some pairs came from
supersession links that a curator had already found. Real `consolidate` candidates are picked
by vector similarity, so expect more `related` and fewer `unrelated` pairs than this balanced
set. Use `viewer.html` to check the misses yourself.
