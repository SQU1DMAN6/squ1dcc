package object

import (
	"fmt"
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
		"write",
		&Builtin{Fn: func(args ...Object) Object {
			for _, arg := range args {
				fmt.Println(arg.Inspect())
			}

			return nil
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
