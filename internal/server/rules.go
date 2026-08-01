// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
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

type storeRuleArgs struct {
	Content string   `json:"content" jsonschema:"the full rule text (normative constraint agents must follow)"`
	Scope   string   `json:"scope" jsonschema:"rule scope: rule:repo:<repo> or rule:project:<project>"`
	Summary string   `json:"summary" jsonschema:"REQUIRED one-line index entry (single physical line, no newlines)"`
	Tags    []string `json:"tags,omitempty" jsonschema:"concern-area labels e.g. vcs, deploy, authz"`
	ID      string   `json:"id,omitempty" jsonschema:"omit to create; supply to replace in place"`
}

type listRulesArgs struct {
	Scopes []string `json:"scopes" jsonschema:"one or more rule:* scopes to fetch the complete rule set from"`
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

// listRules returns the complete rule set across the given rule:* scopes,
// oldest-first, as compact ruleView values (or full store.Memory when full).
// The second return is a human-readable curation advisory for the tool's
// textResult (empty when under threshold); it never changes the {rules} payload.
func (d *deps) listRules(ctx context.Context, c caller, a listRulesArgs) (out []any, advisory string, err error) {
	if len(a.Scopes) == 0 {
		return nil, "", argErrf(classMalformed, HintRequired, "scopes", "at least one rule scope is required")
	}
	for i, sc := range a.Scopes {
		if !validRuleScope(sc) {
			// Field stays the plain "scopes" (never "scopes[i]" or the offending
			// value) — D-12 and the matrix asserts on the identifier, not a
			// per-element value. The position is useful detail text, not a value.
			return nil, "", argErrf(classMalformed, HintPrefix, "scopes", "scope at position %d must be rule:repo:<repo> or rule:project:<project>", i)
		}
	}
	var over []string
	for _, sc := range a.Scopes {
		// Limit:0 = all; Ascending = oldest-first; Categories pins the rule kind.
		ms, _, _, lerr := d.st.List(ctx, sc, c.Subj, store.ListOptions{
			Limit:      0,
			Ascending:  true,
			Categories: []string{"rule"},
			Tags:       a.Tags,
		})
		if lerr != nil {
			return nil, "", lerr
		}
		if len(ms) > ruleThreshold {
			over = append(over, fmt.Sprintf("%d rules in %s", len(ms), sc))
		}
		for _, m := range ms {
			if a.Full {
				out = append(out, m)
			} else {
				out = append(out, toRuleView(m))
			}
		}
	}
	if len(over) > 0 {
		advisory = "curation smell — " + strings.Join(over, "; ") + " — consider consolidating"
	}
	return out, advisory, nil
}
