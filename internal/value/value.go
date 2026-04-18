package value

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
)

// Value represents any Go value during interpretation
type Value interface {
	Type() string
	String() string
	GoValue() interface{}
}

// --- Primitive Values ---

type IntValue struct{ Val int64 }

func (v IntValue) Type() string         { return "int" }
func (v IntValue) String() string       { return fmt.Sprintf("%d", v.Val) }
func (v IntValue) GoValue() interface{} { return v.Val }

type FloatValue struct{ Val float64 }

func (v FloatValue) Type() string         { return "float64" }
func (v FloatValue) String() string       { return fmt.Sprintf("%g", v.Val) }
func (v FloatValue) GoValue() interface{} { return v.Val }

type StringValue struct{ Val string }

func (v StringValue) Type() string         { return "string" }
func (v StringValue) String() string       { return fmt.Sprintf("%q", v.Val) }
func (v StringValue) GoValue() interface{} { return v.Val }

type BoolValue struct{ Val bool }

func (v BoolValue) Type() string         { return "bool" }
func (v BoolValue) String() string       { return fmt.Sprintf("%t", v.Val) }
func (v BoolValue) GoValue() interface{} { return v.Val }

// NilValue represents nil
type NilValue struct{}

func (v NilValue) Type() string         { return "nil" }
func (v NilValue) String() string       { return "nil" }
func (v NilValue) GoValue() interface{} { return nil }

// --- Composite Values ---

// StructValue represents a Go struct instance
type StructValue struct {
	TypeName string
	Fields   map[string]Value
	TypeInfo types.Type // the Go type info for this struct
}

func (v *StructValue) Type() string { return v.TypeName }
func (v *StructValue) String() string {
	parts := make([]string, 0, len(v.Fields))
	for k, val := range v.Fields {
		parts = append(parts, fmt.Sprintf("%s: %s", k, val.String()))
	}
	return fmt.Sprintf("%s{%s}", v.TypeName, strings.Join(parts, ", "))
}
func (v *StructValue) GoValue() interface{} { return v.Fields }

// GetField returns a field value
func (v *StructValue) GetField(name string) (Value, bool) {
	val, ok := v.Fields[name]
	return val, ok
}

// SetField sets a field value
func (v *StructValue) SetField(name string, val Value) {
	v.Fields[name] = val
}

// SliceValue represents a Go slice
type SliceValue struct {
	ElemType string
	Elements []Value
}

func (v *SliceValue) Type() string { return "[]" + v.ElemType }
func (v *SliceValue) String() string {
	parts := make([]string, len(v.Elements))
	for i, e := range v.Elements {
		parts[i] = e.String()
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
}
func (v *SliceValue) GoValue() interface{} { return v.Elements }
func (v *SliceValue) Len() int              { return len(v.Elements) }

func (v *SliceValue) Index(i int) (Value, error) {
	if i < 0 || i >= len(v.Elements) {
		return nil, fmt.Errorf("index out of range [%d] with length %d", i, len(v.Elements))
	}
	return v.Elements[i], nil
}

func (v *SliceValue) Append(vals ...Value) {
	v.Elements = append(v.Elements, vals...)
}

// MapValue represents a Go map
type MapValue struct {
	KeyType  string
	ValType  string
	Entries  map[string]Value // string key representation -> value
	KeyOrder []string         // maintain insertion order for display
}

func (v *MapValue) Type() string { return fmt.Sprintf("map[%s]%s", v.KeyType, v.ValType) }
func (v *MapValue) String() string {
	parts := make([]string, 0, len(v.Entries))
	for _, k := range v.KeyOrder {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v.Entries[k].String()))
	}
	return fmt.Sprintf("map[%s]", strings.Join(parts, ", "))
}
func (v *MapValue) GoValue() interface{} { return v.Entries }

func (v *MapValue) Get(key string) (Value, bool) {
	val, ok := v.Entries[key]
	return val, ok
}

func (v *MapValue) Set(key string, val Value) {
	if _, exists := v.Entries[key]; !exists {
		v.KeyOrder = append(v.KeyOrder, key)
	}
	v.Entries[key] = val
}

// --- Reference Values ---

// PointerValue represents a pointer to a value
type PointerValue struct {
	Elem     *Value // points to the actual storage
	ElemType string
}

func (v *PointerValue) Type() string { return "*" + v.ElemType }
func (v *PointerValue) String() string {
	if v.Elem == nil || *v.Elem == nil {
		return "nil"
	}
	return fmt.Sprintf("&%s", (*v.Elem).String())
}
func (v *PointerValue) GoValue() interface{} {
	if v.Elem == nil {
		return nil
	}
	return (*v.Elem).GoValue()
}

// FuncValue represents a function reference
type FuncValue struct {
	Name     string
	PkgPath  string
	Decl     *ast.FuncDecl
	Receiver Value // nil for non-method functions
	TypeInfo *types.Func
}

func (v *FuncValue) Type() string { return "func" }
func (v *FuncValue) String() string {
	if v.PkgPath != "" {
		return fmt.Sprintf("%s.%s", v.PkgPath, v.Name)
	}
	return v.Name
}
func (v *FuncValue) GoValue() interface{} { return v.Name }

// MultiValue represents multiple return values
type MultiValue struct {
	Values []Value
}

func (v *MultiValue) Type() string { return "multi" }
func (v *MultiValue) String() string {
	parts := make([]string, len(v.Values))
	for i, val := range v.Values {
		parts[i] = val.String()
	}
	return fmt.Sprintf("(%s)", strings.Join(parts, ", "))
}
func (v *MultiValue) GoValue() interface{} { return v.Values }

// --- Helper Constructors ---

func NewInt(v int64) IntValue        { return IntValue{Val: v} }
func NewFloat(v float64) FloatValue  { return FloatValue{Val: v} }
func NewString(v string) StringValue { return StringValue{Val: v} }
func NewBool(v bool) BoolValue       { return BoolValue{Val: v} }
func NewNil() NilValue               { return NilValue{} }

func NewStruct(typeName string, fields map[string]Value) *StructValue {
	if fields == nil {
		fields = make(map[string]Value)
	}
	return &StructValue{TypeName: typeName, Fields: fields}
}

func NewSlice(elemType string, elems ...Value) *SliceValue {
	return &SliceValue{ElemType: elemType, Elements: elems}
}

func NewMap(keyType, valType string) *MapValue {
	return &MapValue{
		KeyType:  keyType,
		ValType:  valType,
		Entries:  make(map[string]Value),
		KeyOrder: []string{},
	}
}

func NewPointer(elem Value) *PointerValue {
	return &PointerValue{Elem: &elem, ElemType: elem.Type()}
}

// --- Type Helpers ---

// IsTruthy returns whether a value is truthy (for conditions)
func IsTruthy(v Value) bool {
	switch val := v.(type) {
	case BoolValue:
		return val.Val
	case IntValue:
		return val.Val != 0
	case FloatValue:
		return val.Val != 0
	case StringValue:
		return val.Val != ""
	case NilValue:
		return false
	case *StructValue:
		return true
	case *SliceValue:
		return true
	case *MapValue:
		return true
	case *PointerValue:
		return v.GoValue() != nil
	default:
		return true
	}
}

// IsNil checks if a value is nil
func IsNil(v Value) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case NilValue:
		return true
	case *PointerValue:
		return val.Elem == nil || *val.Elem == nil
	default:
		return false
	}
}

// Equal checks if two values are equal
func Equal(a, b Value) bool {
	if IsNil(a) && IsNil(b) {
		return true
	}
	if IsNil(a) || IsNil(b) {
		return false
	}
	switch av := a.(type) {
	case IntValue:
		if bv, ok := b.(IntValue); ok {
			return av.Val == bv.Val
		}
		if bv, ok := b.(FloatValue); ok {
			return float64(av.Val) == bv.Val
		}
	case FloatValue:
		if bv, ok := b.(FloatValue); ok {
			return av.Val == bv.Val
		}
		if bv, ok := b.(IntValue); ok {
			return av.Val == float64(bv.Val)
		}
	case StringValue:
		if bv, ok := b.(StringValue); ok {
			return av.Val == bv.Val
		}
	case BoolValue:
		if bv, ok := b.(BoolValue); ok {
			return av.Val == bv.Val
		}
	}
	return false
}

// ZeroValue returns the zero value for a given Go type
func ZeroValue(t types.Type) Value {
	switch underlying := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case underlying.Info()&types.IsInteger != 0:
			return NewInt(0)
		case underlying.Info()&types.IsFloat != 0:
			return NewFloat(0)
		case underlying.Info()&types.IsString != 0:
			return NewString("")
		case underlying.Info()&types.IsBoolean != 0:
			return NewBool(false)
		}
	case *types.Struct:
		fields := make(map[string]Value)
		for i := 0; i < underlying.NumFields(); i++ {
			f := underlying.Field(i)
			fields[f.Name()] = ZeroValue(f.Type())
		}
		name := ""
		if named, ok := t.(*types.Named); ok {
			name = named.Obj().Name()
		}
		return NewStruct(name, fields)
	case *types.Slice:
		return NewSlice(underlying.Elem().String())
	case *types.Map:
		return NewMap(underlying.Key().String(), underlying.Elem().String())
	case *types.Pointer:
		return NewNil()
	case *types.Interface:
		return NewNil()
	}
	return NewNil()
}
