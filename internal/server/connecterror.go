// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/seanb4t/engram/internal/store"
)

// connectError is the SINGLE production mapper from a deps.* business error to
// a Connect error code (17-04). It matches TYPED sentinels via errors.Is, never
// strings — the one place a business rejection becomes a Connect code; every
// Connect write handler and read handler (except the ListScopes exception,
// which has no deps.* counterpart) calls it instead of hand-rolling its own
// per-handler mapping (D-11, T-17-01/T-17-12).
//
// nil -> nil. Known sentinels map to precise codes:
//   - store.ErrNotFound -> CodeNotFound
//   - store.ErrInvalidArgument -> CodeInvalidArgument (covers 17-02's wrapped
//     parseWindow past-not_after/ordering rejections and validateRuleSummary
//     rejections — finding 5)
//   - errRuleImmutable -> CodeFailedPrecondition
//   - errStaleSummary -> CodeFailedPrecondition
//   - store.ErrAmbiguousShortID -> CodeFailedPrecondition (a legacy-data
//     precondition: a short_id matched >1 record via ResolvePointID at
//     store.go:1217 — NOT the generic CodeInternal, round-3 MED-5)
//   - context.Canceled -> CodeCanceled / context.DeadlineExceeded ->
//     CodeDeadlineExceeded (round-8 MED, Codex): the six write handlers and
//     the read handlers thread the request ctx into d.em.Embed/store ops, so a
//     client cancellation or request-deadline timeout must not be
//     misclassified as CodeInternal.
//   - store.ErrAlreadySuperseded -> CodeFailedPrecondition (Phase 25, Plan
//     02): pre-positioning only — supersede_memory is MCP-only this phase, no
//     Connect RPC exposes it yet — kept so the sentinel switch stays
//     exhaustive, exactly like the ErrIdempotencyConflict case below.
//
// Everything else falls through to CodeInternal, which logs the underlying
// error via slog.ErrorContext(ctx, ...) (request-scoped trace/log fields) and
// returns a GENERIC message so raw store/embed internals never reach the
// client (T-17-12). There is intentionally NO CodeAborted arm: no distinct
// concurrency/edit-conflict sentinel exists today (round-5 LOW) — one is added
// only if such a sentinel is later introduced.
func connectError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, store.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, errRuleImmutable):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, errStaleSummary):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, store.ErrAmbiguousShortID):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, store.ErrIdempotencyConflict):
		// Pre-positioning only (Phase 24, Plan 01): idempotency_key is
		// structurally unreachable from the Connect write lane this phase
		// (MCP-first per REQUIREMENTS Deferred), so this row cannot yet be
		// triggered — kept here so the sentinel switch stays exhaustive.
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, store.ErrAlreadySuperseded):
		// Pre-positioning only (Phase 25, Plan 02): supersede_memory is
		// MCP-only this phase — no Connect RPC exposes it yet — so this row
		// cannot yet be triggered from the Connect lane. Kept here so the
		// sentinel switch stays exhaustive, exactly like the
		// ErrIdempotencyConflict case above.
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	default:
		slog.ErrorContext(ctx, "connect handler: unexpected error", "error", err)
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
