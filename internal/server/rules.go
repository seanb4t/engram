// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"
	"strings"

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

// ensure the store import is used even before handlers land (removed once
// storeRule/listRules reference store types in later tasks).
var _ = store.SummarySourceClient
