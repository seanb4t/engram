---
phase: 08-registry-docs-tail
reviewed: 2026-08-21T00:00:00Z
depth: standard
files_reviewed: 16
files_reviewed_list:
  - CLAUDE.md
  - cmd/engram/spine_review_scan.go
  - cmd/engram/spine_review_verify.go
  - cmd/engram/summarize.go
  - cmd/engram/summarize_test.go
  - cmd/engram/sweep_scope.go
  - cmd/engram/sweep_scope_test.go
  - cmd/engram/testdata/catalog.golden
  - cmd/engram/testdata/help.golden
  - docs-site/src/content/docs/guides/migrate.md
  - docs-site/src/content/docs/guides/upgrade.md
  - docs-site/src/content/docs/reference/memory-record.md
  - docs-site/src/content/docs/reference/tools.md
  - internal/surfaces/normalize_test.go
  - internal/surfaces/rules.go
  - internal/surfacesgen/main.go
findings:
  critical: 0
  warning: 10
  info: 7
  total: 17
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-08-21
**Depth:** standard
**Files Reviewed:** 16
**Status:** issues_found

## Summary

Sixteen files across four plans: one registry rule + shared CLI guard (08-01),
two docs rewrites (08-02, 08-03), and a CLAUDE.md correction (08-04).

**What I verified and could not break.** The behavior-preservation claim holds:
`requireSweepScope`'s `scope != "" || allScopes` early-return is the exact De
Morgan dual of the three deleted `scope == "" && !allScopes` guards, and the
present-but-empty `--scope ""` case is handled identically (`resetClientFlags`
→ `resetEveryCommandFlagState` restores flag values, so the tests are not
order-dependent). I mutation-tested the gates rather than trusting them:
deleting the guard from `spine-review verify` turns
`TestSweepLeavesRejectMissingScopeIdentically` and
`TestSweepLeavesRejectPresentButEmptyScope` red; removing `cobraSummarizeFields`
from `exposedForTest()` turns `TestEveryRuleResolvesToNonEmptySurfaceSet` red.
All five test functions named in the plans exist and actually run (no
`[no tests to run]` greens). `validateRuleSet`'s Sentence-containment check is
satisfied — the new Sentence neither contains nor is contained by
`RulePurgeFilterRequiresScope`'s, the one it collides with on `--scope`/
`--all-scopes` vocabulary. The rule was **appended** to `rules` (declaration
order preserved), and both hand-maintained sources of truth
(`surfacesgen.ruleTargets`, `normalize_test.exposedForTest`) were updated
consistently. `go run ./internal/surfacesgen` produces zero drift;
`go test ./...`, `-shuffle=on`, `gofmt`, `go vet`, and `license-eye header
check` are all clean. Every docs claim I spot-checked against source is
correct — the asymmetric window boundaries (`not_before` Lte inclusive at
store.go:1012, `not_after` Gt exclusive at store.go:1016), the `expired`
suppresses `scheduled` precedence and `archived → superseded → expired →
scheduled` canonical order (memory_state.go:39-67, ui/src/lib/memorystate.ts),
the half-open `[created_after, created_before)` analogy (store.go:1289-1301),
the `max(CurrentVersion, m.SchemaVersion)` write-path floor (store.go:646) and
the `Store.Upsert` boundary that escapes it, the Connect `Memory` message
carrying all six record-state fields, `warnPendingMigrations`' read-only
10s-bounded startup probe, `migrateRevertValidateTo`'s three usage errors,
and the 23-command inventory against `catalog.golden`. `migrate.md` correctly
refuses to claim resumability and explicitly documents the
non-shrinking-backlog termination guard.

**Where it is weak.** No BLOCKER-class defect survived scrutiny — but the
review found ten warnings clustered in three places: (1) the compose-once
discipline is only half-applied — every `--all-scopes` Usage string now states
the constraint **twice**, once from the registry and once still hand-typed;
(2) the new rule's gate coverage rests on hand-maintained whitelists and on
incidental prose ("dry-run" appearing in a docs table row), with no
zero-occurrence gate and no unit-level negative proof of the `SurfaceFields`
narrowing — precisely this repo's recurring vacuous-gate shape; (3) four
factual/consistency defects in `CLAUDE.md` and `guides/cli.md`, which per this
repo's own priorities are defects rather than nits because agents and operators
act on them unverified.

## Warnings

### WR-01: The rule is stated twice on every `--all-scopes` flag — one copy still hand-typed

**File:** `cmd/engram/spine_review_scan.go:152`, `cmd/engram/spine_review_verify.go:665`, `cmd/engram/summarize.go:116`

**Issue:** Each Usage string is now:

```
"sweep every scope (required if --scope is omitted); mutually exclusive with --scope; " + sweepScopeRule().Sentence
```

which renders as `sweep every scope (required if --scope is omitted); mutually
exclusive with --scope; a sweep requires an explicit --scope or --all-scopes:
name one scope, or opt into every scope` (confirmed in
`testdata/help.golden:389,405,449`). The leading clause **"required if
`--scope` is omitted"** and the appended registry Sentence assert the identical
constraint. The whole point of composing from the registry (D-03) is that the
constraint has exactly one authored copy; this leaves a second, hand-typed one
in the same string. If the Sentence is ever reworded, the hand-typed prefix
silently disagrees with it on the same line of `--help`, and no gate catches
that: `TestSurfaceConformanceCobraUsage` only checks that the Sentence is
*contained*, never that the surrounding prose does not contradict it. It also
reads badly — the `--all-scopes` entry tells the reader about `--scope`
twice.

**Fix:** Drop the redundant hand-typed clause and keep only the composed
sentence, in all three files:

```go
spineReviewScanCmd.Flags().BoolVar(&spineScanAllScopes, "all-scopes", false,
    "sweep every scope; mutually exclusive with --scope; "+sweepScopeRule().Sentence)
```

Then regenerate the goldens (`task surfaces:gen`).

### WR-02: `rules.go` claims a `field=scope` error-envelope attribution that no surface produces

**File:** `internal/surfaces/rules.go:167-168`

**Issue:** The declaration comment states, in the present tense:

> `Fields is the flag pair alone (["scope", "all-scopes"]) -- it drives field=scope attribution on the error envelope.`

There is no error envelope for this rule. The only enforcement site is
`requireSweepScope` (`cmd/engram/sweep_scope.go:32`), which returns
`usageErrorf("%s", ...)` — a bare usage error with no `field=`/`hint=` prefix.
There is no `conditionalErrf` call site for `RuleSweepScopeOrAllScopesRequired`
anywhere in the tree (`internal/server/tools.go` and `connectapi.go` reference
five other rules; none is this one), and `TagForm` is deliberately empty
because the rule has no MCP/Connect lane at all. `Hint: "conditional_required"`
is likewise inert for the same reason. `rules.go` is the single source of truth
every other surface reads; a false present-tense claim there will be believed
by the next contributor wiring a lane for a sweep verb.

**Fix:** State the conditional, matching how `RuleDestructiveRequiresApply`
handles its own no-envelope case:

```go
// Fields is the flag pair alone (["scope", "all-scopes"]). This rule is
// CLI-only: the sole enforcement site is requireSweepScope, which raises a
// bare usageErrorf, so no error envelope carries field=/hint= for it today.
// Fields WOULD drive field=scope attribution if a lane ever raised it via
// conditionalErrf; Hint is declared for that future, not for a live surface.
```

### WR-03: No gate asserts ZERO remaining hand-rolled sweep guards

**File:** `cmd/engram/sweep_scope_test.go:133-141`

**Issue:** `enforcing` and `nonEnforcing` are hand-maintained literal maps, and
the walk skips every command in neither (`sweep_scope_test.go:146-148`). A
fourth sweep-style leaf added later — with its own `--scope`/`--all-scopes`
pair and its own inline `if scope == "" && !allScopes { return usageErrorf(...) }`
— is silently uncovered by this whole file. `TestSurfaceConformanceCobraUsage`
(`cmd/engram/surfaces_test.go:42`) will not catch it either, because the
`SurfaceFields` narrowing to `{scope, all-scopes, dry-run}` resolves that new
leaf not-applicable unless it happens to carry `--dry-run` as well. This is
exactly the "count, not zero" shape this repo's own review priorities call out:
the conversion of three sites is pinned; the *absence of a fourth* is not.
I confirmed the old literal `"--scope <scope> or --all-scopes is required"` is
gone from all Go source today — nothing prevents its return.

**Fix:** Add a zero-occurrence gate over the live tree, not a count:

```go
// TestNoHandRolledSweepScopeGuards asserts ZERO commands exposing both
// --scope and --all-scopes reject with anything other than the registered
// Sentence -- the whitelist above pins the known leaves; this pins the
// absence of unknown ones.
func TestNoHandRolledSweepScopeGuards(t *testing.T) {
    for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
        if cmd.Flags().Lookup("scope") == nil || cmd.Flags().Lookup("all-scopes") == nil {
            continue
        }
        key := commandKey(cmd)
        if !enforcingSweepLeaves[key] && !nonEnforcingSweepLeaves[key] {
            t.Errorf("%s exposes --scope and --all-scopes but is in neither the "+
                "enforcing nor the non-enforcing set -- classify it and either "+
                "route it through requireSweepScope or record why it is exempt", key)
        }
    }
}
```

Pair it with a source-level zero-assertion (`rg -o` count == 0) for the old
literal in `cmd/engram/*.go`.

### WR-04: The `SurfaceFields` narrowing has no unit-level negative proof

**File:** `internal/surfaces/normalize_test.go:92-98,107`

**Issue:** `cobraSummarizeFields` was added and unioned into a single flat
`SurfaceCobraUsage` list. That union is what makes
`TestEveryRuleResolvesToNonEmptySurfaceSet` pass (verified by mutation: removing
it produces `rule "sweep-scope-or-all-scopes-required": ApplicableSurfaces =
empty`). But a flat union **cannot express** the thing `SurfaceFields` exists
to do — exclude a command that exposes `scope` + `all-scopes` but not
`dry-run`. The registry comment spends 27 lines arguing that
`spine-review consolidate` and `spine-review purge` must resolve
not-applicable; nothing in `internal/surfaces` tests that. Contrast
`TestDiscoveryNotSchedulableExcludesCategoryOnlySurfaces`
(`normalize_test.go:258`), which does exactly this for the one other rule with
a `SurfaceFields` override: it builds a deliberately narrow exposed map and
asserts `len(got) != 0` is a failure. The new rule's narrowing is therefore
proven only by the hand-maintained whitelist in `cmd/engram` (see WR-03), which
is the weaker of the two mechanisms.

**Fix:** Add the mirror of the discovery test:

```go
func TestSweepScopeRuleExcludesScopeAndAllScopesOnlySurfaces(t *testing.T) {
    rule, ok := RuleByID(RuleSweepScopeOrAllScopesRequired)
    if !ok { t.Fatal("RuleSweepScopeOrAllScopesRequired not found in registry") }
    if len(rule.SurfaceFields) == 0 {
        t.Fatal("no SurfaceFields declared -- the narrowing this test guards is missing")
    }
    // Mirrors spine-review consolidate's/purge's own flag set: both scope
    // flags, never dry-run. Neither enforces this rule.
    pairOnly := map[Surface][]string{
        SurfaceCobraUsage: {"scope", "all-scopes", "class", "category", "tags", "older-than"},
    }
    if got := ApplicableSurfaces(rule, pairOnly); len(got) != 0 {
        t.Errorf("ApplicableSurfaces(sweep rule, scope+all-scopes-only set) = %v, want empty", got)
    }
    withDryRun := map[Surface][]string{
        SurfaceCobraUsage: {"scope", "all-scopes", "dry-run"},
    }
    if got := ApplicableSurfaces(rule, withDryRun); !containsSurface(got, SurfaceCobraUsage) {
        t.Errorf("ApplicableSurfaces(sweep rule, summarize-missing set) = %v, want cobra_usage", got)
    }
}
```

### WR-05: The prose-surface gate for this rule hangs on one incidental docs table row

**File:** `internal/surfacesgen/main.go:123-136`, `docs-site/src/content/docs/reference/tools.md:528,536`

**Issue:** `checkProseSurface` only runs for a rule on a surface that
`ApplicableSurfaces` resolves applicable, and `buildProseExposed`
(`internal/surfaces/conformance_test.go:70`) derives that from
`SurfaceApplicabilityFields(rule)` = `{scope, all-scopes, dry-run}` via a raw
`strings.Contains` scan of the prose files. `SurfaceDocsSite` therefore resolves
applicable only because `dry_run`/`dry-run` literally appears in
`reference/tools.md` (exactly one occurrence — the flag table row at line 536)
or in `guides/cli.md`. If both of those incidental mentions are ever reworded
away (e.g. the summarize-missing flag table is restructured), `SurfaceDocsSite`
resolves **empty**, `checkProseSurface` stops running for this rule entirely,
and the anchored region at tools.md:528 can then drift from the registry with
the suite still green. That is a vacuous gate reached by editing prose, with no
warning — and it is the failure shape this repo has shipped three phases in a
row. Note `surfacesgen` would keep rewriting the region correctly; the loss is
the *check*, so the failure is silent in both directions.

**Fix:** Do not let a gate's existence depend on a `strings.Contains` hit in
prose. Assert the applicability outcome directly, so a reword fails loudly:

```go
// In internal/surfaces/conformance_test.go, alongside the per-rule gate:
// rules whose prose surface is asserted to exist, not merely derived.
var proseRequired = map[string][]Surface{
    RuleSweepScopeOrAllScopesRequired: {SurfaceDocsSite},
}
// ... then, per rule: if want, ok := proseRequired[rule.ID]; ok {
//   for _, s := range want {
//     if !containsSurface(applicable, s) {
//       t.Errorf("rule=%s: %s no longer resolves applicable -- the prose that "+
//         "exposed its SurfaceFields was reworded; the anchored region is now "+
//         "unchecked", rule.ID, s)
//     }
//   }
// }
```

### WR-06: `guides/cli.md` documents mutual exclusivity for the three sweep leaves but never their required-ness

**File:** `docs-site/src/content/docs/guides/cli.md:161-164,179-181,206-216` (vs. `:229-236`)

**Issue:** For `summarize-missing`, `spine-review scan`, and `spine-review
verify`, the CLI guide says only *"`--scope` and `--all-scopes` are mutually
exclusive"* — three times, and nothing more. For `spine-review consolidate`
immediately below, it says *"mutually exclusive; **supplying neither** sweeps a
well-defined empty result … never an accidental whole-spine sweep."* Read
together, that contrast tells an operator that supplying neither is a
*supported* mode on all four, differing only in what it returns. On three of
them it is a hard exit-2 rejection. The published operator guide is materially
incomplete, and misleading specifically because of the neighbouring
counter-example.

`rules.go:180-190` justifies not anchoring here on the grounds that "cli.md
documents none of the three sweep leaves' scope/all-scopes contract in a way
that names this rule." That inference is backwards: cli.md *does* document the
contract for all three leaves — it documents only half of it. (Gate-wise the
choice is harmless: `guides/cli.md` already exposes scope + all-scopes +
dry-run, so `SurfaceDocsSite` resolves applicable from either file and
`tools.md`'s single anchor satisfies it. This is a docs-completeness defect,
not a gate failure.)

**Fix:** Add an anchored region to `guides/cli.md` for each of the three leaves
(or one shared paragraph in the operator-commands preamble), and add
`{path: "docs-site/src/content/docs/guides/cli.md", kind: kindMarkdown}` to
`ruleTargets[surfaces.RuleSweepScopeOrAllScopesRequired]` in
`internal/surfacesgen/main.go:134`, then `task surfaces:gen`.

### WR-07: `CLAUDE.md`'s archived paragraph under-states the recall gate it claims to mirror

**File:** `CLAUDE.md:174-177`

**Issue:** The new text says archiving *"shares supersession's
soft-hidden-but-still-fetchable-by-id contract: an archived record drops out of
`search_memory`/`list_memory` but stays reachable by id via `get_memory`."*
Two tools. The supersession paragraph it explicitly cross-references
(`CLAUDE.md:157-159`) names **four**:
`search_memory`/`list_memory`/`search_discovery`/`list_scheduled`. The
implementation matches the four-tool version, not the two-tool one:
`qdrant.NewIsEmpty("archived_at")` appears at all four recall sites —
`internal/store/store.go:1129` (Search), `:1228` (SearchDiscovery), `:1399`
(List), `:1635` (ListScheduled). An agent acting on this routing file would
conclude an archived scheduled record still surfaces in `list_scheduled`. It
does not.

**Fix:**

```markdown
`archived_at` shares supersession's soft-hidden-but-still-fetchable-by-id
contract: an archived record drops out of
`search_memory`/`list_memory`/`search_discovery`/`list_scheduled` but stays
reachable by id via `get_memory`.
```

### WR-08: `CLAUDE.md` calls `migrate-set-owner` an "alias" — it is a separate command with an incompatible flag set

**File:** `CLAUDE.md:15`

**Issue:** The new Layout row reads `migrate-remap-owner` (alias:
`migrate-set-owner`, deprecated)`. `migrate-set-owner` is not a cobra alias: it
is its own `&cobra.Command{Use: "migrate-set-owner"}`
(`cmd/engram/migrate.go:31`) with a *different* flag set. Per
`testdata/catalog.golden`:

- `migrate-remap-owner`: `apply, from, from-anon, from-missing, output, timeout, to`
- `migrate-set-owner`: `output, owner, timeout` — no `--apply`, no `--from*`, no `--to`

An agent that reads "alias" will construct `engram migrate-set-owner
--from-missing --to <owner>`, which fails on unknown flags. (The pre-existing
line at `CLAUDE.md:135` carries the same imprecision; it should be corrected in
the same pass.)

**Fix:**

```markdown
`migrate-remap-owner` (supersedes the deprecated `migrate-set-owner`, which is
a separate command with its own `--owner` flag, not a flag-compatible alias);
```

### WR-09: `CLAUDE.md`'s command inventory omits that `backfill-short-ids` is deprecated

**File:** `CLAUDE.md:15`

**Issue:** The row flags `migrate-set-owner` as deprecated but lists
`backfill-short-ids` as an ordinary operator command. It is deprecated in code:
`backfillShortIDsCmd.Deprecated = "use: engram migrate"`
(`cmd/engram/backfill.go:58`), and both `guides/migrate.md:29-31` and
`guides/cli.md:166-168` describe it as a thin delegating alias onto `engram
migrate`. Given the row explicitly annotates the *other* deprecated verb, the
omission reads as a positive assertion that this one is current — and this file
is consumed by agents without verification.

**Fix:** `…; `backfill-short-ids` (deprecated — delegates to `engram migrate`); …`

### WR-10: `summarize_test.go`'s new test is a weaker duplicate, and its comment describes deleted code

**File:** `cmd/engram/summarize_test.go:40-56`

**Issue:** Two problems in one block.

1. The doc comment is factually wrong as committed: *"pins the bare
   usageErrorf guard (mirroring spine-review scan/verify's identical wording)"*
   and *"taken against unmodified main, before the guard is converted onto the
   registry, as a characterization pin."* There is no bare `usageErrorf` guard
   in `summarize.go` any more — 08-01 replaced it in the same phase — and the
   "before the conversion" framing describes a state that no longer exists in
   the tree. The comment sends a reader looking for code that was deleted.
2. The test is now strictly subsumed by
   `TestSweepLeavesRejectMissingScopeIdentically/summarize-missing`
   (`sweep_scope_test.go:61`), which runs the same `runClient(t,
   "summarize-missing")` and asserts the same exit code **plus** the message.
   This one asserts only `exitUsage`, so it would stay green if the leaf started
   rejecting for an unrelated reason. `resetCommandFlagState(t,
   summarizeMissingCmd)` at line 49 is also redundant — `resetClientFlags`
   already folds in `resetEveryCommandFlagState(rootCmd)`
   (`clienttest_test.go:159`).

**Fix:** Delete the test, or — if the characterization pin is wanted as a
per-leaf regression net — rewrite the comment to describe the code as it now
stands and strengthen the assertion to include the registered Sentence, so it
is not weaker than the shared table test:

```go
// TestSummarizeMissingRequiresScopeOrAllScopes is summarize-missing's own
// per-leaf pin of the shared requireSweepScope guard, independent of the
// three-leaf table in sweep_scope_test.go.
func TestSummarizeMissingRequiresScopeOrAllScopes(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)
	if !ok {
		t.Fatal("surfaces.RuleSweepScopeOrAllScopesRequired not found in registry")
	}
	resetClientFlags(t)
	_, _, err := runClient(t, "summarize-missing")
	// ... assert exitUsage AND err.Error() == rule.Sentence
}
```

## Info

### IN-01: The new test cements `spine-review consolidate`'s false-clean report

**File:** `cmd/engram/sweep_scope_test.go:138-141,159-161`

**Issue:** `nonEnforcing` declares `spine-review consolidate` must **not**
carry the sentence, and line 159 makes that an assertion. Meanwhile
`NearDuplicates` with `Scope:"" AllScopes:false` returns a well-defined empty
result (`internal/store/spine.go:380-387`) — so `engram spine-review
consolidate` with neither flag reports "no near-duplicate candidates" and an
operator reads the spine as clean. The behavior is pre-existing and out of this
diff, but this phase converts "consolidate happens not to say this" into an
enforced invariant with no pointer to the footgun it perpetuates.

**Fix:** Add a comment at the `nonEnforcing` declaration linking to
`guides/cli.md:234-236`'s explanation, or file a follow-up issue to make
consolidate reject neither-flag and update this map in the same change.

### IN-02: The non-enforcement assertion only inspects `--all-scopes`

**File:** `cmd/engram/sweep_scope_test.go:149,155`

**Issue:** Both the positive and negative assertions read
`cmd.Flags().Lookup("all-scopes").Usage` only. A future change that publishes
the Sentence onto `spine-review purge`'s `--scope` (or `--category`) Usage
would pass this test while violating the plan's stated prohibition.

**Fix:** Scan every flag on the command (`collectFlags(cmd)`, as
`surfaces_test.go:80` already does) rather than one named flag.

### IN-03: No per-rule pin test for the new rule, unlike its two closest siblings

**File:** `internal/surfaces/rules_test.go:51,70`

**Issue:** `TestRuleByIDDestructiveRequiresApply` and
`TestRuleByIDVerifyFailOnValues` each pin their rule's declared `Fields`/`Hint`/
`TagForm`. Neither `RulePurgeFilterRequiresScope` nor
`RuleSweepScopeOrAllScopesRequired` has one, so a typo in `Fields` or `Hint`
for the new rule is caught only indirectly.

**Fix:** Add `TestRuleByIDSweepScopeOrAllScopesRequired` following the existing
two-test template.

### IN-04: `CLAUDE.md`'s `migrate` phrasing parallels a parent-only command

**File:** `CLAUDE.md:15`

**Issue:** `` `migrate` (`status`, `revert`) `` uses the same shape as
`` `spine-review` (`scan`, `verify`, …) ``, but `spine-review` is parent-only
(no flags in `catalog.golden`) while `migrate` is itself a runnable verb with
`--apply`/`--output`/`--timeout` — the primary forward sweep, as
`guides/upgrade.md:332` now instructs (`engram migrate --apply`).

**Fix:** `` `migrate` (itself the forward sweep; subcommands `status`, `revert`) ``

### IN-05: `tools.md` demoted a bold required-ness callout into mid-sentence prose

**File:** `docs-site/src/content/docs/reference/tools.md:528`

**Issue:** `**Either `--scope` or `--all-scopes` is required.**` — a standalone
bold callout directly under the synopsis — became a long sentence whose
operative clause sits after a comma and a parenthetical. Correct content, less
scannable, and the only place on the page where the requirement appears.

**Fix:** Keep the anchored region but restore the emphasis, e.g.
`**Required:** <!-- engram:rule:start … -->…<!-- engram:rule:end … -->` on its
own line.

### IN-06: `guides/migrate.md` never says `backfill-short-ids` is deprecated

**File:** `docs-site/src/content/docs/guides/migrate.md:29-31`

**Issue:** It says the command "is now a thin delegating alias onto the same
sweep", which is accurate but stops short of the code's own posture
(`Deprecated = "use: engram migrate"`, `cmd/engram/backfill.go:58`). A reader
of the migration guide has no reason to stop using it.

**Fix:** `…is now a deprecated thin delegating alias onto the same sweep — new
automation should call `engram migrate` directly.`

### IN-07: `tools.md` lists `list_scheduled` among the tools that "hide" scheduled records

**File:** `docs-site/src/content/docs/reference/tools.md:228-230`

**Issue:** The edited lead-in says `get_memory` *"returns every state
`search_memory`/`list_memory`/`search_discovery`/`list_scheduled` hide"* and
then lists `scheduled` as one of them. `list_scheduled` exists specifically to
surface `scheduled` records (`state` defaults to `scheduled`). The tool list is
pre-existing text, but the line was rewritten in this diff without correcting
it.

**Fix:** `…every state `search_memory`/`list_memory`/`search_discovery` hide
(and, for `expired`/`superseded`/`archived`, that `list_scheduled` hides too)…`

---

_Reviewed: 2026-08-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
