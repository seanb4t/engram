// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"net"

	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/seanb4t/engram/internal/store"
)

// notFoundErrorf and unavailableErrorf are usageErrorf's siblings
// (client_common.go), carrying exitNotFound and exitUnavailable
// respectively, in the exact same shape. Kept here, next to their only
// consumer, rather than in client_common.go alongside usageErrorf -- this
// plan owns one file.
func notFoundErrorf(format string, a ...any) error {
	return &cliError{code: exitNotFound, err: fmt.Errorf(format, a...)}
}

func unavailableErrorf(format string, a ...any) error {
	return &cliError{code: exitUnavailable, err: fmt.Errorf(format, a...)}
}

// isUnavailableTransportErr reports whether err represents a transport
// failure reaching Qdrant: a gRPC status carrying grpccodes.Unavailable, or
// (defensively) a raw net.Error reaching this function without ever
// becoming a *status.Status. status.FromError unwraps through any
// fmt.Errorf("...: %w", err) chain -- grpc-go's status.FromError uses
// errors.As internally -- so this reaches through EnsureCollection's and
// Reindex's own wrapping (internal/server/tools.go, internal/store/store.go)
// without needing a bespoke unwrap here.
func isUnavailableTransportErr(err error) bool {
	if st, ok := status.FromError(err); ok && st.Code() == grpccodes.Unavailable {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// classifyOperatorErr is the single CLI-side classifier from a store/
// transport error to a process exit code, for the four sweep-style operator
// commands (reindex, prune-expired, summarize-missing, backfill-short-ids;
// D-03). Modeled on internal/server/connecterror.go's connectError: an
// err == nil early return, then a single switch of errors.Is/errors.As
// arms, then a default -- never classification by message text.
//
// A new function rather than a reuse of exitCodeForConnectErr
// (client_common.go): these four commands never dial through the Connect
// handlers. They call server.StoreFromEnv / StoreAndEmbedderFromEnvNoEnsure
// / StoreAndSummarizerFromEnv and then a store.* method directly, so a
// store/Qdrant error never becomes a *connect.Error, and connect.CodeOf
// would report CodeUnknown for every one of them.
//
// classifyOperatorErr's own default is a true passthrough (D-02: exit code 1
// stays a real backstop for a genuinely unclassified Go error, never a
// silent misclassification). classifyConstructionErr below layers
// call-site knowledge on top of this function for the three store
// constructors specifically, where every error that reaches this function's
// default IS, by construction of internal/server/tools.go, a config/parse
// validation failure -- see its own doc comment.
func classifyOperatorErr(err error) error {
	if err == nil {
		return nil
	}

	var classified interface{ ExitCode() int }
	switch {
	// MUST stay first, before any sentinel arm below: an error that already
	// carries ExitCode() (e.g. a usageErrorf raised by a pure flag-shape
	// guard before this function ever runs) is returned unchanged -- a
	// doubly-classified error keeps its first code (idempotent). Mirrors
	// connectError's own "*argError case must stay first" discipline
	// (STATE.md durable record 667p88n2be): reordering this switch for
	// tidiness risks the same silent-collapse failure mode.
	case errors.As(err, &classified):
		return err

	// A client-side context cancellation/deadline -- an operator --timeout
	// firing, or Ctrl-C/SIGTERM via signal.NotifyContext -- returns the bare
	// context error directly, never wrapped in a gRPC status, because it is
	// detected before (or independent of) any wire round-trip completing.
	// Checked ahead of the transport-status arm below for that reason.
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return unavailableErrorf("%w", err)

	// A gRPC-level dial/RPC failure reaching Qdrant -- covers both "gRPC
	// codes.Unavailable" and "a raw dial failure" from the plan's behavior
	// table; see isUnavailableTransportErr.
	case isUnavailableTransportErr(err):
		return unavailableErrorf("%w", err)

	// Store sentinels (internal/store/store.go), mirroring connectError's
	// shape. None of these four commands constructs a wrapper type whose
	// Unwrap() returns one of these sentinels (unlike connectError's
	// *argError/store.ErrInvalidArgument pair), so there is no comparable
	// ordering hazard among the arms below -- but see STATE.md's durable
	// record before adding one.
	case errors.Is(err, store.ErrNotFound):
		return notFoundErrorf("%w", err)
	case errors.Is(err, store.ErrInvalidArgument), errors.Is(err, store.ErrAmbiguousShortID):
		return usageErrorf("%w", err)
	case errors.Is(err, store.ErrShortIDExhausted):
		// Live trigger: backfill-short-ids' MintShortID call
		// (internal/store/store.go). The generator giving up after real
		// attempts is a backend-capacity condition -- not a caller flag,
		// not a missing target -- so of the remaining 2/4/5 vocabulary,
		// "the backend cannot currently service this" (exitUnavailable) is
		// the closer fit than exitUsage. See 01-05-SUMMARY.md for the
		// recorded rationale.
		return unavailableErrorf("%w", err)
	case errors.Is(err, store.ErrIdempotencyConflict):
		// No live trigger from these four commands' call paths today (only
		// the MCP-facing store_memory/schedule_memory keyed writes raise
		// it); kept mapped so the switch stays exhaustive, mirroring
		// connectError's own documented "pre-positioning only" treatment of
		// this sentinel. Classified exitUsage: a reused idempotency key
		// against different content is a caller-input conflict the caller
		// can fix by choosing a new key -- the same disposition as a bad
		// flag value.
		return usageErrorf("%w", err)
	case errors.Is(err, store.ErrAlreadySuperseded):
		// No live trigger from these four commands' call paths today
		// (supersede_memory is MCP-only); kept mapped for the same
		// exhaustiveness reason. Classified exitUsage, mirroring
		// connectError's CodeFailedPrecondition mapping for this sentinel,
		// which exitCodeForConnectErr itself maps to exitUsage on the
		// client side.
		return usageErrorf("%w", err)

	// Deliberately NO arm maps exit code 3 (authentication/authorization)
	// here: none of these four commands authenticates against the server --
	// they call internal/store directly and never traverse the OIDC/Connect
	// chain -- so there is no live trigger, and a mapping with no trigger is
	// a claim the binary cannot honor.

	default:
		// Genuinely unclassified from this function's own signals: returned
		// unchanged, honoring D-02 -- exit code 1 stays a real backstop for
		// a future, truly unanticipated error type, not a silent
		// misclassification. classifyConstructionErr applies call-site
		// knowledge on top of this at the three store-construction sites.
		return err
	}
}

// classifyConstructionErr wraps classifyOperatorErr for the three store/
// summarizer constructors these four commands call before any sweep begins:
// server.StoreFromEnv, StoreAndEmbedderFromEnvNoEnsure, and
// StoreAndSummarizerFromEnv (internal/server/tools.go). Per that file, the
// ONLY network-touching call any of the three makes is EnsureCollection
// (StoreFromEnv and StoreAndSummarizerFromEnv only -- reindex's no-ensure
// path never dials at construction), already classified by
// classifyOperatorErr's transport arms above. Everything else the three can
// return -- a malformed ENGRAM_EMBED_DIM/ENGRAM_QDRANT_ADDR, an empty
// ENGRAM_SUMMARY_MODEL, a cfg.Validate() failure -- is a plain, unsentineled
// config/parse error: internal/config carries no exported sentinel to
// errors.Is/errors.As against, and classifying by message text is
// prohibited here (see the plan's acceptance criteria). By elimination, AT
// THIS CALL SITE specifically, an error classifyOperatorErr does not
// otherwise recognize is config-shaped: usageErrorf.
//
// This is the "classify at the construction call site" fallback the plan's
// Flagged Assumptions anticipates for exactly this situation, and it keeps
// classifyOperatorErr itself an honest general classifier -- its own
// default stays a true passthrough -- rather than forcing every
// unrecognized error, everywhere, into exitUsage.
func classifyConstructionErr(err error) error {
	if err == nil {
		return nil
	}
	classifiedErr := classifyOperatorErr(err)
	var classified interface{ ExitCode() int }
	if errors.As(classifiedErr, &classified) {
		return classifiedErr
	}
	return usageErrorf("%w", err)
}
