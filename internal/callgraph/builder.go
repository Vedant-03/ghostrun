package callgraph

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// Node represents a function in the call graph
type Node struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	PkgPath  string `json:"pkg_path"`
	PkgName  string `json:"pkg_name"`
	Recv     string `json:"recv,omitempty"` // receiver type for methods
	File     string `json:"file"`
	Line     int    `json:"line"`
	EndLine  int    `json:"end_line"`
	IsMethod bool   `json:"is_method"`
}

// Edge represents a function call in the call graph
type Edge struct {
	CallerID string `json:"caller_id"`
	CalleeID string `json:"callee_id"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Label    string `json:"label,omitempty"` // the call expression text
}

// Graph represents the complete call graph
type Graph struct {
	Nodes map[string]*Node `json:"nodes"`
	Edges []Edge           `json:"edges"`
}

// Build constructs a call graph from loaded packages
func Build(pkgs []*packages.Package, fset *token.FileSet) *Graph {
	g := &Graph{
		Nodes: make(map[string]*Node),
		Edges: []Edge{},
	}

	for _, pkg := range pkgs {
		buildFromPackage(g, pkg, fset)
	}

	return g
}

func buildFromPackage(g *Graph, pkg *packages.Package, fset *token.FileSet) {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			callerID := funcID(pkg, fn)
			pos := fset.Position(fn.Pos())
			endPos := fset.Position(fn.End())

			// Register the function node
			node := &Node{
				ID:      callerID,
				Name:    fn.Name.Name,
				PkgPath: pkg.PkgPath,
				PkgName: pkg.Name,
				File:    pos.Filename,
				Line:    pos.Line,
				EndLine: endPos.Line,
			}

			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				node.IsMethod = true
				node.Recv = recvTypeName(fn.Recv.List[0].Type)
			}

			g.Nodes[callerID] = node

			// Walk the function body to find call expressions
			if fn.Body != nil {
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}

					calleeID, label := resolveCallee(pkg, call, fset)
					if calleeID == "" {
						return true
					}

					callPos := fset.Position(call.Pos())
					g.Edges = append(g.Edges, Edge{
						CallerID: callerID,
						CalleeID: calleeID,
						File:     callPos.Filename,
						Line:     callPos.Line,
						Label:    label,
					})

					return true
				})
			}
		}
	}
}

// funcID creates a unique ID for a function
func funcID(pkg *packages.Package, fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv := recvTypeName(fn.Recv.List[0].Type)
		return fmt.Sprintf("%s.%s.%s", pkg.PkgPath, recv, fn.Name.Name)
	}
	return fmt.Sprintf("%s.%s", pkg.PkgPath, fn.Name.Name)
}

// recvTypeName extracts the type name from a receiver expression
func recvTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.IndexExpr:
		return recvTypeName(t.X)
	default:
		return "unknown"
	}
}

// resolveCallee tries to determine the callee function ID and a label
func resolveCallee(pkg *packages.Package, call *ast.CallExpr, fset *token.FileSet) (string, string) {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		// Local function call: e.g., calculateTotal(...)
		if obj, ok := pkg.TypesInfo.Uses[fn]; ok {
			if f, ok := obj.(*types.Func); ok {
				pkgPath := ""
				if f.Pkg() != nil {
					pkgPath = f.Pkg().Path()
				}
				recv := ""
				if sig, ok := f.Type().(*types.Signature); ok && sig.Recv() != nil {
					recv = typeName(sig.Recv().Type())
					return fmt.Sprintf("%s.%s.%s", pkgPath, recv, fn.Name), fn.Name
				}
				_ = recv
				return fmt.Sprintf("%s.%s", pkgPath, fn.Name), fn.Name
			}
		}
		// Could be a type conversion or builtin
		return "", ""

	case *ast.SelectorExpr:
		// Method call or qualified function: e.g., d.Calculate(...) or order.ProcessOrder(...)
		sel := pkg.TypesInfo.Selections[fn]
		if sel != nil {
			// Method call
			obj := sel.Obj()
			if f, ok := obj.(*types.Func); ok {
				pkgPath := ""
				if f.Pkg() != nil {
					pkgPath = f.Pkg().Path()
				}
				recv := typeName(sel.Recv())
				label := fmt.Sprintf("%s.%s", recv, fn.Sel.Name)
				return fmt.Sprintf("%s.%s.%s", pkgPath, recv, fn.Sel.Name), label
			}
		}

		// Qualified function call (pkg.Func)
		if obj, ok := pkg.TypesInfo.Uses[fn.Sel]; ok {
			if f, ok := obj.(*types.Func); ok {
				pkgPath := ""
				if f.Pkg() != nil {
					pkgPath = f.Pkg().Path()
				}
				label := fn.Sel.Name
				if ident, ok := fn.X.(*ast.Ident); ok {
					label = fmt.Sprintf("%s.%s", ident.Name, fn.Sel.Name)
				}
				return fmt.Sprintf("%s.%s", pkgPath, fn.Sel.Name), label
			}
		}
		return "", ""

	default:
		return "", ""
	}
}

// typeName extracts a clean type name from a types.Type
func typeName(t types.Type) string {
	switch typ := t.(type) {
	case *types.Named:
		return typ.Obj().Name()
	case *types.Pointer:
		return typeName(typ.Elem())
	default:
		return t.String()
	}
}

// Callees returns the IDs of all functions called by the given function
func (g *Graph) Callees(funcID string) []string {
	var result []string
	for _, edge := range g.Edges {
		if edge.CallerID == funcID {
			result = append(result, edge.CalleeID)
		}
	}
	return result
}

// Callers returns the IDs of all functions that call the given function
func (g *Graph) Callers(funcID string) []string {
	var result []string
	for _, edge := range g.Edges {
		if edge.CalleeID == funcID {
			result = append(result, edge.CallerID)
		}
	}
	return result
}

// ToJSON serializes the graph to JSON
func (g *Graph) ToJSON() ([]byte, error) {
	return json.MarshalIndent(g, "", "  ")
}

// NodeList returns all nodes as a slice (for display)
func (g *Graph) NodeList() []*Node {
	nodes := make([]*Node, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nodes = append(nodes, n)
	}
	return nodes
}
