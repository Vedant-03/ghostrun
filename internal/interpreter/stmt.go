package interpreter

import (
	"fmt"
	"go/ast"
	"go/token"

	"dryrun/internal/value"
)

// signalReturn is used to propagate return values up the call stack
type signalReturn struct {
	value value.Value
}

// signalBreak is used to break out of loops
type signalBreak struct{}

// signalContinue is used to continue to next loop iteration
type signalContinue struct{}

// execStmt executes an AST statement, returning a signal if needed
func (interp *Interpreter) execStmt(stmt ast.Stmt) interface{} {
	if !interp.checkStepLimit() {
		return &signalReturn{value: value.NewNil()}
	}

	switch s := stmt.(type) {
	case *ast.BlockStmt:
		return interp.execBlockStmt(s)
	case *ast.ExprStmt:
		interp.evalExpr(s.X)
		return nil
	case *ast.AssignStmt:
		return interp.execAssignStmt(s)
	case *ast.DeclStmt:
		return interp.execDeclStmt(s)
	case *ast.ReturnStmt:
		return interp.execReturnStmt(s)
	case *ast.IfStmt:
		return interp.execIfStmt(s)
	case *ast.ForStmt:
		return interp.execForStmt(s)
	case *ast.RangeStmt:
		return interp.execRangeStmt(s)
	case *ast.SwitchStmt:
		return interp.execSwitchStmt(s)
	case *ast.IncDecStmt:
		return interp.execIncDecStmt(s)
	case *ast.BranchStmt:
		return interp.execBranchStmt(s)
	default:
		return nil
	}
}

func (interp *Interpreter) execBlockStmt(block *ast.BlockStmt) interface{} {
	if block == nil {
		return nil
	}
	for _, stmt := range block.List {
		if sig := interp.execStmt(stmt); sig != nil {
			return sig
		}
	}
	return nil
}

func (interp *Interpreter) execAssignStmt(stmt *ast.AssignStmt) interface{} {
	isCompound := stmt.Tok != token.ASSIGN && stmt.Tok != token.DEFINE

	// Evaluate all RHS values
	var rhs []value.Value
	if len(stmt.Rhs) == 1 && len(stmt.Lhs) > 1 {
		// Multi-value assignment: a, b := f()
		result := interp.evalExpr(stmt.Rhs[0])
		if mv, ok := result.(*value.MultiValue); ok {
			rhs = mv.Values
		} else {
			rhs = []value.Value{result}
		}
	} else {
		for _, r := range stmt.Rhs {
			rhs = append(rhs, interp.evalExpr(r))
		}
	}

	pos := interp.loader.GetPosition(stmt)

	for i, lhs := range stmt.Lhs {
		var rhsVal value.Value
		if i < len(rhs) {
			rhsVal = rhs[i]
		} else {
			rhsVal = value.NewNil()
		}

		switch l := lhs.(type) {
		case *ast.Ident:
			if l.Name == "_" {
				continue
			}

			val := rhsVal
			if isCompound {
				current, _ := interp.scope.Get(l.Name)
				val = applyCompoundOp(stmt.Tok, current, rhsVal)
			}

			if stmt.Tok == token.DEFINE {
				interp.scope.Define(l.Name, val)
			} else {
				if err := interp.scope.Set(l.Name, val); err != nil {
					interp.scope.Define(l.Name, val)
				}
			}
			interp.tracer.RecordAssign(l.Name, val, pos.Filename, pos.Line, interp.scope.Snapshot())

		case *ast.SelectorExpr:
			x := interp.evalExpr(l.X)
			if sv, ok := x.(*value.StructValue); ok {
				sv.SetField(l.Sel.Name, rhsVal)
				interp.tracer.RecordAssign(
					fmt.Sprintf("%s.%s", exprToString(l.X), l.Sel.Name),
					rhsVal, pos.Filename, pos.Line, interp.scope.Snapshot(),
				)
			}

		case *ast.IndexExpr:
			x := interp.evalExpr(l.X)
			idx := interp.evalExpr(l.Index)
			switch collection := x.(type) {
			case *value.SliceValue:
				if idxVal, ok := idx.(value.IntValue); ok {
					ii := int(idxVal.Val)
					if ii >= 0 && ii < len(collection.Elements) {
						collection.Elements[ii] = rhsVal
					}
				}
			case *value.MapValue:
				collection.Set(idx.String(), rhsVal)
			}
		}
	}

	return nil
}

// applyCompoundOp applies a compound assignment operation (+=, -=, etc.)
func applyCompoundOp(tok token.Token, current, rhs value.Value) value.Value {
	switch tok {
	case token.ADD_ASSIGN:
		if ls, ok := current.(value.StringValue); ok {
			if rs, ok := rhs.(value.StringValue); ok {
				return value.NewString(ls.Val + rs.Val)
			}
		}
		if isIntPair(current, rhs) {
			return value.NewInt(toInt64(current) + toInt64(rhs))
		}
		cv, cOk := toFloat64(current)
		rv, rOk := toFloat64(rhs)
		if cOk && rOk {
			return value.NewFloat(cv + rv)
		}
	case token.SUB_ASSIGN:
		if isIntPair(current, rhs) {
			return value.NewInt(toInt64(current) - toInt64(rhs))
		}
		cv, cOk := toFloat64(current)
		rv, rOk := toFloat64(rhs)
		if cOk && rOk {
			return value.NewFloat(cv - rv)
		}
	case token.MUL_ASSIGN:
		if isIntPair(current, rhs) {
			return value.NewInt(toInt64(current) * toInt64(rhs))
		}
		cv, cOk := toFloat64(current)
		rv, rOk := toFloat64(rhs)
		if cOk && rOk {
			return value.NewFloat(cv * rv)
		}
	case token.QUO_ASSIGN:
		if isIntPair(current, rhs) && toInt64(rhs) != 0 {
			return value.NewInt(toInt64(current) / toInt64(rhs))
		}
		cv, cOk := toFloat64(current)
		rv, rOk := toFloat64(rhs)
		if cOk && rOk && rv != 0 {
			return value.NewFloat(cv / rv)
		}
	case token.REM_ASSIGN:
		if isIntPair(current, rhs) && toInt64(rhs) != 0 {
			return value.NewInt(toInt64(current) % toInt64(rhs))
		}
	}
	return current
}

func (interp *Interpreter) execDeclStmt(stmt *ast.DeclStmt) interface{} {
	if genDecl, ok := stmt.Decl.(*ast.GenDecl); ok {
		for _, spec := range genDecl.Specs {
			if vs, ok := spec.(*ast.ValueSpec); ok {
				for i, name := range vs.Names {
					var val value.Value
					if i < len(vs.Values) {
						val = interp.evalExpr(vs.Values[i])
					} else {
						val = value.NewNil()
					}
					interp.scope.Define(name.Name, val)

					pos := interp.loader.GetPosition(stmt)
					interp.tracer.RecordAssign(name.Name, val, pos.Filename, pos.Line, interp.scope.Snapshot())
				}
			}
		}
	}
	return nil
}

func (interp *Interpreter) execReturnStmt(stmt *ast.ReturnStmt) interface{} {
	pos := interp.loader.GetPosition(stmt)

	if len(stmt.Results) == 0 {
		interp.tracer.RecordReturn(value.NewNil(), pos.Filename, pos.Line)
		return &signalReturn{value: value.NewNil()}
	}

	if len(stmt.Results) == 1 {
		val := interp.evalExpr(stmt.Results[0])
		interp.tracer.RecordReturn(val, pos.Filename, pos.Line)
		return &signalReturn{value: val}
	}

	vals := make([]value.Value, len(stmt.Results))
	for i, r := range stmt.Results {
		vals[i] = interp.evalExpr(r)
	}
	mv := &value.MultiValue{Values: vals}
	interp.tracer.RecordReturn(mv, pos.Filename, pos.Line)
	return &signalReturn{value: mv}
}

func (interp *Interpreter) execIfStmt(stmt *ast.IfStmt) interface{} {
	// Execute init statement if present
	if stmt.Init != nil {
		childScope := interp.scope.Child("if-init")
		parentScope := interp.scope
		interp.scope = childScope
		interp.execStmt(stmt.Init)
		defer func() { interp.scope = parentScope }()
	}

	cond := interp.evalExpr(stmt.Cond)
	taken := value.IsTruthy(cond)

	pos := interp.loader.GetPosition(stmt)
	interp.tracer.RecordBranch(exprToString(stmt.Cond), taken, pos.Filename, pos.Line, interp.scope.Snapshot())

	if taken {
		childScope := interp.scope.Child("if-body")
		parentScope := interp.scope
		interp.scope = childScope
		sig := interp.execBlockStmt(stmt.Body)
		interp.scope = parentScope
		return sig
	} else if stmt.Else != nil {
		childScope := interp.scope.Child("else-body")
		parentScope := interp.scope
		interp.scope = childScope
		sig := interp.execStmt(stmt.Else)
		interp.scope = parentScope
		return sig
	}

	return nil
}

func (interp *Interpreter) execForStmt(stmt *ast.ForStmt) interface{} {
	childScope := interp.scope.Child("for")
	parentScope := interp.scope
	interp.scope = childScope
	defer func() { interp.scope = parentScope }()

	if stmt.Init != nil {
		interp.execStmt(stmt.Init)
	}

	iterNum := 0
	for {
		if !interp.checkStepLimit() {
			break
		}

		if stmt.Cond != nil {
			cond := interp.evalExpr(stmt.Cond)
			if !value.IsTruthy(cond) {
				break
			}
		}

		pos := interp.loader.GetPosition(stmt)
		interp.tracer.RecordLoopIter(iterNum, "", pos.Filename, pos.Line, interp.scope.Snapshot())

		sig := interp.execBlockStmt(stmt.Body)
		if sig != nil {
			switch sig.(type) {
			case *signalReturn:
				return sig
			case *signalBreak:
				return nil
			case *signalContinue:
				// fall through to post
			}
		}

		if stmt.Post != nil {
			interp.execStmt(stmt.Post)
		}

		iterNum++
	}

	return nil
}

func (interp *Interpreter) execRangeStmt(stmt *ast.RangeStmt) interface{} {
	x := interp.evalExpr(stmt.X)

	childScope := interp.scope.Child("for-range")
	parentScope := interp.scope
	interp.scope = childScope
	defer func() { interp.scope = parentScope }()

	pos := interp.loader.GetPosition(stmt)

	switch collection := x.(type) {
	case *value.SliceValue:
		for i, elem := range collection.Elements {
			if !interp.checkStepLimit() {
				break
			}

			if stmt.Key != nil {
				if ident, ok := stmt.Key.(*ast.Ident); ok && ident.Name != "_" {
					if stmt.Tok == token.DEFINE {
						interp.scope.Define(ident.Name, value.NewInt(int64(i)))
					} else {
						interp.scope.Set(ident.Name, value.NewInt(int64(i)))
					}
				}
			}
			if stmt.Value != nil {
				if ident, ok := stmt.Value.(*ast.Ident); ok && ident.Name != "_" {
					if stmt.Tok == token.DEFINE {
						interp.scope.Define(ident.Name, elem)
					} else {
						interp.scope.Set(ident.Name, elem)
					}
				}
			}

			interp.tracer.RecordLoopIter(i, fmt.Sprintf("index=%d", i), pos.Filename, pos.Line, interp.scope.Snapshot())

			sig := interp.execBlockStmt(stmt.Body)
			if sig != nil {
				switch sig.(type) {
				case *signalReturn:
					return sig
				case *signalBreak:
					return nil
				case *signalContinue:
					continue
				}
			}
		}

	case *value.MapValue:
		i := 0
		for _, key := range collection.KeyOrder {
			if !interp.checkStepLimit() {
				break
			}

			val, _ := collection.Get(key)

			if stmt.Key != nil {
				if ident, ok := stmt.Key.(*ast.Ident); ok && ident.Name != "_" {
					if stmt.Tok == token.DEFINE {
						interp.scope.Define(ident.Name, value.NewString(key))
					} else {
						interp.scope.Set(ident.Name, value.NewString(key))
					}
				}
			}
			if stmt.Value != nil {
				if ident, ok := stmt.Value.(*ast.Ident); ok && ident.Name != "_" {
					if stmt.Tok == token.DEFINE {
						interp.scope.Define(ident.Name, val)
					} else {
						interp.scope.Set(ident.Name, val)
					}
				}
			}

			interp.tracer.RecordLoopIter(i, fmt.Sprintf("key=%s", key), pos.Filename, pos.Line, interp.scope.Snapshot())

			sig := interp.execBlockStmt(stmt.Body)
			if sig != nil {
				switch sig.(type) {
				case *signalReturn:
					return sig
				case *signalBreak:
					return nil
				case *signalContinue:
					continue
				}
			}
			i++
		}
	}

	return nil
}

func (interp *Interpreter) execSwitchStmt(stmt *ast.SwitchStmt) interface{} {
	if stmt.Init != nil {
		interp.execStmt(stmt.Init)
	}

	var tag value.Value
	if stmt.Tag != nil {
		tag = interp.evalExpr(stmt.Tag)
	}

	pos := interp.loader.GetPosition(stmt)

	for _, clause := range stmt.Body.List {
		cc := clause.(*ast.CaseClause)

		if cc.List == nil {
			// Default case
			interp.tracer.RecordBranch("default", true, pos.Filename, pos.Line, interp.scope.Snapshot())
			for _, s := range cc.Body {
				if sig := interp.execStmt(s); sig != nil {
					return sig
				}
			}
			return nil
		}

		matched := false
		for _, expr := range cc.List {
			caseVal := interp.evalExpr(expr)
			if tag != nil {
				if value.Equal(tag, caseVal) {
					matched = true
					break
				}
			} else {
				if value.IsTruthy(caseVal) {
					matched = true
					break
				}
			}
		}

		if matched {
			interp.tracer.RecordBranch(
				fmt.Sprintf("case %s", exprToString(cc.List[0])),
				true, pos.Filename, pos.Line, interp.scope.Snapshot(),
			)
			for _, s := range cc.Body {
				if sig := interp.execStmt(s); sig != nil {
					return sig
				}
			}
			return nil
		}
	}

	return nil
}

func (interp *Interpreter) execIncDecStmt(stmt *ast.IncDecStmt) interface{} {
	if ident, ok := stmt.X.(*ast.Ident); ok {
		current, ok := interp.scope.Get(ident.Name)
		if !ok {
			return nil
		}

		var newVal value.Value
		pos := interp.loader.GetPosition(stmt)

		switch stmt.Tok {
		case token.INC:
			if v, ok := current.(value.IntValue); ok {
				newVal = value.NewInt(v.Val + 1)
			} else if v, ok := current.(value.FloatValue); ok {
				newVal = value.NewFloat(v.Val + 1)
			}
		case token.DEC:
			if v, ok := current.(value.IntValue); ok {
				newVal = value.NewInt(v.Val - 1)
			} else if v, ok := current.(value.FloatValue); ok {
				newVal = value.NewFloat(v.Val - 1)
			}
		}

		if newVal != nil {
			interp.scope.Set(ident.Name, newVal)
			interp.tracer.RecordAssign(ident.Name, newVal, pos.Filename, pos.Line, interp.scope.Snapshot())
		}
	}
	return nil
}

func (interp *Interpreter) execBranchStmt(stmt *ast.BranchStmt) interface{} {
	switch stmt.Tok {
	case token.BREAK:
		return &signalBreak{}
	case token.CONTINUE:
		return &signalContinue{}
	}
	return nil
}
