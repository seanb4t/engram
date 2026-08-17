// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/surfaces"
)

// purgeSameRunLimitationNotice is the ONE sentence stating D-11's residual
// limitation: an operator cannot preview in one invocation and apply in a
// later one. Declared once, here, so the preview output, the command's
// `Long` help text, and every test asserting either surface all read the
// SAME string rather than three independently-typed copies that could
// silently diverge. The CLI guide restates this sentence in prose (it
// cannot reference a Go constant), but a reviewer confirms that copy cold
// per 03-VALIDATION.md's Manual-Only Verifications rather than a test
// re-deriving it from source.
const purgeSameRunLimitationNotice = "a preview cannot be carried into a later invocation: --apply re-derives eligibility within its own run and deletes only the intersection of what it just showed and what is still eligible at that moment"

// purgeIntersectionScopingNotice is the companion sentence stating exactly
// what the intersection buys and does not: the preview and --apply
// derivations run milliseconds apart in ONE process, so the intersection
// guards against a CONCURRENT WRITER changing eligibility in that window,
// never against operator delay between reading a preview and deciding to
// act. Read alongside purgeSameRunLimitationNotice by both the preview
// output and the `Long` help text, so an operator reading --help cannot
// infer a stronger guarantee than the same-run shape actually gives.
const purgeIntersectionScopingNotice = "the two derivations run milliseconds apart in one process, so the intersection guards against a concurrent writer changing eligibility in that window, not against operator delay -- the extract gate, the mandatory --scope on the filter path, and the discovery/rule category exclusions carry the real safety weight"

// purgeMilestoneSummaryGateNotice states the extract gate's two paths
// honestly and asymmetrically: the per-record path is unforgeable, the
// batch floor is convention-validation over a real record, never proof.
const purgeMilestoneSummaryGateNotice = "the per-record path reads the server-set superseded_by link and cannot be satisfied by anything a caller writes; the batch floor requires a real milestone-summary record whose creation time the server stamped, but its marker tag is caller-mintable, so that path validates a convention, not proof of preservation"

// purgeClassNames is the enumerated --class vocabulary, in the order
// --help lists it -- the single source parsePurgeClasses validates
// against, so the flag's usage text and the validator can never silently
// diverge.
var purgeClassNames = []string{string(store.PurgeClassSuperseded), string(store.PurgeClassExpired), string(store.PurgeClassArchived)}

// parsePurgeClasses validates raw against purgeClassNames, rejecting an
// unrecognized value as a usage error (exit 2) rather than silently
// ignoring it or passing it through to the store layer.
func parsePurgeClasses(raw []string) ([]store.PurgeClass, error) {
	out := make([]store.PurgeClass, 0, len(raw))
	for _, r := range raw {
		valid := false
		for _, name := range purgeClassNames {
			if r == name {
				valid = true
				break
			}
		}
		if !valid {
			return nil, usageErrorf("--class %q is not one of: %s", r, strings.Join(purgeClassNames, ", "))
		}
		out = append(out, store.PurgeClass(r))
	}
	return out, nil
}

// requirePurgeFilterScope enforces D-10's harder filter-path gate: an
// explicit --scope or --all-scopes is required whenever the free-form
// filter path (category/tags/older-than with no class selected --
// store.PurgeFilterPathActive) is engaged. A structural-class-only run
// (even one that supplies --older-than as that class's own window
// override) is allowed without either, since a class is a derivation
// rather than an operator judgment (D-10). Composes the registered
// surfaces.RulePurgeFilterRequiresScope rule's Sentence into the rejection
// -- never a bare, unregistered usage check (RESEARCH.md Pitfall 2).
func requirePurgeFilterScope(opts store.PurgeOptions) error {
	if opts.Scope != "" || opts.AllScopes {
		return nil
	}
	if !store.PurgeFilterPathActive(opts) {
		return nil
	}
	rule, ok := surfaces.RuleByID(surfaces.RulePurgeFilterRequiresScope)
	if !ok {
		panic("spine-review purge: surfaces.RulePurgeFilterRequiresScope is not registered in internal/surfaces/rules.go")
	}
	return usageErrorf("%s", rule.Sentence)
}

var (
	spinePurgeScope     string
	spinePurgeAllScopes bool
	spinePurgeClass     []string
	spinePurgeCategory  string
	spinePurgeTags      []string
	spinePurgeOlderThan time.Duration
	spinePurgeTimeout   time.Duration
	spinePurgeOutput    string
	spinePurgeApply     bool
)

// spinePurgeWithTimeout mirrors pruneWithTimeout (prune.go): 0 disables the
// deadline.
func spinePurgeWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if spinePurgeTimeout > 0 {
		return context.WithTimeout(ctx, spinePurgeTimeout)
	}
	return ctx, func() {}
}

// spinePurgeOptionsFromFlags builds a store.PurgeOptions from the leaf's own
// flags plus the cliNow() clock seam (destructive.go), truncated to the
// second exactly like pruneCutoffNow does -- the single place this
// invocation's derivation instant is read.
func spinePurgeOptionsFromFlags() (store.PurgeOptions, error) {
	classes, err := parsePurgeClasses(spinePurgeClass)
	if err != nil {
		return store.PurgeOptions{}, err
	}
	return store.PurgeOptions{
		Classes: classes, Scope: spinePurgeScope, AllScopes: spinePurgeAllScopes,
		Category: spinePurgeCategory, Tags: spinePurgeTags, OlderThan: spinePurgeOlderThan,
		Now: cliNow().Truncate(time.Second),
	}, nil
}

// purgeClassesText renders classes for the summary header -- "(none)" when
// empty, so a bare filter-path-only run's header never reads as though a
// class were silently assumed.
func purgeClassesText(classes []store.PurgeClass) string {
	if len(classes) == 0 {
		return "(none)"
	}
	names := make([]string, len(classes))
	for i, c := range classes {
		names[i] = string(c)
	}
	return strings.Join(names, ",")
}

// purgeScopeText renders opts' scope selection for the summary header.
func purgeScopeText(opts store.PurgeOptions) string {
	if opts.AllScopes {
		return "(all scopes)"
	}
	if opts.Scope != "" {
		return opts.Scope
	}
	return "(none)"
}

// shellQuote wraps s in single quotes, escaping any embedded single quote
// via the standard POSIX-shell idiom: close the current quote, emit a
// backslash-escaped literal quote, then reopen the quote. The result is
// safe to substitute into a `sh`/`bash`-family command line regardless of
// spaces, globs, or other shell metacharacters s might contain. Values
// this package interpolates into a copy-pasteable command (scope,
// category, tag) are free-form strings an operator can set on the write
// path (WR-03, 06-REVIEW.md), so they cannot be trusted to be shell-safe
// on their own.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// purgeRerunCommand renders the exact command an operator would run to
// apply this same derivation -- the plain re-run the preview output names,
// mirroring prune-expired's own "re-run with --apply" wording, expanded
// here to carry every flag this invocation actually supplied so the
// printed command is genuinely copy-pasteable. Every free-form value
// (--scope, --category, --tags) is shell-quoted via shellQuote so a value
// containing a space or shell metacharacter still round-trips through a
// copy-paste into a shell as the single argument it actually was (WR-03,
// 06-REVIEW.md) -- --class and --older-than need no quoting: both are
// drawn from a closed, operator-controlled vocabulary (purgeClassNames,
// time.Duration.String()) that never contains shell metacharacters.
func purgeRerunCommand(opts store.PurgeOptions) string {
	var b strings.Builder
	b.WriteString("engram spine-review purge --apply")
	for _, c := range opts.Classes {
		fmt.Fprintf(&b, " --class %s", c)
	}
	if opts.AllScopes {
		b.WriteString(" --all-scopes")
	} else if opts.Scope != "" {
		fmt.Fprintf(&b, " --scope %s", shellQuote(opts.Scope))
	}
	if opts.Category != "" {
		fmt.Fprintf(&b, " --category %s", shellQuote(opts.Category))
	}
	for _, tag := range opts.Tags {
		fmt.Fprintf(&b, " --tags %s", shellQuote(tag))
	}
	if opts.OlderThan != 0 {
		fmt.Fprintf(&b, " --older-than %s", opts.OlderThan)
	}
	return b.String()
}

// purgeReportDoc is the JSON-mode shape shared by BOTH preview and applied
// output: an explicit Applied boolean, plus deleted/spared/appeared as
// separate count fields AND separate id lists -- never one field whose
// meaning depends on which mode ran. In preview mode Eligible/EligibleCount
// carry the would-be-deleted set and Deleted/Spared/Appeared are empty (no
// second derivation has happened yet); in applied mode Eligible/
// EligibleCount still carry the manifest this invocation rendered BEFORE
// deleting, and Deleted/Spared/AppearedCount report ApplyPurge's own
// three-way split. Content is never printed on any path -- only ids.
type purgeReportDoc struct {
	Applied       bool     `json:"applied"`
	Classes       []string `json:"classes"`
	Scope         string   `json:"scope,omitempty"`
	AllScopes     bool     `json:"all_scopes"`
	Category      string   `json:"category,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	OlderThan     string   `json:"older_than,omitempty"`
	EligibleCount int      `json:"eligible_count"`
	Eligible      []string `json:"eligible"`
	DeletedCount  int      `json:"deleted_count"`
	Deleted       []string `json:"deleted"`
	SparedCount   int      `json:"spared_count"`
	Spared        []string `json:"spared"`
	AppearedCount int      `json:"appeared_count"`
	Appeared      []string `json:"appeared"`
	// SameRunLimitation/IntersectionScoping carry the two published notices
	// as explicit fields (never left to prose alone in text mode only), so
	// a json consumer sees the identical wording a human reading --help does.
	SameRunLimitation string `json:"same_run_limitation"`
	IntersectionScope string `json:"intersection_scoping"`
	// Rerun is the exact re-run command an operator would use to apply this
	// preview -- added per 06-01-PLAN.md Conversion Rules R2 (gap closure):
	// purgePreviewSummary stated this command in prose and no existing key
	// carried it. Populated on the preview document only (an applied run has
	// nothing left to re-run into being); omitempty so an applied document
	// carries no "rerun" key at all, matching the pre-conversion behavior
	// where only the preview sentence printed a "re-run:" line.
	Rerun string `json:"rerun,omitempty"`
}

// olderThanText renders d as its flag-form string, or "" when zero -- so
// omitempty correctly drops the field rather than emitting "0s".
func olderThanText(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}

// nonNilStrings returns ss unchanged when non-nil, or an empty (non-nil)
// slice -- every id-list field in purgeReportDoc marshals as "[]", never
// "null", mirroring every other spine-review leaf's *Doc converter.
func nonNilStrings(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

// purgePreviewDoc converts a preview-mode eligible-id set into
// purgeReportDoc. Takes the already-extracted id slice, never the
// store.PurgeManifest value itself: PurgeManifest's fields are all
// unexported by design (see its doc comment in internal/store/spine.go),
// so a formatter accepting the id slice directly is trivially unit-testable
// with a hand-built list -- accepting the manifest type here would force
// every such test to dial a live Qdrant just to obtain one.
func purgePreviewDoc(eligible []string, opts store.PurgeOptions) purgeReportDoc {
	classNames := make([]string, len(opts.Classes))
	for i, c := range opts.Classes {
		classNames[i] = string(c)
	}
	return purgeReportDoc{
		Applied: false, Classes: classNames, Scope: opts.Scope, AllScopes: opts.AllScopes,
		Category: opts.Category, Tags: nonNilStrings(opts.Tags), OlderThan: olderThanText(opts.OlderThan),
		EligibleCount: len(eligible), Eligible: nonNilStrings(eligible),
		Deleted: []string{}, Spared: []string{}, Appeared: []string{},
		SameRunLimitation: purgeSameRunLimitationNotice, IntersectionScope: purgeIntersectionScopingNotice,
		Rerun: purgeRerunCommand(opts),
	}
}

// purgeAppliedDoc converts an applied-mode eligible-id set (the manifest
// this invocation rendered before deleting) plus ApplyPurge's result into
// purgeReportDoc. See purgePreviewDoc's doc comment for why this takes
// []string rather than store.PurgeManifest.
func purgeAppliedDoc(eligible []string, res store.PurgeResult, opts store.PurgeOptions) purgeReportDoc {
	doc := purgePreviewDoc(eligible, opts)
	doc.Applied = true
	doc.DeletedCount, doc.Deleted = len(res.Deleted), nonNilStrings(res.Deleted)
	doc.SparedCount, doc.Spared = len(res.Spared), nonNilStrings(res.Spared)
	doc.AppearedCount, doc.Appeared = len(res.Appeared), nonNilStrings(res.Appeared)
	// An applied run has nothing left to re-run into being -- the re-run
	// command is preview-only, mirroring the pre-conversion text where only
	// the preview sentence printed a "re-run:" line.
	doc.Rerun = ""
	return doc
}

// purgePreviewSummary is a headline producer (06-CONTEXT.md D-04) for a bare
// (no --apply) run: the eligible count and the derivation's classes/scope,
// as ONE hand-written prose line -- mirroring reindexSummary's/
// pruneSummary's pure-formatter discipline (no I/O, no *store.Store, no
// context.Context). The eligible id list, the two published notices, and
// the re-run command all moved from this sentence into purgeReportDoc's
// field table (eligible, same_run_limitation, intersection_scoping, rerun);
// the operator view renders them from there. Takes the already-extracted id
// slice, never store.PurgeManifest -- see purgePreviewDoc's doc comment.
func purgePreviewSummary(eligible []string, opts store.PurgeOptions) string {
	return fmt.Sprintf("spine purge preview: %d record(s) eligible (classes=%s scope=%s)",
		len(eligible), purgeClassesText(opts.Classes), purgeScopeText(opts))
}

// purgeAppliedSummary is a headline producer (06-CONTEXT.md D-04) for an
// --apply run: deleted/spared/appeared counts, each named explicitly, with
// the appeared set carrying its own wording so an operator understands
// those records were NOT purged and a re-run would include them (T-03-22's
// mitigation) -- this explanatory nuance is exactly what D-04 says a field
// name cannot carry, so it stays in this hand-written line. The three id
// lists this sentence used to enumerate row-by-row now render from
// purgeReportDoc's own deleted/spared/appeared fields via the operator view.
func purgeAppliedSummary(res store.PurgeResult, opts store.PurgeOptions) string {
	return fmt.Sprintf("spine purge applied: %d deleted, %d spared (ineligible since preview), %d appeared "+
		"(newly eligible since preview -- NOT purged; re-run to include) (classes=%s scope=%s)",
		len(res.Deleted), len(res.Spared), len(res.Appeared), purgeClassesText(opts.Classes), purgeScopeText(opts))
}

// spinePurgePreview is registerDestructive's preview closure: it derives
// eligibility, runs the extract gate, and renders the result -- performing
// NO delete on any path.
func spinePurgePreview(ctx context.Context, cmd *cobra.Command) error {
	format, err := operatorOutputFormat(cmd, spinePurgeOutput)
	if err != nil {
		return err
	}
	opts, err := spinePurgeOptionsFromFlags()
	if err != nil {
		return err
	}
	if err := requirePurgeFilterScope(opts); err != nil {
		return err
	}
	st, err := server.StoreFromEnv()
	if err != nil {
		return classifyOperatorErrConstruction(err)
	}
	ctx, cancel := spinePurgeWithTimeout(ctx)
	defer cancel()
	manifest, err := st.PreviewPurge(ctx, opts)
	if err != nil {
		return classifyOperatorErr(err)
	}
	return renderOperator(cmd, format, purgePreviewSummary(manifest.IDs(), opts), purgePreviewDoc(manifest.IDs(), opts))
}

// spinePurgeApplyRun is registerDestructive's apply closure. It renders the
// set it is about to delete (a diagnostic stderr line naming the count --
// never the main JSON/text document, which stays exactly one document per
// D-13) via a first PreviewPurge call, then calls ApplyPurge with that
// manifest: ApplyPurge itself re-derives eligibility fresh, milliseconds
// later, and deletes only the intersection -- the two derivations this
// invocation's own preview-then-apply flow performs.
func spinePurgeApplyRun(ctx context.Context, cmd *cobra.Command) error {
	format, err := operatorOutputFormat(cmd, spinePurgeOutput)
	if err != nil {
		return err
	}
	opts, err := spinePurgeOptionsFromFlags()
	if err != nil {
		return err
	}
	if err := requirePurgeFilterScope(opts); err != nil {
		return err
	}
	st, err := server.StoreFromEnv()
	if err != nil {
		return classifyOperatorErrConstruction(err)
	}
	ctx, cancel := spinePurgeWithTimeout(ctx)
	defer cancel()

	manifest, err := st.PreviewPurge(ctx, opts)
	if err != nil {
		return classifyOperatorErr(err)
	}
	cmd.PrintErrf("spine purge: about to delete %d record(s), intersected against a fresh re-derivation\n", len(manifest.IDs()))

	applyOpts := opts
	applyOpts.Now = cliNow().Truncate(time.Second)
	res, err := st.ApplyPurge(ctx, manifest, applyOpts)
	if err != nil {
		return classifyOperatorErr(err)
	}
	return renderOperator(cmd, format, purgeAppliedSummary(res, opts), purgeAppliedDoc(manifest.IDs(), res, opts))
}

// spineReviewPurgeCmd is the phase's sharpest edge: preview-by-default,
// intersection-only delete under --apply, gated on rule 7smp8vy9hr's
// extract-before-delete ordering. Registered through registerDestructive
// (destructive.go) -- the shared choke point that owns cmd.RunE and calls
// addApplyFlag internally, so this leaf never assigns its own RunE and
// never registers its own --apply flag. There is NO transport flag of any
// kind: the manifest carrying preview into apply is in-process only (see
// 03-07-PLAN.md's "purge manifest transport is settled" section), so an
// operator has nothing to carry between runs and must not be offered a
// flag suggesting otherwise.
var spineReviewPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Delete purge-eligible records, gated on an extraction precondition",
	Long: "Delete purge-eligible records under one or more structural classes (superseded, expired, " +
		"archived) or a free-form filter (category/tags/older-than, which requires an explicit --scope). " +
		"Every candidate must satisfy rule 7smp8vy9hr's extract-before-delete gate: a server-set " +
		"superseded_by link to a later record, or an authoritative milestone-summary record covering " +
		"the batch. " + purgeSameRunLimitationNotice + ". " + purgeIntersectionScopingNotice + ". " +
		purgeMilestoneSummaryGateNotice + ".",
}

func init() {
	addOperatorOutputFlag(spineReviewPurgeCmd, &spinePurgeOutput)
	spineReviewPurgeCmd.Flags().StringVar(&spinePurgeScope, "scope", "", "restrict the derivation to one scope")
	spineReviewPurgeCmd.Flags().BoolVar(&spinePurgeAllScopes, "all-scopes", false, "span every scope; mutually exclusive with --scope")
	spineReviewPurgeCmd.Flags().StringSliceVar(&spinePurgeClass, "class", nil,
		"structural eligibility class (repeatable): "+strings.Join(purgeClassNames, ", "))
	scopeRule, _ := surfaces.RuleByID(surfaces.RulePurgeFilterRequiresScope)
	spineReviewPurgeCmd.Flags().StringVar(&spinePurgeCategory, "category", "",
		"free-form filter: only this category; "+scopeRule.Sentence)
	spineReviewPurgeCmd.Flags().StringSliceVar(&spinePurgeTags, "tags", nil,
		"free-form filter: records must carry ALL listed tags (repeatable); "+scopeRule.Sentence)
	spineReviewPurgeCmd.Flags().DurationVar(&spinePurgeOlderThan, "older-than", 0,
		"grace/retention window override for the selected class(es) (0 = each class's own default: "+
			"immediate for superseded/expired, 90 days for archived); with NO class selected, a free-form "+
			"created-more-than-this-long-ago filter -- "+scopeRule.Sentence)
	spineReviewPurgeCmd.Flags().DurationVar(&spinePurgeTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the run (0 disables); also cancellable via Ctrl-C")
	spineReviewPurgeCmd.MarkFlagsMutuallyExclusive("scope", "all-scopes")
	// AddCommand FIRST: registerDestructive's classification lookup keys on
	// commandKey(cmd), which walks cmd.CommandPath() through its parent --
	// calling it before this command is attached to spineReviewCmd would
	// resolve to the bare "purge" rather than "spine-review purge", missing
	// the blast-radius row entirely.
	spineReviewCmd.AddCommand(spineReviewPurgeCmd)
	registerDestructive(spineReviewPurgeCmd, &spinePurgeApply, spinePurgePreview, spinePurgeApplyRun)
}
