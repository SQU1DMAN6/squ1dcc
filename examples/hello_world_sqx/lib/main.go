package main

import (
	"fmt"
	"os"
)

func main() {
	module := NewSQXModule("hello_world")

	module.RegisterMany(
		// Add SQX methods here.
		// Name maps to <namespace>.<Name>() in SQU1DLang.
		// Handle receives CLI args as strings and returns a value/error.
		SQXMethod{
			Name:   "execute",
			Return: SQXReturnString,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 0); err != nil {
					return nil, err
				}
				return "Hello, World! This message is from SQX.", nil
			},
		},
		SQXMethod{
			Name:   "bool",
			Return: SQXReturnString,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 1); err != nil {
					return nil, err
				}
				val, err := SQXArgBool(args, 0)
				if err != nil {
					return nil, err
				}
				switch val {
				case true:
					return "The boolean provided was true", nil
				case false:
					return "The boolean provided was false", nil
				}
				return nil, nil
			},
		},
		SQXMethod{
			Name:   "add",
			Return: SQXReturnInt,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 2); err != nil {
					return nil, err
				}
				a, err := SQXArgInt(args, 0)
				if err != nil {
					return nil, err
				}
				b, err := SQXArgInt(args, 1)
				if err != nil {
					return nil, err
				}
				return a + b, nil
			},
		},
		SQXMethod{
			Name:   "sumArray",
			Return: SQXReturnInt,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 1); err != nil {
					return nil, err
				}

				value, err := SQXArgAny(args, 0)
				if err != nil {
					return nil, err
				}
				arr, ok := value.([]interface{})
				if !ok {
					return nil, fmt.Errorf("argument 0 must be an array")
				}

				var total int64
				for i, item := range arr {
					number, ok := item.(float64)
					if !ok {
						return nil, fmt.Errorf("array element %d must be numeric", i)
					}
					total += int64(number)
				}
				return total, nil
			},
		},
		SQXMethod{
			Name:   "countKeys",
			Return: SQXReturnInt,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 1); err != nil {
					return nil, err
				}

				value, err := SQXArgAny(args, 0)
				if err != nil {
					return nil, err
				}
				obj, ok := value.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("argument 0 must be an object")
				}
				return int64(len(obj)), nil
			},
		},
	)

	os.Exit(module.Run(os.Args[1:], os.Stdout, os.Stderr))
}
