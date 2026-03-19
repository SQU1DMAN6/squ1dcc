package evaluator

import (
	"fmt"
	"os"
	"squ1d++/ast"
	"squ1d++/lexer"
	"squ1d++/object"
	"squ1d++/parser"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {

	// Statements
	case *ast.Program:
		return evalProgram(node, env)

	case *ast.BlockStatement:
		return evalBlockStatement(node, env)

	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)

	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}

	case *ast.LetStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			// If the error came from an error-pipe expression (<<), assign it.
			if prefix, ok := node.Value.(*ast.PrefixExpression); ok && prefix.Operator == "<<" {
				env.Set(node.Name.Value, val)
				return nil
			}

			// If error-pipe is used, assign the Error object to the variable.
			if node.ErrorPipe {
				env.Set(node.Name.Value, val)
				return nil
			}

			// Default behavior: assign null to the variable.
			env.Set(node.Name.Value, NULL)
			if node.Unblock {
				return nil
			}
			return val
		}

		// No error occurred
		if node.ErrorPipe {
			// Error-pipe returns null when the inner expression succeeded
			env.Set(node.Name.Value, NULL)
			return nil
		}

		env.Set(node.Name.Value, val)
		return nil

	case *ast.SuppressStatement:
		// Evaluate the inner statement or expression but always suppress
		// any output or error (compiler/VM uses OpSuppress for this).
		if node.Statement != nil {
			Eval(node.Statement, env)
			return NULL
		}

		if node.Expression != nil {
			Eval(node.Expression, env)
			return NULL
		}
		return NULL

	case *ast.WhileStatement:
		return evalWhileLoop(node.Condition, node.Body, env)

	// Expressions
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}

	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}

	case *ast.StringLiteral:
		return &object.String{Value: node.Value}

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)

	case *ast.Null:
		return NULL

	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		// Allow the error-pipe prefix (<<) to observe Error values — don't
		// short-circuit here for that operator. For all other prefixes, keep
		// the previous behavior of returning early on Error.
		if isError(right) && node.Operator != "<<" {
			return right
		}
		return evalPrefixExpression(node.Operator, right)

	case *ast.InfixExpression:
		// Assignment operator: handle specially so we can set identifiers
		if node.Operator == "=" {
			// Identifier assignment: var-like re-assignment
			if ident, ok := node.Left.(*ast.Identifier); ok {
				val := Eval(node.Right, env)
				if isError(val) {
					return val
				}
				env.Set(ident.Value, val)
				return nil
			}

			// Index assignment: e.g. arr[0] = x or hash["k"] = v
			if idxExpr, ok := node.Left.(*ast.IndexExpression); ok {
				leftObj := Eval(idxExpr.Left, env)
				if isError(leftObj) {
					return leftObj
				}

				index := Eval(idxExpr.Index, env)
				if isError(index) {
					return index
				}

				value := Eval(node.Right, env)
				if isError(value) {
					return value
				}

				switch leftObj.Type() {
				case object.ARRAY_OBJ:
					arr := leftObj.(*object.Array)
					if index.Type() != object.INTEGER_OBJ {
						return newError("Index operator requires integer for arrays, got %s", index.Type())
					}
					idx := int(index.(*object.Integer).Value)
					if idx < 0 || idx >= len(arr.Elements) {
						return newError("Index out of bounds: %d", idx)
					}
					arr.Elements[idx] = value
					return nil
				case object.HASH_OBJ:
					h := leftObj.(*object.Hash)
					key, ok := index.(object.Hashable)
					if !ok {
						return newError("%s is unusable as a hash key", index.Type())
					}
					h.Pairs[key.HashKey()] = object.HashPair{Key: index, Value: value}
					return nil
				default:
					return newError("Index operator is not supported: %s", leftObj.Type())
				}
			}

			return newError("Invalid assignment target: %T", node.Left)
		}

		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}

		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}

		return evalInfixExpression(node.Operator, left, right)

	case *ast.IfExpression:
		return evalIfExpression(node, env)

	case *ast.WhileExpression:
		return evalWhileLoop(node.Condition, node.Body, env)

	case *ast.BlockDirective:
		// Evaluate inner statement/expression and if it produces an Error,
		// propagate it so callers (like ExecuteFile) can handle an exit.
		if node.Statement != nil {
			// Special-case `block var x = def() { ... }` so we can check the
			// function body for undefined globals *before* executing it (to
			// mimic the compiler behavior of aborting on undefined symbols).
			if ls, ok := node.Statement.(*ast.LetStatement); ok {
				if fl, ok2 := ls.Value.(*ast.FunctionLiteral); ok2 {
					// build param set
					params := make(map[string]bool)
					for _, p := range fl.Parameters {
						params[p.Value] = true
					}
					if err := findUndefinedInNode(fl.Body, env, params); err != nil {
						return err
					}
				}
			}

			res := Eval(node.Statement, env)
			if isError(res) {
				return res
			}
			return NULL
		}

		if node.Expression != nil {
			res := Eval(node.Expression, env)
			if isError(res) {
				return res
			}
			return NULL
		}

	case *ast.Identifier:
		return evalIdentifier(node, env)

	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		return &object.Function{Parameters: params, Env: env, Body: body}

	case *ast.CallExpression:
		// Special handling for pkg.include(filename, namespace)
		if dotExpr, ok := node.Function.(*ast.DotExpression); ok {
			if ident, ok := dotExpr.Left.(*ast.Identifier); ok && ident.Value == "pkg" {
				switch rhs := dotExpr.Right.(type) {
				case *ast.Identifier:
					if rhs.Value == "include" {
						// This is pkg.include() - handle specially
						return evalPkgInclude(node, env)
					}
				case *ast.StringLiteral:
					if rhs.Value == "include" {
						// This is pkg.include() - handle specially
						return evalPkgInclude(node, env)
					}
				}
			}
		}

		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}

		args := evalExpressions(node.Arguments, env)

		return applyFunction(function, args)

	case *ast.ArrayLiteral:
		elements := evalExpressions(node.Elements, env)
		return &object.Array{Elements: elements}

	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)

	case *ast.HashLiteral:
		return evalHashLiteral(node, env)

	case *ast.DotExpression:
		return evalDotExpression(node, env)

	}

	return nil
}

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}

	return result
}

func evalBlockStatement(
	block *ast.BlockStatement,
	env *object.Environment,
) object.Object {
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}

	return result
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	case "<<":
		// Error-pipe as prefix expression: if the evaluated right side is an
		// Error, return that Error object; otherwise return NULL to indicate
		// no error value.
		if isError(right) {
			return right
		}
		return NULL
	default:
		return newError("Unknown operator: %s%s", operator, right.Type())
	}
}

func evalInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT_OBJ && right.Type() == object.FLOAT_OBJ:
		return evalFloatInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	case operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	case operator == "ac":
		return evalLogicalAndExpression(left, right)
	case operator == "aut":
		return evalLogicalOrExpression(left, right)
	case left.Type() != right.Type():
		return newError("Type mismatch: %s %s %s",
			left.Type(), operator, right.Type())
	default:
		return newError("Unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		return FALSE
	}
}

func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	if right.Type() != object.INTEGER_OBJ {
		return newError("Unknown operator: -%s", right.Type())
	}

	value := right.(*object.Integer).Value
	return &object.Integer{Value: -value}
}

func evalLogicalAndExpression(left, right object.Object) object.Object {
	leftTruthy := isTruthy(left)
	rightTruthy := isTruthy(right)
	return nativeBoolToBooleanObject(leftTruthy && rightTruthy)
}

func evalLogicalOrExpression(left, right object.Object) object.Object {
	leftTruthy := isTruthy(left)
	rightTruthy := isTruthy(right)
	return nativeBoolToBooleanObject(leftTruthy || rightTruthy)
}

func evalIntegerInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		return &object.Integer{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("Unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalFloatInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	leftVal := left.(*object.Float).Value
	rightVal := right.(*object.Float).Value

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "/":
		return &object.Float{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("Unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalStringInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value

	switch operator {
	case "+":
		return &object.String{Value: leftVal + rightVal}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("Unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalIfExpression(
	ie *ast.IfExpression,
	env *object.Environment,
) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func evalWhileLoop(condition ast.Expression, body *ast.BlockStatement, env *object.Environment) object.Object {
	for {
		cond := Eval(condition, env)
		if isError(cond) {
			return cond
		}

		if !isTruthy(cond) {
			return NULL
		}

		result := Eval(body, env)
		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}
}

func evalIdentifier(
	node *ast.Identifier,
	env *object.Environment,
) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}

	if builtin, ok := GetBuiltin(node.Value); ok {
		// Only allow direct access if the builtin has no class
		if builtin.Class == "" {
			return builtin
		}
		return newError("Builtin '%s' is in a class. Maybe use %s.%s instead.", node.Value, builtin.Class, node.Value)
	}

	return newError("Undefined variable %s", node.Value)
}

func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return true
	}
}

func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func evalExpressions(
	exps []ast.Expression,
	env *object.Environment,
) []object.Object {
	var result []object.Object

	for _, e := range exps {
		evaluated := Eval(e, env)
		result = append(result, evaluated)
	}

	return result
}

func applyFunction(fn object.Object, args []object.Object) object.Object {
	switch fn := fn.(type) {

	case *object.Function:
		// Check argument count to avoid panics and return a helpful error
		if len(args) != len(fn.Parameters) {
			return newError("Wrong number of arguments: expected %d, got %d", len(fn.Parameters), len(args))
		}

		extendedEnv := extendFunctionEnv(fn, args)
		evaluated := Eval(fn.Body, extendedEnv)
		return unwrapReturnValue(evaluated)

	case *object.Builtin:
		if result := fn.Fn(args...); result != nil {
			return result
		}
		return NULL

	default:
		return newError("%s is not a function.", fn.Type())
	}
}

func extendFunctionEnv(
	fn *object.Function,
	args []object.Object,
) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)

	for paramIdx, param := range fn.Parameters {
		env.Set(param.Value, args[paramIdx])
	}

	return env
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}

	return obj
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.HASH_OBJ:
		return evalHashIndexExpression(left, index)
	default:
		return newError("Index operator is not supported: %s", left.Type())
	}
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if idx < 0 || idx > max {
		return NULL
	}

	return arrayObject.Elements[idx]
}

func evalHashLiteral(
	node *ast.HashLiteral,
	env *object.Environment,
) object.Object {
	pairs := make(map[object.HashKey]object.HashPair)

	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env)
		if isError(key) {
			return key
		}

		hashKey, ok := key.(object.Hashable)
		if !ok {
			return newError("%s is unusable as a hash key", key.Type())
		}

		value := Eval(valueNode, env)
		if isError(value) {
			return value
		}

		hashed := hashKey.HashKey()
		pairs[hashed] = object.HashPair{Key: key, Value: value}
	}

	return &object.Hash{Pairs: pairs}
}

func evalHashIndexExpression(hash, index object.Object) object.Object {
	hashObject := hash.(*object.Hash)

	key, ok := index.(object.Hashable)
	if !ok {
		return newError("%s is unusable as a hash key", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}

	return pair.Value
}

func evalDotExpression(node *ast.DotExpression, env *object.Environment) object.Object {
	left := Eval(node.Left, env)
	if isError(left) {
		return left
	}

	// Handle builtin access like math.abs, time.sleep, etc.
	if left.Type() == object.HASH_OBJ {
		hash := left.(*object.Hash)

		// Get the right side (should be an identifier or string)
		var key string
		switch right := node.Right.(type) {
		case *ast.Identifier:
			key = right.Value
		case *ast.StringLiteral:
			key = right.Value
		default:
			return newError("Expected identifier or string after dot, got %T", right)
		}

		// Look for the key in the hash
		keyObj := &object.String{Value: key}
		if pair, ok := hash.Pairs[keyObj.HashKey()]; ok {
			return pair.Value
		}

		// If not found in hash, try to find it as a builtin
		// Construct the full name like "math.abs"
		if leftStr, ok := left.(*object.String); ok {
			fullName := leftStr.Value + "." + key
			if builtin, ok := GetBuiltin(fullName); ok {
				return builtin
			}
		}

		return newError("Property '%s' not found", key)
	}

	// Handle direct builtin access like math.abs
	if left.Type() == object.STRING_OBJ {
		leftStr := left.(*object.String).Value

		// Get the right side
		var key string
		switch right := node.Right.(type) {
		case *ast.Identifier:
			key = right.Value
		case *ast.StringLiteral:
			key = right.Value
		default:
			return newError("Expected identifier or string after dot, got %T", right)
		}

		fullName := leftStr + "." + key
		if builtin, ok := GetBuiltin(fullName); ok {
			return builtin
		}

		return newError("Builtin '%s' not found", fullName)
	}

	return newError("Dot operator not supported for type %s", left.Type())
}

func isErrorPipeExpression(node ast.Node) bool {
	if prefix, ok := node.(*ast.PrefixExpression); ok {
		return prefix.Operator == "<<"
	}
	return false
}

// findUndefinedInNode walks an AST subtree looking for identifiers that are
// not defined in the given environment and not present in the provided
// params map. It returns the first undefined identifier wrapped as an
// *object.Error (with Line/Column populated from the identifier token).
func findUndefinedInNode(node ast.Node, env *object.Environment, params map[string]bool) *object.Error {
	switch n := node.(type) {
	case *ast.Identifier:
		if params != nil {
			if _, ok := params[n.Value]; ok {
				return nil
			}
		}
		if _, ok := env.Get(n.Value); ok {
			return nil
		}
		if _, ok := GetBuiltin(n.Value); ok {
			return nil
		}
		return &object.Error{Message: fmt.Sprintf("Undefined variable %s", n.Value), Line: n.Token.Line, Column: n.Token.Column}
	case *ast.BlockStatement:
		for _, s := range n.Statements {
			if err := findUndefinedInNode(s, env, params); err != nil {
				return err
			}
		}
	case *ast.ExpressionStatement:
		return findUndefinedInNode(n.Expression, env, params)
	case *ast.ReturnStatement:
		return findUndefinedInNode(n.ReturnValue, env, params)
	case *ast.PrefixExpression:
		return findUndefinedInNode(n.Right, env, params)
	case *ast.InfixExpression:
		if err := findUndefinedInNode(n.Left, env, params); err != nil {
			return err
		}
		return findUndefinedInNode(n.Right, env, params)
	case *ast.IfExpression:
		if err := findUndefinedInNode(n.Condition, env, params); err != nil {
			return err
		}
		if err := findUndefinedInNode(n.Consequence, env, params); err != nil {
			return err
		}
		if n.Alternative != nil {
			return findUndefinedInNode(n.Alternative, env, params)
		}
	case *ast.FunctionLiteral:
		// For nested functions, we don't treat identifiers in the body as
		// undefined here because they may be resolved when the nested
		// function is executed; skip deeper checks for nested functions.
		return nil
	case *ast.CallExpression:
		if err := findUndefinedInNode(n.Function, env, params); err != nil {
			return err
		}
		for _, a := range n.Arguments {
			if err := findUndefinedInNode(a, env, params); err != nil {
				return err
			}
		}
	case *ast.IndexExpression:
		if err := findUndefinedInNode(n.Left, env, params); err != nil {
			return err
		}
		return findUndefinedInNode(n.Index, env, params)
	case *ast.ArrayLiteral:
		for _, el := range n.Elements {
			if err := findUndefinedInNode(el, env, params); err != nil {
				return err
			}
		}
	case *ast.HashLiteral:
		for k, v := range n.Pairs {
			if err := findUndefinedInNode(k, env, params); err != nil {
				return err
			}
			if err := findUndefinedInNode(v, env, params); err != nil {
				return err
			}
		}
	}
	return nil
}

// evalPkgInclude evaluates pkg.include(filename, namespace) which loads a file
// and makes its functions available under a namespace
func evalPkgInclude(node *ast.CallExpression, env *object.Environment) object.Object {
	if len(node.Arguments) != 2 {
		return newError("pkg.include expects 2 arguments (filename, namespace), got %d", len(node.Arguments))
	}

	// Evaluate the filename argument
	fileArg := Eval(node.Arguments[0], env)
	if isError(fileArg) {
		return fileArg
	}

	filename, ok := fileArg.(*object.String)
	if !ok {
		return newError("First argument to pkg.include must be STRING, got %s", fileArg.Type())
	}

	// Evaluate the namespace argument
	nsArg := Eval(node.Arguments[1], env)
	if isError(nsArg) {
		return nsArg
	}

	namespace, ok := nsArg.(*object.String)
	if !ok {
		return newError("Second argument to pkg.include must be STRING, got %s", nsArg.Type())
	}

	// Read the file
	content, err := os.ReadFile(filename.Value)
	if err != nil {
		return newError("Failed to read file '%s': %v", filename.Value, err)
	}

	// Parse the file
	l := lexer.New(string(content))
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		return newError("Parse errors in '%s': %v", filename.Value, p.Errors())
	}

	// Create a new environment that inherits from the current one
	// This allows the included code to access builtins and other definitions
	includeEnv := object.NewEnclosedEnvironment(env)

	// Evaluate the program in the new environment
	evalResult := Eval(program, includeEnv)
	if isError(evalResult) {
		return evalResult
	}

	// Now extract all variables from the included environment that are Functions/Builtins
	// and create a Hash to represent the namespace
	nsHash := &object.Hash{Pairs: make(map[object.HashKey]object.HashPair)}

	// Get all keys from the include environment's store (not outer)
	for name, obj := range includeEnv.GetStore() {
		// Skip internal/builtin variables - only include user-defined functions
		// We check if it's a Function or Builtin that was defined in this file
		switch obj.(type) {
		case *object.Function, *object.Builtin:
			key := &object.String{Value: name}
			nsHash.Pairs[key.HashKey()] = object.HashPair{Key: key, Value: obj}
		}
	}

	// Set the namespace in the calling environment
	env.Set(namespace.Value, nsHash)

	return &object.Null{}
}
