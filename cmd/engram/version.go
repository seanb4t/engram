package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the engram version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println(version)
	},
}
