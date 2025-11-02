package compiler

import (
	"fmt"
	"sort"
	"squ1d++/ast"
	"squ1d++/code"
	"squ1d++/object"
)

type LoopContext struct {
	loopStartPos  int
	breakJumps    []int
	continueJumps []int
}

type Compiler struct {
	instructions        code.Instructions
	constants           []object.Object
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
	symbolTable         *SymbolTable
	scopes              []CompilationScope
	scopeIndex          int
	loopContexts        []LoopContext
}

type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

type CompilationScope struct {
	instructions        code.Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
}

func New() *Compiler {
	mainScope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	symbolTable := NewSymbolTable()

	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	// Also define class objects (so code compiled with a fresh compiler can
	// reference class names like `array.cat`). Use the same order as the VM
	// / REPL expects.
	classes := object.CreateClassObjects()
	builtinCount := len(object.Builtins)
	classNames := []string{"io", "type", "time", "os", "math", "string", "file", "pkg", "array"}
	for _, className := range classNames {
		if _, ok := classes[className]; ok {
			symbolTable.DefineBuiltin(builtinCount, className)
			builtinCount++
		}
	}

	return &Compiler{
		constants:    []object.Object{},
		symbolTable:  symbolTable,
		scopes:       []CompilationScope{mainScope},
		scopeIndex:   0,
		loopContexts: []LoopContext{},
	}
}

func (c *Compiler) enterLoop(loopStartPos int) {
	c.loopContexts = append(c.loopContexts, LoopContext{
		loopStartPos:  loopStartPos,
		breakJumps:    []int{},
		continueJumps: []int{},
	})
}

func (c *Compiler) exitLoop() {
	if len(c.loopContexts) == 0 {
		return
	}

	// Get the current loop context
	loopCtx := c.loopContexts[len(c.loopContexts)-1]

	// Patch all break jumps to jump to current position (after loop)
	afterLoopPos := len(c.currentInstructions())
	for _, pos := range loopCtx.breakJumps {
		c.changeOperand(pos, afterLoopPos)
	}

	// Patch all continue jumps to jump to loop start
	for _, pos := range loopCtx.continueJumps {
		c.changeOperand(pos, loopCtx.loopStartPos)
	}

	// Remove the loop context
	c.loopContexts = c.loopContexts[:len(c.loopContexts)-1]
}

func (c *Compiler) addBreakJump(pos int) error {
	if len(c.loopContexts) == 0 {
		return fmt.Errorf("break statement not inside a loop")
	}

	c.loopContexts[len(c.loopContexts)-1].breakJumps = append(
		c.loopContexts[len(c.loopContexts)-1].breakJumps, pos)
	return nil
}

func (c *Compiler) addContinueJump(pos int) error {
	if len(c.loopContexts) == 0 {
		return fmt.Errorf("continue statement not inside a loop")
	}

	c.loopContexts[len(c.loopContexts)-1].continueJumps = append(
		c.loopContexts[len(c.loopContexts)-1].continueJumps, pos)
	return nil
}

func NewWithState(s *SymbolTable, constants []object.Object) *Compiler {
	compiler := New()
	compiler.symbolTable = s
	compiler.constants = constants
	return compiler
}

func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.ExpressionStatement:
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
		c.emit(code.OpPop)

	case *ast.BlockStatement:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.PrefixExpression:
		err := c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "!":
			c.emit(code.OpBang)
		case "-":
			c.emit(code.OpNGT)
		default:
			return fmt.Errorf("line %d, column %d: Unknown operator: %s", node.Token.Line, node.Token.Column, node.Operator)
		}

	case *ast.InfixExpression:
		if node.Operator == "<" {
			err := c.Compile(node.Right)
			if err != nil {
				return err
			}

			err = c.Compile(node.Left)
			if err != nil {
				return err
			}

			c.emit(code.OpGreaterThan)
			return nil
		}

		if node.Operator == "<=" {
			err := c.Compile(node.Left)
			if err != nil {
				return err
			}

			err = c.Compile(node.Right)
			if err != nil {
				return err
			}

			c.emit(code.OpGreaterThan)
			// Invert the result for <= (a <= b is !(a > b))
			c.emit(code.OpBang)
			return nil
		}

		if node.Operator == ">=" {
			err := c.Compile(node.Right)
			if err != nil {
				return err
			}

			err = c.Compile(node.Left)
			if err != nil {
				return err
			}

			c.emit(code.OpGreaterThan)
			// Invert the result for >= (a >= b is !(b > a))
			c.emit(code.OpBang)
			return nil
		}

		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "+":
			c.emit(code.OpAdd)
		case "-":
			c.emit(code.OpSub)
		case "*":
			c.emit(code.OpMul)
		case "/":
			c.emit(code.OpDiv)
		case "%":
			c.emit(code.OpMod)
		case ">":
			c.emit(code.OpGreaterThan)
		case "==":
			c.emit(code.OpEqual)
		case "!=":
			c.emit(code.OpNotEqual)
		case "and":
			c.emit(code.OpAnd)
		case "or":
			c.emit(code.OpOr)
		case "=":
			// Handle assignment
			if ident, ok := node.Left.(*ast.Identifier); ok {
				symbol, ok := c.symbolTable.Resolve(ident.Value)
				if !ok {
					return fmt.Errorf("line %d, column %d: Undefined variable %s", ident.Token.Line, ident.Token.Column, ident.Value)
				}

				if symbol.Scope == GlobalScope {
					c.emit(code.OpSetGlobal, symbol.Index)
				} else {
					c.emit(code.OpSetLocal, symbol.Index)
				}
			} else {
				return fmt.Errorf("line %d, column %d: Expected identifier for assignment, got %T", node.Token.Line, node.Token.Column, node.Left)
			}
		default:
			return fmt.Errorf("line %d, column %d: Unknown operator %s", node.Token.Line, node.Token.Column, node.Operator)
		}

	case *ast.IfExpression:
		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(node.Consequence)
		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}

		jumpPos := c.emit(code.OpJump, 9999)

		afterConsequencePos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, afterConsequencePos)

		if node.Alternative == nil {
			c.emit(code.OpNull)
		} else {
			err := c.Compile(node.Alternative)
			if err != nil {
				return err
			}

			if c.lastInstructionIs(code.OpPop) {
				c.removeLastPop()
			}
		}

		afterAlternativePos := len(c.currentInstructions())
		c.changeOperand(jumpPos, afterAlternativePos)

	case *ast.WhileExpression:
		loopStart := len(c.currentInstructions())
		c.enterLoop(loopStart)

		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}

		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(node.Body)
		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}

		// Jump back to the beginning of the loop
		c.emit(code.OpJump, loopStart)

		// Set the jump target for when condition is false
		afterLoopPos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, afterLoopPos)

		// Exit loop context (patches break/continue jumps)
		c.exitLoop()

		// Push null as the result of the while loop
		c.emit(code.OpNull)

	case *ast.WhileStatement:
		loopStart := len(c.currentInstructions())
		c.enterLoop(loopStart)

		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}

		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(node.Body)
		if err != nil {
			return err
		}

		// Pop the result of the body if it exists
		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}

		// Jump back to the beginning of the loop
		c.emit(code.OpJump, loopStart)

		// Set the jump target for when condition is false
		afterLoopPos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, afterLoopPos)

		// Exit loop context (patches break/continue jumps)
		c.exitLoop()

		// Pop the condition result and push null as the result of the while loop
		c.emit(code.OpPop)
		c.emit(code.OpNull)

	case *ast.LetStatement:
		symbol := c.symbolTable.Define(node.Name.Value)
		err := c.Compile(node.Value)
		if err != nil {
			return err
		}

		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

	case *ast.SuppressStatement:
		// Compile the inner expression but emit OpSuppress so the VM will pop
		// the value and mark the opcode as suppression (so REPL won't print it).
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
		c.emit(code.OpSuppress)

	case *ast.BreakStatement:
		jumpPos := c.emit(code.OpJump, 9999)
		err := c.addBreakJump(jumpPos)
		if err != nil {
			return err
		}

	case *ast.ContinueStatement:
		jumpPos := c.emit(code.OpJump, 9999)
		err := c.addContinueJump(jumpPos)
		if err != nil {
			return err
		}

	case *ast.ForStatement:
		// For loops are compiled as: init; while(condition) { body; update; }

		// Compile initialization
		if node.Init != nil {
			err := c.Compile(node.Init)
			if err != nil {
				return err
			}
			if c.lastInstructionIs(code.OpPop) {
				c.removeLastPop()
			}
		}

		loopStart := len(c.currentInstructions())
		c.enterLoop(loopStart)

		// Compile condition (default to true if no condition)
		if node.Condition != nil {
			err := c.Compile(node.Condition)
			if err != nil {
				return err
			}
		} else {
			// No condition means infinite loop (while true)
			c.emit(code.OpTrue)
		}

		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		// Compile body
		err := c.Compile(node.Body)
		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}

		// Compile update expression
		if node.Update != nil {
			err := c.Compile(node.Update)
			if err != nil {
				return err
			}
			if c.lastInstructionIs(code.OpPop) {
				c.removeLastPop()
			}
		}

		// Jump back to condition check
		c.emit(code.OpJump, loopStart)

		// Set jump target for when condition is false
		afterLoopPos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, afterLoopPos)

		// Exit loop context (patches break/continue jumps)
		c.exitLoop()

		// Push null as result
		c.emit(code.OpNull)

	case *ast.FunctionLiteral:
		c.enterScope()

		if node.Name != "" {
			c.symbolTable.DefineFunctionName(node.Name)
		}

		for _, p := range node.Parameters {
			c.symbolTable.Define(p.Value)
		}

		err := c.Compile(node.Body)
		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.replaceLastPopWithReturn()
		}
		if !c.lastInstructionIs(code.OpReturnValue) {
			c.emit(code.OpReturn)
		}

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		for _, s := range freeSymbols {
			c.loadSymbol(s)
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.Parameters),
		}

		fnIndex := c.addConstant(compiledFn)
		c.emit(code.OpClosure, fnIndex, len(freeSymbols))

	case *ast.ReturnStatement:
		err := c.Compile(node.ReturnValue)
		if err != nil {
			return err
		}

		c.emit(code.OpReturnValue)

	case *ast.CallExpression:
		err := c.Compile(node.Function)
		if err != nil {
			return err
		}

		for _, a := range node.Arguments {
			err := c.Compile(a)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpCall, len(node.Arguments))

	case *ast.IntegerLiteral:
		integer := &object.Integer{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(integer))

	case *ast.FloatLiteral:
		float := &object.Float{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(float))

	case *ast.StringLiteral:
		str := &object.String{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(str))

	case *ast.Boolean:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	case *ast.Null:
		c.emit(code.OpNull)

	case *ast.ArrayLiteral:
		for _, el := range node.Elements {
			err := c.Compile(el)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpArray, len(node.Elements))

	case *ast.HashLiteral:
		keys := []ast.Expression{}
		for k := range node.Pairs {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})

		for _, k := range keys {
			err := c.Compile(k)
			if err != nil {
				return err
			}
			err = c.Compile(node.Pairs[k])
			if err != nil {
				return err
			}
		}

		c.emit(code.OpHash, len(node.Pairs)*2)

	case *ast.IndexExpression:
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Index)
		if err != nil {
			return err
		}

		c.emit(code.OpIndex)

	case *ast.DotExpression:
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Right)
		if err != nil {
			return err
		}

		c.emit(code.OpDot)

	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(node.Value)
		if !ok {
			return fmt.Errorf("line %d, column %d: Undefined variable %s", node.Token.Line, node.Token.Column, node.Value)
		}

		// If this symbol is a builtin that belongs to a class, require dot
		// notation (e.g., array.append) to access it. Prevent calling class
		// scoped builtins by bare name at compile-time so dot-notation works.
		if symbol.Scope == BuiltinScope {
			if symbol.Index >= 0 && symbol.Index < len(object.Builtins) {
				def := object.Builtins[symbol.Index]
				if def.Builtin != nil && def.Builtin.Class != "" {
					return fmt.Errorf("line %d, column %d: Builtin '%s' is in a class. Maybe use %s.%s instead.", node.Token.Line, node.Token.Column, node.Value, def.Builtin.Class, node.Value)
				}
			}
		}

		c.loadSymbol(symbol)

	}

	return nil
}

func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)

	return pos
}

func (c *Compiler) currentInstructions() code.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.currentInstructions())
	updatedInstructions := append(c.currentInstructions(), ins...)
	c.scopes[c.scopeIndex].instructions = updatedInstructions
	return posNewInstruction
}

func (c *Compiler) setLastInstruction(op code.Opcode, pos int) {
	previous := c.scopes[c.scopeIndex].lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}

	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {
	if len(c.currentInstructions()) == 0 {
		return false
	}

	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

func (c *Compiler) removeLastPop() {
	last := c.scopes[c.scopeIndex].lastInstruction
	previous := c.scopes[c.scopeIndex].previousInstruction

	old := c.currentInstructions()
	new := old[:last.Position]

	c.scopes[c.scopeIndex].instructions = new
	c.scopes[c.scopeIndex].lastInstruction = previous
}

func (c *Compiler) replaceLastPopWithReturn() {
	lastPos := c.scopes[c.scopeIndex].lastInstruction.Position
	c.replaceInstruction(lastPos, code.Make(code.OpReturnValue))

	c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpReturnValue
}

func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {
	ins := c.currentInstructions()

	for i := 0; i < len(newInstruction); i++ {
		ins[pos+i] = newInstruction[i]
	}
}

func (c *Compiler) changeOperand(opPos int, operand int) {
	op := code.Opcode(c.currentInstructions()[opPos])
	newInstruction := code.Make(op, operand)

	c.replaceInstruction(opPos, newInstruction)
}

func (c *Compiler) enterScope() {
	scope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}
	c.scopes = append(c.scopes, scope)
	c.scopeIndex++
	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

func (c *Compiler) leaveScope() code.Instructions {
	instructions := c.currentInstructions()
	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--
	c.symbolTable = c.symbolTable.Outer

	return instructions
}

func (c *Compiler) loadSymbol(s Symbol) {
	switch s.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobal, s.Index)
	case LocalScope:
		c.emit(code.OpGetLocal, s.Index)
	case BuiltinScope:
		c.emit(code.OpGetBuiltin, s.Index)
	case FreeScope:
		c.emit(code.OpGetFree, s.Index)
	case FunctionScope:
		c.emit(code.OpCurrentClosure)
	}
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
	}
}

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
}
