package object

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// Helper function to create builtins with attributes
func createBuiltin(fn BuiltinFunction, class string, attributes map[string]Object) *Builtin {
	if attributes == nil {
		attributes = make(map[string]Object)
	}
	return &Builtin{
		Fn:         fn,
		Class:      class,
		Attributes: attributes,
	}
}

var Builtins = []struct {
	Name    string
	Builtin *Builtin
}{
	// Core builtins (no specific class)
	{
		"cat",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			switch arg := args[0].(type) {
			case *Array:
				return &Integer{Value: int64(len(arg.Elements))}
			case *String:
				return &Integer{Value: int64(len(arg.Value))}
			default:
				return newError("Argument 0 to `cat` is not supported, got %s", args[0].Type())
			}
		}, "", map[string]Object{
			"description": &String{Value: "Returns the length of an array or string"},
			"usage":       &String{Value: "cat(array_or_string)"},
		}),
	},
	{
		"tp",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			switch args[0].(type) {
			case *Array:
				return &String{Value: "Array"}
			case *String:
				return &String{Value: "String"}
			case *Hash:
				return &String{Value: "Object"}
			case *Integer:
				return &String{Value: "Integer"}
			case *Float:
				return &String{Value: "Float"}
			case *Boolean:
				return &String{Value: "Boolean"}
			case *Builtin:
				return &String{Value: "Builtin"}
			case *Function:
				return &String{Value: "Function"}
			default:
				return &String{Value: "Null"}
			}
		}, "", map[string]Object{
			"description": &String{Value: "Returns the type of an object"},
			"usage":       &String{Value: "tp(object)"},
		}),
	},
	{
		"append",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 2 {
				return newError("Wrong number of arguments. Expected 2, got %d", len(args))
			}

			arr, ok := args[0].(*Array)
			if !ok {
				return newError("Argument 0 to `append` must be ARRAY, got %s", args[0].Type())
			}

			length := len(arr.Elements)
			newElements := make([]Object, length+1, length+1)
			copy(newElements, arr.Elements)
			newElements[length] = args[1]

			return &Array{Elements: newElements}
		}, "", map[string]Object{
			"description": &String{Value: "Appends an element to an array"},
			"usage":       &String{Value: "append(array, element)"},
		}),
	},

	// Base builtins (no attributes)
	{
		"read",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 && len(args) != 1 {
				return newError("Wrong number of arguments. Expected 0 or 1, got %d", len(args))
			}

			if len(args) == 1 {
				prompt, ok := args[0].(*String)
				if !ok {
					return newError("Argument 0 to `read` must be STRING, got %s", args[0].Type())
				}
				fmt.Print(prompt.Value)
			}

			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return newError("Failed to read input: %s", err)
			}

			input = strings.TrimSpace(input)

			var value Object
			if intVal, err := strconv.ParseInt(input, 10, 64); err == nil {
				value = &Integer{Value: intVal}
			} else if floatVal, err := strconv.ParseFloat(input, 64); err == nil {
				value = &Float{Value: floatVal}
			} else {
				value = &String{Value: input}
			}

			return value
		}, "", nil),
	},
	{
		"write",
		createBuiltin(func(args ...Object) Object {
			var elements []string
			for _, arg := range args {
				elements = append(elements, arg.Inspect())
			}

			return &String{Value: strings.Join(elements, "")}
		}, "", nil),
	},
	{
		"null",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}
			return &Null{}
		}, "", nil),
	},
	// OS Builtins
	{
		"env",
		createBuiltin(func(args ...Object) Object {
			if len(args) == 0 {
				// Return all environment variables as a hash
				env := make(map[HashKey]HashPair)
				for _, e := range os.Environ() {
					parts := strings.SplitN(e, "=", 2)
					if len(parts) == 2 {
						key := &String{Value: parts[0]}
						value := &String{Value: parts[1]}
						env[key.HashKey()] = HashPair{Key: key, Value: value}
					}
				}
				return &Hash{Pairs: env}
			} else if len(args) == 1 {
				// Get specific environment variable
				key, ok := args[0].(*String)
				if !ok {
					return newError("Argument 0 to `env` must be STRING, got %s", args[0].Type())
				}
				value := os.Getenv(key.Value)
				if value == "" {
					return newError("Environment variable '%s' not found", key.Value)
				}
				return &String{Value: value}
			} else {
				return newError("Wrong number of arguments. Expected 0 or 1, got %d", len(args))
			}
		}, "os", map[string]Object{
			"description": &String{Value: "Gets environment variables"},
			"usage":       &String{Value: "env() or env(variable_name)"},
		}),
	},

	// Time builtins
	{
		"sleep",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			var duration time.Duration
			switch arg := args[0].(type) {
			case *Integer:
				duration = time.Duration(arg.Value) * time.Millisecond
			case *Float:
				duration = time.Duration(arg.Value*1000) * time.Millisecond
			default:
				return newError("Argument 0 to `sleep` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			time.Sleep(duration)
			return &Integer{Value: 0}
		}, "time", map[string]Object{
			"description": &String{Value: "Sleeps for the specified number of milliseconds"},
			"usage":       &String{Value: "sleep(milliseconds)"},
		}),
	},
	{
		"now",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}
			return &Integer{Value: time.Now().UnixMilli()}
		}, "time", map[string]Object{
			"description": &String{Value: "Returns current time in milliseconds since epoch"},
			"usage":       &String{Value: "now()"},
		}),
	},

	// Math builtins
	{
		"abs",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			switch arg := args[0].(type) {
			case *Integer:
				if arg.Value < 0 {
					return &Integer{Value: -arg.Value}
				}
				return arg
			case *Float:
				return &Float{Value: math.Abs(arg.Value)}
			default:
				return newError("Argument 0 to `abs` must be INTEGER or FLOAT, got %s", args[0].Type())
			}
		}, "math", map[string]Object{
			"description": &String{Value: "Returns absolute value of a number"},
			"usage":       &String{Value: "abs(number)"},
		}),
	},
	{
		"sqrt",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			var value float64
			switch arg := args[0].(type) {
			case *Integer:
				value = float64(arg.Value)
			case *Float:
				value = arg.Value
			default:
				return newError("Argument 0 to `sqrt` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			if value < 0 {
				return newError("Square root of negative number is not defined.")
			}

			return &Float{Value: math.Sqrt(value)}
		}, "math", map[string]Object{
			"description": &String{Value: "Returns square root of a number"},
			"usage":       &String{Value: "sqrt(number)"},
		}),
	},
	{
		"pow",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 2 {
				return newError("Wrong number of arguments. Expected 2, got %d", len(args))
			}

			var base, exponent float64

			switch arg := args[0].(type) {
			case *Integer:
				base = float64(arg.Value)
			case *Float:
				base = arg.Value
			default:
				return newError("Argument 0 to `pow` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			switch arg := args[1].(type) {
			case *Integer:
				exponent = float64(arg.Value)
			case *Float:
				exponent = arg.Value
			default:
				return newError("Argument 1 to `pow` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			return &Float{Value: math.Pow(base, exponent)}
		}, "math", map[string]Object{
			"description": &String{Value: "Returns base raised to the power of exponent"},
			"usage":       &String{Value: "pow(base, exponent)"},
		}),
	},
	{
		"sin",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			var value float64
			switch arg := args[0].(type) {
			case *Integer:
				value = float64(arg.Value)
			case *Float:
				value = arg.Value
			default:
				return newError("Argument 0 to `sin` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			return &Float{Value: math.Sin(value)}
		}, "math", map[string]Object{
			"description": &String{Value: "Returns sine of a number (in radians)"},
			"usage":       &String{Value: "sin(number)"},
		}),
	},
	{
		"cos",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			var value float64
			switch arg := args[0].(type) {
			case *Integer:
				value = float64(arg.Value)
			case *Float:
				value = arg.Value
			default:
				return newError("Argument 0 to `cos` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			return &Float{Value: math.Cos(value)}
		}, "math", map[string]Object{
			"description": &String{Value: "Returns cosine of a number (in radians)"},
			"usage":       &String{Value: "cos(number)"},
		}),
	},
	{
		"pi",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}
			return &Float{Value: math.Pi}
		}, "math", map[string]Object{
			"description": &String{Value: "Returns the mathematical constant π (pi)"},
			"usage":       &String{Value: "pi()"},
		}),
	},
	{
		"e",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}
			return &Float{Value: math.E}
		}, "math", map[string]Object{
			"description": &String{Value: "Returns the mathematical constant e"},
			"usage":       &String{Value: "e()"},
		}),
	},
}

func newError(format string, a ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}

func GetBuiltinByName(name string) *Builtin {
	for _, def := range Builtins {
		if def.Name == name {
			return def.Builtin
		}
	}

	return nil
}

// Create class objects for dot notation access
func CreateClassObjects() map[string]Object {
	classes := make(map[string]Object)

	// Create class objects with their functions as attributes
	timeClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	osClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	mathClass := &Hash{Pairs: make(map[HashKey]HashPair)}

	// Populate class objects with their functions
	for _, def := range Builtins {
		if def.Builtin.Class != "" {
			funcName := &String{Value: def.Name}
			key := funcName.HashKey()

			switch def.Builtin.Class {
			case "time":
				timeClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "os":
				osClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "math":
				mathClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			}
		}
	}

	classes["time"] = timeClass
	classes["os"] = osClass
	classes["math"] = mathClass

	return classes
}
