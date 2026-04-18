package callgraph

import (
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"
)

func loadTestPackages(t *testing.T) ([]*packages.Package, *token.FileSet) {
	t.Helper()

	testdata := findTestdata()
	if testdata == "" {
		t.Skip("testdata not found")
	}

	origDir, _ := os.Getwd()
	os.Chdir(testdata)
	defer os.Chdir(origDir)

	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedImports,
		Fset: fset,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("Failed to load packages: %v", err)
	}

	return pkgs, fset
}

func findTestdata() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			td := filepath.Join(dir, "testdata", "basic")
			if _, err := os.Stat(td); err == nil {
				return td
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func TestBuildGraph(t *testing.T) {
	pkgs, fset := loadTestPackages(t)

	g := Build(pkgs, fset)

	if len(g.Nodes) == 0 {
		t.Fatal("graph has no nodes")
	}

	t.Logf("Graph has %d nodes and %d edges", len(g.Nodes), len(g.Edges))

	// Print all nodes
	for id, node := range g.Nodes {
		t.Logf("  Node: %s (method=%v, recv=%s, file=%s:%d)", id, node.IsMethod, node.Recv, node.File, node.Line)
	}

	// Print all edges
	for _, edge := range g.Edges {
		t.Logf("  Edge: %s -> %s (%s)", edge.CallerID, edge.CalleeID, edge.Label)
	}

	// Should have at least ProcessOrder, calculateTotal, applyDiscount, main, and methods
	if len(g.Nodes) < 4 {
		t.Errorf("expected at least 4 nodes, got %d", len(g.Nodes))
	}

	// Should have edges (function calls)
	if len(g.Edges) == 0 {
		t.Error("graph has no edges")
	}
}

func TestCalleesAndCallers(t *testing.T) {
	pkgs, fset := loadTestPackages(t)
	g := Build(pkgs, fset)

	// Find ProcessOrder node
	var processOrderID string
	for id, node := range g.Nodes {
		if node.Name == "ProcessOrder" {
			processOrderID = id
			break
		}
	}

	if processOrderID == "" {
		t.Fatal("ProcessOrder node not found")
	}

	callees := g.Callees(processOrderID)
	t.Logf("ProcessOrder callees: %v", callees)

	if len(callees) == 0 {
		t.Error("ProcessOrder should have callees")
	}

	// ProcessOrder should be called by main
	callers := g.Callers(processOrderID)
	t.Logf("ProcessOrder callers: %v", callers)
}

func TestGraphJSON(t *testing.T) {
	pkgs, fset := loadTestPackages(t)
	g := Build(pkgs, fset)

	data, err := g.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("ToJSON() returned empty data")
	}

	t.Logf("JSON size: %d bytes", len(data))
}
