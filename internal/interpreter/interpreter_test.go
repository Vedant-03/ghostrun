package interpreter

import (
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"go/ast"

	"dryrun/internal/loader"
	"dryrun/internal/trace"
	"dryrun/internal/value"
)

func loadTestdata(t *testing.T) *loader.PackageData {
	t.Helper()
	testdata := findTestdata()
	if testdata == "" {
		t.Skip("testdata not found")
	}

	origDir, _ := os.Getwd()
	os.Chdir(testdata)
	t.Cleanup(func() { os.Chdir(origDir) })

	pd, err := loader.Load("./...")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}
	return pd
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

func TestExprBasicLiterals(t *testing.T) {
	pd := loadTestdata(t)
	interp := New(pd)

	// Test int
	result := interp.evalExpr(&ast.BasicLit{Kind: token.INT, Value: "42"})
	if v, ok := result.(value.IntValue); !ok || v.Val != 42 {
		t.Errorf("int literal: got %v", result)
	}

	// Test float
	result = interp.evalExpr(&ast.BasicLit{Kind: token.FLOAT, Value: "3.14"})
	if v, ok := result.(value.FloatValue); !ok || v.Val != 3.14 {
		t.Errorf("float literal: got %v", result)
	}

	// Test string
	result = interp.evalExpr(&ast.BasicLit{Kind: token.STRING, Value: `"hello"`})
	if v, ok := result.(value.StringValue); !ok || v.Val != "hello" {
		t.Errorf("string literal: got %v", result)
	}
}

func TestExprBinaryOps(t *testing.T) {
	pd := loadTestdata(t)
	interp := New(pd)

	tests := []struct {
		name   string
		a, b   value.Value
		op     token.Token
		expect string
	}{
		{"add_int", value.NewInt(10), value.NewInt(3), token.ADD, "13"},
		{"sub_int", value.NewInt(10), value.NewInt(3), token.SUB, "7"},
		{"mul_int", value.NewInt(10), value.NewInt(3), token.MUL, "30"},
		{"div_int", value.NewInt(10), value.NewInt(3), token.QUO, "3"},
		{"mod_int", value.NewInt(10), value.NewInt(3), token.REM, "1"},
		{"less_than", value.NewInt(3), value.NewInt(10), token.LSS, "true"},
		{"greater_than", value.NewInt(10), value.NewInt(3), token.GTR, "true"},
		{"equal_int", value.NewInt(5), value.NewInt(5), token.EQL, "true"},
		{"not_equal", value.NewInt(5), value.NewInt(3), token.NEQ, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interp.scope.Define("_left", tt.a)
			interp.scope.Define("_right", tt.b)

			result := interp.evalBinaryExpr(&ast.BinaryExpr{
				X:  &ast.Ident{Name: "_left"},
				Op: tt.op,
				Y:  &ast.Ident{Name: "_right"},
			})

			if result.String() != tt.expect {
				t.Errorf("got %s, want %s", result.String(), tt.expect)
			}
		})
	}
}

func TestTraceProcessOrder(t *testing.T) {
	pd := loadTestdata(t)
	interp := New(pd)

	items := value.NewSlice("Item",
		value.NewStruct("Item", map[string]value.Value{
			"Name":  value.NewString("Pizza"),
			"Price": value.NewFloat(100.0),
		}),
		value.NewStruct("Item", map[string]value.Value{
			"Name":  value.NewString("Coke"),
			"Price": value.NewFloat(50.0),
		}),
	)

	order := value.NewStruct("Order", map[string]value.Value{
		"ID":     value.NewInt(1),
		"Amount": value.NewFloat(150.0),
		"Items":  items,
	})

	result, err := interp.TraceWithArgs("ProcessOrder", []value.Value{order})
	if err != nil {
		t.Fatalf("Trace error: %v", err)
	}

	if result == nil {
		t.Fatal("Trace result is nil")
	}

	t.Logf("Trace function: %s", result.FuncName)
	t.Logf("Trace has %d children", len(result.Children))

	if result.FuncName != "ProcessOrder" {
		t.Errorf("Expected ProcessOrder, got %s", result.FuncName)
	}

	foundCalcTotal := findFuncInTrace(result, "calculateTotal")
	foundApplyDiscount := findFuncInTrace(result, "applyDiscount")

	if !foundCalcTotal {
		t.Error("calculateTotal not found in trace")
	}
	if !foundApplyDiscount {
		t.Error("applyDiscount not found in trace")
	}

	data, err := interp.GetTracer().ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}
	t.Logf("Trace JSON size: %d bytes", len(data))
}

func TestTraceCalculateTotal(t *testing.T) {
	pd := loadTestdata(t)
	interp := New(pd)

	items := value.NewSlice("Item",
		value.NewStruct("Item", map[string]value.Value{
			"Name":  value.NewString("A"),
			"Price": value.NewFloat(30.0),
		}),
		value.NewStruct("Item", map[string]value.Value{
			"Name":  value.NewString("B"),
			"Price": value.NewFloat(70.0),
		}),
	)

	result, err := interp.TraceWithArgs("calculateTotal", []value.Value{items})
	if err != nil {
		t.Fatalf("Trace error: %v", err)
	}

	if result == nil {
		t.Fatal("Trace result is nil")
	}

	if result.ReturnValue == "" {
		t.Error("No return value recorded")
	}
	t.Logf("calculateTotal returned: %s", result.ReturnValue)
}

// findFuncInTrace recursively searches the trace tree for a function call
func findFuncInTrace(node *trace.TraceNode, name string) bool {
	if node.FuncName == name {
		return true
	}
	for _, child := range node.Children {
		if findFuncInTrace(child, name) {
			return true
		}
	}
	return false
}
