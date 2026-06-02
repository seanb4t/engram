package main

import (
	"os"

	"github.com/spf13/cobra"
)

// version is the engram build version, injected at release time via
// -ldflags "-X main.version=X.Y.Z" (goreleaser). Defaults to "dev" for
// local/source builds.
var version = "dev"

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

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
