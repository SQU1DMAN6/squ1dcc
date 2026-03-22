package main

import "os"

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
			Return: SQXReturnBool,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 1); err != nil {
					return nil, err
				}
				var val bool
				var err error
				if val, err = SQXArgBool(args, 0); err != nil {
					return nil, err
				}
				return val, nil
			},
		},
	)

	os.Exit(module.Run(os.Args[1:], os.Stdout, os.Stderr))
}
