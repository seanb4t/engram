// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		return fmt.Errorf("content is required")
	}
	if len(a.Content) > maxRuleContentBytes {
		return fmt.Errorf("content too large: %d bytes (max %d)", len(a.Content), maxRuleContentBytes)
	}
	if a.Scope == "" {
		return fmt.Errorf("scope is required")
	}
	if !validRuleScope(a.Scope) {
		return fmt.Errorf("scope must be rule:repo:<repo> or rule:project:<project>, got %q", a.Scope)
	}
	if err := validateRuleSummary(a.Summary); err != nil {
		return err
	}
	return nil
}

// validateRuleSummary enforces the shared summary contract for rules: non-empty,
// single physical line, within the byte cap. Reused by the update_memory guard.
func validateRuleSummary(summary string) error {
	if summary == "" {
		return fmt.Errorf("summary is required for a rule (it is the one-line index entry)")
	}
	if strings.ContainsAny(summary, "\r\n") {
		return fmt.Errorf("rule summary must be a single line (no newlines); it is the index entry")
	}
	if len(summary) > maxRuleSummaryBytes {
		return fmt.Errorf("summary too long: %d bytes (max %d)", len(summary), maxRuleSummaryBytes)
	}
	return nil
}

// storeRule persists a normative rule, mirroring storeDiscovery: it resolves
// and validates ownership for an in-place replace (a.ID set), mints or
// carries forward the short_id, and returns (id, short_id, error).
func (d *deps) storeRule(ctx context.Context, a storeRuleArgs) (string, string, error) {
	if err := validateStoreRule(a); err != nil {
		return "", "", err
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
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
		if err := d.st.OwnedOrAbsent(ctx, pointID, subj); err != nil {
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
		ID:            id,
		ShortID:       shortID,
		Content:       a.Content,
		Scope:         a.Scope,
		Source:        "user-said",
		Category:      "rule",
		Visibility:    "shared",
		Tags:          a.Tags,
		Summary:       a.Summary,
		SummarySource: store.SummarySourceClient,
		Actor:         actorFromContext(ctx),
		Owner:         subj.Owner(),
		CreatedAt:     d.clock(),
	}
	return m.ID, m.ShortID, d.st.Upsert(ctx, m, vec)
}
