package main

import (
	"fmt"
	"novum-lang/llvm/bindings/go/llvm"
	"os"
)

var (
	module        llvm.Module
	builder       = llvm.NewBuilder()
	namedValues   = map[string]llvm.Value{}
	fcPassManager llvm.PassManager
)

func InitModuleAndPassManager() {
	module = llvm.NewModule("novumroot")
	fcPassManager = llvm.NewFunctionPassManagerForModule(module)
	fcPassManager.AddInstructionCombiningPass()
	fcPassManager.AddGVNPass()
	fcPassManager.AddCFGSimplificationPass()
	fcPassManager.AddLoopUnswitchPass()
	fcPassManager.InitializeFunc()
}

func (s *StringAST) codegen() llvm.Value {
	return builder.CreateGlobalStringPtr(s.Value, "strtmp")
}

func (n *NumberLiteralAST) codegen() llvm.Value {
	if n.Kind() == astNumberInt {
		return llvm.ConstInt(llvm.Int32Type(), uint64(n.Value), false)
	}

	return llvm.ConstFloat(llvm.DoubleType(), n.Value)
}

func (v *VariableAST) codegen() llvm.Value {
	val, ok := namedValues[v.Name]

	if !ok {
		panic(fmt.Sprintf(`Variable "%s" does not exist!`, v.Name))
	}

	return val
}

func (b *BinaryAST) binOpNumberCodegen(l, r llvm.Value, kind string) llvm.Value {
	if l.IsNil() || r.IsNil() {
		panic("null operands")
	}

	switch b.Op {
	case "+":
		if kind == LitInt {
			return builder.CreateAdd(l, r, "addtmp")
		}
		return builder.CreateFAdd(l, r, "addtmp")
	case "-":
		if kind == LitInt {
			return builder.CreateSub(l, r, "addtmp")
		}
		return builder.CreateFSub(l, r, "subtmp")
	case "*":
		if kind == LitInt {
			return builder.CreateMul(l, r, "addtmp")
		}
		return builder.CreateFMul(l, r, "multmo")
	case "/":
		if kind == LitInt {
			return builder.CreateSDiv(l, r, "divtmp")
		}
		if os.Getenv("PRELUDE") == "empty" {
			return builder.CreateFDiv(l, r, "divtmp")
		}
		cond := builder.CreateFCmp(llvm.FloatOEQ, llvm.ConstFloat(llvm.DoubleType(), 0), r, "cmptmp")
		fc := builder.GetInsertBlock().Parent()
		thenBlock := llvm.AddBasicBlock(fc, "thendivchecker")
		elseBlock := llvm.AddBasicBlock(fc, "elsedivchecker")
		exitBlock := llvm.AddBasicBlock(fc, "exitdivchecker")
		builder.CreateCondBr(cond, thenBlock, elseBlock)

		builder.SetInsertPointAtEnd(thenBlock)
		calleePanic := module.NamedFunction("printf")
		if calleePanic.IsNil() {
			panic(fmt.Sprintf(`Function printf could not be referenced`))
		}

		panicMesssage := builder.CreateGlobalStringPtr("Panic: right side of the equation is equal to 0\n", "")
		argsValues := []llvm.Value{panicMesssage}
		builder.CreateCall(calleePanic, argsValues, "")

		calleeExit := module.NamedFunction("exit")
		if calleeExit.IsNil() {
			panic(fmt.Sprintf(`Function exit could not be referenced`))
		}

		builder.CreateCall(calleeExit, []llvm.Value{llvm.ConstInt(llvm.Int32Type(), 0, false)}, "")
		builder.CreateBr(exitBlock)

		builder.SetInsertPointAtEnd(elseBlock)
		result := builder.CreateFDiv(l, r, "divtmp")
		builder.CreateBr(exitBlock)

		builder.SetInsertPointAtEnd(exitBlock)
		return result
	case "<":
		if kind == LitInt {
			return builder.CreateICmp(llvm.IntSLT, l, r, "addtmp")
		}
		return builder.CreateFCmp(llvm.FloatOLT, l, r, "cmptmp")
	case ">":
		if kind == LitInt {
			return builder.CreateICmp(llvm.IntSLT, r, l, "addtmp")
		}
		return builder.CreateFCmp(llvm.FloatOLT, l, r, "cmptmp")
	case "==":
		if kind == LitInt {
			return builder.CreateICmp(llvm.IntEQ, l, r, "addtmp")
		}
		return builder.CreateFCmp(llvm.FloatOEQ, l, r, "cmptmp")
	case "!=":
		if kind == LitInt {
			return builder.CreateICmp(llvm.IntNE, l, r, "addtmp")
		}
		return builder.CreateFCmp(llvm.FloatONE, l, r, "cmptmp")
	default:
		panic(fmt.Sprintf(`Operator "%c" is invalid`, b.Op))
	}
}

func (b *BinaryAST) binOpStrCodegen(l, r llvm.Value) llvm.Value {
	if l.IsNil() || r.IsNil() {
		panic("null operands")
	}

	switch b.Op {
	case "+":
		// Todo figure out string concat
		panic("not implemented: String Concat")
	case "==":
		l = builder.CreatePointerCast(l, llvm.Int8Type(), "pointcast")
		r = builder.CreatePointerCast(r, llvm.Int8Type(), "pointcast")
		return builder.CreateICmp(llvm.IntEQ, l, r, "cmptmp")
	default:
		panic(fmt.Sprintf(`Operator "%c" is invalid`, b.Op))
	}
}

func (b *BinaryAST) binOpBoolCodegen(l, r llvm.Value) llvm.Value {
	if l.IsNil() || r.IsNil() {
		panic("null operands")
	}

	switch b.Op {
	case "!=":
		return builder.CreateICmp(llvm.IntNE, l, r, "cmptmp")
	case "==":
		return builder.CreateICmp(llvm.IntEQ, l, r, "cmptmp")
	default:
		panic(fmt.Sprintf(`Operator "%c" is invalid`, b.Op))
	}
}

func (b *BinaryAST) codegen() llvm.Value {
	l := b.Lhs.codegen()
	lKind := LLVMTypeToLit(l.Type())

	r := b.Rhs.codegen()
	rKind := LLVMTypeToLit(r.Type())

	binOp, ok := BinOpLookup["binary_"+b.Op]
	if ok {
		lOk := lKind == binOp[0].ArgType
		rOk := rKind == binOp[1].ArgType

		if lOk && rOk {
			callee := module.NamedFunction("binary_" + b.Op)
			if callee.IsNil() {
				panic(fmt.Sprintf(`Function "%s" could not be referenced`, b.Op))
			}

			if callee.ParamsCount() != 2 {
				panic(fmt.Sprintf(`Incorrect arguments passed in the function "%s"`, b.Op))
			}

			argsValues := []llvm.Value{l, r}

			return builder.CreateCall(callee, argsValues, "")
		}
	}

	if lKind != rKind {
		panic("Error: Left and right side of the binary operator don't have the same type")
	}

	switch lKind {
	case LitFloat, LitInt:
		return b.binOpNumberCodegen(l, r, lKind)
	case LitString:
		return b.binOpStrCodegen(l, r)
	case LitBool:
		return b.binOpBoolCodegen(l, r)
	default:
		panic("Error: '" + lKind + "' cannot be used with binary operator")
	}
}

func (u *UnaryAST) codegen() llvm.Value {
	operand := u.Operand.codegen()
	if operand.IsNil() {
		panic("Error: Unary operand does not exist")
	}

	callee := module.NamedFunction("unary_" + string(rune(u.Operator)))
	if callee.IsNil() {
		panic("Error: Unary operator '" + string(rune(u.Operator)) + "' does not exist")
	}

	return builder.CreateCall(callee, []llvm.Value{operand}, "")
}

func (c *CallAST) codegen() llvm.Value {
	callee := module.NamedFunction(c.Callee)

	if callee.IsNil() {
		panic(fmt.Sprintf(`Function "%s" could not be referenced`, c.Callee))
	}

	if callee.ParamsCount() != len(c.args) {
		panic(fmt.Sprintf(`Incorrect arguments passed in the function "%s"`, c.Callee))
	}

	var argsValues []llvm.Value

	for _, arg := range c.args {
		argVal := arg.codegen()
		if argVal.IsNil() {
			panic(fmt.Sprintf(`One of the arguments in function "%s" was null`, c.Callee))
		}

		argsValues = append(argsValues, argVal)
	}

	return builder.CreateCall(callee, argsValues, "")
}

func (p *PrototypeAST) codegen() llvm.Value {
	args := make([]llvm.Type, 0, len(p.Args))

	for _, a := range p.Args {
		switch a.ArgType {
		case LitFloat:
			args = append(args, llvm.DoubleType())
		case LitString:
			args = append(args, llvm.PointerType(llvm.Int8Type(), 0))
		case LitBool:
			args = append(args, llvm.Int1Type())
		case LitInt:
			args = append(args, llvm.Int32Type())
		default:
			panic(fmt.Sprintf("type-%s-does-no-exit", a.ArgType))
		}
	}
	var fcType llvm.Type

	switch p.ReturnType {
	case LitFloat:
		fcType = llvm.FunctionType(llvm.DoubleType(), args, false)
	case LitString:
		fcType = llvm.FunctionType(llvm.PointerType(llvm.Int8Type(), 0), args, false)
	case LitVoid:
		fcType = llvm.FunctionType(llvm.VoidType(), args, false)
	case LitInt:
		fcType = llvm.FunctionType(llvm.Int32Type(), args, false)
	case LitBool:
		fcType = llvm.FunctionType(llvm.Int1Type(), args, false)
	default:
		panic(fmt.Sprintf("type-%s-does-no-exit", p.ReturnType))
	}

	fc := llvm.AddFunction(module, p.Name, fcType)

	for i, param := range fc.Params() {
		param.SetName(p.Args[i].Name)
	}

	return fc
}

func (b *BlockAST) codegen() ([]llvm.Value, bool) {
	elements := []llvm.Value{}
	isReturn := false
	for _, stmt := range b.Elements {
		elements = append(elements, stmt.codegen())
		if stmt.Kind() == astReturn {
			isReturn = true
			break
		}
	}

	return elements, isReturn
}

// TODO Check for redefinition
func (p *FunctionAST) codegen() llvm.Value {
	fc := module.NamedFunction(p.Proto.Name)
	if fc.IsNil() {
		fc = p.Proto.codegen()
	}

	if fc.IsNil() {
		panic(fmt.Sprintf(`Could not create function "%s"`, p.Proto.Name))
	}
	block := llvm.AddBasicBlock(fc, "entry")
	builder.SetInsertPointAtEnd(block)

	namedValues = map[string]llvm.Value{}

	for _, param := range fc.Params() {
		namedValues[param.Name()] = param
	}

	p.Body.codegen()

	if llvm.VerifyFunction(fc, llvm.PrintMessageAction) != nil {
		fc.EraseFromParentAsFunction()
		panic(fmt.Sprintf(`Error occurred while verifing function "%s"`, p.Proto.Name))
	}

	if os.Getenv("DEBUG") != "true" {
		fcPassManager.RunFunc(fc)
	}

	return fc
}

func (i *IfElseAST) codegen() llvm.Value {
	cond := i.Condition.codegen()
	if cond.IsNil() {
		panic("No condition")
	}

	fc := builder.GetInsertBlock().Parent()
	thenBlock := llvm.AddBasicBlock(fc, "then")
	elseBlock := llvm.AddBasicBlock(fc, "else")
	exitBlock := llvm.AddBasicBlock(fc, "exit")

	builder.CreateCondBr(cond, thenBlock, elseBlock)

	// build then body
	builder.SetInsertPointAtEnd(thenBlock)
	_, isRet := i.TrueBody.codegen()

	if !isRet {
		builder.CreateBr(exitBlock)
	}

	// build elifs body
	for ind, el := range i.ElseIfBody {
		if ind == 0 {
			builder.SetInsertPointAtEnd(elseBlock)
			elseBlock = llvm.AddBasicBlock(fc, "else")
		}

		elifThenBlock := llvm.AddBasicBlock(fc, "then")
		elifElseBlock := llvm.AddBasicBlock(fc, "else")

		elifCond := el.Condition.codegen()
		if elifCond.IsNil() {
			panic("No condition in elif")
		}

		if ind == len(i.ElseIfBody)-1 {
			builder.CreateCondBr(elifCond, elifThenBlock, elseBlock)
		} else {
			builder.CreateCondBr(elifCond, elifThenBlock, elifElseBlock)
		}

		builder.SetInsertPointAtEnd(elifThenBlock)
		_, isRet := el.Body.codegen()

		if !isRet {
			builder.CreateBr(exitBlock)
		}

		builder.SetInsertPointAtEnd(elifElseBlock)
		if ind == len(i.ElseIfBody)-1 {
			builder.CreateBr(exitBlock)
		}
	}

	// build else body
	builder.SetInsertPointAtEnd(elseBlock)
	_, isRet = i.ElseBody.codegen()

	if !isRet {
		builder.CreateBr(exitBlock)
	}

	builder.SetInsertPointAtEnd(exitBlock)

	return cond
}

func (r *ReturnAST) codegen() llvm.Value {
	if r.Body == nil {
		return builder.CreateRetVoid()
	}
	return builder.CreateRet(r.Body.codegen())
}

func (l *LoopAST) codegen() llvm.Value {
	cond := l.Condition.codegen()
	if cond.IsNil() {
		panic("No condition in the loop")
	}

	zeroInd := llvm.ConstInt(llvm.Int32Type(), 0, false)
	var elemAlloca llvm.Value
	var gep llvm.Value
	var load llvm.Value

	if l.forIn {
		elemAlloca = builder.CreateArrayAlloca(cond.Operand(0).Operand(0).Type().ElementType(), llvm.ConstInt(llvm.Int32Type(), 1, false), "")
		gep = builder.CreateInBoundsGEP(cond, []llvm.Value{zeroInd}, "")
		load = builder.CreateLoad(gep, "load")
		builder.CreateStore(load, elemAlloca)
	}

	fc := builder.GetInsertBlock().Parent()
	headerBlock := builder.GetInsertBlock()
	loopBlock := llvm.AddBasicBlock(fc, "loop")
	exitBlock := llvm.AddBasicBlock(fc, "exitloop")

	if l.forIn {
		builder.CreateBr(loopBlock)
	} else {
		builder.CreateCondBr(builder.CreateICmp(llvm.IntNE, cond, llvm.ConstInt(llvm.Int1Type(), 0, false), "loopcond"), loopBlock, exitBlock)
	}

	builder.SetInsertPointAtEnd(loopBlock)
	valInd := builder.CreatePHI(llvm.Int32Type(), "ind")
	valInd.AddIncoming([]llvm.Value{llvm.ConstInt(llvm.Int32Type(), 0, false)}, []llvm.BasicBlock{headerBlock})

	// shadow variables with index and element
	oldValInd, okInd := namedValues[l.IndexVar]
	oldValElem, okElem := namedValues[l.ElementVar]
	if l.forIn {
		namedValues[l.IndexVar] = valInd
		namedValues[l.ElementVar] = elemAlloca
	}

	// TODO Check if loop's body does not have return inside
	_, isRet := l.Body.codegen()
	if !isRet {
		// Get next element from array and get next index
		nextInd := builder.CreateAdd(valInd, llvm.ConstInt(llvm.Int32Type(), 1, false), "nextind")

		var breakCond llvm.Value
		if l.forIn {
			gep = builder.CreateInBoundsGEP(cond, []llvm.Value{nextInd}, "")
			load = builder.CreateLoad(gep, "load")
			builder.CreateStore(load, elemAlloca)
			breakCond = builder.CreateICmp(llvm.IntNE, llvm.ConstInt(llvm.Int32Type(), uint64(cond.Operand(0).Operand(0).Type().ArrayLength()), false), nextInd, "loopcond")
		} else {
			breakCond = builder.CreateICmp(llvm.IntNE, cond, llvm.ConstInt(llvm.Int1Type(), 0, false), "loopcond")
		}

		loopExitBlock := builder.GetInsertBlock()
		builder.CreateCondBr(breakCond, loopBlock, exitBlock)
		builder.SetInsertPointAtEnd(exitBlock)
		valInd.AddIncoming([]llvm.Value{nextInd}, []llvm.BasicBlock{loopExitBlock})
	} else {
		l.kind = astReturn
	}

	// "unshadow" variables
	if okInd {
		namedValues[l.IndexVar] = oldValInd
	} else {
		delete(namedValues, l.IndexVar)
	}

	if okElem {
		namedValues[l.ElementVar] = oldValElem
	} else {
		delete(namedValues, l.ElementVar)
	}

	return llvm.ConstNull(llvm.Int1Type())
}

func (b *BoolAST) codegen() llvm.Value {
	return llvm.ConstInt(llvm.Int1Type(), uint64(b.Value), false)
}
