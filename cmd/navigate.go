package cmd

import (
	"fmt"
	"os"

	"ghostrun/internal/callgraph"
	"ghostrun/internal/loader"
	"ghostrun/internal/server"

	"github.com/spf13/cobra"
)

var navPort int

var navigateCmd = &cobra.Command{
	Use:   "navigate [package-path]",
	Short: "Browse the call graph of a Go package",
	Long: `Navigate loads a Go package and displays an interactive call graph
in a web browser. No execution or inputs needed — just browse the code structure.

Example:
  ghostrun navigate ./pkg/order`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pkgPath := args[0]

		fmt.Printf("📦 Loading packages from %s...\n", pkgPath)

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

		// Build call graph
		graph := callgraph.Build(pd.Packages, pd.Fset)
		fmt.Printf("📊 Call graph: %d nodes, %d edges\n", len(graph.Nodes), len(graph.Edges))

		// Start web server
		srv := server.New(navPort)
		srv.SetCallGraph(graph)
		srv.SetPackageData(pd)

		fmt.Printf("\n🌐 Opening navigator at http://localhost:%d\n", navPort)
		fmt.Println("   Press Ctrl+C to stop")

		return srv.Start()
	},
}

func init() {
	navigateCmd.Flags().IntVarP(&navPort, "port", "p", 8080, "Port for the web UI")
	rootCmd.AddCommand(navigateCmd)
}
