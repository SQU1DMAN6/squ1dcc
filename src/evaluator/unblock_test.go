package evaluator

import (
	"fmt"
	"os"
	"squ1d++/lexer"
	"squ1d++/object"
	"squ1d++/parser"
	"testing"
)

func TestUnblockLetSwallowsError(t *testing.T) {
	input := `var func = def() { return y }
	unblock var x = func()
	5`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	evaluated := Eval(program, env)

	if evaluated == nil {
		t.Fatalf("expected a result, got nil")
	}

	if evaluated.Type() != object.INTEGER_OBJ {
		t.Fatalf("expected integer result, got %s", evaluated.Type())
	}

	if evaluated.(*object.Integer).Value != 5 {
		t.Fatalf("expected 5, got %d", evaluated.(*object.Integer).Value)
	}

	val, ok := env.Get("x")
	if !ok {
		t.Fatalf("expected variable x to be set")
	}

	if val.Type() != object.NULL_OBJ {
		t.Fatalf("expected x to be null, got %s", val.Type())
	}
}

func TestErrorPipeAssignsError(t *testing.T) {
	input := `var func = def() { return y }
	unblock var y = << func()
	5`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	evaluated := Eval(program, env)

	if evaluated == nil {
		t.Fatalf("expected a result, got nil")
	}

	if evaluated.Type() != object.INTEGER_OBJ {
		t.Fatalf("expected integer result, got %s", evaluated.Type())
	}

	if evaluated.(*object.Integer).Value != 5 {
		t.Fatalf("expected 5, got %d", evaluated.(*object.Integer).Value)
	}

	val, ok := env.Get("y")
	if !ok {
		t.Fatalf("expected variable y to be set")
	}

	if val.Type() != object.ERROR_OBJ {
		t.Fatalf("expected y to be Error object, got %s", val.Type())
	}
}

func TestPkgIncludeSqxPlugin(t *testing.T) {
	// Create a temporary plugin file using .sqx extension
	f, err := os.CreateTemp("", "plugin-*.sqx")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	pluginSource := `#!/usr/bin/env bash
set -euo pipefail
command="${1:-}"
case "$command" in
  __sqx_manifest__)
    printf '{"version":1,"functions":{"query":{"return":"auto"}}}'
    ;;
  __sqx_call__)
    fn="${2:-}"
    shift 2 || true
    case "$fn" in
      query) printf "1" ;;
      *) echo "unknown fn: $fn" >&2; exit 2 ;;
    esac
    ;;
  *)
    echo "unknown command: $command" >&2
    exit 2
    ;;
esac
`
	if _, err := f.WriteString(pluginSource); err != nil {
		t.Fatalf("failed to write plugin file: %v", err)
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0o755); err != nil {
		t.Fatalf("failed to chmod plugin file: %v", err)
	}

	input := fmt.Sprintf("pkg.include(\"%s\", \"sql\")\nsql.query(\"SELECT 1\")", f.Name())

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	evaluated := Eval(program, env)

	if evaluated == nil {
		t.Fatalf("expected a result, got nil")
	}

	integer, ok := evaluated.(*object.Integer)
	if !ok {
		t.Fatalf("expected integer result from plugin call, got %T (%v)", evaluated, evaluated)
	}
	if integer.Value != 1 {
		t.Fatalf("expected sql plugin query result 1, got %d", integer.Value)
	}
}

func TestPkgIncludeSqxPluginTypedAdd(t *testing.T) {
	f, err := os.CreateTemp("", "plugin-*.sqx")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	pluginSource := `#!/usr/bin/env bash
set -euo pipefail
command="${1:-}"
case "$command" in
  __sqx_manifest__)
    printf '{"version":1,"functions":{"add":{"return":"int"}}}'
    ;;
  __sqx_call__)
    fn="${2:-}"
    shift 2 || true
    case "$fn" in
      add)
        printf "%s" "$(($1 + $2))"
        ;;
      *) echo "unknown fn: $fn" >&2; exit 2 ;;
    esac
    ;;
  *) echo "unknown command: $command" >&2; exit 2 ;;
esac
`
	if _, err := f.WriteString(pluginSource); err != nil {
		t.Fatalf("failed to write plugin file: %v", err)
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0o755); err != nil {
		t.Fatalf("failed to chmod plugin file: %v", err)
	}

	input := fmt.Sprintf("pkg.include(\"%s\", \"sql\")\nsql.add(3, 5)", f.Name())

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	evaluated := Eval(program, env)

	if evaluated == nil {
		t.Fatalf("expected a result, got nil")
	}

	integer, ok := evaluated.(*object.Integer)
	if !ok {
		t.Fatalf("expected integer result from sql.add, got %T (%v)", evaluated, evaluated)
	}
	if integer.Value != 8 {
		t.Fatalf("expected sql.add(3, 5)=8, got %d", integer.Value)
	}
}
