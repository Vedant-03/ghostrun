package loader

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// FuncInfo holds everything needed to interpret a function
type FuncInfo struct {
	Name    string
	PkgPath string
	Decl    *ast.FuncDecl
	Type    *types.Func
	Recv    string // receiver type name, empty for non-methods
}

// PackageData holds loaded package information
type PackageData struct {
	Fset      *token.FileSet
	Packages  []*packages.Package
	Funcs     map[string]*FuncInfo           // "pkg.Func" or "pkg.Type.Method" -> FuncInfo
	TypesInfo map[string]*packages.Package   // package path -> package
}

// Load loads Go packages from the given patterns (e.g., "./..." or a package path)
func Load(patterns ...string) (*PackageData, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedImports,
		Fset: token.NewFileSet(),
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	// Check for package errors
	var errs []string
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			errs = append(errs, e.Error())
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("package errors:\n%s", strings.Join(errs, "\n"))
	}

	pd := &PackageData{
		Fset:      cfg.Fset,
		Packages:  pkgs,
		Funcs:     make(map[string]*FuncInfo),
		TypesInfo: make(map[string]*packages.Package),
	}

	// Build function registry from all loaded packages
	for _, pkg := range pkgs {
		pd.TypesInfo[pkg.PkgPath] = pkg
		buildFuncRegistry(pkg, pd)
	}

	return pd, nil
}

// buildFuncRegistry walks all files in a package and registers functions/methods
func buildFuncRegistry(pkg *packages.Package, pd *PackageData) {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			info := &FuncInfo{
				Name:    fn.Name.Name,
				PkgPath: pkg.PkgPath,
				Decl:    fn,
			}

			var key string
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				recvType := resolveRecvType(fn.Recv.List[0].Type)
				info.Recv = recvType
				key = fmt.Sprintf("%s.%s.%s", pkg.PkgPath, recvType, fn.Name.Name)

				// Also register with short package name
				shortKey := fmt.Sprintf("%s.%s.%s", pkg.Name, recvType, fn.Name.Name)
				pd.Funcs[shortKey] = info
			} else {
				key = fmt.Sprintf("%s.%s", pkg.PkgPath, fn.Name.Name)

				// Also register with short package name
				shortKey := fmt.Sprintf("%s.%s", pkg.Name, fn.Name.Name)
				pd.Funcs[shortKey] = info
			}

			pd.Funcs[key] = info

			// Also look up the types.Func object
			obj := pkg.Types.Scope().Lookup(fn.Name.Name)
			if fn.Recv != nil {
				// For methods, look up via the receiver type
				recvType := resolveRecvType(fn.Recv.List[0].Type)
				named := pkg.Types.Scope().Lookup(recvType)
				if named != nil {
					if namedType, ok := named.Type().(*types.Named); ok {
						for i := 0; i < namedType.NumMethods(); i++ {
							m := namedType.Method(i)
							if m.Name() == fn.Name.Name {
								info.Type = m
								break
							}
						}
					}
				}
			} else if obj != nil {
				if f, ok := obj.(*types.Func); ok {
					info.Type = f
				}
			}
		}
	}
}

// resolveRecvType extracts the type name from a receiver expression
func resolveRecvType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return resolveRecvType(t.X)
	case *ast.IndexExpr:
		return resolveRecvType(t.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// FindFunction finds a function by name in the loaded packages.
// name can be "FuncName", "pkg.FuncName", or "pkg.Type.Method"
func (pd *PackageData) FindFunction(name string) (*FuncInfo, error) {
	// Direct lookup
	if info, ok := pd.Funcs[name]; ok {
		return info, nil
	}

	// Try with each package prefix
	for _, pkg := range pd.Packages {
		candidates := []string{
			fmt.Sprintf("%s.%s", pkg.PkgPath, name),
			fmt.Sprintf("%s.%s", pkg.Name, name),
		}
		for _, key := range candidates {
			if info, ok := pd.Funcs[key]; ok {
				return info, nil
			}
		}
	}

	// List available functions for error message
	available := make([]string, 0, len(pd.Funcs))
	seen := make(map[string]bool)
	for k := range pd.Funcs {
		if !seen[k] {
			available = append(available, k)
			seen[k] = true
		}
	}

	return nil, fmt.Errorf("function %q not found. Available: %v", name, available)
}

// FindMethod finds a method implementation for a concrete type
func (pd *PackageData) FindMethod(typeName, methodName string) (*FuncInfo, error) {
	// Try various key formats
	var candidates []string
	for _, pkg := range pd.Packages {
		candidates = append(candidates,
			fmt.Sprintf("%s.%s.%s", pkg.PkgPath, typeName, methodName),
			fmt.Sprintf("%s.%s.%s", pkg.Name, typeName, methodName),
		)
	}

	for _, key := range candidates {
		if info, ok := pd.Funcs[key]; ok {
			return info, nil
		}
	}

	return nil, fmt.Errorf("method %s.%s not found", typeName, methodName)
}

// GetSource returns the source code lines for a file
func (pd *PackageData) GetSource(filename string) ([]string, error) {
	for _, pkg := range pd.Packages {
		for _, file := range pkg.Syntax {
			pos := pd.Fset.Position(file.Pos())
			if pos.Filename == filename {
				f := pd.Fset.File(file.Pos())
				if f == nil {
					continue
				}
			}
		}
	}
	return nil, fmt.Errorf("file %q not found in loaded packages", filename)
}

// GetPosition returns file:line for an AST node
func (pd *PackageData) GetPosition(node ast.Node) token.Position {
	return pd.Fset.Position(node.Pos())
}

// GetTypesInfo returns the types.Info for a package
func (pd *PackageData) GetTypesInfo(pkgPath string) *types.Info {
	if pkg, ok := pd.TypesInfo[pkgPath]; ok {
		return pkg.TypesInfo
	}
	return nil
}

// GetExprType returns the Go type of an expression within a specific package
func (pd *PackageData) GetExprType(pkg *packages.Package, expr ast.Expr) types.Type {
	if pkg.TypesInfo != nil {
		if t, ok := pkg.TypesInfo.Types[expr]; ok {
			return t.Type
		}
	}
	return nil
}

// ListFunctions returns all registered function names
func (pd *PackageData) ListFunctions() []string {
	names := make([]string, 0)
	seen := make(map[string]bool)
	for _, info := range pd.Funcs {
		key := info.Name
		if info.Recv != "" {
			key = info.Recv + "." + info.Name
		}
		if !seen[key] {
			names = append(names, key)
			seen[key] = true
		}
	}
	return names
}

// ResolveInterfaceMethod finds the concrete method implementation for an interface method call.
func (pd *PackageData) ResolveInterfaceMethod(concreteTypeName, methodName string) (*FuncInfo, error) {
	return pd.FindMethod(concreteTypeName, methodName)
}
