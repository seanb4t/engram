# Phase 1: Interface Enforceability - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-03
**Phase:** 1-Interface Enforceability
**Areas discussed:** Fate of exit 1 and 3, Timeout knob shape, Flag-group adoption breadth, Migration-proof mechanism

---

## Area selection

| Option | Description | Selected |
|--------|-------------|----------|
| Fate of exit 1 and 3 | Criterion names only 0/2/4/5 but the code defines six constants including exitGeneric=1 and exitAuth=3 | ✓ |
| Timeout knob shape | Flag vs flag+env, default value, and which exit code a timeout reports | ✓ |
| Flag-group adoption breadth | Which of the three exclusivity claim sites convert to cobra's declarative API | ✓ |
| Migration-proof mechanism | How the before-table is captured, and what closes the consumer audit | ✓ |

**User's choice:** all four areas.

### Todo cross-reference

| Option | Description | Selected |
|--------|-------------|----------|
| Neither | Both matches look like keyword false positives | ✓ |
| Fold the stale-branch cleanup | `docs/v0.12.x-phase-01-context` name collides with this phase's output | |
| Fold the Cloudflare token rotation | This phase writes `guides/upgrade.md` and needs a working docs-site deploy | |

**User's choice:** Neither. Both recorded in CONTEXT.md `<deferred>` as reviewed-but-not-folded.

---

## Fate of exit 1 and 3

### Does exitAuth = 3 survive the unification?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — 3 stays, criterion is shorthand | Read "0/2/4/5" as the codes the unification moves things INTO, not an exhaustive taxonomy; 3 is already distinct, tested, advertised, and reachable only through the D-10 mapper | ✓ |
| No — collapse 3 into 2 | Matches the criterion word-for-word but conflates an expired token with a bad flag | |
| No — collapse 3 into 5 | Keeps auth on the retryable side but implies retrying helps when it won't | |

**User's choice:** Yes — 3 stays.

### What becomes of exitGeneric = 1?

| Option | Description | Selected |
|--------|-------------|----------|
| Keep, redefine as unreachable-by-design internal error | Every classified path is typed; 1 survives only as root.go's errors.As backstop, documented as exactly that | ✓ |
| Retire 1 entirely — delete the constant | Strongest guarantee, but an untyped error would silently masquerade as a usage error | |
| Keep 1 as a reachable, documented catch-all | Smallest diff, but concedes the two-taxonomy split #467 exists to close | |

**User's choice:** Keep, redefine as unreachable-by-design.
**Notes:** The rejected "retire entirely" option was flagged as vulnerable to the collapse failure mode recorded in memory `667p88n2be` — a loose assertion still passes on a collapsed classification.

### How finely should the six operator commands classify failures?

| Option | Description | Selected |
|--------|-------------|----------|
| Full classification — same 2/3/4/5 vocabulary as client verbs | Bad flag → 2, backend unreachable → 5, auth → 3, missing target → 4 | ✓ |
| Coarse — validation → 2, everything else → 5 | Cheap and safe, but scripts still can't branch usefully | |
| Reuse the D-10 mapper wholesale | Minimal judgment, but most operator errors aren't Connect errors so they'd fall to the generic arm — reintroducing 1 through the back door | |

**User's choice:** Full classification.

---

## Timeout knob shape

### How should the CLI timeout be configured?

| Option | Description | Selected |
|--------|-------------|----------|
| `--timeout` + `ENGRAM_TIMEOUT` matching resolveServerURL's pattern | Keeps client config out of the server-side koanf registry | |
| `--timeout` flag only | Simplest surface, most CI friction | |
| Add it to the koanf config registry as a first-class field | Validation and ENGRAM_ prefix for free, at the cost of the test-literal tax | |

**User's choice:** *Other (free text)* — "I think we need to expand the scope and run all client flags/settings through koanf."
**Notes:** This went beyond the offered options and beyond REQ-cli-request-timeout, so a placement question was raised rather than absorbing the expansion silently.

### Where should "all client flags/settings through koanf" land?

| Option | Description | Selected |
|--------|-------------|----------|
| In Phase 1 — timeout arrives as the first koanf client field | Strongest coherence with the phase goal; avoids writing a resolveTimeout helper only to delete it next. Cost: breaking exit-code change AND a client-wide config refactor in one phase | ✓ |
| Its own phase — Phase 1 ships `--timeout` the existing way | Keeps the breaking change reviewable in isolation, but writes `--timeout` twice | |
| Split — scaffold + timeout now, migrate the rest later | New code born right, but two resolution mechanisms coexist — the split-brain condition this milestone keeps finding | |

**User's choice:** In Phase 1.
**Notes:** Flagged at the time that this widens Phase 1 past its four roadmapped requirements, and that the ROADMAP/REQUIREMENTS update must go through `/gsd-phase` rather than a hand-edit, per rule `8dfdhfs5nn` and memory `apfg4fe199`.

### What default timeout, and what does 0 mean?

| Option | Description | Selected |
|--------|-------------|----------|
| 30s default; 0 rejected as a usage error | REQ demands a finite deadline, so no value may mean unbounded | ✓ |
| 30s default; 0 means unbounded, matching migrate.go | Consistent with the existing flag, but reintroduces the indefinite block the REQ eliminates | |
| 60s default; 0 rejected | More forgiving ceiling, longer hang before a dead server is reported | |

**User's choice:** 30s; 0 rejected.

### Which exit code does a timeout report?

| Option | Description | Selected |
|--------|-------------|----------|
| 5 (unavailable) — reuse the existing code | Keeps the taxonomy closed in the phase that's unifying it | |
| A new dedicated code (6) | Distinguishes "never answered in time" from "couldn't connect"; widens the taxonomy and adds a code to the consumer audit | ✓ |
| 2 when the timeout was too low, 5 otherwise | Most informative in principle, but the branch would be a guess dressed as a classification | |

**User's choice:** New dedicated code 6.
**Notes:** Chosen against the recommendation. Consequence noted at the time: the migration audit must now cover a *new* code as well as changed ones, and `TestCatalogExitCodesMatchMapper` will enforce the catalog entry.

---

## Flag-group adoption breadth

### Which exclusivity claims convert to cobra's declarative API?

| Option | Description | Selected |
|--------|-------------|----------|
| All three — paging trio, scope/cross-spine, migrate's exactly-one | Eliminates the three-tier enforcement condition; migrate.go uses MarkFlagsMutuallyExclusive + MarkFlagsOneRequired | ✓ |
| Two — leave migrate.go alone | Keeps buildRemapSource pure, but leaves a bare-exit-1 site the other half of the phase must fix anyway | |
| One — only the unenforced paging trio | Closes the real bug with minimal risk, but "declared where enforced" stays false for two of three sites | |

**User's choice:** All three.

### Is `--page-token` with `--offset` an error, or still silently ignored?

| Option | Description | Selected |
|--------|-------------|----------|
| Error — all three become one mutually-exclusive group | Silently ignoring an explicitly-passed flag is the same defect class as an unenforced claim | ✓ |
| Keep ignoring — only offset/cursor-mode are exclusive | No new breakage, but preserves a combination governed by undocumented precedence | |
| Error only when `--offset` is non-zero | Cobra's flag groups key on Changed, not value, so this needs a hand-rolled guard — reintroducing the tier just eliminated | |

**User's choice:** Error.
**Notes:** Surfaced during scouting that the help text makes two *different* claims about the trio — `--offset`/`--cursor-mode` "mutually exclusive" vs `--page-token` "ignores `--offset`" — so this was a real behavior fork, not a formality.

---

## Migration-proof mechanism

### How does the before-table get captured so it's genuinely a "before"?

| Option | Description | Selected |
|--------|-------------|----------|
| Its own plan, committed before any behavior change | The commit itself proves the baseline was observed, not reconstructed; the only option verifiable by a third party after the fact | ✓ |
| One table with before/after columns, landed with the change | Fewer commits, but the "before" column is authored from reading code rather than observed from running it | |
| Capture the baseline as a golden file | Cheap, and the diff is the upgrade-guide input, but invites re-blessing the snapshot instead of confronting the change | |

**User's choice:** Its own plan, committed first.
**Notes:** Memory `nczgrtfec2` applied — assert before/after codes are distinct where claimed to change and identical where claimed not to, rather than a loose "as expected" assertion that passes on a collapsed classification.

### What closes "an audit of known consumers"?

| Option | Description | Selected |
|--------|-------------|----------|
| In-repo sweep + documented statement of external posture | Names what WAS checked rather than implying a survey of users who can't be enumerated | ✓ |
| In-repo sweep only | Fully verifiable, but leaves the upgrade guide silent on external consideration | |
| In-repo sweep + a GitHub issue soliciting affected users | Genuinely surfaces unknown consumers, but blocks milestone completion on external response | |

**User's choice:** In-repo sweep + documented external posture.

---

## Claude's Discretion

- Mechanism for intercepting cobra's flag-group validation errors and typing them to exit 2
  (`SetFlagErrorFunc` on root vs central classification in `Execute()`). Outcome fixed; mechanism open.
- Rewording of the D-17 note in `catalog.go:92-98`, whose published "1, not 2" promise is retracted.
- The shape of the client koanf config struct introduced by the D-04 scope expansion.
- Whether `client_common.go:236`'s shared guard is retained as a defense-in-depth backstop or removed.

## Deferred Ideas

- None from scope creep. The one expansion raised (koanf client-config unification) was deliberately
  placed *into* this phase rather than deferred.
- Reviewed-but-not-folded todos: Cloudflare API token rotation for docs-site deploy; stale
  `docs/v0.12.x-phase-01-context` branch cleanup.
