// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// exitCodeBaselineCase is one row of the D-09 before-table: a single
// command x failure-mode observation, its currently-observed exit code, and
// the exit code this phase's later plans claim it will have once they ship.
//
// before/after/changes are a typed claim, not a loose "codes are as
// expected" assertion (memory nczgrtfec2): TestExitCodeBaselineClaims proves
// changes==true implies after != before and changes==false implies
// after == before, independent of any command execution.
type exitCodeBaselineCase struct {
	// name is the subtest key, e.g. "list/offset+page-token".
	name string
	// args is the full argv handed to runClient.
	args []string
	// env is applied with t.Setenv for the row.
	env map[string]string
	// before is the exit code observed against production code as it
	// stands at this plan's commit.
	before int
	// after is the exit code this phase claims the row will have once it
	// ships.
	after int
	// changes is the declared intent that after != before.
	changes bool
	// landed is set to true by the later plan that actually moves this
	// row's behavior; TestExitCodeBaseline compares against `after` once
	// landed (or introduced) is true, and against `before` otherwise.
	landed bool
	// introduced is reserved for rows a later plan adds for a capability
	// that does not exist yet at this plan's commit (a flag or code with no
	// meaningful `before`). Such rows assert only `after` and are exempt
	// from the distinct/identical rules below.
	introduced bool
	// hungServer, when true, replaces the literal placeholder argument
	// hungServerPlaceholder in args with a freshly started hung-server URL
	// (timeout_test.go's startHungServer) immediately before the row runs.
	// The static table cannot hard-code a hung server's address the way
	// deadServer hard-codes a refused one -- a hung listener needs a live
	// httptest.Server per run -- so this is the row-level opt-in that lets
	// TestExitCodeBaseline provide one without a second, parallel test loop.
	hungServer bool
}

// hungServerPlaceholder is the args token exitCodeBaseline rows use in
// place of a --server value when hungServer is true; TestExitCodeBaseline
// substitutes it with a real, freshly started hung-server URL.
const hungServerPlaceholder = "HUNG_SERVER"

// deadServer and deadQdrant are addresses nothing listens on, used by rows
// that need a dial to fail (connection refused) rather than hang or
// succeed. --timeout 2s (client rows below this plan's commit had no
// --timeout of their own, so their dial timeout was whatever the transport
// defaulted to; operator rows pass their own existing --timeout flag) keeps
// a dead backend from stalling the suite. As of this plan (01-07), client
// verbs carry their own --timeout too (client.timeout, default 30s); the
// existing client rows below deliberately omit it, since deadServer refuses
// the connection immediately rather than hanging.
const (
	deadServer = "http://127.0.0.1:1"
	deadQdrant = "127.0.0.1:1"
)

// exitCodeBaseline is the D-09 before-table: every command x failure mode
// this phase tracks, with its currently-observed exit code. Declared at
// package scope so later plans in this phase edit one literal — this plan
// (01-01) only observes and commits it; landed flips to true row by row as
// each later plan ships its change.
var exitCodeBaseline = []exitCodeBaselineCase{
	// --- Client verbs ---

	{
		name:    "list/offset+page-token",
		args:    []string{"list", "--server", deadServer, "--scope", "s", "--offset", "1", "--page-token", "X"},
		before:  exitUnavailable, // the trio is enforced nowhere today, so the call dials and fails
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "list/scope+cross-spine",
		args:    []string{"list", "--server", deadServer, "--scope", "s", "--cross-spine"},
		before:  exitUsage,
		after:   exitUsage,
		changes: false,
	},
	{
		name:    "search/scope+cross-spine",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--cross-spine", "--query", "q"},
		before:  exitUsage,
		after:   exitUsage,
		changes: false,
	},
	{
		// The adjacency row: cobra's flag groups count a *supplied* flag,
		// not its value, so a false value must classify identically to a
		// true one once D-07 lands — but today it does not.
		name:    "search/scope+cross-spine-false",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--cross-spine=false", "--query", "q"},
		before:  exitUnavailable, // a false value is accepted today
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "search/unknown-flag",
		args:    []string{"search", "--typo"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "search/unparseable-flag-value",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--query", "q", "--k", "notanumber"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "search/bad-output-value",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--query", "q", "--output", "bogus"},
		before:  exitUsage,
		after:   exitUsage,
		changes: false,
	},
	{
		name:    "search/missing-server",
		args:    []string{"search", "--scope", "s", "--query", "q"},
		env:     map[string]string{"ENGRAM_SERVER_URL": ""},
		before:  exitUsage,
		after:   exitUsage,
		changes: false,
	},
	{
		name:    "search/unreachable-server",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--query", "q"},
		before:  exitUnavailable,
		after:   exitUnavailable,
		changes: false,
	},
	{
		// Landed by this plan (01-07): clientFromFlags now calls
		// config.Load + config.ValidateClient before any dial, so a
		// malformed ENGRAM_TIMEOUT is rejected as exitUsage instead of
		// silently reaching the dead server. This closes the deferral
		// 01-06 recorded (see 01-03-SUMMARY.md's "Next Phase Readiness")
		// and empties exitCodeBaselineFullyMigratedAllowlist below.
		name:    "search/malformed-client-timeout-env",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--query", "q"},
		env:     map[string]string{"ENGRAM_TIMEOUT": "not-a-duration"},
		before:  exitUnavailable,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		// Introduced by this plan (01-07): --timeout did not exist on any
		// client verb before this plan, so there is no meaningful `before`.
		name:       "search/timeout-zero",
		args:       []string{"search", "--server", deadServer, "--scope", "s", "--query", "q", "--timeout", "0s"},
		after:      exitUsage,
		introduced: true,
	},
	{
		name:       "search/timeout-malformed",
		args:       []string{"search", "--server", deadServer, "--scope", "s", "--query", "q", "--timeout", "abc"},
		after:      exitUsage,
		introduced: true,
	},
	{
		// The ordering row (must_haves truth): a bad --timeout together with
		// an unreachable --server exits 2, not 5 -- client-config validation
		// runs before the dial, even against the same dead address every
		// other unreachable-server row in this table dials.
		name:       "search/timeout-zero-beats-unreachable",
		args:       []string{"search", "--server", deadServer, "--scope", "s", "--query", "q", "--timeout", "0"},
		after:      exitUsage,
		introduced: true,
	},
	{
		// Introduced by this plan (01-08): --timeout did not bound any
		// client RPC call site before this plan, so a hung server has no
		// meaningful `before` -- it simply blocked forever. --timeout 300ms
		// keeps the row fast; the hung server itself is started per-run by
		// TestExitCodeBaseline (see hungServer/hungServerPlaceholder).
		name:       "search/hung-server-timeout",
		args:       []string{"search", "--server", hungServerPlaceholder, "--scope", "s", "--query", "q", "--timeout", "300ms"},
		after:      exitTimeout,
		introduced: true,
		hungServer: true,
	},
	{
		name:       "list/hung-server-timeout",
		args:       []string{"list", "--server", hungServerPlaceholder, "--scope", "s", "--timeout", "300ms"},
		after:      exitTimeout,
		introduced: true,
		hungServer: true,
	},
	{
		name:    "store/missing-required",
		args:    []string{"store", "--server", deadServer},
		before:  exitUsage,
		after:   exitUsage,
		changes: false,
	},
	{
		name:    "root/self-describe",
		args:    noArgs,
		before:  exitOK,
		after:   exitOK,
		changes: false,
	},
	{
		// The accepted, documented gap: cobra's Find() fails before
		// execute() runs, so PersistentPreRunE never sees it and no
		// sentinel exists to classify it without string-matching cobra's
		// message (an explicit anti-pattern). Exit 1 stays here by design
		// (D-02).
		name:    "root/unknown-subcommand",
		args:    []string{"bogus-verb"},
		before:  exitGeneric,
		after:   exitGeneric,
		changes: false,
	},
	{
		name:    "root/legacy-env",
		args:    []string{"version"},
		env:     map[string]string{"MEM_QDRANT_ADDR": "old-host:1234"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},

	// --- Operator commands ---

	{
		name:    "reindex/missing-target",
		args:    []string{"reindex"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "reindex/unreachable-qdrant",
		args:    []string{"reindex", "--target", "t", "--timeout", "2s"},
		env:     map[string]string{"ENGRAM_QDRANT_ADDR": deadQdrant},
		before:  exitGeneric,
		after:   exitUnavailable,
		changes: true,
		landed:  true,
	},
	{
		name:    "prune/unreachable-qdrant",
		args:    []string{"prune-expired", "--timeout", "2s"},
		env:     map[string]string{"ENGRAM_QDRANT_ADDR": deadQdrant},
		before:  exitGeneric,
		after:   exitUnavailable,
		changes: true,
		landed:  true,
	},
	{
		name:    "summarize/missing-scope",
		args:    []string{"summarize-missing"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "summarize/missing-model",
		args:    []string{"summarize-missing", "--all-scopes", "--timeout", "2s"},
		env:     map[string]string{"ENGRAM_SUMMARY_MODEL": ""},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "backfill/unreachable-qdrant",
		args:    []string{"backfill-short-ids", "--timeout", "2s"},
		env:     map[string]string{"ENGRAM_QDRANT_ADDR": deadQdrant},
		before:  exitGeneric,
		after:   exitUnavailable,
		changes: true,
		landed:  true,
	},
	{
		name:    "migrate-remap/no-source",
		args:    []string{"migrate-remap-owner", "--to", "x"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "migrate-remap/two-sources",
		args:    []string{"migrate-remap-owner", "--from", "a", "--from-missing", "--to", "x"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "migrate-remap/identical-from-to",
		args:    []string{"migrate-remap-owner", "--from", "a", "--to", "a"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "migrate-set-owner/missing-owner",
		args:    []string{"migrate-set-owner"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "migrate-remap/unreachable-qdrant",
		args:    []string{"migrate-remap-owner", "--from-missing", "--to", "x", "--timeout", "2s"},
		env:     map[string]string{"ENGRAM_QDRANT_ADDR": deadQdrant},
		before:  exitGeneric,
		after:   exitUnavailable,
		changes: true,
		landed:  true,
	},
	{
		name:    "migrate-set-owner/unreachable-qdrant",
		args:    []string{"migrate-set-owner", "--owner", "x", "--timeout", "2s"},
		env:     map[string]string{"ENGRAM_QDRANT_ADDR": deadQdrant},
		before:  exitGeneric,
		after:   exitUnavailable,
		changes: true,
		landed:  true,
	},
	{
		name:    "serve/empty-listen-addr",
		args:    []string{"serve", "--listen-addr", ""},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "serve/ui-enabled-missing-creds",
		args:    []string{"serve", "--listen-addr", "127.0.0.1:0", "--ui-enabled", "true"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "serve/connect-headless-no-auth-lane",
		args:    []string{"serve", "--listen-addr", "127.0.0.1:0", "--connect-headless", "true"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
}

// TestExitCodeBaselineClaims is the memory nczgrtfec2 discipline applied to
// this table: a pure structural test over exitCodeBaseline, with no command
// execution. For every row with changes == true it asserts after != before;
// for every row with changes == false it asserts after == before; for every
// row it asserts introduced implies !changes. A loose "codes are as
// expected" assertion would pass while classification silently collapses —
// this types the claim itself.
//
// introduced rows are exempt from the after/before distinct-or-identical
// checks entirely (per the introduced field's own doc comment above: "no
// meaningful before"), not just from the changes==true branch — before this
// plan's rows, no row had ever set introduced:true, so this exemption was
// documented but never actually implemented; it silently compared an
// introduced row's zero-value `before` against its `after` and failed.
func TestExitCodeBaselineClaims(t *testing.T) {
	for _, c := range exitCodeBaseline {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if c.introduced && c.changes {
				t.Errorf("row %q: introduced rows must not also declare changes=true (no meaningful before)", c.name)
			}
			if c.introduced {
				return
			}
			if c.changes && c.after == c.before {
				t.Errorf("row %q: changes=true but after (%d) == before (%d), want distinct", c.name, c.after, c.before)
			}
			if !c.changes && c.after != c.before {
				t.Errorf("row %q: changes=false but after (%d) != before (%d), want identical", c.name, c.after, c.before)
			}
		})
	}
}

// TestExitCodeBaselineRowCount pins the table's row count and name
// uniqueness so a silently-deleted row fails the test instead of quietly
// shrinking coverage.
func TestExitCodeBaselineRowCount(t *testing.T) {
	const wantRows = 34
	if got := len(exitCodeBaseline); got != wantRows {
		t.Errorf("len(exitCodeBaseline) = %d, want %d", got, wantRows)
	}

	seen := make(map[string]bool, len(exitCodeBaseline))
	for _, c := range exitCodeBaseline {
		if seen[c.name] {
			t.Errorf("duplicate row name %q", c.name)
		}
		seen[c.name] = true
	}
}

// resetEveryCommandFlagState resets flag state (both pflag's Changed latch
// and the Go variable it writes into) for rootCmd itself AND every one of
// its direct subcommands. rootCmd's own tree is flat — every client and
// operator command is a direct child of rootCmd, never nested — so one
// level of Commands() is exhaustive.
//
// This table's rows invoke a different leaf command each (list, search,
// store, reindex, ...), and resetCommandFlagState alone only clears the
// specific *cobra.Command it is handed. A row invoking `store` after an
// earlier row (or an unrelated test elsewhere in this package) has left
// e.g. storeContent non-empty would silently skip store's own early
// validation and dial instead — observed directly: TestExitCodeBaseline's
// store/missing-required row reported exitUnavailable instead of exitUsage
// under `go test ./...` (full-package run order), because only rootCmd's
// own flags were being reset, never storeCmd's. Resetting the whole
// one-level tree before every row is what makes each row's observation
// depend only on its own args/env, independent of what ran before it in
// the same test binary.
func resetEveryCommandFlagState(t *testing.T, root *cobra.Command) {
	t.Helper()
	resetCommandFlagState(t, root)
	for _, c := range root.Commands() {
		resetCommandFlagState(t, c)
	}
}

// substituteHungServerURL returns a copy of args with hungServerPlaceholder
// replaced by a freshly started hung-server URL (timeout_test.go's
// startHungServer), for rows with hungServer set to true. A fresh server
// per row keeps each row's observation independent of what ran before it,
// the same discipline resetEveryCommandFlagState applies to flag state.
func substituteHungServerURL(t *testing.T, args []string) []string {
	t.Helper()
	url := startHungServer(t)
	out := make([]string, len(args))
	for i, a := range args {
		if a == hungServerPlaceholder {
			a = url
		}
		out[i] = a
	}
	return out
}

// TestExitCodeBaseline is the observation test: for each row, it drives
// rootCmd through the runClient harness exactly as Execute() would, and
// compares exitCodeFromError(err) against the row's declared expectation.
// Deliberately built on exitCodeFromError + runClient, NOT on the
// ExitCode()-or-t.Fatal helper client_search_test.go uses for its own
// positive-case tests: that helper aborts the moment an error lacks
// ExitCode(), which is precisely the untyped rows this table exists to pin
// (flag-group errors, config.CheckLegacy, buildRemapSource) — aborting
// instead of recording "falls through to 1" would defeat the point.
//
// Subtests are NOT t.Parallel(): rows mutate process-global env (t.Setenv)
// and shared cobra flag state, so concurrent rows would race each other.
func TestExitCodeBaseline(t *testing.T) {
	for _, c := range exitCodeBaseline {
		c := c
		t.Run(c.name, func(t *testing.T) {
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			resetClientFlags(t)
			resetEveryCommandFlagState(t, rootCmd)

			args := c.args
			if c.hungServer {
				args = substituteHungServerURL(t, args)
			}
			_, _, err := runClient(t, args...)
			got := exitCodeFromError(err)

			want := c.before
			if c.landed || c.introduced {
				want = c.after
			}
			if got != want {
				t.Errorf("row %q: exitCodeFromError(err) = %d, want %d (err=%v)", c.name, got, want, err)
			}
		})
	}
}

// exitCodeBaselineFullyMigratedAllowlist is the explicit, named exception to
// TestExitCodeBaselineFullyMigrated below: a row here is exempt from the
// "changes implies landed" rule because its behavior change is genuinely
// deferred to a later plan, not because it was missed by this plan's own
// sweep. A loose "skip every client-verb row" would silently exempt rows
// nobody actually deferred; naming them individually is what keeps the
// exemption auditable, and is why an entry here must correspond to a real
// row with a comment explaining which later plan owns it.
//
// Empty since plan 01-07 and confirmed still empty by 01-08: the one row
// that had populated this allowlist, "search/malformed-client-timeout-env",
// is landed here (see its own row comment above) -- ValidateClient is now
// wired into the client entry path. 01-08 added two `introduced: true`
// rows (search/hung-server-timeout, list/hung-server-timeout) that need no
// allowlist entry -- introduced rows never carry changes: true, so they are
// outside this gate's scope entirely. This remains the phase's closing
// proof for this gate: every row this table tracks, client and operator
// alike, has changes implying landed with zero exceptions.
var exitCodeBaselineFullyMigratedAllowlist = map[string]bool{}

// TestExitCodeBaselineFullyMigrated is the table-level D-03 closing gate,
// complementing TestNoBareOperatorErrorReturns' source-level one
// (operror_test.go): every row declaring changes: true must also carry
// landed: true, except a row explicitly named in
// exitCodeBaselineFullyMigratedAllowlist. As of this plan (01-08) that
// allowlist is still empty: every row this table tracks -- client and
// operator alike -- is fully landed.
func TestExitCodeBaselineFullyMigrated(t *testing.T) {
	for _, c := range exitCodeBaseline {
		c := c
		if !c.changes {
			continue
		}
		if exitCodeBaselineFullyMigratedAllowlist[c.name] {
			continue
		}
		if !c.landed {
			t.Errorf("row %q: changes=true but landed=false, and not present in exitCodeBaselineFullyMigratedAllowlist -- either this row's change has not shipped yet (add it to the allowlist with a comment naming the owning plan) or it was missed by the plan that should have flipped it", c.name)
		}
	}
}
