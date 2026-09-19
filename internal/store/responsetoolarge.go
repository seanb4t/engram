// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts Phase 2's receive-limit classifier (CONTEXT.md D-01, D-02,
// D-03): a gRPC unary client interceptor, installed in NewQdrantClient's base
// dial options, that classifies a Qdrant response exceeding the client's
// receive limit into ErrResponseTooLarge / ResponseTooLargeError. It matches
// on BOTH the gRPC ResourceExhausted code AND grpc-go's client-side
// receive-limit message shape (D-02) — never on code alone. Matching on code
// alone would also relabel a genuine server-side ResourceExhausted (rate
// limiting, capacity), steering an operator to shrink a request when the
// real remedy is server capacity; it would also swallow the status before
// qdrant-go-client's own rate-limit interceptor — which sits OUTSIDE this
// classifier in the dial chain — ever sees it, breaking that interceptor's
// own retry-after handling. Every other error, including a non-matching
// ResourceExhausted, passes through as the identical value: never re-wrapped,
// never re-classified.
//
// The byte counts (Observed/Limit) and the raw upstream grpc-go text carried
// on ResponseTooLargeError are for SERVER-SIDE LOGS ONLY (D-03) — no lane may
// put them on a wire; the Connect and MCP mappers (plan 02-02) emit a
// scrubbed envelope instead.

package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// classifyResponseTooLarge is a grpc.UnaryClientInterceptor. It calls the
// invoker, and on a codes.ResourceExhausted status whose message matches
// isRecvLimitMessage, returns a *ResponseTooLargeError instead — every other
// result (including nil, a non-ResourceExhausted error, and a
// ResourceExhausted whose message does not match) is returned unchanged, so
// this interceptor is byte-for-byte transparent to everything downstream in
// the dial chain on a non-match.
func classifyResponseTooLarge(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	err := invoker(ctx, method, req, reply, cc, opts...)
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != grpccodes.ResourceExhausted || !isRecvLimitMessage(st.Message()) {
		return err
	}
	return newResponseTooLargeError(method, st.Message())
}

// ErrResponseTooLarge is the sentinel every lane maps by errors.Is when a
// Qdrant response exceeded the client's receive limit. It is a distinct
// sentinel — never folded into ErrInvalidArgument or any other existing
// error — because the remedy (a smaller page, no `full`) differs from every
// other failure this package returns.
var ErrResponseTooLarge = errors.New("qdrant response exceeded the client's receive limit")

// ResponseTooLargeError carries the RPC method and the observed/limit byte
// counts, for SERVER-SIDE LOGS ONLY (D-03) — its fields and Error() text
// never reach any wire; the Connect and MCP mappers (plan 02-02) render a
// separate, scrubbed envelope instead. Observed is best-effort: grpc-go's
// single-number receive shape (the shape this project's own Phase 1 RED
// capture actually observed live) reports only the limit, never the
// observed size, so Observed stays 0 for that shape — documented behavior,
// never silently defaulted to something else.
type ResponseTooLargeError struct {
	// Method is the gRPC method the interceptor received — never parsed
	// from text, always the interceptor's own method argument.
	Method string
	// Limit is the receive limit parsed from the grpc-go message; 0 if the
	// message shape carried no parseable integer.
	Limit int
	// Observed is the received size when the message shape reports one; 0
	// means the message shape did not report it (see type doc comment).
	Observed int
	// upstream is the raw grpc-go message text, carried only for
	// server-side logs via Error().
	upstream string
}

// Error returns the sentinel text, the RPC method, and the raw upstream
// grpc-go text — a server-side-log shape only, never a wire shape.
func (e *ResponseTooLargeError) Error() string {
	return fmt.Sprintf("%s: %s: %s", ErrResponseTooLarge.Error(), e.Method, e.upstream)
}

// Unwrap makes errors.Is(err, ErrResponseTooLarge) match through this type.
func (e *ResponseTooLargeError) Unwrap() error { return ErrResponseTooLarge }

// GRPCStatus keeps status.FromError working on the classified error (D-01),
// so existing status inspection (e.g. store.go's ensureIndexes
// grpccodes.AlreadyExists idiom) keeps working on a classified error too.
func (e *ResponseTooLargeError) GRPCStatus() *status.Status {
	return status.New(grpccodes.ResourceExhausted, e.Error())
}

// recvLimitPairPattern captures the two integers of grpc-go's "(<observed>
// vs. <limit>)" receive-shape suffix.
var recvLimitPairPattern = regexp.MustCompile(`\((\d+) vs\. (\d+)\)`)

// recvLimitOnlyPattern captures the trailing single integer of grpc-go's
// "larger than max <limit>" receive-shape suffix — the shape that reports
// no observed size.
var recvLimitOnlyPattern = regexp.MustCompile(`larger than max (\d+)$`)

// isRecvLimitMessage reports whether msg carries one of grpc-go v1.83.2's
// FOUR client-side receive-limit message shapes:
//   - "grpc: received message larger than max length allowed on current machine (%d vs. %d)" (rpc_util.go:795)
//   - "grpc: received message larger than max (%d vs. %d)" (rpc_util.go:798)
//   - "grpc: message after decompression larger than max (%d vs. %d)" (rpc_util.go:1008)
//   - "grpc: received message after decompression larger than max %d" (rpc_util.go:1039)
//
// and none of the two SEND-side shapes ("grpc: trying to send message
// larger than max (%d vs. %d)" at server.go:1215, and the unprefixed
// "trying to send message larger than max (%d vs. %d)" at
// stream.go:991/1496/1779). This is the matcher's OWN logic under test —
// this package's synthetic-status unit tests (TestClassifyResponseTooLarge)
// mirror these shapes as INPUTS; TestStoreListOverflowIsResponseTooLarge is
// what proves the pinned grpc-go version's real traffic still matches (rule
// m45p2b4bp7 — never asserting grpc-go's own behavior as the test oracle).
func isRecvLimitMessage(msg string) bool {
	if !strings.HasPrefix(msg, "grpc: ") {
		return false
	}
	if !strings.Contains(msg, "larger than max") {
		return false
	}
	return strings.Contains(msg, "received message") || strings.Contains(msg, "message after decompression")
}

// newResponseTooLargeError builds a *ResponseTooLargeError for method and
// msg, parsing Limit (and, when the shape reports one, Observed) as exact
// integers via strconv.Atoi. A parse failure leaves the field at 0 — best
// effort, never an error: Method is what matters most for logs, and is
// always reliably available from the interceptor's own argument, never from
// text.
func newResponseTooLargeError(method, msg string) *ResponseTooLargeError {
	e := &ResponseTooLargeError{Method: method, upstream: msg}
	if m := recvLimitPairPattern.FindStringSubmatch(msg); m != nil {
		if observed, err := strconv.Atoi(m[1]); err == nil {
			e.Observed = observed
		}
		if limit, err := strconv.Atoi(m[2]); err == nil {
			e.Limit = limit
		}
		return e
	}
	if m := recvLimitOnlyPattern.FindStringSubmatch(msg); m != nil {
		if limit, err := strconv.Atoi(m[1]); err == nil {
			e.Limit = limit
		}
	}
	return e
}
