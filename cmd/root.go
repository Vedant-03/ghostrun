package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dryrun",
	Short: "Static Go code tracer — dry run your code without executing it",
	Long: `DryRun parses Go source code and lets you trace execution step-by-step,
tracking variable values and following function calls — all without compiling or running the code.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
