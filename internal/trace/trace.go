package trace

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"dryrun/internal/value"
)

var nodeIDCounter int64

func nextID() int64 {
	return atomic.AddInt64(&nodeIDCounter, 1)
}

// StepType identifies what kind of trace step this is
type StepType string

const (
	StepFuncEnter  StepType = "func_enter"
	StepFuncExit   StepType = "func_exit"
	StepAssign     StepType = "assign"
	StepBranch     StepType = "branch"
	StepLoop       StepType = "loop_iter"
	StepReturn     StepType = "return"
	StepCall       StepType = "call"
	StepExpression StepType = "expression"
)

// VarState represents a variable's value at a point in time
type VarState struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// TraceNode represents a single step in the execution trace
type TraceNode struct {
	ID          int64        `json:"id"`
	StepType    StepType     `json:"step_type"`
	Description string       `json:"description"`
	File        string       `json:"file"`
	Line        int          `json:"line"`
	EndLine     int          `json:"end_line,omitempty"`
	FuncName    string       `json:"func_name,omitempty"`
	Variables   []VarState   `json:"variables,omitempty"`
	ReturnValue string       `json:"return_value,omitempty"`
	Children    []*TraceNode `json:"children,omitempty"`
	SourceLine  string       `json:"source_line,omitempty"`
}

// Recorder builds a trace tree during interpretation
type Recorder struct {
	root    *TraceNode
	stack   []*TraceNode // stack of current function call frames
	stepNum int
	enabled bool
}

// NewRecorder creates a new trace recorder
func NewRecorder() *Recorder {
	root := &TraceNode{
		ID:          nextID(),
		StepType:    StepFuncEnter,
		Description: "root",
		Children:    []*TraceNode{},
	}
	return &Recorder{
		root:    root,
		stack:   []*TraceNode{root},
		enabled: true,
	}
}

// current returns the current frame (top of stack)
func (r *Recorder) current() *TraceNode {
	if len(r.stack) == 0 {
		return r.root
	}
	return r.stack[len(r.stack)-1]
}

// EnterFunc records entering a function call
func (r *Recorder) EnterFunc(name, file string, line int, vars map[string]value.Value) *TraceNode {
	if !r.enabled {
		return nil
	}

	node := &TraceNode{
		ID:          nextID(),
		StepType:    StepFuncEnter,
		Description: fmt.Sprintf("→ %s()", name),
		FuncName:    name,
		File:        file,
		Line:        line,
		Variables:   snapshotVars(vars),
		Children:    []*TraceNode{},
	}

	r.current().Children = append(r.current().Children, node)
	r.stack = append(r.stack, node)
	return node
}

// ExitFunc records exiting a function
func (r *Recorder) ExitFunc(returnVal value.Value) {
	if !r.enabled || len(r.stack) <= 1 {
		return
	}

	current := r.current()
	if returnVal != nil {
		current.ReturnValue = returnVal.String()

		// Add exit step
		exitNode := &TraceNode{
			ID:          nextID(),
			StepType:    StepFuncExit,
			Description: fmt.Sprintf("← return %s", returnVal.String()),
			FuncName:    current.FuncName,
			File:        current.File,
			ReturnValue: returnVal.String(),
		}
		current.Children = append(current.Children, exitNode)
	}

	r.stack = r.stack[:len(r.stack)-1]
}

// RecordStep records a generic execution step
func (r *Recorder) RecordStep(stepType StepType, desc, file string, line int, vars map[string]value.Value) {
	if !r.enabled {
		return
	}

	r.stepNum++
	node := &TraceNode{
		ID:          nextID(),
		StepType:    stepType,
		Description: desc,
		File:        file,
		Line:        line,
		Variables:   snapshotVars(vars),
	}

	r.current().Children = append(r.current().Children, node)
}

// RecordAssign records a variable assignment
func (r *Recorder) RecordAssign(varName string, val value.Value, file string, line int, allVars map[string]value.Value) {
	desc := fmt.Sprintf("%s = %s", varName, val.String())
	r.RecordStep(StepAssign, desc, file, line, allVars)
}

// RecordBranch records a branch decision (if/else/switch)
func (r *Recorder) RecordBranch(condition string, taken bool, file string, line int, vars map[string]value.Value) {
	result := "true → entering"
	if !taken {
		result = "false → skipping"
	}
	desc := fmt.Sprintf("if %s: %s", condition, result)
	r.RecordStep(StepBranch, desc, file, line, vars)
}

// RecordLoopIter records a loop iteration
func (r *Recorder) RecordLoopIter(iterNum int, desc, file string, line int, vars map[string]value.Value) {
	r.RecordStep(StepLoop, fmt.Sprintf("loop iteration %d: %s", iterNum, desc), file, line, vars)
}

// RecordReturn records a return statement
func (r *Recorder) RecordReturn(val value.Value, file string, line int) {
	desc := "return"
	if val != nil {
		desc = fmt.Sprintf("return %s", val.String())
	}
	r.RecordStep(StepReturn, desc, file, line, nil)
}

// Root returns the trace tree root
func (r *Recorder) Root() *TraceNode {
	return r.root
}

// Result returns the first child of root (the top-level function trace)
func (r *Recorder) Result() *TraceNode {
	if len(r.root.Children) > 0 {
		return r.root.Children[0]
	}
	return r.root
}

// TotalSteps returns the total number of steps recorded
func (r *Recorder) TotalSteps() int {
	return countNodes(r.root)
}

// ToJSON serializes the trace tree to JSON
func (r *Recorder) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r.Result(), "", "  ")
}

// SetEnabled enables or disables recording
func (r *Recorder) SetEnabled(enabled bool) {
	r.enabled = enabled
}

// --- helpers ---

func snapshotVars(vars map[string]value.Value) []VarState {
	if vars == nil {
		return nil
	}
	states := make([]VarState, 0, len(vars))
	for name, val := range vars {
		states = append(states, VarState{
			Name:  name,
			Value: val.String(),
			Type:  val.Type(),
		})
	}
	return states
}

func countNodes(node *TraceNode) int {
	count := 1
	for _, child := range node.Children {
		count += countNodes(child)
	}
	return count
}
