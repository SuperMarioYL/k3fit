// Package main is the K3Fit CLI entry point.
package main

import (
	"os"

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
		cmd.Println("k3fit v0.1.0")
	},
}
