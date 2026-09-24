// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// verdictProbabilitiesView decodes the D-05 five-key probability
// distribution keeping every value as json.RawMessage — the exact digit
// text the JSON lane already emitted, never re-formatted through float64
// (CUR-02 precision on the text lane).
type verdictProbabilitiesView struct {
	Duplicate   json.RawMessage `json:"duplicate"`
	Contradicts json.RawMessage `json:"contradicts"`
	Updates     json.RawMessage `json:"updates"`
	Related     json.RawMessage `json:"related"`
	Unrelated   json.RawMessage `json:"unrelated"`
}

// probabilityFor returns relation's own raw probability text from p, or the
// empty string when relation does not match any of the five D-05 names —
// renderVerdictView then renders "p=" with no value and no error, never a
// lookup failure.
func probabilityFor(relation string, p verdictProbabilitiesView) string {
	switch relation {
	case "duplicate":
		return string(p.Duplicate)
	case "contradicts":
		return string(p.Contradicts)
	case "updates":
		return string(p.Updates)
	case "related":
		return string(p.Related)
	case "unrelated":
		return string(p.Unrelated)
	default:
		return ""
	}
}

// verdictView decodes one candidate's advisory verdict JSON — the D-05
// object consolidateVerdictDoc's two-shape union marshals — for text
// rendering (D-07, D-10). Probabilities and SameSubject stay
// json.RawMessage/verdictProbabilitiesView so every number is the JSON
// lane's own verbatim digit text, never round-tripped through float64.
type verdictView struct {
	Relation      string                   `json:"relation"`
	Probabilities verdictProbabilitiesView `json:"probabilities"`
	SameSubject   json.RawMessage          `json:"same_subject"`
	NeedsReview   bool                     `json:"needs_review"`
	Model         string                   `json:"model"`
	Error         string                   `json:"error"`
}

// renderVerdictView is the text rendering of the D-05 verdict object (D-07,
// D-10): registered below as the "verdict" row-field renderer
// (registerRowFieldRenderer, operator_view.go), so it is fed the SAME bytes
// the JSON lane emits — text stays a view of the JSON contract (D-05 of
// this plan). A non-empty Error renders "verdict unavailable (<class>)"
// alone. Otherwise: "verdict=<relation> p=<probability>", " [needs review]"
// when NeedsReview is true, then the same_subject probability, the full
// five-key probability distribution, and the model — all in that fixed
// order. Sanitization is NOT this function's job: viewRow sanitizes every
// renderer's returned string via sanitizeViewValue before it reaches
// output, so this function returns text that may still carry an
// attacker-controlled VALUE (a provider's model string, an error class),
// just never attacker-controlled report STRUCTURE.
func renderVerdictView(raw json.RawMessage) (string, error) {
	var v verdictView
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", fmt.Errorf("operator view: decode verdict: %w", err)
	}
	if v.Error != "" {
		return fmt.Sprintf("verdict unavailable (%s)", v.Error), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "verdict=%s p=%s", v.Relation, probabilityFor(v.Relation, v.Probabilities))
	if v.NeedsReview {
		b.WriteString(" [needs review]")
	}
	fmt.Fprintf(&b, " same_subject=%s probabilities=duplicate:%s,contradicts:%s,updates:%s,related:%s,unrelated:%s model=%s",
		string(v.SameSubject),
		string(v.Probabilities.Duplicate), string(v.Probabilities.Contradicts), string(v.Probabilities.Updates),
		string(v.Probabilities.Related), string(v.Probabilities.Unrelated),
		v.Model)
	return b.String(), nil
}

func init() {
	registerRowFieldRenderer("verdict", renderVerdictView)
}
