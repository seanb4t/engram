// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/config"
)

// version is the engram build version, injected at release time via
// -ldflags "-X main.version=X.Y.Z" (goreleaser). Defaults to "dev" for
// local/source builds.
var version = "dev"

// rootCmd silences cobra's own error/usage printing so Execute can render a
// single clean "Error: <msg>" line itself (cobra otherwise reprints usage on
// every RunE error). SilenceErrors without Execute printing would swallow the
// message entirely — see Execute.
var rootCmd = &cobra.Command{
	Use:           "engram",
	Short:         "Self-hosted, correctable, OAuth-secured memory for coding agents",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
	// RunE makes the root runnable, so a bare invocation reaches the D-15
	// self-describe catalog instead of falling into cobra's
	// help-and-exit-0 path. Args is REQUIRED alongside it, not optional and
	// coupled to RunE — do not remove one without the other. cobra's own
	// legacyArgs fallback (used when Args is nil) happens to reject an
	// unmatched first argument for a root command with subcommands too, but
	// that is an incidental property of cobra's internals, not a contract
	// this package should rely on: any future Args value more permissive
	// than cobra.NoArgs (e.g. cobra.ArbitraryArgs) would route a mistyped
	// verb into this RunE and print the catalog at exit 0 instead of
	// failing (see 02-03-PLAN.md's landmine table and 02-03-SUMMARY.md's
	// RED observations).
	RunE: runSelfDescribe,
	Args: cobra.NoArgs,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		return config.CheckLegacy(os.Environ())
	},
}

func init() {
	rootCmd.Version = version
	rootCmd.AddCommand(serveCmd, versionCmd)
}

// Execute runs the root command and exits non-zero on error, printing the error
// to stderr first. Without this print, SilenceErrors would make every failed
// command (e.g. a reindex timeout, Ctrl-C, or embedder failure) exit 1 with no
// message — indistinguishable from success.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(exitCodeFromError(err))
	}
}

// exitCodeFromError returns 0 for a nil error; otherwise it consults an
// ExitCode() int accessor on err via errors.As and returns that, defaulting
// to 1 when the error carries no such method. This is additive and
// backward-compatible by construction: every existing operator command
// (serve, reindex, prune-expired, migrate-remap-owner, summarize-missing,
// backfill-short-ids) returns a plain error with no such method, so its
// exit status is unchanged (D-09). errors.As rather than a bare type
// assertion is what makes this survive an intermediate fmt.Errorf("%w", …)
// wrap.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var ec interface{ ExitCode() int }
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	return 1
}
