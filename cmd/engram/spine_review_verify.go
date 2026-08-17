// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/surfaces"
)

// Citation verification tiers (D-05/D-06/D-08): the four buckets
// `spine-review verify` classifies every stored citation into. valid means
// the excerpt is exactly where the citation's Locator says it is; moved
// means the excerpt still exists in the SAME file, at a different byte
// offset -- ordinary drift from an edit above the cited range (GitHub issue
// #355's shape); broken means the file is gone or the excerpt is gone from
// it entirely; unverifiable means the classifier did not actually check the
// citation at all (a non-file kind, an empty cached excerpt, a different
// repo, or a ref this command refuses to read) -- every such citation
// carries the reason it was not checked, so a clean report can never be
// read as coverage the run did not have (REQ-citation-drift-verify's
// transparency requirement).
const (
	tierValid        = "valid"
	tierMoved        = "moved"
	tierBroken       = "broken"
	tierUnverifiable = "unverifiable"
)

// The two reasons a file citation classifies broken (D-08): the file itself
// is missing, or the file exists but the cached excerpt is nowhere in it.
// Kept as distinct constants so a caller can distinguish the two causes
// without string-matching a formatted sentence, even though both share the
// one "broken" tier name.
const (
	reasonFileMissing = "file missing"
	reasonExcerptGone = "excerpt gone"
	reasonNoExcerpt   = "no cached excerpt"
)

// citationVerdict is one citation's classification result: the tier and
// reason verifyFileCitation computes, plus enough identity (RecordID,
// ShortID, Index, Ref) for a report row. verifyFileCitation itself only
// ever sets Tier, Reason, and Ref -- the caller (this file's RunE, plan
// 03-04 Task 2) fills RecordID/ShortID/Index once it knows which owning
// record and citation index produced this verdict.
type citationVerdict struct {
	Tier     string
	Reason   string
	RecordID string
	ShortID  string
	Index    int
	Ref      string
}

// verifyFileCitation classifies a single citation against already-read file
// content -- a pure function with zero filesystem access, so the tier logic
// is unit-testable without a live filesystem or store. The caller reads
// each unique Ref once and passes the bytes in.
//
// Tiers are evaluated in EXACTLY this order -- file-missing, then
// no-excerpt, then at-locator, then same-file search, then excerpt-gone --
// which is what keeps a missing file from ever reporting "excerpt gone"
// (D-08's ordering requirement).
//
// The at-locator check is START-ANCHORED: an excerpt is "at locator" when
// fileContent has the excerpt as a prefix beginning at the locator's
// resolved offset -- the excerpt is NOT required to terminate before the
// locator's end line. A citation whose cached excerpt is longer than the
// line range it advertises is common and is not drift; requiring complete
// containment within the locator's range would misclassify that ordinary
// case as moved.
//
// The moved tier is found via a single in-file search for the excerpt,
// reporting the byte offset found -- never a whole-tree search and never a
// fuzzy match. A short excerpt risking a confident wrong match outside its
// own file is the tradeoff this narrow search deliberately accepts, per
// this phase's CONTEXT.md.
//
// A Locator that does not parse, or that resolves past the end of the
// file, falls through to the same-file search rather than erroring -- a
// stale line range is drift, not a malformed citation.
func verifyFileCitation(c store.Citation, fileContent string, fileExists bool) citationVerdict {
	v := citationVerdict{Ref: c.Ref}
	if c.Kind != "file" {
		v.Tier = tierUnverifiable
		v.Reason = fmt.Sprintf("citation kind %q is not independently verifiable", c.Kind)
		return v
	}
	if !fileExists {
		v.Tier = tierBroken
		v.Reason = reasonFileMissing
		return v
	}
	if c.Excerpt == "" {
		v.Tier = tierUnverifiable
		v.Reason = reasonNoExcerpt
		return v
	}
	if offset := excerptOffsetAt(fileContent, c.Locator); offset >= 0 && strings.HasPrefix(fileContent[offset:], c.Excerpt) {
		v.Tier = tierValid
		return v
	}
	// The locator match wins even when the same excerpt also appears
	// elsewhere -- the check above already returned before reaching this
	// search when the locator matched, so an excerpt present at BOTH the
	// locator and elsewhere still classifies valid.
	if idx := strings.Index(fileContent, c.Excerpt); idx >= 0 {
		v.Tier = tierMoved
		v.Reason = fmt.Sprintf("excerpt found at byte offset %d, not at the cited locator", idx)
		return v
	}
	v.Tier = tierBroken
	v.Reason = reasonExcerptGone
	return v
}

// excerptOffsetAt returns the byte offset content's locator-named line
// begins at, or -1 when locator is empty, unparseable, names a line before
// 1, or names a line past the end of content. Supports the two Locator
// forms the repo already writes: a bare line number ("200") and a
// "start-end" line range ("200-240") -- only the start line matters for the
// start-anchored at-locator check, so a range's end value is ignored. Do
// not invent additional forms.
func excerptOffsetAt(content, locator string) int {
	if locator == "" {
		return -1
	}
	start, ok := parseLocatorStart(locator)
	if !ok || start < 1 {
		return -1
	}
	lines := strings.Split(content, "\n")
	if start > len(lines) {
		return -1
	}
	offset := 0
	for i := 0; i < start-1; i++ {
		offset += len(lines[i]) + 1
	}
	return offset
}

// parseLocatorStart parses the leading line number out of a bare-line-number
// or "start-end" range Locator, ignoring everything from the first "-"
// onward (the range's end value is never needed by the start-anchored
// at-locator check).
func parseLocatorStart(locator string) (int, bool) {
	first := locator
	if i := strings.IndexByte(locator, '-'); i >= 0 {
		first = locator[:i]
	}
	n, err := strconv.Atoi(first)
	if err != nil {
		return 0, false
	}
	return n, true
}

// repoIdentityFromCWD derives this working tree's repo identity from `git
// remote get-url origin`, normalised via normalizeGitRemote to the
// host/path form the repo's live scope values already use
// (repo:<host>/<path>). This is a deliberate runtime dependency on the git
// CLI: D-05 rejects git as a verification mechanism for `commit` citations
// specifically (verifying a commit's content is out of scope, per
// CONTEXT.md), but deriving "which repo is this working tree" is a
// different job, and describing this command as having "no git
// dependency" would be inaccurate. Falls back to the working directory's
// base name when there is no origin remote or git is unavailable --
// RESEARCH.md's Assumptions Log row A1, chosen to fail SAFE: a
// mis-derivation here produces "unverifiable" citations downstream, never
// a confident wrong verdict.
func repoIdentityFromCWD() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		wd, wderr := os.Getwd()
		if wderr != nil {
			return "", fmt.Errorf("repo identity: no origin remote, and cannot read the working directory: %w", wderr)
		}
		return filepath.Base(wd), nil
	}
	return normalizeGitRemote(strings.TrimSpace(string(out))), nil
}

// normalizeGitRemote reduces a git remote URL to the host/path form this
// repo's live scope values already use (repo:<host>/<path>): strip the
// scheme, strip user info (git@ or user@), replace the FIRST colon after
// the host with a slash (so an SCP-style remote like
// git@github.com:seanb4t/engram.git normalises the same as its HTTPS
// equivalent, rather than leaving a colon that would misclassify every
// same-repo citation on an SSH-remote clone as "different repo" -- this
// repo's own remote is HTTPS, so the #355 fixture alone would have hidden
// that bug), strip a trailing ".git", and strip a trailing slash.
func normalizeGitRemote(remote string) string {
	s := remote
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[:i] + "/" + s[i+1:]
	}
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")
	return s
}

// sameRepoAsCWD reports whether scope names the SAME repo as
// derivedIdentity (repoIdentityFromCWD's return value), by an EXACT SEGMENT
// match on scope's colon-delimited token sequence -- never a substring
// match, which would false-positive on an unrelated scope like
// "myrepo:x". The rule, spelled out:
//
//  1. Split scope on ":" -- engram scopes are colon-delimited token
//     sequences, and the repo identity value itself (host/path) contains
//     no colon.
//  2. Find the first segment EXACTLY equal to "repo" (full-segment
//     equality, so "myrepo" never matches). No such segment, or it is the
//     last segment (nothing follows it): not the same repo.
//  3. Compare the NEXT segment to derivedIdentity by exact string
//     equality. Everything after that (a ":ws:<workspace>" overlay suffix)
//     is IGNORED -- an overlay citation is in the same repo as its spine.
//
// This covers every scope shape live in this repo today: repo:<host>/<path>
// (spine), repo:<host>/<path>:ws:<workspace> (overlay), and
// discovery:repo:<host>/<path> / rule:repo:<host>/<path> (the two
// citation-bearing categories with a leading category prefix). A scope with
// no "repo" segment at all (project:<name>, discovery:<name>) has no repo
// identity to compare and correctly reports false -- the same fail-safe
// disposition as a genuine mismatch.
func sameRepoAsCWD(scope, derivedIdentity string) bool {
	segments := strings.Split(scope, ":")
	for i, seg := range segments {
		if seg == "repo" {
			if i+1 >= len(segments) {
				return false
			}
			return segments[i+1] == derivedIdentity
		}
	}
	return false
}

// verifyWorkingRoot resolves the process's working directory through
// filepath.EvalSymlinks ONCE, up front -- the RESOLVED root every citation
// Ref's containment is checked against (resolveContainedRef), never the
// lexical (pre-symlink-resolution) working directory.
func verifyWorkingRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(wd)
	if err != nil {
		return "", err
	}
	return root, nil
}

// isContained reports whether resolved (an already filepath.EvalSymlinks-ed
// path) is root itself or lies strictly beneath it, via filepath.Rel --
// never a lexical string-prefix check, which a path like "/repo-evil"
// would defeat against a root of "/repo".
func isContained(root, resolved string) bool {
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// deepestExistingAncestor walks up path's parent directories until it finds
// one that exists (via os.Lstat, so a symlink itself counts as "existing"
// without following it), returning that ancestor. Used when path itself
// does not exist -- filepath.EvalSymlinks requires every path component to
// exist, so a genuinely missing file needs its nearest existing ancestor
// resolved instead, in order to still prove containment before ever
// reporting "file missing".
func deepestExistingAncestor(path string) (string, error) {
	cur := path
	for {
		if _, err := os.Lstat(cur); err == nil {
			return cur, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("no existing ancestor found for %q", path)
		}
		cur = parent
	}
}

// resolveContainedRef is the path-safety gate every citation Ref passes
// through before any os.ReadFile: RESOLVED containment, not lexical.
// Rejects an absolute ref outright. Otherwise joins ref to root, resolves
// symlinks, and requires the resolved path to be root or lie beneath it.
// When the joined path does not exist yet (the common "file missing"
// case), EvalSymlinks fails on the missing leaf -- so this instead resolves
// the DEEPEST EXISTING ANCESTOR and checks containment on that, then
// reconstructs a safe (but still nonexistent) path from the resolved
// ancestor plus the remaining suffix, so the caller's read still classifies
// "file missing" rather than the containment check itself erroring out.
//
// A lexical check that rejects only absolute paths and ".." segments does
// NOT uphold "never reads outside the working tree": a symlink inside the
// tree pointing outside it (repo/link -> /etc), followed by the
// perfectly-ordinary-looking relative ref "link/passwd", would read
// /etc/passwd. Citation Ref is client-authored and validated only for
// non-emptiness (internal/server/tools.go), so this is untrusted input
// reaching the filesystem (T-03-04's mitigation).
func resolveContainedRef(root, ref string) (safePath string, rejectReason string) {
	if filepath.IsAbs(ref) {
		return "", "ref is an absolute path, which is not permitted"
	}
	joined := filepath.Join(root, ref)
	if resolved, err := filepath.EvalSymlinks(joined); err == nil {
		if !isContained(root, resolved) {
			return "", "ref resolves outside the working tree"
		}
		return resolved, ""
	}
	ancestor, err := deepestExistingAncestor(joined)
	if err != nil {
		return "", fmt.Sprintf("cannot resolve ref: %v", err)
	}
	resolvedAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", fmt.Sprintf("cannot resolve ref: %v", err)
	}
	if !isContained(root, resolvedAncestor) {
		return "", "ref resolves outside the working tree"
	}
	rest, err := filepath.Rel(ancestor, joined)
	if err != nil {
		return "", fmt.Sprintf("cannot resolve ref: %v", err)
	}
	return filepath.Join(resolvedAncestor, rest), ""
}

// citationFileReader is the injectable file-read hook every verify run
// consults -- overridden by spine_review_verify_test.go so tests can record
// every path read and count calls without touching a real filesystem.
var citationFileReader = func(path string) (content string, exists bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// refResolution caches one Ref's path-safety-plus-read outcome: either
// content/exists (safe to classify) or a non-empty rejectReason (classify
// unverifiable without ever attempting a read).
type refResolution struct {
	content      string
	exists       bool
	rejectReason string
}

// resolveAndRead resolves ref's containment and reads it exactly once,
// combining resolveContainedRef and citationFileReader into a single
// cacheable outcome.
func resolveAndRead(root, ref string) refResolution {
	safePath, rejectReason := resolveContainedRef(root, ref)
	if rejectReason != "" {
		return refResolution{rejectReason: rejectReason}
	}
	content, exists := citationFileReader(safePath)
	return refResolution{content: content, exists: exists}
}

// verifyEntry is one non-valid citation's report row: enough identity to
// find it again (record id, short id, the cited Ref) plus the reason it
// did not classify valid. Deliberately excludes Excerpt -- the report never
// includes stored record content (T-03-14's mitigation).
type verifyEntry struct {
	RecordID string
	ShortID  string
	Ref      string
	Reason   string
}

// verifyReport is verify's aggregated result: tier counts plus a per-entry
// list for every non-valid tier. Pure data -- no *store.Store, no
// context.Context -- so it is the shared value both verifySummary and
// verifyDoc project from, and the value verifyFailOnErr's gate reads.
type verifyReport struct {
	ScannedAt time.Time

	ValidCount        int
	MovedCount        int
	BrokenCount       int
	UnverifiableCount int

	Moved        []verifyEntry
	Broken       []verifyEntry
	Unverifiable []verifyEntry
}

// verifyCitationRecord classifies one citation from rec, applying the
// repo-identity gate first (it applies regardless of citation kind), then
// the four-tier classifier for file citations (via the shared resolution
// cache, so a Ref cited by several records is read at most once).
func verifyCitationRecord(rec store.CitationRecord, idx int, c store.Citation, repoIdentity, root string, cache map[string]refResolution) citationVerdict {
	v := citationVerdict{RecordID: rec.ID, ShortID: rec.ShortID, Index: idx, Ref: c.Ref}
	if !sameRepoAsCWD(rec.Scope, repoIdentity) {
		v.Tier = tierUnverifiable
		v.Reason = "different repo"
		return v
	}
	if c.Kind != "file" {
		fv := verifyFileCitation(c, "", false)
		v.Tier, v.Reason = fv.Tier, fv.Reason
		return v
	}
	res, ok := cache[c.Ref]
	if !ok {
		res = resolveAndRead(root, c.Ref)
		cache[c.Ref] = res
	}
	if res.rejectReason != "" {
		v.Tier = tierUnverifiable
		v.Reason = res.rejectReason
		return v
	}
	fv := verifyFileCitation(c, res.content, res.exists)
	v.Tier, v.Reason = fv.Tier, fv.Reason
	return v
}

// runVerify classifies every citation across records and aggregates the
// result into a verifyReport. Pure over its inputs (records, repoIdentity,
// root, now) -- the caller supplies already-enumerated records, so this
// function itself never dials Qdrant or reads any file directly (it reads
// through citationFileReader via resolveAndRead, which tests override).
func runVerify(records []store.CitationRecord, repoIdentity, root string, now time.Time) verifyReport {
	report := verifyReport{ScannedAt: now}
	cache := make(map[string]refResolution)
	for _, rec := range records {
		for idx, c := range rec.Citations {
			v := verifyCitationRecord(rec, idx, c, repoIdentity, root, cache)
			entry := verifyEntry{RecordID: v.RecordID, ShortID: v.ShortID, Ref: v.Ref, Reason: v.Reason}
			switch v.Tier {
			case tierValid:
				report.ValidCount++
			case tierMoved:
				report.MovedCount++
				report.Moved = append(report.Moved, entry)
			case tierBroken:
				report.BrokenCount++
				report.Broken = append(report.Broken, entry)
			case tierUnverifiable:
				report.UnverifiableCount++
				report.Unverifiable = append(report.Unverifiable, entry)
			}
		}
	}
	return report
}

// verifyEntryDoc is one verifyEntry's JSON-mode shape.
type verifyEntryDoc struct {
	RecordID string `json:"record_id"`
	ShortID  string `json:"short_id"`
	Ref      string `json:"ref"`
	Reason   string `json:"reason"`
}

// verifyReportDoc is the JSON-mode report's exported-field shape: counts,
// ids, refs, and reasons only -- NEVER the Excerpt or any other stored
// record content, on any spine-review output path (T-03-14's mitigation).
// A hand-declared struct, not an embedded verifyReport, so this exclusion
// is enforced by the type itself. Under D-01 (06-CONTEXT.md) this struct
// is the SOLE serialization -- the text lane renders only what this
// struct marshals -- so the Excerpt exclusion this type enforces now
// bounds the text lane too.
type verifyReportDoc struct {
	ScannedAt           time.Time        `json:"scanned_at"`
	Valid               int              `json:"valid"`
	Moved               int              `json:"moved"`
	Broken              int              `json:"broken"`
	Unverifiable        int              `json:"unverifiable"`
	MovedEntries        []verifyEntryDoc `json:"moved_entries"`
	BrokenEntries       []verifyEntryDoc `json:"broken_entries"`
	UnverifiableEntries []verifyEntryDoc `json:"unverifiable_entries"`
}

// verifyDoc converts report into verifyReportDoc, keeping every entry slice
// non-nil so an empty tier marshals it as "[]", never "null".
func verifyDoc(report verifyReport) verifyReportDoc {
	toDocs := func(entries []verifyEntry) []verifyEntryDoc {
		out := make([]verifyEntryDoc, 0, len(entries))
		for _, e := range entries {
			out = append(out, verifyEntryDoc(e))
		}
		return out
	}
	return verifyReportDoc{
		ScannedAt:           report.ScannedAt,
		Valid:               report.ValidCount,
		Moved:               report.MovedCount,
		Broken:              report.BrokenCount,
		Unverifiable:        report.UnverifiableCount,
		MovedEntries:        toDocs(report.Moved),
		BrokenEntries:       toDocs(report.Broken),
		UnverifiableEntries: toDocs(report.Unverifiable),
	}
}

// verifySummary renders the operator-facing headline. Pure (value types
// only) so it is unit-testable without a live Qdrant or filesystem,
// mirroring spineScanSummary's discipline. Per D-04 (06-CONTEXT.md) this
// is a headline producer only: the scan instant plus the four tier
// counts, returned as a single line with no trailing newline. The
// moved/broken/unverifiable entry rows this function used to also print
// now render only as renderOperatorView's field-table rows, from
// verifyDoc.
func verifySummary(report verifyReport) string {
	return fmt.Sprintf("spine verify at %s: valid=%d moved=%d broken=%d unverifiable=%d",
		report.ScannedAt.Format(time.RFC3339), report.ValidCount, report.MovedCount, report.BrokenCount, report.UnverifiableCount)
}

// verifyFailOnValues is the accepted enum for --fail-on -- the four tier
// names plus "any". Declared once here so both validateFailOn and any
// future caller enumerate the same set rather than a second hand-typed
// copy.
var verifyFailOnValues = []string{tierBroken, tierMoved, tierUnverifiable, "any"}

// validateFailOn rejects v unless it is empty (the default -- see D-14) or
// one of verifyFailOnValues. Returns a usageErrorf naming both --fail-on
// and the accepted values, per the registered
// surfaces.RuleVerifyFailOnValues rule's Sentence, so an illegal value
// exits exitUsage through Phase 1's taxonomy.
func validateFailOn(v string) error {
	if v == "" {
		return nil
	}
	for _, ok := range verifyFailOnValues {
		if v == ok {
			return nil
		}
	}
	rule, ruleOK := surfaces.RuleByID(surfaces.RuleVerifyFailOnValues)
	sentence := "fail-on accepts broken, moved, unverifiable, or any"
	if ruleOK {
		sentence = rule.Sentence
	}
	return usageErrorf("--fail-on %q is invalid: %s", v, sentence)
}

// verifyFailOnErr is the D-14 gate: exit 0 (nil) unless failOn names a tier
// with at least one entry, in which case it returns a *cliError carrying
// exitFindings -- distinguishable from every store/transport error code in
// the taxonomy. Pure over report + failOn, so this is unit-testable via
// exitCodeFromError with no live store and no filesystem access.
func verifyFailOnErr(report verifyReport, failOn string) error {
	var (
		tier  string
		count int
	)
	switch failOn {
	case "":
		return nil
	case tierBroken:
		tier, count = tierBroken, report.BrokenCount
	case tierMoved:
		tier, count = tierMoved, report.MovedCount
	case tierUnverifiable:
		tier, count = tierUnverifiable, report.UnverifiableCount
	case "any":
		tier = "any"
		count = report.MovedCount + report.BrokenCount + report.UnverifiableCount
	default:
		// validateFailOn already rejects every other value before this
		// function is ever reached; unreachable in practice.
		return nil
	}
	if count == 0 {
		return nil
	}
	return &cliError{code: exitFindings, err: fmt.Errorf("--fail-on %s: %d %s citation(s) found", failOn, count, tier)}
}

var (
	spineVerifyScope     string
	spineVerifyAllScopes bool
	spineVerifyTimeout   time.Duration
	spineVerifyOutput    string
	spineVerifyFailOn    string
)

// spineReviewVerifyCmd classifies every stored citation into one of four
// tiers (valid/moved/broken/unverifiable), bounded to the file a citation
// names and to the repo the command runs in. Read-only by construction --
// it never issues a mutating Qdrant RPC -- and Subject-less like every
// other operator-tier command on this binary. Exits 0 by default even when
// it reports broken citations (D-14): --fail-on is the opt-in that turns a
// named tier's findings into a nonzero (exitFindings) exit, for CI.
var spineReviewVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Classify every stored citation as valid, moved, broken, or unverifiable",
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Mirrors spine-review scan's exact wording (spine_review_scan.go).
		if spineVerifyScope == "" && !spineVerifyAllScopes {
			return usageErrorf("--scope <scope> or --all-scopes is required")
		}
		format, err := operatorOutputFormat(cmd, spineVerifyOutput)
		if err != nil {
			return err
		}
		if err := validateFailOn(spineVerifyFailOn); err != nil {
			return err
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if spineVerifyTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, spineVerifyTimeout)
			defer cancel()
		}
		records, err := st.EnumerateCitations(ctx, store.SpineScanOptions{Scope: spineVerifyScope})
		if err != nil {
			return classifyOperatorErr(err)
		}
		root, err := verifyWorkingRoot()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		repoIdentity, err := repoIdentityFromCWD()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		report := runVerify(records, repoIdentity, root, time.Now().UTC())
		if err := renderOperator(cmd, format, verifySummary(report), verifyDoc(report)); err != nil {
			return err
		}
		return verifyFailOnErr(report, spineVerifyFailOn)
	},
}

func init() {
	addOperatorOutputFlag(spineReviewVerifyCmd, &spineVerifyOutput)
	spineReviewVerifyCmd.Flags().StringVar(&spineVerifyScope, "scope", "", "only verify citations from records in this scope")
	spineReviewVerifyCmd.Flags().BoolVar(&spineVerifyAllScopes, "all-scopes", false, "sweep every scope (required if --scope is omitted)")
	spineReviewVerifyCmd.Flags().DurationVar(&spineVerifyTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
	rule, ok := surfaces.RuleByID(surfaces.RuleVerifyFailOnValues)
	if !ok {
		panic("spine-review verify: surfaces.RuleVerifyFailOnValues is not registered in internal/surfaces/rules.go")
	}
	spineReviewVerifyCmd.Flags().StringVar(&spineVerifyFailOn, "fail-on", "", rule.Sentence)
	spineReviewCmd.AddCommand(spineReviewVerifyCmd)
}
