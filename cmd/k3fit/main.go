// Package main is the K3Fit CLI entry point.
package main

import (
	"os"

	k3fit "github.com/SuperMarioYL/k3fit"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the K3Fit version",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("k3fit v%s\n", k3fit.Version)
	},
}
