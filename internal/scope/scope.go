package scope

import (
	"fmt"
	"ghostrun/internal/value"
)

// Scope represents a variable binding scope with optional parent
type Scope struct {
	parent   *Scope
	bindings map[string]value.Value
	name     string // for debugging: "global", "func:ProcessOrder", "block:if", etc.
}

// New creates a new root scope
func New(name string) *Scope {
	return &Scope{
		bindings: make(map[string]value.Value),
		name:     name,
	}
}

// Child creates a child scope with this scope as parent
func (s *Scope) Child(name string) *Scope {
	return &Scope{
		parent:   s,
		bindings: make(map[string]value.Value),
		name:     name,
	}
}

// Define creates a new variable in the current scope.
// This is for := and var declarations — always creates in current scope even if parent has same name.
func (s *Scope) Define(name string, val value.Value) {
	s.bindings[name] = val
}

// Set updates an existing variable, walking up the scope chain.
// Returns error if variable is not found in any scope.
func (s *Scope) Set(name string, val value.Value) error {
	current := s
	for current != nil {
		if _, ok := current.bindings[name]; ok {
			current.bindings[name] = val
			return nil
		}
		current = current.parent
	}
	return fmt.Errorf("undefined variable: %s", name)
}

// Get retrieves a variable value, walking up the scope chain.
func (s *Scope) Get(name string) (value.Value, bool) {
	current := s
	for current != nil {
		if val, ok := current.bindings[name]; ok {
			return val, true
		}
		current = current.parent
	}
	return nil, false
}

// Has checks if a variable exists in any scope in the chain.
func (s *Scope) Has(name string) bool {
	_, ok := s.Get(name)
	return ok
}

// HasLocal checks if a variable exists in the current scope only (not parent).
func (s *Scope) HasLocal(name string) bool {
	_, ok := s.bindings[name]
	return ok
}

// Snapshot returns a copy of all visible variables (for trace recording).
// Variables in child scopes shadow parent scopes.
func (s *Scope) Snapshot() map[string]value.Value {
	result := make(map[string]value.Value)

	// Collect all scopes from root to current
	var chain []*Scope
	current := s
	for current != nil {
		chain = append(chain, current)
		current = current.parent
	}

	// Walk from root to current so child values shadow parent values
	for i := len(chain) - 1; i >= 0; i-- {
		for k, v := range chain[i].bindings {
			result[k] = v
		}
	}

	return result
}

// LocalSnapshot returns only the variables defined in the current scope (not parents).
func (s *Scope) LocalSnapshot() map[string]value.Value {
	result := make(map[string]value.Value)
	for k, v := range s.bindings {
		result[k] = v
	}
	return result
}

// Name returns the scope name
func (s *Scope) Name() string {
	return s.name
}

// Parent returns the parent scope
func (s *Scope) Parent() *Scope {
	return s.parent
}

// Depth returns how deep this scope is in the chain (0 for root)
func (s *Scope) Depth() int {
	depth := 0
	current := s.parent
	for current != nil {
		depth++
		current = current.parent
	}
	return depth
}

// String returns a debug representation
func (s *Scope) String() string {
	return fmt.Sprintf("Scope(%s, %d vars, depth=%d)", s.name, len(s.bindings), s.Depth())
}
