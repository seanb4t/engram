// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package jev

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"

	"github.com/seanb4t/engram/internal/decide"
)

// errorEnvelope is OpenRouter's and LiteLLM's shared error body shape:
// {"error":{"message":"...","code":<n or "n">}}. Code is decoded as
// json.RawMessage so OpenRouter's numeric code and LiteLLM's string code
// both unmarshal without error — Code is never read for classification
// (D-12): only the HTTP status decides the class.
type errorEnvelope struct {
	Error struct {
		Message string          `json:"message"`
		Code    json.RawMessage `json:"code"`
	} `json:"error"`
}

// detailEnvelope is the shape max_tokens_exceeded is reported in, either as
// the whole response body or embedded as a JSON document inside
// errorEnvelope.Error.Message after an "HTTP <status>: " prefix.
type detailEnvelope struct {
	Detail struct {
		ErrorType string `json:"error_type"`
	} `json:"detail"`
}

// classifyStatus maps a non-200 status and its response body to a
// *decide.Error carrying one of the D-12 classes, by HTTP status alone: the
// error body's `code` field is decoded but never inspected, and the message
// text is decoded structurally for context-too-large detection only — never
// substring-matched for control flow. Detail is error.message when the body
// decodes as errorEnvelope, else the trimmed raw body, bounded to
// maxErrorBodyBytes either way.
func classifyStatus(status int, body []byte) error {
	var env errorEnvelope
	var message string
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
		message = env.Error.Message
	}
	detail := message
	if detail == "" {
		detail = strings.TrimSpace(string(body))
	}
	if len(detail) > maxErrorBodyBytes {
		detail = detail[:maxErrorBodyBytes]
	}

	var kind error
	switch {
	case status == 401 || status == 402 || status == 403:
		kind = decide.ErrDecisionAuth
	case status == 400:
		if contextTooLarge(body, message) {
			kind = decide.ErrDecisionContextTooLarge
		} else {
			kind = decide.ErrDecisionBadRequest
		}
	case status == 413:
		kind = decide.ErrDecisionContextTooLarge
	case status == 429:
		kind = decide.ErrDecisionRateLimited
	case status >= 500:
		kind = decide.ErrDecisionUnavailable
	case status >= 400:
		kind = decide.ErrDecisionBadRequest
	default:
		kind = decide.ErrDecisionUnavailable
	}
	return &decide.Error{Kind: kind, Status: status, Detail: detail}
}

// contextTooLarge reports whether body, or message, structurally encodes
// {"detail":{"error_type":"max_tokens_exceeded"}} — the single structured
// exception D-12 allows. message is checked from its first '{' onward (the
// OpenRouter dialect embeds this document inside error.message after an
// "HTTP <status>: " prefix); message is never substring-matched for the
// words themselves, only decoded as JSON once a '{' is located.
func contextTooLarge(body []byte, message string) bool {
	var d detailEnvelope
	if json.Unmarshal(body, &d) == nil && d.Detail.ErrorType == "max_tokens_exceeded" {
		return true
	}
	if i := strings.IndexByte(message, '{'); i >= 0 {
		var md detailEnvelope
		if json.Unmarshal([]byte(message[i:]), &md) == nil && md.Detail.ErrorType == "max_tokens_exceeded" {
			return true
		}
	}
	return false
}

// classifyTransport maps a transport-level failure (no HTTP response
// received) to a *decide.Error, or passes a caller cancellation through
// unchanged so errors.Is(err, context.Canceled) matches with no
// ErrDecision* class attached.
func classifyTransport(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
		return err
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return &decide.Error{Kind: decide.ErrDecisionTimeout, Err: err}
	}
	return &decide.Error{Kind: decide.ErrDecisionUnavailable, Err: err}
}

// isRetryable reports whether err is one of D-11's two retryable classes.
func isRetryable(err error) bool {
	return errors.Is(err, decide.ErrDecisionRateLimited) || errors.Is(err, decide.ErrDecisionUnavailable)
}
