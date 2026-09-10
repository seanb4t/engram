// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/setup"
	"github.com/seanb4t/engram/internal/skills"
	"github.com/seanb4t/engram/internal/surfaces"
)

var (
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

// skillsEnv is the injectable internal/skills.Environment seam the skills
// facet writes through — package-level and t.Cleanup-overridable in a
// test, mirroring setupEnv above, so a test drives an in-memory
// destination instead of a real home directory (repo rule m45p2b4bp7).
var skillsEnv = skills.OSEnvironment

// setupSkillsTarget maps a setup.SkillTarget onto a skills.Target — the
// ONE explicit mapping across the D-05 package boundary, exhaustive over
// the three real SkillFormat values. ok is false for any format this
// switch does not recognize, INCLUDING the Go zero value ("") a runtime
// whose Plan() has not yet authored a SkillTarget naturally carries
// (Codex, opencode, and generic in this wave — only claude-code is wired,
// per this plan's tracer scope). The caller uses ok to skip the skills
// facet entirely for such a runtime, rather than installing to an empty
// destination or silently producing skills.Target's zero value — there is
// deliberately no default case mapping an unrecognized format to
// anything, so an actually-invalid non-empty format (a future authoring
// bug) is just as visibly skipped as the "not wired yet" case, never
// silently coerced into a destination nobody authored.
func setupSkillsTarget(t setup.SkillTarget) (skills.Target, bool) {
	switch t.Format {
	case setup.SkillFormatNone:
		return skills.Target{Format: skills.FormatNone}, true
	case setup.SkillFormatNative:
		return skills.Target{Format: skills.FormatNative, Dir: t.Dir, IndexFile: t.IndexFile}, true
	case setup.SkillFormatAgentsMD:
		return skills.Target{Format: skills.FormatAgentsMD, Dir: t.Dir, IndexFile: t.IndexFile}, true
	default:
		return skills.Target{}, false
	}
}

// setupSkillsDigestSummary renders inv's per-skill content digest as
// "<skill-name>:<12 hex>" entries joined by a single comma with no
// spaces, in inventory order (D-06's row-field shape) — an operator can
// compare two machines' skills without diffing files.
func setupSkillsDigestSummary(inv []skills.Skill) string {
	parts := make([]string, len(inv))
	for i, s := range inv {
		parts[i] = s.Name + ":" + skills.Digest(s)
	}
	return strings.Join(parts, ",")
}

// setupJoinReason appends next onto existing with "; " when existing is
// non-empty, mirroring the shared executor's own Notes-joining idiom
// (internal/setup/apply.go) — used here to fold a skills-facet failure
// into a row's Reason alongside (never instead of) any registration
// failure already recorded there (D-07).
func setupJoinReason(existing, next string) string {
	if existing == "" {
		return next
	}
	return existing + "; " + next
}

// setupApplySkillsFacet composes the skills facet onto row and returns
// row's final, AGGREGATED outcome (D-06): registrationOutcome unchanged
// when target carries no recognized SkillTarget (setupSkillsTarget's
// ok == false — the runtime has not been wired for skills this wave), or
// AggregateOutcome(registrationOutcome, skillsOutcome) when it does.
//
// mutate selects the preview lane (skills.Inventory only — never
// skills.Install; a preview performs no filesystem write of any kind) vs
// the apply lane (skills.Install actually writes through skillsEnv).
// includeContent gates skills_content population per D-03: populated only
// when the resolved output format is not text, so the dense text row
// never carries skill file content.
func setupApplySkillsFacet(row *setupRuntimeRow, registrationOutcome setup.Outcome, planTarget setup.SkillTarget, mutate bool, includeContent bool) setup.Outcome {
	target, ok := setupSkillsTarget(planTarget)
	if !ok {
		return registrationOutcome
	}
	row.Registration = string(registrationOutcome)

	inv, invErr := skills.Inventory()
	if invErr != nil {
		row.Skills = string(setup.OutcomeFailed)
		row.Reason = setupJoinReason(row.Reason, fmt.Sprintf("skills: %v", invErr))
		return setup.AggregateOutcome(registrationOutcome, setup.OutcomeFailed)
	}

	var wrote, alreadyCorrect int
	var installErr error
	if mutate {
		report := skills.Install(skillsEnv, target, inv)
		wrote, alreadyCorrect = len(report.Wrote), len(report.AlreadyCorrect)
		installErr = report.Err
	}

	skillsOutcome := setup.SkillsOutcome(planTarget.Format, mutate, wrote, alreadyCorrect, installErr != nil)
	row.Skills = string(skillsOutcome)
	row.SkillsDest = target.Dir
	row.SkillsIndex = target.IndexFile
	row.SkillsDigest = setupSkillsDigestSummary(inv)
	row.SkillsBytes = strconv.Itoa(skills.TotalBytes(inv))
	if includeContent {
		if content, cErr := skills.ContentJSON(inv); cErr == nil {
			row.SkillsContent = content
		}
	}
	if installErr != nil {
		row.Reason = setupJoinReason(row.Reason, fmt.Sprintf("skills: %v", installErr))
	}
	return setup.AggregateOutcome(registrationOutcome, skillsOutcome)
}

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
//
// Config deliberately does NOT take D-15's second clause: D-15 states
// both that the generic pseudo-runtime's portable config is "minified
// single-line JSON in an ordinary row field" AND that "--output json
// nests it properly as an object for machine consumers". Those two are
// not simultaneously satisfiable against this shipped renderer — a field
// typed to marshal as a JSON object falls through viewScalar's kind
// switch (cmd/engram/operator_view.go), which recognizes only a JSON
// string and JSON null, straight to a verbatim, UNSANITIZED render, and
// TestOperatorViewFixturesHaveNoUnsanitizedNesting
// (operator_output_test.go) fails on exactly that shape by design —
// reopening that gap would put an unsanitized, operator-supplied --url
// into the text lane, the T-06-03 control the guard exists to protect.
// Config therefore stays a Go string (D-15's first clause, honored
// intact): --output json emits it as a JSON STRING whose contents happen
// to be JSON, not as a nested object. A later "consistency" edit that
// promotes this field to a struct, map, or json.RawMessage type will fail
// that guard test rather than silently reopening the gap.
//
// Phase 4 (D-06) promotes Outcome from "the registration outcome" to "the
// aggregated per-runtime setup outcome": Registration and Skills are the
// two FACETS aggregation folds together (setup.AggregateOutcome), each
// riding as its own ordinary key=value field, exactly like every field
// above. SkillsDest/SkillsIndex/SkillsDigest/SkillsBytes/SkillsContent are
// the skills facet's own detail fields (D-03/D-11) — SkillsContent is
// populated ONLY for a non-text output lane (setupApplySkillsFacet), so
// the dense text row never carries skill file content, which can run to
// tens of kilobytes. Every one of these seven fields is a plain string —
// the same "never a struct, map, or raw-message type" constraint Config's
// own comment states above applies identically to each of them.
type setupRuntimeRow struct {
	Name       string `json:"name"`
	Present    bool   `json:"present"`
	Outcome    string `json:"outcome"`
	Command    string `json:"command,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Binary     string `json:"binary,omitempty"`
	Registered string `json:"registered,omitempty"`
	TokenFile  string `json:"token_file,omitempty"`
	Config     string `json:"config,omitempty"`
	Notes      string `json:"notes,omitempty"`

	Registration  string `json:"registration,omitempty"`
	Skills        string `json:"skills,omitempty"`
	SkillsDest    string `json:"skills_dest,omitempty"`
	SkillsIndex   string `json:"skills_index,omitempty"`
	SkillsDigest  string `json:"skills_digest,omitempty"`
	SkillsBytes   string `json:"skills_bytes,omitempty"`
	SkillsContent string `json:"skills_content,omitempty"`
}

// setupReportDoc is the one typed document setupPreview and setupApplyRun
// both render through renderOperator — text and json cannot drift because
// there is no second serialization (D-15).
type setupReportDoc struct {
	Runtimes []setupRuntimeRow `json:"runtimes"`
}

// setupBuildRows runs setup.Preview for every runtime in runtimes against
// env and opts, returning one row per runtime — the SAME setup.Preview /
// setup.Apply shared-executor path setupApplyRun already uses (D-15's
// one-serialization-plus-a-view invariant extended to the row-building
// path itself, 03-05-PLAN.md Task 1): the preview and apply lanes cannot
// drift because there is only one code path from a Runtime to a Result.
// setup.Preview never executes a write action and never produces a
// nonzero-affecting outcome on its own — an absent runtime is
// OutcomeNotPresent (D-07, not a failure), and a Plan() failure (e.g.
// setup.ErrAuthModeUnsupported) becomes an OutcomeFailed row naming the
// reason via err.Error() — never string-matched, but also never elevated
// to a whole-command failure this phase, since a single unsupported
// (runtime, auth) pair must not prevent every OTHER selected runtime's row
// from rendering (D-08: a preview exits nonzero only for usage/config
// errors; per-runtime partial-failure exit codes are out of this plan's
// scope). For a present runtime with a Plan.Probe wired, setup.Preview also
// runs that probe once and reports its output on Result.Registered (D-10)
// — this is what makes the report show present state next to intended
// state without ever changing this function's own no-command-level-error
// contract.
func setupBuildRows(ctx context.Context, env setup.Environment, runtimes []setup.Runtime, opts setup.Options, includeContent bool) []setupRuntimeRow {
	rows := make([]setupRuntimeRow, 0, len(runtimes))
	for _, rt := range runtimes {
		rows = append(rows, setupRuntimeRowFromResult(setup.Preview(ctx, env, rt, opts), false, includeContent))
	}
	return rows
}

// setupPreviewSummary renders the operator-facing one-line PREVIEW
// headline: how many of the selected runtimes are present, that a bare
// invocation reads current state from each present runtime's own CLI (D-10
// — the fact that makes the probe's side effect, including a live network
// dial for two of the three native runtimes, discoverable by reading
// rather than by observing, per REQ-setup-correct-by-reading), and that
// --apply is what actually performs the registration.
func setupPreviewSummary(rows []setupRuntimeRow) string {
	present := 0
	for _, r := range rows {
		if r.Present {
			present++
		}
	}
	return fmt.Sprintf(
		"preview: %d/%d selected runtime(s) present; a present runtime's own CLI is read to show current state (two of the three dial the configured URL); run with --apply to register",
		present, len(rows))
}

// setupResolve resolves --url/--auth through config.Load (CR-01: the
// ENGRAM_URL/ENGRAM_AUTH environment lane this command's --help has always
// advertised), validates --auth, and selects runtimes via setupRuntime —
// the resolution logic setupPlanDoc (preview) and setupApplyRun (apply)
// both need, kept in exactly one place so it cannot drift between the two
// closures (D-14's stated shape for setup).
func setupResolve(cmd *cobra.Command) ([]setup.Runtime, setup.Options, error) {
	// flagToKey (internal/config/registry.go) is keyed by flag NAME, and
	// setup carries flags named "output" and "token-file" that collide with
	// the client.output and client.token_file rows. When either is passed
	// on this command, this Load also writes those client.* keys. That is
	// inert here — setupResolve reads only cfg.Setup and never cfg.Client —
	// but it should be stated rather than discovered.
	cfg, err := config.Load(cmd.Flags())
	if err != nil {
		return nil, setup.Options{}, usageErrorf("load setup configuration: %w", err)
	}

	if err := config.ValidateSetupAuth(cfg.Setup.Auth); err != nil {
		return nil, setup.Options{}, usageErrorf("%w", err)
	}
	// ValidateSetupAuth accepts "" as "use the default" (the validator's
	// own doc comment) — this is the one call site that resolves it,
	// since --auth's own pflag default is already "oauth"
	// (config.FlagDefault) and only an explicit `--auth ""` reaches here
	// empty.
	auth := cfg.Setup.Auth
	if auth == "" {
		auth = "oauth"
	}

	runtimes, err := setup.Select(setupRuntime)
	if err != nil {
		return nil, setup.Options{}, usageErrorf("%w", err)
	}

	if cfg.Setup.URL == "" {
		// Mirrors clientFromFlags' "--server or ENGRAM_SERVER_URL is
		// required" guard (client_common.go) — a runtime usageErrorf, not
		// cobra's own required-flag mechanism (which raises a plain
		// fmt.Errorf bypassing cliError/ExitCode() — the exact defect D-03
		// rejected MarkFlagsMutuallyExclusive for) and not reachable when
		// ENGRAM_URL alone supplies the value, which cobra's mechanism
		// would wrongly demand anyway.
		return nil, setup.Options{}, usageErrorf("--url or ENGRAM_URL is required")
	}

	return runtimes, setup.Options{URL: cfg.Setup.URL, Auth: auth, TokenFile: setupTokenFile}, nil
}

// setupPlanDoc resolves --url/--auth/--runtime (setupResolve) and builds
// the preview report doc setupPreview renders, via setup.Preview
// (setupBuildRows) for every selected runtime — a present runtime with a
// Plan.Probe wired may run that ONE read-only probe (D-10); it never
// performs a write of any kind. T-02-08's exit-code property is
// unaffected: a probe's own result (zero exit, nonzero exit, or a seam
// error) never changes a preview's classification or produces a nonzero
// process exit code — only a usage/configuration error (returned by
// setupResolve, before any runtime is touched) does that.
func setupPlanDoc(ctx context.Context, cmd *cobra.Command, format outputFormat) (setupReportDoc, error) {
	runtimes, opts, err := setupResolve(cmd)
	if err != nil {
		return setupReportDoc{}, err
	}
	rows := setupBuildRows(ctx, setupEnv, runtimes, opts, format != formatText)
	return setupReportDoc{Runtimes: rows}, nil
}

// setupPreview is registerDestructive's preview closure: it builds the
// SAME report setupApplyRun builds and never performs a write of any kind
// — Detect's exec.LookPath call is a read-only resolution, and the one
// probe setup.Preview may run per present runtime (D-10) is itself a
// read-only invocation of that runtime's own CLI, never a mutation.
// cliNow's preview cutoff clock (destructive.go) is deliberately unused
// here (D-14): it exists for store-sweep commands' not_after comparisons
// and carries no meaning for a local-machine detection preview.
func setupPreview(ctx context.Context, cmd *cobra.Command) error {
	format, err := operatorOutputFormat(cmd, setupOutput)
	if err != nil {
		return err
	}
	doc, err := setupPlanDoc(ctx, cmd, format)
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

// setupRuntimeRowFromResult maps one internal/setup.Result onto its
// cmd/engram row shape, field-for-field (D-04/D-07/D-10/D-15 — every new
// Result field is one more key=value the existing renderOperator pipeline
// picks up automatically, no bespoke rendering code).
//
// For a PRESENT runtime, this also composes the skills facet
// (setupApplySkillsFacet) and OVERWRITES Outcome with the aggregated
// value D-06 requires — r.Outcome itself is passed through unchanged as
// the registration facet's own outcome, and is what setupApplySkillsFacet
// returns verbatim when this runtime carries no recognized SkillTarget
// (a runtime not yet wired for skills this wave: setupSkillsTarget's
// ok == false). A not-present runtime is skipped entirely: it keeps its
// OutcomeNotPresent row untouched, with no skills facet (D-07).
func setupRuntimeRowFromResult(r setup.Result, mutate bool, includeContent bool) setupRuntimeRow {
	row := setupRuntimeRow{
		Name:       r.Runtime,
		Present:    r.Present,
		Outcome:    string(r.Outcome),
		Command:    r.Command,
		Reason:     r.Reason,
		Binary:     r.Binary,
		Registered: r.Registered,
		TokenFile:  r.TokenFile,
		Config:     r.Config,
		Notes:      r.Notes,
	}
	if r.Present {
		row.Outcome = string(setupApplySkillsFacet(&row, r.Outcome, r.Skills, mutate, includeContent))
	}
	return row
}

// setupResultsFromRows converts report rows into internal/setup.Result
// values for setup.Classify — a pure, local mapping so internal/setup
// never needs to know about cmd/engram's rendering types (the leaf-purity
// gate, leafpurity_test.go).
func setupResultsFromRows(rows []setupRuntimeRow) []setup.Result {
	results := make([]setup.Result, len(rows))
	for i, r := range rows {
		results[i] = setup.Result{
			Runtime:    r.Name,
			Present:    r.Present,
			Outcome:    setup.Outcome(r.Outcome),
			Command:    r.Command,
			Reason:     r.Reason,
			Binary:     r.Binary,
			Registered: r.Registered,
			TokenFile:  r.TokenFile,
			Config:     r.Config,
			Notes:      r.Notes,
		}
	}
	return results
}

// setupApplySummary renders the operator-facing one-line APPLY headline:
// how many of the selected runtimes were written, were already correct,
// and failed — replacing Phase 2's "registration lands in a later phase"
// wording, which is false as of this phase (D-09's stub is retired).
func setupApplySummary(rows []setupRuntimeRow) string {
	var wrote, already, failed int
	for _, r := range rows {
		switch setup.Outcome(r.Outcome) {
		case setup.OutcomeWrote:
			wrote++
		case setup.OutcomeAlreadyCorrect:
			already++
		case setup.OutcomeFailed:
			failed++
		}
	}
	return fmt.Sprintf("apply: %d wrote, %d already correct, %d failed (of %d selected runtime(s))", wrote, already, failed, len(rows))
}

// setupApplyRun is registerDestructive's apply closure. It resolves
// --url/--auth/--runtime (setupResolve) and calls setup.Apply once per
// selected runtime, accumulating rows independently (D-03, the
// errors.Join-style precedent in internal/migrate/registry.go): one
// runtime's failure never prevents another's attempt or its row from
// rendering.
//
// The report is rendered with renderOperator UNCONDITIONALLY, before any
// error is returned (T-02-06): a nonzero exit must never erase the
// per-runtime record of what happened. setup.Classify then determines the
// process exit code — exitPartial (8) now has its first live production
// producer, since a runtime can genuinely succeed under --apply
// (catalog_test.go's nonConnectProducedCodes already names this).
func setupApplyRun(ctx context.Context, cmd *cobra.Command) error {
	format, err := operatorOutputFormat(cmd, setupOutput)
	if err != nil {
		return err
	}
	runtimes, opts, err := setupResolve(cmd)
	if err != nil {
		return err
	}

	rows := make([]setupRuntimeRow, len(runtimes))
	for i, rt := range runtimes {
		rows[i] = setupRuntimeRowFromResult(setup.Apply(ctx, setupEnv, rt, opts), true, format != formatText)
	}
	doc := setupReportDoc{Runtimes: rows}

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
// that a bare invocation reads each present runtime's own CLI for its
// current state, including a network dial for two of the three (D-10 —
// what makes that side effect discoverable by reading rather than by
// observing); and the four accepted --auth modes, including bearer's
// narrowed --token-file scope (D-06).
func setupLongDescription() string {
	return fmt.Sprintf(`Detect installed agent runtimes and preview registering engram as an MCP server.

Targetable runtimes (select one or more with --runtime): %s. A bare
invocation targets every detected runtime.

A bare invocation (no --apply) contacts each present runtime's own CLI to
read its current registration state for the report; nothing is written.
For claude-code and opencode, that read dials the configured URL; for
codex, it is a pure local read.

%s

Accepted --auth modes:
  oauth         OAuth via the runtime's own login/callback flow (default)
  oauth-client  a pre-registered OAuth client (client id and secret)
  bearer        a static bearer token. A native runtime (claude-code, codex,
                opencode) is registered with an environment-variable
                REFERENCE naming ENGRAM_TOKEN, resolved by that runtime
                itself at connect time — the token never appears on any
                command line or in any config engram writes. --token-file
                applies only to the portable configuration (--runtime
                generic): it names that credential's PROVENANCE there,
                carrying the path, never the secret — and has no effect on
                a native runtime, whose row carries token_file=ignored
                when the flag is supplied
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
	setupCmd.Flags().String("url", config.FlagDefault("url"),
		"MCP endpoint URL to register, used verbatim — never appended to or stripped (required) (default: ENGRAM_URL)")
	setupCmd.Flags().String("auth", config.FlagDefault("auth"),
		`auth mode: "oauth", "oauth-client", "bearer", or "none" (default: ENGRAM_AUTH)`)
	setupCmd.Flags().StringSliceVar(&setupRuntime, "runtime", setupRuntimeEnvDefault(),
		fmt.Sprintf("runtimes to target, comma-separated or repeated (default: every detected runtime); valid values: %s (default: ENGRAM_RUNTIME)",
			strings.Join(setup.Names(), ", ")))
	setupCmd.Flags().StringVar(&setupTokenFile, "token-file", "",
		"path naming the bearer credential's provenance for the portable configuration (--runtime generic) — carries only the PATH, never the secret itself; has no effect for a natively-registered runtime (claude-code, codex, opencode), which resolves the credential itself from its own environment at connect time")
	registerDestructive(setupCmd, &setupApply, setupPreview, setupApplyRun)
	rootCmd.AddCommand(setupCmd)
}
