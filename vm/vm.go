package vm

import (
	"fmt"
	"math"
	"squ1d++/code"
	"squ1d++/compiler"
	"squ1d++/object"
)

const StackSize = 2048
const GlobalsSize = 65536
const MaxFrames = 1024

var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}
var Null = &object.Null{}

type VM struct {
	constants   []object.Object
	stack       []object.Object
	sp          int
	globals     []object.Object
	frames      []*Frame
	framesIndex int
	lastOpcode  code.Opcode
}

func New(bytecode *compiler.Bytecode) *VM {
	mainFn := &object.CompiledFunction{Instructions: bytecode.Instructions}
	mainClosure := &object.Closure{Fn: mainFn}
	mainFrame := NewFrame(mainClosure, 0)

	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	return &VM{
		constants:   bytecode.Constants,
		stack:       make([]object.Object, StackSize),
		sp:          0,
		globals:     make([]object.Object, GlobalsSize),
		frames:      frames,
		framesIndex: 1,
		lastOpcode:  code.OpConstant, // Initialize with a safe default
	}
}

func NewWithGlobalsStore(bytecode *compiler.Bytecode, s []object.Object) *VM {
	vm := New(bytecode)
	vm.globals = s
	return vm
}

func (vm *VM) Run() error {
	var ip int
	var ins code.Instructions
	var op code.Opcode

	for vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		vm.currentFrame().ip++

		ip = vm.currentFrame().ip
		ins = vm.currentFrame().Instructions()

		op = code.Opcode(ins[ip])
		vm.lastOpcode = op

		switch op {
		case code.OpConstant:
			constIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			err := vm.push(vm.constants[constIndex])
			if err != nil {
				return err
			}

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod:
			err := vm.executeBinaryOperation(op)
			if err != nil {
				return err
			}

		case code.OpAnd, code.OpOr:
			err := vm.executeLogicalOperation(op)
			if err != nil {
				return err
			}

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan:
			err := vm.executeComparison(op)
			if err != nil {
				return err
			}

		case code.OpTrue:
			err := vm.push(True)
			if err != nil {
				return err
			}

		case code.OpFalse:
			err := vm.push(False)
			if err != nil {
				return err
			}

		case code.OpBang:
			err := vm.executeBangOperator()
			if err != nil {
				return err
			}

		case code.OpNGT:
			err := vm.executeNegateOperator()
			if err != nil {
				return err
			}

		case code.OpNull:
			err := vm.push(Null)
			if err != nil {
				return err
			}

		case code.OpArray:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			array := vm.buildArray(vm.sp-numElements, vm.sp)
			vm.sp = vm.sp - numElements

			err := vm.push(array)
			if err != nil {
				return err
			}

		case code.OpHash:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			hash, err := vm.buildHash(vm.sp-numElements, vm.sp)
			if err != nil {
				return err
			}

			vm.sp = vm.sp - numElements

			err = vm.push(hash)
			if err != nil {
				return err
			}

		case code.OpIndex:
			index := vm.pop()
			left := vm.pop()

			err := vm.executeIndexExpression(left, index)
			if err != nil {
				return err
			}

		case code.OpDot:
			right := vm.pop()
			left := vm.pop()

			err := vm.executeDotExpression(left, right)
			if err != nil {
				return err
			}

		case code.OpJump:
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip = pos - 1

		case code.OpJumpNotTruthy:
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			condition := vm.pop()
			if !isTruthy(condition) {
				vm.currentFrame().ip = pos - 1
			}

		case code.OpSetGlobal:
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			vm.globals[globalIndex] = vm.pop()

		case code.OpGetGlobal:
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			err := vm.push(vm.globals[globalIndex])
			if err != nil {
				return err
			}

		case code.OpSetLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			frame := vm.currentFrame()

			vm.stack[frame.basePointer+int(localIndex)] = vm.pop()

		case code.OpGetLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			frame := vm.currentFrame()

			err := vm.push(vm.stack[frame.basePointer+int(localIndex)])
			if err != nil {
				return err
			}

		case code.OpGetFree:
			freeIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			currentClosure := vm.currentFrame().cl

			err := vm.push(currentClosure.Free[freeIndex])
			if err != nil {
				return err
			}

		case code.OpGetBuiltin:
			builtinIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			if int(builtinIndex) < len(object.Builtins) {
				definition := object.Builtins[builtinIndex]
				err := vm.push(definition.Builtin)
				if err != nil {
					return err
				}
			} else {
				// Handle class objects
				classIndex := int(builtinIndex) - len(object.Builtins)
				classes := object.CreateClassObjects()
				classNames := []string{"io", "type", "time", "os", "math", "string", "file", "pkg", "array", "sys"}
				if classIndex < len(classNames) {
					className := classNames[classIndex]
					if classObj, ok := classes[className]; ok {
						err := vm.push(classObj)
						if err != nil {
							return err
						}
					} else {
						return fmt.Errorf("Class object not found: %s", className)
					}
				} else {
					return fmt.Errorf("Builtin index out of range: %d", builtinIndex)
				}
			}

		case code.OpCall:
			numArgs := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			err := vm.executeCall(int(numArgs))
			if err != nil {
				return err
			}

		case code.OpClosure:
			constIndex := code.ReadUint16(ins[ip+1:])
			numFree := code.ReadUint8(ins[ip+3:])

			vm.currentFrame().ip += 3

			err := vm.pushClosure(int(constIndex), int(numFree))
			if err != nil {
				return err
			}

		case code.OpCurrentClosure:
			currentClosure := vm.currentFrame().cl
			err := vm.push(currentClosure)
			if err != nil {
				return err
			}

		case code.OpReturnValue:
			returnValue := vm.pop()

			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1

			err := vm.push(returnValue)
			if err != nil {
				return err
			}

		case code.OpReturn:
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1

			err := vm.push(Null)
			if err != nil {
				return err
			}

		case code.OpPop:
			if vm.sp > 0 {
				vm.pop()
			}

		case code.OpSuppress:
			// Similar to OpPop but mark lastOpcode as OpSuppress (vm.lastOpcode is
			// already set to op before executing), so REPL/LastPoppedStackElem can
			// detect suppression and avoid printing. Still evaluate the expression
			// (it was previously pushed) and then discard the value.
			if vm.sp > 0 {
				vm.pop()
			}
		}
	}

	return nil
}

func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	leftType := left.Type()
	rightType := right.Type()

	switch {
	case leftType == object.INTEGER_OBJ && rightType == object.INTEGER_OBJ:
		return vm.executeBinaryIntegerOperation(op, left, right)
	case leftType == object.FLOAT_OBJ && rightType == object.FLOAT_OBJ:
		return vm.executeBinaryFloatOperation(op, left, right)
	case leftType == object.INTEGER_OBJ && rightType == object.FLOAT_OBJ:
		// Convert integer to float and perform float operation
		leftFloat := &object.Float{Value: float64(left.(*object.Integer).Value)}
		return vm.executeBinaryFloatOperation(op, leftFloat, right)
	case leftType == object.FLOAT_OBJ && rightType == object.INTEGER_OBJ:
		// Convert integer to float and perform float operation
		rightFloat := &object.Float{Value: float64(right.(*object.Integer).Value)}
		return vm.executeBinaryFloatOperation(op, left, rightFloat)
	case leftType == object.STRING_OBJ && rightType == object.STRING_OBJ:
		return vm.executeBinaryStringOperation(op, left, right)
	default:
		return fmt.Errorf("Unsupported types for binary operation: %s %s",
			leftType, rightType)
	}
}

func (vm *VM) executeComparison(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeIntegerComparison(op, left, right)
	} else if left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ {
		return vm.executeStringComparison(op, left, right)
	}

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(right == left))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(right != left))
	default:
		return fmt.Errorf("Unknown operator: %d (%s %s)",
			op, left.Type(), right.Type())
	}
}

func (vm *VM) executeBinaryIntegerOperation(
	op code.Opcode,
	left, right object.Object,
) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value

	var result int64

	switch op {
	case code.OpAdd:
		result = leftValue + rightValue

	case code.OpSub:
		result = leftValue - rightValue

	case code.OpMul:
		result = leftValue * rightValue

	case code.OpDiv:
		if rightValue == 0 {
			return fmt.Errorf("Division by zero")
		}
		result = leftValue / rightValue

	case code.OpMod:
		if rightValue == 0 {
			return fmt.Errorf("Modulo by zero")
		}
		result = leftValue % rightValue

	default:
		return fmt.Errorf("Unknown integer operator: %d", op)
	}

	return vm.push(&object.Integer{Value: result})
}

func (vm *VM) executeBinaryFloatOperation(
	op code.Opcode,
	left, right object.Object,
) error {
	leftValue := left.(*object.Float).Value
	rightValue := right.(*object.Float).Value

	var result float64

	switch op {
	case code.OpAdd:
		result = leftValue + rightValue
	case code.OpSub:
		result = leftValue - rightValue
	case code.OpMul:
		result = leftValue * rightValue
	case code.OpDiv:
		if rightValue == 0 {
			return fmt.Errorf("Division by zero")
		}
		result = leftValue / rightValue
	case code.OpMod:
		if rightValue == 0 {
			return fmt.Errorf("Modulo by zero")
		}
		result = math.Mod(leftValue, rightValue)
	default:
		return fmt.Errorf("Unknown float operation: %d", op)
	}

	return vm.push(&object.Float{Value: result})
}

func (vm *VM) executeBinaryStringOperation(
	op code.Opcode,
	left, right object.Object,
) error {
	if op != code.OpAdd {
		return fmt.Errorf("Unknown string operator: %d", op)
	}

	leftValue := left.(*object.String).Value
	rightValue := right.(*object.String).Value

	return vm.push(&object.String{Value: leftValue + rightValue})
}

func (vm *VM) executeIntegerComparison(
	op code.Opcode,
	left, right object.Object,
) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(rightValue == leftValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(rightValue != leftValue))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(leftValue > rightValue))
	default:
		return fmt.Errorf("Unknown operator: %d", op)
	}
}

func (vm *VM) executeStringComparison(
	op code.Opcode,
	left, right object.Object,
) error {
	leftValue := left.(*object.String).Value
	rightValue := right.(*object.String).Value

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(rightValue == leftValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(rightValue != leftValue))
	default:
		return fmt.Errorf("Unknown operator: %d", op)
	}
}

func (vm *VM) executeBangOperator() error {
	operand := vm.pop()

	switch operand {
	case True:
		return vm.push(False)
	case False:
		return vm.push(True)
	case Null:
		return vm.push(True)
	default:
		return vm.push(False)
	}
}

func (vm *VM) executeNegateOperator() error {
	operand := vm.pop()

	if operand.Type() != object.INTEGER_OBJ && operand.Type() != object.FLOAT_OBJ {
		return fmt.Errorf("Unsupported type for negation: %s", operand.Type())
	}

	value := operand.(*object.Integer).Value
	return vm.push(&object.Integer{Value: -value})
}

func (vm *VM) executeLogicalOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	leftTruthy := isTruthy(left)
	rightTruthy := isTruthy(right)

	var result bool
	switch op {
	case code.OpAnd:
		result = leftTruthy && rightTruthy
	case code.OpOr:
		result = leftTruthy || rightTruthy
	default:
		return fmt.Errorf("Unknown logical operator: %d", op)
	}

	return vm.push(nativeBoolToBooleanObject(result))
}

func (vm *VM) buildArray(startIndex, endIndex int) object.Object {
	elements := make([]object.Object, endIndex-startIndex)

	for i := startIndex; i < endIndex; i++ {
		elements[i-startIndex] = vm.stack[i]
	}

	return &object.Array{Elements: elements}
}

func (vm *VM) buildHash(startIndex, endIndex int) (object.Object, error) {
	hashedPairs := make(map[object.HashKey]object.HashPair)

	for i := startIndex; i < endIndex; i += 2 {
		key := vm.stack[i]
		value := vm.stack[i+1]
		pair := object.HashPair{Key: key, Value: value}
		hashKey, ok := key.(object.Hashable)
		if !ok {
			return nil, fmt.Errorf("unusable as hash key: %s", key.Type())
		}

		hashedPairs[hashKey.HashKey()] = pair
	}

	return &object.Hash{Pairs: hashedPairs}, nil
}

func (vm *VM) executeIndexExpression(left, index object.Object) error {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return vm.executeArrayIndex(left, index)

	case left.Type() == object.HASH_OBJ:
		return vm.executeHashIndex(left, index)

	default:
		return fmt.Errorf("Index operator not supported: %s", left.Type())
	}
}

func (vm *VM) executeDotExpression(left, right object.Object) error {
	switch {
	case left.Type() == object.HASH_OBJ:
		return vm.executeHashDot(left, right)
	default:
		return fmt.Errorf("Dot operator not supported: %s", left.Type())
	}
}

func (vm *VM) executeArrayIndex(array, index object.Object) error {
	arrayObject := array.(*object.Array)
	i := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if i < 0 || i > max {
		return vm.push(Null)
	}

	return vm.push(arrayObject.Elements[i])
}

func (vm *VM) executeHashIndex(hash, index object.Object) error {
	hashObject := hash.(*object.Hash)

	key, ok := index.(object.Hashable)
	if !ok {
		return fmt.Errorf("Unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return vm.push(Null)
	}

	return vm.push(pair.Value)
}

func (vm *VM) executeHashDot(hash, right object.Object) error {
	hashObject := hash.(*object.Hash)

	// For dot notation, the right side should be a string identifier
	var keyName string

	switch right := right.(type) {
	case *object.String:
		keyName = right.Value
	default:
		return fmt.Errorf("Dot operator requires string identifier, got: %s", right.Type())
	}

	// Create a string key for the hash lookup
	stringKey := &object.String{Value: keyName}
	pair, ok := hashObject.Pairs[stringKey.HashKey()]
	if !ok {
		return vm.push(Null)
	}

	return vm.push(pair.Value)
}

func (vm *VM) executeCall(numArgs int) error {
	callee := vm.stack[vm.sp-1-numArgs]

	switch callee := callee.(type) {
	case *object.Closure:
		return vm.callClosure(callee, numArgs)
	case *object.Builtin:
		return vm.callBuiltin(callee, numArgs)
	default:
		return fmt.Errorf("Calling non-function and non-builtin function.")
	}
}

func (vm *VM) callClosure(cl *object.Closure, numArgs int) error {
	if numArgs != cl.Fn.NumParameters {
		return fmt.Errorf("Wrong number of arguments. Expected %d, got %d",
			cl.Fn.NumParameters, numArgs)
	}

	frame := NewFrame(cl, vm.sp-numArgs)
	vm.pushFrame(frame)

	vm.sp = frame.basePointer + cl.Fn.NumLocals

	return nil
}

// func (vm *VM) callFunction(fn *object.CompiledFunction, numArgs int) error {
// 	if numArgs != fn.NumParameters {
// 		return fmt.Errorf("Wrong number of arguments. Expected %d, got %d",
// 			fn.NumParameters, numArgs)
// 	}

// 	frame := NewFrame(fn, vm.sp-numArgs)
// 	vm.pushFrame(frame)

// 	vm.sp = frame.basePointer + fn.NumLocals

// 	return nil
// }

func (vm *VM) callBuiltin(builtin *object.Builtin, numArgs int) error {
	args := vm.stack[vm.sp-numArgs : vm.sp]
	result := builtin.Fn(args...)
	vm.sp = vm.sp - numArgs - 1

	if result != nil {
		vm.push(result)
	} else {
		vm.push(Null)
	}

	return nil
}

func (vm *VM) pushClosure(constIndex int, numFree int) error {
	constant := vm.constants[constIndex]
	function, ok := constant.(*object.CompiledFunction)

	if !ok {
		return fmt.Errorf("%+v is not a function.", constant)
	}

	free := make([]object.Object, numFree)
	for i := 0; i < numFree; i++ {
		free[i] = vm.stack[vm.sp-numFree+i]
	}
	vm.sp = vm.sp - numFree

	closure := &object.Closure{Fn: function, Free: free}
	return vm.push(closure)
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	}
	return False
}

func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return false
	default:
		return true
	}
}

func (vm *VM) push(o object.Object) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("STACK OVERFLOW")
	}

	vm.stack[vm.sp] = o
	vm.sp++

	return nil
}

func (vm *VM) pop() object.Object {
	o := vm.stack[vm.sp-1]
	vm.sp--
	return o
}

func (vm *VM) StackTop() object.Object {
	if vm.sp == 0 {
		return nil
	}
	return vm.stack[vm.sp-1]
}

func (vm *VM) LastPoppedStackElem() object.Object {
	// Don't return values for variable assignments - they should be "pure" statements
	if vm.lastOpcode == code.OpSetGlobal || vm.lastOpcode == code.OpSetLocal {
		return nil
	}
	// Also suppress printing when the last opcode was OpSuppress
	if vm.lastOpcode == code.OpSuppress {
		return nil
	}
	return vm.stack[vm.sp]
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}

func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
}

// TriggerEvent executes registered GUI event handlers (closures) for the
// given event name. Handlers are looked up from object.GUIEventHandlers.
// Each handler must be a *object.Closure; the number of provided args must
// match the closure's parameter count. The method will run the VM until the
// handler returns before continuing to the next handler.
func (vm *VM) TriggerEvent(eventName string, args ...object.Object) error {
	if object.GUIEventHandlers == nil {
		return nil
	}

	handlers := object.GUIEventHandlers[eventName]
	if len(handlers) == 0 {
		return nil
	}

	for _, h := range handlers {
		cl, ok := h.(*object.Closure)
		if !ok {
			// non-callable handlers are ignored
			continue
		}

		// Parameter count must match
		if len(args) != cl.Fn.NumParameters {
			return fmt.Errorf("handler parameter mismatch for event '%s': expected %d, got %d",
				eventName, cl.Fn.NumParameters, len(args))
		}

		// Push the closure followed by its arguments (same layout as the
		// compiler produces before OpCall), then call the closure.
		if err := vm.push(cl); err != nil {
			return err
		}
		for _, a := range args {
			if err := vm.push(a); err != nil {
				return err
			}
		}

		if err := vm.callClosure(cl, len(args)); err != nil {
			return err
		}

		// Run VM until handler returns (the Run loop will execute frames on
		// top of the stack and return when there's nothing left to run).
		if err := vm.Run(); err != nil {
			return err
		}

		// Clean up any return value left on the stack from the handler.
		if vm.sp > 0 {
			vm.pop()
		}
	}

	return nil
}
