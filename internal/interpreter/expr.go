package interpreter

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	"dryrun/internal/value"
)

// evalExpr evaluates an AST expression and returns a Value
func (interp *Interpreter) evalExpr(expr ast.Expr) value.Value {
	if expr == nil {
		return value.NewNil()
	}

	switch e := expr.(type) {
	case *ast.BasicLit:
		return interp.evalBasicLit(e)

	case *ast.Ident:
		return interp.evalIdent(e)

	case *ast.BinaryExpr:
		return interp.evalBinaryExpr(e)

	case *ast.UnaryExpr:
		return interp.evalUnaryExpr(e)

	case *ast.ParenExpr:
		return interp.evalExpr(e.X)

	case *ast.CallExpr:
		return interp.evalCallExpr(e)

	case *ast.SelectorExpr:
		return interp.evalSelectorExpr(e)

	case *ast.IndexExpr:
		return interp.evalIndexExpr(e)

	case *ast.CompositeLit:
		return interp.evalCompositeLit(e)

	case *ast.SliceExpr:
		return interp.evalSliceExpr(e)

	case *ast.TypeAssertExpr:
		return interp.evalExpr(e.X)

	case *ast.StarExpr:
		v := interp.evalExpr(e.X)
		if p, ok := v.(*value.PointerValue); ok && p.Elem != nil {
			return *p.Elem
		}
		return v

	case *ast.KeyValueExpr:
		return interp.evalExpr(e.Value)

	case *ast.FuncLit:
		return &value.FuncValue{Name: "<anonymous>"}

	default:
		return value.NewNil()
	}
}

func (interp *Interpreter) evalBasicLit(lit *ast.BasicLit) value.Value {
	switch lit.Kind {
	case token.INT:
		v, _ := strconv.ParseInt(lit.Value, 0, 64)
		return value.NewInt(v)
	case token.FLOAT:
		v, _ := strconv.ParseFloat(lit.Value, 64)
		return value.NewFloat(v)
	case token.STRING:
		v, _ := strconv.Unquote(lit.Value)
		return value.NewString(v)
	case token.CHAR:
		v, _ := strconv.Unquote(lit.Value)
		if len(v) > 0 {
			return value.NewInt(int64(v[0]))
		}
		return value.NewInt(0)
	default:
		return value.NewNil()
	}
}

func (interp *Interpreter) evalIdent(ident *ast.Ident) value.Value {
	switch ident.Name {
	case "true":
		return value.NewBool(true)
	case "false":
		return value.NewBool(false)
	case "nil":
		return value.NewNil()
	}

	if v, ok := interp.scope.Get(ident.Name); ok {
		return v
	}

	return value.NewNil()
}

func (interp *Interpreter) evalBinaryExpr(expr *ast.BinaryExpr) value.Value {
	left := interp.evalExpr(expr.X)

	// Short-circuit for && and ||
	if expr.Op == token.LAND {
		if !value.IsTruthy(left) {
			return value.NewBool(false)
		}
		right := interp.evalExpr(expr.Y)
		return value.NewBool(value.IsTruthy(right))
	}
	if expr.Op == token.LOR {
		if value.IsTruthy(left) {
			return value.NewBool(true)
		}
		right := interp.evalExpr(expr.Y)
		return value.NewBool(value.IsTruthy(right))
	}

	right := interp.evalExpr(expr.Y)

	// String concatenation
	if expr.Op == token.ADD {
		if ls, ok := left.(value.StringValue); ok {
			if rs, ok := right.(value.StringValue); ok {
				return value.NewString(ls.Val + rs.Val)
			}
		}
	}

	// Numeric operations
	leftNum, leftIsNum := toFloat64(left)
	rightNum, rightIsNum := toFloat64(right)

	if leftIsNum && rightIsNum {
		switch expr.Op {
		case token.ADD:
			if isIntPair(left, right) {
				return value.NewInt(toInt64(left) + toInt64(right))
			}
			return value.NewFloat(leftNum + rightNum)
		case token.SUB:
			if isIntPair(left, right) {
				return value.NewInt(toInt64(left) - toInt64(right))
			}
			return value.NewFloat(leftNum - rightNum)
		case token.MUL:
			if isIntPair(left, right) {
				return value.NewInt(toInt64(left) * toInt64(right))
			}
			return value.NewFloat(leftNum * rightNum)
		case token.QUO:
			if rightNum == 0 {
				return value.NewInt(0)
			}
			if isIntPair(left, right) {
				return value.NewInt(toInt64(left) / toInt64(right))
			}
			return value.NewFloat(leftNum / rightNum)
		case token.REM:
			if isIntPair(left, right) && toInt64(right) != 0 {
				return value.NewInt(toInt64(left) % toInt64(right))
			}
			return value.NewInt(0)
		case token.LSS:
			return value.NewBool(leftNum < rightNum)
		case token.GTR:
			return value.NewBool(leftNum > rightNum)
		case token.LEQ:
			return value.NewBool(leftNum <= rightNum)
		case token.GEQ:
			return value.NewBool(leftNum >= rightNum)
		case token.EQL:
			return value.NewBool(leftNum == rightNum)
		case token.NEQ:
			return value.NewBool(leftNum != rightNum)
		}
	}

	// Equality for all types
	switch expr.Op {
	case token.EQL:
		return value.NewBool(value.Equal(left, right))
	case token.NEQ:
		return value.NewBool(!value.Equal(left, right))
	}

	return value.NewNil()
}

func (interp *Interpreter) evalUnaryExpr(expr *ast.UnaryExpr) value.Value {
	operand := interp.evalExpr(expr.X)

	switch expr.Op {
	case token.SUB:
		if v, ok := operand.(value.IntValue); ok {
			return value.NewInt(-v.Val)
		}
		if v, ok := operand.(value.FloatValue); ok {
			return value.NewFloat(-v.Val)
		}
	case token.NOT:
		return value.NewBool(!value.IsTruthy(operand))
	case token.AND:
		return value.NewPointer(operand)
	}
	return operand
}

func (interp *Interpreter) evalSelectorExpr(expr *ast.SelectorExpr) value.Value {
	x := interp.evalExpr(expr.X)

	// Struct field access
	if sv, ok := x.(*value.StructValue); ok {
		if v, found := sv.GetField(expr.Sel.Name); found {
			return v
		}
	}

	// Auto-deref pointer to struct
	if pv, ok := x.(*value.PointerValue); ok && pv.Elem != nil {
		if sv, ok := (*pv.Elem).(*value.StructValue); ok {
			if v, found := sv.GetField(expr.Sel.Name); found {
				return v
			}
		}
	}

	// Method reference on a struct
	if sv, ok := x.(*value.StructValue); ok {
		return &value.FuncValue{
			Name:     expr.Sel.Name,
			Receiver: x,
			PkgPath:  sv.TypeName,
		}
	}

	// Package-qualified identifier
	if ident, ok := expr.X.(*ast.Ident); ok {
		qualName := fmt.Sprintf("%s.%s", ident.Name, expr.Sel.Name)
		if v, found := interp.scope.Get(qualName); found {
			return v
		}
	}

	return value.NewNil()
}

func (interp *Interpreter) evalIndexExpr(expr *ast.IndexExpr) value.Value {
	x := interp.evalExpr(expr.X)
	index := interp.evalExpr(expr.Index)

	switch collection := x.(type) {
	case *value.SliceValue:
		if idx, ok := index.(value.IntValue); ok {
			v, err := collection.Index(int(idx.Val))
			if err != nil {
				return value.NewNil()
			}
			return v
		}
	case *value.MapValue:
		key := index.String()
		if v, ok := collection.Get(key); ok {
			return v
		}
		return value.NewNil()
	}

	return value.NewNil()
}

func (interp *Interpreter) evalCompositeLit(lit *ast.CompositeLit) value.Value {
	typeName := ""
	if lit.Type != nil {
		typeName = exprToString(lit.Type)
	}

	// Slice literal ([]Type{...})
	if lit.Type != nil {
		if _, ok := lit.Type.(*ast.ArrayType); ok {
			elems := make([]value.Value, 0, len(lit.Elts))
			for _, elt := range lit.Elts {
				elems = append(elems, interp.evalExpr(elt))
			}
			return value.NewSlice(typeName, elems...)
		}
	}

	// Map literal (map[K]V{...})
	if lit.Type != nil {
		if _, ok := lit.Type.(*ast.MapType); ok {
			m := value.NewMap("", "")
			for _, elt := range lit.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					key := interp.evalExpr(kv.Key).String()
					val := interp.evalExpr(kv.Value)
					m.Set(key, val)
				}
			}
			return m
		}
	}

	// Struct literal
	fields := make(map[string]value.Value)
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if ident, ok := kv.Key.(*ast.Ident); ok {
				fields[ident.Name] = interp.evalExpr(kv.Value)
			}
		}
	}

	cleanName := typeName
	if idx := lastIndexByte(typeName, '.'); idx >= 0 {
		cleanName = typeName[idx+1:]
	}

	return value.NewStruct(cleanName, fields)
}

func (interp *Interpreter) evalSliceExpr(expr *ast.SliceExpr) value.Value {
	x := interp.evalExpr(expr.X)

	sv, ok := x.(*value.SliceValue)
	if !ok {
		return value.NewNil()
	}

	low := 0
	high := len(sv.Elements)

	if expr.Low != nil {
		if v, ok := interp.evalExpr(expr.Low).(value.IntValue); ok {
			low = int(v.Val)
		}
	}
	if expr.High != nil {
		if v, ok := interp.evalExpr(expr.High).(value.IntValue); ok {
			high = int(v.Val)
		}
	}

	if low < 0 {
		low = 0
	}
	if high > len(sv.Elements) {
		high = len(sv.Elements)
	}

	newElems := make([]value.Value, high-low)
	copy(newElems, sv.Elements[low:high])
	return value.NewSlice(sv.ElemType, newElems...)
}

// --- helpers ---

func toFloat64(v value.Value) (float64, bool) {
	switch val := v.(type) {
	case value.IntValue:
		return float64(val.Val), true
	case value.FloatValue:
		return val.Val, true
	default:
		return 0, false
	}
}

func toInt64(v value.Value) int64 {
	switch val := v.(type) {
	case value.IntValue:
		return val.Val
	case value.FloatValue:
		return int64(val.Val)
	default:
		return 0
	}
}

func isIntPair(a, b value.Value) bool {
	_, aOk := a.(value.IntValue)
	_, bOk := b.(value.IntValue)
	return aOk && bOk
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.ArrayType:
		return "[]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
