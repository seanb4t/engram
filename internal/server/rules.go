// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/seanb4t/engram/internal/store"
)

// Rule size bounds. content is generous full rule text; summary is the one-line
// index entry, capped so one rule ≈ one terminal line in the session-start index.
const (
	maxRuleContentBytes = 8 * 1024 // 8 KiB full rule text
	maxRuleSummaryBytes = 256      // one physical index line
)

// storeRuleArgs' Content/Scope/Summary carry omitempty (D-06a). All three
// already had a Go-level presence check in validateStoreRule/
// validateRuleSummary before this plan — those checks were SHADOWED by the
// schema's own required-ness; relaxing the tag makes them reachable for the
// first time. No new validation logic needed here.
type storeRuleArgs struct {
	Content string   `json:"content,omitempty" jsonschema:"the full rule text (normative constraint agents must follow)"`
	Scope   string   `json:"scope,omitempty" jsonschema:"rule scope: rule:repo:<repo> or rule:project:<project>"`
	Summary string   `json:"summary,omitempty" jsonschema:"REQUIRED one-line index entry (single physical line, no newlines)"`
	Tags    []string `json:"tags,omitempty" jsonschema:"concern-area labels e.g. vcs, deploy, authz"`
	ID      string   `json:"id,omitempty" jsonschema:"omit to create; supply to replace in place"`
}

// listRulesArgs.Scopes carries omitempty (D-06a). Unlike before this plan, an
// empty/omitted Scopes is no longer rejected: it is the all-scopes read
// (D-10, milestone 2026-09-25.01 Phase 3) — listRuleRecords' per-entry
// validRuleScope loop still rejects a blank or non-rule ENTRY within a
// non-empty list.
type listRulesArgs struct {
	Scopes []string `json:"scopes,omitempty" jsonschema:"rule:* scopes to fetch the complete rule set from; omit or leave empty for every readable rule scope"`
	Tags   []string `json:"tags,omitempty" jsonschema:"restrict to rules carrying ALL listed tags (AND)"`
	Full   bool     `json:"full,omitempty" jsonschema:"true adds full content; default returns the compact index shape"`
}

// validRuleScope reports whether s is a well-formed rule scope: rule:repo:<tail>
// or rule:project:<tail> with a non-empty tail.
func validRuleScope(s string) bool {
	for _, prefix := range []string{"rule:repo:", "rule:project:"} {
		if strings.HasPrefix(s, prefix) && len(s) > len(prefix) {
			return true
		}
	}
	return false
}

// validateStoreRule enforces the store_rule contract without touching Qdrant:
// rule scope prefix, required content within the byte cap, and a required
// single-line summary within the byte cap. Newlines in the summary are rejected
// (never normalized): the summary is the index line and munging user input hides
// the problem (explicit/correctable ethos).
func validateStoreRule(a storeRuleArgs) error {
	if a.Content == "" {
		return argErrf(classMalformed, HintRequired, "content", "content is required")
	}
	if len(a.Content) > maxRuleContentBytes {
		return argErrf(classOutOfRange, HintTooLong, "content", "content too large: %d bytes (max %d)", len(a.Content), maxRuleContentBytes)
	}
	if a.Scope == "" {
		return argErrf(classMalformed, HintRequired, "scope", "scope is required")
	}
	if !validRuleScope(a.Scope) {
		return argErrf(classMalformed, HintPrefix, "scope", "scope must be rule:repo:<repo> or rule:project:<project>")
	}
	if err := validateRuleSummary(a.Summary); err != nil {
		return err
	}
	return nil
}

// validateRuleSummary enforces the shared summary contract for rules: non-empty,
// single physical line, within the byte cap. Reused by the update_memory guard.
// Every rejection carries the field+hint argError envelope (D-05/D-09); its
// Unwrap() supplies store.ErrInvalidArgument, so every existing
// errors.Is(err, store.ErrInvalidArgument) consumer and connectError's
// *argError case both keep working with no separate sentinel wrap needed here.
func validateRuleSummary(summary string) error {
	if summary == "" {
		return argErrf(classMalformed, HintRequired, "summary", "summary is required for a rule (it is the one-line index entry)")
	}
	if strings.ContainsAny(summary, "\r\n") {
		return argErrf(classMalformed, HintFormat, "summary", "rule summary must be a single line (no newlines); it is the index entry")
	}
	if len(summary) > maxRuleSummaryBytes {
		return argErrf(classOutOfRange, HintTooLong, "summary", "summary too long: %d bytes (max %d)", len(summary), maxRuleSummaryBytes)
	}
	return nil
}

// storeRule persists a normative rule, mirroring storeDiscovery: it resolves
// and validates ownership for an in-place replace (a.ID set), mints or
// carries forward the short_id, and returns (id, short_id, error).
func (d *deps) storeRule(ctx context.Context, c caller, a storeRuleArgs) (string, string, error) {
	if err := validateStoreRule(a); err != nil {
		return "", "", err
	}

	pointID := ""        // resolved UUID for replace; "" for a fresh create
	carriedShortID := "" // existing handle to preserve across replace
	if a.ID != "" {
		resolved, rerr := d.st.ResolvePointID(ctx, a.ID)
		if rerr != nil {
			return "", "", rerr
		}
		pointID = resolved
		if err := d.st.OwnedOrAbsent(ctx, pointID, c.Subj); err != nil {
			// Re-wrap not-found with the caller's ORIGINAL input: pointID may be
			// another owner's record resolved from their short id, and echoing the
			// resolved UUID would leak existence/identity (404-indistinguishability).
			if errors.Is(err, store.ErrNotFound) {
				return "", "", fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
			}
			return "", "", err
		}
		if existing, gerr := d.st.Get(ctx, pointID); gerr == nil {
			carriedShortID = existing.ShortID
		} else if !errors.Is(gerr, store.ErrNotFound) {
			return "", "", gerr
		}
	}

	vec, err := d.em.Embed(ctx, store.EmbedText(a.Content, a.Tags))
	if err != nil {
		return "", "", err
	}
	id := pointID
	if id == "" {
		id = uuid.NewString()
	}
	shortID := carriedShortID
	if shortID == "" {
		if shortID, err = d.st.MintShortID(ctx, nil); err != nil {
			return "", "", err
		}
	}
	m := store.Memory{
		ID:               id,
		ShortID:          shortID,
		Content:          a.Content,
		Scope:            a.Scope,
		Source:           "user-said",
		Category:         "rule",
		Visibility:       "shared",
		Tags:             a.Tags,
		Summary:          a.Summary,
		SummarySource:    store.SummarySourceClient,
		Actor:            c.Actor,
		Owner:            c.Subj.Owner(),
		CreatedAt:        d.clock(),
		EmbedderIdentity: d.embedderIdentity,
	}
	return m.ID, m.ShortID, d.st.Upsert(ctx, m, vec)
}

// ruleThreshold is the soft rule-count ceiling per scope above which listRules
// returns a curation-smell advisory (textResult only; the {rules} payload is
// unaffected). A rule set is definitionally small.
const ruleThreshold = 50

// ruleView is the compact list_rules result: the one-line index entry plus the
// short handle callers paste into get_memory.
type ruleView struct {
	ShortID   string    `json:"short_id,omitempty"`
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	Tags      []string  `json:"tags,omitempty"`
	Scope     string    `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
}

func toRuleView(m store.Memory) ruleView {
	return ruleView{
		ShortID: m.ShortID, ID: m.ID, Summary: m.Summary,
		Tags: m.Tags, Scope: m.Scope, CreatedAt: m.CreatedAt,
	}
}

// listRuleRecords is the shared raw core behind listRules (MCP shaping,
// below) and the Connect ListRules RPC (D-10, milestone 2026-09-25.01 Phase
// 3): with one or more explicit scopes it behaves exactly as before this
// plan — validate each scope, then a per-scope Store.List loop, appending
// each scope's records in the order supplied, oldest-first within each scope.
// With an EMPTY scopes list it performs ONE cross-scope Store.List — an empty
// scope spans every scope the caller may read (internal/store.listFilter) —
// bounded at store.MaxRecallLimit records IN TOTAL (never per scope), oldest-
// first across the whole result. The second return is the curation advisory
// (ruleAdvisory); it never changes which rules are returned.
func (d *deps) listRuleRecords(ctx context.Context, c caller, a listRulesArgs) ([]store.Memory, string, error) {
	for i, sc := range a.Scopes {
		if !validRuleScope(sc) {
			// Field stays the plain "scopes" (never "scopes[i]" or the offending
			// value) — D-12 and the matrix asserts on the identifier, not a
			// per-element value. The position is useful detail text, not a value.
			return nil, "", argErrf(classMalformed, HintPrefix, "scopes", "scope at position %d must be rule:repo:<repo> or rule:project:<project>", i)
		}
	}

	// Full is copied straight from a.Full: this is the ONE list caller
	// outside the typed core (coreListRequest is never built here —
	// 04-RESEARCH.md Pattern 6, step 4), so it is invisible to
	// deps.listMemory's own Full wiring and must thread it itself, or a
	// full=true rule read would silently regress to summary-shaped
	// (no-content) records once the store's default fetch became
	// summary-shaped (04-05).
	var ms []store.Memory
	if len(a.Scopes) == 0 {
		// D-10: one cross-scope read. Limit:0 resolves to store.MaxRecallLimit
		// at the store (D-01/D-03, plans 04-02/04-06) — up to the documented
		// maximum IN TOTAL, never literally "all" and never internally paged
		// past it.
		got, _, _, err := d.st.List(ctx, "", c.Subj, store.ListOptions{
			Limit:      0,
			Ascending:  true,
			Categories: []string{"rule"},
			Tags:       a.Tags,
			Full:       a.Full,
		})
		if err != nil {
			return nil, "", err
		}
		ms = got
	} else {
		for _, sc := range a.Scopes {
			// Limit:0 resolves to store.MaxRecallLimit at the store — the
			// complete rule set up to the documented maximum PER SCOPE, never
			// internally paged past it.
			got, _, _, err := d.st.List(ctx, sc, c.Subj, store.ListOptions{
				Limit:      0,
				Ascending:  true,
				Categories: []string{"rule"},
				Tags:       a.Tags,
				Full:       a.Full,
			})
			if err != nil {
				return nil, "", err
			}
			ms = append(ms, got...)
		}
	}
	return ms, ruleAdvisory(ms), nil
}

// ruleAdvisory renders the curation-smell advisory over ms: counts rules per
// scope (scopes sorted for a deterministic rendering across the all-scopes
// read), and names every scope whose count exceeds ruleThreshold. For a
// single-scope-over-threshold call (the only shape any existing test
// exercises) this is byte-identical to the text the pre-D-10 inline
// computation produced.
func ruleAdvisory(ms []store.Memory) string {
	counts := make(map[string]int)
	for _, m := range ms {
		counts[m.Scope]++
	}
	scopes := make([]string, 0, len(counts))
	for sc := range counts {
		scopes = append(scopes, sc)
	}
	sort.Strings(scopes)

	var over []string
	for _, sc := range scopes {
		if counts[sc] > ruleThreshold {
			over = append(over, fmt.Sprintf("%d rules in %s", counts[sc], sc))
		}
	}
	if len(over) == 0 {
		return ""
	}
	return "curation smell — " + strings.Join(over, "; ") + " — consider consolidating"
}

// ruleScopeCoverage is searchedScopes filtered to rule:* scopes (D-10,
// T-03-18): the all-scopes ListRules/list_rules read must report ONLY the
// rule scopes it covered, never a non-rule scope the same caller happens to
// be able to read. When allScopes is false this returns the zero-value
// scopeCoverage exactly like searchedScopes(ctx, c, false) — no ListScopes
// call, no coverage keys on the wire (byte-identical to an explicit-scope
// call). When the underlying coverage query failed (cov.Unknown), the value
// passes through unchanged: Unknown stays true and Scopes stays nil, so the
// caller reports scopes_unknown rather than a filtered (and therefore
// misleading) empty list.
func (d *deps) ruleScopeCoverage(ctx context.Context, c caller, allScopes bool) scopeCoverage {
	cov := d.searchedScopes(ctx, c, allScopes)
	if !allScopes || cov.Unknown {
		return cov
	}
	// Freshly allocated, non-nil even when empty (D-10): an all-scopes call
	// with no readable rule scope must render searched_scopes as an empty
	// list, not an absent/null key.
	scopes := make([]string, 0, len(cov.Scopes))
	for _, sc := range cov.Scopes {
		if validRuleScope(sc) {
			scopes = append(scopes, sc)
		}
	}
	cov.Scopes = scopes
	return cov
}

// listRules is the MCP shaping wrapper over listRuleRecords (D-10): the
// pre-D-10 empty-scopes rejection is gone — an empty/omitted Scopes list is
// now the all-scopes read, delegated to listRuleRecords exactly like an
// explicit-scope call. Compact ruleView values are returned by default; full
// store.Memory records when a.Full is set.
func (d *deps) listRules(ctx context.Context, c caller, a listRulesArgs) (out []any, advisory string, err error) {
	ms, advisory, err := d.listRuleRecords(ctx, c, a)
	if err != nil {
		return nil, "", err
	}
	for _, m := range ms {
		if a.Full {
			out = append(out, m)
		} else {
			out = append(out, toRuleView(m))
		}
	}
	return out, advisory, nil
}
