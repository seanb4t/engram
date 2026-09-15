// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

// This file is the plugin-delivery lane: a PARALLEL execution path to
// registration (execute, apply.go), not nested inside it. D-01's
// "outdated" classification is a semantic comparison of a parsed version
// field — something execute()'s blind pre/post byte-compare cannot
// express (03-RESEARCH.md Summary, Anti-Patterns) — and
// REQ-plugin-facet-reported needs a `wrote` registration to stay visible
// beside a `failed` plugin install, which one action sequence cannot
// report. So the plugin lane authors and runs its OWN actions through the
// same runSeam bound every registration exec uses, and reports its own
// PluginResult independently of Result.
//
// Imports are limited to the standard library: the JSON parse for each
// runtime's `plugin list --json` shape lives in that runtime's own file
// (claudecode.go, codex.go), so this file needs no encoding/json import
// at all. Nothing here ever imports the x/mod semver module —
// TestSetupPackageIsStdlibOnlyLeaf fails the build on any non-stdlib
// import reaching this leaf (03-RESEARCH.md Pitfall 3); the stdlib
// comparator below is a deliberate, local reimplementation of the same
// idea x/mod/semver's Compare/IsValid express, used here as a design
// reference only, never as a dependency.
import (
	"context"
	"fmt"
	"regexp"
	"strconv"
)

// PluginState classifies the observed relationship between an installed
// engram plugin and the resolved binary version it is being compared
// against. It follows SkillFormat's (plan.go) never-the-zero-value
// discipline: every value is a REAL, explicit value, and a zero-valued
// PluginState is a programming error, never meaningful data.
type PluginState string

const (
	// PluginAbsent means no engram@engram entry was found in the
	// runtime's `plugin list --json` output (REQ-plugin-three-way-state).
	PluginAbsent PluginState = "absent"
	// PluginOutdated means an engram@engram entry was found, and its
	// version is numerically less than the resolved binary version
	// (REQ-plugin-three-way-state, D-01).
	PluginOutdated PluginState = "outdated"
	// PluginCurrent means an engram@engram entry was found and its
	// version is equal to, newer than, or not numerically comparable to
	// the resolved binary version (REQ-plugin-three-way-state, D-01,
	// D-03) — current is never an update trigger.
	PluginCurrent PluginState = "current"
	// PluginUnavailable means the runtime's plugin capability could not
	// be observed at all: a nonzero probe exit, a seam error (including a
	// runSeam timeout), or output this package could not parse as the
	// runtime's documented JSON shape (D-10, D-12). This is NEVER a
	// failed outcome — see PluginResult.Outcome's own doc comment.
	PluginUnavailable PluginState = "unavailable"
)

// PluginRuntime is an OPTIONAL interface a Runtime may implement to
// declare that it has its own plugin ecosystem this lane can probe and
// drive. Only claude-code and codex implement it; opencode and generic do
// not (REQ-plugin-opencode is deferred; generic has no CLI at all). It is
// type-asserted exactly once, at this lane's entry point (executePlugin
// below) — never a by-name branch anywhere in this package, mirroring
// runtime.go's optInOnlyRuntime idiom. PluginRuntime is EXPORTED — unlike
// optInOnlyRuntime — because cmd/engram's tests and internal/setupgen
// must reach PluginProbes and PluginActions across the package boundary.
//
// AUTHORED-HERE invariant: every string any of these four methods returns
// is authored in the implementing runtime's own file (claudecode.go,
// codex.go) — never centrally, and never derived from the other runtime's
// shapes.
type PluginRuntime interface {
	// PluginProbes returns the two read-only argv this runtime uses to
	// observe plugin state: list is the D-10 capability-and-state probe
	// (`<cli> plugin list --json`), marketplace is the D-11 marketplace
	// probe (`<cli> plugin marketplace list`). Each argv's element 0 is
	// the bare binary name, resolved by the caller exactly like
	// Plan.Probe.
	PluginProbes() (list, marketplace []string)
	// ParsePluginList parses list's stdout. A non-nil error means the
	// output is not this CLI's documented JSON shape — D-10's
	// no-working-plugin-CLI signal, folded into PluginUnavailable by the
	// caller. A parsed list with no engram entry returns installed ==
	// false and a nil error.
	ParsePluginList(stdout string) (version string, installed bool, err error)
	// ParseMarketplaceList parses marketplace's stdout as a COARSE
	// exact-name match for a marketplace named "engram" — never a table
	// model (D-05, D-11). source is the observed source line, verbatim.
	ParseMarketplaceList(stdout string) (present bool, source string)
	// PluginActions authors the write actions this runtime would issue
	// for state, given whether a marketplace named "engram" was already
	// present. Returns nil for PluginCurrent and PluginUnavailable.
	PluginActions(state PluginState, marketplacePresent bool) []Action
}

// PluginResult is one runtime's reported plugin-delivery outcome —
// deliberately separate from Result (plan.go), because the plugin lane
// executes its own authored actions independently of registration's
// Actions/Probe sequence (see this file's own top doc comment). Attempted
// is false when rt does not implement PluginRuntime, or binary is empty
// (the registration lane's already-resolved Result.Binary was never
// produced) — every other field is then at its zero value, and cmd/engram
// omits the facet entirely.
//
// Outcome is empty when State == PluginUnavailable: no attempt was
// planned, so nothing is folded into an aggregate outcome — this is how
// D-12's "never a failed runtime row for a plugin probe failure" holds by
// construction, not by a later filter. Reason carries a failure reason,
// in describeFailure/describeSeamError form, only when Outcome ==
// OutcomeFailed.
//
// PluginResult is also where Phase 4's drift comparison is expected to
// reuse the observed plugin state (03-CONTEXT.md discretion) — nothing in
// this phase depends on that yet.
type PluginResult struct {
	Attempted bool
	State     PluginState
	// Installed is the observed installed version, empty when absent or
	// unavailable.
	Installed string
	// Target is the binary version this plugin was compared against.
	Target string
	// Source is the observed marketplace source line (D-05), verbatim.
	Source string
	// Command is Plan{Actions: actions}.Display() over the authored
	// actions — display stays a pure function of Args, never an
	// independently authored string.
	Command string
	// Note carries D-01's "newer" note, D-03's "dev build" note, D-12's
	// unavailable reason, or a marketplace-probe failure reason — several
	// joined by "; " when more than one applies.
	Note string
	// Outcome mirrors registration's five-value Outcome vocabulary — no
	// sixth constant is added for "unavailable" (Outcome stays empty in
	// that case).
	Outcome Outcome
	// Reason is the failure reason, describeFailure/describeSeamError
	// form, when Outcome == OutcomeFailed.
	Reason string
}

// Delivered reports whether this runtime's skills, hooks, and command are
// delivered by its own plugin system rather than needing engram's native
// copy — D-07's routing predicate, consumed by plan 03-03.
func (p PluginResult) Delivered() bool {
	return p.Attempted && p.State != PluginUnavailable
}

// PluginPreview runs rt's plugin probes and authors the exact actions
// PluginApply would run, without executing any of them.
func PluginPreview(ctx context.Context, env Environment, rt Runtime, binary, binaryVersion string) PluginResult {
	return executePlugin(ctx, env, rt, binary, binaryVersion, false)
}

// PluginApply runs rt's plugin probes and, when a write is needed, runs
// the authored actions through the same runSeam bound every registration
// exec uses.
func PluginApply(ctx context.Context, env Environment, rt Runtime, binary, binaryVersion string) PluginResult {
	return executePlugin(ctx, env, rt, binary, binaryVersion, true)
}

// executePlugin is the shared sequencing core both PluginPreview and
// PluginApply delegate to. Sequence, doc-commented step by step like
// execute's own (apply.go):
//  1. The type assertion to PluginRuntime fails, or binary is empty -> PluginResult{}: no
//     exec of any kind. binary is the registration lane's
//     already-resolved Result.Binary, reused (03-RESEARCH.md Open
//     Question 3, decided) — this file never resolves a path itself.
//  2. pr.PluginProbes() returning an empty list or marketplace argv is an
//     authoring bug on the runtime's own side -> PluginUnavailable naming
//     rt.Name().
//  3. Run the list probe via runSeam. A seam error or nonzero exit ->
//     PluginUnavailable with a describeSeamError/describeFailure-shaped
//     Note, and return — exactly one Run call so far.
//  4. Parse the list probe's stdout. A parse error -> PluginUnavailable.
//     Steps 3-4 are D-10's whole capability predicate.
//  5. Classify absent/outdated/current via classifyPluginVersion.
//  6. Run the marketplace probe (D-11). On failure: if state is
//     PluginAbsent, this is PluginUnavailable (never author a
//     marketplace-add action against unknown marketplace state, D-05);
//     otherwise the failure reason is appended to Note and the
//     marketplace is treated as not present, continuing with the
//     already-classified state (a working, current/outdated plugin is
//     never demoted to unavailable by a failed source read).
//  7. Author actions via pr.PluginActions(state, marketplacePresent);
//     validate every action has non-empty Args (an authoring bug ->
//     OutcomeFailed); render Command.
//  8. mutate == false -> OutcomeWouldWrite, return: a preview runs no
//     write verb.
//  9. len(actions) == 0 -> OutcomeAlreadyCorrect, return: current runs
//     zero write verbs.
//  10. Run each action in order. A Tolerant action's nonzero exit is
//     recorded onto Note (joinPluginNote/toleratedNote — the SAME
//     toleratedNote apply.go's own execute() uses, apply.go:335,344) and
//     the sequence continues; a non-Tolerant action's nonzero exit, or a
//     seam error from ANY action (tolerant or not — a seam error means no
//     exit status was ever produced, so there is nothing to tolerate), is
//     OutcomeFailed immediately and no later action runs. No plugin
//     action authored by claudecode.go or codex.go sets Tolerant today;
//     this step exists so a FUTURE one gets execute()'s own semantics
//     rather than silently failing the row on a tolerated exit (WR-01).
//  11. OutcomeWrote.
func executePlugin(ctx context.Context, env Environment, rt Runtime, binary, binaryVersion string, mutate bool) PluginResult {
	pr, ok := rt.(PluginRuntime)
	if !ok || binary == "" {
		return PluginResult{}
	}
	name := rt.Name()

	list, marketplace := pr.PluginProbes()
	if len(list) == 0 || len(marketplace) == 0 {
		return PluginResult{Attempted: true, State: PluginUnavailable, Note: fmt.Sprintf("%s: authored no plugin probe", name)}
	}

	res := PluginResult{Attempted: true, Target: binaryVersion}

	listResult, listErr := runSeam(ctx, env, binary, list[1:])
	if listErr != nil {
		res.State = PluginUnavailable
		res.Note = describeSeamError(name, quoteArgs(list), listErr)
		return res
	}
	if listResult.ExitCode != 0 {
		res.State = PluginUnavailable
		res.Note = describeFailure(name, quoteArgs(list), listResult.ExitCode, listResult.Stderr)
		return res
	}

	version, installed, parseErr := pr.ParsePluginList(listResult.Stdout)
	if parseErr != nil {
		res.State = PluginUnavailable
		res.Note = describeSeamError(name, quoteArgs(list), parseErr)
		return res
	}

	state, note := classifyPluginVersion(version, installed, binaryVersion)
	res.State = state
	if installed {
		// version is entry.Version parsed straight out of third-party
		// `plugin list --json` output (claudecode.go, codex.go) — bounded
		// the same way apply.go bounds every other third-party capture
		// before it reaches a rendered field (WR-02).
		res.Installed = boundCapture(version)
	}
	res.Note = note

	mktResult, mktErr := runSeam(ctx, env, binary, marketplace[1:])
	var mktFailureReason string
	mktOK := true
	switch {
	case mktErr != nil:
		mktFailureReason = describeSeamError(name, quoteArgs(marketplace), mktErr)
		mktOK = false
	case mktResult.ExitCode != 0:
		mktFailureReason = describeFailure(name, quoteArgs(marketplace), mktResult.ExitCode, mktResult.Stderr)
		mktOK = false
	}

	var marketplacePresent bool
	if !mktOK {
		if state == PluginAbsent {
			res.State = PluginUnavailable
			res.Note = mktFailureReason
			res.Installed = ""
			return res
		}
		if res.Note == "" {
			res.Note = mktFailureReason
		} else {
			res.Note = res.Note + "; " + mktFailureReason
		}
	} else {
		var source string
		marketplacePresent, source = pr.ParseMarketplaceList(mktResult.Stdout)
		// source is the observed "Source:" line, verbatim third-party
		// output — bounded the same way apply.go bounds every other
		// third-party capture before it reaches a rendered field (WR-02).
		res.Source = boundCapture(source)
	}

	actions := pr.PluginActions(res.State, marketplacePresent)
	for i, a := range actions {
		if len(a.Args) == 0 {
			res.Outcome = OutcomeFailed
			res.Reason = fmt.Sprintf("%s: PluginActions authored action %d with no Args", name, i)
			return res
		}
	}
	res.Command = Plan{Actions: actions}.Display()

	if !mutate {
		res.Outcome = OutcomeWouldWrite
		return res
	}

	if len(actions) == 0 {
		res.Outcome = OutcomeAlreadyCorrect
		return res
	}

	for _, action := range actions {
		rr, runErr := runSeam(ctx, env, binary, action.Args[1:])
		switch {
		case runErr != nil:
			res.Outcome = OutcomeFailed
			res.Reason = describeSeamError(name, action.Command(), runErr)
			return res
		case rr.ExitCode != 0 && action.Tolerant:
			// WR-01: mirror apply.go's execute() (apply.go:335,344) — a
			// Tolerant action's nonzero exit is recorded as a note and
			// the sequence continues, never failing the row.
			res.Note = joinPluginNote(res.Note, toleratedNote(action, rr.ExitCode, rr.Stderr))
		case rr.ExitCode != 0:
			res.Outcome = OutcomeFailed
			res.Reason = describeFailure(name, action.Command(), rr.ExitCode, rr.Stderr)
			return res
		case action.Tolerant:
			// A tolerant action's Description is surfaced even on success
			// (exit 0), mirroring apply.go's execute() (apply.go:344-358):
			// it can carry a consequence that only matters if a LATER,
			// non-tolerant action in the same sequence then fails.
			if action.Description != "" {
				res.Note = joinPluginNote(res.Note, action.Description)
			}
		}
	}
	res.Outcome = OutcomeWrote
	return res
}

// joinPluginNote appends add onto existing using the same "; "-joined
// idiom this file already uses when a marketplace-probe failure note is
// appended to a classification note (see executePlugin's marketplace
// branch above) — extracted here because the write-action loop's Tolerant
// handling (WR-01) needs the same joining behavior twice.
func joinPluginNote(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "; " + add
}

// pluginVersionCorePattern anchors parseVersionCore's input to a plain,
// unsigned SemVer core: exactly three dot-separated components, each
// either "0" or a non-zero digit followed by more digits — the same
// grammar as cmd/engram/buildversion.go's patchCorePattern (this file
// lives in internal/setup and cannot import cmd/engram, so the TECHNIQUE
// is copied, not the code). No "v" prefix, no prerelease, no build
// metadata, no leading zeros.
var pluginVersionCorePattern = regexp.MustCompile(`^(0|[1-9][0-9]*)[.](0|[1-9][0-9]*)[.](0|[1-9][0-9]*)$`)

// parseVersionCore parses v as a bare SemVer core, returning ok == false
// on anything that does not match the anchored grammar above — including
// "dev", a "v"-prefixed string, a prerelease/build-metadata suffix, and a
// leading-zero component.
func parseVersionCore(v string) (major, minor, patch uint64, ok bool) {
	m := pluginVersionCorePattern.FindStringSubmatch(v)
	if m == nil {
		return 0, 0, 0, false
	}
	maj, err := strconv.ParseUint(m[1], 10, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	mnr, err := strconv.ParseUint(m[2], 10, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	pat, err := strconv.ParseUint(m[3], 10, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return maj, mnr, pat, true
}

// compareVersionCore returns a total order over two parsed SemVer cores:
// -1 when a < b, 0 when equal, 1 when a > b — a numeric, unsigned-integer
// comparison of major, minor, then patch, never a byte-wise string
// compare (0.9.9 is less than 0.16.1; 0.16.10 is greater than 0.16.9).
func compareVersionCore(aMajor, aMinor, aPatch, bMajor, bMinor, bPatch uint64) int {
	if aMajor != bMajor {
		if aMajor < bMajor {
			return -1
		}
		return 1
	}
	if aMinor != bMinor {
		if aMinor < bMinor {
			return -1
		}
		return 1
	}
	if aPatch != bPatch {
		if aPatch < bPatch {
			return -1
		}
		return 1
	}
	return 0
}

// classifyPluginVersion is D-01/D-03's whole classification: a numeric
// SemVer-core comparison of the installed engram plugin's version against
// the caller-supplied binary version. !isInstalled always yields
// (PluginAbsent, "") regardless of the binary — D-03: a dev build installs
// only when the plugin is absent, so classification never blocks that. A
// binary or installed version that is not a bare release core (a dev
// build, a "v"-prefixed string, a prerelease/build-metadata suffix, a
// leading-zero component) is NEVER an update trigger — installing over,
// or downgrading, a local dev build would churn a real install for
// nothing — so both non-comparable cases resolve to PluginCurrent with an
// explanatory Note rather than PluginOutdated. Among two comparable
// cores: less is PluginOutdated; equal or greater is PluginCurrent — a
// newer-than-binary plugin is reported current with a note and is NEVER
// downgraded (D-01).
//
// installed is compared RAW (untruncated) below — parseVersionCore's own
// anchored grammar is the correctness gate, not a length cap — but every
// note interpolating it is bounded via boundCapture first (WR-02): binary
// is this package's own resolved build version, never third-party, so
// only installed (parsed straight out of `plugin list --json`) needs it.
func classifyPluginVersion(installed string, isInstalled bool, binary string) (PluginState, string) {
	if !isInstalled {
		return PluginAbsent, ""
	}

	bMajor, bMinor, bPatch, bOK := parseVersionCore(binary)
	if !bOK {
		return PluginCurrent, fmt.Sprintf("dev build: this binary (%s) is not a release version; plugin %s is reported current without comparison", binary, boundCapture(installed))
	}

	iMajor, iMinor, iPatch, iOK := parseVersionCore(installed)
	if !iOK {
		return PluginCurrent, fmt.Sprintf("plugin version %s is not a release version; reported current without comparison", boundCapture(installed))
	}

	switch compareVersionCore(iMajor, iMinor, iPatch, bMajor, bMinor, bPatch) {
	case -1:
		return PluginOutdated, ""
	case 0:
		return PluginCurrent, ""
	default:
		return PluginCurrent, fmt.Sprintf("plugin %s is newer than this binary %s", boundCapture(installed), binary)
	}
}
