// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
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

// Preview runs rt's read-only detection and planning against env and opts
// without ever executing a write action: the classified Outcome always
// stays OutcomeWouldWrite for a present runtime (D-10 reverses Phase 2's
// D-12 "shells out to nothing" preview posture only insofar as a probe MAY
// run for informational purposes in a later plan — this task's Preview
// performs no exec at all, so cmd/engram's existing preview path, which
// calls Detect/Plan directly rather than through this function, does not
// regress T-02-08's "a preview cannot produce a nonzero exit code").
func Preview(ctx context.Context, env Environment, rt Runtime, opts Options) Result {
	return execute(ctx, env, rt, opts, false)
}

// Apply performs rt's registration against env and opts: resolve the
// binary, read current state (Plan.Probe), run the write Action(s), read
// state again, and classify wrote vs already-correct by a RAW byte
// compare of the two reads (D-08). One shared, package-level executor
// (D-03) — no per-runtime knowledge lives here; every runtime-specific
// string was already authored in that runtime's own Plan() (D-09).
func Apply(ctx context.Context, env Environment, rt Runtime, opts Options) Result {
	return execute(ctx, env, rt, opts, true)
}

// runSeam bounds one Environment.Run call to execTimeout (D-12) via a
// fresh context.WithTimeout per exec, per the deliberate per-exec (not
// per-runtime, not per-Apply-call) scoping this package's own doc comments
// commit to.
func runSeam(ctx context.Context, env Environment, path string, args []string) (RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	return env.Run(ctx, path, args)
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
	if bounded := boundCapture(stderr); bounded != "" {
		reason += ": " + bounded
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

// toleratedNote builds one Result.Notes entry for a TOLERATED nonzero
// exit: the action's own Description (when authored — e.g.
// claudecode.go's claudeCodeRemoveAction) is prefixed onto the rendered
// argv, exit code, and bounded captured stderr, so a genuinely broken
// tolerant step — and any consequence its Description records for a
// following action's failure — stays legible even though it never fails
// the row.
func toleratedNote(action Action, exitCode int, stderr string) string {
	if action.Description == "" {
		return fmt.Sprintf("%s exited %d: %s", action.Command(), exitCode, boundCapture(stderr))
	}
	return fmt.Sprintf("%s: %s exited %d: %s", action.Description, action.Command(), exitCode, boundCapture(stderr))
}

// execute is the shared sequencing core both Preview and Apply delegate
// to. Sequence:
//  1. rt.Detect(env) false -> OutcomeNotPresent, no exec of any kind.
//  2. rt.Plan(env, opts) error -> OutcomeFailed, Reason = err.Error().
//  3. Resolve Args[0] of the FIRST action via env.LookPath and record it
//     as Result.Binary (D-04) — every subsequent exec in this runtime's
//     sequence reuses that SAME resolved path, closing the TOCTOU window
//     between Detect and the write.
//  4. Run Plan.Probe (read #1) if this runtime has one wired; keep the
//     RAW combined captures in local variables, never bounded.
//  5. mutate == false (Preview): classify OutcomeWouldWrite and return —
//     the probe read above is not yet rendered onto Result this task
//     (03-01-PLAN.md Task 1: "the preview-side probe reporting is wired
//     in a later plan").
//  6. A probe SEAM error (start failure/timeout) under Apply is
//     OutcomeFailed.
//  7. Run each Action in order on the resolved binary. A non-Tolerant
//     action's nonzero exit or seam error is OutcomeFailed immediately.
//     A Tolerant action's nonzero exit is appended to Notes and the
//     sequence continues (03-RESEARCH.md Pattern 1 / Pitfall 1's
//     tolerant-remove-then-fatal-add shape, for a runtime that needs it).
//  8. Run Plan.Probe again (read #2), if this runtime has one.
//  9. Byte-compare the RAW read #1 and read #2 captures: identical means
//     OutcomeAlreadyCorrect, different (or a runtime with no Probe wired,
//     or a read #2 seam error) means OutcomeWrote. D-08's own invariant —
//     ambiguity resolves to wrote, never to already-correct — is what
//     makes "no Probe wired yet" a safe degradation rather than a bug.
func execute(ctx context.Context, env Environment, rt Runtime, opts Options, mutate bool) Result {
	name := rt.Name()
	if !rt.Detect(env) {
		return Result{Runtime: name, Present: false, Outcome: OutcomeNotPresent}
	}

	plan, err := rt.Plan(env, opts)
	if err != nil {
		return Result{Runtime: name, Present: true, Outcome: OutcomeFailed, Reason: err.Error()}
	}
	if len(plan.Actions) == 0 || len(plan.Actions[0].Args) == 0 {
		return Result{
			Runtime: name, Present: true, Outcome: OutcomeFailed,
			Reason: fmt.Sprintf("%s: Plan() authored no actions", name),
		}
	}

	binary, err := env.LookPath(plan.Actions[0].Args[0])
	if err != nil {
		return Result{
			Runtime: name, Present: true, Outcome: OutcomeFailed,
			Reason: fmt.Sprintf("%s: resolve %q on PATH: %v", name, plan.Actions[0].Args[0], err),
		}
	}

	res := Result{Runtime: name, Present: true, Command: plan.Display(), Binary: binary}
	hasProbe := len(plan.Probe) > 0

	var probe1 RunResult
	var probe1Err error
	if hasProbe {
		probe1, probe1Err = runSeam(ctx, env, binary, plan.Probe[1:])
	}

	if !mutate {
		res.Outcome = OutcomeWouldWrite
		return res
	}

	if hasProbe && probe1Err != nil {
		res.Outcome = OutcomeFailed
		res.Reason = describeSeamError(name, quoteArgs(plan.Probe), probe1Err)
		return res
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
	res.Registered = boundCapture(probe2.Stdout + probe2.Stderr)

	if probe1Err == nil && probe1.Stdout == probe2.Stdout && probe1.Stderr == probe2.Stderr {
		res.Outcome = OutcomeAlreadyCorrect
	} else {
		res.Outcome = OutcomeWrote
	}
	return res
}
