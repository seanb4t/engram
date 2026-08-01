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
	"strings"
	"text/tabwriter"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
)

// Shared client flags (D-03): bound once by addClientFlags and shared by
// every client subcommand.
var (
	clientServerURL string
	clientTokenFile string
	clientInsecure  bool
	clientOutput    string
)

// addClientFlags binds the four flags every client command shares.
func addClientFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&clientServerURL, "server", "",
		"engram server base URL (default: ENGRAM_SERVER_URL)")
	cmd.Flags().StringVar(&clientTokenFile, "token-file", "",
		"path to a file containing the bearer credential (default: ENGRAM_TOKEN env var)")
	cmd.Flags().BoolVar(&clientInsecure, "insecure", false,
		"skip TLS certificate verification (always warns on stderr; no environment fallback)")
	cmd.Flags().StringVar(&clientOutput, "output", "",
		`output format: "json" or "text" (default: detect from stdout)`)
}

// resolveServerURL resolves --server, then ENGRAM_SERVER_URL, in that
// order (D-02). Deliberately NOT baked into the flag's default the way
// reindex.go's os.Getenv-in-default idiom is: resolving at run time here
// instead of at init() time keeps the precedence testable with t.Setenv and
// keeps the env value out of --help output. There is no localhost default —
// a CI job that silently queries nothing is the exact failure D-02 exists
// to prevent.
func resolveServerURL(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if v := os.Getenv("ENGRAM_SERVER_URL"); v != "" {
		return v, nil
	}
	return "", usageErrorf("--server or ENGRAM_SERVER_URL is required")
}

// resolveToken resolves ENGRAM_TOKEN, then the file named by
// tokenFilePath, trimmed of surrounding whitespace (a trailing newline is
// the default state of a file written by a shell redirect, and an
// untrimmed value would otherwise become a malformed multi-field
// credential the server's extractor rejects). Neither set: returns "" with
// a nil error — an anonymous call against a no-issuer server is legal.
// There is no flag that accepts a credential value directly; a credential
// must never be able to reach argv (D-13).
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
// subcommand (D-03): it resolves the server URL, resolves the token, and —
// when --insecure is set — writes an unconditional warning to stderr,
// never to stdout, never gated by --output, never suppressible (D-14/D-07).
//
// It does not add cookie-jar or anti-forgery-token plumbing:
// 02-RESEARCH.md Pattern 3 verified that the server exempts the bearer lane
// totally for write procedures, so any such code here would be
// unreachable against every server this client can talk to.
func clientFromFlags(cmd *cobra.Command) (engramv1connect.EngramServiceClient, error) {
	serverURL, err := resolveServerURL(clientServerURL)
	if err != nil {
		return nil, err
	}
	token, err := resolveToken(clientTokenFile)
	if err != nil {
		return nil, err
	}
	if clientInsecure {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
			"WARNING: TLS certificate verification is disabled (--insecure); "+
				"do not use against an untrusted network")
	}
	return engramv1connect.NewEngramServiceClient(
		newHTTPClient(clientInsecure), serverURL,
		connect.WithInterceptors(bearerInterceptor(token)),
	), nil
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

// resolveOutputFormat maps the --output flag value and the caller's TTY
// state to a concrete format. Taking isTTY as a parameter rather than
// calling isTerminal internally is what lets a table test force both
// branches without a pty.
func resolveOutputFormat(flagVal string, isTTY bool) (outputFormat, error) {
	switch flagVal {
	case "json":
		return formatJSON, nil
	case "text":
		return formatText, nil
	case "":
		if isTTY {
			return formatText, nil
		}
		return formatJSON, nil
	default:
		return formatJSON, usageErrorf(`--output must be "json" or "text", got %q`, flagVal)
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
func exitCodeForConnectErr(err error) int {
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated, connect.CodePermissionDenied:
		return exitAuth
	case connect.CodeNotFound:
		return exitNotFound
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeOutOfRange:
		return exitUsage
	case connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeCanceled:
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
