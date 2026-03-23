package main

import (
    "fmt"
    "os"
    "strings"
)

func main() {
    module := NewSQXModule("sql")

    module.RegisterMany(
        SQXMethod{
            Name:   "ping",
            Return: SQXReturnString,
            Handle: func(args []string) (interface{}, error) {
                if err := SQXRequireArgs(args, 0); err != nil {
                    return nil, err
                }
                return "sql pong", nil
            },
        },
        SQXMethod{
            Name:   "query",
            Return: SQXReturnJSON,
            Handle: func(args []string) (interface{}, error) {
                if err := SQXRequireArgs(args, 1); err != nil {
                    return nil, err
                }
                q := strings.TrimSpace(strings.ToLower(args[0]))
                switch {
                case q == "select 1" || strings.HasPrefix(q, "select"):
                    return []map[string]interface{}{{"result": 1}}, nil
                case strings.HasPrefix(q, "insert"):
                    return map[string]interface{}{"ok": true}, nil
                default:
                    return nil, fmt.Errorf("unsupported query: %s", args[0])
                }
            },
        },
    )

    os.Exit(module.Run(os.Args[1:], os.Stdout, os.Stderr))
}
