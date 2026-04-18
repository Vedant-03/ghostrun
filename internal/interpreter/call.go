package interpreter

import (
	"fmt"
	"go/ast"

	"ghostrun/internal/loader"
	"ghostrun/internal/scope"
	"ghostrun/internal/value"
)

// evalCallExpr evaluates a function call expression
func (interp *Interpreter) evalCallExpr(call *ast.CallExpr) value.Value {
	// Evaluate arguments
	args := make([]value.Value, len(call.Args))
	for i, arg := range call.Args {
		args[i] = interp.evalExpr(arg)
	}

	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return interp.callByName(fn.Name, args, nil)

	case *ast.SelectorExpr:
		return interp.callSelector(fn, args)

	default:
		callee := interp.evalExpr(call.Fun)
		if fv, ok := callee.(*value.FuncValue); ok {
			return interp.callFuncValue(fv, args)
		}
		return value.NewNil()
	}
}

// callByName calls a function by its simple name
func (interp *Interpreter) callByName(name string, args []value.Value, receiver value.Value) value.Value {
	if stub, ok := interp.stubs[name]; ok {
		return stub(args)
	}

	if result, ok := interp.callBuiltin(name, args); ok {
		return result
	}

	funcInfo, err := interp.loader.FindFunction(name)
	if err != nil {
		return value.NewNil()
	}

	return interp.callFunc(funcInfo, args, receiver)
}

// callSelector handles method calls and qualified function calls
func (interp *Interpreter) callSelector(sel *ast.SelectorExpr, args []value.Value) value.Value {
	methodName := sel.Sel.Name

	// Check stubs with qualified name
	if ident, ok := sel.X.(*ast.Ident); ok {
		qualName := fmt.Sprintf("%s.%s", ident.Name, methodName)
		if stub, ok := interp.stubs[qualName]; ok {
			return stub(args)
		}
	}

	// Evaluate the receiver/package
	receiver := interp.evalExpr(sel.X)

	// Method call on a struct value
	if sv, ok := receiver.(*value.StructValue); ok {
		return interp.callMethod(sv, sv.TypeName, methodName, args)
	}

	// Method call on a pointer to struct
	if pv, ok := receiver.(*value.PointerValue); ok && pv.Elem != nil {
		if sv, ok := (*pv.Elem).(*value.StructValue); ok {
			return interp.callMethod(sv, sv.TypeName, methodName, args)
		}
	}

	// Qualified function call (e.g., order.ProcessOrder)
	if ident, ok := sel.X.(*ast.Ident); ok {
		qualName := fmt.Sprintf("%s.%s", ident.Name, methodName)

		if stub, ok := interp.stubs[qualName]; ok {
			return stub(args)
		}

		funcInfo, err := interp.loader.FindFunction(qualName)
		if err == nil {
			return interp.callFunc(funcInfo, args, nil)
		}

		if result, ok := interp.callBuiltin(qualName, args); ok {
			return result
		}
	}

	return value.NewNil()
}

// callMethod calls a method on a struct type
func (interp *Interpreter) callMethod(receiver *value.StructValue, typeName, methodName string, args []value.Value) value.Value {
	qualName := fmt.Sprintf("%s.%s", typeName, methodName)
	if stub, ok := interp.stubs[qualName]; ok {
		return stub(args)
	}

	funcInfo, err := interp.loader.FindMethod(typeName, methodName)
	if err != nil {
		return value.NewNil()
	}

	return interp.callFunc(funcInfo, args, receiver)
}

// callFunc executes a function with the interpreter
func (interp *Interpreter) callFunc(funcInfo *loader.FuncInfo, args []value.Value, receiver value.Value) value.Value {
	if funcInfo.Decl.Body == nil {
		return value.NewNil()
	}

	pos := interp.loader.GetPosition(funcInfo.Decl)

	// Create new scope for the function
	funcScope := scope.New(fmt.Sprintf("func:%s", funcInfo.Name))
	parentScope := interp.scope
	interp.scope = funcScope

	// Bind receiver
	if receiver != nil && funcInfo.Decl.Recv != nil && len(funcInfo.Decl.Recv.List) > 0 {
		for _, field := range funcInfo.Decl.Recv.List {
			for _, name := range field.Names {
				interp.scope.Define(name.Name, receiver)
			}
		}
	}

	// Bind parameters
	if funcInfo.Decl.Type.Params != nil {
		argIdx := 0
		for _, field := range funcInfo.Decl.Type.Params.List {
			for _, name := range field.Names {
				var val value.Value
				if argIdx < len(args) {
					val = args[argIdx]
				} else {
					val = value.NewNil()
				}
				interp.scope.Define(name.Name, val)
				argIdx++
			}
		}
	}

	// Record function entry
	interp.tracer.EnterFunc(funcInfo.Name, pos.Filename, pos.Line, interp.scope.Snapshot())

	// Execute function body
	var returnVal value.Value
	sig := interp.execBlockStmt(funcInfo.Decl.Body)
	if ret, ok := sig.(*signalReturn); ok {
		returnVal = ret.value
	}
	if returnVal == nil {
		returnVal = value.NewNil()
	}

	// Record function exit
	interp.tracer.ExitFunc(returnVal)

	// Restore parent scope
	interp.scope = parentScope

	return returnVal
}

// callFuncValue calls a FuncValue (function reference)
func (interp *Interpreter) callFuncValue(fv *value.FuncValue, args []value.Value) value.Value {
	if fv.Decl != nil {
		info := &loader.FuncInfo{
			Name:    fv.Name,
			PkgPath: fv.PkgPath,
			Decl:    fv.Decl,
		}
		return interp.callFunc(info, args, fv.Receiver)
	}

	return interp.callByName(fv.Name, args, fv.Receiver)
}

// callBuiltin handles built-in Go functions
func (interp *Interpreter) callBuiltin(name string, args []value.Value) (value.Value, bool) {
	switch name {
	case "len":
		if len(args) > 0 {
			switch v := args[0].(type) {
			case *value.SliceValue:
				return value.NewInt(int64(v.Len())), true
			case value.StringValue:
				return value.NewInt(int64(len(v.Val))), true
			case *value.MapValue:
				return value.NewInt(int64(len(v.Entries))), true
			}
		}
		return value.NewInt(0), true

	case "append":
		if len(args) >= 2 {
			if sv, ok := args[0].(*value.SliceValue); ok {
				newSlice := value.NewSlice(sv.ElemType)
				newSlice.Elements = make([]value.Value, len(sv.Elements))
				copy(newSlice.Elements, sv.Elements)
				for _, a := range args[1:] {
					newSlice.Append(a)
				}
				return newSlice, true
			}
		}
		return value.NewNil(), true

	case "make":
		return value.NewSlice(""), true

	case "fmt.Sprintf":
		if len(args) > 0 {
			if s, ok := args[0].(value.StringValue); ok {
				result := s.Val
				for i := 1; i < len(args); i++ {
					result = replaceFirst(result, "%v", args[i].String())
					result = replaceFirst(result, "%s", args[i].String())
					result = replaceFirst(result, "%d", args[i].String())
					result = replaceFirst(result, "%f", args[i].String())
				}
				return value.NewString(result), true
			}
		}
		return value.NewString(""), true

	case "fmt.Println", "fmt.Printf", "fmt.Print":
		return value.NewNil(), true
	}

	return value.NewNil(), false
}

func replaceFirst(s, old, new string) string {
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}
