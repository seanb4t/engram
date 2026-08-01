// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// connectMapCtx returns a Connect request context authenticated as a
// per-subtest owner. A distinct owner per subtest avoids any accidental
// cross-subtest data collision in the shared testDeps store.
func connectMapCtx(owner string) context.Context {
	return withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": owner}})
}

// TestStoreRuleValidationIsNotCodeInternal pins the third and last of D-11a's
// bare-unwrapped-error defects, closed by 04-05 Task 1: validateStoreRule's
// four rejections and validateRuleSummary's three both fell through
// connectError's switch to CodeInternal before conversion — a caller's
// invalid input reported as a server fault.
//
// store_rule has NO Connect RPC (RESEARCH: Connect-UNREACHABLE today), so
// this defect is latent — there is no handler to drive an RPC through, and a
// test that tried would not compile against anything real. This test calls
// connectError directly with the error validateStoreRule returns, exactly
// the idiom TestStoreDiscoveryValidationIsNotCodeInternal (04-01) and
// TestCitationValidationIsNotCodeInternal (04-04) already established for
// the first two of D-11a's three sites.
func TestStoreRuleValidationIsNotCodeInternal(t *testing.T) {
	const validContent = "a rule agents must follow"
	const validScope = "rule:repo:engram"
	const validSummary = "a one-line index entry"

	cases := []struct {
		name string
		a    storeRuleArgs
	}{
		{"content_empty", storeRuleArgs{Content: "", Scope: validScope, Summary: validSummary}},
		{"content_too_large", storeRuleArgs{Content: strings.Repeat("x", maxRuleContentBytes+1), Scope: validScope, Summary: validSummary}},
		{"scope_empty", storeRuleArgs{Content: validContent, Scope: "", Summary: validSummary}},
		{"scope_malformed", storeRuleArgs{Content: validContent, Scope: "not-a-rule-scope", Summary: validSummary}},
		{"summary_empty", storeRuleArgs{Content: validContent, Scope: validScope, Summary: ""}},
		{"summary_multiline", storeRuleArgs{Content: validContent, Scope: validScope, Summary: "line one\nline two"}},
		{"summary_too_long", storeRuleArgs{Content: validContent, Scope: validScope, Summary: strings.Repeat("x", maxRuleSummaryBytes+1)}},
	}
	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStoreRule(tc.a)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			got := connect.CodeOf(connectError(ctx, err))
			if got == connect.CodeInternal {
				t.Fatalf("connectError classified a rule rejection as CodeInternal (D-11a): %v", err)
			}
		})
	}
}

// TestConnectValidationCodeMapping is the plan's proof that Task 2 actually
// changed behavior: it is a table over every converted rejection class
// reachable across tools.go, rules.go, and connectapi.go, asserting the
// resulting Connect code equals the class table's assignment AND is a
// member of the CLI-compatible trio as a SET (so a future fourth class
// fails here, not in a user's CLI).
//
// RED transcript (captured this session, with Task 2 already committed):
// connectapi.go's ListMemories cursor_mode/offset check was temporarily
// reverted to its pre-Task-2 hand-wrap,
// `return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cursor_mode is mutually exclusive with offset"))`,
// and `go test ./internal/server/... -run 'TestConnectValidationCodeMapping$/connect_list_memories_cursor_mode_offset' -v -count=1`
// was re-run. It failed:
//
//	=== RUN   TestConnectValidationCodeMapping
//	=== RUN   TestConnectValidationCodeMapping/connect_list_memories_cursor_mode_offset
//	    connectargerror_test.go:258: connect.CodeOf(err) = invalid_argument, want failed_precondition (err: invalid_argument: cursor_mode is mutually exclusive with offset)
//	--- FAIL: TestConnectValidationCodeMapping (0.10s)
//	    --- FAIL: TestConnectValidationCodeMapping/connect_list_memories_cursor_mode_offset (0.10s)
//	FAIL
//
// The revert was undone (`git diff --exit-code -- internal/server/connectapi.go`
// confirmed zero net change against the committed tree), and the subtest
// passes again below. This confirms the table would have caught the exact
// no-op D-11 would have shipped as on the Connect lane had Task 2 been
// skipped — every existing pre-04-05 test asserted CodeInvalidArgument, so a
// plan that stopped after Task 1 would have looked completely green.
func TestConnectValidationCodeMapping(t *testing.T) {
	trio := map[connect.Code]bool{
		connect.CodeInvalidArgument:    true,
		connect.CodeOutOfRange:         true,
		connect.CodeFailedPrecondition: true,
	}
	ctx := context.Background()

	cases := []struct {
		name string
		run  func(t *testing.T) error
		want connect.Code
	}{
		// --- tools.go, direct connectError(ctx, err) calls ---------------
		{
			name: "tools_store_discovery_content_required",
			run: func(_ *testing.T) error {
				return connectError(ctx, validateStoreDiscovery(storeDiscoveryArgs{}))
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "tools_citations_too_many",
			run: func(_ *testing.T) error {
				valid := citationArg{Kind: "file", Ref: "r"}
				over := make([]citationArg, maxDiscoveryCitations+1)
				for i := range over {
					over[i] = valid
				}
				return connectError(ctx, validateCitations(over, 0))
			},
			want: connect.CodeOutOfRange,
		},
		{
			name: "tools_parse_window_ordering",
			run: func(_ *testing.T) error {
				now := timeNow()
				a := scheduleArgs{
					NotBefore: now.Add(2 * time.Hour).Format(time.RFC3339),
					NotAfter:  now.Add(1 * time.Hour).Format(time.RFC3339),
				}
				_, _, err := parseWindow(a, now)
				return connectError(ctx, err)
			},
			want: connect.CodeFailedPrecondition,
		},
		{
			name: "tools_idempotency_key_too_long",
			run: func(_ *testing.T) error {
				d := &deps{}
				_, _, _, _, err := d.checkIdempotentReplay(ctx, "owner", storeArgs{
					Scope:          "tool:project:x",
					IdempotencyKey: strings.Repeat("a", maxIdempotencyKeyBytes+1),
				})
				return connectError(ctx, err)
			},
			want: connect.CodeOutOfRange,
		},
		// --- rules.go, direct connectError(ctx, err) calls ----------------
		{
			name: "rules_store_rule_content_required",
			run: func(_ *testing.T) error {
				return connectError(ctx, validateStoreRule(storeRuleArgs{}))
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "rules_summary_too_long",
			run: func(_ *testing.T) error {
				return connectError(ctx, validateRuleSummary(strings.Repeat("x", maxRuleSummaryBytes+1)))
			},
			want: connect.CodeOutOfRange,
		},
		{
			name: "rules_list_rules_scope_malformed",
			run: func(_ *testing.T) error {
				d := &deps{}
				_, _, err := d.listRules(ctx, caller{}, listRulesArgs{Scopes: []string{"not-a-rule-scope"}})
				return connectError(ctx, err)
			},
			want: connect.CodeInvalidArgument,
		},
		// --- connectapi.go, driven through the REAL Connect handler -------
		// (this is the lane Task 2 rewired; these rows are the proof)
		{
			name: "connect_list_memories_bad_created_after",
			run: func(t *testing.T) error {
				api := &engramAPI{d: testDeps(t)}
				_, err := api.ListMemories(connectMapCtx("sub-map-1"), connect.NewRequest(&engramv1.ListMemoriesRequest{
					Scope: "s:project:x", CreatedAfter: "not-a-date",
				}))
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "connect_search_memories_bad_created_before",
			run: func(t *testing.T) error {
				api := &engramAPI{d: testDeps(t)}
				_, err := api.SearchMemories(connectMapCtx("sub-map-2"), connect.NewRequest(&engramv1.SearchMemoriesRequest{
					Query: "x", Scope: "s:project:x", CreatedBefore: "not-a-date",
				}))
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			// classPrecondition through a real Connect handler: the exact row
			// that would have silently flattened to CodeInvalidArgument had
			// Task 2's hand-wrap removal at connectapi.go been skipped or
			// reverted (T-04-14). See the RED transcript above.
			name: "connect_list_memories_cursor_mode_offset",
			run: func(t *testing.T) error {
				api := &engramAPI{d: testDeps(t)}
				_, err := api.ListMemories(connectMapCtx("sub-map-3"), connect.NewRequest(&engramv1.ListMemoriesRequest{
					Scope: "s:project:x", Limit: 10, Offset: 5, CursorMode: true,
				}))
				return err
			},
			want: connect.CodeFailedPrecondition,
		},
		{
			// classOutOfRange through a real Connect handler.
			name: "connect_store_discovery_too_many_citations",
			run: func(t *testing.T) error {
				api := &engramAPI{d: testDeps(t)}
				cites := make([]*engramv1.Citation, maxDiscoveryCitations+1)
				for i := range cites {
					cites[i] = &engramv1.Citation{Kind: "file", Ref: "r"}
				}
				_, err := api.StoreDiscovery(connectMapCtx("sub-map-4"), connect.NewRequest(&engramv1.StoreDiscoveryRequest{
					Content: "x", Kind: "fact", Scope: "discovery:repo:X", Citations: cites,
				}))
				return err
			},
			want: connect.CodeOutOfRange,
		},
		{
			name: "connect_list_memories_scope_required",
			run: func(t *testing.T) error {
				api := &engramAPI{d: testDeps(t)}
				_, err := api.ListMemories(connectMapCtx("sub-map-5"), connect.NewRequest(&engramv1.ListMemoriesRequest{
					Scope: "", CrossSpine: false, Limit: 10,
				}))
				return err
			},
			want: connect.CodeInvalidArgument,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(t)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			got := connect.CodeOf(err)
			if got != tc.want {
				t.Fatalf("connect.CodeOf(err) = %v, want %v (err: %v)", got, tc.want, err)
			}
			if !trio[got] {
				t.Fatalf("connect.CodeOf(err) = %v is NOT a member of the CLI-compatible trio {InvalidArgument, OutOfRange, FailedPrecondition}", got)
			}
		})
	}
}

// TestConnectCombinationAttribution proves the cursor_mode/offset relational
// rejection carries BOTH field names (never an arbitrary single pick) and
// maps to CodeFailedPrecondition, driven through the real ListMemories
// handler — reusing the same fixture shape as
// TestListMemoriesRejectsCursorModeWithOffset (connectapi_test.go) so the
// wiring is exercised end to end, not just the argErrFieldsf constructor.
func TestConnectCombinationAttribution(t *testing.T) {
	api := &engramAPI{d: testDeps(t)}
	_, err := api.ListMemories(connectMapCtx("sub-combo-attr"), connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: "s:project:x", Limit: 10, Offset: 5, CursorMode: true,
	}))
	if err == nil {
		t.Fatalf("cursor_mode + offset>0 produced no error")
	}
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("connect.CodeOf(err) = %v, want CodeFailedPrecondition", got)
	}
	var ae *argError
	if !errors.As(err, &ae) {
		t.Fatalf("error does not unwrap to *argError: %v", err)
	}
	wantFields := []string{"cursor_mode", "offset"}
	if !reflect.DeepEqual(ae.Fields, wantFields) {
		t.Fatalf("Fields = %v, want %v", ae.Fields, wantFields)
	}
	if ae.Hint != HintMutuallyExclusive {
		t.Fatalf("Hint = %v, want %v", ae.Hint, HintMutuallyExclusive)
	}
}
