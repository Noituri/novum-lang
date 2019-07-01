package main

import (
	"ExpectoLang/llvm/bindings/go/llvm"
	"fmt"
	"strings"
)

var (
	module        llvm.Module
	builder       = llvm.NewBuilder()
	namedValues   = map[string]llvm.Value{}
	fcPassManager llvm.PassManager
)

func InitModuleAndPassManager() {
	module = llvm.NewModule("expectoroot")
	fcPassManager = llvm.NewFunctionPassManagerForModule(module)
	fcPassManager.AddInstructionCombiningPass()
	fcPassManager.AddGVNPass()
	fcPassManager.AddCFGSimplificationPass()
	fcPassManager.InitializeFunc()
}

func (s *StringAST) codegen() llvm.Value {
	return builder.CreateGlobalStringPtr(strings.ReplaceAll(s.Value, `\n`, "\n"), "strtmp")
}

func (n *NumberLiteralAST) codegen() llvm.Value {
	return llvm.ConstFloat(llvm.FloatType(), n.Value)
}

func (v *VariableAST) codegen() llvm.Value {
	val, ok := namedValues[v.Name]

	if !ok {
		panic(fmt.Sprintf(`Variable "%s" does not exist!`, v.Name))
	}

	return val
}

func (b *BinaryAST) codegen() llvm.Value {
	l := b.Lhs.codegen()
	r := b.Rhs.codegen()

	if l.IsNil() || r.IsNil() {
		panic("null operands")
	}

	// TODO check types
	switch b.Op {
	case '+':
		return builder.CreateFAdd(l, r, "addtmp")
	case '-':
		return builder.CreateFSub(l, r, "subtmp")
	case '*':
		return builder.CreateFMul(l, r, "multmo")
	case '/':
		// TODO check if r is not 0
		return builder.CreateFDiv(l, r, "divtmp")
	case '<':
		builder.CreateFCmp(llvm.FloatOLT, l, r, "cmptmp")
		return builder.CreateUIToFP(l, llvm.FloatType(), "booltmp")
	default:
		panic(fmt.Sprintf(`Operator "%c" is invalid`, b.Op))
	}
}

func (c *CallAST) codegen() llvm.Value {
	callee := module.NamedFunction(c.Callee)

	if callee.IsNil() {
		panic(fmt.Sprintf(`Procedure "%s" could not be referenced`, c.Callee))
	}

	if callee.ParamsCount() != len(c.args) {
		panic(fmt.Sprintf(`Incorrect arguments passed in procedure "%s"`, c.Callee))
	}

	argsValues := []llvm.Value{}

	for _, arg := range c.args {
		argVal := arg.codegen()
		if argVal.IsNil() {
			panic(fmt.Sprintf(`One of the arguments in procedure "%s" was null`, c.Callee))
		}

		argsValues = append(argsValues, argVal)
	}

	return builder.CreateCall(callee, argsValues, "calltmp")
}

func (p *PrototypeAST) codegen() llvm.Value {
	args := make([]llvm.Type, 0, len(p.Args))

	for _, a := range p.Args {
		// TODO use args type
		switch a.ArgType {
		case LitFloat:
			args = append(args, llvm.FloatType())
		case LitString:
			args = append(args, llvm.PointerType(llvm.Int8Type(), 0))
		default:
			panic(fmt.Sprintf("type-%s-does-no-exit", a.ArgType))
		}
	}

	var fcType llvm.Type

	switch p.ReturnType {
	case LitFloat:
		fcType = llvm.FunctionType(llvm.FloatType(), args, false)
	case LitString:
		fcType = llvm.FunctionType(llvm.PointerType(llvm.Int8Type(), 0), args, false)
	default:
		fcType = llvm.FunctionType(llvm.Int32Type(), args, false)
		//panic(fmt.Sprintf("type-%s-does-no-exit", p.ReturnType))
	}

	fc := llvm.AddFunction(module, p.Name, fcType)

	for i, param := range fc.Params() {
		param.SetName(p.Args[i].Name)
	}

	return fc
}

// TODO use this to generate body
func (b *BlockAST) codegen() llvm.Value {
	return llvm.Value{}
}

// TODO Check for redefinition
func (p *FunctionAST) codegen() llvm.Value {
	proc := module.NamedFunction(p.Proto.Name)

	if proc.IsNil() {
		proc = p.Proto.codegen()
	}

	if proc.IsNil() {
		panic(fmt.Sprintf(`Could not create procedure "%s"`, p.Proto.Name))
	}
	block := llvm.AddBasicBlock(proc, "entry")
	builder.SetInsertPointAtEnd(block)

	namedValues = map[string]llvm.Value{}

	for _, param := range proc.Params() {
		namedValues[param.Name()] = param
	}

	for _, stmt := range p.Body.Body[:len(p.Body.Body)-1] {
		stmt.codegen()
	}

	retVal := p.Body.Body[len(p.Body.Body)-1].codegen()

	if retVal.IsNil() {
		panic(fmt.Sprintf(`No return in procedure "%s"`, p.Proto.Name))
	}

	builder.CreateRet(retVal)

	if llvm.VerifyFunction(proc, llvm.PrintMessageAction) != nil {
		proc.EraseFromParentAsFunction()
		panic(fmt.Sprintf(`Error occurred while verifing procedure "%s"`, p.Proto.Name))
	}

	fcPassManager.RunFunc(proc)

	return proc
}
