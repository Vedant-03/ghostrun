package interpreter

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"os"

	"dryrun/internal/loader"
	"dryrun/internal/scope"
	"dryrun/internal/trace"
	"dryrun/internal/value"
)

// StubFunc is a user-defined stub that returns fixed values for external functions
type StubFunc func(args []value.Value) value.Value

// Interpreter is the main AST tree-walking interpreter
type Interpreter struct {
	loader   *loader.PackageData
	tracer   *trace.Recorder
	scope    *scope.Scope
	stubs    map[string]StubFunc
	maxSteps int
	steps    int
}

// New creates a new interpreter for the given loaded packages
func New(ld *loader.PackageData) *Interpreter {
	return &Interpreter{
		loader:   ld,
		tracer:   trace.NewRecorder(),
		scope:    scope.New("global"),
		stubs:    make(map[string]StubFunc),
		maxSteps: 10000,
	}
}

// RegisterStub registers a stub function for external calls
func (interp *Interpreter) RegisterStub(name string, stub StubFunc) {
	interp.stubs[name] = stub
}

// Trace executes a function with the given inputs and returns the trace tree
func (interp *Interpreter) Trace(funcName string, inputJSON string) (*trace.TraceNode, error) {
	funcInfo, err := interp.loader.FindFunction(funcName)
	if err != nil {
		return nil, err
	}

	var inputs map[string]interface{}
	if inputJSON != "" {
		if err := json.Unmarshal([]byte(inputJSON), &inputs); err != nil {
			return nil, fmt.Errorf("parsing input JSON: %w", err)
		}
	}

	args, err := interp.buildArgs(funcInfo, inputs)
	if err != nil {
		return nil, fmt.Errorf("building arguments: %w", err)
	}

	interp.callFunc(funcInfo, args, nil)

	return interp.tracer.Result(), nil
}

// TraceWithArgs executes a function with pre-built Value arguments
func (interp *Interpreter) TraceWithArgs(funcName string, args []value.Value) (*trace.TraceNode, error) {
	funcInfo, err := interp.loader.FindFunction(funcName)
	if err != nil {
		return nil, err
	}

	interp.callFunc(funcInfo, args, nil)

	return interp.tracer.Result(), nil
}

// GetTracer returns the trace recorder
func (interp *Interpreter) GetTracer() *trace.Recorder {
	return interp.tracer
}

// buildArgs maps JSON input to function parameter Values
func (interp *Interpreter) buildArgs(funcInfo *loader.FuncInfo, inputs map[string]interface{}) ([]value.Value, error) {
	if funcInfo.Decl.Type.Params == nil {
		return nil, nil
	}

	var args []value.Value
	for _, field := range funcInfo.Decl.Type.Params.List {
		for _, name := range field.Names {
			val, ok := inputs[name.Name]
			if !ok {
				args = append(args, value.NewNil())
				continue
			}
			args = append(args, jsonToValue(val))
		}
	}
	return args, nil
}

// jsonToValue converts a JSON-parsed interface{} to a Value
func jsonToValue(v interface{}) value.Value {
	switch val := v.(type) {
	case float64:
		if val == float64(int64(val)) {
			return value.NewInt(int64(val))
		}
		return value.NewFloat(val)
	case string:
		return value.NewString(val)
	case bool:
		return value.NewBool(val)
	case nil:
		return value.NewNil()
	case map[string]interface{}:
		fields := make(map[string]value.Value)
		for k, v := range val {
			fields[k] = jsonToValue(v)
		}
		return value.NewStruct("", fields)
	case []interface{}:
		elems := make([]value.Value, len(val))
		for i, v := range val {
			elems[i] = jsonToValue(v)
		}
		return value.NewSlice("", elems...)
	default:
		return value.NewNil()
	}
}

// checkStepLimit prevents infinite loops
func (interp *Interpreter) checkStepLimit() bool {
	interp.steps++
	return interp.steps <= interp.maxSteps
}

// readSourceLine reads a source line from a file (best effort)
func readSourceLine(file string, line int) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	lines := splitLines(data)
	if line > 0 && line <= len(lines) {
		return lines[line-1]
	}
	return ""
}

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

// suppress unused import warnings
var (
	_ ast.Expr
	_ = os.ReadFile
)
