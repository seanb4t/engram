// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/setup"
	"github.com/seanb4t/engram/internal/surfaces"
)

var (
	setupURL       string
	setupAuth      string
	setupTokenFile string
	setupOutput    string
	setupRuntime   []string
	setupApply     bool
)

// setupEnv is the injectable internal/setup.Environment seam setupPlanDoc
// consults — package-level and t.Cleanup-overridable in a test, mirroring
// cliNow (destructive.go) and citationFileReader
// (spine_review_verify.go), so a test drives a fake PATH/home instead of
// the real machine (repo rule m45p2b4bp7).
var setupEnv = setup.OSEnvironment

// setupCmd detects which supported agent runtimes are present on the
// machine and reports the exact command it would issue to register engram
// as an MCP server on each — before writing anything. Classified
// Destructive in internal/surfaces/toolclass.go (D-13: a runtime's own
// `mcp add` invocation overwrites an existing "engram" entry rather than
// refusing or merging), so it is registered through registerDestructive
// (destructive.go): a bare invocation previews and performs NO write;
// --apply is reserved for Phase 3, which actually executes the authored
// invocation (D-09 — stubbed this phase). The command does not assign its
// own RunE — registerDestructive owns that, which is what
// TestDestructiveCommandsRouteThroughGate asserts.
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Detect installed agent runtimes and preview registering engram as an MCP server",
}

// setupRuntimeEnvDefault splits ENGRAM_RUNTIME on "," into --runtime's
// default value, returning nil for an unset/empty var (Pitfall 3): unlike
// --url/--auth, --runtime is deliberately NOT routed through the
// internal/config registry — pflag's StringSliceValue.String() returns the
// bracketed display form ("[a b]"), which the registry's changed-flag
// overlay cannot round-trip. Mirrors reindex.go --target's direct
// os.Getenv-default precedent.
func setupRuntimeEnvDefault() []string {
	v := os.Getenv("ENGRAM_RUNTIME")
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}

// setupRuntimeRow is one runtime's row in setupReportDoc.Runtimes,
// rendered through renderOperator with zero bespoke rendering code
// (D-15): viewRow already renders each element as a dense
// "key=value ..." line, and sanitizeViewValue already strips control
// characters from any string field here, including a Plan()-authored
// command string.
type setupRuntimeRow struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
	Outcome string `json:"outcome"`
	Command string `json:"command,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// setupReportDoc is the one typed document setupPreview and setupApplyRun
// both render through renderOperator — text and json cannot drift because
// there is no second serialization (D-15).
type setupReportDoc struct {
	Runtimes []setupRuntimeRow `json:"runtimes"`
}

// setupBuildRows runs Detect+Plan for every runtime in runtimes against
// env and opts, returning one row per runtime. This function never
// returns a command-level error: an absent runtime is OutcomeNotPresent
// (D-07, not a failure), and a Plan() failure (e.g.
// setup.ErrAuthModeUnsupported) becomes an OutcomeFailed row naming the
// reason via err.Error() — never string-matched, but also never elevated
// to a whole-command failure this phase, since a single unsupported
// (runtime, auth) pair must not prevent every OTHER selected runtime's row
// from rendering (D-08: a preview exits nonzero only for usage/config
// errors; per-runtime partial-failure exit codes are out of this plan's
// scope).
func setupBuildRows(env setup.Environment, runtimes []setup.Runtime, opts setup.Options) []setupRuntimeRow {
	rows := make([]setupRuntimeRow, 0, len(runtimes))
	for _, rt := range runtimes {
		if !rt.Detect(env) {
			rows = append(rows, setupRuntimeRow{
				Name:    rt.Name(),
				Present: false,
				Outcome: string(setup.OutcomeNotPresent),
			})
			continue
		}

		plan, err := rt.Plan(env, opts)
		if err != nil {
			rows = append(rows, setupRuntimeRow{
				Name:    rt.Name(),
				Present: true,
				Outcome: string(setup.OutcomeFailed),
				Reason:  err.Error(),
			})
			continue
		}

		var cmdStr string
		if len(plan.Actions) > 0 {
			cmdStr = plan.Actions[0].Command
		}
		rows = append(rows, setupRuntimeRow{
			Name:    rt.Name(),
			Present: true,
			Outcome: string(setup.OutcomeWouldWrite),
			Command: cmdStr,
		})
	}
	return rows
}

// setupPreviewSummary renders the operator-facing one-line PREVIEW
// headline: how many of the selected runtimes are present, and that
// --apply is not yet the way to register them (Phase 3, D-09).
func setupPreviewSummary(rows []setupRuntimeRow) string {
	present := 0
	for _, r := range rows {
		if r.Present {
			present++
		}
	}
	return fmt.Sprintf("preview: %d/%d selected runtime(s) present; registration via --apply lands in a later phase", present, len(rows))
}

// setupPlanDoc validates --auth, selects runtimes via setupRuntime, and
// builds the report doc shared by setupPreview and setupApplyRun — so
// both closures render the identical document (D-14's stated shape for
// setup).
func setupPlanDoc(_ *cobra.Command) (setupReportDoc, error) {
	if err := config.ValidateSetupAuth(setupAuth); err != nil {
		return setupReportDoc{}, usageErrorf("%w", err)
	}
	// ValidateSetupAuth accepts "" as "use the default" (the validator's
	// own doc comment) — this is the one call site that resolves it,
	// since --auth's own pflag default is already "oauth"
	// (config.FlagDefault) and only an explicit `--auth ""` reaches here
	// empty.
	auth := setupAuth
	if auth == "" {
		auth = "oauth"
	}

	runtimes, err := setup.Select(setupRuntime)
	if err != nil {
		return setupReportDoc{}, usageErrorf("%w", err)
	}

	opts := setup.Options{URL: setupURL, Auth: auth, TokenFile: setupTokenFile}
	rows := setupBuildRows(setupEnv, runtimes, opts)
	return setupReportDoc{Runtimes: rows}, nil
}

// setupPreview is registerDestructive's preview closure: it builds the
// SAME report setupApplyRun builds and performs no write of any kind — no
// file, no runtime CLI execution (Detect's exec.LookPath call is a
// read-only resolution, never an invocation). cliNow's preview cutoff
// clock (destructive.go) is deliberately unused here (D-14): it exists
// for store-sweep commands' not_after comparisons and carries no meaning
// for a local-machine detection preview.
func setupPreview(_ context.Context, cmd *cobra.Command) error {
	format, err := operatorOutputFormat(cmd, setupOutput)
	if err != nil {
		return err
	}
	doc, err := setupPlanDoc(cmd)
	if err != nil {
		return err
	}
	return renderOperator(cmd, format, setupPreviewSummary(doc.Runtimes), doc)
}

// setupExitCode maps a setup.ExitClass to this binary's process exit code
// (D-06). The default arm is unreachable while ExitClass has exactly three
// values; it exists only as the backstop catalog.go documents exitGeneric
// to be, not as a code path any caller of this function should reach.
func setupExitCode(c setup.ExitClass) int {
	switch c {
	case setup.ExitTotalSuccess:
		return exitOK
	case setup.ExitPartial:
		return exitPartial
	case setup.ExitTotalFailure:
		return exitSetupFailed
	default:
		return exitGeneric
	}
}

// setupApplyStubReason is the Reason every present (attempted) runtime's
// row carries under --apply this phase (D-09): Apply() itself is not
// implemented until Phase 3, so every attempted runtime fails identically
// regardless of whether Plan() would itself have succeeded.
func setupApplyStubReason() string {
	return fmt.Sprintf("%v — run `engram setup` (without --apply) to preview the invocation it would issue", setup.ErrApplyNotImplemented)
}

// setupResultsFromRows converts report rows into internal/setup.Result
// values for setup.Classify — a pure, local mapping so internal/setup
// never needs to know about cmd/engram's rendering types (the leaf-purity
// gate, leafpurity_test.go).
func setupResultsFromRows(rows []setupRuntimeRow) []setup.Result {
	results := make([]setup.Result, len(rows))
	for i, r := range rows {
		results[i] = setup.Result{
			Runtime: r.Name,
			Present: r.Present,
			Outcome: setup.Outcome(r.Outcome),
			Command: r.Command,
			Reason:  r.Reason,
		}
	}
	return results
}

// setupApplySummary renders the operator-facing one-line APPLY headline:
// how many of the selected runtimes failed this phase's stub, since every
// attempted runtime does (D-09).
func setupApplySummary(rows []setupRuntimeRow) string {
	failed := 0
	for _, r := range rows {
		if r.Outcome == string(setup.OutcomeFailed) {
			failed++
		}
	}
	return fmt.Sprintf("apply: %d/%d selected runtime(s) failed; registration lands in a later phase (Phase 3)", failed, len(rows))
}

// setupApplyRun is registerDestructive's apply closure. Phase 2 ships no
// Apply() implementation (D-09): it builds the SAME per-runtime rows
// setupPreview builds (same Detect(), same Plan(), same ordering — text
// and JSON stay one serialization plus a view), then marks every runtime
// that was actually attempted (Present) as failed with a Reason naming
// setup.ErrApplyNotImplemented. A not-present runtime keeps
// OutcomeNotPresent untouched (D-07) — it was never attempted, so it
// cannot fail.
//
// The report is rendered with renderOperator UNCONDITIONALLY, before any
// error is returned (T-02-06): a nonzero exit must never erase the
// per-runtime record of what happened. setup.Classify then determines the
// process exit code; exitPartial has no live producer until Phase 3 makes
// a runtime capable of succeeding under --apply (every attempted runtime
// fails this phase, so only ExitTotalSuccess — nothing attempted — and
// ExitTotalFailure are reachable here; that gap is recorded in
// catalog_test.go's nonConnectProducedCodes and proven by exit_test.go's
// pure table, not by this command).
func setupApplyRun(_ context.Context, cmd *cobra.Command) error {
	format, err := operatorOutputFormat(cmd, setupOutput)
	if err != nil {
		return err
	}
	doc, err := setupPlanDoc(cmd)
	if err != nil {
		return err
	}

	reason := setupApplyStubReason()
	rows := make([]setupRuntimeRow, len(doc.Runtimes))
	for i, r := range doc.Runtimes {
		if !r.Present {
			rows[i] = r
			continue
		}
		rows[i] = setupRuntimeRow{
			Name:    r.Name,
			Present: true,
			Outcome: string(setup.OutcomeFailed),
			Reason:  reason,
		}
	}
	doc.Runtimes = rows

	if err := renderOperator(cmd, format, setupApplySummary(rows), doc); err != nil {
		return err
	}

	class := setup.Classify(setupResultsFromRows(rows))
	if class == setup.ExitTotalSuccess {
		return nil
	}
	failed := 0
	for _, r := range rows {
		if r.Outcome == string(setup.OutcomeFailed) {
			failed++
		}
	}
	return &cliError{
		code: setupExitCode(class),
		err: fmt.Errorf("engram setup --apply: %d of %d selected runtime(s) failed — run `engram setup` (without --apply) to preview the invocation each would issue",
			failed, len(rows)),
	}
}

// setupApplySentence returns surfaces.RuleDestructiveRequiresApply's own
// Sentence — the exact string addApplyFlag composes --apply's Usage from
// — so setupLongDescription states the same fact by REFERENCE, never by
// re-typing it, keeping TestSurfaceConformanceCobraUsage satisfied by
// construction rather than by a hand-copied string staying in sync by
// convention.
func setupApplySentence() string {
	rule, ok := surfaces.RuleByID(surfaces.RuleDestructiveRequiresApply)
	if !ok {
		panic("setup: surfaces.RuleDestructiveRequiresApply is not registered in internal/surfaces/rules.go")
	}
	return rule.Sentence
}

// setupLongDescription composes setupCmd's Long help text
// (REQ-setup-correct-by-reading, success criterion 5): which runtimes are
// targetable, derived by calling setup.Names() so it cannot drift from
// the registry; that a bare invocation previews and changes nothing while
// --apply performs the registration (setupApplySentence, by reference);
// and the four accepted --auth modes and what each means.
func setupLongDescription() string {
	return fmt.Sprintf(`Detect installed agent runtimes and preview registering engram as an MCP server.

Targetable runtimes (select one or more with --runtime): %s. A bare
invocation targets every detected runtime.

%s

Accepted --auth modes:
  oauth         OAuth via the runtime's own login/callback flow (default)
  oauth-client  a pre-registered OAuth client (client id and secret)
  bearer        a static bearer token, named by provenance from --token-file
                (or ENGRAM_TOKEN when --token-file is omitted) — the token
                itself never appears on the command line
  none          a local / no-auth server`,
		strings.Join(setup.Names(), ", "), setupApplySentence())
}

// setupExample carries three worked invocations
// (REQ-setup-correct-by-reading, success criterion 5): a bare preview, a
// --runtime-scoped preview, and a --auth bearer --token-file preview.
const setupExample = `  engram setup --url https://engram.example.com/mcp
  engram setup --url https://engram.example.com/mcp --runtime claude-code
  engram setup --url https://engram.example.com/mcp --auth bearer --token-file ~/.engram/token`

func init() {
	setupCmd.Long = setupLongDescription()
	setupCmd.Example = setupExample
	addOperatorOutputFlag(setupCmd, &setupOutput)
	setupCmd.Flags().StringVar(&setupURL, "url", config.FlagDefault("url"),
		"MCP endpoint URL to register, used verbatim — never appended to or stripped (default: ENGRAM_URL)")
	setupCmd.Flags().StringVar(&setupAuth, "auth", config.FlagDefault("auth"),
		`auth mode: "oauth", "oauth-client", "bearer", or "none" (default: ENGRAM_AUTH)`)
	setupCmd.Flags().StringSliceVar(&setupRuntime, "runtime", setupRuntimeEnvDefault(),
		fmt.Sprintf("runtimes to target, comma-separated or repeated (default: every detected runtime); valid values: %s (default: ENGRAM_RUNTIME)",
			strings.Join(setup.Names(), ", ")))
	setupCmd.Flags().StringVar(&setupTokenFile, "token-file", "",
		"path to a file containing the bearer credential for --auth bearer (carries only the PATH, never the secret itself; no environment fallback)")
	registerDestructive(setupCmd, &setupApply, setupPreview, setupApplyRun)
	rootCmd.AddCommand(setupCmd)
}
