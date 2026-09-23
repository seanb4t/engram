// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package jev

import (
	"encoding/json"
	"fmt"

	"github.com/seanb4t/engram/internal/decide"
)

// wireRequest, wireQuestion, wireResponse, wireAnswer and wireUsage are the
// unexported wire structs for the Decisions API (D-06 reject-hand-write
// branch): plain net/http + encoding/json, no generated SDK types.
type wireRequest struct {
	Model     string                  `json:"model"`
	State     map[string]any          `json:"state"`
	Questions map[string]wireQuestion `json:"questions"`
}

// wireQuestion's Criteria is `any` rather than a fixed type because its wire
// shape depends on the question type: a noul or choice question sends an
// object, a score question sends an ordered array. encodeRequest sets the
// concrete value; encoding/json renders a map as an object and a slice as an
// array with no further help needed.
type wireQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria"`
}

type wireResponse struct {
	Model   string                `json:"model"`
	Answers map[string]wireAnswer `json:"answers"`
	// Usage is a pointer so an absent "usage" key decodes to nil — E10
	// requires distinguishing "no usage reported" from "usage reported as
	// zero", and both attempt's Response.Usage and the decide span's usage
	// attributes must omit rather than zero in the absent case.
	Usage    *wireUsage `json:"usage"`
	ID       string     `json:"id"`
	Provider string     `json:"provider"`
}

// wireAnswer is the union of every answer shape the Decisions API returns,
// discriminated by Type. Confidence and Legend are pointers/maps left nil by
// json.Unmarshal when the provider omits them (E03/E10): never zeroed, never
// defaulted.
type wireAnswer struct {
	Type          string                     `json:"type"`
	Noul          float64                    `json:"noul"`
	Choice        string                     `json:"choice"`
	Score         float64                    `json:"score"`
	Confidence    *float64                   `json:"confidence"`
	Probabilities map[string]float64         `json:"probabilities"`
	Legend        map[string]json.RawMessage `json:"legend"`
}

type wireUsage struct {
	InputTokens  int64    `json:"input_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	Cost         *float64 `json:"cost"`
}

// encodeRequest builds the JSON body Decide sends to {base}/alpha/decisions
// for model and req (DEC-02): a noul question's criteria is an object with
// keys "true"/"false", a choice question's criteria is its Options map
// verbatim, and a score question's criteria is its Scale slice in order.
// req.Validate is expected to have already rejected any question whose Type
// is not one of these three — the default case below is a defensive
// backstop, not a reachable path from Decide.
func encodeRequest(model string, req decide.Request) ([]byte, error) {
	wireQ := make(map[string]wireQuestion, len(req.Questions))
	for name, q := range req.Questions {
		wq := wireQuestion{Type: string(q.Type), Instructions: q.Instructions}
		switch q.Type {
		case decide.QuestionNoul:
			wq.Criteria = map[string]string{"true": q.WhenTrue, "false": q.WhenFalse}
		case decide.QuestionChoice:
			wq.Criteria = q.Options
		case decide.QuestionScore:
			wq.Criteria = q.Scale
		default:
			return nil, fmt.Errorf("decide: unsupported question type %q for question %q", q.Type, name)
		}
		wireQ[name] = wq
	}
	return json.Marshal(wireRequest{
		Model:     model,
		State:     map[string]any(req.State),
		Questions: wireQ,
	})
}

// decodeResponse decodes body into a decide.Response for every question
// req.Questions names (DEC-02). An undecodable body, a missing answer for a
// requested question, or an answer whose Type is not noul/choice/score
// returns a *decide.Error{Kind: decide.ErrDecisionMalformedResponse} — the
// last two name the offending question. Answers for names not in
// req.Questions are dropped: a provider that answers a question nobody
// asked never leaks it into Response.Answers. Every field is copied
// verbatim — never rounded, clamped or renormalized (E03).
func decodeResponse(body []byte, req decide.Request) (decide.Response, error) {
	var wr wireResponse
	if err := json.Unmarshal(body, &wr); err != nil {
		return decide.Response{}, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Err: err}
	}

	answers := make(map[string]decide.Answer, len(req.Questions))
	for name := range req.Questions {
		wa, ok := wr.Answers[name]
		if !ok {
			return decide.Response{}, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: name}
		}
		switch wa.Type {
		case string(decide.QuestionNoul):
			answers[name] = decide.Answer{Type: decide.QuestionNoul, Probability: wa.Noul}
		case string(decide.QuestionChoice):
			answers[name] = decide.Answer{
				Type:          decide.QuestionChoice,
				Choice:        wa.Choice,
				Probabilities: wa.Probabilities,
				Confidence:    wa.Confidence,
			}
		case string(decide.QuestionScore):
			answers[name] = decide.Answer{
				Type:          decide.QuestionScore,
				Score:         wa.Score,
				Probabilities: wa.Probabilities,
				Confidence:    wa.Confidence,
				Legend:        wa.Legend,
			}
		default:
			return decide.Response{}, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: name}
		}
	}

	var usage *decide.Usage
	if wr.Usage != nil {
		usage = &decide.Usage{
			InputTokens:  wr.Usage.InputTokens,
			OutputTokens: wr.Usage.OutputTokens,
			CostUSD:      wr.Usage.Cost,
		}
	}

	return decide.Response{
		Answers:  answers,
		Model:    wr.Model,
		ID:       wr.ID,
		Provider: wr.Provider,
		Usage:    usage,
	}, nil
}
