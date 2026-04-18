package value

import (
	"testing"
)

func TestPrimitiveValues(t *testing.T) {
	tests := []struct {
		name  string
		val   Value
		typ   string
		str   string
		goVal interface{}
	}{
		{"int", NewInt(42), "int", "42", int64(42)},
		{"float", NewFloat(3.14), "float64", "3.14", 3.14},
		{"string", NewString("hello"), "string", `"hello"`, "hello"},
		{"bool true", NewBool(true), "bool", "true", true},
		{"bool false", NewBool(false), "bool", "false", false},
		{"nil", NewNil(), "nil", "nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.val.Type(); got != tt.typ {
				t.Errorf("Type() = %q, want %q", got, tt.typ)
			}
			if got := tt.val.String(); got != tt.str {
				t.Errorf("String() = %q, want %q", got, tt.str)
			}
			if got := tt.val.GoValue(); got != tt.goVal {
				t.Errorf("GoValue() = %v, want %v", got, tt.goVal)
			}
		})
	}
}

func TestStructValue(t *testing.T) {
	s := NewStruct("Order", map[string]Value{
		"ID":     NewInt(1),
		"Amount": NewFloat(100.5),
	})

	if s.Type() != "Order" {
		t.Errorf("Type() = %q, want %q", s.Type(), "Order")
	}

	// Test GetField
	id, ok := s.GetField("ID")
	if !ok {
		t.Fatal("GetField(ID) returned false")
	}
	if id.GoValue() != int64(1) {
		t.Errorf("GetField(ID) = %v, want 1", id.GoValue())
	}

	// Test SetField
	s.SetField("ID", NewInt(2))
	id, _ = s.GetField("ID")
	if id.GoValue() != int64(2) {
		t.Errorf("After SetField, ID = %v, want 2", id.GoValue())
	}

	// Test missing field
	_, ok = s.GetField("Missing")
	if ok {
		t.Error("GetField(Missing) should return false")
	}
}

func TestSliceValue(t *testing.T) {
	s := NewSlice("int", NewInt(1), NewInt(2), NewInt(3))

	if s.Len() != 3 {
		t.Errorf("Len() = %d, want 3", s.Len())
	}

	v, err := s.Index(1)
	if err != nil {
		t.Fatalf("Index(1) error: %v", err)
	}
	if v.GoValue() != int64(2) {
		t.Errorf("Index(1) = %v, want 2", v.GoValue())
	}

	_, err = s.Index(5)
	if err == nil {
		t.Error("Index(5) should return error")
	}

	s.Append(NewInt(4))
	if s.Len() != 4 {
		t.Errorf("After Append, Len() = %d, want 4", s.Len())
	}
}

func TestMapValue(t *testing.T) {
	m := NewMap("string", "int")
	m.Set("a", NewInt(1))
	m.Set("b", NewInt(2))

	v, ok := m.Get("a")
	if !ok {
		t.Fatal("Get(a) returned false")
	}
	if v.GoValue() != int64(1) {
		t.Errorf("Get(a) = %v, want 1", v.GoValue())
	}

	_, ok = m.Get("missing")
	if ok {
		t.Error("Get(missing) should return false")
	}

	// Test overwrite
	m.Set("a", NewInt(99))
	v, _ = m.Get("a")
	if v.GoValue() != int64(99) {
		t.Errorf("After overwrite, Get(a) = %v, want 99", v.GoValue())
	}
}

func TestPointerValue(t *testing.T) {
	v := NewInt(42)
	p := NewPointer(v)

	if p.Type() != "*int" {
		t.Errorf("Type() = %q, want %q", p.Type(), "*int")
	}
	if p.GoValue() != int64(42) {
		t.Errorf("GoValue() = %v, want 42", p.GoValue())
	}
}

func TestMultiValue(t *testing.T) {
	mv := &MultiValue{Values: []Value{NewInt(42), NewNil()}}
	if mv.Type() != "multi" {
		t.Errorf("Type() = %q, want %q", mv.Type(), "multi")
	}
	if len(mv.Values) != 2 {
		t.Errorf("len(Values) = %d, want 2", len(mv.Values))
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		name string
		val  Value
		want bool
	}{
		{"true", NewBool(true), true},
		{"false", NewBool(false), false},
		{"int 0", NewInt(0), false},
		{"int 1", NewInt(1), true},
		{"empty string", NewString(""), false},
		{"non-empty string", NewString("hello"), true},
		{"nil", NewNil(), false},
		{"struct", NewStruct("S", nil), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTruthy(tt.val); got != tt.want {
				t.Errorf("IsTruthy(%s) = %v, want %v", tt.val.String(), got, tt.want)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b Value
		want bool
	}{
		{"int equal", NewInt(42), NewInt(42), true},
		{"int not equal", NewInt(1), NewInt(2), false},
		{"string equal", NewString("a"), NewString("a"), true},
		{"string not equal", NewString("a"), NewString("b"), false},
		{"bool equal", NewBool(true), NewBool(true), true},
		{"nil equal", NewNil(), NewNil(), true},
		{"int-float cross", NewInt(42), NewFloat(42.0), true},
		{"nil vs int", NewNil(), NewInt(0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Equal(tt.a, tt.b); got != tt.want {
				t.Errorf("Equal(%s, %s) = %v, want %v", tt.a.String(), tt.b.String(), got, tt.want)
			}
		})
	}
}
