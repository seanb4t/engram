// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// execTimeout bounds every single Environment.Run call this package makes
// (D-12). It is a FIXED INTERNAL CONSTANT, deliberately not a --timeout
// flag: setupCmd is currently the only destructive command without one
// (migrate, prune-expired, reindex, spine-review purge, backfill-short-ids
// and migrate-remap-owner all carry a DurationVar publishing the same "max
// wall-clock" sentence, cmd/engram/destructive_test.go:419-433). This is a
// deliberate, reasoned divergence from that repo idiom, not an oversight:
// setup already carries six flags, and every runtime's `mcp add`/`mcp get`
// surface completed in under 2 seconds during 03-RESEARCH.md's live
// probing, so the tunability the sweep-style siblings need is not yet
// earned here. Promoting this to a flag later is purely additive.
const execTimeout = 20 * time.Second

// maxCapturedBytes bounds any third-party stdout/stderr capture placed on
// a rendered Result field (Reason, Notes, Registered) — the mitigation for
// the "third-party stdout becomes report content" threat 03-CONTEXT.md
// names: sanitizeViewValue (cmd/engram/operator_view.go) strips only C0
// controls and DEL, so a runtime that floods stdout could otherwise flood
// the operator's terminal or the --output json lane. Applied ONLY at the
// point a captured string is assigned to a rendered field — never to the
// values compared for D-08's convergence check, which must see the RAW,
// untruncated bytes (see execute's own comment on this point).
const maxCapturedBytes = 4096

// truncationMarker is appended by boundCapture when a capture is cut.
const truncationMarker = "...[truncated]"

// tokenFileIgnoredMarker is the fixed, engram-authored value Result.TokenFile
// carries when --token-file was supplied for a runtime whose write action
// EXECUTES something rather than merely emitting a config document (D-06,
// D-07): the child process only writes config — it never uses the
// credential, which the runtime resolves itself from its own environment
// at connect time — so a --token-file path has nothing to do there. This
// converts a silently inert flag into a legible one (memory zcev96ng18's
// fails-by-absence shape, in reverse). Deliberately never carries the
// supplied path: bearerProvenance already renders the path where it means
// something (generic.go's fallback form), and duplicating it into this
// field would be noise (T-03-26).
const tokenFileIgnoredMarker = "ignored"

// No pre-flight probe of any runtime's `--help` output and no
// version-floor check exist anywhere in this package (D-11), by design.
// Both were rejected in 03-CONTEXT.md/03-RESEARCH.md: a pre-flight
// `--help` scrape would assert a third-party surface, which repo rule
// m45p2b4bp7 forbids (matching tokens in help text is scraping a format
// that drifts just as easily as the flag surface it claims to protect
// against); a version floor gates the proxy (the version string) rather
// than the property (whether the flag surface still works) — memory
// r7n0nejp9f measured codex's flag surface HOLDING across two minor-version
// bumps in six days, which is exactly why a version gate would false-fail
// on drift that does not matter and cannot catch a flag removed inside an
// allowed range. CLI-surface drift is instead detected and reported POST
// HOC ONLY: a nonzero exit from either the probe or the write becomes an
// OutcomeFailed row via describeFailure/describeSeamError below — engram
// reports what it tried and what the runtime said back; it does not itself
// distinguish "unknown flag" from "network error", and does not need to.

// boundCapture truncates s to at most maxCapturedBytes bytes, scanning
// back to the last valid rune boundary so the result is always valid
// UTF-8, then appends truncationMarker. A string already within budget is
// returned unchanged (byte-identical, no marker) — most captures never
// truncate at all.
func boundCapture(s string) string {
	if len(s) <= maxCapturedBytes {
		return s
	}
	limit := maxCapturedBytes
	for limit > 0 && !utf8.RuneStart(s[limit]) {
		limit--
	}
	return s[:limit] + truncationMarker
}

// displayCapture bounds s to the capture budget, then renders it as one
// quoted shell word.
func displayCapture(s string) string {
	return quoteWord(boundCapture(s))
}

// Preview runs rt's read-only detection and planning against env and opts,
// then — for a present runtime with a Plan.Probe wired — runs that probe
// once and compares its output against opts (Phase 4, D-01): the operator
// sees present state next to intended state before anything is written.
//
// A single read is dishonest evidence for "did this change between two
// points in time" (D-08's basis, which the mutate branch below keeps
// unchanged) but honest evidence for "does this match what I ALREADY KNOW
// I would write" (Preview's basis, as of Phase 4) — a distinction this
// comment states explicitly so a future reader does not "fix" the two
// bases back together (04-RESEARCH.md Pitfall 1). For a runtime
// implementing DriftRuntime, drift.go's Compare classifies the observed
// registration as already-correct, would-write (naming which facet(s)
// differ), or preserved (an observed facet the current Options do not
// account for — a write would destroy something setup cannot re-create,
// D-01) — and Result.Registered is REBUILT from the parsed-and-redacted
// observation (D-03), never the raw probe capture. A runtime with no
// scanner (opencode, generic — D-10), a probe seam error, or output the
// scanner cannot frame as a registration at all resolves to
// OutcomeWouldWrite with no facets (D-09: ambiguity of every kind never
// becomes already-correct or preserved) — this is the SAME degradation
// Preview always performed before Phase 4, now scoped precisely to the
// cases a real comparison cannot honestly resolve. No probe result of ANY
// kind ever produces a nonzero process exit code; only a
// usage/configuration error does that. This re-pins T-02-08's exit-code
// property on behavioral grounds now that its original structural
// argument ("setupPreview starts no process") no longer holds — see
// cmd/engram/setup_test.go's TestSetupPreviewExitsZeroWhenProbeFails.
func Preview(ctx context.Context, env Environment, rt Runtime, opts Options) Result {
	return execute(ctx, env, rt, opts, false)
}

// Apply performs rt's registration against env and opts: resolve the
// binary, read current state (Plan.Probe, read #1), and — for a runtime
// implementing DriftRuntime whose read #1 can be framed as a
// registration — consult the SAME classification Preview would report
// BEFORE running any write Action (Phase 5, D-01): already-correct and
// preserved issue ZERO registration-write actions (Claude Code's
// tolerant `mcp remove` included) and return immediately; only
// would-write runs Plan.Actions. After a real write, the executor
// re-probes once and rebuilds Registered through the same
// Observe -> renderObservation path Preview uses (D-02), redaction-safe;
// Outcome stays wrote regardless of what that re-observe says. A runtime
// with no scanner (opencode), or a read #1 that could not be framed at
// all, keeps the ORIGINAL two-read byte compare this function always
// used (D-09/D-10: ambiguity never skips a write). One shared,
// package-level executor (D-03) — no per-runtime knowledge lives here;
// every runtime-specific string was already authored in that runtime's
// own Plan()/Observe() (D-09, AUTHORED-HERE).
func Apply(ctx context.Context, env Environment, rt Runtime, opts Options) Result {
	return execute(ctx, env, rt, opts, true)
}

// runSeam bounds one Environment.Run call to execTimeout (D-12) via a
// fresh context.WithTimeout per exec, per the deliberate per-exec (not
// per-runtime, not per-Apply-call) scoping this package's own doc comments
// commit to. It is also the single place every Environment.Run is bounded,
// so it is the single place a deadline is given its operator-facing name
// (D-11): a context.DeadlineExceeded returned by env.Run (in production,
// osRun's own ctx.Err() case) is wrapped as "timed out after 20s: context
// deadline exceeded" with %w (errors.Is still resolves through the wrap),
// so the row renders as "<runtime>: <argv>: timed out after 20s: context
// deadline exceeded" via the unchanged describeSeamError.
// A plain cancellation (Ctrl-C / a caller's own cancel, the Canceled
// sentinel from the standard library's context package) is deliberately
// left unwrapped — it is not a timeout, and must never be described as
// one.
func runSeam(ctx context.Context, env Environment, path string, args []string) (RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	rr, err := env.Run(ctx, path, args)
	if err != nil && errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("timed out after %s: %w", execTimeout, err)
	}
	return rr, err
}

// describeFailure builds a D-11 failure Reason from four components in a
// FIXED ORDER: the runtime's Name(), the failing action's rendered argv
// display (cmdDisplay — Action.Command() or quoteArgs(Plan.Probe)), the
// numeric exit code, and the bounded captured stderr. The exit code is
// what keeps an empty-stderr failure legible: it is always present, and
// the stderr segment is appended only when non-empty — a runtime that
// fails silently still produces a Reason an operator can act on. Nothing
// here branches on stderr's CONTENT (D-11: engram reports, it does not
// diagnose) — stderr is carried as data, appended verbatim (bounded).
func describeFailure(name, cmdDisplay string, exitCode int, stderr string) string {
	reason := fmt.Sprintf("%s: %s exited %d", name, cmdDisplay, exitCode)
	if stderr != "" {
		reason += ": " + displayCapture(stderr)
	}
	return reason
}

// describeSeamError builds a D-11 failure Reason for an Environment.Run
// SEAM error — a process that never produced an exit status at all (start
// failure, or ctx's deadline expired). There is no exit code in this case;
// the seam's own error text supplies the "what went wrong" component
// describeFailure's fourth component otherwise carries.
func describeSeamError(name, cmdDisplay string, err error) string {
	return fmt.Sprintf("%s: %s: %v", name, cmdDisplay, err)
}

// classification holds the outcome of classifyProbe: probe1's parsed
// comparison against opts, computed ONCE from a single probe1 read and
// shared by both the preview (!mutate) lane and the apply (mutate)
// lane's pre-action gate (Phase 5, D-01) — never a second parse path
// (04-RESEARCH.md Pitfall 3). compared is false for every ambiguous case
// (no probe wired, no DriftRuntime, a probe1 seam error, or an Observe
// that could not frame probe1 as a registration at all) — D-09's
// ambiguity invariant, restated as a struct field so both lanes consult
// the identical classification rather than re-deriving it. notCompared
// is the fixed "not compared" note for that case; dr/obs/drift are only
// meaningful when compared is true.
type classification struct {
	compared    bool
	notCompared string
	dr          DriftRuntime
	obs         Observation
	drift       Drift
}

// classifyProbe runs the SAME DriftRuntime type-assertion / hasProbe /
// probe1Err / dr.Observe / Compare sequence the !mutate branch has run
// since Phase 4 — factored out so the apply (mutate) lane's own
// pre-action gate (D-01) consults the IDENTICAL classification, never a
// second parse path. probe1/probe1Err/hasProbe are the caller's own
// already-made probe1 read (execute's shared prefix); this function
// re-derives nothing and makes no Run call of its own (Pitfall 1: the
// classification is always fresh within ONE execute() call, never
// carried across process invocations). Every "not compared" note string
// is byte-identical to what the !mutate branch produced before this
// move.
func classifyProbe(name string, rt Runtime, plan Plan, hasProbe bool, probe1 RunResult, probe1Err error, opts Options) classification {
	dr, isDrift := rt.(DriftRuntime)
	if !hasProbe || !isDrift {
		// D-10: a runtime with no probe wired, or no drift-capable
		// scanner (opencode, generic), is never compared.
		return classification{notCompared: notComparedNote(name, "runtime authors no registration scanner")}
	}
	if probe1Err != nil {
		// D-09: a probe seam error is unreadable, never preserved or
		// already-correct.
		return classification{notCompared: notComparedNote(name, quoteArgs(plan.Probe)+": "+probe1Err.Error())}
	}
	obs, ok := dr.Observe(probe1.Stdout+probe1.Stderr, opts)
	if !ok {
		// D-09: the scanner could not frame this output as a
		// registration at all — NEVER render probe1's raw bytes here.
		return classification{notCompared: notComparedNote(name, fmt.Sprintf("%s exited %d: output not recognized as a registration", quoteArgs(plan.Probe), probe1.ExitCode))}
	}
	return classification{compared: true, dr: dr, obs: obs, drift: Compare(obs, opts)}
}

// renderClassification writes the five classified fields onto res from a
// COMPARED c (callers must only invoke this when c.compared is true) —
// the same rendering both lanes have always performed, only shared now:
// Outcome/Facets/Drift from c.drift, Registered REBUILT from
// renderObservation(c.obs) (D-03, never raw probe bytes), and — on
// OutcomePreserved — a Reason composed as "<name>: preserved: <causes
// joined by '; '>; <WholeEntryNote>", with D-05's ManualRemediation
// appended after WholeEntryNote (", "; "+ManualRemediation") when the
// observing runtime authored one. WR-01: boundCapture is applied to
// Drift/Registered/Reason AFTER this composition, never in place of it —
// an observed header name/URL/label carries no length cap of its own.
func renderClassification(res *Result, name string, c classification) {
	res.Outcome = c.drift.Outcome
	res.Facets = joinFacets(c.drift.Facets)
	res.Drift = strings.Join(c.drift.Details, "; ")
	res.Registered = renderObservation(c.obs)
	if c.drift.Outcome == OutcomePreserved {
		res.Reason = name + ": preserved: " + strings.Join(c.drift.Preserved, "; ") + "; " + c.obs.WholeEntryNote
		if c.obs.ManualRemediation != "" {
			res.Reason += "; " + c.obs.ManualRemediation
		}
	}
	res.Drift = boundCapture(res.Drift)
	res.Registered = boundCapture(res.Registered)
	res.Reason = boundCapture(res.Reason)
}

// toleratedNote builds one Result.Notes entry for a TOLERATED nonzero
// exit: the action's own Description (when authored — e.g.
// claudecode.go's claudeCodeRemoveAction) is prefixed onto the rendered
// argv, exit code, and bounded captured stderr, so a genuinely broken
// tolerant step — and any consequence its Description records for a
// following action's failure — stays legible even though it never fails
// the row.
func toleratedNote(action Action, exitCode int, stderr string) string {
	if action.Description == "" {
		return fmt.Sprintf("%s exited %d: %s", action.Command(), exitCode, displayCapture(stderr))
	}
	return fmt.Sprintf("%s: %s exited %d: %s", action.Description, action.Command(), exitCode, displayCapture(stderr))
}

// execute is the shared sequencing core both Preview and Apply delegate
// to. Sequence:
//  1. rt.Detect(env) false -> OutcomeNotPresent, no exec of any kind.
//  2. rt.Plan(env, opts) error -> OutcomeFailed, Reason = err.Error().
//     2a. plan.Actions is empty -> OutcomeWouldWrite immediately, in BOTH
//     the preview and --apply lane (mutate is not consulted at all): no
//     env.LookPath call, no env.Run call, no probe of any kind. This is
//     D-16's general rule for a runtime whose entire deliverable is
//     Plan.Config rather than a write action — the generic pseudo-runtime
//     (generic.go) today, and any future runtime shaped the same way —
//     applied uniformly here rather than special-cased on any runtime's
//     name. OutcomeAlreadyCorrect is a claim about OBSERVED existing
//     state (plan.go); a runtime with nothing to write observes nothing,
//     so it can never reach that value, and OutcomeWouldWrite already
//     counts as a non-failed attempt under Classify (exit.go) — nothing
//     about that exhaustive combination table moves.
//  3. Resolve Args[0] of the FIRST action via env.LookPath and record it
//     as Result.Binary (D-04) — every subsequent exec in this runtime's
//     sequence reuses that SAME resolved path, closing the TOCTOU window
//     between Detect and the write.
//  4. Run Plan.Probe (read #1) if this runtime has one wired; keep the
//     RAW combined captures in local variables, never bounded. Then
//     classifyProbe computes ONE shared classification (Phase 5, D-01)
//     from this SAME read #1 — type-asserting DriftRuntime exactly once
//     (the PluginRuntime idiom) — consulted by BOTH lanes below. No probe
//     wired, no DriftRuntime, a probe seam error, or an Observe that
//     reports it could not frame the output as a registration at all
//     (D-09) leaves the classification "not compared". Either way the
//     process exit code is unaffected: no probe result of any kind
//     produces a nonzero exit (T-02-08).
//  5. mutate == false (Preview): Outcome defaults to OutcomeWouldWrite. A
//     "not compared" classification sets Result.Drift to that note
//     (quoting no probe bytes) and returns — Result.Registered stays
//     empty. A compared classification renders Outcome/Facets/Drift/
//     Registered/Reason from it (renderClassification) — Registered is
//     REBUILT from the parsed-and-redacted observation (D-03), never the
//     raw probe capture.
//  6. A probe SEAM error (start failure/timeout) under Apply is
//     OutcomeFailed, checked before the classification is consulted.
//  7. mutate == true (Apply, Phase 5 D-01): a COMPARED classification of
//     already-correct or preserved renders the same five fields Preview
//     would (Facets/Drift/Registered describe what the pre-write read
//     found) and returns HERE — BEFORE this step's own write-action loop
//     runs even once (Pitfall 2: claudeCodeRemoveAction is
//     plan.Actions[0] for every claude-code auth mode). A compared
//     would-write classification, and every NOT-compared (ambiguous)
//     classification, falls through to the SAME loop below unchanged
//     (D-09/D-10: ambiguity never becomes license to skip a write). Run
//     each Action in order on the resolved binary. A non-Tolerant
//     action's nonzero exit or seam error is OutcomeFailed immediately.
//     A Tolerant action's nonzero exit is appended to Notes and the
//     sequence continues (03-RESEARCH.md Pattern 1 / Pitfall 1's
//     tolerant-remove-then-fatal-add shape, for a runtime that needs it).
//  8. Run Plan.Probe again (read #2), if this runtime has one. For a
//     COMPARED runtime, Registered is rebuilt through the SAME
//     Observe -> renderObservation path Preview uses (D-02) — a header
//     value the second read echoes back can never reach this field
//     unredacted; Outcome stays OutcomeWrote regardless of what this
//     re-observe says (D-02 — never reclassifies to already-correct,
//     never fails the row). A NOT-compared runtime keeps the ORIGINAL raw
//     bounded capture.
//  9. For a NOT-compared runtime only: byte-compare the RAW read #1 and
//     read #2 captures: identical means OutcomeAlreadyCorrect, different
//     (or a runtime with no Probe wired, or a read #2 seam error) means
//     OutcomeWrote. D-08/D-09's own invariant — ambiguity resolves to
//     wrote, never to already-correct — is what makes "no Probe wired
//     yet" a safe degradation rather than a bug.
func execute(ctx context.Context, env Environment, rt Runtime, opts Options, mutate bool) Result {
	name := rt.Name()
	if !rt.Detect(env) {
		return Result{Runtime: name, Present: false, Outcome: OutcomeNotPresent}
	}

	plan, err := rt.Plan(env, opts)
	if err != nil {
		return Result{Runtime: name, Present: true, Outcome: OutcomeFailed, Reason: err.Error()}
	}
	if len(plan.Actions) == 0 {
		// D-16: a Plan with no Actions authors nothing to write — its
		// whole deliverable is Plan.Config. Never touch LookPath or Run,
		// and never consult mutate: this outcome is the same in preview
		// and --apply alike. See this function's own doc comment, step
		// 2a, for the full reasoning.
		return Result{Runtime: name, Present: true, Outcome: OutcomeWouldWrite, Command: plan.Display(), Config: plan.Config, Skills: plan.Skills}
	}
	// Validate EVERY action, not only Actions[0]: the run loop below slices
	// action.Args[1:] for each action, which panics on a zero-length slice
	// and is caught by no recover() in this call chain — so a malformed
	// later action would crash the whole --apply invocation and lose every
	// OTHER runtime's row, rather than failing just this runtime's.
	//
	// Index 0 alone was sufficient only while a Plan held exactly one
	// action. Plan.Actions is documented as growable (claude-code authors
	// two; Phase 4 appends more), and that same growability is why
	// Action.Tolerant is an authored field rather than a positional rule.
	// Validating the whole slice keeps the guard true as Plans grow.
	//
	// This runs BEFORE any Run: a malformed Plan is an authoring bug, and
	// executing its well-formed prefix first would half-apply it. For
	// claude-code's shape that prefix is the tolerant `mcp remove`, so
	// running it would clear a real registration on behalf of a Plan that
	// was never going to complete.
	for i, action := range plan.Actions {
		if len(action.Args) == 0 {
			return Result{
				Runtime: name, Present: true, Outcome: OutcomeFailed,
				Reason: fmt.Sprintf("%s: Plan() authored action %d with no Args", name, i),
			}
		}
	}

	binary, err := env.LookPath(plan.Actions[0].Args[0])
	if err != nil {
		return Result{
			Runtime: name, Present: true, Outcome: OutcomeFailed,
			Reason: fmt.Sprintf("%s: resolve %q on PATH: %v", name, plan.Actions[0].Args[0], err),
		}
	}

	res := Result{Runtime: name, Present: true, Command: plan.Display(), Binary: binary, Config: plan.Config, Skills: plan.Skills}
	if opts.TokenFile != "" {
		// D-06/D-07: a --token-file path has no execution meaning for a
		// runtime that registers by EXECUTING something (plan.Actions is
		// non-empty here — the zero-Actions, Config-only case returned
		// above without ever reaching this line) — the child process only
		// writes config, never reads or uses the credential, which the
		// runtime resolves itself from its own environment at connect
		// time. This is a STRUCTURAL rule keyed on "does this Plan carry
		// at least one Action", never a comparison against Name(): the
		// generic pseudo-runtime is excluded because its Plan has zero
		// Actions, not because this code knows its name. The marker never
		// carries the supplied PATH — bearerProvenance already renders
		// the path where it means something (generic.go), and duplicating
		// it into an "ignored" field would be noise (T-03-26).
		res.TokenFile = tokenFileIgnoredMarker
	}
	hasProbe := len(plan.Probe) > 0

	var probe1 RunResult
	var probe1Err error
	if hasProbe {
		probe1, probe1Err = runSeam(ctx, env, binary, plan.Probe[1:])
	}
	// c is the SAME classification (Phase 5, D-01) both lanes below
	// consult — computed once, fresh from THIS execute() call's own
	// probe1 read (Pitfall 1: never carried across process invocations,
	// never a second parse path, 04-RESEARCH.md Pitfall 3).
	c := classifyProbe(name, rt, plan, hasProbe, probe1, probe1Err, opts)

	if !mutate {
		res.Outcome = OutcomeWouldWrite
		if !c.compared {
			// This text is the only Drift note a scanner-less, probe-wired
			// runtime (or a probe1 seam error, or unframeable output) ever
			// produces.
			res.Drift = c.notCompared
			return res
		}
		renderClassification(&res, name, c)
		return res
	}

	if hasProbe && probe1Err != nil {
		res.Outcome = OutcomeFailed
		res.Reason = describeSeamError(name, quoteArgs(plan.Probe), probe1Err)
		return res
	}

	// D-01/Pitfall 2 (the load-bearing ordering rule): on a COMPARED
	// classification, already-correct and preserved return HERE — before
	// the write-action loop's first iteration. claudeCodeRemoveAction is
	// plan.Actions[0] for every claude-code auth mode; a return here means
	// it is NEVER dispatched on a preserved or already-correct row (SC1,
	// SC2, closing the ryr82bf2s2 incident class). Facets/Drift/Registered
	// are rendered here too (they describe what the pre-write read found)
	// even for a would-write row that falls through to the loop below —
	// its Registered is overwritten by the post-write re-observe further
	// down; its Outcome/Reason are overwritten only if a later action
	// fails. An ambiguous (not-compared) read falls through unchanged —
	// D-09/D-10's ambiguity-resolves-to-wrote invariant restated for the
	// apply lane: ambiguity must never become a license to skip a write.
	if c.compared {
		renderClassification(&res, name, c)
		switch c.drift.Outcome {
		case OutcomeAlreadyCorrect, OutcomePreserved:
			return res
		}
	}

	var notes []string
	for _, action := range plan.Actions {
		rr, runErr := runSeam(ctx, env, binary, action.Args[1:])
		switch {
		case runErr != nil:
			res.Outcome = OutcomeFailed
			res.Reason = describeSeamError(name, action.Command(), runErr)
			if len(notes) > 0 {
				res.Notes = strings.Join(notes, "; ")
			}
			return res
		case rr.ExitCode != 0 && action.Tolerant:
			notes = append(notes, toleratedNote(action, rr.ExitCode, rr.Stderr))
		case rr.ExitCode != 0:
			res.Outcome = OutcomeFailed
			res.Reason = describeFailure(name, action.Command(), rr.ExitCode, rr.Stderr)
			if len(notes) > 0 {
				res.Notes = strings.Join(notes, "; ")
			}
			return res
		case action.Tolerant:
			// A tolerant action's DESCRIPTION is surfaced on Notes even when
			// it succeeded (exit 0) — not only when its own exit was
			// tolerated above. A tolerant action's Description (e.g.
			// claudecode.go's claudeCodeRemoveAction) can carry a
			// consequence that only matters if a LATER, non-tolerant
			// action in the same Plan then fails: the operator reading a
			// failed row's Notes alongside its Reason must learn that
			// state, without this executor knowing anything about
			// claude-code by name (03-02-PLAN.md Task 2). This is a
			// general executor behavior applied uniformly to every
			// tolerant action in any runtime's Plan, not a special case.
			if action.Description != "" {
				notes = append(notes, action.Description)
			}
		}
	}
	if len(notes) > 0 {
		res.Notes = strings.Join(notes, "; ")
	}

	if !hasProbe {
		res.Outcome = OutcomeWrote
		return res
	}

	probe2, probe2Err := runSeam(ctx, env, binary, plan.Probe[1:])
	if probe2Err != nil {
		// Ambiguity resolves to wrote, never to already-correct (D-08).
		res.Outcome = OutcomeWrote
		return res
	}
	res.Registered = displayCapture(probe2.Stdout + probe2.Stderr)

	if probe1Err == nil && probe1.Stdout == probe2.Stdout && probe1.Stderr == probe2.Stderr {
		res.Outcome = OutcomeAlreadyCorrect
	} else {
		res.Outcome = OutcomeWrote
	}
	return res
}
