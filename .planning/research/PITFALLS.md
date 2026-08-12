# Pitfalls Research

**Domain:** Curation/maintenance tooling + interface (CLI/MCP) audit added to an already-shipped,
correctable memory store
**Researched:** 2026-08-03
**Confidence:** HIGH (destructive-op design, CLI/semver conventions, alert-fatigue mechanics —
cross-checked against multiple independent sources) / MEDIUM (agent-driven-curation consent
architecture — an emerging area with few mature precedents, reasoned from incident reports plus
this repo's own `supersede_memory`/`store_rule` consent gates)

## Critical Pitfalls

### Pitfall 1: Delete-before-extract ordering has no enforcement mechanism

**What goes wrong:**
`spine-review` purges/consolidates a phase's collapsed per-phase records before the reusable
gotchas inside them are extracted into standalone records. The extraction step is a *judgment
call* (semantic — "is this worth keeping as a standalone fact") that the CLI cannot make; if the
CLI's structural purge step runs on a schedule, a threshold, or a careless flag combination ahead
of the skill's semantic pass, the source material for extraction is gone. This is not
hypothetical — rule `7smp8vy9hr` already mandates this exact ordering today, and nothing in the
codebase enforces it.

**Why it happens:**
The CLI and the skill are two separate execution contexts (a deterministic binary vs. an LLM
agent invoked in a separate session) with no shared transactional boundary. A CLI that exposes
`purge`/`consolidate`/`archive` as independently invocable subcommands makes it trivial for an
operator (or a script, or a future autonomous agent) to call `purge` without ever having called
`extract`. Nothing about the CLI's own state distinguishes "extraction ran and found nothing
worth keeping" from "extraction never ran."

**How to avoid:**
- Make `purge`/`archive` refuse to run against a target set until a **recorded, timestamped
  extraction pass** exists for that set (a state marker the CLI itself writes and checks — not an
  honor system). `spine-review consolidate` should be the only path that transitions a target from
  "reviewable" to "purge-eligible," and it should require the extraction step to have executed
  (even if its answer was "nothing to extract") — not merely permit it to have executed.
- Never make `purge` a single top-level verb that skips the pipeline. If an operator needs an
  emergency bypass, require an explicit `--i-extracted-manually` flag that is loud in `--help` and
  logged, not the default path.
- The skill's propose-never-perform contract (see Pitfall 4) must write its own completion marker
  the CLI can check — don't rely on the operator to have "definitely run the skill first."

**Warning signs:**
- A `spine-review purge` run that produces zero `store_memory` calls beforehand in the session
  transcript.
- Any code path where `purge` and `consolidate`/`extract` are structurally independent
  subcommands with no shared precondition check between them.
- Passing review with a happy-path test that only exercises "extract, then purge" — the
  order-violating case (`purge` first) is exactly the case that must be a test, and it's the case
  most plans forget to write because it's the "wrong" order nobody intends to use.

**Phase to address:**
Spine curation CLI (structural) phase — the precondition-gate must be part of the initial
`spine-review` design, not a follow-up hardening pass, because retrofitting a gate onto an already
-shipped `purge` verb is itself a breaking change to a destructive command.

---

### Pitfall 2: The purge step has no tombstone stage — it goes straight to hard delete

**What goes wrong:**
Mature systems handling bulk destructive maintenance never go straight from "flagged for
cleanup" to "gone." The standard shape is tombstone-then-finalize: mark the record
(hidden from normal reads, still recoverable) and only irreversibly reclaim it in a later, separate,
explicitly-invoked step, often after a grace window. Bigtable, Cassandra, and CDC pipelines all use
this pattern for exactly this reason — a single-phase delete cannot be undone if the judgment that
triggered it (staleness, redundancy, "this is superseded") turns out to be wrong, and judgment
calls about memory content are exactly the class of decision most likely to be wrong at the
margins.

**Why it happens:**
`delete_memory` already exists as a direct, irreversible primitive (that's correct for its
existing use — deleting obvious junk a human explicitly asked to remove). It's tempting to wire
`spine-review purge` straight onto it because the primitive is already there and "it's just calling
the existing tool in a loop." That reuse is the trap: a *bulk, automated* sweep calling a
*single-record, human-intentional* primitive inherits none of the human's implicit judgment that
made direct deletion safe in the first place.

**How to avoid:**
This codebase already has the correct precedent twice over — reuse it rather than inventing a new
pattern:
- **`schedule_memory` + `prune-expired`** is already tombstone-then-finalize: `not_after` soft-hides
  a record from recall immediately, and only the separate, explicit `engram prune-expired` command
  hard-deletes it, typically after the window has been visibly expired for a while.
- **`supersede_memory`** is the same shape for corrections: the old record is soft-hidden
  (`superseded_by` set) but never deleted — it stays fetchable by id forever.
- `spine-review`'s purge-candidate step should **mark** records (a payload flag, or route through
  `schedule_memory`'s existing `not_after` soft-hide with a grace window) rather than calling
  `delete_memory` directly from the sweep. A separate, explicitly-confirmed `spine-review
  finalize`/`--commit` step performs the actual irreversible delete, requiring the operator (or the
  skill's consent gate) to act twice, at two different times, before data is gone.
- Add a confirmation threshold: any purge batch above N records (or any purge touching a `rule`-
  category record at all — rules cannot be superseded and can only be deleted, so they are the
  single highest-consequence purge target) requires an explicit `--yes`/count-echoing confirmation,
  not a silent bulk operation.

**Warning signs:**
- `spine-review purge` (or whatever the terminal verb is named) calling `store.Delete` or
  `delete_memory`'s handler directly, with no intermediate soft-hidden state.
- No `--dry-run` default or requirement — a destructive bulk command that runs for-real on first
  invocation, with dry-run as an opt-in rather than the command requiring an explicit
  `--commit`/`--yes` to go live.
- A test suite that verifies "purge deletes the record" but has no test verifying "a
  purge-candidate is still recoverable N hours/days after being marked, before finalize runs."

**Phase to address:**
Spine curation CLI (structural) phase — the tombstone/finalize split is core to the command's
shape, not an add-on; splitting it out later means every operator script written against the
single-step verb breaks.

---

### Pitfall 3: Partial-failure mid-sweep leaves the spine in an undocumented, unrecoverable middle state

**What goes wrong:**
A multi-record consolidation/purge sweep that dies partway (network blip to Qdrant, process
killed, embedder outage mid-extraction) can leave some records extracted-and-marked, some purged,
and some untouched — with no record of which stage each one reached. Re-running the sweep from
scratch is not obviously safe: does it re-extract (creating duplicate standalone records for
gotchas already extracted), re-purge (erroring or silently no-op'ing on already-gone records), or
skip everything (silently leaving the untouched remainder unprocessed forever)?

**Why it happens:**
Curation sweeps are naturally framed as "process this batch" scripts, and it's easy to write the
happy path (loop over records, extract, then purge) without separately handling "what if this
crashes at record 40 of 100." Non-idempotent sweeps are the default output of straightforward
imperative code; idempotency has to be designed in.

**How to avoid:**
- Design every `spine-review` verb to be safely re-runnable: re-running `extract` on an
  already-extracted record must be a no-op (detect via the state marker from Pitfall 1), not a
  duplicate write. Re-running `purge` on an already-purged/already-tombstoned record must be a
  no-op, not an error that halts the batch.
  - `idempotency_key` already exists on `store_memory`/`schedule_memory` for exactly this shape —
    prefer it for any record the skill's extraction step writes, rather than inventing a second
    duplicate-detection mechanism.
- Process and commit progress per-record (or in small batches with a persisted watermark), not as
  one large all-or-nothing transaction — Qdrant has no cross-record transaction to rely on here.
- After any sweep, `spine-review verify` should be able to detect and report an inconsistent
  middle state (record purged with no corresponding extraction marker; record marked
  purge-eligible but never actually removed) rather than silently reporting green.

**Warning signs:**
- A sweep command with no persisted progress marker — killing the process and re-running produces
  different results than letting it finish uninterrupted.
- `spine-review verify` (or the equivalent) only checks the *current* structural invariants
  (drifted citations, orphaned records) and has no check for "sweep started but never finished."
- Manual testing only ever exercises full successful runs; nobody has run the sweep, killed it at
  record N, and re-run it.

**Phase to address:**
Spine curation CLI (structural) phase, with the idempotency/re-run test as an explicit acceptance
criterion — this is exactly the class of defect that compiles, lints, and passes a happy-path
suite while being wrong.

---

### Pitfall 4: Agent-driven curation makes a confident, wrong, irreversible mutation with no consent checkpoint

**What goes wrong:**
An LLM judging "is this record still true" or "are these two the same fact" is not making a
retrieval decision (recoverable — just search again) — it is proposing a *write*, and a wrong
judgment that reaches the store as a direct action is the single most expensive failure mode in
this milestone. The 2025–2026 incident record for autonomous coding agents is now well-populated
with cases of an agent forming a confident, plausible-sounding theory and executing a destructive
action without any human in the loop to catch it — the common thread across every one of these
incidents is that the agent had *direct execution authority* over the destructive primitive, not
merely the ability to *suggest* it.

**Why it happens:**
It's natural to give a curation skill the same tool access as normal working sessions
(`update_memory`, `supersede_memory`, `delete_memory` all callable directly) because "the skill
already knows the right verb to use." The failure isn't the skill's judgment being wrong sometimes
— it's that the architecture gives a wrong judgment a direct path to an irreversible outcome with
no other party ever seeing the proposal before it executes.

**How to avoid:**
- **Propose-never-perform is the whole design, not a documentation note.** The skill must never
  itself call `supersede_memory`/`update_memory`/`delete_memory`/`store_rule` as its terminal
  action. It should emit a structured proposal (which records, which action, why) that a human (or
  the deterministic `spine-review` CLI, after human confirmation) executes. This mirrors the
  existing, already-proven `store_rule` consent gate in this codebase: "an agent proposes a rule
  candidate... `store_rule` is invoked only after the user blesses it (never promoted
  unilaterally)" — reuse that exact shape rather than inventing a new one.
- Batch proposals with a visible diff-like summary (what changes, what's lost) before any commit,
  the same way a destructive `git` operation shows a diff before an irreversible rebase/reset.
- Never let the skill's own output format be silently executable by tooling downstream (e.g., no
  "if the proposal JSON validates, auto-apply it" convenience path) — that reintroduces direct
  execution authority through a side door.
- Rules are the highest-consequence target: they are user-blessed ground truth and **cannot be
  superseded, only deleted**. The skill must never propose rule deletion with the same casualness
  as an ordinary memory correction — require a distinctly higher-friction confirmation for any
  rule-category proposal.

**Warning signs:**
- Any code or skill instruction path where the agent's own tool-call sequence includes a mutating
  memory verb as a *consequence* of its semantic judgment, rather than a proposal artifact that a
  separate step (human, or a deterministic re-invocation) turns into a tool call.
- A "confidence threshold" gating auto-apply (e.g., "if similarity > 0.95, merge automatically") —
  this reintroduces auto-extraction/auto-mutation by another name, which is the exact thing this
  project's design intent (explicit, zero-junk, no auto-extraction, ever) forbids.
- A demo/cold-read validation that only tests the agent proposing correctly, never tests the
  agent being *wrong* and confirms the wrong proposal still stops at consent (the v0.12.x rule-
  capture phase's own validation method — a cold-read agent — is the right template; it must be
  repeated here with an adversarial/wrong-judgment case, not just a correct one).

**Phase to address:**
Companion curation skill (semantic) phase — the consent architecture is the deliverable, and it
must be validated the same way v0.12.x validated rule capture: a cold-read agent test, including at
least one case where the "obviously right" answer is actually wrong, to prove the gate holds under
a confident bad proposal, not only a correct one.

---

### Pitfall 5: Semantic dedup/merge collapses two records that were not, in fact, the same fact

**What goes wrong:**
Merging two memories that are lexically or embedding-similar but semantically distinct (e.g., "we
decided X for repo A" and "we decided X for repo B," or a decision and its later, narrower
refinement) destroys information. Published guidance on this exact failure mode is consistent:
even a high cosine-similarity threshold (0.9+) can spuriously merge statements that differ along a
critical axis the embedding doesn't weight heavily (scope, sign/polarity, "still true" vs. "was
true then"), and no single scalar threshold is safe as a fully automatic gate.

**Why it happens:**
Vector similarity measures *surface* semantic closeness, not "these are the same fact for the
purposes of curation." Two decisions with nearly identical wording but opposite scope, or a
decision and its later refinement (which this repo already distinguishes structurally —
`supersede_memory` vs. `update_memory`), can sit above any similarity threshold that's loose
enough to catch true duplicates.

**How to avoid:**
- Treat similarity score as a **candidate filter for the semantic skill to examine**, never as a
  merge trigger on its own. The skill's job is exactly the judgment a threshold can't make; a
  threshold's job is only to narrow the candidate set worth showing a human/agent.
- Distinguish "is-the-same-fact" merge candidates from "is-a-correction-of" candidates explicitly
  — the former belongs nowhere in this system's vocabulary as a destructive merge (there's no
  `merge_memory` primitive and there shouldn't be one); the latter already has the correct primitive
  (`supersede_memory`, additive, recoverable). A "these are duplicates" proposal from the skill
  should map to *deletion of the redundant one after extraction of anything unique*, never to a
  new "combined" record replacing two others — a fabricated merged record is unverifiable against
  either original.
- Make any merge/redundancy proposal show both full records side by side in the proposal artifact
  — never a similarity score alone — so the human confirming it can see the axis a score would
  hide (scope, time, negation).

**Warning signs:**
- Any design that treats "cosine similarity above threshold N" as sufficient justification to
  purge one of a pair, with no distinct-content check.
- A `spine-review`/skill proposal format that shows a similarity score without showing both
  records' full content for comparison.
- No test case in the acceptance suite for "two records are similar but scoped to different
  repos/workspaces/time windows and must NOT be proposed as duplicates."

**Phase to address:**
Companion curation skill (semantic) phase, with the "is this the same fact" prompt design itself
being validated against adversarial near-duplicate-but-distinct pairs, not only true duplicates.

---

### Pitfall 6: False-positive staleness detection trains operators to stop trusting the verifier

**What goes wrong:**
An automated "this record looks stale/drifted" check that fires too often on records that are
actually still valid produces the same dynamic documented extensively in security-alert-fatigue
research: repeated investigation of alerts that turn out benign trains the operator to give every
alert — including the real ones — the same skeptical, cursory dismissal. Applied here: if
`spine-review`'s citation-drift or staleness detector cries wolf on `file:line` anchors that
merely moved (rather than becoming actually wrong), operators will start reflexively dismissing
its output, including the times it's caught something real.

**Why it happens:**
`file:line` citations drift on *every* edit to the cited file, including pure reformatting,
unrelated additions above the cited line, or renames that don't touch semantics at all. A naive
"does this line number still exist / does the line still contain roughly this text" check will
fire constantly in a fast-moving codebase, vastly out of proportion to genuinely broken citations.

**How to avoid:**
- Distinguish "line moved but content is unchanged/recognizable" (low severity, informational —
  auto-repairable by re-locating the anchor via a fuzzy/content match) from "the cited content is
  gone entirely" (high severity — the citation is actually broken and needs human attention).
  Never surface both at the same severity.
- Where the anchor can be mechanically re-resolved (same function/symbol, shifted line number),
  auto-repair it as part of the verify pass and report it as a fixed-not-flagged count, rather than
  asking a human to confirm a move that carries no information.
- Track and report the detector's own false-positive rate over time (how many flagged citations
  were, on operator review, actually fine) — treat a persistently high rate as a defect in the
  detector, not a fact about the spine.
- #355 (drifted `tools.go` citation anchors) is explicitly the fixture this verifier needs to get
  right — use it as the calibration case: does the shipped detector correctly classify it as
  "broken" without also flagging a large number of merely-moved, still-valid citations elsewhere
  in the same sweep?

**Warning signs:**
- A first real run of `spine-review verify` against the live spine that flags a large fraction of
  all citations as "drifted" — that's a detector-precision problem, not a spine-health finding, and
  shipping it as-is guarantees the next several runs get ignored.
- No distinction in the tool's output between "auto-repaired, FYI" and "needs your judgment."
- Operators (in dogfooding/UAT) skipping past `spine-review verify` output without reading it
  after the first run or two — that is the alert-fatigue signature and should block ship, not be
  noted as a UX nit.

**Phase to address:**
Spine curation CLI (structural) phase, using #355 as the acceptance fixture and requiring a
false-positive-rate check as part of that phase's verification, not deferred to post-ship
observation.

---

### Pitfall 7: A previously-accepted flag combination is now rejected — a silent breaking change

**What goes wrong:**
Adding `MarkFlagsMutuallyExclusive` (or equivalent hand-rolled validation) to enforce a constraint
that was previously only *documented* in help text (per #453: `client_list.go`'s
`--offset`/`--cursor-mode`/`--page-token` mutual exclusivity) changes the CLI's actual accepted
input surface. Any script, cron job, or agent invocation that was — deliberately or accidentally —
passing more than one of these flags together and silently getting *some* behavior (whichever the
code happened to prioritize) will now get a hard rejection where it previously ran to completion.
That is a breaking change to a shipped CLI's public contract (the CLI's flags are its API), even
though the change looks purely like "finally enforcing what was already documented."

**Why it happens:**
It's easy to treat "the help text already says these are mutually exclusive" as license to add the
enforcement freely — the validation *looks* like it's just catching a bug, not removing a
capability. But an unenforced "shouldn't combine these" is, in practice, a permissive
under-specified behavior, and any caller relying on that permissiveness (even accidentally, even
via a flag passed by a script that no longer needs it but never got cleaned up) breaks.

**How to avoid:**
- Audit real invocation patterns (scripts, CI, the operator console if any command wraps CLI calls,
  agent skill invocations) for the flag combination being newly forbidden *before* landing the
  validation — this is different from checking whether the constraint is documented; it's checking
  whether anyone actually violates it today.
- Where the newly-enforced combination could plausibly be in use, land the enforcement as a
  *warning* first (one release/minor version) before making it a hard error — the standard
  deprecate-then-remove shape (mark deprecated with a clear message for at least one release cycle,
  then remove/error) applies just as much to "flags now conflict" as to "flag now removed."
- Since this repo's CLI is the public API surface and the project treats CLI flags/exit codes as
  the interface (correct-by-reading, D-00), any test suite added for this must include a
  **negative-space regression test** proving the *old* accepted combination now correctly errors —
  not just a test that the *new* rejection message text is right. Silently accepting a
  previously-rejected-in-code-review-but-not-in-code combination is exactly the kind of thing that
  passes `go vet`/lint/happy-path tests.

**Warning signs:**
- A PR adding `MarkFlagsMutuallyExclusive` with no corresponding test asserting the *specific* old
  behavior (e.g., "when both `--offset` and `--cursor-mode` were passed together, the tool used to
  do X") is now gone and replaced by a rejection — if the old behavior was never pinned by a test,
  there's no way to know whether removing it breaks someone.
- No CHANGELOG/release-notes line calling out the new rejection as a **breaking change** distinct
  from ordinary bug fixes — release-please's conventional-commit categorization will bury this as a
  `fix:` unless it's deliberately marked (`!`/`BREAKING CHANGE:`) footer, which changes semver
  bump behavior too.

**Phase to address:**
Documented constraints made enforceable (#453) phase — the audit-before-enforce step belongs at
the start of this phase, not as an afterthought once the validation already exists.

---

### Pitfall 8: Exit-code changes look like cleanup but break every script branching on them

**What goes wrong:**
This repo already carries a live, deliberate split: client-facing verbs use a 0/2/4/5 taxonomy
(D-17) while six operator-command sites stay on exit 1 as a backward-compatibility carve-out
(D-09, tracked as #467 — "no surface states which taxonomy governs which command"). Any attempt to
"finish the job" and unify these during this milestone's audit is itself the highest-risk change
in the whole milestone: a cron job, a CI step, or an operator's own script that branches on
`engram migrate-remap-owner; if [ $? -eq 1 ]` (today's contract) silently does the wrong thing —
or nothing at all — the moment that command's failure exit code moves to match the client
taxonomy.

**Why it happens:**
Exit codes are invisible in code review relative to their blast radius — a one-line change from
`os.Exit(1)` to `os.Exit(2)` reads as trivial, and nothing in a Go type system or a passing test
suite for the command itself will show that some external caller's branch on the old value now
takes the wrong path. This is precisely the class this project has already been bitten by once
(the `*argError` switch-arm-order collapse, `667p88n2be`): a dispatch/mapping change that is
locally correct but globally wrong, verifiable only by checking every call site, not by unit-
testing the changed function alone.

**How to avoid:**
- Resolve #467 by **documenting the boundary explicitly** (which taxonomy governs which command,
  as a decision record) before changing any actual exit code — the milestone goal listed is "one
  exit-code taxonomy, **or a documented boundary**"; the documented-boundary option is strictly
  lower risk and should be the default unless there's a concrete, named consumer that benefits from
  unification enough to justify a breaking change.
- If unification is chosen anyway, treat it exactly like the flag-validation breaking change in
  Pitfall 7: one minor version with both codes supported/documented as deprecated-old-code, a
  CHANGELOG breaking-change entry, and a grep across this repo's own CI/Taskfile/scripts (plus any
  known operator automation, e.g., `prune-expired`/`reindex` invoked from a cron) for exit-code
  branches before flipping the default.
- Pin the **current** per-command exit code with a table-driven test enumerating every operator
  command's exit code for its known failure modes — this is the negative-space test that would
  have caught the switch-arm-order regression class earlier, and it's the only thing that makes
  "we changed exit codes" a visible, reviewable diff instead of an invisible one.

**Warning signs:**
- A code review of an exit-code change that only checks "does the new code make semantic sense,"
  never "what does this command currently return today, pinned by a test, that this diff changes."
- No table anywhere in the codebase or docs enumerating command → exit-code contract — if the
  audit phase produces one, that itself is evidence the gap existed; if it doesn't produce one,
  the boundary is still undocumented and the pitfall is unaddressed.
- Any exit-code dispatch implemented as a sequence of `if`/`switch` arms ordered by
  first-match-wins without an exhaustiveness check — the exact shape that silently collapsed all
  hint-code classes once already in this codebase.

**Phase to address:**
One exit-code taxonomy, or a documented boundary (#467) phase — must ship a decision record either
way, and if unification is chosen, must ship the pinned-current-behavior regression test in the
same change, not after.

---

### Pitfall 9: A verify/test-selection command that matches nothing exits 0 — a false green forever

**What goes wrong:**
Memory `bsbsvn4hbc` already documents this exact failure class in this codebase: `go test -run
<pattern>` matching zero tests exits 0, so a Nyquist `VALIDATION.md` row recording a `-run` command
as "passing" can be **permanently, silently wrong** if the pattern ever stops matching (a rename,
a moved test, a typo introduced during refactor) — nothing distinguishes "0 tests ran because
nothing needed testing" from "0 tests ran because the selector is broken." This is the single
highest-value class the downstream consumer flagged: it passes review, passes CI, and reports
success while verifying nothing.

**Why it happens:**
`go test`'s (and most test runners') exit code answers "did anything that ran, fail," not "did the
thing I intended to run, run." A selector is not part of the tool's contract in a way that gets
checked — it's free text that either matches or doesn't, with silence on the "doesn't" case.

**How to avoid:**
- Any `-run`/selector-based verification command recorded anywhere (a `VALIDATION.md` row, a CI
  step, a `spine-review verify` gate) must be re-resolved against `go test -list` (or the
  equivalent enumeration for the runner in question) at the time it's trusted, not merely
  re-executed and checked for exit 0. A stale selector matching nothing must fail loud.
  For **this milestone's own Nyquist reconciliation work**, that means every one of the six
  `status: draft` rows plus Phase 2's missing file must have its `-run` command actually re-checked
  against `go test -list -run <pattern>` (nonzero test count) as part of reconciling it, not just
  re-run.
- For any *new* verification tooling this milestone ships (`spine-review verify`, a citation-drift
  checker, a CLI/MCP-surface audit script), apply the same principle preemptively: if the checker
  can legitimately find "zero issues" as a true result, it must independently prove it *looked* at
  a nonzero number of records/commands/flags — report a scanned-count alongside the found-count, so
  "0 found because there's nothing to find" is distinguishable from "0 found because the scan
  matched nothing."
- Generalize past `go test`: the same shape applies to `grep`/`rg`-based checks used as acceptance
  gates in this milestone's own tooling — a pattern that stops matching (because of a rename) will
  silently report "clean" forever. Any such gate needs a scanned-count assertion, not just an
  exit-code check.

**Warning signs:**
- A CI/verification step whose only assertion is "exit code 0" with no assertion on how many
  things were checked.
- A `VALIDATION.md` (or new spine-review verify report) that has been "green" across multiple
  unrelated code changes without ever being re-derived from a fresh `-list`/enumeration — a
  suspiciously stable-forever green is itself the warning sign, not reassurance.
- Any acceptance criterion phrased as "the check passes" rather than "the check passes AND
  confirms it examined N ≥ 1 (or the expected N) targets."

**Phase to address:**
Nyquist validation reconciliation phase — this is the phase's entire reason for existing this
milestone, and its own deliverable must not repeat the mistake (a reconciled `VALIDATION.md` that
was never re-checked against a live `-list` is not actually reconciled, just re-stamped).

---

### Pitfall 10: The same constraint drifts silently across CLI help, self-describe catalog, and MCP tool schema

**What goes wrong:**
`effectiveSearchScope` and its sibling conditional-requirement rules must be stated in at least
three independent places: the `cmd/engram/*` flag help strings, the v0.12.x self-describe JSON
catalog, and the `internal/server` MCP tool/argument jsonschema docs. Each surface is hand-authored
prose (or hand-authored schema description text) with no single generator producing all three, so
a fix to the rule's *behavior* is trivially easy to land in code while updating only one of the
three descriptions — the other two silently keep describing the old, now-wrong behavior. Nothing
in CI checks that these three surfaces agree, because they're free text/description fields, not
generated code with a drift-detection build gate the way `gen/` already has for protobuf.

**Why it happens:**
This repo already has exactly the right template for solving this (`buf breaking`/CI drift-check
on the committed `gen/` tree, catching protobuf/codegen divergence) — but that machinery only
covers the wire-format layer, not free-text documentation describing conditional business rules.
Those conditional rules exist in code (e.g., a Go function implementing `effectiveSearchScope`) but
their *description* is duplicated by hand into JSON schema strings and CLI help strings, which is
the same "invented structure in a place a tool doesn't generate" hazard this project already
guards against for planning artifacts, just applied to documentation surfaces instead.

**How to avoid:**
- Where feasible, generate the description text for one conditional rule from a single source
  (a Go const/doc-comment referenced by both the MCP tool schema builder and the CLI help-string
  builder) rather than three independent hand-written sentences saying "the same thing."
- Where full generation isn't practical this milestone, add a **grep-based drift test** (with the
  Pitfall 9 caveat applied — assert a nonzero match count, not just "no failures") that checks the
  same named rule/flag appears with the same key phrase across all three surfaces, so a change to
  one without the others fails CI rather than shipping silently correct-in-code,
  wrong-in-three-places-of-documentation.
- Treat this audit's own output as the enumeration of every place a conditional rule is currently
  stated — the audit phase's deliverable should include, for each conditional rule found, the list
  of surfaces it must appear on, so a future change has a checklist rather than tribal knowledge.

**Warning signs:**
- A PR that changes `effectiveSearchScope`'s logic (or any similar conditional-requirement rule)
  touching only the Go implementation and one description string, with the diff never showing the
  other two surfaces.
- No test anywhere asserting cross-surface agreement — `task lint`/`task` passing is not evidence
  here, since free-text schema descriptions are not type-checked or drift-checked by existing
  tooling.
- The self-describe JSON catalog (v0.12.x Phase 2 D-15) and MCP tool schema jsonschema being
  generated/updated by different code paths with no shared constant.

**Phase to address:**
Self-evident surface audit phase — this is the audit's central finding-and-fix category, and its
deliverable should include the drift-detection test as a permanent regression gate, not just a
one-time fix of the divergences found during the audit.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|------------------|
| `spine-review purge` calls `delete_memory` directly instead of tombstone-then-finalize | Ships faster; reuses an existing primitive | Irreversible if the purge judgment (structural or semantic) is wrong; no grace window to catch mistakes | Never — this is the milestone's highest-blast-radius shortcut |
| Skill proposes AND performs the mutation in one agent turn (no separate confirm step) | Fewer round-trips, feels more "autonomous" | Reintroduces auto-extraction/auto-mutation by another name; violates the project's core "no auto-extraction, ever" invariant | Never |
| Adding `MarkFlagsMutuallyExclusive` without auditing existing scripts for the now-forbidden combination | Closes #453 quickly | Breaks any caller relying on the previously-permissive behavior, with no warning period | Only with a documented one-release deprecation warning first |
| Unifying exit codes without a pinned "current behavior" regression test | Removes the two-taxonomy carve-out cleanly | Breaks any script/cron branching on today's exit 1 for operator commands, invisibly | Only if #467 is resolved by documenting the boundary instead, or if unification ships with the pinned test + a CHANGELOG breaking-change entry |
| Re-stamping a `VALIDATION.md` row as reconciled without re-running `-list` against the current test names | Closes the Nyquist tracking item fast | Reintroduces the exact false-green-forever failure the reconciliation phase exists to fix | Never |
| Similarity-threshold auto-merge without a human/skill judgment step | Removes an interaction round-trip | False-merge rate is unacceptable at any threshold below ~1.0 on records that differ by scope/time/negation; data loss is silent and permanent | Never |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|--------------|-----------------|-------------------|
| `spine-review` CLI ↔ curation skill | Treating the CLI's structural purge as independent of the skill's semantic extraction, letting either run without the other having run first | Gate `purge`/`archive` on a recorded extraction-pass marker the CLI itself checks (Pitfall 1) |
| Curation skill ↔ mutating memory tools | Skill calls `supersede_memory`/`delete_memory`/`update_memory` directly as its own terminal action | Skill emits a proposal artifact only; a separate confirmed step (human or CLI re-invocation) performs the mutation (Pitfall 4) — mirrors the existing `store_rule` consent gate |
| `spine-review` purge ↔ `rule` category | Treating rule-category purge candidates with the same friction as ordinary memory corrections | Require materially higher-friction confirmation for any rule-category proposal — rules cannot be superseded, only deleted, so a wrong rule-purge is maximally destructive |
| CLI help text ↔ cobra flag validation | Documenting a mutual-exclusivity constraint in `Short`/help text without ever enforcing it, then enforcing it later as if it were purely a bug fix | Audit real invocation patterns for the newly-forbidden combination before enforcing; treat enforcement as a breaking change with a deprecation window if any caller might rely on the old permissiveness |
| Operator-command exit codes ↔ any external script/cron/CI branching on them | Changing an exit code because it "should" match the client taxonomy, without checking existing consumers | Pin current behavior with a table-driven test before touching it; prefer documenting the boundary (#467's explicit "or documented boundary" option) over unifying without evidence of a benefiting consumer |
| CLI help / self-describe catalog / MCP tool jsonschema | Hand-editing one surface's description of a conditional rule and assuming the others are consistent because they were consistent at last check | Single source of truth for the rule's description where feasible; otherwise a cross-surface grep-based drift test with a nonzero-match assertion |
| `VALIDATION.md` `-run` commands ↔ actual test names | Treating a `-run` pattern recorded at plan time as still valid because the row says "passing" | Re-resolve every recorded selector against `go test -list` before trusting or reconciling the row (Pitfall 9) |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|-----------------|
| Citation-drift verification scans every `file:line` anchor in the spine on every `spine-review verify` invocation, re-reading files from disk each time | `verify` gets slower every time the codebase grows, eventually timing out or making the tool unpleasant to run regularly (which itself causes it to be skipped — feeding Pitfall 6) | Cache file content/line hashes keyed by commit SHA; only re-check anchors whose target file changed since the last verified commit | Once the spine has enough citation-bearing records that a full-file re-read per anchor is seconds-to-minutes rather than sub-second |
| Semantic-dedup candidate generation does an all-pairs similarity comparison across the whole spine | Fine at hundreds of records, quadratic blowup once the spine is large enough that a full curation pass becomes impractically slow to run | Use Qdrant's own ANN search per-record (top-k similar) rather than a manual all-pairs loop; this is already the store's core capability | Once spine size is large enough that O(n²) all-pairs stops being "a few seconds" |
| Purge/consolidate sweep does one record at a time with a network round-trip per record and no batching | A large sweep takes disproportionately long, increasing the odds of the partial-failure scenario in Pitfall 3 | Batch reads/writes where Qdrant's client supports it; checkpoint progress per-batch, not per-record, but keep the idempotency guarantee from Pitfall 3 | Sweeps touching more than a few hundred records at once |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Curation skill given direct tool access to mutating verbs, reasoning "it's a trusted first-party skill, not a random prompt" | A confident-but-wrong semantic judgment (or a prompt-injected instruction embedded in a memory's own content, which the skill will read while judging it) executes an irreversible mutation with no human check | Propose-never-perform, no exceptions, including for "obviously correct" cases — the incident record shows this exact "obviously right" confidence preceding irreversible mistakes repeatedly |
| Purge sweep treats `shared`-visibility records the same as private ones for a single caller's judgment | An actor curating their own spine could propose deleting a `shared` record another actor depends on, since shared grants read but never write — a curation proposal touching a record the proposer doesn't own must still route through the existing ownership write-gate, never bypass it because "it's just a cleanup" | Route every spine-review mutation (even proposed-then-confirmed ones) through the existing `getWritable`/ownership gate unchanged — curation tooling gets no special authz bypass |
| Bulk purge command accepts a broad selector (e.g., a scope glob) with no echo of exactly what will be affected before committing | A slightly-too-broad selector silently deletes more than intended, with no way to know until it's too late | Any bulk destructive command must print (or require `--dry-run` to have been run and reviewed for) the exact record count and a sample before requiring a second explicit confirmation to commit |
| Exit-code or flag-validation changes shipped without a security-relevant regression check for the existing 404-uniform-not-found invariant (DEC-xa6) | A CLI/MCP surface audit touching validation ordering could accidentally reorder a check so an authz-driven not-found and a plain validation error become distinguishable, reopening a cross-actor existence leak the store layer was designed to prevent | Any change to error/exit dispatch ordering in this milestone must be checked against the existing DEC-xa6 negative tests, not just the new validation behavior being added |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| Over-eager staleness/drift flags on every `spine-review verify` run | Operators stop reading the output after the first few runs (alert fatigue), missing the real findings when they eventually appear | Severity-tier findings; auto-repair the mechanically-fixable class (moved-but-recognizable anchors); report a scanned-count so "clean" is trustworthy |
| Destructive commands with no `--dry-run` default and a single-step commit | An operator (or a script written by an operator under time pressure) runs the real thing on first try, with no preview | Dry-run by default, or require an explicit second flag/confirmation naming the exact record count before any live commit |
| A curation-skill proposal that just says "these look like duplicates, delete one" with no visible reasoning or side-by-side content | Operator can't actually evaluate whether the proposal is right, so either rubber-stamps it (false confidence) or ignores it (skill goes unused) | Show both full records, the reasoning, and what's uniquely present in each before asking for confirmation |
| Two different exit-code taxonomies with no documentation of which governs which command | Script authors guess, get it wrong half the time, and file bugs against engram instead of realizing the split is deliberate | Ship the documented boundary (or the unification) as visible, discoverable documentation — not just an internal decision record — including in `--help` output itself |
| Flag-validation errors that reject a combination but don't say why it's forbidden or what to do instead | Operator trial-and-errors their way to a working invocation instead of reading help correctly the first time (violates this project's own correct-by-reading principle, D-00) | Every new validation rejection must state the constraint in the error text, matching whatever the help text already says, so the two are provably the same sentence |

## "Looks Done But Isn't" Checklist

- [ ] **`spine-review purge`:** Often missing the tombstone/grace-window stage — verify a purge
      candidate is still recoverable for some window before the terminal, irreversible step runs,
      not deleted on first invocation.
- [ ] **Extraction-before-delete ordering:** Often "documented" as a rule (like `7smp8vy9hr`) but
      not mechanically enforced — verify `purge` structurally refuses to run without a recorded
      extraction pass, not merely that the operator was told to run extraction first.
- [ ] **Curation skill consent gate:** Often demonstrated only on a correct-proposal happy path —
      verify it was cold-read-tested with at least one deliberately-wrong "obviously right" semantic
      judgment and confirmed the gate still stopped it before any mutation.
- [ ] **Flag-validation enforcement (#453):** Often shipped without checking whether any existing
      script/skill invocation relies on the newly-forbidden combination — verify a grep/audit of
      known invocation sites (CI, Taskfile, docs examples, skill instructions) was actually done,
      not assumed clean because it "was already documented as forbidden."
- [ ] **Exit-code taxonomy resolution (#467):** Often "resolved" by picking a taxonomy without
      checking what currently consumes the other one — verify either a documented-boundary decision
      record exists, or a pinned-current-behavior regression test shipped alongside any unification.
- [ ] **Nyquist `VALIDATION.md` reconciliation:** Often re-stamped as reconciled by re-running the
      recorded `-run` command and seeing exit 0 — verify each row was re-resolved against
      `go test -list` and shows a nonzero, expected test count, not just a clean exit code.
- [ ] **Multi-surface documentation (CLI help / self-describe catalog / MCP schema):** Often fixed
      in the surface that was reported broken, with the other two assumed still correct — verify all
      three were checked for the same conditional rule, with a grep-based drift test added, not just
      a one-time fix.
- [ ] **New verification/audit tooling's own false-negative mode:** Often validated only by
      confirming it finds known-planted issues — verify it also reports how much it scanned, so a
      selector/pattern silently matching nothing is distinguishable from genuinely finding zero
      issues.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|----------------|------------------|
| Delete-before-extract already happened (records purged, gotchas lost) | HIGH | If Qdrant snapshots/backups exist, restore from the most recent pre-purge snapshot to recover the deleted records' content before re-attempting extraction; if no backup exists, the gotchas are unrecoverable — this is exactly why Pitfall 1's precondition gate must ship before any purge capability, not after |
| Wrong semantic merge/delete proposal was confirmed and executed | MEDIUM–HIGH | If the purge routed through the tombstone/grace-window stage (Pitfall 2), the record is still recoverable via `get_memory` until the grace window's finalize step runs — recover it there; if it went straight to `delete_memory`, recovery depends entirely on external backups |
| Exit-code change broke a script silently | LOW–MEDIUM | Revert the exit-code change as a patch release; add the pinned-current-behavior regression test that should have existed before the change shipped, then re-attempt the change with a documented deprecation window |
| Flag-validation change rejected a previously-working invocation | LOW | Revert to a deprecation-warning state (accept the combination with a loud warning) for one release cycle, then re-attempt hard rejection with advance notice |
| A `VALIDATION.md` row was falsely green (selector matched nothing) | LOW | Re-resolve the selector against `go test -list`, write the correct pattern, re-run, and re-verify the referenced success criterion is actually exercised — cheap once caught, expensive only in the false confidence it produced beforehand |
| Multi-surface documentation drifted (CLI help says one thing, MCP schema says another) | LOW | Reconcile to a single source description and add the cross-surface drift test so it can't recur silently |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|-------------------|----------------|
| Delete-before-extract ordering unenforced | Spine curation CLI (structural) | Test: `purge` on a target with no prior extraction marker refuses to run |
| No tombstone stage before hard delete | Spine curation CLI (structural) | Test: a purge-candidate record is still fetchable via `get_memory` for the grace window before `finalize`/`--commit` runs |
| Partial-failure mid-sweep leaves inconsistent state | Spine curation CLI (structural) | Test: kill the sweep process mid-batch, re-run, and assert identical end state to an uninterrupted run (idempotency) |
| Agent-driven curation executes a wrong proposal directly | Companion curation skill (semantic) | Cold-read test with a deliberately-wrong "obviously right" judgment; assert zero mutating tool calls without a separate confirmed step |
| Semantic dedup false-merges distinct records | Companion curation skill (semantic) | Adversarial test set: near-duplicate-but-scope/time-distinct pairs must never be proposed as merge candidates |
| False-positive staleness/drift detection | Spine curation CLI (structural) | #355 used as the calibration fixture; false-positive rate measured across a real sweep, not just the planted case |
| Previously-accepted flag combination now rejected | Documented constraints made enforceable (#453) | Audit of existing invocation sites for the newly-forbidden combination, completed and recorded before landing the validation |
| Exit-code change breaks scripts | One exit-code taxonomy, or a documented boundary (#467) | Either a documented-boundary decision record, or a pinned-current-behavior table-driven regression test shipped with any unification |
| Test-selector false green (matches nothing, exits 0) | Nyquist validation reconciliation | Every reconciled row's `-run` pattern re-resolved against `go test -list` with a nonzero, expected match count |
| Multi-surface documentation drift (CLI/catalog/MCP schema) | Self-evident surface audit | Cross-surface grep-based drift test with a nonzero-match assertion added as a permanent CI gate |

## Sources

- [Tombstone (data store) — Grokipedia](https://grokipedia.com/page/Tombstone_(data_store)) — HIGH confidence, cross-checked against Bigtable/Cassandra tombstone-then-compaction precedent
- [Tombstone Design Pattern — James's Knowledge Graph](https://www.jamestharpe.com/tombstone-pattern/) — MEDIUM confidence, corroborating pattern description
- [CDC Soft Deletes and Tombstones — Streamkap](https://streamkap.com/resources-and-guides/cdc-soft-deletes-tombstones) — MEDIUM confidence
- [How would you implement soft vs hard TTL for GDPR deletion? — DesignGurus](https://www.designgurus.io/answers/detail/how-would-you-implement-soft-vs-hard-ttl-for-gdpr-deletion) — MEDIUM confidence, "tombstone then finalize" phrasing
- [An AI Agent Deleted a Company's Entire Production Database — Then Lied About It — DEV Community](https://dev.to/arbabyousaf/an-ai-agent-deleted-a-companys-entire-production-database-then-lied-about-it-49mh) — HIGH confidence, widely corroborated incident (Replit, July 2025)
- [Incident 1152 — AI Incident Database](https://incidentdatabase.ai/cite/1152/) — HIGH confidence, independently curated incident record
- [When an Agent Deletes the Production Database — O'Reilly Radar](https://www.oreilly.com/radar/when-an-agent-deletes-the-production-database/) — HIGH confidence, editorial analysis of the failure pattern
- [When AI Agents Delete Production: Lessons from Amazon's Kiro Incident — Particula](https://particula.tech/blog/ai-agent-production-safety-kiro-incident) — MEDIUM confidence, second independent incident (Dec 2025)
- [AI Agent Deleted a Production Database, The Real Failure Was Access Control — Penligent](https://www.penligent.ai/hackinglabs/ai-agent-deleted-a-production-database-the-real-failure-was-access-control/) — MEDIUM confidence, corroborates "direct execution authority is the root cause" framing
- [Semantic Deduplication — NVIDIA NeMo Framework User Guide](https://docs.nvidia.com/nemo-framework/user-guide/25.07/datacuration/semdedup.html) — HIGH confidence, official framework docs on cosine-threshold dedup mechanics
- [Beyond MD5: transformer-based fuzzy deduplication — Medium](https://medium.com/@banavalikar/beyond-md5-implementing-transformer-based-fuzzy-deduplication-for-unstructured-datasets-at-scale-6ebff328da98) — MEDIUM confidence, threshold examples (0.85/0.95)
- [Modeling Clinical Uncertainty in Radiology Reports — arXiv](https://arxiv.org/pdf/2511.04506) — MEDIUM confidence, explicit false-merge-at-high-threshold finding in a domain with similarly high correction stakes
- [cobra package — pkg.go.dev](https://pkg.go.dev/github.com/spf13/cobra) — HIGH confidence, official reference for `MarkFlagsMutuallyExclusive`
- [MarkFlagsMutuallyExclusive does not work with default values — cobra#1752](https://github.com/spf13/cobra/issues/1752) — HIGH confidence, primary-source known limitation directly relevant to #453
- [Working with Flags — Cobra docs](https://cobra.dev/docs/how-to-guides/working-with-flags/) — HIGH confidence, official deprecation-pattern (`MarkDeprecated`) documentation
- [Handling Breaking API Changes — cetra3.github.io](https://cetra3.github.io/blog/breaking-api-changes/) — MEDIUM confidence, "CLI flags/exit codes are the public API" framing
- [Understanding and fighting alert fatigue — Atlassian](https://www.atlassian.com/incident-management/on-call/alert-fatigue) — HIGH confidence, well-established operational-practice source
- [The Analyst Who Cried Malware — CardinalOps](https://cardinalops.com/blog/rethinking-false-positives-alert-fatigue/) — MEDIUM confidence, corroborates the "trains reviewers to distrust severity" mechanism
- [Contract-First APIs: How OpenAPI Becomes Your Single Source of Truth — HackerNoon](https://hackernoon.com/contract-first-apis-how-openapi-becomes-your-single-source-of-truth) — MEDIUM confidence, generalizable single-source-of-truth pattern for multi-surface doc drift
- Repo-internal precedent (`.planning/PROJECT.md`, `CLAUDE.md`, cited memory IDs `7smp8vy9hr`, `bsbsvn4hbc`, `667p88n2be`, `4aksmneehh`, decisions DEC-xa6/DEC-kyz/DEC-iedk/D-09/D-17) — HIGH confidence, primary source

---
*Pitfalls research for: curation/maintenance tooling + interface audit on an existing shipped memory system (engram v0.13.x)*
*Researched: 2026-08-03*
