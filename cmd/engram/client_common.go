// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/config"
)

// addClientFlags binds the five flags every client command shares, each
// bound to a client.* koanf registry row (D-04): no package-level Go var
// backs any of them anymore. clientFromFlags is the single place a value is
// read, by loading the registry from this command's own flag set.
//
// Two deliberate divergences from the registry-default-string idiom
// serve.go's own flags use: --insecure stays a Bool so a bare --insecure
// keeps working (config.Load's changed-flag overlay reads any pflag type's
// String() form, not just string flags, so this still reaches the
// registry); --timeout's usage text states inline that 0 is rejected,
// because this binary's operator commands ship a --timeout of the same name
// whose help text says 0 disables, and a reader comparing `engram search
// --help` against `engram reindex --help` must not be left to infer the
// difference.
func addClientFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String("server", config.FlagDefault("server"),
		"engram server base URL (default: ENGRAM_SERVER_URL)")
	f.String("token-file", config.FlagDefault("token-file"),
		"path to a file containing the bearer credential (default: ENGRAM_TOKEN env var)")
	f.Bool("insecure", false,
		"skip TLS certificate verification (always warns on stderr; no environment fallback)")
	f.String("output", config.FlagDefault("output"),
		`output format: "json" or "text" (default: detect from stdout)`)
	f.String("timeout", config.FlagDefault("timeout"),
		"per-request client deadline (must be > 0, e.g. 30s, 2m; a value of 0 is rejected, "+
			"never treated as unbounded — unlike this binary's operator-command --timeout, where 0 disables)")
}

// resolveToken resolves ENGRAM_TOKEN, then the file named by
// tokenFilePath, trimmed of surrounding whitespace (a trailing newline is
// the default state of a file written by a shell redirect, and an
// untrimmed value would otherwise become a malformed multi-field
// credential the server's extractor rejects). Neither set: returns "" with
// a nil error — an anonymous call against a no-issuer server is legal.
// There is no flag that accepts a credential value directly; a credential
// must never be able to reach argv (D-13). Only the token's PATH is
// declared in the client.* koanf registry (client.token_file) — there is no
// client.token key, and the credential itself never routes through koanf.
func resolveToken(tokenFilePath string) (string, error) {
	if v := os.Getenv("ENGRAM_TOKEN"); v != "" {
		return v, nil
	}
	if tokenFilePath == "" {
		return "", nil
	}
	b, err := os.ReadFile(tokenFilePath)
	if err != nil {
		// usageErrorf, not fmt.Errorf (REVIEW.md WR-01): an unreadable
		// --token-file is the caller naming a bad path, which is the
		// client's own semantic validation — D-17 reserves exit 2 for
		// exactly that. A plain error would exit 1 and be indistinguishable
		// from a server-side failure, so a script could not tell "fix your
		// path" from "retry later".
		return "", usageErrorf("reading --token-file: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// bearerInterceptor attaches the resolved credential to every outgoing
// request as an Authorization header. When token is empty it sets nothing
// at all — an anonymous call must never send a malformed "Bearer " header
// with an empty credential, which the server's extractor rejects. This
// mirrors the server-side connect.UnaryInterceptorFunc idiom already used
// in internal/server, and composes with any transport: TLS policy and
// credential attachment never touch the same object.
func bearerInterceptor(token string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" {
				req.Header().Set("Authorization", "Bearer "+token)
			}
			return next(ctx, req)
		}
	}
}

// newHTTPClient builds the HTTP client the Connect client is constructed
// over. TLS verification is on by default: the zero value of
// InsecureSkipVerify is false, so the do-nothing path is the secure path
// (D-14).
func newHTTPClient(insecure bool) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure}, //nolint:gosec // gated by --insecure, D-14
		},
	}
}

// clientFromFlags is the single shared constructor for every client
// subcommand (D-03): it loads every client setting through the client.*
// koanf registry in one call (D-04), runs the registry's own client
// validator before any dial, resolves the token, and — when --insecure is
// set — writes an unconditional warning to stderr, never to stdout, never
// gated by --output, never suppressible (D-14/D-07).
//
// It returns the resolved output format and the parsed request deadline
// alongside the client, so every setting a client command needs comes from
// this one load. Plan 01-08 wires the returned duration into the RPC calls;
// there is no second resolution path for it.
//
// It does not add cookie-jar or anti-forgery-token plumbing:
// 02-RESEARCH.md Pattern 3 verified that the server exempts the bearer lane
// totally for write procedures, so any such code here would be
// unreachable against every server this client can talk to.
func clientFromFlags(cmd *cobra.Command) (engramv1connect.EngramServiceClient, outputFormat, time.Duration, error) {
	cfg, err := config.Load(cmd.Flags())
	if err != nil {
		return nil, formatJSON, 0, usageErrorf("load client configuration: %w", err)
	}
	if err := config.ValidateClient(cfg.Client); err != nil {
		return nil, formatJSON, 0, usageErrorf("%w", err)
	}
	if cfg.Client.ServerURL == "" {
		return nil, formatJSON, 0, usageErrorf("--server or ENGRAM_SERVER_URL is required")
	}
	token, err := resolveToken(cfg.Client.TokenFile)
	if err != nil {
		return nil, formatJSON, 0, err
	}
	// ValidateClient has already confirmed cfg.Client.Insecure parses as a
	// bool; an error here is unreachable by construction.
	insecure, err := strconv.ParseBool(cfg.Client.Insecure)
	if err != nil {
		return nil, formatJSON, 0, usageErrorf("--insecure %q: must be a boolean: %w", cfg.Client.Insecure, err)
	}
	// ValidateClient has already confirmed cfg.Client.Timeout is a positive
	// Go duration; an error here is unreachable by construction.
	timeout, err := time.ParseDuration(cfg.Client.Timeout)
	if err != nil {
		return nil, formatJSON, 0, usageErrorf("ENGRAM_TIMEOUT %q: must be a Go duration: %w", cfg.Client.Timeout, err)
	}
	if insecure {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
			"WARNING: TLS certificate verification is disabled (--insecure); "+
				"do not use against an untrusted network")
	}
	format := outputFormatFromConfig(cfg.Client.Output, isTerminal(os.Stdout))
	client := engramv1connect.NewEngramServiceClient(
		newHTTPClient(insecure), cfg.Client.ServerURL,
		connect.WithInterceptors(bearerInterceptor(token)),
	)
	return client, format, timeout, nil
}

// isTerminal reports whether f is an interactive character device. Failing
// toward false (JSON) on a Stat error is the safe default for an unknown
// environment (D-05).
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// outputFormat is the resolved rendering choice for a client command's
// response (D-05/D-06).
type outputFormat int

const (
	formatJSON outputFormat = iota
	formatText
)

// outputFormatFromConfig maps an already-validated cfg.Client.Output value
// and the caller's TTY state to a concrete rendering choice. The registry's
// own client validator has already rejected any value outside
// "json"/"text"/"" before this function ever runs, so its default case
// handles only the legal empty value — there is no second rejection site
// here, unlike the retired resolveOutputFormat this replaces. Taking isTTY
// as a parameter rather than calling isTerminal internally is what lets a
// table test force both branches without a pty.
func outputFormatFromConfig(output string, isTTY bool) outputFormat {
	switch output {
	case "json":
		return formatJSON
	case "text":
		return formatText
	default: // "" — detect from stdout
		if isTTY {
			return formatText
		}
		return formatJSON
	}
}

// Exit-code taxonomy (D-09). These constants are the single source of
// truth for the process exit code every client command can produce; Plan
// 03's self-describe catalog is built from them, not a parallel literal
// list (D-11).
const (
	exitOK          = 0 // success
	exitGeneric     = 1 // generic/unclassified
	exitUsage       = 2 // usage or validation error
	exitAuth        = 3 // authentication or authorization failure
	exitNotFound    = 4 // not found
	exitUnavailable = 5 // transport or server unavailable
	exitTimeout     = 6 // request deadline exceeded (D-06)
	// exitFindings is produced by a report command's own explicit opt-in
	// flag (e.g. `spine-review verify --fail-on`) -- and by NO connect-error
	// mapping, which is exactly why TestCatalogExitCodesMatchMapper's
	// derivation must know about it via a named allowlist entry
	// (catalog_test.go's nonConnectProducedCodes) rather than deriving it
	// from exitCodeForConnectErr like every other code in this block. The
	// command itself succeeded; the data just didn't pass a check the
	// caller opted into.
	exitFindings = 7
)

// cliError carries an explicit process exit code alongside a wrapped
// error. Execute() in root.go consults ExitCode() via errors.As.
type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }
func (e *cliError) ExitCode() int { return e.code }

// usageErrorf returns a *cliError carrying exitUsage — the client's own
// semantic validation (D-17 reserves exit 2 for exactly this).
func usageErrorf(format string, a ...any) error {
	return &cliError{code: exitUsage, err: fmt.Errorf(format, a...)}
}

// requireScopeUnlessCrossSpine is the shared D-01/D-02/D-04 pre-flight guard
// used by both searchCmd and listCmd, before dialing. It enforces the one
// rule that survives here after D-07: --scope is required unless
// --cross-spine is set.
//
// The symmetric half of the old guard — --scope and --cross-spine are
// mutually exclusive — moved to a declared cobra flag group
// (MarkFlagsMutuallyExclusive("scope", "cross-spine")) on each of
// searchCmd/listCmd, validated centrally by rootCmd.PersistentPreRunE before
// this function ever runs. It is deliberately NOT re-enforced here as a
// defense-in-depth backstop: a second enforcement of a rule cobra now owns
// is the exact "fourth hand-rolled guard" SC1 exists to eliminate, and a
// backstop that can never fire reads as an active rule to a future reader.
//
// This asymmetric rule has no declarative cobra equivalent:
// MarkFlagsOneRequired expresses "at least one of N flags is required",
// never "A is required unless B is set" — crossSpine=false, scope="" is the
// only rejected row here, which is not a symmetric relationship a flag group
// can express.
//
// It is deliberately STRICTER than internal/server's effectiveSearchScope on
// the scope-plus-cross-spine row — now rejected before this function ever
// runs, by the cobra group described above — because the server accepts
// that combination and silently discards the scope (Phase 3 D-02), logged
// server-side at Info where the calling agent never sees it: an
// explicitly-typed filter being silently ignored is exactly the "surprise"
// D-00 forbids. See TestValidateScopeCrossSpineParity in
// client_common_test.go, which pins this exact divergence as deliberate; do
// not "fix" it into agreement.
func requireScopeUnlessCrossSpine(scope string, crossSpine bool) error {
	if !crossSpine && scope == "" {
		return usageErrorf("--scope is required unless --cross-spine is set")
	}
	return nil
}

// renderCoverageFooter writes the D-05 cross-spine coverage footer to w: a
// count of the scopes the caller was authorized to read, and whether that
// span was truncated. It returns nil immediately when crossSpine is false —
// putting the D-06 "no footer on a scope-confined call" condition inside
// the helper, rather than at each call site, makes it structurally
// impossible for a caller to forget.
//
// Gating on the caller's own crossSpine value (rather than on
// searchedScopes being non-empty) matters: a cross-spine caller authorized
// to read zero scopes still deserves an honest count of zero, and gating on
// the request rather than the response is what makes D-06's byte-identical
// guarantee independent of what the server happens to return.
//
// The footer reports a COUNT, never the scope names (D-05) — naming them is
// unbounded in width and would need its own truncation rule layered on top
// of the server's own scopes_truncated signal. Key names are the proto
// field names verbatim (searched_scopes / scopes_truncated), so the text
// lane and the JSON lane agree about what the coverage information is
// called (Phase 2 D-08).
func renderCoverageFooter(w io.Writer, crossSpine bool, searchedScopes []string, scopesTruncated bool) error {
	if !crossSpine {
		return nil
	}
	if scopesTruncated {
		_, err := fmt.Fprintf(w, "searched_scopes: %d  scopes_truncated: true\n", len(searchedScopes))
		return err
	}
	_, err := fmt.Fprintf(w, "searched_scopes: %d\n", len(searchedScopes))
	return err
}

// wrapRPCError wraps an RPC error in a *cliError whose code is
// exitCodeForConnectErr(err) (D-10).
func wrapRPCError(err error) error {
	return &cliError{code: exitCodeForConnectErr(err), err: err}
}

// exitCodeForConnectErr is the single D-10 mapper from a Connect error code
// to a process exit code.
//
// connect.CodeOf alone is sufficient here; no separate transport check is
// needed. The connect-go client wraps any http.Client.Do failure that is
// not already a *connect.Error as CodeUnavailable, and a context deadline
// or cancellation as CodeDeadlineExceeded/CodeCanceled
// [connectrpc.com/connect@v1.20.0 duplex_http_call.go:313-330,
// error.go:293-313 — see 02-RESEARCH.md Pitfall 5], so a genuine dial
// failure never arrives here as CodeUnknown. Do not classify by matching
// on the error's text.
//
// D-06 splits CodeDeadlineExceeded out of the exitUnavailable arm into its
// own exitTimeout: "the server never answered in time" and "I could not
// connect at all" call for different operator responses — raise the
// timeout versus check the server is up — and folding both into 5 made
// them indistinguishable. CodeCanceled deliberately stays on
// exitUnavailable alongside CodeUnavailable: a caller-initiated
// cancellation (e.g. Ctrl-C) is not a server that failed to answer, so it
// does not belong on exitTimeout either. See
// TestExitCodeTimeoutDistinctFromUnavailable, which asserts 5 and 6 by
// explicit inequality, not by set membership alone (memory 667p88n2be: a
// test that only checks "not the default" still passes on a silent
// collapse).
func exitCodeForConnectErr(err error) int {
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated, connect.CodePermissionDenied:
		return exitAuth
	case connect.CodeNotFound:
		return exitNotFound
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeOutOfRange:
		return exitUsage
	case connect.CodeDeadlineExceeded:
		return exitTimeout
	case connect.CodeUnavailable, connect.CodeCanceled:
		return exitUnavailable
	default:
		return exitGeneric
	}
}

// renderJSON marshals m as a single JSON document (D-08 — one object per
// invocation, not NDJSON) and writes it plus a trailing newline.
//
// Two option choices are load-bearing. EmitDefaultValues is what makes an
// empty result render as "memories":[] rather than omitting the key or
// emitting null (D-12). UseProtoNames keeps the field names identical to
// the .proto declaration and therefore to the short_id / created_at /
// summary_source vocabulary the MCP tool surface and CLAUDE.md's memory
// contract already use — deriving field names from the message rather than
// a hand-written Go struct is what makes D-08's "mirror the response field
// names" structurally true instead of a convention someone can drift from.
func renderJSON(w io.Writer, m proto.Message) error {
	b, err := protojson.MarshalOptions{
		UseProtoNames:     true,
		EmitDefaultValues: true,
		Multiline:         false,
	}.Marshal(m)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// renderMemoryTable renders mems as a human-readable table via
// text/tabwriter. withScore adds a SCORE column (search results carry a
// meaningful score; list results do not).
func renderMemoryTable(w io.Writer, mems []*engramv1.Memory, withScore bool) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	var writeErr error
	writeLine := func(format string, a ...any) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintf(tw, format, a...)
	}
	if withScore {
		writeLine("SHORT_ID\tSCOPE\tCATEGORY\tSCORE\tSUMMARY\n")
	} else {
		writeLine("SHORT_ID\tSCOPE\tCATEGORY\tSUMMARY\n")
	}
	for _, m := range mems {
		summary := truncateSummary(m.GetSummary(), 80)
		if withScore {
			writeLine("%s\t%s\t%s\t%.4f\t%s\n",
				m.GetShortId(), m.GetScope(), m.GetCategory(), m.GetScore(), summary)
		} else {
			writeLine("%s\t%s\t%s\t%s\n",
				m.GetShortId(), m.GetScope(), m.GetCategory(), summary)
		}
	}
	if writeErr != nil {
		return writeErr
	}
	return tw.Flush()
}

// truncateSummary reduces s to a single line, truncated to at most n
// runes, for the fixed-width table renderer.
func truncateSummary(s string, n int) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
