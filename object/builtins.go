package object

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var Builtins = []struct {
	Name    string
	Builtin *Builtin
}{
	{
		"cat",
		&Builtin{Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d",
					len(args))
			}

			switch arg := args[0].(type) {
			case *Array:
				return &Integer{Value: int64(len(arg.Elements))}
			case *String:
				return &Integer{Value: int64(len(arg.Value))}
			default:
				return newError("Argument 0 to `cat` is not supported, got %s",
					args[0].Type())
			}
		},
		},
	},
	{
		"tp",
		&Builtin{Fn: func(args ...Object) Object {
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
			case *Boolean:
				return &String{Value: "Boolean"}
			case *Function:
				return &String{Value: "Function"}
			default:
				return &String{Value: "Null"}
			}
		},
		},
	},
	{
		"read",
		&Builtin{Fn: func(args ...Object) Object {
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
			} else {
				value = &String{Value: input}
			}

			return value
		},
		},
	},
	{
		"write",
		&Builtin{Fn: func(args ...Object) Object {
			var elements []string
			for _, arg := range args {
				elements = append(elements, arg.Inspect())
			}

			return &String{Value: strings.Join(elements, "")}
		},
		},
	},
	{
		"append",
		&Builtin{Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newError("Wrong number of arguments. Expected 2, got %d",
					len(args))
			}

			arr, ok := args[0].(*Array)
			if !ok {
				return newError("Argument 0 to `append` must be ARRAY, got %s",
					args[0].Type())
			}

			length := len(arr.Elements)

			newElements := make([]Object, length+1, length+1)
			copy(newElements, arr.Elements)
			newElements[length] = args[1]

			return &Array{Elements: newElements}
		},
		},
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
