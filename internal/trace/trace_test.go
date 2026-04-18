package trace

import (
	"encoding/json"
	"testing"

	"ghostrun/internal/value"
)

func TestRecorderBasic(t *testing.T) {
	r := NewRecorder()

	vars := map[string]value.Value{
		"x": value.NewInt(10),
		"y": value.NewString("hello"),
	}

	r.EnterFunc("ProcessOrder", "order.go", 10, vars)
	r.RecordAssign("total", value.NewFloat(100.0), "order.go", 12, vars)
	r.RecordBranch("total > 50", true, "order.go", 14, vars)
	r.ExitFunc(value.NewInt(0))

	result := r.Result()

	if result.FuncName != "ProcessOrder" {
		t.Errorf("FuncName = %q, want %q", result.FuncName, "ProcessOrder")
	}
	if result.StepType != StepFuncEnter {
		t.Errorf("StepType = %q, want %q", result.StepType, StepFuncEnter)
	}
	if len(result.Children) != 3 { // assign + branch + exit
		t.Errorf("Children count = %d, want 3", len(result.Children))
	}
}

func TestRecorderNestedCalls(t *testing.T) {
	r := NewRecorder()

	r.EnterFunc("main", "main.go", 1, nil)
	r.RecordAssign("x", value.NewInt(1), "main.go", 2, nil)

	// Nested call
	r.EnterFunc("helper", "helper.go", 10, nil)
	r.RecordAssign("y", value.NewInt(2), "helper.go", 11, nil)
	r.ExitFunc(value.NewInt(42))

	r.ExitFunc(value.NewNil())

	result := r.Result()
	if result.FuncName != "main" {
		t.Errorf("root FuncName = %q, want %q", result.FuncName, "main")
	}

	// main should have: assign(x) + func_enter(helper) + func_exit(main)
	if len(result.Children) < 2 {
		t.Fatalf("main Children count = %d, want at least 2", len(result.Children))
	}

	// Find the helper call
	var helperNode *TraceNode
	for _, child := range result.Children {
		if child.FuncName == "helper" {
			helperNode = child
			break
		}
	}
	if helperNode == nil {
		t.Fatal("helper function node not found")
	}
	if helperNode.ReturnValue != "42" {
		t.Errorf("helper ReturnValue = %q, want %q", helperNode.ReturnValue, "42")
	}
}

func TestRecorderToJSON(t *testing.T) {
	r := NewRecorder()
	r.EnterFunc("test", "test.go", 1, map[string]value.Value{
		"a": value.NewInt(1),
	})
	r.ExitFunc(value.NewBool(true))

	data, err := r.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	// Should be valid JSON
	var node TraceNode
	if err := json.Unmarshal(data, &node); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	if node.FuncName != "test" {
		t.Errorf("JSON FuncName = %q, want %q", node.FuncName, "test")
	}
}

func TestRecorderTotalSteps(t *testing.T) {
	r := NewRecorder()
	r.EnterFunc("main", "main.go", 1, nil)
	r.RecordAssign("x", value.NewInt(1), "main.go", 2, nil)
	r.RecordAssign("y", value.NewInt(2), "main.go", 3, nil)
	r.ExitFunc(value.NewNil())

	total := r.TotalSteps()
	if total < 3 {
		t.Errorf("TotalSteps() = %d, want at least 3", total)
	}
}

func TestRecorderDisabled(t *testing.T) {
	r := NewRecorder()
	r.SetEnabled(false)

	r.EnterFunc("test", "test.go", 1, nil)
	r.RecordAssign("x", value.NewInt(1), "test.go", 2, nil)
	r.ExitFunc(value.NewNil())

	// Root should have no children when disabled
	if len(r.Root().Children) != 0 {
		t.Errorf("Disabled recorder has %d children, want 0", len(r.Root().Children))
	}
}

func TestVarSnapshot(t *testing.T) {
	vars := map[string]value.Value{
		"count":  value.NewInt(5),
		"name":   value.NewString("test"),
		"active": value.NewBool(true),
	}

	states := snapshotVars(vars)
	if len(states) != 3 {
		t.Errorf("snapshotVars returned %d states, want 3", len(states))
	}

	// Verify all vars are captured
	found := make(map[string]bool)
	for _, s := range states {
		found[s.Name] = true
	}
	for name := range vars {
		if !found[name] {
			t.Errorf("variable %q not found in snapshot", name)
		}
	}
}
