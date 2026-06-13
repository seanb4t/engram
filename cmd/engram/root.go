// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
		os.Exit(1)
	}
}
