# Curation verdicts (spine-review consolidate)

## Requirements

- Provider-neutral decision interface (see `decision-transport.md`); Jev is one backend.
- Off by default; **advisory only** — a verdict is a column in `consolidate` output, never an
  automatic supersede / delete / archive. Mutations stay with the agent or operator (the
  `curating-spine` skill's explicit-consent contract is unchanged).
- Spike fixtures built from real memory content are gitignored (public repo).

## How to Build It

1. For each candidate pair from `engram spine-review consolidate`, send ONE Decisions request:
   `state = {context, record_a, record_b}` (truncate each side, ~1.5k chars was enough; keep
   the deciding claim), questions:
   - `relation` — Choice over `duplicate` / `contradicts` / `related` / `unrelated`, criteria:
     - duplicate: "Both state the same fact; keeping both is redundant (one may be more complete)."
     - contradicts: "They make incompatible claims about the same subject; one corrects or reverses the other."
     - related: "Same subject area, but different and compatible facts; both are worth keeping."
     - unrelated: "Different subjects."
   - `same_subject` — Noul: "Are both records about the same specific subject?"
2. Surface `choice`, the top-choice probability, and `same_subject` in the `--output json`
   document (the operator-tier JSON is the contract; text is a rendered view).
3. Tier the display by top-choice probability: **≥ 0.9 → show the verdict plainly;
   < 0.9 → "needs review"**. On the spike sample this split was clean (see Constraints) but
   the sample is small — make the threshold configurable and re-measure on real candidates.
4. Consider a 5th option, `updates` — "B is a newer state or a more complete version of the
   same fact" — before shipping; it should absorb the two dominant miss classes (below).
   Untested.
5. Run requests concurrently (4 in the spike); cost is negligible (~$0.00005/pair).

## What to Avoid

- Do not let a verdict mutate anything — not even at p ≥ 0.9.
- Do not read `related` as "safe to ignore": most misses fell into `related`, so it hides
  refinements and state changes. Pair it with `same_subject` (high same-subject + `related`
  is worth a look).
- Do not truncate blindly: the deciding phrase can sit deep in a record. Truncate with the
  claim kept, or feed summaries.
- Do not commit fixtures with verbatim spine content (public repo).

## Constraints

| Measure (39 real spine pairs, 38 scored) | Value |
|---|---|
| Accuracy | 0.84 |
| 4-class Brier | 0.182 (uniform = 0.75) |
| Top-choice p ≥ 0.9 | n=26, accuracy 1.00 |
| Top-choice p 0.7–0.9 | n=7, accuracy 0.71 |
| Top-choice p < 0.7 | n=5, accuracy 0.20 |
| `unrelated` confusions | 0 |
| duplicate→contradicts confusions | 0 |

Miss classes: refinement read as `related` (B widens A's fact); change-over-time read as
`related` (e.g. "deployment is now 0.14.0", "signing is now OFF"); one false `contradicts`
on two different causes of the same failure kind. Gold labels came from one labeler; real
`consolidate` candidates (vector-similar) will skew toward `related`.

## Origin

Synthesized from spikes: 003 (transport facts from 001)
Source files available in: sources/003-jev-consolidate-verdicts/ (fixture files are local-only)
