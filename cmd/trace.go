package cmd

import (
	"fmt"
	"os"

	"ghostrun/internal/interpreter"
	"ghostrun/internal/loader"
	"ghostrun/internal/server"

	"github.com/spf13/cobra"
)

var (
	traceInput string
	tracePort  int
)

var traceCmd = &cobra.Command{
	Use:   "trace [package-path] [function-name]",
	Short: "Trace a function's execution with sample inputs",
	Long: `Trace interprets a Go function step-by-step without executing it,
tracking variable values and function calls. Results are shown in an interactive web UI.

Example:
  ghostrun trace ./pkg/order ProcessOrder --input '{"o": {"ID": 1, "Amount": 150}}'`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		pkgPath := args[0]
		funcName := args[1]

		fmt.Printf("📦 Loading packages from %s...\n", pkgPath)

		// Change to the directory if it's a relative path
		if stat, err := os.Stat(pkgPath); err == nil && stat.IsDir() {
			origDir, _ := os.Getwd()
			os.Chdir(pkgPath)
			defer os.Chdir(origDir)
			pkgPath = "./..."
		}

		pd, err := loader.Load(pkgPath)
		if err != nil {
			return fmt.Errorf("loading packages: %w", err)
		}

		fmt.Printf("🔍 Found %d functions\n", len(pd.ListFunctions()))
		fmt.Printf("🧪 Tracing %s...\n", funcName)

		interp := interpreter.New(pd)
		traceResult, err := interp.Trace(funcName, traceInput)
		if err != nil {
			return fmt.Errorf("tracing: %w", err)
		}

		totalSteps := interp.GetTracer().TotalSteps()
		fmt.Printf("✅ Trace complete: %d steps recorded\n", totalSteps)

		// Start web server
		srv := server.New(tracePort)
		srv.SetTrace(traceResult)
		srv.SetPackageData(pd)

		fmt.Printf("\n🌐 Opening trace viewer at http://localhost:%d\n", tracePort)
		fmt.Println("   Press Ctrl+C to stop")

		return srv.Start()
	},
}

func init() {
	traceCmd.Flags().StringVarP(&traceInput, "input", "i", "", "JSON input for function parameters")
	traceCmd.Flags().IntVarP(&tracePort, "port", "p", 8080, "Port for the web UI")
	rootCmd.AddCommand(traceCmd)
}
